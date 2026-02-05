# Security Documentation

This document describes the threat model for Peekaboo and the implemented mitigations.

## Threat Model

Peekaboo is a voice-controlled web app for children. The threat model assumes:

- **Attacker profile**: Casual to moderate attacker targeting web applications
- **Attack surface**: HTTP API endpoints, file serving, client-side JavaScript
- **Protected assets**: Media files, server stability, user experience
- **Out of scope**: Physical access, insider threats, advanced persistent threats

## Implemented Mitigations

### 1. Cross-Site Scripting (XSS)

**Threat**: Injection of malicious scripts through user input.

**Mitigations**:
| Control | Location | Implementation |
|---------|----------|----------------|
| Content-Security-Policy | `api/security.go:14` | Restricts script-src to 'self' |
| X-XSS-Protection | `api/security.go:22` | Legacy browser XSS filtering |
| Content-Type headers | `api/encrypted_media.go:78-117` | Explicit MIME types prevent sniffing |

**CSP Directives**:
```
default-src 'self';
script-src 'self';
style-src 'self' 'unsafe-inline';
img-src 'self' data: blob:;
media-src 'self' blob:;
connect-src 'self';
frame-ancestors 'none'
```

### 2. SQL Injection

**Threat**: Malicious SQL via user input to manipulate database queries.

**Mitigations**:
| Query | Location | Parameterization |
|-------|----------|------------------|
| Seed concepts | `db/db.go:110-112` | `INSERT ... VALUES (?, ?)` |
| Seed media sets | `db/db.go:125-127` | `INSERT ... VALUES (?, ?, ?, ?)` |
| Get random media | `db/db.go:138-144` | `WHERE concept_id = ?` |
| Get concept | `db/db.go:166` | `WHERE id = ?` |

All queries use parameterized statements with `?` placeholders. No string interpolation.

### 3. Path Traversal

**Threat**: Accessing files outside the intended directory via `../` sequences.

**Mitigations**:
| Control | Location | Implementation |
|---------|----------|----------------|
| Path sanitization | `api/encrypted_media.go:40` | `filepath.Clean()` removes `..` |
| Concept ID validation | `api/media.go:15` | Regex `^[a-z0-9_]+$` |

Invalid concept IDs return 400 Bad Request with "invalid concept format" message.

### 4. Denial of Service (DoS)

**Threat**: Overwhelming the server with requests or large payloads.

**Mitigations**:
| Control | Location | Limit |
|---------|----------|-------|
| Rate limiting | `api/ratelimit.go:78-91` | 10 req/min per IP |
| Multipart form size | `api/transcribe.go:79` | 10 MB max |
| Audio file min size | `api/transcribe.go:93-97` | 1 KB min |
| Audio file max size | `api/transcribe.go:100-104` | 5 MB max |
| Text length limit | `api/intent.go:68-71` | 500 chars max |

Rate limiter applied to expensive endpoints (`/api/transcribe`, `/api/intent`, `/ws/audio`) in `main.go`.

WebSocket connections are rate-limited BEFORE upgrade - returns HTTP 429 if limit exceeded.

Cleanup goroutine prevents rate limiter memory growth (`main.go:79-90`).

### 5. Clickjacking

**Threat**: Embedding the app in an iframe to trick users.

**Mitigations**:
| Control | Location | Value |
|---------|----------|-------|
| X-Frame-Options | `api/security.go:20` | `DENY` |
| frame-ancestors | `api/security.go:14` | `'none'` (in CSP) |

### 6. Information Leakage

**Threat**: Exposing internal details through error messages.

**Mitigations**:
| Endpoint | Location | Behavior |
|----------|----------|----------|
| /api/media | `api/media.go:65,80` | Generic "media lookup failed" |
| /api/intent | `api/intent.go:83` | Generic "intent extraction failed" |
| /api/transcribe | `api/transcribe.go:112` | Generic "transcription failed" |
| Rate limit | `api/ratelimit.go:86` | Generic "rate limit exceeded" |

