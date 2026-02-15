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

	"github.com/tpott/subtitler/backend/auth"
	"github.com/tpott/subtitler/backend/db"
	"github.com/tpott/subtitler/backend/errmsg"
	"github.com/tpott/subtitler/backend/httputil"
	"github.com/tpott/subtitler/backend/logging"
	"github.com/tpott/subtitler/backend/metrics"
	"github.com/tpott/subtitler/backend/validation"
)

// maxPaginationOffset is the maximum allowed offset for paginated queries
// to prevent query abuse (e.g., scanning the entire database).
const maxPaginationOffset = 100000

// isShuttingDown checks if the server is shutting down.
// Background goroutines should call this at key checkpoints to exit gracefully.
func isShuttingDown() bool {
	select {
	case <-shutdownCtx.Done():
		return true
	default:
		return false
	}
}

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
			httputil.WriteContent(w, []byte(`{"error":"Rate limit exceeded. Please try again later."}`), "user rate limit response")
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

// validatePathID validates an ID from URL path, returning an error response if invalid.
// Returns the ID and true if valid, or empty string and false if invalid (response already written).
func validatePathID(w http.ResponseWriter, id string, fieldName string) (string, bool) {
	if id == "" {
		httputil.RespondError(w, http.StatusBadRequest, fieldName+" required")
		return "", false
	}
	if err := validation.ValidateHexID(id); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid "+fieldName+" format")
		return "", false
	}
	return id, true
}

// getValidSessionID extracts and validates the session_id query parameter.
// Returns the validated session_id, or empty string if missing or invalid.
func getValidSessionID(r *http.Request) string {
	return validation.ValidateSessionID(r.URL.Query().Get("session_id"))
}

// removeWithLogging removes a file and logs a warning if the removal fails.
// This is used for cleanup operations where failure is not critical but should be logged.
func removeWithLogging(path string, description string) {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		logging.Warn("Failed to remove file",
			"description", description,
			"path", path,
			"error", err)
	}
}

// escapeFFmpegFilterPath escapes special characters in a file path for use
// inside ffmpeg filter string values (e.g., subtitles='path':force_style='...').
// FFmpeg filter syntax requires escaping: ' : \ [ ] ; ,
func escapeFFmpegFilterPath(path string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`'`, `\'`,
		`:`, `\:`,
		`[`, `\[`,
		`]`, `\]`,
		`;`, `\;`,
	)
	return replacer.Replace(path)
}

