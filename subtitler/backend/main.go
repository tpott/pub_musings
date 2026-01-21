package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	maxUploadSize = 500 << 20 // 500 MB
	uploadDir     = "uploads"
)

// WhisperSegment represents a transcribed segment with timing
type WhisperSegment struct {
	ID    int     `json:"id"`
	Start float64 `json:"start"` // start time in seconds
	End   float64 `json:"end"`   // end time in seconds
	Text  string  `json:"text"`
}

// WhisperResult represents the full transcription result
type WhisperResult struct {
	Language string           `json:"language"`
	Duration float64          `json:"duration"`
	Text     string           `json:"text"`
	Segments []WhisperSegment `json:"segments"`
}

// TranscriptionStatus tracks the state of a transcription job
type TranscriptionStatus struct {
	Status   string         `json:"status"` // pending, processing, complete, error
	Message  string         `json:"message,omitempty"`
	Result   *WhisperResult `json:"result,omitempty"`
	Progress int            `json:"progress,omitempty"` // 0-100
}

// In-memory storage for transcription statuses (will be replaced with SQLite)
var transcriptions = make(map[string]*TranscriptionStatus)

func generateID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// getWhisperModel returns the whisper model path from env or default
func getWhisperModel() string {
	model := os.Getenv("WHISPER_MODEL")
	if model == "" {
		// Default to medium model - adjust path as needed
		model = os.ExpandEnv("$HOME/Github/whisper.cpp/models/ggml-medium.bin")
	}
	return model
}

// extractAudio uses ffmpeg to extract audio from video as WAV
func extractAudio(videoPath, audioPath string) error {
	cmd := exec.Command("ffmpeg",
		"-i", videoPath,
		"-vn",                 // no video
		"-acodec", "pcm_s16le", // WAV format
		"-ar", "16000",        // 16kHz sample rate (whisper expects this)
		"-ac", "1",            // mono
		"-y",                  // overwrite output
		audioPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg error: %v, output: %s", err, string(output))
	}
	return nil
}

