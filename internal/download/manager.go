package download

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/csams/podcast-tui/internal/models"
)

// Manager handles download operations and queue management
type Manager struct {
	mu              sync.RWMutex
	registry        *Registry
	configManager   *ConfigManager
	storageManager  *StorageManager
	downloader      *Downloader
	queue           chan *DownloadTask
	activeDownloads map[string]*DownloadTask
	progressCh      chan *DownloadProgress
	stopCh          chan struct{}
	running         bool
	configDir       string
}

// NewManager creates a new download manager
func NewManager(configDir string) *Manager {
	configManager := NewConfigManager(configDir)
	registry := NewRegistry(configDir)

	// Ensure directories exist
	if err := configManager.EnsureDownloadDir(); err != nil {
		log.Printf("Failed to create download directory: %v", err)
	}

	downloadDir := configManager.GetDownloadDir()
	tempDir := filepath.Join(downloadDir, "temp")

	manager := &Manager{
		registry:        registry,
		configManager:   configManager,
		downloader:      NewDownloader(tempDir, downloadDir),
		queue:           make(chan *DownloadTask, 100),
		activeDownloads: make(map[string]*DownloadTask),
		progressCh:      make(chan *DownloadProgress, 100),
		stopCh:          make(chan struct{}),
		configDir:       configDir,
	}

	// Initialize storage manager
	manager.storageManager = NewStorageManager(manager, configDir)

	return manager
}

// Start initializes and starts the download manager
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("download manager already running")
	}

	// Load configuration and registry
	if err := m.configManager.Load(); err != nil {
		log.Printf("Failed to load download config: %v", err)
	}

	if err := m.registry.Load(); err != nil {
		log.Printf("Failed to load download registry: %v", err)
	}

	m.running = true

	// Start worker goroutines
	config := m.configManager.GetConfig()
	for i := 0; i < config.MaxConcurrentDownloads; i++ {
		go m.downloadWorker()
	}

	// Start progress reporter
	go m.progressReporter()

	log.Printf("Download manager started with %d workers", config.MaxConcurrentDownloads)
	return nil
}

// Stop gracefully stops the download manager
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	close(m.stopCh)
	m.running = false

	// Cancel all active downloads
	for _, task := range m.activeDownloads {
		if task.Cancel != nil {
			task.Cancel()
		}
	}

	// Save registry
	if err := m.registry.Save(); err != nil {
		log.Printf("Failed to save download registry: %v", err)
	}

	log.Println("Download manager stopped")
}

// QueueDownload adds an episode to the download queue
func (m *Manager) QueueDownload(episode *models.Episode, podcastTitle string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return fmt.Errorf("download manager not running")
	}

	// Check if already downloaded
	if m.registry.IsDownloaded(episode.ID) {
		return fmt.Errorf("episode already downloaded")
	}

	// Check if already in queue or downloading
	if m.registry.IsDownloading(episode.ID) {
		return fmt.Errorf("episode already downloading or queued")
	}

	// Create download task
	ctx, cancel := context.WithCancel(context.Background())
	task := &DownloadTask{
		Episode:     episode,
		PodcastHash: m.GeneratePodcastDirectory(podcastTitle),
		Priority:    0,
		Context:     ctx,
		Cancel:      cancel,
	}

	// Queue the task
	select {
	case m.queue <- task:
		m.registry.SetStatus(episode.ID, StatusQueued)
		log.Printf("Queued download for episode: %s", episode.Title)
		return nil
	default:
		cancel()
		return fmt.Errorf("download queue is full")
	}
}

// CancelDownload cancels a download
func (m *Manager) CancelDownload(episodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Cancel active download
	if task, exists := m.activeDownloads[episodeID]; exists {
		if task.Cancel != nil {
			task.Cancel()
		}
		delete(m.activeDownloads, episodeID)
	}

	// Update registry
	m.registry.SetStatus(episodeID, StatusCancelled)

	// Cleanup temp file
	filename := m.generateFilenameForID(episodeID)
	if err := m.downloader.CleanupTempFile(filename); err != nil {
		log.Printf("Failed to cleanup temp file for %s: %v", episodeID, err)
	}

	// Keep cancelled downloads in registry for user reference

	log.Printf("Cancelled download for episode: %s", episodeID)
	return nil
}

