package align

import (
	"regexp"
	"strings"
)

// Common vocal substitutions in Whisper transcription errors for sung content
var vocalSubstitutions = map[string][]string{
	"you":   {"ooh", "oo", "ya", "yo"},
	"i":     {"ah", "eye", "ay", "ee"},
	"the":   {"da", "tha", "duh"},
	"to":    {"too", "ta", "tuh"},
	"love":  {"loove", "luv", "lov"},
	"yeah":  {"yea", "ya", "yah", "ye"},
	"oh":    {"ooh", "o", "oo"},
	"baby":  {"babe", "bay", "bae"},
	"me":    {"mee", "mi"},
	"we":    {"wee", "wi"},
	"be":    {"bee", "bi"},
	"no":    {"noo", "na"},
	"so":    {"soo", "sa"},
	"go":    {"goo", "ga"},
	"my":    {"ma", "mah"},
	"your":  {"ya", "yur", "yer"},
	"for":   {"fa", "fer", "fo"},
	"and":   {"an", "n"},
	"are":   {"r", "ar"},
	"just":  {"jus", "jes"},
	"come":  {"cum", "com"},
	"want":  {"wanna", "wan"},
	"gonna": {"going", "gon", "gunna"},
	"wanna": {"want", "wan"},
}

// isVocalSubstitution checks if two words are common singing substitutions
func isVocalSubstitution(a, b string) bool {
	a = normalizeWord(a)
	b = normalizeWord(b)

	// Check direct substitution
	if subs, ok := vocalSubstitutions[a]; ok {
		for _, sub := range subs {
			if b == sub {
				return true
			}
		}
	}

	// Check reverse substitution
	if subs, ok := vocalSubstitutions[b]; ok {
		for _, sub := range subs {
			if a == sub {
				return true
			}
		}
	}

	return false
}

// isElongated checks if word 'a' is an elongated version of word 'b'
// e.g., "loooove" is elongated "love"
func isElongated(a, b string) bool {
	a = normalizeWord(a)
	b = normalizeWord(b)

	if len(a) <= len(b) {
		return false
	}

	// Remove repeated characters from 'a' and compare
	collapsed := collapseRepeatedChars(a)
	return collapsed == b
}

// collapseRepeatedChars reduces sequences of repeated chars to single char
// e.g., "loooove" -> "love"
func collapseRepeatedChars(s string) string {
	if len(s) == 0 {
		return s
	}

	var result strings.Builder
	result.WriteByte(s[0])

	for i := 1; i < len(s); i++ {
		if s[i] != s[i-1] {
			result.WriteByte(s[i])
		}
	}

	return result.String()
}

// MusicWordSimilarity calculates similarity with music-specific adjustments
func MusicWordSimilarity(a, b string) float64 {
	// Standard normalized Levenshtein
	base := wordSimilarity(a, b)

	// Boost for common music substitutions
	if isVocalSubstitution(a, b) {
		return max(base, 0.75)
	}

	// Boost for elongated words (looove -> love)
	if isElongated(a, b) || isElongated(b, a) {
		return max(base, 0.85)
	}

	// Boost for collapsed repeated chars
	aNorm := normalizeWord(a)
	bNorm := normalizeWord(b)
	aCollapsed := collapseRepeatedChars(aNorm)
	bCollapsed := collapseRepeatedChars(bNorm)
	if aCollapsed == bCollapsed && aCollapsed != "" {
		return max(base, 0.9)
	}

	return base
}

// Section represents a detected section in lyrics
type Section struct {
	Type      string   // "verse", "chorus", "bridge", "intro", "outro", "unknown"
	Lines     []string // Lines in this section
	StartLine int      // Index of first line in full lyrics
	EndLine   int      // Index of last line (exclusive)
	IsRepeat  bool     // Whether this section is a repeat of another
	SourceIdx int      // If IsRepeat, which section this is a repeat of
}

// LyricsStructure represents the detected structure of lyrics
type LyricsStructure struct {
	Sections []Section
	AllLines []string
}

