package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/trevor/subtitler/backend/db"
)

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

// ValidatePassword performs basic password validation
func ValidatePassword(password string) error {
	if len(password) < MinPasswordLength {
		return fmt.Errorf("password must be at least %d characters", MinPasswordLength)
	}
	if len(password) > 72 { // bcrypt max
		return fmt.Errorf("password is too long (max 72 characters)")
	}
	return nil
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

// SetSessionCookie sets the session cookie on the response
func SetSessionCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		// Secure should be true in production with HTTPS
		// Secure: true,
	})
}

// ClearSessionCookie clears the session cookie
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
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
		database.DeleteSession(token)
		return nil, nil, nil
	}

	user, err := database.GetUserByID(session.UserID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		// Session exists but user doesn't - clean up
		database.DeleteSession(token)
		return nil, nil, nil
	}

	return user, session, nil
}
