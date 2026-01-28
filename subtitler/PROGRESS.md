# Progress

This file tracks high-level progress on the subtitler project. For detailed specifications, see `specs/` directory.

## Project Status Summary

**306 tasks completed** as of 2026-01-28. All core features implemented and tested. User feedback addressed: bionic reading, playback speed, dark mode, subtitle viewer improvements.

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

**307 tasks completed.**

### Recent Work (2026-01-28)

**Task 307: Validate Whisper Model Path from Environment (COMPLETE)**
- Added `ValidateFilePath()` function to `backend/validation/validation.go`
- Checks for path traversal patterns (`..` as path segment) and null bytes
- Updated `getWhisperModel()` to return `(string, error)` instead of just `string`
- Validates path with `ValidateFilePath()` before use in `exec.Command`
- Applies `filepath.Clean()` after validation to normalize the path
- Updated all callers (3 locations) to handle the error
- Added 18 test cases covering valid paths, traversal attacks, and null byte injection
- Defense-in-depth: prevents command injection via malicious WHISPER_MODEL env var

**Task 306: Add Shutdown Context to Transcription/Burn Goroutines (COMPLETE)**
- Added `isShuttingDown()` helper function to check shutdown context
- Updated transcription goroutine (line 4281) to check shutdown at:
  - Before starting work
  - After video decryption
  - After audio extraction
- Updated reprocess goroutine (line 5354) with same checks
- Updated burn goroutine (line 5834) with checks at:
  - Before starting work
  - After video decryption
  - Before starting ffmpeg
- Updated all progress simulation goroutines to also listen on `shutdownCtx.Done()`
- Jobs are marked with "Server shutting down - [type] interrupted" message
- Added 2 unit tests for `isShuttingDown()` behavior
- Ensures clean shutdown without leaving jobs in stuck "processing" state

**Task 305: Extract Form Blur Validation to Shared Utility (COMPLETE)**
- Created `src/utils/form-validation.ts` with reusable validation pattern
- `setupBlurValidation()`: Attaches blur and input event listeners
- `setupBlurValidations()`: Set up multiple fields at once
- `createPasswordMatchValidator()`: Creates confirm password validator
- `createOptionalValidator()`: Wraps validator to skip empty values
- Added 20 unit tests in form-validation.test.ts
- Updated 5 pages to use the new utility:
  - login.astro: email, password, TOTP, magic link email
  - register.astro: email, password, confirm password
  - settings.astro: verify code, disable password/code, regen password/code
  - forgot-password.astro: email
  - reset-password.astro: password, confirm password
- Reduces ~150 lines of duplicated blur/input validation code
- Total frontend tests: 436 -> 456

**Task 304: Verify Password Complexity Claims in SECURITY_CHECKLIST.md (COMPLETE)**
- Verified that docs/SECURITY_CHECKLIST.md correctly documents password requirements
- The documentation claims passwords require: min 8 chars, 1 uppercase, 1 lowercase, 1 number, 1 special char
- Confirmed auth/auth.go ValidatePassword() DOES enforce all these requirements
- Task description was based on a misunderstanding - the docs are accurate
- No changes needed to SECURITY_CHECKLIST.md

**Tasks 300-303: Already Implemented (COMPLETE)**
- Task 300: subtitles.test.ts exists with 30+ tests
- Task 301: history.test.ts exists with 18 tests
- Task 302: email/email_test.go exists with email package tests
- Task 303: httputil/contentdisposition_test.go exists with RFC 5987 encoding tests

**Task 299: Add Timeout to CAPTCHA Script Loading (COMPLETE)**
- Added 15 second timeout when loading hCaptcha script from CDN
- Created `CaptchaLoadTimeoutError` class for specific error handling
- Clears pending promise and removes script on timeout
- Prevents indefinite hanging if CDN is slow/unreachable
- Added 3 unit tests for the error class

**Task 298: Consolidate Password Validation Logic (COMPLETE)**
- auth.ValidatePassword() now calls validation.ValidatePassword() for length checks
- Removed duplicate MinPasswordLength constant from auth package
- Single source of truth for password length constants in validation package
- auth.ValidatePassword() still adds complexity requirements on top
- Updated test to expect "required" message for empty passwords

**Task 297: Log Unchecked os.Remove Errors (COMPLETE)**
- Created `removeWithLogging()` helper function in main.go
- Updated all unchecked `os.Remove()` calls in main.go to use the helper
- Updated cmd/rotate-keys/main.go to log errors on cleanup file removals
- Each removal now logs a warning with description, path, and error if it fails
- `os.IsNotExist` errors are ignored (file already gone is fine for cleanup)
- Prevents silent cleanup failures that could lead to disk space issues

