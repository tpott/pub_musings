# Rate Limiting Reference

This document provides a complete reference for all rate-limited endpoints in the Subtitler API.

## Overview

Rate limiting protects the API from abuse and ensures fair usage across all users. When a rate limit is exceeded, the server returns `429 Too Many Requests` with a `Retry-After` header indicating when the client can retry.

All rate limits are applied **per IP address** using a sliding window algorithm.

## Rate Limit Categories

| Category | Limit | Window | Endpoints |
|----------|-------|--------|-----------|
| **Authentication** | 5 requests | 1 minute | Register, Login, 2FA endpoints |
| **Password Reset** | 3 requests | 15 minutes | Forgot password (stricter to prevent enumeration) |
| **Upload** | 10 requests | 1 minute | Video upload |
| **Transcription** | 5 requests | 1 minute | Start transcription, Reprocess |
| **Burn** | 2 requests | 1 minute | Burn subtitles into video |
| **Download** | 30 requests | 1 minute | Video, thumbnail, burned video downloads |
| **Script Conversion** | 10 requests | 1 minute | Script detection, Text conversion |
| **Metrics** | 10 requests | 1 minute | Prometheus metrics endpoint |
| **Feedback** | 5 requests | 1 minute | User feedback submission and admin management |
| **Chunked Upload** | 60 requests | 1 minute | Upload chunks for large files |
| **Log** | 30 requests | 1 minute | Frontend log forwarding |

## Endpoints by Category

### Authentication (5 req/min)

| Endpoint | Description |
|----------|-------------|
| `POST /api/auth/register` | Create new user account |
| `POST /api/auth/login` | Authenticate and get session |
| `POST /api/auth/totp/setup` | Start 2FA setup |
| `POST /api/auth/totp/verify` | Complete 2FA setup |
| `POST /api/auth/totp/disable` | Disable 2FA |
| `POST /api/auth/totp/recover` | Account recovery with recovery code |
| `POST /api/auth/totp/codes` | Regenerate recovery codes |
| `POST /api/auth/reset-password` | Complete password reset |
| `GET /api/auth/verify` | Verify email address |
| `GET /api/auth/magic-link/verify` | Verify magic link token |

### Password Reset (3 req/15min)

| Endpoint | Description |
|----------|-------------|
| `POST /api/auth/forgot-password` | Request password reset email |
| `POST /api/auth/resend-verification` | Resend email verification |
| `POST /api/auth/magic-link` | Request magic link login |

**Note**: These endpoints have stricter limits to prevent email enumeration attacks.

### Upload (10 req/min)

| Endpoint | Description |
|----------|-------------|
| `POST /api/upload` | Upload video file |
| `POST /api/upload/init` | Initialize chunked upload session |
| `POST /api/upload/complete` | Complete chunked upload |

### Transcription (5 req/min)

| Endpoint | Description |
|----------|-------------|
| `POST /api/transcribe/{id}` | Start transcription for video |
| `POST /api/videos/{id}/reprocess` | Retry failed transcription |

### Burn (2 req/min)

| Endpoint | Description |
|----------|-------------|
| `POST /api/videos/{id}/burn` | Burn subtitles into video |

**Note**: This endpoint has stricter limits because subtitle burning is CPU-intensive.

### Download (30 req/min)

| Endpoint | Description |
|----------|-------------|
| `GET /api/videos/{id}/video` | Download original video file |
| `GET /api/videos/{id}/thumbnail` | Download video thumbnail |
| `GET /api/videos/{id}/burned` | Download video with burned subtitles |
| `GET /api/videos/{id}/embedded-subtitles/{track}` | Extract embedded subtitle track |
| `GET /api/upload/status/{session_id}` | Check chunked upload status |

**Note**: These limits prevent bandwidth abuse and CPU exhaustion from repeated decryption operations.

### Script Conversion (10 req/min)

| Endpoint | Description |
|----------|-------------|
| `POST /api/text/detect-script` | Detect writing system of text |
| `POST /api/text/convert` | Convert text between scripts |

### Metrics (10 req/min)

| Endpoint | Description |
|----------|-------------|
| `GET /metrics` | Prometheus metrics endpoint |

**Note**: This endpoint is rate limited to prevent reconnaissance attacks.

### Feedback (5 req/min)

| Endpoint | Description |
|----------|-------------|
| `POST /api/feedback` | Submit user feedback |
| `GET /api/admin/feedback` | List all feedback (admin only) |
| `GET /api/admin/feedback/{id}` | Get single feedback entry (admin only) |
| `PATCH /api/admin/feedback/{id}` | Update feedback status (admin only) |

**Note**: All feedback endpoints share the same rate limit. Admin endpoints require admin role authentication. Requires authentication via API key (`X-Metrics-API-Key` header or `api_key` query param) or valid user session.

