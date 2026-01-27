-- Remove role column from users table
-- Note: SQLite doesn't support DROP COLUMN directly, so we recreate the table

-- Drop the index first
DROP INDEX IF EXISTS idx_users_role;

-- For users: create new table without role, copy data, replace
CREATE TABLE users_new (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    totp_secret TEXT,
    totp_enabled INTEGER NOT NULL DEFAULT 0,
    email_verified INTEGER NOT NULL DEFAULT 0,
    verified_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO users_new (id, email, password_hash, totp_secret, totp_enabled, email_verified, verified_at, created_at)
SELECT id, email, password_hash, totp_secret, totp_enabled, email_verified, verified_at, created_at FROM users;

DROP TABLE users;
ALTER TABLE users_new RENAME TO users;

-- Recreate index on users
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email);
