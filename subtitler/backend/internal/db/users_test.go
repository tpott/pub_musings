package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateUser(t *testing.T) {
	// Create a temporary database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Run migrations
	migrationsDir := "./migrations"
	if err := db.Migrate(migrationsDir); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Create a user
	email := "test@example.com"
	passwordHash := "hashedpassword123"

	user, err := db.CreateUser(email, passwordHash)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	if user.ID == 0 {
		t.Error("Expected user ID to be non-zero")
	}

	if user.Email != email {
		t.Errorf("Expected email %s, got %s", email, user.Email)
	}

	if user.PasswordHash != passwordHash {
		t.Errorf("Expected password hash %s, got %s", passwordHash, user.PasswordHash)
	}

	if user.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}

	if user.UpdatedAt.IsZero() {
		t.Error("Expected UpdatedAt to be set")
	}
}

func TestCreateUser_DuplicateEmail(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	migrationsDir := "./migrations"
	if err := db.Migrate(migrationsDir); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	email := "test@example.com"
	passwordHash := "hashedpassword123"

	// Create first user
	_, err = db.CreateUser(email, passwordHash)
	if err != nil {
		t.Fatalf("First CreateUser failed: %v", err)
	}

	// Try to create duplicate user
	_, err = db.CreateUser(email, "differentpassword")
	if err != ErrDuplicateEmail {
		t.Errorf("Expected ErrDuplicateEmail, got: %v", err)
	}
}

func TestGetUserByEmail(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	migrationsDir := "./migrations"
	if err := db.Migrate(migrationsDir); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	email := "test@example.com"
	passwordHash := "hashedpassword123"

	// Create a user
	createdUser, err := db.CreateUser(email, passwordHash)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Get user by email
	user, err := db.GetUserByEmail(email)
	if err != nil {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}

	if user.ID != createdUser.ID {
		t.Errorf("Expected ID %d, got %d", createdUser.ID, user.ID)
	}

	if user.Email != email {
		t.Errorf("Expected email %s, got %s", email, user.Email)
	}

	if user.PasswordHash != passwordHash {
		t.Errorf("Expected password hash %s, got %s", passwordHash, user.PasswordHash)
	}
}

func TestGetUserByEmail_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	migrationsDir := "./migrations"
	if err := db.Migrate(migrationsDir); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Try to get non-existent user
	_, err = db.GetUserByEmail("nonexistent@example.com")
	if err != ErrUserNotFound {
		t.Errorf("Expected ErrUserNotFound, got: %v", err)
	}
}

func TestGetUserByID(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	migrationsDir := "./migrations"
	if err := db.Migrate(migrationsDir); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	email := "test@example.com"
	passwordHash := "hashedpassword123"

	// Create a user
	createdUser, err := db.CreateUser(email, passwordHash)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Get user by ID
	user, err := db.GetUserByID(createdUser.ID)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}

	if user.ID != createdUser.ID {
		t.Errorf("Expected ID %d, got %d", createdUser.ID, user.ID)
	}

	if user.Email != email {
		t.Errorf("Expected email %s, got %s", email, user.Email)
	}

	if user.PasswordHash != passwordHash {
		t.Errorf("Expected password hash %s, got %s", passwordHash, user.PasswordHash)
	}
}

func TestGetUserByID_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	migrationsDir := "./migrations"
	if err := db.Migrate(migrationsDir); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Try to get non-existent user
	_, err = db.GetUserByID(999)
	if err != ErrUserNotFound {
		t.Errorf("Expected ErrUserNotFound, got: %v", err)
	}
}

func TestMain(m *testing.M) {
	// Ensure migrations directory exists for tests
	if _, err := os.Stat("./migrations"); os.IsNotExist(err) {
		// Try parent directory (when running from backend/)
		if _, err := os.Stat("../../internal/db/migrations"); err == nil {
			// Create a symlink for tests
			os.Symlink("../../internal/db/migrations", "./migrations")
		}
	}

	code := m.Run()

	// Cleanup
	os.Remove("./migrations")

	os.Exit(code)
}
