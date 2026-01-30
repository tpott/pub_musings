package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tpott/subtitler/backend/auth"
	"github.com/tpott/subtitler/backend/db"
	"github.com/tpott/subtitler/backend/logging"
	"github.com/tpott/subtitler/backend/validation"
)

// Configuration defaults (can be overridden via environment variables)
const (
	defaultMaxUploadSize = 500 << 20 // 500 MB
	defaultUploadDir     = "uploads"
	defaultDBPath        = "data/subtitler.db"
	defaultKeyPath       = "data/age.key"

	// Rate limit defaults (requests per window)
	defaultAuthRateLimit          = 5
	defaultAuthRateWindow         = time.Minute
	defaultPasswordResetRateLimit = 3
	defaultPasswordResetWindow    = 15 * time.Minute
	defaultUploadRateLimit        = 10
	defaultUploadRateWindow       = time.Minute
	defaultTranscribeRateLimit    = 5
	defaultTranscribeRateWindow   = time.Minute
	defaultBurnRateLimit          = 2
	defaultBurnRateWindow         = time.Minute
	defaultScriptRateLimit        = 10
	defaultScriptRateWindow       = time.Minute
	defaultChunkRateLimit         = 60
	defaultChunkRateWindow        = time.Minute
	defaultDownloadRateLimit      = 30
	defaultDownloadRateWindow     = time.Minute
	defaultUserRateLimit          = 60
	defaultUserRateWindow         = time.Minute
	defaultMetricsRateLimit       = 10
	defaultMetricsRateWindow      = time.Minute
	defaultFeedbackRateLimit      = 5
	defaultFeedbackRateWindow     = time.Minute
	defaultLogRateLimit           = 30
	defaultLogRateWindow          = time.Minute

	// Login lockout defaults
	defaultMaxLoginAttempts  = 5
	defaultLoginLockDuration = 15 * time.Minute

	// Chunked upload defaults
	defaultChunkSize     = 50 << 20       // 50 MB
	defaultSessionExpiry = 24 * time.Hour // 24 hours

	// Database maintenance defaults
	defaultDBMaintenanceInterval = 24 * time.Hour // Run VACUUM and ANALYZE daily

	// Whisper transcription defaults
	defaultWhisperThreads  = 4                // Number of threads for whisper-cli
	defaultWhisperTimeout  = 30 * time.Minute // Timeout for whisper-server requests
	maxWhisperResponseSize = 100 << 20        // 100 MB max response from whisper-server (prevents memory exhaustion)

)

// Size range limits to prevent misconfiguration and potential issues
const (
	minReasonableSize = 1 << 20  // 1 MB minimum
	maxReasonableSize = 10 << 30 // 10 GB maximum
)

// Allowed video MIME types for upload validation (used by both single-file and chunked upload handlers)
var allowedMIMETypes = map[string]bool{
	"video/mp4":        true,
	"video/webm":       true,
	"video/quicktime":  true, // .mov files
	"video/x-m4v":      true, // .m4v files
	"video/mpeg":       true, // .mpeg, .mpg files
	"video/x-msvideo":  true, // .avi files
	"video/x-matroska": true, // .mkv files
	"video/ogg":        true, // .ogv files
}

// Graceful shutdown configuration
const (
	defaultShutdownTimeout = 30 * time.Second // Time allowed for graceful shutdown
)

// Configuration values loaded from environment
var (
	maxUploadSize int64
	uploadDir     string
	dbPath        string
	keyPath       string

	// Rate limit configuration
	authRateLimit          int
	authRateWindow         time.Duration
	passwordResetRateLimit int
	passwordResetWindow    time.Duration
	uploadRateLimit        int
	uploadRateWindow       time.Duration
	transcribeRateLimit    int
	transcribeRateWindow   time.Duration
	burnRateLimit          int
	burnRateWindow         time.Duration
	scriptRateLimit        int
	scriptRateWindow       time.Duration
	chunkRateLimit         int
	chunkRateWindow        time.Duration
	downloadRateLimit      int
	downloadRateWindow     time.Duration
	userRateLimit          int
	userRateWindow         time.Duration
	metricsRateLimit       int
	metricsRateWindow      time.Duration
	feedbackRateLimit      int
	feedbackRateWindow     time.Duration
	logRateLimit           int
	logRateWindow          time.Duration

	// Login lockout configuration
	maxLoginAttempts  int
	loginLockDuration time.Duration

	// Chunked upload configuration
	chunkSize     int64
	sessionExpiry time.Duration

	// Database maintenance configuration
	dbMaintenanceInterval time.Duration

	// Database connection pool configuration
	dbMaxOpenConns int
	dbMaxIdleConns int

	// Subtitle font configuration
	subtitleFont string

	// Metrics API key (optional, for /metrics endpoint authentication)
	metricsAPIKey string

	// Whisper transcription configuration
	whisperThreads     int
	whisperTemperature string
	whisperTimeout     time.Duration
)

