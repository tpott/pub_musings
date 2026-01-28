# Metrics System Specification

This document describes the architecture and design decisions for the Prometheus metrics system in Subtitler.

## Overview

The metrics system provides observability into application health, performance, and usage patterns. It exposes Prometheus-compatible metrics at the `/metrics` endpoint for scraping by monitoring infrastructure.

## Architecture

### Components

```
┌─────────────────────────────────────────────────────────────────────┐
│                           HTTP Request                               │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│                      MetricsMiddleware                               │
│  • Records http_requests_total                                       │
│  • Records http_request_duration_seconds                             │
│  • Skips /metrics endpoint (prevents recursion)                      │
└─────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        Application Handlers                          │
│  • Upload handlers → uploads_total, uploads_bytes_total              │
│  • Transcription handlers → transcription_total,                     │
│                             transcription_duration_seconds           │
└─────────────────────────────────────────────────────────────────────┘
```

### Package Structure

```
backend/
├── metrics/
│   ├── metrics.go      # Metric definitions, middleware, recording functions
│   └── metrics_test.go # Path normalization tests
└── main.go             # Metric recording calls in handlers
```

## Design Decisions

### 1. Prometheus as Metrics Backend

**Decision:** Use Prometheus-compatible metrics format with the `prometheus/client_golang` library.

**Rationale:**
- Industry standard for cloud-native monitoring
- Extensive tooling ecosystem (Prometheus, Grafana, AlertManager)
- Pull-based model suits our deployment (single server)
- No additional infrastructure required for basic monitoring

### 2. Path Normalization for Cardinality Control

**Decision:** Replace dynamic path segments (IDs) with `{id}` placeholder before recording metrics.

**Rationale:**
- Video IDs, session IDs, and upload session IDs are 32-character hex strings
- Without normalization, each unique ID creates a new time series
- Unbounded cardinality can overwhelm Prometheus storage
- Pattern: `/api/videos/abc123.../video` → `/api/videos/{id}/video`

**Patterns Normalized:**
| Pattern | Example | Normalized |
|---------|---------|------------|
| 32-char hex | `/api/videos/a1b2c3...` | `/api/videos/{id}` |
| UUID | `/api/sessions/a1b2c3d4-e5f6-...` | `/api/sessions/{id}` |
| Numeric | `/api/chunks/12345` | `/api/chunks/{id}` |

### 3. Dual Authentication for Metrics Endpoint

**Decision:** Support both API key and admin session authentication.

**Rationale:**
- API key enables automated scraping (Prometheus, monitoring scripts)
- Admin session enables ad-hoc debugging via browser
- Rate limiting (10/min) prevents reconnaissance attacks
- Metrics can leak operational details (paths, error rates)

**Authentication Flow:**
```
1. Check X-Metrics-API-Key header or api_key query param
   ├─ Match → Serve metrics (200)
   └─ No match → Continue
2. Check session cookie
   ├─ Invalid → Return 401
   └─ Valid → Check admin role
      ├─ Not admin → Return 403
      └─ Admin → Serve metrics (200)
```

### 4. Histogram Bucket Selection

**Decision:** Custom buckets for transcription duration; default buckets for HTTP latency.

**HTTP Duration Buckets:** Default Prometheus buckets (0.005s to 10s)
- Suitable for typical web API response times
- Provides good granularity for sub-second requests

**Transcription Duration Buckets:** `1s, 5s, 10s, 30s, 60s, 120s, 300s, 600s`
- Transcription jobs range from seconds to 10+ minutes
- Aligned with typical video lengths
- Wider buckets reduce cardinality while preserving insight

### 5. Metric Labels Philosophy

**Decision:** Minimal labels to avoid cardinality explosion.

