package download

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/csams/podcast-tui/internal/models"
)

func TestNewStorageManager(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)
	storageManager := NewStorageManager(manager, tempDir)

	if storageManager.manager != manager {
		t.Error("Expected manager to be set")
	}

	if storageManager.configDir != tempDir {
		t.Errorf("Expected config dir '%s', got '%s'", tempDir, storageManager.configDir)
	}
}

func TestStorageManager_CalculateStorageUsage(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)
	storageManager := NewStorageManager(manager, tempDir)

	// Create test download structure
	downloadDir := filepath.Join(tempDir, "downloads")
	podcastDir := filepath.Join(downloadDir, "podcast1")
	err := os.MkdirAll(podcastDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create podcast directory: %v", err)
	}

	// Create test files
	file1 := filepath.Join(podcastDir, "episode1.mp3")
	file2 := filepath.Join(podcastDir, "episode2.mp3")
	tempFile := filepath.Join(downloadDir, "temp", "episode3.tmp")

	err = os.MkdirAll(filepath.Dir(tempFile), 0755)
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Write files with known sizes
	err = os.WriteFile(file1, make([]byte, 1024), 0644) // 1KB
	if err != nil {
		t.Fatalf("Failed to write file1: %v", err)
	}

	err = os.WriteFile(file2, make([]byte, 2048), 0644) // 2KB
	if err != nil {
		t.Fatalf("Failed to write file2: %v", err)
	}

	err = os.WriteFile(tempFile, make([]byte, 512), 0644) // 0.5KB (should be ignored)
	if err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	// Calculate usage
	usage, err := storageManager.CalculateStorageUsage()
	if err != nil {
		t.Fatalf("Failed to calculate storage usage: %v", err)
	}

	// Should be 1024 + 2048 = 3072 bytes (temp file ignored)
	expectedUsage := int64(3072)
	if usage != expectedUsage {
		t.Errorf("Expected usage %d bytes, got %d", expectedUsage, usage)
	}
}

func TestStorageManager_GetStorageUsageGB(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)
	storageManager := NewStorageManager(manager, tempDir)

	// Create test file
	downloadDir := filepath.Join(tempDir, "downloads")
	err := os.MkdirAll(downloadDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create download directory: %v", err)
	}

	testFile := filepath.Join(downloadDir, "test.mp3")
	// Create 1MB file
	err = os.WriteFile(testFile, make([]byte, 1024*1024), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	usage, err := storageManager.GetStorageUsageGB()
	if err != nil {
		t.Fatalf("Failed to get storage usage in GB: %v", err)
	}

	// Should be approximately 0.001 GB (1MB / 1024MB)
	expectedUsage := float64(1) / 1024
	if usage < expectedUsage*0.9 || usage > expectedUsage*1.1 {
		t.Errorf("Expected usage around %f GB, got %f", expectedUsage, usage)
	}
}

func TestStorageManager_IsStorageNearLimit(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)
	storageManager := NewStorageManager(manager, tempDir)

	// Set a small limit for testing
	config := &Config{
		MaxSizeGB: 1, // 1GB limit
	}
	manager.registry.SetConfig(config)

	// Test with no files (well under limit)
	nearLimit, percentage, err := storageManager.IsStorageNearLimit()
	if err != nil {
		t.Fatalf("Failed to check storage limit: %v", err)
	}

	if nearLimit {
		t.Error("Expected storage to not be near limit with no files")
	}

	if percentage != 0 {
		t.Errorf("Expected 0%% usage, got %f%%", percentage*100)
	}

	// Create files that exceed 90% of limit
	downloadDir := filepath.Join(tempDir, "downloads")
	err = os.MkdirAll(downloadDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create download directory: %v", err)
	}

	// Create 950MB file (95% of 1GB limit)
	bigFile := filepath.Join(downloadDir, "big.mp3")
	bigFileSize := int64(950 * 1024 * 1024) // 950MB
	file, err := os.Create(bigFile)
	if err != nil {
		t.Fatalf("Failed to create big file: %v", err)
	}

	err = file.Truncate(bigFileSize)
	if err != nil {
		t.Fatalf("Failed to set file size: %v", err)
	}
	file.Close()

	nearLimit, percentage, err = storageManager.IsStorageNearLimit()
	if err != nil {
		t.Fatalf("Failed to check storage limit: %v", err)
	}

	if !nearLimit {
		t.Error("Expected storage to be near limit")
	}

	if percentage < 0.9 {
		t.Errorf("Expected percentage >= 90%%, got %f%%", percentage*100)
	}
}

