package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/csams/podcast-tui/internal/download"
	"github.com/csams/podcast-tui/internal/models"
	"github.com/gdamore/tcell/v2"
)

type EpisodeListView struct {
	episodes        []*models.Episode
	filteredEpisodes []*models.Episode  // Episodes after search filtering
	matchResults    map[string]MatchResult  // Match results for highlighting
	currentPodcast  *models.Podcast
	downloadManager *download.Manager
	currentEpisode  *models.Episode
	selectedIdx     int
	scrollOffset    int
	screenHeight    int
	searchState     *SearchState
}

func NewEpisodeListView() *EpisodeListView {
	return &EpisodeListView{
		episodes:    []*models.Episode{},
		filteredEpisodes: []*models.Episode{},
		matchResults: make(map[string]MatchResult),
		selectedIdx: 0,
		searchState: NewSearchState(),
	}
}

func (v *EpisodeListView) SetPodcast(podcast *models.Podcast) {
	// Only reset position if switching to a different podcast
	if v.currentPodcast == nil || v.currentPodcast.URL != podcast.URL {
		v.selectedIdx = 0
		v.scrollOffset = 0
		// Clear search when switching podcasts
		v.searchState.Clear()
	}
	
	v.episodes = podcast.Episodes
	v.currentPodcast = podcast
	v.applyFilter()
}

func (v *EpisodeListView) SetDownloadManager(dm *download.Manager) {
	v.downloadManager = dm
}

func (v *EpisodeListView) SetCurrentEpisode(episode *models.Episode) {
	v.currentEpisode = episode
}

func (v *EpisodeListView) GetSelected() *models.Episode {
	// Use filtered episodes if filter is active
	episodes := v.getActiveEpisodes()
	if v.selectedIdx < 0 || v.selectedIdx >= len(episodes) {
		return nil
	}
	return episodes[v.selectedIdx]
}

func (v *EpisodeListView) GetCurrentPodcast() *models.Podcast {
	return v.currentPodcast
}

func (v *EpisodeListView) Draw(s tcell.Screen) {
	w, h := s.Size()
	v.screenHeight = h

	// Calculate space allocation: reserve bottom area for description
	descriptionHeight := 10 // Reserve 10 lines for description (including borders)
	episodeListHeight := h - descriptionHeight
	if episodeListHeight < 5 { // Minimum space for episode list
		episodeListHeight = h - 2
		descriptionHeight = 2
	}

	// Draw episode list header
	drawText(s, 0, 0, tcell.StyleDefault.Bold(true), "Episodes")
	for x := 0; x < w; x++ {
		s.SetContent(x, 1, '─', nil, tcell.StyleDefault)
	}

	// Draw table header
	v.drawTableHeader(s, 2, w)

	// Show search query if active
	if v.searchState.query != "" {
		searchStyle := tcell.StyleDefault.Foreground(tcell.ColorYellow)
		searchText := fmt.Sprintf("Filter: %s (%d matches)", v.searchState.query, len(v.filteredEpisodes))
		drawText(s, w-len(searchText)-2, 0, searchStyle, searchText)
	}

	// Draw episode list as table
	episodes := v.getActiveEpisodes()
	visibleHeight := episodeListHeight - 4 // Account for header row
	for i := 0; i < visibleHeight && i+v.scrollOffset < len(episodes); i++ {
		idx := i + v.scrollOffset
		episode := episodes[idx]

		style := tcell.StyleDefault
		isSelected := idx == v.selectedIdx
		isCurrentEpisode := v.currentEpisode != nil && episode.ID == v.currentEpisode.ID
		
		if isSelected {
			style = style.Background(tcell.ColorDarkBlue).Foreground(tcell.ColorWhite)
		} else if isCurrentEpisode {
			// Highlight currently playing/paused episode with a different color
			style = style.Background(tcell.ColorDarkGreen).Foreground(tcell.ColorWhite)
		}

		// Draw episode row in table format
		v.drawEpisodeRow(s, i+3, w, episode, isSelected, style)
	}

	// Draw description window at the bottom
	if descriptionHeight > 2 {
		v.drawDescriptionWindow(s, episodeListHeight, w, descriptionHeight)
	}
}

