package api

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"testing"
	"time"

	"github.com/tpott/pub_musings/peekaboo/backend/db"
)

func TestCleanupExpiredAuth_Sessions(t *testing.T) {
	database := setupAuthTestDB(t)
	logger := slog.Default()

	user := createTestUser(t, database, "cleanup@example.com", "password123", true)

	// Create an expired session
	if err := database.CreateSession(&db.Session{
		ID:        "expired-sess",
		UserID:    user.ID,
		Token:     "tok-expired",
		ExpiresAt: time.Now().UTC().Add(-1 * time.Hour),
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Create a valid session
	if err := database.CreateSession(&db.Session{
		ID:        "valid-sess",
		UserID:    user.ID,
		Token:     "tok-valid",
		ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	result := CleanupExpiredAuth(database, logger)

	if result.Sessions != 1 {
		t.Errorf("Expected 1 expired session deleted, got %d", result.Sessions)
	}

	// Verify valid session still exists
	session, err := database.GetSessionByToken("tok-valid")
	if err != nil {
		t.Fatalf("GetSessionByToken failed: %v", err)
	}
	if session == nil {
		t.Error("Valid session should still exist after cleanup")
	}

	// Verify expired session is gone
	expiredSession, err := database.GetSessionByToken("tok-expired")
	if err != nil {
		t.Fatalf("GetSessionByToken failed: %v", err)
	}
	if expiredSession != nil {
		t.Error("Expired session should be deleted after cleanup")
	}
}

func TestCleanupExpiredAuth_NoExpired(t *testing.T) {
	database := setupAuthTestDB(t)
	logger := slog.Default()

	result := CleanupExpiredAuth(database, logger)

	if result.Sessions != 0 {
		t.Errorf("Expected 0 sessions deleted, got %d", result.Sessions)
	}
	if result.LoginAttempts != 0 {
		t.Errorf("Expected 0 attempts deleted, got %d", result.LoginAttempts)
	}
	if result.VerificationTokens != 0 {
		t.Errorf("Expected 0 verification tokens deleted, got %d", result.VerificationTokens)
	}
	if result.MagicLinkTokens != 0 {
		t.Errorf("Expected 0 magic link tokens deleted, got %d", result.MagicLinkTokens)
	}
}

func TestCleanupExpiredAuth_VerificationTokens(t *testing.T) {
	database := setupAuthTestDB(t)
	logger := slog.Default()

	user := createTestUser(t, database, "verify-cleanup@example.com", "password123", false)

	hashToken := func(token string) string {
		h := sha256.Sum256([]byte(token))
		return hex.EncodeToString(h[:])
	}

	// Create an expired verification token
	if err := database.StoreEmailVerificationToken(
		"expired-vt", user.ID, hashToken("expired-token"),
		time.Now().UTC().Add(-1*time.Hour),
	); err != nil {
		t.Fatalf("StoreEmailVerificationToken failed: %v", err)
	}

	// Create a valid verification token
	if err := database.StoreEmailVerificationToken(
		"valid-vt", user.ID, hashToken("valid-token"),
		time.Now().UTC().Add(24*time.Hour),
	); err != nil {
		t.Fatalf("StoreEmailVerificationToken failed: %v", err)
	}

	result := CleanupExpiredAuth(database, logger)

	if result.VerificationTokens != 1 {
		t.Errorf("Expected 1 expired verification token deleted, got %d", result.VerificationTokens)
	}

	// Verify valid token still exists
	id, _, _, _, err := database.GetEmailVerificationToken(hashToken("valid-token"))
	if err != nil {
		t.Fatalf("GetEmailVerificationToken failed: %v", err)
	}
	if id == "" {
		t.Error("Valid verification token should still exist after cleanup")
	}

	// Verify expired token is gone
	id, _, _, _, err = database.GetEmailVerificationToken(hashToken("expired-token"))
	if err != nil {
		t.Fatalf("GetEmailVerificationToken failed: %v", err)
	}
	if id != "" {
		t.Error("Expired verification token should be deleted after cleanup")
	}
}

func TestCleanupExpiredAuth_MagicLinkTokens(t *testing.T) {
	database := setupAuthTestDB(t)
	logger := slog.Default()

	user := createTestUser(t, database, "magic-cleanup@example.com", "password123", true)

	hashToken := func(token string) string {
		h := sha256.Sum256([]byte(token))
		return hex.EncodeToString(h[:])
	}

	// Create an expired magic link token
	if err := database.StoreMagicLinkToken(
		"expired-ml", user.ID, hashToken("expired-magic"),
		time.Now().UTC().Add(-1*time.Hour),
	); err != nil {
		t.Fatalf("StoreMagicLinkToken failed: %v", err)
	}

	// Create a valid magic link token
	if err := database.StoreMagicLinkToken(
		"valid-ml", user.ID, hashToken("valid-magic"),
		time.Now().UTC().Add(24*time.Hour),
	); err != nil {
		t.Fatalf("StoreMagicLinkToken failed: %v", err)
	}

	result := CleanupExpiredAuth(database, logger)

	if result.MagicLinkTokens != 1 {
		t.Errorf("Expected 1 expired magic link token deleted, got %d", result.MagicLinkTokens)
	}

	// Verify valid token still exists
	id, _, _, _, err := database.GetMagicLinkToken(hashToken("valid-magic"))
	if err != nil {
		t.Fatalf("GetMagicLinkToken failed: %v", err)
	}
	if id == "" {
		t.Error("Valid magic link token should still exist after cleanup")
	}

	// Verify expired token is gone
	id, _, _, _, err = database.GetMagicLinkToken(hashToken("expired-magic"))
	if err != nil {
		t.Fatalf("GetMagicLinkToken failed: %v", err)
	}
	if id != "" {
		t.Error("Expired magic link token should be deleted after cleanup")
	}
}

func TestStartAuthCleanupTicker_Stops(t *testing.T) {
	database := setupAuthTestDB(t)
	logger := slog.Default()

	stop := make(chan struct{})
	StartAuthCleanupTicker(database, logger, stop)

	// Close immediately to verify goroutine stops without hanging
	close(stop)

	// Brief pause to let goroutine exit
	time.Sleep(10 * time.Millisecond)
}
