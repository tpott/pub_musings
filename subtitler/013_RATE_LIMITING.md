# 013_RATE_LIMITING

## Overview

Add rate limiting middleware to protect API endpoints from abuse. This will prevent:
- Brute force attacks on authentication endpoints
- Spam registrations
- Resource exhaustion from excessive uploads
- DoS attacks via rapid API calls

## Implementation Plan

### 1. Rate Limiting Strategy

**Approach:** Token bucket algorithm with in-memory storage (simple, no external dependencies)

**Rate Limits by Endpoint Type:**
- **Authentication endpoints** (`/api/register`, `/api/login`): 5 requests/minute per IP
- **Upload endpoints** (`/api/upload`, `/api/transcribe`): 10 requests/hour per authenticated user
- **Public endpoints** (`/api/health`, `/api/analytics/events`): 100 requests/minute per IP
- **Other protected endpoints**: 60 requests/minute per authenticated user

**Key Decision:** Use IP-based rate limiting for unauthenticated endpoints, user ID-based for authenticated endpoints.

### 2. Implementation Steps

#### Step 1: Create rate limiter package
- File: `backend/internal/ratelimit/ratelimit.go`
- Implement token bucket algorithm
- In-memory store with cleanup (map + mutex)
- Configurable limits per identifier

#### Step 2: Create rate limit middleware
- File: `backend/internal/ratelimit/middleware.go`
- HTTP middleware wrapper
- Extract identifier (IP or user ID from context)
- Return 429 Too Many Requests when limit exceeded
- Add rate limit headers (X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset)

#### Step 3: Apply middleware to endpoints
- File: `backend/cmd/server/main.go`
- Apply to auth endpoints (5 req/min per IP)
- Apply to upload endpoints (10 req/hour per user)
- Apply to public endpoints (100 req/min per IP)

#### Step 4: Add configuration
- File: `backend/internal/config/config.go`
- Add rate limit configuration options
- Environment variables for custom limits
- Defaults as specified above

#### Step 5: Add tests
- File: `backend/internal/ratelimit/ratelimit_test.go`
- Test token bucket logic
- Test rate limiter with concurrent requests
- File: `backend/cmd/server/ratelimit_test.go`
- Integration test: rapid requests to /api/register return 429

### 3. Token Bucket Algorithm

```go
type Bucket struct {
    tokens      float64
    capacity    float64
    refillRate  float64  // tokens per second
    lastRefill  time.Time
}

// Allow checks if request is allowed and consumes a token
func (b *Bucket) Allow() bool {
    now := time.Now()
    elapsed := now.Sub(b.lastRefill).Seconds()

    // Refill tokens based on time elapsed
    b.tokens = math.Min(b.capacity, b.tokens + elapsed * b.refillRate)
    b.lastRefill = now

    if b.tokens >= 1.0 {
        b.tokens -= 1.0
        return true
    }
    return false
}
```

### 4. Middleware Implementation

```go
func RateLimitMiddleware(limiter *RateLimiter, getIdentifier func(*http.Request) string) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            identifier := getIdentifier(r)

            if !limiter.Allow(identifier) {
                w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limiter.Limit))
                w.Header().Set("X-RateLimit-Remaining", "0")
                w.Header().Set("Retry-After", "60")
                http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}
```

### 5. Files to Create/Modify

**New Files:**
- `backend/internal/ratelimit/ratelimit.go` - Token bucket implementation
- `backend/internal/ratelimit/middleware.go` - HTTP middleware
- `backend/internal/ratelimit/ratelimit_test.go` - Unit tests

**Modified Files:**
- `backend/cmd/server/main.go` - Apply middleware to endpoints
- `backend/internal/config/config.go` - Add rate limit configuration
- `backend/cmd/server/ratelimit_test.go` - Integration tests

### 6. Configuration

Add to config.go:
```go
type RateLimitConfig struct {
    AuthRequestsPerMinute   int  // Default: 5
    UploadRequestsPerHour   int  // Default: 10
    PublicRequestsPerMinute int  // Default: 100
    DefaultRequestsPerMinute int // Default: 60
}
```

Environment variables:
- `RATE_LIMIT_AUTH_PER_MIN=5`
- `RATE_LIMIT_UPLOAD_PER_HOUR=10`
- `RATE_LIMIT_PUBLIC_PER_MIN=100`
- `RATE_LIMIT_DEFAULT_PER_MIN=60`

### 7. Cleanup Strategy

In-memory store needs periodic cleanup to prevent memory leaks:
- Run cleanup goroutine every 5 minutes
- Remove buckets with no activity for 1 hour
- Log cleanup stats for monitoring

### 8. Testing Strategy

**Unit Tests:**
1. Token bucket refills correctly over time
2. Multiple requests consume tokens correctly
3. Requests are blocked when tokens exhausted
4. Cleanup removes stale buckets

**Integration Tests:**
1. Rapid requests to /api/register are rate limited (5 req/min)
2. Authenticated upload requests are rate limited (10 req/hour)
3. Rate limit headers are returned correctly
4. 429 status code is returned when limit exceeded

**Manual Testing:**
```bash
# Test auth endpoint rate limiting
for i in {1..10}; do
  curl -X POST http://localhost:8080/api/register \
    -H "Content-Type: application/json" \
    -d '{"email":"test'$i'@test.com","password":"password"}' &
done
wait

# Should see 429 responses after 5 requests
```

### 9. Future Enhancements

Not in scope for this task, but potential future improvements:
- Redis-backed rate limiter for multi-instance deployments
- Per-tier rate limits (free vs. paid users)
- Whitelist for trusted IPs
- Rate limit dashboard in admin panel
- Dynamic rate limit adjustment based on system load

## Acceptance Criteria

- [ ] Rate limiter package implemented with token bucket algorithm
- [ ] Middleware applied to all appropriate endpoints
- [ ] Configuration via environment variables
- [ ] Unit tests pass
- [ ] Integration test: 100 rapid requests to /api/register returns 429 after threshold
- [ ] Rate limit headers included in responses
- [ ] Documentation updated in README.md
- [ ] LEARNINGS.md updated with implementation notes
