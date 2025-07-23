# Context Findings for Episode Queue System

## Files That Need Modification

### Core Files to Modify:
1. **internal/models/subscription.go**
   - Add Queue field to Subscriptions struct
   - Update save/load methods to persist queue

2. **internal/ui/app.go**
   - Add queueView as new view
   - Implement TAB view cycling logic
   - Add queue-related keyboard shortcuts
   - Update status bar to show current view

3. **internal/ui/episode_list.go**
   - Add queue indicators to status column
   - Implement l/Enter key handling for enqueue
   - Update formatStatus() to show queue position

4. **internal/player/player.go**
   - Add queue management methods
   - Implement auto-advance on episode completion
   - Handle queue-based playback

### New Files to Create:
1. **internal/models/queue.go**
   - Queue struct with episode IDs and order
   - Methods for add/remove/reorder

2. **internal/ui/queue_view.go**
   - New view implementing View interface
   - Table layout similar to episode list
   - Handle queue-specific keyboard events

## Patterns to Follow

### View Management Pattern:
```go
// Current pattern in app.go
switch app.currentView.(type) {
case *PodcastListView:
    // handle podcast view
case *EpisodeListView:
    // handle episode view
}

// Add QueueView to this pattern
```

### Persistence Pattern:
```go
// Follow subscription.go pattern
type Subscriptions struct {
    Podcasts []Podcast `json:"podcasts"`
    Queue    *Queue    `json:"queue,omitempty"`
}
```

### Column Layout Pattern:
```go
// From episode_list.go
columns := []struct {
    name  string
    width int
}{
    {"Status", 9},
    {"Title", titleWidth},
    {"Date", 10},
    {"Position", 17},
}
```

### Status Indicator Pattern:
```go
// Current indicators in formatStatus()
"►" for playing
"⏸" for paused
"✓" for completed
// Add "Q:1", "Q:2" etc for queue position
```

## Similar Features Analyzed

### 1. Download Queue (internal/download/manager.go)
- Uses goroutines and channels for queue processing
- However, our queue is simpler - just ordered episode IDs

### 2. Episode State Tracking
- Player already tracks current episode
- Position persistence works via subscription save
- Can extend this for queue persistence

### 3. View Switching (h/l keys)
- Currently binary (podcast ↔ episode)
- TAB will make it cyclic (podcast → episode → queue → podcast)

## Technical Constraints

1. **Terminal Size**: Queue window is fixed 15 lines, must handle overflow with scrolling
2. **Episode IDs**: Use GUID from RSS feed as unique identifier
3. **Concurrent Access**: Player runs in separate goroutine, need mutex for queue access
4. **State Consistency**: Queue position indicators must update in real-time

## Integration Points

1. **Player Events**: Hook into player's state changes to:
   - Auto-advance queue on episode completion
   - Update position display in queue view

2. **Episode List**: Modify existing episode list to:
   - Show queue indicators
   - Handle enqueue keyboard shortcuts

3. **Persistence**: Extend subscription system to:
   - Save queue state on changes
   - Restore queue on startup

## UI Layout Specification

```
┌─────────────────────────────────────────┐
│         Podcast/Episode List            │ <- Existing views
├─────────────────────────────────────────┤
│         Description (if episode)         │ <- Existing 15-line window
├─────────────────────────────────────────┤
│         Episode Queue (new)             │ <- New 15-line window
├─────────────────────────────────────────┤
│         Status Bar                      │
└─────────────────────────────────────────┘
```

## Key Implementation Notes

1. **Queue Order**: Most recently added at top means prepending to slice
2. **Duplicate Prevention**: Check episode ID before adding
3. **Playing Logic**: Top of queue (index 0) is always playing episode
4. **Reorder Logic**: Alt+j/k swaps with adjacent items
5. **Focus Management**: Track activeView separately from currentView for TAB cycling