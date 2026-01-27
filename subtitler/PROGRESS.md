# Progress

This file tracks high-level progress on the subtitler project. For detailed specifications, see `specs/` directory.

## Project Status Summary

**204 tasks completed** as of 2026-01-27. All core features implemented and tested.

## Feature Summary

### Core Features (Tasks 1-10)
- ✅ Project scaffolding (Astro frontend + Go backend)
- ✅ Video upload with progress tracking (500MB limit)
- ✅ Whisper transcription integration (whisper-server API)
- ✅ SRT file generation and download
- ✅ Subtitle viewer with synced playback
- ✅ SQLite database for persistence
- ✅ File encryption at rest using age library
- ✅ Video listing page

### Authentication (Tasks 11-18, 37-42, 53, 59, 66, 82, 101)
- ✅ Email/password registration and login
- ✅ Magic link (passwordless) authentication (Task 101)
- ✅ Email verification required before first login
- ✅ Session management with cookie-based auth
- ✅ TOTP 2FA with QR codes (self-hosted generation)
- ✅ Recovery codes (10 single-use codes)
- ✅ Password reset via email (Resend API)
- ✅ Rate limiting on auth endpoints (IP + per-email)
- ✅ Session listing and revocation
- ✅ CAPTCHA protection via hCaptcha (Task 118)
- See: `specs/auth.md`, `specs/totp.md`, `specs/email.md`

### Upload Limits & Retention (Tasks 13-14, 61, 116)
- ✅ Anonymous users: 2 uploads, 48-hour retention
- ✅ Registered users: unlimited uploads, 90-day retention
- ✅ Auto-delete scheduler for expired files
- ✅ Retention countdown display in UI
- ✅ Chunked uploads for large videos (Task 116)
  - Files >50MB automatically split into 50MB chunks
  - Resumable uploads with localStorage tracking
  - Works through Cloudflare's 100MB request limit
  - See: `specs/chunked-upload.md`

### Subtitle Features (Tasks 15-17, 44, 51, 56-57, 69-70, 85, 102)
- ✅ Subtitle editor (text and timing)
- ✅ Transcript paste-and-align with Whisper timing
- ✅ Lyrics mode with music-specific alignment
- ✅ Chorus template timing for repeated sections
- ✅ Script conversion (romanized → native scripts for 9 Indic languages)
- ✅ Subtitle burning into video (ffmpeg)
- ✅ Multiple export formats: SRT, VTT, JSON
- ✅ Language override for transcription (Task 102)
- ✅ Language selection UI for transcription (Task 105)
- See: `specs/lyrics-alignment.md`, `specs/script-conversion.md`

### Testing & Quality (Tasks 19-25, 30, 49-50, 80, 94)
- ✅ Backend Go tests with coverage
- ✅ Frontend Vitest tests
- ✅ E2E Playwright tests (auth, upload flow)
- ✅ Verification scripts and pre-commit hook
- ✅ Form validation with inline errors
- ✅ TypeScript interfaces for type safety
- See: `TESTING.md`, `BROWSER_TESTING.md`

### Documentation (Tasks 20-23, 26, 31-36, 54, 67, 78, 104, 110)
- ✅ TESTING.md, INSTALL.md, LINTERS.md, BROWSER_TESTING.md
- ✅ API documentation in docs/API.md
- ✅ Rate limits reference in docs/RATE_LIMITS.md
- ✅ Environment variable reference in docs/ENV.md (Task 110)
- ✅ Feature specs in specs/ directory
- ✅ Deployment planning docs
- ✅ Production debugging guide (Task 104)

