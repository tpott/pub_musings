# Progress Report

## Current Status (2026-01-19)

**Just completed:** Task 19 - Add file cleanup job.

**Changes made this session:**

### Task 19: Add file cleanup job
- Added cleanup configuration to `internal/config/config.go`:
  - `CLEANUP_ENABLED` (default: true)
  - `CLEANUP_MAX_AGE_DAYS` (default: 30)
  - `CLEANUP_INTERVAL_MINS` (default: 60)
- Added database methods in `internal/db/jobs.go`:
  - `GetOldJobs(maxAgeDays int)` - retrieves completed/failed jobs older than N days
  - `DeleteJob(id int64)` - deletes a job record
- Created cleanup service in `internal/cleanup/`:
  - Runs as background goroutine on configurable interval
  - Deletes uploaded files, result files, and database records
  - Cleans up empty parent directories
  - Logs cleanup statistics (jobs cleaned, bytes freed)
  - Graceful start/stop with proper synchronization
- Integrated cleanup service into `cmd/server/main.go`
- Added unit tests for cleanup service
- Updated `.env.example` with cleanup configuration
- Updated `README.md` with cleanup documentation

**Verification:**
- `go build ./cmd/server` - Backend builds successfully
- `go test ./... -short` - All backend tests pass
- Cleanup service starts on server startup (when enabled)
- Configuration options work via environment variables

## Previous Session Work

### Task 28: Add .env.example files for deployment
- Created `backend/.env.example` with all environment variables documented
- Created `frontend/.env.example` with documentation
- Updated `deploy/README.md` to reference the new .env.example files

### Task 27: Add 404 and Error Pages
- Created `frontend/src/pages/404.astro` - styled 404 page
- Created `frontend/src/pages/500.astro` - styled 500 server error page

### Task 26: Enable Secure Cookies for Production
- Added `COOKIE_SECURE` configuration option
- Modified auth handlers to use configurable Secure flag

### Task 29: Fix Backend Crash on whisper-server Startup
- Increased whisper-server readiness timeout from 30s to 120s
- Made whisper-server startup non-fatal

## Next Priority Tasks

1. **Task 20**: Add usage dashboard for admin
2. **Task 21**: Add language selection for transcription
3. **Task 22**: Add batch processing

## Known Issues

1. **Playwright tests failing** - Some dashboard and upload tests are failing due to auth/API mocking issues. These failures pre-date this session and are not related to recent changes.

## Files Modified This Session

**Backend:**
- `internal/config/config.go` - Added cleanup configuration
- `internal/db/jobs.go` - Added GetOldJobs and DeleteJob methods
- `internal/db/jobs_test.go` - Added tests for new methods
- `internal/cleanup/cleanup.go` (new) - Cleanup service
- `internal/cleanup/cleanup_test.go` (new) - Cleanup tests
- `cmd/server/main.go` - Integrated cleanup service
- `.env.example` - Added cleanup configuration

**Documentation:**
- `README.md` - Added cleanup documentation
