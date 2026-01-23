package align

import (
	"testing"
)

func TestCollapseRepeatedChars(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"love", "love"},
		{"loooove", "love"},
		{"looooooove", "love"},
		{"hello", "helo"},
		{"yeaaaah", "yeah"},
		{"", ""},
		{"a", "a"},
		{"aaa", "a"},
	}

	for _, tt := range tests {
		result := collapseRepeatedChars(tt.input)
		if result != tt.expected {
			t.Errorf("collapseRepeatedChars(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestIsElongated(t *testing.T) {
	tests := []struct {
		a        string
		b        string
		expected bool
	}{
		{"loooove", "love", true},
		{"yeaaaah", "yeah", true},
		{"love", "love", false},
		{"love", "loooove", false}, // b is longer than a
		{"cat", "dog", false},
		{"", "", false},
	}

	for _, tt := range tests {
		result := isElongated(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("isElongated(%q, %q) = %v, want %v", tt.a, tt.b, result, tt.expected)
		}
	}
}

func TestIsVocalSubstitution(t *testing.T) {
	tests := []struct {
		a        string
		b        string
		expected bool
	}{
		{"you", "ooh", true},
		{"ooh", "you", true}, // Should work both directions
		{"i", "ah", true},
		{"yeah", "ya", true},
		{"baby", "babe", true},
		{"love", "hate", false},
		{"hello", "world", false},
	}

	for _, tt := range tests {
		result := isVocalSubstitution(tt.a, tt.b)
		if result != tt.expected {
			t.Errorf("isVocalSubstitution(%q, %q) = %v, want %v", tt.a, tt.b, result, tt.expected)
		}
	}
}

func TestMusicWordSimilarity(t *testing.T) {
	tests := []struct {
		a           string
		b           string
		minExpected float64
	}{
		{"love", "love", 1.0},
		{"loooove", "love", 0.85},
		{"you", "ooh", 0.75},
		{"yeah", "ya", 0.75},
		{"hello", "hello", 1.0},
		{"cat", "dog", 0.0},
	}

	for _, tt := range tests {
		result := MusicWordSimilarity(tt.a, tt.b)
		if result < tt.minExpected {
			t.Errorf("MusicWordSimilarity(%q, %q) = %.2f, want >= %.2f", tt.a, tt.b, result, tt.minExpected)
		}
	}
}

func TestDetectStructure(t *testing.T) {
	lyrics := `[Verse 1]
First verse line one
First verse line two

[Chorus]
This is the chorus
It repeats again

[Verse 2]
Second verse here
More lyrics here

[Chorus]
This is the chorus
It repeats again`

	structure := DetectStructure(lyrics)

	// Should have 4 sections
	if len(structure.Sections) != 4 {
		t.Errorf("Expected 4 sections, got %d", len(structure.Sections))
	}

	// First section should be verse
	if structure.Sections[0].Type != "verse" {
		t.Errorf("Expected first section to be 'verse', got %q", structure.Sections[0].Type)
	}

	// Second section should be chorus
	if structure.Sections[1].Type != "chorus" {
		t.Errorf("Expected second section to be 'chorus', got %q", structure.Sections[1].Type)
	}

	// Fourth section should be a repeat of second
	if !structure.Sections[3].IsRepeat {
		t.Error("Expected fourth section (second chorus) to be marked as repeat")
	}

	if structure.Sections[3].SourceIdx != 1 {
		t.Errorf("Expected fourth section source to be 1, got %d", structure.Sections[3].SourceIdx)
	}
}

func TestDetectStructureNoHeaders(t *testing.T) {
	// Lyrics without explicit headers, just blank line separators
	lyrics := `First section line one
First section line two

Second section line one
Second section line two`

	structure := DetectStructure(lyrics)

	// Should detect 2 sections based on blank lines
	if len(structure.Sections) != 2 {
		t.Errorf("Expected 2 sections, got %d", len(structure.Sections))
	}

	// Both should be "unknown" type
	for i, section := range structure.Sections {
		if section.Type != "unknown" {
			t.Errorf("Section %d: expected type 'unknown', got %q", i, section.Type)
		}
	}
}

func TestNeedlemanWunsch(t *testing.T) {
	lyricsWords := []string{"hello", "world", "today"}
	whisperWords := []Word{
		{Text: "hello", Start: 0.0, End: 0.5},
		{Text: "world", Start: 0.5, End: 1.0},
		{Text: "today", Start: 1.0, End: 1.5},
	}

	alignment := NeedlemanWunsch(lyricsWords, whisperWords, false)

	// Should get direct matches
	if len(alignment) != 3 {
		t.Fatalf("Expected alignment length 3, got %d", len(alignment))
	}

	for i, a := range alignment {
		if a != i {
			t.Errorf("alignment[%d] = %d, want %d", i, a, i)
		}
	}
}

func TestNeedlemanWunschWithMissing(t *testing.T) {
	// Whisper missed the middle word
	lyricsWords := []string{"hello", "beautiful", "world"}
	whisperWords := []Word{
		{Text: "hello", Start: 0.0, End: 0.5},
		{Text: "world", Start: 1.0, End: 1.5},
	}

	alignment := NeedlemanWunsch(lyricsWords, whisperWords, false)

	if len(alignment) != 3 {
		t.Fatalf("Expected alignment length 3, got %d", len(alignment))
	}

	// First word should match
	if alignment[0] != 0 {
		t.Errorf("alignment[0] = %d, want 0", alignment[0])
	}

	// Middle word should be -1 (no match)
	if alignment[1] != -1 {
		t.Errorf("alignment[1] = %d, want -1", alignment[1])
	}

	// Last word should match
	if alignment[2] != 1 {
		t.Errorf("alignment[2] = %d, want 1", alignment[2])
	}
}

func TestNeedlemanWunschWithMusicMode(t *testing.T) {
	// Test that music mode handles vocal substitutions
	lyricsWords := []string{"i", "love", "you"}
	whisperWords := []Word{
		{Text: "ah", Start: 0.0, End: 0.3},
		{Text: "loooove", Start: 0.3, End: 0.8},
		{Text: "ooh", Start: 0.8, End: 1.0},
	}

	alignment := NeedlemanWunsch(lyricsWords, whisperWords, true)

	if len(alignment) != 3 {
		t.Fatalf("Expected alignment length 3, got %d", len(alignment))
	}

	// With music mode, "ah" should match "i"
	if alignment[0] != 0 {
		t.Errorf("alignment[0] = %d, want 0 (music mode should match 'i' to 'ah')", alignment[0])
	}

	// "loooove" should match "love"
	if alignment[1] != 1 {
		t.Errorf("alignment[1] = %d, want 1 (music mode should match 'love' to 'loooove')", alignment[1])
	}

	// "ooh" should match "you"
	if alignment[2] != 2 {
		t.Errorf("alignment[2] = %d, want 2 (music mode should match 'you' to 'ooh')", alignment[2])
	}
}

func TestAlignLyrics(t *testing.T) {
	lyricsText := `[Verse]
Hello world today

[Chorus]
Love you forever`

	whisperSegments := []Segment{
		{ID: 0, Start: 0.0, End: 1.5, Text: "Hello world today"},
		{ID: 1, Start: 2.0, End: 3.5, Text: "Loooove ooh forever"},
	}

	result := AlignLyrics(lyricsText, whisperSegments)

	// Should have 2 segments (verse line + chorus line)
	if len(result.Segments) != 2 {
		t.Errorf("Expected 2 segments, got %d", len(result.Segments))
	}

	// Match rate should be reasonable (not 0)
	if result.Stats.MatchRate < 0.5 {
		t.Errorf("Match rate too low: %.2f", result.Stats.MatchRate)
	}

	// First segment text should be from lyrics
	if result.Segments[0].Text != "Hello world today" {
		t.Errorf("First segment text = %q, want %q", result.Segments[0].Text, "Hello world today")
	}

	// Second segment text should be from lyrics (corrected version)
	if result.Segments[1].Text != "Love you forever" {
		t.Errorf("Second segment text = %q, want %q", result.Segments[1].Text, "Love you forever")
	}
}

func TestRefineTiming(t *testing.T) {
	segments := []Segment{
		{ID: 0, Start: 0.0, End: 0.3, Text: "Short"}, // Too short, but can extend
		{ID: 1, Start: 1.5, End: 2.0, Text: "OK"},    // Gap before this (larger gap)
		{ID: 2, Start: 2.5, End: 3.0, Text: "Also OK"},
	}

	refined := refineTiming(segments)

	// First segment should be extended to minimum duration (0.8)
	minDuration := 0.8
	if refined[0].End-refined[0].Start < minDuration {
		t.Errorf("First segment duration = %.2f, want >= %.2f",
			refined[0].End-refined[0].Start, minDuration)
	}

	// First segment should extend to 0.8 seconds (0.0 + 0.8)
	// Gap to next segment is 0.7 which is > 0.3, so no gap filling
	if refined[0].End != 0.8 {
		t.Errorf("First segment end = %.2f, want 0.8", refined[0].End)
	}

	// Should not have overlap
	for i := 0; i < len(refined)-1; i++ {
		if refined[i].End > refined[i+1].Start {
			t.Errorf("Overlap between segments %d and %d: %.2f > %.2f",
				i, i+1, refined[i].End, refined[i+1].Start)
		}
	}
}

func TestRefineTimingConstrainedByNext(t *testing.T) {
	// Test case where short segment can't fully extend due to next segment
	segments := []Segment{
		{ID: 0, Start: 0.0, End: 0.3, Text: "Short"}, // Too short
		{ID: 1, Start: 0.5, End: 1.5, Text: "Close"}, // Very close, constrains extension
	}

	refined := refineTiming(segments)

	// First segment should extend up to next segment's start (0.5), not the full 0.8
	if refined[0].End != 0.5 {
		t.Errorf("First segment end = %.2f, want 0.5 (constrained by next segment)", refined[0].End)
	}

	// Should not have overlap
	if refined[0].End > refined[1].Start {
		t.Errorf("Overlap between segments: %.2f > %.2f", refined[0].End, refined[1].Start)
	}
}
