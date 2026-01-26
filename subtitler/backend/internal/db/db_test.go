package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpen(t *testing.T) {
	// Create temporary directory for test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Open database
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Verify database file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file was not created")
	}

	// Verify foreign keys are enabled
	var fkEnabled bool
	err = db.QueryRow("PRAGMA foreign_keys").Scan(&fkEnabled)
	if err != nil {
		t.Fatalf("Failed to query foreign keys pragma: %v", err)
	}
	if !fkEnabled {
		t.Error("Foreign keys are not enabled")
	}

	// Verify Path() returns correct path
	if db.Path() != dbPath {
		t.Errorf("Path() = %v, want %v", db.Path(), dbPath)
	}
}

func TestOpen_CreatesParentDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "nested", "dirs", "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Verify parent directories were created
	parentDir := filepath.Dir(dbPath)
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		t.Error("Parent directories were not created")
	}
}

func TestMigrate(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	migrationsDir := filepath.Join(tmpDir, "migrations")

	// Create migrations directory
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatalf("Failed to create migrations directory: %v", err)
	}

	// Create a test migration
	migration := `
		CREATE TABLE test_table (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL
		);
		CREATE INDEX idx_test_name ON test_table(name);
	`
	migrationPath := filepath.Join(migrationsDir, "001_test.sql")
	if err := os.WriteFile(migrationPath, []byte(migration), 0644); err != nil {
		t.Fatalf("Failed to write test migration: %v", err)
	}

	// Initialize database with migrations
	db, err := Initialize(dbPath, migrationsDir)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Verify schema_migrations table exists
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if err != nil {
		t.Fatalf("schema_migrations table does not exist: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 migration recorded, got %d", count)
	}

	// Verify test_table was created
	err = db.QueryRow("SELECT COUNT(*) FROM test_table").Scan(&count)
	if err != nil {
		t.Errorf("test_table was not created: %v", err)
	}

	// Verify index was created
	var indexExists bool
	err = db.QueryRow(`
		SELECT COUNT(*) > 0 FROM sqlite_master
		WHERE type='index' AND name='idx_test_name'
	`).Scan(&indexExists)
	if err != nil {
		t.Fatalf("Failed to query for index: %v", err)
	}
	if !indexExists {
		t.Error("Index idx_test_name was not created")
	}
}

func TestMigrate_Idempotency(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	migrationsDir := filepath.Join(tmpDir, "migrations")

	// Create migrations directory
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatalf("Failed to create migrations directory: %v", err)
	}

	// Create a test migration
	migration := `CREATE TABLE test_table (id INTEGER PRIMARY KEY)`
	migrationPath := filepath.Join(migrationsDir, "001_test.sql")
	if err := os.WriteFile(migrationPath, []byte(migration), 0644); err != nil {
		t.Fatalf("Failed to write test migration: %v", err)
	}

	// Run migrations first time
	db, err := Initialize(dbPath, migrationsDir)
	if err != nil {
		t.Fatalf("Failed to initialize database first time: %v", err)
	}
	db.Close()

	// Run migrations second time (should not error)
	db, err = Initialize(dbPath, migrationsDir)
	if err != nil {
		t.Fatalf("Failed to initialize database second time: %v", err)
	}
	defer db.Close()

	// Verify only one migration is recorded
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count migrations: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 migration recorded after running twice, got %d", count)
	}
}

func TestInitialize_WithRealMigrations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Use the real migrations directory
	migrationsDir := filepath.Join("..", "..", "internal", "db", "migrations")

	// Initialize database
	db, err := Initialize(dbPath, migrationsDir)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Verify users table exists
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		t.Errorf("users table does not exist: %v", err)
	}

	// Verify jobs table exists
	err = db.QueryRow("SELECT COUNT(*) FROM jobs").Scan(&count)
	if err != nil {
		t.Errorf("jobs table does not exist: %v", err)
	}

	// Verify indexes exist
	indexes := []string{
		"idx_users_email",
		"idx_jobs_user_id",
		"idx_jobs_status",
		"idx_jobs_created_at",
	}

	for _, indexName := range indexes {
		var exists bool
		err = db.QueryRow(`
			SELECT COUNT(*) > 0 FROM sqlite_master
			WHERE type='index' AND name=?
		`, indexName).Scan(&exists)
		if err != nil {
			t.Fatalf("Failed to query for index %s: %v", indexName, err)
		}
		if !exists {
			t.Errorf("Index %s was not created", indexName)
		}
	}
}

func TestMigrate_TransactionRollback(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	migrationsDir := filepath.Join(tmpDir, "migrations")

	// Create migrations directory
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatalf("Failed to create migrations directory: %v", err)
	}

	// Create a migration with a syntax error
	migration := `CREATE TABLE test_table (id INTEGER PRIMARY KEY); INVALID SQL;`
	migrationPath := filepath.Join(migrationsDir, "001_bad.sql")
	if err := os.WriteFile(migrationPath, []byte(migration), 0644); err != nil {
		t.Fatalf("Failed to write test migration: %v", err)
	}

	// Initialize database (should fail)
	_, err := Initialize(dbPath, migrationsDir)
	if err == nil {
		t.Fatal("Expected migration to fail, but it succeeded")
	}

	// Open database and verify test_table doesn't exist (transaction rolled back)
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM test_table").Scan(&count)
	if err == nil {
		t.Error("test_table should not exist after failed migration, but it does")
	}
}
