package db

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestInsertConcept(t *testing.T) {
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

	// Insert a new concept
	err = db.InsertConcept("horse", "Horse")
	if err != nil {
		t.Fatalf("InsertConcept(horse) failed: %v", err)
	}

	// Verify it exists
	name, err := db.GetConcept("horse")
	if err != nil {
		t.Fatalf("GetConcept(horse) failed: %v", err)
	}
	if name != "Horse" {
		t.Errorf("GetConcept(horse) = %q, want %q", name, "Horse")
	}
}

func TestInsertConceptDuplicate(t *testing.T) {
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

	// "cat" already exists from seed data
	err = db.InsertConcept("cat", "Cat")
	if err == nil {
		t.Fatal("InsertConcept(cat) should have failed for duplicate")
	}
}

func TestInsertConceptNewDuplicate(t *testing.T) {
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

	// Insert a new concept, then try to insert it again
	err = db.InsertConcept("horse", "Horse")
	if err != nil {
		t.Fatalf("First InsertConcept(horse) failed: %v", err)
	}

	err = db.InsertConcept("horse", "Horse 2")
	if err == nil {
		t.Fatal("Second InsertConcept(horse) should have failed for duplicate")
	}
}

func TestListConceptsWithCounts(t *testing.T) {
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

	// All 6 seed concepts should exist with 0 media sets
	concepts, err := db.ListConceptsWithCounts()
	if err != nil {
		t.Fatalf("ListConceptsWithCounts failed: %v", err)
	}

	if len(concepts) != 6 {
		t.Fatalf("Expected 6 concepts, got %d", len(concepts))
	}

	// All should have 0 media sets
	for _, c := range concepts {
		if c.MediaSetCount != 0 {
			t.Errorf("Concept %q has %d media sets, want 0", c.ID, c.MediaSetCount)
		}
	}

	// Add media for cat
	err = db.SeedMediaSet("cat", "data/media/cat/set1/photo.jpg", "data/media/cat/set1/audio.mp3", "")
	if err != nil {
		t.Fatalf("SeedMediaSet failed: %v", err)
	}

	// Check again
	concepts, err = db.ListConceptsWithCounts()
	if err != nil {
		t.Fatalf("ListConceptsWithCounts after seed failed: %v", err)
	}

	for _, c := range concepts {
		if c.ID == "cat" {
			if c.MediaSetCount != 1 {
				t.Errorf("Concept cat has %d media sets, want 1", c.MediaSetCount)
			}
			if c.Name != "Cat" {
				t.Errorf("Concept cat name = %q, want %q", c.Name, "Cat")
			}
		} else {
			if c.MediaSetCount != 0 {
				t.Errorf("Concept %q has %d media sets, want 0", c.ID, c.MediaSetCount)
			}
		}
	}
}

func TestListConceptsWithCountsMultipleSets(t *testing.T) {
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

	// Add 3 sets for cat
	for i := 1; i <= 3; i++ {
		photo := "data/media/cat/set" + itoa(i) + "/photo.jpg"
		audio := "data/media/cat/set" + itoa(i) + "/audio.mp3"
		if err := db.SeedMediaSet("cat", photo, audio, ""); err != nil {
			t.Fatalf("SeedMediaSet cat set%d failed: %v", i, err)
		}
	}

	// Add 1 set for dog
	if err := db.SeedMediaSet("dog", "data/media/dog/set1/photo.jpg", "", ""); err != nil {
		t.Fatalf("SeedMediaSet dog failed: %v", err)
	}

	concepts, err := db.ListConceptsWithCounts()
	if err != nil {
		t.Fatalf("ListConceptsWithCounts failed: %v", err)
	}

	for _, c := range concepts {
		switch c.ID {
		case "cat":
			if c.MediaSetCount != 3 {
				t.Errorf("cat media_set_count = %d, want 3", c.MediaSetCount)
			}
		case "dog":
			if c.MediaSetCount != 1 {
				t.Errorf("dog media_set_count = %d, want 1", c.MediaSetCount)
			}
		default:
			if c.MediaSetCount != 0 {
				t.Errorf("%s media_set_count = %d, want 0", c.ID, c.MediaSetCount)
			}
		}
	}
}

