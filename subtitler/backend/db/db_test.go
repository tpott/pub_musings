package db

import (
	"os"
	"testing"
	"time"
)

func TestOpenAndMigrate(t *testing.T) {
	// Use temp file for test database
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()
}

func TestCreateAndGetVideo(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	video := &Video{
		ID:          "test-video-123",
		Filename:    "test.mp4",
		Size:        1024,
		ContentType: "video/mp4",
		FilePath:    "/uploads/test-video-123.mp4",
		CreatedAt:   time.Now(),
	}

	if err := db.CreateVideo(video); err != nil {
		t.Fatalf("Failed to create video: %v", err)
	}

	retrieved, err := db.GetVideo("test-video-123")
	if err != nil {
		t.Fatalf("Failed to get video: %v", err)
	}

	if retrieved == nil {
		t.Fatal("Retrieved video is nil")
	}

	if retrieved.ID != video.ID {
		t.Errorf("Expected ID %s, got %s", video.ID, retrieved.ID)
	}
	if retrieved.Filename != video.Filename {
		t.Errorf("Expected Filename %s, got %s", video.Filename, retrieved.Filename)
	}
	if retrieved.Size != video.Size {
		t.Errorf("Expected Size %d, got %d", video.Size, retrieved.Size)
	}
}

func TestGetVideoNotFound(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	retrieved, err := db.GetVideo("nonexistent")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if retrieved != nil {
		t.Error("Expected nil for nonexistent video")
	}
}

func TestTranscriptionLifecycle(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create a video first (foreign key)
	video := &Video{
		ID:          "video-123",
		Filename:    "test.mp4",
		Size:        1024,
		ContentType: "video/mp4",
		FilePath:    "/uploads/video-123.mp4",
		CreatedAt:   time.Now(),
	}
	if err := db.CreateVideo(video); err != nil {
		t.Fatalf("Failed to create video: %v", err)
	}

	// Create transcription
	transcription := &Transcription{
		ID:        "trans-123",
		VideoID:   "video-123",
		Status:    "pending",
		Message:   "Waiting to start",
		Progress:  0,
		CreatedAt: time.Now(),
	}
	if err := db.CreateTranscription(transcription); err != nil {
		t.Fatalf("Failed to create transcription: %v", err)
	}

	// Get transcription
	retrieved, err := db.GetTranscription("video-123")
	if err != nil {
		t.Fatalf("Failed to get transcription: %v", err)
	}
	if retrieved.Status != "pending" {
		t.Errorf("Expected status 'pending', got '%s'", retrieved.Status)
	}

	// Update status
	if err := db.UpdateTranscriptionStatus("video-123", "processing", "Extracting audio", 30); err != nil {
		t.Fatalf("Failed to update status: %v", err)
	}

	retrieved, _ = db.GetTranscription("video-123")
	if retrieved.Status != "processing" {
		t.Errorf("Expected status 'processing', got '%s'", retrieved.Status)
	}
	if retrieved.Progress != 30 {
		t.Errorf("Expected progress 30, got %d", retrieved.Progress)
	}

	// Complete transcription
	segments := []Segment{
		{ID: 0, Start: 0.0, End: 2.5, Text: "Hello world"},
		{ID: 1, Start: 3.0, End: 5.5, Text: "This is a test"},
	}
	if err := db.CompleteTranscription("video-123", "en", 10.0, "Hello world This is a test", segments); err != nil {
		t.Fatalf("Failed to complete transcription: %v", err)
	}

	retrieved, _ = db.GetTranscription("video-123")
	if retrieved.Status != "complete" {
		t.Errorf("Expected status 'complete', got '%s'", retrieved.Status)
	}
	if retrieved.Language != "en" {
		t.Errorf("Expected language 'en', got '%s'", retrieved.Language)
	}

	// Test GetSegments
	parsedSegments, err := retrieved.GetSegments()
	if err != nil {
		t.Fatalf("Failed to parse segments: %v", err)
	}
	if len(parsedSegments) != 2 {
		t.Errorf("Expected 2 segments, got %d", len(parsedSegments))
	}
	if parsedSegments[0].Text != "Hello world" {
		t.Errorf("Expected 'Hello world', got '%s'", parsedSegments[0].Text)
	}
}

