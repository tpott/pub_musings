package db

import (
	"database/sql"
	"fmt"
)

// CreateUser creates a new user record
func (db *DB) CreateUser(user *User) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	// Default to user role if not specified
	role := user.Role
	if role == "" {
		role = RoleUser
	}
	_, err := db.conn.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, totp_secret, totp_enabled, email_verified, verified_at, role, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, user.ID, user.Email, user.PasswordHash, user.TOTPSecret, user.TOTPEnabled, user.EmailVerified, user.VerifiedAt, role, user.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

// GetUserByID retrieves a user by ID
func (db *DB) GetUserByID(id string) (*User, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	user := &User{}
	var totpSecret sql.NullString
	var verifiedAt sql.NullTime
	err := db.conn.QueryRowContext(ctx, `
		SELECT id, email, password_hash, totp_secret, totp_enabled, email_verified, verified_at, role, created_at
		FROM users WHERE id = ?
	`, id).Scan(&user.ID, &user.Email, &user.PasswordHash, &totpSecret, &user.TOTPEnabled, &user.EmailVerified, &verifiedAt, &user.Role, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by ID %s: %w", id, err)
	}
	if totpSecret.Valid {
		user.TOTPSecret = &totpSecret.String
	}
	if verifiedAt.Valid {
		user.VerifiedAt = &verifiedAt.Time
	}
	return user, nil
}

// GetUserByEmail retrieves a user by email
func (db *DB) GetUserByEmail(email string) (*User, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	user := &User{}
	var totpSecret sql.NullString
	var verifiedAt sql.NullTime
	err := db.conn.QueryRowContext(ctx, `
		SELECT id, email, password_hash, totp_secret, totp_enabled, email_verified, verified_at, role, created_at
		FROM users WHERE email = ?
	`, email).Scan(&user.ID, &user.Email, &user.PasswordHash, &totpSecret, &user.TOTPEnabled, &user.EmailVerified, &verifiedAt, &user.Role, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	if totpSecret.Valid {
		user.TOTPSecret = &totpSecret.String
	}
	if verifiedAt.Valid {
		user.VerifiedAt = &verifiedAt.Time
	}
	return user, nil
}

// UpdateUserPassword updates a user's password hash
func (db *DB) UpdateUserPassword(userID, passwordHash string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, userID)
	return err
}

// UpdateUserRole updates a user's role
func (db *DB) UpdateUserRole(userID, role string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	// Validate role
	if role != RoleUser && role != RoleAdmin {
		return fmt.Errorf("invalid role: %s", role)
	}
	_, err := db.conn.ExecContext(ctx, `UPDATE users SET role = ? WHERE id = ?`, role, userID)
	return err
}

// PromoteToAdmin promotes a user to admin role by email
func (db *DB) PromoteToAdmin(email string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	result, err := db.conn.ExecContext(ctx, `UPDATE users SET role = ? WHERE email = ?`, RoleAdmin, email)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("user not found: %s", email)
	}
	return nil
}
