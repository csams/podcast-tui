package markdown

import (
	"html"
	"regexp"
	"strings"
	"unicode/utf8"
)

// MarkdownConverter handles conversion of markdown/HTML to terminal-formatted text
type MarkdownConverter struct {
	// Pre-compiled regex patterns for performance
	boldPattern         *regexp.Regexp
	italicPattern       *regexp.Regexp
	codePattern         *regexp.Regexp
	linkPattern         *regexp.Regexp
	htmlLinkPattern     *regexp.Regexp
	headerPattern       *regexp.Regexp
	listItemPattern     *regexp.Regexp
	blockquotePattern   *regexp.Regexp
	htmlTagPattern      *regexp.Regexp
}


// ConversionResult contains the converted text and associated metadata
type ConversionResult struct {
	Text        string
	Styles      []StyleRange
	PositionMap *PositionMap
}

// PositionMap provides bidirectional mapping between original and converted text positions
type PositionMap struct {
	// Maps from original rune position to converted rune position
	origToConverted map[int]int
	// Maps from converted rune position to original rune position
	convertedToOrig map[int]int
	// Edit operations for building the maps
	edits []positionEdit
}

// positionEdit represents a single text transformation
type positionEdit struct {
	origStart    int
	origLen      int
	convertedStart int
	convertedLen int
}

