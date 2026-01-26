package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/trevor/subtitler/backend/align"
	"github.com/trevor/subtitler/backend/audio"
	"github.com/trevor/subtitler/backend/auth"
	"github.com/trevor/subtitler/backend/crypto"
	"github.com/trevor/subtitler/backend/csrf"
	"github.com/trevor/subtitler/backend/db"
	"github.com/trevor/subtitler/backend/email"
	"github.com/trevor/subtitler/backend/ratelimit"
	"github.com/trevor/subtitler/backend/script"
	"github.com/trevor/subtitler/backend/totp"
)

const (
	maxUploadSize = 500 << 20 // 500 MB
	uploadDir     = "uploads"
	dbPath        = "data/subtitler.db"
	keyPath       = "data/age.key"
)

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

// Global encryptor for file encryption
var encryptor *crypto.Encryptor

// Global rate limiters (per IP, per minute)
var authLimiter = ratelimit.New(5, time.Minute)             // Auth endpoints: 5/min
var passwordResetLimiter = ratelimit.New(3, 15*time.Minute) // Password reset: 3/15min (stricter)
var uploadLimiter = ratelimit.New(10, time.Minute)          // Upload endpoint: 10/min
var transcribeLimiter = ratelimit.New(5, time.Minute)       // Transcribe endpoints: 5/min
var burnLimiter = ratelimit.New(2, time.Minute)             // Burn endpoint: 2/min

// Global email service for transactional emails
var emailService email.EmailService

// Global audio extractor for video processing
var audioExtractor audio.Extractor

// generateRequestID creates a unique request ID for tracing
func generateRequestID() string {
	b := make([]byte, 8) // 16 hex chars
	rand.Read(b)
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

		// Log the request with its ID
		log.Printf("[%s] %s %s", requestID, r.Method, r.URL.Path)

		// Call the next handler
		next.ServeHTTP(w, r)
	})
}

func generateID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
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

