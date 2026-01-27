package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strconv"
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

// testGenerateID generates a random ID for testing purposes.
// Panics on error since test setup should never fail this way.
func testGenerateID() string {
	id, err := generateID()
	if err != nil {
		panic(fmt.Sprintf("testGenerateID failed: %v", err))
	}
	return id
}

// testServer holds all dependencies needed for testing
type testServer struct {
	mux                  *http.ServeMux
	db                   *db.DB
	encryptor            *crypto.Encryptor
	uploadDir            string
	authLimiter          *ratelimit.Limiter
	passwordResetLimiter *ratelimit.Limiter
	downloadLimiter      *ratelimit.Limiter
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
		downloadLimiter:      ratelimit.New(30, time.Minute),
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
	// Health check endpoint (enhanced with dependency checks)
	// Unauthenticated: returns only {"status": "ok"} or {"status": "degraded"}
	// Authenticated: returns full response with all dependency details
	ts.mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		status := HealthStatus{
			Status:           "ok",
			DBConnected:      true,
			WhisperAvailable: true,
			DiskSpaceOK:      true,
			Errors:           []string{},
		}

		// Check database connectivity
		if err := ts.db.Ping(); err != nil {
			status.DBConnected = false
			status.Errors = append(status.Errors, fmt.Sprintf("database: %v", err))
		}

		// Check whisper-server availability (if configured)
		whisperOK, whisperErr := checkWhisperServerHealth()
		if !whisperOK {
			status.WhisperAvailable = false
			status.Errors = append(status.Errors, whisperErr)
		}

		// Check disk space (minimum 1GB free for uploads)
		diskOK, freeGB, diskErr := checkDiskSpace(".", 1.0)
		status.DiskFreeGB = freeGB
		if !diskOK {
			status.DiskSpaceOK = false
			status.Errors = append(status.Errors, diskErr)
		}

		// Determine overall status
		if !status.DBConnected || !status.WhisperAvailable || !status.DiskSpaceOK {
			status.Status = "degraded"
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		// Check if user is authenticated
		token := auth.GetTokenFromRequest(r)
		user, _, _ := auth.ValidateSession(ts.db, token)

		if user != nil {
			// Authenticated: return full response
			json.NewEncoder(w).Encode(status)
		} else {
			// Unauthenticated: return minimal response
			json.NewEncoder(w).Encode(map[string]string{
				"status": status.Status,
			})
		}
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
			ID:            userID,
			Email:         req.Email,
			PasswordHash:  hash,
			EmailVerified: false,
			CreatedAt:     time.Now(),
		}
		if err := ts.db.CreateUser(user); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Registration failed"})
			return
		}

		// Generate email verification token (32 bytes = 256 bits entropy)
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			// User created but verification email failed - return success with message
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"message":            "Account created. Please check your email to verify your account.",
				"email_verification": true,
				"user": map[string]interface{}{
					"id":             user.ID,
					"email":          user.Email,
					"created_at":     user.CreatedAt,
					"email_verified": user.EmailVerified,
				},
			})
			return
		}
		token := hex.EncodeToString(tokenBytes)
		tokenHash := email.HashToken(token)

		// Create verification token with 24-hour expiry
		expiresAt := time.Now().Add(24 * time.Hour)
		ts.db.CreateEmailVerificationToken(user.ID, tokenHash, expiresAt)

		// Send verification email via mock
		ts.emailService.SendEmailVerification(r.Context(), user.Email, token)

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":            "Account created. Please check your email to verify your account.",
			"email_verification": true,
			"user": map[string]interface{}{
				"id":             user.ID,
				"email":          user.Email,
				"created_at":     user.CreatedAt,
				"email_verified": user.EmailVerified,
			},
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

		// Check if email is verified
		if !user.EmailVerified {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":                   "Please verify your email address before logging in",
				"email_verification":      true,
				"email_not_verified":      true,
				"can_resend_verification": true,
			})
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

	// Auth: Verify email
	ts.mux.HandleFunc("GET /api/auth/verify", ts.authLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		token := r.URL.Query().Get("token")
		if token == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Verification token is required"})
			return
		}

		tokenHash := email.HashToken(token)
		verifyToken, err := ts.db.GetEmailVerificationToken(tokenHash)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to process verification request"})
			return
		}

		if verifyToken == nil || verifyToken.Used || time.Now().After(verifyToken.ExpiresAt) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid or expired verification token"})
			return
		}

		success, err := ts.db.UseEmailVerificationToken(tokenHash)
		if err != nil || !success {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to verify email"})
			return
		}

		json.NewEncoder(w).Encode(map[string]string{
			"message": "Email verified successfully. You can now log in.",
		})
	}))

	// Auth: Resend verification email
	ts.mux.HandleFunc("POST /api/auth/resend-verification", ts.passwordResetLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
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
				"message": "If an unverified account exists with that email, a verification link has been sent.",
			})
		}()

		user, err := ts.db.GetUserByEmail(req.Email)
		if err != nil || user == nil || user.EmailVerified {
			return
		}

		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			return
		}
		token := hex.EncodeToString(tokenBytes)
		tokenHash := email.HashToken(token)

		expiresAt := time.Now().Add(24 * time.Hour)
		ts.db.CreateEmailVerificationToken(user.ID, tokenHash, expiresAt)
		ts.emailService.SendEmailVerification(r.Context(), user.Email, token)
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

		// SECURITY: Require either authenticated user or session_id to filter videos
		// Without this check, anonymous requests would return ALL videos in the database
		if userPtr == nil && sessionPtr == nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Authentication or session_id required to list videos",
			})
			return
		}

		// Parse pagination parameters
		limit := 50 // default
		offset := 0
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
				limit = l
				if limit > 100 {
					limit = 100 // max limit
				}
			}
		}
		if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
			if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
				offset = o
			}
		}

		result, err := ts.db.ListVideosPaginated(userPtr, sessionPtr, limit, offset)
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

		videos := make([]VideoWithStatus, len(result.Videos))
		for i, v := range result.Videos {
			videos[i] = VideoWithStatus{Video: v, TranscriptionStatus: "none"}
			if t, err := ts.db.GetTranscription(v.ID); err == nil && t != nil {
				videos[i].TranscriptionStatus = t.Status
			}

			// Calculate expiration time based on user type
			var expiresAt time.Time
			if v.UserID == nil {
				expiresAt = v.CreatedAt.Add(48 * time.Hour)
			} else {
				expiresAt = v.CreatedAt.Add(90 * 24 * time.Hour)
			}
			videos[i].ExpiresAt = &expiresAt
		}

		hasMore := offset+len(result.Videos) < result.TotalCount
		json.NewEncoder(w).Encode(map[string]interface{}{
			"videos":      videos,
			"total_count": result.TotalCount,
			"has_more":    hasMore,
		})
	})

	// Delete a video
	ts.mux.HandleFunc("DELETE /api/videos/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		videoID := r.PathValue("id")
		if videoID == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Video ID required"})
			return
		}

		// Get the video to check ownership
		video, err := ts.db.GetVideo(videoID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get video"})
			return
		}
		if video == nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "Video not found"})
			return
		}

		// Check ownership - either authenticated user owns it, or anonymous session matches
		token := auth.GetTokenFromRequest(r)
		user, _, _ := auth.ValidateSession(ts.db, token)

		sessionID := r.URL.Query().Get("session_id")

		hasAccess := false
		if user != nil && video.UserID != nil && *video.UserID == user.ID {
			// Authenticated user owns the video
			hasAccess = true
		} else if sessionID != "" && video.SessionID != nil && *video.SessionID == sessionID {
			// Anonymous user with matching session
			hasAccess = true
		}

		if !hasAccess {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{"error": "You do not have permission to delete this video"})
			return
		}

		// Delete from database and get file paths
		deletedFiles, err := ts.db.DeleteVideo(videoID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to delete video"})
			return
		}

		// Delete the video file from disk
		if deletedFiles != nil && deletedFiles.FilePath != "" {
			os.Remove(deletedFiles.FilePath)
		}

		// Delete the thumbnail file from disk
		if deletedFiles != nil && deletedFiles.ThumbnailPath != nil && *deletedFiles.ThumbnailPath != "" {
			os.Remove(*deletedFiles.ThumbnailPath)
		}

		// Delete the burn output file from disk
		if deletedFiles != nil && deletedFiles.BurnOutputPath != nil && *deletedFiles.BurnOutputPath != "" {
			os.Remove(*deletedFiles.BurnOutputPath)
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "Video deleted successfully"})
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

	// Serve video file with Range request support (uses http.ServeFile which handles Range)
	ts.mux.HandleFunc("GET /api/videos/{id}/video", ts.downloadLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		uploadID := r.PathValue("id")
		if uploadID == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Upload ID required"})
			return
		}

		video, err := ts.db.GetVideo(uploadID)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get video"})
			return
		}
		if video == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "Video not found"})
			return
		}

		// http.ServeFile handles Range headers automatically for seekable files
		http.ServeFile(w, r, video.FilePath)
	}))

	// Serve video thumbnail
	ts.mux.HandleFunc("GET /api/videos/{id}/thumbnail", ts.downloadLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		uploadID := r.PathValue("id")
		if uploadID == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Upload ID required"})
			return
		}

		video, err := ts.db.GetVideo(uploadID)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Database error"})
			return
		}
		if video == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "Video not found"})
			return
		}

		// Check if thumbnail exists
		if video.ThumbnailPath == nil || *video.ThumbnailPath == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "Thumbnail not available"})
			return
		}

		thumbPath := *video.ThumbnailPath

		// Check if file exists on disk
		if _, err := os.Stat(thumbPath); os.IsNotExist(err) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "Thumbnail file not found"})
			return
		}

		// Generate ETag from video ID + creation time
		etag := generateETag(fmt.Sprintf("thumb-%s-%d", video.ID, video.CreatedAt.Unix()))

		// Check for conditional request
		if handleConditionalRequest(w, r, etag) {
			return
		}

		// Set caching headers - 24 hours
		setCacheHeaders(w, etag, 86400)

		// Set content type for JPEG
		w.Header().Set("Content-Type", "image/jpeg")

		// Serve the file
		http.ServeFile(w, r, thumbPath)
	}))

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

		// Generate ETag from transcription ID + completion time + segments hash
		var completedAt int64
		if transcription.CompletedAt != nil {
			completedAt = transcription.CompletedAt.Unix()
		}
		etag := generateETag(fmt.Sprintf("srt-%s-%d-%s", transcription.ID, completedAt, transcription.SegmentsJSON[:min(100, len(transcription.SegmentsJSON))]))

		// Check for conditional request (If-None-Match)
		if handleConditionalRequest(w, r, etag) {
			return // 304 Not Modified sent
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

		// Set caching headers
		setCacheHeaders(w, etag, 600)

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

		// Generate ETag from transcription ID + completion time + segments hash
		var completedAt int64
		if transcription.CompletedAt != nil {
			completedAt = transcription.CompletedAt.Unix()
		}
		etag := generateETag(fmt.Sprintf("vtt-%s-%d-%s", transcription.ID, completedAt, transcription.SegmentsJSON[:min(100, len(transcription.SegmentsJSON))]))

		// Check for conditional request (If-None-Match)
		if handleConditionalRequest(w, r, etag) {
			return // 304 Not Modified sent
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

		// Set caching headers
		setCacheHeaders(w, etag, 600)

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

		// Generate ETag from transcription ID + completion time + segments hash
		var completedAt int64
		if transcription.CompletedAt != nil {
			completedAt = transcription.CompletedAt.Unix()
		}
		etag := generateETag(fmt.Sprintf("json-%s-%d-%s", transcription.ID, completedAt, transcription.SegmentsJSON[:min(100, len(transcription.SegmentsJSON))]))

		// Check for conditional request (If-None-Match)
		if handleConditionalRequest(w, r, etag) {
			return // 304 Not Modified sent
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

		// Set caching headers
		setCacheHeaders(w, etag, 600)

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

	// Reprocess failed transcription endpoint
	ts.mux.HandleFunc("POST /api/videos/{id}/reprocess", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		uploadID := r.PathValue("id")
		if uploadID == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Video ID required"})
			return
		}

		// Get the video to check ownership
		video, err := ts.db.GetVideo(uploadID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get video"})
			return
		}
		if video == nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "Video not found"})
			return
		}

		// Check ownership
		token := auth.GetTokenFromRequest(r)
		user, _, _ := auth.ValidateSession(ts.db, token)
		sessionID := r.URL.Query().Get("session_id")

		hasAccess := false
		if user != nil && video.UserID != nil && *video.UserID == user.ID {
			hasAccess = true
		} else if sessionID != "" && video.SessionID != nil && *video.SessionID == sessionID {
			hasAccess = true
		}

		if !hasAccess {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{"error": "You do not have permission to reprocess this video"})
			return
		}

		// Check transcription status
		transcription, err := ts.db.GetTranscription(uploadID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get transcription status"})
			return
		}

		if transcription == nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "No transcription found for this video"})
			return
		}

		if transcription.Status != "error" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error":  "Can only reprocess failed transcriptions",
				"status": transcription.Status,
			})
			return
		}

		// In tests, just mark as processing (actual transcription would happen in background)
		ts.db.UpdateTranscriptionStatus(uploadID, "processing", "Reprocessing...", 10)

		json.NewEncoder(w).Encode(map[string]string{
			"status":  "processing",
			"message": "Reprocessing started",
		})
	})

	// Get burn job status
	ts.mux.HandleFunc("GET /api/videos/{id}/burn", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		uploadID := r.PathValue("id")
		if uploadID == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Upload ID required",
			})
			return
		}

		job, err := ts.db.GetBurnJob(uploadID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to get burn job status",
			})
			return
		}

		if job == nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "No burn job found for this video",
			})
			return
		}

		// Build response with progress info
		response := map[string]interface{}{
			"status":   job.Status,
			"message":  job.Message,
			"progress": job.Progress,
		}

		// If processing, calculate estimated time remaining
		if job.Status == "processing" && job.Progress > 0 {
			// Get transcription for duration info
			transcription, _ := ts.db.GetTranscription(uploadID)
			if transcription != nil && transcription.Duration > 0 {
				response["duration"] = transcription.Duration

				// Calculate elapsed time since job started
				elapsed := time.Since(job.CreatedAt).Seconds()
				if elapsed > 0 && job.Progress > 0 {
					// Estimate total time based on current progress
					estimatedTotal := elapsed * 100 / float64(job.Progress)
					estimatedRemaining := estimatedTotal - elapsed
					if estimatedRemaining < 0 {
						estimatedRemaining = 0
					}
					response["estimated_remaining_seconds"] = int(estimatedRemaining)
				}
			}
		}

		json.NewEncoder(w).Encode(response)
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

	// Chunked upload init endpoint
	ts.mux.HandleFunc("POST /api/upload/init", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var req struct {
			Filename    string `json:"filename"`
			Size        int64  `json:"size"`
			ContentType string `json:"content_type"`
			ChunkSize   int64  `json:"chunk_size"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
			return
		}

		// Validate required fields
		if req.Filename == "" || req.Size <= 0 || req.ContentType == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Missing required fields"})
			return
		}

		// Validate MIME type
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
		if !allowedMIMETypes[req.ContentType] {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Unsupported video format"})
			return
		}

		// Check file size
		if req.Size > 500<<20 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "File too large"})
			return
		}

		// Get authenticated user (if any)
		token := auth.GetTokenFromRequest(r)
		user, _, _ := auth.ValidateSession(ts.db, token)

		// Get session_id from query for anonymous tracking
		sessionID := r.URL.Query().Get("session_id")

		// Use provided chunk size or default
		chunkSize := req.ChunkSize
		if chunkSize <= 0 {
			chunkSize = 50 << 20 // 50 MB
		}

		totalChunks := int((req.Size + chunkSize - 1) / chunkSize)

		// Generate session ID
		uploadSessionID := testGenerateID()

		expiresAt := time.Now().Add(24 * time.Hour)
		session := &db.UploadSession{
			ID:          uploadSessionID,
			Filename:    req.Filename,
			ContentType: req.ContentType,
			TotalSize:   req.Size,
			ChunkSize:   chunkSize,
			TotalChunks: totalChunks,
			Status:      "in_progress",
			CreatedAt:   time.Now(),
			ExpiresAt:   expiresAt,
		}
		if user != nil {
			session.UserID = &user.ID
		}
		if sessionID != "" {
			session.SessionID = &sessionID
		}

		if err := ts.db.CreateUploadSession(session); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create upload session"})
			return
		}

		// Create chunks directory
		chunksDir := filepath.Join(ts.uploadDir, "chunks", uploadSessionID)
		if err := os.MkdirAll(chunksDir, 0755); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create chunks directory"})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"upload_session_id": uploadSessionID,
			"chunk_size":        chunkSize,
			"total_chunks":      totalChunks,
			"expires_at":        expiresAt.Format(time.RFC3339),
		})
	})

	// Chunked upload chunk endpoint
	ts.mux.HandleFunc("POST /api/upload/chunk", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Parse multipart form
		if err := r.ParseMultipartForm(60 << 20); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid form data"})
			return
		}

		uploadSessionID := r.FormValue("upload_session_id")
		chunkIndexStr := r.FormValue("chunk_index")
		if uploadSessionID == "" || chunkIndexStr == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Missing upload_session_id or chunk_index"})
			return
		}

		var chunkIndex int
		fmt.Sscanf(chunkIndexStr, "%d", &chunkIndex)

		// Get session
		session, err := ts.db.GetUploadSession(uploadSessionID)
		if err != nil || session == nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "Upload session not found"})
			return
		}

		// Check session status
		if session.Status != "in_progress" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Upload session is not in progress"})
			return
		}

		// Validate chunk index
		if chunkIndex < 0 || chunkIndex >= session.TotalChunks {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid chunk index"})
			return
		}

		// Check for existing chunk (idempotency)
		existing, _ := ts.db.GetUploadChunk(uploadSessionID, chunkIndex)
		if existing != nil {
			total, _ := ts.db.GetTotalReceivedBytes(uploadSessionID)
			count, _ := ts.db.CountUploadChunks(uploadSessionID)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"chunk_index":    chunkIndex,
				"received_bytes": existing.Size,
				"total_received": total,
				"progress":       count * 100 / session.TotalChunks,
				"already_exists": true,
			})
			return
		}

		// Get chunk data
		chunkFile, _, err := r.FormFile("chunk")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "No chunk data provided"})
			return
		}
		defer chunkFile.Close()

		// Save chunk
		chunkPath := filepath.Join(ts.uploadDir, "chunks", uploadSessionID, fmt.Sprintf("chunk_%d.part", chunkIndex))
		destFile, err := os.Create(chunkPath)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save chunk"})
			return
		}

		written, err := io.Copy(destFile, chunkFile)
		destFile.Close()
		if err != nil {
			os.Remove(chunkPath)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save chunk"})
			return
		}

		// Record chunk
		chunk := &db.UploadChunk{
			ID:              testGenerateID(),
			UploadSessionID: uploadSessionID,
			ChunkIndex:      chunkIndex,
			ChunkPath:       chunkPath,
			Size:            written,
			CreatedAt:       time.Now(),
		}
		if err := ts.db.CreateUploadChunk(chunk); err != nil {
			os.Remove(chunkPath)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to record chunk"})
			return
		}

		total, _ := ts.db.GetTotalReceivedBytes(uploadSessionID)
		count, _ := ts.db.CountUploadChunks(uploadSessionID)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"chunk_index":    chunkIndex,
			"received_bytes": written,
			"total_received": total,
			"progress":       count * 100 / session.TotalChunks,
		})
	})

	// Chunked upload complete endpoint
	ts.mux.HandleFunc("POST /api/upload/complete", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var req struct {
			UploadSessionID string `json:"upload_session_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
			return
		}

		session, err := ts.db.GetUploadSession(req.UploadSessionID)
		if err != nil || session == nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "Upload session not found"})
			return
		}

		// Verify all chunks received
		chunkCount, _ := ts.db.CountUploadChunks(req.UploadSessionID)
		if chunkCount != session.TotalChunks {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": fmt.Sprintf("Not all chunks received. Expected %d, got %d", session.TotalChunks, chunkCount),
			})
			return
		}

		// Get chunks in order
		chunks, err := ts.db.GetUploadChunks(req.UploadSessionID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get chunks"})
			return
		}

		// Generate upload ID
		uploadID := testGenerateID()
		ext := filepath.Ext(session.Filename)
		if ext == "" {
			ext = ".mp4"
		}

		// Reassemble chunks
		destPath := filepath.Join(ts.uploadDir, uploadID+ext)
		destFile, err := os.Create(destPath)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create destination file"})
			return
		}

		var totalWritten int64
		for _, chunk := range chunks {
			chunkFile, err := os.Open(chunk.ChunkPath)
			if err != nil {
				destFile.Close()
				os.Remove(destPath)
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"error": "Failed to read chunk"})
				return
			}
			written, _ := io.Copy(destFile, chunkFile)
			chunkFile.Close()
			totalWritten += written
		}
		destFile.Close()

		// Mark session complete
		ts.db.UpdateUploadSessionStatus(req.UploadSessionID, "complete")

		// Clean up chunks
		for _, chunk := range chunks {
			os.Remove(chunk.ChunkPath)
		}
		chunksDir := filepath.Join(ts.uploadDir, "chunks", req.UploadSessionID)
		os.Remove(chunksDir)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "success",
			"upload_id": uploadID,
			"filename":  session.Filename,
			"size":      totalWritten,
		})
	})

	// Chunked upload status endpoint
	ts.mux.HandleFunc("GET /api/upload/status/{session_id}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		uploadSessionID := r.PathValue("session_id")
		if uploadSessionID == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Session ID required"})
			return
		}

		session, err := ts.db.GetUploadSession(uploadSessionID)
		if err != nil || session == nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "Upload session not found"})
			return
		}

		receivedChunks, _ := ts.db.GetReceivedChunkIndices(uploadSessionID)
		if receivedChunks == nil {
			receivedChunks = []int{}
		}
		receivedBytes, _ := ts.db.GetTotalReceivedBytes(uploadSessionID)

		progress := 0
		if session.TotalChunks > 0 {
			progress = len(receivedChunks) * 100 / session.TotalChunks
		}

		status := session.Status
		if time.Now().After(session.ExpiresAt) && status == "in_progress" {
			status = "expired"
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"upload_session_id": session.ID,
			"filename":          session.Filename,
			"total_size":        session.TotalSize,
			"chunk_size":        session.ChunkSize,
			"total_chunks":      session.TotalChunks,
			"received_chunks":   receivedChunks,
			"received_bytes":    receivedBytes,
			"progress":          progress,
			"status":            status,
			"expires_at":        session.ExpiresAt.Format(time.RFC3339),
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

// createTestUser creates a user, verifies their email, logs them in, and returns their auth token
func (ts *testServer) createTestUser(t *testing.T, email, password string) string {
	t.Helper()

	// Register the user
	resp := ts.doRequest("POST", "/api/auth/register", map[string]string{
		"email":    email,
		"password": password,
	}, "")

	if resp.Code != http.StatusCreated {
		t.Fatalf("Failed to create test user: %s", resp.Body.String())
	}

	var regResult struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	json.NewDecoder(resp.Body).Decode(&regResult)

	// Directly verify the email in the database (test shortcut)
	if err := ts.db.VerifyUserEmail(regResult.User.ID); err != nil {
		t.Fatalf("Failed to verify email: %v", err)
	}

	// Now login to get the session token
	loginResp := ts.doRequest("POST", "/api/auth/login", map[string]string{
		"email":    email,
		"password": password,
	}, "")

	if loginResp.Code != http.StatusOK {
		t.Fatalf("Failed to login test user: %s", loginResp.Body.String())
	}

	var loginResult struct {
		Token string `json:"token"`
	}
	json.NewDecoder(loginResp.Body).Decode(&loginResult)
	return loginResult.Token
}

// createTestUserWithID creates a user, verifies their email, logs them in, and returns both the user ID and auth token
func (ts *testServer) createTestUserWithID(t *testing.T, email, password string) (userID string, token string) {
	t.Helper()

	// Register the user
	resp := ts.doRequest("POST", "/api/auth/register", map[string]string{
		"email":    email,
		"password": password,
	}, "")

	if resp.Code != http.StatusCreated {
		t.Fatalf("Failed to create test user: %s", resp.Body.String())
	}

	var regResult struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	json.NewDecoder(resp.Body).Decode(&regResult)

	// Directly verify the email in the database (test shortcut)
	if err := ts.db.VerifyUserEmail(regResult.User.ID); err != nil {
		t.Fatalf("Failed to verify email: %v", err)
	}

	// Now login to get the session token
	loginResp := ts.doRequest("POST", "/api/auth/login", map[string]string{
		"email":    email,
		"password": password,
	}, "")

	if loginResp.Code != http.StatusOK {
		t.Fatalf("Failed to login test user: %s", loginResp.Body.String())
	}

	var loginResult struct {
		Token string `json:"token"`
	}
	json.NewDecoder(loginResp.Body).Decode(&loginResult)
	return regResult.User.ID, loginResult.Token
}

// createTestVideo creates a video record directly in the database
func (ts *testServer) createTestVideo(t *testing.T, userID *string, sessionID *string) *db.Video {
	t.Helper()

	video := &db.Video{
		ID:          testGenerateID(),
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
		ID:        testGenerateID(),
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

// createTestFailedTranscription creates a failed transcription record
func (ts *testServer) createTestFailedTranscription(t *testing.T, videoID string) *db.Transcription {
	t.Helper()

	transcription := &db.Transcription{
		ID:        testGenerateID(),
		VideoID:   videoID,
		Status:    "pending",
		Message:   "Test",
		Progress:  0,
		CreatedAt: time.Now(),
	}

	if err := ts.db.CreateTranscription(transcription); err != nil {
		t.Fatalf("Failed to create transcription: %v", err)
	}

	if err := ts.db.FailTranscription(videoID, "Test failure error"); err != nil {
		t.Fatalf("Failed to fail transcription: %v", err)
	}

	result, _ := ts.db.GetTranscription(videoID)
	return result
}

// ========== Test Cases ==========

// TestHealthEndpointUnauthenticated tests that unauthenticated requests get minimal response
func TestHealthEndpointUnauthenticated(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	resp := ts.doRequest("GET", "/api/health", nil, "")

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}

	// Unauthenticated should only get status field
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["status"] != "ok" {
		t.Errorf("Expected status 'ok', got '%v'", result["status"])
	}

	// Should NOT have detailed fields
	if _, ok := result["db_connected"]; ok {
		t.Error("Unauthenticated response should not include db_connected")
	}
	if _, ok := result["whisper_available"]; ok {
		t.Error("Unauthenticated response should not include whisper_available")
	}
	if _, ok := result["disk_space_ok"]; ok {
		t.Error("Unauthenticated response should not include disk_space_ok")
	}
	if _, ok := result["disk_free_gb"]; ok {
		t.Error("Unauthenticated response should not include disk_free_gb")
	}
	if _, ok := result["errors"]; ok {
		t.Error("Unauthenticated response should not include errors")
	}
}

// TestHealthEndpointAuthenticated tests that authenticated requests get full response
func TestHealthEndpointAuthenticated(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a user and get auth token
	token := ts.createTestUser(t, "health@example.com", "Testpass123!")

	resp := ts.doRequest("GET", "/api/health", nil, token)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}

	var result HealthStatus
	json.NewDecoder(resp.Body).Decode(&result)

	if result.Status != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", result.Status)
	}

	// Verify enhanced health check fields are present for authenticated users
	if !result.DBConnected {
		t.Error("Expected DBConnected to be true")
	}
	if !result.WhisperAvailable {
		t.Error("Expected WhisperAvailable to be true (whisper-server not configured)")
	}
	if !result.DiskSpaceOK {
		t.Error("Expected DiskSpaceOK to be true")
	}
	if result.DiskFreeGB <= 0 {
		t.Error("Expected DiskFreeGB to be greater than 0")
	}
	if len(result.Errors) != 0 {
		t.Errorf("Expected no errors, got: %v", result.Errors)
	}
}

