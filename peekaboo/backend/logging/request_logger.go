package logging

import (
	"bufio"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// statusRecorder wraps http.ResponseWriter to capture the status code.
// It also implements http.Hijacker to support WebSocket upgrades.
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

// Hijack implements http.Hijacker interface, required for WebSocket upgrades.
// It delegates to the underlying ResponseWriter if it supports hijacking.
func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := r.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, http.ErrNotSupported
}

// RequestLoggerMiddleware logs request timing and status information.
// It logs: method, path, status code, duration (ms), request ID.
func RequestLoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap the ResponseWriter to capture status code
		recorder := &statusRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK, // default if WriteHeader not called
		}

		// Call the next handler
		next.ServeHTTP(recorder, r)

		// Calculate duration
		duration := time.Since(start)

		// Log the request with structured fields
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.statusCode,
			"duration_ms", duration.Milliseconds(),
			"request_id", GetRequestID(r.Context()),
		)
	})
}
