// Package align implements text alignment between user-provided transcripts
// and whisper-generated segments with timing information.
package align

import (
	"regexp"
	"strings"
	"unicode"
)

// Segment represents a timed text segment
type Segment struct {
	ID    int     `json:"id"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

// Word represents a single word with timing
type Word struct {
	Text  string
	Start float64
	End   float64
}

// normalizeWord converts a word to lowercase and removes punctuation for comparison
func normalizeWord(word string) string {
	// Remove punctuation and convert to lowercase
	var result strings.Builder
	for _, r := range word {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			result.WriteRune(unicode.ToLower(r))
		}
	}
	return result.String()
}

// splitIntoWords splits text into words, preserving original form
func splitIntoWords(text string) []string {
	// Split on whitespace and filter empty strings
	re := regexp.MustCompile(`\S+`)
	return re.FindAllString(text, -1)
}

// extractWordsFromSegments extracts individual words with interpolated timing
// from whisper segments. Since whisper gives us segment-level timing, we
// interpolate word timing based on character position within the segment.
func extractWordsFromSegments(segments []Segment) []Word {
	var words []Word

	for _, seg := range segments {
		segWords := splitIntoWords(seg.Text)
		if len(segWords) == 0 {
			continue
		}

		// Calculate total character count (excluding spaces)
		totalChars := 0
		for _, w := range segWords {
			totalChars += len(w)
		}

		// Duration per character for interpolation
		duration := seg.End - seg.Start
		if totalChars == 0 {
			totalChars = 1
		}
		charDuration := duration / float64(totalChars)

		// Assign timing to each word based on character position
		charPos := 0
		for _, w := range segWords {
			wordStart := seg.Start + float64(charPos)*charDuration
			wordEnd := seg.Start + float64(charPos+len(w))*charDuration
			words = append(words, Word{
				Text:  w,
				Start: wordStart,
				End:   wordEnd,
			})
			charPos += len(w)
		}
	}

	return words
}

// levenshteinDistance calculates the edit distance between two strings
func levenshteinDistance(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}

	// Create matrix
	matrix := make([][]int, len(a)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(b)+1)
		matrix[i][0] = i
	}
	for j := range matrix[0] {
		matrix[0][j] = j
	}

	// Fill matrix
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len(a)][len(b)]
}

// wordSimilarity returns a similarity score between 0 and 1
func wordSimilarity(a, b string) float64 {
	a = normalizeWord(a)
	b = normalizeWord(b)

	if a == b {
		return 1.0
	}
	if len(a) == 0 || len(b) == 0 {
		return 0.0
	}

	dist := levenshteinDistance(a, b)
	maxLen := max(len(a), len(b))
	return 1.0 - float64(dist)/float64(maxLen)
}

// AlignmentResult represents the result of aligning user text with whisper output
type AlignmentResult struct {
	Segments []Segment `json:"segments"`
	Stats    AlignStats `json:"stats"`
}

// AlignStats provides statistics about the alignment
type AlignStats struct {
	UserWordCount    int     `json:"user_word_count"`
	WhisperWordCount int     `json:"whisper_word_count"`
	MatchedWords     int     `json:"matched_words"`
	MatchRate        float64 `json:"match_rate"`
}

// AlignTranscript aligns user-provided transcript text with whisper segments,
// using the user's text with whisper's timing information.
//
// The algorithm:
// 1. Extract words with timing from whisper segments
// 2. Split user transcript into sentences/lines for segment boundaries
// 3. Use dynamic programming to align user words with whisper words
// 4. Create new segments using user text with interpolated timing from matches
func AlignTranscript(userText string, whisperSegments []Segment) AlignmentResult {
	// Extract words with timing from whisper
	whisperWords := extractWordsFromSegments(whisperSegments)
	if len(whisperWords) == 0 {
		// No whisper words, return empty result
		return AlignmentResult{
			Segments: []Segment{},
			Stats: AlignStats{
				UserWordCount:    len(splitIntoWords(userText)),
				WhisperWordCount: 0,
				MatchedWords:     0,
				MatchRate:        0,
			},
		}
	}

	// Split user text into lines (potential segment boundaries)
	userLines := splitIntoLines(userText)
	userWords := splitIntoWords(userText)

	if len(userWords) == 0 {
		return AlignmentResult{
			Segments: whisperSegments, // Return original if no user text
			Stats: AlignStats{
				UserWordCount:    0,
				WhisperWordCount: len(whisperWords),
				MatchedWords:     0,
				MatchRate:        0,
			},
		}
	}

	// Find optimal alignment between user words and whisper words
	alignment := findAlignment(userWords, whisperWords)

	// Create segments from aligned data
	segments := createAlignedSegments(userLines, whisperWords, alignment)

	// Calculate stats
	matchedWords := 0
	for _, a := range alignment {
		if a >= 0 {
			matchedWords++
		}
	}

	return AlignmentResult{
		Segments: segments,
		Stats: AlignStats{
			UserWordCount:    len(userWords),
			WhisperWordCount: len(whisperWords),
			MatchedWords:     matchedWords,
			MatchRate:        float64(matchedWords) / float64(len(userWords)),
		},
	}
}

// splitIntoLines splits text into lines, filtering empty ones
func splitIntoLines(text string) []string {
	lines := strings.Split(text, "\n")
	var result []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

// findAlignment uses dynamic programming to find the best alignment
// between user words and whisper words. Returns an array where
// alignment[i] = j means user word i aligns with whisper word j
// (-1 means no match).
func findAlignment(userWords []string, whisperWords []Word) []int {
	n := len(userWords)
	m := len(whisperWords)

	// dp[i][j] = best score aligning first i user words with first j whisper words
	// We use a simplified approach: greedy matching with look-ahead
	alignment := make([]int, n)
	for i := range alignment {
		alignment[i] = -1
	}

	whisperIdx := 0
	for userIdx := 0; userIdx < n; userIdx++ {
		userWord := userWords[userIdx]
		bestMatch := -1
		bestScore := 0.0
		bestWhisperIdx := -1

		// Look ahead in whisper words for a good match
		// Allow some skipping to handle missing/extra words
		lookAhead := min(10, m-whisperIdx)
		for j := 0; j < lookAhead; j++ {
			wIdx := whisperIdx + j
			if wIdx >= m {
				break
			}
			score := wordSimilarity(userWord, whisperWords[wIdx].Text)
			// Penalize skipping words (prefer closer matches)
			adjustedScore := score - float64(j)*0.05
			if adjustedScore > bestScore && score >= 0.6 {
				bestScore = adjustedScore
				bestMatch = j
				bestWhisperIdx = wIdx
			}
		}

		if bestMatch >= 0 {
			alignment[userIdx] = bestWhisperIdx
			whisperIdx = bestWhisperIdx + 1
		}
	}

	return alignment
}

// createAlignedSegments creates new segments using user text with whisper timing
func createAlignedSegments(userLines []string, whisperWords []Word, alignment []int) []Segment {
	if len(userLines) == 0 || len(whisperWords) == 0 {
		return []Segment{}
	}

	var segments []Segment
	wordIdx := 0

	for lineIdx, line := range userLines {
		lineWords := splitIntoWords(line)
		if len(lineWords) == 0 {
			continue
		}

		// Find timing for this line's first and last words
		var startTime, endTime float64
		foundStart := false
		foundEnd := false

		lineStartWordIdx := wordIdx
		lineEndWordIdx := wordIdx + len(lineWords) - 1

		// Find start time from first matched word in this line
		for i := lineStartWordIdx; i <= lineEndWordIdx && i < len(alignment); i++ {
			if alignment[i] >= 0 {
				startTime = whisperWords[alignment[i]].Start
				foundStart = true
				break
			}
		}

		// Find end time from last matched word in this line
		for i := lineEndWordIdx; i >= lineStartWordIdx && i < len(alignment); i-- {
			if alignment[i] >= 0 {
				endTime = whisperWords[alignment[i]].End
				foundEnd = true
				break
			}
		}

		// If no matches in this line, interpolate from surrounding segments
		if !foundStart || !foundEnd {
			// Try to interpolate from previous/next segments
			if len(segments) > 0 {
				startTime = segments[len(segments)-1].End
			} else if len(whisperWords) > 0 {
				startTime = whisperWords[0].Start
			}

			// Estimate end time based on word count
			avgWordDuration := 0.3 // rough estimate of seconds per word
			endTime = startTime + float64(len(lineWords))*avgWordDuration
		}

		// Ensure end > start
		if endTime <= startTime {
			endTime = startTime + 0.5
		}

		segments = append(segments, Segment{
			ID:    lineIdx,
			Start: startTime,
			End:   endTime,
			Text:  line,
		})

		wordIdx += len(lineWords)
	}

	return segments
}
