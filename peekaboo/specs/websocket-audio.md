# WebSocket Audio Streaming Spec

## Overview

This spec documents the WebSocket-based audio streaming implementation for Peekaboo.
The goal is to replace the current "record-then-send" pattern with continuous audio
streaming, enabling the "mic stays active while results display" UX described in
specs/architecture.md.

## Library Selection

### Go WebSocket Library

**Selected:** `github.com/coder/websocket`

**Alternatives evaluated:**

| Library | Maintenance | Last Release | Stars | Notes |
|---------|-------------|--------------|-------|-------|
| gorilla/websocket | Archived (Dec 2022) | v1.5.0 (2021) | ~22K | Battle-tested but abandoned |
| coder/websocket | Active (Coder maintains) | Sep 2025 | ~8K | Fork of nhooyr/websocket |
| gobwas/ws | Active | 2024 | ~5K | Zero-alloc, complex API |

**Rationale:**

1. **gorilla/websocket** is archived and no longer maintained. While stable, using an
   unmaintained library for a core feature is risky for long-term maintenance.

2. **coder/websocket** (formerly nhooyr/websocket) is actively maintained by Coder,
   has context.Context support throughout the API, compiles to WASM, and has proven
   quality (used by Traefik, Vault, Cloudflare). The API is idiomatic Go.

3. **gobwas/ws** has excellent performance but a complex, low-level API that adds
   unnecessary complexity for our use case.

**Decision:** Use `coder/websocket` for its active maintenance, idiomatic API, and
context support which aligns with our existing patterns.

### Browser WebSocket API

The browser's native `WebSocket` API is used directly. No library needed.

---

## Message Protocol

### Connection

```
GET /ws/audio → WebSocket upgrade
```

### Message Types

Messages are JSON for control/text and binary for audio.

#### Client → Server

1. **Audio chunk (binary)**
   - Raw audio bytes (webm/opus from MediaRecorder)
   - Sent every ~500ms while recording
   - No JSON wrapper - pure binary for efficiency

2. **Control messages (JSON)**
   ```json
   {"type": "start_recording"}
   {"type": "stop_recording"}
   {"type": "ping"}
   ```

#### Server → Client

1. **Transcript**
   ```json
   {"type": "transcript", "text": "show me a cat"}
   ```

2. **Media result**
   ```json
   {
     "type": "media",
     "subject": "cat",
     "photo_url": "/data/media/cat/set1/photo.jpg",
     "audio_url": "/data/media/cat/set1/audio.mp3"
   }
   ```

3. **Error**
   ```json
   {"type": "error", "message": "transcription failed"}
   ```

4. **Acknowledgment**
   ```json
   {"type": "pong"}
   ```

---

## Backend Implementation

### Audio Buffer Strategy

The backend buffers incoming audio chunks until one of these conditions:

1. **Silence detection** - Volume drops below threshold for N ms
   - Requires audio analysis (future enhancement)

2. **Time threshold** - 3 seconds of audio accumulated
   - Simple initial implementation

3. **Explicit stop** - Client sends `stop_recording` message
   - Fallback for when user releases button

### Processing Pipeline

```
Client audio chunk → WebSocket handler
                          ↓
                   Audio buffer (per-connection)
                          ↓
             [threshold reached or stop_recording]
                          ↓
                Forward to whisper-server (POST /inference)
                          ↓
             Send transcript to client via WebSocket
                          ↓
                Extract intent via LLM
                          ↓
             Lookup media from database
                          ↓
             Send media result to client via WebSocket
                          ↓
             Clear buffer, ready for next utterance
```

### Connection Management

- Each connection has its own audio buffer
- Connection cleanup on close/error
- Idle timeout: 5 minutes
- Max message size: 5MB (same as POST /api/transcribe)

### Session Lifecycle (Continuous Listening)

The WebSocket session supports continuous listening - the mic can stay active across multiple commands:

