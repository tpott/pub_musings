# Security Documentation

This document describes the threat model for Peekaboo and the implemented mitigations.

## Threat Model

Peekaboo is a voice-controlled web app for children. The threat model assumes:

- **Attacker profile**: Casual to moderate attacker targeting web applications
- **Attack surface**: HTTP API endpoints, file serving, WebSocket connections, client-side JavaScript
- **Protected assets**: User accounts, media files, server stability, user experience
- **Out of scope**: Physical access, insider threats, advanced persistent threats

## Implemented Mitigations

### 1. Cross-Site Scripting (XSS)

**Threat**: Injection of malicious scripts through user input.

**Mitigations**:
| Control | Location | Implementation |
|---------|----------|----------------|
| Content-Security-Policy | `api/security.go` `ContentSecurityPolicy` | Restricts script-src to 'self' |
| X-XSS-Protection | `api/security.go` `SecurityHeadersMiddleware()` | Legacy browser XSS filtering |
| Content-Type headers | `api/encrypted_media.go` `getContentType()` | Explicit MIME types prevent sniffing |

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
| Seed concepts | `db/db.go` `Init()` | `INSERT ... VALUES (?, ?)` |
| Seed media sets | `db/db.go` `SeedMediaSet()` | `INSERT ... VALUES (?, ?, ?, ?)` |
| Get random media | `db/db.go` `GetRandomMediaSet()` | `WHERE concept_id = ?` |
| Get concept | `db/db.go` `GetConcept()` | `WHERE id = ?` |

All queries use parameterized statements with `?` placeholders. No string interpolation.

### 3. Path Traversal

**Threat**: Accessing files outside the intended directory via `../` sequences.

**Mitigations**:
| Control | Location | Implementation |
|---------|----------|----------------|
| Path sanitization | `api/encrypted_media.go` `ServeHTTP()` | `filepath.Clean()` removes `..` |
| Concept ID validation | `api/media.go` `validConceptPattern` | Regex `^[a-z0-9_]+$` |

Invalid concept IDs return 400 Bad Request with "invalid concept format" message.

### 4. Denial of Service (DoS)

**Threat**: Overwhelming the server with requests or large payloads.

**Mitigations**:
| Control | Location | Limit |
|---------|----------|-------|
| ReadHeaderTimeout | `main.go` `main()` | 10s (prevents slowloris) |
| IdleTimeout | `main.go` `main()` | 120s (closes idle conns) |
| Rate limiting | `api/ratelimit.go` `RateLimitMiddleware()` | 10 req/min per IP |
| Multipart form size | `api/transcribe.go` `ServeHTTP()` | 10 MB max |
| Audio file min size | `api/transcribe.go` `ServeHTTP()` | 1 KB min |
| Audio file max size | `api/transcribe.go` `ServeHTTP()` | 5 MB max |
| Text length limit | `api/intent.go` `ServeHTTP()` | 500 chars max |

Rate limiter applied to expensive endpoints (`/api/transcribe`, `/api/intent`, `/ws/audio`) in `main.go`.

WebSocket connections are rate-limited BEFORE upgrade - returns HTTP 429 if limit exceeded.

Cleanup goroutine prevents rate limiter memory growth (see `main.go` startup).

**IP Extraction**: By default, `getClientIP()` uses only `RemoteAddr` to prevent clients from spoofing their IP via `X-Forwarded-For` or `X-Real-IP` headers to bypass rate limiting. Set `TRUST_PROXY_HEADERS=true` only when running behind a trusted reverse proxy that sets these headers.

### 5. Clickjacking

**Threat**: Embedding the app in an iframe to trick users.

**Mitigations**:
| Control | Location | Value |
|---------|----------|-------|
| X-Frame-Options | `api/security.go` `SecurityHeadersMiddleware()` | `DENY` |
| frame-ancestors | `api/security.go` `ContentSecurityPolicy` | `'none'` (in CSP) |

### 6. Information Leakage

**Threat**: Exposing internal details through error messages.

**Mitigations**:
| Endpoint | Location | Behavior |
|----------|----------|----------|
| /api/media | `api/media.go` `ServeHTTP()` | Generic "media lookup failed" |
| /api/intent | `api/intent.go` `ServeHTTP()` | Generic "intent extraction failed" |
| /api/transcribe | `api/transcribe.go` `ServeHTTP()` | Generic "transcription failed" |
| Rate limit | `api/ratelimit.go` `RateLimitMiddleware()` | Generic "rate limit exceeded" |

Actual errors logged server-side with request ID for debugging.

### 7. CORS Misconfiguration

**Threat**: Cross-origin requests from unauthorized domains (including WebSocket connections).

**Mitigations**:
| Control | Location | Implementation |
|---------|----------|----------------|
| HTTP CORS middleware | `api/cors.go` `CORSMiddleware()` | Validates `ALLOWED_ORIGIN` for HTTP requests |
| WebSocket origin check | `api/websocket.go` `ServeHTTP()` | Validates Origin header before upgrade |
| Warning for wildcard | `main.go` `main()` | Logs warning if `*` used |

**HTTP requests**: Standard CORS headers applied via middleware.

**WebSocket connections**: Origin validated using `websocket.AcceptOptions.OriginPatterns`. If `ALLOWED_ORIGIN` is `*` or empty, `InsecureSkipVerify` is used (development only). In production, only connections from the specified origin are accepted; others receive HTTP 403.

Production should set `ALLOWED_ORIGIN` to the actual frontend domain (e.g., `https://peekaboo.example.com`).

### 8. MIME Sniffing

