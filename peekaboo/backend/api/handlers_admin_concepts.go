package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/tpott/pub_musings/peekaboo/backend/logging"
)

const maxConceptBodySize = 1024 // 1KB

// adminConceptRequest is the request body for POST /api/admin/concepts.
type adminConceptRequest struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// adminConceptResponse is a single concept in the response.
type adminConceptResponse struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	MediaSetCount int    `json:"media_set_count,omitempty"`
	Error         string `json:"error,omitempty"`
}

// adminConceptsListResponse is the response from GET /api/admin/concepts.
type adminConceptsListResponse struct {
	Concepts []adminConceptResponse `json:"concepts,omitempty"`
	Error    string                 `json:"error,omitempty"`
}

// HandleCreateConcept handles POST /api/admin/concepts.
func (h *AdminHandler) HandleCreateConcept(w http.ResponseWriter, r *http.Request) {
	userID := h.authenticateAdmin(w, r)
	if userID == "" {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxConceptBodySize)

	var req adminConceptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if isBodyTooLargeError(err) {
			writeJSON(w, http.StatusRequestEntityTooLarge, adminConceptResponse{Error: "request body too large"})
			return
		}
		writeJSON(w, http.StatusBadRequest, adminConceptResponse{Error: "invalid JSON body"})
		return
	}

	// Validate ID
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" {
		writeJSON(w, http.StatusBadRequest, adminConceptResponse{Error: "id is required"})
		return
	}
	if len(req.ID) > maxConceptLength {
		writeJSON(w, http.StatusBadRequest, adminConceptResponse{Error: "id too long (max 50 characters)"})
		return
	}
	if !validConceptPattern.MatchString(req.ID) {
		writeJSON(w, http.StatusBadRequest, adminConceptResponse{Error: "id must contain only lowercase letters, numbers, and underscores"})
		return
	}

	// Validate name
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, adminConceptResponse{Error: "name is required"})
		return
	}
	if len(req.Name) > 100 {
		writeJSON(w, http.StatusBadRequest, adminConceptResponse{Error: "name too long (max 100 characters)"})
		return
	}

	// Insert concept
	if err := h.DB.InsertConcept(req.ID, req.Name); err != nil {
		if strings.Contains(err.Error(), "already exists") {
			writeJSON(w, http.StatusConflict, adminConceptResponse{Error: "concept already exists"})
			return
		}
		slog.Error("admin: failed to insert concept",
			"error", err,
			"concept_id", req.ID,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, adminConceptResponse{Error: "internal error"})
		return
	}

	slog.Info("admin created concept",
		"user_id", userID,
		"concept_id", req.ID,
		"concept_name", req.Name,
		"request_id", logging.GetRequestID(r.Context()))

	writeJSON(w, http.StatusCreated, adminConceptResponse{
		ID:   req.ID,
		Name: req.Name,
	})
}

// HandleListConcepts handles GET /api/admin/concepts.
func (h *AdminHandler) HandleListConcepts(w http.ResponseWriter, r *http.Request) {
	userID := h.authenticateAdmin(w, r)
	if userID == "" {
		return
	}

	concepts, err := h.DB.ListConceptsWithCounts()
	if err != nil {
		slog.Error("admin: failed to list concepts",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, adminConceptsListResponse{Error: "internal error"})
		return
	}

	respConcepts := make([]adminConceptResponse, 0, len(concepts))
	for _, c := range concepts {
		respConcepts = append(respConcepts, adminConceptResponse{
			ID:            c.ID,
			Name:          c.Name,
			MediaSetCount: c.MediaSetCount,
		})
	}

	slog.Info("admin listed concepts",
		"user_id", userID,
		"count", len(respConcepts),
		"request_id", logging.GetRequestID(r.Context()))

	writeJSON(w, http.StatusOK, adminConceptsListResponse{
		Concepts: respConcepts,
	})
}
