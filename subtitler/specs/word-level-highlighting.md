# Word-Level Highlighting Specification

**Status: Backend Infrastructure Complete - Frontend Display Pending**

This feature has backend infrastructure in place for word-level timing, but the frontend karaoke-style display is not yet implemented.

## Implementation Status

### Backend Infrastructure (Complete)
- `backend/align/align.go` defines `Word` struct with Start/End timing
- `extractWordsFromSegments()` interpolates word timing from segment-level data
- Word timing is computed during alignment operations
- Infrastructure ready to persist word timing if/when Whisper word timestamps are enabled

### Frontend Display (Pending)
- Karaoke-style word highlighting during playback is not yet implemented
- Requires Phase 2 work (see below) to render words with time-synced highlighting
- Would allow users to see each word highlight as it's spoken

---

This document outlines a plan for implementing word-level highlighting during video playback, similar to karaoke-style subtitle display where individual words highlight as they are spoken.

## Overview

Word-level highlighting provides fine-grained synchronization between audio and displayed subtitles. Instead of showing an entire subtitle segment at once, individual words are highlighted (or revealed) as they are spoken.

## Current State

Currently, subtitler displays subtitles at the **segment level**:
- A segment spans multiple seconds (e.g., 0:05 - 0:08)
- The entire segment text appears at once
- Bionic reading applies at display time but doesn't change during playback

## Proposed Enhancement

Word-level highlighting would:
1. Show the full segment text at segment start time
2. Highlight (bold, color, or reveal) each word as it is spoken
3. Combine with bionic reading for enhanced focus

## Data Requirements

### Current Data Structure

```typescript
interface TranscriptionSegment {
  id: number;
  start: number;  // segment start time in seconds
  end: number;    // segment end time in seconds
  text: string;   // full segment text
}
```

### Required Data Structure for Word-Level

```typescript
interface WordTiming {
  word: string;
  start: number;  // word start time in seconds
  end: number;    // word end time in seconds
}

interface TranscriptionSegment {
  id: number;
  start: number;
  end: number;
  text: string;
  words?: WordTiming[];  // Optional word-level timing
}
```

## Whisper Server Output

### Standard Output (what we currently use)
Whisper outputs segment-level JSON by default:
```json
{
  "segments": [
    {"start": 0.0, "end": 3.5, "text": "Hello world"}
  ]
}
```

### Word-Level Output
Whisper supports word-level timestamps with `--word_timestamps True`:
```json
{
  "segments": [
    {
      "start": 0.0,
      "end": 3.5,
      "text": "Hello world",
      "words": [
        {"word": "Hello", "start": 0.0, "end": 0.8},
        {"word": "world", "start": 1.2, "end": 1.8}
      ]
    }
  ]
}
```

## Implementation Phases

### Phase 1: Backend Updates

1. **Update whisper-server call** to request word timestamps
   - Add `--word_timestamps True` to transcription command
   - Parse and store word-level data

2. **Update database schema**
   - Store word timing data in transcription results
   - Option A: JSON column for words array (simple)
   - Option B: Separate words table with FK to segments (normalized)

3. **Update API responses**
   - Include word timing in transcription GET response
   - Add query param to request word-level data (to avoid bloating responses)

### Phase 2: Frontend Updates

1. **Update segment rendering**
   - Parse word timing data
   - Track current word based on video currentTime
   - Render words with highlighting class

2. **CSS for word highlighting**
   ```css
   .word { opacity: 0.5; }
   .word.spoken { opacity: 1; font-weight: bold; }
   /* or karaoke-style reveal */
   .word.spoken { color: var(--highlight-color); }
   ```

3. **Combine with bionic reading**
   - Apply bionic formatting to each word
   - Add spoken/highlight styling on top
   - Example: `<span class="word spoken"><strong>He</strong>llo</span>`

### Phase 3: Performance Optimization

1. **Efficient time tracking**
   - Use binary search for current word lookup
   - Debounce/throttle timeupdate handler

2. **DOM efficiency**
   - Pre-render all word spans
   - Toggle classes instead of re-rendering

