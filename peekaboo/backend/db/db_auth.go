package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrTokenAlreadyUsed is returned when a token has already been consumed,
// typically by a concurrent request (TOCTOU prevention).
var ErrTokenAlreadyUsed = errors.New("token already used")

// authSchema defines the authentication-related database tables and indexes.
const authSchema = `
CREATE TABLE IF NOT EXISTS users (
	id TEXT PRIMARY KEY,
	email TEXT UNIQUE NOT NULL,
	password_hash TEXT NOT NULL,
	totp_secret TEXT,
	totp_enabled INTEGER NOT NULL DEFAULT 0,
	email_verified INTEGER NOT NULL DEFAULT 0,
	verified_at DATETIME,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

CREATE TABLE IF NOT EXISTS email_verification_tokens (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL REFERENCES users(id),
	token_hash TEXT NOT NULL,
	expires_at DATETIME NOT NULL,
	used INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_email_verification_user_id ON email_verification_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_email_verification_expires ON email_verification_tokens(expires_at);
CREATE INDEX IF NOT EXISTS idx_email_verification_token_hash ON email_verification_tokens(token_hash);

CREATE TABLE IF NOT EXISTS magic_link_tokens (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL REFERENCES users(id),
	token_hash TEXT NOT NULL,
	expires_at DATETIME NOT NULL,
	used INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_magic_link_user_id ON magic_link_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_magic_link_expires ON magic_link_tokens(expires_at);
CREATE INDEX IF NOT EXISTS idx_magic_link_token_hash ON magic_link_tokens(token_hash);

CREATE TABLE IF NOT EXISTS sessions (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL REFERENCES users(id),
	token_hash TEXT UNIQUE NOT NULL,
	expires_at DATETIME NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_sessions_token_hash ON sessions(token_hash);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);

CREATE TABLE IF NOT EXISTS login_attempts (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	email TEXT NOT NULL,
	ip_address TEXT NOT NULL,
	success INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_login_attempts_email ON login_attempts(email, created_at);
`

// User represents a registered user.
type User struct {
	ID            string
	Email         string
	PasswordHash  string
	TOTPSecret    *string
	TOTPEnabled   bool
	EmailVerified bool
	VerifiedAt    *time.Time
	CreatedAt     time.Time
}

// Session represents an authenticated session.
type Session struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// LoginAttempt represents a login attempt record.
type LoginAttempt struct {
	ID        int64
	Email     string
	IPAddress string
	Success   bool
	CreatedAt time.Time
}

// InitAuth creates the authentication tables and indexes.
// Should be called after Init().
func (db *DB) InitAuth() error {
	// Migrate: rename sessions.token → sessions.token_hash (for existing DBs)
	// BEFORE running authSchema, because authSchema creates an index on
	// sessions(token_hash) which fails if the column is still named "token".
	if _, err := db.conn.Exec("ALTER TABLE sessions RENAME COLUMN token TO token_hash"); err != nil {
		if !strings.Contains(err.Error(), "no such column") &&
			!strings.Contains(err.Error(), "no such table") {
			return fmt.Errorf("migrate sessions.token_hash: %w", err)
		}
	}

	// Also drop the old index name so the new one can be created.
	if _, err := db.conn.Exec("DROP INDEX IF EXISTS idx_sessions_token"); err != nil {
		return fmt.Errorf("drop old sessions token index: %w", err)
	}

	if _, err := db.conn.Exec(authSchema); err != nil {
		return fmt.Errorf("create auth schema: %w", err)
	}

	return nil
}

