package ui

import (
	"github.com/gdamore/tcell/v2"
)

// TableColumn defines a column in the table
type TableColumn struct {
	Title      string
	Width      int      // 0 means flexible width
	MinWidth   int      // Minimum width for flexible columns
	MaxWidth   int      // Maximum width for flexible columns (0 = no limit)
	FlexWeight float64  // Weight for distributing available space (0-1)
	Align      Alignment
}

// Alignment specifies text alignment within a cell
type Alignment int

const (
	AlignLeft Alignment = iota
	AlignCenter
	AlignRight
)

// TableRow represents a single row of data
type TableRow interface {
	// GetCell returns the content for a specific column index
	GetCell(columnIndex int) string
	// GetCellStyle returns the style for a specific cell (can return nil for default)
	GetCellStyle(columnIndex int, selected bool) *tcell.Style
	// GetHighlightPositions returns character positions to highlight in a cell
	GetHighlightPositions(columnIndex int) []int
}

// Table is a generic table widget for displaying scrollable data
type Table struct {
	columns      []TableColumn
	rows         []TableRow
	selectedIdx  int
	scrollOffset int
	
	// Display properties
	x, y         int  // Position on screen
	width        int  // Total width available
	height       int  // Total height available
	headerHeight int  // Number of rows for header (usually 1)
	showHeader   bool
	
	// Selection indicator
	selectionIndicator string // e.g., "> "
	
	// Styles
	headerStyle    tcell.Style
	defaultStyle   tcell.Style
	selectedStyle  tcell.Style
	highlightStyle tcell.Style
	
	// Calculated column widths
	columnWidths []int
}

// NewTable creates a new table widget
func NewTable() *Table {
	return &Table{
		columns:            []TableColumn{},
		rows:              []TableRow{},
		selectedIdx:       0,
		scrollOffset:      0,
		headerHeight:      1,
		showHeader:        true,
		selectionIndicator: "> ",
		headerStyle:       tcell.StyleDefault.Bold(true).Foreground(ColorHeader),
		defaultStyle:      tcell.StyleDefault,
		selectedStyle:     tcell.StyleDefault.Background(ColorSelection).Foreground(ColorBright),
		highlightStyle:    tcell.StyleDefault.Foreground(ColorHighlight).Bold(true),
	}
}

// SetColumns sets the column configuration
func (t *Table) SetColumns(columns []TableColumn) {
	t.columns = columns
	t.calculateColumnWidths()
}

// SetRows sets the data rows
func (t *Table) SetRows(rows []TableRow) {
	t.rows = rows
	t.adjustSelection()
}

// SetPosition sets the table's position on screen
func (t *Table) SetPosition(x, y int) {
	t.x = x
	t.y = y
}

// SetSize sets the table's size
func (t *Table) SetSize(width, height int) {
	t.width = width
	t.height = height
	t.calculateColumnWidths()
}

// SetHeaderStyle sets the style for header rows
func (t *Table) SetHeaderStyle(style tcell.Style) {
	t.headerStyle = style
}

// SetSelectedStyle sets the style for selected rows
func (t *Table) SetSelectedStyle(style tcell.Style) {
	t.selectedStyle = style
}

// SetHighlightStyle sets the style for highlighted text
func (t *Table) SetHighlightStyle(style tcell.Style) {
	t.highlightStyle = style
}

// SetSelectionIndicator sets the string used to indicate selection
func (t *Table) SetSelectionIndicator(indicator string) {
	t.selectionIndicator = indicator
}

// GetSelectedIndex returns the currently selected row index
func (t *Table) GetSelectedIndex() int {
	return t.selectedIdx
}

// GetSelectedRow returns the currently selected row
func (t *Table) GetSelectedRow() TableRow {
	if t.selectedIdx >= 0 && t.selectedIdx < len(t.rows) {
		return t.rows[t.selectedIdx]
	}
	return nil
}

// SelectNext moves selection to the next row
func (t *Table) SelectNext() bool {
	if t.selectedIdx < len(t.rows)-1 {
		t.selectedIdx++
		t.ensureVisible()
		return true
	}
	return false
}

// SelectPrevious moves selection to the previous row
func (t *Table) SelectPrevious() bool {
	if t.selectedIdx > 0 {
		t.selectedIdx--
		t.ensureVisible()
		return true
	}
	return false
}

// SelectFirst moves selection to the first row
func (t *Table) SelectFirst() {
	t.selectedIdx = 0
	t.scrollOffset = 0
}

// SelectLast moves selection to the last row
func (t *Table) SelectLast() {
	if len(t.rows) > 0 {
		t.selectedIdx = len(t.rows) - 1
		t.ensureVisible()
	}
}

