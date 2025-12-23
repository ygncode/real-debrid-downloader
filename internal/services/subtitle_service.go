package services

import (
	"context"
	"log"
	"os/exec"
	"time"
)

type SubtitleService struct {
	available      bool
	subliminalPath string
}

func NewSubtitleService(customPath string) *SubtitleService {
	var subliminalPath string

	// If custom path is provided, use it
	if customPath != "" {
		subliminalPath = customPath
		// Verify the custom path exists and is executable
		if _, err := exec.LookPath(customPath); err != nil {
			log.Printf("Warning: subliminal not found at custom path %s: %v", customPath, err)
			log.Println("Subtitle downloads will be skipped.")
			return &SubtitleService{available: false, subliminalPath: ""}
		}
		log.Printf("Using subliminal at: %s", subliminalPath)
		return &SubtitleService{available: true, subliminalPath: subliminalPath}
	}

	// Check if subliminal is available in PATH
	path, err := exec.LookPath("subliminal")
	if err != nil {
		log.Println("Warning: subliminal not found in PATH. Subtitle downloads will be skipped.")
		log.Println("To enable subtitles, install subliminal: pip install subliminal")
		log.Println("Or specify the path with --subliminal-path flag")
		return &SubtitleService{available: false, subliminalPath: ""}
	}

	log.Printf("Using subliminal at: %s", path)
	return &SubtitleService{available: true, subliminalPath: path}
}

func (s *SubtitleService) IsAvailable() bool {
	return s.available
}

func (s *SubtitleService) DownloadSubtitles(videoPath string) error {
	if !s.available {
		log.Printf("Skipping subtitle download for %s (subliminal not available)", videoPath)
		return nil
	}

	log.Printf("Downloading subtitles for: %s", videoPath)

	// Create a context with timeout (2 minutes should be enough for subtitle search)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Run subliminal to download English subtitles
	cmd := exec.CommandContext(ctx, s.subliminalPath, "download", "-l", "en", videoPath)
	output, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		log.Printf("Subliminal timeout for %s", videoPath)
		return ctx.Err()
	}

	if err != nil {
		log.Printf("Subliminal error: %v\nOutput: %s", err, string(output))
		return err
	}

	log.Printf("Subliminal output: %s", string(output))
	return nil
}
