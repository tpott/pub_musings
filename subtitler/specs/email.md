# Email Service Specification

This document describes the email service implementation for the Subtitler application.

## Overview

The email service provides transactional email capabilities for:
- Email verification (required before first login)
- Password reset emails
- (Future) Account notifications

## Files

| File | Purpose |
|------|---------|
| `backend/email/email.go` | Email service interface and Resend implementation |
| `backend/email/email_test.go` | Unit tests with mock implementation |
| `backend/email/mock.go` | Mock email service for testing |
| `backend/email/templates.go` | Email templates |
| `backend/main.go` | Email service initialization |

## Dependencies

Using the official Resend Go SDK:

```bash
go get github.com/resend/resend-go/v3
```

## Configuration

| Environment Variable | Required | Default | Description |
|---------------------|----------|---------|-------------|
| `RESEND_API_KEY` | Yes (prod) | - | Resend API key (starts with `re_`) |
| `EMAIL_FROM` | No | `noreply@subtitler.app` | Sender email address |
| `EMAIL_ENABLED` | No | `true` | Set to `false` to disable email sending |
| `APP_URL` | No | `http://localhost:4321` | Base URL for email links |

## Email Service Interface

```go
package email

import "context"

// EmailService defines the interface for sending emails
type EmailService interface {
    // SendPasswordReset sends a password reset email
    SendPasswordReset(ctx context.Context, to, token string) error

    // SendEmailVerification sends an email verification email
    SendEmailVerification(ctx context.Context, to, token string) error

    // SendEmail sends a generic email
    SendEmail(ctx context.Context, to, subject, htmlBody, textBody string) error

    // IsEnabled returns whether email sending is enabled
    IsEnabled() bool
}
```

## Resend Implementation

```go
package email

import (
    "context"
    "fmt"
    "os"

    "github.com/resend/resend-go/v3"
)

type ResendService struct {
    client  *resend.Client
    from    string
    appURL  string
    enabled bool
}

// NewResendService creates a new Resend email service
func NewResendService() *ResendService {
    apiKey := os.Getenv("RESEND_API_KEY")
    from := os.Getenv("EMAIL_FROM")
    if from == "" {
        from = "noreply@subtitler.app"
    }
    appURL := os.Getenv("APP_URL")
    if appURL == "" {
        appURL = "http://localhost:4321"
    }
    enabled := os.Getenv("EMAIL_ENABLED") != "false"

    var client *resend.Client
    if apiKey != "" {
        client = resend.NewClient(apiKey)
    }

    return &ResendService{
        client:  client,
        from:    from,
        appURL:  appURL,
        enabled: enabled && client != nil,
    }
}

func (s *ResendService) IsEnabled() bool {
    return s.enabled
}

func (s *ResendService) SendEmail(ctx context.Context, to, subject, htmlBody, textBody string) error {
    if !s.enabled {
        return nil // Silently succeed when disabled
    }

    params := &resend.SendEmailRequest{
        From:    s.from,
        To:      []string{to},
        Subject: subject,
        Html:    htmlBody,
        Text:    textBody,
    }

    _, err := s.client.Emails.Send(params)
    return err
}

func (s *ResendService) SendPasswordReset(ctx context.Context, to, token string) error {
    subject := "Reset your Subtitler password"
    resetURL := fmt.Sprintf("%s/reset-password?token=%s", s.appURL, token)

    htmlBody := PasswordResetHTML(resetURL)
    textBody := PasswordResetText(resetURL)

    return s.SendEmail(ctx, to, subject, htmlBody, textBody)
}
```

## Mock Implementation

For testing without sending real emails:

```go
package email

import (
    "context"
    "sync"
)

// SentEmail represents an email that was "sent" by the mock service
type SentEmail struct {
    To       string
    Subject  string
    HtmlBody string
    TextBody string
}

// MockService is a mock email service for testing
type MockService struct {
    mu     sync.Mutex
    Emails []SentEmail
}

func NewMockService() *MockService {
    return &MockService{
        Emails: make([]SentEmail, 0),
    }
}

func (m *MockService) IsEnabled() bool {
    return true
}

func (m *MockService) SendEmail(ctx context.Context, to, subject, htmlBody, textBody string) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.Emails = append(m.Emails, SentEmail{
        To:       to,
        Subject:  subject,
        HtmlBody: htmlBody,
        TextBody: textBody,
    })
    return nil
}

func (m *MockService) SendPasswordReset(ctx context.Context, to, token string) error {
    subject := "Reset your Subtitler password"
    return m.SendEmail(ctx, to, subject, "", "")
}

// GetEmails returns all sent emails and clears the list
func (m *MockService) GetEmails() []SentEmail {
    m.mu.Lock()
    defer m.mu.Unlock()
    emails := m.Emails
    m.Emails = make([]SentEmail, 0)
    return emails
}

// Clear clears all sent emails
func (m *MockService) Clear() {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.Emails = make([]SentEmail, 0)
}
```

## Email Templates

### Password Reset Email

