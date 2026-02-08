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

// --- Logout tests ---

func TestLogout_Success(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	user := createTestUser(t, database, "logout@example.com", "password123", true)

	// Create a session
	sessionID, _ := auth.GenerateID()
	sessionToken, _ := auth.GenerateToken(32)
	session := &db.Session{
		ID:        sessionID,
		UserID:    user.ID,
		Token:     sessionToken,
		ExpiresAt: time.Now().UTC().Add(auth.SessionDuration),
		CreatedAt: time.Now().UTC(),
	}
	if err := database.CreateSession(session); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	w := httptest.NewRecorder()
	handler.HandleLogout(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp logoutResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Message != "Logged out" {
		t.Errorf("Expected 'Logged out', got %q", resp.Message)
	}

	// Verify session was deleted from DB
	s, err := database.GetSessionByToken(sessionToken)
	if err != nil {
		t.Fatalf("GetSessionByToken failed: %v", err)
	}
	if s != nil {
		t.Error("Session should be deleted after logout")
	}

	// Verify cookie was cleared
	cookies := w.Result().Cookies()
	for _, c := range cookies {
		if c.Name == "session" && c.MaxAge < 0 {
			return // found the clearing cookie
		}
	}
	t.Error("Expected session cookie to be cleared")
}

func TestLogout_BearerToken(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	user := createTestUser(t, database, "bearer@example.com", "password123", true)

	sessionID, _ := auth.GenerateID()
	sessionToken, _ := auth.GenerateToken(32)
	session := &db.Session{
		ID:        sessionID,
		UserID:    user.ID,
		Token:     sessionToken,
		ExpiresAt: time.Now().UTC().Add(auth.SessionDuration),
		CreatedAt: time.Now().UTC(),
	}
	if err := database.CreateSession(session); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+sessionToken)
	w := httptest.NewRecorder()
	handler.HandleLogout(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLogout_NotAuthenticated(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	w := httptest.NewRecorder()
	handler.HandleLogout(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", w.Code)
	}
}

func TestLogout_InvalidToken(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "nonexistent-token"})
	w := httptest.NewRecorder()
	handler.HandleLogout(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLogout_WrongMethod(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	req := httptest.NewRequest(http.MethodGet, "/api/auth/logout", nil)
	w := httptest.NewRecorder()
	handler.HandleLogout(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", w.Code)
	}
}

// --- Me tests ---

func TestMe_Success(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	user := createTestUser(t, database, "me@example.com", "password123", true)

	sessionID, _ := auth.GenerateID()
	sessionToken, _ := auth.GenerateToken(32)
	session := &db.Session{
		ID:        sessionID,
		UserID:    user.ID,
		Token:     sessionToken,
		ExpiresAt: time.Now().UTC().Add(auth.SessionDuration),
		CreatedAt: time.Now().UTC(),
	}
	if err := database.CreateSession(session); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	w := httptest.NewRecorder()
	handler.HandleMe(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp meResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.User == nil {
		t.Fatal("Expected user in response")
	}
	if resp.User.Email != "me@example.com" {
		t.Errorf("Expected email me@example.com, got %s", resp.User.Email)
	}
	if resp.User.ID != user.ID {
		t.Errorf("Expected user ID %s, got %s", user.ID, resp.User.ID)
	}
	if !resp.User.EmailVerified {
		t.Error("Expected email_verified=true")
	}
	if resp.User.CreatedAt == "" {
		t.Error("Expected non-empty created_at")
	}
}

func TestMe_NotAuthenticated(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	w := httptest.NewRecorder()
	handler.HandleMe(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", w.Code)
	}
}

func TestMe_ExpiredSession(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	user := createTestUser(t, database, "expired@example.com", "password123", true)

	sessionID, _ := auth.GenerateID()
	sessionToken, _ := auth.GenerateToken(32)
	session := &db.Session{
		ID:        sessionID,
		UserID:    user.ID,
		Token:     sessionToken,
		ExpiresAt: time.Now().UTC().Add(-1 * time.Hour), // expired
		CreatedAt: time.Now().UTC().Add(-31 * 24 * time.Hour),
	}
	if err := database.CreateSession(session); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	w := httptest.NewRecorder()
	handler.HandleMe(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d: %s", w.Code, w.Body.String())
	}

	// Verify expired session was cleaned up
	s, err := database.GetSessionByToken(sessionToken)
	if err != nil {
		t.Fatalf("GetSessionByToken failed: %v", err)
	}
	if s != nil {
		t.Error("Expired session should be deleted")
	}
}

func TestMe_BearerToken(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	user := createTestUser(t, database, "bearer-me@example.com", "password123", true)

	sessionID, _ := auth.GenerateID()
	sessionToken, _ := auth.GenerateToken(32)
	session := &db.Session{
		ID:        sessionID,
		UserID:    user.ID,
		Token:     sessionToken,
		ExpiresAt: time.Now().UTC().Add(auth.SessionDuration),
		CreatedAt: time.Now().UTC(),
	}
	if err := database.CreateSession(session); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.Header.Set("Authorization", "Bearer "+sessionToken)
	w := httptest.NewRecorder()
	handler.HandleMe(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp meResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.User == nil || resp.User.Email != "bearer-me@example.com" {
		t.Error("Expected user with correct email from Bearer auth")
	}
}

func TestMe_WrongMethod(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/me", nil)
	w := httptest.NewRecorder()
	handler.HandleMe(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", w.Code)
	}
}

// --- Integration: Register → Verify → Login flow ---

func TestRegisterVerifyLogin(t *testing.T) {
	database := setupAuthTestDB(t)
	emailSender := &mockEmailSender{}
	handler := NewAuthHandler(database, emailSender)

	// Step 1: Register
	regBody, _ := json.Marshal(registerRequest{
		Email:    "flow@example.com",
		Password: "securepassword123",
	})
	regReq := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(regBody))
	regW := httptest.NewRecorder()
	handler.HandleRegister(regW, regReq)
	if regW.Code != http.StatusCreated {
		t.Fatalf("Register: expected 201, got %d: %s", regW.Code, regW.Body.String())
	}

	// Step 2: Try login before verifying (should fail)
	loginBody, _ := json.Marshal(loginRequest{
		Email:    "flow@example.com",
		Password: "securepassword123",
	})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	loginReq.RemoteAddr = "127.0.0.1:1234"
	loginW := httptest.NewRecorder()
	handler.HandleLogin(loginW, loginReq)
	if loginW.Code != http.StatusForbidden {
		t.Fatalf("Pre-verify login: expected 403, got %d: %s", loginW.Code, loginW.Body.String())
	}

	// Step 3: Verify email
	verifyToken := emailSender.sent[0].Token
	verifyReq := httptest.NewRequest(http.MethodGet, "/api/auth/verify?token="+verifyToken, nil)
	verifyW := httptest.NewRecorder()
	handler.HandleVerify(verifyW, verifyReq)
	if verifyW.Code != http.StatusOK {
		t.Fatalf("Verify: expected 200, got %d: %s", verifyW.Code, verifyW.Body.String())
	}

	// Step 4: Login after verifying (should succeed)
	loginBody2, _ := json.Marshal(loginRequest{
		Email:    "flow@example.com",
		Password: "securepassword123",
	})
	loginReq2 := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody2))
	loginReq2.RemoteAddr = "127.0.0.1:1234"
	loginW2 := httptest.NewRecorder()
	handler.HandleLogin(loginW2, loginReq2)
	if loginW2.Code != http.StatusOK {
		t.Fatalf("Post-verify login: expected 200, got %d: %s", loginW2.Code, loginW2.Body.String())
	}

	var loginResp loginResponse
	json.Unmarshal(loginW2.Body.Bytes(), &loginResp)

	// Step 5: Use session to call /me
	meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meReq.AddCookie(&http.Cookie{Name: "session", Value: loginResp.Token})
	meW := httptest.NewRecorder()
	handler.HandleMe(meW, meReq)
	if meW.Code != http.StatusOK {
		t.Fatalf("Me: expected 200, got %d: %s", meW.Code, meW.Body.String())
	}

	var meResp meResponse
	json.Unmarshal(meW.Body.Bytes(), &meResp)
	if meResp.User == nil || meResp.User.Email != "flow@example.com" {
		t.Error("Expected user info from /me")
	}

	// Step 6: Logout
	logoutReq := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutReq.AddCookie(&http.Cookie{Name: "session", Value: loginResp.Token})
	logoutW := httptest.NewRecorder()
	handler.HandleLogout(logoutW, logoutReq)
	if logoutW.Code != http.StatusOK {
		t.Fatalf("Logout: expected 200, got %d: %s", logoutW.Code, logoutW.Body.String())
	}

	// Step 7: /me should now return 401
	meReq2 := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meReq2.AddCookie(&http.Cookie{Name: "session", Value: loginResp.Token})
	meW2 := httptest.NewRecorder()
	handler.HandleMe(meW2, meReq2)
	if meW2.Code != http.StatusUnauthorized {
		t.Fatalf("Me after logout: expected 401, got %d: %s", meW2.Code, meW2.Body.String())
	}
}
