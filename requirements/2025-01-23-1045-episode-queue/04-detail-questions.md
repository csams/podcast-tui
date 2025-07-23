# Expert Requirements Questions

## Q6: Should the queue view remember its scroll position when switching between views?
**Default if unknown:** Yes (consistent with how podcast and episode list views maintain their selection state)

## Q7: Should pressing Enter on an episode in the queue view skip to that episode immediately?
**Default if unknown:** Yes (allows users to reorder playback order by selecting and playing)

## Q8: Should the queue automatically save and resume when the application restarts while an episode is playing?
**Default if unknown:** Yes (ensures queue continuity - if episode 3 of 5 was playing, it should resume at episode 3 with 4 and 5 still queued)

## Q9: Should there be a visual indicator in the queue view showing which episode is currently playing?
**Default if unknown:** Yes (use the same ▶ indicator as in the episode list for consistency)

## Q10: Should the queue position numbers be displayed as "1", "2", "3" or "Q:1", "Q:2", "Q:3" in the status column?
**Default if unknown:** No (just "1", "2", "3" is cleaner since we're already in the queue view)