// getEnvOrDefault returns the value of an environment variable or a default
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvIntOrDefault parses a non-negative integer from environment variable.
// Returns default value if not set, invalid, or negative.
func getEnvIntOrDefault(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		logging.Warn("Invalid integer, using default", "key", key, "value", value, "default", defaultValue)
		return defaultValue
	}
	if n < 0 {
		logging.Warn("Negative value not allowed, using default", "key", key, "value", n, "default", defaultValue)
		return defaultValue
	}
	return n
}

// getEnvSizeOrDefault parses a size from environment variable (e.g., "500M", "1G")
// Returns default value if not set or invalid.
// Validates that the value is within reasonable bounds (1MB to 10GB) to prevent
// misconfiguration and potential integer overflow issues.
func getEnvSizeOrDefault(key string, defaultValue int64) int64 {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	// Parse size with optional suffix (M for MB, G for GB)
	rawValue := value
	value = strings.TrimSpace(strings.ToUpper(value))
	multiplier := int64(1)

	if strings.HasSuffix(value, "G") {
		multiplier = 1 << 30 // GB
		value = strings.TrimSuffix(value, "G")
	} else if strings.HasSuffix(value, "M") {
		multiplier = 1 << 20 // MB
		value = strings.TrimSuffix(value, "M")
	} else if strings.HasSuffix(value, "K") {
		multiplier = 1 << 10 // KB
		value = strings.TrimSuffix(value, "K")
	}

	var size int64
	if _, err := fmt.Sscanf(value, "%d", &size); err != nil {
		logging.Warn("Invalid size value, using default", "key", key, "value", rawValue, "default_bytes", defaultValue)
		return defaultValue
	}

	result := size * multiplier

	// Validate range to prevent misconfiguration
	if result < minReasonableSize {
		logging.Warn("Size below minimum (1MB), using default",
			"key", key, "value", rawValue, "parsed_bytes", result, "default_bytes", defaultValue)
		return defaultValue
	}
	if result > maxReasonableSize {
		logging.Warn("Size above maximum (10GB), using default",
			"key", key, "value", rawValue, "parsed_bytes", result, "default_bytes", defaultValue)
		return defaultValue
	}

	return result
}

// parseRateLimit parses a rate limit string like "5/min" or "10/hour"
// Returns (count, window) or (defaultCount, defaultWindow) if parsing fails
func parseRateLimit(value string, defaultCount int, defaultWindow time.Duration) (int, time.Duration) {
	if value == "" {
		return defaultCount, defaultWindow
	}

	// Parse format: "count/window" where window is min, hour, or s
	parts := strings.SplitN(value, "/", 2)
	if len(parts) != 2 {
		logging.Warn("Invalid rate limit format, using default", "value", value)
		return defaultCount, defaultWindow
	}

	var count int
	if _, err := fmt.Sscanf(parts[0], "%d", &count); err != nil || count <= 0 {
		logging.Warn("Invalid rate limit count, using default", "value", parts[0])
		return defaultCount, defaultWindow
	}

	windowStr := strings.TrimSpace(strings.ToLower(parts[1]))
	var window time.Duration
	switch windowStr {
	case "s", "sec", "second":
		window = time.Second
	case "m", "min", "minute":
		window = time.Minute
	case "h", "hr", "hour":
		window = time.Hour
	default:
		// Try parsing as duration (e.g., "15m", "1h")
		var err error
		window, err = time.ParseDuration(windowStr)
		if err != nil {
			logging.Warn("Invalid rate limit window, using default", "value", windowStr)
			return defaultCount, defaultWindow
		}
	}

	return count, window
}

// getEnvRateLimitOrDefault parses a rate limit from environment variable
func getEnvRateLimitOrDefault(key string, defaultCount int, defaultWindow time.Duration) (int, time.Duration) {
	return parseRateLimit(os.Getenv(key), defaultCount, defaultWindow)
}

// getEnvDurationOrDefault parses a duration from environment variable.
// Supports formats like "24h", "30m", "1h30m", etc. (Go time.Duration format)
// Set to "0" or "disabled" to disable the feature.
func getEnvDurationOrDefault(key string, defaultValue time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	if value == "0" || strings.ToLower(value) == "disabled" {
		return 0
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		logging.Warn("Invalid duration, using default", "key", key, "value", value, "default", defaultValue)
		return defaultValue
	}
	return d
}

