package email

import (
	"fmt"
	"sync"

	"github.com/tpott/pub_musings/peekaboo/backend/api"
)

// SentEmail records the details of a sent email for test assertions.
type SentEmail struct {
	To      string
	Subject string
	HTML    string
	Text    string
}

// MockEmailSender records emails for test assertions instead of sending them.
type MockEmailSender struct {
	mu     sync.Mutex
	Emails []SentEmail
	// Err is returned by Send methods when non-nil, for simulating failures.
	Err error
}

// NewMockEmailSender creates a MockEmailSender with no recorded emails.
func NewMockEmailSender() *MockEmailSender {
	return &MockEmailSender{
		Emails: make([]SentEmail, 0),
	}
}

// SendVerificationEmail records a verification email.
func (m *MockEmailSender) SendVerificationEmail(to, token string) error {
	if m.Err != nil {
		return m.Err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	verifyURL := fmt.Sprintf("http://localhost/verify-email?token=%s", token)
	m.Emails = append(m.Emails, SentEmail{
		To:      to,
		Subject: "Verify your Peekaboo email",
		HTML:    VerificationHTML(verifyURL),
		Text:    VerificationText(verifyURL),
	})
	return nil
}

// SendMagicLinkEmail records a magic link email.
func (m *MockEmailSender) SendMagicLinkEmail(to, token string) error {
	if m.Err != nil {
		return m.Err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	loginURL := fmt.Sprintf("http://localhost/magic-link?token=%s", token)
	m.Emails = append(m.Emails, SentEmail{
		To:      to,
		Subject: "Sign in to Peekaboo",
		HTML:    MagicLinkHTML(loginURL),
		Text:    MagicLinkText(loginURL),
	})
	return nil
}

// Count returns the number of recorded emails.
func (m *MockEmailSender) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.Emails)
}

// Last returns the most recently recorded email, or nil if none.
func (m *MockEmailSender) Last() *SentEmail {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.Emails) == 0 {
		return nil
	}
	return &m.Emails[len(m.Emails)-1]
}

// Clear removes all recorded emails.
func (m *MockEmailSender) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Emails = make([]SentEmail, 0)
}

// compile-time check that MockEmailSender implements api.EmailSender
var _ api.EmailSender = (*MockEmailSender)(nil)
