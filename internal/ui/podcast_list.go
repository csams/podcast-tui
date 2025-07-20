package ui

import (
	"github.com/csams/podcast-tui/internal/models"
	"github.com/gdamore/tcell/v2"
)

type PodcastListView struct {
	podcasts     []*models.Podcast
	selectedIdx  int
	scrollOffset int
	screenHeight int
}

func NewPodcastListView() *PodcastListView {
	return &PodcastListView{
		podcasts:    []*models.Podcast{},
		selectedIdx: 0,
	}
}

func (v *PodcastListView) SetSubscriptions(subs *models.Subscriptions) {
	v.podcasts = subs.Podcasts
	if v.selectedIdx >= len(v.podcasts) {
		v.selectedIdx = len(v.podcasts) - 1
	}
	if v.selectedIdx < 0 {
		v.selectedIdx = 0
	}
}

func (v *PodcastListView) GetSelected() *models.Podcast {
	if v.selectedIdx < 0 || v.selectedIdx >= len(v.podcasts) {
		return nil
	}
	return v.podcasts[v.selectedIdx]
}

func (v *PodcastListView) Draw(s tcell.Screen) {
	w, h := s.Size()
	v.screenHeight = h

	drawText(s, 0, 0, tcell.StyleDefault.Bold(true), "Podcasts")
	for x := 0; x < w; x++ {
		s.SetContent(x, 1, '─', nil, tcell.StyleDefault)
	}

	// Show helpful message if no podcasts
	if len(v.podcasts) == 0 {
		emptyStyle := tcell.StyleDefault.Foreground(tcell.ColorDarkGray)
		drawText(s, 2, 3, emptyStyle, "No podcasts subscribed")
		drawText(s, 2, 5, emptyStyle, "Press 'a' to add a podcast")
		drawText(s, 2, 6, emptyStyle, "or ':add <feed-url>' to add by URL")
		return
	}

	// Draw table header
	v.drawTableHeader(s, 2, w)

	// Draw podcast rows as table
	visibleHeight := h - 4 // Account for header row
	for i := 0; i < visibleHeight && i+v.scrollOffset < len(v.podcasts); i++ {
		idx := i + v.scrollOffset
		podcast := v.podcasts[idx]

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
		if v.selectedIdx < len(v.podcasts)-1 {
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
		v.selectedIdx = len(v.podcasts) - 1
		v.ensureVisible()
		return true
	}
	return false
}

func (v *PodcastListView) ensureVisible() {
	if len(v.podcasts) == 0 || v.screenHeight == 0 {
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
	maxOffset := len(v.podcasts) - visibleHeight
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
	if len(v.podcasts) == 0 || v.screenHeight == 0 {
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
	if newIdx >= len(v.podcasts) {
		newIdx = len(v.podcasts) - 1
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
	if len(v.podcasts) == 0 || v.screenHeight == 0 {
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
