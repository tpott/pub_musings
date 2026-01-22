# Authentication System Specification

This document describes the authentication system implementation in the Subtitler application.

## Overview

The authentication system provides:
- Email/password registration and login
- Cookie-based sessions with Bearer token support
- Optional two-factor authentication via TOTP (see [totp.md](totp.md))

## Files

| File | Purpose |
|------|---------|
| `backend/auth/auth.go` | Main auth logic: hashing, sessions, validation |
| `backend/auth/auth_test.go` | Unit tests for auth module |
| `backend/db/db.go` | User and session database operations |
| `frontend/src/pages/login.astro` | Login page |
| `frontend/src/pages/register.astro` | Registration page |
| `frontend/src/utils/validation.ts` | Client-side validation |

## Database Schema

### Users Table

```sql
CREATE TABLE users (
    id TEXT PRIMARY KEY,                     -- 32-char hex (16 random bytes)
    email TEXT UNIQUE NOT NULL,              -- Unique, normalized to lowercase
    password_hash TEXT NOT NULL,             -- bcrypt hash (cost 12)
    totp_secret TEXT,                        -- Base32 TOTP secret (NULL if not set)
    totp_enabled INTEGER NOT NULL DEFAULT 0, -- 0=disabled, 1=enabled
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
)
CREATE INDEX idx_users_email ON users(email)
```

### Sessions Table

```sql
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,                     -- 32-char hex
    user_id TEXT NOT NULL REFERENCES users(id),
    token TEXT UNIQUE NOT NULL,              -- 64-char hex (32 random bytes)
    expires_at DATETIME NOT NULL,            -- Session expiration
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
)
CREATE INDEX idx_sessions_token ON sessions(token)
CREATE INDEX idx_sessions_user_id ON sessions(user_id)
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at)
```

## Configuration

| Constant | Value | Description |
|----------|-------|-------------|
| `SessionDuration` | 7 days | Session validity period |
| `BcryptCost` | 12 | bcrypt work factor |
| Min password | 8 chars | Minimum password length |
| Max password | 72 chars | bcrypt hard limit |
| Max email | 255 chars | Email length limit |

## API Endpoints

### POST /api/auth/register

Creates a new user account and session.

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
  "user": {
    "id": "abc123...",
    "email": "user@example.com",
    "created_at": "2026-01-22T...",
    "totp_enabled": false
  },
  "token": "sessiontoken..."
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
7. Insert user into database
8. Create session
9. Set httpOnly cookie

### POST /api/auth/login

Authenticates user and creates session.

**Request:**
```json
{
  "email": "user@example.com",
  "password": "securepassword",
  "totp_code": "123456"  // Optional, required if 2FA enabled
}
```

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

**Errors:**
- 400: Invalid request body
- 401: Invalid credentials or invalid TOTP code

**Processing:**
1. Normalize email
2. Look up user by email
3. Verify password with bcrypt
4. If 2FA enabled:
   - Return `totp_required: true` if no code provided
   - Validate TOTP code if provided
5. Create session (7-day expiry)
6. Set httpOnly cookie

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
    "created_at": "2026-01-22T..."
  }
}
```

## Session Management

### Token Extraction

The system checks for authentication in this order:
1. `Authorization: Bearer <token>` header (API clients)
2. `session` cookie (web browsers)

### Session Validation

`ValidateSession()` performs:
1. Retrieve session by token
2. Check expiration against current time
3. Delete expired sessions automatically
4. Retrieve associated user
5. Clean up orphaned sessions

### Cookie Configuration

```go
http.Cookie{
    Name:     "session",
    Value:    token,
    Path:     "/",
    HttpOnly: true,    // Prevents XSS access
    SameSite: http.SameSiteLaxMode,  // CSRF protection
    // Secure: true,   // Enable in production (HTTPS)
    MaxAge:   7 * 24 * 60 * 60,  // 7 days
}
```

## Password Security

- **Algorithm:** bcrypt with cost factor 12
- **Generic errors:** Login failures don't reveal if email exists
- **Length limits:** 8-72 characters (72 is bcrypt max)
- **Validation:** Both frontend and backend enforce limits

## Video Upload Limits

| User Type | Upload Limit | Retention |
|-----------|--------------|-----------|
| Anonymous | 2 uploads max | 48 hours |
| Registered | Unlimited | 90 days |

Anonymous uploads are tracked by session. The retention cleanup runs hourly.

## Security Considerations

### Implemented

- bcrypt password hashing (cost 12)
- HttpOnly cookies (XSS protection)
- SameSite=Lax (CSRF protection)
- Secure random token generation
- Generic login error messages
- Session expiration with cleanup
- 2FA support via TOTP
- 2FA recovery codes (see [recovery-codes.md](recovery-codes.md))
- Rate limiting on auth endpoints (5 req/min per IP, see `backend/ratelimit/`)

### Not Implemented

- Account lockout after failed attempts
- CAPTCHA integration
- Password complexity requirements beyond length
- IP-based blocking

## Frontend Validation

`frontend/src/utils/validation.ts` provides:

```typescript
validateEmail(email: string)    // Regex + length check
validatePassword(password: string)  // Length 8-72
validateTotpCode(code: string)  // Exactly 6 digits
```

## Related Specs

- [totp.md](totp.md) - Two-factor authentication details
- [recovery-codes.md](recovery-codes.md) - 2FA recovery codes
- [encryption.md](encryption.md) - File encryption at rest
