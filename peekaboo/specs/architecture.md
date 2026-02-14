# Peekaboo Architecture Spec

## Overview

Peekaboo is a voice-controlled web app for children that responds to prompts like "show me a cat" by displaying photos/videos and playing audio. It functions similarly to a Google Home speaker but with visual output.

## Core User Flow

1. User presses button to activate microphone (recording begins)
2. Frontend streams audio chunks (~500ms) to backend via WebSocket
3. Backend forwards chunks to whisper-server for transcription
4. When transcript ready: LLM extracts intent + subject
5. App looks up matching media from database
6. Display photo/video and play audio (TTS or pre-recorded)
7. **Mic remains active** — user can issue another command immediately
8. User presses button again to deactivate microphone (optional)

**Key UX requirement:** Steps 3-6 happen **while the microphone is still listening**.
The user sees results without needing to stop recording first. This enables a
continuous, conversational experience similar to Google Home.

**Transcript display:** A scrollable transcript area below the media display shows
recognized voice commands. This helps users see what the system heard, useful for
debugging microphone issues or unclear speech.

---

## Component Research

### 1. Speech-to-Text (STT)

**Researched:** 2026-02-03
**Confidence:** HIGH

**Chosen approach:** Use **existing whisper-server** running on baremetal Mac.

The whisper-server is already deployed and proven. Flow:
1. Frontend captures audio from microphone (MediaRecorder API)
2. Frontend streams audio chunks (~500ms) to backend via WebSocket
3. Backend buffers chunks and forwards to whisper-server for transcription
4. Backend sends transcript back to frontend via WebSocket
5. Processing happens while mic is still active (no stop-then-send)

**Model selection:**
- **Development/testing:** Use smallest model (`ggml-tiny.en.bin` or `ggml-base.en.bin`) for fast iteration
- **Production:** Use larger model for accuracy with child speech

**Network topology:**
```
┌─────────────────┐      ┌─────────────────┐
│   VM (ralph)    │      │  Baremetal Mac  │
│                 │      │                 │
│  peekaboo       │ ──── │  whisper-server │
│  backend        │ HTTP │  :8765          │
│                 │      │                 │
└─────────────────┘      └─────────────────┘
```

Backend uses `WHISPER_SERVER_URL` env var (default `http://127.0.0.1:8765`) to reach the whisper-server.
Reference subtitler's setup for VM-to-host networking (likely `10.0.2.2` for QEMU or host IP for bridged).

**Future enhancements:**
- Phoneme/partial word output for guessing unclear speech
- Voice activity detection (VAD) to auto-segment utterances

