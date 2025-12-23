package worker

import (
	"context"
	"log"
	"sync"

	"github.com/ygncode/real-debrid-downloader/internal/models"
	"github.com/ygncode/real-debrid-downloader/internal/realdebrid"
	"github.com/ygncode/real-debrid-downloader/internal/services"
	"github.com/ygncode/real-debrid-downloader/internal/storage"
)

type Manager struct {
	downloadService *services.DownloadService
	rdClient        *realdebrid.Client
	repo            *storage.Repository
	moviesPath      string
	subtitleService *services.SubtitleService

	jobs      chan *models.Download
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	maxWorkers int

	// SSE broadcast channel
	updates     chan *models.Download
	subscribers map[chan *models.Download]bool
	subMutex    sync.RWMutex
}

func NewManager(
	downloadService *services.DownloadService,
	rdClient *realdebrid.Client,
	repo *storage.Repository,
	moviesPath string,
	subtitleService *services.SubtitleService,
) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	return &Manager{
		downloadService: downloadService,
		rdClient:        rdClient,
		repo:            repo,
		moviesPath:      moviesPath,
		subtitleService: subtitleService,
		jobs:            make(chan *models.Download, 100),
		ctx:             ctx,
		cancel:          cancel,
		maxWorkers:      2,
		updates:         make(chan *models.Download, 100),
		subscribers:     make(map[chan *models.Download]bool),
	}
}

func (m *Manager) Start() {
	// Start worker goroutines
	for i := 0; i < m.maxWorkers; i++ {
		m.wg.Add(1)
		go m.worker(i)
	}

	// Start broadcast goroutine
	go m.broadcaster()

	log.Printf("Worker manager started with %d workers", m.maxWorkers)
}

func (m *Manager) Stop() {
	log.Println("Stopping worker manager...")
	m.cancel()
	close(m.jobs)
	m.wg.Wait()
	close(m.updates)
	log.Println("Worker manager stopped")
}

func (m *Manager) worker(id int) {
	defer m.wg.Done()
	log.Printf("Worker %d started", id)

	for {
		select {
		case <-m.ctx.Done():
			log.Printf("Worker %d shutting down", id)
			return
		case download, ok := <-m.jobs:
			if !ok {
				return
			}
			m.processDownload(download)
		}
	}
}

func (m *Manager) broadcaster() {
	for update := range m.updates {
		m.subMutex.RLock()
		for sub := range m.subscribers {
			select {
			case sub <- update:
			default:
				// Skip if subscriber is not ready
			}
		}
		m.subMutex.RUnlock()
	}
}

// Subscribe returns a channel for receiving download updates
func (m *Manager) Subscribe() chan *models.Download {
	ch := make(chan *models.Download, 10)
	m.subMutex.Lock()
	m.subscribers[ch] = true
	m.subMutex.Unlock()
	return ch
}

// Unsubscribe removes a subscriber
func (m *Manager) Unsubscribe(ch chan *models.Download) {
	m.subMutex.Lock()
	delete(m.subscribers, ch)
	m.subMutex.Unlock()
	close(ch)
}

// Broadcast sends an update to all subscribers
func (m *Manager) Broadcast(download *models.Download) {
	select {
	case m.updates <- download:
	default:
		// Skip if updates channel is full
	}
}

// QueueDownload adds a download to the processing queue
func (m *Manager) QueueDownload(download *models.Download) {
	select {
	case m.jobs <- download:
		log.Printf("Queued download: %s", download.Name)
	default:
		log.Printf("Warning: job queue full, could not queue download: %s", download.Name)
	}
}

// ResumePendingDownloads queues any pending downloads from the database
func (m *Manager) ResumePendingDownloads() {
	downloads, err := m.repo.GetPendingDownloads()
	if err != nil {
		log.Printf("Error getting pending downloads: %v", err)
		return
	}

	for _, download := range downloads {
		d := download // Create a copy for the goroutine
		m.QueueDownload(&d)
	}

	if len(downloads) > 0 {
		log.Printf("Resumed %d pending downloads", len(downloads))
	}
}
