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

### Configuration

The backend supports the following environment variables:

**Email Configuration (optional):**
```bash
RESEND_API_KEY=re_xxxxx         # Resend API key for email notifications
EMAIL_FROM=noreply@example.com  # Sender email address
ENABLE_EMAIL=false              # Set to true to enable email notifications
```

**Other Configuration:**
- `DATABASE_PATH` - Database file path (default: `./data/db/subtitler.db`)
- `DATA_DIR` - Data directory path (default: `./data`)
- `JWT_SECRET` - JWT signing secret (default: `dev-secret-change-in-production`)
- `SERVER_PORT` - Server port (default: `8080`)
- `FRONTEND_URL` - Frontend URL for CORS (default: `http://localhost:4321`)
- `WHISPER_MODEL_PATH` - Path to whisper model (default: `$HOME/Github/whisper.cpp/models/ggml-medium.bin`)
- `WHISPER_SERVER_PATH` - Path to whisper-server binary (default: `$HOME/Github/whisper.cpp/build/bin/whisper-server`)

**Email Notifications:**

When `ENABLE_EMAIL=true`, users will receive email notifications when:
- A transcription job completes successfully
- A transcription job fails

To enable email notifications:
1. Sign up for a Resend account at https://resend.com
2. Get your API key from the Resend dashboard
3. Set environment variables:
   ```bash
   export RESEND_API_KEY=re_xxxxx
   export EMAIL_FROM=noreply@yourdomain.com
   export ENABLE_EMAIL=true
   ```
4. Restart the backend server

**Note:** Resend free tier includes 100 emails/day, 3000 emails/month.

## API Endpoints

### GET /api/health
Health check endpoint for monitoring and tunnel verification.

**Example:**
```bash
curl http://localhost:8080/api/health
```

**Response:**
```json
{
  "success": true,
  "message": "Service is healthy",
  "version": "1.0.0"
}
```

### POST /api/upload (Protected)
Upload a file for background transcription. Requires authentication.

The file is saved and a job is created with status='pending'. A worker processes the job asynchronously.

**Example:**
```bash
# Upload with default format (SRT)
curl -X POST http://localhost:8080/api/upload \
  -H "Cookie: subtitler_token=YOUR_JWT_TOKEN" \
  -F "file=@/path/to/audio.mp3"

# Upload with specific format
curl -X POST http://localhost:8080/api/upload \
  -H "Cookie: subtitler_token=YOUR_JWT_TOKEN" \
  -F "file=@/path/to/video.mp4" \
  -F "format=embedded"
```

**Supported formats:**
- `srt` - SubRip subtitle format (default)
- `vtt` - WebVTT subtitle format
- `text` - Plain text transcript
- `json` - JSON format with timestamps
- `embedded` - Video file with burned-in subtitles (MP4 output)

**Response:**
```json
{
  "success": true,
  "message": "File uploaded successfully",
  "job_id": 123
}
```

Use `/api/jobs/{id}` to check job status and `/api/jobs/{id}/download` to download the result when complete.

### POST /api/upload-old
Upload a file to the server for validation (old endpoint, unauthenticated).

**Example:**
```bash
curl -X POST http://localhost:8080/api/upload-old \
  -F "file=@/path/to/audio.mp3"
```

### POST /api/transcribe
Upload an audio/video file and receive transcribed subtitles synchronously.

**Example:**
```bash
# Default format (SRT)
curl -X POST http://localhost:8080/api/transcribe \
  -F "file=@~/Github/whisper.cpp/samples/jfk.wav" \
  -o output.srt

# VTT format
curl -X POST "http://localhost:8080/api/transcribe?format=vtt" \
  -F "file=@~/Github/whisper.cpp/samples/jfk.wav" \
  -o output.vtt

# Plain text
curl -X POST "http://localhost:8080/api/transcribe?format=text" \
  -F "file=@~/Github/whisper.cpp/samples/jfk.wav"
```

**Supported output formats (via query parameter):**
- `srt` - SubRip subtitle format (default)
- `vtt` - WebVTT subtitle format
- `text` - Plain text transcript
- `json` - JSON format with timestamps

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

**Supported input formats:**
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

### GET /api/jobs
Get all transcription jobs for the authenticated user (protected endpoint).