func (v *EpisodeListView) HandleKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'j':
			episodes := v.getActiveEpisodes()
			if v.selectedIdx < len(episodes)-1 {
				v.selectedIdx++
				v.ensureVisible()
				return true
			}
		case 'k':
			if v.selectedIdx > 0 {
				v.selectedIdx--
				v.ensureVisible()
				return true
			}
		case 'g':
			v.selectedIdx = 0
			v.scrollOffset = 0
			return true
		case 'G':
			episodes := v.getActiveEpisodes()
			v.selectedIdx = len(episodes) - 1
			v.ensureVisible()
			return true
		}
	}
	return false
}

func (v *EpisodeListView) ensureVisible() {
	episodes := v.getActiveEpisodes()
	if len(episodes) == 0 || v.screenHeight == 0 {
		return
	}

	// Account for description window (10 lines) and headers
	descriptionHeight := 10
	episodeListHeight := v.screenHeight - descriptionHeight
	if episodeListHeight < 5 { // Minimum space for episode list
		episodeListHeight = v.screenHeight - 2
	}

	// Account for episode list header and table header
	visibleHeight := episodeListHeight - 4 // "Episodes" header + separator + table header + padding
	if visibleHeight <= 0 {
		return
	}

	// Center the selection if possible
	targetOffset := v.selectedIdx - visibleHeight/2

	// Apply bounds checking
	maxOffset := len(v.getActiveEpisodes()) - visibleHeight
	if maxOffset < 0 {
		maxOffset = 0
	}

	if targetOffset < 0 {
		v.scrollOffset = 0
	} else if targetOffset > maxOffset {
		v.scrollOffset = maxOffset
	} else {
		v.scrollOffset = targetOffset
	}
}

// HandlePageDown scrolls down by one page (vim Ctrl+F)
func (v *EpisodeListView) HandlePageDown() bool {
	episodes := v.getActiveEpisodes()
	if len(episodes) == 0 || v.screenHeight == 0 {
		return false
	}

	// Calculate visible height accounting for description window
	descriptionHeight := 10
	episodeListHeight := v.screenHeight - descriptionHeight
	if episodeListHeight < 5 {
		episodeListHeight = v.screenHeight - 2
	}
	visibleHeight := episodeListHeight - 4
	if visibleHeight <= 0 {
		return false
	}

	// Page size with one line overlap
	pageSize := visibleHeight - 1
	if pageSize < 1 {
		pageSize = 1
	}

	// Move selection down by page size
	newIdx := v.selectedIdx + pageSize
	if newIdx >= len(v.getActiveEpisodes()) {
		newIdx = len(v.getActiveEpisodes()) - 1
	}

	if newIdx != v.selectedIdx {
		v.selectedIdx = newIdx
		v.ensureVisible()
		return true
	}
	return false
}

// HandlePageUp scrolls up by one page (vim Ctrl+B)
func (v *EpisodeListView) HandlePageUp() bool {
	episodes := v.getActiveEpisodes()
	if len(episodes) == 0 || v.screenHeight == 0 {
		return false
	}

	// Calculate visible height accounting for description window
	descriptionHeight := 10
	episodeListHeight := v.screenHeight - descriptionHeight
	if episodeListHeight < 5 {
		episodeListHeight = v.screenHeight - 2
	}
	visibleHeight := episodeListHeight - 4
	if visibleHeight <= 0 {
		return false
	}

	// Page size with one line overlap
	pageSize := visibleHeight - 1
	if pageSize < 1 {
		pageSize = 1
	}

	// Move selection up by page size
	newIdx := v.selectedIdx - pageSize
	if newIdx < 0 {
		newIdx = 0
	}

	if newIdx != v.selectedIdx {
		v.selectedIdx = newIdx
		v.ensureVisible()
		return true
	}
	return false
}

// drawTableHeader draws the column headers for the episode table
func (v *EpisodeListView) drawTableHeader(s tcell.Screen, y, width int) {
	headerStyle := tcell.StyleDefault.Bold(true).Foreground(tcell.ColorYellow)

	// Calculate column widths
	columns := v.calculateColumnWidths(width)

	// Draw headers with padding
	x := 0
	drawText(s, x, y, headerStyle, "St")
	x += columns.status + 1 // Add 1 space padding

	drawText(s, x, y, headerStyle, "Title")
	x += columns.title + 1 // Add 1 space padding

	drawText(s, x, y, headerStyle, "Date")
	x += columns.date + 1 // Add 1 space padding

	drawText(s, x, y, headerStyle, "Position")
}

