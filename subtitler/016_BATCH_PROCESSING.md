# 016_BATCH_PROCESSING

## Overview

Add batch processing capability so users can upload multiple files at once. Each file becomes a separate job, and the dashboard already supports displaying multiple jobs.

## Current State

- Frontend: Single file selection (`let selectedFile = null`)
- Backend: Single file extraction (`r.FormFile("file")`)
- Dashboard: Already supports multiple jobs
- Job model: One job per file (no changes needed)

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Multiple files per request | Yes | Better UX than repeated single uploads |
| Shared options | Yes | Language applies to all files in batch |
| Backend response | Array of jobs | Return success/failure per file |
| Partial failure handling | Continue processing | Don't fail entire batch for one bad file |
| Max files per batch | 10 | Prevent abuse, keep UI manageable |

## Implementation

### Backend Changes (upload_handlers.go)

1. Add new response type for batch uploads:
```go
type BatchUploadResponse struct {
    Success     bool            `json:"success"`
    Message     string          `json:"message"`
    Jobs        []JobResult     `json:"jobs,omitempty"`
    FailedFiles []FailedFile    `json:"failed_files,omitempty"`
}

type JobResult struct {
    JobID    int64  `json:"job_id"`
    Filename string `json:"filename"`
}

type FailedFile struct {
    Filename string `json:"filename"`
    Error    string `json:"error"`
}
```

2. Modify `handleUpload()` to process multiple files:
   - Use `r.MultipartForm.File["file"]` instead of `r.FormFile("file")`
   - Loop through all files, validate each
   - Create job for each valid file
   - Track successes and failures separately
   - Return batch response

### Frontend Changes (index.astro)

1. Enable multiple file selection:
   - Add `multiple` attribute to file input
   - Change `selectedFile` to `selectedFiles` array

2. Update UI:
   - Show list of selected files with sizes
   - Allow removing individual files before upload
   - Display total count and combined size
   - Add max files validation (10 files)

3. Update submission:
   - Append all files to FormData
   - Handle batch response showing success/failure per file
   - Redirect to dashboard on any success

## Files to Modify

| File | Changes |
|------|---------|
| `backend/cmd/server/upload_handlers.go` | Batch upload logic |
| `frontend/src/pages/index.astro` | Multiple file selection UI |

## Verification

- Upload 2-3 files at once
- Dashboard shows all job statuses
- Works with mixed valid/invalid files (partial success)
- Language option applies to all files
