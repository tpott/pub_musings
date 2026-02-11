package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/tpott/pub_musings/peekaboo/backend/auth"
	"github.com/tpott/pub_musings/peekaboo/backend/db"
	"github.com/tpott/pub_musings/peekaboo/backend/logging"
)

// maxAuthBodySize is the maximum allowed request body size for auth endpoints.
const maxAuthBodySize = 4 << 10 // 4KB
const maxTokenLength = 128      // hex-encoded 64-byte token = 128 chars

// EmailSender is an interface for sending emails. Implementations can use
// SMTP, a third-party service, or a no-op logger for development.
type EmailSender interface {
	SendVerificationEmail(to, token string) error
	SendMagicLinkEmail(to, token string) error
}

// LogEmailSender logs emails instead of sending them (for development).
type LogEmailSender struct{}

// SendVerificationEmail logs the verification email details.
func (s *LogEmailSender) SendVerificationEmail(to, token string) error {
	slog.Info("verification email (not sent)",
		"to", to,
		"token", token)
	return nil
}

// SendMagicLinkEmail logs the magic link email details.
func (s *LogEmailSender) SendMagicLinkEmail(to, token string) error {
	slog.Info("magic link email (not sent)",
		"to", to,
		"token", token)
	return nil
}

// registerRequest is the incoming request to POST /api/auth/register.
type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// registerResponse is the response from POST /api/auth/register.
type registerResponse struct {
	Message           string       `json:"message,omitempty"`
	EmailVerification bool         `json:"email_verification,omitempty"`
	User              *userSummary `json:"user,omitempty"`
	Error             string       `json:"error,omitempty"`
}

// userSummary is a safe subset of user info for API responses.
type userSummary struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	CreatedAt     string `json:"created_at"`
	EmailVerified bool   `json:"email_verified"`
}

