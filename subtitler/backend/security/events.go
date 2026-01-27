// Package security provides security event audit logging.
//
// All security-relevant events should be logged through this package to ensure
// consistent formatting and easy log parsing for security monitoring.
//
// Event types follow a consistent naming convention:
//   - auth.* for authentication events
//   - session.* for session lifecycle events
//   - access.* for access control events
//   - ratelimit.* for rate limit events
package security

import (
	"context"

	"github.com/trevor/subtitler/backend/logging"
)

// Event types for consistent event naming
const (
	// Authentication events
	EventLoginSuccess            = "auth.login.success"
	EventLoginFailedPassword     = "auth.login.failed.password"
	EventLoginFailedUserNotFound = "auth.login.failed.user_not_found"
	EventLoginFailedUnverified   = "auth.login.failed.unverified"
	EventLoginFailedLocked       = "auth.login.failed.locked"
	EventLoginFailedCaptcha      = "auth.login.failed.captcha"
	EventLogout                  = "auth.logout"
	EventRegistration            = "auth.registration"

	// 2FA events
	Event2FASetupInitiated        = "auth.2fa.setup_initiated"
	Event2FAEnabled               = "auth.2fa.enabled"
	Event2FADisabled              = "auth.2fa.disabled"
	Event2FAVerifySuccess         = "auth.2fa.verify.success"
	Event2FAVerifyFailed          = "auth.2fa.verify.failed"
	EventRecoveryCodeUsed         = "auth.2fa.recovery_used"
	EventRecoveryCodeFailed       = "auth.2fa.recovery_failed"
	EventRecoveryCodesRegenerated = "auth.2fa.codes_regenerated"

	// Password events
	EventPasswordResetRequested = "auth.password.reset_requested"
	EventPasswordResetSuccess   = "auth.password.reset_success"
	EventPasswordResetFailed    = "auth.password.reset_failed"

	// Magic link events
	EventMagicLinkRequested = "auth.magiclink.requested"
	EventMagicLinkSuccess   = "auth.magiclink.success"
	EventMagicLinkFailed    = "auth.magiclink.failed"

	// Account lockout events
	EventAccountLocked   = "auth.account.locked"
	EventAccountUnlocked = "auth.account.unlocked"

	// Session events
	EventSessionCreated = "session.created"
	EventSessionRevoked = "session.revoked"
	EventSessionExpired = "session.expired"
	EventSessionInvalid = "session.invalid"

	// Access control events
	EventAccessDenied      = "access.denied"
	EventAccessDeniedOwner = "access.denied.not_owner"
	EventAccessDeniedAuth  = "access.denied.not_authenticated"
	EventAccessDeniedAdmin = "access.denied.not_admin"

	// Rate limit events
	EventRateLimitExceeded = "ratelimit.exceeded"

	// File access events
	EventFileAccess       = "file.access"
	EventFileAccessDenied = "file.access.denied"
)

// LogSecurityEvent logs a security event with standard fields.
// All security events include: event type, IP address, and optional user context.
func LogSecurityEvent(ctx context.Context, event string, ip string, extra ...any) {
	args := []any{"event", event, "ip", ip}
	args = append(args, extra...)
	logging.InfoContext(ctx, "Security event", args...)
}

// LogSecurityWarning logs a security warning (e.g., failed attempts).
func LogSecurityWarning(ctx context.Context, event string, ip string, extra ...any) {
	args := []any{"event", event, "ip", ip}
	args = append(args, extra...)
	logging.WarnContext(ctx, "Security warning", args...)
}

// --- Convenience functions for common events ---

// LoginSuccess logs a successful login event.
func LoginSuccess(ctx context.Context, ip, userID, email string) {
	LogSecurityEvent(ctx, EventLoginSuccess, ip, "user_id", userID, "email", email)
}

// LoginFailedPassword logs a failed login due to wrong password.
func LoginFailedPassword(ctx context.Context, ip, email string, attemptCount int) {
	LogSecurityWarning(ctx, EventLoginFailedPassword, ip, "email", email, "attempt_count", attemptCount)
}

