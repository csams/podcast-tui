# Context Findings for Podcast Playback Control

## Files That Need Modification

### Core Files to Modify:
1. `/internal/player/player.go` - Main player implementation
   - Fix mpv IPC communication
   - Add volume control methods
   - Add speed control methods
   - Implement proper progress tracking

2. `/internal/ui/app.go` - Main UI controller
   - Add new keyboard shortcuts
   - Enhance status bar with progress display
   - Add volume/speed indicators

3. `/internal/models/episode.go` - Episode data model
   - Already has Position field for resume functionality
   - Position data needs persistence implementation

4. `/internal/models/subscriptions.go` - Data persistence
   - Extend Save() to include episode progress
   - Extend Load() to restore episode progress

## Exact Patterns to Follow

### Keyboard Event Handling Pattern:
```go
// In app.go handleKey() method
case tcell.KeyRight:
    if a.player.IsPlaying() {
        a.player.Seek(10) // Forward 10 seconds
    }
```

### Status Display Pattern:
```go
// In app.go drawStatusBar() method
playerStatus := "⏸"
if a.player.IsPlaying() && !a.player.IsPaused() {
    playerStatus = "▶"
}
```

### Thread-Safe Player Method Pattern:
```go
func (p *Player) SetVolume(volume int) error {
    p.mu.Lock()
    defer p.mu.Unlock()
    // Implementation
}
```

## Similar Features Analyzed

1. **Current Play/Pause Implementation**:
   - Uses space bar for toggle
   - Updates UI status immediately
   - Has basic error handling

2. **Episode Selection Pattern**:
   - Enter key selects and plays
   - Stops current playback before starting new

3. **Data Persistence Pattern**:
   - JSON marshaling for subscriptions
   - Saves to `~/.config/podcast-tui/`
   - No current episode progress saving

## Technical Constraints and Considerations

### MPV IPC Constraints:
1. **Current Issue**: Shell piping with exec.Command doesn't work
   ```go
   // This doesn't work:
   cmd := exec.Command("echo", `{"command": ["set_property", "pause", true]}`, "|", "socat", "-", "/tmp/mpv-socket")
   ```

2. **Solution Options**:
   - Use mpv command-line properties: `--input-ipc-server`
   - Implement proper socket communication
   - Use socat as separate process
   - Consider mpv Go bindings

### UI Constraints:
1. **Terminal Width**: Progress bar must adapt to terminal width
2. **Update Frequency**: Avoid excessive redraws for performance
3. **Keybinding Conflicts**: Must not override existing vim-style navigation

### Data Constraints:
1. **Backward Compatibility**: New save format must handle old files
2. **File Size**: Progress data for many episodes could grow large
3. **Atomic Writes**: Prevent corruption during save

## Integration Points Identified

### Player Integration:
1. **MPV Socket Communication**:
   - Socket path: `/tmp/mpv-socket`
   - Protocol: JSON-RPC
   - Commands: pause, seek, volume, speed properties

2. **Progress Updates**:
   - Need goroutine for polling mpv status
   - Update UI through existing app.update channel
   - Store in Episode.Position field

### UI Integration:
1. **Status Bar Enhancement**:
   - Current format: `[MODE] Status Message [Player Status]`
   - New format: `[MODE] Message [▶ 12:34/45:00] [1.5x] [Vol:75%]`

2. **Keyboard Shortcuts**:
   - Available keys: arrows, +/-, <>, numbers
   - Must work in NORMAL mode
   - Show hints in help or status

### Data Integration:
1. **Progress Persistence**:
   - Add to subscription JSON structure
   - Save on pause/stop/quit
   - Restore on episode selection

2. **Settings Storage**:
   - Consider separate settings file
   - Store default volume/speed
   - Remember last played episode

## Best Practices from Codebase

1. **Error Handling**: Return errors up the chain, display in status bar
2. **Mutex Usage**: Lock for all player state changes
3. **UI Updates**: Use app.update channel for async updates
4. **File I/O**: Use os.UserConfigDir() for config location
5. **Logging**: Extensive debug logging already in place

## Required External Documentation

1. **MPV IPC Protocol**: https://mpv.io/manual/stable/#json-ipc
2. **MPV Properties**: https://mpv.io/manual/stable/#properties
3. **tcell Key Events**: Reference for available key codes
4. **Go JSON-RPC**: For socket communication implementation