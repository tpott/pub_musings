package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// DB wraps the SQLite database connection
type DB struct {
	conn *sql.DB
}

// Video represents an uploaded video
type Video struct {
	ID            string    `json:"id"`
	Filename      string    `json:"filename"`
	Size          int64     `json:"size"`
	ContentType   string    `json:"content_type"`
	FilePath      string    `json:"file_path"`
	ThumbnailPath *string   `json:"thumbnail_path,omitempty"` // path to encrypted thumbnail image
	CreatedAt     time.Time `json:"created_at"`
	UserID        *string   `json:"user_id,omitempty"` // null for anonymous uploads
	SessionID     *string   `json:"session_id,omitempty"`
}

// Transcription represents a transcription job and its result
type Transcription struct {
	ID           string     `json:"id"`
	VideoID      string     `json:"video_id"`
	Status       string     `json:"status"` // pending, processing, complete, error
	Message      string     `json:"message,omitempty"`
	Progress     int        `json:"progress"`
	Language     string     `json:"language,omitempty"`
	Duration     float64    `json:"duration,omitempty"`
	FullText     string     `json:"full_text,omitempty"`
	SegmentsJSON string     `json:"-"` // JSON-encoded segments
	CreatedAt    time.Time  `json:"created_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

// Segment represents a single subtitle segment
type Segment struct {
	ID    int     `json:"id"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

// User represents a registered user
type User struct {
	ID            string     `json:"id"`
	Email         string     `json:"email"`
	PasswordHash  string     `json:"-"` // Never serialize
	TOTPSecret    *string    `json:"-"` // Never serialize, nil if 2FA not set up
	TOTPEnabled   bool       `json:"totp_enabled"`
	EmailVerified bool       `json:"email_verified"`
	VerifiedAt    *time.Time `json:"verified_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

// Session represents an authenticated session
type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Token     string    `json:"-"` // Never serialize token in JSON
	IPAddress string    `json:"ip_address,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// BurnJob represents a subtitle burning job
type BurnJob struct {
	ID          string     `json:"id"`
	VideoID     string     `json:"video_id"`
	Status      string     `json:"status"` // pending, processing, complete, error
	Message     string     `json:"message,omitempty"`
	Progress    int        `json:"progress"`
	OutputPath  string     `json:"output_path,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// RecoveryCode represents a hashed 2FA recovery code
type RecoveryCode struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	CodeHash  string     `json:"-"` // Never serialize
	Used      bool       `json:"used"`
	CreatedAt time.Time  `json:"created_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
}

// PasswordResetToken represents a password reset request token
type PasswordResetToken struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	TokenHash string    `json:"-"` // Never serialize, SHA-256 hash of actual token
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
	CreatedAt time.Time `json:"created_at"`
}

// EmailVerificationToken represents an email verification token
type EmailVerificationToken struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	TokenHash string    `json:"-"` // Never serialize, SHA-256 hash of actual token
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
	CreatedAt time.Time `json:"created_at"`
}

// MagicLinkToken represents a passwordless login token
type MagicLinkToken struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	TokenHash string    `json:"-"` // Never serialize, SHA-256 hash of actual token
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
	CreatedAt time.Time `json:"created_at"`
}

// LoginAttempt tracks failed login attempts for rate limiting
type LoginAttempt struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Success   bool      `json:"success"`
	IPAddress string    `json:"ip_address,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Open opens or creates a SQLite database at the given path
func Open(dbPath string) (*DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	conn, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &DB{conn: conn}

	// Run migrations
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return db, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.conn.Close()
}

// Ping checks if the database connection is alive
func (db *DB) Ping() error {
	return db.conn.Ping()
}

// Tx wraps a database transaction for use within the transaction callback
type Tx struct {
	tx *sql.Tx
}

// WithTransaction executes the given function within a database transaction.
// If the function returns an error, the transaction is rolled back.
// If the function succeeds, the transaction is committed.
func (db *DB) WithTransaction(fn func(*Tx) error) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	txWrapper := &Tx{tx: tx}
	if err := fn(txWrapper); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback failed: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// migrate runs database migrations using the file-based migration system
func (db *DB) migrate() error {
	migrator, err := NewMigrator(db.conn)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}

	// Check if this is an existing database without schema_migrations table
	// If the videos table exists but schema_migrations doesn't, mark migration 1 as applied
	if err := db.handleExistingDatabase(migrator); err != nil {
		return fmt.Errorf("failed to handle existing database: %w", err)
	}

	// Run all pending migrations
	if err := migrator.Up(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// handleExistingDatabase handles the case where a database already has tables
// but no schema_migrations table (pre-migration system database)
func (db *DB) handleExistingDatabase(migrator *Migrator) error {
	// Check if schema_migrations table exists
	var tableName string
	err := db.conn.QueryRow(`
		SELECT name FROM sqlite_master
		WHERE type='table' AND name='schema_migrations'
	`).Scan(&tableName)

	if err == nil {
		// schema_migrations exists, nothing to do
		return nil
	}

	// Check if videos table exists (indicates pre-migration database)
	err = db.conn.QueryRow(`
		SELECT name FROM sqlite_master
		WHERE type='table' AND name='videos'
	`).Scan(&tableName)

	if err != nil {
		// No videos table means fresh database, migrations will create everything
		return nil
	}

	// This is a pre-migration database - we need to create schema_migrations
	// and mark the initial migration as applied
	_, err = db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			description TEXT NOT NULL,
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	// Mark migration 1 (initial_schema) as applied since tables already exist
	_, err = db.conn.Exec(`
		INSERT INTO schema_migrations (version, description, applied_at)
		VALUES (1, 'initial_schema', CURRENT_TIMESTAMP)
	`)
	if err != nil {
		return fmt.Errorf("failed to mark initial migration as applied: %w", err)
	}

	return nil
}

// CreateVideo creates a new video record
func (db *DB) CreateVideo(video *Video) error {
	_, err := db.conn.Exec(`
		INSERT INTO videos (id, filename, size, content_type, file_path, thumbnail_path, created_at, user_id, session_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, video.ID, video.Filename, video.Size, video.ContentType, video.FilePath, video.ThumbnailPath, video.CreatedAt, video.UserID, video.SessionID)
	return err
}

