# Detail Questions - Number Key Seeking

## Q6: Should the status message show the exact time being sought to (e.g., "Seeking to 50% (15:30)")?
**Default if unknown:** Yes (providing both percentage and time gives users more precise feedback, consistent with other seek operations)

## Q7: Should number keys be disabled when in search mode or command mode to avoid conflicts?
**Default if unknown:** Yes (number keys should only work in Normal mode when episode view is active, following the pattern of other playback controls)

## Q8: Should seeking work when switching between episodes (e.g., pressing '5' immediately after selecting a new episode)?
**Default if unknown:** No (wait for episode to fully load and duration to be available before allowing percentage seeks)

## Q9: Should the feature handle edge cases like very short episodes (< 10 seconds) gracefully?
**Default if unknown:** Yes (always calculate position accurately even for short content, mpv handles fractional seconds)

## Q10: Should the help dialog group number keys with other seek controls in the "Playback Control" section?
**Default if unknown:** Yes (maintains logical grouping of all seeking-related keybindings together)