# Multi-Layered Speech Detection Specification

## Overview

Add word-level timestamps to subtitle segments so users can toggle between sentence-level and word-level subtitle display during video playback. Editing at the word level propagates changes to the sentence layer.

## Background

### What Whisper already provides

The backend requests `response_format=verbose_json` from whisper-server (`helpers.go`). The whisper.cpp server returns a `words[]` array inside each segment with per-word timestamps:

```json
{
  "segments": [
    {
      "id": 0,
      "start": 0.0,
      "end": 4.5,
      "text": "Hello world, this is a test.",
      "words": [
        { "word": "Hello", "start": 0.0, "end": 0.4, "probability": 0.95 },
        { "word": "world,", "start": 0.5, "end": 0.9, "probability": 0.92 },
        { "word": "this", "start": 1.1, "end": 1.3, "probability": 0.98 },
        { "word": "is", "start": 1.3, "end": 1.5, "probability": 0.99 },
        { "word": "a", "start": 1.5, "end": 1.6, "probability": 0.97 },
        { "word": "test.", "start": 1.7, "end": 2.1, "probability": 0.94 }
      ]
    }
  ]
}
```

**The backend currently discards the `words[]` array during parsing.** The `WhisperSegment` struct only captures `id`, `start`, `end`, `text`.

### Whisper word timestamp quality

Whisper's built-in word timestamps from DTW alignment are approximate. For most subtitle use cases, they are good enough. For precision needs, tools like WhisperX use phoneme-based forced alignment (wav2vec 2.0), but this would require an additional dependency.

**Decision: Use Whisper's built-in word timestamps.** They are already available from whisper-server with zero additional dependencies. If users report poor word timing, WhisperX integration can be evaluated later as a separate task.

## Layers

| Layer | Granularity | Source | Primary use |
|-------|-------------|--------|-------------|
| **Sentence** | Multi-word phrases | Whisper `segments[]` | Default subtitle display, SRT/VTT export |
| **Word** | Individual words | Whisper `segments[].words[]` | Fine-grained timing review, karaoke-style display |

The sentence layer is the **default and primary** layer. The word layer is opt-in for users who need fine-grained timing control.

## Data Model Changes

### Backend: Extend Segment struct

```go
// db/db_types.go

type Word struct {
    Text        string  `json:"text"`
    Start       float64 `json:"start"`
    End         float64 `json:"end"`
    Probability float64 `json:"probability,omitempty"`
}

type Segment struct {
    ID    int     `json:"id"`
    Start float64 `json:"start"`
    End   float64 `json:"end"`
    Text  string  `json:"text"`
    Words []Word  `json:"words,omitempty"` // NEW
}
```

### No database schema changes

The `transcriptions.segments_json` column stores JSON text. Adding `words` to the Segment struct just means the JSON payload includes the nested word array. Existing transcriptions without word data will deserialize with `Words: nil` (the `omitempty` tag handles both directions).

### Backend: Parse word data from Whisper response

In `helpers.go`, update the whisper response parsing struct:

```go
// In transcribeAudioServer function
var whisperResp struct {
    Text     string `json:"text"`
    Segments []struct {
        ID    int     `json:"id"`
        Start float64 `json:"start"`
        End   float64 `json:"end"`
        Text  string  `json:"text"`
        Words []struct {
            Word        string  `json:"word"`
            Start       float64 `json:"start"`
            End         float64 `json:"end"`
            Probability float64 `json:"probability"`
        } `json:"words"`
    } `json:"segments"`
}
```

Convert whisper words to `db.Word` structs when building `db.Segment` slices.

### Frontend: Extend TranscriptionSegment type

```typescript
// types/transcription.ts

export interface TranscriptionWord {
    text: string;
    start: number;
    end: number;
    probability?: number;
}

export interface TranscriptionSegment {
    id: number;
    start: number;
    end: number;
    text: string;
    words?: TranscriptionWord[];  // NEW
}
```

## Video Playback: Layer Toggle

### UI: Layer toggle button

Add a layer toggle button next to the existing speed controls on the upload page. Only visible when word data is available.

```
[Sentences] [Words]    <-- toggle between layers
```

**Sentence mode (default):**
- Current behavior — one subtitle per segment, displayed for the segment duration
- Segments list shows sentence-level entries

**Word mode:**
- Each word appears individually, timed to its word-level timestamps
- Current subtitle area shows individual words appearing and disappearing
- Segments list shows word-level entries (expandable from parent sentence)

### Playback behavior

In sentence mode, the `timeupdate` handler highlights the current segment (existing behavior).

In word mode, the `timeupdate` handler highlights the current **word** within the current segment:
- The current subtitle area shows the full sentence text with the current word highlighted (bold or underlined)
- This creates a karaoke-style effect where words light up as they are spoken

