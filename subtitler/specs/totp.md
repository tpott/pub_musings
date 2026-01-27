# TOTP Two-Factor Authentication Specification

This document describes the TOTP (Time-based One-Time Password) 2FA implementation.

## Overview

Users can optionally enable 2FA using authenticator apps like Google Authenticator, Authy, or 1Password. The implementation follows RFC 6238 (TOTP) and RFC 4226 (HOTP).

## Files

| File | Purpose |
|------|---------|
| `backend/totp/totp.go` | TOTP generation and validation |
| `backend/totp/totp_test.go` | Unit tests |
| `backend/main.go` (lines 596-795) | API endpoints |
| `backend/db/db.go` | Database operations |
| `frontend/src/pages/settings.astro` | 2FA management UI (Security tab) |
| `frontend/src/pages/login.astro` | Login with TOTP support |

## Library

**Pure Go implementation** - No external TOTP library. Uses only:
- `crypto/hmac` - HMAC-SHA1
- `crypto/sha1` - Hash algorithm
- `crypto/rand` - Secure random generation
- `encoding/base32` - Secret encoding

## Configuration

| Constant | Value | Description |
|----------|-------|-------------|
| `SecretLength` | 20 bytes | 160 bits (SHA1 minimum) |
| `Digits` | 6 | Code length |
| `Period` | 30 seconds | Time window |
| `Window` | ±1 | Clock drift tolerance |

## Database Schema

TOTP fields in the `users` table:

```sql
totp_secret TEXT           -- Base32-encoded secret (NULL if not set up)
totp_enabled INTEGER DEFAULT 0  -- 0=disabled, 1=enabled
```

## API Endpoints

### POST /api/auth/totp/setup

Initiates 2FA setup. Generates secret but does NOT enable 2FA.

**Requires:** Authentication

**Response (200):**
```json
{
  "secret": "JBSWY3DPEHPK3PXP...",
  "secret_display": "JBSW Y3DP EHPK 3PXP ...",
  "uri": "otpauth://totp/Subtitler:user@example.com?secret=JBSWY3DP...&issuer=Subtitler&algorithm=SHA1&digits=6&period=30",
  "issuer": "Subtitler"
}
```

**Processing:**
1. Generate 20 random bytes
2. Encode as base32 (no padding)
3. Save to `users.totp_secret` (NOT enabled yet)
4. Return secret and provisioning URI

### POST /api/auth/totp/verify

Verifies code and enables 2FA.

**Requires:** Authentication

**Request:**
```json
{
  "code": "123456"
}
```

**Response (200):**
```json
{
  "message": "2FA enabled successfully",
  "totp_enabled": true
}
```

**Errors:**
- 400: Missing code
- 401: Invalid code

**Processing:**
1. Validate code against stored secret
2. Check current time window ± 1 period
3. Set `totp_enabled = 1` in database

### POST /api/auth/totp/disable

Disables 2FA. Requires both password and current TOTP code for security.

**Requires:** Authentication

**Request:**
```json
{
  "password": "userpassword",
  "code": "123456"
}
```

**Response (200):**
```json
{
  "message": "2FA disabled successfully",
  "totp_enabled": false
}
```

**Errors:**
- 400: Missing password or code
- 401: Invalid password or code

**Processing:**
1. Verify password with bcrypt
2. Validate TOTP code
3. Clear `totp_secret` and set `totp_enabled = 0`

### POST /api/auth/login (with 2FA)

When 2FA is enabled, login requires an additional step.

**Step 1 - Without code:**
```json
{"email": "user@example.com", "password": "password"}
```

**Response (401):**
```json
{
  "error": "2FA code required",
  "totp_required": true
}
```

**Step 2 - With code:**
```json
{
  "email": "user@example.com",
  "password": "password",
  "totp_code": "123456"
}
```

**Response (200):**
```json
{
  "user": {...},
  "token": "sessiontoken..."
}
```

## TOTP Algorithm

### Code Generation

```go
func GenerateCodeAt(secret string, t time.Time) string {
    // 1. Decode base32 secret
    key := base32.StdEncoding.DecodeString(secret)

    // 2. Calculate counter (time / 30)
    counter := uint64(t.Unix() / Period)

    // 3. Generate HOTP (RFC 4226)
    mac := hmac.New(sha1.New, key)
    mac.Write(counterBytes)
    hash := mac.Sum(nil)

    // 4. Dynamic truncation
    offset := hash[len(hash)-1] & 0x0f
    code := (binary.BigEndian.Uint32(hash[offset:]) & 0x7fffffff) % 1000000

    return fmt.Sprintf("%06d", code)
}
```

### Code Validation