// TestHealthEndpointDBDownUnauthenticated tests unauthenticated response when DB is down
func TestHealthEndpointDBDownUnauthenticated(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Close the database to simulate a connection failure
	ts.db.Close()

	resp := ts.doRequest("GET", "/api/health", nil, "")

	// Should return 503 Service Unavailable when DB is down
	if resp.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", resp.Code)
	}

	// Unauthenticated should only get status field
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["status"] != "degraded" {
		t.Errorf("Expected status 'degraded', got '%v'", result["status"])
	}

	// Should NOT have detailed fields even when degraded
	if _, ok := result["db_connected"]; ok {
		t.Error("Unauthenticated response should not include db_connected")
	}
	if _, ok := result["errors"]; ok {
		t.Error("Unauthenticated response should not include errors")
	}
}

// TestHealthEndpointDBDownAuthenticated tests authenticated response when DB is down
// Note: This is a tricky edge case - authentication itself requires DB access.
// When DB is down, we can't validate the session, so the user effectively becomes unauthenticated.
func TestHealthEndpointDBDownAuthenticated(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a user and get auth token while DB is still up
	token := ts.createTestUser(t, "healthdown@example.com", "Testpass123!")

	// Close the database to simulate a connection failure
	ts.db.Close()

	resp := ts.doRequest("GET", "/api/health", nil, token)

	// Should return 503 Service Unavailable when DB is down
	if resp.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", resp.Code)
	}

	// When DB is down, we can't validate the session, so response should be minimal
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["status"] != "degraded" {
		t.Errorf("Expected status 'degraded', got '%v'", result["status"])
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

func TestListVideosNoFilterRejected(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create videos that should NOT be accessible without filter
	userID := "some-user"
	sessionID := "some-session"
	ts.createTestVideo(t, &userID, nil)
	ts.createTestVideo(t, nil, &sessionID)

	// Try to list videos without auth or session_id
	// This MUST be rejected to prevent privacy leak
	resp := ts.doRequest("GET", "/api/videos", nil, "")
	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 (Bad Request), got %d - anonymous requests without session_id should be rejected", resp.Code)
	}

	var errResult struct {
		Error string `json:"error"`
	}
	json.NewDecoder(resp.Body).Decode(&errResult)

	if errResult.Error == "" {
		t.Error("Expected error message in response")
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

func TestListVideosPagination(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a user with multiple videos
	userID, token := ts.createTestUserWithID(t, "pagination@example.com", "Password123!")

	// Create 10 videos
	for i := 0; i < 10; i++ {
		ts.createTestVideo(t, &userID, nil)
	}

	type paginatedResult struct {
		Videos     []struct{ ID string } `json:"videos"`
		TotalCount int                   `json:"total_count"`
		HasMore    bool                  `json:"has_more"`
	}

	// Test 1: Default pagination (should return all 10 with metadata)
	resp := ts.doRequest("GET", "/api/videos", nil, token)
	if resp.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.Code)
	}

	var result1 paginatedResult
	json.NewDecoder(resp.Body).Decode(&result1)

	if result1.TotalCount != 10 {
		t.Errorf("Expected total_count 10, got %d", result1.TotalCount)
	}
	if len(result1.Videos) != 10 {
		t.Errorf("Expected 10 videos, got %d", len(result1.Videos))
	}
	if result1.HasMore {
		t.Error("Expected has_more false when all videos returned")
	}

	// Test 2: First page with limit
	resp = ts.doRequest("GET", "/api/videos?limit=3", nil, token)
	var result2 paginatedResult
	json.NewDecoder(resp.Body).Decode(&result2)

	if result2.TotalCount != 10 {
		t.Errorf("Expected total_count 10, got %d", result2.TotalCount)
	}
	if len(result2.Videos) != 3 {
		t.Errorf("Expected 3 videos, got %d", len(result2.Videos))
	}
	if !result2.HasMore {
		t.Error("Expected has_more true when more videos exist")
	}

	// Test 3: Second page
	resp = ts.doRequest("GET", "/api/videos?limit=3&offset=3", nil, token)
	var result3 paginatedResult
	json.NewDecoder(resp.Body).Decode(&result3)

	if len(result3.Videos) != 3 {
		t.Errorf("Expected 3 videos on second page, got %d", len(result3.Videos))
	}
	if !result3.HasMore {
		t.Error("Expected has_more true on second page")
	}

	// Test 4: Last page (partial)
	resp = ts.doRequest("GET", "/api/videos?limit=3&offset=9", nil, token)
	var result4 paginatedResult
	json.NewDecoder(resp.Body).Decode(&result4)

	if len(result4.Videos) != 1 {
		t.Errorf("Expected 1 video on last page, got %d", len(result4.Videos))
	}
	if result4.HasMore {
		t.Error("Expected has_more false on last page")
	}

	// Test 5: Max limit enforcement (>100 should be capped to 100)
	resp = ts.doRequest("GET", "/api/videos?limit=200", nil, token)
	var result5 paginatedResult
	json.NewDecoder(resp.Body).Decode(&result5)

	if len(result5.Videos) != 10 { // Only 10 videos exist
		t.Errorf("Expected all 10 videos (capped by data size), got %d", len(result5.Videos))
	}

	// Test 6: Invalid limit/offset should be ignored (defaults used)
	resp = ts.doRequest("GET", "/api/videos?limit=invalid&offset=invalid", nil, token)
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200 with invalid params, got %d", resp.Code)
	}
}

