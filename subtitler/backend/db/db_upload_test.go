package db

import (
	"os"
	"testing"
	"time"
)

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
