package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/trevor/subtitler/internal/analytics"
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

// BatchUploadResponse is returned when uploading multiple files
type BatchUploadResponse struct {
	Success     bool         `json:"success"`
	Message     string       `json:"message"`
	Jobs        []JobResult  `json:"jobs,omitempty"`
	FailedFiles []FailedFile `json:"failed_files,omitempty"`
}

// JobResult represents a successfully created job
type JobResult struct {
	JobID    int64  `json:"job_id"`
	Filename string `json:"filename"`
}

// FailedFile represents a file that failed to process
type FailedFile struct {
	Filename string `json:"filename"`
	Error    string `json:"error"`
}

// handleUpload handles file upload and job creation
// POST /api/upload
// Requires authentication
// Supports single file or batch upload (multiple files)
func handleUpload(database *db.DB, workerPool *worker.WorkerPool, analyticsService *analytics.Service) http.HandlerFunc {
	// Valid formats map
	validFormats := map[string]bool{
		"srt":      true,
		"vtt":      true,
		"text":     true,
		"json":     true,
		"embedded": true,
	}

	// Valid languages map (ISO 639-1)
	validLanguages := map[string]bool{
		"en": true, "zh": true, "de": true, "es": true, "ru": true,
		"ko": true, "fr": true, "ja": true, "pt": true, "tr": true,
		"pl": true, "ca": true, "nl": true, "ar": true, "sv": true,
		"it": true, "id": true, "hi": true, "fi": true, "vi": true,
		"he": true, "uk": true, "el": true, "ms": true, "cs": true,
		"ro": true, "da": true, "hu": true, "ta": true, "no": true,
		"th": true, "ur": true, "hr": true, "bg": true, "lt": true,
		"la": true, "mi": true, "ml": true, "cy": true, "sk": true,
		"te": true, "fa": true, "lv": true, "bn": true, "sr": true,
		"az": true, "sl": true, "kn": true, "et": true, "mk": true,
		"br": true, "eu": true, "is": true, "hy": true, "ne": true,
		"mn": true, "bs": true, "kk": true, "sq": true, "sw": true,
		"gl": true, "mr": true, "pa": true, "si": true, "km": true,
		"sn": true, "yo": true, "so": true, "af": true, "oc": true,
		"ka": true, "be": true, "tg": true, "sd": true, "gu": true,
		"am": true, "yi": true, "lo": true, "uz": true, "fo": true,
		"ht": true, "ps": true, "tk": true, "nn": true, "mt": true,
		"sa": true, "lb": true, "my": true, "bo": true, "tl": true,
		"mg": true, "as": true, "tt": true, "haw": true, "ln": true,
		"ha": true, "ba": true, "jw": true, "su": true,
	}

	const maxFilesPerBatch = 10

	return func(w http.ResponseWriter, r *http.Request) {
		// Authenticate user
		userID, err := auth.GetUserIDFromRequest(r)
		if err != nil {
			respondJSON(w, http.StatusUnauthorized, BatchUploadResponse{
				Success: false,
				Message: "Unauthorized: " + err.Error(),
			})
			return
		}

		// Parse multipart form (max 200MB per file * 10 files = 2GB total)
		err = r.ParseMultipartForm(2000 * 1024 * 1024)
		if err != nil {
			respondJSON(w, http.StatusBadRequest, BatchUploadResponse{
				Success: false,
				Message: "Failed to parse form: " + err.Error(),
			})
			return
		}

		// Get files from form - supports both single and multiple
		files := r.MultipartForm.File["file"]
		if len(files) == 0 {
			respondJSON(w, http.StatusBadRequest, BatchUploadResponse{
				Success: false,
				Message: "No files provided",
			})
			return
		}

		// Limit number of files per batch
		if len(files) > maxFilesPerBatch {
			respondJSON(w, http.StatusBadRequest, BatchUploadResponse{
				Success: false,
				Message: fmt.Sprintf("Too many files. Maximum %d files per upload.", maxFilesPerBatch),
			})
			return
		}

		// Get format parameter from form (default: srt)
		formatStr := r.FormValue("format")
		if formatStr == "" {
			formatStr = "srt"
		}

		// Validate format
		if !validFormats[formatStr] {
			respondJSON(w, http.StatusBadRequest, BatchUploadResponse{
				Success: false,
				Message: fmt.Sprintf("Invalid format '%s'. Supported formats: srt, vtt, text, json, embedded", formatStr),
			})
			return
		}

		// Get language parameter from form (optional, empty = auto-detect)
		languageStr := r.FormValue("language")
		var language *string
		if languageStr != "" {
			if !validLanguages[languageStr] {
				respondJSON(w, http.StatusBadRequest, BatchUploadResponse{
					Success: false,
					Message: fmt.Sprintf("Invalid language code '%s'. Use ISO 639-1 codes (e.g., 'en', 'es', 'fr')", languageStr),
				})
				return
			}
			language = &languageStr
		}

		// Get visitor ID for analytics
		visitorID := r.Header.Get("X-Visitor-ID")
		if visitorID == "" {
			user, _ := database.GetUserByID(userID)
			if user != nil {
				visitorID = "server-" + strings.ReplaceAll(user.Email, "@", "-at-")
			}
		}

		// Process each file
		var successfulJobs []JobResult
		var failedFiles []FailedFile

		for _, header := range files {
			// Validate file
			if err := storage.ValidateFile(header.Filename, header.Size); err != nil {
				failedFiles = append(failedFiles, FailedFile{
					Filename: header.Filename,
					Error:    err.Error(),
				})
				continue
			}

			// Open the file
			file, err := header.Open()
			if err != nil {
				failedFiles = append(failedFiles, FailedFile{
					Filename: header.Filename,
					Error:    "Failed to open file: " + err.Error(),
				})
				continue
			}

			// Process this file
			jobID, err := processUploadedFile(database, workerPool, userID, file, header, formatStr, language)
			file.Close()

			if err != nil {
				failedFiles = append(failedFiles, FailedFile{
					Filename: header.Filename,
					Error:    err.Error(),
				})
				continue
			}

			successfulJobs = append(successfulJobs, JobResult{
				JobID:    jobID,
				Filename: header.Filename,
			})

			// Track upload_completed event
			if visitorID != "" && analyticsService != nil {
				go func(jID int64, fSize int64, fname string) {
					eventProps := map[string]interface{}{
						"job_id":        jID,
						"file_size":     fSize,
						"output_format": formatStr,
						"batch_upload":  len(files) > 1,
					}
					if language != nil {
						eventProps["language"] = *language
					}
					err := analyticsService.TrackEvent(r.Context(), visitorID, &userID, "upload_completed", eventProps, nil, nil, nil)
					if err != nil {
						log.Printf("Failed to track upload_completed event: %v", err)
					}
				}(jobID, header.Size, header.Filename)
			}

			log.Printf("Job %d created for user %d: %s (%d bytes)", jobID, userID, header.Filename, header.Size)
		}

		// Determine response status
		if len(successfulJobs) == 0 {
			// All files failed
			respondJSON(w, http.StatusBadRequest, BatchUploadResponse{
				Success:     false,
				Message:     "All files failed to upload",
				FailedFiles: failedFiles,
			})
			return
		}

		// At least one file succeeded
		message := fmt.Sprintf("%d file(s) uploaded successfully", len(successfulJobs))
		if len(failedFiles) > 0 {
			message += fmt.Sprintf(", %d file(s) failed", len(failedFiles))
		}

		respondJSON(w, http.StatusCreated, BatchUploadResponse{
			Success:     true,
			Message:     message,
			Jobs:        successfulJobs,
			FailedFiles: failedFiles,
		})
	}
}

