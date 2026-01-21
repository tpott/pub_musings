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