**Task 296: Database Query Context Timeouts (COMPLETE)**
- Added context timeouts to ALL remaining database functions in db/db.go
- Pattern: `ctx, cancel := db.queryContext(); defer cancel(); db.conn.ExecContext(ctx, ...)`
- Updated functions include:
  - TOTP: DisableTOTP
  - Videos: GetExpiredVideosPaginated, CountExpiredVideos, UpdateSegments, DeleteVideo, UpdateVideoThumbnail, UpdateVideoEmbeddedSubtitles, GetVideosByKeyVersion, GetVideosWithOldKeyVersion, CountVideosByKeyVersion, UpdateVideoKeyVersion, UpdateVideoThumbnailKeyVersion
  - Burn jobs: CreateBurnJob, GetBurnJob, UpdateBurnJobStatus, CompleteBurnJobWithKeyVersion, FailBurnJob
  - Recovery codes: SaveRecoveryCodes, GetUnusedRecoveryCodes, UseRecoveryCode, DeleteRecoveryCodes, CountUnusedRecoveryCodes
  - Password reset: CreatePasswordResetToken, GetPasswordResetToken, UsePasswordResetToken, DeletePasswordResetTokens, DeleteExpiredPasswordResetTokens
  - Email verification: CreateEmailVerificationToken, GetEmailVerificationToken, VerifyUserEmail, DeleteEmailVerificationTokens, DeleteExpiredEmailVerificationTokens, GetUnusedEmailVerificationToken
  - Magic link: CreateMagicLinkToken, GetMagicLinkToken, UseMagicLinkToken, DeleteMagicLinkTokens, DeleteExpiredMagicLinkTokens, CountRecentMagicLinkRequests
  - User management: UpdateUserPassword, UpdateUserRole, PromoteToAdmin
  - Login attempts: RecordLoginAttempt, GetRecentFailedLoginAttempts, ClearLoginAttempts, DeleteExpiredLoginAttempts, IsEmailLocked
  - Maintenance: Vacuum, Analyze
  - Upload sessions: CreateUploadSession, GetUploadSession, UpdateUploadSessionStatus, CreateUploadChunk, GetUploadChunk, GetUploadChunks, CountUploadChunks, GetReceivedChunkIndices, GetTotalReceivedBytes, GetExpiredUploadSessions, DeleteUploadSession, UploadSessionExists
  - Migration helper: handleExistingDatabase
- All `db.conn.Exec()`, `db.conn.Query()`, `db.conn.QueryRow()` calls now use context variants
- Prevents indefinite hangs if database becomes unresponsive

**New Tasks Created from Deep Inspection:**
- Task 296: Backend: Add context timeouts to database query calls (in_progress)
- Task 297: Backend: Log unchecked os.Remove errors
- Task 298: Backend: Consolidate password validation logic
- Task 299: Frontend: Add timeout to CAPTCHA script loading
- Task 300: Frontend: Add tests for subtitles.ts
- Task 301: Frontend: Add tests for history.ts
- Task 302: Backend: Add tests for email package
- Task 303: Backend: Add tests for httputil.ContentDisposition
- Task 304: Docs: Fix password complexity claims in SECURITY_CHECKLIST.md
- Task 305: Frontend: Extract form blur validation to shared utility

**Task 294: Runtime Schema Validation for API Responses**
- Added Zod schemas in `src/utils/api-schemas.ts` for all major API responses
- Updated VideoListResponseSchema to match actual API fields (transcription_status, thumbnail_path, etc.)
- Added schemas: VideoSchema, AuthMeResponseSchema, SessionListResponseSchema, TotpSetupResponseSchema, TotpVerifyResponseSchema, RecoveryCodesResponseSchema, TranscriptionStatusResponseSchema, CsrfTokenResponseSchema, LoginResponseSchema, RegisterResponseSchema
- Added helper functions: safeParse(), parse(), isValid(), validateResponse(), validateResponseOrThrow()
- Updated videos.astro to validate video list and transcription responses
- Updated settings.astro to validate TOTP setup, verify, recovery codes, and sessions responses
- Updated nav-auth.ts to validate auth/me response
- Created 34 unit tests in api-schemas.test.ts
- Total frontend tests increased from 399 to 433

**Task 293: Fetch Timeout Utility**
- Created `src/utils/fetch-timeout.ts` with AbortController-based timeout handling
- Exports: `fetchWithTimeout()`, `FetchTimeoutError`, `isTimeoutError()`, `createTimeoutController()`
- Default 30-second timeout, configurable per request
- Properly chains abort signals for existing controllers
- Added 29 unit tests in fetch-timeout.test.ts

**Task 292: MaxBytesReader for JSON Endpoints**
- Added decodeJSONBody() helper function with 1MB limit
- Updated all 13 JSON endpoint handlers to use the helper
- Returns HTTP 413 Request Entity Too Large for oversized payloads

**Task 291: Backend Panic to Error Conversion**
- WithTransaction now converts panics to PanicError instead of re-panicking
- PanicError includes panic value and stack trace for debugging
- Prevents server crashes from panics during database transactions
- Updated test to verify new behavior

