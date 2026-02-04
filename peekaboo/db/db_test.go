package db

import (
	"os"
	"path/filepath"
	"testing"
)

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
