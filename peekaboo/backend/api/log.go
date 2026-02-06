package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
)

// maxLogBodySize is the maximum allowed request body size for /api/log (2KB).
const maxLogBodySize = 2 << 10

// validLogLevels are the accepted log level values.
var validLogLevels = map[string]bool{
	"debug": true,
	"info":  true,
	"warn":  true,
	"error": true,
}

// LogRequest is the incoming request to POST /api/log.
type LogRequest struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

// LogHandler handles POST /api/log requests, forwarding frontend console logs
// to the backend's structured logger. Only enabled when FORWARD_FRONTEND_LOGS=true.
type LogHandler struct{}

// NewLogHandler creates a new LogHandler.
func NewLogHandler() *LogHandler {
	return &LogHandler{}
}

// ServeHTTP handles the log forwarding request.
func (h *LogHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Limit request body size
	r.Body = http.MaxBytesReader(w, r.Body, maxLogBodySize)

	var req LogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if err.Error() == "http: request body too large" {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Validate level
	level := strings.ToLower(req.Level)
	if !validLogLevels[level] {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Validate message
	msg := strings.TrimSpace(req.Message)
	if msg == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Log at the appropriate level with a "frontend" source tag
	clientIP := getClientIP(r)
	switch level {
	case "debug":
		slog.Debug(msg, "source", "frontend", "client_ip", clientIP)
	case "info":
		slog.Info(msg, "source", "frontend", "client_ip", clientIP)
	case "warn":
		slog.Warn(msg, "source", "frontend", "client_ip", clientIP)
	case "error":
		slog.Error(msg, "source", "frontend", "client_ip", clientIP)
	}

	w.WriteHeader(http.StatusNoContent)
}
