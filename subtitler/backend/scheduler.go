package main

import (
	"os"
	"path/filepath"
	"time"

	"github.com/tpott/subtitler/backend/db"
	"github.com/tpott/subtitler/backend/logging"
	"github.com/tpott/subtitler/backend/security"
)

func startCleanupScheduler() {
	// Run cleanup immediately on startup, then every hour
	runCleanup()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-shutdownCtx.Done():
			logging.Info("Cleanup scheduler shutting down")
			return
		case <-ticker.C:
			runCleanup()
		}
	}
}

// runCleanup deletes expired videos and their associated files
func runCleanup() {
	security.CleanupStarted()
	logging.Info("Running cleanup for expired videos")

	// Get total count of expired videos for logging (non-fatal if this fails)
	totalExpired, err := database.CountExpiredVideos()
	if err != nil {
		logging.Error("Error counting expired videos", "error", err)
	} else if totalExpired == 0 {
		logging.Debug("No expired videos to clean up")
	} else {
		logging.Info("Found expired videos to clean up", "count", totalExpired)
	}

	// Process expired videos in batches to prevent OOM
	deletedCount := 0
	batchSize := db.DefaultCleanupBatchSize
	for {
		// Always fetch from offset 0 since we delete as we go
		expiredVideos, err := database.GetExpiredVideosPaginated(batchSize, 0)
		if err != nil {
			logging.Error("Error getting expired videos batch", "error", err)
			break
		}

		if len(expiredVideos) == 0 {
			break // No more expired videos
		}

		logging.Debug("Processing expired videos batch", "batch_size", len(expiredVideos))

		for _, video := range expiredVideos {
			// Delete from database and get file paths
			deletedFiles, err := database.DeleteVideo(video.ID)
			if err != nil {
				logging.Error("Error deleting video from database", "video_id", video.ID, "error", err)
				continue
			}

			// Delete the video file from disk
			if deletedFiles != nil && deletedFiles.FilePath != "" {
				if err := os.Remove(deletedFiles.FilePath); err != nil {
					if !os.IsNotExist(err) {
						logging.Error("Error deleting video file", "path", deletedFiles.FilePath, "error", err)
					}
				} else {
					logging.Debug("Deleted video file", "path", deletedFiles.FilePath)
				}
			}

			// Delete the thumbnail file from disk
			if deletedFiles != nil && deletedFiles.ThumbnailPath != nil && *deletedFiles.ThumbnailPath != "" {
				if err := os.Remove(*deletedFiles.ThumbnailPath); err != nil {
					if !os.IsNotExist(err) {
						logging.Error("Error deleting thumbnail file", "path", *deletedFiles.ThumbnailPath, "error", err)
					}
				} else {
					logging.Debug("Deleted thumbnail file", "path", *deletedFiles.ThumbnailPath)
				}
			}

			// Delete the burn output file from disk
			if deletedFiles != nil && deletedFiles.BurnOutputPath != nil && *deletedFiles.BurnOutputPath != "" {
				if err := os.Remove(*deletedFiles.BurnOutputPath); err != nil {
					if !os.IsNotExist(err) {
						logging.Error("Error deleting burn output file", "path", *deletedFiles.BurnOutputPath, "error", err)
					}
				} else {
					logging.Debug("Deleted burn output file", "path", *deletedFiles.BurnOutputPath)
				}
			}

			// Log security event for system video deletion
			isAnonymous := video.UserID == nil
			retentionHours := 2160 // 90 days for registered users
			if isAnonymous {
				retentionHours = 48
			}
			security.VideoDeletedBySystem(video.ID, video.Filename, isAnonymous, retentionHours)

			deletedCount++
			logging.Info("Cleaned up expired video",
				"video_id", video.ID,
				"has_user", video.UserID != nil,
				"created_at", video.CreatedAt.Format(time.RFC3339))
		}
	} // End batch processing loop

	// Also clean up expired sessions
	sessionCount, err := database.DeleteExpiredSessions()
	if err != nil {
		logging.Error("Error deleting expired sessions", "error", err)
	} else if sessionCount > 0 {
		logging.Info("Deleted expired sessions", "count", sessionCount)
		// Update session metrics after cleanup
		updateSessionMetrics()
	}

	// Clean up old login attempts (older than 1 hour to be safe)
	loginAttemptCount, err := database.DeleteExpiredLoginAttempts(time.Now().Add(-1 * time.Hour))
	if err != nil {
		logging.Error("Error deleting expired login attempts", "error", err)
	} else if loginAttemptCount > 0 {
		logging.Info("Deleted expired login attempts", "count", loginAttemptCount)
	}

	// Clean up expired upload sessions
	uploadSessionDeleteCount := 0
	expiredSessions, err := database.GetExpiredUploadSessions()
	if err != nil {
		logging.Error("Error getting expired upload sessions", "error", err)
	} else if len(expiredSessions) > 0 {
		for _, session := range expiredSessions {
			chunkPaths, err := database.DeleteUploadSession(session.ID)
			if err != nil {
				logging.Error("Error deleting upload session", "session_id", session.ID, "error", err)
				continue
			}
			// Delete chunk files
			for _, path := range chunkPaths {
				removeWithLogging(path, "expired upload session chunk")
			}
			// Try to remove the chunks directory
			chunksDir := filepath.Join(uploadDir, "chunks", session.ID)
			removeWithLogging(chunksDir, "expired upload session chunks directory")
			uploadSessionDeleteCount++
		}
		if uploadSessionDeleteCount > 0 {
			logging.Info("Deleted expired upload sessions", "count", uploadSessionDeleteCount)
		}
	}

	// Clean up expired auth tokens (password reset, email verification, magic link)
	passwordResetCount, err := database.DeleteExpiredPasswordResetTokens()
	if err != nil {
		logging.Error("Error deleting expired password reset tokens", "error", err)
	} else if passwordResetCount > 0 {
		logging.Info("Deleted expired password reset tokens", "count", passwordResetCount)
	}

	emailVerificationCount, err := database.DeleteExpiredEmailVerificationTokens()
	if err != nil {
		logging.Error("Error deleting expired email verification tokens", "error", err)
	} else if emailVerificationCount > 0 {
		logging.Info("Deleted expired email verification tokens", "count", emailVerificationCount)
	}

	magicLinkCount, err := database.DeleteExpiredMagicLinkTokens()
	if err != nil {
		logging.Error("Error deleting expired magic link tokens", "error", err)
	} else if magicLinkCount > 0 {
		logging.Info("Deleted expired magic link tokens", "count", magicLinkCount)
	}

	// Clean up orphan chunk directories (exist on disk but not in database)
	orphanCleanupCount := cleanupOrphanChunkDirectories()
	if orphanCleanupCount > 0 {
		logging.Info("Deleted orphan chunk directories", "count", orphanCleanupCount)
	}

	// Log security event with cleanup summary
	security.CleanupCompleted(deletedCount, sessionCount, loginAttemptCount, uploadSessionDeleteCount, orphanCleanupCount)
	logging.Info("Cleanup complete", "videos_deleted", deletedCount)
}