// CreateUser inserts a new user into the database.
func (db *DB) CreateUser(user *User) error {
	_, err := db.conn.Exec(`
		INSERT INTO users (id, email, password_hash, totp_secret, totp_enabled, email_verified, verified_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, user.ID, user.Email, user.PasswordHash, user.TOTPSecret, user.TOTPEnabled, user.EmailVerified, user.VerifiedAt, user.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}
	return nil
}

// GetUserByEmail retrieves a user by email address. Returns nil if not found.
func (db *DB) GetUserByEmail(email string) (*User, error) {
	var u User
	var totpSecret sql.NullString
	var verifiedAt sql.NullTime
	var totpEnabled, emailVerified int

	err := db.conn.QueryRow(`
		SELECT id, email, password_hash, totp_secret, totp_enabled, email_verified, verified_at, created_at
		FROM users WHERE email = ?
	`, email).Scan(&u.ID, &u.Email, &u.PasswordHash, &totpSecret, &totpEnabled, &emailVerified, &verifiedAt, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query user by email: %w", err)
	}

	if totpSecret.Valid {
		u.TOTPSecret = &totpSecret.String
	}
	u.TOTPEnabled = totpEnabled != 0
	u.EmailVerified = emailVerified != 0
	if verifiedAt.Valid {
		u.VerifiedAt = &verifiedAt.Time
	}
	return &u, nil
}

// GetUserByID retrieves a user by ID. Returns nil if not found.
func (db *DB) GetUserByID(id string) (*User, error) {
	var u User
	var totpSecret sql.NullString
	var verifiedAt sql.NullTime
	var totpEnabled, emailVerified int

	err := db.conn.QueryRow(`
		SELECT id, email, password_hash, totp_secret, totp_enabled, email_verified, verified_at, created_at
		FROM users WHERE id = ?
	`, id).Scan(&u.ID, &u.Email, &u.PasswordHash, &totpSecret, &totpEnabled, &emailVerified, &verifiedAt, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query user by id: %w", err)
	}

	if totpSecret.Valid {
		u.TOTPSecret = &totpSecret.String
	}
	u.TOTPEnabled = totpEnabled != 0
	u.EmailVerified = emailVerified != 0
	if verifiedAt.Valid {
		u.VerifiedAt = &verifiedAt.Time
	}
	return &u, nil
}

// SetEmailVerified marks a user's email as verified.
func (db *DB) SetEmailVerified(userID string) error {
	result, err := db.conn.Exec(`
		UPDATE users SET email_verified = 1, verified_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, userID)
	if err != nil {
		return fmt.Errorf("set email verified: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("user not found: %s", userID)
	}
	return nil
}

// CreateSession inserts a new session into the database.
// The caller must pass a pre-hashed token in session.TokenHash.
func (db *DB) CreateSession(session *Session) error {
	_, err := db.conn.Exec(`
		INSERT INTO sessions (id, user_id, token_hash, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, session.ID, session.UserID, session.TokenHash, session.ExpiresAt, session.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

// GetSessionByTokenHash retrieves a session by its token hash. Returns nil if not found.
// Callers should pass auth.HashToken(plaintextToken).
func (db *DB) GetSessionByTokenHash(tokenHash string) (*Session, error) {
	var s Session
	err := db.conn.QueryRow(`
		SELECT id, user_id, token_hash, expires_at, created_at
		FROM sessions WHERE token_hash = ?
	`, tokenHash).Scan(&s.ID, &s.UserID, &s.TokenHash, &s.ExpiresAt, &s.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query session by token hash: %w", err)
	}
	return &s, nil
}

// DeleteSession removes a session by its ID.
func (db *DB) DeleteSession(sessionID string) error {
	_, err := db.conn.Exec("DELETE FROM sessions WHERE id = ?", sessionID)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// DeleteExpiredSessions removes all sessions that have expired.
func (db *DB) DeleteExpiredSessions() (int64, error) {
	result, err := db.conn.Exec("DELETE FROM sessions WHERE expires_at < CURRENT_TIMESTAMP")
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}
	return result.RowsAffected()
}

// StoreEmailVerificationToken stores a hashed email verification token.
func (db *DB) StoreEmailVerificationToken(id, userID, tokenHash string, expiresAt time.Time) error {
	_, err := db.conn.Exec(`
		INSERT INTO email_verification_tokens (id, user_id, token_hash, expires_at)
		VALUES (?, ?, ?, ?)
	`, id, userID, tokenHash, expiresAt)
	if err != nil {
		return fmt.Errorf("store email verification token: %w", err)
	}
	return nil
}

// GetEmailVerificationToken retrieves a verification token by its hash.
// Returns the token ID, user ID, expiry, and used status. Returns empty strings if not found.
func (db *DB) GetEmailVerificationToken(tokenHash string) (id, userID string, expiresAt time.Time, used bool, err error) {
	var usedInt int
	e := db.conn.QueryRow(`
		SELECT id, user_id, expires_at, used
		FROM email_verification_tokens WHERE token_hash = ?
	`, tokenHash).Scan(&id, &userID, &expiresAt, &usedInt)
	if e == sql.ErrNoRows {
		return "", "", time.Time{}, false, nil
	}
	if e != nil {
		return "", "", time.Time{}, false, fmt.Errorf("query email verification token: %w", e)
	}
	used = usedInt != 0
	return id, userID, expiresAt, used, nil
}

// MarkEmailVerificationTokenUsed atomically marks a verification token as used.
// Returns ErrTokenAlreadyUsed if the token was already consumed by a concurrent request.
func (db *DB) MarkEmailVerificationTokenUsed(id string) error {
	result, err := db.conn.Exec("UPDATE email_verification_tokens SET used = 1 WHERE id = ? AND used = 0", id)
	if err != nil {
		return fmt.Errorf("mark verification token used: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("mark verification token used: rows affected: %w", err)
	}
	if n == 0 {
		return ErrTokenAlreadyUsed
	}
	return nil
}

// DeleteUnusedEmailVerificationTokens removes unused verification tokens for a user.
func (db *DB) DeleteUnusedEmailVerificationTokens(userID string) error {
	_, err := db.conn.Exec("DELETE FROM email_verification_tokens WHERE user_id = ? AND used = 0", userID)
	if err != nil {
		return fmt.Errorf("delete unused verification tokens: %w", err)
	}
	return nil
}

// StoreMagicLinkToken stores a hashed magic link token.
func (db *DB) StoreMagicLinkToken(id, userID, tokenHash string, expiresAt time.Time) error {
	_, err := db.conn.Exec(`
		INSERT INTO magic_link_tokens (id, user_id, token_hash, expires_at)
		VALUES (?, ?, ?, ?)
	`, id, userID, tokenHash, expiresAt)
	if err != nil {
		return fmt.Errorf("store magic link token: %w", err)
	}
	return nil
}

// GetMagicLinkToken retrieves a magic link token by its hash.
func (db *DB) GetMagicLinkToken(tokenHash string) (id, userID string, expiresAt time.Time, used bool, err error) {
	var usedInt int
	e := db.conn.QueryRow(`
		SELECT id, user_id, expires_at, used
		FROM magic_link_tokens WHERE token_hash = ?
	`, tokenHash).Scan(&id, &userID, &expiresAt, &usedInt)
	if e == sql.ErrNoRows {
		return "", "", time.Time{}, false, nil
	}
	if e != nil {
		return "", "", time.Time{}, false, fmt.Errorf("query magic link token: %w", e)
	}
	used = usedInt != 0
	return id, userID, expiresAt, used, nil
}

// MarkMagicLinkTokenUsed atomically marks a magic link token as used.
// Returns ErrTokenAlreadyUsed if the token was already consumed by a concurrent request.
func (db *DB) MarkMagicLinkTokenUsed(id string) error {
	result, err := db.conn.Exec("UPDATE magic_link_tokens SET used = 1 WHERE id = ? AND used = 0", id)
	if err != nil {
		return fmt.Errorf("mark magic link token used: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("mark magic link token used: rows affected: %w", err)
	}
	if n == 0 {
		return ErrTokenAlreadyUsed
	}
	return nil
}

// DeleteUnusedMagicLinkTokens removes unused magic link tokens for a user.
func (db *DB) DeleteUnusedMagicLinkTokens(userID string) error {
	_, err := db.conn.Exec("DELETE FROM magic_link_tokens WHERE user_id = ? AND used = 0", userID)
	if err != nil {
		return fmt.Errorf("delete unused magic link tokens: %w", err)
	}
	return nil
}

// RecordLoginAttempt records a login attempt (success or failure).
func (db *DB) RecordLoginAttempt(email, ipAddress string, success bool) error {
	successInt := 0
	if success {
		successInt = 1
	}
	_, err := db.conn.Exec(`
		INSERT INTO login_attempts (email, ip_address, success)
		VALUES (?, ?, ?)
	`, email, ipAddress, successInt)
	if err != nil {
		return fmt.Errorf("record login attempt: %w", err)
	}
	return nil
}

// CountRecentFailedAttempts counts failed login attempts for an email within a time window.
func (db *DB) CountRecentFailedAttempts(email string, since time.Time) (int, error) {
	var count int
	err := db.conn.QueryRow(`
		SELECT COUNT(*) FROM login_attempts
		WHERE email = ? AND success = 0 AND created_at >= ?
	`, email, since).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count failed attempts: %w", err)
	}
	return count, nil
}

// ClearLoginAttempts removes failed login attempts for an email.
// Called after successful login to reset the lockout counter.
func (db *DB) ClearLoginAttempts(email string) error {
	_, err := db.conn.Exec("DELETE FROM login_attempts WHERE email = ? AND success = 0", email)
	if err != nil {
		return fmt.Errorf("clear login attempts: %w", err)
	}
	return nil
}

// DeleteExpiredLoginAttempts removes login attempts older than the given time.
func (db *DB) DeleteExpiredLoginAttempts(before time.Time) (int64, error) {
	result, err := db.conn.Exec("DELETE FROM login_attempts WHERE created_at < ?", before)
	if err != nil {
		return 0, fmt.Errorf("delete expired login attempts: %w", err)
	}
	return result.RowsAffected()
}

// DeleteExpiredEmailVerificationTokens removes verification tokens past their expires_at.
func (db *DB) DeleteExpiredEmailVerificationTokens() (int64, error) {
	result, err := db.conn.Exec("DELETE FROM email_verification_tokens WHERE expires_at < CURRENT_TIMESTAMP")
	if err != nil {
		return 0, fmt.Errorf("delete expired email verification tokens: %w", err)
	}
	return result.RowsAffected()
}

// DeleteExpiredMagicLinkTokens removes magic link tokens past their expires_at.
func (db *DB) DeleteExpiredMagicLinkTokens() (int64, error) {
	result, err := db.conn.Exec("DELETE FROM magic_link_tokens WHERE expires_at < CURRENT_TIMESTAMP")
	if err != nil {
		return 0, fmt.Errorf("delete expired magic link tokens: %w", err)
	}
	return result.RowsAffected()
}

// DeleteSessionsByUserID removes all sessions for a given user.
func (db *DB) DeleteSessionsByUserID(userID string) error {
	_, err := db.conn.Exec("DELETE FROM sessions WHERE user_id = ?", userID)
	if err != nil {
		return fmt.Errorf("delete sessions by user id: %w", err)
	}
	return nil
}

// SetTOTPSecret sets the TOTP secret for a user (setup phase, not yet enabled).
func (db *DB) SetTOTPSecret(userID, secret string) error {
	result, err := db.conn.Exec(`
		UPDATE users SET totp_secret = ? WHERE id = ?
	`, secret, userID)
	if err != nil {
		return fmt.Errorf("set totp secret: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("user not found: %s", userID)
	}
	return nil
}

// EnableTOTP sets totp_enabled=1 for a user. The secret must already be set.
func (db *DB) EnableTOTP(userID string) error {
	result, err := db.conn.Exec(`
		UPDATE users SET totp_enabled = 1 WHERE id = ? AND totp_secret IS NOT NULL
	`, userID)
	if err != nil {
		return fmt.Errorf("enable totp: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("user not found or TOTP secret not set: %s", userID)
	}
	return nil
}

// DisableTOTP clears the TOTP secret and sets totp_enabled=0.
func (db *DB) DisableTOTP(userID string) error {
	result, err := db.conn.Exec(`
		UPDATE users SET totp_secret = NULL, totp_enabled = 0 WHERE id = ?
	`, userID)
	if err != nil {
		return fmt.Errorf("disable totp: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("user not found: %s", userID)
	}
	return nil
}
