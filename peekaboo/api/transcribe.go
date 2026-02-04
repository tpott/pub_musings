// Package api provides HTTP handlers for the Peekaboo backend.
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"
)

// TranscribeRequest is the incoming request to /api/transcribe.
// Audio is expected as multipart/form-data with a file field named "audio".
type TranscribeRequest struct {
	Audio io.Reader
}

// TranscribeResponse is the response from /api/transcribe.
type TranscribeResponse struct {
	Text  string `json:"text"`
	Error string `json:"error,omitempty"`
}

// WhisperResponse represents the response from whisper-server.
type WhisperResponse struct {
	Task     string           `json:"task"`
	Language string           `json:"language"`
	Duration float64          `json:"duration"`
	Text     string           `json:"text"`
	Segments []WhisperSegment `json:"segments"`
}

// WhisperSegment represents a segment from whisper-server.
type WhisperSegment struct {
	ID    int     `json:"id"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

// TranscribeHandler handles POST /api/transcribe requests.
// It accepts audio as multipart/form-data and forwards to whisper-server.
type TranscribeHandler struct {
	WhisperURL string
	Client     *http.Client
}

// NewTranscribeHandler creates a new TranscribeHandler.
// If whisperURL is empty, it reads from WHISPER_SERVER_URL env var or defaults to http://127.0.0.1:8765.
func NewTranscribeHandler(whisperURL string) *TranscribeHandler {
	if whisperURL == "" {
		whisperURL = os.Getenv("WHISPER_SERVER_URL")
		if whisperURL == "" {
			whisperURL = "http://127.0.0.1:8765"
		}
	}
	return &TranscribeHandler{
		WhisperURL: whisperURL,
		Client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// ServeHTTP handles the transcription request.
func (h *TranscribeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form (max 10MB)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, TranscribeResponse{Error: "invalid multipart form"})
		return
	}

	// Get audio file from form
	file, _, err := r.FormFile("audio")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, TranscribeResponse{Error: "missing audio file"})
		return
	}
	defer file.Close()

	// Forward to whisper-server
	text, err := h.forwardToWhisper(file)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, TranscribeResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, TranscribeResponse{Text: text})
}

// forwardToWhisper sends audio to whisper-server and returns the transcript.
func (h *TranscribeHandler) forwardToWhisper(audio io.Reader) (string, error) {
	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add audio file
	part, err := writer.CreateFormFile("file", "audio.webm")
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, audio); err != nil {
		return "", fmt.Errorf("copy audio: %w", err)
	}

	// Add required fields
	if err := writer.WriteField("response_format", "verbose_json"); err != nil {
		return "", fmt.Errorf("write response_format: %w", err)
	}
	if err := writer.WriteField("temperature", "0"); err != nil {
		return "", fmt.Errorf("write temperature: %w", err)
	}
	if err := writer.WriteField("language", "en"); err != nil {
		return "", fmt.Errorf("write language: %w", err)
	}

	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("close writer: %w", err)
	}

	// Send request to whisper-server
	req, err := http.NewRequest(http.MethodPost, h.WhisperURL+"/inference", &buf)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := h.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("whisper-server error: %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var whisperResp WhisperResponse
	if err := json.NewDecoder(resp.Body).Decode(&whisperResp); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	return whisperResp.Text, nil
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
