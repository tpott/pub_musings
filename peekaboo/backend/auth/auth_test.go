package auth

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashPassword(t *testing.T) {
	hash, err := HashPassword("testpassword123")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	if hash == "" {
		t.Fatal("HashPassword returned empty string")
	}
	// Verify bcrypt cost
	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil {
		t.Fatalf("bcrypt.Cost failed: %v", err)
	}
	if cost != BcryptCost {
		t.Errorf("bcrypt cost = %d, want %d", cost, BcryptCost)
	}
}

func TestVerifyPassword(t *testing.T) {
	password := "testpassword123"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	// Correct password
	if err := VerifyPassword(hash, password); err != nil {
		t.Errorf("VerifyPassword with correct password failed: %v", err)
	}

	// Wrong password
	if err := VerifyPassword(hash, "wrongpassword"); err == nil {
		t.Error("VerifyPassword with wrong password should fail")
	}
}

func TestVerifyPasswordMismatch(t *testing.T) {
	hash, _ := HashPassword("password1")
	err := VerifyPassword(hash, "password2")
	if err != bcrypt.ErrMismatchedHashAndPassword {
		t.Errorf("expected ErrMismatchedHashAndPassword, got %v", err)
	}
}

func TestGenerateToken(t *testing.T) {
	token1, err := GenerateToken(32)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	// 32 bytes -> 64 hex chars
	if len(token1) != 64 {
		t.Errorf("token length = %d, want 64", len(token1))
	}

	// Tokens should be unique
	token2, err := GenerateToken(32)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	if token1 == token2 {
		t.Error("two generated tokens should not be equal")
	}
}

func TestGenerateTokenLengths(t *testing.T) {
	tests := []struct {
		byteLen int
		hexLen  int
	}{
		{16, 32},
		{32, 64},
		{8, 16},
	}
	for _, tt := range tests {
		token, err := GenerateToken(tt.byteLen)
		if err != nil {
			t.Fatalf("GenerateToken(%d) failed: %v", tt.byteLen, err)
		}
		if len(token) != tt.hexLen {
			t.Errorf("GenerateToken(%d) length = %d, want %d", tt.byteLen, len(token), tt.hexLen)
		}
	}
}

func TestGenerateID(t *testing.T) {
	id, err := GenerateID()
	if err != nil {
		t.Fatalf("GenerateID failed: %v", err)
	}
	// 16 bytes -> 32 hex chars
	if len(id) != 32 {
		t.Errorf("ID length = %d, want 32", len(id))
	}
}

func TestHashToken(t *testing.T) {
	token := "abc123"
	hash := HashToken(token)

	// SHA-256 produces 64 hex chars
	if len(hash) != 64 {
		t.Errorf("hash length = %d, want 64", len(hash))
	}

	// Same input should produce same hash
	hash2 := HashToken(token)
	if hash != hash2 {
		t.Error("same token should produce same hash")
	}

	// Different input should produce different hash
	hash3 := HashToken("different")
	if hash == hash3 {
		t.Error("different tokens should produce different hashes")
	}
}

func TestGenerateCSRF(t *testing.T) {
	secret := []byte("test-csrf-secret")
	sessionToken := "session-token-abc123"

	csrf := GenerateCSRF(sessionToken, secret)
	if csrf == "" {
		t.Fatal("GenerateCSRF returned empty string")
	}

	// HMAC-SHA256 produces 64 hex chars
	if len(csrf) != 64 {
		t.Errorf("CSRF token length = %d, want 64", len(csrf))
	}

	// Same inputs should produce same CSRF
	csrf2 := GenerateCSRF(sessionToken, secret)
	if csrf != csrf2 {
		t.Error("same inputs should produce same CSRF token")
	}

	// Different session token should produce different CSRF
	csrf3 := GenerateCSRF("different-session", secret)
	if csrf == csrf3 {
		t.Error("different session tokens should produce different CSRF tokens")
	}

	// Different secret should produce different CSRF
	csrf4 := GenerateCSRF(sessionToken, []byte("different-secret"))
	if csrf == csrf4 {
		t.Error("different secrets should produce different CSRF tokens")
	}
}

func TestValidateCSRF(t *testing.T) {
	secret := []byte("test-csrf-secret")
	sessionToken := "session-token-abc123"
	csrf := GenerateCSRF(sessionToken, secret)

	// Valid CSRF
	if !ValidateCSRF(csrf, sessionToken, secret) {
		t.Error("ValidateCSRF should return true for valid token")
	}

	// Tampered CSRF
	if ValidateCSRF("tampered-token", sessionToken, secret) {
		t.Error("ValidateCSRF should return false for tampered token")
	}

	// Wrong session token
	if ValidateCSRF(csrf, "wrong-session", secret) {
		t.Error("ValidateCSRF should return false for wrong session token")
	}

	// Wrong secret
	if ValidateCSRF(csrf, sessionToken, []byte("wrong-secret")) {
		t.Error("ValidateCSRF should return false for wrong secret")
	}

	// Empty CSRF token
	if ValidateCSRF("", sessionToken, secret) {
		t.Error("ValidateCSRF should return false for empty token")
	}
}

func TestConstants(t *testing.T) {
	if BcryptCost != 12 {
		t.Errorf("BcryptCost = %d, want 12", BcryptCost)
	}
	if MinPasswordLength != 8 {
		t.Errorf("MinPasswordLength = %d, want 8", MinPasswordLength)
	}
	if MaxPasswordLength != 72 {
		t.Errorf("MaxPasswordLength = %d, want 72", MaxPasswordLength)
	}
	if MaxEmailLength != 255 {
		t.Errorf("MaxEmailLength = %d, want 255", MaxEmailLength)
	}
	if LockoutThreshold != 5 {
		t.Errorf("LockoutThreshold = %d, want 5", LockoutThreshold)
	}
}