// GetVideo retrieves a video by ID
func (db *DB) GetVideo(id string) (*Video, error) {
	video := &Video{}
	err := db.conn.QueryRow(`
		SELECT id, filename, size, content_type, file_path, thumbnail_path, created_at, user_id, session_id
		FROM videos WHERE id = ?
	`, id).Scan(&video.ID, &video.Filename, &video.Size, &video.ContentType, &video.FilePath, &video.ThumbnailPath, &video.CreatedAt, &video.UserID, &video.SessionID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return video, nil
}

// CreateTranscription creates a new transcription record
func (db *DB) CreateTranscription(t *Transcription) error {
	_, err := db.conn.Exec(`
		INSERT INTO transcriptions (id, video_id, status, message, progress, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, t.ID, t.VideoID, t.Status, t.Message, t.Progress, t.CreatedAt)
	return err
}

// GetTranscription retrieves a transcription by video ID
func (db *DB) GetTranscription(videoID string) (*Transcription, error) {
	t := &Transcription{}
	var segmentsJSON sql.NullString
	var language, fullText, message sql.NullString
	var duration sql.NullFloat64
	var completedAt sql.NullTime

	err := db.conn.QueryRow(`
		SELECT id, video_id, status, message, progress, language, duration, full_text, segments_json, created_at, completed_at
		FROM transcriptions WHERE video_id = ?
	`, videoID).Scan(&t.ID, &t.VideoID, &t.Status, &message, &t.Progress, &language, &duration, &fullText, &segmentsJSON, &t.CreatedAt, &completedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
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

// UpdateTranscriptionStatus updates the status and message of a transcription
func (db *DB) UpdateTranscriptionStatus(videoID, status, message string, progress int) error {
	_, err := db.conn.Exec(`
		UPDATE transcriptions SET status = ?, message = ?, progress = ? WHERE video_id = ?
	`, status, message, progress, videoID)
	return err
}

// CompleteTranscription marks a transcription as complete with results
func (db *DB) CompleteTranscription(videoID string, language string, duration float64, fullText string, segments []Segment) error {
	segmentsJSON, err := json.Marshal(segments)
	if err != nil {
		return fmt.Errorf("failed to marshal segments: %w", err)
	}

	now := time.Now()
	_, err = db.conn.Exec(`
		UPDATE transcriptions
		SET status = 'complete', message = 'Transcription complete', progress = 100,
		    language = ?, duration = ?, full_text = ?, segments_json = ?, completed_at = ?
		WHERE video_id = ?
	`, language, duration, fullText, string(segmentsJSON), now, videoID)
	return err
}

// FailTranscription marks a transcription as failed with an error message
func (db *DB) FailTranscription(videoID, errorMessage string) error {
	_, err := db.conn.Exec(`
		UPDATE transcriptions SET status = 'error', message = ? WHERE video_id = ?
	`, errorMessage, videoID)
	return err
}

// GetSegments parses and returns the segments from a transcription
func (t *Transcription) GetSegments() ([]Segment, error) {
	if t.SegmentsJSON == "" {
		return nil, nil
	}
	var segments []Segment
	if err := json.Unmarshal([]byte(t.SegmentsJSON), &segments); err != nil {
		return nil, err
	}
	return segments, nil
}

// ListVideos returns all videos, optionally filtered by user or session
func (db *DB) ListVideos(userID, sessionID *string) ([]Video, error) {
	var rows *sql.Rows
	var err error

	if userID != nil {
		rows, err = db.conn.Query(`
			SELECT id, filename, size, content_type, file_path, thumbnail_path, created_at, user_id, session_id
			FROM videos WHERE user_id = ? ORDER BY created_at DESC
		`, *userID)
	} else if sessionID != nil {
		rows, err = db.conn.Query(`
			SELECT id, filename, size, content_type, file_path, thumbnail_path, created_at, user_id, session_id
			FROM videos WHERE session_id = ? ORDER BY created_at DESC
		`, *sessionID)
	} else {
		rows, err = db.conn.Query(`
			SELECT id, filename, size, content_type, file_path, thumbnail_path, created_at, user_id, session_id
			FROM videos ORDER BY created_at DESC
		`)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []Video
	for rows.Next() {
		var v Video
		if err := rows.Scan(&v.ID, &v.Filename, &v.Size, &v.ContentType, &v.FilePath, &v.ThumbnailPath, &v.CreatedAt, &v.UserID, &v.SessionID); err != nil {
			return nil, err
		}
		videos = append(videos, v)
	}

	return videos, rows.Err()
}

// CountVideosBySession returns the number of videos uploaded by a session
func (db *DB) CountVideosBySession(sessionID string) (int, error) {
	var count int
	err := db.conn.QueryRow(`
		SELECT COUNT(*) FROM videos WHERE session_id = ?
	`, sessionID).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// CreateUser creates a new user record
func (db *DB) CreateUser(user *User) error {
	_, err := db.conn.Exec(`
		INSERT INTO users (id, email, password_hash, totp_secret, totp_enabled, email_verified, verified_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, user.ID, user.Email, user.PasswordHash, user.TOTPSecret, user.TOTPEnabled, user.EmailVerified, user.VerifiedAt, user.CreatedAt)
	return err
}

// GetUserByID retrieves a user by ID
func (db *DB) GetUserByID(id string) (*User, error) {
	user := &User{}
	var totpSecret sql.NullString
	var verifiedAt sql.NullTime
	err := db.conn.QueryRow(`
		SELECT id, email, password_hash, totp_secret, totp_enabled, email_verified, verified_at, created_at
		FROM users WHERE id = ?
	`, id).Scan(&user.ID, &user.Email, &user.PasswordHash, &totpSecret, &user.TOTPEnabled, &user.EmailVerified, &verifiedAt, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if totpSecret.Valid {
		user.TOTPSecret = &totpSecret.String
	}
	if verifiedAt.Valid {
		user.VerifiedAt = &verifiedAt.Time
	}
	return user, nil
}

// GetUserByEmail retrieves a user by email
func (db *DB) GetUserByEmail(email string) (*User, error) {
	user := &User{}
	var totpSecret sql.NullString
	var verifiedAt sql.NullTime
	err := db.conn.QueryRow(`
		SELECT id, email, password_hash, totp_secret, totp_enabled, email_verified, verified_at, created_at
		FROM users WHERE email = ?
	`, email).Scan(&user.ID, &user.Email, &user.PasswordHash, &totpSecret, &user.TOTPEnabled, &user.EmailVerified, &verifiedAt, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if totpSecret.Valid {
		user.TOTPSecret = &totpSecret.String
	}
	if verifiedAt.Valid {
		user.VerifiedAt = &verifiedAt.Time
	}
	return user, nil
}

// CreateSession creates a new session record
func (db *DB) CreateSession(session *Session) error {
	_, err := db.conn.Exec(`
		INSERT INTO sessions (id, user_id, token, ip_address, user_agent, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, session.ID, session.UserID, session.Token, session.IPAddress, session.UserAgent, session.ExpiresAt, session.CreatedAt)
	return err
}

// GetSessionByToken retrieves a session by token
func (db *DB) GetSessionByToken(token string) (*Session, error) {
	session := &Session{}
	var ipAddress, userAgent sql.NullString
	err := db.conn.QueryRow(`
		SELECT id, user_id, token, ip_address, user_agent, expires_at, created_at
		FROM sessions WHERE token = ?
	`, token).Scan(&session.ID, &session.UserID, &session.Token, &ipAddress, &userAgent, &session.ExpiresAt, &session.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if ipAddress.Valid {
		session.IPAddress = ipAddress.String
	}
	if userAgent.Valid {
		session.UserAgent = userAgent.String
	}
	return session, nil
}

// GetSessionsByUserID retrieves all active sessions for a user
func (db *DB) GetSessionsByUserID(userID string) ([]Session, error) {
	rows, err := db.conn.Query(`
		SELECT id, user_id, token, ip_address, user_agent, expires_at, created_at
		FROM sessions
		WHERE user_id = ? AND expires_at > ?
		ORDER BY created_at DESC
	`, userID, time.Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var session Session
		var ipAddress, userAgent sql.NullString
		if err := rows.Scan(&session.ID, &session.UserID, &session.Token, &ipAddress, &userAgent, &session.ExpiresAt, &session.CreatedAt); err != nil {
			return nil, err
		}
		if ipAddress.Valid {
			session.IPAddress = ipAddress.String
		}
		if userAgent.Valid {
			session.UserAgent = userAgent.String
		}
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

// DeleteSessionByID deletes a session by its ID (for session management)
func (db *DB) DeleteSessionByID(sessionID, userID string) error {
	// Require userID to ensure users can only delete their own sessions
	result, err := db.conn.Exec(`DELETE FROM sessions WHERE id = ? AND user_id = ?`, sessionID, userID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("session not found or not owned by user")
	}
	return nil
}

// DeleteSession deletes a session by token
func (db *DB) DeleteSession(token string) error {
	_, err := db.conn.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}

// DeleteExpiredSessions deletes all expired sessions
func (db *DB) DeleteExpiredSessions() (int64, error) {
	result, err := db.conn.Exec(`DELETE FROM sessions WHERE expires_at < ?`, time.Now())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// DeleteUserSessions deletes all sessions for a user
func (db *DB) DeleteUserSessions(userID string) error {
	_, err := db.conn.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
}

// SetTOTPSecret sets the TOTP secret for a user (during 2FA setup)
func (db *DB) SetTOTPSecret(userID, secret string) error {
	_, err := db.conn.Exec(`
		UPDATE users SET totp_secret = ? WHERE id = ?
	`, secret, userID)
	return err
}

// EnableTOTP enables 2FA for a user (after verification)
func (db *DB) EnableTOTP(userID string) error {
	_, err := db.conn.Exec(`
		UPDATE users SET totp_enabled = 1 WHERE id = ?
	`, userID)
	return err
}

// DisableTOTP disables 2FA and clears the secret for a user
func (db *DB) DisableTOTP(userID string) error {
	_, err := db.conn.Exec(`
		UPDATE users SET totp_enabled = 0, totp_secret = NULL WHERE id = ?
	`, userID)
	return err
}

// GetExpiredVideos returns videos that have exceeded their retention period.
// Anonymous videos (no user_id) expire after 48 hours.
// Registered user videos expire after 90 days.
func (db *DB) GetExpiredVideos() ([]Video, error) {
	now := time.Now()
	anonymousExpiry := now.Add(-48 * time.Hour)
	registeredExpiry := now.Add(-90 * 24 * time.Hour)

	rows, err := db.conn.Query(`
		SELECT id, filename, size, content_type, file_path, thumbnail_path, created_at, user_id, session_id
		FROM videos
		WHERE (user_id IS NULL AND created_at < ?)
		   OR (user_id IS NOT NULL AND created_at < ?)
	`, anonymousExpiry, registeredExpiry)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var videos []Video
	for rows.Next() {
		var v Video
		if err := rows.Scan(&v.ID, &v.Filename, &v.Size, &v.ContentType, &v.FilePath, &v.ThumbnailPath, &v.CreatedAt, &v.UserID, &v.SessionID); err != nil {
			return nil, err
		}
		videos = append(videos, v)
	}

	return videos, rows.Err()
}

// UpdateSegments updates the segments for a transcription
func (db *DB) UpdateSegments(videoID string, segments []Segment) error {
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

	_, err = db.conn.Exec(`
		UPDATE transcriptions
		SET segments_json = ?, full_text = ?
		WHERE video_id = ?
	`, string(segmentsJSON), fullText, videoID)
	return err
}

// CreateBurnJob creates a new burn job record
func (db *DB) CreateBurnJob(job *BurnJob) error {
	_, err := db.conn.Exec(`
		INSERT INTO burn_jobs (id, video_id, status, message, progress, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, job.ID, job.VideoID, job.Status, job.Message, job.Progress, job.CreatedAt)
	return err
}

// GetBurnJob retrieves a burn job by video ID
func (db *DB) GetBurnJob(videoID string) (*BurnJob, error) {
	job := &BurnJob{}
	var message, outputPath sql.NullString
	var completedAt sql.NullTime

	err := db.conn.QueryRow(`
		SELECT id, video_id, status, message, progress, output_path, created_at, completed_at
		FROM burn_jobs WHERE video_id = ? ORDER BY created_at DESC LIMIT 1
	`, videoID).Scan(&job.ID, &job.VideoID, &job.Status, &message, &job.Progress, &outputPath, &job.CreatedAt, &completedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if message.Valid {
		job.Message = message.String
	}
	if outputPath.Valid {
		job.OutputPath = outputPath.String
	}
	if completedAt.Valid {
		job.CompletedAt = &completedAt.Time
	}

	return job, nil
}

// UpdateBurnJobStatus updates the status and message of a burn job
func (db *DB) UpdateBurnJobStatus(videoID, status, message string, progress int) error {
	_, err := db.conn.Exec(`
		UPDATE burn_jobs SET status = ?, message = ?, progress = ? WHERE video_id = ?
	`, status, message, progress, videoID)
	return err
}

// CompleteBurnJob marks a burn job as complete with the output file path
func (db *DB) CompleteBurnJob(videoID, outputPath string) error {
	now := time.Now()
	_, err := db.conn.Exec(`
		UPDATE burn_jobs
		SET status = 'complete', message = 'Subtitles burned successfully', progress = 100,
		    output_path = ?, completed_at = ?
		WHERE video_id = ?
	`, outputPath, now, videoID)
	return err
}

// FailBurnJob marks a burn job as failed with an error message
func (db *DB) FailBurnJob(videoID, errorMessage string) error {
	_, err := db.conn.Exec(`
		UPDATE burn_jobs SET status = 'error', message = ? WHERE video_id = ?
	`, errorMessage, videoID)
	return err
}

// DeletedVideoFiles contains paths to files that should be deleted after a video is removed
type DeletedVideoFiles struct {
	FilePath      string  // path to the video file
	ThumbnailPath *string // path to the thumbnail file (may be nil)
}

// DeleteVideo deletes a video and its associated transcription from the database.
// Returns the file paths so the caller can delete the files from disk.
func (db *DB) DeleteVideo(videoID string) (*DeletedVideoFiles, error) {
	// Get the file paths before deleting
	video, err := db.GetVideo(videoID)
	if err != nil {
		return nil, err
	}
	if video == nil {
		return nil, nil
	}

	// Delete transcription first (foreign key constraint)
	_, err = db.conn.Exec(`DELETE FROM transcriptions WHERE video_id = ?`, videoID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete transcription: %w", err)
	}

	// Delete burn jobs
	_, err = db.conn.Exec(`DELETE FROM burn_jobs WHERE video_id = ?`, videoID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete burn jobs: %w", err)
	}

	// Delete video record
	_, err = db.conn.Exec(`DELETE FROM videos WHERE id = ?`, videoID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete video: %w", err)
	}

	return &DeletedVideoFiles{
		FilePath:      video.FilePath,
		ThumbnailPath: video.ThumbnailPath,
	}, nil
}

// UpdateVideoThumbnail updates the thumbnail path for a video
func (db *DB) UpdateVideoThumbnail(videoID, thumbnailPath string) error {
	_, err := db.conn.Exec(`UPDATE videos SET thumbnail_path = ? WHERE id = ?`, thumbnailPath, videoID)
	return err
}

// SaveRecoveryCodes stores hashed recovery codes for a user.
// Deletes any existing unused codes first.
func (db *DB) SaveRecoveryCodes(userID string, codeHashes []string) error {
	// Delete existing unused codes
	_, err := db.conn.Exec(`DELETE FROM recovery_codes WHERE user_id = ? AND used = 0`, userID)
	if err != nil {
		return fmt.Errorf("failed to delete old recovery codes: %w", err)
	}

	// Insert new codes
	for _, hash := range codeHashes {
		id, err := generateID()
		if err != nil {
			return fmt.Errorf("failed to generate recovery code ID: %w", err)
		}
		_, err = db.conn.Exec(`
			INSERT INTO recovery_codes (id, user_id, code_hash, created_at)
			VALUES (?, ?, ?, ?)
		`, id, userID, hash, time.Now())
		if err != nil {
			return fmt.Errorf("failed to insert recovery code: %w", err)
		}
	}

	return nil
}

// generateID generates a random 16-character hex ID
func generateID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate ID: %w", err)
	}
	return fmt.Sprintf("%x", b), nil
}

// GetUnusedRecoveryCodes returns all unused recovery codes for a user
func (db *DB) GetUnusedRecoveryCodes(userID string) ([]RecoveryCode, error) {
	rows, err := db.conn.Query(`
		SELECT id, user_id, code_hash, used, created_at, used_at
		FROM recovery_codes
		WHERE user_id = ? AND used = 0
		ORDER BY created_at
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var codes []RecoveryCode
	for rows.Next() {
		var c RecoveryCode
		var usedAt sql.NullTime
		if err := rows.Scan(&c.ID, &c.UserID, &c.CodeHash, &c.Used, &c.CreatedAt, &usedAt); err != nil {
			return nil, err
		}
		if usedAt.Valid {
			c.UsedAt = &usedAt.Time
		}
		codes = append(codes, c)
	}

	return codes, rows.Err()
}

// UseRecoveryCode marks a recovery code as used.
// Returns true if successful, false if code not found or already used.
func (db *DB) UseRecoveryCode(codeID string) (bool, error) {
	now := time.Now()
	result, err := db.conn.Exec(`
		UPDATE recovery_codes
		SET used = 1, used_at = ?
		WHERE id = ? AND used = 0
	`, now, codeID)
	if err != nil {
		return false, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// DeleteRecoveryCodes deletes all recovery codes for a user
func (db *DB) DeleteRecoveryCodes(userID string) error {
	_, err := db.conn.Exec(`DELETE FROM recovery_codes WHERE user_id = ?`, userID)
	return err
}

// CountUnusedRecoveryCodes returns the number of unused recovery codes for a user
func (db *DB) CountUnusedRecoveryCodes(userID string) (int, error) {
	var count int
	err := db.conn.QueryRow(`
		SELECT COUNT(*) FROM recovery_codes WHERE user_id = ? AND used = 0
	`, userID).Scan(&count)
	return count, err
}

// CreatePasswordResetToken creates a new password reset token for a user.
// Deletes any existing unused tokens for the user first.
func (db *DB) CreatePasswordResetToken(userID, tokenHash string, expiresAt time.Time) (*PasswordResetToken, error) {
	// Delete existing unused tokens for this user
	_, err := db.conn.Exec(`DELETE FROM password_reset_tokens WHERE user_id = ? AND used = 0`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete old reset tokens: %w", err)
	}

	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token ID: %w", err)
	}
	token := &PasswordResetToken{
		ID:        id,
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
		Used:      false,
		CreatedAt: time.Now(),
	}

	_, err = db.conn.Exec(`
		INSERT INTO password_reset_tokens (id, user_id, token_hash, expires_at, used, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, token.ID, token.UserID, token.TokenHash, token.ExpiresAt, 0, token.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create reset token: %w", err)
	}

	return token, nil
}

// GetPasswordResetToken retrieves a password reset token by its hash.
// Returns nil if not found.
func (db *DB) GetPasswordResetToken(tokenHash string) (*PasswordResetToken, error) {
	token := &PasswordResetToken{}
	err := db.conn.QueryRow(`
		SELECT id, user_id, token_hash, expires_at, used, created_at
		FROM password_reset_tokens
		WHERE token_hash = ?
	`, tokenHash).Scan(&token.ID, &token.UserID, &token.TokenHash, &token.ExpiresAt, &token.Used, &token.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return token, nil
}

// UsePasswordResetToken marks a password reset token as used.
// Returns true if the token was valid and unused, false otherwise.
func (db *DB) UsePasswordResetToken(tokenHash string) (bool, error) {
	result, err := db.conn.Exec(`
		UPDATE password_reset_tokens
		SET used = 1
		WHERE token_hash = ? AND used = 0 AND expires_at > ?
	`, tokenHash, time.Now())
	if err != nil {
		return false, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// DeletePasswordResetTokens deletes all password reset tokens for a user
func (db *DB) DeletePasswordResetTokens(userID string) error {
	_, err := db.conn.Exec(`DELETE FROM password_reset_tokens WHERE user_id = ?`, userID)
	return err
}

// DeleteExpiredPasswordResetTokens deletes all expired password reset tokens
func (db *DB) DeleteExpiredPasswordResetTokens() (int64, error) {
	result, err := db.conn.Exec(`DELETE FROM password_reset_tokens WHERE expires_at < ?`, time.Now())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// CreateEmailVerificationToken creates a new email verification token for a user.
// Deletes any existing unused tokens for the user first.
func (db *DB) CreateEmailVerificationToken(userID, tokenHash string, expiresAt time.Time) (*EmailVerificationToken, error) {
	// Delete existing unused tokens for this user
	_, err := db.conn.Exec(`DELETE FROM email_verification_tokens WHERE user_id = ? AND used = 0`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete old verification tokens: %w", err)
	}

	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token ID: %w", err)
	}
	token := &EmailVerificationToken{
		ID:        id,
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
		Used:      false,
		CreatedAt: time.Now(),
	}

	_, err = db.conn.Exec(`
		INSERT INTO email_verification_tokens (id, user_id, token_hash, expires_at, used, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, token.ID, token.UserID, token.TokenHash, token.ExpiresAt, 0, token.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create verification token: %w", err)
	}

	return token, nil
}

// GetEmailVerificationToken retrieves an email verification token by its hash.
// Returns nil if not found.
func (db *DB) GetEmailVerificationToken(tokenHash string) (*EmailVerificationToken, error) {
	token := &EmailVerificationToken{}
	err := db.conn.QueryRow(`
		SELECT id, user_id, token_hash, expires_at, used, created_at
		FROM email_verification_tokens
		WHERE token_hash = ?
	`, tokenHash).Scan(&token.ID, &token.UserID, &token.TokenHash, &token.ExpiresAt, &token.Used, &token.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return token, nil
}

// UseEmailVerificationToken marks an email verification token as used and verifies the user's email.
// Returns true if the token was valid and unused, false otherwise.
func (db *DB) UseEmailVerificationToken(tokenHash string) (bool, error) {
	err := db.WithTransaction(func(tx *Tx) error {
		// First, get the token to find the user ID
		var token EmailVerificationToken
		err := tx.tx.QueryRow(`
			SELECT id, user_id, token_hash, expires_at, used, created_at
			FROM email_verification_tokens
			WHERE token_hash = ? AND used = 0 AND expires_at > ?
		`, tokenHash, time.Now()).Scan(&token.ID, &token.UserID, &token.TokenHash, &token.ExpiresAt, &token.Used, &token.CreatedAt)
		if err == sql.ErrNoRows {
			return fmt.Errorf("token not found or expired")
		}
		if err != nil {
			return err
		}

		// Mark token as used
		_, err = tx.tx.Exec(`UPDATE email_verification_tokens SET used = 1 WHERE id = ?`, token.ID)
		if err != nil {
			return fmt.Errorf("failed to mark token as used: %w", err)
		}

		// Verify the user's email
		now := time.Now()
		_, err = tx.tx.Exec(`UPDATE users SET email_verified = 1, verified_at = ? WHERE id = ?`, now, token.UserID)
		if err != nil {
			return fmt.Errorf("failed to verify user email: %w", err)
		}

		return nil
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

// VerifyUserEmail sets a user's email as verified
func (db *DB) VerifyUserEmail(userID string) error {
	now := time.Now()
	_, err := db.conn.Exec(`UPDATE users SET email_verified = 1, verified_at = ? WHERE id = ?`, now, userID)
	return err
}

// DeleteEmailVerificationTokens deletes all email verification tokens for a user
func (db *DB) DeleteEmailVerificationTokens(userID string) error {
	_, err := db.conn.Exec(`DELETE FROM email_verification_tokens WHERE user_id = ?`, userID)
	return err
}

// DeleteExpiredEmailVerificationTokens deletes all expired email verification tokens
func (db *DB) DeleteExpiredEmailVerificationTokens() (int64, error) {
	result, err := db.conn.Exec(`DELETE FROM email_verification_tokens WHERE expires_at < ?`, time.Now())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// GetUnusedEmailVerificationToken returns the most recent unused verification token for a user
func (db *DB) GetUnusedEmailVerificationToken(userID string) (*EmailVerificationToken, error) {
	token := &EmailVerificationToken{}
	err := db.conn.QueryRow(`
		SELECT id, user_id, token_hash, expires_at, used, created_at
		FROM email_verification_tokens
		WHERE user_id = ? AND used = 0 AND expires_at > ?
		ORDER BY created_at DESC
		LIMIT 1
	`, userID, time.Now()).Scan(&token.ID, &token.UserID, &token.TokenHash, &token.ExpiresAt, &token.Used, &token.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return token, nil
}

// CreateMagicLinkToken creates a new magic link token for passwordless login.
// Deletes any existing unused tokens for the user first.
func (db *DB) CreateMagicLinkToken(userID, tokenHash string, expiresAt time.Time) (*MagicLinkToken, error) {
	// Delete existing unused tokens for this user
	_, err := db.conn.Exec(`DELETE FROM magic_link_tokens WHERE user_id = ? AND used = 0`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete old magic link tokens: %w", err)
	}

	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token ID: %w", err)
	}
	token := &MagicLinkToken{
		ID:        id,
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
		Used:      false,
		CreatedAt: time.Now(),
	}

	_, err = db.conn.Exec(`
		INSERT INTO magic_link_tokens (id, user_id, token_hash, expires_at, used, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, token.ID, token.UserID, token.TokenHash, token.ExpiresAt, 0, token.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create magic link token: %w", err)
	}

	return token, nil
}

// GetMagicLinkToken retrieves a magic link token by its hash.
// Returns nil if not found.
func (db *DB) GetMagicLinkToken(tokenHash string) (*MagicLinkToken, error) {
	token := &MagicLinkToken{}
	err := db.conn.QueryRow(`
		SELECT id, user_id, token_hash, expires_at, used, created_at
		FROM magic_link_tokens
		WHERE token_hash = ?
	`, tokenHash).Scan(&token.ID, &token.UserID, &token.TokenHash, &token.ExpiresAt, &token.Used, &token.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return token, nil
}

// UseMagicLinkToken marks a magic link token as used.
// Returns true if the token was valid and unused, false otherwise.
func (db *DB) UseMagicLinkToken(tokenHash string) (bool, error) {
	result, err := db.conn.Exec(`
		UPDATE magic_link_tokens
		SET used = 1
		WHERE token_hash = ? AND used = 0 AND expires_at > ?
	`, tokenHash, time.Now())
	if err != nil {
		return false, err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return affected > 0, nil
}

// DeleteMagicLinkTokens deletes all magic link tokens for a user
func (db *DB) DeleteMagicLinkTokens(userID string) error {
	_, err := db.conn.Exec(`DELETE FROM magic_link_tokens WHERE user_id = ?`, userID)
	return err
}

// DeleteExpiredMagicLinkTokens deletes all expired magic link tokens
func (db *DB) DeleteExpiredMagicLinkTokens() (int64, error) {
	result, err := db.conn.Exec(`DELETE FROM magic_link_tokens WHERE expires_at < ?`, time.Now())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// UpdateUserPassword updates a user's password hash
func (db *DB) UpdateUserPassword(userID, passwordHash string) error {
	_, err := db.conn.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, userID)
	return err
}

// RecordLoginAttempt records a login attempt for rate limiting
func (db *DB) RecordLoginAttempt(email, ipAddress string, success bool) error {
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return err
	}

	successInt := 0
	if success {
		successInt = 1
	}

	_, err := db.conn.Exec(`
		INSERT INTO login_attempts (id, email, success, ip_address, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, fmt.Sprintf("%x", id), email, successInt, ipAddress, time.Now())
	return err
}

// GetRecentFailedLoginAttempts returns the number of failed login attempts for an email
// in the given time window
func (db *DB) GetRecentFailedLoginAttempts(email string, since time.Time) (int, error) {
	var count int
	err := db.conn.QueryRow(`
		SELECT COUNT(*) FROM login_attempts
		WHERE email = ? AND success = 0 AND created_at > ?
	`, email, since).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// ClearLoginAttempts clears login attempts for an email (e.g., after successful login)
func (db *DB) ClearLoginAttempts(email string) error {
	_, err := db.conn.Exec(`DELETE FROM login_attempts WHERE email = ?`, email)
	return err
}

// DeleteExpiredLoginAttempts deletes login attempts older than the given time
func (db *DB) DeleteExpiredLoginAttempts(olderThan time.Time) (int64, error) {
	result, err := db.conn.Exec(`DELETE FROM login_attempts WHERE created_at < ?`, olderThan)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// IsEmailLocked checks if an email is locked due to too many failed login attempts
// Returns true if locked, along with the time when the lock expires
func (db *DB) IsEmailLocked(email string, maxAttempts int, lockDuration time.Duration) (bool, time.Time, error) {
	since := time.Now().Add(-lockDuration)
	count, err := db.GetRecentFailedLoginAttempts(email, since)
	if err != nil {
		return false, time.Time{}, err
	}

	if count >= maxAttempts {
		// Get the oldest failed attempt in the window to calculate unlock time
		var oldestAttemptStr string
		err := db.conn.QueryRow(`
			SELECT MIN(created_at) FROM login_attempts
			WHERE email = ? AND success = 0 AND created_at > ?
		`, email, since).Scan(&oldestAttemptStr)
		if err != nil {
			return false, time.Time{}, err
		}
		// Parse the datetime string (SQLite format)
		oldestAttempt, err := time.Parse("2006-01-02 15:04:05.999999999-07:00", oldestAttemptStr)
		if err != nil {
			// Try without timezone
			oldestAttempt, err = time.Parse("2006-01-02 15:04:05.999999999", oldestAttemptStr)
			if err != nil {
				// Try RFC3339
				oldestAttempt, err = time.Parse(time.RFC3339Nano, oldestAttemptStr)
				if err != nil {
					return false, time.Time{}, fmt.Errorf("failed to parse oldest attempt time: %v", err)
				}
			}
		}
		unlockTime := oldestAttempt.Add(lockDuration)
		return true, unlockTime, nil
	}

	return false, time.Time{}, nil
}

// EnableTOTPWithRecoveryCodes enables 2FA and saves recovery codes in a single transaction.
// This ensures that if code saving fails, the 2FA enable is rolled back.
func (db *DB) EnableTOTPWithRecoveryCodes(userID string, codeHashes []string) error {
	return db.WithTransaction(func(tx *Tx) error {
		// Enable TOTP
		_, err := tx.tx.Exec(`UPDATE users SET totp_enabled = 1 WHERE id = ?`, userID)
		if err != nil {
			return fmt.Errorf("failed to enable TOTP: %w", err)
		}

		// Delete existing unused recovery codes
		_, err = tx.tx.Exec(`DELETE FROM recovery_codes WHERE user_id = ? AND used = 0`, userID)
		if err != nil {
			return fmt.Errorf("failed to delete old recovery codes: %w", err)
		}

		// Insert new recovery codes
		for _, hash := range codeHashes {
			id, err := generateID()
			if err != nil {
				return fmt.Errorf("failed to generate recovery code ID: %w", err)
			}
			_, err = tx.tx.Exec(`
				INSERT INTO recovery_codes (id, user_id, code_hash, created_at)
				VALUES (?, ?, ?, ?)
			`, id, userID, hash, time.Now())
			if err != nil {
				return fmt.Errorf("failed to insert recovery code: %w", err)
			}
		}

		return nil
	})
}

// CompletePasswordReset updates password, marks token as used, deletes all tokens,
// and deletes all sessions in a single transaction.
func (db *DB) CompletePasswordReset(userID, tokenHash, passwordHash string) error {
	return db.WithTransaction(func(tx *Tx) error {
		// Update password
		_, err := tx.tx.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, userID)
		if err != nil {
			return fmt.Errorf("failed to update password: %w", err)
		}

		// Mark token as used
		_, err = tx.tx.Exec(`
			UPDATE password_reset_tokens
			SET used = 1
			WHERE token_hash = ? AND used = 0 AND expires_at > ?
		`, tokenHash, time.Now())
		if err != nil {
			return fmt.Errorf("failed to mark token as used: %w", err)
		}

		// Delete all password reset tokens for this user
		_, err = tx.tx.Exec(`DELETE FROM password_reset_tokens WHERE user_id = ?`, userID)
		if err != nil {
			return fmt.Errorf("failed to delete password reset tokens: %w", err)
		}

		// Delete all sessions for this user (force re-login)
		_, err = tx.tx.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID)
		if err != nil {
			return fmt.Errorf("failed to delete sessions: %w", err)
		}

		return nil
	})
}

// DisableTOTPAndClearSessions disables 2FA, deletes recovery codes, and clears all sessions
// in a single transaction. Used during account recovery.
func (db *DB) DisableTOTPAndClearSessions(userID string) error {
	return db.WithTransaction(func(tx *Tx) error {
		// Disable TOTP
		_, err := tx.tx.Exec(`UPDATE users SET totp_enabled = 0, totp_secret = NULL WHERE id = ?`, userID)
		if err != nil {
			return fmt.Errorf("failed to disable TOTP: %w", err)
		}

		// Delete recovery codes
		_, err = tx.tx.Exec(`DELETE FROM recovery_codes WHERE user_id = ?`, userID)
		if err != nil {
			return fmt.Errorf("failed to delete recovery codes: %w", err)
		}

		// Delete all sessions
		_, err = tx.tx.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID)
		if err != nil {
			return fmt.Errorf("failed to delete sessions: %w", err)
		}

		return nil
	})
}

// Vacuum runs SQLite VACUUM to reclaim disk space and defragment the database.
// This should be run periodically (e.g., daily) during low-usage periods.
// Note: VACUUM requires exclusive access and may take time for large databases.
func (db *DB) Vacuum() error {
	_, err := db.conn.Exec("VACUUM")
	return err
}

// Analyze runs SQLite ANALYZE to update query planner statistics.
// This should be run after significant data changes to improve query performance.
func (db *DB) Analyze() error {
	_, err := db.conn.Exec("ANALYZE")
	return err
}

// Maintenance runs both VACUUM and ANALYZE operations.
// Returns error from the first operation that fails, or nil if both succeed.
func (db *DB) Maintenance() error {
	if err := db.Vacuum(); err != nil {
		return fmt.Errorf("vacuum failed: %w", err)
	}
	if err := db.Analyze(); err != nil {
		return fmt.Errorf("analyze failed: %w", err)
	}
	return nil
}
