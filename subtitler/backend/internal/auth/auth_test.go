package auth

import (
	"testing"
	"time"
)

func TestHashPassword(t *testing.T) {
	password := "mySecretPassword123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if hash == "" {
		t.Fatal("HashPassword returned empty hash")
	}

	if hash == password {
		t.Fatal("HashPassword returned plaintext password")
	}
}

func TestVerifyPassword(t *testing.T) {
	password := "mySecretPassword123"
	wrongPassword := "wrongPassword"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	// Test correct password
	err = VerifyPassword(hash, password)
	if err != nil {
		t.Errorf("VerifyPassword failed with correct password: %v", err)
	}

	// Test wrong password
	err = VerifyPassword(hash, wrongPassword)
	if err != ErrInvalidPassword {
		t.Errorf("VerifyPassword should return ErrInvalidPassword for wrong password, got: %v", err)
	}
}

func TestGenerateJWT(t *testing.T) {
	userID := int64(123)
	email := "test@example.com"
	secret := "test-secret-key"

	token, err := GenerateJWT(userID, email, secret)
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}

	if token == "" {
		t.Fatal("GenerateJWT returned empty token")
	}
}

func TestValidateJWT(t *testing.T) {
	userID := int64(123)
	email := "test@example.com"
	secret := "test-secret-key"

	// Generate a token
	token, err := GenerateJWT(userID, email, secret)
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}

	// Validate the token
	claims, err := ValidateJWT(token, secret)
	if err != nil {
		t.Fatalf("ValidateJWT failed: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("Expected UserID %d, got %d", userID, claims.UserID)
	}

	if claims.Email != email {
		t.Errorf("Expected Email %s, got %s", email, claims.Email)
	}

	// Validate with wrong secret
	_, err = ValidateJWT(token, "wrong-secret")
	if err != ErrInvalidToken {
		t.Errorf("ValidateJWT should fail with wrong secret, got: %v", err)
	}
}

func TestValidateJWT_InvalidToken(t *testing.T) {
	secret := "test-secret-key"

	// Test with invalid token string
	_, err := ValidateJWT("invalid.token.string", secret)
	if err != ErrInvalidToken {
		t.Errorf("ValidateJWT should return ErrInvalidToken for invalid token, got: %v", err)
	}

	// Test with empty token
	_, err = ValidateJWT("", secret)
	if err != ErrInvalidToken {
		t.Errorf("ValidateJWT should return ErrInvalidToken for empty token, got: %v", err)
	}
}

func TestJWTExpiry(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping expiry test in short mode")
	}

	userID := int64(123)
	email := "test@example.com"
	secret := "test-secret-key"

	token, err := GenerateJWT(userID, email, secret)
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}

	// Token should be valid immediately
	_, err = ValidateJWT(token, secret)
	if err != nil {
		t.Errorf("Token should be valid immediately after generation: %v", err)
	}

	// Verify expiration is set correctly
	claims, _ := ValidateJWT(token, secret)
	expiresAt := claims.ExpiresAt.Time
	expectedExpiry := time.Now().Add(TokenExpiry)

	// Allow 1 second difference for test execution time
	timeDiff := expiresAt.Sub(expectedExpiry)
	if timeDiff > time.Second || timeDiff < -time.Second {
		t.Errorf("Token expiry not set correctly. Expected ~%v, got %v", expectedExpiry, expiresAt)
	}
}
