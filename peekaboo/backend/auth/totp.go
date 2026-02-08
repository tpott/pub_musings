package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //#nosec G505 -- SHA1 is required by the TOTP standard (RFC 6238)
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"math"
	"net/url"
	"strings"
	"time"
)

const (
	// TOTPPeriod is the time step in seconds (standard is 30s).
	TOTPPeriod = 30

	// TOTPDigits is the number of digits in the TOTP code.
	TOTPDigits = 6

	// TOTPSecretLength is the number of random bytes for the TOTP secret.
	TOTPSecretLength = 20

	// TOTPSkew is the number of periods to check before/after current time
	// to account for clock drift. 1 means we check t-1, t, t+1.
	TOTPSkew = 1

	// TOTPIssuer is the issuer name shown in authenticator apps.
	TOTPIssuer = "Peekaboo"
)

// GenerateTOTPSecret generates a random TOTP secret and returns it as a
// base32-encoded string (without padding), suitable for storage and for
// use with authenticator apps.
func GenerateTOTPSecret() (string, error) {
	b := make([]byte, TOTPSecretLength)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate TOTP secret: %w", err)
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), nil
}

// ValidateTOTP checks if the provided TOTP code is valid for the given
// base32-encoded secret at the current time. It allows for clock skew
// of ±TOTPSkew periods.
func ValidateTOTP(secret, code string) (bool, error) {
	return ValidateTOTPAt(secret, code, time.Now())
}

// ValidateTOTPAt checks if the provided TOTP code is valid for the given
// base32-encoded secret at the specified time. Exported for testing.
func ValidateTOTPAt(secret, code string, t time.Time) (bool, error) {
	if len(code) != TOTPDigits {
		return false, nil
	}

	// Decode the base32 secret
	secretBytes, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(
		strings.ToUpper(strings.TrimRight(secret, "=")),
	)
	if err != nil {
		return false, fmt.Errorf("decode TOTP secret: %w", err)
	}

	counter := t.Unix() / TOTPPeriod

	// Check current period and skew window
	for i := -TOTPSkew; i <= TOTPSkew; i++ {
		expected := generateTOTPCode(secretBytes, counter+int64(i))
		if hmac.Equal([]byte(expected), []byte(code)) {
			return true, nil
		}
	}

	return false, nil
}

// generateTOTPCode generates a TOTP code for the given secret and counter
// per RFC 6238 / RFC 4226.
func generateTOTPCode(secret []byte, counter int64) string {
	// Encode counter as big-endian 8-byte value
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(counter))

	// HMAC-SHA1
	mac := hmac.New(sha1.New, secret)
	mac.Write(buf)
	h := mac.Sum(nil)

	// Dynamic truncation (RFC 4226 Section 5.4)
	offset := h[len(h)-1] & 0x0f
	truncated := binary.BigEndian.Uint32(h[offset:offset+4]) & 0x7fffffff

	// Modulo to get the desired number of digits
	otp := truncated % uint32(math.Pow10(TOTPDigits))

	return fmt.Sprintf("%0*d", TOTPDigits, otp)
}

// GenerateCodeAt generates the TOTP code for the given base32 secret at the
// specified time. This is exported for use in tests that need to produce valid codes.
func GenerateCodeAt(secret string, t time.Time) (string, error) {
	secretBytes, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(
		strings.ToUpper(strings.TrimRight(secret, "=")),
	)
	if err != nil {
		return "", fmt.Errorf("decode TOTP secret: %w", err)
	}
	counter := t.Unix() / TOTPPeriod
	return generateTOTPCode(secretBytes, counter), nil
}

// TOTPKeyURI generates an otpauth:// URI for use with authenticator apps.
// The URI follows the format: otpauth://totp/Issuer:account?secret=...&issuer=...
func TOTPKeyURI(account, secret string) string {
	v := url.Values{}
	v.Set("secret", secret)
	v.Set("issuer", TOTPIssuer)
	v.Set("algorithm", "SHA1")
	v.Set("digits", fmt.Sprintf("%d", TOTPDigits))
	v.Set("period", fmt.Sprintf("%d", TOTPPeriod))

	label := url.PathEscape(TOTPIssuer) + ":" + url.PathEscape(account)
	return fmt.Sprintf("otpauth://totp/%s?%s", label, v.Encode())
}
