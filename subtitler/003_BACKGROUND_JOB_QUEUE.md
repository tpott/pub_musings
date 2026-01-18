# 003_BACKGROUND_JOB_QUEUE

## Task 8: Background Job Queue Implementation

**Goal:** Transform the synchronous transcription endpoint into an async job queue system where users upload files, jobs are queued, workers process them in the background, and the dashboard shows status updates.

## Current State Analysis

From exploring the codebase:

1. **Database schema** - Jobs table exists with proper status tracking ('pending', 'processing', 'completed', 'failed')
2. **Jobs API** - Fully implemented (`GET /api/jobs`, `GET /api/jobs/{id}`, `GET /api/jobs/{id}/download`)
3. **Transcription service** - Working synchronously via whisper.cpp
4. **Dashboard UI** - Exists and ready to display jobs

**Critical Gap:** The `/api/transcribe` endpoint does NOT create job records in the database. It processes synchronously and returns results directly. Jobs and transcription are disconnected.

## Implementation Strategy

### Option A: Simple In-Process Worker (Recommended for MVP)

Keep everything in the Go process using channels:
- Upload → create job → send to channel → return job ID
- Worker goroutines consume channel → transcribe → update status
- Frontend polls `/api/jobs/{id}` for status
- No external dependencies (Redis, RabbitMQ)

**Pros:**
- Simple, no new dependencies
- Good enough for single-server MVP
- Easy to test
- Can migrate to Redis/RabbitMQ later if needed

**Cons:**
- Jobs lost if server restarts
- Can't scale horizontally
- Limited monitoring

### Option B: SQLite-Based Queue

Use SQLite database as job queue:
- Upload → create job with status='pending'
- Worker polls database for pending jobs
- Lock jobs using UPDATE with WHERE status='pending'
- Update status as processing progresses

**Pros:**
- No external dependencies
- Jobs survive server restarts
- Simple to implement
- Already have SQLite

**Cons:**
- Database polling overhead
- Not ideal for high-throughput
- Lock contention possible

**DECISION: Go with Option A for now** - Simpler, faster to implement, good enough for MVP. Can migrate to Redis later if needed.

## Architecture Design

```
User Browser
    │
    ↓ POST /api/upload (with file)
Go API Handler
    │
    ├─ Validate file
    ├─ Save to data/files/uploads/{user_id}/{job_id}/{filename}
    ├─ Create job record (status='pending')
    ├─ Send job ID to worker channel
    └─ Return 201 with job ID

    ↓ (channel)

Worker Goroutines (pool of N workers)
    │
    ├─ Read file from disk
    ├─ Update job status to 'processing'
    ├─ Call transcribe service
    ├─ Save transcript to data/files/results/{user_id}/{job_id}/{filename}.srt
    ├─ Update job with status='completed', transcript_path
    └─ (future) Send email notification

User Browser
    │
    ↓ GET /api/jobs/{id} (polling)
    │
    └─ Returns current job status
```

## Implementation Steps

### Step 1: Refactor /api/transcribe → /api/upload (Job Creation)

**File:** `backend/cmd/server/upload_handlers.go` (new file)

**Endpoint:** `POST /api/upload`

**Flow:**
1. Authenticate user (required)
2. Parse multipart form, validate file
3. Generate job ID and file paths
4. Save file to `data/files/uploads/{user_id}/{job_id}/{filename}.encrypted` (or unencrypted for MVP)
5. Create job record in database with status='pending'
6. Send job ID to worker channel
7. Return JSON response with job ID

**Response:**
```json
{
  "success": true,
  "message": "File uploaded successfully",
  "job_id": 42
}
```

### Step 2: Implement Worker Pool

**File:** `backend/internal/worker/worker.go` (new package)

**Components:**
- `JobQueue` - Buffered channel of job IDs
- `StartWorkerPool(numWorkers, queue, db, transcribeService)` - Starts N goroutines
- `worker(id, queue, db, transcribeService)` - Worker goroutine that:
  1. Reads job ID from channel
  2. Fetches job from database
  3. Updates status to 'processing'
  4. Calls transcription service
  5. Saves result to file system
  6. Updates job with status='completed' or 'failed'

**Worker Goroutine Logic:**
```go
func worker(id int, queue <-chan int, db *db.DB, ts *transcribe.Service) {
    for jobID := range queue {
        // Fetch job
        job, err := db.GetJobByID(jobID)
        if err != nil { continue }

        // Update to processing
        db.UpdateJobStatus(jobID, "processing")

        // Transcribe
        transcript, err := ts.TranscribeFile(job.FilePath, options)
        if err != nil {
            db.UpdateJobFailed(jobID, err.Error())
            continue
        }

        // Save result
        transcriptPath := buildResultPath(job)
        saveTranscript(transcriptPath, transcript)

        // Update completed
        db.UpdateJobCompleted(jobID, transcriptPath)
    }
}
```

