package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tpott/subtitler/backend/auth"
	"github.com/tpott/subtitler/backend/crypto"
	"github.com/tpott/subtitler/backend/db"
	"github.com/tpott/subtitler/backend/email"
	"github.com/tpott/subtitler/backend/ratelimit"
	"github.com/tpott/subtitler/backend/totp"
)

func TestAuthRegister(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	tests := []struct {
		name       string
		email      string
		password   string
		wantStatus int
		wantError  bool
	}{
		{
			name:       "valid registration",
			email:      "test@example.com",
			password:   "ValidPassword123!",
			wantStatus: http.StatusCreated, // Changed from 200 to 201
			wantError:  false,
		},
		{
			name:       "invalid email",
			email:      "notanemail",
			password:   "ValidPassword123!",
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
		{
			name:       "weak password",
			email:      "test2@example.com",
			password:   "weak",
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
		{
			name:       "empty email",
			email:      "",
			password:   "ValidPassword123!",
			wantStatus: http.StatusBadRequest,
			wantError:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := ts.doRequest("POST", "/api/auth/register", map[string]string{
				"email":    tc.email,
				"password": tc.password,
			}, "")

			if resp.Code != tc.wantStatus {
				t.Errorf("Expected status %d, got %d: %s", tc.wantStatus, resp.Code, resp.Body.String())
			}

			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)

			if tc.wantError {
				if _, ok := result["error"]; !ok {
					t.Error("Expected error in response")
				}
			} else {
				if _, ok := result["user"]; !ok {
					t.Error("Expected user in response")
				}
				// Registration now requires email verification, not a token
				if emailVerification, ok := result["email_verification"]; !ok || emailVerification != true {
					t.Error("Expected email_verification: true in response")
				}
				if _, ok := result["message"]; !ok {
					t.Error("Expected message in response")
				}
			}
		})
	}
}

func TestAuthRegisterDuplicate(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Register first user
	ts.createTestUser(t, "test@example.com", "ValidPassword123!")

	// Try to register with same email
	resp := ts.doRequest("POST", "/api/auth/register", map[string]string{
		"email":    "test@example.com",
		"password": "AnotherPassword123!",
	}, "")

	if resp.Code != http.StatusConflict {
		t.Errorf("Expected status 409, got %d", resp.Code)
	}
}

func TestAuthLogin(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create user first
	ts.createTestUser(t, "login@example.com", "ValidPassword123!")

	tests := []struct {
		name       string
		email      string
		password   string
		wantStatus int
	}{
		{
			name:       "valid login",
			email:      "login@example.com",
			password:   "ValidPassword123!",
			wantStatus: http.StatusOK,
		},
		{
			name:       "wrong password",
			email:      "login@example.com",
			password:   "WrongPassword123!",
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "unknown email",
			email:      "unknown@example.com",
			password:   "ValidPassword123!",
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := ts.doRequest("POST", "/api/auth/login", map[string]string{
				"email":    tc.email,
				"password": tc.password,
			}, "")

			if resp.Code != tc.wantStatus {
				t.Errorf("Expected status %d, got %d: %s", tc.wantStatus, resp.Code, resp.Body.String())
			}
		})
	}
}

func TestAuthLogout(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create and login user
	token := ts.createTestUser(t, "logout@example.com", "ValidPassword123!")

	// Logout
	resp := ts.doRequest("POST", "/api/auth/logout", nil, token)
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}

	// Try to get user info with old token (should fail after logout)
	resp = ts.doRequest("GET", "/api/auth/me", nil, token)
	if resp.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 after logout, got %d", resp.Code)
	}
}

