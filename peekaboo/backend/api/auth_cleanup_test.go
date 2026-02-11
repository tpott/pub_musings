package api

import (
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

	sessionsDeleted, _ := CleanupExpiredAuth(database, logger)

	if sessionsDeleted != 1 {
		t.Errorf("Expected 1 expired session deleted, got %d", sessionsDeleted)
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

	sessionsDeleted, attemptsDeleted := CleanupExpiredAuth(database, logger)

	if sessionsDeleted != 0 {
		t.Errorf("Expected 0 sessions deleted, got %d", sessionsDeleted)
	}
	if attemptsDeleted != 0 {
		t.Errorf("Expected 0 attempts deleted, got %d", attemptsDeleted)
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
