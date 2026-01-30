# Path Security Specification

This document describes the path validation strategy used in the subtitler backend to prevent path traversal attacks.

## Overview

Path traversal attacks attempt to access files outside the intended directories by using sequences like `../` to navigate up the directory tree. The `backend/pathvalidator` package provides centralized validation to ensure all file operations stay within allowed directories.

## Threat Model

### Attack Vectors

| Attack Type | Example | Target |
|-------------|---------|--------|
| Basic traversal | `../../../etc/passwd` | System files |
| Encoded traversal | `..%2F..%2F..%2Fetc/passwd` | URL-encoded paths |
| Null byte injection | `file.txt\x00.jpg` | Extension bypass |
| Symlink traversal | `uploads/link -> /etc/passwd` | Symlink targets |
| Path normalization | `uploads/../secret/file` | Bypassing checks |
| Mixed separators | `uploads\..\..\secret` | Windows/Unix paths |

### Protected Resources

File paths come from multiple sources:
- User-uploaded filenames
- Database-stored file paths
- URL path parameters
- API request bodies

All of these must be validated before file operations.

## Package: `backend/pathvalidator`

### Location

```
backend/pathvalidator/pathvalidator.go
backend/pathvalidator/pathvalidator_test.go
```

### Initialization

```go
// Create validator with allowed base directories
validator, err := pathvalidator.New("/opt/subtitler/uploads", "/opt/subtitler/data")
if err != nil {
    log.Fatal(err)
}

// Or use MustNew for initialization blocks (panics on error)
var pathValidator = pathvalidator.MustNew("/opt/subtitler/uploads", "/opt/subtitler/data")
```

### Error Types

| Error | Description |
|-------|-------------|
| `ErrPathTraversal` | Detected `..` sequence or null bytes |
| `ErrOutsideBaseDir` | Path resolves outside allowed directories |
| `ErrEmptyPath` | Empty path provided |
| `ErrAbsolutePath` | Absolute path not allowed (context-dependent) |
| `ErrSymlinkTraversal` | Symlink points outside allowed directory |

### Core Functions

#### ValidatePath

Validates a path for traversal patterns:

```go
// Catches obvious traversal attempts
err := validator.ValidatePath("../etc/passwd")  // ErrPathTraversal

// Allows normal paths
err := validator.ValidatePath("videos/abc123.mp4")  // nil
```

#### ValidateAbsolutePath

Validates that an absolute (or relative-converted-to-absolute) path is within allowed directories:

```go
// Path from database
err := validator.ValidateAbsolutePath("/opt/subtitler/uploads/abc123.mp4")  // nil

// Path outside allowed dirs
err := validator.ValidateAbsolutePath("/etc/passwd")  // ErrOutsideBaseDir
```

#### SafeJoin

Safely joins path components, validating the result:

```go
// Safe join
path, err := validator.SafeJoin("/opt/subtitler/uploads", "user123", "video.mp4")
// Result: /opt/subtitler/uploads/user123/video.mp4

// Traversal attempt blocked
path, err := validator.SafeJoin("/opt/subtitler/uploads", "../etc", "passwd")
// Error: ErrPathTraversal
```

#### ValidateAndResolve

Validates and resolves symlinks:

```go
// Validates both path and symlink target
realPath, err := validator.ValidateAndResolve("/opt/subtitler/uploads/video.mp4")
```

### Helper Functions

```go
// Check if a path is within a directory (convenience function)
ok, err := pathvalidator.IsWithin("/opt/subtitler/uploads/file.mp4", "/opt/subtitler/uploads")
// ok: true
```

## Implementation Details

### Validation Order

**Critical:** Check for traversal patterns BEFORE calling `filepath.Clean()`:

```go
// CORRECT ORDER
func (v *Validator) ValidatePath(path string) error {
    // 1. Check for traversal patterns in raw path
    if containsTraversalPatterns(path) {
        return ErrPathTraversal  // Catches "uploads/../etc/passwd"
    }

    // 2. Now it's safe to clean
    cleanPath := filepath.Clean(path)

    // 3. Additional checks on cleaned path
    // ...
}

// WRONG ORDER - Don't do this!
func validateWrong(path string) error {
    cleanPath := filepath.Clean(path)  // "uploads/../etc" becomes "etc"
    if containsTraversalPatterns(cleanPath) {  // Won't detect traversal!
        return ErrPathTraversal
    }
}
```

### Traversal Pattern Detection

The `containsTraversalPatterns()` function checks:

