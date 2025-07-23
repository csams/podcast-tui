# Episode Queue System Requirements Specification

## Problem Statement
Users need a way to queue multiple podcast episodes for continuous playback, similar to a playlist in music players. Currently, users can only play one episode at a time and must manually select the next episode when one finishes.

## Solution Overview
Implement a persistent episode queue system with a dedicated view that allows users to:
- Add episodes to a queue for sequential playback
- View and manage the queue with vim-style navigation
- Automatically advance to the next episode when one completes
- Reorder and remove episodes from the queue
- See queue status in the main episode list

## Functional Requirements

### 1. Queue Management
- **Add to Queue**: Press 'l' or Enter in episode list to enqueue selected episode
- **Prevent Duplicates**: Silently ignore attempts to add already-queued episodes
- **Auto-play**: Start playing immediately if adding first episode to empty queue
- **Queue Persistence**: Save queue to `~/.config/podcast-tui/queue.json` and restore on startup
- **Queue Order**: Most recently added episodes appear at top of queue

### 2. Queue View Display
- **Location**: 15-line window directly below description window
- **Visibility**: Always visible (episodes move up, description shrinks)
- **Columns**:
  - Status (9 chars): Playing/paused indicator + selection marker
  - Podcast Title: Name of podcast episode belongs to
  - Episode Title: Full episode title
  - Date (10 chars): Publication date
  - Position (17 chars): Live-updating position/duration for playing episode

### 3. Navigation & Focus
- **TAB Key**: Cycle through views: Podcast List → Episode List → Queue → Podcast List
- **j/k**: Scroll up/down in queue (when focused)
- **No h/l**: These keys don't work from queue view
- **Focus Indicator**: Highlight current view in status bar

### 4. Queue Actions
- **x**: Remove selected episode from queue
  - If playing, stop immediately and advance to next
  - If queue becomes empty, stop playback
- **Alt+j**: Move selected episode down in queue
- **Alt+k**: Move selected episode up in queue
- **Space**: Toggle play/pause if selected episode is playing

### 5. Playback Behavior
- **Queue Position**: Top episode (index 0) is always the playing episode
- **Auto-advance**: When episode completes, automatically start next in queue
- **Position Tracking**: Save/restore playback position for queued episodes

### 6. Visual Indicators
- **Episode List**: Show "Q:1", "Q:2", etc. in status column for queued episodes
- **Queue View**: Highlight currently playing episode (green) and paused (yellow)
- **Real-time Updates**: Position/duration updates every second while playing

## Technical Requirements

### 1. Data Model
```go
// internal/models/queue.go
type Queue struct {
    Episodes []string `json:"episodes"` // Episode GUIDs in order
}
```

### 2. File Modifications
- **internal/models/subscription.go**: Add Queue field, update persistence
- **internal/ui/app.go**: Add queue view, TAB navigation, view cycling
- **internal/ui/episode_list.go**: Add enqueue keys, queue indicators
- **internal/player/player.go**: Add queue management, auto-advance logic
- **New**: internal/ui/queue_view.go - Implement queue view

### 3. Integration Points
- Hook into player's `onEpisodeComplete` to trigger auto-advance
- Extend existing persistence system for queue saving
- Reuse episode status formatting for consistency
- Share column layout patterns from episode list

### 4. State Management
- Queue modifications trigger immediate save to disk
- Player state updates propagate to all views
- Maintain queue consistency during concurrent operations

## Implementation Hints

### View Cycling Pattern
```go
// In app.HandleKey() for TAB
views := []View{podcastView, episodeView, queueView}
currentIndex := // find current view index
nextIndex := (currentIndex + 1) % len(views)
app.currentView = views[nextIndex]
```

### Queue Indicator in Episode Status
```go
// In formatStatus()
if queuePos := app.getQueuePosition(episode.GUID); queuePos > 0 {
    return fmt.Sprintf("Q:%d", queuePos)
}
```

### Auto-advance Implementation
```go
// In player event handler
case "eof-reached":
    if nextEpisode := app.queue.GetNext(); nextEpisode != nil {
        app.playEpisode(nextEpisode)
    }
```

## Acceptance Criteria

1. ✓ Can add episodes to queue with l/Enter keys
2. ✓ Queue persists between application sessions
3. ✓ Episodes auto-advance when one completes
4. ✓ TAB cycles through all three views
5. ✓ Queue view shows all required columns with live updates
6. ✓ Can reorder queue with Alt+j/k
7. ✓ Can remove episodes with x key
8. ✓ Queue indicators appear in episode list
9. ✓ No duplicate episodes in queue
10. ✓ Playing episode always at top of queue

## Assumptions
- Maximum queue size: Unlimited (no artificial restrictions)
- Queue removal during playback: Stops immediately as requested
- Episode not found on restore: Silently remove from queue
- Network errors during auto-advance: Stop playback, show error