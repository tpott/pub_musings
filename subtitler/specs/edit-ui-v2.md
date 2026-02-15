# Edit UI v2 Specification

## Overview

Improve the subtitle editing experience with:
1. **Timing adjustment buttons** (< > arrows) for fine-tuning segment start/end times
2. **Missing segment detection** with "+" buttons between segments to create and auto-transcribe gaps
3. **Progressive disclosure** for feedback buttons

## Current State

The edit UI has:
- Text editing via textarea
- Time editing via manual text input (`HH:MM:SS.mmm`)
- Segment deletion
- Undo/redo with history stack
- Add segment button (appends at end only)

**Pain points:**
- Adjusting timing requires typing exact timestamps — no quick nudge
- No way to detect or fill gaps between segments
- Feedback buttons (good/early/late/missing) always visible, taking up space

## Feature 1: Timing Adjustment Buttons

### Design

Each segment in edit mode gets arrow buttons for fine-tuning start and end times:

```
 [<] [>]  Start: 00:00:01.500    End: 00:00:03.200  [<] [>]
```

- **Left side** `[<]` `[>]`: Adjust **start time**
  - `[<]` moves start earlier (start -= step)
  - `[>]` moves start later (start += step)
- **Right side** `[<]` `[>]`: Adjust **end time**
  - `[<]` moves end earlier (end -= step)
  - `[>]` moves end later (end += step)

### Step size

Default step: **0.1 seconds** (100ms).

Holding **Shift** while clicking: **0.5 seconds** (500ms) for larger adjustments.

### Constraints

- Start time cannot go below 0
- Start time cannot exceed end time (min gap: 0.1s)
- End time cannot exceed video duration
- End time cannot be less than start time (min gap: 0.1s)

### Visual feedback

When an arrow button is clicked:
- The corresponding time input updates immediately
- The segment's `data-start` or `data-end` attribute updates
- The change is marked as unsaved
- A brief visual pulse on the time input (CSS animation) confirms the change

### HTML structure

```html
<div class="segment editing" data-index="0">
  <div class="segment-edit">
    <div class="time-inputs">
      <div class="time-adjust-group">
        <button class="time-adjust-btn" data-direction="earlier" data-type="start" data-index="0" aria-label="Move start earlier">&#9664;</button>
        <button class="time-adjust-btn" data-direction="later" data-type="start" data-index="0" aria-label="Move start later">&#9654;</button>
        <label>Start:</label>
        <input type="text" class="time-input-start" value="00:00:01.500" />
      </div>
      <div class="time-adjust-group">
        <label>End:</label>
        <input type="text" class="time-input-end" value="00:00:03.200" />
        <button class="time-adjust-btn" data-direction="earlier" data-type="end" data-index="0" aria-label="Move end earlier">&#9664;</button>
        <button class="time-adjust-btn" data-direction="later" data-type="end" data-index="0" aria-label="Move end later">&#9654;</button>
      </div>
    </div>
    <!-- ... textarea, delete button ... -->
  </div>
</div>
```

### Implementation

Add to `segment-editor.ts`:

```typescript
export function adjustSegmentTime(
    state: SegmentEditorState,
    els: SegmentEditorElements,
    index: number,
    type: 'start' | 'end',
    direction: 'earlier' | 'later',
    shiftKey: boolean
): void {
    const step = shiftKey ? 0.5 : 0.1;
    const delta = direction === 'earlier' ? -step : step;
    const segment = state.editedSegments[index];

    pushToHistory(state, els);

    if (type === 'start') {
        segment.start = Math.max(0, segment.start + delta);
        // Ensure start < end with minimum gap
        if (segment.start >= segment.end - 0.1) {
            segment.start = segment.end - 0.1;
        }
    } else {
        segment.end = segment.end + delta;
        // Ensure end > start with minimum gap
        if (segment.end <= segment.start + 0.1) {
            segment.end = segment.start + 0.1;
        }
    }

    markUnsaved(state, els);
    renderSegments(state, els);
}
```

## Feature 2: Missing Segment Detection

