# Requirements Specification: Podcast and Episode List Scrolling

## Problem Statement
Both podcast and episode lists in the terminal-based podcast manager currently have empty `ensureVisible()` implementations, meaning users cannot properly scroll through lists when there are more items than fit on screen. This creates a poor user experience when dealing with large numbers of podcasts or episodes.

## Solution Overview
Implement proper scrolling functionality for both views with vim-style keybindings, smart viewport management, and maintained scroll positions across view transitions.

## Functional Requirements

### FR1: Line-by-Line Scrolling
- **FR1.1:** Implement `ensureVisible()` method in `PodcastListView` to center selected item when possible
- **FR1.2:** Implement `ensureVisible()` method in `EpisodeListView` to center selected item when possible  
- **FR1.3:** Navigation with j/k keys should smoothly scroll content when selection moves off-screen
- **FR1.4:** g/G keys should properly update scroll position when jumping to top/bottom

### FR2: Page-Based Scrolling
- **FR2.1:** Add Ctrl+F keybinding for forward page scroll (vim-style)
- **FR2.2:** Add Ctrl+B keybinding for backward page scroll (vim-style)
- **FR2.3:** Page scrolling should move by approximately one screen height minus overlap
- **FR2.4:** Page scrolling should respect view boundaries (no wrapping)

### FR3: Scroll Position Management
- **FR3.1:** Maintain podcast list scroll position when switching to episode view and back
- **FR3.2:** Maintain episode list scroll position when switching to podcast view and back
- **FR3.3:** Reset episode list position (selectedIdx=0, scrollOffset=0) when selecting a new podcast
- **FR3.4:** Scrolling should stop at boundaries rather than wrap around

### FR4: Episode List Layout Enhancement
- **FR4.1:** Increase description window from 6 lines to 10 lines
- **FR4.2:** Account for 10-line description window in scroll boundary calculations
- **FR4.3:** Ensure episode table scrolling respects reserved description space

### FR5: Edge Case Handling
- **FR5.1:** Handle empty lists gracefully (no crashes, appropriate behavior)
- **FR5.2:** Handle single-item lists gracefully (no unnecessary scrolling)
- **FR5.3:** Handle very small terminal windows appropriately
- **FR5.4:** Ensure scroll positions remain valid when list contents change

## Technical Requirements

### TR1: File Modifications Required
- **TR1.1:** `/home/csams/projects/personal/podcast-tui/internal/ui/podcast_list.go`
  - Implement `ensureVisible()` method at line 109
  - Calculate `visibleHeight = h - 3` for boundary checks
  - Center selection in viewport when scrolling

- **TR1.2:** `/home/csams/projects/personal/podcast-tui/internal/ui/episode_list.go`
  - Implement `ensureVisible()` method at line 117
  - Update `descriptionHeight` from 6 to 10 lines at line 54
  - Account for table header in scroll calculations
  - Center selection in viewport when scrolling

- **TR1.3:** `/home/csams/projects/personal/podcast-tui/internal/ui/app.go`
  - Add Ctrl+F and Ctrl+B keybinding handlers in `handleKey()` method
  - Delegate page scroll events to current view
  - Maintain existing keybinding precedence (dialogs override normal input)

### TR2: Implementation Patterns to Follow
- **TR2.1:** Use existing `scrollOffset` field tracking pattern
- **TR2.2:** Follow existing `HandleKey(ev *tcell.EventKey) bool` interface
- **TR2.3:** Maintain existing drawing loop structure with `i+v.scrollOffset` indexing
- **TR2.4:** Follow existing view delegation pattern from app.go

### TR3: Scroll Algorithm Requirements
- **TR3.1:** Calculate available viewport space: `visibleHeight = totalHeight - headerSpace - footerSpace`
- **TR3.2:** Center selection when possible: `scrollOffset = selectedIdx - visibleHeight/2`
- **TR3.3:** Bounds checking: `scrollOffset = max(0, min(scrollOffset, len(items) - visibleHeight))`
- **TR3.4:** Page scroll amount: `pageSize = visibleHeight - 1` (maintain one line overlap)

## Implementation Hints

### Podcast List ensureVisible() Pattern:
```go
func (v *PodcastListView) ensureVisible() {
    if len(v.podcasts) == 0 {
        return
    }
    
    // Calculate visible area (total height minus header and status bar)
    visibleHeight := /* screen height calculation */ - 3
    
    // Center the selection if possible
    targetOffset := v.selectedIdx - visibleHeight/2
    
    // Apply bounds checking
    v.scrollOffset = max(0, min(targetOffset, len(v.podcasts) - visibleHeight))
}
```

### Episode List ensureVisible() Pattern:
```go
func (v *EpisodeListView) ensureVisible() {
    if len(v.episodes) == 0 {
        return
    }
    
    // Account for description window (now 10 lines) and table header
    descriptionHeight := 10
    episodeListHeight := /* screen height */ - descriptionHeight  
    visibleHeight := episodeListHeight - 4 // Table header space
    
    // Center selection and apply bounds
    targetOffset := v.selectedIdx - visibleHeight/2
    v.scrollOffset = max(0, min(targetOffset, len(v.episodes) - visibleHeight))
}
```

### Page Scroll Integration in app.go:
```go
case tcell.KeyCtrlF:
    // Forward page scroll - delegate to current view
    return a.currentView.HandlePageDown()
case tcell.KeyCtrlB:
    // Backward page scroll - delegate to current view  
    return a.currentView.HandlePageUp()
```

## Acceptance Criteria

### AC1: Basic Scrolling Works
- [ ] j/k navigation smoothly scrolls when selection moves off-screen
- [ ] Selected item is centered in viewport when scrolling
- [ ] g/G keys properly scroll to top/bottom with correct positioning

### AC2: Page Scrolling Works  
- [ ] Ctrl+F scrolls forward by approximately one screen
- [ ] Ctrl+B scrolls backward by approximately one screen
- [ ] Page scrolling maintains one line of overlap for context
- [ ] Page scrolling stops at list boundaries

### AC3: State Management Works
- [ ] Podcast list remembers scroll position when switching to episodes
- [ ] Episode list remembers scroll position when switching to podcasts  
- [ ] Episode list resets when selecting a different podcast
- [ ] No scroll position corruption during view transitions

### AC4: Layout Enhancement Works
- [ ] Description window is 10 lines tall instead of 6
- [ ] Episode table scrolling accounts for 10-line description area
- [ ] Table layout remains properly formatted during scrolling

### AC5: Edge Cases Handled
- [ ] Empty lists don't crash or behave unexpectedly
- [ ] Single-item lists work correctly (no unnecessary scrolling)
- [ ] Very small terminal windows handled gracefully
- [ ] Rapid navigation doesn't cause visual glitches

## Assumptions
- Terminal supports standard Ctrl key combinations
- Users are familiar with vim-style navigation patterns
- tcell library continues to require manual viewport management (no native scrolling)
- Current view delegation architecture remains unchanged
- Existing keybinding patterns should be preserved