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
	"time"

	"github.com/trevor/subtitler/backend/auth"
	"github.com/trevor/subtitler/backend/crypto"
	"github.com/trevor/subtitler/backend/db"
)

const (
	maxUploadSize = 500 << 20 // 500 MB
	uploadDir     = "uploads"
	dbPath        = "data/subtitler.db"
	keyPath       = "data/age.key"
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

// TranscriptionStatus tracks the state of a transcription job (API response format)
type TranscriptionStatus struct {
	Status   string         `json:"status"` // pending, processing, complete, error
	Message  string         `json:"message,omitempty"`
	Result   *WhisperResult `json:"result,omitempty"`
	Progress int            `json:"progress,omitempty"` // 0-100
}

// Global database connection
var database *db.DB

// Global encryptor for file encryption
var encryptor *crypto.Encryptor

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
	// First try to get from database
	if database != nil {
		video, err := database.GetVideo(uploadID)
		if err != nil {
			return "", err
		}
		if video != nil {
			return video.FilePath, nil
		}
	}

	// Fallback to glob search for backwards compatibility
	matches, err := filepath.Glob(filepath.Join(uploadDir, uploadID+".*"))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("video not found for upload ID: %s", uploadID)
	}
	return matches[0], nil
}

// dbTranscriptionToStatus converts a database transcription to API status format
func dbTranscriptionToStatus(t *db.Transcription) *TranscriptionStatus {
	if t == nil {
		return nil
	}

	status := &TranscriptionStatus{
		Status:   t.Status,
		Message:  t.Message,
		Progress: t.Progress,
	}

	if t.Status == "complete" {
		segments, _ := t.GetSegments()
		whisperSegments := make([]WhisperSegment, len(segments))
		for i, s := range segments {
			whisperSegments[i] = WhisperSegment{
				ID:    s.ID,
				Start: s.Start,
				End:   s.End,
				Text:  s.Text,
			}
		}
		status.Result = &WhisperResult{
			Language: t.Language,
			Duration: t.Duration,
			Text:     t.FullText,
			Segments: whisperSegments,
		}
	}

	return status
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

	// Initialize database
	var err error
	database, err = db.Open(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()
	log.Printf("Database initialized at %s", dbPath)

	// Initialize encryptor for file encryption at rest
	var isNewKey bool
	encryptor, isNewKey, err = crypto.LoadOrGenerateKey(keyPath)
	if err != nil {
		log.Fatalf("Failed to initialize encryption: %v", err)
	}
	if isNewKey {
		log.Printf("Generated new encryption key, saved to %s", keyPath)
	} else {
		log.Printf("Loaded encryption key from %s", keyPath)
	}
	log.Printf("Public key: %s", encryptor.PublicKey())

	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
		})
	})

	// Auth: Register new user
	mux.HandleFunc("POST /api/auth/register", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Parse request body
		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid request body",
			})
			return
		}

		// Normalize email
		req.Email = strings.TrimSpace(strings.ToLower(req.Email))

		// Validate email
		if err := auth.ValidateEmail(req.Email); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": err.Error(),
			})
			return
		}

		// Validate password
		if err := auth.ValidatePassword(req.Password); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": err.Error(),
			})
			return
		}

		// Check if email already exists
		existingUser, err := database.GetUserByEmail(req.Email)
		if err != nil {
			log.Printf("Error checking existing user: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Registration failed",
			})
			return
		}
		if existingUser != nil {
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Email already registered",
			})
			return
		}

		// Hash password
		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			log.Printf("Error hashing password: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Registration failed",
			})
			return
		}

		// Generate user ID
		userID, err := auth.GenerateID()
		if err != nil {
			log.Printf("Error generating user ID: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Registration failed",
			})
			return
		}

		// Create user
		user := &db.User{
			ID:           userID,
			Email:        req.Email,
			PasswordHash: hash,
			CreatedAt:    time.Now(),
		}
		if err := database.CreateUser(user); err != nil {
			log.Printf("Error creating user: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Registration failed",
			})
			return
		}

		// Create session
		session, err := auth.CreateSession(database, userID)
		if err != nil {
			log.Printf("Error creating session: %v", err)
			// User was created, but session failed - still return success
			// User can log in to get a session
			json.NewEncoder(w).Encode(map[string]interface{}{
				"user": map[string]interface{}{
					"id":         user.ID,
					"email":      user.Email,
					"created_at": user.CreatedAt,
				},
			})
			return
		}

		// Set session cookie
		auth.SetSessionCookie(w, session.Token, session.ExpiresAt)

		log.Printf("New user registered: %s", user.Email)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"user": map[string]interface{}{
				"id":         user.ID,
				"email":      user.Email,
				"created_at": user.CreatedAt,
			},
			"token": session.Token,
		})
	})

	// Auth: Login
	mux.HandleFunc("POST /api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Parse request body
		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid request body",
			})
			return
		}

		// Normalize email
		req.Email = strings.TrimSpace(strings.ToLower(req.Email))

		// Get user by email
		user, err := database.GetUserByEmail(req.Email)
		if err != nil {
			log.Printf("Error getting user: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Login failed",
			})
			return
		}
		if user == nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid email or password",
			})
			return
		}

		// Check password
		if !auth.CheckPassword(req.Password, user.PasswordHash) {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid email or password",
			})
			return
		}

		// Create session
		session, err := auth.CreateSession(database, user.ID)
		if err != nil {
			log.Printf("Error creating session: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Login failed",
			})
			return
		}

		// Set session cookie
		auth.SetSessionCookie(w, session.Token, session.ExpiresAt)

		log.Printf("User logged in: %s", user.Email)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"user": map[string]interface{}{
				"id":         user.ID,
				"email":      user.Email,
				"created_at": user.CreatedAt,
			},
			"token": session.Token,
		})
	})

	// Auth: Logout
	mux.HandleFunc("POST /api/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		token := auth.GetTokenFromRequest(r)
		if token != "" {
			if err := database.DeleteSession(token); err != nil {
				log.Printf("Error deleting session: %v", err)
			}
		}

		auth.ClearSessionCookie(w)

		json.NewEncoder(w).Encode(map[string]string{
			"message": "Logged out successfully",
		})
	})

	// Auth: Get current user
	mux.HandleFunc("GET /api/auth/me", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		token := auth.GetTokenFromRequest(r)
		user, _, err := auth.ValidateSession(database, token)
		if err != nil {
			log.Printf("Error validating session: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to get user",
			})
			return
		}

		if user == nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Not authenticated",
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"user": map[string]interface{}{
				"id":         user.ID,
				"email":      user.Email,
				"created_at": user.CreatedAt,
			},
		})
	})

	// Upload endpoint - accepts video files
	mux.HandleFunc("POST /api/upload", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Get authenticated user (if any)
		token := auth.GetTokenFromRequest(r)
		user, _, _ := auth.ValidateSession(database, token)

		// Get session_id from form for anonymous session tracking
		sessionID := r.URL.Query().Get("session_id")

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
		destFile.Close() // Close before encrypting

		log.Printf("Uploaded file: %s (%d bytes) -> %s", header.Filename, written, destPath)

		// Encrypt the file at rest
		encPath, err := encryptor.EncryptFile(destPath)
		if err != nil {
			log.Printf("Error encrypting file: %v", err)
			os.Remove(destPath)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to encrypt file",
			})
			return
		}

		// Remove the unencrypted file
		os.Remove(destPath)
		log.Printf("Encrypted file: %s -> %s", destPath, encPath)

		// Save video to database with encrypted file path
		video := &db.Video{
			ID:          uploadID,
			Filename:    header.Filename,
			Size:        written,
			ContentType: contentType,
			FilePath:    encPath,
			CreatedAt:   time.Now(),
		}
		// Set user_id if authenticated
		if user != nil {
			video.UserID = &user.ID
		}
		// Set session_id for anonymous tracking
		if sessionID != "" {
			video.SessionID = &sessionID
		}
		if err := database.CreateVideo(video); err != nil {
			log.Printf("Error saving video to database: %v", err)
			os.Remove(encPath) // Clean up encrypted file
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to save video record",
			})
			return
		}

		// Create initial transcription record
		transcription := &db.Transcription{
			ID:        generateID(),
			VideoID:   uploadID,
			Status:    "pending",
			Message:   "Video uploaded, ready for transcription",
			Progress:  0,
			CreatedAt: time.Now(),
		}
		if err := database.CreateTranscription(transcription); err != nil {
			log.Printf("Error creating transcription record: %v", err)
			// Don't fail the upload, transcription record can be created later
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

		// Check if already processing from database
		existingTranscription, err := database.GetTranscription(uploadID)
		if err != nil {
			log.Printf("Error getting transcription: %v", err)
		}
		if existingTranscription != nil {
			if existingTranscription.Status == "processing" {
				json.NewEncoder(w).Encode(map[string]string{
					"status":  "processing",
					"message": "Transcription already in progress",
				})
				return
			}
			if existingTranscription.Status == "complete" {
				json.NewEncoder(w).Encode(dbTranscriptionToStatus(existingTranscription))
				return
			}
		}

		// Create or update transcription record
		if existingTranscription == nil {
			transcription := &db.Transcription{
				ID:        generateID(),
				VideoID:   uploadID,
				Status:    "processing",
				Message:   "Extracting audio...",
				Progress:  10,
				CreatedAt: time.Now(),
			}
			if err := database.CreateTranscription(transcription); err != nil {
				log.Printf("Error creating transcription record: %v", err)
			}
		} else {
			database.UpdateTranscriptionStatus(uploadID, "processing", "Extracting audio...", 10)
		}

		// Process in background
		go func() {
			log.Printf("Starting transcription for %s", uploadID)

			// Decrypt video file if encrypted
			workingVideoPath := videoPath
			if strings.HasSuffix(videoPath, ".age") {
				database.UpdateTranscriptionStatus(uploadID, "processing", "Decrypting video...", 5)
				decryptedPath, err := encryptor.DecryptToTempFile(videoPath)
				if err != nil {
					log.Printf("Video decryption failed: %v", err)
					database.FailTranscription(uploadID, fmt.Sprintf("Video decryption failed: %v", err))
					return
				}
				workingVideoPath = decryptedPath
				defer os.Remove(decryptedPath) // Clean up decrypted file when done
			}

			// Extract audio
			database.UpdateTranscriptionStatus(uploadID, "processing", "Extracting audio...", 10)
			audioPath := filepath.Join(uploadDir, uploadID+".wav")
			if err := extractAudio(workingVideoPath, audioPath); err != nil {
				log.Printf("Audio extraction failed: %v", err)
				database.FailTranscription(uploadID, fmt.Sprintf("Audio extraction failed: %v", err))
				return
			}

			database.UpdateTranscriptionStatus(uploadID, "processing", "Running transcription...", 30)

			// Run whisper
			outputPath := filepath.Join(uploadDir, uploadID+"_transcript")
			result, err := transcribeAudio(audioPath, outputPath)
			if err != nil {
				log.Printf("Transcription failed: %v", err)
				database.FailTranscription(uploadID, fmt.Sprintf("Transcription failed: %v", err))
				return
			}

			// Convert segments to database format
			segments := make([]db.Segment, len(result.Segments))
			for i, s := range result.Segments {
				segments[i] = db.Segment{
					ID:    s.ID,
					Start: s.Start,
					End:   s.End,
					Text:  s.Text,
				}
			}

			// Success - save to database
			log.Printf("Transcription complete for %s: %d segments", uploadID, len(result.Segments))
			if err := database.CompleteTranscription(uploadID, result.Language, result.Duration, result.Text, segments); err != nil {
				log.Printf("Error saving transcription result: %v", err)
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

		transcription, err := database.GetTranscription(uploadID)
		if err != nil {
			log.Printf("Error getting transcription: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to get transcription status",
			})
			return
		}

		if transcription == nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "No transcription found for this upload",
			})
			return
		}

		json.NewEncoder(w).Encode(dbTranscriptionToStatus(transcription))
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

		transcription, err := database.GetTranscription(uploadID)
		if err != nil {
			log.Printf("Error getting transcription: %v", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to get transcription",
			})
			return
		}

		if transcription == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "No transcription found for this upload",
			})
			return
		}

		if transcription.Status != "complete" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error":  "Transcription not yet complete",
				"status": transcription.Status,
			})
			return
		}

		segments, err := transcription.GetSegments()
		if err != nil || len(segments) == 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "No subtitle segments available",
			})
			return
		}

		// Convert to WhisperResult for SRT generation
		whisperResult := &WhisperResult{
			Language: transcription.Language,
			Duration: transcription.Duration,
			Text:     transcription.FullText,
			Segments: make([]WhisperSegment, len(segments)),
		}
		for i, s := range segments {
			whisperResult.Segments[i] = WhisperSegment{
				ID:    s.ID,
				Start: s.Start,
				End:   s.End,
				Text:  s.Text,
			}
		}

		// Generate SRT content
		srtContent := generateSRT(whisperResult)

		// Set headers for file download
		w.Header().Set("Content-Type", "text/srt; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.srt\"", uploadID))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(srtContent))
	})

	// List all videos
	mux.HandleFunc("GET /api/videos", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Get authenticated user (if any)
		token := auth.GetTokenFromRequest(r)
		user, _, _ := auth.ValidateSession(database, token)

		// Get optional session_id from query params (for anonymous user filtering)
		sessionID := r.URL.Query().Get("session_id")
		var sessionPtr *string
		if sessionID != "" {
			sessionPtr = &sessionID
		}

		// Get user_id from authenticated session
		var userPtr *string
		if user != nil {
			userPtr = &user.ID
		}

		videos, err := database.ListVideos(userPtr, sessionPtr)
		if err != nil {
			log.Printf("Error listing videos: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Failed to list videos",
			})
			return
		}

		// Get transcription status for each video
		type VideoWithStatus struct {
			db.Video
			TranscriptionStatus string `json:"transcription_status"`
		}

		result := make([]VideoWithStatus, len(videos))
		for i, v := range videos {
			result[i] = VideoWithStatus{Video: v, TranscriptionStatus: "none"}
			if t, err := database.GetTranscription(v.ID); err == nil && t != nil {
				result[i].TranscriptionStatus = t.Status
			}
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"videos": result,
		})
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

		// If file is encrypted, decrypt to temp file for serving
		// (http.ServeFile needs seekable file for range requests)
		if strings.HasSuffix(videoPath, ".age") {
			decryptedPath, err := encryptor.DecryptToTempFile(videoPath)
			if err != nil {
				log.Printf("Failed to decrypt video for serving: %v", err)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]string{
					"error": "Failed to decrypt video",
				})
				return
			}
			defer os.Remove(decryptedPath)
			videoPath = decryptedPath
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