| Metric | Labels | Rationale |
|--------|--------|-----------|
| `http_requests_total` | method, path, status | Essential for traffic analysis |
| `http_request_duration_seconds` | method, path | Status not needed (duration matters regardless) |
| `transcription_total` | status | Track success/failure ratio |
| `transcription_duration_seconds` | none | Aggregate duration is sufficient |
| `uploads_total` | status | Track success/failure ratio |
| `uploads_bytes_total` | none | Aggregate bytes, not per-status |
| `active_sessions_total` | none | Single gauge value |

## Metrics Catalog

### HTTP Metrics

| Name | Type | Description |
|------|------|-------------|
| `http_requests_total` | Counter | Total HTTP requests processed |
| `http_request_duration_seconds` | Histogram | Request latency distribution |

Recorded by: `MetricsMiddleware()` in `metrics.go`

### Business Metrics

| Name | Type | Description |
|------|------|-------------|
| `transcription_total` | Counter | Transcription job starts/completions/failures |
| `transcription_duration_seconds` | Histogram | Transcription processing time |
| `uploads_total` | Counter | Upload successes/failures |
| `uploads_bytes_total` | Counter | Total bytes uploaded |
| `active_sessions_total` | Gauge | Current active user sessions |

Recorded by: Application handlers in `main.go`

## Collection Points

### Middleware (Automatic)

All HTTP requests pass through `MetricsMiddleware`:
- Entry: Start timer
- Exit: Record duration and request count
- Excluded: `/metrics` endpoint itself

### Manual Recording

| Event | Function | Location |
|-------|----------|----------|
| Transcription started | `RecordTranscriptionStarted()` | POST /api/transcribe/{id} |
| Transcription completed | `RecordTranscriptionCompleted(duration)` | Transcription goroutine |
| Transcription failed | `RecordTranscriptionFailed()` | Error handlers |
| Upload success | `RecordUploadSuccess()` | POST /api/upload, /api/upload/complete |
| Upload bytes | `RecordUploadBytes(size)` | After file write |
| Session count | `SetActiveSessions(count)` | (Not currently wired) |

## Integration with Monitoring Systems

### Prometheus Configuration

```yaml
scrape_configs:
  - job_name: 'subtitler'
    scrape_interval: 15s
    scheme: https
    static_configs:
      - targets: ['subtitler.example.com']
    metrics_path: /metrics
    bearer_token: '<api-key>'
    # OR
    params:
      api_key: ['<api-key>']
```

### Grafana Integration

See [docs/METRICS.md](../docs/METRICS.md) for:
- Recommended dashboard panels
- PromQL query examples
- Alert rule templates

### AlertManager Integration

Alerts defined in Prometheus rules can route to:
- Email (SMTP)
- Slack/Discord webhooks
- PagerDuty
- Custom webhooks

Example alert flow:
```
Prometheus Rule → AlertManager → Slack/Email
     ↓
HighErrorRate → severity:critical → #oncall channel
TranscriptionBacklog → severity:warning → #monitoring channel
```

## Security Considerations

1. **Authentication Required:** Metrics expose operational details
2. **Rate Limiting:** 10 req/min prevents enumeration
3. **Admin Role:** Session-based access requires admin privileges
4. **No User Data:** Metrics contain no PII or user content
5. **Path Normalization:** IDs are replaced, not exposed

## Future Enhancements

### Planned

- [ ] Wire `SetActiveSessions()` to session management
- [ ] Add `RecordUploadFailed()` calls to failure paths
- [ ] Add database query latency metrics
- [ ] Add whisper-server health metrics

### Considered but Deferred

- **Per-user metrics:** Would explode cardinality
- **Detailed error codes:** Status label sufficient for now
- **Tracing integration:** Not needed at current scale

## References

- [docs/METRICS.md](../docs/METRICS.md) - Reference documentation
- [docs/ENV.md](../docs/ENV.md) - Environment variables
- [docs/SECURITY_EVENTS.md](../docs/SECURITY_EVENTS.md) - Security event logging
- [backend/metrics/metrics.go](../backend/metrics/metrics.go) - Implementation
