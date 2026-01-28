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
	EventRateLimitExceeded          = "ratelimit.exceeded"
	EventMagicLinkRateLimitExceeded = "ratelimit.magiclink.exceeded"

	// File access events
	EventFileAccess       = "file.access"
	EventFileAccessDenied = "file.access.denied"

	// Admin operations - key rotation
	EventKeyRotationStarted      = "admin.rotation.started"
	EventKeyRotationCompleted    = "admin.rotation.completed"
	EventKeyReencryptionStarted  = "admin.rotation.reencrypt_started"
	EventKeyReencryptionProgress = "admin.rotation.reencrypt_progress"
	EventKeyReencryptionComplete = "admin.rotation.reencrypt_completed"
	EventKeyReencryptionFailed   = "admin.rotation.reencrypt_failed"

	// Admin operations - file deletion
	EventVideoDeletedByUser   = "admin.delete.video_user"
	EventVideoDeletedBySystem = "admin.delete.video_system"

	// Admin operations - database maintenance
	EventMaintenanceStarted   = "admin.maintenance.started"
	EventMaintenanceCompleted = "admin.maintenance.completed"
	EventMaintenanceFailed    = "admin.maintenance.failed"
	EventCleanupStarted       = "admin.cleanup.started"
	EventCleanupCompleted     = "admin.cleanup.completed"

	// Admin operations - feedback management
	EventAdminFeedbackUpdated = "admin.feedback.updated"
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
func LoginSuccess(ctx context.Context, ip, userID, email, userAgent string) {
	LogSecurityEvent(ctx, EventLoginSuccess, ip, "user_id", userID, "email", email, "user_agent", userAgent)
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
func SessionCreated(ctx context.Context, ip, userID, sessionID, userAgent string) {
	LogSecurityEvent(ctx, EventSessionCreated, ip, "user_id", userID, "session_id", sessionID, "user_agent", userAgent)
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

// AccessDeniedNotAdmin logs when a non-admin user tries to access an admin-only resource.
func AccessDeniedNotAdmin(ctx context.Context, ip, userID, resource string) {
	LogSecurityWarning(ctx, EventAccessDeniedAdmin, ip, "user_id", userID, "resource", resource)
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

// MagicLinkRateLimitExceeded logs when a user exceeds the per-email magic link rate limit.
func MagicLinkRateLimitExceeded(userID, email string) {
	logging.Warn("Security event", "event", EventMagicLinkRateLimitExceeded, "user_id", userID, "email", email)
}

// --- Admin operation events ---

// KeyRotationStarted logs when a new encryption key is being generated.
func KeyRotationStarted(oldVersion, newVersion int) {
	logging.Info("Security event", "event", EventKeyRotationStarted, "old_version", oldVersion, "new_version", newVersion)
}

// KeyRotationCompleted logs when key rotation is complete.
func KeyRotationCompleted(oldVersion, newVersion int) {
	logging.Info("Security event", "event", EventKeyRotationCompleted, "old_version", oldVersion, "new_version", newVersion)
}

// KeyReencryptionStarted logs when file re-encryption begins.
func KeyReencryptionStarted(totalFiles int) {
	logging.Info("Security event", "event", EventKeyReencryptionStarted, "total_files", totalFiles)
}

// KeyReencryptionProgress logs batch progress during re-encryption.
func KeyReencryptionProgress(processedFiles, totalFiles int, currentVersion int) {
	logging.Info("Security event", "event", EventKeyReencryptionProgress, "processed_files", processedFiles, "total_files", totalFiles, "current_version", currentVersion)
}

// KeyReencryptionCompleted logs successful completion of re-encryption.
func KeyReencryptionCompleted(totalFiles int, durationSecs float64) {
	logging.Info("Security event", "event", EventKeyReencryptionComplete, "total_files", totalFiles, "duration_seconds", durationSecs)
}

// KeyReencryptionFailed logs a re-encryption failure.
func KeyReencryptionFailed(videoID string, err error) {
	logging.Error("Security event", "event", EventKeyReencryptionFailed, "video_id", videoID, "error", err.Error())
}

// VideoDeletedByUser logs when a user deletes their own video.
func VideoDeletedByUser(ctx context.Context, ip, userID, videoID, filename string) {
	LogSecurityEvent(ctx, EventVideoDeletedByUser, ip, "user_id", userID, "video_id", videoID, "filename", filename)
}

// VideoDeletedBySystem logs when the system deletes an expired video.
func VideoDeletedBySystem(videoID, filename string, isAnonymous bool, retentionHours int) {
	logging.Info("Security event", "event", EventVideoDeletedBySystem, "video_id", videoID, "filename", filename, "is_anonymous", isAnonymous, "retention_hours", retentionHours)
}

// MaintenanceStarted logs when database maintenance begins.
func MaintenanceStarted() {
	logging.Info("Security event", "event", EventMaintenanceStarted)
}

// MaintenanceCompleted logs when database maintenance completes successfully.
func MaintenanceCompleted(durationSecs float64) {
	logging.Info("Security event", "event", EventMaintenanceCompleted, "duration_seconds", durationSecs)
}

// MaintenanceFailed logs when database maintenance fails.
func MaintenanceFailed(err error) {
	logging.Error("Security event", "event", EventMaintenanceFailed, "error", err.Error())
}

// CleanupStarted logs when scheduled cleanup begins.
func CleanupStarted() {
	logging.Info("Security event", "event", EventCleanupStarted)
}

// CleanupCompleted logs when scheduled cleanup completes.
func CleanupCompleted(videosDeleted int, sessionsDeleted, loginAttemptsDeleted int64, uploadSessionsDeleted, orphanChunksDeleted int) {
	logging.Info("Security event", "event", EventCleanupCompleted,
		"videos_deleted", videosDeleted,
		"sessions_deleted", sessionsDeleted,
		"login_attempts_deleted", loginAttemptsDeleted,
		"upload_sessions_deleted", uploadSessionsDeleted,
		"orphan_chunks_deleted", orphanChunksDeleted)
}

// AdminFeedbackUpdated logs when an admin updates feedback status.
func AdminFeedbackUpdated(ctx context.Context, ip, adminUserID, feedbackID, newStatus string) {
	LogSecurityEvent(ctx, EventAdminFeedbackUpdated, ip, "admin_user_id", adminUserID, "feedback_id", feedbackID, "new_status", newStatus)
}