**Example:**
```bash
curl -X GET http://localhost:8080/api/jobs \
  -b cookies.txt
```

**Response:**
```json
{
  "success": true,
  "jobs": [
    {
      "id": 1,
      "user_id": 1,
      "status": "completed",
      "original_filename": "video.mp4",
      "file_path": "/data/files/uploads/1/1/video.mp4",
      "file_size": 5242880,
      "output_format": "srt",
      "transcript_path": "/data/files/results/1/1/video.srt",
      "created_at": "2026-01-18T10:00:00Z",
      "updated_at": "2026-01-18T10:05:00Z",
      "completed_at": "2026-01-18T10:05:00Z"
    }
  ]
}
```

Jobs are returned in descending order by creation date (newest first).

### GET /api/jobs/{id}
Get a specific job by ID (protected endpoint, must own the job).

**Example:**
```bash
curl -X GET http://localhost:8080/api/jobs/1 \
  -b cookies.txt
```

**Response:**
```json
{
  "success": true,
  "job": {
    "id": 1,
    "user_id": 1,
    "status": "completed",
    "original_filename": "video.mp4",
    "file_path": "/data/files/uploads/1/1/video.mp4",
    "file_size": 5242880,
    "output_format": "srt",
    "transcript_path": "/data/files/results/1/1/video.srt",
    "created_at": "2026-01-18T10:00:00Z",
    "updated_at": "2026-01-18T10:05:00Z",
    "completed_at": "2026-01-18T10:05:00Z"
  }
}
```

Returns 403 Forbidden if the job doesn't belong to the authenticated user.

### GET /api/jobs/{id}/download
Download the transcript file for a completed job (protected endpoint).

**Example:**
```bash
curl -X GET http://localhost:8080/api/jobs/1/download \
  -b cookies.txt \
  -o transcript.srt
```

Returns the transcript file as an attachment. Only works for completed jobs.

### POST /api/analytics/events
Track an analytics event (public endpoint, no authentication required).

**Example:**
```bash
curl -X POST http://localhost:8080/api/analytics/events \
  -H "Content-Type: application/json" \
  -d '{
    "visitor_id": "550e8400-e29b-41d4-a716-446655440000",
    "event_name": "page_view",
    "properties": {"page": "/"},
    "utm_source": "reddit",
    "utm_medium": "organic"
  }'
```

**Response:**
```json
{
  "success": true
}
```

**Tracked events:**
- `page_view` - User visits a page
- `signup_completed` - User completes registration
- `login_completed` - User logs in
- `upload_completed` - User uploads a file
- `job_completed` - Background worker completes transcription
- `download_completed` - User downloads a transcript

### GET /api/analytics/funnel
Get conversion funnel data (protected endpoint, requires authentication).

**Example:**
```bash
curl -X GET "http://localhost:8080/api/analytics/funnel?start=2026-01-01&end=2026-01-31" \
  -H "Cookie: subtitler_token=YOUR_JWT_TOKEN"
```

**Response:**
```json
{
  "period": {
    "start": "2026-01-01T00:00:00Z",
    "end": "2026-01-31T23:59:59Z"
  },
  "funnel": [
    {"stage": "visitors", "count": 1000},
    {"stage": "signups", "count": 150},
    {"stage": "uploads", "count": 100},
    {"stage": "downloads", "count": 85}
  ],
  "conversion_rates": {
    "visitor_to_signup": 0.15,
    "signup_to_upload": 0.67,
    "upload_to_download": 0.85
  }
}
```

### GET /api/analytics/experiments/{id}
Get A/B test results for a specific experiment (public endpoint).

**Example:**
```bash
curl -X GET "http://localhost:8080/api/analytics/experiments/EXP001?start=2026-01-01&end=2026-01-31"
```

**Response:**
```json
{
  "experiment_id": "EXP001",
  "period": {
    "start": "2026-01-01T00:00:00Z",
    "end": "2026-01-31T23:59:59Z"
  },
  "variants": [
    {
      "variant": "A",
      "visitors": 300,
      "conversions": 45,
      "conversion_rate": 0.15
    },
    {
      "variant": "B",
      "visitors": 310,
      "conversions": 55,
      "conversion_rate": 0.177
    }
  ]
}
```

## User Interface

