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

### Task 14: Backend - Auto-delete old files

**Date**: 2026-01-21

Implemented automatic cleanup of expired videos based on retention policy:

Database layer (`backend/db/db.go`):
- Added `GetExpiredVideos()` method to find videos past retention period
  - Anonymous videos (no user_id): expire after 48 hours
  - Registered user videos: expire after 90 days
- Added `DeleteVideo(videoID)` method to remove video and associated transcription
  - Returns file path for disk cleanup
  - Cascades deletion to transcriptions table

Cleanup scheduler (`backend/main.go`):
- Added `startCleanupScheduler()` goroutine that runs on server start
- Runs cleanup immediately on startup, then every hour
- `runCleanup()` function:
  - Finds all expired videos using `GetExpiredVideos()`
  - Deletes database records via `DeleteVideo()`
  - Removes encrypted files from disk
  - Also cleans up expired sessions via `DeleteExpiredSessions()`
  - Logs all cleanup activity for monitoring

Tests (`backend/db/db_test.go`):
- Added `TestGetExpiredVideos` - validates expiry logic for both anonymous and registered users
- Added `TestDeleteVideo` - validates cascade deletion of video and transcription
- Added `TestDeleteVideoNotFound` - validates handling of non-existent videos

### Task 15: Frontend - Subtitle editor

**Date**: 2026-01-21

Implemented in-browser subtitle editing functionality:

Backend (`backend/main.go`):
- Added `PUT /api/transcribe/{id}/segments` endpoint to save edited segments
- Validates that transcription exists and is complete before allowing edits
- Validates segment timing (no negative values, start <= end)
- Updates both segments and full_text in database

Database layer (`backend/db/db.go`):
- Added `UpdateSegments(videoID, segments)` method
- Automatically regenerates full_text by concatenating segment texts
- Updates segments_json with new segment data

Frontend (`frontend/src/pages/upload.astro`):
- Added "Edit Subtitles" button that toggles edit mode
- Edit mode UI features:
  - Inline text editing via textarea for each segment
  - Time editing via text inputs (format: HH:MM:SS.mmm)
  - Delete button to remove individual segments
  - "Add Segment" button to create new segments
  - Save/Cancel buttons for commit or discard
  - Unsaved changes indicator
- Time input parsing supports multiple formats (00:01:23.456, 1:23.456, 1:23)
- Confirmation prompts when discarding unsaved changes
- Auto-exits edit mode after successful save
- Updated full text display after save

Tests (`backend/db/db_test.go`):
- Added `TestUpdateSegments` to verify segment update functionality
- Tests segment content changes, timing updates, and adding new segments
- Verifies full_text is regenerated from segment texts

### Task 16: Backend - Transcript paste-and-match

**Date**: 2026-01-21

Implemented transcript paste-and-match feature allowing users to paste their own transcript text and have it aligned with whisper's timing information:

New package `backend/align/align.go`:
- Text alignment algorithm that maps user-provided text to whisper segment timing
- `AlignTranscript(userText, whisperSegments)` - main alignment function
- `extractWordsFromSegments` - extracts words with interpolated timing from whisper segments
- `findAlignment` - uses greedy matching with look-ahead to find optimal word alignment
- `createAlignedSegments` - creates new segments using user text with matched timing
- `levenshteinDistance` - edit distance for fuzzy word matching
- `wordSimilarity` - normalized similarity score (0-1) with case/punctuation normalization
- Handles multi-line input by preserving user's line breaks as segment boundaries
- Reports alignment statistics (match rate, word counts)

New API endpoint in `backend/main.go`:
- `POST /api/transcribe/{id}/align` - accepts user text, returns aligned segments
- Request body: `{"text": "user's transcript text"}`
- Response: `{"status": "success", "segments": N, "stats": {...}}`
- Validates transcription exists and is complete before aligning
- Converts between db.Segment and align.Segment types
- Updates database with aligned segments