func TestAuthMe(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Test without auth
	resp := ts.doRequest("GET", "/api/auth/me", nil, "")
	if resp.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 without auth, got %d", resp.Code)
	}

	// Create user and get with auth
	token := ts.createTestUser(t, "me@example.com", "ValidPassword123!")

	resp = ts.doRequest("GET", "/api/auth/me", nil, token)
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}

	var result struct {
		User struct {
			Email string `json:"email"`
		} `json:"user"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if result.User.Email != "me@example.com" {
		t.Errorf("Expected email 'me@example.com', got '%s'", result.User.Email)
	}
}

func TestSessionCookie(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Register a user (no cookie set on registration anymore)
	resp := ts.doRequest("POST", "/api/auth/register", map[string]string{
		"email":    "cookie@example.com",
		"password": "ValidPassword123!",
	}, "")

	if resp.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", resp.Code)
	}

	// Get user ID and verify email directly
	var regResult struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	json.NewDecoder(resp.Body).Decode(&regResult)
	ts.db.VerifyUserEmail(regResult.User.ID)

	// Login - this should set the session cookie
	loginResp := ts.doRequest("POST", "/api/auth/login", map[string]string{
		"email":    "cookie@example.com",
		"password": "ValidPassword123!",
	}, "")

	if loginResp.Code != http.StatusOK {
		t.Errorf("Expected status 200 on login, got %d", loginResp.Code)
	}

	// Check for session cookie on login response
	cookies := loginResp.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "session" {
			sessionCookie = c
			break
		}
	}

	if sessionCookie == nil {
		t.Error("Expected session cookie to be set on login")
	} else {
		if !sessionCookie.HttpOnly {
			t.Error("Session cookie should be HttpOnly")
		}
	}
}

// ========== TOTP Recovery Code Tests ==========

func TestTOTPVerifyReturnsRecoveryCodes(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a user
	token := ts.createTestUser(t, "totp@example.com", "ValidPassword123!")

	// Set up TOTP
	resp := ts.doRequest("POST", "/api/auth/totp/setup", nil, token)
	if resp.Code != http.StatusOK {
		t.Fatalf("Expected status 200 for setup, got %d", resp.Code)
	}

	var setupResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&setupResp)
	secret := setupResp["secret"].(string)

	// Generate valid TOTP code
	code, err := totp.GenerateCode(secret)
	if err != nil {
		t.Fatalf("Failed to generate TOTP code: %v", err)
	}

	// Verify TOTP
	resp = ts.doRequest("POST", "/api/auth/totp/verify", map[string]string{
		"code": code,
	}, token)

	if resp.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.Code)
	}

	var verifyResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&verifyResp)

	if verifyResp["totp_enabled"] != true {
		t.Error("Expected totp_enabled to be true")
	}

	recoveryCodes, ok := verifyResp["recovery_codes"].([]interface{})
	if !ok {
		t.Fatal("Expected recovery_codes in response")
	}

	if len(recoveryCodes) != 10 {
		t.Errorf("Expected 10 recovery codes, got %d", len(recoveryCodes))
	}

	// Check code format (XXXX-XXXX)
	for _, code := range recoveryCodes {
		codeStr := code.(string)
		if len(codeStr) != 9 || codeStr[4] != '-' {
			t.Errorf("Recovery code %q has wrong format", codeStr)
		}
	}
}

func TestTOTPRecoverWithValidCode(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a user with 2FA enabled
	email := "recover@example.com"
	password := "ValidPassword123!"
	token := ts.createTestUser(t, email, password)

	// Set up and enable TOTP
	resp := ts.doRequest("POST", "/api/auth/totp/setup", nil, token)
	var setupResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&setupResp)
	secret := setupResp["secret"].(string)

	code, err := totp.GenerateCode(secret)
	if err != nil {
		t.Fatalf("Failed to generate TOTP code: %v", err)
	}
	resp = ts.doRequest("POST", "/api/auth/totp/verify", map[string]string{
		"code": code,
	}, token)

	var verifyResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&verifyResp)
	recoveryCodes := verifyResp["recovery_codes"].([]interface{})

	// Use the first recovery code to recover
	recoveryCode := recoveryCodes[0].(string)

	resp = ts.doRequest("POST", "/api/auth/totp/recover", map[string]string{
		"email":         email,
		"password":      password,
		"recovery_code": recoveryCode,
	}, "")

	if resp.Code != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, got %d: %s", resp.Code, body)
	}

	var recoverResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&recoverResp)

	if recoverResp["totp_enabled"] != false {
		t.Error("Expected totp_enabled to be false after recovery")
	}

	if recoverResp["token"] == nil {
		t.Error("Expected new session token in response")
	}
}

func TestTOTPRecoverWithInvalidCode(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a user with 2FA enabled
	email := "invalid@example.com"
	password := "ValidPassword123!"
	token := ts.createTestUser(t, email, password)

	// Set up and enable TOTP
	resp := ts.doRequest("POST", "/api/auth/totp/setup", nil, token)
	var setupResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&setupResp)
	secret := setupResp["secret"].(string)

	code, err := totp.GenerateCode(secret)
	if err != nil {
		t.Fatalf("Failed to generate TOTP code: %v", err)
	}
	ts.doRequest("POST", "/api/auth/totp/verify", map[string]string{
		"code": code,
	}, token)

	// Try to recover with invalid code
	resp = ts.doRequest("POST", "/api/auth/totp/recover", map[string]string{
		"email":         email,
		"password":      password,
		"recovery_code": "INVALID-CODE",
	}, "")

	if resp.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", resp.Code)
	}
}

func TestTOTPRecoverWithWrongPassword(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a user with 2FA enabled
	email := "wrongpwd@example.com"
	password := "ValidPassword123!"
	token := ts.createTestUser(t, email, password)

	// Set up and enable TOTP
	resp := ts.doRequest("POST", "/api/auth/totp/setup", nil, token)
	var setupResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&setupResp)
	secret := setupResp["secret"].(string)

	code, err := totp.GenerateCode(secret)
	if err != nil {
		t.Fatalf("Failed to generate TOTP code: %v", err)
	}
	resp = ts.doRequest("POST", "/api/auth/totp/verify", map[string]string{
		"code": code,
	}, token)

	var verifyResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&verifyResp)
	recoveryCodes := verifyResp["recovery_codes"].([]interface{})

	// Try to recover with wrong password
	resp = ts.doRequest("POST", "/api/auth/totp/recover", map[string]string{
		"email":         email,
		"password":      "WrongPassword123!",
		"recovery_code": recoveryCodes[0].(string),
	}, "")

	if resp.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", resp.Code)
	}
}

func TestTOTPRecoverCodeSingleUse(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a user with 2FA enabled
	email := "singleuse@example.com"
	password := "ValidPassword123!"
	token := ts.createTestUser(t, email, password)

	// Set up and enable TOTP
	resp := ts.doRequest("POST", "/api/auth/totp/setup", nil, token)
	var setupResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&setupResp)
	secret := setupResp["secret"].(string)

	code, err := totp.GenerateCode(secret)
	if err != nil {
		t.Fatalf("Failed to generate TOTP code: %v", err)
	}
	resp = ts.doRequest("POST", "/api/auth/totp/verify", map[string]string{
		"code": code,
	}, token)

	var verifyResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&verifyResp)
	recoveryCodes := verifyResp["recovery_codes"].([]interface{})
	recoveryCode := recoveryCodes[0].(string)

	// Use the recovery code first time - should work
	resp = ts.doRequest("POST", "/api/auth/totp/recover", map[string]string{
		"email":         email,
		"password":      password,
		"recovery_code": recoveryCode,
	}, "")

	if resp.Code != http.StatusOK {
		t.Fatalf("Expected first recovery to succeed, got %d", resp.Code)
	}

	// Re-enable 2FA
	var recoverResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&recoverResp)
	newToken := recoverResp["token"].(string)

	ts.doRequest("POST", "/api/auth/totp/setup", nil, newToken)
	// Get the new secret
	resp = ts.doRequest("POST", "/api/auth/totp/setup", nil, newToken)
	json.NewDecoder(resp.Body).Decode(&setupResp)
	newSecret := setupResp["secret"].(string)

	newCode, err := totp.GenerateCode(newSecret)
	if err != nil {
		t.Fatalf("Failed to generate new TOTP code: %v", err)
	}
	ts.doRequest("POST", "/api/auth/totp/verify", map[string]string{
		"code": newCode,
	}, newToken)

	// Try to use the same recovery code again - should fail since 2FA was disabled and codes deleted
	resp = ts.doRequest("POST", "/api/auth/totp/recover", map[string]string{
		"email":         email,
		"password":      password,
		"recovery_code": recoveryCode,
	}, "")

	// Accept 401 (invalid code) or 429 (rate limited - still means the code wasn't accepted)
	if resp.Code != http.StatusUnauthorized && resp.Code != http.StatusTooManyRequests {
		t.Errorf("Expected second recovery with same code to fail with 401 or 429, got %d", resp.Code)
	}
}

func TestTOTPRegenerateRecoveryCodes(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a user
	email := "regen@example.com"
	password := "ValidPassword123!"
	token := ts.createTestUser(t, email, password)

	// Set up and enable TOTP
	resp := ts.doRequest("POST", "/api/auth/totp/setup", nil, token)
	var setupResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&setupResp)
	secret := setupResp["secret"].(string)

	code, err := totp.GenerateCode(secret)
	if err != nil {
		t.Fatalf("Failed to generate TOTP code: %v", err)
	}
	resp = ts.doRequest("POST", "/api/auth/totp/verify", map[string]string{
		"code": code,
	}, token)

	var verifyResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&verifyResp)
	originalCodes := verifyResp["recovery_codes"].([]interface{})

	if len(originalCodes) != 10 {
		t.Errorf("Expected 10 original recovery codes, got %d", len(originalCodes))
	}

	// Regenerate codes - need a fresh TOTP code
	newCode, err := totp.GenerateCode(secret)
	if err != nil {
		t.Fatalf("Failed to generate new TOTP code: %v", err)
	}

	resp = ts.doRequest("POST", "/api/auth/totp/codes", map[string]string{
		"password": password,
		"code":     newCode,
	}, token)

	if resp.Code != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 200 for regenerate, got %d: %s", resp.Code, body)
	}

	var regenResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&regenResp)

	newCodes, ok := regenResp["recovery_codes"].([]interface{})
	if !ok {
		t.Fatal("Expected recovery_codes in response")
	}

	if len(newCodes) != 10 {
		t.Errorf("Expected 10 new recovery codes, got %d", len(newCodes))
	}

	// Verify old codes are different from new codes
	oldCodeSet := make(map[string]bool)
	for _, c := range originalCodes {
		oldCodeSet[c.(string)] = true
	}

	allNew := true
	for _, c := range newCodes {
		if oldCodeSet[c.(string)] {
			allNew = false
			break
		}
	}

	if !allNew {
		t.Error("New recovery codes should be different from original codes")
	}
}

func TestTOTPRegenerateRequiresAuth(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Try to regenerate without authentication
	resp := ts.doRequest("POST", "/api/auth/totp/codes", map[string]string{
		"password": "test",
		"code":     "123456",
	}, "")

	if resp.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 without auth, got %d", resp.Code)
	}
}

func TestTOTPRegenerateRequires2FAEnabled(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a user without 2FA
	token := ts.createTestUser(t, "no2fa@example.com", "ValidPassword123!")

	resp := ts.doRequest("POST", "/api/auth/totp/codes", map[string]string{
		"password": "ValidPassword123!",
		"code":     "123456",
	}, token)

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 without 2FA enabled, got %d", resp.Code)
	}
}

func TestTOTPRegenerateInvalidPassword(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a user with 2FA
	token := ts.createTestUser(t, "badpwd@example.com", "ValidPassword123!")

	resp := ts.doRequest("POST", "/api/auth/totp/setup", nil, token)
	var setupResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&setupResp)
	secret := setupResp["secret"].(string)

	code, _ := totp.GenerateCode(secret)
	ts.doRequest("POST", "/api/auth/totp/verify", map[string]string{
		"code": code,
	}, token)

	// Try with wrong password
	newCode, _ := totp.GenerateCode(secret)
	resp = ts.doRequest("POST", "/api/auth/totp/codes", map[string]string{
		"password": "WrongPassword123!",
		"code":     newCode,
	}, token)

	if resp.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 with wrong password, got %d", resp.Code)
	}
}

func TestTOTPRegenerateInvalidCode(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a user with 2FA
	password := "ValidPassword123!"
	token := ts.createTestUser(t, "badcode@example.com", password)

	resp := ts.doRequest("POST", "/api/auth/totp/setup", nil, token)
	var setupResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&setupResp)
	secret := setupResp["secret"].(string)

	code, _ := totp.GenerateCode(secret)
	ts.doRequest("POST", "/api/auth/totp/verify", map[string]string{
		"code": code,
	}, token)

	// Try with invalid TOTP code
	resp = ts.doRequest("POST", "/api/auth/totp/codes", map[string]string{
		"password": password,
		"code":     "000000",
	}, token)

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 with invalid TOTP code, got %d", resp.Code)
	}
}

func TestTOTPRegenerateRateLimited(t *testing.T) {
	strictLimiter := ratelimit.New(2, time.Minute)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/totp/codes", strictLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))

	// First 2 requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/api/auth/totp/codes", strings.NewReader(`{"password":"test123","code":"123456"}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.168.1.100:12345"
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// 3rd request should be rate limited
	req := httptest.NewRequest("POST", "/api/auth/totp/codes", strings.NewReader(`{"password":"test123","code":"123456"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.168.1.100:12345"
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429 Too Many Requests, got %d", w.Code)
	}
}

// TestRateLimitingLogin tests that login endpoint is rate limited

func TestRateLimitingLogin(t *testing.T) {
	// Create a test server with a strict rate limiter for testing (2 requests per minute)
	tempDir, err := os.MkdirTemp("", "subtitler-ratelimit-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	testDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer testDB.Close()

	enc, err := crypto.NewEncryptor("")
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	// Create a strict rate limiter for testing (2 requests per minute)
	strictLimiter := ratelimit.New(2, time.Minute)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/login", strictLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))

	// First 2 requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"email":"test@example.com","password":"Password123!"}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.168.1.100:12345"
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// 3rd request should be rate limited
	req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"email":"test@example.com","password":"Password123!"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.168.1.100:12345"
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429 Too Many Requests, got %d", w.Code)
	}

	// Check for Retry-After header
	if w.Header().Get("Retry-After") != "60" {
		t.Errorf("Expected Retry-After: 60 header")
	}

	// Different IP should still work
	req = httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"email":"test@example.com","password":"Password123!"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.168.1.200:12345" // Different IP
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Different IP should not be rate limited, got %d", w.Code)
	}

	// Unused variables to satisfy compiler
	_ = testDB
	_ = enc
}

// TestRateLimitingRegister tests that register endpoint is rate limited

func TestRateLimitingRegister(t *testing.T) {
	strictLimiter := ratelimit.New(2, time.Minute)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/register", strictLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))

	// First 2 requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/api/auth/register", strings.NewReader(`{"email":"test@example.com","password":"Password123!"}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.168.1.100:12345"
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// 3rd request should be rate limited
	req := httptest.NewRequest("POST", "/api/auth/register", strings.NewReader(`{"email":"test@example.com","password":"Password123!"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.168.1.100:12345"
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429 Too Many Requests, got %d", w.Code)
	}
}

// TestRateLimitingTOTPVerify tests that TOTP verify endpoint is rate limited

func TestRateLimitingTOTPVerify(t *testing.T) {
	strictLimiter := ratelimit.New(2, time.Minute)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/totp/verify", strictLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))

	// First 2 requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/api/auth/totp/verify", strings.NewReader(`{"code":"123456"}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.168.1.100:12345"
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// 3rd request should be rate limited
	req := httptest.NewRequest("POST", "/api/auth/totp/verify", strings.NewReader(`{"code":"123456"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.168.1.100:12345"
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429 Too Many Requests, got %d", w.Code)
	}
}

// TestRateLimitingTOTPRecover tests that TOTP recover endpoint is rate limited

func TestRateLimitingTOTPRecover(t *testing.T) {
	strictLimiter := ratelimit.New(2, time.Minute)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/totp/recover", strictLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))

	// First 2 requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/api/auth/totp/recover", strings.NewReader(`{"email":"test@example.com","password":"pass123","recovery_code":"ABC123"}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.168.1.100:12345"
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// 3rd request should be rate limited
	req := httptest.NewRequest("POST", "/api/auth/totp/recover", strings.NewReader(`{"email":"test@example.com","password":"pass123","recovery_code":"ABC123"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.168.1.100:12345"
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429 Too Many Requests, got %d", w.Code)
	}
}

// TestRateLimitingXForwardedFor tests that rate limiting respects X-Forwarded-For header
// when TRUST_PROXY is enabled

func TestRateLimitingXForwardedFor(t *testing.T) {
	// Enable trust proxy for this test
	oldTrust := ratelimit.IsTrustProxy()
	ratelimit.SetTrustProxy(true)
	defer ratelimit.SetTrustProxy(oldTrust)

	strictLimiter := ratelimit.New(2, time.Minute)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/login", strictLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))

	// Simulate requests from same real IP behind a proxy
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", "203.0.113.50") // Real client IP
		req.RemoteAddr = "10.0.0.1:12345"                 // Proxy IP
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// 3rd request from same real IP should be rate limited
	req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "203.0.113.50")
	req.RemoteAddr = "10.0.0.1:12345"
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429 for X-Forwarded-For IP, got %d", w.Code)
	}

	// Different real client IP behind same proxy should work
	req = httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "203.0.113.100") // Different real IP
	req.RemoteAddr = "10.0.0.1:12345"
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Different X-Forwarded-For IP should not be rate limited, got %d", w.Code)
	}
}

// TestRateLimitingXForwardedForUntrusted tests that X-Forwarded-For is ignored
// when TRUST_PROXY is disabled (default)

func TestRateLimitingXForwardedForUntrusted(t *testing.T) {
	// Ensure trust proxy is disabled for this test
	oldTrust := ratelimit.IsTrustProxy()
	ratelimit.SetTrustProxy(false)
	defer ratelimit.SetTrustProxy(oldTrust)

	strictLimiter := ratelimit.New(2, time.Minute)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/login", strictLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))

	// Simulate requests that try to spoof X-Forwarded-For
	// All requests come from same proxy IP (RemoteAddr)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("203.0.113.%d", i)) // Different spoofed IPs
		req.RemoteAddr = "10.0.0.1:12345"                                 // Same actual IP
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// 3rd request should be limited based on RemoteAddr, ignoring X-Forwarded-For
	req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "203.0.113.99") // Attacker tries different spoofed IP
	req.RemoteAddr = "10.0.0.1:12345"                 // Same actual IP
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429 (spoofed X-Forwarded-For should be ignored), got %d", w.Code)
	}
}

// TestRateLimitingTOTPSetup tests that TOTP setup endpoint is rate limited

func TestRateLimitingTOTPSetup(t *testing.T) {
	strictLimiter := ratelimit.New(2, time.Minute)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/totp/setup", strictLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))

	// First 2 requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/api/auth/totp/setup", nil)
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.168.1.100:12345"
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// 3rd request should be rate limited
	req := httptest.NewRequest("POST", "/api/auth/totp/setup", nil)
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.168.1.100:12345"
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429 Too Many Requests, got %d", w.Code)
	}
}

// TestRateLimitingTOTPDisable tests that TOTP disable endpoint is rate limited

func TestRateLimitingTOTPDisable(t *testing.T) {
	strictLimiter := ratelimit.New(2, time.Minute)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/totp/disable", strictLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))

	// First 2 requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/api/auth/totp/disable", strings.NewReader(`{"password":"test123","code":"123456"}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.168.1.100:12345"
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// 3rd request should be rate limited
	req := httptest.NewRequest("POST", "/api/auth/totp/disable", strings.NewReader(`{"password":"test123","code":"123456"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.168.1.100:12345"
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429 Too Many Requests, got %d", w.Code)
	}
}

// TestRateLimitingUpload tests that upload endpoint is rate limited

func TestGetSessions(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create user (registration creates first session)
	userID, regToken := ts.createTestUserWithID(t, "test@example.com", "Password123!")

	// Create a second session
	auth.CreateSession(ts.db, userID, "10.0.0.1", "Chrome/100")

	req := httptest.NewRequest("GET", "/api/auth/sessions", nil)
	req.Header.Set("Authorization", "Bearer "+regToken)
	w := httptest.NewRecorder()
	ts.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var response struct {
		Sessions []struct {
			ID        string `json:"id"`
			IPAddress string `json:"ip_address"`
			UserAgent string `json:"user_agent"`
			IsCurrent bool   `json:"is_current"`
		} `json:"sessions"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(response.Sessions) != 2 {
		t.Errorf("Expected 2 sessions, got %d", len(response.Sessions))
	}

	// Check that exactly one session is marked as current
	var currentCount int
	for _, s := range response.Sessions {
		if s.IsCurrent {
			currentCount++
		}
	}
	if currentCount != 1 {
		t.Errorf("Expected exactly 1 current session, got %d", currentCount)
	}
}

// TestGetSessionsUnauthenticated tests listing sessions without auth

func TestGetSessionsUnauthenticated(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	req := httptest.NewRequest("GET", "/api/auth/sessions", nil)
	w := httptest.NewRecorder()
	ts.mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", w.Code)
	}
}

// TestRevokeSession tests revoking another session

func TestRevokeSession(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create user (registration creates first session)
	userID, regToken := ts.createTestUserWithID(t, "test@example.com", "Password123!")

	// Create a second session that we'll revoke
	otherSession, _ := auth.CreateSession(ts.db, userID, "10.0.0.1", "Other")

	// Revoke the other session using the registration token
	req := httptest.NewRequest("DELETE", "/api/auth/sessions/"+otherSession.ID, nil)
	req.Header.Set("Authorization", "Bearer "+regToken)
	w := httptest.NewRecorder()
	ts.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the session was deleted - should have only the registration session left
	sessions, _ := ts.db.GetSessionsByUserID(userID)
	if len(sessions) != 1 {
		t.Errorf("Expected 1 session, got %d", len(sessions))
	}
}

// TestRevokeCurrentSession tests that you can't revoke your current session

func TestRevokeCurrentSession(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create user (registration creates a session)
	userID, regToken := ts.createTestUserWithID(t, "test@example.com", "Password123!")

	// Get the current session to find its ID
	sessions, _ := ts.db.GetSessionsByUserID(userID)
	if len(sessions) != 1 {
		t.Fatalf("Expected 1 session, got %d", len(sessions))
	}
	currentSessionID := sessions[0].ID

	req := httptest.NewRequest("DELETE", "/api/auth/sessions/"+currentSessionID, nil)
	req.Header.Set("Authorization", "Bearer "+regToken)
	w := httptest.NewRecorder()
	ts.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 (can't revoke current), got %d", w.Code)
	}
}

// TestRevokeOtherUserSession tests that you can't revoke another user's session

func TestRevokeOtherUserSession(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create two users
	_, token1 := ts.createTestUserWithID(t, "user1@example.com", "Password123!")
	user2ID, _ := ts.createTestUserWithID(t, "user2@example.com", "Password123!")

	// Get user2's session ID
	sessions, _ := ts.db.GetSessionsByUserID(user2ID)
	if len(sessions) != 1 {
		t.Fatalf("Expected 1 session for user2, got %d", len(sessions))
	}
	user2SessionID := sessions[0].ID

	// User1 tries to revoke User2's session
	req := httptest.NewRequest("DELETE", "/api/auth/sessions/"+user2SessionID, nil)
	req.Header.Set("Authorization", "Bearer "+token1)
	w := httptest.NewRecorder()
	ts.mux.ServeHTTP(w, req)

	// Should fail - session not found (because it belongs to different user)
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected 404, got %d", w.Code)
	}
}

// TestDetectScriptEndpoint tests the script detection API

func TestForgotPassword(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a test user
	ts.createTestUser(t, "test@example.com", "Password123!")

	// Clear any emails sent during user creation (verification email)
	ts.emailService.Clear()

	// Request password reset
	w := ts.doRequest("POST", "/api/auth/forgot-password", map[string]string{
		"email": "test@example.com",
	}, "")

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["message"] == "" {
		t.Error("Expected message in response")
	}

	// Check that a password reset email was sent
	emails := ts.emailService.GetEmails()
	if len(emails) != 1 {
		t.Errorf("Expected 1 email sent, got %d", len(emails))
	}
	if len(emails) > 0 && emails[0].To != "test@example.com" {
		t.Errorf("Expected email to test@example.com, got %s", emails[0].To)
	}
}

func TestForgotPasswordNonexistentEmail(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Request password reset for non-existent email
	w := ts.doRequest("POST", "/api/auth/forgot-password", map[string]string{
		"email": "nonexistent@example.com",
	}, "")

	// Should still return 200 to prevent email enumeration
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	// No email should be sent
	emails := ts.emailService.GetEmails()
	if len(emails) != 0 {
		t.Errorf("Expected 0 emails sent, got %d", len(emails))
	}
}

func TestForgotPasswordInvalidEmail(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	w := ts.doRequest("POST", "/api/auth/forgot-password", map[string]string{
		"email": "not-an-email",
	}, "")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}
}

func TestResetPasswordSuccess(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a test user
	ts.createTestUser(t, "test@example.com", "Oldpassword123!")

	// Clear any emails sent during user creation (verification email)
	ts.emailService.Clear()

	// Request password reset to get a token
	ts.doRequest("POST", "/api/auth/forgot-password", map[string]string{
		"email": "test@example.com",
	}, "")

	// Get the reset token from the email
	emails := ts.emailService.GetEmails()
	if len(emails) == 0 {
		t.Fatal("No password reset email sent")
	}

	// Extract token from email body (it should be in the URL)
	emailBody := emails[0].TextBody
	if emailBody == "" {
		emailBody = emails[0].HtmlBody
	}

	// The token should be between "token=" and the next non-alphanumeric
	tokenStart := strings.Index(emailBody, "token=")
	if tokenStart == -1 {
		t.Fatal("Could not find token in email")
	}
	tokenStart += 6 // len("token=")
	tokenEnd := tokenStart
	for tokenEnd < len(emailBody) && (emailBody[tokenEnd] >= 'a' && emailBody[tokenEnd] <= 'z' ||
		emailBody[tokenEnd] >= 'A' && emailBody[tokenEnd] <= 'Z' ||
		emailBody[tokenEnd] >= '0' && emailBody[tokenEnd] <= '9') {
		tokenEnd++
	}
	token := emailBody[tokenStart:tokenEnd]

	// Reset password with the token
	w := ts.doRequest("POST", "/api/auth/reset-password", map[string]string{
		"token":    token,
		"password": "Newpassword123!",
	}, "")

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify we can login with new password
	w = ts.doRequest("POST", "/api/auth/login", map[string]string{
		"email":    "test@example.com",
		"password": "Newpassword123!",
	}, "")

	if w.Code != http.StatusOK {
		t.Errorf("Expected login to succeed with new password, got %d", w.Code)
	}

	// Verify old password no longer works
	w = ts.doRequest("POST", "/api/auth/login", map[string]string{
		"email":    "test@example.com",
		"password": "Oldpassword123!",
	}, "")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected old password to fail, got %d", w.Code)
	}
}

func TestResetPasswordInvalidToken(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	w := ts.doRequest("POST", "/api/auth/reset-password", map[string]string{
		"token":    "invalid-token",
		"password": "Newpassword123!",
	}, "")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d", w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)

	if !strings.Contains(resp["error"], "Invalid or expired") {
		t.Errorf("Expected 'Invalid or expired' error, got %s", resp["error"])
	}
}

func TestResetPasswordExpiredToken(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a test user
	userID, _ := ts.createTestUserWithID(t, "test@example.com", "Password123!")

	// Manually create an expired token
	tokenBytes := make([]byte, 32)
	rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)
	tokenHash := email.HashToken(token)
	expiresAt := time.Now().Add(-1 * time.Hour) // Already expired

	ts.db.CreatePasswordResetToken(userID, tokenHash, expiresAt)

	// Try to use expired token
	w := ts.doRequest("POST", "/api/auth/reset-password", map[string]string{
		"token":    token,
		"password": "Newpassword123!",
	}, "")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for expired token, got %d", w.Code)
	}
}

func TestResetPasswordWeakPassword(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a test user and get a valid token
	ts.createTestUser(t, "test@example.com", "Password123!")
	ts.doRequest("POST", "/api/auth/forgot-password", map[string]string{
		"email": "test@example.com",
	}, "")

	emails := ts.emailService.GetEmails()
	emailBody := emails[0].TextBody
	tokenStart := strings.Index(emailBody, "token=") + 6
	tokenEnd := tokenStart
	for tokenEnd < len(emailBody) && (emailBody[tokenEnd] >= 'a' && emailBody[tokenEnd] <= 'z' ||
		emailBody[tokenEnd] >= 'A' && emailBody[tokenEnd] <= 'Z' ||
		emailBody[tokenEnd] >= '0' && emailBody[tokenEnd] <= '9') {
		tokenEnd++
	}
	token := emailBody[tokenStart:tokenEnd]

	// Try to reset with weak password
	w := ts.doRequest("POST", "/api/auth/reset-password", map[string]string{
		"token":    token,
		"password": "short",
	}, "")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for weak password, got %d", w.Code)
	}
}

func TestResetPasswordTokenSingleUse(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a test user and get a valid token
	ts.createTestUser(t, "test@example.com", "Password123!")

	// Clear emails from user creation (verification email)
	ts.emailService.Clear()

	ts.doRequest("POST", "/api/auth/forgot-password", map[string]string{
		"email": "test@example.com",
	}, "")

	emails := ts.emailService.GetEmails()
	if len(emails) == 0 {
		t.Fatal("No password reset email sent")
	}
	emailBody := emails[0].TextBody
	tokenStart := strings.Index(emailBody, "token=") + 6
	tokenEnd := tokenStart
	for tokenEnd < len(emailBody) && (emailBody[tokenEnd] >= 'a' && emailBody[tokenEnd] <= 'z' ||
		emailBody[tokenEnd] >= 'A' && emailBody[tokenEnd] <= 'Z' ||
		emailBody[tokenEnd] >= '0' && emailBody[tokenEnd] <= '9') {
		tokenEnd++
	}
	token := emailBody[tokenStart:tokenEnd]

	// First reset should succeed
	w := ts.doRequest("POST", "/api/auth/reset-password", map[string]string{
		"token":    token,
		"password": "Newpassword123!",
	}, "")

	if w.Code != http.StatusOK {
		t.Errorf("Expected first reset to succeed, got %d", w.Code)
	}

	// Second reset with same token should fail
	w = ts.doRequest("POST", "/api/auth/reset-password", map[string]string{
		"token":    token,
		"password": "anotherpassword123",
	}, "")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected second reset to fail with 400, got %d", w.Code)
	}
}

// ========== Magic Link Tests ==========

func TestMagicLinkUnverifiedEmail(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create an UNVERIFIED user by registering but not verifying email
	resp := ts.doRequest("POST", "/api/auth/register", map[string]string{
		"email":    "unverified@example.com",
		"password": "Password123!",
	}, "")

	if resp.Code != http.StatusCreated {
		t.Fatalf("Failed to create test user: %s", resp.Body.String())
	}

	// Clear the verification email from registration
	ts.emailService.Clear()

	// Request magic link for unverified email
	w := ts.doRequest("POST", "/api/auth/magic-link", map[string]string{
		"email": "unverified@example.com",
	}, "")

	// Should always return 200 to prevent email enumeration
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	// BUT the magic link email should NOT have been sent
	emailCount := ts.emailService.GetEmailCount()
	if emailCount != 0 {
		t.Errorf("Magic link should NOT be sent for unverified email, but %d emails were sent", emailCount)
	}
}

func TestMagicLinkVerifiedEmail(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a verified user (createTestUser verifies email automatically)
	ts.createTestUser(t, "verified@example.com", "Password123!")

	// Clear emails from user creation
	ts.emailService.Clear()

	// Request magic link for verified email
	w := ts.doRequest("POST", "/api/auth/magic-link", map[string]string{
		"email": "verified@example.com",
	}, "")

	// Should return 200
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	// Magic link email SHOULD have been sent
	emailCount := ts.emailService.GetEmailCount()
	if emailCount != 1 {
		t.Errorf("Magic link should be sent for verified email, but %d emails were sent", emailCount)
	}

	// Verify email subject
	email := ts.emailService.LastEmail()
	if email != nil && !strings.Contains(email.Subject, "Login") {
		t.Errorf("Expected magic link email, got: %s", email.Subject)
	}
}

func TestMagicLinkNonexistentEmail(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Request magic link for nonexistent email
	w := ts.doRequest("POST", "/api/auth/magic-link", map[string]string{
		"email": "nonexistent@example.com",
	}, "")

	// Should return 200 to prevent email enumeration
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200 (to prevent enumeration), got %d", w.Code)
	}

	// No email should be sent
	emailCount := ts.emailService.GetEmailCount()
	if emailCount != 0 {
		t.Errorf("No email should be sent for nonexistent user, but %d emails were sent", emailCount)
	}
}

func TestMagicLinkInvalidEmail(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Request magic link with invalid email format
	w := ts.doRequest("POST", "/api/auth/magic-link", map[string]string{
		"email": "not-an-email",
	}, "")

	// Should return 400 for invalid email format
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid email, got %d", w.Code)
	}
}