// cleanupOrphanChunkDirectories removes chunk directories that exist on disk
// but don't have corresponding records in the upload_sessions table.
// This handles edge cases like server crashes during upload or manual database cleanup.
func cleanupOrphanChunkDirectories() int {
	return cleanupOrphanChunkDirectoriesWithDB(database, uploadDir)
}

// cleanupOrphanChunkDirectoriesWithDB is the testable implementation of orphan cleanup.
// It scans the chunks directory for directories that don't have matching database records.
func cleanupOrphanChunkDirectoriesWithDB(database *db.DB, baseUploadDir string) int {
	chunksBaseDir := filepath.Join(baseUploadDir, "chunks")

	// Check if chunks directory exists
	entries, err := os.ReadDir(chunksBaseDir)
	if err != nil {
		if !os.IsNotExist(err) {
			logging.Error("Error reading chunks directory", "path", chunksBaseDir, "error", err)
		}
		return 0
	}

	orphanCount := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		sessionID := entry.Name()
		exists, err := database.UploadSessionExists(sessionID)
		if err != nil {
			logging.Error("Error checking upload session existence", "session_id", sessionID, "error", err)
			continue
		}

		if !exists {
			// This is an orphan directory - remove it and its contents
			orphanDir := filepath.Join(chunksBaseDir, sessionID)
			if err := os.RemoveAll(orphanDir); err != nil {
				logging.Error("Error removing orphan chunk directory", "path", orphanDir, "error", err)
			} else {
				logging.Debug("Removed orphan chunk directory", "session_id", sessionID)
				orphanCount++
			}
		}
	}

	return orphanCount
}

// startMaintenanceScheduler runs periodic database maintenance (VACUUM and ANALYZE).
// Set DB_MAINTENANCE_INTERVAL environment variable to configure interval (default: 24h).
// Set to "0" or "disabled" to disable maintenance.
// The scheduler exits when shutdownCtx is cancelled.
func startMaintenanceScheduler() {
	if dbMaintenanceInterval <= 0 {
		logging.Info("Database maintenance scheduler disabled")
		return
	}

	logging.Info("Database maintenance scheduler started", "interval", dbMaintenanceInterval)

	ticker := time.NewTicker(dbMaintenanceInterval)
	defer ticker.Stop()

	for {
		select {
		case <-shutdownCtx.Done():
			logging.Info("Maintenance scheduler shutting down")
			return
		case <-ticker.C:
			runDatabaseMaintenance()
		}
	}
}

// runDatabaseMaintenance performs VACUUM and ANALYZE on the SQLite database.
func runDatabaseMaintenance() {
	security.MaintenanceStarted()
	logging.Info("Running database maintenance (VACUUM + ANALYZE)")
	startTime := time.Now()

	if err := database.Maintenance(); err != nil {
		security.MaintenanceFailed(err)
		logging.Error("Database maintenance failed", "error", err)
		return
	}

	duration := time.Since(startTime)
	security.MaintenanceCompleted(duration.Seconds())
	logging.Info("Database maintenance complete", "duration", duration)
}
