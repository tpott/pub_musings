# Status

This file tracks high level progress on the peekaboo project.

## Current State

Production-ready voice-controlled web app for children. 300 tasks completed.

### Architecture
- **Go backend** with SQLite, WebSocket audio streaming, age encryption
- **Astro frontend** with TypeScript, 328 unit tests, 56 E2E tests
- **External services**: whisper-server (STT), Anthropic/OpenAI (intent), optional Piper (TTS)

### Key Features
- Full voice flow: record → transcribe → intent → media display
- WebSocket streaming (default) with continuous listening mode
- Day/night theme (light/dark/auto) with warm neutral palette
- Feedback form with database persistence and rate limiting
- Piper TTS integration (optional)
- Accessibility: ARIA labels, keyboard navigation, screen reader support
- Security: CSP headers, path traversal defense, URL validation, rate limiting, CORS
- Structured logging, health probes, graceful shutdown

### Documentation
- docs/API.md, docs/DEPLOY.md, docs/SECURITY.md, docs/TROUBLESHOOTING.md, docs/PERFORMANCE.md
- specs/ directory with architecture, WebSocket protocol, continuous listening, feedback, Piper TTS, Bazel research

### Quality
- Linting: gofmt, go vet, golangci-lint, npm build, filesize lint (scripts/lint.sh)
- Pre-commit hook runs lint + tests for changed projects
- Test organization: files split to stay under 500 lines

## Blocked

- **HELP.md filed (2026-02-11):** Production media 404 (encrypted files, missing age key), local whisper-server broken. Needs human intervention — see HELP.md for details.

## Last Completed

- Tasks 333-336, 340-341 (2026-02-13): PBT framework complete:
  - pbt.config.ts (workers:1, timeout:120s, no webServer)
  - show-cat.spec.ts, show-unknown.spec.ts, two-commands.spec.ts, fixture-cat.spec.ts
  - test:pbt script in package.json
  - Tests import createPBT from helpers/pbt/runner — won't compile until runner.ts built
  - Task 334: types.ts, piper-client.ts (Piper HTTP), audio-pipeline.ts (ffmpeg WAV→WebM, chunking)
  - Task 335: scripted-recorder.ts (multi-session MediaRecorder mock), ws-observer.ts (WS message capture)
  - Task 336: runner.ts (createPBT factory, PBTRunner with say/sayFixture/assert methods)
  - Task 340: Refactored all helpers under 150 lines
  - Task 341: Updated specs/pbt.md and AGENTS.md with PBT commands
- Tasks 331-332 (2026-02-11): Deep inspection round 23 — two fixes:
  - Task 331: Update specs/auth.md to match current code (fix handler file path to backend/api/handlers_auth.go, update sessions schema token→token_hash)
  - Task 332: Fix star rating accessibility — remove incorrect role=radiogroup, add aria-pressed to toggle buttons
- Tasks 328-330 (2026-02-11): Deep inspection round 22 — three fixes:
  - Task 328: Fix unhandled promise rejection in peekaboo-flow.ts ondataavailable (add .catch to sendAudioChunk)
  - Task 329: Add aria-hidden="true" to all decorative SVG icons (8 components/pages, 10 SVGs total)
  - Task 330: Add early hex format validation in CSRF middleware before DB lookup (skip SHA-256 hash + DB query for malformed tokens)
