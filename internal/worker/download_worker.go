package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ygncode/real-debrid-downloader/internal/models"
)

func (m *Manager) processDownload(download *models.Download) {
	log.Printf("Processing download: %s (status: %s)", download.Name, download.Status)

	switch download.Status {
	case models.StatusPending:
		m.pollUntilFilesReady(download)
	case models.StatusProcessing:
		m.pollUntilDownloaded(download)
	case models.StatusDownloading:
		m.downloadFiles(download)
	}
}

// pollUntilFilesReady polls Real-Debrid until files are ready for selection
func (m *Manager) pollUntilFilesReady(download *models.Download) {
	ctx, cancel := context.WithTimeout(m.ctx, 30*time.Minute)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.setError(download, "Timeout waiting for torrent to be ready")
			return
		case <-ticker.C:
			info, err := m.rdClient.GetTorrentInfo(ctx, download.TorrentID)
			if err != nil {
				log.Printf("Error getting torrent info: %v", err)
				continue
			}

			// Update name if we got it from the API
			if info.Filename != "" && download.Name == "Processing..." {
				download.Name = info.Filename
				m.repo.UpdateDownload(download)
				m.Broadcast(download)
			}

			switch info.Status {
			case models.RDStatusWaitingFilesSelection:
				// Files are ready for selection
				filesJSON, _ := json.Marshal(info.Files)
				download.FilesJSON = string(filesJSON)
				download.Status = models.StatusAwaitingSelection
				download.TotalSize = info.Bytes
				m.repo.UpdateDownload(download)
				m.Broadcast(download)
				log.Printf("Torrent %s ready for file selection", download.Name)
				return

			case models.RDStatusMagnetError, models.RDStatusError, models.RDStatusVirus, models.RDStatusDead:
				m.setError(download, fmt.Sprintf("Torrent error: %s", info.Status))
				return

			case models.RDStatusMagnetConversion:
				log.Printf("Torrent %s: converting magnet...", download.Name)
			}
		}
	}
}

// pollUntilDownloaded polls Real-Debrid until the torrent is fully downloaded
func (m *Manager) pollUntilDownloaded(download *models.Download) {
	ctx, cancel := context.WithTimeout(m.ctx, 24*time.Hour)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.setError(download, "Timeout waiting for torrent download")
			return
		case <-ticker.C:
			info, err := m.rdClient.GetTorrentInfo(ctx, download.TorrentID)
			if err != nil {
				log.Printf("Error getting torrent info: %v", err)
				continue
			}

			// Update progress
			download.Progress = info.Progress
			m.repo.UpdateDownload(download)
			m.Broadcast(download)

			switch info.Status {
			case models.RDStatusDownloaded:
				// Torrent is ready, store links and start downloading
				linksJSON, _ := json.Marshal(info.Links)
				download.Links = string(linksJSON)
				download.Status = models.StatusDownloading
				download.Progress = 0 // Reset progress for file download phase
				m.repo.UpdateDownload(download)
				m.Broadcast(download)
				log.Printf("Torrent %s downloaded on Real-Debrid, starting file download", download.Name)
				m.downloadFiles(download)
				return

			case models.RDStatusMagnetError, models.RDStatusError, models.RDStatusVirus, models.RDStatusDead:
				m.setError(download, fmt.Sprintf("Torrent error: %s", info.Status))
				return

			case models.RDStatusQueued:
				log.Printf("Torrent %s: queued on Real-Debrid", download.Name)

			case models.RDStatusDownloading:
				log.Printf("Torrent %s: downloading %.1f%%", download.Name, info.Progress)
			}
		}
	}
}