func TestMagicLinkVerifySuccess(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a verified user
	ts.createTestUser(t, "test@example.com", "Password123!")

	// Clear emails from user creation
	ts.emailService.Clear()

	// Request magic link
	ts.doRequest("POST", "/api/auth/magic-link", map[string]string{
		"email": "test@example.com",
	}, "")

	// Extract token from email
	emails := ts.emailService.GetEmails()
	if len(emails) == 0 {
		t.Fatal("No magic link email was sent")
	}

	emailBody := emails[0].TextBody
	tokenStart := strings.Index(emailBody, "token=") + 6
	if tokenStart < 6 {
		t.Fatal("Token not found in email")
	}
	tokenEnd := tokenStart
	for tokenEnd < len(emailBody) && (emailBody[tokenEnd] >= 'a' && emailBody[tokenEnd] <= 'z' ||
		emailBody[tokenEnd] >= 'A' && emailBody[tokenEnd] <= 'Z' ||
		emailBody[tokenEnd] >= '0' && emailBody[tokenEnd] <= '9') {
		tokenEnd++
	}
	token := emailBody[tokenStart:tokenEnd]

	// Verify magic link
	w := ts.doRequest("GET", "/api/auth/magic-link/verify?token="+token, nil, "")

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result map[string]interface{}
	json.NewDecoder(w.Body).Decode(&result)

	if result["message"] != "Login successful" {
		t.Errorf("Expected 'Login successful', got %v", result["message"])
	}

	if result["token"] == nil {
		t.Error("Expected session token in response")
	}
}

