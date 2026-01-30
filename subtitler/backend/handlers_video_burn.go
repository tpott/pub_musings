package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tpott/subtitler/backend/auth"
	"github.com/tpott/subtitler/backend/db"
	"github.com/tpott/subtitler/backend/errmsg"
	"github.com/tpott/subtitler/backend/httputil"
	"github.com/tpott/subtitler/backend/logging"
	"github.com/tpott/subtitler/backend/ratelimit"
	"github.com/tpott/subtitler/backend/validation"
)

// registerVideoBurnHandlers registers handlers for subtitle burn/embed operations.
func registerVideoBurnHandlers(mux *http.ServeMux) { //nolint:funlen // route registration
	// Start burning subtitles into video (rate limited: 2/min per IP)
	// Mode: "burn" (default) hardcodes subtitles into video frames
	//       "embed" creates soft subtitle track (much faster, no re-encoding)
	mux.HandleFunc("POST /api/videos/{id}/burn", burnLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {

		uploadID, valid := validatePathID(w, r.PathValue("id"), "Upload ID")
		if !valid {
			return
		}

		// Parse and validate burn mode: "burn" (hardcode into video) or "embed" (soft subtitle track)
		// Default is "burn" for backwards compatibility
		burnModeParam := r.URL.Query().Get("mode")
		burnMode, err := validation.ValidateBurnMode(burnModeParam)
		if err != nil {
			httputil.RespondError(w, http.StatusBadRequest, err.Error())
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
			httputil.RespondError(w, http.StatusNotFound, "No transcription found - please transcribe the video first")
			return
		}
		if transcription.Status != "complete" {
			httputil.RespondJSON(w, http.StatusBadRequest, map[string]string{
				"error":  "Cannot burn subtitles - transcription not complete",
				"status": transcription.Status,
			})
			return
		}

		// Check if already processing
		existingJob, err := database.GetBurnJob(uploadID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting burn job", "error", err)
		}
		if existingJob != nil {
			if existingJob.Status == "processing" {
				httputil.RespondJSON(w, http.StatusOK, map[string]interface{}{
					"status":   "processing",
					"message":  existingJob.Message,
					"progress": existingJob.Progress,
				})
				return
			}
			if existingJob.Status == "complete" {
				httputil.RespondJSON(w, http.StatusOK, map[string]interface{}{
					"status":   "complete",
					"message":  existingJob.Message,
					"progress": 100,
				})
				return
			}
		}

		// Find the video file and get key version
		video, err := getVideoForDecryption(uploadID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Video not found for burn", "error", err, "upload_id", uploadID)
			httputil.RespondError(w, http.StatusNotFound, errmsg.ForVideoNotFound(err))
			return
		}
		videoPath := video.FilePath
		keyVersion := video.KeyVersion

		// Create or update burn job
		if existingJob == nil {
			burnJobID, err := generateID()
			if err != nil {
				logging.ErrorContext(r.Context(), "Error generating burn job ID", "error", err)
				httputil.RespondError(w, http.StatusInternalServerError, "Failed to generate burn job ID")
				return
			}
			burnJob := &db.BurnJob{
				ID:        burnJobID,
				VideoID:   uploadID,
				Status:    "processing",
				Message:   "Starting subtitle burn...",
				Progress:  0,
				CreatedAt: time.Now(),
			}
			if err := database.CreateBurnJob(burnJob); err != nil {
				logging.ErrorContext(r.Context(), "Error creating burn job", "error", err)
			}
		} else {
			if err := database.UpdateBurnJobStatus(uploadID, "processing", "Starting subtitle burn...", 0); err != nil {
				logging.Error("Failed to update burn job status", "video_id", uploadID, "error", err)
			}
		}

		// Process in background (capture key version and burn mode)
		go processBurnJob(uploadID, videoPath, keyVersion, string(burnMode), transcription)

		// Return immediately with processing status
		httputil.RespondJSON(w, http.StatusOK, map[string]interface{}{
			"status":   "processing",
			"message":  "Subtitle burn started",
			"progress": 0,
		})
	}))

	// Get burn job status
	mux.HandleFunc("GET /api/videos/{id}/burn", func(w http.ResponseWriter, r *http.Request) {

		uploadID, valid := validatePathID(w, r.PathValue("id"), "Upload ID")
		if !valid {
			return
		}

		job, err := database.GetBurnJob(uploadID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting burn job", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to get burn job status")
			return
		}

		if job == nil {
			httputil.RespondError(w, http.StatusNotFound, "No burn job found for this video")
			return
		}

		// Build response with progress info
		response := map[string]interface{}{
			"status":   job.Status,
			"message":  job.Message,
			"progress": job.Progress,
		}

		// If processing, calculate estimated time remaining
		if job.Status == "processing" && job.Progress > 0 {
			// Get transcription for duration info
			transcription, _ := database.GetTranscription(uploadID)
			if transcription != nil && transcription.Duration > 0 {
				response["duration"] = transcription.Duration

				// Calculate elapsed time since job started
				elapsed := time.Since(job.CreatedAt).Seconds()
				if elapsed > 0 && job.Progress > 0 {
					// Estimate total time based on current progress
					// progress% complete took elapsed seconds, so 100% will take:
					estimatedTotal := elapsed * 100 / float64(job.Progress)
					estimatedRemaining := estimatedTotal - elapsed
					if estimatedRemaining < 0 {
						estimatedRemaining = 0
					}
					response["estimated_remaining_seconds"] = int(estimatedRemaining)
				}
			}
		}

		httputil.RespondJSON(w, http.StatusOK, response)
	})

	// Download burned video (rate limited: 30/min per IP)
	mux.HandleFunc("GET /api/videos/{id}/burned", downloadLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		uploadID, valid := validatePathID(w, r.PathValue("id"), "Upload ID")
		if !valid {
			return
		}

		job, err := database.GetBurnJob(uploadID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting burn job", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to get burn job")
			return
		}

		if job == nil {
			httputil.RespondError(w, http.StatusNotFound, "No burn job found - start one first with POST /api/videos/{id}/burn")
			return
		}

		if job.Status != "complete" {
			httputil.RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
				"error":    "Burn job not complete",
				"status":   job.Status,
				"message":  job.Message,
				"progress": job.Progress,
			})
			return
		}

		if job.OutputPath == "" {
			httputil.RespondError(w, http.StatusNotFound, "Burned video file not found")
			return
		}

		// Validate that the burned output path is within the allowed upload directory
		// This prevents path traversal attacks if the database is compromised
		if err := pathValidator.ValidateAbsolutePath(job.OutputPath); err != nil {
			logging.ErrorContext(r.Context(), "Path validation failed for burned video download", "error", err, "path", job.OutputPath)
			httputil.RespondError(w, http.StatusForbidden, "Access denied")
			return
		}

		// Decrypt if encrypted
		servePath := job.OutputPath
		if strings.HasSuffix(job.OutputPath, ".age") {
			// Use burn job's output key version (defaults to 1 for older jobs)
			outputKeyVersion := 1
			if job.OutputKeyVersion != nil {
				outputKeyVersion = *job.OutputKeyVersion
			}
			decryptedPath, err := multiEnc.DecryptToTempFile(job.OutputPath, outputKeyVersion)
			if err != nil {
				logging.ErrorContext(r.Context(), "Failed to decrypt burned video", "error", err)
				httputil.RespondError(w, http.StatusInternalServerError, "Failed to decrypt video")
				return
			}
			defer removeWithLogging(decryptedPath, "decrypted video cleanup after subtitle extraction")
			servePath = decryptedPath
		}

		// Get original video filename for download name
		video, err := database.GetVideo(uploadID)
		downloadName := uploadID + "_subtitled.mp4"
		if err == nil && video != nil {
			// Use original filename with _subtitled suffix
			ext := filepath.Ext(video.Filename)
			baseName := strings.TrimSuffix(video.Filename, ext)
			downloadName = baseName + "_subtitled.mp4"
		}

		// Audit log: file access
		sessionID := r.URL.Query().Get("session_id")
		var userID string
		if token := auth.GetTokenFromRequest(r); token != "" {
			if session, err := database.GetSessionByToken(token); err == nil && session != nil {
				userID = session.UserID
			}
		}
		logging.InfoContext(r.Context(), "File access: burned video download",
			"file_type", "burned_video",
			"video_id", uploadID,
			"user_id", userID,
			"session_id", sessionID,
			"client_ip", ratelimit.GetClientIP(r),
		)

		// Set headers for download
		// Use RFC 5987 encoding for proper handling of non-ASCII characters
		w.Header().Set("Content-Disposition", httputil.ContentDisposition(downloadName))
		http.ServeFile(w, r, servePath)
	}))
}