func TestFailTranscription(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create a video first
	video := &Video{
		ID:          "video-fail",
		Filename:    "test.mp4",
		Size:        1024,
		ContentType: "video/mp4",
		FilePath:    "/uploads/video-fail.mp4",
		CreatedAt:   time.Now(),
	}
	db.CreateVideo(video)

	// Create transcription
	transcription := &Transcription{
		ID:        "trans-fail",
		VideoID:   "video-fail",
		Status:    "processing",
		Message:   "Processing",
		Progress:  50,
		CreatedAt: time.Now(),
	}
	db.CreateTranscription(transcription)

	// Fail it
	if err := db.FailTranscription("video-fail", "Something went wrong"); err != nil {
		t.Fatalf("Failed to fail transcription: %v", err)
	}

	retrieved, _ := db.GetTranscription("video-fail")
	if retrieved.Status != "error" {
		t.Errorf("Expected status 'error', got '%s'", retrieved.Status)
	}
	if retrieved.Message != "Something went wrong" {
		t.Errorf("Expected message 'Something went wrong', got '%s'", retrieved.Message)
	}
}

func TestListVideos(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create videos
	userID := "user-1"
	for i := 0; i < 3; i++ {
		video := &Video{
			ID:          "video-" + string(rune('a'+i)),
			Filename:    "test.mp4",
			Size:        1024,
			ContentType: "video/mp4",
			FilePath:    "/uploads/video.mp4",
			CreatedAt:   time.Now(),
			UserID:      &userID,
		}
		db.CreateVideo(video)
	}

	// List all
	videos, err := db.ListVideos(nil, nil)
	if err != nil {
		t.Fatalf("Failed to list videos: %v", err)
	}
	if len(videos) != 3 {
		t.Errorf("Expected 3 videos, got %d", len(videos))
	}

	// List by user
	videos, err = db.ListVideos(&userID, nil)
	if err != nil {
		t.Fatalf("Failed to list videos by user: %v", err)
	}
	if len(videos) != 3 {
		t.Errorf("Expected 3 videos for user, got %d", len(videos))
	}
}

func TestCountVideosBySession(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	sessionID := "session-abc123"

	// Initially should be 0
	count, err := db.CountVideosBySession(sessionID)
	if err != nil {
		t.Fatalf("Failed to count videos: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 videos, got %d", count)
	}

	// Create 2 videos with the session
	for i := 0; i < 2; i++ {
		video := &Video{
			ID:          "video-session-" + string(rune('a'+i)),
			Filename:    "test.mp4",
			Size:        1024,
			ContentType: "video/mp4",
			FilePath:    "/uploads/video.mp4",
			CreatedAt:   time.Now(),
			SessionID:   &sessionID,
		}
		if err := db.CreateVideo(video); err != nil {
			t.Fatalf("Failed to create video: %v", err)
		}
	}

	// Now should be 2
	count, err = db.CountVideosBySession(sessionID)
	if err != nil {
		t.Fatalf("Failed to count videos: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 videos, got %d", count)
	}

	// Different session should have 0
	count, err = db.CountVideosBySession("different-session")
	if err != nil {
		t.Fatalf("Failed to count videos: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 videos for different session, got %d", count)
	}
}

