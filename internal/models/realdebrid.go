package models

// AddTorrentResponse is returned when adding a magnet or torrent file
type AddTorrentResponse struct {
	ID  string `json:"id"`
	URI string `json:"uri"`
}

// TorrentInfo contains detailed information about a torrent
type TorrentInfo struct {
	ID               string        `json:"id"`
	Filename         string        `json:"filename"`
	OriginalFilename string        `json:"original_filename"`
	Hash             string        `json:"hash"`
	Bytes            int64         `json:"bytes"`
	OriginalBytes    int64         `json:"original_bytes"`
	Host             string        `json:"host"`
	Split            int           `json:"split"`
	Progress         float64       `json:"progress"`
	Status           string        `json:"status"`
	Added            string        `json:"added"`
	Files            []TorrentFile `json:"files"`
	Links            []string      `json:"links"`
	Ended            string        `json:"ended,omitempty"`
	Speed            int64         `json:"speed,omitempty"`
	Seeders          int           `json:"seeders,omitempty"`
}

// UnrestrictedLink is returned when unrestricting a link
type UnrestrictedLink struct {
	ID         string `json:"id"`
	Filename   string `json:"filename"`
	MimeType   string `json:"mimeType"`
	Filesize   int64  `json:"filesize"`
	Link       string `json:"link"`
	Host       string `json:"host"`
	Chunks     int    `json:"chunks"`
	CRC        int    `json:"crc"`
	Download   string `json:"download"`
	Streamable int    `json:"streamable"`
}

// APIError represents an error response from Real-Debrid
type APIError struct {
	Error     string `json:"error"`
	ErrorCode int    `json:"error_code,omitempty"`
}

// Torrent status constants from Real-Debrid API
const (
	RDStatusMagnetError          = "magnet_error"
	RDStatusMagnetConversion     = "magnet_conversion"
	RDStatusWaitingFilesSelection = "waiting_files_selection"
	RDStatusQueued               = "queued"
	RDStatusDownloading          = "downloading"
	RDStatusDownloaded           = "downloaded"
	RDStatusError                = "error"
	RDStatusVirus                = "virus"
	RDStatusCompressing          = "compressing"
	RDStatusUploading            = "uploading"
	RDStatusDead                 = "dead"
)
