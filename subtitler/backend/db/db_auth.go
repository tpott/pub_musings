package db

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"time"
)

// SetTOTPSecret sets the TOTP secret for a user (during 2FA setup)
func (db *DB) SetTOTPSecret(userID, secret string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `
		UPDATE users SET totp_secret = ? WHERE id = ?
	`, secret, userID)
	if err != nil {
		return fmt.Errorf("failed to set TOTP secret: %w", err)
	}
	return nil
}

// EnableTOTP enables 2FA for a user (after verification)
func (db *DB) EnableTOTP(userID string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `
		UPDATE users SET totp_enabled = 1 WHERE id = ?
	`, userID)
	if err != nil {
		return fmt.Errorf("failed to enable TOTP: %w", err)
	}
	return nil
}

// DisableTOTP disables 2FA and clears the secret for a user
func (db *DB) DisableTOTP(userID string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `
		UPDATE users SET totp_enabled = 0, totp_secret = NULL WHERE id = ?
	`, userID)
	if err != nil {
		return fmt.Errorf("failed to disable TOTP: %w", err)
	}
	return nil
}

// SaveRecoveryCodes stores hashed recovery codes for a user.
// Deletes any existing unused codes first.
func (db *DB) SaveRecoveryCodes(userID string, codeHashes []string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	// Delete existing unused codes
	_, err := db.conn.ExecContext(ctx, `DELETE FROM recovery_codes WHERE user_id = ? AND used = 0`, userID)
	if err != nil {
		return fmt.Errorf("failed to delete old recovery codes: %w", err)
	}

	// Insert new codes
	for _, hash := range codeHashes {
		id, err := generateID()
		if err != nil {
			return fmt.Errorf("failed to generate recovery code ID: %w", err)
		}
		_, err = db.conn.ExecContext(ctx, `
			INSERT INTO recovery_codes (id, user_id, code_hash, created_at)
			VALUES (?, ?, ?, ?)
		`, id, userID, hash, time.Now())
		if err != nil {
			return fmt.Errorf("failed to insert recovery code: %w", err)
		}
	}

	return nil
}

// generateID generates a random 16-character hex ID
func generateID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate ID: %w", err)
	}
	return fmt.Sprintf("%x", b), nil
}

// GetUnusedRecoveryCodes returns all unused recovery codes for a user
func (db *DB) GetUnusedRecoveryCodes(userID string) ([]RecoveryCode, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	rows, err := db.conn.QueryContext(ctx, `
		SELECT id, user_id, code_hash, used, created_at, used_at
		FROM recovery_codes
		WHERE user_id = ? AND used = 0
		ORDER BY created_at
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var codes []RecoveryCode
	for rows.Next() {
		var c RecoveryCode
		var usedAt sql.NullTime
		if err := rows.Scan(&c.ID, &c.UserID, &c.CodeHash, &c.Used, &c.CreatedAt, &usedAt); err != nil {
			return nil, err
		}
		if usedAt.Valid {
			c.UsedAt = &usedAt.Time
		}
		codes = append(codes, c)
	}

	return codes, rows.Err()
}

// UseRecoveryCode marks a recovery code as used.
// Returns true if successful, false if code not found or already used.
func (db *DB) UseRecoveryCode(codeID string) (bool, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	now := time.Now()
	result, err := db.conn.ExecContext(ctx, `
		UPDATE recovery_codes
		SET used = 1, used_at = ?
		WHERE id = ? AND used = 0
	`, now, codeID)
	if err != nil {
		return false, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// DeleteRecoveryCodes deletes all recovery codes for a user
func (db *DB) DeleteRecoveryCodes(userID string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `DELETE FROM recovery_codes WHERE user_id = ?`, userID)
	return err
}

// CountUnusedRecoveryCodes returns the number of unused recovery codes for a user
func (db *DB) CountUnusedRecoveryCodes(userID string) (int, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	var count int
	err := db.conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM recovery_codes WHERE user_id = ? AND used = 0
	`, userID).Scan(&count)
	return count, err
}

