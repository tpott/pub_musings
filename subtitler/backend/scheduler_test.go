package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tpott/subtitler/backend/db"
)

// setupSchedulerTest creates a temp directory with a test database and sets
// the package-level globals (database, uploadDir, shutdownCtx) for scheduler tests.
// Returns a cleanup function that must be deferred.
func setupSchedulerTest(t *testing.T) func() {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "scheduler-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tempDir, "test.db")
	testDB, err := db.Open(dbPath)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to open database: %v", err)
	}

	// Save original globals
	origDatabase := database
	origUploadDir := uploadDir
	origCtx := shutdownCtx
	origCancel := shutdownCancel

	// Set test globals
	database = testDB
	uploadDir = tempDir
	shutdownCtx, shutdownCancel = context.WithCancel(context.Background())

	// Create uploads directory structure
	if err := os.MkdirAll(filepath.Join(tempDir, "chunks"), 0755); err != nil {
		testDB.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create chunks dir: %v", err)
	}

	return func() {
		testDB.Close()
		os.RemoveAll(tempDir)
		database = origDatabase
		uploadDir = origUploadDir
		shutdownCtx = origCtx
		shutdownCancel = origCancel
	}
}

func TestRunCleanupDeletesExpiredAnonymousVideos(t *testing.T) {
	cleanup := setupSchedulerTest(t)
	defer cleanup()

	// Create an anonymous video with created_at 3 days ago (>48h)
	expiredVideo := &db.Video{
		ID:          "expired-anon-1",
		Filename:    "expired.mp4",
		Size:        1024,
		ContentType: "video/mp4",
		FilePath:    filepath.Join(uploadDir, "expired.mp4"),
		KeyVersion:  1,
		CreatedAt:   time.Now().Add(-72 * time.Hour),
		UserID:      nil, // anonymous
	}
	if err := database.CreateVideo(expiredVideo); err != nil {
		t.Fatalf("Failed to create expired video: %v", err)
	}

	// Create a dummy file for the video
	if err := os.WriteFile(expiredVideo.FilePath, []byte("video data"), 0644); err != nil {
		t.Fatalf("Failed to create video file: %v", err)
	}

	runCleanup()

	// Verify video was deleted from database
	v, err := database.GetVideo(expiredVideo.ID)
	if err != nil {
		t.Fatalf("Unexpected error checking video: %v", err)
	}
	if v != nil {
		t.Error("Expected expired anonymous video to be deleted from database")
	}

	// Verify file was deleted from disk
	if _, err := os.Stat(expiredVideo.FilePath); !os.IsNotExist(err) {
		t.Error("Expected expired video file to be deleted from disk")
	}
}

func TestRunCleanupPreservesNonExpiredVideos(t *testing.T) {
	cleanup := setupSchedulerTest(t)
	defer cleanup()

	// Create an anonymous video created 1 hour ago (<48h)
	recentVideo := &db.Video{
		ID:          "recent-anon-1",
		Filename:    "recent.mp4",
		Size:        1024,
		ContentType: "video/mp4",
		FilePath:    filepath.Join(uploadDir, "recent.mp4"),
		KeyVersion:  1,
		CreatedAt:   time.Now().Add(-1 * time.Hour),
		UserID:      nil, // anonymous
	}
	if err := database.CreateVideo(recentVideo); err != nil {
		t.Fatalf("Failed to create recent video: %v", err)
	}

	runCleanup()

	// Verify video is still in database
	v, err := database.GetVideo(recentVideo.ID)
	if err != nil {
		t.Fatalf("Unexpected error checking video: %v", err)
	}
	if v == nil {
		t.Error("Expected recent video to be preserved in database")
	}
}

func TestRunCleanupDeletesExpiredSessions(t *testing.T) {
	cleanup := setupSchedulerTest(t)
	defer cleanup()

	// Create a user first (sessions require a user)
	user := &db.User{
		ID:           "user-1",
		Email:        "test@example.com",
		PasswordHash: "hash",
		Role:         "user",
		CreatedAt:    time.Now(),
	}
	if err := database.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create an expired session
	expiredSession := &db.Session{
		ID:        "session-expired",
		UserID:    "user-1",
		Token:     "token-expired",
		ExpiresAt: time.Now().Add(-1 * time.Hour),
		CreatedAt: time.Now().Add(-25 * time.Hour),
	}
	if err := database.CreateSession(expiredSession); err != nil {
		t.Fatalf("Failed to create expired session: %v", err)
	}

	// Create a valid session
	validSession := &db.Session{
		ID:        "session-valid",
		UserID:    "user-1",
		Token:     "token-valid",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
	}
	if err := database.CreateSession(validSession); err != nil {
		t.Fatalf("Failed to create valid session: %v", err)
	}

	runCleanup()

	// Verify expired session was deleted
	s, err := database.GetSessionByToken("token-expired")
	if err != nil {
		t.Fatalf("Unexpected error checking session: %v", err)
	}
	if s != nil {
		t.Error("Expected expired session to be deleted")
	}

	// Verify valid session is preserved
	s, err = database.GetSessionByToken("token-valid")
	if err != nil {
		t.Fatalf("Unexpected error checking session: %v", err)
	}
	if s == nil {
		t.Error("Expected valid session to be preserved")
	}
}

