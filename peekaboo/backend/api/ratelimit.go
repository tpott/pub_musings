// Package api provides HTTP handlers for the Peekaboo backend.
package api

import (
	"net/http"
	"sync"
	"time"
)

// RateLimiter implements a sliding window rate limiter per IP.
type RateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int           // max requests per window
	window   time.Duration // time window
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

// Allow checks if a request from the given IP should be allowed.
// Returns true if allowed, false if rate limited.
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Get existing requests and filter out old ones
	reqs := rl.requests[ip]
	var valid []time.Time
	for _, t := range reqs {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	// Check if under limit
	if len(valid) >= rl.limit {
		rl.requests[ip] = valid
		return false
	}

	// Add new request
	rl.requests[ip] = append(valid, now)
	return true
}

// Cleanup removes old entries from the rate limiter.
// Call this periodically to prevent memory growth.
func (rl *RateLimiter) Cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-rl.window)
	for ip, reqs := range rl.requests {
		var valid []time.Time
		for _, t := range reqs {
			if t.After(cutoff) {
				valid = append(valid, t)
			}
		}
		if len(valid) == 0 {
			delete(rl.requests, ip)
		} else {
			rl.requests[ip] = valid
		}
	}
}

// RateLimitMiddleware wraps an http.Handler with rate limiting.
func RateLimitMiddleware(next http.Handler, limiter *RateLimiter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := getClientIP(r)
		if !limiter.Allow(ip) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate limit exceeded, try again later"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// getClientIP extracts the client IP from the request.
// Handles X-Forwarded-For and X-Real-IP headers for proxy setups.
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For first (comma-separated list, first is client)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the list
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}

	// Check X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr (may include port)
	addr := r.RemoteAddr
	// Strip port if present
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i]
		}
	}
	return addr
}
