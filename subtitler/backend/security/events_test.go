package security

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/trevor/subtitler/backend/logging"
)

// setupTestLogger creates a test logger that writes to a buffer.
func setupTestLogger() *bytes.Buffer {
	buf := &bytes.Buffer{}
	handler := slog.NewTextHandler(buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logging.Logger = slog.New(handler)
	return buf
}

func TestLoginSuccess(t *testing.T) {
	buf := setupTestLogger()
	ctx := context.Background()

	LoginSuccess(ctx, "192.168.1.1", "user123", "test@example.com")

	output := buf.String()
	if !contains(output, EventLoginSuccess) {
		t.Errorf("expected event %s in output: %s", EventLoginSuccess, output)
	}
	if !contains(output, "192.168.1.1") {
		t.Errorf("expected IP in output: %s", output)
	}
	if !contains(output, "user123") {
		t.Errorf("expected user_id in output: %s", output)
	}
	if !contains(output, "test@example.com") {
		t.Errorf("expected email in output: %s", output)
	}
}

func TestLoginFailedPassword(t *testing.T) {
	buf := setupTestLogger()
	ctx := context.Background()

	LoginFailedPassword(ctx, "10.0.0.1", "user@test.com", 3)

	output := buf.String()
	if !contains(output, EventLoginFailedPassword) {
		t.Errorf("expected event %s in output: %s", EventLoginFailedPassword, output)
	}
	if !contains(output, "attempt_count=3") {
		t.Errorf("expected attempt_count in output: %s", output)
	}
}

func TestLoginFailedLocked(t *testing.T) {
	buf := setupTestLogger()
	ctx := context.Background()

	LoginFailedLocked(ctx, "10.0.0.1", "locked@test.com", 12)

	output := buf.String()
	if !contains(output, EventLoginFailedLocked) {
		t.Errorf("expected event %s in output: %s", EventLoginFailedLocked, output)
	}
	if !contains(output, "locked_minutes_remaining=12") {
		t.Errorf("expected locked_minutes_remaining in output: %s", output)
	}
}

func TestTwoFAEvents(t *testing.T) {
	tests := []struct {
		name     string
		fn       func(context.Context)
		expected string
	}{
		{
			name: "setup initiated",
			fn: func(ctx context.Context) {
				TwoFASetupInitiated(ctx, "1.2.3.4", "u1", "e@test.com")
			},
			expected: Event2FASetupInitiated,
		},
		{
			name: "enabled",
			fn: func(ctx context.Context) {
				TwoFAEnabled(ctx, "1.2.3.4", "u1", "e@test.com")
			},
			expected: Event2FAEnabled,
		},
		{
			name: "disabled",
			fn: func(ctx context.Context) {
				TwoFADisabled(ctx, "1.2.3.4", "u1", "e@test.com", false)
			},
			expected: Event2FADisabled,
		},
		{
			name: "verify success",
			fn: func(ctx context.Context) {
				TwoFAVerifySuccess(ctx, "1.2.3.4", "u1", "e@test.com")
			},
			expected: Event2FAVerifySuccess,
		},
		{
			name: "verify failed",
			fn: func(ctx context.Context) {
				TwoFAVerifyFailed(ctx, "1.2.3.4", "u1", "e@test.com")
			},
			expected: Event2FAVerifyFailed,
		},
		{
			name: "recovery code used",
			fn: func(ctx context.Context) {
				RecoveryCodeUsed(ctx, "1.2.3.4", "u1", "e@test.com")
			},
			expected: EventRecoveryCodeUsed,
		},
		{
			name: "recovery code failed",
			fn: func(ctx context.Context) {
				RecoveryCodeFailed(ctx, "1.2.3.4", "e@test.com")
			},
			expected: EventRecoveryCodeFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := setupTestLogger()
			ctx := context.Background()

			tt.fn(ctx)

			output := buf.String()
			if !contains(output, tt.expected) {
				t.Errorf("expected event %s in output: %s", tt.expected, output)
			}
		})
	}
}

func TestAccessDenied(t *testing.T) {
	buf := setupTestLogger()
	ctx := context.Background()

	AccessDenied(ctx, "192.168.0.1", "/api/admin", "not authorized", "user456")

	output := buf.String()
	if !contains(output, EventAccessDenied) {
		t.Errorf("expected event %s in output: %s", EventAccessDenied, output)
	}
	if !contains(output, "resource=/api/admin") {
		t.Errorf("expected resource in output: %s", output)
	}
	if !contains(output, "user_id=user456") {
		t.Errorf("expected user_id in output: %s", output)
	}
}

