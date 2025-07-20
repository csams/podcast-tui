# Detail Answers

## Q6: Should the ensureVisible() implementation center the selected item in the viewport when possible?
**Answer:** Yes

## Q7: Should page-scroll keybindings use Ctrl+F/Ctrl+B (vim-style) or PageUp/PageDown keys?
**Answer:** Follow the vim pattern (Ctrl+F/Ctrl+B)

## Q8: Should the episode list account for the description window when calculating scroll boundaries?
**Answer:** Yes, and make the description window 10 lines instead of 6

## Q9: Should scrolling past the end of a list wrap around to the beginning, or stop at the boundaries?
**Answer:** Stop at the boundaries

## Q10: Should the scroll offset reset to 0 when switching between podcast and episode views?
**Answer:** No (maintain scroll positions as stated in Q2, but reset selectedIdx to 0 when switching to a new podcast's episodes)