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

## In Progress

None

## Next Up

See `TASKS.jsonl` for the full task breakdown.
