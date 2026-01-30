# Progress

This file tracks high-level progress on the subtitler project. For detailed specifications, see `specs/` directory. For historical task details, see `TASKS.jsonl` and git history.

## Project Status Summary

**480 tasks completed** as of 2026-01-30.
Completed tasks archived to `TASKS_archive.jsonl`.

- **Backend:** Go server (52 source files, ~16,100 lines total), 602 tests across 45 files
- **Frontend:** Astro/TypeScript, 832 tests across 29 files
- **E2E:** Playwright tests (71 scenarios across 7 spec files, 58 active + 13 skipped)
- **Total:** 1505+ tests, 32 specification documents

## Feature Summary

### Core
- Video upload (500MB limit) with chunked upload support (>50MB auto-split)
- Whisper transcription integration (whisper-server and whisper-cli)
- Multi-format export: SRT, VTT, JSON (client-side generation)
- Subtitle editor with timing adjustment
- Video subtitle burning (ffmpeg encode or soft subtitles)
- SQLite database with versioned migrations (9 migrations)

### Language & Internationalization
- Language auto-detection (metadata + filename patterns)
- Language override before/after transcription
- Script conversion (romanized to native for 9 Indic languages)
- Lyrics mode with smart alignment and chorus templates
- Embedded subtitle detection and extraction

### User Experience
- Bionic reading mode (bold first portion of words)
- Variable playback speed (0.8x, 0.9x, 1.0x)
- Dark/light/auto theme toggle
- Video thumbnails on My Videos page
- Keyboard shortcuts for video control
- Styled dialog modals (replaced native alert/confirm)
- Kid mode (screen lock) for mobile video playback
- Feedback system with star rating

### Authentication & Security
- Email/password registration and login
- Magic link (passwordless) authentication
- TOTP 2FA with recovery codes
- Session management with revocation
- Password reset via email (Resend API)
- CAPTCHA protection (hCaptcha)
- Rate limiting on all endpoints (configurable)
- File encryption at rest (age library) with key rotation
- Path traversal protection, CSRF protection
- Security event audit logging
- Admin role system

### Infrastructure
- Structured logging (slog) with configurable levels
- Database query profiling (slow query detection)
- Prometheus metrics with path normalization
- Health check endpoint (public + authenticated detail)
- SQLite connection pooling and maintenance scheduler
- HTTP range requests for video streaming
- Graceful shutdown with context cancellation

## Recent Work

### Tasks 477-480: Chunk upload race fix, close error handling, doc updates (2026-01-30)
- **Task 477:** Fixed TOCTOU race condition in chunked upload — `CreateUploadChunk` now uses `INSERT OR IGNORE` and returns `(bool, error)` indicating whether the row was actually inserted. Handler treats concurrent duplicate as idempotent success (cleans up duplicate file, returns 200). Added 2 tests (idempotent + concurrent). Prevents 500 errors from UNIQUE constraint violations during concurrent chunk uploads
- **Task 478:** Updated PROGRESS.md — status summary, E2E count (71→67), removed duplicate pending section, compacted recent work
- **Task 479:** Updated TESTING.md backend count from 600 to 602, clarified E2E count (71 total: 58 active + 13 skipped)
- **Task 480:** Made chunk `destFile.Close()` error fatal — chunks have no downstream validation (unlike regular uploads which use `ValidateVideoFile`), so a close failure could leave corrupted data. Now removes chunk and returns 500 instead of continuing

### Tasks 469-475: Security, memory leaks, accessibility, and UX fixes (2026-01-30)
- Enforced minimum chunk size (1MB), fixed memory leak in speed controls (AbortController), cleared speedIndicatorTimeout on modal close
- Added aria-label to embedded subtitle track select, fixed video modal hiding video on subtitle load failure
- Updated TESTING.md test counts. Wontfix: generateID duplication is acceptable Go design

### Tasks 436-468: Security hardening, bug fixes, performance, and docs (2026-01-30)
- **Security:** Added ownership checks to 13 endpoints (subtitles, transcription, burn, language-hints), validated session_id format, rate-limited burn status
- **Performance:** Fixed N+1 query in video listing, cached HTTPS_ONLY and CAPTCHA_SITE_KEY at startup
- **Bug fixes:** Fixed double WriteHeader in health endpoint, scheduler early-return, feedback timestamp comparison
- **Transactions:** Wrapped DeleteVideo, SaveRecoveryCodes, DeleteUploadSession in DB transactions
- **Docs:** Updated API.md auth requirements, added missing endpoints to RATE_LIMITS.md, fixed API.md register status code

### Tasks 397-435: User-reported bugs, testing, refactoring (2026-01-29—2026-01-30)
- Fixed 6 user-reported bugs (night mode CTA, re-transcribe progress, bionic whitespace, scroll position, feedback modal, speed buttons)
- Added 500+ frontend tests across 15 new test files (498→832 tests, 25→29 files)
- Refactored: split upload.astro (3844→975), videos.astro (1818→311), settings.astro (1663→441)
- Split all 8 files over 1000 lines, split api_test.go (10863→5 files)
- Cross-language file size linter, doc sync linting, double-submit prevention, kid mode, fetch timeouts

### Tasks 376-396: Infrastructure, refactoring, and documentation (2026-01-27—2026-01-29)
- Split backend main.go (6340→207 lines) into 9 handler files, set up golangci-lint
- Added decision tracking to LEARNINGS.md, Python security events query tool, fetch-feedback Python rewrite
- Compacted docs (PROGRESS.md, LEARNINGS.md, TASKS.jsonl), added missing deps to deps.md
- Fixed subtitle feedback buttons, evaluation scripts, 15 E2E tests
- Processed user feedback: filed 19 tasks, restricted playback speeds to 0.8x/0.9x/1.0x

## Key Files

| Purpose | Location |
|---------|----------|
| Main specification | `specs/subtitler.md` |
| Authentication | `specs/auth.md`, `specs/totp.md` |
| Encryption | `specs/encryption.md`, `specs/key-rotation.md` |
| Email service | `specs/email.md` |
| Lyrics alignment | `specs/lyrics-alignment.md` |
| Script conversion | `specs/script-conversion.md` |
| Chunked uploads | `specs/chunked-upload.md` |
| API documentation | `docs/API.md` |
| Environment vars | `docs/ENV.md` |
| Error codes | `docs/ERROR_CODES.md` |
| Rate limits | `docs/RATE_LIMITS.md` |
| Security checklist | `docs/SECURITY_CHECKLIST.md` |
| Security events | `docs/SECURITY_EVENTS.md` |
| Security query tool | `scripts/query-security-events.py` |
| File size linter | `scripts/lint-filesize.py` |
| Prometheus metrics | `docs/METRICS.md` |
| Test documentation | `TESTING.md`, `BROWSER_TESTING.md` |
| Learnings | `LEARNINGS.md` |
| Dependencies | `docs/deps.md` |

## Notes

- Whisper-server configurable via `WHISPER_URL` env var
- Email requires `RESEND_API_KEY` env var
- All tests: `./scripts/verify-all.sh`
- Pre-commit hook: `ln -sf ../../scripts/pre-commit .git/hooks/pre-commit`
