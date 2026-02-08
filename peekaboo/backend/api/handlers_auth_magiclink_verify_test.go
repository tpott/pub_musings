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

// --- Magic link verify tests ---

func TestMagicLinkVerify_Success(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	user := createTestUser(t, database, "verify-magic@example.com", "password123", true)

	// Create a magic link token
	token := "test-magic-link-token-abc123"
	tokenHash := auth.HashToken(token)
	tokenID, _ := auth.GenerateID()
	expiresAt := time.Now().UTC().Add(15 * time.Minute)
	if err := database.StoreMagicLinkToken(tokenID, user.ID, tokenHash, expiresAt); err != nil {
		t.Fatalf("StoreMagicLinkToken failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/magic-link/verify?token="+token, nil)
	w := httptest.NewRecorder()
	handler.HandleMagicLinkVerify(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp magicLinkVerifyResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Message != "Login successful" {
		t.Errorf("Expected 'Login successful', got %q", resp.Message)
	}
	if resp.User == nil {
		t.Fatal("Expected user in response")
	}
	if resp.User.Email != "verify-magic@example.com" {
		t.Errorf("Expected email verify-magic@example.com, got %s", resp.User.Email)
	}
	if resp.User.ID != user.ID {
		t.Errorf("Expected user ID %s, got %s", user.ID, resp.User.ID)
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

	// Verify session works with /me
	meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meReq.AddCookie(&http.Cookie{Name: "session", Value: resp.Token})
	meW := httptest.NewRecorder()
	handler.HandleMe(meW, meReq)

	if meW.Code != http.StatusOK {
		t.Fatalf("Me: expected 200, got %d: %s", meW.Code, meW.Body.String())
	}
}

func TestMagicLinkVerify_BypassesTOTP(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	// Create user with TOTP enabled
	hash, _ := auth.HashPassword("password123")
	id, _ := auth.GenerateID()
	totpSecret := "JBSWY3DPEHPK3PXP"
	now := time.Now().UTC()
	user := &db.User{
		ID:            id,
		Email:         "totp-magic@example.com",
		PasswordHash:  hash,
		TOTPSecret:    &totpSecret,
		TOTPEnabled:   true,
		EmailVerified: true,
		CreatedAt:     now,
		VerifiedAt:    &now,
	}
	if err := database.CreateUser(user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Create a magic link token
	token := "totp-bypass-magic-link-token"
	tokenHash := auth.HashToken(token)
	tokenID, _ := auth.GenerateID()
	expiresAt := time.Now().UTC().Add(15 * time.Minute)
	if err := database.StoreMagicLinkToken(tokenID, user.ID, tokenHash, expiresAt); err != nil {
		t.Fatalf("StoreMagicLinkToken failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/magic-link/verify?token="+token, nil)
	w := httptest.NewRecorder()
	handler.HandleMagicLinkVerify(w, req)

	// Magic link should succeed even though TOTP is enabled
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 (TOTP bypass), got %d: %s", w.Code, w.Body.String())
	}

	var resp magicLinkVerifyResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.User == nil {
		t.Fatal("Expected user in response")
	}
	if resp.Token == "" {
		t.Error("Expected session token (TOTP bypassed)")
	}
}

func TestMagicLinkVerify_MissingToken(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	req := httptest.NewRequest(http.MethodGet, "/api/auth/magic-link/verify", nil)
	w := httptest.NewRecorder()
	handler.HandleMagicLinkVerify(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}

	var resp magicLinkVerifyResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error != "missing token parameter" {
		t.Errorf("Expected 'missing token parameter', got %q", resp.Error)
	}
}

func TestMagicLinkVerify_InvalidToken(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	req := httptest.NewRequest(http.MethodGet, "/api/auth/magic-link/verify?token=nonexistent-token", nil)
	w := httptest.NewRecorder()
	handler.HandleMagicLinkVerify(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}

	var resp magicLinkVerifyResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error != "invalid or expired token" {
		t.Errorf("Expected 'invalid or expired token', got %q", resp.Error)
	}
}

func TestMagicLinkVerify_ExpiredToken(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	user := createTestUser(t, database, "expired-magic@example.com", "password123", true)

	token := "expired-magic-link-token"
	tokenHash := auth.HashToken(token)
	tokenID, _ := auth.GenerateID()
	expiresAt := time.Now().UTC().Add(-1 * time.Hour) // expired
	if err := database.StoreMagicLinkToken(tokenID, user.ID, tokenHash, expiresAt); err != nil {
		t.Fatalf("StoreMagicLinkToken failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/magic-link/verify?token="+token, nil)
	w := httptest.NewRecorder()
	handler.HandleMagicLinkVerify(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d: %s", w.Code, w.Body.String())
	}

	var resp magicLinkVerifyResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error != "token expired" {
		t.Errorf("Expected 'token expired', got %q", resp.Error)
	}
}

func TestMagicLinkVerify_AlreadyUsedToken(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	user := createTestUser(t, database, "used-magic@example.com", "password123", true)

	token := "used-magic-link-token"
	tokenHash := auth.HashToken(token)
	tokenID, _ := auth.GenerateID()
	expiresAt := time.Now().UTC().Add(15 * time.Minute)
	if err := database.StoreMagicLinkToken(tokenID, user.ID, tokenHash, expiresAt); err != nil {
		t.Fatalf("StoreMagicLinkToken failed: %v", err)
	}
	if err := database.MarkMagicLinkTokenUsed(tokenID); err != nil {
		t.Fatalf("MarkMagicLinkTokenUsed failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/magic-link/verify?token="+token, nil)
	w := httptest.NewRecorder()
	handler.HandleMagicLinkVerify(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d: %s", w.Code, w.Body.String())
	}

	var resp magicLinkVerifyResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error != "token already used" {
		t.Errorf("Expected 'token already used', got %q", resp.Error)
	}
}

func TestMagicLinkVerify_WrongMethod(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/magic-link/verify?token=abc", nil)
	w := httptest.NewRecorder()
	handler.HandleMagicLinkVerify(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", w.Code)
	}
}

// --- Integration: Magic Link request → verify → session flow ---

func TestMagicLinkFlow(t *testing.T) {
	database := setupAuthTestDB(t)
	emailSender := &mockEmailSender{}
	handler := NewAuthHandler(database, emailSender)

	// Step 1: Create a verified user
	createTestUser(t, database, "flow-magic@example.com", "password123", true)

	// Step 2: Request magic link
	body, _ := json.Marshal(magicLinkRequest{
		Email: "flow-magic@example.com",
	})
	mlReq := httptest.NewRequest(http.MethodPost, "/api/auth/magic-link", bytes.NewReader(body))
	mlW := httptest.NewRecorder()
	handler.HandleMagicLink(mlW, mlReq)

	if mlW.Code != http.StatusOK {
		t.Fatalf("Magic link request: expected 200, got %d: %s", mlW.Code, mlW.Body.String())
	}

	// Get the token from the email sender
	if len(emailSender.magicLinkSent) != 1 {
		t.Fatalf("Expected 1 magic link email, got %d", len(emailSender.magicLinkSent))
	}
	magicToken := emailSender.magicLinkSent[0].Token

	// Step 3: Verify magic link token
	verifyReq := httptest.NewRequest(http.MethodGet, "/api/auth/magic-link/verify?token="+magicToken, nil)
	verifyW := httptest.NewRecorder()
	handler.HandleMagicLinkVerify(verifyW, verifyReq)

	if verifyW.Code != http.StatusOK {
		t.Fatalf("Magic link verify: expected 200, got %d: %s", verifyW.Code, verifyW.Body.String())
	}

	var verifyResp magicLinkVerifyResponse
	json.Unmarshal(verifyW.Body.Bytes(), &verifyResp)
	if verifyResp.Token == "" {
		t.Fatal("Expected session token from magic link verify")
	}

	// Step 4: Use session with /me
	meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meReq.AddCookie(&http.Cookie{Name: "session", Value: verifyResp.Token})
	meW := httptest.NewRecorder()
	handler.HandleMe(meW, meReq)

	if meW.Code != http.StatusOK {
		t.Fatalf("Me: expected 200, got %d: %s", meW.Code, meW.Body.String())
	}

	var meResp meResponse
	json.Unmarshal(meW.Body.Bytes(), &meResp)
	if meResp.User == nil || meResp.User.Email != "flow-magic@example.com" {
		t.Error("Expected user info from /me after magic link login")
	}

	// Step 5: Try using the same magic link token again (should fail)
	verifyReq2 := httptest.NewRequest(http.MethodGet, "/api/auth/magic-link/verify?token="+magicToken, nil)
	verifyW2 := httptest.NewRecorder()
	handler.HandleMagicLinkVerify(verifyW2, verifyReq2)

	if verifyW2.Code != http.StatusBadRequest {
		t.Fatalf("Reuse magic link: expected 400, got %d: %s", verifyW2.Code, verifyW2.Body.String())
	}

	var reuseResp magicLinkVerifyResponse
	json.Unmarshal(verifyW2.Body.Bytes(), &reuseResp)
	if reuseResp.Error != "token already used" {
		t.Errorf("Expected 'token already used', got %q", reuseResp.Error)
	}

	// Step 6: Logout
	logoutReq := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutReq.AddCookie(&http.Cookie{Name: "session", Value: verifyResp.Token})
	logoutW := httptest.NewRecorder()
	handler.HandleLogout(logoutW, logoutReq)

	if logoutW.Code != http.StatusOK {
		t.Fatalf("Logout: expected 200, got %d: %s", logoutW.Code, logoutW.Body.String())
	}

	// Step 7: /me should now return 401
	meReq2 := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meReq2.AddCookie(&http.Cookie{Name: "session", Value: verifyResp.Token})
	meW2 := httptest.NewRecorder()
	handler.HandleMe(meW2, meReq2)

	if meW2.Code != http.StatusUnauthorized {
		t.Fatalf("Me after logout: expected 401, got %d: %s", meW2.Code, meW2.Body.String())
	}
}
