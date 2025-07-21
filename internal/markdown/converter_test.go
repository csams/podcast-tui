package markdown

import (
	"strings"
	"testing"
)

func TestBrTagConversion(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple br tag",
			input:    "Line 1<br>Line 2",
			expected: "Line 1\nLine 2",
		},
		{
			name:     "Self-closing br tag with slash",
			input:    "Line 1<br/>Line 2",
			expected: "Line 1\nLine 2",
		},
		{
			name:     "Self-closing br tag with space",
			input:    "Line 1<br />Line 2",
			expected: "Line 1\nLine 2",
		},
		{
			name:     "Multiple br tags",
			input:    "Line 1<br>Line 2<br/>Line 3<br />Line 4",
			expected: "Line 1\nLine 2\nLine 3\nLine 4",
		},
		{
			name:     "BR in uppercase",
			input:    "Line 1<BR>Line 2<BR/>Line 3<BR />Line 4",
			expected: "Line 1\nLine 2\nLine 3\nLine 4",
		},
		{
			name:     "Mixed case",
			input:    "Line 1<Br>Line 2<bR/>Line 3<BR />Line 4",
			expected: "Line 1\nLine 2\nLine 3\nLine 4",
		},
	}

	converter := NewMarkdownConverter()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := converter.Convert(tt.input)
			if result.Text != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result.Text)
			}
		})
	}
}

func TestBrTagWithHighlighting(t *testing.T) {
	converter := NewMarkdownConverter()
	
	// Test that highlighting positions are correctly mapped after br tag conversion
	input := "Hello world<br/>This is a test"
	result := converter.Convert(input)
	
	// The converted text should be "Hello world\nThis is a test"
	expected := "Hello world\nThis is a test"
	if result.Text != expected {
		t.Errorf("Expected %q, got %q", expected, result.Text)
	}
	
	// Test that position mapping works correctly
	// The word "test" starts at position 24 in the original (after <br/>)
	// In the converted text, it should be at position 23 (after \n)
	originalTestPos := strings.Index(input, "test")
	convertedTestPos := strings.Index(result.Text, "test")
	
	// Map the position
	mappedPos := result.PositionMap.OriginalToConverted(originalTestPos)
	
	// The mapped position should be close to the actual position in converted text
	// Allow for some variance due to rune vs byte counting
	if abs(mappedPos - convertedTestPos) > 1 {
		t.Errorf("Position mapping incorrect: original pos %d mapped to %d, expected near %d", 
			originalTestPos, mappedPos, convertedTestPos)
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}