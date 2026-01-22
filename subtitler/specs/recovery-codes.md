# 2FA Recovery Codes Specification

This document describes the implementation of recovery codes for 2FA (Task 37).

## Overview

When a user enables 2FA, they receive 10 single-use recovery codes. If a user loses access to their authenticator app, they can use one of these codes to disable 2FA and regain access to their account.

## Files

| File | Purpose |
|------|---------|
| `backend/db/db.go` | Recovery codes table, CRUD operations |
| `backend/totp/recovery.go` | Recovery code generation and validation |
| `backend/totp/recovery_test.go` | Unit tests |
| `backend/main.go` | API endpoint updates |
| `frontend/src/pages/security.astro` | Show recovery codes after 2FA setup |

## Database Schema

New `recovery_codes` table:

```sql
CREATE TABLE IF NOT EXISTS recovery_codes (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    code_hash TEXT NOT NULL,
    used INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    used_at DATETIME
);

CREATE INDEX IF NOT EXISTS idx_recovery_codes_user_id ON recovery_codes(user_id);
```

## Recovery Code Format

- **Length:** 8 characters
- **Alphabet:** A-Z0-9 (uppercase letters and digits, no ambiguous chars: 0/O, 1/I/L)
- **Display format:** XXXX-XXXX (grouped with hyphen for readability)
- **Storage:** bcrypt hash (cost 10, lower than passwords since codes are random)
- **Example:** `A3X7-K9M2`

## API Changes

### POST /api/auth/totp/verify (Updated)

When 2FA is successfully enabled, returns recovery codes in response:

**Response (200):**
```json
{
  "message": "2FA enabled successfully",
  "totp_enabled": true,
  "recovery_codes": [
    "A3X7-K9M2",
    "B5F2-H8N4",
    ...
  ]
}
```

**Processing:**
1. Validate TOTP code
2. Enable TOTP in database
3. Generate 10 recovery codes
4. Hash and store codes in database
5. Return plaintext codes (only time they're shown)

### POST /api/auth/totp/recover (New)

Uses a recovery code to disable 2FA.

**Request:**
```json
{
  "email": "user@example.com",
  "password": "userpassword",
  "recovery_code": "A3X7-K9M2"
}
```

**Response (200):**
```json
{
  "message": "2FA disabled. Please set up 2FA again to re-enable."
}
```

**Errors:**
- 400: Missing fields
- 401: Invalid email/password
- 401: Invalid or already-used recovery code
- 403: 2FA not enabled for this account

**Processing:**
1. Verify email/password
2. Check that 2FA is enabled
3. Validate recovery code against stored hashes
4. Mark code as used
5. Disable TOTP (same as /api/auth/totp/disable)
6. Clear all sessions except current

## Recovery Code Generation (`totp/recovery.go`)

```go
const (
    CodeLength = 8
    NumCodes   = 10
    Alphabet   = "ABCDEFGHJKMNPQRSTUVWXYZ23456789" // No 0,O,1,I,L
)

// GenerateRecoveryCodes generates n random recovery codes
func GenerateRecoveryCodes(n int) ([]string, error)

// FormatCode formats a code with hyphen (XXXX-XXXX)
func FormatCode(code string) string

// NormalizeCode removes hyphens and converts to uppercase
func NormalizeCode(input string) string

// HashCode hashes a recovery code for storage
func HashCode(code string) (string, error)

// CheckCode verifies a code against its hash
func CheckCode(code, hash string) bool
```

## Database Operations (`db/db.go`)

```go
// RecoveryCode represents a hashed recovery code
type RecoveryCode struct {
    ID        string
    UserID    string
    CodeHash  string
    Used      bool
    CreatedAt time.Time
    UsedAt    *time.Time
}

// SaveRecoveryCodes stores hashed recovery codes for a user
// Deletes any existing unused codes first
func (db *DB) SaveRecoveryCodes(userID string, codeHashes []string) error

// GetUnusedRecoveryCodes returns all unused recovery codes for a user
func (db *DB) GetUnusedRecoveryCodes(userID string) ([]RecoveryCode, error)

// UseRecoveryCode marks a recovery code as used
// Returns true if successful, false if code not found or already used
func (db *DB) UseRecoveryCode(codeID string) (bool, error)

// DeleteRecoveryCodes deletes all recovery codes for a user
func (db *DB) DeleteRecoveryCodes(userID string) error
```

## Frontend Changes

### Security Page (`security.astro`)

After successful 2FA verification, display recovery codes:

1. Show codes in a clear, copyable format
2. Warning: "Save these codes. They won't be shown again."
3. Copy all button
4. Print button (optional)
5. Confirmation checkbox: "I have saved my recovery codes"
6. Only then show "Continue" button

### Login Page (`login.astro`)

Add "Lost access to authenticator?" link that:
1. Shows email/password fields
2. Shows recovery code input field
3. Calls /api/auth/totp/recover

## Security Considerations

1. **One-time display:** Recovery codes shown only once after setup
2. **Hashed storage:** Codes stored as bcrypt hashes
3. **Single use:** Each code can only be used once
4. **Rate limiting:** Recovery endpoint should be rate limited (Task 38)
5. **Password required:** Recovery still requires valid password
6. **Audit trail:** `used_at` timestamp for compliance

## Test Cases

1. Generate 10 unique codes
2. Codes only contain valid alphabet characters
3. Hashing and verification works correctly
4. Using a code marks it as used
5. Used code cannot be reused
6. Invalid code rejected
7. All codes deleted when new ones generated
8. Codes deleted when 2FA disabled
9. Recovery requires valid password
10. Recovery fails if 2FA not enabled

## Migration Notes

- Existing users with 2FA enabled won't have recovery codes
- They should regenerate via security settings
- Add "Regenerate Recovery Codes" button to security page

## Related Specs

- [totp.md](totp.md) - TOTP 2FA implementation
- [auth.md](auth.md) - Authentication system
