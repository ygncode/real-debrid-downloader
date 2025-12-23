package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type AddMagnetRequest struct {
	Magnet       string `json:"magnet" binding:"required"`
	DownloadSubs *bool  `json:"download_subs"` // Pointer to distinguish between false and not provided
}

func (s *Server) handleAddMagnet(c *gin.Context) {
	var req AddMagnetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: magnet link required"})
		return
	}

	// Default to true if not provided
	downloadSubs := true
	if req.DownloadSubs != nil {
		downloadSubs = *req.DownloadSubs
	}

	download, err := s.downloadService.AddMagnet(c.Request.Context(), req.Magnet, downloadSubs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Queue the download for processing
	s.workerManager.QueueDownload(download)

	c.JSON(http.StatusOK, gin.H{
		"id":     download.ID,
		"status": download.Status,
		"name":   download.Name,
	})
}

func (s *Server) handleAddTorrentFile(c *gin.Context) {
	file, header, err := c.Request.FormFile("torrent")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No torrent file provided"})
		return
	}
	defer file.Close()

	// Check for download_subs form field, default to true
	downloadSubs := true
	if c.PostForm("download_subs") == "false" {
		downloadSubs = false
	}

	download, err := s.downloadService.AddTorrent(c.Request.Context(), header.Filename, file, downloadSubs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Queue the download for processing
	s.workerManager.QueueDownload(download)

	c.JSON(http.StatusOK, gin.H{
		"id":     download.ID,
		"status": download.Status,
		"name":   download.Name,
	})
}
