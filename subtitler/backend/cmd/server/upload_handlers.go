package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/trevor/subtitler/internal/auth"
	"github.com/trevor/subtitler/internal/db"
	"github.com/trevor/subtitler/internal/storage"
	"github.com/trevor/subtitler/internal/worker"
)

type JobUploadResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	JobID   int64  `json:"job_id,omitempty"`
}

// handleUpload handles file upload and job creation
// POST /api/upload
// Requires authentication
func handleUpload(database *db.DB, workerPool *worker.WorkerPool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Authenticate user
		userID, err := auth.GetUserIDFromRequest(r)
		if err != nil {
			respondJSON(w, http.StatusUnauthorized, JobUploadResponse{
				Success: false,
				Message: "Unauthorized: " + err.Error(),
			})
			return
		}

		// Parse multipart form (max 200MB)
		err = r.ParseMultipartForm(200 * 1024 * 1024)
		if err != nil {
			respondJSON(w, http.StatusBadRequest, JobUploadResponse{
				Success: false,
				Message: "Failed to parse form: " + err.Error(),
			})
			return
		}

		// Get file from form
		file, header, err := r.FormFile("file")
		if err != nil {
			respondJSON(w, http.StatusBadRequest, JobUploadResponse{
				Success: false,
				Message: "No file provided: " + err.Error(),
			})
			return
		}
		defer file.Close()

		// Validate file
		if err := storage.ValidateFile(header.Filename, header.Size); err != nil {
			respondJSON(w, http.StatusBadRequest, JobUploadResponse{
				Success: false,
				Message: "Invalid file: " + err.Error(),
			})
			return
		}

		// Get format parameter from form (default: srt)
		formatStr := r.FormValue("format")
		if formatStr == "" {
			formatStr = "srt"
		}

		// Validate format
		validFormats := map[string]bool{
			"srt":      true,
			"vtt":      true,
			"text":     true,
			"json":     true,
			"embedded": true,
		}
		if !validFormats[formatStr] {
			respondJSON(w, http.StatusBadRequest, JobUploadResponse{
				Success: false,
				Message: fmt.Sprintf("Invalid format '%s'. Supported formats: srt, vtt, text, json, embedded", formatStr),
			})
			return
		}

		// Create job record with status='pending'
		job := &db.Job{
			UserID:           userID,
			Status:           "pending",
			OriginalFilename: header.Filename,
			FileSize:         header.Size,
			OutputFormat:     formatStr,
		}

		err = database.CreateJob(job)
		if err != nil {
			log.Printf("Failed to create job: %v", err)
			respondJSON(w, http.StatusInternalServerError, JobUploadResponse{
				Success: false,
				Message: "Failed to create job: " + err.Error(),
			})
			return
		}

		// Build file path: data/files/uploads/{user_id}/{job_id}/{filename}
		filePath := buildUploadPath(job.UserID, job.ID, header.Filename)

		// Create directory if it doesn't exist
		uploadDir := filepath.Dir(filePath)
		if err := os.MkdirAll(uploadDir, 0755); err != nil {
			log.Printf("Failed to create upload directory: %v", err)
			// Mark job as failed
			database.UpdateJobFailed(job.ID, "Failed to create upload directory")
			respondJSON(w, http.StatusInternalServerError, JobUploadResponse{
				Success: false,
				Message: "Failed to create upload directory: " + err.Error(),
			})
			return
		}

		// Save file to disk
		outFile, err := os.Create(filePath)
		if err != nil {
			log.Printf("Failed to create file: %v", err)
			database.UpdateJobFailed(job.ID, "Failed to create file")
			respondJSON(w, http.StatusInternalServerError, JobUploadResponse{
				Success: false,
				Message: "Failed to create file: " + err.Error(),
			})
			return
		}
		defer outFile.Close()

		_, err = io.Copy(outFile, file)
		if err != nil {
			log.Printf("Failed to save file: %v", err)
			database.UpdateJobFailed(job.ID, "Failed to save file")
			respondJSON(w, http.StatusInternalServerError, JobUploadResponse{
				Success: false,
				Message: "Failed to save file: " + err.Error(),
			})
			return
		}

		// Update job with file path
		job.FilePath = filePath
		err = database.UpdateJobFilePath(job.ID, filePath)
		if err != nil {
			log.Printf("Failed to update job file path: %v", err)
			// Continue anyway - file is saved, worker can still process it
		}

		// Enqueue job for processing
		err = workerPool.Enqueue(job.ID)
		if err != nil {
			log.Printf("Failed to enqueue job: %v", err)
			database.UpdateJobFailed(job.ID, "Failed to enqueue job")
			respondJSON(w, http.StatusInternalServerError, JobUploadResponse{
				Success: false,
				Message: "Failed to enqueue job: " + err.Error(),
			})
			return
		}

		log.Printf("Job %d created for user %d: %s (%d bytes)", job.ID, userID, header.Filename, header.Size)

		respondJSON(w, http.StatusCreated, JobUploadResponse{
			Success: true,
			Message: "File uploaded successfully",
			JobID:   job.ID,
		})
	}
}

// buildUploadPath generates the file path for uploaded files
func buildUploadPath(userID, jobID int64, filename string) string {
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	return filepath.Join(
		dataDir,
		"files",
		"uploads",
		fmt.Sprintf("%d", userID),
		fmt.Sprintf("%d", jobID),
		filename,
	)
}

// respondJSON is a helper to send JSON responses
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
