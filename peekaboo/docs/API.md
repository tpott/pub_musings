# Peekaboo API Documentation

This document describes the HTTP API endpoints exposed by the Peekaboo backend.

## Base URL

The API is served on the port specified by the `PORT` environment variable (default: `8080`).

```
http://localhost:8080
```

## Endpoints

### Health Check (Basic)

Check if the server is running. Simple plain-text response.

```
GET /health
```

#### Response

- **200 OK**: Server is healthy

```
OK
```

#### Example

```bash
curl http://localhost:8080/health
```

---

### Liveness Probe

Kubernetes-style liveness probe. Returns 200 if the server process is running.

```
GET /health/live
```

#### Response

**Success (200 OK)**:
```json
{
  "status": "ok"
}
```

#### Example

```bash
curl http://localhost:8080/health/live
```

---

### Readiness Probe

Kubernetes-style readiness probe. Returns 200 if the server and all dependencies are ready to accept traffic.

Checks:
- **Database**: SQLite connectivity
- **Whisper-server**: Speech-to-text service availability
- **Piper TTS** (optional): Only checked if `PIPER_SERVER_URL` is set
- **LLM Provider**: API key validity and provider availability (Anthropic or OpenAI)

```
GET /health/ready
```

#### Response

**Success (200 OK)**:
```json
{
  "status": "ok",
  "details": {
    "database": "ok",
    "whisper": "ok",
    "llm": "ok"
  }
}
```

**Error (503 Service Unavailable)**:
```json
{
  "status": "unavailable",
  "error": "llm provider unavailable",
  "details": {
    "database": "ok",
    "whisper": "ok",
    "llm": "unavailable"
  }
}
```

Possible error values:
- `"database unavailable"` - SQLite connection failed
- `"whisper-server unavailable"` - Whisper service unreachable
- `"piper-server unavailable"` - Piper TTS unreachable (if configured)
- `"llm provider unavailable"` - LLM API key invalid or provider unreachable

#### Example

```bash
curl http://localhost:8080/health/ready
```

---

### Transcribe Audio

Convert audio to text using the whisper-server.

```
POST /api/transcribe
```

#### Request

- **Content-Type**: `multipart/form-data`
- **Min Size**: 1KB
- **Max Size**: 5MB

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `audio` | file | Yes | Audio file (webm, wav, mp3, etc.) |

#### Response

**Success (200 OK)**:
```json
{
  "text": "show me a cat"
}
```

**Error (400 Bad Request)**:
```json
{
  "error": "missing audio file"
}
```

```json
{
  "error": "audio file too small (minimum 1KB)"
}
```

```json
{
  "error": "invalid multipart form"
}
```

**Error (500 Internal Server Error)**:
```json
{
  "error": "transcription failed"
}
```

#### Example

```bash
curl -X POST http://localhost:8080/api/transcribe \
  -F "audio=@recording.webm"
```

---

### Extract Intent

Extract the subject/intent from natural language text using an LLM.

```
POST /api/intent
```

#### Request

- **Content-Type**: `application/json`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `text` | string | Yes | The transcribed text to extract intent from |

#### Response

**Success (200 OK)**:
```json
{
  "subject": "cat"
}
```

**Error (400 Bad Request)**:
```json
{
  "error": "invalid JSON body"
}
```

```json
{
  "error": "missing text field"
}
```

**Error (500 Internal Server Error)**:
```json
{
  "error": "intent extraction failed"
}
```

#### Example

```bash
curl -X POST http://localhost:8080/api/intent \
  -H "Content-Type: application/json" \
  -d '{"text": "show me a cat"}'
```

---

### Get Media

Retrieve media URLs for a concept (animal).

```
GET /api/media/{concept}
```

#### Path Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `concept` | string | The concept ID (e.g., `cat`, `dog`, `duck`, `pig`, `chicken`, `cow`) |

**Concept ID constraints:**
- Maximum length: 50 characters
- Allowed characters: lowercase letters (`a-z`), numbers (`0-9`), and underscores (`_`)
- Must match the pattern: `^[a-z0-9_]+$`

#### Response

**Success (200 OK)**:
```json
{
  "photo_url": "/data/media/cat/set1/photo.jpg",
  "audio_url": "/data/media/cat/set1/audio.mp3"
}
```