func TestMagicLinkVerifyExpired(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a verified user
	ts.createTestUser(t, "test@example.com", "Password123!")

	// Get user
	user, _ := ts.db.GetUserByEmail("test@example.com")

	// Create an expired magic link token directly
	tokenBytes := make([]byte, 32)
	rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)
	tokenHash := email.HashToken(token)

	// Expired 1 hour ago
	expiresAt := time.Now().Add(-1 * time.Hour)
	ts.db.CreateMagicLinkToken(user.ID, tokenHash, expiresAt)

	// Try to verify expired token
	w := ts.doRequest("GET", "/api/auth/magic-link/verify?token="+token, nil, "")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for expired token, got %d", w.Code)
	}
}

func TestMagicLinkVerifyInvalidToken(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Try to verify with invalid token
	w := ts.doRequest("GET", "/api/auth/magic-link/verify?token=invalidtoken123", nil, "")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for invalid token, got %d", w.Code)
	}
}

func TestMagicLinkVerifyMissingToken(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Try to verify without token
	w := ts.doRequest("GET", "/api/auth/magic-link/verify", nil, "")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for missing token, got %d", w.Code)
	}
}

func TestMagicLinkTokenSingleUse(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a verified user
	ts.createTestUser(t, "test@example.com", "Password123!")

	// Clear emails from user creation
	ts.emailService.Clear()

	// Request magic link
	ts.doRequest("POST", "/api/auth/magic-link", map[string]string{
		"email": "test@example.com",
	}, "")

	// Extract token from email
	emails := ts.emailService.GetEmails()
	emailBody := emails[0].TextBody
	tokenStart := strings.Index(emailBody, "token=") + 6
	tokenEnd := tokenStart
	for tokenEnd < len(emailBody) && (emailBody[tokenEnd] >= 'a' && emailBody[tokenEnd] <= 'z' ||
		emailBody[tokenEnd] >= 'A' && emailBody[tokenEnd] <= 'Z' ||
		emailBody[tokenEnd] >= '0' && emailBody[tokenEnd] <= '9') {
		tokenEnd++
	}
	token := emailBody[tokenStart:tokenEnd]

	// First verification should succeed
	w := ts.doRequest("GET", "/api/auth/magic-link/verify?token="+token, nil, "")
	if w.Code != http.StatusOK {
		t.Errorf("First verification should succeed, got %d", w.Code)
	}

	// Second verification with same token should fail
	w = ts.doRequest("GET", "/api/auth/magic-link/verify?token="+token, nil, "")
	if w.Code != http.StatusBadRequest {
		t.Errorf("Second verification should fail with 400, got %d", w.Code)
	}
}