func TestAccessDeniedNotOwner(t *testing.T) {
	buf := setupTestLogger()
	ctx := context.Background()

	AccessDeniedNotOwner(ctx, "10.0.0.5", "user789", "video", "vid123")

	output := buf.String()
	if !contains(output, EventAccessDeniedOwner) {
		t.Errorf("expected event %s in output: %s", EventAccessDeniedOwner, output)
	}
	if !contains(output, "resource_type=video") {
		t.Errorf("expected resource_type in output: %s", output)
	}
	if !contains(output, "resource_id=vid123") {
		t.Errorf("expected resource_id in output: %s", output)
	}
}

func TestAccessDeniedNotAdmin(t *testing.T) {
	buf := setupTestLogger()
	ctx := context.Background()

	AccessDeniedNotAdmin(ctx, "10.0.0.6", "user456", "/metrics")

	output := buf.String()
	if !contains(output, EventAccessDeniedAdmin) {
		t.Errorf("expected event %s in output: %s", EventAccessDeniedAdmin, output)
	}
	if !contains(output, "user_id=user456") {
		t.Errorf("expected user_id in output: %s", output)
	}
	if !contains(output, "resource=/metrics") {
		t.Errorf("expected resource in output: %s", output)
	}
}

func TestRateLimitExceeded(t *testing.T) {
	buf := setupTestLogger()
	ctx := context.Background()

	RateLimitExceeded(ctx, "8.8.8.8", "/api/auth/login", "auth")

	output := buf.String()
	if !contains(output, EventRateLimitExceeded) {
		t.Errorf("expected event %s in output: %s", EventRateLimitExceeded, output)
	}
	if !contains(output, "endpoint=/api/auth/login") {
		t.Errorf("expected endpoint in output: %s", output)
	}
	if !contains(output, "limit_name=auth") {
		t.Errorf("expected limit_name in output: %s", output)
	}
}

func TestSessionEvents(t *testing.T) {
	buf := setupTestLogger()
	ctx := context.Background()

	SessionCreated(ctx, "1.1.1.1", "userA", "sess123")
	SessionRevoked(ctx, "1.1.1.1", "userA", "sess123", true)

	output := buf.String()
	if !contains(output, EventSessionCreated) {
		t.Errorf("expected event %s in output: %s", EventSessionCreated, output)
	}
	if !contains(output, EventSessionRevoked) {
		t.Errorf("expected event %s in output: %s", EventSessionRevoked, output)
	}
	if !contains(output, "self_revoke=true") {
		t.Errorf("expected self_revoke in output: %s", output)
	}
}

func TestPasswordResetEvents(t *testing.T) {
	t.Run("user found", func(t *testing.T) {
		buf := setupTestLogger()
		ctx := context.Background()

		PasswordResetRequested(ctx, "5.5.5.5", "found@test.com", true)

		output := buf.String()
		if !contains(output, EventPasswordResetRequested) {
			t.Errorf("expected event %s in output: %s", EventPasswordResetRequested, output)
		}
	})

	t.Run("user not found logs at debug", func(t *testing.T) {
		buf := setupTestLogger()
		ctx := context.Background()

		PasswordResetRequested(ctx, "5.5.5.5", "notfound@test.com", false)

		output := buf.String()
		// Should still contain the event but at debug level
		if !contains(output, EventPasswordResetRequested) {
			t.Errorf("expected event %s in output: %s", EventPasswordResetRequested, output)
		}
		if !contains(output, "user_found=false") {
			t.Errorf("expected user_found=false in output: %s", output)
		}
	})
}

func TestFileAccessEvents(t *testing.T) {
	buf := setupTestLogger()
	ctx := context.Background()

	FileAccess(ctx, "3.3.3.3", "userX", "video", "vid456")

	output := buf.String()
	if !contains(output, EventFileAccess) {
		t.Errorf("expected event %s in output: %s", EventFileAccess, output)
	}
	if !contains(output, "file_type=video") {
		t.Errorf("expected file_type in output: %s", output)
	}
}

func TestFileAccessDenied(t *testing.T) {
	buf := setupTestLogger()
	ctx := context.Background()

	FileAccessDenied(ctx, "6.6.6.6", "video", "vid789", "path traversal")

	output := buf.String()
	if !contains(output, EventFileAccessDenied) {
		t.Errorf("expected event %s in output: %s", EventFileAccessDenied, output)
	}
	if !contains(output, "reason=\"path traversal\"") {
		t.Errorf("expected reason in output: %s", output)
	}
}

func TestAccountLocked(t *testing.T) {
	buf := setupTestLogger()
	ctx := context.Background()

	AccountLocked(ctx, "7.7.7.7", "locked@example.com", 5)

	output := buf.String()
	if !contains(output, EventAccountLocked) {
		t.Errorf("expected event %s in output: %s", EventAccountLocked, output)
	}
	if !contains(output, "failed_attempts=5") {
		t.Errorf("expected failed_attempts in output: %s", output)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}