// drawEpisodeRow draws a single episode as a table row
func (v *EpisodeListView) drawEpisodeRow(s tcell.Screen, y, width int, episode *models.Episode, selected bool, style tcell.Style) {
	// Calculate column widths
	columns := v.calculateColumnWidths(width)

	// Clear the entire row with the selection style if selected
	if selected {
		for x := 0; x < width; x++ {
			s.SetContent(x, y, ' ', nil, style)
		}
	}

	// Draw selection indicator and download status
	x := 0
	statusText := ""
	if selected {
		statusText = ">"
	} else {
		statusText = " "
	}
	
	// Add download indicator
	downloadIndicator := v.getDownloadIndicator(episode)
	if downloadIndicator != "" {
		statusText += downloadIndicator
	}
	
	v.drawColumnText(s, x, y, columns.status, statusText, style)
	x += columns.status + 1 // Add 1 space padding

	// Draw title with highlighting if search is active
	title := episode.Title
	if v.searchState.query != "" && v.matchResults[episode.ID].Positions != nil {
		v.drawColumnTextWithHighlight(s, x, y, columns.title, title, style, v.matchResults[episode.ID].Positions)
	} else {
		v.drawColumnText(s, x, y, columns.title, title, style)
	}
	x += columns.title + 1 // Add 1 space padding

	// Draw publish date
	publishDate := v.formatPublishDate(episode.PublishDate)
	v.drawColumnText(s, x, y, columns.date, publishDate, style)
	x += columns.date + 1 // Add 1 space padding

	// Draw listening position
	position := v.formatListeningPosition(episode)
	v.drawColumnText(s, x, y, columns.position, position, style)
}

// columnWidths represents the calculated widths for each column
type columnWidths struct {
	status   int
	title    int
	date     int
	position int
}

// calculateColumnWidths calculates optimal column widths based on terminal width
func (v *EpisodeListView) calculateColumnWidths(totalWidth int) columnWidths {
	// Define minimum and preferred column widths
	const (
		statusMin   = 8  // ">[⬇100%] " - widest possible status indicator
		dateMin     = 10 // "2024-01-15"
		positionMin = 12 // "15:30/45:30"
	)

	// Calculate available width after accounting for fixed columns and padding
	const padding = 3 // One space between each of the 4 columns (3 spaces total)
	fixedWidth := statusMin + dateMin + positionMin + padding
	availableForTitle := totalWidth - fixedWidth

	// Ensure minimum title width
	if availableForTitle < 20 {
		availableForTitle = 20
	}

	return columnWidths{
		status:   statusMin,
		title:    availableForTitle,
		date:     dateMin,
		position: positionMin,
	}
}

