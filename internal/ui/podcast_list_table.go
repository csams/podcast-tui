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
	const edgePadding = 2 // 1 char padding on left and right edges
	fixedWidth := statusMin + latestMin + countMin + padding + edgePadding
	
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
	headerStyle := tcell.StyleDefault.Bold(true).Foreground(ColorHeader)

	// Calculate column widths
	columns := v.calculateColumnWidths(width)

	// Draw headers with padding
	x := 1 // Start with 1 char padding from left edge
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

	// Draw selection indicator with left padding
	x := 1 // Start with 1 char padding from left edge
	statusText := "  " // Two spaces by default
	if selected {
		statusText = "> " // Selection indicator with space
	}
	v.drawColumnText(s, x, y, columns.status, statusText, style)
	x += columns.status

	// Get match result if search is active
	var matchResult *PodcastMatchResult
	if v.searchState.query != "" {
		if mr, ok := v.matchResults[podcast.URL]; ok {
			matchResult = &mr
		}
	}

	// Draw title
	if matchResult != nil && matchResult.MatchField == "title" {
		v.drawColumnTextWithHighlight(s, x, y, columns.title, podcast.Title, style, matchResult.Positions)
	} else {
		v.drawColumnText(s, x, y, columns.title, podcast.Title, style)
	}
	x += columns.title + 1

	// Draw URL (no highlighting - not searchable)
	v.drawColumnText(s, x, y, columns.url, podcast.URL, style)
	x += columns.url + 1

	// Draw latest episode date (always show date, no highlighting)
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

	// Convert to runes for proper Unicode handling
	runes := []rune(text)
	displayRunes := runes

	// Truncate by rune count if it's too long for the column
	if len(runes) > width {
		if width > 3 {
			displayRunes = append(runes[:width-3], []rune("...")...)
		} else {
			displayRunes = runes[:width]
		}
	}

	// Draw the text
	charPos := 0
	for _, r := range displayRunes {
		if charPos >= width {
			break
		}
		s.SetContent(x+charPos, y, r, nil, style)
		charPos++
	}

	// Pad remaining space in column
	for i := charPos; i < width; i++ {
		s.SetContent(x+i, y, ' ', nil, style)
	}
}

// drawColumnTextWithHighlight draws text with highlighting at specified positions
func (v *PodcastListView) drawColumnTextWithHighlight(s tcell.Screen, x, y, width int, text string, style tcell.Style, highlightPositions []int) {
	if width <= 0 {
		return
	}

	// Create highlight map
	highlightMap := make(map[int]bool)
	for _, pos := range highlightPositions {
		highlightMap[pos] = true
	}
	
	highlightStyle := style.Foreground(ColorHighlight).Bold(true)
	if style.Background(ColorSelection) == style {
		// If selected, use different highlight color
		highlightStyle = style.Foreground(ColorBgDark).Background(ColorHighlight).Bold(true)
	}

	// Convert text to runes for proper Unicode handling
	runes := []rune(text)
	truncated := false
	displayRunes := runes
	
	// Truncate by rune count, not byte count
	if len(runes) > width {
		truncated = true
		if width > 3 {
			displayRunes = runes[:width-3]
		} else {
			displayRunes = runes[:width]
		}
	}

	// Draw the text with highlights
	charPos := 0
	for runeIdx, r := range displayRunes {
		if charPos >= width {
			break
		}
		
		charStyle := style
		if highlightMap[runeIdx] {
			charStyle = highlightStyle
		}
		
		s.SetContent(x+charPos, y, r, nil, charStyle)
		charPos++
	}
	
	// Add ellipsis if truncated
	if truncated && width > 3 {
		for i := 0; i < 3 && charPos < width; i++ {
			s.SetContent(x+charPos, y, '.', nil, style)
			charPos++
		}
	}

	// Pad remaining space in column
	for i := charPos; i < width; i++ {
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

	// Convert to local time and format date
	localDate := latestDate.Local()
	now := time.Now()
	if localDate.Year() == now.Year() {
		return localDate.Format("Jan 02")
	}
	return localDate.Format("2006-01-02")
}