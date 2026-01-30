// Package captcha provides CAPTCHA validation for bot protection.
// Supports hCaptcha with fallback to disabled mode for development.
package captcha

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// hCaptcha verification endpoint
	hCaptchaVerifyURL = "https://hcaptcha.com/siteverify"

	// Request timeout for verification
	verifyTimeout = 10 * time.Second
)

var (
	// ErrMissingToken is returned when no CAPTCHA token is provided
	ErrMissingToken = errors.New("CAPTCHA token required")

	// ErrInvalidToken is returned when CAPTCHA verification fails
	ErrInvalidToken = errors.New("CAPTCHA verification failed")

	// ErrServiceUnavailable is returned when the CAPTCHA service is unreachable
	ErrServiceUnavailable = errors.New("CAPTCHA service unavailable")
)

// Verifier validates CAPTCHA tokens
type Verifier interface {
	Verify(ctx context.Context, token string, remoteIP string) error
	IsEnabled() bool
	SiteKey() string
}

// Config holds CAPTCHA configuration
type Config struct {
	// SiteKey is the public hCaptcha site key (for frontend)
	SiteKey string
	// SecretKey is the private hCaptcha secret key (for verification)
	SecretKey string
}

// hCaptchaVerifier implements CAPTCHA verification using hCaptcha
type hCaptchaVerifier struct {
	siteKey   string
	secretKey string
	client    *http.Client
}

// hCaptchaResponse is the response from hCaptcha verification API
type hCaptchaResponse struct {
	Success     bool     `json:"success"`
	ChallengeTs string   `json:"challenge_ts,omitempty"`
	Hostname    string   `json:"hostname,omitempty"`
	ErrorCodes  []string `json:"error-codes,omitempty"`
}

// New creates a new CAPTCHA verifier.
// If secretKey is empty, returns a disabled verifier that always succeeds.
func New(cfg Config) Verifier {
	if cfg.SecretKey == "" {
		return &disabledVerifier{}
	}
	return &hCaptchaVerifier{
		siteKey:   cfg.SiteKey,
		secretKey: cfg.SecretKey,
		client: &http.Client{
			Timeout: verifyTimeout,
		},
	}
}

// IsEnabled returns true if CAPTCHA verification is active
func (v *hCaptchaVerifier) IsEnabled() bool {
	return true
}

// SiteKey returns the public site key for frontend use
func (v *hCaptchaVerifier) SiteKey() string {
	return v.siteKey
}

// Verify validates the CAPTCHA token with hCaptcha
func (v *hCaptchaVerifier) Verify(ctx context.Context, token string, remoteIP string) error {
	if token == "" {
		return ErrMissingToken
	}

	// Prepare form data
	form := url.Values{}
	form.Set("response", token)
	form.Set("secret", v.secretKey)
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", hCaptchaVerifyURL, strings.NewReader(form.Encode()))
	if err != nil {
		return ErrServiceUnavailable
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Send request
	resp, err := v.client.Do(req)
	if err != nil {
		return ErrServiceUnavailable
	}
	defer resp.Body.Close()

	// Parse response
	var result hCaptchaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ErrServiceUnavailable
	}

	if !result.Success {
		return ErrInvalidToken
	}

	return nil
}

// disabledVerifier is used when CAPTCHA is not configured
type disabledVerifier struct{}

// IsEnabled returns false for disabled verifier
func (v *disabledVerifier) IsEnabled() bool {
	return false
}

// SiteKey returns an empty string for disabled verifier
func (v *disabledVerifier) SiteKey() string {
	return ""
}

// Verify always succeeds for disabled verifier
func (v *disabledVerifier) Verify(ctx context.Context, token string, remoteIP string) error {
	return nil
}

// MockVerifier is a test verifier with configurable behavior
type MockVerifier struct {
	ShouldFail   bool
	RequireToken bool
	VerifyCalls  int
	LastToken    string
	LastRemoteIP string
	MockEnabled  bool
}

// NewMockVerifier creates a mock verifier for testing
func NewMockVerifier(enabled bool) *MockVerifier {
	return &MockVerifier{
		MockEnabled: enabled,
	}
}

// IsEnabled returns the configured enabled state
func (v *MockVerifier) IsEnabled() bool {
	return v.MockEnabled
}

// SiteKey returns a test site key
func (v *MockVerifier) SiteKey() string {
	return "test-site-key"
}

// Verify records the call and returns based on configuration
func (v *MockVerifier) Verify(ctx context.Context, token string, remoteIP string) error {
	v.VerifyCalls++
	v.LastToken = token
	v.LastRemoteIP = remoteIP

	if v.RequireToken && token == "" {
		return ErrMissingToken
	}
	if v.ShouldFail {
		return ErrInvalidToken
	}
	return nil
}
