# Authentication Specification

This document describes the authentication system for Peekaboo.

## Overview

The authentication system provides:
- Email/password registration with email verification
- Magic link (passwordless) authentication
- Cookie-based sessions with Bearer token support
- Optional two-factor authentication via TOTP
- WebSocket authentication via session cookie

## Motivation

Peekaboo is designed for children to use on a shared family device. Authentication
serves the parent, not the child:
1. **Interaction logging** — tie STT/LLM traces to a household (see [interaction-logging.md](interaction-logging.md))
2. **Usage limits** — prevent abuse from anonymous users
3. **Session persistence** — a parent logs in once, the child uses the device for weeks

## Files

| File | Purpose |
|------|---------|
| `backend/auth/auth.go` | Hashing, session creation, validation |
| `backend/auth/auth_test.go` | Unit tests for auth module |
| `backend/db/db_auth.go` | User, session, token DB operations |
| `backend/db/db_auth_test.go` | Unit tests for auth DB operations |
| `backend/handlers_auth.go` | HTTP handlers for auth endpoints |
| `frontend/src/pages/login.astro` | Login page |
| `frontend/src/pages/register.astro` | Registration page |
| `frontend/src/pages/verify-email.astro` | Email verification page |
| `frontend/src/pages/magic-link.astro` | Magic link login page |

## Database Schema

### Users Table

```sql
CREATE TABLE users (
    id TEXT PRIMARY KEY,                      -- 32-char hex (16 random bytes)
    email TEXT UNIQUE NOT NULL,               -- Normalized to lowercase
    password_hash TEXT NOT NULL,              -- bcrypt hash (cost 12)
    totp_secret TEXT,                         -- Base32 TOTP secret (NULL if not set)
    totp_enabled INTEGER NOT NULL DEFAULT 0,  -- 0=disabled, 1=enabled
    email_verified INTEGER NOT NULL DEFAULT 0,
    verified_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_users_email ON users(email);
```

### Email Verification Tokens Table

```sql
CREATE TABLE email_verification_tokens (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    token_hash TEXT NOT NULL,                 -- SHA-256 hash of token
    expires_at DATETIME NOT NULL,             -- 24 hours from creation
    used INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_email_verification_user_id ON email_verification_tokens(user_id);
CREATE INDEX idx_email_verification_expires ON email_verification_tokens(expires_at);
```

### Magic Link Tokens Table

```sql
CREATE TABLE magic_link_tokens (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    token_hash TEXT NOT NULL,                 -- SHA-256 hash of token
    expires_at DATETIME NOT NULL,             -- 15 minutes from creation
    used INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_magic_link_user_id ON magic_link_tokens(user_id);
CREATE INDEX idx_magic_link_expires ON magic_link_tokens(expires_at);
```

### Sessions Table

```sql
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,                      -- 32-char hex
    user_id TEXT NOT NULL REFERENCES users(id),
    token TEXT UNIQUE NOT NULL,               -- 64-char hex (32 random bytes)
    expires_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_sessions_token ON sessions(token);
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
```

### Login Attempts Table

```sql
CREATE TABLE login_attempts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT NOT NULL,
    ip_address TEXT NOT NULL,
    success INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_login_attempts_email ON login_attempts(email, created_at);
```

## Configuration

| Constant | Value | Description |
|----------|-------|-------------|
| `SessionDuration` | 30 days | Long-lived — parent logs in once, child uses device |
| `BcryptCost` | 12 | bcrypt work factor |
| Min password | 8 chars | Minimum password length |
| Max password | 72 chars | bcrypt hard limit |
| Max email | 255 chars | Email length limit |

## API Endpoints

### POST /api/auth/register

Creates a new user account and sends verification email.

**Request:**
```json
{
  "email": "user@example.com",
  "password": "securepassword"
}
```

**Response (201):**
```json
{
  "message": "Account created. Please check your email to verify your account.",
  "email_verification": true,
  "user": {
    "id": "abc123...",
    "email": "user@example.com",
    "created_at": "2026-02-07T...",
    "email_verified": false
  }
}
```

**Errors:**
- 400: Invalid email format, password too short/long
- 409: Email already registered

