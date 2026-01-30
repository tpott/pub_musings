package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tpott/subtitler/backend/align"
	"github.com/tpott/subtitler/backend/audio"
	"github.com/tpott/subtitler/backend/db"
	"github.com/tpott/subtitler/backend/errmsg"
	"github.com/tpott/subtitler/backend/httputil"
	"github.com/tpott/subtitler/backend/language"
	"github.com/tpott/subtitler/backend/logging"
	"github.com/tpott/subtitler/backend/metrics"
	"github.com/tpott/subtitler/backend/script"
	"github.com/tpott/subtitler/backend/validation"
)

func registerTranscriptionHandlers(mux *http.ServeMux) { //nolint:funlen // route registration
	// Start transcription for an upload (rate limited: 5/min per IP)
	// Optional query parameter: language (ISO 639-1 code, e.g., "en", "es", "ja")
	// If not provided or "auto", whisper will auto-detect the language
	mux.HandleFunc("POST /api/transcribe/{id}", transcribeLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {

		uploadID, valid := validatePathID(w, r.PathValue("id"), "Upload ID")
		if !valid {
			return
		}

		// Parse optional language parameter (defaults to "auto" for auto-detection)
		language := r.URL.Query().Get("language")
		if language == "" {
			language = "auto"
		}
		// Validate language code length
		if err := validation.ValidateLanguageCode(language); err != nil {
			httputil.RespondError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Find the video file and get key version for decryption
		video, err := getVideoForDecryption(uploadID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Video not found for transcription", "error", err, "upload_id", uploadID)
			httputil.RespondError(w, http.StatusNotFound, errmsg.ForVideoNotFound(err))
			return
		}
		videoPath := video.FilePath
		keyVersion := video.KeyVersion

		// Check for force parameter (used when re-transcribing with different language)
		force := r.URL.Query().Get("force") == "true"

		// Check if already processing from database
		existingTranscription, err := database.GetTranscription(uploadID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting transcription", "error", err)
		}
		if existingTranscription != nil {
			if existingTranscription.Status == "processing" {
				httputil.RespondJSON(w, http.StatusOK, map[string]string{
					"status":  "processing",
					"message": "Transcription already in progress",
				})
				return
			}
			if existingTranscription.Status == "complete" {
				// Check if the requested language is different from the existing one
				// or if force is explicitly requested
				languageChanged := language != "auto" && existingTranscription.Language != language
				if !force && !languageChanged {
					// Same language, return existing result
					httputil.RespondJSON(w, http.StatusOK, dbTranscriptionToStatus(existingTranscription))
					return
				}
				// Language changed or force requested - proceed with re-transcription
				logging.Info("Re-transcribing video",
					"upload_id", uploadID,
					"old_language", existingTranscription.Language,
					"new_language", language,
					"force", force)
			}
		}

		// Create or update transcription record
		if existingTranscription == nil {
			newTranscriptionID, err := generateID()
			if err != nil {
				logging.ErrorContext(r.Context(), "Error generating transcription ID", "error", err)
				httputil.RespondError(w, http.StatusInternalServerError, "Failed to generate transcription ID")
				return
			}
			transcription := &db.Transcription{
				ID:        newTranscriptionID,
				VideoID:   uploadID,
				Status:    "processing",
				Message:   "Extracting audio...",
				Progress:  10,
				CreatedAt: time.Now(),
			}
			if err := database.CreateTranscription(transcription); err != nil {
				logging.ErrorContext(r.Context(), "Error creating transcription record", "error", err)
			}
		} else {
			if err := database.UpdateTranscriptionStatus(uploadID, "processing", "Extracting audio...", 10); err != nil {
				logging.Error("Failed to update transcription status", "video_id", uploadID, "error", err)
			}
		}

		// Process in background (capture language and key version in closure)
		go func(lang string, kv int) {
			// Panic recovery to prevent goroutine crashes from leaving jobs in stuck state
			defer func() {
				if r := recover(); r != nil {
					logging.Error("Panic in transcription goroutine", "upload_id", uploadID, "panic", r)
					metrics.RecordTranscriptionFailed()
					if err := database.FailTranscription(uploadID, "Internal error: transcription process crashed"); err != nil {
						logging.Error("Failed to mark transcription as failed", "video_id", uploadID, "error", err)
					}
				}
			}()

			// Check for shutdown before starting
			if isShuttingDown() {
				logging.Info("Transcription cancelled due to server shutdown", "upload_id", uploadID)
				if err := database.FailTranscription(uploadID, "Server shutting down - transcription interrupted"); err != nil {
					logging.Error("Failed to mark transcription as failed", "video_id", uploadID, "error", err)
				}
				return
			}

			transcriptionStart := time.Now()
			metrics.RecordTranscriptionStarted()
			logging.Info("Starting transcription", "upload_id", uploadID, "language", lang, "key_version", kv)

			// Decrypt video file if encrypted
			workingVideoPath := videoPath
			if strings.HasSuffix(videoPath, ".age") {
				if err := database.UpdateTranscriptionStatus(uploadID, "processing", "Decrypting video...", 5); err != nil {
					logging.Error("Failed to update transcription status", "video_id", uploadID, "error", err)
				}
				decryptedPath, err := multiEnc.DecryptToTempFile(videoPath, kv)
				if err != nil {
					logging.Error("Video decryption failed", "error", err)
					metrics.RecordTranscriptionFailed()
					if err := database.FailTranscription(uploadID, fmt.Sprintf("Video decryption failed: %v", err)); err != nil {
						logging.Error("Failed to mark transcription as failed", "video_id", uploadID, "error", err)
					}
					return
				}
				workingVideoPath = decryptedPath
				defer removeWithLogging(decryptedPath, "decrypted video cleanup after transcription")
			}

			// Check for shutdown after decryption
			if isShuttingDown() {
				logging.Info("Transcription cancelled due to server shutdown", "upload_id", uploadID)
				if err := database.FailTranscription(uploadID, "Server shutting down - transcription interrupted"); err != nil {
					logging.Error("Failed to mark transcription as failed", "video_id", uploadID, "error", err)
				}
				return
			}

			// Extract audio
			if err := database.UpdateTranscriptionStatus(uploadID, "processing", "Extracting audio...", 10); err != nil {
				logging.Error("Failed to update transcription status", "video_id", uploadID, "error", err)
			}
			audioPath := filepath.Join(uploadDir, uploadID+".wav")
			if err := audioExtractor.ExtractAudio(workingVideoPath, audioPath); err != nil {
				logging.Error("Audio extraction failed", "error", err)
				metrics.RecordTranscriptionFailed()
				if err := database.FailTranscription(uploadID, fmt.Sprintf("Audio extraction failed: %v", err)); err != nil {
					logging.Error("Failed to mark transcription as failed", "video_id", uploadID, "error", err)
				}
				return
			}
			defer removeWithLogging(audioPath, "audio file cleanup after transcription")

			// Check for shutdown after audio extraction
			if isShuttingDown() {
				logging.Info("Transcription cancelled due to server shutdown", "upload_id", uploadID)
				if err := database.FailTranscription(uploadID, "Server shutting down - transcription interrupted"); err != nil {
					logging.Error("Failed to mark transcription as failed", "video_id", uploadID, "error", err)
				}
				return
			}

			if err := database.UpdateTranscriptionStatus(uploadID, "processing", "Running transcription...", 30); err != nil {
				logging.Error("Failed to update transcription status", "video_id", uploadID, "error", err)
			}

			// Start a goroutine to simulate progress updates during transcription
			// Since whisper doesn't provide progress callbacks, we estimate based on time
			// Use context for cancellation so it's safe to call cancel multiple times
			// (e.g., from defer recover and normal completion)
			progressCtx, cancelProgress := context.WithCancel(context.Background())
			defer cancelProgress() // Ensure cleanup even if we panic
			go func() {
				ticker := time.NewTicker(5 * time.Second)
				defer ticker.Stop()
				progress := 30
				for {
					select {
					case <-progressCtx.Done():
						return
					case <-shutdownCtx.Done():
						return
					case <-ticker.C:
						// Increment progress slowly from 30% to 90% during transcription
						if progress < 90 {
							progress += 5
							if err := database.UpdateTranscriptionStatus(uploadID, "processing", "Transcribing audio...", progress); err != nil {
								logging.Error("Failed to update transcription status", "video_id", uploadID, "error", err)
							}
						}
					}
				}
			}()

			// Run whisper (server or CLI based on configuration)
			outputPath := filepath.Join(uploadDir, uploadID+"_transcript")
			result, err := transcribe(audioPath, outputPath, lang)
			cancelProgress() // Stop progress simulation

			if err != nil {
				logging.Error("Transcription failed", "error", err)
				metrics.RecordTranscriptionFailed()
				if err := database.FailTranscription(uploadID, fmt.Sprintf("Transcription failed: %v", err)); err != nil {
					logging.Error("Failed to mark transcription as failed", "video_id", uploadID, "error", err)
				}
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

			// Success - save to database and record metrics
			transcriptionDuration := time.Since(transcriptionStart)
			metrics.RecordTranscriptionCompleted(transcriptionDuration)
			logging.Info("Transcription complete", "upload_id", uploadID, "segment_count", len(result.Segments), "duration_sec", transcriptionDuration.Seconds())
			if err := database.CompleteTranscription(uploadID, result.Language, result.Duration, result.Text, segments); err != nil {
				logging.Error("Error saving transcription result", "error", err)
			}
			// Note: audioPath is cleaned up by defer above
		}(language, keyVersion)

		// Return immediately with processing status
		httputil.RespondJSON(w, http.StatusOK, map[string]string{
			"status":  "processing",
			"message": "Transcription started",
		})
	}))

	// Get transcription status/result
	mux.HandleFunc("GET /api/transcribe/{id}", func(w http.ResponseWriter, r *http.Request) {
		uploadID, valid := validatePathID(w, r.PathValue("id"), "Upload ID")
		if !valid {
			return
		}

		transcription, err := database.GetTranscription(uploadID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting transcription", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to get transcription status")
			return
		}

		if transcription == nil {
			httputil.RespondError(w, http.StatusNotFound, "No transcription found for this upload")
			return
		}

		status := dbTranscriptionToStatus(transcription)

		// Include embedded subtitles info from the video
		video, err := database.GetVideo(uploadID)
		if err == nil && video != nil && video.EmbeddedSubtitlesJSON != nil {
			var tracks []audio.SubtitleTrack
			if err := json.Unmarshal([]byte(*video.EmbeddedSubtitlesJSON), &tracks); err == nil {
				status.EmbeddedSubtitles = tracks
			}
		}

		httputil.RespondJSON(w, http.StatusOK, status)
	})

	// Update segments for a transcription (edit subtitles)
	mux.HandleFunc("PUT /api/transcribe/{id}/segments", func(w http.ResponseWriter, r *http.Request) {
		uploadID, valid := validatePathID(w, r.PathValue("id"), "Upload ID")
		if !valid {
			return
		}

		// Check if transcription exists and is complete
		transcription, err := database.GetTranscription(uploadID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting transcription", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to get transcription")
			return
		}
		if transcription == nil {
			httputil.RespondError(w, http.StatusNotFound, "No transcription found for this upload")
			return
		}
		if transcription.Status != "complete" {
			httputil.RespondJSON(w, http.StatusBadRequest, map[string]string{
				"error":  "Cannot edit segments - transcription not complete",
				"status": transcription.Status,
			})
			return
		}

		// Parse request body
		var req struct {
			Segments []db.Segment `json:"segments"`
		}
		if err := httputil.DecodeJSONBody(r, w, &req, 0); err != nil {
			if err.Error() == "http: request body too large" {
				httputil.RespondError(w, http.StatusRequestEntityTooLarge, "Request body too large")
			} else {
				httputil.RespondError(w, http.StatusBadRequest, "Invalid request body")
			}
			return
		}

		// Validate segments
		for i, seg := range req.Segments {
			if seg.Start < 0 || seg.End < 0 {
				httputil.RespondErrorf(w, http.StatusBadRequest, "Segment %d has invalid timing (negative values)", i)
				return
			}
			if seg.Start > seg.End {
				httputil.RespondErrorf(w, http.StatusBadRequest, "Segment %d: start time cannot be greater than end time", i)
				return
			}
			// Validate segment text length
			if err := validation.ValidateSegmentText(seg.Text); err != nil {
				httputil.RespondErrorf(w, http.StatusBadRequest, "Segment %d: %v", i, err)
				return
			}
		}

		// Update segments in database
		if err := database.UpdateSegments(uploadID, req.Segments); err != nil {
			logging.ErrorContext(r.Context(), "Error updating segments", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to update segments")
			return
		}

		logging.InfoContext(r.Context(), "Updated segments", "upload_id", uploadID, "segment_count", len(req.Segments))
		httputil.RespondJSON(w, http.StatusOK, map[string]interface{}{
			"status":   "success",
			"segments": len(req.Segments),
		})
	})

	// Paste-and-match: align user-provided transcript with whisper timing
	mux.HandleFunc("POST /api/transcribe/{id}/align", func(w http.ResponseWriter, r *http.Request) {

		uploadID, valid := validatePathID(w, r.PathValue("id"), "Upload ID")
		if !valid {
			return
		}

		// Check if transcription exists and is complete
		transcription, err := database.GetTranscription(uploadID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting transcription", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to get transcription")
			return
		}
		if transcription == nil {
			httputil.RespondError(w, http.StatusNotFound, "No transcription found for this upload")
			return
		}
		if transcription.Status != "complete" {
			httputil.RespondJSON(w, http.StatusBadRequest, map[string]string{
				"error":  "Cannot align - transcription not complete",
				"status": transcription.Status,
			})
			return
		}

		// Parse request body
		var req struct {
			Text            string `json:"text"`
			Mode            string `json:"mode"`              // "lyrics" for music-specific alignment
			ConvertToScript string `json:"convert_to_script"` // Optional: target script (e.g., "Devanagari")
			Language        string `json:"language"`          // Required if convert_to_script is set
		}
		if err := httputil.DecodeJSONBody(r, w, &req, 0); err != nil {
			if err.Error() == "http: request body too large" {
				httputil.RespondError(w, http.StatusRequestEntityTooLarge, "Request body too large")
			} else {
				httputil.RespondError(w, http.StatusBadRequest, "Invalid request body")
			}
			return
		}

		// Validate align text length
		if err := validation.ValidateAlignText(req.Text); err != nil {
			httputil.RespondError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Validate align mode
		if _, err := validation.ValidateAlignMode(req.Mode); err != nil {
			httputil.RespondError(w, http.StatusBadRequest, err.Error())
			return
		}

		// Script conversion if requested
		scriptConverted := false
		var targetScript script.Script
		if req.ConvertToScript != "" {
			targetScript = script.Script(req.ConvertToScript)
			if !script.IsScriptSupported(targetScript) {
				httputil.RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
					"error":             "Unsupported target script",
					"supported_scripts": script.SupportedScripts(),
				})
				return
			}
			if !script.IsLanguageSupported(req.Language) {
				httputil.RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
					"error":               "Language required for script conversion",
					"supported_languages": script.SupportedLanguages(),
				})
				return
			}
		}

		// Get existing segments
		existingSegments, err := transcription.GetSegments()
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting segments", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to get existing segments")
			return
		}

		// Convert db.Segment to align.Segment
		alignSegments := make([]align.Segment, len(existingSegments))
		for i, s := range existingSegments {
			alignSegments[i] = align.Segment{
				ID:    s.ID,
				Start: s.Start,
				End:   s.End,
				Text:  s.Text,
			}
		}

		// Perform alignment - use lyrics mode if specified
		var result align.AlignmentResult
		if req.Mode == "lyrics" {
			result = align.AlignLyrics(req.Text, alignSegments)
		} else {
			result = align.AlignTranscript(req.Text, alignSegments)
		}

		// Convert back to db.Segment
		newSegments := make([]db.Segment, len(result.Segments))
		for i, s := range result.Segments {
			newSegments[i] = db.Segment{
				ID:    s.ID,
				Start: s.Start,
				End:   s.End,
				Text:  s.Text,
			}
		}

		// Apply script conversion if requested
		var failedConversionIndices []int
		if req.ConvertToScript != "" {
			converter := script.NewConverter()
			for i := range newSegments {
				converted, err := converter.Convert(newSegments[i].Text, req.Language, targetScript)
				if err == nil {
					newSegments[i].Text = converted
				} else {
					// Track which segments failed conversion
					failedConversionIndices = append(failedConversionIndices, i)
					logging.WarnContext(r.Context(), "Script conversion failed for segment", "segment_index", i, "error", err)
				}
			}
			scriptConverted = true
			if len(failedConversionIndices) > 0 {
				logging.WarnContext(r.Context(), "Script conversion had failures", "target_script", targetScript, "upload_id", uploadID, "failed_count", len(failedConversionIndices))
			} else {
				logging.InfoContext(r.Context(), "Applied script conversion", "target_script", targetScript, "upload_id", uploadID)
			}
		}

		// Update segments in database
		if err := database.UpdateSegments(uploadID, newSegments); err != nil {
			logging.ErrorContext(r.Context(), "Error updating segments", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to save aligned segments")
			return
		}

		mode := "standard"
		if req.Mode == "lyrics" {
			mode = "lyrics"
		}
		logging.InfoContext(r.Context(), "Aligned transcript", "upload_id", uploadID, "mode", mode, "segment_count", len(newSegments), "match_rate", result.Stats.MatchRate*100)

		response := map[string]interface{}{
			"status":   "success",
			"segments": len(newSegments),
			"stats":    result.Stats,
			"mode":     mode,
		}
		if scriptConverted {
			response["script_converted"] = true
			response["target_script"] = string(targetScript)
			// Include failed segment indices so frontend can display which need manual review
			if len(failedConversionIndices) > 0 {
				response["conversion_failed_indices"] = failedConversionIndices
			}
		}
		httputil.RespondJSON(w, http.StatusOK, response)
	})
}

