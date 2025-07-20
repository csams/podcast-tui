package feed

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseFeed_Success(t *testing.T) {
	// Sample RSS feed XML
	rssContent := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:itunes="http://itunes.apple.com/dtds/podcast-1.0.dtd">
  <channel>
    <title>Test Podcast</title>
    <description>A test podcast for unit testing</description>
    <link>https://example.com</link>
    <image>
      <url>https://example.com/image.jpg</url>
    </image>
    <item>
      <title>Episode 1</title>
      <description>First test episode</description>
      <enclosure url="https://example.com/episode1.mp3" type="audio/mpeg" length="1024"/>
      <pubDate>Mon, 15 Oct 2023 12:00:00 GMT</pubDate>
      <itunes:duration>30:00</itunes:duration>
    </item>
    <item>
      <title>Episode 2</title>
      <description>Second test episode</description>
      <enclosure url="https://example.com/episode2.mp3" type="audio/mpeg" length="2048"/>
      <pubDate>Tue, 16 Oct 2023 12:00:00 GMT</pubDate>
      <itunes:duration>45:00</itunes:duration>
    </item>
  </channel>
</rss>`

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(rssContent))
	}))
	defer server.Close()

	// Parse feed
	podcast, err := ParseFeed(server.URL)
	if err != nil {
		t.Fatalf("Failed to parse feed: %v", err)
	}

	// Verify podcast metadata
	if podcast.Title != "Test Podcast" {
		t.Errorf("Expected title 'Test Podcast', got '%s'", podcast.Title)
	}

	if podcast.Description != "A test podcast for unit testing" {
		t.Errorf("Expected description 'A test podcast for unit testing', got '%s'", podcast.Description)
	}

	if podcast.URL != server.URL {
		t.Errorf("Expected URL '%s', got '%s'", server.URL, podcast.URL)
	}

	if podcast.ImageURL != "https://example.com/image.jpg" {
		t.Errorf("Expected image URL 'https://example.com/image.jpg', got '%s'", podcast.ImageURL)
	}

	// Verify episodes
	if len(podcast.Episodes) != 2 {
		t.Fatalf("Expected 2 episodes, got %d", len(podcast.Episodes))
	}

	// Verify first episode
	episode1 := podcast.Episodes[0]
	if episode1.Title != "Episode 1" {
		t.Errorf("Expected episode1 title 'Episode 1', got '%s'", episode1.Title)
	}

	if episode1.Description != "First test episode" {
		t.Errorf("Expected episode1 description 'First test episode', got '%s'", episode1.Description)
	}

	if episode1.URL != "https://example.com/episode1.mp3" {
		t.Errorf("Expected episode1 URL 'https://example.com/episode1.mp3', got '%s'", episode1.URL)
	}

	expectedDuration := 30 * time.Minute
	if episode1.Duration != expectedDuration {
		t.Errorf("Expected episode1 duration %v, got %v", expectedDuration, episode1.Duration)
	}

	// Verify episode ID was generated
	if episode1.ID == "" {
		t.Error("Expected episode1 to have generated ID")
	}

	if len(episode1.ID) != 16 {
		t.Errorf("Expected episode1 ID length 16, got %d", len(episode1.ID))
	}

	// Verify second episode
	episode2 := podcast.Episodes[1]
	if episode2.Title != "Episode 2" {
		t.Errorf("Expected episode2 title 'Episode 2', got '%s'", episode2.Title)
	}

	if episode2.ID == "" {
		t.Error("Expected episode2 to have generated ID")
	}

	// Verify episodes have different IDs
	if episode1.ID == episode2.ID {
		t.Error("Expected different IDs for different episodes")
	}

	// Verify publish dates were parsed
	if episode1.PublishDate.IsZero() {
		t.Error("Expected episode1 publish date to be parsed")
	}

	if episode2.PublishDate.IsZero() {
		t.Error("Expected episode2 publish date to be parsed")
	}

	// Verify episodes are chronologically ordered (episode1 should be earlier)
	if !episode1.PublishDate.Before(episode2.PublishDate) {
		t.Error("Expected episode1 to be published before episode2")
	}
}

func TestParseFeed_IDConsistency(t *testing.T) {
	// Test that parsing the same feed produces consistent episode IDs
	rssContent := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Consistency Test Podcast</title>
    <description>Testing ID consistency</description>
    <item>
      <title>Consistent Episode</title>
      <description>This episode should have a consistent ID</description>
      <enclosure url="https://example.com/consistent.mp3" type="audio/mpeg" length="1024"/>
      <pubDate>Mon, 15 Oct 2023 12:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(rssContent))
	}))
	defer server.Close()

	// Parse feed twice
	podcast1, err := ParseFeed(server.URL)
	if err != nil {
		t.Fatalf("Failed to parse feed first time: %v", err)
	}

	podcast2, err := ParseFeed(server.URL)
	if err != nil {
		t.Fatalf("Failed to parse feed second time: %v", err)
	}

	// Verify IDs are consistent
	if len(podcast1.Episodes) != 1 || len(podcast2.Episodes) != 1 {
		t.Fatal("Expected exactly one episode in each podcast")
	}

	episode1 := podcast1.Episodes[0]
	episode2 := podcast2.Episodes[0]

	if episode1.ID != episode2.ID {
		t.Errorf("Expected consistent episode IDs, got '%s' and '%s'", episode1.ID, episode2.ID)
	}
}

func TestParseFeed_WithMissingFields(t *testing.T) {
	// Test parsing with some missing fields
	rssContent := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Minimal Podcast</title>
    <description>A minimal podcast</description>
    <item>
      <title>Episode Without Duration</title>
      <enclosure url="https://example.com/noduration.mp3" type="audio/mpeg" length="1024"/>
      <pubDate>Mon, 15 Oct 2023 12:00:00 GMT</pubDate>
    </item>
    <item>
      <title>Episode Without Date</title>
      <description>No publish date</description>
      <enclosure url="https://example.com/nodate.mp3" type="audio/mpeg" length="2048"/>
    </item>
    <item>
      <title>Episode Without Description</title>
      <enclosure url="https://example.com/nodesc.mp3" type="audio/mpeg" length="1024"/>
      <pubDate>Tue, 16 Oct 2023 12:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(rssContent))
	}))
	defer server.Close()

	podcast, err := ParseFeed(server.URL)
	if err != nil {
		t.Fatalf("Failed to parse feed with missing fields: %v", err)
	}

	if len(podcast.Episodes) != 3 {
		t.Fatalf("Expected 3 episodes, got %d", len(podcast.Episodes))
	}

	// All episodes should still have IDs
	for i, episode := range podcast.Episodes {
		if episode.ID == "" {
			t.Errorf("Episode %d should have generated ID even with missing fields", i)
		}

		if len(episode.ID) != 16 {
			t.Errorf("Episode %d ID should be 16 characters, got %d", i, len(episode.ID))
		}
	}

	// Test specific episodes
	episode1 := podcast.Episodes[0] // No duration
	if episode1.Duration != 0 {
		t.Errorf("Expected zero duration for episode1, got %v", episode1.Duration)
	}

	episode2 := podcast.Episodes[1] // No date
	if !episode2.PublishDate.IsZero() {
		t.Error("Expected zero publish date for episode2")
	}

	episode3 := podcast.Episodes[2] // No description
	if episode3.Description != "" {
		t.Errorf("Expected empty description for episode3, got '%s'", episode3.Description)
	}
}

func TestParseFeed_EmptyFeed(t *testing.T) {
	// Test parsing empty feed
	rssContent := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Empty Podcast</title>
    <description>A podcast with no episodes</description>
  </channel>
</rss>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(rssContent))
	}))
	defer server.Close()

	podcast, err := ParseFeed(server.URL)
	if err != nil {
		t.Fatalf("Failed to parse empty feed: %v", err)
	}

	if len(podcast.Episodes) != 0 {
		t.Errorf("Expected 0 episodes for empty feed, got %d", len(podcast.Episodes))
	}

	if podcast.Title != "Empty Podcast" {
		t.Errorf("Expected title 'Empty Podcast', got '%s'", podcast.Title)
	}
}

func TestParseFeed_NetworkError(t *testing.T) {
	// Test with invalid URL
	_, err := ParseFeed("http://invalid-url-that-does-not-exist.com/feed.xml")
	if err == nil {
		t.Error("Expected error for invalid URL")
	}

	if !strings.Contains(err.Error(), "failed to fetch feed") {
		t.Errorf("Expected 'failed to fetch feed' error, got: %v", err)
	}
}

func TestParseFeed_ServerError(t *testing.T) {
	// Test with server returning error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := ParseFeed(server.URL)
	if err == nil {
		t.Error("Expected error for server error response")
	}
}

func TestParseFeed_InvalidXML(t *testing.T) {
	// Test with invalid XML
	invalidXML := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Invalid XML Podcast</title>
    <description>This XML is malformed
  </channel>
</rss>` // Missing closing tag for description

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(invalidXML))
	}))
	defer server.Close()

	_, err := ParseFeed(server.URL)
	if err == nil {
		t.Error("Expected error for invalid XML")
	}

	if !strings.Contains(err.Error(), "failed to parse RSS") {
		t.Errorf("Expected 'failed to parse RSS' error, got: %v", err)
	}
}