// sectionHeaderPattern matches common section markers like [Verse 1], [Chorus], etc.
var sectionHeaderPattern = regexp.MustCompile(`^\s*\[(Verse\s*\d*|Chorus|Bridge|Intro|Outro|Pre[-\s]?Chorus|Hook|Refrain)\]?\s*$`)

// DetectStructure analyzes lyrics text and identifies sections
func DetectStructure(lyrics string) LyricsStructure {
	lines := strings.Split(lyrics, "\n")
	structure := LyricsStructure{
		AllLines: make([]string, 0),
	}

	var currentSection *Section
	lineIndex := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for section header
		if match := sectionHeaderPattern.FindStringSubmatch(trimmed); len(match) > 1 {
			// Save previous section if exists
			if currentSection != nil && len(currentSection.Lines) > 0 {
				currentSection.EndLine = lineIndex
				structure.Sections = append(structure.Sections, *currentSection)
			}

			// Start new section
			sectionType := strings.ToLower(match[1])
			sectionType = strings.ReplaceAll(sectionType, " ", "")
			// Normalize section types
			switch {
			case strings.HasPrefix(sectionType, "verse"):
				sectionType = "verse"
			case strings.Contains(sectionType, "chorus") || sectionType == "refrain" || sectionType == "hook":
				sectionType = "chorus"
			case strings.HasPrefix(sectionType, "pre"):
				sectionType = "prechorus"
			}

			currentSection = &Section{
				Type:      sectionType,
				Lines:     make([]string, 0),
				StartLine: lineIndex,
			}
			continue
		}

		// Skip empty lines but use them to detect section boundaries if no headers
		if trimmed == "" {
			if currentSection != nil && len(currentSection.Lines) > 0 {
				// Check if next non-empty line starts a new section
				// (happens when lyrics don't have explicit headers)
				currentSection.EndLine = lineIndex
				structure.Sections = append(structure.Sections, *currentSection)
				currentSection = nil
			}
			continue
		}

		// Add line to current section or create new one
		if currentSection == nil {
			currentSection = &Section{
				Type:      "unknown",
				Lines:     make([]string, 0),
				StartLine: lineIndex,
			}
		}

		currentSection.Lines = append(currentSection.Lines, trimmed)
		structure.AllLines = append(structure.AllLines, trimmed)
		lineIndex++
	}

	// Don't forget the last section
	if currentSection != nil && len(currentSection.Lines) > 0 {
		currentSection.EndLine = lineIndex
		structure.Sections = append(structure.Sections, *currentSection)
	}

	// Detect repeated sections (choruses, etc.)
	structure.detectRepeats()

	return structure
}

// detectRepeats finds sections that are repeats of earlier sections
func (ls *LyricsStructure) detectRepeats() {
	for i := range ls.Sections {
		for j := 0; j < i; j++ {
			if sectionsMatch(&ls.Sections[i], &ls.Sections[j]) {
				ls.Sections[i].IsRepeat = true
				ls.Sections[i].SourceIdx = j
				break
			}
		}
	}
}

// sectionsMatch checks if two sections have the same content
func sectionsMatch(a, b *Section) bool {
	if len(a.Lines) != len(b.Lines) {
		return false
	}

	matchCount := 0
	for i := range a.Lines {
		// Compare normalized lines
		aNorm := strings.ToLower(strings.TrimSpace(a.Lines[i]))
		bNorm := strings.ToLower(strings.TrimSpace(b.Lines[i]))
		if aNorm == bNorm {
			matchCount++
		}
	}

	// Consider sections matching if 80% of lines match
	return float64(matchCount)/float64(len(a.Lines)) >= 0.8
}

