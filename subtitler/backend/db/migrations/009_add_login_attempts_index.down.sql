-- Rollback migration 009: Remove login_attempts composite index

DROP INDEX IF EXISTS idx_login_attempts_email_created;