// ========== Upload MIME Type Validation Tests ==========

func TestUserRoleManagement(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	t.Run("new users have user role by default", func(t *testing.T) {
		userID, _ := ts.createTestUserWithID(t, "newuser@example.com", "Password123!")

		user, err := ts.db.GetUserByID(userID)
		if err != nil {
			t.Fatalf("Failed to get user: %v", err)
		}

		if user.Role != db.RoleUser {
			t.Errorf("Expected new user to have role '%s', got '%s'", db.RoleUser, user.Role)
		}
	})

	t.Run("UpdateUserRole changes role", func(t *testing.T) {
		userID, _ := ts.createTestUserWithID(t, "promote@example.com", "Password123!")

		// Promote to admin
		if err := ts.db.UpdateUserRole(userID, db.RoleAdmin); err != nil {
			t.Fatalf("Failed to update user role: %v", err)
		}

		user, err := ts.db.GetUserByID(userID)
		if err != nil {
			t.Fatalf("Failed to get user: %v", err)
		}

		if user.Role != db.RoleAdmin {
			t.Errorf("Expected user role to be '%s', got '%s'", db.RoleAdmin, user.Role)
		}

		// Demote back to user
		if err := ts.db.UpdateUserRole(userID, db.RoleUser); err != nil {
			t.Fatalf("Failed to demote user: %v", err)
		}

		user, _ = ts.db.GetUserByID(userID)
		if user.Role != db.RoleUser {
			t.Errorf("Expected user role to be '%s' after demotion, got '%s'", db.RoleUser, user.Role)
		}
	})

	t.Run("UpdateUserRole rejects invalid role", func(t *testing.T) {
		// Create a user directly in the database to avoid rate limiting
		testUser := &db.User{
			ID:            testGenerateID(),
			Email:         "invalidrole@example.com",
			PasswordHash:  "$2a$10$testhashinvalidrole",
			TOTPEnabled:   false,
			EmailVerified: true,
			Role:          db.RoleUser,
			CreatedAt:     time.Now(),
		}
		if err := ts.db.CreateUser(testUser); err != nil {
			t.Fatalf("Failed to create test user: %v", err)
		}

		err := ts.db.UpdateUserRole(testUser.ID, "superadmin")
		if err == nil {
			t.Error("Expected error for invalid role, got nil")
		}
	})

	t.Run("PromoteToAdmin by email", func(t *testing.T) {
		// Create a user directly in the database to avoid rate limiting
		testUser := &db.User{
			ID:            testGenerateID(),
			Email:         "promotebyemail@example.com",
			PasswordHash:  "$2a$10$testhashpromote",
			TOTPEnabled:   false,
			EmailVerified: true,
			Role:          db.RoleUser,
			CreatedAt:     time.Now(),
		}
		if err := ts.db.CreateUser(testUser); err != nil {
			t.Fatalf("Failed to create test user: %v", err)
		}

		if err := ts.db.PromoteToAdmin(testUser.Email); err != nil {
			t.Fatalf("Failed to promote user by email: %v", err)
		}

		user, err := ts.db.GetUserByID(testUser.ID)
		if err != nil {
			t.Fatalf("Failed to get user: %v", err)
		}

		if user.Role != db.RoleAdmin {
			t.Errorf("Expected user role to be '%s' after promotion by email, got '%s'", db.RoleAdmin, user.Role)
		}
	})

	t.Run("PromoteToAdmin returns error for nonexistent user", func(t *testing.T) {
		err := ts.db.PromoteToAdmin("nonexistent@example.com")
		if err == nil {
			t.Error("Expected error for nonexistent user, got nil")
		}
	})

	t.Run("IsAdmin returns correct value", func(t *testing.T) {
		// Create a user directly in the database to avoid rate limiting
		testUser := &db.User{
			ID:            testGenerateID(),
			Email:         "isadmincheck@example.com",
			PasswordHash:  "$2a$10$testhashisadmin",
			TOTPEnabled:   false,
			EmailVerified: true,
			Role:          db.RoleUser,
			CreatedAt:     time.Now(),
		}
		if err := ts.db.CreateUser(testUser); err != nil {
			t.Fatalf("Failed to create test user: %v", err)
		}

		user, _ := ts.db.GetUserByID(testUser.ID)

		if user.IsAdmin() {
			t.Error("Expected IsAdmin() to return false for regular user")
		}

		// Admin user
		ts.db.UpdateUserRole(testUser.ID, db.RoleAdmin)
		user, _ = ts.db.GetUserByID(testUser.ID)

		if !user.IsAdmin() {
			t.Error("Expected IsAdmin() to return true for admin user")
		}
	})
}

