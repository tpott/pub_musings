package main

import (
	"net/http"

	"github.com/tpott/subtitler/backend/auth"
	"github.com/tpott/subtitler/backend/httputil"
	"github.com/tpott/subtitler/backend/logging"
	"github.com/tpott/subtitler/backend/ratelimit"
	"github.com/tpott/subtitler/backend/security"
	"github.com/tpott/subtitler/backend/totp"
	"github.com/tpott/subtitler/backend/validation"
)

// registerAuthTOTPHandlers registers 2FA/TOTP-related authentication handlers.
func registerAuthTOTPHandlers(mux *http.ServeMux) { //nolint:funlen // route registration
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
}
