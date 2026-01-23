package email

import (
	"context"
	"sync"
)

// SentEmail represents an email that was "sent" by the mock service
type SentEmail struct {
	To       string
	Subject  string
	HtmlBody string
	TextBody string
}

// MockService is a mock email service for testing
type MockService struct {
	mu     sync.Mutex
	Emails []SentEmail
	appURL string
}

// NewMockService creates a new mock email service
func NewMockService() *MockService {
	return &MockService{
		Emails: make([]SentEmail, 0),
		appURL: "http://localhost:4321",
	}
}

// IsEnabled always returns true for the mock service
func (m *MockService) IsEnabled() bool {
	return true
}

// GetAppURL returns the configured application URL
func (m *MockService) GetAppURL() string {
	return m.appURL
}

// SetAppURL sets the application URL for testing
func (m *MockService) SetAppURL(url string) {
	m.appURL = url
}

// SendEmail records the email for later inspection
func (m *MockService) SendEmail(ctx context.Context, to, subject, htmlBody, textBody string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Emails = append(m.Emails, SentEmail{
		To:       to,
		Subject:  subject,
		HtmlBody: htmlBody,
		TextBody: textBody,
	})
	return nil
}

// SendPasswordReset sends a mock password reset email
func (m *MockService) SendPasswordReset(ctx context.Context, to, token string) error {
	subject := "Reset your Subtitler password"
	resetURL := m.appURL + "/reset-password?token=" + token
	htmlBody := PasswordResetHTML(resetURL)
	textBody := PasswordResetText(resetURL)
	return m.SendEmail(ctx, to, subject, htmlBody, textBody)
}

// GetEmails returns all sent emails and clears the list
func (m *MockService) GetEmails() []SentEmail {
	m.mu.Lock()
	defer m.mu.Unlock()
	emails := m.Emails
	m.Emails = make([]SentEmail, 0)
	return emails
}

// GetEmailCount returns the number of emails sent
func (m *MockService) GetEmailCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.Emails)
}

// Clear clears all sent emails
func (m *MockService) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Emails = make([]SentEmail, 0)
}

// LastEmail returns the most recently sent email, or nil if none
func (m *MockService) LastEmail() *SentEmail {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.Emails) == 0 {
		return nil
	}
	return &m.Emails[len(m.Emails)-1]
}
