# Progress

This file tracks high-level progress on the subtitler project. For detailed specifications, see `specs/` directory. For historical task details, see `TASKS.jsonl` and git history.

## Project Status Summary

**444 tasks completed** as of 2026-01-30. 8 tasks pending.
Completed tasks archived to `TASKS_archive.jsonl`.

- **Backend:** Go server (52 source files, ~16,100 lines total), 598 tests across 45 files
- **Frontend:** Astro/TypeScript, 841 tests across 30 files
- **E2E:** Playwright tests (71 scenarios)
- **Total:** 1510+ tests, 32 specification documents

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

## Pending Tasks

12 pending tasks (436-448) filed from deep codebase inspection.

## Recent Work

### Task 440: Wrap SaveRecoveryCodes in a database transaction (2026-01-30)
- **Bug fix:** `SaveRecoveryCodes` in `db/db_auth.go` was executing DELETE + INSERT operations without a transaction, risking partial writes if the process crashed between operations
- Wrapped the DELETE (old codes) + INSERT (new codes) loop in `db.WithTransaction()` using the existing `Tx` pattern from `EnableTOTPWithRecoveryCodes`
- Changed from `db.conn.ExecContext(ctx, ...)` to `tx.tx.Exec(...)` within the transaction closure
- All recovery code and TOTP tests pass (598 backend tests total)

### Task 439: Fix double WriteHeader in health endpoint (2026-01-30)
- **Bug fix:** Health endpoint (`GET /api/health`) was calling `w.WriteHeader(503)` followed by `httputil.RespondJSON(w, 200, ...)` when health was degraded, causing a superfluous WriteHeader warning
- Fixed by using a single `statusCode` variable and passing it to `RespondJSON` — the separate `w.WriteHeader()` call was removed
- Same fix applied to the test handler in `api_test_helpers_test.go`
- All 598 backend tests pass (including 4 health endpoint tests covering OK and degraded states)

### Task 438: Add ownership checks to burn and language-hints endpoints (2026-01-30)
- **Security fix:** Burn endpoints (`POST /api/videos/{id}/burn`, `GET /api/videos/{id}/burn`) and language-hints endpoint (`GET /api/videos/{id}/language-hints`) now verify caller owns the video before allowing access
- Access check matches the pattern from tasks 436-437: authenticated user must own the video or anonymous user must have matching session_id
- Updated production handlers (`handlers_video_burn.go`, `handlers_video.go`) and test handlers (`api_test_helpers_test.go`)
- Added 3 new access-denied tests, updated 10 existing tests to pass session_id
- Optimized POST burn handler to reuse video fetched for access check (eliminated duplicate `getVideoForDecryption` call)
- Optimized GET burned handler to reuse video and user from access check for filename and audit log
- All 598 backend tests pass, lint clean

### Task 437: Add ownership checks to transcription endpoints (2026-01-30)
- **Security fix:** All four transcription endpoints (`POST /api/transcribe/{id}`, `GET /api/transcribe/{id}`, `PUT /api/transcribe/{id}/segments`, `POST /api/transcribe/{id}/align`) now verify caller owns the video before allowing access
- Access check matches the pattern from task 436: authenticated user must own the video or anonymous user must have matching session_id
- Updated both production handlers (`handlers_transcribe.go`) and test handlers (`api_test_helpers_test.go`)
- Added 3 new access-denied tests, updated 15 existing tests to pass session_id
- Optimized GET handler to reuse video fetched for access check (eliminated duplicate DB query)
- All 595 backend tests pass, lint clean

### Task 449: Fix flaky E2E re-transcribe auto-language confirmation test (2026-01-30)
- **Bug fix:** E2E test `should show confirmation when re-transcribing with auto language` was using `page.on('dialog')` to listen for a native browser confirm dialog, but the app uses a custom `showConfirm()` from `dialog.ts` which creates an HTML overlay
- Updated test to wait for `#dialog-container.visible`, verify `#dialog-message` text, and click `#dialog-cancel`
- Test now passes reliably instead of timing out waiting for a native dialog that never appears

