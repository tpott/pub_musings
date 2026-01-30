package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/tpott/subtitler/backend/auth"
)

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
	auth.InitHTTPSOnly()
	defer auth.InitHTTPSOnly() // re-initialize after restoring env var

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
