# 008: Job Status and Notifications

## Overview

Implement real-time job status updates on the dashboard and email notifications when transcription jobs complete or fail.

## Requirements (from Task 10)

**Done When:** Upload file, dashboard shows progress updating; email received when job completes

## Components

### 1. Email Notification System

Add email notifications using Resend API (following the pattern from `personal/001_INITIALIZATION.md`).

**Backend changes:**
- Add email package (`backend/internal/email/`)
  - `SendJobCompleted(userEmail, jobID, filename string)`
  - `SendJobFailed(userEmail, jobID, filename, errorMessage string)`
- Update worker to send emails after job completion/failure
- Add configuration for Resend API:
  - `RESEND_API_KEY` - API key for Resend
  - `EMAIL_FROM` - Sender email address (e.g., `noreply@subtitler.example.com`)
  - `ENABLE_EMAIL` - Feature flag (default: false for development)

**Email templates:**
- Success email:
  ```
  Subject: Your transcription is ready

  Your transcription for "{filename}" is complete!

  Job ID: {job_id}
  Format: {output_format}

  Download your transcript at:
  https://subtitler.example.com/dashboard

  This job took {duration} to complete.
  ```

- Failure email:
  ```
  Subject: Transcription failed

  Your transcription for "{filename}" could not be completed.

  Job ID: {job_id}
  Error: {error_message}

  Please try uploading your file again. If this problem persists, contact support.
  ```

### 2. Dashboard Auto-Refresh

Update the frontend dashboard to poll for job status updates.

**Frontend changes (`frontend/src/pages/dashboard.astro`):**
- Add polling logic with JavaScript in `<script>` tag
- Poll `/api/jobs` endpoint every 5 seconds when there are pending jobs
- Stop polling when all jobs are completed/failed
- Update UI without full page reload
- Show visual feedback for status changes

**UX considerations:**
- Don't poll if there are no pending jobs
- Visual indicator that updates are happening (e.g., "Checking for updates...")
- Smooth transitions when job status changes
- Toast notification when job completes (optional enhancement)

### 3. Progress Tracking (Future Enhancement)

For now, we'll use simple polling. Future enhancements could include:
- WebSocket connection for real-time updates
- Server-Sent Events (SSE) for one-way updates
- Progress percentage during transcription

## Implementation Plan

### Step 1: Email Configuration
1. Add email configuration to `backend/internal/config/config.go`
2. Document environment variables in README.md
3. Update `.env.example` if it exists

### Step 2: Email Package
1. Create `backend/internal/email/email.go`
2. Implement `Client` struct with Resend API integration
3. Add `SendJobCompleted()` method
4. Add `SendJobFailed()` method
5. Handle feature flag (skip sending if `ENABLE_EMAIL=false`)

### Step 3: Worker Integration
1. Update `backend/internal/worker/worker.go`
2. Pass email client to worker pool
3. Send email after job completes (success or failure)
4. Get user email from database before sending

### Step 4: Dashboard Polling
1. Update `frontend/src/pages/dashboard.astro`
2. Add JavaScript polling logic in `<script>` tag
3. Only poll when pending jobs exist
4. Update UI dynamically when status changes
5. Add visual feedback for polling state

### Step 5: Testing
1. Manual testing:
   - Upload file, verify email received on completion
   - Cause job to fail (bad file), verify failure email
   - Open dashboard with pending job, verify auto-refresh
2. Update integration test to verify email sending (if ENABLE_EMAIL=true)
3. Consider adding Playwright test for dashboard auto-refresh

## Files to Create/Modify

**New files:**
- `backend/internal/email/email.go` - Email client and methods
- `backend/internal/email/email_test.go` - Unit tests (mock Resend API)

**Modified files:**
- `backend/internal/config/config.go` - Add email config
- `backend/internal/worker/worker.go` - Send emails after job completion
- `backend/internal/db/users.go` - May need `GetUserByID()` if it doesn't exist
- `backend/cmd/server/main.go` - Initialize email client, pass to worker pool
- `frontend/src/pages/dashboard.astro` - Add polling JavaScript
- `README.md` - Document email configuration
- `CLAUDE.md` - Update environment variables section

## Configuration

Add to `backend/internal/config/config.go`:

```go
type Config struct {
    // ... existing fields ...

    // Email settings
    ResendAPIKey string
    EmailFrom    string
    EnableEmail  bool
}
```

Environment variables:
```bash
RESEND_API_KEY=re_xxxxx
EMAIL_FROM=noreply@subtitler.example.com
ENABLE_EMAIL=false  # Set to true in production
```

## Testing Strategy

### Unit Tests
- Test email formatting (subject, body content)
- Test email sending with mocked HTTP client
- Test feature flag (ENABLE_EMAIL=false skips sending)

### Integration Tests
- Upload file with ENABLE_EMAIL=false (no actual email sent)
- Verify email client is initialized correctly
- Verify worker calls email methods

### Manual Tests
1. Set ENABLE_EMAIL=false, upload file, verify no email sent
2. Set ENABLE_EMAIL=true with valid RESEND_API_KEY
3. Upload small audio file (jfk.wav)
4. Wait for completion
5. Check email inbox for success notification
6. Upload invalid file, verify failure email

### E2E Tests (Playwright)
- Open dashboard with pending job
- Wait for job to complete (or mock completion)
- Verify status updates without page reload
- Verify download link appears when complete

## Dependencies

**Go packages:**
- No new dependencies required (use `net/http` for Resend API calls)
- Consider `html/template` for email templates (optional)

**Frontend:**
- No new dependencies (use vanilla JavaScript for polling)

## Security Considerations

1. **Email addresses**: Only send emails to verified user email addresses
2. **Rate limiting**: Consider rate limiting emails per user (future enhancement)
3. **API key**: Store RESEND_API_KEY securely (use sops+age in production)
4. **Email content**: Don't include sensitive information in email body
5. **Polling**: Use reasonable interval (5s) to avoid overloading server

## Notes

- Resend API docs: https://resend.com/docs/api-reference/emails/send-email
- Resend free tier: 100 emails/day, 3000 emails/month
- Email verification: Resend requires domain verification for production use
- For development, can use ENABLE_EMAIL=false to skip email sending
- Polling is simpler than WebSockets for initial implementation
- WebSocket support can be added later if real-time updates are critical
