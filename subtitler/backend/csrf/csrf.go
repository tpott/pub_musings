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
	"path/filepath"
	"sync"

	"github.com/trevor/subtitler/backend/logging"
)

const (
	// HeaderName is the HTTP header name for CSRF tokens
	HeaderName = "X-CSRF-Token"

	// TokenLength is the length of the random server secret in bytes
	secretLength = 32

	// defaultSecretPath is the default path for persisting the CSRF secret
	defaultSecretPath = "data/csrf.key"
)

var (
	// serverSecret is the HMAC key used to derive CSRF tokens
	serverSecret []byte
	secretOnce   sync.Once
	secretErr    error
)

// resetSecretForTesting resets the secret state. Only for testing.
func resetSecretForTesting() {
	serverSecret = nil
	secretErr = nil
	secretOnce = sync.Once{}
}

// getSecret returns the server secret, loading or generating it if needed.
// Priority:
// 1. CSRF_SECRET environment variable
// 2. Existing file at data/csrf.key (or CSRF_SECRET_PATH)
// 3. Generate new secret, save to file, and use it
// Returns an error if the secret could not be loaded or generated.
func getSecret() ([]byte, error) {
	secretOnce.Do(func() {
		// Priority 1: Try to get from environment
		envSecret := os.Getenv("CSRF_SECRET")
		if envSecret != "" {
			serverSecret = []byte(envSecret)
			return
		}

		// Determine secret file path
		secretPath := os.Getenv("CSRF_SECRET_PATH")
		if secretPath == "" {
			secretPath = defaultSecretPath
		}

		// Priority 2: Try to load from file
		data, err := os.ReadFile(secretPath)
		if err == nil && len(data) >= secretLength {
			// File exists and has enough bytes
			serverSecret = data[:secretLength]
			return
		}

		// Priority 3: Generate new secret and save to file
		serverSecret = make([]byte, secretLength)
		if _, err := rand.Read(serverSecret); err != nil {
			secretErr = fmt.Errorf("failed to generate CSRF secret: %w", err)
			serverSecret = nil
			return
		}

		// Attempt to save the secret for future restarts
		// Errors here are not fatal - we still have a working secret
		if err := saveSecret(secretPath, serverSecret); err != nil {
			// Log but don't fail - the secret works, just won't persist
			fmt.Fprintf(os.Stderr, "warning: could not persist CSRF secret to %s: %v\n", secretPath, err)
		}
	})
	return serverSecret, secretErr
}

// saveSecret writes the secret to the specified file path.
// Creates the parent directory if it doesn't exist.
func saveSecret(path string, secret []byte) error {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write secret with restrictive permissions (owner read/write only)
	if err := os.WriteFile(path, secret, 0600); err != nil {
		return fmt.Errorf("failed to write secret file: %w", err)
	}

	return nil
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
			if _, err := w.Write([]byte(`{"error":"Invalid or missing CSRF token"}`)); err != nil {
				logging.Warn("failed to write CSRF error response", "error", err.Error())
			}
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
				if _, err := w.Write([]byte(`{"error":"Invalid or missing CSRF token"}`)); err != nil {
					logging.Warn("failed to write CSRF error response", "error", err.Error())
				}
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
