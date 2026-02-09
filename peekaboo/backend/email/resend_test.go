package email

import (
	"strings"
	"testing"
)

// --- MockEmailSender tests ---

func TestMockEmailSender_New(t *testing.T) {
	mock := NewMockEmailSender()
	if mock.Count() != 0 {
		t.Errorf("new mock should have 0 emails, got %d", mock.Count())
	}
	if mock.Last() != nil {
		t.Error("Last() should return nil when no emails sent")
	}
}

func TestMockEmailSender_SendVerification(t *testing.T) {
	mock := NewMockEmailSender()

	err := mock.SendVerificationEmail("alice@example.com", "tok123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mock.Count() != 1 {
		t.Fatalf("expected 1 email, got %d", mock.Count())
	}
	last := mock.Last()
	if last.To != "alice@example.com" {
		t.Errorf("To = %q, want alice@example.com", last.To)
	}
	if last.Subject != "Verify your Peekaboo email" {
		t.Errorf("Subject = %q, want Verify your Peekaboo email", last.Subject)
	}
	if !strings.Contains(last.HTML, "tok123") {
		t.Error("HTML should contain the token in the URL")
	}
	if !strings.Contains(last.Text, "tok123") {
		t.Error("Text should contain the token in the URL")
	}
}

func TestMockEmailSender_SendMagicLink(t *testing.T) {
	mock := NewMockEmailSender()

	err := mock.SendMagicLinkEmail("bob@example.com", "ml-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mock.Count() != 1 {
		t.Fatalf("expected 1 email, got %d", mock.Count())
	}
	last := mock.Last()
	if last.To != "bob@example.com" {
		t.Errorf("To = %q, want bob@example.com", last.To)
	}
	if last.Subject != "Sign in to Peekaboo" {
		t.Errorf("Subject = %q, want Sign in to Peekaboo", last.Subject)
	}
	if !strings.Contains(last.HTML, "ml-token") {
		t.Error("HTML should contain the token in the URL")
	}
}

func TestMockEmailSender_Error(t *testing.T) {
	mock := NewMockEmailSender()
	mock.Err = errFake

	err := mock.SendVerificationEmail("x@x.com", "t")
	if err != errFake {
		t.Errorf("expected errFake, got %v", err)
	}
	if mock.Count() != 0 {
		t.Error("should not record email on error")
	}

	err = mock.SendMagicLinkEmail("x@x.com", "t")
	if err != errFake {
		t.Errorf("expected errFake, got %v", err)
	}
}

func TestMockEmailSender_Clear(t *testing.T) {
	mock := NewMockEmailSender()
	if err := mock.SendVerificationEmail("a@b.com", "t1"); err != nil {
		t.Fatal(err)
	}
	if err := mock.SendMagicLinkEmail("c@d.com", "t2"); err != nil {
		t.Fatal(err)
	}

	if mock.Count() != 2 {
		t.Fatalf("expected 2 emails, got %d", mock.Count())
	}

	mock.Clear()
	if mock.Count() != 0 {
		t.Error("Clear should remove all emails")
	}
	if mock.Last() != nil {
		t.Error("Last should return nil after Clear")
	}
}

// --- Template tests ---

func TestVerificationHTML(t *testing.T) {
	html := VerificationHTML("https://peekaboo.example.com/verify-email?token=abc123")

	checks := []string{
		"https://peekaboo.example.com/verify-email?token=abc123",
		"Verify Your Email",
		"24 hours",
		"Peekaboo",
	}
	for _, want := range checks {
		if !strings.Contains(html, want) {
			t.Errorf("HTML missing %q", want)
		}
	}
}

func TestVerificationText(t *testing.T) {
	text := VerificationText("https://peekaboo.example.com/verify-email?token=abc123")

	checks := []string{
		"https://peekaboo.example.com/verify-email?token=abc123",
		"Verify Your Email",
		"24 hours",
	}
	for _, want := range checks {
		if !strings.Contains(text, want) {
			t.Errorf("text missing %q", want)
		}
	}
}

func TestMagicLinkHTML(t *testing.T) {
	html := MagicLinkHTML("https://peekaboo.example.com/magic-link?token=xyz789")

	checks := []string{
		"https://peekaboo.example.com/magic-link?token=xyz789",
		"Sign In to Peekaboo",
		"15 minutes",
		"Peekaboo",
	}
	for _, want := range checks {
		if !strings.Contains(html, want) {
			t.Errorf("HTML missing %q", want)
		}
	}
}

func TestMagicLinkText(t *testing.T) {
	text := MagicLinkText("https://peekaboo.example.com/magic-link?token=xyz789")

	checks := []string{
		"https://peekaboo.example.com/magic-link?token=xyz789",
		"Sign In to Peekaboo",
		"15 minutes",
	}
	for _, want := range checks {
		if !strings.Contains(text, want) {
			t.Errorf("text missing %q", want)
		}
	}
}

// --- ResendEmailSender construction test ---

func TestNewResendEmailSender(t *testing.T) {
	sender := NewResendEmailSender("re_test_key", "noreply@example.com", "https://app.example.com")
	if sender.from != "noreply@example.com" {
		t.Errorf("from = %q, want noreply@example.com", sender.from)
	}
	if sender.appURL != "https://app.example.com" {
		t.Errorf("appURL = %q, want https://app.example.com", sender.appURL)
	}
	if sender.client == nil {
		t.Error("client should not be nil")
	}
}

// sentinel error for tests
var errFake = &fakeErr{}

type fakeErr struct{}

func (e *fakeErr) Error() string { return "fake error" }
