package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// CreateTranscription creates a new transcription record
func (db *DB) CreateTranscription(t *Transcription) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `
		INSERT INTO transcriptions (id, video_id, status, message, progress, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, t.ID, t.VideoID, t.Status, t.Message, t.Progress, t.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create transcription for video %s: %w", t.VideoID, err)
	}
	return nil
}

// GetTranscription retrieves a transcription by video ID
func (db *DB) GetTranscription(videoID string) (*Transcription, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	t := &Transcription{}
	var segmentsJSON sql.NullString
	var language, fullText, message sql.NullString
	var duration sql.NullFloat64
	var completedAt sql.NullTime

	err := db.conn.QueryRowContext(ctx, `
		SELECT id, video_id, status, message, progress, language, duration, full_text, segments_json, created_at, completed_at
		FROM transcriptions WHERE video_id = ?
	`, videoID).Scan(&t.ID, &t.VideoID, &t.Status, &message, &t.Progress, &language, &duration, &fullText, &segmentsJSON, &t.CreatedAt, &completedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get transcription for video %s: %w", videoID, err)
	}

	if message.Valid {
		t.Message = message.String
	}
	if language.Valid {
		t.Language = language.String
	}
	if duration.Valid {
		t.Duration = duration.Float64
	}
	if fullText.Valid {
		t.FullText = fullText.String
	}
	if segmentsJSON.Valid {
		t.SegmentsJSON = segmentsJSON.String
	}
	if completedAt.Valid {
		t.CompletedAt = &completedAt.Time
	}

	return t, nil
}

// UpdateTranscriptionStatus updates the status and message of a transcription.
// Only updates if the current status is 'pending' or 'processing' to avoid race conditions
// with completion/failure updates from the main goroutine. Once status becomes 'complete'
// or 'error', progress updates are blocked.
func (db *DB) UpdateTranscriptionStatus(videoID, status, message string, progress int) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `
		UPDATE transcriptions SET status = ?, message = ?, progress = ?
		WHERE video_id = ? AND status IN ('pending', 'processing')
	`, status, message, progress, videoID)
	if err != nil {
		return fmt.Errorf("failed to update transcription status for video %s: %w", videoID, err)
	}
	return nil
}

// CompleteTranscription marks a transcription as complete with results
func (db *DB) CompleteTranscription(videoID string, language string, duration float64, fullText string, segments []Segment) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	segmentsJSON, err := json.Marshal(segments)
	if err != nil {
		return fmt.Errorf("failed to marshal segments: %w", err)
	}

	now := time.Now()
	_, err = db.conn.ExecContext(ctx, `
		UPDATE transcriptions
		SET status = 'complete', message = 'Transcription complete', progress = 100,
		    language = ?, duration = ?, full_text = ?, segments_json = ?, completed_at = ?
		WHERE video_id = ?
	`, language, duration, fullText, string(segmentsJSON), now, videoID)
	if err != nil {
		return fmt.Errorf("failed to complete transcription for video %s: %w", videoID, err)
	}
	return nil
}

// FailTranscription marks a transcription as failed with an error message
func (db *DB) FailTranscription(videoID, errorMessage string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `
		UPDATE transcriptions SET status = 'error', message = ? WHERE video_id = ?
	`, errorMessage, videoID)
	if err != nil {
		return fmt.Errorf("failed to mark transcription as failed for video %s: %w", videoID, err)
	}
	return nil
}

// GetSegments parses and returns the segments from a transcription
func (t *Transcription) GetSegments() ([]Segment, error) {
	if t.SegmentsJSON == "" {
		return nil, nil
	}
	var segments []Segment
	if err := json.Unmarshal([]byte(t.SegmentsJSON), &segments); err != nil {
		return nil, fmt.Errorf("failed to parse segments JSON: %w", err)
	}
	return segments, nil
}

// UpdateSegments updates the segments for a transcription
func (db *DB) UpdateSegments(videoID string, segments []Segment) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	segmentsJSON, err := json.Marshal(segments)
	if err != nil {
		return fmt.Errorf("failed to marshal segments: %w", err)
	}

	// Also update the full_text by concatenating all segment texts
	var fullText string
	for i, seg := range segments {
		if i > 0 {
			fullText += " "
		}
		fullText += seg.Text
	}

	_, err = db.conn.ExecContext(ctx, `
		UPDATE transcriptions
		SET segments_json = ?, full_text = ?
		WHERE video_id = ?
	`, string(segmentsJSON), fullText, videoID)
	if err != nil {
		return fmt.Errorf("failed to update segments for video %s: %w", videoID, err)
	}
	return nil
}
