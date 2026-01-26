package db

import (
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Migration represents a single migration file
type Migration struct {
	Version     int
	Description string
	UpSQL       string
	DownSQL     string
}

// Migrator handles database migrations
type Migrator struct {
	db         *sql.DB
	migrations []Migration
}

// NewMigrator creates a new Migrator with migrations loaded from the embedded filesystem
func NewMigrator(db *sql.DB) (*Migrator, error) {
	m := &Migrator{db: db}

	if err := m.loadMigrations(); err != nil {
		return nil, fmt.Errorf("failed to load migrations: %w", err)
	}

	return m, nil
}

// loadMigrations reads migration files from the embedded filesystem
func (m *Migrator) loadMigrations() error {
	// Pattern: NNN_description.up.sql or NNN_description.down.sql
	pattern := regexp.MustCompile(`^(\d+)_(.+)\.(up|down)\.sql$`)

	entries, err := fs.ReadDir(MigrationsFS, MigrationsDir)
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// Map to collect up/down files by version
	migrationMap := make(map[int]*Migration)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		matches := pattern.FindStringSubmatch(entry.Name())
		if matches == nil {
			continue
		}

		version, err := strconv.Atoi(matches[1])
		if err != nil {
			continue
		}

		description := matches[2]
		direction := matches[3]

		content, err := fs.ReadFile(MigrationsFS, MigrationsDir+"/"+entry.Name())
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", entry.Name(), err)
		}

		if _, exists := migrationMap[version]; !exists {
			migrationMap[version] = &Migration{
				Version:     version,
				Description: description,
			}
		}

		switch direction {
		case "up":
			migrationMap[version].UpSQL = string(content)
		case "down":
			migrationMap[version].DownSQL = string(content)
		}
	}

	// Convert map to sorted slice
	for _, migration := range migrationMap {
		if migration.UpSQL == "" {
			return fmt.Errorf("migration %d missing up file", migration.Version)
		}
		m.migrations = append(m.migrations, *migration)
	}

	sort.Slice(m.migrations, func(i, j int) bool {
		return m.migrations[i].Version < m.migrations[j].Version
	})

	return nil
}

