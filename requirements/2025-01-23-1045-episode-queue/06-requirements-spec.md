# Requirements Specification: Episode Queue

Generated: 2025-01-23T18:45:00Z
Status: Complete

## Problem Statement
Users need a way to queue multiple podcast episodes for continuous playback without manual intervention between episodes. Currently, episodes must be played one at a time with no automatic progression.

## Solution Overview
Implement a persistent queue system that allows users to add episodes from any podcast, view and manage the queue, and have episodes automatically play in sequence. The queue will be accessible via a dedicated view that follows the existing UI patterns.

## Functional Requirements

### 1. Queue Management
- **FR1.1**: Pressing Enter or 'l' in episode list adds the episode to the queue
- **FR1.2**: If the queue is empty when adding first episode, playback starts immediately
- **FR1.3**: Subsequent episodes are appended to the queue without interrupting playback
- **FR1.4**: Duplicate episodes cannot be added to the queue
- **FR1.5**: Queue has no size limit

### 2. Queue View
- **FR2.1**: TAB key toggles to queue view from any view, returns to previous view when pressed in queue
- **FR2.2**: Queue view uses identical table layout as episode list (Status, Title, Date, Position columns)
- **FR2.3**: Status column shows queue position numbers (1, 2, 3...)
- **FR2.4**: Currently playing episode shows ▶ indicator with row highlighting
- **FR2.5**: Download status indicators (✔, [⬇]) are displayed
- **FR2.6**: Progress information updates in real-time for playing episode
- **FR2.7**: No description window at bottom (full height for queue list)
- **FR2.8**: Maintains scroll position when switching between views

### 3. Queue Operations
- **FR3.1**: 'u' key removes selected episode from queue (in both episode list and queue view)
- **FR3.2**: Enter key on queued episode immediately plays that episode
- **FR3.3**: Standard navigation keys work (j/k, g, G, Ctrl-d/u)
- **FR3.4**: Search functionality ('/' key) works within queue
- **FR3.5**: Alt+j/Alt+k reorder queue items (move down/up)
- **FR3.6**: 'g' or 'e' in queue view navigates to episode in its podcast context

### 4. Playback Behavior
- **FR4.1**: When episode reaches >95% completion, automatically play next in queue
- **FR4.2**: Use seamless track switching (no gap between episodes)
- **FR4.3**: Completed episodes are removed from queue
- **FR4.4**: If last episode completes, player stops

### 5. Persistence
- **FR5.1**: Queue state persists across application restarts
- **FR5.2**: Queue position maintained when resuming playback
- **FR5.3**: Queue stored in subscriptions.json file

### 6. Status Bar
- **FR6.1**: Identical appearance and behavior as other views
- **FR6.2**: Shows current mode, player status, and progress

## Technical Requirements

### 1. Data Model
- **TR1.1**: Create `QueueEntry` struct in `internal/models/queue.go`:
  ```go
  type QueueEntry struct {
      EpisodeID string    `json:"episode_id"`
      AddedAt   time.Time `json:"added_at"` 
      Position  int       `json:"position"`
  }
  ```
- **TR1.2**: Add `Queue []*QueueEntry` field to `Subscriptions` struct
- **TR1.3**: Add queue management methods to `Subscriptions`:
  - `AddToQueue(episodeID string) error`
  - `RemoveFromQueue(episodeID string)`
  - `GetQueuePosition(episodeID string) int`
  - `GetNextInQueue() *Episode`
  - `ReorderQueue(positions []int)`

### 2. UI Implementation
- **TR2.1**: Create `internal/ui/queue_view.go` implementing View interface
- **TR2.2**: Implement `QueueTableRow` for Table abstraction
- **TR2.3**: Reuse existing table columns configuration from episode list
- **TR2.4**: Handle keyboard events for queue-specific operations

### 3. Application Integration
- **TR3.1**: Add `queue *QueueView` field to App struct
- **TR3.2**: Initialize queue view in `NewApp()`
- **TR3.3**: Add TAB key handler for view cycling with previous view tracking
- **TR3.4**: Implement `playNextInQueue()` method
- **TR3.5**: Modify `handleProgress()` to detect completion and trigger next

