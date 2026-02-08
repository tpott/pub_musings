package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tpott/pub_musings/peekaboo/backend/auth"
	"github.com/tpott/pub_musings/peekaboo/backend/db"
)

var testCSRFSecret = []byte("test-csrf-secret-32-bytes-long!!")

// --- CSRF token endpoint tests ---

func TestCSRF_Success(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})
	handler.CSRFSecret = testCSRFSecret

	user := createTestUser(t, database, "csrf@example.com", "password123", true)

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

	req := httptest.NewRequest(http.MethodGet, "/api/auth/csrf", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	w := httptest.NewRecorder()
	handler.HandleCSRF(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp csrfResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.Token == "" {
		t.Fatal("Expected non-empty CSRF token")
	}

	// Verify the token is valid
	if !auth.ValidateCSRF(resp.Token, sessionToken, testCSRFSecret) {
		t.Error("CSRF token should be valid for the session")
	}
}

func TestCSRF_ConsistentToken(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})
	handler.CSRFSecret = testCSRFSecret

	user := createTestUser(t, database, "consistent@example.com", "password123", true)

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

	// Fetch CSRF token twice — should be the same
	req1 := httptest.NewRequest(http.MethodGet, "/api/auth/csrf", nil)
	req1.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	w1 := httptest.NewRecorder()
	handler.HandleCSRF(w1, req1)

	req2 := httptest.NewRequest(http.MethodGet, "/api/auth/csrf", nil)
	req2.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	w2 := httptest.NewRecorder()
	handler.HandleCSRF(w2, req2)

	var resp1, resp2 csrfResponse
	json.Unmarshal(w1.Body.Bytes(), &resp1)
	json.Unmarshal(w2.Body.Bytes(), &resp2)

	if resp1.Token != resp2.Token {
		t.Error("Same session should produce same CSRF token")
	}
}

func TestCSRF_NotAuthenticated(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})
	handler.CSRFSecret = testCSRFSecret

	req := httptest.NewRequest(http.MethodGet, "/api/auth/csrf", nil)
	w := httptest.NewRecorder()
	handler.HandleCSRF(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", w.Code)
	}
}

func TestCSRF_ExpiredSession(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})
	handler.CSRFSecret = testCSRFSecret

	user := createTestUser(t, database, "expired-csrf@example.com", "password123", true)

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

	req := httptest.NewRequest(http.MethodGet, "/api/auth/csrf", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	w := httptest.NewRecorder()
	handler.HandleCSRF(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCSRF_WrongMethod(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})
	handler.CSRFSecret = testCSRFSecret

	req := httptest.NewRequest(http.MethodPost, "/api/auth/csrf", nil)
	w := httptest.NewRecorder()
	handler.HandleCSRF(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", w.Code)
	}
}

// --- CSRF middleware tests ---

func TestCSRFMiddleware_AllowsSafeMethods(t *testing.T) {
	database := setupAuthTestDB(t)
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := CSRFMiddleware(next, testCSRFSecret, database)

	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		called = false
		req := httptest.NewRequest(method, "/api/auth/me", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if !called {
			t.Errorf("%s should pass through without CSRF check", method)
		}
	}
}

func TestCSRFMiddleware_AllowsExemptPaths(t *testing.T) {
	database := setupAuthTestDB(t)
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := CSRFMiddleware(next, testCSRFSecret, database)

	exemptPaths := []string{
		"/api/auth/login",
		"/api/auth/register",
		"/api/auth/resend-verification",
		"/api/auth/magic-link",
	}

	for _, path := range exemptPaths {
		called = false
		req := httptest.NewRequest(http.MethodPost, path, nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if !called {
			t.Errorf("POST %s should be exempt from CSRF check", path)
		}
	}
}

func TestCSRFMiddleware_AllowsUnauthenticatedRequests(t *testing.T) {
	database := setupAuthTestDB(t)
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := CSRFMiddleware(next, testCSRFSecret, database)

	// POST without session token — should pass through (auth handler will reject)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("Request without session token should pass through CSRF middleware")
	}
}

func TestCSRFMiddleware_RejectsMissingToken(t *testing.T) {
	database := setupAuthTestDB(t)
	user := createTestUser(t, database, "csrf-missing@example.com", "password123", true)

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

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := CSRFMiddleware(next, testCSRFSecret, database)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if called {
		t.Error("Handler should NOT be called when CSRF token is missing")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("Expected 403, got %d", w.Code)
	}

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != "missing CSRF token" {
		t.Errorf("Expected 'missing CSRF token', got %q", resp["error"])
	}
}

func TestCSRFMiddleware_RejectsInvalidToken(t *testing.T) {
	database := setupAuthTestDB(t)
	user := createTestUser(t, database, "csrf-invalid@example.com", "password123", true)

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

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := CSRFMiddleware(next, testCSRFSecret, database)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	req.Header.Set("X-CSRF-Token", "tampered-invalid-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if called {
		t.Error("Handler should NOT be called with invalid CSRF token")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("Expected 403, got %d", w.Code)
	}

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != "invalid CSRF token" {
		t.Errorf("Expected 'invalid CSRF token', got %q", resp["error"])
	}
}

