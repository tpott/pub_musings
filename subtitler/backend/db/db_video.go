package db

import (
	"database/sql"
	"fmt"
	"time"
)

// CreateVideo creates a new video record
func (db *DB) CreateVideo(video *Video) error {
	// Validate video size is within reasonable bounds
	if video.Size < MinVideoSize || video.Size > MaxVideoSize {
		return fmt.Errorf("%w: size %d is outside valid range (%d to %d bytes)",
			ErrInvalidVideoSize, video.Size, MinVideoSize, MaxVideoSize)
	}

	ctx, cancel := db.queryContext()
	defer cancel()

	// Default key_version to 1 if not set
	keyVersion := video.KeyVersion
	if keyVersion == 0 {
		keyVersion = 1
	}
	_, err := db.conn.ExecContext(ctx, `
		INSERT INTO videos (id, filename, size, content_type, file_path, thumbnail_path, key_version, embedded_subtitles_json, created_at, user_id, session_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, video.ID, video.Filename, video.Size, video.ContentType, video.FilePath, video.ThumbnailPath, keyVersion, video.EmbeddedSubtitlesJSON, video.CreatedAt, video.UserID, video.SessionID)
	return err
}

// GetVideo retrieves a video by ID
func (db *DB) GetVideo(id string) (*Video, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	video := &Video{}
	err := db.conn.QueryRowContext(ctx, `
		SELECT id, filename, size, content_type, file_path, thumbnail_path, key_version, embedded_subtitles_json, created_at, user_id, session_id
		FROM videos WHERE id = ?
	`, id).Scan(&video.ID, &video.Filename, &video.Size, &video.ContentType, &video.FilePath, &video.ThumbnailPath, &video.KeyVersion, &video.EmbeddedSubtitlesJSON, &video.CreatedAt, &video.UserID, &video.SessionID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get video %s: %w", id, err)
	}
	return video, nil
}

// ListVideos returns all videos, optionally filtered by user or session
func (db *DB) ListVideos(userID, sessionID *string) ([]Video, error) {
	result, err := db.ListVideosPaginated(userID, sessionID, 0, 0)
	if err != nil {
		return nil, err
	}
	return result.Videos, nil
}

// ListVideosPaginated returns videos with pagination support
// limit=0 means no limit, offset=0 starts from the beginning
func (db *DB) ListVideosPaginated(userID, sessionID *string, limit, offset int) (*VideoListResult, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	var whereClause string
	var args []interface{}

	if userID != nil {
		whereClause = "WHERE user_id = ?"
		args = append(args, *userID)
	} else if sessionID != nil {
		whereClause = "WHERE session_id = ?"
		args = append(args, *sessionID)
	}

	// Get total count
	countQuery := "SELECT COUNT(*) FROM videos " + whereClause
	var totalCount int
	if err := db.conn.QueryRowContext(ctx, countQuery, args...).Scan(&totalCount); err != nil {
		return nil, fmt.Errorf("failed to count videos: %w", err)
	}

	// Build paginated query with parameterized LIMIT/OFFSET
	query := `SELECT id, filename, size, content_type, file_path, thumbnail_path, key_version, embedded_subtitles_json, created_at, user_id, session_id
		FROM videos ` + whereClause + ` ORDER BY created_at DESC`

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
		if offset > 0 {
			query += " OFFSET ?"
			args = append(args, offset)
		}
	}

	rows, err := db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query videos: %w", err)
	}
	defer rows.Close()

	var videos []Video
	for rows.Next() {
		var v Video
		if err := rows.Scan(&v.ID, &v.Filename, &v.Size, &v.ContentType, &v.FilePath, &v.ThumbnailPath, &v.KeyVersion, &v.EmbeddedSubtitlesJSON, &v.CreatedAt, &v.UserID, &v.SessionID); err != nil {
			return nil, fmt.Errorf("failed to scan video row: %w", err)
		}
		videos = append(videos, v)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating video rows: %w", err)
	}

	return &VideoListResult{
		Videos:     videos,
		TotalCount: totalCount,
	}, nil
}

// CountVideosBySession returns the number of videos uploaded by a session
func (db *DB) CountVideosBySession(sessionID string) (int, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	var count int
	err := db.conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM videos WHERE session_id = ?
	`, sessionID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count videos for session: %w", err)
	}
	return count, nil
}