```go
func ValidateAt(secret, code string, t time.Time) bool {
    // Check current window and ±1 periods for clock drift
    for w := -Window; w <= Window; w++ {
        windowTime := t.Add(time.Duration(w) * Period * time.Second)
        if hmac.Equal([]byte(GenerateCodeAt(secret, windowTime)), []byte(code)) {
            return true
        }
    }
    return false
}
```

**Security:** Uses `hmac.Equal()` for constant-time comparison to prevent timing attacks.

## Provisioning URI Format

```
otpauth://totp/ISSUER:ACCOUNT?secret=SECRET&issuer=ISSUER&algorithm=SHA1&digits=6&period=30
```

Example:
```
otpauth://totp/Subtitler:user@example.com?secret=JBSWY3DPEHPK3PXP&issuer=Subtitler&algorithm=SHA1&digits=6&period=30
```

## QR Code Generation

**Status: IMPLEMENTED** (Task 47)

QR codes are generated server-side using `github.com/skip2/go-qrcode`. No external services required.

### Implementation Details

```go
import "github.com/skip2/go-qrcode"

// GenerateQRCode generates a QR code PNG as a base64 data URL
func GenerateQRCode(uri string) (string, error) {
    png, err := qrcode.Encode(uri, qrcode.Medium, 200)
    if err != nil {
        return "", err
    }
    b64 := base64.StdEncoding.EncodeToString(png)
    return "data:image/png;base64," + b64, nil
}
```

### API Response

`POST /api/auth/totp/setup` now returns `qr_code` field:

```json
{
  "secret": "JBSWY3DPEHPK3PXP...",
  "secret_display": "JBSW Y3DP EHPK 3PXP ...",
  "uri": "otpauth://totp/...",
  "issuer": "Subtitler",
  "qr_code": "data:image/png;base64,iVBOR..."
}
```

### Frontend Usage

```html
<img src={data.qr_code} alt="Scan QR code with authenticator app" />
```

### Benefits

- No external API calls (qrserver.com removed)
- No CSP exceptions needed
- Faster response times
- Works offline
- No privacy concerns about URI leakage to third parties

## Frontend Implementation

### Settings Page (`settings.astro`)

Features:
- Setup section: Shows QR code and formatted secret
- Verify section: 6-digit code input
- Disable section: Requires password + current code
- `autocomplete="one-time-code"` for better UX

### Login Page (`login.astro`)

Two-step flow:
1. Email/password form
2. If `totp_required: true`, show TOTP input
3. Submit credentials + code together

Uses `inputmode="numeric"` for mobile keyboard.

## Recovery Mechanisms

**Status: IMPLEMENTED** (Task 37)

See [recovery-codes.md](recovery-codes.md) for full specification.

**Features:**
- 10 single-use recovery codes generated when 2FA is enabled
- Codes stored as bcrypt hashes in `recovery_codes` table
- Format: XXXX-XXXX (8 characters, uppercase + digits)
- Recovery endpoint: `POST /api/auth/totp/recover`
- Requires email + password + recovery code
- Using a code disables 2FA and clears remaining codes

**Still missing:**
- Account lockout recovery without recovery code
- Alternative authentication methods

## Testing

Tests in `backend/totp/totp_test.go`:

- Secret generation uniqueness
- Code format validation (6 digits)
- Code validation within time window
- Code rejection outside window
- Provisioning URI format
- Secret display formatting
- Time-based code changes
- Invalid secret handling

Tests in `backend/totp/recovery_test.go`:

- Recovery code generation (10 unique codes)
- Code format validation (XXXX-XXXX)
- Alphabet validation (no ambiguous characters)
- Normalization (case-insensitive, hyphen removal)
- Hash and verify codes
- Wrong code rejection

Tests in `backend/db/db_test.go`:

- Recovery codes lifecycle (save, get, use, count)
- Regeneration replaces old unused codes
- Delete all codes

API tests in `backend/api_test.go`:

- TOTP verify returns 10 recovery codes
- Recover with valid code succeeds
- Recover with invalid code fails
- Recover with wrong password fails
- Recovery codes are single-use

Run tests:
```bash
cd backend && go test ./totp -v
cd backend && go test -run "TOTP|Recovery" -v
```

## Security Characteristics

### Strengths

- RFC 6238/4226 compliant implementation
- HMAC-SHA1 for code generation
- Constant-time comparison prevents timing attacks
- Time window allows ±30s clock drift
- Disable requires password + current code
- Secrets stored only in database

### Considerations

- Rate limiting (5 req/min per IP) on all TOTP endpoints (setup, verify, disable, recover)
- No TOTP resynchronization mechanism

## Related Specs

- [auth.md](auth.md) - Authentication system (login, sessions)
- [encryption.md](encryption.md) - File encryption (separate concern)