The `video_url` field is also included when video content is available:
```json
{
  "photo_url": "/data/media/cat/set1/photo.jpg",
  "audio_url": "/data/media/cat/set1/audio.mp3",
  "video_url": "/data/media/cat/set1/video.mp4"
}
```

**Error (400 Bad Request)**:
```json
{
  "error": "missing concept parameter"
}
```

```json
{
  "error": "invalid concept format"
}
```

```json
{
  "error": "concept ID too long (max 50 characters)"
}
```

**Error (404 Not Found)**:
```json
{
  "error": "concept not found"
}
```

```json
{
  "error": "no media found for concept"
}
```

**Error (429 Too Many Requests)**:
```json
{
  "error": "rate limit exceeded, try again later"
}
```

Rate limit: 30 requests per minute per IP address.

**Error (500 Internal Server Error)**:
```json
{
  "error": "media lookup failed"
}
```

#### Example

```bash
curl http://localhost:8080/api/media/cat
```

---

### Media Files

Static media files are served from the `/data/media/` path.

```
GET /data/media/{concept}/{set}/{filename}
```

If an age encryption key is configured, media files are decrypted on-the-fly.

#### Example

```bash
# Fetch a photo
curl http://localhost:8080/data/media/cat/set1/photo.jpg -o cat.jpg

# Fetch audio
curl http://localhost:8080/data/media/cat/set1/audio.mp3 -o cat.mp3
```

---

### Synthesize Speech (TTS)

Convert text to speech using the Piper TTS server. This endpoint is optional and only available when `PIPER_SERVER_URL` is configured.

```
POST /api/speak
```

#### Request

- **Content-Type**: `application/json`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `text` | string | Yes | The text to synthesize (max 256 characters) |

#### Response

**Success (200 OK)**:
- **Content-Type**: `audio/wav`
- Returns raw WAV audio data (16-bit PCM)

**Error (400 Bad Request)**:
```json
{
  "error": "invalid JSON body"
}
```

```json
{
  "error": "missing text field"
}
```

```json
{
  "error": "text too long (max 256 characters)"
}
```

**Error (500 Internal Server Error)**:
```json
{
  "error": "speech synthesis failed"
}
```

#### Example

```bash
# Synthesize speech and save to file
curl -X POST http://localhost:8080/api/speak \
  -H "Content-Type: application/json" \
  -d '{"text": "Here is a cat!"}' \
  -o speech.wav

# Play the audio (requires aplay on Linux)
aplay speech.wav
```

#### Configuration

TTS is disabled by default. To enable it, set the `PIPER_SERVER_URL` environment variable:

```bash
export PIPER_SERVER_URL=http://localhost:5000
```

See [specs/piper.md](../specs/piper.md) for Piper server setup instructions.

---

### Submit Feedback

Submit user feedback, bug reports, or feature requests.

```
POST /api/feedback
```

#### Request

- **Content-Type**: `application/json`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Yes | Feedback type: `general`, `bug`, or `feature` |
| `rating` | integer | No | Rating from 1 to 5 |
| `message` | string | Yes | Feedback message (max 5000 characters) |
| `context.session_id` | string | Yes | Anonymous session identifier |
| `context.page_url` | string | Yes | URL where feedback was submitted |
| `context.concept_id` | string | No | Last displayed animal concept |
| `context.transcript` | string | No | Last recognized voice command |
| `context.user_agent` | string | No | Browser user agent string |

#### Response

**Success (200 OK)**:
```json
{
  "id": "feedback_abc123def456789012345678",
  "status": "ok"
}
```

**Error (400 Bad Request)**:
```json
{
  "error": "message is required"
}
```

```json
{
  "error": "invalid type: must be general, bug, or feature"
}
```

```json
{
  "error": "rating must be between 1 and 5"
}
```

**Error (413 Payload Too Large)**:
```json
{
  "error": "message too long (max 5000 characters)"
}
```

**Error (429 Too Many Requests)**:
```json
{
  "error": "rate limit exceeded, try again later"
}
```

#### Example

