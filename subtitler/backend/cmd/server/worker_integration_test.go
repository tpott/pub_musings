package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/trevor/subtitler/internal/auth"
	"github.com/trevor/subtitler/internal/config"
	"github.com/trevor/subtitler/internal/db"
	"github.com/trevor/subtitler/internal/email"
	"github.com/trevor/subtitler/internal/transcribe"
	"github.com/trevor/subtitler/internal/worker"
)

// TestWorkerIntegration tests the full upload -> worker -> completion flow
// This test requires whisper.cpp to be properly configured
func TestWorkerIntegration(t *testing.T) {
	// Skip in short mode
	if testing.Short() {
		t.Skip("Skipping worker integration test in short mode")
	}

	// Setup test database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}
	defer database.Close()

	// Run migrations
	migrationsDir := filepath.Join("..", "..", "internal", "db", "migrations")
	if err := database.Migrate(migrationsDir); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Setup test config
	cfg := &config.Config{
		WhisperServerPath: os.Getenv("WHISPER_SERVER_PATH"),
		WhisperModelPath:  os.Getenv("WHISPER_MODEL_PATH"),
		WhisperServerPort: 9091, // Use different port for testing
		WhisperThreads:    4,
		JWTSecret:         "test-secret-key-for-integration-testing",
		DatabasePath:      dbPath,
		DataDir:           tempDir,
	}

	// Set default paths if not provided
	if cfg.WhisperServerPath == "" {
		home := os.Getenv("HOME")
		cfg.WhisperServerPath = filepath.Join(home, "Github/whisper.cpp/build/bin/whisper-server")
	}
	if cfg.WhisperModelPath == "" {
		home := os.Getenv("HOME")
		cfg.WhisperModelPath = filepath.Join(home, "Github/whisper.cpp/models/ggml-medium.bin")
	}

	// Check if whisper.cpp is available
	if _, err := os.Stat(cfg.WhisperServerPath); os.IsNotExist(err) {
		t.Skip("whisper-server not found, skipping integration test")
	}
	if _, err := os.Stat(cfg.WhisperModelPath); os.IsNotExist(err) {
		t.Skip("whisper model not found, skipping integration test")
	}

	// Start transcription service
	transcribeService := transcribe.NewService(cfg)
	if err := transcribeService.Start(); err != nil {
		t.Fatalf("Failed to start transcription service: %v", err)
	}
	defer transcribeService.Stop()

	// Wait for whisper-server to be ready
	time.Sleep(2 * time.Second)

	// Initialize email client (disabled for testing)
	emailClient := email.NewClient("", "", false)

	// Start worker pool with just 1 worker for testing
	workerPool := worker.NewWorkerPool(1, 10, database, transcribeService, emailClient)
	workerPool.Start()
	defer workerPool.Stop()

	// Create a test user
	testEmail := "test@example.com"
	testPassword := "test-password"
	passwordHash, err := auth.HashPassword(testPassword)
	if err != nil {
		t.Fatalf("Failed to hash password: %v", err)
	}

	testUser, err := database.CreateUser(testEmail, passwordHash)
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// Generate a valid JWT token for the test user
	token, err := auth.GenerateJWT(testUser.ID, testUser.Email, cfg.JWTSecret)
	if err != nil {
		t.Fatalf("Failed to generate JWT token: %v", err)
	}

	// Create a test audio file
	// Use the JFK sample from whisper.cpp if available
	testAudioPath := filepath.Join(os.Getenv("HOME"), "Github/whisper.cpp/samples/jfk.wav")
	if _, err := os.Stat(testAudioPath); os.IsNotExist(err) {
		t.Skip("JFK sample audio not found, skipping integration test")
	}

	audioData, err := os.ReadFile(testAudioPath)
	if err != nil {
		t.Fatalf("Failed to read test audio: %v", err)
	}

	// Create upload request
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "jfk.wav")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}
	_, err = part.Write(audioData)
	if err != nil {
		t.Fatalf("Failed to write audio data: %v", err)
	}
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.AddCookie(&http.Cookie{
		Name:  "subtitler_token",
		Value: token,
	})

	// Add claims to request context (simulating auth middleware)
	claims := &auth.Claims{
		UserID: testUser.ID,
		Email:  testUser.Email,
	}
	ctx := auth.AddClaimsToContext(req.Context(), claims)
	req = req.WithContext(ctx)

	// Send upload request
	rr := httptest.NewRecorder()
	handler := handleUpload(database, workerPool)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("Upload failed with status %d: %s", rr.Code, rr.Body.String())
	}

	// Parse response to get job ID
	var uploadResp JobUploadResponse
	err = json.NewDecoder(rr.Body).Decode(&uploadResp)
	if err != nil {
		t.Fatalf("Failed to decode upload response: %v", err)
	}

	if !uploadResp.Success {
		t.Fatalf("Upload response indicates failure: %s", uploadResp.Message)
	}

	jobID := uploadResp.JobID
	t.Logf("Job %d created successfully", jobID)

	// Give the worker a moment to pick up the job
	time.Sleep(500 * time.Millisecond)

	// Poll for job completion (max 60 seconds)
	maxWait := 60 * time.Second
	pollInterval := 500 * time.Millisecond
	timeout := time.After(maxWait)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	var job *db.Job
	completed := false
	for !completed {
		select {
		case <-timeout:
			t.Fatalf("Timeout waiting for job to complete after %v", maxWait)
		case <-ticker.C:
			job, err = database.GetJobByID(jobID)
			if err != nil {
				t.Logf("Failed to get job status (will retry): %v", err)
				continue // Retry on next tick
			}

			t.Logf("Job %d status: %s", jobID, job.Status)

			if job.Status == "completed" {
				completed = true
			} else if job.Status == "failed" {
				errorMsg := ""
				if job.ErrorMessage != nil {
					errorMsg = *job.ErrorMessage
				}
				t.Fatalf("Job failed with error: %s", errorMsg)
			}
		}
	}

	// Verify job details
	if job.TranscriptPath == nil || *job.TranscriptPath == "" {
		t.Error("Expected transcript path to be set")
	}

	if job.CompletedAt == nil {
		t.Error("Expected completed_at to be set")
	}

	// Verify transcript file exists
	transcriptPath := *job.TranscriptPath
	if _, err := os.Stat(transcriptPath); os.IsNotExist(err) {
		t.Errorf("Transcript file does not exist at %s", transcriptPath)
	}

	// Read transcript file and verify it's not empty
	transcriptData, err := os.ReadFile(transcriptPath)
	if err != nil {
		t.Fatalf("Failed to read transcript file: %v", err)
	}

	if len(transcriptData) == 0 {
		t.Error("Transcript file is empty")
	}

	t.Logf("Transcript preview (first 200 chars): %s", string(transcriptData[:min(200, len(transcriptData))]))

	// Verify transcript contains expected content (SRT format)
	transcriptStr := string(transcriptData)
	if len(transcriptStr) < 10 {
		t.Error("Transcript is too short")
	}

	// Basic SRT format check (should start with "1")
	if transcriptStr[0] != '1' {
		t.Errorf("Expected SRT to start with '1', got '%c'", transcriptStr[0])
	}

	t.Log("Worker integration test completed successfully")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
