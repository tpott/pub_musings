package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/trevorsmith/peekaboo/db"
)

func TestLivenessHandler(t *testing.T) {
	handler := NewLivenessHandler()

	t.Run("returns 200 OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		var resp HealthResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		if resp.Status != "ok" {
			t.Errorf("expected status 'ok', got %q", resp.Status)
		}
	})

	t.Run("rejects non-GET methods", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/health/live", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
		}
	})
}

func TestReadinessHandler(t *testing.T) {
	// Create test database
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer database.Close()

	if err := database.Init(); err != nil {
		t.Fatalf("init database: %v", err)
	}

	handler := NewReadinessHandler(database)

	t.Run("returns 200 when database is available", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		var resp HealthResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		if resp.Status != "ok" {
			t.Errorf("expected status 'ok', got %q", resp.Status)
		}
	})

	t.Run("returns 503 when database is unavailable", func(t *testing.T) {
		// Create handler with closed database
		closedDB, err := db.Open(":memory:")
		if err != nil {
			t.Fatalf("open database: %v", err)
		}
		closedDB.Close() // Close it to make ping fail

		handler := NewReadinessHandler(closedDB)

		req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusServiceUnavailable {
			t.Errorf("expected status %d, got %d", http.StatusServiceUnavailable, w.Code)
		}

		var resp HealthResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		if resp.Status != "unavailable" {
			t.Errorf("expected status 'unavailable', got %q", resp.Status)
		}
	})

	t.Run("rejects non-GET methods", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/health/ready", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
		}
	})
}
