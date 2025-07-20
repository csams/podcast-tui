package download

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewRegistry(t *testing.T) {
	tempDir := t.TempDir()
	registry := NewRegistry(tempDir)

	expectedPath := filepath.Join(tempDir, "downloads", "registry.json")
	if registry.registryPath != expectedPath {
		t.Errorf("Expected registry path '%s', got '%s'", expectedPath, registry.registryPath)
	}

	if registry.downloads == nil {
		t.Error("Expected downloads map to be initialized")
	}

	if registry.config == nil {
		t.Error("Expected config to be initialized")
	}

	if len(registry.downloads) != 0 {
		t.Errorf("Expected empty downloads map, got %d entries", len(registry.downloads))
	}
}

func TestRegistry_SaveAndLoad(t *testing.T) {
	tempDir := t.TempDir()
	registry := NewRegistry(tempDir)

	// Add some download info
	info1 := &DownloadInfo{
		EpisodeID:       "episode1",
		Status:          "downloading",
		Progress:        0.5,
		Speed:           1024000,
		BytesDownloaded: 5242880,
		TotalBytes:      10485760,
		RetryCount:      1,
		StartTime:       time.Now(),
		EstimatedTime:   5 * time.Second,
	}

	info2 := &DownloadInfo{
		EpisodeID:       "episode2",
		Status:          "completed",
		Progress:        1.0,
		Speed:           0,
		BytesDownloaded: 20971520,
		TotalBytes:      20971520,
		RetryCount:      0,
		StartTime:       time.Now().Add(-10 * time.Minute),
		EstimatedTime:   0,
	}

	registry.downloads["episode1"] = info1
	registry.downloads["episode2"] = info2

	// Modify config
	registry.config.MaxSizeGB = 10
	registry.config.MaxConcurrentDownloads = 5

	// Save registry
	err := registry.Save()
	if err != nil {
		t.Fatalf("Failed to save registry: %v", err)
	}

	// Verify file exists
	registryPath := filepath.Join(tempDir, "downloads", "registry.json")
	if _, err := os.Stat(registryPath); os.IsNotExist(err) {
		t.Fatal("Registry file was not created")
	}

	// Create new registry and load
	registry2 := NewRegistry(tempDir)
	err = registry2.Load()
	if err != nil {
		t.Fatalf("Failed to load registry: %v", err)
	}

	// Verify downloads were loaded
	if len(registry2.downloads) != 2 {
		t.Errorf("Expected 2 downloads, got %d", len(registry2.downloads))
	}

	loadedInfo1, exists := registry2.downloads["episode1"]
	if !exists {
		t.Error("Episode1 download info not found")
	} else {
		if loadedInfo1.Status != "downloading" {
			t.Errorf("Expected status 'downloading', got '%s'", loadedInfo1.Status)
		}
		if loadedInfo1.Progress != 0.5 {
			t.Errorf("Expected progress 0.5, got %f", loadedInfo1.Progress)
		}
		if loadedInfo1.RetryCount != 1 {
			t.Errorf("Expected retry count 1, got %d", loadedInfo1.RetryCount)
		}
	}

	loadedInfo2, exists := registry2.downloads["episode2"]
	if !exists {
		t.Error("Episode2 download info not found")
	} else {
		if loadedInfo2.Status != "completed" {
			t.Errorf("Expected status 'completed', got '%s'", loadedInfo2.Status)
		}
		if loadedInfo2.Progress != 1.0 {
			t.Errorf("Expected progress 1.0, got %f", loadedInfo2.Progress)
		}
	}

	// Verify config was loaded
	if registry2.config.MaxSizeGB != 10 {
		t.Errorf("Expected MaxSizeGB 10, got %d", registry2.config.MaxSizeGB)
	}
	if registry2.config.MaxConcurrentDownloads != 5 {
		t.Errorf("Expected MaxConcurrentDownloads 5, got %d", registry2.config.MaxConcurrentDownloads)
	}
}

