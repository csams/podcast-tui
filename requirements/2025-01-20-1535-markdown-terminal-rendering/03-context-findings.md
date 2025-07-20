# Context Findings - Markdown Terminal Rendering

## Current Implementation Analysis

### Description Handling Flow
1. **Feed Parsing** (`internal/feed/parser.go`):
   - RSS descriptions are parsed as raw strings from XML
   - No HTML entity decoding or markdown processing happens at parse time
   - Descriptions may contain HTML entities (`&lt;`, `&gt;`, `&amp;`, etc.)
   - Descriptions may contain HTML tags wrapped in CDATA or entity-encoded

2. **Description Cleaning** (`internal/ui/podcast_list.go:507-525`, `internal/ui/episode_list.go:684-697`):
   - `cleanDescription()` removes excessive whitespace and newlines
   - Replaces tabs, carriage returns, and newlines with single spaces
   - Multiple spaces are collapsed to single spaces
   - No HTML/markdown processing currently

3. **Text Wrapping with Highlights** (`internal/ui/podcast_list.go:524-624`, `internal/ui/episode_list.go:751-862`):
   - `wrapTextWithHighlights()` splits text by words and wraps to width
   - Maintains highlight positions through word wrapping
   - Uses rune positions for proper Unicode handling
   - Critical: Position tracking maps original text positions to wrapped line positions

4. **Search Highlighting** (`internal/ui/search.go`):
   - Uses fzf algorithm for fuzzy matching
   - Returns match positions as rune indices in the original text
   - Positions are preserved through text transformations

### Key Files to Modify

1. **`internal/ui/markdown.go`** (NEW):
   - Create markdown parser/converter
   - Handle HTML entity decoding
   - Convert markdown to Unicode-formatted text
   - Maintain position mapping for search highlights

2. **`internal/ui/podcast_list.go`** (MODIFY):
   - Update `cleanDescription()` to use markdown converter
   - Ensure position mapping is maintained for highlights

3. **`internal/ui/episode_list.go`** (MODIFY):
   - Update `cleanDescription()` to use markdown converter
   - Ensure position mapping is maintained for highlights

### Technical Constraints

1. **Position Preservation**:
   - Search highlighting depends on exact character positions
   - Markdown conversion changes text length (e.g., `**bold**` → `bold`)
   - Need bidirectional position mapping between original and converted text

2. **Unicode Rendering**:
   - Terminal supports Unicode characters for formatting
   - Can use: 
     - Bold: ANSI escape sequences (not reliable in all terminals)
     - Bullets: `•`, `◦`, `▸`, `‣`
     - Quotes: `│`, `▌`, `┃`
     - Links: Underline with tcell styling

3. **Real-World Content**:
   - Podcast descriptions contain:
     - HTML entities: `&lt;b&gt;`, `&amp;`, `&quot;`
     - HTML tags: `<b>`, `<i>`, `<p>`, `<ul>`, `<li>`, `<a href>`
     - Markdown: `**bold**`, `*italic*`, `[link](url)`, `- lists`
     - Mixed content: Both HTML and markdown in same description

### Similar Features Analyzed

1. **Search Highlighting**:
   - Already handles Unicode text properly
   - Maps positions through text transformations
   - Uses tcell styles for visual highlighting

2. **Text Wrapping**:
   - Preserves positions through line wrapping
   - Handles Unicode characters correctly

### Integration Points

1. **Feed Parser**: No changes needed - keep raw descriptions
2. **Description Display**: Intercept in `cleanDescription()` methods
3. **Search Matching**: Apply to cleaned/converted text
4. **Position Mapping**: Maintain original→converted position map

### External Dependencies

Consider using:
- `html` package for entity decoding
- Custom markdown parser for position tracking
- Or lightweight markdown library that preserves positions

### Implementation Strategy

1. Create position-preserving markdown converter
2. Integrate into description cleaning pipeline
3. Map search highlight positions through conversion
4. Use tcell styles and Unicode characters for formatting
5. Test with real-world podcast descriptions containing mixed HTML/markdown