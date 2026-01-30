package db

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// ErrInvalidVideoSize is returned when video size is outside valid bounds.
var ErrInvalidVideoSize = errors.New("invalid video size")

// DefaultQueryTimeout is the default timeout for database queries.
// Can be overridden via DB_QUERY_TIMEOUT environment variable.
const DefaultQueryTimeout = 30 * time.Second

// Default connection pool settings optimized for SQLite.
// SQLite typically doesn't benefit from many connections due to
// file-level locking, but these allow for concurrent reads with WAL mode.
const (
	DefaultMaxOpenConns = 10 // Max simultaneous connections
	DefaultMaxIdleConns = 5  // Max idle connections to retain
)

// Video size validation bounds
const (
	MinVideoSize = 1              // Minimum valid video size (1 byte)
	MaxVideoSize = 10 * (1 << 30) // Maximum valid video size (10 GB, defensive upper bound)
)

// PoolConfig holds database connection pool configuration.
type PoolConfig struct {
	MaxOpenConns int // Maximum number of open connections (0 = unlimited)
	MaxIdleConns int // Maximum number of idle connections
}

// DefaultPoolConfig returns the default connection pool configuration.
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxOpenConns: DefaultMaxOpenConns,
		MaxIdleConns: DefaultMaxIdleConns,
	}
}

// DB wraps the SQLite database connection
type DB struct {
	conn         *sql.DB
	queryTimeout time.Duration
}

// SetQueryTimeout sets the timeout for database queries.
// If set to 0, no timeout is applied (uses DefaultQueryTimeout).
func (db *DB) SetQueryTimeout(timeout time.Duration) {
	db.queryTimeout = timeout
}

// GetQueryTimeout returns the configured query timeout.
func (db *DB) GetQueryTimeout() time.Duration {
	if db.queryTimeout == 0 {
		return DefaultQueryTimeout
	}
	return db.queryTimeout
}

// sqliteTimestampFormats lists the formats SQLite may return for datetime values.
// SQLite stores timestamps as strings and returns them in various formats
// depending on how they were inserted. Aggregate functions like MIN() return strings.
var sqliteTimestampFormats = []string{
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02T15:04:05.999999999-07:00",
	"2006-01-02T15:04:05.999999999Z07:00",
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
}

// parseSQLiteTimestamp parses a timestamp string from SQLite.
// SQLite stores timestamps as strings and returns them in various formats.
// This function tries multiple formats to handle different insertion methods.
func parseSQLiteTimestamp(s string) (time.Time, error) {
	for _, format := range sqliteTimestampFormats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("failed to parse SQLite timestamp: %q", s)
}

// queryContext returns a context with the configured query timeout.
func (db *DB) queryContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), db.GetQueryTimeout())
}

