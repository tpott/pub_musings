-- Rollback initial schema
-- Drop all tables in reverse order of creation (respecting foreign keys)

DROP INDEX IF EXISTS idx_magic_link_expires;
DROP INDEX IF EXISTS idx_magic_link_user_id;
DROP TABLE IF EXISTS magic_link_tokens;

DROP INDEX IF EXISTS idx_email_verification_expires;
DROP INDEX IF EXISTS idx_email_verification_user_id;
DROP TABLE IF EXISTS email_verification_tokens;

DROP INDEX IF EXISTS idx_login_attempts_created_at;
DROP INDEX IF EXISTS idx_login_attempts_email;
DROP TABLE IF EXISTS login_attempts;

DROP INDEX IF EXISTS idx_password_reset_expires;
DROP INDEX IF EXISTS idx_password_reset_user_id;
DROP TABLE IF EXISTS password_reset_tokens;

DROP INDEX IF EXISTS idx_recovery_codes_user_id;
DROP TABLE IF EXISTS recovery_codes;

DROP INDEX IF EXISTS idx_burn_jobs_video_id;
DROP TABLE IF EXISTS burn_jobs;

DROP INDEX IF EXISTS idx_sessions_expires_at;
DROP INDEX IF EXISTS idx_sessions_user_id;
DROP INDEX IF EXISTS idx_sessions_token;
DROP TABLE IF EXISTS sessions;

DROP INDEX IF EXISTS idx_users_email;
DROP TABLE IF EXISTS users;

DROP INDEX IF EXISTS idx_transcriptions_video_id;
DROP INDEX IF EXISTS idx_videos_created_at;
DROP INDEX IF EXISTS idx_videos_session_id;
DROP INDEX IF EXISTS idx_videos_user_id;
DROP TABLE IF EXISTS transcriptions;
DROP TABLE IF EXISTS videos;
