package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestSecurityHeadersMiddleware(t *testing.T) {
	// Create a simple handler that returns 200 OK
	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with security headers middleware
	handler := SecurityHeadersMiddleware(innerHandler)

	// Create test request
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	// Call handler
	handler.ServeHTTP(rec, req)

	// Verify response
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	// Verify security headers
	tests := []struct {
		header string
		want   string
	}{
		{"Content-Security-Policy", ContentSecurityPolicy},
		{"X-Frame-Options", "DENY"},
		{"X-Content-Type-Options", "nosniff"},
		{"X-XSS-Protection", "1; mode=block"},
		{"Referrer-Policy", "strict-origin-when-cross-origin"},
		{"Permissions-Policy", PermissionsPolicy},
	}

	for _, tt := range tests {
		got := rec.Header().Get(tt.header)
		if got != tt.want {
			t.Errorf("header %s = %q, want %q", tt.header, got, tt.want)
		}
	}
}

func TestSecurityHeadersMiddleware_PassesThrough(t *testing.T) {
	// Verify the middleware passes the request through to the inner handler
	called := false
	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
	})

	handler := SecurityHeadersMiddleware(innerHandler)

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("inner handler was not called")
	}

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rec.Code)
	}
}

func TestContentSecurityPolicy_Directives(t *testing.T) {
	// Verify CSP contains all required directives
	directives := []string{
		"default-src 'self'",
		"script-src 'self'",
		"style-src 'self' 'unsafe-inline'",
		"img-src 'self' data: blob:",
		"media-src 'self' blob:",
		"connect-src 'self'",
		"frame-ancestors 'none'",
	}

	for _, directive := range directives {
		if !strings.Contains(ContentSecurityPolicy, directive) {
			t.Errorf("CSP missing directive: %q", directive)
		}
	}
}

func TestSecurityHeadersMiddleware_HSTS_WhenHTTPSOnly(t *testing.T) {
	os.Setenv("HTTPS_ONLY", "true")
	defer os.Unsetenv("HTTPS_ONLY")

	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := SecurityHeadersMiddleware(innerHandler)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	got := rec.Header().Get("Strict-Transport-Security")
	want := "max-age=31536000; includeSubDomains"
	if got != want {
		t.Errorf("HSTS header = %q, want %q", got, want)
	}
}

func TestSecurityHeadersMiddleware_NoHSTS_WhenNotHTTPSOnly(t *testing.T) {
	os.Unsetenv("HTTPS_ONLY")

	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := SecurityHeadersMiddleware(innerHandler)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	got := rec.Header().Get("Strict-Transport-Security")
	if got != "" {
		t.Errorf("HSTS header should be absent when HTTPS_ONLY is not set, got %q", got)
	}
}

func TestPermissionsPolicy_Directives(t *testing.T) {
	// Verify microphone is allowed (required for voice input)
	if !strings.Contains(PermissionsPolicy, "microphone=(self)") {
		t.Error("PermissionsPolicy should allow microphone for self")
	}

	// Verify other features are denied
	denied := []string{"camera=()", "geolocation=()", "payment=()", "usb=()"}
	for _, d := range denied {
		if !strings.Contains(PermissionsPolicy, d) {
			t.Errorf("PermissionsPolicy missing denied feature: %q", d)
		}
	}
}
