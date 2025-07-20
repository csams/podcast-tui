package models

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestGenerateEpisodeID(t *testing.T) {
	podcastURL := "https://example.com/feed.xml"
	episodeURL := "https://example.com/episode1.mp3"
	publishDate := time.Date(2023, 10, 15, 12, 0, 0, 0, time.UTC)

	// Test basic ID generation
	id1 := GenerateEpisodeID(podcastURL, episodeURL, publishDate)

	if id1 == "" {
		t.Error("Expected non-empty ID")
	}

	if len(id1) != 16 {
		t.Errorf("Expected ID length 16, got %d", len(id1))
	}

	// Test consistency - same inputs should produce same ID
	id2 := GenerateEpisodeID(podcastURL, episodeURL, publishDate)
	if id1 != id2 {
		t.Error("Expected consistent ID generation for same inputs")
	}

	// Test uniqueness - different inputs should produce different IDs
	differentURL := "https://example.com/episode2.mp3"
	id3 := GenerateEpisodeID(podcastURL, differentURL, publishDate)
	if id1 == id3 {
		t.Error("Expected different IDs for different episode URLs")
	}

	// Test date sensitivity
	differentDate := publishDate.Add(24 * time.Hour)
	id4 := GenerateEpisodeID(podcastURL, episodeURL, differentDate)
	if id1 == id4 {
		t.Error("Expected different IDs for different publish dates")
	}

	// Test podcast URL sensitivity
	differentPodcastURL := "https://different.com/feed.xml"
	id5 := GenerateEpisodeID(differentPodcastURL, episodeURL, publishDate)
	if id1 == id5 {
		t.Error("Expected different IDs for different podcast URLs")
	}

	// Test that ID only contains valid characters (hex)
	for _, char := range id1 {
		if !strings.ContainsRune("0123456789abcdef", char) {
			t.Errorf("ID contains invalid character: %c", char)
		}
	}
}

func TestGenerateEpisodeID_EmptyInputs(t *testing.T) {
	publishDate := time.Date(2023, 10, 15, 12, 0, 0, 0, time.UTC)

	// Test with empty podcast URL
	id1 := GenerateEpisodeID("", "https://example.com/episode.mp3", publishDate)
	if id1 == "" {
		t.Error("Should generate ID even with empty podcast URL")
	}

	// Test with empty episode URL
	id2 := GenerateEpisodeID("https://example.com/feed.xml", "", publishDate)
	if id2 == "" {
		t.Error("Should generate ID even with empty episode URL")
	}

	// Test with zero time
	id3 := GenerateEpisodeID("https://example.com/feed.xml", "https://example.com/episode.mp3", time.Time{})
	if id3 == "" {
		t.Error("Should generate ID even with zero time")
	}

	// Test all empty
	id4 := GenerateEpisodeID("", "", time.Time{})
	if id4 == "" {
		t.Error("Should generate ID even with all empty inputs")
	}

	// Verify they're all different
	ids := []string{id1, id2, id3, id4}
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			if ids[i] == ids[j] {
				t.Errorf("Expected different IDs for different inputs: %s == %s", ids[i], ids[j])
			}
		}
	}
}

func TestGenerateEpisodeID_UnicodeHandling(t *testing.T) {
	// Test with Unicode characters
	podcastURL := "https://example.com/podcast-with-émojis-🎧.xml"
	episodeURL := "https://example.com/episode-with-ñ-and-漢字.mp3"
	publishDate := time.Date(2023, 10, 15, 12, 0, 0, 0, time.UTC)

	id := GenerateEpisodeID(podcastURL, episodeURL, publishDate)

	if id == "" {
		t.Error("Should handle Unicode characters")
	}

	if len(id) != 16 {
		t.Errorf("Expected ID length 16 even with Unicode, got %d", len(id))
	}

	// Verify consistency
	id2 := GenerateEpisodeID(podcastURL, episodeURL, publishDate)
	if id != id2 {
		t.Error("Unicode handling should be consistent")
	}
}

func TestEpisode_GenerateID(t *testing.T) {
	podcastURL := "https://example.com/feed.xml"
	publishDate := time.Date(2023, 10, 15, 12, 0, 0, 0, time.UTC)

	episode := &Episode{
		Title:       "Test Episode",
		URL:         "https://example.com/episode1.mp3",
		PublishDate: publishDate,
	}

	// Test that ID is initially empty
	if episode.ID != "" {
		t.Error("Expected empty ID initially")
	}

	// Generate ID
	episode.GenerateID(podcastURL)

	// Verify ID was set
	if episode.ID == "" {
		t.Error("Expected ID to be generated")
	}

	if len(episode.ID) != 16 {
		t.Errorf("Expected ID length 16, got %d", len(episode.ID))
	}

	// Verify it matches direct function call
	expectedID := GenerateEpisodeID(podcastURL, episode.URL, episode.PublishDate)
	if episode.ID != expectedID {
		t.Errorf("Expected ID '%s', got '%s'", expectedID, episode.ID)
	}

	// Test regeneration produces same ID
	originalID := episode.ID
	episode.GenerateID(podcastURL)
	if episode.ID != originalID {
		t.Error("Regenerating ID should produce same result")
	}
}