// assembleChunks copies chunk files into a single destination file in order.
// Returns the total bytes written or an error. On error, the partial file is cleaned up.
func assembleChunks(ctx context.Context, chunks []db.UploadChunk, destPath string) (int64, error) {
	destFile, err := os.Create(destPath)
	if err != nil {
		logging.ErrorContext(ctx, "Error creating destination file", "error", err)
		return 0, fmt.Errorf("Failed to create destination file")
	}

	var totalWritten int64
	for _, chunk := range chunks {
		chunkFile, err := os.Open(chunk.ChunkPath)
		if err != nil {
			if closeErr := destFile.Close(); closeErr != nil {
				logging.WarnContext(ctx, "Error closing dest file after chunk open failure", "error", closeErr)
			}
			removeWithLogging(destPath, "partial assembled file after chunk open failure")
			logging.ErrorContext(ctx, "Error opening chunk file", "chunk_index", chunk.ChunkIndex, "error", err)
			return 0, fmt.Errorf("Failed to read chunk")
		}
		written, err := io.Copy(destFile, chunkFile)
		if closeErr := chunkFile.Close(); closeErr != nil {
			logging.WarnContext(ctx, "Error closing chunk file", "chunk_index", chunk.ChunkIndex, "error", closeErr)
		}
		if err != nil {
			if closeErr := destFile.Close(); closeErr != nil {
				logging.WarnContext(ctx, "Error closing dest file after copy failure", "error", closeErr)
			}
			removeWithLogging(destPath, "partial assembled file after copy failure")
			logging.ErrorContext(ctx, "Error copying chunk", "chunk_index", chunk.ChunkIndex, "error", err)
			return 0, fmt.Errorf("Failed to assemble file")
		}
		totalWritten += written
	}

	if err := destFile.Close(); err != nil {
		logging.WarnContext(ctx, "Error closing assembled file", "path", destPath, "error", err)
	}
	return totalWritten, nil
}

