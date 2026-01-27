package db

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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

func TestListVideosPaginated(t *testing.T) {
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

	// Create 10 videos for a user
	userID := "user-pagination"
	for i := 0; i < 10; i++ {
		video := &Video{
			ID:          fmt.Sprintf("video-page-%02d", i),
			Filename:    fmt.Sprintf("test-%d.mp4", i),
			Size:        1024,
			ContentType: "video/mp4",
			FilePath:    "/uploads/video.mp4",
			CreatedAt:   time.Now().Add(time.Duration(-i) * time.Minute), // Stagger creation times
			UserID:      &userID,
		}
		db.CreateVideo(video)
	}

	// Test default (no pagination)
	result, err := db.ListVideosPaginated(&userID, nil, 0, 0)
	if err != nil {
		t.Fatalf("Failed to list videos: %v", err)
	}
	if result.TotalCount != 10 {
		t.Errorf("Expected total_count 10, got %d", result.TotalCount)
	}
	if len(result.Videos) != 10 {
		t.Errorf("Expected 10 videos, got %d", len(result.Videos))
	}

	// Test first page (limit 3)
	result, err = db.ListVideosPaginated(&userID, nil, 3, 0)
	if err != nil {
		t.Fatalf("Failed to list videos: %v", err)
	}
	if result.TotalCount != 10 {
		t.Errorf("Expected total_count 10, got %d", result.TotalCount)
	}
	if len(result.Videos) != 3 {
		t.Errorf("Expected 3 videos, got %d", len(result.Videos))
	}

	// Test second page (limit 3, offset 3)
	result, err = db.ListVideosPaginated(&userID, nil, 3, 3)
	if err != nil {
		t.Fatalf("Failed to list videos: %v", err)
	}
	if len(result.Videos) != 3 {
		t.Errorf("Expected 3 videos on second page, got %d", len(result.Videos))
	}

	// Test last partial page (limit 3, offset 9)
	result, err = db.ListVideosPaginated(&userID, nil, 3, 9)
	if err != nil {
		t.Fatalf("Failed to list videos: %v", err)
	}
	if len(result.Videos) != 1 {
		t.Errorf("Expected 1 video on last page, got %d", len(result.Videos))
	}

	// Test offset beyond total
	result, err = db.ListVideosPaginated(&userID, nil, 3, 15)
	if err != nil {
		t.Fatalf("Failed to list videos: %v", err)
	}
	if len(result.Videos) != 0 {
		t.Errorf("Expected 0 videos beyond total, got %d", len(result.Videos))
	}
	if result.TotalCount != 10 {
		t.Errorf("Expected total_count 10 even beyond offset, got %d", result.TotalCount)
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
	deletedFiles, err := db.DeleteVideo("video-to-delete")
	if err != nil {
		t.Fatalf("Failed to delete video: %v", err)
	}

	// Check that the file path was returned
	if deletedFiles == nil {
		t.Fatal("Expected deletedFiles to be non-nil")
	}
	if deletedFiles.FilePath != "/uploads/video-to-delete.mp4" {
		t.Errorf("Expected file path '/uploads/video-to-delete.mp4', got '%s'", deletedFiles.FilePath)
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
	deletedFiles, err := db.DeleteVideo("nonexistent")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if deletedFiles != nil {
		t.Errorf("Expected nil for nonexistent video, got %+v", deletedFiles)
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

func TestPasswordResetTokenLifecycle(t *testing.T) {
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
		ID:           "user-reset-token",
		Email:        "reset@example.com",
		PasswordHash: "hashedpassword",
		CreatedAt:    time.Now(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create a reset token
	tokenHash := "abc123hashedtoken"
	expiresAt := time.Now().Add(1 * time.Hour)
	token, err := db.CreatePasswordResetToken(user.ID, tokenHash, expiresAt)
	if err != nil {
		t.Fatalf("Failed to create reset token: %v", err)
	}

	if token.UserID != user.ID {
		t.Errorf("Expected user ID %s, got %s", user.ID, token.UserID)
	}
	if token.TokenHash != tokenHash {
		t.Errorf("Expected token hash %s, got %s", tokenHash, token.TokenHash)
	}
	if token.Used {
		t.Error("New token should not be marked as used")
	}

	// Get the token
	retrieved, err := db.GetPasswordResetToken(tokenHash)
	if err != nil {
		t.Fatalf("Failed to get reset token: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Retrieved token is nil")
	}
	if retrieved.ID != token.ID {
		t.Errorf("Expected token ID %s, got %s", token.ID, retrieved.ID)
	}

	// Use the token
	used, err := db.UsePasswordResetToken(tokenHash)
	if err != nil {
		t.Fatalf("Failed to use reset token: %v", err)
	}
	if !used {
		t.Error("UsePasswordResetToken should return true for valid token")
	}

	// Try to use again (should fail)
	usedAgain, err := db.UsePasswordResetToken(tokenHash)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if usedAgain {
		t.Error("UsePasswordResetToken should return false for already-used token")
	}
}

func TestPasswordResetTokenExpired(t *testing.T) {
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
		ID:           "user-expired-token",
		Email:        "expired@example.com",
		PasswordHash: "hashedpassword",
		CreatedAt:    time.Now(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create an expired token
	tokenHash := "expiredtokenhash"
	expiresAt := time.Now().Add(-1 * time.Hour) // Already expired
	_, err = db.CreatePasswordResetToken(user.ID, tokenHash, expiresAt)
	if err != nil {
		t.Fatalf("Failed to create reset token: %v", err)
	}

	// Try to use expired token
	used, err := db.UsePasswordResetToken(tokenHash)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if used {
		t.Error("UsePasswordResetToken should return false for expired token")
	}
}

func TestPasswordResetTokenNotFound(t *testing.T) {
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

	// Try to get non-existent token
	token, err := db.GetPasswordResetToken("nonexistent")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if token != nil {
		t.Error("Expected nil for non-existent token")
	}
}

func TestDeletePasswordResetTokens(t *testing.T) {
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
		ID:           "user-delete-tokens",
		Email:        "deletetokens@example.com",
		PasswordHash: "hashedpassword",
		CreatedAt:    time.Now(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create a token
	tokenHash := "deletetokenhash"
	_, err = db.CreatePasswordResetToken(user.ID, tokenHash, time.Now().Add(1*time.Hour))
	if err != nil {
		t.Fatalf("Failed to create reset token: %v", err)
	}

	// Delete all tokens for user
	if err := db.DeletePasswordResetTokens(user.ID); err != nil {
		t.Fatalf("Failed to delete tokens: %v", err)
	}

	// Verify token is deleted
	token, _ := db.GetPasswordResetToken(tokenHash)
	if token != nil {
		t.Error("Token should be deleted")
	}
}

func TestUpdateUserPassword(t *testing.T) {
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
		ID:           "user-update-password",
		Email:        "updatepwd@example.com",
		PasswordHash: "oldhash",
		CreatedAt:    time.Now(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Update password
	newHash := "newhash"
	if err := db.UpdateUserPassword(user.ID, newHash); err != nil {
		t.Fatalf("Failed to update password: %v", err)
	}

	// Verify update
	updated, _ := db.GetUserByID(user.ID)
	if updated.PasswordHash != newHash {
		t.Errorf("Expected password hash %s, got %s", newHash, updated.PasswordHash)
	}
}

func TestLoginAttempts(t *testing.T) {
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

	email := "test@example.com"
	ip := "192.168.1.1"

	// Initially should have 0 failed attempts
	count, err := db.GetRecentFailedLoginAttempts(email, time.Now().Add(-15*time.Minute))
	if err != nil {
		t.Fatalf("Failed to get login attempts: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 failed attempts, got %d", count)
	}

	// Record 3 failed attempts
	for i := 0; i < 3; i++ {
		if err := db.RecordLoginAttempt(email, ip, false); err != nil {
			t.Fatalf("Failed to record login attempt: %v", err)
		}
	}

	// Should now have 3 failed attempts
	count, err = db.GetRecentFailedLoginAttempts(email, time.Now().Add(-15*time.Minute))
	if err != nil {
		t.Fatalf("Failed to get login attempts: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected 3 failed attempts, got %d", count)
	}

	// Email should not be locked yet (under 5 attempts)
	locked, _, err := db.IsEmailLocked(email, 5, 15*time.Minute)
	if err != nil {
		t.Fatalf("Failed to check email lock: %v", err)
	}
	if locked {
		t.Error("Email should not be locked with only 3 failed attempts")
	}

	// Record 2 more failed attempts (total 5)
	for i := 0; i < 2; i++ {
		if err := db.RecordLoginAttempt(email, ip, false); err != nil {
			t.Fatalf("Failed to record login attempt: %v", err)
		}
	}

	// Email should now be locked
	locked, unlockTime, err := db.IsEmailLocked(email, 5, 15*time.Minute)
	if err != nil {
		t.Fatalf("Failed to check email lock: %v", err)
	}
	if !locked {
		t.Error("Email should be locked with 5 failed attempts")
	}
	if unlockTime.Before(time.Now()) {
		t.Error("Unlock time should be in the future")
	}

	// Clear attempts (simulating successful login)
	if err := db.ClearLoginAttempts(email); err != nil {
		t.Fatalf("Failed to clear login attempts: %v", err)
	}

	// Should no longer be locked
	locked, _, err = db.IsEmailLocked(email, 5, 15*time.Minute)
	if err != nil {
		t.Fatalf("Failed to check email lock: %v", err)
	}
	if locked {
		t.Error("Email should not be locked after clearing attempts")
	}

	// Count should be 0 again
	count, err = db.GetRecentFailedLoginAttempts(email, time.Now().Add(-15*time.Minute))
	if err != nil {
		t.Fatalf("Failed to get login attempts: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 failed attempts after clearing, got %d", count)
	}
}

func TestDeleteExpiredLoginAttempts(t *testing.T) {
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

	email := "test@example.com"
	ip := "192.168.1.1"

	// Record 3 failed attempts
	for i := 0; i < 3; i++ {
		if err := db.RecordLoginAttempt(email, ip, false); err != nil {
			t.Fatalf("Failed to record login attempt: %v", err)
		}
	}

	// Verify we have 3 attempts
	count, _ := db.GetRecentFailedLoginAttempts(email, time.Now().Add(-1*time.Hour))
	if count != 3 {
		t.Errorf("Expected 3 attempts, got %d", count)
	}

	// Delete attempts older than "now" (should delete all)
	deleted, err := db.DeleteExpiredLoginAttempts(time.Now().Add(1 * time.Second))
	if err != nil {
		t.Fatalf("Failed to delete expired attempts: %v", err)
	}
	if deleted != 3 {
		t.Errorf("Expected to delete 3 attempts, deleted %d", deleted)
	}

	// Should have 0 attempts now
	count, _ = db.GetRecentFailedLoginAttempts(email, time.Now().Add(-1*time.Hour))
	if count != 0 {
		t.Errorf("Expected 0 attempts after deletion, got %d", count)
	}
}

func TestWithTransaction(t *testing.T) {
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
		ID:           "txtest123",
		Email:        "txtest@example.com",
		PasswordHash: "hashhash",
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Test successful transaction
	err = db.WithTransaction(func(tx *Tx) error {
		_, err := tx.tx.Exec(`UPDATE users SET email = ? WHERE id = ?`, "newemail@example.com", user.ID)
		return err
	})
	if err != nil {
		t.Fatalf("Transaction should have succeeded: %v", err)
	}

	// Verify the change was committed
	updatedUser, err := db.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}
	if updatedUser.Email != "newemail@example.com" {
		t.Errorf("Expected email 'newemail@example.com', got '%s'", updatedUser.Email)
	}
}

func TestWithTransactionRollback(t *testing.T) {
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
		ID:           "txrollback123",
		Email:        "txrollback@example.com",
		PasswordHash: "hashhash",
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Test transaction that fails and rolls back
	testErr := fmt.Errorf("intentional test error")
	err = db.WithTransaction(func(tx *Tx) error {
		// Make a change
		_, err := tx.tx.Exec(`UPDATE users SET email = ? WHERE id = ?`, "shouldrollback@example.com", user.ID)
		if err != nil {
			return err
		}
		// Then fail
		return testErr
	})
	if err != testErr {
		t.Errorf("Expected test error, got: %v", err)
	}

	// Verify the change was rolled back
	unchangedUser, err := db.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}
	if unchangedUser.Email != "txrollback@example.com" {
		t.Errorf("Expected original email 'txrollback@example.com', got '%s' (rollback failed)", unchangedUser.Email)
	}
}

func TestWithTransactionUsesContext(t *testing.T) {
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

	// Verify that WithTransaction uses a timeout context by setting a very short timeout
	// and performing a simple operation - it should still work since SQLite is fast
	db.SetQueryTimeout(100 * time.Millisecond)

	user := &User{
		ID:           "txtimeout123",
		Email:        "txtimeout@example.com",
		PasswordHash: "hashhash",
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Transaction should succeed with short timeout for fast operations
	err = db.WithTransaction(func(tx *Tx) error {
		_, err := tx.tx.Exec(`UPDATE users SET email = ? WHERE id = ?`, "updated@example.com", user.ID)
		return err
	})
	if err != nil {
		t.Errorf("Transaction with short timeout should succeed for fast operations: %v", err)
	}

	// Verify the change was committed
	updatedUser, err := db.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}
	if updatedUser.Email != "updated@example.com" {
		t.Errorf("Expected email 'updated@example.com', got '%s'", updatedUser.Email)
	}
}

func TestEnableTOTPWithRecoveryCodes(t *testing.T) {
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

	// Create a user with TOTP secret
	user := &User{
		ID:           "totp123",
		Email:        "totp@example.com",
		PasswordHash: "hashhash",
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Set TOTP secret first
	if err := db.SetTOTPSecret(user.ID, "secret123"); err != nil {
		t.Fatalf("Failed to set TOTP secret: %v", err)
	}

	// Enable TOTP with recovery codes atomically
	codeHashes := []string{"hash1", "hash2", "hash3"}
	if err := db.EnableTOTPWithRecoveryCodes(user.ID, codeHashes); err != nil {
		t.Fatalf("Failed to enable TOTP with recovery codes: %v", err)
	}

	// Verify TOTP is enabled
	updatedUser, err := db.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}
	if !updatedUser.TOTPEnabled {
		t.Error("Expected TOTP to be enabled")
	}

	// Verify recovery codes were saved
	codes, err := db.GetUnusedRecoveryCodes(user.ID)
	if err != nil {
		t.Fatalf("Failed to get recovery codes: %v", err)
	}
	if len(codes) != 3 {
		t.Errorf("Expected 3 recovery codes, got %d", len(codes))
	}
}

func TestCompletePasswordReset(t *testing.T) {
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
		ID:           "pwreset123",
		Email:        "pwreset@example.com",
		PasswordHash: "oldhash",
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create a session
	session := &Session{
		ID:        "session123",
		UserID:    user.ID,
		Token:     "token123",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	if err := db.CreateSession(session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Create a password reset token
	tokenHash := "tokenhash123"
	token, err := db.CreatePasswordResetToken(user.ID, tokenHash, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("Failed to create reset token: %v", err)
	}

	// Complete password reset atomically
	newPasswordHash := "newhash"
	if err := db.CompletePasswordReset(user.ID, token.TokenHash, newPasswordHash); err != nil {
		t.Fatalf("Failed to complete password reset: %v", err)
	}

	// Verify password was updated
	updatedUser, err := db.GetUserByEmail(user.Email)
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}
	if updatedUser.PasswordHash != newPasswordHash {
		t.Errorf("Expected password hash '%s', got '%s'", newPasswordHash, updatedUser.PasswordHash)
	}

	// Verify sessions were deleted (GetSessionByToken returns nil, nil for not found)
	deletedSession, err := db.GetSessionByToken("token123")
	if err != nil {
		t.Fatalf("Error checking session: %v", err)
	}
	if deletedSession != nil {
		t.Error("Expected session to be deleted")
	}

	// Verify token was deleted
	retrievedToken, err := db.GetPasswordResetToken(tokenHash)
	if err == nil && retrievedToken != nil && !retrievedToken.Used {
		t.Error("Expected token to be used or deleted")
	}
}

func TestDisableTOTPAndClearSessions(t *testing.T) {
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

	// Create a user with 2FA enabled
	user := &User{
		ID:           "disable2fa123",
		Email:        "disable2fa@example.com",
		PasswordHash: "hashhash",
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Enable TOTP
	if err := db.SetTOTPSecret(user.ID, "secret"); err != nil {
		t.Fatalf("Failed to set TOTP secret: %v", err)
	}
	if err := db.EnableTOTP(user.ID); err != nil {
		t.Fatalf("Failed to enable TOTP: %v", err)
	}

	// Add recovery codes
	if err := db.SaveRecoveryCodes(user.ID, []string{"hash1", "hash2"}); err != nil {
		t.Fatalf("Failed to save recovery codes: %v", err)
	}

	// Create a session
	session := &Session{
		ID:        "session2fa123",
		UserID:    user.ID,
		Token:     "token2fa123",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	if err := db.CreateSession(session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Disable 2FA and clear sessions atomically
	if err := db.DisableTOTPAndClearSessions(user.ID); err != nil {
		t.Fatalf("Failed to disable TOTP and clear sessions: %v", err)
	}

	// Verify TOTP is disabled
	updatedUser, err := db.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}
	if updatedUser.TOTPEnabled {
		t.Error("Expected TOTP to be disabled")
	}

	// Verify recovery codes were deleted
	codes, err := db.GetUnusedRecoveryCodes(user.ID)
	if err != nil {
		t.Fatalf("Failed to get recovery codes: %v", err)
	}
	if len(codes) != 0 {
		t.Errorf("Expected 0 recovery codes, got %d", len(codes))
	}

	// Verify sessions were deleted (GetSessionByToken returns nil, nil for not found)
	deletedSession, err := db.GetSessionByToken("token2fa123")
	if err != nil {
		t.Fatalf("Error checking session: %v", err)
	}
	if deletedSession != nil {
		t.Error("Expected session to be deleted")
	}
}

func TestVacuum(t *testing.T) {
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

	// Add some data then delete it to create fragmentation
	for i := 0; i < 100; i++ {
		video := &Video{
			ID:          fmt.Sprintf("vacuum-test-%d", i),
			Filename:    "test.mp4",
			Size:        1024,
			ContentType: "video/mp4",
			FilePath:    "/tmp/test.mp4",
			CreatedAt:   time.Now(),
		}
		if err := db.CreateVideo(video); err != nil {
			t.Fatalf("Failed to create video %d: %v", i, err)
		}
	}

	// Delete the videos to create free space
	for i := 0; i < 100; i++ {
		db.DeleteVideo(fmt.Sprintf("vacuum-test-%d", i))
	}

	// VACUUM should reclaim the space
	if err := db.Vacuum(); err != nil {
		t.Fatalf("Vacuum failed: %v", err)
	}
}

func TestAnalyze(t *testing.T) {
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

	// Add some data to have statistics to analyze
	for i := 0; i < 10; i++ {
		video := &Video{
			ID:          fmt.Sprintf("analyze-test-%d", i),
			Filename:    "test.mp4",
			Size:        1024,
			ContentType: "video/mp4",
			FilePath:    "/tmp/test.mp4",
			CreatedAt:   time.Now(),
		}
		if err := db.CreateVideo(video); err != nil {
			t.Fatalf("Failed to create video %d: %v", i, err)
		}
	}

	// ANALYZE should update query planner statistics
	if err := db.Analyze(); err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
}

func TestMaintenance(t *testing.T) {
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

	// Add some test data
	video := &Video{
		ID:          "maintenance-test",
		Filename:    "test.mp4",
		Size:        1024,
		ContentType: "video/mp4",
		FilePath:    "/tmp/test.mp4",
		CreatedAt:   time.Now(),
	}
	if err := db.CreateVideo(video); err != nil {
		t.Fatalf("Failed to create video: %v", err)
	}

	// Maintenance runs both VACUUM and ANALYZE
	if err := db.Maintenance(); err != nil {
		t.Fatalf("Maintenance failed: %v", err)
	}

	// Verify data is still intact after maintenance
	retrieved, err := db.GetVideo("maintenance-test")
	if err != nil {
		t.Fatalf("Failed to get video after maintenance: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Video not found after maintenance")
	}
	if retrieved.Filename != "test.mp4" {
		t.Errorf("Expected filename 'test.mp4', got '%s'", retrieved.Filename)
	}
}

func TestUploadSessionExists(t *testing.T) {
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

	// Test non-existent session
	exists, err := db.UploadSessionExists("nonexistent-session")
	if err != nil {
		t.Fatalf("Unexpected error checking session existence: %v", err)
	}
	if exists {
		t.Error("Expected session to not exist")
	}

	// Create an upload session
	session := &UploadSession{
		ID:          "test-session-exists",
		Filename:    "test.mp4",
		ContentType: "video/mp4",
		TotalSize:   1024000,
		ChunkSize:   512000,
		TotalChunks: 2,
		Status:      "in_progress",
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}
	if err := db.CreateUploadSession(session); err != nil {
		t.Fatalf("Failed to create upload session: %v", err)
	}

	// Test existing session
	exists, err = db.UploadSessionExists("test-session-exists")
	if err != nil {
		t.Fatalf("Unexpected error checking session existence: %v", err)
	}
	if !exists {
		t.Error("Expected session to exist")
	}

	// Delete the session
	_, err = db.DeleteUploadSession("test-session-exists")
	if err != nil {
		t.Fatalf("Failed to delete session: %v", err)
	}

	// Verify it no longer exists
	exists, err = db.UploadSessionExists("test-session-exists")
	if err != nil {
		t.Fatalf("Unexpected error checking session existence: %v", err)
	}
	if exists {
		t.Error("Expected session to not exist after deletion")
	}
}

// hashToken creates a SHA-256 hash of a token (same as email.HashToken)
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func TestEmailVerificationTokenCleanupAfterVerification(t *testing.T) {
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

	// Create a test user
	user := &User{
		ID:            "test-user-cleanup",
		Email:         "cleanup@example.com",
		PasswordHash:  "testhash",
		EmailVerified: false,
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create a verification token
	expiresAt := time.Now().Add(24 * time.Hour)
	tokenHash := hashToken("verification-token")
	_, err = db.CreateEmailVerificationToken(user.ID, tokenHash, expiresAt)
	if err != nil {
		t.Fatalf("Failed to create verification token: %v", err)
	}

	// Verify we have 1 token for this user
	var count int
	err = db.conn.QueryRow(`SELECT COUNT(*) FROM email_verification_tokens WHERE user_id = ?`, user.ID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count tokens: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 token before verification, got %d", count)
	}

	// Use the token to verify email
	verified, err := db.UseEmailVerificationToken(tokenHash)
	if err != nil {
		t.Fatalf("Failed to use verification token: %v", err)
	}
	if !verified {
		t.Error("Expected token verification to succeed")
	}

	// Verify user is now verified
	verifiedUser, err := db.GetUserByEmail("cleanup@example.com")
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}
	if !verifiedUser.EmailVerified {
		t.Error("Expected user email to be verified")
	}

	// Verify the token has been deleted (cleanup)
	err = db.conn.QueryRow(`SELECT COUNT(*) FROM email_verification_tokens WHERE user_id = ?`, user.ID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count tokens after verification: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 tokens after verification (cleanup), got %d", count)
	}
}

func TestEmailVerificationTokenCleanupWithMultipleTokens(t *testing.T) {
	// Test that cleanup works when there are "used" tokens in the database
	// (Note: CreateEmailVerificationToken only keeps 1 unused token per user,
	//  but used tokens can accumulate. This test verifies we clean those up too.)
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

	// Create a test user
	user := &User{
		ID:            "test-user-multi",
		Email:         "multi@example.com",
		PasswordHash:  "testhash",
		EmailVerified: false,
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Manually insert some "used" tokens to simulate historical accumulation
	for i := 0; i < 3; i++ {
		id, _ := generateID()
		_, err := db.conn.Exec(`
			INSERT INTO email_verification_tokens (id, user_id, token_hash, expires_at, used, created_at)
			VALUES (?, ?, ?, ?, 1, ?)
		`, id, user.ID, hashToken(fmt.Sprintf("old-token-%d", i)), time.Now().Add(-1*time.Hour), time.Now().Add(-2*time.Hour))
		if err != nil {
			t.Fatalf("Failed to insert old token: %v", err)
		}
	}

	// Create a fresh verification token
	expiresAt := time.Now().Add(24 * time.Hour)
	tokenHash := hashToken("fresh-token")
	_, err = db.CreateEmailVerificationToken(user.ID, tokenHash, expiresAt)
	if err != nil {
		t.Fatalf("Failed to create verification token: %v", err)
	}

	// Verify we have 4 tokens total (3 used + 1 unused)
	var count int
	err = db.conn.QueryRow(`SELECT COUNT(*) FROM email_verification_tokens WHERE user_id = ?`, user.ID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count tokens: %v", err)
	}
	if count != 4 {
		t.Errorf("Expected 4 tokens before verification, got %d", count)
	}

	// Use the fresh token to verify email
	verified, err := db.UseEmailVerificationToken(tokenHash)
	if err != nil {
		t.Fatalf("Failed to use verification token: %v", err)
	}
	if !verified {
		t.Error("Expected token verification to succeed")
	}

	// Verify ALL tokens for this user have been deleted (including used ones)
	err = db.conn.QueryRow(`SELECT COUNT(*) FROM email_verification_tokens WHERE user_id = ?`, user.ID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count tokens after verification: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 tokens after verification (cleanup of all tokens), got %d", count)
	}
}

// TestTranscriptionStatusRaceProtection verifies that progress updates don't
// overwrite completed/failed status (race condition fix)
func TestTranscriptionStatusRaceProtection(t *testing.T) {
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
		ID:          "video-race-test",
		Filename:    "test.mp4",
		Size:        1024,
		ContentType: "video/mp4",
		FilePath:    "/uploads/video-race-test.mp4",
		CreatedAt:   time.Now(),
	}
	if err := db.CreateVideo(video); err != nil {
		t.Fatalf("Failed to create video: %v", err)
	}

	t.Run("progress update blocked after completion", func(t *testing.T) {
		// Create transcription in processing state
		transcription := &Transcription{
			ID:        "trans-race-1",
			VideoID:   "video-race-test",
			Status:    "processing",
			Message:   "Transcribing...",
			Progress:  50,
			CreatedAt: time.Now(),
		}
		if err := db.CreateTranscription(transcription); err != nil {
			t.Fatalf("Failed to create transcription: %v", err)
		}

		// Complete the transcription
		segments := []Segment{{ID: 0, Start: 0, End: 1, Text: "Test"}}
		if err := db.CompleteTranscription("video-race-test", "en", 1.0, "Test", segments); err != nil {
			t.Fatalf("Failed to complete transcription: %v", err)
		}

		// Simulate a late progress update (race condition scenario)
		// This should NOT overwrite the completed status
		if err := db.UpdateTranscriptionStatus("video-race-test", "processing", "Late update", 75); err != nil {
			t.Fatalf("UpdateTranscriptionStatus returned error: %v", err)
		}

		// Verify status is still 'complete', not 'processing'
		retrieved, err := db.GetTranscription("video-race-test")
		if err != nil {
			t.Fatalf("Failed to get transcription: %v", err)
		}
		if retrieved.Status != "complete" {
			t.Errorf("Status was overwritten by late progress update: expected 'complete', got '%s'", retrieved.Status)
		}
		if retrieved.Progress != 100 {
			t.Errorf("Progress was overwritten: expected 100, got %d", retrieved.Progress)
		}

		// Cleanup for next subtest
		db.conn.Exec(`DELETE FROM transcriptions WHERE video_id = ?`, "video-race-test")
	})

	t.Run("progress update blocked after failure", func(t *testing.T) {
		// Create transcription in processing state
		transcription := &Transcription{
			ID:        "trans-race-2",
			VideoID:   "video-race-test",
			Status:    "processing",
			Message:   "Transcribing...",
			Progress:  30,
			CreatedAt: time.Now(),
		}
		if err := db.CreateTranscription(transcription); err != nil {
			t.Fatalf("Failed to create transcription: %v", err)
		}

		// Fail the transcription
		if err := db.FailTranscription("video-race-test", "Transcription failed"); err != nil {
			t.Fatalf("Failed to fail transcription: %v", err)
		}

		// Simulate a late progress update (race condition scenario)
		if err := db.UpdateTranscriptionStatus("video-race-test", "processing", "Late update", 60); err != nil {
			t.Fatalf("UpdateTranscriptionStatus returned error: %v", err)
		}

		// Verify status is still 'error', not 'processing'
		retrieved, err := db.GetTranscription("video-race-test")
		if err != nil {
			t.Fatalf("Failed to get transcription: %v", err)
		}
		if retrieved.Status != "error" {
			t.Errorf("Status was overwritten by late progress update: expected 'error', got '%s'", retrieved.Status)
		}
		if retrieved.Message != "Transcription failed" {
			t.Errorf("Message was overwritten: expected 'Transcription failed', got '%s'", retrieved.Message)
		}

		// Cleanup
		db.conn.Exec(`DELETE FROM transcriptions WHERE video_id = ?`, "video-race-test")
	})

	t.Run("progress update works when still processing", func(t *testing.T) {
		// Create transcription in processing state
		transcription := &Transcription{
			ID:        "trans-race-3",
			VideoID:   "video-race-test",
			Status:    "processing",
			Message:   "Transcribing...",
			Progress:  30,
			CreatedAt: time.Now(),
		}
		if err := db.CreateTranscription(transcription); err != nil {
			t.Fatalf("Failed to create transcription: %v", err)
		}

		// Update progress while still processing (normal case)
		if err := db.UpdateTranscriptionStatus("video-race-test", "processing", "Still transcribing...", 60); err != nil {
			t.Fatalf("UpdateTranscriptionStatus returned error: %v", err)
		}

		// Verify progress was updated
		retrieved, err := db.GetTranscription("video-race-test")
		if err != nil {
			t.Fatalf("Failed to get transcription: %v", err)
		}
		if retrieved.Status != "processing" {
			t.Errorf("Expected status 'processing', got '%s'", retrieved.Status)
		}
		if retrieved.Progress != 60 {
			t.Errorf("Expected progress 60, got %d", retrieved.Progress)
		}
		if retrieved.Message != "Still transcribing..." {
			t.Errorf("Expected message 'Still transcribing...', got '%s'", retrieved.Message)
		}
	})
}

// TestBurnJobStatusRaceProtection verifies that burn job progress updates don't
// overwrite completed/failed status (race condition fix)
func TestBurnJobStatusRaceProtection(t *testing.T) {
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
		ID:          "video-burn-race",
		Filename:    "test.mp4",
		Size:        1024,
		ContentType: "video/mp4",
		FilePath:    "/uploads/video-burn-race.mp4",
		CreatedAt:   time.Now(),
	}
	if err := db.CreateVideo(video); err != nil {
		t.Fatalf("Failed to create video: %v", err)
	}

	t.Run("progress update blocked after burn completion", func(t *testing.T) {
		// Create burn job in processing state
		burnJob := &BurnJob{
			ID:        "burn-race-1",
			VideoID:   "video-burn-race",
			Status:    "processing",
			Message:   "Burning...",
			Progress:  50,
			CreatedAt: time.Now(),
		}
		if err := db.CreateBurnJob(burnJob); err != nil {
			t.Fatalf("Failed to create burn job: %v", err)
		}

		// Complete the burn job
		if err := db.CompleteBurnJob("video-burn-race", "/output/burned.mp4"); err != nil {
			t.Fatalf("Failed to complete burn job: %v", err)
		}

		// Simulate a late progress update
		if err := db.UpdateBurnJobStatus("video-burn-race", "processing", "Late update", 75); err != nil {
			t.Fatalf("UpdateBurnJobStatus returned error: %v", err)
		}

		// Verify status is still 'complete'
		retrieved, err := db.GetBurnJob("video-burn-race")
		if err != nil {
			t.Fatalf("Failed to get burn job: %v", err)
		}
		if retrieved.Status != "complete" {
			t.Errorf("Status was overwritten: expected 'complete', got '%s'", retrieved.Status)
		}
		if retrieved.Progress != 100 {
			t.Errorf("Progress was overwritten: expected 100, got %d", retrieved.Progress)
		}

		// Cleanup
		db.conn.Exec(`DELETE FROM burn_jobs WHERE video_id = ?`, "video-burn-race")
	})

	t.Run("progress update blocked after burn failure", func(t *testing.T) {
		// Create burn job in processing state
		burnJob := &BurnJob{
			ID:        "burn-race-2",
			VideoID:   "video-burn-race",
			Status:    "processing",
			Message:   "Burning...",
			Progress:  30,
			CreatedAt: time.Now(),
		}
		if err := db.CreateBurnJob(burnJob); err != nil {
			t.Fatalf("Failed to create burn job: %v", err)
		}

		// Fail the burn job
		if err := db.FailBurnJob("video-burn-race", "FFmpeg failed"); err != nil {
			t.Fatalf("Failed to fail burn job: %v", err)
		}

		// Simulate a late progress update
		if err := db.UpdateBurnJobStatus("video-burn-race", "processing", "Late update", 60); err != nil {
			t.Fatalf("UpdateBurnJobStatus returned error: %v", err)
		}

		// Verify status is still 'error'
		retrieved, err := db.GetBurnJob("video-burn-race")
		if err != nil {
			t.Fatalf("Failed to get burn job: %v", err)
		}
		if retrieved.Status != "error" {
			t.Errorf("Status was overwritten: expected 'error', got '%s'", retrieved.Status)
		}
		if retrieved.Message != "FFmpeg failed" {
			t.Errorf("Message was overwritten: expected 'FFmpeg failed', got '%s'", retrieved.Message)
		}
	})
}

// TestCountRecentMagicLinkRequests tests the per-email rate limiting for magic links
func TestCountRecentMagicLinkRequests(t *testing.T) {
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

	// Create a test user
	user := &User{
		ID:            "user-magiclink",
		Email:         "magic@example.com",
		PasswordHash:  "hash",
		EmailVerified: true,
		CreatedAt:     time.Now(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	t.Run("no tokens initially", func(t *testing.T) {
		count, err := db.CountRecentMagicLinkRequests(user.ID, time.Now().Add(-15*time.Minute))
		if err != nil {
			t.Fatalf("Failed to count tokens: %v", err)
		}
		if count != 0 {
			t.Errorf("Expected 0 tokens, got %d", count)
		}
	})

	t.Run("counts requests including used and expired tokens", func(t *testing.T) {
		// Insert tokens directly to test counting (bypasses CreateMagicLinkToken's delete behavior)
		// Token 1: Active and unused
		db.conn.Exec(`
			INSERT INTO magic_link_tokens (id, user_id, token_hash, used, created_at, expires_at)
			VALUES (?, ?, ?, 0, ?, ?)
		`, "token-1", user.ID, "hash1", time.Now(), time.Now().Add(15*time.Minute))

		// Token 2: Used (still counted for rate limiting)
		db.conn.Exec(`
			INSERT INTO magic_link_tokens (id, user_id, token_hash, used, created_at, expires_at)
			VALUES (?, ?, ?, 1, ?, ?)
		`, "token-2", user.ID, "hash2", time.Now(), time.Now().Add(15*time.Minute))

		// Token 3: Expired (still counted for rate limiting)
		db.conn.Exec(`
			INSERT INTO magic_link_tokens (id, user_id, token_hash, used, created_at, expires_at)
			VALUES (?, ?, ?, 0, ?, ?)
		`, "token-3", user.ID, "hash3", time.Now(), time.Now().Add(-1*time.Minute))

		// All 3 should be counted for rate limiting
		count, err := db.CountRecentMagicLinkRequests(user.ID, time.Now().Add(-15*time.Minute))
		if err != nil {
			t.Fatalf("Failed to count tokens: %v", err)
		}
		if count != 3 {
			t.Errorf("Expected 3 tokens (including used and expired), got %d", count)
		}
	})

	t.Run("excludes tokens before since time", func(t *testing.T) {
		// Create a token that's too old (created before the window)
		db.conn.Exec(`
			INSERT INTO magic_link_tokens (id, user_id, token_hash, used, created_at, expires_at)
			VALUES (?, ?, ?, 0, ?, ?)
		`, "old-token", user.ID, "old-hash", time.Now().Add(-30*time.Minute), time.Now().Add(15*time.Minute))

		// Count should exclude the old token
		count, err := db.CountRecentMagicLinkRequests(user.ID, time.Now().Add(-15*time.Minute))
		if err != nil {
			t.Fatalf("Failed to count tokens: %v", err)
		}
		if count != 3 {
			t.Errorf("Expected 3 tokens (old token not counted), got %d", count)
		}
	})

	t.Run("rate limit blocks after max requests", func(t *testing.T) {
		// Simulate behavior: after 3 requests, rate limit should kick in
		// The check happens before creating the token, so 3 tokens = rate limited
		const maxRequests = 3
		count, err := db.CountRecentMagicLinkRequests(user.ID, time.Now().Add(-15*time.Minute))
		if err != nil {
			t.Fatalf("Failed to count tokens: %v", err)
		}
		if count < maxRequests {
			t.Errorf("Expected at least %d requests to trigger rate limit, got %d", maxRequests, count)
		}
	})
}

func TestQueryTimeout(t *testing.T) {
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

	t.Run("default timeout is 30 seconds", func(t *testing.T) {
		timeout := db.GetQueryTimeout()
		if timeout != DefaultQueryTimeout {
			t.Errorf("Expected default timeout %v, got %v", DefaultQueryTimeout, timeout)
		}
		if timeout != 30*time.Second {
			t.Errorf("Expected 30s timeout, got %v", timeout)
		}
	})

	t.Run("can set custom timeout", func(t *testing.T) {
		db.SetQueryTimeout(10 * time.Second)
		timeout := db.GetQueryTimeout()
		if timeout != 10*time.Second {
			t.Errorf("Expected 10s timeout, got %v", timeout)
		}
		// Reset to default
		db.SetQueryTimeout(DefaultQueryTimeout)
	})

	t.Run("query works with context timeout", func(t *testing.T) {
		// Create a video
		video := &Video{
			ID:          "timeout-test-video",
			Filename:    "test.mp4",
			Size:        1024,
			ContentType: "video/mp4",
			FilePath:    "/uploads/test.mp4",
			CreatedAt:   time.Now(),
		}
		if err := db.CreateVideo(video); err != nil {
			t.Fatalf("Failed to create video: %v", err)
		}

		// Retrieve it - this uses QueryRowContext internally
		retrieved, err := db.GetVideo("timeout-test-video")
		if err != nil {
			t.Fatalf("Failed to get video: %v", err)
		}
		if retrieved == nil {
			t.Fatal("Expected video to be found")
		}
		if retrieved.Filename != "test.mp4" {
			t.Errorf("Expected filename 'test.mp4', got '%s'", retrieved.Filename)
		}
	})
}
