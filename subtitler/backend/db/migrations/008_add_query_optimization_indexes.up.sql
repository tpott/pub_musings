-- Migration 008: Add query optimization indexes
-- Tasks 250, 251, 252

-- Task 250: Index on transcriptions.status for filtering pending/processing jobs
CREATE INDEX IF NOT EXISTS idx_transcriptions_status ON transcriptions(status);

-- Task 251: Composite index on burn_jobs for video_id + created_at queries
-- The DESC ordering helps queries that need recent jobs first
CREATE INDEX IF NOT EXISTS idx_burn_jobs_video_created ON burn_jobs(video_id, created_at DESC);

-- Task 252: Composite index on feedback for admin dashboard filtering
-- Combines status and type for efficient multi-column filtering
CREATE INDEX IF NOT EXISTS idx_feedback_status_type ON feedback(status, feedback_type);
