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

func TestRequestLoggerMiddleware(t *testing.T) {
	// Test that the middleware calls the next handler
	called := false
	handler := RequestLoggerMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated) // Use non-default status to verify capture
	}))

	req := httptest.NewRequest("POST", "/api/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if !called {
		t.Error("RequestLoggerMiddleware did not call next handler")
	}
	if rr.Code != http.StatusCreated {
		t.Errorf("Response status = %d, want %d", rr.Code, http.StatusCreated)
	}
}

func TestStatusRecorder(t *testing.T) {
	tests := []struct {
		name     string
		handler  http.HandlerFunc
		wantCode int
	}{
		{
			name: "default status OK",
			handler: func(w http.ResponseWriter, r *http.Request) {
				// Don't call WriteHeader - should default to 200
				w.Write([]byte("OK"))
			},
			wantCode: http.StatusOK,
		},
		{
			name: "explicit status 404",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			},
			wantCode: http.StatusNotFound,
		},
		{
			name: "explicit status 500",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			wantCode: http.StatusInternalServerError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			recorder := &statusRecorder{
				ResponseWriter: rr,
				statusCode:     http.StatusOK,
			}

			req := httptest.NewRequest("GET", "/test", nil)
			tc.handler(recorder, req)

			if recorder.statusCode != tc.wantCode {
				t.Errorf("statusRecorder.statusCode = %d, want %d", recorder.statusCode, tc.wantCode)
			}
		})
	}
}
