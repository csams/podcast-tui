package ui

import (
	"strings"
	
	"github.com/junegunn/fzf/src/algo"
	"github.com/junegunn/fzf/src/util"
)

// SearchState holds the state for search functionality
type SearchState struct {
	query           string
	cursorPos       int
	caseSensitive   bool
	lastQuery       string  // Previous query to detect changes
	minScore        int     // Minimum score threshold for matches
}

// Score threshold constants (based on raw fzf scores)
const (
	ScoreThresholdStrict     = 70  // Only high quality matches  
	ScoreThresholdNormal     = 50  // Balanced (default)
	ScoreThresholdPermissive = 30  // Include marginal matches
	ScoreThresholdNone       = 0   // Accept all matches
)


// NewSearchState creates a new search state
func NewSearchState() *SearchState {
	return &SearchState{
		caseSensitive: false,
		minScore:      ScoreThresholdNormal, // Default to balanced threshold
	}
}

// SetQuery sets the search query and resets cursor
func (s *SearchState) SetQuery(query string) {
	s.query = query
	s.cursorPos = len(query)
}

// Clear clears the search state
func (s *SearchState) Clear() {
	s.query = ""
	s.cursorPos = 0
}

// SetMinScore sets the minimum score threshold
func (s *SearchState) SetMinScore(score int) {
	s.minScore = score
}

// GetMinScore returns the current minimum score threshold
func (s *SearchState) GetMinScore() int {
	return s.minScore
}

// InsertChar inserts a character at the cursor position
func (s *SearchState) InsertChar(ch rune) {
	if s.cursorPos >= len(s.query) {
		s.query += string(ch)
	} else {
		s.query = s.query[:s.cursorPos] + string(ch) + s.query[s.cursorPos:]
	}
	s.cursorPos++
}

// DeleteChar deletes the character before the cursor (backspace)
func (s *SearchState) DeleteChar() {
	if s.cursorPos > 0 {
		s.query = s.query[:s.cursorPos-1] + s.query[s.cursorPos:]
		s.cursorPos--
	}
}

// DeleteCharForward deletes the character at the cursor (delete)
func (s *SearchState) DeleteCharForward() {
	if s.cursorPos < len(s.query) {
		s.query = s.query[:s.cursorPos] + s.query[s.cursorPos+1:]
	}
}

// MoveCursorLeft moves cursor left
func (s *SearchState) MoveCursorLeft() {
	if s.cursorPos > 0 {
		s.cursorPos--
	}
}

// MoveCursorRight moves cursor right
func (s *SearchState) MoveCursorRight() {
	if s.cursorPos < len(s.query) {
		s.cursorPos++
	}
}

// MoveCursorStart moves cursor to start (Ctrl+A)
func (s *SearchState) MoveCursorStart() {
	s.cursorPos = 0
}

// MoveCursorEnd moves cursor to end (Ctrl+E)
func (s *SearchState) MoveCursorEnd() {
	s.cursorPos = len(s.query)
}

// DeleteToEnd deletes from cursor to end (Ctrl+K)
func (s *SearchState) DeleteToEnd() {
	s.query = s.query[:s.cursorPos]
}

// DeleteWord deletes the word before cursor (Ctrl+W)
func (s *SearchState) DeleteWord() {
	if s.cursorPos == 0 {
		return
	}
	
	// Find the start of the word
	start := s.cursorPos - 1
	for start > 0 && s.query[start] == ' ' {
		start--
	}
	for start > 0 && s.query[start-1] != ' ' {
		start--
	}
	
	s.query = s.query[:start] + s.query[s.cursorPos:]
	s.cursorPos = start
}

// MoveCursorWordForward moves cursor forward by one word (Alt+F)
func (s *SearchState) MoveCursorWordForward() {
	if s.cursorPos >= len(s.query) {
		return
	}
	
	// If we're in a word, skip to the end of it
	for s.cursorPos < len(s.query) && s.query[s.cursorPos] != ' ' {
		s.cursorPos++
	}
	// Skip spaces to the next word
	for s.cursorPos < len(s.query) && s.query[s.cursorPos] == ' ' {
		s.cursorPos++
	}
}

// MoveCursorWordBackward moves cursor backward by one word (Alt+B)
func (s *SearchState) MoveCursorWordBackward() {
	if s.cursorPos == 0 {
		return
	}
	
	// If we're at the start of a word or in spaces, move back
	if s.cursorPos > 0 && (s.cursorPos == len(s.query) || s.query[s.cursorPos] == ' ' || s.query[s.cursorPos-1] == ' ') {
		// Skip any spaces we're in
		for s.cursorPos > 0 && s.query[s.cursorPos-1] == ' ' {
			s.cursorPos--
		}
		// Skip the previous word to find its start
		for s.cursorPos > 0 && s.query[s.cursorPos-1] != ' ' {
			s.cursorPos--
		}
	} else {
		// We're in the middle of a word, go to its beginning
		for s.cursorPos > 0 && s.query[s.cursorPos-1] != ' ' {
			s.cursorPos--
		}
	}
}

// DeleteWordForward deletes the word after cursor (Alt+D)
func (s *SearchState) DeleteWordForward() {
	if s.cursorPos >= len(s.query) {
		return
	}
	
	// Find the end of the word
	end := s.cursorPos
	// Skip spaces
	for end < len(s.query) && s.query[end] == ' ' {
		end++
	}
	// Skip word
	for end < len(s.query) && s.query[end] != ' ' {
		end++
	}
	
	s.query = s.query[:s.cursorPos] + s.query[end:]
}