func TestGetExpiredVideos(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	userID := "user-123"

	// Create an anonymous video older than 48 hours (should be expired)
	expiredAnon := &Video{
		ID:          "anon-old",
		Filename:    "old-anon.mp4",
		Size:        1024,
		ContentType: "video/mp4",
		FilePath:    "/uploads/anon-old.mp4",
		CreatedAt:   time.Now().Add(-49 * time.Hour),
	}
	if err := db.CreateVideo(expiredAnon); err != nil {
		t.Fatalf("Failed to create expired anonymous video: %v", err)
	}

	// Create an anonymous video less than 48 hours old (should NOT be expired)
	freshAnon := &Video{
		ID:          "anon-fresh",
		Filename:    "fresh-anon.mp4",
		Size:        1024,
		ContentType: "video/mp4",
		FilePath:    "/uploads/anon-fresh.mp4",
		CreatedAt:   time.Now().Add(-24 * time.Hour),
	}
	if err := db.CreateVideo(freshAnon); err != nil {
		t.Fatalf("Failed to create fresh anonymous video: %v", err)
	}

	// Create a registered user video older than 90 days (should be expired)
	expiredUser := &Video{
		ID:          "user-old",
		Filename:    "old-user.mp4",
		Size:        1024,
		ContentType: "video/mp4",
		FilePath:    "/uploads/user-old.mp4",
		CreatedAt:   time.Now().Add(-91 * 24 * time.Hour),
		UserID:      &userID,
	}
	if err := db.CreateVideo(expiredUser); err != nil {
		t.Fatalf("Failed to create expired user video: %v", err)
	}

	// Create a registered user video less than 90 days old (should NOT be expired)
	freshUser := &Video{
		ID:          "user-fresh",
		Filename:    "fresh-user.mp4",
		Size:        1024,
		ContentType: "video/mp4",
		FilePath:    "/uploads/user-fresh.mp4",
		CreatedAt:   time.Now().Add(-30 * 24 * time.Hour),
		UserID:      &userID,
	}
	if err := db.CreateVideo(freshUser); err != nil {
		t.Fatalf("Failed to create fresh user video: %v", err)
	}

	// Get expired videos
	expired, err := db.GetExpiredVideos()
	if err != nil {
		t.Fatalf("Failed to get expired videos: %v", err)
	}

	// Should have exactly 2 expired videos
	if len(expired) != 2 {
		t.Errorf("Expected 2 expired videos, got %d", len(expired))
	}

	// Check that the correct videos are in the list
	expiredIDs := make(map[string]bool)
	for _, v := range expired {
		expiredIDs[v.ID] = true
	}

	if !expiredIDs["anon-old"] {
		t.Error("Expected 'anon-old' to be expired")
	}
	if !expiredIDs["user-old"] {
		t.Error("Expected 'user-old' to be expired")
	}
	if expiredIDs["anon-fresh"] {
		t.Error("Did not expect 'anon-fresh' to be expired")
	}
	if expiredIDs["user-fresh"] {
		t.Error("Did not expect 'user-fresh' to be expired")
	}
}

func TestDeleteVideo(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create a video with a transcription
	video := &Video{
		ID:          "video-to-delete",
		Filename:    "test.mp4",
		Size:        1024,
		ContentType: "video/mp4",
		FilePath:    "/uploads/video-to-delete.mp4",
		CreatedAt:   time.Now(),
	}
	if err := db.CreateVideo(video); err != nil {
		t.Fatalf("Failed to create video: %v", err)
	}

	transcription := &Transcription{
		ID:        "trans-to-delete",
		VideoID:   "video-to-delete",
		Status:    "complete",
		Message:   "Done",
		Progress:  100,
		CreatedAt: time.Now(),
	}
	if err := db.CreateTranscription(transcription); err != nil {
		t.Fatalf("Failed to create transcription: %v", err)
	}

	// Delete the video
	filePath, err := db.DeleteVideo("video-to-delete")
	if err != nil {
		t.Fatalf("Failed to delete video: %v", err)
	}

	// Check that the file path was returned
	if filePath != "/uploads/video-to-delete.mp4" {
		t.Errorf("Expected file path '/uploads/video-to-delete.mp4', got '%s'", filePath)
	}

	// Verify video is deleted
	retrieved, err := db.GetVideo("video-to-delete")
	if err != nil {
		t.Fatalf("Unexpected error getting deleted video: %v", err)
	}
	if retrieved != nil {
		t.Error("Video should have been deleted")
	}

	// Verify transcription is deleted
	trans, err := db.GetTranscription("video-to-delete")
	if err != nil {
		t.Fatalf("Unexpected error getting transcription: %v", err)
	}
	if trans != nil {
		t.Error("Transcription should have been deleted")
	}
}

