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

### Task 10: Frontend - List uploaded videos

**Date**: 2026-01-21

Implemented video listing page and API endpoint:

Backend (`backend/main.go`):
- Added `GET /api/videos` endpoint to list all videos
- Returns videos with their transcription status
- Supports optional `session_id` query param for filtering (for future anonymous session support)
- Uses existing `ListVideos` function from database layer

Frontend:
- Created new `/videos` page at `frontend/src/pages/videos.astro`
- Shows list of uploaded videos with filename, size, date, and transcription status
- Status badges (Complete, Processing, Pending, Error, Not Started)
- Action buttons: View (for completed), Download SRT (for completed), Transcribe (for pending)
- Empty state with call-to-action to upload first video
- Updated homepage (`index.astro`) with "My Videos" button

### Task 8: Backend - File encryption at rest

**Date**: 2026-01-21

Implemented file encryption at rest using the `age` encryption library (filippo.io/age):

New package `backend/crypto/crypto.go`:
- `Encryptor` type wraps age identity (private key) and recipient (public key)
- `NewEncryptor(privateKey)` creates encryptor from existing key or generates new one
- `LoadOrGenerateKey(keyPath)` loads key from file or generates and saves new key
- `EncryptFile(srcPath)` encrypts a file, returns path to `.age` encrypted file
- `DecryptFile(encPath)` decrypts a `.age` file, returns plaintext bytes
- `DecryptToTempFile(encPath)` decrypts to temp file (for ffmpeg/http.ServeFile)
- `EncryptBytes/DecryptBytes` for in-memory operations
- Thread-safe with mutex protection

Comprehensive tests in `backend/crypto/crypto_test.go`:
- Key generation and parsing tests
- Encrypt/decrypt round-trip tests
- Large file (1MB) encryption test
- File and temp file operations
- Key persistence (load/generate) tests
- Wrong key rejection test

Updated `backend/main.go`:
- Initializes encryptor on startup from `data/age.key`
- Generates new key if none exists, logs public key
- Upload handler encrypts files after saving, stores `.age` path in database
- Transcription handler decrypts video to temp file before processing
- Video serving endpoint decrypts to temp file for streaming

Environment:
- Encryption key stored at `data/age.key`
- Key file format includes public key comment for reference
- Encrypted files stored as `uploads/{id}.{ext}.age`

### Task 11: Backend - Authentication system

**Date**: 2026-01-21

Implemented email/password authentication with session management:

Database schema (`backend/db/db.go`):
- Added `users` table: id, email (unique), password_hash, created_at
- Added `sessions` table: id, user_id, token (unique), expires_at, created_at
- Added indexes for efficient lookups

New package `backend/auth/auth.go`:
- `HashPassword(password)` - bcrypt hashing with cost 12
- `CheckPassword(password, hash)` - bcrypt comparison
- `GenerateToken()` - 32-byte cryptographically secure token
- `GenerateID()` - 16-byte random ID generation
- `ValidateEmail(email)` - basic email format validation
- `ValidatePassword(password)` - min 8 chars, max 72 (bcrypt limit)
- `CreateSession(db, userID)` - creates 7-day session
- `ValidateSession(db, token)` - validates token, returns user and session
- `GetTokenFromRequest(r)` - extracts token from Authorization header or cookie
- `SetSessionCookie/ClearSessionCookie` - cookie management

New API endpoints in `backend/main.go`:
- `POST /api/auth/register` - create new user account, returns session token
- `POST /api/auth/login` - authenticate and get session token
- `POST /api/auth/logout` - invalidate session
- `GET /api/auth/me` - get current authenticated user

Updated endpoints:
- `POST /api/upload` - now extracts user_id from session and stores with video
- `GET /api/videos` - filters by authenticated user_id when logged in

Comprehensive tests in `backend/auth/auth_test.go`:
- Password hashing and verification tests
- Token and ID generation tests
- Email and password validation tests
- Session lifecycle tests (create, validate, delete)
- User lifecycle tests (create, get by ID, get by email)

### Task 12: Frontend - Auth pages

**Date**: 2026-01-21

Implemented frontend authentication pages and navigation:

New pages:
- `frontend/src/pages/login.astro` - Login form with email/password
- `frontend/src/pages/register.astro` - Registration form with email/password/confirm

Features:
- Clean, centered card design matching Subtitler's aesthetic
- Client-side form validation
- Error message display for failed login/registration
- Loading states on submit buttons
- Automatic redirect to /videos on successful login/registration
- Links between login and register pages

Updated existing pages with auth navigation:
- `frontend/src/pages/index.astro` - Added nav bar with Log in/Sign up links
- `frontend/src/pages/videos.astro` - Added nav bar with auth state
- `frontend/src/pages/upload.astro` - Added nav bar with auth state

Auth navigation behavior:
- When logged out: Shows "Log in" and "Sign up" links
- When logged in: Shows user email and "Log out" button
- Logout clears session and reloads page

### Task 13: Backend - Upload limits for anonymous users

**Date**: 2026-01-21

Implemented upload limits for anonymous (unauthenticated) users:

Database layer (`backend/db/db.go`):
- Added `CountVideosBySession(sessionID)` method to count uploads by session ID
- Used to check upload count before allowing new uploads

Backend (`backend/main.go`):
- Added upload limit check in `/api/upload` endpoint
- Anonymous users (no auth token) limited to 2 uploads per session
- Returns 403 Forbidden with message prompting registration when limit reached
- Authenticated users have no upload limit

Tests (`backend/db/db_test.go`):
- Added `TestCountVideosBySession` to verify count functionality
- Tests initial 0 count, incrementing count, and isolation between sessions

## In Progress

None

## Next Up

See `TASKS.jsonl` for the full task breakdown.
