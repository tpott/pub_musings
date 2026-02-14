# Testing

This document describes how to run tests for the Subtitler project.

## Overview

The project has comprehensive test coverage across both backend and frontend:

- **Backend (Go)**: 602 tests across 45 test files
- **Frontend (TypeScript)**: 900 tests across 34 test files
- **E2E (Playwright)**: 71 scenarios across 7 spec files (66 active, 5 permanently skipped)
- **Total**: 1,573 tests

## Backend Tests

### Running Tests

```bash
cd backend

# Run all tests with verbose output
go test ./... -v

# Run tests with short output
go test ./...

# Run tests for a specific package
go test ./auth -v
go test ./db -v
go test ./crypto -v
go test ./align -v
go test ./totp -v

# Run a specific test by name
go test -run TestAuthLogin -v
```

### Test Files

| File | Package | Description |
|------|---------|-------------|
| `api_auth_test.go` | main | Auth API tests (login, register, sessions, TOTP, password reset, magic link) |
| `api_upload_test.go` | main | Upload API tests (single + chunked uploads) |
| `api_video_test.go` | main | Video API tests (list, delete, transcription, segments, burn, caching) |
| `api_system_test.go` | main | System API tests (health, logs, feedback, admin, metrics) |
| `api_test_helpers_test.go` | main | Shared test infrastructure (testServer, registerHandlers) |
| `subtitle_format_test.go` | main | SRT formatting unit tests |
| `config_test.go` | main | Configuration and environment variable tests |
| `middleware_test.go` | main | HTTP middleware tests |
| `helpers_test.go` | main | Helper function tests |
| `video_helpers_test.go` | main | Video helper function tests |
| `goroutine_test.go` | main | Goroutine management tests |
| `align/align_test.go` | align | Transcript alignment algorithm |
| `align/lyrics_test.go` | align | Lyrics mode alignment tests |
| `audio/audio_test.go` | audio | Audio extraction and magic byte validation |
| `auth/auth_test.go` | auth | Password hashing, token generation, validation, sessions |
| `captcha/captcha_test.go` | captcha | hCaptcha verification tests |
| `crypto/crypto_test.go` | crypto | File encryption/decryption with age library |
| `crypto/multi_test.go` | crypto | Multi-key encryption for key rotation |
| `csrf/csrf_test.go` | csrf | CSRF token generation and validation |
| `db/db_video_test.go` | db | Video CRUD operations |
| `db/db_transcription_test.go` | db | Transcription lifecycle tests |
| `db/db_auth_password_test.go` | db | Password reset and email verification tests |
| `db/db_auth_totp_test.go` | db | TOTP secret storage and recovery codes |
| `db/db_session_test.go` | db | Session management tests |
| `db/db_burnjob_test.go` | db | Burn job lifecycle tests |
| `db/db_feedback_test.go` | db | Feedback CRUD tests |
| `db/db_upload_test.go` | db | Upload and chunked upload tests |
| `db/db_maintenance_test.go` | db | Maintenance and cleanup query tests |
| `db/db_transaction_test.go` | db | Transaction wrapper tests |
| `db/db_helpers_test.go` | db | Database helper function tests |
| `db/migrate_test.go` | db | Database migration tests |
| `db/profiler_test.go` | db | Query performance profiler tests |
| `email/email_test.go` | email | Email service and Resend API tests |
| `errmsg/errmsg_test.go` | errmsg | User-friendly error message tests |
| `httputil/contentdisposition_test.go` | httputil | Content-Disposition header generation tests |
| `language/language_test.go` | language | ISO 639-2 to ISO 639-1 language code conversion tests |
| `logging/logging_test.go` | logging | Structured logging tests |
| `metrics/metrics_test.go` | metrics | Prometheus metrics and path normalization |
| `pathvalidator/pathvalidator_test.go` | pathvalidator | Path traversal prevention tests |
| `ratelimit/ratelimit_test.go` | ratelimit | Rate limiter tests |
| `script/script_test.go` | script | Script detection and conversion tests |
| `security/events_test.go` | security | Security event audit logging tests |
| `totp/totp_test.go` | totp | TOTP code generation and validation |
| `totp/recovery_test.go` | totp | Recovery code generation and validation |
| `validation/validation_test.go` | validation | Input validation tests |

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
npm run test:watch
```

> **Note:** The frontend also has end-to-end (E2E) browser tests using Playwright. For E2E testing documentation, see [BROWSER_TESTING.md](BROWSER_TESTING.md).

### Test Files

| File | Description |
|------|-------------|
| `src/utils/api-schemas.test.ts` | API response schema validation and parsing |
| `src/utils/bionic.test.ts` | Bionic Reading formatting and state management |
| `src/utils/captcha.test.ts` | hCaptcha integration |
| `src/utils/console-forwarder.test.ts` | Dev mode console forwarding |
| `src/utils/csrf.test.ts` | CSRF token handling |
| `src/utils/dialog.test.ts` | Dialog utility (alerts, confirmations) |
| `src/utils/dom.test.ts` | DOM utility and element lookup functions |
| `src/utils/fetch-timeout.test.ts` | Fetch timeout error class and utilities |
| `src/utils/form-errors.test.ts` | Form field error display and clearing |
| `src/utils/form-validation.test.ts` | Form validation (blur handlers, password matching) |
| `src/utils/format.test.ts` | Formatting utilities (bytes, time, dates) |
| `src/utils/history.test.ts` | Undo/redo history management |
| `src/utils/html.test.ts` | HTML escaping for XSS prevention |
| `src/utils/nav-auth.test.ts` | Navigation auth check utilities |
| `src/utils/playback-speed.test.ts` | Playback speed constants and persistence |
| `src/utils/processing-speed.test.ts` | Transcription time estimation |
| `src/utils/segment-editor.test.ts` | Segment editing, undo/redo, feedback, navigation |
| `src/utils/session.test.ts` | Session and upload session management |
| `src/utils/settings-preferences.test.ts` | Bionic reading preferences setup |
| `src/utils/settings-sessions.test.ts` | Session management, parseUserAgent, revoke flow |
| `src/utils/settings-totp.test.ts` | TOTP setup, verification, disable, recovery codes |
| `src/utils/subtitle-sync.test.ts` | Subtitle synchronization, speed UI, burn subtitles |
| `src/utils/subtitles.test.ts` | Subtitle format generation (SRT, VTT, JSON) |
| `src/utils/transcription-polling.test.ts` | Polling, ETA calculation, SRT parsing |
| `src/utils/upload-file.test.ts` | File upload, chunked upload, progress tracking |
| `src/utils/validation.test.ts` | Input validation (email, password, TOTP, files) |
| `src/utils/videos-list.test.ts` | Video list rendering, date formatting, operations |
| `src/utils/videos-modal.test.ts` | Modal lifecycle, kid mode, keyboard handling |
| `src/utils/videos-progress.test.ts` | Real-time transcription progress polling on My Videos page |
| `src/utils/videos-subtitles.test.ts` | Subtitle caching (LRU), fetching, downloads |
| `src/utils/retranscribe-progress.test.ts` | Re-transcribe progress box show/update/hide/collapse |
| `src/utils/upload-collapsible.test.ts` | Collapsible section toggle with localStorage persistence |
| `src/utils/feedback-form.test.ts` | Feedback form state management, button reset, rating clear |
| `src/utils/segment-editor-timing.test.ts` | Timing adjustment buttons (nudge start/end, shift for 0.5s, constraints) |

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
go test ./... -cover

# Generate HTML coverage report
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
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
cd backend && go test ./... && cd ../frontend && npm test
```

All tests must pass before merging to trunk.