### Dashboard (/dashboard)
The dashboard page displays all transcription jobs for the authenticated user. Features:
- View list of all jobs with status (pending, processing, completed, failed)
- **Auto-refresh**: Automatically polls for job status updates every 5 seconds when there are pending/processing jobs
- Download completed transcripts
- See file information (name, size, format, date)
- View error messages for failed jobs
- Real-time status indicator showing when updates were last checked

Access the dashboard at `http://localhost:4321/dashboard` (requires authentication).

**Auto-refresh behavior:**
- Polling starts automatically when jobs with status `pending` or `processing` exist
- Polling stops when all jobs are `completed` or `failed`
- Refresh indicator shows "Checking for updates..." during polling
- "Last checked" timestamp displays after each successful update

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

# Test worker integration (upload → worker → completion, requires whisper.cpp)
cd backend && /home/trevor/go/bin/go test ./cmd/server -v -run TestWorkerIntegration -timeout 2m
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

# Run specific test file
cd frontend && npx playwright test tests/dashboard.spec.ts
```

## Deployment

### Cloudflare Tunnel Setup

The service can be exposed publicly using Cloudflare Tunnel. See [009_CLOUDFLARE_TUNNEL.md](009_CLOUDFLARE_TUNNEL.md) for detailed setup instructions.

**Quick overview:**
1. Install `cloudflared` on your server
2. Create a tunnel: `cloudflared tunnel create subtitler`
3. Configure DNS routes for frontend and API domains
4. Create tunnel config at `/etc/cloudflared/config.yml`
5. Install as systemd service: `sudo cloudflared service install`

**Verify tunnel setup:**
```bash
# Check service status
sudo systemctl status cloudflared

# Test health endpoint
curl https://api.subtitler.yourdomain.com/api/health
# Expected: {"success":true,"message":"Service is healthy","version":"1.0.0"}
```

### Production Configuration

For production deployments, set these environment variables:

```bash
# Backend
JWT_SECRET=<generate-secure-random-secret>
RESEND_API_KEY=<your-resend-api-key>
EMAIL_FROM=noreply@yourdomain.com
ENABLE_EMAIL=true
FRONTEND_URL=https://subtitler.yourdomain.com

# Frontend
PUBLIC_API_URL=https://api.subtitler.yourdomain.com
```

## Analytics

Subtitler includes a built-in analytics system for tracking user behavior and running A/B tests. The system is privacy-first and stores all data locally in SQLite.

**Key features:**
- Event tracking (page views, signups, uploads, downloads)
- Conversion funnel analysis
- A/B test variant assignment and results
- Anonymous visitor IDs (no third-party services)
- UTM parameter tracking for marketing campaigns

**Frontend integration:**
```typescript
import { trackEvent, trackPageView } from '../lib/analytics';

// Track page view
trackPageView();

// Track custom event
trackEvent('button_clicked', { button_id: 'signup' });
```

**Database tables:**
- `analytics_visitors` - Tracks anonymous visitors with UTM parameters
- `analytics_events` - Stores all tracked events with JSON properties
- `analytics_experiments` - A/B test variant assignments

See [010_ANALYTICS_INTEGRATION.md](010_ANALYTICS_INTEGRATION.md) and [EXPERIMENTATION_PLAN.md](EXPERIMENTATION_PLAN.md) for detailed documentation.

## Documentation

- [INSTALL.md](INSTALL.md) - Setup and installation guide
- [INTEGRATION_TESTS.md](INTEGRATION_TESTS.md) - Integration test documentation
- [001_RALPH_SUBTITLER.md](001_RALPH_SUBTITLER.md) - Architecture and planning
- [009_CLOUDFLARE_TUNNEL.md](009_CLOUDFLARE_TUNNEL.md) - Cloudflare Tunnel setup guide
- [010_ANALYTICS_INTEGRATION.md](010_ANALYTICS_INTEGRATION.md) - Analytics implementation
- [EXPERIMENTATION_PLAN.md](EXPERIMENTATION_PLAN.md) - A/B testing framework
- [MARKETING_PLAN.md](MARKETING_PLAN.md) - Marketing strategy
- [TASKS.jsonl](TASKS.jsonl) - Task tracking

## Development

This project is developed using the Ralph Wiggum autonomous agent loop. See PROMPT.md for more details.

## License

TBD