// PageDown moves selection down by one page
func (t *Table) PageDown() bool {
	visibleHeight := t.getVisibleHeight()
	if visibleHeight <= 0 {
		return false
	}
	
	pageSize := visibleHeight - 1
	if pageSize < 1 {
		pageSize = 1
	}
	
	newIdx := t.selectedIdx + pageSize
	if newIdx >= len(t.rows) {
		newIdx = len(t.rows) - 1
	}
	
	if newIdx != t.selectedIdx {
		t.selectedIdx = newIdx
		t.ensureVisible()
		return true
	}
	return false
}

// PageUp moves selection up by one page
func (t *Table) PageUp() bool {
	visibleHeight := t.getVisibleHeight()
	if visibleHeight <= 0 {
		return false
	}
	
	pageSize := visibleHeight - 1
	if pageSize < 1 {
		pageSize = 1
	}
	
	newIdx := t.selectedIdx - pageSize
	if newIdx < 0 {
		newIdx = 0
	}
	
	if newIdx != t.selectedIdx {
		t.selectedIdx = newIdx
		t.ensureVisible()
		return true
	}
	return false
}

// Draw renders the table to the screen
func (t *Table) Draw(s tcell.Screen) {
	if t.width <= 0 || t.height <= 0 {
		return
	}
	
	// Clear the table area
	t.clear(s)
	
	currentY := t.y
	
	// Draw header if enabled
	if t.showHeader {
		t.drawHeader(s, currentY)
		currentY += t.headerHeight
	}
	
	// Draw rows
	visibleHeight := t.getVisibleHeight()
	for i := 0; i < visibleHeight && i+t.scrollOffset < len(t.rows); i++ {
		rowIdx := i + t.scrollOffset
		row := t.rows[rowIdx]
		isSelected := rowIdx == t.selectedIdx
		
		t.drawRow(s, currentY+i, row, isSelected)
	}
}

// GetScrollInfo returns information about the current scroll position
func (t *Table) GetScrollInfo() (firstVisible, lastVisible, total int) {
	visibleHeight := t.getVisibleHeight()
	firstVisible = t.scrollOffset + 1
	lastVisible = t.scrollOffset + visibleHeight
	if lastVisible > len(t.rows) {
		lastVisible = len(t.rows)
	}
	total = len(t.rows)
	return
}

// private methods

func (t *Table) getVisibleHeight() int {
	height := t.height
	if t.showHeader {
		height -= t.headerHeight
	}
	if height < 0 {
		height = 0
	}
	return height
}

func (t *Table) ensureVisible() {
	visibleHeight := t.getVisibleHeight()
	if visibleHeight <= 0 {
		return
	}
	
	// Center the selection if possible
	targetOffset := t.selectedIdx - visibleHeight/2
	
	// Apply bounds checking
	maxOffset := len(t.rows) - visibleHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	
	if targetOffset < 0 {
		t.scrollOffset = 0
	} else if targetOffset > maxOffset {
		t.scrollOffset = maxOffset
	} else {
		t.scrollOffset = targetOffset
	}
}

func (t *Table) adjustSelection() {
	if len(t.rows) == 0 {
		t.selectedIdx = 0
		t.scrollOffset = 0
		return
	}
	
	if t.selectedIdx >= len(t.rows) {
		t.selectedIdx = len(t.rows) - 1
	}
	if t.selectedIdx < 0 {
		t.selectedIdx = 0
	}
	t.ensureVisible()
}

func (t *Table) calculateColumnWidths() {
	if len(t.columns) == 0 || t.width <= 0 {
		return
	}
	
	t.columnWidths = make([]int, len(t.columns))
	
	// First pass: assign fixed widths and calculate total flex weight
	fixedWidth := 0
	totalFlexWeight := 0.0
	flexColumns := 0
	
	// Account for selection indicator if first column
	indicatorWidth := 0
	if t.selectionIndicator != "" {
		indicatorWidth = len([]rune(t.selectionIndicator))
	}
	
	for i, col := range t.columns {
		if col.Width > 0 {
			// Fixed width column
			width := col.Width
			if i == 0 {
				width += indicatorWidth
			}
			t.columnWidths[i] = width
			fixedWidth += width
		} else {
			// Flexible width column
			flexColumns++
			if col.FlexWeight > 0 {
				totalFlexWeight += col.FlexWeight
			} else {
				totalFlexWeight += 1.0 // Default weight
			}
		}
	}
	
	// Calculate padding between columns
	padding := len(t.columns) - 1
	if padding < 0 {
		padding = 0
	}
	
	// Second pass: distribute remaining width to flexible columns
	availableWidth := t.width - fixedWidth - padding
	if availableWidth > 0 && flexColumns > 0 {
		for i, col := range t.columns {
			if col.Width == 0 {
				// Calculate width based on flex weight
				weight := col.FlexWeight
				if weight <= 0 {
					weight = 1.0
				}
				
				width := int(float64(availableWidth) * (weight / totalFlexWeight))
				
				// Apply min/max constraints
				if col.MinWidth > 0 && width < col.MinWidth {
					width = col.MinWidth
				}
				if col.MaxWidth > 0 && width > col.MaxWidth {
					width = col.MaxWidth
				}
				
				// Add indicator width to first column
				if i == 0 {
					width += indicatorWidth
				}
				
				t.columnWidths[i] = width
			}
		}
	}
}

