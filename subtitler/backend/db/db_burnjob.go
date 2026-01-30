package db

import (
	"database/sql"
	"fmt"
	"time"
)

// CreateBurnJob creates a new burn job record
func (db *DB) CreateBurnJob(job *BurnJob) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `
		INSERT INTO burn_jobs (id, video_id, status, message, progress, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, job.ID, job.VideoID, job.Status, job.Message, job.Progress, job.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create burn job for video %s: %w", job.VideoID, err)
	}
	return nil
}

// GetBurnJob retrieves a burn job by video ID
func (db *DB) GetBurnJob(videoID string) (*BurnJob, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	job := &BurnJob{}
	var message, outputPath sql.NullString
	var outputKeyVersion sql.NullInt64
	var completedAt sql.NullTime

	err := db.conn.QueryRowContext(ctx, `
		SELECT id, video_id, status, message, progress, output_path, output_key_version, created_at, completed_at
		FROM burn_jobs WHERE video_id = ? ORDER BY created_at DESC LIMIT 1
	`, videoID).Scan(&job.ID, &job.VideoID, &job.Status, &message, &job.Progress, &outputPath, &outputKeyVersion, &job.CreatedAt, &completedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get burn job for video %s: %w", videoID, err)
	}

	if message.Valid {
		job.Message = message.String
	}
	if outputPath.Valid {
		job.OutputPath = outputPath.String
	}
	if outputKeyVersion.Valid {
		v := int(outputKeyVersion.Int64)
		job.OutputKeyVersion = &v
	}
	if completedAt.Valid {
		job.CompletedAt = &completedAt.Time
	}

	return job, nil
}

// UpdateBurnJobStatus updates the status and message of a burn job.
// Only updates if the current status is 'pending' or 'processing' to avoid race conditions
// with completion/failure updates from the main goroutine. Once status becomes 'complete'
// or 'error', progress updates are blocked.
func (db *DB) UpdateBurnJobStatus(videoID, status, message string, progress int) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `
		UPDATE burn_jobs SET status = ?, message = ?, progress = ?
		WHERE video_id = ? AND status IN ('pending', 'processing')
	`, status, message, progress, videoID)
	if err != nil {
		return fmt.Errorf("failed to update burn job status for video %s: %w", videoID, err)
	}
	return nil
}

// CompleteBurnJob marks a burn job as complete with the output file path
func (db *DB) CompleteBurnJob(videoID, outputPath string) error {
	return db.CompleteBurnJobWithKeyVersion(videoID, outputPath, 1)
}

// CompleteBurnJobWithKeyVersion marks a burn job as complete with output file path and key version
func (db *DB) CompleteBurnJobWithKeyVersion(videoID, outputPath string, keyVersion int) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	now := time.Now()
	_, err := db.conn.ExecContext(ctx, `
		UPDATE burn_jobs
		SET status = 'complete', message = 'Subtitles burned successfully', progress = 100,
		    output_path = ?, output_key_version = ?, completed_at = ?
		WHERE video_id = ?
	`, outputPath, keyVersion, now, videoID)
	if err != nil {
		return fmt.Errorf("failed to complete burn job for video %s: %w", videoID, err)
	}
	return nil
}

// FailBurnJob marks a burn job as failed with an error message
func (db *DB) FailBurnJob(videoID, errorMessage string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `
		UPDATE burn_jobs SET status = 'error', message = ? WHERE video_id = ?
	`, errorMessage, videoID)
	if err != nil {
		return fmt.Errorf("failed to mark burn job as failed for video %s: %w", videoID, err)
	}
	return nil
}
