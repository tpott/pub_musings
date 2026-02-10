package main

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/tpott/pub_musings/peekaboo/backend/api"
	"github.com/tpott/pub_musings/peekaboo/backend/crypto"
	"github.com/tpott/pub_musings/peekaboo/backend/db"
	"github.com/tpott/pub_musings/peekaboo/backend/email"
	"github.com/tpott/pub_musings/peekaboo/backend/llm"
	"github.com/tpott/pub_musings/peekaboo/backend/logging"
	"github.com/tpott/pub_musings/peekaboo/backend/tts"
)

func main() {
	// Setup structured logging
	logging.Setup()

	// Initialize database
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "data/peekaboo.db"
	}

	// Ensure data directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		slog.Error("failed to create data directory", "error", err)
		os.Exit(1)
	}

	database, err := db.Open(dbPath)
	if err != nil {
		slog.Error("failed to open database", "error", err, "path", dbPath)
		os.Exit(1)
	}
	defer func() {
		if err := database.Close(); err != nil {
			slog.Warn("error closing database", "error", err)
		}
	}()

	// Initialize schema and seed concepts
	if err := database.Init(); err != nil {
		slog.Error("failed to initialize database", "error", err)
		os.Exit(1)
	}

	// Seed media sets from data/media directory if they don't exist
	if err := seedMediaFromDisk(database); err != nil {
		slog.Warn("failed to seed media from disk", "error", err)
	}

	// Create LLM provider from environment
	llmProvider, err := llm.NewProviderFromEnv()
	if err != nil {
		slog.Error("failed to create LLM provider", "error", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()

	// Health check endpoints
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			slog.Debug("failed to write health response", "error", err)
		}
	})
	mux.Handle("GET /health/live", api.NewLivenessHandler())
	mux.Handle("GET /health/ready", api.NewReadinessHandlerWithLLM(database, llmProvider))

	// Create rate limiter for expensive endpoints (10 requests per minute per IP)
	rateLimiter := api.NewRateLimiter(10, time.Minute)

	// Start rate limiter cleanup goroutine to prevent memory growth from stale entries.
	// The cleanup runs every 5 minutes and removes entries with no recent requests.
	// The goroutine stops when cleanupCtx is canceled during shutdown.
	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				rateLimiter.Cleanup()
			case <-cleanupCtx.Done():
				return
			}
		}
	}()

	// Start daily audio blob cleanup if INTERACTION_LOG_AUDIO is enabled
	api.StartAudioBlobCleanupTicker(database, slog.Default(), cleanupCtx.Done())

	// Configure proxy header trust for rate limiter IP extraction.
	// Default is false (use RemoteAddr only) to prevent IP spoofing.
	// Set TRUST_PROXY_HEADERS=true when behind a trusted reverse proxy.
	if strings.EqualFold(os.Getenv("TRUST_PROXY_HEADERS"), "true") {
		api.TrustProxyHeaders = true
		slog.Info("proxy header trust enabled (X-Forwarded-For, X-Real-IP)")
	}

	// Get allowed origin from environment (used by CORS middleware and WebSocket)
	allowedOrigin := os.Getenv("ALLOWED_ORIGIN")
	if allowedOrigin == "" {
		// Default to wildcard for development; set ALLOWED_ORIGIN in production
		allowedOrigin = "*"
		slog.Warn("ALLOWED_ORIGIN not set, using wildcard", "origin", "*")
	} else {
		slog.Info("CORS configured", "allowed_origin", allowedOrigin)
	}

	// Initialize CSRF secret from env var or generate random one
	csrfSecret := initCSRFSecret()

	// Auth endpoints — use Resend when API key is configured, otherwise log-only
	var emailSender api.EmailSender
	if resendKey := os.Getenv("RESEND_API_KEY"); resendKey != "" {
		emailFrom := os.Getenv("EMAIL_FROM")
		if emailFrom == "" {
			emailFrom = "noreply@peekaboo.pottingers.us"
		}
		appURL := os.Getenv("APP_URL")
		if appURL == "" {
			appURL = "http://localhost:4321"
		}
		emailSender = email.NewResendEmailSender(resendKey, emailFrom, appURL)
		slog.Info("email sending enabled via Resend", "from", emailFrom)
	} else {
		slog.Info("email sending disabled (RESEND_API_KEY not set, using LogEmailSender)")
	}
	authHandler := api.NewAuthHandler(database, emailSender) // nil falls back to LogEmailSender
	authHandler.CSRFSecret = csrfSecret
	resendVerificationLimiter := api.NewRateLimiter(3, 15*time.Minute)
	magicLinkLimiter := api.NewRateLimiter(5, time.Minute)
	loginLimiter := api.NewRateLimiter(10, time.Minute)
	registerLimiter := api.NewRateLimiter(5, time.Minute)
	totpLimiter := api.NewRateLimiter(10, time.Minute)
	csrfLimiter := api.NewRateLimiter(30, time.Minute)
	mux.Handle("POST /api/auth/register", api.RateLimitMiddleware(http.HandlerFunc(authHandler.HandleRegister), registerLimiter))
	mux.HandleFunc("GET /api/auth/verify", authHandler.HandleVerify)
	mux.Handle("POST /api/auth/resend-verification", api.RateLimitMiddleware(http.HandlerFunc(authHandler.HandleResendVerification), resendVerificationLimiter))
	mux.Handle("POST /api/auth/login", api.RateLimitMiddleware(http.HandlerFunc(authHandler.HandleLogin), loginLimiter))
	mux.HandleFunc("POST /api/auth/logout", authHandler.HandleLogout)
	mux.HandleFunc("GET /api/auth/me", authHandler.HandleMe)
	mux.Handle("POST /api/auth/magic-link", api.RateLimitMiddleware(http.HandlerFunc(authHandler.HandleMagicLink), magicLinkLimiter))
	mux.HandleFunc("GET /api/auth/magic-link/verify", authHandler.HandleMagicLinkVerify)
	mux.Handle("GET /api/auth/csrf", api.RateLimitMiddleware(http.HandlerFunc(authHandler.HandleCSRF), csrfLimiter))
	mux.Handle("POST /api/auth/totp/setup", api.RateLimitMiddleware(http.HandlerFunc(authHandler.HandleTOTPSetup), totpLimiter))
	mux.Handle("POST /api/auth/totp/enable", api.RateLimitMiddleware(http.HandlerFunc(authHandler.HandleTOTPEnable), totpLimiter))
	mux.Handle("POST /api/auth/totp/disable", api.RateLimitMiddleware(http.HandlerFunc(authHandler.HandleTOTPDisable), totpLimiter))

	// API endpoints
	mux.Handle("POST /api/transcribe", api.RateLimitMiddleware(api.NewTranscribeHandler(""), rateLimiter))
	mux.Handle("POST /api/intent", api.RateLimitMiddleware(api.NewIntentHandlerWithProvider(llmProvider), rateLimiter))
	// Media endpoint with its own rate limiter (30 requests per minute per IP - more generous for browsing)
	mediaLimiter := api.NewRateLimiter(30, time.Minute)
	mux.Handle("GET /api/media/{concept}", api.RateLimitMiddleware(api.NewMediaHandler(database), mediaLimiter))

	// Feedback endpoint with its own rate limiter (5 requests per minute per IP)
	feedbackLimiter := api.NewRateLimiter(5, time.Minute)
	mux.Handle("POST /api/feedback", api.RateLimitMiddleware(api.NewFeedbackHandler(database), feedbackLimiter))

	// Admin endpoints (requires TRUSTED_USERS)
	adminHandler := api.NewAdminHandler(database, os.Getenv("TRUSTED_USERS"))
	mux.HandleFunc("GET /api/admin/feedback", adminHandler.HandleListFeedback)

	// Frontend log forwarding (development only, gated by FORWARD_FRONTEND_LOGS=true)
	if strings.EqualFold(os.Getenv("FORWARD_FRONTEND_LOGS"), "true") {
		mux.Handle("POST /api/log", api.NewLogHandler())
		slog.Info("frontend log forwarding enabled (POST /api/log)")
	}

	// TTS provider (optional - only enabled if PIPER_SERVER_URL is set)
	piperURL := os.Getenv("PIPER_SERVER_URL")
	var ttsProvider tts.Provider
	if piperURL != "" {
		var err error
		ttsProvider, err = tts.NewProvider(tts.Config{
			Provider:  "piper",
			ServerURL: piperURL,
		})
		if err != nil {
			slog.Error("failed to create TTS provider", "error", err)
			os.Exit(1)
		}
		mux.Handle("POST /api/speak", api.RateLimitMiddleware(api.NewSpeakHandler(ttsProvider), rateLimiter))
		slog.Info("TTS enabled", "server", piperURL)
	} else {
		slog.Debug("TTS disabled (PIPER_SERVER_URL not set)")
	}

	// WebSocket endpoint for audio streaming with rate limiting and origin validation
	whisperURL := os.Getenv("WHISPER_SERVER_URL")
	if whisperURL == "" {
		whisperURL = "http://127.0.0.1:8765"
	}
	wsHandler := api.NewAudioWebSocketHandlerWithOptions(whisperURL, llmProvider, database, rateLimiter, allowedOrigin)
	wsHandler.TTSProvider = ttsProvider // nil if Piper not configured
	mux.Handle("GET /ws/audio", wsHandler)

	// Static file server for media files
	mediaDir := os.Getenv("MEDIA_DIR")
	if mediaDir == "" {
		mediaDir = "data/media"
	}

	// Check for age key file - if present, use encrypted file server
	ageKeyFile := os.Getenv("AGE_KEY_FILE")
	if ageKeyFile == "" {
		ageKeyFile = "data/age.key"
	}

	if _, err := os.Stat(ageKeyFile); err == nil {
		// Age key exists - use encrypted file server
		identity, err := crypto.LoadIdentityFromFile(ageKeyFile)
		if err != nil {
			slog.Error("failed to load age key", "path", ageKeyFile, "error", err)
			os.Exit(1)
		}
		slog.Info("encrypted media serving enabled", "key_file", ageKeyFile)
		mux.Handle("/data/media/", http.StripPrefix("/data/media/", api.NewEncryptedFileServer(mediaDir, identity)))
	} else {
		// No age key - use plain file server
		slog.Warn("serving media files unencrypted", "key_file", ageKeyFile)
		mux.Handle("/data/media/", http.StripPrefix("/data/media/", http.FileServer(http.Dir(mediaDir))))
	}

	// Static file server for test fixtures (for e2e tests)
	mux.Handle("/fixtures/", http.StripPrefix("/fixtures/", http.FileServer(http.Dir("tests/fixtures"))))

	// Wrap with middleware chain: request logger -> request ID -> security headers -> CSRF -> CORS
	handler := logging.RequestLoggerMiddleware(logging.RequestIDMiddleware(api.SecurityHeadersMiddleware(api.CSRFMiddleware(api.CORSMiddleware(mux, allowedOrigin), csrfSecret, database))))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port

	// Create server with configured handler and timeouts.
	// ReadHeaderTimeout prevents slowloris attacks (slow header sends).
	// IdleTimeout closes idle keep-alive connections.
	// ReadTimeout and WriteTimeout are NOT set because they apply to the
	// entire connection lifetime, which would kill long-lived WebSocket connections.
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Start server in goroutine so we can handle shutdown signals
	go func() {
		slog.Info("starting server", "addr", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal (SIGINT or SIGTERM)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("received shutdown signal", "signal", sig.String())

	// Cancel cleanup goroutine
	cleanupCancel()

	// Create shutdown context with 30 second timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "error", err)
	} else {
		slog.Info("server stopped gracefully")
	}

	// Database will be closed by defer database.Close() when main returns
	slog.Info("shutdown complete")
}

