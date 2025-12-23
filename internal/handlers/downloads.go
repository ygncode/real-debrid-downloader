package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ygncode/real-debrid-downloader/internal/models"
)

func (s *Server) handleListDownloads(c *gin.Context) {
	downloads, err := s.downloadService.GetAllDownloads()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "components/download_table.html", gin.H{
			"error": err.Error(),
		})
		return
	}

	c.HTML(http.StatusOK, "components/download_table.html", gin.H{
		"downloads": downloads,
	})
}

func (s *Server) handleGetDownloadFiles(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid download ID"})
		return
	}

	files, err := s.downloadService.GetDownloadFiles(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	download, _ := s.downloadService.GetDownload(uint(id))

	c.HTML(http.StatusOK, "components/file_selection.html", gin.H{
		"files":      files,
		"downloadID": id,
		"download":   download,
	})
}

type SelectFilesRequest struct {
	FileIDs string `json:"file_ids" binding:"required"`
}

func (s *Server) handleSelectFiles(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid download ID"})
		return
	}

	var req SelectFilesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: file_ids required"})
		return
	}

	if err := s.downloadService.SelectFiles(c.Request.Context(), uint(id), req.FileIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get the updated download and queue it for processing
	download, _ := s.downloadService.GetDownload(uint(id))
	if download != nil {
		s.workerManager.QueueDownload(download)
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleDeleteDownload(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid download ID"})
		return
	}

	if err := s.downloadService.DeleteDownload(c.Request.Context(), uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (s *Server) handleSSE(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	// Subscribe to updates
	updates := s.workerManager.Subscribe()
	defer s.workerManager.Unsubscribe(updates)

	// Send initial downloads state
	downloads, _ := s.downloadService.GetAllDownloads()
	for _, d := range downloads {
		sendDownloadEvent(c, &d)
	}

	// Keep connection alive and send updates
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	clientGone := c.Request.Context().Done()

	for {
		select {
		case <-clientGone:
			return
		case download := <-updates:
			sendDownloadEvent(c, download)
		case <-ticker.C:
			// Send keepalive
			fmt.Fprintf(c.Writer, ": keepalive\n\n")
			c.Writer.Flush()
		}
	}
}

func sendDownloadEvent(c *gin.Context, download *models.Download) {
	data, _ := json.Marshal(download)
	fmt.Fprintf(c.Writer, "event: download\n")
	fmt.Fprintf(c.Writer, "data: %s\n\n", data)
	c.Writer.Flush()

	// If download is complete, send a refresh-movies event
	if download.Status == models.StatusComplete {
		fmt.Fprintf(c.Writer, "event: refresh-movies\n")
		fmt.Fprintf(c.Writer, "data: {}\n\n")
		c.Writer.Flush()
	}
}
