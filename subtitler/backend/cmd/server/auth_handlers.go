package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/trevor/subtitler/internal/auth"
	"github.com/trevor/subtitler/internal/db"
)

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message,omitempty"`
	Error   string          `json:"error,omitempty"`
	User    *AuthUserResult `json:"user,omitempty"`
}

type AuthUserResult struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
}

func handleRegister(database *db.DB, jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if enableCORS(w, r) {
			return
		}

		if r.Method != http.MethodPost {
			writeAuthError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		// Parse request body
		var req RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAuthError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		// Validate input
		if req.Email == "" || req.Password == "" {
			writeAuthError(w, http.StatusBadRequest, "Email and password are required")
			return
		}

		// Basic email validation
		if !strings.Contains(req.Email, "@") {
			writeAuthError(w, http.StatusBadRequest, "Invalid email address")
			return
		}

		// Validate password length
		if len(req.Password) < 8 {
			writeAuthError(w, http.StatusBadRequest, "Password must be at least 8 characters")
			return
		}

		// Hash password
		passwordHash, err := auth.HashPassword(req.Password)
		if err != nil {
			writeAuthError(w, http.StatusInternalServerError, "Failed to process password")
			return
		}

		// Create user in database
		user, err := database.CreateUser(req.Email, passwordHash)
		if err != nil {
			if err == db.ErrDuplicateEmail {
				writeAuthError(w, http.StatusConflict, "Email already exists")
				return
			}
			writeAuthError(w, http.StatusInternalServerError, "Failed to create user")
			return
		}

		// Generate JWT token
		token, err := auth.GenerateJWT(user.ID, user.Email, jwtSecret)
		if err != nil {
			writeAuthError(w, http.StatusInternalServerError, "Failed to generate token")
			return
		}

		// Set cookie
		setAuthCookie(w, token)

		// Return success response
		response := AuthResponse{
			Success: true,
			Message: "User registered successfully",
			User: &AuthUserResult{
				ID:    user.ID,
				Email: user.Email,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(response)
	}
}

func handleLogin(database *db.DB, jwtSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if enableCORS(w, r) {
			return
		}

		if r.Method != http.MethodPost {
			writeAuthError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		// Parse request body
		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAuthError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		// Validate input
		if req.Email == "" || req.Password == "" {
			writeAuthError(w, http.StatusBadRequest, "Email and password are required")
			return
		}

		// Get user from database
		user, err := database.GetUserByEmail(req.Email)
		if err != nil {
			if err == db.ErrUserNotFound {
				writeAuthError(w, http.StatusUnauthorized, "Invalid email or password")
				return
			}
			writeAuthError(w, http.StatusInternalServerError, "Login failed")
			return
		}

		// Verify password
		if err := auth.VerifyPassword(user.PasswordHash, req.Password); err != nil {
			writeAuthError(w, http.StatusUnauthorized, "Invalid email or password")
			return
		}

		// Generate JWT token
		token, err := auth.GenerateJWT(user.ID, user.Email, jwtSecret)
		if err != nil {
			writeAuthError(w, http.StatusInternalServerError, "Failed to generate token")
			return
		}

		// Set cookie
		setAuthCookie(w, token)

		// Return success response
		response := AuthResponse{
			Success: true,
			Message: "Login successful",
			User: &AuthUserResult{
				ID:    user.ID,
				Email: user.Email,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	if enableCORS(w, r) {
		return
	}

	if r.Method != http.MethodPost {
		writeAuthError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Clear cookie
	clearAuthCookie(w)

	response := AuthResponse{
		Success: true,
		Message: "Logged out successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleMe(w http.ResponseWriter, r *http.Request) {
	if enableCORS(w, r) {
		return
	}

	if r.Method != http.MethodGet {
		writeAuthError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Get claims from context (set by middleware)
	claims, ok := auth.GetClaims(r)
	if !ok {
		writeAuthError(w, http.StatusUnauthorized, "Authentication required")
		return
	}

	response := AuthResponse{
		Success: true,
		User: &AuthUserResult{
			ID:    claims.UserID,
			Email: claims.Email,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func setAuthCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(auth.TokenExpiry),
	})
}

func clearAuthCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     auth.CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

func writeAuthError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(AuthResponse{
		Success: false,
		Error:   message,
	})
}
