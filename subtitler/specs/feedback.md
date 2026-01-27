# User Feedback System

## Overview

A global feedback system that allows users to submit feedback, bug reports, and feature requests from any page. The system captures context about the user's current state to help diagnose issues.

## User Story

As a user, I want to quickly send feedback about my experience, report bugs I encounter, or request new features without leaving the current page.

## Components

### 1. Frontend: FeedbackButton Component

A floating button that appears on all pages and opens a feedback modal.

**Location:** `frontend/src/components/FeedbackButton.astro`

**Features:**
- Fixed position button (bottom-right corner)
- Opens modal dialog on click
- Supports keyboard navigation (Escape to close)
- Accessible (proper ARIA attributes)

**Modal Fields:**
- Type dropdown: General, Bug Report, Feature Request
- Rating: 1-5 stars (optional)
- Message: Text area (required)
- Submit button

**Context Captured Automatically:**
- Page URL
- Video ID (if viewing a video)
- Session ID (for anonymous users)
- User Agent
- Viewport dimensions
- Timestamp

### 2. Backend: Feedback API

**Endpoint:** `POST /api/feedback`

**Request Body:**
```json
{
  "text": "string (required, max 10KB)",
  "type": "general | bug | feature",
  "rating": 1-5 or null,
  "page_url": "string",
  "video_id": "string or null",
  "session_id": "string or null",
  "browser_info": "string"
}
```

**Response:**
```json
{
  "status": "ok",
  "id": "feedback_id"
}
```

**Rate Limit:** 5 requests per minute per IP

### 3. Database Schema

**Table:** `feedback`

```sql
CREATE TABLE IF NOT EXISTS feedback (
    id TEXT PRIMARY KEY,
    user_id TEXT,                    -- NULL for anonymous users
    session_id TEXT,                 -- For anonymous tracking
    video_id TEXT,                   -- Which video (optional)
    page_url TEXT NOT NULL,          -- Current page URL
    feedback_text TEXT NOT NULL,     -- User's message
    rating INTEGER,                  -- 1-5 or NULL
    feedback_type TEXT NOT NULL,     -- "general", "bug", "feature"
    browser_info TEXT,               -- User agent, viewport, etc.
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    status TEXT DEFAULT 'new',       -- "new", "read", "resolved"
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL,
    FOREIGN KEY (video_id) REFERENCES videos(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_feedback_user_id ON feedback(user_id);
CREATE INDEX IF NOT EXISTS idx_feedback_created_at ON feedback(created_at);
CREATE INDEX IF NOT EXISTS idx_feedback_status ON feedback(status);
```

**Migration:** `007_add_feedback.up.sql`

## Implementation Details

### Frontend

1. **FeedbackButton.astro** - Global component imported in all pages
2. Uses `csrfFetch()` from `utils/csrf.ts` for secure submission
3. Uses `getOrCreateSessionId()` from `utils/session.ts` for anonymous tracking
4. Star rating with visual feedback
5. Form validation (message required)
6. Success message display

### Backend

1. **Rate limiting** - `feedbackLimiter` at 5 req/min per IP
2. **Input validation** - Text length, type enum, rating range
3. **Context capture** - User ID from session if authenticated
4. **Structured logging** - Log feedback submissions for monitoring

### Pages to Update

Import `FeedbackButton.astro` before `</body>` in:
- `index.astro`
- `upload.astro`
- `videos.astro`
- `settings.astro`
- `login.astro`
- `register.astro`
- `forgot-password.astro`
- `reset-password.astro`
- `verify-email.astro`
- `magic-link.astro`

## Security Considerations

1. **CSRF Protection** - Required for POST endpoint
2. **Rate Limiting** - Prevent spam/abuse
3. **Input Validation** - Max text length (10KB)
4. **No sensitive data in response** - Only return feedback ID
5. **Anonymous allowed** - But session_id tracked for context

## Testing

### Backend Tests (`api_test.go`)
- `TestFeedbackSubmitSuccess` - Valid submission
- `TestFeedbackSubmitMissingText` - Required field validation
- `TestFeedbackSubmitInvalidType` - Enum validation
- `TestFeedbackSubmitRateLimited` - Rate limit enforcement
- `TestFeedbackWithAuthentication` - User ID captured when logged in
- `TestFeedbackAnonymous` - Works without auth

### Frontend Tests (E2E)
- Feedback button visible on all pages
- Modal opens/closes correctly
- Form validation works
- Successful submission shows confirmation
- Keyboard navigation (Escape to close)

## Future Enhancements

1. **Admin Dashboard** - View and manage feedback (separate task)
2. **Email Notifications** - Notify admin of new feedback
3. **Screenshot Capture** - Optional screenshot attachment
4. **Recent Actions Log** - Track last N user actions for context
5. **Feedback Response** - Allow admins to respond to users
