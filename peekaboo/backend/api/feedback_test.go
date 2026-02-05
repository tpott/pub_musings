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

func TestFeedbackHandler_InvalidType(t *testing.T) {
	database := setupFeedbackTestDB(t)
	handler := NewFeedbackHandler(database)

	body := FeedbackRequest{
		Type:    "invalid",
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

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var resp FeedbackResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp.Error, "invalid type") {
		t.Errorf("Expected error about invalid type, got %q", resp.Error)
	}
}

func TestFeedbackHandler_EmptyMessage(t *testing.T) {
	database := setupFeedbackTestDB(t)
	handler := NewFeedbackHandler(database)

	body := FeedbackRequest{
		Type:    "general",
		Message: "",
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

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var resp FeedbackResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp.Error, "message is required") {
		t.Errorf("Expected error about required message, got %q", resp.Error)
	}
}

func TestFeedbackHandler_WhitespaceOnlyMessage(t *testing.T) {
	database := setupFeedbackTestDB(t)
	handler := NewFeedbackHandler(database)

	body := FeedbackRequest{
		Type:    "general",
		Message: "   \n\t  ",
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

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestFeedbackHandler_MessageTooLong(t *testing.T) {
	database := setupFeedbackTestDB(t)
	handler := NewFeedbackHandler(database)

	longMessage := strings.Repeat("a", 5001) // Just over 5000 char limit

	body := FeedbackRequest{
		Type:    "general",
		Message: longMessage,
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

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected status 413, got %d", w.Code)
	}

	var resp FeedbackResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp.Error, "too long") {
		t.Errorf("Expected error about message too long, got %q", resp.Error)
	}
}

func TestFeedbackHandler_InvalidRating(t *testing.T) {
	tests := []struct {
		name   string
		rating int
	}{
		{"rating too low", 0},
		{"rating too high", 6},
		{"rating negative", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			database := setupFeedbackTestDB(t)
			handler := NewFeedbackHandler(database)

			rating := tt.rating
			body := FeedbackRequest{
				Type:    "general",
				Rating:  &rating,
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

			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected status 400, got %d", w.Code)
			}

			var resp FeedbackResponse
			json.Unmarshal(w.Body.Bytes(), &resp)
			if !strings.Contains(resp.Error, "rating must be between 1 and 5") {
				t.Errorf("Expected error about rating range, got %q", resp.Error)
			}
		})
	}
}

func TestFeedbackHandler_MissingSessionID(t *testing.T) {
	database := setupFeedbackTestDB(t)
	handler := NewFeedbackHandler(database)

	body := FeedbackRequest{
		Type:    "general",
		Message: "Test",
		Context: FeedbackContext{
			PageURL: "/",
		},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/feedback", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var resp FeedbackResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp.Error, "session_id is required") {
		t.Errorf("Expected error about session_id, got %q", resp.Error)
	}
}

func TestFeedbackHandler_MissingPageURL(t *testing.T) {
	database := setupFeedbackTestDB(t)
	handler := NewFeedbackHandler(database)

	body := FeedbackRequest{
		Type:    "general",
		Message: "Test",
		Context: FeedbackContext{
			SessionID: "test",
		},
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/feedback", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var resp FeedbackResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp.Error, "page_url is required") {
		t.Errorf("Expected error about page_url, got %q", resp.Error)
	}
}

func TestFeedbackHandler_InvalidJSON(t *testing.T) {
	database := setupFeedbackTestDB(t)
	handler := NewFeedbackHandler(database)

	req := httptest.NewRequest(http.MethodPost, "/api/feedback", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var resp FeedbackResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp.Error, "invalid JSON") {
		t.Errorf("Expected error about invalid JSON, got %q", resp.Error)
	}
}

func TestFeedbackHandler_MethodNotAllowed(t *testing.T) {
	database := setupFeedbackTestDB(t)
	handler := NewFeedbackHandler(database)

	req := httptest.NewRequest(http.MethodGet, "/api/feedback", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestFeedbackHandler_BodyTooLarge(t *testing.T) {
	database := setupFeedbackTestDB(t)
	handler := NewFeedbackHandler(database)

	// Create a valid JSON body larger than 8KB limit
	largeMessage := strings.Repeat("x", 10*1024)
	body := FeedbackRequest{
		Type:    "general",
		Message: largeMessage,
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

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected status 413, got %d: %s", w.Code, w.Body.String())
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
