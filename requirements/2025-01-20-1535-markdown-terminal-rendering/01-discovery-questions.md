# Discovery Questions - Markdown Terminal Rendering

## Q1: Will the markdown content come from RSS feeds that may contain various markdown flavors (CommonMark, GitHub Flavored, etc.)?
**Default if unknown:** Yes (RSS feeds often contain various markdown formats and we should handle common variants)

## Q2: Should the markdown rendering preserve inline formatting like bold, italic, and code blocks within the search results?
**Default if unknown:** Yes (users expect formatted text to remain readable even when searching)

## Q3: Will users expect clickable links in markdown to be indicated visually even though they can't be clicked in a terminal?
**Default if unknown:** Yes (visual indication of links helps users understand content structure)

## Q4: Do podcast descriptions currently contain HTML entities or tags that need to be handled alongside markdown?
**Default if unknown:** Yes (RSS feeds often mix HTML and markdown content)

## Q5: Should markdown lists and blockquotes be rendered with special Unicode characters for better visual hierarchy?
**Default if unknown:** Yes (Unicode characters like bullets and quotes improve readability in terminals)