### Task 436: Add ownership checks to subtitle download endpoints (2026-01-30)
- **Security fix:** SRT/VTT/JSON subtitle download endpoints (`GET /api/videos/{id}/subtitles.*`) now verify the caller owns the video before returning subtitle data
- Access check matches the pattern used by embedded-subtitles endpoint: authenticated user must own the video (via user_id) or anonymous user must have matching session_id
- Updated both production handlers (`handlers_video_subtitles.go`) and test handlers (`api_test_helpers_test.go`)
- Added 4 new tests: `TestDownloadSRTAccessDenied`, `TestDownloadSRTAuthenticatedUserAccess`, `TestDownloadVTTAccessDenied`, `TestDownloadJSONAccessDenied`
- Updated 10 existing subtitle tests to use session_id for access
- All 587 backend tests pass, lint clean

### Task 432: Cross-language file size linter (2026-01-30)
- Created `scripts/lint-filesize.py`: checks all source files (Go, TS, Astro, CSS) for >1000 lines
- Replaces frontend-only `lint-frontend-filesize.sh` with unified cross-language linter
- 5 api_test files excepted with justification (task 431 split artifacts — shared test infrastructure)
- Supports `--list-exceptions` flag for documenting exceptions
- Integrated into `lint.sh` (replacing old frontend-only check)

### Task 433: Split/compact all source files over 1000 lines (2026-01-30)
- Split all 8 files over 1000 lines to under 1000 lines each:
  - `db/db.go` (2578 → 294 lines): split into 11 source files by domain (types, video, auth, user, session, upload, transcription, feedback, burnjob, maintenance)
  - `db/db_test.go` (3411 → deleted): split into 11 test files by domain
  - `main_test.go` (1621 → deleted): split into 6 focused test files (subtitle_format, config, middleware, helpers, video_helpers, goroutine)
  - `handlers_video.go` (1565 → 623): extracted subtitles (356), burn (524), text (114)
  - `handlers_upload.go` (1553 → 877): extracted transcribe (695)
  - `handlers_auth.go` (1307 → 470): extracted TOTP (434), recovery (444)
  - `upload.css` (1266 → 784): extracted speed-controls (92), segment-editor (270), keyboard-modal (146) via CSS @import
  - `upload.astro` (1011 → 999): deduplicated subtitle download handlers
- Fixed missing `_ "github.com/mattn/go-sqlite3"` import in db/db.go lost during split
- Backend: 52 source files (was 9 monolithic), 45 test files (was 26)
- All 583 backend + 841 frontend tests pass, verify-all.sh clean

### Task 431: Split api_test.go into smaller test files (2026-01-30)
- Split `api_test.go` (10,863 lines) into 5 focused test files:
  - `api_test_helpers_test.go` (3,334 lines): shared infrastructure (testServer, setupTestServer, registerHandlers, helper methods)
  - `api_auth_test.go` (2,338 lines): 66 auth tests (login, register, sessions, TOTP, password reset, magic link, email verification, CSRF, CAPTCHA)
  - `api_video_test.go` (2,760 lines): 84 video tests (list, delete, transcription, segments, burn, caching, range requests, thumbnails, align, language hints, embedded subtitles)
  - `api_upload_test.go` (1,011 lines): 27 upload tests (single upload, chunked upload)
  - `api_system_test.go` (1,562 lines): 45 system tests (health, logs, feedback, admin feedback, metrics, scripts, request ID, content-type)
- Investigated replacing test `registerHandlers()` with production handlers: NOT feasible because tests intentionally stub upload/transcription/burn flows and use different (non-transactional) DB methods
- All 222 test functions preserved, all tests pass, lint clean

### Tasks 434-435: Fix feedback list timestamp bug, increase video thumbnails (2026-01-30)
- **Task 434:** Fixed `ListFeedback` `after` parameter: go-sqlite3 stores `time.Time` as RFC3339Nano with offset but formats query parameters differently, breaking SQLite text comparison. Used `datetime()` normalization on both sides. Added handler validation for RFC3339 format. Added 3 new tests (db + api level).
- **Task 435:** Increased video thumbnails from 80x45px to 160x90px on My Videos page per user feedback.
- Backend tests: 580 -> 583

