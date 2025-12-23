package services

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ygncode/real-debrid-downloader/internal/models"
)

var videoExtensions = map[string]bool{
	".mp4":  true,
	".mkv":  true,
	".avi":  true,
	".mov":  true,
	".wmv":  true,
	".flv":  true,
	".webm": true,
	".m4v":  true,
	".ts":   true,
	".m2ts": true,
}

var subtitleExtensions = map[string]bool{
	".srt": true,
	".sub": true,
	".ass": true,
	".ssa": true,
	".vtt": true,
}

var mediaExtensions = map[string]bool{
	// Video
	".mp4": true, ".mkv": true, ".avi": true, ".mov": true,
	".wmv": true, ".flv": true, ".webm": true, ".m4v": true,
	".ts": true, ".m2ts": true,
	// Subtitles
	".srt": true, ".sub": true, ".ass": true, ".ssa": true, ".vtt": true,
	// Other media files
	".nfo": true, ".txt": true, ".jpg": true, ".jpeg": true, ".png": true,
}

type MovieService struct {
	moviesPath string
}

func NewMovieService(moviesPath string) *MovieService {
	return &MovieService{moviesPath: moviesPath}
}

func (s *MovieService) ListMovies() ([]models.Movie, error) {
	var movies []models.Movie

	err := filepath.Walk(s.moviesPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Skip the root directory itself
		if path == s.moviesPath {
			return nil
		}

		// Get relative path from movies directory
		relPath, _ := filepath.Rel(s.moviesPath, path)

		if info.IsDir() {
			// Check if directory contains any media files
			hasMedia := false
			filepath.Walk(path, func(subPath string, subInfo os.FileInfo, subErr error) error {
				if subErr == nil && !subInfo.IsDir() && isMediaFile(subPath) {
					hasMedia = true
					return filepath.SkipDir
				}
				return nil
			})

			if hasMedia {
				// Calculate total size of folder
				var totalSize int64
				filepath.Walk(path, func(subPath string, subInfo os.FileInfo, subErr error) error {
					if subErr == nil && !subInfo.IsDir() {
						totalSize += subInfo.Size()
					}
					return nil
				})

				movies = append(movies, models.Movie{
					Name:     info.Name(),
					Path:     relPath,
					Size:     totalSize,
					ModTime:  info.ModTime(),
					IsFolder: true,
				})
				return filepath.SkipDir // Don't recurse into movie folders
			}
		} else if isMediaFile(path) {
			movies = append(movies, models.Movie{
				Name:     info.Name(),
				Path:     relPath,
				Size:     info.Size(),
				ModTime:  info.ModTime(),
				IsFolder: false,
				FileType: getFileType(path),
			})
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort by modification time (newest first)
	sort.Slice(movies, func(i, j int) bool {
		return movies[i].ModTime.After(movies[j].ModTime)
	})

	return movies, nil
}

func isVideo(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return videoExtensions[ext]
}

func isSubtitle(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return subtitleExtensions[ext]
}

func isMediaFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return mediaExtensions[ext]
}

func getFileType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if videoExtensions[ext] {
		return "video"
	}
	if subtitleExtensions[ext] {
		return "subtitle"
	}
	return "other"
}

func (s *MovieService) GetMoviesPath() string {
	return s.moviesPath
}

// DeleteFile deletes a file or folder from the movies directory
func (s *MovieService) DeleteFile(relativePath string) error {
	// Sanitize the path to prevent directory traversal
	cleanPath := filepath.Clean(relativePath)
	if strings.HasPrefix(cleanPath, "..") {
		return fmt.Errorf("invalid path")
	}

	fullPath := filepath.Join(s.moviesPath, cleanPath)

	// Verify the path is still within the movies directory
	if !strings.HasPrefix(fullPath, s.moviesPath) {
		return fmt.Errorf("invalid path")
	}

	// Check if file/folder exists
	info, err := os.Stat(fullPath)
	if err != nil {
		return fmt.Errorf("file not found: %w", err)
	}

	if info.IsDir() {
		return os.RemoveAll(fullPath)
	}
	return os.Remove(fullPath)
}
