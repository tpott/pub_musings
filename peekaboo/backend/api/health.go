// Package api provides HTTP handlers for the Peekaboo backend.
package api

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/tpott/pub_musings/peekaboo/backend/db"
	"github.com/tpott/pub_musings/peekaboo/backend/llm"
)

// HealthResponse is the response from health check endpoints.
type HealthResponse struct {
	Status  string            `json:"status"`
	Error   string            `json:"error,omitempty"`
	Details map[string]string `json:"details,omitempty"`
}

// LivenessHandler handles GET /health/live requests.
// Returns 200 if the server is running.
type LivenessHandler struct{}

// NewLivenessHandler creates a new LivenessHandler.
func NewLivenessHandler() *LivenessHandler {
	return &LivenessHandler{}
}

// ServeHTTP handles the liveness check.
func (h *LivenessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, HealthResponse{Status: "ok"})
}

// ReadinessHandler handles GET /health/ready requests.
// Returns 200 if the server and all dependencies are ready.
type ReadinessHandler struct {
	DB          *db.DB
	WhisperURL  string
	PiperURL    string       // Optional - only checked if non-empty
	LLMProvider llm.Provider // Optional - only checked if non-nil
	Client      *http.Client
}

// NewReadinessHandler creates a new ReadinessHandler.
// Reads URLs from environment: WHISPER_SERVER_URL (required) and PIPER_SERVER_URL (optional).
func NewReadinessHandler(database *db.DB) *ReadinessHandler {
	whisperURL := os.Getenv("WHISPER_SERVER_URL")
	if whisperURL == "" {
		whisperURL = "http://127.0.0.1:8765"
	}
	piperURL := os.Getenv("PIPER_SERVER_URL") // Optional - empty means TTS disabled
	return &ReadinessHandler{
		DB:         database,
		WhisperURL: whisperURL,
		PiperURL:   piperURL,
		Client: &http.Client{
			Timeout: 5 * time.Second, // Short timeout for health checks
		},
	}
}

// NewReadinessHandlerWithLLM creates a new ReadinessHandler with an LLM provider for health checks.
// Reads URLs from environment: WHISPER_SERVER_URL (required) and PIPER_SERVER_URL (optional).
func NewReadinessHandlerWithLLM(database *db.DB, provider llm.Provider) *ReadinessHandler {
	whisperURL := os.Getenv("WHISPER_SERVER_URL")
	if whisperURL == "" {
		whisperURL = "http://127.0.0.1:8765"
	}
	piperURL := os.Getenv("PIPER_SERVER_URL") // Optional - empty means TTS disabled
	return &ReadinessHandler{
		DB:          database,
		WhisperURL:  whisperURL,
		PiperURL:    piperURL,
		LLMProvider: provider,
		Client: &http.Client{
			Timeout: 5 * time.Second, // Short timeout for health checks
		},
	}
}

// NewReadinessHandlerWithWhisperURL creates a ReadinessHandler with a custom whisper URL.
// Used for testing.
func NewReadinessHandlerWithWhisperURL(database *db.DB, whisperURL string) *ReadinessHandler {
	return &ReadinessHandler{
		DB:         database,
		WhisperURL: whisperURL,
		Client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// NewReadinessHandlerWithURLs creates a ReadinessHandler with custom whisper and piper URLs.
// Used for testing. piperURL can be empty to disable Piper health check.
func NewReadinessHandlerWithURLs(database *db.DB, whisperURL, piperURL string) *ReadinessHandler {
	return &ReadinessHandler{
		DB:         database,
		WhisperURL: whisperURL,
		PiperURL:   piperURL,
		Client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// NewReadinessHandlerWithAll creates a ReadinessHandler with all optional dependencies.
// Used for testing. piperURL can be empty to disable Piper check, llmProvider can be nil.
func NewReadinessHandlerWithAll(database *db.DB, whisperURL, piperURL string, llmProvider llm.Provider) *ReadinessHandler {
	return &ReadinessHandler{
		DB:          database,
		WhisperURL:  whisperURL,
		PiperURL:    piperURL,
		LLMProvider: llmProvider,
		Client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// ServeHTTP handles the readiness check.
func (h *ReadinessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	details := make(map[string]string)

	// Check database connectivity
	if err := h.DB.Ping(); err != nil {
		details["database"] = "unavailable"
		writeJSON(w, http.StatusServiceUnavailable, HealthResponse{
			Status:  "unavailable",
			Error:   "database unavailable",
			Details: details,
		})
		return
	}
	details["database"] = "ok"

	// Check whisper-server connectivity
	if err := h.checkWhisperServer(); err != nil {
		details["whisper"] = "unavailable"
		writeJSON(w, http.StatusServiceUnavailable, HealthResponse{
			Status:  "unavailable",
			Error:   "whisper-server unavailable",
			Details: details,
		})
		return
	}
	details["whisper"] = "ok"

	// Check Piper TTS server connectivity (optional - only if URL is configured)
	if h.PiperURL != "" {
		if err := h.checkPiperServer(); err != nil {
			details["piper"] = "unavailable"
			writeJSON(w, http.StatusServiceUnavailable, HealthResponse{
				Status:  "unavailable",
				Error:   "piper-server unavailable",
				Details: details,
			})
			return
		}
		details["piper"] = "ok"
	}

	// Check LLM provider connectivity (optional - only if provider is configured)
	if h.LLMProvider != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if err := h.LLMProvider.HealthCheck(ctx); err != nil {
			details["llm"] = "unavailable"
			writeJSON(w, http.StatusServiceUnavailable, HealthResponse{
				Status:  "unavailable",
				Error:   "llm provider unavailable",
				Details: details,
			})
			return
		}
		details["llm"] = "ok"
	}

	writeJSON(w, http.StatusOK, HealthResponse{Status: "ok", Details: details})
}

// checkWhisperServer verifies the whisper-server is reachable.
// Uses a GET request to the root endpoint for minimal overhead.
func (h *ReadinessHandler) checkWhisperServer() error {
	req, err := http.NewRequest(http.MethodGet, h.WhisperURL, nil)
	if err != nil {
		return err
	}

	resp, err := h.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Any response (even 404) means the server is reachable
	// whisper.cpp server typically returns 200 OK on root
	return nil
}

// checkPiperServer verifies the Piper TTS server is reachable.
// Uses a GET request to the root endpoint for minimal overhead.
func (h *ReadinessHandler) checkPiperServer() error {
	req, err := http.NewRequest(http.MethodGet, h.PiperURL, nil)
	if err != nil {
		return err
	}

	resp, err := h.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Any response means the server is reachable
	// Piper HTTP server may return different status codes on root
	return nil
}
