package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tpott/subtitler/backend/align"
	"github.com/tpott/subtitler/backend/audio"
	"github.com/tpott/subtitler/backend/auth"
	"github.com/tpott/subtitler/backend/db"
	"github.com/tpott/subtitler/backend/errmsg"
	"github.com/tpott/subtitler/backend/httputil"
	"github.com/tpott/subtitler/backend/language"
	"github.com/tpott/subtitler/backend/logging"
	"github.com/tpott/subtitler/backend/metrics"
	"github.com/tpott/subtitler/backend/script"
	"github.com/tpott/subtitler/backend/validation"
)

func registerUploadHandlers(mux *http.ServeMux) {
	// Upload endpoint - accepts video files (rate limited: 10/min per IP)
	mux.HandleFunc("POST /api/upload", uploadLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {

		// Get authenticated user (if any)
		token := auth.GetTokenFromRequest(r)
		user, _, _ := auth.ValidateSession(database, token)

		// Get session_id from form for anonymous session tracking
		sessionID := r.URL.Query().Get("session_id")

		// Enforce upload limit for anonymous users (2 uploads max)
		if user == nil && sessionID != "" {
			count, err := database.CountVideosBySession(sessionID)
			if err != nil {
				logging.ErrorContext(r.Context(), "Error counting videos for session", "error", err)
			} else if count >= 2 {
				httputil.RespondError(w, http.StatusForbidden, "Anonymous users are limited to 2 uploads. Please register to upload more videos.")
				return
			}
		}

		// Limit request body size
		r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)

		// Parse multipart form
		if err := r.ParseMultipartForm(maxUploadSize); err != nil {
			logging.ErrorContext(r.Context(), "Error parsing form", "error", err)
			metrics.RecordUploadFailed()
			httputil.RespondError(w, http.StatusBadRequest, "File too large or invalid form data")
			return
		}

		// Get the file from the form
		file, header, err := r.FormFile("video")
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting form file", "error", err)
			metrics.RecordUploadFailed()
			httputil.RespondError(w, http.StatusBadRequest, "No video file provided")
			return
		}
		defer file.Close()

		// Validate file is not empty/zero-size
		if header.Size == 0 {
			metrics.RecordUploadFailed()
			httputil.RespondError(w, http.StatusBadRequest, "File is empty. Please upload a valid video file.")
			return
		}

		// Validate file type by checking content type against whitelist
		contentType := header.Header.Get("Content-Type")
		allowedMIMETypes := map[string]bool{
			"video/mp4":        true,
			"video/webm":       true,
			"video/quicktime":  true, // .mov files
			"video/x-m4v":      true, // .m4v files
			"video/mpeg":       true, // .mpeg, .mpg files
			"video/x-msvideo":  true, // .avi files
			"video/x-matroska": true, // .mkv files
			"video/ogg":        true, // .ogv files
		}
		if !allowedMIMETypes[contentType] {
			httputil.RespondJSON(w, http.StatusBadRequest, map[string]string{
				"error":             "Unsupported video format. Allowed formats: MP4, WebM, MOV, M4V, MPEG, AVI, MKV, OGV",
				"provided_mimetype": contentType,
			})
			return
		}

		// Validate magic bytes (file signature) - defense against MIME spoofing
		// Read first 12 bytes to check signature, then seek back to start
		magicBytes := make([]byte, 12)
		n, err := file.Read(magicBytes)
		if err != nil || n < 8 {
			httputil.RespondError(w, http.StatusBadRequest, "File is too small or could not be read")
			return
		}
		// Seek back to beginning for copy later
		if seeker, ok := file.(io.Seeker); ok {
			if _, err := seeker.Seek(0, io.SeekStart); err != nil {
				logging.ErrorContext(r.Context(), "Failed to seek file", "error", err)
				metrics.RecordUploadFailed()
				httputil.RespondError(w, http.StatusInternalServerError, "Failed to process file")
				return
			}
		}
		// Validate the magic bytes against known video formats
		if err := audio.ValidateMagicBytes(bytes.NewReader(magicBytes[:n])); err != nil {
			logging.WarnContext(r.Context(), "Invalid magic bytes", "mimetype", contentType, "error", err)
			httputil.RespondError(w, http.StatusBadRequest, "File content does not match a valid video format. The file may be corrupted or renamed.")
			return
		}

		// Generate unique ID for this upload
		uploadID, err := generateID()
		if err != nil {
			logging.ErrorContext(r.Context(), "Error generating upload ID", "error", err)
			metrics.RecordUploadFailed()
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to generate upload ID")
			return
		}

		// Get file extension from original filename and sanitize it
		ext := filepath.Ext(header.Filename)
		ext, err = validation.SanitizeFileExtension(ext)
		if err != nil {
			logging.WarnContext(r.Context(), "Invalid file extension", "filename", header.Filename, "error", err)
			httputil.RespondError(w, http.StatusBadRequest, "Invalid file extension")
			return
		}

		// Create destination file
		destPath := filepath.Join(uploadDir, uploadID+ext)
		destFile, err := os.Create(destPath)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error creating destination file", "error", err)
			metrics.RecordUploadFailed()
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to save file")
			return
		}
		defer destFile.Close()

		// Copy the uploaded file to destination
		written, err := io.Copy(destFile, file)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error copying file", "error", err)
			removeWithLogging(destPath, "partial file cleanup after copy error")
			metrics.RecordUploadFailed()
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to save file")
			return
		}
		// Close before encrypting - log any close error but continue since data is written
		if err := destFile.Close(); err != nil {
			logging.WarnContext(r.Context(), "Error closing destination file", "path", destPath, "error", err)
		}

		logging.InfoContext(r.Context(), "Uploaded file", "filename", header.Filename, "bytes", written, "dest_path", destPath)

		// Validate the file is actually a valid video (defense-in-depth beyond MIME check)
		if err := audio.ValidateVideoFile(destPath); err != nil {
			logging.WarnContext(r.Context(), "Video validation failed", "path", destPath, "error", err)
			removeWithLogging(destPath, "invalid video file cleanup")
			httputil.RespondError(w, http.StatusBadRequest, "File is not a valid video. Please upload a valid video file.")
			return
		}

		// Generate thumbnail from the validated video (before encryption)
		thumbPath := filepath.Join(uploadDir, uploadID+"_thumb.jpg")
		var encThumbPath *string
		if err := audio.GenerateThumbnail(destPath, thumbPath); err != nil {
			logging.WarnContext(r.Context(), "Failed to generate thumbnail", "path", destPath, "error", err)
			// Non-fatal: continue without thumbnail
		} else {
			// Encrypt the thumbnail with current key version
			encPath, _, err := multiEnc.EncryptFile(thumbPath)
			if err != nil {
				logging.WarnContext(r.Context(), "Failed to encrypt thumbnail", "error", err)
				removeWithLogging(thumbPath, "unencrypted thumbnail cleanup after encryption error")
			} else {
				removeWithLogging(thumbPath, "unencrypted thumbnail cleanup after successful encryption")
				encThumbPath = &encPath
				logging.InfoContext(r.Context(), "Generated and encrypted thumbnail", "thumb_path", encPath)
			}
		}

		// Detect embedded subtitle tracks before encryption
		var embeddedSubtitlesJSON *string
		subtitleTracks, err := audio.GetSubtitleTracks(destPath)
		if err != nil {
			logging.WarnContext(r.Context(), "Failed to detect embedded subtitles", "error", err)
			// Non-fatal: continue without subtitle detection
		} else if len(subtitleTracks) > 0 {
			// Serialize subtitle tracks to JSON
			subtitlesData, err := json.Marshal(subtitleTracks)
			if err != nil {
				logging.WarnContext(r.Context(), "Failed to serialize subtitle tracks", "error", err)
			} else {
				jsonStr := string(subtitlesData)
				embeddedSubtitlesJSON = &jsonStr
				logging.InfoContext(r.Context(), "Detected embedded subtitle tracks", "count", len(subtitleTracks))
			}
		}

		// Detect language hints from video metadata and filename (before encryption)
		languageHints := language.Detect(destPath, header.Filename)
		if languageHints != nil && len(languageHints.Hints) > 0 {
			logging.InfoContext(r.Context(), "Detected language hints",
				"suggested_language", languageHints.SuggestedLanguage,
				"confidence", languageHints.SuggestedConfidence,
				"hint_count", len(languageHints.Hints))
		}

		// Encrypt the file at rest with current key version
		encPath, keyVersion, err := multiEnc.EncryptFile(destPath)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error encrypting file", "error", err)
			removeWithLogging(destPath, "unencrypted upload after encryption failure")
			metrics.RecordUploadFailed()
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to encrypt file")
			return
		}

		// Remove the unencrypted file
		removeWithLogging(destPath, "unencrypted upload after successful encryption")
		logging.InfoContext(r.Context(), "Encrypted file", "src_path", destPath, "enc_path", encPath, "key_version", keyVersion)

		// Save video to database with encrypted file path and key version
		video := &db.Video{
			ID:                    uploadID,
			Filename:              header.Filename,
			Size:                  written,
			ContentType:           contentType,
			FilePath:              encPath,
			ThumbnailPath:         encThumbPath,
			KeyVersion:            keyVersion,
			EmbeddedSubtitlesJSON: embeddedSubtitlesJSON,
			CreatedAt:             time.Now(),
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
			logging.ErrorContext(r.Context(), "Error saving video to database", "error", err)
			removeWithLogging(encPath, "encrypted file after DB save failure")
			if encThumbPath != nil {
				removeWithLogging(*encThumbPath, "encrypted thumbnail after DB save failure")
			}
			metrics.RecordUploadFailed()
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to save video record")
			return
		}

		// Create initial transcription record
		transcriptionID, err := generateID()
		if err != nil {
			logging.ErrorContext(r.Context(), "Error generating transcription ID", "error", err)
			removeWithLogging(encPath, "encrypted file after transcription ID generation failure")
			metrics.RecordUploadFailed()
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to generate transcription ID")
			return
		}
		transcription := &db.Transcription{
			ID:        transcriptionID,
			VideoID:   uploadID,
			Status:    "pending",
			Message:   "Video uploaded, ready for transcription",
			Progress:  0,
			CreatedAt: time.Now(),
		}
		if err := database.CreateTranscription(transcription); err != nil {
			logging.ErrorContext(r.Context(), "Error creating transcription record", "error", err)
			// Don't fail the upload, transcription record can be created later
		}

		// Record metrics
		metrics.RecordUploadSuccess()
		metrics.RecordUploadBytes(written)

		// Return success with upload ID and language hints
		response := map[string]interface{}{
			"status":    "success",
			"upload_id": uploadID,
			"filename":  header.Filename,
			"size":      written,
			"message":   fmt.Sprintf("File uploaded successfully (%d bytes)", written),
		}
		// Include language hints if any were detected
		if languageHints != nil && len(languageHints.Hints) > 0 {
			response["language_hints"] = languageHints
		}
		httputil.RespondJSON(w, http.StatusOK, response)
	}))

	// Initialize chunked upload session (rate limited: 10/min per IP)
	mux.HandleFunc("POST /api/upload/init", uploadLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		// Get authenticated user (if any)
		token := auth.GetTokenFromRequest(r)
		user, _, _ := auth.ValidateSession(database, token)

		// Parse request body
		var req struct {
			Filename    string `json:"filename"`
			Size        int64  `json:"size"`
			ContentType string `json:"content_type"`
			ChunkSize   int64  `json:"chunk_size"`
		}
		if err := httputil.DecodeJSONBody(r, w, &req, 0); err != nil {
			if err.Error() == "http: request body too large" {
				httputil.RespondError(w, http.StatusRequestEntityTooLarge, "Request body too large")
			} else {
				httputil.RespondError(w, http.StatusBadRequest, "Invalid request body")
			}
			return
		}

		// Validate required fields
		if req.Filename == "" || req.ContentType == "" {
			httputil.RespondError(w, http.StatusBadRequest, "Missing required fields: filename, content_type")
			return
		}

		// Validate file size is positive (reject zero-size files)
		if req.Size <= 0 {
			httputil.RespondError(w, http.StatusBadRequest, "File size must be greater than zero. Empty files are not allowed.")
			return
		}

		// Validate MIME type
		allowedMIMETypes := map[string]bool{
			"video/mp4":        true,
			"video/webm":       true,
			"video/quicktime":  true,
			"video/x-m4v":      true,
			"video/mpeg":       true,
			"video/x-msvideo":  true,
			"video/x-matroska": true,
			"video/ogg":        true,
		}
		if !allowedMIMETypes[req.ContentType] {
			httputil.RespondJSON(w, http.StatusBadRequest, map[string]string{
				"error":             "Unsupported video format. Allowed formats: MP4, WebM, MOV, M4V, MPEG, AVI, MKV, OGV",
				"provided_mimetype": req.ContentType,
			})
			return
		}

		// Validate total size
		if req.Size > maxUploadSize {
			httputil.RespondErrorf(w, http.StatusBadRequest, "File too large. Maximum size is %d MB", maxUploadSize/(1<<20))
			return
		}

		// Get session_id from query for anonymous tracking
		sessionID := r.URL.Query().Get("session_id")

		// Enforce upload limit for anonymous users
		if user == nil && sessionID != "" {
			count, err := database.CountVideosBySession(sessionID)
			if err != nil {
				logging.ErrorContext(r.Context(), "Error counting videos for session", "error", err)
			} else if count >= 2 {
				httputil.RespondError(w, http.StatusForbidden, "Anonymous users are limited to 2 uploads. Please register to upload more videos.")
				return
			}
		}

		// Use default chunk size if not specified
		requestedChunkSize := req.ChunkSize
		if requestedChunkSize <= 0 {
			requestedChunkSize = chunkSize
		}
		// Cap chunk size at server's configured limit
		if requestedChunkSize > chunkSize {
			requestedChunkSize = chunkSize
		}

		// Calculate total chunks
		totalChunks := int((req.Size + requestedChunkSize - 1) / requestedChunkSize)

		// Generate session ID
		uploadSessionID, err := generateID()
		if err != nil {
			logging.ErrorContext(r.Context(), "Error generating upload session ID", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to generate session ID")
			return
		}

		// Create session record
		expiresAt := time.Now().Add(sessionExpiry)
		session := &db.UploadSession{
			ID:          uploadSessionID,
			Filename:    req.Filename,
			ContentType: req.ContentType,
			TotalSize:   req.Size,
			ChunkSize:   requestedChunkSize,
			TotalChunks: totalChunks,
			Status:      "in_progress",
			CreatedAt:   time.Now(),
			ExpiresAt:   expiresAt,
		}
		if user != nil {
			session.UserID = &user.ID
		}
		if sessionID != "" {
			session.SessionID = &sessionID
		}

		if err := database.CreateUploadSession(session); err != nil {
			logging.ErrorContext(r.Context(), "Error creating upload session", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to create upload session")
			return
		}

		// Create chunks directory
		chunksDir := filepath.Join(uploadDir, "chunks", uploadSessionID)
		if err := os.MkdirAll(chunksDir, 0755); err != nil {
			logging.ErrorContext(r.Context(), "Error creating chunks directory", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to create chunks directory")
			return
		}

		logging.InfoContext(r.Context(), "Created chunked upload session",
			"session_id", uploadSessionID,
			"filename", req.Filename,
			"total_size", req.Size,
			"chunk_size", requestedChunkSize,
			"total_chunks", totalChunks)

		httputil.RespondJSON(w, http.StatusOK, map[string]interface{}{
			"upload_session_id": uploadSessionID,
			"chunk_size":        requestedChunkSize,
			"total_chunks":      totalChunks,
			"expires_at":        expiresAt.Format(time.RFC3339),
		})
	}))

	// Upload a single chunk (rate limited: 60/min per IP)
	mux.HandleFunc("POST /api/upload/chunk", chunkLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {

		// Limit chunk size
		r.Body = http.MaxBytesReader(w, r.Body, chunkSize+10*1024) // chunk + overhead

		// Parse multipart form
		if err := r.ParseMultipartForm(chunkSize + 10*1024); err != nil {
			logging.ErrorContext(r.Context(), "Error parsing chunk form", "error", err)
			httputil.RespondError(w, http.StatusBadRequest, "Chunk too large or invalid form data")
			return
		}

		// Get session ID and chunk index from form
		uploadSessionID := r.FormValue("upload_session_id")
		chunkIndexStr := r.FormValue("chunk_index")
		if uploadSessionID == "" || chunkIndexStr == "" {
			httputil.RespondError(w, http.StatusBadRequest, "Missing upload_session_id or chunk_index")
			return
		}

		// Defense-in-depth: validate session ID format before using in file paths
		if err := validation.ValidateHexID(uploadSessionID); err != nil {
			httputil.RespondError(w, http.StatusBadRequest, "Invalid upload_session_id format")
			return
		}

		chunkIndex, err := strconv.Atoi(chunkIndexStr)
		if err != nil || chunkIndex < 0 {
			httputil.RespondError(w, http.StatusBadRequest, "Invalid chunk_index")
			return
		}

		// Early upper bound validation - prevents integer overflow issues
		// Maximum reasonable chunks: 500GB / 50MB = 10,000 chunks
		const maxChunkIndex = 100000
		if chunkIndex >= maxChunkIndex {
			httputil.RespondError(w, http.StatusBadRequest, "Chunk index exceeds maximum allowed value")
			return
		}

		// Get session
		session, err := database.GetUploadSession(uploadSessionID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting upload session", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to get upload session")
			return
		}
		if session == nil {
			httputil.RespondError(w, http.StatusNotFound, "Upload session not found")
			return
		}

		// Check session expiration
		if time.Now().After(session.ExpiresAt) {
			httputil.RespondError(w, http.StatusGone, "Upload session has expired")
			return
		}

		// Check session status
		if session.Status != "in_progress" {
			httputil.RespondError(w, http.StatusBadRequest, "Upload session is not in progress")
			return
		}

		// Validate ownership
		token := auth.GetTokenFromRequest(r)
		user, _, _ := auth.ValidateSession(database, token)
		requestSessionID := r.URL.Query().Get("session_id")

		if session.UserID != nil {
			if user == nil || *session.UserID != user.ID {
				httputil.RespondError(w, http.StatusForbidden, "Access denied")
				return
			}
		} else if session.SessionID != nil {
			if requestSessionID == "" || *session.SessionID != requestSessionID {
				httputil.RespondError(w, http.StatusForbidden, "Access denied")
				return
			}
		}

		// Validate chunk index
		if chunkIndex >= session.TotalChunks {
			httputil.RespondError(w, http.StatusBadRequest, "Invalid chunk index")
			return
		}

		// Check if chunk already exists (idempotent)
		existingChunk, err := database.GetUploadChunk(uploadSessionID, chunkIndex)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error checking existing chunk", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to check chunk status")
			return
		}
		if existingChunk != nil {
			// Chunk already uploaded, return success for idempotency
			totalReceived, err := database.GetTotalReceivedBytes(uploadSessionID)
			if err != nil {
				logging.WarnContext(r.Context(), "Error getting total received bytes", "error", err, "session_id", uploadSessionID)
				totalReceived = 0
			}
			chunkCount, err := database.CountUploadChunks(uploadSessionID)
			if err != nil {
				logging.WarnContext(r.Context(), "Error counting upload chunks", "error", err, "session_id", uploadSessionID)
				chunkCount = 0
			}
			progress := int(float64(chunkCount) / float64(session.TotalChunks) * 100)
			httputil.RespondJSON(w, http.StatusOK, map[string]interface{}{
				"chunk_index":    chunkIndex,
				"received_bytes": existingChunk.Size,
				"total_received": totalReceived,
				"progress":       progress,
				"already_exists": true,
			})
			return
		}

		// Get the chunk file
		chunkFile, _, err := r.FormFile("chunk")
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting chunk file", "error", err)
			httputil.RespondError(w, http.StatusBadRequest, "No chunk data provided")
			return
		}
		defer chunkFile.Close()

		// Calculate expected chunk size
		expectedSize := session.ChunkSize
		if chunkIndex == session.TotalChunks-1 {
			// Last chunk may be smaller
			expectedSize = session.TotalSize - int64(chunkIndex)*session.ChunkSize
		}

		// Save chunk to disk
		chunkPath := filepath.Join(uploadDir, "chunks", uploadSessionID, fmt.Sprintf("chunk_%d.part", chunkIndex))
		destFile, err := os.Create(chunkPath)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error creating chunk file", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to save chunk")
			return
		}
		defer destFile.Close()

		written, err := io.Copy(destFile, chunkFile)
		if err != nil {
			removeWithLogging(chunkPath, "partial chunk after write failure")
			logging.ErrorContext(r.Context(), "Error writing chunk file", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to save chunk")
			return
		}
		// Close after writing - log any error but continue since data is written
		if err := destFile.Close(); err != nil {
			logging.WarnContext(r.Context(), "Error closing chunk destination file", "path", chunkPath, "error", err)
		}

		// Validate chunk size (allow up to expected size; last chunk may be smaller)
		if written > expectedSize {
			removeWithLogging(chunkPath, "oversized chunk")
			httputil.RespondErrorf(w, http.StatusBadRequest, "Chunk too large. Expected max %d bytes, got %d", expectedSize, written)
			return
		}

		// Record chunk in database
		chunkID, err := generateID()
		if err != nil {
			removeWithLogging(chunkPath, "chunk after ID generation failure")
			logging.ErrorContext(r.Context(), "Error generating chunk ID", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to generate chunk ID")
			return
		}

		chunk := &db.UploadChunk{
			ID:              chunkID,
			UploadSessionID: uploadSessionID,
			ChunkIndex:      chunkIndex,
			ChunkPath:       chunkPath,
			Size:            written,
			CreatedAt:       time.Now(),
		}
		if err := database.CreateUploadChunk(chunk); err != nil {
			removeWithLogging(chunkPath, "chunk after DB record failure")
			logging.ErrorContext(r.Context(), "Error saving chunk record", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to record chunk")
			return
		}

		// Calculate progress
		totalReceived, err := database.GetTotalReceivedBytes(uploadSessionID)
		if err != nil {
			logging.WarnContext(r.Context(), "Error getting total received bytes", "error", err, "session_id", uploadSessionID)
			totalReceived = 0
		}
		chunkCount, err := database.CountUploadChunks(uploadSessionID)
		if err != nil {
			logging.WarnContext(r.Context(), "Error counting upload chunks", "error", err, "session_id", uploadSessionID)
			chunkCount = 0
		}
		progress := int(float64(chunkCount) / float64(session.TotalChunks) * 100)

		logging.InfoContext(r.Context(), "Received chunk",
			"session_id", uploadSessionID,
			"chunk_index", chunkIndex,
			"size", written,
			"progress", progress)

		httputil.RespondJSON(w, http.StatusOK, map[string]interface{}{
			"chunk_index":    chunkIndex,
			"received_bytes": written,
			"total_received": totalReceived,
			"progress":       progress,
		})
	}))

	// Complete chunked upload (rate limited: 10/min per IP)
	mux.HandleFunc("POST /api/upload/complete", uploadLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			UploadSessionID string `json:"upload_session_id"`
		}
		if err := httputil.DecodeJSONBody(r, w, &req, 0); err != nil {
			if err.Error() == "http: request body too large" {
				httputil.RespondError(w, http.StatusRequestEntityTooLarge, "Request body too large")
			} else {
				httputil.RespondError(w, http.StatusBadRequest, "Invalid request body")
			}
			return
		}

		if req.UploadSessionID == "" {
			httputil.RespondError(w, http.StatusBadRequest, "Missing upload_session_id")
			return
		}

		// Defense-in-depth: validate session ID format before using in file paths
		if err := validation.ValidateHexID(req.UploadSessionID); err != nil {
			httputil.RespondError(w, http.StatusBadRequest, "Invalid upload_session_id format")
			return
		}

		// Get session
		session, err := database.GetUploadSession(req.UploadSessionID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting upload session", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to get upload session")
			return
		}
		if session == nil {
			httputil.RespondError(w, http.StatusNotFound, "Upload session not found")
			return
		}

		// Validate ownership
		token := auth.GetTokenFromRequest(r)
		user, _, _ := auth.ValidateSession(database, token)
		requestSessionID := r.URL.Query().Get("session_id")

		if session.UserID != nil {
			if user == nil || *session.UserID != user.ID {
				httputil.RespondError(w, http.StatusForbidden, "Access denied")
				return
			}
		} else if session.SessionID != nil {
			if requestSessionID == "" || *session.SessionID != requestSessionID {
				httputil.RespondError(w, http.StatusForbidden, "Access denied")
				return
			}
		}

		// Check session status
		if session.Status != "in_progress" {
			httputil.RespondError(w, http.StatusBadRequest, "Upload session is not in progress")
			return
		}

		// Verify all chunks received
		chunkCount, err := database.CountUploadChunks(req.UploadSessionID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error counting chunks", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to verify chunks")
			return
		}
		if chunkCount != session.TotalChunks {
			httputil.RespondErrorf(w, http.StatusBadRequest, "Not all chunks received. Expected %d, got %d", session.TotalChunks, chunkCount)
			return
		}

		// Get chunks in order
		chunks, err := database.GetUploadChunks(req.UploadSessionID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting chunks", "error", err)
			metrics.RecordUploadFailed()
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to get chunks")
			return
		}

		// Generate upload ID for the final file
		uploadID, err := generateID()
		if err != nil {
			logging.ErrorContext(r.Context(), "Error generating upload ID", "error", err)
			metrics.RecordUploadFailed()
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to generate upload ID")
			return
		}

		// Get file extension and sanitize it
		ext := filepath.Ext(session.Filename)
		ext, err = validation.SanitizeFileExtension(ext)
		if err != nil {
			logging.WarnContext(r.Context(), "Invalid file extension", "filename", session.Filename, "error", err)
			httputil.RespondError(w, http.StatusBadRequest, "Invalid file extension")
			return
		}

		// Reassemble chunks into final file
		destPath := filepath.Join(uploadDir, uploadID+ext)
		destFile, err := os.Create(destPath)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error creating destination file", "error", err)
			metrics.RecordUploadFailed()
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to create destination file")
			return
		}

		var totalWritten int64
		for _, chunk := range chunks {
			chunkFile, err := os.Open(chunk.ChunkPath)
			if err != nil {
				if closeErr := destFile.Close(); closeErr != nil {
					logging.WarnContext(r.Context(), "Error closing dest file after chunk open failure", "error", closeErr)
				}
				removeWithLogging(destPath, "partial assembled file after chunk open failure")
				logging.ErrorContext(r.Context(), "Error opening chunk file", "chunk_index", chunk.ChunkIndex, "error", err)
				metrics.RecordUploadFailed()
				httputil.RespondError(w, http.StatusInternalServerError, "Failed to read chunk")
				return
			}
			written, err := io.Copy(destFile, chunkFile)
			if closeErr := chunkFile.Close(); closeErr != nil {
				logging.WarnContext(r.Context(), "Error closing chunk file", "chunk_index", chunk.ChunkIndex, "error", closeErr)
			}
			if err != nil {
				if closeErr := destFile.Close(); closeErr != nil {
					logging.WarnContext(r.Context(), "Error closing dest file after copy failure", "error", closeErr)
				}
				removeWithLogging(destPath, "partial assembled file after copy failure")
				logging.ErrorContext(r.Context(), "Error copying chunk", "chunk_index", chunk.ChunkIndex, "error", err)
				metrics.RecordUploadFailed()
				httputil.RespondError(w, http.StatusInternalServerError, "Failed to assemble file")
				return
			}
			totalWritten += written
		}
		// Close assembled file - log any error but continue since data is written
		if err := destFile.Close(); err != nil {
			logging.WarnContext(r.Context(), "Error closing assembled file", "path", destPath, "error", err)
		}

		logging.InfoContext(r.Context(), "Reassembled chunks", "upload_id", uploadID, "total_bytes", totalWritten)

		// Validate magic bytes (file signature) - defense against MIME spoofing
		if err := audio.ValidateMagicBytesFromFile(destPath); err != nil {
			logging.WarnContext(r.Context(), "Invalid magic bytes for reassembled file", "path", destPath, "error", err)
			removeWithLogging(destPath, "assembled file with invalid magic bytes")
			httputil.RespondError(w, http.StatusBadRequest, "File content does not match a valid video format. The file may be corrupted.")
			return
		}

		// Validate the assembled file with ffprobe
		if err := audio.ValidateVideoFile(destPath); err != nil {
			logging.WarnContext(r.Context(), "Video validation failed", "path", destPath, "error", err)
			removeWithLogging(destPath, "assembled file that failed video validation")
			httputil.RespondError(w, http.StatusBadRequest, "Assembled file is not a valid video. Please try uploading again.")
			return
		}

		// Generate thumbnail
		thumbPath := filepath.Join(uploadDir, uploadID+"_thumb.jpg")
		var encThumbPath *string
		if err := audio.GenerateThumbnail(destPath, thumbPath); err != nil {
			logging.WarnContext(r.Context(), "Failed to generate thumbnail", "path", destPath, "error", err)
		} else {
			encPath, _, err := multiEnc.EncryptFile(thumbPath)
			if err != nil {
				logging.WarnContext(r.Context(), "Failed to encrypt thumbnail", "error", err)
				removeWithLogging(thumbPath, "unencrypted thumbnail after encryption failure")
			} else {
				removeWithLogging(thumbPath, "unencrypted thumbnail after successful encryption")
				encThumbPath = &encPath
			}
		}

		// Detect embedded subtitle tracks before encryption
		var embeddedSubtitlesJSON *string
		subtitleTracks, err := audio.GetSubtitleTracks(destPath)
		if err != nil {
			logging.WarnContext(r.Context(), "Failed to detect embedded subtitles", "error", err)
		} else if len(subtitleTracks) > 0 {
			subtitlesData, err := json.Marshal(subtitleTracks)
			if err != nil {
				logging.WarnContext(r.Context(), "Failed to serialize subtitle tracks", "error", err)
			} else {
				jsonStr := string(subtitlesData)
				embeddedSubtitlesJSON = &jsonStr
				logging.InfoContext(r.Context(), "Detected embedded subtitle tracks", "count", len(subtitleTracks))
			}
		}

		// Detect language hints from video metadata and filename (before encryption)
		languageHints := language.Detect(destPath, session.Filename)
		if languageHints != nil && len(languageHints.Hints) > 0 {
			logging.InfoContext(r.Context(), "Detected language hints",
				"suggested_language", languageHints.SuggestedLanguage,
				"confidence", languageHints.SuggestedConfidence,
				"hint_count", len(languageHints.Hints))
		}

		// Encrypt the file with current key version
		encPath, keyVersion, err := multiEnc.EncryptFile(destPath)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error encrypting file", "error", err)
			removeWithLogging(destPath, "unencrypted assembled file after encryption failure")
			metrics.RecordUploadFailed()
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to encrypt file")
			return
		}
		removeWithLogging(destPath, "unencrypted assembled file after successful encryption")

		// Save video to database with key version
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
			logging.ErrorContext(r.Context(), "Error saving video to database", "error", err)
			removeWithLogging(encPath, "encrypted file after DB save failure")
			if encThumbPath != nil {
				removeWithLogging(*encThumbPath, "encrypted thumbnail after DB save failure")
			}
			metrics.RecordUploadFailed()
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to save video record")
			return
		}

		// Create transcription record
		transcriptionID, err := generateID()
		if err != nil {
			logging.ErrorContext(r.Context(), "Error generating transcription ID", "error", err)
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
				logging.ErrorContext(r.Context(), "Error creating transcription record", "error", err)
			}
		}

		// Mark session as complete
		if err := database.UpdateUploadSessionStatus(req.UploadSessionID, "complete"); err != nil {
			logging.ErrorContext(r.Context(), "Error updating session status", "error", err)
		}

		// Clean up chunk files
		chunksDir := filepath.Join(uploadDir, "chunks", req.UploadSessionID)
		for _, chunk := range chunks {
			removeWithLogging(chunk.ChunkPath, "uploaded chunk after successful assembly")
		}
		removeWithLogging(chunksDir, "empty chunks directory after cleanup")

		// Record metrics
		metrics.RecordUploadSuccess()
		metrics.RecordUploadBytes(totalWritten)

		logging.InfoContext(r.Context(), "Completed chunked upload",
			"session_id", req.UploadSessionID,
			"upload_id", uploadID,
			"filename", session.Filename,
			"size", totalWritten)

		// Return success with language hints
		response := map[string]interface{}{
			"status":    "success",
			"upload_id": uploadID,
			"filename":  session.Filename,
			"size":      totalWritten,
			"message":   fmt.Sprintf("File uploaded successfully (%d bytes)", totalWritten),
		}
		// Include language hints if any were detected
		if languageHints != nil && len(languageHints.Hints) > 0 {
			response["language_hints"] = languageHints
		}
		httputil.RespondJSON(w, http.StatusOK, response)
	}))

	// Get chunked upload status (rate limited: 30/min per IP)
	mux.HandleFunc("GET /api/upload/status/{session_id}", scriptLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {

		uploadSessionID := r.PathValue("session_id")
		if uploadSessionID == "" {
			httputil.RespondError(w, http.StatusBadRequest, "Session ID required")
			return
		}

		// Defense-in-depth: validate session ID format
		if err := validation.ValidateHexID(uploadSessionID); err != nil {
			httputil.RespondError(w, http.StatusBadRequest, "Invalid session ID format")
			return
		}

		session, err := database.GetUploadSession(uploadSessionID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting upload session", "error", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to get upload session")
			return
		}
		if session == nil {
			httputil.RespondError(w, http.StatusNotFound, "Upload session not found")
			return
		}

		// Validate ownership
		token := auth.GetTokenFromRequest(r)
		user, _, _ := auth.ValidateSession(database, token)
		requestSessionID := r.URL.Query().Get("session_id")

		if session.UserID != nil {
			if user == nil || *session.UserID != user.ID {
				httputil.RespondError(w, http.StatusForbidden, "Access denied")
				return
			}
		} else if session.SessionID != nil {
			if requestSessionID == "" || *session.SessionID != requestSessionID {
				httputil.RespondError(w, http.StatusForbidden, "Access denied")
				return
			}
		}

		// Get received chunks
		receivedChunks, err := database.GetReceivedChunkIndices(uploadSessionID)
		if err != nil {
			logging.ErrorContext(r.Context(), "Error getting received chunks", "error", err)
			receivedChunks = []int{}
		}

		receivedBytes, _ := database.GetTotalReceivedBytes(uploadSessionID)
		progress := 0
		if session.TotalChunks > 0 {
			progress = int(float64(len(receivedChunks)) / float64(session.TotalChunks) * 100)
		}

		// Check if expired
		status := session.Status
		if time.Now().After(session.ExpiresAt) && status == "in_progress" {
			status = "expired"
		}

		httputil.RespondJSON(w, http.StatusOK, map[string]interface{}{
			"upload_session_id": session.ID,
			"filename":          session.Filename,
			"total_size":        session.TotalSize,
			"chunk_size":        session.ChunkSize,
			"total_chunks":      session.TotalChunks,
			"received_chunks":   receivedChunks,
			"received_bytes":    receivedBytes,
			"progress":          progress,
			"status":            status,
			"expires_at":        session.ExpiresAt.Format(time.RFC3339),
		})
	}))
}

func registerTranscriptionHandlers(mux *http.ServeMux) {
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