// NewMarkdownConverter creates a new markdown converter with compiled patterns
func NewMarkdownConverter() *MarkdownConverter {
	return &MarkdownConverter{
		// Markdown patterns
		boldPattern:       regexp.MustCompile(`\*\*([^*]+)\*\*|__([^_]+)__`),
		italicPattern:     regexp.MustCompile(`\*([^*]+)\*|_([^_]+)_`),
		codePattern:       regexp.MustCompile("`([^`]+)`"),
		linkPattern:       regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`),
		headerPattern:     regexp.MustCompile(`(?m)^(#{1,6})\s+(.+)$`),
		listItemPattern:   regexp.MustCompile(`(?m)^(\s*)([-*+]|\d+\.)\s+(.+)$`),
		blockquotePattern: regexp.MustCompile(`(?m)^>\s*(.+)$`),
		
		// HTML patterns
		htmlLinkPattern: regexp.MustCompile(`<a\s+(?:[^>]*?\s+)?href="([^"]+)"[^>]*>([^<]+)</a>`),
		htmlTagPattern:  regexp.MustCompile(`<(/?)([^>]+)>`),
	}
}

// Convert processes markdown/HTML text and returns formatted result with position mapping
func (mc *MarkdownConverter) Convert(text string) ConversionResult {
	// Step 1: Decode HTML entities
	text = html.UnescapeString(text)
	
	// Initialize position mapping
	pm := &PositionMap{
		origToConverted: make(map[int]int),
		convertedToOrig: make(map[int]int),
		edits:           []positionEdit{},
	}
	
	// Initialize styles slice
	var styles []StyleRange
	
	// Step 2: Process HTML tags first
	text, htmlStyles := mc.processHTML(text, pm)
	styles = append(styles, htmlStyles...)
	
	// Step 3: Process markdown
	text, mdStyles := mc.processMarkdown(text, pm)
	styles = append(styles, mdStyles...)
	
	// Step 4: Build final position maps
	pm.buildMaps(text)
	
	return ConversionResult{
		Text:        text,
		Styles:      styles,
		PositionMap: pm,
	}
}

// processHTML converts HTML tags to markdown or plain text
func (mc *MarkdownConverter) processHTML(text string, pm *PositionMap) (string, []StyleRange) {
	var styles []StyleRange
	offset := 0
	
	// Process HTML links first (before stripping other tags)
	text = mc.htmlLinkPattern.ReplaceAllStringFunc(text, func(match string) string {
		submatches := mc.htmlLinkPattern.FindStringSubmatch(match)
		if len(submatches) >= 3 {
			url := submatches[1]
			linkText := submatches[2]
			replacement := linkText + " (" + url + ")"
			
			// Record the edit
			matchStart := strings.Index(text[offset:], match) + offset
			pm.addEdit(matchStart, len(match), matchStart, len(replacement))
			offset = matchStart + len(replacement)
			
			return replacement
		}
		return match
	})
	
	// Reset offset for next pass
	offset = 0
	
	// Process other HTML tags
	processedText := text
	for {
		match := mc.htmlTagPattern.FindStringIndex(processedText[offset:])
		if match == nil {
			break
		}
		
		matchStart := offset + match[0]
		matchEnd := offset + match[1]
		tagMatch := processedText[matchStart:matchEnd]
		
		// Parse the tag
		tagSubmatches := mc.htmlTagPattern.FindStringSubmatch(tagMatch)
		if len(tagSubmatches) >= 3 {
			isClosing := tagSubmatches[1] == "/"
			tagContent := tagSubmatches[2]
			// Extract tag name, handling self-closing tags like <br/> or <br />
			tagParts := strings.Fields(tagContent)
			if len(tagParts) == 0 {
				continue
			}
			tagName := strings.ToLower(strings.TrimRight(tagParts[0], "/"))
			
			replacement := ""
			switch tagName {
			case "br":
				replacement = "\n"
			case "p":
				if isClosing {
					replacement = "\n\n"
				}
			case "b", "strong":
				if !isClosing {
					replacement = "**"
				} else {
					replacement = "**"
				}
			case "i", "em":
				if !isClosing {
					replacement = "*"
				} else {
					replacement = "*"
				}
			case "ul", "ol":
				if !isClosing {
					replacement = "\n"
				}
			case "li":
				if !isClosing {
					replacement = "• "
				} else {
					replacement = "\n"
				}
			}
			
			// Apply the replacement
			processedText = processedText[:matchStart] + replacement + processedText[matchEnd:]
			pm.addEdit(matchStart, matchEnd-matchStart, matchStart, len(replacement))
			offset = matchStart + len(replacement)
		} else {
			offset = matchEnd
		}
	}
	
	return processedText, styles
}

// processMarkdown converts markdown syntax to terminal-appropriate format
func (mc *MarkdownConverter) processMarkdown(text string, pm *PositionMap) (string, []StyleRange) {
	var styles []StyleRange
	
	// Track cumulative offset from replacements
	totalOffset := 0
	
	// Process in order: headers, lists, blockquotes, then inline formatting
	
	// 1. Headers - just make them bold
	text = mc.replaceWithStyle(text, mc.headerPattern, func(matches []string) (string, *StyleRange) {
		if len(matches) >= 3 {
			headerText := matches[2]
			
			// Just return the header text without the # symbols
			style := &StyleRange{
				Type:  StyleHeader,
			}
			return headerText + "\n", style
		}
		return matches[0], nil
	}, &styles, pm, &totalOffset)
	
	// 2. Lists
	text = mc.processLists(text, &styles, pm, &totalOffset)
	
	// 3. Blockquotes
	text = mc.replaceWithStyle(text, mc.blockquotePattern, func(matches []string) (string, *StyleRange) {
		if len(matches) >= 2 {
			return "│ " + matches[1], nil
		}
		return matches[0], nil
	}, &styles, pm, &totalOffset)
	
	// 4. Links (markdown style)
	text = mc.replaceWithStyle(text, mc.linkPattern, func(matches []string) (string, *StyleRange) {
		if len(matches) >= 3 {
			linkText := matches[1]
			url := matches[2]
			return linkText + " (" + url + ")", &StyleRange{
				Type:  StyleLink,
			}
		}
		return matches[0], nil
	}, &styles, pm, &totalOffset)
	
	// 5. Code
	text = mc.replaceWithStyle(text, mc.codePattern, func(matches []string) (string, *StyleRange) {
		if len(matches) >= 2 {
			return matches[1], &StyleRange{
				Type:  StyleCode,
			}
		}
		return matches[0], nil
	}, &styles, pm, &totalOffset)
	
	// 6. Bold (process before italic to handle **_text_**)
	text = mc.replaceWithStyle(text, mc.boldPattern, func(matches []string) (string, *StyleRange) {
		// Check which capture group has content
		boldText := matches[1]
		if boldText == "" {
			boldText = matches[2]
		}
		if boldText != "" {
			return boldText, &StyleRange{
				Type:  StyleBold,
			}
		}
		return matches[0], nil
	}, &styles, pm, &totalOffset)
	
	// 7. Italic
	text = mc.replaceWithStyle(text, mc.italicPattern, func(matches []string) (string, *StyleRange) {
		// Check which capture group has content
		italicText := matches[1]
		if italicText == "" && len(matches) > 2 {
			italicText = matches[2]
		}
		if italicText != "" {
			return italicText, &StyleRange{
				Type:  StyleItalic,
			}
		}
		return matches[0], nil
	}, &styles, pm, &totalOffset)
	
	return text, styles
}

// replaceWithStyle is a helper that replaces regex matches and tracks styles
func (mc *MarkdownConverter) replaceWithStyle(text string, pattern *regexp.Regexp, 
	replacer func([]string) (string, *StyleRange), styles *[]StyleRange, 
	pm *PositionMap, totalOffset *int) string {
	
	matches := pattern.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return text
	}
	
	result := strings.Builder{}
	lastEnd := 0
	
	for _, match := range matches {
		// Add text before match
		result.WriteString(text[lastEnd:match[0]])
		
		// Get matched text and submatches
		var submatches []string
		for i := 0; i < len(match)/2; i++ {
			if match[i*2] >= 0 {
				submatches = append(submatches, text[match[i*2]:match[i*2+1]])
			} else {
				submatches = append(submatches, "")
			}
		}
		
		// Apply replacement
		replacement, style := replacer(submatches)
		
		// Calculate positions in the result string (in runes, not bytes)
		startInResult := utf8.RuneCountInString(result.String())
		result.WriteString(replacement)
		endInResult := utf8.RuneCountInString(result.String())
		
		// Add style if provided
		if style != nil {
			style.Start = startInResult
			style.End = endInResult
			*styles = append(*styles, *style)
		}
		
		// Record the edit
		pm.addEdit(match[0], match[1]-match[0], startInResult-*totalOffset, len(replacement))
		*totalOffset += (match[1] - match[0]) - len(replacement)
		
		lastEnd = match[1]
	}
	
	// Add remaining text
	result.WriteString(text[lastEnd:])
	
	return result.String()
}

