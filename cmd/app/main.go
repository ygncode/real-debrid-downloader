package main

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/ygncode/real-debrid-downloader/internal/config"
	"github.com/ygncode/real-debrid-downloader/internal/handlers"
	"github.com/ygncode/real-debrid-downloader/internal/realdebrid"
	"github.com/ygncode/real-debrid-downloader/internal/services"
	"github.com/ygncode/real-debrid-downloader/internal/storage"
	"github.com/ygncode/real-debrid-downloader/internal/worker"
	"github.com/ygncode/real-debrid-downloader/web"
)

var (
	moviesPath     string
	port           int
	apiKey         string
	subliminalPath string
	password       string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "rd-downloader",
		Short: "Real-Debrid movie downloader with web interface",
		Long: `RD Downloader is a web application that lists movies from a directory
and allows downloading new movies via Real-Debrid API with automatic subtitle fetching.`,
		Run: runServer,
	}

	rootCmd.Flags().StringVarP(&moviesPath, "path", "p", "", "Path to movies directory (required)")
	rootCmd.Flags().IntVar(&port, "port", 8080, "Port to run the web server on")
	rootCmd.Flags().StringVar(&apiKey, "api-key", "", "Real-Debrid API key (or set REALDEBRID_API_KEY env var)")
	rootCmd.Flags().StringVar(&subliminalPath, "subliminal-path", "", "Path to subliminal binary (e.g., /home/user/miniconda3/bin/subliminal)")
	rootCmd.Flags().StringVar(&password, "password", "", "Password to protect the web interface (optional)")

	rootCmd.MarkFlagRequired("path")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func runServer(cmd *cobra.Command, args []string) {
	// Validate movies path
	info, err := os.Stat(moviesPath)
	if err != nil {
		log.Fatalf("Error accessing movies path: %v", err)
	}
	if !info.IsDir() {
		log.Fatalf("Movies path must be a directory: %s", moviesPath)
	}

	// Get API key from flag or environment
	if apiKey == "" {
		apiKey = os.Getenv("REALDEBRID_API_KEY")
	}
	if apiKey == "" {
		log.Fatal("Real-Debrid API key is required. Set via --api-key flag or REALDEBRID_API_KEY environment variable")
	}

	// Initialize configuration
	cfg := config.New(moviesPath, apiKey, port)

	// Initialize database
	db, err := storage.NewDatabase(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Initialize Real-Debrid client
	rdClient := realdebrid.NewClient(apiKey)

	// Initialize repository
	repo := storage.NewRepository(db)

	// Initialize services
	movieService := services.NewMovieService(cfg.MoviesPath)
	subtitleService := services.NewSubtitleService(subliminalPath)
	downloadService := services.NewDownloadService(repo, rdClient, cfg.MoviesPath, subtitleService)

	// Initialize worker manager
	workerManager := worker.NewManager(downloadService, rdClient, repo, cfg.MoviesPath, subtitleService)
	workerManager.Start()
	defer workerManager.Stop()

	// Resume any pending downloads
	workerManager.ResumePendingDownloads()

	// Initialize and start HTTP server
	server := handlers.NewServer(cfg, movieService, downloadService, repo, workerManager, web.TemplatesFS, web.StaticFS, password)

	log.Printf("Starting RD Downloader server on port %d", cfg.Port)
	log.Printf("Movies directory: %s", cfg.MoviesPath)
	if password != "" {
		log.Println("Password protection: enabled")
	}

	if err := server.Run(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
