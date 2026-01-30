-- Add role column to users table for authorization
-- Default is 'user' for regular users, 'admin' for administrators

-- Add role column with default value 'user' for existing users
ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'user';

-- Create index for efficient role-based queries
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
