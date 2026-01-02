package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
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

		// Extract if it's a .rar file
		extractedFiles, err := m.extractRarIfNeeded(destPath)
		if err != nil {
			log.Printf("Failed to extract rar file: %v", err)
			downloadedPaths = append(downloadedPaths, destPath)
		} else if len(extractedFiles) > 0 {
			// File was extracted, add extracted files and remove rar
			downloadedPaths = append(downloadedPaths, extractedFiles...)
			for _, extracted := range extractedFiles {
				if isVideoFile(extracted) {
					videoPaths = append(videoPaths, extracted)
				}
			}
		} else {
			downloadedPaths = append(downloadedPaths, destPath)
			if isVideoFile(destPath) {
				videoPaths = append(videoPaths, destPath)
			}
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
		writer:     out,
		total:      totalSize,
		download:   download,
		manager:    m,
		linkIndex:  linkIndex,
		totalLinks: totalLinks,
		lastUpdate: time.Now(),
	}

	// Copy with progress tracking
	_, err = io.Copy(pw, resp.Body)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	return nil
}

type progressWriter struct {
	writer     io.Writer
	total      int64
	written    int64
	download   *models.Download
	manager    *Manager
	linkIndex  int
	totalLinks int
	lastUpdate time.Time
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

func isRarFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".rar"
}

func (m *Manager) extractRarIfNeeded(destPath string) ([]string, error) {
	if !isRarFile(destPath) {
		return nil, nil
	}

	log.Printf("Detected rar file: %s, extracting...", filepath.Base(destPath))

	// Create extraction directory (same directory as rar file)
	extractDir := filepath.Dir(destPath)

	// Try to find a suitable RAR extraction tool
	var extractor string
	var args []string

	if _, err := exec.LookPath("unar"); err == nil {
		extractor = "unar"
		args = []string{"-o", extractDir + "/", destPath}
	} else if _, err := exec.LookPath("unrar"); err == nil {
		extractor = "unrar"
		args = []string{"x", "-y", destPath, extractDir + "/"}
	} else {
		return nil, fmt.Errorf("no RAR extraction tool found (unar or unrar required)")
	}

	// Extract the rar file
	cmd := exec.Command(extractor, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s failed: %w, output: %s", extractor, err, string(output))
	}

	log.Printf("Successfully extracted %s using %s", filepath.Base(destPath), extractor)

	// Get list of extracted files
	extractedFiles := []string{}
	files, err := os.ReadDir(extractDir)
	if err != nil {
		log.Printf("Warning: could not list files in %s: %v", extractDir, err)
	} else {
		rarModTime, _ := os.Stat(destPath)
		for _, file := range files {
			// Skip the rar file itself
			if file.Name() == filepath.Base(destPath) {
				continue
			}

			filePath := filepath.Join(extractDir, file.Name())
			fileInfo, _ := file.Info()

			// Check if this is a directory created by extraction or a newly created file
			if fileInfo != nil {
				if fileInfo.IsDir() {
					// List files in the extracted directory
					subFiles, err := os.ReadDir(filePath)
					if err != nil {
						log.Printf("Warning: could not read directory %s: %v", filePath, err)
					} else {
						for _, subFile := range subFiles {
							subFilePath := filepath.Join(filePath, subFile.Name())
							extractedFiles = append(extractedFiles, subFilePath)
							log.Printf("Extracted: %s/%s", file.Name(), subFile.Name())
						}
					}
				} else if fileInfo.ModTime().After(rarModTime.ModTime()) {
					// Newly created file at root level
					extractedFiles = append(extractedFiles, filePath)
					log.Printf("Extracted: %s", file.Name())
				}
			}
		}
	}

	// Remove the rar file
	if err := os.Remove(destPath); err != nil {
		log.Printf("Warning: could not remove rar file %s: %v", destPath, err)
	} else {
		log.Printf("Removed rar file: %s", filepath.Base(destPath))
	}

	if len(extractedFiles) == 0 {
		return nil, fmt.Errorf("no extracted files found")
	}

	return extractedFiles, nil
}
