package api

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tpott/pub_musings/peekaboo/backend/db"
	"github.com/tpott/pub_musings/peekaboo/backend/logging"
)

// AdminHandler handles admin-only API endpoints.
type AdminHandler struct {
	DB           *db.DB
	TrustedUsers map[string]bool // user IDs allowed to access admin endpoints
}

// NewAdminHandler creates a new AdminHandler.
// trustedUsers is a comma-separated list of user IDs (from TRUSTED_USERS env var).
func NewAdminHandler(database *db.DB, trustedUsers string) *AdminHandler {
	trusted := make(map[string]bool)
	for _, id := range strings.Split(trustedUsers, ",") {
		id = strings.TrimSpace(id)
		if id != "" {
			trusted[id] = true
		}
	}
	return &AdminHandler{DB: database, TrustedUsers: trusted}
}

// adminFeedbackItem is a single feedback item in the admin response.
type adminFeedbackItem struct {
	ID         string  `json:"id"`
	Type       string  `json:"type"`
	Rating     *int    `json:"rating,omitempty"`
	Message    string  `json:"message"`
	SessionID  string  `json:"session_id"`
	UserID     *string `json:"user_id,omitempty"`
	ConceptID  *string `json:"concept_id,omitempty"`
	Transcript *string `json:"transcript,omitempty"`
	PageURL    string  `json:"page_url"`
	CreatedAt  string  `json:"created_at"`
	Status     string  `json:"status"`
}

// adminFeedbackResponse is the response from GET /api/admin/feedback.
type adminFeedbackResponse struct {
	Feedback []adminFeedbackItem `json:"feedback"`
	Total    int                 `json:"total"`
	Limit    int                 `json:"limit"`
	Error    string              `json:"error,omitempty"`
}

// HandleListFeedback handles GET /api/admin/feedback.
func (h *AdminHandler) HandleListFeedback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Authenticate
	sessionToken := extractSessionToken(r)
	if sessionToken == "" {
		writeJSON(w, http.StatusUnauthorized, adminFeedbackResponse{Error: "not authenticated"})
		return
	}

	session, err := h.DB.GetSessionByToken(sessionToken)
	if err != nil {
		slog.Error("admin feedback: failed to look up session",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, adminFeedbackResponse{Error: "internal error"})
		return
	}

	if session == nil || time.Now().UTC().After(session.ExpiresAt) {
		writeJSON(w, http.StatusUnauthorized, adminFeedbackResponse{Error: "not authenticated"})
		return
	}

	user, err := h.DB.GetUserByID(session.UserID)
	if err != nil {
		slog.Error("admin feedback: failed to look up user",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, adminFeedbackResponse{Error: "internal error"})
		return
	}

	if user == nil {
		writeJSON(w, http.StatusUnauthorized, adminFeedbackResponse{Error: "not authenticated"})
		return
	}

	// Fail-closed: deny all if no TRUSTED_USERS configured
	if len(h.TrustedUsers) == 0 {
		writeJSON(w, http.StatusForbidden, adminFeedbackResponse{Error: "TRUSTED_USERS not configured"})
		return
	}

	// Check TRUSTED_USERS authorization
	if !h.TrustedUsers[user.ID] {
		writeJSON(w, http.StatusForbidden, adminFeedbackResponse{Error: "admin access required"})
		return
	}

	// Parse query parameters
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "new"
	}

	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	after := r.URL.Query().Get("after")
	if after != "" {
		if _, err := time.Parse(time.RFC3339, after); err != nil {
			if _, err := time.Parse(time.RFC3339Nano, after); err != nil {
				writeJSON(w, http.StatusBadRequest, adminFeedbackResponse{Error: "invalid after timestamp (expected RFC3339)"})
				return
			}
		}
	}

	// Query feedback
	items, total, err := h.DB.ListFeedback(status, limit, after)
	if err != nil {
		slog.Error("admin feedback: failed to list feedback",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, adminFeedbackResponse{Error: "internal error"})
		return
	}

	// Convert to response format
	respItems := make([]adminFeedbackItem, 0, len(items))
	for _, f := range items {
		respItems = append(respItems, adminFeedbackItem{
			ID:         f.ID,
			Type:       f.FeedbackType,
			Rating:     f.Rating,
			Message:    f.Message,
			SessionID:  f.SessionID,
			UserID:     f.UserID,
			ConceptID:  f.ConceptID,
			Transcript: f.Transcript,
			PageURL:    f.PageURL,
			CreatedAt:  f.CreatedAt,
			Status:     f.Status,
		})
	}

	slog.Info("admin listed feedback",
		"user_id", user.ID,
		"status_filter", status,
		"total", total,
		"returned", len(respItems),
		"request_id", logging.GetRequestID(r.Context()))

	writeJSON(w, http.StatusOK, adminFeedbackResponse{
		Feedback: respItems,
		Total:    total,
		Limit:    limit,
	})
}