// processVideoMetadata generates a thumbnail, detects embedded subtitles,
// and detects language hints from the video file before encryption.
func processVideoMetadata(ctx context.Context, destPath, uploadID, filename string) (*string, *string, *language.DetectionResult) {
	// Generate thumbnail
	thumbPath := filepath.Join(uploadDir, uploadID+"_thumb.jpg")
	var encThumbPath *string
	if err := audio.GenerateThumbnail(destPath, thumbPath); err != nil {
		logging.WarnContext(ctx, "Failed to generate thumbnail", "path", destPath, "error", err)
	} else {
		encPath, _, err := multiEnc.EncryptFile(thumbPath)
		if err != nil {
			logging.WarnContext(ctx, "Failed to encrypt thumbnail", "error", err)
			removeWithLogging(thumbPath, "unencrypted thumbnail after encryption failure")
		} else {
			removeWithLogging(thumbPath, "unencrypted thumbnail after successful encryption")
			encThumbPath = &encPath
		}
	}

	// Detect embedded subtitle tracks
	var embeddedSubtitlesJSON *string
	subtitleTracks, err := audio.GetSubtitleTracks(destPath)
	if err != nil {
		logging.WarnContext(ctx, "Failed to detect embedded subtitles", "error", err)
	} else if len(subtitleTracks) > 0 {
		subtitlesData, err := json.Marshal(subtitleTracks)
		if err != nil {
			logging.WarnContext(ctx, "Failed to serialize subtitle tracks", "error", err)
		} else {
			jsonStr := string(subtitlesData)
			embeddedSubtitlesJSON = &jsonStr
			logging.InfoContext(ctx, "Detected embedded subtitle tracks", "count", len(subtitleTracks))
		}
	}

	// Detect language hints from video metadata and filename
	languageHints := language.Detect(destPath, filename)
	if languageHints != nil && len(languageHints.Hints) > 0 {
		logging.InfoContext(ctx, "Detected language hints",
			"suggested_language", languageHints.SuggestedLanguage,
			"confidence", languageHints.SuggestedConfidence,
			"hint_count", len(languageHints.Hints))
	}

	return encThumbPath, embeddedSubtitlesJSON, languageHints
}

