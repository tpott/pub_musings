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
