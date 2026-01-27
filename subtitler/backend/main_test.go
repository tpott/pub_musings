package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestFormatSRTTimestamp(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{0.0, "00:00:00,000"},
		{1.5, "00:00:01,500"},
		{61.123, "00:01:01,123"},
		{3661.999, "01:01:01,999"},
		{3723.456, "01:02:03,456"},
	}

	for _, tc := range tests {
		result := formatSRTTimestamp(tc.input)
		if result != tc.expected {
			t.Errorf("formatSRTTimestamp(%f) = %s, expected %s", tc.input, result, tc.expected)
		}
	}
}

func TestGenerateSRT(t *testing.T) {
	result := &WhisperResult{
		Language: "en",
		Duration: 10.0,
		Text:     "Hello world. This is a test.",
		Segments: []WhisperSegment{
			{ID: 0, Start: 0.0, End: 2.5, Text: " Hello world."},
			{ID: 1, Start: 3.0, End: 5.5, Text: " This is a test."},
		},
	}

	srt := generateSRT(result)

	// Check that we have proper SRT format
	if !strings.Contains(srt, "1\n") {
		t.Error("SRT should contain sequence number 1")
	}
	if !strings.Contains(srt, "2\n") {
		t.Error("SRT should contain sequence number 2")
	}
	if !strings.Contains(srt, "00:00:00,000 --> 00:00:02,500") {
		t.Error("SRT should contain first timestamp")
	}
	if !strings.Contains(srt, "00:00:03,000 --> 00:00:05,500") {
		t.Error("SRT should contain second timestamp")
	}
	if !strings.Contains(srt, "Hello world.") {
		t.Error("SRT should contain first subtitle text")
	}
	if !strings.Contains(srt, "This is a test.") {
		t.Error("SRT should contain second subtitle text")
	}

	// Check proper spacing between entries
	lines := strings.Split(srt, "\n")
	if len(lines) < 8 {
		t.Errorf("SRT should have at least 8 lines, got %d", len(lines))
	}
}

func TestGenerateSRTEmpty(t *testing.T) {
	result := &WhisperResult{
		Language: "en",
		Duration: 0.0,
		Text:     "",
		Segments: []WhisperSegment{},
	}

	srt := generateSRT(result)

	if srt != "" {
		t.Errorf("Empty segments should produce empty SRT, got: %s", srt)
	}
}

func TestFormatVTTTimestamp(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{0.0, "00:00:00.000"},
		{1.5, "00:00:01.500"},
		{61.123, "00:01:01.123"},
		{3661.999, "01:01:01.999"},
		{3723.456, "01:02:03.456"},
	}

	for _, tc := range tests {
		result := formatVTTTimestamp(tc.input)
		if result != tc.expected {
			t.Errorf("formatVTTTimestamp(%f) = %s, expected %s", tc.input, result, tc.expected)
		}
	}
}

func TestGenerateVTT(t *testing.T) {
	result := &WhisperResult{
		Language: "en",
		Duration: 10.0,
		Text:     "Hello world. This is a test.",
		Segments: []WhisperSegment{
			{ID: 0, Start: 0.0, End: 2.5, Text: " Hello world."},
			{ID: 1, Start: 3.0, End: 5.5, Text: " This is a test."},
		},
	}

	vtt := generateVTT(result)

	// Check WEBVTT header
	if !strings.HasPrefix(vtt, "WEBVTT\n\n") {
		t.Error("VTT should start with 'WEBVTT' header followed by blank line")
	}

	// Check that we have proper VTT format
	if !strings.Contains(vtt, "1\n") {
		t.Error("VTT should contain cue identifier 1")
	}
	if !strings.Contains(vtt, "2\n") {
		t.Error("VTT should contain cue identifier 2")
	}
	// VTT uses period instead of comma for milliseconds
	if !strings.Contains(vtt, "00:00:00.000 --> 00:00:02.500") {
		t.Error("VTT should contain first timestamp with period separator")
	}
	if !strings.Contains(vtt, "00:00:03.000 --> 00:00:05.500") {
		t.Error("VTT should contain second timestamp with period separator")
	}
	if !strings.Contains(vtt, "Hello world.") {
		t.Error("VTT should contain first subtitle text")
	}
	if !strings.Contains(vtt, "This is a test.") {
		t.Error("VTT should contain second subtitle text")
	}
}

func TestGenerateVTTEmpty(t *testing.T) {
	result := &WhisperResult{
		Language: "en",
		Duration: 0.0,
		Text:     "",
		Segments: []WhisperSegment{},
	}

	vtt := generateVTT(result)

	// Even with no segments, VTT should have header
	if vtt != "WEBVTT\n\n" {
		t.Errorf("Empty segments should produce just WEBVTT header, got: %s", vtt)
	}
}

func TestGetEnvOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		expected     string
	}{
		{
			name:         "uses default when env not set",
			key:          "TEST_CONFIG_NOT_SET",
			defaultValue: "default-value",
			envValue:     "",
			expected:     "default-value",
		},
		{
			name:         "uses env value when set",
			key:          "TEST_CONFIG_SET",
			defaultValue: "default-value",
			envValue:     "env-value",
			expected:     "env-value",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Clear the env var first
			os.Unsetenv(tc.key)

			// Set env if test case specifies a value
			if tc.envValue != "" {
				os.Setenv(tc.key, tc.envValue)
				defer os.Unsetenv(tc.key)
			}

			result := getEnvOrDefault(tc.key, tc.defaultValue)
			if result != tc.expected {
				t.Errorf("getEnvOrDefault(%s, %s) = %s, expected %s",
					tc.key, tc.defaultValue, result, tc.expected)
			}
		})
	}
}

func TestGetEnvSizeOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue int64
		envValue     string
		expected     int64
	}{
		{
			name:         "uses default when env not set",
			key:          "TEST_SIZE_NOT_SET",
			defaultValue: 100,
			envValue:     "",
			expected:     100,
		},
		{
			name:         "parses plain number",
			key:          "TEST_SIZE_PLAIN",
			defaultValue: 100,
			envValue:     "500",
			expected:     500,
		},
		{
			name:         "parses KB suffix",
			key:          "TEST_SIZE_KB",
			defaultValue: 100,
			envValue:     "10K",
			expected:     10 * 1024,
		},
		{
			name:         "parses MB suffix",
			key:          "TEST_SIZE_MB",
			defaultValue: 100,
			envValue:     "500M",
			expected:     500 * 1024 * 1024,
		},
		{
			name:         "parses GB suffix",
			key:          "TEST_SIZE_GB",
			defaultValue: 100,
			envValue:     "1G",
			expected:     1 * 1024 * 1024 * 1024,
		},
		{
			name:         "handles lowercase suffix",
			key:          "TEST_SIZE_LOWER",
			defaultValue: 100,
			envValue:     "500m",
			expected:     500 * 1024 * 1024,
		},
		{
			name:         "handles whitespace",
			key:          "TEST_SIZE_WHITESPACE",
			defaultValue: 100,
			envValue:     " 500M ",
			expected:     500 * 1024 * 1024,
		},
		{
			name:         "uses default for invalid value",
			key:          "TEST_SIZE_INVALID",
			defaultValue: 100,
			envValue:     "not-a-number",
			expected:     100,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Clear the env var first
			os.Unsetenv(tc.key)

			// Set env if test case specifies a value
			if tc.envValue != "" {
				os.Setenv(tc.key, tc.envValue)
				defer os.Unsetenv(tc.key)
			}

			result := getEnvSizeOrDefault(tc.key, tc.defaultValue)
			if result != tc.expected {
				t.Errorf("getEnvSizeOrDefault(%s, %d) = %d, expected %d",
					tc.key, tc.defaultValue, result, tc.expected)
			}
		})
	}
}

func TestInitConfig(t *testing.T) {
	// Save original values
	origMaxUploadSize := maxUploadSize
	origUploadDir := uploadDir
	origDBPath := dbPath
	origKeyPath := keyPath

	// Restore after test
	defer func() {
		maxUploadSize = origMaxUploadSize
		uploadDir = origUploadDir
		dbPath = origDBPath
		keyPath = origKeyPath
	}()

	// Set environment variables
	os.Setenv("MAX_UPLOAD_SIZE", "100M")
	os.Setenv("UPLOAD_DIR", "/custom/uploads")
	os.Setenv("DB_PATH", "/custom/db.sqlite")
	os.Setenv("KEY_PATH", "/custom/key.age")
	defer func() {
		os.Unsetenv("MAX_UPLOAD_SIZE")
		os.Unsetenv("UPLOAD_DIR")
		os.Unsetenv("DB_PATH")
		os.Unsetenv("KEY_PATH")
	}()

	// Call initConfig
	initConfig()

	// Verify values
	if maxUploadSize != 100*1024*1024 {
		t.Errorf("maxUploadSize = %d, expected %d", maxUploadSize, 100*1024*1024)
	}
	if uploadDir != "/custom/uploads" {
		t.Errorf("uploadDir = %s, expected /custom/uploads", uploadDir)
	}
	if dbPath != "/custom/db.sqlite" {
		t.Errorf("dbPath = %s, expected /custom/db.sqlite", dbPath)
	}
	if keyPath != "/custom/key.age" {
		t.Errorf("keyPath = %s, expected /custom/key.age", keyPath)
	}
}

