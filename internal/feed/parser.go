package feed

import (
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/csams/podcast-tui/internal/markdown"
	"github.com/csams/podcast-tui/internal/models"
)

type RSS struct {
	XMLName xml.Name `xml:"rss"`
	Channel Channel  `xml:"channel"`
}

type Channel struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
	Link        string `xml:"link"`
	Image       Image  `xml:"image"`
	ITunesImage ITunesImage `xml:"itunes:image"`
	Items       []Item `xml:"item"`
}

type ITunesImage struct {
	Href string `xml:"href,attr"`
}

type Image struct {
	URL string `xml:"url"`
}

type Item struct {
	Title         string    `xml:"title"`
	Description   string    `xml:"description"`
	Enclosure     Enclosure `xml:"enclosure"`
	PubDate       string    `xml:"pubDate"`
	ITunesDuration string   `xml:"itunes:duration"`
	Duration      string    `xml:"duration"`
}

type Enclosure struct {
	URL    string `xml:"url,attr"`
	Type   string `xml:"type,attr"`
	Length string `xml:"length,attr"`
}

func ParseFeed(url string) (*models.Podcast, error) {
	// Create custom HTTP client with Firefox user agent
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("Feed parser: Failed to create request for URL %s: %v", url, err)
		return nil, fmt.Errorf("failed to create request for %s: %w", url, err)
	}
	
	// Set Firefox user agent
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux i686; rv:141.0) Gecko/20100101 Firefox/141.0")
	
	log.Printf("Feed parser: Fetching feed from %s", url)
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Feed parser: HTTP request failed for %s: %v", url, err)
		return nil, fmt.Errorf("failed to fetch feed from %s: %w", url, err)
	}
	defer resp.Body.Close()
	
	// Log response details
	log.Printf("Feed parser: Response for %s - Status: %s, Content-Type: %s, Content-Length: %s",
		url, resp.Status, resp.Header.Get("Content-Type"), resp.Header.Get("Content-Length"))
	
	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		log.Printf("Feed parser: Non-OK status code %d for %s", resp.StatusCode, url)
		return nil, fmt.Errorf("server returned status %d for %s", resp.StatusCode, url)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Feed parser: Failed to read response body for %s: %v", url, err)
		return nil, fmt.Errorf("failed to read response from %s: %w", url, err)
	}
	
	log.Printf("Feed parser: Read %d bytes from %s", len(data), url)

	var rss RSS
	if err := xml.Unmarshal(data, &rss); err != nil {
		// Log first 500 bytes of response for debugging
		sample := string(data)
		if len(sample) > 500 {
			sample = sample[:500] + "..."
		}
		log.Printf("Feed parser: XML parsing failed for %s: %v\nFirst 500 bytes: %s", url, err, sample)
		return nil, fmt.Errorf("failed to parse RSS from %s: %w", url, err)
	}

	// Try to get the best image URL
	imageURL := rss.Channel.Image.URL
	if imageURL == "" && rss.Channel.ITunesImage.Href != "" {
		imageURL = rss.Channel.ITunesImage.Href
	}
	
	// Create markdown converter
	converter := markdown.NewMarkdownConverter()
	
	podcast := &models.Podcast{
		Title:       rss.Channel.Title,
		Description: rss.Channel.Description,
		URL:         url,
		ImageURL:    imageURL,
		LastUpdated: time.Now(),
		Episodes:    make([]*models.Episode, 0, len(rss.Channel.Items)),
	}
	
	// Convert podcast description
	if podcast.Description != "" {
		result := converter.Convert(podcast.Description)
		podcast.ConvertedDescription = result.Text
	}

	for _, item := range rss.Channel.Items {
		// Try to get duration from iTunes namespace first, then fallback to plain duration
		duration := item.ITunesDuration
		if duration == "" {
			duration = item.Duration
		}
		
		episode := &models.Episode{
			Title:       item.Title,
			Description: item.Description,
			URL:         item.Enclosure.URL,
			Duration:    parseDuration(duration),
		}

		if pubDate, err := parseRFC2822Date(item.PubDate); err == nil {
			episode.PublishDate = pubDate
		} else if item.PubDate != "" {
			log.Printf("Feed parser: Warning - Failed to parse date '%s' for episode '%s' in feed %s: %v",
				item.PubDate, item.Title, url, err)
		}

		// Generate unique ID for the episode
		episode.GenerateID(url)
		
		// Convert episode description
		if episode.Description != "" {
			result := converter.Convert(episode.Description)
			episode.ConvertedDescription = result.Text
		}

		podcast.Episodes = append(podcast.Episodes, episode)
	}

	// Log successful parsing
	log.Printf("Feed parser: Successfully parsed feed from %s - Title: %s, Episodes: %d",
		url, podcast.Title, len(podcast.Episodes))
	
	// Warn about potential issues
	if podcast.Title == "" {
		log.Printf("Feed parser: Warning - Empty title for feed %s", url)
	}
	if len(podcast.Episodes) == 0 {
		log.Printf("Feed parser: Warning - No episodes found in feed %s", url)
	}
	
	return podcast, nil
}

func parseRFC2822Date(dateStr string) (time.Time, error) {
	layouts := []string{
		time.RFC1123Z,
		time.RFC1123,
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 -0700",
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", dateStr)
}

// parseDuration converts various duration formats to time.Duration
func parseDuration(duration string) time.Duration {
	if duration == "" {
		return 0
	}
	
	// Try to parse as seconds first (most common case)
	if seconds, err := strconv.Atoi(duration); err == nil {
		return time.Duration(seconds) * time.Second
	}
	
	// Try to parse as HH:MM:SS or MM:SS format
	if strings.Contains(duration, ":") {
		return parseTimeFormatDuration(duration)
	}
	
	// If we can't parse it, return 0
	return 0
}

// parseTimeFormatDuration parses HH:MM:SS or MM:SS format into time.Duration
func parseTimeFormatDuration(timeStr string) time.Duration {
	parts := strings.Split(timeStr, ":")
	
	var hours, minutes, seconds int
	var err error
	
	switch len(parts) {
	case 2: // MM:SS format
		if minutes, err = strconv.Atoi(parts[0]); err != nil {
			return 0
		}
		if seconds, err = strconv.Atoi(parts[1]); err != nil {
			return 0
		}
	case 3: // HH:MM:SS format
		if hours, err = strconv.Atoi(parts[0]); err != nil {
			return 0
		}
		if minutes, err = strconv.Atoi(parts[1]); err != nil {
			return 0
		}
		if seconds, err = strconv.Atoi(parts[2]); err != nil {
			return 0
		}
	default:
		return 0
	}
	
	return time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second
}
