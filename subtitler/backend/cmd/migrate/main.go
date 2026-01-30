// Command migrate provides CLI tools for managing database migrations.
//
// Usage:
//
//	go run ./cmd/migrate [command] [args]
//
// Commands:
//
//	up        Run all pending migrations
//	down      Rollback the last applied migration
//	status    Show current migration status
//	version   Show current schema version
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strconv"

	_ "github.com/mattn/go-sqlite3"
	"github.com/tpott/subtitler/backend/db"
	"github.com/tpott/subtitler/backend/logging"
)

func main() {
	// Initialize logging
	logging.Init(os.Getenv("LOG_LEVEL"))

	// Parse flags
	dbPath := flag.String("db", "", "Path to SQLite database (default: data/subtitler.db or DB_PATH env var)")
	flag.Parse()

	// Get database path
	path := *dbPath
	if path == "" {
		path = os.Getenv("DB_PATH")
	}
	if path == "" {
		path = "data/subtitler.db"
	}

	// Get command
	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	command := args[0]
	cmdArgs := args[1:]

	// Open database connection (raw sql.DB for migration tool)
	conn, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		logging.Fatal("Failed to open database", "error", err)
	}
	defer conn.Close()

	// Create migrator
	migrator, err := db.NewMigrator(conn)
	if err != nil {
		logging.Fatal("Failed to create migrator", "error", err)
	}

	// Execute command
	switch command {
	case "up":
		if err := cmdUp(migrator, cmdArgs); err != nil {
			logging.Fatal("Migration failed", "error", err)
		}
	case "down":
		if err := cmdDown(migrator, cmdArgs); err != nil {
			logging.Fatal("Rollback failed", "error", err)
		}
	case "status":
		if err := cmdStatus(migrator); err != nil {
			logging.Fatal("Failed to get status", "error", err)
		}
	case "version":
		if err := cmdVersion(migrator); err != nil {
			logging.Fatal("Failed to get version", "error", err)
		}
	case "help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Database Migration Tool

Usage:
  go run ./cmd/migrate [flags] <command> [args]

Flags:
  -db string    Path to SQLite database (default: data/subtitler.db or DB_PATH env var)

Commands:
  up [version]  Run migrations up to version (default: all pending)
  down [n]      Rollback n migrations (default: 1)
  status        Show migration status
  version       Show current schema version
  help          Show this help message

Examples:
  go run ./cmd/migrate up              # Run all pending migrations
  go run ./cmd/migrate up 3            # Run migrations up to version 3
  go run ./cmd/migrate down            # Rollback the last migration
  go run ./cmd/migrate down 2          # Rollback 2 migrations
  go run ./cmd/migrate status          # Show migration status
  go run ./cmd/migrate -db /path/to/db.sqlite status`)
}

func cmdUp(migrator *db.Migrator, args []string) error {
	if len(args) > 0 {
		// Run up to specific version
		version, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid version: %s", args[0])
		}
		return migrator.UpTo(version)
	}

	// Run all pending migrations
	return migrator.Up()
}

func cmdDown(migrator *db.Migrator, args []string) error {
	count := 1
	if len(args) > 0 {
		var err error
		count, err = strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid count: %s", args[0])
		}
	}

	for i := 0; i < count; i++ {
		if err := migrator.Down(); err != nil {
			return err
		}
	}
	return nil
}

func cmdStatus(migrator *db.Migrator) error {
	status, err := migrator.Status()
	if err != nil {
		return err
	}
	fmt.Print(status)
	return nil
}

func cmdVersion(migrator *db.Migrator) error {
	version, err := migrator.GetCurrentVersion()
	if err != nil {
		return err
	}
	latest := migrator.GetLatestVersion()

	fmt.Printf("Current version: %d\n", version)
	fmt.Printf("Latest version:  %d\n", latest)

	if version < latest {
		fmt.Printf("\n%d pending migration(s)\n", latest-version)
	} else if version == latest {
		fmt.Println("\nDatabase is up to date")
	}

	return nil
}