### Step 3: Wire Up in main.go

**File:** `backend/cmd/server/main.go`

**Changes:**
1. Create job queue channel: `jobQueue := make(chan int, 100)`
2. Start worker pool: `worker.StartWorkerPool(4, jobQueue, db, transcribeService)`
3. Pass queue to upload handler
4. Keep `/api/transcribe` for backward compatibility (optional)

### Step 4: Update Dashboard to Poll for Status

**File:** `frontend/src/pages/dashboard.astro`

**Changes:**
- Already fetches jobs from `/api/jobs`
- May need to add auto-refresh or polling
- Show "processing" status badge
- Disable download for pending/processing jobs

### Step 5: Add Tests

**Unit Tests:**
- `backend/internal/worker/worker_test.go` - Test worker logic with mocks

**Integration Tests:**
- `backend/cmd/server/upload_test.go` - Test /api/upload endpoint
- Verify job creation
- Verify worker picks up job
- Verify status transitions

**E2E Tests:**
- `frontend/tests/upload-and-wait.spec.ts` - Upload file, poll for completion, download result

## File Structure

```
backend/
├── cmd/server/
│   ├── main.go              # Modified: Add worker pool startup
│   ├── upload_handlers.go   # NEW: POST /api/upload
│   ├── upload_test.go       # NEW: Tests for upload
│   └── transcribe.go        # Keep for backward compat (optional)
├── internal/
│   ├── worker/              # NEW: Worker package
│   │   ├── worker.go        # Worker pool and goroutines
│   │   └── worker_test.go   # Unit tests
│   ├── storage/             # Existing: File storage utilities
│   └── db/                  # Existing: Database operations
└── data/
    └── files/
        ├── uploads/{user_id}/{job_id}/   # User uploads
        └── results/{user_id}/{job_id}/   # Transcription results
```

## Acceptance Criteria (done_when)

Task 8 is complete when:

```bash
# 1. Upload file via new /api/upload endpoint
curl -X POST http://localhost:8080/api/upload \
  -H "Cookie: subtitler_token=YOUR_TOKEN" \
  -F "file=@test.mp3" \
  | jq '.job_id'  # Returns job ID

# 2. Check job status (should show "pending" or "processing")
curl -X GET http://localhost:8080/api/jobs/{job_id} \
  -H "Cookie: subtitler_token=YOUR_TOKEN" \
  | jq '.job.status'  # Returns "pending" or "processing"

# 3. Wait a few seconds, check again (should show "completed")
sleep 10
curl -X GET http://localhost:8080/api/jobs/{job_id} \
  -H "Cookie: subtitler_token=YOUR_TOKEN" \
  | jq '.job.status'  # Returns "completed"

# 4. Download transcript
curl -X GET http://localhost:8080/api/jobs/{job_id}/download \
  -H "Cookie: subtitler_token=YOUR_TOKEN" \
  -o transcript.srt

# Verify transcript.srt contains valid SRT content
cat transcript.srt
```

## Migration Notes

**Backward Compatibility:**
- Keep `/api/transcribe` endpoint for now (synchronous)
- New users should use `/api/upload` (async with queue)
- Dashboard only shows jobs from database

**Future Enhancements (Not in this task):**
- Redis/RabbitMQ for distributed queue
- WebSocket for real-time status updates
- Email notifications on job completion
- Job retry logic
- Job cancellation

## Risks and Mitigations

**Risk:** Jobs lost if server restarts
- **Mitigation:** Document limitation, plan for SQLite-backed queue or Redis in future

**Risk:** Worker exhausts file descriptors or memory
- **Mitigation:** Limit worker pool size (4 workers), add resource monitoring

**Risk:** Long-running jobs block workers
- **Mitigation:** Set timeout on transcription service calls

**Risk:** File storage grows unbounded
- **Mitigation:** Add cleanup job (future task), document storage management

## Testing Strategy

1. **Unit tests:** Worker pool logic with mocks
2. **Integration tests:** Upload → worker → completion flow
3. **Manual testing:** Upload via curl, verify status transitions
4. **E2E tests:** Playwright test uploading and waiting for completion
5. **Stress test:** Upload 10 files concurrently, verify all complete

## Success Metrics

- ✅ Upload returns immediately with job ID (< 500ms)
- ✅ Worker picks up job within 1 second
- ✅ Status transitions correctly (pending → processing → completed)
- ✅ Transcript saved to correct file path
- ✅ Download endpoint serves correct file
- ✅ Dashboard shows real-time status
- ✅ All tests pass
