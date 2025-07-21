# Context Findings - Number Key Seeking

## Technical Analysis

### Current Seek Implementation

The player component (`internal/player/player.go`) provides a `Seek(seconds int)` method that:
- Accepts seconds as an integer parameter
- Uses "relative" seek for values ≤ 300 seconds
- Uses "absolute" seek for values > 300 seconds
- Sends commands to mpv via IPC socket

### Existing Seek Keybindings

Current seek operations in `internal/ui/app.go`:
- `f`: Seek forward 30 seconds (relative)
- `b`: Seek backward 30 seconds (relative)
- Arrow Left/Right: Seek backward/forward 10 seconds (relative)
- `R`: Restart episode from beginning (uses dedicated function)

All seek operations follow this pattern:
1. Check player state (not stopped)
2. Execute operation asynchronously
3. Update status message on success/failure

### Keybinding Architecture

The `handleKey()` method in `app.go`:
1. Prioritizes dialogs (help, confirmation)
2. Processes based on mode (Normal, Command, Search)
3. In Normal mode, uses nested switch statements for key handling
4. Number keys (0-9) are currently unused in Normal mode

### Duration and Position Management

- `player.GetDuration()` provides current episode duration
- Duration is continuously updated via progress monitoring
- Position saving happens automatically via existing mechanisms
- The `saveEpisodePosition()` function handles persistence

### Percentage Calculation Strategy

For number key `n` (where n = 0-9):
- Percentage = n × 10%
- Target seconds = duration.Seconds() × (n / 10.0)
- Use absolute seeking (value will exceed 300 seconds threshold)

### Visual Feedback System

Status messages are displayed via:
- `a.statusMessage` field in the app struct
- Messages appear in the status bar immediately
- Format: "Seeking to X%" provides clear feedback

### Integration Points Identified

1. **Primary Implementation**: Add number key cases in `app.go` handleKey()
2. **Help Documentation**: Update help dialog with new keybindings
3. **No Player Modifications**: Existing Seek() method is sufficient
4. **No Model Changes**: Position saving works automatically

### Related Features Analyzed

- Restart function (`R` key) shows pattern for absolute positioning
- Progress bar updates automatically after seeking
- Position ticker ensures UI stays current
- Episode list view properly handles all playback keys

### Files Requiring Modification

1. `internal/ui/app.go` - Add number key handlers
2. `internal/ui/help_dialog.go` - Document new keybindings