// Package api provides HTTP handlers for the Peekaboo backend.
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"time"

	"github.com/tpott/pub_musings/peekaboo/backend/logging"
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

// maxAudioSize is the maximum allowed audio file size (5MB).
// This limit is enforced both via Content-Length header and actual stream bytes.
const maxAudioSize = 5 << 20

// minAudioSize is the minimum allowed audio file size (1KB).
const minAudioSize = 1024

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
	file, header, err := r.FormFile("audio")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, TranscribeResponse{Error: "missing audio file"})
		return
	}
	defer file.Close()

	// Validate minimum file size (1KB) to reject empty or too-small files
	if header.Size < minAudioSize {
		writeJSON(w, http.StatusBadRequest, TranscribeResponse{Error: "audio file too small (minimum 1KB)"})
		return
	}

	// Validate maximum file size (5MB) via Content-Length header
	// Note: We also enforce this limit on the actual stream in forwardToWhisper
	// to prevent attackers from lying about Content-Length
	if header.Size > maxAudioSize {
		writeJSON(w, http.StatusRequestEntityTooLarge, TranscribeResponse{Error: "audio file too large (maximum 5MB)"})
		return
	}

	// Forward to whisper-server with stream size limit enforcement
	text, err := h.forwardToWhisper(file)
	if err != nil {
		// Check if stream exceeded size limit (attacker lied about Content-Length)
		if err == errStreamTooLarge {
			writeJSON(w, http.StatusRequestEntityTooLarge, TranscribeResponse{Error: "audio file too large (maximum 5MB)"})
			return
		}
		slog.Error("transcription failed",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, TranscribeResponse{Error: "transcription failed"})
		return
	}

	writeJSON(w, http.StatusOK, TranscribeResponse{Text: text})
}

// errStreamTooLarge is returned when the audio stream exceeds the max size limit.
var errStreamTooLarge = fmt.Errorf("audio stream exceeds maximum size of %d bytes", maxAudioSize)

// forwardToWhisper sends audio to whisper-server and returns the transcript.
// It enforces a maximum stream size of maxAudioSize bytes to prevent attacks
// that lie about Content-Length.
func (h *TranscribeHandler) forwardToWhisper(audio io.Reader) (string, error) {
	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add audio file with size limit enforcement.
	// We read maxAudioSize+1 bytes to detect if the stream exceeds the limit.
	part, err := writer.CreateFormFile("file", "audio.webm")
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}

	// Use LimitReader to cap the read, but read one extra byte to detect overflow
	limitedReader := io.LimitReader(audio, maxAudioSize+1)
	n, err := io.Copy(part, limitedReader)
	if err != nil {
		return "", fmt.Errorf("copy audio: %w", err)
	}

	// If we read more than maxAudioSize, the stream exceeded the limit
	if n > maxAudioSize {
		return "", errStreamTooLarge
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
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Debug("failed to write JSON response", "error", err)
	}
}
