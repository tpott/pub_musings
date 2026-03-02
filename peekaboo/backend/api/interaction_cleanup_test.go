package api

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tpott/pub_musings/peekaboo/backend/db"
)

func TestCleanupExpiredAudioBlobs(t *testing.T) {
	database := setupTestDB(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Create an old interaction with an audio file
	oldTime := time.Now().UTC().Add(-40 * 24 * time.Hour)
	oldLog := &db.InteractionLog{
		ID:             "cleanup-old",
		ConnectionID:   "conn-1",
		STT:            &db.STTLog{Transcript: "old"},
		TotalLatencyMs: 100,
		CreatedAt:      oldTime,
	}
	if err := database.InsertInteraction(oldLog); err != nil {
		t.Fatalf("InsertInteraction: %v", err)
	}

	// Write a fake audio file and set the path
	audioDir := filepath.Join(tmpDir, "data", "interactions", "2025-12-01")
	if err := os.MkdirAll(audioDir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	audioPath := filepath.Join(audioDir, "cleanup-old.webm")
	if err := os.WriteFile(audioPath, []byte("old audio"), 0o640); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := database.UpdateInteractionAudioPath("cleanup-old", audioPath); err != nil {
		t.Fatalf("UpdateInteractionAudioPath: %v", err)
	}

	// Create a recent interaction with audio
	recentLog := &db.InteractionLog{
		ID:             "cleanup-recent",
		ConnectionID:   "conn-2",
		STT:            &db.STTLog{Transcript: "recent"},
		TotalLatencyMs: 100,
		CreatedAt:      time.Now().UTC(),
	}
	if err := database.InsertInteraction(recentLog); err != nil {
		t.Fatalf("InsertInteraction: %v", err)
	}
	recentDir := filepath.Join(tmpDir, "data", "interactions", time.Now().Format("2006-01-02"))
	if err := os.MkdirAll(recentDir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	recentPath := filepath.Join(recentDir, "cleanup-recent.webm")
	if err := os.WriteFile(recentPath, []byte("recent audio"), 0o640); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := database.UpdateInteractionAudioPath("cleanup-recent", recentPath); err != nil {
		t.Fatalf("UpdateInteractionAudioPath: %v", err)
	}

	// Run cleanup with default 30-day retention
	t.Setenv("INTERACTION_RETENTION_DAYS", "30")
	cleaned := CleanupExpiredAudioBlobs(database, logger)

	if cleaned != 1 {
		t.Errorf("cleaned: got %d, want 1", cleaned)
	}

	// Old file should be deleted
	if _, err := os.Stat(audioPath); !os.IsNotExist(err) {
		t.Error("old audio file should have been deleted")
	}

	// Recent file should still exist
	if _, err := os.Stat(recentPath); err != nil {
		t.Errorf("recent audio file should still exist: %v", err)
	}

	// Old interaction should have NULL audio_blob_path
	old, err := database.GetInteraction("cleanup-old")
	if err != nil {
		t.Fatalf("GetInteraction: %v", err)
	}
	if old.STT != nil && old.STT.AudioBlobPath != nil {
		t.Errorf("old interaction AudioBlobPath should be nil, got %q", *old.STT.AudioBlobPath)
	}

	// Recent interaction should still have audio_blob_path
	recent, err := database.GetInteraction("cleanup-recent")
	if err != nil {
		t.Fatalf("GetInteraction: %v", err)
	}
	if recent.STT == nil || recent.STT.AudioBlobPath == nil {
		t.Error("recent interaction AudioBlobPath should still be set")
	}
}

func TestCleanupExpiredAudioBlobs_ShortRetention(t *testing.T) {
	database := setupTestDB(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Create an interaction 2 days old
	oldTime := time.Now().UTC().Add(-2 * 24 * time.Hour)
	log := &db.InteractionLog{
		ID:             "short-ret",
		ConnectionID:   "conn-1",
		STT:            &db.STTLog{Transcript: "test"},
		TotalLatencyMs: 100,
		CreatedAt:      oldTime,
	}
	if err := database.InsertInteraction(log); err != nil {
		t.Fatalf("InsertInteraction: %v", err)
	}
	dir := filepath.Join(tmpDir, "data", "interactions", "test")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	path := filepath.Join(dir, "short-ret.webm")
	if err := os.WriteFile(path, []byte("audio"), 0o640); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := database.UpdateInteractionAudioPath("short-ret", path); err != nil {
		t.Fatalf("UpdateInteractionAudioPath: %v", err)
	}

	// With 1-day retention, 2-day-old entry should be cleaned
	t.Setenv("INTERACTION_RETENTION_DAYS", "1")
	cleaned := CleanupExpiredAudioBlobs(database, logger)

	if cleaned != 1 {
		t.Errorf("cleaned: got %d, want 1", cleaned)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("audio file should have been deleted with short retention")
	}
}

func TestGetRetentionDays_Default(t *testing.T) {
	t.Setenv("INTERACTION_RETENTION_DAYS", "")
	if got := getRetentionDays(); got != 30 {
		t.Errorf("default retention: got %d, want 30", got)
	}
}

func TestGetRetentionDays_Custom(t *testing.T) {
	t.Setenv("INTERACTION_RETENTION_DAYS", "7")
	if got := getRetentionDays(); got != 7 {
		t.Errorf("custom retention: got %d, want 7", got)
	}
}