// CreatePasswordResetToken creates a new password reset token for a user.
// Deletes any existing unused tokens for the user first.
func (db *DB) CreatePasswordResetToken(userID, tokenHash string, expiresAt time.Time) (*PasswordResetToken, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	// Delete existing unused tokens for this user
	_, err := db.conn.ExecContext(ctx, `DELETE FROM password_reset_tokens WHERE user_id = ? AND used = 0`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete old reset tokens: %w", err)
	}

	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token ID: %w", err)
	}
	token := &PasswordResetToken{
		ID:        id,
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
		Used:      false,
		CreatedAt: time.Now(),
	}

	_, err = db.conn.ExecContext(ctx, `
		INSERT INTO password_reset_tokens (id, user_id, token_hash, expires_at, used, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, token.ID, token.UserID, token.TokenHash, token.ExpiresAt, 0, token.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create reset token: %w", err)
	}

	return token, nil
}

// GetPasswordResetToken retrieves a password reset token by its hash.
// Returns nil if not found.
func (db *DB) GetPasswordResetToken(tokenHash string) (*PasswordResetToken, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	token := &PasswordResetToken{}
	err := db.conn.QueryRowContext(ctx, `
		SELECT id, user_id, token_hash, expires_at, used, created_at
		FROM password_reset_tokens
		WHERE token_hash = ?
	`, tokenHash).Scan(&token.ID, &token.UserID, &token.TokenHash, &token.ExpiresAt, &token.Used, &token.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return token, nil
}

// UsePasswordResetToken marks a password reset token as used.
// Returns true if the token was valid and unused, false otherwise.
func (db *DB) UsePasswordResetToken(tokenHash string) (bool, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	result, err := db.conn.ExecContext(ctx, `
		UPDATE password_reset_tokens
		SET used = 1
		WHERE token_hash = ? AND used = 0 AND expires_at > ?
	`, tokenHash, time.Now())
	if err != nil {
		return false, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// DeletePasswordResetTokens deletes all password reset tokens for a user
func (db *DB) DeletePasswordResetTokens(userID string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `DELETE FROM password_reset_tokens WHERE user_id = ?`, userID)
	return err
}

// DeleteExpiredPasswordResetTokens deletes all expired password reset tokens
func (db *DB) DeleteExpiredPasswordResetTokens() (int64, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	result, err := db.conn.ExecContext(ctx, `DELETE FROM password_reset_tokens WHERE expires_at < ?`, time.Now())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// CreateEmailVerificationToken creates a new email verification token for a user.
// Deletes any existing unused tokens for the user first.
func (db *DB) CreateEmailVerificationToken(userID, tokenHash string, expiresAt time.Time) (*EmailVerificationToken, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	// Delete existing unused tokens for this user
	_, err := db.conn.ExecContext(ctx, `DELETE FROM email_verification_tokens WHERE user_id = ? AND used = 0`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete old verification tokens: %w", err)
	}

	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token ID: %w", err)
	}
	token := &EmailVerificationToken{
		ID:        id,
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
		Used:      false,
		CreatedAt: time.Now(),
	}

	_, err = db.conn.ExecContext(ctx, `
		INSERT INTO email_verification_tokens (id, user_id, token_hash, expires_at, used, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, token.ID, token.UserID, token.TokenHash, token.ExpiresAt, 0, token.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create verification token: %w", err)
	}

	return token, nil
}

