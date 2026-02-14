package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/tpott/pub_musings/peekaboo/backend/auth"
	"github.com/tpott/pub_musings/peekaboo/backend/db"
	"github.com/tpott/pub_musings/peekaboo/backend/logging"
)

// totpSetupResponse is the response from POST /api/auth/totp/setup.
type totpSetupResponse struct {
	Secret string `json:"secret,omitempty"`
	URI    string `json:"uri,omitempty"`
	Error  string `json:"error,omitempty"`
}

// totpEnableRequest is the incoming request to POST /api/auth/totp/enable.
type totpEnableRequest struct {
	Code     string `json:"code"`
	Password string `json:"password"`
}

// totpEnableResponse is the response from POST /api/auth/totp/enable.
type totpEnableResponse struct {
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// totpDisableRequest is the incoming request to POST /api/auth/totp/disable.
type totpDisableRequest struct {
	Password string `json:"password"`
}

// totpDisableResponse is the response from POST /api/auth/totp/disable.
type totpDisableResponse struct {
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// HandleTOTPSetup handles POST /api/auth/totp/setup.
// Generates a new TOTP secret and returns it with the otpauth URI.
// The secret is stored in the database but TOTP is not yet enabled.
// Requires authentication.
func (h *AuthHandler) HandleTOTPSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user, err := h.authenticateRequest(r)
	if err != nil {
		slog.Error("totp-setup: auth error",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, totpSetupResponse{Error: "internal error"})
		return
	}
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, totpSetupResponse{Error: "not authenticated"})
		return
	}

	if user.TOTPEnabled {
		writeJSON(w, http.StatusConflict, totpSetupResponse{Error: "TOTP is already enabled"})
		return
	}

	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		slog.Error("totp-setup: failed to generate secret",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, totpSetupResponse{Error: "internal error"})
		return
	}

	if err := h.DB.SetTOTPSecret(user.ID, secret); err != nil {
		slog.Error("totp-setup: failed to store secret",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, totpSetupResponse{Error: "internal error"})
		return
	}

	uri := auth.TOTPKeyURI(user.Email, secret)

	writeJSON(w, http.StatusOK, totpSetupResponse{
		Secret: secret,
		URI:    uri,
	})
}

// HandleTOTPEnable handles POST /api/auth/totp/enable.
// Validates the TOTP code against the stored secret, then enables TOTP.
// Requires password confirmation to prevent unauthorized enabling.
func (h *AuthHandler) HandleTOTPEnable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodySize)

	user, err := h.authenticateRequest(r)
	if err != nil {
		slog.Error("totp-enable: auth error",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, totpEnableResponse{Error: "internal error"})
		return
	}
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, totpEnableResponse{Error: "not authenticated"})
		return
	}

	var req totpEnableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if isBodyTooLargeError(err) {
			writeJSON(w, http.StatusRequestEntityTooLarge, totpEnableResponse{Error: "request body too large"})
			return
		}
		writeJSON(w, http.StatusBadRequest, totpEnableResponse{Error: "invalid JSON body"})
		return
	}

	if req.Password == "" {
		writeJSON(w, http.StatusBadRequest, totpEnableResponse{Error: "password is required"})
		return
	}

	if req.Code == "" {
		writeJSON(w, http.StatusBadRequest, totpEnableResponse{Error: "TOTP code is required"})
		return
	}

	// Verify password
	if err := auth.VerifyPassword(user.PasswordHash, req.Password); err != nil {
		writeJSON(w, http.StatusUnauthorized, totpEnableResponse{Error: "invalid password"})
		return
	}

	if user.TOTPEnabled {
		writeJSON(w, http.StatusConflict, totpEnableResponse{Error: "TOTP is already enabled"})
		return
	}

	if user.TOTPSecret == nil {
		writeJSON(w, http.StatusBadRequest, totpEnableResponse{Error: "TOTP not set up, call /api/auth/totp/setup first"})
		return
	}

	// Validate the TOTP code
	valid, err := auth.ValidateTOTP(*user.TOTPSecret, req.Code)
	if err != nil {
		slog.Error("totp-enable: validation error",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, totpEnableResponse{Error: "internal error"})
		return
	}
	if !valid {
		writeJSON(w, http.StatusUnauthorized, totpEnableResponse{Error: "invalid TOTP code"})
		return
	}

	if err := h.DB.EnableTOTP(user.ID); err != nil {
		slog.Error("totp-enable: failed to enable",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, totpEnableResponse{Error: "internal error"})
		return
	}

	writeJSON(w, http.StatusOK, totpEnableResponse{Message: "TOTP enabled successfully"})
}

// HandleTOTPDisable handles POST /api/auth/totp/disable.
// Disables TOTP and clears the stored secret.
// Requires password confirmation.
func (h *AuthHandler) HandleTOTPDisable(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodySize)

	user, err := h.authenticateRequest(r)
	if err != nil {
		slog.Error("totp-disable: auth error",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, totpDisableResponse{Error: "internal error"})
		return
	}
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, totpDisableResponse{Error: "not authenticated"})
		return
	}

	var req totpDisableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if isBodyTooLargeError(err) {
			writeJSON(w, http.StatusRequestEntityTooLarge, totpDisableResponse{Error: "request body too large"})
			return
		}
		writeJSON(w, http.StatusBadRequest, totpDisableResponse{Error: "invalid JSON body"})
		return
	}

	if req.Password == "" {
		writeJSON(w, http.StatusBadRequest, totpDisableResponse{Error: "password is required"})
		return
	}

	// Verify password
	if err := auth.VerifyPassword(user.PasswordHash, req.Password); err != nil {
		writeJSON(w, http.StatusUnauthorized, totpDisableResponse{Error: "invalid password"})
		return
	}

	if !user.TOTPEnabled {
		writeJSON(w, http.StatusConflict, totpDisableResponse{Error: "TOTP is not enabled"})
		return
	}

	if err := h.DB.DisableTOTP(user.ID); err != nil {
		slog.Error("totp-disable: failed to disable",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, totpDisableResponse{Error: "internal error"})
		return
	}

	writeJSON(w, http.StatusOK, totpDisableResponse{Message: "TOTP disabled successfully"})
}

// authenticateRequest extracts the session token, validates the session,
// and returns the associated user. Returns (nil, nil) if not authenticated.
func (h *AuthHandler) authenticateRequest(r *http.Request) (*db.User, error) {
	sessionToken := extractSessionToken(r)
	if sessionToken == "" {
		return nil, nil
	}

	session, err := h.DB.GetSessionByTokenHash(auth.HashToken(sessionToken))
	if err != nil {
		return nil, err
	}
	if session == nil || time.Now().UTC().After(session.ExpiresAt) {
		return nil, nil
	}

	user, err := h.DB.GetUserByID(session.UserID)
	if err != nil {
		return nil, err
	}

	return user, nil
}
