# Progress

This file tracks high-level progress on the subtitler project. For detailed specifications, see `specs/` directory.

## Project Status Summary

**130 tasks completed** as of 2026-01-26. All core features implemented and tested.

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

**145 tasks completed!**

### Recently Completed
- ✅ Task 138: Upload timeout mechanism - Added timeout support to both single-file and chunked uploads. Single-file uploads have a 5-minute timeout. Chunked uploads have a 1-minute timeout per chunk. User-friendly timeout error message with retry suggestion. Cleanup on page unload and when starting new uploads.
- ✅ Task 142: Database indices for frequent queries - Already implemented! All required indices exist in 001_initial_schema.up.sql: idx_users_email, idx_videos_user_id, idx_videos_session_id, idx_sessions_user_id, idx_sessions_expires_at, idx_transcriptions_video_id.
- ✅ Task 140: File access audit logging - Added structured logging to all file download endpoints (video, thumbnail, burned video). Each download is logged with file_type, video_id, user_id, session_id, and client_ip. Helps audit access patterns and investigate security incidents.
- ✅ Task 141: Max limit validation for pagination - Already implemented! GET /api/videos caps limit at 100 when client requests more. Verified with existing test at api_test.go:3085.
- ✅ Task 139: Add aria-labels to download buttons - Added descriptive aria-labels to all SRT/VTT/JSON download buttons in videos.astro and upload.astro. Labels like "Download subtitles in VTT format" help screen reader users understand what each button does.
- ✅ Task 137: Add focus management to video modal - When video modal opens, focus moves to close button. Tab/Shift+Tab keys are trapped inside the modal (cycles between focusable elements). Focus is restored to the trigger element (View button or thumbnail) when the modal closes. Improves keyboard accessibility.
- ✅ Task 136: Consolidate duplicate escapeHtml functions - Removed duplicate escapeHtml implementations from upload.astro, videos.astro, and security.astro. Each file now imports from utils/html.ts instead. The format.ts version remains separate as it has different behavior (handles null/undefined, escapes quotes) for test compatibility.
- ✅ Task 135: Persist CSRF secret across restarts - Modified csrf package to persist generated secrets to `data/csrf.key` file. Priority order: 1) CSRF_SECRET env var, 2) existing file, 3) generate new and save. Added CSRF_SECRET_PATH env var for custom file location. File created with 0600 permissions. Tests verify persistence, env var priority, and directory creation. Documented in ENV.md.
- ✅ Task 134: Apply pathvalidator to file-serving endpoints - Integrated the pathvalidator package into video download, thumbnail, and burned video endpoints. Each endpoint now validates file paths from the database stay within the uploads directory before serving. Returns 403 Forbidden if path validation fails (e.g., if database is compromised and contains path traversal). Added comprehensive test `TestVideoDownloadPathValidation` with subtests for valid paths, video path traversal, and thumbnail path traversal.
- ✅ Task 131: HSTS and security headers - Added `Strict-Transport-Security` header (enabled when HTTPS_ONLY=true) and `Permissions-Policy` header to disable unused browser features (geolocation, camera, microphone, etc.). Updated specs/auth.md with full documentation. Tests verify both headers are present.
- ✅ Task 130: Path validation utility - Created `pathvalidator` package to prevent path traversal attacks. Validates that file paths stay within allowed directories (uploads/, data/). Functions: `New()`, `ValidatePath()`, `SafeJoin()`, `ValidateAndResolve()`. Comprehensive tests verify protection against `../` traversal, null bytes, and symlink-based attacks.
- ✅ Task 129: Production error messages - Created `errmsg` package with user-friendly error messages that don't leak internal details. Added `LOG_VERBOSE` env var (default false) to enable detailed errors in development. Updated endpoints to use the new error handling. Documented in ENV.md and API.md with security warnings. Tests verify that production errors don't leak paths, IPs, or SQL queries.

### Tasks 134-142 (ALL COMPLETE)
All 9 tasks from code review batch are complete:
- Task 134 (done): Apply pathvalidator to file-serving endpoints
- Task 135 (done): Persist CSRF secret across restarts
- Task 136 (done): Consolidate duplicate escapeHtml functions
- Task 137 (done): Add focus management to video modal
- Task 138 (done): Add upload timeout mechanism
- Task 139 (done): Add aria-labels to download buttons
- Task 140 (done): Add file access audit logging
- Task 141 (done): Max limit validation for pagination (already implemented)
- Task 142 (done): Add database indices for frequent queries (already implemented)

**No remaining tasks in TASKS.jsonl. Project is feature-complete.**

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