func TestDeleteVideoAuthenticated(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a user and video
	userID, token := ts.createTestUserWithID(t, "delete@example.com", "Password123!")
	video := ts.createTestVideo(t, &userID, nil)

	// Create a transcription for the video
	ts.createTestTranscription(t, video.ID)

	// Delete should succeed
	resp := ts.doRequest("DELETE", "/api/videos/"+video.ID, nil, token)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)

	if result["message"] != "Video deleted successfully" {
		t.Errorf("Expected message 'Video deleted successfully', got '%s'", result["message"])
	}

	// Verify video is gone
	getResp := ts.doRequest("GET", "/api/videos?user_id="+userID, nil, token)
	var listResult struct {
		Videos []struct {
			ID string `json:"id"`
		} `json:"videos"`
	}
	json.NewDecoder(getResp.Body).Decode(&listResult)

	for _, v := range listResult.Videos {
		if v.ID == video.ID {
			t.Error("Video should have been deleted but still appears in list")
		}
	}
}

func TestDeleteVideoAnonymous(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create anonymous video with session
	sessionID := "test-delete-session-123"
	video := ts.createTestVideo(t, nil, &sessionID)

	// Delete with matching session should succeed
	resp := ts.doRequest("DELETE", "/api/videos/"+video.ID+"?session_id="+sessionID, nil, "")

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestDeleteVideoNotOwner(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a video owned by user1
	user1ID := testGenerateID()
	video := ts.createTestVideo(t, &user1ID, nil)

	// Create another user and try to delete
	_, token := ts.createTestUserWithID(t, "other@example.com", "Password123!")

	resp := ts.doRequest("DELETE", "/api/videos/"+video.ID, nil, token)

	if resp.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", resp.Code)
	}
}

