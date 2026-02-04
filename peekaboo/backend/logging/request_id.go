package logging

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// requestIDKey is the context key for request IDs.
type requestIDKey struct{}

// RequestIDMiddleware adds a unique request ID to each request.
// The ID is stored in the request context and added to the response header.
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for existing request ID header (from upstream proxy)
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}

		// Add to response header
		w.Header().Set("X-Request-ID", requestID)

		// Store in context
		ctx := context.WithValue(r.Context(), requestIDKey{}, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID returns the request ID from the context, or empty string if not set.
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey{}).(string); ok {
		return id
	}
	return ""
}

// generateRequestID creates a random 16-character hex string.
func generateRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback to a fixed string on error (should not happen)
		return "00000000"
	}
	return hex.EncodeToString(b)
}
