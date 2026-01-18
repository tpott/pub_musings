package config

import (
	"os"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	// Clear any environment variables that might interfere
	os.Unsetenv("WHISPER_MODEL_PATH")
	os.Unsetenv("WHISPER_SERVER_PATH")
	os.Unsetenv("WHISPER_SERVER_PORT")
	os.Unsetenv("WHISPER_THREADS")
	os.Unsetenv("TEMP_DIR")
	os.Unsetenv("SERVER_PORT")
	os.Unsetenv("DATABASE_PATH")
	os.Unsetenv("DATA_DIR")

	cfg := Load()

	if cfg.WhisperServerPort != 9090 {
		t.Errorf("Expected WhisperServerPort to be 9090, got %d", cfg.WhisperServerPort)
	}

	if cfg.WhisperThreads != 4 {
		t.Errorf("Expected WhisperThreads to be 4, got %d", cfg.WhisperThreads)
	}

	if cfg.ServerPort != 8080 {
		t.Errorf("Expected ServerPort to be 8080, got %d", cfg.ServerPort)
	}

	if cfg.TempDir == "" {
		t.Error("Expected TempDir to be set to system temp directory")
	}

	if cfg.DatabasePath != "./data/db/subtitler.db" {
		t.Errorf("Expected DatabasePath to be './data/db/subtitler.db', got %s", cfg.DatabasePath)
	}

	if cfg.DataDir != "./data" {
		t.Errorf("Expected DataDir to be './data', got %s", cfg.DataDir)
	}
}

func TestLoad_FromEnv(t *testing.T) {
	os.Setenv("WHISPER_SERVER_PORT", "8888")
	os.Setenv("WHISPER_THREADS", "8")
	os.Setenv("SERVER_PORT", "3000")
	os.Setenv("DATABASE_PATH", "/custom/path/db.db")
	os.Setenv("DATA_DIR", "/custom/data")
	defer func() {
		os.Unsetenv("WHISPER_SERVER_PORT")
		os.Unsetenv("WHISPER_THREADS")
		os.Unsetenv("SERVER_PORT")
		os.Unsetenv("DATABASE_PATH")
		os.Unsetenv("DATA_DIR")
	}()

	cfg := Load()

	if cfg.WhisperServerPort != 8888 {
		t.Errorf("Expected WhisperServerPort to be 8888, got %d", cfg.WhisperServerPort)
	}

	if cfg.WhisperThreads != 8 {
		t.Errorf("Expected WhisperThreads to be 8, got %d", cfg.WhisperThreads)
	}

	if cfg.ServerPort != 3000 {
		t.Errorf("Expected ServerPort to be 3000, got %d", cfg.ServerPort)
	}

	if cfg.DatabasePath != "/custom/path/db.db" {
		t.Errorf("Expected DatabasePath to be '/custom/path/db.db', got %s", cfg.DatabasePath)
	}

	if cfg.DataDir != "/custom/data" {
		t.Errorf("Expected DataDir to be '/custom/data', got %s", cfg.DataDir)
	}
}

func TestGetEnvInt_InvalidValue(t *testing.T) {
	os.Setenv("TEST_INT", "not_a_number")
	defer os.Unsetenv("TEST_INT")

	result := getEnvInt("TEST_INT", 42)
	if result != 42 {
		t.Errorf("Expected default value 42 for invalid int, got %d", result)
	}
}
