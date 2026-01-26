# Smart Lyrics Alignment Specification

This document describes the algorithm for aligning known lyrics with Whisper transcription timing.

## Implementation Status

| Phase | Description | Status |
|-------|-------------|--------|
| Phase 1 | Structural Analysis | **Implemented** |
| Phase 2 | Enhanced Word Matching | **Implemented** |
| Phase 3 | Needleman-Wunsch Alignment | **Implemented** |
| Phase 4 | Chorus Template Timing | Partial (detection only) |
| Phase 5 | Timing Refinement | **Implemented** |

**Implementation files:**
- `backend/align/lyrics.go` - All lyrics-specific functions
- `backend/align/lyrics_test.go` - Unit tests
- `backend/main.go` - API endpoint with `mode: "lyrics"` support

## Overview

Music videos and karaoke applications require precise subtitle timing. Whisper AI provides accurate timing but often misrecognizes sung words. Users can paste known lyrics to correct transcription errors while preserving accurate timing.

## Current API

### Request

`POST /api/transcribe/{id}/align`

```json
{
  "text": "lyrics text here...",
  "mode": "lyrics",
  "convert_to_script": "devanagari",
  "language": "hi"
}
```

When `mode: "lyrics"`:
- Uses `MusicWordSimilarity` instead of `wordSimilarity`
- Detects section structure and repeats
- Uses Needleman-Wunsch global alignment
- Applies timing refinement

### Response

```json
{
  "status": "success",
  "segments": [...],
  "stats": {
    "user_word_count": 150,
    "whisper_word_count": 145,
    "matched_words": 138,
    "match_rate": 0.92
  }
}
```

## Algorithm Design

### Phase 1: Structural Analysis (Implemented)

Analyzes lyrics to detect sections and repeats:

```go
type Section struct {
    Type      string   // "verse", "chorus", "bridge", "intro", "outro", "unknown"
    Lines     []string
    StartLine int
    EndLine   int
    IsRepeat  bool     // Whether this is a repeat of another section
    SourceIdx int      // If repeat, index of original section
}

type LyricsStructure struct {
    Sections []Section
    AllLines []string
}
```

**Detection heuristics (implemented):**
- Bracketed headers: `[Verse 1]`, `[Chorus]`, `[Bridge]`, `[Pre-Chorus]`, `[Hook]`, `[Refrain]`
- Blank line separators create section breaks
- Repeated line groups detected with 80% match threshold

### Phase 2: Enhanced Word Matching (Implemented)

`MusicWordSimilarity` accounts for singing distortions:

```go
func MusicWordSimilarity(a, b string) float64 {
    base := wordSimilarity(a, b)  // Levenshtein-based

    if isVocalSubstitution(a, b) {
        return max(base, 0.75)
    }
    if isElongated(a, b) || isElongated(b, a) {
        return max(base, 0.85)
    }
    if collapseRepeatedChars(a) == collapseRepeatedChars(b) {
        return max(base, 0.9)
    }
    return base
}
```

**Vocal substitutions (30+ mappings):**
- `you` ↔ `ooh`, `oo`, `ya`, `yo`
- `i` ↔ `ah`, `eye`, `ay`, `ee`
- `love` ↔ `loove`, `luv`, `lov`
- `yeah` ↔ `yea`, `ya`, `yah`, `ye`
- And more common singing distortions

**Elongation detection:**
- `loooove` → `love` (repeated chars collapsed)
- `noooo` → `no`

### Phase 3: Needleman-Wunsch Alignment (Implemented)

Global sequence alignment using dynamic programming:

```go
func NeedlemanWunsch(lyricsWords []string, whisperWords []Word, useMusic bool) []int
```

**Scoring parameters:**
- Match bonus: `2.0 * similarity`
- Mismatch penalty: `-1.0 * (1 - similarity)`
- Gap in lyrics (Whisper missed word): `-0.3`
- Gap in Whisper (extra noise): `-0.15`

Returns alignment array where `result[i]` is the whisper word index aligned to lyrics word `i` (-1 if gap).

### Phase 4: Chorus Template Timing (Partial)

**Implemented:**
- Section detection with repeat marking
- `IsRepeat` and `SourceIdx` fields populated

**Not yet implemented:**
- Using first occurrence timing as template for repeated sections
- Offset estimation based on audio position
- See Task 85 for planned implementation

### Phase 5: Timing Refinement (Implemented)

```go
func refineTiming(segments []Segment) []Segment
```

Three-pass refinement:
1. Ensure minimum segment duration (0.8 seconds)
2. Fill small gaps between segments (< 0.3 seconds)
3. Prevent segment overlap (sanity check)

## Music-Specific Challenges Addressed

| Challenge | Solution |
|-----------|----------|
| Held notes ("Loooove" → "Love") | `isElongated()` + `collapseRepeatedChars()` |
| Whisper errors ("you" → "ooh") | `vocalSubstitutions` map (30+ mappings) |
| Missing better matches | Needleman-Wunsch global alignment |
| Short segments | Minimum duration enforcement (0.8s) |
| Gaps between words | Small gap filling (< 0.3s) |

## Testing

Unit tests in `backend/align/lyrics_test.go`:
- `TestMusicWordSimilarity` - vocal substitutions, elongation
- `TestDetectStructure` - section parsing, repeat detection
- `TestNeedlemanWunsch` - alignment algorithm
- `TestAlignLyrics` - full end-to-end alignment

## Future Improvements

1. **Chorus template timing** (Task 85) - Use first chorus occurrence timing for repeats
2. **Word-level timestamps** - Use `whisper-timestamped` for better precision
3. **Phonetic matching** - Phoneme comparison for better singing recognition
4. **Beat detection** - Align to musical beats for rhythmic subtitles
5. **User section hints** - Manual verse/chorus marking

## See Also

- [subtitler.md](subtitler.md) - Main project specification
- [script-conversion.md](script-conversion.md) - Script/language conversion
