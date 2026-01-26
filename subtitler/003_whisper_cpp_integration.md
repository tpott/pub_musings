# 003_whisper_cpp_integration

**Implements Task:** 2

## Objective
Integrate whisper.cpp into the Go backend so it can transcribe audio files and return transcript text. This task creates the foundation for the transcription service.

## Current State
- whisper.cpp is installed at `~/Github/whisper.cpp`
- whisper-server binary is already built at `~/Github/whisper.cpp/build/bin/whisper-server`
- Medium model is available at `~/Github/whisper.cpp/models/ggml-medium.bin`
- ffmpeg is available for audio format conversion
- Backend has `internal/transcribe/` package directory (empty placeholder)
- Go backend at `backend/` with basic HTTP server

## Architecture Decision

We have two main approaches for whisper.cpp integration:

**Option A: Subprocess Integration (Recommended)**
- Call `whisper-server` as a subprocess or use the CLI binary directly
- Simpler implementation, less coupling with whisper.cpp internals
- Easier to update whisper.cpp independently
- Good process isolation

**Option B: CGO Integration**
- Use CGO to call whisper.cpp C++ library directly
- More complex, requires CGO build setup and linking
- Tighter integration but harder to maintain

**Decision: Use Option A (Subprocess Integration)**
- Start with calling the whisper-server HTTP API internally
- This gives us flexibility: we can run whisper-server as a separate service or embedded subprocess
- Better matches our architecture (Go backend calling whisper.cpp)

## Plan

### 1. Create Transcription Service Package
- Implement `internal/transcribe/service.go` with:
  - `TranscribeFile(audioPath string, options TranscribeOptions) (string, error)` function
  - `TranscribeOptions` struct for model selection, language, format, etc.
  - Support for output formats: text, json, srt, vtt
- Handle calling whisper-server via HTTP (using net/http client)
- Handle audio format conversion if needed (via ffmpeg)

### 2. Implement Whisper Server Management
- Create `internal/transcribe/whisper_server.go`:
  - Function to start whisper-server as a subprocess
  - Health check endpoint monitoring
  - Graceful shutdown handling
  - Configuration for model path, threads, etc.
- Use localhost port (e.g., 9090) for whisper-server communication
- Ensure only one whisper-server instance runs at a time

### 3. Add Configuration
- Create `internal/config/config.go` for application configuration:
  - Whisper model path (default: `~/Github/whisper.cpp/models/ggml-medium.bin`)
  - Whisper server binary path (default: `~/Github/whisper.cpp/build/bin/whisper-server`)
  - Whisper server port (default: 9090)
  - Number of threads for transcription
  - Temporary directory for audio file processing
- Support loading config from environment variables

### 4. Add Dependencies
- Add required Go modules to `go.mod`:
  - No external dependencies needed (using stdlib net/http)
- Document any system dependencies (ffmpeg, whisper.cpp)

### 5. Create Integration Test
- Create `backend/internal/transcribe/transcribe_test.go`:
  - Test with a sample audio file (can use whisper.cpp samples)
  - Verify transcription output is reasonable text
  - Test different output formats (text, srt)
  - Test error handling (invalid file, missing model)
- Document the test in `INTEGRATION_TESTS.md`

### 6. Update Documentation
- Update `README.md` with:
  - Prerequisites: whisper.cpp installation and model download
  - How to run transcription tests
- Create/update `INSTALL.md` with:
  - Instructions for setting up whisper.cpp if not already installed
  - Model download instructions
  - Environment variable configuration
- Update `CLAUDE.md` with:
  - Location of whisper.cpp installation
  - Configuration details
  - How transcription integration works

## Implementation Details

### Whisper Server Lifecycle
```go
// Start whisper-server as subprocess
cmd := exec.Command(
    whisperServerPath,
    "-m", modelPath,
    "-t", strconv.Itoa(threads),
    "--port", strconv.Itoa(port),
    "--convert", // Enable ffmpeg conversion
)

// Wait for health check before using
// Make HTTP requests to localhost:9090/inference

// On shutdown, send SIGTERM to subprocess
```

### Transcription API Call
```go
// Prepare multipart request
file, _ := os.Open(audioPath)
body := &bytes.Buffer{}
writer := multipart.NewWriter(body)
part, _ := writer.CreateFormFile("file", filepath.Base(audioPath))
io.Copy(part, file)
writer.WriteField("response_format", format) // "srt", "json", "text", "vtt"

// Send to whisper-server
resp, err := http.Post(
    fmt.Sprintf("http://localhost:%d/inference", port),
    writer.FormDataContentType(),
    body,
)
```

## Testing Acceptance Criteria

1. **Unit Tests**:
   - Configuration loading works correctly
   - Whisper server process management functions work
   - Error handling for missing files/models

2. **Integration Tests** (Document in INTEGRATION_TESTS.md):
   - Start whisper-server subprocess successfully
   - Health check endpoint responds
   - Transcribe a test audio file (use whisper.cpp samples directory)
   - Verify transcript contains expected text
   - Test SRT format output has proper timing format
   - Test with multiple concurrent requests
   - Graceful shutdown stops whisper-server process

3. **Command Verification**:
   - Run integration tests and verify they pass
   - Verify whisper-server can be started manually with config

## Sample Test Audio

Use existing whisper.cpp samples:
- `~/Github/whisper.cpp/samples/jfk.wav` - JFK "ask not what your country can do" speech
- Expected transcript should contain "ask not what your country can do for you"

## Notes

- This implementation keeps whisper.cpp as an external dependency
- Future optimization: could explore CGO integration if subprocess overhead becomes an issue
- The whisper-server has a `--convert` flag that uses ffmpeg for audio conversion, which we'll enable
- Model loading happens once at whisper-server startup, not per-request
- The medium model provides a good balance of speed and accuracy

## Success Criteria

Task 2 acceptance criteria met when:
- Go backend can call whisper.cpp to transcribe an audio file and return the transcript text
- Integration handles model loading (medium model)
- Integration handles audio format conversion
- All tests pass
- Documentation is complete and commands work
