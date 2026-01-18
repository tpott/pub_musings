# 010 Analytics Integration

## Overview

Implement lightweight analytics tracking to support the experimentation framework defined in EXPERIMENTATION_PLAN.md. Focus on event tracking, A/B test variant assignment, and funnel analysis without adding external dependencies.

## Goals

1. Track key user events (signups, uploads, downloads, conversions)
2. Support A/B test variant assignment and tracking
3. Enable funnel analysis (visitor → signup → upload → download)
4. Store analytics data in SQLite (keep it simple)
5. Provide basic query API for analysis
6. Privacy-first: minimal PII, local storage, no third-party tracking

## Architecture

```
Frontend (Astro/JS)
  ├── trackEvent(event_name, properties)
  └── POST /api/analytics/events

Backend (Go)
  ├── POST /api/analytics/events (store event)
  ├── GET /api/analytics/funnel (query funnel data)
  └── SQLite analytics tables
      ├── events
      ├── visitors
      └── experiments
```

## Database Schema

### Table: `analytics_visitors`

Tracks anonymous visitors and their assigned experiment variants.

```sql
CREATE TABLE analytics_visitors (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    visitor_id TEXT NOT NULL UNIQUE,  -- UUID generated client-side
    user_id INTEGER,                   -- NULL for anonymous, FK to users if logged in
    utm_source TEXT,
    utm_medium TEXT,
    utm_campaign TEXT,
    first_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE INDEX idx_visitors_visitor_id ON analytics_visitors(visitor_id);
CREATE INDEX idx_visitors_user_id ON analytics_visitors(user_id);
```

### Table: `analytics_events`

Stores all tracked events with properties.

```sql
CREATE TABLE analytics_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    visitor_id TEXT NOT NULL,
    user_id INTEGER,
    event_name TEXT NOT NULL,
    properties TEXT,  -- JSON blob for flexible event properties
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE INDEX idx_events_visitor_id ON analytics_events(visitor_id);
CREATE INDEX idx_events_user_id ON analytics_events(user_id);
CREATE INDEX idx_events_event_name ON analytics_events(event_name);
CREATE INDEX idx_events_created_at ON analytics_events(created_at);
```

### Table: `analytics_experiments`

Tracks experiment variant assignments.

```sql
CREATE TABLE analytics_experiments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    visitor_id TEXT NOT NULL,
    experiment_id TEXT NOT NULL,  -- e.g., "EXP001"
    variant TEXT NOT NULL,         -- e.g., "A", "B", "C"
    assigned_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(visitor_id, experiment_id)
);

CREATE INDEX idx_experiments_visitor_id ON analytics_experiments(visitor_id);
CREATE INDEX idx_experiments_experiment_id ON analytics_experiments(experiment_id);
```

## Key Events to Track

### Core Funnel Events

| Event Name | Trigger | Properties |
|------------|---------|------------|
| `page_view` | Page load | `{page: "/", referrer: "..."}` |
| `signup_started` | User clicks "Sign up" | `{}` |
| `signup_completed` | User successfully registers | `{user_id: 123}` |
| `login_completed` | User successfully logs in | `{user_id: 123}` |
| `upload_started` | User clicks upload button | `{}` |
| `upload_completed` | File successfully uploaded | `{job_id: 456, file_size_bytes: 1024, format: "srt"}` |
| `job_completed` | Transcription finishes | `{job_id: 456, duration_seconds: 120, format: "srt"}` |
| `download_completed` | User downloads transcript | `{job_id: 456, format: "srt"}` |

### Experiment Events

| Event Name | Trigger | Properties |
|------------|---------|------------|
| `experiment_viewed` | User sees experiment variant | `{experiment_id: "EXP001", variant: "B"}` |
| `experiment_converted` | User completes experiment goal | `{experiment_id: "EXP001", variant: "B", goal: "signup"}` |

## API Endpoints

### POST /api/analytics/events

Store an analytics event.