// NeedlemanWunsch performs global sequence alignment using the Needleman-Wunsch algorithm
// Returns alignment where result[i] is the whisper word index aligned to lyrics word i (-1 if gap)
func NeedlemanWunsch(lyricsWords []string, whisperWords []Word, useMusic bool) []int {
	n := len(lyricsWords)
	m := len(whisperWords)

	if n == 0 || m == 0 {
		result := make([]int, n)
		for i := range result {
			result[i] = -1
		}
		return result
	}

	// Scoring parameters tuned for lyrics alignment
	matchBonus := 2.0
	mismatchPenalty := -1.0
	gapLyrics := -0.3   // Penalty for skipping lyric word (Whisper missed it)
	gapWhisper := -0.15 // Penalty for skipping Whisper word (extra noise/word)

	// Similarity function
	simFunc := wordSimilarity
	if useMusic {
		simFunc = MusicWordSimilarity
	}

	// DP matrix: dp[i][j] = best score aligning first i lyrics words with first j whisper words
	dp := make([][]float64, n+1)
	for i := range dp {
		dp[i] = make([]float64, m+1)
	}

	// Initialize gap penalties
	for i := 1; i <= n; i++ {
		dp[i][0] = float64(i) * gapLyrics
	}
	for j := 1; j <= m; j++ {
		dp[0][j] = float64(j) * gapWhisper
	}

	// Fill DP matrix
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			sim := simFunc(lyricsWords[i-1], whisperWords[j-1].Text)
			var score float64
			if sim >= 0.5 {
				score = matchBonus * sim
			} else {
				score = mismatchPenalty * (1 - sim)
			}

			dp[i][j] = max(
				dp[i-1][j-1]+score,    // Match/substitute
				dp[i-1][j]+gapLyrics,  // Skip lyric word (gap in whisper)
				dp[i][j-1]+gapWhisper, // Skip whisper word (gap in lyrics)
			)
		}
	}

	// Traceback to find alignment
	alignment := make([]int, n)
	for i := range alignment {
		alignment[i] = -1
	}

	i, j := n, m
	for i > 0 && j > 0 {
		sim := simFunc(lyricsWords[i-1], whisperWords[j-1].Text)
		var score float64
		if sim >= 0.5 {
			score = matchBonus * sim
		} else {
			score = mismatchPenalty * (1 - sim)
		}

		if dp[i][j] == dp[i-1][j-1]+score {
			// Diagonal: match/substitute
			if sim >= 0.5 {
				alignment[i-1] = j - 1
			}
			i--
			j--
		} else if dp[i][j] == dp[i-1][j]+gapLyrics {
			// Vertical: gap in whisper (lyric word not found)
			i--
		} else {
			// Horizontal: gap in lyrics (extra whisper word)
			j--
		}
	}

	return alignment
}

// SectionTiming stores the timing template for a section (relative durations)
type SectionTiming struct {
	LineDurations []float64 // Duration of each line in the section
	LineGaps      []float64 // Gap between consecutive lines (len = LineDurations - 1)
	TotalDuration float64   // Total duration of the section
}

// AlignLyrics aligns lyrics with whisper segments using music-specific algorithm
func AlignLyrics(lyricsText string, whisperSegments []Segment) AlignmentResult {
	// Detect lyrics structure
	structure := DetectStructure(lyricsText)

	// Extract words with timing from whisper
	whisperWords := extractWordsFromSegments(whisperSegments)
	if len(whisperWords) == 0 || len(structure.AllLines) == 0 {
		return AlignmentResult{
			Segments: whisperSegments,
			Stats: AlignStats{
				UserWordCount:    len(splitIntoWords(lyricsText)),
				WhisperWordCount: len(whisperWords),
				MatchedWords:     0,
				MatchRate:        0,
			},
		}
	}

	// Get all lyrics words
	lyricsWords := splitIntoWords(strings.Join(structure.AllLines, " "))

	// Perform global alignment using Needleman-Wunsch with music similarity
	alignment := NeedlemanWunsch(lyricsWords, whisperWords, true)

	// Create segments from aligned data (initial pass)
	segments := createAlignedSegments(structure.AllLines, whisperWords, alignment)

	// Apply chorus template timing for repeated sections
	segments = applyChorusTemplates(segments, structure, alignment, whisperWords)

	// Refine timing
	segments = refineTiming(segments)

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
			UserWordCount:    len(lyricsWords),
			WhisperWordCount: len(whisperWords),
			MatchedWords:     matchedWords,
			MatchRate:        float64(matchedWords) / float64(len(lyricsWords)),
		},
	}
}

