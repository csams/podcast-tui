# Context Findings

## Architecture Analysis

### Table Abstraction Pattern
The `Table` widget in `internal/ui/table.go` provides a reusable scrollable table component with:
- Dynamic column width calculation with fixed and flexible columns
- Selection tracking and navigation (j/k, gg, G, Ctrl-d/u)
- Custom row rendering via `TableRow` interface
- Highlight support for search matches
- Built-in scroll management

### View Management
- Views are direct struct fields in `App`, not array-based
- Current pattern: `a.currentView = a.episodes`
- TAB key is completely unused - perfect for queue view cycling
- View interface requires `Draw()` and `HandleKey()` methods

### Player Track Completion
- NO built-in track completion detection in player
- Episode completion checked in `app.go` by monitoring progress > 95%
- Player provides `SwitchTrack(url)` for seamless transitions
- Progress updates sent every second via `watchProgress()` goroutine

### Episode List Implementation
Key patterns from `internal/ui/episode_list.go`:
- Uses `Table` abstraction with custom `EpisodeTableRow`
- Columns: Status/Local (12 chars), Title (flex), Date (11 chars), Position (9 chars)
- Status indicators: ✔ (downloaded), ▶ (playing), ⏸ (paused), [⬇] (downloading)
- Split view with description window at bottom

### Persistence Model
From `internal/models/subscription.go`:
- All data stored in `~/.config/podcast-tui/subscriptions.json`
- Root `Subscriptions` struct contains `Podcasts []*Podcast`
- Episode lookup optimized with `episodeIndex map[string]*Episode`
- Episodes have unique IDs: podcast URL + episode URL + publish date

### Status Bar Pattern
From `drawStatusBar()` in `app.go`:
- Fixed at screen height - 1
- Left: Mode (NORMAL/COMMAND/SEARCH)
- Center: Status messages
- Right: Player status with progress

## Implementation Requirements

### Files to Modify
1. **internal/models/subscription.go**:
   - Add `Queue []*QueueEntry` field to Subscriptions
   - Add methods: AddToQueue(), RemoveFromQueue(), GetQueuePosition()

2. **internal/ui/app.go**:
   - Add `queue *QueueView` field
   - Add TAB key handler for view cycling
   - Modify `handleProgress()` to detect completion and play next
   - Add `playNextInQueue()` method

3. **internal/ui/episode_list.go**:
   - Modify status column to show queue position (e.g., "Q:1")
   - Change Enter/l handler to add to queue instead of direct play

4. **internal/ui/help_dialog.go**:
   - Add queue-related keybindings

### Files to Create
1. **internal/models/queue.go**:
   ```go
   type QueueEntry struct {
       EpisodeID string    `json:"episode_id"`
       AddedAt   time.Time `json:"added_at"`
       Position  int       `json:"position"`
   }
   ```

2. **internal/ui/queue_view.go**:
   - Implement View interface
   - Use Table abstraction like episode list
   - Same columns as episode list
   - No description window
   - Handle 'x' key for removal

### Key Implementation Details

1. **Queue Persistence**: Store queue as array of episode IDs with positions in subscriptions.json

2. **Auto-advance Logic**: In `handleProgress()`, check if position/duration > 0.95, then call `playNextInQueue()`

3. **View Cycling**: TAB rotates through podcasts → episodes → queue, tracking previous view for return

4. **Queue Indicators**: Extend episode status to show "Q:1", "Q:2" etc. alongside existing indicators

5. **Duplicate Prevention**: Check episode ID before adding to queue

6. **Seamless Playback**: Use `player.SwitchTrack()` for gapless transitions between queued episodes