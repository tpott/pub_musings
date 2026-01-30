package main

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/tpott/subtitler/backend/auth"
	"github.com/tpott/subtitler/backend/email"
	"github.com/tpott/subtitler/backend/httputil"
	"github.com/tpott/subtitler/backend/logging"
	"github.com/tpott/subtitler/backend/ratelimit"
	"github.com/tpott/subtitler/backend/security"
)

// registerAuthRecoveryHandlers registers password reset, magic link, and email verification handlers.
func registerAuthRecoveryHandlers(mux *http.ServeMux) { //nolint:funlen // route registration
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
