package models

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
	
	"github.com/csams/podcast-tui/internal/markdown"
)

type Subscriptions struct {
	Podcasts []*Podcast     `json:"podcasts"`
	Queue    []*QueueEntry  `json:"queue,omitempty"`
	
	// episodeIndex is a map from episode ID to episode pointer for fast lookups
	// This is not serialized to JSON and is rebuilt on load
	episodeIndex map[string]*Episode `json:"-"`
	
	// podcastIndex is a map from episode ID to podcast pointer for fast lookups
	// This is not serialized to JSON and is rebuilt on load
	podcastIndex map[string]*Podcast `json:"-"`
	
	// queueMutex protects queue operations from concurrent access
	queueMutex sync.RWMutex `json:"-"`
}

func LoadSubscriptions() (*Subscriptions, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}

	path := filepath.Join(configDir, "podcast-tui", "subscriptions.json")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			subs := &Subscriptions{
				Podcasts: []*Podcast{},
				Queue:    []*QueueEntry{},
				episodeIndex: make(map[string]*Episode),
				podcastIndex: make(map[string]*Podcast),
			}
			return subs, nil
		}
		return nil, err
	}

	var subs Subscriptions
	if err := json.Unmarshal(data, &subs); err != nil {
		return nil, err
	}
	
	// Build the episode index
	subs.buildIndex()
	
	// Clean up any invalid queue entries
	queueCleaned := subs.CleanQueue()
	
	// Convert any missing descriptions
	descriptionsConverted := subs.ConvertMissingDescriptions()
	
	// Save if we made any changes
	if queueCleaned || descriptionsConverted {
		if err := subs.Save(); err != nil {
			// Log but don't fail - cleanup will happen again next time
			// This is not critical for app functionality
		}
	}

	return &subs, nil
}

func (s *Subscriptions) Save() error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}

	dir := filepath.Join(configDir, "podcast-tui")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path := filepath.Join(dir, "subscriptions.json")

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// ConvertMissingDescriptions converts any podcast or episode descriptions that haven't been converted yet
// Returns true if any conversions were performed
func (s *Subscriptions) ConvertMissingDescriptions() bool {
	converter := markdown.NewMarkdownConverter()
	converted := false
	
	for _, podcast := range s.Podcasts {
		// Convert podcast description if missing
		if podcast.Description != "" && podcast.ConvertedDescription == "" {
			result := converter.Convert(podcast.Description)
			podcast.ConvertedDescription = result.Text
			converted = true
		}
		
		// Convert episode descriptions if missing
		for _, episode := range podcast.Episodes {
			if episode.Description != "" && episode.ConvertedDescription == "" {
				result := converter.Convert(episode.Description)
				episode.ConvertedDescription = result.Text
				converted = true
			}
		}
	}
	
	return converted
}

func (s *Subscriptions) Add(podcast *Podcast) {
	for _, p := range s.Podcasts {
		if p.URL == podcast.URL {
			return
		}
	}
	s.Podcasts = append(s.Podcasts, podcast)
	
	// Add episodes to indexes
	if s.episodeIndex == nil {
		s.episodeIndex = make(map[string]*Episode)
	}
	if s.podcastIndex == nil {
		s.podcastIndex = make(map[string]*Podcast)
	}
	for _, episode := range podcast.Episodes {
		if episode.ID != "" {
			s.episodeIndex[episode.ID] = episode
			s.podcastIndex[episode.ID] = podcast
		}
	}
}

func (s *Subscriptions) Remove(url string) {
	for i, p := range s.Podcasts {
		if p.URL == url {
			// Remove episodes from indexes before removing podcast
			for _, episode := range p.Episodes {
				delete(s.episodeIndex, episode.ID)
				delete(s.podcastIndex, episode.ID)
			}
			s.Podcasts = append(s.Podcasts[:i], s.Podcasts[i+1:]...)
			return
		}
	}
}

// buildIndex rebuilds the episode and podcast indexes from scratch
func (s *Subscriptions) buildIndex() {
	s.episodeIndex = make(map[string]*Episode)
	s.podcastIndex = make(map[string]*Podcast)
	for _, podcast := range s.Podcasts {
		for _, episode := range podcast.Episodes {
			if episode.ID != "" {
				s.episodeIndex[episode.ID] = episode
				s.podcastIndex[episode.ID] = podcast
			}
		}
	}
}

// GetEpisodeByID returns an episode by its ID using the index
func (s *Subscriptions) GetEpisodeByID(episodeID string) *Episode {
	if s.episodeIndex == nil {
		s.buildIndex()
	}
	return s.episodeIndex[episodeID]
}

// UpdateEpisodeIndex updates the index when episodes are added or modified
func (s *Subscriptions) UpdateEpisodeIndex(episode *Episode, podcast *Podcast) {
	if s.episodeIndex == nil {
		s.episodeIndex = make(map[string]*Episode)
	}
	if s.podcastIndex == nil {
		s.podcastIndex = make(map[string]*Podcast)
	}
	if episode.ID != "" {
		s.episodeIndex[episode.ID] = episode
		if podcast != nil {
			s.podcastIndex[episode.ID] = podcast
		}
	}
}

