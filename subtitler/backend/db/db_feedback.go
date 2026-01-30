package db

import (
	"database/sql"
	"fmt"
	"time"
)

// CreateFeedback creates a new feedback record
func (db *DB) CreateFeedback(feedback *Feedback) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `
		INSERT INTO feedback (id, user_id, session_id, video_id, page_url, feedback_text, rating, feedback_type, browser_info, created_at, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, feedback.ID, feedback.UserID, feedback.SessionID, feedback.VideoID, feedback.PageURL, feedback.Text, feedback.Rating, feedback.Type, feedback.BrowserInfo, feedback.CreatedAt, feedback.Status)
	if err != nil {
		return fmt.Errorf("failed to create feedback: %w", err)
	}
	return nil
}

// GetFeedback retrieves feedback by ID
func (db *DB) GetFeedback(id string) (*Feedback, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	feedback := &Feedback{}
	err := db.conn.QueryRowContext(ctx, `
		SELECT id, user_id, session_id, video_id, page_url, feedback_text, rating, feedback_type, browser_info, created_at, status
		FROM feedback WHERE id = ?
	`, id).Scan(&feedback.ID, &feedback.UserID, &feedback.SessionID, &feedback.VideoID, &feedback.PageURL, &feedback.Text, &feedback.Rating, &feedback.Type, &feedback.BrowserInfo, &feedback.CreatedAt, &feedback.Status)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get feedback %s: %w", id, err)
	}
	return feedback, nil
}

// MaxFeedbackLimit is the maximum number of feedback items to return in a single query
const MaxFeedbackLimit = 100

// ListFeedback retrieves feedback with optional filters.
// If after is non-empty, only feedback created after that ISO timestamp is returned.
func (db *DB) ListFeedback(status string, feedbackType string, limit, offset int, after string) ([]*Feedback, int, error) {
	// Enforce maximum limit to prevent memory exhaustion
	if limit <= 0 {
		limit = 50 // default
	} else if limit > MaxFeedbackLimit {
		limit = MaxFeedbackLimit
	}
	if offset < 0 {
		offset = 0
	}

	ctx, cancel := db.queryContext()
	defer cancel()

	// Build query with filters
	query := "SELECT id, user_id, session_id, video_id, page_url, feedback_text, rating, feedback_type, browser_info, created_at, status FROM feedback WHERE 1=1"
	countQuery := "SELECT COUNT(*) FROM feedback WHERE 1=1"
	args := []interface{}{}

	if status != "" {
		query += " AND status = ?"
		countQuery += " AND status = ?"
		args = append(args, status)
	}
	if feedbackType != "" {
		query += " AND feedback_type = ?"
		countQuery += " AND feedback_type = ?"
		args = append(args, feedbackType)
	}
	if after != "" {
		// Validate the after timestamp is valid RFC3339.
		parsedAfter, err := time.Parse(time.RFC3339, after)
		if err != nil {
			parsedAfter, err = time.Parse(time.RFC3339Nano, after)
		}
		if err != nil {
			return nil, 0, fmt.Errorf("invalid after timestamp: %w", err)
		}
		// Use SQLite's datetime() on both sides to normalize formats.
		// go-sqlite3 stores time.Time as RFC3339Nano with timezone offset (e.g. "2026-01-30T00:05:07.123-08:00")
		// but formats time.Time parameters differently (e.g. "2026-01-30 09:05:07+00:00").
		// SQLite's datetime() normalizes both to "YYYY-MM-DD HH:MM:SS" in UTC for correct comparison.
		query += " AND datetime(created_at) > datetime(?)"
		countQuery += " AND datetime(created_at) > datetime(?)"
		args = append(args, parsedAfter.UTC().Format("2006-01-02 15:04:05"))
	}

	// Get total count
	var total int
	if err := db.conn.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count feedback: %w", err)
	}

	// Add ordering and pagination
	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.conn.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list feedback: %w", err)
	}
	defer rows.Close()

	var feedbackList []*Feedback
	for rows.Next() {
		f := &Feedback{}
		if err := rows.Scan(&f.ID, &f.UserID, &f.SessionID, &f.VideoID, &f.PageURL, &f.Text, &f.Rating, &f.Type, &f.BrowserInfo, &f.CreatedAt, &f.Status); err != nil {
			return nil, 0, fmt.Errorf("failed to scan feedback row: %w", err)
		}
		feedbackList = append(feedbackList, f)
	}

	return feedbackList, total, rows.Err()
}

// UpdateFeedbackStatus updates the status of a feedback record
func (db *DB) UpdateFeedbackStatus(id, status string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	result, err := db.conn.ExecContext(ctx, `
		UPDATE feedback SET status = ? WHERE id = ?
	`, status, id)
	if err != nil {
		return fmt.Errorf("failed to update feedback status: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("feedback not found: %s", id)
	}
	return nil
}
