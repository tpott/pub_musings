# Integration Tests

This document describes the integration testing strategy for the Subtitler project.

## Status

Integration tests will be implemented once core functionality is in place (Tasks 2-8).

## Test Scenarios

### 1. End-to-End Upload and Transcription
- Upload an audio file
- Wait for background processing
- Download SRT output
- Verify SRT format and content

### 2. Authentication Flow
- Register new user
- Login with credentials
- Access protected endpoints
- Logout

### 3. Job Processing
- Submit multiple jobs
- Verify queue processing
- Check job status updates
- Download completed results

### 4. Error Handling
- Upload unsupported file format
- Exceed file size limit
- Handle transcription failures
- Test network interruptions

## Test Environment

Integration tests will require:
- Test SQLite database
- Test file storage directory
- Mock whisper.cpp or small test model
- Test email provider (mock Resend API)

## Running Integration Tests

```bash
# Backend integration tests
cd backend
go test -tags=integration ./tests/integration

# Frontend E2E tests (framework TBD)
cd frontend
npm run test:e2e
```

## CI/CD Integration

Integration tests should run:
- On pull requests
- Before deployment
- On scheduled basis (nightly)

## To Be Implemented

- [ ] Test fixtures and sample files
- [ ] Test database setup/teardown scripts
- [ ] Mock external service responses
- [ ] E2E test suite
- [ ] Performance benchmarks
