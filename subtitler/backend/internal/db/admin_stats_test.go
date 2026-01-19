package db

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) *DB {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	migrationsDir := filepath.Join(tmpDir, "migrations")

	// Create migrations directory
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatalf("Failed to create migrations directory: %v", err)
	}

	// Create initial schema migration
	migration := `
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			is_admin BOOLEAN NOT NULL DEFAULT FALSE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX idx_users_email ON users(email);

		CREATE TABLE jobs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			status TEXT NOT NULL CHECK(status IN ('pending', 'processing', 'completed', 'failed')),
			original_filename TEXT NOT NULL,
			file_path TEXT NOT NULL,
			file_size INTEGER NOT NULL,
			output_format TEXT NOT NULL CHECK(output_format IN ('srt', 'vtt', 'embedded')),
			transcript_path TEXT,
			error_message TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		);
		CREATE INDEX idx_jobs_user_id ON jobs(user_id);
		CREATE INDEX idx_jobs_status ON jobs(status);
		CREATE INDEX idx_jobs_created_at ON jobs(created_at);
	`
	migrationPath := filepath.Join(migrationsDir, "001_initial.sql")
	if err := os.WriteFile(migrationPath, []byte(migration), 0644); err != nil {
		t.Fatalf("Failed to write test migration: %v", err)
	}

	db, err := Initialize(dbPath, migrationsDir)
	if err != nil {
		t.Fatalf("Failed to initialize test database: %v", err)
	}

	return db
}

func TestGetUserStats(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Get stats when empty
	now := time.Now()
	start := now.AddDate(0, 0, -30)
	stats, err := db.GetUserStats(start, now)
	if err != nil {
		t.Fatalf("GetUserStats failed: %v", err)
	}
	if stats.Total != 0 {
		t.Errorf("Expected 0 total users, got %d", stats.Total)
	}

	// Create test users
	user1, err := db.CreateUser("user1@test.com", "hash1")
	if err != nil {
		t.Fatalf("Failed to create user1: %v", err)
	}
	user2, err := db.CreateUser("user2@test.com", "hash2")
	if err != nil {
		t.Fatalf("Failed to create user2: %v", err)
	}

	// Create a job for user1 (to count as active)
	_, err = db.Exec(`
		INSERT INTO jobs (user_id, status, original_filename, file_path, file_size, output_format)
		VALUES (?, 'completed', 'test.mp3', '/path/test.mp3', 1000, 'srt')
	`, user1.ID)
	if err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	// Get stats
	stats, err = db.GetUserStats(start, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("GetUserStats failed: %v", err)
	}

	if stats.Total != 2 {
		t.Errorf("Expected 2 total users, got %d", stats.Total)
	}
	if stats.NewThisPeriod != 2 {
		t.Errorf("Expected 2 new users, got %d", stats.NewThisPeriod)
	}
	if stats.ActiveThisPeriod != 1 {
		t.Errorf("Expected 1 active user, got %d", stats.ActiveThisPeriod)
	}

	_ = user2 // Silence unused variable warning
}

func TestGetJobStats(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create test user
	user, err := db.CreateUser("user@test.com", "hash")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Get stats when empty
	now := time.Now()
	start := now.AddDate(0, 0, -30)
	stats, err := db.GetJobStats(start, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("GetJobStats failed: %v", err)
	}
	if stats.Total != 0 {
		t.Errorf("Expected 0 total jobs, got %d", stats.Total)
	}

	// Create test jobs
	testJobs := []struct {
		status string
		format string
	}{
		{"completed", "srt"},
		{"completed", "srt"},
		{"completed", "vtt"},
		{"failed", "srt"},
		{"pending", "embedded"},
	}

	for i, j := range testJobs {
		_, err = db.Exec(`
			INSERT INTO jobs (user_id, status, original_filename, file_path, file_size, output_format)
			VALUES (?, ?, ?, ?, ?, ?)
		`, user.ID, j.status, "test.mp3", "/path/test.mp3", 1000*(i+1), j.format)
		if err != nil {
			t.Fatalf("Failed to create job: %v", err)
		}
	}

	// Get stats
	stats, err = db.GetJobStats(start, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("GetJobStats failed: %v", err)
	}

	if stats.Total != 5 {
		t.Errorf("Expected 5 total jobs, got %d", stats.Total)
	}
	if stats.Completed != 3 {
		t.Errorf("Expected 3 completed jobs, got %d", stats.Completed)
	}
	if stats.Failed != 1 {
		t.Errorf("Expected 1 failed job, got %d", stats.Failed)
	}
	if stats.Pending != 1 {
		t.Errorf("Expected 1 pending job, got %d", stats.Pending)
	}

	// Check format breakdown
	if stats.ByFormat["srt"] != 3 {
		t.Errorf("Expected 3 SRT jobs, got %d", stats.ByFormat["srt"])
	}
	if stats.ByFormat["vtt"] != 1 {
		t.Errorf("Expected 1 VTT job, got %d", stats.ByFormat["vtt"])
	}
	if stats.ByFormat["embedded"] != 1 {
		t.Errorf("Expected 1 embedded job, got %d", stats.ByFormat["embedded"])
	}
}

