# Peekaboo API Documentation

This document describes the HTTP API endpoints exposed by the Peekaboo backend.

## Base URL

The API is served on the port specified by the `PORT` environment variable (default: `8080`).

```
http://localhost:8080
```

## Endpoint Summary

| Method | Path | Auth | Rate Limit | Description |
|--------|------|------|------------|-------------|
| GET | `/health` | No | — | Basic health check (plain text `OK`) |
| GET | `/health/live` | No | — | Liveness probe |
| GET | `/health/ready` | No | — | Readiness probe (checks DB, whisper, LLM, Piper) |
| POST | `/api/transcribe` | No | 10/min | Audio-to-text via whisper-server |
| POST | `/api/intent` | No | 10/min | Extract subject from text via LLM |
| GET | `/api/media/{concept}` | No | 30/min | Get media URLs for a concept |
| GET | `/data/media/{concept}/{set}/{file}` | No | — | Serve media files (auto-decrypts if age key set) |
| POST | `/api/speak` | No | 10/min | Text-to-speech via Piper (optional) |
| POST | `/api/feedback` | No | 5/min | Submit user feedback |
| GET | `/api/admin/feedback` | Yes+TRUSTED | — | List feedback (admin) |
| GET | `/ws/audio` | No* | 10/min | WebSocket upgrade for audio streaming |
| POST | `/api/auth/register` | No | 5/min | Create account ([details](../specs/auth.md)) |
| POST | `/api/auth/login` | No | 10/min + Lockout 5/15min | Login ([details](../specs/auth.md)) |
| POST | `/api/auth/logout` | Yes | — | Invalidate session ([details](../specs/auth.md)) |
| GET | `/api/auth/me` | Yes | — | Get current user ([details](../specs/auth.md)) |
| GET | `/api/auth/verify` | No | — | Verify email token ([details](../specs/auth.md)) |
| POST | `/api/auth/resend-verification` | No | 3/15min | Resend verification email ([details](../specs/auth.md)) |
| POST | `/api/auth/magic-link` | No | 5/min | Request magic link ([details](../specs/auth.md)) |
| GET | `/api/auth/magic-link/verify` | No | — | Verify magic link token ([details](../specs/auth.md)) |
| GET | `/api/auth/csrf` | Yes | 30/min | Get CSRF token ([details](../specs/auth.md)) |
| POST | `/api/auth/totp/setup` | Yes+CSRF | 10/min | Generate TOTP secret ([details](../specs/auth.md)) |
| POST | `/api/auth/totp/enable` | Yes+CSRF | 10/min | Enable TOTP 2FA ([details](../specs/auth.md)) |
| POST | `/api/auth/totp/disable` | Yes+CSRF | 10/min | Disable TOTP 2FA ([details](../specs/auth.md)) |

\* WebSocket optionally uses session cookie for authenticated sessions.

## Core Endpoints

### Health Check

```
GET /health
```

Returns plain text `OK` with status 200.

### Readiness Probe

```
GET /health/ready
```

Returns JSON with status of each dependency:

```json
{
  "status": "ok",
  "details": { "database": "ok", "whisper": "ok", "llm": "ok" }
}
```

Returns 503 if any dependency is unavailable. Optional `piper` field appears when `PIPER_SERVER_URL` is set.

### Transcribe Audio

```
POST /api/transcribe
Content-Type: multipart/form-data
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `audio` | file | Yes | Audio file (webm, wav, mp3). Min 1KB, max 5MB |

```json
{"text": "show me a cat"}
```

Errors: 400 (missing/too small audio), 500 (transcription failed).

### Extract Intent

```
POST /api/intent
Content-Type: application/json
```

```json
{"text": "show me a cat"}
```

Response: `{"subject": "cat"}`

Errors: 400 (missing text), 500 (extraction failed).

### Get Media

```
GET /api/media/{concept}
```

Concept must match `^[a-z0-9_]+$`, max 50 characters.

```json
{
  "photo_url": "/data/media/cat/set1/photo.jpg",
  "audio_url": "/data/media/cat/set1/audio.mp3",
  "video_url": "/data/media/cat/set1/video.mp4"
}
```

`audio_url` and `video_url` are optional. Errors: 400 (invalid format), 404 (not found), 429 (rate limited).

### Synthesize Speech

```
POST /api/speak
Content-Type: application/json
```

```json
{"text": "Here is a cat!"}
```

Returns `audio/wav` (16-bit PCM). Max 256 characters. Only available when `PIPER_SERVER_URL` is set. See [specs/piper.md](../specs/piper.md).

### Submit Feedback

```
POST /api/feedback
Content-Type: application/json
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Yes | `general`, `bug`, or `feature` |
| `rating` | integer | No | 1-5 |
| `message` | string | Yes | Max 5000 characters |
| `context.session_id` | string | Yes | Anonymous session ID |
| `context.page_url` | string | Yes | Page URL |
| `context.concept_id` | string | No | Last displayed concept |
| `context.transcript` | string | No | Last voice command |
| `context.user_agent` | string | No | Browser user agent |

Response: `{"id": "feedback_abc123...", "status": "ok"}`

### List Feedback (Admin)

```
GET /api/admin/feedback
Authorization: Bearer <session_token>
```

