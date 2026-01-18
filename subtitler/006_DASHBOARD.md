# 006_DASHBOARD - User Dashboard Implementation

## Goal
Implement a user dashboard where logged-in users can view their transcription jobs with download links.

## Done When
Login as user, navigate to /dashboard, see list of past jobs with download links.

## Architecture

### Backend Components
1. **GET /api/jobs** - Returns list of jobs for authenticated user
2. **GET /api/jobs/:id** - Returns single job details
3. **GET /api/jobs/:id/download** - Downloads transcription result file

### Frontend Components
1. **/dashboard** - Protected dashboard page
2. **Login/Register pages** - If not already exist
3. **Navigation** - Links to dashboard from main page

### Database
- Already exists: `jobs` table with user_id, status, file paths, timestamps
- May need to add columns if missing: `original_filename`, `result_format`, `duration`

## Implementation Steps

### 1. Backend: Job API Endpoints
- [ ] Create `backend/cmd/server/jobs_handlers.go`
- [ ] Implement `handleListJobs` (GET /api/jobs) - Returns jobs for authenticated user
- [ ] Implement `handleGetJob` (GET /api/jobs/:id) - Returns single job, verify ownership
- [ ] Implement `handleDownloadResult` (GET /api/jobs/:id/download) - Streams result file
- [ ] Wire up routes in `main.go` with auth middleware
- [ ] Add unit tests for handlers

### 2. Backend: Job Database Functions
- [ ] Review `backend/internal/db/jobs.go` - May already exist from Task 5
- [ ] Ensure functions exist: `GetJobsByUserID`, `GetJobByID`
- [ ] Add tests if not already present

### 3. Frontend: Dashboard Page
- [ ] Create `frontend/src/pages/dashboard.astro`
- [ ] Implement authentication check (redirect to login if not authenticated)
- [ ] Fetch jobs from `/api/jobs` on page load
- [ ] Display jobs in a table: filename, status, created date, duration, actions
- [ ] Add download button for completed jobs (links to `/api/jobs/:id/download`)
- [ ] Show empty state if no jobs exist

### 4. Frontend: Login/Register Pages (if needed)
- [ ] Check if `frontend/src/pages/login.astro` exists
- [ ] Check if `frontend/src/pages/register.astro` exists
- [ ] If missing, create login form (POST to /api/login)
- [ ] If missing, create register form (POST to /api/register)
- [ ] Handle cookie-based authentication
- [ ] Redirect to dashboard after successful login/register

### 5. Frontend: Navigation
- [ ] Update homepage to show "Dashboard" link if authenticated
- [ ] Update homepage to show "Login" / "Register" links if not authenticated
- [ ] Add "Logout" button in dashboard (POST to /api/logout)

### 6. Integration Testing
- [ ] Manual test: Register new user, login, see empty dashboard
- [ ] Manual test: Upload file via existing UI, check dashboard shows job
- [ ] Manual test: Download result from dashboard
- [ ] Playwright test: Full flow (register -> upload -> dashboard -> download)

## Key Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Auth flow | Cookie-based (already implemented) | Consistent with Task 6 |
| Download strategy | Stream file directly from `/api/jobs/:id/download` | Simple, works for encrypted files |
| Job status display | Show all jobs (pending, complete, failed) | User visibility into all activity |
| Empty state | Show friendly message if no jobs | Better UX than blank page |

## Files to Create/Modify

### Backend
- `backend/cmd/server/jobs_handlers.go` (new)
- `backend/cmd/server/main.go` (modify - add routes)
- `backend/internal/db/jobs.go` (review/modify if needed)

### Frontend
- `frontend/src/pages/dashboard.astro` (new)
- `frontend/src/pages/login.astro` (new if doesn't exist)
- `frontend/src/pages/register.astro` (new if doesn't exist)
- `frontend/src/pages/index.astro` (modify - add navigation)
- `frontend/tests/dashboard.spec.ts` (new - Playwright test)

## Testing Strategy

### Unit Tests
- Backend: Test job handlers with mock DB
- Backend: Test job DB functions

### Integration Tests
- Manual curl tests for /api/jobs endpoints
- Manual browser test for dashboard page

### E2E Tests
- Playwright: Register -> Upload -> Dashboard -> Download flow

## Known Constraints
- Currently no job persistence from /api/transcribe (it's synchronous)
- Will need to modify /api/transcribe to save job records (can do in this task or Task 8)
- For now, dashboard may show empty list until Task 8 (background jobs) is implemented

## Decision: Should Task 7 Save Jobs?
**Yes** - To make the dashboard useful immediately, we should modify `/api/transcribe` to:
1. Create a job record before transcription
2. Update job status after transcription completes
3. Store result file path in database

This is a small addition to Task 4's work and makes Task 7 immediately verifiable.
