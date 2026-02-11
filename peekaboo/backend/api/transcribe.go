// Package api provides HTTP handlers for the Peekaboo backend.
package api

import (
	"bytes"
	"context"
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
	Text     string           `json:"text"`
	Error    string           `json:"error,omitempty"`
	Segments []WhisperSegment `json:"segments,omitempty"`
}

// WhisperResponse represents the full verbose_json response from whisper-server.
type WhisperResponse struct {
	Task     string           `json:"task"`
	Language string           `json:"language"`
	Duration float64          `json:"duration"`
	Text     string           `json:"text"`
	Segments []WhisperSegment `json:"segments"`
}

// WhisperSegment represents a segment from whisper-server's verbose_json output.
type WhisperSegment struct {
	ID           int           `json:"id"`
	Start        float64       `json:"start"`
	End          float64       `json:"end"`
	Text         string        `json:"text"`
	Tokens       []int         `json:"tokens,omitempty"`
	Words        []WhisperWord `json:"words,omitempty"`
	Temperature  float64       `json:"temperature,omitempty"`
	AvgLogProb   float64       `json:"avg_logprob,omitempty"`
	NoSpeechProb float64       `json:"no_speech_prob,omitempty"`
}

// WhisperWord represents a single word with timing and confidence from
// whisper's verbose_json response. Available when split_on_word=true.
type WhisperWord struct {
	Word        string  `json:"word"`
	Start       float64 `json:"start"`
	End         float64 `json:"end"`
	Probability float64 `json:"probability"`
}

// maxAudioSize is the maximum allowed audio file size (5MB).
// This limit is enforced both via Content-Length header and actual stream bytes.
const maxAudioSize = 5 << 20

// minAudioSize is the minimum allowed audio file size (1KB).
const minAudioSize = 1024

// maxErrorBodyBytes limits how much of an error response body we read
// from external services to prevent memory exhaustion.
const maxErrorBodyBytes = 10 * 1024 // 10 KB

// maxSuccessBodyBytes limits how much of a success response body we read
// from external services to prevent memory exhaustion from a rogue server.
const maxSuccessBodyBytes = 1 << 20 // 1 MB

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
	whisperResp, err := h.forwardToWhisper(r.Context(), file)
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

	writeJSON(w, http.StatusOK, TranscribeResponse{
		Text:     whisperResp.Text,
		Segments: whisperResp.Segments,
	})
}

// errStreamTooLarge is returned when the audio stream exceeds the max size limit.
var errStreamTooLarge = fmt.Errorf("audio stream exceeds maximum size of %d bytes", maxAudioSize)

// forwardToWhisper sends audio to whisper-server and returns the full response
// including word-level timing and probabilities.
// It enforces a maximum stream size of maxAudioSize bytes to prevent attacks
// that lie about Content-Length.
func (h *TranscribeHandler) forwardToWhisper(ctx context.Context, audio io.Reader) (*WhisperResponse, error) {
	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add audio file with size limit enforcement.
	// We read maxAudioSize+1 bytes to detect if the stream exceeds the limit.
	part, err := writer.CreateFormFile("file", "audio.webm")
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}

	// Use LimitReader to cap the read, but read one extra byte to detect overflow
	limitedReader := io.LimitReader(audio, maxAudioSize+1)
	n, err := io.Copy(part, limitedReader)
	if err != nil {
		return nil, fmt.Errorf("copy audio: %w", err)
	}

	// If we read more than maxAudioSize, the stream exceeded the limit
	if n > maxAudioSize {
		return nil, errStreamTooLarge
	}

	// Add required fields
	if err := writer.WriteField("response_format", "verbose_json"); err != nil {
		return nil, fmt.Errorf("write response_format: %w", err)
	}
	if err := writer.WriteField("temperature", "0"); err != nil {
		return nil, fmt.Errorf("write temperature: %w", err)
	}
	if err := writer.WriteField("language", "en"); err != nil {
		return nil, fmt.Errorf("write language: %w", err)
	}
	if err := writer.WriteField("split_on_word", "true"); err != nil {
		return nil, fmt.Errorf("write split_on_word: %w", err)
	}
	if err := writer.WriteField("vad", "true"); err != nil {
		return nil, fmt.Errorf("write vad: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close writer: %w", err)
	}

	// Send request to whisper-server
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.WhisperURL+"/inference", &buf)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := h.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		return nil, fmt.Errorf("whisper-server error: %d: %s", resp.StatusCode, string(body))
	}

	// Parse response (limit read size to prevent memory exhaustion)
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxSuccessBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var whisperResp WhisperResponse
	if err := json.Unmarshal(respBody, &whisperResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &whisperResp, nil
}

// silenceThreshold is the minimum trailing silence duration (seconds) from
// whisper's word timing to consider the user as having paused.
const silenceThreshold = 0.5

// detectTrailingSilence calculates the silence gap (in seconds) between the last
// spoken word's end time and the total audio duration. Returns 0 if the gap is
// below silenceThreshold or if timing data is unavailable.
//
// This enables VAD-based silence detection: when whisper returns word-level
// timing showing speech ended well before the audio buffer's duration,
// the user has likely paused or finished their utterance.
func detectTrailingSilence(resp *WhisperResponse) float64 {
	if resp == nil || len(resp.Segments) == 0 {
		return 0
	}

	// Find the last word across all segments
	var lastWordEnd float64
	found := false
	for i := len(resp.Segments) - 1; i >= 0; i-- {
		seg := resp.Segments[i]
		if len(seg.Words) > 0 {
			lastWordEnd = seg.Words[len(seg.Words)-1].End
			found = true
			break
		}
	}

	if !found {
		return 0
	}

	gap := resp.Duration - lastWordEnd
	if gap < silenceThreshold {
		return 0
	}
	return gap
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Debug("failed to write JSON response", "error", err)
	}
}
