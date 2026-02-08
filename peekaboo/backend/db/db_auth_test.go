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
		"idx_sessions_token",
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
		Token:     "session-token-abc123",
		ExpiresAt: now.Add(30 * 24 * time.Hour),
		CreatedAt: now,
	}
	if err := db.CreateSession(session); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	got, err := db.GetSessionByToken(session.Token)
	if err != nil {
		t.Fatalf("GetSessionByToken failed: %v", err)
	}
	if got == nil {
		t.Fatal("GetSessionByToken returned nil")
	}
	if got.ID != session.ID {
		t.Errorf("Session ID = %q, want %q", got.ID, session.ID)
	}
	if got.UserID != session.UserID {
		t.Errorf("Session UserID = %q, want %q", got.UserID, session.UserID)
	}
}

func TestGetSessionByTokenNotFound(t *testing.T) {
	db := openTestDB(t)

	got, err := db.GetSessionByToken("nonexistent-token")
	if err != nil {
		t.Fatalf("GetSessionByToken failed: %v", err)
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
		Token:     "del-token-1",
		ExpiresAt: time.Now().UTC().Add(time.Hour),
		CreatedAt: time.Now().UTC(),
	}
	if err := db.CreateSession(session); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if err := db.DeleteSession(session.ID); err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}

	got, err := db.GetSessionByToken(session.Token)
	if err != nil {
		t.Fatalf("GetSessionByToken after delete failed: %v", err)
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
		Token:     "expired-token-1",
		ExpiresAt: time.Now().UTC().Add(-time.Hour),
		CreatedAt: time.Now().UTC().Add(-2 * time.Hour),
	}
	if err := db.CreateSession(expired); err != nil {
		t.Fatalf("CreateSession (expired) failed: %v", err)
	}

	valid := &Session{
		ID:        "valid-session-1",
		UserID:    user.ID,
		Token:     "valid-token-1",
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

	got, _ := db.GetSessionByToken("expired-token-1")
	if got != nil {
		t.Error("expired session should be deleted")
	}

	got2, _ := db.GetSessionByToken("valid-token-1")
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
			Token:     fmt.Sprintf("ms-token-%d", i),
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
		got, _ := db.GetSessionByToken(fmt.Sprintf("ms-token-%d", i))
		if got != nil {
			t.Errorf("session %d should be deleted", i)
		}
	}
}

func TestUserWithTOTPSecret(t *testing.T) {
	db := openTestDB(t)

	totpSecret := "JBSWY3DPEHPK3PXP"
	user := &User{
		ID:           "totp-user-id",
		Email:        "totp@example.com",
		PasswordHash: "hash",
		TOTPSecret:   &totpSecret,
		TOTPEnabled:  true,
		CreatedAt:    time.Now().UTC(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	got, err := db.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if got.TOTPSecret == nil {
		t.Fatal("TOTPSecret should not be nil")
	}
	if *got.TOTPSecret != totpSecret {
		t.Errorf("TOTPSecret = %q, want %q", *got.TOTPSecret, totpSecret)
	}
	if !got.TOTPEnabled {
		t.Error("TOTPEnabled should be true")
	}
}

func TestSetTOTPSecret(t *testing.T) {
	db := openTestDB(t)

	user := &User{
		ID:           "set-totp-user",
		Email:        "set-totp@example.com",
		PasswordHash: "hash",
		CreatedAt:    time.Now().UTC(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Initially no TOTP secret
	got, _ := db.GetUserByID(user.ID)
	if got.TOTPSecret != nil {
		t.Fatal("TOTPSecret should initially be nil")
	}

	// Set secret
	if err := db.SetTOTPSecret(user.ID, "NEWSECRET"); err != nil {
		t.Fatalf("SetTOTPSecret failed: %v", err)
	}

	got, _ = db.GetUserByID(user.ID)
	if got.TOTPSecret == nil || *got.TOTPSecret != "NEWSECRET" {
		t.Errorf("TOTPSecret = %v, want NEWSECRET", got.TOTPSecret)
	}
	if got.TOTPEnabled {
		t.Error("TOTPEnabled should still be false after SetTOTPSecret")
	}
}

func TestSetTOTPSecret_NotFound(t *testing.T) {
	db := openTestDB(t)

	err := db.SetTOTPSecret("nonexistent", "SECRET")
	if err == nil {
		t.Error("SetTOTPSecret should fail for nonexistent user")
	}
}

func TestEnableTOTP(t *testing.T) {
	db := openTestDB(t)

	user := &User{
		ID:           "enable-totp-user",
		Email:        "enable-totp@example.com",
		PasswordHash: "hash",
		CreatedAt:    time.Now().UTC(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Enable without secret should fail
	if err := db.EnableTOTP(user.ID); err == nil {
		t.Error("EnableTOTP should fail when no secret is set")
	}

	// Set secret then enable
	if err := db.SetTOTPSecret(user.ID, "TESTSECRET"); err != nil {
		t.Fatalf("SetTOTPSecret failed: %v", err)
	}
	if err := db.EnableTOTP(user.ID); err != nil {
		t.Fatalf("EnableTOTP failed: %v", err)
	}

	got, _ := db.GetUserByID(user.ID)
	if !got.TOTPEnabled {
		t.Error("TOTPEnabled should be true after EnableTOTP")
	}
}

func TestDisableTOTP(t *testing.T) {
	db := openTestDB(t)

	secret := "DISABLETEST"
	user := &User{
		ID:           "disable-totp-user",
		Email:        "disable-totp@example.com",
		PasswordHash: "hash",
		TOTPSecret:   &secret,
		TOTPEnabled:  true,
		CreatedAt:    time.Now().UTC(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if err := db.DisableTOTP(user.ID); err != nil {
		t.Fatalf("DisableTOTP failed: %v", err)
	}

	got, _ := db.GetUserByID(user.ID)
	if got.TOTPEnabled {
		t.Error("TOTPEnabled should be false after DisableTOTP")
	}
	if got.TOTPSecret != nil {
		t.Error("TOTPSecret should be nil after DisableTOTP")
	}
}

func TestDisableTOTP_NotFound(t *testing.T) {
	db := openTestDB(t)

	err := db.DisableTOTP("nonexistent")
	if err == nil {
		t.Error("DisableTOTP should fail for nonexistent user")
	}
}
