package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/tpott/pub_musings/peekaboo/backend/auth"
	"github.com/tpott/pub_musings/peekaboo/backend/db"
	"github.com/tpott/pub_musings/peekaboo/backend/logging"
)

// maxFeedbackBodySize is the maximum allowed request body size for /api/feedback.
// 8KB is sufficient for max message (5000 chars) + context fields + JSON overhead.
const maxFeedbackBodySize = 8 << 10 // 8KB

// maxFeedbackMessageLength is the maximum allowed message length.
const maxFeedbackMessageLength = 5000

// validFeedbackTypes are the allowed feedback type values.
var validFeedbackTypes = map[string]bool{
	"general": true,
	"bug":     true,
	"feature": true,
}

// FeedbackContext contains optional context about the user's session.
type FeedbackContext struct {
	SessionID  string  `json:"session_id"`
	ConceptID  *string `json:"concept_id,omitempty"`
	Transcript *string `json:"transcript,omitempty"`
	PageURL    string  `json:"page_url"`
	UserAgent  *string `json:"user_agent,omitempty"`
}

// FeedbackRequest is the incoming request to POST /api/feedback.
type FeedbackRequest struct {
	Type    string          `json:"type"`
	Rating  *int            `json:"rating,omitempty"`
	Message string          `json:"message"`
	Context FeedbackContext `json:"context"`
}

// FeedbackResponse is the response from POST /api/feedback.
type FeedbackResponse struct {
	ID     string `json:"id,omitempty"`
	Status string `json:"status,omitempty"`
	Error  string `json:"error,omitempty"`
}

// FeedbackHandler handles POST /api/feedback requests.
type FeedbackHandler struct {
	database *db.DB
}

// NewFeedbackHandler creates a new FeedbackHandler.
func NewFeedbackHandler(database *db.DB) *FeedbackHandler {
	return &FeedbackHandler{database: database}
}

// ServeHTTP handles the feedback submission request.
func (h *FeedbackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit request body size to prevent DoS
	r.Body = http.MaxBytesReader(w, r.Body, maxFeedbackBodySize)

	// Parse JSON body
	var req FeedbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if err.Error() == "http: request body too large" {
			writeJSON(w, http.StatusRequestEntityTooLarge, FeedbackResponse{Error: "request body too large"})
			return
		}
		writeJSON(w, http.StatusBadRequest, FeedbackResponse{Error: "invalid JSON body"})
		return
	}

	// Validate type
	if !validFeedbackTypes[req.Type] {
		writeJSON(w, http.StatusBadRequest, FeedbackResponse{Error: "invalid type: must be general, bug, or feature"})
		return
	}

	// Validate message
	msg := strings.TrimSpace(req.Message)
	if msg == "" {
		writeJSON(w, http.StatusBadRequest, FeedbackResponse{Error: "message is required"})
		return
	}
	if len(msg) > maxFeedbackMessageLength {
		writeJSON(w, http.StatusRequestEntityTooLarge, FeedbackResponse{Error: "message too long (max 5000 characters)"})
		return
	}

	// Validate rating if provided
	if req.Rating != nil {
		if *req.Rating < 1 || *req.Rating > 5 {
			writeJSON(w, http.StatusBadRequest, FeedbackResponse{Error: "rating must be between 1 and 5"})
			return
		}
	}

	// Validate session_id
	if req.Context.SessionID == "" {
		writeJSON(w, http.StatusBadRequest, FeedbackResponse{Error: "context.session_id is required"})
		return
	}

	// Validate page_url
	if req.Context.PageURL == "" {
		writeJSON(w, http.StatusBadRequest, FeedbackResponse{Error: "context.page_url is required"})
		return
	}

	// Generate feedback ID
	feedbackID, err := generateFeedbackID()
	if err != nil {
		slog.Error("failed to generate feedback ID",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, FeedbackResponse{Error: "internal error"})
		return
	}

	// Get client IP for abuse tracking
	clientIP := getClientIP(r)

	// Extract user_id from session cookie if authenticated
	var userID *string
	if token := extractSessionToken(r); token != "" && h.database != nil {
		if session, err := h.database.GetSessionByTokenHash(auth.HashToken(token)); err == nil && session != nil && time.Now().UTC().Before(session.ExpiresAt) {
			userID = &session.UserID
		}
	}

	// Create feedback record
	feedback := &db.Feedback{
		ID:           feedbackID,
		FeedbackType: req.Type,
		Rating:       req.Rating,
		Message:      msg,
		SessionID:    req.Context.SessionID,
		UserID:       userID,
		ConceptID:    req.Context.ConceptID,
		Transcript:   req.Context.Transcript,
		PageURL:      req.Context.PageURL,
		UserAgent:    req.Context.UserAgent,
		IPAddress:    &clientIP,
	}

	// Insert into database
	if err := h.database.InsertFeedback(feedback); err != nil {
		slog.Error("failed to insert feedback",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, FeedbackResponse{Error: "failed to save feedback"})
		return
	}

	slog.Info("feedback submitted",
		"feedback_id", feedbackID,
		"type", req.Type,
		"has_rating", req.Rating != nil,
		"session_id", req.Context.SessionID,
		"request_id", logging.GetRequestID(r.Context()))

	writeJSON(w, http.StatusOK, FeedbackResponse{ID: feedbackID, Status: "ok"})
}

// generateFeedbackID generates a random feedback ID in the format "feedback_<hex>".
func generateFeedbackID() (string, error) {
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "feedback_" + hex.EncodeToString(bytes), nil
}
