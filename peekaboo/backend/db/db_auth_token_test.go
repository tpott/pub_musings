package db

import (
	"testing"
	"time"
)

func TestEmailVerificationTokenRoundtrip(t *testing.T) {
	db := openTestDB(t)

	user := &User{
		ID:           "evtoken-user",
		Email:        "evtoken@example.com",
		PasswordHash: "hash",
		CreatedAt:    time.Now().UTC(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	tokenHash := "sha256hashoftoken"
	expiresAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)

	if err := db.StoreEmailVerificationToken("evtoken-1", user.ID, tokenHash, expiresAt); err != nil {
		t.Fatalf("StoreEmailVerificationToken failed: %v", err)
	}

	id, userID, exp, used, err := db.GetEmailVerificationToken(tokenHash)
	if err != nil {
		t.Fatalf("GetEmailVerificationToken failed: %v", err)
	}
	if id != "evtoken-1" {
		t.Errorf("id = %q, want %q", id, "evtoken-1")
	}
	if userID != user.ID {
		t.Errorf("userID = %q, want %q", userID, user.ID)
	}
	if used {
		t.Error("token should not be used yet")
	}
	if exp.IsZero() {
		t.Error("expires_at should not be zero")
	}
}

func TestEmailVerificationTokenNotFound(t *testing.T) {
	db := openTestDB(t)

	id, userID, _, _, err := db.GetEmailVerificationToken("nonexistent-hash")
	if err != nil {
		t.Fatalf("GetEmailVerificationToken failed: %v", err)
	}
	if id != "" || userID != "" {
		t.Errorf("expected empty strings for nonexistent token, got id=%q userID=%q", id, userID)
	}
}

func TestMarkEmailVerificationTokenUsed(t *testing.T) {
	db := openTestDB(t)

	user := &User{
		ID:           "mark-used-user",
		Email:        "markused@example.com",
		PasswordHash: "hash",
		CreatedAt:    time.Now().UTC(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	tokenHash := "token-to-mark-used"
	if err := db.StoreEmailVerificationToken("mark-1", user.ID, tokenHash, time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	if err := db.MarkEmailVerificationTokenUsed("mark-1"); err != nil {
		t.Fatalf("MarkEmailVerificationTokenUsed failed: %v", err)
	}

	_, _, _, used, err := db.GetEmailVerificationToken(tokenHash)
	if err != nil {
		t.Fatalf("GetEmailVerificationToken failed: %v", err)
	}
	if !used {
		t.Error("token should be marked as used")
	}

	// Second call should return ErrTokenAlreadyUsed (TOCTOU prevention)
	err = db.MarkEmailVerificationTokenUsed("mark-1")
	if err != ErrTokenAlreadyUsed {
		t.Errorf("second MarkEmailVerificationTokenUsed: expected ErrTokenAlreadyUsed, got %v", err)
	}
}

func TestDeleteUnusedEmailVerificationTokens(t *testing.T) {
	db := openTestDB(t)

	user := &User{
		ID:           "del-unused-user",
		Email:        "delunused@example.com",
		PasswordHash: "hash",
		CreatedAt:    time.Now().UTC(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if err := db.StoreEmailVerificationToken("unused-1", user.ID, "hash-unused", time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatalf("Store failed: %v", err)
	}
	if err := db.StoreEmailVerificationToken("used-1", user.ID, "hash-used", time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatalf("Store failed: %v", err)
	}
	if err := db.MarkEmailVerificationTokenUsed("used-1"); err != nil {
		t.Fatalf("MarkUsed failed: %v", err)
	}

	if err := db.DeleteUnusedEmailVerificationTokens(user.ID); err != nil {
		t.Fatalf("DeleteUnused failed: %v", err)
	}

	id, _, _, _, _ := db.GetEmailVerificationToken("hash-unused")
	if id != "" {
		t.Error("unused token should be deleted")
	}

	id2, _, _, _, _ := db.GetEmailVerificationToken("hash-used")
	if id2 == "" {
		t.Error("used token should still exist")
	}
}

func TestMagicLinkTokenRoundtrip(t *testing.T) {
	db := openTestDB(t)

	user := &User{
		ID:           "mltoken-user",
		Email:        "mltoken@example.com",
		PasswordHash: "hash",
		CreatedAt:    time.Now().UTC(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	tokenHash := "ml-sha256hash"
	expiresAt := time.Now().UTC().Add(15 * time.Minute).Truncate(time.Second)

	if err := db.StoreMagicLinkToken("mltoken-1", user.ID, tokenHash, expiresAt); err != nil {
		t.Fatalf("StoreMagicLinkToken failed: %v", err)
	}

	id, userID, _, used, err := db.GetMagicLinkToken(tokenHash)
	if err != nil {
		t.Fatalf("GetMagicLinkToken failed: %v", err)
	}
	if id != "mltoken-1" {
		t.Errorf("id = %q, want %q", id, "mltoken-1")
	}
	if userID != user.ID {
		t.Errorf("userID = %q, want %q", userID, user.ID)
	}
	if used {
		t.Error("token should not be used yet")
	}
}

func TestMagicLinkTokenNotFound(t *testing.T) {
	db := openTestDB(t)

	id, userID, _, _, err := db.GetMagicLinkToken("nonexistent-hash")
	if err != nil {
		t.Fatalf("GetMagicLinkToken failed: %v", err)
	}
	if id != "" || userID != "" {
		t.Errorf("expected empty strings, got id=%q userID=%q", id, userID)
	}
}

func TestMarkMagicLinkTokenUsed(t *testing.T) {
	db := openTestDB(t)

	user := &User{
		ID:           "ml-mark-user",
		Email:        "mlmark@example.com",
		PasswordHash: "hash",
		CreatedAt:    time.Now().UTC(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if err := db.StoreMagicLinkToken("ml-mark-1", user.ID, "ml-hash-mark", time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	if err := db.MarkMagicLinkTokenUsed("ml-mark-1"); err != nil {
		t.Fatalf("MarkMagicLinkTokenUsed failed: %v", err)
	}

	_, _, _, used, err := db.GetMagicLinkToken("ml-hash-mark")
	if err != nil {
		t.Fatalf("GetMagicLinkToken failed: %v", err)
	}
	if !used {
		t.Error("token should be marked as used")
	}

	// Second call should return ErrTokenAlreadyUsed (TOCTOU prevention)
	err = db.MarkMagicLinkTokenUsed("ml-mark-1")
	if err != ErrTokenAlreadyUsed {
		t.Errorf("second MarkMagicLinkTokenUsed: expected ErrTokenAlreadyUsed, got %v", err)
	}
}

func TestDeleteUnusedMagicLinkTokens(t *testing.T) {
	db := openTestDB(t)

	user := &User{
		ID:           "ml-del-user",
		Email:        "mldel@example.com",
		PasswordHash: "hash",
		CreatedAt:    time.Now().UTC(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if err := db.StoreMagicLinkToken("ml-unused-1", user.ID, "ml-hash-unused", time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatalf("Store failed: %v", err)
	}
	if err := db.StoreMagicLinkToken("ml-used-1", user.ID, "ml-hash-used", time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatalf("Store failed: %v", err)
	}
	if err := db.MarkMagicLinkTokenUsed("ml-used-1"); err != nil {
		t.Fatalf("MarkUsed failed: %v", err)
	}

	if err := db.DeleteUnusedMagicLinkTokens(user.ID); err != nil {
		t.Fatalf("DeleteUnused failed: %v", err)
	}

	id, _, _, _, _ := db.GetMagicLinkToken("ml-hash-unused")
	if id != "" {
		t.Error("unused magic link token should be deleted")
	}

	id2, _, _, _, _ := db.GetMagicLinkToken("ml-hash-used")
	if id2 == "" {
		t.Error("used magic link token should still exist")
	}
}

func TestLoginAttempts(t *testing.T) {
	db := openTestDB(t)

	email := "lockout@example.com"
	ip := "192.168.1.1"

	for i := 0; i < 3; i++ {
		if err := db.RecordLoginAttempt(email, ip, false); err != nil {
			t.Fatalf("RecordLoginAttempt failed: %v", err)
		}
	}

	since := time.Now().UTC().Add(-15 * time.Minute)
	count, err := db.CountRecentFailedAttempts(email, since)
	if err != nil {
		t.Fatalf("CountRecentFailedAttempts failed: %v", err)
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}

	// Success should not affect failed count
	if err := db.RecordLoginAttempt(email, ip, true); err != nil {
		t.Fatalf("RecordLoginAttempt (success) failed: %v", err)
	}

	count2, err := db.CountRecentFailedAttempts(email, since)
	if err != nil {
		t.Fatalf("CountRecentFailedAttempts failed: %v", err)
	}
	if count2 != 3 {
		t.Errorf("count after success = %d, want 3", count2)
	}
}

func TestClearLoginAttempts(t *testing.T) {
	db := openTestDB(t)

	email := "clear@example.com"
	for i := 0; i < 5; i++ {
		if err := db.RecordLoginAttempt(email, "1.2.3.4", false); err != nil {
			t.Fatalf("RecordLoginAttempt failed: %v", err)
		}
	}
	// Also record a success — ClearLoginAttempts should preserve it
	if err := db.RecordLoginAttempt(email, "1.2.3.4", true); err != nil {
		t.Fatalf("RecordLoginAttempt (success) failed: %v", err)
	}

	if err := db.ClearLoginAttempts(email); err != nil {
		t.Fatalf("ClearLoginAttempts failed: %v", err)
	}

	since := time.Now().UTC().Add(-time.Hour)
	count, err := db.CountRecentFailedAttempts(email, since)
	if err != nil {
		t.Fatalf("CountRecentFailedAttempts failed: %v", err)
	}
	if count != 0 {
		t.Errorf("count after clear = %d, want 0", count)
	}

	// Success records should still exist (verify via raw query)
	var successCount int
	err = db.conn.QueryRow(
		"SELECT COUNT(*) FROM login_attempts WHERE email = ? AND success = 1", email,
	).Scan(&successCount)
	if err != nil {
		t.Fatalf("count success records: %v", err)
	}
	if successCount != 1 {
		t.Errorf("expected 1 success record preserved, got %d", successCount)
	}
}

func TestDeleteExpiredLoginAttempts(t *testing.T) {
	db := openTestDB(t)

	email := "expiry@example.com"
	for i := 0; i < 3; i++ {
		if err := db.RecordLoginAttempt(email, "1.2.3.4", false); err != nil {
			t.Fatalf("RecordLoginAttempt failed: %v", err)
		}
	}

	deleted, err := db.DeleteExpiredLoginAttempts(time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatalf("DeleteExpiredLoginAttempts failed: %v", err)
	}
	if deleted != 3 {
		t.Errorf("deleted = %d, want 3", deleted)
	}
}

func TestVerifyEmailWithToken(t *testing.T) {
	db := openTestDB(t)

	user := &User{
		ID:           "verify-tx-user",
		Email:        "verifytx@example.com",
		PasswordHash: "hash",
		CreatedAt:    time.Now().UTC(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	tokenHash := "verify-tx-hash"
	tokenID := "verify-tx-1"
	if err := db.StoreEmailVerificationToken(tokenID, user.ID, tokenHash, time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Verify atomically marks token + sets email verified
	if err := db.VerifyEmailWithToken(tokenID, user.ID); err != nil {
		t.Fatalf("VerifyEmailWithToken failed: %v", err)
	}

	// Token should be marked used
	_, _, _, used, err := db.GetEmailVerificationToken(tokenHash)
	if err != nil {
		t.Fatalf("GetEmailVerificationToken failed: %v", err)
	}
	if !used {
		t.Error("token should be marked as used")
	}

	// User should be verified
	u, err := db.GetUserByEmail(user.Email)
	if err != nil {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}
	if !u.EmailVerified {
		t.Error("user email should be verified")
	}

	// Second call should return ErrTokenAlreadyUsed
	err = db.VerifyEmailWithToken(tokenID, user.ID)
	if err != ErrTokenAlreadyUsed {
		t.Errorf("second VerifyEmailWithToken: expected ErrTokenAlreadyUsed, got %v", err)
	}
}

func TestRedeemMagicLinkToken(t *testing.T) {
	db := openTestDB(t)

	user := &User{
		ID:           "redeem-tx-user",
		Email:        "redeemtx@example.com",
		PasswordHash: "hash",
		CreatedAt:    time.Now().UTC(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	tokenID := "redeem-tx-1"
	if err := db.StoreMagicLinkToken(tokenID, user.ID, "redeem-tx-hash", time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	now := time.Now().UTC()
	session := &Session{
		ID:        "redeem-sess-1",
		UserID:    user.ID,
		TokenHash: "redeem-session-hash",
		ExpiresAt: now.Add(24 * time.Hour),
		CreatedAt: now,
	}

	// Redeem atomically marks token + creates session
	if err := db.RedeemMagicLinkToken(tokenID, session); err != nil {
		t.Fatalf("RedeemMagicLinkToken failed: %v", err)
	}

	// Token should be marked used
	_, _, _, used, err := db.GetMagicLinkToken("redeem-tx-hash")
	if err != nil {
		t.Fatalf("GetMagicLinkToken failed: %v", err)
	}
	if !used {
		t.Error("token should be marked as used")
	}

	// Session should exist
	s, err := db.GetSessionByTokenHash("redeem-session-hash")
	if err != nil {
		t.Fatalf("GetSessionByTokenHash failed: %v", err)
	}
	if s == nil {
		t.Fatal("session should exist after redeem")
	}
	if s.UserID != user.ID {
		t.Errorf("session user_id = %q, want %q", s.UserID, user.ID)
	}

	// Second call should return ErrTokenAlreadyUsed
	err = db.RedeemMagicLinkToken(tokenID, session)
	if err != ErrTokenAlreadyUsed {
		t.Errorf("second RedeemMagicLinkToken: expected ErrTokenAlreadyUsed, got %v", err)
	}
}

func TestDeleteExpiredEmailVerificationTokens(t *testing.T) {
	db := openTestDB(t)

	user := &User{
		ID:           "evcleanup-user",
		Email:        "evcleanup@example.com",
		PasswordHash: "hash",
		CreatedAt:    time.Now().UTC(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Store one expired token and one fresh token
	expired := time.Now().UTC().Add(-1 * time.Hour)
	fresh := time.Now().UTC().Add(24 * time.Hour)

	if err := db.StoreEmailVerificationToken("ev-expired", user.ID, "hash-expired", expired); err != nil {
		t.Fatalf("StoreEmailVerificationToken(expired) failed: %v", err)
	}
	if err := db.StoreEmailVerificationToken("ev-fresh", user.ID, "hash-fresh", fresh); err != nil {
		t.Fatalf("StoreEmailVerificationToken(fresh) failed: %v", err)
	}

	deleted, err := db.DeleteExpiredEmailVerificationTokens()
	if err != nil {
		t.Fatalf("DeleteExpiredEmailVerificationTokens failed: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}

	// Fresh token should still exist
	_, _, _, _, err = db.GetEmailVerificationToken("hash-fresh")
	if err != nil {
		t.Errorf("fresh token should still exist: %v", err)
	}

	// Expired token should be gone
	id, _, _, _, err := db.GetEmailVerificationToken("hash-expired")
	if err != nil {
		t.Fatalf("GetEmailVerificationToken(expired) error: %v", err)
	}
	if id != "" {
		t.Error("expired token should have been deleted")
	}
}

func TestDeleteExpiredMagicLinkTokens(t *testing.T) {
	db := openTestDB(t)

	user := &User{
		ID:           "mlcleanup-user",
		Email:        "mlcleanup@example.com",
		PasswordHash: "hash",
		CreatedAt:    time.Now().UTC(),
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Store one expired token and one fresh token
	expired := time.Now().UTC().Add(-1 * time.Hour)
	fresh := time.Now().UTC().Add(24 * time.Hour)

	if err := db.StoreMagicLinkToken("ml-expired", user.ID, "ml-hash-expired", expired); err != nil {
		t.Fatalf("StoreMagicLinkToken(expired) failed: %v", err)
	}
	if err := db.StoreMagicLinkToken("ml-fresh", user.ID, "ml-hash-fresh", fresh); err != nil {
		t.Fatalf("StoreMagicLinkToken(fresh) failed: %v", err)
	}

	deleted, err := db.DeleteExpiredMagicLinkTokens()
	if err != nil {
		t.Fatalf("DeleteExpiredMagicLinkTokens failed: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}

	// Fresh token should still exist
	_, _, _, _, err = db.GetMagicLinkToken("ml-hash-fresh")
	if err != nil {
		t.Errorf("fresh token should still exist: %v", err)
	}

	// Expired token should be gone
	id, _, _, _, err := db.GetMagicLinkToken("ml-hash-expired")
	if err != nil {
		t.Fatalf("GetMagicLinkToken(expired) error: %v", err)
	}
	if id != "" {
		t.Error("expired token should have been deleted")
	}
}
