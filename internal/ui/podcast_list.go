package ui

import (
	"fmt"
	"sort"
	"strings"
	
	"github.com/csams/podcast-tui/internal/markdown"
	"github.com/csams/podcast-tui/internal/models"
	"github.com/gdamore/tcell/v2"
)

type PodcastListView struct {
	podcasts         []*models.Podcast
	filteredPodcasts []*models.Podcast  // Podcasts after search filtering
	matchResults     map[string]PodcastMatchResult  // Match results keyed by podcast URL
	selectedIdx      int
	scrollOffset     int
	screenHeight     int
	searchState      *SearchState
	descScrollOffset int  // Scroll offset for description window
}

// PodcastMatchResult stores match result and which field matched
type PodcastMatchResult struct {
	MatchResult
	MatchField string  // "title" or "description"
}

func NewPodcastListView() *PodcastListView {
	return &PodcastListView{
		podcasts:         []*models.Podcast{},
		filteredPodcasts: []*models.Podcast{},
		matchResults:     make(map[string]PodcastMatchResult),
		selectedIdx:      0,
		searchState:      NewSearchState(),
	}
}

func (v *PodcastListView) SetSubscriptions(subs *models.Subscriptions) {
	v.podcasts = subs.Podcasts
	v.applyFilter()
}

func (v *PodcastListView) GetSelected() *models.Podcast {
	// Use filtered podcasts if filter is active
	podcasts := v.getActivePodcasts()
	if v.selectedIdx < 0 || v.selectedIdx >= len(podcasts) {
		return nil
	}
	return podcasts[v.selectedIdx]
}

func (v *PodcastListView) Draw(s tcell.Screen) {
	w, h := s.Size()
	v.screenHeight = h

	// Calculate space allocation: reserve bottom area for description
	descriptionHeight := 15 // Reserve 15 lines for description (including borders)
	podcastListHeight := h - descriptionHeight
	if podcastListHeight < 5 { // Minimum space for podcast list
		podcastListHeight = h - 2
		descriptionHeight = 2
	}

	drawText(s, 0, 0, tcell.StyleDefault.Bold(true), "Podcasts")
	for x := 0; x < w; x++ {
		s.SetContent(x, 1, '─', nil, tcell.StyleDefault)
	}

	// Show search query if active
	if v.searchState.query != "" {
		searchStyle := tcell.StyleDefault.Foreground(ColorHighlight)
		modeText := ""
		switch v.searchState.GetMinScore() {
		case ScoreThresholdStrict:
			modeText = "[Strict] "
		case ScoreThresholdPermissive:
			modeText = "[Permissive] "
		case ScoreThresholdNone:
			modeText = "[All] "
		// Normal mode shows no prefix
		}
		searchText := fmt.Sprintf("%sFilter: %s (%d matches)", modeText, v.searchState.query, len(v.filteredPodcasts))
		drawText(s, w-len(searchText)-2, 0, searchStyle, searchText)
	}

	// Show helpful message if no podcasts
	podcasts := v.getActivePodcasts()
	if len(podcasts) == 0 {
		emptyStyle := tcell.StyleDefault.Foreground(ColorDimmed)
		if v.searchState.query != "" {
			drawText(s, 2, 3, emptyStyle, "No podcasts match your search")
		} else {
			drawText(s, 2, 3, emptyStyle, "No podcasts subscribed")
			drawText(s, 2, 5, emptyStyle, "Press 'a' to add a podcast")
			drawText(s, 2, 6, emptyStyle, "or ':add <feed-url>' to add by URL")
		}
		return
	}

	// Draw table header
	v.drawTableHeader(s, 2, w)

	// Draw podcast rows as table
	visibleHeight := podcastListHeight - 4 // Account for header row
	for i := 0; i < visibleHeight && i+v.scrollOffset < len(podcasts); i++ {
		idx := i + v.scrollOffset
		podcast := podcasts[idx]

		style := tcell.StyleDefault
		if idx == v.selectedIdx {
			style = style.Background(ColorSelection).Foreground(ColorBright)
		}

		// Draw podcast row in table format
		v.drawPodcastRow(s, i+3, w, podcast, idx == v.selectedIdx, style)
	}
	
	// Show scroll indicator if there are more items than visible
	if len(podcasts) > visibleHeight {
		scrollStyle := tcell.StyleDefault.Foreground(ColorDimmed)
		// Calculate visible range
		firstVisible := v.scrollOffset + 1
		lastVisible := v.scrollOffset + visibleHeight
		if lastVisible > len(podcasts) {
			lastVisible = len(podcasts)
		}
		scrollInfo := fmt.Sprintf("[%d-%d/%d]", firstVisible, lastVisible, len(podcasts))
		
		// Position in the title bar, but not overlapping with search info
		scrollX := len("Podcasts") + 2
		if v.searchState.query != "" {
			// If search is active, make sure we don't overlap
			searchTextLen := len(fmt.Sprintf("Filter: %s (%d matches)", v.searchState.query, len(v.filteredPodcasts))) + 10
			maxScrollX := w - searchTextLen - 2 - len(scrollInfo)
			if scrollX > maxScrollX {
				scrollX = maxScrollX
			}
		}
		drawText(s, scrollX, 0, scrollStyle, scrollInfo)
	}
	
	// Draw description window at the bottom
	if descriptionHeight > 2 {
		v.drawDescriptionWindow(s, podcastListHeight, w, descriptionHeight)
	}
}

