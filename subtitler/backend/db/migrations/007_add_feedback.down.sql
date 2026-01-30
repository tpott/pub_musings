-- Migration 007 rollback: Remove feedback table

DROP INDEX IF EXISTS idx_feedback_type;
DROP INDEX IF EXISTS idx_feedback_status;
DROP INDEX IF EXISTS idx_feedback_created_at;
DROP INDEX IF EXISTS idx_feedback_user_id;
DROP TABLE IF EXISTS feedback;
