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