func TestGetStorageStats(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create test user
	user, err := db.CreateUser("user@test.com", "hash")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Get stats when empty
	stats, err := db.GetStorageStats()
	if err != nil {
		t.Fatalf("GetStorageStats failed: %v", err)
	}
	if stats.TotalBytes != 0 {
		t.Errorf("Expected 0 total bytes, got %d", stats.TotalBytes)
	}

	// Create jobs with different file sizes
	_, err = db.Exec(`
		INSERT INTO jobs (user_id, status, original_filename, file_path, file_size, output_format)
		VALUES (?, 'completed', 'test1.mp3', '/path/test1.mp3', 1000000, 'srt')
	`, user.ID)
	if err != nil {
		t.Fatalf("Failed to create job 1: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO jobs (user_id, status, original_filename, file_path, file_size, output_format)
		VALUES (?, 'completed', 'test2.mp3', '/path/test2.mp3', 2000000, 'vtt')
	`, user.ID)
	if err != nil {
		t.Fatalf("Failed to create job 2: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO jobs (user_id, status, original_filename, file_path, file_size, output_format)
		VALUES (?, 'completed', 'test3.mp4', '/path/test3.mp4', 5000000, 'embedded')
	`, user.ID)
	if err != nil {
		t.Fatalf("Failed to create job 3: %v", err)
	}

	// Get stats
	stats, err = db.GetStorageStats()
	if err != nil {
		t.Fatalf("GetStorageStats failed: %v", err)
	}

	// Total uploads should be sum of all file sizes
	expectedUploads := int64(1000000 + 2000000 + 5000000)
	if stats.UploadsBytes != expectedUploads {
		t.Errorf("Expected %d uploads bytes, got %d", expectedUploads, stats.UploadsBytes)
	}

	// Results should include estimated SRT/VTT size (1%) + embedded size
	// SRT/VTT: (1000000 + 2000000) / 100 = 30000
	// Embedded: 5000000
	expectedResults := int64(30000 + 5000000)
	if stats.ResultsBytes != expectedResults {
		t.Errorf("Expected %d results bytes, got %d", expectedResults, stats.ResultsBytes)
	}

	if stats.TotalBytes != stats.UploadsBytes+stats.ResultsBytes {
		t.Errorf("TotalBytes should equal UploadsBytes + ResultsBytes")
	}
}

func TestUserIsAdmin(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create non-admin user
	user, err := db.CreateUser("user@test.com", "hash")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Verify user is not admin by default
	if user.IsAdmin {
		t.Error("User should not be admin by default")
	}

	// Set user as admin
	_, err = db.Exec("UPDATE users SET is_admin = TRUE WHERE id = ?", user.ID)
	if err != nil {
		t.Fatalf("Failed to set admin flag: %v", err)
	}

	// Fetch user again and verify admin status
	user, err = db.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}
	if !user.IsAdmin {
		t.Error("User should be admin after update")
	}

	// Test GetUserByEmail returns admin status
	user, err = db.GetUserByEmail("user@test.com")
	if err != nil {
		t.Fatalf("Failed to get user by email: %v", err)
	}
	if !user.IsAdmin {
		t.Error("GetUserByEmail should return admin status")
	}
}
