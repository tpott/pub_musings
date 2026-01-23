package totp

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

func TestGenerateSecret(t *testing.T) {
	secret1, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret() error = %v", err)
	}
	if secret1 == "" {
		t.Error("GenerateSecret() returned empty string")
	}

	// Check it's valid base32
	if len(secret1) < 20 {
		t.Errorf("GenerateSecret() returned too short secret: %d chars", len(secret1))
	}

	// Generate another secret and ensure they're different
	secret2, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret() error = %v", err)
	}
	if secret1 == secret2 {
		t.Error("GenerateSecret() returned same secret twice")
	}
}

func TestGenerateCode(t *testing.T) {
	// Test with a known secret
	secret := "JBSWY3DPEHPK3PXP"

	code, err := GenerateCode(secret)
	if err != nil {
		t.Fatalf("GenerateCode() error = %v", err)
	}

	// Code should be 6 digits
	if len(code) != 6 {
		t.Errorf("GenerateCode() returned code with %d digits, want 6", len(code))
	}

	// All characters should be digits
	for _, c := range code {
		if c < '0' || c > '9' {
			t.Errorf("GenerateCode() returned non-digit character: %c", c)
		}
	}
}

func TestGenerateCodeAt(t *testing.T) {
	// RFC 6238 test vectors
	// Note: These use SHA1, 30 second period, 8 digits in the RFC
	// We use 6 digits so we test our own implementation

	secret := "JBSWY3DPEHPK3PXP" // "Hello!" in base32

	// Test that same time produces same code
	fixedTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	code1, err := GenerateCodeAt(secret, fixedTime)
	if err != nil {
		t.Fatalf("GenerateCodeAt() error = %v", err)
	}
	code2, err := GenerateCodeAt(secret, fixedTime)
	if err != nil {
		t.Fatalf("GenerateCodeAt() error = %v", err)
	}
	if code1 != code2 {
		t.Errorf("GenerateCodeAt() returned different codes for same time: %s != %s", code1, code2)
	}

	// Test that different times produce different codes
	laterTime := fixedTime.Add(60 * time.Second) // 2 periods later
	code3, err := GenerateCodeAt(secret, laterTime)
	if err != nil {
		t.Fatalf("GenerateCodeAt() error = %v", err)
	}
	if code1 == code3 {
		t.Error("GenerateCodeAt() returned same code for different times")
	}
}

func TestValidate(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret() error = %v", err)
	}

	// Generate code at current time
	code, err := GenerateCode(secret)
	if err != nil {
		t.Fatalf("GenerateCode() error = %v", err)
	}

	// Should validate successfully
	if !Validate(secret, code) {
		t.Error("Validate() returned false for valid code")
	}

	// Invalid code should fail
	if Validate(secret, "000000") {
		// Small chance this could be the actual code, but very unlikely
		if code != "000000" {
			t.Error("Validate() returned true for invalid code")
		}
	}
}

func TestValidateAt(t *testing.T) {
	secret := "JBSWY3DPEHPK3PXP"
	fixedTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)

	code, err := GenerateCodeAt(secret, fixedTime)
	if err != nil {
		t.Fatalf("GenerateCodeAt() error = %v", err)
	}

	// Should validate at same time
	if !ValidateAt(secret, code, fixedTime) {
		t.Error("ValidateAt() returned false for valid code at same time")
	}

	// Should validate within window (30 seconds)
	withinWindow := fixedTime.Add(25 * time.Second)
	if !ValidateAt(secret, code, withinWindow) {
		t.Error("ValidateAt() returned false for code within window")
	}

	// Should fail outside window (more than 1 period away)
	outsideWindow := fixedTime.Add(120 * time.Second)
	if ValidateAt(secret, code, outsideWindow) {
		t.Error("ValidateAt() returned true for code outside window")
	}
}

func TestValidateInvalidSecret(t *testing.T) {
	// Invalid base32 should not panic, just return false
	if Validate("invalid!", "123456") {
		t.Error("Validate() should return false for invalid secret")
	}
}

