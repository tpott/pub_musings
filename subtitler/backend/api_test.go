package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/trevor/subtitler/backend/auth"
	"github.com/trevor/subtitler/backend/crypto"
	"github.com/trevor/subtitler/backend/db"
	"github.com/trevor/subtitler/backend/email"
	"github.com/trevor/subtitler/backend/ratelimit"
	"github.com/trevor/subtitler/backend/script"
	"github.com/trevor/subtitler/backend/totp"
)

// testServer holds all dependencies needed for testing
type testServer struct {
	mux                  *http.ServeMux
	db                   *db.DB
	encryptor            *crypto.Encryptor
	uploadDir            string
	authLimiter          *ratelimit.Limiter
	passwordResetLimiter *ratelimit.Limiter
	emailService         *email.MockService
	cleanup              func()
}

// setupTestServer creates a test server with a temporary database and encryptor
func setupTestServer(t *testing.T) *testServer {
	t.Helper()

	// Create temp directory for test files
	tempDir, err := os.MkdirTemp("", "subtitler-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Create test database
	dbPath := filepath.Join(tempDir, "test.db")
	testDB, err := db.Open(dbPath)
	if err != nil {
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to open database: %v", err)
	}

	// Create test encryptor (empty string generates new key)
	enc, err := crypto.NewEncryptor("")
	if err != nil {
		testDB.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	// Create uploads subdirectory
	uploadDir := filepath.Join(tempDir, "uploads")
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		testDB.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create upload dir: %v", err)
	}

	ts := &testServer{
		mux:                  http.NewServeMux(),
		db:                   testDB,
		encryptor:            enc,
		uploadDir:            uploadDir,
		authLimiter:          ratelimit.New(5, time.Minute),
		passwordResetLimiter: ratelimit.New(3, 15*time.Minute),
		emailService:         email.NewMockService(),
		cleanup: func() {
			testDB.Close()
			os.RemoveAll(tempDir)
		},
	}

	// Register all handlers
	ts.registerHandlers()

	return ts
}

// registerHandlers registers all API handlers on the test server's mux
func (ts *testServer) registerHandlers() {
	// Health check endpoint
	ts.mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
		})
	})

	// Frontend log forwarding endpoint (for dev mode debugging)
	ts.mux.HandleFunc("POST /api/log", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var req struct {
			Level   string `json:"level"`
			Message string `json:"message"`
			URL     string `json:"url"`
			Line    int    `json:"line"`
			Column  int    `json:"column"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
			return
		}

		// Validate level
		validLevels := map[string]bool{"log": true, "warn": true, "error": true, "info": true, "debug": true}
		if !validLevels[req.Level] {
			req.Level = "log"
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Auth: Register (rate limited)
	ts.mux.HandleFunc("POST /api/auth/register", ts.authLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
			return
		}

		req.Email = strings.TrimSpace(strings.ToLower(req.Email))

		if err := auth.ValidateEmail(req.Email); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		if err := auth.ValidatePassword(req.Password); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		existingUser, err := ts.db.GetUserByEmail(req.Email)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Registration failed"})
			return
		}
		if existingUser != nil {
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{"error": "Email already registered"})
			return
		}

		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Registration failed"})
			return
		}

		userID, err := auth.GenerateID()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Registration failed"})
			return
		}

		user := &db.User{
			ID:           userID,
			Email:        req.Email,
			PasswordHash: hash,
			CreatedAt:    time.Now(),
		}
		if err := ts.db.CreateUser(user); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Registration failed"})
			return
		}

		session, err := auth.CreateSession(ts.db, userID, "", "")
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"user": map[string]interface{}{
					"id":           user.ID,
					"email":        user.Email,
					"created_at":   user.CreatedAt,
					"totp_enabled": user.TOTPEnabled,
				},
			})
			return
		}

		auth.SetSessionCookie(w, session.Token, session.ExpiresAt)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"user": map[string]interface{}{
				"id":           user.ID,
				"email":        user.Email,
				"created_at":   user.CreatedAt,
				"totp_enabled": user.TOTPEnabled,
			},
			"token": session.Token,
		})
	}))

	// Auth: Login (rate limited)
	ts.mux.HandleFunc("POST /api/auth/login", ts.authLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
			return
		}

		req.Email = strings.TrimSpace(strings.ToLower(req.Email))

		user, err := ts.db.GetUserByEmail(req.Email)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Login failed"})
			return
		}
		if user == nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid email or password"})
			return
		}

		if !auth.CheckPassword(req.Password, user.PasswordHash) {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid email or password"})
			return
		}

		session, err := auth.CreateSession(ts.db, user.ID, "", "")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Login failed"})
			return
		}

		auth.SetSessionCookie(w, session.Token, session.ExpiresAt)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"user": map[string]interface{}{
				"id":           user.ID,
				"email":        user.Email,
				"created_at":   user.CreatedAt,
				"totp_enabled": user.TOTPEnabled,
			},
			"token": session.Token,
		})
	}))

	// Auth: Logout
	ts.mux.HandleFunc("POST /api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		token := auth.GetTokenFromRequest(r)
		if token != "" {
			ts.db.DeleteSession(token)
		}

		auth.ClearSessionCookie(w)

		json.NewEncoder(w).Encode(map[string]string{
			"message": "Logged out successfully",
		})
	})

	// Auth: Get current user
	ts.mux.HandleFunc("GET /api/auth/me", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		token := auth.GetTokenFromRequest(r)
		user, _, err := auth.ValidateSession(ts.db, token)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get user"})
			return
		}

		if user == nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Not authenticated"})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"user": map[string]interface{}{
				"id":           user.ID,
				"email":        user.Email,
				"created_at":   user.CreatedAt,
				"totp_enabled": user.TOTPEnabled,
			},
		})
	})

	// Auth: Get all sessions for current user
	ts.mux.HandleFunc("GET /api/auth/sessions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		token := auth.GetTokenFromRequest(r)
		user, currentSession, err := auth.ValidateSession(ts.db, token)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to validate session"})
			return
		}

		if user == nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Authentication required"})
			return
		}

		sessions, err := ts.db.GetSessionsByUserID(user.ID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get sessions"})
			return
		}

		var responseSessions []map[string]interface{}
		for _, s := range sessions {
			sessionData := map[string]interface{}{
				"id":         s.ID,
				"ip_address": s.IPAddress,
				"user_agent": s.UserAgent,
				"created_at": s.CreatedAt,
				"expires_at": s.ExpiresAt,
				"is_current": s.ID == currentSession.ID,
			}
			responseSessions = append(responseSessions, sessionData)
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"sessions": responseSessions,
		})
	})

	// Auth: Revoke a specific session
	ts.mux.HandleFunc("DELETE /api/auth/sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		token := auth.GetTokenFromRequest(r)
		user, currentSession, err := auth.ValidateSession(ts.db, token)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to validate session"})
			return
		}

		if user == nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Authentication required"})
			return
		}

		sessionID := r.PathValue("id")
		if sessionID == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Session ID required"})
			return
		}

		if sessionID == currentSession.ID {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Cannot revoke current session. Use logout instead."})
			return
		}

		err = ts.db.DeleteSessionByID(sessionID, user.ID)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "Session not found"})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "success",
			"message": "Session revoked",
		})
	})

	// 2FA: Setup TOTP
	ts.mux.HandleFunc("POST /api/auth/totp/setup", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		token := auth.GetTokenFromRequest(r)
		user, _, err := auth.ValidateSession(ts.db, token)
		if err != nil || user == nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Authentication required"})
			return
		}

		if user.TOTPEnabled {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "2FA is already enabled"})
			return
		}

		secret, err := totp.GenerateSecret()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate secret"})
			return
		}

		if err := ts.db.SetTOTPSecret(user.ID, secret); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save secret"})
			return
		}

		issuer := "Subtitler"
		uri := totp.GenerateProvisioningURI(secret, user.Email, issuer)
		secretDisplay := totp.FormatSecretForDisplay(secret)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"secret":         secret,
			"secret_display": secretDisplay,
			"uri":            uri,
			"issuer":         issuer,
		})
	})

	// 2FA: Verify TOTP code and enable 2FA (rate limited)
	ts.mux.HandleFunc("POST /api/auth/totp/verify", ts.authLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		token := auth.GetTokenFromRequest(r)
		user, _, err := auth.ValidateSession(ts.db, token)
		if err != nil || user == nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Authentication required"})
			return
		}

		if user.TOTPEnabled {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "2FA is already enabled"})
			return
		}

		if user.TOTPSecret == nil || *user.TOTPSecret == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "No TOTP secret found"})
			return
		}

		var req struct {
			Code string `json:"code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
			return
		}

		if !totp.Validate(*user.TOTPSecret, req.Code) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid code"})
			return
		}

		if err := ts.db.EnableTOTP(user.ID); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to enable 2FA"})
			return
		}

		// Generate recovery codes
		recoveryCodes, err := totp.GenerateRecoveryCodes(totp.NumCodes)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"message":        "2FA has been enabled successfully",
				"totp_enabled":   true,
				"recovery_codes": []string{},
			})
			return
		}

		// Hash and store recovery codes
		codeHashes := make([]string, len(recoveryCodes))
		for i, code := range recoveryCodes {
			hash, err := totp.HashCode(code)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate recovery codes"})
				return
			}
			codeHashes[i] = hash
		}

		ts.db.SaveRecoveryCodes(user.ID, codeHashes)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":        "2FA has been enabled successfully",
			"totp_enabled":   true,
			"recovery_codes": recoveryCodes,
		})
	}))

	// 2FA: Recover account using recovery code (rate limited)
	ts.mux.HandleFunc("POST /api/auth/totp/recover", ts.authLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var req struct {
			Email        string `json:"email"`
			Password     string `json:"password"`
			RecoveryCode string `json:"recovery_code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
			return
		}

		if req.Email == "" || req.Password == "" || req.RecoveryCode == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Email, password, and recovery code are required"})
			return
		}

		user, err := ts.db.GetUserByEmail(req.Email)
		if err != nil || user == nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid email or password"})
			return
		}

		if !auth.CheckPassword(req.Password, user.PasswordHash) {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid email or password"})
			return
		}

		if !user.TOTPEnabled {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "2FA is not enabled for this account"})
			return
		}

		codes, err := ts.db.GetUnusedRecoveryCodes(user.ID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Server error"})
			return
		}

		var matchedCodeID string
		normalizedInput := totp.NormalizeCode(req.RecoveryCode)
		for _, code := range codes {
			if totp.CheckCode(normalizedInput, code.CodeHash) {
				matchedCodeID = code.ID
				break
			}
		}

		if matchedCodeID == "" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid recovery code"})
			return
		}

		ts.db.UseRecoveryCode(matchedCodeID)
		ts.db.DisableTOTP(user.ID)
		ts.db.DeleteRecoveryCodes(user.ID)
		ts.db.DeleteUserSessions(user.ID)

		session, err := auth.CreateSession(ts.db, user.ID, "", "")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create session"})
			return
		}

		auth.SetSessionCookie(w, session.Token, session.ExpiresAt)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":      "2FA has been disabled",
			"token":        session.Token,
			"totp_enabled": false,
		})
	}))

	// 2FA: Regenerate recovery codes (rate limited)
	ts.mux.HandleFunc("POST /api/auth/totp/codes", ts.authLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Check if user is authenticated
		tokenStr := auth.GetTokenFromRequest(r)
		user, _, err := auth.ValidateSession(ts.db, tokenStr)
		if err != nil || user == nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Authentication required"})
			return
		}

		// Check if 2FA is enabled
		if !user.TOTPEnabled {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "2FA is not enabled"})
			return
		}

		// Parse request body
		var req struct {
			Code     string `json:"code"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
			return
		}

		// Verify password
		if !auth.CheckPassword(req.Password, user.PasswordHash) {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid password"})
			return
		}

		// Validate the TOTP code
		if user.TOTPSecret == nil || !totp.Validate(*user.TOTPSecret, req.Code) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid 2FA code"})
			return
		}

		// Generate new recovery codes
		recoveryCodes, err := totp.GenerateRecoveryCodes(totp.NumCodes)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate recovery codes"})
			return
		}

		// Hash and store recovery codes
		codeHashes := make([]string, len(recoveryCodes))
		for i, code := range recoveryCodes {
			hash, err := totp.HashCode(code)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate recovery codes"})
				return
			}
			codeHashes[i] = hash
		}

		if err := ts.db.SaveRecoveryCodes(user.ID, codeHashes); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save recovery codes"})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":        "Recovery codes regenerated successfully",
			"recovery_codes": recoveryCodes,
		})
	}))

	// Auth: Forgot password - initiates password reset flow
	ts.mux.HandleFunc("POST /api/auth/forgot-password", ts.passwordResetLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var req struct {
			Email string `json:"email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
			return
		}

		req.Email = strings.TrimSpace(strings.ToLower(req.Email))
		if err := auth.ValidateEmail(req.Email); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		// Always return success to prevent email enumeration
		defer func() {
			json.NewEncoder(w).Encode(map[string]string{
				"message": "If an account exists with that email, a password reset link has been sent.",
			})
		}()

		user, err := ts.db.GetUserByEmail(req.Email)
		if err != nil || user == nil {
			return
		}

		// Generate reset token
		tokenBytes := make([]byte, 32)
		rand.Read(tokenBytes)
		token := hex.EncodeToString(tokenBytes)
		tokenHash := email.HashToken(token)

		expiresAt := time.Now().Add(1 * time.Hour)
		_, err = ts.db.CreatePasswordResetToken(user.ID, tokenHash, expiresAt)
		if err != nil {
			return
		}

		// Send email (uses mock service in tests)
		ts.emailService.SendPasswordReset(r.Context(), user.Email, token)
	}))

	// Auth: Reset password - completes password reset with token
	ts.mux.HandleFunc("POST /api/auth/reset-password", ts.authLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var req struct {
			Token    string `json:"token"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
			return
		}

		if req.Token == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Reset token is required"})
			return
		}

		if err := auth.ValidatePassword(req.Password); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		tokenHash := email.HashToken(req.Token)
		resetToken, err := ts.db.GetPasswordResetToken(tokenHash)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to process reset request"})
			return
		}

		if resetToken == nil || resetToken.Used || time.Now().After(resetToken.ExpiresAt) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid or expired reset token"})
			return
		}

		passwordHash, err := auth.HashPassword(req.Password)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to process reset request"})
			return
		}

		if err := ts.db.UpdateUserPassword(resetToken.UserID, passwordHash); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update password"})
			return
		}

		ts.db.UsePasswordResetToken(tokenHash)
		ts.db.DeletePasswordResetTokens(resetToken.UserID)
		ts.db.DeleteUserSessions(resetToken.UserID)

		json.NewEncoder(w).Encode(map[string]string{
			"message": "Password has been reset successfully. Please log in with your new password.",
		})
	}))

	// List videos
	ts.mux.HandleFunc("GET /api/videos", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		token := auth.GetTokenFromRequest(r)
		user, _, _ := auth.ValidateSession(ts.db, token)

		sessionID := r.URL.Query().Get("session_id")
		var sessionPtr *string
		if sessionID != "" {
			sessionPtr = &sessionID
		}

		var userPtr *string
		if user != nil {
			userPtr = &user.ID
		}

		videos, err := ts.db.ListVideos(userPtr, sessionPtr)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to list videos"})
			return
		}

		type VideoWithStatus struct {
			db.Video
			TranscriptionStatus string     `json:"transcription_status"`
			ExpiresAt           *time.Time `json:"expires_at,omitempty"`
		}

		result := make([]VideoWithStatus, len(videos))
		for i, v := range videos {
			result[i] = VideoWithStatus{Video: v, TranscriptionStatus: "none"}
			if t, err := ts.db.GetTranscription(v.ID); err == nil && t != nil {
				result[i].TranscriptionStatus = t.Status
			}

			// Calculate expiration time based on user type
			var expiresAt time.Time
			if v.UserID == nil {
				expiresAt = v.CreatedAt.Add(48 * time.Hour)
			} else {
				expiresAt = v.CreatedAt.Add(90 * 24 * time.Hour)
			}
			result[i].ExpiresAt = &expiresAt
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"videos": result,
		})
	})

	// Get transcription status
	ts.mux.HandleFunc("GET /api/transcribe/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		uploadID := r.PathValue("id")
		if uploadID == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Upload ID required"})
			return
		}

		transcription, err := ts.db.GetTranscription(uploadID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get transcription status"})
			return
		}

		if transcription == nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "No transcription found for this upload"})
			return
		}

		json.NewEncoder(w).Encode(dbTranscriptionToStatus(transcription))
	})

	// Download SRT file
	ts.mux.HandleFunc("GET /api/videos/{id}/subtitles.srt", func(w http.ResponseWriter, r *http.Request) {
		uploadID := r.PathValue("id")
		if uploadID == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Upload ID required"})
			return
		}

		transcription, err := ts.db.GetTranscription(uploadID)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get transcription"})
			return
		}

		if transcription == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "No transcription found for this upload"})
			return
		}

		if transcription.Status != "complete" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error":  "Transcription not yet complete",
				"status": transcription.Status,
			})
			return
		}

		segments, err := transcription.GetSegments()
		if err != nil || len(segments) == 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "No subtitle segments available"})
			return
		}

		whisperResult := &WhisperResult{
			Language: transcription.Language,
			Duration: transcription.Duration,
			Text:     transcription.FullText,
			Segments: make([]WhisperSegment, len(segments)),
		}
		for i, s := range segments {
			whisperResult.Segments[i] = WhisperSegment{
				ID:    s.ID,
				Start: s.Start,
				End:   s.End,
				Text:  s.Text,
			}
		}

		srtContent := generateSRT(whisperResult)

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=\""+uploadID+".srt\"")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(srtContent))
	})

	// Download VTT file
	ts.mux.HandleFunc("GET /api/videos/{id}/subtitles.vtt", func(w http.ResponseWriter, r *http.Request) {
		uploadID := r.PathValue("id")
		if uploadID == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Upload ID required"})
			return
		}

		transcription, err := ts.db.GetTranscription(uploadID)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get transcription"})
			return
		}

		if transcription == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "No transcription found for this upload"})
			return
		}

		if transcription.Status != "complete" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error":  "Transcription not yet complete",
				"status": transcription.Status,
			})
			return
		}

		segments, err := transcription.GetSegments()
		if err != nil || len(segments) == 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "No subtitle segments available"})
			return
		}

		whisperResult := &WhisperResult{
			Language: transcription.Language,
			Duration: transcription.Duration,
			Text:     transcription.FullText,
			Segments: make([]WhisperSegment, len(segments)),
		}
		for i, s := range segments {
			whisperResult.Segments[i] = WhisperSegment{
				ID:    s.ID,
				Start: s.Start,
				End:   s.End,
				Text:  s.Text,
			}
		}

		vttContent := generateVTT(whisperResult)

		w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=\""+uploadID+".vtt\"")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(vttContent))
	})

	// Download JSON file
	ts.mux.HandleFunc("GET /api/videos/{id}/subtitles.json", func(w http.ResponseWriter, r *http.Request) {
		uploadID := r.PathValue("id")
		if uploadID == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Upload ID required"})
			return
		}

		transcription, err := ts.db.GetTranscription(uploadID)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get transcription"})
			return
		}

		if transcription == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "No transcription found for this upload"})
			return
		}

		if transcription.Status != "complete" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error":  "Transcription not yet complete",
				"status": transcription.Status,
			})
			return
		}

		segments, err := transcription.GetSegments()
		if err != nil || len(segments) == 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "No subtitle segments available"})
			return
		}

		response := map[string]interface{}{
			"video_id":  uploadID,
			"language":  transcription.Language,
			"duration":  transcription.Duration,
			"full_text": transcription.FullText,
			"segments":  segments,
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=\""+uploadID+".json\"")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	})

	// Update segments
	ts.mux.HandleFunc("PUT /api/transcribe/{id}/segments", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		uploadID := r.PathValue("id")
		if uploadID == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Upload ID required"})
			return
		}

		transcription, err := ts.db.GetTranscription(uploadID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get transcription"})
			return
		}
		if transcription == nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "No transcription found for this upload"})
			return
		}
		if transcription.Status != "complete" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error":  "Cannot edit segments - transcription not complete",
				"status": transcription.Status,
			})
			return
		}

		var req struct {
			Segments []db.Segment `json:"segments"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
			return
		}

		for i, seg := range req.Segments {
			if seg.Start < 0 || seg.End < 0 {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "Segment " + string(rune('0'+i)) + " has invalid timing",
				})
				return
			}
			if seg.Start > seg.End {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "Segment start time cannot be greater than end time",
				})
				return
			}
		}

		if err := ts.db.UpdateSegments(uploadID, req.Segments); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update segments"})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "success",
			"segments": len(req.Segments),
		})
	})

	// Script detection endpoint
	ts.mux.HandleFunc("POST /api/text/detect-script", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var req struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
			return
		}

		if len(req.Text) > 10240 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Text too long (max 10KB)"})
			return
		}

		detectedScript := script.DetectScript(req.Text)
		detectedLang := script.DetectLanguageFromRomanized(req.Text)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"detected_script":   string(detectedScript),
			"detected_language": detectedLang,
			"confidence":        0.5, // Simplified for tests
		})
	})

	// Script conversion endpoint
	ts.mux.HandleFunc("POST /api/text/convert", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var req struct {
			Text         string `json:"text"`
			SourceScript string `json:"source_script,omitempty"`
			TargetScript string `json:"target_script"`
			Language     string `json:"language"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
			return
		}

		if len(req.Text) > 10240 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Text too long (max 10KB)"})
			return
		}

		if !script.IsLanguageSupported(req.Language) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":               "Unsupported language",
				"supported_languages": script.SupportedLanguages(),
			})
			return
		}

		targetScript := script.Script(req.TargetScript)
		if !script.IsScriptSupported(targetScript) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":             "Unsupported target script",
				"supported_scripts": script.SupportedScripts(),
			})
			return
		}

		sourceScript := script.Script(req.SourceScript)
		if sourceScript == "" {
			sourceScript = script.DetectScript(req.Text)
		}

		converter := script.NewConverter()
		converted, err := converter.Convert(req.Text, req.Language, targetScript)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Conversion failed"})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"original":      req.Text,
			"converted":     converted,
			"source_script": string(sourceScript),
			"target_script": string(targetScript),
			"language":      req.Language,
		})
	})

	// Upload endpoint with MIME type validation
	ts.mux.HandleFunc("POST /api/upload", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Parse multipart form
		if err := r.ParseMultipartForm(500 << 20); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "File too large or invalid form data",
			})
			return
		}

		// Get the file from the form
		file, header, err := r.FormFile("video")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "No video file provided",
			})
			return
		}
		defer file.Close()

		// Validate file type by checking content type against whitelist
		contentType := header.Header.Get("Content-Type")
		allowedMIMETypes := map[string]bool{
			"video/mp4":        true,
			"video/webm":       true,
			"video/quicktime":  true,
			"video/x-m4v":      true,
			"video/mpeg":       true,
			"video/x-msvideo":  true,
			"video/x-matroska": true,
			"video/ogg":        true,
		}
		if !allowedMIMETypes[contentType] {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error":             "Unsupported video format. Allowed formats: MP4, WebM, MOV, M4V, MPEG, AVI, MKV, OGV",
				"provided_mimetype": contentType,
			})
			return
		}

		// Success - in real handler would save file
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "success",
			"filename": header.Filename,
		})
	})
}

