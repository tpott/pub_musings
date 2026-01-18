package email

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client handles sending emails via Resend API
type Client struct {
	apiKey      string
	fromEmail   string
	enabled     bool
	httpClient  *http.Client
}

// NewClient creates a new email client
func NewClient(apiKey, fromEmail string, enabled bool) *Client {
	return &Client{
		apiKey:    apiKey,
		fromEmail: fromEmail,
		enabled:   enabled,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// resendEmailRequest represents the Resend API request format
type resendEmailRequest struct {
	From    string `json:"from"`
	To      []string `json:"to"`
	Subject string `json:"subject"`
	Text    string `json:"text"`
}

// resendEmailResponse represents the Resend API response
type resendEmailResponse struct {
	ID string `json:"id"`
}

// SendJobCompleted sends an email notification when a job completes successfully
func (c *Client) SendJobCompleted(toEmail string, jobID int64, filename string, outputFormat string, duration time.Duration) error {
	if !c.enabled {
		return nil // Email disabled, skip sending
	}

	subject := "Your transcription is ready"

	body := fmt.Sprintf(`Your transcription for "%s" is complete!

Job ID: %d
Format: %s
Duration: %s

Download your transcript at:
https://subtitler.example.com/dashboard

This job took %s to complete.`,
		filename,
		jobID,
		outputFormat,
		formatDuration(duration),
		formatDuration(duration))

	return c.sendEmail(toEmail, subject, body)
}

// SendJobFailed sends an email notification when a job fails
func (c *Client) SendJobFailed(toEmail string, jobID int64, filename string, errorMessage string) error {
	if !c.enabled {
		return nil // Email disabled, skip sending
	}

	subject := "Transcription failed"

	body := fmt.Sprintf(`Your transcription for "%s" could not be completed.

Job ID: %d
Error: %s

Please try uploading your file again. If this problem persists, contact support.`,
		filename,
		jobID,
		errorMessage)

	return c.sendEmail(toEmail, subject, body)
}

// sendEmail sends an email via the Resend API
func (c *Client) sendEmail(toEmail, subject, body string) error {
	reqBody := resendEmailRequest{
		From:    c.fromEmail,
		To:      []string{toEmail},
		Subject: subject,
		Text:    body,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal email request: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.resend.com/emails", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("resend API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var resendResp resendEmailResponse
	if err := json.NewDecoder(resp.Body).Decode(&resendResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	return nil
}

// formatDuration formats a duration into a human-readable string
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	}
	if d < time.Hour {
		minutes := int(d.Minutes())
		seconds := int(d.Seconds()) % 60
		if seconds == 0 {
			return fmt.Sprintf("%d minutes", minutes)
		}
		return fmt.Sprintf("%d minutes, %d seconds", minutes, seconds)
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if minutes == 0 {
		return fmt.Sprintf("%d hours", hours)
	}
	return fmt.Sprintf("%d hours, %d minutes", hours, minutes)
}