Actual errors logged server-side with request ID for debugging.

### 7. CORS Misconfiguration

**Threat**: Cross-origin requests from unauthorized domains (including WebSocket connections).

**Mitigations**:
| Control | Location | Implementation |
|---------|----------|----------------|
| HTTP CORS middleware | `api/cors.go:14-39` | Validates `ALLOWED_ORIGIN` for HTTP requests |
| WebSocket origin check | `api/websocket.go:151-159` | Validates Origin header before upgrade |
| Warning for wildcard | `main.go:98-102` | Logs warning if `*` used |

**HTTP requests**: Standard CORS headers applied via middleware.

**WebSocket connections**: Origin validated using `websocket.AcceptOptions.OriginPatterns`. If `ALLOWED_ORIGIN` is `*` or empty, `InsecureSkipVerify` is used (development only). In production, only connections from the specified origin are accepted; others receive HTTP 403.

Production should set `ALLOWED_ORIGIN` to the actual frontend domain (e.g., `https://peekaboo.example.com`).

### 8. MIME Sniffing

**Threat**: Browser interpreting files as different types than intended.

**Mitigations**:
| Control | Location | Implementation |
|---------|----------|----------------|
| X-Content-Type-Options | `api/security.go:21` | `nosniff` |
| Explicit Content-Type | `api/encrypted_media.go:78-117` | Based on file extension |

### 9. Encryption at Rest

**Threat**: Unauthorized access to stored media files.

**Mitigations**:
| Control | Location | Implementation |
|---------|----------|----------------|
| Age encryption | `crypto/age.go` | X25519 encryption |
| On-demand decrypt | `api/encrypted_media.go:62-89` | Decrypts when serving |
| Key file permissions | `crypto/age.go:167-184` | 0600 on key file |

## Remaining Risks

### Accepted Risks

1. **No authentication**: By design, this is a public-facing app for children. No user accounts.

2. **LLM API keys in environment**: Keys stored in `.env` file. Mitigated by file permissions and sops encryption for deployment.

3. **Whisper server trust**: Backend trusts whisper-server responses. Mitigated by running whisper-server locally.

4. **unsafe-inline styles**: Required for Astro framework. Limited risk as no user-generated styles.

### Out of Scope

1. **DDoS attacks**: Requires infrastructure-level mitigation (CDN, WAF)
2. **API key brute force**: Mitigated by provider-side rate limiting
3. **Supply chain attacks**: Out of scope for application-level security
4. **Browser vulnerabilities**: Relies on browser security features

## Security Testing

### Automated Tests

Security-related tests exist in:
- `api/security_test.go` - Security headers verification
- `api/ratelimit_test.go` - Rate limiting behavior
- `api/media_test.go` - Input validation (path traversal prevention)

Run with:
```bash
cd backend && go test ./api -v -run Security
cd backend && go test ./api -v -run RateLimit
cd backend && go test ./api -v -run InvalidConcept
```

### Manual Testing

1. **XSS**: Verify CSP blocks inline scripts in browser console
2. **Path traversal**: `curl localhost:8080/api/media/../etc/passwd` returns 400
3. **Rate limiting**: Send 11 requests in 1 minute, verify 429 response
4. **CORS**: Make cross-origin request from different domain, verify block

## Security Checklist for Deployment

- [ ] Set `ALLOWED_ORIGIN` to production domain (not `*`)
- [ ] Use HTTPS (Caddy auto-HTTPS recommended)
- [ ] Restrict age key file permissions (`chmod 600`)
- [ ] Store API keys in encrypted secrets (sops)
- [ ] Enable JSON logging for security audit trail (`LOG_FORMAT=json`)
- [ ] Configure firewall to restrict whisper-server access to backend only

## Reporting Security Issues

Security issues should be reported to the project maintainer privately before public disclosure.