// verifyResponse is the response from GET /api/auth/verify.
type verifyResponse struct {
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// resendVerificationRequest is the incoming request to POST /api/auth/resend-verification.
type resendVerificationRequest struct {
	Email string `json:"email"`
}

// resendVerificationResponse is the response from POST /api/auth/resend-verification.
type resendVerificationResponse struct {
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	DB          *db.DB
	EmailSender EmailSender
	CSRFSecret  []byte
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(database *db.DB, emailSender EmailSender) *AuthHandler {
	if emailSender == nil {
		emailSender = &LogEmailSender{}
	}
	return &AuthHandler{
		DB:          database,
		EmailSender: emailSender,
	}
}

// HandleRegister handles POST /api/auth/register.
func (h *AuthHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodySize)

	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if err.Error() == "http: request body too large" {
			writeJSON(w, http.StatusRequestEntityTooLarge, registerResponse{Error: "request body too large"})
			return
		}
		writeJSON(w, http.StatusBadRequest, registerResponse{Error: "invalid JSON body"})
		return
	}

	// Normalize and validate email
	email := auth.NormalizeEmail(req.Email)
	if err := auth.ValidateEmail(email); err != nil {
		writeJSON(w, http.StatusBadRequest, registerResponse{Error: err.Error()})
		return
	}

	// Validate password
	if err := auth.ValidatePassword(req.Password); err != nil {
		writeJSON(w, http.StatusBadRequest, registerResponse{Error: err.Error()})
		return
	}

	// Check if email already exists
	existing, err := h.DB.GetUserByEmail(email)
	if err != nil {
		slog.Error("register: failed to check existing user",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, registerResponse{Error: "internal error"})
		return
	}
	if existing != nil {
		writeJSON(w, http.StatusConflict, registerResponse{Error: "email already registered"})
		return
	}

	// Hash password
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		slog.Error("register: failed to hash password",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, registerResponse{Error: "internal error"})
		return
	}

	// Generate user ID
	userID, err := auth.GenerateID()
	if err != nil {
		slog.Error("register: failed to generate user ID",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, registerResponse{Error: "internal error"})
		return
	}

	now := time.Now().UTC()
	user := &db.User{
		ID:            userID,
		Email:         email,
		PasswordHash:  passwordHash,
		EmailVerified: false,
		CreatedAt:     now,
	}

	if err := h.DB.CreateUser(user); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			writeJSON(w, http.StatusConflict, registerResponse{Error: "email already registered"})
			return
		}
		slog.Error("register: failed to create user",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, registerResponse{Error: "internal error"})
		return
	}

	// Generate email verification token
	token, err := auth.GenerateToken(32)
	if err != nil {
		slog.Error("register: failed to generate verification token",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		// User was created but token failed — still return success
		writeJSON(w, http.StatusCreated, registerResponse{
			Message:           "Account created. Please check your email to verify your account.",
			EmailVerification: true,
			User: &userSummary{
				ID:            userID,
				Email:         email,
				CreatedAt:     now.Format(time.RFC3339),
				EmailVerified: false,
			},
		})
		return
	}

	tokenHash := auth.HashToken(token)
	tokenID, err := auth.GenerateID()
	if err != nil {
		slog.Error("register: failed to generate token ID",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
	} else {
		expiresAt := now.Add(auth.EmailVerificationTokenExpiry)
		if err := h.DB.StoreEmailVerificationToken(tokenID, userID, tokenHash, expiresAt); err != nil {
			slog.Error("register: failed to store verification token",
				"error", err,
				"request_id", logging.GetRequestID(r.Context()))
		} else {
			if err := h.EmailSender.SendVerificationEmail(email, token); err != nil {
				slog.Error("register: failed to send verification email",
					"error", err,
					"request_id", logging.GetRequestID(r.Context()))
			}
		}
	}

	writeJSON(w, http.StatusCreated, registerResponse{
		Message:           "Account created. Please check your email to verify your account.",
		EmailVerification: true,
		User: &userSummary{
			ID:            userID,
			Email:         email,
			CreatedAt:     now.Format(time.RFC3339),
			EmailVerified: false,
		},
	})
}

// HandleVerify handles GET /api/auth/verify.
func (h *AuthHandler) HandleVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := r.URL.Query().Get("token")
	if token == "" {
		writeJSON(w, http.StatusBadRequest, verifyResponse{Error: "missing token parameter"})
		return
	}
	if len(token) > maxTokenLength {
		writeJSON(w, http.StatusBadRequest, verifyResponse{Error: "invalid token"})
		return
	}

	tokenHash := auth.HashToken(token)

	id, userID, expiresAt, used, err := h.DB.GetEmailVerificationToken(tokenHash)
	if err != nil {
		slog.Error("verify: failed to look up token",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, verifyResponse{Error: "internal error"})
		return
	}

	if id == "" {
		writeJSON(w, http.StatusBadRequest, verifyResponse{Error: "invalid or expired token"})
		return
	}

	if used {
		writeJSON(w, http.StatusBadRequest, verifyResponse{Error: "token already used"})
		return
	}

	if time.Now().UTC().After(expiresAt) {
		writeJSON(w, http.StatusBadRequest, verifyResponse{Error: "token expired"})
		return
	}

	// Mark token as used
	if err := h.DB.MarkEmailVerificationTokenUsed(id); err != nil {
		slog.Error("verify: failed to mark token used",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, verifyResponse{Error: "internal error"})
		return
	}

	// Set user as verified
	if err := h.DB.SetEmailVerified(userID); err != nil {
		slog.Error("verify: failed to set email verified",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, verifyResponse{Error: "internal error"})
		return
	}

	writeJSON(w, http.StatusOK, verifyResponse{
		Message: "Email verified successfully. You can now log in.",
	})
}

// loginRequest is the incoming request to POST /api/auth/login.
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	TOTPCode string `json:"totp_code,omitempty"`
}