func TestStorageManager_CleanupOldDownloads(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)
	storageManager := NewStorageManager(manager, tempDir)

	// Set up config with auto-cleanup enabled
	config := &Config{
		MaxSizeGB:   1, // 1GB limit
		AutoCleanup: true,
	}
	manager.registry.SetConfig(config)

	// Create subscriptions with episodes
	now := time.Now()
	oldTime := now.Add(-10 * 24 * time.Hour) // 10 days ago
	recentTime := now.Add(-1 * time.Hour)    // 1 hour ago

	episode1 := &models.Episode{
		ID:           "episode1",
		Title:        "Old Episode",
		Downloaded:   true,
		DownloadPath: filepath.Join(tempDir, "downloads", "podcast1", "episode1.mp3"),
		DownloadSize: 500 * 1024 * 1024, // 500MB
		DownloadDate: oldTime,
		LastPlayed:   oldTime,
	}

	episode2 := &models.Episode{
		ID:           "episode2",
		Title:        "Recent Episode",
		Downloaded:   true,
		DownloadPath: filepath.Join(tempDir, "downloads", "podcast1", "episode2.mp3"),
		DownloadSize: 600 * 1024 * 1024, // 600MB
		DownloadDate: recentTime,
		LastPlayed:   recentTime,
	}

	podcast := &models.Podcast{
		Title:    "Test Podcast",
		Episodes: []*models.Episode{episode1, episode2},
	}

	subscriptions := &models.Subscriptions{
		Podcasts: []*models.Podcast{podcast},
	}

	// Create actual files
	err := os.MkdirAll(filepath.Dir(episode1.DownloadPath), 0755)
	if err != nil {
		t.Fatalf("Failed to create directories: %v", err)
	}

	err = os.WriteFile(episode1.DownloadPath, make([]byte, episode1.DownloadSize), 0644)
	if err != nil {
		t.Fatalf("Failed to create episode1 file: %v", err)
	}

	err = os.WriteFile(episode2.DownloadPath, make([]byte, episode2.DownloadSize), 0644)
	if err != nil {
		t.Fatalf("Failed to create episode2 file: %v", err)
	}

	// Verify files exist before cleanup
	if _, err := os.Stat(episode1.DownloadPath); os.IsNotExist(err) {
		t.Fatal("Episode1 file should exist before cleanup")
	}
	if _, err := os.Stat(episode2.DownloadPath); os.IsNotExist(err) {
		t.Fatal("Episode2 file should exist before cleanup")
	}

	// Trigger cleanup (should be triggered due to 110% usage)
	err = storageManager.CleanupOldDownloads(subscriptions)
	if err != nil {
		t.Fatalf("Failed to cleanup old downloads: %v", err)
	}

	// Verify old episode was cleaned up
	if _, err := os.Stat(episode1.DownloadPath); !os.IsNotExist(err) {
		t.Error("Episode1 file should have been cleaned up")
	}

	// Verify episode model was updated
	if episode1.Downloaded {
		t.Error("Episode1 should be marked as not downloaded")
	}
	if episode1.DownloadPath != "" {
		t.Error("Episode1 download path should be cleared")
	}

	// Recent episode should still exist if we're now under 80% limit
	// (This depends on the exact cleanup algorithm behavior)
}