// ensureSchemaTable creates the schema_migrations table if it doesn't exist
func (m *Migrator) ensureSchemaTable() error {
	_, err := m.db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			description TEXT NOT NULL,
			applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

// GetCurrentVersion returns the current schema version (highest applied migration)
func (m *Migrator) GetCurrentVersion() (int, error) {
	if err := m.ensureSchemaTable(); err != nil {
		return 0, err
	}

	var version sql.NullInt64
	err := m.db.QueryRow("SELECT MAX(version) FROM schema_migrations").Scan(&version)
	if err != nil {
		return 0, err
	}

	if !version.Valid {
		return 0, nil
	}
	return int(version.Int64), nil
}

// GetAppliedVersions returns all applied migration versions
func (m *Migrator) GetAppliedVersions() ([]int, error) {
	if err := m.ensureSchemaTable(); err != nil {
		return nil, err
	}

	rows, err := m.db.Query("SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []int
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

// Up runs all pending migrations
func (m *Migrator) Up() error {
	if err := m.ensureSchemaTable(); err != nil {
		return fmt.Errorf("failed to ensure schema table: %w", err)
	}

	applied, err := m.GetAppliedVersions()
	if err != nil {
		return fmt.Errorf("failed to get applied versions: %w", err)
	}

	appliedSet := make(map[int]bool)
	for _, v := range applied {
		appliedSet[v] = true
	}

	// Run pending migrations in order
	for _, migration := range m.migrations {
		if appliedSet[migration.Version] {
			continue
		}

		log.Printf("Applying migration %03d: %s", migration.Version, migration.Description)

		if err := m.runMigration(migration, true); err != nil {
			return fmt.Errorf("migration %d failed: %w", migration.Version, err)
		}

		log.Printf("Applied migration %03d successfully", migration.Version)
	}

	return nil
}

// UpTo runs migrations up to and including the specified version
func (m *Migrator) UpTo(targetVersion int) error {
	if err := m.ensureSchemaTable(); err != nil {
		return fmt.Errorf("failed to ensure schema table: %w", err)
	}

	applied, err := m.GetAppliedVersions()
	if err != nil {
		return fmt.Errorf("failed to get applied versions: %w", err)
	}

	appliedSet := make(map[int]bool)
	for _, v := range applied {
		appliedSet[v] = true
	}

	for _, migration := range m.migrations {
		if migration.Version > targetVersion {
			break
		}
		if appliedSet[migration.Version] {
			continue
		}

		log.Printf("Applying migration %03d: %s", migration.Version, migration.Description)

		if err := m.runMigration(migration, true); err != nil {
			return fmt.Errorf("migration %d failed: %w", migration.Version, err)
		}

		log.Printf("Applied migration %03d successfully", migration.Version)
	}

	return nil
}

// Down rolls back the last applied migration
func (m *Migrator) Down() error {
	if err := m.ensureSchemaTable(); err != nil {
		return fmt.Errorf("failed to ensure schema table: %w", err)
	}

	currentVersion, err := m.GetCurrentVersion()
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	if currentVersion == 0 {
		log.Println("No migrations to rollback")
		return nil
	}

	// Find the migration to rollback
	var migration *Migration
	for i := range m.migrations {
		if m.migrations[i].Version == currentVersion {
			migration = &m.migrations[i]
			break
		}
	}

	if migration == nil {
		return fmt.Errorf("migration %d not found", currentVersion)
	}

	if migration.DownSQL == "" {
		return fmt.Errorf("migration %d has no down file", currentVersion)
	}

	log.Printf("Rolling back migration %03d: %s", migration.Version, migration.Description)

	if err := m.runMigration(*migration, false); err != nil {
		return fmt.Errorf("rollback of migration %d failed: %w", migration.Version, err)
	}

	log.Printf("Rolled back migration %03d successfully", migration.Version)

	return nil
}

// DownTo rolls back migrations down to (but not including) the specified version
func (m *Migrator) DownTo(targetVersion int) error {
	for {
		currentVersion, err := m.GetCurrentVersion()
		if err != nil {
			return err
		}

		if currentVersion <= targetVersion {
			break
		}

		if err := m.Down(); err != nil {
			return err
		}
	}
	return nil
}

// runMigration executes a single migration (up or down)
func (m *Migrator) runMigration(migration Migration, up bool) error {
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var sqlContent string
	if up {
		sqlContent = migration.UpSQL
	} else {
		sqlContent = migration.DownSQL
	}

	// Execute each statement separately (SQLite doesn't support multi-statement exec well)
	statements := splitStatements(sqlContent)
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || strings.HasPrefix(stmt, "--") {
			continue
		}
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("failed to execute statement: %s\nerror: %w", stmt, err)
		}
	}

	if up {
		// Record migration as applied
		_, err = tx.Exec(
			"INSERT INTO schema_migrations (version, description, applied_at) VALUES (?, ?, ?)",
			migration.Version, migration.Description, time.Now(),
		)
	} else {
		// Remove migration record
		_, err = tx.Exec("DELETE FROM schema_migrations WHERE version = ?", migration.Version)
	}

	if err != nil {
		return fmt.Errorf("failed to update schema_migrations: %w", err)
	}

	return tx.Commit()
}

// splitStatements splits SQL content into individual statements
func splitStatements(content string) []string {
	// Simple split on semicolon - handles most cases
	// Note: This doesn't handle semicolons in string literals, but our migrations don't have those
	var statements []string
	var current strings.Builder

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and comments at the start
		if current.Len() == 0 && (trimmed == "" || strings.HasPrefix(trimmed, "--")) {
			continue
		}

		current.WriteString(line)
		current.WriteString("\n")

		if strings.HasSuffix(trimmed, ";") {
			statements = append(statements, current.String())
			current.Reset()
		}
	}

	// Don't forget any trailing content without semicolon
	if remaining := strings.TrimSpace(current.String()); remaining != "" {
		statements = append(statements, remaining)
	}

	return statements
}

// Status returns the current migration status as a formatted string
func (m *Migrator) Status() (string, error) {
	if err := m.ensureSchemaTable(); err != nil {
		return "", err
	}

	applied, err := m.GetAppliedVersions()
	if err != nil {
		return "", err
	}

	appliedSet := make(map[int]bool)
	for _, v := range applied {
		appliedSet[v] = true
	}

	var sb strings.Builder
	sb.WriteString("Migration Status:\n")
	sb.WriteString("=================\n")

	for _, migration := range m.migrations {
		status := "pending"
		if appliedSet[migration.Version] {
			status = "applied"
		}
		sb.WriteString(fmt.Sprintf("%03d: %-30s [%s]\n", migration.Version, migration.Description, status))
	}

	if len(m.migrations) == 0 {
		sb.WriteString("No migrations found\n")
	}

	return sb.String(), nil
}

// GetLatestVersion returns the highest version number available
func (m *Migrator) GetLatestVersion() int {
	if len(m.migrations) == 0 {
		return 0
	}
	return m.migrations[len(m.migrations)-1].Version
}

// IsCurrent returns true if all migrations have been applied
func (m *Migrator) IsCurrent() (bool, error) {
	current, err := m.GetCurrentVersion()
	if err != nil {
		return false, err
	}
	return current == m.GetLatestVersion(), nil
}

// GetMigrations returns all available migrations
func (m *Migrator) GetMigrations() []Migration {
	return m.migrations
}
