# PBT (Peekaboo Tester) - Implementation Plan

## Context

Peekaboo is a voice-controlled children's app where a child says "show me a cat" and sees a photo. The existing e2e tests use **mocked** APIs and pre-recorded audio fixtures. There is no way to write simple, human-readable integration tests that exercise the **real** Piper TTS + Whisper STT + LLM pipeline.

PBT is a DSL-driven test tool that lets you write scripts like:

```
say show me a cat
assert media cat
assert no tts
```

It synthesizes real audio via Piper, feeds it through the app's real WebSocket pipeline (Whisper STT -> LLM intent -> media lookup), and asserts on the results using Playwright.

## DSL Syntax

```
# Comments start with #
wait <duration>                  # e.g. wait 2s, wait 500ms
say <text>                       # synthesize audio, inject into app
assert media <subject>           # media-image visible with subject in src
assert no media                  # no media-image visible
assert tts [contains <text>]     # tts_audio WS message received
assert no tts                    # no tts_audio WS message received
assert transcript <text>         # transcript display contains text
assert error [<text>]            # error WS message received
clear                            # reset captured WS messages
```

Assertions poll with a configurable timeout (default 30s), so explicit `wait` after `say` is usually unnecessary.

## Architecture

```
.pbt script  -->  DSL Parser  -->  commands[]
                                      |
                              Pre-synthesize audio:
                              Piper (text->WAV) + ffmpeg (WAV->WebM/Opus)
                                      |
                              Build RecordingSessions
                              (one session per "say" command)
                                      |
                              Inject into Playwright page:
                              - ScriptedMediaRecorder (replays audio)
                              - WebSocket observer (captures server msgs)
                                      |
                              Execute commands sequentially:
                              say -> click mic, chunks flow, backend auto-processes
                              wait -> page.waitForTimeout()
                              assert -> Playwright expect() with polling
                              clear -> reset WS message buffer
```

### How `say` works

Each `say` = one mic start/stop cycle. The ScriptedMediaRecorder holds pre-synthesized audio for each session. On each `start(500)` call, it advances to the next session and emits chunks at 500ms intervals.

1. If mic is currently recording, click to stop first
2. Click mic to start -> `ScriptedMediaRecorder.start(500)` emits WebM chunks
3. Backend's buffer threshold fires after 3s, sends audio to Whisper
4. Whisper transcribes -> LLM extracts intent -> media/TTS response sent back
5. Assertions poll for expected state (up to 30s timeout)

### How assertions work

- **DOM assertions** (`assert media`, `assert transcript`): Playwright `expect(locator)` with timeout
- **WS assertions** (`assert tts`, `assert no tts`, `assert error`): Read captured messages from `window.__PBT_WS_MESSAGES__` via `page.evaluate()`
- **`clear`**: Resets `__PBT_WS_MESSAGES__` array for multi-command tests

## Files to Create

### 1. `frontend/tests/helpers/pbt/types.ts` (~60 lines)
Type definitions: DSL command types, `SynthesizedAudio`, `RecordingSession`, `PBTConfig`.

### 2. `frontend/tests/helpers/pbt/dsl-parser.ts` (~120 lines)
Line-by-line regex parser. `parsePBTFile(path) -> PBTScript`. `parseDuration("2s") -> {ms: 2000}`.

### 3. `frontend/tests/helpers/pbt/piper-client.ts` (~40 lines)
`synthesizeWAV(text, config) -> Buffer`. POST JSON `{text, length_scale}` to Piper server (same protocol as `backend/tts/piper.go`). Returns WAV bytes.

### 4. `frontend/tests/helpers/pbt/audio-pipeline.ts` (~80 lines)
- `wavToWebmOpus(wav: Buffer) -> Buffer` via `execFileSync('ffmpeg', ['-i','pipe:0','-c:a','libopus','-b:a','32k','-ar','16000','-ac','1','-f','webm','pipe:1'])`
- `splitIntoBase64Chunks(webm: Buffer) -> string[]` (4KB chunks, matching `real-services.spec.ts` pattern)
- `synthesizeAndChunk(text, config) -> SynthesizedAudio` (full pipeline)

