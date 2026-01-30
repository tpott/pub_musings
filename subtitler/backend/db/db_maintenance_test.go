package db

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestOpenAndMigrate(t *testing.T) {
	// Use temp file for test database
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()
}

func TestVacuum(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Add some data then delete it to create fragmentation
	for i := 0; i < 100; i++ {
		video := &Video{
			ID:          fmt.Sprintf("vacuum-test-%d", i),
			Filename:    "test.mp4",
			Size:        1024,
			ContentType: "video/mp4",
			FilePath:    "/tmp/test.mp4",
			CreatedAt:   time.Now(),
		}
		if err := db.CreateVideo(video); err != nil {
			t.Fatalf("Failed to create video %d: %v", i, err)
		}
	}

	// Delete the videos to create free space
	for i := 0; i < 100; i++ {
		db.DeleteVideo(fmt.Sprintf("vacuum-test-%d", i))
	}

	// VACUUM should reclaim the space
	if err := db.Vacuum(); err != nil {
		t.Fatalf("Vacuum failed: %v", err)
	}
}

func TestAnalyze(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Add some data to have statistics to analyze
	for i := 0; i < 10; i++ {
		video := &Video{
			ID:          fmt.Sprintf("analyze-test-%d", i),
			Filename:    "test.mp4",
			Size:        1024,
			ContentType: "video/mp4",
			FilePath:    "/tmp/test.mp4",
			CreatedAt:   time.Now(),
		}
		if err := db.CreateVideo(video); err != nil {
			t.Fatalf("Failed to create video %d: %v", i, err)
		}
	}

	// ANALYZE should update query planner statistics
	if err := db.Analyze(); err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
}

func TestMaintenance(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Add some test data
	video := &Video{
		ID:          "maintenance-test",
		Filename:    "test.mp4",
		Size:        1024,
		ContentType: "video/mp4",
		FilePath:    "/tmp/test.mp4",
		CreatedAt:   time.Now(),
	}
	if err := db.CreateVideo(video); err != nil {
		t.Fatalf("Failed to create video: %v", err)
	}

	// Maintenance runs both VACUUM and ANALYZE
	if err := db.Maintenance(); err != nil {
		t.Fatalf("Maintenance failed: %v", err)
	}

	// Verify data is still intact after maintenance
	retrieved, err := db.GetVideo("maintenance-test")
	if err != nil {
		t.Fatalf("Failed to get video after maintenance: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Video not found after maintenance")
	}
	if retrieved.Filename != "test.mp4" {
		t.Errorf("Expected filename 'test.mp4', got '%s'", retrieved.Filename)
	}
}

func TestQueryTimeout(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	db, err := Open(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	t.Run("default timeout is 30 seconds", func(t *testing.T) {
		timeout := db.GetQueryTimeout()
		if timeout != DefaultQueryTimeout {
			t.Errorf("Expected default timeout %v, got %v", DefaultQueryTimeout, timeout)
		}
		if timeout != 30*time.Second {
			t.Errorf("Expected 30s timeout, got %v", timeout)
		}
	})

	t.Run("can set custom timeout", func(t *testing.T) {
		db.SetQueryTimeout(10 * time.Second)
		timeout := db.GetQueryTimeout()
		if timeout != 10*time.Second {
			t.Errorf("Expected 10s timeout, got %v", timeout)
		}
		// Reset to default
		db.SetQueryTimeout(DefaultQueryTimeout)
	})

	t.Run("query works with context timeout", func(t *testing.T) {
		// Create a video
		video := &Video{
			ID:          "timeout-test-video",
			Filename:    "test.mp4",
			Size:        1024,
			ContentType: "video/mp4",
			FilePath:    "/uploads/test.mp4",
			CreatedAt:   time.Now(),
		}
		if err := db.CreateVideo(video); err != nil {
			t.Fatalf("Failed to create video: %v", err)
		}

		// Retrieve it - this uses QueryRowContext internally
		retrieved, err := db.GetVideo("timeout-test-video")
		if err != nil {
			t.Fatalf("Failed to get video: %v", err)
		}
		if retrieved == nil {
			t.Fatal("Expected video to be found")
		}
		if retrieved.Filename != "test.mp4" {
			t.Errorf("Expected filename 'test.mp4', got '%s'", retrieved.Filename)
		}
	})
}

func TestOpenWithConfig(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-pool-*.db")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	t.Run("default config uses default values", func(t *testing.T) {
		cfg := DefaultPoolConfig()
		if cfg.MaxOpenConns != DefaultMaxOpenConns {
			t.Errorf("Expected MaxOpenConns %d, got %d", DefaultMaxOpenConns, cfg.MaxOpenConns)
		}
		if cfg.MaxIdleConns != DefaultMaxIdleConns {
			t.Errorf("Expected MaxIdleConns %d, got %d", DefaultMaxIdleConns, cfg.MaxIdleConns)
		}
	})

	t.Run("opens with custom pool config", func(t *testing.T) {
		cfg := PoolConfig{
			MaxOpenConns: 20,
			MaxIdleConns: 10,
		}
		db, err := OpenWithConfig(tmpFile.Name(), cfg)
		if err != nil {
			t.Fatalf("Failed to open database: %v", err)
		}
		defer db.Close()

		// Verify database is functional
		if err := db.Ping(); err != nil {
			t.Errorf("Ping failed: %v", err)
		}
	})

	t.Run("opens with zero pool limits (unlimited)", func(t *testing.T) {
		tmpFile2, err := os.CreateTemp("", "test-pool-unlimited-*.db")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		tmpFile2.Close()
		defer os.Remove(tmpFile2.Name())

		cfg := PoolConfig{
			MaxOpenConns: 0, // unlimited
			MaxIdleConns: 0, // unlimited
		}
		db, err := OpenWithConfig(tmpFile2.Name(), cfg)
		if err != nil {
			t.Fatalf("Failed to open database with unlimited pool: %v", err)
		}
		defer db.Close()

		// Verify database is functional
		if err := db.Ping(); err != nil {
			t.Errorf("Ping failed: %v", err)
		}
	})

	t.Run("Open uses default pool config", func(t *testing.T) {
		tmpFile3, err := os.CreateTemp("", "test-pool-default-*.db")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		tmpFile3.Close()
		defer os.Remove(tmpFile3.Name())

		// Open without explicit config should work
		db, err := Open(tmpFile3.Name())
		if err != nil {
			t.Fatalf("Failed to open database: %v", err)
		}
		defer db.Close()

		// Verify database is functional
		if err := db.Ping(); err != nil {
			t.Errorf("Ping failed: %v", err)
		}
	})
}