// drawColumnText draws text within a column, handling truncation
func (v *EpisodeListView) drawColumnText(s tcell.Screen, x, y, width int, text string, style tcell.Style) {
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
func (v *EpisodeListView) drawColumnTextWithHighlight(s tcell.Screen, x, y, width int, text string, style tcell.Style, highlightPositions []int) {
	if width <= 0 {
		return
	}

	// Create highlight map
	highlightMap := make(map[int]bool)
	for _, pos := range highlightPositions {
		highlightMap[pos] = true
	}
	
	highlightStyle := style.Foreground(tcell.ColorYellow).Bold(true)
	if style.Background(tcell.ColorDarkBlue) == style {
		// If selected, use different highlight color
		highlightStyle = style.Foreground(tcell.ColorBlack).Background(tcell.ColorYellow).Bold(true)
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

// formatPublishDate formats a publish date for display
func (v *EpisodeListView) formatPublishDate(publishDate time.Time) string {
	if publishDate.IsZero() {
		return "—"
	}

	now := time.Now()
	if publishDate.Year() == now.Year() {
		return publishDate.Format("Jan 02")
	}
	return publishDate.Format("2006-01-02")
}

// formatListeningPosition formats the listening position with total duration context
func (v *EpisodeListView) formatListeningPosition(episode *models.Episode) string {
	position := episode.Position
	duration := episode.Duration

	if position == 0 {
		if duration > 0 {
			return "0:00/" + v.formatDuration(duration)
		}
		return "—"
	}

	// Format position
	posStr := v.formatDuration(position)

	// Add total duration if available
	if duration > 0 {
		return posStr + "/" + v.formatDuration(duration)
	}

	return posStr
}

// formatDuration formats a time.Duration into a readable string
func (v *EpisodeListView) formatDuration(d time.Duration) string {
	totalSeconds := int(d.Seconds())
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}

// drawDescriptionWindow renders the description window at the bottom of the screen
func (v *EpisodeListView) drawDescriptionWindow(s tcell.Screen, startY, width, height int) {
	// Get selected episode description
	selectedEpisode := v.GetSelected()
	description := ""
	if selectedEpisode != nil {
		description = selectedEpisode.Description
	}

	// Draw separator line
	separatorStyle := tcell.StyleDefault.Foreground(tcell.ColorGray)
	for x := 0; x < width; x++ {
		s.SetContent(x, startY, '─', nil, separatorStyle)
	}

	// Draw description header
	headerStyle := tcell.StyleDefault.Bold(true)
	drawText(s, 0, startY+1, headerStyle, "Description")

	// Draw description content with text wrapping
	if description != "" {
		// Check if we have match positions for this episode's description
		var highlightPositions []int
		if selectedEpisode != nil && v.searchState.query != "" {
			if matchResult, ok := v.matchResults[selectedEpisode.ID]; ok {
				// Only use positions if the match was in the description (not title)
				titleResult := v.searchState.matchWithPositions(selectedEpisode.Title)
				if titleResult.Score < 0 {
					// Title didn't match, so the match must be in description
					highlightPositions = matchResult.Positions
				}
			}
		}

		// Clean up description: remove excessive whitespace and newlines
		cleanDesc := v.cleanDescription(description)

		// Wrap text to fit width with padding
		contentWidth := width - 2 // Leave 1 char padding on each side
		wrappedLines := v.wrapTextWithHighlights(cleanDesc, contentWidth, highlightPositions)

		// Draw description lines (limit to available height)
		descStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite)
		highlightStyle := tcell.StyleDefault.Foreground(tcell.ColorYellow).Bold(true)
		maxLines := height - 3 // Account for separator, header, and padding

		for i, lineData := range wrappedLines {
			if i >= maxLines {
				break
			}
			v.drawLineWithHighlights(s, 1, startY+2+i, contentWidth, descStyle, highlightStyle, lineData)
		}

		// Show truncation indicator if there are more lines
		if len(wrappedLines) > maxLines && maxLines > 0 {
			truncStyle := tcell.StyleDefault.Foreground(tcell.ColorDarkGray)
			drawText(s, 1, startY+1+maxLines, truncStyle, "...")
		}
	} else {
		// Show placeholder when no description available
		placeholderStyle := tcell.StyleDefault.Foreground(tcell.ColorDarkGray)
		drawText(s, 1, startY+2, placeholderStyle, "No description available")
	}
}

// cleanDescription removes excessive whitespace and normalizes the description text
func (v *EpisodeListView) cleanDescription(desc string) string {
	// Replace multiple whitespace characters with single spaces
	desc = strings.ReplaceAll(desc, "\t", " ")
	desc = strings.ReplaceAll(desc, "\r\n", " ")
	desc = strings.ReplaceAll(desc, "\n", " ")
	desc = strings.ReplaceAll(desc, "\r", " ")

	// Replace multiple spaces with single space
	for strings.Contains(desc, "  ") {
		desc = strings.ReplaceAll(desc, "  ", " ")
	}

	return strings.TrimSpace(desc)
}

// wrapText wraps text to fit within the specified width
func (v *EpisodeListView) wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{}
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{}
	}

	var lines []string
	var currentLine strings.Builder

	for _, word := range words {
		// Check if adding this word would exceed the width
		if currentLine.Len() > 0 && currentLine.Len()+1+len(word) > width {
			// Start a new line
			lines = append(lines, currentLine.String())
			currentLine.Reset()
		}

		// Add word to current line
		if currentLine.Len() > 0 {
			currentLine.WriteString(" ")
		}
		currentLine.WriteString(word)

		// Handle very long words that exceed width
		if currentLine.Len() > width {
			lines = append(lines, currentLine.String())
			currentLine.Reset()
		}
	}

	// Add the last line if it has content
	if currentLine.Len() > 0 {
		lines = append(lines, currentLine.String())
	}

	return lines
}