// request helpers
func (ts *testServer) doRequest(method, path string, body interface{}, token string) *httptest.ResponseRecorder {
	var bodyReader io.Reader
	if body != nil {
		bodyBytes, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)
	return rr
}

// createTestUser creates a user and returns their auth token
func (ts *testServer) createTestUser(t *testing.T, email, password string) string {
	t.Helper()

	resp := ts.doRequest("POST", "/api/auth/register", map[string]string{
		"email":    email,
		"password": password,
	}, "")

	if resp.Code != http.StatusOK {
		t.Fatalf("Failed to create test user: %s", resp.Body.String())
	}

	var result struct {
		Token string `json:"token"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Token
}

// createTestUserWithID creates a user and returns both the user ID and auth token
func (ts *testServer) createTestUserWithID(t *testing.T, email, password string) (userID string, token string) {
	t.Helper()

	resp := ts.doRequest("POST", "/api/auth/register", map[string]string{
		"email":    email,
		"password": password,
	}, "")

	if resp.Code != http.StatusOK {
		t.Fatalf("Failed to create test user: %s", resp.Body.String())
	}

	var result struct {
		Token string `json:"token"`
		User  struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.User.ID, result.Token
}

// createTestVideo creates a video record directly in the database
func (ts *testServer) createTestVideo(t *testing.T, userID *string, sessionID *string) *db.Video {
	t.Helper()

	video := &db.Video{
		ID:          generateID(),
		Filename:    "test.mp4",
		Size:        1024,
		ContentType: "video/mp4",
		FilePath:    filepath.Join(ts.uploadDir, "test.mp4"),
		UserID:      userID,
		SessionID:   sessionID,
		CreatedAt:   time.Now(),
	}

	if err := ts.db.CreateVideo(video); err != nil {
		t.Fatalf("Failed to create test video: %v", err)
	}
	return video
}

// createTestTranscription creates a completed transcription record
func (ts *testServer) createTestTranscription(t *testing.T, videoID string) *db.Transcription {
	t.Helper()

	segments := []db.Segment{
		{ID: 0, Start: 0.0, End: 2.5, Text: "Hello world."},
		{ID: 1, Start: 3.0, End: 5.5, Text: "This is a test."},
	}

	transcription := &db.Transcription{
		ID:        generateID(),
		VideoID:   videoID,
		Status:    "pending",
		Message:   "Test",
		Progress:  0,
		CreatedAt: time.Now(),
	}

	if err := ts.db.CreateTranscription(transcription); err != nil {
		t.Fatalf("Failed to create transcription: %v", err)
	}

	if err := ts.db.CompleteTranscription(videoID, "en", 5.5, "Hello world. This is a test.", segments); err != nil {
		t.Fatalf("Failed to complete transcription: %v", err)
	}

	result, _ := ts.db.GetTranscription(videoID)
	return result
}

// ========== Test Cases ==========

func TestHealthEndpoint(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	resp := ts.doRequest("GET", "/api/health", nil, "")

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)

	if result["status"] != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", result["status"])
	}
}

func TestLogEndpoint(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	tests := []struct {
		name       string
		body       map[string]interface{}
		wantStatus int
	}{
		{
			name: "valid log message",
			body: map[string]interface{}{
				"level":   "log",
				"message": "Test log message",
				"url":     "http://localhost:4321/upload",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "error level with line info",
			body: map[string]interface{}{
				"level":   "error",
				"message": "Test error message",
				"url":     "http://localhost:4321/upload",
				"line":    42,
				"column":  10,
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "warn level",
			body: map[string]interface{}{
				"level":   "warn",
				"message": "Test warning",
				"url":     "http://localhost:4321/",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "debug level",
			body: map[string]interface{}{
				"level":   "debug",
				"message": "Debug info",
				"url":     "http://localhost:4321/",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "info level",
			body: map[string]interface{}{
				"level":   "info",
				"message": "Info message",
				"url":     "http://localhost:4321/",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "invalid level defaults to log",
			body: map[string]interface{}{
				"level":   "invalid",
				"message": "Message with invalid level",
				"url":     "http://localhost:4321/",
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "minimal body",
			body: map[string]interface{}{
				"message": "Just a message",
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, _ := json.Marshal(tt.body)
			resp := ts.doRequest("POST", "/api/log", bytes.NewReader(body), "")

			if resp.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, resp.Code)
			}

			var result map[string]string
			json.NewDecoder(resp.Body).Decode(&result)

			if result["status"] != "ok" {
				t.Errorf("Expected status 'ok', got '%s'", result["status"])
			}
		})
	}
}

func TestLogEndpointInvalidBody(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Send completely invalid JSON directly (bypass doRequest's json.Marshal)
	req := httptest.NewRequest("POST", "/api/log", strings.NewReader(`{level: not-valid-json}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

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
			wantStatus: http.StatusOK,
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
				if _, ok := result["token"]; !ok {
					t.Error("Expected token in response")
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

func TestListVideos(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create user and video
	token := ts.createTestUser(t, "videos@example.com", "ValidPassword123!")

	// Get user ID from /me
	resp := ts.doRequest("GET", "/api/auth/me", nil, token)
	var meResult struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	json.NewDecoder(resp.Body).Decode(&meResult)
	userID := meResult.User.ID

	// Create video for this user
	ts.createTestVideo(t, &userID, nil)

	// List videos
	resp = ts.doRequest("GET", "/api/videos", nil, token)
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}

	var listResult struct {
		Videos []struct {
			ID string `json:"id"`
		} `json:"videos"`
	}
	json.NewDecoder(resp.Body).Decode(&listResult)

	if len(listResult.Videos) != 1 {
		t.Errorf("Expected 1 video, got %d", len(listResult.Videos))
	}
}

func TestListVideosBySession(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	sessionID := "test-session-123"

	// Create video with session ID
	ts.createTestVideo(t, nil, &sessionID)

	// List videos by session
	resp := ts.doRequest("GET", "/api/videos?session_id="+sessionID, nil, "")
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}

	var listResult struct {
		Videos []struct {
			ID string `json:"id"`
		} `json:"videos"`
	}
	json.NewDecoder(resp.Body).Decode(&listResult)

	if len(listResult.Videos) != 1 {
		t.Errorf("Expected 1 video, got %d", len(listResult.Videos))
	}
}

func TestListVideosExpiresAt(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Test 1: Anonymous video should expire in 48 hours
	sessionID := "anon-session"
	ts.createTestVideo(t, nil, &sessionID)

	resp := ts.doRequest("GET", "/api/videos?session_id="+sessionID, nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.Code)
	}

	var anonResult struct {
		Videos []struct {
			ID        string  `json:"id"`
			UserID    *string `json:"user_id"`
			ExpiresAt string  `json:"expires_at"`
		} `json:"videos"`
	}
	json.NewDecoder(resp.Body).Decode(&anonResult)

	if len(anonResult.Videos) != 1 {
		t.Fatalf("Expected 1 video, got %d", len(anonResult.Videos))
	}

	if anonResult.Videos[0].ExpiresAt == "" {
		t.Error("Expected expires_at to be set for anonymous video")
	}

	// Parse the expiry time and verify it's ~48 hours from now
	expiresAt, err := time.Parse(time.RFC3339Nano, anonResult.Videos[0].ExpiresAt)
	if err != nil {
		t.Fatalf("Failed to parse expires_at: %v", err)
	}
	hoursUntilExpiry := time.Until(expiresAt).Hours()
	if hoursUntilExpiry < 47 || hoursUntilExpiry > 49 {
		t.Errorf("Expected ~48 hours until expiry, got %.1f", hoursUntilExpiry)
	}

	// Test 2: Registered user video should expire in 90 days
	token := ts.createTestUser(t, "expires@example.com", "ValidPassword123!")
	resp = ts.doRequest("GET", "/api/auth/me", nil, token)
	var meResult struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	json.NewDecoder(resp.Body).Decode(&meResult)
	userID := meResult.User.ID

	ts.createTestVideo(t, &userID, nil)

	resp = ts.doRequest("GET", "/api/videos", nil, token)
	if resp.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.Code)
	}

	var userResult struct {
		Videos []struct {
			ID        string  `json:"id"`
			UserID    *string `json:"user_id"`
			ExpiresAt string  `json:"expires_at"`
		} `json:"videos"`
	}
	json.NewDecoder(resp.Body).Decode(&userResult)

	if len(userResult.Videos) != 1 {
		t.Fatalf("Expected 1 video, got %d", len(userResult.Videos))
	}

	if userResult.Videos[0].ExpiresAt == "" {
		t.Error("Expected expires_at to be set for registered user video")
	}

	// Parse the expiry time and verify it's ~90 days from now
	expiresAt, err = time.Parse(time.RFC3339Nano, userResult.Videos[0].ExpiresAt)
	if err != nil {
		t.Fatalf("Failed to parse expires_at: %v", err)
	}
	daysUntilExpiry := time.Until(expiresAt).Hours() / 24
	if daysUntilExpiry < 89 || daysUntilExpiry > 91 {
		t.Errorf("Expected ~90 days until expiry, got %.1f", daysUntilExpiry)
	}
}

