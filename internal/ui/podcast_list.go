package ui

import (
	"fmt"
	"sort"
	
	"github.com/csams/podcast-tui/internal/models"
	"github.com/gdamore/tcell/v2"
)

type PodcastListView struct {
	podcasts         []*models.Podcast
	filteredPodcasts []*models.Podcast  // Podcasts after search filtering
	matchResults     map[string]PodcastMatchResult  // Match results for highlighting
	selectedIdx      int
	scrollOffset     int
	screenHeight     int
	searchState      *SearchState
}

// PodcastMatchResult stores match result and which field matched
type PodcastMatchResult struct {
	MatchResult
	MatchField string  // "title", "url", or "latest"
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

	drawText(s, 0, 0, tcell.StyleDefault.Bold(true), "Podcasts")
	for x := 0; x < w; x++ {
		s.SetContent(x, 1, '─', nil, tcell.StyleDefault)
	}

	// Show search query if active
	if v.searchState.query != "" {
		searchStyle := tcell.StyleDefault.Foreground(tcell.ColorYellow)
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
		emptyStyle := tcell.StyleDefault.Foreground(tcell.ColorDarkGray)
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
	visibleHeight := h - 4 // Account for header row
	for i := 0; i < visibleHeight && i+v.scrollOffset < len(podcasts); i++ {
		idx := i + v.scrollOffset
		podcast := podcasts[idx]

		style := tcell.StyleDefault
		if idx == v.selectedIdx {
			style = style.Background(tcell.ColorDarkBlue).Foreground(tcell.ColorWhite)
		}

		// Draw podcast row in table format
		v.drawPodcastRow(s, i+3, w, podcast, idx == v.selectedIdx, style)
	}
}

func (v *PodcastListView) HandleKey(ev *tcell.EventKey) bool {
	switch ev.Rune() {
	case 'j':
		podcasts := v.getActivePodcasts()
		if v.selectedIdx < len(podcasts)-1 {
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
		podcasts := v.getActivePodcasts()
		v.selectedIdx = len(podcasts) - 1
		v.ensureVisible()
		return true
	}
	return false
}

func (v *PodcastListView) ensureVisible() {
	podcasts := v.getActivePodcasts()
	if len(podcasts) == 0 || v.screenHeight == 0 {
		return
	}

	// Calculate visible area (total height minus header, separator, and status bar)
	visibleHeight := v.screenHeight - 3
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

	visibleHeight := v.screenHeight - 3
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

	visibleHeight := v.screenHeight - 3
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
		// Create description from latest episode info for matching
		description := ""
		if len(podcast.Episodes) > 0 {
			description = podcast.Episodes[0].Title
		}
		
		if matches, score, matchResult, matchField := v.searchState.MatchPodcastWithPositions(podcast.Title, podcast.URL, description); matches {
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
		// Store match result by podcast ID
		v.matchResults[m.podcast.ID] = PodcastMatchResult{
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