// applyChorusTemplates uses timing from first chorus occurrence as template for repeats
func applyChorusTemplates(segments []Segment, structure LyricsStructure, alignment []int, whisperWords []Word) []Segment {
	if len(structure.Sections) == 0 || len(segments) == 0 {
		return segments
	}

	// Build mapping from line index to segment index
	lineToSegment := make(map[int]int)
	segIdx := 0
	for lineIdx := range structure.AllLines {
		if segIdx < len(segments) {
			lineToSegment[lineIdx] = segIdx
			segIdx++
		}
	}

	// Calculate timing templates for source sections (first occurrences)
	sectionTimings := make(map[int]*SectionTiming)
	for sectionIdx, section := range structure.Sections {
		if section.IsRepeat {
			continue // Skip repeats, we'll use the source's timing
		}

		timing := extractSectionTiming(section, segments, lineToSegment)
		if timing != nil {
			sectionTimings[sectionIdx] = timing
		}
	}

	// Apply templates to repeated sections
	for _, section := range structure.Sections {
		if !section.IsRepeat {
			continue
		}

		sourceTiming := sectionTimings[section.SourceIdx]
		if sourceTiming == nil || len(sourceTiming.LineDurations) == 0 {
			continue
		}

		// Get the segments for this repeated section
		repeatSegments := getSectionSegments(section, segments, lineToSegment)
		if len(repeatSegments) == 0 {
			continue
		}

		// Check if this repeated section has poor alignment (needs template)
		if !needsTemplateAlignment(repeatSegments, section, alignment, structure) {
			continue
		}

		// Find the anchor point for this repeat (first segment with good timing)
		anchorStart := findRepeatAnchor(repeatSegments, section, segments, lineToSegment, alignment, whisperWords)
		if anchorStart < 0 {
			continue
		}

		// Apply the template timing
		applyTimingTemplate(repeatSegments, sourceTiming, anchorStart)
	}

	return segments
}

// extractSectionTiming calculates the timing template from a section's segments
func extractSectionTiming(section Section, segments []Segment, lineToSegment map[int]int) *SectionTiming {
	if len(section.Lines) == 0 {
		return nil
	}

	timing := &SectionTiming{
		LineDurations: make([]float64, 0, len(section.Lines)),
		LineGaps:      make([]float64, 0, len(section.Lines)-1),
	}

	var prevEnd float64
	for i, lineIdx := range getLineIndices(section) {
		segIdx, ok := lineToSegment[lineIdx]
		if !ok || segIdx >= len(segments) {
			continue
		}

		seg := segments[segIdx]
		duration := seg.End - seg.Start
		timing.LineDurations = append(timing.LineDurations, duration)

		if i > 0 && prevEnd > 0 {
			gap := seg.Start - prevEnd
			if gap < 0 {
				gap = 0
			}
			timing.LineGaps = append(timing.LineGaps, gap)
		}

		prevEnd = seg.End
	}

	if len(timing.LineDurations) > 0 {
		// Calculate total duration from first segment start to last segment end
		firstLineIdx := section.StartLine
		lastLineIdx := section.EndLine - 1
		if firstSegIdx, ok := lineToSegment[firstLineIdx]; ok {
			if lastSegIdx, ok := lineToSegment[lastLineIdx]; ok {
				if firstSegIdx < len(segments) && lastSegIdx < len(segments) {
					timing.TotalDuration = segments[lastSegIdx].End - segments[firstSegIdx].Start
				}
			}
		}
	}

	return timing
}

// getLineIndices returns the global line indices for a section
func getLineIndices(section Section) []int {
	indices := make([]int, len(section.Lines))
	for i := range section.Lines {
		indices[i] = section.StartLine + i
	}
	return indices
}

// getSectionSegments returns pointers to segments for a section
func getSectionSegments(section Section, segments []Segment, lineToSegment map[int]int) []*Segment {
	result := make([]*Segment, 0, len(section.Lines))
	for _, lineIdx := range getLineIndices(section) {
		if segIdx, ok := lineToSegment[lineIdx]; ok && segIdx < len(segments) {
			result = append(result, &segments[segIdx])
		}
	}
	return result
}