```
┌─────────────────────────────────────────────────────────────────┐
│                     Continuous Listening Session                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  1. User clicks mic → start_recording sent                      │
│                                                                  │
│  2. Audio chunks stream to server                                │
│                                                                  │
│  3. After 3s threshold (or stop_recording):                     │
│     - Server transcribes audio                                   │
│     - Sends transcript message                                   │
│     - Extracts intent via LLM                                   │
│     - Sends media message                                        │
│     - Clears audio buffer                                        │
│     - Stays ready for more audio (does NOT close session)       │
│                                                                  │
│  4. Client continues sending audio chunks (mic still active)    │
│                                                                  │
│  5. Repeat step 3 for each utterance                            │
│                                                                  │
│  6. User clicks mic again → stop_recording sent                 │
│     - Final audio processed                                      │
│     - Mic turned off on frontend                                 │
│                                                                  │
│  Note: The WebSocket connection remains open throughout.         │
│  Multiple commands can be processed without reconnecting.        │
└─────────────────────────────────────────────────────────────────┘
```

**Key implementation details:**

1. **Backend (`api/websocket.go`):**
   - After processing audio, clears buffer but keeps `isRecording = true`
   - Ready to receive new audio chunks immediately
   - Only sets `isRecording = false` on explicit `stop_recording` message

2. **Frontend (`peekaboo-flow.ts`):**
   - MediaRecorder keeps running after receiving media results
   - State remains `recording` even while displaying media
   - Mic button shows pulsing animation throughout
   - User must click mic again to stop recording

---

## Frontend Implementation

### WebSocket Client

```typescript
// frontend/src/lib/websocket-audio.ts

class AudioWebSocket {
  private ws: WebSocket | null = null;
  private reconnectAttempts = 0;

  connect(): void;
  disconnect(): void;

  startRecording(): void;
  stopRecording(): void;
  sendAudioChunk(chunk: Blob): void;

  onTranscript: (text: string) => void;
  onMedia: (media: MediaResult) => void;
  onError: (error: string) => void;
}
```

### Integration with MediaRecorder

```typescript
// Modify existing audio-recorder.ts

const recorder = new MediaRecorder(stream, {
  mimeType: 'audio/webm;codecs=opus',
  audioBitsPerSecond: 16000
});

recorder.ondataavailable = (event) => {
  if (event.data.size > 0) {
    audioWebSocket.sendAudioChunk(event.data);
  }
};

// Request data every 500ms
recorder.start(500);
```

### Integration with PeekabooFlow

The existing PeekabooFlow class needs modification:

1. Replace `POST /api/transcribe` with WebSocket chunk streaming
2. Keep mic recording while processing results
3. Handle multiple results from single recording session
4. Graceful degradation if WebSocket unavailable (fall back to POST)

---

## Testing

### Unit Tests (Go)

- WebSocket upgrade handler
- Message parsing (binary vs JSON)
- Audio buffer accumulation
- Threshold detection
- Connection cleanup on close

### Unit Tests (TypeScript)

- WebSocket connection management
- Reconnection logic
- Audio chunk sending
- Message handling

### E2E Tests (Playwright)

Mock WebSocket responses to test:
- Audio streaming triggers transcript
- Transcript triggers media display
- Continuous recording (multiple results)
- Connection recovery after disconnect

---

## Implementation Order

1. **Task 66** (this spec): Research and document - DONE
2. **Task 67**: Backend WebSocket endpoint
3. **Task 68**: Frontend WebSocket client
4. **Task 69**: Integration and E2E tests

---

## Sources

- [coder/websocket GitHub](https://github.com/coder/websocket)
- [WebSocket in 2025 - Go Forum](https://forum.golangbridge.org/t/websocket-in-2025/38671)
- [Gorilla Toolkit archived](https://forum.golangbridge.org/t/gorilla-toolkit-is-in-archive-mode/29946)
- [Coder blog: A New Home for nhooyr/websocket](https://coder.com/blog/websocket)
