package storage

import (
	"github.com/ygncode/real-debrid-downloader/internal/models"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateDownload(download *models.Download) error {
	return r.db.Create(download).Error
}

func (r *Repository) GetDownload(id uint) (*models.Download, error) {
	var download models.Download
	if err := r.db.First(&download, id).Error; err != nil {
		return nil, err
	}
	return &download, nil
}

func (r *Repository) GetDownloadByTorrentID(torrentID string) (*models.Download, error) {
	var download models.Download
	if err := r.db.Where("torrent_id = ?", torrentID).First(&download).Error; err != nil {
		return nil, err
	}
	return &download, nil
}

func (r *Repository) GetAllDownloads() ([]models.Download, error) {
	var downloads []models.Download
	if err := r.db.Order("created_at DESC").Find(&downloads).Error; err != nil {
		return nil, err
	}
	return downloads, nil
}

func (r *Repository) GetActiveDownloads() ([]models.Download, error) {
	var downloads []models.Download
	if err := r.db.Where("status NOT IN ?", []models.DownloadStatus{
		models.StatusComplete,
		models.StatusError,
	}).Find(&downloads).Error; err != nil {
		return nil, err
	}
	return downloads, nil
}

func (r *Repository) GetPendingDownloads() ([]models.Download, error) {
	var downloads []models.Download
	if err := r.db.Where("status IN ?", []models.DownloadStatus{
		models.StatusPending,
		models.StatusProcessing,
		models.StatusDownloading,
	}).Find(&downloads).Error; err != nil {
		return nil, err
	}
	return downloads, nil
}

func (r *Repository) UpdateDownload(download *models.Download) error {
	return r.db.Save(download).Error
}

func (r *Repository) UpdateDownloadStatus(id uint, status models.DownloadStatus) error {
	return r.db.Model(&models.Download{}).Where("id = ?", id).Update("status", status).Error
}

func (r *Repository) UpdateDownloadProgress(id uint, progress float64, downloaded int64) error {
	return r.db.Model(&models.Download{}).Where("id = ?", id).Updates(map[string]interface{}{
		"progress":   progress,
		"downloaded": downloaded,
	}).Error
}

func (r *Repository) UpdateDownloadError(id uint, errorMsg string) error {
	return r.db.Model(&models.Download{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":        models.StatusError,
		"error_message": errorMsg,
	}).Error
}

func (r *Repository) DeleteDownload(id uint) error {
	return r.db.Delete(&models.Download{}, id).Error
}
