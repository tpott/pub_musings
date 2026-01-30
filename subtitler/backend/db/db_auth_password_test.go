package db

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestPasswordResetTokenLifecycle(t *testing.T) {
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
		ID:           "user-reset-token",
		Email:        "reset@example.com",
		PasswordHash: "hashedpassword",
		CreatedAt:    time.Now(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create a reset token
	tokenHash := "abc123hashedtoken"
	expiresAt := time.Now().Add(1 * time.Hour)
	token, err := db.CreatePasswordResetToken(user.ID, tokenHash, expiresAt)
	if err != nil {
		t.Fatalf("Failed to create reset token: %v", err)
	}

	if token.UserID != user.ID {
		t.Errorf("Expected user ID %s, got %s", user.ID, token.UserID)
	}
	if token.TokenHash != tokenHash {
		t.Errorf("Expected token hash %s, got %s", tokenHash, token.TokenHash)
	}
	if token.Used {
		t.Error("New token should not be marked as used")
	}

	// Get the token
	retrieved, err := db.GetPasswordResetToken(tokenHash)
	if err != nil {
		t.Fatalf("Failed to get reset token: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Retrieved token is nil")
	}
	if retrieved.ID != token.ID {
		t.Errorf("Expected token ID %s, got %s", token.ID, retrieved.ID)
	}

	// Use the token
	used, err := db.UsePasswordResetToken(tokenHash)
	if err != nil {
		t.Fatalf("Failed to use reset token: %v", err)
	}
	if !used {
		t.Error("UsePasswordResetToken should return true for valid token")
	}

	// Try to use again (should fail)
	usedAgain, err := db.UsePasswordResetToken(tokenHash)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if usedAgain {
		t.Error("UsePasswordResetToken should return false for already-used token")
	}
}

func TestPasswordResetTokenExpired(t *testing.T) {
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
		ID:           "user-expired-token",
		Email:        "expired@example.com",
		PasswordHash: "hashedpassword",
		CreatedAt:    time.Now(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create an expired token
	tokenHash := "expiredtokenhash"
	expiresAt := time.Now().Add(-1 * time.Hour) // Already expired
	_, err = db.CreatePasswordResetToken(user.ID, tokenHash, expiresAt)
	if err != nil {
		t.Fatalf("Failed to create reset token: %v", err)
	}

	// Try to use expired token
	used, err := db.UsePasswordResetToken(tokenHash)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if used {
		t.Error("UsePasswordResetToken should return false for expired token")
	}
}

func TestPasswordResetTokenNotFound(t *testing.T) {
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

	// Try to get non-existent token
	token, err := db.GetPasswordResetToken("nonexistent")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if token != nil {
		t.Error("Expected nil for non-existent token")
	}
}

func TestDeletePasswordResetTokens(t *testing.T) {
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
		ID:           "user-delete-tokens",
		Email:        "deletetokens@example.com",
		PasswordHash: "hashedpassword",
		CreatedAt:    time.Now(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create a token
	tokenHash := "deletetokenhash"
	_, err = db.CreatePasswordResetToken(user.ID, tokenHash, time.Now().Add(1*time.Hour))
	if err != nil {
		t.Fatalf("Failed to create reset token: %v", err)
	}

	// Delete all tokens for user
	if err := db.DeletePasswordResetTokens(user.ID); err != nil {
		t.Fatalf("Failed to delete tokens: %v", err)
	}

	// Verify token is deleted
	token, _ := db.GetPasswordResetToken(tokenHash)
	if token != nil {
		t.Error("Token should be deleted")
	}
}

func TestUpdateUserPassword(t *testing.T) {
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
		ID:           "user-update-password",
		Email:        "updatepwd@example.com",
		PasswordHash: "oldhash",
		CreatedAt:    time.Now(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Update password
	newHash := "newhash"
	if err := db.UpdateUserPassword(user.ID, newHash); err != nil {
		t.Fatalf("Failed to update password: %v", err)
	}

	// Verify update
	updated, _ := db.GetUserByID(user.ID)
	if updated.PasswordHash != newHash {
		t.Errorf("Expected password hash %s, got %s", newHash, updated.PasswordHash)
	}
}

func TestLoginAttempts(t *testing.T) {
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

	email := "test@example.com"
	ip := "192.168.1.1"

	// Initially should have 0 failed attempts
	count, err := db.GetRecentFailedLoginAttempts(email, time.Now().Add(-15*time.Minute))
	if err != nil {
		t.Fatalf("Failed to get login attempts: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 failed attempts, got %d", count)
	}

	// Record 3 failed attempts
	for i := 0; i < 3; i++ {
		if err := db.RecordLoginAttempt(email, ip, false); err != nil {
			t.Fatalf("Failed to record login attempt: %v", err)
		}
	}

	// Should now have 3 failed attempts
	count, err = db.GetRecentFailedLoginAttempts(email, time.Now().Add(-15*time.Minute))
	if err != nil {
		t.Fatalf("Failed to get login attempts: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected 3 failed attempts, got %d", count)
	}

	// Email should not be locked yet (under 5 attempts)
	locked, _, err := db.IsEmailLocked(email, 5, 15*time.Minute)
	if err != nil {
		t.Fatalf("Failed to check email lock: %v", err)
	}
	if locked {
		t.Error("Email should not be locked with only 3 failed attempts")
	}

	// Record 2 more failed attempts (total 5)
	for i := 0; i < 2; i++ {
		if err := db.RecordLoginAttempt(email, ip, false); err != nil {
			t.Fatalf("Failed to record login attempt: %v", err)
		}
	}

	// Email should now be locked
	locked, unlockTime, err := db.IsEmailLocked(email, 5, 15*time.Minute)
	if err != nil {
		t.Fatalf("Failed to check email lock: %v", err)
	}
	if !locked {
		t.Error("Email should be locked with 5 failed attempts")
	}
	if unlockTime.Before(time.Now()) {
		t.Error("Unlock time should be in the future")
	}

	// Clear attempts (simulating successful login)
	if err := db.ClearLoginAttempts(email); err != nil {
		t.Fatalf("Failed to clear login attempts: %v", err)
	}

	// Should no longer be locked
	locked, _, err = db.IsEmailLocked(email, 5, 15*time.Minute)
	if err != nil {
		t.Fatalf("Failed to check email lock: %v", err)
	}
	if locked {
		t.Error("Email should not be locked after clearing attempts")
	}

	// Count should be 0 again
	count, err = db.GetRecentFailedLoginAttempts(email, time.Now().Add(-15*time.Minute))
	if err != nil {
		t.Fatalf("Failed to get login attempts: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 failed attempts after clearing, got %d", count)
	}
}

func TestDeleteExpiredLoginAttempts(t *testing.T) {
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

	email := "test@example.com"
	ip := "192.168.1.1"

	// Record 3 failed attempts
	for i := 0; i < 3; i++ {
		if err := db.RecordLoginAttempt(email, ip, false); err != nil {
			t.Fatalf("Failed to record login attempt: %v", err)
		}
	}

	// Verify we have 3 attempts
	count, _ := db.GetRecentFailedLoginAttempts(email, time.Now().Add(-1*time.Hour))
	if count != 3 {
		t.Errorf("Expected 3 attempts, got %d", count)
	}

	// Delete attempts older than "now" (should delete all)
	deleted, err := db.DeleteExpiredLoginAttempts(time.Now().Add(1 * time.Second))
	if err != nil {
		t.Fatalf("Failed to delete expired attempts: %v", err)
	}
	if deleted != 3 {
		t.Errorf("Expected to delete 3 attempts, deleted %d", deleted)
	}

	// Should have 0 attempts now
	count, _ = db.GetRecentFailedLoginAttempts(email, time.Now().Add(-1*time.Hour))
	if count != 0 {
		t.Errorf("Expected 0 attempts after deletion, got %d", count)
	}
}

func TestCompletePasswordReset(t *testing.T) {
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
		ID:           "pwreset123",
		Email:        "pwreset@example.com",
		PasswordHash: "oldhash",
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create a session
	session := &Session{
		ID:        "session123",
		UserID:    user.ID,
		Token:     "token123",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	if err := db.CreateSession(session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Create a password reset token
	tokenHash := "tokenhash123"
	token, err := db.CreatePasswordResetToken(user.ID, tokenHash, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("Failed to create reset token: %v", err)
	}

	// Complete password reset atomically
	newPasswordHash := "newhash"
	if err := db.CompletePasswordReset(user.ID, token.TokenHash, newPasswordHash); err != nil {
		t.Fatalf("Failed to complete password reset: %v", err)
	}

	// Verify password was updated
	updatedUser, err := db.GetUserByEmail(user.Email)
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}
	if updatedUser.PasswordHash != newPasswordHash {
		t.Errorf("Expected password hash '%s', got '%s'", newPasswordHash, updatedUser.PasswordHash)
	}

	// Verify sessions were deleted (GetSessionByToken returns nil, nil for not found)
	deletedSession, err := db.GetSessionByToken("token123")
	if err != nil {
		t.Fatalf("Error checking session: %v", err)
	}
	if deletedSession != nil {
		t.Error("Expected session to be deleted")
	}

	// Verify token was deleted
	retrievedToken, err := db.GetPasswordResetToken(tokenHash)
	if err == nil && retrievedToken != nil && !retrievedToken.Used {
		t.Error("Expected token to be used or deleted")
	}
}

func TestEmailVerificationTokenCleanupAfterVerification(t *testing.T) {
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

	// Create a test user
	user := &User{
		ID:            "test-user-cleanup",
		Email:         "cleanup@example.com",
		PasswordHash:  "testhash",
		EmailVerified: false,
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create a verification token
	expiresAt := time.Now().Add(24 * time.Hour)
	tokenHash := hashToken("verification-token")
	_, err = db.CreateEmailVerificationToken(user.ID, tokenHash, expiresAt)
	if err != nil {
		t.Fatalf("Failed to create verification token: %v", err)
	}

	// Verify we have 1 token for this user
	var count int
	err = db.conn.QueryRow(`SELECT COUNT(*) FROM email_verification_tokens WHERE user_id = ?`, user.ID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count tokens: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 token before verification, got %d", count)
	}

	// Use the token to verify email
	verified, err := db.UseEmailVerificationToken(tokenHash)
	if err != nil {
		t.Fatalf("Failed to use verification token: %v", err)
	}
	if !verified {
		t.Error("Expected token verification to succeed")
	}

	// Verify user is now verified
	verifiedUser, err := db.GetUserByEmail("cleanup@example.com")
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}
	if !verifiedUser.EmailVerified {
		t.Error("Expected user email to be verified")
	}

	// Verify the token has been deleted (cleanup)
	err = db.conn.QueryRow(`SELECT COUNT(*) FROM email_verification_tokens WHERE user_id = ?`, user.ID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count tokens after verification: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 tokens after verification (cleanup), got %d", count)
	}
}

func TestEmailVerificationTokenCleanupWithMultipleTokens(t *testing.T) {
	// Test that cleanup works when there are "used" tokens in the database
	// (Note: CreateEmailVerificationToken only keeps 1 unused token per user,
	//  but used tokens can accumulate. This test verifies we clean those up too.)
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

	// Create a test user
	user := &User{
		ID:            "test-user-multi",
		Email:         "multi@example.com",
		PasswordHash:  "testhash",
		EmailVerified: false,
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Manually insert some "used" tokens to simulate historical accumulation
	for i := 0; i < 3; i++ {
		id, _ := generateID()
		_, err := db.conn.Exec(`
			INSERT INTO email_verification_tokens (id, user_id, token_hash, expires_at, used, created_at)
			VALUES (?, ?, ?, ?, 1, ?)
		`, id, user.ID, hashToken(fmt.Sprintf("old-token-%d", i)), time.Now().Add(-1*time.Hour), time.Now().Add(-2*time.Hour))
		if err != nil {
			t.Fatalf("Failed to insert old token: %v", err)
		}
	}

	// Create a fresh verification token
	expiresAt := time.Now().Add(24 * time.Hour)
	tokenHash := hashToken("fresh-token")
	_, err = db.CreateEmailVerificationToken(user.ID, tokenHash, expiresAt)
	if err != nil {
		t.Fatalf("Failed to create verification token: %v", err)
	}

	// Verify we have 4 tokens total (3 used + 1 unused)
	var count int
	err = db.conn.QueryRow(`SELECT COUNT(*) FROM email_verification_tokens WHERE user_id = ?`, user.ID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count tokens: %v", err)
	}
	if count != 4 {
		t.Errorf("Expected 4 tokens before verification, got %d", count)
	}

	// Use the fresh token to verify email
	verified, err := db.UseEmailVerificationToken(tokenHash)
	if err != nil {
		t.Fatalf("Failed to use verification token: %v", err)
	}
	if !verified {
		t.Error("Expected token verification to succeed")
	}

	// Verify ALL tokens for this user have been deleted (including used ones)
	err = db.conn.QueryRow(`SELECT COUNT(*) FROM email_verification_tokens WHERE user_id = ?`, user.ID).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count tokens after verification: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 tokens after verification (cleanup of all tokens), got %d", count)
	}
}

// TestCountRecentMagicLinkRequests tests the per-email rate limiting for magic links
func TestCountRecentMagicLinkRequests(t *testing.T) {
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

	// Create a test user
	user := &User{
		ID:            "user-magiclink",
		Email:         "magic@example.com",
		PasswordHash:  "hash",
		EmailVerified: true,
		CreatedAt:     time.Now(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	t.Run("no tokens initially", func(t *testing.T) {
		count, err := db.CountRecentMagicLinkRequests(user.ID, time.Now().Add(-15*time.Minute))
		if err != nil {
			t.Fatalf("Failed to count tokens: %v", err)
		}
		if count != 0 {
			t.Errorf("Expected 0 tokens, got %d", count)
		}
	})

	t.Run("counts requests including used and expired tokens", func(t *testing.T) {
		// Insert tokens directly to test counting (bypasses CreateMagicLinkToken's delete behavior)
		// Token 1: Active and unused
		db.conn.Exec(`
			INSERT INTO magic_link_tokens (id, user_id, token_hash, used, created_at, expires_at)
			VALUES (?, ?, ?, 0, ?, ?)
		`, "token-1", user.ID, "hash1", time.Now(), time.Now().Add(15*time.Minute))

		// Token 2: Used (still counted for rate limiting)
		db.conn.Exec(`
			INSERT INTO magic_link_tokens (id, user_id, token_hash, used, created_at, expires_at)
			VALUES (?, ?, ?, 1, ?, ?)
		`, "token-2", user.ID, "hash2", time.Now(), time.Now().Add(15*time.Minute))

		// Token 3: Expired (still counted for rate limiting)
		db.conn.Exec(`
			INSERT INTO magic_link_tokens (id, user_id, token_hash, used, created_at, expires_at)
			VALUES (?, ?, ?, 0, ?, ?)
		`, "token-3", user.ID, "hash3", time.Now(), time.Now().Add(-1*time.Minute))

		// All 3 should be counted for rate limiting
		count, err := db.CountRecentMagicLinkRequests(user.ID, time.Now().Add(-15*time.Minute))
		if err != nil {
			t.Fatalf("Failed to count tokens: %v", err)
		}
		if count != 3 {
			t.Errorf("Expected 3 tokens (including used and expired), got %d", count)
		}
	})

	t.Run("excludes tokens before since time", func(t *testing.T) {
		// Create a token that's too old (created before the window)
		db.conn.Exec(`
			INSERT INTO magic_link_tokens (id, user_id, token_hash, used, created_at, expires_at)
			VALUES (?, ?, ?, 0, ?, ?)
		`, "old-token", user.ID, "old-hash", time.Now().Add(-30*time.Minute), time.Now().Add(15*time.Minute))

		// Count should exclude the old token
		count, err := db.CountRecentMagicLinkRequests(user.ID, time.Now().Add(-15*time.Minute))
		if err != nil {
			t.Fatalf("Failed to count tokens: %v", err)
		}
		if count != 3 {
			t.Errorf("Expected 3 tokens (old token not counted), got %d", count)
		}
	})

	t.Run("rate limit blocks after max requests", func(t *testing.T) {
		// Simulate behavior: after 3 requests, rate limit should kick in
		// The check happens before creating the token, so 3 tokens = rate limited
		const maxRequests = 3
		count, err := db.CountRecentMagicLinkRequests(user.ID, time.Now().Add(-15*time.Minute))
		if err != nil {
			t.Fatalf("Failed to count tokens: %v", err)
		}
		if count < maxRequests {
			t.Errorf("Expected at least %d requests to trigger rate limit, got %d", maxRequests, count)
		}
	})
}