// encryptAndPersistVideo encrypts the assembled video file and saves
// the video record to the database. On error, encrypted files are cleaned up.
func encryptAndPersistVideo(ctx context.Context, destPath, uploadID string, totalWritten int64, session *db.UploadSession, encThumbPath, embeddedSubtitlesJSON *string) error {
	encPath, keyVersion, err := multiEnc.EncryptFile(destPath)
	if err != nil {
		logging.ErrorContext(ctx, "Error encrypting file", "error", err)
		removeWithLogging(destPath, "unencrypted assembled file after encryption failure")
		return fmt.Errorf("Failed to encrypt file")
	}
	removeWithLogging(destPath, "unencrypted assembled file after successful encryption")

	video := &db.Video{
		ID:                    uploadID,
		Filename:              session.Filename,
		Size:                  totalWritten,
		ContentType:           session.ContentType,
		FilePath:              encPath,
		ThumbnailPath:         encThumbPath,
		KeyVersion:            keyVersion,
		EmbeddedSubtitlesJSON: embeddedSubtitlesJSON,
		CreatedAt:             time.Now(),
		UserID:                session.UserID,
		SessionID:             session.SessionID,
	}
	if err := database.CreateVideo(video); err != nil {
		logging.ErrorContext(ctx, "Error saving video to database", "error", err)
		removeWithLogging(encPath, "encrypted file after DB save failure")
		if encThumbPath != nil {
			removeWithLogging(*encThumbPath, "encrypted thumbnail after DB save failure")
		}
		return fmt.Errorf("Failed to save video record")
	}

	return nil
}

