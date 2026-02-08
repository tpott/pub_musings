package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tpott/pub_musings/peekaboo/backend/auth"
	"github.com/tpott/pub_musings/peekaboo/backend/db"
)

// --- Login tests ---

func TestLogin_Success(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	createTestUser(t, database, "login@example.com", "correctpassword", true)

	body, _ := json.Marshal(loginRequest{
		Email:    "login@example.com",
		Password: "correctpassword",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	handler.HandleLogin(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp loginResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.User == nil {
		t.Fatal("Expected user in response")
	}
	if resp.User.Email != "login@example.com" {
		t.Errorf("Expected email login@example.com, got %s", resp.User.Email)
	}
	if resp.Token == "" {
		t.Error("Expected non-empty session token")
	}

	// Verify session cookie was set
	cookies := w.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("Expected session cookie to be set")
	}
	if !sessionCookie.HttpOnly {
		t.Error("Expected HttpOnly cookie")
	}
	if sessionCookie.Value != resp.Token {
		t.Error("Cookie value should match response token")
	}
}

func TestLogin_EmailNormalization(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	createTestUser(t, database, "norm@example.com", "correctpassword", true)

	body, _ := json.Marshal(loginRequest{
		Email:    "  NORM@EXAMPLE.COM  ",
		Password: "correctpassword",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	handler.HandleLogin(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	createTestUser(t, database, "wrong@example.com", "correctpassword", true)

	body, _ := json.Marshal(loginRequest{
		Email:    "wrong@example.com",
		Password: "wrongpassword",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	handler.HandleLogin(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401, got %d: %s", w.Code, w.Body.String())
	}

	var resp loginResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error != "invalid email or password" {
		t.Errorf("Expected generic error message, got %q", resp.Error)
	}
}

func TestLogin_NonexistentUser(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	body, _ := json.Marshal(loginRequest{
		Email:    "nobody@example.com",
		Password: "somepassword",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	handler.HandleLogin(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401, got %d: %s", w.Code, w.Body.String())
	}

	var resp loginResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	// Should be same generic message as wrong password (anti-enumeration)
	if resp.Error != "invalid email or password" {
		t.Errorf("Expected generic error, got %q", resp.Error)
	}
}

func TestLogin_UnverifiedEmail(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	createTestUser(t, database, "unverified@example.com", "correctpassword", false)

	body, _ := json.Marshal(loginRequest{
		Email:    "unverified@example.com",
		Password: "correctpassword",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	handler.HandleLogin(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("Expected 403, got %d: %s", w.Code, w.Body.String())
	}

	var resp loginResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.EmailNotVerified {
		t.Error("Expected email_not_verified=true")
	}
	if !resp.CanResendVerification {
		t.Error("Expected can_resend_verification=true")
	}
}

func TestLogin_AccountLockout(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	createTestUser(t, database, "locked@example.com", "correctpassword", true)

	// Record 5 failed attempts
	for i := 0; i < auth.LockoutThreshold; i++ {
		if err := database.RecordLoginAttempt("locked@example.com", "127.0.0.1", false); err != nil {
			t.Fatalf("RecordLoginAttempt failed: %v", err)
		}
	}

	body, _ := json.Marshal(loginRequest{
		Email:    "locked@example.com",
		Password: "correctpassword",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	handler.HandleLogin(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("Expected 429, got %d: %s", w.Code, w.Body.String())
	}

	var resp loginResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.RetryAfterMin == nil {
		t.Fatal("Expected retry_after_min in response")
	}
	if *resp.RetryAfterMin != 15 {
		t.Errorf("Expected retry_after_min=15, got %d", *resp.RetryAfterMin)
	}
}

func TestLogin_SuccessClearsAttempts(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	createTestUser(t, database, "clear@example.com", "correctpassword", true)

	// Record some failed attempts (below threshold)
	for i := 0; i < 3; i++ {
		if err := database.RecordLoginAttempt("clear@example.com", "127.0.0.1", false); err != nil {
			t.Fatalf("RecordLoginAttempt failed: %v", err)
		}
	}

	// Login successfully
	body, _ := json.Marshal(loginRequest{
		Email:    "clear@example.com",
		Password: "correctpassword",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	handler.HandleLogin(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify attempts were cleared
	since := time.Now().UTC().Add(-auth.LockoutWindow)
	count, err := database.CountRecentFailedAttempts("clear@example.com", since)
	if err != nil {
		t.Fatalf("CountRecentFailedAttempts failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 failed attempts after successful login, got %d", count)
	}
}

func TestLogin_InvalidJSON(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	handler.HandleLogin(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

func TestLogin_WrongMethod(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	req := httptest.NewRequest(http.MethodGet, "/api/auth/login", nil)
	w := httptest.NewRecorder()
	handler.HandleLogin(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", w.Code)
	}
}

func TestLogin_TOTPRequired(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	// Create user with TOTP enabled
	hash, _ := auth.HashPassword("correctpassword")
	id, _ := auth.GenerateID()
	totpSecret := "JBSWY3DPEHPK3PXP"
	user := &db.User{
		ID:            id,
		Email:         "totp@example.com",
		PasswordHash:  hash,
		TOTPSecret:    &totpSecret,
		TOTPEnabled:   true,
		EmailVerified: true,
		CreatedAt:     time.Now().UTC(),
	}
	now := time.Now().UTC()
	user.VerifiedAt = &now
	if err := database.CreateUser(user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	body, _ := json.Marshal(loginRequest{
		Email:    "totp@example.com",
		Password: "correctpassword",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	handler.HandleLogin(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected 401, got %d: %s", w.Code, w.Body.String())
	}

	var resp loginResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !resp.TOTPRequired {
		t.Error("Expected totp_required=true")
	}
	if resp.Error != "2FA code required" {
		t.Errorf("Expected '2FA code required', got %q", resp.Error)
	}
}
