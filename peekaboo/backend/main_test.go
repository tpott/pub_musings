package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHasAgeFiles(t *testing.T) {
	t.Run("returns false for empty directory", func(t *testing.T) {
		dir := t.TempDir()
		if hasAgeFiles(dir) {
			t.Error("expected false for empty directory")
		}
	})

	t.Run("returns false for directory with only plain files", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "photo.jpg"), []byte("img"), 0644); err != nil {
			t.Fatal(err)
		}
		if hasAgeFiles(dir) {
			t.Error("expected false when no .age files present")
		}
	})

	t.Run("returns true for directory with .age files", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "photo.jpg.age"), []byte("enc"), 0644); err != nil {
			t.Fatal(err)
		}
		if !hasAgeFiles(dir) {
			t.Error("expected true when .age files present")
		}
	})

	t.Run("returns true for nested .age files", func(t *testing.T) {
		dir := t.TempDir()
		nested := filepath.Join(dir, "cat", "set1")
		if err := os.MkdirAll(nested, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(nested, "photo.jpg.age"), []byte("enc"), 0644); err != nil {
			t.Fatal(err)
		}
		if !hasAgeFiles(dir) {
			t.Error("expected true when nested .age files present")
		}
	})

	t.Run("returns false for nonexistent directory", func(t *testing.T) {
		if hasAgeFiles("/nonexistent/path/unlikely/to/exist") {
			t.Error("expected false for nonexistent directory")
		}
	})
}
