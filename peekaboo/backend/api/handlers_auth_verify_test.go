package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tpott/pub_musings/peekaboo/backend/auth"
)

// --- Verify tests ---

func TestVerify_Success(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	user := createTestUser(t, database, "verify@example.com", "password123", false)

	// Create a verification token
	token := "test-verification-token-abc123"
	tokenHash := auth.HashToken(token)
	tokenID, _ := auth.GenerateID()
	expiresAt := time.Now().UTC().Add(24 * time.Hour)
	if err := database.StoreEmailVerificationToken(tokenID, user.ID, tokenHash, expiresAt); err != nil {
		t.Fatalf("StoreEmailVerificationToken failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/verify?token="+token, nil)
	w := httptest.NewRecorder()
	handler.HandleVerify(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp verifyResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Message != "Email verified successfully. You can now log in." {
		t.Errorf("Unexpected message: %q", resp.Message)
	}

	// Verify user is now verified in DB
	updatedUser, err := database.GetUserByEmail("verify@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}
	if !updatedUser.EmailVerified {
		t.Error("User should be verified after token verification")
	}
}

func TestVerify_MissingToken(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	req := httptest.NewRequest(http.MethodGet, "/api/auth/verify", nil)
	w := httptest.NewRecorder()
	handler.HandleVerify(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

func TestVerify_OversizedToken(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	oversizedToken := strings.Repeat("a", maxTokenLength+1)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/verify?token="+oversizedToken, nil)
	w := httptest.NewRecorder()
	handler.HandleVerify(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}

	var resp verifyResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error != "invalid token" {
		t.Errorf("Expected 'invalid token', got %q", resp.Error)
	}
}

func TestVerify_InvalidToken(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	req := httptest.NewRequest(http.MethodGet, "/api/auth/verify?token=nonexistent-token", nil)
	w := httptest.NewRecorder()
	handler.HandleVerify(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}

	var resp verifyResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error != "invalid or expired token" {
		t.Errorf("Expected 'invalid or expired token', got %q", resp.Error)
	}
}

func TestVerify_ExpiredToken(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	user := createTestUser(t, database, "expired@example.com", "password123", false)

	token := "expired-verification-token"
	tokenHash := auth.HashToken(token)
	tokenID, _ := auth.GenerateID()
	expiresAt := time.Now().UTC().Add(-1 * time.Hour) // expired
	if err := database.StoreEmailVerificationToken(tokenID, user.ID, tokenHash, expiresAt); err != nil {
		t.Fatalf("StoreEmailVerificationToken failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/verify?token="+token, nil)
	w := httptest.NewRecorder()
	handler.HandleVerify(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d: %s", w.Code, w.Body.String())
	}

	var resp verifyResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error != "token expired" {
		t.Errorf("Expected 'token expired', got %q", resp.Error)
	}
}

func TestVerify_AlreadyUsedToken(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	user := createTestUser(t, database, "used@example.com", "password123", false)

	token := "used-verification-token"
	tokenHash := auth.HashToken(token)
	tokenID, _ := auth.GenerateID()
	expiresAt := time.Now().UTC().Add(24 * time.Hour)
	if err := database.StoreEmailVerificationToken(tokenID, user.ID, tokenHash, expiresAt); err != nil {
		t.Fatalf("StoreEmailVerificationToken failed: %v", err)
	}
	if err := database.MarkEmailVerificationTokenUsed(tokenID); err != nil {
		t.Fatalf("MarkEmailVerificationTokenUsed failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/verify?token="+token, nil)
	w := httptest.NewRecorder()
	handler.HandleVerify(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d: %s", w.Code, w.Body.String())
	}

	var resp verifyResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error != "token already used" {
		t.Errorf("Expected 'token already used', got %q", resp.Error)
	}
}

func TestVerify_WrongMethod(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/verify?token=abc", nil)
	w := httptest.NewRecorder()
	handler.HandleVerify(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", w.Code)
	}
}

// --- Resend verification tests ---

func TestResendVerification_Success(t *testing.T) {
	database := setupAuthTestDB(t)
	emailSender := &mockEmailSender{}
	handler := NewAuthHandler(database, emailSender)

	createTestUser(t, database, "unverified@example.com", "password123", false)

	body, _ := json.Marshal(resendVerificationRequest{
		Email: "unverified@example.com",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/resend-verification", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.HandleResendVerification(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify email was sent
	if len(emailSender.sent) != 1 {
		t.Fatalf("Expected 1 email sent, got %d", len(emailSender.sent))
	}
	if emailSender.sent[0].To != "unverified@example.com" {
		t.Errorf("Expected email to unverified@example.com, got %s", emailSender.sent[0].To)
	}
}

func TestResendVerification_NonexistentEmail(t *testing.T) {
	database := setupAuthTestDB(t)
	emailSender := &mockEmailSender{}
	handler := NewAuthHandler(database, emailSender)

	body, _ := json.Marshal(resendVerificationRequest{
		Email: "nobody@example.com",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/resend-verification", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.HandleResendVerification(w, req)

	// Should still return 200 to prevent enumeration
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 (anti-enumeration), got %d: %s", w.Code, w.Body.String())
	}

	// No email should be sent
	if len(emailSender.sent) != 0 {
		t.Errorf("Expected 0 emails sent, got %d", len(emailSender.sent))
	}
}

func TestResendVerification_AlreadyVerified(t *testing.T) {
	database := setupAuthTestDB(t)
	emailSender := &mockEmailSender{}
	handler := NewAuthHandler(database, emailSender)

	createTestUser(t, database, "verified@example.com", "password123", true)

	body, _ := json.Marshal(resendVerificationRequest{
		Email: "verified@example.com",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/resend-verification", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.HandleResendVerification(w, req)

	// Should still return 200
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// No email should be sent
	if len(emailSender.sent) != 0 {
		t.Errorf("Expected 0 emails sent for already-verified user, got %d", len(emailSender.sent))
	}
}

func TestResendVerification_InvalidEmail(t *testing.T) {
	database := setupAuthTestDB(t)
	emailSender := &mockEmailSender{}
	handler := NewAuthHandler(database, emailSender)

	body, _ := json.Marshal(resendVerificationRequest{
		Email: "not-valid",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/resend-verification", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.HandleResendVerification(w, req)

	// Should still return 200 to prevent enumeration
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 (anti-enumeration), got %d: %s", w.Code, w.Body.String())
	}

	if len(emailSender.sent) != 0 {
		t.Errorf("Expected 0 emails sent, got %d", len(emailSender.sent))
	}
}

func TestResendVerification_DeletesOldTokens(t *testing.T) {
	database := setupAuthTestDB(t)
	emailSender := &mockEmailSender{}
	handler := NewAuthHandler(database, emailSender)

	user := createTestUser(t, database, "resend@example.com", "password123", false)

	// Create an existing unused token
	oldToken := "old-verification-token"
	oldTokenHash := auth.HashToken(oldToken)
	oldTokenID, _ := auth.GenerateID()
	expiresAt := time.Now().UTC().Add(24 * time.Hour)
	if err := database.StoreEmailVerificationToken(oldTokenID, user.ID, oldTokenHash, expiresAt); err != nil {
		t.Fatalf("StoreEmailVerificationToken failed: %v", err)
	}

	body, _ := json.Marshal(resendVerificationRequest{
		Email: "resend@example.com",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/resend-verification", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.HandleResendVerification(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Old token should no longer work
	id, _, _, _, _ := database.GetEmailVerificationToken(oldTokenHash)
	if id != "" {
		t.Error("Old unused token should have been deleted")
	}

	// New email should have been sent
	if len(emailSender.sent) != 1 {
		t.Fatalf("Expected 1 email sent, got %d", len(emailSender.sent))
	}
}

func TestResendVerification_WrongMethod(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	req := httptest.NewRequest(http.MethodGet, "/api/auth/resend-verification", nil)
	w := httptest.NewRecorder()
	handler.HandleResendVerification(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", w.Code)
	}
}

func TestResendVerification_InvalidJSON(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/resend-verification", bytes.NewReader([]byte("bad json")))
	w := httptest.NewRecorder()
	handler.HandleResendVerification(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

func TestResendVerification_RateLimited(t *testing.T) {
	database := setupAuthTestDB(t)
	emailSender := &mockEmailSender{}
	handler := NewAuthHandler(database, emailSender)

	createTestUser(t, database, "ratelimit@example.com", "password123", false)

	// Wrap handler with rate limiter (3 per 15min, matching main.go config)
	limiter := NewRateLimiter(3, 15*time.Minute)
	limited := RateLimitMiddleware(http.HandlerFunc(handler.HandleResendVerification), limiter)

	body, _ := json.Marshal(resendVerificationRequest{
		Email: "ratelimit@example.com",
	})

	// First 3 requests should succeed
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/resend-verification", bytes.NewReader(body))
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()
		limited.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Request %d: expected 200, got %d: %s", i+1, w.Code, w.Body.String())
		}
	}

	// 4th request should be rate limited
	req := httptest.NewRequest(http.MethodPost, "/api/auth/resend-verification", bytes.NewReader(body))
	req.RemoteAddr = "192.168.1.1:12345"
	w := httptest.NewRecorder()
	limited.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429, got %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Error string `json:"error"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error != "rate limit exceeded, try again later" {
		t.Errorf("Unexpected error: %q", resp.Error)
	}

	if w.Header().Get("Retry-After") != "60" {
		t.Errorf("Expected Retry-After: 60, got %q", w.Header().Get("Retry-After"))
	}
}

func TestResendVerification_EmailSendFailure(t *testing.T) {
	database := setupAuthTestDB(t)
	emailSender := &mockEmailSender{err: fmt.Errorf("SMTP connection refused")}
	handler := NewAuthHandler(database, emailSender)

	createTestUser(t, database, "fail@example.com", "password123", false)

	body, _ := json.Marshal(resendVerificationRequest{
		Email: "fail@example.com",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/resend-verification", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.HandleResendVerification(w, req)

	// Should still return 200 (error is logged, not exposed)
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200 even on email failure, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the success message is still returned
	var resp resendVerificationResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	expectedMsg := "If an unverified account exists with that email, a verification link has been sent."
	if resp.Message != expectedMsg {
		t.Errorf("Expected message %q, got %q", expectedMsg, resp.Message)
	}

	// Verify no email was recorded (error prevented it)
	if len(emailSender.sent) != 0 {
		t.Errorf("Expected 0 emails sent on failure, got %d", len(emailSender.sent))
	}
}
