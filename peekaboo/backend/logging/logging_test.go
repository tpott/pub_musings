package logging

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"INFO", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"WARN", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"ERROR", slog.LevelError},
		{"", slog.LevelInfo},
		{"invalid", slog.LevelInfo},
	}

	for _, tc := range tests {
		got := parseLevel(tc.input)
		if got != tc.want {
			t.Errorf("parseLevel(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestGetRequestID(t *testing.T) {
	// Test without request ID
	ctx := context.Background()
	if got := GetRequestID(ctx); got != "" {
		t.Errorf("GetRequestID(empty ctx) = %q, want empty", got)
	}

	// Test with request ID set via middleware
	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := GetRequestID(r.Context())
		if id == "" {
			t.Error("GetRequestID returned empty string after middleware")
		}
		if len(id) != 16 {
			t.Errorf("GetRequestID returned %q with len %d, want len 16", id, len(id))
		}
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Check response header
	if rr.Header().Get("X-Request-ID") == "" {
		t.Error("X-Request-ID header not set in response")
	}
}

func TestRequestIDMiddleware_PreservesExisting(t *testing.T) {
	existingID := "existing-request-id"
	var capturedID string

	handler := RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = GetRequestID(r.Context())
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", existingID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if capturedID != existingID {
		t.Errorf("RequestIDMiddleware did not preserve existing ID: got %q, want %q", capturedID, existingID)
	}
	if rr.Header().Get("X-Request-ID") != existingID {
		t.Errorf("Response header X-Request-ID = %q, want %q", rr.Header().Get("X-Request-ID"), existingID)
	}
}

func TestGenerateRequestID(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := generateRequestID()
		if len(id) != 16 {
			t.Errorf("generateRequestID() = %q with len %d, want len 16", id, len(id))
		}
		if ids[id] {
			t.Errorf("generateRequestID() generated duplicate ID: %q", id)
		}
		ids[id] = true
	}
}
