# Expert Requirements Answers

## Q6: When the queue view has focus and the user presses 'h', should it switch back to the podcast list (maintaining the current h/l navigation pattern)?
**Answer:** No
- h/l navigation only works within podcast/episode views
- TAB is the only way to navigate to/from queue view

## Q7: Should the queue position/duration column update in real-time like it does in the episode list (every second while playing)?
**Answer:** Yes
- Queue view will show live position updates for playing episode
- Maintains consistency with episode list behavior

## Q8: When an episode is removed from the queue while playing, should playback stop immediately or finish the current episode before stopping?
**Answer:** Stop immediately
- User explicitly removed it, honor that action
- **Note:** Use 'x' key instead of 'd' for removal

## Q9: Should the space bar in the queue view resume/pause the selected episode if it's the currently playing one?
**Answer:** Yes
- Space bar toggles play/pause in queue view (consistent with episode list)
- **Clarification:** In episode list, Enter/'l' now enqueues (not toggles playback)

## Q10: When adding an episode that's already in the queue, should the app show a status message indicating it's already queued?
**Answer:** No
- Silently ignore duplicate additions
- Queue indicators in episode list will show it's already queued

## Additional Clarifications from User:
1. **Episode List Keys:**
   - Space bar: Toggle play/pause for current episode
   - Enter or 'l': Enqueue selected episode (starts playing if first in queue)
2. **Queue Removal Key:** Use 'x' instead of 'd'