1. **Null bytes**: Used to truncate paths in some attacks
2. **`..` as path segment**: Only flags `..` as a complete segment, not substrings like `..test`
3. **Cross-platform**: Checks both `/` and OS-specific separators

```go
func containsTraversalPatterns(path string) bool {
    // Null byte check
    if strings.Contains(path, "\x00") {
        return true
    }

    // Check for ".." as complete path segment
    parts := strings.Split(path, string(filepath.Separator))
    for _, part := range parts {
        if part == ".." {
            return true
        }
    }

    // Also check forward slash for cross-platform
    if filepath.Separator != '/' {
        parts = strings.Split(path, "/")
        for _, part := range parts {
            if part == ".." {
                return true
            }
        }
    }

    return false
}
```

### Directory Containment Check

The `isWithinDir()` function ensures proper prefix matching:

```go
func isWithinDir(path, dir string) bool {
    path = filepath.Clean(path)
    dir = filepath.Clean(dir)

    // Exact match
    if path == dir {
        return true
    }

    // Prefix match with separator (prevents /foo/bar matching /foo/baz)
    return strings.HasPrefix(path, dir+string(filepath.Separator))
}
```

## Usage in Subtitler

### File Upload Handling

```go
func handleUpload(w http.ResponseWriter, r *http.Request) {
    // Get filename from upload
    filename := header.Filename

    // Validate filename doesn't contain traversal
    if err := pathValidator.ValidatePath(filename); err != nil {
        http.Error(w, "Invalid filename", http.StatusBadRequest)
        return
    }

    // Safe join with upload directory
    filepath, err := pathValidator.SafeJoin(uploadDir, filename)
    if err != nil {
        http.Error(w, "Invalid path", http.StatusBadRequest)
        return
    }

    // Now safe to write file
    // ...
}
```

### File Serving

```go
func handleDownload(w http.ResponseWriter, r *http.Request) {
    // Get path from database
    video, err := db.GetVideo(videoID)
    if err != nil {
        // handle error
    }

    // Validate path from database (defense-in-depth)
    if err := pathValidator.ValidateAbsolutePath(video.FilePath); err != nil {
        logging.ErrorContext(r.Context(), "Path validation failed",
            "error", err,
            "path", video.FilePath)
        http.Error(w, "Access denied", http.StatusForbidden)
        return
    }

    // Safe to serve
    http.ServeFile(w, r, video.FilePath)
}
```

### File Extension Validation

Combine with extension validation (see `validation.SanitizeFileExtension()`):

```go
func saveFile(filename string) error {
    // Validate path traversal
    if err := pathValidator.ValidatePath(filename); err != nil {
        return err
    }

    // Validate extension (no null bytes, path separators, etc.)
    ext, err := validation.SanitizeFileExtension(filepath.Ext(filename))
    if err != nil {
        return err
    }

    // ...
}
```

## Allowed Directories

The subtitler backend uses these allowed directories:

| Directory | Purpose |
|-----------|---------|
| `uploads/` | User-uploaded video files |
| `data/` | Database, encryption keys |
| `data/keys/` | Age encryption keys |
| `uploads/chunks/` | Chunked upload temporary files |

## Security Considerations

### Defense in Depth

Validate paths at multiple points:
1. **On input**: When receiving filename from user
2. **On join**: When constructing file paths
3. **Before serving**: Even for paths from database (in case of DB compromise)

### Symlink Protection

Use `ValidateAndResolve()` when symlinks are a concern:

```go
// Resolves symlinks and validates target
realPath, err := validator.ValidateAndResolve(path)
if err != nil {
    // Handles: ErrSymlinkTraversal
}
```

### Logging

Log validation failures for security monitoring:

```go
if err := pathValidator.ValidateAbsolutePath(path); err != nil {
    logging.WarnContext(r.Context(), "Path validation failed",
        "error", err,
        "path", path,
        "remote_addr", r.RemoteAddr)
    security.LogEvent(security.EventPathTraversalAttempt, ...)
}
```

## Testing

### Test Cases

The test suite covers:
- Empty paths
- Simple traversal (`../`)
- Deep traversal (`../../..`)
- Traversal in filename (`video/../../../etc/passwd`)
- Null byte injection
- Symlink traversal
- Safe joins that stay within bounds
- Joins that escape bounds

### Running Tests

```bash
cd backend
go test ./pathvalidator/... -v
```

## Related Documentation

- [specs/error-handling.md](error-handling.md) - Error message sanitization
- [LEARNINGS.md](../LEARNINGS.md) - Entry on path validation timing
- [backend/validation](../backend/validation/) - File extension sanitization
