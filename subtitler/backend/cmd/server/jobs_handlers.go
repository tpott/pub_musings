package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/trevor/subtitler/internal/db"
)

type JobsResponse struct {
	Success bool      `json:"success"`
	Message string    `json:"message,omitempty"`
	Jobs    []*db.Job `json:"jobs,omitempty"`
}

type JobResponse struct {
	Success bool     `json:"success"`
	Message string   `json:"message,omitempty"`
	Job     *db.Job  `json:"job,omitempty"`
}

// handleListJobs returns all jobs for the authenticated user
func handleListJobs(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if enableCORS(w, r) {
			return
		}

		if r.Method != http.MethodGet {
			writeJobsError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		// Get user ID from context (set by auth middleware)
		userID, ok := r.Context().Value("user_id").(int64)
		if !ok {
			writeJobsError(w, http.StatusUnauthorized, "User not authenticated")
			return
		}

		// Get jobs for user
		jobs, err := database.GetJobsByUserID(userID)
		if err != nil {
			writeJobsError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get jobs: %v", err))
			return
		}

		response := JobsResponse{
			Success: true,
			Jobs:    jobs,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// handleGetJob returns a single job by ID
func handleGetJob(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if enableCORS(w, r) {
			return
		}

		if r.Method != http.MethodGet {
			writeJobError(w, http.StatusMethodNotAllowed, "Method not allowed")
			return
		}

		// Get user ID from context
		userID, ok := r.Context().Value("user_id").(int64)
		if !ok {
			writeJobError(w, http.StatusUnauthorized, "User not authenticated")
			return
		}

		// Extract job ID from URL path
		// Expected format: /api/jobs/{id}
		path := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
		jobID, err := strconv.ParseInt(path, 10, 64)
		if err != nil {
			writeJobError(w, http.StatusBadRequest, "Invalid job ID")
			return
		}

		// Get job
		job, err := database.GetJobByID(jobID)
		if err != nil {
			writeJobError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get job: %v", err))
			return
		}

		if job == nil {
			writeJobError(w, http.StatusNotFound, "Job not found")
			return
		}

		// Verify ownership
		if job.UserID != userID {
			writeJobError(w, http.StatusForbidden, "Access denied")
			return
		}

		response := JobResponse{
			Success: true,
			Job:     job,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// handleDownloadResult streams the transcript file for download
func handleDownloadResult(database *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if enableCORS(w, r) {
			return
		}

		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Get user ID from context
		userID, ok := r.Context().Value("user_id").(int64)
		if !ok {
			http.Error(w, "User not authenticated", http.StatusUnauthorized)
			return
		}

		// Extract job ID from URL path
		// Expected format: /api/jobs/{id}/download
		path := strings.TrimPrefix(r.URL.Path, "/api/jobs/")
		path = strings.TrimSuffix(path, "/download")
		jobID, err := strconv.ParseInt(path, 10, 64)
		if err != nil {
			http.Error(w, "Invalid job ID", http.StatusBadRequest)
			return
		}

		// Get job
		job, err := database.GetJobByID(jobID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get job: %v", err), http.StatusInternalServerError)
			return
		}

		if job == nil {
			http.Error(w, "Job not found", http.StatusNotFound)
			return
		}

		// Verify ownership
		if job.UserID != userID {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}

		// Check if job is completed
		if job.Status != "completed" {
			http.Error(w, "Job not completed", http.StatusBadRequest)
			return
		}

		// Check if transcript path exists
		if job.TranscriptPath == nil || *job.TranscriptPath == "" {
			http.Error(w, "Transcript not available", http.StatusNotFound)
			return
		}

		// Open the transcript file
		file, err := os.Open(*job.TranscriptPath)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to open transcript: %v", err), http.StatusInternalServerError)
			return
		}
		defer file.Close()

		// Get file info for size
		fileInfo, err := file.Stat()
		if err != nil {
			http.Error(w, "Failed to get file info", http.StatusInternalServerError)
			return
		}

		// Set headers for file download
		filename := fmt.Sprintf("%s.%s", job.OriginalFilename, job.OutputFormat)
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

		// Stream the file
		_, err = io.Copy(w, file)
		if err != nil {
			// Can't send error response here since headers are already sent
			// Just log it
			fmt.Printf("Error streaming file: %v\n", err)
		}
	}
}

func writeJobsError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(JobsResponse{
		Success: false,
		Message: message,
	})
}

func writeJobError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(JobResponse{
		Success: false,
		Message: message,
	})
}
