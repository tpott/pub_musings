package db

import (
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
	ID           string    `json:"id"`
	Filename     string    `json:"filename"`
	Size         int64     `json:"size"`
	ContentType  string    `json:"content_type"`
	FilePath     string    `json:"file_path"`
	CreatedAt    time.Time `json:"created_at"`
	UserID       *string   `json:"user_id,omitempty"` // null for anonymous uploads
	SessionID    *string   `json:"session_id,omitempty"`
}

// Transcription represents a transcription job and its result
type Transcription struct {
	ID          string    `json:"id"`
	VideoID     string    `json:"video_id"`
	Status      string    `json:"status"` // pending, processing, complete, error
	Message     string    `json:"message,omitempty"`
	Progress    int       `json:"progress"`
	Language    string    `json:"language,omitempty"`
	Duration    float64   `json:"duration,omitempty"`
	FullText    string    `json:"full_text,omitempty"`
	SegmentsJSON string   `json:"-"` // JSON-encoded segments
	CreatedAt   time.Time `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// Segment represents a single subtitle segment
type Segment struct {
	ID    int     `json:"id"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
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

// migrate runs database migrations
func (db *DB) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS videos (
			id TEXT PRIMARY KEY,
			filename TEXT NOT NULL,
			size INTEGER NOT NULL,
			content_type TEXT NOT NULL,
			file_path TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			user_id TEXT,
			session_id TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS transcriptions (
			id TEXT PRIMARY KEY,
			video_id TEXT NOT NULL REFERENCES videos(id),
			status TEXT NOT NULL DEFAULT 'pending',
			message TEXT,
			progress INTEGER NOT NULL DEFAULT 0,
			language TEXT,
			duration REAL,
			full_text TEXT,
			segments_json TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME
		)`,
		`CREATE INDEX IF NOT EXISTS idx_videos_user_id ON videos(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_videos_session_id ON videos(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_videos_created_at ON videos(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_transcriptions_video_id ON transcriptions(video_id)`,
	}

	for _, migration := range migrations {
		if _, err := db.conn.Exec(migration); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	return nil
}

// CreateVideo creates a new video record
func (db *DB) CreateVideo(video *Video) error {
	_, err := db.conn.Exec(`
		INSERT INTO videos (id, filename, size, content_type, file_path, created_at, user_id, session_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, video.ID, video.Filename, video.Size, video.ContentType, video.FilePath, video.CreatedAt, video.UserID, video.SessionID)
	return err
}

// GetVideo retrieves a video by ID
func (db *DB) GetVideo(id string) (*Video, error) {
	video := &Video{}
	err := db.conn.QueryRow(`
		SELECT id, filename, size, content_type, file_path, created_at, user_id, session_id
		FROM videos WHERE id = ?
	`, id).Scan(&video.ID, &video.Filename, &video.Size, &video.ContentType, &video.FilePath, &video.CreatedAt, &video.UserID, &video.SessionID)
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
			SELECT id, filename, size, content_type, file_path, created_at, user_id, session_id
			FROM videos WHERE user_id = ? ORDER BY created_at DESC
		`, *userID)
	} else if sessionID != nil {
		rows, err = db.conn.Query(`
			SELECT id, filename, size, content_type, file_path, created_at, user_id, session_id
			FROM videos WHERE session_id = ? ORDER BY created_at DESC
		`, *sessionID)
	} else {
		rows, err = db.conn.Query(`
			SELECT id, filename, size, content_type, file_path, created_at, user_id, session_id
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
		if err := rows.Scan(&v.ID, &v.Filename, &v.Size, &v.ContentType, &v.FilePath, &v.CreatedAt, &v.UserID, &v.SessionID); err != nil {
			return nil, err
		}
		videos = append(videos, v)
	}

	return videos, rows.Err()
}
