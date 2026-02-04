package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiter_Allow(t *testing.T) {
	limiter := NewRateLimiter(3, time.Minute)

	// First 3 requests should be allowed
	for i := 0; i < 3; i++ {
		if !limiter.Allow("192.168.1.1") {
			t.Errorf("Request %d should be allowed", i+1)
		}
	}

	// 4th request should be denied
	if limiter.Allow("192.168.1.1") {
		t.Error("4th request should be rate limited")
	}

	// Different IP should still be allowed
	if !limiter.Allow("192.168.1.2") {
		t.Error("Request from different IP should be allowed")
	}
}

func TestRateLimiter_WindowExpiry(t *testing.T) {
	// Use a very short window for testing
	limiter := NewRateLimiter(2, 50*time.Millisecond)

	// Use up the limit
	limiter.Allow("192.168.1.1")
	limiter.Allow("192.168.1.1")

	// Should be rate limited
	if limiter.Allow("192.168.1.1") {
		t.Error("Should be rate limited")
	}

	// Wait for window to expire
	time.Sleep(60 * time.Millisecond)

	// Should be allowed again
	if !limiter.Allow("192.168.1.1") {
		t.Error("Should be allowed after window expires")
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	limiter := NewRateLimiter(10, 50*time.Millisecond)

	// Add some requests
	limiter.Allow("192.168.1.1")
	limiter.Allow("192.168.1.2")

	// Wait for window to expire
	time.Sleep(60 * time.Millisecond)

	// Cleanup should remove old entries
	limiter.Cleanup()

	// Check that entries were cleaned up by verifying map is empty
	limiter.mu.Lock()
	count := len(limiter.requests)
	limiter.mu.Unlock()

	if count != 0 {
		t.Errorf("Expected 0 entries after cleanup, got %d", count)
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	limiter := NewRateLimiter(2, time.Minute)

	// Create a simple handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Wrap with rate limiter
	limited := RateLimitMiddleware(handler, limiter)

	t.Run("allows requests under limit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		w := httptest.NewRecorder()

		limited.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", w.Code)
		}
	})

	t.Run("returns 429 when rate limited", func(t *testing.T) {
		// Make requests up to and over the limit
		for i := 0; i < 3; i++ {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = "192.168.1.200:12345"
			w := httptest.NewRecorder()

			limited.ServeHTTP(w, req)

			if i < 2 {
				if w.Code != http.StatusOK {
					t.Errorf("Request %d: expected 200, got %d", i+1, w.Code)
				}
			} else {
				if w.Code != http.StatusTooManyRequests {
					t.Errorf("Request %d: expected 429, got %d", i+1, w.Code)
				}

				// Verify response format
				var resp struct {
					Error string `json:"error"`
				}
				if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}
				if resp.Error != "rate limit exceeded, try again later" {
					t.Errorf("Unexpected error message: %q", resp.Error)
				}

				// Verify Retry-After header
				if w.Header().Get("Retry-After") != "60" {
					t.Errorf("Expected Retry-After: 60, got %q", w.Header().Get("Retry-After"))
				}
			}
		}
	})
}

func TestGetClientIP(t *testing.T) {
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