// needsTemplateAlignment checks if a repeated section has poor alignment and needs template
func needsTemplateAlignment(repeatSegments []*Segment, section Section, alignment []int, structure LyricsStructure) bool {
	if len(repeatSegments) == 0 {
		return false
	}

	// Count how many lines in this section have good word matches
	goodMatches := 0
	wordIdx := 0

	// Find starting word index for this section
	for i := 0; i < section.StartLine && i < len(structure.AllLines); i++ {
		wordIdx += len(splitIntoWords(structure.AllLines[i]))
	}

	// Check word matches for each line in section
	for _, line := range section.Lines {
		lineWords := splitIntoWords(line)
		matchedInLine := 0
		for range lineWords {
			if wordIdx < len(alignment) && alignment[wordIdx] >= 0 {
				matchedInLine++
			}
			wordIdx++
		}
		// Consider a line well-matched if > 50% of words matched
		if len(lineWords) > 0 && float64(matchedInLine)/float64(len(lineWords)) > 0.5 {
			goodMatches++
		}
	}

	// Need template if less than half the lines have good matches
	return float64(goodMatches)/float64(len(section.Lines)) < 0.5
}

// findRepeatAnchor finds the start time for the repeated section based on audio position
func findRepeatAnchor(repeatSegments []*Segment, section Section, segments []Segment, lineToSegment map[int]int, alignment []int, whisperWords []Word) float64 {
	// First, try to find anchor from any matched word in this section
	wordIdx := 0

	// Find starting word index for this section's lines
	for lineIdx := 0; lineIdx < section.StartLine; lineIdx++ {
		// Count words in lines before this section
		lineSegIdx, ok := lineToSegment[lineIdx]
		if ok && lineSegIdx < len(segments) {
			wordIdx += len(splitIntoWords(segments[lineSegIdx].Text))
		}
	}

	// Look for first matched word in this section
	for _, line := range section.Lines {
		lineWords := splitIntoWords(line)
		for range lineWords {
			if wordIdx < len(alignment) && alignment[wordIdx] >= 0 {
				whisperIdx := alignment[wordIdx]
				if whisperIdx < len(whisperWords) {
					return whisperWords[whisperIdx].Start
				}
			}
			wordIdx++
		}
	}

	// If no word matches, use the first segment's current start time
	if len(repeatSegments) > 0 {
		return repeatSegments[0].Start
	}

	return -1
}

// applyTimingTemplate applies the timing template to repeated section segments
func applyTimingTemplate(repeatSegments []*Segment, template *SectionTiming, anchorStart float64) {
	if len(repeatSegments) == 0 || template == nil || len(template.LineDurations) == 0 {
		return
	}

	currentStart := anchorStart

	for i, seg := range repeatSegments {
		if i >= len(template.LineDurations) {
			break
		}

		// Add gap before this line (except first line)
		if i > 0 && i-1 < len(template.LineGaps) {
			currentStart += template.LineGaps[i-1]
		}

		// Apply the template duration
		seg.Start = currentStart
		seg.End = currentStart + template.LineDurations[i]
		currentStart = seg.End
	}
}

// refineTiming adjusts segment timing for better subtitle display
func refineTiming(segments []Segment) []Segment {
	if len(segments) == 0 {
		return segments
	}

	minDuration := 0.8

	// First pass: ensure minimum duration, but respect next segment's start time
	for i := range segments {
		currentDuration := segments[i].End - segments[i].Start
		if currentDuration < minDuration {
			desiredEnd := segments[i].Start + minDuration

			// Don't extend past next segment's start
			if i < len(segments)-1 {
				maxEnd := segments[i+1].Start
				if desiredEnd > maxEnd {
					desiredEnd = maxEnd
				}
			}
			segments[i].End = desiredEnd
		}
	}

	// Second pass: fill small gaps between segments
	for i := 0; i < len(segments)-1; i++ {
		gap := segments[i+1].Start - segments[i].End
		if gap > 0 && gap < 0.3 {
			// Small gap: extend current segment to fill
			segments[i].End = segments[i+1].Start
		}
	}

	// Third pass: ensure no overlap (sanity check)
	for i := 0; i < len(segments)-1; i++ {
		if segments[i].End > segments[i+1].Start {
			segments[i].End = segments[i+1].Start
		}
	}

	return segments
}
