package db

import (
	"database/sql"
	"fmt"
	"time"
)

// CreateSession creates a new session record
func (db *DB) CreateSession(session *Session) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `
		INSERT INTO sessions (id, user_id, token, ip_address, user_agent, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, session.ID, session.UserID, session.Token, session.IPAddress, session.UserAgent, session.ExpiresAt, session.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	return nil
}

// GetSessionByToken retrieves a session by token
func (db *DB) GetSessionByToken(token string) (*Session, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	session := &Session{}
	var ipAddress, userAgent sql.NullString
	err := db.conn.QueryRowContext(ctx, `
		SELECT id, user_id, token, ip_address, user_agent, expires_at, created_at
		FROM sessions WHERE token = ?
	`, token).Scan(&session.ID, &session.UserID, &session.Token, &ipAddress, &userAgent, &session.ExpiresAt, &session.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session by token: %w", err)
	}
	if ipAddress.Valid {
		session.IPAddress = ipAddress.String
	}
	if userAgent.Valid {
		session.UserAgent = userAgent.String
	}
	return session, nil
}

// MaxSessionsPerUser is the maximum number of sessions returned per user.
// This prevents memory issues for users with many sessions.
const MaxSessionsPerUser = 100

// GetSessionsByUserID retrieves active sessions for a user, limited to MaxSessionsPerUser.
// Sessions are ordered by creation date (newest first).
func (db *DB) GetSessionsByUserID(userID string) ([]Session, error) {
	return db.GetSessionsByUserIDWithLimit(userID, MaxSessionsPerUser)
}

// GetSessionsByUserIDWithLimit retrieves active sessions for a user with a custom limit.
// If limit is 0 or negative, MaxSessionsPerUser is used.
func (db *DB) GetSessionsByUserIDWithLimit(userID string, limit int) ([]Session, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	if limit <= 0 {
		limit = MaxSessionsPerUser
	}

	rows, err := db.conn.QueryContext(ctx, `
		SELECT id, user_id, token, ip_address, user_agent, expires_at, created_at
		FROM sessions
		WHERE user_id = ? AND expires_at > ?
		ORDER BY created_at DESC
		LIMIT ?
	`, userID, time.Now(), limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query sessions for user: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var session Session
		var ipAddress, userAgent sql.NullString
		if err := rows.Scan(&session.ID, &session.UserID, &session.Token, &ipAddress, &userAgent, &session.ExpiresAt, &session.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan session row: %w", err)
		}
		if ipAddress.Valid {
			session.IPAddress = ipAddress.String
		}
		if userAgent.Valid {
			session.UserAgent = userAgent.String
		}
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate sessions: %w", err)
	}
	return sessions, nil
}

// DeleteSessionByID deletes a session by its ID (for session management)
func (db *DB) DeleteSessionByID(sessionID, userID string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	// Require userID to ensure users can only delete their own sessions
	result, err := db.conn.ExecContext(ctx, `DELETE FROM sessions WHERE id = ? AND user_id = ?`, sessionID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("session not found or not owned by user")
	}
	return nil
}

// DeleteSession deletes a session by token
func (db *DB) DeleteSession(token string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `DELETE FROM sessions WHERE token = ?`, token)
	if err != nil {
		return fmt.Errorf("failed to delete session by token: %w", err)
	}
	return nil
}

// DeleteExpiredSessions deletes all expired sessions
func (db *DB) DeleteExpiredSessions() (int64, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	result, err := db.conn.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`, time.Now())
	if err != nil {
		return 0, fmt.Errorf("failed to delete expired sessions: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get affected rows: %w", err)
	}
	return count, nil
}

// DeleteUserSessions deletes all sessions for a user
func (db *DB) DeleteUserSessions(userID string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user sessions: %w", err)
	}
	return nil
}

// CountActiveSessions returns the count of non-expired sessions.
// Used for metrics reporting.
func (db *DB) CountActiveSessions() (int, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	var count int
	err := db.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE expires_at > ?`, time.Now()).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count active sessions: %w", err)
	}
	return count, nil
}
