package db

import (
	"os"
	"testing"
	"time"
)

func TestTOTPLifecycle(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create a user
	user := &User{
		ID:           "user-totp-test",
		Email:        "totp@example.com",
		PasswordHash: "hashedpassword",
		CreatedAt:    time.Now(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Verify user is created without TOTP
	retrieved, err := db.GetUserByID("user-totp-test")
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}
	if retrieved.TOTPEnabled {
		t.Error("Expected TOTP to be disabled initially")
	}
	if retrieved.TOTPSecret != nil {
		t.Error("Expected TOTP secret to be nil initially")
	}

	// Set TOTP secret
	secret := "JBSWY3DPEHPK3PXP"
	if err := db.SetTOTPSecret("user-totp-test", secret); err != nil {
		t.Fatalf("Failed to set TOTP secret: %v", err)
	}

	// Verify secret is set but not enabled
	retrieved, _ = db.GetUserByID("user-totp-test")
	if retrieved.TOTPSecret == nil || *retrieved.TOTPSecret != secret {
		t.Error("Expected TOTP secret to be set")
	}
	if retrieved.TOTPEnabled {
		t.Error("Expected TOTP to still be disabled")
	}

	// Enable TOTP
	if err := db.EnableTOTP("user-totp-test"); err != nil {
		t.Fatalf("Failed to enable TOTP: %v", err)
	}

	// Verify TOTP is now enabled
	retrieved, _ = db.GetUserByID("user-totp-test")
	if !retrieved.TOTPEnabled {
		t.Error("Expected TOTP to be enabled")
	}
	if retrieved.TOTPSecret == nil || *retrieved.TOTPSecret != secret {
		t.Error("Expected TOTP secret to still be set")
	}

	// Disable TOTP
	if err := db.DisableTOTP("user-totp-test"); err != nil {
		t.Fatalf("Failed to disable TOTP: %v", err)
	}

	// Verify TOTP is disabled and secret is cleared
	retrieved, _ = db.GetUserByID("user-totp-test")
	if retrieved.TOTPEnabled {
		t.Error("Expected TOTP to be disabled")
	}
	if retrieved.TOTPSecret != nil {
		t.Error("Expected TOTP secret to be cleared")
	}
}

func TestGetUserByEmailWithTOTP(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create a user with TOTP enabled
	user := &User{
		ID:           "user-email-totp",
		Email:        "totpemail@example.com",
		PasswordHash: "hashedpassword",
		CreatedAt:    time.Now(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Set and enable TOTP
	secret := "TESTSECRETZZZZZZ"
	if err := db.SetTOTPSecret("user-email-totp", secret); err != nil {
		t.Fatalf("Failed to set TOTP secret: %v", err)
	}
	if err := db.EnableTOTP("user-email-totp"); err != nil {
		t.Fatalf("Failed to enable TOTP: %v", err)
	}

	// Get by email and verify TOTP fields
	retrieved, err := db.GetUserByEmail("totpemail@example.com")
	if err != nil {
		t.Fatalf("Failed to get user by email: %v", err)
	}
	if !retrieved.TOTPEnabled {
		t.Error("Expected TOTP to be enabled")
	}
	if retrieved.TOTPSecret == nil || *retrieved.TOTPSecret != secret {
		t.Error("Expected TOTP secret to match")
	}
}

func TestRecoveryCodesLifecycle(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create a user
	user := &User{
		ID:           "user-recovery",
		Email:        "recovery@example.com",
		PasswordHash: "hashedpassword",
		CreatedAt:    time.Now(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Initially no recovery codes
	codes, err := db.GetUnusedRecoveryCodes("user-recovery")
	if err != nil {
		t.Fatalf("Failed to get recovery codes: %v", err)
	}
	if len(codes) != 0 {
		t.Errorf("Expected 0 codes initially, got %d", len(codes))
	}

	// Save 10 hashed recovery codes
	hashes := []string{
		"$2a$10$hash1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		"$2a$10$hash2xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		"$2a$10$hash3xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		"$2a$10$hash4xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		"$2a$10$hash5xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		"$2a$10$hash6xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		"$2a$10$hash7xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		"$2a$10$hash8xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		"$2a$10$hash9xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		"$2a$10$hash10xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
	}
	if err := db.SaveRecoveryCodes("user-recovery", hashes); err != nil {
		t.Fatalf("Failed to save recovery codes: %v", err)
	}

	// Verify all 10 codes exist
	codes, err = db.GetUnusedRecoveryCodes("user-recovery")
	if err != nil {
		t.Fatalf("Failed to get recovery codes: %v", err)
	}
	if len(codes) != 10 {
		t.Errorf("Expected 10 codes, got %d", len(codes))
	}

	// Count unused codes
	count, err := db.CountUnusedRecoveryCodes("user-recovery")
	if err != nil {
		t.Fatalf("Failed to count codes: %v", err)
	}
	if count != 10 {
		t.Errorf("Expected count 10, got %d", count)
	}

	// Use one code
	codeID := codes[0].ID
	success, err := db.UseRecoveryCode(codeID)
	if err != nil {
		t.Fatalf("Failed to use recovery code: %v", err)
	}
	if !success {
		t.Error("Expected use to succeed")
	}

	// Verify count reduced
	count, _ = db.CountUnusedRecoveryCodes("user-recovery")
	if count != 9 {
		t.Errorf("Expected count 9 after using one, got %d", count)
	}

	// Try to use the same code again
	success, err = db.UseRecoveryCode(codeID)
	if err != nil {
		t.Fatalf("Failed to check used code: %v", err)
	}
	if success {
		t.Error("Expected second use to fail")
	}
}

func TestRecoveryCodesRegenerate(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create a user
	user := &User{
		ID:           "user-regen",
		Email:        "regen@example.com",
		PasswordHash: "hashedpassword",
		CreatedAt:    time.Now(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Save initial codes
	initialHashes := []string{"$2a$10$initialxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"}
	if err := db.SaveRecoveryCodes("user-regen", initialHashes); err != nil {
		t.Fatalf("Failed to save initial codes: %v", err)
	}

	// Regenerate with new codes (old ones should be deleted)
	newHashes := []string{
		"$2a$10$new1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
		"$2a$10$new2xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
	}
	if err := db.SaveRecoveryCodes("user-regen", newHashes); err != nil {
		t.Fatalf("Failed to save new codes: %v", err)
	}

	// Should have exactly 2 new codes
	codes, err := db.GetUnusedRecoveryCodes("user-regen")
	if err != nil {
		t.Fatalf("Failed to get codes: %v", err)
	}
	if len(codes) != 2 {
		t.Errorf("Expected 2 codes after regeneration, got %d", len(codes))
	}
}

func TestRecoveryCodesDelete(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create a user
	user := &User{
		ID:           "user-delete-codes",
		Email:        "delete@example.com",
		PasswordHash: "hashedpassword",
		CreatedAt:    time.Now(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Save codes
	hashes := []string{"$2a$10$deletexxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"}
	if err := db.SaveRecoveryCodes("user-delete-codes", hashes); err != nil {
		t.Fatalf("Failed to save codes: %v", err)
	}

	// Delete all codes
	if err := db.DeleteRecoveryCodes("user-delete-codes"); err != nil {
		t.Fatalf("Failed to delete codes: %v", err)
	}

	// Verify all deleted
	count, _ := db.CountUnusedRecoveryCodes("user-delete-codes")
	if count != 0 {
		t.Errorf("Expected 0 codes after delete, got %d", count)
	}
}

func TestEnableTOTPWithRecoveryCodes(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create a user with TOTP secret
	user := &User{
		ID:           "totp123",
		Email:        "totp@example.com",
		PasswordHash: "hashhash",
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Set TOTP secret first
	if err := db.SetTOTPSecret(user.ID, "secret123"); err != nil {
		t.Fatalf("Failed to set TOTP secret: %v", err)
	}

	// Enable TOTP with recovery codes atomically
	codeHashes := []string{"hash1", "hash2", "hash3"}
	if err := db.EnableTOTPWithRecoveryCodes(user.ID, codeHashes); err != nil {
		t.Fatalf("Failed to enable TOTP with recovery codes: %v", err)
	}

	// Verify TOTP is enabled
	updatedUser, err := db.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}
	if !updatedUser.TOTPEnabled {
		t.Error("Expected TOTP to be enabled")
	}

	// Verify recovery codes were saved
	codes, err := db.GetUnusedRecoveryCodes(user.ID)
	if err != nil {
		t.Fatalf("Failed to get recovery codes: %v", err)
	}
	if len(codes) != 3 {
		t.Errorf("Expected 3 recovery codes, got %d", len(codes))
	}
}

func TestDisableTOTPAndClearSessions(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Create a user with 2FA enabled
	user := &User{
		ID:           "disable2fa123",
		Email:        "disable2fa@example.com",
		PasswordHash: "hashhash",
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Enable TOTP
	if err := db.SetTOTPSecret(user.ID, "secret"); err != nil {
		t.Fatalf("Failed to set TOTP secret: %v", err)
	}
	if err := db.EnableTOTP(user.ID); err != nil {
		t.Fatalf("Failed to enable TOTP: %v", err)
	}

	// Add recovery codes
	if err := db.SaveRecoveryCodes(user.ID, []string{"hash1", "hash2"}); err != nil {
		t.Fatalf("Failed to save recovery codes: %v", err)
	}

	// Create a session
	session := &Session{
		ID:        "session2fa123",
		UserID:    user.ID,
		Token:     "token2fa123",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	if err := db.CreateSession(session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Disable 2FA and clear sessions atomically
	if err := db.DisableTOTPAndClearSessions(user.ID); err != nil {
		t.Fatalf("Failed to disable TOTP and clear sessions: %v", err)
	}

	// Verify TOTP is disabled
	updatedUser, err := db.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}
	if updatedUser.TOTPEnabled {
		t.Error("Expected TOTP to be disabled")
	}

	// Verify recovery codes were deleted
	codes, err := db.GetUnusedRecoveryCodes(user.ID)
	if err != nil {
		t.Fatalf("Failed to get recovery codes: %v", err)
	}
	if len(codes) != 0 {
		t.Errorf("Expected 0 recovery codes, got %d", len(codes))
	}

	// Verify sessions were deleted (GetSessionByToken returns nil, nil for not found)
	deletedSession, err := db.GetSessionByToken("token2fa123")
	if err != nil {
		t.Fatalf("Error checking session: %v", err)
	}
	if deletedSession != nil {
		t.Error("Expected session to be deleted")
	}
}
