package db

import (
	"fmt"
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

func TestCreateUploadChunkIdempotent(t *testing.T) {
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

	// Create a session first
	session := &UploadSession{
		ID:          "test-session-idempotent",
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

	// First insert should succeed
	chunk := &UploadChunk{
		ID:              "chunk-1",
		UploadSessionID: "test-session-idempotent",
		ChunkIndex:      0,
		ChunkPath:       "/tmp/chunk_0.part",
		Size:            512000,
		CreatedAt:       time.Now(),
	}
	inserted, err := db.CreateUploadChunk(chunk)
	if err != nil {
		t.Fatalf("First CreateUploadChunk failed: %v", err)
	}
	if !inserted {
		t.Error("Expected first insert to succeed (inserted=true)")
	}

	// Second insert with same session+index should be ignored (not error)
	chunk2 := &UploadChunk{
		ID:              "chunk-1-duplicate",
		UploadSessionID: "test-session-idempotent",
		ChunkIndex:      0,
		ChunkPath:       "/tmp/chunk_0_dup.part",
		Size:            512000,
		CreatedAt:       time.Now(),
	}
	inserted, err = db.CreateUploadChunk(chunk2)
	if err != nil {
		t.Fatalf("Duplicate CreateUploadChunk should not error: %v", err)
	}
	if inserted {
		t.Error("Expected duplicate insert to be ignored (inserted=false)")
	}

	// Verify only the original chunk exists
	existing, err := db.GetUploadChunk("test-session-idempotent", 0)
	if err != nil {
		t.Fatalf("GetUploadChunk failed: %v", err)
	}
	if existing == nil {
		t.Fatal("Expected chunk to exist")
	}
	if existing.ID != "chunk-1" {
		t.Errorf("Expected original chunk ID 'chunk-1', got '%s'", existing.ID)
	}

	// Different chunk index should still insert
	chunk3 := &UploadChunk{
		ID:              "chunk-2",
		UploadSessionID: "test-session-idempotent",
		ChunkIndex:      1,
		ChunkPath:       "/tmp/chunk_1.part",
		Size:            512000,
		CreatedAt:       time.Now(),
	}
	inserted, err = db.CreateUploadChunk(chunk3)
	if err != nil {
		t.Fatalf("CreateUploadChunk for different index failed: %v", err)
	}
	if !inserted {
		t.Error("Expected different chunk index insert to succeed")
	}

	// Verify count
	count, err := db.CountUploadChunks("test-session-idempotent")
	if err != nil {
		t.Fatalf("CountUploadChunks failed: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2 chunks, got %d", count)
	}
}

func TestCreateUploadChunkConcurrent(t *testing.T) {
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

	// Create a session
	session := &UploadSession{
		ID:          "test-session-concurrent",
		Filename:    "test.mp4",
		ContentType: "video/mp4",
		TotalSize:   5120000,
		ChunkSize:   512000,
		TotalChunks: 10,
		Status:      "in_progress",
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}
	if err := db.CreateUploadSession(session); err != nil {
		t.Fatalf("Failed to create upload session: %v", err)
	}

	// Simulate concurrent inserts of the same chunk
	const goroutines = 5
	results := make(chan bool, goroutines)
	errors := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			chunk := &UploadChunk{
				ID:              fmt.Sprintf("concurrent-chunk-%d", idx),
				UploadSessionID: "test-session-concurrent",
				ChunkIndex:      0, // Same chunk index
				ChunkPath:       fmt.Sprintf("/tmp/concurrent_%d.part", idx),
				Size:            512000,
				CreatedAt:       time.Now(),
			}
			inserted, err := db.CreateUploadChunk(chunk)
			if err != nil {
				errors <- err
				return
			}
			errors <- nil
			results <- inserted
		}(i)
	}

	// Collect results
	insertCount := 0
	for i := 0; i < goroutines; i++ {
		if err := <-errors; err != nil {
			t.Fatalf("Concurrent CreateUploadChunk should not error: %v", err)
		}
		if <-results {
			insertCount++
		}
	}

	// Exactly one goroutine should have inserted
	if insertCount != 1 {
		t.Errorf("Expected exactly 1 successful insert, got %d", insertCount)
	}

	// Verify only 1 chunk exists
	count, err := db.CountUploadChunks("test-session-concurrent")
	if err != nil {
		t.Fatalf("CountUploadChunks failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 chunk, got %d", count)
	}
}
