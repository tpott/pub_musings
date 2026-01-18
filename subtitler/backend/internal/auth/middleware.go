package auth

import (
	"context"
	"encoding/json"
	"net/http"
)

type contextKey string

const claimsContextKey contextKey = "claims"

// AuthMiddleware returns a middleware that validates JWT tokens from cookies
func AuthMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get token from cookie
			cookie, err := r.Cookie(CookieName)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "Authentication required")
				return
			}

			// Validate token
			claims, err := ValidateJWT(cookie.Value, secret)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "Invalid or expired token")
				return
			}

			// Add claims to request context
			ctx := context.WithValue(r.Context(), claimsContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetClaims extracts claims from the request context
func GetClaims(r *http.Request) (*Claims, bool) {
	claims, ok := r.Context().Value(claimsContextKey).(*Claims)
	return claims, ok
}

// writeError sends a JSON error response
func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   message,
	})
}
