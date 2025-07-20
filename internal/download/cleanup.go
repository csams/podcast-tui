package download

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/csams/podcast-tui/internal/models"
)

// StorageManager handles storage cleanup and management
type StorageManager struct {
	manager   *Manager
	configDir string
}

// NewStorageManager creates a new storage manager
func NewStorageManager(manager *Manager, configDir string) *StorageManager {
	return &StorageManager{
		manager:   manager,
		configDir: configDir,
	}
}

// CalculateStorageUsage returns the total storage used by downloads in bytes
func (sm *StorageManager) CalculateStorageUsage() (int64, error) {
	downloadDir := filepath.Join(sm.configDir, "downloads")

	var totalSize int64
	err := filepath.Walk(downloadDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) != ".tmp" {
			totalSize += info.Size()
		}
		return nil
	})

	return totalSize, err
}

// GetStorageUsageGB returns storage usage in gigabytes
func (sm *StorageManager) GetStorageUsageGB() (float64, error) {
	bytes, err := sm.CalculateStorageUsage()
	if err != nil {
		return 0, err
	}
	return float64(bytes) / (1024 * 1024 * 1024), nil
}

// IsStorageNearLimit checks if storage is approaching the configured limit
func (sm *StorageManager) IsStorageNearLimit() (bool, float64, error) {
	config := sm.manager.registry.GetConfig()
	usage, err := sm.GetStorageUsageGB()
	if err != nil {
		return false, 0, err
	}

	limitGB := float64(config.MaxSizeGB)
	percentage := usage / limitGB

	return percentage >= 0.9, percentage, nil // 90% threshold
}

// CleanupOldDownloads removes old downloads based on LRU and age policies
func (sm *StorageManager) CleanupOldDownloads(subscriptions *models.Subscriptions) error {
	config := sm.manager.registry.GetConfig()

	if !config.AutoCleanup {
		return nil
	}

	// Get all downloaded episodes with their last played times
	candidates := sm.getCleanupCandidates(subscriptions)

	// Check if cleanup is needed
	nearLimit, percentage, err := sm.IsStorageNearLimit()
	if err != nil {
		return fmt.Errorf("failed to check storage usage: %w", err)
	}

	if !nearLimit && percentage < 0.8 {
		return nil // No cleanup needed
	}

	log.Printf("Storage cleanup triggered: %.1f%% of limit used", percentage*100)

	// Sort candidates by priority (LRU + age)
	sort.Slice(candidates, func(i, j int) bool {
		return sm.getCleanupPriority(candidates[i], config) < sm.getCleanupPriority(candidates[j], config)
	})

	// Remove episodes until we're back under 80% of limit
	targetUsage := float64(config.MaxSizeGB) * 0.8
	currentUsage, _ := sm.GetStorageUsageGB()

	for _, episode := range candidates {
		if currentUsage <= targetUsage {
			break
		}

		if err := sm.removeEpisodeFiles(episode); err != nil {
			log.Printf("Failed to remove episode %s: %v", episode.Title, err)
			continue
		}

		// Update current usage estimate
		if episode.DownloadSize > 0 {
			currentUsage -= float64(episode.DownloadSize) / (1024 * 1024 * 1024)
		}

		log.Printf("Cleaned up episode: %s", episode.Title)
	}

	return nil
}

// CleanupByAge removes downloads older than the configured age
func (sm *StorageManager) CleanupByAge(subscriptions *models.Subscriptions) error {
	config := sm.manager.registry.GetConfig()

	if !config.AutoCleanup || config.CleanupDays <= 0 {
		return nil
	}

	cutoffTime := time.Now().AddDate(0, 0, -config.CleanupDays)
	candidates := sm.getCleanupCandidates(subscriptions)

	for _, episode := range candidates {
		// Remove if not played recently or if downloaded long ago
		if episode.LastPlayed.Before(cutoffTime) ||
			(episode.LastPlayed.IsZero() && episode.DownloadDate.Before(cutoffTime)) {

			if err := sm.removeEpisodeFiles(episode); err != nil {
				log.Printf("Failed to remove old episode %s: %v", episode.Title, err)
				continue
			}

			log.Printf("Cleaned up old episode: %s", episode.Title)
		}
	}

	return nil
}