// lineWithHighlights represents a line of text with highlight positions
type lineWithHighlights struct {
	text      string
	positions []int
}

// wrapTextWithHighlights wraps text while preserving highlight positions
func (v *EpisodeListView) wrapTextWithHighlights(text string, width int, highlightPositions []int) []lineWithHighlights {
	if width <= 0 {
		return []lineWithHighlights{}
	}

	// Create a map for quick highlight position lookup
	highlightMap := make(map[int]bool)
	for _, pos := range highlightPositions {
		highlightMap[pos] = true
	}

	// Handle text cleaning - we need to track how positions map after cleaning
	// Since cleanDescription converts multiple spaces to single spaces,
	// we need to map original positions to cleaned positions
	cleanedText := text
	positionMapping := make(map[int]int) // original pos -> cleaned pos
	
	// Simple position mapping (assumes cleanDescription was already called)
	for i := 0; i < len(text); i++ {
		positionMapping[i] = i
	}

	words := strings.Fields(cleanedText)
	if len(words) == 0 {
		return []lineWithHighlights{}
	}

	var lines []lineWithHighlights
	var currentLine strings.Builder
	var currentPositions []int

	// Find word positions in the cleaned text
	wordPositions := make([]int, len(words))
	searchPos := 0
	for i, word := range words {
		idx := strings.Index(cleanedText[searchPos:], word)
		if idx >= 0 {
			wordPositions[i] = searchPos + idx
			searchPos = searchPos + idx + len(word)
		}
	}

	for wordIdx, word := range words {
		wordStartPos := wordPositions[wordIdx]
		
		// Check if adding this word would exceed the width
		if currentLine.Len() > 0 && currentLine.Len()+1+len(word) > width {
			// Start a new line
			lines = append(lines, lineWithHighlights{
				text:      currentLine.String(),
				positions: currentPositions,
			})
			currentLine.Reset()
			currentPositions = nil
		}

		// Add space before word (if not first word)
		lineOffset := currentLine.Len()
		if currentLine.Len() > 0 {
			currentLine.WriteString(" ")
			lineOffset++
		}

		// Add word to current line
		currentLine.WriteString(word)

		// Map highlight positions for this word
		for i := 0; i < len(word); i++ {
			origPos := wordStartPos + i
			if highlightMap[origPos] {
				currentPositions = append(currentPositions, lineOffset+i)
			}
		}

		// Handle very long words that exceed width
		if currentLine.Len() > width {
			lines = append(lines, lineWithHighlights{
				text:      currentLine.String(),
				positions: currentPositions,
			})
			currentLine.Reset()
			currentPositions = nil
		}
	}

	// Add the last line if it has content
	if currentLine.Len() > 0 {
		lines = append(lines, lineWithHighlights{
			text:      currentLine.String(),
			positions: currentPositions,
		})
	}

	return lines
}

// drawLineWithHighlights draws a single line with highlighted positions
func (v *EpisodeListView) drawLineWithHighlights(s tcell.Screen, x, y, maxWidth int, style, highlightStyle tcell.Style, line lineWithHighlights) {
	// Create highlight map for this line
	highlightMap := make(map[int]bool)
	for _, pos := range line.positions {
		highlightMap[pos] = true
	}

	// Draw each character with appropriate style
	for i, r := range line.text {
		if i >= maxWidth {
			break
		}
		
		charStyle := style
		if highlightMap[i] {
			charStyle = highlightStyle
		}
		
		s.SetContent(x+i, y, r, nil, charStyle)
	}
	
	// Pad the rest of the line
	for i := len(line.text); i < maxWidth; i++ {
		s.SetContent(x+i, y, ' ', nil, style)
	}
}

