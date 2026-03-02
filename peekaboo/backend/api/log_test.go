package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLogHandler_ValidDebug(t *testing.T) {
	handler := NewLogHandler()
	body, _ := json.Marshal(LogRequest{Level: "debug", Message: "test debug message"})
	req := httptest.NewRequest(http.MethodPost, "/api/log", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestLogHandler_ValidError(t *testing.T) {
	handler := NewLogHandler()
	body, _ := json.Marshal(LogRequest{Level: "error", Message: "test error message"})
	req := httptest.NewRequest(http.MethodPost, "/api/log", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestLogHandler_ValidWarn(t *testing.T) {
	handler := NewLogHandler()
	body, _ := json.Marshal(LogRequest{Level: "warn", Message: "test warn message"})
	req := httptest.NewRequest(http.MethodPost, "/api/log", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestLogHandler_ValidInfo(t *testing.T) {
	handler := NewLogHandler()
	body, _ := json.Marshal(LogRequest{Level: "info", Message: "test info message"})
	req := httptest.NewRequest(http.MethodPost, "/api/log", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestLogHandler_InvalidLevel(t *testing.T) {
	handler := NewLogHandler()
	body, _ := json.Marshal(LogRequest{Level: "trace", Message: "test message"})
	req := httptest.NewRequest(http.MethodPost, "/api/log", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestLogHandler_EmptyMessage(t *testing.T) {
	handler := NewLogHandler()
	body, _ := json.Marshal(LogRequest{Level: "info", Message: ""})
	req := httptest.NewRequest(http.MethodPost, "/api/log", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestLogHandler_WhitespaceMessage(t *testing.T) {
	handler := NewLogHandler()
	body, _ := json.Marshal(LogRequest{Level: "info", Message: "   "})
	req := httptest.NewRequest(http.MethodPost, "/api/log", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestLogHandler_InvalidJSON(t *testing.T) {
	handler := NewLogHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/log", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestLogHandler_BodyTooLarge(t *testing.T) {
	handler := NewLogHandler()
	// Create a message larger than 2KB
	bigMessage := strings.Repeat("x", 3000)
	body, _ := json.Marshal(LogRequest{Level: "info", Message: bigMessage})
	req := httptest.NewRequest(http.MethodPost, "/api/log", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413, got %d", w.Code)
	}
}

func TestLogHandler_CaseInsensitiveLevel(t *testing.T) {
	handler := NewLogHandler()
	body, _ := json.Marshal(LogRequest{Level: "ERROR", Message: "test uppercase level"})
	req := httptest.NewRequest(http.MethodPost, "/api/log", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", w.Code)
	}
}
