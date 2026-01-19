package db

import (
	"database/sql"
	"time"
)

// Job represents a transcription job
type Job struct {
	ID               int64      `json:"id"`
	UserID           int64      `json:"user_id"`
	Status           string     `json:"status"`
	OriginalFilename string     `json:"original_filename"`
	FilePath         string     `json:"file_path"`
	FileSize         int64      `json:"file_size"`
	OutputFormat     string     `json:"output_format"`
	Language         *string    `json:"language,omitempty"` // ISO 639-1 code, nil = auto-detect
	TranscriptPath   *string    `json:"transcript_path,omitempty"`
	ErrorMessage     *string    `json:"error_message,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
}

// CreateJob creates a new job record (accepts a Job struct)
func (db *DB) CreateJob(job *Job) error {
	query := `
		INSERT INTO jobs (user_id, status, original_filename, file_path, file_size, output_format, language)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	// Set default status if not provided
	if job.Status == "" {
		job.Status = "pending"
	}

	result, err := db.Exec(query, job.UserID, job.Status, job.OriginalFilename, job.FilePath, job.FileSize, job.OutputFormat, job.Language)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	// Update job with the new ID
	job.ID = id

	// Fetch the created job to populate timestamps
	createdJob, err := db.GetJobByID(id)
	if err != nil {
		return err
	}

	// Copy timestamps back
	job.CreatedAt = createdJob.CreatedAt
	job.UpdatedAt = createdJob.UpdatedAt

	return nil
}

// UpdateJobFilePath updates a job's file path
func (db *DB) UpdateJobFilePath(id int64, filePath string) error {
	query := `
		UPDATE jobs
		SET file_path = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	_, err := db.Exec(query, filePath, id)
	return err
}

// GetJobByID retrieves a job by ID
func (db *DB) GetJobByID(id int64) (*Job, error) {
	query := `
		SELECT id, user_id, status, original_filename, file_path, file_size,
		       output_format, language, transcript_path, error_message, created_at,
		       updated_at, completed_at
		FROM jobs
		WHERE id = ?
	`

	job := &Job{}
	err := db.QueryRow(query, id).Scan(
		&job.ID, &job.UserID, &job.Status, &job.OriginalFilename,
		&job.FilePath, &job.FileSize, &job.OutputFormat, &job.Language,
		&job.TranscriptPath, &job.ErrorMessage,
		&job.CreatedAt, &job.UpdatedAt, &job.CompletedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return job, nil
}

// GetJobsByUserID retrieves all jobs for a user, ordered by created_at descending
func (db *DB) GetJobsByUserID(userID int64) ([]*Job, error) {
	query := `
		SELECT id, user_id, status, original_filename, file_path, file_size,
		       output_format, language, transcript_path, error_message, created_at,
		       updated_at, completed_at
		FROM jobs
		WHERE user_id = ?
		ORDER BY created_at DESC
	`

	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := []*Job{}
	for rows.Next() {
		job := &Job{}
		err := rows.Scan(
			&job.ID, &job.UserID, &job.Status, &job.OriginalFilename,
			&job.FilePath, &job.FileSize, &job.OutputFormat, &job.Language,
			&job.TranscriptPath, &job.ErrorMessage,
			&job.CreatedAt, &job.UpdatedAt, &job.CompletedAt,
		)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return jobs, nil
}

// UpdateJobStatus updates a job's status
func (db *DB) UpdateJobStatus(id int64, status string) error {
	query := `
		UPDATE jobs
		SET status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	_, err := db.Exec(query, status, id)
	return err
}

// UpdateJobCompleted marks a job as completed with transcript path
func (db *DB) UpdateJobCompleted(id int64, transcriptPath string) error {
	query := `
		UPDATE jobs
		SET status = 'completed',
		    transcript_path = ?,
		    completed_at = CURRENT_TIMESTAMP,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	_, err := db.Exec(query, transcriptPath, id)
	return err
}

// UpdateJobFailed marks a job as failed with error message
func (db *DB) UpdateJobFailed(id int64, errorMessage string) error {
	query := `
		UPDATE jobs
		SET status = 'failed',
		    error_message = ?,
		    completed_at = CURRENT_TIMESTAMP,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	_, err := db.Exec(query, errorMessage, id)
	return err
}

// GetOldJobs retrieves all jobs older than maxAgeDays
// Only returns completed or failed jobs (not pending or processing)
func (db *DB) GetOldJobs(maxAgeDays int) ([]*Job, error) {
	query := `
		SELECT id, user_id, status, original_filename, file_path, file_size,
		       output_format, language, transcript_path, error_message, created_at,
		       updated_at, completed_at
		FROM jobs
		WHERE created_at < datetime('now', '-' || ? || ' days')
		  AND status IN ('completed', 'failed')
		ORDER BY created_at ASC
	`

	rows, err := db.Query(query, maxAgeDays)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := []*Job{}
	for rows.Next() {
		job := &Job{}
		err := rows.Scan(
			&job.ID, &job.UserID, &job.Status, &job.OriginalFilename,
			&job.FilePath, &job.FileSize, &job.OutputFormat, &job.Language,
			&job.TranscriptPath, &job.ErrorMessage,
			&job.CreatedAt, &job.UpdatedAt, &job.CompletedAt,
		)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return jobs, nil
}

// DeleteJob deletes a job record by ID
func (db *DB) DeleteJob(id int64) error {
	query := `DELETE FROM jobs WHERE id = ?`
	_, err := db.Exec(query, id)
	return err
}