func TestRunCleanupDeletesExpiredTokens(t *testing.T) {
	cleanup := setupSchedulerTest(t)
	defer cleanup()

	// Create a user
	user := &db.User{
		ID:           "user-tokens",
		Email:        "tokens@example.com",
		PasswordHash: "hash",
		Role:         "user",
		CreatedAt:    time.Now(),
	}
	if err := database.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create expired password reset token
	if _, err := database.CreatePasswordResetToken("user-tokens", "hash-expired", time.Now().Add(-1*time.Hour)); err != nil {
		t.Fatalf("Failed to create expired token: %v", err)
	}

	// Create expired email verification token
	if _, err := database.CreateEmailVerificationToken("user-tokens", "hash-email-expired", time.Now().Add(-1*time.Hour)); err != nil {
		t.Fatalf("Failed to create expired email token: %v", err)
	}

	// Create expired magic link token
	if _, err := database.CreateMagicLinkToken("user-tokens", "hash-magic-expired", time.Now().Add(-1*time.Hour)); err != nil {
		t.Fatalf("Failed to create expired magic link: %v", err)
	}

	runCleanup()

	// Expired tokens should be gone - verify by counting
	// (individual Get methods aren't available, but we can count via another cleanup call)
	prtCount, _ := database.DeleteExpiredPasswordResetTokens()
	evtCount, _ := database.DeleteExpiredEmailVerificationTokens()
	mlCount, _ := database.DeleteExpiredMagicLinkTokens()

	if prtCount != 0 {
		t.Errorf("Expected 0 remaining expired password reset tokens, got %d", prtCount)
	}
	if evtCount != 0 {
		t.Errorf("Expected 0 remaining expired email verification tokens, got %d", evtCount)
	}
	if mlCount != 0 {
		t.Errorf("Expected 0 remaining expired magic link tokens, got %d", mlCount)
	}
}

func TestRunCleanupDeletesExpiredUploadSessions(t *testing.T) {
	cleanup := setupSchedulerTest(t)
	defer cleanup()

	// Create an expired upload session
	expiredUpload := &db.UploadSession{
		ID:          "upload-expired",
		Filename:    "big-file.mp4",
		ContentType: "video/mp4",
		TotalSize:   100000000,
		ChunkSize:   50000000,
		TotalChunks: 2,
		Status:      "in_progress",
		CreatedAt:   time.Now().Add(-25 * time.Hour),
		ExpiresAt:   time.Now().Add(-1 * time.Hour),
	}
	if err := database.CreateUploadSession(expiredUpload); err != nil {
		t.Fatalf("Failed to create expired upload session: %v", err)
	}

	// Create a chunk directory and file for it
	chunkDir := filepath.Join(uploadDir, "chunks", "upload-expired")
	if err := os.MkdirAll(chunkDir, 0755); err != nil {
		t.Fatalf("Failed to create chunk dir: %v", err)
	}
	chunkPath := filepath.Join(chunkDir, "chunk_0.part")
	if err := os.WriteFile(chunkPath, []byte("chunk data"), 0644); err != nil {
		t.Fatalf("Failed to create chunk file: %v", err)
	}

	// Record chunk in DB
	chunk := &db.UploadChunk{
		ID:              "chunk-0",
		UploadSessionID: "upload-expired",
		ChunkIndex:      0,
		ChunkPath:       chunkPath,
		Size:            10,
		CreatedAt:       time.Now().Add(-25 * time.Hour),
	}
	if _, err := database.CreateUploadChunk(chunk); err != nil {
		t.Fatalf("Failed to create chunk: %v", err)
	}

	runCleanup()

	// Verify upload session was deleted
	exists, err := database.UploadSessionExists("upload-expired")
	if err != nil {
		t.Fatalf("Error checking upload session: %v", err)
	}
	if exists {
		t.Error("Expected expired upload session to be deleted")
	}

	// Verify chunk directory was cleaned up
	if _, err := os.Stat(chunkDir); !os.IsNotExist(err) {
		t.Error("Expected chunk directory to be removed")
	}
}

