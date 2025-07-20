# Detail Questions

## Q6: Should the ensureVisible() implementation center the selected item in the viewport when possible?
**Default if unknown:** Yes (provides better visual context by showing items above and below the selection)

## Q7: Should page-scroll keybindings use Ctrl+F/Ctrl+B (vim-style) or PageUp/PageDown keys?
**Default if unknown:** Yes (Ctrl+F/Ctrl+B to maintain vim consistency, since j/k/g/G are already vim-style)

## Q8: Should the episode list account for the description window when calculating scroll boundaries?
**Default if unknown:** Yes (the description window takes 6 lines at bottom, so scroll calculations should respect this reserved space)

## Q9: Should scrolling past the end of a list wrap around to the beginning, or stop at the boundaries?
**Default if unknown:** No (stop at boundaries - most terminal applications and vim don't wrap, and it's less confusing)

## Q10: Should the scroll offset reset to 0 when switching between podcast and episode views?
**Default if unknown:** No (maintain scroll positions as stated in Q2, but reset selectedIdx to 0 when switching to a new podcast's episodes)