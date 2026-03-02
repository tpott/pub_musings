package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tpott/pub_musings/peekaboo/backend/db"
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

	// Create mock whisper server
	mockWhisper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer mockWhisper.Close()

	handler := NewReadinessHandlerWithWhisperURL(database, mockWhisper.URL)

	t.Run("returns 200 when all dependencies are available", func(t *testing.T) {
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

		// Check details
		if resp.Details["database"] != "ok" {
			t.Errorf("expected database status 'ok', got %q", resp.Details["database"])
		}
		if resp.Details["whisper"] != "ok" {
			t.Errorf("expected whisper status 'ok', got %q", resp.Details["whisper"])
		}
	})

	t.Run("returns 503 when database is unavailable", func(t *testing.T) {
		// Create handler with closed database
		closedDB, err := db.Open(":memory:")
		if err != nil {
			t.Fatalf("open database: %v", err)
		}
		closedDB.Close() // Close it to make ping fail

		handler := NewReadinessHandlerWithWhisperURL(closedDB, mockWhisper.URL)

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

		if resp.Details["database"] != "unavailable" {
			t.Errorf("expected database detail 'unavailable', got %q", resp.Details["database"])
		}
	})

	t.Run("returns 503 when whisper-server is unavailable", func(t *testing.T) {
		// Use an invalid URL that will fail connection
		handler := NewReadinessHandlerWithWhisperURL(database, "http://127.0.0.1:1")

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

		if resp.Error != "whisper-server unavailable" {
			t.Errorf("expected error 'whisper-server unavailable', got %q", resp.Error)
		}

		if resp.Details["database"] != "ok" {
			t.Errorf("expected database detail 'ok', got %q", resp.Details["database"])
		}

		if resp.Details["whisper"] != "unavailable" {
			t.Errorf("expected whisper detail 'unavailable', got %q", resp.Details["whisper"])
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

	t.Run("returns 200 with piper when all dependencies available", func(t *testing.T) {
		// Create mock piper server
		mockPiper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer mockPiper.Close()

		handler := NewReadinessHandlerWithURLs(database, mockWhisper.URL, mockPiper.URL)

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

		// Check all details
		if resp.Details["database"] != "ok" {
			t.Errorf("expected database status 'ok', got %q", resp.Details["database"])
		}
		if resp.Details["whisper"] != "ok" {
			t.Errorf("expected whisper status 'ok', got %q", resp.Details["whisper"])
		}
		if resp.Details["piper"] != "ok" {
			t.Errorf("expected piper status 'ok', got %q", resp.Details["piper"])
		}
	})

	t.Run("returns 503 when piper-server is unavailable", func(t *testing.T) {
		// Use an invalid URL that will fail connection
		handler := NewReadinessHandlerWithURLs(database, mockWhisper.URL, "http://127.0.0.1:1")

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

		if resp.Error != "piper-server unavailable" {
			t.Errorf("expected error 'piper-server unavailable', got %q", resp.Error)
		}

		if resp.Details["database"] != "ok" {
			t.Errorf("expected database detail 'ok', got %q", resp.Details["database"])
		}

		if resp.Details["whisper"] != "ok" {
			t.Errorf("expected whisper detail 'ok', got %q", resp.Details["whisper"])
		}

		if resp.Details["piper"] != "unavailable" {
			t.Errorf("expected piper detail 'unavailable', got %q", resp.Details["piper"])
		}
	})

	t.Run("skips piper check when URL is empty", func(t *testing.T) {
		// Empty piper URL means TTS is disabled - should skip check
		handler := NewReadinessHandlerWithURLs(database, mockWhisper.URL, "")

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

		// Piper should not be in details when disabled
		if _, exists := resp.Details["piper"]; exists {
			t.Errorf("expected no piper detail when disabled, got %q", resp.Details["piper"])
		}
	})

	t.Run("returns 200 with llm when all dependencies available", func(t *testing.T) {
		mockLLM := &mockLLMProvider{healthErr: nil}
		handler := NewReadinessHandlerWithAll(database, mockWhisper.URL, "", mockLLM)

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

		if resp.Details["llm"] != "ok" {
			t.Errorf("expected llm status 'ok', got %q", resp.Details["llm"])
		}
	})

	t.Run("returns 503 when llm provider is unavailable", func(t *testing.T) {
		mockLLM := &mockLLMProvider{healthErr: errors.New("API key invalid")}
		handler := NewReadinessHandlerWithAll(database, mockWhisper.URL, "", mockLLM)

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

		if resp.Error != "llm provider unavailable" {
			t.Errorf("expected error 'llm provider unavailable', got %q", resp.Error)
		}

		if resp.Details["database"] != "ok" {
			t.Errorf("expected database detail 'ok', got %q", resp.Details["database"])
		}

		if resp.Details["whisper"] != "ok" {
			t.Errorf("expected whisper detail 'ok', got %q", resp.Details["whisper"])
		}

		if resp.Details["llm"] != "unavailable" {
			t.Errorf("expected llm detail 'unavailable', got %q", resp.Details["llm"])
		}
	})

	t.Run("skips llm check when provider is nil", func(t *testing.T) {
		// nil LLM provider means LLM check is disabled
		handler := NewReadinessHandlerWithAll(database, mockWhisper.URL, "", nil)

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

		// LLM should not be in details when disabled
		if _, exists := resp.Details["llm"]; exists {
			t.Errorf("expected no llm detail when disabled, got %q", resp.Details["llm"])
		}
	})
}
