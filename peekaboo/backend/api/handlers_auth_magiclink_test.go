package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tpott/pub_musings/peekaboo/backend/auth"
)

// --- Magic link request tests ---

func TestMagicLink_Success(t *testing.T) {
	database := setupAuthTestDB(t)
	emailSender := &mockEmailSender{}
	handler := NewAuthHandler(database, emailSender)

	createTestUser(t, database, "magic@example.com", "password123", true)

	body, _ := json.Marshal(magicLinkRequest{
		Email: "magic@example.com",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/magic-link", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.HandleMagicLink(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp magicLinkResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	expectedMsg := "If an account exists with that email, a login link has been sent."
	if resp.Message != expectedMsg {
		t.Errorf("Expected message %q, got %q", expectedMsg, resp.Message)
	}

	// Verify magic link email was sent
	if len(emailSender.magicLinkSent) != 1 {
		t.Fatalf("Expected 1 magic link email sent, got %d", len(emailSender.magicLinkSent))
	}
	if emailSender.magicLinkSent[0].To != "magic@example.com" {
		t.Errorf("Expected email to magic@example.com, got %s", emailSender.magicLinkSent[0].To)
	}
	if emailSender.magicLinkSent[0].Token == "" {
		t.Error("Expected non-empty magic link token")
	}
}

func TestMagicLink_NonexistentEmail(t *testing.T) {
	database := setupAuthTestDB(t)
	emailSender := &mockEmailSender{}
	handler := NewAuthHandler(database, emailSender)

	body, _ := json.Marshal(magicLinkRequest{
		Email: "nobody@example.com",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/magic-link", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.HandleMagicLink(w, req)

	// Should still return 200 to prevent enumeration
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 (anti-enumeration), got %d: %s", w.Code, w.Body.String())
	}

	// No email should be sent
	if len(emailSender.magicLinkSent) != 0 {
		t.Errorf("Expected 0 magic link emails sent, got %d", len(emailSender.magicLinkSent))
	}
}

func TestMagicLink_UnverifiedEmail(t *testing.T) {
	database := setupAuthTestDB(t)
	emailSender := &mockEmailSender{}
	handler := NewAuthHandler(database, emailSender)

	createTestUser(t, database, "unverified@example.com", "password123", false)

	body, _ := json.Marshal(magicLinkRequest{
		Email: "unverified@example.com",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/magic-link", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.HandleMagicLink(w, req)

	// Should return 200 but not send email (unverified users can't use magic link)
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	if len(emailSender.magicLinkSent) != 0 {
		t.Errorf("Expected 0 magic link emails sent for unverified user, got %d", len(emailSender.magicLinkSent))
	}
}

func TestMagicLink_InvalidEmail(t *testing.T) {
	database := setupAuthTestDB(t)
	emailSender := &mockEmailSender{}
	handler := NewAuthHandler(database, emailSender)

	body, _ := json.Marshal(magicLinkRequest{
		Email: "not-valid",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/magic-link", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.HandleMagicLink(w, req)

	// Should still return 200 to prevent enumeration
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 (anti-enumeration), got %d: %s", w.Code, w.Body.String())
	}

	if len(emailSender.magicLinkSent) != 0 {
		t.Errorf("Expected 0 emails sent, got %d", len(emailSender.magicLinkSent))
	}
}

func TestMagicLink_DeletesOldTokens(t *testing.T) {
	database := setupAuthTestDB(t)
	emailSender := &mockEmailSender{}
	handler := NewAuthHandler(database, emailSender)

	user := createTestUser(t, database, "cleanup@example.com", "password123", true)

	// Create an existing unused magic link token
	oldToken := "old-magic-link-token"
	oldTokenHash := auth.HashToken(oldToken)
	oldTokenID, _ := auth.GenerateID()
	expiresAt := time.Now().UTC().Add(15 * time.Minute)
	if err := database.StoreMagicLinkToken(oldTokenID, user.ID, oldTokenHash, expiresAt); err != nil {
		t.Fatalf("StoreMagicLinkToken failed: %v", err)
	}

	body, _ := json.Marshal(magicLinkRequest{
		Email: "cleanup@example.com",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/magic-link", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.HandleMagicLink(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Old token should no longer work
	id, _, _, _, _ := database.GetMagicLinkToken(oldTokenHash)
	if id != "" {
		t.Error("Old unused magic link token should have been deleted")
	}

	// New email should have been sent
	if len(emailSender.magicLinkSent) != 1 {
		t.Fatalf("Expected 1 magic link email sent, got %d", len(emailSender.magicLinkSent))
	}
}

func TestMagicLink_InvalidJSON(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/magic-link", bytes.NewReader([]byte("bad json")))
	w := httptest.NewRecorder()
	handler.HandleMagicLink(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

func TestMagicLink_WrongMethod(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	req := httptest.NewRequest(http.MethodGet, "/api/auth/magic-link", nil)
	w := httptest.NewRecorder()
	handler.HandleMagicLink(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", w.Code)
	}
}

func TestMagicLink_BodyTooLarge(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	largeEmail := strings.Repeat("x", 5*1024) + "@example.com"
	body, _ := json.Marshal(magicLinkRequest{
		Email: largeEmail,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/magic-link", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.HandleMagicLink(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected 413, got %d: %s", w.Code, w.Body.String())
	}
}
