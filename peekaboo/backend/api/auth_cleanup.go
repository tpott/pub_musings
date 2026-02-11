package api

import (
	"log/slog"
	"time"

	"github.com/tpott/pub_musings/peekaboo/backend/db"
)

// loginAttemptRetention is how long login attempts are kept before cleanup.
const loginAttemptRetention = 7 * 24 * time.Hour // 7 days

// CleanupExpiredAuth deletes expired sessions and old login attempts.
// Returns the number of sessions and login attempts deleted.
func CleanupExpiredAuth(database *db.DB, logger *slog.Logger) (int64, int64) {
	sessionsDeleted, err := database.DeleteExpiredSessions()
	if err != nil {
		logger.Error("failed to clean up expired sessions", "error", err)
		sessionsDeleted = 0
	}

	cutoff := time.Now().UTC().Add(-loginAttemptRetention)
	attemptsDeleted, err := database.DeleteExpiredLoginAttempts(cutoff)
	if err != nil {
		logger.Error("failed to clean up old login attempts", "error", err)
		attemptsDeleted = 0
	}

	if sessionsDeleted > 0 || attemptsDeleted > 0 {
		logger.Info("auth cleanup completed",
			"sessions_deleted", sessionsDeleted,
			"attempts_deleted", attemptsDeleted)
	}

	return sessionsDeleted, attemptsDeleted
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
