package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tpott/subtitler/backend/db"
)

func TestHashAndCheckPassword(t *testing.T) {
	password := "testpassword123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	// Hash should not be the same as password
	if hash == password {
		t.Error("Hash should not equal plain password")
	}

	// Check correct password
	if !CheckPassword(password, hash) {
		t.Error("CheckPassword should return true for correct password")
	}

	// Check wrong password
	if CheckPassword("wrongpassword", hash) {
		t.Error("CheckPassword should return false for wrong password")
	}
}

func TestGenerateToken(t *testing.T) {
	token1, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	token2, err := GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	// Tokens should be 64 hex characters (32 bytes)
	if len(token1) != 64 {
		t.Errorf("Token length should be 64, got %d", len(token1))
	}

	// Tokens should be unique
	if token1 == token2 {
		t.Error("Generated tokens should be unique")
	}
}

func TestGenerateID(t *testing.T) {
	id1, err := GenerateID()
	if err != nil {
		t.Fatalf("GenerateID failed: %v", err)
	}

	id2, err := GenerateID()
	if err != nil {
		t.Fatalf("GenerateID failed: %v", err)
	}

	// IDs should be 32 hex characters (16 bytes)
	if len(id1) != 32 {
		t.Errorf("ID length should be 32, got %d", len(id1))
	}

	// IDs should be unique
	if id1 == id2 {
		t.Error("Generated IDs should be unique")
	}
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		email   string
		wantErr bool
	}{
		{"test@example.com", false},
		{"user@domain.org", false},
		{"", true},
		{"invalid", true},
		{"no@dots", true},
		{"   test@example.com   ", false}, // should be trimmed
	}

	for _, tt := range tests {
		err := ValidateEmail(tt.email)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateEmail(%q) error = %v, wantErr %v", tt.email, err, tt.wantErr)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		password string
		wantErr  bool
		errMsg   string
	}{
		// Valid passwords
		{"Password1!", false, ""},
		{"Abcdefg1@", false, ""},
		{"Complex$Pass123", false, ""},
		{"Test@2024", false, ""},

		// Too short
		{"short", true, "at least 8 characters"},
		{"1234567", true, "at least 8 characters"},
		{"Aa1!", true, "at least 8 characters"},
		{"", true, "required"},

		// Missing uppercase
		{"password1!", true, "uppercase"},
		{"abcdefg1@", true, "uppercase"},

		// Missing lowercase
		{"PASSWORD1!", true, "lowercase"},
		{"ABCDEFG1@", true, "lowercase"},

		// Missing number
		{"Password!", true, "number"},
		{"Abcdefgh@", true, "number"},

		// Missing special character
		{"Password1", true, "special character"},
		{"Abcdefg12", true, "special character"},
		{"12345678Aa", true, "special character"},

		// Just passes all requirements (edge case)
		{"Passw0rd!", false, ""},
	}

	for _, tt := range tests {
		err := ValidatePassword(tt.password)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidatePassword(%q) error = %v, wantErr %v", tt.password, err, tt.wantErr)
		}
		if tt.wantErr && err != nil && tt.errMsg != "" {
			if !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("ValidatePassword(%q) error = %q, expected to contain %q", tt.password, err.Error(), tt.errMsg)
			}
		}
	}
}

func TestIsSpecialChar(t *testing.T) {
	// Test special characters
	specialChars := "!@#$%^&*()_+-=[]{}|;:'\",.<>?/`~\\"
	for _, r := range specialChars {
		if !isSpecialChar(r) {
			t.Errorf("isSpecialChar(%q) = false, want true", r)
		}
	}

	// Test non-special characters
	nonSpecial := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	for _, r := range nonSpecial {
		if isSpecialChar(r) {
			t.Errorf("isSpecialChar(%q) = true, want false", r)
		}
	}
}

func TestSessionLifecycle(t *testing.T) {
	// Create temp database
	tmpDir, err := os.MkdirTemp("", "auth_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	// Create a user
	userID, _ := GenerateID()
	hash, _ := HashPassword("testpassword")
	user := &db.User{
		ID:           userID,
		Email:        "test@example.com",
		PasswordHash: hash,
		CreatedAt:    time.Now(),
	}
	if err := database.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create session with IP and user agent
	session, err := CreateSession(database, userID, "192.168.1.1", "Test Agent")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if session.UserID != userID {
		t.Errorf("Session UserID = %v, want %v", session.UserID, userID)
	}

	if session.Token == "" {
		t.Error("Session Token should not be empty")
	}

	if session.IPAddress != "192.168.1.1" {
		t.Errorf("Session IPAddress = %v, want %v", session.IPAddress, "192.168.1.1")
	}

	if session.UserAgent != "Test Agent" {
		t.Errorf("Session UserAgent = %v, want %v", session.UserAgent, "Test Agent")
	}

	// Validate session
	validatedUser, validatedSession, err := ValidateSession(database, session.Token)
	if err != nil {
		t.Fatalf("ValidateSession failed: %v", err)
	}

	if validatedUser == nil {
		t.Fatal("ValidateSession should return user")
	}

	if validatedUser.ID != userID {
		t.Errorf("Validated user ID = %v, want %v", validatedUser.ID, userID)
	}

	if validatedSession.Token != session.Token {
		t.Errorf("Validated session token mismatch")
	}

	// Validate with wrong token should return nil
	nilUser, nilSession, err := ValidateSession(database, "invalidtoken")
	if err != nil {
		t.Fatalf("ValidateSession with invalid token failed: %v", err)
	}
	if nilUser != nil || nilSession != nil {
		t.Error("ValidateSession with invalid token should return nil user and session")
	}

	// Delete session
	if err := database.DeleteSession(session.Token); err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}

	// Session should no longer be valid
	nilUser, nilSession, err = ValidateSession(database, session.Token)
	if err != nil {
		t.Fatalf("ValidateSession after delete failed: %v", err)
	}
	if nilUser != nil || nilSession != nil {
		t.Error("ValidateSession after delete should return nil")
	}
}