Tests `backend/align/align_test.go`:
- Word normalization tests (case, punctuation)
- Word splitting tests
- Levenshtein distance tests
- Word similarity tests
- Segment timing extraction tests
- Full alignment tests (exact match, with corrections, empty inputs, multiple lines)
- Line splitting tests

Algorithm summary:
1. Extract words with interpolated timing from whisper segments
2. Split user text into lines (preserving structure as segment boundaries)
3. Greedy alignment with 10-word look-ahead and 60% similarity threshold
4. Create segments using user text with timing from matched whisper words
5. Interpolate timing for unmatched segments based on neighbors

### Task 17: Backend - Embed subtitles in video

**Date**: 2026-01-21

Implemented subtitle burning feature that hardcodes subtitles into video files using ffmpeg's subtitles filter:

Database layer (`backend/db/db.go`):
- Added `burn_jobs` table to track subtitle burning jobs
- Added `BurnJob` struct with fields: ID, VideoID, Status, Message, Progress, OutputPath, CreatedAt, CompletedAt
- Added CRUD methods: `CreateBurnJob`, `GetBurnJob`, `UpdateBurnJobStatus`, `CompleteBurnJob`, `FailBurnJob`
- Updated `DeleteVideo` to cascade delete burn jobs

New API endpoints in `backend/main.go`:
- `POST /api/videos/{id}/burn` - Start burning subtitles into video
  - Validates transcription exists and is complete
  - Decrypts video if encrypted
  - Generates SRT file from segments
  - Burns subtitles using ffmpeg with customizable styling (FontSize=24, white text, black outline)
  - Encrypts output file at rest
  - Background processing with progress updates
- `GET /api/videos/{id}/burn` - Get burn job status (status, message, progress)
- `GET /api/videos/{id}/burned` - Download the burned video
  - Decrypts on-demand for serving
  - Sets download filename based on original video name

Frontend (`frontend/src/pages/upload.astro`):
- Added "Burn Subtitles into Video" button in actions section
- Burn status panel with progress bar
- Download button appears when burn completes
- Polling mechanism for status updates during processing
- Error handling with retry capability

Tests (`backend/db/db_test.go`):
- Added `TestBurnJobLifecycle` - tests create, get, update status, complete
- Added `TestFailBurnJob` - tests error state handling
- Added `TestDeleteVideoWithBurnJob` - tests cascade deletion

ffmpeg command used:
```bash
ffmpeg -i input.mp4 -vf "subtitles='subs.srt':force_style='FontSize=24,PrimaryColour=&HFFFFFF,OutlineColour=&H000000,Outline=2'" -c:a copy -y output.mp4
```

### Task 18: Backend - 2FA with TOTP

**Date**: 2026-01-21

Implemented Google Authenticator style two-factor authentication using TOTP (Time-based One-Time Password):

New package `backend/totp/totp.go`:
- RFC 6238 compliant TOTP implementation using HMAC-SHA1
- `GenerateSecret()` - creates 20-byte cryptographically secure random secret
- `GenerateCode(secret)` / `GenerateCodeAt(secret, time)` - generates 6-digit TOTP codes
- `Validate(secret, code)` / `ValidateAt(secret, code, time)` - validates codes with 1-period window for clock drift
- `GenerateProvisioningURI(secret, email, issuer)` - creates otpauth:// URI for QR codes
- `FormatSecretForDisplay(secret)` - formats secret with spaces for readability
- 30-second time period, 6 digits (standard Google Authenticator compatible)

Database schema (`backend/db/db.go`):
- Added `totp_secret` column to users table (encrypted at rest via SQLite)
- Added `totp_enabled` boolean column to users table
- Added `SetTOTPSecret(userID, secret)` - store secret during setup
- Added `EnableTOTP(userID)` - activate 2FA after verification
- Added `DisableTOTP(userID)` - clear secret and disable 2FA