func TestParseFeed_SpecialCharacters(t *testing.T) {
	// Test with special characters and HTML entities
	rssContent := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Special Characters &amp; HTML Entities</title>
    <description>Testing ñ, émojis 🎧, and &lt;HTML&gt; entities</description>
    <item>
      <title>Episode with &quot;Quotes&quot; &amp; Special Chars</title>
      <description>Contains émojis 🎵 and HTML: &lt;b&gt;bold&lt;/b&gt;</description>
      <enclosure url="https://example.com/special-chars-episode.mp3" type="audio/mpeg" length="1024"/>
      <pubDate>Mon, 15 Oct 2023 12:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(rssContent))
	}))
	defer server.Close()

	podcast, err := ParseFeed(server.URL)
	if err != nil {
		t.Fatalf("Failed to parse feed with special characters: %v", err)
	}

	// Verify special characters were handled correctly
	expectedTitle := "Special Characters & HTML Entities"
	if podcast.Title != expectedTitle {
		t.Errorf("Expected title '%s', got '%s'", expectedTitle, podcast.Title)
	}

	if len(podcast.Episodes) != 1 {
		t.Fatalf("Expected 1 episode, got %d", len(podcast.Episodes))
	}

	episode := podcast.Episodes[0]
	if episode.ID == "" {
		t.Error("Episode with special characters should have generated ID")
	}

	// Verify episode title with special characters
	expectedEpisodeTitle := "Episode with \"Quotes\" & Special Chars"
	if episode.Title != expectedEpisodeTitle {
		t.Errorf("Expected episode title '%s', got '%s'", expectedEpisodeTitle, episode.Title)
	}
}

