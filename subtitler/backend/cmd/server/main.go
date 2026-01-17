package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/trevor/subtitler/internal/config"
	"github.com/trevor/subtitler/internal/storage"
	"github.com/trevor/subtitler/internal/transcribe"
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

func enableCORS(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:4321")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// Handle preflight requests
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return true
	}
	return false
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
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

	// Transcribe the file
	startTime := time.Now()
	transcript, err := transcribeService.TranscribeFile(tmpPath, transcribe.TranscribeOptions{
		Format: transcribe.FormatSRT,
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
		Format:     "srt",
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

	// Basic HTTP server placeholder
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Subtitler API Server")
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK")
	})

	http.HandleFunc("/api/upload", handleUpload)
	http.HandleFunc("/api/transcribe", handleTranscribe)

	port := fmt.Sprintf(":%d", cfg.ServerPort)
	log.Printf("Starting server on %s", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal(err)
	}
}