**Request**:
```json
{
  "visitor_id": "uuid-v4",
  "event_name": "signup_completed",
  "properties": {
    "user_id": 123
  },
  "utm_source": "reddit",
  "utm_medium": "organic",
  "utm_campaign": null
}
```

**Response**:
```json
{
  "success": true
}
```

**Implementation Notes**:
- Accept events from both authenticated and anonymous users
- Extract `user_id` from JWT if authenticated
- Store UTM parameters in `analytics_visitors` table
- Properties stored as JSON text blob

### GET /api/analytics/funnel

Query funnel conversion rates.

**Request**: `GET /api/analytics/funnel?start=2026-01-01&end=2026-01-31`

**Response**:
```json
{
  "period": {
    "start": "2026-01-01",
    "end": "2026-01-31"
  },
  "funnel": [
    {"stage": "visitors", "count": 1000},
    {"stage": "signups", "count": 150},
    {"stage": "uploads", "count": 100},
    {"stage": "downloads", "count": 85}
  ],
  "conversion_rates": {
    "visitor_to_signup": 0.15,
    "signup_to_upload": 0.67,
    "upload_to_download": 0.85
  }
}
```

**Implementation Notes**:
- Requires authentication (admin only for MVP)
- Default to last 30 days if no date range provided
- Calculate distinct counts at each stage

### GET /api/analytics/experiments/:experiment_id

Query A/B test results for a specific experiment.

**Request**: `GET /api/analytics/experiments/EXP001?start=2026-01-01&end=2026-01-31`

**Response**:
```json
{
  "experiment_id": "EXP001",
  "period": {
    "start": "2026-01-01",
    "end": "2026-01-31"
  },
  "variants": [
    {
      "variant": "A",
      "visitors": 300,
      "conversions": 45,
      "conversion_rate": 0.15
    },
    {
      "variant": "B",
      "visitors": 310,
      "conversions": 55,
      "conversion_rate": 0.177
    },
    {
      "variant": "C",
      "visitors": 290,
      "conversions": 38,
      "conversion_rate": 0.131
    }
  ],
  "recommendation": "Variant B shows 18% improvement over control (p < 0.05)"
}
```

**Implementation Notes**:
- Calculate conversion rates by joining experiments and events tables
- Use chi-square test for statistical significance
- Conversion defined as `experiment_converted` event for that experiment

## Frontend Implementation

### Client-side Tracking

Create a simple analytics client at `frontend/src/lib/analytics.ts`:

```typescript
// Generate or retrieve visitor ID from localStorage
function getVisitorId(): string {
  let visitorId = localStorage.getItem('visitor_id');
  if (!visitorId) {
    visitorId = crypto.randomUUID();
    localStorage.setItem('visitor_id', visitorId);
  }
  return visitorId;
}

// Extract UTM parameters from URL
function getUTMParams(): { source?: string; medium?: string; campaign?: string } {
  const params = new URLSearchParams(window.location.search);
  return {
    source: params.get('utm_source') || undefined,
    medium: params.get('utm_medium') || undefined,
    campaign: params.get('utm_campaign') || undefined,
  };
}

// Track an event
export async function trackEvent(eventName: string, properties: Record<string, any> = {}) {
  const visitorId = getVisitorId();
  const utm = getUTMParams();

  try {
    await fetch('/api/analytics/events', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      credentials: 'include',
      body: JSON.stringify({
        visitor_id: visitorId,
        event_name: eventName,
        properties,
        utm_source: utm.source,
        utm_medium: utm.medium,
        utm_campaign: utm.campaign,
      }),
    });
  } catch (error) {
    // Fail silently - don't disrupt user experience
    console.error('Analytics tracking failed:', error);
  }
}

// Track page view
export function trackPageView() {
  trackEvent('page_view', {
    page: window.location.pathname,
    referrer: document.referrer,
  });
}
```

### Usage in Pages

```astro
---
// In index.astro
---
<script>
  import { trackPageView, trackEvent } from '../lib/analytics';

  // Track page view on load
  trackPageView();

  // Track signup button click
  document.getElementById('signup-btn')?.addEventListener('click', () => {
    trackEvent('signup_started');
  });
</script>
```

