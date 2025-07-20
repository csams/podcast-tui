package feed

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch feed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var rss RSS
	if err := xml.Unmarshal(data, &rss); err != nil {
		return nil, fmt.Errorf("failed to parse RSS: %w", err)
	}

	// Try to get the best image URL
	imageURL := rss.Channel.Image.URL
	if imageURL == "" && rss.Channel.ITunesImage.Href != "" {
		imageURL = rss.Channel.ITunesImage.Href
	}
	
	podcast := &models.Podcast{
		Title:       rss.Channel.Title,
		Description: rss.Channel.Description,
		URL:         url,
		ImageURL:    imageURL,
		LastUpdated: time.Now(),
		Episodes:    make([]*models.Episode, 0, len(rss.Channel.Items)),
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
		}

		// Generate unique ID for the episode
		episode.GenerateID(url)

		podcast.Episodes = append(podcast.Episodes, episode)
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
