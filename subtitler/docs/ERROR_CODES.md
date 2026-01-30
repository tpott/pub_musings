# API Error Codes

This document lists all HTTP status codes and error responses returned by the Subtitler API.

## HTTP Status Codes

| Code | Name | Description |
|------|------|-------------|
| 200 | OK | Request succeeded |
| 201 | Created | Resource created successfully |
| 304 | Not Modified | Resource unchanged (caching) |
| 400 | Bad Request | Invalid request data |
| 401 | Unauthorized | Authentication required or failed |
| 403 | Forbidden | Access denied (insufficient permissions) |
| 404 | Not Found | Resource not found |
| 409 | Conflict | Resource already exists |
| 429 | Too Many Requests | Rate limit exceeded |
| 500 | Internal Server Error | Server-side error |
| 503 | Service Unavailable | Service temporarily unavailable |

## Error Response Format

All error responses are JSON objects with an `error` field:

```json
{
  "error": "Error message describing what went wrong"
}
```

Some errors include additional fields for context:

```json
{
  "error": "Unsupported video format",
  "provided_mimetype": "image/jpeg"
}
```

## Authentication Errors (401, 403)

| Error Message | Context |
|---------------|---------|
| `Authentication required` | No valid session token provided |
| `Invalid session` | Session token is expired or invalid |
| `Not authenticated` | User session validation failed |
| `Invalid email or password` | Login credentials incorrect |
| `2FA code required` | Two-factor authentication needed |
| `Invalid 2FA code` | TOTP code verification failed |
| `Please verify your email address before logging in` | Email not verified |
| `Admin access required` | Endpoint requires admin role |

## Rate Limiting (429)

| Error Message | Context |
|---------------|---------|
| `Rate limit exceeded. Please try again later.` | General rate limit hit |
| `Too many failed login attempts. Please try again later.` | Login attempt rate limit |

## Validation Errors (400)

### General Validation

| Error Message | Context |
|---------------|---------|
| `Invalid request body` | JSON parsing failed |
| `Missing required fields: filename, content_type` | Chunked upload init missing fields |
| `{field} required` | Required path parameter missing |
| `Invalid {field} format` | Path parameter format invalid (must be 32-char hex) |

### File Upload

| Error Message | Context |
|---------------|---------|
| `File is empty. Please upload a valid video file.` | Zero-size file upload attempt |
| `File size must be greater than zero. Empty files are not allowed.` | Chunked upload with size <= 0 |
| `File too large or invalid form data` | Upload exceeds size limit |
| `No video file provided` | Missing video in form data |
| `File too large. Maximum size is {N} MB` | Chunked upload size exceeds limit |
| `Unsupported video format. Allowed formats: MP4, WebM, MOV, M4V, MPEG, AVI, MKV, OGV` | Invalid MIME type |
| `File is too small or could not be read` | File smaller than 8 bytes (can't verify format) |

### Chunked Upload

| Error Message | Context |
|---------------|---------|
| `Missing upload_session_id or chunk_index` | Chunk upload missing required fields |
| `Invalid chunk index` | Chunk index out of range |
| `Upload session not found` | Session ID doesn't exist |
| `No chunk data provided` | Chunk request missing file data |
| `Not all chunks received. Expected {N}, got {M}` | Complete called before all chunks uploaded |

### User Registration

| Error Message | Context |
|---------------|---------|
| `Email already registered` | Registration with existing email |
| `Invalid email format` | Email validation failed |
| `Password must be at least 8 characters` | Password too short |
| `CAPTCHA verification failed. Please try again.` | CAPTCHA validation failed |

### Two-Factor Authentication

| Error Message | Context |
|---------------|---------|
| `2FA is already enabled. Disable it first to set up a new authenticator.` | Setup when already enabled |
| `2FA is already enabled` | Verify when already enabled |
| `No TOTP secret found. Please start setup first.` | Verify without setup |
| `2FA is not enabled for this account` | Disable when not enabled |
| `Invalid TOTP code` | Incorrect 6-digit code |

### Session Management

| Error Message | Context |
|---------------|---------|
| `Cannot revoke current session. Use logout instead.` | Revoking own session |
| `Session not found` | Revoking non-existent session |

## Resource Errors (404)

| Error Message | Context |
|---------------|---------|
| `Video not found` | Video ID doesn't exist |
| `No transcription found for this upload` | Video has no transcription |
| `No subtitle segments available` | Transcription has no segments |
| `Upload session not found` | Chunked upload session doesn't exist |
| `Burned video file not found` | Burn job output doesn't exist |

## Processing Errors (400)

| Error Message | Context |
|---------------|---------|
| `Transcription not yet complete` | Download before transcription done |
| `Transcription in progress or failed` | Reprocess while busy/error |
| `Burn job is still processing` | Download before burn complete |
| `Burn job failed` | Download failed burn job |

## Permission Errors (403)

| Error Message | Context |
|---------------|---------|
| `Anonymous users are limited to 2 uploads. Please register to upload more videos.` | Upload limit for non-registered users |
| `Access denied` | Path validation failed (security) |

## Server Errors (500)

| Error Message | Context |
|---------------|---------|
| `Failed to get video` | Database error retrieving video |
| `Failed to get transcription` | Database error retrieving transcription |
| `Failed to list videos` | Database error listing videos |
| `Registration failed` | User creation error |
| `Login failed` | Session creation error |
| `Failed to generate secret` | TOTP secret generation error |
| `Failed to generate QR code` | QR code generation error |
| `Failed to decrypt video` | Encryption key or file error |

## Client Handling Recommendations

### Retry Logic

- **429 Too Many Requests**: Implement exponential backoff with jitter. Check `X-RateLimit-Reset` header.
- **500 Internal Server Error**: Retry up to 3 times with exponential backoff.
- **503 Service Unavailable**: Retry after delay indicated in `Retry-After` header if present.

### User-Facing Messages

For user-facing applications, consider mapping technical errors to friendlier messages:

| API Error | User-Friendly Message |
|-----------|----------------------|
| `Invalid email or password` | "Incorrect email or password. Please try again." |
| `Rate limit exceeded` | "Too many requests. Please wait a moment and try again." |
| `File too large` | "This file is too large. Maximum size is 500 MB." |
| `2FA code required` | "Please enter the 6-digit code from your authenticator app." |

### Error Logging

Log the full error response including any additional fields for debugging:

```javascript
catch (error) {
  console.error('API Error:', {
    status: response.status,
    error: data.error,
    ...data  // Additional context fields
  });
}
```
