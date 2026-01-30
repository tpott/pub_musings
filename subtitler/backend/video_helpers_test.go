package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tpott/subtitler/backend/db"
	"github.com/tpott/subtitler/backend/pathvalidator"
)

func TestFindVideoFile(t *testing.T) {
	// Save and restore global variables
	origDatabase := database
	origUploadDir := uploadDir
	defer func() {
		database = origDatabase
		uploadDir = origUploadDir
	}()

	t.Run("finds video from database", func(t *testing.T) {
		// Create temp directory for test files
		tempDir, err := os.MkdirTemp("", "findvideo-test-*")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Create test database
		dbPath := filepath.Join(tempDir, "test.db")
		testDB, err := db.Open(dbPath)
		if err != nil {
			t.Fatalf("Failed to open database: %v", err)
		}
		defer testDB.Close()

		// Set global variables for test
		database = testDB
		uploadDir = filepath.Join(tempDir, "uploads")
		os.MkdirAll(uploadDir, 0755)

		// Create a test video file
		videoID := "testvideo12345678901234567890ab"
		videoPath := filepath.Join(uploadDir, videoID+".mp4")
		if err := os.WriteFile(videoPath, []byte("test video content"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Add video to database
		video := &db.Video{
			ID:          videoID,
			Filename:    "test.mp4",
			Size:        100,
			ContentType: "video/mp4",
			FilePath:    videoPath,
			KeyVersion:  1,
			CreatedAt:   time.Now(),
		}
		err = testDB.CreateVideo(video)
		if err != nil {
			t.Fatalf("Failed to create video in DB: %v", err)
		}

		// Test findVideoFile
		foundPath, err := findVideoFile(videoID)
		if err != nil {
			t.Errorf("findVideoFile() returned error: %v", err)
		}
		if foundPath != videoPath {
			t.Errorf("findVideoFile() = %q, expected %q", foundPath, videoPath)
		}
	})

	t.Run("falls back to glob when database returns nil", func(t *testing.T) {
		// Create temp directory for test files
		tempDir, err := os.MkdirTemp("", "findvideo-test-*")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Create test database
		dbPath := filepath.Join(tempDir, "test.db")
		testDB, err := db.Open(dbPath)
		if err != nil {
			t.Fatalf("Failed to open database: %v", err)
		}
		defer testDB.Close()

		// Set global variables for test
		database = testDB
		uploadDir = filepath.Join(tempDir, "uploads")
		os.MkdirAll(uploadDir, 0755)

		// Create a test video file (but don't add to database)
		videoID := "testfallback1234"
		videoPath := filepath.Join(uploadDir, videoID+".webm")
		if err := os.WriteFile(videoPath, []byte("test video content"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Test findVideoFile - should find via glob fallback
		foundPath, err := findVideoFile(videoID)
		if err != nil {
			t.Errorf("findVideoFile() returned error: %v", err)
		}
		if foundPath != videoPath {
			t.Errorf("findVideoFile() = %q, expected %q", foundPath, videoPath)
		}
	})

	t.Run("returns error when video not found", func(t *testing.T) {
		// Create temp directory for test files
		tempDir, err := os.MkdirTemp("", "findvideo-test-*")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Create test database
		dbPath := filepath.Join(tempDir, "test.db")
		testDB, err := db.Open(dbPath)
		if err != nil {
			t.Fatalf("Failed to open database: %v", err)
		}
		defer testDB.Close()

		// Set global variables for test
		database = testDB
		uploadDir = filepath.Join(tempDir, "uploads")
		os.MkdirAll(uploadDir, 0755)

		// Test findVideoFile for non-existent video
		_, err = findVideoFile("nonexistent123456")
		if err == nil {
			t.Error("findVideoFile() should return error for non-existent video")
		}
		if !strings.Contains(err.Error(), "video not found") {
			t.Errorf("Error should contain 'video not found', got: %v", err)
		}
	})

	t.Run("works when database is nil", func(t *testing.T) {
		// Create temp directory for test files
		tempDir, err := os.MkdirTemp("", "findvideo-test-*")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Set global variables for test - nil database
		database = nil
		uploadDir = filepath.Join(tempDir, "uploads")
		os.MkdirAll(uploadDir, 0755)

		// Create a test video file
		videoID := "testnildb12345678"
		videoPath := filepath.Join(uploadDir, videoID+".mp4")
		if err := os.WriteFile(videoPath, []byte("test video content"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Test findVideoFile - should work via glob
		foundPath, err := findVideoFile(videoID)
		if err != nil {
			t.Errorf("findVideoFile() returned error: %v", err)
		}
		if foundPath != videoPath {
			t.Errorf("findVideoFile() = %q, expected %q", foundPath, videoPath)
		}
	})

	t.Run("rejects path traversal in upload ID", func(t *testing.T) {
		// Save and restore globals
		origDatabase := database
		origUploadDir := uploadDir
		defer func() {
			database = origDatabase
			uploadDir = origUploadDir
		}()

		database = nil
		uploadDir = "/tmp/uploads"

		testCases := []string{
			"../etc/passwd",
			"..\\windows\\system32",
			"foo/../bar",
			"id\x00inject",
		}

		for _, id := range testCases {
			_, err := findVideoFile(id)
			if err == nil {
				t.Errorf("findVideoFile(%q) should return error for path traversal attempt", id)
			}
			if err != nil && !strings.Contains(err.Error(), "invalid upload ID") {
				t.Errorf("findVideoFile(%q) error should contain 'invalid upload ID', got: %v", id, err)
			}
		}
	})

	t.Run("validates glob results against pathValidator", func(t *testing.T) {
		// Save and restore globals
		origDatabase := database
		origUploadDir := uploadDir
		origPathValidator := pathValidator
		defer func() {
			database = origDatabase
			uploadDir = origUploadDir
			pathValidator = origPathValidator
		}()

		// Create temp directory for test files
		tempDir, err := os.MkdirTemp("", "findvideo-pathval-*")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		database = nil
		uploadDir = filepath.Join(tempDir, "uploads")
		os.MkdirAll(uploadDir, 0755)

		// Create pathValidator for a DIFFERENT directory (simulates misconfiguration)
		pathValidator, err = pathvalidator.New(filepath.Join(tempDir, "other"))
		if err != nil {
			t.Fatalf("Failed to create pathValidator: %v", err)
		}

		// Create a test file in uploads dir
		videoID := "testpathval123456789012"
		videoPath := filepath.Join(uploadDir, videoID+".mp4")
		if err := os.WriteFile(videoPath, []byte("test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// findVideoFile should reject because glob result is outside validator's base dir
		_, err = findVideoFile(videoID)
		if err == nil {
			t.Error("findVideoFile() should return error when glob result fails path validation")
		}
		if err != nil && !strings.Contains(err.Error(), "path validation failed") {
			t.Errorf("Error should contain 'path validation failed', got: %v", err)
		}
	})
}

func TestGetVideoForDecryption(t *testing.T) {
	// Save and restore global database variable
	origDatabase := database
	defer func() {
		database = origDatabase
	}()

	t.Run("returns video when found", func(t *testing.T) {
		// Create temp directory for test files
		tempDir, err := os.MkdirTemp("", "getvideo-test-*")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Create test database
		dbPath := filepath.Join(tempDir, "test.db")
		testDB, err := db.Open(dbPath)
		if err != nil {
			t.Fatalf("Failed to open database: %v", err)
		}
		defer testDB.Close()

		// Set global variable for test
		database = testDB

		// Create a test video
		videoID := "testvideodecrypt123456789012345"
		video := &db.Video{
			ID:          videoID,
			Filename:    "test.mp4",
			Size:        1000,
			ContentType: "video/mp4",
			FilePath:    "/path/to/test.mp4",
			KeyVersion:  1,
			CreatedAt:   time.Now(),
		}
		err = testDB.CreateVideo(video)
		if err != nil {
			t.Fatalf("Failed to create video in DB: %v", err)
		}

		// Test getVideoForDecryption
		foundVideo, err := getVideoForDecryption(videoID)
		if err != nil {
			t.Errorf("getVideoForDecryption() returned error: %v", err)
		}
		if foundVideo == nil {
			t.Fatal("getVideoForDecryption() returned nil video")
		}
		if foundVideo.ID != videoID {
			t.Errorf("video.ID = %q, expected %q", foundVideo.ID, videoID)
		}
		if foundVideo.Filename != "test.mp4" {
			t.Errorf("video.Filename = %q, expected 'test.mp4'", foundVideo.Filename)
		}
	})

	t.Run("returns error when video not found", func(t *testing.T) {
		// Create temp directory for test files
		tempDir, err := os.MkdirTemp("", "getvideo-test-*")
		if err != nil {
			t.Fatalf("Failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Create test database
		dbPath := filepath.Join(tempDir, "test.db")
		testDB, err := db.Open(dbPath)
		if err != nil {
			t.Fatalf("Failed to open database: %v", err)
		}
		defer testDB.Close()

		// Set global variable for test
		database = testDB

		// Test getVideoForDecryption for non-existent video
		_, err = getVideoForDecryption("nonexistentvideo1")
		if err == nil {
			t.Error("getVideoForDecryption() should return error for non-existent video")
		}
		if !strings.Contains(err.Error(), "video not found") {
			t.Errorf("Error should contain 'video not found', got: %v", err)
		}
	})
}
