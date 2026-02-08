package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
