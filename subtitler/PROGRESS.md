# Progress

This file tracks high-level progress on the subtitler project. For detailed specifications, see `specs/` directory.

## Project Status Summary

**114 tasks completed** as of 2026-01-26. All core features implemented and tested.

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
- See: `specs/auth.md`, `specs/totp.md`, `specs/email.md`

### Upload Limits & Retention (Tasks 13-14, 61)
- ✅ Anonymous users: 2 uploads, 48-hour retention
- ✅ Registered users: unlimited uploads, 90-day retention
- ✅ Auto-delete scheduler for expired files
- ✅ Retention countdown display in UI

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

### Technical Infrastructure (Tasks 109, 113)
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

## Current Work

**2 tasks remaining** (112, 116):

1. UX improvement (112): Burn progress indicators
2. Performance (116): Chunked uploads for large videos

### Recently Completed
- ✅ Task 111: Video thumbnail generation - thumbnails auto-generated during upload at 10% of video duration, displayed on My Videos page, encrypted at rest
- ✅ Task 115: Removed hardcoded Go paths - scripts now auto-detect Go in PATH or `$HOME/go/bin`
- ✅ Task 114: Authenticated health check - public requests now return minimal `{"status": "ok/degraded"}`, authenticated users get full details

See `TASKS.jsonl` for details.

## Key Files

| Purpose | Location |
|---------|----------|
| Main specification | `specs/subtitler.md` |
| Authentication | `specs/auth.md`, `specs/totp.md` |
| Encryption | `specs/encryption.md` |
| Email service | `specs/email.md` |
| Lyrics alignment | `specs/lyrics-alignment.md` |
| Script conversion | `specs/script-conversion.md` |
| API documentation | `docs/API.md` |
| Test documentation | `TESTING.md`, `BROWSER_TESTING.md` |
| Learnings | `LEARNINGS.md` |

## Notes

- Whisper-server configurable via `WHISPER_URL` env var
- Email requires `RESEND_API_KEY` env var
- All tests: `./scripts/verify-all.sh`