// processLists handles list items with proper bullet points
func (mc *MarkdownConverter) processLists(text string, styles *[]StyleRange, pm *PositionMap, totalOffset *int) string {
	lines := strings.Split(text, "\n")
	result := strings.Builder{}
	
	for i, line := range lines {
		if match := mc.listItemPattern.FindStringSubmatch(line); match != nil {
			indent := match[1]
			content := match[3]
			
			// Determine nesting level
			level := len(indent) / 2
			if level > 2 {
				level = 2
			}
			
			// Choose bullet based on level
			var bullet string
			switch level {
			case 0:
				bullet = "• "
			case 1:
				bullet = "  ◦ "
			case 2:
				bullet = "    ▸ "
			}
			
			// Build the new line
			newLine := bullet + content
			result.WriteString(newLine)
			
			// Record position mapping
			if i < len(lines)-1 {
				result.WriteString("\n")
			}
		} else {
			result.WriteString(line)
			if i < len(lines)-1 {
				result.WriteString("\n")
			}
		}
	}
	
	return result.String()
}

// addEdit records a text transformation for position mapping
func (pm *PositionMap) addEdit(origStart, origLen, convertedStart, convertedLen int) {
	pm.edits = append(pm.edits, positionEdit{
		origStart:      origStart,
		origLen:        origLen,
		convertedStart: convertedStart,
		convertedLen:   convertedLen,
	})
}

// buildMaps constructs the bidirectional position maps from recorded edits
func (pm *PositionMap) buildMaps(finalText string) {
	// First, establish identity mapping for all positions
	convertedRunes := []rune(finalText)
	
	// For now, create a simple mapping
	// TODO: This needs to be properly implemented with the original text
	for i := 0; i < len(convertedRunes); i++ {
		pm.origToConverted[i] = i
		pm.convertedToOrig[i] = i
	}
	
	// Apply edits to adjust mappings
	for _, edit := range pm.edits {
		offset := edit.convertedLen - edit.origLen
		
		// Adjust all positions after this edit
		for origPos := range pm.origToConverted {
			if origPos > edit.origStart+edit.origLen {
				pm.origToConverted[origPos] += offset
			}
		}
	}
}

// OriginalToConverted maps a position from original text to converted text
func (pm *PositionMap) OriginalToConverted(pos int) int {
	if convertedPos, ok := pm.origToConverted[pos]; ok {
		return convertedPos
	}
	
	// Find nearest mapped position
	bestOrig := -1
	for origPos := range pm.origToConverted {
		if origPos <= pos && origPos > bestOrig {
			bestOrig = origPos
		}
	}
	
	if bestOrig >= 0 {
		offset := pos - bestOrig
		return pm.origToConverted[bestOrig] + offset
	}
	
	return pos // Fallback
}

// ConvertedToOriginal maps a position from converted text to original text  
func (pm *PositionMap) ConvertedToOriginal(pos int) int {
	if origPos, ok := pm.convertedToOrig[pos]; ok {
		return origPos
	}
	
	// Find nearest mapped position
	bestConverted := -1
	for convertedPos := range pm.convertedToOrig {
		if convertedPos <= pos && convertedPos > bestConverted {
			bestConverted = convertedPos
		}
	}
	
	if bestConverted >= 0 {
		offset := pos - bestConverted
		return pm.convertedToOrig[bestConverted] + offset
	}
	
	return pos // Fallback
}

// MapPositions maps an array of positions from original to converted text
func (pm *PositionMap) MapPositions(positions []int) []int {
	mapped := make([]int, len(positions))
	for i, pos := range positions {
		mapped[i] = pm.OriginalToConverted(pos)
	}
	return mapped
}

// Helper function to count runes up to byte position
func runeOffset(s string, byteOffset int) int {
	if byteOffset >= len(s) {
		return utf8.RuneCountInString(s)
	}
	return utf8.RuneCountInString(s[:byteOffset])
}