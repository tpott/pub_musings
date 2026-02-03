# Architecture Patterns

**Domain:** Voice-activated content viewer for kids
**Researched:** 2026-02-02
**Confidence:** MEDIUM (patterns verified from subtitler project + web research; some streaming patterns need validation)

## Recommended Architecture

```
+------------------+     WebSocket (audio)      +------------------+
|                  | -------------------------> |                  |
|   React/Next.js  |                            |   Go Backend     |
|     Frontend     | <------------------------- |                  |
|                  |     WebSocket (transcript) +--------+---------+
+------------------+                                     |
        |                                                |
        | HTTP (content requests)                        | HTTP (audio chunks)
        v                                                v
+------------------+                            +------------------+
|   Go Backend     |                            |  Whisper Server  |
|  (content API)   |                            |  (Mac Mini host) |
+------------------+                            +------------------+
        |                                                ^
        | Read/Decrypt                                   |
        v                                                |
+------------------+                            Transcription results
|   SQLite DB      |                            (via callback/polling)
|  + .age files    |
+------------------+
        |
        | LLM API calls
        v
+------------------+
|  Anthropic/OpenAI|
|   (intent detect)|
+------------------+
```

### Component Boundaries

| Component | Responsibility | Communicates With |
|-----------|---------------|-------------------|
| **React Frontend** | Audio capture, UI rendering, WebSocket client, content display | Go Backend (WebSocket + HTTP) |
| **Go Backend** | WebSocket server, audio forwarding, LLM orchestration, content API, encryption | Frontend, Whisper, LLM APIs, SQLite |
| **Whisper Server** | Speech-to-text transcription | Go Backend (HTTP) |
| **SQLite + .age files** | Content catalog, encrypted media storage | Go Backend only |
| **LLM Provider** | Intent detection from transcription stream | Go Backend only |

### Data Flow

**Voice-to-Content Loop (the core interaction):**

```
1. User speaks "show me a cat"
         |
         v
2. Browser captures audio via getUserMedia()
   - MediaRecorder with opus/webm codec
   - Chunks every 250-500ms
         |
         v
3. Frontend sends audio chunks via WebSocket (binary messages)
         |
         v
4. Go Backend receives chunks, forwards to Whisper Server
   - Accumulates audio in buffer
   - Sends to Whisper via HTTP POST when buffer threshold reached
   - OR: Maintains streaming connection if whisper-stream supports it
         |
         v
5. Whisper returns transcription (partial + final)
         |
         v
6. Go Backend feeds transcription to LLM
   - Maintains sliding window of recent transcription
   - Asks LLM: "Is this directed at peekaboo? What concept?"
   - Uses streaming LLM response for fast intent detection
         |
         v
7. LLM identifies intent: {directed: true, concept: "cat"}
         |
         v
8. Go Backend queries SQLite for "cat" content
         |
         v
9. Go Backend decrypts .age file on demand
         |
         v
10. Frontend receives:
    - Live transcription updates (WebSocket text messages)
    - Content response (HTTP or WebSocket message with media URL)
         |
         v
11. UI displays content (photo/video) and plays audio clip
```

**Estimated latency budget:**
- Audio capture to backend: ~50ms
- Whisper transcription: ~500-1500ms (depending on chunk size)
- LLM intent detection: ~200-500ms (with streaming)
- Content lookup + decryption: ~100ms
- **Total:** 850-2150ms (acceptable for "magical" feel)

## Component Details

### 1. Browser Audio Capture

**Technology:** Web Audio API + MediaRecorder

**Pattern:** Chunk-based streaming with VAD consideration

```typescript
// Simplified audio capture pattern
const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
const mediaRecorder = new MediaRecorder(stream, {
  mimeType: 'audio/webm;codecs=opus'
});

mediaRecorder.ondataavailable = (event) => {
  if (event.data.size > 0 && websocket.readyState === WebSocket.OPEN) {
    websocket.send(event.data); // Binary message
  }
};

mediaRecorder.start(250); // Chunk every 250ms
```

**Key considerations:**
- Chrome prefers webm/opus; may need transcoding on backend for Whisper (expects WAV/PCM)
- Consider client-side VAD to reduce unnecessary audio transmission
- Handle microphone permissions gracefully (especially for kid users)

**Confidence:** MEDIUM - Pattern well-established, but audio format conversion needs validation with specific Whisper setup.