**Task 295: Processing Speed localStorage Cleanup**
- getHistory() now clears corrupted localStorage instead of just logging
- Also clears invalid structure (valid JSON but missing records array)
- Updated 2 tests to verify cleanup behavior

**Task 290: CSRF JSON Parse Error Handling**
- Added separate try-catch for response.json() in fetchCsrfToken()
- Better error messages distinguishing parse errors from network errors
- Added 2 new tests for JSON parse failure and missing csrf_token field

**Task 289: Form Error Handling Utility**
- Created src/utils/form-errors.ts with clearFieldError() and showFieldError()
- Removed duplicate implementations from 5 astro files (login, register, settings, forgot-password, reset-password)
- Added 10 unit tests in form-errors.test.ts

**Task 288: Session Loading Timeout**
- Added AbortController with 30 second timeout to `loadSessions()` in settings.astro
- Shows "Request timed out. Please try again." message on timeout
- Prevents UI hanging on slow networks

**Task 281: GetExpiredVideos Pagination**
- Added `GetExpiredVideosPaginated(limit, offset)` with `CountExpiredVideos()` helper
- Updated `runCleanup()` to process in batches of 100 to prevent OOM

**Task 282: Error Context in ListVideosPaginated**
- Added fmt.Errorf wrapping for all database errors

**Task 283: IsEmailLocked Timestamp Parsing**
- Created `parseSQLiteTimestamp()` helper for aggregate function results
- SQLite MIN() returns strings, so sql.NullTime doesn't work directly

**Task 284: Session Listener Memory Leak**
- Fixed settings.astro to use event delegation for revoke buttons

**Task 285: Clear Cached Modal Segments**
- videos.astro now clears `cachedModalSegmentElements` on modal close

**Task 286: Deduplicate escapeHtml**
- Consolidated 3 implementations into one in html.ts
- Added fallback for non-browser environments (Node.js tests)

**Task 287: dom.ts Tests**
- Added 15 tests for DOM utility functions with mocked document

### Task 280: Session Cleanup Error Logging (2026-01-28)

- ✅ Task 280: Backend - Added logging for DeleteSession errors in auth cleanup paths
  - In `auth/auth.go` ValidateSession(), two DeleteSession calls silently ignored errors
  - Added logging.Warn() calls for expired session cleanup and orphaned session cleanup
  - Logs include session_id, user_id, and error message to aid debugging
  - Uses Warn level since cleanup failures are non-fatal but worth investigating

### Tasks 275-279: Error Handling and Testing (2026-01-28)

- ✅ Task 275: Backend - Added error handling to whisper WriteField calls
  - Check errors from writer.WriteField() for response_format, temperature, language
  - Check error from writer.Close() after writing all fields
  - Return descriptive errors instead of ignoring them

- ✅ Task 276: Backend - Standardized JSON response encoding with httputil helpers
  - Converted 36 direct json.NewEncoder(w).Encode() calls to httputil helpers
  - Uses RespondError() for error responses, RespondJSON() for success responses
  - Benefits: error handling for encoding failures, consistent headers, reduced duplication

- ✅ Task 277: Backend - Added error handling to file Close() calls in upload handlers
  - Simple upload: destFile.Close() after copying uploaded file
  - Chunked upload: destFile.Close() after writing chunk data
  - Chunked upload complete: destFile.Close() and chunkFile.Close() during assembly
  - Close errors logged with WarnContext but don't fail operations (data already written)

- ✅ Task 278: Backend - Added tests for goroutine panic recovery
  - TestGoroutinePanicRecovery: panic is recovered, callback called, runtime errors, error values
  - TestJobFailureOnPanic: job transitions to error state, normal completion works
  - Tests verify defer/recover pattern in transcription, reprocess, and burn goroutines

- ✅ Task 279: Docs - Documented existing E2E upload test scenarios in BROWSER_TESTING.md
  - Upload Flow Tests: file selection, validation, transcription, video visibility
  - Chunked Upload Tests: initialization, progress, error handling, resumability

### Task 274: Create DOM query helper (2026-01-28)

- ✅ Task 274: Created utils/dom.ts for type-safe element access
  - `getRequiredElement(id, type)`: Get single element, throws if missing/wrong type
  - `getRequiredElements(spec)`: Get multiple elements from spec object
  - `getOptionalElement(id, type)`: Returns null if not found
  - `queryRequiredElement(selector, type, parent?)`: Query by CSS selector
  - `queryAllElements(selector, type, parent?)`: Query multiple by selector
  - `ElementNotFoundError` class for missing elements
  - Tests verify module exports and function signatures

### Task 273: Add RecordUploadFailed metrics (2026-01-28)

- ✅ Task 273: Added metrics.RecordUploadFailed() to all upload error paths
  - Audited both POST /api/upload and POST /api/upload/complete handlers
  - Added 15 RecordUploadFailed() calls total:
    - POST /api/upload: 10 error paths (form parsing, file errors, encryption, DB save)
    - POST /api/upload/complete: 5 error paths (chunk handling, encryption, DB save)
  - Skipped validation errors (unsupported format, magic bytes) - not system failures
  - Improves observability of upload failures in Prometheus metrics