func TestParseFeed_DateFormats(t *testing.T) {
	// Test various date formats that might appear in RSS feeds
	testCases := []struct {
		dateString  string
		shouldParse bool
		description string
	}{
		{"Mon, 15 Oct 2023 12:00:00 GMT", true, "RFC1123Z format"},
		{"Mon, 15 Oct 2023 12:00:00 +0000", true, "RFC1123Z with +0000"},
		{"Mon, 15 Oct 2023 12:00:00", true, "RFC1123 without timezone"},
		{"Mon, 2 Oct 2023 12:00:00 GMT", true, "Single digit day"},
		{"Invalid date format", false, "Invalid format"},
		{"", false, "Empty date"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			rssContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Date Test Podcast</title>
    <description>Testing date parsing</description>
    <item>
      <title>Date Test Episode</title>
      <enclosure url="https://example.com/datetest.mp3" type="audio/mpeg" length="1024"/>
      <pubDate>%s</pubDate>
    </item>
  </channel>
</rss>`, tc.dateString)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/rss+xml")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(rssContent))
			}))
			defer server.Close()

			podcast, err := ParseFeed(server.URL)
			if err != nil {
				t.Fatalf("Failed to parse feed: %v", err)
			}

			if len(podcast.Episodes) != 1 {
				t.Fatalf("Expected 1 episode, got %d", len(podcast.Episodes))
			}

			episode := podcast.Episodes[0]

			// Episode should always have an ID regardless of date parsing
			if episode.ID == "" {
				t.Error("Episode should have ID even with date parsing issues")
			}

			// Check if date was parsed correctly
			if tc.shouldParse {
				if episode.PublishDate.IsZero() {
					t.Errorf("Expected date to be parsed for format: %s", tc.dateString)
				}
			} else {
				if !episode.PublishDate.IsZero() {
					t.Errorf("Expected date parsing to fail for format: %s", tc.dateString)
				}
			}
		})
	}
}

func TestParseRFC2822Date(t *testing.T) {
	// Test the internal date parsing function
	testCases := []struct {
		input    string
		expected time.Time
		hasError bool
	}{
		{
			"Mon, 15 Oct 2023 12:00:00 GMT",
			time.Date(2023, 10, 15, 12, 0, 0, 0, time.UTC),
			false,
		},
		{
			"Mon, 2 Jan 2023 09:30:45 +0000",
			time.Date(2023, 1, 2, 9, 30, 45, 0, time.UTC),
			false,
		},
		{
			"Invalid date",
			time.Time{},
			true,
		},
	}

	for _, tc := range testCases {
		result, err := parseRFC2822Date(tc.input)

		if tc.hasError {
			if err == nil {
				t.Errorf("Expected error for input '%s'", tc.input)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error for input '%s': %v", tc.input, err)
			}

			if !result.Equal(tc.expected) {
				t.Errorf("For input '%s', expected %v, got %v", tc.input, tc.expected, result)
			}
		}
	}
}

func TestParseFeed_IDUniquenessAcrossFeeds(t *testing.T) {
	// Test that same episode URL in different feeds gets different IDs
	episodeURL := "https://example.com/same-episode.mp3"
	pubDate := "Mon, 15 Oct 2023 12:00:00 GMT"

	feed1Content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Podcast 1</title>
    <description>First podcast</description>
    <item>
      <title>Same Episode</title>
      <enclosure url="%s" type="audio/mpeg" length="1024"/>
      <pubDate>%s</pubDate>
    </item>
  </channel>
</rss>`, episodeURL, pubDate)

	feed2Content := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Podcast 2</title>
    <description>Second podcast</description>
    <item>
      <title>Same Episode</title>
      <enclosure url="%s" type="audio/mpeg" length="1024"/>
      <pubDate>%s</pubDate>
    </item>
  </channel>
</rss>`, episodeURL, pubDate)

	// Create servers for both feeds
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(feed1Content))
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(feed2Content))
	}))
	defer server2.Close()

	// Parse both feeds
	podcast1, err := ParseFeed(server1.URL)
	if err != nil {
		t.Fatalf("Failed to parse feed 1: %v", err)
	}

	podcast2, err := ParseFeed(server2.URL)
	if err != nil {
		t.Fatalf("Failed to parse feed 2: %v", err)
	}

	// Both should have one episode
	if len(podcast1.Episodes) != 1 || len(podcast2.Episodes) != 1 {
		t.Fatal("Expected both podcasts to have exactly one episode")
	}

	episode1 := podcast1.Episodes[0]
	episode2 := podcast2.Episodes[0]

	// Episodes should have different IDs because they're from different podcasts
	if episode1.ID == episode2.ID {
		t.Error("Same episode from different podcasts should have different IDs")
	}

	// Both should have valid IDs
	if episode1.ID == "" || episode2.ID == "" {
		t.Error("Both episodes should have generated IDs")
	}
}
