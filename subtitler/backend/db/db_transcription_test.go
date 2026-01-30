package db

import (
	"os"
	"testing"
	"time"
)

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
