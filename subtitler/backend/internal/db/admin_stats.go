package db

import (
	"time"
)

// UserStats contains statistics about users
type UserStats struct {
	Total            int   `json:"total"`
	NewThisPeriod    int   `json:"new_this_period"`
	ActiveThisPeriod int   `json:"active_this_period"`
}

// JobStats contains statistics about jobs
type JobStats struct {
	Total     int            `json:"total"`
	Completed int            `json:"completed"`
	Failed    int            `json:"failed"`
	Pending   int            `json:"pending"`
	ByFormat  map[string]int `json:"by_format"`
}

// StorageStats contains storage usage statistics
type StorageStats struct {
	TotalBytes   int64 `json:"total_bytes"`
	UploadsBytes int64 `json:"uploads_bytes"`
	ResultsBytes int64 `json:"results_bytes"`
}

// GetUserStats retrieves user statistics for the admin dashboard
func (db *DB) GetUserStats(start, end time.Time) (*UserStats, error) {
	stats := &UserStats{}

	// Total users
	err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&stats.Total)
	if err != nil {
		return nil, err
	}

	// New users in period
	err = db.QueryRow(`
		SELECT COUNT(*) FROM users
		WHERE created_at >= ? AND created_at <= ?
	`, start, end).Scan(&stats.NewThisPeriod)
	if err != nil {
		return nil, err
	}

	// Active users (users with jobs) in period
	err = db.QueryRow(`
		SELECT COUNT(DISTINCT user_id) FROM jobs
		WHERE created_at >= ? AND created_at <= ?
	`, start, end).Scan(&stats.ActiveThisPeriod)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

// GetJobStats retrieves job statistics for the admin dashboard
func (db *DB) GetJobStats(start, end time.Time) (*JobStats, error) {
	stats := &JobStats{
		ByFormat: make(map[string]int),
	}

	// Total jobs in period
	err := db.QueryRow(`
		SELECT COUNT(*) FROM jobs
		WHERE created_at >= ? AND created_at <= ?
	`, start, end).Scan(&stats.Total)
	if err != nil {
		return nil, err
	}

	// Jobs by status
	rows, err := db.Query(`
		SELECT status, COUNT(*) FROM jobs
		WHERE created_at >= ? AND created_at <= ?
		GROUP BY status
	`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		switch status {
		case "completed":
			stats.Completed = count
		case "failed":
			stats.Failed = count
		case "pending":
			stats.Pending = count
		case "processing":
			stats.Pending += count // Count processing as pending
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}

	// Jobs by format
	rows2, err := db.Query(`
		SELECT output_format, COUNT(*) FROM jobs
		WHERE created_at >= ? AND created_at <= ?
		GROUP BY output_format
	`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows2.Close()

	for rows2.Next() {
		var format string
		var count int
		if err := rows2.Scan(&format, &count); err != nil {
			return nil, err
		}
		stats.ByFormat[format] = count
	}
	if err = rows2.Err(); err != nil {
		return nil, err
	}

	return stats, nil
}

// GetStorageStats retrieves storage usage statistics
func (db *DB) GetStorageStats() (*StorageStats, error) {
	stats := &StorageStats{}

	// Total uploads size
	var uploadsSize *int64
	err := db.QueryRow("SELECT SUM(file_size) FROM jobs").Scan(&uploadsSize)
	if err != nil {
		return nil, err
	}
	if uploadsSize != nil {
		stats.UploadsBytes = *uploadsSize
	}

	// For results, we estimate based on completed jobs
	// SRT/VTT files are typically ~1% of source file size
	// Embedded video would be similar to source size
	var srtVttSize *int64
	err = db.QueryRow(`
		SELECT SUM(file_size) / 100 FROM jobs
		WHERE status = 'completed' AND output_format IN ('srt', 'vtt')
	`).Scan(&srtVttSize)
	if err != nil {
		return nil, err
	}
	if srtVttSize != nil {
		stats.ResultsBytes = *srtVttSize
	}

	var embeddedSize *int64
	err = db.QueryRow(`
		SELECT SUM(file_size) FROM jobs
		WHERE status = 'completed' AND output_format = 'embedded'
	`).Scan(&embeddedSize)
	if err != nil {
		return nil, err
	}
	if embeddedSize != nil {
		stats.ResultsBytes += *embeddedSize
	}

	stats.TotalBytes = stats.UploadsBytes + stats.ResultsBytes

	return stats, nil
}