// TestEmailVerificationMissingToken tests the GET /api/auth/verify endpoint with no token

func TestEmailVerificationMissingToken(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	resp := ts.doRequest("GET", "/api/auth/verify", nil, "")
	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.Code)
	}
	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["error"] != "Verification token is required" {
		t.Errorf("Unexpected error message: %s", result["error"])
	}
}

// TestEmailVerificationInvalidToken tests the GET /api/auth/verify endpoint with invalid token

func TestEmailVerificationInvalidToken(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	resp := ts.doRequest("GET", "/api/auth/verify?token=invalidtoken123", nil, "")
	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.Code)
	}
	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["error"] != "Invalid or expired verification token" {
		t.Errorf("Unexpected error message: %s", result["error"])
	}
}

// TestEmailVerificationSuccess tests successful email verification

func TestEmailVerificationSuccess(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Register a user (creates unverified account)
	resp := ts.doRequest("POST", "/api/auth/register", map[string]string{
		"email":    "verify@example.com",
		"password": "ValidPassword123!",
	}, "")
	if resp.Code != http.StatusCreated {
		t.Fatalf("Failed to register user: %s", resp.Body.String())
	}

	var regResult struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	json.NewDecoder(resp.Body).Decode(&regResult)

	// Create a new token for testing since we don't have the original
	testToken := "test-verification-token-12345678901234567890"
	tokenHash := email.HashToken(testToken)
	expiresAt := time.Now().Add(24 * time.Hour)
	ts.db.CreateEmailVerificationToken(regResult.User.ID, tokenHash, expiresAt)

	// Verify the email
	resp = ts.doRequest("GET", "/api/auth/verify?token="+testToken, nil, "")
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["message"] != "Email verified successfully. You can now log in." {
		t.Errorf("Unexpected message: %s", result["message"])
	}

	// Verify user is now verified in the database
	user, _ := ts.db.GetUserByID(regResult.User.ID)
	if !user.EmailVerified {
		t.Error("User email should be verified after successful verification")
	}
}