func TestCSRFMiddleware_AcceptsValidToken(t *testing.T) {
	database := setupAuthTestDB(t)
	user := createTestUser(t, database, "csrf-valid@example.com", "password123", true)

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

	csrfToken := auth.GenerateCSRF(sessionToken, testCSRFSecret)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := CSRFMiddleware(next, testCSRFSecret, database)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	req.Header.Set("X-CSRF-Token", csrfToken)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("Handler should be called with valid CSRF token")
	}
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestCSRFMiddleware_AllStateMethods(t *testing.T) {
	database := setupAuthTestDB(t)
	user := createTestUser(t, database, "csrf-methods@example.com", "password123", true)

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

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := CSRFMiddleware(next, testCSRFSecret, database)

	// All state-changing methods should reject without CSRF token
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		req := httptest.NewRequest(method, "/api/protected", nil)
		req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("%s without CSRF token: expected 403, got %d", method, w.Code)
		}
	}
}

func TestCSRFMiddleware_BearerAuth(t *testing.T) {
	database := setupAuthTestDB(t)
	user := createTestUser(t, database, "csrf-bearer@example.com", "password123", true)

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

	csrfToken := auth.GenerateCSRF(sessionToken, testCSRFSecret)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := CSRFMiddleware(next, testCSRFSecret, database)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+sessionToken)
	req.Header.Set("X-CSRF-Token", csrfToken)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("Handler should be called with Bearer auth + valid CSRF token")
	}
}

// --- Integration: fetch CSRF token, then use it ---

func TestCSRFFlow(t *testing.T) {
	database := setupAuthTestDB(t)
	emailSender := &mockEmailSender{}
	handler := NewAuthHandler(database, emailSender)
	handler.CSRFSecret = testCSRFSecret

	// Create and login a user
	user := createTestUser(t, database, "csrf-flow@example.com", "password123", true)
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

	// Step 1: Fetch CSRF token
	csrfReq := httptest.NewRequest(http.MethodGet, "/api/auth/csrf", nil)
	csrfReq.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	csrfW := httptest.NewRecorder()
	handler.HandleCSRF(csrfW, csrfReq)

	if csrfW.Code != http.StatusOK {
		t.Fatalf("CSRF: expected 200, got %d: %s", csrfW.Code, csrfW.Body.String())
	}

	var csrfResp csrfResponse
	json.Unmarshal(csrfW.Body.Bytes(), &csrfResp)
	if csrfResp.Token == "" {
		t.Fatal("Expected CSRF token")
	}

	// Step 2: Use CSRF token for logout (through middleware)
	csrfMw := CSRFMiddleware(http.HandlerFunc(handler.HandleLogout), testCSRFSecret, database)

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutReq.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	logoutReq.Header.Set("X-CSRF-Token", csrfResp.Token)
	logoutW := httptest.NewRecorder()
	csrfMw.ServeHTTP(logoutW, logoutReq)

	if logoutW.Code != http.StatusOK {
		t.Fatalf("Logout with CSRF: expected 200, got %d: %s", logoutW.Code, logoutW.Body.String())
	}

	// Step 3: Verify logout without CSRF token is rejected
	// Need new session since we logged out
	sessionID2, _ := auth.GenerateID()
	sessionToken2, _ := auth.GenerateToken(32)
	session2 := &db.Session{
		ID:        sessionID2,
		UserID:    user.ID,
		Token:     sessionToken2,
		ExpiresAt: time.Now().UTC().Add(auth.SessionDuration),
		CreatedAt: time.Now().UTC(),
	}
	if err := database.CreateSession(session2); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	logoutReq2 := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutReq2.AddCookie(&http.Cookie{Name: "session", Value: sessionToken2})
	// No X-CSRF-Token header
	logoutW2 := httptest.NewRecorder()
	csrfMw.ServeHTTP(logoutW2, logoutReq2)

	if logoutW2.Code != http.StatusForbidden {
		t.Fatalf("Logout without CSRF: expected 403, got %d: %s", logoutW2.Code, logoutW2.Body.String())
	}
}

func TestCSRFMiddleware_FeedbackEndpoint(t *testing.T) {
	database := setupAuthTestDB(t)
	user := createTestUser(t, database, "csrf-feedback@example.com", "password123", true)

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

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := CSRFMiddleware(next, testCSRFSecret, database)

	// POST /api/feedback with session but no CSRF should be rejected
	req := httptest.NewRequest(http.MethodPost, "/api/feedback",
		strings.NewReader(`{"type":"bug","message":"test"}`))
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if called {
		t.Error("POST /api/feedback with session but no CSRF should be rejected")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("Expected 403, got %d", w.Code)
	}
}