### Chunked Upload (60 req/min)

| Endpoint | Description |
|----------|-------------|
| `POST /api/upload/chunk` | Upload a single chunk of a large file |

**Note**: Higher limit than regular upload because chunked uploads send many small requests as part of a single logical upload.

### Log (30 req/min)

| Endpoint | Description |
|----------|-------------|
| `POST /api/log` | Forward frontend console logs to backend |

**Note**: Rate limited to prevent log spam and DoS via unlimited log submissions.

## Error Response

When rate limited, the API returns:

```http
HTTP/1.1 429 Too Many Requests
Content-Type: application/json
Retry-After: 60

{
  "error": "Rate limit exceeded. Please try again in 60 seconds."
}
```

**Headers**:
- `Retry-After`: Number of seconds until the rate limit resets

## Per-Email Login Lockout

In addition to per-IP rate limiting, the login endpoint has per-email lockout protection:

- **After 5 failed login attempts** for the same email address, that email is locked for 15 minutes
- This applies regardless of which IP addresses the attempts came from
- Successful login clears the failed attempt counter
- Failed attempts older than 1 hour are automatically cleaned up

This prevents distributed brute force attacks where an attacker uses multiple IP addresses.

## Client Retry Strategies

### Simple Retry

Wait for the `Retry-After` duration before retrying:

```javascript
async function fetchWithRetry(url, options, maxRetries = 3) {
  for (let i = 0; i < maxRetries; i++) {
    const response = await fetch(url, options);

    if (response.status === 429) {
      const retryAfter = parseInt(response.headers.get('Retry-After') || '60');
      await new Promise(resolve => setTimeout(resolve, retryAfter * 1000));
      continue;
    }

    return response;
  }
  throw new Error('Max retries exceeded');
}
```

### Exponential Backoff

For non-rate-limit errors, use exponential backoff:

```javascript
async function fetchWithBackoff(url, options, maxRetries = 3) {
  for (let i = 0; i < maxRetries; i++) {
    try {
      const response = await fetch(url, options);

      if (response.status === 429) {
        const retryAfter = parseInt(response.headers.get('Retry-After') || '60');
        await new Promise(resolve => setTimeout(resolve, retryAfter * 1000));
        continue;
      }

      if (response.status >= 500) {
        // Server error - use exponential backoff
        const delay = Math.pow(2, i) * 1000 + Math.random() * 1000;
        await new Promise(resolve => setTimeout(resolve, delay));
        continue;
      }

      return response;
    } catch (error) {
      if (i === maxRetries - 1) throw error;
      const delay = Math.pow(2, i) * 1000;
      await new Promise(resolve => setTimeout(resolve, delay));
    }
  }
}
```

### Best Practices

1. **Respect rate limits**: Don't retry immediately after a 429 response
2. **Use Retry-After**: The header tells you exactly when to retry
3. **Add jitter**: Random delays prevent thundering herd problems
4. **Log rate limit hits**: Monitor your client's rate limit usage
5. **Batch requests**: Where possible, batch multiple operations
6. **Cache responses**: Reduce API calls by caching where appropriate

## Configuration

Rate limits are configured via environment variables (see [ENV.md](ENV.md) for full reference). Defaults are defined in `backend/config.go`:

| Variable | Default | Description |
|----------|---------|-------------|
| `AUTH_RATE_LIMIT` | `5,1m` | Authentication endpoints |
| `PASSWORD_RESET_RATE_LIMIT` | `3,15m` | Password reset requests |
| `UPLOAD_RATE_LIMIT` | `10,1m` | Video upload |
| `TRANSCRIBE_RATE_LIMIT` | `5,1m` | Transcription requests |
| `BURN_RATE_LIMIT` | `2,1m` | Subtitle burning |
| `DOWNLOAD_RATE_LIMIT` | `30,1m` | Video/thumbnail downloads |
| `SCRIPT_RATE_LIMIT` | `10,1m` | Script conversion |
| `CHUNK_RATE_LIMIT` | `60,1m` | Chunked upload pieces |
| `METRICS_RATE_LIMIT` | `10,1m` | Prometheus metrics |
| `FEEDBACK_RATE_LIMIT` | `5,1m` | Feedback submission |
| `LOG_RATE_LIMIT` | `30,1m` | Frontend log forwarding |
| `USER_RATE_LIMIT` | `60,1m` | Per-user overall rate limit (reserved) |

Format: `count,window` (e.g., `5,1m` = 5 requests per 1 minute).

**Note**: `USER_RATE_LIMIT` is configured but not currently applied to any endpoints. It is a per-user (not per-IP) limiter reserved for future use.

## Related Documentation

- [API Documentation](API.md) - Full API reference
- [Authentication](../specs/auth.md) - Auth system specification
