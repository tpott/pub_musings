# Audio Timing & Transcription Architecture Analysis

**Date:** 2026-02-05
**Status:** Implemented (Tasks 137-144)

---

## Implementation Summary

All 5 phases implemented in tasks 137-144:

- **Phase 1** (Tasks 137-138): EBML init segment caching fixes WebM container corruption. Full whisper verbose_json parsing with word-level timing and probabilities.
- **Phase 2** (Task 139): 12-byte framed audio protocol (magic 0xAB01, uint16 seq, float64 timestamp). Client timestamp mapping for wall-clock correlation.
- **Phase 3** (Tasks 140-141): LLM boundary detection with `tool_choice:any` and three tools (show_media, text_to_speech, wait_for_more). Audio buffer trimming at Cluster boundaries. Transcript accumulation across cycles.
- **Phase 4** (Task 142): TTS tool execution via Piper. Multi-tool sequential processing with per-call error handling. Frontend plays TTS before media display.
- **Phase 5** (Tasks 143-144): Whisper VAD integration (`vad=true` form field). E2E endurance test for continuous listening surviving multiple buffer cycles.

Key implementation files:
- `backend/api/webm_parser.go` — incremental EBML parser using `at-wat/ebml-go`
- `backend/api/websocket.go` — framed protocol, buffer management, LLM integration, TTS execution
- `frontend/src/lib/websocket-audio.ts` — framed audio sending with 12-byte headers
- `frontend/src/lib/peekaboo-flow.ts` — TTS audio playback, transcript display

The analysis below is preserved as historical context documenting the problems and proposed solutions.

---

## Current System: What's Wrong

### Sequence Diagram (Current)

```
Timeline (seconds)          0.0   0.5   1.0   1.5   2.0   2.5   3.0   3.5   4.0   4.5   5.0   5.5   6.0
                            │     │     │     │     │     │     │     │     │     │     │     │     │
User speaks:                │ "show me a ────────── cat" ──│     │silence│     │     │     │     │
                            │     │     │     │     │     │     │     │     │     │     │     │     │
Client Date.now():          ├─T0──┤     │     │     │     │     │     │     │     │     │     │     │
(never sent)                │     │     │     │     │     │     │     │     │     │     │     │     │
                            │     │     │     │     │     │     │     │     │     │     │     │     │
MediaRecorder chunks:       ├──C1─┤──C2─┤──C3─┤──C4─┤──C5─┤──C6─┤──C7─┤     │     │     │     │     │
(500ms each, raw binary)    │     │     │     │     │     │     │     │     │     │     │     │     │
                            │     │     │     │     │     │     │     │     │     │     │     │     │
WS sends binary:            ├─►B──┤─►B──┤─►B──┤─►B──┤─►B──┤─►B──┤─►B──┤     │     │     │     │     │
(no metadata, no seq#)      │     │     │     │     │     │     │     │     │     │     │     │     │
                            │     │     │     │     │     │     │     │     │     │     │     │     │
Backend buffer:             │▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓│GRAB! │     │     │     │     │     │
(3s threshold)              │     │     │     │     │     │     │ ┌────┤▓▓▓▓▓│     │ (buffer 2 starts)
                            │     │     │     │     │     │     │ │    │     │     │     │     │     │
POST to whisper:            │     │     │     │     │     │     │ ├───────────────►│     │     │     │
(120s timeout)              │     │     │     │     │     │     │ │    │     │     │     │     │     │
                            │     │     │     │     │     │     │ │    │     │     │     │     │     │
Whisper returns:            │     │     │     │     │     │     │ │    │     │  ◄──┤"show me a cat" │
(only .Text used)           │     │     │     │     │     │     │ │    │     │     │     │     │     │
                            │     │     │     │     │     │     │ │    │     │     │ LLM ──────►│    │
                            │     │     │     │     │     │     │ │    │     │     │     │     │media│
```

### Problem 1: Audio Container Corruption on Buffer Split

This is a **showstopper bug** in continuous listening mode.

WebM uses a container format where:
- **Chunk 1** from MediaRecorder contains the EBML header + Track info (initialization segment)
- **Chunks 2-N** contain only Cluster elements (media segments)

When the backend grabs buffer at the 3-second mark:
- **Buffer 1** = C1+C2+C3+C4+C5+C6 → Valid WebM (has EBML header from C1)
- **Buffer 2** = C7+C8+C9+... → **INVALID WebM** (missing EBML header!)

The second and all subsequent buffers sent to whisper are malformed audio files.
Whisper-server's `--convert` flag (ffmpeg) might salvage some, but this is fragile
and lossy.