### Design

"+" buttons appear between each pair of segments and before the first / after the last segment. They represent gaps in the transcription.

```
    [+ Missing?]           <-- before first segment (if segment starts > 0.5s)
    ┌─────────────────┐
    │ Segment 1        │
    │ 00:00.5 - 03:02  │
    └─────────────────┘
    [+ Missing?]           <-- between segment 1 and 2 (if gap > 0.5s)
    ┌─────────────────┐
    │ Segment 2        │
    │ 05:00 - 08:30    │
    └─────────────────┘
    [+ Missing?]           <-- after last segment (if video duration - end > 0.5s)
```

### When to show "+" buttons

A "+" button appears when there's a gap of **> 0.5 seconds** between:
- Video start (0.0) and first segment's start time
- Previous segment's end time and next segment's start time
- Last segment's end time and video duration

### What clicking "+" does

1. **Creates a new empty segment** spanning the gap:
   - Start: previous segment's end (or 0.0 for first gap)
   - End: next segment's start (or video duration for last gap)
   - Text: empty string

2. **Inserts the segment** at the correct position in the array

3. **Kicks off async transcription** for the gap:
   - Calls `POST /api/transcribe/{id}/gap` with `start` and `end` parameters
   - Backend extracts the audio for that time range and transcribes it
   - Returns the transcribed text
   - Frontend updates the empty segment's text with the result
   - Shows a loading spinner on the segment while transcribing

### Backend: Gap transcription endpoint

**New endpoint:** `POST /api/transcribe/{id}/gap`

Request:
```json
{
    "start": 3.02,
    "end": 5.0
}
```

Response:
```json
{
    "text": "The transcribed text for this gap",
    "segments": [
        { "id": 0, "start": 3.1, "end": 4.8, "text": "The transcribed text for this gap" }
    ]
}
```

**Implementation approach:**
1. Decrypt the uploaded video file to a temp file
2. Extract the audio segment using ffmpeg: `ffmpeg -i video.mp4 -ss 3.02 -to 5.0 -vn -f wav pipe:1`
3. Send the extracted audio to whisper-server for transcription
4. Return the result
5. Clean up temp files

**Rate limiting:** 10 requests/minute per IP (same as transcribe endpoint).

### Gap detection function

```typescript
export interface GapInfo {
    start: number;    // gap start time
    end: number;      // gap end time
    afterIndex: number; // insert after this segment index (-1 for before first)
}

export function detectGaps(
    segments: TranscriptionSegment[],
    videoDuration: number,
    minGap: number = 0.5
): GapInfo[] {
    const gaps: GapInfo[] = [];

    if (segments.length === 0) return gaps;

    // Gap before first segment
    if (segments[0].start > minGap) {
        gaps.push({ start: 0, end: segments[0].start, afterIndex: -1 });
    }

    // Gaps between segments
    for (let i = 0; i < segments.length - 1; i++) {
        const gap = segments[i + 1].start - segments[i].end;
        if (gap > minGap) {
            gaps.push({
                start: segments[i].end,
                end: segments[i + 1].start,
                afterIndex: i,
            });
        }
    }

    // Gap after last segment
    const lastEnd = segments[segments.length - 1].end;
    if (videoDuration - lastEnd > minGap) {
        gaps.push({
            start: lastEnd,
            end: videoDuration,
            afterIndex: segments.length - 1,
        });
    }

    return gaps;
}
```

## Feature 3: Progressive Disclosure for Feedback Buttons

### Current state

Feedback buttons (good/early/late/missing) are always visible when a segment is active. They occupy vertical space between the current subtitle and the segment list.

### New design

Feedback buttons are collapsed behind a single "Rate" button:

```
[Current subtitle text here]
[Rate]    <-- collapsed state

[Current subtitle text here]
[👍] [⏪ Early] [⏩ Late] [❓ Missing]    <-- expanded state
```

### Behavior

- Clicking "Rate" expands the feedback buttons
- Selecting a feedback type collapses them back (with the selected feedback indicated)
- Clicking "Rate" again when already expanded collapses without changing feedback
- The "Rate" button shows a colored dot matching the current feedback type (if any)

