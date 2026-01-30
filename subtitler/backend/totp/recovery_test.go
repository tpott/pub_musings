package totp

import (
	"strings"
	"testing"
)

func TestGenerateRecoveryCodes(t *testing.T) {
	codes, err := GenerateRecoveryCodes(NumCodes)
	if err != nil {
		t.Fatalf("Failed to generate codes: %v", err)
	}

	if len(codes) != NumCodes {
		t.Errorf("Expected %d codes, got %d", NumCodes, len(codes))
	}

	// Check uniqueness
	seen := make(map[string]bool)
	for _, code := range codes {
		if seen[code] {
			t.Errorf("Duplicate code found: %s", code)
		}
		seen[code] = true
	}
}

func TestCodeFormat(t *testing.T) {
	codes, err := GenerateRecoveryCodes(1)
	if err != nil {
		t.Fatalf("Failed to generate code: %v", err)
	}

	code := codes[0]

	// Should be formatted as XXXX-XXXX
	if len(code) != 9 { // 8 chars + 1 hyphen
		t.Errorf("Expected code length 9, got %d: %s", len(code), code)
	}

	if code[4] != '-' {
		t.Errorf("Expected hyphen at position 4, got: %s", code)
	}
}

func TestCodeAlphabet(t *testing.T) {
	codes, err := GenerateRecoveryCodes(100) // Generate many to test randomness
	if err != nil {
		t.Fatalf("Failed to generate codes: %v", err)
	}

	for _, code := range codes {
		normalized := NormalizeCode(code)
		for _, char := range normalized {
			if !strings.ContainsRune(Alphabet, char) {
				t.Errorf("Invalid character %c in code %s", char, code)
			}
		}
	}
}

func TestNormalizeCode(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"A3X7-K9M2", "A3X7K9M2"},
		{"a3x7-k9m2", "A3X7K9M2"},
		{"a3x7k9m2", "A3X7K9M2"},
		{"A3X7K9M2", "A3X7K9M2"},
		{"", ""},
	}

	for _, tt := range tests {
		result := NormalizeCode(tt.input)
		if result != tt.expected {
			t.Errorf("NormalizeCode(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestFormatCode(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"A3X7K9M2", "A3X7-K9M2"},
		{"12345678", "1234-5678"},
		{"short", "short"}, // Returns unchanged if wrong length
	}

	for _, tt := range tests {
		result := FormatCode(tt.input)
		if result != tt.expected {
			t.Errorf("FormatCode(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestHashAndCheckCode(t *testing.T) {
	codes, err := GenerateRecoveryCodes(1)
	if err != nil {
		t.Fatalf("Failed to generate code: %v", err)
	}

	code := codes[0]
	hash, err := HashCode(code)
	if err != nil {
		t.Fatalf("Failed to hash code: %v", err)
	}

	// Verify correct code
	if !CheckCode(code, hash) {
		t.Error("CheckCode should return true for correct code")
	}

	// Verify lowercase version works
	if !CheckCode(strings.ToLower(code), hash) {
		t.Error("CheckCode should work with lowercase input")
	}

	// Verify without hyphen works
	if !CheckCode(NormalizeCode(code), hash) {
		t.Error("CheckCode should work without hyphen")
	}
}

func TestCheckCodeWrong(t *testing.T) {
	codes, err := GenerateRecoveryCodes(2)
	if err != nil {
		t.Fatalf("Failed to generate codes: %v", err)
	}

	hash, err := HashCode(codes[0])
	if err != nil {
		t.Fatalf("Failed to hash code: %v", err)
	}

	// Wrong code should fail
	if CheckCode(codes[1], hash) {
		t.Error("CheckCode should return false for wrong code")
	}

	// Random string should fail
	if CheckCode("INVALID1", hash) {
		t.Error("CheckCode should return false for invalid code")
	}
}

func TestHashCodeUnique(t *testing.T) {
	code := "A3X7K9M2"
	hash1, err := HashCode(code)
	if err != nil {
		t.Fatalf("Failed to hash code: %v", err)
	}

	hash2, err := HashCode(code)
	if err != nil {
		t.Fatalf("Failed to hash code: %v", err)
	}

	// Bcrypt should produce different hashes for same input (due to salt)
	if hash1 == hash2 {
		t.Error("HashCode should produce unique hashes due to salt")
	}

	// But both should verify
	if !CheckCode(code, hash1) || !CheckCode(code, hash2) {
		t.Error("Both hashes should verify the same code")
	}
}