**Processing:**
1. Normalize email (trim, lowercase)
2. Validate email format and length
3. Validate password length (8-72 chars)
4. Check email not already registered
5. Hash password with bcrypt (cost 12)
6. Generate user ID (16 random bytes)
7. Insert user (email_verified=false)
8. Generate email verification token (32 random bytes)
9. Store SHA-256 hash of token with 24-hour expiry
10. Send verification email with token link

### POST /api/auth/login

Authenticates user and creates session.

**Request:**
```json
{
  "email": "user@example.com",
  "password": "securepassword",
  "totp_code": "123456"
}
```

`totp_code` is optional — required only if 2FA is enabled.

**Response (200):**
```json
{
  "user": {
    "id": "abc123...",
    "email": "user@example.com",
    "totp_enabled": true
  },
  "token": "sessiontoken..."
}
```

**Response if 2FA required (401):**
```json
{
  "error": "2FA code required",
  "totp_required": true
}
```

**Response if email not verified (403):**
```json
{
  "error": "Please verify your email address before logging in",
  "email_not_verified": true,
  "can_resend_verification": true
}
```

**Errors:**
- 400: Invalid request body
- 401: Invalid credentials or invalid TOTP code
- 403: Email not verified
- 429: Account locked (too many failed attempts)

**Processing:**
1. Normalize email
2. Check account lockout (5 failed attempts in 15 min → locked 15 min)
3. Look up user by email
4. Verify password with bcrypt
5. Record login attempt (success or failure)
6. Check email is verified
7. If 2FA enabled and no code provided: return `totp_required: true`
8. If 2FA enabled and code provided: validate TOTP
9. Create session (30-day expiry)
10. Set httpOnly cookie
11. Clear login attempts on success

### GET /api/auth/verify

Verifies email address using the token from the verification email.

**Query Parameters:**
- `token` (required): Verification token from the email link

**Response (200):**
```json
{
  "message": "Email verified successfully. You can now log in."
}
```

**Errors:**
- 400: Missing, invalid, expired, or already-used token

**Processing:**
1. SHA-256 hash the incoming token
2. Look up token hash in database
3. Check not used and not expired
4. Mark token as used
5. Set user's `email_verified=1` and `verified_at`

### POST /api/auth/resend-verification

Resends the email verification link. Rate limited (3 per 15 minutes per IP).

**Request:**
```json
{
  "email": "user@example.com"
}
```

**Response (200):**
```json
{
  "message": "If an unverified account exists with that email, a verification link has been sent."
}
```

Always returns success to prevent email enumeration.

**Processing:**
1. Normalize email
2. Look up user — if missing or already verified, return success anyway
3. Delete existing unused verification tokens for user
4. Generate new token, store hash with 24-hour expiry
5. Send verification email

### POST /api/auth/magic-link

Sends a magic link login email. Rate limited (5 per minute per IP, 3 per 15 minutes per email).

**Request:**
```json
{
  "email": "user@example.com"
}
```

**Response (200):**
```json
{
  "message": "If an account exists with that email, a login link has been sent."
}
```

Always returns success to prevent email enumeration.

**Processing:**
1. Normalize email
2. Look up user — if missing or email not verified, return success anyway
3. Delete existing unused magic link tokens for user
4. Generate token (32 random bytes), store hash with 15-minute expiry
5. Send magic link email

### GET /api/auth/magic-link/verify

Verifies magic link token and creates a session. Bypasses 2FA (magic link proves
email access).

**Query Parameters:**
- `token` (required): Magic link token from the email

**Response (200):**
```json
{
  "message": "Login successful",
  "user": {
    "id": "abc123...",
    "email": "user@example.com"
  }
}
```

**Errors:**
- 400: Missing, invalid, expired, or already-used token

**Processing:**
1. SHA-256 hash the incoming token
2. Look up token hash — check not expired, not used
3. Mark token as used atomically
4. Create session (30-day expiry)
5. Set httpOnly cookie

### POST /api/auth/logout

Invalidates the current session.

**Requires:** Authentication

**Response (200):**
```json
{"message": "Logged out"}
```

### GET /api/auth/me

Returns current user information.

**Requires:** Authentication

**Response (200):**
```json
{
  "user": {
    "id": "abc123...",
    "email": "user@example.com",
    "totp_enabled": false,
    "email_verified": true,
    "created_at": "2026-02-07T..."
  }
}
```

### GET /api/auth/csrf

Returns a CSRF token derived from the current session.

**Requires:** Authentication