Requires authenticated user whose ID is in `TRUSTED_USERS` env var. Returns 403 if unauthorized or `TRUSTED_USERS` is not configured.

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `status` | string | `new` | `new`, `reviewed`, or `all` |
| `limit` | integer | `50` | Max items (1-100) |
| `after` | string | — | RFC3339 cursor for pagination |

```json
{
  "feedback": [
    {"id": "feedback_abc...", "type": "bug", "rating": 4, "message": "...", "page_url": "/", "created_at": "2026-02-08T...", "status": "new"}
  ],
  "total": 42,
  "limit": 50
}
```

## WebSocket Audio Streaming

Real-time audio streaming for continuous voice interaction.

```
GET /ws/audio → WebSocket upgrade
```

- **Rate Limit**: 10 connections/min per IP
- **Connection Limit**: 100 concurrent (configurable via `WEBSOCKET_MAX_CONNECTIONS`)
- **Idle Timeout**: 5 min (configurable via `WEBSOCKET_IDLE_TIMEOUT_SECS`)
- **Max Message Size**: 5MB

### Client → Server

**Audio frames (binary)** — 12-byte header + audio data:

```
Byte 0-1:   Magic 0xAB01 (big-endian uint16)
Byte 2-3:   Sequence number (big-endian uint16)
Byte 4-11:  Client timestamp (big-endian float64, ms since epoch)
Byte 12+:   Audio data (WebM/Opus from MediaRecorder)
```

**Control messages (JSON)**:
- `{"type": "start_recording", "client_time": 1707234567890}` — Start buffering audio
- `{"type": "stop_recording"}` — Process buffered audio
- `{"type": "ping"}` — Keep-alive

### Server → Client

| Type | Description |
|------|-------------|
| `transcript` | Transcription with optional `segments` (per-word timing) and `audio_start_time` |
| `tts_audio` | Base64-encoded WAV audio with `audio_data` and `text` fields |
| `media` | Media result with `subject`, `photo_url`, optional `audio_url`/`video_url` |
| `error` | Error with `message` field |
| `pong` | Keep-alive response |

### Processing Flow

1. Client sends `start_recording` with `client_time`
2. Client streams framed audio chunks while user speaks
3. Server accumulates audio, parses WebM container incrementally
4. Server transcribes via whisper-server with VAD, sends `transcript`
5. LLM processes transcript, returns tool actions:
   - **show_media**: Look up media, send `media` message, trim buffer
   - **text_to_speech**: Synthesize via Piper, send `tts_audio`
   - **wait_for_more**: Transcript incomplete, wait for more audio
6. Client can continue recording for the next command

For detailed architecture, see [specs/audio-timing.md](../specs/audio-timing.md) and [specs/websocket-audio.md](../specs/websocket-audio.md).

## Authentication

Session-based authentication with HttpOnly cookies. Full details including request/response formats, error codes, database schema, and CSRF protection are in [specs/auth.md](../specs/auth.md).

Key points:
- Sessions last 30 days, set via `session` cookie (HttpOnly, SameSite=Lax)
- CSRF token required for authenticated POST/PUT/DELETE (via `X-CSRF-Token` header)
- TOTP 2FA optional per-user (SHA1, 6-digit, 30-second period)
- Login lockout after 5 failed attempts in 15 minutes
- Email verification required before login
- Magic links valid for 15 minutes, verification tokens for 24 hours

## CORS

- **Allowed Origin**: `ALLOWED_ORIGIN` env var (default: `*` for development)
- **Allowed Methods**: `GET`, `POST`, `OPTIONS`
- **Allowed Headers**: `Content-Type`, `X-CSRF-Token`

## Rate Limiting

| Limit | Endpoints |
|-------|-----------|
| 10/min per IP | `POST /api/transcribe`, `POST /api/intent`, `POST /api/speak`, `GET /ws/audio`, `POST /api/auth/login`, `POST /api/auth/totp/*` |
| 30/min per IP | `GET /api/media/{concept}`, `GET /api/auth/csrf` |
| 5/min per IP | `POST /api/feedback`, `POST /api/auth/magic-link`, `POST /api/auth/register` |
| 3/15min per IP | `POST /api/auth/resend-verification` |
| 5 failures/15min | `POST /api/auth/login` (account lockout, in addition to rate limit) |

Rate-limited responses return HTTP 429 with `Retry-After: 60` header.

## Error Handling

All errors return JSON with an `error` field. Internal details are logged server-side only.

| Code | Description |
|------|-------------|
| 200 | Success |
| 400 | Bad Request (invalid input) |
| 404 | Not Found |
| 405 | Method Not Allowed |
| 429 | Too Many Requests |
| 500 | Internal Server Error |
| 503 | Service Unavailable (capacity or dependency) |

## Full Flow Example

```bash
# 1. Transcribe audio
curl -X POST http://localhost:8080/api/transcribe -F "audio=@recording.webm"
# {"text": "show me a cat"}

# 2. Extract intent
curl -X POST http://localhost:8080/api/intent \
  -H "Content-Type: application/json" -d '{"text": "show me a cat"}'
# {"subject": "cat"}

# 3. Get media
curl http://localhost:8080/api/media/cat
# {"photo_url": "/data/media/cat/set1/photo.jpg", "audio_url": "..."}

# 4. Fetch media files
curl http://localhost:8080/data/media/cat/set1/photo.jpg -o cat.jpg
```
