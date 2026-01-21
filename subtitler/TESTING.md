# Testing

This document describes how to run tests for the Subtitler project.

## Overview

The project has comprehensive test coverage across both backend and frontend:

- **Backend (Go)**: 84 tests across 7 test files
- **Frontend (TypeScript)**: 44 tests across 2 test files
- **Total**: 128 tests

## Backend Tests

### Running Tests

```bash
cd backend

# Run all tests with verbose output
/home/trevor/go/bin/go test ./... -v

# Run tests with short output
/home/trevor/go/bin/go test ./...

# Run tests for a specific package
/home/trevor/go/bin/go test ./auth -v
/home/trevor/go/bin/go test ./db -v
/home/trevor/go/bin/go test ./crypto -v
/home/trevor/go/bin/go test ./align -v
/home/trevor/go/bin/go test ./totp -v

# Run a specific test by name
/home/trevor/go/bin/go test -run TestAuthLogin -v
```

> **Note**: Go may not be in PATH on some systems. Use the full path `/home/trevor/go/bin/go` if needed.

### Test Files

| File | Package | Description |
|------|---------|-------------|
| `api_test.go` | main | API integration tests (endpoints, auth, sessions) |
| `main_test.go` | main | Unit tests (SRT formatting, video upload form) |
| `auth/auth_test.go` | auth | Password hashing, token generation, validation, sessions |
| `db/db_test.go` | db | Database CRUD, migrations, video/transcription/burn job lifecycle |
| `crypto/crypto_test.go` | crypto | File encryption/decryption with age library |
| `align/align_test.go` | align | Transcript alignment algorithm |
| `totp/totp_test.go` | totp | TOTP code generation and validation |

### Test Categories

**Unit Tests**:
- Password hashing and verification
- Token and ID generation
- Email and password validation
- SRT timestamp formatting
- TOTP code generation
- Word alignment algorithms

**Integration Tests** (api_test.go):
- Health endpoint
- User registration (valid/invalid inputs, duplicates)
- User login (valid/invalid credentials)
- Session management (logout, cookies)
- Video listing (by user, by session)
- Transcription status retrieval
- SRT download
- Segment editing with validation

**Database Tests**:
- Schema migration
- Video CRUD operations
- Transcription lifecycle (create, update, complete, fail)
- Burn job lifecycle
- TOTP secret storage
- Cascade deletes
- Expiry queries

## Frontend Tests

### Running Tests

```bash
cd frontend

# Install dependencies (if not already done)
npm install

# Run all tests once
npm test

# Run tests in watch mode (for development)
npm test:watch
```

### Test Files

| File | Description |
|------|-------------|
| `src/utils/format.test.ts` | 23 tests for formatting utilities |
| `src/utils/validation.test.ts` | 21 tests for validation utilities |

### Test Categories

**Format Utilities** (`format.test.ts`):
- `formatBytes()` - human-readable file sizes (bytes, KB, MB, GB)
- `formatTime()` - SRT-style timestamps (HH:MM:SS,mmm)
- `formatTimeForInput()` - input field format (HH:MM:SS.mmm)
- `parseTimeInput()` - parse multiple time formats
- `formatDate()` - date formatting
- `escapeHtml()` - XSS prevention
- `getStatusInfo()` - transcription status badges

**Validation Utilities** (`validation.test.ts`):
- `validateEmail()` - email format validation
- `validatePassword()` - password strength requirements
- `validateTotpCode()` - 6-digit TOTP validation
- `validateVideoFile()` - video file type and size limits
- `validateSegmentTiming()` - subtitle timing validation

### Test Framework

The frontend uses [Vitest](https://vitest.dev/) v3.2+ for testing. Configuration is in `vitest.config.ts`:

```typescript
import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    include: ['src/**/*.test.ts'],
    globals: true,
  },
});
```

## Writing New Tests

### Backend

Create test files with the `_test.go` suffix in the same package:

```go
// mypackage/mycode_test.go
package mypackage

import "testing"

func TestMyFunction(t *testing.T) {
    result := MyFunction("input")
    if result != "expected" {
        t.Errorf("got %v, want %v", result, "expected")
    }
}
```

For table-driven tests:

```go
func TestMyFunction(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"case 1", "input1", "output1"},
        {"case 2", "input2", "output2"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := MyFunction(tt.input)
            if result != tt.expected {
                t.Errorf("got %v, want %v", result, tt.expected)
            }
        })
    }
}
```

### Frontend

Create test files with the `.test.ts` suffix in the `src/` directory:

```typescript
// src/utils/myutil.test.ts
import { describe, it, expect } from 'vitest';
import { myFunction } from './myutil';

describe('myFunction', () => {
  it('handles basic case', () => {
    expect(myFunction('input')).toBe('expected');
  });

  it('handles edge case', () => {
    expect(myFunction('')).toBe('default');
  });
});
```

## Test Coverage

To view test coverage:

### Backend

```bash
cd backend
/home/trevor/go/bin/go test ./... -cover

# Generate HTML coverage report
/home/trevor/go/bin/go test ./... -coverprofile=coverage.out
/home/trevor/go/bin/go tool cover -html=coverage.out -o coverage.html
```

### Frontend

```bash
cd frontend
npm test -- --coverage
```

## CI/CD Integration

Tests should be run before committing:

```bash
# Quick test check (both backend and frontend)
cd backend && /home/trevor/go/bin/go test ./... && cd ../frontend && npm test
```

All tests must pass before merging to trunk.