- Tasks 326-327 (2026-02-11): Deep inspection round 21 — fix go.mod direct/indirect dependency classification (6 deps), harden systemd service file
- Tasks 321-325 (2026-02-10): Deep inspection round 19 — WSAuthTracker cleanup, token validation, promise caching, TOTP input cleanup, recording double-tap guard
- Task 315 (2026-02-10): Hash session tokens before DB storage with SHA-256
- Tasks 291-314, 316-320 (2026-02-10): Deep inspection rounds 5-18 + auth cleanup + test coverage
- Tasks 276-290 (2026-02-10): User feedback fixes + deep inspection rounds 3-4
- Tasks 257-275 (2026-02-09): Deep inspection rounds 1-2 + codebase inspection fixes

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
- Task 140 (2026-02-06): LLM boundary detection with tool_choice:any and three tools
- Task 141 (2026-02-06): Audio buffer trimming at Cluster boundaries + transcript accumulation
- Task 142 (2026-02-06): TTS tool execution and multi-tool sequential processing
- Task 143 (2026-02-06): Whisper VAD integration for silence-based triggering
- Task 144 (2026-02-06): E2E endurance test for continuous listening
- Task 145 (2026-02-06): Split oversized websocket_llm_test.go into two files
- Task 146 (2026-02-06): Split 5 remaining oversized test files (backend + frontend)
- Task 147 (2026-02-06): Update audio-timing.md spec status to Implemented
- Task 148 (2026-02-06): Update API.md WebSocket protocol documentation
- Task 149 (2026-02-06): Update outdated documentation references
- Task 150 (2026-02-06): Fix PeekabooFlow.destroy() resource cleanup gaps
- Task 151 (2026-02-06): Fix response body leak in fetchWithRetry on retryable status codes
- Task 152 (2026-02-06): Fix InstructionEndWordIdx zero-value ambiguity in LLM tool parsing
- Task 153 (2026-02-06): Fix WebSocket reconnection bug + reconnect/ping/message tests
- Task 154 (2026-02-06): Add media load error handlers + action loop context cancellation
- Task 155 (2026-02-06): Wrap handleWsMedia in try-catch to prevent stuck state on display error
- Task 156 (2026-02-06): Add tests for untested PeekabooFlow code paths
- Tasks 157-162 (2026-02-06): Security hardening batch + E2E theme test coverage
- Tasks 163-166 (2026-02-06): Piper TTS hardening, feedback improvements, spec maintenance
- Tasks 167-171 (2026-02-06): JSON parse safety, TTS response limit, doc fixes, proxy header trust
- Task 200 (2026-02-07): Auth database schema and auth module (bcrypt, sessions, tokens, CSRF)
- Task 201 (2026-02-07): Auth registration and email verification endpoints (register, verify, resend)
- Task 202 (2026-02-07): Auth login, logout, /me, account lockout (21 tests)
- Task 203 (2026-02-08): Auth magic link endpoints (15 tests)
- Task 204 (2026-02-08): Auth CSRF middleware (15 tests)
- Task 205 (2026-02-08): Auth security headers upgrade (Referrer-Policy, Permissions-Policy, HSTS)
- Task 206 (2026-02-08): WebSocket authentication (session extraction, per-user/IP limits)
- Task 207 (2026-02-08): Frontend login/register pages (auth.ts, login.astro, register.astro, 22 tests)
- Task 208 (2026-02-08): Email verification and magic link pages (verify-email.astro, magic-link.astro, 9 tests)
- Task 209 (2026-02-08): Auth E2E integration tests (17 Playwright tests for auth flows)
- Task 211 (2026-02-08): Interaction logging DB schema (InteractionLog structs, InsertInteraction, 7 tests)
- Task 212 (2026-02-08): STT data capture in processAudio (buildSTTLog, 6 tests)
- Task 213 (2026-02-08): LLM data capture in processAudio (buildLLMLog, 7 tests)
- Task 214 (2026-02-08): TTS and buffer data capture in processAudio
- Task 215 (2026-02-08): Interaction log persistence with defer-save (5 tests)
- Task 216 (2026-02-08): Audio blob retention cleanup (6 tests)
- Task 218 (2026-02-08): Piper TTS feedback for unrecognized concepts (2 tests)
- Task 210 (2026-02-08): Piper TTS welcome greeting on login (3 tests)
- Task 217 (2026-02-08): Interaction logging E2E integration tests (2 real-services tests)
- Task 219 (2026-02-08): Update SECURITY.md for auth system
- Task 220 (2026-02-08): Implement TOTP 2FA validation (stdlib, RFC 6238, 21 handler tests)
- Task 222 (2026-02-08): Document auth API endpoints in API.md
- Task 223 (2026-02-08): io.LimitReader for error body reads + health check body draining
- Task 224 (2026-02-08): Document INTERACTION_LOG_AUDIO and INTERACTION_RETENTION_DAYS env vars
- Task 225 (2026-02-08): Split 9 oversized test files (907→18 files, all under 500 lines)
- Task 226 (2026-02-08): Populate MediaSetID in interaction logs (2 new tests)
- Tasks 227-230 (2026-02-08): Security/quality: io.LimitReader health drains, Permissions-Policy docs, innerHTML→replaceChildren, ebml error logging
- Task 231 (2026-02-08): Frontend CSRF token integration (csrf.ts module, 14 tests, all POST callers updated)
- Task 232 (2026-02-08): Logout UI button + auth.ts logout() (LogoutButton.astro, 5 unit + 2 E2E tests)
- Tasks 235, 238 (2026-02-08): Resend email delivery + docs (.env.example, DEPLOY.md)
- Task 239 (2026-02-08): Account settings page with TOTP management (SettingsButton, settings.astro)
- Task 240 (2026-02-08): E2E tests for settings page (11 Playwright tests, setup-success fix)
- Tasks 233-234 (2026-02-08): Rate limiter IP validation (net.ParseIP, strings.Cut, 8 new tests)
- Tasks 241-242 (2026-02-08): LLM ProcessTranscript test coverage (28 new tests across 2 files)
