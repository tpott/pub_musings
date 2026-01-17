package main

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/trevor/subtitler/internal/config"
	"github.com/trevor/subtitler/internal/transcribe"
)

// TestHandleTranscribe_Integration tests the /api/transcribe endpoint with real whisper.cpp
func TestHandleTranscribe_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Initialize transcription service
	cfg := config.Load()
	transcribeService = transcribe.NewService(cfg)
	if err := transcribeService.Start(); err != nil {
		t.Fatalf("Failed to start transcription service: %v", err)
	}
	defer transcribeService.Stop()

	// Test with JFK sample audio
	jfkPath := filepath.Join(os.Getenv("HOME"), "Github/whisper.cpp/samples/jfk.wav")
	if _, err := os.Stat(jfkPath); os.IsNotExist(err) {
		t.Skipf("JFK sample audio not found at %s", jfkPath)
	}

	// Create multipart request
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	file, err := os.Open(jfkPath)
	if err != nil {
		t.Fatalf("Failed to open test file: %v", err)
	}
	defer file.Close()

	part, err := writer.CreateFormFile("file", "jfk.wav")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}

	if _, err := io.Copy(part, file); err != nil {
		t.Fatalf("Failed to copy file data: %v", err)
	}
	writer.Close()

	// Create request
	req := httptest.NewRequest("POST", "/api/transcribe", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Record response
	rr := httptest.NewRecorder()
	handleTranscribe(rr, req)

	// Check status code
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	// Parse response
	var response TranscribeResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify response structure
	if !response.Success {
		t.Errorf("Expected success=true, got false. Message: %s", response.Message)
	}

	if response.Transcript == "" {
		t.Errorf("Expected non-empty transcript")
	}

	if response.Format != "srt" {
		t.Errorf("Expected format=srt, got %s", response.Format)
	}

	if response.Filename != "jfk.wav" {
		t.Errorf("Expected filename=jfk.wav, got %s", response.Filename)
	}

	// Verify SRT format
	transcript := response.Transcript
	if !strings.Contains(transcript, "-->") {
		t.Errorf("Transcript doesn't contain SRT timestamp separator '-->'")
	}

	// Verify expected content (JFK speech should contain "ask not")
	transcriptLower := strings.ToLower(transcript)
	if !strings.Contains(transcriptLower, "ask not") {
		t.Logf("Full transcript: %s", transcript)
		t.Errorf("Expected transcript to contain 'ask not', but it doesn't")
	}

	t.Logf("Transcription completed in %.2f seconds", response.Duration)
	t.Logf("Transcript preview (first 200 chars): %s", truncate(transcript, 200))
}

// TestHandleTranscribe_MethodNotAllowed tests that non-POST requests are rejected
func TestHandleTranscribe_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/transcribe", nil)
	rr := httptest.NewRecorder()

	handleTranscribe(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", rr.Code)
	}
}

// TestHandleTranscribe_MissingFile tests that requests without a file are rejected
func TestHandleTranscribe_MissingFile(t *testing.T) {
	// Create empty multipart request
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.Close()

	req := httptest.NewRequest("POST", "/api/transcribe", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rr := httptest.NewRecorder()
	handleTranscribe(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}

	var response TranscribeResponse
	json.NewDecoder(rr.Body).Decode(&response)

	if response.Success {
		t.Errorf("Expected success=false")
	}
}

// TestHandleTranscribe_InvalidFormat tests that invalid file formats are rejected
func TestHandleTranscribe_InvalidFormat(t *testing.T) {
	// Create multipart request with invalid file type
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", "test.exe")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}
	part.Write([]byte("fake content"))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/transcribe", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rr := httptest.NewRecorder()
	handleTranscribe(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}

	var response TranscribeResponse
	json.NewDecoder(rr.Body).Decode(&response)

	if response.Success {
		t.Errorf("Expected success=false")
	}

	if !strings.Contains(response.Message, "file type") {
		t.Errorf("Expected error message about file type, got: %s", response.Message)
	}
}

// TestHandleTranscribe_ServiceNotInitialized tests behavior when service is not initialized
func TestHandleTranscribe_ServiceNotInitialized(t *testing.T) {
	// Save current service and set to nil
	savedService := transcribeService
	transcribeService = nil
	defer func() { transcribeService = savedService }()

	// Create minimal valid request
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "test.wav")
	part.Write([]byte("fake audio"))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/transcribe", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	rr := httptest.NewRecorder()
	handleTranscribe(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", rr.Code)
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
