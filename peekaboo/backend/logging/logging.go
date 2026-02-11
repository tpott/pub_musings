// Package logging provides structured logging utilities using slog.
package logging

import (
	"log/slog"
	"os"
	"strings"
)

// Setup configures the global logger based on environment variables.
// LOG_LEVEL: debug, info (default), warn, error
// LOG_FORMAT: json (for production), text (default for development)
func Setup() {
	level := parseLevel(os.Getenv("LOG_LEVEL"))
	format := os.Getenv("LOG_FORMAT")

	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: level,
	}

	if strings.ToLower(format) == "json" {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}

	slog.SetDefault(slog.New(handler))
}

// parseLevel converts a string log level to slog.Level.
func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "", "info":
		return slog.LevelInfo
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		// Use stderr directly since slog isn't configured yet
		os.Stderr.WriteString("WARNING: unrecognized LOG_LEVEL \"" + level + "\", falling back to info\n")
		return slog.LevelInfo
	}
}