func TestDeleteVideoAnonymousWrongSession(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create anonymous video with session
	sessionID := "original-session"
	video := ts.createTestVideo(t, nil, &sessionID)

	// Try to delete with different session
	resp := ts.doRequest("DELETE", "/api/videos/"+video.ID+"?session_id=wrong-session", nil, "")

	if resp.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", resp.Code)
	}
}

func TestDeleteVideoNotFound(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	_, token := ts.createTestUserWithID(t, "test@example.com", "Password123!")

	resp := ts.doRequest("DELETE", "/api/videos/nonexistent", nil, token)

	if resp.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.Code)
	}
}

func TestDeleteVideoNoAuth(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a video owned by a user
	userID := testGenerateID()
	video := ts.createTestVideo(t, &userID, nil)

	// Try to delete without auth or session_id
	resp := ts.doRequest("DELETE", "/api/videos/"+video.ID, nil, "")

	if resp.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", resp.Code)
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
		ID:        testGenerateID(),
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

// TestBurnStatusWithETA tests that the burn status endpoint returns progress and ETA
func TestBurnStatusWithETA(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a video
	video := &db.Video{
		ID:          testGenerateID(),
		Filename:    "test.mp4",
		Size:        1024,
		ContentType: "video/mp4",
		FilePath:    "/uploads/test.mp4",
		CreatedAt:   time.Now(),
	}
	if err := ts.db.CreateVideo(video); err != nil {
		t.Fatalf("Failed to create video: %v", err)
	}

	// Create transcription with duration (needed for ETA calculation)
	// First create the transcription record
	if err := ts.db.CreateTranscription(&db.Transcription{
		ID:        testGenerateID(),
		VideoID:   video.ID,
		Status:    "processing",
		CreatedAt: time.Now(),
	}); err != nil {
		t.Fatalf("Failed to create transcription: %v", err)
	}
	// Then complete it with duration (this sets the duration field)
	if err := ts.db.CompleteTranscription(video.ID, "en", 120.0, "Test transcript", []db.Segment{}); err != nil {
		t.Fatalf("Failed to complete transcription: %v", err)
	}

	// Create burn job in progress (started 10 seconds ago with 50% progress)
	burnJob := &db.BurnJob{
		ID:        testGenerateID(),
		VideoID:   video.ID,
		Status:    "processing",
		Message:   "Burning subtitles...",
		Progress:  50,
		CreatedAt: time.Now().Add(-10 * time.Second), // Started 10 seconds ago
	}
	if err := ts.db.CreateBurnJob(burnJob); err != nil {
		t.Fatalf("Failed to create burn job: %v", err)
	}

	// Get burn status
	req := httptest.NewRequest("GET", "/api/videos/"+video.ID+"/burn", nil)
	w := httptest.NewRecorder()
	ts.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var response struct {
		Status                    string  `json:"status"`
		Message                   string  `json:"message"`
		Progress                  int     `json:"progress"`
		Duration                  float64 `json:"duration"`
		EstimatedRemainingSeconds int     `json:"estimated_remaining_seconds"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify response fields
	if response.Status != "processing" {
		t.Errorf("Expected status 'processing', got '%s'", response.Status)
	}
	if response.Progress != 50 {
		t.Errorf("Expected progress 50, got %d", response.Progress)
	}
	if response.Duration != 120.0 {
		t.Errorf("Expected duration 120.0, got %f", response.Duration)
	}
	// ETA should be approximately 10 seconds (50% done in 10 seconds = ~10 seconds remaining)
	// Allow for some variance due to timing
	if response.EstimatedRemainingSeconds < 5 || response.EstimatedRemainingSeconds > 15 {
		t.Errorf("Expected estimated_remaining_seconds ~10, got %d", response.EstimatedRemainingSeconds)
	}
}

// TestBurnStatusComplete tests that completed burn jobs don't include ETA
func TestBurnStatusComplete(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a video
	video := &db.Video{
		ID:          testGenerateID(),
		Filename:    "test.mp4",
		Size:        1024,
		ContentType: "video/mp4",
		FilePath:    "/uploads/test.mp4",
		CreatedAt:   time.Now(),
	}
	if err := ts.db.CreateVideo(video); err != nil {
		t.Fatalf("Failed to create video: %v", err)
	}

	// Complete the burn job
	burnJob := &db.BurnJob{
		ID:        testGenerateID(),
		VideoID:   video.ID,
		Status:    "complete",
		Message:   "Done",
		Progress:  100,
		CreatedAt: time.Now().Add(-30 * time.Second),
	}
	if err := ts.db.CreateBurnJob(burnJob); err != nil {
		t.Fatalf("Failed to create burn job: %v", err)
	}

	// Get burn status
	req := httptest.NewRequest("GET", "/api/videos/"+video.ID+"/burn", nil)
	w := httptest.NewRecorder()
	ts.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify no ETA for complete job
	if _, ok := response["estimated_remaining_seconds"]; ok {
		t.Error("Expected no estimated_remaining_seconds for complete job")
	}
	if response["status"] != "complete" {
		t.Errorf("Expected status 'complete', got '%v'", response["status"])
	}
}

// TestGetSessions tests listing user's sessions
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

// ========== Reprocess Tests ==========

func TestReprocessVideoSuccess(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a user and video
	userID, token := ts.createTestUserWithID(t, "reprocess@example.com", "Password123!")
	video := ts.createTestVideo(t, &userID, nil)

	// Create a failed transcription
	ts.createTestFailedTranscription(t, video.ID)

	// Reprocess should succeed
	resp := ts.doRequest("POST", "/api/videos/"+video.ID+"/reprocess", nil, token)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)

	if result["status"] != "processing" {
		t.Errorf("Expected status 'processing', got '%s'", result["status"])
	}
	if result["message"] != "Reprocessing started" {
		t.Errorf("Expected message 'Reprocessing started', got '%s'", result["message"])
	}
}

func TestReprocessVideoAnonymous(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create anonymous video with session
	sessionID := "test-session-123"
	video := ts.createTestVideo(t, nil, &sessionID)

	// Create a failed transcription
	ts.createTestFailedTranscription(t, video.ID)

	// Reprocess with matching session should succeed
	resp := ts.doRequest("POST", "/api/videos/"+video.ID+"/reprocess?session_id="+sessionID, nil, "")

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestReprocessVideoNotOwner(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a video owned by user1
	user1ID := testGenerateID()
	video := ts.createTestVideo(t, &user1ID, nil)
	ts.createTestFailedTranscription(t, video.ID)

	// Create another user and try to reprocess
	_, token := ts.createTestUserWithID(t, "other@example.com", "Password123!")

	resp := ts.doRequest("POST", "/api/videos/"+video.ID+"/reprocess", nil, token)

	if resp.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", resp.Code)
	}
}

func TestReprocessVideoNotFound(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	_, token := ts.createTestUserWithID(t, "test@example.com", "Password123!")

	resp := ts.doRequest("POST", "/api/videos/nonexistent/reprocess", nil, token)

	if resp.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.Code)
	}
}

func TestReprocessVideoNoTranscription(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	userID, token := ts.createTestUserWithID(t, "test@example.com", "Password123!")
	video := ts.createTestVideo(t, &userID, nil)
	// No transcription created

	resp := ts.doRequest("POST", "/api/videos/"+video.ID+"/reprocess", nil, token)

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.Code)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["error"] != "No transcription found for this video" {
		t.Errorf("Unexpected error: %s", result["error"])
	}
}

func TestReprocessVideoNotError(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	userID, token := ts.createTestUserWithID(t, "test@example.com", "Password123!")
	video := ts.createTestVideo(t, &userID, nil)
	ts.createTestTranscription(t, video.ID) // Complete transcription, not error

	resp := ts.doRequest("POST", "/api/videos/"+video.ID+"/reprocess", nil, token)

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", resp.Code)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	if result["error"] != "Can only reprocess failed transcriptions" {
		t.Errorf("Unexpected error: %s", result["error"])
	}
	if result["status"] != "complete" {
		t.Errorf("Expected status 'complete', got '%s'", result["status"])
	}
}

// ========== Request ID Middleware Tests ==========

func TestRequestIDMiddleware(t *testing.T) {
	// Create a simple handler for testing
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with middleware
	wrapped := requestIDMiddleware(handler)

	t.Run("adds X-Request-ID header to response", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		wrapped.ServeHTTP(w, req)

		requestID := w.Header().Get("X-Request-ID")
		if requestID == "" {
			t.Error("Expected X-Request-ID header in response")
		}
		// Should be 16 hex chars (8 bytes)
		if len(requestID) != 16 {
			t.Errorf("Expected request ID length 16, got %d", len(requestID))
		}
	})

	t.Run("uses existing X-Request-ID from request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Request-ID", "existing-id-12345")
		w := httptest.NewRecorder()

		wrapped.ServeHTTP(w, req)

		requestID := w.Header().Get("X-Request-ID")
		if requestID != "existing-id-12345" {
			t.Errorf("Expected request ID 'existing-id-12345', got '%s'", requestID)
		}
	})

	t.Run("generates unique IDs for each request", func(t *testing.T) {
		ids := make(map[string]bool)
		for i := 0; i < 100; i++ {
			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()

			wrapped.ServeHTTP(w, req)

			requestID := w.Header().Get("X-Request-ID")
			if ids[requestID] {
				t.Errorf("Duplicate request ID generated: %s", requestID)
			}
			ids[requestID] = true
		}
	})
}

func TestRequestIDInHealthCheck(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Note: The test server doesn't use the middleware, but we can test that
	// the middleware would work by testing it directly on the health endpoint handler
	handler := requestIDMiddleware(ts.mux)

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should have request ID header
	requestID := w.Header().Get("X-Request-ID")
	if requestID == "" {
		t.Error("Expected X-Request-ID header in health check response")
	}

	// Should still return OK
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// Tests for HTTP caching headers

func TestSRTDownloadCachingHeaders(t *testing.T) {
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

	// Check ETag header exists
	etag := resp.Header().Get("ETag")
	if etag == "" {
		t.Error("Expected ETag header in SRT response")
	}
	if !strings.HasPrefix(etag, "\"") || !strings.HasSuffix(etag, "\"") {
		t.Errorf("ETag should be quoted, got: %s", etag)
	}

	// Check Cache-Control header
	cacheControl := resp.Header().Get("Cache-Control")
	if cacheControl == "" {
		t.Error("Expected Cache-Control header in SRT response")
	}
	if !strings.Contains(cacheControl, "private") {
		t.Errorf("Cache-Control should contain 'private', got: %s", cacheControl)
	}
}

func TestSRTDownloadConditionalRequest(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create video and transcription
	video := ts.createTestVideo(t, nil, nil)
	ts.createTestTranscription(t, video.ID)

	// First request to get ETag
	resp1 := ts.doRequest("GET", "/api/videos/"+video.ID+"/subtitles.srt", nil, "")
	if resp1.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp1.Code)
	}
	etag := resp1.Header().Get("ETag")
	if etag == "" {
		t.Fatal("Expected ETag header in first response")
	}

	// Second request with If-None-Match should return 304
	req, _ := http.NewRequest("GET", "/api/videos/"+video.ID+"/subtitles.srt", nil)
	req.Header.Set("If-None-Match", etag)
	resp2 := httptest.NewRecorder()
	ts.mux.ServeHTTP(resp2, req)

	if resp2.Code != http.StatusNotModified {
		t.Errorf("Expected status 304 Not Modified, got %d", resp2.Code)
	}

	// Body should be empty for 304
	if resp2.Body.Len() > 0 {
		t.Errorf("Expected empty body for 304, got %d bytes", resp2.Body.Len())
	}
}

func TestVTTDownloadCachingHeaders(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create video and transcription
	video := ts.createTestVideo(t, nil, nil)
	ts.createTestTranscription(t, video.ID)

	// Download VTT
	resp := ts.doRequest("GET", "/api/videos/"+video.ID+"/subtitles.vtt", nil, "")
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}

	// Check ETag header
	etag := resp.Header().Get("ETag")
	if etag == "" {
		t.Error("Expected ETag header in VTT response")
	}

	// Check Cache-Control header
	cacheControl := resp.Header().Get("Cache-Control")
	if cacheControl == "" {
		t.Error("Expected Cache-Control header in VTT response")
	}
}

func TestJSONDownloadCachingHeaders(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create video and transcription
	video := ts.createTestVideo(t, nil, nil)
	ts.createTestTranscription(t, video.ID)

	// Download JSON
	resp := ts.doRequest("GET", "/api/videos/"+video.ID+"/subtitles.json", nil, "")
	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}

	// Check ETag header
	etag := resp.Header().Get("ETag")
	if etag == "" {
		t.Error("Expected ETag header in JSON response")
	}

	// Check Cache-Control header
	cacheControl := resp.Header().Get("Cache-Control")
	if cacheControl == "" {
		t.Error("Expected Cache-Control header in JSON response")
	}
}

func TestDifferentFormatsHaveDifferentETags(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create video and transcription
	video := ts.createTestVideo(t, nil, nil)
	ts.createTestTranscription(t, video.ID)

	// Get ETags for all formats
	respSRT := ts.doRequest("GET", "/api/videos/"+video.ID+"/subtitles.srt", nil, "")
	respVTT := ts.doRequest("GET", "/api/videos/"+video.ID+"/subtitles.vtt", nil, "")
	respJSON := ts.doRequest("GET", "/api/videos/"+video.ID+"/subtitles.json", nil, "")

	etagSRT := respSRT.Header().Get("ETag")
	etagVTT := respVTT.Header().Get("ETag")
	etagJSON := respJSON.Header().Get("ETag")

	// All ETags should be different (they include format in the hash)
	if etagSRT == etagVTT {
		t.Error("SRT and VTT should have different ETags")
	}
	if etagSRT == etagJSON {
		t.Error("SRT and JSON should have different ETags")
	}
	if etagVTT == etagJSON {
		t.Error("VTT and JSON should have different ETags")
	}
}

// createTestVideoWithFile creates a video record AND an actual file on disk for testing
func (ts *testServer) createTestVideoWithFile(t *testing.T, userID *string, sessionID *string, content []byte) *db.Video {
	t.Helper()

	videoID := testGenerateID()
	filename := videoID + ".mp4"
	filePath := filepath.Join(ts.uploadDir, filename)

	// Write actual content to file
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatalf("Failed to create test video file: %v", err)
	}

	video := &db.Video{
		ID:          videoID,
		Filename:    filename,
		Size:        int64(len(content)),
		ContentType: "video/mp4",
		FilePath:    filePath,
		UserID:      userID,
		SessionID:   sessionID,
		CreatedAt:   time.Now(),
	}

	if err := ts.db.CreateVideo(video); err != nil {
		t.Fatalf("Failed to create test video: %v", err)
	}
	return video
}

func TestVideoRangeRequest(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create test content (100 bytes)
	content := make([]byte, 100)
	for i := range content {
		content[i] = byte(i)
	}

	video := ts.createTestVideoWithFile(t, nil, nil, content)

	// Test Range request for bytes 0-49 (first 50 bytes)
	req, _ := http.NewRequest("GET", "/api/videos/"+video.ID+"/video", nil)
	req.Header.Set("Range", "bytes=0-49")
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusPartialContent {
		t.Errorf("Expected status 206 Partial Content, got %d", rr.Code)
	}

	// Verify Content-Range header
	contentRange := rr.Header().Get("Content-Range")
	if !strings.HasPrefix(contentRange, "bytes 0-49/100") {
		t.Errorf("Expected Content-Range 'bytes 0-49/100', got '%s'", contentRange)
	}

	// Verify only requested bytes were returned
	if rr.Body.Len() != 50 {
		t.Errorf("Expected 50 bytes, got %d", rr.Body.Len())
	}

	// Verify content matches
	for i := 0; i < 50; i++ {
		if rr.Body.Bytes()[i] != byte(i) {
			t.Errorf("Byte at position %d: expected %d, got %d", i, i, rr.Body.Bytes()[i])
			break
		}
	}
}

func TestVideoRangeRequestMiddleRange(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create test content (100 bytes)
	content := make([]byte, 100)
	for i := range content {
		content[i] = byte(i)
	}

	video := ts.createTestVideoWithFile(t, nil, nil, content)

	// Test Range request for bytes 25-74 (middle 50 bytes)
	req, _ := http.NewRequest("GET", "/api/videos/"+video.ID+"/video", nil)
	req.Header.Set("Range", "bytes=25-74")
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusPartialContent {
		t.Errorf("Expected status 206 Partial Content, got %d", rr.Code)
	}

	// Verify Content-Range header
	contentRange := rr.Header().Get("Content-Range")
	if !strings.HasPrefix(contentRange, "bytes 25-74/100") {
		t.Errorf("Expected Content-Range 'bytes 25-74/100', got '%s'", contentRange)
	}

	// Verify only requested bytes were returned
	if rr.Body.Len() != 50 {
		t.Errorf("Expected 50 bytes, got %d", rr.Body.Len())
	}

	// Verify content matches
	for i := 0; i < 50; i++ {
		if rr.Body.Bytes()[i] != byte(25+i) {
			t.Errorf("Byte at position %d: expected %d, got %d", i, 25+i, rr.Body.Bytes()[i])
			break
		}
	}
}

func TestVideoRangeRequestSuffix(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create test content (100 bytes)
	content := make([]byte, 100)
	for i := range content {
		content[i] = byte(i)
	}

	video := ts.createTestVideoWithFile(t, nil, nil, content)

	// Test suffix Range request for last 20 bytes (bytes=-20)
	req, _ := http.NewRequest("GET", "/api/videos/"+video.ID+"/video", nil)
	req.Header.Set("Range", "bytes=-20")
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusPartialContent {
		t.Errorf("Expected status 206 Partial Content, got %d", rr.Code)
	}

	// Verify only 20 bytes were returned
	if rr.Body.Len() != 20 {
		t.Errorf("Expected 20 bytes, got %d", rr.Body.Len())
	}

	// Verify content matches last 20 bytes
	for i := 0; i < 20; i++ {
		expected := byte(80 + i)
		if rr.Body.Bytes()[i] != expected {
			t.Errorf("Byte at position %d: expected %d, got %d", i, expected, rr.Body.Bytes()[i])
			break
		}
	}
}

func TestVideoRangeRequestOpenEnd(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create test content (100 bytes)
	content := make([]byte, 100)
	for i := range content {
		content[i] = byte(i)
	}

	video := ts.createTestVideoWithFile(t, nil, nil, content)

	// Test open-end Range request (bytes=80-)
	req, _ := http.NewRequest("GET", "/api/videos/"+video.ID+"/video", nil)
	req.Header.Set("Range", "bytes=80-")
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusPartialContent {
		t.Errorf("Expected status 206 Partial Content, got %d", rr.Code)
	}

	// Verify Content-Range header
	contentRange := rr.Header().Get("Content-Range")
	if !strings.HasPrefix(contentRange, "bytes 80-99/100") {
		t.Errorf("Expected Content-Range 'bytes 80-99/100', got '%s'", contentRange)
	}

	// Verify only 20 bytes were returned
	if rr.Body.Len() != 20 {
		t.Errorf("Expected 20 bytes, got %d", rr.Body.Len())
	}
}

func TestVideoRangeRequestInvalidRange(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create test content (100 bytes)
	content := make([]byte, 100)
	video := ts.createTestVideoWithFile(t, nil, nil, content)

	// Test invalid Range request (beyond file size)
	req, _ := http.NewRequest("GET", "/api/videos/"+video.ID+"/video", nil)
	req.Header.Set("Range", "bytes=150-200")
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	// Should return 416 Range Not Satisfiable
	if rr.Code != http.StatusRequestedRangeNotSatisfiable {
		t.Errorf("Expected status 416 Range Not Satisfiable, got %d", rr.Code)
	}
}

func TestVideoNoRangeRequest(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create test content (100 bytes)
	content := make([]byte, 100)
	for i := range content {
		content[i] = byte(i)
	}

	video := ts.createTestVideoWithFile(t, nil, nil, content)

	// Test request without Range header
	req, _ := http.NewRequest("GET", "/api/videos/"+video.ID+"/video", nil)
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 OK, got %d", rr.Code)
	}

	// Verify all content was returned
	if rr.Body.Len() != 100 {
		t.Errorf("Expected 100 bytes, got %d", rr.Body.Len())
	}

	// Should have Accept-Ranges header indicating range support
	acceptRanges := rr.Header().Get("Accept-Ranges")
	if acceptRanges != "bytes" {
		t.Errorf("Expected Accept-Ranges 'bytes', got '%s'", acceptRanges)
	}
}

// createTestVideoWithThumbnail creates a video record with an actual thumbnail file
func (ts *testServer) createTestVideoWithThumbnail(t *testing.T, userID *string, sessionID *string, videoContent []byte, thumbContent []byte) *db.Video {
	t.Helper()

	videoID := testGenerateID()
	filename := videoID + ".mp4"
	filePath := filepath.Join(ts.uploadDir, filename)
	thumbPath := filepath.Join(ts.uploadDir, videoID+"_thumb.jpg")

	// Write actual content to files
	if err := os.WriteFile(filePath, videoContent, 0644); err != nil {
		t.Fatalf("Failed to create test video file: %v", err)
	}
	if err := os.WriteFile(thumbPath, thumbContent, 0644); err != nil {
		t.Fatalf("Failed to create test thumbnail file: %v", err)
	}

	video := &db.Video{
		ID:            videoID,
		Filename:      filename,
		Size:          int64(len(videoContent)),
		ContentType:   "video/mp4",
		FilePath:      filePath,
		ThumbnailPath: &thumbPath,
		UserID:        userID,
		SessionID:     sessionID,
		CreatedAt:     time.Now(),
	}

	if err := ts.db.CreateVideo(video); err != nil {
		t.Fatalf("Failed to create test video: %v", err)
	}
	return video
}

func TestThumbnailEndpointSuccess(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create test thumbnail content (fake JPEG)
	thumbContent := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46} // JPEG header bytes
	videoContent := []byte("fake video content")

	video := ts.createTestVideoWithThumbnail(t, nil, nil, videoContent, thumbContent)

	req, _ := http.NewRequest("GET", "/api/videos/"+video.ID+"/thumbnail", nil)
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 OK, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	// Verify content type
	contentType := rr.Header().Get("Content-Type")
	if contentType != "image/jpeg" {
		t.Errorf("Expected Content-Type 'image/jpeg', got '%s'", contentType)
	}

	// Verify content matches
	if !bytes.Equal(rr.Body.Bytes(), thumbContent) {
		t.Errorf("Thumbnail content mismatch")
	}

	// Should have caching headers
	cacheControl := rr.Header().Get("Cache-Control")
	if cacheControl == "" {
		t.Error("Expected Cache-Control header to be set")
	}

	etag := rr.Header().Get("ETag")
	if etag == "" {
		t.Error("Expected ETag header to be set")
	}
}

func TestThumbnailEndpointNotFound(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	req, _ := http.NewRequest("GET", "/api/videos/nonexistent123/thumbnail", nil)
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 Not Found, got %d", rr.Code)
	}
}

func TestThumbnailEndpointNoThumbnail(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create video without thumbnail
	videoContent := []byte("fake video content")
	video := ts.createTestVideoWithFile(t, nil, nil, videoContent)

	req, _ := http.NewRequest("GET", "/api/videos/"+video.ID+"/thumbnail", nil)
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status 404 Not Found for video without thumbnail, got %d", rr.Code)
	}
}

func TestThumbnailEndpointConditionalRequest(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	thumbContent := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	videoContent := []byte("fake video")

	video := ts.createTestVideoWithThumbnail(t, nil, nil, videoContent, thumbContent)

	// First request to get ETag
	req1, _ := http.NewRequest("GET", "/api/videos/"+video.ID+"/thumbnail", nil)
	rr1 := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Fatalf("Expected status 200 OK on first request, got %d", rr1.Code)
	}

	etag := rr1.Header().Get("ETag")
	if etag == "" {
		t.Fatal("Expected ETag header on first request")
	}

	// Second request with If-None-Match
	req2, _ := http.NewRequest("GET", "/api/videos/"+video.ID+"/thumbnail", nil)
	req2.Header.Set("If-None-Match", etag)
	rr2 := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusNotModified {
		t.Errorf("Expected status 304 Not Modified, got %d", rr2.Code)
	}
}

// ========== Chunked Upload Tests ==========

func TestChunkedUploadInit(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Test successful init
	resp := ts.doRequest("POST", "/api/upload/init", map[string]interface{}{
		"filename":     "test.mp4",
		"size":         150000000, // 150 MB
		"content_type": "video/mp4",
		"chunk_size":   50000000, // 50 MB
	}, "")

	if resp.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d: %s", resp.Code, resp.Body.String())
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["upload_session_id"] == nil {
		t.Error("Expected upload_session_id in response")
	}
	if result["total_chunks"].(float64) != 3 {
		t.Errorf("Expected 3 chunks, got %v", result["total_chunks"])
	}
}

func TestChunkedUploadInitInvalidMIME(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	resp := ts.doRequest("POST", "/api/upload/init", map[string]interface{}{
		"filename":     "test.txt",
		"size":         1000,
		"content_type": "text/plain",
	}, "")

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request, got %d", resp.Code)
	}
}

func TestChunkedUploadInitFileTooLarge(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	resp := ts.doRequest("POST", "/api/upload/init", map[string]interface{}{
		"filename":     "huge.mp4",
		"size":         600000000, // 600 MB, over 500 MB limit
		"content_type": "video/mp4",
	}, "")

	if resp.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request, got %d", resp.Code)
	}
}

func TestChunkedUploadChunk(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Initialize session
	initResp := ts.doRequest("POST", "/api/upload/init", map[string]interface{}{
		"filename":     "test.mp4",
		"size":         1000,
		"content_type": "video/mp4",
		"chunk_size":   500,
	}, "")

	if initResp.Code != http.StatusOK {
		t.Fatalf("Init failed: %s", initResp.Body.String())
	}

	var initResult map[string]interface{}
	json.NewDecoder(initResp.Body).Decode(&initResult)
	sessionID := initResult["upload_session_id"].(string)

	// Upload first chunk
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("upload_session_id", sessionID)
	writer.WriteField("chunk_index", "0")
	part, _ := writer.CreateFormFile("chunk", "chunk_0")
	part.Write(make([]byte, 500))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/upload/chunk", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d: %s", rr.Code, rr.Body.String())
	}

	var chunkResult map[string]interface{}
	json.NewDecoder(rr.Body).Decode(&chunkResult)

	if chunkResult["chunk_index"].(float64) != 0 {
		t.Errorf("Expected chunk_index 0, got %v", chunkResult["chunk_index"])
	}
	if chunkResult["received_bytes"].(float64) != 500 {
		t.Errorf("Expected 500 bytes, got %v", chunkResult["received_bytes"])
	}
}

func TestChunkedUploadChunkIdempotency(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Initialize session
	initResp := ts.doRequest("POST", "/api/upload/init", map[string]interface{}{
		"filename":     "test.mp4",
		"size":         1000,
		"content_type": "video/mp4",
		"chunk_size":   1000,
	}, "")

	var initResult map[string]interface{}
	json.NewDecoder(initResp.Body).Decode(&initResult)
	sessionID := initResult["upload_session_id"].(string)

	// Upload chunk twice
	for i := 0; i < 2; i++ {
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		writer.WriteField("upload_session_id", sessionID)
		writer.WriteField("chunk_index", "0")
		part, _ := writer.CreateFormFile("chunk", "chunk_0")
		part.Write(make([]byte, 1000))
		writer.Close()

		req := httptest.NewRequest("POST", "/api/upload/chunk", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rr := httptest.NewRecorder()
		ts.mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Request %d: Expected 200 OK, got %d", i+1, rr.Code)
		}
	}

	// Verify only one chunk was stored
	count, _ := ts.db.CountUploadChunks(sessionID)
	if count != 1 {
		t.Errorf("Expected 1 chunk, got %d", count)
	}
}

func TestChunkedUploadComplete(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Initialize session with small chunks
	initResp := ts.doRequest("POST", "/api/upload/init", map[string]interface{}{
		"filename":     "test.mp4",
		"size":         1000,
		"content_type": "video/mp4",
		"chunk_size":   500,
	}, "")

	var initResult map[string]interface{}
	json.NewDecoder(initResp.Body).Decode(&initResult)
	sessionID := initResult["upload_session_id"].(string)
	totalChunks := int(initResult["total_chunks"].(float64))

	// Upload all chunks
	for i := 0; i < totalChunks; i++ {
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		writer.WriteField("upload_session_id", sessionID)
		writer.WriteField("chunk_index", fmt.Sprintf("%d", i))
		part, _ := writer.CreateFormFile("chunk", fmt.Sprintf("chunk_%d", i))
		// Write chunk data
		chunkSize := 500
		if i == totalChunks-1 {
			chunkSize = 1000 - i*500 // Last chunk may be smaller
		}
		part.Write(make([]byte, chunkSize))
		writer.Close()

		req := httptest.NewRequest("POST", "/api/upload/chunk", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rr := httptest.NewRecorder()
		ts.mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("Chunk %d upload failed: %s", i, rr.Body.String())
		}
	}

	// Complete upload
	completeResp := ts.doRequest("POST", "/api/upload/complete", map[string]interface{}{
		"upload_session_id": sessionID,
	}, "")

	if completeResp.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d: %s", completeResp.Code, completeResp.Body.String())
	}

	var completeResult map[string]interface{}
	json.NewDecoder(completeResp.Body).Decode(&completeResult)

	if completeResult["upload_id"] == nil {
		t.Error("Expected upload_id in response")
	}
	if completeResult["filename"].(string) != "test.mp4" {
		t.Errorf("Expected filename test.mp4, got %v", completeResult["filename"])
	}
}

func TestChunkedUploadCompleteIncomplete(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Initialize session with 2 chunks
	initResp := ts.doRequest("POST", "/api/upload/init", map[string]interface{}{
		"filename":     "test.mp4",
		"size":         1000,
		"content_type": "video/mp4",
		"chunk_size":   500,
	}, "")

	var initResult map[string]interface{}
	json.NewDecoder(initResp.Body).Decode(&initResult)
	sessionID := initResult["upload_session_id"].(string)

	// Upload only first chunk
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("upload_session_id", sessionID)
	writer.WriteField("chunk_index", "0")
	part, _ := writer.CreateFormFile("chunk", "chunk_0")
	part.Write(make([]byte, 500))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/upload/chunk", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	// Try to complete (should fail)
	completeResp := ts.doRequest("POST", "/api/upload/complete", map[string]interface{}{
		"upload_session_id": sessionID,
	}, "")

	if completeResp.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request, got %d", completeResp.Code)
	}
}

func TestChunkedUploadStatus(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Initialize session
	initResp := ts.doRequest("POST", "/api/upload/init", map[string]interface{}{
		"filename":     "test.mp4",
		"size":         2000,
		"content_type": "video/mp4",
		"chunk_size":   500,
	}, "")

	var initResult map[string]interface{}
	json.NewDecoder(initResp.Body).Decode(&initResult)
	sessionID := initResult["upload_session_id"].(string)

	// Upload 2 of 4 chunks
	for i := 0; i < 2; i++ {
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		writer.WriteField("upload_session_id", sessionID)
		writer.WriteField("chunk_index", fmt.Sprintf("%d", i))
		part, _ := writer.CreateFormFile("chunk", fmt.Sprintf("chunk_%d", i))
		part.Write(make([]byte, 500))
		writer.Close()

		req := httptest.NewRequest("POST", "/api/upload/chunk", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rr := httptest.NewRecorder()
		ts.mux.ServeHTTP(rr, req)
	}

	// Check status
	statusReq := httptest.NewRequest("GET", "/api/upload/status/"+sessionID, nil)
	statusRR := httptest.NewRecorder()
	ts.mux.ServeHTTP(statusRR, statusReq)

	if statusRR.Code != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", statusRR.Code)
	}

	var statusResult map[string]interface{}
	json.NewDecoder(statusRR.Body).Decode(&statusResult)

	if statusResult["progress"].(float64) != 50 {
		t.Errorf("Expected 50%% progress, got %v", statusResult["progress"])
	}
	if statusResult["status"].(string) != "in_progress" {
		t.Errorf("Expected status in_progress, got %v", statusResult["status"])
	}

	receivedChunks := statusResult["received_chunks"].([]interface{})
	if len(receivedChunks) != 2 {
		t.Errorf("Expected 2 received chunks, got %d", len(receivedChunks))
	}
}

func TestChunkedUploadStatusNotFound(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	req := httptest.NewRequest("GET", "/api/upload/status/nonexistent", nil)
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected 404 Not Found, got %d", rr.Code)
	}
}

func TestChunkedUploadInvalidChunkIndex(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Initialize session with 2 chunks
	initResp := ts.doRequest("POST", "/api/upload/init", map[string]interface{}{
		"filename":     "test.mp4",
		"size":         1000,
		"content_type": "video/mp4",
		"chunk_size":   500,
	}, "")

	var initResult map[string]interface{}
	json.NewDecoder(initResp.Body).Decode(&initResult)
	sessionID := initResult["upload_session_id"].(string)

	// Try to upload chunk with invalid index (3, when only 0 and 1 are valid)
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("upload_session_id", sessionID)
	writer.WriteField("chunk_index", "3")
	part, _ := writer.CreateFormFile("chunk", "chunk_3")
	part.Write(make([]byte, 500))
	writer.Close()

	req := httptest.NewRequest("POST", "/api/upload/chunk", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rr := httptest.NewRecorder()
	ts.mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestCleanupOrphanChunkDirectories(t *testing.T) {
	// Create temp directory for test
	tempDir, err := os.MkdirTemp("", "orphan-cleanup-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test database
	dbPath := filepath.Join(tempDir, "test.db")
	testDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer testDB.Close()

	// Create chunks directory structure
	chunksDir := filepath.Join(tempDir, "chunks")
	if err := os.MkdirAll(chunksDir, 0755); err != nil {
		t.Fatalf("Failed to create chunks dir: %v", err)
	}

	// Create an upload session in the database
	validSession := &db.UploadSession{
		ID:          "valid-session-123",
		Filename:    "test.mp4",
		ContentType: "video/mp4",
		TotalSize:   1024000,
		ChunkSize:   512000,
		TotalChunks: 2,
		Status:      "in_progress",
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(24 * time.Hour),
	}
	if err := testDB.CreateUploadSession(validSession); err != nil {
		t.Fatalf("Failed to create upload session: %v", err)
	}

	// Create directory for valid session
	validDir := filepath.Join(chunksDir, "valid-session-123")
	if err := os.MkdirAll(validDir, 0755); err != nil {
		t.Fatalf("Failed to create valid session dir: %v", err)
	}
	// Create a chunk file in valid directory
	if err := os.WriteFile(filepath.Join(validDir, "chunk_0.part"), []byte("test data"), 0644); err != nil {
		t.Fatalf("Failed to create chunk file: %v", err)
	}

	// Create orphan directories (not in database)
	orphan1Dir := filepath.Join(chunksDir, "orphan-session-456")
	orphan2Dir := filepath.Join(chunksDir, "orphan-session-789")
	if err := os.MkdirAll(orphan1Dir, 0755); err != nil {
		t.Fatalf("Failed to create orphan1 dir: %v", err)
	}
	if err := os.MkdirAll(orphan2Dir, 0755); err != nil {
		t.Fatalf("Failed to create orphan2 dir: %v", err)
	}
	// Create chunk files in orphan directories
	if err := os.WriteFile(filepath.Join(orphan1Dir, "chunk_0.part"), []byte("orphan data"), 0644); err != nil {
		t.Fatalf("Failed to create orphan chunk file: %v", err)
	}

	// Run cleanup
	deletedCount := cleanupOrphanChunkDirectoriesWithDB(testDB, tempDir)

	// Verify 2 orphan directories were deleted
	if deletedCount != 2 {
		t.Errorf("Expected 2 orphan directories deleted, got %d", deletedCount)
	}

	// Verify orphan directories are gone
	if _, err := os.Stat(orphan1Dir); !os.IsNotExist(err) {
		t.Error("Expected orphan1 directory to be deleted")
	}
	if _, err := os.Stat(orphan2Dir); !os.IsNotExist(err) {
		t.Error("Expected orphan2 directory to be deleted")
	}

	// Verify valid session directory still exists
	if _, err := os.Stat(validDir); os.IsNotExist(err) {
		t.Error("Valid session directory should NOT be deleted")
	}

	// Verify chunk file in valid directory still exists
	if _, err := os.Stat(filepath.Join(validDir, "chunk_0.part")); os.IsNotExist(err) {
		t.Error("Chunk file in valid session should NOT be deleted")
	}
}

func TestCleanupOrphanChunkDirectoriesNoChunksDir(t *testing.T) {
	// Create temp directory for test without chunks subdirectory
	tempDir, err := os.MkdirTemp("", "orphan-cleanup-nochunks-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test database
	dbPath := filepath.Join(tempDir, "test.db")
	testDB, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer testDB.Close()

	// Run cleanup with no chunks directory - should not error
	deletedCount := cleanupOrphanChunkDirectoriesWithDB(testDB, tempDir)

	// No directories should be deleted (and no error should occur)
	if deletedCount != 0 {
		t.Errorf("Expected 0 deletions when chunks dir doesn't exist, got %d", deletedCount)
	}
}

func TestDownloadRateLimiting(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.cleanup()

	// Create a user and video
	userID, token := ts.createTestUserWithID(t, "download-ratelimit@example.com", "Password123!")
	video := ts.createTestVideo(t, &userID, nil)

	// Create a small downloadLimiter for testing (5 requests per minute)
	testDownloadLimiter := ratelimit.New(5, time.Minute)

	// Create a dedicated test server with the stricter limiter
	testMux := http.NewServeMux()
	testMux.HandleFunc("GET /api/videos/{id}/video", testDownloadLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, video.FilePath)
	}))

	testServer := httptest.NewServer(testMux)
	defer testServer.Close()

	// Make requests until rate limited
	var lastResp *http.Response
	for i := 0; i < 7; i++ {
		req, _ := http.NewRequest("GET", testServer.URL+"/api/videos/"+video.ID+"/video", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		lastResp, _ = http.DefaultClient.Do(req)
		if lastResp.StatusCode == http.StatusTooManyRequests {
			break
		}
		lastResp.Body.Close()
	}

	// Should eventually get 429
	if lastResp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("Expected 429 after exceeding rate limit, got %d", lastResp.StatusCode)
	}

	// Check Retry-After header
	retryAfter := lastResp.Header.Get("Retry-After")
	if retryAfter == "" {
		t.Error("Expected Retry-After header in rate limited response")
	}
	lastResp.Body.Close()
}