```bash
curl -X POST http://localhost:8080/api/feedback \
  -H "Content-Type: application/json" \
  -d '{
    "type": "feature",
    "rating": 5,
    "message": "Would love to see elephants!",
    "context": {
      "session_id": "abc123",
      "page_url": "/",
      "concept_id": "cat",
      "transcript": "show me a cat"
    }
  }'
```

#### Rate Limiting

Feedback submissions are limited to **5 requests per minute per IP address**.

---

## WebSocket Audio Streaming

The WebSocket endpoint provides real-time audio streaming for continuous voice interaction. Audio is sent using a framed binary protocol with timestamps, transcribed via whisper-server, and processed by an LLM that decides what action to take (show media, speak, or wait for more input).

```
GET /ws/audio → WebSocket upgrade
```

### Connection

1. Client sends HTTP upgrade request to `/ws/audio`
2. Server validates origin (if `ALLOWED_ORIGIN` is set) and rate limit
3. On success, connection upgrades to WebSocket protocol

**Rate Limiting**: 10 WebSocket connections per minute per IP address. Rate-limited requests receive HTTP 429 before upgrade.

**Connection Limit**: Maximum 100 concurrent WebSocket connections (configurable via `WEBSOCKET_MAX_CONNECTIONS`). Excess connections receive HTTP 503 with `"server at capacity, try again later"`.

**Idle Timeout**: Connections are closed after 5 minutes of inactivity (configurable via `WEBSOCKET_IDLE_TIMEOUT_SECS`).

**Max Message Size**: 5MB per binary message.

### Message Protocol

Messages are JSON for control/text and framed binary for audio data.

#### Client → Server

**Audio frames (binary)**

Each audio chunk is sent as a binary WebSocket message with a 12-byte header:

```
Byte 0-1:   Magic number 0xAB01 (big-endian uint16)
Byte 2-3:   Sequence number (big-endian uint16, wraps at 65535)
Byte 4-11:  Client timestamp (big-endian float64, milliseconds since epoch)
Byte 12+:   Audio data (WebM/Opus bytes from MediaRecorder)
```

Sent every ~500ms while recording. The header enables the server to correlate whisper's audio-relative timestamps with client wall-clock time.

**Control messages (JSON)**

```json
{"type": "start_recording", "client_time": 1707234567890}
```
Tells server to start buffering incoming audio. `client_time` is `Date.now()` at recording start, used for clock offset calculation.

```json
{"type": "stop_recording"}
```
Tells server to process buffered audio and return results.

```json
{"type": "ping"}
```
Keep-alive message to prevent idle timeout.

#### Server → Client

**Transcript**

Sent when audio transcription completes. Includes per-word timing and probability data from whisper:

```json
{
  "type": "transcript",
  "text": "show me a cat",
  "segments": [
    {
      "id": 0,
      "start": 0.0,
      "end": 1.5,
      "text": "show me a cat",
      "words": [
        {"word": "show", "start": 0.0, "end": 0.3, "probability": 0.98},
        {"word": "me", "start": 0.4, "end": 0.6, "probability": 0.99},
        {"word": "a", "start": 0.7, "end": 0.8, "probability": 0.97},
        {"word": "cat", "start": 0.9, "end": 1.3, "probability": 0.99}
      ]
    }
  ],
  "audio_start_time": 1707234567890.0
}
```

Fields: `segments` and `audio_start_time` are optional (present when word-level data is available).

**TTS audio**

Sent when the LLM requests text-to-speech (requires Piper TTS). Contains base64-encoded WAV audio:

```json
{
  "type": "tts_audio",
  "audio_data": "UklGRiQAAABXQVZFZm10IBAAAA...",
  "text": "Here is a cat!"
}
```

The client should play TTS audio before displaying any subsequent media result.

**Media result**

Sent when the LLM identifies a subject and media is found:

```json
{
  "type": "media",
  "subject": "cat",
  "photo_url": "/data/media/cat/set1/photo.jpg",
  "audio_url": "/data/media/cat/set1/audio.mp3",
  "video_url": "/data/media/cat/set1/video.mp4"
}
```

Note: `audio_url` and `video_url` are optional fields, only present when media is available. At most one `media` message is sent per LLM processing cycle.

**Error**

Sent on processing errors:

```json
{"type": "error", "message": "transcription failed"}
```

Possible error messages:
- `"invalid message format"` - Client sent malformed JSON
- `"audio too short"` - Recorded audio was too small (< 1KB)
- `"No audio recorded"` - Recording stopped with empty buffer
- `"No speech detected. Please try again."` - Whisper returned empty transcript
- `"transcription failed"` - Whisper server error
- `"intent extraction failed"` - LLM provider error
- `"no media found for {subject}"` - No media in database for extracted subject
- `"media lookup unavailable"` - Database not configured

**Pong**

Sent in response to ping:

```json
{"type": "pong"}
```

### Processing Flow

1. Client sends `start_recording` with `client_time`
2. Client streams framed audio chunks (binary with 12-byte headers) while user speaks
3. Server accumulates audio and parses WebM container incrementally (EBML init segment + Clusters)
4. Server triggers transcription when buffer threshold is reached (time-based) or on `stop_recording`
5. Server transcribes audio via whisper-server with VAD enabled, getting per-word timing and probabilities
6. Server sends `transcript` message to client with word-level data
7. Server sends accumulated transcript + word data to LLM with `tool_choice: any`
8. LLM returns one of three tool actions:
   - **show_media**: Server looks up media in database, sends `media` message. Trims audio buffer at the instruction boundary (Cluster-aligned) for the next cycle.
   - **text_to_speech**: Server synthesizes speech via Piper, sends `tts_audio` message. Often followed by `show_media`.
   - **wait_for_more**: Transcript appears incomplete. Server waits for more audio without acting.
9. If multiple tools are returned (e.g., `text_to_speech` + `show_media`), they execute in order
10. Client can continue recording for the next command (mic stays active)

### Example (JavaScript)

```javascript
const ws = new WebSocket('ws://localhost:8080/ws/audio');

ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  switch (msg.type) {
    case 'transcript':
      console.log('Heard:', msg.text);
      break;
    case 'tts_audio':
      const audio = new Audio('data:audio/wav;base64,' + msg.audio_data);
      await audio.play();
      break;
    case 'media':
      displayMedia(msg.photo_url, msg.audio_url);
      break;
    case 'error':
      showError(msg.message);
      break;
  }
};

// Start recording
ws.send(JSON.stringify({ type: 'start_recording', client_time: Date.now() }));

// Send framed audio chunks from MediaRecorder
mediaRecorder.ondataavailable = (e) => {
  if (e.data.size > 0) {
    // In practice, use AudioWebSocket which adds the 12-byte header
    ws.send(e.data);
  }
};

// Stop recording
ws.send(JSON.stringify({ type: 'stop_recording' }));
```

For detailed architecture, see [specs/audio-timing.md](../specs/audio-timing.md) and [specs/websocket-audio.md](../specs/websocket-audio.md).

---

## Authentication

All auth endpoints use JSON request/response bodies. Session tokens are set as HttpOnly cookies.

### Register

Create a new user account. Sends a verification email (logged in dev mode).

```
POST /api/auth/register
```

**CSRF-exempt.** No rate limit on this endpoint.

#### Request

- **Content-Type**: `application/json`
- **Max Body Size**: 4KB

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `email` | string | Yes | User's email address |
| `password` | string | Yes | Password (minimum 8 characters) |

#### Response

**Success (201 Created)**:
```json
{
  "message": "Account created. Please check your email to verify your account.",
  "email_verification": true,
  "user": {
    "id": "abc123...",
    "email": "user@example.com",
    "created_at": "2026-02-08T00:00:00Z",
    "email_verified": false
  }
}
```

**Error (400 Bad Request)**:
```json
{"error": "invalid email address"}
```
```json
{"error": "password must be at least 8 characters"}
```

**Error (409 Conflict)**:
```json
{"error": "email already registered"}
```

---

### Login

Authenticate with email and password. Sets a session cookie on success.

```
POST /api/auth/login
```

**CSRF-exempt.**

#### Request

- **Content-Type**: `application/json`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `email` | string | Yes | User's email address |
| `password` | string | Yes | User's password |
| `totp_code` | string | No | TOTP 2FA code (required if user has 2FA enabled) |

#### Response

