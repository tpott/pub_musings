# Smart Lyrics Alignment Specification

This document describes the algorithm for aligning known lyrics with Whisper transcription timing.

## Overview

Music videos and karaoke applications require precise subtitle timing. Whisper AI provides accurate timing but often misrecognizes sung words. Users can paste known lyrics to correct transcription errors while preserving accurate timing.

## Problem Statement

### Current Alignment Limitations

The existing `align.AlignTranscript()` function works well for spoken word but has issues with music:

1. **Greedy matching** - Picks first reasonable match, missing better global alignment
2. **Low similarity threshold** (60%) - Too loose for lyrics where similar sounds are common
3. **Fixed look-ahead** (10 words) - May miss matches when Whisper skips sections
4. **No chorus handling** - Repeated sections confuse the greedy algorithm
5. **Character-based timing interpolation** - Works poorly for stretched/held notes

### Music-Specific Challenges

| Challenge | Example | Impact |
|-----------|---------|--------|
| **Held notes** | "Loooove" → "Love" | Timing stretched beyond word |
| **Melisma** | One syllable, multiple notes | Timing estimation fails |
| **Chorus repeats** | Same lyrics, different timing | Wrong section gets matched |
| **Whisper errors** | "you" → "ooh", "I" → "ah" | Low similarity scores |
| **Background vocals** | Whisper captures backing track | Extra words in transcription |

## Algorithm Design

### Phase 1: Structural Analysis

Before word-level alignment, analyze the lyrics structure:

```go
type LyricsStructure struct {
    Sections []Section  // [Verse 1, Chorus, Verse 2, Chorus, Bridge, Chorus]
    Repeats  []Repeat   // Which sections repeat (indices)
}

type Section struct {
    Type   string   // "verse", "chorus", "bridge", "intro", "outro"
    Lines  []string
    Start  int      // Line index in full lyrics
    End    int
}

type Repeat struct {
    SectionIndex int
    Occurrences  []int // Line indices where this section appears
}
```

**Detection heuristics:**
- Bracketed headers: `[Verse 1]`, `[Chorus]`, `[Bridge]`
- Repeated line groups (exact or near-exact)
- Blank line separators indicate section breaks

### Phase 2: Enhanced Word Matching

Improve word similarity for music context:

```go
// MusicWordSimilarity accounts for singing distortions
func MusicWordSimilarity(a, b string) float64 {
    // Standard normalized Levenshtein
    base := wordSimilarity(a, b)

    // Boost for common music substitutions
    if isVocalSubstitution(a, b) {
        return max(base, 0.7)
    }

    // Boost for elongated words (looove → love)
    if isElongated(a, b) || isElongated(b, a) {
        return max(base, 0.8)
    }

    return base
}

// Common vocal substitutions in transcription errors
var vocalSubstitutions = map[string][]string{
    "you":  {"ooh", "oo", "ya"},
    "i":    {"ah", "eye", "ay"},
    "the":  {"da", "tha"},
    "to":   {"too", "ta"},
    "love": {"loove", "luv"},
    "yeah": {"yea", "ya", "yah"},
    "oh":   {"ooh", "o"},
    "baby": {"babe", "bay"},
}
```

### Phase 3: Global Alignment with Dynamic Programming

Replace greedy matching with Needleman-Wunsch algorithm:

```go
// NeedlemanWunsch finds optimal global alignment between two sequences
func NeedlemanWunsch(lyrics []string, whisper []Word) []int {
    n, m := len(lyrics), len(whisper)

    // Scoring parameters
    match := 2.0      // Reward for matching
    mismatch := -1.0  // Penalty for substitution
    gapLyrics := -0.5 // Penalty for skipping lyric word (Whisper missed it)
    gapWhisper := -0.3 // Penalty for skipping Whisper word (extra word)

    // DP matrix
    dp := make([][]float64, n+1)
    for i := range dp {
        dp[i] = make([]float64, m+1)
    }

    // Initialize gaps
    for i := 1; i <= n; i++ {
        dp[i][0] = float64(i) * gapLyrics
    }
    for j := 1; j <= m; j++ {
        dp[0][j] = float64(j) * gapWhisper
    }

    // Fill matrix
    for i := 1; i <= n; i++ {
        for j := 1; j <= m; j++ {
            sim := MusicWordSimilarity(lyrics[i-1], whisper[j-1].Text)
            score := match * sim + mismatch * (1 - sim)

            dp[i][j] = max(
                dp[i-1][j-1] + score,  // Match/substitute
                dp[i-1][j] + gapLyrics,   // Skip lyric word
                dp[i][j-1] + gapWhisper,  // Skip Whisper word
            )
        }
    }

    // Traceback to get alignment
    return traceback(dp, n, m)
}
```

