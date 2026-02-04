// Package api provides HTTP handlers for the Peekaboo backend.
package api

import (
	"encoding/json"
	"net/http"

	"github.com/trevorsmith/peekaboo/db"
)

// HealthResponse is the response from health check endpoints.
type HealthResponse struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
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
	DB *db.DB
}

// NewReadinessHandler creates a new ReadinessHandler.
func NewReadinessHandler(database *db.DB) *ReadinessHandler {
	return &ReadinessHandler{DB: database}
}

// ServeHTTP handles the readiness check.
func (h *ReadinessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check database connectivity
	if err := h.DB.Ping(); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(HealthResponse{Status: "unavailable", Error: "database unavailable"})
		return
	}

	writeJSON(w, http.StatusOK, HealthResponse{Status: "ok"})
}