func TestDeleteVideoNotFound(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Try to delete a non-existent video
	filePath, err := db.DeleteVideo("nonexistent")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if filePath != "" {
		t.Errorf("Expected empty file path for nonexistent video, got '%s'", filePath)
	}
}

func TestUpdateSegments(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create a video and transcription
	video := &Video{
		ID:          "video-update-segs",
		Filename:    "test.mp4",
		Size:        1024,
		ContentType: "video/mp4",
		FilePath:    "/uploads/video.mp4",
		CreatedAt:   time.Now(),
	}
	if err := db.CreateVideo(video); err != nil {
		t.Fatalf("Failed to create video: %v", err)
	}

	transcription := &Transcription{
		ID:        "trans-update-segs",
		VideoID:   "video-update-segs",
		Status:    "pending",
		CreatedAt: time.Now(),
	}
	if err := db.CreateTranscription(transcription); err != nil {
		t.Fatalf("Failed to create transcription: %v", err)
	}

	// Complete transcription with initial segments
	initialSegments := []Segment{
		{ID: 0, Start: 0.0, End: 2.5, Text: "Hello world"},
		{ID: 1, Start: 3.0, End: 5.5, Text: "This is a test"},
	}
	if err := db.CompleteTranscription("video-update-segs", "en", 10.0, "Hello world This is a test", initialSegments); err != nil {
		t.Fatalf("Failed to complete transcription: %v", err)
	}

	// Update segments with new content
	updatedSegments := []Segment{
		{ID: 0, Start: 0.0, End: 2.0, Text: "Hello everyone"},
		{ID: 1, Start: 2.5, End: 5.0, Text: "This is an edited test"},
		{ID: 2, Start: 5.5, End: 8.0, Text: "New segment added"},
	}
	if err := db.UpdateSegments("video-update-segs", updatedSegments); err != nil {
		t.Fatalf("Failed to update segments: %v", err)
	}

	// Retrieve and verify
	retrieved, err := db.GetTranscription("video-update-segs")
	if err != nil {
		t.Fatalf("Failed to get transcription: %v", err)
	}

	parsedSegments, err := retrieved.GetSegments()
	if err != nil {
		t.Fatalf("Failed to parse segments: %v", err)
	}

	if len(parsedSegments) != 3 {
		t.Errorf("Expected 3 segments, got %d", len(parsedSegments))
	}

	if parsedSegments[0].Text != "Hello everyone" {
		t.Errorf("Expected 'Hello everyone', got '%s'", parsedSegments[0].Text)
	}

	if parsedSegments[0].End != 2.0 {
		t.Errorf("Expected end time 2.0, got %f", parsedSegments[0].End)
	}

	if parsedSegments[2].Text != "New segment added" {
		t.Errorf("Expected 'New segment added', got '%s'", parsedSegments[2].Text)
	}

	// Verify full_text was updated
	expectedFullText := "Hello everyone This is an edited test New segment added"
	if retrieved.FullText != expectedFullText {
		t.Errorf("Expected full_text '%s', got '%s'", expectedFullText, retrieved.FullText)
	}
}