// finalizeUploadSession creates the transcription record, marks the upload
// session as complete, and cleans up chunk files.
func finalizeUploadSession(ctx context.Context, uploadID, sessionID string, chunks []db.UploadChunk) {
	// Create transcription record
	transcriptionID, err := generateID()
	if err != nil {
		logging.ErrorContext(ctx, "Error generating transcription ID", "error", err)
	} else {
		transcription := &db.Transcription{
			ID:        transcriptionID,
			VideoID:   uploadID,
			Status:    "pending",
			Message:   "Video uploaded, ready for transcription",
			Progress:  0,
			CreatedAt: time.Now(),
		}
		if err := database.CreateTranscription(transcription); err != nil {
			logging.ErrorContext(ctx, "Error creating transcription record", "error", err)
		}
	}

	// Mark session as complete
	if err := database.UpdateUploadSessionStatus(sessionID, "complete"); err != nil {
		logging.ErrorContext(ctx, "Error updating session status", "error", err)
	}

	// Clean up chunk files
	chunksDir := filepath.Join(uploadDir, "chunks", sessionID)
	for _, chunk := range chunks {
		removeWithLogging(chunk.ChunkPath, "uploaded chunk after successful assembly")
	}
	removeWithLogging(chunksDir, "empty chunks directory after cleanup")
}
