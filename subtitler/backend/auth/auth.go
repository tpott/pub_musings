package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode"

	"golang.org/x/crypto/bcrypt"

	"github.com/trevor/subtitler/backend/db"
	"github.com/trevor/subtitler/backend/logging"
)

// IsHTTPSOnly returns true if HTTPS_ONLY env var is set to a truthy value.
// When true, session cookies will have the Secure flag set.
func IsHTTPSOnly() bool {
	val := os.Getenv("HTTPS_ONLY")
	return val == "1" || strings.ToLower(val) == "true"
}

const (
	// SessionDuration is how long a session is valid
	SessionDuration = 7 * 24 * time.Hour // 7 days

	// TokenLength is the length of session tokens in bytes (32 bytes = 64 hex chars)
	TokenLength = 32

	// MinPasswordLength is the minimum password length
	MinPasswordLength = 8

	// BcryptCost is the bcrypt cost factor
	BcryptCost = 12
)

// HashPassword creates a bcrypt hash of the password
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hash), nil
}

// CheckPassword compares a password with a bcrypt hash
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateToken generates a cryptographically secure random token
func GenerateToken() (string, error) {
	bytes := make([]byte, TokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateID generates a random ID for users/sessions
func GenerateID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate ID: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// ValidateEmail performs basic email validation
func ValidateEmail(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return fmt.Errorf("email is required")
	}
	if !strings.Contains(email, "@") || !strings.Contains(email, ".") {
		return fmt.Errorf("invalid email format")
	}
	if len(email) > 255 {
		return fmt.Errorf("email is too long")
	}
	return nil
}

// ValidatePassword validates password strength including complexity requirements.
// Password must be at least 8 characters and contain:
// - At least one uppercase letter
// - At least one lowercase letter
// - At least one number
// - At least one special character
func ValidatePassword(password string) error {
	if len(password) < MinPasswordLength {
		return fmt.Errorf("password must be at least %d characters", MinPasswordLength)
	}
	if len(password) > 72 { // bcrypt max
		return fmt.Errorf("password is too long (max 72 characters)")
	}

	var hasUpper, hasLower, hasNumber, hasSpecial bool
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasNumber = true
		case isSpecialChar(r):
			hasSpecial = true
		}
	}

	if !hasUpper {
		return fmt.Errorf("password must contain at least one uppercase letter")
	}
	if !hasLower {
		return fmt.Errorf("password must contain at least one lowercase letter")
	}
	if !hasNumber {
		return fmt.Errorf("password must contain at least one number")
	}
	if !hasSpecial {
		return fmt.Errorf("password must contain at least one special character")
	}

	return nil
}

// isSpecialChar checks if a character is a special character
func isSpecialChar(r rune) bool {
	specialChars := "!@#$%^&*()_+-=[]{}|;:'\",.<>?/`~\\"
	return strings.ContainsRune(specialChars, r)
}

// CreateSession creates a new session for a user with optional IP address and user agent
func CreateSession(database *db.DB, userID, ipAddress, userAgent string) (*db.Session, error) {
	id, err := GenerateID()
	if err != nil {
		return nil, err
	}

	token, err := GenerateToken()
	if err != nil {
		return nil, err
	}

	session := &db.Session{
		ID:        id,
		UserID:    userID,
		Token:     token,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		ExpiresAt: time.Now().Add(SessionDuration),
		CreatedAt: time.Now(),
	}

	if err := database.CreateSession(session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return session, nil
}

// GetTokenFromRequest extracts the session token from the request
// Checks Authorization header (Bearer token) and cookies
func GetTokenFromRequest(r *http.Request) string {
	// Check Authorization header first
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}

	// Check cookie
	cookie, err := r.Cookie("session")
	if err == nil {
		return cookie.Value
	}

	return ""
}

// SetSessionCookie sets the session cookie on the response.
// The Secure flag is set when HTTPS_ONLY env var is set to "1" or "true".
func SetSessionCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   IsHTTPSOnly(),
	})
}

// ClearSessionCookie clears the session cookie.
// The Secure flag is set when HTTPS_ONLY env var is set to "1" or "true".
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   IsHTTPSOnly(),
	})
}

// ValidateSession validates a session token and returns the user if valid
func ValidateSession(database *db.DB, token string) (*db.User, *db.Session, error) {
	if token == "" {
		return nil, nil, nil
	}

	session, err := database.GetSessionByToken(token)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get session: %w", err)
	}
	if session == nil {
		return nil, nil, nil
	}

	// Check if expired
	if time.Now().After(session.ExpiresAt) {
		// Clean up expired session
		if err := database.DeleteSession(token); err != nil {
			logging.Warn("Failed to delete expired session during cleanup",
				"session_id", session.ID,
				"user_id", session.UserID,
				"error", err)
		}
		return nil, nil, nil
	}

	user, err := database.GetUserByID(session.UserID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		// Session exists but user doesn't - clean up orphaned session
		if err := database.DeleteSession(token); err != nil {
			logging.Warn("Failed to delete orphaned session during cleanup",
				"session_id", session.ID,
				"user_id", session.UserID,
				"error", err)
		}
		return nil, nil, nil
	}

	return user, session, nil
}
