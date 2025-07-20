package models

import (
	"encoding/json"
	"os"
	"path/filepath"
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