// processUploadedFile handles creating a job and saving a single uploaded file
func processUploadedFile(database *db.DB, workerPool *worker.WorkerPool, userID int64, file io.Reader, header *multipart.FileHeader, formatStr string, language *string) (int64, error) {
	// Create job record with status='pending'
	job := &db.Job{
		UserID:           userID,
		Status:           "pending",
		OriginalFilename: header.Filename,
		FileSize:         header.Size,
		OutputFormat:     formatStr,
		Language:         language,
	}

	err := database.CreateJob(job)
	if err != nil {
		return 0, fmt.Errorf("failed to create job: %w", err)
	}

	// Build file path: data/files/uploads/{user_id}/{job_id}/{filename}
	filePath := buildUploadPath(job.UserID, job.ID, header.Filename)

	// Create directory if it doesn't exist
	uploadDir := filepath.Dir(filePath)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		database.UpdateJobFailed(job.ID, "Failed to create upload directory")
		return 0, fmt.Errorf("failed to create upload directory: %w", err)
	}

	// Save file to disk
	outFile, err := os.Create(filePath)
	if err != nil {
		database.UpdateJobFailed(job.ID, "Failed to create file")
		return 0, fmt.Errorf("failed to create file: %w", err)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, file)
	if err != nil {
		database.UpdateJobFailed(job.ID, "Failed to save file")
		return 0, fmt.Errorf("failed to save file: %w", err)
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
		database.UpdateJobFailed(job.ID, "Failed to enqueue job")
		return 0, fmt.Errorf("failed to enqueue job: %w", err)
	}

	return job.ID, nil
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
