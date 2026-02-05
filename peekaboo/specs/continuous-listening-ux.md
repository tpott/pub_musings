# Spec: Continuous Listening UX Improvements

**Created:** 2026-02-05
**Status:** Pending implementation

## Overview

This spec addresses two UX gaps in the current WebSocket audio streaming implementation:

1. **Mic stays on until explicitly turned off** - Currently the mic stops after displaying media results
2. **Transcript display** - Users cannot see what was transcribed, making debugging difficult

## Problem Statement

### Issue 1: Mic Stops After Media Display

The original architecture spec (`specs/architecture.md:16-20`) states:

> **Key UX requirement:** Steps 3-6 happen **while the microphone is still listening**.
> The user sees results without needing to stop recording first. This enables a
> continuous, conversational experience similar to Google Home.

Current behavior:
1. User clicks mic to start recording
2. Audio streams to backend via WebSocket
3. Backend auto-processes after 3-second threshold
4. Media displays
5. **Bug:** Recording stops - user must click mic again for next command

Expected behavior:
1. User clicks mic to start recording (mic turns ON)
2. Audio streams continuously
3. Backend processes when it detects speech completion (silence/threshold)
4. Media displays while mic **continues listening**
5. User can issue another command immediately
6. User clicks mic again only to turn mic OFF

### Issue 2: No Transcript Visibility

Users have no feedback about what the system heard. This causes confusion when:
- Wrong microphone is selected (recording silence)
- Speech wasn't recognized correctly
- Whisper misheard the command

Real-world example: User tested manually and nothing worked. Only after inspecting `/transcribe` responses did they realize the browser was using the wrong mic and recording silence.

## Proposed Solution

### Continuous Listening Mode

Modify the WebSocket flow to keep the MediaRecorder and stream active after receiving media results:

```
┌─────────────────────────────────────────────────────────────────┐
│                     Continuous Listening Flow                    │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  User clicks mic ──► Mic ON (recording starts)                  │
│         │                                                        │
│         ▼                                                        │
│  ┌──────────────┐                                               │
│  │ Audio chunks │──► Backend buffers ──► Process when ready     │
│  │   streaming  │         │                                      │
│  └──────────────┘         │                                      │
│         │                 ▼                                      │
│         │          ┌─────────────┐                               │
│         │          │ Transcript  │──► Display in UI              │
│         │          │   + Media   │                               │
│         │          └─────────────┘                               │
│         │                 │                                      │
│         │                 ▼                                      │
│         │          Clear buffer, ready for next utterance        │
│         │                 │                                      │
│         └────────────────►│ (mic still recording!)               │
│                           │                                      │
│  User clicks mic ──► Mic OFF (recording stops)                  │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

**Key changes:**

1. **Frontend (`peekaboo-flow.ts`):**
   - Don't stop MediaRecorder when receiving media results
   - Keep `this.stream` and `this.mediaRecorder` active
   - Only stop when user explicitly clicks mic button while in recording state
   - State transitions: `idle` → `recording` → stays `recording` even during `displaying`

2. **Backend (`api/websocket.go`):**
   - After sending media response, clear audio buffer
   - Stay ready to receive new audio chunks
   - Don't close WebSocket or reset session state

3. **State machine update:**
   - New compound state: `recording+displaying` (mic on, media shown)
   - Allow new processing cycles without stopping recording

### Transcript Display

Add a transcript display area below the photo/video rendering:

```
┌───────────────────────────────────────┐
│                                       │
│           [Photo/Video]               │
│                                       │
├───────────────────────────────────────┤
│  "show me a cat"                      │  ◄── Transcript area
└───────────────────────────────────────┘
            [Mic Button]                   ◄── Smaller button
```

**UI Requirements:**
- Transcript appears below media display
- Transcripts persist and scroll (showing history of commands)
- Styled subtly (smaller font, muted color) to not distract from media
- Accessible: `aria-live="polite"` for screen reader announcements
- Mic button continues pulsing while listening, even when media is displayed

**Frontend changes:**
1. Add `[data-testid="transcript-display"]` element to `MediaDisplay.astro`
2. Update `handleWsTranscript()` in `peekaboo-flow.ts` to append transcript to history
3. Transcript area should be scrollable

**Note for implementation:** The mic button can be made smaller since the main interaction area is now the media display. Exact sizing left to implementer's judgment.

## Testing Requirements

### Unit Tests

**Frontend (`peekaboo-flow.test.ts`):**
- Test: Receiving media result does NOT stop MediaRecorder
- Test: MediaRecorder only stops when user clicks mic during recording
- Test: Transcript is displayed when received from WebSocket
- Test: Multiple transcripts accumulate in scrollable history
- Test: Mic button keeps pulsing animation while media is displayed

**Backend (`websocket_test.go`):**
- Test: After sending media response, handler can receive new audio chunks
- Test: Multiple utterances in single WebSocket session are processed sequentially

### E2E Tests

**Playwright (`peekaboo.spec.ts`):**
- Test: "continuous listening - multiple commands without stopping mic"
  - Click mic to start
  - First command processed, media displayed
  - Mic still recording (verify via aria-pressed or visual indicator)
  - Second command processed, new media displayed
  - Click mic to stop

- Test: "transcript display shows recognized speech"
  - Mock WebSocket to send transcript
  - Verify transcript text visible in DOM
  - Verify transcript has correct `data-testid`

## Implementation Order

1. **Task 98:** Update `specs/architecture.md` to reflect continuous listening as implemented behavior
2. **Task 99:** Fix backend WebSocket handler to not reset session after media response
3. **Task 100:** Update frontend `PeekabooFlow` to keep recording after media display
4. **Task 101:** Add transcript display UI component and wiring
5. **Task 102:** Add E2E tests for continuous listening and transcript display
6. **Task 103:** Update docs to reflect new UX behavior

## Sources

- Original architecture spec: `specs/architecture.md`
- WebSocket implementation: `specs/websocket-audio.md`
- User feedback: 2026-02-05 conversation identifying gaps
