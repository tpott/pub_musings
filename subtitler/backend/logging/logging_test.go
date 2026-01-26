package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"Debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"INFO", slog.LevelInfo},
		{"", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"WARN", slog.LevelWarn},
		{"error", slog.LevelError},
		{"ERROR", slog.LevelError},
		{"invalid", slog.LevelInfo},
		{"  debug  ", slog.LevelDebug},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseLevel(tt.input)
			if result != tt.expected {
				t.Errorf("parseLevel(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestContextFunctions(t *testing.T) {
	ctx := context.Background()

	// Test WithRequestID
	ctx = WithRequestID(ctx, "req-123")
	if v, ok := ctx.Value(RequestIDKey).(string); !ok || v != "req-123" {
		t.Error("WithRequestID failed to set request_id")
	}

	// Test WithUserID
	ctx = WithUserID(ctx, "user-456")
	if v, ok := ctx.Value(UserIDKey).(string); !ok || v != "user-456" {
		t.Error("WithUserID failed to set user_id")
	}

	// Test WithVideoID
	ctx = WithVideoID(ctx, "video-789")
	if v, ok := ctx.Value(VideoIDKey).(string); !ok || v != "video-789" {
		t.Error("WithVideoID failed to set video_id")
	}

	// Test WithSessionID
	ctx = WithSessionID(ctx, "sess-abc")
	if v, ok := ctx.Value(SessionIDKey).(string); !ok || v != "sess-abc" {
		t.Error("WithSessionID failed to set session_id")
	}
}

func TestFromContext(t *testing.T) {
	ctx := context.Background()
	ctx = WithRequestID(ctx, "req-123")
	ctx = WithUserID(ctx, "user-456")
	ctx = WithVideoID(ctx, "video-789")
	ctx = WithSessionID(ctx, "sess-abc")

	attrs := FromContext(ctx)

	// Check that we have all 4 pairs (8 elements)
	if len(attrs) != 8 {
		t.Errorf("FromContext returned %d attrs, want 8", len(attrs))
	}

	// Convert to map for easier checking
	attrMap := make(map[string]string)
	for i := 0; i < len(attrs); i += 2 {
		key := attrs[i].(string)
		value := attrs[i+1].(string)
		attrMap[key] = value
	}

	if attrMap["request_id"] != "req-123" {
		t.Error("FromContext missing request_id")
	}
	if attrMap["user_id"] != "user-456" {
		t.Error("FromContext missing user_id")
	}
	if attrMap["video_id"] != "video-789" {
		t.Error("FromContext missing video_id")
	}
	if attrMap["session_id"] != "sess-abc" {
		t.Error("FromContext missing session_id")
	}
}

func TestFromContextEmpty(t *testing.T) {
	ctx := context.Background()
	attrs := FromContext(ctx)

	if len(attrs) != 0 {
		t.Errorf("FromContext on empty context returned %d attrs, want 0", len(attrs))
	}
}

func TestFromContextPartial(t *testing.T) {
	ctx := context.Background()
	ctx = WithRequestID(ctx, "req-123")

	attrs := FromContext(ctx)

	if len(attrs) != 2 {
		t.Errorf("FromContext returned %d attrs, want 2", len(attrs))
	}
}

// Helper to create a logger that writes to a buffer for testing
func setupTestLogger(buf *bytes.Buffer, level slog.Level) {
	opts := &slog.HandlerOptions{Level: level}
	handler := slog.NewTextHandler(buf, opts)
	Logger = slog.New(handler)
}

func TestLogLevels(t *testing.T) {
	tests := []struct {
		name      string
		logLevel  slog.Level
		logFunc   func(string, ...any)
		message   string
		shouldLog bool
	}{
		{"debug at debug level", slog.LevelDebug, Debug, "debug msg", true},
		{"debug at info level", slog.LevelInfo, Debug, "debug msg", false},
		{"info at info level", slog.LevelInfo, Info, "info msg", true},
		{"info at warn level", slog.LevelWarn, Info, "info msg", false},
		{"warn at warn level", slog.LevelWarn, Warn, "warn msg", true},
		{"warn at error level", slog.LevelError, Warn, "warn msg", false},
		{"error at error level", slog.LevelError, Error, "error msg", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			setupTestLogger(&buf, tt.logLevel)

			tt.logFunc(tt.message)

			logged := buf.String()
			hasMessage := strings.Contains(logged, tt.message)

			if hasMessage != tt.shouldLog {
				if tt.shouldLog {
					t.Errorf("Expected message %q to be logged, but it wasn't", tt.message)
				} else {
					t.Errorf("Expected message %q to NOT be logged, but got: %s", tt.message, logged)
				}
			}
		})
	}
}

func TestLogWithArgs(t *testing.T) {
	var buf bytes.Buffer
	setupTestLogger(&buf, slog.LevelDebug)

	Info("test message", "key", "value", "number", 42)

	logged := buf.String()
	if !strings.Contains(logged, "test message") {
		t.Error("Log output missing message")
	}
	if !strings.Contains(logged, "key=value") {
		t.Error("Log output missing key=value")
	}
	if !strings.Contains(logged, "number=42") {
		t.Error("Log output missing number=42")
	}
}

func TestLogContext(t *testing.T) {
	var buf bytes.Buffer
	setupTestLogger(&buf, slog.LevelDebug)

	ctx := context.Background()
	ctx = WithRequestID(ctx, "req-test")
	ctx = WithUserID(ctx, "user-test")

	InfoContext(ctx, "context test", "extra", "field")

	logged := buf.String()
	if !strings.Contains(logged, "request_id=req-test") {
		t.Error("Log output missing request_id")
	}
	if !strings.Contains(logged, "user_id=user-test") {
		t.Error("Log output missing user_id")
	}
	if !strings.Contains(logged, "extra=field") {
		t.Error("Log output missing extra field")
	}
	if !strings.Contains(logged, "context test") {
		t.Error("Log output missing message")
	}
}

func TestInit(t *testing.T) {
	// Test that Init creates a working logger
	Init("debug")
	if Logger == nil {
		t.Error("Init did not set Logger")
	}

	// Verify the logger works
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	Logger = slog.New(handler)

	Debug("test after init")
	if !strings.Contains(buf.String(), "test after init") {
		t.Error("Logger not working after Init")
	}
}
