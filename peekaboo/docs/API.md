# Peekaboo API Documentation

This document describes the HTTP API endpoints exposed by the Peekaboo backend.

## Base URL

The API is served on the port specified by the `PORT` environment variable (default: `8080`).

```
http://localhost:8080
```

## Endpoints

### Health Check

Check if the server is running.

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

### Transcribe Audio

Convert audio to text using the whisper-server.

```
POST /api/transcribe
```

#### Request

- **Content-Type**: `multipart/form-data`
- **Max Size**: 10MB

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

Valid concept format: lowercase letters, numbers, and underscores only.

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

## CORS

The API supports CORS with the following configuration:

- **Allowed Origin**: Set via `ALLOWED_ORIGIN` environment variable (default: `*` for development)
- **Allowed Methods**: `GET`, `POST`, `OPTIONS`
- **Allowed Headers**: `Content-Type`

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
| 500 | Internal Server Error |

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