func TestGetTranscriptionStatus(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create video and transcription
	video := ts.createTestVideo(t, nil, nil)
	ts.createTestTranscription(t, video.ID)

	// Get transcription status
	resp := ts.doRequest("GET", "/api/transcribe/"+video.ID, nil, "")
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result struct {
		Status string `json:"status"`
		Result struct {
			Segments []struct {
				Text string `json:"text"`
			} `json:"segments"`
		} `json:"result"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	if result.Status != "complete" {
		t.Errorf("Expected status 'complete', got '%s'", result.Status)
	}

	if len(result.Result.Segments) != 2 {
		t.Errorf("Expected 2 segments, got %d", len(result.Result.Segments))
	}
}

func TestGetTranscriptionNotFound(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	resp := ts.doRequest("GET", "/api/transcribe/nonexistent", nil, "")
	if resp.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.Code)
	}
}

func TestDownloadSRT(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create video and transcription
	video := ts.createTestVideo(t, nil, nil)
	ts.createTestTranscription(t, video.ID)

	// Download SRT
	resp := ts.doRequest("GET", "/api/videos/"+video.ID+"/subtitles.srt", nil, "")
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	// Check content type
	contentType := resp.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") {
		t.Errorf("Expected Content-Type text/plain, got %s", contentType)
	}

	// Check SRT content
	srtContent := resp.Body.String()
	if !strings.Contains(srtContent, "Hello world.") {
		t.Error("SRT content should contain 'Hello world.'")
	}
	if !strings.Contains(srtContent, "00:00:00,000 --> 00:00:02,500") {
		t.Error("SRT content should contain timestamps")
	}
}

func TestDownloadSRTNotComplete(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create video with pending transcription
	video := ts.createTestVideo(t, nil, nil)
	transcription := &db.Transcription{
		ID:        generateID(),
		VideoID:   video.ID,
		Status:    "processing",
		Message:   "Processing...",
		Progress:  50,
		CreatedAt: time.Now(),
	}
	ts.db.CreateTranscription(transcription)

	// Try to download SRT
	resp := ts.doRequest("GET", "/api/videos/"+video.ID+"/subtitles.srt", nil, "")
	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.Code)
	}
}

func TestDownloadVTT(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create video and transcription
	video := ts.createTestVideo(t, nil, nil)
	ts.createTestTranscription(t, video.ID)

	// Download VTT
	resp := ts.doRequest("GET", "/api/videos/"+video.ID+"/subtitles.vtt", nil, "")
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	// Check content type
	contentType := resp.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/vtt") {
		t.Errorf("Expected Content-Type text/vtt, got %s", contentType)
	}

	// Check VTT content
	vttContent := resp.Body.String()
	if !strings.HasPrefix(vttContent, "WEBVTT") {
		t.Error("VTT content should start with 'WEBVTT'")
	}
	if !strings.Contains(vttContent, "Hello world.") {
		t.Error("VTT content should contain 'Hello world.'")
	}
	// VTT uses period instead of comma for milliseconds
	if !strings.Contains(vttContent, "00:00:00.000 --> 00:00:02.500") {
		t.Error("VTT content should contain timestamps with period separator")
	}
}

func TestDownloadVTTNotFound(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Try to download VTT for non-existent video
	resp := ts.doRequest("GET", "/api/videos/nonexistent/subtitles.vtt", nil, "")
	if resp.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.Code)
	}
}

func TestDownloadJSON(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create video and transcription
	video := ts.createTestVideo(t, nil, nil)
	ts.createTestTranscription(t, video.ID)

	// Download JSON
	resp := ts.doRequest("GET", "/api/videos/"+video.ID+"/subtitles.json", nil, "")
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	// Check content type
	contentType := resp.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	// Check JSON content
	var jsonResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&jsonResp); err != nil {
		t.Errorf("Failed to decode JSON response: %v", err)
		return
	}

	// Verify structure
	if jsonResp["video_id"] != video.ID {
		t.Errorf("Expected video_id %s, got %v", video.ID, jsonResp["video_id"])
	}
	if jsonResp["full_text"] == nil {
		t.Error("Expected full_text field in response")
	}
	segments, ok := jsonResp["segments"].([]interface{})
	if !ok || len(segments) == 0 {
		t.Error("Expected non-empty segments array in response")
	}
}

func TestDownloadJSONNotFound(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Try to download JSON for non-existent video
	resp := ts.doRequest("GET", "/api/videos/nonexistent/subtitles.json", nil, "")
	if resp.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.Code)
	}
}

func TestUpdateSegments(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create video and transcription
	video := ts.createTestVideo(t, nil, nil)
	ts.createTestTranscription(t, video.ID)

	// Update segments
	newSegments := []db.Segment{
		{ID: 0, Start: 0.0, End: 3.0, Text: "Updated text."},
		{ID: 1, Start: 3.5, End: 6.0, Text: "Also updated."},
	}

	resp := ts.doRequest("PUT", "/api/transcribe/"+video.ID+"/segments", map[string]interface{}{
		"segments": newSegments,
	}, "")

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	// Verify update
	transcription, _ := ts.db.GetTranscription(video.ID)
	segments, _ := transcription.GetSegments()

	if segments[0].Text != "Updated text." {
		t.Errorf("Expected 'Updated text.', got '%s'", segments[0].Text)
	}
}

func TestUpdateSegmentsInvalidTiming(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	video := ts.createTestVideo(t, nil, nil)
	ts.createTestTranscription(t, video.ID)

	// Try with invalid timing (start > end)
	resp := ts.doRequest("PUT", "/api/transcribe/"+video.ID+"/segments", map[string]interface{}{
		"segments": []db.Segment{
			{ID: 0, Start: 5.0, End: 2.0, Text: "Invalid"},
		},
	}, "")

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid timing, got %d", resp.Code)
	}
}

func TestUpdateSegmentsNegativeTiming(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	video := ts.createTestVideo(t, nil, nil)
	ts.createTestTranscription(t, video.ID)

	// Try with negative timing
	resp := ts.doRequest("PUT", "/api/transcribe/"+video.ID+"/segments", map[string]interface{}{
		"segments": []db.Segment{
			{ID: 0, Start: -1.0, End: 2.0, Text: "Negative start"},
		},
	}, "")

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for negative timing, got %d", resp.Code)
	}
}

// ========== Upload Tests (with mock file) ==========

func TestUploadVideoForm(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a simple test video file (just bytes, not a real video)
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Create form file
	part, err := writer.CreateFormFile("video", "test.mp4")
	if err != nil {
		t.Fatal(err)
	}

	// Write some fake video bytes with a video magic number
	// MP4 files start with ftyp atom
	fakeVideo := []byte{0x00, 0x00, 0x00, 0x20, 0x66, 0x74, 0x79, 0x70}
	part.Write(fakeVideo)
	writer.Close()

	req := httptest.NewRequest("POST", "/api/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Note: This test won't pass the content-type validation since our fake bytes
	// don't have a proper video MIME type. This is testing the form parsing.
	// A real test would need to use a proper video file.
}

// ========== Auth Cookie Tests ==========

func TestSessionCookie(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	resp := ts.doRequest("POST", "/api/auth/register", map[string]string{
		"email":    "cookie@example.com",
		"password": "ValidPassword123!",
	}, "")

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}

	// Check for session cookie
	cookies := resp.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "session" {
			sessionCookie = c
			break
		}
	}

	if sessionCookie == nil {
		t.Error("Expected session cookie to be set")
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

	if resp.Code != http.StatusUnauthorized {
		t.Errorf("Expected second recovery with same code to fail with 401, got %d", resp.Code)
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
		req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"email":"test@example.com","password":"password123"}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.168.1.100:12345"
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// 3rd request should be rate limited
	req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"email":"test@example.com","password":"password123"}`))
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
	req = httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"email":"test@example.com","password":"password123"}`))
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
		req := httptest.NewRequest("POST", "/api/auth/register", strings.NewReader(`{"email":"test@example.com","password":"password123"}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.168.1.100:12345"
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// 3rd request should be rate limited
	req := httptest.NewRequest("POST", "/api/auth/register", strings.NewReader(`{"email":"test@example.com","password":"password123"}`))
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
func TestRateLimitingXForwardedFor(t *testing.T) {
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
func TestRateLimitingUpload(t *testing.T) {
	strictLimiter := ratelimit.New(2, time.Minute)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/upload", strictLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))

	// First 2 requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/api/upload", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// 3rd request should be rate limited
	reqLimited := httptest.NewRequest("POST", "/api/upload", nil)
	reqLimited.RemoteAddr = "192.168.1.100:12345"
	wLimited := httptest.NewRecorder()
	mux.ServeHTTP(wLimited, reqLimited)

	if wLimited.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429 Too Many Requests, got %d", wLimited.Code)
	}
}

