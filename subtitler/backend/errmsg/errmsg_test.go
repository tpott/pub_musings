package errmsg

import (
	"errors"
	"testing"
)

func TestUserMessage(t *testing.T) {
	detailedErr := errors.New("sql: database is locked at /opt/subtitler/data/db.sqlite")

	tests := []struct {
		name        string
		verbose     bool
		userMsg     string
		detailedErr error
		want        string
	}{
		{
			name:        "non-verbose mode returns user message",
			verbose:     false,
			userMsg:     ErrDatabaseError,
			detailedErr: detailedErr,
			want:        ErrDatabaseError,
		},
		{
			name:        "verbose mode returns detailed error",
			verbose:     true,
			userMsg:     ErrDatabaseError,
			detailedErr: detailedErr,
			want:        detailedErr.Error(),
		},
		{
			name:        "nil error returns user message even in verbose mode",
			verbose:     true,
			userMsg:     ErrDatabaseError,
			detailedErr: nil,
			want:        ErrDatabaseError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetVerbose(tt.verbose)
			defer SetVerbose(false) // reset

			got := UserMessage(tt.userMsg, tt.detailedErr)
			if got != tt.want {
				t.Errorf("UserMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestForInternalError(t *testing.T) {
	err := errors.New("panic: runtime error in processVideo at main.go:1234")

	// Non-verbose mode
	SetVerbose(false)
	got := ForInternalError(err)
	if got != ErrInternalServer {
		t.Errorf("ForInternalError() non-verbose = %q, want %q", got, ErrInternalServer)
	}

	// Verify no path/line info leaked
	if contains(got, "main.go") || contains(got, "1234") || contains(got, "panic") {
		t.Error("ForInternalError() leaked internal details in non-verbose mode")
	}

	// Verbose mode
	SetVerbose(true)
	defer SetVerbose(false)
	got = ForInternalError(err)
	if got != err.Error() {
		t.Errorf("ForInternalError() verbose = %q, want %q", got, err.Error())
	}
}

func TestForDatabaseError(t *testing.T) {
	err := errors.New("SQLITE_BUSY: database is locked (database file: /opt/subtitler/data/subtitler.db)")

	SetVerbose(false)
	got := ForDatabaseError(err)
	if got != ErrDatabaseError {
		t.Errorf("ForDatabaseError() = %q, want %q", got, ErrDatabaseError)
	}

	// Verify no path leaked
	if contains(got, "/opt") || contains(got, "subtitler.db") || contains(got, "SQLITE") {
		t.Error("ForDatabaseError() leaked internal details")
	}
}

func TestForVideoNotFound(t *testing.T) {
	err := errors.New("video not found for upload ID: abc123xyz at path /opt/subtitler/uploads/abc123xyz.mp4.age")

	SetVerbose(false)
	got := ForVideoNotFound(err)
	if got != ErrVideoNotFound {
		t.Errorf("ForVideoNotFound() = %q, want %q", got, ErrVideoNotFound)
	}

	// Verify no path or ID leaked
	if contains(got, "/opt") || contains(got, "abc123xyz") || contains(got, ".age") {
		t.Error("ForVideoNotFound() leaked internal details")
	}
}

func TestForTranscriptionError(t *testing.T) {
	err := errors.New("whisper-server request failed after 3 attempts: connection refused at 10.0.2.2:8765")

	SetVerbose(false)
	got := ForTranscriptionError(err)
	if got != ErrTranscribeFailed {
		t.Errorf("ForTranscriptionError() = %q, want %q", got, ErrTranscribeFailed)
	}

	// Verify no internal details leaked
	if contains(got, "10.0.2.2") || contains(got, "8765") || contains(got, "whisper-server") {
		t.Error("ForTranscriptionError() leaked internal details")
	}
}

func TestForUploadError(t *testing.T) {
	err := errors.New("failed to write file to /opt/subtitler/uploads/xyz.mp4: no space left on device")

	SetVerbose(false)
	got := ForUploadError(err)
	if got != ErrUploadFailed {
		t.Errorf("ForUploadError() = %q, want %q", got, ErrUploadFailed)
	}

	// Verify no path leaked
	if contains(got, "/opt") || contains(got, "xyz.mp4") {
		t.Error("ForUploadError() leaked internal details")
	}
}

func TestForServiceError(t *testing.T) {
	err := errors.New("whisper-server error (status 500): internal server error at http://10.0.2.2:8765/transcribe")

	SetVerbose(false)
	got := ForServiceError(err)
	if got != ErrServiceUnavailable {
		t.Errorf("ForServiceError() = %q, want %q", got, ErrServiceUnavailable)
	}

	// Verify no URL leaked
	if contains(got, "10.0.2.2") || contains(got, "8765") || contains(got, "http://") {
		t.Error("ForServiceError() leaked internal details")
	}
}

func TestForBurnError(t *testing.T) {
	err := errors.New("ffmpeg error: Invalid data found when processing input at /tmp/ffmpeg_123456")

	SetVerbose(false)
	got := ForBurnError(err)
	if got != ErrBurnFailed {
		t.Errorf("ForBurnError() = %q, want %q", got, ErrBurnFailed)
	}

	// Verify no path leaked
	if contains(got, "/tmp") || contains(got, "ffmpeg") {
		t.Error("ForBurnError() leaked internal details")
	}
}

func TestForAlignError(t *testing.T) {
	err := errors.New("alignment failed: no matching segments found in whisper output at line 42 of align/align.go")

	SetVerbose(false)
	got := ForAlignError(err)
	if got != ErrAlignFailed {
		t.Errorf("ForAlignError() = %q, want %q", got, ErrAlignFailed)
	}

	// Verify no file/line info leaked
	if contains(got, "align.go") || contains(got, "line 42") {
		t.Error("ForAlignError() leaked internal details")
	}
}

func TestIsVerbose(t *testing.T) {
	SetVerbose(false)
	if IsVerbose() {
		t.Error("IsVerbose() = true, want false")
	}

	SetVerbose(true)
	defer SetVerbose(false)
	if !IsVerbose() {
		t.Error("IsVerbose() = false, want true")
	}
}

func TestProductionErrorsDoNotLeakDetails(t *testing.T) {
	SetVerbose(false)
	defer SetVerbose(false)

	// Test various error patterns that might appear in production
	sensitivePatterns := []struct {
		err     error
		handler func(error) string
		name    string
	}{
		{
			err:     errors.New("open /opt/subtitler/uploads/abc.mp4: permission denied"),
			handler: ForUploadError,
			name:    "file path in error",
		},
		{
			err:     errors.New("sql: no rows in result set (query: SELECT * FROM videos WHERE id='xyz')"),
			handler: ForDatabaseError,
			name:    "SQL query in error",
		},
		{
			err:     errors.New("connect: connection refused at 192.168.1.100:5432"),
			handler: ForServiceError,
			name:    "IP address in error",
		},
		{
			err:     errors.New("env var RESEND_API_KEY not set or invalid: re_abc123"),
			handler: ForInternalError,
			name:    "API key in error",
		},
		{
			err:     errors.New("user password hash: $2a$10$invalidhashhere"),
			handler: ForInternalError,
			name:    "password hash in error",
		},
	}

	for _, tc := range sensitivePatterns {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.handler(tc.err)
			// Result should be the generic user message, not the detailed error
			if result == tc.err.Error() {
				t.Errorf("handler leaked detailed error: %s", result)
			}
			// Result should not contain common sensitive patterns
			sensitiveStrings := []string{
				"/opt/", "/home/", "/tmp/", "/var/",
				"SELECT", "INSERT", "UPDATE", "DELETE",
				"192.168", "10.0.", "127.0.0",
				"$2a$", "re_", "sk_",
				".age", ".key", ".db",
			}
			for _, s := range sensitiveStrings {
				if contains(result, s) {
					t.Errorf("result contains sensitive string %q: %s", s, result)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
