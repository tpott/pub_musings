package auth

import (
	"encoding/base32"
	"strings"
	"testing"
	"time"
)

func TestGenerateTOTPSecret(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret() error: %v", err)
	}

	if secret == "" {
		t.Fatal("GenerateTOTPSecret() returned empty string")
	}

	// Should be valid base32 (no padding)
	decoded, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		t.Fatalf("Secret is not valid base32: %v", err)
	}

	if len(decoded) != TOTPSecretLength {
		t.Errorf("Decoded secret length = %d, want %d", len(decoded), TOTPSecretLength)
	}

	// Two secrets should be different
	secret2, _ := GenerateTOTPSecret()
	if secret == secret2 {
		t.Error("Two generated secrets should not be equal")
	}
}

func TestValidateTOTPAt_ValidCode(t *testing.T) {
	// Use a known secret and time to generate an expected code
	secret := "JBSWY3DPEHPK3PXP"    // well-known test secret ("Hello!\xDE\xAD\xBE\xEF")
	now := time.Unix(1700000000, 0) // fixed time

	// Generate the expected code at this time
	secretBytes, _ := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	counter := now.Unix() / TOTPPeriod
	expected := generateTOTPCode(secretBytes, counter)

	valid, err := ValidateTOTPAt(secret, expected, now)
	if err != nil {
		t.Fatalf("ValidateTOTPAt() error: %v", err)
	}
	if !valid {
		t.Errorf("ValidateTOTPAt() = false for correct code %q", expected)
	}
}

func TestValidateTOTPAt_ClockSkew(t *testing.T) {
	secret := "JBSWY3DPEHPK3PXP"
	now := time.Unix(1700000000, 0)

	secretBytes, _ := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)

	// Code from previous period should be accepted
	prevCounter := now.Unix()/TOTPPeriod - 1
	prevCode := generateTOTPCode(secretBytes, prevCounter)

	valid, err := ValidateTOTPAt(secret, prevCode, now)
	if err != nil {
		t.Fatalf("ValidateTOTPAt() error: %v", err)
	}
	if !valid {
		t.Error("Code from previous period should be valid (clock skew)")
	}

	// Code from next period should be accepted
	nextCounter := now.Unix()/TOTPPeriod + 1
	nextCode := generateTOTPCode(secretBytes, nextCounter)

	valid, err = ValidateTOTPAt(secret, nextCode, now)
	if err != nil {
		t.Fatalf("ValidateTOTPAt() error: %v", err)
	}
	if !valid {
		t.Error("Code from next period should be valid (clock skew)")
	}
}

func TestValidateTOTPAt_InvalidCode(t *testing.T) {
	secret := "JBSWY3DPEHPK3PXP"
	now := time.Unix(1700000000, 0)

	valid, err := ValidateTOTPAt(secret, "000000", now)
	if err != nil {
		t.Fatalf("ValidateTOTPAt() error: %v", err)
	}
	// The chance this is actually correct is 3/1000000 - negligible
	if valid {
		t.Error("ValidateTOTPAt() should reject wrong code")
	}
}

func TestValidateTOTPAt_WrongLength(t *testing.T) {
	secret := "JBSWY3DPEHPK3PXP"
	now := time.Unix(1700000000, 0)

	tests := []struct {
		name string
		code string
	}{
		{"too short", "12345"},
		{"too long", "1234567"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid, err := ValidateTOTPAt(secret, tt.code, now)
			if err != nil {
				t.Fatalf("ValidateTOTPAt() error: %v", err)
			}
			if valid {
				t.Error("ValidateTOTPAt() should reject wrong-length code")
			}
		})
	}
}

func TestValidateTOTPAt_InvalidSecret(t *testing.T) {
	now := time.Unix(1700000000, 0)

	_, err := ValidateTOTPAt("not-valid-base32!!!", "123456", now)
	if err == nil {
		t.Error("ValidateTOTPAt() should return error for invalid base32 secret")
	}
}

func TestValidateTOTPAt_OutsideSkewWindow(t *testing.T) {
	secret := "JBSWY3DPEHPK3PXP"
	now := time.Unix(1700000000, 0)

	secretBytes, _ := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)

	// Code from 2 periods ago (outside ±1 skew)
	oldCounter := now.Unix()/TOTPPeriod - 2
	oldCode := generateTOTPCode(secretBytes, oldCounter)

	valid, err := ValidateTOTPAt(secret, oldCode, now)
	if err != nil {
		t.Fatalf("ValidateTOTPAt() error: %v", err)
	}
	if valid {
		t.Error("Code from 2 periods ago should be rejected")
	}
}

func TestTOTPKeyURI(t *testing.T) {
	uri := TOTPKeyURI("user@example.com", "JBSWY3DPEHPK3PXP")

	if !strings.HasPrefix(uri, "otpauth://totp/") {
		t.Errorf("URI should start with otpauth://totp/, got %q", uri)
	}

	if !strings.Contains(uri, "secret=JBSWY3DPEHPK3PXP") {
		t.Errorf("URI should contain secret, got %q", uri)
	}

	if !strings.Contains(uri, "issuer=Peekaboo") {
		t.Errorf("URI should contain issuer, got %q", uri)
	}

	if !strings.Contains(uri, "Peekaboo") {
		t.Errorf("URI should contain issuer label, got %q", uri)
	}

	if !strings.Contains(uri, "user@example.com") && !strings.Contains(uri, "user%40example.com") {
		t.Errorf("URI should contain email, got %q", uri)
	}
}

func TestGenerateTOTPCode_KnownVectors(t *testing.T) {
	// RFC 6238 test vector: SHA1, secret = "12345678901234567890"
	// These are the standard test vectors from RFC 6238 Appendix B
	secret := []byte("12345678901234567890")

	tests := []struct {
		time     int64
		expected string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			counter := tt.time / TOTPPeriod
			got := generateTOTPCode(secret, counter)
			if got != tt.expected {
				t.Errorf("generateTOTPCode(time=%d) = %q, want %q", tt.time, got, tt.expected)
			}
		})
	}
}