### 2. WebSocket Architecture (Go Backend)

**Technology:** gorilla/websocket

**Pattern:** Single persistent connection per client, bidirectional messaging

```go
// Simplified WebSocket handler pattern
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        return
    }
    defer conn.Close()

    client := &Client{
        conn:          conn,
        audioBuffer:   &bytes.Buffer{},
        transcriptBuf: &TranscriptBuffer{},
    }

    // Read loop (audio in)
    go client.readPump()

    // Write loop (transcripts out)
    client.writePump()
}

func (c *Client) readPump() {
    for {
        messageType, data, err := c.conn.ReadMessage()
        if err != nil {
            break
        }

        if messageType == websocket.BinaryMessage {
            // Audio chunk received
            c.processAudioChunk(data)
        } else if messageType == websocket.TextMessage {
            // Control message (e.g., mic toggle)
            c.handleControlMessage(data)
        }
    }
}
```

**Message types:**
| Direction | Type | Content |
|-----------|------|---------|
| Client -> Server | Binary | Audio chunks (webm/opus) |
| Client -> Server | Text/JSON | Control messages (mic on/off, config) |
| Server -> Client | Text/JSON | Transcription updates |
| Server -> Client | Text/JSON | Content display commands |
| Server -> Client | Text/JSON | Error messages |

**Confidence:** HIGH - gorilla/websocket is battle-tested, pattern proven in subtitler project.

### 3. Whisper Integration

**Architecture decision:** Whisper runs as separate service on Mac Mini (proven pattern from subtitler).

**Two integration approaches:**

**Option A: HTTP POST per chunk (simpler)**
```
Backend accumulates 2-5 seconds of audio
     |
     v
Convert webm -> WAV (ffmpeg)
     |
     v
POST to whisper-server /inference
     |
     v
Receive JSON transcription
```

**Option B: Streaming via whisper-stream (lower latency)**
```
Backend maintains WebSocket to whisper-stream
     |
     v
Forward audio chunks directly
     |
     v
Receive partial + final transcriptions
```

**Recommendation:** Start with Option A for simplicity. The subtitler uses HTTP POST and it works well. Option B requires building/forking whisper-stream with webhook support (mentioned in plan.md as future work).

**Audio format conversion:**
- Browser sends: webm/opus
- Whisper expects: WAV (16-bit PCM, 16kHz)
- Backend must transcode using ffmpeg

**Confidence:** HIGH for Option A (proven in subtitler). MEDIUM for Option B (requires custom whisper-stream work).

### 4. LLM Intent Detection

**Architecture:** Backend orchestrates LLM calls, not frontend.

**Pattern:** Sliding window of transcription + streaming LLM response

```go
type IntentDetector struct {
    provider     LLMProvider  // Anthropic or OpenAI
    transcriptBuf *RingBuffer // Last N seconds of transcription
}

func (d *IntentDetector) DetectIntent(newText string) (*Intent, error) {
    d.transcriptBuf.Append(newText)

    prompt := fmt.Sprintf(`You are a voice-activated content viewer for kids called "peekaboo".

Recent speech transcription:
%s

Determine:
1. Is this speech directed at the app? (look for "show me", "peekaboo", direct requests)
2. If yes, what concept/thing is being requested?

Respond in JSON: {"directed": bool, "concept": string or null, "confidence": float}`,
        d.transcriptBuf.String())

    return d.provider.Complete(prompt)
}
```

**Dual provider support:**

```go
type LLMProvider interface {
    Complete(prompt string) (*Intent, error)
    Stream(prompt string, callback func(chunk string)) error
}

type AnthropicProvider struct { /* ... */ }
type OpenAIProvider struct { /* ... */ }
```

**When to call LLM:**
- Not on every transcription chunk (too expensive, too slow)
- After utterance boundaries (silence detection)
- OR on a debounced timer (e.g., 500ms after last word)
- Consider caching common queries ("show me a cat" -> cat)

**Confidence:** MEDIUM - Pattern is sound, but prompt engineering needs iteration. Streaming LLM for lower latency may need tuning.

### 5. Content Storage and Encryption

**Pattern:** Reuse subtitler's age encryption exactly.

```
data/
  age.key           # X25519 private key (0600 permissions)
  media/
    cat_001.jpg.age
    cat_002.mp4.age
    cat_meow.mp3.age
    dog_001.jpg.age
    ...
```

