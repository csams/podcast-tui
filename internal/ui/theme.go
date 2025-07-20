package ui

import "github.com/gdamore/tcell/v2"

// TokyoNight color palette
var (
	// Background colors
	ColorBg         = tcell.NewRGBColor(0x1a, 0x1b, 0x26) // #1a1b26 - Dark background
	ColorBgDark     = tcell.NewRGBColor(0x16, 0x16, 0x1e) // #16161e - Darker background
	ColorBgHighlight = tcell.NewRGBColor(0x29, 0x2e, 0x42) // #292e42 - Highlighted background
	
	// Foreground colors
	ColorFg         = tcell.NewRGBColor(0xc0, 0xca, 0xf5) // #c0caf5 - Default text
	ColorFgDark     = tcell.NewRGBColor(0x56, 0x5f, 0x89) // #565f89 - Dimmed text
	ColorFgGutter   = tcell.NewRGBColor(0x3b, 0x42, 0x61) // #3b4261 - Gutter/border
	
	// Accent colors
	ColorBlue       = tcell.NewRGBColor(0x7a, 0xa2, 0xf7) // #7aa2f7 - Primary blue
	ColorBlue1      = tcell.NewRGBColor(0x2a, 0xc3, 0xde) // #2ac3de - Light blue/cyan
	ColorBlue2      = tcell.NewRGBColor(0x0d, 0xb9, 0xd7) // #0db9d7 - Cyan
	ColorBlue5      = tcell.NewRGBColor(0x89, 0xdd, 0xff) // #89ddff - Bright cyan
	ColorBlue6      = tcell.NewRGBColor(0xb4, 0xf9, 0xf8) // #b4f9f8 - Very light cyan
	ColorBlue7      = tcell.NewRGBColor(0x39, 0x4b, 0x70) // #394b70 - Dark blue
	
	ColorCyan       = tcell.NewRGBColor(0x7d, 0xcf, 0xff) // #7dcfff - Cyan
	ColorGreen      = tcell.NewRGBColor(0x9e, 0xce, 0x6a) // #9ece6a - Green
	ColorGreen1     = tcell.NewRGBColor(0x73, 0xda, 0xca) // #73daca - Teal
	ColorGreen2     = tcell.NewRGBColor(0x41, 0xa6, 0xb5) // #41a6b5 - Dark teal
	ColorMagenta    = tcell.NewRGBColor(0xbb, 0x9a, 0xf7) // #bb9af7 - Purple/Magenta
	ColorMagenta2   = tcell.NewRGBColor(0xff, 0x00, 0x7c) // #ff007c - Bright magenta
	ColorOrange     = tcell.NewRGBColor(0xff, 0x9e, 0x64) // #ff9e64 - Orange
	ColorPurple     = tcell.NewRGBColor(0x9d, 0x7c, 0xd8) // #9d7cd8 - Purple
	ColorRed        = tcell.NewRGBColor(0xf7, 0x76, 0x8e) // #f7768e - Red
	ColorRed1       = tcell.NewRGBColor(0xdb, 0x4b, 0x4b) // #db4b4b - Dark red
	ColorTeal       = tcell.NewRGBColor(0x1a, 0xbc, 0x9c) // #1abc9c - Teal
	ColorYellow     = tcell.NewRGBColor(0xe0, 0xaf, 0x68) // #e0af68 - Yellow
	
	// Special colors
	ColorComment    = tcell.NewRGBColor(0x56, 0x5f, 0x89) // #565f89 - Comments
	ColorBlack      = tcell.NewRGBColor(0x41, 0x4b, 0x68) // #414868 - Black (lighter than bg)
	ColorBorder     = tcell.NewRGBColor(0x29, 0x2e, 0x42) // #292e42 - Borders
	
	// UI-specific color mappings
	ColorSelection  = ColorBgHighlight // Selected item background
	ColorHeader     = ColorBlue        // Headers
	ColorHighlight  = ColorYellow      // Search highlights
	ColorPlaying    = ColorGreen       // Currently playing indicator
	ColorPaused     = ColorYellow      // Paused indicator
	ColorError      = ColorRed         // Error messages
	ColorSuccess    = ColorGreen       // Success messages
	ColorInfo       = ColorBlue        // Info messages
	ColorDimmed     = ColorFgDark      // Dimmed text
	ColorBright     = ColorFg          // Bright text
)