New API endpoints in `backend/main.go`:
- `POST /api/auth/totp/setup` - generates new TOTP secret, returns secret and provisioning URI
- `POST /api/auth/totp/verify` - verifies code and enables 2FA
- `POST /api/auth/totp/disable` - requires password and TOTP code, disables 2FA

Updated endpoints:
- `POST /api/auth/login` - now checks for `totp_enabled`, requires `totp_code` field when 2FA is active
  - Returns `{"totp_required": true}` when 2FA code is needed
- `GET /api/auth/me` - now includes `totp_enabled` in user response
- Registration and login responses include `totp_enabled` field

Frontend updates:
- `frontend/src/pages/login.astro` - added 2FA code input that appears when `totp_required` is returned
- `frontend/src/pages/security.astro` - new security settings page for managing 2FA
  - Setup flow with QR code (via qrserver.com API) and manual secret entry
  - Verification step to enable 2FA
  - Disable flow requiring password and current TOTP code
- `frontend/src/pages/videos.astro` - added "Security" link in nav when logged in

Tests:
- `backend/totp/totp_test.go` - comprehensive tests for TOTP generation and validation
- `backend/db/db_test.go` - added TestTOTPLifecycle and TestGetUserByEmailWithTOTP

### Task 19: Write tests

**Date**: 2026-01-21

Implemented comprehensive test coverage for both backend and frontend:

**Backend API Integration Tests (`backend/api_test.go`)**:
- New test infrastructure with `testServer` struct that creates isolated test environments
- `setupTestServer()` creates temporary database, encryptor, and upload directory
- Helper functions: `doRequest()`, `createTestUser()`, `createTestVideo()`, `createTestTranscription()`

Test cases added:
- `TestHealthEndpoint` - validates health check API
- `TestAuthRegister` - registration with valid/invalid inputs
- `TestAuthRegisterDuplicate` - duplicate email rejection
- `TestAuthLogin` - login with valid/invalid credentials
- `TestAuthLogout` - logout and session invalidation
- `TestAuthMe` - current user retrieval with/without auth
- `TestListVideos` - video listing by authenticated user
- `TestListVideosBySession` - video listing by session ID
- `TestGetTranscriptionStatus` - transcription status retrieval
- `TestGetTranscriptionNotFound` - 404 for missing transcription
- `TestDownloadSRT` - SRT file download
- `TestDownloadSRTNotComplete` - error for incomplete transcription
- `TestUpdateSegments` - segment editing
- `TestUpdateSegmentsInvalidTiming` - validation for start > end
- `TestUpdateSegmentsNegativeTiming` - validation for negative times
- `TestSessionCookie` - session cookie attributes (HttpOnly, etc.)

**Frontend Utility Modules and Tests**:

New utility modules extracted from inline page code:
- `frontend/src/utils/format.ts` - formatting functions
- `frontend/src/utils/validation.ts` - validation functions

`format.ts` functions:
- `formatBytes(bytes)` - human-readable file sizes
- `formatTime(seconds)` - SRT-style timestamps
- `formatTimeForInput(seconds)` - input field format
- `parseTimeInput(timeStr)` - parse multiple time formats
- `formatDate(dateStr)` - date formatting
- `escapeHtml(text)` - XSS prevention
- `getStatusInfo(status)` - transcription status badges

`validation.ts` functions:
- `validateEmail(email)` - email format validation
- `validatePassword(password)` - password strength
- `validateTotpCode(code)` - 6-digit TOTP validation
- `validateVideoFile(file, maxSize)` - video file type and size
- `validateSegmentTiming(start, end)` - subtitle timing validation

Tests added (`frontend/src/utils/*.test.ts`):
- 23 tests for format utilities (bytes, times, dates, HTML escaping, status badges)
- 21 tests for validation utilities (email, password, TOTP, video files, segment timing)

**Test Infrastructure**:
- Added Vitest ^3.2.0 to frontend devDependencies
- Created `vitest.config.ts` configuration
- Added `npm test` and `npm test:watch` scripts

