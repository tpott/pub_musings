# Integration Tests

This document describes the integration tests for Subtitler and how to run them.

## Overview

Integration tests verify that components work together correctly, particularly:
- whisper.cpp integration for audio transcription
- whisper-server subprocess management
- End-to-end transcription workflows

## Prerequisites

Before running integration tests, ensure:
1. whisper.cpp is installed at `~/Github/whisper.cpp/`
2. whisper-server binary is built at `~/Github/whisper.cpp/build/bin/whisper-server`
3. Medium model is downloaded at `~/Github/whisper.cpp/models/ggml-medium.bin`
4. JFK sample audio exists at `~/Github/whisper.cpp/samples/jfk.wav`
5. ffmpeg is installed (for audio format conversion)

See [INSTALL.md](INSTALL.md) for setup instructions.

## Running Integration Tests

### Run All Integration Tests

```bash
cd backend
/home/trevor/go/bin/go test ./internal/transcribe -v

# Also test the transcribe endpoint
cd backend
/home/trevor/go/bin/go test ./cmd/server -v -run TestHandleTranscribe_Integration
```

### Run Specific Test

```bash
# Test JFK transcription
cd backend
/home/trevor/go/bin/go test ./internal/transcribe -v -run TestTranscribeJFK

# Test whisper-server lifecycle
cd backend
/home/trevor/go/bin/go test ./internal/transcribe -v -run TestWhisperServerLifecycle

# Test error handling
cd backend
/home/trevor/go/bin/go test ./internal/transcribe -v -run TestTranscribeNonexistentFile
```

### Skip Integration Tests

Integration tests are skipped when running in short mode:

```bash
cd backend
/home/trevor/go/bin/go test ./... -short
```

This is useful for quick test runs that don't require external dependencies.

## Test Descriptions

### TestTranscribeJFK

Tests transcription using the JFK sample audio file.

**What it tests:**
- Starting whisper-server subprocess
- Transcribing audio file
- Multiple output formats (text, SRT, VTT)
- Verifying transcript contains expected content

**Expected output:**
- Text format: Contains "ask not" and "country"
- SRT format: Contains timing markers (`-->`)
- VTT format: Starts with `WEBVTT`

**Runtime:** ~30-60 seconds (depends on CPU and model loading time)

### TestTranscribeNonexistentFile

Tests error handling when trying to transcribe a file that doesn't exist.

**What it tests:**
- Error handling for missing files
- Proper error messages

**Runtime:** ~1-2 seconds

### TestWhisperServerLifecycle

Tests starting and stopping the whisper-server subprocess.

**What it tests:**
- Starting whisper-server
- Health check verification
- Server status tracking
- Graceful shutdown

**Runtime:** ~30 seconds (model loading time)

## Configuration

Integration tests use the same configuration as the main application. Override defaults with environment variables:

```bash
export WHISPER_SERVER_PATH="/custom/path/to/whisper-server"
export WHISPER_MODEL_PATH="/custom/path/to/model.bin"
export WHISPER_SERVER_PORT=9090
export WHISPER_THREADS=4

cd backend
/home/trevor/go/bin/go test ./internal/transcribe -v
```

## Troubleshooting

### Tests are skipped

If you see messages like "whisper-server not found, skipping integration test", verify:
1. whisper.cpp is installed correctly
2. Paths match the configuration (see prerequisites above)

### Tests timeout

If tests timeout during model loading:
1. Increase the timeout in `whisper_server.go` (`waitForReady` function)
2. Use a smaller model (e.g., `tiny` or `base`)
3. Increase `WHISPER_THREADS` for faster processing

### Port already in use

If whisper-server port (default 9090) is already in use:
```bash
export WHISPER_SERVER_PORT=9091
cd backend
/home/trevor/go/bin/go test ./internal/transcribe -v
```

### Model loading takes too long

The medium model is ~1.5GB and takes time to load. First test run will be slower. Subsequent tests in the same session may be faster if the model is cached in memory.

Consider using a smaller model for development:
```bash
cd ~/Github/whisper.cpp
bash ./models/download-ggml-model.sh base

export WHISPER_MODEL_PATH="$HOME/Github/whisper.cpp/models/ggml-base.bin"
cd backend
/home/trevor/go/bin/go test ./internal/transcribe -v
```

## CI/CD Integration

For automated testing in CI/CD pipelines:

```bash
# Skip integration tests in CI (unless whisper.cpp is installed)
go test ./... -short

# Or run integration tests if dependencies are available
go test ./internal/transcribe -v -timeout 5m
```

Consider:
- Using a smaller model in CI (base or small)
- Caching the model to speed up builds
- Running integration tests only on specific branches or PRs

## Adding New Integration Tests

When adding new integration tests to `transcribe_test.go`:

1. Use `testing.Short()` to allow skipping:
   ```go
   if testing.Short() {
       t.Skip("Skipping integration test in short mode")
   }
   ```

2. Check prerequisites exist before running:
   ```go
   if _, err := os.Stat(cfg.WhisperServerPath); os.IsNotExist(err) {
       t.Skipf("whisper-server not found, skipping test")
   }
   ```

3. Clean up resources (stop servers, delete temp files):
   ```go
   defer service.Stop()
   ```

4. Use descriptive test names and subtests:
   ```go
   t.Run("SpecificScenario", func(t *testing.T) {
       // Test code
   })
   ```

5. Document the test in this file with:
   - What it tests
   - Expected output
   - Approximate runtime

### TestHandleTranscribe_Integration (Task 4 - Completed)

Tests the `/api/transcribe` endpoint with real whisper.cpp integration.

**Location:** `backend/cmd/server/transcribe_test.go`

**What it tests:**
- Starting transcription service
- Uploading audio file to `/api/transcribe` endpoint
- Processing with whisper.cpp
- Receiving SRT-formatted response
- Verifying transcript contains expected content ("ask not")
- Verifying SRT format (contains `-->` timestamps)

**Expected output:**
- HTTP 200 status
- JSON response with `success: true`
- `transcript` field containing valid SRT format
- `format` field set to "srt"
- Transcript contains "ask not" from JFK speech

**Runtime:** ~30-60 seconds (depends on CPU and model)

**Example run:**
```bash
cd backend
/home/trevor/go/bin/go test ./cmd/server -v -run TestHandleTranscribe_Integration
```

### Other Endpoint Tests (Task 4 - Completed)

Additional tests in `backend/cmd/server/transcribe_test.go`:
- `TestHandleTranscribe_MethodNotAllowed`: Verifies non-POST requests are rejected
- `TestHandleTranscribe_MissingFile`: Verifies requests without files are rejected
- `TestHandleTranscribe_InvalidFormat`: Verifies invalid file formats are rejected
- `TestHandleTranscribe_ServiceNotInitialized`: Verifies behavior when service isn't started

## Future Test Scenarios

As more features are implemented, integration tests will expand to cover:

### Authentication Flow (Task 6)
- Register new user
- Login with credentials
- Access protected endpoints
- Logout

### Job Processing (Task 8)
- Submit multiple jobs
- Verify queue processing
- Check job status updates
- Download completed results

### Error Handling
- Upload unsupported file format
- Exceed file size limit
- Handle transcription failures
- Test network interruptions