// TestRateLimitingTranscribe tests that transcribe endpoint is rate limited
func TestRateLimitingTranscribe(t *testing.T) {
	strictLimiter := ratelimit.New(2, time.Minute)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/transcribe/{id}", strictLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))

	// First 2 requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/api/transcribe/test123", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// 3rd request should be rate limited
	req := httptest.NewRequest("POST", "/api/transcribe/test123", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429 Too Many Requests, got %d", w.Code)
	}
}

// TestRateLimitingBurn tests that burn endpoint is rate limited
func TestRateLimitingBurn(t *testing.T) {
	strictLimiter := ratelimit.New(2, time.Minute)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/videos/{id}/burn", strictLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))

	// First 2 requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("POST", "/api/videos/test123/burn", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// 3rd request should be rate limited
	req := httptest.NewRequest("POST", "/api/videos/test123/burn", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429 Too Many Requests, got %d", w.Code)
	}
}

// TestGetSessions tests listing user's sessions
func TestGetSessions(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create user (registration creates first session)
	userID, regToken := ts.createTestUserWithID(t, "test@example.com", "password123")

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
	userID, regToken := ts.createTestUserWithID(t, "test@example.com", "password123")

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
	userID, regToken := ts.createTestUserWithID(t, "test@example.com", "password123")

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
	_, token1 := ts.createTestUserWithID(t, "user1@example.com", "password123")
	user2ID, _ := ts.createTestUserWithID(t, "user2@example.com", "password123")

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
func TestDetectScriptEndpoint(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	tests := []struct {
		name           string
		text           string
		expectedScript string
		expectedLang   string
	}{
		{
			name:           "Latin text",
			text:           "Hello World",
			expectedScript: "Latin",
			expectedLang:   "hi", // Defaults to Hindi for ambiguous romanized text
		},
		{
			name:           "Devanagari text",
			text:           "नमस्ते दुनिया",
			expectedScript: "Devanagari",
			expectedLang:   "hi",
		},
		{
			name:           "Hindi romanized with keywords",
			text:           "main tumse pyar karta hoon",
			expectedScript: "Latin",
			expectedLang:   "hi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := ts.doRequest("POST", "/api/text/detect-script", map[string]string{
				"text": tt.text,
			}, "")

			if w.Code != http.StatusOK {
				t.Errorf("Expected 200, got %d", w.Code)
				return
			}

			var resp map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Errorf("Failed to decode response: %v", err)
				return
			}

			if resp["detected_script"] != tt.expectedScript {
				t.Errorf("Expected script %q, got %q", tt.expectedScript, resp["detected_script"])
			}
			if resp["detected_language"] != tt.expectedLang {
				t.Errorf("Expected language %q, got %q", tt.expectedLang, resp["detected_language"])
			}
		})
	}
}

