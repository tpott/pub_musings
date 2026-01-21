# Progress

## Completed

### Task 1: Initialize project scaffolding

**Date**: 2026-01-21

Created the basic project structure with:

- **Frontend (Astro)**: Located in `frontend/`
  - Basic home page with project branding
  - Upload page with drag-and-drop video upload UI
  - Proxy configured to forward `/api/*` to backend
  - Runs on http://localhost:4321

- **Backend (Go)**: Located in `backend/`
  - Basic HTTP server with health check endpoint (`/api/health`)
  - Placeholder upload endpoint (`/api/upload`)
  - Runs on http://localhost:8080

### Task 2: Backend - Accept video file uploads

**Date**: 2026-01-21

Implemented video file upload handling in `backend/main.go`:

- Accepts multipart form data with video file (`video` field)
- Validates content type is `video/*`
- Enforces 500 MB max file size
- Generates unique upload ID using crypto/rand
- Saves files to `backend/uploads/` directory
- Returns JSON response with `upload_id`, `filename`, `size`

### Task 3: Frontend - Complete upload flow

**Date**: 2026-01-21

Updated `frontend/src/pages/upload.astro` to:

- Actually send video files to backend `/api/upload`
- Show real-time upload progress with progress bar
- Display success/error states with appropriate styling
- Prevent duplicate uploads while one is in progress
- Validate file size on client side (500 MB limit)

### Task 4: Backend - Integrate whisper-cli

**Date**: 2026-01-21

Implemented transcription functionality in `backend/main.go`:

- Added `POST /api/transcribe/{id}` endpoint to start transcription
- Added `GET /api/transcribe/{id}` endpoint to poll status/get results
- Uses ffmpeg to extract audio from video (16kHz mono WAV)
- Invokes whisper-cli with JSON output mode
- Parses whisper JSON to extract segments with timestamps
- Background processing with status updates (pending, processing, complete, error)
- Configurable model via `WHISPER_MODEL` env var (defaults to ggml-medium.bin)

Updated `frontend/src/pages/upload.astro` to:

- Automatically start transcription after upload completes
- Poll backend for transcription status every 2 seconds
- Display transcription progress and status
- Show completed transcription with:
  - Full transcript text
  - Segments with timestamps (formatted HH:MM:SS.mmm)

### Task 5: Backend - Generate SRT files

**Date**: 2026-01-21

Implemented SRT file generation in `backend/main.go`:

- Added `formatSRTTimestamp` function to convert seconds to SRT format (HH:MM:SS,mmm)
- Added `generateSRT` function to convert WhisperResult segments to valid SRT format
- Added `GET /api/videos/{id}/subtitles.srt` endpoint for downloading SRT files
- Returns proper Content-Type (`text/srt; charset=utf-8`) and Content-Disposition headers
- Handles edge cases: missing transcription, incomplete transcription, empty segments

Added unit tests in `backend/main_test.go`:
- `TestFormatSRTTimestamp` - validates timestamp formatting
- `TestGenerateSRT` - validates full SRT generation
- `TestGenerateSRTEmpty` - validates empty segment handling

### Task 6: Frontend - Subtitle viewer

**Date**: 2026-01-21

Implemented synced subtitle viewer in `frontend/src/pages/upload.astro`:

- Added video serving endpoint `GET /api/videos/{id}/video` in backend
- Current subtitle display: Shows active subtitle text in a dark overlay during playback
- Clickable segments: Click any segment to seek to that timestamp
- Active segment highlighting: Currently playing segment is highlighted with blue border
- Auto-scroll: Segment list auto-scrolls to keep active segment visible
- Download SRT button: Allows downloading the generated SRT file

### Task 7: Backend - SQLite database

**Date**: 2026-01-21

Implemented SQLite database for persistent storage in `backend/db/db.go`:

Database schema:
- `videos` table: stores uploaded video metadata (id, filename, size, content_type, file_path, created_at, user_id, session_id)
- `transcriptions` table: stores transcription jobs and results (id, video_id, status, message, progress, language, duration, full_text, segments_json, created_at, completed_at)

Features:
- Automatic migration on startup
- WAL mode for better concurrent access
- JSON serialization for segments
- Support for listing videos by user or session

Updated handlers in `backend/main.go`:
- Upload handler saves video records to database
- Transcription handlers read/write from database
- SRT download uses database for transcription lookup

Added comprehensive tests in `backend/db/db_test.go`:
- Database open/migrate tests
- Video CRUD tests
- Transcription lifecycle tests (create, update status, complete, fail)
- List videos tests

Database file stored at `data/subtitler.db`

## In Progress

None

## Next Up

See `TASKS.jsonl` for the full task breakdown.
