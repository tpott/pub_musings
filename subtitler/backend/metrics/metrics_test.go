package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRecordHTTPRequest(t *testing.T) {
	// Record some requests
	RecordHTTPRequest("GET", "/api/videos", 200, 100*time.Millisecond)
	RecordHTTPRequest("POST", "/api/upload", 201, 500*time.Millisecond)
	RecordHTTPRequest("GET", "/api/videos", 500, 50*time.Millisecond)

	// Verify counter was incremented
	count := testutil.ToFloat64(httpRequestsTotal.WithLabelValues("GET", "/api/videos", "200"))
	if count != 1 {
		t.Errorf("expected 1 GET /api/videos 200, got %f", count)
	}

	count = testutil.ToFloat64(httpRequestsTotal.WithLabelValues("POST", "/api/upload", "201"))
	if count != 1 {
		t.Errorf("expected 1 POST /api/upload 201, got %f", count)
	}

	count = testutil.ToFloat64(httpRequestsTotal.WithLabelValues("GET", "/api/videos", "500"))
	if count != 1 {
		t.Errorf("expected 1 GET /api/videos 500, got %f", count)
	}
}

func TestRecordTranscription(t *testing.T) {
	// Record transcription events
	RecordTranscriptionStarted()
	RecordTranscriptionStarted()
	RecordTranscriptionCompleted(30 * time.Second)
	RecordTranscriptionFailed()

	// Verify counters
	started := testutil.ToFloat64(transcriptionTotal.WithLabelValues("started"))
	if started != 2 {
		t.Errorf("expected 2 started, got %f", started)
	}

	completed := testutil.ToFloat64(transcriptionTotal.WithLabelValues("completed"))
	if completed != 1 {
		t.Errorf("expected 1 completed, got %f", completed)
	}

	failed := testutil.ToFloat64(transcriptionTotal.WithLabelValues("failed"))
	if failed != 1 {
		t.Errorf("expected 1 failed, got %f", failed)
	}
}

func TestSetActiveSessions(t *testing.T) {
	SetActiveSessions(5)
	count := testutil.ToFloat64(activeSessionsTotal)
	if count != 5 {
		t.Errorf("expected 5 active sessions, got %f", count)
	}

	SetActiveSessions(10)
	count = testutil.ToFloat64(activeSessionsTotal)
	if count != 10 {
		t.Errorf("expected 10 active sessions, got %f", count)
	}

	SetActiveSessions(3)
	count = testutil.ToFloat64(activeSessionsTotal)
	if count != 3 {
		t.Errorf("expected 3 active sessions, got %f", count)
	}
}

func TestRecordUploadBytes(t *testing.T) {
	// Reset by recording from a known state
	initial := testutil.ToFloat64(uploadsBytesTotal)

	RecordUploadBytes(1000)
	RecordUploadBytes(5000)

	current := testutil.ToFloat64(uploadsBytesTotal)
	expected := initial + 6000
	if current != expected {
		t.Errorf("expected %f bytes total, got %f", expected, current)
	}
}

func TestRecordUploadStatus(t *testing.T) {
	// Record upload events
	RecordUploadSuccess()
	RecordUploadSuccess()
	RecordUploadFailed()

	success := testutil.ToFloat64(uploadsTotal.WithLabelValues("success"))
	if success < 2 {
		t.Errorf("expected at least 2 successful uploads, got %f", success)
	}

	failed := testutil.ToFloat64(uploadsTotal.WithLabelValues("failed"))
	if failed < 1 {
		t.Errorf("expected at least 1 failed upload, got %f", failed)
	}
}

func TestResponseRecorder(t *testing.T) {
	// Test default status code
	w := httptest.NewRecorder()
	recorder := NewResponseRecorder(w)
	if recorder.StatusCode != http.StatusOK {
		t.Errorf("expected default status 200, got %d", recorder.StatusCode)
	}

	// Test WriteHeader
	recorder.WriteHeader(http.StatusNotFound)
	if recorder.StatusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", recorder.StatusCode)
	}
}

func TestMetricsMiddleware(t *testing.T) {
	// Create a simple handler that returns 200
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with metrics middleware
	wrapped := MetricsMiddleware(handler)

	// Make a request
	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	wrapped.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify metrics were recorded
	count := testutil.ToFloat64(httpRequestsTotal.WithLabelValues("GET", "/api/test", "200"))
	if count < 1 {
		t.Error("expected request to be recorded in metrics")
	}
}

