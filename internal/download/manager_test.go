package download

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/csams/podcast-tui/internal/models"
)

func TestNewManager(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)

	if manager.registry == nil {
		t.Error("Expected registry to be initialized")
	}

	if manager.configManager == nil {
		t.Error("Expected config manager to be initialized")
	}

	if manager.storageManager == nil {
		t.Error("Expected storage manager to be initialized")
	}

	if manager.downloader == nil {
		t.Error("Expected downloader to be initialized")
	}

	if manager.queue == nil {
		t.Error("Expected queue to be initialized")
	}

	if manager.activeDownloads == nil {
		t.Error("Expected active downloads map to be initialized")
	}

	if manager.progressCh == nil {
		t.Error("Expected progress channel to be initialized")
	}

	if manager.stopCh == nil {
		t.Error("Expected stop channel to be initialized")
	}

	if manager.configDir != tempDir {
		t.Errorf("Expected config dir '%s', got '%s'", tempDir, manager.configDir)
	}

	if manager.running {
		t.Error("Expected manager to not be running initially")
	}
}

func TestManager_StartStop(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)

	// Test start
	err := manager.Start()
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}

	if !manager.running {
		t.Error("Expected manager to be running after start")
	}

	// Test start when already running
	err = manager.Start()
	if err == nil {
		t.Error("Expected error when starting already running manager")
	}

	// Test stop
	manager.Stop()

	if manager.running {
		t.Error("Expected manager to not be running after stop")
	}

	// Test stop when not running (should not panic)
	manager.Stop()
}

func TestManager_QueueDownload(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)

	err := manager.Start()
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop()

	episode := &models.Episode{
		ID:    "test-episode-1",
		Title: "Test Episode",
		URL:   "https://example.com/test.mp3",
	}

	// Test successful queue
	err = manager.QueueDownload(episode, "Test Podcast")
	if err != nil {
		t.Fatalf("Failed to queue download: %v", err)
	}

	// Verify episode is in registry
	if !manager.registry.IsDownloading("test-episode-1") {
		t.Error("Expected episode to be marked as downloading")
	}

	// Test duplicate queue
	err = manager.QueueDownload(episode, "Test Podcast")
	if err == nil {
		t.Error("Expected error when queueing already downloading episode")
	}
}

func TestManager_QueueDownload_NotRunning(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)

	episode := &models.Episode{
		ID:    "test-episode-1",
		Title: "Test Episode",
		URL:   "https://example.com/test.mp3",
	}

	err := manager.QueueDownload(episode, "Test Podcast")
	if err == nil {
		t.Error("Expected error when queueing download with manager not running")
	}

	if !strings.Contains(err.Error(), "download manager not running") {
		t.Errorf("Expected 'not running' error, got: %v", err)
	}
}

func TestManager_CancelDownload(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)

	err := manager.Start()
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop()

	episode := &models.Episode{
		ID:    "test-episode-1",
		Title: "Test Episode",
		URL:   "https://example.com/test.mp3",
	}

	// Queue download
	err = manager.QueueDownload(episode, "Test Podcast")
	if err != nil {
		t.Fatalf("Failed to queue download: %v", err)
	}

	// Cancel download
	err = manager.CancelDownload("test-episode-1")
	if err != nil {
		t.Fatalf("Failed to cancel download: %v", err)
	}

	// Give some time for cancellation to process
	time.Sleep(100 * time.Millisecond)

	// Verify episode is marked as cancelled
	info, exists := manager.registry.GetDownloadInfo("test-episode-1")
	if !exists {
		t.Error("Expected download info to exist")
	} else if info.Status != "cancelled" {
		t.Errorf("Expected status 'cancelled', got '%s'", info.Status)
	}
}