// processBurnJob runs the subtitle burn pipeline in a background goroutine.
// It decrypts the video (if encrypted), generates an SRT file, runs ffmpeg
// to burn or embed subtitles, encrypts the output, and updates the burn job status.
func processBurnJob(uploadID, videoPath string, keyVersion int, mode string, transcription *db.Transcription) {
	// Panic recovery to prevent goroutine crashes from leaving jobs in stuck state
	defer func() {
		if r := recover(); r != nil {
			logging.Error("Panic in burn goroutine", "upload_id", uploadID, "panic", r)
			if err := database.FailBurnJob(uploadID, "Internal error: burn process crashed"); err != nil {
				logging.Error("Failed to mark burn job as failed", "video_id", uploadID, "error", err)
			}
		}
	}()

	if isShuttingDown() {
		failBurnShutdown(uploadID)
		return
	}

	logging.Info("Starting subtitle burn", "upload_id", uploadID, "key_version", keyVersion, "mode", mode)

	// Decrypt video if encrypted
	workingVideoPath, cleanup, err := decryptVideoForBurn(uploadID, videoPath, keyVersion)
	if err != nil {
		return // error already logged and job marked as failed
	}
	if cleanup != nil {
		defer cleanup()
	}

	if isShuttingDown() {
		failBurnShutdown(uploadID)
		return
	}

	// Generate SRT temp file from transcription segments
	srtPath, err := generateBurnSRTFile(uploadID, transcription)
	if err != nil {
		return // error already logged and job marked as failed
	}
	defer removeWithLogging(srtPath, "temp SRT file cleanup after burn")

	progressMsg := "Burning subtitles into video..."
	if mode == "embed" {
		progressMsg = "Embedding subtitle track..."
	}
	if err := database.UpdateBurnJobStatus(uploadID, "processing", progressMsg, 20); err != nil {
		logging.Error("Failed to update burn job status", "video_id", uploadID, "error", err)
	}

	outputPath := filepath.Join(uploadDir, uploadID+"_burned.mp4")
	defer removeWithLogging(outputPath, "unencrypted burn output cleanup")

	if isShuttingDown() {
		failBurnShutdown(uploadID)
		return
	}

	// Build and run ffmpeg
	cmd := buildBurnFFmpegCmd(workingVideoPath, srtPath, outputPath, mode)
	cancelProgress := startBurnProgressTracker(uploadID, progressMsg, transcription.Duration, mode)
	defer cancelProgress()

	cmdOutput, err := cmd.CombinedOutput()
	cancelProgress()

	if err != nil {
		logging.Error("ffmpeg burn subtitles failed", "error", err, "output", string(cmdOutput))
		if err := database.FailBurnJob(uploadID, fmt.Sprintf("Failed to burn subtitles: %v", err)); err != nil {
			logging.Error("Failed to mark burn job as failed", "video_id", uploadID, "error", err)
		}
		return
	}

	// Encrypt the output
	encryptBurnOutput(uploadID, outputPath, mode)
}

