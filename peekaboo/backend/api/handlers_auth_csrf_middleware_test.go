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
		TokenHash: auth.HashToken(sessionToken),
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
		TokenHash: auth.HashToken(sessionToken),
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
		TokenHash: auth.HashToken(sessionToken),
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
		TokenHash: auth.HashToken(sessionToken),
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
		TokenHash: auth.HashToken(sessionToken),
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
		TokenHash: auth.HashToken(sessionToken),
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
		TokenHash: auth.HashToken(sessionToken2),
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

func TestCSRFMiddleware_MalformedTokenSkipsCSRF(t *testing.T) {
	database := setupAuthTestDB(t)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := CSRFMiddleware(next, testCSRFSecret, database)

	tests := []struct {
		name  string
		token string
	}{
		{"non-hex characters", "not-a-valid-hex-token-at-all!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"},
		{"too short", "abcdef0123456789"},
		{"too long", "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789ff"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called = false
			req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
			req.AddCookie(&http.Cookie{Name: "session", Value: tt.token})
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if !called {
				t.Error("Malformed session token should skip CSRF and pass to handler")
			}
		})
	}
}

func TestCSRFMiddleware_DBError_Returns500(t *testing.T) {
	database := setupAuthTestDB(t)

	// Use a valid 64-char hex token so the middleware attempts a DB lookup
	sessionToken, _ := auth.GenerateToken(32)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := CSRFMiddleware(next, testCSRFSecret, database)

	// Close the database to simulate a DB error
	database.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionToken})
	req.Header.Set("X-CSRF-Token", "any-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if called {
		t.Error("Handler should NOT be called when DB lookup fails")
	}
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected 500 on DB error, got %d", w.Code)
	}

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != "internal error" {
		t.Errorf("Expected 'internal error', got %q", resp["error"])
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
		TokenHash: auth.HashToken(sessionToken),
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
