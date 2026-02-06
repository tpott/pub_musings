# Status

This file tracks high level progress on the peekaboo project.

## Current State

Production-ready voice-controlled web app for children. 139 tasks completed.

### Architecture
- **Go backend** with SQLite, WebSocket audio streaming, age encryption
- **Astro frontend** with TypeScript, 241+ unit tests, 19+ E2E tests
- **External services**: whisper-server (STT), Anthropic/OpenAI (intent), optional Piper (TTS)

### Key Features
- Full voice flow: record → transcribe → intent → media display
- WebSocket streaming (default) with continuous listening mode
- Day/night theme (light/dark/auto) with warm neutral palette
- Feedback form with database persistence and rate limiting
- Piper TTS integration (optional)
- Accessibility: ARIA labels, skip-to-content, keyboard navigation, screen reader support
- Security: CSP headers, path traversal defense, URL validation, rate limiting, CORS
- Structured logging, health probes, graceful shutdown

### Documentation
- docs/API.md, docs/DEPLOY.md, docs/SECURITY.md, docs/TROUBLESHOOTING.md, docs/PERFORMANCE.md
- specs/ directory with architecture, WebSocket protocol, continuous listening, feedback, Piper TTS, Bazel research

### Quality
- Linting: gofmt, go vet, golangci-lint, npm build, filesize lint (scripts/lint.sh)
- Pre-commit hook runs lint + tests for changed projects
- Test organization: files split to stay under 500 lines

## Last Completed

- Task 139 (2026-02-06): Phase 2 — Framed audio protocol with 12-byte headers
  - Frontend `sendAudioChunk` prepends 12-byte header: magic 0xAB01, uint16 seq, float64 timestamp
  - Frontend `startRecording` includes `client_time` (Date.now()) in control message
  - Sequence number resets on each new recording session
  - Backend `handleAudioChunk` parses frame header, strips it before buffering
  - Backward compatible: raw binary without magic prefix still works (legacy)
  - `AudioChunkMeta` records seq, client timestamp, server timestamp per chunk
  - `TranscriptMessage` includes `audio_start_time` (client wall-clock ms of first chunk)
  - 4 new backend tests (`websocket_framing_test.go`), 2 updated frontend tests

## Milestone History

- Tasks 1-14 (2026-02-03/04): Initial scaffold, media, DB, APIs, frontend flow, E2E
- Tasks 15-25 (2026-02-04): Deployment, env config, CORS, SQLite optimization
- Tasks 26-59 (2026-02-04): Security, validation, health checks, rate limiting, accessibility
- Tasks 60-81 (2026-02-04): Piper TTS, WebSocket audio streaming, comprehensive testing
- Tasks 82-102 (2026-02-04/05): Performance tuning, error handling, continuous listening UX
- Tasks 103-123 (2026-02-05): Code organization, feedback, theme, linting
- Tasks 124-127 (2026-02-05): Security hardening, code cleanup, docs, CI guard
- Tasks 128-131 (2026-02-05): Server timeouts, XSS fix, doc fixes, event listener cleanup
- Task 132 (2026-02-05): Timer leak fixes (error timeout, reconnect timeout)
- Task 133 (2026-02-05): Unchecked ResponseWriter.Write error handling
- Task 134 (2026-02-05): WebSocket magic numbers → named constants
- Task 135 (2026-02-05): Failing test proving WebM container corruption on buffer split
- Task 136 (2026-02-05): Failing test proving whisper verbose_json word data is discarded
- Task 137 (2026-02-06): Fix WebM container corruption with EBML init segment caching
- Task 138 (2026-02-06): Parse full whisper verbose_json with word-level timing
- Task 139 (2026-02-06): Framed audio protocol with 12-byte headers and timestamp mapping
