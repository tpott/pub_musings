# Peekaboo

## What This Is

A voice-activated content viewer for kids. A web app where a toddler or parent says things like "show me a cat" and the app responds by displaying a photo or video of a cat, optionally accompanied by an audio clip (like a meow). It listens continuously via streaming speech-to-text, and an LLM judges whether speech is directed at the app based on context and intent — no rigid wake word required.

## Core Value

A child says "show me a [thing]" and immediately sees and hears that thing. The voice-to-visual loop must feel instant and magical.

## Requirements

### Validated

(None yet — ship to validate)

### Active

- [ ] Continuous speech-to-text streaming via whisper (separate service)
- [ ] Real-time transcription display in a scrolling dialog box
- [ ] LLM intent detection from transcription stream (supports Anthropic and OpenAI APIs)
- [ ] Content display area for photos, videos, and optional audio clips
- [ ] SQLite database mapping concepts (e.g. "cat") to encrypted media files
- [ ] Age-encrypted media storage on the filesystem (.age files, X25519 + ChaCha20-Poly1305)
- [ ] Mic toggle button (listening/not listening) with visual indicator (red = listening)
- [ ] Admin API for adding/managing content
- [ ] Python REPL + helper scripts for local content review and upload to server
- [ ] Deployment via webhook-deployer (same host as subtitler)

### Out of Scope

- Text-to-speech responses — deferred to v2, not needed for core content display
- Weather, time, and other non-content tool calls — v2 feature
- API-sourced content (Unsplash, Pexels) — v2 enhancement to content pipeline
- Wake word detection — LLM judges intent from context, no rigid trigger phrase
- Mobile native app — web-first

## Context

- **Existing patterns:** The subtitler project (same repo) provides proven patterns for age encryption, whisper integration (as a separate service), Go backend architecture, and webhook-deployer configuration.
- **Encryption spec:** Follow the pattern from `subtitler/specs/encryption.md` — age library, `.age` file extension, decrypt-on-demand, key stored at `data/age.key` with 0600 permissions.
- **Whisper architecture:** Whisper runs as a separate service outside the VM (same pattern as subtitler). Plan to build/fork whisper-stream with webhook support for streaming transcription results back to the Go backend.
- **Target users:** Toddlers speaking directly to the app, and parents speaking on behalf of children (e.g. "show her a dog"). The STT and LLM must handle both adult and child speech patterns.
- **Intent detection:** The LLM receives continuous transcription and makes judgment calls about whether speech is directed at the app. "Hey peekaboo" is a strong signal but not required — the LLM should also recognize direct commands like "show me a cat" from context.
- **Content model:** Each concept (cat, dog, giraffe, etc.) maps to one or more media items. Each media item has a type (photo, video, audio), an encrypted file path, and metadata. Audio clips are optional companions to photos/videos.
- **Admin workflow:** Content is curated by hand. A Python REPL with helper scripts interacts with an admin API on the server. Review happens locally (see photos/videos in browser), approved content is pushed to the remote server.

## Constraints

- **Backend language**: Go — consistent with subtitler, reuse patterns
- **Frontend framework**: React/Next.js
- **Database**: SQLite for content catalog
- **Encryption**: Age (filippo.io/age) — same library and pattern as subtitler
- **STT**: Whisper running as a separate service, streaming transcription
- **LLM providers**: Must support both Anthropic and OpenAI APIs
- **Deployment**: Same host as subtitler, configured via `pub_musings/webhook-deployer/config.yaml`
- **No side-effect commands**: Per repo conventions, do not run `npm install`, `pip install`, etc. during implementation — ask the user to run them.

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go backend | Consistency with subtitler, reuse encryption/deployment patterns | — Pending |
| Whisper as separate service | Follows proven subtitler architecture, keeps compute isolated | — Pending |
| Dual LLM provider support | Flexibility to switch between Anthropic and OpenAI based on quality/cost | — Pending |
| SQLite for content DB | Simple, no external dependencies, sufficient for concept-to-media mapping | — Pending |
| Age encryption for media | Proven pattern from subtitler, strong security for kid content | — Pending |
| LLM-based intent detection (no wake word) | More natural interaction for kids and parents, handles ambiguous speech | — Pending |
| Admin via Python REPL + API | Separate admin tooling from kid-facing app, local review before upload | — Pending |
| Content display only in v1 (no TTS) | Reduce scope, core value is visual/audio content from database | — Pending |

---
*Last updated: 2026-02-02 after initialization*