func TestGenerateProvisioningURI(t *testing.T) {
	secret := "JBSWY3DPEHPK3PXP"
	email := "user@example.com"
	issuer := "Subtitler"

	uri := GenerateProvisioningURI(secret, email, issuer)

	// Should start with otpauth://totp/
	if !strings.HasPrefix(uri, "otpauth://totp/") {
		t.Errorf("GenerateProvisioningURI() URI doesn't start with otpauth://totp/: %s", uri)
	}

	// Should contain the secret
	if !strings.Contains(uri, "secret="+secret) {
		t.Errorf("GenerateProvisioningURI() URI doesn't contain secret: %s", uri)
	}

	// Should contain the issuer
	if !strings.Contains(uri, "issuer="+issuer) {
		t.Errorf("GenerateProvisioningURI() URI doesn't contain issuer: %s", uri)
	}

	// Should contain the email/account
	if !strings.Contains(uri, email) {
		t.Errorf("GenerateProvisioningURI() URI doesn't contain email: %s", uri)
	}
}

func TestFormatSecretForDisplay(t *testing.T) {
	tests := []struct {
		secret   string
		expected string
	}{
		{"JBSWY3DPEHPK3PXP", "JBSW Y3DP EHPK 3PXP"},
		{"ABCD", "ABCD"},
		{"ABCDEFGH", "ABCD EFGH"},
		{"ABCDEFGHI", "ABCD EFGH I"},
		{"", ""},
	}

	for _, tt := range tests {
		result := FormatSecretForDisplay(tt.secret)
		if result != tt.expected {
			t.Errorf("FormatSecretForDisplay(%q) = %q, want %q", tt.secret, result, tt.expected)
		}
	}
}

func TestCodeChangesEvery30Seconds(t *testing.T) {
	secret := "JBSWY3DPEHPK3PXP"

	// Get code at the start of a period
	period1Start := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	code1, _ := GenerateCodeAt(secret, period1Start)

	// Get code at the end of the same period (29 seconds later)
	period1End := period1Start.Add(29 * time.Second)
	code2, _ := GenerateCodeAt(secret, period1End)

	// Should be the same
	if code1 != code2 {
		t.Error("Code changed within same 30-second period")
	}

	// Get code at the start of the next period (30 seconds later)
	period2Start := period1Start.Add(30 * time.Second)
	code3, _ := GenerateCodeAt(secret, period2Start)

	// Should be different
	if code1 == code3 {
		t.Error("Code didn't change across 30-second periods")
	}
}

func TestGenerateQRCode(t *testing.T) {
	uri := "otpauth://totp/Subtitler:user@example.com?secret=JBSWY3DPEHPK3PXP&issuer=Subtitler"

	dataURL, err := GenerateQRCode(uri)
	if err != nil {
		t.Fatalf("GenerateQRCode() error = %v", err)
	}

	// Should be a data URL
	prefix := "data:image/png;base64,"
	if !strings.HasPrefix(dataURL, prefix) {
		t.Errorf("GenerateQRCode() result doesn't start with %s: got %s...", prefix, dataURL[:min(len(dataURL), 50)])
	}

	// Should be valid base64
	b64Data := strings.TrimPrefix(dataURL, prefix)
	pngData, err := base64.StdEncoding.DecodeString(b64Data)
	if err != nil {
		t.Fatalf("GenerateQRCode() returned invalid base64: %v", err)
	}

	// Should be a PNG (check magic bytes)
	if len(pngData) < 8 {
		t.Fatal("GenerateQRCode() returned too small PNG data")
	}
	pngMagic := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	for i, b := range pngMagic {
		if pngData[i] != b {
			t.Errorf("GenerateQRCode() PNG magic byte %d: got %02x, want %02x", i, pngData[i], b)
		}
	}
}

func TestGenerateQRCodeDifferentInputs(t *testing.T) {
	uri1 := "otpauth://totp/Test:user1@example.com?secret=AAAA"
	uri2 := "otpauth://totp/Test:user2@example.com?secret=BBBB"

	qr1, err := GenerateQRCode(uri1)
	if err != nil {
		t.Fatalf("GenerateQRCode(uri1) error = %v", err)
	}

	qr2, err := GenerateQRCode(uri2)
	if err != nil {
		t.Fatalf("GenerateQRCode(uri2) error = %v", err)
	}

	// Different inputs should produce different QR codes
	if qr1 == qr2 {
		t.Error("GenerateQRCode() returned same QR code for different URIs")
	}
}