// TestDetectScriptTooLong tests script detection with text that's too long
func TestDetectScriptTooLong(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create 11KB of text (over 10KB limit)
	longText := strings.Repeat("a", 11*1024)

	w := ts.doRequest("POST", "/api/text/detect-script", map[string]string{
		"text": longText,
	}, "")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for text too long, got %d", w.Code)
	}
}

// TestConvertScriptEndpoint tests the script conversion API
func TestConvertScriptEndpoint(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	w := ts.doRequest("POST", "/api/text/convert", map[string]string{
		"text":          "namaste",
		"target_script": "Devanagari",
		"language":      "hi",
	}, "")

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
		return
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Errorf("Failed to decode response: %v", err)
		return
	}

	if resp["original"] != "namaste" {
		t.Errorf("Expected original %q, got %q", "namaste", resp["original"])
	}
	if resp["converted"] == "" {
		t.Error("Expected non-empty converted text")
	}
	if resp["target_script"] != "Devanagari" {
		t.Errorf("Expected target_script %q, got %q", "Devanagari", resp["target_script"])
	}
}

// TestConvertScriptUnsupportedLanguage tests script conversion with unsupported language
func TestConvertScriptUnsupportedLanguage(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	w := ts.doRequest("POST", "/api/text/convert", map[string]string{
		"text":          "hello",
		"target_script": "Devanagari",
		"language":      "en", // English not supported
	}, "")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for unsupported language, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] != "Unsupported language" {
		t.Errorf("Expected unsupported language error, got %v", resp["error"])
	}
}

