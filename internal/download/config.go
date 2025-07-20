package download

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ConfigManager handles loading and saving download configuration
type ConfigManager struct {
	configPath string
	config     *Config
}

// NewConfigManager creates a new configuration manager
func NewConfigManager(configDir string) *ConfigManager {
	return &ConfigManager{
		configPath: filepath.Join(configDir, "download-config.json"),
		config:     DefaultConfig(),
	}
}

// Load loads the configuration from disk
func (cm *ConfigManager) Load() error {
	if _, err := os.Stat(cm.configPath); os.IsNotExist(err) {
		// Create default config if it doesn't exist
		return cm.Save()
	}

	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if err := json.Unmarshal(data, cm.config); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	return nil
}

// Save saves the configuration to disk
func (cm *ConfigManager) Save() error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(cm.configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(cm.config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(cm.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetConfig returns the current configuration
func (cm *ConfigManager) GetConfig() *Config {
	return cm.config
}

// SetConfig updates the configuration
func (cm *ConfigManager) SetConfig(config *Config) {
	cm.config = config
}

// GetDownloadDir returns the download directory path
func (cm *ConfigManager) GetDownloadDir() string {
	if cm.config.DownloadPath != "" {
		return cm.config.DownloadPath
	}

	// Default to ~/Music/Podcasts
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to config dir if home dir unavailable
		return filepath.Join(filepath.Dir(cm.configPath), "downloads")
	}
	return filepath.Join(homeDir, "Music", "Podcasts")
}

// EnsureDownloadDir creates the download directory if it doesn't exist
func (cm *ConfigManager) EnsureDownloadDir() error {
	downloadDir := cm.GetDownloadDir()
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		return fmt.Errorf("failed to create download directory: %w", err)
	}

	// Create temp directory for in-progress downloads
	tempDir := filepath.Join(downloadDir, "temp")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	return nil
}