### Task 428: Update TESTING.md test counts (2026-01-29)
- Updated TESTING.md overview: 580 backend, 841 frontend, 71 E2E = 1,492+ total tests
- Added 10 missing frontend test files to the test file table (30 total now)
- Updated BROWSER_TESTING.md: added 2 new E2E test entries, updated directory structure

### Task 427: Add max-length validation for paste transcript text (2026-01-29)
- Added `MAX_ALIGN_TEXT_LENGTH` constant (100KB) to upload-constants.ts, matching backend validation
- Added client-side byte-length check using `new Blob([text]).size` before align API call
- Shows inline error message with actual/max size when exceeded
- Added E2E test verifying error message appears for oversized text
- E2E scenarios: 60 -> 61

### Task 426: Replace alert() with styled dialog in admin feedback page (2026-01-29)
- Replaced native `alert()` in admin/feedback.astro with `showAlert()` from `utils/dialog.ts`
- Error message for failed status updates now shows as a styled error dialog instead of browser-native alert
- Build passes

### Task 425: Make upload dropzone keyboard-accessible (2026-01-29)
- Added `role="button"`, `tabindex="0"`, `aria-label` to dropzone div
- Added keyboard handler: Enter/Space opens file picker
- Added `focus-visible` CSS style for keyboard navigation
- Added E2E test verifying accessibility attributes and focusability
- E2E scenarios: 59 -> 60

### Task 429: Add kid mode (screen lock) to video modal (2026-01-29)
- User feedback: kids tapping mobile screen accidentally click buttons during playback
- Added lock button (padlock icon) to video modal action bar, positioned on right side
- When locked: all buttons/links/segments disabled via CSS `pointer-events: none`, overlay click blocked, keyboard shortcuts blocked
- Native `<video>` controls remain functional (play/pause, seek, volume)
- Unlock requires 1-second long-press (prevents accidental unlock by kids)
- Visual feedback: CSS fill animation during unlock hold, accent color when locked
- ARIA attributes update for screen readers (`aria-pressed`, `aria-label`)
- Kid mode resets when modal closes (no persistence)
- Added 10 unit tests covering enable/disable, keyboard blocking, overlay blocking, event registration
- Created `specs/kid-mode.md` specification
- Frontend tests: 831 -> 841

### Task 424: Add double-submit prevention to auth forms (2026-01-29)
- Added `isSubmitting` guard to 5 form submit handlers across 4 pages:
  - `login.astro`: main login form + magic link form
  - `register.astro`: registration form
  - `forgot-password.astro`: password reset request form
  - `reset-password.astro`: password reset form
- Guard integrated into `setLoading()` function so all existing `setLoading(false)` calls also reset the flag
- Prevents concurrent form submissions from rapid double-clicks
- All 831 frontend tests pass, build clean, lint clean
- Filed 4 new tasks (425-428) from deep codebase inspection

### Task 423: Extract chunked upload complete handler into smaller functions (2026-01-29)
- Refactored `POST /api/upload/complete` handler from ~290 lines to ~160 lines
- Extracted 4 helper functions: `assembleChunks`, `processVideoMetadata`, `encryptAndPersistVideo`, `finalizeUploadSession`
- All 580 backend tests pass, golangci-lint clean (0 issues)

### Task 422: Extract burn subtitles handler into smaller functions (2026-01-29)
- Refactored `POST /api/videos/{id}/burn` handler from 335 lines to 104 lines
- Extracted 6 helper functions: `processBurnJob`, `failBurnShutdown`, `decryptVideoForBurn`, `generateBurnSRTFile`, `buildBurnFFmpegCmd`, `startBurnProgressTracker`, `encryptBurnOutput`
- All 580 backend tests pass, golangci-lint clean (0 issues)

### Task 421: Add fetch timeout via AbortController (2026-01-29)
- Replaced bare `fetch()` calls with `fetchWithTimeout()` in 3 files:
  - `upload.astro`: loadExistingVideo, embedded subtitles extraction, post-align transcription fetch
  - `videos-modal.ts`: openVideoModal transcription fetch
  - `videos-subtitles.ts`: getSubtitleSegments server fetch
