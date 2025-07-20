package download

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewConfigManager(t *testing.T) {
	tempDir := t.TempDir()
	cm := NewConfigManager(tempDir)

	expectedPath := filepath.Join(tempDir, "download-config.json")
	if cm.configPath != expectedPath {
		t.Errorf("Expected config path '%s', got '%s'", expectedPath, cm.configPath)
	}

	if cm.config == nil {
		t.Error("Expected config to be initialized")
	}

	// Verify default config is loaded
	if cm.config.MaxSizeGB != 5 {
		t.Errorf("Expected default MaxSizeGB 5, got %d", cm.config.MaxSizeGB)
	}
}

func TestConfigManager_SaveAndLoad(t *testing.T) {
	tempDir := t.TempDir()
	cm := NewConfigManager(tempDir)

	// Modify config
	cm.config.MaxSizeGB = 15
	cm.config.MaxEpisodesPerPodcast = 25
	cm.config.AutoCleanup = false
	cm.config.CleanupDays = 60
	cm.config.MaxConcurrentDownloads = 7
	cm.config.DownloadPath = "/custom/download/path"

	// Save config
	err := cm.Save()
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Verify file exists
	configPath := filepath.Join(tempDir, "download-config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}

	// Create new config manager and load
	cm2 := NewConfigManager(tempDir)
	err = cm2.Load()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify loaded config matches saved config
	if cm2.config.MaxSizeGB != 15 {
		t.Errorf("Expected MaxSizeGB 15, got %d", cm2.config.MaxSizeGB)
	}

	if cm2.config.MaxEpisodesPerPodcast != 25 {
		t.Errorf("Expected MaxEpisodesPerPodcast 25, got %d", cm2.config.MaxEpisodesPerPodcast)
	}

	if cm2.config.AutoCleanup {
		t.Error("Expected AutoCleanup to be false")
	}

	if cm2.config.CleanupDays != 60 {
		t.Errorf("Expected CleanupDays 60, got %d", cm2.config.CleanupDays)
	}

	if cm2.config.MaxConcurrentDownloads != 7 {
		t.Errorf("Expected MaxConcurrentDownloads 7, got %d", cm2.config.MaxConcurrentDownloads)
	}

	if cm2.config.DownloadPath != "/custom/download/path" {
		t.Errorf("Expected DownloadPath '/custom/download/path', got '%s'", cm2.config.DownloadPath)
	}
}

func TestConfigManager_LoadNonExistentFile(t *testing.T) {
	tempDir := t.TempDir()
	cm := NewConfigManager(tempDir)

	// Load should create default config when file doesn't exist
	err := cm.Load()
	if err != nil {
		t.Fatalf("Failed to load non-existent config: %v", err)
	}

	// Verify default values are set
	if cm.config.MaxSizeGB != 5 {
		t.Errorf("Expected default MaxSizeGB 5, got %d", cm.config.MaxSizeGB)
	}

	// Verify file was created
	configPath := filepath.Join(tempDir, "download-config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file should have been created")
	}
}

func TestConfigManager_LoadCorruptedFile(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "download-config.json")

	// Create corrupted JSON file
	err := os.WriteFile(configPath, []byte("invalid json {"), 0644)
	if err != nil {
		t.Fatalf("Failed to create corrupted file: %v", err)
	}

	cm := NewConfigManager(tempDir)
	err = cm.Load()
	if err == nil {
		t.Error("Expected error when loading corrupted config file")
	}
}

func TestConfigManager_GetConfig(t *testing.T) {
	tempDir := t.TempDir()
	cm := NewConfigManager(tempDir)

	config := cm.GetConfig()
	if config == nil {
		t.Error("Expected config to be returned")
	}

	if config != cm.config {
		t.Error("Expected returned config to be the same instance")
	}
}

func TestConfigManager_SetConfig(t *testing.T) {
	tempDir := t.TempDir()
	cm := NewConfigManager(tempDir)

	newConfig := &Config{
		MaxSizeGB:              20,
		MaxEpisodesPerPodcast:  50,
		AutoCleanup:            false,
		CleanupDays:            90,
		MaxConcurrentDownloads: 10,
		DownloadPath:           "/new/path",
	}

	cm.SetConfig(newConfig)

	if cm.config != newConfig {
		t.Error("Expected config to be updated")
	}

	if cm.config.MaxSizeGB != 20 {
		t.Errorf("Expected MaxSizeGB 20, got %d", cm.config.MaxSizeGB)
	}
}

