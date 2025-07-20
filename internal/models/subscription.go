package models

import (
	"encoding/json"
	"os"
	"path/filepath"
	
	"github.com/csams/podcast-tui/internal/markdown"
)

type Subscriptions struct {
	Podcasts []*Podcast `json:"podcasts"`
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
			return &Subscriptions{Podcasts: []*Podcast{}}, nil
		}
		return nil, err
	}

	var subs Subscriptions
	if err := json.Unmarshal(data, &subs); err != nil {
		return nil, err
	}
	
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
}

func (s *Subscriptions) Remove(url string) {
	for i, p := range s.Podcasts {
		if p.URL == url {
			s.Podcasts = append(s.Podcasts[:i], s.Podcasts[i+1:]...)
			return
		}
	}
}
