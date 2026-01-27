# Authentication System Specification

This document describes the authentication system implementation in the Subtitler application.

## Overview

The authentication system provides:
- Email/password registration with email verification
- Magic link (passwordless) authentication
- Cookie-based sessions with Bearer token support
- Optional two-factor authentication via TOTP (see [totp.md](totp.md))
- Optional CAPTCHA protection (hCaptcha) for registration and login

## Files

| File | Purpose |
|------|---------|
| `backend/auth/auth.go` | Main auth logic: hashing, sessions, validation |
| `backend/auth/auth_test.go` | Unit tests for auth module |
| `backend/captcha/captcha.go` | CAPTCHA verification (hCaptcha) |
| `backend/captcha/captcha_test.go` | Unit tests for CAPTCHA module |
| `backend/db/db.go` | User and session database operations |
| `backend/email/email.go` | Email service for verification emails |
| `frontend/src/pages/login.astro` | Login page |
| `frontend/src/pages/register.astro` | Registration page |
| `frontend/src/pages/verify-email.astro` | Email verification page |
| `frontend/src/pages/magic-link.astro` | Magic link login page |
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
    email_verified INTEGER NOT NULL DEFAULT 0, -- 0=unverified, 1=verified
    verified_at DATETIME,                    -- When email was verified
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
)
CREATE INDEX idx_users_email ON users(email)
```

### Email Verification Tokens Table

```sql
CREATE TABLE email_verification_tokens (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    token_hash TEXT NOT NULL,                -- SHA-256 hash of token
    expires_at DATETIME NOT NULL,            -- 24 hours from creation
    used INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
)
CREATE INDEX idx_email_verification_user_id ON email_verification_tokens(user_id)
CREATE INDEX idx_email_verification_expires ON email_verification_tokens(expires_at)
```

### Magic Link Tokens Table

```sql
CREATE TABLE magic_link_tokens (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    token_hash TEXT NOT NULL,                -- SHA-256 hash of token
    expires_at DATETIME NOT NULL,            -- 15 minutes from creation
    used INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
)
CREATE INDEX idx_magic_link_user_id ON magic_link_tokens(user_id)
CREATE INDEX idx_magic_link_expires ON magic_link_tokens(expires_at)
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

Creates a new user account and sends verification email. User must verify email before logging in.

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
    "created_at": "2026-01-22T...",
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
7. Insert user into database (email_verified=false)
8. Generate email verification token (32 random bytes)
9. Store token hash with 24-hour expiry
10. Send verification email with token link

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

**Response if email not verified (403):**
```json
{
  "error": "Please verify your email address before logging in",
  "email_verification": true,
  "email_not_verified": true,
  "can_resend_verification": true
}
```

**Errors:**
- 400: Invalid request body
- 401: Invalid credentials or invalid TOTP code
- 403: Email not verified

**Processing:**
1. Normalize email
2. Look up user by email
3. Verify password with bcrypt
4. Check email is verified (return 403 if not)
5. If 2FA enabled:
   - Return `totp_required: true` if no code provided
   - Validate TOTP code if provided
6. Create session (7-day expiry)
7. Set httpOnly cookie

### GET /api/auth/verify

Verifies a user's email address using the token from the verification email.

**Query Parameters:**
- `token` (required): The verification token from the email link

**Response (200):**
```json
{
  "message": "Email verified successfully. You can now log in."
}
```

**Errors:**
- 400: Missing token or invalid/expired token

**Processing:**
1. Hash incoming token with SHA-256
2. Look up token hash in database
3. Check token is not used and not expired
4. Mark token as used
5. Set user's email_verified=1 and verified_at timestamp

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

**Note:** Always returns success to prevent email enumeration.

**Processing:**
1. Normalize email
2. Look up user by email
3. If user doesn't exist or is already verified, return success (prevents enumeration)
4. Delete any existing unused verification tokens for user
5. Generate new verification token (32 random bytes)
6. Store token hash with 24-hour expiry
7. Send verification email

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

**Note:** Always returns success to prevent email enumeration.

**Processing:**
1. Normalize email
2. Look up user by email
3. If user doesn't exist or email not verified, return success (prevents enumeration)
4. Delete any existing unused magic link tokens for user
5. Generate new token (32 random bytes)
6. Store token hash with 15-minute expiry
7. Send magic link email

### GET /api/auth/magic-link/verify

Verifies a magic link token and creates a session. Bypasses 2FA if enabled (magic link proves email access).

**Query Parameters:**
- `token` (required): The magic link token from the email

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
- 400: Missing token or invalid/expired/used token

**Processing:**
1. Hash incoming token with SHA-256
2. Look up token hash in database
3. Check token is not expired and not used
4. Mark token as used atomically
5. Get user by user_id
6. Create session (7-day expiry)
7. Set httpOnly cookie
8. Return user info