// GetDownloadProgress returns the current progress for an episode
func (m *Manager) GetDownloadProgress(episodeID string) (*DownloadProgress, bool) {
	if info, exists := m.registry.GetDownloadInfo(episodeID); exists {
		progress := &DownloadProgress{
			EpisodeID:       info.EpisodeID,
			Status:          m.parseStatus(info.Status),
			Progress:        info.Progress,
			Speed:           info.Speed,
			BytesDownloaded: info.BytesDownloaded,
			TotalBytes:      info.TotalBytes,
			ETA:             info.EstimatedTime,
			LastError:       info.LastError,
			StartTime:       info.StartTime,
			RetryCount:      info.RetryCount,
		}
		return progress, true
	}
	return nil, false
}

// GetAllDownloads returns information about all downloads
func (m *Manager) GetAllDownloads() map[string]*DownloadProgress {
	result := make(map[string]*DownloadProgress)
	for episodeID, info := range m.registry.GetAllDownloads() {
		result[episodeID] = &DownloadProgress{
			EpisodeID:       info.EpisodeID,
			Status:          m.parseStatus(info.Status),
			Progress:        info.Progress,
			Speed:           info.Speed,
			BytesDownloaded: info.BytesDownloaded,
			TotalBytes:      info.TotalBytes,
			ETA:             info.EstimatedTime,
			LastError:       info.LastError,
			StartTime:       info.StartTime,
			RetryCount:      info.RetryCount,
		}
	}
	return result
}

// GetStorageStats returns storage usage statistics
func (m *Manager) GetStorageStats() (*StorageStats, error) {
	return m.storageManager.GetStorageStats()
}

// TriggerCleanup performs storage cleanup
func (m *Manager) TriggerCleanup(subscriptions *models.Subscriptions) error {
	if err := m.storageManager.CleanupOldDownloads(subscriptions); err != nil {
		return err
	}
	if err := m.storageManager.CleanupByAge(subscriptions); err != nil {
		return err
	}
	if err := m.storageManager.CleanupByPodcastLimit(subscriptions); err != nil {
		return err
	}
	// Save subscriptions after cleanup
	return subscriptions.Save()
}

// IsDownloaded checks if an episode is downloaded by checking the filesystem
func (m *Manager) IsDownloaded(episodeID string) bool {
	// First check if it's in the registry as completed
	if m.registry.IsDownloaded(episodeID) {
		return true
	}
	
	// Check filesystem directly - this is the source of truth
	return m.isDownloadedOnFilesystem(episodeID)
}

// isDownloadedOnFilesystem checks if the episode file actually exists on disk
func (m *Manager) isDownloadedOnFilesystem(episodeID string) bool {
	// We need to check all possible locations since we don't know the podcast
	// This is not ideal but necessary for the episodeID-only interface
	baseDir := m.configManager.GetDownloadDir()
	
	// Try to find the file by searching subdirectories
	// This is expensive but ensures we find existing files
	return m.findEpisodeFile(baseDir, episodeID) != ""
}

