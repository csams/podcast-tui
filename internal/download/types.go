package download

import (
	"context"
	"time"

	"github.com/csams/podcast-tui/internal/models"
)

// DownloadStatus represents the current state of a download
type DownloadStatus int

const (
	StatusQueued DownloadStatus = iota
	StatusDownloading
	StatusCompleted
	StatusFailed
	StatusCancelled
	StatusPaused
)

func (s DownloadStatus) String() string {
	switch s {
	case StatusQueued:
		return "queued"
	case StatusDownloading:
		return "downloading"
	case StatusCompleted:
		return "completed"
	case StatusFailed:
		return "failed"
	case StatusCancelled:
		return "cancelled"
	case StatusPaused:
		return "paused"
	default:
		return "unknown"
	}
}

// DownloadTask represents a download request
type DownloadTask struct {
	Episode     *models.Episode
	PodcastHash string
	Priority    int
	Context     context.Context
	Cancel      context.CancelFunc
}

// DownloadProgress represents current download progress
type DownloadProgress struct {
	EpisodeID       string
	Status          DownloadStatus
	Progress        float64 // 0.0 to 1.0
	Speed           int64   // bytes per second
	BytesDownloaded int64
	TotalBytes      int64
	ETA             time.Duration
	LastError       string
	StartTime       time.Time
	RetryCount      int
}

// DownloadInfo represents persisted download information
type DownloadInfo struct {
	EpisodeID       string        `json:"episodeId"`
	Status          string        `json:"status"`
	Progress        float64       `json:"progress"`
	Speed           int64         `json:"speed"`
	BytesDownloaded int64         `json:"bytesDownloaded"`
	TotalBytes      int64         `json:"totalBytes"`
	RetryCount      int           `json:"retryCount"`
	LastError       string        `json:"lastError,omitempty"`
	StartTime       time.Time     `json:"startTime"`
	EstimatedTime   time.Duration `json:"estimatedTime"`
}

// Config represents download configuration
type Config struct {
	MaxSizeGB              int    `json:"maxSizeGB"`
	MaxEpisodesPerPodcast  int    `json:"maxEpisodesPerPodcast"`
	AutoCleanup            bool   `json:"autoCleanup"`
	CleanupDays            int    `json:"cleanupDays"`
	MaxConcurrentDownloads int    `json:"maxConcurrentDownloads"`
	DownloadPath           string `json:"downloadPath"`
}

// DefaultConfig returns the default download configuration
func DefaultConfig() *Config {
	return &Config{
		MaxSizeGB:              5,
		MaxEpisodesPerPodcast:  10,
		AutoCleanup:            true,
		CleanupDays:            30,
		MaxConcurrentDownloads: 3,
		DownloadPath:           "", // Will be set to ~/Music/Podcasts
	}
}
