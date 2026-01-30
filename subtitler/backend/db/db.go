package db

import (
	"context"
	"database/sql"
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
