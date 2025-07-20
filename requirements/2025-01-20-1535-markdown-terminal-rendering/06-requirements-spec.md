# Requirements Specification - Markdown Terminal Rendering

## Problem Statement
Podcast RSS feeds contain descriptions with mixed content including HTML entities, HTML tags, and markdown formatting. Currently, the podcast-tui application displays this content as plain text, stripping only whitespace. This results in users seeing raw HTML/markdown syntax instead of properly formatted text, making descriptions harder to read.

## Solution Overview
Implement a markdown-to-terminal converter that:
1. Decodes HTML entities (`&lt;`, `&gt;`, `&amp;`, etc.)
2. Strips or converts HTML tags to appropriate representations
3. Converts markdown formatting to terminal-appropriate styling
4. Maintains exact character position mapping for search highlighting
5. Uses terminal styling capabilities via tcell for formatting

## Functional Requirements

### FR1: HTML Entity Decoding
- Decode common HTML entities before markdown processing
- Support at minimum: `&lt;`, `&gt;`, `&amp;`, `&quot;`, `&apos;`, `&#39;`
- Preserve Unicode characters that are already decoded

### FR2: HTML Tag Handling
- Strip unsupported HTML tags
- Convert supported tags:
  - `<b>`, `<strong>` → Bold terminal style
  - `<i>`, `<em>` → Italic terminal style
  - `<a href="url">text</a>` → "text (url)"
  - `<p>` → Paragraph breaks (double newline)
  - `<br>` → Single newline
  - `<ul>`, `<ol>`, `<li>` → List with bullets

### FR3: Markdown Conversion
- Support CommonMark basics:
  - `**bold**`, `__bold__` → Bold terminal style
  - `*italic*`, `_italic_` → Italic terminal style
  - `` `code` `` → Different color/style
  - `[text](url)` → "text (url)"
  - Headers (`#`, `##`, etc.) → Bold + newlines
  - Lists (`-`, `*`, `+`, `1.`) → Unicode bullets
  - Blockquotes (`>`) → Indented with `│` prefix

### FR4: List Rendering
- Use Unicode bullets for different nesting levels:
  - Level 1: `•` (bullet)
  - Level 2: `◦` (white bullet)
  - Level 3: `▸` (triangular bullet)
- Maintain proper indentation for nested lists

### FR5: Position Mapping
- Create bidirectional mapping between original and converted text positions
- Ensure search highlight positions map correctly through all transformations
- Support rune-based positioning for proper Unicode handling

### FR6: Search Integration
- Search should work on converted/cleaned text
- Highlight positions must display correctly on rendered text
- Original fzf matching behavior must be preserved

## Technical Requirements

### TR1: New Module Structure
Create `internal/ui/markdown.go` with:
```go
type MarkdownConverter struct {
    // Position mapping structures
}

type ConversionResult struct {
    Text string
    Styles []StyleRange  // For applying tcell styles
    PositionMap *PositionMap
}

type StyleRange struct {
    Start, End int
    Style tcell.Style
}

type PositionMap struct {
    // Bidirectional mapping implementation
}

func (mc *MarkdownConverter) Convert(text string) ConversionResult
func (pm *PositionMap) OriginalToConverted(pos int) int
func (pm *PositionMap) ConvertedToOriginal(pos int) int
```

### TR2: Integration Points
Modify in `internal/ui/podcast_list.go` and `internal/ui/episode_list.go`:
- Replace `cleanDescription()` with markdown conversion
- Update `wrapTextWithHighlights()` to handle styled text
- Modify `drawLineWithHighlights()` to apply styles

### TR3: Rendering Changes
- Use tcell styles instead of Unicode for bold/italic
- Apply styles character by character during rendering
- Maintain existing highlight style layering

### TR4: Testing Requirements
- Unit tests for position mapping accuracy
- Tests with real-world RSS content examples
- Search highlighting verification tests
- Unicode handling tests

## Implementation Hints

### Processing Pipeline
1. Decode HTML entities
2. Parse and convert HTML tags
3. Parse and convert markdown
4. Build position mappings during each step
5. Generate style ranges for formatting

### Position Mapping Strategy
- Track transformations as a series of edits
- Each edit records: original position, original length, new position, new length
- Build cumulative mapping table for O(1) lookups

### Style Application
- Merge overlapping styles (e.g., bold + italic)
- Later styles override earlier ones
- Highlight style takes precedence over all formatting

## Acceptance Criteria

### AC1: Visual Formatting
- [ ] Bold text appears bold in terminal
- [ ] Italic text appears italic (if supported)
- [ ] Links show as "text (url)" format
- [ ] Lists display with proper bullets and indentation
- [ ] No raw HTML/markdown syntax visible

### AC2: Search Functionality
- [ ] Search highlighting works correctly on formatted text
- [ ] Search matches on converted text content
- [ ] Highlight positions align with displayed characters
- [ ] No regression in search performance

### AC3: Content Preservation
- [ ] All text content is preserved (no data loss)
- [ ] Unicode characters display correctly
- [ ] Long descriptions wrap properly
- [ ] Scrolling works as before

### AC4: Edge Cases
- [ ] Mixed HTML and markdown renders correctly
- [ ] Malformed HTML/markdown doesn't crash
- [ ] Empty descriptions work as before
- [ ] Very long URLs in links wrap appropriately

## Assumptions
1. Terminal supports basic text styling (bold, italic, underline)
2. Users prefer visual formatting over raw syntax
3. RSS feeds follow common patterns for HTML/markdown usage
4. Performance impact of conversion is acceptable
5. Most important markdown features are inline formatting and lists

## Out of Scope
- Syntax highlighting for code blocks
- Table rendering
- Image alt text display
- Interactive link clicking (just visual indication)
- Custom markdown extensions