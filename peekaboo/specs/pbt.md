# PBT (Peekaboo Tester)

**Status:** Implemented (tasks 333-336, 340). Awaiting real-service validation (tasks 337-339).

## Context

Peekaboo is a voice-controlled children's app where a child says "show me a cat"
and sees a photo. The existing e2e tests use **mocked** APIs and pre-recorded audio
fixtures. There is no way to write simple, human-readable integration tests that
exercise the **real** Piper TTS + Whisper STT + LLM pipeline.

PBT is a set of Playwright helper methods that let you write tests like:

```typescript
test('show me a cat displays cat media', async ({ page }) => {
  const pbt = await createPBT(page);
  await pbt.say('show me a cat');
  await pbt.assertMedia('cat');
  await pbt.assertNoTTS();
});
```

It synthesizes real audio via Piper, feeds it through the app's real WebSocket
pipeline (Whisper STT -> LLM intent -> media lookup), and asserts on the results.

## Test API

### PBTRunner methods

```typescript
// Create runner. Reads PIPER_SERVER_URL from env.
// say() throws if PIPER_SERVER_URL not set; sayFixture() works without it.
createPBT(page: Page, opts?: { timeout?: number }) => Promise<PBTRunner>

// --- Input ---
pbt.say(text: string)         // Synthesize via Piper, inject audio, click mic
pbt.sayFixture(filename: string) // Use pre-recorded .webm from tests/fixtures/

// --- Assertions (all poll with timeout, default 30s) ---
pbt.assertMedia(subject: string)         // media-image visible, src contains subject
pbt.assertNoMedia()                      // no media-image visible
pbt.assertTTS(contains?: string)         // tts_audio WS message received
pbt.assertNoTTS()                        // no tts_audio WS message in buffer
pbt.assertTranscript(text: string)       // transcript display contains text
pbt.assertError(text?: string)           // error WS message received

// --- Utilities ---
pbt.clear()   // Reset captured WS message buffer (for multi-command tests)
```

### Example tests

```typescript
// frontend/tests/pbt/show-cat.spec.ts
import { test } from '@playwright/test';
import { createPBT } from '../helpers/pbt/runner';

test('show me a cat displays cat media', async ({ page }) => {
  const pbt = await createPBT(page);
  await pbt.say('show me a cat');
  await pbt.assertMedia('cat');
  await pbt.assertTranscript('cat');
  await pbt.assertNoTTS();
});

// frontend/tests/pbt/show-unknown.spec.ts
test('unknown subject triggers TTS fallback', async ({ page }) => {
  const pbt = await createPBT(page);
  await pbt.say('what is the weather today');
  await pbt.assertNoMedia();
  await pbt.assertTTS();
});

// frontend/tests/pbt/two-commands.spec.ts
test('two sequential voice commands', async ({ page }) => {
  const pbt = await createPBT(page);
  await pbt.say('show me a cat');
  await pbt.assertMedia('cat');
  await pbt.clear();
  await pbt.say('show me a dog');
  await pbt.assertMedia('dog');
});

// frontend/tests/pbt/fixture-cat.spec.ts
// Works without Piper — uses pre-recorded audio
test('show cat from fixture', async ({ page }) => {
  const pbt = await createPBT(page);
  await pbt.sayFixture('me-show-me-a-cat.webm');
  await pbt.assertMedia('cat');
});
```

## Architecture

```
Test calls pbt.say("show me a cat")
    |
    v
Piper HTTP API: text -> WAV bytes
    |
    v
ffmpeg: WAV -> WebM/Opus
    |
    v
Split into 4KB base64 chunks
    |
    v
Inject ScriptedMediaRecorder via addInitScript (first say)
  or push session via page.evaluate (subsequent says)
    |
    v
Navigate to page (first say only)
    |
    v
Click mic button -> ScriptedMediaRecorder emits chunks
    |
    v
Real WebSocket -> real backend -> Whisper -> LLM -> response
    |
    v
Assertions poll DOM / captured WS messages
```

### How `say()` works

**First call** in a test:
1. Synthesize audio: Piper (text -> WAV) + ffmpeg (WAV -> WebM/Opus)
2. Split WebM into 4KB base64 chunks
3. `page.addInitScript()` to inject ScriptedMediaRecorder + WS observer
4. Set `window.__PBT_SESSIONS__ = [chunks]`
5. `page.goto('/')`
6. Click mic -> ScriptedMediaRecorder emits chunks at 500ms intervals
7. Backend buffer threshold fires after 3s, sends to Whisper

**Subsequent calls** in the same test:
1. Synthesize + chunk audio (same as above)
2. `page.evaluate()` to push new session to `window.__PBT_SESSIONS__`
3. If recorder is active, click mic to stop
4. Click mic to start -> ScriptedMediaRecorder advances to next session

### How `sayFixture()` works

Same as `say()` but reads the .webm file from `tests/fixtures/` instead of
calling Piper + ffmpeg. Useful for:
- Running tests when Piper is unavailable
- Deterministic audio (no TTS variance)

### ScriptedMediaRecorder

Injected via `addInitScript()`. Reads sessions from `window.__PBT_SESSIONS__`
(an array of `string[][]` — array of sessions, each session is an array of
base64 chunks). On each `start(500)` call, advances to the next session and
emits chunks at 500ms intervals. On `stop()`, flushes remaining chunks.