### Task 272: Create specs/path-security.md (2026-01-28)

- ✅ Task 272: Created path security specification document
  - Documents the `backend/pathvalidator` package and its usage
  - Explains threat model (traversal attacks, symlinks, null bytes)
  - Details the critical order: check BEFORE filepath.Clean()
  - Documents all functions: ValidatePath, SafeJoin, ValidateAndResolve
  - Includes usage patterns for uploads and downloads
  - Lists allowed directories (uploads/, data/, data/keys/)

### Task 271: Create specs/error-handling.md (2026-01-28)

- ✅ Task 271: Created error handling specification document
  - Documents the `backend/errmsg` package and its usage
  - Explains verbose mode controlled by `LOG_VERBOSE` env var
  - Lists all user-friendly error message constants
  - Provides usage guidelines (when to use vs not use)
  - Documents sensitive information patterns to protect
  - Includes testing examples for production error verification

### Task 270: Add arrow key navigation to speed dropdown (2026-01-28)

- ✅ Task 270: Added keyboard navigation to speed dropdown menu
  - Updated `speed-control.ts` with arrow key navigation (Up/Down to move, Enter/Space to select)
  - Escape closes menu, Tab closes and moves to next element
  - Added `focusedIndex` state tracking for keyboard focus
  - Added `.focused` CSS class to both upload.astro and videos.astro
  - Focus automatically moves to active speed option when menu opens
  - Tests updated to document keyboard navigation behavior

### Task 269: Add composite index for login_attempts (2026-01-28)

- ✅ Task 269: Added composite index for login_attempts queries
  - Created migration 009 with index on `login_attempts(email, created_at DESC)`
  - Improves performance of `GetRecentFailedLoginAttempts()` and `IsEmailLocked()` queries
  - These queries filter by email AND time range - composite index more efficient than separate indexes

### Task 268: Wire SetActiveSessions metrics (2026-01-28)

- ✅ Task 268: Wired up active sessions gauge in Prometheus metrics
  - Added `CountActiveSessions()` method to db package
  - Added `updateSessionMetrics()` helper function to main.go
  - Called after: login, logout, session revocation, recovery code login, magic link login
  - Also called after expired session cleanup scheduler runs
  - Added 3 unit tests for CountActiveSessions
  - Metrics now accurately reflect active session count

### Task 267: Add request body size limits (2026-01-28)

- ✅ Task 267: Added DecodeJSONBody helper with request size limits
  - Created `httputil.DecodeJSONBody()` function with http.MaxBytesReader
  - Default 1MB limit, customizable per endpoint
  - Applied to critical auth endpoints: register, login, forgot-password, reset-password, feedback
  - Added 7 tests for size limit enforcement
  - Prevents DoS through oversized JSON payloads

### Task 265-266: Dialog and FeedbackButton cleanup (2026-01-28)

- ✅ Task 265: Added destroyDialog() cleanup function to dialog.ts
  - Stored event handler references for proper cleanup
  - destroyDialog() removes all listeners and DOM elements
  - Useful for testing and explicit cleanup scenarios
  - Added 2 new tests (total tests: 326)

- ✅ Task 266: Refactored FeedbackButton star rating to use event delegation
  - Replaced 15 individual listeners (5 stars × 3 events) with 3 delegated listeners
  - Single mouseenter, mouseleave, and click handler on container
  - Better maintainability and no risk of listener accumulation

### Task 261: Replace any types with proper interfaces (2026-01-28)

- ✅ Task 261: Replaced `any` types in upload.astro with proper TypeScript interfaces
  - Added `UploadSession` interface to types/transcription.ts
  - Changed `uploadSession: any` to `uploadSession: UploadSession | null`
  - Changed `speed as any` to `speed as PlaybackSpeed`
  - Build passes, type safety improved

### Task 262: CSRF retry circuit breaker (2026-01-28)

- ✅ Task 262: Added circuit breaker to CSRF token refresh
  - Track consecutive refresh failures (max 2 before tripping)
  - Skip retry on CSRF 403 when circuit breaker is tripped
  - Reset breaker on successful token fetch, setCsrfToken, or clearCsrfToken
  - Added 2 new tests for circuit breaker behavior (total tests: 324)

### Task 264: Session.ts cleanup tests already exist (2026-01-28)

- ✅ Task 264: Verified session.ts tests already cover stale session cleanup
  - 27 tests in session.test.ts including cleanupStaleUploadSessions() tests
  - Tests cover: 48-hour threshold, fresh vs stale sessions, untracked legacy sessions, invalid JSON

### Task 263: Replace FeedbackButton native alerts with styled dialogs (2026-01-28)

- ✅ Task 263: Replaced native alert() with styled showAlert() in FeedbackButton.astro
  - Import showAlert from dialog utility
  - Error responses now use styled modal instead of native browser alert
  - Better UX with consistent styling and dark mode support