### Security & Infrastructure (Tasks 52, 60, 63-64, 68, 72, 74-75, 79, 83, 87-89, 93, 95, 97, 99)
- ✅ Rate limiting on all sensitive endpoints
- ✅ MIME type validation on uploads
- ✅ Secure cookie flag for HTTPS
- ✅ Request ID tracing for debugging
- ✅ Enhanced health check (DB, Whisper, disk)
- ✅ Database transactions for critical operations
- ✅ Retry logic for whisper-server
- ✅ HTTP caching headers for video/subtitle endpoints
- ✅ CSRF protection for state-changing endpoints
- ✅ Proper error handling for cryptographic random generation
- ✅ Configurable paths via environment variables (Task 88)
- ✅ X-Forwarded-For trust validation (Task 89)
- ✅ Encryption toggle via environment variable (Task 93)
- ✅ Configurable rate limits via environment variables (Task 95)
- ✅ HTTP Range requests for efficient video streaming (Task 97)
- ✅ SQLite database maintenance scheduler (VACUUM + ANALYZE) (Task 98)
- ✅ Fixed privacy vulnerability in My Videos page (Task 99)

### UI/UX (Tasks 65, 71, 76-77, 90-91, 96, 100, 103, 111)
- ✅ Upload progress bar with real bytes/percentage
- ✅ Accessibility improvements (ARIA labels, keyboard nav)
- ✅ Mobile responsive design
- ✅ Dark mode with toggle and localStorage persistence
- ✅ Time input validation with max 24 hours and inline errors
- ✅ Event listener cleanup to prevent memory leaks
- ✅ Namespaced localStorage keys (Task 96)
- ✅ Cookie consent banner (Task 100)
- ✅ Video player modal on My Videos page (Task 103)
- ✅ Video thumbnails on My Videos page (Task 111)

### Video Management (Tasks 73)
- ✅ Video reprocessing for failed transcriptions

### Research (Task 48)
- ✅ MoltenVK/qemu GPU passthrough research completed
- **Conclusion:** Not recommended for production - macOS/MoltenVK lacks DMA buffer support needed by Venus
- Current host-based whisper-server approach remains the best option
- See: `specs/metal-moltenvk.md`

### Security Hardening (Tasks 106-108, 114)
- ✅ Account lockout after failed login attempts (Task 106) - already implemented, documented
- ✅ Video file validation with ffprobe (Task 107)
- ✅ Content Security Policy headers (Task 108)
- ✅ Authenticated health check (Task 114) - public requests get minimal response, auth required for details

### Technical Infrastructure (Tasks 109, 113, 120)
- ✅ Database migration system (Task 109)
  - Migration files in `backend/db/migrations/` as versioned .sql files
  - Schema version tracked in `schema_migrations` table
  - CLI tool: `go run ./cmd/migrate [up|down|status|version]`
  - Automatic upgrade from pre-migration databases
- ✅ Structured logging with slog (Task 113)
  - Package: `backend/logging/` using Go's standard `log/slog`
  - Log levels: Debug, Info, Warn, Error, Fatal
  - Context-aware logging with request_id, user_id, video_id, session_id
  - Configure via `LOG_LEVEL` env var (debug, info, warn, error)
- ✅ Encryption key rotation support (Task 120)
  - Multi-key encryptor supports multiple key versions
  - Videos track which key version encrypted them
  - CLI tool: `go run ./cmd/rotate-keys [status|rotate|reencrypt]`
  - Zero-downtime key rotation with gradual re-encryption
  - See: `specs/key-rotation.md`, `specs/encryption.md`

## Current Work

**207 tasks completed.** Fifth deep inspection (2026-01-27) identified improvements; user feedback filed 14 new tasks (211-224).

### Recently Completed (2026-01-27)
- Task 205: SECURITY - Sanitize whisper error messages before sending to client
  - Modified `dbTranscriptionToStatus()` to sanitize error messages
  - When status is "error" and verbose mode is off, returns user-friendly message
  - Raw error (containing paths, IPs, port numbers) is only shown in verbose mode
  - Uses existing `errmsg` package for consistent sanitization
  - Added `TestTranscriptionErrorMessageSanitization` and `TestTranscriptionErrorVerboseMode` tests
  - Prevents leaking: IP addresses, paths, port numbers, internal service names

- Task 204: Add missing environment variables to ENV.md
  - Added `CHUNK_SIZE` for configuring chunked upload size (default 50MB)
  - Added `UPLOAD_SESSION_EXPIRY` for chunked upload session timeout (default 24h)
  - Added `KEYS_DIR` for multi-key encryption directory (default data/keys)
  - Added `CHUNK_RATE_LIMIT` for chunked upload rate limiting (default 60/min)
  - Added `LOG_LEVEL` for structured logging configuration (debug/info/warn/error)
  - Added new "Chunked Upload Configuration" section
  - Updated Quick Reference table with all new variables

