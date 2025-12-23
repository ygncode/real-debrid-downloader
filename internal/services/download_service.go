package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"time"

	"github.com/ygncode/real-debrid-downloader/internal/models"
	"github.com/ygncode/real-debrid-downloader/internal/realdebrid"
	"github.com/ygncode/real-debrid-downloader/internal/storage"
)

type DownloadService struct {
	repo            *storage.Repository
	rdClient        *realdebrid.Client
	moviesPath      string
	subtitleService *SubtitleService
}

func NewDownloadService(
	repo *storage.Repository,
	rdClient *realdebrid.Client,
	moviesPath string,
	subtitleService *SubtitleService,
) *DownloadService {
	return &DownloadService{
		repo:            repo,
		rdClient:        rdClient,
		moviesPath:      moviesPath,
		subtitleService: subtitleService,
	}
}

// AddMagnet adds a magnet link and creates a download entry
func (s *DownloadService) AddMagnet(ctx context.Context, magnetLink string, downloadSubs bool) (*models.Download, error) {
	// Extract name from magnet link if possible
	name := extractNameFromMagnet(magnetLink)
	if name == "" {
		name = "Processing..."
	}

	// Add magnet to Real-Debrid
	result, err := s.rdClient.AddMagnet(ctx, magnetLink)
	if err != nil {
		return nil, fmt.Errorf("failed to add magnet to Real-Debrid: %w", err)
	}

	// Create download entry
	download := &models.Download{
		TorrentID:    result.ID,
		Name:         name,
		Status:       models.StatusPending,
		Progress:     0,
		DownloadSubs: downloadSubs,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.repo.CreateDownload(download); err != nil {
		// Try to clean up the torrent on Real-Debrid
		s.rdClient.DeleteTorrent(ctx, result.ID)
		return nil, fmt.Errorf("failed to create download entry: %w", err)
	}

	return download, nil
}

// AddTorrent uploads a .torrent file and creates a download entry
func (s *DownloadService) AddTorrent(ctx context.Context, filename string, torrentData io.Reader, downloadSubs bool) (*models.Download, error) {
	// Add torrent to Real-Debrid
	result, err := s.rdClient.AddTorrent(ctx, filename, torrentData)
	if err != nil {
		return nil, fmt.Errorf("failed to add torrent to Real-Debrid: %w", err)
	}

	// Create download entry
	download := &models.Download{
		TorrentID:    result.ID,
		Name:         filename,
		Status:       models.StatusPending,
		Progress:     0,
		DownloadSubs: downloadSubs,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.repo.CreateDownload(download); err != nil {
		s.rdClient.DeleteTorrent(ctx, result.ID)
		return nil, fmt.Errorf("failed to create download entry: %w", err)
	}

	return download, nil
}

// SelectFiles selects which files to download from a torrent
func (s *DownloadService) SelectFiles(ctx context.Context, downloadID uint, fileIDs string) error {
	download, err := s.repo.GetDownload(downloadID)
	if err != nil {
		return fmt.Errorf("download not found: %w", err)
	}

	if download.Status != models.StatusAwaitingSelection {
		return fmt.Errorf("download is not awaiting file selection")
	}

	// Select files on Real-Debrid
	if err := s.rdClient.SelectFiles(ctx, download.TorrentID, fileIDs); err != nil {
		return fmt.Errorf("failed to select files: %w", err)
	}

	// Update download status
	download.SelectedIDs = fileIDs
	download.Status = models.StatusProcessing
	download.UpdatedAt = time.Now()

	if err := s.repo.UpdateDownload(download); err != nil {
		return fmt.Errorf("failed to update download: %w", err)
	}

	return nil
}

// GetDownload retrieves a download by ID
func (s *DownloadService) GetDownload(id uint) (*models.Download, error) {
	return s.repo.GetDownload(id)
}

// GetAllDownloads retrieves all downloads
func (s *DownloadService) GetAllDownloads() ([]models.Download, error) {
	return s.repo.GetAllDownloads()
}

// GetDownloadFiles returns the files available for selection
func (s *DownloadService) GetDownloadFiles(downloadID uint) ([]models.TorrentFile, error) {
	download, err := s.repo.GetDownload(downloadID)
	if err != nil {
		return nil, err
	}

	if download.FilesJSON == "" {
		return nil, fmt.Errorf("no files available for selection")
	}

	var files []models.TorrentFile
	if err := json.Unmarshal([]byte(download.FilesJSON), &files); err != nil {
		return nil, fmt.Errorf("failed to parse files: %w", err)
	}

	return files, nil
}

// DeleteDownload removes a download entry and optionally the torrent from Real-Debrid
func (s *DownloadService) DeleteDownload(ctx context.Context, id uint) error {
	download, err := s.repo.GetDownload(id)
	if err != nil {
		return err
	}

	// Try to delete from Real-Debrid (ignore errors)
	if download.TorrentID != "" {
		s.rdClient.DeleteTorrent(ctx, download.TorrentID)
	}

	return s.repo.DeleteDownload(id)
}

// extractNameFromMagnet extracts the display name from a magnet link
func extractNameFromMagnet(magnet string) string {
	re := regexp.MustCompile(`dn=([^&]+)`)
	matches := re.FindStringSubmatch(magnet)
	if len(matches) > 1 {
		// Properly URL decode the name
		name, err := url.QueryUnescape(matches[1])
		if err != nil {
			// Fallback: replace + with space if decode fails
			name = regexp.MustCompile(`\+`).ReplaceAllString(matches[1], " ")
		}
		return name
	}
	return ""
}
