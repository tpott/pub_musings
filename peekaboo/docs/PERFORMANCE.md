# Performance Tuning Guide

This document covers performance tuning parameters and monitoring recommendations for the Peekaboo backend.

## Database Connection Pool

SQLite is configured with connection pooling via Go's `database/sql` package.

### Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_MAX_OPEN_CONNS` | 1 | Maximum number of open connections |
| `DB_MAX_IDLE_CONNS` | 1 | Maximum number of idle connections |

### SQLite Considerations

SQLite only supports one writer at a time, even with WAL mode enabled. The default of 1 connection is recommended for most workloads.

**When to increase connections:**
- Read-heavy workloads may benefit from 2-5 connections
- Multiple concurrent read operations while a write is in progress

**Risks of higher values:**
- "database is locked" errors during write-heavy workloads
- Increased memory usage per connection
- No benefit for write-heavy workloads

**Recommendation:** Keep defaults unless you observe read latency issues in production logs.

## WebSocket Configuration

### Idle Timeout

| Variable | Default | Description |
|----------|---------|-------------|
| `WEBSOCKET_IDLE_TIMEOUT_SECS` | 300 | Close connections after this many seconds of inactivity |

**When to adjust:**
- **Increase (600-900s):** Users report disconnections during long pauses between commands
- **Decrease (60-120s):** Memory constrained environments, many concurrent users
- **Keep default:** Most use cases; 5 minutes accommodates normal interaction patterns

### Max Concurrent Connections

| Variable | Default | Description |
|----------|---------|-------------|
| `WEBSOCKET_MAX_CONNECTIONS` | 100 | Maximum concurrent WebSocket connections |

**Behavior when limit reached:**
- New connection attempts receive HTTP 503 Service Unavailable
- Response includes JSON: `{"error":"server at capacity, try again later"}`
- A WARN log is emitted: `websocket connection limit reached`

**When to adjust:**
- **Increase (200-500):** High-traffic deployments with sufficient memory
- **Decrease (25-50):** Memory-constrained environments, want to reserve capacity
- **Keep default:** Most use cases; 100 concurrent users is generous for a single instance

**Memory impact:** Each WebSocket connection uses approximately 10-20KB of memory for connection state and audio buffering. At 100 connections, this is ~1-2MB base plus audio buffers.

### Buffer Threshold

Audio is automatically processed when buffered for 3 seconds without `stop_recording`. This is hardcoded but could be made configurable if needed.

## Rate Limiting

### Configuration

Rate limits are set in code (10 requests per minute per IP for transcribe, intent, and WebSocket endpoints).

| Parameter | Value | Description |
|-----------|-------|-------------|
| Rate limit | 10 req/min | Requests allowed per IP per minute |
| Max entries | 10,000 | Maximum unique IPs tracked |

### Max Entries Behavior

When the rate limiter tracks 10,000 unique IPs and a new IP makes a request:
1. The IP with the oldest "last request" time is evicted
2. The new IP is added and allowed
3. An INFO log is emitted: `rate limiter evicted IP due to max entries limit`

**Memory impact:** Each tracked IP uses approximately 100-200 bytes. At 10K IPs, this is ~1-2MB.

**When eviction indicates problems:**
- Frequent eviction logs suggest possible attack or need for external rate limiting (e.g., at load balancer)
- Consider adding rate limiting at the infrastructure level (Caddy, nginx, CloudFlare)

## Monitoring Recommendations

### Log-Based Monitoring

Peekaboo uses structured logging (slog). In production, set `LOG_FORMAT=json` for easier parsing.

#### Key Metrics to Monitor

**Rate Limiting:**
```bash
# Count rate limit rejections (last hour)
grep "rate limit exceeded" /var/log/peekaboo.log | wc -l

# Find IPs hitting rate limits
grep "rate limit exceeded" /var/log/peekaboo.log | jq -r '.ip' | sort | uniq -c | sort -rn | head -10
```

**Request Latency:**
```bash
# Find slow requests (>5s)
grep "duration_ms" /var/log/peekaboo.log | jq 'select(.duration_ms > 5000)'

# Average duration by endpoint
grep "duration_ms" /var/log/peekaboo.log | jq -r '[.path, .duration_ms] | @tsv' | \
  awk '{sum[$1]+=$2; count[$1]++} END {for (p in sum) print p, sum[p]/count[p]}'
```

**WebSocket Connections:**
```bash
# Count active WebSocket connections
grep "websocket connection established" /var/log/peekaboo.log | wc -l

# Find idle timeout disconnections
grep "closing idle connection" /var/log/peekaboo.log | wc -l

# Find connection limit rejections
grep "websocket connection limit reached" /var/log/peekaboo.log | wc -l
```

**Errors:**
```bash
# Count errors by type
grep '"level":"ERROR"' /var/log/peekaboo.log | jq -r '.msg' | sort | uniq -c | sort -rn

# Find transcription failures
grep "transcription failed" /var/log/peekaboo.log | tail -20
```

### Health Checks

Use the health endpoints for monitoring:

```bash
# Liveness (is process running?)
curl -s http://localhost:8080/health/live | jq

# Readiness (are dependencies available?)
curl -s http://localhost:8080/health/ready | jq
```

The readiness endpoint checks:
- Database connectivity
- Whisper server availability
- LLM provider availability (if configured)
- Piper TTS availability (if configured)

### Request ID Tracing

Each request gets a unique `request_id` logged in all related log entries. To trace a specific request:

```bash
grep "abc123" /var/log/peekaboo.log | jq
```

## Performance Baseline

Typical latencies on a standard VM:

| Operation | Expected Latency |
|-----------|------------------|
| Health check | <5ms |
| Media lookup | <50ms |
| Transcription (whisper) | 500ms-3s (depends on audio length) |
| Intent extraction (LLM) | 200ms-1s (depends on provider) |
| Full voice flow | 1-5s total |

If you observe latencies significantly higher than these, check:
1. Network latency to external services (whisper, LLM)
2. Database lock contention (if running multiple instances)
3. Memory pressure (check system memory usage)
