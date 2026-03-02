package auth

import (
	"strings"
	"testing"
)

func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"test@example.com", "test@example.com"},
		{"TEST@EXAMPLE.COM", "test@example.com"},
		{"  test@example.com  ", "test@example.com"},
		{"  TEST@Example.COM  ", "test@example.com"},
		{"", ""},
	}

	for _, tt := range tests {
		result := NormalizeEmail(tt.input)
		if result != tt.expected {
			t.Errorf("NormalizeEmail(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		email string
		valid bool
	}{
		{"user@example.com", true},
		{"user@sub.example.com", true},
		{"user+tag@example.com", true},
		{"a@b.co", true},
		{"", false},
		{"noatsign", false},
		{"@example.com", false},
		{"user@", false},
		{"user@localhost", false},
		{strings.Repeat("a", 250) + "@b.com", false}, // too long
	}

	for _, tt := range tests {
		err := ValidateEmail(tt.email)
		if tt.valid && err != nil {
			t.Errorf("ValidateEmail(%q) returned error %v, expected valid", tt.email, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("ValidateEmail(%q) returned nil, expected error", tt.email)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		password string
		valid    bool
	}{
		{"12345678", true},
		{strings.Repeat("a", 72), true},
		{"", false},
		{"short", false},
		{"1234567", false},
		{strings.Repeat("a", 73), false},
	}

	for _, tt := range tests {
		err := ValidatePassword(tt.password)
		if tt.valid && err != nil {
			t.Errorf("ValidatePassword(%q) returned error %v, expected valid", tt.password, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("ValidatePassword(%q) returned nil, expected error", tt.password)
		}
	}
}
