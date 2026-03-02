package api

import (
	"log/slog"
	"time"

	"github.com/tpott/pub_musings/peekaboo/backend/db"
)

// loginAttemptRetention is how long login attempts are kept before cleanup.
const loginAttemptRetention = 7 * 24 * time.Hour // 7 days

// AuthCleanupResult holds the counts of deleted records from auth cleanup.
type AuthCleanupResult struct {
	Sessions           int64
	LoginAttempts      int64
	VerificationTokens int64
	MagicLinkTokens    int64
}

// CleanupExpiredAuth deletes expired sessions, old login attempts,
// and expired verification/magic link tokens.
func CleanupExpiredAuth(database *db.DB, logger *slog.Logger) AuthCleanupResult {
	var result AuthCleanupResult

	var err error
	result.Sessions, err = database.DeleteExpiredSessions()
	if err != nil {
		logger.Error("failed to clean up expired sessions", "error", err)
		result.Sessions = 0
	}

	cutoff := time.Now().UTC().Add(-loginAttemptRetention)
	result.LoginAttempts, err = database.DeleteExpiredLoginAttempts(cutoff)
	if err != nil {
		logger.Error("failed to clean up old login attempts", "error", err)
		result.LoginAttempts = 0
	}

	result.VerificationTokens, err = database.DeleteExpiredEmailVerificationTokens()
	if err != nil {
		logger.Error("failed to clean up expired verification tokens", "error", err)
		result.VerificationTokens = 0
	}

	result.MagicLinkTokens, err = database.DeleteExpiredMagicLinkTokens()
	if err != nil {
		logger.Error("failed to clean up expired magic link tokens", "error", err)
		result.MagicLinkTokens = 0
	}

	if result.Sessions > 0 || result.LoginAttempts > 0 || result.VerificationTokens > 0 || result.MagicLinkTokens > 0 {
		logger.Info("auth cleanup completed",
			"sessions_deleted", result.Sessions,
			"attempts_deleted", result.LoginAttempts,
			"verification_tokens_deleted", result.VerificationTokens,
			"magic_link_tokens_deleted", result.MagicLinkTokens)
	}

	return result
}

// StartAuthCleanupTicker starts an hourly goroutine that cleans up expired
// sessions and old login attempts. Returns immediately; cleanup runs in the
// background. The ticker stops when the stop channel fires.
func StartAuthCleanupTicker(database *db.DB, logger *slog.Logger, stop <-chan struct{}) {
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				CleanupExpiredAuth(database, logger)
			case <-stop:
				return
			}
		}
	}()
}
