// Package auth provides authentication primitives for the Peekaboo application.
// It handles password hashing, token generation, session management, and CSRF protection.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	// BcryptCost is the bcrypt work factor.
	BcryptCost = 12

	// SessionDuration is how long a session remains valid.
	SessionDuration = 30 * 24 * time.Hour // 30 days

	// MinPasswordLength is the minimum allowed password length.
	MinPasswordLength = 8

	// MaxPasswordLength is the maximum allowed password length (bcrypt limit).
	MaxPasswordLength = 72

	// MaxEmailLength is the maximum allowed email length.
	MaxEmailLength = 255

	// EmailVerificationTokenExpiry is how long an email verification token is valid.
	EmailVerificationTokenExpiry = 24 * time.Hour

	// MagicLinkTokenExpiry is how long a magic link token is valid.
	MagicLinkTokenExpiry = 15 * time.Minute

	// LockoutThreshold is the number of failed attempts before lockout.
	LockoutThreshold = 5

	// LockoutWindow is the time window for counting failed attempts.
	LockoutWindow = 15 * time.Minute

	// LockoutDuration is how long an account is locked after too many failures.
	LockoutDuration = 15 * time.Minute
)

// HashPassword hashes a password using bcrypt with the configured cost.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// VerifyPassword checks a password against a bcrypt hash.
// Returns nil on success, bcrypt.ErrMismatchedHashAndPassword on mismatch.
func VerifyPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// GenerateToken generates a cryptographically random token of the given byte length
// and returns it as a hex-encoded string.
func GenerateToken(byteLength int) (string, error) {
	b := make([]byte, byteLength)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// GenerateID generates a 32-character hex ID (16 random bytes).
func GenerateID() (string, error) {
	return GenerateToken(16)
}

// HashToken computes the SHA-256 hash of a token and returns it as a hex string.
// Used for storing token hashes instead of plaintext tokens.
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

// GenerateCSRF generates a CSRF token from a session token using HMAC-SHA256.
func GenerateCSRF(sessionToken string, secret []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(sessionToken))
	return hex.EncodeToString(mac.Sum(nil))
}

// ValidateCSRF checks if a CSRF token is valid for the given session token.
func ValidateCSRF(csrfToken, sessionToken string, secret []byte) bool {
	expected := GenerateCSRF(sessionToken, secret)
	return hmac.Equal([]byte(csrfToken), []byte(expected))
}
