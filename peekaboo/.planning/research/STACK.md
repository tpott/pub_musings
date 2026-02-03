# Technology Stack

**Project:** Peekaboo - Voice-activated content viewer for kids
**Researched:** 2026-02-02
**Overall Confidence:** HIGH

## Recommended Stack

### Backend (Go)

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| Go | 1.24.x | Runtime | Matches subtitler (1.24.12), enables latest SDK requirements (Go 1.22+) | HIGH |
| github.com/coder/websocket | v1.8.14 | WebSocket server | Idiomatic Go, full context.Context support, zero dependencies, actively maintained (Sept 2025). Better than gorilla/websocket for new projects. | HIGH |
| filippo.io/age | v1.3.1 | Media encryption | Proven in subtitler, X25519 + ChaCha20-Poly1305, actively maintained (Dec 2025) | HIGH |
| github.com/mattn/go-sqlite3 | v1.14.33 | Database driver | Already used in subtitler, better write performance than modernc. CGO acceptable given deployment model. | HIGH |
| github.com/anthropics/anthropic-sdk-go | v1.20.0 | Anthropic LLM client | Official SDK, streaming support via NewStreaming(), requires Go 1.22+ | HIGH |
| github.com/openai/openai-go | v3.17.0 | OpenAI LLM client | Official SDK, streaming via ssestream.Stream, requires Go 1.22+ | HIGH |

### Frontend (React/Next.js)

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| Next.js | 15.5.x or 16.x | Framework | Stable, App Router supports WebSocket via custom server or instrumentation.ts. Note: subtitler uses Astro but project spec calls for React. | HIGH |
| React | 19.x | UI library | Bundled with Next.js 15+, stable | HIGH |
| TypeScript | 5.x | Type safety | Standard for production React apps | HIGH |
| Zod | 3.x | Validation | Already used in subtitler frontend, good for API type safety | HIGH |

### Speech-to-Text (Whisper Service)

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| whisper.cpp | v1.8.1 | STT engine | C++ port, runs on CPU/GPU, includes server mode at /inference endpoint | HIGH |
| WhisperLive (alternative) | latest | Streaming STT | If whisper.cpp server insufficient, provides WebSocket streaming with faster_whisper backend | MEDIUM |

### Testing

| Technology | Version | Purpose | Why | Confidence |
|------------|---------|---------|-----|------------|
| Vitest | 3.x | Unit tests (frontend) | Used in subtitler, fast, Vite-native | HIGH |
| Playwright | 1.57.x | E2E tests | Used in subtitler, cross-browser | HIGH |
| Go testing | stdlib | Unit tests (backend) | Standard Go testing patterns from subtitler | HIGH |

## Detailed Rationale

### WebSocket: coder/websocket over gorilla/websocket

**Decision:** Use `github.com/coder/websocket` (v1.8.14)

**Why:**
1. **Context support** - Full context.Context integration for cancellation/timeouts
2. **Actively maintained** - Coder took over stewardship in 2024, releases through Sept 2025
3. **Zero dependencies** - Simpler dependency tree
4. **Idiomatic Go** - Cleaner API than gorilla's lower-level approach
5. **WebAssembly support** - Client side compiles to WASM (not needed but nice)

**Why not gorilla/websocket:**
- Less active development
- No context support in core API
- Uses unsafe operations internally
- API is more verbose

