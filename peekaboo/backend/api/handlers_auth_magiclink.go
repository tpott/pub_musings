package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/tpott/pub_musings/peekaboo/backend/auth"
	"github.com/tpott/pub_musings/peekaboo/backend/db"
	"github.com/tpott/pub_musings/peekaboo/backend/logging"
)

// magicLinkRequest is the incoming request to POST /api/auth/magic-link.
type magicLinkRequest struct {
	Email string `json:"email"`
}

// magicLinkResponse is the response from POST /api/auth/magic-link.
type magicLinkResponse struct {
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// magicLinkVerifyResponse is the response from GET /api/auth/magic-link/verify.
type magicLinkVerifyResponse struct {
	Message string            `json:"message,omitempty"`
	User    *loginUserSummary `json:"user,omitempty"`
	Token   string            `json:"token,omitempty"`
	Error   string            `json:"error,omitempty"`
}

// HandleMagicLink handles POST /api/auth/magic-link.
// Generates a 15-minute single-use magic link token and sends it via email.
// Always returns 200 to prevent email enumeration.
func (h *AuthHandler) HandleMagicLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodySize)

	var req magicLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if err.Error() == "http: request body too large" {
			writeJSON(w, http.StatusRequestEntityTooLarge, magicLinkResponse{Error: "request body too large"})
			return
		}
		writeJSON(w, http.StatusBadRequest, magicLinkResponse{Error: "invalid JSON body"})
		return
	}

	// Always return success to prevent email enumeration
	successMsg := "If an account exists with that email, a login link has been sent."

	email := auth.NormalizeEmail(req.Email)
	if err := auth.ValidateEmail(email); err != nil {
		writeJSON(w, http.StatusOK, magicLinkResponse{Message: successMsg})
		return
	}

	user, err := h.DB.GetUserByEmail(email)
	if err != nil {
		slog.Error("magic-link: failed to look up user",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusOK, magicLinkResponse{Message: successMsg})
		return
	}

	// Only send if user exists and email is verified
	if user == nil || !user.EmailVerified {
		writeJSON(w, http.StatusOK, magicLinkResponse{Message: successMsg})
		return
	}

	// Delete existing unused magic link tokens
	if err := h.DB.DeleteUnusedMagicLinkTokens(user.ID); err != nil {
		slog.Error("magic-link: failed to delete old tokens",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
	}

	// Generate new token
	token, err := auth.GenerateToken(32)
	if err != nil {
		slog.Error("magic-link: failed to generate token",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusOK, magicLinkResponse{Message: successMsg})
		return
	}

	tokenHash := auth.HashToken(token)
	tokenID, err := auth.GenerateID()
	if err != nil {
		slog.Error("magic-link: failed to generate token ID",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusOK, magicLinkResponse{Message: successMsg})
		return
	}

	expiresAt := time.Now().UTC().Add(auth.MagicLinkTokenExpiry)
	if err := h.DB.StoreMagicLinkToken(tokenID, user.ID, tokenHash, expiresAt); err != nil {
		slog.Error("magic-link: failed to store token",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusOK, magicLinkResponse{Message: successMsg})
		return
	}

	if err := h.EmailSender.SendMagicLinkEmail(email, token); err != nil {
		slog.Error("magic-link: failed to send email",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
	}

	writeJSON(w, http.StatusOK, magicLinkResponse{Message: successMsg})
}

// HandleMagicLinkVerify handles GET /api/auth/magic-link/verify.
// Validates the magic link token, creates a session, and sets the session cookie.
// Bypasses TOTP when 2FA is enabled (magic link proves email access).
func (h *AuthHandler) HandleMagicLinkVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := r.URL.Query().Get("token")
	if token == "" {
		writeJSON(w, http.StatusBadRequest, magicLinkVerifyResponse{Error: "missing token parameter"})
		return
	}
	if len(token) > maxTokenLength {
		writeJSON(w, http.StatusBadRequest, magicLinkVerifyResponse{Error: "invalid token"})
		return
	}

	tokenHash := auth.HashToken(token)

	id, userID, expiresAt, used, err := h.DB.GetMagicLinkToken(tokenHash)
	if err != nil {
		slog.Error("magic-link-verify: failed to look up token",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, magicLinkVerifyResponse{Error: "internal error"})
		return
	}

	if id == "" {
		writeJSON(w, http.StatusBadRequest, magicLinkVerifyResponse{Error: "invalid or expired token"})
		return
	}

	if used {
		writeJSON(w, http.StatusBadRequest, magicLinkVerifyResponse{Error: "token already used"})
		return
	}

	if time.Now().UTC().After(expiresAt) {
		writeJSON(w, http.StatusBadRequest, magicLinkVerifyResponse{Error: "token expired"})
		return
	}

	// Atomically mark token as used (prevents TOCTOU race with concurrent requests)
	if err := h.DB.MarkMagicLinkTokenUsed(id); err != nil {
		if errors.Is(err, db.ErrTokenAlreadyUsed) {
			writeJSON(w, http.StatusBadRequest, magicLinkVerifyResponse{Error: "token already used"})
			return
		}
		slog.Error("magic-link-verify: failed to mark token used",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, magicLinkVerifyResponse{Error: "internal error"})
		return
	}

	// Look up user
	user, err := h.DB.GetUserByID(userID)
	if err != nil {
		slog.Error("magic-link-verify: failed to look up user",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, magicLinkVerifyResponse{Error: "internal error"})
		return
	}

	if user == nil {
		writeJSON(w, http.StatusBadRequest, magicLinkVerifyResponse{Error: "invalid or expired token"})
		return
	}

	// Create session (bypasses TOTP — magic link proves email access)
	sessionID, err := auth.GenerateID()
	if err != nil {
		slog.Error("magic-link-verify: failed to generate session ID",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, magicLinkVerifyResponse{Error: "internal error"})
		return
	}

	sessionToken, err := auth.GenerateToken(32)
	if err != nil {
		slog.Error("magic-link-verify: failed to generate session token",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, magicLinkVerifyResponse{Error: "internal error"})
		return
	}

	now := time.Now().UTC()
	session := &db.Session{
		ID:        sessionID,
		UserID:    user.ID,
		TokenHash: auth.HashToken(sessionToken),
		ExpiresAt: now.Add(auth.SessionDuration),
		CreatedAt: now,
	}

	if err := h.DB.CreateSession(session); err != nil {
		slog.Error("magic-link-verify: failed to create session",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, magicLinkVerifyResponse{Error: "internal error"})
		return
	}

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    sessionToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isHTTPSOnly(),
		MaxAge:   int(auth.SessionDuration.Seconds()),
	})

	writeJSON(w, http.StatusOK, magicLinkVerifyResponse{
		Message: "Login successful",
		User: &loginUserSummary{
			ID:          user.ID,
			Email:       user.Email,
			TOTPEnabled: user.TOTPEnabled,
		},
		Token: sessionToken,
	})
}