// Video represents an uploaded video
type Video struct {
	ID                    string    `json:"id"`
	Filename              string    `json:"filename"`
	Size                  int64     `json:"size"`
	ContentType           string    `json:"content_type"`
	FilePath              string    `json:"file_path"`
	ThumbnailPath         *string   `json:"thumbnail_path,omitempty"`          // path to encrypted thumbnail image
	KeyVersion            int       `json:"key_version"`                       // encryption key version (for key rotation)
	EmbeddedSubtitlesJSON *string   `json:"embedded_subtitles_json,omitempty"` // JSON-encoded embedded subtitle tracks
	CreatedAt             time.Time `json:"created_at"`
	UserID                *string   `json:"user_id,omitempty"` // null for anonymous uploads
	SessionID             *string   `json:"session_id,omitempty"`
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

// Role constants for user authorization
const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

// User represents a registered user
type User struct {
	ID            string     `json:"id"`
	Email         string     `json:"email"`
	PasswordHash  string     `json:"-"` // Never serialize
	TOTPSecret    *string    `json:"-"` // Never serialize, nil if 2FA not set up
	TOTPEnabled   bool       `json:"totp_enabled"`
	EmailVerified bool       `json:"email_verified"`
	VerifiedAt    *time.Time `json:"verified_at,omitempty"`
	Role          string     `json:"role"` // "user" or "admin"
	CreatedAt     time.Time  `json:"created_at"`
}

// IsAdmin returns true if the user has admin role
func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
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
	ID               string     `json:"id"`
	VideoID          string     `json:"video_id"`
	Status           string     `json:"status"` // pending, processing, complete, error
	Message          string     `json:"message,omitempty"`
	Progress         int        `json:"progress"`
	OutputPath       string     `json:"output_path,omitempty"`
	OutputKeyVersion *int       `json:"output_key_version,omitempty"` // encryption key version for output file
	CreatedAt        time.Time  `json:"created_at"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
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

// UploadSession represents a chunked upload session
type UploadSession struct {
	ID          string     `json:"id"`
	Filename    string     `json:"filename"`
	ContentType string     `json:"content_type"`
	TotalSize   int64      `json:"total_size"`
	ChunkSize   int64      `json:"chunk_size"`
	TotalChunks int        `json:"total_chunks"`
	UserID      *string    `json:"user_id,omitempty"`
	SessionID   *string    `json:"session_id,omitempty"`
	Status      string     `json:"status"` // in_progress, complete, expired
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// UploadChunk represents a single chunk within an upload session
type UploadChunk struct {
	ID              string    `json:"id"`
	UploadSessionID string    `json:"upload_session_id"`
	ChunkIndex      int       `json:"chunk_index"`
	ChunkPath       string    `json:"chunk_path"`
	Size            int64     `json:"size"`
	CreatedAt       time.Time `json:"created_at"`
}

// Feedback status constants
const (
	FeedbackStatusNew      = "new"
	FeedbackStatusRead     = "read"
	FeedbackStatusResolved = "resolved"
)

// Feedback type constants
const (
	FeedbackTypeGeneral = "general"
	FeedbackTypeBug     = "bug"
	FeedbackTypeFeature = "feature"
)

// Feedback represents user feedback
type Feedback struct {
	ID          string    `json:"id"`
	UserID      *string   `json:"user_id,omitempty"`    // NULL for anonymous users
	SessionID   *string   `json:"session_id,omitempty"` // For anonymous tracking
	VideoID     *string   `json:"video_id,omitempty"`   // Which video (optional)
	PageURL     string    `json:"page_url"`             // Current page URL
	Text        string    `json:"text"`                 // User's message
	Rating      *int      `json:"rating,omitempty"`     // 1-5 or NULL
	Type        string    `json:"type"`                 // "general", "bug", "feature"
	BrowserInfo string    `json:"browser_info"`         // User agent, viewport, etc.
	CreatedAt   time.Time `json:"created_at"`
	Status      string    `json:"status"` // "new", "read", "resolved"
}

// Open opens or creates a SQLite database at the given path with default pool settings.
func Open(dbPath string) (*DB, error) {
	return OpenWithConfig(dbPath, DefaultPoolConfig())
}

// OpenWithConfig opens or creates a SQLite database with custom pool configuration.
func OpenWithConfig(dbPath string, cfg PoolConfig) (*DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	conn, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	if cfg.MaxOpenConns > 0 {
		conn.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		conn.SetMaxIdleConns(cfg.MaxIdleConns)
	}

	// Test connection
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &DB{
		conn:         conn,
		queryTimeout: DefaultQueryTimeout,
	}

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

// PanicError wraps a panic value and stack trace as an error.
// This allows panics to propagate through the error handling system
// rather than crashing the server.
type PanicError struct {
	Value interface{}
	Stack string
}

func (e *PanicError) Error() string {
	return fmt.Sprintf("panic in transaction: %v\n%s", e.Value, e.Stack)
}

// WithTransaction executes the given function within a database transaction.
// If the function returns an error, the transaction is rolled back.
// If the function succeeds, the transaction is committed.
// If the function panics, the transaction is rolled back and the panic is
// converted to a PanicError that is returned, preventing server crashes.
// The transaction uses a context with timeout (default 30s) to prevent indefinite hangs.
func (db *DB) WithTransaction(fn func(*Tx) error) (err error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	tx, err := db.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Ensure transaction is rolled back on panic and convert panic to error
	defer func() {
		if p := recover(); p != nil {
			// Attempt to rollback - best effort
			_ = tx.Rollback()
			// Capture stack trace
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			// Convert panic to error instead of re-panicking
			err = &PanicError{
				Value: p,
				Stack: string(buf[:n]),
			}
		}
	}()

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
	ctx, cancel := db.queryContext()
	defer cancel()

	// Check if schema_migrations table exists
	var tableName string
	err := db.conn.QueryRowContext(ctx, `
		SELECT name FROM sqlite_master
		WHERE type='table' AND name='schema_migrations'
	`).Scan(&tableName)

	if err == nil {
		// schema_migrations exists, nothing to do
		return nil
	}

	// Check if videos table exists (indicates pre-migration database)
	err = db.conn.QueryRowContext(ctx, `
		SELECT name FROM sqlite_master
		WHERE type='table' AND name='videos'
	`).Scan(&tableName)

	if err != nil {
		// No videos table means fresh database, migrations will create everything
		return nil
	}

	// This is a pre-migration database - we need to create schema_migrations
	// and mark the initial migration as applied
	_, err = db.conn.ExecContext(ctx, `
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
	_, err = db.conn.ExecContext(ctx, `
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

// VideoListResult contains paginated video results
type VideoListResult struct {
	Videos     []Video
	TotalCount int
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

// CreateUser creates a new user record
func (db *DB) CreateUser(user *User) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	// Default to user role if not specified
	role := user.Role
	if role == "" {
		role = RoleUser
	}
	_, err := db.conn.ExecContext(ctx, `
		INSERT INTO users (id, email, password_hash, totp_secret, totp_enabled, email_verified, verified_at, role, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, user.ID, user.Email, user.PasswordHash, user.TOTPSecret, user.TOTPEnabled, user.EmailVerified, user.VerifiedAt, role, user.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

// GetUserByID retrieves a user by ID
func (db *DB) GetUserByID(id string) (*User, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	user := &User{}
	var totpSecret sql.NullString
	var verifiedAt sql.NullTime
	err := db.conn.QueryRowContext(ctx, `
		SELECT id, email, password_hash, totp_secret, totp_enabled, email_verified, verified_at, role, created_at
		FROM users WHERE id = ?
	`, id).Scan(&user.ID, &user.Email, &user.PasswordHash, &totpSecret, &user.TOTPEnabled, &user.EmailVerified, &verifiedAt, &user.Role, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by ID %s: %w", id, err)
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
	ctx, cancel := db.queryContext()
	defer cancel()

	user := &User{}
	var totpSecret sql.NullString
	var verifiedAt sql.NullTime
	err := db.conn.QueryRowContext(ctx, `
		SELECT id, email, password_hash, totp_secret, totp_enabled, email_verified, verified_at, role, created_at
		FROM users WHERE email = ?
	`, email).Scan(&user.ID, &user.Email, &user.PasswordHash, &totpSecret, &user.TOTPEnabled, &user.EmailVerified, &verifiedAt, &user.Role, &user.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by email: %w", err)
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
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `
		INSERT INTO sessions (id, user_id, token, ip_address, user_agent, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, session.ID, session.UserID, session.Token, session.IPAddress, session.UserAgent, session.ExpiresAt, session.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	return nil
}

// GetSessionByToken retrieves a session by token
func (db *DB) GetSessionByToken(token string) (*Session, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	session := &Session{}
	var ipAddress, userAgent sql.NullString
	err := db.conn.QueryRowContext(ctx, `
		SELECT id, user_id, token, ip_address, user_agent, expires_at, created_at
		FROM sessions WHERE token = ?
	`, token).Scan(&session.ID, &session.UserID, &session.Token, &ipAddress, &userAgent, &session.ExpiresAt, &session.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session by token: %w", err)
	}
	if ipAddress.Valid {
		session.IPAddress = ipAddress.String
	}
	if userAgent.Valid {
		session.UserAgent = userAgent.String
	}
	return session, nil
}

// MaxSessionsPerUser is the maximum number of sessions returned per user.
// This prevents memory issues for users with many sessions.
const MaxSessionsPerUser = 100

// GetSessionsByUserID retrieves active sessions for a user, limited to MaxSessionsPerUser.
// Sessions are ordered by creation date (newest first).
func (db *DB) GetSessionsByUserID(userID string) ([]Session, error) {
	return db.GetSessionsByUserIDWithLimit(userID, MaxSessionsPerUser)
}

// GetSessionsByUserIDWithLimit retrieves active sessions for a user with a custom limit.
// If limit is 0 or negative, MaxSessionsPerUser is used.
func (db *DB) GetSessionsByUserIDWithLimit(userID string, limit int) ([]Session, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	if limit <= 0 {
		limit = MaxSessionsPerUser
	}

	rows, err := db.conn.QueryContext(ctx, `
		SELECT id, user_id, token, ip_address, user_agent, expires_at, created_at
		FROM sessions
		WHERE user_id = ? AND expires_at > ?
		ORDER BY created_at DESC
		LIMIT ?
	`, userID, time.Now(), limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query sessions for user: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var session Session
		var ipAddress, userAgent sql.NullString
		if err := rows.Scan(&session.ID, &session.UserID, &session.Token, &ipAddress, &userAgent, &session.ExpiresAt, &session.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan session row: %w", err)
		}
		if ipAddress.Valid {
			session.IPAddress = ipAddress.String
		}
		if userAgent.Valid {
			session.UserAgent = userAgent.String
		}
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate sessions: %w", err)
	}
	return sessions, nil
}

// DeleteSessionByID deletes a session by its ID (for session management)
func (db *DB) DeleteSessionByID(sessionID, userID string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	// Require userID to ensure users can only delete their own sessions
	result, err := db.conn.ExecContext(ctx, `DELETE FROM sessions WHERE id = ? AND user_id = ?`, sessionID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("session not found or not owned by user")
	}
	return nil
}

// DeleteSession deletes a session by token
func (db *DB) DeleteSession(token string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `DELETE FROM sessions WHERE token = ?`, token)
	if err != nil {
		return fmt.Errorf("failed to delete session by token: %w", err)
	}
	return nil
}

// DeleteExpiredSessions deletes all expired sessions
func (db *DB) DeleteExpiredSessions() (int64, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	result, err := db.conn.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`, time.Now())
	if err != nil {
		return 0, fmt.Errorf("failed to delete expired sessions: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get affected rows: %w", err)
	}
	return count, nil
}

// DeleteUserSessions deletes all sessions for a user
func (db *DB) DeleteUserSessions(userID string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID)
	if err != nil {
		return fmt.Errorf("failed to delete user sessions: %w", err)
	}
	return nil
}

// CountActiveSessions returns the count of non-expired sessions.
// Used for metrics reporting.
func (db *DB) CountActiveSessions() (int, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	var count int
	err := db.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM sessions WHERE expires_at > ?`, time.Now()).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count active sessions: %w", err)
	}
	return count, nil
}

// SetTOTPSecret sets the TOTP secret for a user (during 2FA setup)
func (db *DB) SetTOTPSecret(userID, secret string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `
		UPDATE users SET totp_secret = ? WHERE id = ?
	`, secret, userID)
	if err != nil {
		return fmt.Errorf("failed to set TOTP secret: %w", err)
	}
	return nil
}

// EnableTOTP enables 2FA for a user (after verification)
func (db *DB) EnableTOTP(userID string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `
		UPDATE users SET totp_enabled = 1 WHERE id = ?
	`, userID)
	if err != nil {
		return fmt.Errorf("failed to enable TOTP: %w", err)
	}
	return nil
}

// DisableTOTP disables 2FA and clears the secret for a user
func (db *DB) DisableTOTP(userID string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `
		UPDATE users SET totp_enabled = 0, totp_secret = NULL WHERE id = ?
	`, userID)
	if err != nil {
		return fmt.Errorf("failed to disable TOTP: %w", err)
	}
	return nil
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

// DeletedVideoFiles contains paths to files that should be deleted after a video is removed
type DeletedVideoFiles struct {
	FilePath       string  // path to the video file
	ThumbnailPath  *string // path to the thumbnail file (may be nil)
	BurnOutputPath *string // path to the burned video file (may be nil)
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

// SaveRecoveryCodes stores hashed recovery codes for a user.
// Deletes any existing unused codes first.
func (db *DB) SaveRecoveryCodes(userID string, codeHashes []string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	// Delete existing unused codes
	_, err := db.conn.ExecContext(ctx, `DELETE FROM recovery_codes WHERE user_id = ? AND used = 0`, userID)
	if err != nil {
		return fmt.Errorf("failed to delete old recovery codes: %w", err)
	}

	// Insert new codes
	for _, hash := range codeHashes {
		id, err := generateID()
		if err != nil {
			return fmt.Errorf("failed to generate recovery code ID: %w", err)
		}
		_, err = db.conn.ExecContext(ctx, `
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
	ctx, cancel := db.queryContext()
	defer cancel()

	rows, err := db.conn.QueryContext(ctx, `
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
	ctx, cancel := db.queryContext()
	defer cancel()

	now := time.Now()
	result, err := db.conn.ExecContext(ctx, `
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
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `DELETE FROM recovery_codes WHERE user_id = ?`, userID)
	return err
}

// CountUnusedRecoveryCodes returns the number of unused recovery codes for a user
func (db *DB) CountUnusedRecoveryCodes(userID string) (int, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	var count int
	err := db.conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM recovery_codes WHERE user_id = ? AND used = 0
	`, userID).Scan(&count)
	return count, err
}

// CreatePasswordResetToken creates a new password reset token for a user.
// Deletes any existing unused tokens for the user first.
func (db *DB) CreatePasswordResetToken(userID, tokenHash string, expiresAt time.Time) (*PasswordResetToken, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	// Delete existing unused tokens for this user
	_, err := db.conn.ExecContext(ctx, `DELETE FROM password_reset_tokens WHERE user_id = ? AND used = 0`, userID)
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

	_, err = db.conn.ExecContext(ctx, `
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
	ctx, cancel := db.queryContext()
	defer cancel()

	token := &PasswordResetToken{}
	err := db.conn.QueryRowContext(ctx, `
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
	ctx, cancel := db.queryContext()
	defer cancel()

	result, err := db.conn.ExecContext(ctx, `
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
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `DELETE FROM password_reset_tokens WHERE user_id = ?`, userID)
	return err
}

// DeleteExpiredPasswordResetTokens deletes all expired password reset tokens
func (db *DB) DeleteExpiredPasswordResetTokens() (int64, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	result, err := db.conn.ExecContext(ctx, `DELETE FROM password_reset_tokens WHERE expires_at < ?`, time.Now())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// CreateEmailVerificationToken creates a new email verification token for a user.
// Deletes any existing unused tokens for the user first.
func (db *DB) CreateEmailVerificationToken(userID, tokenHash string, expiresAt time.Time) (*EmailVerificationToken, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	// Delete existing unused tokens for this user
	_, err := db.conn.ExecContext(ctx, `DELETE FROM email_verification_tokens WHERE user_id = ? AND used = 0`, userID)
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

	_, err = db.conn.ExecContext(ctx, `
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
	ctx, cancel := db.queryContext()
	defer cancel()

	token := &EmailVerificationToken{}
	err := db.conn.QueryRowContext(ctx, `
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

		// Delete all verification tokens for this user (cleanup - prevents accumulation)
		_, err = tx.tx.Exec(`DELETE FROM email_verification_tokens WHERE user_id = ?`, token.UserID)
		if err != nil {
			return fmt.Errorf("failed to cleanup verification tokens: %w", err)
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
	ctx, cancel := db.queryContext()
	defer cancel()

	now := time.Now()
	_, err := db.conn.ExecContext(ctx, `UPDATE users SET email_verified = 1, verified_at = ? WHERE id = ?`, now, userID)
	return err
}

// DeleteEmailVerificationTokens deletes all email verification tokens for a user
func (db *DB) DeleteEmailVerificationTokens(userID string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `DELETE FROM email_verification_tokens WHERE user_id = ?`, userID)
	return err
}

// DeleteExpiredEmailVerificationTokens deletes all expired email verification tokens
func (db *DB) DeleteExpiredEmailVerificationTokens() (int64, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	result, err := db.conn.ExecContext(ctx, `DELETE FROM email_verification_tokens WHERE expires_at < ?`, time.Now())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// GetUnusedEmailVerificationToken returns the most recent unused verification token for a user
func (db *DB) GetUnusedEmailVerificationToken(userID string) (*EmailVerificationToken, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	token := &EmailVerificationToken{}
	err := db.conn.QueryRowContext(ctx, `
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
	ctx, cancel := db.queryContext()
	defer cancel()

	// Delete existing unused tokens for this user
	_, err := db.conn.ExecContext(ctx, `DELETE FROM magic_link_tokens WHERE user_id = ? AND used = 0`, userID)
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

	_, err = db.conn.ExecContext(ctx, `
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
	ctx, cancel := db.queryContext()
	defer cancel()

	token := &MagicLinkToken{}
	err := db.conn.QueryRowContext(ctx, `
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
	ctx, cancel := db.queryContext()
	defer cancel()

	result, err := db.conn.ExecContext(ctx, `
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
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `DELETE FROM magic_link_tokens WHERE user_id = ?`, userID)
	return err
}

// DeleteExpiredMagicLinkTokens deletes all expired magic link tokens
func (db *DB) DeleteExpiredMagicLinkTokens() (int64, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	result, err := db.conn.ExecContext(ctx, `DELETE FROM magic_link_tokens WHERE expires_at < ?`, time.Now())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// CountRecentMagicLinkRequests counts magic link tokens created for a user since the given time.
// Used for per-email rate limiting to prevent abuse. Note: this counts all created tokens,
// including used and expired ones, to track request frequency.
func (db *DB) CountRecentMagicLinkRequests(userID string, since time.Time) (int, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	var count int
	err := db.conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM magic_link_tokens
		WHERE user_id = ? AND created_at > ?
	`, userID, since).Scan(&count)
	return count, err
}

// UpdateUserPassword updates a user's password hash
func (db *DB) UpdateUserPassword(userID, passwordHash string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, userID)
	return err
}

// UpdateUserRole updates a user's role
func (db *DB) UpdateUserRole(userID, role string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	// Validate role
	if role != RoleUser && role != RoleAdmin {
		return fmt.Errorf("invalid role: %s", role)
	}
	_, err := db.conn.ExecContext(ctx, `UPDATE users SET role = ? WHERE id = ?`, role, userID)
	return err
}

// PromoteToAdmin promotes a user to admin role by email
func (db *DB) PromoteToAdmin(email string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	result, err := db.conn.ExecContext(ctx, `UPDATE users SET role = ? WHERE email = ?`, RoleAdmin, email)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("user not found: %s", email)
	}
	return nil
}

// RecordLoginAttempt records a login attempt for rate limiting
func (db *DB) RecordLoginAttempt(email, ipAddress string, success bool) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return err
	}

	successInt := 0
	if success {
		successInt = 1
	}

	_, err := db.conn.ExecContext(ctx, `
		INSERT INTO login_attempts (id, email, success, ip_address, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, fmt.Sprintf("%x", id), email, successInt, ipAddress, time.Now())
	return err
}

// GetRecentFailedLoginAttempts returns the number of failed login attempts for an email
// in the given time window
func (db *DB) GetRecentFailedLoginAttempts(email string, since time.Time) (int, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	var count int
	err := db.conn.QueryRowContext(ctx, `
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
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `DELETE FROM login_attempts WHERE email = ?`, email)
	return err
}

// DeleteExpiredLoginAttempts deletes login attempts older than the given time
func (db *DB) DeleteExpiredLoginAttempts(olderThan time.Time) (int64, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	result, err := db.conn.ExecContext(ctx, `DELETE FROM login_attempts WHERE created_at < ?`, olderThan)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// IsEmailLocked checks if an email is locked due to too many failed login attempts
// Returns true if locked, along with the time when the lock expires
func (db *DB) IsEmailLocked(email string, maxAttempts int, lockDuration time.Duration) (bool, time.Time, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	since := time.Now().Add(-lockDuration)
	count, err := db.GetRecentFailedLoginAttempts(email, since)
	if err != nil {
		return false, time.Time{}, err
	}

	if count >= maxAttempts {
		// Get the oldest failed attempt in the window to calculate unlock time
		// Note: MIN() returns a string in SQLite, so we scan to string and parse
		var oldestAttemptStr sql.NullString
		err := db.conn.QueryRowContext(ctx, `
			SELECT MIN(created_at) FROM login_attempts
			WHERE email = ? AND success = 0 AND created_at > ?
		`, email, since).Scan(&oldestAttemptStr)
		if err != nil {
			return false, time.Time{}, fmt.Errorf("failed to get oldest login attempt: %w", err)
		}
		if !oldestAttemptStr.Valid {
			// This shouldn't happen since count >= maxAttempts, but handle gracefully
			return false, time.Time{}, fmt.Errorf("no login attempts found despite count >= %d", maxAttempts)
		}
		oldestAttempt, err := parseSQLiteTimestamp(oldestAttemptStr.String)
		if err != nil {
			return false, time.Time{}, fmt.Errorf("failed to parse oldest login attempt time: %w", err)
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
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, "VACUUM")
	return err
}

// Analyze runs SQLite ANALYZE to update query planner statistics.
// This should be run after significant data changes to improve query performance.
func (db *DB) Analyze() error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, "ANALYZE")
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

// CreateUploadSession creates a new chunked upload session
func (db *DB) CreateUploadSession(session *UploadSession) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `
		INSERT INTO upload_sessions (id, filename, content_type, total_size, chunk_size, total_chunks, user_id, session_id, status, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, session.ID, session.Filename, session.ContentType, session.TotalSize, session.ChunkSize, session.TotalChunks, session.UserID, session.SessionID, session.Status, session.CreatedAt, session.ExpiresAt)
	return err
}

// GetUploadSession retrieves an upload session by ID
func (db *DB) GetUploadSession(id string) (*UploadSession, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	session := &UploadSession{}
	var userID, sessionID sql.NullString
	var completedAt sql.NullTime

	err := db.conn.QueryRowContext(ctx, `
		SELECT id, filename, content_type, total_size, chunk_size, total_chunks, user_id, session_id, status, created_at, expires_at, completed_at
		FROM upload_sessions WHERE id = ?
	`, id).Scan(&session.ID, &session.Filename, &session.ContentType, &session.TotalSize, &session.ChunkSize, &session.TotalChunks, &userID, &sessionID, &session.Status, &session.CreatedAt, &session.ExpiresAt, &completedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if userID.Valid {
		session.UserID = &userID.String
	}
	if sessionID.Valid {
		session.SessionID = &sessionID.String
	}
	if completedAt.Valid {
		session.CompletedAt = &completedAt.Time
	}

	return session, nil
}

// UpdateUploadSessionStatus updates the status of an upload session
func (db *DB) UpdateUploadSessionStatus(sessionID, status string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	var completedAt interface{}
	if status == "complete" {
		completedAt = time.Now()
	}
	_, err := db.conn.ExecContext(ctx, `
		UPDATE upload_sessions SET status = ?, completed_at = ? WHERE id = ?
	`, status, completedAt, sessionID)
	return err
}

// CreateUploadChunk records a chunk upload
func (db *DB) CreateUploadChunk(chunk *UploadChunk) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `
		INSERT INTO upload_chunks (id, upload_session_id, chunk_index, chunk_path, size, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, chunk.ID, chunk.UploadSessionID, chunk.ChunkIndex, chunk.ChunkPath, chunk.Size, chunk.CreatedAt)
	return err
}

// GetUploadChunk retrieves a specific chunk by session ID and index
func (db *DB) GetUploadChunk(sessionID string, chunkIndex int) (*UploadChunk, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	chunk := &UploadChunk{}
	err := db.conn.QueryRowContext(ctx, `
		SELECT id, upload_session_id, chunk_index, chunk_path, size, created_at
		FROM upload_chunks WHERE upload_session_id = ? AND chunk_index = ?
	`, sessionID, chunkIndex).Scan(&chunk.ID, &chunk.UploadSessionID, &chunk.ChunkIndex, &chunk.ChunkPath, &chunk.Size, &chunk.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return chunk, nil
}

// GetUploadChunks retrieves all chunks for an upload session
func (db *DB) GetUploadChunks(sessionID string) ([]UploadChunk, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	rows, err := db.conn.QueryContext(ctx, `
		SELECT id, upload_session_id, chunk_index, chunk_path, size, created_at
		FROM upload_chunks WHERE upload_session_id = ? ORDER BY chunk_index
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []UploadChunk
	for rows.Next() {
		var chunk UploadChunk
		if err := rows.Scan(&chunk.ID, &chunk.UploadSessionID, &chunk.ChunkIndex, &chunk.ChunkPath, &chunk.Size, &chunk.CreatedAt); err != nil {
			return nil, err
		}
		chunks = append(chunks, chunk)
	}
	return chunks, rows.Err()
}

// CountUploadChunks returns the number of chunks uploaded for a session
func (db *DB) CountUploadChunks(sessionID string) (int, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	var count int
	err := db.conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM upload_chunks WHERE upload_session_id = ?
	`, sessionID).Scan(&count)
	return count, err
}

// GetReceivedChunkIndices returns the indices of all chunks received for a session
func (db *DB) GetReceivedChunkIndices(sessionID string) ([]int, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	rows, err := db.conn.QueryContext(ctx, `
		SELECT chunk_index FROM upload_chunks WHERE upload_session_id = ? ORDER BY chunk_index
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var indices []int
	for rows.Next() {
		var index int
		if err := rows.Scan(&index); err != nil {
			return nil, err
		}
		indices = append(indices, index)
	}
	return indices, rows.Err()
}

// GetTotalReceivedBytes returns the sum of all chunk sizes for a session
func (db *DB) GetTotalReceivedBytes(sessionID string) (int64, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	var total sql.NullInt64
	err := db.conn.QueryRowContext(ctx, `
		SELECT SUM(size) FROM upload_chunks WHERE upload_session_id = ?
	`, sessionID).Scan(&total)
	if err != nil {
		return 0, err
	}
	if !total.Valid {
		return 0, nil
	}
	return total.Int64, nil
}

// GetExpiredUploadSessions returns all expired upload sessions
func (db *DB) GetExpiredUploadSessions() ([]UploadSession, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	rows, err := db.conn.QueryContext(ctx, `
		SELECT id, filename, content_type, total_size, chunk_size, total_chunks, user_id, session_id, status, created_at, expires_at, completed_at
		FROM upload_sessions WHERE expires_at < ? AND status = 'in_progress'
	`, time.Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []UploadSession
	for rows.Next() {
		var session UploadSession
		var userID, sessionID sql.NullString
		var completedAt sql.NullTime
		if err := rows.Scan(&session.ID, &session.Filename, &session.ContentType, &session.TotalSize, &session.ChunkSize, &session.TotalChunks, &userID, &sessionID, &session.Status, &session.CreatedAt, &session.ExpiresAt, &completedAt); err != nil {
			return nil, err
		}
		if userID.Valid {
			session.UserID = &userID.String
		}
		if sessionID.Valid {
			session.SessionID = &sessionID.String
		}
		if completedAt.Valid {
			session.CompletedAt = &completedAt.Time
		}
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

// DeleteUploadSession deletes an upload session and its chunks
// Returns the chunk paths so caller can delete files
func (db *DB) DeleteUploadSession(sessionID string) ([]string, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	// Get chunk paths before deleting
	chunks, err := db.GetUploadChunks(sessionID)
	if err != nil {
		return nil, err
	}

	var paths []string
	for _, chunk := range chunks {
		paths = append(paths, chunk.ChunkPath)
	}

	// Delete chunks first (foreign key constraint)
	_, err = db.conn.ExecContext(ctx, `DELETE FROM upload_chunks WHERE upload_session_id = ?`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete upload chunks: %w", err)
	}

	// Delete session
	_, err = db.conn.ExecContext(ctx, `DELETE FROM upload_sessions WHERE id = ?`, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to delete upload session: %w", err)
	}

	return paths, nil
}

// UploadSessionExists checks if an upload session exists in the database
func (db *DB) UploadSessionExists(sessionID string) (bool, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	var count int
	err := db.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM upload_sessions WHERE id = ?`, sessionID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
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