- Task 203: Add missing endpoints to API.md
  - Added GET /metrics endpoint documentation with API key and admin auth details
  - Added GET /api/captcha/config endpoint documentation
  - Added GET /api/auth/verify and POST /api/auth/resend-verification email verification endpoints
  - Added GET /api/videos/{id}/thumbnail endpoint documentation
  - Updated table of contents with new sections (Metrics, CAPTCHA, Email Verification, Thumbnails)

- Task 217: BUG - Fix Re-transcribe button not working for language changes
  - Root cause: Backend POST /api/transcribe/{id} returned existing result if status was "complete"
    even when user wanted to re-transcribe with a different language
  - Solution: Added `force=true` query parameter to allow re-transcription
  - Backend now checks if language changed or force is requested before allowing re-transcription
  - Frontend now passes `force=true` when retranscribe button is clicked
  - Added E2E tests verifying force parameter is sent and confirmation dialog for auto language

- Task 211: CRITICAL - Implement client-side SRT/VTT/JSON generation for downloads
  - Created `frontend/src/utils/subtitles.ts` with generateSRT(), generateVTT(), generateJSON()
  - Download buttons now generate files client-side using Blob + URL.createObjectURL()
  - No server request made - instant downloads from already-loaded data
  - Upload page: uses transcriptionSegments or editedSegments (includes unsaved edits)
  - Videos page modal: uses loaded modalSubtitleSegments
  - Videos page inline buttons: fetch transcription once, then generate client-side
  - Added transcription cache to avoid refetching
  - Created 30 unit tests in subtitles.test.ts
  - Created E2E tests in e2e/subtitle-download.spec.ts verifying no network requests
  - Added LEARNINGS.md entry documenting the issue and solution

### Previously Completed (2026-01-27)
- ✅ Task 202: Add chunked upload endpoints to API.md
  - Added full documentation for POST /api/upload/init, POST /api/upload/chunk,
    POST /api/upload/complete, GET /api/upload/status/{session_id}
  - Includes request/response schemas, error codes, example curl commands
  - Added flow diagram and resume instructions for interrupted uploads
  - Updated table of contents with Chunked Upload section

- ✅ Task 201: Create ERROR_CODES.md with all API error codes
  - Created docs/ERROR_CODES.md (189 lines) with comprehensive documentation
  - Covers HTTP status codes, error response format, all error categories
  - Includes client handling recommendations (retry logic, user-friendly messages)

- ✅ Task 200: Validate zero-size file uploads
  - Added explicit zero-size check to POST /api/upload with clear error message
  - Updated POST /api/upload/init to validate size > 0 separately from other required fields
  - Added 3 tests: TestUploadZeroSizeRejected, TestChunkedUploadInitZeroSizeRejected,
    TestChunkedUploadInitNegativeSizeRejected

- ✅ Task 199: Add database query context timeouts
  - Added `DefaultQueryTimeout = 30 seconds` constant
  - Added `SetQueryTimeout()`, `GetQueryTimeout()`, `queryContext()` methods to DB
  - Updated critical queries: GetVideo, CreateVideo, GetTranscription,
    GetUserByEmail, GetSessionByToken to use context.WithTimeout
  - Added TestQueryTimeout with 3 subtests

- ✅ Task 198: Add aria-label to delete confirmation buttons
  - Added `aria-label="Delete segment ${n}"` to segment delete buttons in upload.astro
  - videos.astro already had proper aria-label for video delete buttons

- ✅ Task 197: Add tests for chunked upload edge cases
  - Added 7 new tests: out-of-order chunks, gap in sequence, negative index,
    zero-size file, missing session ID, nonexistent session, no chunk data
  - Idempotency (duplicate submission) was already tested
  - Total chunked upload tests: 15

