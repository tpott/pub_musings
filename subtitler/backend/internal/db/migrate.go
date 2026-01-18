package db

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Migrate runs all pending migrations from the given directory
func (db *DB) Migrate(migrationsDir string) error {
	// Create schema_migrations table if it doesn't exist
	if err := db.createMigrationsTable(); err != nil {
		return err
	}

	// Get list of applied migrations
	applied, err := db.getAppliedMigrations()
	if err != nil {
		return err
	}

	// Get list of migration files
	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// Sort migration files by name
	var migrationFiles []string
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".sql") {
			migrationFiles = append(migrationFiles, file.Name())
		}
	}
	sort.Strings(migrationFiles)

	// Run pending migrations
	for _, filename := range migrationFiles {
		if _, ok := applied[filename]; ok {
			continue // Migration already applied
		}

		// Read migration file
		path := filepath.Join(migrationsDir, filename)
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w", filename, err)
		}

		// Execute migration in a transaction
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction for %s: %w", filename, err)
		}

		if _, err := tx.Exec(string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to execute migration %s: %w", filename, err)
		}

		// Record migration
		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", filename); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %s: %w", filename, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", filename, err)
		}
	}

	return nil
}

// MigrateFromEmbed runs all pending migrations from an embedded filesystem
func (db *DB) MigrateFromEmbed(migrations embed.FS, migrationsPath string) error {
	// Create schema_migrations table if it doesn't exist
	if err := db.createMigrationsTable(); err != nil {
		return err
	}

	// Get list of applied migrations
	applied, err := db.getAppliedMigrations()
	if err != nil {
		return err
	}

	// Get list of migration files
	entries, err := migrations.ReadDir(migrationsPath)
	if err != nil {
		return fmt.Errorf("failed to read migrations from embed: %w", err)
	}

	// Sort migration files by name
	var migrationFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			migrationFiles = append(migrationFiles, entry.Name())
		}
	}
	sort.Strings(migrationFiles)

	// Run pending migrations
	for _, filename := range migrationFiles {
		if _, ok := applied[filename]; ok {
			continue // Migration already applied
		}

		// Read migration file from embed
		path := filepath.Join(migrationsPath, filename)
		content, err := migrations.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read migration %s from embed: %w", filename, err)
		}

		// Execute migration in a transaction
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction for %s: %w", filename, err)
		}

		if _, err := tx.Exec(string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to execute migration %s: %w", filename, err)
		}

		// Record migration
		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", filename); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %s: %w", filename, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %s: %w", filename, err)
		}
	}

	return nil
}

func (db *DB) createMigrationsTable() error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

func (db *DB) getAppliedMigrations() (map[string]bool, error) {
	rows, err := db.Query("SELECT version FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}

	return applied, rows.Err()
}
