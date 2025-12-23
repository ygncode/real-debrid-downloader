package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) handleIndex(c *gin.Context) {
	movies, err := s.movieService.ListMovies()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "index.html", gin.H{
			"error":      err.Error(),
			"moviesPath": s.config.MoviesPath,
		})
		return
	}

	downloads, _ := s.downloadService.GetAllDownloads()

	c.HTML(http.StatusOK, "index.html", gin.H{
		"movies":     movies,
		"downloads":  downloads,
		"moviesPath": s.config.MoviesPath,
	})
}

func (s *Server) handleListMovies(c *gin.Context) {
	movies, err := s.movieService.ListMovies()
	if err != nil {
		c.HTML(http.StatusInternalServerError, "components/movie_list.html", gin.H{
			"error": err.Error(),
		})
		return
	}

	c.HTML(http.StatusOK, "components/movie_list.html", gin.H{
		"movies": movies,
	})
}

type DeleteFileRequest struct {
	Path string `json:"path" binding:"required"`
}

func (s *Server) handleDeleteFile(c *gin.Context) {
	var req DeleteFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Path is required"})
		return
	}

	if err := s.movieService.DeleteFile(req.Path); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}