func TestListConceptsWithCountsIncludesNewConcepts(t *testing.T) {
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

	// Add a new concept
	if err := db.InsertConcept("horse", "Horse"); err != nil {
		t.Fatalf("InsertConcept failed: %v", err)
	}

	concepts, err := db.ListConceptsWithCounts()
	if err != nil {
		t.Fatalf("ListConceptsWithCounts failed: %v", err)
	}

	if len(concepts) != 7 {
		t.Fatalf("Expected 7 concepts (6 seed + horse), got %d", len(concepts))
	}

	found := false
	for _, c := range concepts {
		if c.ID == "horse" {
			found = true
			if c.Name != "Horse" {
				t.Errorf("horse name = %q, want %q", c.Name, "Horse")
			}
			if c.MediaSetCount != 0 {
				t.Errorf("horse media_set_count = %d, want 0", c.MediaSetCount)
			}
		}
	}
	if !found {
		t.Error("horse concept not found in ListConceptsWithCounts")
	}
}

func TestNextMediaSetNumber(t *testing.T) {
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

	// No sets exist - should return 1
	num, err := db.NextMediaSetNumber("cat")
	if err != nil {
		t.Fatalf("NextMediaSetNumber(cat) failed: %v", err)
	}
	if num != 1 {
		t.Errorf("NextMediaSetNumber(cat) = %d, want 1", num)
	}

	// Add set1
	if err := db.SeedMediaSet("cat", "data/media/cat/set1/photo.jpg", "", ""); err != nil {
		t.Fatalf("SeedMediaSet set1 failed: %v", err)
	}

	num, err = db.NextMediaSetNumber("cat")
	if err != nil {
		t.Fatalf("NextMediaSetNumber(cat) after set1 failed: %v", err)
	}
	if num != 2 {
		t.Errorf("NextMediaSetNumber(cat) after set1 = %d, want 2", num)
	}

	// Add set2
	if err := db.SeedMediaSet("cat", "data/media/cat/set2/photo.jpg", "", ""); err != nil {
		t.Fatalf("SeedMediaSet set2 failed: %v", err)
	}

	num, err = db.NextMediaSetNumber("cat")
	if err != nil {
		t.Fatalf("NextMediaSetNumber(cat) after set2 failed: %v", err)
	}
	if num != 3 {
		t.Errorf("NextMediaSetNumber(cat) after set2 = %d, want 3", num)
	}
}

func TestNextMediaSetNumberNoConflict(t *testing.T) {
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

	// Add sets for cat and dog, verify they don't affect each other
	if err := db.SeedMediaSet("cat", "data/media/cat/set1/photo.jpg", "", ""); err != nil {
		t.Fatalf("SeedMediaSet cat failed: %v", err)
	}
	if err := db.SeedMediaSet("cat", "data/media/cat/set2/photo.jpg", "", ""); err != nil {
		t.Fatalf("SeedMediaSet cat set2 failed: %v", err)
	}
	if err := db.SeedMediaSet("dog", "data/media/dog/set1/photo.jpg", "", ""); err != nil {
		t.Fatalf("SeedMediaSet dog failed: %v", err)
	}

	catNum, err := db.NextMediaSetNumber("cat")
	if err != nil {
		t.Fatalf("NextMediaSetNumber(cat) failed: %v", err)
	}
	if catNum != 3 {
		t.Errorf("NextMediaSetNumber(cat) = %d, want 3", catNum)
	}

	dogNum, err := db.NextMediaSetNumber("dog")
	if err != nil {
		t.Fatalf("NextMediaSetNumber(dog) failed: %v", err)
	}
	if dogNum != 2 {
		t.Errorf("NextMediaSetNumber(dog) = %d, want 2", dogNum)
	}
}

// itoa is a simple int-to-string helper for test data.
func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
