// Package ratelimit provides IP-based rate limiting for HTTP endpoints.
package ratelimit

import (
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// trustProxy indicates whether to trust X-Forwarded-For and X-Real-IP headers.
// This should only be enabled when behind a trusted reverse proxy (e.g., Caddy, nginx).
// Set TRUST_PROXY=true or TRUST_PROXY=1 environment variable to enable.
var trustProxy = os.Getenv("TRUST_PROXY") == "true" || os.Getenv("TRUST_PROXY") == "1"

// SetTrustProxy allows programmatic control of proxy trust (mainly for testing).
func SetTrustProxy(trust bool) {
	trustProxy = trust
}

// IsTrustProxy returns whether proxy headers are trusted.
func IsTrustProxy() bool {
	return trustProxy
}

// Limiter tracks request counts per IP within a sliding time window.
type Limiter struct {
	requests map[string][]time.Time
	mu       sync.RWMutex
	limit    int           // max requests per window
	window   time.Duration // sliding window duration
}

// New creates a new rate limiter with the specified limit and window.
// Example: New(5, time.Minute) allows 5 requests per minute per IP.
func New(limit int, window time.Duration) *Limiter {
	l := &Limiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
	// Start background cleanup goroutine
	go l.cleanup()
	return l
}

// Allow checks if the given IP is allowed to make another request.
// Returns true if under the limit, false if rate limited.
func (l *Limiter) Allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.window)

	// Get existing requests and filter old ones
	requests := l.requests[ip]
	var valid []time.Time
	for _, t := range requests {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	// Check if under limit
	if len(valid) >= l.limit {
		l.requests[ip] = valid
		return false
	}

	// Add this request
	valid = append(valid, now)
	l.requests[ip] = valid
	return true
}

// cleanup periodically removes old entries to prevent memory growth.
func (l *Limiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		l.mu.Lock()
		now := time.Now()
		cutoff := now.Add(-l.window)

		for ip, requests := range l.requests {
			var valid []time.Time
			for _, t := range requests {
				if t.After(cutoff) {
					valid = append(valid, t)
				}
			}
			if len(valid) == 0 {
				delete(l.requests, ip)
			} else {
				l.requests[ip] = valid
			}
		}
		l.mu.Unlock()
	}
}

// GetClientIP extracts the client IP address from an HTTP request.
// When TRUST_PROXY is enabled, it checks X-Forwarded-For and X-Real-IP headers
// for proxied requests. Otherwise, it only uses RemoteAddr to prevent IP spoofing.
func GetClientIP(r *http.Request) string {
	// Only trust proxy headers when explicitly configured
	if trustProxy {
		// Check X-Forwarded-For header (may contain multiple IPs)
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			// Take the first IP (original client)
			parts := strings.Split(forwarded, ",")
			return strings.TrimSpace(parts[0])
		}

		// Check X-Real-IP header
		if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
			return realIP
		}
	}

	// Fall back to RemoteAddr (strip port)
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// RemoteAddr might not have a port
		return r.RemoteAddr
	}
	return ip
}

// Remaining returns the number of remaining requests allowed for the given key.
func (l *Limiter) Remaining(key string) int {
	l.mu.RLock()
	defer l.mu.RUnlock()

	now := time.Now()
	cutoff := now.Add(-l.window)

	requests := l.requests[key]
	var count int
	for _, t := range requests {
		if t.After(cutoff) {
			count++
		}
	}

	remaining := l.limit - count
	if remaining < 0 {
		return 0
	}
	return remaining
}

// GetLimit returns the configured limit for this limiter.
func (l *Limiter) GetLimit() int {
	return l.limit
}

// RateLimitCallback is called when a rate limit is exceeded.
// Parameters: context, IP, endpoint name
type RateLimitCallback func(r *http.Request, ip string, endpoint string)

// OnLimitExceeded is called when a rate limit is exceeded. Set this to add
// custom logging for rate limit violations.
var OnLimitExceeded RateLimitCallback

// Wrap returns a middleware that rate-limits requests to the given handler.
// If rate limited, responds with 429 Too Many Requests.
func (l *Limiter) Wrap(next http.HandlerFunc) http.HandlerFunc {
	return l.WrapNamed("", next)
}

// WrapNamed returns a middleware that rate-limits requests with a named endpoint.
// The endpoint name is used for logging/metrics when rate limits are exceeded.
func (l *Limiter) WrapNamed(endpoint string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := GetClientIP(r)

		if !l.Allow(ip) {
			if OnLimitExceeded != nil {
				OnLimitExceeded(r, ip, endpoint)
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"Too many requests. Please try again later."}`))
			return
		}

		next(w, r)
	}
}

// UserLimiter provides per-user rate limiting for authenticated requests.
// It uses user ID as the key for authenticated users and falls back to IP for anonymous users.
type UserLimiter struct {
	limiter *Limiter
}

// NewUserLimiter creates a new per-user rate limiter with the specified limit and window.
// Example: NewUserLimiter(60, time.Minute) allows 60 requests per minute per user.
func NewUserLimiter(limit int, window time.Duration) *UserLimiter {
	return &UserLimiter{
		limiter: New(limit, window),
	}
}

// AllowUser checks if the given user ID is allowed to make another request.
// Returns true if under the limit, false if rate limited.
func (ul *UserLimiter) AllowUser(userID string) bool {
	key := "user:" + userID
	return ul.limiter.Allow(key)
}

// RemainingUser returns the number of remaining requests for the given user ID.
func (ul *UserLimiter) RemainingUser(userID string) int {
	key := "user:" + userID
	return ul.limiter.Remaining(key)
}

// GetLimit returns the configured limit for this limiter.
func (ul *UserLimiter) GetLimit() int {
	return ul.limiter.GetLimit()
}

// UserRateLimitFunc is a function type for extracting user ID from a request.
// It should return the user ID if authenticated, or empty string if not.
type UserRateLimitFunc func(r *http.Request) string

// OnUserLimitExceeded is called when a per-user rate limit is exceeded.
// Parameters: request, user ID, endpoint name
var OnUserLimitExceeded func(r *http.Request, userID string, endpoint string)

// WrapWithUserLimit returns a middleware that applies per-user rate limiting.
// It uses the provided function to extract the user ID from the request.
// If the user is not authenticated (empty userID), the request is allowed through
// without per-user rate limiting (IP-based limiting should be applied separately).
// Headers X-RateLimit-Limit and X-RateLimit-Remaining are added to responses.
func (ul *UserLimiter) WrapWithUserLimit(getUserID UserRateLimitFunc, next http.HandlerFunc) http.HandlerFunc {
	return ul.WrapWithUserLimitNamed("", getUserID, next)
}

// WrapWithUserLimitNamed returns a middleware that applies per-user rate limiting with a named endpoint.
func (ul *UserLimiter) WrapWithUserLimitNamed(endpoint string, getUserID UserRateLimitFunc, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := getUserID(r)

		// If not authenticated, skip per-user rate limiting
		// (IP-based limiting should be applied separately)
		if userID == "" {
			next(w, r)
			return
		}

		// Add rate limit headers
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(ul.GetLimit()))

		if !ul.AllowUser(userID) {
			if OnUserLimitExceeded != nil {
				OnUserLimitExceeded(r, userID, endpoint)
			}
			remaining := ul.RemainingUser(userID)
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"Rate limit exceeded. Please try again later."}`))
			return
		}

		remaining := ul.RemainingUser(userID)
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

		next(w, r)
	}
}