**SQLite schema:**

```sql
CREATE TABLE concepts (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,  -- "cat", "dog", "giraffe"
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE media (
    id INTEGER PRIMARY KEY,
    concept_id INTEGER NOT NULL REFERENCES concepts(id),
    type TEXT NOT NULL,          -- "photo", "video", "audio"
    file_path TEXT NOT NULL,     -- "data/media/cat_001.jpg.age"
    metadata TEXT,               -- JSON: duration, dimensions, etc.
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_media_concept ON media(concept_id);
```

**Decryption flow:**
```
Content request received
     |
     v
Query SQLite for concept -> media mapping
     |
     v
Decrypt .age file to temp location
     |
     v
Serve via HTTP (with cache headers)
     |
     v
Clean up temp file after response
```

**Confidence:** HIGH - Direct reuse of proven subtitler pattern.

### 6. Frontend State Management

**Pattern:** Custom hook for WebSocket + React Context for shared state

```typescript
// WebSocket connection hook
function useVoiceConnection() {
  const [isConnected, setIsConnected] = useState(false);
  const [isListening, setIsListening] = useState(false);
  const [transcript, setTranscript] = useState<TranscriptEntry[]>([]);
  const [currentContent, setCurrentContent] = useState<Content | null>(null);
  const wsRef = useRef<WebSocket | null>(null);

  // Connection management with reconnection logic
  // Audio capture integration
  // Message handling

  return {
    isConnected,
    isListening,
    transcript,
    currentContent,
    toggleMic,
  };
}

// Context for app-wide access
const VoiceContext = createContext<VoiceState | null>(null);

function VoiceProvider({ children }) {
  const voice = useVoiceConnection();
  return (
    <VoiceContext.Provider value={voice}>
      {children}
    </VoiceContext.Provider>
  );
}
```

**State structure:**

```typescript
interface VoiceState {
  // Connection
  isConnected: boolean;
  connectionError: string | null;

  // Microphone
  isListening: boolean;
  micPermissionStatus: 'granted' | 'denied' | 'prompt';

  // Transcription (scrolling dialog box)
  transcript: TranscriptEntry[];  // Rolling window of recent speech

  // Content display
  currentContent: Content | null;
  isLoadingContent: boolean;
}

interface TranscriptEntry {
  id: string;
  text: string;
  timestamp: number;
  isFinal: boolean;
}

interface Content {
  type: 'photo' | 'video';
  url: string;
  audioUrl?: string;  // Optional companion audio
  concept: string;
}
```

**Confidence:** MEDIUM - Patterns well-established, but specific implementation needs validation.

## Patterns to Follow

### Pattern 1: Message-Based WebSocket Protocol

**What:** Define a clear JSON protocol for WebSocket text messages.

**Why:** Enables clean separation of concerns and easier debugging.

```typescript
// Client -> Server messages
type ClientMessage =
  | { type: 'control'; action: 'mic_on' | 'mic_off' }
  | { type: 'config'; settings: UserSettings };

// Server -> Client messages
type ServerMessage =
  | { type: 'transcript'; text: string; isFinal: boolean }
  | { type: 'content'; content: Content }
  | { type: 'error'; message: string; code: string }
  | { type: 'status'; connected: boolean; listening: boolean };
```

### Pattern 2: Graceful Degradation

**What:** Handle component failures without crashing the whole system.

**Why:** Kids will encounter errors; the app should recover gracefully.

```go
// If Whisper is down, show friendly message
// If LLM is slow, use cached intent mappings
// If content missing, show generic "I don't know that one"
```

### Pattern 3: Debounced LLM Calls

**What:** Don't call LLM on every transcription update.

**Why:** Cost, latency, and rate limits.

```go
type Debouncer struct {
    timer    *time.Timer
    delay    time.Duration
    callback func()
}

// Call LLM 500ms after last transcription update
func (d *Debouncer) Trigger() {
    if d.timer != nil {
        d.timer.Stop()
    }
    d.timer = time.AfterFunc(d.delay, d.callback)
}
```

## Anti-Patterns to Avoid

### Anti-Pattern 1: Frontend LLM Calls

**What:** Calling LLM APIs directly from the browser.

**Why bad:**
- Exposes API keys
- No rate limiting
- Can't use server-side caching
- Harder to switch providers

