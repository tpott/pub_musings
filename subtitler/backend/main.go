package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/tpott/subtitler/backend/audio"
	"github.com/tpott/subtitler/backend/auth"
	"github.com/tpott/subtitler/backend/captcha"
	"github.com/tpott/subtitler/backend/crypto"
	"github.com/tpott/subtitler/backend/csrf"
	"github.com/tpott/subtitler/backend/db"
	"github.com/tpott/subtitler/backend/email"
	"github.com/tpott/subtitler/backend/logging"
	"github.com/tpott/subtitler/backend/metrics"
	"github.com/tpott/subtitler/backend/pathvalidator"
)

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

	// Initialize database with connection pool configuration
	poolCfg := db.PoolConfig{
		MaxOpenConns: dbMaxOpenConns,
		MaxIdleConns: dbMaxIdleConns,
	}
	database, err = db.OpenWithConfig(dbPath, poolCfg)
	if err != nil {
		logging.Fatal("Failed to open database", "error", err)
	}
	defer database.Close()
	logging.Info("Database initialized", "path", dbPath, "max_open_conns", dbMaxOpenConns, "max_idle_conns", dbMaxIdleConns)

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

	// Register all handler groups
	registerSystemHandlers(mux)
	registerAuthHandlers(mux)
	registerUploadHandlers(mux)
	registerTranscriptionHandlers(mux)
	registerVideoHandlers(mux)
	registerTextHandlers(mux)

	// Initialize shutdown context for graceful shutdown
	// This context is used by background goroutines to know when to exit
	shutdownCtx, shutdownCancel = context.WithCancel(context.Background())

	// Start the cleanup scheduler for expired videos
	go startCleanupScheduler()

	// Start the database maintenance scheduler (VACUUM + ANALYZE)
	go startMaintenanceScheduler()

	whisperModel, whisperModelErr := getWhisperModel()
	if whisperModelErr != nil {
		logging.Warn("Invalid whisper model path", "error", whisperModelErr)
		whisperModel = "(invalid)"
	}
	logging.Info("Backend server starting", "port", port, "whisper_model", whisperModel)

	// Wrap mux with middleware chain (outermost runs first):
	// 1. Security headers - add CSP and other security headers
	// 2. Request ID - add X-Request-ID for tracing
	// 3. CSRF - validate CSRF tokens on state-changing requests
	// 4. Per-user rate limiting - applies to authenticated users
	// 5. Metrics - record request metrics for Prometheus
	csrfMiddleware := csrf.Middleware(auth.GetTokenFromRequest)
	handler := metrics.MetricsMiddleware(securityHeadersMiddleware(requestIDMiddleware(userRateLimitMiddleware(csrfMiddleware(mux)))))

	// Create HTTP server with graceful shutdown support
	server := &http.Server{
		Addr:    ":" + port,
		Handler: handler,
	}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logging.Fatal("Failed to start server", "error", err)
		}
	}()

	logging.Info("Server started successfully")

	// Wait for shutdown signal
	sig := <-sigChan
	logging.Info("Received shutdown signal", "signal", sig.String())

	// Initiate graceful shutdown
	shutdownCancel() // Signal background goroutines to stop

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
	defer cancel()

	logging.Info("Shutting down server", "timeout", defaultShutdownTimeout)

	// Gracefully shutdown the HTTP server
	if err := server.Shutdown(ctx); err != nil {
		logging.Error("Server shutdown error", "error", err)
	}

	logging.Info("Server shutdown complete")
}