- ✅ Task 196: Add io.LimitReader to whisper response reading
  - Added `maxWhisperResponseSize = 100 MB` constant
  - Wrapped `io.ReadAll(resp.Body)` with `io.LimitReader()` for memory safety
  - Prevents memory exhaustion from malformed whisper-server responses

- ✅ Task 195: Add RFC 5987 encoding for Content-Disposition filenames
  - Created `backend/httputil` package with `ContentDisposition()` function
  - Implements RFC 5987 with ASCII fallback + `filename*=UTF-8''` encoding
  - Applied to SRT, VTT, JSON, and burned video download endpoints
  - Handles non-ASCII filenames (Japanese, Chinese, Hindi, emoji, etc.)
  - 40 tests covering various encodings and edge cases

- ✅ Task 194: Improve error wrapping consistency in db.go
  - Added fmt.Errorf wrapping to 25+ database functions
  - Errors now include context like "failed to get video %s: %w"
  - Improves debugging by showing which operation failed

- ✅ Task 193: E2E test for chunked upload flow
  - Added frontend/e2e/chunked-upload.spec.ts with 5 tests
  - Tests: init request format, chunk progress, failure handling, localStorage session, API validation
  - Uses Playwright route mocking for large files (no actual 50MB files needed)

- ✅ Task 192: Add defensive XSS escaping to parseUserAgent
  - Wrapped parseUserAgent() output with escapeHtml() in security.astro
  - Defense-in-depth even though current output is hardcoded strings

- ✅ Task 191: Add tests for generateID and helper functions
  - Added TestGenerateID, TestFindVideoFile, TestGetVideoForDecryption to main_test.go
  - Added TestGenerateETag, TestHandleConditionalRequest, TestSetCacheHeaders, TestValidatePathID
  - Tests cover success paths and error handling

- ✅ Task 190: Add tests for transcript align endpoint (HIGH PRIORITY)
  - Added 11 tests for POST /api/transcribe/{id}/align endpoint
  - Tests cover: standard alignment, lyrics mode, no transcription, incomplete transcription,
    invalid ID, empty text, invalid mode, script conversion, unsupported script, missing language, invalid body
  - Added align endpoint to test server's registerHandlers
  - Added align package import to api_test.go
  - All 11 tests passing

- ✅ Task 189: Add tests for CSRF token and CAPTCHA config endpoints
  - Added 4 tests for GET /api/auth/csrf (authenticated success, unauthenticated, invalid session, expired session)
  - Added 3 tests for GET /api/captcha/config (disabled state, enabled state, no auth required)
  - Added captchaVerifier field to testServer struct
  - Added CSRF and CAPTCHA endpoints to test server's registerHandlers
  - 7 new tests total, all passing

- ✅ Task 188: Add tests for email verification endpoints
  - Added 5 tests for GET /api/auth/verify (missing token, invalid token, success, expired, already used)
  - Added 6 tests for POST /api/auth/resend-verification (invalid format, empty, nonexistent, verified, unverified, invalid body)
  - 11 new tests total, all passing

### Previously Completed
- ✅ Task 187: Add comprehensive audit logging for auth events (SECURITY)
  - Added User-Agent parameter to `LoginSuccess()` and `SessionCreated()` security events
  - Added `TwoFASetupInitiated` security event call in TOTP setup endpoint
  - Security events now log IP, User-Agent, timestamp, and outcome
  - Updated tests for new function signatures

- ✅ Task 185: Return partial conversion errors in script conversion
  - Script conversion now tracks failed segment indices
  - Response includes `conversion_failed_indices` array when conversion fails for some segments
  - Added warning logs for failed conversions
  - Frontend can display which segments need manual review

- ✅ Task 183: Add per-email rate limiting for magic link requests (SECURITY)
  - Added `CountRecentMagicLinkRequests()` to count recent magic link requests per user
  - Magic link endpoint now checks for max 3 requests per 15 minutes per email
  - Rate-limited requests still return generic success (prevents enumeration)
  - Added `EventMagicLinkRateLimitExceeded` security event logging
  - Added `TestCountRecentMagicLinkRequests` test

