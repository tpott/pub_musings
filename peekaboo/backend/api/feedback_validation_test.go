package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Feedback handler tests for input validation and edge cases.

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