### Problem 2: No Timestamps on Audio Chunks

Client sends raw binary with zero metadata:
```typescript
// Current: fire and forget
this.ws.send(data);  // raw ArrayBuffer, nothing else
```

There's no way to map whisper's relative timestamps (e.g., word at 1.3s within the
buffer) back to when the user actually said that word in wall-clock time.

### Problem 3: Fixed 3-Second Boundary Splits Speech Arbitrarily

The buffer threshold is a wall-clock timer on the server, not aligned to speech.
If the user says "show me a cat" from T=0.8 to T=2.3:

- At T=3.0 the threshold fires, buffer gets grabbed
- Works fine for this case, but what about: "show me a really big fluffy cat"?
  - Buffer 1 (0-3s): "show me a really big"
  - Buffer 2 (3-6s): "fluffy cat"
  - Each buffer transcribed independently → two separate LLM calls
  - LLM sees "show me a really big" → intent unclear, subject = "big"??
  - LLM sees "fluffy cat" → subject = "cat" → correct but we lost context

### Problem 4: Token Timestamps Are Already Available But Discarded

The whisper request already uses `response_format=verbose_json`. Looking at
whisper.cpp server source (server.cpp:1032-1097), verbose_json returns:

```json
{
  "text": " show me a cat",
  "segments": [{
    "id": 0,
    "start": 0.0,
    "end": 2.5,
    "text": " show me a cat",
    "tokens": [4010, 502, 257, 3857],
    "words": [
      {"word": " show",  "start": 0.00, "end": 0.52, "probability": 0.95, "t_dtw": -1},
      {"word": " me",    "start": 0.52, "end": 0.76, "probability": 0.98, "t_dtw": -1},
      {"word": " a",     "start": 0.76, "end": 0.92, "probability": 0.97, "t_dtw": -1},
      {"word": " cat",   "start": 0.92, "end": 1.30, "probability": 0.99, "t_dtw": -1}
    ],
    "temperature": 0.0,
    "avg_logprob": -0.23,
    "no_speech_prob": 0.01
  }]
}
```

The Go backend parses this but throws away everything except `.Text`:
```go
// transcribe.go:200 — only Text is returned!
return whisperResp.Text, nil
```

**We already have**: token IDs, per-token text, per-token start/end times,
per-token probability. All discarded.

### Problem 5: No Sentence Boundary Detection

The system blindly processes every 3 seconds. It doesn't know if the user:
- Is mid-word ("sh—" at the boundary)
- Is mid-sentence ("show me a—" waiting for the noun)
- Has finished speaking and is silent
- Has said two commands ("show me a cat show me a dog")

### Problem 6: Word Probabilities Are Discarded

Whisper produces per-word probabilities that indicate transcription confidence.
Low-probability words signal uncertain recognition — exactly the cases where
the LLM needs the most help fuzzy-matching intent (e.g., "kitty tat" where
"tat" has low probability likely means "cat"). These probabilities are
currently thrown away along with the rest of verbose_json.

### Problem 7: Each Buffer Is Independent (No Accumulation)

No running transcript. Each 3-second window is transcribed and LLM'd in isolation.
There's no mechanism to:
- Carry over context from previous buffers
- Re-evaluate when more audio arrives
- Merge partial transcripts into a complete utterance

---

## What Whisper.cpp Server Actually Supports

### Per-Request Form Parameters (multipart/form-data)

| Parameter | Default | Notes |
|-----------|---------|-------|
| `file` | required | Audio file |
| `response_format` | `json` | `json`, `verbose_json`, `text`, `srt`, `vtt` |
| `temperature` | `0.0` | Decoding temperature |
| `temperature_inc` | `0.2` | Temperature increment on fallback |
| `language` | `en` | Language code or `auto` |
| `split_on_word` | `false` | **Split on word boundaries, not token** |
| `word_thold` | `0.01` | Word timestamp probability threshold |
| `no_timestamps` | `false` | Disable timestamps |
| `prompt` | `""` | Initial prompt / context |
| `vad` | `false` | **Enable Voice Activity Detection** |
| `vad_threshold` | `0.50` | VAD speech/silence threshold |
| `vad_min_speech_duration_ms` | `250` | Min speech duration |
| `vad_min_silence_duration_ms` | `100` | Min silence to split |
| `suppress_nst` | `false` | Suppress non-speech tokens |

### Server Startup-Only Parameters

| Parameter | Notes |
|-----------|-------|
| `--dtw MODEL` | Token-level timestamps via Dynamic Time Warping (more accurate) |
| `--convert` | Use ffmpeg to convert audio (handles non-WAV input) |

### verbose_json Output Structure

