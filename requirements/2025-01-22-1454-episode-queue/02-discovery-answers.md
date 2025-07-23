# Discovery Answers

## Q1: Should the queue persist between application sessions?
**Answer:** Yes
- Queue will be saved to disk (similar to subscriptions.json)
- Queue will be restored on application startup

## Q2: Should the queue support duplicate episodes (same episode added multiple times)?
**Answer:** No
- Attempting to add an already-queued episode will be ignored
- Prevents confusion and accidental duplicates

## Q3: Should the queue automatically advance to the next episode when one finishes?
**Answer:** Yes
- When an episode completes, the next in queue will automatically start
- Provides continuous playback experience

## Q4: Should there be a visual indicator in the episode list showing which episodes are in the queue?
**Answer:** Yes
- Episodes in the queue will have a visual marker in the episode list view
- Helps users track what's already queued

## Q5: Should the queue have a maximum size limit?
**Answer:** No
- Users can queue as many episodes as they want
- No artificial restrictions on queue size