package ratelimit

import (
	"testing"
	"time"
)

func TestBucket_Allow(t *testing.T) {
	now := time.Now()
	bucket := &Bucket{
		tokens:     5.0,
		capacity:   5.0,
		refillRate: 1.0, // 1 token per second
		lastRefill: now,
	}

	// Should allow 5 requests immediately
	for i := 0; i < 5; i++ {
		if !bucket.Allow(now) {
			t.Errorf("Request %d should be allowed", i)
		}
	}

	// 6th request should be denied (no tokens left)
	if bucket.Allow(now) {
		t.Error("6th request should be denied")
	}

	// Wait 1 second, should refill 1 token
	now = now.Add(1 * time.Second)
	if !bucket.Allow(now) {
		t.Error("Request after 1 second should be allowed (1 token refilled)")
	}

	// Should be denied again
	if bucket.Allow(now) {
		t.Error("Next request should be denied")
	}

	// Wait 5 seconds, should fully refill to capacity
	now = now.Add(5 * time.Second)
	// Allow should trigger refill and succeed
	for i := 0; i < 5; i++ {
		if !bucket.Allow(now) {
			t.Errorf("After 5 seconds, request %d should be allowed (bucket should be refilled)", i)
		}
	}
}

func TestLimiter_Allow(t *testing.T) {
	// Create limiter with 5 requests per minute
	limiter := NewLimiter(5)
	defer limiter.Close()

	identifier := "test-user"

	// Should allow 5 requests
	for i := 0; i < 5; i++ {
		if !limiter.Allow(identifier) {
			t.Errorf("Request %d should be allowed", i)
		}
	}

	// 6th request should be denied
	if limiter.Allow(identifier) {
		t.Error("6th request should be denied")
	}

	// Check remaining
	remaining := limiter.Remaining(identifier)
	if remaining != 0 {
		t.Errorf("Expected 0 remaining, got %d", remaining)
	}
}

func TestLimiter_MultipleIdentifiers(t *testing.T) {
	limiter := NewLimiter(5)
	defer limiter.Close()

	// Each identifier should have independent limits
	for i := 0; i < 5; i++ {
		if !limiter.Allow("user1") {
			t.Errorf("user1 request %d should be allowed", i)
		}
		if !limiter.Allow("user2") {
			t.Errorf("user2 request %d should be allowed", i)
		}
	}

	// Both should be rate limited now
	if limiter.Allow("user1") {
		t.Error("user1 should be rate limited")
	}
	if limiter.Allow("user2") {
		t.Error("user2 should be rate limited")
	}
}

func TestLimiter_Cleanup(t *testing.T) {
	limiter := NewLimiter(5)
	limiter.cleanupInterval = 100 * time.Millisecond
	limiter.maxIdleTime = 200 * time.Millisecond
	defer limiter.Close()

	// Create buckets
	limiter.Allow("user1")
	limiter.Allow("user2")

	// Check buckets exist
	limiter.mu.Lock()
	if len(limiter.buckets) != 2 {
		t.Errorf("Expected 2 buckets, got %d", len(limiter.buckets))
	}
	limiter.mu.Unlock()

	// Wait for cleanup to run
	time.Sleep(500 * time.Millisecond)

	// Buckets should be cleaned up (idle > 200ms)
	limiter.mu.Lock()
	if len(limiter.buckets) != 0 {
		t.Errorf("Expected 0 buckets after cleanup, got %d", len(limiter.buckets))
	}
	limiter.mu.Unlock()
}

func TestLimiter_ConcurrentAccess(t *testing.T) {
	limiter := NewLimiter(100)
	defer limiter.Close()

	identifier := "concurrent-user"
	done := make(chan bool)

	// Run 10 goroutines, each making 20 requests
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 20; j++ {
				limiter.Allow(identifier)
			}
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have used 100 tokens (or more if refilling)
	// This test mainly checks for race conditions
	remaining := limiter.Remaining(identifier)
	if remaining > 100 {
		t.Errorf("Remaining should not exceed capacity, got %d", remaining)
	}
}

func TestBucket_Remaining(t *testing.T) {
	now := time.Now()
	bucket := &Bucket{
		tokens:     3.5,
		capacity:   5.0,
		refillRate: 1.0,
		lastRefill: now,
	}

	// Should return floor of tokens
	remaining := bucket.Remaining()
	if remaining != 3 {
		t.Errorf("Expected 3 remaining (floor of 3.5), got %d", remaining)
	}
}

func TestLimiter_Limit(t *testing.T) {
	limiter := NewLimiter(60)
	defer limiter.Close()

	if limiter.Limit() != 60 {
		t.Errorf("Expected limit of 60, got %d", limiter.Limit())
	}
}
