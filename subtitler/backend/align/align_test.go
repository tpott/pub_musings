package align

import (
	"math"
	"testing"
)

func TestNormalizeWord(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello", "hello"},
		{"WORLD", "world"},
		{"Hello,", "hello"},
		{"don't", "dont"},
		{"123", "123"},
		{"test123", "test123"},
		{"  spaces  ", "spaces"},
		{"", ""},
		{"!!!", ""},
	}

	for _, tc := range tests {
		result := normalizeWord(tc.input)
		if result != tc.expected {
			t.Errorf("normalizeWord(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

func TestSplitIntoWords(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"hello world", []string{"hello", "world"}},
		{"  multiple   spaces  ", []string{"multiple", "spaces"}},
		{"one", []string{"one"}},
		{"", nil},
		{"   ", nil},
		{"Hello, World!", []string{"Hello,", "World!"}},
	}

	for _, tc := range tests {
		result := splitIntoWords(tc.input)
		if len(result) != len(tc.expected) {
			t.Errorf("splitIntoWords(%q) = %v, expected %v", tc.input, result, tc.expected)
			continue
		}
		for i := range result {
			if result[i] != tc.expected[i] {
				t.Errorf("splitIntoWords(%q)[%d] = %q, expected %q", tc.input, i, result[i], tc.expected[i])
			}
		}
	}
}

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		a, b     string
		expected int
	}{
		{"", "", 0},
		{"a", "", 1},
		{"", "a", 1},
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"kitten", "sitting", 3},
		{"hello", "hallo", 1},
	}

	for _, tc := range tests {
		result := levenshteinDistance(tc.a, tc.b)
		if result != tc.expected {
			t.Errorf("levenshteinDistance(%q, %q) = %d, expected %d", tc.a, tc.b, result, tc.expected)
		}
	}
}

func TestWordSimilarity(t *testing.T) {
	tests := []struct {
		a, b     string
		minScore float64
	}{
		{"hello", "hello", 1.0},
		{"Hello", "hello", 1.0},  // case insensitive
		{"hello,", "hello", 1.0}, // punctuation removed
		{"hello", "hallo", 0.8},  // 1 char different
		{"cat", "dog", 0.0},      // completely different
		{"", "hello", 0.0},       // empty string
	}

	for _, tc := range tests {
		result := wordSimilarity(tc.a, tc.b)
		if result < tc.minScore {
			t.Errorf("wordSimilarity(%q, %q) = %f, expected >= %f", tc.a, tc.b, result, tc.minScore)
		}
	}
}

func TestExtractWordsFromSegments(t *testing.T) {
	segments := []Segment{
		{ID: 0, Start: 0.0, End: 2.0, Text: "Hello world"},
		{ID: 1, Start: 2.0, End: 4.0, Text: "This is a test"},
	}

	words := extractWordsFromSegments(segments)

	if len(words) != 6 {
		t.Errorf("Expected 6 words, got %d", len(words))
	}

	// Check first word timing
	if words[0].Text != "Hello" {
		t.Errorf("First word should be 'Hello', got %q", words[0].Text)
	}
	if words[0].Start != 0.0 {
		t.Errorf("First word start should be 0.0, got %f", words[0].Start)
	}

	// Check word from second segment
	if words[2].Text != "This" {
		t.Errorf("Third word should be 'This', got %q", words[2].Text)
	}
	if words[2].Start < 2.0 {
		t.Errorf("Third word start should be >= 2.0, got %f", words[2].Start)
	}
}

func TestAlignTranscriptExactMatch(t *testing.T) {
	whisperSegments := []Segment{
		{ID: 0, Start: 0.0, End: 2.0, Text: "Hello world"},
		{ID: 1, Start: 2.0, End: 4.0, Text: "How are you"},
	}

	userText := "Hello world\nHow are you"

	result := AlignTranscript(userText, whisperSegments)

	if len(result.Segments) != 2 {
		t.Errorf("Expected 2 segments, got %d", len(result.Segments))
	}

	// Check timing is preserved
	if result.Segments[0].Start != 0.0 {
		t.Errorf("First segment start should be 0.0, got %f", result.Segments[0].Start)
	}

	// Check text is from user
	if result.Segments[0].Text != "Hello world" {
		t.Errorf("First segment text should be 'Hello world', got %q", result.Segments[0].Text)
	}

	// Check stats
	if result.Stats.MatchRate < 0.8 {
		t.Errorf("Match rate should be >= 0.8 for exact match, got %f", result.Stats.MatchRate)
	}
}

func TestAlignTranscriptWithCorrections(t *testing.T) {
	// Whisper misheard some words
	whisperSegments := []Segment{
		{ID: 0, Start: 0.0, End: 3.0, Text: "Hello wrold this is a tset"},
	}

	// User provides correct text
	userText := "Hello world this is a test"

	result := AlignTranscript(userText, whisperSegments)

	if len(result.Segments) != 1 {
		t.Errorf("Expected 1 segment, got %d", len(result.Segments))
	}

	// User text should be used
	if result.Segments[0].Text != userText {
		t.Errorf("Segment text should be user text %q, got %q", userText, result.Segments[0].Text)
	}

	// Timing should be approximately from whisper
	if math.Abs(result.Segments[0].Start-0.0) > 0.1 {
		t.Errorf("Segment start should be ~0.0, got %f", result.Segments[0].Start)
	}
}

func TestAlignTranscriptEmptyInputs(t *testing.T) {
	// Empty user text
	result1 := AlignTranscript("", []Segment{{ID: 0, Start: 0.0, End: 1.0, Text: "test"}})
	if len(result1.Segments) != 1 {
		t.Errorf("Empty user text should return original segments")
	}

	// Empty whisper segments
	result2 := AlignTranscript("test text", []Segment{})
	if len(result2.Segments) != 0 {
		t.Errorf("Empty whisper segments should return empty result")
	}
}

func TestAlignTranscriptMultipleLines(t *testing.T) {
	whisperSegments := []Segment{
		{ID: 0, Start: 0.0, End: 2.0, Text: "First line here"},
		{ID: 1, Start: 2.5, End: 4.0, Text: "Second line now"},
		{ID: 2, Start: 4.5, End: 6.0, Text: "Third and final"},
	}

	userText := `First line here
Second line now
Third and final`

	result := AlignTranscript(userText, whisperSegments)

	if len(result.Segments) != 3 {
		t.Errorf("Expected 3 segments, got %d", len(result.Segments))
	}

	// Check segments are in order with proper timing
	for i := 1; i < len(result.Segments); i++ {
		if result.Segments[i].Start < result.Segments[i-1].Start {
			t.Errorf("Segments should be in time order")
		}
	}
}

func TestSplitIntoLines(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"line1\nline2", []string{"line1", "line2"}},
		{"line1\n\nline2", []string{"line1", "line2"}},
		{"  line1  \n  line2  ", []string{"line1", "line2"}},
		{"single line", []string{"single line"}},
		{"", nil},
		{"\n\n\n", nil},
	}

	for _, tc := range tests {
		result := splitIntoLines(tc.input)
		if len(result) != len(tc.expected) {
			t.Errorf("splitIntoLines(%q) = %v, expected %v", tc.input, result, tc.expected)
			continue
		}
		for i := range result {
			if result[i] != tc.expected[i] {
				t.Errorf("splitIntoLines(%q)[%d] = %q, expected %q", tc.input, i, result[i], tc.expected[i])
			}
		}
	}
}
