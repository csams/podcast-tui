# Context Findings

## Current Implementation Analysis

### Files That Need Modification
1. `/home/csams/projects/personal/podcast-tui/internal/ui/podcast_list.go` - Line 109: `ensureVisible()` method is empty
2. `/home/csams/projects/personal/podcast-tui/internal/ui/episode_list.go` - Line 117: `ensureVisible()` method is empty  
3. `/home/csams/projects/personal/podcast-tui/internal/ui/app.go` - May need new keybindings for page-based scrolling

### Current State
Both views already have:
- `scrollOffset int` field for tracking scroll position
- `selectedIdx int` field for tracking current selection
- `ensureVisible()` method calls in navigation handlers (j/k/g/G keys)
- Drawing loops that respect `scrollOffset`: `i+v.scrollOffset < len(v.episodes)`

### Drawing Logic Analysis
**Podcast List (podcast_list.go:48):**
- `visibleHeight := h - 3` (header + separator + status bar)
- Loop: `for i := 0; i < visibleHeight && i+v.scrollOffset < len(v.podcasts)`

**Episode List (episode_list.go:71):**  
- `visibleHeight := episodeListHeight - 4` (accounts for table header)
- Loop: `for i := 0; i < visibleHeight && i+v.scrollOffset < len(v.episodes)`
- Also reserves space for description window at bottom

### Existing Keybinding Patterns
- Current navigation: `j/k` (up/down), `g/G` (top/bottom)
- Available keys in `app.go`: Arrow keys used for player control, but `tcell.KeyCtrl*` combinations available
- Vim-style patterns already established

## Technical Constraints and Considerations

### tcell Library Limitations
From research: tcell doesn't provide native scrolling operations, so applications must:
1. Maintain their own viewport/offset tracking
2. Redraw visible portions on scroll events  
3. Handle key events to update viewport

### Integration Points Identified
- Views implement `HandleKey(ev *tcell.EventKey) bool` interface
- App delegates j/k keys to `currentView.HandleKey(ev)` 
- Page-based scrolling will need special handling in `app.go` since it's not character-based

### Similar Features Analyzed
The application already handles:
- Modal dialogs (help, confirmation) with proper key event precedence
- View switching (h/l keys) with maintained state
- Player controls using special keys (arrows, ctrl combinations)

## Implementation Patterns to Follow

### Scroll Position Management
Both views should calculate:
- Available screen space for content
- Total items vs visible items  
- Bounds checking for scroll offset
- Selection visibility within viewport

### Keybinding Integration
Following existing patterns:
- Add page scroll handling to `app.go` for global keys (Ctrl+F/B)
- Keep line-by-line scrolling in view-specific `HandleKey` methods
- Maintain vim-style consistency

### State Persistence 
Views already maintain their state (selectedIdx, scrollOffset) when switching between views, which aligns with requirements.

## Key Files and Their Roles
- `podcast_list.go`: Simple list view, needs basic `ensureVisible()` implementation
- `episode_list.go`: Table view with description window, needs complex `ensureVisible()` considering bottom panel
- `app.go`: Global key handling, may need page-scroll keybindings added