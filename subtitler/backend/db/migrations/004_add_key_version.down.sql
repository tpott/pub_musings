-- Remove key_version columns
-- Note: SQLite doesn't support DROP COLUMN directly, so we recreate tables

-- Drop the index first
DROP INDEX IF EXISTS idx_videos_key_version;

-- For videos: create new table without key_version, copy data, replace
CREATE TABLE videos_new (
    id TEXT PRIMARY KEY,
    filename TEXT NOT NULL,
    size INTEGER NOT NULL,
    content_type TEXT NOT NULL,
    file_path TEXT NOT NULL,
    thumbnail_path TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    user_id TEXT,
    session_id TEXT
);

INSERT INTO videos_new (id, filename, size, content_type, file_path, thumbnail_path, created_at, user_id, session_id)
SELECT id, filename, size, content_type, file_path, thumbnail_path, created_at, user_id, session_id FROM videos;

DROP TABLE videos;
ALTER TABLE videos_new RENAME TO videos;

-- Recreate indexes on videos
CREATE INDEX IF NOT EXISTS idx_videos_user_id ON videos(user_id);
CREATE INDEX IF NOT EXISTS idx_videos_session_id ON videos(session_id);
CREATE INDEX IF NOT EXISTS idx_videos_created_at ON videos(created_at);

-- For burn_jobs: create new table without output_key_version, copy data, replace
CREATE TABLE burn_jobs_new (
    id TEXT PRIMARY KEY,
    video_id TEXT NOT NULL REFERENCES videos(id),
    status TEXT NOT NULL DEFAULT 'pending',
    message TEXT,
    progress INTEGER NOT NULL DEFAULT 0,
    output_path TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME
);

INSERT INTO burn_jobs_new (id, video_id, status, message, progress, output_path, created_at, completed_at)
SELECT id, video_id, status, message, progress, output_path, created_at, completed_at FROM burn_jobs;

DROP TABLE burn_jobs;
ALTER TABLE burn_jobs_new RENAME TO burn_jobs;

-- Recreate index on burn_jobs
CREATE INDEX IF NOT EXISTS idx_burn_jobs_video_id ON burn_jobs(video_id);
