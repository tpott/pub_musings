// Package metrics provides Prometheus metrics for the subtitler backend.
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// HTTP metrics
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests by method, path, and status code",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	// Transcription metrics
	transcriptionTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "transcription_total",
			Help: "Total number of transcription jobs by status",
		},
		[]string{"status"},
	)

	transcriptionDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "transcription_duration_seconds",
			Help:    "Transcription job duration in seconds",
			Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600},
		},
	)

	// Session metrics
	activeSessionsTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "active_sessions_total",
			Help: "Current number of active user sessions",
		},
	)

	// Upload metrics
	uploadsBytesTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "uploads_bytes_total",
			Help: "Total bytes uploaded",
		},
	)

	uploadsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "uploads_total",
			Help: "Total number of uploads by status",
		},
		[]string{"status"},
	)
)

func init() {
	// Register all metrics with the default registry
	prometheus.MustRegister(
		httpRequestsTotal,
		httpRequestDuration,
		transcriptionTotal,
		transcriptionDuration,
		activeSessionsTotal,
		uploadsBytesTotal,
		uploadsTotal,
	)
}

// Handler returns the Prometheus HTTP handler for the /metrics endpoint.
func Handler() http.Handler {
	return promhttp.Handler()
}

// RecordHTTPRequest records an HTTP request metric.
func RecordHTTPRequest(method, path string, statusCode int, duration time.Duration) {
	status := strconv.Itoa(statusCode)
	httpRequestsTotal.WithLabelValues(method, normalizePath(path), status).Inc()
	httpRequestDuration.WithLabelValues(method, normalizePath(path)).Observe(duration.Seconds())
}

// RecordTranscriptionStarted records that a transcription job has started.
func RecordTranscriptionStarted() {
	transcriptionTotal.WithLabelValues("started").Inc()
}

// RecordTranscriptionCompleted records that a transcription job completed successfully.
func RecordTranscriptionCompleted(duration time.Duration) {
	transcriptionTotal.WithLabelValues("completed").Inc()
	transcriptionDuration.Observe(duration.Seconds())
}

// RecordTranscriptionFailed records that a transcription job failed.
func RecordTranscriptionFailed() {
	transcriptionTotal.WithLabelValues("failed").Inc()
}

// SetActiveSessions updates the current number of active sessions.
func SetActiveSessions(count int) {
	activeSessionsTotal.Set(float64(count))
}

// RecordUploadBytes adds to the total uploaded bytes counter.
func RecordUploadBytes(bytes int64) {
	uploadsBytesTotal.Add(float64(bytes))
}

// RecordUploadSuccess records a successful upload.
func RecordUploadSuccess() {
	uploadsTotal.WithLabelValues("success").Inc()
}

// RecordUploadFailed records a failed upload.
func RecordUploadFailed() {
	uploadsTotal.WithLabelValues("failed").Inc()
}

// normalizePath normalizes URL paths for consistent metric labels.
// Replaces dynamic path segments (UUIDs, etc.) with placeholders.
func normalizePath(path string) string {
	// Keep path as-is for now - can be enhanced to replace dynamic segments
	// For example: /api/videos/abc123/video -> /api/videos/{id}/video
	// This prevents cardinality explosion from unique IDs
	return path
}

// ResponseRecorder wraps http.ResponseWriter to capture the status code.
type ResponseRecorder struct {
	http.ResponseWriter
	StatusCode int
}

// NewResponseRecorder creates a new ResponseRecorder.
func NewResponseRecorder(w http.ResponseWriter) *ResponseRecorder {
	return &ResponseRecorder{
		ResponseWriter: w,
		StatusCode:     http.StatusOK, // Default status
	}
}

// WriteHeader captures the status code and writes it.
func (r *ResponseRecorder) WriteHeader(statusCode int) {
	r.StatusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

// MetricsMiddleware wraps an http.Handler to record request metrics.
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip metrics endpoint itself to avoid recursion
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		recorder := NewResponseRecorder(w)
		next.ServeHTTP(recorder, r)
		duration := time.Since(start)

		RecordHTTPRequest(r.Method, r.URL.Path, recorder.StatusCode, duration)
	})
}
