// Package api provides HTTP handlers for the Peekaboo backend.
package api

import (
	"encoding/json"
	"net/http"
	"os"
	"time"

	"github.com/trevorsmith/peekaboo/db"
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
	DB         *db.DB
	WhisperURL string
	Client     *http.Client
}

// NewReadinessHandler creates a new ReadinessHandler.
// If whisperURL is empty, it reads from WHISPER_SERVER_URL env var or defaults to http://127.0.0.1:8765.
func NewReadinessHandler(database *db.DB) *ReadinessHandler {
	whisperURL := os.Getenv("WHISPER_SERVER_URL")
	if whisperURL == "" {
		whisperURL = "http://127.0.0.1:8765"
	}
	return &ReadinessHandler{
		DB:         database,
		WhisperURL: whisperURL,
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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(HealthResponse{
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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(HealthResponse{
			Status:  "unavailable",
			Error:   "whisper-server unavailable",
			Details: details,
		})
		return
	}
	details["whisper"] = "ok"

	writeJSON(w, http.StatusOK, HealthResponse{Status: "ok", Details: details})
}

// checkWhisperServer verifies the whisper-server is reachable.
// Uses a HEAD request to the root endpoint for minimal overhead.
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
