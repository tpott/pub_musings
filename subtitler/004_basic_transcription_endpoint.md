# 004_basic_transcription_endpoint

**Implements Task:** 4

## Objective
Create a POST /api/transcribe endpoint that accepts audio file uploads and returns SRT-formatted subtitles. This endpoint ties together the file upload functionality (Task 3) and whisper.cpp integration (Task 2) into a complete transcription API.

## Current State
- File upload endpoint at `/api/upload` successfully receives and validates files (Task 3)
- Whisper.cpp integration in `internal/transcribe/` can transcribe audio files (Task 2)
- Storage validation supports common audio formats: mp3, wav, m4a, ogg, flac
- Storage validation supports video formats: mp4, webm, mkv, avi, mov
- Frontend has a basic upload UI

## Architecture Decision

The `/api/transcribe` endpoint will:
1. Accept multipart file uploads (similar to `/api/upload`)
2. Validate the file (reuse existing storage validation)
3. Save the file temporarily
4. Call the transcription service to process it
5. Return the SRT-formatted result
6. Clean up temporary files

For this initial version:
- No authentication required (matches current `/api/upload` behavior)
- No database persistence (just in-memory processing)
- Synchronous processing (user waits for transcription to complete)
- Return SRT format directly in response body

Future enhancements (Task 8 and beyond):
- Async job processing with database
- Multiple output formats (Task 9)
- User authentication and job history (Task 6, 7)

## Plan

### 1. Create Transcription Handler
- Add `handleTranscribe` function to `backend/cmd/server/main.go`:
  - Accept POST requests with multipart file upload
  - Validate file using existing `storage.ValidateFile`
  - Save file to temporary location
  - Start transcription service if not already running
  - Call `transcribe.Service.TranscribeFile` with SRT format
  - Return SRT content with appropriate Content-Type
  - Clean up temporary file after processing
- Handle CORS for frontend requests (match existing `/api/upload` pattern)
- Add proper error handling and status codes

### 2. Update Main Server
- Initialize transcription service in `main()`:
  - Load configuration
  - Create `transcribe.NewService(config)`
  - Start whisper-server subprocess via `service.Start()`
  - Register shutdown handler to call `service.Stop()`
- Register `/api/transcribe` route
- Ensure graceful cleanup on server shutdown

### 3. Add Response Types
- Create `TranscribeResponse` struct:
  - `Success bool`
  - `Message string`
  - `Transcript string` (SRT-formatted subtitle content)
  - `Filename string` (original filename)
  - `Format string` (output format, e.g., "srt")
  - `Duration float64` (optional: transcription time in seconds)

### 4. Implement Temporary File Management
- Use Go's `os.CreateTemp()` for temporary file creation
- Ensure files are cleaned up even if errors occur (use defer)
- Create temporary directory if needed (e.g., `./tmp/transcribe`)
- Add helper function `cleanupTempFile(path string)`

### 5. Integration Testing
- Create `backend/cmd/server/transcribe_test.go`:
  - Test transcribe endpoint with valid audio file
  - Verify SRT format output
  - Test with multiple audio formats (mp3, wav, m4a)
  - Test error cases: invalid file, unsupported format, missing file
  - Test file size limits
- Use whisper.cpp sample file: `~/Github/whisper.cpp/samples/jfk.wav`
- Verify expected transcript content (should contain "ask not")

### 6. End-to-End Testing with Playwright
- Create `frontend/tests/transcribe.spec.ts`:
  - Start backend and frontend servers
  - Navigate to upload page
  - Upload a test audio file
  - Wait for transcription to complete
  - Verify SRT content is displayed or downloadable
  - Verify error handling (file too large, invalid format)
- This provides human-verifiable evidence the feature works

### 7. Update Documentation
- Update `README.md`:
  - Add example curl command for `/api/transcribe`
  - Document response format
  - Add to "Running Tests" section
- Update `INTEGRATION_TESTS.md`:
  - Add transcribe endpoint integration test
  - Document expected behavior and test data
- Update `CLAUDE.md`:
  - Note that whisper-server must be running for `/api/transcribe`
  - Document temporary file handling

## Implementation Details

