# Subtitler API Documentation

This document describes all available API endpoints for the Subtitler backend.

**Base URL**: `http://localhost:8080/api` (development)

## Authentication

Most endpoints support optional authentication. Some endpoints require authentication.

### Methods

1. **Session Cookie**: Automatically set on login/register
2. **Authorization Header**: `Authorization: Bearer <token>`

### Rate Limiting

Authentication endpoints are rate limited to 5 requests per minute per IP address.
When exceeded, returns `429 Too Many Requests` with `Retry-After` header.

Rate-limited endpoints:
- `POST /api/auth/register` - 5/min per IP
- `POST /api/auth/login` - 5/min per IP
- `POST /api/auth/totp/setup` - 5/min per IP
- `POST /api/auth/totp/verify` - 5/min per IP
- `POST /api/auth/totp/disable` - 5/min per IP
- `POST /api/auth/totp/recover` - 5/min per IP
- `POST /api/auth/totp/codes` - 5/min per IP
- `POST /api/auth/forgot-password` - 3/15min per IP (stricter)
- `POST /api/auth/reset-password` - 5/min per IP

---

## Table of Contents

- [Health](#health)
- [Debug](#debug)
- [Authentication](#authentication-endpoints)
- [Two-Factor Authentication (2FA)](#two-factor-authentication-2fa)
- [Password Reset](#password-reset)
- [Session Management](#session-management)
- [Videos](#videos)
- [Transcription](#transcription)
- [Subtitles](#subtitles)
- [Script Conversion](#script-conversion)

---

## Health

### GET /api/health

Health check endpoint.

**Authentication**: Not required

**Response** `200 OK`:
```json
{
  "status": "ok"
}
```

**Example**:
```bash
curl http://localhost:8080/api/health
```

---

## Debug

### POST /api/log

Frontend console log forwarding (development mode only).

**Authentication**: Not required

**Request Body**:
```json
{
  "level": "log",
  "message": "Debug message",
  "url": "http://localhost:4321/upload",
  "line": 42,
  "column": 10
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `level` | string | No | Log level: `log`, `warn`, `error`, `info`, `debug` |
| `message` | string | Yes | Log message |
| `url` | string | No | Page URL where log originated |
| `line` | number | No | Source line number |
| `column` | number | No | Source column number |

**Response** `200 OK`:
```json
{
  "status": "ok"
}
```

---

## Authentication Endpoints

### POST /api/auth/register

Create a new user account.

**Authentication**: Not required
**Rate Limited**: Yes (5/min per IP)

**Request Body**:
```json
{
  "email": "user@example.com",
  "password": "securepassword123"
}
```

| Field | Type | Required | Validation |
|-------|------|----------|------------|
| `email` | string | Yes | Valid email format |
| `password` | string | Yes | 8-72 characters |

**Response** `200 OK`:
```json
{
  "user": {
    "id": "abc123...",
    "email": "user@example.com",
    "created_at": "2026-01-22T10:00:00Z",
    "totp_enabled": false
  },
  "token": "session_token_here"
}
```

**Errors**:
- `400 Bad Request`: Invalid email or password format
- `409 Conflict`: Email already registered
- `429 Too Many Requests`: Rate limit exceeded

**Example**:
```bash
curl -X POST http://localhost:8080/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"securepassword123"}'
```

---

### POST /api/auth/login

Authenticate and get session token.

**Authentication**: Not required
**Rate Limited**: Yes (5/min per IP)

**Request Body**:
```json
{
  "email": "user@example.com",
  "password": "securepassword123",
  "totp_code": "123456"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `email` | string | Yes | User email |
| `password` | string | Yes | User password |
| `totp_code` | string | Conditional | Required if 2FA is enabled |

**Response** `200 OK`:
```json
{
  "user": {
    "id": "abc123...",
    "email": "user@example.com",
    "created_at": "2026-01-22T10:00:00Z",
    "totp_enabled": false
  },
  "token": "session_token_here"
}
```

**Response** `401 Unauthorized` (2FA required):
```json
{
  "error": "2FA code required",
  "totp_required": true
}
```

When `totp_required: true` is returned, resubmit with the `totp_code` field.

**Errors**:
- `400 Bad Request`: Invalid request body
- `401 Unauthorized`: Invalid credentials or 2FA code
- `429 Too Many Requests`: Rate limit exceeded

**Example**:
```bash
# Without 2FA
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"securepassword123"}'

# With 2FA
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"securepassword123","totp_code":"123456"}'
```

---

### POST /api/auth/logout

Invalidate current session.

**Authentication**: Optional (logs out current session if authenticated)

**Response** `200 OK`:
```json
{
  "message": "Logged out successfully"
}
```

**Example**:
```bash
curl -X POST http://localhost:8080/api/auth/logout \
  -H "Authorization: Bearer your_token_here"
```

---

### GET /api/auth/me

Get current authenticated user.

**Authentication**: Required

**Response** `200 OK`:
```json
{
  "user": {
    "id": "abc123...",
    "email": "user@example.com",
    "created_at": "2026-01-22T10:00:00Z",
    "totp_enabled": false
  }
}
```

**Errors**:
- `401 Unauthorized`: Not authenticated

**Example**:
```bash
curl http://localhost:8080/api/auth/me \
  -H "Authorization: Bearer your_token_here"
```

---

## Two-Factor Authentication (2FA)

### POST /api/auth/totp/setup

Start 2FA setup - generates a new TOTP secret.

**Authentication**: Required
**Rate Limited**: Yes (5/min per IP)

**Response** `200 OK`:
```json
{
  "secret": "JBSWY3DPEHPK3PXP",
  "secret_display": "JBSW Y3DP EHPK 3PXP",
  "uri": "otpauth://totp/Subtitler:user@example.com?secret=JBSWY3DPEHPK3PXP&issuer=Subtitler",
  "issuer": "Subtitler",
  "qr_code": "data:image/png;base64,iVBOR..."
}
```

| Field | Description |
|-------|-------------|
| `secret` | Raw TOTP secret for manual entry |
| `secret_display` | Secret formatted with spaces for readability |
| `uri` | Provisioning URI for QR codes |
| `qr_code` | Base64-encoded PNG QR code (data URL) |

**Errors**:
- `400 Bad Request`: 2FA already enabled
- `401 Unauthorized`: Not authenticated

**Example**:
```bash
curl -X POST http://localhost:8080/api/auth/totp/setup \
  -H "Authorization: Bearer your_token_here"
```

---

### POST /api/auth/totp/verify

Verify TOTP code and enable 2FA. Returns recovery codes.

**Authentication**: Required
**Rate Limited**: Yes (5/min per IP)

**Request Body**:
```json
{
  "code": "123456"
}
```

**Response** `200 OK`:
```json
{
  "message": "2FA has been enabled successfully",
  "totp_enabled": true,
  "recovery_codes": [
    "ABCD-1234",
    "EFGH-5678",
    "IJKL-9012",
    "MNOP-3456",
    "QRST-7890",
    "UVWX-1234",
    "YZAB-5678",
    "CDEF-9012",
    "GHIJ-3456",
    "KLMN-7890"
  ]
}
```

**Important**: Store the recovery codes securely. They are only shown once and can be used to disable 2FA if you lose access to your authenticator.

**Errors**:
- `400 Bad Request`: Invalid code, 2FA already enabled, or no secret set up
- `401 Unauthorized`: Not authenticated

**Example**:
```bash
curl -X POST http://localhost:8080/api/auth/totp/verify \
  -H "Authorization: Bearer your_token_here" \
  -H "Content-Type: application/json" \
  -d '{"code":"123456"}'
```

---

### POST /api/auth/totp/disable

Disable 2FA. Requires password and current TOTP code.

**Authentication**: Required
**Rate Limited**: Yes (5/min per IP)

**Request Body**:
```json
{
  "password": "your_password",
  "code": "123456"
}
```

**Response** `200 OK`:
```json
{
  "message": "2FA has been disabled successfully",
  "totp_enabled": false
}
```

**Errors**:
- `400 Bad Request`: 2FA not enabled or invalid code
- `401 Unauthorized`: Invalid password or not authenticated

**Example**:
```bash
curl -X POST http://localhost:8080/api/auth/totp/disable \
  -H "Authorization: Bearer your_token_here" \
  -H "Content-Type: application/json" \
  -d '{"password":"your_password","code":"123456"}'
```

---

### POST /api/auth/totp/recover

Disable 2FA using a recovery code (for account recovery).

**Authentication**: Not required (uses email + password + recovery code)
**Rate Limited**: Yes (5/min per IP)

**Request Body**:
```json
{
  "email": "user@example.com",
  "password": "your_password",
  "recovery_code": "ABCD-1234"
}
```

**Response** `200 OK`:
```json
{
  "message": "2FA has been disabled. Please set up 2FA again if you want to re-enable it.",
  "token": "new_session_token",
  "totp_enabled": false
}
```

**Note**: Using a recovery code:
- Disables 2FA on the account
- Invalidates all existing sessions
- Creates a new session for the user
- Marks the recovery code as used (single-use)

**Errors**:
- `400 Bad Request`: 2FA not enabled or missing fields
- `401 Unauthorized`: Invalid credentials or recovery code

**Example**:
```bash
curl -X POST http://localhost:8080/api/auth/totp/recover \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"your_password","recovery_code":"ABCD-1234"}'
```

---

### POST /api/auth/totp/codes

Regenerate recovery codes. Requires password and current TOTP code.

**Authentication**: Required
**Rate Limited**: Yes (5/min per IP)

**Request Body**:
```json
{
  "password": "your_password",
  "code": "123456"
}
```

**Response** `200 OK`:
```json
{
  "message": "Recovery codes regenerated successfully",
  "recovery_codes": [
    "ABCD-1234",
    "EFGH-5678",
    ...
  ]
}
```

**Note**: Regenerating codes invalidates all previous recovery codes.

**Errors**:
- `400 Bad Request`: 2FA not enabled or invalid TOTP code
- `401 Unauthorized`: Invalid password or not authenticated

**Example**:
```bash
curl -X POST http://localhost:8080/api/auth/totp/codes \
  -H "Authorization: Bearer your_token_here" \
  -H "Content-Type: application/json" \
  -d '{"password":"your_password","code":"123456"}'
```

---

## Password Reset

### POST /api/auth/forgot-password

Request a password reset email.

**Rate Limited**: 3 requests per 15 minutes per IP

**Request Body**:
```json
{
  "email": "user@example.com"
}
```

**Response (200)**:
```json
{
  "message": "If an account exists with that email, a password reset link has been sent."
}
```

**Note**: Always returns the same response regardless of whether the email exists (prevents email enumeration).

**Example**:
```bash
curl -X POST http://localhost:8080/api/auth/forgot-password \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com"}'
```

---

### POST /api/auth/reset-password

Reset password using a token from the reset email.

**Rate Limited**: 5 requests per minute per IP

**Request Body**:
```json
{
  "token": "reset_token_from_email",
  "password": "new_secure_password"
}
```

**Response (200)**:
```json
{
  "message": "Password has been reset successfully. Please log in with your new password."
}
```

**Errors**:
- `400 Bad Request`: Missing fields, invalid password (8-72 chars), or invalid/expired token

**Example**:
```bash
curl -X POST http://localhost:8080/api/auth/reset-password \
  -H "Content-Type: application/json" \
  -d '{"token":"abc123...","password":"newpassword123"}'
```

**Notes**:
- Tokens expire after 1 hour
- Tokens can only be used once
- All existing sessions are terminated after password reset

---

## Session Management

### GET /api/auth/sessions

List all active sessions for the current user.

**Requires**: Authentication

**Response (200)**:
```json
{
  "sessions": [
    {
      "id": "session_id",
      "user_id": "user_id",
      "ip_address": "192.168.1.1",
      "user_agent": "Mozilla/5.0...",
      "expires_at": "2026-01-29T12:00:00Z",
      "created_at": "2026-01-22T12:00:00Z",
      "is_current": true
    }
  ]
}
```

**Example**:
```bash
curl http://localhost:8080/api/auth/sessions \
  -H "Authorization: Bearer your_token_here"
```

---

### DELETE /api/auth/sessions/{id}

Revoke a specific session.

**Requires**: Authentication

**Response (200)**:
```json
{
  "message": "Session revoked"
}
```

**Errors**:
- `400 Bad Request`: Cannot revoke current session
- `401 Unauthorized`: Not authenticated
- `403 Forbidden`: Session belongs to different user
- `404 Not Found`: Session not found

**Example**:
```bash
curl -X DELETE http://localhost:8080/api/auth/sessions/session_id \
  -H "Authorization: Bearer your_token_here"
```

---

## Videos

### POST /api/upload

Upload a video file for transcription.

**Authentication**: Optional (affects upload limits)

**Query Parameters**:
| Parameter | Type | Description |
|-----------|------|-------------|
| `session_id` | string | Anonymous session ID for tracking uploads |

**Request**: `multipart/form-data`
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `video` | file | Yes | Video file (max 500 MB) |

**Response** `200 OK`:
```json
{
  "status": "success",
  "upload_id": "abc123def456...",
  "filename": "my_video.mp4",
  "size": 12345678,
  "message": "File uploaded successfully (12345678 bytes)"
}
```

**Upload Limits**:
- Anonymous users: 2 uploads per session
- Authenticated users: Unlimited

**Errors**:
- `400 Bad Request`: No file, invalid file type, or file too large
- `403 Forbidden`: Anonymous upload limit reached

**Example**:
```bash
# Authenticated upload
curl -X POST http://localhost:8080/api/upload \
  -H "Authorization: Bearer your_token_here" \
  -F "video=@my_video.mp4"

# Anonymous upload with session tracking
curl -X POST "http://localhost:8080/api/upload?session_id=my-session-123" \
  -F "video=@my_video.mp4"
```

---

### GET /api/videos

List uploaded videos.

**Authentication**: Optional (filters by user if authenticated)

**Query Parameters**:
| Parameter | Type | Description |
|-----------|------|-------------|
| `session_id` | string | Filter by anonymous session ID |

**Response** `200 OK`:
```json
{
  "videos": [
    {
      "id": "abc123...",
      "filename": "my_video.mp4",
      "size": 12345678,
      "content_type": "video/mp4",
      "created_at": "2026-01-22T10:00:00Z",
      "transcription_status": "complete",
      "expires_at": "2026-04-22T10:00:00Z"
    }
  ]
}
```

| Field | Description |
|-------|-------------|
| `expires_at` | When the video will be deleted (48h for anonymous, 90d for registered users) |

| `transcription_status` | Description |
|------------------------|-------------|
| `none` | No transcription started |
| `pending` | Waiting to start |
| `processing` | Transcription in progress |
| `complete` | Transcription finished |
| `error` | Transcription failed |

**Example**:
```bash
# List videos for authenticated user
curl http://localhost:8080/api/videos \
  -H "Authorization: Bearer your_token_here"

# List videos for anonymous session
curl "http://localhost:8080/api/videos?session_id=my-session-123"
```

---

### GET /api/videos/{id}/video

Stream or download the original video file.

**Authentication**: Not required

**Response**: Video file with appropriate `Content-Type`

**Example**:
```bash
curl -o video.mp4 http://localhost:8080/api/videos/abc123/video
```

---

## Transcription

### POST /api/transcribe/{id}

Start transcription for an uploaded video.

**Authentication**: Not required

**Response** `200 OK` (started):
```json
{
  "status": "processing",
  "message": "Transcription started"
}
```

**Response** `200 OK` (already processing):
```json
{
  "status": "processing",
  "message": "Transcription already in progress"
}
```

**Response** `200 OK` (already complete):
```json
{
  "status": "complete",
  "message": "",
  "progress": 100,
  "result": {
    "language": "en",
    "duration": 120.5,
    "text": "Full transcript text...",
    "segments": [
      {
        "id": 0,
        "start": 0.0,
        "end": 2.5,
        "text": "Hello world"
      }
    ]
  }
}
```

**Errors**:
- `400 Bad Request`: Missing upload ID
- `404 Not Found`: Video not found

**Example**:
```bash
curl -X POST http://localhost:8080/api/transcribe/abc123
```

---

### GET /api/transcribe/{id}

Get transcription status or result.

**Authentication**: Not required

**Response** `200 OK`:
```json
{
  "status": "complete",
  "message": "",
  "progress": 100,
  "result": {
    "language": "en",
    "duration": 120.5,
    "text": "Full transcript text...",
    "segments": [
      {
        "id": 0,
        "start": 0.0,
        "end": 2.5,
        "text": "Hello world"
      }
    ]
  }
}
```

| `status` | Description |
|----------|-------------|
| `pending` | Waiting to start |
| `processing` | In progress (check `progress` 0-100) |
| `complete` | Finished (includes `result`) |
| `error` | Failed (check `message`) |

**Errors**:
- `404 Not Found`: No transcription found

**Example**:
```bash
curl http://localhost:8080/api/transcribe/abc123
```

---

### PUT /api/transcribe/{id}/segments

Update subtitle segments (edit subtitles).

**Authentication**: Not required

**Request Body**:
```json
{
  "segments": [
    {
      "id": 0,
      "start": 0.0,
      "end": 2.5,
      "text": "Updated subtitle text"
    },
    {
      "id": 1,
      "start": 2.5,
      "end": 5.0,
      "text": "Second subtitle"
    }
  ]
}
```

**Validation Rules**:
- `start` and `end` must be non-negative
- `start` must be less than or equal to `end`

**Response** `200 OK`:
```json
{
  "status": "success",
  "segments": 2
}
```

**Errors**:
- `400 Bad Request`: Invalid timing or transcription not complete
- `404 Not Found`: No transcription found

**Example**:
```bash
curl -X PUT http://localhost:8080/api/transcribe/abc123/segments \
  -H "Content-Type: application/json" \
  -d '{"segments":[{"id":0,"start":0,"end":2.5,"text":"Hello"}]}'
```

---

### POST /api/transcribe/{id}/align

Align user-provided transcript with whisper timing (paste-and-match).

**Authentication**: Not required

**Request Body**:
```json
{
  "text": "Known lyrics or transcript\nLine 2\nLine 3",
  "mode": "lyrics"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `text` | string | Yes | User-provided transcript (newlines = segment boundaries) |
| `mode` | string | No | `"lyrics"` for music-specific alignment |

**Response** `200 OK`:
```json
{
  "status": "success",
  "segments": 3,
  "stats": {
    "match_rate": 0.95,
    "user_word_count": 50,
    "matched_word_count": 47
  },
  "mode": "standard"
}
```

**Mode Differences**:
- `standard`: General transcript alignment
- `lyrics`: Music-specific alignment with:
  - Vocal substitution handling (you→ooh, I→ah)
  - Elongated word recognition (loooove→love)
  - Needleman-Wunsch global alignment

**Errors**:
- `400 Bad Request`: Empty text or transcription not complete
- `404 Not Found`: No transcription found

**Example**:
```bash
curl -X POST http://localhost:8080/api/transcribe/abc123/align \
  -H "Content-Type: application/json" \
  -d '{"text":"Hello world\nThis is line two","mode":"lyrics"}'
```

---

## Subtitles

### GET /api/videos/{id}/subtitles.srt

Download SRT subtitle file.

**Authentication**: Not required

**Response**: SRT file (`text/plain; charset=utf-8`)
```
1
00:00:00,000 --> 00:00:02,500
Hello world

2
00:00:02,500 --> 00:00:05,000
This is line two

```

**Headers**:
- `Content-Type: text/plain; charset=utf-8`
- `Content-Disposition: attachment; filename="abc123.srt"`

**Errors**:
- `400 Bad Request`: Transcription not complete
- `404 Not Found`: No transcription or segments found

**Example**:
```bash
curl -o subtitles.srt http://localhost:8080/api/videos/abc123/subtitles.srt
```

---

### POST /api/videos/{id}/burn

Start burning subtitles into video file.

**Authentication**: Not required

**Response** `200 OK` (started):
```json
{
  "status": "processing",
  "message": "Subtitle burn started",
  "progress": 0
}
```

**Response** `200 OK` (already complete):
```json
{
  "status": "complete",
  "message": "Subtitles burned successfully",
  "progress": 100
}
```

**Errors**:
- `400 Bad Request`: Transcription not complete
- `404 Not Found`: Video or transcription not found

**Example**:
```bash
curl -X POST http://localhost:8080/api/videos/abc123/burn
```

---

### GET /api/videos/{id}/burn

Get subtitle burn job status.

**Authentication**: Not required

**Response** `200 OK`:
```json
{
  "status": "processing",
  "message": "Burning subtitles into video...",
  "progress": 45
}
```

| `status` | Description |
|----------|-------------|
| `processing` | In progress |
| `complete` | Finished, ready for download |
| `error` | Failed (check `message`) |

**Errors**:
- `404 Not Found`: No burn job found

**Example**:
```bash
curl http://localhost:8080/api/videos/abc123/burn
```

---

### GET /api/videos/{id}/burned

Download video with burned-in subtitles.

**Authentication**: Not required

**Response**: Video file with `Content-Disposition: attachment`

**Headers**:
- `Content-Disposition: attachment; filename="original_name_subtitled.mp4"`

**Errors**:
- `400 Bad Request`: Burn job not complete
- `404 Not Found`: No burn job or output file

**Example**:
```bash
curl -o video_with_subs.mp4 http://localhost:8080/api/videos/abc123/burned
```

---

## Error Response Format

All errors return JSON:
```json
{
  "error": "Error message description"
}
```

Some endpoints include additional context:
```json
{
  "error": "Cannot edit segments - transcription not complete",
  "status": "processing"
}
```

---

## Script Conversion

Endpoints for detecting and converting scripts (e.g., romanized Hindi to Devanagari).

### POST /api/text/detect-script

Detect the writing script of text.

**Authentication**: Not required

**Rate Limiting**: 10 requests per minute per IP

**Request**:
```json
{
  "text": "namaste duniya"
}
```

**Response** `200 OK`:
```json
{
  "detected_script": "Latin",
  "detected_language": "hi",
  "confidence": 0.85
}
```

| Field | Description |
|-------|-------------|
| `detected_script` | Detected writing system (Latin, Devanagari, Bengali, etc.) |
| `detected_language` | Guessed language code for romanized text |
| `confidence` | Confidence score (0-1) |

**Errors**:
- `400 Bad Request`: Text too long (max 10KB)

**Example**:
```bash
curl -X POST http://localhost:8080/api/text/detect-script \
  -H "Content-Type: application/json" \
  -d '{"text": "namaste duniya"}'
```

---

### POST /api/text/convert

Convert text from one script to another (e.g., romanized to native script).

**Authentication**: Not required

**Rate Limiting**: 10 requests per minute per IP

**Request**:
```json
{
  "text": "namaste duniya",
  "target_script": "Devanagari",
  "language": "hi",
  "source_script": "Latin"
}
```

| Field | Required | Description |
|-------|----------|-------------|
| `text` | Yes | Text to convert |
| `target_script` | Yes | Target script (Devanagari, Bengali, Tamil, etc.) |
| `language` | Yes | Language code (hi, ml, ta, te, kn, bn, gu, or, pa) |
| `source_script` | No | Auto-detected if omitted |

**Response** `200 OK`:
```json
{
  "original": "namaste duniya",
  "converted": "नमसते दुनिय",
  "source_script": "Latin",
  "target_script": "Devanagari",
  "language": "hi"
}
```

**Supported Scripts** (as targets):
- Devanagari, Bengali, Tamil, Telugu, Kannada, Malayalam, Gujarati, Gurmukhi, Oriya

**Errors**:
- `400 Bad Request`: Unsupported language or script
- `400 Bad Request`: Text too long (max 10KB)

**Example**:
```bash
curl -X POST http://localhost:8080/api/text/convert \
  -H "Content-Type: application/json" \
  -d '{"text": "namaste", "target_script": "Devanagari", "language": "hi"}'
```

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Backend server port |
| `WHISPER_MODEL` | `~/Github/whisper.cpp/models/ggml-medium.bin` | Path to whisper model (CLI mode) |
| `WHISPER_SERVER_URL` | `http://127.0.0.1:8765` | Whisper server URL |
| `USE_WHISPER_SERVER` | `false` | Set to `true` to use whisper-server |
| `HTTPS_ONLY` | `false` | Set to `true` to enable Secure flag on session cookies |
| `RESEND_API_KEY` | *(none)* | Resend API key for transactional emails |
| `EMAIL_FROM` | `noreply@subtitler.app` | Sender email address |
| `APP_URL` | `http://localhost:4321` | Base URL for email links |
| `EMAIL_ENABLED` | `true` | Set to `false` to disable email sending |

See [backend/README.md](../backend/README.md) for detailed environment configuration.

---

## Related Documentation

- [INSTALL.md](../INSTALL.md) - Installation instructions
- [TESTING.md](../TESTING.md) - Testing guide
- [specs/auth.md](../specs/auth.md) - Authentication system spec
- [specs/totp.md](../specs/totp.md) - 2FA specification
- [specs/encryption.md](../specs/encryption.md) - File encryption spec
