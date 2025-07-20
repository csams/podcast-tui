# Podcast Playback Control Requirements Specification

## Problem Statement and Solution Overview

The podcast-tui application currently has basic playback functionality but lacks essential controls that podcast listeners expect. Users need fine-grained control over playback including seeking, volume adjustment, speed control, and the ability to resume episodes where they left off. The existing mpv integration has implementation issues that prevent proper pause/resume and seek functionality.

This feature will enhance the player module to provide comprehensive playback controls through keyboard shortcuts, display real-time progress information in the status bar, and persist playback positions to enable seamless episode resumption.

## Functional Requirements

### 1. Keyboard Shortcuts (NORMAL mode)
- **Arrow Keys**:
  - `→` (Right Arrow): Seek forward 10 seconds
  - `←` (Left Arrow): Seek backward 10 seconds
  - `↑` (Up Arrow): Increase volume by 5%
  - `↓` (Down Arrow): Decrease volume by 5%
- **Playback Speed**:
  - `<` : Decrease playback speed (cycle: 0.5x → 0.75x → 1.0x)
  - `>` : Increase playback speed (cycle: 1.0x → 1.25x → 1.5x → 2.0x)
  - `=` : Reset to normal speed (1.0x)
- **Additional Controls**:
  - `m` : Mute/unmute toggle
  - `f` : Seek forward 30 seconds
  - `b` : Seek backward 30 seconds
- **Existing** (maintained):
  - `Space` : Play/pause toggle
  - `Enter` : Play selected episode

### 2. Progress Display
- **Status Bar Format**:
  - Compact: `[NORMAL] [▶ 12:34/45:00]` (minimum width)
  - Full: `[NORMAL] [▶ 12:34/45:00] [1.5x] [Vol:75%]` (wide terminals)
- **Dynamic Sizing**:
  - Detect terminal width and adjust display accordingly
  - Priority: Mode > Player Status > Progress > Speed > Volume
- **Update Frequency**: Every second during playback

### 3. Playback Speed Control
- **Supported Speeds**: 0.5x, 0.75x, 1.0x, 1.25x, 1.5x, 2.0x
- **Default**: Always starts at 1.0x (not persisted)
- **Display**: Show current speed when not 1.0x

### 4. Volume Control
- **Range**: 0-100%
- **Default**: System/mpv default (not persisted)
- **Mute**: Toggle with 'm' key, show 🔇 when muted

### 5. Position Memory
- **Save Timing**:
  - On pause
  - On stop
  - On application quit
  - Every 30 seconds during playback
- **Resume Behavior**:
  - When selecting a partially played episode, resume from saved position
  - Show visual indicator for partially played episodes (e.g., `[▶ 50%]`)
- **Completion**: Mark episode as played when >95% complete

## Technical Requirements

### 1. Fix MPV IPC Communication (`/internal/player/player.go`)
- **Replace** shell piping approach with proper socket communication
- **Options**:
  1. Direct socket communication using net.Dial()
  2. JSON-RPC protocol implementation
  3. Use socat as a separate process (fallback)
- **Required Commands**:
  - `{"command": ["set_property", "pause", <bool>]}`
  - `{"command": ["seek", <seconds>, "relative"]}`
  - `{"command": ["set_property", "volume", <0-100>]}`
  - `{"command": ["set_property", "speed", <float>]}`
  - `{"command": ["get_property", "time-pos"]}`
  - `{"command": ["get_property", "duration"]}`

### 2. Player Module Enhancements (`/internal/player/player.go`)
- **New Methods**:
  ```go
  func (p *Player) GetVolume() (int, error)
  func (p *Player) SetVolume(volume int) error
  func (p *Player) GetSpeed() (float64, error)
  func (p *Player) SetSpeed(speed float64) error
  func (p *Player) GetPosition() (time.Duration, error)
  func (p *Player) GetDuration() (time.Duration, error)
  func (p *Player) IsMuted() bool
  func (p *Player) ToggleMute() error
  ```
- **Fix Existing**:
  - `Seek()` method to use proper IPC
  - `TogglePause()` to use proper IPC
  - Progress goroutine to poll actual position

### 3. UI Integration (`/internal/ui/app.go`)
- **Extend handleKey()** for new shortcuts
- **Enhance drawStatusBar()** with progress formatting
- **Add helper methods**:
  - `formatProgress(position, duration time.Duration) string`
  - `formatPlayerStatus() string`
- **Terminal width detection** for adaptive display

### 4. Data Persistence (`/internal/models/`)
- **Extend Episode struct** usage (Position field already exists)
- **Modify subscriptions.go**:
  - Include episode positions in Save()
  - Restore positions in Load()
  - Maintain backward compatibility
- **JSON Structure**:
  ```json
  {
    "podcasts": [{
      "title": "...",
      "episodes": [{
        "guid": "...",
        "position": 1234,  // seconds
        "played": false
      }]
    }]
  }
  ```

## Implementation Hints and Patterns

### 1. MPV Socket Communication Pattern
```go
type mpvCommand struct {
    Command []interface{} `json:"command"`
}

func (p *Player) sendCommand(cmd mpvCommand) error {
    conn, err := net.Dial("unix", "/tmp/mpv-socket")
    if err != nil {
        return err
    }
    defer conn.Close()
    
    data, _ := json.Marshal(cmd)
    data = append(data, '\n')
    _, err = conn.Write(data)
    return err
}
```

### 2. Status Bar Width Adaptation
```go
func (a *App) formatPlayerStatus(width int) string {
    if width < 40 {
        return fmt.Sprintf("[▶ %s/%s]", formatTime(pos), formatTime(dur))
    }
    // Full format for wider terminals
}
```

### 3. Keyboard Handler Pattern
```go
case tcell.KeyRight:
    if a.player.IsPlaying() {
        go func() {
            if err := a.player.Seek(10); err != nil {
                a.setStatus(fmt.Sprintf("Seek failed: %v", err))
            }
        }()
    }
    return nil
```

## Acceptance Criteria

1. ✓ All keyboard shortcuts work as specified in NORMAL mode
2. ✓ Progress bar shows current position and duration
3. ✓ Playback speed can be adjusted and is displayed
4. ✓ Volume can be adjusted up/down and muted
5. ✓ Episode positions are saved and restored correctly
6. ✓ Status bar adapts to terminal width
7. ✓ Pause/resume functionality works properly
8. ✓ Seek forward/backward works accurately
9. ✓ No regression in existing functionality
10. ✓ Partially played episodes show visual indicator

## Assumptions

1. **MPV Availability**: mpv is installed and accessible in PATH
2. **Socket Path**: `/tmp/mpv-socket` is writable and accessible
3. **Terminal**: Supports UTF-8 for player status symbols
4. **File Permissions**: Config directory is writable for saving progress
5. **Performance**: Progress polling every second is acceptable
6. **Episode Identity**: Episode GUID is stable for position tracking