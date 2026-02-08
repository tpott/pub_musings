package api

import (
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/tpott/pub_musings/peekaboo/backend/db"
)

const defaultRetentionDays = 30

// getRetentionDays reads INTERACTION_RETENTION_DAYS env var (default: 30).
func getRetentionDays() int {
	if s := os.Getenv("INTERACTION_RETENTION_DAYS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	return defaultRetentionDays
}

// CleanupExpiredAudioBlobs deletes audio blob files older than the retention
// period and sets their audio_blob_path to NULL in the database.
// Returns the number of files cleaned.
func CleanupExpiredAudioBlobs(database *db.DB, logger *slog.Logger) int {
	retentionDays := getRetentionDays()
	cutoff := time.Now().UTC().Add(-time.Duration(retentionDays) * 24 * time.Hour)

	entries, err := database.ListExpiredAudioBlobs(cutoff)
	if err != nil {
		logger.Error("failed to list expired audio blobs", "error", err)
		return 0
	}

	if len(entries) == 0 {
		return 0
	}

	cleaned := 0
	for _, e := range entries {
		// Delete the file from disk
		if err := os.Remove(e.AudioBlobPath); err != nil && !os.IsNotExist(err) {
			logger.Warn("failed to delete audio blob", "path", e.AudioBlobPath, "error", err)
			continue
		}

		// Clear the path in DB
		if err := database.ClearAudioBlobPath(e.ID); err != nil {
			logger.Warn("failed to clear audio blob path", "id", e.ID, "error", err)
			continue
		}

		cleaned++
	}

	logger.Info("audio blob cleanup completed", "cleaned", cleaned, "retention_days", retentionDays)
	return cleaned
}

// StartAudioBlobCleanupTicker starts a daily goroutine that cleans up expired
// audio blob files. Returns immediately; the cleanup runs in the background.
// The ticker stops when the context's done channel fires (via cancel).
func StartAudioBlobCleanupTicker(database *db.DB, logger *slog.Logger, stop <-chan struct{}) {
	ticker := time.NewTicker(24 * time.Hour)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				CleanupExpiredAudioBlobs(database, logger)
			case <-stop:
				return
			}
		}
	}()
}
