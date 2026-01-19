package cleanup

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/trevor/subtitler/internal/db"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 bytes"},
		{100, "100 bytes"},
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
		{1048576, "1.00 MB"},
		{1572864, "1.50 MB"},
		{1073741824, "1.00 GB"},
		{1610612736, "1.50 GB"},
	}

	for _, tc := range tests {
		result := formatBytes(tc.bytes)
		if result != tc.expected {
			t.Errorf("formatBytes(%d) = %q, expected %q", tc.bytes, result, tc.expected)
		}
	}
}

func TestDeleteFileIfExists(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")

	content := []byte("test content")
	err := os.WriteFile(tmpFile, content, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Delete the file
	size, err := deleteFileIfExists(tmpFile)
	if err != nil {
		t.Fatalf("deleteFileIfExists failed: %v", err)
	}

	if size != int64(len(content)) {
		t.Errorf("Expected size %d, got %d", len(content), size)
	}

	// Verify file is deleted
	if _, err := os.Stat(tmpFile); !os.IsNotExist(err) {
		t.Error("File should not exist after deletion")
	}

	// Try to delete non-existent file (should not error)
	size, err = deleteFileIfExists(tmpFile)
	if err != nil {
		t.Errorf("deleteFileIfExists should not error for non-existent file: %v", err)
	}
	if size != 0 {
		t.Errorf("Expected size 0 for non-existent file, got %d", size)
	}
}

func TestRemoveEmptyParentDirs(t *testing.T) {
	// Create a nested directory structure
	tmpDir := t.TempDir()
	nestedDir := filepath.Join(tmpDir, "a", "b", "c")
	err := os.MkdirAll(nestedDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create nested dirs: %v", err)
	}

	// Create a file in the deepest directory
	tmpFile := filepath.Join(nestedDir, "test.txt")
	err = os.WriteFile(tmpFile, []byte("test"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Remove the file
	os.Remove(tmpFile)

	// Try to remove empty parent dirs (2 levels)
	removeEmptyParentDirs(tmpFile, 2)

	// Check that c and b were removed, but a still exists
	if _, err := os.Stat(filepath.Join(tmpDir, "a", "b", "c")); !os.IsNotExist(err) {
		t.Error("Directory 'c' should have been removed")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "a", "b")); !os.IsNotExist(err) {
		t.Error("Directory 'b' should have been removed")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "a")); os.IsNotExist(err) {
		t.Error("Directory 'a' should still exist (only 2 levels removed)")
	}
}

func TestServiceStartStop(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping service start/stop test in short mode")
	}

	// Initialize test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	migrationsDir := "../../internal/db/migrations"

	database, err := db.Initialize(dbPath, migrationsDir)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	// Create cleanup service with very short interval for testing
	service := NewService(database, 30, 1) // 1 minute interval

	// Start the service
	service.Start()

	// Wait a moment for initial cleanup to run
	time.Sleep(100 * time.Millisecond)

	// Verify service is running
	service.mu.Lock()
	running := service.running
	service.mu.Unlock()

	if !running {
		t.Error("Service should be running after Start()")
	}

	// Stop the service
	service.Stop()

	// Verify service is stopped
	service.mu.Lock()
	running = service.running
	service.mu.Unlock()

	if running {
		t.Error("Service should not be running after Stop()")
	}
}

func TestServiceStats(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping service stats test in short mode")
	}

	// Initialize test database
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	migrationsDir := "../../internal/db/migrations"

	database, err := db.Initialize(dbPath, migrationsDir)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	service := NewService(database, 30, 60)

	// Initial stats should be zero
	cleanupCount, bytesFreed, lastCleanup := service.Stats()
	if cleanupCount != 0 || bytesFreed != 0 {
		t.Error("Initial stats should be zero")
	}
	if !lastCleanup.IsZero() {
		t.Error("Last cleanup should be zero initially")
	}

	// Run a cleanup manually
	service.runCleanup()

	// Check that lastCleanup was updated
	_, _, lastCleanup = service.Stats()
	if lastCleanup.IsZero() {
		t.Error("Last cleanup should be updated after runCleanup()")
	}
}