// loginResponse is the response from POST /api/auth/login.
type loginResponse struct {
	User         *loginUserSummary `json:"user,omitempty"`
	Token        string            `json:"token,omitempty"`
	Error        string            `json:"error,omitempty"`
	TOTPRequired bool              `json:"totp_required,omitempty"`
	// Returned when email is not verified
	EmailNotVerified      bool `json:"email_not_verified,omitempty"`
	CanResendVerification bool `json:"can_resend_verification,omitempty"`
	// Returned when account is locked
	RetryAfterMin *int `json:"retry_after_min,omitempty"`
}

// loginUserSummary is user info returned on login.
type loginUserSummary struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	TOTPEnabled bool   `json:"totp_enabled"`
}

// logoutResponse is the response from POST /api/auth/logout.
type logoutResponse struct {
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// meResponse is the response from GET /api/auth/me.
type meResponse struct {
	User  *meUserSummary `json:"user,omitempty"`
	Error string         `json:"error,omitempty"`
}

// meUserSummary is detailed user info for /me endpoint.
type meUserSummary struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	TOTPEnabled   bool   `json:"totp_enabled"`
	EmailVerified bool   `json:"email_verified"`
	CreatedAt     string `json:"created_at"`
}

// HandleLogin handles POST /api/auth/login.
func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodySize)

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if err.Error() == "http: request body too large" {
			writeJSON(w, http.StatusRequestEntityTooLarge, loginResponse{Error: "request body too large"})
			return
		}
		writeJSON(w, http.StatusBadRequest, loginResponse{Error: "invalid JSON body"})
		return
	}

	email := auth.NormalizeEmail(req.Email)
	if err := auth.ValidateEmail(email); err != nil {
		// Use generic message to prevent email enumeration
		writeJSON(w, http.StatusUnauthorized, loginResponse{Error: "invalid email or password"})
		return
	}

	// Check account lockout
	since := time.Now().UTC().Add(-auth.LockoutWindow)
	failedCount, err := h.DB.CountRecentFailedAttempts(email, since)
	if err != nil {
		slog.Error("login: failed to count login attempts",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, loginResponse{Error: "internal error"})
		return
	}
	if failedCount >= auth.LockoutThreshold {
		retryMin := int(auth.LockoutDuration.Minutes())
		writeJSON(w, http.StatusTooManyRequests, loginResponse{
			Error:         "too many failed login attempts, please try again later",
			RetryAfterMin: &retryMin,
		})
		return
	}

	// Look up user
	user, err := h.DB.GetUserByEmail(email)
	if err != nil {
		slog.Error("login: failed to look up user",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, loginResponse{Error: "internal error"})
		return
	}

	clientIP := getClientIP(r)

	if user == nil {
		// Record failed attempt even for nonexistent users (prevents timing attacks)
		if err := h.DB.RecordLoginAttempt(email, clientIP, false); err != nil {
			slog.Error("login: failed to record login attempt",
				"error", err,
				"request_id", logging.GetRequestID(r.Context()))
		}
		writeJSON(w, http.StatusUnauthorized, loginResponse{Error: "invalid email or password"})
		return
	}

	// Verify password
	if err := auth.VerifyPassword(user.PasswordHash, req.Password); err != nil {
		if err := h.DB.RecordLoginAttempt(email, clientIP, false); err != nil {
			slog.Error("login: failed to record login attempt",
				"error", err,
				"request_id", logging.GetRequestID(r.Context()))
		}
		writeJSON(w, http.StatusUnauthorized, loginResponse{Error: "invalid email or password"})
		return
	}

	// Record successful attempt
	if err := h.DB.RecordLoginAttempt(email, clientIP, true); err != nil {
		slog.Error("login: failed to record login attempt",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
	}

	// Check email verified
	if !user.EmailVerified {
		writeJSON(w, http.StatusForbidden, loginResponse{
			Error:                 "Please verify your email address before logging in",
			EmailNotVerified:      true,
			CanResendVerification: true,
		})
		return
	}

	// Check 2FA
	if user.TOTPEnabled {
		if req.TOTPCode == "" {
			writeJSON(w, http.StatusUnauthorized, loginResponse{
				Error:        "2FA code required",
				TOTPRequired: true,
			})
			return
		}
		if user.TOTPSecret == nil {
			slog.Error("login: TOTP enabled but no secret stored",
				"user_id", user.ID,
				"request_id", logging.GetRequestID(r.Context()))
			writeJSON(w, http.StatusInternalServerError, loginResponse{Error: "internal error"})
			return
		}
		valid, err := auth.ValidateTOTP(*user.TOTPSecret, req.TOTPCode)
		if err != nil {
			slog.Error("login: TOTP validation error",
				"error", err,
				"request_id", logging.GetRequestID(r.Context()))
			writeJSON(w, http.StatusInternalServerError, loginResponse{Error: "internal error"})
			return
		}
		if !valid {
			writeJSON(w, http.StatusUnauthorized, loginResponse{Error: "invalid 2FA code"})
			return
		}
	}

	// Clear failed attempts on successful login
	if err := h.DB.ClearLoginAttempts(email); err != nil {
		slog.Error("login: failed to clear login attempts",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
	}

	// Create session
	sessionID, err := auth.GenerateID()
	if err != nil {
		slog.Error("login: failed to generate session ID",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, loginResponse{Error: "internal error"})
		return
	}

	sessionToken, err := auth.GenerateToken(32)
	if err != nil {
		slog.Error("login: failed to generate session token",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, loginResponse{Error: "internal error"})
		return
	}

	now := time.Now().UTC()
	session := &db.Session{
		ID:        sessionID,
		UserID:    user.ID,
		Token:     sessionToken,
		ExpiresAt: now.Add(auth.SessionDuration),
		CreatedAt: now,
	}

	if err := h.DB.CreateSession(session); err != nil {
		slog.Error("login: failed to create session",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, loginResponse{Error: "internal error"})
		return
	}

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    sessionToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isHTTPSOnly(),
		MaxAge:   int(auth.SessionDuration.Seconds()),
	})

	writeJSON(w, http.StatusOK, loginResponse{
		User: &loginUserSummary{
			ID:          user.ID,
			Email:       user.Email,
			TOTPEnabled: user.TOTPEnabled,
		},
		Token: sessionToken,
	})
}