func (v *PodcastListView) HandleKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyRune:
		// Check for Alt+j and Alt+k
		if ev.Modifiers()&tcell.ModAlt != 0 {
			switch ev.Rune() {
			case 'j':
				// Scroll description down
				v.descScrollOffset++
				return true
			case 'k':
				// Scroll description up
				if v.descScrollOffset > 0 {
					v.descScrollOffset--
				}
				return true
			}
		}
		
		switch ev.Rune() {
		case 'j':
			podcasts := v.getActivePodcasts()
			if v.selectedIdx < len(podcasts)-1 {
				v.selectedIdx++
				v.ensureVisible()
				v.descScrollOffset = 0  // Reset description scroll when changing podcasts
				return true
			}
		case 'k':
			if v.selectedIdx > 0 {
				v.selectedIdx--
				v.ensureVisible()
				v.descScrollOffset = 0  // Reset description scroll when changing podcasts
				return true
			}
		case 'g':
			v.selectedIdx = 0
			v.scrollOffset = 0
			v.descScrollOffset = 0  // Reset description scroll
			return true
		case 'G':
			podcasts := v.getActivePodcasts()
			v.selectedIdx = len(podcasts) - 1
			v.ensureVisible()
			v.descScrollOffset = 0  // Reset description scroll
			return true
		}
	}
	return false
}

func (v *PodcastListView) ensureVisible() {
	podcasts := v.getActivePodcasts()
	if len(podcasts) == 0 || v.screenHeight == 0 {
		return
	}

	// Account for description window (15 lines) and headers
	descriptionHeight := 15
	podcastListHeight := v.screenHeight - descriptionHeight
	if podcastListHeight < 5 { // Minimum space for podcast list
		podcastListHeight = v.screenHeight - 2
	}
	
	// Calculate visible area for podcast list
	visibleHeight := podcastListHeight - 4 // Account for headers and separators
	if visibleHeight <= 0 {
		return
	}

	// Center the selection if possible
	targetOffset := v.selectedIdx - visibleHeight/2

	// Apply bounds checking
	maxOffset := len(v.getActivePodcasts()) - visibleHeight
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
func (v *PodcastListView) HandlePageDown() bool {
	podcasts := v.getActivePodcasts()
	if len(podcasts) == 0 || v.screenHeight == 0 {
		return false
	}

	// Account for description window
	descriptionHeight := 15
	podcastListHeight := v.screenHeight - descriptionHeight
	if podcastListHeight < 5 {
		podcastListHeight = v.screenHeight - 2
	}
	
	visibleHeight := podcastListHeight - 4
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
	if newIdx >= len(v.getActivePodcasts()) {
		newIdx = len(v.getActivePodcasts()) - 1
	}

	if newIdx != v.selectedIdx {
		v.selectedIdx = newIdx
		v.ensureVisible()
		v.descScrollOffset = 0  // Reset description scroll when changing podcasts
		return true
	}
	return false
}

// HandlePageUp scrolls up by one page (vim Ctrl+B)
func (v *PodcastListView) HandlePageUp() bool {
	podcasts := v.getActivePodcasts()
	if len(podcasts) == 0 || v.screenHeight == 0 {
		return false
	}

	// Account for description window
	descriptionHeight := 15
	podcastListHeight := v.screenHeight - descriptionHeight
	if podcastListHeight < 5 {
		podcastListHeight = v.screenHeight - 2
	}
	
	visibleHeight := podcastListHeight - 4
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
		v.descScrollOffset = 0  // Reset description scroll when changing podcasts
		return true
	}
	return false
}