func TestRegistry_LoadNonExistentFile(t *testing.T) {
	tempDir := t.TempDir()
	registry := NewRegistry(tempDir)

	// Load should create empty registry when file doesn't exist
	err := registry.Load()
	if err != nil {
		t.Fatalf("Failed to load non-existent registry: %v", err)
	}

	// Verify empty state
	if len(registry.downloads) != 0 {
		t.Errorf("Expected empty downloads map, got %d entries", len(registry.downloads))
	}

	// Verify file was created
	registryPath := filepath.Join(tempDir, "downloads", "registry.json")
	if _, err := os.Stat(registryPath); os.IsNotExist(err) {
		t.Error("Registry file should have been created")
	}
}

func TestRegistry_SetStatus(t *testing.T) {
	tempDir := t.TempDir()
	registry := NewRegistry(tempDir)

	// Set status for new episode
	registry.SetStatus("episode1", StatusDownloading)

	info, exists := registry.downloads["episode1"]
	if !exists {
		t.Fatal("Download info was not created")
	}

	if info.Status != "downloading" {
		t.Errorf("Expected status 'downloading', got '%s'", info.Status)
	}

	if info.EpisodeID != "episode1" {
		t.Errorf("Expected episode ID 'episode1', got '%s'", info.EpisodeID)
	}

	// Update existing status
	registry.SetStatus("episode1", StatusCompleted)

	info, exists = registry.downloads["episode1"]
	if !exists {
		t.Fatal("Download info should still exist")
	}

	if info.Status != "completed" {
		t.Errorf("Expected status 'completed', got '%s'", info.Status)
	}
}

