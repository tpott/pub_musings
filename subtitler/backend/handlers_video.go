package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tpott/subtitler/backend/audio"
	"github.com/tpott/subtitler/backend/auth"
	"github.com/tpott/subtitler/backend/db"
	"github.com/tpott/subtitler/backend/errmsg"
	"github.com/tpott/subtitler/backend/httputil"
	"github.com/tpott/subtitler/backend/language"
	"github.com/tpott/subtitler/backend/logging"
	"github.com/tpott/subtitler/backend/metrics"
	"github.com/tpott/subtitler/backend/ratelimit"
	"github.com/tpott/subtitler/backend/script"
	"github.com/tpott/subtitler/backend/security"
	"github.com/tpott/subtitler/backend/validation"
)

func registerVideoHandlers(mux *http.ServeMux) { //nolint:funlen // route registration
	// Download SRT file for a transcription
	mux.HandleFunc("GET /api/videos/{id}/subtitles.srt", func(w http.ResponseWriter, r *http.Request) {
		uploadID, valid := validatePathID(w, r.PathValue("id"), "Upload ID")
		if !valid {
			return
		}

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
				"error":  "Transcription not yet complete",
				"status": transcription.Status,
			})
			return
		}

		// Generate ETag from transcription ID + completion time + segments hash
		// Subtitles can change if edited or re-transcribed, so use CompletedAt
		var completedAt int64
		if transcription.CompletedAt != nil {
			completedAt = transcription.CompletedAt.Unix()
		}
		etag := generateETag(fmt.Sprintf("srt-%s-%d-%s", transcription.ID, completedAt, transcription.SegmentsJSON[:min(100, len(transcription.SegmentsJSON))]))

		// Check for conditional request (If-None-Match)
		if handleConditionalRequest(w, r, etag) {
			return // 304 Not Modified sent
		}

		segments, err := transcription.GetSegments()
		if err != nil || len(segments) == 0 {
			httputil.RespondError(w, http.StatusNotFound, "No subtitle segments available")
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

		// Set caching headers - subtitles may be edited, so use shorter cache time
		// Cache for 10 minutes, must revalidate after that
		setCacheHeaders(w, etag, 600)

		// Set headers for file download
		// Use text/plain as it's universally supported by browsers for download
		// application/x-subrip is the registered MIME type but has limited browser support
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", httputil.ContentDisposition(uploadID+".srt"))
		w.WriteHeader(http.StatusOK)
		httputil.WriteContent(w, []byte(srtContent), "SRT download")
	})

	// Download VTT file for a transcription
	mux.HandleFunc("GET /api/videos/{id}/subtitles.vtt", func(w http.ResponseWriter, r *http.Request) {
		uploadID, valid := validatePathID(w, r.PathValue("id"), "Upload ID")
		if !valid {
			return
		}

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
				"error":  "Transcription not yet complete",
				"status": transcription.Status,
			})
			return
		}

		// Generate ETag from transcription ID + completion time + segments hash
		var completedAt int64
		if transcription.CompletedAt != nil {
			completedAt = transcription.CompletedAt.Unix()
		}
		etag := generateETag(fmt.Sprintf("vtt-%s-%d-%s", transcription.ID, completedAt, transcription.SegmentsJSON[:min(100, len(transcription.SegmentsJSON))]))

		// Check for conditional request (If-None-Match)
		if handleConditionalRequest(w, r, etag) {
			return // 304 Not Modified sent
		}

		segments, err := transcription.GetSegments()
		if err != nil || len(segments) == 0 {
			httputil.RespondError(w, http.StatusNotFound, "No subtitle segments available")
			return
		}

		// Convert to WhisperResult for VTT generation
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

		// Generate VTT content
		vttContent := generateVTT(whisperResult)

		// Set caching headers
		setCacheHeaders(w, etag, 600)

		// Set headers for file download
		// text/vtt is the official MIME type for WebVTT
		w.Header().Set("Content-Type", "text/vtt; charset=utf-8")
		w.Header().Set("Content-Disposition", httputil.ContentDisposition(uploadID+".vtt"))
		w.WriteHeader(http.StatusOK)
		httputil.WriteContent(w, []byte(vttContent), "VTT download")
	})

	// Download JSON file for a transcription
	mux.HandleFunc("GET /api/videos/{id}/subtitles.json", func(w http.ResponseWriter, r *http.Request) {
		uploadID, valid := validatePathID(w, r.PathValue("id"), "Upload ID")
		if !valid {
			return
		}

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
				"error":  "Transcription not yet complete",
				"status": transcription.Status,
			})
			return
		}

		// Generate ETag from transcription ID + completion time + segments hash
		var completedAt int64
		if transcription.CompletedAt != nil {
			completedAt = transcription.CompletedAt.Unix()
		}
		etag := generateETag(fmt.Sprintf("json-%s-%d-%s", transcription.ID, completedAt, transcription.SegmentsJSON[:min(100, len(transcription.SegmentsJSON))]))

		// Check for conditional request (If-None-Match)
		if handleConditionalRequest(w, r, etag) {
			return // 304 Not Modified sent
		}

		segments, err := transcription.GetSegments()
		if err != nil || len(segments) == 0 {
			httputil.RespondError(w, http.StatusNotFound, "No subtitle segments available")
			return
		}

		// Return JSON with segments
		response := map[string]interface{}{
			"video_id":  uploadID,
			"language":  transcription.Language,
			"duration":  transcription.Duration,
			"full_text": transcription.FullText,
			"segments":  segments,
		}

		// Set caching headers
		setCacheHeaders(w, etag, 600)

		// Set headers for file download
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", httputil.ContentDisposition(uploadID+".json"))
		httputil.RespondJSON(w, http.StatusOK, response)
	})

	// Extract embedded subtitle track from video
	mux.HandleFunc("GET /api/videos/{id}/embedded-subtitles/{track}", downloadLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		videoID, valid := validatePathID(w, r.PathValue("id"), "Video ID")
		if !valid {
			return
		}

		// Parse track index
		trackStr := r.PathValue("track")
		trackIndex, err := strconv.Atoi(trackStr)
		if err != nil || trackIndex < 0 {
			httputil.RespondError(w, http.StatusBadRequest, "Invalid track index")
			return
		}

		// Get the video
		video, err := database.GetVideo(videoID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting video", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to get video")
			return
		}
		if video == nil {
			httputil.RespondError(w, http.StatusNotFound, "Video not found")
			return
		}

		// Verify the track exists in the video's embedded subtitles
		if video.EmbeddedSubtitlesJSON == nil {
			httputil.RespondError(w, http.StatusNotFound, "This video has no embedded subtitles")
			return
		}

		var tracks []audio.SubtitleTrack
		if err := json.Unmarshal([]byte(*video.EmbeddedSubtitlesJSON), &tracks); err != nil {
			logging.ErrorContext(r.Context(), "Error parsing embedded subtitles", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to parse subtitle information")
			return
		}

		// Find the track with the given index
		var foundTrack *audio.SubtitleTrack
		for _, t := range tracks {
			if t.Index == trackIndex {
				foundTrack = &t
				break
			}
		}
		if foundTrack == nil {
			httputil.RespondError(w, http.StatusNotFound, "Subtitle track not found")
			return
		}

		// Only text-based subtitles can be extracted
		if !foundTrack.TextBased {
			httputil.RespondError(w, http.StatusBadRequest, "Cannot extract image-based subtitle track (use OCR for Blu-ray/DVD subtitles)")
			return
		}

		// Check access (user owns video or has matching session_id)
		token := auth.GetTokenFromRequest(r)
		user, _, _ := auth.ValidateSession(database, token)
		sessionID := r.URL.Query().Get("session_id")

		hasAccess := false
		if user != nil && video.UserID != nil && *video.UserID == user.ID {
			hasAccess = true
		} else if sessionID != "" && video.SessionID != nil && *video.SessionID == sessionID {
			hasAccess = true
		}
		if !hasAccess {
			httputil.RespondError(w, http.StatusForbidden, "You do not have permission to access this video")
			return
		}

		// Get format from query parameter (default to srt)
		format := r.URL.Query().Get("format")
		if format == "" {
			format = "srt"
		}
		if format != "srt" && format != "vtt" {
			httputil.RespondError(w, http.StatusBadRequest, "Format must be 'srt' or 'vtt'")
			return
		}

		// Decrypt the video to a temp file for extraction
		decryptedPath, err := multiEnc.DecryptToTempFile(video.FilePath, video.KeyVersion)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error decrypting video", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to access video")
			return
		}
		defer removeWithLogging(decryptedPath, "decrypted video cleanup after subtitle download")

		// Extract the subtitle track
		content, err := audio.ExtractSubtitleTrack(decryptedPath, trackIndex, format)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error extracting subtitle track", "track", trackIndex, "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to extract subtitle track")
			return
		}

		// Set response headers
		contentType := "text/plain; charset=utf-8"
		ext := ".srt"
		if format == "vtt" {
			contentType = "text/vtt; charset=utf-8"
			ext = ".vtt"
		}

		filename := strings.TrimSuffix(video.Filename, filepath.Ext(video.Filename))
		if foundTrack.Language != "" {
			filename += "_" + foundTrack.Language
		}
		filename += "_embedded" + ext

		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", httputil.ContentDisposition(filename))
		w.WriteHeader(http.StatusOK)
		httputil.WriteContent(w, []byte(content), "embedded subtitles download")
	}))

	// List all videos
	mux.HandleFunc("GET /api/videos", func(w http.ResponseWriter, r *http.Request) {
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

		// SECURITY: Require either authenticated user or session_id to filter videos
		// Without this check, anonymous requests would return ALL videos in the database
		if userPtr == nil && sessionPtr == nil {
			httputil.RespondError(w, http.StatusBadRequest, "Authentication or session_id required to list videos")
			return
		}

		// Parse pagination parameters
		limit := 50 // default
		offset := 0
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
				limit = l
				if limit > 100 {
					limit = 100 // max limit
				}
			}
		}
		if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
			if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
				offset = o
			}
		}

		result, err := database.ListVideosPaginated(userPtr, sessionPtr, limit, offset)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error listing videos", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to list videos")
			return
		}

		// Get transcription status and calculate expiry for each video
		type VideoWithStatus struct {
			db.Video
			TranscriptionStatus string                `json:"transcription_status"`
			ExpiresAt           *time.Time            `json:"expires_at,omitempty"`
			EmbeddedSubtitles   []audio.SubtitleTrack `json:"embedded_subtitles,omitempty"`
		}

		videos := make([]VideoWithStatus, len(result.Videos))
		for i, v := range result.Videos {
			videos[i] = VideoWithStatus{Video: v, TranscriptionStatus: "none"}
			if t, err := database.GetTranscription(v.ID); err == nil && t != nil {
				videos[i].TranscriptionStatus = t.Status
			}

			// Parse embedded subtitles JSON if present
			if v.EmbeddedSubtitlesJSON != nil {
				var tracks []audio.SubtitleTrack
				if err := json.Unmarshal([]byte(*v.EmbeddedSubtitlesJSON), &tracks); err == nil {
					videos[i].EmbeddedSubtitles = tracks
				}
			}

			// Calculate expiration time based on user type
			// Anonymous: 48 hours, Registered: 90 days
			var expiresAt time.Time
			if v.UserID == nil {
				expiresAt = v.CreatedAt.Add(48 * time.Hour)
			} else {
				expiresAt = v.CreatedAt.Add(90 * 24 * time.Hour)
			}
			videos[i].ExpiresAt = &expiresAt
		}

		hasMore := offset+len(result.Videos) < result.TotalCount
		httputil.RespondJSON(w, http.StatusOK, map[string]interface{}{
			"videos":      videos,
			"total_count": result.TotalCount,
			"has_more":    hasMore,
		})
	})

	// Delete a video
	mux.HandleFunc("DELETE /api/videos/{id}", func(w http.ResponseWriter, r *http.Request) {
		videoID, valid := validatePathID(w, r.PathValue("id"), "Video ID")
		if !valid {
			return
		}

		// Get the video to check ownership
		video, err := database.GetVideo(videoID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting video", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to get video")
			return
		}
		if video == nil {
			httputil.RespondError(w, http.StatusNotFound, "Video not found")
			return
		}

		// Check ownership - either authenticated user owns it, or anonymous session matches
		token := auth.GetTokenFromRequest(r)
		user, _, _ := auth.ValidateSession(database, token)

		sessionID := r.URL.Query().Get("session_id")

		hasAccess := false
		if user != nil && video.UserID != nil && *video.UserID == user.ID {
			// Authenticated user owns the video
			hasAccess = true
		} else if sessionID != "" && video.SessionID != nil && *video.SessionID == sessionID {
			// Anonymous user with matching session
			hasAccess = true
		}

		if !hasAccess {
			httputil.RespondError(w, http.StatusForbidden, "You do not have permission to delete this video")
			return
		}

		// Delete from database and get file paths
		deletedFiles, err := database.DeleteVideo(videoID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error deleting video from database", "video_id", videoID, "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to delete video")
			return
		}

		// Delete the video file from disk
		if deletedFiles != nil && deletedFiles.FilePath != "" {
			removeWithLogging(deletedFiles.FilePath, "video file deletion")
		}

		// Delete the thumbnail file from disk
		if deletedFiles != nil && deletedFiles.ThumbnailPath != nil && *deletedFiles.ThumbnailPath != "" {
			removeWithLogging(*deletedFiles.ThumbnailPath, "thumbnail file deletion")
		}

		// Delete the burn output file from disk
		if deletedFiles != nil && deletedFiles.BurnOutputPath != nil && *deletedFiles.BurnOutputPath != "" {
			removeWithLogging(*deletedFiles.BurnOutputPath, "burn output file deletion")
		}

		// Log security event for video deletion
		userID := ""
		if user != nil {
			userID = user.ID
		}
		security.VideoDeletedByUser(r.Context(), ratelimit.GetClientIP(r), userID, videoID, video.Filename)

		httputil.RespondJSON(w, http.StatusOK, map[string]string{"message": "Video deleted successfully"})
	})

	// Reprocess a failed transcription
	mux.HandleFunc("POST /api/videos/{id}/reprocess", transcribeLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {

		uploadID, valid := validatePathID(w, r.PathValue("id"), "Video ID")
		if !valid {
			return
		}

		// Get the video to check ownership
		video, err := database.GetVideo(uploadID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting video", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to get video")
			return
		}
		if video == nil {
			httputil.RespondError(w, http.StatusNotFound, "Video not found")
			return
		}

		// Check ownership - either authenticated user owns it, or anonymous session matches
		token := auth.GetTokenFromRequest(r)
		user, _, _ := auth.ValidateSession(database, token)

		sessionID := r.URL.Query().Get("session_id")

		hasAccess := false
		if user != nil && video.UserID != nil && *video.UserID == user.ID {
			// Authenticated user owns the video
			hasAccess = true
		} else if sessionID != "" && video.SessionID != nil && *video.SessionID == sessionID {
			// Anonymous user with matching session
			hasAccess = true
		}

		if !hasAccess {
			httputil.RespondError(w, http.StatusForbidden, "You do not have permission to reprocess this video")
			return
		}

		// Check if transcription is in error state
		transcription, err := database.GetTranscription(uploadID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting transcription", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to get transcription status")
			return
		}

		if transcription == nil {
			httputil.RespondError(w, http.StatusBadRequest, "No transcription found for this video")
			return
		}

		if transcription.Status != "error" {
			httputil.RespondJSON(w, http.StatusBadRequest, map[string]string{
				"error":  "Can only reprocess failed transcriptions",
				"status": transcription.Status,
			})
			return
		}

		// Use already-fetched video for file path and key version
		videoPath := video.FilePath
		keyVersion := video.KeyVersion

		// Update transcription status to processing
		if err := database.UpdateTranscriptionStatus(uploadID, "processing", "Extracting audio...", 10); err != nil {
			logging.Error("Failed to update transcription status", "video_id", uploadID, "error", err)
		}

		// Process in background (same logic as POST /api/transcribe/{id})
		go func(kv int) {
			// Panic recovery to prevent goroutine crashes from leaving jobs in stuck state
			defer func() {
				if r := recover(); r != nil {
					logging.Error("Panic in reprocess goroutine", "upload_id", uploadID, "panic", r)
					metrics.RecordTranscriptionFailed()
					if err := database.FailTranscription(uploadID, "Internal error: reprocess crashed"); err != nil {
						logging.Error("Failed to mark transcription as failed", "video_id", uploadID, "error", err)
					}
				}
			}()

			// Check for shutdown before starting
			if isShuttingDown() {
				logging.Info("Reprocess cancelled due to server shutdown", "upload_id", uploadID)
				if err := database.FailTranscription(uploadID, "Server shutting down - reprocess interrupted"); err != nil {
					logging.Error("Failed to mark transcription as failed", "video_id", uploadID, "error", err)
				}
				return
			}

			logging.Info("Reprocessing transcription", "upload_id", uploadID, "key_version", kv)

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
				defer removeWithLogging(decryptedPath, "decrypted video cleanup after reprocess")
			}

			// Check for shutdown after decryption
			if isShuttingDown() {
				logging.Info("Reprocess cancelled due to server shutdown", "upload_id", uploadID)
				if err := database.FailTranscription(uploadID, "Server shutting down - reprocess interrupted"); err != nil {
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
			defer removeWithLogging(audioPath, "audio file cleanup after reprocess")

			// Check for shutdown after audio extraction
			if isShuttingDown() {
				logging.Info("Reprocess cancelled due to server shutdown", "upload_id", uploadID)
				if err := database.FailTranscription(uploadID, "Server shutting down - reprocess interrupted"); err != nil {
					logging.Error("Failed to mark transcription as failed", "video_id", uploadID, "error", err)
				}
				return
			}

			if err := database.UpdateTranscriptionStatus(uploadID, "processing", "Running transcription...", 30); err != nil {
				logging.Error("Failed to update transcription status", "video_id", uploadID, "error", err)
			}

			// Start progress simulation goroutine
			// Use context for cancellation so it's safe to call cancel multiple times
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
						if progress < 90 {
							progress += 5
							if err := database.UpdateTranscriptionStatus(uploadID, "processing", "Transcribing audio...", progress); err != nil {
								logging.Error("Failed to update transcription status", "video_id", uploadID, "error", err)
							}
						}
					}
				}
			}()

			// Run whisper (reprocess uses auto-detect)
			outputPath := filepath.Join(uploadDir, uploadID+"_transcript")
			result, err := transcribe(audioPath, outputPath, "auto")
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

			// Success
			logging.Info("Reprocessing complete", "upload_id", uploadID, "segment_count", len(result.Segments))
			if err := database.CompleteTranscription(uploadID, result.Language, result.Duration, result.Text, segments); err != nil {
				logging.Error("Error saving transcription result", "error", err)
			}
			// Note: audioPath is cleaned up by defer above
		}(keyVersion)

		httputil.RespondJSON(w, http.StatusOK, map[string]string{
			"status":  "processing",
			"message": "Reprocessing started",
		})
	}))

	// Serve uploaded video files for playback
	mux.HandleFunc("GET /api/videos/{id}/video", downloadLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		uploadID, valid := validatePathID(w, r.PathValue("id"), "Upload ID")
		if !valid {
			return
		}

		// Get video metadata from database for ETag generation and decryption
		video, err := getVideoForDecryption(uploadID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Video not found for download", "error", err, "upload_id", uploadID)
			httputil.RespondError(w, http.StatusNotFound, errmsg.ForVideoNotFound(err))
			return
		}

		// Generate ETag from video ID + creation time + size
		// Video files don't change after upload, so this is stable
		etag := generateETag(fmt.Sprintf("%s-%d-%d", video.ID, video.CreatedAt.Unix(), video.Size))

		// Check for conditional request (If-None-Match)
		if handleConditionalRequest(w, r, etag) {
			return // 304 Not Modified sent
		}

		videoPath := video.FilePath

		// Validate that the file path is within the allowed upload directory
		// This prevents path traversal attacks if the database is compromised
		if err := pathValidator.ValidateAbsolutePath(videoPath); err != nil {
			logging.ErrorContext(r.Context(), "Path validation failed for video download", "error", err, "path", videoPath)
			httputil.RespondError(w, http.StatusForbidden, "Access denied")
			return
		}

		// If file is encrypted, decrypt to temp file for serving
		// (http.ServeFile needs seekable file for range requests)
		if strings.HasSuffix(videoPath, ".age") {
			decryptedPath, err := multiEnc.DecryptToTempFile(videoPath, video.KeyVersion)
			if err != nil {
				logging.ErrorContext(r.Context(), "Failed to decrypt video for serving", "error", err)
				httputil.RespondError(w, http.StatusInternalServerError, "Failed to decrypt video")
				return
			}
			defer removeWithLogging(decryptedPath, "decrypted video cleanup after download")
			videoPath = decryptedPath
		}

		// Set caching headers before serving
		// Video files are immutable (don't change after upload), so can be cached
		// Cache for 1 hour, must revalidate after that
		if etag != "" {
			setCacheHeaders(w, etag, 3600) // 1 hour
		}

		// Audit log: file access
		sessionID := r.URL.Query().Get("session_id")
		var userID string
		if token := auth.GetTokenFromRequest(r); token != "" {
			if session, err := database.GetSessionByToken(token); err == nil && session != nil {
				userID = session.UserID
			}
		}
		logging.InfoContext(r.Context(), "File access: video download",
			"file_type", "video",
			"video_id", uploadID,
			"user_id", userID,
			"session_id", sessionID,
			"client_ip", ratelimit.GetClientIP(r),
		)

		// Serve the file
		http.ServeFile(w, r, videoPath)
	}))

	// Serve video thumbnail
	mux.HandleFunc("GET /api/videos/{id}/thumbnail", downloadLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		uploadID, valid := validatePathID(w, r.PathValue("id"), "Upload ID")
		if !valid {
			return
		}

		// Get video from database to get thumbnail path
		video, err := database.GetVideo(uploadID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting video", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Database error")
			return
		}
		if video == nil {
			httputil.RespondError(w, http.StatusNotFound, "Video not found")
			return
		}

		// Check if thumbnail exists
		if video.ThumbnailPath == nil || *video.ThumbnailPath == "" {
			httputil.RespondError(w, http.StatusNotFound, "Thumbnail not available")
			return
		}

		thumbPath := *video.ThumbnailPath

		// Validate that the thumbnail path is within the allowed upload directory
		// This prevents path traversal attacks if the database is compromised
		if err := pathValidator.ValidateAbsolutePath(thumbPath); err != nil {
			logging.ErrorContext(r.Context(), "Path validation failed for thumbnail download", "error", err, "path", thumbPath)
			httputil.RespondError(w, http.StatusForbidden, "Access denied")
			return
		}

		// Check if file exists on disk
		if _, err := os.Stat(thumbPath); os.IsNotExist(err) {
			httputil.RespondError(w, http.StatusNotFound, "Thumbnail file not found")
			return
		}

		// Generate ETag from video ID + creation time (thumbnails are immutable)
		etag := generateETag(fmt.Sprintf("thumb-%s-%d", video.ID, video.CreatedAt.Unix()))

		// Check for conditional request (If-None-Match)
		if handleConditionalRequest(w, r, etag) {
			return // 304 Not Modified sent
		}

		// If file is encrypted, decrypt to temp file for serving
		// Thumbnail uses same key version as the video
		if strings.HasSuffix(thumbPath, ".age") {
			decryptedPath, err := multiEnc.DecryptToTempFile(thumbPath, video.KeyVersion)
			if err != nil {
				logging.ErrorContext(r.Context(), "Failed to decrypt thumbnail for serving", "error", err)
				httputil.RespondError(w, http.StatusInternalServerError, "Failed to decrypt thumbnail")
				return
			}
			defer removeWithLogging(decryptedPath, "decrypted file cleanup after thumbnail download")
			thumbPath = decryptedPath
		}

		// Set caching headers - thumbnails are immutable, cache for 24 hours
		setCacheHeaders(w, etag, 86400) // 24 hours

		// Set content type for JPEG
		w.Header().Set("Content-Type", "image/jpeg")

		// Audit log: file access
		sessionID := r.URL.Query().Get("session_id")
		var userID string
		if token := auth.GetTokenFromRequest(r); token != "" {
			if session, err := database.GetSessionByToken(token); err == nil && session != nil {
				userID = session.UserID
			}
		}
		logging.InfoContext(r.Context(), "File access: thumbnail download",
			"file_type", "thumbnail",
			"video_id", uploadID,
			"user_id", userID,
			"session_id", sessionID,
			"client_ip", ratelimit.GetClientIP(r),
		)

		// Serve the file
		http.ServeFile(w, r, thumbPath)
	}))

	// Get language detection hints for a video
	// This analyzes video metadata (ffprobe) and filename patterns to suggest the spoken language
	mux.HandleFunc("GET /api/videos/{id}/language-hints", func(w http.ResponseWriter, r *http.Request) {
		uploadID, valid := validatePathID(w, r.PathValue("id"), "Upload ID")
		if !valid {
			return
		}

		// Get video from database
		video, err := database.GetVideo(uploadID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting video for language hints", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Database error")
			return
		}
		if video == nil {
			httputil.RespondError(w, http.StatusNotFound, "Video not found")
			return
		}

		// Get the decrypted video file path for metadata analysis
		// We need the actual file to run ffprobe
		var videoPath string
		if video.FilePath != "" {
			if strings.HasSuffix(video.FilePath, ".age") {
				// Decrypt temporarily for analysis
				decryptedPath, err := multiEnc.DecryptToTempFile(video.FilePath, video.KeyVersion)
				if err != nil {
					logging.ErrorContext(r.Context(), "Failed to decrypt video for language analysis", "error", err)
					// Fall back to filename-only detection
					result := language.Detect("", video.Filename)
					httputil.RespondJSON(w, http.StatusOK, result)
					return
				}
				defer removeWithLogging(decryptedPath, "decrypted video cleanup after language hints")
				videoPath = decryptedPath
			} else {
				videoPath = video.FilePath
			}
		}

		// Run language detection
		result := language.Detect(videoPath, video.Filename)

		httputil.RespondJSON(w, http.StatusOK, result)
	})

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
		go func(kv int, mode string) {
			// Panic recovery to prevent goroutine crashes from leaving jobs in stuck state
			defer func() {
				if r := recover(); r != nil {
					logging.Error("Panic in burn goroutine", "upload_id", uploadID, "panic", r)
					if err := database.FailBurnJob(uploadID, "Internal error: burn process crashed"); err != nil {
						logging.Error("Failed to mark burn job as failed", "video_id", uploadID, "error", err)
					}
				}
			}()

			// Check for shutdown before starting
			if isShuttingDown() {
				logging.Info("Burn cancelled due to server shutdown", "upload_id", uploadID)
				if err := database.FailBurnJob(uploadID, "Server shutting down - burn interrupted"); err != nil {
					logging.Error("Failed to mark burn job as failed", "video_id", uploadID, "error", err)
				}
				return
			}

			logging.Info("Starting subtitle burn", "upload_id", uploadID, "key_version", kv, "mode", mode)

			// Decrypt video if encrypted
			workingVideoPath := videoPath
			if strings.HasSuffix(videoPath, ".age") {
				if err := database.UpdateBurnJobStatus(uploadID, "processing", "Decrypting video...", 5); err != nil {
					logging.Error("Failed to update burn job status", "video_id", uploadID, "error", err)
				}
				decryptedPath, err := multiEnc.DecryptToTempFile(videoPath, kv)
				if err != nil {
					logging.Error("Video decryption failed", "error", err)
					if err := database.FailBurnJob(uploadID, fmt.Sprintf("Video decryption failed: %v", err)); err != nil {
						logging.Error("Failed to mark burn job as failed", "video_id", uploadID, "error", err)
					}
					return
				}
				workingVideoPath = decryptedPath
				defer removeWithLogging(decryptedPath, "decrypted video cleanup after burn")
			}

			// Check for shutdown after decryption
			if isShuttingDown() {
				logging.Info("Burn cancelled due to server shutdown", "upload_id", uploadID)
				if err := database.FailBurnJob(uploadID, "Server shutting down - burn interrupted"); err != nil {
					logging.Error("Failed to mark burn job as failed", "video_id", uploadID, "error", err)
				}
				return
			}

			// Get segments for SRT generation
			segments, err := transcription.GetSegments()
			if err != nil || len(segments) == 0 {
				logging.Error("No segments available", "error", err)
				if err := database.FailBurnJob(uploadID, "No subtitle segments available"); err != nil {
					logging.Error("Failed to mark burn job as failed", "video_id", uploadID, "error", err)
				}
				return
			}

			if err := database.UpdateBurnJobStatus(uploadID, "processing", "Generating subtitles...", 10); err != nil {
				logging.Error("Failed to update burn job status", "video_id", uploadID, "error", err)
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

			// Write SRT to temp file
			srtContent := generateSRT(whisperResult)
			srtPath := filepath.Join(uploadDir, uploadID+"_burn.srt")
			if err := os.WriteFile(srtPath, []byte(srtContent), 0644); err != nil {
				logging.Error("Failed to write SRT file", "error", err)
				if err := database.FailBurnJob(uploadID, fmt.Sprintf("Failed to write SRT file: %v", err)); err != nil {
					logging.Error("Failed to mark burn job as failed", "video_id", uploadID, "error", err)
				}
				return
			}
			defer removeWithLogging(srtPath, "temp SRT file cleanup after burn")

			progressMsg := "Burning subtitles into video..."
			if mode == "embed" {
				progressMsg = "Embedding subtitle track..."
			}
			if err := database.UpdateBurnJobStatus(uploadID, "processing", progressMsg, 20); err != nil {
				logging.Error("Failed to update burn job status", "video_id", uploadID, "error", err)
			}

			// Output to a temp file first, then encrypt
			outputPath := filepath.Join(uploadDir, uploadID+"_burned.mp4")
			// Defer removal of unencrypted output file (cleaned up even on panic/error)
			// Note: This is a no-op if the file doesn't exist or was already removed
			defer removeWithLogging(outputPath, "unencrypted burn output cleanup")

			// Check for shutdown before starting ffmpeg (the long-running operation)
			if isShuttingDown() {
				logging.Info("Burn cancelled due to server shutdown", "upload_id", uploadID)
				if err := database.FailBurnJob(uploadID, "Server shutting down - burn interrupted"); err != nil {
					logging.Error("Failed to mark burn job as failed", "video_id", uploadID, "error", err)
				}
				return
			}

			var cmd *exec.Cmd
			if mode == "embed" {
				// Embed mode: Create soft subtitle track (much faster, no re-encoding)
				// Uses mov_text codec which is compatible with MP4/MOV containers
				// Subtitles can be toggled on/off by the player
				cmd = exec.Command("ffmpeg",
					"-i", workingVideoPath,
					"-i", srtPath,
					"-c:v", "copy", // Copy video stream (no re-encoding)
					"-c:a", "copy", // Copy audio stream (no re-encoding)
					"-c:s", "mov_text", // Embed subtitles as text track
					"-y",
					outputPath,
				)
			} else {
				// Burn mode: Hardcode subtitles into video frames (slower, re-encodes video)
				// Build subtitle style with optional font for Indic script support
				// FontName is added if SUBTITLE_FONT env var is set, enabling proper rendering
				// of Hindi, Tamil, Telugu and other scripts that require specific fonts
				subtitleStyle := "FontSize=24,PrimaryColour=&HFFFFFF,OutlineColour=&H000000,Outline=2"
				if subtitleFont != "" {
					// Escape font name for ffmpeg filter syntax (remove quotes, colons, semicolons)
					safeFontName := strings.NewReplacer(`'`, ``, `"`, ``, `:`, ``, `;`, ``).Replace(subtitleFont)
					subtitleStyle = fmt.Sprintf("FontName=%s,%s", safeFontName, subtitleStyle)
				}

				cmd = exec.Command("ffmpeg",
					"-i", workingVideoPath,
					"-vf", fmt.Sprintf("subtitles='%s':force_style='%s'", escapeFFmpegFilterPath(srtPath), subtitleStyle),
					"-c:a", "copy",
					"-y",
					outputPath,
				)
			}

			// Start progress update goroutine for burn operation
			// FFmpeg doesn't provide progress callbacks, so we simulate progress
			// by incrementing from 20% to 85% in steps based on video duration
			// Use context for cancellation so it's safe to call cancel multiple times
			burnProgressCtx, cancelBurnProgress := context.WithCancel(context.Background())
			defer cancelBurnProgress() // Ensure cleanup even if we panic
			go func() {
				// Use video duration to estimate tick interval
				// Shorter videos = shorter intervals, longer videos = longer intervals
				// Embed mode is much faster (no re-encoding), burn mode takes ~1x video duration
				estimatedBurnTime := transcription.Duration
				if mode == "embed" {
					// Embed mode is fast - just copying streams plus adding subtitle track
					// Estimate ~5-10 seconds for most videos
					estimatedBurnTime = 10
				}
				if estimatedBurnTime < 10 {
					estimatedBurnTime = 10 // Minimum 10 seconds
				}
				if estimatedBurnTime > 600 {
					estimatedBurnTime = 600 // Cap at 10 minutes
				}

				// Calculate tick interval to go from 20% to 85% (65 points) during burn
				numTicks := 13 // 65 / 5 = 13 updates of 5% each
				tickInterval := time.Duration(estimatedBurnTime/float64(numTicks)) * time.Second
				if tickInterval < time.Second {
					tickInterval = time.Second
				}

				ticker := time.NewTicker(tickInterval)
				defer ticker.Stop()
				progress := 20
				statusMsg := progressMsg
				for {
					select {
					case <-burnProgressCtx.Done():
						return
					case <-shutdownCtx.Done():
						return
					case <-ticker.C:
						if progress < 85 {
							progress += 5
							if err := database.UpdateBurnJobStatus(uploadID, "processing", statusMsg, progress); err != nil {
								logging.Error("Failed to update burn job status", "video_id", uploadID, "error", err)
							}
						}
					}
				}
			}()

			cmdOutput, err := cmd.CombinedOutput()
			cancelBurnProgress() // Stop progress updates

			if err != nil {
				logging.Error("ffmpeg burn subtitles failed", "error", err, "output", string(cmdOutput))
				if err := database.FailBurnJob(uploadID, fmt.Sprintf("Failed to burn subtitles: %v", err)); err != nil {
					logging.Error("Failed to mark burn job as failed", "video_id", uploadID, "error", err)
				}
				return
			}

			if err := database.UpdateBurnJobStatus(uploadID, "processing", "Encrypting output...", 90); err != nil {
				logging.Error("Failed to update burn job status", "video_id", uploadID, "error", err)
			}

			// Encrypt the output file with current key version
			encOutputPath, keyVersion, err := multiEnc.EncryptFile(outputPath)
			if err != nil {
				logging.Error("Failed to encrypt burned video", "error", err)
				// Note: outputPath is cleaned up by defer above
				if err := database.FailBurnJob(uploadID, fmt.Sprintf("Failed to encrypt output: %v", err)); err != nil {
					logging.Error("Failed to mark burn job as failed", "video_id", uploadID, "error", err)
				}
				return
			}
			// Note: outputPath (unencrypted file) is cleaned up by defer above

			logging.Info("Subtitle burn complete", "upload_id", uploadID, "output_path", encOutputPath, "key_version", keyVersion, "mode", mode)
			if err := database.CompleteBurnJobWithKeyVersion(uploadID, encOutputPath, keyVersion); err != nil {
				logging.Error("Failed to complete burn job", "video_id", uploadID, "error", err)
			}
		}(keyVersion, string(burnMode))

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

func registerTextHandlers(mux *http.ServeMux) {
	// Script detection endpoint (rate limited)
	mux.HandleFunc("POST /api/text/detect-script", scriptLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {

		var req struct {
			Text string `json:"text"`
		}
		if err := httputil.DecodeJSONBody(r, w, &req, 0); err != nil {
			if err.Error() == "http: request body too large" {
				httputil.RespondError(w, http.StatusRequestEntityTooLarge, "Request body too large")
			} else {
				httputil.RespondError(w, http.StatusBadRequest, "Invalid request body")
			}
			return
		}

		if len(req.Text) > 10240 { // 10KB limit
			httputil.RespondError(w, http.StatusBadRequest, "Text too long (max 10KB)")
			return
		}

		detectedScript := script.DetectScript(req.Text)
		detectedLang := script.DetectLanguageFromRomanized(req.Text)

		// Calculate confidence based on character count
		confidence := 0.0
		if detectedScript != script.ScriptUnknown {
			// Simple confidence: more characters = higher confidence
			confidence = math.Min(float64(len(req.Text))/100.0, 1.0)
		}

		httputil.RespondJSON(w, http.StatusOK, map[string]interface{}{
			"detected_script":   string(detectedScript),
			"detected_language": detectedLang,
			"confidence":        confidence,
		})
	}))

	// Script conversion endpoint (rate limited)
	mux.HandleFunc("POST /api/text/convert", scriptLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {

		var req struct {
			Text         string `json:"text"`
			SourceScript string `json:"source_script,omitempty"` // Optional, auto-detected if omitted
			TargetScript string `json:"target_script"`
			Language     string `json:"language"` // Required for romanized input
		}
		if err := httputil.DecodeJSONBody(r, w, &req, 0); err != nil {
			if err.Error() == "http: request body too large" {
				httputil.RespondError(w, http.StatusRequestEntityTooLarge, "Request body too large")
			} else {
				httputil.RespondError(w, http.StatusBadRequest, "Invalid request body")
			}
			return
		}

		// Validate text length (10KB limit)
		if len(req.Text) > 10240 {
			httputil.RespondError(w, http.StatusBadRequest, "Text too long (max 10KB)")
			return
		}

		// Validate language
		if !script.IsLanguageSupported(req.Language) {
			httputil.RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
				"error":               "Unsupported language",
				"supported_languages": script.SupportedLanguages(),
			})
			return
		}

		// Validate target script
		targetScript := script.Script(req.TargetScript)
		if !script.IsScriptSupported(targetScript) {
			httputil.RespondJSON(w, http.StatusBadRequest, map[string]interface{}{
				"error":             "Unsupported target script",
				"supported_scripts": script.SupportedScripts(),
			})
			return
		}

		// Auto-detect source script if not provided
		sourceScript := script.Script(req.SourceScript)
		if sourceScript == "" {
			sourceScript = script.DetectScript(req.Text)
		}

		// Perform conversion
		converter := script.NewConverter()
		converted, err := converter.Convert(req.Text, req.Language, targetScript)
		if err != nil {
			httputil.RespondErrorf(w, http.StatusInternalServerError, "Conversion failed: %v", err)
			return
		}

		httputil.RespondJSON(w, http.StatusOK, map[string]interface{}{
			"original":      req.Text,
			"converted":     converted,
			"source_script": string(sourceScript),
			"target_script": string(targetScript),
			"language":      req.Language,
		})
	}))
}
