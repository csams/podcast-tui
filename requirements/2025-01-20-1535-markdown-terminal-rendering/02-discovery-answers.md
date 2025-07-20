# Discovery Answers - Markdown Terminal Rendering

## Q1: Will the markdown content come from RSS feeds that may contain various markdown flavors (CommonMark, GitHub Flavored, etc.)?
**Answer:** Yes (default accepted)
**Implication:** We need a robust markdown parser that can handle common markdown variants found in RSS feeds.

## Q2: Should the markdown rendering preserve inline formatting like bold, italic, and code blocks within the search results?
**Answer:** Yes
**Implication:** The markdown-to-Unicode conversion must maintain character positions for search highlighting to work correctly.

## Q3: Will users expect clickable links in markdown to be indicated visually even though they can't be clicked in a terminal?
**Answer:** Yes
**Implication:** Links should be rendered with distinctive formatting (colors/underlines) for visual identification.

## Q4: Do podcast descriptions currently contain HTML entities or tags that need to be handled alongside markdown?
**Answer:** Yes
**Implication:** We need to decode HTML entities and potentially strip HTML tags before markdown processing.

## Q5: Should markdown lists and blockquotes be rendered with special Unicode characters for better visual hierarchy?
**Answer:** Yes
**Implication:** Use Unicode bullet points, indentation markers, and quote indicators for better visual structure.