// failBurnShutdown marks a burn job as failed due to server shutdown.
func failBurnShutdown(uploadID string) {
	logging.Info("Burn cancelled due to server shutdown", "upload_id", uploadID)
	if err := database.FailBurnJob(uploadID, "Server shutting down - burn interrupted"); err != nil {
		logging.Error("Failed to mark burn job as failed", "video_id", uploadID, "error", err)
	}
}

// decryptVideoForBurn decrypts an encrypted video file to a temp location for burning.
// Returns the working video path, a cleanup function (nil if no decryption needed), and any error.
func decryptVideoForBurn(uploadID, videoPath string, keyVersion int) (string, func(), error) {
	if !strings.HasSuffix(videoPath, ".age") {
		return videoPath, nil, nil
	}

	if err := database.UpdateBurnJobStatus(uploadID, "processing", "Decrypting video...", 5); err != nil {
		logging.Error("Failed to update burn job status", "video_id", uploadID, "error", err)
	}

	decryptedPath, err := multiEnc.DecryptToTempFile(videoPath, keyVersion)
	if err != nil {
		logging.Error("Video decryption failed", "error", err)
		if err := database.FailBurnJob(uploadID, fmt.Sprintf("Video decryption failed: %v", err)); err != nil {
			logging.Error("Failed to mark burn job as failed", "video_id", uploadID, "error", err)
		}
		return "", nil, err
	}

	cleanup := func() {
		removeWithLogging(decryptedPath, "decrypted video cleanup after burn")
	}
	return decryptedPath, cleanup, nil
}

