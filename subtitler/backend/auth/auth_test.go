package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/trevor/subtitler/backend/db"
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
	}{
		{"password123", false},
		{"short", true},     // Too short
		{"1234567", true},   // Too short (7 chars)
		{"12345678", false}, // Exactly 8 chars (minimum)
		{"", true},
	}

	for _, tt := range tests {
		err := ValidatePassword(tt.password)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidatePassword(%q) error = %v, wantErr %v", tt.password, err, tt.wantErr)
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

	// Create session
	session, err := CreateSession(database, userID)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if session.UserID != userID {
		t.Errorf("Session UserID = %v, want %v", session.UserID, userID)
	}

	if session.Token == "" {
		t.Error("Session Token should not be empty")
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