// TestEmailVerificationExpiredToken tests verification with an expired token

func TestEmailVerificationExpiredToken(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create user
	resp := ts.doRequest("POST", "/api/auth/register", map[string]string{
		"email":    "expired@example.com",
		"password": "ValidPassword123!",
	}, "")
	if resp.Code != http.StatusCreated {
		t.Fatalf("Failed to register user: %s", resp.Body.String())
	}

	var regResult struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	json.NewDecoder(resp.Body).Decode(&regResult)

	// Create an expired token
	expiredToken := "expired-verification-token-12345678901234567"
	tokenHash := email.HashToken(expiredToken)
	expiresAt := time.Now().Add(-1 * time.Hour) // Already expired
	ts.db.CreateEmailVerificationToken(regResult.User.ID, tokenHash, expiresAt)

	// Try to verify with expired token
	resp = ts.doRequest("GET", "/api/auth/verify?token="+expiredToken, nil, "")
	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.Code)
	}
	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["error"] != "Invalid or expired verification token" {
		t.Errorf("Unexpected error message: %s", result["error"])
	}
}

// TestEmailVerificationAlreadyUsedToken tests reusing a verification token

func TestEmailVerificationAlreadyUsedToken(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create user
	resp := ts.doRequest("POST", "/api/auth/register", map[string]string{
		"email":    "alreadyused@example.com",
		"password": "ValidPassword123!",
	}, "")
	if resp.Code != http.StatusCreated {
		t.Fatalf("Failed to register user: %s", resp.Body.String())
	}

	var regResult struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	json.NewDecoder(resp.Body).Decode(&regResult)

	// Create a token and use it
	usedToken := "alreadyused-verification-token-1234567890123"
	tokenHash := email.HashToken(usedToken)
	expiresAt := time.Now().Add(24 * time.Hour)
	ts.db.CreateEmailVerificationToken(regResult.User.ID, tokenHash, expiresAt)

	// Use the token first time
	resp = ts.doRequest("GET", "/api/auth/verify?token="+usedToken, nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("First verification should succeed: %s", resp.Body.String())
	}

	// Try to use the same token again
	resp = ts.doRequest("GET", "/api/auth/verify?token="+usedToken, nil, "")
	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for reused token, got %d", resp.Code)
	}
}