```
response
├── task: "transcribe"
├── language: "en"
├── duration: 2.5
├── text: "show me a cat"          ← currently the ONLY field used
└── segments[]
    ├── id: 0
    ├── start: 0.0 (seconds)
    ├── end: 2.5
    ├── text: "show me a cat"
    ├── tokens: [4010, 502, ...]   ← whisper token IDs
    ├── words[]                    ← TOKEN-LEVEL timing (auto-enabled for verbose_json)
    │   ├── word: " show"
    │   ├── start: 0.00
    │   ├── end: 0.52
    │   ├── probability: 0.95
    │   └── t_dtw: -1              ← DTW timestamp (-1 if --dtw not used)
    ├── temperature: 0.0
    ├── avg_logprob: -0.23
    └── no_speech_prob: 0.01
```

---

## Proposed Architecture

### Design Principles

1. **Accumulate, don't window.** Keep a growing audio buffer. Send the whole thing
   (or a smart overlap) to whisper each time. Compare successive transcripts.
2. **Client timestamps every chunk.** Framed binary protocol with sequence numbers
   and wall-clock timestamps.
3. **Use all of whisper's output.** Per-word timing, probabilities, segments.
4. **LLM decides when to act.** Pass accumulated transcript + word probabilities
   to LLM with `tool_choice: any` — every call produces a tool response.
   LLM returns: `show_media` (at most once), `text_to_speech`, or `wait_for_more`.
5. **Word probabilities for fuzzy matching.** Low-probability words from whisper
   signal uncertain transcription. The LLM uses these to interpret noisy input
   (e.g., "kitty tat" with low prob on "tat" → likely "cat").
6. **Split audio on instruction boundary.** When LLM says "instruction ends at
   word N" and we have word-level timing, trim the audio buffer: discard
   everything before the instruction end, carry the remainder forward.

### Sequence Diagram (Proposed)

```
Timeline (seconds)       0.0   0.5   1.0   1.5   2.0   2.5   3.0   3.5   4.0   4.5
                         │     │     │     │     │     │     │     │     │     │
User speaks:             │ "show me a ─── cat" ─│ (silence) │ "show│me a dog"─│
                         │     │     │     │     │     │     │     │     │     │
Client chunks:           ├─C1──┤─C2──┤─C3──┤─C4──┤─C5──┤─C6──┤─C7──┤─C8──┤─C9──┤
                         │seq=1│seq=2│seq=3│seq=4│seq=5│seq=6│seq=7│seq=8│seq=9│
                         │t=T0 │t=T1 │t=T2 │t=T3 │t=T4 │t=T5 │t=T6 │t=T7 │t=T8 │
                         │     │     │     │     │     │     │     │     │     │
WS binary frame:         │     │     │     │     │     │     │     │     │     │
  [8B header][audio]     ├─►───┤─►───┤─►───┤─►───┤─►───┤─►───┤─►───┤─►───┤─►───┤
                         │     │     │     │     │     │     │     │     │     │
Backend accumulates:     │▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓│     │     │     │
                         │     │     │     │ trigger│     │     │     │     │     │
                         │     │     │     │ (VAD  │     │     │     │     │     │
                         │     │     │     │  +time)│     │     │     │     │     │
                         │     │     │     │     │     │     │     │     │     │
Whisper (whole buffer):  │     │     │     ├──────────────►│     │     │     │     │
                         │     │     │     │     │     │  ◄┤ verbose_json:     │
                         │     │     │     │     │     │   │ tokens + timing   │
                         │     │     │     │     │     │   │     │     │     │
Send to client:          │     │     │     │     │     │   ├──► transcript+words
                         │     │     │     │     │     │   │     │     │     │
LLM (with word probs):   │     │     │     │     │     │   ├─────────►│     │
  "show me a cat"        │     │     │     │     │     │   │     │  ◄─┤     │
  [prob: 0.95,0.98,..]   │     │     │     │     │     │   │     │ subject=cat
                         │     │     │     │     │     │   │     │ boundary=word3
                         │     │     │     │     │     │   │     │     │     │
Backend acts:            │     │     │     │     │     │   │     │ ────►media│
  - trim audio at 2.0s   │     │     │     │     │     │   │     │     │     │
  - carry remainder       │     │     │     │     │     │   │     │▓▓▓▓▓│(silence carryover)
  - keep listening        │     │     │     │     │     │   │     │     │     │
                         │     │     │     │     │     │   │     │     │     │
Buffer 2 (carried):      │     │     │     │     │     │   │     │▓▓▓▓▓▓▓▓▓▓│
                         │     │     │     │     │     │   │     │     │  trigger
                         │     │     │     │     │     │   │     │     │     ├──►whisper
```