// downloadFiles downloads all files from the unrestricted links
func (m *Manager) downloadFiles(download *models.Download) {
	ctx := m.ctx

	// Parse links from JSON
	var links []string
	if err := json.Unmarshal([]byte(download.Links), &links); err != nil {
		m.setError(download, fmt.Sprintf("Failed to parse links: %v", err))
		return
	}

	if len(links) == 0 {
		m.setError(download, "No links to download")
		return
	}

	var downloadedPaths []string
	var videoPaths []string
	totalLinks := len(links)

	for i, link := range links {
		// Unrestrict the link
		unrestricted, err := m.rdClient.UnrestrictLink(ctx, link)
		if err != nil {
			log.Printf("Failed to unrestrict link %s: %v", link, err)
			continue
		}

		// Download the file
		destPath := filepath.Join(m.moviesPath, unrestricted.Filename)
		log.Printf("Downloading %s to %s", unrestricted.Filename, destPath)

		err = m.downloadFile(ctx, download, unrestricted.Download, destPath, unrestricted.Filesize, i, totalLinks)
		if err != nil {
			log.Printf("Failed to download %s: %v", unrestricted.Filename, err)
			continue
		}

		downloadedPaths = append(downloadedPaths, destPath)

		// Track video files for subtitle download
		if isVideoFile(destPath) {
			videoPaths = append(videoPaths, destPath)
		}
	}

	// Update paths
	pathsJSON, _ := json.Marshal(downloadedPaths)
	download.FilePaths = string(pathsJSON)

	// Download subtitles if enabled
	if download.DownloadSubs && len(videoPaths) > 0 && m.subtitleService.IsAvailable() {
		download.Status = models.StatusSubtitles
		download.SubtitleStatus = "Downloading subtitles..."
		download.Progress = 100
		m.repo.UpdateDownload(download)
		m.Broadcast(download)

		subtitleResults := []string{}
		for _, videoPath := range videoPaths {
			log.Printf("Downloading subtitles for %s", videoPath)
			if err := m.subtitleService.DownloadSubtitles(videoPath); err != nil {
				log.Printf("Failed to download subtitles for %s: %v", videoPath, err)
				subtitleResults = append(subtitleResults, fmt.Sprintf("%s: failed", filepath.Base(videoPath)))
			} else {
				subtitleResults = append(subtitleResults, fmt.Sprintf("%s: ok", filepath.Base(videoPath)))
			}
		}
		download.SubtitleStatus = strings.Join(subtitleResults, ", ")
	} else if download.DownloadSubs && !m.subtitleService.IsAvailable() {
		download.SubtitleStatus = "Skipped (subliminal not installed)"
	} else if !download.DownloadSubs {
		download.SubtitleStatus = "Disabled"
	}

	// Update final status
	download.Status = models.StatusComplete
	download.Progress = 100
	m.repo.UpdateDownload(download)
	m.Broadcast(download)
	log.Printf("Download complete: %s", download.Name)
}

// downloadFile downloads a single file with progress tracking
func (m *Manager) downloadFile(ctx context.Context, download *models.Download, url, destPath string, totalSize int64, linkIndex, totalLinks int) error {
	// Create the destination file
	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	// Start download request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to start download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	// Use content-length if available
	if totalSize == 0 && resp.ContentLength > 0 {
		totalSize = resp.ContentLength
	}

	// Create progress writer
	pw := &progressWriter{
		writer:      out,
		total:       totalSize,
		download:    download,
		manager:     m,
		linkIndex:   linkIndex,
		totalLinks:  totalLinks,
		lastUpdate:  time.Now(),
	}

	// Copy with progress tracking
	_, err = io.Copy(pw, resp.Body)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	return nil
}

type progressWriter struct {
	writer      io.Writer
	total       int64
	written     int64
	download    *models.Download
	manager     *Manager
	linkIndex   int
	totalLinks  int
	lastUpdate  time.Time
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n, err := pw.writer.Write(p)
	pw.written += int64(n)

	// Update progress every second
	if time.Since(pw.lastUpdate) > time.Second {
		pw.lastUpdate = time.Now()

		var progress float64
		if pw.total > 0 {
			// Calculate progress including completed links
			linkProgress := float64(pw.written) / float64(pw.total) * 100
			progress = (float64(pw.linkIndex)*100 + linkProgress) / float64(pw.totalLinks)
		}

		pw.download.Progress = progress
		pw.download.Downloaded = pw.written
		pw.manager.repo.UpdateDownloadProgress(pw.download.ID, progress, pw.written)
		pw.manager.Broadcast(pw.download)
	}

	return n, err
}

func (m *Manager) setError(download *models.Download, msg string) {
	log.Printf("Download error for %s: %s", download.Name, msg)
	download.Status = models.StatusError
	download.ErrorMessage = msg
	m.repo.UpdateDownload(download)
	m.Broadcast(download)
}

func isVideoFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	videoExts := []string{".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v"}
	for _, ve := range videoExts {
		if ext == ve {
			return true
		}
	}
	return false
}
