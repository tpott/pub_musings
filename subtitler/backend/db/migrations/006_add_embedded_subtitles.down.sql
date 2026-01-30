-- Migration 006 rollback: Remove embedded_subtitles_json column from videos table
-- Note: SQLite doesn't support DROP COLUMN in older versions, so we recreate the table

-- Create a new table without the column
CREATE TABLE videos_new (
    id TEXT PRIMARY KEY,
    filename TEXT NOT NULL,
    size INTEGER NOT NULL,
    content_type TEXT NOT NULL,
    file_path TEXT NOT NULL,
    thumbnail_path TEXT,
    key_version INTEGER DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    user_id TEXT,
    session_id TEXT
);

-- Copy data from old table to new table
INSERT INTO videos_new (id, filename, size, content_type, file_path, thumbnail_path, key_version, created_at, user_id, session_id)
SELECT id, filename, size, content_type, file_path, thumbnail_path, key_version, created_at, user_id, session_id
FROM videos;

-- Drop old table and rename new table
DROP TABLE videos;
ALTER TABLE videos_new RENAME TO videos;

-- Recreate indices
CREATE INDEX IF NOT EXISTS idx_videos_user_id ON videos(user_id);
CREATE INDEX IF NOT EXISTS idx_videos_session_id ON videos(session_id);
CREATE INDEX IF NOT EXISTS idx_videos_created_at ON videos(created_at);