### Implementation

```html
<div class="subtitle-feedback" id="subtitleFeedback" style="display: none;">
    <button class="feedback-toggle-btn" id="feedbackToggle">
        <span class="feedback-dot" id="feedbackDot"></span>
        Rate
    </button>
    <div class="feedback-options" id="feedbackOptions" style="display: none;">
        <button class="feedback-btn good" data-feedback="good">👍</button>
        <button class="feedback-btn early" data-feedback="early">⏪ Early</button>
        <button class="feedback-btn late" data-feedback="late">⏩ Late</button>
        <button class="feedback-btn missing" data-feedback="missing">❓ Missing</button>
    </div>
</div>
```

CSS for the colored dot:

```css
.feedback-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    display: inline-block;
    margin-right: 4px;
}
.feedback-dot.good { background-color: #22c55e; }
.feedback-dot.early { background-color: #f59e0b; }
.feedback-dot.late { background-color: #f97316; }
.feedback-dot.missing { background-color: #ef4444; }
```

## TDD Test Plan

### Timing adjustment tests (Red-Green-Refactor)

**Test file:** `segment-editor.test.ts` (extend existing)

1. `adjustSegmentTime moves start earlier by 0.1s`
2. `adjustSegmentTime moves start later by 0.1s`
3. `adjustSegmentTime moves end earlier by 0.1s`
4. `adjustSegmentTime moves end later by 0.1s`
5. `adjustSegmentTime uses 0.5s step when shift is held`
6. `adjustSegmentTime prevents start from going below 0`
7. `adjustSegmentTime prevents start from exceeding end - 0.1`
8. `adjustSegmentTime prevents end from going below start + 0.1`
9. `adjustSegmentTime pushes to undo history before changing`
10. `adjustSegmentTime marks state as unsaved`

### Gap detection tests (new file or extend segment-editor.test.ts)

11. `detectGaps finds gap before first segment`
12. `detectGaps finds gap between two segments`
13. `detectGaps finds gap after last segment`
14. `detectGaps ignores gaps smaller than threshold`
15. `detectGaps returns empty array when no gaps exist`
16. `detectGaps handles empty segments array`
17. `detectGaps returns correct afterIndex for insertion`

### Progressive disclosure tests

18. `feedback toggle shows/hides feedback options`
19. `selecting feedback collapses options`
20. `feedback dot shows correct color for current feedback type`
21. `feedback dot is hidden when no feedback set`

### Backend gap transcription tests

22. `POST /api/transcribe/{id}/gap validates start < end`
23. `POST /api/transcribe/{id}/gap validates start >= 0`
24. `POST /api/transcribe/{id}/gap returns 404 for missing video`
25. `POST /api/transcribe/{id}/gap returns transcribed segments`

## Implementation Order

1. **Phase 1: Timing adjustment buttons** (task 511)
   - Write tests first (TDD)
   - Implement `adjustSegmentTime` function
   - Update `renderSegments` HTML to include arrow buttons
   - Add CSS for arrow buttons
   - Wire up event delegation in `setupEditHandlers`

2. **Phase 2: Progressive disclosure** (can be done alongside Phase 1)
   - Update feedback button HTML
   - Add toggle logic
   - Add colored dot indicator

3. **Phase 3: Gap detection** (task 512, part 1)
   - Write `detectGaps` function with tests
   - Render "+" buttons in edit mode
   - Create empty segments on click

4. **Phase 4: Async gap transcription** (task 512, part 2)
   - Backend: add `POST /api/transcribe/{id}/gap` endpoint
   - Backend: ffmpeg audio extraction for time range
   - Frontend: loading spinner, async fetch, segment text update

## Constraints

- Timing adjustments work only in edit mode
- Gap detection requires video duration (available from `<video>` element)
- Async transcription requires whisper-server to be running
- Arrow buttons use event delegation (single listener on segments container)
- All timing changes go through undo/redo system