### Transcription Endpoint Flow
```go
func handleTranscribe(w http.ResponseWriter, r *http.Request) {
    // 1. CORS and method validation
    if enableCORS(w, r) {
        return
    }

    // 2. Parse multipart form
    r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
    r.ParseMultipartForm(32 << 20)

    // 3. Get file and validate
    file, header, err := r.FormFile("file")
    storage.ValidateFile(header.Filename, header.Size)

    // 4. Save to temporary location
    tmpFile, err := os.CreateTemp("./tmp/transcribe", "audio-*"+filepath.Ext(header.Filename))
    defer os.Remove(tmpFile.Name())
    io.Copy(tmpFile, file)

    // 5. Transcribe with SRT format
    transcript, err := transcribeService.TranscribeFile(tmpFile.Name(), transcribe.TranscribeOptions{
        Format: transcribe.FormatSRT,
    })

    // 6. Return SRT response
    w.Header().Set("Content-Type", "text/plain; charset=utf-8")
    w.Write([]byte(transcript))
}
```

### Server Initialization with Transcription Service
```go
func main() {
    // Load config
    cfg := config.LoadConfig()

    // Initialize transcription service
    transcribeService = transcribe.NewService(cfg)
    if err := transcribeService.Start(); err != nil {
        log.Fatalf("Failed to start transcription service: %v", err)
    }
    defer transcribeService.Stop()

    // Register routes
    http.HandleFunc("/api/upload", handleUpload)
    http.HandleFunc("/api/transcribe", handleTranscribe)

    // Start server
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

### Temporary File Management
- Directory: `./tmp/transcribe/`
- Pattern: `audio-<random>.<ext>`
- Cleanup: Immediate deletion after transcription completes
- Error handling: `defer os.Remove(path)` ensures cleanup even on errors

## Testing Acceptance Criteria

1. **Unit Tests** (backend/cmd/server/transcribe_test.go):
   - Endpoint returns 405 for non-POST methods
   - Endpoint returns 400 for missing file
   - Endpoint returns 400 for invalid file format
   - Endpoint returns 413 for file too large
   - Mock transcription service to test handler logic in isolation

2. **Integration Tests** (Document in INTEGRATION_TESTS.md):
   - POST audio file to `/api/transcribe`
   - Verify response contains valid SRT format:
     - Subtitle numbering (1, 2, 3...)
     - Timestamps in format `HH:MM:SS,mmm --> HH:MM:SS,mmm`
     - Text content matching expected transcript
   - Test with JFK sample audio: verify "ask not" appears in output
   - Test with multiple file formats (mp3, wav, m4a)
   - Verify temporary files are cleaned up after processing
   - Test concurrent requests (2-3 files at once)

3. **Playwright E2E Tests** (frontend/tests/transcribe.spec.ts):
   - Upload audio file via frontend form
   - Wait for transcription result (may take 10-60 seconds)
   - Verify SRT content is displayed in UI
   - Verify user can download SRT file
   - Take screenshot of successful transcription result

4. **Command Verification**:
   - Run all backend tests: `cd backend && go test ./... -v`
   - Run integration test: `cd backend && go test ./cmd/server -v -run TestTranscribe`
   - Run Playwright test: `cd frontend && npm test`
   - Manual curl test:
     ```bash
     curl -X POST http://localhost:8080/api/transcribe \
       -F "file=@~/Github/whisper.cpp/samples/jfk.wav" \
       -o output.srt
     cat output.srt  # Should show SRT format with "ask not" text
     ```

## Sample Test Data

Use whisper.cpp sample files:
- **JFK Speech**: `~/Github/whisper.cpp/samples/jfk.wav`
  - Expected: SRT output with "ask not what your country can do for you"
  - Duration: ~11 seconds
  - Expected transcription time: 5-30 seconds depending on system

## Notes

- Transcription can take 5-60 seconds depending on audio length and system resources
- The endpoint is synchronous (user waits), which is acceptable for this initial version
- Future Task 8 will add async job processing with database persistence
- whisper-server subprocess is started once at server startup, not per request
- The medium model provides good accuracy for English content
- For non-English audio, users would need to specify language in future versions

## Error Handling

Handle these error cases:
- `400 Bad Request`: Invalid file format, missing file, file too large
- `500 Internal Server Error`: Transcription service failure, whisper-server not running
- `503 Service Unavailable`: whisper-server not started or crashed

Return JSON error responses matching the existing pattern:
```json
{
  "success": false,
  "message": "Error description"
}
```

## Success Criteria

Task 4 acceptance criteria met when:
- POST /api/transcribe accepts audio file upload
- Returns SRT-formatted subtitles
- Handles common audio formats: mp3, wav, m4a
- Integration tests pass with whisper.cpp sample audio
- Playwright tests demonstrate end-to-end functionality
- Documentation includes curl example and test instructions