// findEpisodeFile searches for an episode file in the download directory
func (m *Manager) findEpisodeFile(baseDir, episodeID string) string {
	// Search in all podcast subdirectories for any .mp3 files
	// We'll match by episode ID in the registry or by checking episode metadata
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return ""
	}
	
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		
		podcastDir := filepath.Join(baseDir, entry.Name())
		files, err := os.ReadDir(podcastDir)
		if err != nil {
			continue
		}
		
		for _, file := range files {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".mp3") {
				filePath := filepath.Join(podcastDir, file.Name())
				
				// Check if this file corresponds to our episode ID
				// For now, we'll need a way to map files to episode IDs
				// This is a limitation of the current design
				// TODO: Improve this by storing episode ID -> file path mapping
				
				// For the immediate fix, check if there's a registry entry
				// that maps this episode ID to a file path
				if info, exists := m.registry.GetDownloadInfo(episodeID); exists {
					if strings.Contains(filePath, filepath.Base(info.EpisodeID)) ||
					   strings.Contains(info.EpisodeID, filepath.Base(filePath)) {
						return filePath
					}
				}
			}
		}
	}
	
	return ""
}

// IsDownloading checks if an episode is currently downloading
func (m *Manager) IsDownloading(episodeID string) bool {
	return m.registry.IsDownloading(episodeID)
}

// IsEpisodeDownloaded checks if an episode is downloaded using episode metadata
func (m *Manager) IsEpisodeDownloaded(episode *models.Episode, podcastTitle string) bool {
	// First check registry
	if m.registry.IsDownloaded(episode.ID) {
		return true
	}
	
	// Check if episode model already says it's downloaded
	if episode.Downloaded && episode.DownloadPath != "" {
		// Verify the file still exists
		if _, err := os.Stat(episode.DownloadPath); err == nil {
			return true
		}
		// File is missing, reset the episode state
		episode.Downloaded = false
		episode.DownloadPath = ""
	}
	
	// Check if file exists using the actual naming scheme
	podcastDir := m.GeneratePodcastDirectory(podcastTitle)
	filename := m.GenerateFilename(episode)
	fullPath := filepath.Join(m.configManager.GetDownloadDir(), podcastDir)
	filePath := filepath.Join(fullPath, filename)
	
	if _, err := os.Stat(filePath); err == nil {
		// File exists! Update the episode model and registry to reflect this
		episode.Downloaded = true
		episode.DownloadPath = filePath
		
		// Also update registry for consistency
		m.registry.SetStatus(episode.ID, StatusCompleted)
		return true
	}
	
	return false
}

// RemoveFromRegistry removes an episode from the download registry
func (m *Manager) RemoveFromRegistry(episodeID string) {
	m.registry.RemoveDownload(episodeID)
}

// GetProgressChannel returns the progress channel for UI updates
func (m *Manager) GetProgressChannel() <-chan *DownloadProgress {
	return m.progressCh
}

// GetDownloadDir returns the configured download directory
func (m *Manager) GetDownloadDir() string {
	return m.configManager.GetDownloadDir()
}

// downloadWorker processes downloads from the queue
func (m *Manager) downloadWorker() {
	for {
		select {
		case task := <-m.queue:
			m.processDownload(task)
		case <-m.stopCh:
			return
		}
	}
}

