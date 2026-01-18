package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/trevor/subtitler/internal/analytics"
	"github.com/trevor/subtitler/internal/auth"
	"github.com/trevor/subtitler/internal/config"
	"github.com/trevor/subtitler/internal/db"
	"github.com/trevor/subtitler/internal/email"
	"github.com/trevor/subtitler/internal/storage"
	"github.com/trevor/subtitler/internal/transcribe"
	"github.com/trevor/subtitler/internal/worker"
)

const (
	// Maximum request body size (200MB + some overhead)
	maxRequestBodySize = 210 * 1024 * 1024
	// Upload directory for temporary files
	uploadDir = "./uploads"
	// Temporary directory for transcription
	transcribeDir = "./tmp/transcribe"
)

var (
	// Global transcription service instance
	transcribeService *transcribe.Service
)

type UploadResponse struct {
	Success  bool   `json:"success"`
	Message  string `json:"message"`
	Filename string `json:"filename,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Type     string `json:"type,omitempty"`
}

type TranscribeResponse struct {
	Success    bool    `json:"success"`
	Message    string  `json:"message"`
	Transcript string  `json:"transcript,omitempty"`
	Filename   string  `json:"filename,omitempty"`
	Format     string  `json:"format,omitempty"`
	Duration   float64 `json:"duration,omitempty"`
}

var allowedOrigin string

func enableCORS(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Credentials", "true")

	// Handle preflight requests
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return true
	}
	return false
}

func handleUploadOld(w http.ResponseWriter, r *http.Request) {
	if enableCORS(w, r) {
		return
	}

	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Limit the request body size
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)

	// Parse multipart form (32MB max memory)
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Failed to parse form: %v", err))
		return
	}

	// Get the file from the form
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "No file uploaded")
		return
	}
	defer file.Close()

	// Validate the file
	err = storage.ValidateFile(header.Filename, header.Size)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Create upload directory if it doesn't exist
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create upload directory")
		return
	}

	// Generate a unique filename to avoid collisions
	timestamp := time.Now().Unix()
	ext := filepath.Ext(header.Filename)
	baseFilename := header.Filename[:len(header.Filename)-len(ext)]
	destPath := filepath.Join(uploadDir, fmt.Sprintf("%s_%d%s", baseFilename, timestamp, ext))

	// Save the file
	err = storage.SaveUploadedFile(file, destPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to save file: %v", err))
		return
	}

	// Return success response
	response := UploadResponse{
		Success:  true,
		Message:  "File uploaded successfully",
		Filename: header.Filename,
		Size:     header.Size,
		Type:     header.Header.Get("Content-Type"),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(UploadResponse{
		Success: false,
		Message: message,
	})
}

func handleTranscribe(w http.ResponseWriter, r *http.Request) {
	if enableCORS(w, r) {
		return
	}

	if r.Method != http.MethodPost {
		writeTranscribeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Limit the request body size
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)

	// Parse multipart form (32MB max memory)
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		writeTranscribeError(w, http.StatusBadRequest, fmt.Sprintf("Failed to parse form: %v", err))
		return
	}

	// Get the file from the form
	file, header, err := r.FormFile("file")
	if err != nil {
		writeTranscribeError(w, http.StatusBadRequest, "No file uploaded")
		return
	}
	defer file.Close()

	// Validate the file
	err = storage.ValidateFile(header.Filename, header.Size)
	if err != nil {
		writeTranscribeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Check if transcription service is available
	if transcribeService == nil {
		writeTranscribeError(w, http.StatusServiceUnavailable, "Transcription service not initialized")
		return
	}

	// Create temporary directory if it doesn't exist
	if err := os.MkdirAll(transcribeDir, 0755); err != nil {
		writeTranscribeError(w, http.StatusInternalServerError, "Failed to create temporary directory")
		return
	}

	// Save to temporary file
	ext := filepath.Ext(header.Filename)
	tmpFile, err := os.CreateTemp(transcribeDir, "audio-*"+ext)
	if err != nil {
		writeTranscribeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create temporary file: %v", err))
		return
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath) // Clean up temporary file

	// Copy uploaded file to temporary location
	_, err = io.Copy(tmpFile, file)
	tmpFile.Close()
	if err != nil {
		writeTranscribeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to save temporary file: %v", err))
		return
	}

	// Get format parameter from query string (default: srt)
	formatStr := r.URL.Query().Get("format")
	if formatStr == "" {
		formatStr = "srt"
	}

	// Validate and convert format
	var format transcribe.OutputFormat
	switch formatStr {
	case "srt":
		format = transcribe.FormatSRT
	case "vtt":
		format = transcribe.FormatVTT
	case "text":
		format = transcribe.FormatText
	case "json":
		format = transcribe.FormatJSON
	default:
		writeTranscribeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid format '%s'. Supported formats: srt, vtt, text, json", formatStr))
		return
	}

	// Transcribe the file
	startTime := time.Now()
	transcript, err := transcribeService.TranscribeFile(tmpPath, transcribe.TranscribeOptions{
		Format: format,
	})
	duration := time.Since(startTime).Seconds()

	if err != nil {
		writeTranscribeError(w, http.StatusInternalServerError, fmt.Sprintf("Transcription failed: %v", err))
		return
	}

	// Return success response
	response := TranscribeResponse{
		Success:    true,
		Message:    "Transcription completed successfully",
		Transcript: transcript,
		Filename:   header.Filename,
		Format:     formatStr,
		Duration:   duration,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func writeTranscribeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(TranscribeResponse{
		Success: false,
		Message: message,
	})
}

func main() {
	// Load configuration
	cfg := config.Load()

	// Set CORS allowed origin from config
	allowedOrigin = cfg.FrontendURL
	log.Printf("CORS configured for origin: %s", allowedOrigin)

	// Initialize database
	log.Println("Initializing database...")
	migrationsDir := filepath.Join("internal", "db", "migrations")
	database, err := db.Initialize(cfg.DatabasePath, migrationsDir)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()
	log.Printf("Database initialized at %s", cfg.DatabasePath)

	// Create file storage directories
	log.Println("Creating file storage directories...")
	uploadsDir := filepath.Join(cfg.DataDir, "files", "uploads")
	resultsDir := filepath.Join(cfg.DataDir, "files", "results")
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		log.Fatalf("Failed to create uploads directory: %v", err)
	}
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		log.Fatalf("Failed to create results directory: %v", err)
	}
	log.Println("File storage directories created")

	// Initialize transcription service
	log.Println("Initializing transcription service...")
	transcribeService = transcribe.NewService(cfg)
	if err := transcribeService.Start(); err != nil {
		log.Fatalf("Failed to start transcription service: %v", err)
	}
	defer func() {
		log.Println("Stopping transcription service...")
		if err := transcribeService.Stop(); err != nil {
			log.Printf("Error stopping transcription service: %v", err)
		}
	}()
	log.Println("Transcription service started successfully")

	// Initialize email client
	log.Println("Initializing email client...")
	emailClient := email.NewClient(cfg.ResendAPIKey, cfg.EmailFrom, cfg.EnableEmail)
	if cfg.EnableEmail {
		log.Printf("Email notifications enabled (from: %s)", cfg.EmailFrom)
	} else {
		log.Println("Email notifications disabled")
	}

	// Initialize analytics service
	log.Println("Initializing analytics service...")
	analyticsService := analytics.NewService(database.DB)
	log.Println("Analytics service initialized")

	// Initialize worker pool for background job processing
	log.Println("Starting worker pool...")
	workerPool := worker.NewWorkerPool(4, 100, database, transcribeService, emailClient, analyticsService)
	workerPool.Start()
	defer workerPool.Stop()
	log.Println("Worker pool started with 4 workers")

	// Basic HTTP server placeholder
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Subtitler API Server")
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})

	// Health check endpoint for tunnel verification
	http.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Service is healthy",
			"version": "1.0.0",
		})
	})

	// Old /api/upload endpoint (kept for backward compatibility, no auth)
	http.HandleFunc("/api/upload-old", handleUploadOld)

	// New /api/upload endpoint (with auth and job queue)
	http.Handle("/api/upload", auth.AuthMiddleware(cfg.JWTSecret)(handleUpload(database, workerPool, analyticsService)))

	// Keep /api/transcribe for backward compatibility (synchronous)
	http.HandleFunc("/api/transcribe", handleTranscribe)

	// Auth endpoints
	http.HandleFunc("/api/register", handleRegister(database, cfg.JWTSecret, analyticsService))
	http.HandleFunc("/api/login", handleLogin(database, cfg.JWTSecret, analyticsService))
	http.HandleFunc("/api/logout", handleLogout)
	http.Handle("/api/me", auth.AuthMiddleware(cfg.JWTSecret)(http.HandlerFunc(handleMe)))

	// Jobs endpoints (protected)
	http.Handle("/api/jobs", auth.AuthMiddleware(cfg.JWTSecret)(handleListJobs(database)))
	http.Handle("/api/jobs/", auth.AuthMiddleware(cfg.JWTSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Route based on URL pattern
		if strings.HasSuffix(r.URL.Path, "/download") {
			handleDownloadResult(database)(w, r)
		} else if strings.Count(r.URL.Path, "/") == 3 { // /api/jobs/{id}
			handleGetJob(database)(w, r)
		} else {
			http.NotFound(w, r)
		}
	})))

	// Analytics endpoints
	http.HandleFunc("/api/analytics/events", handleTrackEvent(analyticsService))
	http.Handle("/api/analytics/funnel", auth.AuthMiddleware(cfg.JWTSecret)(handleGetFunnel(analyticsService)))
	http.HandleFunc("/api/analytics/experiments/", handleGetExperiment(analyticsService))

	port := fmt.Sprintf(":%d", cfg.ServerPort)
	log.Printf("Starting server on %s", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal(err)
	}
}