// getActivePodcasts returns either filtered podcasts or all podcasts based on search state
func (v *PodcastListView) getActivePodcasts() []*models.Podcast {
	if v.searchState.query != "" {
		return v.filteredPodcasts
	}
	return v.podcasts
}

// applyFilter filters podcasts based on the current search query
func (v *PodcastListView) applyFilter() {
	// Clear match results
	v.matchResults = make(map[string]PodcastMatchResult)
	
	if v.searchState.query == "" {
		v.filteredPodcasts = v.podcasts
		v.adjustSelectionAfterFilter()
		return
	}
	
	// Score and filter podcasts
	type scoredPodcast struct {
		podcast     *models.Podcast
		score       int
		matchResult MatchResult
		matchField  string
	}
	
	var matched []scoredPodcast
	for _, podcast := range v.podcasts {
		// Use pre-converted description for searching
		if matches, score, matchResult, matchField := v.searchState.MatchPodcastWithPositions(podcast.Title, podcast.ConvertedDescription); matches {
			matched = append(matched, scoredPodcast{
				podcast:     podcast,
				score:       score,
				matchResult: matchResult,
				matchField:  matchField,
			})
		}
	}
	
	// Sort by score (highest first)
	sort.SliceStable(matched, func(i, j int) bool {
		return matched[i].score > matched[j].score
	})
	
	// Extract sorted podcasts and store match results
	v.filteredPodcasts = make([]*models.Podcast, len(matched))
	for i, m := range matched {
		v.filteredPodcasts[i] = m.podcast
		// Store match result by podcast URL (since podcast.ID is not set)
		v.matchResults[m.podcast.URL] = PodcastMatchResult{
			MatchResult: m.matchResult,
			MatchField:  m.matchField,
		}
	}
	
	v.adjustSelectionAfterFilter()
}