This is the same pattern as `FileMediaRecorder` in `real-services.spec.ts`
but generalized to support multiple sequential sessions.

### WS observer

Injected via `addInitScript()`. Wraps `WebSocket` constructor to intercept
incoming messages. Stores parsed JSON messages in `window.__PBT_WS_MESSAGES__`.
Exposes `window.__PBT_CLEAR_MESSAGES__()` for the `clear()` method.

## Files (Implemented)

### Helpers (`frontend/tests/helpers/pbt/`)

**`types.ts`** (15 lines)
- `PBTConfig`: `{ piperUrl: string, timeout: number }`
- `SynthesizedAudio`: `{ text: string, chunks: string[] }`

**`piper-client.ts`** (26 lines)
- `synthesizeWAV(text: string, piperUrl: string): Promise<Buffer>`
- POST `{ text, length_scale: 1.0 }` to Piper server, returns WAV bytes
- Same protocol as `backend/tts/piper.go` Synthesize method

**`audio-pipeline.ts`** (62 lines)
- `wavToWebmOpus(wav: Buffer): Buffer` — ffmpeg via `execFileSync`
- `splitIntoChunks(webm: Buffer, chunkSize?: number): string[]` — 4KB base64 chunks
- `synthesizeAndChunk(text: string, piperUrl: string): Promise<SynthesizedAudio>`
- `fixtureToChunks(fixturePath: string): string[]`

**`scripted-recorder.ts`** (96 lines)
- `getScriptedRecorderScript(): () => void` for `page.addInitScript()`
- Defines ScriptedMediaRecorder class that reads from `window.__PBT_SESSIONS__`
- Defines mock `getUserMedia` returning a mock stream

**`ws-observer.ts`** (44 lines)
- `getWSObserverScript(): () => void` for `page.addInitScript()`
- Wraps WebSocket to capture incoming JSON messages
- `window.__PBT_WS_MESSAGES__` and `window.__PBT_CLEAR_MESSAGES__()`

**`runner.ts`** (145 lines)
- `createPBT(page, opts?)` factory function
- `PBTRunner` class with all methods described in Test API
- Handles first-say vs subsequent-say injection logic
- All assertion methods use Playwright `expect()` with configurable timeout
- `pollWSMessage()` deduplicates assertTTS/assertError polling

### Tests (`frontend/tests/pbt/`)

**`pbt.config.ts`** (20 lines) — `workers: 1`, `timeout: 120_000`, no `webServer`
**`show-cat.spec.ts`** (10 lines), **`show-unknown.spec.ts`** (10 lines),
**`two-commands.spec.ts`** (14 lines), **`fixture-cat.spec.ts`** (9 lines)

### Package script

`frontend/package.json`: `"test:pbt": "playwright test --config tests/pbt/pbt.config.ts"`

## Patterns Reused

| Pattern | Source | Reuse |
|---------|--------|-------|
| FileMediaRecorder | `real-services.spec.ts` injectFileMediaRecorder | ScriptedMediaRecorder generalizes this |
| Mock getUserMedia | `mock-media-recorder.ts` | Same mock stream object |
| Piper HTTP protocol | `backend/tts/piper.go` Synthesize method | Same JSON request format |
| WebM chunk splitting | `real-services.spec.ts` | Same 4KB chunk size |

## Implementation Tasks

1. ~~**Task 333:** Write test files + pbt.config.ts + package script~~ Done
2. ~~**Task 334:** Write types.ts, piper-client.ts, audio-pipeline.ts~~ Done
3. ~~**Task 335:** Write scripted-recorder.ts, ws-observer.ts~~ Done
4. ~~**Task 336:** Write runner.ts~~ Done
5. ~~**Task 340:** Refactor helpers under 150 lines~~ Done
6. **Task 337:** Validate fixture-cat.spec.ts with real services (needs backend + whisper)
7. **Task 338:** Validate show-cat.spec.ts with real services (needs Piper too)
8. **Task 339:** Validate remaining specs (show-unknown, two-commands)
9. ~~**Task 341:** Update specs/pbt.md and AGENTS.md~~ Done

## Prerequisites

- Backend running (port 8080)
- Frontend dev server running (port 4321)
- Whisper server running (`WHISPER_SERVER_URL`)
- ffmpeg with libopus in PATH (confirmed available)
- Piper server running (`PIPER_SERVER_URL`) — only for `say()`, not `sayFixture()`

## Environment Setup

PBT helpers read service URLs from environment variables. Source the project
`.env` before running:

```bash
# Load environment (from project root)
source .env

# Or export individually
export PIPER_SERVER_URL=http://10.0.2.2:8051
export WHISPER_SERVER_URL=http://10.0.2.2:8765
```

The `pbt.config.ts` Playwright config does NOT start services — backend,
frontend, Whisper, and Piper must already be running.

Fixture-based tests (`sayFixture`) need Whisper + backend + frontend.
Piper-based tests (`say`) additionally need Piper.

## Verification

```bash
# Run all pbt tests (needs all services)
cd frontend && npm run test:pbt

# Run fixture-only tests (no Piper needed)
cd frontend && npx playwright test --config tests/pbt/pbt.config.ts fixture-cat

# Run a specific test
cd frontend && npx playwright test --config tests/pbt/pbt.config.ts --grep "show.*cat"
```