**Success (200 OK)**:
```json
{
  "user": {
    "id": "abc123...",
    "email": "user@example.com",
    "totp_enabled": false
  },
  "token": "session-token-value"
}
```

Sets `session` cookie (HttpOnly, SameSite=Lax, 30-day expiry).

**Error (401 Unauthorized)**:
```json
{"error": "invalid email or password"}
```
```json
{"totp_required": true}
```

**Error (403 Forbidden)** (email not verified):
```json
{
  "error": "Please verify your email address before logging in",
  "email_not_verified": true,
  "can_resend_verification": true
}
```

**Error (429 Too Many Requests)** (account lockout: 5 failures in 15 min):
```json
{
  "error": "too many failed login attempts, please try again later",
  "retry_after_min": 15
}
```

---

### Logout

Invalidate the current session. Requires authentication.

```
POST /api/auth/logout
```

#### Response

**Success (200 OK)**:
```json
{"message": "Logged out"}
```

Clears the `session` cookie.

**Error (401 Unauthorized)**:
```json
{"error": "not authenticated"}
```

---

### Get Current User

Return the authenticated user's profile. Requires authentication.

```
GET /api/auth/me
```

#### Response

**Success (200 OK)**:
```json
{
  "user": {
    "id": "abc123...",
    "email": "user@example.com",
    "totp_enabled": false,
    "email_verified": true,
    "created_at": "2026-02-08T00:00:00Z"
  }
}
```

**Error (401 Unauthorized)**:
```json
{"error": "not authenticated"}
```

---

### Verify Email

Verify a user's email address using the token from the verification email.

```
GET /api/auth/verify?token={token}
```

#### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `token` | string | Yes | Verification token from email (valid 24 hours) |

#### Response

**Success (200 OK)**:
```json
{"message": "Email verified successfully. You can now log in."}
```

**Error (400 Bad Request)**:
```json
{"error": "missing token parameter"}
```
```json
{"error": "invalid or expired token"}
```
```json
{"error": "token already used"}
```
```json
{"error": "token expired"}
```

---

### Resend Verification Email

Resend the email verification link. Rate limited.

```
POST /api/auth/resend-verification
```

**CSRF-exempt.** Rate limit: 3 requests per 15 minutes per IP.

#### Request

- **Content-Type**: `application/json`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `email` | string | Yes | Email address to resend verification to |

#### Response

**Success (200 OK)** (always returns 200 to prevent email enumeration):
```json
{"message": "If an account exists with that email, a verification link has been sent."}
```

---

### Request Magic Link

Request a passwordless login link sent via email. Rate limited.

```
POST /api/auth/magic-link
```

**CSRF-exempt.** Rate limit: 5 requests per minute per IP.

#### Request

- **Content-Type**: `application/json`

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `email` | string | Yes | Email address to send magic link to |

#### Response

**Success (200 OK)** (always returns 200 to prevent email enumeration):
```json
{"message": "If an account exists with that email, a login link has been sent."}
```

---

### Verify Magic Link

Authenticate using a magic link token. Sets a session cookie on success.

```
GET /api/auth/magic-link/verify?token={token}
```

#### Query Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `token` | string | Yes | Magic link token from email (valid 15 minutes) |

#### Response

**Success (200 OK)**:
```json
{
  "message": "Login successful",
  "user": {
    "id": "abc123...",
    "email": "user@example.com",
    "totp_enabled": false
  }
}
```

Sets `session` cookie (HttpOnly, SameSite=Lax, 30-day expiry).

**Error (400 Bad Request)**:
```json
{"error": "missing token parameter"}
```
```json
{"error": "invalid or expired token"}
```
```json
{"error": "token already used"}
```
```json
{"error": "token expired"}
```

---

### Get CSRF Token

Get a CSRF token for state-changing requests. Requires authentication.

```
GET /api/auth/csrf
```

#### Response

**Success (200 OK)**:
```json
{"token": "csrf-token-value"}
```

Include this token in the `X-CSRF-Token` header for POST/PUT/DELETE requests (except CSRF-exempt paths).

**Error (401 Unauthorized)**:
```json
{"error": "not authenticated"}
```

### TOTP Setup

`POST /api/auth/totp/setup`