### 5. `frontend/tests/helpers/pbt/scripted-recorder.ts` (~120 lines)
`getScriptedRecorderScript(sessions: RecordingSession[])` returns a function for `page.addInitScript()`. Replaces `window.MediaRecorder` with a class that:
- Tracks a session index (advances on each `start()` call)
- Emits pre-loaded base64 chunks at `timeslice` intervals via `setInterval`
- On `stop()`, flushes remaining chunks synchronously (matching existing `FileMediaRecorder` pattern from `real-services.spec.ts:82-122`)

### 6. `frontend/tests/helpers/pbt/ws-observer.ts` (~40 lines)
`getWSObserverScript()` returns a function for `page.addInitScript()`. Extends `WebSocket` class, adds `addEventListener('message', ...)` in constructor to capture all incoming JSON messages into `window.__PBT_WS_MESSAGES__`. Exposes `window.__PBT_CLEAR_MESSAGES__()`.

### 7. `frontend/tests/helpers/pbt/runner.ts` (~250 lines)
Core orchestrator:
- `loadConfig()`: reads `PIPER_SERVER_URL`, `PBT_APP_URL`, `PBT_ASSERT_TIMEOUT` from env
- `presynthesizeAudio(script, config)`: parallel Piper+ffmpeg for all `say` commands
- `buildRecordingSessions(script, audioMap)`: one session per `say`
- `executePBTScript(page, script, config)`: inject scripts, navigate, execute commands
- `executeCommand()`: dispatch per command type
- Assertion functions: `assertMedia`, `assertNoMedia`, `assertTTS`, `assertNoTTS`, `assertTranscript`, `assertError`

### 8. `frontend/tests/pbt/pbt.config.ts` (~25 lines)
Playwright config: `testDir: '.'`, `workers: 1`, `timeout: 120000`, no `webServer` (assumes services running).

### 9. `frontend/tests/pbt/pbt-runner.spec.ts` (~50 lines)
Discovers `.pbt` files from `tests/pbt/`, creates one Playwright test per file. Skips if `PIPER_SERVER_URL` not set.

### 10. `tests/pbt/show-cat.pbt`
```
# Basic: say "show me a cat", verify cat media
say show me a cat
assert media cat
assert transcript cat
assert no tts
```

### 11. `tests/pbt/show-unknown.pbt`
```
# Unknown subject should trigger TTS fallback
say what is the weather today
assert no media
assert tts
```

### 12. `tests/pbt/two-commands.pbt`
```
# Two sequential voice commands
say show me a cat
assert media cat
clear
say show me a dog
assert media dog
```

### 13. `frontend/package.json` - add script
```json
"test:pbt": "playwright test --config tests/pbt/pbt.config.ts"
```

## Key Patterns Reused

| Pattern | Source File | Reuse |
|---------|-----------|-------|
| FileMediaRecorder (chunk loading, start/stop, base64->Uint8Array) | `frontend/tests/e2e/real-services.spec.ts:32-85` | ScriptedMediaRecorder extends this pattern |
| Mock getUserMedia + stream | `frontend/tests/helpers/mock-media-recorder.ts:124-131` | Same mock stream object |
| Piper HTTP protocol | `backend/tts/piper.go:69-116` | Same JSON request format |
| WebM chunk splitting | `real-services.spec.ts:20-24` | Same 4KB chunk size |

## Prerequisites

- Backend running (port 8080)
- Frontend dev server running (port 4321)
- Whisper server running (`WHISPER_SERVER_URL`)
- Piper server running (`PIPER_SERVER_URL`)
- `ffmpeg` with `libopus` codec in PATH

## Verification

```bash
# Run all pbt tests
cd frontend && PIPER_SERVER_URL=http://localhost:5000 npm run test:pbt

# Run a specific test
cd frontend && PIPER_SERVER_URL=http://localhost:5000 npx playwright test --config tests/pbt/pbt.config.ts --grep "show-cat"
```

## Implementation Order

1. `types.ts` (no deps)
2. `dsl-parser.ts` (depends on types)
3. `piper-client.ts` (depends on types)
4. `audio-pipeline.ts` (depends on piper-client)
5. `ws-observer.ts` (standalone browser script)
6. `scripted-recorder.ts` (depends on types)
7. `runner.ts` (depends on all above)
8. `pbt.config.ts` + `pbt-runner.spec.ts`
9. `.pbt` example scripts
10. `package.json` script addition
