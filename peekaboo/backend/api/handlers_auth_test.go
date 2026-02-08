package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tpott/pub_musings/peekaboo/backend/auth"
	"github.com/tpott/pub_musings/peekaboo/backend/db"
)

// mockEmailSender records sent emails for test assertions.
type mockEmailSender struct {
	sent          []sentEmail
	magicLinkSent []sentEmail
}

type sentEmail struct {
	To    string
	Token string
}

func (m *mockEmailSender) SendVerificationEmail(to, token string) error {
	m.sent = append(m.sent, sentEmail{To: to, Token: token})
	return nil
}

func (m *mockEmailSender) SendMagicLinkEmail(to, token string) error {
	m.magicLinkSent = append(m.magicLinkSent, sentEmail{To: to, Token: token})
	return nil
}

func setupAuthTestDB(t *testing.T) *db.DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	if err := database.Init(); err != nil {
		t.Fatalf("Failed to init database: %v", err)
	}

	t.Cleanup(func() {
		database.Close()
		os.Remove(dbPath)
	})

	return database
}

func createTestUser(t *testing.T, database *db.DB, email, password string, verified bool) *db.User {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	id, err := auth.GenerateID()
	if err != nil {
		t.Fatalf("GenerateID failed: %v", err)
	}
	user := &db.User{
		ID:            id,
		Email:         email,
		PasswordHash:  hash,
		EmailVerified: verified,
		CreatedAt:     time.Now().UTC(),
	}
	if verified {
		now := time.Now().UTC()
		user.VerifiedAt = &now
	}
	if err := database.CreateUser(user); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	return user
}

// --- Register tests ---

func TestRegister_Success(t *testing.T) {
	database := setupAuthTestDB(t)
	emailSender := &mockEmailSender{}
	handler := NewAuthHandler(database, emailSender)

	body, _ := json.Marshal(registerRequest{
		Email:    "test@example.com",
		Password: "securepassword123",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.HandleRegister(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp registerResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.User == nil {
		t.Fatal("Expected user in response")
	}
	if resp.User.Email != "test@example.com" {
		t.Errorf("Expected email test@example.com, got %s", resp.User.Email)
	}
	if resp.User.EmailVerified {
		t.Error("Expected email_verified=false")
	}
	if !resp.EmailVerification {
		t.Error("Expected email_verification=true")
	}
	if resp.User.ID == "" {
		t.Error("Expected non-empty user ID")
	}

	// Verify email was sent
	if len(emailSender.sent) != 1 {
		t.Fatalf("Expected 1 email sent, got %d", len(emailSender.sent))
	}
	if emailSender.sent[0].To != "test@example.com" {
		t.Errorf("Expected email to test@example.com, got %s", emailSender.sent[0].To)
	}
	if emailSender.sent[0].Token == "" {
		t.Error("Expected non-empty verification token")
	}

	// Verify user was saved in DB
	user, err := database.GetUserByEmail("test@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}
	if user == nil {
		t.Fatal("User not found in database")
	}
	if user.EmailVerified {
		t.Error("User should not be verified")
	}
}

func TestRegister_EmailNormalization(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	body, _ := json.Marshal(registerRequest{
		Email:    "  TEST@EXAMPLE.COM  ",
		Password: "securepassword123",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.HandleRegister(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// Verify email was normalized
	user, err := database.GetUserByEmail("test@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}
	if user == nil {
		t.Fatal("User not found with normalized email")
	}
}

func TestRegister_DuplicateEmail(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	createTestUser(t, database, "existing@example.com", "password123", false)

	body, _ := json.Marshal(registerRequest{
		Email:    "existing@example.com",
		Password: "securepassword123",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler.HandleRegister(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("Expected 409, got %d: %s", w.Code, w.Body.String())
	}

	var resp registerResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error != "email already registered" {
		t.Errorf("Expected 'email already registered', got %q", resp.Error)
	}
}

func TestRegister_InvalidEmail(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	tests := []struct {
		name  string
		email string
	}{
		{"empty", ""},
		{"no at sign", "noatsign"},
		{"no domain", "user@"},
		{"no local part", "@example.com"},
		{"no tld", "user@localhost"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(registerRequest{
				Email:    tt.email,
				Password: "securepassword123",
			})

			req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
			w := httptest.NewRecorder()
			handler.HandleRegister(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected 400 for email %q, got %d: %s", tt.email, w.Code, w.Body.String())
			}
		})
	}
}

func TestRegister_InvalidPassword(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	tests := []struct {
		name     string
		password string
	}{
		{"too short", "short"},
		{"empty", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(registerRequest{
				Email:    "test@example.com",
				Password: tt.password,
			})

			req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
			w := httptest.NewRecorder()
			handler.HandleRegister(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected 400 for password %q, got %d: %s", tt.password, w.Code, w.Body.String())
			}
		})
	}
}

func TestRegister_InvalidJSON(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	handler.HandleRegister(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

func TestRegister_WrongMethod(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAuthHandler(database, &mockEmailSender{})

	req := httptest.NewRequest(http.MethodGet, "/api/auth/register", nil)
	w := httptest.NewRecorder()
	handler.HandleRegister(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected 405, got %d", w.Code)
	}
}

// --- Register + Verify integration test ---

func TestRegisterThenVerify(t *testing.T) {
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

	// Get the token from the email sender
	if len(emailSender.sent) != 1 {
		t.Fatalf("Expected 1 email, got %d", len(emailSender.sent))
	}
	verificationToken := emailSender.sent[0].Token

	// Step 2: Verify
	verifyReq := httptest.NewRequest(http.MethodGet, "/api/auth/verify?token="+verificationToken, nil)
	verifyW := httptest.NewRecorder()
	handler.HandleVerify(verifyW, verifyReq)

	if verifyW.Code != http.StatusOK {
		t.Fatalf("Verify: expected 200, got %d: %s", verifyW.Code, verifyW.Body.String())
	}

	// Step 3: Verify user is now verified in DB
	user, err := database.GetUserByEmail("flow@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}
	if !user.EmailVerified {
		t.Error("User should be verified after full flow")
	}
}
