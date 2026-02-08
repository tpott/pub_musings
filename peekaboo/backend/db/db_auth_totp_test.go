package db

import (
	"testing"
	"time"
)

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
