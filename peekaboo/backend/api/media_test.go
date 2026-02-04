package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/trevorsmith/peekaboo/db"
)

func setupTestDB(t *testing.T) *db.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	if err := database.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	return database
}

func TestMediaHandler_Success(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	// Seed media set for cat
	err := database.SeedMediaSet("cat", "data/media/cat/set1/photo.jpg", "data/media/cat/set1/audio.mp3", "")
	if err != nil {
		t.Fatalf("SeedMediaSet failed: %v", err)
	}

	handler := NewMediaHandler(database)

	req := httptest.NewRequest(http.MethodGet, "/api/media/cat", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp MediaResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.PhotoURL != "/data/media/cat/set1/photo.jpg" {
		t.Errorf("Expected photo URL '/data/media/cat/set1/photo.jpg', got %q", resp.PhotoURL)
	}
	if resp.AudioURL != "/data/media/cat/set1/audio.mp3" {
		t.Errorf("Expected audio URL '/data/media/cat/set1/audio.mp3', got %q", resp.AudioURL)
	}
	if resp.Error != "" {
		t.Errorf("Expected no error, got %q", resp.Error)
	}
}

func TestMediaHandler_WithVideo(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	// Seed media set with video
	err := database.SeedMediaSet("dog", "data/media/dog/set1/photo.jpg", "data/media/dog/set1/audio.mp3", "data/media/dog/set1/video.mp4")
	if err != nil {
		t.Fatalf("SeedMediaSet failed: %v", err)
	}

	handler := NewMediaHandler(database)

	req := httptest.NewRequest(http.MethodGet, "/api/media/dog", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp MediaResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.VideoURL != "/data/media/dog/set1/video.mp4" {
		t.Errorf("Expected video URL '/data/media/dog/set1/video.mp4', got %q", resp.VideoURL)
	}
}

func TestMediaHandler_ConceptNotFound(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewMediaHandler(database)

	req := httptest.NewRequest(http.MethodGet, "/api/media/elephant", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp MediaResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.Error == "" {
		t.Error("Expected error message in response")
	}
}

func TestMediaHandler_NoMediaForConcept(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	// Concept exists (seeded by Init) but no media sets
	handler := NewMediaHandler(database)

	req := httptest.NewRequest(http.MethodGet, "/api/media/cat", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp MediaResponse
	json.NewDecoder(rr.Body).Decode(&resp)

	if resp.Error != "no media found for concept" {
		t.Errorf("Expected 'no media found for concept', got %q", resp.Error)
	}
}

func TestMediaHandler_MissingConcept(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewMediaHandler(database)

	req := httptest.NewRequest(http.MethodGet, "/api/media/", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestMediaHandler_WrongMethod(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewMediaHandler(database)

	req := httptest.NewRequest(http.MethodPost, "/api/media/cat", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", rr.Code)
	}
}

func TestMediaHandler_InvalidConceptFormat(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewMediaHandler(database)

	// Test various invalid concept formats
	invalidConcepts := []struct {
		name string
		path string
	}{
		{"path_traversal", "/api/media/../etc/passwd"},
		{"nested_traversal", "/api/media/cat/../dog"},
		{"uppercase", "/api/media/CAT"},
		{"exclamation", "/api/media/cat!"},
		{"at_symbol", "/api/media/cat@dog"},
		{"forward_slash", "/api/media/a/b"},
		{"backslash", "/api/media/a%5Cb"}, // URL-encoded backslash
		{"parent_dir", "/api/media/.."},
		{"current_dir", "/api/media/."},
		{"null_byte", "/api/media/cat%00dog"},
		{"unicode", "/api/media/%E7%8C%AB"}, // URL-encoded 猫
		{"space", "/api/media/cat%20dog"},   // URL-encoded space
	}

	for _, tc := range invalidConcepts {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("Expected status 400 for %s (%s), got %d: %s", tc.name, tc.path, rr.Code, rr.Body.String())
			}

			var resp MediaResponse
			json.NewDecoder(rr.Body).Decode(&resp)

			if resp.Error != "invalid concept format" {
				t.Errorf("Expected 'invalid concept format' error for %s, got %q", tc.name, resp.Error)
			}
		})
	}
}

func TestMediaHandler_ValidConceptFormats(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	handler := NewMediaHandler(database)

	// Test valid concept formats (should pass validation, may return 404 if not found)
	validConcepts := []string{
		"cat",         // Lowercase letters
		"cat123",      // Letters and numbers
		"my_cat",      // Underscore
		"cat_dog_123", // Mixed
		"a",           // Single character
		"1",           // Number only
		"_test",       // Leading underscore
	}

	for _, concept := range validConcepts {
		t.Run(concept, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/media/"+concept, nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			// Should not be 400 Bad Request (might be 404 Not Found which is fine)
			if rr.Code == http.StatusBadRequest {
				var resp MediaResponse
				json.NewDecoder(rr.Body).Decode(&resp)
				if resp.Error == "invalid concept format" {
					t.Errorf("Concept %q should be valid but got 'invalid concept format'", concept)
				}
			}
		})
	}
}

func TestMediaHandler_AllAnimals(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	// Seed media for all 6 animals
	animals := []string{"cat", "dog", "duck", "pig", "chicken", "cow"}
	for _, animal := range animals {
		err := database.SeedMediaSet(animal, "data/media/"+animal+"/set1/photo.jpg", "data/media/"+animal+"/set1/audio.mp3", "")
		if err != nil {
			t.Fatalf("SeedMediaSet failed for %s: %v", animal, err)
		}
	}

	handler := NewMediaHandler(database)

	for _, animal := range animals {
		t.Run(animal, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/media/"+animal, nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("Expected status 200 for %s, got %d: %s", animal, rr.Code, rr.Body.String())
			}

			var resp MediaResponse
			json.NewDecoder(rr.Body).Decode(&resp)

			expectedPhoto := "/data/media/" + animal + "/set1/photo.jpg"
			if resp.PhotoURL != expectedPhoto {
				t.Errorf("Expected photo URL %q, got %q", expectedPhoto, resp.PhotoURL)
			}

			expectedAudio := "/data/media/" + animal + "/set1/audio.mp3"
			if resp.AudioURL != expectedAudio {
				t.Errorf("Expected audio URL %q, got %q", expectedAudio, resp.AudioURL)
			}
		})
	}
}