// DefaultCleanupBatchSize is the default number of expired videos to process in each batch
const DefaultCleanupBatchSize = 100

// GetExpiredVideos returns videos that have exceeded their retention period.
// Anonymous videos (no user_id) expire after 48 hours.
// Registered user videos expire after 90 days.
// Deprecated: Use GetExpiredVideosPaginated for large datasets to prevent OOM.
func (db *DB) GetExpiredVideos() ([]Video, error) {
	return db.GetExpiredVideosPaginated(0, 0) // No limit for backwards compatibility
}

// GetExpiredVideosPaginated returns a batch of expired videos with pagination.
// Use limit=0 for no limit (not recommended for large datasets).
// Anonymous videos expire after 48 hours, registered videos after 90 days.
func (db *DB) GetExpiredVideosPaginated(limit, offset int) ([]Video, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	now := time.Now()
	anonymousExpiry := now.Add(-48 * time.Hour)
	registeredExpiry := now.Add(-90 * 24 * time.Hour)

	query := `
		SELECT id, filename, size, content_type, file_path, thumbnail_path, key_version, created_at, user_id, session_id
		FROM videos
		WHERE (user_id IS NULL AND created_at < ?)
		   OR (user_id IS NOT NULL AND created_at < ?)
		ORDER BY created_at ASC`

	args := []interface{}{anonymousExpiry, registeredExpiry}

	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
		if offset > 0 {
			query += " OFFSET ?"
			args = append(args, offset)
		}
	}

	rows, err := db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query expired videos: %w", err)
	}
	defer rows.Close()

	var videos []Video
	for rows.Next() {
		var v Video
		if err := rows.Scan(&v.ID, &v.Filename, &v.Size, &v.ContentType, &v.FilePath, &v.ThumbnailPath, &v.KeyVersion, &v.CreatedAt, &v.UserID, &v.SessionID); err != nil {
			return nil, fmt.Errorf("failed to scan expired video: %w", err)
		}
		videos = append(videos, v)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating expired videos: %w", err)
	}

	return videos, nil
}