**Total Test Count**:
- Backend: 84 tests (68 existing + 16 new integration tests)
- Frontend: 44 tests (all new)
- Total: 128 tests

### Task 20: Create TESTING.md documentation

**Date**: 2026-01-21

Created comprehensive testing documentation in `TESTING.md`:

- Overview of test coverage (128 total tests: 84 backend, 44 frontend)
- Backend test commands using Go test with verbose output and coverage
- Frontend test commands using Vitest
- Description of all 7 backend test files and their purpose
- Description of 2 frontend test files and their purpose
- Guide for writing new tests (both backend and frontend)
- Coverage report generation instructions
- CI/CD integration notes

Also updated `README.md` to:
- Add Testing section with link to TESTING.md
- Include quick test commands
- Remove Testing from TODO list

### Task 21: Create INSTALL.md documentation

**Date**: 2026-01-21

Created comprehensive installation documentation in `INSTALL.md`:

- System prerequisites (Linux/macOS)
- Node.js installation (v18+ required for Astro)
- Go installation (v1.22+ required for backend)
- FFmpeg installation (for audio extraction and subtitle burning)
- whisper-cli installation from whisper.cpp source
- Whisper model download instructions
- SQLite installation (optional, usually pre-installed)
- Environment variables table (PORT, WHISPER_MODEL)
- Quick start commands
- Troubleshooting section for common issues
- Links to related documentation

### Task 22: Create LINTERS.md documentation

**Date**: 2026-01-21

Created linting guide documentation in `LINTERS.md`:

- Go linting with golangci-lint installation and usage
- Built-in Go tools (go fmt, go vet)
- Optional golangci.yml configuration example
- ESLint setup for TypeScript/Astro
- Prettier setup for code formatting
- Package.json script additions
- Pre-commit hooks setup with Husky
- Quick reference table of all linting commands
- Links to related documentation

### Task 23: Create BROWSER_TESTING.md documentation

**Date**: 2026-01-21

Created browser automation testing guide in `BROWSER_TESTING.md`:

- Overview of E2E testing benefits
- Playwright installation and configuration
- playwright.config.ts with multi-browser support
- Test directory structure
- Example tests for homepage, upload flow, and authentication
- Test running commands (headed, UI, debug modes)
- Test fixtures (creating test video with ffmpeg)
- Database reset for consistent tests
- GitHub Actions CI/CD integration example
- Debugging failed tests (reports, traces, screenshots)
- Alternative: Cypress brief overview
- Quick reference command table
- Links to related documentation

### Task 24: Frontend - Console log forwarding to backend

**Date**: 2026-01-21

Implemented console log forwarding from frontend to backend for easier debugging during development (as specified in the architecture requirements in specs/subtitler.md):

Backend (`backend/main.go`):
- Added `POST /api/log` endpoint to receive console messages from frontend
- Accepts JSON payload with: `level`, `message`, `url`, `line`, `column`
- Validates log levels (log, warn, error, info, debug) with fallback to "log"
- Formats and outputs received logs with `[FRONTEND]`, `[FRONTEND ERROR]`, etc. prefixes
- Includes source URL and line/column info when available

Frontend (`frontend/src/utils/console-forwarder.ts`):
- `installConsoleForwarder(isDev)` - installs interceptors for console.log/warn/error/info/debug
- Only activates when `isDev` is true (development mode only)
- Preserves original console behavior - messages still appear in browser console
- Formats arguments as strings (handles objects, arrays, errors, undefined, circular refs)
- Captures unhandled errors and promise rejections via window event listeners
- Queue system prevents infinite loops when logging during send
- `uninstallConsoleForwarder()` restores original console methods

Astro component (`frontend/src/components/DevConsoleForwarder.astro`):
- Simple component to include in pages that need log forwarding
- Uses `import.meta.env.DEV` to only enable in development builds
- Included in all 6 pages: index, upload, videos, login, register, security

