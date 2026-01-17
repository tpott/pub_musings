# 003: Basic File Upload UI

**Implements Task 3**

## Goal

Create a single-page upload form in the Astro frontend where users can select a video/audio file and submit it to the backend via multipart form data. No authentication required at this stage.

## Acceptance Criteria

- Astro frontend has a single-page upload form where users can select a video/audio file
- Form submits multipart data to the backend
- No authentication required

## Implementation Plan

### 1. Frontend: Create Upload Form Page

**File: `frontend/src/pages/index.astro`**

Replace the placeholder content with a proper upload form:

- HTML form with file input (accept audio/video formats)
- File size display and validation (10 minute video limit ~100-200MB)
- Submit button
- Basic styling for usability
- JavaScript to handle form submission via fetch API
- Display upload progress
- Show success/error messages

Supported formats:
- Audio: mp3, wav, m4a, ogg, flac
- Video: mp4, webm, mkv, avi, mov

### 2. Backend: Add File Upload Endpoint

**File: `backend/cmd/server/main.go`**

Add a POST `/api/upload` endpoint that:
- Accepts multipart/form-data
- Validates file type and size
- Saves file to a temporary location
- Returns a success response with file details (name, size, type)
- Handles errors gracefully

**File: `backend/internal/storage/storage.go` (new)**

Create a storage package to handle file operations:
- `SaveUploadedFile(file, destPath) error` - Save uploaded file to disk
- `ValidateFile(filename, size) error` - Validate file type and size
- Define constants for max file size (200MB)
- Define allowed file extensions

### 3. Frontend: Add Result Display

Update `frontend/src/pages/index.astro` to show:
- Upload success message with file details
- Error messages if upload fails
- "Upload Another File" button to reset the form

### 4. Configuration

Update the Go backend to:
- Configure CORS to allow frontend origin (localhost:4321)
- Set up proper error handling middleware
- Configure maximum request body size (200MB)

## Testing

### Manual Testing

1. **Happy path**: Upload a small audio file (e.g., 1MB mp3)
   - Verify file is accepted
   - Verify success message is displayed
   - Verify file details are shown (name, size)

2. **File type validation**: Try uploading a text file
   - Verify error message is displayed

3. **File size validation**: Try uploading a file larger than 200MB
   - Verify error message is displayed

4. **Network error handling**: Stop the backend server and try uploading
   - Verify appropriate error message is displayed

### Automated Tests (Playwright)

**File: `frontend/tests/upload.spec.ts` (new)**

Create Playwright tests to verify:
- Form renders correctly
- File input accepts files
- Submit button triggers upload
- Success message appears on successful upload
- Error message appears on failed upload
- Form can be reset for another upload

### Backend Tests

**File: `backend/internal/storage/storage_test.go` (new)**

Unit tests for:
- `ValidateFile()` with valid extensions
- `ValidateFile()` with invalid extensions
- `ValidateFile()` with oversized files
- `SaveUploadedFile()` happy path

**File: `backend/cmd/server/upload_test.go` (new)**

Integration tests for:
- POST `/api/upload` with valid file
- POST `/api/upload` with invalid file type
- POST `/api/upload` with oversized file
- POST `/api/upload` with missing file

## Files to Create/Modify

### Create:
- `backend/internal/storage/storage.go`
- `backend/internal/storage/storage_test.go`
- `backend/cmd/server/upload_test.go`
- `frontend/tests/upload.spec.ts`
- `frontend/package.json` (add Playwright dependency)

### Modify:
- `frontend/src/pages/index.astro`
- `backend/cmd/server/main.go`

## Testing Commands

Add to README.md:

```bash
# Run Playwright tests
cd frontend && npm run test

# Run backend storage tests
cd backend && /home/trevor/go/bin/go test ./internal/storage -v

# Run backend upload endpoint tests
cd backend && /home/trevor/go/bin/go test ./cmd/server -v
```

## Dependencies

- Task 1 (Initialize project structure) - Complete ✓
- Playwright for frontend testing (will be installed)

## Notes

- This implementation does not include authentication (Task 6 dependency)
- Files are saved to a temporary location; proper storage will be implemented in Task 5
- No database integration yet; files are just stored locally
- No transcription happens in this task; that's Task 4
- We're focusing on the upload flow only: UI → Backend → File Save → Response
