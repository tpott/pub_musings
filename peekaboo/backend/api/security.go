// Package api provides HTTP handlers for the Peekaboo backend.
package api

import "net/http"

// SecurityHeadersMiddleware wraps an http.Handler to add security headers.
// These headers help protect against common web vulnerabilities:
// - X-Frame-Options: DENY prevents clickjacking by disallowing iframe embedding
// - X-Content-Type-Options: nosniff prevents MIME type sniffing attacks
// - X-XSS-Protection: 1; mode=block enables browser XSS filtering (legacy, but harmless)
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		next.ServeHTTP(w, r)
	})
}