func TestManager_GetDownloadProgress(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)

	// Test non-existent episode
	progress, exists := manager.GetDownloadProgress("nonexistent")
	if exists {
		t.Error("Expected false for non-existent episode")
	}
	if progress != nil {
		t.Error("Expected nil progress for non-existent episode")
	}

	// Add download info to registry
	info := &DownloadInfo{
		EpisodeID:       "test-episode",
		Status:          "downloading",
		Progress:        0.75,
		Speed:           1024000,
		BytesDownloaded: 7340032,
		TotalBytes:      9787392,
		RetryCount:      1,
		StartTime:       time.Now(),
		EstimatedTime:   2 * time.Second,
	}
	manager.registry.downloads["test-episode"] = info

	// Test existing episode
	progress, exists = manager.GetDownloadProgress("test-episode")
	if !exists {
		t.Error("Expected true for existing episode")
	}
	if progress == nil {
		t.Fatal("Expected non-nil progress")
	}

	if progress.EpisodeID != "test-episode" {
		t.Errorf("Expected episode ID 'test-episode', got '%s'", progress.EpisodeID)
	}
	if progress.Status != StatusDownloading {
		t.Errorf("Expected status %v, got %v", StatusDownloading, progress.Status)
	}
	if progress.Progress != 0.75 {
		t.Errorf("Expected progress 0.75, got %f", progress.Progress)
	}
}

func TestManager_GetAllDownloads(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)

	// Test empty manager
	all := manager.GetAllDownloads()
	if len(all) != 0 {
		t.Errorf("Expected empty map, got %d entries", len(all))
	}

	// Add multiple downloads
	info1 := &DownloadInfo{EpisodeID: "episode1", Status: "downloading", Progress: 0.5}
	info2 := &DownloadInfo{EpisodeID: "episode2", Status: "completed", Progress: 1.0}
	manager.registry.downloads["episode1"] = info1
	manager.registry.downloads["episode2"] = info2

	all = manager.GetAllDownloads()
	if len(all) != 2 {
		t.Errorf("Expected 2 downloads, got %d", len(all))
	}

	// Verify conversion from DownloadInfo to DownloadProgress
	progress1, exists := all["episode1"]
	if !exists {
		t.Error("Episode1 not found in all downloads")
	} else {
		if progress1.Status != StatusDownloading {
			t.Errorf("Expected status %v, got %v", StatusDownloading, progress1.Status)
		}
	}
}

func TestManager_IsDownloaded(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)

	// Test non-existent episode
	if manager.IsDownloaded("nonexistent") {
		t.Error("Expected false for non-existent episode")
	}

	// Test completed download
	manager.registry.downloads["completed"] = &DownloadInfo{Status: "completed"}
	if !manager.IsDownloaded("completed") {
		t.Error("Expected true for completed episode")
	}

	// Test downloading episode
	manager.registry.downloads["downloading"] = &DownloadInfo{Status: "downloading"}
	if manager.IsDownloaded("downloading") {
		t.Error("Expected false for downloading episode")
	}
}

func TestManager_IsDownloading(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)

	// Test non-existent episode
	if manager.IsDownloading("nonexistent") {
		t.Error("Expected false for non-existent episode")
	}

	// Test downloading episode
	manager.registry.downloads["downloading"] = &DownloadInfo{Status: "downloading"}
	if !manager.IsDownloading("downloading") {
		t.Error("Expected true for downloading episode")
	}

	// Test queued episode
	manager.registry.downloads["queued"] = &DownloadInfo{Status: "queued"}
	if !manager.IsDownloading("queued") {
		t.Error("Expected true for queued episode")
	}

	// Test completed episode
	manager.registry.downloads["completed"] = &DownloadInfo{Status: "completed"}
	if manager.IsDownloading("completed") {
		t.Error("Expected false for completed episode")
	}
}

func TestManager_GetProgressChannel(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)

	progressCh := manager.GetProgressChannel()
	if progressCh == nil {
		t.Error("Expected non-nil progress channel")
	}

	// Verify it's a read-only channel
	select {
	case <-progressCh:
		// Should not receive anything initially
		t.Error("Expected empty channel initially")
	default:
		// This is expected
	}
}