**Source:** [coder/websocket GitHub](https://github.com/coder/websocket), [Go WebSocket comparison 2025](https://amf-co.com/which-golang-websocket-library-should-you-use-in-2025/)

### LLM SDKs: Official Over Community

**Decision:** Use official Anthropic and OpenAI Go SDKs

**Anthropic:** `github.com/anthropics/anthropic-sdk-go` v1.20.0
- Official SDK from Anthropic
- Streaming via `client.Messages.NewStreaming()`
- MIT licensed
- Latest model support (Claude Sonnet 4.5)

**OpenAI:** `github.com/openai/openai-go` v3.17.0
- Official SDK from OpenAI
- Streaming via `ssestream.Stream[T]`
- Chat Completions API for text generation

**Why not community SDKs:**
- `sashabaranov/go-openai` is popular but unofficial
- `liushuangls/go-anthropic` is good but official SDK now exists
- Official SDKs get priority for new features and fixes

**Source:** [anthropic-sdk-go GitHub](https://github.com/anthropics/anthropic-sdk-go), [openai-go GitHub](https://github.com/openai/openai-go)

### SQLite Driver: mattn/go-sqlite3

**Decision:** Keep `github.com/mattn/go-sqlite3` (v1.14.33)

**Why:**
1. **Consistency** - Already used in subtitler
2. **Write performance** - Benchmarks show 1531ms insert vs 5288ms for modernc
3. **Maturity** - Battle-tested, well-documented
4. **Acceptable CGO** - Deployment model (same host as subtitler) already handles CGO

**Alternative considered:** `modernc.org/sqlite` (CGO-free)
- Would choose if cross-compilation were needed
- Query performance is comparable but write performance is notably slower

**Source:** [go-sqlite-bench (Aug 2025)](https://github.com/cvilsmeier/go-sqlite-bench)

### Whisper Service Architecture

**Decision:** whisper.cpp server mode with HTTP API

**Architecture:**
1. Frontend captures audio via Web Audio API / MediaRecorder
2. Frontend streams audio chunks to Go backend via WebSocket
3. Go backend accumulates audio and sends to whisper.cpp server via HTTP POST to `/inference`
4. Transcription results flow back through WebSocket to frontend

**Why whisper.cpp server over alternatives:**
1. **Proven pattern** - Subtitler already uses whisper as separate service
2. **Simple HTTP API** - `/inference` endpoint accepts multipart audio
3. **VAD support** - Voice Activity Detection reduces processing overhead
4. **Model flexibility** - Can use base.en for speed or large-v3 for accuracy

**Alternative:** WhisperLive (Collabora)
- Use if true streaming (word-by-word) is needed
- Adds WebSocket complexity between Go and Whisper
- Supports faster_whisper, TensorRT, OpenVINO backends

**Source:** [whisper.cpp server README](https://github.com/ggml-org/whisper.cpp/blob/master/examples/server/README.md), [WhisperLive GitHub](https://github.com/collabora/WhisperLive)

### Frontend Framework: Next.js vs Astro

**Decision:** Next.js 15.5+ (as specified in PROJECT.md)

**Why not continue with Astro (subtitler's choice):**
1. **Project spec requires React** - Astro is MPA-first, React is more natural for real-time apps
2. **WebSocket handling** - Next.js has patterns for WebSocket via custom server or instrumentation.ts
3. **Real-time state** - React's state model suits continuous transcription display
4. **App Router** - Supports streaming, Server Components, and modern patterns

**WebSocket in Next.js 15:**
- Use `instrumentation.ts` (since Next.js 14) to bootstrap WebSocket server
- Or run custom server with ws/socket.io
- Or connect directly to Go backend's WebSocket (recommended)

**Recommendation:** Frontend connects directly to Go backend WebSocket, not through Next.js API routes.

**Source:** [Next.js WebSocket discussion](https://github.com/vercel/next.js/discussions/14950), [Streaming in Next.js 15](https://hackernoon.com/streaming-in-nextjs-15-websockets-vs-server-sent-events)

### Browser Audio Capture

**Decision:** Web Audio API + MediaRecorder

**Pattern:**
```typescript
// Access microphone
const stream = await navigator.mediaDevices.getUserMedia({ audio: true });

// Create MediaRecorder for chunks
const mediaRecorder = new MediaRecorder(stream, { mimeType: 'audio/webm' });

// For analysis (optional visualizer)
const audioContext = new AudioContext();
const source = audioContext.createMediaStreamSource(stream);
const analyser = audioContext.createAnalyser();
source.connect(analyser);
```

**Why NOT Web Speech API:**
- Chrome sends audio to Google servers (privacy concern for kids app)
- Limited browser support (Chrome/Chromium only)
- Can't use custom Whisper model

**Why MediaRecorder + custom backend:**
- Full control over audio processing
- Uses our own Whisper service
- Works offline (if whisper runs locally)
- Privacy: audio stays on our infrastructure

**Source:** [MDN MediaRecorder](https://developer.mozilla.org/en-US/docs/Web/API/MediaRecorder), [Browser Microphone Access (Speechmatics)](https://blog.speechmatics.com/browser-microphone-access)

## Alternatives Considered

| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| WebSocket | coder/websocket | gorilla/websocket | Less idiomatic, no context support, less active |
| SQLite | mattn/go-sqlite3 | modernc.org/sqlite | Slower writes, CGO acceptable in our setup |
| Anthropic SDK | official SDK | liushuangls/go-anthropic | Official now exists, better support |
| OpenAI SDK | official SDK | sashabaranov/go-openai | Official now exists, better support |
| Frontend | Next.js | Astro | Project spec requires React, real-time needs |
| STT | whisper.cpp server | Web Speech API | Privacy (sends to Google), limited browser support |
| STT streaming | whisper.cpp HTTP | WhisperLive WebSocket | HTTP simpler, streaming optional enhancement |

## What NOT to Use

| Technology | Why Avoid |
|------------|-----------|
| Web Speech API for STT | Sends audio to Google servers - privacy concern for kids app |
| gorilla/websocket | Superseded by coder/websocket for new projects |
| Community LLM SDKs | Official SDKs now available from both Anthropic and OpenAI |
| Socket.io (frontend) | Overkill - native WebSocket sufficient for this use case |
| Astro for this project | MPA-first framework, React needed for real-time updates |
| OpenAI Whisper API (cloud) | Privacy concern, latency, cost - self-hosted whisper.cpp preferred |

## Installation

### Backend (Go)

```bash
# In backend directory
go mod init github.com/tpott/peekaboo/backend

# Core dependencies
go get github.com/coder/websocket@v1.8.14
go get filippo.io/age@v1.3.1
go get github.com/mattn/go-sqlite3@v1.14.33

# LLM SDKs
go get github.com/anthropics/anthropic-sdk-go@v1.20.0
go get github.com/openai/openai-go/v3@v3.17.0
```

### Frontend (Next.js)

```bash
# Create Next.js app
npx create-next-app@latest frontend --typescript --tailwind --app

# Additional dependencies
npm install zod
npm install -D vitest @playwright/test
```

### Whisper Service

```bash
# Build whisper.cpp with server
git clone https://github.com/ggml-org/whisper.cpp
cd whisper.cpp
cmake -B build -DWHISPER_SDL2=OFF
cmake --build build -j --config Release

# Download model
./models/download-ggml-model.sh base.en

# Run server
./build/bin/whisper-server -m ./models/ggml-base.en.bin -t 4
```

## Version Pinning Rationale

| Dependency | Version | Pinning Reason |
|------------|---------|----------------|
| Go | 1.24.x | Match subtitler toolchain |
| coder/websocket | 1.8.14 | Latest stable (Sept 2025) |
| age | 1.3.1 | Latest stable (Dec 2025) |
| go-sqlite3 | 1.14.33 | Match subtitler |
| anthropic-sdk-go | 1.20.0 | Current release with Claude Sonnet 4.5 |
| openai-go | 3.17.0 | Current release |
| Next.js | 15.5.x | Stable, or 16.x if available |
| whisper.cpp | 1.8.1 | Latest stable with server mode |

## Sources

**HIGH Confidence (Official Documentation):**
- [coder/websocket GitHub](https://github.com/coder/websocket) - v1.8.14 confirmed
- [anthropics/anthropic-sdk-go](https://github.com/anthropics/anthropic-sdk-go) - v1.20.0, Go 1.22+
- [openai/openai-go](https://github.com/openai/openai-go) - v3.17.0, Go 1.22+
- [filippo.io/age releases](https://github.com/FiloSottile/age/releases) - v1.3.1 (Dec 2025)
- [whisper.cpp server](https://github.com/ggml-org/whisper.cpp/blob/master/examples/server/README.md) - v1.8.1
- [Next.js 15.5 blog](https://nextjs.org/blog/next-15-5) - Stable release

**MEDIUM Confidence (Verified with Multiple Sources):**
- [go-sqlite-bench benchmarks](https://github.com/cvilsmeier/go-sqlite-bench) - Aug 2025 results
- [WhisperLive GitHub](https://github.com/collabora/WhisperLive) - Streaming alternative

**LOW Confidence (Single Source/Needs Validation):**
- Next.js 16 status - mentioned but verify before adopting
