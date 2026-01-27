# Chunked Upload Specification

## Overview

Implement chunked uploads to support large video files (>50MB) through Cloudflare's 100MB free-tier request limit. This allows reliable uploads of videos up to the existing 500MB limit with resumability.

## Problem Statement

1. **Cloudflare limit**: Free tier has 100MB request body limit
2. **Network reliability**: Large uploads fail on unstable connections
3. **Progress tracking**: Users need accurate progress for large files
4. **Resumability**: Failed uploads should resume from where they left off

## Design

### Chunk Size Strategy

- **Default chunk size**: 50MB (well under Cloudflare's 100MB limit)
- **Minimum chunk size**: 5MB (for testing and low-bandwidth scenarios)
- **Configurable**: Via `CHUNK_SIZE` env var on backend

Files smaller than 50MB continue using the existing single-request upload.

### API Endpoints

#### POST /api/upload/init

Initialize a chunked upload session.

**Request:**
```json
{
  "filename": "video.mp4",
  "size": 150000000,
  "content_type": "video/mp4",
  "chunk_size": 52428800
}
```

**Response:**
```json
{
  "upload_session_id": "abc123",
  "chunk_size": 52428800,
  "total_chunks": 3,
  "expires_at": "2026-01-26T18:00:00Z"
}
```

**Validation:**
- MIME type must be in allowed whitelist
- Total size must be under `MAX_UPLOAD_SIZE` (500MB default)
- Anonymous users checked against 2-upload limit

#### POST /api/upload/chunk

Upload a single chunk.

**Request (multipart/form-data):**
- `upload_session_id`: Session ID from init
- `chunk_index`: 0-based chunk index
- `chunk`: The chunk data

**Response:**
```json
{
  "chunk_index": 0,
  "received_bytes": 52428800,
  "total_received": 52428800,
  "progress": 33
}
```

**Validation:**
- Session must exist and not be expired
- Chunk index must be valid (0 to total_chunks - 1)
- Chunk must not already be uploaded (idempotent for retries)
- Chunk size must match expected (except last chunk)

#### POST /api/upload/complete

Finalize the upload after all chunks received.

**Request:**
```json
{
  "upload_session_id": "abc123"
}
```

**Response:**
```json
{
  "upload_id": "xyz789",
  "filename": "video.mp4",
  "size": 150000000,
  "message": "Upload complete"
}
```

**Process:**
1. Verify all chunks received
2. Reassemble chunks into final file
3. Validate video with ffprobe
4. Encrypt file
5. Generate thumbnail
6. Create database records
7. Clean up chunk files
8. Return same response format as single-file upload

#### GET /api/upload/status/{session_id}

Check upload session status (for resumability).

**Response:**
```json
{
  "upload_session_id": "abc123",
  "filename": "video.mp4",
  "total_size": 150000000,
  "total_chunks": 3,
  "received_chunks": [0, 1],
  "received_bytes": 104857600,
  "progress": 66,
  "status": "in_progress",
  "expires_at": "2026-01-26T18:00:00Z"
}
```

### Backend Storage

#### Upload Sessions Table

```sql
CREATE TABLE upload_sessions (
    id TEXT PRIMARY KEY,
    filename TEXT NOT NULL,
    content_type TEXT NOT NULL,
    total_size INTEGER NOT NULL,
    chunk_size INTEGER NOT NULL,
    total_chunks INTEGER NOT NULL,
    user_id TEXT,
    session_id TEXT,
    status TEXT DEFAULT 'in_progress',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    completed_at TIMESTAMP
);
```

#### Upload Chunks Table

```sql
CREATE TABLE upload_chunks (
    id TEXT PRIMARY KEY,
    upload_session_id TEXT NOT NULL,
    chunk_index INTEGER NOT NULL,
    chunk_path TEXT NOT NULL,
    size INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (upload_session_id) REFERENCES upload_sessions(id),
    UNIQUE(upload_session_id, chunk_index)
);
```

#### Chunk Storage

- Chunks stored in `uploads/chunks/{session_id}/` directory
- Named `chunk_{index}.part` (e.g., `chunk_0.part`, `chunk_1.part`)
- Chunks are NOT encrypted (only final reassembled file is encrypted)
- Expired sessions cleaned up by existing auto-delete scheduler

### Frontend Implementation

#### Upload Logic

```typescript
// Pseudocode for chunked upload
async function uploadLargeFile(file: File) {
  const CHUNK_SIZE = 50 * 1024 * 1024; // 50MB

  // Use single upload for small files
  if (file.size <= CHUNK_SIZE) {
    return uploadFile(file); // existing implementation
  }

  // Initialize chunked upload
  const session = await fetch('/api/upload/init', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      filename: file.name,
      size: file.size,
      content_type: file.type,
      chunk_size: CHUNK_SIZE
    })
  }).then(r => r.json());

  // Check for resume capability
  const existingSession = localStorage.getItem(`upload:${file.name}:${file.size}`);
  if (existingSession) {
    const status = await checkUploadStatus(existingSession);
    if (status.status === 'in_progress') {
      // Resume from existing session
      session = status;
    }
  }

  // Store session for resumability
  localStorage.setItem(`upload:${file.name}:${file.size}`, session.upload_session_id);

  // Upload chunks sequentially
  for (let i = 0; i < session.total_chunks; i++) {
    if (session.received_chunks?.includes(i)) continue; // Skip already uploaded

    const start = i * CHUNK_SIZE;
    const end = Math.min(start + CHUNK_SIZE, file.size);
    const chunk = file.slice(start, end);

    const formData = new FormData();
    formData.append('upload_session_id', session.upload_session_id);
    formData.append('chunk_index', i.toString());
    formData.append('chunk', chunk);

    await uploadChunk(formData, (progress) => {
      // Calculate overall progress
      const chunkProgress = (i + progress / 100) / session.total_chunks;
      showProgress(Math.round(chunkProgress * 100));
    });
  }

  // Complete upload
  const result = await fetch('/api/upload/complete', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ upload_session_id: session.upload_session_id })
  }).then(r => r.json());

  // Clean up localStorage
  localStorage.removeItem(`upload:${file.name}:${file.size}`);

  return result;
}
```

#### Progress Display

- Show per-chunk progress within overall progress
- Format: "Uploading chunk 2/3 (65%)"
- Overall progress calculated as: `(completed_chunks + current_chunk_progress) / total_chunks`

#### Resume UI

When a file matches a stored session:
1. Check session status via `/api/upload/status/{session_id}`
2. If still valid, prompt user: "Resume previous upload? (66% complete)"
3. If expired, start fresh

### Rate Limiting

- `/api/upload/init`: 10/min per IP (same as current upload)
- `/api/upload/chunk`: 60/min per IP (higher since multiple chunks)
- `/api/upload/complete`: 10/min per IP
- `/api/upload/status/{session_id}`: 30/min per IP

### Session Expiration & Cleanup

**Expiration timeout:** 24 hours from session creation

**Cleanup scheduler** (`runCleanup()` in main.go):
- Runs on server startup
- Runs every hour thereafter
- Cleanup operations:

1. **Expired sessions cleanup:**
   - Queries `upload_sessions` where `status = 'in_progress'` AND `expires_at < now()`
   - Deletes session record and chunk records from database
   - Deletes chunk files from disk (`uploads/chunks/{session_id}/chunk_*.part`)
   - Removes the session's chunks directory

2. **Orphan directory cleanup:**
   - Scans `uploads/chunks/` for directories
   - Checks each directory name against `upload_sessions` table
   - Removes directories that don't have matching database records
   - Handles edge cases: server crashes during upload, manual database cleanup

**Completed sessions:**
- When `/api/upload/complete` succeeds, chunk files are deleted immediately
- Session record status is set to "complete" but kept for auditing
- Completed session records are NOT automatically deleted (only expire if `in_progress`)

**Frontend localStorage cleanup:**
- Client stores upload session IDs in localStorage for resume capability
- Stale sessions (older than 48 hours) should be cleaned from localStorage on app startup
- See: `frontend/src/utils/upload-session.ts`

### Security Considerations

1. **Session validation**: All chunk endpoints verify session ownership (user_id or session_id)
2. **Chunk validation**: Size must match expected (within tolerance for last chunk)
3. **MIME type**: Validated at init, verified at complete via ffprobe
4. **Rate limiting**: Prevents abuse of init endpoint
5. **Expiration**: Prevents disk exhaustion from abandoned uploads

### Migration

Create migration file `003_add_upload_sessions.sql`:

```sql
-- Upload sessions for chunked uploads
CREATE TABLE IF NOT EXISTS upload_sessions (
    id TEXT PRIMARY KEY,
    filename TEXT NOT NULL,
    content_type TEXT NOT NULL,
    total_size INTEGER NOT NULL,
    chunk_size INTEGER NOT NULL,
    total_chunks INTEGER NOT NULL,
    user_id TEXT,
    session_id TEXT,
    status TEXT DEFAULT 'in_progress',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    completed_at TIMESTAMP
);

-- Upload chunks tracking
CREATE TABLE IF NOT EXISTS upload_chunks (
    id TEXT PRIMARY KEY,
    upload_session_id TEXT NOT NULL,
    chunk_path TEXT NOT NULL,
    chunk_index INTEGER NOT NULL,
    size INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (upload_session_id) REFERENCES upload_sessions(id),
    UNIQUE(upload_session_id, chunk_index)
);

-- Index for cleanup queries
CREATE INDEX IF NOT EXISTS idx_upload_sessions_expires_at ON upload_sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_upload_sessions_status ON upload_sessions(status);
```

### Testing Plan

1. **Unit tests (backend)**:
   - Init validation (MIME type, size limits)
   - Chunk upload and storage
   - Complete reassembly
   - Session expiration
   - Resume with missing chunks

2. **Unit tests (frontend)**:
   - Chunk size calculation
   - Progress calculation
   - Resume detection

3. **E2E test**:
   - Upload file > 50MB split into chunks
   - Verify all endpoints work
   - Mock large file for CI (avoid slow tests)

### Backwards Compatibility

- Existing `POST /api/upload` remains unchanged
- Frontend uses chunked upload only for files > 50MB
- Small files continue using existing single-request flow
- No breaking changes to existing API

## Implementation Phases

### Phase 1: Backend API
1. Add database migration for upload_sessions and upload_chunks tables
2. Implement POST /api/upload/init
3. Implement POST /api/upload/chunk
4. Implement POST /api/upload/complete
5. Implement GET /api/upload/status/{session_id}
6. Add cleanup for expired sessions

### Phase 2: Frontend
1. Add chunked upload logic to upload.astro
2. Update progress display for chunk-by-chunk progress
3. Add resume capability with localStorage
4. Handle errors and retries per-chunk

### Phase 3: Testing
1. Backend unit tests for all new endpoints
2. Frontend unit tests for chunk logic
3. E2E test for full flow

## Related Files

- `backend/main.go` - Add new endpoints
- `backend/db/db.go` - Add session/chunk database operations
- `backend/db/migrations/003_add_upload_sessions.sql` - Migration
- `frontend/src/pages/upload.astro` - Update upload logic
- `docs/API.md` - Document new endpoints
