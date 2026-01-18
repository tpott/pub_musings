package config

import (
	"os"
	"strconv"
)

// Config holds application configuration
type Config struct {
	// Whisper configuration
	WhisperModelPath      string
	WhisperServerPath     string
	WhisperServerPort     int
	WhisperThreads        int
	TempDir               string

	// Server configuration
	ServerPort            int

	// Database configuration
	DatabasePath          string
	DataDir               string

	// Auth configuration
	JWTSecret             string
}

// Load returns a Config with values from environment variables or defaults
func Load() *Config {
	return &Config{
		WhisperModelPath:  getEnv("WHISPER_MODEL_PATH", os.ExpandEnv("$HOME/Github/whisper.cpp/models/ggml-medium.bin")),
		WhisperServerPath: getEnv("WHISPER_SERVER_PATH", os.ExpandEnv("$HOME/Github/whisper.cpp/build/bin/whisper-server")),
		WhisperServerPort: getEnvInt("WHISPER_SERVER_PORT", 9090),
		WhisperThreads:    getEnvInt("WHISPER_THREADS", 4),
		TempDir:           getEnv("TEMP_DIR", os.TempDir()),
		ServerPort:        getEnvInt("SERVER_PORT", 8080),
		DatabasePath:      getEnv("DATABASE_PATH", "./data/db/subtitler.db"),
		DataDir:           getEnv("DATA_DIR", "./data"),
		JWTSecret:         getEnv("JWT_SECRET", "dev-secret-change-in-production"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}