func TestBurnJobLifecycle(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create a video first (foreign key)
	video := &Video{
		ID:          "video-burn",
		Filename:    "test.mp4",
		Size:        1024,
		ContentType: "video/mp4",
		FilePath:    "/uploads/video-burn.mp4",
		CreatedAt:   time.Now(),
	}
	if err := db.CreateVideo(video); err != nil {
		t.Fatalf("Failed to create video: %v", err)
	}

	// Initially no burn job should exist
	job, err := db.GetBurnJob("video-burn")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if job != nil {
		t.Error("Expected no burn job initially")
	}

	// Create burn job
	burnJob := &BurnJob{
		ID:        "burn-123",
		VideoID:   "video-burn",
		Status:    "pending",
		Message:   "Waiting to start",
		Progress:  0,
		CreatedAt: time.Now(),
	}
	if err := db.CreateBurnJob(burnJob); err != nil {
		t.Fatalf("Failed to create burn job: %v", err)
	}

	// Get burn job
	job, err = db.GetBurnJob("video-burn")
	if err != nil {
		t.Fatalf("Failed to get burn job: %v", err)
	}
	if job == nil {
		t.Fatal("Expected burn job to exist")
	}
	if job.Status != "pending" {
		t.Errorf("Expected status 'pending', got '%s'", job.Status)
	}

	// Update status
	if err := db.UpdateBurnJobStatus("video-burn", "processing", "Burning subtitles...", 50); err != nil {
		t.Fatalf("Failed to update status: %v", err)
	}

	job, _ = db.GetBurnJob("video-burn")
	if job.Status != "processing" {
		t.Errorf("Expected status 'processing', got '%s'", job.Status)
	}
	if job.Progress != 50 {
		t.Errorf("Expected progress 50, got %d", job.Progress)
	}

	// Complete burn job
	if err := db.CompleteBurnJob("video-burn", "/uploads/video-burn_burned.mp4.age"); err != nil {
		t.Fatalf("Failed to complete burn job: %v", err)
	}

	job, _ = db.GetBurnJob("video-burn")
	if job.Status != "complete" {
		t.Errorf("Expected status 'complete', got '%s'", job.Status)
	}
	if job.Progress != 100 {
		t.Errorf("Expected progress 100, got %d", job.Progress)
	}
	if job.OutputPath != "/uploads/video-burn_burned.mp4.age" {
		t.Errorf("Expected output path '/uploads/video-burn_burned.mp4.age', got '%s'", job.OutputPath)
	}
	if job.CompletedAt == nil {
		t.Error("Expected completed_at to be set")
	}
}

func TestFailBurnJob(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create a video first
	video := &Video{
		ID:          "video-burn-fail",
		Filename:    "test.mp4",
		Size:        1024,
		ContentType: "video/mp4",
		FilePath:    "/uploads/video-burn-fail.mp4",
		CreatedAt:   time.Now(),
	}
	if err := db.CreateVideo(video); err != nil {
		t.Fatalf("Failed to create video: %v", err)
	}

	// Create burn job
	burnJob := &BurnJob{
		ID:        "burn-fail",
		VideoID:   "video-burn-fail",
		Status:    "processing",
		Message:   "Processing...",
		Progress:  30,
		CreatedAt: time.Now(),
	}
	if err := db.CreateBurnJob(burnJob); err != nil {
		t.Fatalf("Failed to create burn job: %v", err)
	}

	// Fail the job
	if err := db.FailBurnJob("video-burn-fail", "ffmpeg failed"); err != nil {
		t.Fatalf("Failed to fail burn job: %v", err)
	}

	job, _ := db.GetBurnJob("video-burn-fail")
	if job.Status != "error" {
		t.Errorf("Expected status 'error', got '%s'", job.Status)
	}
	if job.Message != "ffmpeg failed" {
		t.Errorf("Expected message 'ffmpeg failed', got '%s'", job.Message)
	}
}

func TestDeleteVideoWithBurnJob(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create a video with a burn job
	video := &Video{
		ID:          "video-delete-burn",
		Filename:    "test.mp4",
		Size:        1024,
		ContentType: "video/mp4",
		FilePath:    "/uploads/video-delete-burn.mp4",
		CreatedAt:   time.Now(),
	}
	if err := db.CreateVideo(video); err != nil {
		t.Fatalf("Failed to create video: %v", err)
	}

	burnJob := &BurnJob{
		ID:        "burn-to-delete",
		VideoID:   "video-delete-burn",
		Status:    "complete",
		Message:   "Done",
		Progress:  100,
		CreatedAt: time.Now(),
	}
	if err := db.CreateBurnJob(burnJob); err != nil {
		t.Fatalf("Failed to create burn job: %v", err)
	}

	// Delete the video
	_, err = db.DeleteVideo("video-delete-burn")
	if err != nil {
		t.Fatalf("Failed to delete video: %v", err)
	}

	// Verify burn job is deleted
	job, err := db.GetBurnJob("video-delete-burn")
	if err != nil {
		t.Fatalf("Unexpected error getting burn job: %v", err)
	}
	if job != nil {
		t.Error("Burn job should have been deleted")
	}
}

