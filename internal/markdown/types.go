package markdown

// StyleType represents different text styling types
type StyleType int

const (
	StyleNormal StyleType = iota
	StyleBold
	StyleItalic
	StyleCode
	StyleLink
	StyleHeader
)

// StyleRange represents a range of text with a specific style
type StyleRange struct {
	Start int        // Rune position in converted text
	End   int        // Rune position in converted text
	Type  StyleType
}