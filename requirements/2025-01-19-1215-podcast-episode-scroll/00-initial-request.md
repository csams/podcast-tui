# Initial Request

**Timestamp:** 2025-01-19 12:15
**Request:** ensure podcast and episode lists scroll

## Context
User has requested to ensure that both podcast and episode lists in the terminal-based podcast manager have proper scrolling functionality. This appears to be a UI/UX enhancement to handle cases where there are more items than can fit on screen.

## Initial Observations
- This is a terminal-based application (podcast-tui) built with Go and tcell
- There are separate views for podcast lists and episode lists
- Current table format was recently implemented for episode lists
- Need to ensure proper scrolling behavior for both views