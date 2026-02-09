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

func setupFeedbackDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	if err := db.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestListFeedback_Empty(t *testing.T) {
	db := setupFeedbackDB(t)
	items, total, err := db.ListFeedback("new", 50, "")
	if err != nil {
		t.Fatalf("ListFeedback failed: %v", err)
	}
	if total != 0 {
		t.Errorf("Expected total=0, got %d", total)
	}
	if len(items) != 0 {
		t.Errorf("Expected 0 items, got %d", len(items))
	}
}

func insertFB(t *testing.T, database *DB, id, fbType, message string) {
	t.Helper()
	if err := database.InsertFeedback(&Feedback{
		ID: id, FeedbackType: fbType,
		Message: message, SessionID: "s1", PageURL: "/",
	}); err != nil {
		t.Fatalf("InsertFeedback(%s) failed: %v", id, err)
	}
}

func TestListFeedback_ReturnsItems(t *testing.T) {
	db := setupFeedbackDB(t)
	insertFB(t, db, "fb-1", "bug", "Test")
	insertFB(t, db, "fb-2", "feature", "Test2")

	items, total, err := db.ListFeedback("new", 50, "")
	if err != nil {
		t.Fatalf("ListFeedback failed: %v", err)
	}
	if total != 2 {
		t.Errorf("Expected total=2, got %d", total)
	}
	if len(items) != 2 {
		t.Errorf("Expected 2 items, got %d", len(items))
	}

	// Verify fields on items
	for _, item := range items {
		if item.Status != "new" {
			t.Errorf("Expected status=new, got %q", item.Status)
		}
		if item.CreatedAt == "" {
			t.Error("Expected non-empty created_at")
		}
	}
}

func TestListFeedback_StatusFilter(t *testing.T) {
	db := setupFeedbackDB(t)
	insertFB(t, db, "fb-1", "bug", "Test")

	// Filter by "reviewed" — should get 0 (all default to "new")
	_, total, err := db.ListFeedback("reviewed", 50, "")
	if err != nil {
		t.Fatalf("ListFeedback failed: %v", err)
	}
	if total != 0 {
		t.Errorf("Expected total=0 for reviewed, got %d", total)
	}

	// Filter by "all" — should get 1
	_, total, err = db.ListFeedback("all", 50, "")
	if err != nil {
		t.Fatalf("ListFeedback failed: %v", err)
	}
	if total != 1 {
		t.Errorf("Expected total=1 for all, got %d", total)
	}
}

func TestListFeedback_Limit(t *testing.T) {
	db := setupFeedbackDB(t)
	for i := 0; i < 5; i++ {
		insertFB(t, db, "fb-"+string(rune('a'+i)), "general", "Test")
	}

	items, total, err := db.ListFeedback("new", 2, "")
	if err != nil {
		t.Fatalf("ListFeedback failed: %v", err)
	}
	if total != 5 {
		t.Errorf("Expected total=5, got %d", total)
	}
	if len(items) != 2 {
		t.Errorf("Expected 2 items with limit=2, got %d", len(items))
	}
}

func TestListFeedback_LimitCap(t *testing.T) {
	db := setupFeedbackDB(t)
	// Limit > 100 should be capped to 50
	_, _, err := db.ListFeedback("new", 200, "")
	if err != nil {
		t.Fatalf("ListFeedback failed: %v", err)
	}
}

func TestListFeedback_NullableFields(t *testing.T) {
	db := setupFeedbackDB(t)
	insertFB(t, db, "fb-null", "general", "Null test")

	items, _, err := db.ListFeedback("new", 50, "")
	if err != nil {
		t.Fatalf("ListFeedback failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(items))
	}
	if items[0].Rating != nil {
		t.Error("Expected nil rating")
	}
	if items[0].ConceptID != nil {
		t.Error("Expected nil concept_id")
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
