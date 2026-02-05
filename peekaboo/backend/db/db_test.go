package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetEnvInt(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		envVal     string
		defaultVal int
		want       int
	}{
		{"env not set returns default", "TEST_DB_NOT_SET", "", 42, 42},
		{"env set to valid int", "TEST_DB_VALID", "10", 42, 10},
		{"env set to invalid int returns default", "TEST_DB_INVALID", "not-a-number", 42, 42},
		{"env set to zero", "TEST_DB_ZERO", "0", 42, 0},
		{"env set to negative", "TEST_DB_NEG", "-5", 42, -5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVal != "" {
				os.Setenv(tt.key, tt.envVal)
				defer os.Unsetenv(tt.key)
			}

			got := getEnvInt(tt.key, tt.defaultVal)
			if got != tt.want {
				t.Errorf("getEnvInt(%q, %d) = %d, want %d", tt.key, tt.defaultVal, got, tt.want)
			}
		})
	}
}

func TestOpenAndClose(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	if err := db.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// File should exist
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file was not created")
	}
}

func TestSQLiteConfiguration(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Verify busy_timeout is set
	var busyTimeout int
	err = db.conn.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout)
	if err != nil {
		t.Fatalf("Failed to query busy_timeout: %v", err)
	}
	if busyTimeout != 5000 {
		t.Errorf("busy_timeout = %d, want 5000", busyTimeout)
	}

	// Verify journal_mode is WAL
	var journalMode string
	err = db.conn.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	if err != nil {
		t.Fatalf("Failed to query journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("journal_mode = %q, want %q", journalMode, "wal")
	}

	// Verify synchronous is NORMAL (1)
	var synchronous int
	err = db.conn.QueryRow("PRAGMA synchronous").Scan(&synchronous)
	if err != nil {
		t.Fatalf("Failed to query synchronous: %v", err)
	}
	if synchronous != 1 {
		t.Errorf("synchronous = %d, want 1 (NORMAL)", synchronous)
	}
}

func TestInit(t *testing.T) {
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

	// Running Init twice should not fail (idempotent)
	if err := db.Init(); err != nil {
		t.Fatalf("Second Init failed: %v", err)
	}
}

func TestIndexExists(t *testing.T) {
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

	// Verify index exists on media_sets.concept_id
	var indexName string
	err = db.conn.QueryRow(`
		SELECT name FROM sqlite_master
		WHERE type='index' AND tbl_name='media_sets' AND name='idx_media_sets_concept_id'
	`).Scan(&indexName)
	if err != nil {
		t.Fatalf("Index idx_media_sets_concept_id not found: %v", err)
	}
	if indexName != "idx_media_sets_concept_id" {
		t.Errorf("Index name = %q, want %q", indexName, "idx_media_sets_concept_id")
	}
}

func TestGetConcept(t *testing.T) {
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

	// Test all 6 MVP animals exist
	animals := []struct {
		id   string
		name string
	}{
		{"cat", "Cat"},
		{"dog", "Dog"},
		{"duck", "Duck"},
		{"pig", "Pig"},
		{"chicken", "Chicken"},
		{"cow", "Cow"},
	}

	for _, animal := range animals {
		name, err := db.GetConcept(animal.id)
		if err != nil {
			t.Errorf("GetConcept(%q) failed: %v", animal.id, err)
			continue
		}
		if name != animal.name {
			t.Errorf("GetConcept(%q) = %q, want %q", animal.id, name, animal.name)
		}
	}

	// Test non-existent concept
	name, err := db.GetConcept("elephant")
	if err != nil {
		t.Errorf("GetConcept(elephant) failed: %v", err)
	}
	if name != "" {
		t.Errorf("GetConcept(elephant) = %q, want empty string", name)
	}
}

func TestSeedMediaSetAndGetRandomMediaSet(t *testing.T) {
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

	// Seed media for cat
	err = db.SeedMediaSet("cat", "data/media/cat/set1/photo.jpg", "data/media/cat/set1/audio.mp3", "")
	if err != nil {
		t.Fatalf("SeedMediaSet failed: %v", err)
	}

	// Get random media set for cat
	ms, err := db.GetRandomMediaSet("cat")
	if err != nil {
		t.Fatalf("GetRandomMediaSet(cat) failed: %v", err)
	}
	if ms == nil {
		t.Fatal("GetRandomMediaSet(cat) returned nil, expected media set")
	}

	if ms.ConceptID != "cat" {
		t.Errorf("MediaSet.ConceptID = %q, want %q", ms.ConceptID, "cat")
	}
	if ms.PhotoPath != "data/media/cat/set1/photo.jpg" {
		t.Errorf("MediaSet.PhotoPath = %q, want %q", ms.PhotoPath, "data/media/cat/set1/photo.jpg")
	}
	if ms.AudioPath != "data/media/cat/set1/audio.mp3" {
		t.Errorf("MediaSet.AudioPath = %q, want %q", ms.AudioPath, "data/media/cat/set1/audio.mp3")
	}
}