func TestInitConfigDefaults(t *testing.T) {
	// Save original values
	origMaxUploadSize := maxUploadSize
	origUploadDir := uploadDir
	origDBPath := dbPath
	origKeyPath := keyPath

	// Restore after test
	defer func() {
		maxUploadSize = origMaxUploadSize
		uploadDir = origUploadDir
		dbPath = origDBPath
		keyPath = origKeyPath
	}()

	// Clear environment variables
	os.Unsetenv("MAX_UPLOAD_SIZE")
	os.Unsetenv("UPLOAD_DIR")
	os.Unsetenv("DB_PATH")
	os.Unsetenv("KEY_PATH")

	// Call initConfig
	initConfig()

	// Verify defaults are used
	if maxUploadSize != defaultMaxUploadSize {
		t.Errorf("maxUploadSize = %d, expected default %d", maxUploadSize, defaultMaxUploadSize)
	}
	if uploadDir != defaultUploadDir {
		t.Errorf("uploadDir = %s, expected default %s", uploadDir, defaultUploadDir)
	}
	if dbPath != defaultDBPath {
		t.Errorf("dbPath = %s, expected default %s", dbPath, defaultDBPath)
	}
	if keyPath != defaultKeyPath {
		t.Errorf("keyPath = %s, expected default %s", keyPath, defaultKeyPath)
	}
}

func TestParseRateLimit(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		defaultCount  int
		defaultWindow time.Duration
		expectCount   int
		expectWindow  time.Duration
	}{
		{
			name:          "empty uses defaults",
			input:         "",
			defaultCount:  5,
			defaultWindow: time.Minute,
			expectCount:   5,
			expectWindow:  time.Minute,
		},
		{
			name:          "parse 5/min",
			input:         "5/min",
			defaultCount:  10,
			defaultWindow: time.Hour,
			expectCount:   5,
			expectWindow:  time.Minute,
		},
		{
			name:          "parse 10/minute",
			input:         "10/minute",
			defaultCount:  5,
			defaultWindow: time.Second,
			expectCount:   10,
			expectWindow:  time.Minute,
		},
		{
			name:          "parse 3/hour",
			input:         "3/hour",
			defaultCount:  5,
			defaultWindow: time.Minute,
			expectCount:   3,
			expectWindow:  time.Hour,
		},
		{
			name:          "parse 100/s",
			input:         "100/s",
			defaultCount:  5,
			defaultWindow: time.Minute,
			expectCount:   100,
			expectWindow:  time.Second,
		},
		{
			name:          "parse 3/15m duration format",
			input:         "3/15m",
			defaultCount:  5,
			defaultWindow: time.Minute,
			expectCount:   3,
			expectWindow:  15 * time.Minute,
		},
		{
			name:          "parse 2/30s duration format",
			input:         "2/30s",
			defaultCount:  5,
			defaultWindow: time.Minute,
			expectCount:   2,
			expectWindow:  30 * time.Second,
		},
		{
			name:          "invalid format uses defaults",
			input:         "invalid",
			defaultCount:  5,
			defaultWindow: time.Minute,
			expectCount:   5,
			expectWindow:  time.Minute,
		},
		{
			name:          "invalid count uses defaults",
			input:         "abc/min",
			defaultCount:  5,
			defaultWindow: time.Minute,
			expectCount:   5,
			expectWindow:  time.Minute,
		},
		{
			name:          "zero count uses defaults",
			input:         "0/min",
			defaultCount:  5,
			defaultWindow: time.Minute,
			expectCount:   5,
			expectWindow:  time.Minute,
		},
		{
			name:          "negative count uses defaults",
			input:         "-1/min",
			defaultCount:  5,
			defaultWindow: time.Minute,
			expectCount:   5,
			expectWindow:  time.Minute,
		},
		{
			name:          "invalid window uses defaults",
			input:         "5/xyz",
			defaultCount:  3,
			defaultWindow: time.Hour,
			expectCount:   3,
			expectWindow:  time.Hour,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			count, window := parseRateLimit(tc.input, tc.defaultCount, tc.defaultWindow)
			if count != tc.expectCount {
				t.Errorf("parseRateLimit(%q) count = %d, expected %d", tc.input, count, tc.expectCount)
			}
			if window != tc.expectWindow {
				t.Errorf("parseRateLimit(%q) window = %v, expected %v", tc.input, window, tc.expectWindow)
			}
		})
	}
}