func TestValidateSessionExpired(t *testing.T) {
	// Create temp database
	tmpDir, err := os.MkdirTemp("", "auth_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	// Create a user
	userID, _ := GenerateID()
	hash, _ := HashPassword("testpassword")
	user := &db.User{
		ID:           userID,
		Email:        "expired@example.com",
		PasswordHash: hash,
		CreatedAt:    time.Now(),
	}
	if err := database.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create a session that's already expired
	token, _ := GenerateToken()
	sessionID, _ := GenerateID()
	expiredSession := &db.Session{
		ID:        sessionID,
		UserID:    userID,
		Token:     token,
		IPAddress: "192.168.1.1",
		UserAgent: "Test Agent",
		CreatedAt: time.Now().Add(-48 * time.Hour),
		ExpiresAt: time.Now().Add(-24 * time.Hour), // Expired 24 hours ago
	}
	if err := database.CreateSession(expiredSession); err != nil {
		t.Fatalf("Failed to create expired session: %v", err)
	}

	// Validate expired session should return nil and clean it up
	validatedUser, validatedSession, err := ValidateSession(database, token)
	if err != nil {
		t.Fatalf("ValidateSession with expired token failed: %v", err)
	}
	if validatedUser != nil || validatedSession != nil {
		t.Error("ValidateSession with expired session should return nil user and session")
	}

	// Session should have been deleted from database
	dbSession, err := database.GetSessionByToken(token)
	if err != nil {
		t.Fatalf("GetSessionByToken failed: %v", err)
	}
	if dbSession != nil {
		t.Error("Expired session should have been deleted from database")
	}
}

func TestUserLifecycle(t *testing.T) {
	// Create temp database
	tmpDir, err := os.MkdirTemp("", "auth_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	// Create a user
	userID, _ := GenerateID()
	hash, _ := HashPassword("testpassword")
	email := "test@example.com"

	user := &db.User{
		ID:           userID,
		Email:        email,
		PasswordHash: hash,
		CreatedAt:    time.Now(),
	}
	if err := database.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Get user by ID
	foundUser, err := database.GetUserByID(userID)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if foundUser == nil {
		t.Fatal("GetUserByID should return user")
	}
	if foundUser.Email != email {
		t.Errorf("User email = %v, want %v", foundUser.Email, email)
	}

	// Get user by email
	foundUser, err = database.GetUserByEmail(email)
	if err != nil {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}
	if foundUser == nil {
		t.Fatal("GetUserByEmail should return user")
	}
	if foundUser.ID != userID {
		t.Errorf("User ID = %v, want %v", foundUser.ID, userID)
	}

	// Get non-existent user
	notFound, err := database.GetUserByEmail("notexist@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail for non-existent failed: %v", err)
	}
	if notFound != nil {
		t.Error("GetUserByEmail should return nil for non-existent user")
	}

	// Check password
	if !CheckPassword("testpassword", foundUser.PasswordHash) {
		t.Error("CheckPassword should return true for correct password")
	}
	if CheckPassword("wrongpassword", foundUser.PasswordHash) {
		t.Error("CheckPassword should return false for wrong password")
	}
}

func TestIsHTTPSOnly(t *testing.T) {
	// Save original env var and restore after test
	original := os.Getenv("HTTPS_ONLY")
	defer os.Setenv("HTTPS_ONLY", original)

	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{"empty", "", false},
		{"zero", "0", false},
		{"one", "1", true},
		{"true lowercase", "true", true},
		{"TRUE uppercase", "TRUE", true},
		{"True mixed", "True", true},
		{"false", "false", false},
		{"random", "yes", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("HTTPS_ONLY", tt.envValue)
			InitHTTPSOnly()
			if got := IsHTTPSOnly(); got != tt.expected {
				t.Errorf("IsHTTPSOnly() with HTTPS_ONLY=%q = %v, want %v", tt.envValue, got, tt.expected)
			}
		})
	}
}
