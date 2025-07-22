package ui

import (
	"strings"
	
	"github.com/csams/podcast-tui/internal/markdown"
)

// PodcastMatchResult stores match result and which field matched
type PodcastMatchResult struct {
	MatchResult
	MatchField string  // "title" or "description"
}

// EpisodeMatchResult stores match result and which field matched
type EpisodeMatchResult struct {
	MatchResult
	MatchField string  // "title" or "description"
}

// styledLineWithHighlights represents a line with both styling and highlight information
type styledLineWithHighlights struct {
	text      string
	positions []int  // Highlight positions
	styles    []markdown.StyleRange  // Style ranges that apply to this line
}

// lineWithHighlights represents a line of text with highlight positions
type lineWithHighlights struct {
	text      string
	positions []int
}

// splitPreservingNewlines splits text into lines, preserving empty lines
func splitPreservingNewlines(text string) []string {
	// Handle empty string case
	if text == "" {
		return []string{""}
	}
	
	// Split by newline but preserve the structure
	lines := strings.Split(text, "\n")
	return lines
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}