// MatchResult contains match score and positions
type MatchResult struct {
	Score     int
	Positions []int
}

// matchScore calculates the match score for a text against the search query
func (s *SearchState) matchScore(text string) int {
	result := s.matchWithPositions(text)
	return result.Score
}

// matchWithPositions calculates match score and positions for highlighting
func (s *SearchState) matchWithPositions(text string) MatchResult {
	if s.query == "" {
		return MatchResult{Score: 0, Positions: nil}
	}
	
	// Initialize fzf algo if needed
	algo.Init("default")
	
	// Convert to lowercase for case-insensitive search
	searchText := text
	pattern := s.query
	if !s.caseSensitive {
		searchText = strings.ToLower(text)
		pattern = strings.ToLower(s.query)
	}
	
	// Create util.Chars from string
	chars := util.ToChars([]byte(searchText))
	patternRunes := []rune(pattern)
	
	// Use fzf v2 algorithm with position tracking
	slab := util.MakeSlab(16384, 1024)
	result, positions := algo.FuzzyMatchV2(s.caseSensitive, false, true, &chars, patternRunes, true, slab)
	
	if result.Start < 0 {
		return MatchResult{Score: -1, Positions: nil}
	}
	
	// Extract match positions
	var matchPositions []int
	if positions != nil {
		// fzf returns positions as indices into the Chars array,
		// which already corresponds to rune positions
		matchPositions = make([]int, len(*positions))
		copy(matchPositions, *positions)
	}
	
	return MatchResult{Score: result.Score, Positions: matchPositions}
}

// MatchEpisode checks if an episode matches the search query
func (s *SearchState) MatchEpisode(title, description string) (bool, int) {
	if s.query == "" {
		return true, 0
	}
	
	// Try matching title first
	titleScore := s.matchScore(title)
	if titleScore >= 0 {
		if s.minScore == 0 || titleScore >= s.minScore {
			return true, titleScore
		}
	}
	
	// Try matching description
	descScore := s.matchScore(description)
	if descScore >= 0 {
		if s.minScore == 0 || descScore >= s.minScore {
			return true, descScore
		}
	}
	
	return false, -1
}

// MatchPodcast checks if a podcast matches the search query
func (s *SearchState) MatchPodcast(title, url, latestEpisode string) (bool, int) {
	if s.query == "" {
		return true, 0
	}
	
	// Try matching title first
	titleScore := s.matchScore(title)
	if titleScore >= 0 {
		if s.minScore == 0 || titleScore >= s.minScore {
			return true, titleScore
		}
	}
	
	// Try matching URL
	urlScore := s.matchScore(url)
	if urlScore >= 0 {
		if s.minScore == 0 || urlScore >= s.minScore {
			return true, urlScore
		}
	}
	
	// Try matching latest episode
	if latestEpisode != "" {
		episodeScore := s.matchScore(latestEpisode)
		if episodeScore >= 0 {
			if s.minScore == 0 || episodeScore >= s.minScore {
				return true, episodeScore
			}
		}
	}
	
	return false, -1
}

// MatchEpisodeWithPositions checks if an episode matches and returns positions for highlighting
func (s *SearchState) MatchEpisodeWithPositions(title, description string) (bool, int, MatchResult) {
	if s.query == "" {
		return true, 0, MatchResult{Score: 0, Positions: nil}
	}
	
	// Try matching title first
	titleResult := s.matchWithPositions(title)
	if titleResult.Score >= 0 {
		if s.minScore == 0 || titleResult.Score >= s.minScore {
			return true, titleResult.Score, titleResult
		}
	}
	
	// Try matching description
	descResult := s.matchWithPositions(description)
	if descResult.Score >= 0 {
		// Check if description match meets minimum score threshold
		if s.minScore == 0 || descResult.Score >= s.minScore {
			return true, descResult.Score, descResult
		}
	}
	
	return false, -1, MatchResult{Score: -1, Positions: nil}
}

// MatchPodcastWithPositions checks if a podcast matches and returns positions for highlighting
func (s *SearchState) MatchPodcastWithPositions(title, url, latestEpisode string) (bool, int, MatchResult, string) {
	if s.query == "" {
		return true, 0, MatchResult{Score: 0, Positions: nil}, ""
	}
	
	// Try matching title first
	titleResult := s.matchWithPositions(title)
	if titleResult.Score >= 0 {
		if s.minScore == 0 || titleResult.Score >= s.minScore {
			return true, titleResult.Score, titleResult, "title"
		}
	}
	
	// Try matching URL
	urlResult := s.matchWithPositions(url)
	if urlResult.Score >= 0 {
		if s.minScore == 0 || urlResult.Score >= s.minScore {
			return true, urlResult.Score, urlResult, "url"
		}
	}
	
	// Try matching latest episode
	if latestEpisode != "" {
		episodeResult := s.matchWithPositions(latestEpisode)
		if episodeResult.Score >= 0 {
			// Latest episode matches need to meet threshold without bonus
			if s.minScore == 0 || episodeResult.Score >= s.minScore {
				return true, episodeResult.Score, episodeResult, "latest"
			}
		}
	}
	
	return false, -1, MatchResult{Score: -1, Positions: nil}, ""
}