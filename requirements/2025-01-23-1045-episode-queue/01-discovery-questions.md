# Discovery Questions

## Q1: Should episodes in the queue be persisted across application restarts?
**Default if unknown:** Yes (consistent with current subscription and position persistence patterns)

## Q2: Should users be able to remove episodes from the queue before they play?
**Default if unknown:** Yes (standard queue management behavior users expect)

## Q3: Should the queue have a maximum size limit to prevent memory issues?
**Default if unknown:** No (podcast episodes are just metadata, not actual audio files in memory)

## Q4: Should adding a duplicate episode to the queue be prevented?
**Default if unknown:** Yes (prevents confusing duplicate entries in the queue)

## Q5: Should the queue view show download status indicators like the episode list?
**Default if unknown:** Yes (maintains UI consistency with existing episode list view)