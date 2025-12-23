package models

import (
	"time"
)

type DownloadStatus string

const (
	StatusPending           DownloadStatus = "pending"
	StatusAwaitingSelection DownloadStatus = "awaiting_selection"
	StatusProcessing        DownloadStatus = "processing"
	StatusDownloading       DownloadStatus = "downloading"
	StatusSubtitles         DownloadStatus = "subtitles"
	StatusComplete          DownloadStatus = "complete"
	StatusError             DownloadStatus = "error"
)

type Download struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	TorrentID       string         `gorm:"uniqueIndex" json:"torrent_id"`
	Name            string         `json:"name"`
	Status          DownloadStatus `gorm:"default:pending" json:"status"`
	Progress        float64        `gorm:"default:0" json:"progress"`
	ErrorMessage    string         `json:"error_message,omitempty"`
	Links           string         `json:"links,omitempty"`        // JSON array of unrestricted links
	FilesJSON       string         `json:"files_json,omitempty"`   // JSON array of torrent files for selection
	SelectedIDs     string         `json:"selected_ids,omitempty"` // Comma-separated selected file IDs
	FilePaths       string         `json:"file_paths,omitempty"`   // JSON array of downloaded file paths
	TotalSize       int64          `json:"total_size"`             // Total size in bytes
	Downloaded      int64          `json:"downloaded"`             // Downloaded bytes
	DownloadSubs    bool           `gorm:"default:true" json:"download_subs"` // Whether to download subtitles
	SubtitleStatus  string         `json:"subtitle_status,omitempty"`         // Status of subtitle download
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

type TorrentFile struct {
	ID       int    `json:"id"`
	Path     string `json:"path"`
	Bytes    int64  `json:"bytes"`
	Selected int    `json:"selected"` // 0 or 1 from Real-Debrid API
}

// IsSelected returns true if the file is selected
func (f TorrentFile) IsSelected() bool {
	return f.Selected == 1
}