func TestConfigManager_GetDownloadDir(t *testing.T) {
	tempDir := t.TempDir()
	cm := NewConfigManager(tempDir)

	// Test default download directory (now uses ~/Music/Podcasts)
	homeDir, err := os.UserHomeDir()
	var downloadDir string
	if err != nil {
		// Fallback to old behavior when home dir is unavailable
		expectedDir := filepath.Join(tempDir, "downloads")
		downloadDir = cm.GetDownloadDir()
		if downloadDir != expectedDir {
			t.Errorf("Expected download dir '%s', got '%s'", expectedDir, downloadDir)
		}
	} else {
		expectedDir := filepath.Join(homeDir, "Music", "Podcasts")
		downloadDir = cm.GetDownloadDir()
		if downloadDir != expectedDir {
			t.Errorf("Expected download dir '%s', got '%s'", expectedDir, downloadDir)
		}
	}

	// Test custom download path
	cm.config.DownloadPath = "/custom/downloads"
	downloadDir = cm.GetDownloadDir()
	if downloadDir != "/custom/downloads" {
		t.Errorf("Expected download dir '/custom/downloads', got '%s'", downloadDir)
	}
}

func TestConfigManager_EnsureDownloadDir(t *testing.T) {
	tempDir := t.TempDir()
	cm := NewConfigManager(tempDir)

	err := cm.EnsureDownloadDir()
	if err != nil {
		t.Fatalf("Failed to ensure download directory: %v", err)
	}

	// Verify main download directory exists
	downloadDir := cm.GetDownloadDir()
	if _, err := os.Stat(downloadDir); os.IsNotExist(err) {
		t.Error("Download directory was not created")
	}

	// Verify temp directory exists
	tempDownloadDir := filepath.Join(downloadDir, "temp")
	if _, err := os.Stat(tempDownloadDir); os.IsNotExist(err) {
		t.Error("Temp download directory was not created")
	}
}

func TestConfigManager_EnsureDownloadDirCustomPath(t *testing.T) {
	tempDir := t.TempDir()
	customPath := filepath.Join(tempDir, "custom-downloads")

	cm := NewConfigManager(tempDir)
	cm.config.DownloadPath = customPath

	err := cm.EnsureDownloadDir()
	if err != nil {
		t.Fatalf("Failed to ensure custom download directory: %v", err)
	}

	// Verify custom download directory exists
	if _, err := os.Stat(customPath); os.IsNotExist(err) {
		t.Error("Custom download directory was not created")
	}

	// Verify temp directory exists in custom path
	tempDownloadDir := filepath.Join(customPath, "temp")
	if _, err := os.Stat(tempDownloadDir); os.IsNotExist(err) {
		t.Error("Temp directory was not created in custom path")
	}
}

func TestConfigManager_SaveInvalidDirectory(t *testing.T) {
	// Try to save to a non-existent parent directory without permissions
	invalidDir := "/root/invalid/path/that/should/not/exist"
	cm := NewConfigManager(invalidDir)

	err := cm.Save()
	if err == nil {
		t.Error("Expected error when saving to invalid directory")
	}
}

func TestConfigManager_JSONMarshalingFormat(t *testing.T) {
	tempDir := t.TempDir()
	cm := NewConfigManager(tempDir)

	// Save with custom config
	cm.config.MaxSizeGB = 8
	cm.config.AutoCleanup = true

	err := cm.Save()
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Read the raw JSON file
	configPath := filepath.Join(tempDir, "download-config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	// Verify it's properly formatted JSON (indented)
	jsonStr := string(data)
	if !contains(jsonStr, "{\n  ") {
		t.Error("Expected indented JSON format")
	}

	if !contains(jsonStr, "\"maxSizeGB\": 8") {
		t.Error("Expected maxSizeGB field in JSON")
	}

	if !contains(jsonStr, "\"autoCleanup\": true") {
		t.Error("Expected autoCleanup field in JSON")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
