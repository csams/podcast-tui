package models

import (
	"time"
)

// QueueEntry represents a single episode in the playback queue
type QueueEntry struct {
	EpisodeID string    `json:"episode_id"`
	AddedAt   time.Time `json:"added_at"`
	Position  int       `json:"position"`
}