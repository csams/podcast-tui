package ui

import (
	"github.com/gdamore/tcell/v2"
)

type HelpDialog struct {
	visible      bool
	scrollOffset int
}

func NewHelpDialog() *HelpDialog {
	return &HelpDialog{
		visible: false,
	}
}

func (h *HelpDialog) Show() {
	h.visible = true
	h.scrollOffset = 0 // Reset scroll when showing
}

func (h *HelpDialog) Hide() {
	h.visible = false
}

func (h *HelpDialog) IsVisible() bool {
	return h.visible
}

func (h *HelpDialog) Draw(s tcell.Screen) {
	if !h.visible {
		return
	}

	w, screenHeight := s.Size()

	// Get help content to calculate required width
	helpLines := h.getHelpContent()
	
	// Calculate required width based on content
	maxLineWidth := 0
	for _, line := range helpLines {
		if len(line) > maxLineWidth {
			maxLineWidth = len(line)
		}
	}
	
	// Add padding for borders and margins
	requiredWidth := maxLineWidth + 4 // 2 for borders, 2 for margins
	dialogWidth := requiredWidth
	if dialogWidth > w-4 { // Leave at least 2 chars margin on each side
		dialogWidth = w - 4
	}
	if dialogWidth < 40 { // Minimum width
		dialogWidth = 40
	}

	// Calculate dialog height (use most of screen but leave some margin)
	maxDialogHeight := screenHeight - 4 // Leave 2 lines margin top/bottom
	dialogHeight := len(helpLines) + 6 // Content + borders + title + padding
	if dialogHeight > maxDialogHeight {
		dialogHeight = maxDialogHeight
	}
	if dialogHeight < 10 { // Minimum height
		dialogHeight = 10
	}

	// Center the dialog
	startX := (w - dialogWidth) / 2
	startY := (screenHeight - dialogHeight) / 2

	// Ensure dialog fits on screen
	if startX < 1 {
		startX = 1
	}
	if startY < 1 {
		startY = 1
	}
	if startX+dialogWidth > w-1 {
		startX = w - dialogWidth - 1
	}
	if startY+dialogHeight > screenHeight-1 {
		startY = screenHeight - dialogHeight - 1
	}

	// Draw dialog background
	dialogStyle := tcell.StyleDefault.Background(ColorBgHighlight).Foreground(ColorFg)
	for y := startY; y < startY+dialogHeight; y++ {
		for x := startX; x < startX+dialogWidth; x++ {
			s.SetContent(x, y, ' ', nil, dialogStyle)
		}
	}

	// Draw border
	borderStyle := tcell.StyleDefault.Background(ColorBgHighlight).Foreground(ColorBorder)

	// Top and bottom border
	for x := startX; x < startX+dialogWidth; x++ {
		if x == startX {
			s.SetContent(x, startY, '┌', nil, borderStyle)
			s.SetContent(x, startY+dialogHeight-1, '└', nil, borderStyle)
		} else if x == startX+dialogWidth-1 {
			s.SetContent(x, startY, '┐', nil, borderStyle)
			s.SetContent(x, startY+dialogHeight-1, '┘', nil, borderStyle)
		} else {
			s.SetContent(x, startY, '─', nil, borderStyle)
			s.SetContent(x, startY+dialogHeight-1, '─', nil, borderStyle)
		}
	}

	// Left and right border
	for y := startY + 1; y < startY+dialogHeight-1; y++ {
		s.SetContent(startX, y, '│', nil, borderStyle)
		s.SetContent(startX+dialogWidth-1, y, '│', nil, borderStyle)
	}

	// Title
	titleStyle := tcell.StyleDefault.Background(ColorBgHighlight).Foreground(ColorHeader).Bold(true)
	title := "Help - Keybindings"
	titleX := startX + (dialogWidth-len(title))/2
	drawText(s, titleX, startY+1, titleStyle, title)

	// (helpLines already declared above for width calculation)

	// Calculate visible content area
	contentStartY := startY + 3
	contentHeight := dialogHeight - 4 // Subtract borders and title
	visibleLines := contentHeight - 1 // Leave room for scroll indicator

	// Draw help content with scrolling
	contentStyle := tcell.StyleDefault.Background(ColorBgHighlight).Foreground(ColorFg)
	for i := 0; i < visibleLines && i+h.scrollOffset < len(helpLines); i++ {
		lineIndex := i + h.scrollOffset
		line := helpLines[lineIndex]
		
		// Truncate line if it's too long for the dialog
		maxContentWidth := dialogWidth - 4 // Account for borders and margins
		if len(line) > maxContentWidth {
			if maxContentWidth > 3 {
				line = line[:maxContentWidth-3] + "..."
			} else {
				line = line[:maxContentWidth]
			}
		}
		
		drawText(s, startX+2, contentStartY+i, contentStyle, line)
	}

	// Draw scroll indicator if content is scrollable
	if len(helpLines) > visibleLines {
		scrollStyle := tcell.StyleDefault.Background(ColorBgHighlight).Foreground(ColorDimmed)
		scrollInfo := ""
		
		if h.scrollOffset > 0 && h.scrollOffset+visibleLines < len(helpLines) {
			scrollInfo = "↑↓ Use j/k or Up/Down to scroll, Esc to close"
		} else if h.scrollOffset > 0 {
			scrollInfo = "↑ Use k or Up to scroll up, Esc to close"
		} else {
			scrollInfo = "↓ Use j or Down to scroll down, Esc to close"
		}
		
		// Center scroll indicator  
		scrollX := startX + (dialogWidth-len(scrollInfo))/2
		if scrollX < startX+2 {
			scrollX = startX + 2
		}
		drawText(s, scrollX, startY+dialogHeight-2, scrollStyle, scrollInfo)
	} else {
		// Standard close message when no scrolling needed
		closeMsg := "Press Esc or ? to close this help dialog"
		scrollStyle := tcell.StyleDefault.Background(ColorBgHighlight).Foreground(ColorDimmed)
		scrollX := startX + (dialogWidth-len(closeMsg))/2
		if scrollX < startX+2 {
			scrollX = startX + 2
		}
		drawText(s, scrollX, startY+dialogHeight-2, scrollStyle, closeMsg)
	}
}

