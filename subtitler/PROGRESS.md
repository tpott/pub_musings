# Progress

This file tracks high-level progress on the subtitler project. For detailed specifications, see `specs/` directory. For historical task details, see `TASKS.jsonl` and git history.

## Project Status Summary

**402 tasks completed** as of 2026-01-29. All core features implemented and tested. 1 task pending.
Completed tasks archived to `TASKS_archive.jsonl`.

- **Backend:** Go server (9 source files, ~6500 lines total), 578 tests across 26 files
- **Frontend:** Astro/TypeScript, 498 tests across 20 files
- **E2E:** Playwright tests (59 scenarios)
- **Total:** 1080+ tests, 31 specification documents

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

### Task 404: Refactor settings.astro into smaller files (2026-01-29)
- Split settings.astro from 1663 lines to 441 lines (under 1000 target)
- Extracted CSS to `styles/settings.css` (623 lines)
- Extracted TOTP setup/verify/disable/recovery code logic to `utils/settings-totp.ts` (458 lines)
- Extracted session management to `utils/settings-sessions.ts` (133 lines)
- Extracted bionic reading preferences to `utils/settings-preferences.ts` (61 lines)
- All 1080+ tests pass, build clean, lint clean

### Task 403: Refactor videos.astro into smaller files (2026-01-29)
- Split videos.astro from 1818 lines to 311 lines (under 1000 target)
- Extracted CSS to `styles/videos.css` (702 lines)
- Extracted video list rendering and operations to `utils/videos-list.ts` (248 lines)
- Extracted modal playback, subtitle sync, speed controls to `utils/videos-modal.ts` (474 lines)
- Extracted subtitle caching and download helpers to `utils/videos-subtitles.ts` (99 lines)
- All 1080+ tests pass, build clean, lint clean

### Tasks 397-402: Fix 6 user-reported bugs from FEEDBACK.md (2026-01-29)
- **Task 397:** Fixed night mode CTA text unreadable on home page
- **Task 398:** Fixed re-transcribe button showing no progress feedback
- **Task 399:** Fixed bionic reading whitespace collapse in upload page
- **Task 400:** Fixed subtitle segments scrolling too far ahead on videos page
- **Task 401:** Fixed feedback modal interfering with video playback
- **Task 402:** Fixed playback speed not applying on videos page

### Task 384: Add decision tracking process to LEARNINGS.md (2026-01-29)
- Added "Decisions" section to LEARNINGS.md with Context/Options/Decision/Outcome format
- Documented 6 key decisions: SQLite, Astro, age encryption, file size linting, fetch-feedback Python rewrite, Resend email
- Updated CLAUDE.md with "Decision Tracking" section instructing future Ralphs to document decisions
- Added decision format template alongside existing lesson format

### Task 396: Split upload.astro into smaller files (2026-01-29)
- Split `upload.astro` from 3844 lines to 975 lines (75% reduction)
- Extracted 5 TypeScript utility modules using state objects + callbacks pattern:
  - `upload-constants.ts` (100 lines): Language maps, localStorage keys, upload config
  - `upload-file.ts` (352 lines): File upload with chunked resume support
  - `transcription-polling.ts` (217 lines): Polling, ETA, SRT parsing, time formatting
  - `segment-editor.ts` (510 lines): Editing, undo/redo, feedback, rendering
  - `subtitle-sync.ts` (497 lines): Video sync, speed controls, burn, keyboard shortcuts
- Extracted CSS to `styles/upload.css` (1256 lines) imported via Astro frontmatter
- Fixed bug: `updateUnsavedIndicator()` call replaced with correct `markUnsaved()` pattern
- All 1076+ tests pass, file size lint clean (upload.astro no longer in warnings)

### Task 379: Frontend file size linting (2026-01-29)
- Researched options: ESLint+eslint-plugin-astro (4+ deps for 1 rule), Biome (no .astro support), shell script (zero deps)
- Chose shell script approach per dependency policy
- Created `scripts/lint-frontend-filesize.sh`: error at 4000 lines, warn at 1000 lines, test files excluded
- Integrated into `scripts/lint.sh` as new "Checking Frontend File Sizes" step
- 3 files flagged as warnings: upload.astro (3844), videos.astro (1811), settings.astro (1663)
- Filed Task 396 to split upload.astro
- Added LEARNINGS.md entry documenting decision rationale

### Task 377: Set up golangci-lint with file length linter (2026-01-29)
- Installed golangci-lint v2.8.0, created `backend/.golangci.yml` (v2 format)
- Enabled linters: `funlen` (max 150 lines/function), `revive` with `file-length-limit` (max 2600 lines/file)
- Added `//nolint:funlen` to 5 route-table registration functions (`register*Handlers`)
- Test files excluded from both linters
- Updated `scripts/lint.sh` to run golangci-lint when available (graceful fallback)
- Updated `LINTERS.md` to mark golangci-lint as IN USE
- Added golangci-lint to `docs/deps.md` Development Tools section
- All 1076+ tests pass, lint clean

