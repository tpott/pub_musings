# Status

This file tracks high level progress on the peekaboo project.

## Current State

Production-ready voice-controlled web app for children. 124 tasks completed.

### Architecture
- **Go backend** with SQLite, WebSocket audio streaming, age encryption
- **Astro frontend** with TypeScript, 237+ unit tests, 19+ E2E tests
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

- Task 124: Path traversal defense-in-depth for EncryptedFileServer (2026-02-05)
  - Added strings.HasPrefix validation to ensure resolved paths stay within BaseDir
  - Added slog.Error logging for failed io.Copy in serveEncrypted
  - Added path traversal test cases (parent_dir, encoded_traversal, legit file)
  - All backend tests pass, linting passes

## Open Tasks

- Task 125: Remove redundant handler parameter from bufferThresholdWatcher
- Task 126: Update README.md with feedback, theme, continuous listening, WebSocket default
- Task 127: Add CI skip guard to real-services E2E test

## Milestone History

- Tasks 1-14 (2026-02-03/04): Initial scaffold, media, DB, APIs, frontend flow, E2E
- Tasks 15-25 (2026-02-04): Deployment, env config, CORS, SQLite optimization
- Tasks 26-59 (2026-02-04): Security, validation, health checks, rate limiting, accessibility
- Tasks 60-81 (2026-02-04): Piper TTS, WebSocket audio streaming, comprehensive testing
- Tasks 82-102 (2026-02-04/05): Performance tuning, error handling, continuous listening UX
- Tasks 103-123 (2026-02-05): Code organization, feedback, theme, linting
- Task 124 (2026-02-05): Security hardening - path traversal defense
