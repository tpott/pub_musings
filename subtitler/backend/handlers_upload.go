package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/tpott/subtitler/backend/audio"
	"github.com/tpott/subtitler/backend/auth"
	"github.com/tpott/subtitler/backend/db"
	"github.com/tpott/subtitler/backend/httputil"
	"github.com/tpott/subtitler/backend/language"
	"github.com/tpott/subtitler/backend/logging"
	"github.com/tpott/subtitler/backend/metrics"
	"github.com/tpott/subtitler/backend/validation"
)

func registerUploadHandlers(mux *http.ServeMux) { //nolint:funlen // route registration
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
		totalWritten, err := assembleChunks(r.Context(), chunks, destPath)
		if err != nil {
			metrics.RecordUploadFailed()
			httputil.RespondError(w, http.StatusInternalServerError, err.Error())
			return
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

		// Process video metadata (thumbnail, subtitles, language hints)
		encThumbPath, embeddedSubtitlesJSON, languageHints := processVideoMetadata(r.Context(), destPath, uploadID, session.Filename)

		// Encrypt and persist to database
		err = encryptAndPersistVideo(r.Context(), destPath, uploadID, totalWritten, session, encThumbPath, embeddedSubtitlesJSON)
		if err != nil {
			metrics.RecordUploadFailed()
			httputil.RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Create transcription record and clean up session
		finalizeUploadSession(r.Context(), uploadID, req.UploadSessionID, chunks)

		// Record metrics and respond
		metrics.RecordUploadSuccess()
		metrics.RecordUploadBytes(totalWritten)

		logging.InfoContext(r.Context(), "Completed chunked upload",
			"session_id", req.UploadSessionID,
			"upload_id", uploadID,
			"filename", session.Filename,
			"size", totalWritten)

		response := map[string]interface{}{
			"status":    "success",
			"upload_id": uploadID,
			"filename":  session.Filename,
			"size":      totalWritten,
			"message":   fmt.Sprintf("File uploaded successfully (%d bytes)", totalWritten),
		}
		if languageHints != nil && len(languageHints.Hints) > 0 {
			response["language_hints"] = languageHints
		}
		httputil.RespondJSON(w, http.StatusOK, response)
	}))

	// Get chunked upload status (rate limited: 30/min per IP)
	mux.HandleFunc("GET /api/upload/status/{session_id}", downloadLimiter.Wrap(func(w http.ResponseWriter, r *http.Request) {

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
