package realdebrid

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"

	"github.com/ygncode/real-debrid-downloader/internal/models"
)

// AddMagnet adds a magnet link to Real-Debrid
func (c *Client) AddMagnet(ctx context.Context, magnet string) (*models.AddTorrentResponse, error) {
	data := url.Values{}
	data.Set("magnet", magnet)

	var result models.AddTorrentResponse
	if err := c.post(ctx, "/torrents/addMagnet", data, &result); err != nil {
		return nil, fmt.Errorf("failed to add magnet: %w", err)
	}

	return &result, nil
}

// AddTorrent uploads a .torrent file to Real-Debrid
func (c *Client) AddTorrent(ctx context.Context, filename string, torrentData io.Reader) (*models.AddTorrentResponse, error) {
	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(part, torrentData); err != nil {
		return nil, fmt.Errorf("failed to copy torrent data: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	var result models.AddTorrentResponse
	if err := c.put(ctx, "/torrents/addTorrent", &buf, writer.FormDataContentType(), &result); err != nil {
		return nil, fmt.Errorf("failed to add torrent: %w", err)
	}

	return &result, nil
}

// GetTorrentInfo retrieves information about a specific torrent
func (c *Client) GetTorrentInfo(ctx context.Context, torrentID string) (*models.TorrentInfo, error) {
	var result models.TorrentInfo
	if err := c.get(ctx, "/torrents/info/"+torrentID, &result); err != nil {
		return nil, fmt.Errorf("failed to get torrent info: %w", err)
	}

	return &result, nil
}

// SelectFiles selects which files to download from a torrent
// fileIDs should be comma-separated file IDs or "all"
func (c *Client) SelectFiles(ctx context.Context, torrentID string, fileIDs string) error {
	data := url.Values{}
	data.Set("files", fileIDs)

	if err := c.post(ctx, "/torrents/selectFiles/"+torrentID, data, nil); err != nil {
		return fmt.Errorf("failed to select files: %w", err)
	}

	return nil
}

// DeleteTorrent removes a torrent from Real-Debrid
func (c *Client) DeleteTorrent(ctx context.Context, torrentID string) error {
	if err := c.delete(ctx, "/torrents/delete/"+torrentID); err != nil {
		return fmt.Errorf("failed to delete torrent: %w", err)
	}

	return nil
}

// GetTorrents lists all torrents
func (c *Client) GetTorrents(ctx context.Context) ([]models.TorrentInfo, error) {
	var result []models.TorrentInfo
	if err := c.get(ctx, "/torrents", &result); err != nil {
		return nil, fmt.Errorf("failed to get torrents: %w", err)
	}

	return result, nil
}