func TestGetRandomMediaSetNoData(t *testing.T) {
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

	// Get random media set for cat (no media seeded)
	ms, err := db.GetRandomMediaSet("cat")
	if err != nil {
		t.Fatalf("GetRandomMediaSet(cat) failed: %v", err)
	}
	if ms != nil {
		t.Errorf("GetRandomMediaSet(cat) = %+v, want nil (no media seeded)", ms)
	}
}

func TestGetRandomMediaSetAllAnimals(t *testing.T) {
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

	// Seed media for all 6 animals (matching source-media.sh structure)
	animals := []string{"cat", "dog", "duck", "pig", "chicken", "cow"}
	for _, animal := range animals {
		photoPath := "data/media/" + animal + "/set1/photo.jpg"
		audioPath := "data/media/" + animal + "/set1/audio.mp3"
		err = db.SeedMediaSet(animal, photoPath, audioPath, "")
		if err != nil {
			t.Fatalf("SeedMediaSet(%s) failed: %v", animal, err)
		}
	}

	// Verify GetRandomMediaSet returns valid data for each animal
	for _, animal := range animals {
		ms, err := db.GetRandomMediaSet(animal)
		if err != nil {
			t.Errorf("GetRandomMediaSet(%s) failed: %v", animal, err)
			continue
		}
		if ms == nil {
			t.Errorf("GetRandomMediaSet(%s) returned nil", animal)
			continue
		}
		if ms.ConceptID != animal {
			t.Errorf("GetRandomMediaSet(%s).ConceptID = %q, want %q", animal, ms.ConceptID, animal)
		}
		expectedPhoto := "data/media/" + animal + "/set1/photo.jpg"
		if ms.PhotoPath != expectedPhoto {
			t.Errorf("GetRandomMediaSet(%s).PhotoPath = %q, want %q", animal, ms.PhotoPath, expectedPhoto)
		}
	}
}

func TestGetRandomMediaSetRandomness(t *testing.T) {
	// Test that GetRandomMediaSet returns different sets over multiple calls
	// when multiple sets exist for a concept.
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

	// Seed 3 media sets for cat
	sets := []struct {
		photo string
		audio string
	}{
		{"data/media/cat/set1/photo.jpg", "data/media/cat/set1/audio.mp3"},
		{"data/media/cat/set2/photo.jpg", "data/media/cat/set2/audio.mp3"},
		{"data/media/cat/set3/photo.jpg", "data/media/cat/set3/audio.mp3"},
	}

	for _, s := range sets {
		err = db.SeedMediaSet("cat", s.photo, s.audio, "")
		if err != nil {
			t.Fatalf("SeedMediaSet failed: %v", err)
		}
	}

	// Call GetRandomMediaSet 20 times and track which sets are returned
	seenSets := make(map[string]int) // photo path -> count
	numCalls := 20

	for i := 0; i < numCalls; i++ {
		ms, err := db.GetRandomMediaSet("cat")
		if err != nil {
			t.Fatalf("GetRandomMediaSet(%d) failed: %v", i, err)
		}
		if ms == nil {
			t.Fatalf("GetRandomMediaSet(%d) returned nil", i)
		}
		seenSets[ms.PhotoPath]++
	}

	// Verify multiple different sets were returned (not always the same)
	// With true randomness and 3 sets over 20 calls, we expect to see at least 2 different sets
	// (probability of seeing only 1 set is (1/3)^19 ≈ 0 for any practical purpose)
	if len(seenSets) < 2 {
		t.Errorf("Expected at least 2 different sets over %d calls, but only got %d: %v",
			numCalls, len(seenSets), seenSets)
	}

	// Log the distribution for debugging (not a failure condition)
	t.Logf("Set distribution over %d calls: %v", numCalls, seenSets)
}

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