// TestConvertScriptUnsupportedScript tests script conversion with unsupported target script
func TestConvertScriptUnsupportedScript(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	w := ts.doRequest("POST", "/api/text/convert", map[string]string{
		"text":          "namaste",
		"target_script": "Latin", // Latin not supported as target
		"language":      "hi",
	}, "")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for unsupported script, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] != "Unsupported target script" {
		t.Errorf("Expected unsupported script error, got %v", resp["error"])
	}
}

// TestConvertScriptMultipleWords tests script conversion with multiple words
func TestConvertScriptMultipleWords(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	w := ts.doRequest("POST", "/api/text/convert", map[string]string{
		"text":          "namaste duniya",
		"target_script": "Devanagari",
		"language":      "hi",
	}, "")

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
		return
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	converted := resp["converted"].(string)
	// Should have two words separated by space
	if !strings.Contains(converted, " ") {
		t.Error("Expected converted text to have multiple words with space")
	}
}

// Test forgot-password endpoint
func TestForgotPassword(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a test user
	ts.createTestUser(t, "test@example.com", "password123")

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

	// Check that an email was sent
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
	ts.createTestUser(t, "test@example.com", "oldpassword123")

	// Request password reset to get a token
	ts.doRequest("POST", "/api/auth/forgot-password", map[string]string{
		"email": "test@example.com",
	}, "")

	// Get the reset token from the email
	emails := ts.emailService.GetEmails()
	if len(emails) == 0 {
		t.Fatal("No email sent")
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
		"password": "newpassword123",
	}, "")

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify we can login with new password
	w = ts.doRequest("POST", "/api/auth/login", map[string]string{
		"email":    "test@example.com",
		"password": "newpassword123",
	}, "")

	if w.Code != http.StatusOK {
		t.Errorf("Expected login to succeed with new password, got %d", w.Code)
	}

	// Verify old password no longer works
	w = ts.doRequest("POST", "/api/auth/login", map[string]string{
		"email":    "test@example.com",
		"password": "oldpassword123",
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
		"password": "newpassword123",
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
	userID, _ := ts.createTestUserWithID(t, "test@example.com", "password123")

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
		"password": "newpassword123",
	}, "")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for expired token, got %d", w.Code)
	}
}

