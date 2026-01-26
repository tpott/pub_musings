// Package logging provides structured logging using slog.
package logging

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

// Context keys for structured logging fields
type contextKey string

const (
	RequestIDKey contextKey = "request_id"
	UserIDKey    contextKey = "user_id"
	VideoIDKey   contextKey = "video_id"
	SessionIDKey contextKey = "session_id"
)

// Logger is the global structured logger
var Logger *slog.Logger

func init() {
	// Initialize with default logger if not already set.
	// This ensures the logger works even if Init() is never called.
	if Logger == nil {
		Logger = slog.Default()
	}
}

// Init initializes the global logger with the specified log level.
// Valid levels: debug, info, warn, error. Default is info.
func Init(levelStr string) {
	level := parseLevel(levelStr)

	opts := &slog.HandlerOptions{
		Level: level,
	}

	handler := slog.NewTextHandler(os.Stdout, opts)
	Logger = slog.New(handler)
	slog.SetDefault(Logger)
}

// parseLevel converts a string log level to slog.Level.
// Returns info level for invalid/empty strings.
func parseLevel(levelStr string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(levelStr)) {
	case "debug":
		return slog.LevelDebug
	case "info", "":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// WithRequestID returns a new context with the request ID attached.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// WithUserID returns a new context with the user ID attached.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

// WithVideoID returns a new context with the video ID attached.
func WithVideoID(ctx context.Context, videoID string) context.Context {
	return context.WithValue(ctx, VideoIDKey, videoID)
}

// WithSessionID returns a new context with the session ID attached.
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, SessionIDKey, sessionID)
}

// FromContext extracts logging attributes from context.
func FromContext(ctx context.Context) []any {
	var attrs []any

	if requestID, ok := ctx.Value(RequestIDKey).(string); ok && requestID != "" {
		attrs = append(attrs, "request_id", requestID)
	}
	if userID, ok := ctx.Value(UserIDKey).(string); ok && userID != "" {
		attrs = append(attrs, "user_id", userID)
	}
	if videoID, ok := ctx.Value(VideoIDKey).(string); ok && videoID != "" {
		attrs = append(attrs, "video_id", videoID)
	}
	if sessionID, ok := ctx.Value(SessionIDKey).(string); ok && sessionID != "" {
		attrs = append(attrs, "session_id", sessionID)
	}

	return attrs
}

// Debug logs a debug message with optional key-value pairs.
func Debug(msg string, args ...any) {
	Logger.Debug(msg, args...)
}

// DebugContext logs a debug message with context fields.
func DebugContext(ctx context.Context, msg string, args ...any) {
	attrs := append(FromContext(ctx), args...)
	Logger.Debug(msg, attrs...)
}

// Info logs an info message with optional key-value pairs.
func Info(msg string, args ...any) {
	Logger.Info(msg, args...)
}

// InfoContext logs an info message with context fields.
func InfoContext(ctx context.Context, msg string, args ...any) {
	attrs := append(FromContext(ctx), args...)
	Logger.Info(msg, attrs...)
}

// Warn logs a warning message with optional key-value pairs.
func Warn(msg string, args ...any) {
	Logger.Warn(msg, args...)
}

// WarnContext logs a warning message with context fields.
func WarnContext(ctx context.Context, msg string, args ...any) {
	attrs := append(FromContext(ctx), args...)
	Logger.Warn(msg, attrs...)
}

// Error logs an error message with optional key-value pairs.
func Error(msg string, args ...any) {
	Logger.Error(msg, args...)
}

// ErrorContext logs an error message with context fields.
func ErrorContext(ctx context.Context, msg string, args ...any) {
	attrs := append(FromContext(ctx), args...)
	Logger.Error(msg, attrs...)
}

// Fatal logs an error message and exits the program.
func Fatal(msg string, args ...any) {
	Logger.Error(msg, args...)
	os.Exit(1)
}
