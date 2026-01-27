# Prometheus Metrics Reference

This document describes the Prometheus metrics exposed by the Subtitler backend at the `/metrics` endpoint.

## Endpoint Access

The `/metrics` endpoint is protected. Access requires either:

1. **API Key** - Set `METRICS_API_KEY` env var, then use:
   ```bash
   curl -H "X-Metrics-API-Key: your-key" https://example.com/metrics
   ```

2. **Admin Session** - Login as admin user (set via `INITIAL_ADMIN_EMAIL`)

Rate limited to 10 requests/minute per IP (configurable via `METRICS_RATE_LIMIT`).

## Available Metrics

### HTTP Request Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `http_requests_total` | Counter | `method`, `path`, `status` | Total HTTP requests |
| `http_request_duration_seconds` | Histogram | `method`, `path` | Request latency distribution |

**Path Normalization:** Dynamic path segments (video IDs, session IDs) are replaced with `{id}` to prevent cardinality explosion.

### Transcription Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `transcription_total` | Counter | `status` | Transcription job count |
| `transcription_duration_seconds` | Histogram | - | Transcription processing time |

**Status values:** `started`, `completed`, `failed`

**Duration buckets:** 1s, 5s, 10s, 30s, 60s, 120s, 300s, 600s

### Upload Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `uploads_total` | Counter | `status` | Upload count by outcome |
| `uploads_bytes_total` | Counter | - | Total bytes uploaded |

**Status values:** `success`, `failed`

### Session Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `active_sessions_total` | Gauge | - | Current active user sessions |

## PromQL Queries

### Traffic Analysis

**Request rate (per second):**
```promql
rate(http_requests_total[5m])
```

**Request rate by endpoint:**
```promql
sum by (path) (rate(http_requests_total[5m]))
```

**Error rate (5xx responses):**
```promql
sum(rate(http_requests_total{status=~"5.."}[5m]))
  / sum(rate(http_requests_total[5m])) * 100
```

**Client error rate (4xx responses):**
```promql
sum(rate(http_requests_total{status=~"4.."}[5m]))
  / sum(rate(http_requests_total[5m])) * 100
```

### Latency Analysis

**Average request duration by endpoint:**
```promql
rate(http_request_duration_seconds_sum[5m])
  / rate(http_request_duration_seconds_count[5m])
```

**95th percentile latency:**
```promql
histogram_quantile(0.95, sum by (le) (rate(http_request_duration_seconds_bucket[5m])))
```

**99th percentile latency by path:**
```promql
histogram_quantile(0.99, sum by (path, le) (rate(http_request_duration_seconds_bucket[5m])))
```

**Slowest endpoints (p95):**
```promql
topk(5, histogram_quantile(0.95, sum by (path, le) (rate(http_request_duration_seconds_bucket[5m]))))
```

### Transcription Monitoring

**Transcription throughput (jobs/minute):**
```promql
rate(transcription_total{status="completed"}[5m]) * 60
```

**Transcription success rate:**
```promql
sum(rate(transcription_total{status="completed"}[1h]))
  / sum(rate(transcription_total{status=~"completed|failed"}[1h])) * 100
```

**Average transcription duration:**
```promql
rate(transcription_duration_seconds_sum[5m])
  / rate(transcription_duration_seconds_count[5m])
```

**Transcription queue depth (jobs started but not completed):**
```promql
transcription_total{status="started"} - transcription_total{status="completed"} - transcription_total{status="failed"}
```

### Upload Monitoring

**Upload throughput (MB/hour):**
```promql
rate(uploads_bytes_total[1h]) * 3600 / 1024 / 1024
```

**Upload success rate:**
```promql
sum(rate(uploads_total{status="success"}[1h]))
  / sum(rate(uploads_total[1h])) * 100
```

**Failed uploads (last hour):**
```promql
increase(uploads_total{status="failed"}[1h])
```

### Session Monitoring

**Active sessions:**
```promql
active_sessions_total
```

**Session growth rate (per hour):**
```promql
delta(active_sessions_total[1h])
```

## Alerting Rules

Save as `/etc/prometheus/rules/subtitler.yml`:

```yaml
groups:
  - name: subtitler
    interval: 1m
    rules:
      # High error rate
      - alert: HighErrorRate
        expr: |
          sum(rate(http_requests_total{status=~"5.."}[5m]))
          / sum(rate(http_requests_total[5m])) * 100 > 5
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "High error rate detected"
          description: "{{ $value | printf \"%.1f\" }}% of requests returning 5xx"

      # High latency
      - alert: HighLatency
        expr: |
          histogram_quantile(0.95, sum by (le) (rate(http_request_duration_seconds_bucket[5m]))) > 2
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High request latency"
          description: "95th percentile latency is {{ $value | printf \"%.2f\" }}s"

      # Transcription queue backing up
      - alert: TranscriptionBacklog
        expr: |
          (transcription_total{status="started"} - transcription_total{status="completed"} - transcription_total{status="failed"}) > 10
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Transcription backlog growing"
          description: "{{ $value }} transcriptions queued"

      # Transcription failures
      - alert: TranscriptionFailures
        expr: |
          rate(transcription_total{status="failed"}[1h]) > 0.1
        for: 15m
        labels:
          severity: warning
        annotations:
          summary: "Elevated transcription failures"
          description: "{{ $value | printf \"%.2f\" }} failures per second"

      # Upload failures spike
      - alert: UploadFailuresSpike
        expr: |
          sum(rate(uploads_total{status="failed"}[15m]))
          / sum(rate(uploads_total[15m])) * 100 > 10
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Upload failure rate elevated"
          description: "{{ $value | printf \"%.1f\" }}% of uploads failing"

      # Disk space (if node_exporter is present)
      - alert: DiskSpaceLow
        expr: |
          node_filesystem_avail_bytes{mountpoint="/"} / node_filesystem_size_bytes{mountpoint="/"} * 100 < 10
        for: 10m
        labels:
          severity: critical
        annotations:
          summary: "Low disk space"
          description: "{{ $value | printf \"%.1f\" }}% disk space remaining"
```

## Grafana Dashboard

### Recommended Panels

**Row 1: Traffic Overview**
- Request rate (time series)
- Error rate gauge (stat panel, threshold: >1% warning, >5% critical)
- Active sessions (stat panel)

**Row 2: Latency**
- Request duration heatmap
- 95th percentile latency by endpoint (time series)
- Slowest endpoints table

**Row 3: Transcription**
- Transcription throughput (completed/minute)
- Success rate gauge
- Duration distribution (histogram)
- Queue depth (gauge)

**Row 4: Uploads**
- Upload volume (MB/hour)
- Upload count by status (stacked bar)
- Failure rate over time

### Dashboard JSON

A sample Grafana dashboard can be imported from:
```
https://grafana.com/grafana/dashboards/[dashboard-id]
```

Or create manually using the PromQL queries above.

## Capacity Planning

### Baseline Metrics

Establish baselines during normal operation:

```promql
# Average request rate
avg_over_time(rate(http_requests_total[5m])[24h:5m])

# Average transcription duration
avg_over_time(
  (rate(transcription_duration_seconds_sum[5m]) / rate(transcription_duration_seconds_count[5m]))[24h:5m]
)

# Average upload rate
avg_over_time(rate(uploads_bytes_total[1h])[7d:1h])
```

### Capacity Thresholds

Recommended alert thresholds based on your infrastructure:

| Resource | Warning | Critical |
|----------|---------|----------|
| CPU usage | 70% | 85% |
| Memory usage | 75% | 90% |
| Disk space | 20% free | 10% free |
| Request latency (p95) | 1s | 3s |
| Error rate | 1% | 5% |
| Transcription queue | 5 jobs | 15 jobs |

### Scaling Indicators

Consider scaling when:
- p95 latency consistently > 500ms
- Transcription queue depth regularly > 5
- CPU usage > 70% sustained
- Memory usage > 75% sustained

## See Also

- [docs/ENV.md](ENV.md) - Environment variable reference
- [docs/SECURITY_EVENTS.md](SECURITY_EVENTS.md) - Security event monitoring
- [docs/RATE_LIMITS.md](RATE_LIMITS.md) - Rate limiting configuration
- [backend/metrics/metrics.go](../backend/metrics/metrics.go) - Metrics implementation