func TestStorageManager_CleanupByAge(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)
	storageManager := NewStorageManager(manager, tempDir)

	// Set up config with age-based cleanup
	config := &Config{
		AutoCleanup: true,
		CleanupDays: 7, // 7 days
	}
	manager.registry.SetConfig(config)

	now := time.Now()
	oldTime := now.Add(-10 * 24 * time.Hour)   // 10 days ago (should be cleaned)
	recentTime := now.Add(-3 * 24 * time.Hour) // 3 days ago (should be kept)

	episode1 := &models.Episode{
		ID:           "old-episode",
		Title:        "Old Episode",
		Downloaded:   true,
		DownloadPath: filepath.Join(tempDir, "downloads", "podcast1", "old-episode.mp3"),
		DownloadDate: oldTime,
		LastPlayed:   oldTime,
	}

	episode2 := &models.Episode{
		ID:           "recent-episode",
		Title:        "Recent Episode",
		Downloaded:   true,
		DownloadPath: filepath.Join(tempDir, "downloads", "podcast1", "recent-episode.mp3"),
		DownloadDate: recentTime,
		LastPlayed:   recentTime,
	}

	podcast := &models.Podcast{
		Title:    "Test Podcast",
		Episodes: []*models.Episode{episode1, episode2},
	}

	subscriptions := &models.Subscriptions{
		Podcasts: []*models.Podcast{podcast},
	}

	// Create files
	err := os.MkdirAll(filepath.Dir(episode1.DownloadPath), 0755)
	if err != nil {
		t.Fatalf("Failed to create directories: %v", err)
	}

	err = os.WriteFile(episode1.DownloadPath, []byte("old content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create old episode file: %v", err)
	}

	err = os.WriteFile(episode2.DownloadPath, []byte("recent content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create recent episode file: %v", err)
	}

	// Run age-based cleanup
	err = storageManager.CleanupByAge(subscriptions)
	if err != nil {
		t.Fatalf("Failed to cleanup by age: %v", err)
	}

	// Verify old episode was cleaned up
	if _, err := os.Stat(episode1.DownloadPath); !os.IsNotExist(err) {
		t.Error("Old episode file should have been cleaned up")
	}

	if episode1.Downloaded {
		t.Error("Old episode should be marked as not downloaded")
	}

	// Verify recent episode was kept
	if _, err := os.Stat(episode2.DownloadPath); os.IsNotExist(err) {
		t.Error("Recent episode file should have been kept")
	}

	if !episode2.Downloaded {
		t.Error("Recent episode should still be marked as downloaded")
	}
}

func TestStorageManager_CleanupByPodcastLimit(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)
	storageManager := NewStorageManager(manager, tempDir)

	// Set podcast limit to 2 episodes
	config := &Config{
		MaxEpisodesPerPodcast: 2,
	}
	manager.registry.SetConfig(config)

	now := time.Now()

	// Create 4 episodes, should keep only 2 most recent
	episodes := []*models.Episode{
		{
			ID:           "episode1",
			Title:        "Episode 1",
			Downloaded:   true,
			DownloadPath: filepath.Join(tempDir, "downloads", "podcast1", "episode1.mp3"),
			DownloadDate: now.Add(-4 * 24 * time.Hour),
			LastPlayed:   now.Add(-4 * 24 * time.Hour),
		},
		{
			ID:           "episode2",
			Title:        "Episode 2",
			Downloaded:   true,
			DownloadPath: filepath.Join(tempDir, "downloads", "podcast1", "episode2.mp3"),
			DownloadDate: now.Add(-3 * 24 * time.Hour),
			LastPlayed:   now.Add(-2 * 24 * time.Hour), // Played more recently
		},
		{
			ID:           "episode3",
			Title:        "Episode 3",
			Downloaded:   true,
			DownloadPath: filepath.Join(tempDir, "downloads", "podcast1", "episode3.mp3"),
			DownloadDate: now.Add(-2 * 24 * time.Hour),
			LastPlayed:   time.Time{}, // Never played
		},
		{
			ID:           "episode4",
			Title:        "Episode 4",
			Downloaded:   true,
			DownloadPath: filepath.Join(tempDir, "downloads", "podcast1", "episode4.mp3"),
			DownloadDate: now.Add(-1 * 24 * time.Hour),
			LastPlayed:   now.Add(-1 * time.Hour), // Most recently played
		},
	}

	podcast := &models.Podcast{
		Title:    "Test Podcast",
		Episodes: episodes,
	}

	subscriptions := &models.Subscriptions{
		Podcasts: []*models.Podcast{podcast},
	}

	// Create files
	for _, episode := range episodes {
		err := os.MkdirAll(filepath.Dir(episode.DownloadPath), 0755)
		if err != nil {
			t.Fatalf("Failed to create directory for %s: %v", episode.ID, err)
		}

		err = os.WriteFile(episode.DownloadPath, []byte("content"), 0644)
		if err != nil {
			t.Fatalf("Failed to create file for %s: %v", episode.ID, err)
		}
	}

	// Run cleanup by podcast limit
	err := storageManager.CleanupByPodcastLimit(subscriptions)
	if err != nil {
		t.Fatalf("Failed to cleanup by podcast limit: %v", err)
	}

	// Count remaining downloaded episodes
	downloadedCount := 0
	for _, episode := range episodes {
		if episode.Downloaded {
			downloadedCount++
		}
	}

	if downloadedCount != 2 {
		t.Errorf("Expected 2 downloaded episodes remaining, got %d", downloadedCount)
	}

	// Verify the most recently played episodes were kept
	if !episodes[1].Downloaded { // episode2 - played recently
		t.Error("Episode2 should have been kept (played recently)")
	}

	if !episodes[3].Downloaded { // episode4 - most recently played
		t.Error("Episode4 should have been kept (most recently played)")
	}
}

