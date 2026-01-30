package main

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/tpott/subtitler/backend/auth"
	"github.com/tpott/subtitler/backend/csrf"
	"github.com/tpott/subtitler/backend/db"
	"github.com/tpott/subtitler/backend/email"
	"github.com/tpott/subtitler/backend/httputil"
	"github.com/tpott/subtitler/backend/logging"
	"github.com/tpott/subtitler/backend/ratelimit"
	"github.com/tpott/subtitler/backend/security"
	"github.com/tpott/subtitler/backend/totp"
)

func registerAuthHandlers(mux *http.ServeMux) { //nolint:funlen // route registration
	// Delegate TOTP/2FA handlers to separate file
	registerAuthTOTPHandlers(mux)

	// Delegate password reset, magic link, and email verification handlers
	registerAuthRecoveryHandlers(mux)

	// Auth: Register new user (rate limited)
	mux.HandleFunc("POST /api/auth/register", authLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		// Parse request body (limited to 64KB - auth payloads are small)
		var req struct {
			Email        string `json:"email"`
			Password     string `json:"password"`
			CaptchaToken string `json:"captcha_token,omitempty"`
		}
		if err := httputil.DecodeJSONBody(r, w, &req, 64*1024); err != nil {
			httputil.RespondError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		// Verify CAPTCHA if enabled
		if captchaVerifier.IsEnabled() {
			clientIP := ratelimit.GetClientIP(r)
			if err := captchaVerifier.Verify(r.Context(), req.CaptchaToken, clientIP); err != nil {
				logging.WarnContext(r.Context(), "CAPTCHA verification failed", "error", err, "ip", clientIP)
				httputil.RespondError(w, http.StatusBadRequest, "CAPTCHA verification failed. Please try again.")
				return
			}
		}

		// Normalize email
		req.Email = strings.TrimSpace(strings.ToLower(req.Email))

		// Validate email
		if err := auth.ValidateEmail(req.Email); err != nil {
			httputil.RespondError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Validate password
		if err := auth.ValidatePassword(req.Password); err != nil {
			httputil.RespondError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Check if email already exists
		existingUser, err := database.GetUserByEmail(req.Email)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error checking existing user", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Registration failed")
			return
		}
		if existingUser != nil {
			httputil.RespondError(w, http.StatusConflict, "Email already registered")
			return
		}

		// Hash password
		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error hashing password", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Registration failed")
			return
		}

		// Generate user ID
		userID, err := auth.GenerateID()
		if err != nil {
			logging.ErrorContext(r.Context(), "Error generating user ID", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Registration failed")
			return
		}

		// Create user (email_verified defaults to false)
		user := &db.User{
			ID:            userID,
			Email:         req.Email,
			PasswordHash:  hash,
			EmailVerified: false,
			CreatedAt:     time.Now(),
		}
		if err := database.CreateUser(user); err != nil {
			logging.ErrorContext(r.Context(), "Error creating user", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Registration failed")
			return
		}

		// Generate email verification token (32 bytes = 256 bits entropy)
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			logging.ErrorContext(r.Context(), "Error generating verification token", "error", err)
			// User created but verification email failed - return success with message
			httputil.RespondJSON(w, http.StatusCreated, map[string]interface{}{
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
		_, err = database.CreateEmailVerificationToken(user.ID, tokenHash, expiresAt)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error creating verification token", "error", err)
		}

		// Send verification email
		if err := emailService.SendEmailVerification(r.Context(), user.Email, token); err != nil {
			logging.ErrorContext(r.Context(), "Error sending verification email", "error", err)
		}

		clientIP := ratelimit.GetClientIP(r)
		security.Registration(r.Context(), clientIP, user.ID, user.Email)
		httputil.RespondJSON(w, http.StatusCreated, map[string]interface{}{
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
	// Email rate limiting: configurable via MAX_LOGIN_ATTEMPTS and LOGIN_LOCK_DURATION env vars
	mux.HandleFunc("POST /api/auth/login", authLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		// Parse request body (limited to 64KB - auth payloads are small)
		var req struct {
			Email        string `json:"email"`
			Password     string `json:"password"`
			TOTPCode     string `json:"totp_code,omitempty"`     // Required if 2FA is enabled
			CaptchaToken string `json:"captcha_token,omitempty"` // Required if CAPTCHA is enabled
		}
		if err := httputil.DecodeJSONBody(r, w, &req, 64*1024); err != nil {
			httputil.RespondError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		// Normalize email
		req.Email = strings.TrimSpace(strings.ToLower(req.Email))
		clientIP := ratelimit.GetClientIP(r)

		// Verify CAPTCHA if enabled (only for initial login attempt, not TOTP retry)
		// Skip CAPTCHA for TOTP code submission (user already passed CAPTCHA on initial login)
		if captchaVerifier.IsEnabled() && req.TOTPCode == "" {
			if err := captchaVerifier.Verify(r.Context(), req.CaptchaToken, clientIP); err != nil {
				security.LoginFailedCaptcha(r.Context(), clientIP, err.Error())
				httputil.RespondError(w, http.StatusBadRequest, "CAPTCHA verification failed. Please try again.")
				return
			}
		}

		// Check if email is locked due to too many failed attempts
		locked, unlockTime, err := database.IsEmailLocked(req.Email, maxLoginAttempts, loginLockDuration)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error checking email lock", "email", req.Email, "error", err)
		}
		if locked {
			remainingMins := int(time.Until(unlockTime).Minutes()) + 1
			security.LoginFailedLocked(r.Context(), clientIP, req.Email, remainingMins)
			httputil.RespondJSON(w, http.StatusTooManyRequests, map[string]interface{}{
				"error":           "Too many failed login attempts. Please try again later.",
				"retry_after_min": remainingMins,
			})
			return
		}

		// Helper to record failed attempt
		recordFailure := func() {
			if err := database.RecordLoginAttempt(req.Email, clientIP, false); err != nil {
				logging.ErrorContext(r.Context(), "Error recording login attempt", "email", req.Email, "error", err)
			}
		}

		// Get user by email
		user, err := database.GetUserByEmail(req.Email)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting user", "email", req.Email, "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Login failed")
			return
		}
		if user == nil {
			recordFailure()
			security.LoginFailedUserNotFound(r.Context(), clientIP, req.Email)
			httputil.RespondError(w, http.StatusUnauthorized, "Invalid email or password")
			return
		}

		// Check password
		if !auth.CheckPassword(req.Password, user.PasswordHash) {
			recordFailure()
			// Get current attempt count for logging (includes the one we just recorded)
			attempts, _ := database.GetRecentFailedLoginAttempts(req.Email, time.Now().Add(-loginLockDuration))
			security.LoginFailedPassword(r.Context(), clientIP, req.Email, attempts)
			// Check if this attempt caused a lockout
			if attempts >= maxLoginAttempts {
				security.AccountLocked(r.Context(), clientIP, req.Email, attempts)
			}
			httputil.RespondError(w, http.StatusUnauthorized, "Invalid email or password")
			return
		}

		// Check if email is verified
		if !user.EmailVerified {
			security.LoginFailedUnverified(r.Context(), clientIP, req.Email)
			httputil.RespondJSON(w, http.StatusForbidden, map[string]interface{}{
				"error":                   "Please verify your email address before logging in",
				"email_verification":      true,
				"email_not_verified":      true,
				"can_resend_verification": true,
			})
			return
		}

		// Check 2FA if enabled
		if user.TOTPEnabled {
			if req.TOTPCode == "" {
				// Don't record as failed attempt - just needs 2FA code
				httputil.RespondJSON(w, http.StatusUnauthorized, map[string]interface{}{
					"error":         "2FA code required",
					"totp_required": true,
				})
				return
			}

			// Validate the TOTP code
			if user.TOTPSecret == nil || !totp.Validate(*user.TOTPSecret, req.TOTPCode) {
				recordFailure()
				security.TwoFAVerifyFailed(r.Context(), clientIP, user.ID, user.Email)
				httputil.RespondError(w, http.StatusUnauthorized, "Invalid 2FA code")
				return
			}
			security.TwoFAVerifySuccess(r.Context(), clientIP, user.ID, user.Email)
		}

		// Login successful - clear failed attempts for this email
		if err := database.ClearLoginAttempts(req.Email); err != nil {
			logging.ErrorContext(r.Context(), "Error clearing login attempts", "email", req.Email, "error", err)
		}

		// Create session with IP and user agent
		userAgent := r.Header.Get("User-Agent")
		session, err := auth.CreateSession(database, user.ID, clientIP, userAgent)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error creating session", "user_id", user.ID, "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Login failed")
			return
		}

		// Set session cookie
		auth.SetSessionCookie(w, session.Token, session.ExpiresAt)

		// Update metrics after session creation
		updateSessionMetrics()

		security.LoginSuccess(r.Context(), clientIP, user.ID, user.Email, userAgent)
		security.SessionCreated(r.Context(), clientIP, user.ID, session.ID, userAgent)
		httputil.RespondJSON(w, http.StatusOK, map[string]interface{}{
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
	mux.HandleFunc("POST /api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		clientIP := ratelimit.GetClientIP(r)

		token := auth.GetTokenFromRequest(r)
		var userID string
		if token != "" {
			// Get user ID before deleting session for audit logging
			if user, _, err := auth.ValidateSession(database, token); err == nil && user != nil {
				userID = user.ID
			}
			if err := database.DeleteSession(token); err != nil {
				logging.ErrorContext(r.Context(), "Error deleting session", "error", err)
			} else {
				// Update metrics after successful session deletion
				updateSessionMetrics()
			}
		}

		if userID != "" {
			security.Logout(r.Context(), clientIP, userID)
		}
		auth.ClearSessionCookie(w)

		httputil.RespondJSON(w, http.StatusOK, map[string]string{"message": "Logged out successfully"})
	})

	// Auth: Get current user
	mux.HandleFunc("GET /api/auth/me", func(w http.ResponseWriter, r *http.Request) {
		token := auth.GetTokenFromRequest(r)
		user, _, err := auth.ValidateSession(database, token)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error validating session", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to get user")
			return
		}

		if user == nil {
			httputil.RespondError(w, http.StatusUnauthorized, "Not authenticated")
			return
		}

		httputil.RespondJSON(w, http.StatusOK, map[string]interface{}{
			"user": map[string]interface{}{
				"id":             user.ID,
				"email":          user.Email,
				"created_at":     user.CreatedAt,
				"totp_enabled":   user.TOTPEnabled,
				"email_verified": user.EmailVerified,
			},
		})
	})

	// Auth: Get CSRF token for current session
	mux.HandleFunc("GET /api/auth/csrf", func(w http.ResponseWriter, r *http.Request) {

		sessionToken := auth.GetTokenFromRequest(r)
		if sessionToken == "" {
			httputil.RespondError(w, http.StatusUnauthorized, "Not authenticated")
			return
		}

		// Validate the session exists
		user, _, err := auth.ValidateSession(database, sessionToken)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error validating session", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to validate session")
			return
		}

		if user == nil {
			httputil.RespondError(w, http.StatusUnauthorized, "Not authenticated")
			return
		}

		// Generate CSRF token from session token
		csrfToken := csrf.GenerateToken(sessionToken)

		httputil.RespondJSON(w, http.StatusOK, map[string]string{
			"csrf_token": csrfToken,
		})
	})

	// Auth: Get all sessions for current user
	mux.HandleFunc("GET /api/auth/sessions", func(w http.ResponseWriter, r *http.Request) {

		token := auth.GetTokenFromRequest(r)
		user, currentSession, err := auth.ValidateSession(database, token)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error validating session", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to validate session")
			return
		}

		if user == nil {
			httputil.RespondError(w, http.StatusUnauthorized, "Authentication required")
			return
		}

		sessions, err := database.GetSessionsByUserID(user.ID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting sessions", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to get sessions")
			return
		}

		// Format sessions for API response, marking current session
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

		httputil.RespondJSON(w, http.StatusOK, map[string]interface{}{
			"sessions": responseSessions,
		})
	})

	// Auth: Revoke a specific session
	mux.HandleFunc("DELETE /api/auth/sessions/{id}", func(w http.ResponseWriter, r *http.Request) {

		token := auth.GetTokenFromRequest(r)
		user, currentSession, err := auth.ValidateSession(database, token)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error validating session", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to validate session")
			return
		}

		if user == nil {
			httputil.RespondError(w, http.StatusUnauthorized, "Authentication required")
			return
		}

		sessionID, valid := validatePathID(w, r.PathValue("id"), "Session ID")
		if !valid {
			return
		}

		// Prevent deleting current session through this endpoint
		if sessionID == currentSession.ID {
			httputil.RespondError(w, http.StatusBadRequest, "Cannot revoke current session. Use logout instead.")
			return
		}

		err = database.DeleteSessionByID(sessionID, user.ID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error deleting session", "error", err)
			httputil.RespondError(w, http.StatusNotFound, "Session not found")
			return
		}

		// Update metrics after session deletion
		updateSessionMetrics()

		clientIP := ratelimit.GetClientIP(r)
		security.SessionRevoked(r.Context(), clientIP, user.ID, sessionID, false)

		httputil.RespondJSON(w, http.StatusOK, map[string]interface{}{
			"status":  "success",
			"message": "Session revoked",
		})
	})

}