func TestMetricsEndpoint(t *testing.T) {
	// Record some test data
	RecordHTTPRequest("GET", "/api/health", 200, 10*time.Millisecond)
	SetActiveSessions(42)

	// Get metrics from handler
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	Handler().ServeHTTP(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	body := w.Body.String()

	// Verify some expected metrics are present
	if !strings.Contains(body, "http_requests_total") {
		t.Error("expected http_requests_total metric in output")
	}
	if !strings.Contains(body, "http_request_duration_seconds") {
		t.Error("expected http_request_duration_seconds metric in output")
	}
	if !strings.Contains(body, "active_sessions_total") {
		t.Error("expected active_sessions_total metric in output")
	}
	if !strings.Contains(body, "uploads_bytes_total") {
		t.Error("expected uploads_bytes_total metric in output")
	}
	if !strings.Contains(body, "transcription_total") {
		t.Error("expected transcription_total metric in output")
	}
}

func TestNormalizePath(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		// Static paths (no changes)
		{
			name:     "static path",
			input:    "/api/health",
			expected: "/api/health",
		},
		{
			name:     "static path with multiple segments",
			input:    "/api/auth/login",
			expected: "/api/auth/login",
		},

		// 32-character hex IDs (common in this codebase)
		{
			name:     "hex ID in middle of path",
			input:    "/api/videos/abc123def456789012345678901234ab/video",
			expected: "/api/videos/{id}/video",
		},
		{
			name:     "hex ID at end of path",
			input:    "/api/videos/abc123def456789012345678901234ab",
			expected: "/api/videos/{id}",
		},
		{
			name:     "hex ID with subtitles suffix",
			input:    "/api/videos/1234567890abcdef1234567890abcdef/subtitles.srt",
			expected: "/api/videos/{id}/subtitles.srt",
		},
		{
			name:     "hex ID for thumbnail",
			input:    "/api/videos/fedcba0987654321fedcba0987654321/thumbnail",
			expected: "/api/videos/{id}/thumbnail",
		},

		// Standard UUIDs with dashes
		{
			name:     "UUID in path",
			input:    "/api/upload/chunk/a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			expected: "/api/upload/chunk/{id}",
		},
		{
			name:     "UUID with trailing path",
			input:    "/api/sessions/a1b2c3d4-e5f6-7890-abcd-ef1234567890/revoke",
			expected: "/api/sessions/{id}/revoke",
		},

		// Numeric IDs
		{
			name:     "numeric ID",
			input:    "/api/auth/sessions/12345",
			expected: "/api/auth/sessions/{id}",
		},
		{
			name:     "single digit ID",
			input:    "/api/items/1",
			expected: "/api/items/{id}",
		},

		// Multiple IDs in same path (edge case)
		{
			name:     "multiple hex IDs",
			input:    "/api/videos/abc123def456789012345678901234ab/chunks/def456abc789012345678901234567cd",
			expected: "/api/videos/{id}/chunks/{id}",
		},

		// Query strings should not be affected
		{
			name:     "path with query params",
			input:    "/api/videos?limit=10",
			expected: "/api/videos?limit=10",
		},

		// Short hex strings should not match (need full 32 chars)
		{
			name:     "short hex should not match",
			input:    "/api/videos/abc123",
			expected: "/api/videos/abc123",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := normalizePath(tc.input)
			if result != tc.expected {
				t.Errorf("normalizePath(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestNormalizePathReducesCardinality(t *testing.T) {
	// Simulate recording metrics for many unique IDs
	uniquePaths := []string{
		"/api/videos/abc123def456789012345678901234ab/video",
		"/api/videos/def456abc789012345678901234567cd/video",
		"/api/videos/111222333444555666777888999000aa/video",
		"/api/videos/fffeeeddddcccbbbaaaa9998887776ab/video",
	}

	// All should normalize to the same path
	for _, path := range uniquePaths {
		normalized := normalizePath(path)
		expected := "/api/videos/{id}/video"
		if normalized != expected {
			t.Errorf("Expected all paths to normalize to %q, got %q for input %q", expected, normalized, path)
		}
	}
}
