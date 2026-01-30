package totp

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"

	goqr "github.com/piglig/go-qr"
)

const (
	// SecretLength is the length of the TOTP secret in bytes
	// 20 bytes = 160 bits, which is the recommended minimum for SHA1
	SecretLength = 20

	// Digits is the number of digits in a TOTP code
	Digits = 6

	// Period is the time step in seconds (standard is 30 seconds)
	Period = 30

	// Window is the number of periods to check before/after current time
	// This allows for clock drift between server and authenticator app
	Window = 1
)

// GenerateSecret generates a new random TOTP secret
func GenerateSecret() (string, error) {
	bytes := make([]byte, SecretLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	// Encode as base32 without padding (standard for TOTP)
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(bytes)
	return secret, nil
}

// GenerateCode generates a TOTP code for the given secret at the current time
func GenerateCode(secret string) (string, error) {
	return GenerateCodeAt(secret, time.Now())
}

// GenerateCodeAt generates a TOTP code for the given secret at a specific time
func GenerateCodeAt(secret string, t time.Time) (string, error) {
	// Decode the base32 secret
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		return "", fmt.Errorf("invalid secret: %w", err)
	}

	// Calculate the counter value (number of time steps since Unix epoch)
	counter := uint64(t.Unix()) / Period

	// Generate HOTP code using the counter
	code := generateHOTP(key, counter, Digits)

	return fmt.Sprintf("%0*d", Digits, code), nil
}

// Validate validates a TOTP code against a secret
// It checks the current time period and allows for a configurable window
func Validate(secret, code string) bool {
	return ValidateAt(secret, code, time.Now())
}

// ValidateAt validates a TOTP code against a secret at a specific time
func ValidateAt(secret, code string, t time.Time) bool {
	// Check the current time period and window periods before/after
	for i := -Window; i <= Window; i++ {
		checkTime := t.Add(time.Duration(i*Period) * time.Second)
		expectedCode, err := GenerateCodeAt(secret, checkTime)
		if err != nil {
			continue
		}
		if hmac.Equal([]byte(expectedCode), []byte(code)) {
			return true
		}
	}
	return false
}

// generateHOTP generates an HOTP code using HMAC-SHA1
func generateHOTP(key []byte, counter uint64, digits int) int {
	// Convert counter to 8-byte big-endian
	counterBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(counterBytes, counter)

	// Calculate HMAC-SHA1
	h := hmac.New(sha1.New, key)
	h.Write(counterBytes)
	hash := h.Sum(nil)

	// Dynamic truncation (RFC 4226)
	offset := hash[len(hash)-1] & 0x0f
	code := (int(hash[offset]&0x7f) << 24) |
		(int(hash[offset+1]) << 16) |
		(int(hash[offset+2]) << 8) |
		int(hash[offset+3])

	// Take the specified number of digits
	mod := 1
	for i := 0; i < digits; i++ {
		mod *= 10
	}
	return code % mod
}

// GenerateProvisioningURI generates an otpauth:// URI for use with QR codes
// This URI can be scanned by authenticator apps like Google Authenticator
func GenerateProvisioningURI(secret, email, issuer string) string {
	// Format: otpauth://totp/ISSUER:ACCOUNT?secret=SECRET&issuer=ISSUER&algorithm=SHA1&digits=6&period=30
	accountName := url.PathEscape(fmt.Sprintf("%s:%s", issuer, email))
	params := url.Values{}
	params.Set("secret", secret)
	params.Set("issuer", issuer)
	params.Set("algorithm", "SHA1")
	params.Set("digits", fmt.Sprintf("%d", Digits))
	params.Set("period", fmt.Sprintf("%d", Period))

	return fmt.Sprintf("otpauth://totp/%s?%s", accountName, params.Encode())
}

// FormatSecretForDisplay formats a secret with spaces for easier reading
// e.g., "JBSWY3DPEHPK3PXP" becomes "JBSW Y3DP EHPK 3PXP"
func FormatSecretForDisplay(secret string) string {
	var parts []string
	for i := 0; i < len(secret); i += 4 {
		end := i + 4
		if end > len(secret) {
			end = len(secret)
		}
		parts = append(parts, secret[i:end])
	}
	return strings.Join(parts, " ")
}

// GenerateQRCode generates a QR code PNG as a base64 data URL
// The returned string can be used directly as an img src attribute
func GenerateQRCode(uri string) (string, error) {
	// Generate QR code at medium recovery level
	qr, err := goqr.EncodeText(uri, goqr.Medium)
	if err != nil {
		return "", fmt.Errorf("failed to encode QR code: %w", err)
	}

	// Configure QR code image: cell size 8, margin 4 (results in ~200x200 for typical TOTP URIs)
	config := goqr.NewQrCodeImgConfig(8, 4)

	// Write PNG to buffer
	var buf bytes.Buffer
	if err := qr.WriteAsPNG(config, &buf); err != nil {
		return "", fmt.Errorf("failed to generate QR code PNG: %w", err)
	}

	// Encode as base64 data URL
	b64 := base64.StdEncoding.EncodeToString(buf.Bytes())
	return "data:image/png;base64," + b64, nil
}