- ✅ Task 182: Implement comprehensive temp file cleanup in transcription goroutines
  - Added `defer os.Remove(audioPath)` in transcription goroutine
  - Added `defer os.Remove(audioPath)` in reprocess goroutine
  - Added `defer os.Remove(outputPath)` in burn goroutine (unencrypted temp file)
  - Removed manual cleanup calls that are now handled by defer
  - All temp files now cleaned up even on panic/error

- ✅ Task 181: Add transaction locking for transcription status updates
  - Modified `UpdateTranscriptionStatus()` to only update when status is 'pending' or 'processing'
  - Modified `UpdateBurnJobStatus()` with same fix
  - Prevents progress goroutine from overwriting 'complete' or 'error' status
  - Added tests: TestTranscriptionStatusRaceProtection, TestBurnJobStatusRaceProtection
  - Database-level solution is more robust than goroutine-level mutex

- ✅ Task 180: Consolidate password validation between auth and validation packages
  - Updated `validation.ValidatePassword()` to check min (8) and max (72)
  - Changed MaxPasswordLength from 128 to 72 (bcrypt limit)
  - Added MinPasswordLength = 8 constant
  - Documented: auth.ValidatePassword() for full complexity, validation.ValidatePassword() for length-only
  - Updated specs/auth.md with validation package details

- ✅ Task 186: Clean up email verification tokens after successful verification
  - Modified `UseEmailVerificationToken` in db.go to delete all tokens for user after verification
  - Added within transaction: `DELETE FROM email_verification_tokens WHERE user_id = ?`
  - Added 2 tests: basic cleanup, cleanup with historical "used" tokens
  - Prevents token table accumulation

- ✅ Task 179: Magic link email verification check
  - Code was already correct (checks `!user.EmailVerified` and returns early)
  - Returns generic success to prevent email enumeration (per spec)
  - Added SendMagicLink to MockService for testing
  - Added magic link handlers to test server
  - Added 9 tests: unverified/verified email, nonexistent, invalid, verify success/expired/invalid/missing, single-use
  - Tests verify magic links are NOT sent for unverified emails

- ✅ Task 184: Verified expired session handling in ValidateSession
  - Code was already correct (returns immediately after delete at line 230)
  - Added TestValidateSessionExpired test to verify behavior
  - Test confirms expired sessions return nil and are deleted from DB

### Previously Completed
- ✅ Task 178: Add ID format validation before database queries
  - Created `ValidateHexID()` in validation package (32-char hex check)
  - Created `validatePathID()` helper in main.go for endpoint use
  - Applied validation to all 14 endpoints using PathValue("id")
  - Returns 400 Bad Request for invalid ID format before DB query
  - Added comprehensive tests for ValidateHexID
- ✅ Task 174: Add unsaved changes warning before navigation
  - Added beforeunload event listener on window
  - Checks hasUnsavedChanges flag and prompts user before leaving page
  - Uses e.preventDefault() and e.returnValue for browser compatibility
- ✅ Task 173: Add keyboard support to modal segment navigation
  - Added `tabindex="0"` and `role="button"` to segment elements
  - Added `aria-label` with timestamp and truncated text for screen readers
  - Added keydown handler for Enter and Space key activation
  - Uses event delegation to prevent memory leaks
- ✅ Task 172: Add aria-labels to video thumbnail buttons
  - Added `aria-label="Play video: {filename}"` to clickable thumbnails
  - Both img thumbnails and placeholder div thumbnails now have descriptive labels
  - Screen readers can now identify what clicking thumbnails will do
- ✅ Task 171: Remove persistent keydown listener in videos.astro
  - Moved keydown handler setup from page-load to modal-open
  - Used AbortController to add/remove document keydown listener dynamically
  - Listener removed in closeVideoModal() via controller.abort()
  - No listener persists when modal is closed
- ✅ Task 170: Fix memory leak in videos.astro modal segment handlers
  - Replaced per-element click handlers with event delegation on modalSegments container
  - Single listener handles all segment clicks using `closest('.modal-segment')`
  - Prevents memory leaks from accumulating handlers on repeated modal open/close
