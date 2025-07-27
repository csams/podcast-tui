package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/csams/podcast-tui/internal/download"
	"github.com/csams/podcast-tui/internal/markdown"
	"github.com/csams/podcast-tui/internal/models"
	"github.com/csams/podcast-tui/internal/player"
	"github.com/gdamore/tcell/v2"
)

// EpisodeTableRow adapts an episode to the TableRow interface
type EpisodeTableRow struct {
	episode         *models.Episode
	matchResult     *EpisodeMatchResult
	downloadManager *download.Manager
	currentEpisode  *models.Episode
	player          *player.Player
	podcastTitle    string
	subscriptions   *models.Subscriptions
}

func (r *EpisodeTableRow) GetCell(columnIndex int) string {
	switch columnIndex {
	case 0: // Status/Local column
		return r.getDownloadIndicator()
	case 1: // Title
		return r.episode.Title
	case 2: // Date
		return r.formatPublishDate()
	case 3: // Position
		return r.formatListeningPosition()
	default:
		return ""
	}
}

func (r *EpisodeTableRow) GetCellStyle(columnIndex int, selected bool) *tcell.Style {
	// Check if this is the currently playing/paused episode
	if r.currentEpisode != nil && r.episode.ID == r.currentEpisode.ID && r.player != nil {
		if !selected {
			if r.player.GetState() == player.StatePlaying {
				style := tcell.StyleDefault.Background(ColorBlue7).Foreground(ColorGreen)
				return &style
			} else if r.player.GetState() == player.StatePaused {
				style := tcell.StyleDefault.Background(ColorBlue7).Foreground(ColorYellow)
				return &style
			}
		}
	}
	return nil
}