// adjustSelectionAfterFilter ensures selection stays valid after filtering
func (v *PodcastListView) adjustSelectionAfterFilter() {
	podcasts := v.getActivePodcasts()
	if len(podcasts) == 0 {
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
	
	// Ensure selection is within bounds
	if v.selectedIdx >= len(podcasts) {
		v.selectedIdx = len(podcasts) - 1
	}
	if v.selectedIdx < 0 {
		v.selectedIdx = 0
	}
	v.ensureVisible()
}

// GetSearchState returns the search state for external access
func (v *PodcastListView) GetSearchState() *SearchState {
	return v.searchState
}

// UpdateSearch updates the search and applies filtering
func (v *PodcastListView) UpdateSearch() {
	v.applyFilter()
}

// drawDescriptionWindow renders the description window at the bottom of the screen
func (v *PodcastListView) drawDescriptionWindow(s tcell.Screen, startY, width, height int) {
	// Get the actual screen height to ensure we clear everything
	_, screenHeight := s.Size()
	
	// Clear from startY to the bottom of the screen (not just the allocated height)
	// Force tcell to update by using different runes and styles
	clearStyle := tcell.StyleDefault.Background(ColorBg).Foreground(ColorBg)
	for y := startY; y < screenHeight; y++ {
		for x := 0; x < width; x++ {
			// First set to a different character to force update
			s.SetContent(x, y, '\u00A0', nil, clearStyle) // Non-breaking space
			// Then set to regular space
			s.SetContent(x, y, ' ', nil, clearStyle)
		}
	}
	
	// Get selected podcast and its converted description
	selectedPodcast := v.GetSelected()
	description := ""
	if selectedPodcast != nil {
		description = selectedPodcast.ConvertedDescription
		// Fall back to original if not converted yet
		if description == "" {
			description = selectedPodcast.Description
		}
	}

	// Draw separator line
	separatorStyle := tcell.StyleDefault.Foreground(ColorFgGutter)
	for x := 0; x < width; x++ {
		s.SetContent(x, startY, '─', nil, separatorStyle)
	}
	
	// Draw description header
	headerStyle := tcell.StyleDefault.Bold(true)
	drawText(s, 0, startY+1, headerStyle, "Description")

	// Draw description content with text wrapping
	if description != "" {
		// Check if we have match positions for this podcast's description
		var highlightPositions []int
		if selectedPodcast != nil && v.searchState.query != "" {
			// Check if the match was in the description
			if matchResult, ok := v.matchResults[selectedPodcast.URL]; ok && matchResult.MatchField == "description" {
				// Use the stored match positions directly
				highlightPositions = matchResult.Positions
			}
		}

		// Wrap text to fit width with padding
		contentWidth := width - 2 // Leave 1 char padding on each side
		// Since we're using pre-converted text, we don't have styles
		// We could re-parse for styles if needed, but for now just display without styles
		wrappedLines := v.wrapStyledText(description, contentWidth, highlightPositions, nil)

		// Draw description lines (limit to available height)
		maxLines := height - 3 // Account for separator, header, and padding

		// Ensure scroll offset doesn't exceed content
		maxScrollOffset := len(wrappedLines) - maxLines
		if maxScrollOffset < 0 {
			maxScrollOffset = 0
		}
		if v.descScrollOffset > maxScrollOffset {
			v.descScrollOffset = maxScrollOffset
		}

		// Draw visible lines based on scroll offset
		for i := 0; i < maxLines; i++ {
			lineY := startY + 2 + i
			lineIdx := i + v.descScrollOffset
			
			// Draw content if available
			if lineIdx < len(wrappedLines) {
				v.drawStyledLine(s, 1, lineY, contentWidth, wrappedLines[lineIdx])
			}
		}

		// Show scroll indicators
		if v.descScrollOffset > 0 || len(wrappedLines) > maxLines {
			scrollStyle := tcell.StyleDefault.Foreground(ColorDimmed)
			scrollInfo := fmt.Sprintf("[%d-%d/%d]", v.descScrollOffset+1, 
				min(v.descScrollOffset+maxLines, len(wrappedLines)), len(wrappedLines))
			drawText(s, width-len(scrollInfo)-2, startY+1, scrollStyle, scrollInfo)
		}
	} else {
		// Show placeholder when no description available
		placeholderStyle := tcell.StyleDefault.Foreground(ColorDimmed)
		drawText(s, 1, startY+2, placeholderStyle, "No description available")
	}
}



// wrapTextWithHighlights wraps text and preserves highlight positions
func (v *PodcastListView) wrapTextWithHighlights(text string, width int, highlightPositions []int) []lineWithHighlights {
	if width <= 0 {
		return []lineWithHighlights{}
	}

	// Create a map for quick highlight position lookup
	highlightMap := make(map[int]bool)
	for _, pos := range highlightPositions {
		highlightMap[pos] = true
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return []lineWithHighlights{}
	}

	var lines []lineWithHighlights
	var currentLine strings.Builder
	var currentPositions []int

	// Find word positions in the text (as rune positions)
	wordPositions := make([]int, len(words))
	runePos := 0
	textRunes := []rune(text)
	
	for i, word := range words {
		// Find the word starting from current position
		wordRunes := []rune(word)
		found := false
		
		for j := runePos; j <= len(textRunes)-len(wordRunes); j++ {
			if string(textRunes[j:j+len(wordRunes)]) == word {
				wordPositions[i] = j
				runePos = j + len(wordRunes)
				found = true
				break
			}
		}
		
		if !found {
			// Shouldn't happen with properly split words
			wordPositions[i] = runePos
		}
	}

	for wordIdx, word := range words {
		wordStartPos := wordPositions[wordIdx]
		
		// Check if adding this word would exceed the width
		currentLineRuneCount := len([]rune(currentLine.String()))
		wordRuneCount := len([]rune(word))
		if currentLineRuneCount > 0 && currentLineRuneCount+1+wordRuneCount > width {
			// Start a new line
			lines = append(lines, lineWithHighlights{
				text:      currentLine.String(),
				positions: currentPositions,
			})
			currentLine.Reset()
			currentPositions = nil
		}

		// Add space before word (if not first word)
		lineOffset := len([]rune(currentLine.String()))
		if currentLine.Len() > 0 {
			currentLine.WriteString(" ")
			lineOffset++
		}

		// Add word to current line
		currentLine.WriteString(word)

		// Map highlight positions for this word
		wordRunes := []rune(word)
		for i := 0; i < len(wordRunes); i++ {
			origPos := wordStartPos + i
			if highlightMap[origPos] {
				currentPositions = append(currentPositions, lineOffset+i)
			}
		}

		// Handle very long words that exceed width
		if len([]rune(currentLine.String())) > width {
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
func (v *PodcastListView) drawLineWithHighlights(s tcell.Screen, x, y, maxWidth int, style, highlightStyle tcell.Style, line lineWithHighlights) {
	// Create highlight map for this line
	highlightMap := make(map[int]bool)
	for _, pos := range line.positions {
		highlightMap[pos] = true
	}

	// Convert to runes for proper positioning
	runes := []rune(line.text)
	
	// Draw each character with appropriate style
	screenPos := 0
	for runeIdx, r := range runes {
		if screenPos >= maxWidth {
			break
		}
		
		charStyle := style
		if highlightMap[runeIdx] {
			charStyle = highlightStyle
		}
		
		s.SetContent(x+screenPos, y, r, nil, charStyle)
		screenPos++
	}
	
	// Pad the rest of the line
	for i := screenPos; i < maxWidth; i++ {
		s.SetContent(x+i, y, ' ', nil, style)
	}
}

// styledLineWithHighlights represents a line with both styling and highlight information
type styledLineWithHighlights struct {
	text      string
	positions []int  // Highlight positions
	styles    []markdown.StyleRange  // Style ranges that apply to this line
}

// splitPreservingNewlines splits text into lines, preserving empty lines
func splitPreservingNewlines(text string) []string {
	// Handle empty string case
	if text == "" {
		return []string{""}
	}
	
	// Split by newline but preserve the structure
	lines := strings.Split(text, "\n")
	return lines
}

// wrapStyledText wraps styled text while preserving both styles and highlight positions
func (v *PodcastListView) wrapStyledText(text string, width int, highlightPositions []int, styles []markdown.StyleRange) []styledLineWithHighlights {
	if width <= 0 {
		return []styledLineWithHighlights{}
	}

	// Create highlight map
	highlightMap := make(map[int]bool)
	for _, pos := range highlightPositions {
		highlightMap[pos] = true
	}

	// Split text by newlines first to preserve paragraph breaks
	paragraphs := splitPreservingNewlines(text)
	var allLines []styledLineWithHighlights
	globalRunePos := 0 // Track position across entire text
	
	for paragraphIdx, paragraph := range paragraphs {
		// Handle empty lines (preserve them)
		if paragraph == "" {
			allLines = append(allLines, styledLineWithHighlights{
				text:      "",
				positions: nil,
				styles:    nil,
			})
			// Account for the newline character if not the last paragraph
			if paragraphIdx < len(paragraphs)-1 {
				globalRunePos++
			}
			continue
		}
		
		// Process non-empty paragraph
		paragraphRunes := []rune(paragraph)
		
		// Wrap this paragraph
		paragraphLines := v.wrapParagraphWithStyles(paragraph, width, globalRunePos, highlightMap, styles)
		allLines = append(allLines, paragraphLines...)
		
		// Update global position
		globalRunePos += len(paragraphRunes)
		// Account for the newline character if not the last paragraph
		if paragraphIdx < len(paragraphs)-1 {
			globalRunePos++
		}
	}

	return allLines
}

// wrapParagraphWithStyles wraps a single paragraph preserving styles and highlights
func (v *PodcastListView) wrapParagraphWithStyles(paragraph string, width int, paragraphStartPos int, highlightMap map[int]bool, styles []markdown.StyleRange) []styledLineWithHighlights {
	words := strings.Fields(paragraph)
	if len(words) == 0 {
		return []styledLineWithHighlights{}
	}

	var lines []styledLineWithHighlights
	var currentLine strings.Builder
	var currentHighlights []int
	var currentStyles []markdown.StyleRange

	// Find word positions in the paragraph (as rune positions)
	wordPositions := make([]int, len(words))
	runePos := 0
	paragraphRunes := []rune(paragraph)
	
	for i, word := range words {
		// Find the word starting from current position
		wordRunes := []rune(word)
		found := false
		
		for j := runePos; j <= len(paragraphRunes)-len(wordRunes); j++ {
			if string(paragraphRunes[j:j+len(wordRunes)]) == word {
				wordPositions[i] = j
				runePos = j + len(wordRunes)
				found = true
				break
			}
		}
		
		if !found {
			wordPositions[i] = runePos
		}
	}

	for wordIdx, word := range words {
		wordStartPosInParagraph := wordPositions[wordIdx]
		wordStartPosGlobal := paragraphStartPos + wordStartPosInParagraph
		
		// Check if adding this word would exceed the width
		currentLineRuneCount := len([]rune(currentLine.String()))
		wordRuneCount := len([]rune(word))
		if currentLineRuneCount > 0 && currentLineRuneCount+1+wordRuneCount > width {
			// Start a new line
			lines = append(lines, styledLineWithHighlights{
				text:      currentLine.String(),
				positions: currentHighlights,
				styles:    currentStyles,
			})
			currentLine.Reset()
			currentHighlights = nil
			currentStyles = nil
		}

		// Add space before word (if not first word)
		lineOffset := len([]rune(currentLine.String()))
		if currentLine.Len() > 0 {
			currentLine.WriteString(" ")
			lineOffset++
		}

		// Add word to current line
		currentLine.WriteString(word)

		// Map highlight positions for this word
		wordRunes := []rune(word)
		for i := 0; i < len(wordRunes); i++ {
			globalPos := wordStartPosGlobal + i
			if highlightMap[globalPos] {
				currentHighlights = append(currentHighlights, lineOffset+i)
			}
		}

		// Find styles that apply to this word
		for _, style := range styles {
			if style.Start <= wordStartPosGlobal && style.End > wordStartPosGlobal {
				// This style applies to at least part of this word
				lineStyle := markdown.StyleRange{
					Start: lineOffset,
					End:   lineOffset + wordRuneCount,
					Type:  style.Type,
				}
				
				// Adjust if style starts or ends within the word
				if style.Start > wordStartPosGlobal {
					lineStyle.Start = lineOffset + (style.Start - wordStartPosGlobal)
				}
				if style.End < wordStartPosGlobal + wordRuneCount {
					lineStyle.End = lineOffset + (style.End - wordStartPosGlobal)
				}
				
				currentStyles = append(currentStyles, lineStyle)
			}
		}

		// Handle very long words that exceed width
		if len([]rune(currentLine.String())) > width {
			lines = append(lines, styledLineWithHighlights{
				text:      currentLine.String(),
				positions: currentHighlights,
				styles:    currentStyles,
			})
			currentLine.Reset()
			currentHighlights = nil
			currentStyles = nil
		}
	}

	// Add the last line if it has content
	if currentLine.Len() > 0 {
		lines = append(lines, styledLineWithHighlights{
			text:      currentLine.String(),
			positions: currentHighlights,
			styles:    currentStyles,
		})
	}

	return lines
}

// drawStyledLine draws a line with both styles and highlights
func (v *PodcastListView) drawStyledLine(s tcell.Screen, x, y, maxWidth int, line styledLineWithHighlights) {
	// Create highlight map for this line
	highlightMap := make(map[int]bool)
	for _, pos := range line.positions {
		highlightMap[pos] = true
	}

	// Convert to runes for proper positioning
	runes := []rune(line.text)
	
	// Default styles
	defaultStyle := tcell.StyleDefault.Foreground(ColorFg)
	
	// Draw each character with appropriate style
	screenPos := 0
	for runeIdx, r := range runes {
		if screenPos >= maxWidth {
			break
		}
		
		// Start with default style
		charStyle := defaultStyle
		
		// Apply any styles that cover this position
		for _, styleRange := range line.styles {
			if runeIdx >= styleRange.Start && runeIdx < styleRange.End {
				charStyle = GetTcellStyle(styleRange.Type)
				break
			}
		}
		
		// Apply highlight if needed (highlight takes precedence)
		if highlightMap[runeIdx] {
			// Preserve existing formatting but add highlight color
			charStyle = charStyle.Foreground(ColorHighlight).Bold(true)
		}
		
		s.SetContent(x+screenPos, y, r, nil, charStyle)
		screenPos++
	}
	
	// Pad the rest of the line
	for i := screenPos; i < maxWidth; i++ {
		s.SetContent(x+i, y, ' ', nil, defaultStyle)
	}
}

