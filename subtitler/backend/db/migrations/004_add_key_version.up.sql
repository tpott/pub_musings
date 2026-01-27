-- Add key_version column to track which encryption key was used for each file
-- This enables key rotation by allowing the system to identify which key decrypts each file

-- Add key_version to videos table
-- Default 1 for existing files (they were encrypted with the original key)
ALTER TABLE videos ADD COLUMN key_version INTEGER NOT NULL DEFAULT 1;

-- Add key_version to burn_jobs output files
-- Nullable because not all burn jobs have output files
ALTER TABLE burn_jobs ADD COLUMN output_key_version INTEGER;

-- Index for efficient querying of files that need re-encryption
CREATE INDEX IF NOT EXISTS idx_videos_key_version ON videos(key_version);