**Security Notes:**
- Magic links expire after 15 minutes
- Each link can only be used once
- Magic link login bypasses 2FA because email access proves identity
- Rate limited to prevent abuse

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
    HttpOnly: true,                  // Prevents XSS access
    SameSite: http.SameSiteLaxMode,  // CSRF protection
    Secure:   IsHTTPSOnly(),         // Requires HTTPS when HTTPS_ONLY=true
    MaxAge:   7 * 24 * 60 * 60,      // 7 days
}
```

The `Secure` flag is controlled by the `HTTPS_ONLY` environment variable:
- When `HTTPS_ONLY=true` or `HTTPS_ONLY=1`, cookies have `Secure: true` (HTTPS required)
- When unset or false, cookies work over HTTP (development mode)

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
- SameSite=Lax (basic CSRF protection)
- CSRF tokens for state-changing requests (see below)
- Secure random token generation
- Generic login error messages
- Session expiration with cleanup
- Email verification required before login
- 2FA support via TOTP
- 2FA recovery codes (see [recovery-codes.md](recovery-codes.md))
- Rate limiting on auth endpoints (5 req/min per IP, see `backend/ratelimit/`)

### CSRF Protection

All POST/PUT/DELETE/PATCH requests to authenticated endpoints require a valid `X-CSRF-Token` header.

**How it works:**
1. CSRF tokens are derived from session tokens using HMAC-SHA256 (stateless)
2. Frontend fetches token via `GET /api/auth/csrf` after authentication
3. Frontend includes token in all state-changing requests
4. Exempt endpoints: `/api/auth/login`, `/api/auth/register`, `/api/auth/forgot-password`, `/api/auth/reset-password`

**Configuration:**
- `CSRF_SECRET` env var: Optional HMAC key for CSRF tokens. If not set, a random secret is generated on startup (tokens won't survive restarts).

**Files:**
- `backend/csrf/csrf.go` - Token generation and validation
- `frontend/src/utils/csrf.ts` - Frontend CSRF utilities

### Account Lockout

After 5 failed login attempts for an email within 15 minutes, the account is temporarily locked for 15 minutes.

**Implementation:**
- `login_attempts` table tracks all login attempts (failed and successful)
- `IsEmailLocked()` checks if email has 5+ failed attempts in the last 15 minutes
- On lock, returns HTTP 429 with `retry_after_min` field showing minutes until unlock
- Successful login clears all login attempts for that email
- Expired login attempt records are cleaned up by the maintenance scheduler

**Files:**
- `backend/main.go:1023-1025` - Constants: `maxLoginAttempts = 5`, `loginLockDuration = 15 * time.Minute`
- `backend/db/db.go` - `RecordLoginAttempt()`, `IsEmailLocked()`, `ClearLoginAttempts()`

### Security Headers

The backend sets security headers via `securityHeadersMiddleware`:

**Content-Security-Policy (CSP):**
```
default-src 'self';
script-src 'self' 'unsafe-inline';
style-src 'self' 'unsafe-inline';
img-src 'self' data: blob:;
media-src 'self' blob:;
connect-src 'self';
font-src 'self';
object-src 'none';
frame-ancestors 'none';
base-uri 'self';
form-action 'self'
```

**Other security headers:**
- `X-Content-Type-Options: nosniff` - Prevent MIME type sniffing
- `X-Frame-Options: DENY` - Prevent clickjacking (legacy)
- `Referrer-Policy: strict-origin-when-cross-origin` - Limit referrer leakage
- `X-XSS-Protection: 1; mode=block` - Browser XSS filter (legacy)

**Notes:**
- `unsafe-inline` is required for Astro's inline scripts/styles
- `data:` and `blob:` are required for QR codes and video playback
- Production deployments via Caddy can apply stricter CSP if needed

**Files:**
- `backend/main.go` - `securityHeadersMiddleware()`

### Future Enhancements

The following security features are planned but not yet implemented:

- **CAPTCHA integration** - Bot protection for registration and login forms
- **Password complexity requirements** - Beyond length, require mixed case/numbers/symbols
- **IP-based blocking** - Block IPs with suspicious activity patterns

## Frontend Validation

`frontend/src/utils/validation.ts` provides:

```typescript
validateEmail(email: string)    // Regex + length check
validatePassword(password: string)  // Length 8-72
validateTotpCode(code: string)  // Exactly 6 digits
```

## Related Specs

- [email.md](email.md) - Email service configuration
- [totp.md](totp.md) - Two-factor authentication details
- [recovery-codes.md](recovery-codes.md) - 2FA recovery codes
- [encryption.md](encryption.md) - File encryption at rest
