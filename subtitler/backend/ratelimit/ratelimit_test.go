package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Helper to save and restore trustProxy state
func withTrustProxy(trust bool, fn func()) {
	oldTrust := IsTrustProxy()
	SetTrustProxy(trust)
	defer SetTrustProxy(oldTrust)
	fn()
}

func TestNewLimiter(t *testing.T) {
	l := New(5, time.Minute)
	if l == nil {
		t.Fatal("expected limiter to be created")
	}
	if l.limit != 5 {
		t.Errorf("expected limit 5, got %d", l.limit)
	}
	if l.window != time.Minute {
		t.Errorf("expected window 1 minute, got %v", l.window)
	}
}

func TestAllowUnderLimit(t *testing.T) {
	l := New(5, time.Minute)
	ip := "192.168.1.1"

	// Should allow 5 requests
	for i := 0; i < 5; i++ {
		if !l.Allow(ip) {
			t.Errorf("request %d should be allowed", i+1)
		}
	}
}

func TestAllowOverLimit(t *testing.T) {
	l := New(5, time.Minute)
	ip := "192.168.1.1"

	// Use up all 5 allowed requests
	for i := 0; i < 5; i++ {
		l.Allow(ip)
	}

	// 6th request should be denied
	if l.Allow(ip) {
		t.Error("6th request should be denied")
	}
}

func TestAllowDifferentIPs(t *testing.T) {
	l := New(2, time.Minute)

	// IP 1 uses up its limit
	l.Allow("192.168.1.1")
	l.Allow("192.168.1.1")
	if l.Allow("192.168.1.1") {
		t.Error("IP 1 should be rate limited")
	}

	// IP 2 should still be allowed
	if !l.Allow("192.168.1.2") {
		t.Error("IP 2 should be allowed")
	}
}

func TestAllowWindowExpiry(t *testing.T) {
	// Use a very short window for testing
	l := New(2, 50*time.Millisecond)
	ip := "192.168.1.1"

	// Use up limit
	l.Allow(ip)
	l.Allow(ip)
	if l.Allow(ip) {
		t.Error("should be rate limited")
	}

	// Wait for window to expire
	time.Sleep(60 * time.Millisecond)

	// Should be allowed again
	if !l.Allow(ip) {
		t.Error("should be allowed after window expires")
	}
}

func TestGetClientIP(t *testing.T) {
	// Test with TRUST_PROXY disabled (default)
	t.Run("untrusted mode", func(t *testing.T) {
		withTrustProxy(false, func() {
			tests := []struct {
				name       string
				remoteAddr string
				headers    map[string]string
				expected   string
			}{
				{
					name:       "direct connection",
					remoteAddr: "192.168.1.1:12345",
					headers:    nil,
					expected:   "192.168.1.1",
				},
				{
					name:       "ignores X-Forwarded-For when untrusted",
					remoteAddr: "10.0.0.1:12345",
					headers:    map[string]string{"X-Forwarded-For": "203.0.113.50"},
					expected:   "10.0.0.1",
				},
				{
					name:       "ignores X-Real-IP when untrusted",
					remoteAddr: "10.0.0.1:12345",
					headers:    map[string]string{"X-Real-IP": "203.0.113.100"},
					expected:   "10.0.0.1",
				},
				{
					name:       "ignores all proxy headers when untrusted",
					remoteAddr: "10.0.0.1:12345",
					headers: map[string]string{
						"X-Forwarded-For": "203.0.113.50",
						"X-Real-IP":       "203.0.113.100",
					},
					expected: "10.0.0.1",
				},
				{
					name:       "RemoteAddr without port",
					remoteAddr: "192.168.1.1",
					headers:    nil,
					expected:   "192.168.1.1",
				},
				{
					name:       "IPv6 address",
					remoteAddr: "[::1]:12345",
					headers:    nil,
					expected:   "::1",
				},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					r := httptest.NewRequest("GET", "/", nil)
					r.RemoteAddr = tt.remoteAddr
					for k, v := range tt.headers {
						r.Header.Set(k, v)
					}

					ip := GetClientIP(r)
					if ip != tt.expected {
						t.Errorf("expected %q, got %q", tt.expected, ip)
					}
				})
			}
		})
	})

	// Test with TRUST_PROXY enabled
	t.Run("trusted mode", func(t *testing.T) {
		withTrustProxy(true, func() {
			tests := []struct {
				name       string
				remoteAddr string
				headers    map[string]string
				expected   string
			}{
				{
					name:       "direct connection",
					remoteAddr: "192.168.1.1:12345",
					headers:    nil,
					expected:   "192.168.1.1",
				},
				{
					name:       "X-Forwarded-For single",
					remoteAddr: "10.0.0.1:12345",
					headers:    map[string]string{"X-Forwarded-For": "203.0.113.50"},
					expected:   "203.0.113.50",
				},
				{
					name:       "X-Forwarded-For multiple",
					remoteAddr: "10.0.0.1:12345",
					headers:    map[string]string{"X-Forwarded-For": "203.0.113.50, 70.41.3.18, 150.172.238.178"},
					expected:   "203.0.113.50",
				},
				{
					name:       "X-Real-IP",
					remoteAddr: "10.0.0.1:12345",
					headers:    map[string]string{"X-Real-IP": "203.0.113.100"},
					expected:   "203.0.113.100",
				},
				{
					name:       "X-Forwarded-For takes precedence",
					remoteAddr: "10.0.0.1:12345",
					headers: map[string]string{
						"X-Forwarded-For": "203.0.113.50",
						"X-Real-IP":       "203.0.113.100",
					},
					expected: "203.0.113.50",
				},
				{
					name:       "RemoteAddr without port",
					remoteAddr: "192.168.1.1",
					headers:    nil,
					expected:   "192.168.1.1",
				},
				{
					name:       "IPv6 address",
					remoteAddr: "[::1]:12345",
					headers:    nil,
					expected:   "::1",
				},
			}

			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					r := httptest.NewRequest("GET", "/", nil)
					r.RemoteAddr = tt.remoteAddr
					for k, v := range tt.headers {
						r.Header.Set(k, v)
					}

					ip := GetClientIP(r)
					if ip != tt.expected {
						t.Errorf("expected %q, got %q", tt.expected, ip)
					}
				})
			}
		})
	})
}