func TestTOTPLifecycle(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create a user
	user := &User{
		ID:           "user-totp-test",
		Email:        "totp@example.com",
		PasswordHash: "hashedpassword",
		CreatedAt:    time.Now(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Verify user is created without TOTP
	retrieved, err := db.GetUserByID("user-totp-test")
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}
	if retrieved.TOTPEnabled {
		t.Error("Expected TOTP to be disabled initially")
	}
	if retrieved.TOTPSecret != nil {
		t.Error("Expected TOTP secret to be nil initially")
	}

	// Set TOTP secret
	secret := "JBSWY3DPEHPK3PXP"
	if err := db.SetTOTPSecret("user-totp-test", secret); err != nil {
		t.Fatalf("Failed to set TOTP secret: %v", err)
	}

	// Verify secret is set but not enabled
	retrieved, _ = db.GetUserByID("user-totp-test")
	if retrieved.TOTPSecret == nil || *retrieved.TOTPSecret != secret {
		t.Error("Expected TOTP secret to be set")
	}
	if retrieved.TOTPEnabled {
		t.Error("Expected TOTP to still be disabled")
	}

	// Enable TOTP
	if err := db.EnableTOTP("user-totp-test"); err != nil {
		t.Fatalf("Failed to enable TOTP: %v", err)
	}

	// Verify TOTP is now enabled
	retrieved, _ = db.GetUserByID("user-totp-test")
	if !retrieved.TOTPEnabled {
		t.Error("Expected TOTP to be enabled")
	}
	if retrieved.TOTPSecret == nil || *retrieved.TOTPSecret != secret {
		t.Error("Expected TOTP secret to still be set")
	}

	// Disable TOTP
	if err := db.DisableTOTP("user-totp-test"); err != nil {
		t.Fatalf("Failed to disable TOTP: %v", err)
	}

	// Verify TOTP is disabled and secret is cleared
	retrieved, _ = db.GetUserByID("user-totp-test")
	if retrieved.TOTPEnabled {
		t.Error("Expected TOTP to be disabled")
	}
	if retrieved.TOTPSecret != nil {
		t.Error("Expected TOTP secret to be cleared")
	}
}

func TestGetUserByEmailWithTOTP(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create a user with TOTP enabled
	user := &User{
		ID:           "user-email-totp",
		Email:        "totpemail@example.com",
		PasswordHash: "hashedpassword",
		CreatedAt:    time.Now(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Set and enable TOTP
	secret := "TESTSECRETZZZZZZ"
	if err := db.SetTOTPSecret("user-email-totp", secret); err != nil {
		t.Fatalf("Failed to set TOTP secret: %v", err)
	}
	if err := db.EnableTOTP("user-email-totp"); err != nil {
		t.Fatalf("Failed to enable TOTP: %v", err)
	}

	// Get by email and verify TOTP fields
	retrieved, err := db.GetUserByEmail("totpemail@example.com")
	if err != nil {
		t.Fatalf("Failed to get user by email: %v", err)
	}
	if !retrieved.TOTPEnabled {
		t.Error("Expected TOTP to be enabled")
	}
	if retrieved.TOTPSecret == nil || *retrieved.TOTPSecret != secret {
		t.Error("Expected TOTP secret to match")
	}
}

func TestRecoveryCodesLifecycle(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create a user
	user := &User{
		ID:           "user-recovery",
		Email:        "recovery@example.com",
		PasswordHash: "hashedpassword",
		CreatedAt:    time.Now(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Initially no recovery codes
	codes, err := db.GetUnusedRecoveryCodes("user-recovery")
	if err != nil {
		t.Fatalf("Failed to get recovery codes: %v", err)
	}
	if len(codes) != 0 {
		t.Errorf("Expected 0 codes initially, got %d", len(codes))
	}

	// Save 10 hashed recovery codes
	hashes := []string{
		"$2a$10$hash1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		"$2a$10$hash2xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		"$2a$10$hash3xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		"$2a$10$hash4xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		"$2a$10$hash5xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		"$2a$10$hash6xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		"$2a$10$hash7xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		"$2a$10$hash8xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		"$2a$10$hash9xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		"$2a$10$hash10xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
	}
	if err := db.SaveRecoveryCodes("user-recovery", hashes); err != nil {
		t.Fatalf("Failed to save recovery codes: %v", err)
	}

	// Verify all 10 codes exist
	codes, err = db.GetUnusedRecoveryCodes("user-recovery")
	if err != nil {
		t.Fatalf("Failed to get recovery codes: %v", err)
	}
	if len(codes) != 10 {
		t.Errorf("Expected 10 codes, got %d", len(codes))
	}

	// Count unused codes
	count, err := db.CountUnusedRecoveryCodes("user-recovery")
	if err != nil {
		t.Fatalf("Failed to count codes: %v", err)
	}
	if count != 10 {
		t.Errorf("Expected count 10, got %d", count)
	}

	// Use one code
	codeID := codes[0].ID
	success, err := db.UseRecoveryCode(codeID)
	if err != nil {
		t.Fatalf("Failed to use recovery code: %v", err)
	}
	if !success {
		t.Error("Expected use to succeed")
	}

	// Verify count reduced
	count, _ = db.CountUnusedRecoveryCodes("user-recovery")
	if count != 9 {
		t.Errorf("Expected count 9 after using one, got %d", count)
	}

	// Try to use the same code again
	success, err = db.UseRecoveryCode(codeID)
	if err != nil {
		t.Fatalf("Failed to check used code: %v", err)
	}
	if success {
		t.Error("Expected second use to fail")
	}
}

func TestRecoveryCodesRegenerate(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create a user
	user := &User{
		ID:           "user-regen",
		Email:        "regen@example.com",
		PasswordHash: "hashedpassword",
		CreatedAt:    time.Now(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Save initial codes
	initialHashes := []string{"$2a$10$initialxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"}
	if err := db.SaveRecoveryCodes("user-regen", initialHashes); err != nil {
		t.Fatalf("Failed to save initial codes: %v", err)
	}

	// Regenerate with new codes (old ones should be deleted)
	newHashes := []string{
		"$2a$10$new1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		"$2a$10$new2xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
	}
	if err := db.SaveRecoveryCodes("user-regen", newHashes); err != nil {
		t.Fatalf("Failed to save new codes: %v", err)
	}

	// Should have exactly 2 new codes
	codes, err := db.GetUnusedRecoveryCodes("user-regen")
	if err != nil {
		t.Fatalf("Failed to get codes: %v", err)
	}
	if len(codes) != 2 {
		t.Errorf("Expected 2 codes after regeneration, got %d", len(codes))
	}
}

func TestRecoveryCodesDelete(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create a user
	user := &User{
		ID:           "user-delete-codes",
		Email:        "delete@example.com",
		PasswordHash: "hashedpassword",
		CreatedAt:    time.Now(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Save codes
	hashes := []string{"$2a$10$deletexxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"}
	if err := db.SaveRecoveryCodes("user-delete-codes", hashes); err != nil {
		t.Fatalf("Failed to save codes: %v", err)
	}

	// Delete all codes
	if err := db.DeleteRecoveryCodes("user-delete-codes"); err != nil {
		t.Fatalf("Failed to delete codes: %v", err)
	}

	// Verify all deleted
	count, _ := db.CountUnusedRecoveryCodes("user-delete-codes")
	if count != 0 {
		t.Errorf("Expected 0 codes after delete, got %d", count)
	}
}
