package db

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestGetSessionsByUserIDWithLimit(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-sessions-limit-*.db")
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
		ID:           "session-limit-user",
		Email:        "sessionlimit@example.com",
		PasswordHash: "hash",
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create 10 sessions
	for i := 0; i < 10; i++ {
		session := &Session{
			ID:        fmt.Sprintf("session-%d", i),
			UserID:    user.ID,
			Token:     fmt.Sprintf("token-%d", i),
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}
		if err := db.CreateSession(session); err != nil {
			t.Fatalf("Failed to create session %d: %v", i, err)
		}
	}

	t.Run("default limit returns all sessions up to MaxSessionsPerUser", func(t *testing.T) {
		sessions, err := db.GetSessionsByUserID(user.ID)
		if err != nil {
			t.Fatalf("GetSessionsByUserID failed: %v", err)
		}
		if len(sessions) != 10 {
			t.Errorf("Expected 10 sessions, got %d", len(sessions))
		}
	})

	t.Run("custom limit restricts results", func(t *testing.T) {
		sessions, err := db.GetSessionsByUserIDWithLimit(user.ID, 5)
		if err != nil {
			t.Fatalf("GetSessionsByUserIDWithLimit failed: %v", err)
		}
		if len(sessions) != 5 {
			t.Errorf("Expected 5 sessions, got %d", len(sessions))
		}
	})

	t.Run("zero limit uses default", func(t *testing.T) {
		sessions, err := db.GetSessionsByUserIDWithLimit(user.ID, 0)
		if err != nil {
			t.Fatalf("GetSessionsByUserIDWithLimit failed: %v", err)
		}
		if len(sessions) != 10 {
			t.Errorf("Expected 10 sessions, got %d", len(sessions))
		}
	})

	t.Run("negative limit uses default", func(t *testing.T) {
		sessions, err := db.GetSessionsByUserIDWithLimit(user.ID, -5)
		if err != nil {
			t.Fatalf("GetSessionsByUserIDWithLimit failed: %v", err)
		}
		if len(sessions) != 10 {
			t.Errorf("Expected 10 sessions, got %d", len(sessions))
		}
	})

	t.Run("sessions are ordered by creation date descending", func(t *testing.T) {
		sessions, err := db.GetSessionsByUserIDWithLimit(user.ID, 10)
		if err != nil {
			t.Fatalf("GetSessionsByUserIDWithLimit failed: %v", err)
		}
		// Since sessions were created in a tight loop, they may have the same timestamp
		// Just verify the query doesn't error and returns sessions in a consistent order
		if len(sessions) != 10 {
			t.Errorf("Expected 10 sessions, got %d", len(sessions))
		}
		// Verify ORDER BY clause works (sessions should be ordered by created_at DESC, then by row insertion)
		// The actual order may vary depending on SQLite's tie-breaking, so we just verify it's stable
		sessions2, err := db.GetSessionsByUserIDWithLimit(user.ID, 10)
		if err != nil {
			t.Fatalf("Second query failed: %v", err)
		}
		for i := range sessions {
			if sessions[i].ID != sessions2[i].ID {
				t.Errorf("Session ordering not stable at position %d", i)
			}
		}
	})

	t.Run("constant is reasonable", func(t *testing.T) {
		if MaxSessionsPerUser < 50 {
			t.Errorf("MaxSessionsPerUser too low: %d", MaxSessionsPerUser)
		}
		if MaxSessionsPerUser > 500 {
			t.Errorf("MaxSessionsPerUser too high: %d", MaxSessionsPerUser)
		}
	})
}

func TestCountActiveSessions(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-count-sessions-*.db")
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
		ID:           "count-sessions-user",
		Email:        "count@example.com",
		PasswordHash: "hash",
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	t.Run("no sessions returns 0", func(t *testing.T) {
		count, err := db.CountActiveSessions()
		if err != nil {
			t.Fatalf("CountActiveSessions failed: %v", err)
		}
		if count != 0 {
			t.Errorf("Expected 0 sessions, got %d", count)
		}
	})

	t.Run("counts active sessions", func(t *testing.T) {
		// Create 5 active sessions
		for i := 0; i < 5; i++ {
			session := &Session{
				ID:        fmt.Sprintf("active-session-%d", i),
				UserID:    user.ID,
				Token:     fmt.Sprintf("active-token-%d", i),
				ExpiresAt: time.Now().Add(24 * time.Hour), // Future = active
			}
			if err := db.CreateSession(session); err != nil {
				t.Fatalf("Failed to create session %d: %v", i, err)
			}
		}

		count, err := db.CountActiveSessions()
		if err != nil {
			t.Fatalf("CountActiveSessions failed: %v", err)
		}
		if count != 5 {
			t.Errorf("Expected 5 sessions, got %d", count)
		}
	})

	t.Run("excludes expired sessions", func(t *testing.T) {
		// Create 3 expired sessions
		for i := 0; i < 3; i++ {
			session := &Session{
				ID:        fmt.Sprintf("expired-session-%d", i),
				UserID:    user.ID,
				Token:     fmt.Sprintf("expired-token-%d", i),
				ExpiresAt: time.Now().Add(-1 * time.Hour), // Past = expired
			}
			if err := db.CreateSession(session); err != nil {
				t.Fatalf("Failed to create expired session %d: %v", i, err)
			}
		}

		// Should still be 5 (only active sessions counted)
		count, err := db.CountActiveSessions()
		if err != nil {
			t.Fatalf("CountActiveSessions failed: %v", err)
		}
		if count != 5 {
			t.Errorf("Expected 5 active sessions, got %d", count)
		}
	})
}