- ✅ Task 169: Add context cancellation to progress simulation goroutines
  - Replaced `chan struct{}` with `context.WithCancel()` in 3 locations
  - POST /api/transcribe/{id}, POST /api/videos/{id}/reprocess, POST /api/burn/{id}
  - Context pattern is safer: cancel() is idempotent, close(chan) panics if called twice
  - Added `defer cancelProgress()` to ensure cleanup even on panic
- ✅ Task 177: Add E2E test reference to TESTING.md
  - Added note in Frontend Tests section referencing BROWSER_TESTING.md
- ✅ Task 176: Add cross-references to ENV.md
  - Added ENV.md reference to backend/README.md after env var table
  - Added ENV.md reference to INSTALL.md env var section
- ✅ Task 175: Fix TESTING.md npm run command typo
  - Corrected `npm test:watch` to `npm run test:watch`
- ✅ Task 168: Add panic recovery to transcription/burn goroutines
  - Added defer recover() to POST /api/transcribe/{id} goroutine
  - Added defer recover() to POST /api/videos/{id}/reprocess goroutine
  - Added defer recover() to POST /api/burn/{id} goroutine
  - Goroutine crashes now log error and mark jobs as failed
- ✅ Task 167: Use parameterized queries for pagination
  - Changed db/db.go ListVideosPaginated() from fmt.Sprintf to parameterized ? placeholders
  - Prevents theoretical SQL injection in LIMIT/OFFSET clauses
- ✅ Task 166: Clarify LINTERS.md about optional tools
  - Reorganized to clearly show what's IN USE vs optional
  - Added "NOT CURRENTLY USED" labels to ESLint, Prettier, Husky sections
  - Updated Quick Reference table with status column
  - Clarified pre-commit hook uses shell script, not Husky
- ✅ Task 165: Update BROWSER_TESTING.md to match Playwright config
  - Updated config example to show only Chromium (matches actual config)
  - Added note explaining Firefox/WebKit are optional
  - Updated multi-browser commands to show they require config changes
- ✅ Task 164: Update TESTING.md with accurate test file counts
  - Updated from "86 tests in 7 files" to "442 tests in 24 files" for backend
  - Updated from "64 tests in 3 files" to "197 tests in 9 files" for frontend
  - Total: 639 tests (was documented as 150)
  - Added all test file descriptions to the tables
- ✅ Task 163: Make whisper parameters configurable
  - Added `WHISPER_THREADS` (default 4) for whisper-cli thread count
  - Added `WHISPER_TEMPERATURE` (default 0.0) for whisper-server temperature
  - Added `WHISPER_TIMEOUT` (default 30m) for whisper-server request timeout
  - Added `getEnvIntOrDefault()` helper function for int env vars
  - Updated docs/ENV.md with all new environment variables
- ✅ Task 162: Handle GetSegments error in transcription status
  - Fixed ignored error in `main.go:896` where `t.GetSegments()` error was discarded with `_`
  - Now logs error with transcription_id, video_id, and returns empty segments array
  - Updated test code in `api_test.go` to check errors explicitly with `t.Fatal`
  - New tasks 163-166 created from deep inspection findings
- ✅ Task 161: Add database query performance logging
  - Created `backend/db/profiler.go` with query timing wrapper
  - Logs slow queries (>100ms by default) with operation, table, duration, rows
  - Enable via `LOG_SLOW_QUERIES=true`, threshold via `SLOW_QUERY_THRESHOLD_MS`
  - Added 12 unit tests covering all profiler functionality
  - Updated docs/ENV.md with new debugging configuration section
- ✅ Task 160: Create metrics dashboard documentation
  - Created comprehensive `docs/METRICS.md` documenting all 7 Prometheus metrics
  - Included 20+ PromQL queries for traffic, latency, transcription, uploads
  - Added Prometheus alert rules for errors, latency, queue backlog
  - Added capacity planning section with baseline metrics and scaling indicators
  - Recommended Grafana dashboard layout with panel descriptions
- ✅ Task 159: Enhance SECURITY_EVENTS.md with monitoring examples
  - Added 8 journalctl commands for filtering security events
  - Added Prometheus alert rules for brute force, lockouts, rate limits, admin access
  - Added Grafana Loki and ELK Stack query examples
  - Added incident response playbooks for brute force and account compromise
  - Document now actionable for production monitoring setup