func TestManager_Integration_SuccessfulDownload(t *testing.T) {
	// Create test server
	testData := "This is test podcast episode content."
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(testData)))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testData))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	manager := NewManager(tempDir)

	err := manager.Start()
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop()

	episode := &models.Episode{
		ID:    "integration-test-episode",
		Title: "Integration Test Episode",
		URL:   server.URL,
	}

	// Queue download
	err = manager.QueueDownload(episode, "Test Podcast")
	if err != nil {
		t.Fatalf("Failed to queue download: %v", err)
	}

	// Wait for download to complete
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var completed bool
	for !completed {
		select {
		case <-timeout:
			t.Fatal("Download did not complete within timeout")
		case <-ticker.C:
			if manager.IsDownloaded("integration-test-episode") {
				completed = true
			}
		}
	}

	// Verify download completed
	info, exists := manager.registry.GetDownloadInfo("integration-test-episode")
	if !exists {
		t.Fatal("Download info not found")
	}

	if info.Status != "completed" {
		t.Errorf("Expected status 'completed', got '%s'", info.Status)
	}

	if info.Progress != 1.0 {
		t.Errorf("Expected progress 1.0, got %f", info.Progress)
	}

	// Verify file was created
	podcastHash := manager.generatePodcastHash("Test Podcast")
	downloadDir := manager.configManager.GetDownloadDir()
	expectedPath := filepath.Join(downloadDir, podcastHash, "integration-test-episode.mp3")

	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Error("Downloaded file was not created")
	}

	// Verify file content
	content, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("Failed to read downloaded file: %v", err)
	}

	if string(content) != testData {
		t.Errorf("Expected file content '%s', got '%s'", testData, string(content))
	}
}

func TestManager_Integration_FailedDownload(t *testing.T) {
	// Create test server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	manager := NewManager(tempDir)

	err := manager.Start()
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop()

	episode := &models.Episode{
		ID:    "failed-test-episode",
		Title: "Failed Test Episode",
		URL:   server.URL,
	}

	// Queue download
	err = manager.QueueDownload(episode, "Test Podcast")
	if err != nil {
		t.Fatalf("Failed to queue download: %v", err)
	}

	// Wait for download to fail
	timeout := time.After(15 * time.Second) // Longer timeout due to retries
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var failed bool
	for !failed {
		select {
		case <-timeout:
			t.Fatal("Download did not fail within timeout")
		case <-ticker.C:
			info, exists := manager.registry.GetDownloadInfo("failed-test-episode")
			if exists && info.Status == "failed" {
				failed = true
			}
		}
	}

	// Verify download failed
	info, exists := manager.registry.GetDownloadInfo("failed-test-episode")
	if !exists {
		t.Fatal("Download info not found")
	}

	if info.Status != "failed" {
		t.Errorf("Expected status 'failed', got '%s'", info.Status)
	}

	if info.RetryCount == 0 {
		t.Error("Expected retry attempts to be made")
	}

	if info.LastError == "" {
		t.Error("Expected error message to be recorded")
	}
}

func TestManager_ConcurrentDownloads(t *testing.T) {
	// Create test server with delays
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond) // Simulate slow download
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(http.StatusOK)
		w.Write(make([]byte, 100))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	manager := NewManager(tempDir)

	// Set max concurrent downloads to 2 for testing
	config := manager.registry.GetConfig()
	config.MaxConcurrentDownloads = 2
	manager.registry.SetConfig(config)

	err := manager.Start()
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop()

	// Queue multiple downloads
	episodes := []*models.Episode{
		{ID: "episode1", Title: "Episode 1", URL: server.URL},
		{ID: "episode2", Title: "Episode 2", URL: server.URL},
		{ID: "episode3", Title: "Episode 3", URL: server.URL},
		{ID: "episode4", Title: "Episode 4", URL: server.URL},
	}

	for _, episode := range episodes {
		err = manager.QueueDownload(episode, "Test Podcast")
		if err != nil {
			t.Fatalf("Failed to queue download for %s: %v", episode.ID, err)
		}
	}

	// Wait for some downloads to start
	time.Sleep(300 * time.Millisecond)

	// Count active downloads
	manager.mu.RLock()
	activeCount := len(manager.activeDownloads)
	manager.mu.RUnlock()

	if activeCount > 2 {
		t.Errorf("Expected at most 2 active downloads, got %d", activeCount)
	}

	// Wait for all downloads to complete
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Fatal("Not all downloads completed within timeout")
		case <-ticker.C:
			completed := 0
			for _, episode := range episodes {
				if manager.IsDownloaded(episode.ID) {
					completed++
				}
			}
			if completed == len(episodes) {
				return // All downloads completed
			}
		}
	}
}

