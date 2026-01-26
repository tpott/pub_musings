# Unit Tests

This document describes the unit testing strategy for the Subtitler project.

## Status

Unit tests will be implemented as features are developed in subsequent tasks.

## Frontend Tests

Framework: To be determined (likely Vitest or Jest)

Location: `frontend/src/**/*.test.ts`

Run tests:
```bash
cd frontend
npm test
```

## Backend Tests

Framework: Go standard testing library

Location: `backend/internal/**/*_test.go`

Run tests:
```bash
cd backend
go test ./...
```

Run with coverage:
```bash
cd backend
go test -cover ./...
```

## Test Coverage Goals

- Target: 80% code coverage for business logic
- Critical paths (auth, transcription) should have >90% coverage
- Integration tests will supplement unit tests

## Testing Guidelines

1. Write tests alongside feature implementation
2. Test edge cases and error conditions
3. Mock external dependencies (whisper.cpp, database, file system)
4. Keep tests fast and independent
5. Use table-driven tests for Go code

## To Be Implemented

- [ ] Frontend component tests
- [ ] Frontend utility function tests
- [ ] Backend handler tests
- [ ] Backend business logic tests
- [ ] Mock implementations for external services