```go
package email

import "fmt"

// PasswordResetHTML returns the HTML body for password reset emails
func PasswordResetHTML(resetURL string) string {
    return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Reset Your Password</title>
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; line-height: 1.6; color: #333; max-width: 600px; margin: 0 auto; padding: 20px;">
    <div style="text-align: center; margin-bottom: 30px;">
        <h1 style="color: #1a1a1a; margin: 0;">Subtitler</h1>
    </div>

    <div style="background: #f9f9f9; border-radius: 8px; padding: 30px;">
        <h2 style="margin-top: 0;">Reset Your Password</h2>
        <p>We received a request to reset your password. Click the button below to create a new password:</p>

        <div style="text-align: center; margin: 30px 0;">
            <a href="%s" style="display: inline-block; background: #4F46E5; color: white; text-decoration: none; padding: 12px 30px; border-radius: 6px; font-weight: 500;">Reset Password</a>
        </div>

        <p style="color: #666; font-size: 14px;">This link will expire in 1 hour. If you didn't request a password reset, you can safely ignore this email.</p>

        <hr style="border: none; border-top: 1px solid #eee; margin: 20px 0;">

        <p style="color: #999; font-size: 12px;">If the button doesn't work, copy and paste this link into your browser:</p>
        <p style="color: #4F46E5; font-size: 12px; word-break: break-all;">%s</p>
    </div>

    <div style="text-align: center; margin-top: 30px; color: #999; font-size: 12px;">
        <p>Subtitler - AI-powered video subtitles</p>
    </div>
</body>
</html>`, resetURL, resetURL)
}

// PasswordResetText returns the plain text body for password reset emails
func PasswordResetText(resetURL string) string {
    return fmt.Sprintf(`Reset Your Password

We received a request to reset your Subtitler password.

Click the link below to create a new password:
%s

This link will expire in 1 hour.

If you didn't request a password reset, you can safely ignore this email.

---
Subtitler - AI-powered video subtitles
`, resetURL)
}
```

## Database Schema

Password reset tokens are stored in a new table:

```sql
CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id),
    token_hash TEXT NOT NULL,
    expires_at DATETIME NOT NULL,
    used INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_password_reset_user_id ON password_reset_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_password_reset_expires ON password_reset_tokens(expires_at);
```

## API Endpoints

### POST /api/auth/forgot-password

Initiates password reset flow.

**Request:**
```json
{
  "email": "user@example.com"
}
```

**Response (200):**
```json
{
  "message": "If an account exists with that email, a password reset link has been sent."
}
```

**Notes:**
- Always returns success (prevents email enumeration)
- Rate limited: 3 requests per 15 minutes per IP
- Rate limited: 1 request per email per 5 minutes

**Processing:**
1. Validate email format
2. Look up user by email (don't reveal if exists)
3. Generate secure reset token (32 bytes)
4. Hash token and store in database with 1-hour expiry
5. Delete any existing reset tokens for this user
6. Send password reset email
7. Return generic success message

### POST /api/auth/reset-password

Completes password reset.

**Request:**
```json
{
  "token": "reset_token_here",
  "password": "new_secure_password"
}
```

**Response (200):**
```json
{
  "message": "Password has been reset successfully. Please log in with your new password."
}
```

**Errors:**
- 400: Missing fields, invalid password
- 400: Invalid or expired token

**Processing:**
1. Validate token format
2. Hash token and look up in database
3. Check expiration and used status
4. Validate new password (8-72 chars)
5. Hash new password
6. Update user's password
7. Mark token as used
8. Delete all user's sessions (security: log out everywhere)
9. Delete all reset tokens for user
10. Return success

## Security Considerations

### Token Security
- Tokens are 32 random bytes (256 bits entropy)
- Stored as SHA-256 hash (not bcrypt - tokens are random)
- 1-hour expiration
- Single use only
- All tokens invalidated after successful reset

### Rate Limiting
- Forgot password: 3 requests per 15 minutes per IP
- Reset password: 5 requests per minute per IP
- Prevents brute force on reset tokens

### Email Enumeration Prevention
- Forgot password always returns same message regardless of email existence
- Consistent response timing (no timing attacks)

### Session Invalidation
- All existing sessions cleared after password change
- Forces re-authentication with new password

## Test Cases

1. **SendEmail**
   - Sends email with correct parameters
   - Returns error when Resend fails
   - No-op when disabled

2. **SendPasswordReset**
   - Generates correct reset URL
   - Includes both HTML and text body
   - Uses correct from address

3. **MockService**
   - Records sent emails
   - Returns correct count
   - Clear works correctly

4. **API: Forgot Password**
   - Returns success for existing email
   - Returns success for non-existing email (same response)
   - Rate limited appropriately
   - Token stored in database

5. **API: Reset Password**
   - Resets password with valid token
   - Rejects expired token
   - Rejects used token
   - Rejects invalid token
   - Logs out all sessions
   - Rate limited

## Integration with main.go

```go
// In main.go initialization
var emailService email.EmailService

func init() {
    emailService = email.NewResendService()
    if !emailService.IsEnabled() {
        log.Println("WARNING: Email service is disabled (no RESEND_API_KEY)")
    }
}
```

## Development Mode

When `RESEND_API_KEY` is not set:
- `IsEnabled()` returns false
- `SendEmail()` succeeds silently (no-op)
- Console logs what would have been sent

For local development testing:
1. Set `EMAIL_ENABLED=false` to disable email entirely
2. Or use Resend's test API key for development
3. Or check mock service in tests

## Related Specs

- [auth.md](auth.md) - Authentication system
- [recovery-codes.md](recovery-codes.md) - 2FA recovery codes