func TestManager_GeneratePodcastDirectory(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)

	// Test consistent directory naming
	dir1 := manager.GeneratePodcastDirectory("Test Podcast")
	dir2 := manager.GeneratePodcastDirectory("Test Podcast")

	if dir1 != dir2 {
		t.Error("Expected consistent directory name for same podcast title")
	}

	// Test different titles produce different directory names
	dir3 := manager.GeneratePodcastDirectory("Different Podcast")
	if dir1 == dir3 {
		t.Error("Expected different directory names for different podcast titles")
	}

	// Test whitespace handling
	dir4 := manager.GeneratePodcastDirectory("  Test Podcast  ")
	if dir1 != dir4 {
		t.Error("Expected same directory name for trimmed title")
	}

	// Test sanitization
	dir5 := manager.GeneratePodcastDirectory("Test: Podcast!")
	expected := "Test__Podcast_"
	if dir5 != expected {
		t.Errorf("Expected sanitized directory name '%s', got '%s'", expected, dir5)
	}
}

func TestManager_GenerateFilename(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)

	episode := &models.Episode{
		ID:    "test-episode-id",
		Title: "My Test Episode!",
	}

	filename := manager.GenerateFilename(episode)
	expected := "My_Test_Episode.mp3"

	if filename != expected {
		t.Errorf("Expected filename '%s', got '%s'", expected, filename)
	}

	// Test with ID-only fallback
	filename2 := manager.generateFilenameForID("test-episode-id")
	expected2 := "test-episode-id.mp3"

	if filename2 != expected2 {
		t.Errorf("Expected filename '%s', got '%s'", expected2, filename2)
	}
}

func TestManager_ParseStatus(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)

	tests := []struct {
		status   string
		expected DownloadStatus
	}{
		{"queued", StatusQueued},
		{"downloading", StatusDownloading},
		{"completed", StatusCompleted},
		{"failed", StatusFailed},
		{"cancelled", StatusCancelled},
		{"paused", StatusPaused},
		{"unknown", StatusFailed}, // Default for unknown
		{"", StatusFailed},        // Default for empty
	}

	for _, test := range tests {
		result := manager.parseStatus(test.status)
		if result != test.expected {
			t.Errorf("For status '%s', expected %v, got %v", test.status, test.expected, result)
		}
	}
}

func TestManager_ProgressReporting(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)

	err := manager.Start()
	if err != nil {
		t.Fatalf("Failed to start manager: %v", err)
	}
	defer manager.Stop()

	// Get progress channel
	progressCh := manager.GetProgressChannel()

	// Create a test progress update
	progress := &DownloadProgress{
		EpisodeID: "test-progress",
		Status:    StatusDownloading,
		Progress:  0.5,
		Speed:     1024000,
	}

	// Send progress update
	go func() {
		time.Sleep(50 * time.Millisecond)
		select {
		case manager.progressCh <- progress:
		default:
			t.Error("Failed to send progress update")
		}
	}()

	// Receive progress update
	select {
	case receivedProgress := <-progressCh:
		if receivedProgress.EpisodeID != "test-progress" {
			t.Errorf("Expected episode ID 'test-progress', got '%s'", receivedProgress.EpisodeID)
		}
		if receivedProgress.Status != StatusDownloading {
			t.Errorf("Expected status %v, got %v", StatusDownloading, receivedProgress.Status)
		}
	case <-time.After(1 * time.Second):
		t.Error("Did not receive progress update within timeout")
	}
}