Tests:
- Backend: `TestLogEndpoint` (7 subtests) and `TestLogEndpointInvalidBody` in `api_test.go`
- Frontend: 20 tests in `console-forwarder.test.ts` covering formatArgs, installation, interception

**Total Test Count Update**:
- Backend: 86 tests (84 + 2 new)
- Frontend: 64 tests (44 + 20 new)
- Total: 150 tests

### Task 25: Implement BROWSER_TESTING.md setup

**Date**: 2026-01-22

Implemented the E2E testing infrastructure that was documented but never created:

**Playwright Setup**:
- Installed `@playwright/test` and Chromium browser
- Created `playwright.config.ts` with:
  - Test directory: `./e2e`
  - Base URL: `http://localhost:4321`
  - Screenshot on failure, trace on retry
  - WebServer configuration to auto-start backend (with full Go path) and frontend

**Test Scripts** (`package.json`):
- `npm run test:e2e` - Run all E2E tests
- `npm run test:e2e:ui` - Run with interactive UI
- `npm run test:e2e:headed` - Run with visible browser
- `npm run test:e2e:debug` - Run in debug mode

**E2E Tests** (`e2e/home.spec.ts`):
- 6 passing tests for the homepage:
  - Title verification
  - Hero section with tagline
  - Navigation buttons presence
  - Navigate to upload page
  - Navigate to videos page
  - Auth links for unauthenticated users

**Verification**:
- Ran `npm run test:e2e` - all 6 tests pass
- Ran `npm test` - all 64 unit tests still pass

This closes the gap identified in LEARNINGS.md where Task 23 documented testing but never implemented it.

### Task 26: Create specs for major features

**Date**: 2026-01-22

Created comprehensive design documentation for the three major security features:

**specs/auth.md** - Authentication System Specification:
- Database schema for users and sessions tables
- All authentication API endpoints (register, login, logout, me)
- Session management with cookie and Bearer token support
- Password security (bcrypt cost 12)
- Video upload limits (anonymous: 2 uploads/48hr, registered: unlimited/90 days)
- Security considerations (implemented and not implemented)
- Frontend validation utilities

**specs/encryption.md** - File Encryption Specification:
- `age` library (filippo.io/age) with X25519 + ChaCha20-Poly1305
- Encryptor type and all methods
- Key management (data/age.key with 0600 permissions)
- Encryption flow for upload, transcription, serving, and burning
- Database integration (file_path stores .age extension)
- Error handling and security characteristics
- Test coverage documentation

**specs/totp.md** - TOTP 2FA Specification:
- Pure Go RFC 6238 compliant implementation
- Configuration (20 bytes secret, 6 digits, 30s period, ±1 window)
- API endpoints (setup, verify, disable)
- Login integration with two-step flow
- Algorithm details (HMAC-SHA1 with time-based counter)
- Provisioning URI format for authenticator apps
- Frontend implementation notes
- Recovery mechanism gap identified (not implemented)

All specs cross-link to each other for easy navigation.

### Task 28: Fix upload progress and SRT download

**Date**: 2026-01-22

Fixed two UX issues with upload flow:

**Upload Progress Fix:**
- Problem: Progress bar stuck at 30% during whisper transcription
- Root cause: Whisper subprocess provides no progress callbacks
- Solution: Added background goroutine that simulates progress updates every 5 seconds during transcription (30% → 90%)
- Uses a channel to signal when whisper completes

**SRT Download Fix:**
- Problem: Used non-standard `Content-Type: text/srt` MIME type
- Solution: Changed to `text/plain; charset=utf-8` for universal browser support
- The `Content-Disposition: attachment` header controls download behavior

Updated tests to match new Content-Type.

### Task 31: Fill in TODOs in specs/subtitler.md

**Date**: 2026-01-22

Addressed all TODO items in the main spec file:

**Completed directly:**
- Added accuracy/speed evaluation plan with WER benchmarks
- Researched and documented competitors (Rev, Happy Scribe, GoTranscript, Otter.ai, etc.)
- Added competitor pricing, accuracy, and speed comparison table
- Added competitive advantages and improvement areas

**Converted to explicit tasks:**
- Task 32: Document whisper-server passthrough for qemu VM
- Task 33: Write deployment plan for Mac Mini qemu VM
- Task 34: Research Mac Metal via MoltenVK to qemu
- Task 35: Plan webhook-deployer integration

The spec file now has zero TODO markers - all are either completed or tracked as tasks.

### Task 30: Create verification scripts and pre-commit hook

**Date**: 2026-01-22

Created helper scripts in `scripts/` directory for development workflow:

**Scripts created:**
- `lint.sh` - Run Go fmt/vet and frontend build
- `test-backend.sh` - Run backend Go tests
- `test-frontend.sh` - Run frontend Vitest tests
- `test-e2e.sh` - Run Playwright E2E tests
- `verify-all.sh` - Run lint + all unit tests
- `pre-commit` - Git pre-commit hook

**Pre-commit hook:**
- Runs lint and unit tests before each commit
- Skips E2E tests (too slow for pre-commit)
- Install with: `ln -sf ../../scripts/pre-commit .git/hooks/pre-commit`

Updated AGENTS.md with script documentation and usage table.

### Task 29: Audit - Compare capabilities vs specs/docs

**Date**: 2026-01-22

Performed systematic audit of all documentation against actual implementation.

**Verification completed:**
- INSTALL.md: All commands work (Node 22+, Go 1.22+, ffmpeg, whisper-cli)
- TESTING.md: All commands work (86 backend + 64 frontend tests pass)
- BROWSER_TESTING.md: E2E tests exist and configured (6 tests in home.spec.ts)
- LINTERS.md: Go fmt/vet work; ESLint documented but intentionally not installed (guide only)
- Backend API: All documented endpoints implemented

**Bug fixed:**
- `scripts/lint.sh` had relative path bug: after `cd` to backend, `cd "$PROJECT_ROOT/frontend"` failed
- Fixed by making `SCRIPT_DIR` absolute via `$(cd "$(dirname "$0")" && pwd)`

**Gaps identified:**
- No `backend/README.md` for env var documentation (covered by Task 32)
- LINTERS.md shows ESLint setup but it's not installed (intentional - guide for optional setup)

Added learning to LEARNINGS.md about shell script cd with relative paths.

### Task 27: Backend - Switch from whisper-cli to whisper-server

**Date**: 2026-01-22

Implemented whisper-server HTTP API integration as an alternative to spawning whisper-cli:

**New functions in `backend/main.go`:**
- `getWhisperServerURL()` - returns whisper-server URL from env or default `http://127.0.0.1:8765`
- `isWhisperServerEnabled()` - checks if server mode should be used
- `transcribeAudioServer(audioPath)` - sends audio via multipart POST to `/inference`
- `transcribe(audioPath, outputPath)` - dispatcher that chooses server or CLI based on config

**Server Mode Features:**
- Sends WAV audio file to whisper-server `/inference` endpoint
- Uses `response_format=verbose_json` to get segments with timing
- Parses response into existing `WhisperResult` struct
- 30-minute timeout for long transcriptions
- Falls back to CLI mode when server not configured

**Environment Variables:**
- `WHISPER_SERVER_URL` - URL of whisper-server (enables server mode when set)
- `USE_WHISPER_SERVER=true` - enables server mode with default URL

**Documentation:**
- Updated `INSTALL.md` with whisper-server build/run instructions
- Updated environment variables table with new options
- Added mode selection explanation

**Benefits of Server Mode:**
- Model stays loaded in memory (faster transcriptions)
- No subprocess spawning overhead
- Supports remote transcription (GPU server)

### Task 32: Document whisper-server passthrough for qemu VM