Generates a new TOTP secret for the authenticated user. The secret is stored but TOTP is not yet enabled. Call `/api/auth/totp/enable` with a valid code to activate.

**Requires**: Authentication, CSRF token

**Response (200 OK)**:
```json
{
  "secret": "JBSWY3DPEHPK3PXP...",
  "uri": "otpauth://totp/Peekaboo:user@example.com?secret=...&issuer=Peekaboo&algorithm=SHA1&digits=6&period=30"
}
```

The `uri` can be rendered as a QR code for scanning with an authenticator app (Google Authenticator, Authy, etc.).

**Errors**:
- 401: Not authenticated
- 409: TOTP is already enabled

### TOTP Enable

`POST /api/auth/totp/enable`

Enables TOTP 2FA after verifying a valid code from the authenticator app and the user's password.

**Requires**: Authentication, CSRF token

**Request**:
```json
{
  "code": "123456",
  "password": "userpassword"
}
```

**Response (200 OK)**:
```json
{"message": "TOTP enabled successfully"}
```

**Errors**:
- 400: Missing code or password, or TOTP not set up yet
- 401: Invalid password or invalid TOTP code
- 409: TOTP is already enabled

### TOTP Disable

`POST /api/auth/totp/disable`

Disables TOTP 2FA and clears the stored secret. Requires password confirmation.

**Requires**: Authentication, CSRF token

**Request**:
```json
{
  "password": "userpassword"
}
```

**Response (200 OK)**:
```json
{"message": "TOTP disabled successfully"}
```

**Errors**:
- 400: Missing password
- 401: Invalid password
- 409: TOTP is not enabled

---

## CORS

The API supports CORS with the following configuration:

- **Allowed Origin**: Set via `ALLOWED_ORIGIN` environment variable (default: `*` for development)
- **Allowed Methods**: `GET`, `POST`, `OPTIONS`
- **Allowed Headers**: `Content-Type`

---

## Rate Limiting

Expensive endpoints are rate limited to prevent abuse.

- **Limit**: 10 requests per minute per IP address
- **Applies to**: `POST /api/transcribe`, `POST /api/intent`, `POST /api/speak`, `GET /ws/audio` (connection upgrade)

- **Limit**: 30 requests per minute per IP address
- **Applies to**: `GET /api/media/{concept}`

- **Limit**: 5 requests per minute per IP address
- **Applies to**: `POST /api/feedback`, `POST /api/auth/magic-link`

- **Limit**: 3 requests per 15 minutes per IP address
- **Applies to**: `POST /api/auth/resend-verification`

- **Response when limited**: HTTP 429 Too Many Requests

```json
{
  "error": "rate limit exceeded, try again later"
}
```

The `Retry-After: 60` header is included in rate-limited responses.

---

## Error Handling

All API errors return JSON responses with an `error` field containing a sanitized error message. Internal details are logged server-side but not exposed to clients.

Common HTTP status codes:

| Code | Description |
|------|-------------|
| 200 | Success |
| 400 | Bad Request (invalid input) |
| 404 | Not Found (concept or media not found) |
| 405 | Method Not Allowed |
| 429 | Too Many Requests (rate limited) |
| 500 | Internal Server Error |
| 503 | Service Unavailable (WebSocket connection limit) |
| 503 | Service Unavailable (dependency unavailable) |

---

## Full Flow Example

Here's how a typical voice-to-media flow works:

```bash
# 1. Record audio and transcribe it
curl -X POST http://localhost:8080/api/transcribe \
  -F "audio=@recording.webm"
# Response: {"text": "show me a cat"}

# 2. Extract intent from the transcription
curl -X POST http://localhost:8080/api/intent \
  -H "Content-Type: application/json" \
  -d '{"text": "show me a cat"}'
# Response: {"subject": "cat"}

# 3. Get media for the extracted subject
curl http://localhost:8080/api/media/cat
# Response: {"photo_url": "/data/media/cat/set1/photo.jpg", "audio_url": "/data/media/cat/set1/audio.mp3"}

# 4. Fetch the actual media files
curl http://localhost:8080/data/media/cat/set1/photo.jpg -o cat.jpg
curl http://localhost:8080/data/media/cat/set1/audio.mp3 -o cat.mp3
```