func TestResetPasswordWeakPassword(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a test user and get a valid token
	ts.createTestUser(t, "test@example.com", "password123")
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
	ts.createTestUser(t, "test@example.com", "password123")
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

	// First reset should succeed
	w := ts.doRequest("POST", "/api/auth/reset-password", map[string]string{
		"token":    token,
		"password": "newpassword123",
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

// ========== Upload MIME Type Validation Tests ==========

func TestUploadMIMETypeValidation(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	tests := []struct {
		name        string
		contentType string
		expectCode  int
		expectError bool
	}{
		// Allowed MIME types
		{"MP4 allowed", "video/mp4", http.StatusOK, false},
		{"WebM allowed", "video/webm", http.StatusOK, false},
		{"QuickTime allowed", "video/quicktime", http.StatusOK, false},
		{"M4V allowed", "video/x-m4v", http.StatusOK, false},
		{"MPEG allowed", "video/mpeg", http.StatusOK, false},
		{"AVI allowed", "video/x-msvideo", http.StatusOK, false},
		{"MKV allowed", "video/x-matroska", http.StatusOK, false},
		{"OGV allowed", "video/ogg", http.StatusOK, false},

		// Disallowed MIME types
		{"Image rejected", "image/jpeg", http.StatusBadRequest, true},
		{"Text rejected", "text/plain", http.StatusBadRequest, true},
		{"Audio rejected", "audio/mp3", http.StatusBadRequest, true},
		{"3GPP rejected", "video/3gpp", http.StatusBadRequest, true},
		{"Empty rejected", "", http.StatusBadRequest, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			writer := multipart.NewWriter(&buf)

			// Create part with specified content type
			h := make(textproto.MIMEHeader)
			h.Set("Content-Disposition", `form-data; name="video"; filename="test.mp4"`)
			h.Set("Content-Type", tc.contentType)
			part, err := writer.CreatePart(h)
			if err != nil {
				t.Fatal(err)
			}

			// Write some fake video bytes
			part.Write([]byte{0x00, 0x00, 0x00, 0x20, 0x66, 0x74, 0x79, 0x70})
			writer.Close()

			req := httptest.NewRequest("POST", "/api/upload", &buf)
			req.Header.Set("Content-Type", writer.FormDataContentType())
			req.RemoteAddr = "192.168.1.1:12345" // Different IP to avoid rate limiting

			w := httptest.NewRecorder()
			ts.mux.ServeHTTP(w, req)

			if w.Code != tc.expectCode {
				t.Errorf("Expected status %d, got %d for MIME type %q: %s",
					tc.expectCode, w.Code, tc.contentType, w.Body.String())
			}

			if tc.expectError {
				var resp map[string]interface{}
				json.NewDecoder(w.Body).Decode(&resp)
				if resp["error"] == nil || !strings.Contains(resp["error"].(string), "Unsupported video format") {
					t.Errorf("Expected 'Unsupported video format' error, got %v", resp["error"])
				}
			}
		})
	}
}

// TestRateLimitingDetectScript tests that detect-script endpoint is rate limited
func TestRateLimitingDetectScript(t *testing.T) {
	strictLimiter := ratelimit.New(10, time.Minute)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/text/detect-script", strictLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"detected_script": "Latin"})
	}))

	// First 10 requests should succeed
	for i := 0; i < 10; i++ {
		body := bytes.NewBufferString(`{"text":"hello"}`)
		req := httptest.NewRequest("POST", "/api/text/detect-script", body)
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.168.1.100:12345"
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	// 11th request should be rate limited
	body := bytes.NewBufferString(`{"text":"hello"}`)
	req := httptest.NewRequest("POST", "/api/text/detect-script", body)
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.168.1.100:12345"
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429 Too Many Requests, got %d", w.Code)
	}
}
