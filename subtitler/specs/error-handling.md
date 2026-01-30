# Error Handling Specification

This document describes the error handling and message sanitization strategy used in the subtitler backend.

## Overview

Error messages returned to clients must be carefully controlled to prevent information disclosure vulnerabilities. Internal error details (file paths, IP addresses, database queries, API keys) should never be exposed to end users in production.

The `backend/errmsg` package provides a centralized approach to error message handling that:
1. Logs detailed errors for debugging
2. Returns user-friendly messages to clients
3. Allows verbose mode for development/debugging

## Package: `backend/errmsg`

### Location

```
backend/errmsg/errmsg.go
backend/errmsg/errmsg_test.go
```

### Core Concepts

#### Verbose Mode

Controlled by the `LOG_VERBOSE` environment variable:

| Value | Behavior |
|-------|----------|
| `false` (default) | Return user-friendly messages only |
| `true` or `1` | Return detailed error messages (development only) |

```go
// Check if verbose mode is enabled
if errmsg.IsVerbose() {
    // Development mode - can return detailed errors
}

// In tests, override verbose mode
errmsg.SetVerbose(true)
defer errmsg.SetVerbose(false)
```

#### User Messages

Pre-defined user-friendly messages for common error scenarios:

| Constant | Message | Use Case |
|----------|---------|----------|
| `ErrInternalServer` | "An unexpected error occurred. Please try again." | Generic server errors |
| `ErrBadRequest` | "Invalid request. Please check your input and try again." | Malformed requests |
| `ErrNotFound` | "The requested resource was not found." | Missing resources |
| `ErrUnauthorized` | "Authentication required. Please log in." | Missing auth |
| `ErrForbidden` | "You don't have permission to access this resource." | Permission denied |
| `ErrLoginFailed` | "Invalid email or password." | Auth failures |
| `ErrAccountLocked` | "Your account is temporarily locked. Please try again later." | Rate limiting |
| `ErrUploadFailed` | "Failed to upload file. Please try again." | Upload errors |
| `ErrFileTooLarge` | "File exceeds maximum allowed size." | Size validation |
| `ErrInvalidFileType` | "Invalid file type. Please upload a video file." | MIME validation |
| `ErrTranscribeFailed` | "Transcription failed. Please try again." | Whisper errors |
| `ErrVideoNotFound` | "Video not found." | Missing video |
| `ErrVideoProcessing` | "Video is still processing. Please wait." | In-progress |
| `ErrSubtitlesNotReady` | "Subtitles are not ready yet. Please wait for transcription to complete." | Pending transcription |
| `ErrBurnFailed` | "Failed to embed subtitles. Please try again." | FFmpeg errors |
| `ErrAlignFailed` | "Failed to align transcript. Please try again." | Alignment errors |
| `ErrDatabaseError` | "A database error occurred. Please try again." | SQLite errors |
| `ErrServiceUnavailable` | "A required service is temporarily unavailable. Please try again." | External service errors |
| `ErrEmailFailed` | "Failed to send email. Please try again." | Email service errors |
| `ErrEmailNotVerified` | "Please verify your email address first." | Unverified users |
| `ErrInvalidToken` | "Invalid or expired verification link." | Token validation |

### Helper Functions

Convenience functions that return appropriate user messages based on verbose mode:

```go
// For internal server errors
msg := errmsg.ForInternalError(err)

// For database errors
msg := errmsg.ForDatabaseError(err)

// For video not found
msg := errmsg.ForVideoNotFound(err)

// For transcription failures
msg := errmsg.ForTranscriptionError(err)

// For upload failures
msg := errmsg.ForUploadError(err)

// For external service failures
msg := errmsg.ForServiceError(err)

// For subtitle burning failures
msg := errmsg.ForBurnError(err)

// For transcript alignment failures
msg := errmsg.ForAlignError(err)
```

### Direct Usage

For custom messages:

```go
// Returns userMsg in production, detailedErr.Error() in verbose mode
msg := errmsg.UserMessage("Upload failed", err)
```

## Usage Guidelines

### When to Use errmsg

Use the errmsg package for errors from:
- Database operations
- File system operations
- External services (Whisper, email)
- Encryption/decryption
- Any operation that might expose internal details

### When NOT to Use errmsg

Return errors directly for:
- User input validation (email format, password requirements)
- Business logic validation (file too large, unsupported format)
- Authentication failures (already have generic messages)

**Rationale:** Validation errors help users fix their input. Internal errors don't.

### Pattern: Log Then Return

Always log the detailed error before returning the user-friendly message:

```go
func handleUpload(w http.ResponseWriter, r *http.Request) {
    err := saveFile(file)
    if err != nil {
        // Log detailed error for debugging
        logging.ErrorContext(r.Context(), "Failed to save uploaded file",
            "error", err,
            "filename", filename)

        // Return user-friendly message
        http.Error(w, errmsg.ForUploadError(err), http.StatusInternalServerError)
        return
    }
}
```

### Pattern: Structured Logging Context

Include request context in logs for traceability:

```go
logging.ErrorContext(r.Context(), "Operation failed",
    "error", err,
    "video_id", videoID,
    "user_id", userID,
    "request_id", r.Header.Get("X-Request-ID"))
```

## Sensitive Information to Protect

Error messages should NEVER contain:

| Category | Examples |
|----------|----------|
| File Paths | `/opt/subtitler/uploads/abc123.mp4`, `/home/user/.env` |
| IP Addresses | `10.0.2.2:8765`, `192.168.1.100` |
| Database Details | SQL queries, column names, table structures |
| API Keys | `sk-...`, `AKIA...` |
| Internal Service Names | `whisper-server`, `resend-api` |
| Stack Traces | Go panic traces with line numbers |
| Configuration Values | Environment variable values |

## Testing

### Sensitive Pattern Test

The test suite includes pattern matching to catch information leaks:

```go
func TestProductionErrorsDoNotLeakSensitiveInfo(t *testing.T) {
    sensitivePatterns := []string{
        `/home/`, `/opt/`, `/var/`, `/tmp/`,   // Paths
        `10.0.`, `192.168.`, `127.0.0.`,       // IPs
        `sql`, `SELECT`, `INSERT`, `DELETE`,   // SQL
        `sk-`, `AKIA`,                          // API keys
    }

    for _, pattern := range sensitivePatterns {
        if strings.Contains(userMessage, pattern) {
            t.Errorf("User message contains sensitive pattern: %s", pattern)
        }
    }
}
```

### Testing Verbose Mode

```go
func TestVerboseMode(t *testing.T) {
    // Save original
    original := errmsg.IsVerbose()
    defer errmsg.SetVerbose(original)

    // Test production mode
    errmsg.SetVerbose(false)
    msg := errmsg.ForInternalError(errors.New("detailed error"))
    if msg != errmsg.ErrInternalServer {
        t.Error("Expected user-friendly message in production mode")
    }

    // Test verbose mode
    errmsg.SetVerbose(true)
    msg = errmsg.ForInternalError(errors.New("detailed error"))
    if msg != "detailed error" {
        t.Error("Expected detailed error in verbose mode")
    }
}
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `LOG_VERBOSE` | `false` | Enable detailed error messages |

### Recommended Settings

| Environment | `LOG_VERBOSE` |
|-------------|---------------|
| Development | `true` |
| Staging | `true` |
| Production | `false` |

## Related Documentation

- [docs/ENV.md](../docs/ENV.md) - Environment variable reference
- [docs/ERROR_CODES.md](../docs/ERROR_CODES.md) - HTTP status codes and error format
- [LEARNINGS.md](../LEARNINGS.md) - Entry on production error message handling
