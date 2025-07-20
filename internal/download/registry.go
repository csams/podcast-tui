package download

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Registry manages download state persistence
type Registry struct {
	mu           sync.RWMutex
	registryPath string
	downloads    map[string]*DownloadInfo
	config       *Config
}

// RegistryData represents the persisted registry structure
type RegistryData struct {
	Downloads map[string]*DownloadInfo `json:"downloads"`
	Config    *Config                  `json:"config"`
	Version   int                      `json:"version"`
}

// NewRegistry creates a new download registry
func NewRegistry(configDir string) *Registry {
	return &Registry{
		registryPath: filepath.Join(configDir, "downloads", "registry.json"),
		downloads:    make(map[string]*DownloadInfo),
		config:       DefaultConfig(),
	}
}

// Load loads the registry from disk
func (r *Registry) Load() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, err := os.Stat(r.registryPath); os.IsNotExist(err) {
		// Create empty registry if it doesn't exist
		return r.saveUnsafe()
	}

	data, err := os.ReadFile(r.registryPath)
	if err != nil {
		return fmt.Errorf("failed to read registry file: %w", err)
	}

	var registryData RegistryData
	if err := json.Unmarshal(data, &registryData); err != nil {
		return fmt.Errorf("failed to parse registry file: %w", err)
	}

	r.downloads = registryData.Downloads
	if r.downloads == nil {
		r.downloads = make(map[string]*DownloadInfo)
	}

	if registryData.Config != nil {
		r.config = registryData.Config
	}

	return nil
}

// Save saves the registry to disk
func (r *Registry) Save() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.saveUnsafe()
}

func (r *Registry) saveUnsafe() error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(r.registryPath), 0755); err != nil {
		return fmt.Errorf("failed to create registry directory: %w", err)
	}

	registryData := RegistryData{
		Downloads: r.downloads,
		Config:    r.config,
		Version:   1,
	}

	data, err := json.MarshalIndent(registryData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal registry: %w", err)
	}

	if err := os.WriteFile(r.registryPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write registry file: %w", err)
	}

	return nil
}

// SetStatus updates the status of a download
func (r *Registry) SetStatus(episodeID string, status DownloadStatus) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Validate episode ID
	if episodeID == "" {
		return // Ignore empty episode IDs
	}

	info, exists := r.downloads[episodeID]
	if !exists {
		info = &DownloadInfo{
			EpisodeID: episodeID,
			StartTime: time.Now(),
		}
		r.downloads[episodeID] = info
	}

	info.Status = status.String()
}

// UpdateProgress updates the download progress
func (r *Registry) UpdateProgress(progress *DownloadProgress) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Validate episode ID
	if progress.EpisodeID == "" {
		return // Ignore empty episode IDs
	}

	info, exists := r.downloads[progress.EpisodeID]
	if !exists {
		info = &DownloadInfo{
			EpisodeID: progress.EpisodeID,
			StartTime: progress.StartTime,
		}
		r.downloads[progress.EpisodeID] = info
	}

	info.Status = progress.Status.String()
	info.Progress = progress.Progress
	info.Speed = progress.Speed
	info.BytesDownloaded = progress.BytesDownloaded
	info.TotalBytes = progress.TotalBytes
	info.RetryCount = progress.RetryCount
	info.LastError = progress.LastError
	info.EstimatedTime = progress.ETA
}

// GetDownloadInfo returns download information for an episode
func (r *Registry) GetDownloadInfo(episodeID string) (*DownloadInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, exists := r.downloads[episodeID]
	if !exists {
		return nil, false
	}

	// Return a copy to avoid data races
	infoCopy := *info
	return &infoCopy, true
}

// GetAllDownloads returns all download information
func (r *Registry) GetAllDownloads() map[string]*DownloadInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*DownloadInfo)
	for id, info := range r.downloads {
		infoCopy := *info
		result[id] = &infoCopy
	}
	return result
}

// RemoveDownload removes a download from the registry
func (r *Registry) RemoveDownload(episodeID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.downloads, episodeID)
}

// IsDownloaded checks if an episode is downloaded
func (r *Registry) IsDownloaded(episodeID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, exists := r.downloads[episodeID]
	return exists && info.Status == StatusCompleted.String()
}

// IsDownloading checks if an episode is currently downloading
func (r *Registry) IsDownloading(episodeID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	info, exists := r.downloads[episodeID]
	return exists && (info.Status == StatusDownloading.String() || info.Status == StatusQueued.String())
}

// GetConfig returns the configuration
func (r *Registry) GetConfig() *Config {
	r.mu.RLock()
	defer r.mu.RUnlock()

	configCopy := *r.config
	return &configCopy
}

// SetConfig updates the configuration
func (r *Registry) SetConfig(config *Config) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.config = config
}
