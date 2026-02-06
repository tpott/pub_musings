package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tpott/pub_musings/peekaboo/backend/db"
)

// Feedback handler tests for successful submission and valid inputs.

func setupFeedbackTestDB(t *testing.T) *db.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	if err := database.Init(); err != nil {
		t.Fatalf("Failed to init database: %v", err)
	}

	t.Cleanup(func() {
		database.Close()
		os.Remove(dbPath)
	})

	return database
}

func TestFeedbackHandler_Success(t *testing.T) {
	database := setupFeedbackTestDB(t)
	handler := NewFeedbackHandler(database)

	rating := 5
	body := FeedbackRequest{
		Type:    "bug",
		Rating:  &rating,
		Message: "The cat picture was too small",
		Context: FeedbackContext{
			SessionID: "test-session-123",
			PageURL:   "https://peekaboo.example.com/",
		},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/feedback", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp FeedbackResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("Expected status 'ok', got %q", resp.Status)
	}
	if !strings.HasPrefix(resp.ID, "feedback_") {
		t.Errorf("Expected ID to start with 'feedback_', got %q", resp.ID)
	}
	if resp.Error != "" {
		t.Errorf("Expected no error, got %q", resp.Error)
	}
}

func TestFeedbackHandler_MinimalRequest(t *testing.T) {
	database := setupFeedbackTestDB(t)
	handler := NewFeedbackHandler(database)

	// Only required fields
	body := FeedbackRequest{
		Type:    "general",
		Message: "Great app!",
		Context: FeedbackContext{
			SessionID: "session-xyz",
			PageURL:   "/",
		},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/feedback", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFeedbackHandler_WithContext(t *testing.T) {
	database := setupFeedbackTestDB(t)
	handler := NewFeedbackHandler(database)

	conceptID := "cat"
	transcript := "show me a cat"
	userAgent := "Mozilla/5.0"
	rating := 4

	body := FeedbackRequest{
		Type:    "feature",
		Rating:  &rating,
		Message: "Would love to see elephants!",
		Context: FeedbackContext{
			SessionID:  "session-abc",
			ConceptID:  &conceptID,
			Transcript: &transcript,
			PageURL:    "/",
			UserAgent:  &userAgent,
		},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/feedback", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestFeedbackHandler_ValidRatings(t *testing.T) {
	// Test all valid rating values (1-5)
	for rating := 1; rating <= 5; rating++ {
		t.Run("valid rating "+string(rune('0'+rating)), func(t *testing.T) {
			database := setupFeedbackTestDB(t)
			handler := NewFeedbackHandler(database)

			r := rating
			body := FeedbackRequest{
				Type:    "general",
				Rating:  &r,
				Message: "Test",
				Context: FeedbackContext{
					SessionID: "test",
					PageURL:   "/",
				},
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest(http.MethodPost, "/api/feedback", bytes.NewReader(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200 for rating %d, got %d: %s", rating, w.Code, w.Body.String())
			}
		})
	}
}

func TestFeedbackHandler_AllValidTypes(t *testing.T) {
	types := []string{"general", "bug", "feature"}

	for _, feedbackType := range types {
		t.Run(feedbackType, func(t *testing.T) {
			database := setupFeedbackTestDB(t)
			handler := NewFeedbackHandler(database)

			body := FeedbackRequest{
				Type:    feedbackType,
				Message: "Test",
				Context: FeedbackContext{
					SessionID: "test",
					PageURL:   "/",
				},
			}
			jsonBody, _ := json.Marshal(body)

			req := httptest.NewRequest(http.MethodPost, "/api/feedback", bytes.NewReader(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200 for type %q, got %d: %s", feedbackType, w.Code, w.Body.String())
			}
		})
	}
}

func TestGenerateFeedbackID(t *testing.T) {
	id1, err := generateFeedbackID()
	if err != nil {
		t.Fatalf("generateFeedbackID failed: %v", err)
	}

	if !strings.HasPrefix(id1, "feedback_") {
		t.Errorf("ID should start with 'feedback_', got %q", id1)
	}

	// 12 bytes = 24 hex chars
	expectedLen := len("feedback_") + 24
	if len(id1) != expectedLen {
		t.Errorf("ID length should be %d, got %d", expectedLen, len(id1))
	}

	// Verify uniqueness
	id2, _ := generateFeedbackID()
	if id1 == id2 {
		t.Error("Generated IDs should be unique")
	}
}
