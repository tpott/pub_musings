package db

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	if err := db.Init(); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestInitAuthCreatesAllTables(t *testing.T) {
	db := openTestDB(t)

	tables := []string{"users", "email_verification_tokens", "magic_link_tokens", "sessions", "login_attempts"}
	for _, table := range tables {
		var name string
		err := db.conn.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %s not found: %v", table, err)
		}
	}
}

func TestInitAuthCreatesIndexes(t *testing.T) {
	db := openTestDB(t)

	indexes := []string{
		"idx_users_email",
		"idx_email_verification_user_id",
		"idx_email_verification_expires",
		"idx_magic_link_user_id",
		"idx_magic_link_expires",
		"idx_sessions_token_hash",
		"idx_sessions_user_id",
		"idx_sessions_expires_at",
		"idx_login_attempts_email",
	}
	for _, idx := range indexes {
		var name string
		err := db.conn.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='index' AND name=?", idx,
		).Scan(&name)
		if err != nil {
			t.Errorf("index %s not found: %v", idx, err)
		}
	}
}

func TestInitAuthIdempotent(t *testing.T) {
	db := openTestDB(t)
	if err := db.InitAuth(); err != nil {
		t.Fatalf("second InitAuth failed: %v", err)
	}
}

func TestCreateAndGetUser(t *testing.T) {
	db := openTestDB(t)

	now := time.Now().UTC().Truncate(time.Second)
	user := &User{
		ID:            "abc123def456abc123def456abc123de",
		Email:         "test@example.com",
		PasswordHash:  "$2a$12$fakehashfakehashfakehashfakehashfakehashfakehashfakehas",
		TOTPEnabled:   false,
		EmailVerified: false,
		CreatedAt:     now,
	}

	if err := db.CreateUser(user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Get by email
	got, err := db.GetUserByEmail("test@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}
	if got == nil {
		t.Fatal("GetUserByEmail returned nil")
	}
	if got.ID != user.ID {
		t.Errorf("ID = %q, want %q", got.ID, user.ID)
	}
	if got.Email != user.Email {
		t.Errorf("Email = %q, want %q", got.Email, user.Email)
	}
	if got.TOTPEnabled {
		t.Error("TOTPEnabled should be false")
	}
	if got.EmailVerified {
		t.Error("EmailVerified should be false")
	}
	if got.TOTPSecret != nil {
		t.Errorf("TOTPSecret should be nil, got %v", got.TOTPSecret)
	}
	if got.VerifiedAt != nil {
		t.Errorf("VerifiedAt should be nil, got %v", got.VerifiedAt)
	}

	// Get by ID
	got2, err := db.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if got2 == nil {
		t.Fatal("GetUserByID returned nil")
	}
	if got2.Email != user.Email {
		t.Errorf("GetUserByID Email = %q, want %q", got2.Email, user.Email)
	}
}

func TestGetUserNotFound(t *testing.T) {
	db := openTestDB(t)

	got, err := db.GetUserByEmail("nonexistent@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}

	got2, err := db.GetUserByID("nonexistent-id")
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if got2 != nil {
		t.Errorf("expected nil, got %+v", got2)
	}
}

func TestCreateUserDuplicateEmail(t *testing.T) {
	db := openTestDB(t)

	user := &User{
		ID:           "id1",
		Email:        "dup@example.com",
		PasswordHash: "hash1",
		CreatedAt:    time.Now().UTC(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	user2 := &User{
		ID:           "id2",
		Email:        "dup@example.com",
		PasswordHash: "hash2",
		CreatedAt:    time.Now().UTC(),
	}
	err := db.CreateUser(user2)
	if err == nil {
		t.Fatal("expected error for duplicate email, got nil")
	}
}

func TestSetEmailVerified(t *testing.T) {
	db := openTestDB(t)

	user := &User{
		ID:           "verify-test-id",
		Email:        "verify@example.com",
		PasswordHash: "hash",
		CreatedAt:    time.Now().UTC(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if err := db.SetEmailVerified(user.ID); err != nil {
		t.Fatalf("SetEmailVerified failed: %v", err)
	}

	got, err := db.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if !got.EmailVerified {
		t.Error("EmailVerified should be true after SetEmailVerified")
	}
	if got.VerifiedAt == nil {
		t.Error("VerifiedAt should be set after SetEmailVerified")
	}
}

func TestSetEmailVerifiedUserNotFound(t *testing.T) {
	db := openTestDB(t)
	err := db.SetEmailVerified("nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent user, got nil")
	}
}

func TestCreateAndGetSession(t *testing.T) {
	db := openTestDB(t)

	user := &User{
		ID:           "session-user-id",
		Email:        "session@example.com",
		PasswordHash: "hash",
		CreatedAt:    time.Now().UTC(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	session := &Session{
		ID:        "session-id-1",
		UserID:    user.ID,
		TokenHash:     "session-token-abc123",
		ExpiresAt: now.Add(30 * 24 * time.Hour),
		CreatedAt: now,
	}
	if err := db.CreateSession(session); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	got, err := db.GetSessionByTokenHash(session.TokenHash)
	if err != nil {
		t.Fatalf("GetSessionByTokenHash failed: %v", err)
	}
	if got == nil {
		t.Fatal("GetSessionByTokenHash returned nil")
	}
	if got.ID != session.ID {
		t.Errorf("Session ID = %q, want %q", got.ID, session.ID)
	}
	if got.UserID != session.UserID {
		t.Errorf("Session UserID = %q, want %q", got.UserID, session.UserID)
	}
}

func TestGetSessionByTokenHashNotFound(t *testing.T) {
	db := openTestDB(t)

	got, err := db.GetSessionByTokenHash("nonexistent-token")
	if err != nil {
		t.Fatalf("GetSessionByTokenHash failed: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestDeleteSession(t *testing.T) {
	db := openTestDB(t)

	user := &User{
		ID:           "delete-session-user",
		Email:        "delsession@example.com",
		PasswordHash: "hash",
		CreatedAt:    time.Now().UTC(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	session := &Session{
		ID:        "del-session-1",
		UserID:    user.ID,
		TokenHash:     "del-token-1",
		ExpiresAt: time.Now().UTC().Add(time.Hour),
		CreatedAt: time.Now().UTC(),
	}
	if err := db.CreateSession(session); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if err := db.DeleteSession(session.ID); err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}

	got, err := db.GetSessionByTokenHash(session.TokenHash)
	if err != nil {
		t.Fatalf("GetSessionByTokenHash after delete failed: %v", err)
	}
	if got != nil {
		t.Error("session should be deleted")
	}
}

func TestDeleteExpiredSessions(t *testing.T) {
	db := openTestDB(t)

	user := &User{
		ID:           "expired-session-user",
		Email:        "expired@example.com",
		PasswordHash: "hash",
		CreatedAt:    time.Now().UTC(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	expired := &Session{
		ID:        "expired-session-1",
		UserID:    user.ID,
		TokenHash:     "expired-token-1",
		ExpiresAt: time.Now().UTC().Add(-time.Hour),
		CreatedAt: time.Now().UTC().Add(-2 * time.Hour),
	}
	if err := db.CreateSession(expired); err != nil {
		t.Fatalf("CreateSession (expired) failed: %v", err)
	}

	valid := &Session{
		ID:        "valid-session-1",
		UserID:    user.ID,
		TokenHash:     "valid-token-1",
		ExpiresAt: time.Now().UTC().Add(time.Hour),
		CreatedAt: time.Now().UTC(),
	}
	if err := db.CreateSession(valid); err != nil {
		t.Fatalf("CreateSession (valid) failed: %v", err)
	}

	deleted, err := db.DeleteExpiredSessions()
	if err != nil {
		t.Fatalf("DeleteExpiredSessions failed: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}

	got, _ := db.GetSessionByTokenHash("expired-token-1")
	if got != nil {
		t.Error("expired session should be deleted")
	}

	got2, _ := db.GetSessionByTokenHash("valid-token-1")
	if got2 == nil {
		t.Error("valid session should still exist")
	}
}

func TestDeleteSessionsByUserID(t *testing.T) {
	db := openTestDB(t)

	user := &User{
		ID:           "multi-session-user",
		Email:        "multi@example.com",
		PasswordHash: "hash",
		CreatedAt:    time.Now().UTC(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	for i := 0; i < 3; i++ {
		s := &Session{
			ID:        fmt.Sprintf("ms-%d", i),
			UserID:    user.ID,
			TokenHash:     fmt.Sprintf("ms-token-%d", i),
			ExpiresAt: time.Now().UTC().Add(time.Hour),
			CreatedAt: time.Now().UTC(),
		}
		if err := db.CreateSession(s); err != nil {
			t.Fatalf("CreateSession %d failed: %v", i, err)
		}
	}

	if err := db.DeleteSessionsByUserID(user.ID); err != nil {
		t.Fatalf("DeleteSessionsByUserID failed: %v", err)
	}

	for i := 0; i < 3; i++ {
		got, _ := db.GetSessionByTokenHash(fmt.Sprintf("ms-token-%d", i))
		if got != nil {
			t.Errorf("session %d should be deleted", i)
		}
	}
}