**Date**: 2026-01-22

Created comprehensive `backend/README.md` documentation:

**Environment Variables:**
- Complete table of all backend env vars (PORT, WHISPER_MODEL, WHISPER_SERVER_URL, USE_WHISPER_SERVER)
- Explanation of CLI vs Server mode selection
- Example configurations for both modes

**qemu VM Networking:**
- Option 1: User-mode networking with hostfwd (recommended for simplicity)
  - VM accesses host at `10.0.2.2` (default qemu gateway)
- Option 2: Bridge networking for advanced setups
  - Create br0 bridge, assign IPs, configure VM
- Option 3: macvtap for production (VM gets LAN IP)

**Additional Documentation:**
- Firewall configuration (ufw rules)
- Connectivity verification commands
- Full API endpoint reference table
- Project structure overview
- Testing commands
- Links to related specs and docs

### Task 33: Write deployment plan

**Date**: 2026-01-22

Created comprehensive deployment specification in `specs/deployment.md`:

**Architecture:**
- ASCII diagram showing Mac Mini host with qemu VM
- VM runs Caddy + Go backend
- Host runs whisper-server (GPU-accelerated)
- Network: User-mode with port forwarding

**Component Documentation:**
- qemu VM specs and launch command (hvf acceleration, virtio devices)
- Caddy configuration (static files, API proxy, TLS, security headers)
- Go backend systemd service file
- whisper-server launchd plist for macOS

**Operational Guides:**
- Directory structure for deployment
- Initial setup steps
- Update/deployment workflow
- SSL/TLS certificate handling
- Health check commands
- Log locations
- Backup strategy (database + encryption key)

**Security Considerations:**
- Firewall configuration
- Backend isolation
- File encryption
- Permission hardening

### Task 34: Research Mac Metal via MoltenVK to qemu

**Date**: 2026-01-22

Created research document `specs/metal-moltenvk.md`:

**Key Finding: Not feasible with current technology.**

**Research Covered:**
- MoltenVK capabilities (Vulkan→Metal translation, but HOST-side only)
- qemu GPU options on macOS (virtio-gpu, vmsvga - no Metal support)
- Why passthrough doesn't exist:
  - No Apple SR-IOV/IOMMU for GPUs
  - Metal tightly coupled to macOS kernel
  - Apple Silicon unified memory architecture
  - HVF doesn't support PCIe passthrough

**Alternative Approaches Evaluated:**
1. Host-based whisper-server (Recommended - already implemented)
2. Remote GPU server with NVIDIA CUDA
3. Cloud GPU on-demand (AWS, GCP, RunPod)
4. Apple Neural Engine via Core ML

**Technical Deep Dive:**
- MoltenVK architecture diagram showing why it can't help
- whisper.cpp GPU backend status table
- Comparison of passthrough requirements vs available tech

**Verdict:** Continue with current architecture (whisper-server on host). GPU passthrough is unlikely to be supported by Apple due to their security model.

### Task 35: Plan webhook-deployer integration

**Date**: 2026-01-22

Created comprehensive integration plan in `specs/webhook-deployer.md`:

**Current State Analysis:**
- Reviewed existing webhook-deployer code
- Identified single-site limitation
- Documented GitHub webhook payload structure

**Multi-Site Architecture:**
- Site configuration struct with path, branch, repository
- Config file (`config.yaml`) for managing multiple sites
- Path-based routing using GitHub commit file lists
- Extended `GitHubPushEvent` to include commit details

**Subtitler Deployment Plans:**

Frontend deploy:
- `npm ci && npm run build`
- `rsync` to `/var/www/subtitler/`

Backend deploy:
- `go build` to temp location
- Atomic binary swap
- `systemctl restart` via sudoers entry
- Health check verification

**Implementation Phases:**
1. Multi-site support in webhook-deployer (2-3 hours)
2. Deploy scripts for subtitler (1 hour)
3. Integration and testing (1-2 hours)
4. Monitoring and notifications (optional, 2-3 hours)

