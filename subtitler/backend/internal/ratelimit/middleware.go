package ratelimit

import (
	"net/http"
	"strconv"
	"strings"
)

// GetIPFromRequest extracts the client IP from the request
// Checks X-Forwarded-For header first (for proxied requests), then RemoteAddr
func GetIPFromRequest(r *http.Request) string {
	// Check X-Forwarded-For header (set by proxies)
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		// X-Forwarded-For can contain multiple IPs (client, proxy1, proxy2, ...)
		// We want the first one (the client)
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header (alternative proxy header)
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Fall back to RemoteAddr
	// RemoteAddr is in format "IP:port", extract just the IP
	ip := r.RemoteAddr
	if colon := strings.LastIndex(ip, ":"); colon != -1 {
		ip = ip[:colon]
	}

	return ip
}

// GetUserIDFromRequest extracts the user ID from the request context
// Returns empty string if not authenticated
func GetUserIDFromRequest(r *http.Request) string {
	// User ID is set in context by auth middleware
	if userID := r.Context().Value("user_id"); userID != nil {
		if id, ok := userID.(int); ok {
			return strconv.Itoa(id)
		}
	}
	return ""
}

// Middleware creates a rate limiting middleware
// identifier function extracts the identifier from the request (IP or user ID)
func Middleware(limiter *Limiter, identifier func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := identifier(r)

			// Set rate limit headers
			limit := limiter.Limit()
			remaining := limiter.Remaining(id)

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

			// Check if request is allowed
			if !limiter.Allow(id) {
				w.Header().Set("Retry-After", "60")
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// IPMiddleware creates a rate limiting middleware that uses IP address as identifier
func IPMiddleware(limiter *Limiter) func(http.Handler) http.Handler {
	return Middleware(limiter, GetIPFromRequest)
}

// UserMiddleware creates a rate limiting middleware that uses user ID as identifier
// Falls back to IP if user is not authenticated
func UserMiddleware(limiter *Limiter) func(http.Handler) http.Handler {
	return Middleware(limiter, func(r *http.Request) string {
		if userID := GetUserIDFromRequest(r); userID != "" {
			return "user:" + userID
		}
		return "ip:" + GetIPFromRequest(r)
	})
}