### Task 260: localStorage error handling review (2026-01-28)

- ✅ Task 260: Reviewed localStorage error handling in processing-speed.ts and session.ts
  - Existing code already handles errors correctly with try-catch and fallback values
  - Console.error is appropriate for internal utilities (vs user-facing alerts)
  - Added learning to LEARNINGS.md about graceful degradation patterns

### Task 259: Unit tests for nav-auth.ts (2026-01-28)

- ✅ Task 259: Added unit tests for nav-auth.ts utility
  - Created 15 tests covering module exports, interfaces, and integration expectations
  - Tests document required DOM element IDs, API endpoints, and callback patterns
  - Test count increased from 307 to 322

### Task 256: Video player speed control utility (2026-01-28)

- ✅ Task 256: Created shared speed control utility for video players
  - Created `frontend/src/utils/speed-control.ts` with reusable speed control logic
  - Exports `createSpeedControl()` factory function
  - Handles: speed button, dropdown menu, keyboard shortcuts ([/]), indicator overlay
  - Includes cleanup method for proper event listener removal
  - Can be incrementally adopted by upload.astro and videos.astro
  - 5 module structure tests added

### Task 257: Loading states with meaningful messages (2026-01-28)

- ✅ Task 257: Added meaningful loading states with aria-busy attributes
  - Changed download button loading from `'...'` to `'Loading...'`
  - Added `aria-busy="true"` during async operations for download, delete, and retry buttons
  - Screen readers now properly announce that buttons are processing

### Task 258: Aria-live regions for subtitle changes (2026-01-28)

- ✅ Task 258: Added aria-live regions for subtitle segment changes
  - Added `aria-live="polite"` and `aria-atomic="true"` to current subtitle display in upload.astro
  - Added `aria-live="polite"` and `aria-atomic="true"` to modal subtitle display in videos.astro
  - Screen readers now announce subtitle text changes during video playback
  - Uses `polite` mode so announcements don't interrupt other content

### Task 255: Styled Dialog Modals (2026-01-28)

- ✅ Task 255: Replaced native alert/confirm with styled modals
  - Created `frontend/src/utils/dialog.ts` with showAlert() and showConfirm() utilities
  - Supports info, warning, error, and confirm dialog types
  - Dark mode compatible with CSS variables
  - Keyboard accessible (Escape to close, focus management)
  - Replaced 8 dialogs in videos.astro (7 alerts + 1 confirm)

### Task 254: Frontend DOM Query Optimization (2026-01-28)

- ✅ Task 254: Optimized DOM queries in video timeupdate handlers
  - Cached segment elements after render instead of querySelectorAll on every timeupdate
  - Track lastActiveSegmentIndex to only update when segment changes
  - Reduces DOM queries from ~30/sec to 0 during playback
  - Applied to both upload.astro and videos.astro modal

### Tasks 250-253: Database Query Optimization (2026-01-28)

- ✅ Task 250: Added index on transcriptions(status) for filtering pending/processing jobs
- ✅ Task 251: Added composite index on burn_jobs(video_id, created_at DESC) for recent jobs queries
- ✅ Task 252: Added composite index on feedback(status, feedback_type) for admin dashboard filtering
- ✅ Task 253: Added pagination limit validation to ListFeedback (cap at 100, defense-in-depth)
- Migration 008: `backend/db/migrations/008_add_query_optimization_indexes.up.sql`

### User Feedback Fixes (2026-01-28)

