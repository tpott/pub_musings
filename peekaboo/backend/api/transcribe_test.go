package api

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTranscribeHandler_Success(t *testing.T) {
	// Create mock whisper-server
	mockWhisper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.URL.Path != "/inference" {
			t.Errorf("Expected /inference, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}

		// Parse multipart form
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Errorf("Failed to parse form: %v", err)
		}

		// Verify required fields
		if r.FormValue("response_format") != "verbose_json" {
			t.Errorf("Expected response_format=verbose_json, got %s", r.FormValue("response_format"))
		}

		// Return mock response
		resp := WhisperResponse{
			Task:     "transcribe",
			Language: "en",
			Duration: 2.5,
			Text:     "show me a cat",
			Segments: []WhisperSegment{
				{ID: 0, Start: 0.0, End: 2.5, Text: "show me a cat"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockWhisper.Close()

	// Create handler with mock server
	handler := NewTranscribeHandler(mockWhisper.URL)

	// Create test request
	req := createMultipartRequest(t, "audio", "test.webm", []byte("fake audio data"))
	rr := httptest.NewRecorder()

	// Make request
	handler.ServeHTTP(rr, req)

	// Check response
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp TranscribeResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Text != "show me a cat" {
		t.Errorf("Expected text 'show me a cat', got %q", resp.Text)
	}
	if resp.Error != "" {
		t.Errorf("Expected no error, got %q", resp.Error)
	}
}

func TestTranscribeHandler_MissingAudio(t *testing.T) {
	handler := NewTranscribeHandler("http://localhost:9999")

	// Create request without audio file
	req := httptest.NewRequest(http.MethodPost, "/api/transcribe", nil)
	req.Header.Set("Content-Type", "multipart/form-data")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

func TestTranscribeHandler_WrongMethod(t *testing.T) {
	handler := NewTranscribeHandler("http://localhost:9999")

	req := httptest.NewRequest(http.MethodGet, "/api/transcribe", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", rr.Code)
	}
}

func TestTranscribeHandler_WhisperServerError(t *testing.T) {
	// Create mock whisper-server that returns error
	mockWhisper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer mockWhisper.Close()

	handler := NewTranscribeHandler(mockWhisper.URL)

	req := createMultipartRequest(t, "audio", "test.webm", []byte("fake audio"))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", rr.Code)
	}

	var resp TranscribeResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Error should be generic, not exposing internal details
	if resp.Error != "transcription failed" {
		t.Errorf("Expected generic error 'transcription failed', got %q", resp.Error)
	}
}

func TestTranscribeHandler_EmptyTranscript(t *testing.T) {
	// Create mock whisper-server that returns empty text
	mockWhisper := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := WhisperResponse{
			Task:     "transcribe",
			Language: "en",
			Duration: 0.5,
			Text:     "",
			Segments: []WhisperSegment{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockWhisper.Close()

	handler := NewTranscribeHandler(mockWhisper.URL)

	req := createMultipartRequest(t, "audio", "silence.webm", []byte("silent audio"))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var resp TranscribeResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Text != "" {
		t.Errorf("Expected empty text, got %q", resp.Text)
	}
}

// createMultipartRequest creates a test HTTP request with a multipart form containing a file.
func createMultipartRequest(t *testing.T, fieldName, filename string, content []byte) *http.Request {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatalf("CreateFormFile failed: %v", err)
	}
	if _, err := io.Copy(part, bytes.NewReader(content)); err != nil {
		t.Fatalf("Copy failed: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/transcribe", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}
