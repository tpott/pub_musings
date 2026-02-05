package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
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

func TestRateLimiter_MaxEntries(t *testing.T) {
	// Create a rate limiter with max 3 IPs
	limiter := NewRateLimiterWithMaxEntries(10, time.Minute, 3)

	// Add requests from 3 different IPs
	limiter.Allow("192.168.1.1")
	limiter.Allow("192.168.1.2")
	limiter.Allow("192.168.1.3")

	// Verify we have 3 entries
	limiter.mu.Lock()
	count := len(limiter.requests)
	limiter.mu.Unlock()
	if count != 3 {
		t.Errorf("Expected 3 entries, got %d", count)
	}

	// Add a 4th IP - should evict the oldest
	limiter.Allow("192.168.1.4")

	// Should still have 3 entries
	limiter.mu.Lock()
	count = len(limiter.requests)
	limiter.mu.Unlock()
	if count != 3 {
		t.Errorf("Expected 3 entries after eviction, got %d", count)
	}

	// The new IP should be tracked
	limiter.mu.Lock()
	_, hasNew := limiter.requests["192.168.1.4"]
	limiter.mu.Unlock()
	if !hasNew {
		t.Error("New IP should be tracked after eviction")
	}
}

func TestRateLimiter_MaxEntriesEvictsOldest(t *testing.T) {
	// Create a rate limiter with max 2 IPs
	limiter := NewRateLimiterWithMaxEntries(10, time.Minute, 2)

	// Add IP1 first
	limiter.Allow("192.168.1.1")
	time.Sleep(10 * time.Millisecond)

	// Add IP2 second (more recent)
	limiter.Allow("192.168.1.2")
	time.Sleep(10 * time.Millisecond)

	// Add IP3 - should evict IP1 (oldest)
	limiter.Allow("192.168.1.3")

	limiter.mu.Lock()
	_, hasIP1 := limiter.requests["192.168.1.1"]
	_, hasIP2 := limiter.requests["192.168.1.2"]
	_, hasIP3 := limiter.requests["192.168.1.3"]
	limiter.mu.Unlock()

	if hasIP1 {
		t.Error("IP1 (oldest) should have been evicted")
	}
	if !hasIP2 {
		t.Error("IP2 should still be tracked")
	}
	if !hasIP3 {
		t.Error("IP3 should be tracked")
	}
}

func TestRateLimiter_ExistingIPDoesNotTriggerEviction(t *testing.T) {
	// Create a rate limiter with max 2 IPs
	limiter := NewRateLimiterWithMaxEntries(10, time.Minute, 2)

	// Add 2 IPs
	limiter.Allow("192.168.1.1")
	limiter.Allow("192.168.1.2")

	// Make another request from IP1 (existing IP)
	limiter.Allow("192.168.1.1")

	// Should still have both IPs (no eviction needed for existing IP)
	limiter.mu.Lock()
	count := len(limiter.requests)
	_, hasIP1 := limiter.requests["192.168.1.1"]
	_, hasIP2 := limiter.requests["192.168.1.2"]
	limiter.mu.Unlock()

	if count != 2 {
		t.Errorf("Expected 2 entries, got %d", count)
	}
	if !hasIP1 {
		t.Error("IP1 should still be tracked")
	}
	if !hasIP2 {
		t.Error("IP2 should still be tracked")
	}
}

func TestRateLimiter_ConcurrentAllowAndCleanup(t *testing.T) {
	// Test that concurrent Allow() and Cleanup() calls don't cause panics or races
	limiter := NewRateLimiter(100, 100*time.Millisecond)

	var wg sync.WaitGroup

	// Goroutine 1: continuously call Allow() with different IPs
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			limiter.Allow(fmt.Sprintf("192.168.1.%d", i%256))
		}
	}()

	// Goroutine 2: continuously call Cleanup()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			limiter.Cleanup()
			time.Sleep(time.Millisecond)
		}
	}()

	// Goroutine 3: continuously call Allow() with same IP
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			limiter.Allow("10.0.0.1")
		}
	}()

	// Wait for all goroutines to complete
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	// Add timeout to prevent hanging test
	select {
	case <-done:
		// Success - no panic occurred
	case <-time.After(10 * time.Second):
		t.Fatal("Test timed out - possible deadlock")
	}
}

func TestRateLimiter_ConcurrentAllowWithEviction(t *testing.T) {
	// Test concurrent access with eviction enabled
	limiter := NewRateLimiterWithMaxEntries(100, 100*time.Millisecond, 10)

	var wg sync.WaitGroup

	// Multiple goroutines calling Allow() with different IPs (will trigger eviction)
	for g := 0; g < 5; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				ip := fmt.Sprintf("10.%d.%d.1", goroutineID, i)
				limiter.Allow(ip)
			}
		}(g)
	}

	// Wait for all goroutines
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success - verify we still have valid state
		limiter.mu.Lock()
		count := len(limiter.requests)
		limiter.mu.Unlock()
		if count > 10 {
			t.Errorf("Expected at most 10 entries due to max limit, got %d", count)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Test timed out - possible deadlock")
	}
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