// LoginFailedUserNotFound logs a login attempt for non-existent user.
func LoginFailedUserNotFound(ctx context.Context, ip, email string) {
	LogSecurityWarning(ctx, EventLoginFailedUserNotFound, ip, "email", email)
}

// LoginFailedUnverified logs a login attempt for unverified email.
func LoginFailedUnverified(ctx context.Context, ip, email string) {
	LogSecurityWarning(ctx, EventLoginFailedUnverified, ip, "email", email)
}

// LoginFailedLocked logs a login attempt on locked account.
func LoginFailedLocked(ctx context.Context, ip, email string, remainingMins int) {
	LogSecurityWarning(ctx, EventLoginFailedLocked, ip, "email", email, "locked_minutes_remaining", remainingMins)
}

// LoginFailedCaptcha logs a failed CAPTCHA verification.
func LoginFailedCaptcha(ctx context.Context, ip string, reason string) {
	LogSecurityWarning(ctx, EventLoginFailedCaptcha, ip, "reason", reason)
}

// Logout logs a user logout event.
func Logout(ctx context.Context, ip, userID string) {
	LogSecurityEvent(ctx, EventLogout, ip, "user_id", userID)
}

// Registration logs a new user registration.
func Registration(ctx context.Context, ip, userID, email string) {
	LogSecurityEvent(ctx, EventRegistration, ip, "user_id", userID, "email", email)
}

// TwoFASetupInitiated logs when a user begins 2FA setup.
func TwoFASetupInitiated(ctx context.Context, ip, userID, email string) {
	LogSecurityEvent(ctx, Event2FASetupInitiated, ip, "user_id", userID, "email", email)
}

// TwoFAEnabled logs when a user successfully enables 2FA.
func TwoFAEnabled(ctx context.Context, ip, userID, email string) {
	LogSecurityEvent(ctx, Event2FAEnabled, ip, "user_id", userID, "email", email)
}

// TwoFADisabled logs when a user disables 2FA.
func TwoFADisabled(ctx context.Context, ip, userID, email string, viaRecovery bool) {
	LogSecurityEvent(ctx, Event2FADisabled, ip, "user_id", userID, "email", email, "via_recovery", viaRecovery)
}

// TwoFAVerifySuccess logs a successful 2FA verification.
func TwoFAVerifySuccess(ctx context.Context, ip, userID, email string) {
	LogSecurityEvent(ctx, Event2FAVerifySuccess, ip, "user_id", userID, "email", email)
}

// TwoFAVerifyFailed logs a failed 2FA verification attempt.
func TwoFAVerifyFailed(ctx context.Context, ip, userID, email string) {
	LogSecurityWarning(ctx, Event2FAVerifyFailed, ip, "user_id", userID, "email", email)
}

// RecoveryCodeUsed logs successful use of a recovery code.
func RecoveryCodeUsed(ctx context.Context, ip, userID, email string) {
	LogSecurityEvent(ctx, EventRecoveryCodeUsed, ip, "user_id", userID, "email", email)
}

// RecoveryCodeFailed logs a failed recovery code attempt.
func RecoveryCodeFailed(ctx context.Context, ip, email string) {
	LogSecurityWarning(ctx, EventRecoveryCodeFailed, ip, "email", email)
}

// RecoveryCodesRegenerated logs when recovery codes are regenerated.
func RecoveryCodesRegenerated(ctx context.Context, ip, userID, email string) {
	LogSecurityEvent(ctx, EventRecoveryCodesRegenerated, ip, "user_id", userID, "email", email)
}

// PasswordResetRequested logs a password reset request.
func PasswordResetRequested(ctx context.Context, ip, email string, userFound bool) {
	if userFound {
		LogSecurityEvent(ctx, EventPasswordResetRequested, ip, "email", email)
	} else {
		// Log at lower priority - don't reveal if email exists
		logging.DebugContext(ctx, "Security event", "event", EventPasswordResetRequested, "ip", ip, "email", email, "user_found", false)
	}
}

// PasswordResetSuccess logs a successful password reset.
func PasswordResetSuccess(ctx context.Context, ip, userID string) {
	LogSecurityEvent(ctx, EventPasswordResetSuccess, ip, "user_id", userID)
}