- ✅ Task 158: Implement metrics path normalization
  - Added regex patterns for 32-char hex IDs, UUIDs with dashes, and numeric IDs
  - `normalizePath()` reduces cardinality by replacing dynamic IDs with `{id}` placeholder
  - 15+ test cases for various path patterns including edge cases
  - Prevents Prometheus metrics cardinality explosion from unique video/session IDs
- ✅ Task 151: Implement admin role system for metrics endpoint
  - Created migration 005 to add `role` column to users table (default: "user")
  - Added `Role` field to User struct and `IsAdmin()` method
  - Updated `/metrics` endpoint to require admin role (or API key)
  - Added `INITIAL_ADMIN_EMAIL` env var for bootstrapping first admin
  - Added `AccessDeniedNotAdmin()` security event helper
  - Tests: `TestMetricsEndpointAdminOnly`, `TestUserRoleManagement`
  - Updated docs/ENV.md with admin configuration section
- ✅ Task 152: Add security event audit logging
  - Created `backend/security/events.go` package for consistent audit logging
  - 30+ event types covering: auth, 2FA, sessions, access control, rate limits
  - Added callbacks to ratelimit package for violation logging
  - Updated all auth handlers with security events
  - Created `docs/SECURITY_EVENTS.md` with monitoring recommendations
  - 15 unit tests for security package
- ✅ Task 155: Create production security hardening checklist (docs)
  - Created comprehensive `docs/SECURITY_CHECKLIST.md` with 10 categories
  - Covers: server hardening, env var security, database, encryption, network, rate limiting, monitoring, backup, access control, dependencies
  - Includes verification commands and periodic review schedule
  - Referenced from `specs/deployment.md`
- ✅ Task 154: Rate limit /metrics endpoint
  - Added metricsLimiter (10 req/min) to prevent reconnaissance attacks
  - Applied rate limit wrapper to GET /metrics endpoint
  - Updated docs/RATE_LIMITS.md and docs/ENV.md with METRICS_RATE_LIMIT
  - Added test TestRateLimitingMetrics
- ✅ Task 157: Make transcript paste and full text sections collapsible
  - Addressed user feedback about elements cluttering the UI
  - Added collapsible accordion UI with localStorage persistence
  - Full text section starts collapsed, paste transcript expanded
  - Reduces vertical space so video and subtitles are easier to read
- ✅ Task 153: Add video file magic byte validation
  - Added `ValidateMagicBytes()` and `ValidateMagicBytesFromFile()` to audio package
  - Validates MP4/MOV/M4V (ftyp), WebM/MKV (EBML), AVI (RIFF+AVI), OGV (OggS), MPEG
  - Integrated into both simple upload and chunked upload handlers
  - 20 new tests covering all formats and edge cases
- ✅ Task 156: Fix video scrolling out of view during playback (3rd report)
  - **Root cause:** `scrollIntoView()` scrolls ALL ancestor containers including the page
  - **Fix:** Replaced with container-only scrolling in upload.astro
  - **Prevention:** Added E2E regression tests, LEARNINGS.md entry, and `specs/video-scroll-fix.md`

### Previously Completed (Tasks 134-142)
All 9 tasks from code review batch complete:
- Task 134: Apply pathvalidator to file-serving endpoints
- Task 135: Persist CSRF secret across restarts
- Task 136: Consolidate duplicate escapeHtml functions
- Task 137: Add focus management to video modal
- Task 138: Add upload timeout mechanism
- Task 139: Add aria-labels to download buttons
- Task 140: Add file access audit logging
- Task 141: Max limit validation for pagination (already implemented)
- Task 142: Add database indices for frequent queries (already implemented)

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
| Test documentation | `TESTING.md`, `BROWSER_TESTING.md` |
| Learnings | `LEARNINGS.md` |

## Notes

- Whisper-server configurable via `WHISPER_URL` env var
- Email requires `RESEND_API_KEY` env var
- All tests: `./scripts/verify-all.sh`