// processDownload handles the actual download process
func (m *Manager) processDownload(task *DownloadTask) {
	episodeID := task.Episode.ID

	m.mu.Lock()
	m.activeDownloads[episodeID] = task
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		delete(m.activeDownloads, episodeID)
		m.mu.Unlock()
	}()

	// Update status to downloading
	m.registry.SetStatus(episodeID, StatusDownloading)

	// Create progress tracking
	progress := &DownloadProgress{
		EpisodeID: episodeID,
		Status:    StatusDownloading,
		StartTime: time.Now(),
	}

	// Get file size first
	totalSize, err := m.downloader.GetFileSize(task.Context, task.Episode.URL)
	if err != nil {
		log.Printf("Failed to get file size for %s: %v", task.Episode.Title, err)
		// Continue with unknown size
	} else {
		progress.TotalBytes = totalSize
	}

	// Create progress callback
	progressCallback := func(current, total, speed int64) {
		progress.BytesDownloaded = current
		progress.TotalBytes = total
		progress.Speed = speed

		if total > 0 {
			progress.Progress = float64(current) / float64(total)
		}

		if speed > 0 && total > current {
			remaining := total - current
			progress.ETA = time.Duration(remaining/speed) * time.Second
		}

		// Update registry
		m.registry.UpdateProgress(progress)

		// Send to progress channel (non-blocking)
		select {
		case m.progressCh <- progress:
		default:
		}
	}

	// Generate filename
	filename := m.GenerateFilename(task.Episode)
	podcastDir := filepath.Join(m.configManager.GetDownloadDir(), task.PodcastHash)

	// Ensure podcast directory exists
	if err := os.MkdirAll(podcastDir, 0755); err != nil {
		m.handleDownloadError(task, fmt.Errorf("failed to create podcast directory: %w", err))
		return
	}

	// Set target directory for this download
	m.downloader.targetDir = podcastDir

	// Attempt download with retries
	var lastErr error
	maxRetries := 5

	for retry := 0; retry <= maxRetries; retry++ {
		if retry > 0 {
			// Exponential backoff
			delay := time.Duration(math.Pow(2, float64(retry-1))) * time.Second
			if delay > 16*time.Second {
				delay = 16 * time.Second
			}

			log.Printf("Retrying download for %s (attempt %d/%d) after %v", task.Episode.Title, retry+1, maxRetries+1, delay)

			select {
			case <-time.After(delay):
			case <-task.Context.Done():
				m.registry.SetStatus(episodeID, StatusCancelled)
				return
			}
		}

		progress.RetryCount = retry
		m.registry.UpdateProgress(progress)

		// Attempt download
		err = m.downloader.DownloadFile(task.Context, task.Episode.URL, filename, progressCallback)
		if err == nil {
			// Success!
			m.handleDownloadSuccess(task, podcastDir, filename)
			return
		}

		lastErr = err
		log.Printf("Download failed for %s (attempt %d/%d): %v", task.Episode.Title, retry+1, maxRetries+1, err)

		// Check if context was cancelled
		if task.Context.Err() != nil {
			m.registry.SetStatus(episodeID, StatusCancelled)
			return
		}
	}

	// All retries exhausted
	m.handleDownloadError(task, fmt.Errorf("download failed after %d retries: %w", maxRetries+1, lastErr))
}

// handleDownloadSuccess processes a successful download
func (m *Manager) handleDownloadSuccess(task *DownloadTask, podcastDir, filename string) {
	episodeID := task.Episode.ID
	filePath := filepath.Join(podcastDir, filename)

	// Get file size
	stat, err := os.Stat(filePath)
	if err != nil {
		log.Printf("Failed to get file stats for %s: %v", task.Episode.Title, err)
	}

	// Update episode model
	task.Episode.Downloaded = true
	task.Episode.DownloadPath = filePath
	task.Episode.DownloadDate = time.Now()
	if stat != nil {
		task.Episode.DownloadSize = stat.Size()
	}

	// Update registry
	progress := &DownloadProgress{
		EpisodeID:       episodeID,
		Status:          StatusCompleted,
		Progress:        1.0,
		BytesDownloaded: task.Episode.DownloadSize,
		TotalBytes:      task.Episode.DownloadSize,
		StartTime:       time.Now(),
	}
	m.registry.UpdateProgress(progress)

	// Send final progress update
	select {
	case m.progressCh <- progress:
	default:
	}

	log.Printf("Download completed for episode: %s", task.Episode.Title)

	// Keep completed downloads in registry - they serve as the source of truth
	// for download status and should not be automatically removed

	// Trigger cleanup check after successful download
	go func() {
		if nearLimit, _, err := m.storageManager.IsStorageNearLimit(); err == nil && nearLimit {
			log.Println("Storage near limit, cleanup will be triggered by next UI interaction")
		}
	}()
}