### 4. Episode List Modifications
- **TR4.1**: Change Enter/l behavior to call `AddToQueue()` instead of direct play
- **TR4.2**: Extend status column to show queue indicators in episode list ("Q:1", "Q:2")
- **TR4.3**: Update `GetCellStyle()` to handle queue position display

### 5. Player Integration
- **TR5.1**: Use existing `player.SwitchTrack()` for seamless transitions
- **TR5.2**: Ensure player state updates correctly on auto-advance

## Implementation Hints

### View Toggle Pattern
```go
// In handleKey() for TAB
case tcell.KeyTAB:
    if a.currentView == a.podcasts || a.currentView == a.episodes {
        a.previousView = a.currentView
        a.currentView = a.queue
        a.queue.refresh()
    } else if a.currentView == a.queue {
        a.currentView = a.previousView
    }
```

### Auto-advance Detection
```go
// In handleProgress()
progress := a.player.GetProgress()
if progress.Duration > 0 && progress.Position > 0 {
    completion := float64(progress.Position) / float64(progress.Duration)
    if completion > 0.95 {
        if next := a.subscriptions.GetNextInQueue(); next != nil {
            go a.playNextInQueue()
        }
    }
}
```

### Queue Status in Episode List
```go
// In EpisodeTableRow.GetCell() for status column
if queuePos := subscriptions.GetQueuePosition(e.episode.ID); queuePos > 0 {
    status = fmt.Sprintf("Q:%d", queuePos)
}
```

## Acceptance Criteria

1. **Queue Functionality**
   - ✓ Episodes can be added to queue from episode list
   - ✓ First episode starts playing immediately
   - ✓ Episodes play sequentially without gaps
   - ✓ Completed episodes are removed from queue

2. **Queue View**
   - ✓ TAB cycles through all three views correctly
   - ✓ Queue view displays episodes with proper formatting
   - ✓ Currently playing episode is highlighted
   - ✓ Progress updates in real-time
   - ✓ 'x' removes episodes from queue
   - ✓ Enter skips to selected episode

3. **Persistence**
   - ✓ Queue survives application restart
   - ✓ Queue position maintained on resume
   - ✓ No data loss on crash

4. **UI Consistency**
   - ✓ Status bar identical to other views
   - ✓ Navigation keys work as expected
   - ✓ No visual glitches when switching views
   - ✓ p/e/q keys provide quick navigation between views

## Implementation Summary

The episode queue feature has been fully implemented with the following enhancements beyond the initial specification:

### Key Binding Enhancements
- **Navigation**: p (podcast), e (episode), q (queue) keys for quick view switching
- **Queue Management**: 'u' key unified for removing from queue in all views
- **Reordering**: Alt+j/Alt+k for moving queue items while maintaining playback
- **Context Navigation**: 'g' or 'e' from queue jumps to episode in its podcast
- **Quit**: Changed to capital 'Q' to avoid conflict with queue navigation

### Technical Enhancements
- **Efficient Lookups**: Episode-to-podcast index for O(1) podcast lookups
- **Seamless Playback**: Queue operations don't interrupt current playback
- **Real-time Updates**: Queue position indicators update across all views
- **Header Display**: Shows "Episode Queue (count)" with current queue size

### User Experience Improvements
- Consistent status messages for all operations
- Queue position preserved when reordering items
- Currently playing episode continues when removed from queue
- Visual indicators (Q:1, Q:2) in episode list for queued items

## Testing Completed

All functionality has been implemented and tested:
1. ✓ Queue persistence across restarts
2. ✓ Auto-advance at >95% completion
3. ✓ Seamless playback during queue operations
4. ✓ All navigation keys working correctly
5. ✓ Queue reordering maintains playback state
6. ✓ Proper handling of edge cases (empty queue, single item)

## Future Enhancement Opportunities
- Batch operations (clear all, save as playlist)
- Queue repeat/shuffle modes
- Drag-and-drop reordering (if terminal support improves)
- Queue history tracking
- Import/export queue as M3U playlist