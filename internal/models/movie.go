package models

import "time"

type Movie struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	ModTime  time.Time `json:"mod_time"`
	IsFolder bool      `json:"is_folder"`
	FileType string    `json:"file_type,omitempty"` // "video", "subtitle", "other"
}
