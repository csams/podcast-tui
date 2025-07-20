# Detail Questions - Markdown Terminal Rendering

## Q6: Should the markdown converter create a new `internal/ui/markdown.go` file to keep the conversion logic separate and reusable?
**Default if unknown:** Yes (follows Go best practices for separation of concerns and makes the feature testable)

## Q7: When converting markdown bold (`**text**`) to terminal display, should we use Unicode mathematical bold characters (𝐭𝐞𝐱𝐭) instead of relying on terminal styling?
**Default if unknown:** No (Unicode bold characters may not display correctly in all terminals and could break text selection/copying)

## Q8: Should HTML anchor tags (`<a href="url">text</a>`) show the URL in parentheses after the link text, like "text (url)"?
**Default if unknown:** No (URLs can be very long and would clutter the description display - just underline or color the link text)

## Q9: Should nested markdown lists maintain their nesting level with multiple bullet types (• for level 1, ◦ for level 2, ▸ for level 3)?
**Default if unknown:** Yes (visual hierarchy helps users understand content structure in the terminal)

## Q10: Should the position mapping between original and converted text be bidirectional to support both search highlighting and potential future features?
**Default if unknown:** Yes (bidirectional mapping is more flexible and only slightly more complex to implement)