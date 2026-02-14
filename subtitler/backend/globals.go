package main

import (
	"context"
	"net/http"

	"github.com/tpott/subtitler/backend/audio"
	"github.com/tpott/subtitler/backend/captcha"
	"github.com/tpott/subtitler/backend/crypto"
	"github.com/tpott/subtitler/backend/db"
	"github.com/tpott/subtitler/backend/email"
	"github.com/tpott/subtitler/backend/pathvalidator"
	"github.com/tpott/subtitler/backend/ratelimit"
	"github.com/tpott/subtitler/backend/security"
)

// WhisperWord represents a single word with timing from Whisper
type WhisperWord struct {
	Word        string  `json:"word"`
	Start       float64 `json:"start"`
	End         float64 `json:"end"`
	Probability float64 `json:"probability"`
}

// WhisperSegment represents a transcribed segment with timing
type WhisperSegment struct {
	ID    int           `json:"id"`
	Start float64       `json:"start"` // start time in seconds
	End   float64       `json:"end"`   // end time in seconds
	Text  string        `json:"text"`
	Words []WhisperWord `json:"words,omitempty"`
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
	Status            string                `json:"status"` // pending, processing, complete, error
	Message           string                `json:"message,omitempty"`
	Result            *WhisperResult        `json:"result,omitempty"`
	Progress          int                   `json:"progress,omitempty"` // 0-100
	EmbeddedSubtitles []audio.SubtitleTrack `json:"embedded_subtitles,omitempty"`
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
var feedbackLimiter *ratelimit.Limiter
var logLimiter *ratelimit.Limiter
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
	feedbackLimiter = ratelimit.New(feedbackRateLimit, feedbackRateWindow)
	logLimiter = ratelimit.New(logRateLimit, logRateWindow)
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

// Shutdown context for graceful shutdown
// Background goroutines should use this context to know when to stop
var shutdownCtx context.Context
var shutdownCancel context.CancelFunc

// Global audio extractor for video processing
var audioExtractor audio.Extractor

// Global path validator for file serving security
var pathValidator *pathvalidator.Validator