### Implementation: Karaoke display

```typescript
// In timeupdate handler, when word mode is active:
function updateWordHighlight(segment: TranscriptionSegment, currentTime: number) {
    if (!segment.words) return;

    const words = segment.words.map(w => {
        const isCurrent = currentTime >= w.start && currentTime <= w.end;
        const isPast = currentTime > w.end;
        const cls = isCurrent ? 'word-current' : isPast ? 'word-past' : 'word-future';
        return `<span class="${cls}">${escapeHtml(w.text)}</span>`;
    }).join(' ');

    currentSubtitle.innerHTML = words;
}
```

CSS:
```css
.word-current { font-weight: bold; color: var(--accent-color); }
.word-past { opacity: 0.7; }
.word-future { opacity: 0.5; }
```

## Editing: Cross-Layer Propagation

### Editing at the word level

When a user edits a word's text or timing in word mode:
1. The word's `text`/`start`/`end` is updated directly
2. The parent segment's `text` is reconstructed by joining all word texts with spaces
3. The parent segment's `start` is updated to `min(word.start for word in words)`
4. The parent segment's `end` is updated to `max(word.end for word in words)`

### Editing at the sentence level

When a user edits a segment's text in sentence mode:
- If the edit changes the text, the word-level data is **cleared** (`words = undefined`)
- Reason: word-level timestamps are no longer valid after free-form text editing
- The user can re-transcribe to regenerate word data

When a user edits a segment's timing (start/end) in sentence mode:
- Word data is preserved
- No word-level adjustment needed (words stay at their absolute timestamps)

### Deleting words

When all words in a segment are deleted:
- The segment itself is removed
- Surrounding segments are not merged (user can do this manually)

### Adding words

Not supported. Word timestamps require audio analysis. Users should re-transcribe the segment to get new word data.

## Export: SRT/VTT/JSON

### SRT and VTT export

Default: exports sentence-level segments (existing behavior, unchanged).

If the user has word mode active, exports use word-level timing:
- Each word becomes a separate subtitle entry
- This creates a dense subtitle file suitable for karaoke/learning applications

The export buttons should indicate which mode will be used:
- "Download SRT (sentences)" vs "Download SRT (words)"

### JSON export

Always includes all available data (both sentence and word layers):

```json
[
  {
    "id": 0,
    "start": 0.0,
    "end": 4.5,
    "text": "Hello world, this is a test.",
    "words": [
      { "text": "Hello", "start": 0.0, "end": 0.4 },
      { "text": "world,", "start": 0.5, "end": 0.9 }
    ]
  }
]
```

## Implementation Plan

### Phase 1: Backend — Capture word data (no UI changes)

1. Add `Word` struct to `db/db_types.go`
2. Add `Words []Word` field to `Segment` struct with `omitempty`
3. Update whisper response parsing in `helpers.go` to capture `words[]`
4. Backend tests: verify word data roundtrips through `segments_json` storage
5. API response includes word data automatically (no API changes needed)

**Risk: Increased `segments_json` size.** A 5-minute video with ~500 words would add ~20KB of word data. This is acceptable for the SQLite storage model.

### Phase 2: Frontend — Word data display

1. Update `TranscriptionSegment` type to include `words?`
2. Add layer toggle UI (sentence/word buttons)
3. Implement karaoke-style word highlighting in the `timeupdate` handler
4. Store layer preference in localStorage

### Phase 3: Frontend — Word-level editing

1. Word mode in edit view shows individual word entries
2. Word text/timing edits propagate to parent segment
3. Sentence text edits clear word data

### Phase 4: Export support

1. Export buttons reflect active layer
2. Word-mode SRT/VTT generates per-word entries

## Testing Strategy

### Backend tests

- Whisper response with `words[]` is parsed into `db.Segment.Words`
- `segments_json` serialization/deserialization preserves word data
- Existing transcriptions without word data continue to work (`Words: nil`)
- Word data survives segment save/load cycle

### Frontend unit tests

- `TranscriptionSegment` with words renders karaoke display correctly
- Layer toggle switches between sentence and word display
- Word edit propagates to parent segment text
- Sentence text edit clears word data
- Export in word mode generates per-word SRT entries

### E2E tests

- Upload page shows layer toggle when word data is available
- Layer toggle hides when word data is absent (old transcriptions)
- Switching layers updates the subtitle display
- Word highlighting tracks playback position

## Constraints

- **No new dependencies.** Word timestamps come from whisper-server's existing `verbose_json` format.
- **Backwards compatible.** Existing transcriptions without word data continue to work unchanged.
- **Sentence layer is default.** Word layer is opt-in — casual users never see it.
- **Word timestamps are approximate.** Whisper's DTW alignment is good but not phoneme-perfect. If higher precision is needed, WhisperX integration can be evaluated as a future task.