func (r *EpisodeTableRow) GetHighlightPositions(columnIndex int) []int {
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

func (r *EpisodeTableRow) getDownloadIndicator() string {
	var indicators []string
	
	// Queue position first
	if r.subscriptions != nil {
		if queuePos := r.subscriptions.GetQueuePosition(r.episode.ID); queuePos > 0 {
			indicators = append(indicators, fmt.Sprintf("Q:%d", queuePos))
		}
	}
	
	// Playing/paused indicators
	if r.currentEpisode != nil && r.episode.ID == r.currentEpisode.ID && r.player != nil {
		if r.player.GetState() == player.StatePlaying {
			indicators = append(indicators, "▶")
		} else if r.player.GetState() == player.StatePaused {
			indicators = append(indicators, "⏸")
		}
	}
	
	// Download status
	if r.downloadManager != nil {
		// Check if downloaded
		if r.downloadManager.IsEpisodeDownloaded(r.episode, r.podcastTitle) {
			indicators = append(indicators, "✔")
		} else if r.downloadManager.IsDownloading(r.episode.ID) {
			// Check download progress
			if progress, exists := r.downloadManager.GetDownloadProgress(r.episode.ID); exists {
				switch progress.Status {
				case download.StatusDownloading:
					indicators = append(indicators, fmt.Sprintf("[⬇%.0f%%]", progress.Progress*100))
				case download.StatusQueued:
					indicators = append(indicators, "[⏸]")
				case download.StatusFailed:
					indicators = append(indicators, "[⚠]")
				default:
					indicators = append(indicators, "[⬇]")
				}
			} else {
				indicators = append(indicators, "[⬇]")
			}
		} else {
			// Check for failed downloads
			if progress, exists := r.downloadManager.GetDownloadProgress(r.episode.ID); exists {
				if progress.Status == download.StatusFailed {
					indicators = append(indicators, "[⚠]")
				}
			}
		}
	}

	// Check for notes
	if r.noteExists() {
		indicators = append(indicators, "✎")
	}
	
	// Join all indicators with space
	return strings.Join(indicators, " ")
}

func (r *EpisodeTableRow) formatPublishDate() string {
	if r.episode.PublishDate.IsZero() {
		return "—"
	}

	localDate := r.episode.PublishDate.Local()
	now := time.Now()
	
	if localDate.Year() == now.Year() {
		return localDate.Format("Jan 02")
	}
	return localDate.Format("2006-01-02")
}

func (r *EpisodeTableRow) formatListeningPosition() string {
	position := r.episode.Position
	duration := r.episode.Duration

	if position == 0 {
		if duration > 0 {
			return "0:00/" + formatDuration(duration)
		}
		return "—"
	}

	posStr := formatDuration(position)
	if duration > 0 {
		return posStr + "/" + formatDuration(duration)
	}

	return posStr
}

func (r *EpisodeTableRow) noteExists() bool {
	// Use the shared NoteExists function
	return NoteExists(r.episode, r.podcastTitle, r.downloadManager)
}

// EpisodeListView is the episode list using the table abstraction
type EpisodeListView struct {
	table            *Table
	episodes         []*models.Episode
	filteredEpisodes []*models.Episode
	matchResults     map[string]EpisodeMatchResult
	currentPodcast   *models.Podcast
	downloadManager  *download.Manager
	currentEpisode   *models.Episode
	player           *player.Player
	searchState      *SearchState
	descScrollOffset int
	subscriptions    *models.Subscriptions
}

func NewEpisodeListView() *EpisodeListView {
	v := &EpisodeListView{
		table:        NewTable(),
		episodes:     []*models.Episode{},
		matchResults: make(map[string]EpisodeMatchResult),
		searchState:  NewSearchState(),
	}
	
	// Configure table columns
	v.table.SetColumns([]TableColumn{
		{Title: "Local", Width: 9, Align: AlignLeft},              // Status/Download
		{Title: "Title", MinWidth: 20, FlexWeight: 1.0, Align: AlignLeft}, // Title
		{Title: "Date", Width: 10, Align: AlignLeft},              // Date
		{Title: "Position", Width: 17, Align: AlignLeft},          // Position
	})
	
	return v
}

func (v *EpisodeListView) SetPodcast(podcast *models.Podcast) {
	// Only reset position if switching to a different podcast
	if v.currentPodcast == nil || v.currentPodcast.URL != podcast.URL {
		v.table.SelectFirst()
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
	// Update table rows to reflect new current episode
	v.updateTableRows()
}

func (v *EpisodeListView) SetPlayer(p *player.Player) {
	v.player = p
}

func (v *EpisodeListView) SetSubscriptions(s *models.Subscriptions) {
	v.subscriptions = s
}

func (v *EpisodeListView) UpdateEpisodeDuration(episodeID string, newDuration time.Duration) {
	// Update in main episodes list
	for _, ep := range v.episodes {
		if ep.ID == episodeID {
			ep.Duration = newDuration
			break
		}
	}
	
	// Also update in filtered episodes
	for _, ep := range v.filteredEpisodes {
		if ep.ID == episodeID {
			ep.Duration = newDuration
			break
		}
	}
	
	// Update table rows
	v.updateTableRows()
}

func (v *EpisodeListView) UpdateCurrentEpisodePosition(s tcell.Screen) {
	if v.currentEpisode == nil {
		return
	}

	// Find and update the episode's position
	episodes := v.getActiveEpisodes()
	for _, episode := range episodes {
		if episode.ID == v.currentEpisode.ID {
			episode.Position = v.currentEpisode.Position
			episode.Duration = v.currentEpisode.Duration
			break
		}
	}
	
	// Redraw just the table (this is more efficient than full screen redraw)
	v.table.Draw(s)
}

func (v *EpisodeListView) GetSelected() *models.Episode {
	row := v.table.GetSelectedRow()
	if row != nil {
		if episodeRow, ok := row.(*EpisodeTableRow); ok {
			return episodeRow.episode
		}
	}
	return nil
}

func (v *EpisodeListView) GetCurrentPodcast() *models.Podcast {
	return v.currentPodcast
}

func (v *EpisodeListView) Draw(s tcell.Screen) {
	w, h := s.Size()
	
	// Calculate space allocation
	descriptionHeight := 15
	episodeListHeight := h - descriptionHeight
	if episodeListHeight < 5 {
		episodeListHeight = h - 2
		descriptionHeight = 2
	}
	
	// Draw header with podcast name
	headerText := "Episodes"
	if v.currentPodcast != nil && v.currentPodcast.Title != "" {
		headerText = fmt.Sprintf("Episodes - %s", v.currentPodcast.Title)
	}
	drawText(s, 0, 0, tcell.StyleDefault.Bold(true), headerText)
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
		searchText := fmt.Sprintf("%sFilter: %s (%d matches)", modeText, v.searchState.query, len(v.filteredEpisodes))
		drawText(s, w-len(searchText)-2, 0, searchStyle, searchText)
	}
	
	// Configure and draw table
	v.table.SetPosition(0, 2)
	v.table.SetSize(w, episodeListHeight - 2)
	v.table.Draw(s)
	
	// Show scroll indicator
	if first, last, total := v.table.GetScrollInfo(); total > last-first+1 {
		scrollStyle := tcell.StyleDefault.Foreground(ColorDimmed)
		scrollInfo := fmt.Sprintf("[%d-%d/%d]", first, last, total)
		
		scrollX := len(headerText) + 2
		if v.searchState.query != "" {
			searchTextLen := len(fmt.Sprintf("Filter: %s (%d matches)", v.searchState.query, len(v.filteredEpisodes))) + 10
			maxScrollX := w - searchTextLen - 2 - len(scrollInfo)
			if scrollX > maxScrollX {
				scrollX = maxScrollX
			}
		}
		drawText(s, scrollX, 0, scrollStyle, scrollInfo)
	}
	
	// Draw description window
	if descriptionHeight > 2 {
		v.drawDescriptionWindow(s, episodeListHeight, w, descriptionHeight)
	}
}

func (v *EpisodeListView) HandleKey(ev *tcell.EventKey) bool {
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

func (v *EpisodeListView) HandlePageDown() bool {
	if v.table.PageDown() {
		v.descScrollOffset = 0
		return true
	}
	return false
}

func (v *EpisodeListView) HandlePageUp() bool {
	if v.table.PageUp() {
		v.descScrollOffset = 0
		return true
	}
	return false
}

func (v *EpisodeListView) getActiveEpisodes() []*models.Episode {
	if v.searchState.query != "" {
		return v.filteredEpisodes
	}
	return v.episodes
}

func (v *EpisodeListView) applyFilter() {
	v.matchResults = make(map[string]EpisodeMatchResult)
	
	if v.searchState.query == "" {
		v.filteredEpisodes = v.episodes
		v.updateTableRows()
		return
	}
	
	type scoredEpisode struct {
		episode     *models.Episode
		score       int
		matchResult MatchResult
		matchField  string
	}
	
	var matched []scoredEpisode
	for _, episode := range v.episodes {
		if matches, score, matchResult, matchField := v.searchState.MatchEpisodeWithPositions(episode.Title, episode.ConvertedDescription); matches {
			matched = append(matched, scoredEpisode{
				episode:     episode,
				score:       score,
				matchResult: matchResult,
				matchField:  matchField,
			})
		}
	}
	
	sort.SliceStable(matched, func(i, j int) bool {
		return matched[i].score > matched[j].score
	})
	
	v.filteredEpisodes = make([]*models.Episode, len(matched))
	for i, m := range matched {
		v.filteredEpisodes[i] = m.episode
		v.matchResults[m.episode.ID] = EpisodeMatchResult{
			MatchResult: m.matchResult,
			MatchField:  m.matchField,
		}
	}
	
	v.updateTableRows()
}

func (v *EpisodeListView) updateTableRows() {
	episodes := v.getActiveEpisodes()
	rows := make([]TableRow, len(episodes))
	
	podcastTitle := ""
	if v.currentPodcast != nil {
		podcastTitle = v.currentPodcast.Title
	}
	
	for i, episode := range episodes {
		var matchResult *EpisodeMatchResult
		if mr, ok := v.matchResults[episode.ID]; ok {
			matchResult = &mr
		}
		
		rows[i] = &EpisodeTableRow{
			episode:         episode,
			matchResult:     matchResult,
			downloadManager: v.downloadManager,
			currentEpisode:  v.currentEpisode,
			player:          v.player,
			podcastTitle:    podcastTitle,
			subscriptions:   v.subscriptions,
		}
	}
	
	v.table.SetRows(rows)
}

func (v *EpisodeListView) GetSearchState() *SearchState {
	return v.searchState
}

func (v *EpisodeListView) UpdateSearch() {
	v.applyFilter()
}

// SelectEpisodeByID finds and selects an episode by its ID
func (v *EpisodeListView) SelectEpisodeByID(episodeID string) bool {
	episodes := v.getActiveEpisodes()
	for i, episode := range episodes {
		if episode.ID == episodeID {
			v.table.selectedIdx = i
			v.table.ensureVisible()
			v.descScrollOffset = 0 // Reset description scroll
			return true
		}
	}
	return false
}

// Helper function for duration formatting
func formatDuration(d time.Duration) string {
	totalSeconds := int(d.Seconds())
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}

// drawDescriptionWindow and related methods are similar to PodcastListViewV2
func (v *EpisodeListView) drawDescriptionWindow(s tcell.Screen, startY, width, height int) {
	_, screenHeight := s.Size()
	
	clearStyle := tcell.StyleDefault.Background(ColorBg).Foreground(ColorBg)
	for y := startY; y < screenHeight; y++ {
		for x := 0; x < width; x++ {
			s.SetContent(x, y, ' ', nil, clearStyle)
		}
	}
	
	selectedEpisode := v.GetSelected()
	description := ""
	if selectedEpisode != nil {
		description = selectedEpisode.ConvertedDescription
		if description == "" {
			description = selectedEpisode.Description
		}
	}

	separatorStyle := tcell.StyleDefault.Foreground(ColorFgGutter)
	for x := 0; x < width; x++ {
		s.SetContent(x, startY, '─', nil, separatorStyle)
	}
	
	headerStyle := tcell.StyleDefault.Bold(true)
	drawText(s, 0, startY+1, headerStyle, "Description")

	if description != "" {
		var highlightPositions []int
		if selectedEpisode != nil && v.searchState.query != "" {
			if matchResult, ok := v.matchResults[selectedEpisode.ID]; ok && matchResult.MatchField == "description" {
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

// Reuse text wrapping methods from PodcastListViewV2
func (v *EpisodeListView) wrapStyledText(text string, width int, highlightPositions []int, styles []markdown.StyleRange) []styledLineWithHighlights {
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

func (v *EpisodeListView) wrapParagraphWithStyles(paragraph string, width int, paragraphStartPos int, highlightMap map[int]bool, styles []markdown.StyleRange) []styledLineWithHighlights {
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

func (v *EpisodeListView) drawStyledLine(s tcell.Screen, x, y, maxWidth int, line styledLineWithHighlights) {
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