-- Add admin flag to users table
-- Admins can view system-wide analytics at /admin/analytics

ALTER TABLE users ADD COLUMN is_admin BOOLEAN NOT NULL DEFAULT FALSE;