// AddToQueue adds an episode to the playback queue
func (s *Subscriptions) AddToQueue(episodeID string) error {
	s.queueMutex.Lock()
	defer s.queueMutex.Unlock()
	
	// Check if episode exists
	if s.GetEpisodeByID(episodeID) == nil {
		return fmt.Errorf("episode not found: %s", episodeID)
	}
	
	// Check for duplicates
	for _, entry := range s.Queue {
		if entry.EpisodeID == episodeID {
			return fmt.Errorf("episode already in queue")
		}
	}
	
	// Add to queue
	position := len(s.Queue) + 1
	entry := &QueueEntry{
		EpisodeID: episodeID,
		AddedAt:   time.Now(),
		Position:  position,
	}
	s.Queue = append(s.Queue, entry)
	return nil
}

// RemoveFromQueue removes an episode from the queue
func (s *Subscriptions) RemoveFromQueue(episodeID string) {
	s.queueMutex.Lock()
	defer s.queueMutex.Unlock()
	
	newQueue := make([]*QueueEntry, 0, len(s.Queue))
	for _, entry := range s.Queue {
		if entry.EpisodeID != episodeID {
			newQueue = append(newQueue, entry)
		}
	}
	s.Queue = newQueue
	s.reindexQueue()
}

// GetQueuePosition returns the position of an episode in the queue (0 if not in queue)
func (s *Subscriptions) GetQueuePosition(episodeID string) int {
	s.queueMutex.RLock()
	defer s.queueMutex.RUnlock()
	
	for i, entry := range s.Queue {
		if entry.EpisodeID == episodeID {
			return i + 1
		}
	}
	return 0
}

// GetNextInQueue returns the next episode in the queue
func (s *Subscriptions) GetNextInQueue() *Episode {
	s.queueMutex.RLock()
	defer s.queueMutex.RUnlock()
	
	if len(s.Queue) > 0 {
		return s.GetEpisodeByID(s.Queue[0].EpisodeID)
	}
	return nil
}

// ReorderQueue reorders the queue based on new positions
func (s *Subscriptions) ReorderQueue(positions []int) {
	s.queueMutex.Lock()
	defer s.queueMutex.Unlock()
	
	if len(positions) != len(s.Queue) {
		return
	}
	
	newQueue := make([]*QueueEntry, len(s.Queue))
	for i, pos := range positions {
		if pos-1 < len(s.Queue) && pos-1 >= 0 {
			newQueue[i] = s.Queue[pos-1]
		}
	}
	s.Queue = newQueue
	s.reindexQueue()
}

// MoveQueueItemUp moves an item up in the queue (towards position 1)
func (s *Subscriptions) MoveQueueItemUp(index int) bool {
	s.queueMutex.Lock()
	defer s.queueMutex.Unlock()
	
	if index <= 0 || index >= len(s.Queue) {
		return false
	}
	
	// Swap with previous item
	s.Queue[index], s.Queue[index-1] = s.Queue[index-1], s.Queue[index]
	s.reindexQueue()
	return true
}

// MoveQueueItemDown moves an item down in the queue (towards the end)
func (s *Subscriptions) MoveQueueItemDown(index int) bool {
	s.queueMutex.Lock()
	defer s.queueMutex.Unlock()
	
	if index < 0 || index >= len(s.Queue)-1 {
		return false
	}
	
	// Swap with next item
	s.Queue[index], s.Queue[index+1] = s.Queue[index+1], s.Queue[index]
	s.reindexQueue()
	return true
}

// reindexQueue updates position numbers after queue changes
func (s *Subscriptions) reindexQueue() {
	for i, entry := range s.Queue {
		entry.Position = i + 1
	}
}

// GetQueueEpisodes returns all episodes in the queue in order
func (s *Subscriptions) GetQueueEpisodes() []*Episode {
	s.queueMutex.RLock()
	defer s.queueMutex.RUnlock()
	
	episodes := make([]*Episode, 0, len(s.Queue))
	for _, entry := range s.Queue {
		if episode := s.GetEpisodeByID(entry.EpisodeID); episode != nil {
			episodes = append(episodes, episode)
		}
	}
	return episodes
}

// CleanQueue removes any queue entries that reference non-existent episodes
func (s *Subscriptions) CleanQueue() bool {
	s.queueMutex.Lock()
	defer s.queueMutex.Unlock()
	
	newQueue := make([]*QueueEntry, 0, len(s.Queue))
	cleaned := false
	
	for _, entry := range s.Queue {
		if episode := s.GetEpisodeByID(entry.EpisodeID); episode != nil {
			newQueue = append(newQueue, entry)
		} else {
			cleaned = true
		}
	}
	
	if cleaned {
		s.Queue = newQueue
		s.reindexQueue()
	}
	
	return cleaned
}

// GetPodcastForEpisode returns the podcast that contains the given episode
func (s *Subscriptions) GetPodcastForEpisode(episodeID string) *Podcast {
	if s.podcastIndex == nil {
		s.buildIndex()
	}
	return s.podcastIndex[episodeID]
}
