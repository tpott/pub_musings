-- Rollback migration 008: Remove query optimization indexes

DROP INDEX IF EXISTS idx_transcriptions_status;
DROP INDEX IF EXISTS idx_burn_jobs_video_created;
DROP INDEX IF EXISTS idx_feedback_status_type;