// handleDownloadError processes a failed download
func (m *Manager) handleDownloadError(task *DownloadTask, err error) {
	episodeID := task.Episode.ID

	progress := &DownloadProgress{
		EpisodeID: episodeID,
		Status:    StatusFailed,
		LastError: err.Error(),
		StartTime: time.Now(),
	}

	m.registry.UpdateProgress(progress)

	// Send error update
	select {
	case m.progressCh <- progress:
	default:
	}

	// Cleanup temp file
	filename := m.generateFilenameForID(episodeID)
	if cleanupErr := m.downloader.CleanupTempFile(filename); cleanupErr != nil {
		log.Printf("Failed to cleanup temp file for %s: %v", episodeID, cleanupErr)
	}

	// Keep failed downloads in registry for user reference

	log.Printf("Download failed for episode %s: %v", task.Episode.Title, err)
}

// progressReporter saves registry periodically
func (m *Manager) progressReporter() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := m.registry.Save(); err != nil {
				log.Printf("Failed to save registry: %v", err)
			}
		case <-m.stopCh:
			return
		}
	}
}

// GeneratePodcastDirectory creates a sanitized directory name for the podcast
func (m *Manager) GeneratePodcastDirectory(podcastTitle string) string {
	// Sanitize the podcast title for use as a directory name
	sanitized := strings.TrimSpace(podcastTitle)

	// Replace all non-alphanumeric characters with underscores
	var result strings.Builder
	for _, r := range sanitized {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == ' ' {
			result.WriteRune(r)
		} else {
			result.WriteRune('_')
		}
	}
	sanitized = result.String()

	// Replace multiple spaces/underscores with single underscore
	sanitized = strings.ReplaceAll(sanitized, "  ", " ")
	sanitized = strings.ReplaceAll(sanitized, " ", "_")
	sanitized = strings.ReplaceAll(sanitized, "__", "_")

	// Trim underscores from start/end
	sanitized = strings.Trim(sanitized, "_")

	// Limit length to prevent filesystem issues
	if len(sanitized) > 255 {
		sanitized = sanitized[:255]
		sanitized = strings.Trim(sanitized, "_")
	}

	// Fallback to hash if sanitization results in empty string
	if sanitized == "" {
		h := sha256.New()
		h.Write([]byte(strings.ToLower(strings.TrimSpace(podcastTitle))))
		return fmt.Sprintf("podcast_%x", h.Sum(nil))[:20]
	}

	return sanitized
}

// GenerateFilename creates a filename for an episode
func (m *Manager) GenerateFilename(episode *models.Episode) string {
	// Sanitize episode title for use as filename
	title := strings.TrimSpace(episode.Title)
	if title == "" {
		title = episode.ID
	}

	// Replace all non-alphanumeric characters with underscores
	var result strings.Builder
	for _, r := range title {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == ' ' {
			result.WriteRune(r)
		} else {
			result.WriteRune('_')
		}
	}
	title = result.String()

	// Replace multiple spaces with single underscore
	title = strings.ReplaceAll(title, "  ", " ")
	title = strings.ReplaceAll(title, " ", "_")
	title = strings.ReplaceAll(title, "__", "_")

	// Trim underscores from start/end
	title = strings.Trim(title, "_")

	// Limit length to prevent filesystem issues  
	// Reserve 4 chars for .mp3 extension
	if len(title) > 251 {
		title = title[:251]
		title = strings.Trim(title, "_")
	}

	// Fallback to ID if sanitization results in empty string
	if title == "" {
		title = episode.ID
	}

	return title + ".mp3" // Assume MP3 for now, could be enhanced to detect format
}

// generateFilenameForID creates a filename using just the episode ID (for cleanup operations)
func (m *Manager) generateFilenameForID(episodeID string) string {
	return episodeID + ".mp3"
}

// parseStatus converts string status back to DownloadStatus
func (m *Manager) parseStatus(status string) DownloadStatus {
	switch status {
	case "queued":
		return StatusQueued
	case "downloading":
		return StatusDownloading
	case "completed":
		return StatusCompleted
	case "failed":
		return StatusFailed
	case "cancelled":
		return StatusCancelled
	case "paused":
		return StatusPaused
	default:
		return StatusFailed
	}
}