// getDownloadIndicator returns the appropriate download status indicator for an episode
func (v *EpisodeListView) getDownloadIndicator(episode *models.Episode) string {
	if v.downloadManager == nil {
		return ""
	}

	// Check if episode is downloaded using comprehensive check (filesystem + registry)
	podcastTitle := ""
	if v.currentPodcast != nil {
		podcastTitle = v.currentPodcast.Title
	}
	
	if v.downloadManager.IsEpisodeDownloaded(episode, podcastTitle) {
		return "[D]"
	}

	// Only check download manager state if episode is NOT already downloaded
	// and if it's actively downloading or queued
	if v.downloadManager.IsDownloading(episode.ID) {
		if progress, exists := v.downloadManager.GetDownloadProgress(episode.ID); exists {
			switch progress.Status {
			case download.StatusDownloading:
				return fmt.Sprintf("[⬇%.0f%%]", progress.Progress*100)
			case download.StatusQueued:
				return "[⏸]"
			case download.StatusFailed:
				return "[⚠]"
			default:
				return "[⬇]"
			}
		} else {
			// Downloading but no progress yet
			return "[⬇]"
		}
	}

	// Check for failed downloads that aren't currently downloading
	if progress, exists := v.downloadManager.GetDownloadProgress(episode.ID); exists {
		if progress.Status == download.StatusFailed {
			return "[⚠]"
		}
	}

	return ""
}

// getActiveEpisodes returns either filtered episodes or all episodes based on search state
func (v *EpisodeListView) getActiveEpisodes() []*models.Episode {
	if v.searchState.query != "" {
		return v.filteredEpisodes
	}
	return v.episodes
}

// applyFilter filters episodes based on the current search query
func (v *EpisodeListView) applyFilter() {
	// Clear match results
	v.matchResults = make(map[string]MatchResult)
	
	if v.searchState.query == "" {
		v.filteredEpisodes = v.episodes
		v.adjustSelectionAfterFilter()
		return
	}
	
	// Score and filter episodes
	type scoredEpisode struct {
		episode *models.Episode
		score   int
		matchResult MatchResult
	}
	
	var matched []scoredEpisode
	for _, episode := range v.episodes {
		if matches, score, matchResult := v.searchState.MatchEpisodeWithPositions(episode.Title, episode.Description); matches {
			matched = append(matched, scoredEpisode{
				episode: episode, 
				score: score,
				matchResult: matchResult,
			})
		}
	}
	
	// Sort by score (highest first) and maintain original order for equal scores
	sort.SliceStable(matched, func(i, j int) bool {
		return matched[i].score > matched[j].score
	})
	
	// Extract sorted episodes and store match results
	v.filteredEpisodes = make([]*models.Episode, len(matched))
	for i, m := range matched {
		v.filteredEpisodes[i] = m.episode
		// Store match result by episode ID for later highlighting
		v.matchResults[m.episode.ID] = m.matchResult
	}
	
	v.adjustSelectionAfterFilter()
}

// adjustSelectionAfterFilter ensures selection stays valid after filtering
func (v *EpisodeListView) adjustSelectionAfterFilter() {
	episodes := v.getActiveEpisodes()
	if len(episodes) == 0 {
		v.selectedIdx = 0
		v.scrollOffset = 0
		return
	}
	
	// When search query is active, always select the first result
	if v.searchState.query != "" {
		v.selectedIdx = 0
		v.scrollOffset = 0
		return
	}
	
	// When clearing search, try to keep the same episode selected if possible
	if v.selectedIdx < len(v.episodes) {
		prevSelected := v.episodes[v.selectedIdx]
		for i, ep := range episodes {
			if ep.ID == prevSelected.ID {
				v.selectedIdx = i
				v.ensureVisible()
				return
			}
		}
	}
	
	// Otherwise reset to top
	v.selectedIdx = 0
	v.scrollOffset = 0
}

// GetSearchState returns the search state for external access
func (v *EpisodeListView) GetSearchState() *SearchState {
	return v.searchState
}

// UpdateSearch updates the search and applies filtering
func (v *EpisodeListView) UpdateSearch() {
	v.applyFilter()
}