// initCSRFSecret reads the CSRF secret from CSRF_SECRET env var.
// If unset, generates a random 32-byte secret (tokens won't survive restarts).
func initCSRFSecret() []byte {
	if secret := os.Getenv("CSRF_SECRET"); secret != "" {
		decoded, err := hex.DecodeString(secret)
		if err != nil {
			// Not hex — use the raw string as-is
			slog.Info("CSRF secret loaded from environment")
			return []byte(secret)
		}
		slog.Info("CSRF secret loaded from environment (hex)")
		return decoded
	}

	// Generate random secret
	secret := make([]byte, 32)
	if _, err := cryptorand.Read(secret); err != nil {
		slog.Error("failed to generate CSRF secret", "error", err)
		os.Exit(1)
	}
	slog.Warn("CSRF_SECRET not set, using random secret (tokens won't survive restarts)")
	return secret
}

// seedMediaFromDisk scans data/media/{concept}/set* directories and seeds the database.
// Supports both plain files (photo.jpg) and encrypted files (photo.jpg.age).
func seedMediaFromDisk(database *db.DB) error {
	mediaDir := os.Getenv("MEDIA_DIR")
	if mediaDir == "" {
		mediaDir = "data/media"
	}

	// Check if media directory exists
	if _, err := os.Stat(mediaDir); os.IsNotExist(err) {
		slog.Debug("media directory does not exist, skipping seeding", "path", mediaDir)
		return nil
	}

	// Iterate over concept directories
	concepts := []string{"cat", "dog", "duck", "pig", "chicken", "cow"}
	for _, concept := range concepts {
		conceptDir := filepath.Join(mediaDir, concept)
		if _, err := os.Stat(conceptDir); os.IsNotExist(err) {
			continue
		}

		// Find set directories
		entries, err := os.ReadDir(conceptDir)
		if err != nil {
			slog.Warn("cannot read concept directory", "path", conceptDir, "error", err)
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "set") {
				continue
			}

			setDir := filepath.Join(conceptDir, entry.Name())

			// Check for required photo file (plain or encrypted)
			photoPath := filepath.Join(setDir, "photo.jpg")
			photoExists := false
			if _, err := os.Stat(photoPath); err == nil {
				photoExists = true
			} else if _, err := os.Stat(photoPath + ".age"); err == nil {
				photoExists = true
			}
			if !photoExists {
				continue
			}

			// Check for optional audio file (plain or encrypted)
			audioPath := ""
			audioFile := filepath.Join(setDir, "audio.mp3")
			if _, err := os.Stat(audioFile); err == nil {
				audioPath = filepath.Join("data/media", concept, entry.Name(), "audio.mp3")
			} else if _, err := os.Stat(audioFile + ".age"); err == nil {
				audioPath = filepath.Join("data/media", concept, entry.Name(), "audio.mp3")
			}

			// Relative path for database (without .age extension - handler adds it)
			relPhotoPath := filepath.Join("data/media", concept, entry.Name(), "photo.jpg")

			// Seed the media set (database SeedMediaSet handles duplicates)
			if err := database.SeedMediaSet(concept, relPhotoPath, audioPath, ""); err != nil {
				slog.Warn("failed to seed media set", "concept", concept, "set", entry.Name(), "error", err)
			}
		}
	}

	return nil
}