func TestTrustProxySetterGetter(t *testing.T) {
	// Save original state
	original := IsTrustProxy()
	defer SetTrustProxy(original)

	// Test SetTrustProxy(true)
	SetTrustProxy(true)
	if !IsTrustProxy() {
		t.Error("expected IsTrustProxy to return true after SetTrustProxy(true)")
	}

	// Test SetTrustProxy(false)
	SetTrustProxy(false)
	if IsTrustProxy() {
		t.Error("expected IsTrustProxy to return false after SetTrustProxy(false)")
	}
}

func TestGetClientIPSpoofingPrevention(t *testing.T) {
	// This test verifies that when TRUST_PROXY is disabled,
	// malicious X-Forwarded-For headers cannot spoof the client IP
	withTrustProxy(false, func() {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = "192.168.1.1:12345"
		// Attacker tries to spoof their IP
		r.Header.Set("X-Forwarded-For", "10.10.10.10")

		ip := GetClientIP(r)
		if ip != "192.168.1.1" {
			t.Errorf("IP spoofing succeeded: expected 192.168.1.1, got %q", ip)
		}
	})
}

func TestWrapAllowed(t *testing.T) {
	l := New(5, time.Minute)
	called := false

	handler := l.Wrap(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()

	handler(w, req)

	if !called {
		t.Error("handler should have been called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestWrapRateLimited(t *testing.T) {
	l := New(2, time.Minute)
	callCount := 0

	handler := l.Wrap(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"

	// First two requests should succeed
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		handler(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("request %d: expected status 200, got %d", i+1, w.Code)
		}
	}

	// Third request should be rate limited
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", w.Code)
	}
	if callCount != 2 {
		t.Errorf("expected handler to be called 2 times, got %d", callCount)
	}
	if w.Header().Get("Retry-After") != "60" {
		t.Errorf("expected Retry-After header to be 60")
	}
}

func TestWrapDifferentIPs(t *testing.T) {
	l := New(1, time.Minute)

	handler := l.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Request from IP 1
	req1 := httptest.NewRequest("POST", "/test", nil)
	req1.RemoteAddr = "192.168.1.1:12345"
	w1 := httptest.NewRecorder()
	handler(w1, req1)
	if w1.Code != http.StatusOK {
		t.Errorf("IP 1 first request: expected 200, got %d", w1.Code)
	}

	// IP 1 should now be limited
	w2 := httptest.NewRecorder()
	handler(w2, req1)
	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("IP 1 second request: expected 429, got %d", w2.Code)
	}

	// IP 2 should still work
	req2 := httptest.NewRequest("POST", "/test", nil)
	req2.RemoteAddr = "192.168.1.2:12345"
	w3 := httptest.NewRecorder()
	handler(w3, req2)
	if w3.Code != http.StatusOK {
		t.Errorf("IP 2 first request: expected 200, got %d", w3.Code)
	}
}

// --- UserLimiter tests ---

func TestNewUserLimiter(t *testing.T) {
	ul := NewUserLimiter(60, time.Minute)
	if ul == nil {
		t.Fatal("expected user limiter to be created")
	}
	if ul.GetLimit() != 60 {
		t.Errorf("expected limit 60, got %d", ul.GetLimit())
	}
}

func TestUserLimiterAllowUser(t *testing.T) {
	ul := NewUserLimiter(5, time.Minute)
	userID := "user123"

	// Should allow 5 requests
	for i := 0; i < 5; i++ {
		if !ul.AllowUser(userID) {
			t.Errorf("request %d should be allowed", i+1)
		}
	}

	// 6th request should be denied
	if ul.AllowUser(userID) {
		t.Error("6th request should be denied")
	}
}

func TestUserLimiterDifferentUsers(t *testing.T) {
	ul := NewUserLimiter(2, time.Minute)

	// User 1 uses up limit
	ul.AllowUser("user1")
	ul.AllowUser("user1")
	if ul.AllowUser("user1") {
		t.Error("user1 should be rate limited")
	}

	// User 2 should still be allowed
	if !ul.AllowUser("user2") {
		t.Error("user2 should be allowed")
	}
}

func TestUserLimiterRemaining(t *testing.T) {
	ul := NewUserLimiter(5, time.Minute)
	userID := "user123"

	// Initially should have 5 remaining
	if remaining := ul.RemainingUser(userID); remaining != 5 {
		t.Errorf("expected 5 remaining, got %d", remaining)
	}

	// After one request, should have 4 remaining
	ul.AllowUser(userID)
	if remaining := ul.RemainingUser(userID); remaining != 4 {
		t.Errorf("expected 4 remaining, got %d", remaining)
	}

	// Use up all remaining
	for i := 0; i < 4; i++ {
		ul.AllowUser(userID)
	}

	// Should have 0 remaining
	if remaining := ul.RemainingUser(userID); remaining != 0 {
		t.Errorf("expected 0 remaining, got %d", remaining)
	}
}

func TestUserLimiterWrapWithUserLimit(t *testing.T) {
	ul := NewUserLimiter(2, time.Minute)
	called := 0

	// Mock function that returns user ID
	getUserID := func(r *http.Request) string {
		return r.Header.Get("X-User-ID")
	}

	handler := ul.WrapWithUserLimit(getUserID, func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusOK)
	})

	// Create request with user ID
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-User-ID", "user123")

	// First two requests should succeed
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		handler(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("request %d: expected status 200, got %d", i+1, w.Code)
		}
		// Check rate limit headers are set
		if w.Header().Get("X-RateLimit-Limit") != "2" {
			t.Errorf("expected X-RateLimit-Limit header to be 2, got %s", w.Header().Get("X-RateLimit-Limit"))
		}
	}

	// Third request should be rate limited
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 429, got %d", w.Code)
	}
	if called != 2 {
		t.Errorf("expected handler to be called 2 times, got %d", called)
	}
	// Check remaining header shows 0
	if w.Header().Get("X-RateLimit-Remaining") != "0" {
		t.Errorf("expected X-RateLimit-Remaining header to be 0, got %s", w.Header().Get("X-RateLimit-Remaining"))
	}
}

func TestUserLimiterSkipsAnonymous(t *testing.T) {
	ul := NewUserLimiter(1, time.Minute)
	called := 0

	// Mock function that returns empty string for anonymous
	getUserID := func(r *http.Request) string {
		return "" // No user ID = anonymous
	}

	handler := ul.WrapWithUserLimit(getUserID, func(w http.ResponseWriter, r *http.Request) {
		called++
		w.WriteHeader(http.StatusOK)
	})

	// Anonymous requests should not be rate limited by user limiter
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		handler(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("request %d: expected status 200, got %d", i+1, w.Code)
		}
	}

	if called != 5 {
		t.Errorf("expected handler to be called 5 times, got %d", called)
	}
}
