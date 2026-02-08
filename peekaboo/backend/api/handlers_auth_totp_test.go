package api

import (
	"bytes"
	"encoding/base32"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tpott/pub_musings/peekaboo/backend/auth"
	"github.com/tpott/pub_musings/peekaboo/backend/db"
)

// --- TOTP Setup tests ---

func TestTOTPSetup_Success(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	user := createTestUser(t, database, "totp-setup@example.com", "password123", true)
	sessionToken := createSessionForUser(t, database, user.ID)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/totp/setup", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	w := httptest.NewRecorder()
	handler.HandleTOTPSetup(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp totpSetupResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Secret == "" {
		t.Error("Expected non-empty secret")
	}

	// Secret should be valid base32
	_, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(resp.Secret)
	if err != nil {
		t.Errorf("Secret is not valid base32: %v", err)
	}

	if resp.URI == "" {
		t.Error("Expected non-empty URI")
	}

	// Secret should be stored in DB but TOTP not yet enabled
	dbUser, _ := database.GetUserByID(user.ID)
	if dbUser.TOTPSecret == nil || *dbUser.TOTPSecret != resp.Secret {
		t.Error("Secret should be stored in database")
	}
	if dbUser.TOTPEnabled {
		t.Error("TOTP should not be enabled yet after setup")
	}
}

func TestTOTPSetup_NotAuthenticated(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/totp/setup", nil)
	w := httptest.NewRecorder()
	handler.HandleTOTPSetup(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTOTPSetup_AlreadyEnabled(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	user := createTOTPUser(t, database, "already-enabled@example.com", "password123")
	sessionToken := createSessionForUser(t, database, user.ID)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/totp/setup", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	w := httptest.NewRecorder()
	handler.HandleTOTPSetup(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("Expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTOTPSetup_WrongMethod(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	req := httptest.NewRequest(http.MethodGet, "/api/auth/totp/setup", nil)
	w := httptest.NewRecorder()
	handler.HandleTOTPSetup(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", w.Code)
	}
}

// --- TOTP Enable tests ---

func TestTOTPEnable_Success(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	user := createTestUser(t, database, "totp-enable@example.com", "password123", true)
	sessionToken := createSessionForUser(t, database, user.ID)

	secret, _ := auth.GenerateTOTPSecret()
	if err := database.SetTOTPSecret(user.ID, secret); err != nil {
		t.Fatalf("SetTOTPSecret failed: %v", err)
	}

	code, err := auth.GenerateCodeAt(secret, time.Now())
	if err != nil {
		t.Fatalf("GenerateCodeAt failed: %v", err)
	}

	body, _ := json.Marshal(totpEnableRequest{Code: code, Password: "password123"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/totp/enable", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	w := httptest.NewRecorder()
	handler.HandleTOTPEnable(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp totpEnableResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Message != "TOTP enabled successfully" {
		t.Errorf("Expected success message, got %q", resp.Message)
	}

	dbUser, _ := database.GetUserByID(user.ID)
	if !dbUser.TOTPEnabled {
		t.Error("TOTP should be enabled in database")
	}
}

func TestTOTPEnable_InvalidCode(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	user := createTestUser(t, database, "bad-code@example.com", "password123", true)
	sessionToken := createSessionForUser(t, database, user.ID)

	secret, _ := auth.GenerateTOTPSecret()
	if err := database.SetTOTPSecret(user.ID, secret); err != nil {
		t.Fatalf("SetTOTPSecret failed: %v", err)
	}

	body, _ := json.Marshal(totpEnableRequest{Code: "000000", Password: "password123"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/totp/enable", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	w := httptest.NewRecorder()
	handler.HandleTOTPEnable(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTOTPEnable_WrongPassword(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	user := createTestUser(t, database, "wrong-pw@example.com", "password123", true)
	sessionToken := createSessionForUser(t, database, user.ID)

	secret, _ := auth.GenerateTOTPSecret()
	if err := database.SetTOTPSecret(user.ID, secret); err != nil {
		t.Fatalf("SetTOTPSecret failed: %v", err)
	}

	body, _ := json.Marshal(totpEnableRequest{Code: "123456", Password: "wrongpassword"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/totp/enable", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	w := httptest.NewRecorder()
	handler.HandleTOTPEnable(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTOTPEnable_MissingFields(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	user := createTestUser(t, database, "missing@example.com", "password123", true)
	sessionToken := createSessionForUser(t, database, user.ID)

	tests := []struct {
		name string
		body totpEnableRequest
	}{
		{"no password", totpEnableRequest{Code: "123456"}},
		{"no code", totpEnableRequest{Password: "password123"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/api/auth/totp/enable", bytes.NewReader(body))
			req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
			w := httptest.NewRecorder()
			handler.HandleTOTPEnable(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected 400, got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestTOTPEnable_NoSetup(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	user := createTestUser(t, database, "no-setup@example.com", "password123", true)
	sessionToken := createSessionForUser(t, database, user.ID)

	body, _ := json.Marshal(totpEnableRequest{Code: "123456", Password: "password123"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/totp/enable", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	w := httptest.NewRecorder()
	handler.HandleTOTPEnable(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTOTPEnable_AlreadyEnabled(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	user := createTOTPUser(t, database, "already@example.com", "password123")
	sessionToken := createSessionForUser(t, database, user.ID)

	body, _ := json.Marshal(totpEnableRequest{Code: "123456", Password: "password123"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/totp/enable", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	w := httptest.NewRecorder()
	handler.HandleTOTPEnable(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("Expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTOTPEnable_NotAuthenticated(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	body, _ := json.Marshal(totpEnableRequest{Code: "123456", Password: "pw"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/totp/enable", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.HandleTOTPEnable(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", w.Code)
	}
}

// --- Test helpers ---

func createSessionForUser(t *testing.T, database *db.DB, userID string) string {
	t.Helper()
	sessionID, _ := auth.GenerateID()
	sessionToken, _ := auth.GenerateToken(32)
	session := &db.Session{
		ID:        sessionID,
		UserID:    userID,
		Token:     sessionToken,
		ExpiresAt: time.Now().UTC().Add(auth.SessionDuration),
		CreatedAt: time.Now().UTC(),
	}
	if err := database.CreateSession(session); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	return sessionToken
}

func createTOTPUser(t *testing.T, database *db.DB, email, password string) *db.User {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	id, _ := auth.GenerateID()
	secret := "JBSWY3DPEHPK3PXP"
	now := time.Now().UTC()
	user := &db.User{
		ID: id, Email: email, PasswordHash: hash,
		TOTPSecret: &secret, TOTPEnabled: true,
		EmailVerified: true, VerifiedAt: &now, CreatedAt: now,
	}
	if err := database.CreateUser(user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	return user
}
