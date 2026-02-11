package api

import "net/http"

// CORSMiddleware adds CORS headers for frontend access.
// allowedOrigin specifies the allowed origin for CORS requests.
// Use "*" to allow any origin (development only), or a specific origin like "https://peekaboo.example.com".
func CORSMiddleware(next http.Handler, allowedOrigin string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-CSRF-Token")

		// Allow credentials (cookies) for cross-origin requests when a
		// specific origin is configured. Wildcard "*" is incompatible
		// with credentials per the CORS specification.
		if allowedOrigin != "*" {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
