package api

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/tpott/pub_musings/peekaboo/backend/llm"
)

// Test files for WebSocket configuration and mock infrastructure.

func TestGetIdleTimeout(t *testing.T) {
	tests := []struct {
		name   string
		envVal string
		want   time.Duration
	}{
		{"default when not set", "", 5 * time.Minute},
		{"custom value", "120", 120 * time.Second},
		{"invalid value returns default", "not-a-number", 5 * time.Minute},
		{"zero returns default", "0", 5 * time.Minute},
		{"negative returns default", "-100", 5 * time.Minute},
		{"large value", "3600", 3600 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVal != "" {
				os.Setenv("WEBSOCKET_IDLE_TIMEOUT_SECS", tt.envVal)
				defer os.Unsetenv("WEBSOCKET_IDLE_TIMEOUT_SECS")
			} else {
				os.Unsetenv("WEBSOCKET_IDLE_TIMEOUT_SECS")
			}

			got := getIdleTimeout()
			if got != tt.want {
				t.Errorf("getIdleTimeout() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetMaxConnections(t *testing.T) {
	tests := []struct {
		name   string
		envVal string
		want   int
	}{
		{"default when not set", "", 100},
		{"custom value", "50", 50},
		{"invalid value returns default", "not-a-number", 100},
		{"zero returns default", "0", 100},
		{"negative returns default", "-10", 100},
		{"large value", "1000", 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVal != "" {
				os.Setenv("WEBSOCKET_MAX_CONNECTIONS", tt.envVal)
				defer os.Unsetenv("WEBSOCKET_MAX_CONNECTIONS")
			} else {
				os.Unsetenv("WEBSOCKET_MAX_CONNECTIONS")
			}

			got := getMaxConnections()
			if got != tt.want {
				t.Errorf("getMaxConnections() = %v, want %v", got, tt.want)
			}
		})
	}
}

// mockLLMProvider implements llm.Provider for testing.
type mockLLMProvider struct {
	subject   string
	err       error
	healthErr error
}

func (m *mockLLMProvider) ExtractIntent(ctx context.Context, text string) (*llm.IntentResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &llm.IntentResult{Subject: m.subject}, nil
}

func (m *mockLLMProvider) HealthCheck(ctx context.Context) error {
	return m.healthErr
}

// dynamicMockLLMProvider returns different subjects based on call count.
type dynamicMockLLMProvider struct {
	subjects  []string
	callCount int
}

func (m *dynamicMockLLMProvider) ExtractIntent(ctx context.Context, text string) (*llm.IntentResult, error) {
	subject := m.subjects[m.callCount%len(m.subjects)]
	m.callCount++
	return &llm.IntentResult{Subject: subject}, nil
}

func (m *dynamicMockLLMProvider) HealthCheck(ctx context.Context) error {
	return nil
}

func TestConnectionTracker(t *testing.T) {
	t.Run("TryAcquire succeeds under limit", func(t *testing.T) {
		tracker := NewConnectionTracker(3)

		if !tracker.TryAcquire() {
			t.Error("first TryAcquire should succeed")
		}
		if tracker.Count() != 1 {
			t.Errorf("expected count 1, got %d", tracker.Count())
		}

		if !tracker.TryAcquire() {
			t.Error("second TryAcquire should succeed")
		}
		if tracker.Count() != 2 {
			t.Errorf("expected count 2, got %d", tracker.Count())
		}

		if !tracker.TryAcquire() {
			t.Error("third TryAcquire should succeed")
		}
		if tracker.Count() != 3 {
			t.Errorf("expected count 3, got %d", tracker.Count())
		}
	})

	t.Run("TryAcquire fails at limit", func(t *testing.T) {
		tracker := NewConnectionTracker(2)

		tracker.TryAcquire()
		tracker.TryAcquire()

		if tracker.TryAcquire() {
			t.Error("third TryAcquire should fail when at limit")
		}
		if tracker.Count() != 2 {
			t.Errorf("expected count to stay at 2, got %d", tracker.Count())
		}
	})

	t.Run("Release frees slot", func(t *testing.T) {
		tracker := NewConnectionTracker(2)

		tracker.TryAcquire()
		tracker.TryAcquire()

		if tracker.TryAcquire() {
			t.Error("should be at limit")
		}

		tracker.Release()
		if tracker.Count() != 1 {
			t.Errorf("expected count 1 after release, got %d", tracker.Count())
		}

		if !tracker.TryAcquire() {
			t.Error("TryAcquire should succeed after Release")
		}
	})

	t.Run("Max returns correct value", func(t *testing.T) {
		tracker := NewConnectionTracker(42)
		if tracker.Max() != 42 {
			t.Errorf("expected max 42, got %d", tracker.Max())
		}
	})
}