// GetEmailVerificationToken retrieves an email verification token by its hash.
// Returns nil if not found.
func (db *DB) GetEmailVerificationToken(tokenHash string) (*EmailVerificationToken, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	token := &EmailVerificationToken{}
	err := db.conn.QueryRowContext(ctx, `
		SELECT id, user_id, token_hash, expires_at, used, created_at
		FROM email_verification_tokens
		WHERE token_hash = ?
	`, tokenHash).Scan(&token.ID, &token.UserID, &token.TokenHash, &token.ExpiresAt, &token.Used, &token.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return token, nil
}

// UseEmailVerificationToken marks an email verification token as used and verifies the user's email.
// Returns true if the token was valid and unused, false otherwise.
func (db *DB) UseEmailVerificationToken(tokenHash string) (bool, error) {
	err := db.WithTransaction(func(tx *Tx) error {
		// First, get the token to find the user ID
		var token EmailVerificationToken
		err := tx.tx.QueryRow(`
			SELECT id, user_id, token_hash, expires_at, used, created_at
			FROM email_verification_tokens
			WHERE token_hash = ? AND used = 0 AND expires_at > ?
		`, tokenHash, time.Now()).Scan(&token.ID, &token.UserID, &token.TokenHash, &token.ExpiresAt, &token.Used, &token.CreatedAt)
		if err == sql.ErrNoRows {
			return fmt.Errorf("token not found or expired")
		}
		if err != nil {
			return err
		}

		// Mark token as used
		_, err = tx.tx.Exec(`UPDATE email_verification_tokens SET used = 1 WHERE id = ?`, token.ID)
		if err != nil {
			return fmt.Errorf("failed to mark token as used: %w", err)
		}

		// Verify the user's email
		now := time.Now()
		_, err = tx.tx.Exec(`UPDATE users SET email_verified = 1, verified_at = ? WHERE id = ?`, now, token.UserID)
		if err != nil {
			return fmt.Errorf("failed to verify user email: %w", err)
		}

		// Delete all verification tokens for this user (cleanup - prevents accumulation)
		_, err = tx.tx.Exec(`DELETE FROM email_verification_tokens WHERE user_id = ?`, token.UserID)
		if err != nil {
			return fmt.Errorf("failed to cleanup verification tokens: %w", err)
		}

		return nil
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

// VerifyUserEmail sets a user's email as verified
func (db *DB) VerifyUserEmail(userID string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	now := time.Now()
	_, err := db.conn.ExecContext(ctx, `UPDATE users SET email_verified = 1, verified_at = ? WHERE id = ?`, now, userID)
	return err
}

// DeleteEmailVerificationTokens deletes all email verification tokens for a user
func (db *DB) DeleteEmailVerificationTokens(userID string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `DELETE FROM email_verification_tokens WHERE user_id = ?`, userID)
	return err
}

// DeleteExpiredEmailVerificationTokens deletes all expired email verification tokens
func (db *DB) DeleteExpiredEmailVerificationTokens() (int64, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	result, err := db.conn.ExecContext(ctx, `DELETE FROM email_verification_tokens WHERE expires_at < ?`, time.Now())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// GetUnusedEmailVerificationToken returns the most recent unused verification token for a user
func (db *DB) GetUnusedEmailVerificationToken(userID string) (*EmailVerificationToken, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	token := &EmailVerificationToken{}
	err := db.conn.QueryRowContext(ctx, `
		SELECT id, user_id, token_hash, expires_at, used, created_at
		FROM email_verification_tokens
		WHERE user_id = ? AND used = 0 AND expires_at > ?
		ORDER BY created_at DESC
		LIMIT 1
	`, userID, time.Now()).Scan(&token.ID, &token.UserID, &token.TokenHash, &token.ExpiresAt, &token.Used, &token.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return token, nil
}

// CreateMagicLinkToken creates a new magic link token for passwordless login.
// Deletes any existing unused tokens for the user first.
func (db *DB) CreateMagicLinkToken(userID, tokenHash string, expiresAt time.Time) (*MagicLinkToken, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	// Delete existing unused tokens for this user
	_, err := db.conn.ExecContext(ctx, `DELETE FROM magic_link_tokens WHERE user_id = ? AND used = 0`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete old magic link tokens: %w", err)
	}

	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token ID: %w", err)
	}
	token := &MagicLinkToken{
		ID:        id,
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
		Used:      false,
		CreatedAt: time.Now(),
	}

	_, err = db.conn.ExecContext(ctx, `
		INSERT INTO magic_link_tokens (id, user_id, token_hash, expires_at, used, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, token.ID, token.UserID, token.TokenHash, token.ExpiresAt, 0, token.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create magic link token: %w", err)
	}

	return token, nil
}

// GetMagicLinkToken retrieves a magic link token by its hash.
// Returns nil if not found.
func (db *DB) GetMagicLinkToken(tokenHash string) (*MagicLinkToken, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	token := &MagicLinkToken{}
	err := db.conn.QueryRowContext(ctx, `
		SELECT id, user_id, token_hash, expires_at, used, created_at
		FROM magic_link_tokens
		WHERE token_hash = ?
	`, tokenHash).Scan(&token.ID, &token.UserID, &token.TokenHash, &token.ExpiresAt, &token.Used, &token.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return token, nil
}

// UseMagicLinkToken marks a magic link token as used.
// Returns true if the token was valid and unused, false otherwise.
func (db *DB) UseMagicLinkToken(tokenHash string) (bool, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	result, err := db.conn.ExecContext(ctx, `
		UPDATE magic_link_tokens
		SET used = 1
		WHERE token_hash = ? AND used = 0 AND expires_at > ?
	`, tokenHash, time.Now())
	if err != nil {
		return false, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// DeleteMagicLinkTokens deletes all magic link tokens for a user
func (db *DB) DeleteMagicLinkTokens(userID string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `DELETE FROM magic_link_tokens WHERE user_id = ?`, userID)
	return err
}

// DeleteExpiredMagicLinkTokens deletes all expired magic link tokens
func (db *DB) DeleteExpiredMagicLinkTokens() (int64, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	result, err := db.conn.ExecContext(ctx, `DELETE FROM magic_link_tokens WHERE expires_at < ?`, time.Now())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// CountRecentMagicLinkRequests counts magic link tokens created for a user since the given time.
// Used for per-email rate limiting to prevent abuse. Note: this counts all created tokens,
// including used and expired ones, to track request frequency.
func (db *DB) CountRecentMagicLinkRequests(userID string, since time.Time) (int, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	var count int
	err := db.conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM magic_link_tokens
		WHERE user_id = ? AND created_at > ?
	`, userID, since).Scan(&count)
	return count, err
}

// RecordLoginAttempt records a login attempt for rate limiting
func (db *DB) RecordLoginAttempt(email, ipAddress string, success bool) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return err
	}

	successInt := 0
	if success {
		successInt = 1
	}

	_, err := db.conn.ExecContext(ctx, `
		INSERT INTO login_attempts (id, email, success, ip_address, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, fmt.Sprintf("%x", id), email, successInt, ipAddress, time.Now())
	return err
}

// GetRecentFailedLoginAttempts returns the number of failed login attempts for an email
// in the given time window
func (db *DB) GetRecentFailedLoginAttempts(email string, since time.Time) (int, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	var count int
	err := db.conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM login_attempts
		WHERE email = ? AND success = 0 AND created_at > ?
	`, email, since).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// ClearLoginAttempts clears login attempts for an email (e.g., after successful login)
func (db *DB) ClearLoginAttempts(email string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `DELETE FROM login_attempts WHERE email = ?`, email)
	return err
}

// DeleteExpiredLoginAttempts deletes login attempts older than the given time
func (db *DB) DeleteExpiredLoginAttempts(olderThan time.Time) (int64, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	result, err := db.conn.ExecContext(ctx, `DELETE FROM login_attempts WHERE created_at < ?`, olderThan)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// IsEmailLocked checks if an email is locked due to too many failed login attempts
// Returns true if locked, along with the time when the lock expires
func (db *DB) IsEmailLocked(email string, maxAttempts int, lockDuration time.Duration) (bool, time.Time, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	since := time.Now().Add(-lockDuration)
	count, err := db.GetRecentFailedLoginAttempts(email, since)
	if err != nil {
		return false, time.Time{}, err
	}

	if count >= maxAttempts {
		// Get the oldest failed attempt in the window to calculate unlock time
		// Note: MIN() returns a string in SQLite, so we scan to string and parse
		var oldestAttemptStr sql.NullString
		err := db.conn.QueryRowContext(ctx, `
			SELECT MIN(created_at) FROM login_attempts
			WHERE email = ? AND success = 0 AND created_at > ?
		`, email, since).Scan(&oldestAttemptStr)
		if err != nil {
			return false, time.Time{}, fmt.Errorf("failed to get oldest login attempt: %w", err)
		}
		if !oldestAttemptStr.Valid {
			// This shouldn't happen since count >= maxAttempts, but handle gracefully
			return false, time.Time{}, fmt.Errorf("no login attempts found despite count >= %d", maxAttempts)
		}
		oldestAttempt, err := parseSQLiteTimestamp(oldestAttemptStr.String)
		if err != nil {
			return false, time.Time{}, fmt.Errorf("failed to parse oldest login attempt time: %w", err)
		}
		unlockTime := oldestAttempt.Add(lockDuration)
		return true, unlockTime, nil
	}

	return false, time.Time{}, nil
}

// EnableTOTPWithRecoveryCodes enables 2FA and saves recovery codes in a single transaction.
// This ensures that if code saving fails, the 2FA enable is rolled back.
func (db *DB) EnableTOTPWithRecoveryCodes(userID string, codeHashes []string) error {
	return db.WithTransaction(func(tx *Tx) error {
		// Enable TOTP
		_, err := tx.tx.Exec(`UPDATE users SET totp_enabled = 1 WHERE id = ?`, userID)
		if err != nil {
			return fmt.Errorf("failed to enable TOTP: %w", err)
		}

		// Delete existing unused recovery codes
		_, err = tx.tx.Exec(`DELETE FROM recovery_codes WHERE user_id = ? AND used = 0`, userID)
		if err != nil {
			return fmt.Errorf("failed to delete old recovery codes: %w", err)
		}

		// Insert new recovery codes
		for _, hash := range codeHashes {
			id, err := generateID()
			if err != nil {
				return fmt.Errorf("failed to generate recovery code ID: %w", err)
			}
			_, err = tx.tx.Exec(`
				INSERT INTO recovery_codes (id, user_id, code_hash, created_at)
				VALUES (?, ?, ?, ?)
			`, id, userID, hash, time.Now())
			if err != nil {
				return fmt.Errorf("failed to insert recovery code: %w", err)
			}
		}

		return nil
	})
}

// CompletePasswordReset updates password, marks token as used, deletes all tokens,
// and deletes all sessions in a single transaction.
func (db *DB) CompletePasswordReset(userID, tokenHash, passwordHash string) error {
	return db.WithTransaction(func(tx *Tx) error {
		// Update password
		_, err := tx.tx.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, userID)
		if err != nil {
			return fmt.Errorf("failed to update password: %w", err)
		}

		// Mark token as used
		_, err = tx.tx.Exec(`
			UPDATE password_reset_tokens
			SET used = 1
			WHERE token_hash = ? AND used = 0 AND expires_at > ?
		`, tokenHash, time.Now())
		if err != nil {
			return fmt.Errorf("failed to mark token as used: %w", err)
		}

		// Delete all password reset tokens for this user
		_, err = tx.tx.Exec(`DELETE FROM password_reset_tokens WHERE user_id = ?`, userID)
		if err != nil {
			return fmt.Errorf("failed to delete password reset tokens: %w", err)
		}

		// Delete all sessions for this user (force re-login)
		_, err = tx.tx.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID)
		if err != nil {
			return fmt.Errorf("failed to delete sessions: %w", err)
		}

		return nil
	})
}

// DisableTOTPAndClearSessions disables 2FA, deletes recovery codes, and clears all sessions
// in a single transaction. Used during account recovery.
func (db *DB) DisableTOTPAndClearSessions(userID string) error {
	return db.WithTransaction(func(tx *Tx) error {
		// Disable TOTP
		_, err := tx.tx.Exec(`UPDATE users SET totp_enabled = 0, totp_secret = NULL WHERE id = ?`, userID)
		if err != nil {
			return fmt.Errorf("failed to disable TOTP: %w", err)
		}

		// Delete recovery codes
		_, err = tx.tx.Exec(`DELETE FROM recovery_codes WHERE user_id = ?`, userID)
		if err != nil {
			return fmt.Errorf("failed to delete recovery codes: %w", err)
		}

		// Delete all sessions
		_, err = tx.tx.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID)
		if err != nil {
			return fmt.Errorf("failed to delete sessions: %w", err)
		}

		return nil
	})
}
