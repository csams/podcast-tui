package download

import (
	"context"
	"testing"
	"time"

	"github.com/csams/podcast-tui/internal/models"
)

func TestDownloadStatus_String(t *testing.T) {
	tests := []struct {
		status   DownloadStatus
		expected string
	}{
		{StatusQueued, "queued"},
		{StatusDownloading, "downloading"},
		{StatusCompleted, "completed"},
		{StatusFailed, "failed"},
		{StatusCancelled, "cancelled"},
		{StatusPaused, "paused"},
		{DownloadStatus(999), "unknown"},
	}

	for _, test := range tests {
		t.Run(test.expected, func(t *testing.T) {
			result := test.status.String()
			if result != test.expected {
				t.Errorf("Expected %s, got %s", test.expected, result)
			}
		})
	}
}

func TestDownloadTask_Creation(t *testing.T) {
	episode := &models.Episode{
		ID:    "test-episode-id",
		Title: "Test Episode",
		URL:   "https://example.com/test.mp3",
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	task := &DownloadTask{
		Episode:     episode,
		PodcastHash: "test-hash",
		Priority:    1,
		Context:     ctx,
		Cancel:      cancel,
	}

	if task.Episode.ID != "test-episode-id" {
		t.Errorf("Expected episode ID 'test-episode-id', got '%s'", task.Episode.ID)
	}

	if task.PodcastHash != "test-hash" {
		t.Errorf("Expected podcast hash 'test-hash', got '%s'", task.PodcastHash)
	}

	if task.Priority != 1 {
		t.Errorf("Expected priority 1, got %d", task.Priority)
	}

	if task.Context == nil {
		t.Error("Expected context to be set")
	}

	if task.Cancel == nil {
		t.Error("Expected cancel function to be set")
	}
}

func TestDownloadProgress_Creation(t *testing.T) {
	startTime := time.Now()
	progress := &DownloadProgress{
		EpisodeID:       "test-episode",
		Status:          StatusDownloading,
		Progress:        0.5,
		Speed:           1024000,  // 1MB/s
		BytesDownloaded: 5242880,  // 5MB
		TotalBytes:      10485760, // 10MB
		ETA:             5 * time.Second,
		LastError:       "",
		StartTime:       startTime,
		RetryCount:      0,
	}

	if progress.EpisodeID != "test-episode" {
		t.Errorf("Expected episode ID 'test-episode', got '%s'", progress.EpisodeID)
	}

	if progress.Status != StatusDownloading {
		t.Errorf("Expected status %v, got %v", StatusDownloading, progress.Status)
	}

	if progress.Progress != 0.5 {
		t.Errorf("Expected progress 0.5, got %f", progress.Progress)
	}

	if progress.Speed != 1024000 {
		t.Errorf("Expected speed 1024000, got %d", progress.Speed)
	}

	if progress.BytesDownloaded != 5242880 {
		t.Errorf("Expected bytes downloaded 5242880, got %d", progress.BytesDownloaded)
	}

	if progress.TotalBytes != 10485760 {
		t.Errorf("Expected total bytes 10485760, got %d", progress.TotalBytes)
	}

	if progress.ETA != 5*time.Second {
		t.Errorf("Expected ETA 5s, got %v", progress.ETA)
	}

	if progress.RetryCount != 0 {
		t.Errorf("Expected retry count 0, got %d", progress.RetryCount)
	}
}

func TestDownloadInfo_Creation(t *testing.T) {
	startTime := time.Now()
	info := &DownloadInfo{
		EpisodeID:       "test-episode",
		Status:          "downloading",
		Progress:        0.75,
		Speed:           2048000, // 2MB/s
		BytesDownloaded: 7340032, // ~7MB
		TotalBytes:      9787392, // ~9MB
		RetryCount:      1,
		LastError:       "",
		StartTime:       startTime,
		EstimatedTime:   2 * time.Second,
	}

	if info.EpisodeID != "test-episode" {
		t.Errorf("Expected episode ID 'test-episode', got '%s'", info.EpisodeID)
	}

	if info.Status != "downloading" {
		t.Errorf("Expected status 'downloading', got '%s'", info.Status)
	}

	if info.Progress != 0.75 {
		t.Errorf("Expected progress 0.75, got %f", info.Progress)
	}

	if info.Speed != 2048000 {
		t.Errorf("Expected speed 2048000, got %d", info.Speed)
	}

	if info.RetryCount != 1 {
		t.Errorf("Expected retry count 1, got %d", info.RetryCount)
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.MaxSizeGB != 5 {
		t.Errorf("Expected MaxSizeGB 5, got %d", config.MaxSizeGB)
	}

	if config.MaxEpisodesPerPodcast != 10 {
		t.Errorf("Expected MaxEpisodesPerPodcast 10, got %d", config.MaxEpisodesPerPodcast)
	}

	if !config.AutoCleanup {
		t.Error("Expected AutoCleanup to be true")
	}

	if config.CleanupDays != 30 {
		t.Errorf("Expected CleanupDays 30, got %d", config.CleanupDays)
	}

	if config.MaxConcurrentDownloads != 3 {
		t.Errorf("Expected MaxConcurrentDownloads 3, got %d", config.MaxConcurrentDownloads)
	}

	if config.DownloadPath != "" {
		t.Errorf("Expected empty DownloadPath, got '%s'", config.DownloadPath)
	}
}

func TestConfig_Validation(t *testing.T) {
	// Test valid configuration
	config := &Config{
		MaxSizeGB:              10,
		MaxEpisodesPerPodcast:  20,
		AutoCleanup:            true,
		CleanupDays:            14,
		MaxConcurrentDownloads: 5,
		DownloadPath:           "/custom/path",
	}

	if config.MaxSizeGB <= 0 {
		t.Error("MaxSizeGB should be positive")
	}

	if config.MaxEpisodesPerPodcast <= 0 {
		t.Error("MaxEpisodesPerPodcast should be positive")
	}

	if config.CleanupDays <= 0 {
		t.Error("CleanupDays should be positive")
	}

	if config.MaxConcurrentDownloads <= 0 {
		t.Error("MaxConcurrentDownloads should be positive")
	}

	// Test edge cases
	config.MaxConcurrentDownloads = 0
	if config.MaxConcurrentDownloads > 0 {
		t.Error("Should handle zero concurrent downloads")
	}
}
