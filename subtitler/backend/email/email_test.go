package email

import (
	"context"
	"strings"
	"testing"
)

func TestMockService(t *testing.T) {
	mock := NewMockService()

	if !mock.IsEnabled() {
		t.Error("Mock service should always be enabled")
	}

	if mock.GetEmailCount() != 0 {
		t.Error("New mock service should have no emails")
	}
}

func TestMockServiceSendEmail(t *testing.T) {
	mock := NewMockService()
	ctx := context.Background()

	err := mock.SendEmail(ctx, "test@example.com", "Test Subject", "<h1>HTML</h1>", "Text")
	if err != nil {
		t.Fatalf("SendEmail failed: %v", err)
	}

	if mock.GetEmailCount() != 1 {
		t.Errorf("Expected 1 email, got %d", mock.GetEmailCount())
	}

	email := mock.LastEmail()
	if email == nil {
		t.Fatal("LastEmail returned nil")
	}

	if email.To != "test@example.com" {
		t.Errorf("Expected to=test@example.com, got %s", email.To)
	}
	if email.Subject != "Test Subject" {
		t.Errorf("Expected subject=Test Subject, got %s", email.Subject)
	}
	if email.HtmlBody != "<h1>HTML</h1>" {
		t.Errorf("Expected htmlBody=<h1>HTML</h1>, got %s", email.HtmlBody)
	}
	if email.TextBody != "Text" {
		t.Errorf("Expected textBody=Text, got %s", email.TextBody)
	}
}

func TestMockServiceSendPasswordReset(t *testing.T) {
	mock := NewMockService()
	mock.SetAppURL("http://test.com")
	ctx := context.Background()

	err := mock.SendPasswordReset(ctx, "user@example.com", "reset-token-123")
	if err != nil {
		t.Fatalf("SendPasswordReset failed: %v", err)
	}

	email := mock.LastEmail()
	if email == nil {
		t.Fatal("LastEmail returned nil")
	}

	if email.To != "user@example.com" {
		t.Errorf("Expected to=user@example.com, got %s", email.To)
	}
	if email.Subject != "Reset your Subtitler password" {
		t.Errorf("Expected reset subject, got %s", email.Subject)
	}
	if !strings.Contains(email.HtmlBody, "http://test.com/reset-password?token=reset-token-123") {
		t.Error("HTML body should contain reset URL")
	}
	if !strings.Contains(email.TextBody, "http://test.com/reset-password?token=reset-token-123") {
		t.Error("Text body should contain reset URL")
	}
}

func TestMockServiceGetEmails(t *testing.T) {
	mock := NewMockService()
	ctx := context.Background()

	mock.SendEmail(ctx, "a@test.com", "Subject 1", "", "")
	mock.SendEmail(ctx, "b@test.com", "Subject 2", "", "")

	if mock.GetEmailCount() != 2 {
		t.Errorf("Expected 2 emails, got %d", mock.GetEmailCount())
	}

	emails := mock.GetEmails()
	if len(emails) != 2 {
		t.Errorf("GetEmails should return 2 emails, got %d", len(emails))
	}

	// GetEmails clears the list
	if mock.GetEmailCount() != 0 {
		t.Error("GetEmails should clear the email list")
	}
}

func TestMockServiceClear(t *testing.T) {
	mock := NewMockService()
	ctx := context.Background()

	mock.SendEmail(ctx, "test@test.com", "Subject", "", "")
	if mock.GetEmailCount() != 1 {
		t.Error("Expected 1 email")
	}

	mock.Clear()
	if mock.GetEmailCount() != 0 {
		t.Error("Clear should remove all emails")
	}
}

func TestMockServiceLastEmailEmpty(t *testing.T) {
	mock := NewMockService()
	if mock.LastEmail() != nil {
		t.Error("LastEmail should return nil when no emails")
	}
}

func TestPasswordResetHTML(t *testing.T) {
	html := PasswordResetHTML("https://example.com/reset?token=abc123")

	if !strings.Contains(html, "https://example.com/reset?token=abc123") {
		t.Error("HTML should contain the reset URL")
	}
	if !strings.Contains(html, "Reset Your Password") {
		t.Error("HTML should contain the title")
	}
	if !strings.Contains(html, "1 hour") {
		t.Error("HTML should mention expiration time")
	}
	if !strings.Contains(html, "Subtitler") {
		t.Error("HTML should contain brand name")
	}
}

func TestPasswordResetText(t *testing.T) {
	text := PasswordResetText("https://example.com/reset?token=abc123")

	if !strings.Contains(text, "https://example.com/reset?token=abc123") {
		t.Error("Text should contain the reset URL")
	}
	if !strings.Contains(text, "Reset Your Password") {
		t.Error("Text should contain the title")
	}
	if !strings.Contains(text, "1 hour") {
		t.Error("Text should mention expiration time")
	}
}

func TestHashToken(t *testing.T) {
	token := "test-token-12345"
	hash1 := HashToken(token)
	hash2 := HashToken(token)

	// Same input should produce same hash
	if hash1 != hash2 {
		t.Error("HashToken should be deterministic")
	}

	// Hash should be hex string (64 chars for SHA-256)
	if len(hash1) != 64 {
		t.Errorf("Expected 64 char hash, got %d", len(hash1))
	}

	// Different inputs should produce different hashes
	differentHash := HashToken("different-token")
	if hash1 == differentHash {
		t.Error("Different tokens should produce different hashes")
	}
}

func TestResendServiceDisabled(t *testing.T) {
	// Without RESEND_API_KEY, service should be disabled
	service := NewResendService()
	if service.IsEnabled() {
		t.Error("Service should be disabled without API key")
	}

	// SendEmail should succeed silently when disabled
	ctx := context.Background()
	err := service.SendEmail(ctx, "test@test.com", "Subject", "", "")
	if err != nil {
		t.Errorf("SendEmail should not error when disabled: %v", err)
	}
}

func TestResendServiceAppURL(t *testing.T) {
	service := NewResendService()
	// Default should be localhost
	if service.GetAppURL() != "http://localhost:4321" {
		t.Errorf("Expected default app URL, got %s", service.GetAppURL())
	}
}