**Sources:**
- [whisper.cpp](https://github.com/ggml-org/whisper.cpp)
- [subtitler whisper integration](/Users/trevor/Github/pub_musings/subtitler/README.md)

---

### 2. Text-to-Speech (TTS)

**Researched:** 2026-02-03
**Confidence:** MEDIUM
**MVP Status:** Not required for MVP

**Chosen approach:** Use **Piper** for local neural TTS (post-MVP).

Piper is a fast, local neural text-to-speech system:
- Runs entirely offline (no cloud dependency)
- Uses ONNX models trained with VITS
- 10x faster than cloud TTS for real-time apps
- Install via `pip install piper-tts`
- Works on Linux, macOS (including Apple Silicon)

**Deployment options:**
- Run piper as a service on backend
- Pre-generate common phrases as audio files
- Containerized web API via FastAPI available

**For MVP:** Rely on pre-recorded audio clips bundled with media sets.

**Sources:**
- [Piper GitHub](https://github.com/rhasspy/piper)
- [Piper Voice Samples](https://rhasspy.github.io/piper-samples/)
- [piper-tts PyPI](https://pypi.org/project/piper-tts/)

---

### 3. LLM Intent Recognition

**Researched:** 2026-02-03
**Confidence:** HIGH

**Problem:** Convert speech like "show me a cat" or "I want to see a giraffe" into structured intent:
```json
{"action": "show", "subject": "cat"}
```

**Chosen approach:** Use **LLM tool calls** (function calling) with Claude Haiku for MVP.

The LLM receives the transcript and a tool definition. It extracts the intent by "calling" the tool with structured parameters. This approach:
- Guarantees structured JSON output
- Handles fuzzy/unclear speech naturally
- No brittle regex patterns to maintain

**Tool definition:**
```json
{
  "name": "show_media",
  "description": "Show a photo or video of something",
  "input_schema": {
    "type": "object",
    "properties": {
      "subject": {
        "type": "string",
        "description": "The thing to show (e.g., cat, dog, cow)"
      }
    },
    "required": ["subject"]
  }
}
```

**Example flow:**
1. User says: "I wanna see a kitty cat"
2. Whisper transcribes: "I wanna see a kitty cat"
3. Claude Haiku with tool call returns: `{"subject": "cat"}`
4. Backend looks up "cat" in media database

**MVP:** Anthropic API with Claude Haiku (claude-3-haiku)
**Future:** Abstract LLM provider to support OpenAI function calling

**Sources:**
- [Anthropic Tool Use](https://docs.anthropic.com/en/docs/build-with-claude/tool-use)
- [OpenAI Function Calling](https://platform.openai.com/docs/guides/function-calling)

---

### 4. Media Database

**Researched:** 2026-02-03
**Confidence:** HIGH

**Chosen approach:** Curated local database with SQLite + filesystem.

**Media set structure:**
Each concept (e.g., "cat") has one or more media sets stored in `data/` (gitignored, encrypted):
```
data/
  media/
    cat/
      set1/
        photo.jpg       # required
        audio.mp3       # optional (animal sound)
      set2/
        video.mp4
        ...
```

**Sourcing script:**
`scripts/source-media.sh` downloads CC0/public domain media for all concepts:
- Photos from Unsplash, Pexels, Wikimedia Commons
- Audio from Freesound, BBC Sound Effects
- Stores license info in `data/media/{concept}/LICENSE.txt`

**MVP animals (6 concepts):**
- cat
- dog
- duck
- pig
- chicken
- cow

**Database schema:**
```sql
CREATE TABLE concepts (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL
);

CREATE TABLE media_sets (
  id INTEGER PRIMARY KEY,
  concept_id TEXT REFERENCES concepts(id),
  photo_path TEXT NOT NULL,
  audio_path TEXT,
  video_path TEXT
);
```

**Content sourcing:**
- Photos: Creative Commons from Unsplash, Pexels, Wikimedia
- Audio: Freesound.org, BBC Sound Effects (CC licensed)
- Videos: Pexels Videos, Pixabay

**Sources:**
- [Unsplash](https://unsplash.com) - Free high-quality photos
- [Freesound](https://freesound.org) - CC-licensed audio
- [Pexels Videos](https://www.pexels.com/videos/) - Free stock videos

---

### 5. Deployment Architecture

**Researched:** 2026-02-03
**Confidence:** HIGH

**Chosen approach:** Follow subtitler pattern with mobile-first frontend.

```
┌─────────────────────────────────────────────────────┐
│           Browser (mobile-first, touch)             │
│  ┌──────────┐  ┌──────────────────────────────────┐ │
│  │ Mic      │  │ Media Display (photo/video)      │ │
│  │ (Record) │  │                                  │ │
│  └────┬─────┘  └───────────────▲──────────────────┘ │
│       │                        │                    │
│       └────────┬───────────────┘                    │
│                │                                    │
│       ┌────────▼────────┐                           │
│       │    Frontend     │                           │
│       │    (Astro)      │                           │
│       └────────┬────────┘                           │
└────────────────┼────────────────────────────────────┘
                 │ /api/*
┌────────────────▼────────────────────────────────────┐
│              Backend (Go)                           │
│  ┌──────────┐  ┌──────────┐  ┌──────────────────┐  │
│  │ Intent   │  │ Media    │  │ Whisper Client   │  │
│  │ Processor│  │ Database │  │ (to existing     │  │
│  │          │  │ (SQLite) │  │  whisper-server) │  │
│  └──────────┘  └──────────┘  └──────────────────┘  │
└─────────────────────────────────────────────────────┘
                                        │
                                        ▼
                              ┌──────────────────┐
                              │ whisper-server   │
                              │ (Mac host,       │
                              │  existing)       │
                              └──────────────────┘
```

**Target devices:**
- Primary: Touch-based phone browser
- Secondary: Laptop browser

**Deployment targets:**
- webhook-deployer config for frontend + backend (like subtitler)
- Reuse existing whisper-server on Mac host

---

## Decisions Made

1. **STT:** Use existing whisper-server (not browser Web Speech API)
2. **TTS:** Piper for future; MVP uses pre-recorded audio only
3. **Frontend:** Astro (mobile-first, touch-friendly)
4. **Backend:** Go with SQLite
5. **Target:** Primary phone (touch), secondary laptop
6. **MVP animals:** cat, dog, duck, pig, chicken, cow
7. **Intent:** LLM tool calls with Anthropic Haiku (MVP), OpenAI support later

---

### 6. Encryption

**Researched:** 2026-02-03
**Confidence:** HIGH

**Chosen approach:** Follow subtitler encryption pattern.

**Media encryption (age library):**
- Use `filippo.io/age` for file-at-rest encryption
- Media files stored as `{path}.age`
- Decrypt on-demand when serving to client
- Key stored in `data/age.key` with 0600 permissions

**Environment variables (sops):**
- Use sops to encrypt sensitive env vars (API keys, etc.)
- Decrypt to `.env` files at deploy time
- `.env` and `data/` in `.gitignore`

**Files to gitignore:**
```
.env
data/
```

**Sources:**
- [subtitler encryption spec](../subtitler/specs/encryption.md)
- [age library](https://github.com/FiloSottile/age)
- [sops](https://github.com/getsops/sops)

---

### 7. Testing

**Researched:** 2026-02-03
**Confidence:** HIGH

**Chosen approach:** Playwright e2e tests with mocked external services.

**E2E test flow:**
1. Click microphone button
2. Play mock audio file ("show me a cat" - fair use test recording)
3. Mock whisper-server returns transcript: "show me a cat"
4. Mock Anthropic API returns tool call: `{"subject": "cat"}`
5. Assert: cat photo displayed, cat audio plays

**Mock strategy:**
- Use Playwright's route interception for API mocking
- Pre-recorded test audio (own voice saying "show me a cat" = fair use)
- Test media files: simple CC0/public domain cat photo + audio

**Test file structure:**
```
tests/
  e2e/
    peekaboo.spec.ts    # Main e2e test
  fixtures/
    test-audio-show-me-cat.webm  # Self-recorded test audio
    mock-cat-photo.jpg           # CC0 test image
    mock-cat-audio.mp3           # CC0 test audio
```

**Playwright test outline:**
```typescript
test('voice command shows cat media', async ({ page }) => {
  // Mock whisper-server
  await page.route('**/api/transcribe', route =>
    route.fulfill({ json: { text: 'show me a cat' } })
  );

  // Mock Anthropic (via backend proxy)
  await page.route('**/api/intent', route =>
    route.fulfill({ json: { subject: 'cat' } })
  );

  // Mock media endpoint to return test fixtures
  await page.route('**/api/media/cat', route =>
    route.fulfill({ json: {
      photo: '/fixtures/mock-cat-photo.jpg',
      audio: '/fixtures/mock-cat-audio.mp3'
    }})
  );

  await page.goto('/');
  await page.click('[data-testid="mic-button"]');

  // Simulate audio input (or skip recording in test mode)
  await page.waitForSelector('[data-testid="media-display"] img');
  await expect(page.locator('img')).toHaveAttribute('src', /cat/);
});
```

**Sources:**
- [Playwright docs](https://playwright.dev/docs/mock)

---

## MVP Scope

**Phase 1 - MVP:**
- Astro frontend (mobile-first, big touch button)
- Go backend with SQLite
- Whisper-server integration for STT
- Claude Haiku tool calls for intent recognition
- 6 animals with curated media sets (photo + optional audio/video)
- Toggle button for mic on/off
- **WebSocket audio streaming** (process while mic active, no stop-then-send)
- Deployment via webhook-deployer

**Phase 2 - Enhanced:** *(Piper TTS, OpenAI provider, and auth system done)*
- More concepts (colors, shapes, numbers)
- Multiple media sets per concept

**Phase 3 - Advanced:** *(VAD done)*
- Phoneme/partial word output for unclear speech
- ~~Voice activity detection (VAD) for auto-segmentation~~ *(done — whisper VAD integration, task 143)*
- Real-time streaming whisper (sub-chunk latency)
