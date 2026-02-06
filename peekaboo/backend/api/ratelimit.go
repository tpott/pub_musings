// Package api provides HTTP handlers for the Peekaboo backend.
package api

import (
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// DefaultMaxEntries is the default maximum number of IPs tracked by the rate limiter.
// This prevents memory exhaustion from many unique IPs hitting the server.
const DefaultMaxEntries = 10000

// TrustProxyHeaders controls whether getClientIP trusts X-Forwarded-For and
// X-Real-IP headers. When false (default), only RemoteAddr is used for IP
// extraction. Set to true only when running behind a trusted reverse proxy.
var TrustProxyHeaders bool

// RateLimiter implements a sliding window rate limiter per IP.
type RateLimiter struct {
	mu         sync.Mutex
	requests   map[string][]time.Time
	limit      int           // max requests per window
	window     time.Duration // time window
	maxEntries int           // max IPs to track (prevents memory exhaustion)
}

// NewRateLimiter creates a new rate limiter with default max entries.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return NewRateLimiterWithMaxEntries(limit, window, DefaultMaxEntries)
}

// NewRateLimiterWithMaxEntries creates a new rate limiter with custom max entries.
// maxEntries limits how many unique IPs can be tracked to prevent memory exhaustion.
func NewRateLimiterWithMaxEntries(limit int, window time.Duration, maxEntries int) *RateLimiter {
	return &RateLimiter{
		requests:   make(map[string][]time.Time),
		limit:      limit,
		window:     window,
		maxEntries: maxEntries,
	}
}

// Allow checks if a request from the given IP should be allowed.
// Returns true if allowed, false if rate limited.
// If maxEntries is reached and this is a new IP, the oldest IP entry is evicted.
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

	// If this is a new IP and we're at max entries, evict oldest
	if len(reqs) == 0 && len(rl.requests) >= rl.maxEntries {
		rl.evictOldest(cutoff)
	}

	// Add new request
	rl.requests[ip] = append(valid, now)
	return true
}

// evictOldest removes the IP with the oldest last request time.
// Must be called with rl.mu held.
func (rl *RateLimiter) evictOldest(cutoff time.Time) {
	var oldestIP string
	var oldestTime time.Time

	for ip, reqs := range rl.requests {
		// Find the most recent request for this IP
		var mostRecent time.Time
		for _, t := range reqs {
			if t.After(mostRecent) {
				mostRecent = t
			}
		}

		// If this IP's most recent request is older than our current oldest, update
		if oldestIP == "" || mostRecent.Before(oldestTime) {
			oldestIP = ip
			oldestTime = mostRecent
		}
	}

	if oldestIP != "" {
		delete(rl.requests, oldestIP)
		slog.Info("rate limiter evicted IP due to max entries limit",
			"evicted_ip", oldestIP,
			"max_entries", rl.maxEntries)
	}
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
			slog.Warn("rate limit exceeded",
				"ip", ip,
				"method", r.Method,
				"path", r.URL.Path)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			if _, err := w.Write([]byte(`{"error":"rate limit exceeded, try again later"}`)); err != nil {
				slog.Debug("failed to write rate limit response", "error", err)
			}
			return
		}
		next.ServeHTTP(w, r)
	})
}

// getClientIP extracts the client IP from the request.
// When TrustProxyHeaders is true, checks X-Forwarded-For and X-Real-IP headers
// for proxy setups. When false (default), only uses RemoteAddr to prevent
// clients from spoofing their IP to bypass rate limiting.
func getClientIP(r *http.Request) string {
	if TrustProxyHeaders {
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