### New WebSocket Protocol

#### Client → Server: Audio Frame (Binary)

Replace raw binary with a lightweight framed format:

```
Byte offset  Size  Field
0            2     Magic: 0xAB01 (audio frame marker)
2            2     Sequence number (uint16, wraps at 65535)
4            8     Client timestamp (float64, milliseconds since epoch)
12           N     Audio data (WebM/Opus bytes)
```

Total overhead: 12 bytes per chunk. At 500ms intervals = 24 bytes/second.

Why float64 for timestamp: `Date.now()` returns milliseconds as a Number (float64
in JS). No precision loss for the epoch range we care about.

#### Client → Server: Control Messages (Text, unchanged)

```json
{"type": "start_recording", "client_time": 1738764000000}
{"type": "stop_recording"}
{"type": "ping"}
```

Adding `client_time` to `start_recording` gives us a reference point to compute
clock offset between client and server (server records its own `time.Now()` on
receipt).

#### Server → Client: Rich Transcript

```json
{
  "type": "transcript",
  "text": "show me a cat",
  "words": [
    {"word": "show", "start": 0.00, "end": 0.52, "prob": 0.95},
    {"word": "me",   "start": 0.52, "end": 0.76, "prob": 0.98},
    {"word": "a",    "start": 0.76, "end": 0.92, "prob": 0.97},
    {"word": "cat",  "start": 0.92, "end": 1.30, "prob": 0.99}
  ],
  "audio_start_time": 1738764000500,
  "audio_duration": 3.0,
  "is_final": false,
  "segment_id": 1
}
```

- `words`: Per-word timing and probability from whisper verbose_json
- `audio_start_time`: Client wall-clock time of first audio sample in this
  transcription (mapped from chunk sequence numbers)
- `is_final`: True if LLM confirmed instruction boundary
- `segment_id`: Monotonically increasing, so client can track which segments
  are partial updates vs new segments

#### Server → Client: Media (with instruction boundary info)

```json
{
  "type": "media",
  "subject": "cat",
  "photo_url": "/data/media/cat/set1/photo.jpg",
  "audio_url": "/data/media/cat/set1/audio.mp3",
  "instruction": {
    "text": "show me a cat",
    "word_range": [0, 3],
    "audio_time_range": [0.00, 1.30],
    "client_time_range": [1738764000800, 1738764002100]
  }
}
```

### Audio Buffer Management (Proposed)

#### The WebM Container Problem: Parse It Properly (Option D)

WebM is a subset of Matroska, which is EBML (Extensible Binary Meta Language).
Every element is `[ID][Size][Data]` with variable-length ID and size fields.
The structure that matters:

```
WebM file
├── EBML Header         (element ID: 0x1A45DFA3)
├── Segment             (element ID: 0x18538067)
│   ├── Info            (element ID: 0x1549A966)  ─┐
│   ├── Tracks          (element ID: 0x1654AE6B)   ├─ "init segment"
│   ├── Cluster         (element ID: 0x1F43B675)  ─┘
│   │   ├── Timecode    (element ID: 0xE7) = 0ms
│   │   └── SimpleBlock (audio frames)
│   ├── Cluster
│   │   ├── Timecode = 480ms
│   │   └── SimpleBlock
│   ├── Cluster
│   │   ├── Timecode = 960ms
│   │   └── SimpleBlock
│   └── ...
```

The backend parses the EBML stream incrementally as bytes arrive from the
WebSocket. It maintains an index of the structural elements:

```go
type connectionState struct {
    mu              sync.Mutex
    webmParser      *webm.StreamParser  // incremental EBML parser
    initSegment     []byte              // EBML Header + Info + Tracks
    clusters        []ClusterRef        // parsed cluster index
    audioChunks     []AudioChunk        // raw chunks with metadata
    rawBuffer       []byte              // all bytes received (parser reads from this)
    isRecording     bool
    lastActivity    time.Time
    transcriptAccum string              // running transcript across buffers
    lastWhisperEnd  float64             // last confirmed word end time (seconds)
}

type ClusterRef struct {
    TimecodeMs  uint64    // absolute timecode from the Cluster element
    ByteOffset  int       // offset in rawBuffer where this Cluster starts
    ByteLength  int       // length of the complete Cluster
}

type AudioChunk struct {
    Seq       uint16
    ClientTS  float64   // client Date.now() in ms
    ServerTS  time.Time
    Data      []byte
    Offset    int       // byte offset in rawBuffer
}
```

As each WebSocket binary message arrives, bytes are appended to `rawBuffer`
and the EBML parser advances, updating `initSegment` and `clusters[]`.