**Threat**: Browser interpreting files as different types than intended.

**Mitigations**:
| Control | Location | Implementation |
|---------|----------|----------------|
| X-Content-Type-Options | `api/security.go` `SecurityHeadersMiddleware()` | `nosniff` |
| Explicit Content-Type | `api/encrypted_media.go` `getContentType()` | Based on file extension |

### 9. Encryption at Rest

**Threat**: Unauthorized access to stored media files.

**Mitigations**:
| Control | Location | Implementation |
|---------|----------|----------------|
| Age encryption | `crypto/age.go` | X25519 encryption |
| On-demand decrypt | `api/encrypted_media.go` `serveEncrypted()` | Decrypts when serving |
| Key file permissions | `crypto/age.go` `LoadIdentityFromFile()` | 0600 on key file |

### 10. Authentication and Sessions

**Threat**: Unauthorized access, credential theft, session hijacking.

**Mitigations**:
| Control | Location | Implementation |
|---------|----------|----------------|
| Password hashing | `auth/auth.go` `HashPassword()` | bcrypt with cost 12 |
| Session tokens | `auth/auth.go` `GenerateToken()` | 32-byte crypto/rand tokens |
| Session cookie | `api/handlers_auth.go` `HandleLogin()` | HttpOnly, SameSite=Lax, optional Secure flag |
| Session duration | `auth/auth.go` `SessionDuration` | 30 days |
| CSRF protection | `api/csrf.go` `CSRFMiddleware()` | HMAC-SHA256 token validated on state-changing requests |
| Account lockout | `api/handlers_auth.go` `HandleLogin()` | 5 failures in 15 min → locked 15 min |
| Email verification | `api/handlers_auth.go` `HandleRegister()` | Required before login |
| Generic errors | `api/handlers_auth.go` `HandleLogin()` | "invalid email or password" (prevents enumeration) |
| WebSocket auth | `api/websocket.go` `ServeHTTP()` | Session cookie extracted before WS upgrade |
| Per-user WS limits | `api/websocket_auth.go` `WSAuthTracker` | 3 concurrent WS per user, 1 per anon IP |
| Anon rate limiting | `api/websocket_auth.go` `AllowAnonInteraction()` | 30 interactions/hour per IP |
| TOTP 2FA | `auth/totp.go` `ValidateTOTP()` | HMAC-SHA1 per RFC 6238, ±1 period skew, password required to enable/disable |

**CSRF-exempt paths**: `/api/auth/login`, `/api/auth/register`, `/api/auth/resend-verification`, `/api/auth/magic-link` (these accept credentials directly).

**Security headers**:
| Header | Value | Location |
|--------|-------|----------|
| Referrer-Policy | `strict-origin-when-cross-origin` | `api/security.go` `SecurityHeadersMiddleware()` |
| Permissions-Policy | `microphone=(self), camera=(), geolocation=(), payment=(), usb=(), interest-cohort=()` | `api/security.go` `SecurityHeadersMiddleware()` |
| Strict-Transport-Security | `max-age=31536000; includeSubDomains` | `api/security.go` (when `HTTPS_ONLY=true`) |

## Remaining Risks

### Accepted Risks

1. **LLM API keys in environment**: Keys stored in `.env` file. Mitigated by file permissions and sops encryption for deployment.

2. **Whisper server trust**: Backend trusts whisper-server responses. Mitigated by running whisper-server locally.

3. **unsafe-inline styles**: Required for Astro framework. Limited risk as no user-generated styles.

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
- `api/handlers_auth_test.go` - Registration, verification, email enumeration prevention
- `api/handlers_auth_login_test.go` - Login, lockout, TOTP flow
- `api/handlers_auth_csrf_test.go` - CSRF middleware validation
- `api/handlers_auth_magiclink_test.go` - Magic link auth
- `api/handlers_auth_totp_test.go` - TOTP setup, enable, disable, login validation
- `api/websocket_auth_test.go` - WebSocket per-user/per-IP limits
- `auth/auth_test.go` - Password hashing, token generation, CSRF primitives
- `auth/totp_test.go` - TOTP code generation, validation, RFC 6238 vectors

Run with:
```bash
cd backend && go test ./api -v -run Security
cd backend && go test ./api -v -run RateLimit
cd backend && go test ./api -v -run InvalidConcept
cd backend && go test ./api -v -run Auth
cd backend && go test ./api -v -run CSRF
cd backend && go test ./auth -v
```

### Manual Testing

1. **XSS**: Verify CSP blocks inline scripts in browser console
2. **Path traversal**: `curl localhost:8080/api/media/../etc/passwd` returns 400
3. **Rate limiting**: Send 11 requests in 1 minute, verify 429 response
4. **CORS**: Make cross-origin request from different domain, verify block

## Security Checklist for Deployment

- [ ] Set `TRUST_PROXY_HEADERS=true` if behind a reverse proxy (default: false)
- [ ] Set `ALLOWED_ORIGIN` to production domain (not `*`)
- [ ] Use HTTPS (Caddy auto-HTTPS recommended)
- [ ] Set `HTTPS_ONLY=true` (sets Secure flag on session cookies, enables HSTS)
- [ ] Set `CSRF_SECRET` to a stable random value (default: random per restart)
- [ ] Restrict age key file permissions (`chmod 600`)
- [ ] Store API keys in encrypted secrets (sops)
- [ ] Enable JSON logging for security audit trail (`LOG_FORMAT=json`)
- [ ] Configure firewall to restrict whisper-server access to backend only

## Reporting Security Issues

Security issues should be reported to the project maintainer privately before public disclosure.
