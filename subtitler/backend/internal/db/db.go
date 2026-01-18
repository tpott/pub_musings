package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// DB wraps the database connection and provides helper methods
type DB struct {
	*sql.DB
	path string
}

// Open opens a connection to the SQLite database at the given path.
// It creates the parent directory if it doesn't exist.
func Open(dbPath string) (*DB, error) {
	// Create parent directory if it doesn't exist
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open database connection
	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Enable foreign keys
	if _, err := sqlDB.Exec("PRAGMA foreign_keys = ON"); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	db := &DB{
		DB:   sqlDB,
		path: dbPath,
	}

	return db, nil
}

// Initialize opens the database connection and runs migrations
func Initialize(dbPath string, migrationsDir string) (*DB, error) {
	db, err := Open(dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Migrate(migrationsDir); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}

// Path returns the database file path
func (db *DB) Path() string {
	return db.path
}
