package db

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

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

func TestCreateVideoSizeValidation(t *testing.T) {
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

	tests := []struct {
		name      string
		size      int64
		wantErr   bool
		errString string
	}{
		{
			name:    "valid size - 1 byte",
			size:    1,
			wantErr: false,
		},
		{
			name:    "valid size - typical video",
			size:    100 * 1024 * 1024, // 100 MB
			wantErr: false,
		},
		{
			name:    "valid size - at max boundary",
			size:    MaxVideoSize,
			wantErr: false,
		},
		{
			name:      "invalid size - zero",
			size:      0,
			wantErr:   true,
			errString: "invalid video size",
		},
		{
			name:      "invalid size - negative",
			size:      -1,
			wantErr:   true,
			errString: "invalid video size",
		},
		{
			name:      "invalid size - over max",
			size:      MaxVideoSize + 1,
			wantErr:   true,
			errString: "invalid video size",
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			video := &Video{
				ID:          fmt.Sprintf("test-video-size-%d", i),
				Filename:    "test.mp4",
				Size:        tt.size,
				ContentType: "video/mp4",
				FilePath:    fmt.Sprintf("/uploads/test-video-size-%d.mp4", i),
				CreatedAt:   time.Now(),
			}

			err := db.CreateVideo(video)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error for size %d, got nil", tt.size)
				} else if tt.errString != "" && !strings.Contains(err.Error(), tt.errString) {
					t.Errorf("Error %q should contain %q", err.Error(), tt.errString)
				}
				// Also check error type
				if !errors.Is(err, ErrInvalidVideoSize) {
					t.Errorf("Error should wrap ErrInvalidVideoSize, got: %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for size %d: %v", tt.size, err)
				}
			}
		})
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

func TestGetExpiredVideosPaginated(t *testing.T) {
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

	// Create 5 expired anonymous videos
	for i := 0; i < 5; i++ {
		video := &Video{
			ID:          fmt.Sprintf("expired-%d", i),
			Filename:    fmt.Sprintf("expired-%d.mp4", i),
			Size:        1024,
			ContentType: "video/mp4",
			FilePath:    fmt.Sprintf("/uploads/expired-%d.mp4", i),
			CreatedAt:   time.Now().Add(-50*time.Hour - time.Duration(i)*time.Hour), // Stagger times for consistent ordering
		}
		if err := db.CreateVideo(video); err != nil {
			t.Fatalf("Failed to create expired video %d: %v", i, err)
		}
	}

	// Test pagination with limit
	t.Run("limit 2", func(t *testing.T) {
		expired, err := db.GetExpiredVideosPaginated(2, 0)
		if err != nil {
			t.Fatalf("Failed to get expired videos: %v", err)
		}
		if len(expired) != 2 {
			t.Errorf("Expected 2 expired videos, got %d", len(expired))
		}
	})

	t.Run("limit 3 offset 2", func(t *testing.T) {
		expired, err := db.GetExpiredVideosPaginated(3, 2)
		if err != nil {
			t.Fatalf("Failed to get expired videos: %v", err)
		}
		if len(expired) != 3 {
			t.Errorf("Expected 3 expired videos, got %d", len(expired))
		}
	})

	t.Run("offset beyond data", func(t *testing.T) {
		expired, err := db.GetExpiredVideosPaginated(10, 100)
		if err != nil {
			t.Fatalf("Failed to get expired videos: %v", err)
		}
		if len(expired) != 0 {
			t.Errorf("Expected 0 expired videos with large offset, got %d", len(expired))
		}
	})

	t.Run("no limit returns all", func(t *testing.T) {
		expired, err := db.GetExpiredVideosPaginated(0, 0)
		if err != nil {
			t.Fatalf("Failed to get expired videos: %v", err)
		}
		if len(expired) != 5 {
			t.Errorf("Expected 5 expired videos with no limit, got %d", len(expired))
		}
	})

	t.Run("ordering by created_at ASC", func(t *testing.T) {
		expired, err := db.GetExpiredVideosPaginated(5, 0)
		if err != nil {
			t.Fatalf("Failed to get expired videos: %v", err)
		}
		// Oldest first (highest -time offset, so expired-4 should be first)
		if expired[0].ID != "expired-4" {
			t.Errorf("Expected oldest video (expired-4) first, got %s", expired[0].ID)
		}
	})
}

func TestCountExpiredVideos(t *testing.T) {
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

	// Initially no expired videos
	count, err := db.CountExpiredVideos()
	if err != nil {
		t.Fatalf("Failed to count expired videos: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 expired videos initially, got %d", count)
	}

	// Create 3 expired anonymous videos
	for i := 0; i < 3; i++ {
		video := &Video{
			ID:          fmt.Sprintf("expired-count-%d", i),
			Filename:    fmt.Sprintf("expired-%d.mp4", i),
			Size:        1024,
			ContentType: "video/mp4",
			FilePath:    fmt.Sprintf("/uploads/expired-%d.mp4", i),
			CreatedAt:   time.Now().Add(-50 * time.Hour),
		}
		if err := db.CreateVideo(video); err != nil {
			t.Fatalf("Failed to create expired video %d: %v", i, err)
		}
	}

	// Create 2 fresh videos (should not be counted)
	for i := 0; i < 2; i++ {
		video := &Video{
			ID:          fmt.Sprintf("fresh-count-%d", i),
			Filename:    fmt.Sprintf("fresh-%d.mp4", i),
			Size:        1024,
			ContentType: "video/mp4",
			FilePath:    fmt.Sprintf("/uploads/fresh-%d.mp4", i),
			CreatedAt:   time.Now().Add(-1 * time.Hour),
		}
		if err := db.CreateVideo(video); err != nil {
			t.Fatalf("Failed to create fresh video %d: %v", i, err)
		}
	}

	// Should count only expired videos
	count, err = db.CountExpiredVideos()
	if err != nil {
		t.Fatalf("Failed to count expired videos: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected 3 expired videos, got %d", count)
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
