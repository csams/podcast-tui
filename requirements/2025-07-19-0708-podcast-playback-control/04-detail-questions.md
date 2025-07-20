# Expert Requirements Questions for Podcast Playback Control

## Q6: Should we fix the existing mpv IPC implementation in /internal/player/player.go to use proper socket communication instead of shell piping?
**Default if unknown:** Yes (current shell piping approach with exec.Command doesn't work correctly for pause/resume/seek commands)

## Q7: Will the progress data (playback position) be saved in the existing subscriptions.json file at ~/.config/podcast-tui/?
**Default if unknown:** Yes (keeps all podcast data in one place and follows existing persistence pattern)

## Q8: Should volume and playback speed settings persist between application restarts?
**Default if unknown:** No (most media players reset to default volume/speed on restart for predictable behavior)

## Q9: Will the status bar need to dynamically resize the progress display based on terminal width?
**Default if unknown:** Yes (ensures UI remains usable on narrow terminals while showing full info on wider ones)

## Q10: Should keyboard shortcuts follow the existing vim-style pattern (single letters in NORMAL mode)?
**Default if unknown:** Yes (maintains consistency with existing j/k navigation and space for play/pause)