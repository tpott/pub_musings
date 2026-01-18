package email

import (
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client := NewClient("test-api-key", "test@example.com", true)

	if client.apiKey != "test-api-key" {
		t.Errorf("expected apiKey to be 'test-api-key', got '%s'", client.apiKey)
	}
	if client.fromEmail != "test@example.com" {
		t.Errorf("expected fromEmail to be 'test@example.com', got '%s'", client.fromEmail)
	}
	if !client.enabled {
		t.Error("expected enabled to be true")
	}
}

func TestSendJobCompleted_Disabled(t *testing.T) {
	// When email is disabled, should return nil without sending
	client := NewClient("", "", false)

	err := client.SendJobCompleted("user@example.com", 123, "test.mp4", "srt", 30*time.Second)
	if err != nil {
		t.Errorf("expected no error when disabled, got %v", err)
	}
}

func TestSendJobFailed_Disabled(t *testing.T) {
	// When email is disabled, should return nil without sending
	client := NewClient("", "", false)

	err := client.SendJobFailed("user@example.com", 123, "test.mp4", "file not found")
	if err != nil {
		t.Errorf("expected no error when disabled, got %v", err)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{30 * time.Second, "30 seconds"},
		{1 * time.Minute, "1 minutes"},
		{90 * time.Second, "1 minutes, 30 seconds"},
		{5 * time.Minute, "5 minutes"},
		{1 * time.Hour, "1 hours"},
		{90 * time.Minute, "1 hours, 30 minutes"},
		{2*time.Hour + 15*time.Minute, "2 hours, 15 minutes"},
	}

	for _, tt := range tests {
		result := formatDuration(tt.duration)
		if result != tt.expected {
			t.Errorf("formatDuration(%v) = %s; want %s", tt.duration, result, tt.expected)
		}
	}
}

// Note: We don't test actual API calls to Resend in unit tests
// Those would be integration tests requiring a real API key
