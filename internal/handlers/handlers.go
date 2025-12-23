package handlers

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/ygncode/real-debrid-downloader/internal/config"
	"github.com/ygncode/real-debrid-downloader/internal/services"
	"github.com/ygncode/real-debrid-downloader/internal/storage"
	"github.com/ygncode/real-debrid-downloader/internal/worker"
)

type Server struct {
	config          *config.Config
	movieService    *services.MovieService
	downloadService *services.DownloadService
	repo            *storage.Repository
	workerManager   *worker.Manager
	router          *gin.Engine
	password        string
	sessions        map[string]bool
	sessionMu       sync.RWMutex
}

func NewServer(
	cfg *config.Config,
	movieService *services.MovieService,
	downloadService *services.DownloadService,
	repo *storage.Repository,
	workerManager *worker.Manager,
	templatesFS embed.FS,
	staticFS embed.FS,
	password string,
) *Server {
	gin.SetMode(gin.ReleaseMode)

	s := &Server{
		config:          cfg,
		movieService:    movieService,
		downloadService: downloadService,
		repo:            repo,
		workerManager:   workerManager,
		router:          gin.Default(),
		password:        password,
		sessions:        make(map[string]bool),
	}

	s.setupRoutes(templatesFS, staticFS)
	return s
}

func (s *Server) setupRoutes(templatesFS embed.FS, staticFS embed.FS) {
	// Load templates
	tmpl := template.Must(template.New("").Funcs(template.FuncMap{
		"formatBytes": formatBytes,
		"formatProgress": func(p float64) string {
			return fmt.Sprintf("%.1f", p)
		},
	}).ParseFS(templatesFS, "templates/*.html", "templates/**/*.html"))
	s.router.SetHTMLTemplate(tmpl)

	// Serve static files
	staticSub, _ := fs.Sub(staticFS, "static")
	s.router.StaticFS("/static", http.FS(staticSub))

	// Public routes (login)
	s.router.GET("/login", s.handleLoginPage)
	s.router.POST("/login", s.handleLogin)
	s.router.GET("/logout", s.handleLogout)

	// Protected routes
	protected := s.router.Group("/")
	protected.Use(s.authMiddleware())
	{
		protected.GET("/", s.handleIndex)
	}

	api := s.router.Group("/api")
	api.Use(s.authMiddleware())
	{
		api.GET("/movies", s.handleListMovies)
		api.DELETE("/movies", s.handleDeleteFile)
		api.POST("/torrents/magnet", s.handleAddMagnet)
		api.POST("/torrents/file", s.handleAddTorrentFile)
		api.GET("/downloads", s.handleListDownloads)
		api.GET("/downloads/:id/files", s.handleGetDownloadFiles)
		api.POST("/downloads/:id/select", s.handleSelectFiles)
		api.DELETE("/downloads/:id", s.handleDeleteDownload)
		api.GET("/downloads/stream", s.handleSSE)
	}
}

// authMiddleware checks if the user is authenticated
func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// If no password is set, allow all requests
		if s.password == "" {
			c.Next()
			return
		}

		// Check for session cookie
		sessionID, err := c.Cookie("session")
		if err != nil || sessionID == "" {
			s.redirectToLogin(c)
			return
		}

		// Validate session
		s.sessionMu.RLock()
		valid := s.sessions[sessionID]
		s.sessionMu.RUnlock()

		if !valid {
			s.redirectToLogin(c)
			return
		}

		c.Next()
	}
}

func (s *Server) redirectToLogin(c *gin.Context) {
	// For API requests, return 401
	if len(c.Request.URL.Path) >= 4 && c.Request.URL.Path[:4] == "/api" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		c.Abort()
		return
	}

	// For page requests, redirect to login
	c.Redirect(http.StatusFound, "/login")
	c.Abort()
}

func (s *Server) handleLoginPage(c *gin.Context) {
	// If no password is set, redirect to home
	if s.password == "" {
		c.Redirect(http.StatusFound, "/")
		return
	}

	// If already logged in, redirect to home
	if sessionID, err := c.Cookie("session"); err == nil {
		s.sessionMu.RLock()
		valid := s.sessions[sessionID]
		s.sessionMu.RUnlock()
		if valid {
			c.Redirect(http.StatusFound, "/")
			return
		}
	}

	c.HTML(http.StatusOK, "login.html", gin.H{
		"error": c.Query("error"),
	})
}

func (s *Server) handleLogin(c *gin.Context) {
	password := c.PostForm("password")

	if password != s.password {
		c.Redirect(http.StatusFound, "/login?error=Invalid+password")
		return
	}

	// Create session
	sessionID := generateSessionID()
	s.sessionMu.Lock()
	s.sessions[sessionID] = true
	s.sessionMu.Unlock()

	// Set cookie (expires in 30 days)
	c.SetCookie("session", sessionID, 60*60*24*30, "/", "", false, true)
	c.Redirect(http.StatusFound, "/")
}

func (s *Server) handleLogout(c *gin.Context) {
	if sessionID, err := c.Cookie("session"); err == nil {
		s.sessionMu.Lock()
		delete(s.sessions, sessionID)
		s.sessionMu.Unlock()
	}

	c.SetCookie("session", "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/login")
}

func generateSessionID() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Server) Run() error {
	return s.router.Run(fmt.Sprintf(":%d", s.config.Port))
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
