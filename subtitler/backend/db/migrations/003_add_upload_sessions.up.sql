-- Upload sessions table for chunked uploads
-- Tracks multi-part upload sessions that can be resumed if interrupted

CREATE TABLE IF NOT EXISTS upload_sessions (
    id TEXT PRIMARY KEY,
    filename TEXT NOT NULL,
    content_type TEXT NOT NULL,
    total_size INTEGER NOT NULL,
    chunk_size INTEGER NOT NULL,
    total_chunks INTEGER NOT NULL,
    user_id TEXT,
    session_id TEXT,
    status TEXT NOT NULL DEFAULT 'in_progress',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL,
    completed_at DATETIME
);

-- Upload chunks table tracks individual chunks within a session
CREATE TABLE IF NOT EXISTS upload_chunks (
    id TEXT PRIMARY KEY,
    upload_session_id TEXT NOT NULL REFERENCES upload_sessions(id),
    chunk_index INTEGER NOT NULL,
    chunk_path TEXT NOT NULL,
    size INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(upload_session_id, chunk_index)
);

-- Indexes for efficient queries
CREATE INDEX IF NOT EXISTS idx_upload_sessions_expires_at ON upload_sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_upload_sessions_status ON upload_sessions(status);
CREATE INDEX IF NOT EXISTS idx_upload_sessions_user_id ON upload_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_upload_sessions_session_id ON upload_sessions(session_id);
CREATE INDEX IF NOT EXISTS idx_upload_chunks_session_id ON upload_chunks(upload_session_id);
