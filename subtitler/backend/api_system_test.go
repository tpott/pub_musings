package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tpott/subtitler/backend/audio"
	"github.com/tpott/subtitler/backend/db"
	"github.com/tpott/subtitler/backend/ratelimit"
)

func TestHealthEndpointUnauthenticated(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	resp := ts.doRequest("GET", "/api/health", nil, "")

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}

	// Unauthenticated should only get status field
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["status"] != "ok" {
		t.Errorf("Expected status 'ok', got '%v'", result["status"])
	}

	// Should NOT have detailed fields
	if _, ok := result["db_connected"]; ok {
		t.Error("Unauthenticated response should not include db_connected")
	}
	if _, ok := result["whisper_available"]; ok {
		t.Error("Unauthenticated response should not include whisper_available")
	}
	if _, ok := result["disk_space_ok"]; ok {
		t.Error("Unauthenticated response should not include disk_space_ok")
	}
	if _, ok := result["disk_free_gb"]; ok {
		t.Error("Unauthenticated response should not include disk_free_gb")
	}
	if _, ok := result["errors"]; ok {
		t.Error("Unauthenticated response should not include errors")
	}
}

// TestHealthEndpointAuthenticated tests that authenticated requests get full response

func TestHealthEndpointAuthenticated(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a user and get auth token
	token := ts.createTestUser(t, "health@example.com", "Testpass123!")

	resp := ts.doRequest("GET", "/api/health", nil, token)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}

	var result HealthStatus
	json.NewDecoder(resp.Body).Decode(&result)

	if result.Status != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", result.Status)
	}

	// Verify enhanced health check fields are present for authenticated users
	if !result.DBConnected {
		t.Error("Expected DBConnected to be true")
	}
	if !result.WhisperAvailable {
		t.Error("Expected WhisperAvailable to be true (whisper-server not configured)")
	}
	if !result.DiskSpaceOK {
		t.Error("Expected DiskSpaceOK to be true")
	}
	if result.DiskFreeGB <= 0 {
		t.Error("Expected DiskFreeGB to be greater than 0")
	}
	if len(result.Errors) != 0 {
		t.Errorf("Expected no errors, got: %v", result.Errors)
	}
}

// TestHealthEndpointDBDownUnauthenticated tests unauthenticated response when DB is down

func TestHealthEndpointDBDownUnauthenticated(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Close the database to simulate a connection failure
	ts.db.Close()

	resp := ts.doRequest("GET", "/api/health", nil, "")

	// Should return 503 Service Unavailable when DB is down
	if resp.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", resp.Code)
	}

	// Unauthenticated should only get status field
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["status"] != "degraded" {
		t.Errorf("Expected status 'degraded', got '%v'", result["status"])
	}

	// Should NOT have detailed fields even when degraded
	if _, ok := result["db_connected"]; ok {
		t.Error("Unauthenticated response should not include db_connected")
	}
	if _, ok := result["errors"]; ok {
		t.Error("Unauthenticated response should not include errors")
	}
}

// TestHealthEndpointDBDownAuthenticated tests authenticated response when DB is down
// Note: This is a tricky edge case - authentication itself requires DB access.
// When DB is down, we can't validate the session, so the user effectively becomes unauthenticated.

func TestHealthEndpointDBDownAuthenticated(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a user and get auth token while DB is still up
	token := ts.createTestUser(t, "healthdown@example.com", "Testpass123!")

	// Close the database to simulate a connection failure
	ts.db.Close()

	resp := ts.doRequest("GET", "/api/health", nil, token)

	// Should return 503 Service Unavailable when DB is down
	if resp.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", resp.Code)
	}

	// When DB is down, we can't validate the session, so response should be minimal
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["status"] != "degraded" {
		t.Errorf("Expected status 'degraded', got '%v'", result["status"])
	}
}