// initConfig initializes configuration from environment variables
func initConfig() {
	maxUploadSize = getEnvSizeOrDefault("MAX_UPLOAD_SIZE", defaultMaxUploadSize)
	uploadDir = getEnvOrDefault("UPLOAD_DIR", defaultUploadDir)
	dbPath = getEnvOrDefault("DB_PATH", defaultDBPath)
	keyPath = getEnvOrDefault("KEY_PATH", defaultKeyPath)

	// Validate directory/file paths for security (path traversal, null bytes)
	if err := validation.ValidateFilePath(uploadDir); err != nil {
		logging.Fatal("Invalid UPLOAD_DIR path", "path", uploadDir, "error", err)
	}
	uploadDir = filepath.Clean(uploadDir)

	if err := validation.ValidateFilePath(dbPath); err != nil {
		logging.Fatal("Invalid DB_PATH path", "path", dbPath, "error", err)
	}
	dbPath = filepath.Clean(dbPath)

	if err := validation.ValidateFilePath(keyPath); err != nil {
		logging.Fatal("Invalid KEY_PATH path", "path", keyPath, "error", err)
	}
	keyPath = filepath.Clean(keyPath)

	// Rate limit configuration
	authRateLimit, authRateWindow = getEnvRateLimitOrDefault("AUTH_RATE_LIMIT", defaultAuthRateLimit, defaultAuthRateWindow)
	passwordResetRateLimit, passwordResetWindow = getEnvRateLimitOrDefault("PASSWORD_RESET_RATE_LIMIT", defaultPasswordResetRateLimit, defaultPasswordResetWindow)
	uploadRateLimit, uploadRateWindow = getEnvRateLimitOrDefault("UPLOAD_RATE_LIMIT", defaultUploadRateLimit, defaultUploadRateWindow)
	transcribeRateLimit, transcribeRateWindow = getEnvRateLimitOrDefault("TRANSCRIBE_RATE_LIMIT", defaultTranscribeRateLimit, defaultTranscribeRateWindow)
	burnRateLimit, burnRateWindow = getEnvRateLimitOrDefault("BURN_RATE_LIMIT", defaultBurnRateLimit, defaultBurnRateWindow)
	scriptRateLimit, scriptRateWindow = getEnvRateLimitOrDefault("SCRIPT_RATE_LIMIT", defaultScriptRateLimit, defaultScriptRateWindow)
	chunkRateLimit, chunkRateWindow = getEnvRateLimitOrDefault("CHUNK_RATE_LIMIT", defaultChunkRateLimit, defaultChunkRateWindow)
	downloadRateLimit, downloadRateWindow = getEnvRateLimitOrDefault("DOWNLOAD_RATE_LIMIT", defaultDownloadRateLimit, defaultDownloadRateWindow)
	userRateLimit, userRateWindow = getEnvRateLimitOrDefault("USER_RATE_LIMIT", defaultUserRateLimit, defaultUserRateWindow)
	metricsRateLimit, metricsRateWindow = getEnvRateLimitOrDefault("METRICS_RATE_LIMIT", defaultMetricsRateLimit, defaultMetricsRateWindow)
	feedbackRateLimit, feedbackRateWindow = getEnvRateLimitOrDefault("FEEDBACK_RATE_LIMIT", defaultFeedbackRateLimit, defaultFeedbackRateWindow)
	logRateLimit, logRateWindow = getEnvRateLimitOrDefault("LOG_RATE_LIMIT", defaultLogRateLimit, defaultLogRateWindow)

	// Login lockout configuration
	maxLoginAttempts = getEnvIntOrDefault("MAX_LOGIN_ATTEMPTS", defaultMaxLoginAttempts)
	if maxLoginAttempts == 0 {
		maxLoginAttempts = defaultMaxLoginAttempts
	}
	loginLockDuration = getEnvDurationOrDefault("LOGIN_LOCK_DURATION", defaultLoginLockDuration)
	if loginLockDuration == 0 {
		loginLockDuration = defaultLoginLockDuration
	}

	// Chunked upload configuration
	chunkSize = getEnvSizeOrDefault("CHUNK_SIZE", defaultChunkSize)
	sessionExpiry = getEnvDurationOrDefault("UPLOAD_SESSION_EXPIRY", defaultSessionExpiry)

	// Database maintenance configuration
	dbMaintenanceInterval = getEnvDurationOrDefault("DB_MAINTENANCE_INTERVAL", defaultDBMaintenanceInterval)

	// Database connection pool configuration
	dbMaxOpenConns = getEnvIntOrDefault("DB_MAX_OPEN_CONNS", db.DefaultMaxOpenConns)
	dbMaxIdleConns = getEnvIntOrDefault("DB_MAX_IDLE_CONNS", db.DefaultMaxIdleConns)

	// Subtitle font configuration
	// For proper Indic script rendering (Hindi, Tamil, etc.), install fonts like:
	// - Noto Sans Devanagari (for Hindi, Marathi, Sanskrit)
	// - Noto Sans Tamil, Noto Sans Telugu, etc.
	// Set SUBTITLE_FONT to the font path or name that supports your target scripts.
	subtitleFont = getEnvOrDefault("SUBTITLE_FONT", "")

	// Metrics API key for /metrics endpoint
	metricsAPIKey = os.Getenv("METRICS_API_KEY")

	// Whisper transcription configuration
	whisperThreads = getEnvIntOrDefault("WHISPER_THREADS", defaultWhisperThreads)
	whisperTemperature = getEnvOrDefault("WHISPER_TEMPERATURE", "0.0")
	whisperTimeout = getEnvDurationOrDefault("WHISPER_TIMEOUT", defaultWhisperTimeout)

	// Cache HTTPS_ONLY for cookie and HSTS configuration
	auth.InitHTTPSOnly()
}
