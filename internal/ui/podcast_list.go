package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/csams/podcast-tui/internal/markdown"
	"github.com/csams/podcast-tui/internal/models"
	"github.com/gdamore/tcell/v2"
)

// PodcastTableRow adapts a podcast to the TableRow interface
type PodcastTableRow struct {
	podcast     *models.Podcast
	matchResult *PodcastMatchResult
}

func (r *PodcastTableRow) GetCell(columnIndex int) string {
	switch columnIndex {
	case 0: // Status column (selection indicator handled by table)
		return ""
	case 1: // Title
		return r.podcast.Title
	case 2: // URL
		return r.podcast.URL
	case 3: // Latest episode date
		return r.getLatestEpisodeDate()
	case 4: // Episode count
		return fmt.Sprintf("%d eps", len(r.podcast.Episodes))
	default:
		return ""
	}
}

func (r *PodcastTableRow) GetCellStyle(columnIndex int, selected bool) *tcell.Style {
	// No custom cell styles needed for podcasts
	return nil
}

func (r *PodcastTableRow) GetHighlightPositions(columnIndex int) []int {
	if r.matchResult == nil {
		return nil
	}
	
	switch columnIndex {
	case 1: // Title
		if r.matchResult.MatchField == "title" {
			return r.matchResult.Positions
		}
	}
	return nil
}

func (r *PodcastTableRow) getLatestEpisodeDate() string {
	if len(r.podcast.Episodes) == 0 {
		return "—"
	}

	var latestDate time.Time
	for _, episode := range r.podcast.Episodes {
		if episode.PublishDate.After(latestDate) {
			latestDate = episode.PublishDate
		}
	}

	if latestDate.IsZero() {
		return "—"
	}

	localDate := latestDate.Local()
	now := time.Now()
	if localDate.Year() == now.Year() {
		return localDate.Format("Jan 02")
	}
	return localDate.Format("2006-01-02")
}

// PodcastListView is the podcast list using the table abstraction
type PodcastListView struct {
	table            *Table
	podcasts         []*models.Podcast
	filteredPodcasts []*models.Podcast
	matchResults     map[string]PodcastMatchResult
	searchState      *SearchState
	descScrollOffset int
}

func NewPodcastListView() *PodcastListView {
	v := &PodcastListView{
		table:        NewTable(),
		podcasts:     []*models.Podcast{},
		matchResults: make(map[string]PodcastMatchResult),
		searchState:  NewSearchState(),
	}
	
	// Configure table columns
	v.table.SetColumns([]TableColumn{
		{Title: "", Width: 2, Align: AlignLeft},                    // Status
		{Title: "Title", MinWidth: 20, FlexWeight: 0.6, Align: AlignLeft},   // Title
		{Title: "Feed URL", MinWidth: 20, FlexWeight: 0.4, Align: AlignLeft}, // URL
		{Title: "Latest", Width: 10, Align: AlignLeft},            // Latest date
		{Title: "Episodes", Width: 8, Align: AlignLeft},           // Count
	})
	
	return v
}

func (v *PodcastListView) SetSubscriptions(subs *models.Subscriptions) {
	v.podcasts = subs.Podcasts
	v.applyFilter()
}

func (v *PodcastListView) GetSelected() *models.Podcast {
	row := v.table.GetSelectedRow()
	if row != nil {
		if podcastRow, ok := row.(*PodcastTableRow); ok {
			return podcastRow.podcast
		}
	}
	return nil
}

func (v *PodcastListView) Draw(s tcell.Screen) {
	w, h := s.Size()
	
	// Calculate space allocation
	descriptionHeight := 15
	podcastListHeight := h - descriptionHeight
	if podcastListHeight < 5 {
		podcastListHeight = h - 2
		descriptionHeight = 2
	}
	
	// Draw header
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
	
	// Configure and draw table
	v.table.SetPosition(0, 2)
	v.table.SetSize(w, podcastListHeight - 2)
	v.table.Draw(s)
	
	// Show scroll indicator
	if first, last, total := v.table.GetScrollInfo(); total > last-first+1 {
		scrollStyle := tcell.StyleDefault.Foreground(ColorDimmed)
		scrollInfo := fmt.Sprintf("[%d-%d/%d]", first, last, total)
		
		headerText := "Podcasts"
		scrollX := len(headerText) + 2
		if v.searchState.query != "" {
			searchTextLen := len(fmt.Sprintf("Filter: %s (%d matches)", v.searchState.query, len(v.filteredPodcasts))) + 10
			maxScrollX := w - searchTextLen - 2 - len(scrollInfo)
			if scrollX > maxScrollX {
				scrollX = maxScrollX
			}
		}
		drawText(s, scrollX, 0, scrollStyle, scrollInfo)
	}
	
	// Draw description window
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
				v.descScrollOffset++
				return true
			case 'k':
				if v.descScrollOffset > 0 {
					v.descScrollOffset--
				}
				return true
			}
		}
		
		switch ev.Rune() {
		case 'j':
			if v.table.SelectNext() {
				v.descScrollOffset = 0
				return true
			}
		case 'k':
			if v.table.SelectPrevious() {
				v.descScrollOffset = 0
				return true
			}
		case 'g':
			v.table.SelectFirst()
			v.descScrollOffset = 0
			return true
		case 'G':
			v.table.SelectLast()
			v.descScrollOffset = 0
			return true
		}
	}
	return false
}