// HandleLogout handles POST /api/auth/logout.
func (h *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionToken := extractSessionToken(r)
	if sessionToken == "" {
		writeJSON(w, http.StatusUnauthorized, logoutResponse{Error: "not authenticated"})
		return
	}

	session, err := h.DB.GetSessionByToken(sessionToken)
	if err != nil {
		slog.Error("logout: failed to look up session",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, logoutResponse{Error: "internal error"})
		return
	}

	if session == nil {
		writeJSON(w, http.StatusUnauthorized, logoutResponse{Error: "not authenticated"})
		return
	}

	// Delete session from DB
	if err := h.DB.DeleteSession(session.ID); err != nil {
		slog.Error("logout: failed to delete session",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, logoutResponse{Error: "internal error"})
		return
	}

	// Clear session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isHTTPSOnly(),
		MaxAge:   -1,
	})

	writeJSON(w, http.StatusOK, logoutResponse{Message: "Logged out"})
}

// HandleMe handles GET /api/auth/me.
func (h *AuthHandler) HandleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionToken := extractSessionToken(r)
	if sessionToken == "" {
		writeJSON(w, http.StatusUnauthorized, meResponse{Error: "not authenticated"})
		return
	}

	session, err := h.DB.GetSessionByToken(sessionToken)
	if err != nil {
		slog.Error("me: failed to look up session",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, meResponse{Error: "internal error"})
		return
	}

	if session == nil || time.Now().UTC().After(session.ExpiresAt) {
		if session != nil {
			// Clean up expired session
			if err := h.DB.DeleteSession(session.ID); err != nil {
				slog.Error("me: failed to delete expired session",
					"error", err,
					"request_id", logging.GetRequestID(r.Context()))
			}
		}
		writeJSON(w, http.StatusUnauthorized, meResponse{Error: "not authenticated"})
		return
	}

	user, err := h.DB.GetUserByID(session.UserID)
	if err != nil {
		slog.Error("me: failed to look up user",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, meResponse{Error: "internal error"})
		return
	}

	if user == nil {
		writeJSON(w, http.StatusUnauthorized, meResponse{Error: "not authenticated"})
		return
	}

	writeJSON(w, http.StatusOK, meResponse{
		User: &meUserSummary{
			ID:            user.ID,
			Email:         user.Email,
			TOTPEnabled:   user.TOTPEnabled,
			EmailVerified: user.EmailVerified,
			CreatedAt:     user.CreatedAt.Format(time.RFC3339),
		},
	})
}

