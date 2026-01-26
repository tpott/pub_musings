// Package csrf provides CSRF protection using the double-submit cookie pattern.
// CSRF tokens are derived from session tokens using HMAC-SHA256, making them
// stateless and tied to the session.
package csrf

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"sync"
)

const (
	// HeaderName is the HTTP header name for CSRF tokens
	HeaderName = "X-CSRF-Token"

	// TokenLength is the length of the random server secret in bytes
	secretLength = 32
)

var (
	// serverSecret is the HMAC key used to derive CSRF tokens
	serverSecret []byte
	secretOnce   sync.Once
	secretErr    error
)

// getSecret returns the server secret, generating it if needed.
// The secret is either from CSRF_SECRET env var or randomly generated.
// Returns an error if the secret could not be generated.
func getSecret() ([]byte, error) {
	secretOnce.Do(func() {
		// Try to get from environment
		envSecret := os.Getenv("CSRF_SECRET")
		if envSecret != "" {
			serverSecret = []byte(envSecret)
			return
		}

		// Generate a random secret (will change on restart)
		serverSecret = make([]byte, secretLength)
		if _, err := rand.Read(serverSecret); err != nil {
			secretErr = fmt.Errorf("failed to generate CSRF secret: %w", err)
			serverSecret = nil
		}
	})
	return serverSecret, secretErr
}

// GenerateToken creates a CSRF token for the given session token.
// The token is an HMAC-SHA256 of the session token, encoded as hex.
// Returns empty string if session token is empty or secret generation fails.
func GenerateToken(sessionToken string) string {
	if sessionToken == "" {
		return ""
	}
	secret, err := getSecret()
	if err != nil || secret == nil {
		// Log error but return empty string to avoid crashing
		// This will cause CSRF validation to fail safely
		return ""
	}
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(sessionToken))
	return hex.EncodeToString(h.Sum(nil))
}

// ValidateToken checks if the provided CSRF token is valid for the session token.
func ValidateToken(sessionToken, csrfToken string) bool {
	if sessionToken == "" || csrfToken == "" {
		return false
	}
	expected := GenerateToken(sessionToken)
	// Use constant-time comparison to prevent timing attacks
	return hmac.Equal([]byte(expected), []byte(csrfToken))
}

// GetTokenFromRequest extracts the CSRF token from the request header.
func GetTokenFromRequest(r *http.Request) string {
	return r.Header.Get(HeaderName)
}

// isExemptPath returns true if the path is exempt from CSRF protection.
// Login, register, and password reset must work without CSRF token since
// the user doesn't have a session yet.
func isExemptPath(path string) bool {
	exemptPaths := []string{
		"/api/auth/login",
		"/api/auth/register",
		"/api/auth/forgot-password",
		"/api/auth/reset-password",
		"/api/health",
		"/api/log",
	}
	for _, p := range exemptPaths {
		if path == p {
			return true
		}
	}
	return false
}

// isStateChangingMethod returns true if the method can change state.
func isStateChangingMethod(method string) bool {
	return method == "POST" || method == "PUT" || method == "DELETE" || method == "PATCH"
}

// ProtectFunc wraps an http.HandlerFunc with CSRF protection.
// Requires X-CSRF-Token header for POST/PUT/DELETE/PATCH requests.
// The getSessionToken function should extract the session token from the request.
func ProtectFunc(handler http.HandlerFunc, getSessionToken func(*http.Request) string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Only check CSRF for state-changing methods
		if !isStateChangingMethod(r.Method) {
			handler(w, r)
			return
		}

		// Skip CSRF check for exempt paths
		if isExemptPath(r.URL.Path) {
			handler(w, r)
			return
		}

		// Get session token
		sessionToken := getSessionToken(r)
		if sessionToken == "" {
			// No session = no CSRF protection needed (will fail auth anyway)
			handler(w, r)
			return
		}

		// Validate CSRF token
		csrfToken := GetTokenFromRequest(r)
		if !ValidateToken(sessionToken, csrfToken) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error":"Invalid or missing CSRF token"}`))
			return
		}

		handler(w, r)
	}
}

// Middleware returns HTTP middleware that applies CSRF protection.
// The getSessionToken function should extract the session token from the request.
func Middleware(getSessionToken func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only check CSRF for state-changing methods
			if !isStateChangingMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}

			// Skip CSRF check for exempt paths
			if isExemptPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// Get session token
			sessionToken := getSessionToken(r)
			if sessionToken == "" {
				// No session = no CSRF protection needed (will fail auth anyway)
				next.ServeHTTP(w, r)
				return
			}

			// Validate CSRF token
			csrfToken := GetTokenFromRequest(r)
			if !ValidateToken(sessionToken, csrfToken) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"error":"Invalid or missing CSRF token"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
