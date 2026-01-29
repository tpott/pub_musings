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
	"github.com/tpott/subtitler/backend/validation"
)

func registerAuthHandlers(mux *http.ServeMux) {
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
	// Email rate limiting: 5 failed attempts = 15 minute lockout
	const maxLoginAttempts = 5
	const loginLockDuration = 15 * time.Minute

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

	// 2FA: Start TOTP setup - generates a new secret (rate limited)
	mux.HandleFunc("POST /api/auth/totp/setup", authLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		clientIP := ratelimit.GetClientIP(r)

		// Require authentication
		token := auth.GetTokenFromRequest(r)
		user, _, err := auth.ValidateSession(database, token)
		if err != nil || user == nil {
			httputil.RespondError(w, http.StatusUnauthorized, "Authentication required")
			return
		}

		// Check if 2FA is already enabled
		if user.TOTPEnabled {
			httputil.RespondError(w, http.StatusBadRequest, "2FA is already enabled. Disable it first to set up a new authenticator.")
			return
		}

		// Generate a new TOTP secret
		secret, err := totp.GenerateSecret()
		if err != nil {
			logging.ErrorContext(r.Context(), "Error generating TOTP secret", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to generate secret")
			return
		}

		// Save the secret to the database (not yet enabled)
		if err := database.SetTOTPSecret(user.ID, secret); err != nil {
			logging.ErrorContext(r.Context(), "Error saving TOTP secret", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to save secret")
			return
		}

		// Generate the provisioning URI for QR code
		issuer := "Subtitler"
		uri := totp.GenerateProvisioningURI(secret, user.Email, issuer)

		// Generate QR code as base64 data URL
		qrCode, err := totp.GenerateQRCode(uri)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error generating QR code", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to generate QR code")
			return
		}

		security.TwoFASetupInitiated(r.Context(), clientIP, user.ID, user.Email)
		httputil.RespondJSON(w, http.StatusOK, map[string]interface{}{
			"secret":         secret,
			"secret_display": totp.FormatSecretForDisplay(secret),
			"uri":            uri,
			"issuer":         issuer,
			"qr_code":        qrCode,
		})
	}))

	// 2FA: Verify TOTP code and enable 2FA (rate limited)
	mux.HandleFunc("POST /api/auth/totp/verify", authLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {

		// Require authentication
		token := auth.GetTokenFromRequest(r)
		user, _, err := auth.ValidateSession(database, token)
		if err != nil || user == nil {
			httputil.RespondError(w, http.StatusUnauthorized, "Authentication required")
			return
		}

		// Check if 2FA is already enabled
		if user.TOTPEnabled {
			httputil.RespondError(w, http.StatusBadRequest, "2FA is already enabled")
			return
		}

		// Check if a secret has been set up
		if user.TOTPSecret == nil || *user.TOTPSecret == "" {
			httputil.RespondError(w, http.StatusBadRequest, "No TOTP secret found. Please start setup first.")
			return
		}

		// Parse request body
		var req struct {
			Code string `json:"code"`
		}
		if err := httputil.DecodeJSONBody(r, w, &req, 0); err != nil {
			if err.Error() == "http: request body too large" {
				httputil.RespondError(w, http.StatusRequestEntityTooLarge, "Request body too large")
			} else {
				httputil.RespondError(w, http.StatusBadRequest, "Invalid request body")
			}
			return
		}

		// Validate TOTP code length
		if err := validation.ValidateTOTPCode(req.Code); err != nil {
			httputil.RespondError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Validate the code
		clientIP := ratelimit.GetClientIP(r)
		if !totp.Validate(*user.TOTPSecret, req.Code) {
			security.TwoFAVerifyFailed(r.Context(), clientIP, user.ID, user.Email)
			httputil.RespondError(w, http.StatusBadRequest, "Invalid code. Please try again.")
			return
		}

		// Generate recovery codes
		recoveryCodes, err := totp.GenerateRecoveryCodes(totp.NumCodes)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error generating recovery codes", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to generate recovery codes")
			return
		}

		// Hash recovery codes
		codeHashes := make([]string, len(recoveryCodes))
		for i, code := range recoveryCodes {
			hash, err := totp.HashCode(code)
			if err != nil {
				logging.ErrorContext(r.Context(), "Error hashing recovery code", "error", err)
				httputil.RespondError(w, http.StatusInternalServerError, "Failed to generate recovery codes")
				return
			}
			codeHashes[i] = hash
		}

		// Enable 2FA and save recovery codes in a single transaction
		if err := database.EnableTOTPWithRecoveryCodes(user.ID, codeHashes); err != nil {
			logging.ErrorContext(r.Context(), "Error enabling TOTP with recovery codes", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to enable 2FA")
			return
		}

		security.TwoFAEnabled(r.Context(), clientIP, user.ID, user.Email)
		httputil.RespondJSON(w, http.StatusOK, map[string]interface{}{
			"message":        "2FA has been enabled successfully",
			"totp_enabled":   true,
			"recovery_codes": recoveryCodes,
		})
	}))

	// 2FA: Disable TOTP (rate limited)
	mux.HandleFunc("POST /api/auth/totp/disable", authLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {

		// Require authentication
		token := auth.GetTokenFromRequest(r)
		user, _, err := auth.ValidateSession(database, token)
		if err != nil || user == nil {
			httputil.RespondError(w, http.StatusUnauthorized, "Authentication required")
			return
		}

		// Check if 2FA is enabled
		if !user.TOTPEnabled {
			httputil.RespondError(w, http.StatusBadRequest, "2FA is not enabled")
			return
		}

		// Parse request body - require current TOTP code and password for security
		var req struct {
			Code     string `json:"code"`
			Password string `json:"password"`
		}
		if err := httputil.DecodeJSONBody(r, w, &req, 0); err != nil {
			if err.Error() == "http: request body too large" {
				httputil.RespondError(w, http.StatusRequestEntityTooLarge, "Request body too large")
			} else {
				httputil.RespondError(w, http.StatusBadRequest, "Invalid request body")
			}
			return
		}

		// Validate TOTP code length
		if err := validation.ValidateTOTPCode(req.Code); err != nil {
			httputil.RespondError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Verify password
		if !auth.CheckPassword(req.Password, user.PasswordHash) {
			httputil.RespondError(w, http.StatusUnauthorized, "Invalid password")
			return
		}

		// Validate the TOTP code
		clientIP := ratelimit.GetClientIP(r)
		if user.TOTPSecret == nil || !totp.Validate(*user.TOTPSecret, req.Code) {
			security.TwoFAVerifyFailed(r.Context(), clientIP, user.ID, user.Email)
			httputil.RespondError(w, http.StatusBadRequest, "Invalid 2FA code")
			return
		}

		// Disable 2FA
		if err := database.DisableTOTP(user.ID); err != nil {
			logging.ErrorContext(r.Context(), "Error disabling TOTP", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to disable 2FA")
			return
		}

		// Delete recovery codes
		if err := database.DeleteRecoveryCodes(user.ID); err != nil {
			logging.ErrorContext(r.Context(), "Error deleting recovery codes", "error", err)
			// Continue - 2FA is disabled even if codes couldn't be deleted
		}

		security.TwoFADisabled(r.Context(), clientIP, user.ID, user.Email, false)
		httputil.RespondJSON(w, http.StatusOK, map[string]interface{}{
			"message":      "2FA has been disabled successfully",
			"totp_enabled": false,
		})
	}))

	// 2FA: Recover account using recovery code (rate limited)
	mux.HandleFunc("POST /api/auth/totp/recover", authLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {

		// Parse request body
		var req struct {
			Email        string `json:"email"`
			Password     string `json:"password"`
			RecoveryCode string `json:"recovery_code"`
		}
		if err := httputil.DecodeJSONBody(r, w, &req, 0); err != nil {
			if err.Error() == "http: request body too large" {
				httputil.RespondError(w, http.StatusRequestEntityTooLarge, "Request body too large")
			} else {
				httputil.RespondError(w, http.StatusBadRequest, "Invalid request body")
			}
			return
		}

		// Validate required fields
		if req.Email == "" || req.Password == "" || req.RecoveryCode == "" {
			httputil.RespondError(w, http.StatusBadRequest, "Email, password, and recovery code are required")
			return
		}

		// Validate email length
		if err := validation.ValidateEmail(req.Email); err != nil {
			httputil.RespondError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Validate recovery code length
		if err := validation.ValidateRecoveryCode(req.RecoveryCode); err != nil {
			httputil.RespondError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Get user by email
		user, err := database.GetUserByEmail(req.Email)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting user", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Server error")
			return
		}
		if user == nil {
			httputil.RespondError(w, http.StatusUnauthorized, "Invalid email or password")
			return
		}

		// Verify password
		if !auth.CheckPassword(req.Password, user.PasswordHash) {
			httputil.RespondError(w, http.StatusUnauthorized, "Invalid email or password")
			return
		}

		// Check if 2FA is enabled
		if !user.TOTPEnabled {
			httputil.RespondError(w, http.StatusBadRequest, "2FA is not enabled for this account")
			return
		}

		// Get unused recovery codes
		codes, err := database.GetUnusedRecoveryCodes(user.ID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting recovery codes", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Server error")
			return
		}

		// Check each code until we find a match
		var matchedCodeID string
		normalizedInput := totp.NormalizeCode(req.RecoveryCode)
		for _, code := range codes {
			if totp.CheckCode(normalizedInput, code.CodeHash) {
				matchedCodeID = code.ID
				break
			}
		}

		clientIP := ratelimit.GetClientIP(r)
		if matchedCodeID == "" {
			security.RecoveryCodeFailed(r.Context(), clientIP, req.Email)
			httputil.RespondError(w, http.StatusUnauthorized, "Invalid recovery code")
			return
		}

		// Mark the code as used
		success, err := database.UseRecoveryCode(matchedCodeID)
		if err != nil || !success {
			logging.ErrorContext(r.Context(), "Error using recovery code", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to use recovery code")
			return
		}

		// Disable 2FA, delete recovery codes, and clear sessions in a single transaction
		if err := database.DisableTOTPAndClearSessions(user.ID); err != nil {
			logging.ErrorContext(r.Context(), "Error disabling TOTP and clearing sessions", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to disable 2FA")
			return
		}

		// Create a new session with IP and user agent
		userAgent := r.Header.Get("User-Agent")
		session, err := auth.CreateSession(database, user.ID, clientIP, userAgent)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error creating session", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to create session")
			return
		}

		// Set session cookie
		auth.SetSessionCookie(w, session.Token, session.ExpiresAt)

		// Update metrics after session creation
		updateSessionMetrics()

		security.RecoveryCodeUsed(r.Context(), clientIP, user.ID, user.Email)
		security.TwoFADisabled(r.Context(), clientIP, user.ID, user.Email, true)
		security.SessionCreated(r.Context(), clientIP, user.ID, session.ID, userAgent)
		httputil.RespondJSON(w, http.StatusOK, map[string]interface{}{
			"message":      "2FA has been disabled. Please set up 2FA again if you want to re-enable it.",
			"token":        session.Token,
			"totp_enabled": false,
		})
	}))

	// 2FA: Regenerate recovery codes (rate limited)
	mux.HandleFunc("POST /api/auth/totp/codes", authLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {

		// Check if user is authenticated
		token := auth.GetTokenFromRequest(r)
		user, _, err := auth.ValidateSession(database, token)
		if err != nil || user == nil {
			httputil.RespondError(w, http.StatusUnauthorized, "Authentication required")
			return
		}

		// Check if 2FA is enabled
		if !user.TOTPEnabled {
			httputil.RespondError(w, http.StatusBadRequest, "2FA is not enabled")
			return
		}

		// Parse request body - require password and TOTP code for security
		var req struct {
			Code     string `json:"code"`
			Password string `json:"password"`
		}
		if err := httputil.DecodeJSONBody(r, w, &req, 0); err != nil {
			if err.Error() == "http: request body too large" {
				httputil.RespondError(w, http.StatusRequestEntityTooLarge, "Request body too large")
			} else {
				httputil.RespondError(w, http.StatusBadRequest, "Invalid request body")
			}
			return
		}

		// Validate TOTP code length
		if err := validation.ValidateTOTPCode(req.Code); err != nil {
			httputil.RespondError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Verify password
		if !auth.CheckPassword(req.Password, user.PasswordHash) {
			httputil.RespondError(w, http.StatusUnauthorized, "Invalid password")
			return
		}

		// Validate the TOTP code
		if user.TOTPSecret == nil || !totp.Validate(*user.TOTPSecret, req.Code) {
			httputil.RespondError(w, http.StatusBadRequest, "Invalid 2FA code")
			return
		}

		// Generate new recovery codes
		recoveryCodes, err := totp.GenerateRecoveryCodes(totp.NumCodes)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error generating recovery codes", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to generate recovery codes")
			return
		}

		// Hash and store recovery codes (this deletes old codes first)
		codeHashes := make([]string, len(recoveryCodes))
		for i, code := range recoveryCodes {
			hash, err := totp.HashCode(code)
			if err != nil {
				logging.ErrorContext(r.Context(), "Error hashing recovery code", "error", err)
				httputil.RespondError(w, http.StatusInternalServerError, "Failed to generate recovery codes")
				return
			}
			codeHashes[i] = hash
		}

		if err := database.SaveRecoveryCodes(user.ID, codeHashes); err != nil {
			logging.ErrorContext(r.Context(), "Error saving recovery codes", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to save recovery codes")
			return
		}

		logging.InfoContext(r.Context(), "Regenerated recovery codes", "email", user.Email)
		httputil.RespondJSON(w, http.StatusOK, map[string]interface{}{
			"message":        "Recovery codes regenerated successfully",
			"recovery_codes": recoveryCodes,
		})
	}))

	// Auth: Forgot password - initiates password reset flow (stricter rate limiting)
	mux.HandleFunc("POST /api/auth/forgot-password", passwordResetLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		// Parse request body (limited to 8KB - just an email)
		var req struct {
			Email string `json:"email"`
		}
		if err := httputil.DecodeJSONBody(r, w, &req, 8*1024); err != nil {
			httputil.RespondError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		// Normalize email
		req.Email = strings.TrimSpace(strings.ToLower(req.Email))

		// Validate email format
		if err := auth.ValidateEmail(req.Email); err != nil {
			httputil.RespondError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Always return success to prevent email enumeration
		// Do the actual work in background-ish but keep same timing
		defer func() {
			httputil.RespondJSON(w, http.StatusOK, map[string]string{"message": "If an account exists with that email, a password reset link has been sent."})
		}()

		// Look up user (don't reveal if exists)
		clientIP := ratelimit.GetClientIP(r)
		user, err := database.GetUserByEmail(req.Email)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error looking up user for password reset", "error", err)
			return
		}
		if user == nil {
			// User doesn't exist - log for security monitoring, return success anyway
			security.PasswordResetRequested(r.Context(), clientIP, req.Email, false)
			return
		}

		// Generate reset token (32 bytes = 256 bits entropy)
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			logging.ErrorContext(r.Context(), "Error generating reset token", "error", err)
			return
		}
		token := hex.EncodeToString(tokenBytes)
		tokenHash := email.HashToken(token)

		// Create reset token with 1-hour expiry
		expiresAt := time.Now().Add(1 * time.Hour)
		_, err = database.CreatePasswordResetToken(user.ID, tokenHash, expiresAt)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error creating password reset token", "error", err)
			return
		}

		// Send password reset email
		ctx := r.Context()
		if err := emailService.SendPasswordReset(ctx, user.Email, token); err != nil {
			logging.ErrorContext(r.Context(), "Error sending password reset email", "error", err)
			// Still return success to prevent enumeration
			return
		}

		security.PasswordResetRequested(r.Context(), clientIP, user.Email, true)
	}))

	// Auth: Reset password - completes password reset with token
	mux.HandleFunc("POST /api/auth/reset-password", authLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {

		// Parse request body (limited to 64KB - token + password)
		var req struct {
			Token    string `json:"token"`
			Password string `json:"password"`
		}
		if err := httputil.DecodeJSONBody(r, w, &req, 64*1024); err != nil {
			httputil.RespondError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		// Validate token format
		if req.Token == "" {
			httputil.RespondError(w, http.StatusBadRequest, "Reset token is required")
			return
		}

		// Validate password
		if err := auth.ValidatePassword(req.Password); err != nil {
			httputil.RespondError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Hash the token and look it up
		tokenHash := email.HashToken(req.Token)
		resetToken, err := database.GetPasswordResetToken(tokenHash)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error looking up reset token", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to process reset request")
			return
		}

		// Check if token exists and is valid
		clientIP := ratelimit.GetClientIP(r)
		if resetToken == nil {
			security.PasswordResetFailed(r.Context(), clientIP, "token_not_found")
			httputil.RespondError(w, http.StatusBadRequest, "Invalid or expired reset token")
			return
		}

		// Check if token is used or expired
		if resetToken.Used || time.Now().After(resetToken.ExpiresAt) {
			security.PasswordResetFailed(r.Context(), clientIP, "token_expired_or_used")
			httputil.RespondError(w, http.StatusBadRequest, "Invalid or expired reset token")
			return
		}

		// Hash the new password
		passwordHash, err := auth.HashPassword(req.Password)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error hashing new password", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to process reset request")
			return
		}

		// Complete password reset in a single transaction:
		// - Update password
		// - Mark token as used
		// - Delete all tokens for user
		// - Delete all sessions for user
		if err := database.CompletePasswordReset(resetToken.UserID, tokenHash, passwordHash); err != nil {
			logging.ErrorContext(r.Context(), "Error completing password reset", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to update password")
			return
		}

		security.PasswordResetSuccess(r.Context(), clientIP, resetToken.UserID)
		httputil.RespondJSON(w, http.StatusOK, map[string]string{"message": "Password has been reset successfully. Please log in with your new password."})
	}))

	// Auth: Request magic link - sends a login link via email (stricter rate limiting)
	mux.HandleFunc("POST /api/auth/magic-link", passwordResetLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {

		var req struct {
			Email string `json:"email"`
		}
		if err := httputil.DecodeJSONBody(r, w, &req, 0); err != nil {
			if err.Error() == "http: request body too large" {
				httputil.RespondError(w, http.StatusRequestEntityTooLarge, "Request body too large")
			} else {
				httputil.RespondError(w, http.StatusBadRequest, "Invalid request body")
			}
			return
		}

		// Normalize email
		req.Email = strings.TrimSpace(strings.ToLower(req.Email))

		// Validate email format
		if err := auth.ValidateEmail(req.Email); err != nil {
			httputil.RespondError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Always return success to prevent email enumeration
		defer func() {
			httputil.RespondJSON(w, http.StatusOK, map[string]string{"message": "If an account exists with that email, a login link has been sent."})
		}()

		// Look up user (don't reveal if exists)
		user, err := database.GetUserByEmail(req.Email)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error looking up user for magic link", "error", err)
			return
		}
		if user == nil {
			// User doesn't exist - return success anyway
			logging.DebugContext(r.Context(), "Magic link requested for non-existent email", "email", req.Email)
			return
		}

		// Check if email is verified
		if !user.EmailVerified {
			logging.DebugContext(r.Context(), "Magic link requested for unverified email", "email", req.Email)
			return
		}

		// Per-email rate limit: max 3 magic link requests per 15 minutes
		const maxMagicLinkRequests = 3
		const magicLinkRateLimitWindow = 15 * time.Minute
		recentCount, err := database.CountRecentMagicLinkRequests(user.ID, time.Now().Add(-magicLinkRateLimitWindow))
		if err != nil {
			logging.ErrorContext(r.Context(), "Error checking magic link rate limit", "error", err)
			return
		}
		if recentCount >= maxMagicLinkRequests {
			logging.WarnContext(r.Context(), "Magic link rate limit exceeded", "email", req.Email, "count", recentCount)
			security.MagicLinkRateLimitExceeded(user.ID, req.Email)
			// Still return generic success to prevent enumeration
			return
		}

		// Generate magic link token (32 bytes = 256 bits entropy)
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			logging.ErrorContext(r.Context(), "Error generating magic link token", "error", err)
			return
		}
		token := hex.EncodeToString(tokenBytes)
		tokenHash := email.HashToken(token)

		// Create magic link token with 15-minute expiry
		expiresAt := time.Now().Add(15 * time.Minute)
		_, err = database.CreateMagicLinkToken(user.ID, tokenHash, expiresAt)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error creating magic link token", "error", err)
			return
		}

		// Send magic link email
		ctx := r.Context()
		if err := emailService.SendMagicLink(ctx, user.Email, token); err != nil {
			logging.ErrorContext(r.Context(), "Error sending magic link email", "error", err)
			// Still return success to prevent enumeration
			return
		}

		logging.InfoContext(r.Context(), "Magic link email sent", "email", user.Email)
	}))

	// Auth: Verify magic link - logs user in with magic link token
	mux.HandleFunc("GET /api/auth/magic-link/verify", authLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {

		// Get token from query parameter
		token := r.URL.Query().Get("token")
		if token == "" {
			httputil.RespondError(w, http.StatusBadRequest, "Token is required")
			return
		}

		// Hash the token and look it up
		tokenHash := email.HashToken(token)
		magicToken, err := database.GetMagicLinkToken(tokenHash)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error looking up magic link token", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to verify magic link")
			return
		}

		// Check if token exists
		if magicToken == nil {
			httputil.RespondError(w, http.StatusBadRequest, "Invalid or expired magic link")
			return
		}

		// Check if token is used or expired
		if magicToken.Used || time.Now().After(magicToken.ExpiresAt) {
			httputil.RespondError(w, http.StatusBadRequest, "Invalid or expired magic link")
			return
		}

		// Mark token as used atomically
		used, err := database.UseMagicLinkToken(tokenHash)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error marking magic link token as used", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to verify magic link")
			return
		}
		if !used {
			// Token was already used or expired
			httputil.RespondError(w, http.StatusBadRequest, "Invalid or expired magic link")
			return
		}

		// Get user
		user, err := database.GetUserByID(magicToken.UserID)
		if err != nil || user == nil {
			logging.ErrorContext(r.Context(), "Error getting user for magic link", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to complete login")
			return
		}

		// Create session (similar to normal login)
		clientIP := ratelimit.GetClientIP(r)
		userAgent := r.Header.Get("User-Agent")
		session, err := auth.CreateSession(database, user.ID, clientIP, userAgent)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error creating session", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to create session")
			return
		}

		// Set session cookie
		auth.SetSessionCookie(w, session.Token, session.ExpiresAt)

		// Update metrics after session creation
		updateSessionMetrics()

		logging.InfoContext(r.Context(), "Magic link login successful", "user_id", user.ID)
		httputil.RespondJSON(w, http.StatusOK, map[string]interface{}{
			"message": "Login successful",
			"user": map[string]interface{}{
				"id":    user.ID,
				"email": user.Email,
			},
		})
	}))

	// Auth: Verify email - verifies email address with token
	mux.HandleFunc("GET /api/auth/verify", authLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {

		// Get token from query parameter
		token := r.URL.Query().Get("token")
		if token == "" {
			httputil.RespondError(w, http.StatusBadRequest, "Verification token is required")
			return
		}

		// Hash the token and look it up
		tokenHash := email.HashToken(token)
		verifyToken, err := database.GetEmailVerificationToken(tokenHash)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error looking up verification token", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to process verification request")
			return
		}

		// Check if token exists and is valid
		if verifyToken == nil {
			httputil.RespondError(w, http.StatusBadRequest, "Invalid or expired verification token")
			return
		}

		// Check if token is used or expired
		if verifyToken.Used || time.Now().After(verifyToken.ExpiresAt) {
			httputil.RespondError(w, http.StatusBadRequest, "Invalid or expired verification token")
			return
		}

		// Verify the email (marks token as used and sets email_verified=1)
		success, err := database.UseEmailVerificationToken(tokenHash)
		if err != nil || !success {
			logging.ErrorContext(r.Context(), "Error verifying email", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to verify email")
			return
		}

		logging.InfoContext(r.Context(), "Email verified", "user_id", verifyToken.UserID)
		httputil.RespondJSON(w, http.StatusOK, map[string]string{
			"message": "Email verified successfully. You can now log in.",
		})
	}))

	// Auth: Resend verification email (rate limited)
	mux.HandleFunc("POST /api/auth/resend-verification", passwordResetLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {

		var req struct {
			Email string `json:"email"`
		}
		if err := httputil.DecodeJSONBody(r, w, &req, 0); err != nil {
			if err.Error() == "http: request body too large" {
				httputil.RespondError(w, http.StatusRequestEntityTooLarge, "Request body too large")
			} else {
				httputil.RespondError(w, http.StatusBadRequest, "Invalid request body")
			}
			return
		}

		// Normalize email
		req.Email = strings.TrimSpace(strings.ToLower(req.Email))

		// Validate email format
		if err := auth.ValidateEmail(req.Email); err != nil {
			httputil.RespondError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Always return success to prevent email enumeration
		defer func() {
			httputil.RespondJSON(w, http.StatusOK, map[string]string{"message": "If an unverified account exists with that email, a verification link has been sent."})
		}()

		// Look up user
		user, err := database.GetUserByEmail(req.Email)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error looking up user for resend verification", "error", err)
			return
		}
		if user == nil {
			// User doesn't exist - return success anyway
			return
		}

		// Check if already verified
		if user.EmailVerified {
			// Already verified - return success anyway to prevent enumeration
			return
		}

		// Generate new verification token (32 bytes = 256 bits entropy)
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			logging.ErrorContext(r.Context(), "Error generating verification token", "error", err)
			return
		}
		token := hex.EncodeToString(tokenBytes)
		tokenHash := email.HashToken(token)

		// Create verification token with 24-hour expiry
		expiresAt := time.Now().Add(24 * time.Hour)
		_, err = database.CreateEmailVerificationToken(user.ID, tokenHash, expiresAt)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error creating verification token", "error", err)
			return
		}

		// Send verification email
		ctx := r.Context()
		if err := emailService.SendEmailVerification(ctx, user.Email, token); err != nil {
			logging.ErrorContext(r.Context(), "Error sending verification email", "error", err)
			return
		}

		logging.InfoContext(r.Context(), "Verification email resent", "email", user.Email)
	}))
}
