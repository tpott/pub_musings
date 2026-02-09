// Package email provides email sending via the Resend API.
package email

import (
	"fmt"
	"log/slog"

	"github.com/resend/resend-go/v3"
	"github.com/tpott/pub_musings/peekaboo/backend/api"
)

// ResendEmailSender implements api.EmailSender using the Resend API.
type ResendEmailSender struct {
	client *resend.Client
	from   string
	appURL string
}

// NewResendEmailSender creates a ResendEmailSender with the given config.
// apiKey is the Resend API key.
// from is the sender address (e.g. "noreply@peekaboo.pottingers.us").
// appURL is the base URL for email links (e.g. "https://peekaboo.pottingers.us").
func NewResendEmailSender(apiKey, from, appURL string) *ResendEmailSender {
	return &ResendEmailSender{
		client: resend.NewClient(apiKey),
		from:   from,
		appURL: appURL,
	}
}

// SendVerificationEmail sends a verification email with a link to APP_URL/verify-email?token=X.
func (s *ResendEmailSender) SendVerificationEmail(to, token string) error {
	verifyURL := fmt.Sprintf("%s/verify-email?token=%s", s.appURL, token)

	params := &resend.SendEmailRequest{
		From:    s.from,
		To:      []string{to},
		Subject: "Verify your Peekaboo email",
		Html:    VerificationHTML(verifyURL),
		Text:    VerificationText(verifyURL),
	}

	_, err := s.client.Emails.Send(params)
	if err != nil {
		return fmt.Errorf("resend: send verification email: %w", err)
	}

	slog.Info("verification email sent", "to", to)
	return nil
}

// SendMagicLinkEmail sends a magic link email with a link to APP_URL/magic-link?token=X.
func (s *ResendEmailSender) SendMagicLinkEmail(to, token string) error {
	loginURL := fmt.Sprintf("%s/magic-link?token=%s", s.appURL, token)

	params := &resend.SendEmailRequest{
		From:    s.from,
		To:      []string{to},
		Subject: "Sign in to Peekaboo",
		Html:    MagicLinkHTML(loginURL),
		Text:    MagicLinkText(loginURL),
	}

	_, err := s.client.Emails.Send(params)
	if err != nil {
		return fmt.Errorf("resend: send magic link email: %w", err)
	}

	slog.Info("magic link email sent", "to", to)
	return nil
}

// compile-time check that ResendEmailSender implements api.EmailSender
var _ api.EmailSender = (*ResendEmailSender)(nil)