**Response (200):**
```json
{
  "csrf_token": "hmac-derived-token..."
}
```

## Session Management

### Token Extraction Order

1. `Authorization: Bearer <token>` header (API clients)
2. `session` cookie (web browsers)

### Validation

1. Retrieve session by token
2. Check expiration
3. Delete expired sessions on encounter
4. Retrieve associated user

### Cookie Configuration

```go
http.Cookie{
    Name:     "session",
    Value:    token,
    Path:     "/",
    HttpOnly: true,
    SameSite: http.SameSiteLaxMode,
    Secure:   IsHTTPSOnly(),
    MaxAge:   30 * 24 * 60 * 60,  // 30 days
}
```

`Secure` flag is controlled by `HTTPS_ONLY` env var.

## WebSocket Authentication

The browser's WebSocket API does not support custom headers on the upgrade
request. Authentication uses the session cookie, which is sent automatically.

```
Browser opens ws://host/api/ws
    │
    ├─ session cookie present?
    │      │
    │      ├─ yes ──► ValidateSession()
    │      │              │
    │      │              ├─ valid ──► state.userID = user.ID
    │      │              │            state.sessionID = session.ID
    │      │              │
    │      │              └─ expired/invalid ──► anonymous connection
    │      │
    │      └─ no ──► anonymous connection
    │
    ▼
Accept WebSocket upgrade
    │
    ▼
connectionState populated, audio streaming begins
```

- Authenticated connections: interactions logged with `user_id` and `session_id`
- Anonymous connections: interactions logged with `connection_id` only, reduced quotas apply

No Bearer token fallback for WebSocket.

## Usage Limits

| User Type | Concurrent WebSocket | Interactions/hour | Audio Blob Retention |
|-----------|---------------------|-------------------|---------------------|
| Anonymous | 1 | 30 | Not saved |
| Registered | 3 | Unlimited | 30 days |

Anonymous connections tracked by IP. Registered users tracked by user ID.

## Account Lockout

After 5 failed login attempts for an email within 15 minutes, the account is
temporarily locked for 15 minutes.

- Returns HTTP 429 with `retry_after_min` field
- Successful login clears all attempts for that email
- Expired records cleaned up by maintenance scheduler

## CSRF Protection

All POST/PUT/DELETE/PATCH requests to authenticated endpoints require a valid
`X-CSRF-Token` header.

1. Tokens derived from session tokens using HMAC-SHA256 (stateless)
2. Frontend fetches via `GET /api/auth/csrf` after login
3. Frontend includes in all state-changing requests
4. Exempt endpoints: `/api/auth/login`, `/api/auth/register`,
   `/api/auth/resend-verification`, `/api/auth/magic-link`

**Configuration:** `CSRF_SECRET` env var provides the HMAC key. If unset, a
random secret is generated on startup (tokens don't survive restarts).

## Password Security

- **Algorithm:** bcrypt, cost 12
- **Generic errors:** login failures never reveal whether email exists
- **Length limits:** 8-72 characters

## Security Headers

Update `SecurityHeadersMiddleware` to add:

```go
// Existing
w.Header().Set("Content-Security-Policy", ContentSecurityPolicy)
w.Header().Set("X-Frame-Options", "DENY")
w.Header().Set("X-Content-Type-Options", "nosniff")
w.Header().Set("X-XSS-Protection", "1; mode=block")

// New
w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
w.Header().Set("Permissions-Policy",
    "geolocation=(), camera=(), payment=(), usb=(), interest-cohort=()")
// Note: microphone is NOT disabled — peekaboo requires it
```

When `HTTPS_ONLY=true`:
```go
w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `HTTPS_ONLY` | Secure cookies + HSTS | `false` |
| `CSRF_SECRET` | HMAC key for CSRF tokens | random per startup |
| `EMAIL_FROM` | Sender address for auth emails | - |
| `EMAIL_SMTP_HOST` | SMTP server hostname | - |
| `EMAIL_SMTP_PORT` | SMTP server port | `587` |
| `EMAIL_SMTP_USER` | SMTP username | - |
| `EMAIL_SMTP_PASS` | SMTP password | - |

## Related Specs

- [interaction-logging.md](interaction-logging.md) - STT/LLM/TTS logging (uses session/user IDs from this spec)
- [architecture.md](architecture.md) - System overview
- [websocket-audio.md](websocket-audio.md) - WebSocket protocol
