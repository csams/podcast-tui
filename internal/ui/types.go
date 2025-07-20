package ui

import (
	"github.com/csams/podcast-tui/internal/markdown"
	"github.com/gdamore/tcell/v2"
)

// GetTcellStyle converts a markdown StyleType to a tcell Style
func GetTcellStyle(styleType markdown.StyleType) tcell.Style {
	switch styleType {
	case markdown.StyleBold:
		return tcell.StyleDefault.Bold(true)
	case markdown.StyleItalic:
		return tcell.StyleDefault.Italic(true)
	case markdown.StyleCode:
		return tcell.StyleDefault.Foreground(ColorComment)
	case markdown.StyleLink:
		return tcell.StyleDefault.Underline(true)
	case markdown.StyleHeader:
		return tcell.StyleDefault.Bold(true)
	default:
		return tcell.StyleDefault
	}
}