// transcribeAudio runs whisper-cli on the audio file
func transcribeAudio(audioPath, outputPath string) (*WhisperResult, error) {
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
		"-t", "4", // 4 threads
		"-l", "auto", // auto-detect language
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

// transcribeAudioServer sends audio to whisper-server HTTP API
func transcribeAudioServer(audioPath string) (*WhisperResult, error) {
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
	writer.WriteField("temperature", "0.0")
	writer.WriteField("language", "auto")

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

		// Send request (with long timeout for transcription)
		client := &http.Client{Timeout: 30 * time.Minute}
		resp, err = client.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxRetries-1 {
				log.Printf("whisper-server request failed (attempt %d/%d): %v, retrying in %v", attempt+1, maxRetries, err, retryDelays[attempt])
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
				log.Printf("failed to read response (attempt %d/%d): %v, retrying in %v", attempt+1, maxRetries, err, retryDelays[attempt])
				time.Sleep(retryDelays[attempt])
				continue
			}
			return nil, fmt.Errorf("failed to read response after %d attempts: %v", maxRetries, err)
		}

		// Check for server errors (5xx) that warrant a retry
		if resp.StatusCode >= 500 && resp.StatusCode < 600 {
			lastErr = fmt.Errorf("server error (status %d): %s", resp.StatusCode, string(respBody))
			if attempt < maxRetries-1 {
				log.Printf("whisper-server returned %d (attempt %d/%d): %s, retrying in %v", resp.StatusCode, attempt+1, maxRetries, string(respBody), retryDelays[attempt])
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

// transcribe sends audio for transcription, using server if enabled, otherwise CLI
func transcribe(audioPath, outputPath string) (*WhisperResult, error) {
	if isWhisperServerEnabled() {
		log.Printf("Using whisper-server at %s", getWhisperServerURL())
		return transcribeAudioServer(audioPath)
	}
	log.Printf("Using whisper-cli with model %s", getWhisperModel())
	return transcribeAudio(audioPath, outputPath)
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
		segments, _ := t.GetSegments()
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
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Ensure upload directory exists
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Fatalf("Failed to create upload directory: %v", err)
	}

	// Initialize database
	var err error
	database, err = db.Open(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()
	log.Printf("Database initialized at %s", dbPath)

	// Initialize encryptor for file encryption at rest
	var isNewKey bool
	encryptor, isNewKey, err = crypto.LoadOrGenerateKey(keyPath)
	if err != nil {
		log.Fatalf("Failed to initialize encryption: %v", err)
	}
	if isNewKey {
		log.Printf("Generated new encryption key, saved to %s", keyPath)
	} else {
		log.Printf("Loaded encryption key from %s", keyPath)
	}
	log.Printf("Public key: %s", encryptor.PublicKey())

	// Initialize email service
	emailService = email.NewResendService()
	if emailService.IsEnabled() {
		log.Printf("Email service enabled")
	} else {
		log.Printf("Email service disabled (no RESEND_API_KEY set)")
	}

	// Initialize audio extractor and check ffmpeg availability
	audioExtractor = audio.NewFFmpegExtractor()
	if err := audio.CheckFFmpegAvailable(); err != nil {
		log.Printf("WARNING: %v", err)
		log.Printf("Video transcription and subtitle burning will fail until ffmpeg is installed")
	} else {
		log.Printf("ffmpeg available for video processing")
	}

	mux := http.NewServeMux()

	// Health check endpoint
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

		json.NewEncoder(w).Encode(status)
	})

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

		// Format the log message with source info
		var prefix string
		switch req.Level {
		case "error":
			prefix = "[FRONTEND ERROR]"
		case "warn":
			prefix = "[FRONTEND WARN]"
		case "debug":
			prefix = "[FRONTEND DEBUG]"
		case "info":
			prefix = "[FRONTEND INFO]"
		default:
			prefix = "[FRONTEND]"
		}

		// Build log message
		logMsg := fmt.Sprintf("%s %s", prefix, req.Message)
		if req.URL != "" {
			logMsg = fmt.Sprintf("%s (from %s", logMsg, req.URL)
			if req.Line > 0 {
				logMsg = fmt.Sprintf("%s:%d", logMsg, req.Line)
				if req.Column > 0 {
					logMsg = fmt.Sprintf("%s:%d", logMsg, req.Column)
				}
			}
			logMsg = logMsg + ")"
		}

		// Log to backend console
		log.Println(logMsg)

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
		})
	})

	// Auth: Register new user (rate limited)
	mux.HandleFunc("POST /api/auth/register", authLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Parse request body
		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
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
			log.Printf("Error checking existing user: %v", err)
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
			log.Printf("Error hashing password: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Registration failed",
			})
			return
		}

		// Generate user ID
		userID, err := auth.GenerateID()
		if err != nil {
			log.Printf("Error generating user ID: %v", err)
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
			log.Printf("Error creating user: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Registration failed",
			})
			return
		}

		// Generate email verification token (32 bytes = 256 bits entropy)
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			log.Printf("Error generating verification token: %v", err)
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
			log.Printf("Error creating verification token: %v", err)
		}

		// Send verification email
		ctx := r.Context()
		if err := emailService.SendEmailVerification(ctx, user.Email, token); err != nil {
			log.Printf("Error sending verification email: %v", err)
		}

		log.Printf("New user registered (pending verification): %s", user.Email)
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
			Email    string `json:"email"`
			Password string `json:"password"`
			TOTPCode string `json:"totp_code,omitempty"` // Required if 2FA is enabled
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

		// Check if email is locked due to too many failed attempts
		locked, unlockTime, err := database.IsEmailLocked(req.Email, maxLoginAttempts, loginLockDuration)
		if err != nil {
			log.Printf("Error checking email lock: %v", err)
		}
		if locked {
			remainingMins := int(time.Until(unlockTime).Minutes()) + 1
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
				log.Printf("Error recording login attempt: %v", err)
			}
		}

		// Get user by email
		user, err := database.GetUserByEmail(req.Email)
		if err != nil {
			log.Printf("Error getting user: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Login failed",
			})
			return
		}
		if user == nil {
			recordFailure()
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid email or password",
			})
			return
		}

		// Check password
		if !auth.CheckPassword(req.Password, user.PasswordHash) {
			recordFailure()
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid email or password",
			})
			return
		}

		// Check if email is verified
		if !user.EmailVerified {
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
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "Invalid 2FA code",
				})
				return
			}
		}

		// Login successful - clear failed attempts for this email
		if err := database.ClearLoginAttempts(req.Email); err != nil {
			log.Printf("Error clearing login attempts: %v", err)
		}

		// Create session with IP and user agent
		userAgent := r.Header.Get("User-Agent")
		session, err := auth.CreateSession(database, user.ID, clientIP, userAgent)
		if err != nil {
			log.Printf("Error creating session: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Login failed",
			})
			return
		}

		// Set session cookie
		auth.SetSessionCookie(w, session.Token, session.ExpiresAt)

		log.Printf("User logged in: %s", user.Email)
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

		token := auth.GetTokenFromRequest(r)
		if token != "" {
			if err := database.DeleteSession(token); err != nil {
				log.Printf("Error deleting session: %v", err)
			}
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
			log.Printf("Error validating session: %v", err)
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
			log.Printf("Error validating session: %v", err)
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
			log.Printf("Error validating session: %v", err)
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
			log.Printf("Error getting sessions: %v", err)
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
			log.Printf("Error validating session: %v", err)
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
			log.Printf("Error deleting session: %v", err)
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Session not found",
			})
			return
		}

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
			log.Printf("Error generating TOTP secret: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to generate secret",
			})
			return
		}

		// Save the secret to the database (not yet enabled)
		if err := database.SetTOTPSecret(user.ID, secret); err != nil {
			log.Printf("Error saving TOTP secret: %v", err)
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
			log.Printf("Error generating QR code: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to generate QR code",
			})
			return
		}

		log.Printf("TOTP setup initiated for user: %s", user.Email)
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

		// Validate the code
		if !totp.Validate(*user.TOTPSecret, req.Code) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid code. Please try again.",
			})
			return
		}

		// Generate recovery codes
		recoveryCodes, err := totp.GenerateRecoveryCodes(totp.NumCodes)
		if err != nil {
			log.Printf("Error generating recovery codes: %v", err)
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
				log.Printf("Error hashing recovery code: %v", err)
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
			log.Printf("Error enabling TOTP with recovery codes: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to enable 2FA",
			})
			return
		}

		log.Printf("2FA enabled for user: %s with %d recovery codes", user.Email, len(recoveryCodes))
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

		// Disable 2FA
		if err := database.DisableTOTP(user.ID); err != nil {
			log.Printf("Error disabling TOTP: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to disable 2FA",
			})
			return
		}

		// Delete recovery codes
		if err := database.DeleteRecoveryCodes(user.ID); err != nil {
			log.Printf("Error deleting recovery codes: %v", err)
			// Continue - 2FA is disabled even if codes couldn't be deleted
		}

		log.Printf("2FA disabled for user: %s", user.Email)
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

		// Get user by email
		user, err := database.GetUserByEmail(req.Email)
		if err != nil {
			log.Printf("Error getting user: %v", err)
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
			log.Printf("Error getting recovery codes: %v", err)
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

		if matchedCodeID == "" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid recovery code",
			})
			return
		}

		// Mark the code as used
		success, err := database.UseRecoveryCode(matchedCodeID)
		if err != nil || !success {
			log.Printf("Error using recovery code: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to use recovery code",
			})
			return
		}

		// Disable 2FA, delete recovery codes, and clear sessions in a single transaction
		if err := database.DisableTOTPAndClearSessions(user.ID); err != nil {
			log.Printf("Error disabling TOTP and clearing sessions: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to disable 2FA",
			})
			return
		}

		// Create a new session with IP and user agent
		clientIP := ratelimit.GetClientIP(r)
		userAgent := r.Header.Get("User-Agent")
		session, err := auth.CreateSession(database, user.ID, clientIP, userAgent)
		if err != nil {
			log.Printf("Error creating session: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to create session",
			})
			return
		}

		// Set session cookie
		auth.SetSessionCookie(w, session.Token, session.ExpiresAt)

		log.Printf("2FA disabled via recovery code for user: %s", user.Email)
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
			log.Printf("Error generating recovery codes: %v", err)
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
				log.Printf("Error hashing recovery code: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "Failed to generate recovery codes",
				})
				return
			}
			codeHashes[i] = hash
		}

		if err := database.SaveRecoveryCodes(user.ID, codeHashes); err != nil {
			log.Printf("Error saving recovery codes: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to save recovery codes",
			})
			return
		}

		log.Printf("Regenerated recovery codes for user: %s", user.Email)
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
		user, err := database.GetUserByEmail(req.Email)
		if err != nil {
			log.Printf("Error looking up user for password reset: %v", err)
			return
		}
		if user == nil {
			// User doesn't exist - return success anyway
			log.Printf("Password reset requested for non-existent email: %s", req.Email)
			return
		}

		// Generate reset token (32 bytes = 256 bits entropy)
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			log.Printf("Error generating reset token: %v", err)
			return
		}
		token := hex.EncodeToString(tokenBytes)
		tokenHash := email.HashToken(token)

		// Create reset token with 1-hour expiry
		expiresAt := time.Now().Add(1 * time.Hour)
		_, err = database.CreatePasswordResetToken(user.ID, tokenHash, expiresAt)
		if err != nil {
			log.Printf("Error creating password reset token: %v", err)
			return
		}

		// Send password reset email
		ctx := r.Context()
		if err := emailService.SendPasswordReset(ctx, user.Email, token); err != nil {
			log.Printf("Error sending password reset email: %v", err)
			// Still return success to prevent enumeration
			return
		}

		log.Printf("Password reset email sent to: %s", user.Email)
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
			log.Printf("Error looking up reset token: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to process reset request",
			})
			return
		}

		// Check if token exists and is valid
		if resetToken == nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid or expired reset token",
			})
			return
		}

		// Check if token is used or expired
		if resetToken.Used || time.Now().After(resetToken.ExpiresAt) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid or expired reset token",
			})
			return
		}

		// Hash the new password
		passwordHash, err := auth.HashPassword(req.Password)
		if err != nil {
			log.Printf("Error hashing new password: %v", err)
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
			log.Printf("Error completing password reset: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to update password",
			})
			return
		}

		log.Printf("Password reset successful for user: %s", resetToken.UserID)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Password has been reset successfully. Please log in with your new password.",
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
			log.Printf("Error looking up verification token: %v", err)
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
			log.Printf("Error verifying email: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to verify email",
			})
			return
		}

		log.Printf("Email verified for user: %s", verifyToken.UserID)
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
			log.Printf("Error looking up user for resend verification: %v", err)
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
			log.Printf("Error generating verification token: %v", err)
			return
		}
		token := hex.EncodeToString(tokenBytes)
		tokenHash := email.HashToken(token)

		// Create verification token with 24-hour expiry
		expiresAt := time.Now().Add(24 * time.Hour)
		_, err = database.CreateEmailVerificationToken(user.ID, tokenHash, expiresAt)
		if err != nil {
			log.Printf("Error creating verification token: %v", err)
			return
		}

		// Send verification email
		ctx := r.Context()
		if err := emailService.SendEmailVerification(ctx, user.Email, token); err != nil {
			log.Printf("Error sending verification email: %v", err)
			return
		}

		log.Printf("Verification email resent to: %s", user.Email)
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
				log.Printf("Error counting videos for session: %v", err)
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
			log.Printf("Error parsing form: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "File too large or invalid form data",
			})
			return
		}

		// Get the file from the form
		file, header, err := r.FormFile("video")
		if err != nil {
			log.Printf("Error getting form file: %v", err)
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

		// Generate unique ID for this upload
		uploadID := generateID()

		// Get file extension from original filename
		ext := filepath.Ext(header.Filename)
		if ext == "" {
			ext = ".mp4" // default extension
		}

		// Create destination file
		destPath := filepath.Join(uploadDir, uploadID+ext)
		destFile, err := os.Create(destPath)
		if err != nil {
			log.Printf("Error creating destination file: %v", err)
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
			log.Printf("Error copying file: %v", err)
			os.Remove(destPath) // Clean up partial file
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to save file",
			})
			return
		}
		destFile.Close() // Close before encrypting

		log.Printf("Uploaded file: %s (%d bytes) -> %s", header.Filename, written, destPath)

		// Encrypt the file at rest
		encPath, err := encryptor.EncryptFile(destPath)
		if err != nil {
			log.Printf("Error encrypting file: %v", err)
			os.Remove(destPath)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to encrypt file",
			})
			return
		}

		// Remove the unencrypted file
		os.Remove(destPath)
		log.Printf("Encrypted file: %s -> %s", destPath, encPath)

		// Save video to database with encrypted file path
		video := &db.Video{
			ID:          uploadID,
			Filename:    header.Filename,
			Size:        written,
			ContentType: contentType,
			FilePath:    encPath,
			CreatedAt:   time.Now(),
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
			log.Printf("Error saving video to database: %v", err)
			os.Remove(encPath) // Clean up encrypted file
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to save video record",
			})
			return
		}

		// Create initial transcription record
		transcription := &db.Transcription{
			ID:        generateID(),
			VideoID:   uploadID,
			Status:    "pending",
			Message:   "Video uploaded, ready for transcription",
			Progress:  0,
			CreatedAt: time.Now(),
		}
		if err := database.CreateTranscription(transcription); err != nil {
			log.Printf("Error creating transcription record: %v", err)
			// Don't fail the upload, transcription record can be created later
		}

		// Return success with upload ID
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "success",
			"upload_id": uploadID,
			"filename":  header.Filename,
			"size":      written,
			"message":   fmt.Sprintf("File uploaded successfully (%d bytes)", written),
		})
	}))

	// Start transcription for an upload (rate limited: 5/min per IP)
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

		// Find the video file
		videoPath, err := findVideoFile(uploadID)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": err.Error(),
			})
			return
		}

		// Check if already processing from database
		existingTranscription, err := database.GetTranscription(uploadID)
		if err != nil {
			log.Printf("Error getting transcription: %v", err)
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
			transcription := &db.Transcription{
				ID:        generateID(),
				VideoID:   uploadID,
				Status:    "processing",
				Message:   "Extracting audio...",
				Progress:  10,
				CreatedAt: time.Now(),
			}
			if err := database.CreateTranscription(transcription); err != nil {
				log.Printf("Error creating transcription record: %v", err)
			}
		} else {
			database.UpdateTranscriptionStatus(uploadID, "processing", "Extracting audio...", 10)
		}

		// Process in background
		go func() {
			log.Printf("Starting transcription for %s", uploadID)

			// Decrypt video file if encrypted
			workingVideoPath := videoPath
			if strings.HasSuffix(videoPath, ".age") {
				database.UpdateTranscriptionStatus(uploadID, "processing", "Decrypting video...", 5)
				decryptedPath, err := encryptor.DecryptToTempFile(videoPath)
				if err != nil {
					log.Printf("Video decryption failed: %v", err)
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
				log.Printf("Audio extraction failed: %v", err)
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
			result, err := transcribe(audioPath, outputPath)
			close(progressDone) // Stop progress simulation

			if err != nil {
				log.Printf("Transcription failed: %v", err)
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

			// Success - save to database
			log.Printf("Transcription complete for %s: %d segments", uploadID, len(result.Segments))
			if err := database.CompleteTranscription(uploadID, result.Language, result.Duration, result.Text, segments); err != nil {
				log.Printf("Error saving transcription result: %v", err)
			}

			// Clean up intermediate files
			os.Remove(audioPath)
		}()

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
			log.Printf("Error getting transcription: %v", err)
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
			log.Printf("Error getting transcription: %v", err)
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
		}

		// Update segments in database
		if err := database.UpdateSegments(uploadID, req.Segments); err != nil {
			log.Printf("Error updating segments: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to update segments",
			})
			return
		}

		log.Printf("Updated segments for %s: %d segments", uploadID, len(req.Segments))
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
			log.Printf("Error getting transcription: %v", err)
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

		if strings.TrimSpace(req.Text) == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Text is required",
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
			log.Printf("Error getting segments: %v", err)
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
			log.Printf("Applied script conversion to %s for upload %s", targetScript, uploadID)
		}

		// Update segments in database
		if err := database.UpdateSegments(uploadID, newSegments); err != nil {
			log.Printf("Error updating segments: %v", err)
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
		log.Printf("Aligned transcript for %s (mode=%s): %d segments, %.1f%% match rate",
			uploadID, mode, len(newSegments), result.Stats.MatchRate*100)

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
			log.Printf("Error getting transcription: %v", err)
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
			log.Printf("Error getting transcription: %v", err)
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
			log.Printf("Error getting transcription: %v", err)
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

		videos, err := database.ListVideos(userPtr, sessionPtr)
		if err != nil {
			log.Printf("Error listing videos: %v", err)
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

		result := make([]VideoWithStatus, len(videos))
		for i, v := range videos {
			result[i] = VideoWithStatus{Video: v, TranscriptionStatus: "none"}
			if t, err := database.GetTranscription(v.ID); err == nil && t != nil {
				result[i].TranscriptionStatus = t.Status
			}

			// Calculate expiration time based on user type
			// Anonymous: 48 hours, Registered: 90 days
			var expiresAt time.Time
			if v.UserID == nil {
				expiresAt = v.CreatedAt.Add(48 * time.Hour)
			} else {
				expiresAt = v.CreatedAt.Add(90 * 24 * time.Hour)
			}
			result[i].ExpiresAt = &expiresAt
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"videos": result,
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
			log.Printf("Error getting video: %v", err)
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
			log.Printf("Error getting transcription: %v", err)
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

		// Find the video file
		videoPath, err := findVideoFile(uploadID)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": err.Error(),
			})
			return
		}

		// Update transcription status to processing
		database.UpdateTranscriptionStatus(uploadID, "processing", "Extracting audio...", 10)

		// Process in background (same logic as POST /api/transcribe/{id})
		go func() {
			log.Printf("Reprocessing transcription for %s", uploadID)

			// Decrypt video file if encrypted
			workingVideoPath := videoPath
			if strings.HasSuffix(videoPath, ".age") {
				database.UpdateTranscriptionStatus(uploadID, "processing", "Decrypting video...", 5)
				decryptedPath, err := encryptor.DecryptToTempFile(videoPath)
				if err != nil {
					log.Printf("Video decryption failed: %v", err)
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
				log.Printf("Audio extraction failed: %v", err)
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

			// Run whisper
			outputPath := filepath.Join(uploadDir, uploadID+"_transcript")
			result, err := transcribe(audioPath, outputPath)
			close(progressDone)

			if err != nil {
				log.Printf("Transcription failed: %v", err)
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
			log.Printf("Reprocessing complete for %s: %d segments", uploadID, len(result.Segments))
			if err := database.CompleteTranscription(uploadID, result.Language, result.Duration, result.Text, segments); err != nil {
				log.Printf("Error saving transcription result: %v", err)
			}

			// Clean up
			os.Remove(audioPath)
		}()

		json.NewEncoder(w).Encode(map[string]string{
			"status":  "processing",
			"message": "Reprocessing started",
		})
	}))

	// Serve uploaded video files for playback
	mux.HandleFunc("GET /api/videos/{id}/video", func(w http.ResponseWriter, r *http.Request) {
		uploadID := r.PathValue("id")
		if uploadID == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Upload ID required",
			})
			return
		}

		// Get video metadata from database for ETag generation
		var etag string
		if database != nil {
			video, err := database.GetVideo(uploadID)
			if err == nil && video != nil {
				// Generate ETag from video ID + creation time + size
				// Video files don't change after upload, so this is stable
				etag = generateETag(fmt.Sprintf("%s-%d-%d", video.ID, video.CreatedAt.Unix(), video.Size))

				// Check for conditional request (If-None-Match)
				if handleConditionalRequest(w, r, etag) {
					return // 304 Not Modified sent
				}
			}
		}

		// Find the video file
		videoPath, err := findVideoFile(uploadID)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": err.Error(),
			})
			return
		}

		// If file is encrypted, decrypt to temp file for serving
		// (http.ServeFile needs seekable file for range requests)
		if strings.HasSuffix(videoPath, ".age") {
			decryptedPath, err := encryptor.DecryptToTempFile(videoPath)
			if err != nil {
				log.Printf("Failed to decrypt video for serving: %v", err)
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

		// Serve the file
		http.ServeFile(w, r, videoPath)
	})

	// Start burning subtitles into video (rate limited: 2/min per IP)
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

		// Check if transcription exists and is complete
		transcription, err := database.GetTranscription(uploadID)
		if err != nil {
			log.Printf("Error getting transcription: %v", err)
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
			log.Printf("Error getting burn job: %v", err)
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

		// Find the video file
		videoPath, err := findVideoFile(uploadID)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": err.Error(),
			})
			return
		}

		// Create or update burn job
		if existingJob == nil {
			burnJob := &db.BurnJob{
				ID:        generateID(),
				VideoID:   uploadID,
				Status:    "processing",
				Message:   "Starting subtitle burn...",
				Progress:  0,
				CreatedAt: time.Now(),
			}
			if err := database.CreateBurnJob(burnJob); err != nil {
				log.Printf("Error creating burn job: %v", err)
			}
		} else {
			database.UpdateBurnJobStatus(uploadID, "processing", "Starting subtitle burn...", 0)
		}

		// Process in background
		go func() {
			log.Printf("Starting subtitle burn for %s", uploadID)

			// Decrypt video if encrypted
			workingVideoPath := videoPath
			if strings.HasSuffix(videoPath, ".age") {
				database.UpdateBurnJobStatus(uploadID, "processing", "Decrypting video...", 5)
				decryptedPath, err := encryptor.DecryptToTempFile(videoPath)
				if err != nil {
					log.Printf("Video decryption failed: %v", err)
					database.FailBurnJob(uploadID, fmt.Sprintf("Video decryption failed: %v", err))
					return
				}
				workingVideoPath = decryptedPath
				defer os.Remove(decryptedPath)
			}

			// Get segments for SRT generation
			segments, err := transcription.GetSegments()
			if err != nil || len(segments) == 0 {
				log.Printf("No segments available: %v", err)
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
				log.Printf("Failed to write SRT file: %v", err)
				database.FailBurnJob(uploadID, fmt.Sprintf("Failed to write SRT file: %v", err))
				return
			}
			defer os.Remove(srtPath)

			database.UpdateBurnJobStatus(uploadID, "processing", "Burning subtitles into video...", 20)

			// Burn subtitles using ffmpeg with subtitles filter
			// Output to a temp file first, then encrypt
			outputPath := filepath.Join(uploadDir, uploadID+"_burned.mp4")
			cmd := exec.Command("ffmpeg",
				"-i", workingVideoPath,
				"-vf", fmt.Sprintf("subtitles='%s':force_style='FontSize=24,PrimaryColour=&HFFFFFF,OutlineColour=&H000000,Outline=2'", srtPath),
				"-c:a", "copy",
				"-y",
				outputPath,
			)
			cmdOutput, err := cmd.CombinedOutput()
			if err != nil {
				log.Printf("ffmpeg burn subtitles failed: %v, output: %s", err, string(cmdOutput))
				database.FailBurnJob(uploadID, fmt.Sprintf("Failed to burn subtitles: %v", err))
				return
			}

			database.UpdateBurnJobStatus(uploadID, "processing", "Encrypting output...", 90)

			// Encrypt the output file
			encOutputPath, err := encryptor.EncryptFile(outputPath)
			if err != nil {
				log.Printf("Failed to encrypt burned video: %v", err)
				os.Remove(outputPath)
				database.FailBurnJob(uploadID, fmt.Sprintf("Failed to encrypt output: %v", err))
				return
			}
			os.Remove(outputPath) // Remove unencrypted file

			log.Printf("Subtitle burn complete for %s: %s", uploadID, encOutputPath)
			database.CompleteBurnJob(uploadID, encOutputPath)
		}()

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
			log.Printf("Error getting burn job: %v", err)
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

		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":   job.Status,
			"message":  job.Message,
			"progress": job.Progress,
		})
	})

	// Download burned video
	mux.HandleFunc("GET /api/videos/{id}/burned", func(w http.ResponseWriter, r *http.Request) {
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
			log.Printf("Error getting burn job: %v", err)
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

		// Decrypt if encrypted
		servePath := job.OutputPath
		if strings.HasSuffix(job.OutputPath, ".age") {
			decryptedPath, err := encryptor.DecryptToTempFile(job.OutputPath)
			if err != nil {
				log.Printf("Failed to decrypt burned video: %v", err)
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

		// Set headers for download
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", downloadName))
		http.ServeFile(w, r, servePath)
	})

	// Script conversion rate limiter (10/min, same as upload)
	scriptLimiter := ratelimit.New(10, time.Minute)

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

	log.Printf("Backend server starting on :%s", port)
	log.Printf("Using whisper model: %s", getWhisperModel())

	// Wrap mux with CSRF middleware then request ID middleware
	csrfMiddleware := csrf.Middleware(auth.GetTokenFromRequest)
	handler := requestIDMiddleware(csrfMiddleware(mux))

	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatalf("Failed to start server: %v", err)
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
	log.Println("Running cleanup for expired videos...")

	expiredVideos, err := database.GetExpiredVideos()
	if err != nil {
		log.Printf("Error getting expired videos: %v", err)
		return
	}

	if len(expiredVideos) == 0 {
		log.Println("No expired videos to clean up")
		return
	}

	log.Printf("Found %d expired videos to clean up", len(expiredVideos))

	deletedCount := 0
	for _, video := range expiredVideos {
		// Delete from database and get file path
		filePath, err := database.DeleteVideo(video.ID)
		if err != nil {
			log.Printf("Error deleting video %s from database: %v", video.ID, err)
			continue
		}

		// Delete the file from disk
		if filePath != "" {
			if err := os.Remove(filePath); err != nil {
				if !os.IsNotExist(err) {
					log.Printf("Error deleting file %s: %v", filePath, err)
				}
			} else {
				log.Printf("Deleted file: %s", filePath)
			}
		}

		deletedCount++
		log.Printf("Cleaned up expired video: %s (user: %v, created: %s)",
			video.ID, video.UserID != nil, video.CreatedAt.Format(time.RFC3339))
	}

	// Also clean up expired sessions
	sessionCount, err := database.DeleteExpiredSessions()
	if err != nil {
		log.Printf("Error deleting expired sessions: %v", err)
	} else if sessionCount > 0 {
		log.Printf("Deleted %d expired sessions", sessionCount)
	}

	// Clean up old login attempts (older than 1 hour to be safe)
	loginAttemptCount, err := database.DeleteExpiredLoginAttempts(time.Now().Add(-1 * time.Hour))
	if err != nil {
		log.Printf("Error deleting expired login attempts: %v", err)
	} else if loginAttemptCount > 0 {
		log.Printf("Deleted %d expired login attempts", loginAttemptCount)
	}

	log.Printf("Cleanup complete: %d videos deleted", deletedCount)
}