// extractSessionToken extracts the session token from the request.
// It checks the Authorization header first, then the session cookie.
func extractSessionToken(r *http.Request) string {
	// Check Authorization header
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}

	// Check session cookie
	cookie, err := r.Cookie("session")
	if err != nil {
		return ""
	}
	return cookie.Value
}

// isHTTPSOnly returns true if the HTTPS_ONLY env var is set to "true".
func isHTTPSOnly() bool {
	return strings.EqualFold(os.Getenv("HTTPS_ONLY"), "true")
}

// HandleResendVerification handles POST /api/auth/resend-verification.
func (h *AuthHandler) HandleResendVerification(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxAuthBodySize)

	var req resendVerificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if err.Error() == "http: request body too large" {
			writeJSON(w, http.StatusRequestEntityTooLarge, resendVerificationResponse{Error: "request body too large"})
			return
		}
		writeJSON(w, http.StatusBadRequest, resendVerificationResponse{Error: "invalid JSON body"})
		return
	}

	// Always return success to prevent email enumeration
	successMsg := "If an unverified account exists with that email, a verification link has been sent."

	email := auth.NormalizeEmail(req.Email)
	if err := auth.ValidateEmail(email); err != nil {
		// Still return success to prevent enumeration
		writeJSON(w, http.StatusOK, resendVerificationResponse{Message: successMsg})
		return
	}

	user, err := h.DB.GetUserByEmail(email)
	if err != nil {
		slog.Error("resend-verification: failed to look up user",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		// Still return success to prevent enumeration
		writeJSON(w, http.StatusOK, resendVerificationResponse{Message: successMsg})
		return
	}

	// If user doesn't exist or is already verified, return success anyway
	if user == nil || user.EmailVerified {
		writeJSON(w, http.StatusOK, resendVerificationResponse{Message: successMsg})
		return
	}

	// Delete existing unused verification tokens
	if err := h.DB.DeleteUnusedEmailVerificationTokens(user.ID); err != nil {
		slog.Error("resend-verification: failed to delete old tokens",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
	}

	// Generate new token
	token, err := auth.GenerateToken(32)
	if err != nil {
		slog.Error("resend-verification: failed to generate token",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusOK, resendVerificationResponse{Message: successMsg})
		return
	}

	tokenHash := auth.HashToken(token)
	tokenID, err := auth.GenerateID()
	if err != nil {
		slog.Error("resend-verification: failed to generate token ID",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusOK, resendVerificationResponse{Message: successMsg})
		return
	}

	expiresAt := time.Now().UTC().Add(auth.EmailVerificationTokenExpiry)
	if err := h.DB.StoreEmailVerificationToken(tokenID, user.ID, tokenHash, expiresAt); err != nil {
		slog.Error("resend-verification: failed to store token",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusOK, resendVerificationResponse{Message: successMsg})
		return
	}

	if err := h.EmailSender.SendVerificationEmail(email, token); err != nil {
		slog.Error("resend-verification: failed to send email",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
	}

	writeJSON(w, http.StatusOK, resendVerificationResponse{Message: successMsg})
}
