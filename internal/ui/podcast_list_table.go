package ui

import (
	"fmt"
	"time"

	"github.com/csams/podcast-tui/internal/models"
	"github.com/gdamore/tcell/v2"
)

// columnWidths represents the calculated widths for each column
type podcastColumnWidths struct {
	status  int
	title   int
	url     int
	latest  int
	count   int
}

// calculateColumnWidths calculates optimal column widths based on terminal width
func (v *PodcastListView) calculateColumnWidths(totalWidth int) podcastColumnWidths {
	// Define minimum column widths
	const (
		statusMin  = 2   // "> "
		latestMin  = 10  // "2024-01-15"
		countMin   = 8   // "999 eps"
	)

	// Calculate available width after accounting for fixed columns and padding
	const padding = 3 // One space between each of the 4 columns (3 spaces total)
	fixedWidth := statusMin + latestMin + countMin + padding
	
	// Split remaining width between title and URL (60/40 split)
	availableWidth := totalWidth - fixedWidth
	if availableWidth < 40 { // Minimum for title + URL
		availableWidth = 40
	}
	
	titleWidth := int(float64(availableWidth) * 0.6)
	urlWidth := availableWidth - titleWidth
	
	// Ensure minimums
	if titleWidth < 20 {
		titleWidth = 20
	}
	if urlWidth < 20 {
		urlWidth = 20
	}

	return podcastColumnWidths{
		status: statusMin,
		title:  titleWidth,
		url:    urlWidth,
		latest: latestMin,
		count:  countMin,
	}
}

// drawTableHeader draws the column headers for the podcast table
func (v *PodcastListView) drawTableHeader(s tcell.Screen, y, width int) {
	headerStyle := tcell.StyleDefault.Bold(true).Foreground(tcell.ColorYellow)

	// Calculate column widths
	columns := v.calculateColumnWidths(width)

	// Draw headers with padding
	x := 0
	x += columns.status // No header for status column
	
	drawText(s, x, y, headerStyle, "Title")
	x += columns.title + 1

	drawText(s, x, y, headerStyle, "Feed URL")
	x += columns.url + 1

	drawText(s, x, y, headerStyle, "Latest")
	x += columns.latest + 1

	drawText(s, x, y, headerStyle, "Episodes")
}

// drawPodcastRow draws a single podcast as a table row
func (v *PodcastListView) drawPodcastRow(s tcell.Screen, y, width int, podcast *models.Podcast, selected bool, style tcell.Style) {
	// Calculate column widths
	columns := v.calculateColumnWidths(width)

	// Clear the entire row with the selection style if selected
	if selected {
		for x := 0; x < width; x++ {
			s.SetContent(x, y, ' ', nil, style)
		}
	}

	// Draw selection indicator
	x := 0
	statusText := " "
	if selected {
		statusText = ">"
	}
	v.drawColumnText(s, x, y, columns.status, statusText, style)
	x += columns.status

	// Draw title
	v.drawColumnText(s, x, y, columns.title, podcast.Title, style)
	x += columns.title + 1

	// Draw URL (truncated)
	v.drawColumnText(s, x, y, columns.url, podcast.URL, style)
	x += columns.url + 1

	// Draw latest episode date
	latestDate := v.getLatestEpisodeDate(podcast)
	v.drawColumnText(s, x, y, columns.latest, latestDate, style)
	x += columns.latest + 1

	// Draw episode count
	countText := fmt.Sprintf("%d eps", len(podcast.Episodes))
	v.drawColumnText(s, x, y, columns.count, countText, style)
}

// drawColumnText draws text within a column, handling truncation
func (v *PodcastListView) drawColumnText(s tcell.Screen, x, y, width int, text string, style tcell.Style) {
	if width <= 0 {
		return
	}

	// Truncate text if it's too long for the column
	if len(text) > width {
		if width > 3 {
			text = text[:width-3] + "..."
		} else {
			text = text[:width]
		}
	}

	// Draw the text
	for i, r := range text {
		if i >= width {
			break
		}
		s.SetContent(x+i, y, r, nil, style)
	}

	// Pad remaining space in column
	for i := len(text); i < width; i++ {
		s.SetContent(x+i, y, ' ', nil, style)
	}
}

// getLatestEpisodeDate returns the date of the most recent episode
func (v *PodcastListView) getLatestEpisodeDate(podcast *models.Podcast) string {
	if len(podcast.Episodes) == 0 {
		return "—"
	}

	// Find the most recent episode
	var latestDate time.Time
	for _, episode := range podcast.Episodes {
		if episode.PublishDate.After(latestDate) {
			latestDate = episode.PublishDate
		}
	}

	if latestDate.IsZero() {
		return "—"
	}

	// Format date
	now := time.Now()
	if latestDate.Year() == now.Year() {
		return latestDate.Format("Jan 02")
	}
	return latestDate.Format("2006-01-02")
}