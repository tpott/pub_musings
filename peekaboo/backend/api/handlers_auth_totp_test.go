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

// --- TOTP Disable tests ---

func TestTOTPDisable_Success(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	user := createTOTPUser(t, database, "disable@example.com", "password123")
	sessionToken := createSessionForUser(t, database, user.ID)

	body, _ := json.Marshal(totpDisableRequest{Password: "password123"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/totp/disable", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	w := httptest.NewRecorder()
	handler.HandleTOTPDisable(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp totpDisableResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Message != "TOTP disabled successfully" {
		t.Errorf("Expected success message, got %q", resp.Message)
	}

	dbUser, _ := database.GetUserByID(user.ID)
	if dbUser.TOTPEnabled {
		t.Error("TOTP should be disabled in database")
	}
	if dbUser.TOTPSecret != nil {
		t.Error("TOTP secret should be cleared")
	}
}

func TestTOTPDisable_WrongPassword(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	user := createTOTPUser(t, database, "disable-pw@example.com", "password123")
	sessionToken := createSessionForUser(t, database, user.ID)

	body, _ := json.Marshal(totpDisableRequest{Password: "wrongpassword"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/totp/disable", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	w := httptest.NewRecorder()
	handler.HandleTOTPDisable(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTOTPDisable_NotEnabled(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	user := createTestUser(t, database, "no-totp@example.com", "password123", true)
	sessionToken := createSessionForUser(t, database, user.ID)

	body, _ := json.Marshal(totpDisableRequest{Password: "password123"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/totp/disable", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	w := httptest.NewRecorder()
	handler.HandleTOTPDisable(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("Expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTOTPDisable_NotAuthenticated(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	body, _ := json.Marshal(totpDisableRequest{Password: "pw"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/totp/disable", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.HandleTOTPDisable(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", w.Code)
	}
}

func TestTOTPDisable_MissingPassword(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	user := createTOTPUser(t, database, "no-pw@example.com", "password123")
	sessionToken := createSessionForUser(t, database, user.ID)

	body, _ := json.Marshal(totpDisableRequest{})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/totp/disable", bytes.NewReader(body))
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	w := httptest.NewRecorder()
	handler.HandleTOTPDisable(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Login with TOTP validation tests ---

func TestLogin_TOTPValidCode(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	totpSecret := "JBSWY3DPEHPK3PXP"
	hash, _ := auth.HashPassword("correctpassword")
	id, _ := auth.GenerateID()
	now := time.Now().UTC()
	user := &db.User{
		ID: id, Email: "totp-valid@example.com", PasswordHash: hash,
		TOTPSecret: &totpSecret, TOTPEnabled: true,
		EmailVerified: true, VerifiedAt: &now, CreatedAt: now,
	}
	if err := database.CreateUser(user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	code, _ := auth.GenerateCodeAt(totpSecret, time.Now())

	body, _ := json.Marshal(loginRequest{
		Email: "totp-valid@example.com", Password: "correctpassword", TOTPCode: code,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	handler.HandleLogin(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp loginResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Token == "" {
		t.Error("Expected session token on successful TOTP login")
	}
}

func TestLogin_TOTPInvalidCode(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	totpSecret := "JBSWY3DPEHPK3PXP"
	hash, _ := auth.HashPassword("correctpassword")
	id, _ := auth.GenerateID()
	now := time.Now().UTC()
	user := &db.User{
		ID: id, Email: "totp-invalid@example.com", PasswordHash: hash,
		TOTPSecret: &totpSecret, TOTPEnabled: true,
		EmailVerified: true, VerifiedAt: &now, CreatedAt: now,
	}
	if err := database.CreateUser(user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	body, _ := json.Marshal(loginRequest{
		Email: "totp-invalid@example.com", Password: "correctpassword", TOTPCode: "000000",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	handler.HandleLogin(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d: %s", w.Code, w.Body.String())
	}

	var resp loginResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error != "invalid 2FA code" {
		t.Errorf("Expected 'invalid 2FA code', got %q", resp.Error)
	}
}

// --- Integration: Setup → Enable → Login with TOTP → Disable ---

func TestTOTPFullFlow(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	user := createTestUser(t, database, "totp-flow@example.com", "password123", true)
	sessionToken := createSessionForUser(t, database, user.ID)

	// Step 1: Setup TOTP
	setupReq := httptest.NewRequest(http.MethodPost, "/api/auth/totp/setup", nil)
	setupReq.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	setupW := httptest.NewRecorder()
	handler.HandleTOTPSetup(setupW, setupReq)
	if setupW.Code != http.StatusOK {
		t.Fatalf("Setup: expected 200, got %d: %s", setupW.Code, setupW.Body.String())
	}
	var setupResp totpSetupResponse
	json.Unmarshal(setupW.Body.Bytes(), &setupResp)

	// Step 2: Enable TOTP with valid code
	code, _ := auth.GenerateCodeAt(setupResp.Secret, time.Now())
	enableBody, _ := json.Marshal(totpEnableRequest{Code: code, Password: "password123"})
	enableReq := httptest.NewRequest(http.MethodPost, "/api/auth/totp/enable", bytes.NewReader(enableBody))
	enableReq.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	enableW := httptest.NewRecorder()
	handler.HandleTOTPEnable(enableW, enableReq)
	if enableW.Code != http.StatusOK {
		t.Fatalf("Enable: expected 200, got %d: %s", enableW.Code, enableW.Body.String())
	}

	// Step 3: Login without TOTP code — requires TOTP
	loginBody, _ := json.Marshal(loginRequest{Email: "totp-flow@example.com", Password: "password123"})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	loginReq.RemoteAddr = "127.0.0.1:1234"
	loginW := httptest.NewRecorder()
	handler.HandleLogin(loginW, loginReq)
	if loginW.Code != http.StatusUnauthorized {
		t.Fatalf("Login without TOTP: expected 401, got %d", loginW.Code)
	}
	var loginResp loginResponse
	json.Unmarshal(loginW.Body.Bytes(), &loginResp)
	if !loginResp.TOTPRequired {
		t.Error("Expected totp_required=true")
	}

	// Step 4: Login with valid TOTP code
	code2, _ := auth.GenerateCodeAt(setupResp.Secret, time.Now())
	loginBody2, _ := json.Marshal(loginRequest{
		Email: "totp-flow@example.com", Password: "password123", TOTPCode: code2,
	})
	loginReq2 := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody2))
	loginReq2.RemoteAddr = "127.0.0.1:1234"
	loginW2 := httptest.NewRecorder()
	handler.HandleLogin(loginW2, loginReq2)
	if loginW2.Code != http.StatusOK {
		t.Fatalf("Login with TOTP: expected 200, got %d: %s", loginW2.Code, loginW2.Body.String())
	}

	// Step 5: Disable TOTP
	var loginResp2 loginResponse
	json.Unmarshal(loginW2.Body.Bytes(), &loginResp2)
	disableBody, _ := json.Marshal(totpDisableRequest{Password: "password123"})
	disableReq := httptest.NewRequest(http.MethodPost, "/api/auth/totp/disable", bytes.NewReader(disableBody))
	disableReq.AddCookie(&http.Cookie{Name: "session", Value: loginResp2.Token})
	disableW := httptest.NewRecorder()
	handler.HandleTOTPDisable(disableW, disableReq)
	if disableW.Code != http.StatusOK {
		t.Fatalf("Disable: expected 200, got %d: %s", disableW.Code, disableW.Body.String())
	}

	// Step 6: Login without TOTP succeeds now
	loginBody3, _ := json.Marshal(loginRequest{Email: "totp-flow@example.com", Password: "password123"})
	loginReq3 := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody3))
	loginReq3.RemoteAddr = "127.0.0.1:1234"
	loginW3 := httptest.NewRecorder()
	handler.HandleLogin(loginW3, loginReq3)
	if loginW3.Code != http.StatusOK {
		t.Fatalf("Login after disable: expected 200, got %d: %s", loginW3.Code, loginW3.Body.String())
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