**Rollback Strategy:**
- Keep N backups of frontend builds
- Keep N versions of backend binary
- Simple restoration commands

### Task 36: Add debugging documentation to README

**Date**: 2026-01-22

Added debugging section to README.md covering:

**Go Backend (Delve):**
- Installation command (`go install github.com/go-delve/delve/cmd/dlv@latest`)
- Running with debugger (`dlv debug .`)
- VS Code launch.json configuration for F5 debugging
- Remote debugging setup (headless mode on port 2345)

**Frontend (TypeScript):**
- Browser DevTools instructions
- VS Code launch.json for Chrome debugging
- Console log forwarding explanation (dev mode feature)

This resolves the final TODO item in README.md.

### Task 37: Backend - 2FA recovery codes

**Date**: 2026-01-22

Implemented recovery codes for 2FA to prevent users from being permanently locked out of their accounts:

**New files:**
- `backend/totp/recovery.go` - Recovery code generation and hashing
- `backend/totp/recovery_test.go` - Unit tests for recovery code functions
- `specs/recovery-codes.md` - Feature specification

**Database changes:**
- Added `recovery_codes` table with user_id, code_hash, used, created_at, used_at
- Added index on user_id for efficient lookups

**API changes:**
- `POST /api/auth/totp/verify` now returns 10 recovery codes when 2FA is enabled
- New `POST /api/auth/totp/recover` endpoint for account recovery using a code
- `POST /api/auth/totp/disable` now deletes recovery codes

**Recovery code features:**
- 10 single-use codes generated when 2FA is enabled
- Format: XXXX-XXXX (8 chars, uppercase + digits, no ambiguous chars)
- Stored as bcrypt hashes (cost 10)
- Recovery requires email + password + valid code
- Using a code disables 2FA and clears all sessions

**Tests added:**
- 8 unit tests in `totp/recovery_test.go`
- 3 DB tests in `db/db_test.go` (lifecycle, regenerate, delete)
- 5 API integration tests (verify returns codes, valid/invalid recovery, single-use)

### Task 38: Backend - Rate limiting for auth endpoints

**Date**: 2026-01-22

Implemented IP-based rate limiting for authentication endpoints to protect against brute force attacks:

**New files:**
- `backend/ratelimit/ratelimit.go` - Rate limiter implementation with sliding window
- `backend/ratelimit/ratelimit_test.go` - Unit tests for rate limiter

**Rate limiter features:**
- Sliding window algorithm: 5 requests per minute per IP
- Supports X-Forwarded-For and X-Real-IP headers for proxy environments
- Automatic cleanup of stale entries (every 5 minutes)
- Thread-safe using sync.RWMutex
- Returns 429 Too Many Requests with Retry-After header when exceeded

**Endpoints protected:**
- `POST /api/auth/register`
- `POST /api/auth/login`
- `POST /api/auth/totp/verify`
- `POST /api/auth/totp/recover`

**Tests added:**
- 10 unit tests in `ratelimit/ratelimit_test.go` (limiter, GetClientIP, Wrap middleware)
- 5 API integration tests in `api_test.go` (login, register, TOTP verify, TOTP recover, X-Forwarded-For)

## Summary

38 tasks completed. The subtitler application is feature-complete with:
- Video upload and transcription with Whisper AI
- Subtitle generation, viewing, editing, and downloading (SRT format)
- Subtitle burning into video files
- User authentication with optional 2FA (TOTP) and recovery codes
- Rate limiting on auth endpoints (5 req/min per IP)
- File encryption at rest
- Anonymous and registered user support with retention policies
- Frontend console log forwarding to backend for dev debugging
- Comprehensive test coverage (175+ tests including 6 E2E tests)
- Complete documentation (INSTALL.md, TESTING.md, LINTERS.md, BROWSER_TESTING.md)
- Working E2E test infrastructure with Playwright