func TestStorageManager_GetCleanupCandidates(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)
	storageManager := NewStorageManager(manager, tempDir)

	// Create mix of downloaded and non-downloaded episodes
	episodes := []*models.Episode{
		{ID: "episode1", Downloaded: true, DownloadPath: "/path/1"},
		{ID: "episode2", Downloaded: false},
		{ID: "episode3", Downloaded: true, DownloadPath: "/path/3"},
		{ID: "episode4", Downloaded: true, DownloadPath: ""}, // Downloaded but no path
		{ID: "episode5", Downloaded: false},
	}

	podcast := &models.Podcast{Episodes: episodes}
	subscriptions := &models.Subscriptions{Podcasts: []*models.Podcast{podcast}}

	candidates := storageManager.getCleanupCandidates(subscriptions)

	// Should return only episodes that are downloaded with paths
	expectedCount := 2 // episode1 and episode3
	if len(candidates) != expectedCount {
		t.Errorf("Expected %d candidates, got %d", expectedCount, len(candidates))
	}

	// Verify correct episodes are returned
	foundIDs := make(map[string]bool)
	for _, candidate := range candidates {
		foundIDs[candidate.ID] = true
	}

	if !foundIDs["episode1"] {
		t.Error("Expected episode1 in candidates")
	}
	if !foundIDs["episode3"] {
		t.Error("Expected episode3 in candidates")
	}
	if foundIDs["episode2"] || foundIDs["episode4"] || foundIDs["episode5"] {
		t.Error("Unexpected episodes in candidates")
	}
}

func TestStorageManager_GetCleanupPriority(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)
	storageManager := NewStorageManager(manager, tempDir)
	config := &Config{}

	now := time.Now()

	// Test episode that was never played (should have high priority)
	neverPlayedEpisode := &models.Episode{
		ID:           "never-played",
		DownloadDate: now.Add(-5 * 24 * time.Hour),
		LastPlayed:   time.Time{},      // Zero time = never played
		DownloadSize: 10 * 1024 * 1024, // 10MB
	}

	// Test episode played recently (should have lower priority)
	recentlyPlayedEpisode := &models.Episode{
		ID:           "recently-played",
		DownloadDate: now.Add(-5 * 24 * time.Hour),
		LastPlayed:   now.Add(-1 * time.Hour),
		DownloadSize: 10 * 1024 * 1024, // 10MB
	}

	neverPlayedPriority := storageManager.getCleanupPriority(neverPlayedEpisode, config)
	recentlyPlayedPriority := storageManager.getCleanupPriority(recentlyPlayedEpisode, config)

	// Never played should have higher priority (larger number)
	if neverPlayedPriority <= recentlyPlayedPriority {
		t.Errorf("Never played episode should have higher priority than recently played. "+
			"Never played: %f, Recently played: %f", neverPlayedPriority, recentlyPlayedPriority)
	}

	// Test that older episodes have higher priority than newer ones
	oldEpisode := &models.Episode{
		ID:           "old",
		DownloadDate: now.Add(-30 * 24 * time.Hour),
		LastPlayed:   now.Add(-30 * 24 * time.Hour),
		DownloadSize: 10 * 1024 * 1024,
	}

	newEpisode := &models.Episode{
		ID:           "new",
		DownloadDate: now.Add(-1 * 24 * time.Hour),
		LastPlayed:   now.Add(-1 * 24 * time.Hour),
		DownloadSize: 10 * 1024 * 1024,
	}

	oldPriority := storageManager.getCleanupPriority(oldEpisode, config)
	newPriority := storageManager.getCleanupPriority(newEpisode, config)

	if oldPriority <= newPriority {
		t.Errorf("Older episode should have higher priority than newer episode. "+
			"Old: %f, New: %f", oldPriority, newPriority)
	}
}