### Task 378: Split main.go into smaller files (2026-01-29)
- Split `backend/main.go` from 6340 lines to 207 lines (9 files total)
- New files: `config.go` (334), `globals.go` (125), `helpers.go` (704), `scheduler.go` (268), `handlers_system.go` (462), `handlers_auth.go` (1310), `handlers_upload.go` (1546), `handlers_video.go` (1543)
- Handlers extracted via `registerXxxHandlers(mux)` pattern
- All 578 backend tests pass, full verification passes

### Task 389: Create Python security events query tool (2026-01-29)
- Created `scripts/query-security-events.py` for filtering/analyzing security events
- Parses slog logfmt output from journalctl stdin
- Filters: `--event` (prefix match), `--ip`, `--user` (email/user_id), `--level`, `--since`/`--until`
- Output: `--format short|raw`, `--count-by FIELD` for aggregation, `--tail N`
- Documented in `docs/SECURITY_EVENTS.md` with usage examples

### Task 394: Add --auto-fetch-feedback-script CLI arg to ralph.py (2026-01-29)
- Added `--auto-fetch-feedback-script <path>` CLI argument to `ralph.py`
- `fetch_feedback()` now accepts optional `script_path` parameter
- When flag is omitted, uses default `scripts/fetch-feedback.py`
- Added 5 tests for the new functionality in `test_ralph.py`
- Updated `specs/feedback-fetch.md` Integration section

### Task 393: Rewrite fetch-feedback.sh in Python (2026-01-29)
- Replaced `scripts/fetch-feedback.sh` with `scripts/fetch-feedback.py`
- Secrets resolution: env vars > `.env` file > `secrets.enc.yaml` via sops
- Uses only stdlib (`urllib`, `json`, `subprocess`) - no new dependencies
- `ralph.py` updated to call Python script via `sys.executable`
- Updated specs/feedback-fetch.md, AGENTS.md, .gitignore

### Task 387: Add Missing Dependencies to deps.md (2026-01-29)
- Added CDN Scripts section (hCaptcha widget)
- Added System Dependencies section (ffmpeg, ffprobe, whisper-cli, whisper-server, sqlite3)
- Added External Services section (Resend, hCaptcha API)
- Added Deployment Dependencies section (Caddy, Cloudflare Tunnel, sops, systemd, fail2ban)
- Added Build-Time Dependencies section (Go, Node.js/npm, Noto fonts)

### Tasks 376, 380, 381: Documentation Compaction (2026-01-29)
- PROGRESS.md compacted from 1492 to ~130 lines (kept summary, features, recent work, key files)
- LEARNINGS.md compacted from 1099 to ~550 lines, sorted by category (Backend, Frontend, Security, Infrastructure, Process)
- TASKS.jsonl compacted from 395 to 13 lines; 383 completed tasks archived to TASKS_archive.jsonl

### Task 390: Verify and Fix Evaluation Scripts (2026-01-29)
- Fixed `evaluate.py` to defer imports so `--help` and `--list` work without dependencies
- Updated `evaluation/README.md` with Prerequisites, Limitations sections
- Added `evaluation/venv/` to `.gitignore`

### Task 391: Fix Subtitle Feedback Buttons (2026-01-28)
- Moved feedback buttons from every segment to current subtitle display only
- Persistent click handler on feedback container (not per-segment listeners)
- Backend integration documented as pending

### Task 395: E2E Test Fixes (2026-01-27)
- Fixed 15 of 16 failing Playwright E2E tests
- Root causes: cookie consent blocking, timing races, route pattern mismatches, schema nullability
- Added retry mechanism for local E2E runs

### Tasks 382-385, 392: Feedback Processing (2026-01-27)
- Processed FEEDBACK.md: filed 19 new tasks (376-394)
- Restricted playback speeds to 0.8x, 0.9x, 1.0x only (recurring issue)
- Added Settings link to all pages for authenticated users
- Added E2E tests to pre-commit hook

## Pending Tasks

| ID | Name |
|----|------|
| 388 | Research doc sync linting for docs/ files |

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
| Prometheus metrics | `docs/METRICS.md` |
| Test documentation | `TESTING.md`, `BROWSER_TESTING.md` |
| Learnings | `LEARNINGS.md` |
| Dependencies | `docs/deps.md` |

## Notes

- Whisper-server configurable via `WHISPER_URL` env var
- Email requires `RESEND_API_KEY` env var
- All tests: `./scripts/verify-all.sh`
- Pre-commit hook: `ln -sf ../../scripts/pre-commit .git/hooks/pre-commit`
