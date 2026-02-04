// Package db provides SQLite database access for the Peekaboo media database.
package db

import (
	"database/sql"
	"fmt"

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

// schema defines the database tables.
const schema = `
CREATE TABLE IF NOT EXISTS concepts (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS media_sets (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	concept_id TEXT NOT NULL REFERENCES concepts(id),
	photo_path TEXT NOT NULL,
	audio_path TEXT,
	video_path TEXT
);
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
func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Test connection
	if err := conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &DB{conn: conn}, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
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
func (db *DB) SeedMediaSet(conceptID, photoPath, audioPath, videoPath string) error {
	_, err := db.conn.Exec(
		"INSERT INTO media_sets (concept_id, photo_path, audio_path, video_path) VALUES (?, ?, ?, ?)",
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
