# Detail Answers - Markdown Terminal Rendering

## Q6: Should the markdown converter create a new `internal/ui/markdown.go` file to keep the conversion logic separate and reusable?
**Answer:** Yes
**Implication:** Create a dedicated markdown module with clean interfaces for conversion and position mapping.

## Q7: When converting markdown bold (`**text**`) to terminal display, should we use Unicode mathematical bold characters (𝐭𝐞𝐱𝐭) instead of relying on terminal styling?
**Answer:** Use terminal styling
**Implication:** Use tcell styles (Bold, Italic, Underline) for text formatting instead of Unicode characters.

## Q8: Should HTML anchor tags (`<a href="url">text</a>`) show the URL in parentheses after the link text, like "text (url)"?
**Answer:** Yes (show URL in parentheses)
**Implication:** Convert `<a href="url">text</a>` to "text (url)" format for clarity.

## Q9: Should nested markdown lists maintain their nesting level with multiple bullet types (• for level 1, ◦ for level 2, ▸ for level 3)?
**Answer:** Yes
**Implication:** Implement multi-level bullet rendering with different Unicode characters per level.

## Q10: Should the position mapping between original and converted text be bidirectional to support both search highlighting and potential future features?
**Answer:** Yes
**Implication:** Build a bidirectional position mapping system for maximum flexibility.