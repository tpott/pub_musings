package ratelimit

import (
	"math"
	"sync"
	"time"
)

// Bucket represents a token bucket for rate limiting
type Bucket struct {
	tokens     float64
	capacity   float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

// Allow checks if a request is allowed and consumes a token
func (b *Bucket) Allow(now time.Time) bool {
	elapsed := now.Sub(b.lastRefill).Seconds()

	// Refill tokens based on time elapsed
	b.tokens = math.Min(b.capacity, b.tokens+elapsed*b.refillRate)
	b.lastRefill = now

	if b.tokens >= 1.0 {
		b.tokens -= 1.0
		return true
	}
	return false
}

// Remaining returns the number of tokens remaining
func (b *Bucket) Remaining() int {
	return int(math.Floor(b.tokens))
}

// Limiter manages rate limiting for multiple identifiers
type Limiter struct {
	mu              sync.Mutex
	buckets         map[string]*Bucket
	requestsPerMin  int
	cleanupInterval time.Duration
	maxIdleTime     time.Duration
	stopCleanup     chan struct{}
}

// NewLimiter creates a new rate limiter
// requestsPerMin: number of requests allowed per minute
func NewLimiter(requestsPerMin int) *Limiter {
	limiter := &Limiter{
		buckets:         make(map[string]*Bucket),
		requestsPerMin:  requestsPerMin,
		cleanupInterval: 5 * time.Minute,
		maxIdleTime:     1 * time.Hour,
		stopCleanup:     make(chan struct{}),
	}

	// Start cleanup goroutine
	go limiter.cleanupLoop()

	return limiter
}

// Allow checks if a request from the given identifier is allowed
func (l *Limiter) Allow(identifier string) bool {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	bucket, exists := l.buckets[identifier]
	if !exists {
		// Create new bucket
		refillRate := float64(l.requestsPerMin) / 60.0 // tokens per second
		bucket = &Bucket{
			tokens:     float64(l.requestsPerMin),
			capacity:   float64(l.requestsPerMin),
			refillRate: refillRate,
			lastRefill: now,
		}
		l.buckets[identifier] = bucket
	}

	return bucket.Allow(now)
}

// Remaining returns the number of requests remaining for the given identifier
func (l *Limiter) Remaining(identifier string) int {
	l.mu.Lock()
	defer l.mu.Unlock()

	bucket, exists := l.buckets[identifier]
	if !exists {
		return l.requestsPerMin
	}

	// Update bucket tokens before checking remaining
	now := time.Now()
	elapsed := now.Sub(bucket.lastRefill).Seconds()
	bucket.tokens = math.Min(bucket.capacity, bucket.tokens+elapsed*bucket.refillRate)

	return bucket.Remaining()
}

// Limit returns the maximum requests per minute
func (l *Limiter) Limit() int {
	return l.requestsPerMin
}

// cleanupLoop periodically removes stale buckets
func (l *Limiter) cleanupLoop() {
	ticker := time.NewTicker(l.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.cleanup()
		case <-l.stopCleanup:
			return
		}
	}
}

// cleanup removes buckets that haven't been used recently
func (l *Limiter) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	for identifier, bucket := range l.buckets {
		if now.Sub(bucket.lastRefill) > l.maxIdleTime {
			delete(l.buckets, identifier)
		}
	}
}

// Close stops the cleanup goroutine
func (l *Limiter) Close() {
	close(l.stopCleanup)
}