- **Bionic reading word squashing fix**: Changed `.modal-current-subtitle` from `display: flex` to `display: table` to prevent `<strong>` tags from breaking word spacing
- **Playback speed update**: Changed options from `0.5x-2x` to `0.8x, 0.9x, 1x` for language learning focus
- **Playback speed location**: Moved speed control from separate row to action bar (saves vertical space), overlay on upload video
- **Modal segments scroll**: Changed from centering active segment to positioning 2/3 down (shows more past context, less future)
- **Dark mode refresh**: Softened colors - warmer backgrounds (#1a1a1f), brighter accents (#60a5fa), better contrast throughout
- **SRT/VTT/JSON viewer**: Changed from blob download to HTML viewer page with copy-to-clipboard button (prevents browser download prompts)
- **Word-level bionic reading spec**: Created `specs/word-level-highlighting.md` documenting plan for karaoke-style word highlighting

### Tasks 244-248: Documentation Updates (2026-01-27)

- ✅ Task 244: Added feedback endpoints to docs/API.md (POST /api/feedback, admin endpoints)
- ✅ Task 245: Added language hints and embedded subtitles endpoints to docs/API.md
- ✅ Task 246: Added feedback rate limit to docs/RATE_LIMITS.md
- ✅ Task 247: Added admin.feedback.updated to docs/SECURITY_EVENTS.md
- ✅ Task 248: Updated specs/script-conversion.md to document pure Go implementation

### Tasks 225-243: Deep Inspection Findings (2026-01-27)

Deep code review using exploration agents identified areas for improvement:

**Frontend (Memory/Performance):**
- ✅ Task 225: Fixed memory leak in videos.astro pagination handlers
  - Replaced per-element event handlers with event delegation on videoList container
  - Single click/keydown listeners handle all button and thumbnail interactions
  - No more accumulating handlers on each pagination
- ✅ Task 226: Added focus indicators for keyboard navigation elements
  - Added `:focus-visible` styling to all interactive elements in videos.astro
  - Focus indicators on: thumbnails, modal segments, buttons, pagination, nav links
  - Uses consistent `outline: 2px solid var(--accent-color); outline-offset: 2px`
  - Changed `:focus` to `:focus-visible` for better UX (only shows on keyboard nav)
- ✅ Task 230: Added LRU cache cleanup (max 10 entries) to transcriptionCache
  - Evicts oldest entry when cache is full
  - Moves accessed entries to end on cache hit for proper LRU behavior

**Frontend (Code Quality):**
- ✅ Task 227: Extract navigation component to reduce duplication (~150 lines)
  - Created `Navigation.astro` component with all nav styles and HTML
  - Supports `showSettings` prop for conditional Settings link
- ✅ Task 228: Extract auth check logic to shared utility (~160 lines)
  - Created `utils/nav-auth.ts` with `checkAuthAndUpdateNav()` function
  - Accepts callbacks for page-specific auth handling
  - Used by index.astro, videos.astro, settings.astro, upload.astro
- ✅ Task 229: Add aria-describedby for form error associations
  - Added `aria-describedby` attributes linking inputs to error divs
  - Added `role="alert"` to error divs for screen reader announcements
  - Updated: login.astro, register.astro, settings.astro, forgot-password.astro, reset-password.astro
  - Password hint in register.astro also linked via aria-describedby

**Backend (Security/Performance):**
- ✅ Task 231: Sanitize file extensions in path construction
  - Created `validation.SanitizeFileExtension()` to validate file extensions
  - Rejects null bytes, path separators, `..` sequences, too-long extensions
  - Applied to both simple upload and chunked upload finalize handlers
  - 20 unit tests covering all malicious extension patterns
- ✅ Task 232: Configure SQLite connection pool limits
  - Added `DB_MAX_OPEN_CONNS` (default 10) and `DB_MAX_IDLE_CONNS` (default 5) env vars
  - Created `db.PoolConfig` struct and `db.OpenWithConfig()` function
  - Updated main.go to read pool config from environment and log values at startup
  - Documented in docs/ENV.md with tuning guidance
- ✅ Task 233: Add session listing pagination/limit
  - Added `MaxSessionsPerUser = 100` constant to limit session query results
  - Modified `GetSessionsByUserID()` to use the limit by default
  - Added `GetSessionsByUserIDWithLimit()` for custom limit queries
  - 6 unit tests covering limit behavior and edge cases
- ✅ Task 234: Add panic recovery to WithTransaction
  - Added `defer recover()` to `WithTransaction()` to catch panics
  - Rolls back transaction before re-panicking to preserve stack trace
  - Test verifies database changes are rolled back when transaction panics
- ✅ Task 235: Validate environment variable ranges
  - Added range validation to `getEnvSizeOrDefault()` (1MB min, 10GB max)
  - Added negative value validation to `getEnvIntOrDefault()`
  - Logs warnings when values are out of range, uses defaults
  - 16 new tests covering size/int validation edge cases

**Backend (Operations):**
- ✅ Task 236: Add security events for admin operations
  - Added 14 new event types for key rotation, video deletion, maintenance, cleanup
  - Added 13 convenience functions for easy event logging
  - Updated DELETE /api/videos/{id}, runCleanup(), runDatabaseMaintenance()
  - Updated SECURITY_EVENTS.md with comprehensive documentation

**Documentation:**
- ✅ Task 237: Updated deployment.md - moved chunked uploads from Future to Implemented
- ✅ Task 238: Created specs/metrics.md for Prometheus metrics system
  - Documents architecture, design decisions, metrics catalog
  - Covers integration with monitoring systems
  - Cross-referenced from docs/METRICS.md
- ✅ Task 241: Added comprehensive Features section to README.md
  - Documents core functionality, language/script support, UX features
  - Covers advanced features and security capabilities

**Features:**
- ✅ Task 239: Implemented embedded subtitle extraction (Phase 2)
  - Added ExtractSubtitleTrack() function using ffmpeg
  - Added GET /api/videos/{id}/embedded-subtitles/{track} endpoint
  - Frontend shows "Use embedded subtitles" notice with track selection
  - Loads extracted subtitles into editor, replacing Whisper transcription
- ✅ Task 240: Admin dashboard for feedback viewing
  - Backend: Added admin-only API endpoints at /api/admin/feedback
    - GET /api/admin/feedback: List feedback with filters (status, type, pagination)
    - GET /api/admin/feedback/{id}: Get feedback by ID
    - PATCH /api/admin/feedback/{id}: Update feedback status (new/read/resolved)
  - Frontend: Created /admin/feedback page with:
    - Status and type filter dropdowns
    - Feedback list with type badges, status badges, ratings, dates
    - Inline status change dropdowns per feedback item
    - Pagination controls
    - Admin-only access with role check
  - Security: Admin role required for all endpoints, security event logging
  - Added 15 backend tests for admin feedback endpoints

**UX:**
- ✅ Task 242: Added on-screen speed indicator for [ and ] keyboard shortcuts
  - Shows centered overlay with speed (e.g., "0.75x") for 800ms when changing speed
  - Added to both upload.astro and videos.astro modal player
- ✅ Task 243: Improved file size error message to show actual vs max (e.g., "750.5 MB. Maximum allowed size is 500 MB")

### Tasks 244-258: Seventh Deep Inspection (2026-01-27)

Comprehensive deep inspection using parallel exploration agents analyzed frontend, backend, specs, and documentation:

**Documentation Gaps (5 tasks):**
- Task 244: Add feedback endpoints to API.md (POST /api/feedback, admin endpoints)
- Task 245: Add language hints and embedded subtitles endpoints to API.md
- Task 246: Add feedback rate limit to RATE_LIMITS.md
- Task 247: Add admin.feedback.updated to SECURITY_EVENTS.md
- Task 248: Update specs/script-conversion.md to document pure Go implementation

**Backend Performance/Quality (4 tasks):**
- Task 249: Check json.Encode errors in API handlers (50+ unchecked calls)
- Task 250: Add index on transcriptions.status column (query optimization)
- Task 251: Add composite index for burn_jobs queries
- Task 252: Add composite index for feedback queries
- Task 253: Add pagination limit validation to ListFeedback

**Frontend Performance/UX (5 tasks):**
- Task 254: Optimize DOM queries in video timeupdate handler (30+ queries/sec)
- Task 255: Replace native alert/confirm dialogs with styled modals
- Task 256: Extract video player component from upload and videos pages
- Task 257: Add loading states with meaningful messages
- Task 258: Add aria-live regions for subtitle segment changes

### Recently Completed (2026-01-27)
- Task 218: Feature - Improve language auto-detection
  - Created `backend/language` package with comprehensive language detection
  - Implements `DetectFromMetadata()` using ffprobe to extract audio track language tags
  - Implements `DetectFromFilename()` using regex patterns (ISO codes, language names)
  - Added `GET /api/videos/{id}/language-hints` endpoint
  - Language hints returned in upload response (simple and chunked)
  - Frontend displays hints before transcription with confidence indicators
  - 136+ tests covering language detection and API endpoints
  - See: `specs/language-detection.md`

- Task 213: Feature - Implement user feedback system
  - Created `specs/feedback.md` with design document
  - Database migration 007 adds `feedback` table
  - Backend `POST /api/feedback` endpoint with rate limiting (5 req/min)
  - Validates: text required, type (general/bug/feature), rating (1-5), max 10KB text
  - Captures user context: page URL, video ID, session ID, browser info
  - Created `FeedbackButton.astro` component with floating button and modal
  - Added to all 10 pages in frontend
  - 14 backend tests for feedback endpoint
  - Star rating UI with hover effects
  - Form validation with character count

- Task 221: Feature - Variable playback speed for language learning
  - Added speed control dropdown to video player (upload.astro and videos.astro modal)
  - Speed options: 0.5x, 0.75x, 1x (default), 1.25x, 1.5x, 2x
  - Keyboard shortcuts: `[` for slower, `]` for faster
  - Speed preference persisted in localStorage (`subtitler:playback-speed`)
  - Created `frontend/src/utils/playback-speed.ts` utility with 27 unit tests
  - Updated keyboard shortcuts documentation in modal and README.md
  - Uses native HTML5 `playbackRate` with browser's pitch preservation
  - See: `specs/playback-speed.md`

- Task 219: Feature - Detect embedded subtitles
  - Added `GetSubtitleTracks()` to `backend/audio/audio.go` using ffprobe
  - Created `SubtitleTrack` struct with index, language, title, codec, default, forced, text_based
  - Identifies text-based vs image-based subtitle codecs
  - Database migration 006 adds `embedded_subtitles_json` column
  - Detection runs during both simple and chunked uploads (before encryption)
  - API returns parsed embedded_subtitles array in video list response
  - Frontend shows badge with languages on My Videos page
  - See: `specs/embedded-subtitles.md`

- Task 214: Research - Alternative speech-to-text providers
  - Compared AssemblyAI, Deepgram, Google Cloud Speech-to-Text, AWS Transcribe
  - Evaluated: word timestamps, diarization, phoneme support, sentiment analysis, pricing
  - Conclusion: Whisper remains best default (self-hosted, no API costs, good accuracy)
  - AssemblyAI recommended for premium tier with audio intelligence
  - Deepgram recommended for real-time streaming use cases
  - No mainstream providers offer phoneme-level timestamps (need Speechace/SpeechSuper)
  - See: `specs/stt-providers.md`

- Task 224: Feature - Implement Bionic Reading mode for subtitles
  - Created `frontend/src/utils/bionic.ts` with bionic reading utility functions
  - `toBionicSegments()`: Converts text to array of {text, bold} segments
  - `toBionicHTML()`: Generates HTML with `<strong>` tags for fixation points
  - `renderBionicText()`: Main entry point, checks localStorage settings
  - localStorage keys: `subtitler:bionic-reading`, `subtitler:bionic-fixation`
  - Added toggle and fixation slider to Settings > Preferences tab
  - Live preview of bionic text in settings
  - Applied to upload.astro segments (view mode only, not edit mode)
  - Applied to videos.astro modal segments
  - Created 49 unit tests in bionic.test.ts covering all edge cases
  - Disclaimer text warns users about lack of scientific evidence
  - See: `specs/bionic-reading.md`

- Task 223: Research - Bionic Reading implementation
  - Documented what letters to bold (first 30-50% of each word)
  - Researched algorithm variations (fixation percentage, min word length)
  - Reviewed scientific evidence (no proven benefits for general population)
  - Documented alternatives: BeeLine Reader (color gradients), OpenDyslexic font
  - Created implementation plan for Task 224
  - See: `specs/bionic-reading.md`

- Task 215: Frontend - Rename Security to Settings with tabs
  - Created new `/settings` page with tab navigation (Security, Account, Preferences)
  - Security tab contains all existing 2FA and session management features
  - Account and Preferences tabs are placeholders for future features
  - Tab state persisted in localStorage and URL hash
  - Keyboard navigation support (arrow keys) between tabs
  - Updated all navigation links from `/security` to `/settings`
  - Removed old security.astro, updated specs and E2E tests

- Task 222: Frontend - Improve modalSegments layout
  - Fixed height at 180px to show approximately 3 visible subtitles
  - Active segment is now centered in the container (not at bottom)
  - Added smooth scrolling for better UX
  - Mobile responsive: 150px height on smaller screens

- Task 220: Frontend - Add View button after upload completes
  - Added "View in My Videos" button with eye icon to actions section
  - Button navigates to `/videos?play={id}` for cleaner playback experience
  - Positioned prominently at the start of actions row

- Task 216: Frontend - Add Auto mode to theme toggle
  - Theme toggle now cycles through: Light → Dark → Auto
  - Auto mode follows system's prefers-color-scheme setting
  - Added computer/monitor icon for Auto mode
  - Preference persisted in localStorage as 'light', 'dark', or 'auto'
  - Listens for system preference changes when in auto mode

- Task 212: Frontend - Add subtitle alignment feedback buttons
  - Added feedback buttons (good 👍, misaligned ⏱️, missing ❓) to each segment
  - Feedback stored per-segment in localStorage keyed by video ID
  - Toggle behavior: clicking same feedback clears it, clicking different sets new
  - Buttons hidden in edit mode, visible in view mode
  - Documented backend integration plan for POST /api/videos/{id}/feedback

- Task 210: Backend - Add graceful shutdown with signal handling
  - Added `shutdownCtx` and `shutdownCancel` for signaling background goroutines to stop
  - Created `http.Server` with `Shutdown()` method instead of `ListenAndServe`
  - Added signal handling for SIGTERM and SIGINT
  - Updated `startCleanupScheduler()` and `startMaintenanceScheduler()` to exit on shutdown
  - Default shutdown timeout of 30 seconds
  - Added `TestShutdownContextCancellation` test

- Task 209: Backend - Create respondError helper function
  - Created `httputil.RespondError(w, statusCode, message)` and `httputil.RespondErrorf(w, statusCode, format, args...)`
  - Helper sets Content-Type header and writes JSON error response in one call
  - Applied to 10 handlers: upload/complete, register, login, transcribe, segments, delete, videos list, auth/me, upload/init, forgot-password
  - Added comprehensive tests for both functions
  - Reduces boilerplate from 3 lines to 1 line per error response

- Task 208: Frontend - Consolidate duplicate escapeHtml functions
  - Removed escapeHtml from format.ts (kept in html.ts)
  - Removed duplicate tests from format.test.ts (covered by html.test.ts)
  - All pages already import from html.ts

- Task 207: Backend - Add transaction timeout context
  - Modified WithTransaction() to use BeginTx with context timeout
  - Uses existing queryContext() with 30s default timeout
  - Prevents indefinite hangs if database becomes unresponsive
  - Added TestWithTransactionUsesContext test

- Task 206: Backend - Add chunk count validation before assembly
  - Already implemented at main.go:3528-3542 (CountUploadChunks validation)
  - Test exists: TestChunkedUploadCompleteIncomplete at api_test.go:7203
  - Returns 400 with "Not all chunks received. Expected X, got Y" if chunks missing

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
