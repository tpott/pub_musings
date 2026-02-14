package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tpott/subtitler/backend/align"
	"github.com/tpott/subtitler/backend/audio"
	"github.com/tpott/subtitler/backend/auth"
	"github.com/tpott/subtitler/backend/captcha"
	"github.com/tpott/subtitler/backend/crypto"
	"github.com/tpott/subtitler/backend/csrf"
	"github.com/tpott/subtitler/backend/db"
	"github.com/tpott/subtitler/backend/email"
	"github.com/tpott/subtitler/backend/httputil"
	"github.com/tpott/subtitler/backend/language"
	"github.com/tpott/subtitler/backend/metrics"
	"github.com/tpott/subtitler/backend/pathvalidator"
	"github.com/tpott/subtitler/backend/ratelimit"
	"github.com/tpott/subtitler/backend/script"
	"github.com/tpott/subtitler/backend/security"
	"github.com/tpott/subtitler/backend/totp"
	"github.com/tpott/subtitler/backend/validation"
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
	metricsLimiter       *ratelimit.Limiter
	metricsAPIKey        string
	emailService         *email.MockService
	pathValidator        *pathvalidator.Validator
	captchaVerifier      *captcha.MockVerifier
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

	// Create path validator for secure file serving
	pv, err := pathvalidator.New(uploadDir)
	if err != nil {
		testDB.Close()
		os.RemoveAll(tempDir)
		t.Fatalf("Failed to create path validator: %v", err)
	}

	ts := &testServer{
		mux:                  http.NewServeMux(),
		db:                   testDB,
		encryptor:            enc,
		uploadDir:            uploadDir,
		authLimiter:          ratelimit.New(5, time.Minute),
		passwordResetLimiter: ratelimit.New(3, 15*time.Minute),
		downloadLimiter:      ratelimit.New(30, time.Minute),
		metricsLimiter:       ratelimit.New(10, time.Minute),
		metricsAPIKey:        os.Getenv("METRICS_API_KEY"),
		emailService:         email.NewMockService(),
		pathValidator:        pv,
		captchaVerifier:      captcha.NewMockVerifier(false), // CAPTCHA disabled by default in tests
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

		// Determine overall status and HTTP status code
		statusCode := http.StatusOK
		if !status.DBConnected || !status.WhisperAvailable || !status.DiskSpaceOK {
			status.Status = "degraded"
			statusCode = http.StatusServiceUnavailable
		}

		// Check if user is authenticated
		token := auth.GetTokenFromRequest(r)
		user, _, _ := auth.ValidateSession(ts.db, token)

		w.WriteHeader(statusCode)
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

	// Feedback endpoint
	ts.mux.HandleFunc("POST /api/feedback", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var req struct {
			Text        string  `json:"text"`
			Type        string  `json:"type"`
			Rating      *int    `json:"rating"`
			PageURL     string  `json:"page_url"`
			VideoID     *string `json:"video_id"`
			SessionID   *string `json:"session_id"`
			BrowserInfo string  `json:"browser_info"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
			return
		}

		// Validate required fields
		if req.Text == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Feedback text is required"})
			return
		}

		// Validate text length (max 10KB)
		if len(req.Text) > 10240 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Feedback text is too long (max 10KB)"})
			return
		}

		// Validate type
		validTypes := map[string]bool{
			db.FeedbackTypeGeneral: true,
			db.FeedbackTypeBug:     true,
			db.FeedbackTypeFeature: true,
		}
		if req.Type == "" {
			req.Type = db.FeedbackTypeGeneral
		} else if !validTypes[req.Type] {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid feedback type"})
			return
		}

		// Validate rating if provided
		if req.Rating != nil && (*req.Rating < 1 || *req.Rating > 5) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Rating must be between 1 and 5"})
			return
		}

		// Get user context if authenticated
		var userID *string
		token := auth.GetTokenFromRequest(r)
		if token != "" {
			user, _, _ := auth.ValidateSession(ts.db, token)
			if user != nil {
				userID = &user.ID
			}
		}

		// Generate feedback ID
		feedbackID, err := auth.GenerateID()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save feedback"})
			return
		}

		// Create feedback record
		feedback := &db.Feedback{
			ID:          feedbackID,
			UserID:      userID,
			SessionID:   req.SessionID,
			VideoID:     req.VideoID,
			PageURL:     req.PageURL,
			Text:        req.Text,
			Rating:      req.Rating,
			Type:        req.Type,
			BrowserInfo: req.BrowserInfo,
			CreatedAt:   time.Now(),
			Status:      db.FeedbackStatusNew,
		}

		if err := ts.db.CreateFeedback(feedback); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save feedback"})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"id":     feedbackID,
		})
	})

	// Admin: List feedback (admin only)
	ts.mux.HandleFunc("GET /api/admin/feedback", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Check authentication
		token := auth.GetTokenFromRequest(r)
		if token == "" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Authentication required"})
			return
		}

		user, _, err := auth.ValidateSession(ts.db, token)
		if err != nil || user == nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid session"})
			return
		}

		// Check admin role
		if !user.IsAdmin() {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{"error": "Admin access required"})
			return
		}

		// Parse query parameters
		status := r.URL.Query().Get("status")
		feedbackType := r.URL.Query().Get("type")
		limitStr := r.URL.Query().Get("limit")
		offsetStr := r.URL.Query().Get("offset")
		after := r.URL.Query().Get("after")

		limit := 50
		if limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
				limit = l
			}
		}

		offset := 0
		if offsetStr != "" {
			if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
				offset = o
			}
		}
		if offset > maxPaginationOffset {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Offset exceeds maximum allowed value"})
			return
		}

		// List feedback
		feedbackList, total, err := ts.db.ListFeedback(status, feedbackType, limit, offset, after)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to list feedback"})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"feedback": feedbackList,
			"total":    total,
			"limit":    limit,
			"offset":   offset,
		})
	})

	// Admin: Get feedback by ID (admin only)
	ts.mux.HandleFunc("GET /api/admin/feedback/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Check authentication
		token := auth.GetTokenFromRequest(r)
		if token == "" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Authentication required"})
			return
		}

		user, _, err := auth.ValidateSession(ts.db, token)
		if err != nil || user == nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid session"})
			return
		}

		// Check admin role
		if !user.IsAdmin() {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{"error": "Admin access required"})
			return
		}

		// Validate ID format
		feedbackID := r.PathValue("id")
		if err := validation.ValidateHexID(feedbackID); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid feedback ID format"})
			return
		}

		// Get feedback
		feedback, err := ts.db.GetFeedback(feedbackID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get feedback"})
			return
		}

		if feedback == nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "Feedback not found"})
			return
		}

		json.NewEncoder(w).Encode(feedback)
	})

	// Admin: Update feedback status (admin only)
	ts.mux.HandleFunc("PATCH /api/admin/feedback/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Check authentication
		token := auth.GetTokenFromRequest(r)
		if token == "" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Authentication required"})
			return
		}

		user, _, err := auth.ValidateSession(ts.db, token)
		if err != nil || user == nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid session"})
			return
		}

		// Check admin role
		if !user.IsAdmin() {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{"error": "Admin access required"})
			return
		}

		// Validate ID format
		feedbackID := r.PathValue("id")
		if err := validation.ValidateHexID(feedbackID); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid feedback ID format"})
			return
		}

		// Parse request body
		var req struct {
			Status string `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
			return
		}

		// Validate status
		validStatuses := map[string]bool{
			db.FeedbackStatusNew:      true,
			db.FeedbackStatusRead:     true,
			db.FeedbackStatusResolved: true,
		}
		if !validStatuses[req.Status] {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid status. Must be: new, read, or resolved"})
			return
		}

		// Update status
		if err := ts.db.UpdateFeedbackStatus(feedbackID, req.Status); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update feedback status"})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ok",
			"id":      feedbackID,
			"updated": req.Status,
		})
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

	// Auth: Request magic link - sends a login link via email
	ts.mux.HandleFunc("POST /api/auth/magic-link", ts.passwordResetLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
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
				"message": "If an account exists with that email, a login link has been sent.",
			})
		}()

		user, err := ts.db.GetUserByEmail(req.Email)
		if err != nil || user == nil {
			return
		}

		// Check if email is verified - magic links only work for verified users
		if !user.EmailVerified {
			return
		}

		// Generate magic link token
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			return
		}
		token := hex.EncodeToString(tokenBytes)
		tokenHash := email.HashToken(token)

		expiresAt := time.Now().Add(15 * time.Minute)
		_, err = ts.db.CreateMagicLinkToken(user.ID, tokenHash, expiresAt)
		if err != nil {
			return
		}

		ts.emailService.SendMagicLink(r.Context(), user.Email, token)
	}))

	// Auth: Verify magic link - logs user in with magic link token
	ts.mux.HandleFunc("GET /api/auth/magic-link/verify", ts.authLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		token := r.URL.Query().Get("token")
		if token == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Token is required"})
			return
		}

		tokenHash := email.HashToken(token)
		magicToken, err := ts.db.GetMagicLinkToken(tokenHash)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to verify token"})
			return
		}

		if magicToken == nil || magicToken.Used || time.Now().After(magicToken.ExpiresAt) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid or expired token"})
			return
		}

		// Mark token as used
		used, err := ts.db.UseMagicLinkToken(tokenHash)
		if err != nil || !used {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Token already used"})
			return
		}

		// Get user and create session
		user, err := ts.db.GetUserByID(magicToken.UserID)
		if err != nil || user == nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "User not found"})
			return
		}

		session, err := auth.CreateSession(ts.db, user.ID, "", "")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create session"})
			return
		}

		auth.SetSessionCookie(w, session.Token, session.ExpiresAt)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Login successful",
			"token":   session.Token,
			"user": map[string]interface{}{
				"id":    user.ID,
				"email": user.Email,
			},
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
		if offset > maxPaginationOffset {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Offset exceeds maximum allowed value"})
			return
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

		// Batch fetch transcription statuses (single query instead of N+1)
		videoIDs := make([]string, len(result.Videos))
		for i, v := range result.Videos {
			videoIDs[i] = v.ID
		}
		statusMap, err := ts.db.GetTranscriptionStatuses(videoIDs)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get transcription statuses"})
			return
		}

		videos := make([]VideoWithStatus, len(result.Videos))
		for i, v := range result.Videos {
			videos[i] = VideoWithStatus{Video: v, TranscriptionStatus: "none"}
			if status, ok := statusMap[v.ID]; ok {
				videos[i].TranscriptionStatus = status
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

		// Check ownership
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
			json.NewEncoder(w).Encode(map[string]string{"error": "You do not have permission to access this video"})
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

		// Validate that the file path is within the allowed upload directory
		if err := ts.pathValidator.ValidateAbsolutePath(video.FilePath); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{"error": "Access denied"})
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

		// Validate that the thumbnail path is within the allowed upload directory
		if err := ts.pathValidator.ValidateAbsolutePath(thumbPath); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{"error": "Access denied"})
			return
		}

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

	// Get language hints for a video
	ts.mux.HandleFunc("GET /api/videos/{id}/language-hints", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		uploadID, valid := validatePathID(w, r.PathValue("id"), "Upload ID")
		if !valid {
			return
		}

		video, err := ts.db.GetVideo(uploadID)
		if err != nil {
			httputil.RespondError(w, http.StatusInternalServerError, "Database error")
			return
		}
		if video == nil {
			httputil.RespondError(w, http.StatusNotFound, "Video not found")
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
			httputil.RespondError(w, http.StatusForbidden, "You do not have permission to access this video")
			return
		}

		// Run language detection (no decryption needed in tests, just use filename)
		result := language.Detect("", video.Filename)
		json.NewEncoder(w).Encode(result)
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

		// Check ownership
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
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{"error": "You do not have permission to access this video"})
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
		w.Header().Set("Content-Disposition", httputil.ContentDisposition(uploadID+".srt"))
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

		// Check ownership
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
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{"error": "You do not have permission to access this video"})
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
		w.Header().Set("Content-Disposition", httputil.ContentDisposition(uploadID+".vtt"))
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

		// Check ownership
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
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{"error": "You do not have permission to access this video"})
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
		w.Header().Set("Content-Disposition", httputil.ContentDisposition(uploadID+".json"))
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

		// Check ownership
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
			json.NewEncoder(w).Encode(map[string]string{"error": "You do not have permission to access this video"})
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
			// Validate segment text length
			if err := validation.ValidateSegmentText(seg.Text); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{
					"error": fmt.Sprintf("Segment %d: %v", i, err),
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

		// Check ownership
		video, err := ts.db.GetVideo(uploadID)
		if err != nil {
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to get video")
			return
		}
		if video == nil {
			httputil.RespondError(w, http.StatusNotFound, "Video not found")
			return
		}
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
			httputil.RespondError(w, http.StatusForbidden, "You do not have permission to access this video")
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

	// Start burn job
	ts.mux.HandleFunc("POST /api/videos/{id}/burn", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		uploadID := r.PathValue("id")
		if uploadID == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Upload ID required"})
			return
		}

		// Check ownership
		video, err := ts.db.GetVideo(uploadID)
		if err != nil {
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to get video")
			return
		}
		if video == nil {
			httputil.RespondError(w, http.StatusNotFound, "Video not found")
			return
		}
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
			httputil.RespondError(w, http.StatusForbidden, "You do not have permission to access this video")
			return
		}

		// Validate burn mode
		burnModeParam := r.URL.Query().Get("mode")
		_, err = validation.ValidateBurnMode(burnModeParam)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		// Check transcription exists and is complete
		transcription, err := ts.db.GetTranscription(uploadID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get transcription"})
			return
		}
		if transcription == nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "No transcription found - please transcribe the video first"})
			return
		}
		if transcription.Status != "complete" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":  "Cannot burn subtitles - transcription not complete",
				"status": transcription.Status,
			})
			return
		}

		// Check if already processing
		existingJob, _ := ts.db.GetBurnJob(uploadID)
		if existingJob != nil {
			if existingJob.Status == "processing" {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status":   "processing",
					"message":  existingJob.Message,
					"progress": existingJob.Progress,
				})
				return
			}
			if existingJob.Status == "complete" {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status":   "complete",
					"message":  existingJob.Message,
					"progress": 100,
				})
				return
			}
		}

		// Create burn job (test server doesn't launch goroutine)
		burnJob := &db.BurnJob{
			ID:        testGenerateID(),
			VideoID:   uploadID,
			Status:    "processing",
			Message:   "Starting subtitle burn...",
			Progress:  0,
			CreatedAt: time.Now(),
		}
		if err := ts.db.CreateBurnJob(burnJob); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create burn job"})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "processing",
			"message":  "Starting subtitle burn...",
			"progress": 0,
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

		// Validate file is not empty/zero-size
		if header.Size == 0 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "File is empty. Please upload a valid video file.",
			})
			return
		}

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
		if req.Filename == "" || req.ContentType == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Missing required fields: filename, content_type"})
			return
		}

		// Validate file size is positive (reject zero-size files)
		if req.Size <= 0 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "File size must be greater than zero. Empty files are not allowed."})
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
		// Note: production handler enforces minChunkSize (1MB).
		// Mock allows small chunks for test convenience.

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

		// Early validation: reject negative index
		if chunkIndex < 0 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid chunk_index"})
			return
		}

		// Early upper bound validation - prevents integer overflow issues
		const maxChunkIndex = 100000
		if chunkIndex >= maxChunkIndex {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Chunk index exceeds maximum allowed value"})
			return
		}

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
		if _, err := ts.db.CreateUploadChunk(chunk); err != nil {
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

	// Prometheus metrics endpoint
	// Protected by API key or admin authentication
	ts.mux.HandleFunc("GET /metrics", ts.metricsLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		// Check API key first
		apiKey := r.Header.Get("X-Metrics-API-Key")
		if apiKey == "" {
			apiKey = r.URL.Query().Get("api_key")
		}

		if ts.metricsAPIKey != "" && apiKey == ts.metricsAPIKey {
			// Valid API key, serve metrics
			metrics.Handler().ServeHTTP(w, r)
			return
		}

		// Fall back to checking for admin user
		token := auth.GetTokenFromRequest(r)
		if token == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Authentication required",
			})
			return
		}

		user, _, err := auth.ValidateSession(ts.db, token)
		if err != nil || user == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid session",
			})
			return
		}

		// Check if user has admin role
		if !user.IsAdmin() {
			security.AccessDeniedNotAdmin(r.Context(), ratelimit.GetClientIP(r), user.ID, "/metrics")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Admin access required",
			})
			return
		}

		metrics.Handler().ServeHTTP(w, r)
	}))

	// CAPTCHA config endpoint (returns site key if CAPTCHA is enabled)
	ts.mux.HandleFunc("GET /api/captcha/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled":  ts.captchaVerifier.IsEnabled(),
			"site_key": ts.captchaVerifier.SiteKey(),
		})
	})

	// Auth: Get CSRF token for current session
	ts.mux.HandleFunc("GET /api/auth/csrf", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		sessionToken := auth.GetTokenFromRequest(r)
		if sessionToken == "" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Not authenticated",
			})
			return
		}

		// Validate the session exists
		user, _, err := auth.ValidateSession(ts.db, sessionToken)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to validate session",
			})
			return
		}

		if user == nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Not authenticated",
			})
			return
		}

		// Generate CSRF token from session token
		csrfToken := csrf.GenerateToken(sessionToken)

		json.NewEncoder(w).Encode(map[string]string{
			"csrf_token": csrfToken,
		})
	})

	// Align transcript with user-provided text
	ts.mux.HandleFunc("POST /api/transcribe/{id}/align", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		uploadID := r.PathValue("id")
		if uploadID == "" || len(uploadID) != 32 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid upload ID format"})
			return
		}

		// Check ownership
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
			json.NewEncoder(w).Encode(map[string]string{"error": "You do not have permission to access this video"})
			return
		}

		// Check if transcription exists and is complete
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
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":  "Cannot align - transcription not complete",
				"status": transcription.Status,
			})
			return
		}

		// Parse request body
		var req struct {
			Text            string `json:"text"`
			Mode            string `json:"mode"`
			ConvertToScript string `json:"convert_to_script"`
			Language        string `json:"language"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
			return
		}

		// Validate align text length
		if err := validation.ValidateAlignText(req.Text); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		// Validate align mode
		if _, err := validation.ValidateAlignMode(req.Mode); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		// Script conversion validation if requested
		scriptConverted := false
		var targetScript script.Script
		if req.ConvertToScript != "" {
			targetScript = script.Script(req.ConvertToScript)
			if !script.IsScriptSupported(targetScript) {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":             "Unsupported target script",
					"supported_scripts": script.SupportedScripts(),
				})
				return
			}
			if !script.IsLanguageSupported(req.Language) {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":               "Language required for script conversion",
					"supported_languages": script.SupportedLanguages(),
				})
				return
			}
		}

		// Get existing segments
		existingSegments, err := transcription.GetSegments()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get existing segments"})
			return
		}

		// Convert db.Segment to align.Segment
		alignSegments := make([]align.Segment, len(existingSegments))
		for i, s := range existingSegments {
			alignSegments[i] = align.Segment{
				ID:    s.ID,
				Start: s.Start,
				End:   s.End,
				Text:  s.Text,
			}
		}

		// Perform alignment
		var result align.AlignmentResult
		if req.Mode == "lyrics" {
			result = align.AlignLyrics(req.Text, alignSegments)
		} else {
			result = align.AlignTranscript(req.Text, alignSegments)
		}

		// Convert back to db.Segment
		newSegments := make([]db.Segment, len(result.Segments))
		for i, s := range result.Segments {
			newSegments[i] = db.Segment{
				ID:    s.ID,
				Start: s.Start,
				End:   s.End,
				Text:  s.Text,
			}
		}

		// Apply script conversion if requested
		var failedConversionIndices []int
		if req.ConvertToScript != "" {
			converter := script.NewConverter()
			for i := range newSegments {
				converted, err := converter.Convert(newSegments[i].Text, req.Language, targetScript)
				if err == nil {
					newSegments[i].Text = converted
				} else {
					failedConversionIndices = append(failedConversionIndices, i)
				}
			}
			scriptConverted = true
		}

		// Update segments in database
		if err := ts.db.UpdateSegments(uploadID, newSegments); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save aligned segments"})
			return
		}

		mode := "standard"
		if req.Mode == "lyrics" {
			mode = "lyrics"
		}

		response := map[string]interface{}{
			"status":   "success",
			"segments": len(newSegments),
			"stats":    result.Stats,
			"mode":     mode,
		}
		if scriptConverted {
			response["script_converted"] = true
			response["target_script"] = string(targetScript)
			if len(failedConversionIndices) > 0 {
				response["conversion_failed_indices"] = failedConversionIndices
			}
		}
		json.NewEncoder(w).Encode(response)
	})

	// Extract embedded subtitle track from video
	ts.mux.HandleFunc("GET /api/videos/{id}/embedded-subtitles/{track}", ts.downloadLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		videoID, valid := validatePathID(w, r.PathValue("id"), "Video ID")
		if !valid {
			return
		}

		// Parse track index
		trackStr := r.PathValue("track")
		trackIndex, err := strconv.Atoi(trackStr)
		if err != nil || trackIndex < 0 {
			httputil.RespondError(w, http.StatusBadRequest, "Invalid track index")
			return
		}

		// Get the video
		video, err := ts.db.GetVideo(videoID)
		if err != nil {
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to get video")
			return
		}
		if video == nil {
			httputil.RespondError(w, http.StatusNotFound, "Video not found")
			return
		}

		// Verify the track exists in the video's embedded subtitles
		if video.EmbeddedSubtitlesJSON == nil {
			httputil.RespondError(w, http.StatusNotFound, "This video has no embedded subtitles")
			return
		}

		var tracks []audio.SubtitleTrack
		if err := json.Unmarshal([]byte(*video.EmbeddedSubtitlesJSON), &tracks); err != nil {
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to parse subtitle information")
			return
		}

		// Find the track with the given index
		var foundTrack *audio.SubtitleTrack
		for _, t := range tracks {
			if t.Index == trackIndex {
				foundTrack = &t
				break
			}
		}
		if foundTrack == nil {
			httputil.RespondError(w, http.StatusNotFound, "Subtitle track not found")
			return
		}

		// Only text-based subtitles can be extracted
		if !foundTrack.TextBased {
			httputil.RespondError(w, http.StatusBadRequest, "Cannot extract image-based subtitle track (use OCR for Blu-ray/DVD subtitles)")
			return
		}

		// Check access (user owns video or has matching session_id)
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
			httputil.RespondError(w, http.StatusForbidden, "You do not have permission to access this video")
			return
		}

		// Get format from query parameter (default to srt)
		format := r.URL.Query().Get("format")
		if format == "" {
			format = "srt"
		}
		if format != "srt" && format != "vtt" {
			httputil.RespondError(w, http.StatusBadRequest, "Format must be 'srt' or 'vtt'")
			return
		}

		// In tests, return mock subtitle content instead of calling ffmpeg
		content := "1\n00:00:00,000 --> 00:00:02,500\nHello world.\n\n2\n00:00:03,000 --> 00:00:05,500\nThis is a test.\n"
		if format == "vtt" {
			content = "WEBVTT\n\n00:00:00.000 --> 00:00:02.500\nHello world.\n\n00:00:03.000 --> 00:00:05.500\nThis is a test.\n"
		}

		contentType := "text/plain; charset=utf-8"
		ext := ".srt"
		if format == "vtt" {
			contentType = "text/vtt; charset=utf-8"
			ext = ".vtt"
		}

		filename := strings.TrimSuffix(video.Filename, filepath.Ext(video.Filename))
		if foundTrack.Language != "" {
			filename += "_" + foundTrack.Language
		}
		filename += "_embedded" + ext

		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", httputil.ContentDisposition(filename))
		w.WriteHeader(http.StatusOK)
		httputil.WriteContent(w, []byte(content), "embedded subtitles download")
	}))

	// Gap transcription endpoint
	ts.mux.HandleFunc("POST /api/transcribe/{id}/gap", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		uploadID := r.PathValue("id")
		if uploadID == "" || len(uploadID) != 32 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid upload ID format"})
			return
		}

		// Parse request body
		var req struct {
			Start    float64 `json:"start"`
			End      float64 `json:"end"`
			Language string  `json:"language"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
			return
		}

		// Validate time range
		if req.Start < 0 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Start time must be >= 0"})
			return
		}
		if req.End <= req.Start {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "End time must be greater than start time"})
			return
		}
		if req.End-req.Start > 300 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Gap duration cannot exceed 5 minutes"})
			return
		}

		lang := req.Language
		if lang == "" {
			lang = "auto"
		}
		if err := validation.ValidateLanguageCode(lang); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		// Check ownership
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
			json.NewEncoder(w).Encode(map[string]string{"error": "You do not have permission to access this video"})
			return
		}

		// In tests, return mock gap transcription result
		// Adjust mock times to be absolute (add start offset)
		mockSegments := []WhisperSegment{
			{ID: 0, Start: req.Start + 0.1, End: req.End - 0.1, Text: "Transcribed gap text"},
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"text":     "Transcribed gap text",
			"segments": mockSegments,
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

// createTestAdminUser creates an admin user, verifies their email, logs them in, and returns both the user ID and auth token
func (ts *testServer) createTestAdminUser(t *testing.T, email, password string) (userID string, token string) {
	t.Helper()

	// First create a regular user
	userID, token = ts.createTestUserWithID(t, email, password)

	// Then promote them to admin
	if err := ts.db.UpdateUserRole(userID, db.RoleAdmin); err != nil {
		t.Fatalf("Failed to promote user to admin: %v", err)
	}

	return userID, token
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
