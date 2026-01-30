package db

import (
	"os"
	"testing"
	"time"
)

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
