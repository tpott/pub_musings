package db

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestMigratorLoadsMigrations(t *testing.T) {
	// Create temp database
	tmpFile, err := os.CreateTemp("", "migrate_test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	conn, err := sql.Open("sqlite3", tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	migrator, err := NewMigrator(conn)
	if err != nil {
		t.Fatalf("Failed to create migrator: %v", err)
	}

	// Verify migrations were loaded
	migrations := migrator.GetMigrations()
	if len(migrations) == 0 {
		t.Fatal("No migrations loaded")
	}

	// Check that migration 1 is the initial schema
	if migrations[0].Version != 1 {
		t.Errorf("Expected first migration version 1, got %d", migrations[0].Version)
	}
	if migrations[0].Description != "initial_schema" {
		t.Errorf("Expected first migration description 'initial_schema', got %s", migrations[0].Description)
	}
}

func TestMigratorUp(t *testing.T) {
	// Create temp database
	tmpFile, err := os.CreateTemp("", "migrate_test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	conn, err := sql.Open("sqlite3", tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	migrator, err := NewMigrator(conn)
	if err != nil {
		t.Fatalf("Failed to create migrator: %v", err)
	}

	// Run all migrations
	if err := migrator.Up(); err != nil {
		t.Fatalf("Migration up failed: %v", err)
	}

	// Verify current version
	version, err := migrator.GetCurrentVersion()
	if err != nil {
		t.Fatalf("Failed to get current version: %v", err)
	}
	if version != migrator.GetLatestVersion() {
		t.Errorf("Expected version %d, got %d", migrator.GetLatestVersion(), version)
	}

	// Verify tables were created
	tables := []string{"videos", "transcriptions", "users", "sessions", "burn_jobs",
		"recovery_codes", "password_reset_tokens", "login_attempts",
		"email_verification_tokens", "magic_link_tokens", "schema_migrations"}

	for _, table := range tables {
		var name string
		err := conn.QueryRow(`
			SELECT name FROM sqlite_master WHERE type='table' AND name=?
		`, table).Scan(&name)
		if err != nil {
			t.Errorf("Table %s not found: %v", table, err)
		}
	}
}

func TestMigratorDown(t *testing.T) {
	// Create temp database
	tmpFile, err := os.CreateTemp("", "migrate_test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	conn, err := sql.Open("sqlite3", tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	migrator, err := NewMigrator(conn)
	if err != nil {
		t.Fatalf("Failed to create migrator: %v", err)
	}

	// Run all migrations
	if err := migrator.Up(); err != nil {
		t.Fatalf("Migration up failed: %v", err)
	}

	initialVersion, _ := migrator.GetCurrentVersion()

	// Rollback
	if err := migrator.Down(); err != nil {
		t.Fatalf("Migration down failed: %v", err)
	}

	// Verify version decreased
	newVersion, _ := migrator.GetCurrentVersion()
	if newVersion >= initialVersion {
		t.Errorf("Expected version to decrease, was %d, now %d", initialVersion, newVersion)
	}
}

func TestMigratorStatus(t *testing.T) {
	// Create temp database
	tmpFile, err := os.CreateTemp("", "migrate_test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	conn, err := sql.Open("sqlite3", tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	migrator, err := NewMigrator(conn)
	if err != nil {
		t.Fatalf("Failed to create migrator: %v", err)
	}

	// Get status before any migrations
	status, err := migrator.Status()
	if err != nil {
		t.Fatalf("Failed to get status: %v", err)
	}

	if status == "" {
		t.Error("Status should not be empty")
	}
}

func TestMigratorIsCurrent(t *testing.T) {
	// Create temp database
	tmpFile, err := os.CreateTemp("", "migrate_test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	conn, err := sql.Open("sqlite3", tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	migrator, err := NewMigrator(conn)
	if err != nil {
		t.Fatalf("Failed to create migrator: %v", err)
	}

	// Should not be current before migrations
	isCurrent, err := migrator.IsCurrent()
	if err != nil {
		t.Fatalf("Failed to check if current: %v", err)
	}
	if isCurrent {
		t.Error("Should not be current before migrations")
	}

	// Run migrations
	if err := migrator.Up(); err != nil {
		t.Fatalf("Migration up failed: %v", err)
	}

	// Should be current after migrations
	isCurrent, err = migrator.IsCurrent()
	if err != nil {
		t.Fatalf("Failed to check if current: %v", err)
	}
	if !isCurrent {
		t.Error("Should be current after migrations")
	}
}

func TestMigratorUpTo(t *testing.T) {
	// Create temp database
	tmpFile, err := os.CreateTemp("", "migrate_test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	conn, err := sql.Open("sqlite3", tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	migrator, err := NewMigrator(conn)
	if err != nil {
		t.Fatalf("Failed to create migrator: %v", err)
	}

	// Run up to version 1
	if err := migrator.UpTo(1); err != nil {
		t.Fatalf("Migration up to version 1 failed: %v", err)
	}

	version, err := migrator.GetCurrentVersion()
	if err != nil {
		t.Fatalf("Failed to get current version: %v", err)
	}
	if version != 1 {
		t.Errorf("Expected version 1, got %d", version)
	}
}

func TestMigratorIdempotent(t *testing.T) {
	// Create temp database
	tmpFile, err := os.CreateTemp("", "migrate_test_*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	conn, err := sql.Open("sqlite3", tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer conn.Close()

	migrator, err := NewMigrator(conn)
	if err != nil {
		t.Fatalf("Failed to create migrator: %v", err)
	}

	// Run migrations twice - should be idempotent
	if err := migrator.Up(); err != nil {
		t.Fatalf("First migration failed: %v", err)
	}

	if err := migrator.Up(); err != nil {
		t.Fatalf("Second migration failed (should be no-op): %v", err)
	}
}

func TestSplitStatements(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "single statement",
			input:    "CREATE TABLE test (id INTEGER);",
			expected: 1,
		},
		{
			name:     "multiple statements",
			input:    "CREATE TABLE test1 (id INTEGER);\nCREATE TABLE test2 (id INTEGER);",
			expected: 2,
		},
		{
			name:     "with comments",
			input:    "-- comment\nCREATE TABLE test (id INTEGER);",
			expected: 1,
		},
		{
			name:     "multiline statement",
			input:    "CREATE TABLE test (\n  id INTEGER,\n  name TEXT\n);",
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statements := splitStatements(tt.input)
			if len(statements) != tt.expected {
				t.Errorf("Expected %d statements, got %d: %v", tt.expected, len(statements), statements)
			}
		})
	}
}
