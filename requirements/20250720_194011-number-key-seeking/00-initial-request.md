# Initial Request

## Date: 2025-01-20

### User Request
Implement number key seeking functionality where pressing number keys 0-9 during episode playback will seek to the corresponding tenth of the episode duration.

### Description
Add support for quick seeking through episodes using number keys:
- Key 0: Seek to beginning (0%)
- Key 1: Seek to 10% of episode duration
- Key 2: Seek to 20% of episode duration
- Key 3: Seek to 30% of episode duration
- Key 4: Seek to 40% of episode duration
- Key 5: Seek to 50% of episode duration
- Key 6: Seek to 60% of episode duration
- Key 7: Seek to 70% of episode duration
- Key 8: Seek to 80% of episode duration
- Key 9: Seek to 90% of episode duration

This feature allows users to quickly jump to approximate positions in an episode without needing to use the arrow keys for incremental seeking.

### Implementation Context
- Should work in both podcast list and episode list views when an episode is playing
- Requires getting the total duration from the player
- Should calculate the target position as a percentage of total duration
- Visual feedback in the progress bar should update immediately