func (h *HelpDialog) HandleKey(ev *tcell.EventKey) bool {
	if !h.visible {
		return false
	}

	switch ev.Key() {
	case tcell.KeyEscape:
		h.Hide()
		return true
	case tcell.KeyUp:
		h.scrollUp()
		return true
	case tcell.KeyDown:
		// Calculate max scroll more precisely
		h.scrollDown()
		return true
	case tcell.KeyRune:
		switch ev.Rune() {
		case '?':
			h.Hide()
			return true
		case 'j':
			// Scroll down
			h.scrollDown()
			return true
		case 'k':
			// Scroll up
			h.scrollUp()
			return true
		case 'g':
			// Go to top
			h.scrollOffset = 0
			return true
		case 'G':
			// Go to bottom
			h.scrollToBottom()
			return true
		}
	}

	return true // Consume all other keys when visible
}

// getHelpContent returns the help text content
func (h *HelpDialog) getHelpContent() []string {
	return []string{
		"",
		"Navigation:",
		"  j / k         Move down/up in lists",
		"  Ctrl+F / B    Page down/up in lists",
		"  h / l         Switch between podcast and episode views",
		"  p             Go to podcast view (from episode/queue view)",
		"  e             Go to episode view (from podcast/queue view)",
		"  q             Go to queue view (from podcast/episode view)",
		"  Tab           Toggle queue view / return to previous view",
		"  Enter         Select item (same as 'l')",
		"  g             Go to top of list",
		"  G             Go to bottom of list",
		"",
		"Episode View:",
		"  Split view with episode list and description window",
		"  Description shows markdown/HTML converted to terminal text",
		"  Alt+j / Alt+k Scroll description down/up",
		"",
		"Episode Status Indicators:",
		"  Q:1, Q:2      Queue position",
		"  ▶             Currently playing episode",
		"  ⏸             Currently paused episode",
		"  ✔             Downloaded episode",
		"  [⬇50%]        Downloading (with progress)",
		"  [⏸]           Download queued",
		"  [⚠]           Download failed",
		"",
		"Search (Podcast/Episode List):",
		"  /             Enter search mode (fuzzy search with highlighting)",
		"  Ctrl+T        Toggle search quality (Normal/Strict/Permissive/All)",
		"  Enter/Esc     Exit search mode (filter stays active)",
		"  Empty search  Clears filter and shows all items",
		"",
		"Search Mode Editing (Emacs-style):",
		"  Ctrl+A/Home   Move cursor to beginning",
		"  Ctrl+E/End    Move cursor to end",
		"  Ctrl+F/Right  Move cursor forward one character",
		"  Ctrl+B/Left   Move cursor backward one character",
		"  Alt+F         Move cursor forward one word",
		"  Alt+B         Move cursor backward one word",
		"  Ctrl+K        Delete from cursor to end",
		"  Ctrl+U        Delete entire search query",
		"  Ctrl+W        Delete word before cursor",
		"  Alt+D         Delete word after cursor",
		"  Ctrl+D/Delete Delete character at cursor",
		"",
		"Queue Management:",
		"  Enter/'l'     Add episode to queue (from episode list)",
		"  u             Remove episode from queue (from episode/queue view)",
		"  Enter         Play episode immediately (from queue view)",
		"  g             Go to episode in episode list (from queue view)",
		"  Alt+j         Move episode down in queue (from queue view)",
		"  Alt+k         Move episode up in queue (from queue view)",
		"  R             Restart episode from beginning (from queue view)",
		"  0-9           Seek to 0%-90% of episode (from queue view)",
		"",
		"  Note: First episode added to empty queue starts playing automatically",
		"  Episodes play sequentially; completed episodes are removed from queue",
		"  Auto-advances to next episode when one completes",
		"",
		"Playback Control:",
		"  Space         Pause/resume current episode",
		"  s             Stop playback",
		"  R             Restart episode from beginning (reset to 0:00)",
		"  f             Seek forward 30 seconds",
		"  b             Seek backward 30 seconds",
		"  Left/Right    Seek backward/forward 10 seconds",
		"  0-9           Seek to 0%-90% of episode duration",
		"  m             Mute/unmute",
		"  < / >         Decrease/increase playback speed",
		"  =             Reset to normal speed (1.0x)",
		"  Up/Down       Increase/decrease volume by 5%",
		"",
		"  Note: Positions are saved automatically and updated in real-time",
		"",
		"Episode Downloads:",
		"  d             Download selected episode",
		"  x             Cancel download or delete episode",
		"",
		"Podcast Management:",
		"  a             Add new podcast (enters command mode)",
		"  x             Delete selected podcast (with confirmation)",
		"  r             Refresh feeds (all in podcast list, current in episode list)",
		"",
		"Other:",
		"  :             Enter command mode",
		"  ?             Show this help dialog",
		"  Esc           Return to normal mode / close dialogs",
		"  Q             Quit application (uppercase Q required)",
		"",
		"Command Mode:",
		"  :add <url>    Add new podcast by RSS feed URL",
		"  :q            Go to queue view (from podcast/episode view)",
		"  :Q or :quit   Quit the application",
		"",
		"Note: When help dialog is scrollable, use j/k or Up/Down to scroll",
		"Press Esc or ? to close this help dialog",
	}
}

// scrollUp scrolls the help content up by one line
func (h *HelpDialog) scrollUp() {
	if h.scrollOffset > 0 {
		h.scrollOffset--
	}
}

// scrollDown scrolls the help content down by one line
func (h *HelpDialog) scrollDown() {
	helpLines := h.getHelpContent()
	// Use a reasonable estimate for visible lines, will be overridden during draw
	visibleLines := 15 // Conservative estimate
	maxScroll := len(helpLines) - visibleLines
	if maxScroll < 0 {
		maxScroll = 0
	}
	if h.scrollOffset < maxScroll {
		h.scrollOffset++
	}
}

// scrollToBottom scrolls to the bottom of the help content
func (h *HelpDialog) scrollToBottom() {
	helpLines := h.getHelpContent()
	// Use a reasonable estimate for visible lines
	visibleLines := 15 // Conservative estimate
	maxScroll := len(helpLines) - visibleLines
	if maxScroll < 0 {
		maxScroll = 0
	}
	h.scrollOffset = maxScroll
}