// CleanupByPodcastLimit enforces per-podcast episode limits
func (sm *StorageManager) CleanupByPodcastLimit(subscriptions *models.Subscriptions) error {
	config := sm.manager.registry.GetConfig()

	if config.MaxEpisodesPerPodcast <= 0 {
		return nil
	}

	for _, podcast := range subscriptions.Podcasts {
		downloadedEpisodes := sm.getDownloadedEpisodesForPodcast(podcast)

		if len(downloadedEpisodes) <= config.MaxEpisodesPerPodcast {
			continue
		}

		// Sort by last played time (oldest first)
		sort.Slice(downloadedEpisodes, func(i, j int) bool {
			if downloadedEpisodes[i].LastPlayed.IsZero() && downloadedEpisodes[j].LastPlayed.IsZero() {
				return downloadedEpisodes[i].DownloadDate.Before(downloadedEpisodes[j].DownloadDate)
			}
			if downloadedEpisodes[i].LastPlayed.IsZero() {
				return true
			}
			if downloadedEpisodes[j].LastPlayed.IsZero() {
				return false
			}
			return downloadedEpisodes[i].LastPlayed.Before(downloadedEpisodes[j].LastPlayed)
		})

		// Remove excess episodes
		excess := len(downloadedEpisodes) - config.MaxEpisodesPerPodcast
		for i := 0; i < excess; i++ {
			episode := downloadedEpisodes[i]
			if err := sm.removeEpisodeFiles(episode); err != nil {
				log.Printf("Failed to remove excess episode %s: %v", episode.Title, err)
				continue
			}
			log.Printf("Removed excess episode from %s: %s", podcast.Title, episode.Title)
		}
	}

	return nil
}

// getCleanupCandidates returns all downloaded episodes that can be cleaned up
func (sm *StorageManager) getCleanupCandidates(subscriptions *models.Subscriptions) []*models.Episode {
	var candidates []*models.Episode

	for _, podcast := range subscriptions.Podcasts {
		for _, episode := range podcast.Episodes {
			if episode.Downloaded && episode.DownloadPath != "" {
				candidates = append(candidates, episode)
			}
		}
	}

	return candidates
}

// getDownloadedEpisodesForPodcast returns downloaded episodes for a specific podcast
func (sm *StorageManager) getDownloadedEpisodesForPodcast(podcast *models.Podcast) []*models.Episode {
	var downloaded []*models.Episode

	for _, episode := range podcast.Episodes {
		if episode.Downloaded && episode.DownloadPath != "" {
			downloaded = append(downloaded, episode)
		}
	}

	return downloaded
}

// getCleanupPriority calculates cleanup priority (lower = clean first)
func (sm *StorageManager) getCleanupPriority(episode *models.Episode, config *Config) float64 {
	priority := 0.0

	// Age factor (older = higher priority for cleanup)
	if !episode.DownloadDate.IsZero() {
		daysSinceDownload := time.Since(episode.DownloadDate).Hours() / 24
		priority += daysSinceDownload * 0.1
	}

	// Last played factor (not played recently = higher priority for cleanup)
	if episode.LastPlayed.IsZero() {
		priority += 1000.0 // Never played gets high cleanup priority
	} else {
		daysSinceLastPlayed := time.Since(episode.LastPlayed).Hours() / 24
		priority += daysSinceLastPlayed * 0.5
	}

	// Size factor (larger files get slightly higher priority)
	if episode.DownloadSize > 0 {
		sizeMB := float64(episode.DownloadSize) / (1024 * 1024)
		priority += sizeMB * 0.01
	}

	return priority
}

// removeEpisodeFiles removes the downloaded files for an episode
func (sm *StorageManager) removeEpisodeFiles(episode *models.Episode) error {
	if episode.DownloadPath == "" {
		return nil
	}

	// Remove the audio file
	if err := os.Remove(episode.DownloadPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove audio file: %w", err)
	}

	// Remove metadata file if it exists
	metadataPath := episode.DownloadPath + ".json"
	if err := os.Remove(metadataPath); err != nil && !os.IsNotExist(err) {
		// Don't fail if metadata file removal fails
		log.Printf("Failed to remove metadata file %s: %v", metadataPath, err)
	}

	// Update episode model
	episode.Downloaded = false
	episode.DownloadPath = ""
	episode.DownloadSize = 0
	episode.DownloadDate = time.Time{}

	// Remove from download registry
	sm.manager.registry.RemoveDownload(episode.ID)

	return nil
}

// GetStorageStats returns comprehensive storage statistics
func (sm *StorageManager) GetStorageStats() (*StorageStats, error) {
	totalBytes, err := sm.CalculateStorageUsage()
	if err != nil {
		return nil, err
	}

	config := sm.manager.registry.GetConfig()
	limitBytes := int64(config.MaxSizeGB) * 1024 * 1024 * 1024

	stats := &StorageStats{
		TotalBytes:   totalBytes,
		TotalGB:      float64(totalBytes) / (1024 * 1024 * 1024),
		LimitBytes:   limitBytes,
		LimitGB:      float64(config.MaxSizeGB),
		UsagePercent: float64(totalBytes) / float64(limitBytes) * 100,
		EpisodeCount: len(sm.manager.registry.GetAllDownloads()),
	}

	return stats, nil
}

// StorageStats represents storage usage statistics
type StorageStats struct {
	TotalBytes   int64   `json:"totalBytes"`
	TotalGB      float64 `json:"totalGB"`
	LimitBytes   int64   `json:"limitBytes"`
	LimitGB      float64 `json:"limitGB"`
	UsagePercent float64 `json:"usagePercent"`
	EpisodeCount int     `json:"episodeCount"`
}
