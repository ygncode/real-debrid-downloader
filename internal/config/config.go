package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	MoviesPath    string
	APIKey        string
	Port          int
	DBPath        string
	MaxConcurrent int
	PollInterval  int // seconds
}

func New(moviesPath, apiKey string, port int) *Config {
	homeDir, _ := os.UserHomeDir()
	dbDir := filepath.Join(homeDir, ".rd-downloader")
	os.MkdirAll(dbDir, 0755)

	return &Config{
		MoviesPath:    moviesPath,
		APIKey:        apiKey,
		Port:          port,
		DBPath:        filepath.Join(dbDir, "rd-downloader.db"),
		MaxConcurrent: 2,
		PollInterval:  5,
	}
}
