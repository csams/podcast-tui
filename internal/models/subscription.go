package models

import (
	"encoding/json"
	"os"
	"path/filepath"
	
	"github.com/csams/podcast-tui/internal/markdown"
)

type Subscriptions struct {
	Podcasts []*Podcast `json:"podcasts"`
	
	// episodeIndex is a map from episode ID to episode pointer for fast lookups
	// This is not serialized to JSON and is rebuilt on load
	episodeIndex map[string]*Episode `json:"-"`
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
				episodeIndex: make(map[string]*Episode),
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
	
	// Convert any missing descriptions
	if subs.ConvertMissingDescriptions() {
		// Save if we converted any descriptions
		if err := subs.Save(); err != nil {
			// Log but don't fail - conversions will happen again next time
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
	
	// Add episodes to index
	if s.episodeIndex == nil {
		s.episodeIndex = make(map[string]*Episode)
	}
	for _, episode := range podcast.Episodes {
		if episode.ID != "" {
			s.episodeIndex[episode.ID] = episode
		}
	}
}

func (s *Subscriptions) Remove(url string) {
	for i, p := range s.Podcasts {
		if p.URL == url {
			// Remove episodes from index before removing podcast
			for _, episode := range p.Episodes {
				delete(s.episodeIndex, episode.ID)
			}
			s.Podcasts = append(s.Podcasts[:i], s.Podcasts[i+1:]...)
			return
		}
	}
}

// buildIndex rebuilds the episode index from scratch
func (s *Subscriptions) buildIndex() {
	s.episodeIndex = make(map[string]*Episode)
	for _, podcast := range s.Podcasts {
		for _, episode := range podcast.Episodes {
			if episode.ID != "" {
				s.episodeIndex[episode.ID] = episode
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
func (s *Subscriptions) UpdateEpisodeIndex(episode *Episode) {
	if s.episodeIndex == nil {
		s.episodeIndex = make(map[string]*Episode)
	}
	if episode.ID != "" {
		s.episodeIndex[episode.ID] = episode
	}
}