func TestRunCleanupOrphanChunkDirectories(t *testing.T) {
	cleanup := setupSchedulerTest(t)
	defer cleanup()

	// Create an orphan chunk directory (no matching DB record)
	orphanDir := filepath.Join(uploadDir, "chunks", "orphan-session-xyz")
	if err := os.MkdirAll(orphanDir, 0755); err != nil {
		t.Fatalf("Failed to create orphan dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(orphanDir, "chunk_0.part"), []byte("orphan"), 0644); err != nil {
		t.Fatalf("Failed to create orphan chunk: %v", err)
	}

	runCleanup()

	// Verify orphan directory was removed
	if _, err := os.Stat(orphanDir); !os.IsNotExist(err) {
		t.Error("Expected orphan chunk directory to be removed")
	}
}

func TestRunCleanupPreservesActiveUploadSessions(t *testing.T) {
	cleanup := setupSchedulerTest(t)
	defer cleanup()

	// Create an active (non-expired) upload session
	activeUpload := &db.UploadSession{
		ID:          "upload-active",
		Filename:    "in-progress.mp4",
		ContentType: "video/mp4",
		TotalSize:   100000000,
		ChunkSize:   50000000,
		TotalChunks: 2,
		Status:      "in_progress",
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}
	if err := database.CreateUploadSession(activeUpload); err != nil {
		t.Fatalf("Failed to create active upload session: %v", err)
	}

	// Create chunk directory
	chunkDir := filepath.Join(uploadDir, "chunks", "upload-active")
	if err := os.MkdirAll(chunkDir, 0755); err != nil {
		t.Fatalf("Failed to create chunk dir: %v", err)
	}

	runCleanup()

	// Verify active session is preserved
	exists, err := database.UploadSessionExists("upload-active")
	if err != nil {
		t.Fatalf("Error checking upload session: %v", err)
	}
	if !exists {
		t.Error("Expected active upload session to be preserved")
	}

	// Verify chunk directory is preserved (not orphaned since session exists)
	if _, err := os.Stat(chunkDir); os.IsNotExist(err) {
		t.Error("Expected active chunk directory to be preserved")
	}
}

func TestRunCleanupDeletesVideoWithThumbnailAndBurnOutput(t *testing.T) {
	cleanup := setupSchedulerTest(t)
	defer cleanup()

	// Create file paths
	videoPath := filepath.Join(uploadDir, "expired-with-extras.mp4")
	thumbPath := filepath.Join(uploadDir, "expired-thumb.jpg")
	burnPath := filepath.Join(uploadDir, "expired-burned.mp4")

	// Create dummy files
	for _, p := range []string{videoPath, thumbPath, burnPath} {
		if err := os.WriteFile(p, []byte("data"), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", p, err)
		}
	}

	// Create expired anonymous video with thumbnail and burn output
	video := &db.Video{
		ID:            "expired-full",
		Filename:      "expired-with-extras.mp4",
		Size:          1024,
		ContentType:   "video/mp4",
		FilePath:      videoPath,
		ThumbnailPath: &thumbPath,
		KeyVersion:    1,
		CreatedAt:     time.Now().Add(-72 * time.Hour), // 3 days old
		UserID:        nil,
	}
	if err := database.CreateVideo(video); err != nil {
		t.Fatalf("Failed to create video: %v", err)
	}

	// Create a transcription so we can add a burn job
	transcription := &db.Transcription{
		ID:      "trans-1",
		VideoID: "expired-full",
		Status:  "complete",
	}
	if err := database.CreateTranscription(transcription); err != nil {
		t.Fatalf("Failed to create transcription: %v", err)
	}

	// Create a burn job, then complete it with output path
	burnJob := &db.BurnJob{
		ID:      "burn-1",
		VideoID: "expired-full",
		Status:  "pending",
	}
	if err := database.CreateBurnJob(burnJob); err != nil {
		t.Fatalf("Failed to create burn job: %v", err)
	}
	if err := database.CompleteBurnJobWithKeyVersion("expired-full", burnPath, 1); err != nil {
		t.Fatalf("Failed to complete burn job: %v", err)
	}

	runCleanup()

	// Verify all files deleted
	for _, p := range []string{videoPath, thumbPath, burnPath} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("Expected file %s to be deleted", p)
		}
	}
}
