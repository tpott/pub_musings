// Package db provides SQLite database access for the Peekaboo media database.
package db

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"

	_ "github.com/mattn/go-sqlite3"
)

// DB wraps the SQLite database connection.
type DB struct {
	conn *sql.DB
}

// MediaSet represents a set of media files for a concept.
type MediaSet struct {
	ID        int64
	ConceptID string
	PhotoPath string
	AudioPath string
	VideoPath string
}

// schema defines the database tables and indexes.
const schema = `
CREATE TABLE IF NOT EXISTS concepts (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS media_sets (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	concept_id TEXT NOT NULL REFERENCES concepts(id),
	photo_path TEXT NOT NULL UNIQUE,
	audio_path TEXT,
	video_path TEXT
);

CREATE INDEX IF NOT EXISTS idx_media_sets_concept_id ON media_sets(concept_id);
`

// seedData contains the 6 MVP animals.
var seedData = []struct {
	id   string
	name string
}{
	{"cat", "Cat"},
	{"dog", "Dog"},
	{"duck", "Duck"},
	{"pig", "Pig"},
	{"chicken", "Chicken"},
	{"cow", "Cow"},
}

// Open opens a connection to the SQLite database at the given path.
// If the database doesn't exist, it will be created.
// Configures WAL mode, busy timeout, and connection limits for production use.
//
// Connection pool can be configured via environment variables:
//   - DB_MAX_OPEN_CONNS: Maximum open connections (default: 1, recommended for SQLite)
//   - DB_MAX_IDLE_CONNS: Maximum idle connections (default: 1)
//
// Note: SQLite only supports one writer at a time, even with WAL mode.
// Higher connection counts may improve read performance but can cause
// "database is locked" errors on write-heavy workloads.
func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Configure connection pool from environment (with SQLite-safe defaults)
	maxOpenConns := getEnvInt("DB_MAX_OPEN_CONNS", 1)
	maxIdleConns := getEnvInt("DB_MAX_IDLE_CONNS", 1)

	// Limit connections - SQLite doesn't handle concurrent writers well
	// With WAL mode, we can have concurrent readers but still only one writer
	conn.SetMaxOpenConns(maxOpenConns)
	conn.SetMaxIdleConns(maxIdleConns)

	// Test connection
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	// Configure SQLite for better performance and reliability
	pragmas := []string{
		"PRAGMA busy_timeout = 5000",  // Wait up to 5 seconds if database is locked
		"PRAGMA journal_mode = WAL",   // Write-Ahead Logging for better concurrency
		"PRAGMA synchronous = NORMAL", // Balance between safety and performance
	}
	for _, pragma := range pragmas {
		if _, err := conn.Exec(pragma); err != nil {
			conn.Close()
			return nil, fmt.Errorf("set %s: %w", pragma, err)
		}
	}

	return &DB{conn: conn}, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

// Ping checks the database connection is alive.
func (db *DB) Ping() error {
	return db.conn.Ping()
}

// Init initializes the database schema and seeds the concepts table.
func (db *DB) Init() error {
	// Create tables
	if _, err := db.conn.Exec(schema); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}

	// Seed concepts
	for _, c := range seedData {
		_, err := db.conn.Exec(
			"INSERT OR IGNORE INTO concepts (id, name) VALUES (?, ?)",
			c.id, c.name,
		)
		if err != nil {
			return fmt.Errorf("seed concept %s: %w", c.id, err)
		}
	}

	return nil
}

// SeedMediaSet inserts a media set for a concept.
// If a media set with the same photo_path already exists, it's ignored.
func (db *DB) SeedMediaSet(conceptID, photoPath, audioPath, videoPath string) error {
	_, err := db.conn.Exec(
		"INSERT OR IGNORE INTO media_sets (concept_id, photo_path, audio_path, video_path) VALUES (?, ?, ?, ?)",
		conceptID, photoPath, audioPath, videoPath,
	)
	if err != nil {
		return fmt.Errorf("insert media set for %s: %w", conceptID, err)
	}
	return nil
}

// GetRandomMediaSet retrieves a random media set for the given concept.
// Returns nil if no media set exists for the concept.
func (db *DB) GetRandomMediaSet(conceptID string) (*MediaSet, error) {
	row := db.conn.QueryRow(`
		SELECT id, concept_id, photo_path, audio_path, video_path
		FROM media_sets
		WHERE concept_id = ?
		ORDER BY RANDOM()
		LIMIT 1
	`, conceptID)

	var ms MediaSet
	var audioPath, videoPath sql.NullString

	err := row.Scan(&ms.ID, &ms.ConceptID, &ms.PhotoPath, &audioPath, &videoPath)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query media set for %s: %w", conceptID, err)
	}

	ms.AudioPath = audioPath.String
	ms.VideoPath = videoPath.String

	return &ms, nil
}

// GetConcept checks if a concept exists by ID.
func (db *DB) GetConcept(id string) (string, error) {
	var name string
	err := db.conn.QueryRow("SELECT name FROM concepts WHERE id = ?", id).Scan(&name)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("query concept %s: %w", id, err)
	}
	return name, nil
}

// getEnvInt reads an integer from an environment variable, returning defaultVal if not set or invalid.
func getEnvInt(key string, defaultVal int) int {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return n
}
