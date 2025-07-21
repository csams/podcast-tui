package models

import (
	"crypto/sha256"
	"fmt"
	"time"
)

type Podcast struct {
	ID          string
	Title       string
	Description string
	URL         string
	ImageURL    string
	Author      string
	Episodes    []*Episode
	LastUpdated time.Time
	
	// Converted description (persisted for performance)
	ConvertedDescription string `json:"convertedDescription,omitempty"`
}

type Episode struct {
	ID           string        `json:"id"`
	Title        string        `json:"title"`
	Description  string        `json:"description"`
	URL          string        `json:"url"`
	Duration     time.Duration `json:"duration,omitempty"` // Discovered durations take precedence over RSS feed data
	PublishDate  time.Time     `json:"publishDate"`
	Played       bool          `json:"played"`
	Position     time.Duration `json:"position"`
	Downloaded   bool          `json:"downloaded,omitempty"`
	DownloadPath string        `json:"downloadPath,omitempty"`
	DownloadSize int64         `json:"downloadSize,omitempty"`
	DownloadDate time.Time     `json:"downloadDate,omitempty"`
	LastPlayed   time.Time     `json:"lastPlayed,omitempty"`
	
	// Converted description (persisted for performance)
	ConvertedDescription string `json:"convertedDescription,omitempty"`
}

// GenerateEpisodeID creates a unique ID for an episode based on podcast URL, episode URL, and publish date
func GenerateEpisodeID(podcastURL, episodeURL string, publishDate time.Time) string {
	h := sha256.New()
	h.Write([]byte(podcastURL + episodeURL + publishDate.Format(time.RFC3339)))
	return fmt.Sprintf("%x", h.Sum(nil))[:16] // First 16 chars for filename safety
}

// GenerateID generates an ID for this episode using the parent podcast URL
func (e *Episode) GenerateID(podcastURL string) {
	e.ID = GenerateEpisodeID(podcastURL, e.URL, e.PublishDate)
}