func TestLogEndpoint(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	tests := []struct {
		name       string
		body       map[string]interface{}
		wantStatus int
	}{
		{
			name: "valid log message",
			body: map[string]interface{}{
				"level":   "log",
				"message": "Test log message",
				"url":     "http://localhost:4321/upload",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "error level with line info",
			body: map[string]interface{}{
				"level":   "error",
				"message": "Test error message",
				"url":     "http://localhost:4321/upload",
				"line":    42,
				"column":  10,
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "warn level",
			body: map[string]interface{}{
				"level":   "warn",
				"message": "Test warning",
				"url":     "http://localhost:4321/",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "debug level",
			body: map[string]interface{}{
				"level":   "debug",
				"message": "Debug info",
				"url":     "http://localhost:4321/",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "info level",
			body: map[string]interface{}{
				"level":   "info",
				"message": "Info message",
				"url":     "http://localhost:4321/",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "invalid level defaults to log",
			body: map[string]interface{}{
				"level":   "invalid",
				"message": "Message with invalid level",
				"url":     "http://localhost:4321/",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "minimal body",
			body: map[string]interface{}{
				"message": "Just a message",
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			resp := ts.doRequest("POST", "/api/log", bytes.NewReader(body), "")

			if resp.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, resp.Code)
			}

			var result map[string]string
			json.NewDecoder(resp.Body).Decode(&result)

			if result["status"] != "ok" {
				t.Errorf("Expected status 'ok', got '%s'", result["status"])
			}
		})
	}
}

func TestLogEndpointInvalidBody(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Send completely invalid JSON directly (bypass doRequest's json.Marshal)
	req := httptest.NewRequest("POST", "/api/log", strings.NewReader(`{level: not-valid-json}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

func TestDetectScriptEndpoint(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	tests := []struct {
		name           string
		text           string
		expectedScript string
		expectedLang   string
	}{
		{
			name:           "Latin text",
			text:           "Hello World",
			expectedScript: "Latin",
			expectedLang:   "hi", // Defaults to Hindi for ambiguous romanized text
		},
		{
			name:           "Devanagari text",
			text:           "नमस्ते दुनिया",
			expectedScript: "Devanagari",
			expectedLang:   "hi",
		},
		{
			name:           "Hindi romanized with keywords",
			text:           "main tumse pyar karta hoon",
			expectedScript: "Latin",
			expectedLang:   "hi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := ts.doRequest("POST", "/api/text/detect-script", map[string]string{
				"text": tt.text,
			}, "")

			if w.Code != http.StatusOK {
				t.Errorf("Expected 200, got %d", w.Code)
				return
			}

			var resp map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Errorf("Failed to decode response: %v", err)
				return
			}

			if resp["detected_script"] != tt.expectedScript {
				t.Errorf("Expected script %q, got %q", tt.expectedScript, resp["detected_script"])
			}
			if resp["detected_language"] != tt.expectedLang {
				t.Errorf("Expected language %q, got %q", tt.expectedLang, resp["detected_language"])
			}
		})
	}
}

// TestDetectScriptTooLong tests script detection with text that's too long

func TestDetectScriptTooLong(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create 11KB of text (over 10KB limit)
	longText := strings.Repeat("a", 11*1024)

	w := ts.doRequest("POST", "/api/text/detect-script", map[string]string{
		"text": longText,
	}, "")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for text too long, got %d", w.Code)
	}
}

// TestConvertScriptEndpoint tests the script conversion API

func TestConvertScriptEndpoint(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	w := ts.doRequest("POST", "/api/text/convert", map[string]string{
		"text":          "namaste",
		"target_script": "Devanagari",
		"language":      "hi",
	}, "")

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
		return
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Errorf("Failed to decode response: %v", err)
		return
	}

	if resp["original"] != "namaste" {
		t.Errorf("Expected original %q, got %q", "namaste", resp["original"])
	}
	if resp["converted"] == "" {
		t.Error("Expected non-empty converted text")
	}
	if resp["target_script"] != "Devanagari" {
		t.Errorf("Expected target_script %q, got %q", "Devanagari", resp["target_script"])
	}
}

// TestConvertScriptUnsupportedLanguage tests script conversion with unsupported language

func TestConvertScriptUnsupportedLanguage(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	w := ts.doRequest("POST", "/api/text/convert", map[string]string{
		"text":          "hello",
		"target_script": "Devanagari",
		"language":      "en", // English not supported
	}, "")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for unsupported language, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] != "Unsupported language" {
		t.Errorf("Expected unsupported language error, got %v", resp["error"])
	}
}

// TestConvertScriptUnsupportedScript tests script conversion with unsupported target script

func TestConvertScriptUnsupportedScript(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	w := ts.doRequest("POST", "/api/text/convert", map[string]string{
		"text":          "namaste",
		"target_script": "Latin", // Latin not supported as target
		"language":      "hi",
	}, "")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for unsupported script, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] != "Unsupported target script" {
		t.Errorf("Expected unsupported script error, got %v", resp["error"])
	}
}

// TestConvertScriptMultipleWords tests script conversion with multiple words

func TestConvertScriptMultipleWords(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	w := ts.doRequest("POST", "/api/text/convert", map[string]string{
		"text":          "namaste duniya",
		"target_script": "Devanagari",
		"language":      "hi",
	}, "")

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
		return
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	converted := resp["converted"].(string)
	// Should have two words separated by space
	if !strings.Contains(converted, " ") {
		t.Error("Expected converted text to have multiple words with space")
	}
}

// Test forgot-password endpoint

func TestRateLimitingDetectScript(t *testing.T) {
	strictLimiter := ratelimit.New(10, time.Minute)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/text/detect-script", strictLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"detected_script": "Latin"})
	}))

	// First 10 requests should succeed
	for i := 0; i < 10; i++ {
		body := bytes.NewBufferString(`{"text":"hello"}`)
		req := httptest.NewRequest("POST", "/api/text/detect-script", body)
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.168.1.100:12345"
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// 11th request should be rate limited
	body := bytes.NewBufferString(`{"text":"hello"}`)
	req := httptest.NewRequest("POST", "/api/text/detect-script", body)
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.168.1.100:12345"
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429 Too Many Requests, got %d", w.Code)
	}
}

// TestRateLimitingMetrics tests that /metrics endpoint is rate limited

func TestRateLimitingMetrics(t *testing.T) {
	strictLimiter := ratelimit.New(10, time.Minute)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /metrics", strictLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("# metrics\n"))
	}))

	// First 10 requests should succeed
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/metrics", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// 11th request should be rate limited
	req := httptest.NewRequest("GET", "/metrics", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429 Too Many Requests, got %d", w.Code)
	}
}

// ========== Reprocess Tests ==========

func TestRequestIDMiddleware(t *testing.T) {
	// Create a simple handler for testing
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with middleware
	wrapped := requestIDMiddleware(handler)

	t.Run("adds X-Request-ID header to response", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		wrapped.ServeHTTP(w, req)

		requestID := w.Header().Get("X-Request-ID")
		if requestID == "" {
			t.Error("Expected X-Request-ID header in response")
		}
		// Should be 16 hex chars (8 bytes)
		if len(requestID) != 16 {
			t.Errorf("Expected request ID length 16, got %d", len(requestID))
		}
	})

	t.Run("uses existing X-Request-ID from request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Request-ID", "existing-id-12345")
		w := httptest.NewRecorder()

		wrapped.ServeHTTP(w, req)

		requestID := w.Header().Get("X-Request-ID")
		if requestID != "existing-id-12345" {
			t.Errorf("Expected request ID 'existing-id-12345', got '%s'", requestID)
		}
	})

	t.Run("generates unique IDs for each request", func(t *testing.T) {
		ids := make(map[string]bool)
		for i := 0; i < 100; i++ {
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			wrapped.ServeHTTP(w, req)

			requestID := w.Header().Get("X-Request-ID")
			if ids[requestID] {
				t.Errorf("Duplicate request ID generated: %s", requestID)
			}
			ids[requestID] = true
		}
	})
}

func TestRequestIDInHealthCheck(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Note: The test server doesn't use the middleware, but we can test that
	// the middleware would work by testing it directly on the health endpoint handler
	handler := requestIDMiddleware(ts.mux)

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should have request ID header
	requestID := w.Header().Get("X-Request-ID")
	if requestID == "" {
		t.Error("Expected X-Request-ID header in health check response")
	}

	// Should still return OK
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// Tests for HTTP caching headers

func TestMetricsEndpointAdminOnly(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	t.Run("unauthenticated user gets 401", func(t *testing.T) {
		resp := ts.doRequest("GET", "/metrics", nil, "")

		if resp.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401 for unauthenticated user, got %d: %s", resp.Code, resp.Body.String())
		}
	})

	t.Run("regular user gets 403", func(t *testing.T) {
		// Create a regular (non-admin) user
		_, token := ts.createTestUserWithID(t, "regularuser@example.com", "Password123!")

		resp := ts.doRequest("GET", "/metrics", nil, token)

		if resp.Code != http.StatusForbidden {
			t.Errorf("Expected status 403 for non-admin user, got %d: %s", resp.Code, resp.Body.String())
		}

		var result map[string]string
		json.NewDecoder(resp.Body).Decode(&result)
		if result["error"] != "Admin access required" {
			t.Errorf("Expected error 'Admin access required', got '%s'", result["error"])
		}
	})

	t.Run("admin user gets 200", func(t *testing.T) {
		// Create an admin user
		_, token := ts.createTestAdminUser(t, "adminuser@example.com", "Password123!")

		resp := ts.doRequest("GET", "/metrics", nil, token)

		if resp.Code != http.StatusOK {
			t.Errorf("Expected status 200 for admin user, got %d: %s", resp.Code, resp.Body.String())
		}
	})

	t.Run("api key bypasses admin check", func(t *testing.T) {
		// Test that API key authentication bypasses admin role check
		// Note: This requires setting METRICS_API_KEY env var in the test server
		// For now, we just verify the endpoint works with API key via header
		os.Setenv("METRICS_API_KEY", "test-api-key-123")
		defer os.Unsetenv("METRICS_API_KEY")

		// Create a new test server with the API key set
		ts2 := setupTestServer(t)
		defer ts2.cleanup()

		req := httptest.NewRequest("GET", "/metrics", nil)
		req.Header.Set("X-Metrics-API-Key", "test-api-key-123")

		rr := httptest.NewRecorder()
		ts2.mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200 with valid API key, got %d: %s", rr.Code, rr.Body.String())
		}
	})
}

func TestFeedbackSubmitSuccess(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	body := map[string]interface{}{
		"text":         "This is great feedback!",
		"type":         "general",
		"page_url":     "http://localhost:4321/upload",
		"browser_info": "Mozilla/5.0 Test Browser",
	}

	resp := ts.doRequest("POST", "/api/feedback", body, "")

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["status"] != "ok" {
		t.Errorf("Expected status 'ok', got '%v'", result["status"])
	}

	if result["id"] == nil || result["id"] == "" {
		t.Error("Expected feedback ID in response")
	}
}

func TestFeedbackSubmitWithRating(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	body := map[string]interface{}{
		"text":     "Great app, 5 stars!",
		"type":     "general",
		"rating":   5,
		"page_url": "http://localhost:4321/videos",
	}

	resp := ts.doRequest("POST", "/api/feedback", body, "")

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestFeedbackSubmitBugReport(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	body := map[string]interface{}{
		"text":     "Found a bug with video upload",
		"type":     "bug",
		"page_url": "http://localhost:4321/upload",
	}

	resp := ts.doRequest("POST", "/api/feedback", body, "")

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestFeedbackSubmitFeatureRequest(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	body := map[string]interface{}{
		"text":     "Please add dark mode",
		"type":     "feature",
		"page_url": "http://localhost:4321/settings",
	}

	resp := ts.doRequest("POST", "/api/feedback", body, "")

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestFeedbackSubmitMissingText(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	body := map[string]interface{}{
		"type":     "general",
		"page_url": "http://localhost:4321/",
	}

	resp := ts.doRequest("POST", "/api/feedback", body, "")

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.Code)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["error"] != "Feedback text is required" {
		t.Errorf("Expected 'Feedback text is required' error, got '%v'", result["error"])
	}
}

func TestFeedbackSubmitEmptyText(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	body := map[string]interface{}{
		"text":     "",
		"type":     "general",
		"page_url": "http://localhost:4321/",
	}

	resp := ts.doRequest("POST", "/api/feedback", body, "")

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.Code)
	}
}

func TestFeedbackSubmitInvalidType(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	body := map[string]interface{}{
		"text":     "Test feedback",
		"type":     "invalid_type",
		"page_url": "http://localhost:4321/",
	}

	resp := ts.doRequest("POST", "/api/feedback", body, "")

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.Code)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["error"] != "Invalid feedback type" {
		t.Errorf("Expected 'Invalid feedback type' error, got '%v'", result["error"])
	}
}

func TestFeedbackSubmitInvalidRating(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Rating too high
	body := map[string]interface{}{
		"text":     "Test feedback",
		"type":     "general",
		"rating":   10,
		"page_url": "http://localhost:4321/",
	}

	resp := ts.doRequest("POST", "/api/feedback", body, "")

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.Code)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["error"] != "Rating must be between 1 and 5" {
		t.Errorf("Expected rating error, got '%v'", result["error"])
	}
}

func TestFeedbackSubmitRatingTooLow(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	body := map[string]interface{}{
		"text":     "Test feedback",
		"type":     "general",
		"rating":   0,
		"page_url": "http://localhost:4321/",
	}

	resp := ts.doRequest("POST", "/api/feedback", body, "")

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.Code)
	}
}

func TestFeedbackSubmitTextTooLong(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create text longer than 10KB
	longText := strings.Repeat("a", 10241)

	body := map[string]interface{}{
		"text":     longText,
		"type":     "general",
		"page_url": "http://localhost:4321/",
	}

	resp := ts.doRequest("POST", "/api/feedback", body, "")

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.Code)
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["error"] != "Feedback text is too long (max 10KB)" {
		t.Errorf("Expected text too long error, got '%v'", result["error"])
	}
}

func TestFeedbackSubmitAuthenticated(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a user
	token := ts.createTestUser(t, "feedback@example.com", "Testpass123!")

	body := map[string]interface{}{
		"text":     "Feedback from authenticated user",
		"type":     "general",
		"page_url": "http://localhost:4321/upload",
	}

	resp := ts.doRequest("POST", "/api/feedback", body, token)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	// Verify user_id was captured
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	feedbackID := result["id"].(string)
	feedback, err := ts.db.GetFeedback(feedbackID)
	if err != nil {
		t.Fatalf("Failed to get feedback: %v", err)
	}

	if feedback.UserID == nil {
		t.Error("Expected user_id to be set for authenticated request")
	}
}

func TestFeedbackSubmitWithVideoID(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a video
	video := ts.createTestVideo(t, nil, nil)

	body := map[string]interface{}{
		"text":     "Feedback about a specific video",
		"type":     "bug",
		"video_id": video.ID,
		"page_url": "http://localhost:4321/upload?video=" + video.ID,
	}

	resp := ts.doRequest("POST", "/api/feedback", body, "")

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	// Verify video_id was captured
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	feedbackID := result["id"].(string)
	feedback, err := ts.db.GetFeedback(feedbackID)
	if err != nil {
		t.Fatalf("Failed to get feedback: %v", err)
	}

	if feedback.VideoID == nil || *feedback.VideoID != video.ID {
		t.Error("Expected video_id to match the submitted video")
	}
}

func TestFeedbackSubmitDefaultType(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	body := map[string]interface{}{
		"text":     "Feedback without explicit type",
		"page_url": "http://localhost:4321/",
	}

	resp := ts.doRequest("POST", "/api/feedback", body, "")

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	// Verify default type is "general"
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	feedbackID := result["id"].(string)
	feedback, err := ts.db.GetFeedback(feedbackID)
	if err != nil {
		t.Fatalf("Failed to get feedback: %v", err)
	}

	if feedback.Type != "general" {
		t.Errorf("Expected default type 'general', got '%s'", feedback.Type)
	}
}

// Language hints endpoint tests

func TestAdminFeedbackListUnauthenticated(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	resp := ts.doRequest("GET", "/api/admin/feedback", nil, "")

	if resp.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 for unauthenticated user, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestAdminFeedbackListNonAdmin(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a regular user (not admin)
	_, token := ts.createTestUserWithID(t, "regularuser@example.com", "Password123!")

	resp := ts.doRequest("GET", "/api/admin/feedback", nil, token)

	if resp.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 for non-admin user, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["error"] != "Admin access required" {
		t.Errorf("Expected 'Admin access required' error, got: %v", result["error"])
	}
}

func TestAdminFeedbackListSuccess(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create an admin user
	_, token := ts.createTestAdminUser(t, "adminuser@example.com", "Password123!")

	// First create some feedback
	body := map[string]interface{}{
		"text":     "Test feedback 1",
		"type":     "general",
		"page_url": "http://localhost/test",
	}
	resp := ts.doRequest("POST", "/api/feedback", body, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("Failed to create test feedback: %s", resp.Body.String())
	}

	body["text"] = "Test feedback 2"
	body["type"] = "bug"
	resp = ts.doRequest("POST", "/api/feedback", body, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("Failed to create test feedback: %s", resp.Body.String())
	}

	// Now list feedback as admin
	resp = ts.doRequest("GET", "/api/admin/feedback", nil, token)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200 for admin user, got %d: %s", resp.Code, resp.Body.String())
	}

	var result struct {
		Feedback []db.Feedback `json:"feedback"`
		Total    int           `json:"total"`
		Limit    int           `json:"limit"`
		Offset   int           `json:"offset"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result.Total != 2 {
		t.Errorf("Expected 2 feedback items, got %d", result.Total)
	}
}

func TestAdminFeedbackListWithFilters(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create an admin user
	_, token := ts.createTestAdminUser(t, "adminuser@example.com", "Password123!")

	// Create feedback with different types
	body := map[string]interface{}{
		"text":     "General feedback",
		"type":     "general",
		"page_url": "http://localhost/test",
	}
	resp := ts.doRequest("POST", "/api/feedback", body, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("Failed to create test feedback: %s", resp.Body.String())
	}

	body["text"] = "Bug report"
	body["type"] = "bug"
	resp = ts.doRequest("POST", "/api/feedback", body, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("Failed to create test feedback: %s", resp.Body.String())
	}

	// Filter by type
	resp = ts.doRequest("GET", "/api/admin/feedback?type=bug", nil, token)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result struct {
		Feedback []db.Feedback `json:"feedback"`
		Total    int           `json:"total"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result.Total != 1 {
		t.Errorf("Expected 1 bug feedback, got %d", result.Total)
	}

	if len(result.Feedback) > 0 && result.Feedback[0].Type != "bug" {
		t.Errorf("Expected bug type, got '%s'", result.Feedback[0].Type)
	}
}

func TestAdminFeedbackGetByIDSuccess(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create an admin user
	_, token := ts.createTestAdminUser(t, "adminuser@example.com", "Password123!")

	// Create a feedback item
	body := map[string]interface{}{
		"text":     "Test feedback for get by ID",
		"type":     "feature",
		"page_url": "http://localhost/test",
	}
	resp := ts.doRequest("POST", "/api/feedback", body, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("Failed to create test feedback: %s", resp.Body.String())
	}

	var createResult map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&createResult)
	feedbackID := createResult["id"].(string)

	// Get feedback by ID as admin
	resp = ts.doRequest("GET", "/api/admin/feedback/"+feedbackID, nil, token)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var feedback db.Feedback
	if err := json.NewDecoder(resp.Body).Decode(&feedback); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if feedback.ID != feedbackID {
		t.Errorf("Expected feedback ID '%s', got '%s'", feedbackID, feedback.ID)
	}

	if feedback.Text != "Test feedback for get by ID" {
		t.Errorf("Expected text 'Test feedback for get by ID', got '%s'", feedback.Text)
	}
}

func TestAdminFeedbackGetByIDNotFound(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create an admin user
	_, token := ts.createTestAdminUser(t, "adminuser@example.com", "Password123!")

	// Try to get a non-existent feedback
	resp := ts.doRequest("GET", "/api/admin/feedback/"+testGenerateID(), nil, token)

	if resp.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestAdminFeedbackGetByIDInvalidFormat(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create an admin user
	_, token := ts.createTestAdminUser(t, "adminuser@example.com", "Password123!")

	// Try to get feedback with invalid ID format
	resp := ts.doRequest("GET", "/api/admin/feedback/invalid-id", nil, token)

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestAdminFeedbackUpdateStatusSuccess(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create an admin user
	_, token := ts.createTestAdminUser(t, "adminuser@example.com", "Password123!")

	// Create a feedback item
	body := map[string]interface{}{
		"text":     "Test feedback for status update",
		"type":     "general",
		"page_url": "http://localhost/test",
	}
	resp := ts.doRequest("POST", "/api/feedback", body, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("Failed to create test feedback: %s", resp.Body.String())
	}

	var createResult map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&createResult)
	feedbackID := createResult["id"].(string)

	// Update status to "read"
	updateBody := map[string]string{"status": "read"}
	resp = ts.doRequest("PATCH", "/api/admin/feedback/"+feedbackID, updateBody, token)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["updated"] != "read" {
		t.Errorf("Expected updated status 'read', got '%v'", result["updated"])
	}

	// Verify the status was actually updated
	feedback, err := ts.db.GetFeedback(feedbackID)
	if err != nil {
		t.Fatalf("Failed to get feedback: %v", err)
	}
	if feedback.Status != "read" {
		t.Errorf("Expected status 'read' in database, got '%s'", feedback.Status)
	}
}

func TestAdminFeedbackUpdateStatusToResolved(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create an admin user
	_, token := ts.createTestAdminUser(t, "adminuser@example.com", "Password123!")

	// Create a feedback item
	body := map[string]interface{}{
		"text":     "Test feedback for resolved status",
		"type":     "bug",
		"page_url": "http://localhost/test",
	}
	resp := ts.doRequest("POST", "/api/feedback", body, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("Failed to create test feedback: %s", resp.Body.String())
	}

	var createResult map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&createResult)
	feedbackID := createResult["id"].(string)

	// Update status to "resolved"
	updateBody := map[string]string{"status": "resolved"}
	resp = ts.doRequest("PATCH", "/api/admin/feedback/"+feedbackID, updateBody, token)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	// Verify the status was updated
	feedback, err := ts.db.GetFeedback(feedbackID)
	if err != nil {
		t.Fatalf("Failed to get feedback: %v", err)
	}
	if feedback.Status != "resolved" {
		t.Errorf("Expected status 'resolved' in database, got '%s'", feedback.Status)
	}
}

func TestAdminFeedbackUpdateStatusInvalid(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create an admin user
	_, token := ts.createTestAdminUser(t, "adminuser@example.com", "Password123!")

	// Create a feedback item
	body := map[string]interface{}{
		"text":     "Test feedback",
		"type":     "general",
		"page_url": "http://localhost/test",
	}
	resp := ts.doRequest("POST", "/api/feedback", body, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("Failed to create test feedback: %s", resp.Body.String())
	}

	var createResult map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&createResult)
	feedbackID := createResult["id"].(string)

	// Try to update with invalid status
	updateBody := map[string]string{"status": "invalid_status"}
	resp = ts.doRequest("PATCH", "/api/admin/feedback/"+feedbackID, updateBody, token)

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["error"] != "Invalid status. Must be: new, read, or resolved" {
		t.Errorf("Unexpected error message: %s", result["error"])
	}
}

func TestAdminFeedbackUpdateStatusNonAdmin(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a regular user (not admin)
	_, regularToken := ts.createTestUserWithID(t, "regularuser@example.com", "Password123!")

	// Create an admin to make some feedback
	_, adminToken := ts.createTestAdminUser(t, "adminuser@example.com", "Password123!")

	// Create a feedback item
	body := map[string]interface{}{
		"text":     "Test feedback",
		"type":     "general",
		"page_url": "http://localhost/test",
	}
	resp := ts.doRequest("POST", "/api/feedback", body, adminToken)
	if resp.Code != http.StatusOK {
		t.Fatalf("Failed to create test feedback: %s", resp.Body.String())
	}

	var createResult map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&createResult)
	feedbackID := createResult["id"].(string)

	// Try to update as non-admin
	updateBody := map[string]string{"status": "read"}
	resp = ts.doRequest("PATCH", "/api/admin/feedback/"+feedbackID, updateBody, regularToken)

	if resp.Code != http.StatusForbidden {
		t.Errorf("Expected status 403 for non-admin, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestAdminFeedbackListPagination(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create an admin user
	_, token := ts.createTestAdminUser(t, "adminuser@example.com", "Password123!")

	// Create 5 feedback items
	for i := 0; i < 5; i++ {
		body := map[string]interface{}{
			"text":     "Test feedback " + strconv.Itoa(i),
			"type":     "general",
			"page_url": "http://localhost/test",
		}
		resp := ts.doRequest("POST", "/api/feedback", body, "")
		if resp.Code != http.StatusOK {
			t.Fatalf("Failed to create test feedback: %s", resp.Body.String())
		}
	}

	// Test pagination with limit=2
	resp := ts.doRequest("GET", "/api/admin/feedback?limit=2&offset=0", nil, token)
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result struct {
		Feedback []db.Feedback `json:"feedback"`
		Total    int           `json:"total"`
		Limit    int           `json:"limit"`
		Offset   int           `json:"offset"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result.Total != 5 {
		t.Errorf("Expected total 5, got %d", result.Total)
	}

	if len(result.Feedback) != 2 {
		t.Errorf("Expected 2 feedback items in response, got %d", len(result.Feedback))
	}

	if result.Limit != 2 {
		t.Errorf("Expected limit 2, got %d", result.Limit)
	}

	// Test second page
	resp = ts.doRequest("GET", "/api/admin/feedback?limit=2&offset=2", nil, token)
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(result.Feedback) != 2 {
		t.Errorf("Expected 2 feedback items on second page, got %d", len(result.Feedback))
	}

	if result.Offset != 2 {
		t.Errorf("Expected offset 2, got %d", result.Offset)
	}
}

func TestAdminFeedbackListExcessiveOffset(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	_, token := ts.createTestAdminUser(t, "adminoffset@example.com", "Password123!")

	// Offset at the maximum should succeed
	resp := ts.doRequest("GET", "/api/admin/feedback?offset=100000", nil, token)
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200 for offset at max, got %d: %s", resp.Code, resp.Body.String())
	}

	// Offset exceeding the maximum should return 400
	resp = ts.doRequest("GET", "/api/admin/feedback?offset=100001", nil, token)
	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for excessive offset, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["error"] != "Offset exceeds maximum allowed value" {
		t.Errorf("Expected offset error message, got '%s'", result["error"])
	}
}

func TestAdminFeedbackListAfterTimestamp(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	_, token := ts.createTestAdminUser(t, "adminafter@example.com", "Password123!")

	// Create feedback items with staggered times
	for i := 0; i < 3; i++ {
		fb := &db.Feedback{
			ID:        fmt.Sprintf("feedback-after-%d", i),
			PageURL:   "http://test.com",
			Text:      fmt.Sprintf("Feedback %d", i),
			Type:      "general",
			Status:    db.FeedbackStatusNew,
			CreatedAt: time.Now().Add(time.Duration(i) * time.Second),
		}
		if err := ts.db.CreateFeedback(fb); err != nil {
			t.Fatalf("Failed to create feedback: %v", err)
		}
	}

	// Query with after=RFC3339 timestamp that should exclude the first 2 items
	cutoff := time.Now().Add(1 * time.Second).UTC().Format(time.RFC3339)
	resp := ts.doRequest("GET", "/api/admin/feedback?after="+cutoff, nil, token)
	if resp.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result struct {
		Feedback []db.Feedback `json:"feedback"`
		Total    int           `json:"total"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("Expected 1 feedback item after cutoff, got %d", result.Total)
	}
}

// TestContentTypeHeader verifies that all JSON API endpoints set Content-Type: application/json
// before writing the response, including error responses.

func TestContentTypeHeader(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	tests := []struct {
		name   string
		method string
		path   string
		body   interface{}
	}{
		{
			name:   "POST /api/auth/register sets Content-Type",
			method: "POST",
			path:   "/api/auth/register",
			body:   map[string]string{"email": "bad", "password": "x"},
		},
		{
			name:   "POST /api/auth/login sets Content-Type",
			method: "POST",
			path:   "/api/auth/login",
			body:   map[string]string{"email": "none@example.com", "password": "wrong"},
		},
		{
			name:   "GET /api/auth/me sets Content-Type",
			method: "GET",
			path:   "/api/auth/me",
			body:   nil,
		},
		{
			name:   "POST /api/auth/forgot-password sets Content-Type",
			method: "POST",
			path:   "/api/auth/forgot-password",
			body:   map[string]string{"email": "nobody@example.com"},
		},
		{
			name:   "GET /api/videos sets Content-Type",
			method: "GET",
			path:   "/api/videos",
			body:   nil,
		},
		{
			name:   "DELETE /api/videos/{id} sets Content-Type",
			method: "DELETE",
			path:   "/api/videos/nonexistent",
			body:   nil,
		},
		{
			name:   "PUT /api/transcribe/{id}/segments sets Content-Type",
			method: "PUT",
			path:   "/api/transcribe/nonexistent/segments",
			body:   map[string]interface{}{"segments": []interface{}{}},
		},
		{
			name:   "GET /api/health sets Content-Type",
			method: "GET",
			path:   "/api/health",
			body:   nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := ts.doRequest(tc.method, tc.path, tc.body, "")
			ct := resp.Header().Get("Content-Type")
			if !strings.HasPrefix(ct, "application/json") {
				t.Errorf("Expected Content-Type starting with application/json, got %q (status %d)", ct, resp.Code)
			}
		})
	}
}

// setVideoEmbeddedSubtitles is a test helper that sets embedded subtitles JSON on a video.
func (ts *testServer) setVideoEmbeddedSubtitles(t *testing.T, videoID string, tracks []audio.SubtitleTrack) {
	t.Helper()
	tracksJSON, err := json.Marshal(tracks)
	if err != nil {
		t.Fatalf("Failed to marshal subtitle tracks: %v", err)
	}
	jsonStr := string(tracksJSON)
	if err := ts.db.UpdateVideoEmbeddedSubtitles(videoID, &jsonStr); err != nil {
		t.Fatalf("Failed to update embedded subtitles: %v", err)
	}
}
