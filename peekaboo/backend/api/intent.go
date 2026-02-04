// Package api provides HTTP handlers for the Peekaboo backend.
package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/tpott/pub_musings/peekaboo/backend/llm"
	"github.com/tpott/pub_musings/peekaboo/backend/logging"
)

// IntentRequest is the incoming request to /api/intent.
type IntentRequest struct {
	Text string `json:"text"`
}

// IntentResponse is the response from /api/intent.
type IntentResponse struct {
	Subject string `json:"subject,omitempty"`
	Error   string `json:"error,omitempty"`
}

// IntentHandler handles POST /api/intent requests.
type IntentHandler struct {
	provider llm.Provider
}

// NewIntentHandler creates a new IntentHandler using a provider created from environment.
// This is a convenience constructor that creates the provider automatically.
// For testing or custom providers, use NewIntentHandlerWithProvider.
func NewIntentHandler() (*IntentHandler, error) {
	provider, err := llm.NewProviderFromEnv()
	if err != nil {
		return nil, err
	}
	return &IntentHandler{provider: provider}, nil
}

// NewIntentHandlerWithProvider creates a new IntentHandler with a specific provider.
// Use this for testing or when you need a custom provider configuration.
func NewIntentHandlerWithProvider(provider llm.Provider) *IntentHandler {
	return &IntentHandler{provider: provider}
}

// ServeHTTP handles the intent recognition request.
func (h *IntentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse JSON body
	var req IntentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, IntentResponse{Error: "invalid JSON body"})
		return
	}

	if req.Text == "" {
		writeJSON(w, http.StatusBadRequest, IntentResponse{Error: "missing text field"})
		return
	}

	// Limit text length to prevent token explosion in LLM (500 chars is ~100 tokens)
	const maxTextLength = 500
	if len(req.Text) > maxTextLength {
		writeJSON(w, http.StatusBadRequest, IntentResponse{Error: "text too long (max 500 characters)"})
		return
	}

	// Extract intent using LLM provider
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	result, err := h.provider.ExtractIntent(ctx, req.Text)
	if err != nil {
		slog.Error("intent extraction failed",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, IntentResponse{Error: "intent extraction failed"})
		return
	}

	writeJSON(w, http.StatusOK, IntentResponse{Subject: result.Subject})
}