func TestStorageManager_GetStorageStats(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)
	storageManager := NewStorageManager(manager, tempDir)

	// Set config
	config := &Config{MaxSizeGB: 5}
	manager.registry.SetConfig(config)

	// Add some downloads to registry
	manager.registry.downloads["episode1"] = &DownloadInfo{EpisodeID: "episode1", Status: "completed"}
	manager.registry.downloads["episode2"] = &DownloadInfo{EpisodeID: "episode2", Status: "downloading"}

	// Create test files
	downloadDir := filepath.Join(tempDir, "downloads")
	err := os.MkdirAll(downloadDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create download directory: %v", err)
	}

	// Create 2MB file
	testFile := filepath.Join(downloadDir, "test.mp3")
	err = os.WriteFile(testFile, make([]byte, 2*1024*1024), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	stats, err := storageManager.GetStorageStats()
	if err != nil {
		t.Fatalf("Failed to get storage stats: %v", err)
	}

	// Verify stats
	if stats.TotalBytes != 2*1024*1024 {
		t.Errorf("Expected total bytes %d, got %d", 2*1024*1024, stats.TotalBytes)
	}

	expectedGB := float64(2) / 1024 // 2MB in GB
	if stats.TotalGB < expectedGB*0.9 || stats.TotalGB > expectedGB*1.1 {
		t.Errorf("Expected total GB around %f, got %f", expectedGB, stats.TotalGB)
	}

	if stats.LimitGB != 5.0 {
		t.Errorf("Expected limit GB 5.0, got %f", stats.LimitGB)
	}

	expectedUsagePercent := (float64(2*1024*1024) / float64(5*1024*1024*1024)) * 100
	if stats.UsagePercent < expectedUsagePercent*0.9 || stats.UsagePercent > expectedUsagePercent*1.1 {
		t.Errorf("Expected usage percent around %f, got %f", expectedUsagePercent, stats.UsagePercent)
	}

	if stats.EpisodeCount != 2 {
		t.Errorf("Expected episode count 2, got %d", stats.EpisodeCount)
	}
}

func TestStorageManager_RemoveEpisodeFiles(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)
	storageManager := NewStorageManager(manager, tempDir)

	// Create test file
	testFile := filepath.Join(tempDir, "test-episode.mp3")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create metadata file
	metadataFile := testFile + ".json"
	err = os.WriteFile(metadataFile, []byte("{}"), 0644)
	if err != nil {
		t.Fatalf("Failed to create metadata file: %v", err)
	}

	episode := &models.Episode{
		ID:           "test-episode",
		Downloaded:   true,
		DownloadPath: testFile,
		DownloadSize: 12,
		DownloadDate: time.Now(),
	}

	// Add to registry
	manager.registry.downloads["test-episode"] = &DownloadInfo{
		EpisodeID: "test-episode",
		Status:    "completed",
	}

	// Remove episode files
	err = storageManager.removeEpisodeFiles(episode)
	if err != nil {
		t.Fatalf("Failed to remove episode files: %v", err)
	}

	// Verify main file was removed
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("Main episode file should have been removed")
	}

	// Verify metadata file was removed (but don't fail if it wasn't)
	if _, err := os.Stat(metadataFile); !os.IsNotExist(err) {
		t.Log("Metadata file was not removed (this is okay)")
	}

	// Verify episode model was updated
	if episode.Downloaded {
		t.Error("Episode should be marked as not downloaded")
	}
	if episode.DownloadPath != "" {
		t.Error("Episode download path should be cleared")
	}
	if episode.DownloadSize != 0 {
		t.Error("Episode download size should be cleared")
	}

	// Verify registry was updated
	_, exists := manager.registry.GetDownloadInfo("test-episode")
	if exists {
		t.Error("Episode should be removed from registry")
	}
}

func TestStorageManager_CleanupDisabled(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)
	storageManager := NewStorageManager(manager, tempDir)

	// Disable auto-cleanup
	config := &Config{
		AutoCleanup: false,
		MaxSizeGB:   1,
	}
	manager.registry.SetConfig(config)

	subscriptions := &models.Subscriptions{}

	// Should not perform cleanup when disabled
	err := storageManager.CleanupOldDownloads(subscriptions)
	if err != nil {
		t.Fatalf("Cleanup should not error when disabled: %v", err)
	}

	err = storageManager.CleanupByAge(subscriptions)
	if err != nil {
		t.Fatalf("Age cleanup should not error when disabled: %v", err)
	}
}

func TestStorageManager_NonexistentDownloadDir(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewManager(tempDir)
	storageManager := NewStorageManager(manager, tempDir)

	// Calculate usage when download directory doesn't exist
	usage, err := storageManager.CalculateStorageUsage()

	// Should not error, should return 0
	if err != nil {
		t.Fatalf("Should not error on non-existent directory: %v", err)
	}

	if usage != 0 {
		t.Errorf("Expected 0 usage for non-existent directory, got %d", usage)
	}
}
