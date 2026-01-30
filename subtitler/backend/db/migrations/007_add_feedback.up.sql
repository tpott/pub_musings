-- Migration 007: Add feedback table for user feedback system

CREATE TABLE IF NOT EXISTS feedback (
    id TEXT PRIMARY KEY,
    user_id TEXT,                    -- NULL for anonymous users
    session_id TEXT,                 -- For anonymous tracking
    video_id TEXT,                   -- Which video (optional)
    page_url TEXT NOT NULL,          -- Current page URL
    feedback_text TEXT NOT NULL,     -- User's message
    rating INTEGER,                  -- 1-5 or NULL
    feedback_type TEXT NOT NULL,     -- "general", "bug", "feature"
    browser_info TEXT,               -- User agent, viewport, etc.
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    status TEXT NOT NULL DEFAULT 'new',  -- "new", "read", "resolved"
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL,
    FOREIGN KEY (video_id) REFERENCES videos(id) ON DELETE SET NULL
);

CREATE INDEX IF NOT EXISTS idx_feedback_user_id ON feedback(user_id);
CREATE INDEX IF NOT EXISTS idx_feedback_created_at ON feedback(created_at);
CREATE INDEX IF NOT EXISTS idx_feedback_status ON feedback(status);
CREATE INDEX IF NOT EXISTS idx_feedback_type ON feedback(feedback_type);