func (v *PodcastListView) HandlePageDown() bool {
	if v.table.PageDown() {
		v.descScrollOffset = 0
		return true
	}
	return false
}

func (v *PodcastListView) HandlePageUp() bool {
	if v.table.PageUp() {
		v.descScrollOffset = 0
		return true
	}
	return false
}

func (v *PodcastListView) getActivePodcasts() []*models.Podcast {
	if v.searchState.query != "" {
		return v.filteredPodcasts
	}
	return v.podcasts
}

func (v *PodcastListView) applyFilter() {
	v.matchResults = make(map[string]PodcastMatchResult)
	
	if v.searchState.query == "" {
		v.filteredPodcasts = v.podcasts
		v.updateTableRows()
		return
	}
	
	type scoredPodcast struct {
		podcast     *models.Podcast
		score       int
		matchResult MatchResult
		matchField  string
	}
	
	var matched []scoredPodcast
	for _, podcast := range v.podcasts {
		if matches, score, matchResult, matchField := v.searchState.MatchPodcastWithPositions(podcast.Title, podcast.ConvertedDescription); matches {
			matched = append(matched, scoredPodcast{
				podcast:     podcast,
				score:       score,
				matchResult: matchResult,
				matchField:  matchField,
			})
		}
	}
	
	sort.SliceStable(matched, func(i, j int) bool {
		return matched[i].score > matched[j].score
	})
	
	v.filteredPodcasts = make([]*models.Podcast, len(matched))
	for i, m := range matched {
		v.filteredPodcasts[i] = m.podcast
		v.matchResults[m.podcast.URL] = PodcastMatchResult{
			MatchResult: m.matchResult,
			MatchField:  m.matchField,
		}
	}
	
	v.updateTableRows()
}

func (v *PodcastListView) updateTableRows() {
	podcasts := v.getActivePodcasts()
	rows := make([]TableRow, len(podcasts))
	
	for i, podcast := range podcasts {
		var matchResult *PodcastMatchResult
		if mr, ok := v.matchResults[podcast.URL]; ok {
			matchResult = &mr
		}
		
		rows[i] = &PodcastTableRow{
			podcast:     podcast,
			matchResult: matchResult,
		}
	}
	
	v.table.SetRows(rows)
}

func (v *PodcastListView) GetSearchState() *SearchState {
	return v.searchState
}

func (v *PodcastListView) UpdateSearch() {
	v.applyFilter()
}