// getWhisperModel returns the whisper model path from env or default.
// Returns an error if the path contains path traversal patterns.
func getWhisperModel() (string, error) {
	model := os.Getenv("WHISPER_MODEL")
	if model == "" {
		// Default to medium model - adjust path as needed
		model = os.ExpandEnv("$HOME/Github/whisper.cpp/models/ggml-medium.bin")
	}

	// Validate the path for security issues (path traversal, null bytes)
	if err := validation.ValidateFilePath(model); err != nil {
		return "", fmt.Errorf("invalid WHISPER_MODEL path: %w", err)
	}

	// Clean the path to normalize it
	model = filepath.Clean(model)

	return model, nil
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
	io.Copy(io.Discard, resp.Body) // drain for connection reuse

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

// transcribeAudio runs whisper-cli on the audio file.
// language is an ISO 639-1 code (e.g., "en", "es") or "auto" for auto-detection.
func transcribeAudio(audioPath, outputPath, language string) (*WhisperResult, error) {
	model, err := getWhisperModel()
	if err != nil {
		return nil, err
	}

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
	if err := writer.WriteField("response_format", "verbose_json"); err != nil {
		return nil, fmt.Errorf("failed to write response_format field: %v", err)
	}
	if err := writer.WriteField("temperature", whisperTemperature); err != nil {
		return nil, fmt.Errorf("failed to write temperature field: %v", err)
	}
	if err := writer.WriteField("language", language); err != nil {
		return nil, fmt.Errorf("failed to write language field: %v", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %v", err)
	}

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

		// Read response with size limit to prevent memory exhaustion from malformed responses
		respBody, err = io.ReadAll(io.LimitReader(resp.Body, maxWhisperResponseSize))
		// Close response body immediately after reading to prevent leaks on retries
		resp.Body.Close()
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
	// Each segment may include a words[] array with per-word timestamps
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
			Words []struct {
				Word        string  `json:"word"`
				Start       float64 `json:"start"`
				End         float64 `json:"end"`
				Probability float64 `json:"probability"`
			} `json:"words"`
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
		ws := WhisperSegment{
			ID:    seg.ID,
			Start: seg.Start,
			End:   seg.End,
			Text:  seg.Text,
		}
		if len(seg.Words) > 0 {
			ws.Words = make([]WhisperWord, len(seg.Words))
			for j, w := range seg.Words {
				ws.Words[j] = WhisperWord{
					Word:        w.Word,
					Start:       w.Start,
					End:         w.End,
					Probability: w.Probability,
				}
			}
		}
		result.Segments[i] = ws
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
	model, err := getWhisperModel()
	if err != nil {
		return nil, err
	}
	logging.Info("Using whisper-cli", "model", model, "language", language)
	return transcribeAudio(audioPath, outputPath, language)
}

// whisperWordsToDBWords converts WhisperWord slices to db.Word slices
func whisperWordsToDBWords(words []WhisperWord) []db.Word {
	if len(words) == 0 {
		return nil
	}
	result := make([]db.Word, len(words))
	for i, w := range words {
		result[i] = db.Word{
			Text:        w.Word,
			Start:       w.Start,
			End:         w.End,
			Probability: w.Probability,
		}
	}
	return result
}

// dbWordsToWhisperWords converts db.Word slices to WhisperWord slices
func dbWordsToWhisperWords(words []db.Word) []WhisperWord {
	if len(words) == 0 {
		return nil
	}
	result := make([]WhisperWord, len(words))
	for i, w := range words {
		result[i] = WhisperWord{
			Word:        w.Text,
			Start:       w.Start,
			End:         w.End,
			Probability: w.Probability,
		}
	}
	return result
}

// whisperSegmentsToDBSegments converts WhisperSegment slices to db.Segment slices
func whisperSegmentsToDBSegments(segments []WhisperSegment) []db.Segment {
	result := make([]db.Segment, len(segments))
	for i, s := range segments {
		result[i] = db.Segment{
			ID:    s.ID,
			Start: s.Start,
			End:   s.End,
			Text:  s.Text,
			Words: whisperWordsToDBWords(s.Words),
		}
	}
	return result
}

// dbSegmentsToWhisperSegments converts db.Segment slices to WhisperSegment slices
func dbSegmentsToWhisperSegments(segments []db.Segment) []WhisperSegment {
	result := make([]WhisperSegment, len(segments))
	for i, s := range segments {
		result[i] = WhisperSegment{
			ID:    s.ID,
			Start: s.Start,
			End:   s.End,
			Text:  s.Text,
			Words: dbWordsToWhisperWords(s.Words),
		}
	}
	return result
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
	// Reject IDs containing path separators or traversal characters
	if strings.ContainsAny(uploadID, "/\\") || strings.Contains(uploadID, "..") || strings.Contains(uploadID, "\x00") {
		return "", fmt.Errorf("invalid upload ID: %s", uploadID)
	}

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

	// Defense-in-depth: validate glob result is within upload directory
	if pathValidator != nil {
		if err := pathValidator.ValidateAbsolutePath(matches[0]); err != nil {
			logging.Warn("findVideoFile: glob result failed path validation",
				"path", matches[0],
				"upload_id", uploadID,
				"error", err.Error(),
			)
			return "", fmt.Errorf("video path validation failed for upload ID: %s", uploadID)
		}
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

	// Sanitize error messages for client consumption
	// The raw message may contain internal details (paths, IPs, server errors)
	message := t.Message
	if t.Status == "error" && t.Message != "" && !errmsg.IsVerbose() {
		// Return user-friendly message instead of raw error
		message = errmsg.ErrTranscribeFailed
	}

	status := &TranscriptionStatus{
		Status:   t.Status,
		Message:  message,
		Progress: t.Progress,
	}

	if t.Status == "complete" {
		segments, err := t.GetSegments()
		if err != nil {
			logging.Error("Failed to get transcription segments", "transcription_id", t.ID, "video_id", t.VideoID, "error", err)
			// Return empty segments array instead of failing completely
			segments = []db.Segment{}
		}
		whisperSegments := dbSegmentsToWhisperSegments(segments)
		status.Result = &WhisperResult{
			Language: t.Language,
			Duration: t.Duration,
			Text:     t.FullText,
			Segments: whisperSegments,
		}
	}

	return status
}

// updateSessionMetrics updates the Prometheus active sessions gauge.
// Called after session creation/deletion to keep metrics accurate.
func updateSessionMetrics() {
	if database == nil {
		return // Database not yet initialized
	}
	count, err := database.CountActiveSessions()
	if err != nil {
		logging.Error("Failed to count active sessions for metrics", "error", err)
		return
	}
	metrics.SetActiveSessions(count)
}