To build a valid WebM for any time range:

```go
// "Give me audio from 1440ms to 3000ms"
func (s *connectionState) buildWebM(fromMs, toMs uint64) []byte {
    var buf bytes.Buffer
    buf.Write(s.initSegment)
    for _, c := range s.clusters {
        if c.TimecodeMs >= fromMs && c.TimecodeMs < toMs {
            buf.Write(s.rawBuffer[c.ByteOffset : c.ByteOffset+c.ByteLength])
        }
    }
    return buf.Bytes()
}
```

This produces a valid WebM file: init segment + selected Clusters with their
original absolute timecodes. Whisper decodes it correctly regardless of which
Clusters are included.

**Why not simpler approaches:**
- Re-sending the entire buffer each time (option A) works for short sessions
  but scales poorly — re-transcribing 30s of audio to get the last 3s is waste.
- Raw PCM (option C) loses Opus compression: ~256kbps vs ~32kbps.
- Restarting MediaRecorder (option B) creates audible gaps.
- Parsing gives us precise time-range queries with zero audio duplication.

#### Go Library for WebM/EBML Parsing

Research of 11 Go EBML/WebM libraries (2026-02-05). Top 3:

| Library | Stars | Last Commit | License | Notes |
|---------|-------|-------------|---------|-------|
| **[at-wat/ebml-go](https://github.com/at-wat/ebml-go)** | 89 | Sep 2025 | Apache-2.0 | **Recommended.** Highest adoption, active maintenance. Marshal/unmarshal pattern like encoding/json. Includes mkvcore package for Matroska-specific operations. Bi-directional (read + write). |
| [ebml-go/webm](https://github.com/ebml-go/webm) | 44 | Low activity | BSD-3-Clause | WebM-specific reader/seeker abstractions. Only 15 commits total. Depends on separate EBML decoder. |
| [remko/go-mkvparse](https://github.com/remko/go-mkvparse) | 39 | Feb 2021 | MIT | Best API for streaming (event-based push parser, short-circuit support). Zero deps. But **abandoned since 2021**. |

**Recommendation: `at-wat/ebml-go`**

- Most active project in this space (commits in 2025, 89 stars)
- Uses `io.Reader`/`io.Writer` interfaces — can wrap our `rawBuffer` as a reader
- Includes `mkvcore` package with BlockReader/BlockWriter for Cluster manipulation
- Apache-2.0 is compatible with our project
- Can both parse incoming WebM and reconstruct valid WebM for whisper

If `at-wat/ebml-go` proves awkward for incremental parsing, fallback is to fork
`remko/go-mkvparse` (best streaming API, MIT license, just unmaintained).

Also evaluated and rejected: coding-socks/ebml (2 stars), pixelbender/go-matroska
(no streaming), coding-socks/matroska (alpha), quadrifoglio/go-mkv (archived, GPL),
pankrator/ebml-parser (1 star, abandoned), ebml-go/ebml (8 stars, minimal),
mewbak/matroska2 (0 stars), akyoto/go-matroska (1 star).

#### Trigger Logic (Proposed)

Replace the fixed 3-second timer with a smarter trigger:

1. **Minimum accumulation**: At least 1.5 seconds of new audio since last
   transcription (don't spam whisper)
2. **VAD silence detection**: If whisper-server is started with `--vad`, pass
   `vad=true` as a form field. Whisper will segment on silence gaps.
3. **Adaptive threshold**: If whisper returns high-probability tokens with a
   clear sentence end (period, question mark), process immediately. If tokens
   are low-probability or sentence seems incomplete, wait for more audio.
4. **Maximum accumulation**: Hard cap at 10 seconds to prevent unbounded growth.

### LLM Integration (Proposed)

#### System Prompt

```
You are the intent engine for a voice-controlled media application. You receive
transcribed speech and decide what action to take.

Transcription may be noisy — background sounds, microphone artifacts, or
unclear pronunciation can cause errors. Low-probability words (shown in the
word list) are especially suspect. Be generous in interpretation.

<available-concepts>
<concept>cat</concept>
<concept>dog</concept>
<concept>duck</concept>
<concept>pig</concept>
<concept>chicken</concept>
<concept>cow</concept>
</available-concepts>

Rules:
- Call show_media at most once per request.
- If the user names multiple subjects ("a cat and a dog"), call text_to_speech
  to explain you will show one, then call show_media for the first one mentioned.
- If the transcript is clearly incomplete (cut off mid-phrase), call wait_for_more.
- If you cannot match any known concept, call text_to_speech to ask the user
  to try again.
- Keep all spoken text short and friendly.
```

The available concepts list is injected at startup from the database. When new
concepts are added (e.g., `elephant`), they appear in the system prompt
automatically.

#### User Message

The user message contains the transcript text and per-word data from whisper:

```
Transcript: "show me a cat and a dog"
Words:
[0] "show"  start=0.00 end=0.52 prob=0.95
[1] "me"    start=0.52 end=0.76 prob=0.98
[2] "a"     start=0.76 end=0.92 prob=0.97
[3] "cat"   start=0.92 end=1.30 prob=0.99
[4] "and"   start=1.40 end=1.55 prob=0.96
[5] "a"     start=1.55 end=1.65 prob=0.94
[6] "dog"   start=1.65 end=2.10 prob=0.98
```

Word indices and probabilities give the LLM the information it needs to:
- Identify which words form the command
- Report `instruction_end_word_index` for audio buffer trimming
- Gauge transcription confidence from low-probability words

#### Tool Definitions

```json
[
  {
    "name": "show_media",
    "description": "Display a photo or video of the requested subject with its sound. At most one show_media call per request.",
    "input_schema": {
      "type": "object",
      "properties": {
        "subject": {
          "type": "string",
          "description": "The media subject — must be one of the available concepts"
        },
        "instruction_end_word_index": {
          "type": "integer",
          "description": "0-based index of the last transcript word belonging to this command"
        }
      },
      "required": ["subject", "instruction_end_word_index"]
    }
  },
  {
    "name": "text_to_speech",
    "description": "Speak a short message to the user. Use for feedback, clarification, or explaining partial fulfillment.",
    "input_schema": {
      "type": "object",
      "properties": {
        "text": {
          "type": "string",
          "description": "The message to speak aloud. Keep under 30 words."
        }
      },
      "required": ["text"]
    }
  },
  {
    "name": "wait_for_more",
    "description": "The transcript appears incomplete — a sentence was cut off mid-phrase. Wait for more audio before acting.",
    "input_schema": {
      "type": "object",
      "properties": {
        "reason": {
          "type": "string",
          "description": "Why the transcript seems incomplete"
        }
      },
      "required": ["reason"]
    }
  }
]
```

Use `tool_choice: {"type": "any"}` so every LLM call produces at least one
tool response. The model never returns bare text — it always commits to an
action (show, speak, or wait).

#### Tool Call Execution

The LLM may return multiple tool calls (e.g., `text_to_speech` + `show_media`
for the "cat and a dog" case). The backend executes them **in the order the
LLM returned them**, with independent error handling per tool call:

- If `text_to_speech` fails (Piper is down), log a warning and continue to the
  next tool call. The user misses the spoken feedback but still sees media.
- If `show_media` fails (concept not in DB), send an error message to the
  client for that step. Prior successful tool calls (e.g., TTS audio already
  sent) are not rolled back.
- If `wait_for_more` is returned, no further tool calls are processed — the
  backend simply waits for the next transcription cycle.

Each tool call produces a WebSocket message to the client:

| Tool | Client message |
|------|---------------|
| `text_to_speech` | `{"type": "tts_audio", "audio_data": "<base64 WAV>", "text": "..."}` |
| `show_media` | `{"type": "media", "subject": "cat", "photo_url": "...", ...}` |
| `wait_for_more` | No message sent (backend waits silently) |

The client processes messages in arrival order. For `text_to_speech` followed
by `show_media`: play the TTS audio (await the `ended` event), then display
the media and play its sound.

#### Example: "show me a cat and a dog"

LLM response (two content blocks):
```
tool_use: text_to_speech({ "text": "I can only show one at a time — here is the cat!" })
tool_use: show_media({ "subject": "cat", "instruction_end_word_index": 6 })
```

Backend execution:
1. POST "I can only show one at a time — here is the cat!" to Piper → get WAV
2. Send `tts_audio` message to client
3. Look up cat media in DB → get photo/audio URLs
4. Send `media` message to client
5. Trim audio buffer at word 6 ("dog") end time (2.10s)

Client playback:
1. Receive `tts_audio` → play WAV, wait for `ended`
2. Receive `media` → display cat photo, play meow

The `instruction_end_word_index` tells the backend exactly where in the
transcript the command ends. Combined with per-word timestamps from whisper,
the backend computes where in the audio buffer to trim, carrying the
remainder forward for the next transcription cycle.

### Audio Trimming on Instruction Boundary

When LLM returns `instruction_end_word_index=3` and whisper told us word 3
("cat") ends at 1.30s in the audio:

1. Convert word end time to Cluster timecode: 1.30s → 1300ms
2. Find the first Cluster with `TimecodeMs > 1300` (e.g., cluster at 1440ms)
3. Drop all Clusters before that one from the index
4. Update `rawBuffer` to discard consumed bytes (or use an offset pointer)
5. The remaining clusters + future incoming data form the next whisper window
6. `initSegment` is retained — it's always prepended when building WebM

This means if the user says "show me a cat show me a dog" in one breath:
- First transcription gets "show me a cat show me a dog"
- LLM returns instruction_end_word_index=3 (end of "cat")
- Backend trims to keep "show me a dog" audio
- Next transcription cycle picks up "show me a dog"
- LLM returns subject="dog"

---

## Timestamp Alignment: Client ↔ Whisper

### The Problem

- Client has `Date.now()` = wall-clock milliseconds
- Whisper has audio-relative seconds (0.0 = start of audio buffer)
- These two time domains need to be mapped

### The Solution

```
Client sends:
  start_recording { client_time: T_client_start }
  chunk[seq=1] { client_ts: T0 }   ← first audio data
  chunk[seq=2] { client_ts: T1 }
  ...

Server records:
  T_server_start = time.Now()  at start_recording
  clock_offset ≈ T_client_start - T_server_start  (rough, not NTP-grade)

Whisper returns:
  word "cat" at audio_time = 1.30s (relative to start of audio buffer)

Map to client time:
  client_time_of_word = T0 + (audio_time * 1000)
  (T0 = client timestamp of first chunk in the buffer sent to whisper)

Map to server time:
  server_time_of_word = chunk[1].ServerTS + (audio_time * time.Second)
```

This is imprecise (±500ms due to chunk boundaries and network latency) but
sufficient for this use case. We're correlating "when did the user say cat"
to within half a second, not syncing subtitles to music.

---

## Migration Path

All phases follow TDD: write the failing test first, then implement to make
it pass.

### Phase 1: Prove the Bugs, Parse What We Already Have ✓ (Tasks 137-138)

#### Step 1a: Write failing tests (before any implementation)

**Backend unit test: `websocket_webm_test.go`**

`TestBufferSplitProducesValidWebM` — proves the container corruption bug:

- Read the real WebM fixture `tests/fixtures/me-show-me-a-cat.webm` (27,791
  bytes, starts with EBML magic `0x1A 0x45 0xDF 0xA3`, init segment is 497
  bytes, one Cluster from offset 497 to end).
- Create a mock whisper server that captures every multipart `file` field it
  receives (the raw audio bytes) into a `[][]byte` slice.
- Create a WebSocket handler with `BufferThreshold = 500ms`.
- Send the WebM data as 1KB binary chunks with 100ms delays (28 chunks × 100ms
  = 2.8s of sending). The 500ms threshold fires multiple times, each time
  grabbing and clearing the buffer.
- Wait for at least 2 captured whisper requests.
- Assert: **every** request's audio starts with EBML magic `[0x1A 0x45 0xDF 0xA3]`.

Expected result today: request 0 passes (bytes 0-N include the EBML header),
request 1+ fails (bytes start mid-Cluster, no EBML header). This proves the
container corruption bug.

**Backend unit test: `transcribe_test.go` or `websocket_webm_test.go`**

`TestWhisperResponseFullParse` — proves verbose_json data is discarded:

- Create a mock whisper that returns a full verbose_json response including
  `segments[].words[]` with token text, start/end times, and probabilities
  (see the example verbose_json structure earlier in this spec).
- Call the transcription path.
- Assert: the returned result includes words with timing and probability, not
  just the flat text string.

Expected result today: fails because `WhisperResponse` struct has no `Words`
field and `transcribeAudio()` returns only `.Text`.

**E2E test: extend `real-services.spec.ts`**

Add a second test case `continuous listening survives multiple buffer cycles`:

- Use the existing `me-show-me-a-cat.webm` fixture but split into 1KB chunks
  instead of 4KB (28 chunks × 500ms = 14 seconds of apparent recording). This
  triggers the 3-second buffer threshold 4+ times.
- Click mic, wait for cat media to appear (from first buffer cycle).
- Wait several more seconds for subsequent buffer cycles to process.
- Assert: app has not crashed, no error popup visible, mic is still in
  recording state (aria-pressed=true, recording CSS class).

Expected result today: likely fails because subsequent whisper requests get
invalid WebM, causing "transcription failed" errors that trigger error state.

#### Step 1b: Implementation

1. Add `at-wat/ebml-go` dependency.
2. Build incremental WebM parser: extract init segment + Cluster index as bytes
   arrive. Wrap `rawBuffer` in an `io.Reader` for the ebml-go unmarshaler.
3. When building audio for whisper, use `initSegment + selectedClusters` for
   valid WebM on every request (fixes the container corruption bug).
   → `TestBufferSplitProducesValidWebM` passes.
4. Add `Words` and `Tokens` fields to `WhisperResponse`/`WhisperSegment`.
   Parse full verbose_json response (segments, words, tokens, probabilities)
   instead of discarding everything except `.Text`.
   → `TestWhisperResponseFullParse` passes.
5. Send rich transcript to client (words with timing and probability).
6. Pass `split_on_word=true` to whisper for cleaner word boundaries.
   → Extended `real-services.spec.ts` passes.

### Phase 2: Framed Audio Protocol + Timestamp Mapping ✓ (Task 139)

#### Step 2a: Write failing tests

- Frontend unit test: `sendAudioChunk` produces a framed binary message with
  12-byte header (magic `0xAB01`, uint16 seq, float64 timestamp), not raw bytes.
- Backend unit test: `handleAudioChunk` parses the 12-byte header and records
  seq + client timestamp in `AudioChunk` metadata. Rejects frames without the
  magic prefix.
- Backend unit test: after whisper returns word timing, the backend maps
  audio-relative seconds to client wall-clock milliseconds using the chunk
  timestamps.

#### Step 2b: Implementation

1. Add 12-byte header to binary frames (magic, seq, timestamp)
2. Track chunk metadata on backend (correlate chunk seq → Cluster timecodes)
3. Map whisper timestamps to client wall-clock times
4. Add `client_time` to `start_recording` for clock offset

### Phase 3: Smart Triggering + LLM Boundary Detection ✓ (Tasks 140-141)

#### Step 3a: Write failing tests

- Backend unit test: LLM receives a system prompt containing `<concept>` tags
  for all concepts in the database, and per-word data (text, timing,
  probability) in the user message. `tool_choice` is set to `any`.
- Backend unit test: LLM returns `wait_for_more` tool call for an incomplete
  sentence like "show me a". The handler does NOT look up media — it waits
  for more audio.
- Backend unit test: LLM returns `show_media` with `instruction_end_word_index=3`
  for "show me a cat". The handler trims the audio buffer at the Cluster
  boundary corresponding to word 3's end time (1.30s). Remaining audio bytes
  are carried forward in the buffer.
- Backend unit test: LLM returns two `show_media` calls for "show me a cat
  show me a dog". Only the first `show_media` is executed. The second is
  ignored (at most one `show_media` per request).
- Backend unit test: transcript accumulation across buffer cycles. First cycle
  transcribes "show me a", LLM says wait. Second cycle transcribes "show me a
  cat" (accumulated), LLM says show_media. Verify only one media lookup happens.

#### Step 3b: Implementation

1. Replace fixed 3-second timer with adaptive trigger
2. Rewrite LLM integration: system prompt with `<concept>` tags, user message
   with per-word data, `tool_choice: any`, three tools (show_media,
   text_to_speech, wait_for_more)
3. Enforce at most one `show_media` per LLM response
4. Implement audio buffer trimming at Cluster boundaries on instruction end
5. Accumulate transcript across trigger cycles

### Phase 4: TTS Tool + Multi-Tool Execution ✓ (Task 142)

#### Step 4a: Write failing tests

- Backend unit test: LLM returns `text_to_speech` tool call. The handler
  POSTs the text to Piper, receives WAV audio, and sends a `tts_audio`
  WebSocket message to the client with base64-encoded audio data.
- Backend unit test: LLM returns `text_to_speech` + `show_media` (two tool
  calls). Both are executed in order. Client receives `tts_audio` then `media`.
- Backend unit test: Piper is unreachable. `text_to_speech` fails gracefully
  (warning logged), execution continues to `show_media`. Client still receives
  the `media` message.
- Frontend unit test: client receives `tts_audio` followed by `media`. TTS
  audio plays first (await `ended` event), then media is displayed with its
  sound.

#### Step 4b: Implementation

1. Add `text_to_speech` tool to LLM tool definitions
2. Implement sequential tool call execution with per-call error handling
3. Add `tts_audio` WebSocket message type (server → client)
4. Frontend: action queue that plays TTS before rendering media

### Phase 5: VAD Integration ✓ (Tasks 143-144)

#### Step 5a: Write failing tests

- Backend unit test: whisper request includes `vad=true` form field.
- Backend unit test: when whisper returns segments with silence gaps, the
  trigger logic uses segment boundaries instead of fixed time threshold.

#### Step 5b: Implementation

1. Enable VAD in whisper requests (`vad=true`)
2. Use VAD segments to improve trigger timing
3. Auto-detect end-of-utterance via silence
