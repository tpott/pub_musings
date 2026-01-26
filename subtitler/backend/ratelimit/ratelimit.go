// Package ratelimit provides IP-based rate limiting for HTTP endpoints.
package ratelimit

import (
	"net"
	"net/http"
	"os"
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

// Wrap returns a middleware that rate-limits requests to the given handler.
// If rate limited, responds with 429 Too Many Requests.
func (l *Limiter) Wrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := GetClientIP(r)

		if !l.Allow(ip) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"Too many requests. Please try again later."}`))
			return
		}

		next(w, r)
	}
}
