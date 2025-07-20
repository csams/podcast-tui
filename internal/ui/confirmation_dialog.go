package ui

import (
	"github.com/gdamore/tcell/v2"
)

type ConfirmationDialog struct {
	visible bool
	title   string
	message string
	onYes   func()
	onNo    func()
}

func NewConfirmationDialog() *ConfirmationDialog {
	return &ConfirmationDialog{
		visible: false,
	}
}

func (c *ConfirmationDialog) Show(title, message string, onYes, onNo func()) {
	c.visible = true
	c.title = title
	c.message = message
	c.onYes = onYes
	c.onNo = onNo
}

func (c *ConfirmationDialog) Hide() {
	c.visible = false
	c.title = ""
	c.message = ""
	c.onYes = nil
	c.onNo = nil
}

func (c *ConfirmationDialog) IsVisible() bool {
	return c.visible
}

func (c *ConfirmationDialog) Draw(s tcell.Screen) {
	if !c.visible {
		return
	}

	w, screenHeight := s.Size()

	// Calculate dialog dimensions and position
	dialogWidth := 50
	dialogHeight := 8
	startX := (w - dialogWidth) / 2
	startY := (screenHeight - dialogHeight) / 2

	// Ensure dialog fits on screen
	if startX < 0 {
		startX = 0
		dialogWidth = w
	}
	if startY < 0 {
		startY = 0
		dialogHeight = screenHeight
	}
	if startX+dialogWidth > w {
		dialogWidth = w - startX
	}
	if startY+dialogHeight > screenHeight {
		dialogHeight = screenHeight - startY
	}

	// Draw dialog background
	dialogStyle := tcell.StyleDefault.Background(tcell.ColorDarkRed).Foreground(tcell.ColorWhite)
	for y := startY; y < startY+dialogHeight; y++ {
		for x := startX; x < startX+dialogWidth; x++ {
			s.SetContent(x, y, ' ', nil, dialogStyle)
		}
	}

	// Draw border
	borderStyle := tcell.StyleDefault.Background(tcell.ColorDarkRed).Foreground(tcell.ColorWhite)

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
	titleStyle := tcell.StyleDefault.Background(tcell.ColorDarkRed).Foreground(tcell.ColorYellow).Bold(true)
	titleX := startX + (dialogWidth-len(c.title))/2
	if titleX < startX+2 {
		titleX = startX + 2
	}
	drawText(s, titleX, startY+1, titleStyle, c.title)

	// Message
	messageStyle := tcell.StyleDefault.Background(tcell.ColorDarkRed).Foreground(tcell.ColorWhite)
	messageLines := wrapText(c.message, dialogWidth-4)
	for i, line := range messageLines {
		if i+3 >= dialogHeight-2 {
			break
		}
		drawText(s, startX+2, startY+3+i, messageStyle, line)
	}

	// Buttons
	buttonStyle := tcell.StyleDefault.Background(tcell.ColorDarkRed).Foreground(tcell.ColorWhite).Bold(true)
	buttonsY := startY + dialogHeight - 2

	// "Y" button
	yButtonX := startX + dialogWidth/2 - 6
	drawText(s, yButtonX, buttonsY, buttonStyle, "[Y]es")

	// "N" button
	nButtonX := startX + dialogWidth/2 + 2
	drawText(s, nButtonX, buttonsY, buttonStyle, "[N]o")
}

func (c *ConfirmationDialog) HandleKey(ev *tcell.EventKey) bool {
	if !c.visible {
		return false
	}

	switch ev.Key() {
	case tcell.KeyEscape:
		if c.onNo != nil {
			c.onNo()
		}
		c.Hide()
		return true
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'y', 'Y':
			if c.onYes != nil {
				c.onYes()
			}
			c.Hide()
			return true
		case 'n', 'N':
			if c.onNo != nil {
				c.onNo()
			}
			c.Hide()
			return true
		}
	}

	return true // Consume all other keys when visible
}

// wrapText wraps text to fit within the specified width
func wrapText(text string, width int) []string {
	if len(text) <= width {
		return []string{text}
	}

	var lines []string
	for len(text) > width {
		// Find the last space before width
		breakPoint := width
		for i := width - 1; i >= 0; i-- {
			if text[i] == ' ' {
				breakPoint = i
				break
			}
		}

		lines = append(lines, text[:breakPoint])
		text = text[breakPoint:]
		if len(text) > 0 && text[0] == ' ' {
			text = text[1:] // Remove leading space
		}
	}

	if len(text) > 0 {
		lines = append(lines, text)
	}

	return lines
}
