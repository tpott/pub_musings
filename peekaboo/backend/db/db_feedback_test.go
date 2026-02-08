package db

import (
	"path/filepath"
	"testing"
)

func TestInsertFeedback(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if err := db.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Test inserting feedback with all fields
	rating := 5
	conceptID := "cat"
	transcript := "show me a cat"
	userAgent := "Mozilla/5.0"
	ipAddr := "192.168.1.1"

	f := &Feedback{
		ID:           "test-feedback-123",
		FeedbackType: "bug",
		Rating:       &rating,
		Message:      "The cat picture was too small",
		SessionID:    "session-abc",
		ConceptID:    &conceptID,
		Transcript:   &transcript,
		PageURL:      "https://peekaboo.example.com/",
		UserAgent:    &userAgent,
		IPAddress:    &ipAddr,
	}

	err = db.InsertFeedback(f)
	if err != nil {
		t.Fatalf("InsertFeedback failed: %v", err)
	}

	// Verify feedback was inserted
	var count int
	err = db.conn.QueryRow("SELECT COUNT(*) FROM feedback WHERE id = ?", f.ID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query feedback: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 feedback row, got %d", count)
	}

	// Verify data was stored correctly
	var storedType, storedMessage, storedSessionID, storedPageURL string
	var storedRating int
	err = db.conn.QueryRow(`
		SELECT feedback_type, rating, message, session_id, page_url
		FROM feedback WHERE id = ?
	`, f.ID).Scan(&storedType, &storedRating, &storedMessage, &storedSessionID, &storedPageURL)
	if err != nil {
		t.Fatalf("Failed to query feedback data: %v", err)
	}

	if storedType != "bug" {
		t.Errorf("feedback_type = %q, want %q", storedType, "bug")
	}
	if storedRating != 5 {
		t.Errorf("rating = %d, want %d", storedRating, 5)
	}
	if storedMessage != "The cat picture was too small" {
		t.Errorf("message = %q, want %q", storedMessage, "The cat picture was too small")
	}
	if storedSessionID != "session-abc" {
		t.Errorf("session_id = %q, want %q", storedSessionID, "session-abc")
	}
}

func TestInsertFeedbackMinimalFields(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if err := db.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Test inserting feedback with only required fields (nil optional fields)
	f := &Feedback{
		ID:           "minimal-feedback-456",
		FeedbackType: "general",
		Rating:       nil, // optional
		Message:      "Great app!",
		SessionID:    "session-xyz",
		ConceptID:    nil, // optional
		Transcript:   nil, // optional
		PageURL:      "/",
		UserAgent:    nil, // optional
		IPAddress:    nil, // optional
	}

	err = db.InsertFeedback(f)
	if err != nil {
		t.Fatalf("InsertFeedback with minimal fields failed: %v", err)
	}

	// Verify feedback was inserted
	var count int
	err = db.conn.QueryRow("SELECT COUNT(*) FROM feedback WHERE id = ?", f.ID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query feedback: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 feedback row, got %d", count)
	}
}

func TestFeedbackIndexesExist(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if err := db.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Verify feedback indexes exist
	indexes := []string{"idx_feedback_created_at", "idx_feedback_status"}
	for _, idx := range indexes {
		var indexName string
		err = db.conn.QueryRow(`
			SELECT name FROM sqlite_master
			WHERE type='index' AND tbl_name='feedback' AND name=?
		`, idx).Scan(&indexName)
		if err != nil {
			t.Errorf("Index %s not found: %v", idx, err)
		}
	}
}
