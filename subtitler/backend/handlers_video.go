package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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
	"github.com/tpott/subtitler/backend/security"
)

func registerVideoHandlers(mux *http.ServeMux) { //nolint:funlen // route registration
	// Delegate subtitle download/extraction handlers to separate file
	registerVideoSubtitleHandlers(mux)

	// Delegate burn/embed handlers to separate file
	registerVideoBurnHandlers(mux)

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
		if offset > maxPaginationOffset {
			httputil.RespondError(w, http.StatusBadRequest, "Offset exceeds maximum allowed value")
			return
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

		// Batch fetch transcription statuses (single query instead of N+1)
		videoIDs := make([]string, len(result.Videos))
		for i, v := range result.Videos {
			videoIDs[i] = v.ID
		}
		statusMap, err := database.GetTranscriptionStatuses(videoIDs)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting transcription statuses", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to list videos")
			return
		}

		videos := make([]VideoWithStatus, len(result.Videos))
		for i, v := range result.Videos {
			videos[i] = VideoWithStatus{Video: v, TranscriptionStatus: "none"}
			if status, ok := statusMap[v.ID]; ok {
				videos[i].TranscriptionStatus = status
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

		// Check ownership - either authenticated user owns it, or anonymous session matches
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
}
