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

	// Email configuration
	ResendAPIKey          string
	EmailFrom             string
	EnableEmail           bool

	// CORS configuration
	FrontendURL           string

	// Cookie configuration
	CookieSecure          bool

	// Rate limiting configuration
	RateLimitAuthPerMin    int
	RateLimitUploadPerHour int
	RateLimitPublicPerMin  int
	RateLimitDefaultPerMin int

	// Cleanup configuration
	CleanupEnabled      bool
	CleanupMaxAgeDays   int
	CleanupIntervalMins int
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
		ResendAPIKey:      getEnv("RESEND_API_KEY", ""),
		EmailFrom:         getEnv("EMAIL_FROM", "noreply@subtitler.example.com"),
		EnableEmail:       getEnvBool("ENABLE_EMAIL", false),
		FrontendURL:       getEnv("FRONTEND_URL", "http://localhost:4321"),
		CookieSecure:      getEnvBool("COOKIE_SECURE", false),
		RateLimitAuthPerMin:    getEnvInt("RATE_LIMIT_AUTH_PER_MIN", 5),
		RateLimitUploadPerHour: getEnvInt("RATE_LIMIT_UPLOAD_PER_HOUR", 10),
		RateLimitPublicPerMin:  getEnvInt("RATE_LIMIT_PUBLIC_PER_MIN", 100),
		RateLimitDefaultPerMin: getEnvInt("RATE_LIMIT_DEFAULT_PER_MIN", 60),
		CleanupEnabled:         getEnvBool("CLEANUP_ENABLED", true),
		CleanupMaxAgeDays:      getEnvInt("CLEANUP_MAX_AGE_DAYS", 30),
		CleanupIntervalMins:    getEnvInt("CLEANUP_INTERVAL_MINS", 60),
	}
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return value == "true" || value == "1"
	}
	return defaultValue
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