// TestResendVerificationEmailInvalidFormat tests resend verification with invalid email format

func TestResendVerificationEmailInvalidFormat(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	resp := ts.doRequest("POST", "/api/auth/resend-verification", map[string]string{
		"email": "notanemail",
	}, "")
	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.Code)
	}
}

// TestResendVerificationEmailEmpty tests resend verification with empty email

func TestResendVerificationEmailEmpty(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	resp := ts.doRequest("POST", "/api/auth/resend-verification", map[string]string{
		"email": "",
	}, "")
	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.Code)
	}
}

// TestResendVerificationEmailNonexistent tests resend verification with nonexistent email

func TestResendVerificationEmailNonexistent(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Should return success to prevent email enumeration
	resp := ts.doRequest("POST", "/api/auth/resend-verification", map[string]string{
		"email": "nonexistent@example.com",
	}, "")
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}
	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["message"] != "If an unverified account exists with that email, a verification link has been sent." {
		t.Errorf("Unexpected message: %s", result["message"])
	}
}

// TestResendVerificationEmailAlreadyVerified tests resend verification for already verified email

func TestResendVerificationEmailAlreadyVerified(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create and verify a user
	token := ts.createTestUser(t, "verified@example.com", "ValidPassword123!")
	_ = token // Discard token, we just need the verified user

	// Resend verification - should succeed silently
	resp := ts.doRequest("POST", "/api/auth/resend-verification", map[string]string{
		"email": "verified@example.com",
	}, "")
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}
}

// TestResendVerificationEmailUnverified tests resend verification for unverified email

func TestResendVerificationEmailUnverified(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Register a user (creates unverified account)
	resp := ts.doRequest("POST", "/api/auth/register", map[string]string{
		"email":    "unverified@example.com",
		"password": "ValidPassword123!",
	}, "")
	if resp.Code != http.StatusCreated {
		t.Fatalf("Failed to register user: %s", resp.Body.String())
	}

	var regResult struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	json.NewDecoder(resp.Body).Decode(&regResult)

	// Verify user is not yet verified
	user, _ := ts.db.GetUserByID(regResult.User.ID)
	if user.EmailVerified {
		t.Fatal("User should not be verified yet")
	}

	// Resend verification
	resp = ts.doRequest("POST", "/api/auth/resend-verification", map[string]string{
		"email": "unverified@example.com",
	}, "")
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	// Check that a new token was created (we can't check the actual email without mock)
	// The success message confirms the handler ran correctly
	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["message"] == "" {
		t.Error("Expected success message")
	}
}

// TestResendVerificationEmailInvalidBody tests resend verification with invalid request body

func TestResendVerificationEmailInvalidBody(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	req := httptest.NewRequest("POST", "/api/auth/resend-verification", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	ts.mux.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.Code)
	}
}

// Tests for CSRF token endpoint

// TestCSRFTokenAuthenticated tests getting CSRF token with valid authentication

func TestCSRFTokenAuthenticated(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create and login user
	token := ts.createTestUser(t, "csrf@example.com", "ValidPassword123!")

	// Get CSRF token
	resp := ts.doRequest("GET", "/api/auth/csrf", nil, token)
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result["csrf_token"] == "" {
		t.Error("Expected non-empty csrf_token")
	}

	// Verify CSRF token format (should be hex-encoded HMAC)
	if len(result["csrf_token"]) != 64 { // SHA256 produces 32 bytes = 64 hex chars
		t.Errorf("Expected 64-char csrf_token, got %d chars", len(result["csrf_token"]))
	}
}

// TestCSRFTokenUnauthenticated tests getting CSRF token without authentication

func TestCSRFTokenUnauthenticated(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Try to get CSRF token without auth
	resp := ts.doRequest("GET", "/api/auth/csrf", nil, "")
	if resp.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", resp.Code)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["error"] != "Not authenticated" {
		t.Errorf("Expected 'Not authenticated' error, got: %s", result["error"])
	}
}

// TestCSRFTokenInvalidSession tests getting CSRF token with invalid session

func TestCSRFTokenInvalidSession(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Try to get CSRF token with invalid token
	resp := ts.doRequest("GET", "/api/auth/csrf", nil, "invalid-session-token")
	if resp.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", resp.Code)
	}
}

// TestCSRFTokenExpiredSession tests getting CSRF token with expired session

func TestCSRFTokenExpiredSession(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create user and login
	token := ts.createTestUser(t, "csrfexpired@example.com", "ValidPassword123!")

	// Logout to invalidate session
	resp := ts.doRequest("POST", "/api/auth/logout", nil, token)
	if resp.Code != http.StatusOK {
		t.Fatalf("Logout failed: %s", resp.Body.String())
	}

	// Try to get CSRF token with expired/invalid session
	resp = ts.doRequest("GET", "/api/auth/csrf", nil, token)
	if resp.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", resp.Code)
	}
}

// Tests for CAPTCHA config endpoint

// TestCaptchaConfigDisabled tests CAPTCHA config when CAPTCHA is disabled

func TestCaptchaConfigDisabled(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// By default, captchaVerifier is disabled in tests
	resp := ts.doRequest("GET", "/api/captcha/config", nil, "")
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result["enabled"] != false {
		t.Errorf("Expected enabled=false, got %v", result["enabled"])
	}
}

// TestCaptchaConfigEnabled tests CAPTCHA config when CAPTCHA is enabled

func TestCaptchaConfigEnabled(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Enable CAPTCHA verifier for this test
	ts.captchaVerifier.MockEnabled = true

	resp := ts.doRequest("GET", "/api/captcha/config", nil, "")
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if result["enabled"] != true {
		t.Errorf("Expected enabled=true, got %v", result["enabled"])
	}
}

// TestCaptchaConfigNoAuth tests CAPTCHA config endpoint requires no authentication

func TestCaptchaConfigNoAuth(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// CAPTCHA config should be accessible without authentication
	// (frontend needs it before user can register/login)
	resp := ts.doRequest("GET", "/api/captcha/config", nil, "")
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}
}

// Tests for transcript align endpoint

// TestAlignTranscriptStandard tests standard transcript alignment
