package api

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/tpott/pub_musings/peekaboo/backend/auth"
	"github.com/tpott/pub_musings/peekaboo/backend/db"
)

// --- WSAuthTracker unit tests ---

func TestWSAuthTracker_AnonConcurrentLimit(t *testing.T) {
	tracker := NewWSAuthTracker(DefaultWSAuthLimits())

	// First connection should succeed
	if !tracker.TryAcquireAnon("192.168.1.1") {
		t.Fatal("first anon connection should be allowed")
	}

	// Second connection from same IP should fail (limit is 1)
	if tracker.TryAcquireAnon("192.168.1.1") {
		t.Fatal("second anon connection from same IP should be rejected")
	}

	// Connection from different IP should succeed
	if !tracker.TryAcquireAnon("192.168.1.2") {
		t.Fatal("connection from different IP should be allowed")
	}

	// Release first connection, should allow new one
	tracker.ReleaseAnon("192.168.1.1")
	if !tracker.TryAcquireAnon("192.168.1.1") {
		t.Fatal("connection after release should be allowed")
	}
}

func TestWSAuthTracker_UserConcurrentLimit(t *testing.T) {
	tracker := NewWSAuthTracker(DefaultWSAuthLimits())

	// Open 3 connections for user (limit is 3)
	for i := 0; i < 3; i++ {
		if !tracker.TryAcquireUser("user-1") {
			t.Fatalf("connection %d should be allowed", i+1)
		}
	}

	// 4th connection should fail
	if tracker.TryAcquireUser("user-1") {
		t.Fatal("4th user connection should be rejected")
	}

	// Different user should succeed
	if !tracker.TryAcquireUser("user-2") {
		t.Fatal("different user should be allowed")
	}

	// Release one, should allow new one
	tracker.ReleaseUser("user-1")
	if !tracker.TryAcquireUser("user-1") {
		t.Fatal("connection after release should be allowed")
	}
}

func TestWSAuthTracker_AnonInteractionLimit(t *testing.T) {
	limits := WSAuthLimits{
		AnonMaxConcurrent:       1,
		AnonMaxInteractionsHr:   3, // Low limit for testing
		RegisteredMaxConcurrent: 3,
		InteractionWindow:       time.Hour,
	}
	tracker := NewWSAuthTracker(limits)

	ip := "10.0.0.1"

	// First 3 interactions should succeed
	for i := 0; i < 3; i++ {
		if !tracker.AllowAnonInteraction(ip) {
			t.Fatalf("interaction %d should be allowed", i+1)
		}
	}

	// 4th interaction should be rate limited
	if tracker.AllowAnonInteraction(ip) {
		t.Fatal("4th interaction should be rate limited")
	}

	// Different IP should still work
	if !tracker.AllowAnonInteraction("10.0.0.2") {
		t.Fatal("different IP should be allowed")
	}
}

func TestWSAuthTracker_AnonInteractionPrunesOld(t *testing.T) {
	limits := WSAuthLimits{
		AnonMaxConcurrent:       1,
		AnonMaxInteractionsHr:   2,
		RegisteredMaxConcurrent: 3,
		InteractionWindow:       100 * time.Millisecond, // Very short window for testing
	}
	tracker := NewWSAuthTracker(limits)

	ip := "10.0.0.1"

	// Use up the quota
	tracker.AllowAnonInteraction(ip)
	tracker.AllowAnonInteraction(ip)

	// Should be rate limited
	if tracker.AllowAnonInteraction(ip) {
		t.Fatal("should be rate limited")
	}

	// Wait for window to expire
	time.Sleep(150 * time.Millisecond)

	// Should be allowed again after window expires
	if !tracker.AllowAnonInteraction(ip) {
		t.Fatal("should be allowed after window expires")
	}
}

func TestWSAuthTracker_ConnCountTracking(t *testing.T) {
	tracker := NewWSAuthTracker(DefaultWSAuthLimits())

	if tracker.AnonConnCount("10.0.0.1") != 0 {
		t.Error("initial anon count should be 0")
	}
	if tracker.UserConnCount("user-1") != 0 {
		t.Error("initial user count should be 0")
	}

	tracker.TryAcquireAnon("10.0.0.1")
	if tracker.AnonConnCount("10.0.0.1") != 1 {
		t.Error("anon count should be 1 after acquire")
	}

	tracker.TryAcquireUser("user-1")
	tracker.TryAcquireUser("user-1")
	if tracker.UserConnCount("user-1") != 2 {
		t.Error("user count should be 2 after two acquires")
	}

	tracker.ReleaseAnon("10.0.0.1")
	if tracker.AnonConnCount("10.0.0.1") != 0 {
		t.Error("anon count should be 0 after release")
	}
}

func TestWSAuthTracker_CleanupStaleEntries(t *testing.T) {
	limits := WSAuthLimits{
		AnonMaxConcurrent:       1,
		AnonMaxInteractionsHr:   10,
		RegisteredMaxConcurrent: 3,
		InteractionWindow:       100 * time.Millisecond,
	}
	tracker := NewWSAuthTracker(limits)

	// Add interactions for two IPs
	tracker.AllowAnonInteraction("10.0.0.1")
	tracker.AllowAnonInteraction("10.0.0.2")

	// Immediately cleanup — entries are recent, nothing should be removed
	removed := tracker.CleanupStaleEntries()
	if removed != 0 {
		t.Errorf("Expected 0 removed (entries are recent), got %d", removed)
	}

	// Wait for window to expire
	time.Sleep(150 * time.Millisecond)

	// Add a fresh interaction for one IP
	tracker.AllowAnonInteraction("10.0.0.1")

	// Cleanup should remove only the stale IP
	removed = tracker.CleanupStaleEntries()
	if removed != 1 {
		t.Errorf("Expected 1 stale entry removed, got %d", removed)
	}

	// Verify 10.0.0.1 still has an entry (fresh interaction)
	tracker.mu.Lock()
	_, has1 := tracker.anonInteractions["10.0.0.1"]
	_, has2 := tracker.anonInteractions["10.0.0.2"]
	tracker.mu.Unlock()

	if !has1 {
		t.Error("10.0.0.1 should still have an entry (had recent interaction)")
	}
	if has2 {
		t.Error("10.0.0.2 should have been removed (all interactions expired)")
	}
}

// --- Helper functions for WebSocket auth integration tests ---

func setupWSAuthTestDB(t *testing.T) *db.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	if err := database.Init(); err != nil {
		t.Fatalf("Failed to init database: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func createWSTestUser(t *testing.T, database *db.DB) (*db.User, string) {
	t.Helper()
	hash, err := auth.HashPassword("testpassword")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	id, err := auth.GenerateID()
	if err != nil {
		t.Fatalf("GenerateID failed: %v", err)
	}
	now := time.Now().UTC()
	user := &db.User{
		ID:            id,
		Email:         "wstest@example.com",
		PasswordHash:  hash,
		EmailVerified: true,
		VerifiedAt:    &now,
		CreatedAt:     now,
	}
	if err := database.CreateUser(user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	sessionToken, err := auth.GenerateToken(32)
	if err != nil {
		t.Fatalf("GenerateSessionToken failed: %v", err)
	}
	sessionID, err := auth.GenerateID()
	if err != nil {
		t.Fatalf("GenerateID failed: %v", err)
	}
	session := &db.Session{
		ID:        sessionID,
		UserID:    user.ID,
		Token:     sessionToken,
		ExpiresAt: now.Add(24 * time.Hour),
		CreatedAt: now,
	}
	if err := database.CreateSession(session); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	return user, sessionToken
}
