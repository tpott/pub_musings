package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/tpott/pub_musings/peekaboo/backend/logging"
	"github.com/tpott/pub_musings/peekaboo/backend/tts"
)

// maxSpeakBodySize is the maximum allowed request body size for /api/speak.
// 2KB is sufficient for any valid speak request (text max 256 chars + JSON overhead).
const maxSpeakBodySize = 2 << 10 // 2KB

// SpeakRequest is the incoming request to /api/speak.
type SpeakRequest struct {
	Text string `json:"text"`
}

// SpeakResponse is the error response from /api/speak.
// On success, audio/wav is returned directly instead of JSON.
type SpeakResponse struct {
	Error string `json:"error,omitempty"`
}

// SpeakHandler handles POST /api/speak requests.
type SpeakHandler struct {
	provider tts.Provider
}

// NewSpeakHandler creates a new SpeakHandler with a TTS provider.
func NewSpeakHandler(provider tts.Provider) *SpeakHandler {
	return &SpeakHandler{provider: provider}
}

// ServeHTTP handles the text-to-speech request.
// On success, returns audio/wav data directly.
// On error, returns JSON with error message.
func (h *SpeakHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		slog.Info("speak request completed",
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id", logging.GetRequestID(r.Context()))
	}()

	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit request body size to prevent DoS
	r.Body = http.MaxBytesReader(w, r.Body, maxSpeakBodySize)

	// Parse JSON body
	var req SpeakRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// MaxBytesReader returns a specific error type when limit exceeded
		if err.Error() == "http: request body too large" {
			writeJSON(w, http.StatusRequestEntityTooLarge, SpeakResponse{Error: "request body too large"})
			return
		}
		writeJSON(w, http.StatusBadRequest, SpeakResponse{Error: "invalid JSON body"})
		return
	}

	if req.Text == "" {
		writeJSON(w, http.StatusBadRequest, SpeakResponse{Error: "missing text field"})
		return
	}

	// Limit text length to prevent abuse (256 chars is plenty for "Here is a cat!")
	const maxTextLength = 256
	if len(req.Text) > maxTextLength {
		writeJSON(w, http.StatusBadRequest, SpeakResponse{Error: "text too long (max 256 characters)"})
		return
	}

	// Synthesize speech using TTS provider
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	audio, err := h.provider.Synthesize(ctx, req.Text)
	if err != nil {
		slog.Error("speech synthesis failed",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, SpeakResponse{Error: "speech synthesis failed"})
		return
	}

	// Return WAV audio directly
	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Content-Length", strconv.Itoa(len(audio)))
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(audio); err != nil {
		slog.Debug("failed to write audio response", "error", err)
	}
}