// transcribeAudio runs whisper-cli on the audio file
func transcribeAudio(audioPath, outputPath string) (*WhisperResult, error) {
	model := getWhisperModel()

	// Check if model exists
	if _, err := os.Stat(model); os.IsNotExist(err) {
		return nil, fmt.Errorf("whisper model not found at %s - set WHISPER_MODEL env var", model)
	}

	// Run whisper-cli with JSON output
	cmd := exec.Command("whisper-cli",
		"-m", model,
		"-f", audioPath,
		"-oj",               // output JSON
		"-of", outputPath,   // output file (without extension, whisper adds .json)
		"-t", "4",           // 4 threads
		"-l", "auto",        // auto-detect language
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

// findVideoFile finds the video file for an upload ID
func findVideoFile(uploadID string) (string, error) {
	// Look for video file with any extension
	matches, err := filepath.Glob(filepath.Join(uploadDir, uploadID+".*"))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("video not found for upload ID: %s", uploadID)
	}
	return matches[0], nil
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Ensure upload directory exists
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Fatalf("Failed to create upload directory: %v", err)
	}

	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
		})
	})

	// Upload endpoint - accepts video files
	mux.HandleFunc("POST /api/upload", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Limit request body size
		r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

		// Parse multipart form
		if err := r.ParseMultipartForm(maxUploadSize); err != nil {
			log.Printf("Error parsing form: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "File too large or invalid form data",
			})
			return
		}

		// Get the file from the form
		file, header, err := r.FormFile("video")
		if err != nil {
			log.Printf("Error getting form file: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "No video file provided",
			})
			return
		}
		defer file.Close()

		// Validate file type by checking content type
		contentType := header.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "video/") {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "File must be a video",
			})
			return
		}

		// Generate unique ID for this upload
		uploadID := generateID()

		// Get file extension from original filename
		ext := filepath.Ext(header.Filename)
		if ext == "" {
			ext = ".mp4" // default extension
		}

		// Create destination file
		destPath := filepath.Join(uploadDir, uploadID+ext)
		destFile, err := os.Create(destPath)
		if err != nil {
			log.Printf("Error creating destination file: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to save file",
			})
			return
		}
		defer destFile.Close()

		// Copy the uploaded file to destination
		written, err := io.Copy(destFile, file)
		if err != nil {
			log.Printf("Error copying file: %v", err)
			os.Remove(destPath) // Clean up partial file
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to save file",
			})
			return
		}

		log.Printf("Uploaded file: %s (%d bytes) -> %s", header.Filename, written, destPath)

		// Initialize transcription status as pending
		transcriptions[uploadID] = &TranscriptionStatus{
			Status:  "pending",
			Message: "Video uploaded, ready for transcription",
		}

		// Return success with upload ID
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "success",
			"upload_id": uploadID,
			"filename":  header.Filename,
			"size":      written,
			"message":   fmt.Sprintf("File uploaded successfully (%d bytes)", written),
		})
	})

	// Start transcription for an upload
	mux.HandleFunc("POST /api/transcribe/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		uploadID := r.PathValue("id")
		if uploadID == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Upload ID required",
			})
			return
		}

		// Find the video file
		videoPath, err := findVideoFile(uploadID)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": err.Error(),
			})
			return
		}

		// Check if already processing
		if status, exists := transcriptions[uploadID]; exists {
			if status.Status == "processing" {
				json.NewEncoder(w).Encode(map[string]string{
					"status":  "processing",
					"message": "Transcription already in progress",
				})
				return
			}
			if status.Status == "complete" {
				json.NewEncoder(w).Encode(status)
				return
			}
		}

		// Update status to processing
		transcriptions[uploadID] = &TranscriptionStatus{
			Status:   "processing",
			Message:  "Extracting audio...",
			Progress: 10,
		}

		// Process in background
		go func() {
			log.Printf("Starting transcription for %s", uploadID)

			// Extract audio
			audioPath := filepath.Join(uploadDir, uploadID+".wav")
			if err := extractAudio(videoPath, audioPath); err != nil {
				log.Printf("Audio extraction failed: %v", err)
				transcriptions[uploadID] = &TranscriptionStatus{
					Status:  "error",
					Message: fmt.Sprintf("Audio extraction failed: %v", err),
				}
				return
			}

			transcriptions[uploadID].Message = "Running transcription..."
			transcriptions[uploadID].Progress = 30

			// Run whisper
			outputPath := filepath.Join(uploadDir, uploadID+"_transcript")
			result, err := transcribeAudio(audioPath, outputPath)
			if err != nil {
				log.Printf("Transcription failed: %v", err)
				transcriptions[uploadID] = &TranscriptionStatus{
					Status:  "error",
					Message: fmt.Sprintf("Transcription failed: %v", err),
				}
				return
			}

			// Success!
			log.Printf("Transcription complete for %s: %d segments", uploadID, len(result.Segments))
			transcriptions[uploadID] = &TranscriptionStatus{
				Status:   "complete",
				Message:  "Transcription complete",
				Progress: 100,
				Result:   result,
			}

			// Clean up intermediate files
			os.Remove(audioPath)
		}()

		// Return immediately with processing status
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "processing",
			"message": "Transcription started",
		})
	})

	// Get transcription status/result
	mux.HandleFunc("GET /api/transcribe/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		uploadID := r.PathValue("id")
		if uploadID == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Upload ID required",
			})
			return
		}

		status, exists := transcriptions[uploadID]
		if !exists {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "No transcription found for this upload",
			})
			return
		}

		json.NewEncoder(w).Encode(status)
	})

	// Download SRT file for a transcription
	mux.HandleFunc("GET /api/videos/{id}/subtitles.srt", func(w http.ResponseWriter, r *http.Request) {
		uploadID := r.PathValue("id")
		if uploadID == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Upload ID required",
			})
			return
		}

		status, exists := transcriptions[uploadID]
		if !exists {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "No transcription found for this upload",
			})
			return
		}

		if status.Status != "complete" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error":  "Transcription not yet complete",
				"status": status.Status,
			})
			return
		}

		if status.Result == nil || len(status.Result.Segments) == 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "No subtitle segments available",
			})
			return
		}

		// Generate SRT content
		srtContent := generateSRT(status.Result)

		// Set headers for file download
		w.Header().Set("Content-Type", "text/srt; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.srt\"", uploadID))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(srtContent))
	})

	// Serve uploaded video files for playback
	mux.HandleFunc("GET /api/videos/{id}/video", func(w http.ResponseWriter, r *http.Request) {
		uploadID := r.PathValue("id")
		if uploadID == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Upload ID required",
			})
			return
		}

		// Find the video file
		videoPath, err := findVideoFile(uploadID)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": err.Error(),
			})
			return
		}

		// Serve the file
		http.ServeFile(w, r, videoPath)
	})

	log.Printf("Backend server starting on :%s", port)
	log.Printf("Using whisper model: %s", getWhisperModel())
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