3. **Memory considerations**
   - Word data increases response size ~3-5x
   - Consider lazy loading word data on modal open

## Design Questions

### Q1: Does every word need substitution during playback?

**Answer: No.**

The approach is to:
1. Render all words at segment start
2. Toggle CSS classes to highlight current word
3. No DOM manipulation during playback (just class changes)

```javascript
// On segment start, render once:
segment.innerHTML = words.map((w, i) =>
  `<span class="word" data-idx="${i}">${formatWord(w)}</span>`
).join(' ');

// On timeupdate, just toggle classes:
words.forEach((w, i) => {
  const el = segment.querySelector(`[data-idx="${i}"]`);
  el.classList.toggle('spoken', currentTime >= w.start);
});
```

### Q2: Do we persist full JSON from whisper-server?

**Answer: Yes, recommended.**

Benefits:
- Word timing data is expensive to regenerate
- Enables future features (search by word, export with timing)
- Only ~3-5x size increase (acceptable for our use case)

Storage options:
1. **JSON column**: Simple, flexible, good for our SQLite backend
2. **Separate table**: More complex, better for queries, not needed initially

**Recommendation**: Store word timing in a JSON column on the transcription record. This keeps the schema simple while preserving all data.

### Q3: How to align with existing bionic reading?

The rendering flow becomes:

```
1. Get segment text + word timings
2. For each word:
   a. Apply bionic formatting (bold first portion)
   b. Wrap in <span class="word">
   c. Add data-start/data-end attributes
3. On timeupdate:
   a. Find current word by time
   b. Add "spoken" class to current + all previous words
```

## Database Migration

```sql
-- Add column for word-level timing
ALTER TABLE transcriptions
ADD COLUMN word_timestamps TEXT;  -- JSON array of word timings
```

Or store per-segment:
```sql
-- If we have a segments table
ALTER TABLE segments
ADD COLUMN words TEXT;  -- JSON array: [{word, start, end}, ...]
```

## API Changes

### GET /api/transcribe/:id

Current response includes `segments` array.

Updated response with word timing:
```json
{
  "status": "complete",
  "result": {
    "segments": [
      {
        "id": 1,
        "start": 0.0,
        "end": 3.5,
        "text": "Hello world",
        "words": [
          {"word": "Hello", "start": 0.0, "end": 0.8},
          {"word": "world", "start": 1.2, "end": 1.8}
        ]
      }
    ]
  }
}
```

## UI/UX Considerations

1. **Setting to enable/disable**
   - Some users may find word highlighting distracting
   - Add toggle in Settings > Preferences

2. **Mobile performance**
   - More DOM elements = slower on mobile
   - Consider simpler highlighting on mobile

3. **Subtitle export**
   - Standard SRT/VTT don't support word-level timing
   - Could offer "karaoke SRT" variant with per-word cues

## Estimated Effort

| Phase | Task | Effort |
|-------|------|--------|
| 1.1 | Update whisper call | Small |
| 1.2 | Store word timing | Small |
| 1.3 | Update API | Small |
| 2.1 | Frontend word rendering | Medium |
| 2.2 | CSS styling | Small |
| 2.3 | Bionic + word integration | Medium |
| 3.x | Performance optimization | Medium |

**Total estimate**: Medium-sized feature (~2-3 days of work)

## Alternatives Considered

### Highlighted segment only (no word-level)
- Simpler but less engaging
- Already implemented with current active segment styling

### SSML/TTS-based timing
- Generate timing from text-to-speech output
- More complex, less accurate than Whisper word timestamps

### Client-side word timing estimation
- Estimate word timing from segment duration
- Inaccurate, poor UX

## Conclusion

Word-level highlighting is feasible and would enhance the language learning use case. The key requirement is enabling word timestamps in the whisper-server transcription call and storing the additional data.

**Recommended approach:**
1. Enable word timestamps in whisper-server call
2. Store word timing JSON alongside segment data
3. Render words with highlighting in frontend
4. Make it an opt-in setting for users who want it
