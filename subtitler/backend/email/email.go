// Package email provides email sending capabilities for the Subtitler application.
package email

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"

	"github.com/resend/resend-go/v2"
)

// EmailService defines the interface for sending emails
type EmailService interface {
	// SendPasswordReset sends a password reset email
	SendPasswordReset(ctx context.Context, to, token string) error

	// SendEmail sends a generic email
	SendEmail(ctx context.Context, to, subject, htmlBody, textBody string) error

	// IsEnabled returns whether email sending is enabled
	IsEnabled() bool
}

// ResendService implements EmailService using the Resend API
type ResendService struct {
	client  *resend.Client
	from    string
	appURL  string
	enabled bool
}

// NewResendService creates a new Resend email service
// It reads configuration from environment variables:
// - RESEND_API_KEY: Required for sending emails
// - EMAIL_FROM: Sender email address (default: noreply@subtitler.app)
// - APP_URL: Base URL for email links (default: http://localhost:4321)
// - EMAIL_ENABLED: Set to "false" to disable email sending
func NewResendService() *ResendService {
	apiKey := os.Getenv("RESEND_API_KEY")
	from := os.Getenv("EMAIL_FROM")
	if from == "" {
		from = "noreply@subtitler.app"
	}
	appURL := os.Getenv("APP_URL")
	if appURL == "" {
		appURL = "http://localhost:4321"
	}
	enabled := os.Getenv("EMAIL_ENABLED") != "false"

	var client *resend.Client
	if apiKey != "" {
		client = resend.NewClient(apiKey)
	}

	return &ResendService{
		client:  client,
		from:    from,
		appURL:  appURL,
		enabled: enabled && client != nil,
	}
}

// IsEnabled returns true if the email service is configured and enabled
func (s *ResendService) IsEnabled() bool {
	return s.enabled
}

// GetAppURL returns the configured application URL
func (s *ResendService) GetAppURL() string {
	return s.appURL
}

// SendEmail sends an email with the given parameters
func (s *ResendService) SendEmail(ctx context.Context, to, subject, htmlBody, textBody string) error {
	if !s.enabled {
		// Log what would have been sent in dev mode
		log.Printf("[EMAIL] Would send to=%s subject=%s", to, subject)
		return nil
	}

	params := &resend.SendEmailRequest{
		From:    s.from,
		To:      []string{to},
		Subject: subject,
		Html:    htmlBody,
		Text:    textBody,
	}

	_, err := s.client.Emails.Send(params)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// SendPasswordReset sends a password reset email with a link containing the token
func (s *ResendService) SendPasswordReset(ctx context.Context, to, token string) error {
	subject := "Reset your Subtitler password"
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", s.appURL, token)

	htmlBody := PasswordResetHTML(resetURL)
	textBody := PasswordResetText(resetURL)

	return s.SendEmail(ctx, to, subject, htmlBody, textBody)
}

// HashToken creates a SHA-256 hash of a token for storage
// This is used for password reset tokens (not passwords - those use bcrypt)
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