## Backend Implementation

### Package Structure

```
backend/internal/analytics/
├── analytics.go      # Core event tracking logic
├── models.go         # Event, Visitor, Experiment structs
├── queries.go        # Funnel and experiment queries
└── analytics_test.go # Unit tests
```

### Key Functions

**TrackEvent**:
```go
func (s *Service) TrackEvent(ctx context.Context, event Event) error {
    // 1. Upsert visitor with UTM params
    // 2. Insert event with properties (JSON)
    // 3. Return success
}
```

**GetFunnel**:
```go
func (s *Service) GetFunnel(ctx context.Context, start, end time.Time) (*FunnelReport, error) {
    // 1. Count distinct visitors
    // 2. Count distinct signups (event: signup_completed)
    // 3. Count distinct uploads (event: upload_completed)
    // 4. Count distinct downloads (event: download_completed)
    // 5. Calculate conversion rates
}
```

**AssignExperiment**:
```go
func (s *Service) AssignExperiment(ctx context.Context, visitorID, experimentID string, variants []string) (string, error) {
    // 1. Check if visitor already assigned
    // 2. If not, randomly assign variant
    // 3. Store in analytics_experiments table
    // 4. Return assigned variant
}
```

**GetExperimentResults**:
```go
func (s *Service) GetExperimentResults(ctx context.Context, experimentID string, start, end time.Time) (*ExperimentReport, error) {
    // 1. Get all visitors assigned to experiment
    // 2. Count conversions per variant
    // 3. Calculate conversion rates
    // 4. Compute statistical significance (chi-square)
}
```

## Privacy & Ethics

1. **No PII collection**: Only track visitor_id (anonymous UUID) and user_id (internal)
2. **Local storage**: All analytics data stays in SQLite, no third-party services
3. **Opt-out**: Add "Do Not Track" detection and respect it
4. **Data retention**: Delete analytics events older than 1 year
5. **Transparency**: Document what we track in Privacy Policy

## Testing Strategy

### Unit Tests

- Test event insertion and retrieval
- Test funnel calculation logic
- Test experiment variant assignment (should be deterministic for same visitor)
- Test experiment results aggregation

### Integration Tests

- Test full event tracking flow (frontend → backend → database)
- Test funnel query with known test data
- Test experiment assignment persistence

### Manual Testing

1. Open site in incognito mode
2. Check localStorage for visitor_id
3. Click signup button, verify `signup_started` event in database
4. Complete signup, verify `signup_completed` event
5. Query funnel API, verify counts match

## Deployment Considerations

1. **Migration**: Add analytics tables to migration system
2. **Indexes**: Ensure proper indexes for query performance
3. **Monitoring**: Log analytics API errors (but don't fail on tracking errors)
4. **Backup**: Include analytics tables in database backups

## Success Criteria

Task 16 is complete when:

1. ✅ Analytics tables created via migration
2. ✅ POST /api/analytics/events endpoint works
3. ✅ Frontend trackEvent() function works
4. ✅ Page views tracked on index and dashboard
5. ✅ Signup/upload/download events tracked
6. ✅ GET /api/analytics/funnel returns conversion data
7. ✅ Unit tests pass for analytics package
8. ✅ README.md documents analytics usage

**Verification command**:
```bash
# Upload and download a file
# Then query funnel:
curl -H "Cookie: auth_token=..." http://localhost:8080/api/analytics/funnel
# Should return funnel data with at least 1 upload and 1 download
```

## Future Enhancements (Not in Scope)

- Dashboard UI for viewing analytics (can query via curl for now)
- Cohort analysis (group users by signup date, compare behavior)
- Retention metrics (N-day retention rates)
- Real-time analytics (current implementation is batch/query-based)
- Export to CSV for external analysis

## References

- EXPERIMENTATION_PLAN.md (defines required experiments)
- MARKETING_PLAN.md (defines success metrics)
- backend/internal/db/migrations/ (migration system)