func TestRegistry_UpdateProgress(t *testing.T) {
	tempDir := t.TempDir()
	registry := NewRegistry(tempDir)

	startTime := time.Now()
	progress := &DownloadProgress{
		EpisodeID:       "episode1",
		Status:          StatusDownloading,
		Progress:        0.75,
		Speed:           2048000,
		BytesDownloaded: 15728640,
		TotalBytes:      20971520,
		RetryCount:      2,
		LastError:       "timeout",
		StartTime:       startTime,
		ETA:             3 * time.Second,
	}

	registry.UpdateProgress(progress)

	info, exists := registry.downloads["episode1"]
	if !exists {
		t.Fatal("Download info was not created")
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

	if info.BytesDownloaded != 15728640 {
		t.Errorf("Expected bytes downloaded 15728640, got %d", info.BytesDownloaded)
	}

	if info.TotalBytes != 20971520 {
		t.Errorf("Expected total bytes 20971520, got %d", info.TotalBytes)
	}

	if info.RetryCount != 2 {
		t.Errorf("Expected retry count 2, got %d", info.RetryCount)
	}

	if info.LastError != "timeout" {
		t.Errorf("Expected last error 'timeout', got '%s'", info.LastError)
	}

	if info.EstimatedTime != 3*time.Second {
		t.Errorf("Expected estimated time 3s, got %v", info.EstimatedTime)
	}
}

func TestRegistry_GetDownloadInfo(t *testing.T) {
	tempDir := t.TempDir()
	registry := NewRegistry(tempDir)

	// Test non-existent episode
	info, exists := registry.GetDownloadInfo("nonexistent")
	if exists {
		t.Error("Expected false for non-existent episode")
	}
	if info != nil {
		t.Error("Expected nil info for non-existent episode")
	}

	// Add download info
	originalInfo := &DownloadInfo{
		EpisodeID: "episode1",
		Status:    "downloading",
		Progress:  0.5,
		Speed:     1024000,
	}
	registry.downloads["episode1"] = originalInfo

	// Get download info (should return copy)
	info, exists = registry.GetDownloadInfo("episode1")
	if !exists {
		t.Error("Expected true for existing episode")
	}
	if info == nil {
		t.Fatal("Expected non-nil info")
	}

	// Verify it's a copy, not the same instance
	if info == originalInfo {
		t.Error("Expected copy, not same instance")
	}

	// Verify values match
	if info.EpisodeID != "episode1" {
		t.Errorf("Expected episode ID 'episode1', got '%s'", info.EpisodeID)
	}
	if info.Status != "downloading" {
		t.Errorf("Expected status 'downloading', got '%s'", info.Status)
	}
	if info.Progress != 0.5 {
		t.Errorf("Expected progress 0.5, got %f", info.Progress)
	}
}

func TestRegistry_GetAllDownloads(t *testing.T) {
	tempDir := t.TempDir()
	registry := NewRegistry(tempDir)

	// Test empty registry
	all := registry.GetAllDownloads()
	if len(all) != 0 {
		t.Errorf("Expected empty map, got %d entries", len(all))
	}

	// Add multiple downloads
	registry.downloads["episode1"] = &DownloadInfo{EpisodeID: "episode1", Status: "downloading"}
	registry.downloads["episode2"] = &DownloadInfo{EpisodeID: "episode2", Status: "completed"}

	all = registry.GetAllDownloads()
	if len(all) != 2 {
		t.Errorf("Expected 2 downloads, got %d", len(all))
	}

	// Verify copies are returned
	for id, info := range all {
		original := registry.downloads[id]
		if info == original {
			t.Errorf("Expected copy for episode %s, got same instance", id)
		}
		if info.EpisodeID != original.EpisodeID {
			t.Errorf("Expected episode ID '%s', got '%s'", original.EpisodeID, info.EpisodeID)
		}
	}
}

func TestRegistry_RemoveDownload(t *testing.T) {
	tempDir := t.TempDir()
	registry := NewRegistry(tempDir)

	// Add download
	registry.downloads["episode1"] = &DownloadInfo{EpisodeID: "episode1", Status: "completed"}
	registry.downloads["episode2"] = &DownloadInfo{EpisodeID: "episode2", Status: "downloading"}

	// Remove one download
	registry.RemoveDownload("episode1")

	if len(registry.downloads) != 1 {
		t.Errorf("Expected 1 download remaining, got %d", len(registry.downloads))
	}

	_, exists := registry.downloads["episode1"]
	if exists {
		t.Error("Episode1 should have been removed")
	}

	_, exists = registry.downloads["episode2"]
	if !exists {
		t.Error("Episode2 should still exist")
	}

	// Remove non-existent download (should not panic)
	registry.RemoveDownload("nonexistent")
	if len(registry.downloads) != 1 {
		t.Errorf("Expected 1 download still, got %d", len(registry.downloads))
	}
}

func TestRegistry_IsDownloaded(t *testing.T) {
	tempDir := t.TempDir()
	registry := NewRegistry(tempDir)

	// Test non-existent episode
	if registry.IsDownloaded("nonexistent") {
		t.Error("Expected false for non-existent episode")
	}

	// Test downloading episode
	registry.downloads["episode1"] = &DownloadInfo{EpisodeID: "episode1", Status: "downloading"}
	if registry.IsDownloaded("episode1") {
		t.Error("Expected false for downloading episode")
	}

	// Test completed episode
	registry.downloads["episode2"] = &DownloadInfo{EpisodeID: "episode2", Status: "completed"}
	if !registry.IsDownloaded("episode2") {
		t.Error("Expected true for completed episode")
	}

	// Test failed episode
	registry.downloads["episode3"] = &DownloadInfo{EpisodeID: "episode3", Status: "failed"}
	if registry.IsDownloaded("episode3") {
		t.Error("Expected false for failed episode")
	}
}

func TestRegistry_IsDownloading(t *testing.T) {
	tempDir := t.TempDir()
	registry := NewRegistry(tempDir)

	// Test non-existent episode
	if registry.IsDownloading("nonexistent") {
		t.Error("Expected false for non-existent episode")
	}

	// Test queued episode
	registry.downloads["episode1"] = &DownloadInfo{EpisodeID: "episode1", Status: "queued"}
	if !registry.IsDownloading("episode1") {
		t.Error("Expected true for queued episode")
	}

	// Test downloading episode
	registry.downloads["episode2"] = &DownloadInfo{EpisodeID: "episode2", Status: "downloading"}
	if !registry.IsDownloading("episode2") {
		t.Error("Expected true for downloading episode")
	}

	// Test completed episode
	registry.downloads["episode3"] = &DownloadInfo{EpisodeID: "episode3", Status: "completed"}
	if registry.IsDownloading("episode3") {
		t.Error("Expected false for completed episode")
	}

	// Test failed episode
	registry.downloads["episode4"] = &DownloadInfo{EpisodeID: "episode4", Status: "failed"}
	if registry.IsDownloading("episode4") {
		t.Error("Expected false for failed episode")
	}
}

func TestRegistry_GetSetConfig(t *testing.T) {
	tempDir := t.TempDir()
	registry := NewRegistry(tempDir)

	// Test getting default config
	config := registry.GetConfig()
	if config == nil {
		t.Fatal("Expected non-nil config")
	}

	// Verify it's a copy
	if config == registry.config {
		t.Error("Expected copy, not same instance")
	}

	// Test setting new config
	newConfig := &Config{
		MaxSizeGB:              15,
		MaxEpisodesPerPodcast:  30,
		AutoCleanup:            false,
		CleanupDays:            45,
		MaxConcurrentDownloads: 8,
		DownloadPath:           "/custom/path",
	}

	registry.SetConfig(newConfig)

	if registry.config != newConfig {
		t.Error("Expected config to be updated")
	}

	// Verify config was actually updated
	updatedConfig := registry.GetConfig()
	if updatedConfig.MaxSizeGB != 15 {
		t.Errorf("Expected MaxSizeGB 15, got %d", updatedConfig.MaxSizeGB)
	}
	if updatedConfig.MaxEpisodesPerPodcast != 30 {
		t.Errorf("Expected MaxEpisodesPerPodcast 30, got %d", updatedConfig.MaxEpisodesPerPodcast)
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()
	registry := NewRegistry(tempDir)

	// Test concurrent reads and writes
	done := make(chan bool, 10)

	// Start multiple goroutines doing different operations
	for i := 0; i < 5; i++ {
		go func(id int) {
			episodeID := fmt.Sprintf("episode%d", id)
			registry.SetStatus(episodeID, StatusDownloading)

			progress := &DownloadProgress{
				EpisodeID: episodeID,
				Status:    StatusDownloading,
				Progress:  0.5,
			}
			registry.UpdateProgress(progress)

			// Read operations
			registry.IsDownloaded(episodeID)
			registry.IsDownloading(episodeID)
			registry.GetDownloadInfo(episodeID)

			done <- true
		}(i)
	}

	// Start goroutines doing read-only operations
	for i := 0; i < 5; i++ {
		go func() {
			registry.GetAllDownloads()
			registry.GetConfig()
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify final state
	if len(registry.downloads) != 5 {
		t.Errorf("Expected 5 downloads, got %d", len(registry.downloads))
	}
}

func TestRegistry_LoadCorruptedFile(t *testing.T) {
	tempDir := t.TempDir()

	// Create directory structure
	registryDir := filepath.Join(tempDir, "downloads")
	err := os.MkdirAll(registryDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create registry directory: %v", err)
	}

	registryPath := filepath.Join(registryDir, "registry.json")

	// Create corrupted JSON file
	err = os.WriteFile(registryPath, []byte("invalid json {"), 0644)
	if err != nil {
		t.Fatalf("Failed to create corrupted file: %v", err)
	}

	registry := NewRegistry(tempDir)
	err = registry.Load()
	if err == nil {
		t.Error("Expected error when loading corrupted registry file")
	}
}