func (t *Table) clear(s tcell.Screen) {
	for y := 0; y < t.height; y++ {
		for x := 0; x < t.width; x++ {
			s.SetContent(t.x+x, t.y+y, ' ', nil, t.defaultStyle)
		}
	}
}

func (t *Table) drawHeader(s tcell.Screen, y int) {
	x := t.x
	
	for i, col := range t.columns {
		if i > 0 {
			x++ // Add padding between columns
		}
		
		// Skip drawing header for first column if it's just selection indicator
		if i == 0 && t.selectionIndicator != "" {
			// Just move x position
			x += len([]rune(t.selectionIndicator))
		}
		
		if col.Title != "" {
			t.drawText(s, x, y, t.columnWidths[i], col.Title, t.headerStyle, col.Align)
		}
		
		x += t.columnWidths[i]
	}
}

func (t *Table) drawRow(s tcell.Screen, y int, row TableRow, selected bool) {
	// Clear row with selection style if selected
	if selected {
		for x := 0; x < t.width; x++ {
			s.SetContent(t.x+x, y, ' ', nil, t.selectedStyle)
		}
	}
	
	x := t.x
	
	for i, col := range t.columns {
		if i > 0 {
			x++ // Add padding between columns
		}
		
		// Get cell content and style
		content := row.GetCell(i)
		
		// Add selection indicator to first column
		if i == 0 && t.selectionIndicator != "" {
			if selected {
				content = t.selectionIndicator + content
			} else {
				// Add spaces to maintain alignment
				spaces := ""
				for j := 0; j < len([]rune(t.selectionIndicator)); j++ {
					spaces += " "
				}
				content = spaces + content
			}
		}
		
		// Get cell-specific style or use default
		style := t.defaultStyle
		if selected {
			style = t.selectedStyle
		}
		if cellStyle := row.GetCellStyle(i, selected); cellStyle != nil {
			style = *cellStyle
		}
		
		// Check for highlights
		highlights := row.GetHighlightPositions(i)
		if len(highlights) > 0 {
			t.drawTextWithHighlight(s, x, y, t.columnWidths[i], content, style, highlights, col.Align)
		} else {
			t.drawText(s, x, y, t.columnWidths[i], content, style, col.Align)
		}
		
		x += t.columnWidths[i]
	}
}

func (t *Table) drawText(s tcell.Screen, x, y, width int, text string, style tcell.Style, align Alignment) {
	if width <= 0 {
		return
	}
	
	runes := []rune(text)
	displayRunes := runes
	
	// Truncate if too long
	if len(runes) > width {
		if width > 3 {
			displayRunes = append(runes[:width-3], []rune("...")...)
		} else {
			displayRunes = runes[:width]
		}
	}
	
	// Calculate starting position based on alignment
	startX := x
	textWidth := len(displayRunes)
	if textWidth < width {
		switch align {
		case AlignCenter:
			startX = x + (width-textWidth)/2
		case AlignRight:
			startX = x + width - textWidth
		}
	}
	
	// Draw the text
	for i, r := range displayRunes {
		s.SetContent(startX+i, y, r, nil, style)
	}
}

func (t *Table) drawTextWithHighlight(s tcell.Screen, x, y, width int, text string, style tcell.Style, highlights []int, align Alignment) {
	if width <= 0 {
		return
	}
	
	// Create highlight map
	highlightMap := make(map[int]bool)
	for _, pos := range highlights {
		highlightMap[pos] = true
	}
	
	// Determine highlight style
	highlightStyle := t.highlightStyle
	if style.Background(ColorSelection) == style {
		// If selected, use inverted highlight
		highlightStyle = style.Foreground(ColorBgDark).Background(ColorHighlight).Bold(true)
	}
	
	runes := []rune(text)
	displayRunes := runes
	truncated := false
	
	// Truncate if too long
	if len(runes) > width {
		truncated = true
		if width > 3 {
			displayRunes = runes[:width-3]
		} else {
			displayRunes = runes[:width]
		}
	}
	
	// Calculate starting position based on alignment
	startX := x
	textWidth := len(displayRunes)
	if !truncated && textWidth < width {
		switch align {
		case AlignCenter:
			startX = x + (width-textWidth)/2
		case AlignRight:
			startX = x + width - textWidth
		}
	}
	
	// Draw the text with highlights
	for i, r := range displayRunes {
		charStyle := style
		if highlightMap[i] {
			charStyle = highlightStyle
		}
		s.SetContent(startX+i, y, r, nil, charStyle)
	}
	
	// Add ellipsis if truncated
	if truncated && width > 3 {
		ellipsisStart := startX + len(displayRunes)
		for i := 0; i < 3; i++ {
			s.SetContent(ellipsisStart+i, y, '.', nil, style)
		}
	}
}