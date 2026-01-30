# Progress

This file tracks high-level progress on the subtitler project. For detailed specifications, see `specs/` directory. For historical task details, see `TASKS.jsonl` and git history.

## Project Status Summary

**500 tasks completed** as of 2026-01-30.
Completed tasks archived to `TASKS_archive.jsonl`.

- **Backend:** Go server (52 source files, ~16,100 lines total), 602 tests across 45 files
- **Frontend:** Astro/TypeScript, 875 tests across 32 files
- **E2E:** Playwright tests (71 scenarios across 7 spec files, 66 active + 5 permanently skipped)
- **Total:** 1,548 tests, 32 specification documents

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

### Tasks 497-500: Deep inspection fixes (2026-01-30)
- **Task 497:** Fixed `EncryptFile` and `DecryptToFile` in crypto/crypto.go ignoring `dst.Close()` errors on write files. Both now check close error, remove partial/corrupted file, and return error. Consistent with close-error patterns from tasks 480, 490
- **Task 498:** Fixed TESTING.md — E2E breakdown updated to 66 active / 5 permanently skipped (was 58/13). Added 3 missing frontend test files to table (videos-progress, retranscribe-progress, upload-collapsible)
- **Task 499:** Added `fetchWithTimeout` (15s timeout) to `videos-progress.ts` polling fetch. Previously used bare `fetch()` which could hang indefinitely. Tests updated with mock
- **Task 500:** Added `fetchWithTimeout` to `videos.astro` video list fetch. Previously used bare `fetch()`. Build verified
- Filed tasks 501-504: TypeScript `any` cleanup, scheduler tests, PROGRESS.md update

### Task 494: Re-transcribe progress box with real-time updates (2026-01-30)
- Added progress box HTML/CSS to upload page — shows bar, percentage, and status message during re-transcription
- Progress box animates closed (collapse transition) after 1.5s on completion, hides immediately on error
- Extracted `retranscribe-progress.ts` utility (show/update/hide/collapse functions) and `upload-collapsible.ts` (section toggle with localStorage persistence)
- Kept `upload.astro` at 1000 lines (within lint limit) by extracting collapsible logic
- Added 17 tests: 9 for retranscribe-progress, 8 for upload-collapsible. 875 frontend tests across 32 files pass

### Task 495: My Videos page — real-time transcription progress (2026-01-30)
- Added `videos-progress.ts` utility that polls `GET /api/transcribe/{id}` for processing/pending videos
- Shows compact progress bar (4px height, max 200px wide) with status text and percentage in each video card
- Polls every 3 seconds, stops on completion/error. Handles 403/404 (permanent stop) vs 500/network (retry)
- When transcription completes, re-renders the video card in-place with new buttons (View, SRT, VTT, JSON)
- Supports both authenticated and anonymous users (includes session_id for anonymous)
- Added 14 tests in `videos-progress.test.ts`. 858 frontend tests across 30 files pass

### Task 496: My Videos page — larger thumbnails, less button prominence (2026-01-30)
- Restructured video card layout: thumbnail (240x135, up from 160x90) and info in a `video-top` row, action buttons in a compact secondary row below
- Download buttons (SRT, VTT, JSON) changed from full `btn-secondary` buttons to subtle text-style `btn-link` links
- Delete button changed from prominent red `btn-delete` to compact text-only `btn-delete-sm` pushed right with `margin-left: auto`
- Actions row separated from content by a subtle `border-top` divider
- Mobile responsive: thumbnail goes full-width, actions wrap naturally
- Added 2 tests (video-top structure, btn-link class usage). 844 frontend tests pass

### Tasks 494-496: Filed from user feedback (2026-01-30)
- **Task 495:** My Videos page should show real-time transcription progress for processing videos (pending)

### Tasks 491-493: Error handling fixes — swallowed DB errors (2026-01-30)
- **Task 491:** Fixed transcribe handler swallowing `GetTranscription()` DB error — previously logged error but continued, risking duplicate transcription starts or missing a "processing" state. Now returns 500
- **Task 492:** Fixed burn status handler silently discarding `GetTranscription()` error with `_` — now logs warning so DB issues are visible in logs. Duration/ETA still gracefully omitted on failure
- **Task 493:** Fixed `finalizeUploadSession()` swallowing critical DB errors — function now returns error when transcription record creation fails. Caller returns 500 instead of reporting upload success when transcription initialization failed. Session status and chunk cleanup remain best-effort

### Tasks 486-488: Spec, schema, and API doc drift fixes (2026-01-30)
- **Task 486:** Fixed TOTP spec/schema drift — added `qr_code` to setup response in specs/totp.md, added `recovery_codes` and recovery code generation steps to verify response in spec, added `uri` and `issuer` optional fields to `TotpSetupResponseSchema`. Added test for full backend response shape
- **Task 487:** Fixed 5 Zod schema mismatches: SessionSchema `last_used_at` → `expires_at`, LanguageHintsResponseSchema `recommended` → `suggested_language`/`suggested_confidence`, LanguageHintSchema added `language_name`/`raw_value`, UploadCompleteResponseSchema `id` → `upload_id` and `language_hints` from array to DetectionResult object, UserSchema added `email_verified`. Added 7 tests
- **Task 488:** Fixed API.md register response — showed full `user` object with `email_verified` field matching backend. Removed false "email disabled returns token" section (never implemented in code)
- **Task 489:** Fixed frontend type drift — added `embedded_subtitles` to `TranscriptionStatusResponseSchema`, `duration`/`estimated_remaining_seconds` to `BurnStatusResponse`, `conversion_failed_indices` to `AlignmentResponse`. Added test
- **Task 490:** Made `assembleChunks` `destFile.Close()` error fatal — removes partial file and returns error instead of silently continuing with potentially corrupted assembled file (same pattern as Task 480 for chunk uploads)

### Tasks 483-485: User-reported bugs from FEEDBACK.md (2026-01-30)
- **Task 483:** Fixed TOTP 2FA enable returning "API response validation failed" — frontend Zod schema expected `{success: boolean}` but backend returned `{message, totp_enabled, recovery_codes}`. Updated `TotpVerifyResponseSchema` to match backend. Added regression test
- **Task 484:** Fixed session IP showing 127.0.0.1 behind cloudflared — `GetClientIP()` now checks `CF-Connecting-IP` header first (Cloudflare tunnel sets this), before `X-Forwarded-For` and `X-Real-IP`. This also fixes security events recording localhost IPs. Added 3 tests (CF-Connecting-IP trusted/untrusted/precedence)
- **Task 485:** Added journalctl troubleshooting section to SECURITY_EVENTS.md (6 diagnostic steps). Added `StandardOutput=journal`, `StandardError=journal`, `SyslogIdentifier=subtitler` to deployment.md systemd service file

### Tasks 481-482: Error handling and caching fixes (2026-01-30)
- **Task 481:** Fixed burn job handler swallowing `GetBurnJob()` DB error — previously logged error but continued, potentially creating duplicate burn jobs. Now returns 500 on DB failure
- **Task 482:** Fixed subtitle ETag using only first 100 chars of SegmentsJSON — editing segments beyond position 100 wouldn't invalidate cache. Now hashes full SegmentsJSON for reliable cache invalidation

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
