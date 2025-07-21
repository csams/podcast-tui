# Requirements Specification - Number Key Seeking

## Problem Statement
Users currently must use relative seeking (forward/backward by fixed amounts) or restart from the beginning to navigate within podcast episodes. This makes it difficult to quickly jump to specific portions of an episode, such as returning to the middle after a restart or skipping introductions that are typically a fixed percentage of the episode length.

## Solution Overview
Implement number key shortcuts (0-9) that allow users to instantly seek to percentage-based positions within the currently playing or paused episode. Each number key represents a tenth of the episode duration:
- 0 = 0% (beginning)
- 1 = 10%
- 2 = 20%
- ... 
- 9 = 90%

## Functional Requirements

### FR1: Number Key Seeking
- Number keys 0-9 trigger percentage-based seeking when pressed
- Works in both playing and paused states
- Only active when an episode is loaded in the player
- Only works in Normal mode (not in search or command modes)
- Only works in episode list view

### FR2: Position Calculation
- Key `n` seeks to position: `duration × (n / 10.0)`
- Use absolute positioning (not relative)
- Handle fractional seconds for short episodes
- Gracefully handle edge cases (very short episodes < 10 seconds)

### FR3: Visual Feedback
- Show status message with percentage and time: "Seeking to 50% (15:30)"
- Format time as MM:SS or HH:MM:SS based on duration
- Message appears immediately upon keypress
- Message remains visible using existing status message timeout

### FR4: Integration Requirements
- Respect existing position-saving mechanism
- Position updates automatically after seek
- Progress bar reflects new position
- Real-time position updates continue after seek

### FR5: Error Handling
- Do nothing if player is stopped
- Do nothing if duration is not yet available
- Do nothing if episode is still loading
- Fail silently in edge cases (no error messages for invalid states)

## Technical Requirements

### TR1: Keybinding Implementation
Add to `internal/ui/app.go` in `handleKey()` method, inside the Normal mode switch:
```go
case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
    if a.currentView == a.episodes && a.player.GetState() != player.StateStopped {
        duration := a.player.GetDuration()
        if duration > 0 {
            percentage := float64(ev.Rune() - '0') / 10.0
            targetSeconds := int(duration.Seconds() * percentage)
            
            go func() {
                if err := a.player.Seek(targetSeconds); err != nil {
                    a.statusMessage = fmt.Sprintf("Seek error: %v", err)
                } else {
                    // Format time display
                    targetTime := time.Duration(targetSeconds) * time.Second
                    a.statusMessage = fmt.Sprintf("Seeking to %d%% (%s)", 
                        int(percentage*100), formatDuration(targetTime))
                }
            }()
        }
    }
    return true
```

### TR2: Duration Formatting
Use existing duration formatting or implement simple helper:
```go
func formatDuration(d time.Duration) string {
    hours := int(d.Hours())
    minutes := int(d.Minutes()) % 60
    seconds := int(d.Seconds()) % 60
    
    if hours > 0 {
        return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
    }
    return fmt.Sprintf("%d:%02d", minutes, seconds)
}
```

### TR3: Help Dialog Update
Update `internal/ui/help_dialog.go` in the "Playback Control" section:
```go
"  0-9           Seek to 0%-90% of episode duration",
```

Insert after existing seek controls (f, b, arrow keys) for logical grouping.

### TR4: Mode and View Checks
- Verify `a.mode == ModeNormal` (implicit in switch structure)
- Verify `a.currentView == a.episodes` 
- Verify player state is not stopped
- Verify duration > 0 before calculating position

## Implementation Notes

### Seek Method Usage
- Values > 300 automatically trigger absolute seek in existing implementation
- Most podcast episodes exceed 5 minutes, ensuring absolute mode
- For short content, the seek will still work correctly

### Position Saving
- No modifications needed to position saving
- Existing `saveEpisodePosition()` handles all updates
- Position ticker continues to update saved position

### Timing Considerations
- Wait for duration to be available before allowing seeks
- Duration is populated quickly after playback starts
- Progress watcher ensures duration stays current

## Acceptance Criteria

### AC1: Basic Functionality
- [ ] Pressing 0 seeks to 0% (beginning) of episode
- [ ] Pressing 5 seeks to 50% (middle) of episode
- [ ] Pressing 9 seeks to 90% of episode
- [ ] All number keys 0-9 are implemented

### AC2: State Handling
- [ ] Works when episode is playing
- [ ] Works when episode is paused
- [ ] Does nothing when player is stopped
- [ ] Does nothing when no episode is loaded

### AC3: Visual Feedback
- [ ] Status message shows percentage and time
- [ ] Time format adjusts for episode length
- [ ] Message appears immediately
- [ ] Error messages shown for seek failures

### AC4: Mode Integration
- [ ] Only works in Normal mode
- [ ] Disabled in Search mode
- [ ] Disabled in Command mode
- [ ] Only works in episode list view

### AC5: Edge Cases
- [ ] Handles very short episodes (< 10 seconds)
- [ ] Handles very long episodes (> 1 hour)
- [ ] Gracefully handles missing duration
- [ ] Works correctly after episode switch

### AC6: Documentation
- [ ] Help dialog updated with new keybinding
- [ ] Keybinding listed in logical position
- [ ] Clear description of functionality

## Assumptions
1. Users understand percentage-based navigation
2. 10% increments provide sufficient granularity
3. Visual feedback is sufficient without preview
4. Existing seek infrastructure is reliable
5. Episode durations are accurate when available

## Out of Scope
- Configurable percentage increments
- Visual preview of target position
- Seeking in podcast list view
- Seeking without a loaded episode
- Custom keybinding configuration
- Seek position markers or chapters