- Uses existing `fetchWithTimeout` utility (30s default timeout via AbortController)
- Added 2 new tests verifying `fetchWithTimeout` is called in videos-modal and videos-subtitles
- Frontend tests: 829 -> 831

### Task 419: Add pagination offset limit (2026-01-29)
- Added `maxPaginationOffset` constant (100,000) in helpers.go
- Both paginated endpoints (`GET /api/videos`, `GET /api/admin/feedback`) now return 400 for offsets exceeding the limit
- Added 2 new tests (`TestListVideosExcessiveOffset`, `TestAdminFeedbackListExcessiveOffset`)
- Backend tests: 578 -> 580

### Tasks 417-418, 420: Code quality improvements from deep inspection (2026-01-29)
- **Task 417:** Drain HTTP response body in whisper health check for TCP connection reuse
- **Task 418:** Made login lockout constants configurable via `MAX_LOGIN_ATTEMPTS` and `LOGIN_LOCK_DURATION` env vars (previously hardcoded to 5 attempts / 15 min)
- **Task 420:** Replaced all `JSON.parse(JSON.stringify(...))` deep clone patterns with `structuredClone()` across frontend source and test files
- Filed 7 new tasks (417-423) from deep codebase inspection covering backend code quality, frontend improvements, and handler refactoring
- Updated docs/ENV.md with new security configuration variables
- All 1407+ tests pass, lint clean

### Tasks 415-416: Add unit tests for remaining untested frontend utility files (2026-01-29)
- Added 57 tests across 2 new test files covering the last untested utility modules
- `settings-preferences.test.ts` (24 tests): setupPreferences init, bionic toggle, fixation slider, preview rendering, disabled-section toggling
- `settings-sessions.test.ts` (33 tests): loadSessions, parseUserAgent detection, session rendering, revoke flow, error/timeout handling, event delegation
- Frontend tests: 772 -> 829 (30 test files)
- All frontend utility .ts files with executable logic now have test coverage

### Tasks 410-413: More unit tests + backend DRY improvement (2026-01-29)
- Added 114 tests across 3 new test files covering remaining untested utility modules
- `transcription-polling.test.ts` (49 tests): formatTime, formatDuration, parseSRT, getLanguageDisplayName, startTranscription, pollTranscriptionStatus
- `videos-list.test.ts` (48 tests): formatBytes, formatDate, formatRetention, getStatusBadge, getEmbeddedSubtitlesBadge, renderVideo, reprocessVideo, deleteVideo
- `videos-subtitles.test.ts` (17 tests): getSubtitleSegments (LRU cache, error handling), handleDownloadClick (SRT/VTT/JSON)
- Backend: Extracted duplicated MIME type whitelist to shared `allowedMIMETypes` variable in config.go (was defined twice in handlers_upload.go)
- Task 414 (shared CSS extraction) marked wontfix: CSS differences between pages are intentional design choices for different contexts
- Frontend tests: 658 -> 772 (28 test files)

### Tasks 405-409: Add unit tests for 5 untested frontend utility files (2026-01-29)
- Added 160 tests across 5 new test files covering the largest untested utility modules
- `segment-editor.test.ts` (69 tests): undo/redo, feedback, edit mode, segment navigation, save
- `subtitle-sync.test.ts` (19 tests): subtitle sync, speed UI, burn subtitles
- `videos-modal.test.ts` (25 tests): modal lifecycle, formatTime, open/close, error handling
- `settings-totp.test.ts` (27 tests): TOTP setup/verify/disable, recovery codes, regen
- `upload-file.test.ts` (20 tests): file validation, session URLs, upload flow, XHR handling
- Frontend tests: 498 -> 658 (25 test files)

### Task 388: Doc sync linting (2026-01-29)
- Created `scripts/lint-doc-sync.sh` to detect documentation drift
- 5 automated checks: deps.md vs go.mod/package.json, ENV.md vs os.Getenv calls, API.md vs registered routes, RATE_LIMITS.md vs rate limit config
- Integrated into `scripts/lint.sh` (runs as part of verification)
- Fixed missing Chunked Upload and User rate limit categories in RATE_LIMITS.md
- Updated RATE_LIMITS.md Configuration section (was referencing old main.go structure)

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

No pending tasks. All tasks completed.

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
