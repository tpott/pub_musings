# Progress Report

## Current Status (2026-01-19)

**Just completed:** Task 21 - Add language selection for transcription.

**Changes made this session:**

### Task 21: Add language selection for transcription

**Backend:**
- Created migration `004_add_language_to_jobs.sql` - Adds nullable `language` column to jobs table
- Updated `Job` struct in `internal/db/jobs.go` to include `Language *string` field
- Updated all job queries (CreateJob, GetJobByID, GetJobsByUserID, GetOldJobs) to include language column
- Updated `handleUpload()` in `cmd/server/upload_handlers.go`:
  - Reads `language` form parameter
  - Validates against supported ISO 639-1 language codes (99 languages supported)
  - Stores language in job record
  - Includes language in analytics tracking
- Updated worker in `internal/worker/worker.go`:
  - `processJob()` now passes job.Language to TranscribeOptions
  - `processEmbeddedJob()` also passes language for embedded format

**Frontend:**
- Updated `src/pages/index.astro`:
  - Added language dropdown with optgroups (Common, European, Asian, Other)
  - 50+ languages available for selection
  - "Auto-detect" is the default option
  - Dropdown appears after file selection
  - Language is included in form submission when selected
  - Reset button clears language selection

**Tests:**
- Added `TestJobLanguageField` in `internal/db/jobs_test.go`:
  - Tests job creation without language (auto-detect)
  - Tests job creation with language specified
  - Tests language retrieval in GetJobsByUserID

**Documentation:**
- Created implementation plan `015_LANGUAGE_SELECTION.md`

**Verification:**
- `go build ./cmd/server` - Backend builds successfully
- `go test ./... -short` - All backend tests pass (26 tests)
- `npm run build` - Frontend builds successfully

## Previous Session Work

### Task 20: Add admin usage dashboard
- Added `is_admin` field to users table
- Admin analytics API endpoint and dashboard page
- Stats cards for users, jobs, storage

### Task 19: Add file cleanup job
- Automatic deletion of old job files (>30 days)

### Task 28: Add .env.example files for deployment

### Task 27: Add 404 and Error Pages

## Next Priority Tasks

1. **Task 22**: Add batch processing - User can upload multiple files at once
2. **Task 11**: Cloudflare Tunnel setup (blocked - requires human action)

## Known Issues

1. **Playwright tests failing** - Some dashboard and upload tests are failing due to auth/API mocking issues. These failures pre-date this session and are not related to recent changes.

## Files Created This Session

**Backend:**
- `internal/db/migrations/004_add_language_to_jobs.sql` - Language column migration

**Documentation:**
- `015_LANGUAGE_SELECTION.md` - Implementation plan

**Modified Files:**
- `internal/db/jobs.go` - Added Language field and updated queries
- `internal/db/jobs_test.go` - Added TestJobLanguageField
- `cmd/server/upload_handlers.go` - Language parameter handling
- `internal/worker/worker.go` - Pass language to transcription
- `frontend/src/pages/index.astro` - Language dropdown UI
