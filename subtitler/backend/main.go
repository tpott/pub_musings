package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/trevor/subtitler/backend/align"
	"github.com/trevor/subtitler/backend/audio"
	"github.com/trevor/subtitler/backend/auth"
	"github.com/trevor/subtitler/backend/captcha"
	"github.com/trevor/subtitler/backend/crypto"
	"github.com/trevor/subtitler/backend/csrf"
	"github.com/trevor/subtitler/backend/db"
	"github.com/trevor/subtitler/backend/email"
	"github.com/trevor/subtitler/backend/errmsg"
	"github.com/trevor/subtitler/backend/logging"
	"github.com/trevor/subtitler/backend/metrics"
	"github.com/trevor/subtitler/backend/pathvalidator"
	"github.com/trevor/subtitler/backend/ratelimit"
	"github.com/trevor/subtitler/backend/script"
	"github.com/trevor/subtitler/backend/security"
	"github.com/trevor/subtitler/backend/totp"
	"github.com/trevor/subtitler/backend/validation"
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

	// Chunked upload defaults
	defaultChunkSize     = 50 << 20       // 50 MB
	defaultSessionExpiry = 24 * time.Hour // 24 hours

	// Database maintenance defaults
	defaultDBMaintenanceInterval = 24 * time.Hour // Run VACUUM and ANALYZE daily

	// Whisper transcription defaults
	defaultWhisperThreads = 4                // Number of threads for whisper-cli
	defaultWhisperTimeout = 30 * time.Minute // Timeout for whisper-server requests
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

	// Chunked upload configuration
	chunkSize     int64
	sessionExpiry time.Duration

	// Database maintenance configuration
	dbMaintenanceInterval time.Duration

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

// getEnvIntOrDefault parses an integer from environment variable.
// Returns default value if not set or invalid.
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
	return n
}

// getEnvSizeOrDefault parses a size from environment variable (e.g., "500M", "1G")
// Returns default value if not set or invalid
func getEnvSizeOrDefault(key string, defaultValue int64) int64 {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	// Parse size with optional suffix (M for MB, G for GB)
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
		logging.Warn("Invalid MAX_UPLOAD_SIZE, using default", "value", os.Getenv(key), "default_bytes", defaultValue)
		return defaultValue
	}

	return size * multiplier
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

	// Chunked upload configuration
	chunkSize = getEnvSizeOrDefault("CHUNK_SIZE", defaultChunkSize)
	sessionExpiry = getEnvDurationOrDefault("UPLOAD_SESSION_EXPIRY", defaultSessionExpiry)

	// Database maintenance configuration
	dbMaintenanceInterval = getEnvDurationOrDefault("DB_MAINTENANCE_INTERVAL", defaultDBMaintenanceInterval)

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
}

// WhisperSegment represents a transcribed segment with timing
type WhisperSegment struct {
	ID    int     `json:"id"`
	Start float64 `json:"start"` // start time in seconds
	End   float64 `json:"end"`   // end time in seconds
	Text  string  `json:"text"`
}

// WhisperResult represents the full transcription result
type WhisperResult struct {
	Language string           `json:"language"`
	Duration float64          `json:"duration"`
	Text     string           `json:"text"`
	Segments []WhisperSegment `json:"segments"`
}

// TranscriptionStatus tracks the state of a transcription job (API response format)
type TranscriptionStatus struct {
	Status   string         `json:"status"` // pending, processing, complete, error
	Message  string         `json:"message,omitempty"`
	Result   *WhisperResult `json:"result,omitempty"`
	Progress int            `json:"progress,omitempty"` // 0-100
}

// Global database connection
var database *db.DB

// Global multi-key encryptor for file encryption with key rotation support
var multiEnc *crypto.MultiKeyEncryptor

// encryptor returns the current encryptor for backward compatibility
// This is a convenience function that returns the encryptor for the current key version
func encryptor() *crypto.Encryptor {
	return multiEnc.GetEncryptor()
}

// Global rate limiters (initialized by initRateLimiters after config is loaded)
var authLimiter *ratelimit.Limiter
var passwordResetLimiter *ratelimit.Limiter
var uploadLimiter *ratelimit.Limiter
var transcribeLimiter *ratelimit.Limiter
var burnLimiter *ratelimit.Limiter
var scriptLimiter *ratelimit.Limiter
var chunkLimiter *ratelimit.Limiter
var downloadLimiter *ratelimit.Limiter
var metricsLimiter *ratelimit.Limiter
var userLimiter *ratelimit.UserLimiter

// initRateLimiters creates rate limiters based on configuration
// Must be called after initConfig()
func initRateLimiters() {
	authLimiter = ratelimit.New(authRateLimit, authRateWindow)
	passwordResetLimiter = ratelimit.New(passwordResetRateLimit, passwordResetWindow)
	uploadLimiter = ratelimit.New(uploadRateLimit, uploadRateWindow)
	transcribeLimiter = ratelimit.New(transcribeRateLimit, transcribeRateWindow)
	burnLimiter = ratelimit.New(burnRateLimit, burnRateWindow)
	scriptLimiter = ratelimit.New(scriptRateLimit, scriptRateWindow)
	chunkLimiter = ratelimit.New(chunkRateLimit, chunkRateWindow)
	downloadLimiter = ratelimit.New(downloadRateLimit, downloadRateWindow)
	metricsLimiter = ratelimit.New(metricsRateLimit, metricsRateWindow)
	userLimiter = ratelimit.NewUserLimiter(userRateLimit, userRateWindow)

	// Set up rate limit security event logging
	ratelimit.OnLimitExceeded = func(r *http.Request, ip string, endpoint string) {
		if endpoint == "" {
			endpoint = r.URL.Path
		}
		security.RateLimitExceeded(r.Context(), ip, endpoint, "ip")
	}
	ratelimit.OnUserLimitExceeded = func(r *http.Request, userID string, endpoint string) {
		if endpoint == "" {
			endpoint = r.URL.Path
		}
		ip := ratelimit.GetClientIP(r)
		security.RateLimitExceeded(r.Context(), ip, endpoint, "user:"+userID)
	}
}

// Global email service for transactional emails
var emailService email.EmailService

// Global CAPTCHA verifier for bot protection
var captchaVerifier captcha.Verifier

// Global audio extractor for video processing
var audioExtractor audio.Extractor

// Global path validator for file serving security
var pathValidator *pathvalidator.Validator

// getUserIDFromRequest extracts the user ID from the request if authenticated.
// Returns empty string if not authenticated or session is invalid.
// This is used for per-user rate limiting.
func getUserIDFromRequest(r *http.Request) string {
	token := auth.GetTokenFromRequest(r)
	if token == "" {
		return ""
	}
	user, _, err := auth.ValidateSession(database, token)
	if err != nil || user == nil {
		return ""
	}
	return user.ID
}

// generateRequestID creates a unique request ID for tracing
func generateRequestID() string {
	b := make([]byte, 8) // 16 hex chars
	if _, err := rand.Read(b); err != nil {
		// Fall back to timestamp-based ID if random fails
		return fmt.Sprintf("%016x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// requestIDMiddleware adds X-Request-ID header to all requests and responses
func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if request already has an ID (from proxy)
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}

		// Set the request ID in response header
		w.Header().Set("X-Request-ID", requestID)

		// Add request ID to context for downstream handlers
		ctx := logging.WithRequestID(r.Context(), requestID)
		r = r.WithContext(ctx)

		// Log the request with its ID
		logging.InfoContext(ctx, "Request received", "method", r.Method, "path", r.URL.Path)

		// Call the next handler
		next.ServeHTTP(w, r)
	})
}