// drawDescriptionWindow renders the description window at the bottom
func (v *PodcastListView) drawDescriptionWindow(s tcell.Screen, startY, width, height int) {
	// Get the actual screen height to ensure we clear everything
	_, screenHeight := s.Size()
	
	// Clear from startY to the bottom of the screen
	clearStyle := tcell.StyleDefault.Background(ColorBg).Foreground(ColorBg)
	for y := startY; y < screenHeight; y++ {
		for x := 0; x < width; x++ {
			s.SetContent(x, y, ' ', nil, clearStyle)
		}
	}
	
	// Get selected podcast and its converted description
	selectedPodcast := v.GetSelected()
	description := ""
	if selectedPodcast != nil {
		description = selectedPodcast.ConvertedDescription
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

	// Draw description content
	if description != "" {
		var highlightPositions []int
		if selectedPodcast != nil && v.searchState.query != "" {
			if matchResult, ok := v.matchResults[selectedPodcast.URL]; ok && matchResult.MatchField == "description" {
				highlightPositions = matchResult.Positions
			}
		}

		contentWidth := width - 2
		wrappedLines := v.wrapStyledText(description, contentWidth, highlightPositions, nil)

		maxLines := height - 3
		maxScrollOffset := len(wrappedLines) - maxLines
		if maxScrollOffset < 0 {
			maxScrollOffset = 0
		}
		if v.descScrollOffset > maxScrollOffset {
			v.descScrollOffset = maxScrollOffset
		}

		for i := 0; i < maxLines; i++ {
			lineY := startY + 2 + i
			lineIdx := i + v.descScrollOffset
			
			if lineIdx < len(wrappedLines) {
				v.drawStyledLine(s, 1, lineY, contentWidth, wrappedLines[lineIdx])
			}
		}

		if v.descScrollOffset > 0 || len(wrappedLines) > maxLines {
			scrollStyle := tcell.StyleDefault.Foreground(ColorDimmed)
			scrollInfo := fmt.Sprintf("[%d-%d/%d]", v.descScrollOffset+1, 
				min(v.descScrollOffset+maxLines, len(wrappedLines)), len(wrappedLines))
			drawText(s, width-len(scrollInfo)-2, startY+1, scrollStyle, scrollInfo)
		}
	} else {
		placeholderStyle := tcell.StyleDefault.Foreground(ColorDimmed)
		drawText(s, 1, startY+2, placeholderStyle, "No description available")
	}
}

// The following methods are copied from the original implementation to support description window
// In a real refactor, these would be moved to a shared utility

func (v *PodcastListView) wrapStyledText(text string, width int, highlightPositions []int, styles []markdown.StyleRange) []styledLineWithHighlights {
	if width <= 0 {
		return []styledLineWithHighlights{}
	}

	highlightMap := make(map[int]bool)
	for _, pos := range highlightPositions {
		highlightMap[pos] = true
	}

	paragraphs := splitPreservingNewlines(text)
	var allLines []styledLineWithHighlights
	globalRunePos := 0
	
	for paragraphIdx, paragraph := range paragraphs {
		if paragraph == "" {
			allLines = append(allLines, styledLineWithHighlights{
				text:      "",
				positions: nil,
				styles:    nil,
			})
			if paragraphIdx < len(paragraphs)-1 {
				globalRunePos++
			}
			continue
		}
		
		paragraphRunes := []rune(paragraph)
		paragraphLines := v.wrapParagraphWithStyles(paragraph, width, globalRunePos, highlightMap, styles)
		allLines = append(allLines, paragraphLines...)
		
		globalRunePos += len(paragraphRunes)
		if paragraphIdx < len(paragraphs)-1 {
			globalRunePos++
		}
	}

	return allLines
}

func (v *PodcastListView) wrapParagraphWithStyles(paragraph string, width int, paragraphStartPos int, highlightMap map[int]bool, styles []markdown.StyleRange) []styledLineWithHighlights {
	words := strings.Fields(paragraph)
	if len(words) == 0 {
		return []styledLineWithHighlights{}
	}

	var lines []styledLineWithHighlights
	var currentLine strings.Builder
	var currentHighlights []int
	var currentStyles []markdown.StyleRange

	wordPositions := make([]int, len(words))
	runePos := 0
	paragraphRunes := []rune(paragraph)
	
	for i, word := range words {
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
		
		currentLineRuneCount := len([]rune(currentLine.String()))
		wordRuneCount := len([]rune(word))
		if currentLineRuneCount > 0 && currentLineRuneCount+1+wordRuneCount > width {
			lines = append(lines, styledLineWithHighlights{
				text:      currentLine.String(),
				positions: currentHighlights,
				styles:    currentStyles,
			})
			currentLine.Reset()
			currentHighlights = nil
			currentStyles = nil
		}

		lineOffset := len([]rune(currentLine.String()))
		if currentLine.Len() > 0 {
			currentLine.WriteString(" ")
			lineOffset++
		}

		currentLine.WriteString(word)

		wordRunes := []rune(word)
		for i := 0; i < len(wordRunes); i++ {
			globalPos := wordStartPosGlobal + i
			if highlightMap[globalPos] {
				currentHighlights = append(currentHighlights, lineOffset+i)
			}
		}

		for _, style := range styles {
			if style.Start <= wordStartPosGlobal && style.End > wordStartPosGlobal {
				lineStyle := markdown.StyleRange{
					Start: lineOffset,
					End:   lineOffset + wordRuneCount,
					Type:  style.Type,
				}
				
				if style.Start > wordStartPosGlobal {
					lineStyle.Start = lineOffset + (style.Start - wordStartPosGlobal)
				}
				if style.End < wordStartPosGlobal + wordRuneCount {
					lineStyle.End = lineOffset + (style.End - wordStartPosGlobal)
				}
				
				currentStyles = append(currentStyles, lineStyle)
			}
		}

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

	if currentLine.Len() > 0 {
		lines = append(lines, styledLineWithHighlights{
			text:      currentLine.String(),
			positions: currentHighlights,
			styles:    currentStyles,
		})
	}

	return lines
}

func (v *PodcastListView) drawStyledLine(s tcell.Screen, x, y, maxWidth int, line styledLineWithHighlights) {
	highlightMap := make(map[int]bool)
	for _, pos := range line.positions {
		highlightMap[pos] = true
	}

	runes := []rune(line.text)
	defaultStyle := tcell.StyleDefault.Foreground(ColorFg)
	
	screenPos := 0
	for runeIdx, r := range runes {
		if screenPos >= maxWidth {
			break
		}
		
		charStyle := defaultStyle
		
		for _, styleRange := range line.styles {
			if runeIdx >= styleRange.Start && runeIdx < styleRange.End {
				charStyle = GetTcellStyle(styleRange.Type)
				break
			}
		}
		
		if highlightMap[runeIdx] {
			charStyle = charStyle.Foreground(ColorHighlight).Bold(true)
		}
		
		s.SetContent(x+screenPos, y, r, nil, charStyle)
		screenPos++
	}
	
	for i := screenPos; i < maxWidth; i++ {
		s.SetContent(x+i, y, ' ', nil, defaultStyle)
	}
}