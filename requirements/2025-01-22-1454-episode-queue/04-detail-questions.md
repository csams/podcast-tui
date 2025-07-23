# Expert Requirements Questions

These detailed questions clarify specific system behaviors based on the codebase analysis.

## Q6: When the queue view has focus and the user presses 'h', should it switch back to the podcast list (maintaining the current h/l navigation pattern)?
**Default if unknown:** No (h/l should only work within podcast/episode views, not from queue)

## Q7: Should the queue position/duration column update in real-time like it does in the episode list (every second while playing)?
**Default if unknown:** Yes (consistent behavior across all views showing playing episodes)

## Q8: When an episode is removed from the queue while playing, should playback stop immediately or finish the current episode before stopping?
**Default if unknown:** Yes, stop immediately (user explicitly removed it, so honor that action)

## Q9: Should the Enter key in the queue view resume/pause the selected episode if it's the currently playing one?
**Default if unknown:** Yes (consistent with episode list behavior where Enter toggles playback)

## Q10: When adding an episode that's already in the queue, should the app show a status message indicating it's already queued?
**Default if unknown:** No (silently ignore to avoid UI clutter, the queue indicator will show it's already there)