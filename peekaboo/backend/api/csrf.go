package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/tpott/pub_musings/peekaboo/backend/auth"
	"github.com/tpott/pub_musings/peekaboo/backend/db"
	"github.com/tpott/pub_musings/peekaboo/backend/logging"
)

// csrfResponse is the response from GET /api/auth/csrf.
type csrfResponse struct {
	Token string `json:"token,omitempty"`
	Error string `json:"error,omitempty"`
}

// HandleCSRF handles GET /api/auth/csrf.
// Returns an HMAC-SHA256 CSRF token derived from the session token.
// Requires authentication.
func (h *AuthHandler) HandleCSRF(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionToken := extractSessionToken(r)
	if sessionToken == "" {
		writeJSON(w, http.StatusUnauthorized, csrfResponse{Error: "not authenticated"})
		return
	}

	session, err := h.DB.GetSessionByToken(sessionToken)
	if err != nil {
		slog.Error("csrf: failed to look up session",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, csrfResponse{Error: "internal error"})
		return
	}

	if session == nil || time.Now().UTC().After(session.ExpiresAt) {
		writeJSON(w, http.StatusUnauthorized, csrfResponse{Error: "not authenticated"})
		return
	}

	csrfToken := auth.GenerateCSRF(sessionToken, h.CSRFSecret)
	writeJSON(w, http.StatusOK, csrfResponse{Token: csrfToken})
}

// csrfExemptPaths are endpoints that don't require CSRF validation.
// These are either unauthenticated endpoints or endpoints that accept
// credentials directly (login, register).
var csrfExemptPaths = map[string]bool{
	"/api/auth/login":               true,
	"/api/auth/register":            true,
	"/api/auth/resend-verification": true,
	"/api/auth/magic-link":          true,
}

// CSRFMiddleware validates CSRF tokens on state-changing requests
// (POST, PUT, DELETE, PATCH). GET, HEAD, and OPTIONS are safe methods
// and don't require CSRF protection. Exempt paths (login, register, etc.)
// are skipped.
func CSRFMiddleware(next http.Handler, csrfSecret []byte, database *db.DB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Safe methods don't need CSRF protection
		if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		// Check exempt paths
		if csrfExemptPaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}

		// Extract session token
		sessionToken := extractSessionToken(r)
		if sessionToken == "" {
			// No session = no CSRF needed (request will fail auth elsewhere)
			next.ServeHTTP(w, r)
			return
		}

		// Verify session exists and is not expired — fail closed on DB errors
		session, err := database.GetSessionByToken(sessionToken)
		if err != nil {
			slog.Error("csrf: failed to look up session",
				"error", err,
				"request_id", logging.GetRequestID(r.Context()))
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		if session == nil || time.Now().UTC().After(session.ExpiresAt) {
			// Invalid/expired session — let the handler deal with auth
			next.ServeHTTP(w, r)
			return
		}

		// Valid session exists — require CSRF token
		csrfToken := r.Header.Get("X-CSRF-Token")
		if csrfToken == "" {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "missing CSRF token"})
			return
		}

		if !auth.ValidateCSRF(csrfToken, sessionToken, csrfSecret) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "invalid CSRF token"})
			return
		}

		next.ServeHTTP(w, r)
	})
}
