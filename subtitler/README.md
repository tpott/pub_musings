# Subtitler

A web-based subtitling service that allows users to upload video/audio files and receive transcribed subtitles or files with embedded subtitles. Built using whisper.cpp for transcription.

## Overview

Subtitler provides:
- Automatic transcription of audio and video files using whisper.cpp
- Multiple output formats (SRT, VTT, embedded video)
- User accounts and job history
- Background processing with notifications

## Tech Stack

- **Frontend**: Astro (TypeScript)
- **Backend**: Go
- **Transcription**: whisper.cpp (medium model)
- **Database**: SQLite
- **File Storage**: Local, encrypted with age
- **Email**: Resend API

## Project Structure

```
subtitler/
├── frontend/           # Astro web application
│   ├── src/
│   └── package.json
├── backend/            # Go API server
│   ├── cmd/server/     # Main server entry point
│   ├── internal/       # Internal packages
│   │   ├── auth/       # Authentication logic
│   │   ├── transcribe/ # Transcription integration
│   │   └── storage/    # File storage
│   └── go.mod
├── v1/                 # Previous Python implementation (archived)
└── deploy/             # Deployment configurations (future)
```

## Prerequisites

Before running Subtitler, you need:
- Go 1.22.10 or higher (installed at `/home/trevor/go/bin/go`)
- Node.js 18.x or higher
- whisper.cpp with whisper-server built
- whisper.cpp medium model downloaded
- ffmpeg (for audio format conversion)

See [INSTALL.md](INSTALL.md) for detailed setup instructions.

## Getting Started

Quick start:
```bash
# Frontend
cd frontend && npm install && npm run dev

# Backend (in separate terminal)
cd backend && /home/trevor/go/bin/go run ./cmd/server
```

On first run, the backend will:
- Initialize the SQLite database at `./data/db/subtitler.db`
- Run database migrations automatically
- Create file storage directories at `./data/files/uploads/` and `./data/files/results/`

## API Endpoints

### POST /api/upload
Upload a file to the server for validation.

**Example:**
```bash
curl -X POST http://localhost:8080/api/upload \
  -F "file=@/path/to/audio.mp3"
```

### POST /api/transcribe
Upload an audio/video file and receive SRT-formatted subtitles.

**Example:**
```bash
curl -X POST http://localhost:8080/api/transcribe \
  -F "file=@~/Github/whisper.cpp/samples/jfk.wav" \
  -o output.srt
```

**Response format (JSON):**
```json
{
  "success": true,
  "message": "Transcription completed successfully",
  "transcript": "1\n00:00:00,000 --> 00:00:05,000\nAnd so my fellow Americans...",
  "filename": "jfk.wav",
  "format": "srt",
  "duration": 15.5
}
```

**Supported formats:**
- Audio: mp3, wav, m4a, ogg, flac
- Video: mp4, webm, mkv, avi, mov
- Max file size: 200MB

### POST /api/register
Register a new user account.

**Example:**
```bash
curl -X POST http://localhost:8080/api/register \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"yourpassword"}' \
  -c cookies.txt
```

**Response:**
```json
{
  "success": true,
  "message": "User registered successfully",
  "user": {"id": 1, "email": "user@example.com"}
}
```

Sets a JWT token in an HTTP-only cookie named `subtitler_token` (expires in 7 days).

### POST /api/login
Login with email and password.

**Example:**
```bash
curl -X POST http://localhost:8080/api/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"yourpassword"}' \
  -c cookies.txt
```

**Response:**
```json
{
  "success": true,
  "message": "Login successful",
  "user": {"id": 1, "email": "user@example.com"}
}
```

Sets a JWT token in an HTTP-only cookie named `subtitler_token` (expires in 7 days).

### POST /api/logout
Logout and clear session cookie.

**Example:**
```bash
curl -X POST http://localhost:8080/api/logout \
  -b cookies.txt
```

**Response:**
```json
{
  "success": true,
  "message": "Logged out successfully"
}
```

### GET /api/me
Get current authenticated user (protected endpoint).

**Example:**
```bash
curl -X GET http://localhost:8080/api/me \
  -b cookies.txt
```

**Response:**
```json
{
  "success": true,
  "user": {"id": 1, "email": "user@example.com"}
}
```

Returns 401 Unauthorized if not authenticated.

## Running Tests

### Unit Tests
```bash
# Backend unit tests
cd backend && /home/trevor/go/bin/go test ./internal/config -v
cd backend && /home/trevor/go/bin/go test ./internal/storage -v
cd backend && /home/trevor/go/bin/go test ./internal/db -v
cd backend && /home/trevor/go/bin/go test ./internal/auth -v

# Run all backend tests (short mode, skips integration tests)
cd backend && /home/trevor/go/bin/go test ./... -short

# Backend server endpoint tests
cd backend && /home/trevor/go/bin/go test ./cmd/server -v

# Test transcribe endpoint specifically (integration test, requires whisper.cpp)
cd backend && /home/trevor/go/bin/go test ./cmd/server -v -run TestHandleTranscribe_Integration
```

### Integration Tests
```bash
# Run integration tests (requires whisper.cpp setup)
cd backend && /home/trevor/go/bin/go test ./internal/transcribe -v

# See INTEGRATION_TESTS.md for more details
```

### Frontend Tests
```bash
# Playwright tests (requires frontend and backend to be running)
cd frontend && npm test

# Run Playwright tests in headed mode (visible browser)
cd frontend && npm run test:headed
```

## Documentation

- [INSTALL.md](INSTALL.md) - Setup and installation guide
- [INTEGRATION_TESTS.md](INTEGRATION_TESTS.md) - Integration test documentation
- [001_RALPH_SUBTITLER.md](001_RALPH_SUBTITLER.md) - Architecture and planning
- [TASKS.jsonl](TASKS.jsonl) - Task tracking

## Development

This project is developed using the Ralph Wiggum autonomous agent loop. See PROMPT.md for more details.

## License

TBD
