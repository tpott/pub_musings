-- Rollback upload sessions tables
DROP INDEX IF EXISTS idx_upload_chunks_session_id;
DROP INDEX IF EXISTS idx_upload_sessions_session_id;
DROP INDEX IF EXISTS idx_upload_sessions_user_id;
DROP INDEX IF EXISTS idx_upload_sessions_status;
DROP INDEX IF EXISTS idx_upload_sessions_expires_at;
DROP TABLE IF EXISTS upload_chunks;
DROP TABLE IF EXISTS upload_sessions;