### Phase 4: Chorus-Aware Alignment

When repeated sections are detected:

1. Find the first occurrence of each section in Whisper output
2. Use first-occurrence timing as template for repeats
3. Adjust template timing based on position in audio

```go
func AlignWithChorusHandling(lyrics LyricsStructure, whisper []Word) []Segment {
    segments := make([]Segment, 0)
    whisperIdx := 0

    for _, section := range lyrics.Sections {
        if section.IsRepeat && section.TemplateIndex >= 0 {
            // Use timing from first occurrence, adjusted for position
            template := segments[section.TemplateIndex]
            offset := estimateOffset(whisperIdx, whisper)
            segments = append(segments, adjustTiming(template, offset))
        } else {
            // Normal alignment for first occurrence
            aligned := alignSection(section, whisper[whisperIdx:])
            segments = append(segments, aligned...)
            whisperIdx += countMatchedWords(aligned)
        }
    }

    return segments
}
```

### Phase 5: Timing Refinement

After alignment, refine segment timing:

```go
func RefineTiming(segments []Segment, whisperWords []Word) []Segment {
    for i := range segments {
        // Expand segment if there's a gap to next segment
        if i < len(segments)-1 {
            gap := segments[i+1].Start - segments[i].End
            if gap > 0 && gap < 0.5 {
                // Small gap: extend current segment to fill
                segments[i].End = segments[i+1].Start
            }
        }

        // Ensure minimum segment duration for readability
        minDuration := 1.0 // seconds
        if segments[i].End - segments[i].Start < minDuration {
            segments[i].End = segments[i].Start + minDuration
        }
    }

    return segments
}
```

## API Changes

### Request Enhancement

The `/api/transcribe/{id}/align` endpoint accepts optional mode:

```json
{
  "text": "lyrics text here...",
  "mode": "lyrics"  // Optional: "lyrics" enables music-specific alignment
}
```

When `mode: "lyrics"`:
- Uses `MusicWordSimilarity` instead of `wordSimilarity`
- Applies chorus detection and template matching
- Uses Needleman-Wunsch instead of greedy alignment

### Response Enhancement

```json
{
  "status": "success",
  "segments": 24,
  "stats": {
    "user_word_count": 150,
    "whisper_word_count": 145,
    "matched_words": 138,
    "match_rate": 0.92,
    "mode": "lyrics",
    "sections_detected": 6,
    "repeated_sections": 2
  }
}
```

## Implementation Plan

### File Changes

| File | Changes |
|------|---------|
| `backend/align/align.go` | Add `MusicWordSimilarity`, `NeedlemanWunsch` |
| `backend/align/lyrics.go` | New file for lyrics-specific functions |
| `backend/align/lyrics_test.go` | Tests for lyrics alignment |
| `backend/main.go` | Add `mode` parameter handling |

### Testing Strategy

1. **Unit tests** for new similarity functions
2. **Integration tests** with known lyrics + simulated Whisper output
3. **Manual verification** with real music video transcriptions

### Test Cases

```go
// Test elongated word matching
TestMusicWordSimilarity("love", "loooove")  // Should be >= 0.8
TestMusicWordSimilarity("you", "ooh")       // Should be >= 0.7

// Test chorus detection
lyrics := `
[Verse 1]
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
It repeats again
`
structure := DetectStructure(lyrics)
assert.Equal(t, 4, len(structure.Sections))
assert.Equal(t, 1, len(structure.Repeats))
```

## Future Improvements

1. **Word-level Whisper timestamps** - Use `whisper-timestamped` for better word timing
2. **Phonetic matching** - Use phoneme comparison for better singing recognition
3. **Beat detection** - Align to musical beats for better rhythmic subtitles
4. **User hints** - Allow marking sections as verse/chorus manually

## See Also

- [subtitler.md](subtitler.md) - Main project specification
- [evaluation-cleanroom.md](evaluation-cleanroom.md) - Model evaluation framework