// CountExpiredVideos returns the total count of expired videos.
func (db *DB) CountExpiredVideos() (int, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	now := time.Now()
	anonymousExpiry := now.Add(-48 * time.Hour)
	registeredExpiry := now.Add(-90 * 24 * time.Hour)

	var count int
	err := db.conn.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM videos
		WHERE (user_id IS NULL AND created_at < ?)
		   OR (user_id IS NOT NULL AND created_at < ?)
	`, anonymousExpiry, registeredExpiry).Scan(&count)

	if err != nil {
		return 0, fmt.Errorf("failed to count expired videos: %w", err)
	}

	return count, nil
}

// DeleteVideo deletes a video and its associated transcription from the database.
// Returns the file paths so the caller can delete the files from disk.
func (db *DB) DeleteVideo(videoID string) (*DeletedVideoFiles, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	// Get the file paths before deleting
	video, err := db.GetVideo(videoID)
	if err != nil {
		return nil, err
	}
	if video == nil {
		return nil, nil
	}

	// Get burn job output path before deleting
	var burnOutputPath *string
	burnJob, err := db.GetBurnJob(videoID)
	if err != nil {
		return nil, fmt.Errorf("failed to get burn job: %w", err)
	}
	if burnJob != nil && burnJob.OutputPath != "" {
		burnOutputPath = &burnJob.OutputPath
	}

	// Delete transcription first (foreign key constraint)
	_, err = db.conn.ExecContext(ctx, `DELETE FROM transcriptions WHERE video_id = ?`, videoID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete transcription: %w", err)
	}

	// Delete burn jobs
	_, err = db.conn.ExecContext(ctx, `DELETE FROM burn_jobs WHERE video_id = ?`, videoID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete burn jobs: %w", err)
	}

	// Delete video record
	_, err = db.conn.ExecContext(ctx, `DELETE FROM videos WHERE id = ?`, videoID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete video: %w", err)
	}

	return &DeletedVideoFiles{
		FilePath:       video.FilePath,
		ThumbnailPath:  video.ThumbnailPath,
		BurnOutputPath: burnOutputPath,
	}, nil
}

// UpdateVideoThumbnail updates the thumbnail path for a video
func (db *DB) UpdateVideoThumbnail(videoID, thumbnailPath string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `UPDATE videos SET thumbnail_path = ? WHERE id = ?`, thumbnailPath, videoID)
	return err
}

// UpdateVideoEmbeddedSubtitles updates the embedded subtitles JSON for a video
func (db *DB) UpdateVideoEmbeddedSubtitles(videoID string, subtitlesJSON *string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `UPDATE videos SET embedded_subtitles_json = ? WHERE id = ?`, subtitlesJSON, videoID)
	return err
}

// GetVideosByKeyVersion returns all videos encrypted with a specific key version
func (db *DB) GetVideosByKeyVersion(keyVersion int) ([]Video, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	rows, err := db.conn.QueryContext(ctx, `
		SELECT id, filename, size, content_type, file_path, thumbnail_path, key_version, created_at, user_id, session_id
		FROM videos WHERE key_version = ? ORDER BY created_at
	`, keyVersion)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []Video
	for rows.Next() {
		var v Video
		if err := rows.Scan(&v.ID, &v.Filename, &v.Size, &v.ContentType, &v.FilePath, &v.ThumbnailPath, &v.KeyVersion, &v.CreatedAt, &v.UserID, &v.SessionID); err != nil {
			return nil, err
		}
		videos = append(videos, v)
	}
	return videos, rows.Err()
}

// GetVideosWithOldKeyVersion returns videos encrypted with keys older than the specified version
// Useful for finding files that need re-encryption during key rotation
func (db *DB) GetVideosWithOldKeyVersion(currentVersion, limit int) ([]Video, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	rows, err := db.conn.QueryContext(ctx, `
		SELECT id, filename, size, content_type, file_path, thumbnail_path, key_version, created_at, user_id, session_id
		FROM videos WHERE key_version < ? ORDER BY created_at LIMIT ?
	`, currentVersion, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []Video
	for rows.Next() {
		var v Video
		if err := rows.Scan(&v.ID, &v.Filename, &v.Size, &v.ContentType, &v.FilePath, &v.ThumbnailPath, &v.KeyVersion, &v.CreatedAt, &v.UserID, &v.SessionID); err != nil {
			return nil, err
		}
		videos = append(videos, v)
	}
	return videos, rows.Err()
}

// CountVideosByKeyVersion returns the count of videos per key version
func (db *DB) CountVideosByKeyVersion() (map[int]int, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	rows, err := db.conn.QueryContext(ctx, `
		SELECT key_version, COUNT(*) FROM videos GROUP BY key_version ORDER BY key_version
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[int]int)
	for rows.Next() {
		var version, count int
		if err := rows.Scan(&version, &count); err != nil {
			return nil, err
		}
		counts[version] = count
	}
	return counts, rows.Err()
}

// UpdateVideoKeyVersion updates the key version and file path for a video after re-encryption
func (db *DB) UpdateVideoKeyVersion(videoID string, newKeyVersion int, newFilePath string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `
		UPDATE videos SET key_version = ?, file_path = ? WHERE id = ?
	`, newKeyVersion, newFilePath, videoID)
	return err
}

// UpdateVideoThumbnailKeyVersion updates the thumbnail path for a video after re-encryption
func (db *DB) UpdateVideoThumbnailKeyVersion(videoID string, newThumbnailPath string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `
		UPDATE videos SET thumbnail_path = ? WHERE id = ?
	`, newThumbnailPath, videoID)
	return err
}
