package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Tests for getClientIP — IP extraction from HTTP requests.

func TestGetClientIP_TrustProxy(t *testing.T) {
	// Save and restore the global setting
	origTrust := TrustProxyHeaders
	defer func() { TrustProxyHeaders = origTrust }()

	TrustProxyHeaders = true

	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		xri        string
		expected   string
	}{
		{
			name:       "RemoteAddr only",
			remoteAddr: "192.168.1.1:12345",
			expected:   "192.168.1.1",
		},
		{
			name:       "X-Forwarded-For single",
			remoteAddr: "10.0.0.1:12345",
			xff:        "192.168.1.1",
			expected:   "192.168.1.1",
		},
		{
			name:       "X-Forwarded-For multiple",
			remoteAddr: "10.0.0.1:12345",
			xff:        "192.168.1.1, 10.0.0.1",
			expected:   "192.168.1.1",
		},
		{
			name:       "X-Real-IP",
			remoteAddr: "10.0.0.1:12345",
			xri:        "192.168.1.1",
			expected:   "192.168.1.1",
		},
		{
			name:       "X-Forwarded-For takes precedence",
			remoteAddr: "10.0.0.1:12345",
			xff:        "1.1.1.1",
			xri:        "2.2.2.2",
			expected:   "1.1.1.1",
		},
		{
			name:       "X-Forwarded-For with spaces",
			remoteAddr: "10.0.0.1:12345",
			xff:        "  192.168.1.1 , 10.0.0.1",
			expected:   "192.168.1.1",
		},
		{
			name:       "invalid X-Real-IP falls back to RemoteAddr",
			remoteAddr: "10.0.0.1:12345",
			xri:        "not-an-ip",
			expected:   "10.0.0.1",
		},
		{
			name:       "invalid X-Forwarded-For falls back to RemoteAddr",
			remoteAddr: "10.0.0.1:12345",
			xff:        "garbage, 10.0.0.1",
			expected:   "10.0.0.1",
		},
		{
			name:       "invalid X-Forwarded-For falls through to valid X-Real-IP",
			remoteAddr: "10.0.0.1:12345",
			xff:        "not-valid",
			xri:        "192.168.1.1",
			expected:   "192.168.1.1",
		},
		{
			name:       "both headers invalid falls back to RemoteAddr",
			remoteAddr: "10.0.0.1:12345",
			xff:        "abc",
			xri:        "def",
			expected:   "10.0.0.1",
		},
		{
			name:       "X-Forwarded-For IPv6",
			remoteAddr: "10.0.0.1:12345",
			xff:        "::1",
			expected:   "::1",
		},
		{
			name:       "X-Real-IP IPv6",
			remoteAddr: "10.0.0.1:12345",
			xri:        "2001:db8::1",
			expected:   "2001:db8::1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xri != "" {
				req.Header.Set("X-Real-IP", tt.xri)
			}

			ip := getClientIP(req)
			if ip != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, ip)
			}
		})
	}
}

func TestGetClientIP_NoTrustProxy(t *testing.T) {
	// Save and restore the global setting
	origTrust := TrustProxyHeaders
	defer func() { TrustProxyHeaders = origTrust }()

	TrustProxyHeaders = false

	tests := []struct {
		name       string
		remoteAddr string
		xff        string
		xri        string
		expected   string
	}{
		{
			name:       "uses RemoteAddr",
			remoteAddr: "192.168.1.1:12345",
			expected:   "192.168.1.1",
		},
		{
			name:       "ignores X-Forwarded-For",
			remoteAddr: "10.0.0.1:12345",
			xff:        "192.168.1.1",
			expected:   "10.0.0.1",
		},
		{
			name:       "ignores X-Real-IP",
			remoteAddr: "10.0.0.1:12345",
			xri:        "192.168.1.1",
			expected:   "10.0.0.1",
		},
		{
			name:       "ignores both headers",
			remoteAddr: "10.0.0.1:12345",
			xff:        "1.1.1.1",
			xri:        "2.2.2.2",
			expected:   "10.0.0.1",
		},
		{
			name:       "handles RemoteAddr without port",
			remoteAddr: "10.0.0.1",
			expected:   "10.0.0.1",
		},
		{
			name:       "handles IPv6 with port",
			remoteAddr: "[::1]:12345",
			expected:   "::1",
		},
		{
			name:       "handles IPv6 without port",
			remoteAddr: "::1",
			expected:   "::1",
		},
		{
			name:       "handles full IPv6 with port",
			remoteAddr: "[2001:db8::1]:8080",
			expected:   "2001:db8::1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = tt.remoteAddr
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xri != "" {
				req.Header.Set("X-Real-IP", tt.xri)
			}

			ip := getClientIP(req)
			if ip != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, ip)
			}
		})
	}
}