// userRateLimitMiddleware applies per-user rate limiting for authenticated requests.
// It adds X-RateLimit-Limit and X-RateLimit-Remaining headers to responses.
// Anonymous requests are allowed through (they rely on IP-based rate limiting).
func userRateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := getUserIDFromRequest(r)

		// If not authenticated, skip per-user rate limiting
		if userID == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Add rate limit headers
		w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", userLimiter.GetLimit()))

		if !userLimiter.AllowUser(userID) {
			remaining := userLimiter.RemainingUser(userID)
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"Rate limit exceeded. Please try again later."}`))
			return
		}

		remaining := userLimiter.RemainingUser(userID)
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

		next.ServeHTTP(w, r)
	})
}

// securityHeadersMiddleware adds security headers to all responses
// These provide defense-in-depth against common web vulnerabilities
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Content-Security-Policy: Prevent XSS by restricting script/style sources
		// - 'self': Allow resources from same origin
		// - 'unsafe-inline': Required for Astro's inline scripts and styles
		// - data: scheme: Allow data URLs for inline images (like QR codes)
		// - blob: scheme: Allow blob URLs for video playback
		// Note: In production, Caddy can override with stricter CSP if needed
		csp := "default-src 'self'; " +
			"script-src 'self' 'unsafe-inline'; " +
			"style-src 'self' 'unsafe-inline'; " +
			"img-src 'self' data: blob:; " +
			"media-src 'self' blob:; " +
			"connect-src 'self'; " +
			"font-src 'self'; " +
			"object-src 'none'; " +
			"frame-ancestors 'none'; " +
			"base-uri 'self'; " +
			"form-action 'self'"
		w.Header().Set("Content-Security-Policy", csp)

		// X-Content-Type-Options: Prevent MIME type sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// X-Frame-Options: Prevent clickjacking (legacy, CSP frame-ancestors is preferred)
		w.Header().Set("X-Frame-Options", "DENY")

		// Referrer-Policy: Limit referrer information leakage
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// X-XSS-Protection: Enable browser XSS filter (legacy browsers)
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Strict-Transport-Security (HSTS): Force HTTPS in production
		// Only set when HTTPS_ONLY is enabled to avoid issues in development
		// max-age=31536000 (1 year) tells browsers to only use HTTPS
		// includeSubDomains applies to all subdomains
		if auth.IsHTTPSOnly() {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		// Permissions-Policy: Disable unnecessary browser features
		// - geolocation, microphone, camera: Not needed by this app
		// - payment, usb: Potential attack vectors
		// - interest-cohort: Opt out of FLoC tracking
		permPolicy := "geolocation=(), microphone=(), camera=(), payment=(), usb=(), interest-cohort=()"
		w.Header().Set("Permissions-Policy", permPolicy)

		next.ServeHTTP(w, r)
	})
}

func generateID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate ID: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// getWhisperModel returns the whisper model path from env or default
func getWhisperModel() string {
	model := os.Getenv("WHISPER_MODEL")
	if model == "" {
		// Default to medium model - adjust path as needed
		model = os.ExpandEnv("$HOME/Github/whisper.cpp/models/ggml-medium.bin")
	}
	return model
}

// getWhisperServerURL returns the whisper-server URL from env or default
func getWhisperServerURL() string {
	url := os.Getenv("WHISPER_SERVER_URL")
	if url == "" {
		// Default to local whisper-server (different port to avoid conflict with backend)
		url = "http://127.0.0.1:8765"
	}
	return url
}

// isWhisperServerEnabled returns true if WHISPER_SERVER_URL is set or USE_WHISPER_SERVER=true
func isWhisperServerEnabled() bool {
	// If WHISPER_SERVER_URL is explicitly set, use server mode
	if os.Getenv("WHISPER_SERVER_URL") != "" {
		return true
	}
	// Otherwise check USE_WHISPER_SERVER flag
	return os.Getenv("USE_WHISPER_SERVER") == "true"
}

// checkWhisperServerHealth checks if whisper-server is available (when configured)
// Returns (available bool, error string)
func checkWhisperServerHealth() (bool, string) {
	if !isWhisperServerEnabled() {
		// Not configured, so we consider it "available" (will use CLI mode)
		return true, ""
	}

	serverURL := getWhisperServerURL()
	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(serverURL + "/")
	if err != nil {
		return false, fmt.Sprintf("cannot reach whisper-server at %s: %v", serverURL, err)
	}
	defer resp.Body.Close()

	// Any response means the server is reachable
	return true, ""
}

// checkDiskSpace checks if there's enough free disk space for uploads
// Returns (ok bool, freeGB float64, error string)
func checkDiskSpace(path string, minFreeGB float64) (bool, float64, string) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return false, 0, fmt.Sprintf("cannot stat filesystem: %v", err)
	}

	// Calculate free space in GB
	freeBytes := stat.Bavail * uint64(stat.Bsize)
	freeGB := float64(freeBytes) / (1024 * 1024 * 1024)

	if freeGB < minFreeGB {
		return false, freeGB, fmt.Sprintf("low disk space: %.2f GB free (minimum: %.2f GB)", freeGB, minFreeGB)
	}

	return true, freeGB, ""
}

// HealthStatus represents the response from /api/health
type HealthStatus struct {
	Status           string   `json:"status"` // "ok" or "degraded"
	DBConnected      bool     `json:"db_connected"`
	WhisperAvailable bool     `json:"whisper_available"`
	DiskSpaceOK      bool     `json:"disk_space_ok"`
	DiskFreeGB       float64  `json:"disk_free_gb,omitempty"`
	Errors           []string `json:"errors,omitempty"`
}

// transcribeAudio runs whisper-cli on the audio file.
// language is an ISO 639-1 code (e.g., "en", "es") or "auto" for auto-detection.
func transcribeAudio(audioPath, outputPath, language string) (*WhisperResult, error) {
	model := getWhisperModel()

	// Check if model exists
	if _, err := os.Stat(model); os.IsNotExist(err) {
		return nil, fmt.Errorf("whisper model not found at %s - set WHISPER_MODEL env var", model)
	}

	// Run whisper-cli with JSON output
	cmd := exec.Command("whisper-cli",
		"-m", model,
		"-f", audioPath,
		"-oj",             // output JSON
		"-of", outputPath, // output file (without extension, whisper adds .json)
		"-t", strconv.Itoa(whisperThreads), // thread count (configurable via WHISPER_THREADS)
		"-l", language, // language code or "auto"
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("whisper error: %v, output: %s", err, string(output))
	}

	// Read the JSON output file
	jsonPath := outputPath + ".json"
	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read whisper output: %v", err)
	}

	// Parse JSON
	var result WhisperResult
	if err := json.Unmarshal(jsonData, &result); err != nil {
		return nil, fmt.Errorf("failed to parse whisper JSON: %v", err)
	}

	return &result, nil
}

// transcribeAudioServer sends audio to whisper-server HTTP API.
// language is an ISO 639-1 code (e.g., "en", "es") or "auto" for auto-detection.
func transcribeAudioServer(audioPath, language string) (*WhisperResult, error) {
	serverURL := getWhisperServerURL()

	// Open the audio file
	file, err := os.Open(audioPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open audio file: %v", err)
	}
	defer file.Close()

	// Create multipart form
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add the audio file
	part, err := writer.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %v", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("failed to copy file to form: %v", err)
	}

	// Add request parameters
	// Use verbose_json to get segments with timing
	writer.WriteField("response_format", "verbose_json")
	writer.WriteField("temperature", whisperTemperature) // configurable via WHISPER_TEMPERATURE
	writer.WriteField("language", language)

	writer.Close()

	// Retry configuration: 3 attempts with exponential backoff (1s, 2s, 4s)
	maxRetries := 3
	retryDelays := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}
	formDataContentType := writer.FormDataContentType()
	requestBody := body.Bytes()

	var resp *http.Response
	var respBody []byte
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Create HTTP request (need fresh request for each attempt)
		req, err := http.NewRequest("POST", serverURL+"/inference", bytes.NewReader(requestBody))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %v", err)
		}
		req.Header.Set("Content-Type", formDataContentType)

		// Send request (with long timeout for transcription, configurable via WHISPER_TIMEOUT)
		client := &http.Client{Timeout: whisperTimeout}
		resp, err = client.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxRetries-1 {
				logging.Warn("whisper-server request failed, retrying", "attempt", attempt+1, "max_retries", maxRetries, "error", err, "retry_delay", retryDelays[attempt])
				time.Sleep(retryDelays[attempt])
				continue
			}
			return nil, fmt.Errorf("whisper-server request failed after %d attempts: %v", maxRetries, err)
		}
		defer resp.Body.Close()

		// Read response
		respBody, err = io.ReadAll(resp.Body)
		if err != nil {
			lastErr = err
			if attempt < maxRetries-1 {
				logging.Warn("failed to read response, retrying", "attempt", attempt+1, "max_retries", maxRetries, "error", err, "retry_delay", retryDelays[attempt])
				time.Sleep(retryDelays[attempt])
				continue
			}
			return nil, fmt.Errorf("failed to read response after %d attempts: %v", maxRetries, err)
		}

		// Check for server errors (5xx) that warrant a retry
		if resp.StatusCode >= 500 && resp.StatusCode < 600 {
			lastErr = fmt.Errorf("server error (status %d): %s", resp.StatusCode, string(respBody))
			if attempt < maxRetries-1 {
				logging.Warn("whisper-server returned error, retrying", "status_code", resp.StatusCode, "attempt", attempt+1, "max_retries", maxRetries, "response", string(respBody), "retry_delay", retryDelays[attempt])
				time.Sleep(retryDelays[attempt])
				continue
			}
			return nil, fmt.Errorf("whisper-server error after %d attempts (status %d): %s", maxRetries, resp.StatusCode, string(respBody))
		}

		// Non-retryable error (4xx) or success (2xx)
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("whisper-server error (status %d): %s", resp.StatusCode, string(respBody))
		}

		// Success - break out of retry loop
		break
	}

	// If we got here from exhausting retries without success
	if resp == nil || (lastErr != nil && resp.StatusCode != http.StatusOK) {
		return nil, fmt.Errorf("whisper-server request failed after %d attempts: %v", maxRetries, lastErr)
	}

	// Parse verbose_json response
	// verbose_json format has: task, language, duration, text, segments[]
	var serverResp struct {
		Task     string  `json:"task"`
		Language string  `json:"language"`
		Duration float64 `json:"duration"`
		Text     string  `json:"text"`
		Segments []struct {
			ID    int     `json:"id"`
			Start float64 `json:"start"`
			End   float64 `json:"end"`
			Text  string  `json:"text"`
		} `json:"segments"`
	}
	if err := json.Unmarshal(respBody, &serverResp); err != nil {
		return nil, fmt.Errorf("failed to parse whisper-server response: %v (body: %s)", err, string(respBody))
	}

	// Convert to our WhisperResult format
	result := &WhisperResult{
		Language: serverResp.Language,
		Duration: serverResp.Duration,
		Text:     serverResp.Text,
		Segments: make([]WhisperSegment, len(serverResp.Segments)),
	}
	for i, seg := range serverResp.Segments {
		result.Segments[i] = WhisperSegment{
			ID:    seg.ID,
			Start: seg.Start,
			End:   seg.End,
			Text:  seg.Text,
		}
	}

	return result, nil
}

// transcribe sends audio for transcription, using server if enabled, otherwise CLI.
// language is an ISO 639-1 language code (e.g., "en", "es", "ja") or "auto" for auto-detection.
// If empty, defaults to "auto".
func transcribe(audioPath, outputPath, language string) (*WhisperResult, error) {
	if language == "" {
		language = "auto"
	}
	if isWhisperServerEnabled() {
		logging.Info("Using whisper-server", "url", getWhisperServerURL(), "language", language)
		return transcribeAudioServer(audioPath, language)
	}
	logging.Info("Using whisper-cli", "model", getWhisperModel(), "language", language)
	return transcribeAudio(audioPath, outputPath, language)
}

// formatSRTTimestamp formats seconds as SRT timestamp (HH:MM:SS,mmm)
func formatSRTTimestamp(seconds float64) string {
	hours := int(seconds) / 3600
	minutes := (int(seconds) % 3600) / 60
	secs := int(seconds) % 60
	millis := int(math.Round((seconds - math.Floor(seconds)) * 1000))
	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, secs, millis)
}

// generateSRT converts WhisperResult segments to SRT format
func generateSRT(result *WhisperResult) string {
	var sb strings.Builder
	for i, segment := range result.Segments {
		// SRT sequence numbers are 1-indexed
		sb.WriteString(fmt.Sprintf("%d\n", i+1))
		sb.WriteString(fmt.Sprintf("%s --> %s\n", formatSRTTimestamp(segment.Start), formatSRTTimestamp(segment.End)))
		sb.WriteString(strings.TrimSpace(segment.Text))
		sb.WriteString("\n\n")
	}
	return sb.String()
}

// formatVTTTimestamp formats seconds as WebVTT timestamp (HH:MM:SS.mmm)
func formatVTTTimestamp(seconds float64) string {
	hours := int(seconds) / 3600
	minutes := (int(seconds) % 3600) / 60
	secs := int(seconds) % 60
	millis := int(math.Round((seconds - math.Floor(seconds)) * 1000))
	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, secs, millis)
}

// generateVTT converts WhisperResult segments to WebVTT format
func generateVTT(result *WhisperResult) string {
	var sb strings.Builder
	sb.WriteString("WEBVTT\n\n")
	for i, segment := range result.Segments {
		// VTT cue identifiers are optional but helpful
		sb.WriteString(fmt.Sprintf("%d\n", i+1))
		sb.WriteString(fmt.Sprintf("%s --> %s\n", formatVTTTimestamp(segment.Start), formatVTTTimestamp(segment.End)))
		sb.WriteString(strings.TrimSpace(segment.Text))
		sb.WriteString("\n\n")
	}
	return sb.String()
}

// findVideoFile finds the video file for an upload ID
func findVideoFile(uploadID string) (string, error) {
	// First try to get from database
	if database != nil {
		video, err := database.GetVideo(uploadID)
		if err != nil {
			return "", err
		}
		if video != nil {
			return video.FilePath, nil
		}
	}

	// Fallback to glob search for backwards compatibility
	matches, err := filepath.Glob(filepath.Join(uploadDir, uploadID+".*"))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("video not found for upload ID: %s", uploadID)
	}
	return matches[0], nil
}

// getVideoForDecryption retrieves video with key version for decryption operations
func getVideoForDecryption(uploadID string) (*db.Video, error) {
	video, err := database.GetVideo(uploadID)
	if err != nil {
		return nil, err
	}
	if video == nil {
		return nil, fmt.Errorf("video not found for upload ID: %s", uploadID)
	}
	return video, nil
}

// generateETag creates an ETag from input data using SHA256
func generateETag(data string) string {
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("\"%s\"", hex.EncodeToString(hash[:8])) // Use first 8 bytes (16 hex chars)
}

// handleConditionalRequest checks If-None-Match header and returns true if 304 should be sent
func handleConditionalRequest(w http.ResponseWriter, r *http.Request, etag string) bool {
	if match := r.Header.Get("If-None-Match"); match != "" {
		if match == etag || match == "*" {
			w.Header().Set("ETag", etag)
			w.WriteHeader(http.StatusNotModified)
			return true
		}
	}
	return false
}

// setCacheHeaders sets ETag and Cache-Control headers for responses
// maxAge is in seconds, 0 means no cache (must-revalidate)
func setCacheHeaders(w http.ResponseWriter, etag string, maxAge int) {
	w.Header().Set("ETag", etag)
	if maxAge > 0 {
		w.Header().Set("Cache-Control", fmt.Sprintf("private, max-age=%d", maxAge))
	} else {
		w.Header().Set("Cache-Control", "private, no-cache, must-revalidate")
	}
}

// dbTranscriptionToStatus converts a database transcription to API status format
func dbTranscriptionToStatus(t *db.Transcription) *TranscriptionStatus {
	if t == nil {
		return nil
	}

	status := &TranscriptionStatus{
		Status:   t.Status,
		Message:  t.Message,
		Progress: t.Progress,
	}

	if t.Status == "complete" {
		segments, err := t.GetSegments()
		if err != nil {
			logging.Error("Failed to get transcription segments", "transcription_id", t.ID, "video_id", t.VideoID, "error", err)
			// Return empty segments array instead of failing completely
			segments = []db.Segment{}
		}
		whisperSegments := make([]WhisperSegment, len(segments))
		for i, s := range segments {
			whisperSegments[i] = WhisperSegment{
				ID:    s.ID,
				Start: s.Start,
				End:   s.End,
				Text:  s.Text,
			}
		}
		status.Result = &WhisperResult{
			Language: t.Language,
			Duration: t.Duration,
			Text:     t.FullText,
			Segments: whisperSegments,
		}
	}

	return status
}

func main() {
	// Initialize structured logging first
	logging.Init(os.Getenv("LOG_LEVEL"))

	// Initialize configuration from environment variables
	initConfig()
	initRateLimiters()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Log configuration
	logging.Info("Configuration loaded",
		"max_upload_size", maxUploadSize,
		"upload_dir", uploadDir,
		"db_path", dbPath,
		"key_path", keyPath)
	logging.Info("Rate limits configured",
		"auth", fmt.Sprintf("%d/%v", authRateLimit, authRateWindow),
		"upload", fmt.Sprintf("%d/%v", uploadRateLimit, uploadRateWindow),
		"transcribe", fmt.Sprintf("%d/%v", transcribeRateLimit, transcribeRateWindow),
		"burn", fmt.Sprintf("%d/%v", burnRateLimit, burnRateWindow),
		"script", fmt.Sprintf("%d/%v", scriptRateLimit, scriptRateWindow))

	// Ensure upload directory exists
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		logging.Fatal("Failed to create upload directory", "error", err)
	}

	// Initialize path validator for secure file serving
	// Validates that file paths stay within the upload directory
	var err error
	pathValidator, err = pathvalidator.New(uploadDir)
	if err != nil {
		logging.Fatal("Failed to initialize path validator", "error", err)
	}
	logging.Info("Path validator initialized", "allowed_dirs", uploadDir)

	// Initialize database
	database, err = db.Open(dbPath)
	if err != nil {
		logging.Fatal("Failed to open database", "error", err)
	}
	defer database.Close()
	logging.Info("Database initialized", "path", dbPath)

	// Check for initial admin email configuration
	// If set, promotes the user with this email to admin role
	initialAdminEmail := os.Getenv("INITIAL_ADMIN_EMAIL")
	if initialAdminEmail != "" {
		if err := database.PromoteToAdmin(initialAdminEmail); err != nil {
			// Log as info since user might not exist yet
			logging.Info("Could not promote initial admin (user may not exist yet)", "email", initialAdminEmail, "error", err)
		} else {
			logging.Info("Promoted user to admin", "email", initialAdminEmail)
		}
	}

	// Initialize multi-key encryptor for file encryption at rest with key rotation support
	// Check if KEYS_DIR is set (new multi-key mode), otherwise use legacy keyPath
	keysDir := os.Getenv("KEYS_DIR")
	if keysDir == "" {
		// Default to data/keys subdirectory, but check for legacy single-key setup
		keysDir = filepath.Join(filepath.Dir(keyPath), "keys")
	}
	multiEnc = crypto.NewMultiKeyEncryptor(keysDir)
	if err := multiEnc.LoadOrInitialize(); err != nil {
		logging.Fatal("Failed to initialize encryption", "error", err)
	}
	logging.Info("Encryption initialized",
		"keys_dir", keysDir,
		"current_version", multiEnc.GetCurrentVersion(),
		"available_versions", multiEnc.GetVersions(),
		"public_key", encryptor().PublicKey())

	// Initialize email service
	emailService = email.NewResendService()
	if emailService.IsEnabled() {
		logging.Info("Email service enabled")
	} else {
		logging.Info("Email service disabled (no RESEND_API_KEY set)")
	}

	// Initialize CAPTCHA verifier
	captchaVerifier = captcha.New(captcha.Config{
		SiteKey:   os.Getenv("CAPTCHA_SITE_KEY"),
		SecretKey: os.Getenv("CAPTCHA_SECRET_KEY"),
	})
	if captchaVerifier.IsEnabled() {
		logging.Info("CAPTCHA protection enabled")
	} else {
		logging.Info("CAPTCHA protection disabled (no CAPTCHA_SECRET_KEY set)")
	}

	// Initialize audio extractor and check ffmpeg availability
	audioExtractor = audio.NewFFmpegExtractor()
	if err := audio.CheckFFmpegAvailable(); err != nil {
		logging.Warn("ffmpeg not available", "error", err)
		logging.Warn("Video transcription and subtitle burning will fail until ffmpeg is installed")
	} else {
		logging.Info("ffmpeg available for video processing")
	}

	mux := http.NewServeMux()

	// Health check endpoint
	// Unauthenticated: returns only {"status": "ok"} or {"status": "degraded"}
	// Authenticated: returns full response with all dependency details
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		status := HealthStatus{
			Status:           "ok",
			DBConnected:      true,
			WhisperAvailable: true,
			DiskSpaceOK:      true,
			Errors:           []string{},
		}

		// Check database connectivity
		if err := database.Ping(); err != nil {
			status.DBConnected = false
			status.Errors = append(status.Errors, fmt.Sprintf("database: %v", err))
		}

		// Check whisper-server availability (if configured)
		whisperOK, whisperErr := checkWhisperServerHealth()
		if !whisperOK {
			status.WhisperAvailable = false
			status.Errors = append(status.Errors, whisperErr)
		}

		// Check disk space (minimum 1GB free for uploads)
		diskOK, freeGB, diskErr := checkDiskSpace(".", 1.0)
		status.DiskFreeGB = freeGB
		if !diskOK {
			status.DiskSpaceOK = false
			status.Errors = append(status.Errors, diskErr)
		}

		// Determine overall status
		if !status.DBConnected || !status.WhisperAvailable || !status.DiskSpaceOK {
			status.Status = "degraded"
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		// Check if user is authenticated
		token := auth.GetTokenFromRequest(r)
		user, _, _ := auth.ValidateSession(database, token)

		if user != nil {
			// Authenticated: return full response
			json.NewEncoder(w).Encode(status)
		} else {
			// Unauthenticated: return minimal response
			json.NewEncoder(w).Encode(map[string]string{
				"status": status.Status,
			})
		}
	})

	// Prometheus metrics endpoint
	// Protected by API key (via METRICS_API_KEY env var) or admin authentication
	// Rate limited to prevent reconnaissance attacks
	mux.HandleFunc("GET /metrics", metricsLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		// Check API key first
		apiKey := r.Header.Get("X-Metrics-API-Key")
		if apiKey == "" {
			apiKey = r.URL.Query().Get("api_key")
		}

		if metricsAPIKey != "" && apiKey == metricsAPIKey {
			// Valid API key, serve metrics
			metrics.Handler().ServeHTTP(w, r)
			return
		}

		// Fall back to checking for admin user
		token := auth.GetTokenFromRequest(r)
		if token == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Authentication required",
			})
			return
		}

		user, _, err := auth.ValidateSession(database, token)
		if err != nil || user == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid session",
			})
			return
		}

		// Check if user has admin role
		if !user.IsAdmin() {
			security.AccessDeniedNotAdmin(r.Context(), ratelimit.GetClientIP(r), user.ID, "/metrics")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Admin access required",
			})
			return
		}

		metrics.Handler().ServeHTTP(w, r)
	}))

	// Frontend log forwarding endpoint (for dev mode debugging)
	mux.HandleFunc("POST /api/log", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Parse request body
		var req struct {
			Level   string        `json:"level"`   // log, warn, error, info, debug
			Message string        `json:"message"` // formatted message string
			Args    []interface{} `json:"args"`    // additional arguments (optional)
			URL     string        `json:"url"`     // page URL where log originated
			Line    int           `json:"line"`    // line number (optional)
			Column  int           `json:"column"`  // column number (optional)
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid request body",
			})
			return
		}

		// Validate level
		validLevels := map[string]bool{"log": true, "warn": true, "error": true, "info": true, "debug": true}
		if !validLevels[req.Level] {
			req.Level = "log"
		}

		// Log frontend message
		attrs := []any{"level", req.Level, "message", req.Message}
		if req.URL != "" {
			attrs = append(attrs, "url", req.URL)
			if req.Line > 0 {
				attrs = append(attrs, "line", req.Line)
				if req.Column > 0 {
					attrs = append(attrs, "column", req.Column)
				}
			}
		}
		logging.DebugContext(r.Context(), "Frontend log", attrs...)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
		})
	})

	// CAPTCHA config endpoint (returns site key if CAPTCHA is enabled)
	mux.HandleFunc("GET /api/captcha/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"enabled":  captchaVerifier.IsEnabled(),
			"site_key": os.Getenv("CAPTCHA_SITE_KEY"),
		})
	})

	// Auth: Register new user (rate limited)
	mux.HandleFunc("POST /api/auth/register", authLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Parse request body
		var req struct {
			Email        string `json:"email"`
			Password     string `json:"password"`
			CaptchaToken string `json:"captcha_token,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid request body",
			})
			return
		}

		// Verify CAPTCHA if enabled
		if captchaVerifier.IsEnabled() {
			clientIP := ratelimit.GetClientIP(r)
			if err := captchaVerifier.Verify(r.Context(), req.CaptchaToken, clientIP); err != nil {
				logging.WarnContext(r.Context(), "CAPTCHA verification failed", "error", err, "ip", clientIP)
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "CAPTCHA verification failed. Please try again.",
				})
				return
			}
		}

		// Normalize email
		req.Email = strings.TrimSpace(strings.ToLower(req.Email))

		// Validate email
		if err := auth.ValidateEmail(req.Email); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": err.Error(),
			})
			return
		}

		// Validate password
		if err := auth.ValidatePassword(req.Password); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": err.Error(),
			})
			return
		}

		// Check if email already exists
		existingUser, err := database.GetUserByEmail(req.Email)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error checking existing user", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Registration failed",
			})
			return
		}
		if existingUser != nil {
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Email already registered",
			})
			return
		}

		// Hash password
		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error hashing password", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Registration failed",
			})
			return
		}

		// Generate user ID
		userID, err := auth.GenerateID()
		if err != nil {
			logging.ErrorContext(r.Context(), "Error generating user ID", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Registration failed",
			})
			return
		}

		// Create user (email_verified defaults to false)
		user := &db.User{
			ID:            userID,
			Email:         req.Email,
			PasswordHash:  hash,
			EmailVerified: false,
			CreatedAt:     time.Now(),
		}
		if err := database.CreateUser(user); err != nil {
			logging.ErrorContext(r.Context(), "Error creating user", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Registration failed",
			})
			return
		}

		// Generate email verification token (32 bytes = 256 bits entropy)
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			logging.ErrorContext(r.Context(), "Error generating verification token", "error", err)
			// User created but verification email failed - return success with message
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"message":            "Account created. Please check your email to verify your account.",
				"email_verification": true,
				"user": map[string]interface{}{
					"id":             user.ID,
					"email":          user.Email,
					"created_at":     user.CreatedAt,
					"email_verified": user.EmailVerified,
				},
			})
			return
		}
		token := hex.EncodeToString(tokenBytes)
		tokenHash := email.HashToken(token)

		// Create verification token with 24-hour expiry
		expiresAt := time.Now().Add(24 * time.Hour)
		_, err = database.CreateEmailVerificationToken(user.ID, tokenHash, expiresAt)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error creating verification token", "error", err)
		}

		// Send verification email
		if err := emailService.SendEmailVerification(r.Context(), user.Email, token); err != nil {
			logging.ErrorContext(r.Context(), "Error sending verification email", "error", err)
		}

		clientIP := ratelimit.GetClientIP(r)
		security.Registration(r.Context(), clientIP, user.ID, user.Email)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":            "Account created. Please check your email to verify your account.",
			"email_verification": true,
			"user": map[string]interface{}{
				"id":             user.ID,
				"email":          user.Email,
				"created_at":     user.CreatedAt,
				"email_verified": user.EmailVerified,
			},
		})
	}))

	// Auth: Login (rate limited)
	// Email rate limiting: 5 failed attempts = 15 minute lockout
	const maxLoginAttempts = 5
	const loginLockDuration = 15 * time.Minute

	mux.HandleFunc("POST /api/auth/login", authLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Parse request body
		var req struct {
			Email        string `json:"email"`
			Password     string `json:"password"`
			TOTPCode     string `json:"totp_code,omitempty"`     // Required if 2FA is enabled
			CaptchaToken string `json:"captcha_token,omitempty"` // Required if CAPTCHA is enabled
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid request body",
			})
			return
		}

		// Normalize email
		req.Email = strings.TrimSpace(strings.ToLower(req.Email))
		clientIP := ratelimit.GetClientIP(r)

		// Verify CAPTCHA if enabled (only for initial login attempt, not TOTP retry)
		// Skip CAPTCHA for TOTP code submission (user already passed CAPTCHA on initial login)
		if captchaVerifier.IsEnabled() && req.TOTPCode == "" {
			if err := captchaVerifier.Verify(r.Context(), req.CaptchaToken, clientIP); err != nil {
				security.LoginFailedCaptcha(r.Context(), clientIP, err.Error())
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "CAPTCHA verification failed. Please try again.",
				})
				return
			}
		}

		// Check if email is locked due to too many failed attempts
		locked, unlockTime, err := database.IsEmailLocked(req.Email, maxLoginAttempts, loginLockDuration)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error checking email lock", "email", req.Email, "error", err)
		}
		if locked {
			remainingMins := int(time.Until(unlockTime).Minutes()) + 1
			security.LoginFailedLocked(r.Context(), clientIP, req.Email, remainingMins)
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":           "Too many failed login attempts. Please try again later.",
				"retry_after_min": remainingMins,
			})
			return
		}

		// Helper to record failed attempt
		recordFailure := func() {
			if err := database.RecordLoginAttempt(req.Email, clientIP, false); err != nil {
				logging.ErrorContext(r.Context(), "Error recording login attempt", "email", req.Email, "error", err)
			}
		}

		// Get user by email
		user, err := database.GetUserByEmail(req.Email)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting user", "email", req.Email, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Login failed",
			})
			return
		}
		if user == nil {
			recordFailure()
			security.LoginFailedUserNotFound(r.Context(), clientIP, req.Email)
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid email or password",
			})
			return
		}

		// Check password
		if !auth.CheckPassword(req.Password, user.PasswordHash) {
			recordFailure()
			// Get current attempt count for logging (includes the one we just recorded)
			attempts, _ := database.GetRecentFailedLoginAttempts(req.Email, time.Now().Add(-loginLockDuration))
			security.LoginFailedPassword(r.Context(), clientIP, req.Email, attempts)
			// Check if this attempt caused a lockout
			if attempts >= maxLoginAttempts {
				security.AccountLocked(r.Context(), clientIP, req.Email, attempts)
			}
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid email or password",
			})
			return
		}

		// Check if email is verified
		if !user.EmailVerified {
			security.LoginFailedUnverified(r.Context(), clientIP, req.Email)
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":                   "Please verify your email address before logging in",
				"email_verification":      true,
				"email_not_verified":      true,
				"can_resend_verification": true,
			})
			return
		}

		// Check 2FA if enabled
		if user.TOTPEnabled {
			if req.TOTPCode == "" {
				// Don't record as failed attempt - just needs 2FA code
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":         "2FA code required",
					"totp_required": true,
				})
				return
			}

			// Validate the TOTP code
			if user.TOTPSecret == nil || !totp.Validate(*user.TOTPSecret, req.TOTPCode) {
				recordFailure()
				security.TwoFAVerifyFailed(r.Context(), clientIP, user.ID, user.Email)
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "Invalid 2FA code",
				})
				return
			}
			security.TwoFAVerifySuccess(r.Context(), clientIP, user.ID, user.Email)
		}

		// Login successful - clear failed attempts for this email
		if err := database.ClearLoginAttempts(req.Email); err != nil {
			logging.ErrorContext(r.Context(), "Error clearing login attempts", "email", req.Email, "error", err)
		}

		// Create session with IP and user agent
		userAgent := r.Header.Get("User-Agent")
		session, err := auth.CreateSession(database, user.ID, clientIP, userAgent)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error creating session", "user_id", user.ID, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Login failed",
			})
			return
		}

		// Set session cookie
		auth.SetSessionCookie(w, session.Token, session.ExpiresAt)

		security.LoginSuccess(r.Context(), clientIP, user.ID, user.Email)
		security.SessionCreated(r.Context(), clientIP, user.ID, session.ID)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"user": map[string]interface{}{
				"id":           user.ID,
				"email":        user.Email,
				"created_at":   user.CreatedAt,
				"totp_enabled": user.TOTPEnabled,
			},
			"token": session.Token,
		})
	}))

	// Auth: Logout
	mux.HandleFunc("POST /api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		clientIP := ratelimit.GetClientIP(r)

		token := auth.GetTokenFromRequest(r)
		var userID string
		if token != "" {
			// Get user ID before deleting session for audit logging
			if user, _, err := auth.ValidateSession(database, token); err == nil && user != nil {
				userID = user.ID
			}
			if err := database.DeleteSession(token); err != nil {
				logging.ErrorContext(r.Context(), "Error deleting session", "error", err)
			}
		}

		if userID != "" {
			security.Logout(r.Context(), clientIP, userID)
		}
		auth.ClearSessionCookie(w)

		json.NewEncoder(w).Encode(map[string]string{
			"message": "Logged out successfully",
		})
	})

	// Auth: Get current user
	mux.HandleFunc("GET /api/auth/me", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		token := auth.GetTokenFromRequest(r)
		user, _, err := auth.ValidateSession(database, token)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error validating session", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to get user",
			})
			return
		}

		if user == nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Not authenticated",
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"user": map[string]interface{}{
				"id":             user.ID,
				"email":          user.Email,
				"created_at":     user.CreatedAt,
				"totp_enabled":   user.TOTPEnabled,
				"email_verified": user.EmailVerified,
			},
		})
	})

	// Auth: Get CSRF token for current session
	mux.HandleFunc("GET /api/auth/csrf", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		sessionToken := auth.GetTokenFromRequest(r)
		if sessionToken == "" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Not authenticated",
			})
			return
		}

		// Validate the session exists
		user, _, err := auth.ValidateSession(database, sessionToken)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error validating session", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to validate session",
			})
			return
		}

		if user == nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Not authenticated",
			})
			return
		}

		// Generate CSRF token from session token
		csrfToken := csrf.GenerateToken(sessionToken)

		json.NewEncoder(w).Encode(map[string]string{
			"csrf_token": csrfToken,
		})
	})

	// Auth: Get all sessions for current user
	mux.HandleFunc("GET /api/auth/sessions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		token := auth.GetTokenFromRequest(r)
		user, currentSession, err := auth.ValidateSession(database, token)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error validating session", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to validate session",
			})
			return
		}

		if user == nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Authentication required",
			})
			return
		}

		sessions, err := database.GetSessionsByUserID(user.ID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting sessions", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to get sessions",
			})
			return
		}

		// Format sessions for API response, marking current session
		var responseSessions []map[string]interface{}
		for _, s := range sessions {
			sessionData := map[string]interface{}{
				"id":         s.ID,
				"ip_address": s.IPAddress,
				"user_agent": s.UserAgent,
				"created_at": s.CreatedAt,
				"expires_at": s.ExpiresAt,
				"is_current": s.ID == currentSession.ID,
			}
			responseSessions = append(responseSessions, sessionData)
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"sessions": responseSessions,
		})
	})

	// Auth: Revoke a specific session
	mux.HandleFunc("DELETE /api/auth/sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		token := auth.GetTokenFromRequest(r)
		user, currentSession, err := auth.ValidateSession(database, token)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error validating session", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to validate session",
			})
			return
		}

		if user == nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Authentication required",
			})
			return
		}

		sessionID := r.PathValue("id")
		if sessionID == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Session ID required",
			})
			return
		}

		// Prevent deleting current session through this endpoint
		if sessionID == currentSession.ID {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Cannot revoke current session. Use logout instead.",
			})
			return
		}

		err = database.DeleteSessionByID(sessionID, user.ID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error deleting session", "error", err)
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Session not found",
			})
			return
		}

		clientIP := ratelimit.GetClientIP(r)
		security.SessionRevoked(r.Context(), clientIP, user.ID, sessionID, false)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "success",
			"message": "Session revoked",
		})
	})

	// 2FA: Start TOTP setup - generates a new secret (rate limited)
	mux.HandleFunc("POST /api/auth/totp/setup", authLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Require authentication
		token := auth.GetTokenFromRequest(r)
		user, _, err := auth.ValidateSession(database, token)
		if err != nil || user == nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Authentication required",
			})
			return
		}

		// Check if 2FA is already enabled
		if user.TOTPEnabled {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "2FA is already enabled. Disable it first to set up a new authenticator.",
			})
			return
		}

		// Generate a new TOTP secret
		secret, err := totp.GenerateSecret()
		if err != nil {
			logging.ErrorContext(r.Context(), "Error generating TOTP secret", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to generate secret",
			})
			return
		}

		// Save the secret to the database (not yet enabled)
		if err := database.SetTOTPSecret(user.ID, secret); err != nil {
			logging.ErrorContext(r.Context(), "Error saving TOTP secret", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to save secret",
			})
			return
		}

		// Generate the provisioning URI for QR code
		issuer := "Subtitler"
		uri := totp.GenerateProvisioningURI(secret, user.Email, issuer)

		// Generate QR code as base64 data URL
		qrCode, err := totp.GenerateQRCode(uri)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error generating QR code", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to generate QR code",
			})
			return
		}

		logging.InfoContext(r.Context(), "TOTP setup initiated", "email", user.Email)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"secret":         secret,
			"secret_display": totp.FormatSecretForDisplay(secret),
			"uri":            uri,
			"issuer":         issuer,
			"qr_code":        qrCode,
		})
	}))

	// 2FA: Verify TOTP code and enable 2FA (rate limited)
	mux.HandleFunc("POST /api/auth/totp/verify", authLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Require authentication
		token := auth.GetTokenFromRequest(r)
		user, _, err := auth.ValidateSession(database, token)
		if err != nil || user == nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Authentication required",
			})
			return
		}

		// Check if 2FA is already enabled
		if user.TOTPEnabled {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "2FA is already enabled",
			})
			return
		}

		// Check if a secret has been set up
		if user.TOTPSecret == nil || *user.TOTPSecret == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "No TOTP secret found. Please start setup first.",
			})
			return
		}

		// Parse request body
		var req struct {
			Code string `json:"code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid request body",
			})
			return
		}

		// Validate TOTP code length
		if err := validation.ValidateTOTPCode(req.Code); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": err.Error(),
			})
			return
		}

		// Validate the code
		clientIP := ratelimit.GetClientIP(r)
		if !totp.Validate(*user.TOTPSecret, req.Code) {
			security.TwoFAVerifyFailed(r.Context(), clientIP, user.ID, user.Email)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid code. Please try again.",
			})
			return
		}

		// Generate recovery codes
		recoveryCodes, err := totp.GenerateRecoveryCodes(totp.NumCodes)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error generating recovery codes", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to generate recovery codes",
			})
			return
		}

		// Hash recovery codes
		codeHashes := make([]string, len(recoveryCodes))
		for i, code := range recoveryCodes {
			hash, err := totp.HashCode(code)
			if err != nil {
				logging.ErrorContext(r.Context(), "Error hashing recovery code", "error", err)
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "Failed to generate recovery codes",
				})
				return
			}
			codeHashes[i] = hash
		}

		// Enable 2FA and save recovery codes in a single transaction
		if err := database.EnableTOTPWithRecoveryCodes(user.ID, codeHashes); err != nil {
			logging.ErrorContext(r.Context(), "Error enabling TOTP with recovery codes", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to enable 2FA",
			})
			return
		}

		security.TwoFAEnabled(r.Context(), clientIP, user.ID, user.Email)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":        "2FA has been enabled successfully",
			"totp_enabled":   true,
			"recovery_codes": recoveryCodes,
		})
	}))

	// 2FA: Disable TOTP (rate limited)
	mux.HandleFunc("POST /api/auth/totp/disable", authLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Require authentication
		token := auth.GetTokenFromRequest(r)
		user, _, err := auth.ValidateSession(database, token)
		if err != nil || user == nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Authentication required",
			})
			return
		}

		// Check if 2FA is enabled
		if !user.TOTPEnabled {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "2FA is not enabled",
			})
			return
		}

		// Parse request body - require current TOTP code and password for security
		var req struct {
			Code     string `json:"code"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid request body",
			})
			return
		}

		// Validate TOTP code length
		if err := validation.ValidateTOTPCode(req.Code); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": err.Error(),
			})
			return
		}

		// Verify password
		if !auth.CheckPassword(req.Password, user.PasswordHash) {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid password",
			})
			return
		}

		// Validate the TOTP code
		clientIP := ratelimit.GetClientIP(r)
		if user.TOTPSecret == nil || !totp.Validate(*user.TOTPSecret, req.Code) {
			security.TwoFAVerifyFailed(r.Context(), clientIP, user.ID, user.Email)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid 2FA code",
			})
			return
		}

		// Disable 2FA
		if err := database.DisableTOTP(user.ID); err != nil {
			logging.ErrorContext(r.Context(), "Error disabling TOTP", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to disable 2FA",
			})
			return
		}

		// Delete recovery codes
		if err := database.DeleteRecoveryCodes(user.ID); err != nil {
			logging.ErrorContext(r.Context(), "Error deleting recovery codes", "error", err)
			// Continue - 2FA is disabled even if codes couldn't be deleted
		}

		security.TwoFADisabled(r.Context(), clientIP, user.ID, user.Email, false)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":      "2FA has been disabled successfully",
			"totp_enabled": false,
		})
	}))

	// 2FA: Recover account using recovery code (rate limited)
	mux.HandleFunc("POST /api/auth/totp/recover", authLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Parse request body
		var req struct {
			Email        string `json:"email"`
			Password     string `json:"password"`
			RecoveryCode string `json:"recovery_code"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid request body",
			})
			return
		}

		// Validate required fields
		if req.Email == "" || req.Password == "" || req.RecoveryCode == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Email, password, and recovery code are required",
			})
			return
		}

		// Validate email length
		if err := validation.ValidateEmail(req.Email); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": err.Error(),
			})
			return
		}

		// Validate recovery code length
		if err := validation.ValidateRecoveryCode(req.RecoveryCode); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": err.Error(),
			})
			return
		}

		// Get user by email
		user, err := database.GetUserByEmail(req.Email)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting user", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Server error",
			})
			return
		}
		if user == nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid email or password",
			})
			return
		}

		// Verify password
		if !auth.CheckPassword(req.Password, user.PasswordHash) {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid email or password",
			})
			return
		}

		// Check if 2FA is enabled
		if !user.TOTPEnabled {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "2FA is not enabled for this account",
			})
			return
		}

		// Get unused recovery codes
		codes, err := database.GetUnusedRecoveryCodes(user.ID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting recovery codes", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Server error",
			})
			return
		}

		// Check each code until we find a match
		var matchedCodeID string
		normalizedInput := totp.NormalizeCode(req.RecoveryCode)
		for _, code := range codes {
			if totp.CheckCode(normalizedInput, code.CodeHash) {
				matchedCodeID = code.ID
				break
			}
		}

		clientIP := ratelimit.GetClientIP(r)
		if matchedCodeID == "" {
			security.RecoveryCodeFailed(r.Context(), clientIP, req.Email)
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid recovery code",
			})
			return
		}

		// Mark the code as used
		success, err := database.UseRecoveryCode(matchedCodeID)
		if err != nil || !success {
			logging.ErrorContext(r.Context(), "Error using recovery code", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to use recovery code",
			})
			return
		}

		// Disable 2FA, delete recovery codes, and clear sessions in a single transaction
		if err := database.DisableTOTPAndClearSessions(user.ID); err != nil {
			logging.ErrorContext(r.Context(), "Error disabling TOTP and clearing sessions", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to disable 2FA",
			})
			return
		}

		// Create a new session with IP and user agent
		userAgent := r.Header.Get("User-Agent")
		session, err := auth.CreateSession(database, user.ID, clientIP, userAgent)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error creating session", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to create session",
			})
			return
		}

		// Set session cookie
		auth.SetSessionCookie(w, session.Token, session.ExpiresAt)

		security.RecoveryCodeUsed(r.Context(), clientIP, user.ID, user.Email)
		security.TwoFADisabled(r.Context(), clientIP, user.ID, user.Email, true)
		security.SessionCreated(r.Context(), clientIP, user.ID, session.ID)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":      "2FA has been disabled. Please set up 2FA again if you want to re-enable it.",
			"token":        session.Token,
			"totp_enabled": false,
		})
	}))

	// 2FA: Regenerate recovery codes (rate limited)
	mux.HandleFunc("POST /api/auth/totp/codes", authLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Check if user is authenticated
		token := auth.GetTokenFromRequest(r)
		user, _, err := auth.ValidateSession(database, token)
		if err != nil || user == nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Authentication required",
			})
			return
		}

		// Check if 2FA is enabled
		if !user.TOTPEnabled {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "2FA is not enabled",
			})
			return
		}

		// Parse request body - require password and TOTP code for security
		var req struct {
			Code     string `json:"code"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid request body",
			})
			return
		}

		// Validate TOTP code length
		if err := validation.ValidateTOTPCode(req.Code); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": err.Error(),
			})
			return
		}

		// Verify password
		if !auth.CheckPassword(req.Password, user.PasswordHash) {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid password",
			})
			return
		}

		// Validate the TOTP code
		if user.TOTPSecret == nil || !totp.Validate(*user.TOTPSecret, req.Code) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid 2FA code",
			})
			return
		}

		// Generate new recovery codes
		recoveryCodes, err := totp.GenerateRecoveryCodes(totp.NumCodes)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error generating recovery codes", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to generate recovery codes",
			})
			return
		}

		// Hash and store recovery codes (this deletes old codes first)
		codeHashes := make([]string, len(recoveryCodes))
		for i, code := range recoveryCodes {
			hash, err := totp.HashCode(code)
			if err != nil {
				logging.ErrorContext(r.Context(), "Error hashing recovery code", "error", err)
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "Failed to generate recovery codes",
				})
				return
			}
			codeHashes[i] = hash
		}

		if err := database.SaveRecoveryCodes(user.ID, codeHashes); err != nil {
			logging.ErrorContext(r.Context(), "Error saving recovery codes", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to save recovery codes",
			})
			return
		}

		logging.InfoContext(r.Context(), "Regenerated recovery codes", "email", user.Email)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":        "Recovery codes regenerated successfully",
			"recovery_codes": recoveryCodes,
		})
	}))

	// Auth: Forgot password - initiates password reset flow (stricter rate limiting)
	mux.HandleFunc("POST /api/auth/forgot-password", passwordResetLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var req struct {
			Email string `json:"email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid request body",
			})
			return
		}

		// Normalize email
		req.Email = strings.TrimSpace(strings.ToLower(req.Email))

		// Validate email format
		if err := auth.ValidateEmail(req.Email); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": err.Error(),
			})
			return
		}

		// Always return success to prevent email enumeration
		// Do the actual work in background-ish but keep same timing
		defer func() {
			json.NewEncoder(w).Encode(map[string]string{
				"message": "If an account exists with that email, a password reset link has been sent.",
			})
		}()

		// Look up user (don't reveal if exists)
		clientIP := ratelimit.GetClientIP(r)
		user, err := database.GetUserByEmail(req.Email)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error looking up user for password reset", "error", err)
			return
		}
		if user == nil {
			// User doesn't exist - log for security monitoring, return success anyway
			security.PasswordResetRequested(r.Context(), clientIP, req.Email, false)
			return
		}

		// Generate reset token (32 bytes = 256 bits entropy)
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			logging.ErrorContext(r.Context(), "Error generating reset token", "error", err)
			return
		}
		token := hex.EncodeToString(tokenBytes)
		tokenHash := email.HashToken(token)

		// Create reset token with 1-hour expiry
		expiresAt := time.Now().Add(1 * time.Hour)
		_, err = database.CreatePasswordResetToken(user.ID, tokenHash, expiresAt)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error creating password reset token", "error", err)
			return
		}

		// Send password reset email
		ctx := r.Context()
		if err := emailService.SendPasswordReset(ctx, user.Email, token); err != nil {
			logging.ErrorContext(r.Context(), "Error sending password reset email", "error", err)
			// Still return success to prevent enumeration
			return
		}

		security.PasswordResetRequested(r.Context(), clientIP, user.Email, true)
	}))

	// Auth: Reset password - completes password reset with token
	mux.HandleFunc("POST /api/auth/reset-password", authLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var req struct {
			Token    string `json:"token"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid request body",
			})
			return
		}

		// Validate token format
		if req.Token == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Reset token is required",
			})
			return
		}

		// Validate password
		if err := auth.ValidatePassword(req.Password); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": err.Error(),
			})
			return
		}

		// Hash the token and look it up
		tokenHash := email.HashToken(req.Token)
		resetToken, err := database.GetPasswordResetToken(tokenHash)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error looking up reset token", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to process reset request",
			})
			return
		}

		// Check if token exists and is valid
		clientIP := ratelimit.GetClientIP(r)
		if resetToken == nil {
			security.PasswordResetFailed(r.Context(), clientIP, "token_not_found")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid or expired reset token",
			})
			return
		}

		// Check if token is used or expired
		if resetToken.Used || time.Now().After(resetToken.ExpiresAt) {
			security.PasswordResetFailed(r.Context(), clientIP, "token_expired_or_used")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid or expired reset token",
			})
			return
		}

		// Hash the new password
		passwordHash, err := auth.HashPassword(req.Password)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error hashing new password", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to process reset request",
			})
			return
		}

		// Complete password reset in a single transaction:
		// - Update password
		// - Mark token as used
		// - Delete all tokens for user
		// - Delete all sessions for user
		if err := database.CompletePasswordReset(resetToken.UserID, tokenHash, passwordHash); err != nil {
			logging.ErrorContext(r.Context(), "Error completing password reset", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to update password",
			})
			return
		}

		security.PasswordResetSuccess(r.Context(), clientIP, resetToken.UserID)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Password has been reset successfully. Please log in with your new password.",
		})
	}))

	// Auth: Request magic link - sends a login link via email (stricter rate limiting)
	mux.HandleFunc("POST /api/auth/magic-link", passwordResetLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var req struct {
			Email string `json:"email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid request body",
			})
			return
		}

		// Normalize email
		req.Email = strings.TrimSpace(strings.ToLower(req.Email))

		// Validate email format
		if err := auth.ValidateEmail(req.Email); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": err.Error(),
			})
			return
		}

		// Always return success to prevent email enumeration
		defer func() {
			json.NewEncoder(w).Encode(map[string]string{
				"message": "If an account exists with that email, a login link has been sent.",
			})
		}()

		// Look up user (don't reveal if exists)
		user, err := database.GetUserByEmail(req.Email)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error looking up user for magic link", "error", err)
			return
		}
		if user == nil {
			// User doesn't exist - return success anyway
			logging.DebugContext(r.Context(), "Magic link requested for non-existent email", "email", req.Email)
			return
		}

		// Check if email is verified
		if !user.EmailVerified {
			logging.DebugContext(r.Context(), "Magic link requested for unverified email", "email", req.Email)
			return
		}

		// Generate magic link token (32 bytes = 256 bits entropy)
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			logging.ErrorContext(r.Context(), "Error generating magic link token", "error", err)
			return
		}
		token := hex.EncodeToString(tokenBytes)
		tokenHash := email.HashToken(token)

		// Create magic link token with 15-minute expiry
		expiresAt := time.Now().Add(15 * time.Minute)
		_, err = database.CreateMagicLinkToken(user.ID, tokenHash, expiresAt)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error creating magic link token", "error", err)
			return
		}

		// Send magic link email
		ctx := r.Context()
		if err := emailService.SendMagicLink(ctx, user.Email, token); err != nil {
			logging.ErrorContext(r.Context(), "Error sending magic link email", "error", err)
			// Still return success to prevent enumeration
			return
		}

		logging.InfoContext(r.Context(), "Magic link email sent", "email", user.Email)
	}))

	// Auth: Verify magic link - logs user in with magic link token
	mux.HandleFunc("GET /api/auth/magic-link/verify", authLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Get token from query parameter
		token := r.URL.Query().Get("token")
		if token == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Token is required",
			})
			return
		}

		// Hash the token and look it up
		tokenHash := email.HashToken(token)
		magicToken, err := database.GetMagicLinkToken(tokenHash)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error looking up magic link token", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to verify magic link",
			})
			return
		}

		// Check if token exists
		if magicToken == nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid or expired magic link",
			})
			return
		}

		// Check if token is used or expired
		if magicToken.Used || time.Now().After(magicToken.ExpiresAt) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid or expired magic link",
			})
			return
		}

		// Mark token as used atomically
		used, err := database.UseMagicLinkToken(tokenHash)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error marking magic link token as used", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to verify magic link",
			})
			return
		}
		if !used {
			// Token was already used or expired
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid or expired magic link",
			})
			return
		}

		// Get user
		user, err := database.GetUserByID(magicToken.UserID)
		if err != nil || user == nil {
			logging.ErrorContext(r.Context(), "Error getting user for magic link", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to complete login",
			})
			return
		}

		// Create session (similar to normal login)
		clientIP := ratelimit.GetClientIP(r)
		userAgent := r.Header.Get("User-Agent")
		session, err := auth.CreateSession(database, user.ID, clientIP, userAgent)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error creating session", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to create session",
			})
			return
		}

		// Set session cookie
		auth.SetSessionCookie(w, session.Token, session.ExpiresAt)

		logging.InfoContext(r.Context(), "Magic link login successful", "user_id", user.ID)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Login successful",
			"user": map[string]interface{}{
				"id":    user.ID,
				"email": user.Email,
			},
		})
	}))

	// Auth: Verify email - verifies email address with token
	mux.HandleFunc("GET /api/auth/verify", authLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Get token from query parameter
		token := r.URL.Query().Get("token")
		if token == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Verification token is required",
			})
			return
		}

		// Hash the token and look it up
		tokenHash := email.HashToken(token)
		verifyToken, err := database.GetEmailVerificationToken(tokenHash)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error looking up verification token", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to process verification request",
			})
			return
		}

		// Check if token exists and is valid
		if verifyToken == nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid or expired verification token",
			})
			return
		}

		// Check if token is used or expired
		if verifyToken.Used || time.Now().After(verifyToken.ExpiresAt) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid or expired verification token",
			})
			return
		}

		// Verify the email (marks token as used and sets email_verified=1)
		success, err := database.UseEmailVerificationToken(tokenHash)
		if err != nil || !success {
			logging.ErrorContext(r.Context(), "Error verifying email", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to verify email",
			})
			return
		}

		logging.InfoContext(r.Context(), "Email verified", "user_id", verifyToken.UserID)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Email verified successfully. You can now log in.",
		})
	}))

	// Auth: Resend verification email (rate limited)
	mux.HandleFunc("POST /api/auth/resend-verification", passwordResetLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var req struct {
			Email string `json:"email"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid request body",
			})
			return
		}

		// Normalize email
		req.Email = strings.TrimSpace(strings.ToLower(req.Email))

		// Validate email format
		if err := auth.ValidateEmail(req.Email); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": err.Error(),
			})
			return
		}

		// Always return success to prevent email enumeration
		defer func() {
			json.NewEncoder(w).Encode(map[string]string{
				"message": "If an unverified account exists with that email, a verification link has been sent.",
			})
		}()

		// Look up user
		user, err := database.GetUserByEmail(req.Email)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error looking up user for resend verification", "error", err)
			return
		}
		if user == nil {
			// User doesn't exist - return success anyway
			return
		}

		// Check if already verified
		if user.EmailVerified {
			// Already verified - return success anyway to prevent enumeration
			return
		}

		// Generate new verification token (32 bytes = 256 bits entropy)
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			logging.ErrorContext(r.Context(), "Error generating verification token", "error", err)
			return
		}
		token := hex.EncodeToString(tokenBytes)
		tokenHash := email.HashToken(token)

		// Create verification token with 24-hour expiry
		expiresAt := time.Now().Add(24 * time.Hour)
		_, err = database.CreateEmailVerificationToken(user.ID, tokenHash, expiresAt)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error creating verification token", "error", err)
			return
		}

		// Send verification email
		ctx := r.Context()
		if err := emailService.SendEmailVerification(ctx, user.Email, token); err != nil {
			logging.ErrorContext(r.Context(), "Error sending verification email", "error", err)
			return
		}

		logging.InfoContext(r.Context(), "Verification email resent", "email", user.Email)
	}))

	// Upload endpoint - accepts video files (rate limited: 10/min per IP)
	mux.HandleFunc("POST /api/upload", uploadLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Get authenticated user (if any)
		token := auth.GetTokenFromRequest(r)
		user, _, _ := auth.ValidateSession(database, token)

		// Get session_id from form for anonymous session tracking
		sessionID := r.URL.Query().Get("session_id")

		// Enforce upload limit for anonymous users (2 uploads max)
		if user == nil && sessionID != "" {
			count, err := database.CountVideosBySession(sessionID)
			if err != nil {
				logging.ErrorContext(r.Context(), "Error counting videos for session", "error", err)
			} else if count >= 2 {
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "Anonymous users are limited to 2 uploads. Please register to upload more videos.",
				})
				return
			}
		}

		// Limit request body size
		r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

		// Parse multipart form
		if err := r.ParseMultipartForm(maxUploadSize); err != nil {
			logging.ErrorContext(r.Context(), "Error parsing form", "error", err)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "File too large or invalid form data",
			})
			return
		}

		// Get the file from the form
		file, header, err := r.FormFile("video")
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting form file", "error", err)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "No video file provided",
			})
			return
		}
		defer file.Close()

		// Validate file type by checking content type against whitelist
		contentType := header.Header.Get("Content-Type")
		allowedMIMETypes := map[string]bool{
			"video/mp4":        true,
			"video/webm":       true,
			"video/quicktime":  true, // .mov files
			"video/x-m4v":      true, // .m4v files
			"video/mpeg":       true, // .mpeg, .mpg files
			"video/x-msvideo":  true, // .avi files
			"video/x-matroska": true, // .mkv files
			"video/ogg":        true, // .ogv files
		}
		if !allowedMIMETypes[contentType] {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error":             "Unsupported video format. Allowed formats: MP4, WebM, MOV, M4V, MPEG, AVI, MKV, OGV",
				"provided_mimetype": contentType,
			})
			return
		}

		// Validate magic bytes (file signature) - defense against MIME spoofing
		// Read first 12 bytes to check signature, then seek back to start
		magicBytes := make([]byte, 12)
		n, err := file.Read(magicBytes)
		if err != nil || n < 8 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "File is too small or could not be read",
			})
			return
		}
		// Seek back to beginning for copy later
		if seeker, ok := file.(io.Seeker); ok {
			if _, err := seeker.Seek(0, io.SeekStart); err != nil {
				logging.ErrorContext(r.Context(), "Failed to seek file", "error", err)
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "Failed to process file",
				})
				return
			}
		}
		// Validate the magic bytes against known video formats
		if err := audio.ValidateMagicBytes(bytes.NewReader(magicBytes[:n])); err != nil {
			logging.WarnContext(r.Context(), "Invalid magic bytes", "mimetype", contentType, "error", err)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "File content does not match a valid video format. The file may be corrupted or renamed.",
			})
			return
		}

		// Generate unique ID for this upload
		uploadID, err := generateID()
		if err != nil {
			logging.ErrorContext(r.Context(), "Error generating upload ID", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate upload ID"})
			return
		}

		// Get file extension from original filename
		ext := filepath.Ext(header.Filename)
		if ext == "" {
			ext = ".mp4" // default extension
		}

		// Create destination file
		destPath := filepath.Join(uploadDir, uploadID+ext)
		destFile, err := os.Create(destPath)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error creating destination file", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to save file",
			})
			return
		}
		defer destFile.Close()

		// Copy the uploaded file to destination
		written, err := io.Copy(destFile, file)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error copying file", "error", err)
			os.Remove(destPath) // Clean up partial file
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to save file",
			})
			return
		}
		destFile.Close() // Close before encrypting

		logging.InfoContext(r.Context(), "Uploaded file", "filename", header.Filename, "bytes", written, "dest_path", destPath)

		// Validate the file is actually a valid video (defense-in-depth beyond MIME check)
		if err := audio.ValidateVideoFile(destPath); err != nil {
			logging.WarnContext(r.Context(), "Video validation failed", "path", destPath, "error", err)
			os.Remove(destPath) // Clean up invalid file
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "File is not a valid video. Please upload a valid video file.",
			})
			return
		}

		// Generate thumbnail from the validated video (before encryption)
		thumbPath := filepath.Join(uploadDir, uploadID+"_thumb.jpg")
		var encThumbPath *string
		if err := audio.GenerateThumbnail(destPath, thumbPath); err != nil {
			logging.WarnContext(r.Context(), "Failed to generate thumbnail", "path", destPath, "error", err)
			// Non-fatal: continue without thumbnail
		} else {
			// Encrypt the thumbnail with current key version
			encPath, _, err := multiEnc.EncryptFile(thumbPath)
			if err != nil {
				logging.WarnContext(r.Context(), "Failed to encrypt thumbnail", "error", err)
				os.Remove(thumbPath) // Clean up unencrypted thumbnail
			} else {
				os.Remove(thumbPath) // Clean up unencrypted thumbnail
				encThumbPath = &encPath
				logging.InfoContext(r.Context(), "Generated and encrypted thumbnail", "thumb_path", encPath)
			}
		}

		// Encrypt the file at rest with current key version
		encPath, keyVersion, err := multiEnc.EncryptFile(destPath)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error encrypting file", "error", err)
			os.Remove(destPath)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to encrypt file",
			})
			return
		}

		// Remove the unencrypted file
		os.Remove(destPath)
		logging.InfoContext(r.Context(), "Encrypted file", "src_path", destPath, "enc_path", encPath, "key_version", keyVersion)

		// Save video to database with encrypted file path and key version
		video := &db.Video{
			ID:            uploadID,
			Filename:      header.Filename,
			Size:          written,
			ContentType:   contentType,
			FilePath:      encPath,
			ThumbnailPath: encThumbPath,
			KeyVersion:    keyVersion,
			CreatedAt:     time.Now(),
		}
		// Set user_id if authenticated
		if user != nil {
			video.UserID = &user.ID
		}
		// Set session_id for anonymous tracking
		if sessionID != "" {
			video.SessionID = &sessionID
		}
		if err := database.CreateVideo(video); err != nil {
			logging.ErrorContext(r.Context(), "Error saving video to database", "error", err)
			os.Remove(encPath) // Clean up encrypted file
			if encThumbPath != nil {
				os.Remove(*encThumbPath) // Clean up encrypted thumbnail
			}
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to save video record",
			})
			return
		}

		// Create initial transcription record
		transcriptionID, err := generateID()
		if err != nil {
			logging.ErrorContext(r.Context(), "Error generating transcription ID", "error", err)
			os.Remove(encPath) // Clean up encrypted file
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate transcription ID"})
			return
		}
		transcription := &db.Transcription{
			ID:        transcriptionID,
			VideoID:   uploadID,
			Status:    "pending",
			Message:   "Video uploaded, ready for transcription",
			Progress:  0,
			CreatedAt: time.Now(),
		}
		if err := database.CreateTranscription(transcription); err != nil {
			logging.ErrorContext(r.Context(), "Error creating transcription record", "error", err)
			// Don't fail the upload, transcription record can be created later
		}

		// Record metrics
		metrics.RecordUploadSuccess()
		metrics.RecordUploadBytes(written)

		// Return success with upload ID
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "success",
			"upload_id": uploadID,
			"filename":  header.Filename,
			"size":      written,
			"message":   fmt.Sprintf("File uploaded successfully (%d bytes)", written),
		})
	}))

	// Initialize chunked upload session (rate limited: 10/min per IP)
	mux.HandleFunc("POST /api/upload/init", uploadLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Get authenticated user (if any)
		token := auth.GetTokenFromRequest(r)
		user, _, _ := auth.ValidateSession(database, token)

		// Parse request body
		var req struct {
			Filename    string `json:"filename"`
			Size        int64  `json:"size"`
			ContentType string `json:"content_type"`
			ChunkSize   int64  `json:"chunk_size"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
			return
		}

		// Validate required fields
		if req.Filename == "" || req.Size <= 0 || req.ContentType == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Missing required fields: filename, size, content_type"})
			return
		}

		// Validate MIME type
		allowedMIMETypes := map[string]bool{
			"video/mp4":        true,
			"video/webm":       true,
			"video/quicktime":  true,
			"video/x-m4v":      true,
			"video/mpeg":       true,
			"video/x-msvideo":  true,
			"video/x-matroska": true,
			"video/ogg":        true,
		}
		if !allowedMIMETypes[req.ContentType] {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error":             "Unsupported video format. Allowed formats: MP4, WebM, MOV, M4V, MPEG, AVI, MKV, OGV",
				"provided_mimetype": req.ContentType,
			})
			return
		}

		// Validate total size
		if req.Size > maxUploadSize {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": fmt.Sprintf("File too large. Maximum size is %d MB", maxUploadSize/(1<<20)),
			})
			return
		}

		// Get session_id from query for anonymous tracking
		sessionID := r.URL.Query().Get("session_id")

		// Enforce upload limit for anonymous users
		if user == nil && sessionID != "" {
			count, err := database.CountVideosBySession(sessionID)
			if err != nil {
				logging.ErrorContext(r.Context(), "Error counting videos for session", "error", err)
			} else if count >= 2 {
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "Anonymous users are limited to 2 uploads. Please register to upload more videos.",
				})
				return
			}
		}

		// Use default chunk size if not specified
		requestedChunkSize := req.ChunkSize
		if requestedChunkSize <= 0 {
			requestedChunkSize = chunkSize
		}
		// Cap chunk size at server's configured limit
		if requestedChunkSize > chunkSize {
			requestedChunkSize = chunkSize
		}

		// Calculate total chunks
		totalChunks := int((req.Size + requestedChunkSize - 1) / requestedChunkSize)

		// Generate session ID
		uploadSessionID, err := generateID()
		if err != nil {
			logging.ErrorContext(r.Context(), "Error generating upload session ID", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate session ID"})
			return
		}

		// Create session record
		expiresAt := time.Now().Add(sessionExpiry)
		session := &db.UploadSession{
			ID:          uploadSessionID,
			Filename:    req.Filename,
			ContentType: req.ContentType,
			TotalSize:   req.Size,
			ChunkSize:   requestedChunkSize,
			TotalChunks: totalChunks,
			Status:      "in_progress",
			CreatedAt:   time.Now(),
			ExpiresAt:   expiresAt,
		}
		if user != nil {
			session.UserID = &user.ID
		}
		if sessionID != "" {
			session.SessionID = &sessionID
		}

		if err := database.CreateUploadSession(session); err != nil {
			logging.ErrorContext(r.Context(), "Error creating upload session", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create upload session"})
			return
		}

		// Create chunks directory
		chunksDir := filepath.Join(uploadDir, "chunks", uploadSessionID)
		if err := os.MkdirAll(chunksDir, 0755); err != nil {
			logging.ErrorContext(r.Context(), "Error creating chunks directory", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create chunks directory"})
			return
		}

		logging.InfoContext(r.Context(), "Created chunked upload session",
			"session_id", uploadSessionID,
			"filename", req.Filename,
			"total_size", req.Size,
			"chunk_size", requestedChunkSize,
			"total_chunks", totalChunks)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"upload_session_id": uploadSessionID,
			"chunk_size":        requestedChunkSize,
			"total_chunks":      totalChunks,
			"expires_at":        expiresAt.Format(time.RFC3339),
		})
	}))

	// Upload a single chunk (rate limited: 60/min per IP)
	mux.HandleFunc("POST /api/upload/chunk", chunkLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Limit chunk size
		r.Body = http.MaxBytesReader(w, r.Body, chunkSize+10*1024) // chunk + overhead

		// Parse multipart form
		if err := r.ParseMultipartForm(chunkSize + 10*1024); err != nil {
			logging.ErrorContext(r.Context(), "Error parsing chunk form", "error", err)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Chunk too large or invalid form data"})
			return
		}

		// Get session ID and chunk index from form
		uploadSessionID := r.FormValue("upload_session_id")
		chunkIndexStr := r.FormValue("chunk_index")
		if uploadSessionID == "" || chunkIndexStr == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Missing upload_session_id or chunk_index"})
			return
		}

		chunkIndex, err := strconv.Atoi(chunkIndexStr)
		if err != nil || chunkIndex < 0 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid chunk_index"})
			return
		}

		// Get session
		session, err := database.GetUploadSession(uploadSessionID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting upload session", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get upload session"})
			return
		}
		if session == nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "Upload session not found"})
			return
		}

		// Check session expiration
		if time.Now().After(session.ExpiresAt) {
			w.WriteHeader(http.StatusGone)
			json.NewEncoder(w).Encode(map[string]string{"error": "Upload session has expired"})
			return
		}

		// Check session status
		if session.Status != "in_progress" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Upload session is not in progress"})
			return
		}

		// Validate ownership
		token := auth.GetTokenFromRequest(r)
		user, _, _ := auth.ValidateSession(database, token)
		requestSessionID := r.URL.Query().Get("session_id")

		if session.UserID != nil {
			if user == nil || *session.UserID != user.ID {
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]string{"error": "Access denied"})
				return
			}
		} else if session.SessionID != nil {
			if requestSessionID == "" || *session.SessionID != requestSessionID {
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]string{"error": "Access denied"})
				return
			}
		}

		// Validate chunk index
		if chunkIndex >= session.TotalChunks {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid chunk index"})
			return
		}

		// Check if chunk already exists (idempotent)
		existingChunk, err := database.GetUploadChunk(uploadSessionID, chunkIndex)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error checking existing chunk", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to check chunk status"})
			return
		}
		if existingChunk != nil {
			// Chunk already uploaded, return success for idempotency
			totalReceived, err := database.GetTotalReceivedBytes(uploadSessionID)
			if err != nil {
				logging.WarnContext(r.Context(), "Error getting total received bytes", "error", err, "session_id", uploadSessionID)
				totalReceived = 0
			}
			chunkCount, err := database.CountUploadChunks(uploadSessionID)
			if err != nil {
				logging.WarnContext(r.Context(), "Error counting upload chunks", "error", err, "session_id", uploadSessionID)
				chunkCount = 0
			}
			progress := int(float64(chunkCount) / float64(session.TotalChunks) * 100)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"chunk_index":    chunkIndex,
				"received_bytes": existingChunk.Size,
				"total_received": totalReceived,
				"progress":       progress,
				"already_exists": true,
			})
			return
		}

		// Get the chunk file
		chunkFile, _, err := r.FormFile("chunk")
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting chunk file", "error", err)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "No chunk data provided"})
			return
		}
		defer chunkFile.Close()

		// Calculate expected chunk size
		expectedSize := session.ChunkSize
		if chunkIndex == session.TotalChunks-1 {
			// Last chunk may be smaller
			expectedSize = session.TotalSize - int64(chunkIndex)*session.ChunkSize
		}

		// Save chunk to disk
		chunkPath := filepath.Join(uploadDir, "chunks", uploadSessionID, fmt.Sprintf("chunk_%d.part", chunkIndex))
		destFile, err := os.Create(chunkPath)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error creating chunk file", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save chunk"})
			return
		}
		defer destFile.Close()

		written, err := io.Copy(destFile, chunkFile)
		if err != nil {
			os.Remove(chunkPath)
			logging.ErrorContext(r.Context(), "Error writing chunk file", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save chunk"})
			return
		}
		destFile.Close()

		// Validate chunk size (allow up to expected size; last chunk may be smaller)
		if written > expectedSize {
			os.Remove(chunkPath)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": fmt.Sprintf("Chunk too large. Expected max %d bytes, got %d", expectedSize, written),
			})
			return
		}

		// Record chunk in database
		chunkID, err := generateID()
		if err != nil {
			os.Remove(chunkPath)
			logging.ErrorContext(r.Context(), "Error generating chunk ID", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate chunk ID"})
			return
		}

		chunk := &db.UploadChunk{
			ID:              chunkID,
			UploadSessionID: uploadSessionID,
			ChunkIndex:      chunkIndex,
			ChunkPath:       chunkPath,
			Size:            written,
			CreatedAt:       time.Now(),
		}
		if err := database.CreateUploadChunk(chunk); err != nil {
			os.Remove(chunkPath)
			logging.ErrorContext(r.Context(), "Error saving chunk record", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to record chunk"})
			return
		}

		// Calculate progress
		totalReceived, err := database.GetTotalReceivedBytes(uploadSessionID)
		if err != nil {
			logging.WarnContext(r.Context(), "Error getting total received bytes", "error", err, "session_id", uploadSessionID)
			totalReceived = 0
		}
		chunkCount, err := database.CountUploadChunks(uploadSessionID)
		if err != nil {
			logging.WarnContext(r.Context(), "Error counting upload chunks", "error", err, "session_id", uploadSessionID)
			chunkCount = 0
		}
		progress := int(float64(chunkCount) / float64(session.TotalChunks) * 100)

		logging.InfoContext(r.Context(), "Received chunk",
			"session_id", uploadSessionID,
			"chunk_index", chunkIndex,
			"size", written,
			"progress", progress)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"chunk_index":    chunkIndex,
			"received_bytes": written,
			"total_received": totalReceived,
			"progress":       progress,
		})
	}))

	// Complete chunked upload (rate limited: 10/min per IP)
	mux.HandleFunc("POST /api/upload/complete", uploadLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var req struct {
			UploadSessionID string `json:"upload_session_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
			return
		}

		if req.UploadSessionID == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Missing upload_session_id"})
			return
		}

		// Get session
		session, err := database.GetUploadSession(req.UploadSessionID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting upload session", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get upload session"})
			return
		}
		if session == nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "Upload session not found"})
			return
		}

		// Validate ownership
		token := auth.GetTokenFromRequest(r)
		user, _, _ := auth.ValidateSession(database, token)
		requestSessionID := r.URL.Query().Get("session_id")

		if session.UserID != nil {
			if user == nil || *session.UserID != user.ID {
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]string{"error": "Access denied"})
				return
			}
		} else if session.SessionID != nil {
			if requestSessionID == "" || *session.SessionID != requestSessionID {
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]string{"error": "Access denied"})
				return
			}
		}

		// Check session status
		if session.Status != "in_progress" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Upload session is not in progress"})
			return
		}

		// Verify all chunks received
		chunkCount, err := database.CountUploadChunks(req.UploadSessionID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error counting chunks", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to verify chunks"})
			return
		}
		if chunkCount != session.TotalChunks {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": fmt.Sprintf("Not all chunks received. Expected %d, got %d", session.TotalChunks, chunkCount),
			})
			return
		}

		// Get chunks in order
		chunks, err := database.GetUploadChunks(req.UploadSessionID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting chunks", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get chunks"})
			return
		}

		// Generate upload ID for the final file
		uploadID, err := generateID()
		if err != nil {
			logging.ErrorContext(r.Context(), "Error generating upload ID", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate upload ID"})
			return
		}

		// Get file extension
		ext := filepath.Ext(session.Filename)
		if ext == "" {
			ext = ".mp4"
		}

		// Reassemble chunks into final file
		destPath := filepath.Join(uploadDir, uploadID+ext)
		destFile, err := os.Create(destPath)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error creating destination file", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to create destination file"})
			return
		}

		var totalWritten int64
		for _, chunk := range chunks {
			chunkFile, err := os.Open(chunk.ChunkPath)
			if err != nil {
				destFile.Close()
				os.Remove(destPath)
				logging.ErrorContext(r.Context(), "Error opening chunk file", "chunk_index", chunk.ChunkIndex, "error", err)
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"error": "Failed to read chunk"})
				return
			}
			written, err := io.Copy(destFile, chunkFile)
			chunkFile.Close()
			if err != nil {
				destFile.Close()
				os.Remove(destPath)
				logging.ErrorContext(r.Context(), "Error copying chunk", "chunk_index", chunk.ChunkIndex, "error", err)
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"error": "Failed to assemble file"})
				return
			}
			totalWritten += written
		}
		destFile.Close()

		logging.InfoContext(r.Context(), "Reassembled chunks", "upload_id", uploadID, "total_bytes", totalWritten)

		// Validate magic bytes (file signature) - defense against MIME spoofing
		if err := audio.ValidateMagicBytesFromFile(destPath); err != nil {
			logging.WarnContext(r.Context(), "Invalid magic bytes for reassembled file", "path", destPath, "error", err)
			os.Remove(destPath)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "File content does not match a valid video format. The file may be corrupted.",
			})
			return
		}

		// Validate the assembled file with ffprobe
		if err := audio.ValidateVideoFile(destPath); err != nil {
			logging.WarnContext(r.Context(), "Video validation failed", "path", destPath, "error", err)
			os.Remove(destPath)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Assembled file is not a valid video. Please try uploading again.",
			})
			return
		}

		// Generate thumbnail
		thumbPath := filepath.Join(uploadDir, uploadID+"_thumb.jpg")
		var encThumbPath *string
		if err := audio.GenerateThumbnail(destPath, thumbPath); err != nil {
			logging.WarnContext(r.Context(), "Failed to generate thumbnail", "path", destPath, "error", err)
		} else {
			encPath, _, err := multiEnc.EncryptFile(thumbPath)
			if err != nil {
				logging.WarnContext(r.Context(), "Failed to encrypt thumbnail", "error", err)
				os.Remove(thumbPath)
			} else {
				os.Remove(thumbPath)
				encThumbPath = &encPath
			}
		}

		// Encrypt the file with current key version
		encPath, keyVersion, err := multiEnc.EncryptFile(destPath)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error encrypting file", "error", err)
			os.Remove(destPath)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to encrypt file"})
			return
		}
		os.Remove(destPath)

		// Save video to database with key version
		video := &db.Video{
			ID:            uploadID,
			Filename:      session.Filename,
			Size:          totalWritten,
			ContentType:   session.ContentType,
			FilePath:      encPath,
			ThumbnailPath: encThumbPath,
			KeyVersion:    keyVersion,
			CreatedAt:     time.Now(),
			UserID:        session.UserID,
			SessionID:     session.SessionID,
		}
		if err := database.CreateVideo(video); err != nil {
			logging.ErrorContext(r.Context(), "Error saving video to database", "error", err)
			os.Remove(encPath)
			if encThumbPath != nil {
				os.Remove(*encThumbPath)
			}
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save video record"})
			return
		}

		// Create transcription record
		transcriptionID, err := generateID()
		if err != nil {
			logging.ErrorContext(r.Context(), "Error generating transcription ID", "error", err)
		} else {
			transcription := &db.Transcription{
				ID:        transcriptionID,
				VideoID:   uploadID,
				Status:    "pending",
				Message:   "Video uploaded, ready for transcription",
				Progress:  0,
				CreatedAt: time.Now(),
			}
			if err := database.CreateTranscription(transcription); err != nil {
				logging.ErrorContext(r.Context(), "Error creating transcription record", "error", err)
			}
		}

		// Mark session as complete
		if err := database.UpdateUploadSessionStatus(req.UploadSessionID, "complete"); err != nil {
			logging.ErrorContext(r.Context(), "Error updating session status", "error", err)
		}

		// Clean up chunk files
		chunksDir := filepath.Join(uploadDir, "chunks", req.UploadSessionID)
		for _, chunk := range chunks {
			os.Remove(chunk.ChunkPath)
		}
		os.Remove(chunksDir)

		// Record metrics
		metrics.RecordUploadSuccess()
		metrics.RecordUploadBytes(totalWritten)

		logging.InfoContext(r.Context(), "Completed chunked upload",
			"session_id", req.UploadSessionID,
			"upload_id", uploadID,
			"filename", session.Filename,
			"size", totalWritten)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "success",
			"upload_id": uploadID,
			"filename":  session.Filename,
			"size":      totalWritten,
			"message":   fmt.Sprintf("File uploaded successfully (%d bytes)", totalWritten),
		})
	}))

	// Get chunked upload status (rate limited: 30/min per IP)
	mux.HandleFunc("GET /api/upload/status/{session_id}", scriptLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		uploadSessionID := r.PathValue("session_id")
		if uploadSessionID == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Session ID required"})
			return
		}

		session, err := database.GetUploadSession(uploadSessionID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting upload session", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get upload session"})
			return
		}
		if session == nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "Upload session not found"})
			return
		}

		// Validate ownership
		token := auth.GetTokenFromRequest(r)
		user, _, _ := auth.ValidateSession(database, token)
		requestSessionID := r.URL.Query().Get("session_id")

		if session.UserID != nil {
			if user == nil || *session.UserID != user.ID {
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]string{"error": "Access denied"})
				return
			}
		} else if session.SessionID != nil {
			if requestSessionID == "" || *session.SessionID != requestSessionID {
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]string{"error": "Access denied"})
				return
			}
		}

		// Get received chunks
		receivedChunks, err := database.GetReceivedChunkIndices(uploadSessionID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting received chunks", "error", err)
			receivedChunks = []int{}
		}

		receivedBytes, _ := database.GetTotalReceivedBytes(uploadSessionID)
		progress := 0
		if session.TotalChunks > 0 {
			progress = int(float64(len(receivedChunks)) / float64(session.TotalChunks) * 100)
		}

		// Check if expired
		status := session.Status
		if time.Now().After(session.ExpiresAt) && status == "in_progress" {
			status = "expired"
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"upload_session_id": session.ID,
			"filename":          session.Filename,
			"total_size":        session.TotalSize,
			"chunk_size":        session.ChunkSize,
			"total_chunks":      session.TotalChunks,
			"received_chunks":   receivedChunks,
			"received_bytes":    receivedBytes,
			"progress":          progress,
			"status":            status,
			"expires_at":        session.ExpiresAt.Format(time.RFC3339),
		})
	}))

	// Start transcription for an upload (rate limited: 5/min per IP)
	// Optional query parameter: language (ISO 639-1 code, e.g., "en", "es", "ja")
	// If not provided or "auto", whisper will auto-detect the language
	mux.HandleFunc("POST /api/transcribe/{id}", transcribeLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		uploadID := r.PathValue("id")
		if uploadID == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Upload ID required",
			})
			return
		}

		// Parse optional language parameter (defaults to "auto" for auto-detection)
		language := r.URL.Query().Get("language")
		if language == "" {
			language = "auto"
		}
		// Validate language code length
		if err := validation.ValidateLanguageCode(language); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": err.Error(),
			})
			return
		}

		// Find the video file and get key version for decryption
		video, err := getVideoForDecryption(uploadID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Video not found for transcription", "error", err, "upload_id", uploadID)
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": errmsg.ForVideoNotFound(err),
			})
			return
		}
		videoPath := video.FilePath
		keyVersion := video.KeyVersion

		// Check if already processing from database
		existingTranscription, err := database.GetTranscription(uploadID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting transcription", "error", err)
		}
		if existingTranscription != nil {
			if existingTranscription.Status == "processing" {
				json.NewEncoder(w).Encode(map[string]string{
					"status":  "processing",
					"message": "Transcription already in progress",
				})
				return
			}
			if existingTranscription.Status == "complete" {
				json.NewEncoder(w).Encode(dbTranscriptionToStatus(existingTranscription))
				return
			}
		}

		// Create or update transcription record
		if existingTranscription == nil {
			newTranscriptionID, err := generateID()
			if err != nil {
				logging.ErrorContext(r.Context(), "Error generating transcription ID", "error", err)
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate transcription ID"})
				return
			}
			transcription := &db.Transcription{
				ID:        newTranscriptionID,
				VideoID:   uploadID,
				Status:    "processing",
				Message:   "Extracting audio...",
				Progress:  10,
				CreatedAt: time.Now(),
			}
			if err := database.CreateTranscription(transcription); err != nil {
				logging.ErrorContext(r.Context(), "Error creating transcription record", "error", err)
			}
		} else {
			database.UpdateTranscriptionStatus(uploadID, "processing", "Extracting audio...", 10)
		}

		// Process in background (capture language and key version in closure)
		go func(lang string, kv int) {
			// Panic recovery to prevent goroutine crashes from leaving jobs in stuck state
			defer func() {
				if r := recover(); r != nil {
					logging.Error("Panic in transcription goroutine", "upload_id", uploadID, "panic", r)
					metrics.RecordTranscriptionFailed()
					database.FailTranscription(uploadID, "Internal error: transcription process crashed")
				}
			}()

			transcriptionStart := time.Now()
			metrics.RecordTranscriptionStarted()
			logging.Info("Starting transcription", "upload_id", uploadID, "language", lang, "key_version", kv)

			// Decrypt video file if encrypted
			workingVideoPath := videoPath
			if strings.HasSuffix(videoPath, ".age") {
				database.UpdateTranscriptionStatus(uploadID, "processing", "Decrypting video...", 5)
				decryptedPath, err := multiEnc.DecryptToTempFile(videoPath, kv)
				if err != nil {
					logging.Error("Video decryption failed", "error", err)
					metrics.RecordTranscriptionFailed()
					database.FailTranscription(uploadID, fmt.Sprintf("Video decryption failed: %v", err))
					return
				}
				workingVideoPath = decryptedPath
				defer os.Remove(decryptedPath) // Clean up decrypted file when done
			}

			// Extract audio
			database.UpdateTranscriptionStatus(uploadID, "processing", "Extracting audio...", 10)
			audioPath := filepath.Join(uploadDir, uploadID+".wav")
			if err := audioExtractor.ExtractAudio(workingVideoPath, audioPath); err != nil {
				logging.Error("Audio extraction failed", "error", err)
				metrics.RecordTranscriptionFailed()
				database.FailTranscription(uploadID, fmt.Sprintf("Audio extraction failed: %v", err))
				return
			}

			database.UpdateTranscriptionStatus(uploadID, "processing", "Running transcription...", 30)

			// Start a goroutine to simulate progress updates during transcription
			// Since whisper doesn't provide progress callbacks, we estimate based on time
			progressDone := make(chan struct{})
			go func() {
				ticker := time.NewTicker(5 * time.Second)
				defer ticker.Stop()
				progress := 30
				for {
					select {
					case <-progressDone:
						return
					case <-ticker.C:
						// Increment progress slowly from 30% to 90% during transcription
						if progress < 90 {
							progress += 5
							database.UpdateTranscriptionStatus(uploadID, "processing", "Transcribing audio...", progress)
						}
					}
				}
			}()

			// Run whisper (server or CLI based on configuration)
			outputPath := filepath.Join(uploadDir, uploadID+"_transcript")
			result, err := transcribe(audioPath, outputPath, lang)
			close(progressDone) // Stop progress simulation

			if err != nil {
				logging.Error("Transcription failed", "error", err)
				metrics.RecordTranscriptionFailed()
				database.FailTranscription(uploadID, fmt.Sprintf("Transcription failed: %v", err))
				return
			}

			// Convert segments to database format
			segments := make([]db.Segment, len(result.Segments))
			for i, s := range result.Segments {
				segments[i] = db.Segment{
					ID:    s.ID,
					Start: s.Start,
					End:   s.End,
					Text:  s.Text,
				}
			}

			// Success - save to database and record metrics
			transcriptionDuration := time.Since(transcriptionStart)
			metrics.RecordTranscriptionCompleted(transcriptionDuration)
			logging.Info("Transcription complete", "upload_id", uploadID, "segment_count", len(result.Segments), "duration_sec", transcriptionDuration.Seconds())
			if err := database.CompleteTranscription(uploadID, result.Language, result.Duration, result.Text, segments); err != nil {
				logging.Error("Error saving transcription result", "error", err)
			}

			// Clean up intermediate files
			os.Remove(audioPath)
		}(language, keyVersion)

		// Return immediately with processing status
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "processing",
			"message": "Transcription started",
		})
	}))

	// Get transcription status/result
	mux.HandleFunc("GET /api/transcribe/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		uploadID := r.PathValue("id")
		if uploadID == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Upload ID required",
			})
			return
		}

		transcription, err := database.GetTranscription(uploadID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting transcription", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to get transcription status",
			})
			return
		}

		if transcription == nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "No transcription found for this upload",
			})
			return
		}

		json.NewEncoder(w).Encode(dbTranscriptionToStatus(transcription))
	})

	// Update segments for a transcription (edit subtitles)
	mux.HandleFunc("PUT /api/transcribe/{id}/segments", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		uploadID := r.PathValue("id")
		if uploadID == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Upload ID required",
			})
			return
		}

		// Check if transcription exists and is complete
		transcription, err := database.GetTranscription(uploadID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting transcription", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to get transcription",
			})
			return
		}
		if transcription == nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "No transcription found for this upload",
			})
			return
		}
		if transcription.Status != "complete" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error":  "Cannot edit segments - transcription not complete",
				"status": transcription.Status,
			})
			return
		}

		// Parse request body
		var req struct {
			Segments []db.Segment `json:"segments"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid request body",
			})
			return
		}

		// Validate segments
		for i, seg := range req.Segments {
			if seg.Start < 0 || seg.End < 0 {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{
					"error": fmt.Sprintf("Segment %d has invalid timing (negative values)", i),
				})
				return
			}
			if seg.Start > seg.End {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{
					"error": fmt.Sprintf("Segment %d: start time cannot be greater than end time", i),
				})
				return
			}
			// Validate segment text length
			if err := validation.ValidateSegmentText(seg.Text); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{
					"error": fmt.Sprintf("Segment %d: %v", i, err),
				})
				return
			}
		}

		// Update segments in database
		if err := database.UpdateSegments(uploadID, req.Segments); err != nil {
			logging.ErrorContext(r.Context(), "Error updating segments", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to update segments",
			})
			return
		}

		logging.InfoContext(r.Context(), "Updated segments", "upload_id", uploadID, "segment_count", len(req.Segments))
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "success",
			"segments": len(req.Segments),
		})
	})

	// Paste-and-match: align user-provided transcript with whisper timing
	mux.HandleFunc("POST /api/transcribe/{id}/align", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		uploadID := r.PathValue("id")
		if uploadID == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Upload ID required",
			})
			return
		}

		// Check if transcription exists and is complete
		transcription, err := database.GetTranscription(uploadID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting transcription", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to get transcription",
			})
			return
		}
		if transcription == nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "No transcription found for this upload",
			})
			return
		}
		if transcription.Status != "complete" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error":  "Cannot align - transcription not complete",
				"status": transcription.Status,
			})
			return
		}

		// Parse request body
		var req struct {
			Text            string `json:"text"`
			Mode            string `json:"mode"`              // "lyrics" for music-specific alignment
			ConvertToScript string `json:"convert_to_script"` // Optional: target script (e.g., "Devanagari")
			Language        string `json:"language"`          // Required if convert_to_script is set
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid request body",
			})
			return
		}

		// Validate align text length
		if err := validation.ValidateAlignText(req.Text); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": err.Error(),
			})
			return
		}

		// Validate align mode
		if _, err := validation.ValidateAlignMode(req.Mode); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": err.Error(),
			})
			return
		}

		// Script conversion if requested
		scriptConverted := false
		var targetScript script.Script
		if req.ConvertToScript != "" {
			targetScript = script.Script(req.ConvertToScript)
			if !script.IsScriptSupported(targetScript) {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":             "Unsupported target script",
					"supported_scripts": script.SupportedScripts(),
				})
				return
			}
			if !script.IsLanguageSupported(req.Language) {
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":               "Language required for script conversion",
					"supported_languages": script.SupportedLanguages(),
				})
				return
			}
		}

		// Get existing segments
		existingSegments, err := transcription.GetSegments()
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting segments", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to get existing segments",
			})
			return
		}

		// Convert db.Segment to align.Segment
		alignSegments := make([]align.Segment, len(existingSegments))
		for i, s := range existingSegments {
			alignSegments[i] = align.Segment{
				ID:    s.ID,
				Start: s.Start,
				End:   s.End,
				Text:  s.Text,
			}
		}

		// Perform alignment - use lyrics mode if specified
		var result align.AlignmentResult
		if req.Mode == "lyrics" {
			result = align.AlignLyrics(req.Text, alignSegments)
		} else {
			result = align.AlignTranscript(req.Text, alignSegments)
		}

		// Convert back to db.Segment
		newSegments := make([]db.Segment, len(result.Segments))
		for i, s := range result.Segments {
			newSegments[i] = db.Segment{
				ID:    s.ID,
				Start: s.Start,
				End:   s.End,
				Text:  s.Text,
			}
		}

		// Apply script conversion if requested
		if req.ConvertToScript != "" {
			converter := script.NewConverter()
			for i := range newSegments {
				converted, err := converter.Convert(newSegments[i].Text, req.Language, targetScript)
				if err == nil {
					newSegments[i].Text = converted
				}
			}
			scriptConverted = true
			logging.InfoContext(r.Context(), "Applied script conversion", "target_script", targetScript, "upload_id", uploadID)
		}

		// Update segments in database
		if err := database.UpdateSegments(uploadID, newSegments); err != nil {
			logging.ErrorContext(r.Context(), "Error updating segments", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to save aligned segments",
			})
			return
		}

		mode := "standard"
		if req.Mode == "lyrics" {
			mode = "lyrics"
		}
		logging.InfoContext(r.Context(), "Aligned transcript", "upload_id", uploadID, "mode", mode, "segment_count", len(newSegments), "match_rate", result.Stats.MatchRate*100)

		response := map[string]interface{}{
			"status":   "success",
			"segments": len(newSegments),
			"stats":    result.Stats,
			"mode":     mode,
		}
		if scriptConverted {
			response["script_converted"] = true
			response["target_script"] = string(targetScript)
		}
		json.NewEncoder(w).Encode(response)
	})

	// Download SRT file for a transcription
	mux.HandleFunc("GET /api/videos/{id}/subtitles.srt", func(w http.ResponseWriter, r *http.Request) {
		uploadID := r.PathValue("id")
		if uploadID == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Upload ID required",
			})
			return
		}

		transcription, err := database.GetTranscription(uploadID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting transcription", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to get transcription",
			})
			return
		}

		if transcription == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "No transcription found for this upload",
			})
			return
		}

		if transcription.Status != "complete" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error":  "Transcription not yet complete",
				"status": transcription.Status,
			})
			return
		}

		// Generate ETag from transcription ID + completion time + segments hash
		// Subtitles can change if edited or re-transcribed, so use CompletedAt
		var completedAt int64
		if transcription.CompletedAt != nil {
			completedAt = transcription.CompletedAt.Unix()
		}
		etag := generateETag(fmt.Sprintf("srt-%s-%d-%s", transcription.ID, completedAt, transcription.SegmentsJSON[:min(100, len(transcription.SegmentsJSON))]))

		// Check for conditional request (If-None-Match)
		if handleConditionalRequest(w, r, etag) {
			return // 304 Not Modified sent
		}

		segments, err := transcription.GetSegments()
		if err != nil || len(segments) == 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "No subtitle segments available",
			})
			return
		}

		// Convert to WhisperResult for SRT generation
		whisperResult := &WhisperResult{
			Language: transcription.Language,
			Duration: transcription.Duration,
			Text:     transcription.FullText,
			Segments: make([]WhisperSegment, len(segments)),
		}
		for i, s := range segments {
			whisperResult.Segments[i] = WhisperSegment{
				ID:    s.ID,
				Start: s.Start,
				End:   s.End,
				Text:  s.Text,
			}
		}

		// Generate SRT content
		srtContent := generateSRT(whisperResult)

		// Set caching headers - subtitles may be edited, so use shorter cache time
		// Cache for 10 minutes, must revalidate after that
		setCacheHeaders(w, etag, 600)

		// Set headers for file download
		// Use text/plain as it's universally supported by browsers for download
		// application/x-subrip is the registered MIME type but has limited browser support
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.srt\"", uploadID))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(srtContent))
	})

	// Download VTT file for a transcription
	mux.HandleFunc("GET /api/videos/{id}/subtitles.vtt", func(w http.ResponseWriter, r *http.Request) {
		uploadID := r.PathValue("id")
		if uploadID == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Upload ID required",
			})
			return
		}

		transcription, err := database.GetTranscription(uploadID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting transcription", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to get transcription",
			})
			return
		}

		if transcription == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "No transcription found for this upload",
			})
			return
		}

		if transcription.Status != "complete" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error":  "Transcription not yet complete",
				"status": transcription.Status,
			})
			return
		}

		// Generate ETag from transcription ID + completion time + segments hash
		var completedAt int64
		if transcription.CompletedAt != nil {
			completedAt = transcription.CompletedAt.Unix()
		}
		etag := generateETag(fmt.Sprintf("vtt-%s-%d-%s", transcription.ID, completedAt, transcription.SegmentsJSON[:min(100, len(transcription.SegmentsJSON))]))

		// Check for conditional request (If-None-Match)
		if handleConditionalRequest(w, r, etag) {
			return // 304 Not Modified sent
		}

		segments, err := transcription.GetSegments()
		if err != nil || len(segments) == 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "No subtitle segments available",
			})
			return
		}

		// Convert to WhisperResult for VTT generation
		whisperResult := &WhisperResult{
			Language: transcription.Language,
			Duration: transcription.Duration,
			Text:     transcription.FullText,
			Segments: make([]WhisperSegment, len(segments)),
		}
		for i, s := range segments {
			whisperResult.Segments[i] = WhisperSegment{
				ID:    s.ID,
				Start: s.Start,
				End:   s.End,
				Text:  s.Text,
			}
		}

		// Generate VTT content
		vttContent := generateVTT(whisperResult)

		// Set caching headers
		setCacheHeaders(w, etag, 600)

		// Set headers for file download
		// text/vtt is the official MIME type for WebVTT
		w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.vtt\"", uploadID))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(vttContent))
	})

	// Download JSON file for a transcription
	mux.HandleFunc("GET /api/videos/{id}/subtitles.json", func(w http.ResponseWriter, r *http.Request) {
		uploadID := r.PathValue("id")
		if uploadID == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Upload ID required",
			})
			return
		}

		transcription, err := database.GetTranscription(uploadID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting transcription", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to get transcription",
			})
			return
		}

		if transcription == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "No transcription found for this upload",
			})
			return
		}

		if transcription.Status != "complete" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error":  "Transcription not yet complete",
				"status": transcription.Status,
			})
			return
		}

		// Generate ETag from transcription ID + completion time + segments hash
		var completedAt int64
		if transcription.CompletedAt != nil {
			completedAt = transcription.CompletedAt.Unix()
		}
		etag := generateETag(fmt.Sprintf("json-%s-%d-%s", transcription.ID, completedAt, transcription.SegmentsJSON[:min(100, len(transcription.SegmentsJSON))]))

		// Check for conditional request (If-None-Match)
		if handleConditionalRequest(w, r, etag) {
			return // 304 Not Modified sent
		}

		segments, err := transcription.GetSegments()
		if err != nil || len(segments) == 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "No subtitle segments available",
			})
			return
		}

		// Return JSON with segments
		response := map[string]interface{}{
			"video_id":  uploadID,
			"language":  transcription.Language,
			"duration":  transcription.Duration,
			"full_text": transcription.FullText,
			"segments":  segments,
		}

		// Set caching headers
		setCacheHeaders(w, etag, 600)

		// Set headers for file download
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.json\"", uploadID))
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	})

	// List all videos
	mux.HandleFunc("GET /api/videos", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Get authenticated user (if any)
		token := auth.GetTokenFromRequest(r)
		user, _, _ := auth.ValidateSession(database, token)

		// Get optional session_id from query params (for anonymous user filtering)
		sessionID := r.URL.Query().Get("session_id")
		var sessionPtr *string
		if sessionID != "" {
			sessionPtr = &sessionID
		}

		// Get user_id from authenticated session
		var userPtr *string
		if user != nil {
			userPtr = &user.ID
		}

		// SECURITY: Require either authenticated user or session_id to filter videos
		// Without this check, anonymous requests would return ALL videos in the database
		if userPtr == nil && sessionPtr == nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Authentication or session_id required to list videos",
			})
			return
		}

		// Parse pagination parameters
		limit := 50 // default
		offset := 0
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
				limit = l
				if limit > 100 {
					limit = 100 // max limit
				}
			}
		}
		if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
			if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
				offset = o
			}
		}

		result, err := database.ListVideosPaginated(userPtr, sessionPtr, limit, offset)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error listing videos", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to list videos",
			})
			return
		}

		// Get transcription status and calculate expiry for each video
		type VideoWithStatus struct {
			db.Video
			TranscriptionStatus string     `json:"transcription_status"`
			ExpiresAt           *time.Time `json:"expires_at,omitempty"`
		}

		videos := make([]VideoWithStatus, len(result.Videos))
		for i, v := range result.Videos {
			videos[i] = VideoWithStatus{Video: v, TranscriptionStatus: "none"}
			if t, err := database.GetTranscription(v.ID); err == nil && t != nil {
				videos[i].TranscriptionStatus = t.Status
			}

			// Calculate expiration time based on user type
			// Anonymous: 48 hours, Registered: 90 days
			var expiresAt time.Time
			if v.UserID == nil {
				expiresAt = v.CreatedAt.Add(48 * time.Hour)
			} else {
				expiresAt = v.CreatedAt.Add(90 * 24 * time.Hour)
			}
			videos[i].ExpiresAt = &expiresAt
		}

		hasMore := offset+len(result.Videos) < result.TotalCount
		json.NewEncoder(w).Encode(map[string]interface{}{
			"videos":      videos,
			"total_count": result.TotalCount,
			"has_more":    hasMore,
		})
	})

	// Delete a video
	mux.HandleFunc("DELETE /api/videos/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		videoID := r.PathValue("id")
		if videoID == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Video ID required",
			})
			return
		}

		// Get the video to check ownership
		video, err := database.GetVideo(videoID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting video", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to get video",
			})
			return
		}
		if video == nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Video not found",
			})
			return
		}

		// Check ownership - either authenticated user owns it, or anonymous session matches
		token := auth.GetTokenFromRequest(r)
		user, _, _ := auth.ValidateSession(database, token)

		sessionID := r.URL.Query().Get("session_id")

		hasAccess := false
		if user != nil && video.UserID != nil && *video.UserID == user.ID {
			// Authenticated user owns the video
			hasAccess = true
		} else if sessionID != "" && video.SessionID != nil && *video.SessionID == sessionID {
			// Anonymous user with matching session
			hasAccess = true
		}

		if !hasAccess {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "You do not have permission to delete this video",
			})
			return
		}

		// Delete from database and get file paths
		deletedFiles, err := database.DeleteVideo(videoID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error deleting video from database", "video_id", videoID, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to delete video",
			})
			return
		}

		// Delete the video file from disk
		if deletedFiles != nil && deletedFiles.FilePath != "" {
			if err := os.Remove(deletedFiles.FilePath); err != nil {
				if !os.IsNotExist(err) {
					logging.ErrorContext(r.Context(), "Error deleting video file", "path", deletedFiles.FilePath, "error", err)
				}
			}
		}

		// Delete the thumbnail file from disk
		if deletedFiles != nil && deletedFiles.ThumbnailPath != nil && *deletedFiles.ThumbnailPath != "" {
			if err := os.Remove(*deletedFiles.ThumbnailPath); err != nil {
				if !os.IsNotExist(err) {
					logging.ErrorContext(r.Context(), "Error deleting thumbnail file", "path", *deletedFiles.ThumbnailPath, "error", err)
				}
			}
		}

		// Delete the burn output file from disk
		if deletedFiles != nil && deletedFiles.BurnOutputPath != nil && *deletedFiles.BurnOutputPath != "" {
			if err := os.Remove(*deletedFiles.BurnOutputPath); err != nil {
				if !os.IsNotExist(err) {
					logging.ErrorContext(r.Context(), "Error deleting burn output file", "path", *deletedFiles.BurnOutputPath, "error", err)
				}
			}
		}

		logging.InfoContext(r.Context(), "Video deleted", "video_id", videoID)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Video deleted successfully",
		})
	})

	// Reprocess a failed transcription
	mux.HandleFunc("POST /api/videos/{id}/reprocess", transcribeLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		uploadID := r.PathValue("id")
		if uploadID == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Video ID required",
			})
			return
		}

		// Get the video to check ownership
		video, err := database.GetVideo(uploadID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting video", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to get video",
			})
			return
		}
		if video == nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Video not found",
			})
			return
		}

		// Check ownership - either authenticated user owns it, or anonymous session matches
		token := auth.GetTokenFromRequest(r)
		user, _, _ := auth.ValidateSession(database, token)

		sessionID := r.URL.Query().Get("session_id")

		hasAccess := false
		if user != nil && video.UserID != nil && *video.UserID == user.ID {
			// Authenticated user owns the video
			hasAccess = true
		} else if sessionID != "" && video.SessionID != nil && *video.SessionID == sessionID {
			// Anonymous user with matching session
			hasAccess = true
		}

		if !hasAccess {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "You do not have permission to reprocess this video",
			})
			return
		}

		// Check if transcription is in error state
		transcription, err := database.GetTranscription(uploadID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting transcription", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to get transcription status",
			})
			return
		}

		if transcription == nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "No transcription found for this video",
			})
			return
		}

		if transcription.Status != "error" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error":  "Can only reprocess failed transcriptions",
				"status": transcription.Status,
			})
			return
		}

		// Use already-fetched video for file path and key version
		videoPath := video.FilePath
		keyVersion := video.KeyVersion

		// Update transcription status to processing
		database.UpdateTranscriptionStatus(uploadID, "processing", "Extracting audio...", 10)

		// Process in background (same logic as POST /api/transcribe/{id})
		go func(kv int) {
			// Panic recovery to prevent goroutine crashes from leaving jobs in stuck state
			defer func() {
				if r := recover(); r != nil {
					logging.Error("Panic in reprocess goroutine", "upload_id", uploadID, "panic", r)
					metrics.RecordTranscriptionFailed()
					database.FailTranscription(uploadID, "Internal error: reprocess crashed")
				}
			}()

			logging.Info("Reprocessing transcription", "upload_id", uploadID, "key_version", kv)

			// Decrypt video file if encrypted
			workingVideoPath := videoPath
			if strings.HasSuffix(videoPath, ".age") {
				database.UpdateTranscriptionStatus(uploadID, "processing", "Decrypting video...", 5)
				decryptedPath, err := multiEnc.DecryptToTempFile(videoPath, kv)
				if err != nil {
					logging.Error("Video decryption failed", "error", err)
					metrics.RecordTranscriptionFailed()
					database.FailTranscription(uploadID, fmt.Sprintf("Video decryption failed: %v", err))
					return
				}
				workingVideoPath = decryptedPath
				defer os.Remove(decryptedPath)
			}

			// Extract audio
			database.UpdateTranscriptionStatus(uploadID, "processing", "Extracting audio...", 10)
			audioPath := filepath.Join(uploadDir, uploadID+".wav")
			if err := audioExtractor.ExtractAudio(workingVideoPath, audioPath); err != nil {
				logging.Error("Audio extraction failed", "error", err)
				metrics.RecordTranscriptionFailed()
				database.FailTranscription(uploadID, fmt.Sprintf("Audio extraction failed: %v", err))
				return
			}

			database.UpdateTranscriptionStatus(uploadID, "processing", "Running transcription...", 30)

			// Start progress simulation goroutine
			progressDone := make(chan struct{})
			go func() {
				ticker := time.NewTicker(5 * time.Second)
				defer ticker.Stop()
				progress := 30
				for {
					select {
					case <-progressDone:
						return
					case <-ticker.C:
						if progress < 90 {
							progress += 5
							database.UpdateTranscriptionStatus(uploadID, "processing", "Transcribing audio...", progress)
						}
					}
				}
			}()

			// Run whisper (reprocess uses auto-detect)
			outputPath := filepath.Join(uploadDir, uploadID+"_transcript")
			result, err := transcribe(audioPath, outputPath, "auto")
			close(progressDone)

			if err != nil {
				logging.Error("Transcription failed", "error", err)
				metrics.RecordTranscriptionFailed()
				database.FailTranscription(uploadID, fmt.Sprintf("Transcription failed: %v", err))
				return
			}

			// Convert segments to database format
			segments := make([]db.Segment, len(result.Segments))
			for i, s := range result.Segments {
				segments[i] = db.Segment{
					ID:    s.ID,
					Start: s.Start,
					End:   s.End,
					Text:  s.Text,
				}
			}

			// Success
			logging.Info("Reprocessing complete", "upload_id", uploadID, "segment_count", len(result.Segments))
			if err := database.CompleteTranscription(uploadID, result.Language, result.Duration, result.Text, segments); err != nil {
				logging.Error("Error saving transcription result", "error", err)
			}

			// Clean up
			os.Remove(audioPath)
		}(keyVersion)

		json.NewEncoder(w).Encode(map[string]string{
			"status":  "processing",
			"message": "Reprocessing started",
		})
	}))

	// Serve uploaded video files for playback
	mux.HandleFunc("GET /api/videos/{id}/video", downloadLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		uploadID := r.PathValue("id")
		if uploadID == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Upload ID required",
			})
			return
		}

		// Get video metadata from database for ETag generation and decryption
		video, err := getVideoForDecryption(uploadID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Video not found for download", "error", err, "upload_id", uploadID)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": errmsg.ForVideoNotFound(err),
			})
			return
		}

		// Generate ETag from video ID + creation time + size
		// Video files don't change after upload, so this is stable
		etag := generateETag(fmt.Sprintf("%s-%d-%d", video.ID, video.CreatedAt.Unix(), video.Size))

		// Check for conditional request (If-None-Match)
		if handleConditionalRequest(w, r, etag) {
			return // 304 Not Modified sent
		}

		videoPath := video.FilePath

		// Validate that the file path is within the allowed upload directory
		// This prevents path traversal attacks if the database is compromised
		if err := pathValidator.ValidateAbsolutePath(videoPath); err != nil {
			logging.ErrorContext(r.Context(), "Path validation failed for video download", "error", err, "path", videoPath)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Access denied",
			})
			return
		}

		// If file is encrypted, decrypt to temp file for serving
		// (http.ServeFile needs seekable file for range requests)
		if strings.HasSuffix(videoPath, ".age") {
			decryptedPath, err := multiEnc.DecryptToTempFile(videoPath, video.KeyVersion)
			if err != nil {
				logging.ErrorContext(r.Context(), "Failed to decrypt video for serving", "error", err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "Failed to decrypt video",
				})
				return
			}
			defer os.Remove(decryptedPath)
			videoPath = decryptedPath
		}

		// Set caching headers before serving
		// Video files are immutable (don't change after upload), so can be cached
		// Cache for 1 hour, must revalidate after that
		if etag != "" {
			setCacheHeaders(w, etag, 3600) // 1 hour
		}

		// Audit log: file access
		sessionID := r.URL.Query().Get("session_id")
		var userID string
		if token := auth.GetTokenFromRequest(r); token != "" {
			if session, err := database.GetSessionByToken(token); err == nil && session != nil {
				userID = session.UserID
			}
		}
		logging.InfoContext(r.Context(), "File access: video download",
			"file_type", "video",
			"video_id", uploadID,
			"user_id", userID,
			"session_id", sessionID,
			"client_ip", ratelimit.GetClientIP(r),
		)

		// Serve the file
		http.ServeFile(w, r, videoPath)
	}))

	// Serve video thumbnail
	mux.HandleFunc("GET /api/videos/{id}/thumbnail", downloadLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		uploadID := r.PathValue("id")
		if uploadID == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Upload ID required",
			})
			return
		}

		// Get video from database to get thumbnail path
		video, err := database.GetVideo(uploadID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting video", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Database error",
			})
			return
		}
		if video == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Video not found",
			})
			return
		}

		// Check if thumbnail exists
		if video.ThumbnailPath == nil || *video.ThumbnailPath == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Thumbnail not available",
			})
			return
		}

		thumbPath := *video.ThumbnailPath

		// Validate that the thumbnail path is within the allowed upload directory
		// This prevents path traversal attacks if the database is compromised
		if err := pathValidator.ValidateAbsolutePath(thumbPath); err != nil {
			logging.ErrorContext(r.Context(), "Path validation failed for thumbnail download", "error", err, "path", thumbPath)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Access denied",
			})
			return
		}

		// Check if file exists on disk
		if _, err := os.Stat(thumbPath); os.IsNotExist(err) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Thumbnail file not found",
			})
			return
		}

		// Generate ETag from video ID + creation time (thumbnails are immutable)
		etag := generateETag(fmt.Sprintf("thumb-%s-%d", video.ID, video.CreatedAt.Unix()))

		// Check for conditional request (If-None-Match)
		if handleConditionalRequest(w, r, etag) {
			return // 304 Not Modified sent
		}

		// If file is encrypted, decrypt to temp file for serving
		// Thumbnail uses same key version as the video
		if strings.HasSuffix(thumbPath, ".age") {
			decryptedPath, err := multiEnc.DecryptToTempFile(thumbPath, video.KeyVersion)
			if err != nil {
				logging.ErrorContext(r.Context(), "Failed to decrypt thumbnail for serving", "error", err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "Failed to decrypt thumbnail",
				})
				return
			}
			defer os.Remove(decryptedPath)
			thumbPath = decryptedPath
		}

		// Set caching headers - thumbnails are immutable, cache for 24 hours
		setCacheHeaders(w, etag, 86400) // 24 hours

		// Set content type for JPEG
		w.Header().Set("Content-Type", "image/jpeg")

		// Audit log: file access
		sessionID := r.URL.Query().Get("session_id")
		var userID string
		if token := auth.GetTokenFromRequest(r); token != "" {
			if session, err := database.GetSessionByToken(token); err == nil && session != nil {
				userID = session.UserID
			}
		}
		logging.InfoContext(r.Context(), "File access: thumbnail download",
			"file_type", "thumbnail",
			"video_id", uploadID,
			"user_id", userID,
			"session_id", sessionID,
			"client_ip", ratelimit.GetClientIP(r),
		)

		// Serve the file
		http.ServeFile(w, r, thumbPath)
	}))

	// Start burning subtitles into video (rate limited: 2/min per IP)
	// Mode: "burn" (default) hardcodes subtitles into video frames
	//       "embed" creates soft subtitle track (much faster, no re-encoding)
	mux.HandleFunc("POST /api/videos/{id}/burn", burnLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		uploadID := r.PathValue("id")
		if uploadID == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Upload ID required",
			})
			return
		}

		// Parse and validate burn mode: "burn" (hardcode into video) or "embed" (soft subtitle track)
		// Default is "burn" for backwards compatibility
		burnModeParam := r.URL.Query().Get("mode")
		burnMode, err := validation.ValidateBurnMode(burnModeParam)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": err.Error(),
			})
			return
		}

		// Check if transcription exists and is complete
		transcription, err := database.GetTranscription(uploadID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting transcription", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to get transcription",
			})
			return
		}
		if transcription == nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "No transcription found - please transcribe the video first",
			})
			return
		}
		if transcription.Status != "complete" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error":  "Cannot burn subtitles - transcription not complete",
				"status": transcription.Status,
			})
			return
		}

		// Check if already processing
		existingJob, err := database.GetBurnJob(uploadID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting burn job", "error", err)
		}
		if existingJob != nil {
			if existingJob.Status == "processing" {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status":   "processing",
					"message":  existingJob.Message,
					"progress": existingJob.Progress,
				})
				return
			}
			if existingJob.Status == "complete" {
				json.NewEncoder(w).Encode(map[string]interface{}{
					"status":   "complete",
					"message":  existingJob.Message,
					"progress": 100,
				})
				return
			}
		}

		// Find the video file and get key version
		video, err := getVideoForDecryption(uploadID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Video not found for burn", "error", err, "upload_id", uploadID)
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": errmsg.ForVideoNotFound(err),
			})
			return
		}
		videoPath := video.FilePath
		keyVersion := video.KeyVersion

		// Create or update burn job
		if existingJob == nil {
			burnJobID, err := generateID()
			if err != nil {
				logging.ErrorContext(r.Context(), "Error generating burn job ID", "error", err)
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate burn job ID"})
				return
			}
			burnJob := &db.BurnJob{
				ID:        burnJobID,
				VideoID:   uploadID,
				Status:    "processing",
				Message:   "Starting subtitle burn...",
				Progress:  0,
				CreatedAt: time.Now(),
			}
			if err := database.CreateBurnJob(burnJob); err != nil {
				logging.ErrorContext(r.Context(), "Error creating burn job", "error", err)
			}
		} else {
			database.UpdateBurnJobStatus(uploadID, "processing", "Starting subtitle burn...", 0)
		}

		// Process in background (capture key version and burn mode)
		go func(kv int, mode string) {
			// Panic recovery to prevent goroutine crashes from leaving jobs in stuck state
			defer func() {
				if r := recover(); r != nil {
					logging.Error("Panic in burn goroutine", "upload_id", uploadID, "panic", r)
					database.FailBurnJob(uploadID, "Internal error: burn process crashed")
				}
			}()

			logging.Info("Starting subtitle burn", "upload_id", uploadID, "key_version", kv, "mode", mode)

			// Decrypt video if encrypted
			workingVideoPath := videoPath
			if strings.HasSuffix(videoPath, ".age") {
				database.UpdateBurnJobStatus(uploadID, "processing", "Decrypting video...", 5)
				decryptedPath, err := multiEnc.DecryptToTempFile(videoPath, kv)
				if err != nil {
					logging.Error("Video decryption failed", "error", err)
					database.FailBurnJob(uploadID, fmt.Sprintf("Video decryption failed: %v", err))
					return
				}
				workingVideoPath = decryptedPath
				defer os.Remove(decryptedPath)
			}

			// Get segments for SRT generation
			segments, err := transcription.GetSegments()
			if err != nil || len(segments) == 0 {
				logging.Error("No segments available", "error", err)
				database.FailBurnJob(uploadID, "No subtitle segments available")
				return
			}

			database.UpdateBurnJobStatus(uploadID, "processing", "Generating subtitles...", 10)

			// Convert to WhisperResult for SRT generation
			whisperResult := &WhisperResult{
				Language: transcription.Language,
				Duration: transcription.Duration,
				Text:     transcription.FullText,
				Segments: make([]WhisperSegment, len(segments)),
			}
			for i, s := range segments {
				whisperResult.Segments[i] = WhisperSegment{
					ID:    s.ID,
					Start: s.Start,
					End:   s.End,
					Text:  s.Text,
				}
			}

			// Write SRT to temp file
			srtContent := generateSRT(whisperResult)
			srtPath := filepath.Join(uploadDir, uploadID+"_burn.srt")
			if err := os.WriteFile(srtPath, []byte(srtContent), 0644); err != nil {
				logging.Error("Failed to write SRT file", "error", err)
				database.FailBurnJob(uploadID, fmt.Sprintf("Failed to write SRT file: %v", err))
				return
			}
			defer os.Remove(srtPath)

			progressMsg := "Burning subtitles into video..."
			if mode == "embed" {
				progressMsg = "Embedding subtitle track..."
			}
			database.UpdateBurnJobStatus(uploadID, "processing", progressMsg, 20)

			// Output to a temp file first, then encrypt
			outputPath := filepath.Join(uploadDir, uploadID+"_burned.mp4")

			var cmd *exec.Cmd
			if mode == "embed" {
				// Embed mode: Create soft subtitle track (much faster, no re-encoding)
				// Uses mov_text codec which is compatible with MP4/MOV containers
				// Subtitles can be toggled on/off by the player
				cmd = exec.Command("ffmpeg",
					"-i", workingVideoPath,
					"-i", srtPath,
					"-c:v", "copy", // Copy video stream (no re-encoding)
					"-c:a", "copy", // Copy audio stream (no re-encoding)
					"-c:s", "mov_text", // Embed subtitles as text track
					"-y",
					outputPath,
				)
			} else {
				// Burn mode: Hardcode subtitles into video frames (slower, re-encodes video)
				// Build subtitle style with optional font for Indic script support
				// FontName is added if SUBTITLE_FONT env var is set, enabling proper rendering
				// of Hindi, Tamil, Telugu and other scripts that require specific fonts
				subtitleStyle := "FontSize=24,PrimaryColour=&HFFFFFF,OutlineColour=&H000000,Outline=2"
				if subtitleFont != "" {
					subtitleStyle = fmt.Sprintf("FontName=%s,%s", subtitleFont, subtitleStyle)
				}

				cmd = exec.Command("ffmpeg",
					"-i", workingVideoPath,
					"-vf", fmt.Sprintf("subtitles='%s':force_style='%s'", srtPath, subtitleStyle),
					"-c:a", "copy",
					"-y",
					outputPath,
				)
			}

			// Start progress update goroutine for burn operation
			// FFmpeg doesn't provide progress callbacks, so we simulate progress
			// by incrementing from 20% to 85% in steps based on video duration
			burnProgressDone := make(chan struct{})
			go func() {
				// Use video duration to estimate tick interval
				// Shorter videos = shorter intervals, longer videos = longer intervals
				// Embed mode is much faster (no re-encoding), burn mode takes ~1x video duration
				estimatedBurnTime := transcription.Duration
				if mode == "embed" {
					// Embed mode is fast - just copying streams plus adding subtitle track
					// Estimate ~5-10 seconds for most videos
					estimatedBurnTime = 10
				}
				if estimatedBurnTime < 10 {
					estimatedBurnTime = 10 // Minimum 10 seconds
				}
				if estimatedBurnTime > 600 {
					estimatedBurnTime = 600 // Cap at 10 minutes
				}

				// Calculate tick interval to go from 20% to 85% (65 points) during burn
				numTicks := 13 // 65 / 5 = 13 updates of 5% each
				tickInterval := time.Duration(estimatedBurnTime/float64(numTicks)) * time.Second
				if tickInterval < time.Second {
					tickInterval = time.Second
				}

				ticker := time.NewTicker(tickInterval)
				defer ticker.Stop()
				progress := 20
				statusMsg := progressMsg
				for {
					select {
					case <-burnProgressDone:
						return
					case <-ticker.C:
						if progress < 85 {
							progress += 5
							database.UpdateBurnJobStatus(uploadID, "processing", statusMsg, progress)
						}
					}
				}
			}()

			cmdOutput, err := cmd.CombinedOutput()
			close(burnProgressDone) // Stop progress updates

			if err != nil {
				logging.Error("ffmpeg burn subtitles failed", "error", err, "output", string(cmdOutput))
				database.FailBurnJob(uploadID, fmt.Sprintf("Failed to burn subtitles: %v", err))
				return
			}

			database.UpdateBurnJobStatus(uploadID, "processing", "Encrypting output...", 90)

			// Encrypt the output file with current key version
			encOutputPath, keyVersion, err := multiEnc.EncryptFile(outputPath)
			if err != nil {
				logging.Error("Failed to encrypt burned video", "error", err)
				os.Remove(outputPath)
				database.FailBurnJob(uploadID, fmt.Sprintf("Failed to encrypt output: %v", err))
				return
			}
			os.Remove(outputPath) // Remove unencrypted file

			logging.Info("Subtitle burn complete", "upload_id", uploadID, "output_path", encOutputPath, "key_version", keyVersion, "mode", mode)
			database.CompleteBurnJobWithKeyVersion(uploadID, encOutputPath, keyVersion)
		}(keyVersion, string(burnMode))

		// Return immediately with processing status
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   "processing",
			"message":  "Subtitle burn started",
			"progress": 0,
		})
	}))

	// Get burn job status
	mux.HandleFunc("GET /api/videos/{id}/burn", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		uploadID := r.PathValue("id")
		if uploadID == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Upload ID required",
			})
			return
		}

		job, err := database.GetBurnJob(uploadID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting burn job", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to get burn job status",
			})
			return
		}

		if job == nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "No burn job found for this video",
			})
			return
		}

		// Build response with progress info
		response := map[string]interface{}{
			"status":   job.Status,
			"message":  job.Message,
			"progress": job.Progress,
		}

		// If processing, calculate estimated time remaining
		if job.Status == "processing" && job.Progress > 0 {
			// Get transcription for duration info
			transcription, _ := database.GetTranscription(uploadID)
			if transcription != nil && transcription.Duration > 0 {
				response["duration"] = transcription.Duration

				// Calculate elapsed time since job started
				elapsed := time.Since(job.CreatedAt).Seconds()
				if elapsed > 0 && job.Progress > 0 {
					// Estimate total time based on current progress
					// progress% complete took elapsed seconds, so 100% will take:
					estimatedTotal := elapsed * 100 / float64(job.Progress)
					estimatedRemaining := estimatedTotal - elapsed
					if estimatedRemaining < 0 {
						estimatedRemaining = 0
					}
					response["estimated_remaining_seconds"] = int(estimatedRemaining)
				}
			}
		}

		json.NewEncoder(w).Encode(response)
	})

	// Download burned video (rate limited: 30/min per IP)
	mux.HandleFunc("GET /api/videos/{id}/burned", downloadLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		uploadID := r.PathValue("id")
		if uploadID == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Upload ID required",
			})
			return
		}

		job, err := database.GetBurnJob(uploadID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting burn job", "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to get burn job",
			})
			return
		}

		if job == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "No burn job found - start one first with POST /api/videos/{id}/burn",
			})
			return
		}

		if job.Status != "complete" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":    "Burn job not complete",
				"status":   job.Status,
				"message":  job.Message,
				"progress": job.Progress,
			})
			return
		}

		if job.OutputPath == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Burned video file not found",
			})
			return
		}

		// Validate that the burned output path is within the allowed upload directory
		// This prevents path traversal attacks if the database is compromised
		if err := pathValidator.ValidateAbsolutePath(job.OutputPath); err != nil {
			logging.ErrorContext(r.Context(), "Path validation failed for burned video download", "error", err, "path", job.OutputPath)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Access denied",
			})
			return
		}

		// Decrypt if encrypted
		servePath := job.OutputPath
		if strings.HasSuffix(job.OutputPath, ".age") {
			// Use burn job's output key version (defaults to 1 for older jobs)
			outputKeyVersion := 1
			if job.OutputKeyVersion != nil {
				outputKeyVersion = *job.OutputKeyVersion
			}
			decryptedPath, err := multiEnc.DecryptToTempFile(job.OutputPath, outputKeyVersion)
			if err != nil {
				logging.ErrorContext(r.Context(), "Failed to decrypt burned video", "error", err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "Failed to decrypt video",
				})
				return
			}
			defer os.Remove(decryptedPath)
			servePath = decryptedPath
		}

		// Get original video filename for download name
		video, err := database.GetVideo(uploadID)
		downloadName := uploadID + "_subtitled.mp4"
		if err == nil && video != nil {
			// Use original filename with _subtitled suffix
			ext := filepath.Ext(video.Filename)
			baseName := strings.TrimSuffix(video.Filename, ext)
			downloadName = baseName + "_subtitled.mp4"
		}

		// Audit log: file access
		sessionID := r.URL.Query().Get("session_id")
		var userID string
		if token := auth.GetTokenFromRequest(r); token != "" {
			if session, err := database.GetSessionByToken(token); err == nil && session != nil {
				userID = session.UserID
			}
		}
		logging.InfoContext(r.Context(), "File access: burned video download",
			"file_type", "burned_video",
			"video_id", uploadID,
			"user_id", userID,
			"session_id", sessionID,
			"client_ip", ratelimit.GetClientIP(r),
		)

		// Set headers for download
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", downloadName))
		http.ServeFile(w, r, servePath)
	}))

	// Script detection endpoint (rate limited)
	mux.HandleFunc("POST /api/text/detect-script", scriptLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var req struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
			return
		}

		if len(req.Text) > 10240 { // 10KB limit
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Text too long (max 10KB)"})
			return
		}

		detectedScript := script.DetectScript(req.Text)
		detectedLang := script.DetectLanguageFromRomanized(req.Text)

		// Calculate confidence based on character count
		confidence := 0.0
		if detectedScript != script.ScriptUnknown {
			// Simple confidence: more characters = higher confidence
			confidence = math.Min(float64(len(req.Text))/100.0, 1.0)
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"detected_script":   string(detectedScript),
			"detected_language": detectedLang,
			"confidence":        confidence,
		})
	}))

	// Script conversion endpoint (rate limited)
	mux.HandleFunc("POST /api/text/convert", scriptLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var req struct {
			Text         string `json:"text"`
			SourceScript string `json:"source_script,omitempty"` // Optional, auto-detected if omitted
			TargetScript string `json:"target_script"`
			Language     string `json:"language"` // Required for romanized input
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
			return
		}

		// Validate text length (10KB limit)
		if len(req.Text) > 10240 {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "Text too long (max 10KB)"})
			return
		}

		// Validate language
		if !script.IsLanguageSupported(req.Language) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":               "Unsupported language",
				"supported_languages": script.SupportedLanguages(),
			})
			return
		}

		// Validate target script
		targetScript := script.Script(req.TargetScript)
		if !script.IsScriptSupported(targetScript) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":             "Unsupported target script",
				"supported_scripts": script.SupportedScripts(),
			})
			return
		}

		// Auto-detect source script if not provided
		sourceScript := script.Script(req.SourceScript)
		if sourceScript == "" {
			sourceScript = script.DetectScript(req.Text)
		}

		// Perform conversion
		converter := script.NewConverter()
		converted, err := converter.Convert(req.Text, req.Language, targetScript)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Conversion failed: %v", err)})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"original":      req.Text,
			"converted":     converted,
			"source_script": string(sourceScript),
			"target_script": string(targetScript),
			"language":      req.Language,
		})
	}))

	// Start the cleanup scheduler for expired videos
	go startCleanupScheduler()

	// Start the database maintenance scheduler (VACUUM + ANALYZE)
	go startMaintenanceScheduler()

	logging.Info("Backend server starting", "port", port, "whisper_model", getWhisperModel())

	// Wrap mux with middleware chain (outermost runs first):
	// 1. Security headers - add CSP and other security headers
	// 2. Request ID - add X-Request-ID for tracing
	// 3. CSRF - validate CSRF tokens on state-changing requests
	// 4. Per-user rate limiting - applies to authenticated users
	// 5. Metrics - record request metrics for Prometheus
	csrfMiddleware := csrf.Middleware(auth.GetTokenFromRequest)
	handler := metrics.MetricsMiddleware(securityHeadersMiddleware(requestIDMiddleware(userRateLimitMiddleware(csrfMiddleware(mux)))))

	if err := http.ListenAndServe(":"+port, handler); err != nil {
		logging.Fatal("Failed to start server", "error", err)
	}
}

// startCleanupScheduler runs periodic cleanup of expired videos.
// Anonymous videos are deleted after 48 hours, registered user videos after 90 days.
func startCleanupScheduler() {
	// Run cleanup immediately on startup, then every hour
	runCleanup()

	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		runCleanup()
	}
}

// runCleanup deletes expired videos and their associated files
func runCleanup() {
	logging.Info("Running cleanup for expired videos")

	expiredVideos, err := database.GetExpiredVideos()
	if err != nil {
		logging.Error("Error getting expired videos", "error", err)
		return
	}

	if len(expiredVideos) == 0 {
		logging.Debug("No expired videos to clean up")
		return
	}

	logging.Info("Found expired videos to clean up", "count", len(expiredVideos))

	deletedCount := 0
	for _, video := range expiredVideos {
		// Delete from database and get file paths
		deletedFiles, err := database.DeleteVideo(video.ID)
		if err != nil {
			logging.Error("Error deleting video from database", "video_id", video.ID, "error", err)
			continue
		}

		// Delete the video file from disk
		if deletedFiles != nil && deletedFiles.FilePath != "" {
			if err := os.Remove(deletedFiles.FilePath); err != nil {
				if !os.IsNotExist(err) {
					logging.Error("Error deleting video file", "path", deletedFiles.FilePath, "error", err)
				}
			} else {
				logging.Debug("Deleted video file", "path", deletedFiles.FilePath)
			}
		}

		// Delete the thumbnail file from disk
		if deletedFiles != nil && deletedFiles.ThumbnailPath != nil && *deletedFiles.ThumbnailPath != "" {
			if err := os.Remove(*deletedFiles.ThumbnailPath); err != nil {
				if !os.IsNotExist(err) {
					logging.Error("Error deleting thumbnail file", "path", *deletedFiles.ThumbnailPath, "error", err)
				}
			} else {
				logging.Debug("Deleted thumbnail file", "path", *deletedFiles.ThumbnailPath)
			}
		}

		// Delete the burn output file from disk
		if deletedFiles != nil && deletedFiles.BurnOutputPath != nil && *deletedFiles.BurnOutputPath != "" {
			if err := os.Remove(*deletedFiles.BurnOutputPath); err != nil {
				if !os.IsNotExist(err) {
					logging.Error("Error deleting burn output file", "path", *deletedFiles.BurnOutputPath, "error", err)
				}
			} else {
				logging.Debug("Deleted burn output file", "path", *deletedFiles.BurnOutputPath)
			}
		}

		deletedCount++
		logging.Info("Cleaned up expired video",
			"video_id", video.ID,
			"has_user", video.UserID != nil,
			"created_at", video.CreatedAt.Format(time.RFC3339))
	}

	// Also clean up expired sessions
	sessionCount, err := database.DeleteExpiredSessions()
	if err != nil {
		logging.Error("Error deleting expired sessions", "error", err)
	} else if sessionCount > 0 {
		logging.Info("Deleted expired sessions", "count", sessionCount)
	}

	// Clean up old login attempts (older than 1 hour to be safe)
	loginAttemptCount, err := database.DeleteExpiredLoginAttempts(time.Now().Add(-1 * time.Hour))
	if err != nil {
		logging.Error("Error deleting expired login attempts", "error", err)
	} else if loginAttemptCount > 0 {
		logging.Info("Deleted expired login attempts", "count", loginAttemptCount)
	}

	// Clean up expired upload sessions
	expiredSessions, err := database.GetExpiredUploadSessions()
	if err != nil {
		logging.Error("Error getting expired upload sessions", "error", err)
	} else if len(expiredSessions) > 0 {
		sessionDeleteCount := 0
		for _, session := range expiredSessions {
			chunkPaths, err := database.DeleteUploadSession(session.ID)
			if err != nil {
				logging.Error("Error deleting upload session", "session_id", session.ID, "error", err)
				continue
			}
			// Delete chunk files
			for _, path := range chunkPaths {
				os.Remove(path)
			}
			// Try to remove the chunks directory
			chunksDir := filepath.Join(uploadDir, "chunks", session.ID)
			os.Remove(chunksDir)
			sessionDeleteCount++
		}
		if sessionDeleteCount > 0 {
			logging.Info("Deleted expired upload sessions", "count", sessionDeleteCount)
		}
	}

	// Clean up orphan chunk directories (exist on disk but not in database)
	orphanCleanupCount := cleanupOrphanChunkDirectories()
	if orphanCleanupCount > 0 {
		logging.Info("Deleted orphan chunk directories", "count", orphanCleanupCount)
	}

	logging.Info("Cleanup complete", "videos_deleted", deletedCount)
}

// cleanupOrphanChunkDirectories removes chunk directories that exist on disk
// but don't have corresponding records in the upload_sessions table.
// This handles edge cases like server crashes during upload or manual database cleanup.
func cleanupOrphanChunkDirectories() int {
	return cleanupOrphanChunkDirectoriesWithDB(database, uploadDir)
}

// cleanupOrphanChunkDirectoriesWithDB is the testable implementation of orphan cleanup.
// It scans the chunks directory for directories that don't have matching database records.
func cleanupOrphanChunkDirectoriesWithDB(database *db.DB, baseUploadDir string) int {
	chunksBaseDir := filepath.Join(baseUploadDir, "chunks")

	// Check if chunks directory exists
	entries, err := os.ReadDir(chunksBaseDir)
	if err != nil {
		if !os.IsNotExist(err) {
			logging.Error("Error reading chunks directory", "path", chunksBaseDir, "error", err)
		}
		return 0
	}

	orphanCount := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		sessionID := entry.Name()
		exists, err := database.UploadSessionExists(sessionID)
		if err != nil {
			logging.Error("Error checking upload session existence", "session_id", sessionID, "error", err)
			continue
		}

		if !exists {
			// This is an orphan directory - remove it and its contents
			orphanDir := filepath.Join(chunksBaseDir, sessionID)
			if err := os.RemoveAll(orphanDir); err != nil {
				logging.Error("Error removing orphan chunk directory", "path", orphanDir, "error", err)
			} else {
				logging.Debug("Removed orphan chunk directory", "session_id", sessionID)
				orphanCount++
			}
		}
	}

	return orphanCount
}

// startMaintenanceScheduler runs periodic database maintenance (VACUUM and ANALYZE).
// Set DB_MAINTENANCE_INTERVAL environment variable to configure interval (default: 24h).
// Set to "0" or "disabled" to disable maintenance.
func startMaintenanceScheduler() {
	if dbMaintenanceInterval <= 0 {
		logging.Info("Database maintenance scheduler disabled")
		return
	}

	logging.Info("Database maintenance scheduler started", "interval", dbMaintenanceInterval)

	ticker := time.NewTicker(dbMaintenanceInterval)
	defer ticker.Stop()

	for range ticker.C {
		runDatabaseMaintenance()
	}
}

// runDatabaseMaintenance performs VACUUM and ANALYZE on the SQLite database.
func runDatabaseMaintenance() {
	logging.Info("Running database maintenance (VACUUM + ANALYZE)")
	startTime := time.Now()

	if err := database.Maintenance(); err != nil {
		logging.Error("Database maintenance failed", "error", err)
		return
	}

	duration := time.Since(startTime)
	logging.Info("Database maintenance complete", "duration", duration)
}