**Instead:** All LLM calls go through Go backend.

### Anti-Pattern 2: Synchronous Audio Processing

**What:** Blocking the main thread while processing audio.

**Why bad:**
- Dropped audio frames
- UI freezes
- Poor user experience

**Instead:** Use Web Workers for audio processing, async WebSocket sends.

### Anti-Pattern 3: Decrypting All Content at Startup

**What:** Pre-decrypting media files for faster serving.

**Why bad:**
- Security risk (unencrypted files on disk)
- Memory pressure
- Slow startup

**Instead:** Decrypt on-demand, serve immediately, clean up temp files.

### Anti-Pattern 4: Polling for Transcription

**What:** Using HTTP polling instead of WebSocket for transcription updates.

**Why bad:**
- High latency
- Server load
- Poor real-time experience

**Instead:** Persistent WebSocket connection for streaming updates.

## Build Order (Dependencies)

Based on component dependencies, the recommended build order is:

### Phase 1: Foundation (no external dependencies)
1. **SQLite schema + Go models** - Content catalog foundation
2. **Age encryption module** - Copy from subtitler, verify with tests
3. **Basic Go HTTP server** - Health checks, static file serving

### Phase 2: Whisper Integration (depends on Phase 1)
4. **Audio format conversion** - webm -> WAV using ffmpeg
5. **Whisper HTTP client** - POST audio, receive transcription
6. **Test harness** - Upload audio file, verify transcription

### Phase 3: WebSocket Pipeline (depends on Phase 2)
7. **Go WebSocket server** - gorilla/websocket setup
8. **React audio capture** - getUserMedia, MediaRecorder
9. **React WebSocket client** - Connect, send audio, receive transcripts
10. **End-to-end test** - Speak, see transcription appear

### Phase 4: LLM Intent Detection (depends on Phase 3)
11. **LLM provider abstraction** - Interface for Anthropic + OpenAI
12. **Intent detection logic** - Prompt engineering, debouncing
13. **Content lookup** - SQLite query by concept
14. **Content serving** - Decrypt + HTTP response

### Phase 5: UI Polish (depends on Phase 4)
15. **Transcription display** - Scrolling dialog box
16. **Content display area** - Photo/video/audio player
17. **Mic toggle UI** - Red indicator when listening
18. **Error states** - Friendly messages for kids

### Phase 6: Admin Tools (parallel to Phase 5)
19. **Admin API endpoints** - CRUD for concepts and media
20. **Python REPL helpers** - Local review, upload scripts

## Scalability Considerations

| Concern | At 1-10 users | At 100 users | At 1000+ users |
|---------|---------------|--------------|----------------|
| **WebSocket connections** | Single Go process | Still fine | Need connection pooling |
| **Whisper processing** | Single Mac Mini | Queue with backpressure | Multiple Whisper instances |
| **LLM rate limits** | Well within limits | Monitor usage | May need caching layer |
| **Content storage** | Local filesystem | Still fine | Consider CDN for media |
| **SQLite** | Perfect | Still fine | Still probably fine |

**Note:** Peekaboo is a personal/family app, not a public service. Scaling beyond 10 concurrent users is unlikely to be needed.

## Sources

### Verified (HIGH confidence)
- Subtitler encryption spec: `/Users/trevor/Github/pub_musings/subtitler/specs/encryption.md`
- Subtitler backend spec: `/Users/trevor/Github/pub_musings/subtitler/specs/backend.md`
- Subtitler STT providers: `/Users/trevor/Github/pub_musings/subtitler/specs/stt-providers.md`
- gorilla/websocket: https://github.com/gorilla/websocket

### WebSearch verified (MEDIUM confidence)
- VoiceStreamAI architecture: https://github.com/alesaccoia/VoiceStreamAI
- React WebSocket patterns: https://oneuptime.com/blog/post/2026-01-15-websockets-react-real-time-applications/view
- Whisper streaming approaches: https://github.com/ufal/whisper_streaming
- Real-time transcription with Deepgram: https://deepgram.com/learn/build-a-real-time-transcription-app-with-react-and-deepgram

### WebSearch only (LOW confidence - needs validation)
- Browser audio format handling may vary by browser version
- Exact Whisper latency depends on model and hardware configuration
- LLM streaming response latency varies by provider and prompt length
