# 014 Admin Usage Dashboard

## Overview

Task 20: Add admin analytics dashboard showing system-wide usage statistics.

**Done When:** Admin can view /admin/analytics showing total users, jobs, storage used, and conversion funnels.

## Current State

- Analytics service exists with funnel queries
- No admin concept exists in user model
- No admin routes or middleware

## Implementation Plan

### Phase 1: Database Migration

Add `is_admin` boolean field to users table.

**Migration:** `003_add_admin_flag.sql`
```sql
ALTER TABLE users ADD COLUMN is_admin BOOLEAN DEFAULT FALSE;
```

### Phase 2: Admin Middleware

Create admin check middleware that:
1. Uses existing auth middleware to verify JWT
2. Checks if user's `is_admin` flag is true
3. Returns 403 if not admin

**File:** `backend/internal/auth/admin.go`

### Phase 3: Backend Admin Analytics Endpoint

**Endpoint:** `GET /api/admin/analytics`

**Response Structure:**
```json
{
  "period": {"start": "2026-01-01", "end": "2026-01-19"},
  "users": {
    "total": 150,
    "new_this_period": 25,
    "active_this_period": 80
  },
  "jobs": {
    "total": 1200,
    "completed": 1100,
    "failed": 50,
    "pending": 50,
    "by_format": {
      "srt": 600,
      "vtt": 400,
      "embedded": 200
    }
  },
  "storage": {
    "total_bytes": 5368709120,
    "uploads_bytes": 4294967296,
    "results_bytes": 1073741824
  },
  "funnel": {
    "visitors": 1000,
    "signups": 150,
    "uploads": 100,
    "downloads": 80,
    "conversion_rates": {
      "visitor_to_signup": 15.0,
      "signup_to_upload": 66.7,
      "upload_to_download": 80.0
    }
  }
}
```

### Phase 4: Frontend Admin Dashboard

**Page:** `frontend/src/pages/admin/analytics.astro`

Features:
- Check if user is admin via `/api/me` (add is_admin to response)
- Redirect to home if not admin
- Display analytics cards:
  - Total Users / New Users
  - Total Jobs / Success Rate
  - Storage Usage
  - Conversion Funnel

### Phase 5: Route Registration

Add routes in `main.go`:
- `GET /api/admin/analytics` - Admin analytics endpoint

## Files to Create/Modify

### New Files
- `backend/internal/db/migrations/003_add_admin_flag.sql`
- `backend/internal/auth/admin.go`
- `backend/cmd/server/admin_handlers.go`
- `frontend/src/pages/admin/analytics.astro`

### Modified Files
- `backend/internal/db/users.go` - Add IsAdmin field to User struct
- `backend/cmd/server/auth_handlers.go` - Return is_admin in /api/me response
- `backend/cmd/server/main.go` - Register admin routes

## Verification

1. Create test admin user via SQL: `UPDATE users SET is_admin = true WHERE email = 'admin@test.com'`
2. Login as admin
3. Navigate to /admin/analytics
4. Verify all stats display correctly
5. Verify non-admin users get redirected

## Security Notes

- Admin endpoints protected by auth middleware + admin check
- Admin flag cannot be set via API (DB only for now)
- Rate limiting applied to admin routes