// PasswordResetFailed logs a failed password reset (invalid/expired token).
func PasswordResetFailed(ctx context.Context, ip string, reason string) {
	LogSecurityWarning(ctx, EventPasswordResetFailed, ip, "reason", reason)
}

// MagicLinkRequested logs a magic link request.
func MagicLinkRequested(ctx context.Context, ip, email string, userFound bool) {
	if userFound {
		LogSecurityEvent(ctx, EventMagicLinkRequested, ip, "email", email)
	} else {
		logging.DebugContext(ctx, "Security event", "event", EventMagicLinkRequested, "ip", ip, "email", email, "user_found", false)
	}
}

// MagicLinkSuccess logs a successful magic link login.
func MagicLinkSuccess(ctx context.Context, ip, userID, email string) {
	LogSecurityEvent(ctx, EventMagicLinkSuccess, ip, "user_id", userID, "email", email)
}

// MagicLinkFailed logs a failed magic link login.
func MagicLinkFailed(ctx context.Context, ip string, reason string) {
	LogSecurityWarning(ctx, EventMagicLinkFailed, ip, "reason", reason)
}

// AccountLocked logs when an account becomes locked.
func AccountLocked(ctx context.Context, ip, email string, failedAttempts int) {
	LogSecurityWarning(ctx, EventAccountLocked, ip, "email", email, "failed_attempts", failedAttempts)
}

// SessionCreated logs when a new session is created.
func SessionCreated(ctx context.Context, ip, userID, sessionID string) {
	LogSecurityEvent(ctx, EventSessionCreated, ip, "user_id", userID, "session_id", sessionID)
}

// SessionRevoked logs when a session is explicitly revoked.
func SessionRevoked(ctx context.Context, ip, userID, sessionID string, selfRevoke bool) {
	LogSecurityEvent(ctx, EventSessionRevoked, ip, "user_id", userID, "session_id", sessionID, "self_revoke", selfRevoke)
}

// SessionInvalid logs when an invalid session is presented.
func SessionInvalid(ctx context.Context, ip string, reason string) {
	LogSecurityWarning(ctx, EventSessionInvalid, ip, "reason", reason)
}

// AccessDenied logs an access denied event.
func AccessDenied(ctx context.Context, ip string, resource string, reason string, userID string) {
	extra := []any{"resource", resource, "reason", reason}
	if userID != "" {
		extra = append(extra, "user_id", userID)
	}
	LogSecurityWarning(ctx, EventAccessDenied, ip, extra...)
}

// AccessDeniedNotOwner logs when a user tries to access a resource they don't own.
func AccessDeniedNotOwner(ctx context.Context, ip, userID, resourceType, resourceID string) {
	LogSecurityWarning(ctx, EventAccessDeniedOwner, ip, "user_id", userID, "resource_type", resourceType, "resource_id", resourceID)
}

// AccessDeniedNotAuthenticated logs when an unauthenticated user tries to access a protected resource.
func AccessDeniedNotAuthenticated(ctx context.Context, ip, resource string) {
	LogSecurityWarning(ctx, EventAccessDeniedAuth, ip, "resource", resource)
}

// RateLimitExceeded logs when a rate limit is exceeded.
func RateLimitExceeded(ctx context.Context, ip, endpoint string, limitName string) {
	LogSecurityWarning(ctx, EventRateLimitExceeded, ip, "endpoint", endpoint, "limit_name", limitName)
}

// FileAccess logs file access for audit purposes.
func FileAccess(ctx context.Context, ip, userID, fileType, fileID string) {
	extra := []any{"file_type", fileType, "file_id", fileID}
	if userID != "" {
		extra = append(extra, "user_id", userID)
	}
	LogSecurityEvent(ctx, EventFileAccess, ip, extra...)
}

// FileAccessDenied logs when file access is denied (e.g., path traversal attempt).
func FileAccessDenied(ctx context.Context, ip, fileType, fileID, reason string) {
	LogSecurityWarning(ctx, EventFileAccessDenied, ip, "file_type", fileType, "file_id", fileID, "reason", reason)
}