func TestGetEnvRateLimitOrDefault(t *testing.T) {
	key := "TEST_RATE_LIMIT"

	// Clear any existing value
	os.Unsetenv(key)

	// Test default when not set
	count, window := getEnvRateLimitOrDefault(key, 5, time.Minute)
	if count != 5 || window != time.Minute {
		t.Errorf("expected (5, 1m), got (%d, %v)", count, window)
	}

	// Test with env var set
	os.Setenv(key, "10/hour")
	defer os.Unsetenv(key)

	count, window = getEnvRateLimitOrDefault(key, 5, time.Minute)
	if count != 10 || window != time.Hour {
		t.Errorf("expected (10, 1h), got (%d, %v)", count, window)
	}
}

func TestInitRateLimiters(t *testing.T) {
	// Save original config values
	origAuthRateLimit := authRateLimit
	origAuthRateWindow := authRateWindow

	// Restore after test
	defer func() {
		authRateLimit = origAuthRateLimit
		authRateWindow = origAuthRateWindow
	}()

	// Set custom values
	authRateLimit = 100
	authRateWindow = time.Hour

	// Initialize rate limiters
	initRateLimiters()

	// Verify limiters were created (not nil)
	if authLimiter == nil {
		t.Error("authLimiter should not be nil after initRateLimiters")
	}
	if passwordResetLimiter == nil {
		t.Error("passwordResetLimiter should not be nil after initRateLimiters")
	}
	if uploadLimiter == nil {
		t.Error("uploadLimiter should not be nil after initRateLimiters")
	}
	if transcribeLimiter == nil {
		t.Error("transcribeLimiter should not be nil after initRateLimiters")
	}
	if burnLimiter == nil {
		t.Error("burnLimiter should not be nil after initRateLimiters")
	}
	if scriptLimiter == nil {
		t.Error("scriptLimiter should not be nil after initRateLimiters")
	}
}

func TestSecurityHeadersMiddleware(t *testing.T) {
	// Create a simple handler that we can wrap
	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap it with security headers middleware
	handler := securityHeadersMiddleware(innerHandler)

	// Create a test request
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()

	// Call the handler
	handler.ServeHTTP(rec, req)

	// Verify CSP header is set
	csp := rec.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Error("Content-Security-Policy header not set")
	}
	if !strings.Contains(csp, "default-src 'self'") {
		t.Error("CSP should contain default-src 'self'")
	}
	if !strings.Contains(csp, "script-src 'self'") {
		t.Error("CSP should contain script-src")
	}
	if !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Error("CSP should contain frame-ancestors 'none'")
	}

	// Verify other security headers
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("X-Content-Type-Options should be 'nosniff'")
	}
	if rec.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("X-Frame-Options should be 'DENY'")
	}
	if rec.Header().Get("Referrer-Policy") != "strict-origin-when-cross-origin" {
		t.Error("Referrer-Policy should be 'strict-origin-when-cross-origin'")
	}
	if rec.Header().Get("X-XSS-Protection") != "1; mode=block" {
		t.Error("X-XSS-Protection should be '1; mode=block'")
	}

	// Verify the inner handler was called
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}

	// Verify Permissions-Policy header (always set)
	permPolicy := rec.Header().Get("Permissions-Policy")
	if permPolicy == "" {
		t.Error("Permissions-Policy header not set")
	}
	if !strings.Contains(permPolicy, "geolocation=()") {
		t.Error("Permissions-Policy should disable geolocation")
	}
	if !strings.Contains(permPolicy, "microphone=()") {
		t.Error("Permissions-Policy should disable microphone")
	}
	if !strings.Contains(permPolicy, "camera=()") {
		t.Error("Permissions-Policy should disable camera")
	}

	// HSTS should NOT be set when HTTPS_ONLY is not enabled
	hsts := rec.Header().Get("Strict-Transport-Security")
	if hsts != "" {
		t.Error("HSTS should not be set when HTTPS_ONLY is not enabled")
	}
}

func TestSecurityHeadersMiddlewareWithHTTPS(t *testing.T) {
	// Save and restore HTTPS_ONLY env var
	original := os.Getenv("HTTPS_ONLY")
	defer os.Setenv("HTTPS_ONLY", original)

	// Enable HTTPS_ONLY
	os.Setenv("HTTPS_ONLY", "true")

	// Create a simple handler
	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := securityHeadersMiddleware(innerHandler)
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Verify HSTS is set when HTTPS_ONLY is enabled
	hsts := rec.Header().Get("Strict-Transport-Security")
	if hsts == "" {
		t.Error("Strict-Transport-Security header should be set when HTTPS_ONLY is enabled")
	}
	if !strings.Contains(hsts, "max-age=31536000") {
		t.Error("HSTS should have max-age of 31536000 (1 year)")
	}
	if !strings.Contains(hsts, "includeSubDomains") {
		t.Error("HSTS should include includeSubDomains")
	}
}
