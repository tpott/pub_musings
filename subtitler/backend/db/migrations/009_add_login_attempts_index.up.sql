-- Migration 009: Add composite index for login_attempts queries
-- Task 269

-- Composite index on login_attempts for email + created_at queries
-- Used by GetRecentFailedLoginAttempts and IsEmailLocked functions
-- The DESC ordering helps queries that filter by recent timestamps
CREATE INDEX IF NOT EXISTS idx_login_attempts_email_created ON login_attempts(email, created_at DESC);
