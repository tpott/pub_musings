// Package api provides HTTP handlers for the Peekaboo backend.
package api

import (
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/trevorsmith/peekaboo/db"
	"github.com/trevorsmith/peekaboo/logging"
)

// validConceptPattern matches valid concept IDs (lowercase letters, numbers, underscores).
var validConceptPattern = regexp.MustCompile(`^[a-z0-9_]+$`)

// MediaResponse is the response from GET /api/media/{concept}.
type MediaResponse struct {
	PhotoURL string `json:"photo_url,omitempty"`
	AudioURL string `json:"audio_url,omitempty"`
	VideoURL string `json:"video_url,omitempty"`
	Error    string `json:"error,omitempty"`
}

// MediaHandler handles GET /api/media/{concept} requests.
type MediaHandler struct {
	DB *db.DB
}

// NewMediaHandler creates a new MediaHandler.
func NewMediaHandler(database *db.DB) *MediaHandler {
	return &MediaHandler{
		DB: database,
	}
}

// ServeHTTP handles the media lookup request.
func (h *MediaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract concept from path: /api/media/{concept}
	path := r.URL.Path
	concept := strings.TrimPrefix(path, "/api/media/")
	if concept == "" || concept == path {
		writeJSON(w, http.StatusBadRequest, MediaResponse{Error: "missing concept parameter"})
		return
	}

	// Validate concept format (prevent path traversal and injection)
	if !validConceptPattern.MatchString(concept) {
		writeJSON(w, http.StatusBadRequest, MediaResponse{Error: "invalid concept format"})
		return
	}

	// Validate concept exists
	name, err := h.DB.GetConcept(concept)
	if err != nil {
		slog.Error("media lookup failed",
			"concept", concept,
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, MediaResponse{Error: "media lookup failed"})
		return
	}
	if name == "" {
		writeJSON(w, http.StatusNotFound, MediaResponse{Error: "concept not found"})
		return
	}

	// Get random media set for concept
	mediaSet, err := h.DB.GetRandomMediaSet(concept)
	if err != nil {
		slog.Error("media set lookup failed",
			"concept", concept,
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, MediaResponse{Error: "media lookup failed"})
		return
	}
	if mediaSet == nil {
		writeJSON(w, http.StatusNotFound, MediaResponse{Error: "no media found for concept"})
		return
	}

	// Build response with URLs
	resp := MediaResponse{
		PhotoURL: "/" + mediaSet.PhotoPath,
	}
	if mediaSet.AudioPath != "" {
		resp.AudioURL = "/" + mediaSet.AudioPath
	}
	if mediaSet.VideoPath != "" {
		resp.VideoURL = "/" + mediaSet.VideoPath
	}

	writeJSON(w, http.StatusOK, resp)
}
