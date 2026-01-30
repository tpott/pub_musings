package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tpott/subtitler/backend/audio"
	"github.com/tpott/subtitler/backend/auth"
	"github.com/tpott/subtitler/backend/httputil"
	"github.com/tpott/subtitler/backend/logging"
)

// registerVideoSubtitleHandlers registers handlers for subtitle download and extraction.
func registerVideoSubtitleHandlers(mux *http.ServeMux) { //nolint:funlen // route registration
	// Download SRT file for a transcription
	mux.HandleFunc("GET /api/videos/{id}/subtitles.srt", func(w http.ResponseWriter, r *http.Request) {
		uploadID, valid := validatePathID(w, r.PathValue("id"), "Upload ID")
		if !valid {
			return
		}

		// Check ownership - either authenticated user owns it, or anonymous session matches
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

		token := auth.GetTokenFromRequest(r)
		user, _, _ := auth.ValidateSession(database, token)
		sessionID := getValidSessionID(r)

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

		// Check ownership - either authenticated user owns it, or anonymous session matches
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

		token := auth.GetTokenFromRequest(r)
		user, _, _ := auth.ValidateSession(database, token)
		sessionID := getValidSessionID(r)

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

		// Check ownership - either authenticated user owns it, or anonymous session matches
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

		token := auth.GetTokenFromRequest(r)
		user, _, _ := auth.ValidateSession(database, token)
		sessionID := getValidSessionID(r)

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
		sessionID := getValidSessionID(r)

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
}