// generateBurnSRTFile creates a temporary SRT file from transcription segments for ffmpeg.
func generateBurnSRTFile(uploadID string, transcription *db.Transcription) (string, error) {
	segments, err := transcription.GetSegments()
	if err != nil || len(segments) == 0 {
		logging.Error("No segments available", "error", err)
		if err := database.FailBurnJob(uploadID, "No subtitle segments available"); err != nil {
			logging.Error("Failed to mark burn job as failed", "video_id", uploadID, "error", err)
		}
		return "", fmt.Errorf("no segments available")
	}

	if err := database.UpdateBurnJobStatus(uploadID, "processing", "Generating subtitles...", 10); err != nil {
		logging.Error("Failed to update burn job status", "video_id", uploadID, "error", err)
	}

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

	srtContent := generateSRT(whisperResult)
	srtPath := filepath.Join(uploadDir, uploadID+"_burn.srt")
	if err := os.WriteFile(srtPath, []byte(srtContent), 0644); err != nil {
		logging.Error("Failed to write SRT file", "error", err)
		if err := database.FailBurnJob(uploadID, fmt.Sprintf("Failed to write SRT file: %v", err)); err != nil {
			logging.Error("Failed to mark burn job as failed", "video_id", uploadID, "error", err)
		}
		return "", err
	}

	return srtPath, nil
}

// buildBurnFFmpegCmd constructs the ffmpeg command for burning or embedding subtitles.
func buildBurnFFmpegCmd(workingVideoPath, srtPath, outputPath, mode string) *exec.Cmd {
	if mode == "embed" {
		return exec.Command("ffmpeg",
			"-i", workingVideoPath,
			"-i", srtPath,
			"-c:v", "copy",
			"-c:a", "copy",
			"-c:s", "mov_text",
			"-y",
			outputPath,
		)
	}

	subtitleStyle := "FontSize=24,PrimaryColour=&HFFFFFF,OutlineColour=&H000000,Outline=2"
	if subtitleFont != "" {
		safeFontName := strings.NewReplacer(`'`, ``, `"`, ``, `:`, ``, `;`, ``).Replace(subtitleFont)
		subtitleStyle = fmt.Sprintf("FontName=%s,%s", safeFontName, subtitleStyle)
	}

	return exec.Command("ffmpeg",
		"-i", workingVideoPath,
		"-vf", fmt.Sprintf("subtitles='%s':force_style='%s'", escapeFFmpegFilterPath(srtPath), subtitleStyle),
		"-c:a", "copy",
		"-y",
		outputPath,
	)
}

// startBurnProgressTracker spawns a goroutine that periodically updates burn job
// progress from 20% to 85% based on estimated burn time. Returns a cancel function.
func startBurnProgressTracker(uploadID, progressMsg string, duration float64, mode string) context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		estimatedBurnTime := duration
		if mode == "embed" {
			estimatedBurnTime = 10
		}
		if estimatedBurnTime < 10 {
			estimatedBurnTime = 10
		}
		if estimatedBurnTime > 600 {
			estimatedBurnTime = 600
		}

		numTicks := 13 // 65 / 5 = 13 updates of 5% each
		tickInterval := time.Duration(estimatedBurnTime/float64(numTicks)) * time.Second
		if tickInterval < time.Second {
			tickInterval = time.Second
		}

		ticker := time.NewTicker(tickInterval)
		defer ticker.Stop()
		progress := 20
		for {
			select {
			case <-ctx.Done():
				return
			case <-shutdownCtx.Done():
				return
			case <-ticker.C:
				if progress < 85 {
					progress += 5
					if err := database.UpdateBurnJobStatus(uploadID, "processing", progressMsg, progress); err != nil {
						logging.Error("Failed to update burn job status", "video_id", uploadID, "error", err)
					}
				}
			}
		}
	}()
	return cancel
}

// encryptBurnOutput encrypts the burned video output and completes the burn job.
func encryptBurnOutput(uploadID, outputPath, mode string) {
	if err := database.UpdateBurnJobStatus(uploadID, "processing", "Encrypting output...", 90); err != nil {
		logging.Error("Failed to update burn job status", "video_id", uploadID, "error", err)
	}

	encOutputPath, keyVersion, err := multiEnc.EncryptFile(outputPath)
	if err != nil {
		logging.Error("Failed to encrypt burned video", "error", err)
		if err := database.FailBurnJob(uploadID, fmt.Sprintf("Failed to encrypt output: %v", err)); err != nil {
			logging.Error("Failed to mark burn job as failed", "video_id", uploadID, "error", err)
		}
		return
	}

	logging.Info("Subtitle burn complete", "upload_id", uploadID, "output_path", encOutputPath, "key_version", keyVersion, "mode", mode)
	if err := database.CompleteBurnJobWithKeyVersion(uploadID, encOutputPath, keyVersion); err != nil {
		logging.Error("Failed to complete burn job", "video_id", uploadID, "error", err)
	}
}