func TestEpisode_GenerateID_DifferentEpisodes(t *testing.T) {
	podcastURL := "https://example.com/feed.xml"
	publishDate := time.Date(2023, 10, 15, 12, 0, 0, 0, time.UTC)

	episode1 := &Episode{
		Title:       "Episode 1",
		URL:         "https://example.com/episode1.mp3",
		PublishDate: publishDate,
	}

	episode2 := &Episode{
		Title:       "Episode 2",
		URL:         "https://example.com/episode2.mp3",
		PublishDate: publishDate,
	}

	episode1.GenerateID(podcastURL)
	episode2.GenerateID(podcastURL)

	if episode1.ID == episode2.ID {
		t.Error("Different episodes should have different IDs")
	}

	// Test same episode data but different objects
	episode3 := &Episode{
		Title:       "Episode 1", // Same as episode1
		URL:         "https://example.com/episode1.mp3",
		PublishDate: publishDate,
	}

	episode3.GenerateID(podcastURL)

	if episode1.ID != episode3.ID {
		t.Error("Same episode data should produce same ID")
	}
}

func TestEpisode_GenerateID_WithDifferentPodcasts(t *testing.T) {
	publishDate := time.Date(2023, 10, 15, 12, 0, 0, 0, time.UTC)

	episode := &Episode{
		Title:       "Test Episode",
		URL:         "https://example.com/episode1.mp3",
		PublishDate: publishDate,
	}

	// Generate ID for first podcast
	episode.GenerateID("https://podcast1.com/feed.xml")
	id1 := episode.ID

	// Generate ID for second podcast (same episode)
	episode.GenerateID("https://podcast2.com/feed.xml")
	id2 := episode.ID

	if id1 == id2 {
		t.Error("Same episode from different podcasts should have different IDs")
	}
}

func TestEpisode_IDStability(t *testing.T) {
	// Test that IDs remain stable across different scenarios
	podcastURL := "https://example.com/feed.xml"

	testCases := []struct {
		name     string
		episode  Episode
		expected string
	}{
		{
			name: "basic episode",
			episode: Episode{
				URL:         "https://example.com/episode1.mp3",
				PublishDate: time.Date(2023, 10, 15, 12, 0, 0, 0, time.UTC),
			},
			// This should produce a consistent hash
		},
		{
			name: "episode with special characters",
			episode: Episode{
				URL:         "https://example.com/episode with spaces & chars!.mp3",
				PublishDate: time.Date(2023, 10, 15, 12, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "episode with different time",
			episode: Episode{
				URL:         "https://example.com/episode1.mp3",
				PublishDate: time.Date(2023, 10, 16, 15, 30, 45, 123456789, time.UTC),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			episode := tc.episode

			// Generate ID multiple times
			episode.GenerateID(podcastURL)
			id1 := episode.ID

			episode.GenerateID(podcastURL)
			id2 := episode.ID

			// Should be consistent
			if id1 != id2 {
				t.Errorf("ID generation not stable: %s != %s", id1, id2)
			}

			// Should be valid length
			if len(id1) != 16 {
				t.Errorf("Expected ID length 16, got %d", len(id1))
			}

			// Should be hexadecimal
			for _, char := range id1 {
				if !strings.ContainsRune("0123456789abcdef", char) {
					t.Errorf("ID contains non-hex character: %c", char)
				}
			}
		})
	}
}

func TestEpisode_IDCollisionResistance(t *testing.T) {
	// Test that we don't get collisions with similar inputs
	podcastURL := "https://example.com/feed.xml"
	baseDate := time.Date(2023, 10, 15, 12, 0, 0, 0, time.UTC)

	// Generate many IDs and check for collisions
	seenIDs := make(map[string]bool)
	collisionCount := 0

	for i := 0; i < 1000; i++ {
		episode := &Episode{
			URL:         fmt.Sprintf("https://example.com/episode%d.mp3", i),
			PublishDate: baseDate.Add(time.Duration(i) * time.Hour),
		}

		episode.GenerateID(podcastURL)

		if seenIDs[episode.ID] {
			collisionCount++
		}
		seenIDs[episode.ID] = true
	}

	if collisionCount > 0 {
		t.Errorf("Found %d ID collisions out of 1000 episodes", collisionCount)
	}
}

func TestEpisode_IDWithZeroTime(t *testing.T) {
	podcastURL := "https://example.com/feed.xml"

	episode := &Episode{
		URL:         "https://example.com/episode.mp3",
		PublishDate: time.Time{}, // Zero time
	}

	episode.GenerateID(podcastURL)

	if episode.ID == "" {
		t.Error("Should generate ID even with zero publish date")
	}

	// Should be consistent
	originalID := episode.ID
	episode.GenerateID(podcastURL)
	if episode.ID != originalID {
		t.Error("ID with zero time should be consistent")
	}
}

func TestEpisode_JSONTagsPresent(t *testing.T) {
	// Verify that new download fields have proper JSON tags
	episode := Episode{
		ID:           "test-id",
		Downloaded:   true,
		DownloadPath: "/path/to/file.mp3",
		DownloadSize: 1024,
		DownloadDate: time.Now(),
		LastPlayed:   time.Now(),
	}

	// This is more of a compilation test to ensure the struct is properly defined
	// The actual JSON marshaling would be tested in integration tests
	if episode.ID == "" {
		t.Error("ID field should be accessible")
	}

	if !episode.Downloaded {
		t.Error("Downloaded field should be accessible")
	}

	if episode.DownloadPath == "" {
		t.Error("DownloadPath field should be accessible")
	}

	if episode.DownloadSize == 0 {
		t.Error("DownloadSize field should be accessible")
	}

	if episode.DownloadDate.IsZero() {
		t.Error("DownloadDate field should be accessible")
	}

	if episode.LastPlayed.IsZero() {
		t.Error("LastPlayed field should be accessible")
	}
}
