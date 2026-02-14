package api

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/tpott/pub_musings/peekaboo/backend/logging"
)

const (
	maxUploadPhotoSize = 10 << 20 // 10 MB
	maxUploadAudioSize = 5 << 20  // 5 MB
	maxUploadVideoSize = 50 << 20 // 50 MB
	maxUploadTotal     = 70 << 20 // 70 MB total (generous)
)

// adminMediaResponse is the response from POST /api/admin/media.
type adminMediaResponse struct {
	ConceptID string `json:"concept_id,omitempty"`
	Set       string `json:"set,omitempty"`
	PhotoPath string `json:"photo_path,omitempty"`
	AudioPath string `json:"audio_path,omitempty"`
	VideoPath string `json:"video_path,omitempty"`
	Error     string `json:"error,omitempty"`
}

// allowedImageTypes maps MIME types to file extensions for photos.
var allowedImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

// allowedAudioTypes maps MIME types to file extensions for audio.
var allowedAudioTypes = map[string]string{
	"audio/mpeg": ".mp3",
	"audio/wav":  ".wav",
	"audio/ogg":  ".ogg",
}

// allowedVideoTypes maps MIME types to file extensions for video.
var allowedVideoTypes = map[string]string{
	"video/mp4":  ".mp4",
	"video/webm": ".webm",
}

// HandleUploadMedia handles POST /api/admin/media.
func (h *AdminHandler) HandleUploadMedia(w http.ResponseWriter, r *http.Request) {
	userID := h.authenticateAdmin(w, r)
	if userID == "" {
		return
	}

	// Limit total upload size
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadTotal)

	if err := r.ParseMultipartForm(maxUploadTotal); err != nil {
		if strings.Contains(err.Error(), "http: request body too large") {
			writeJSON(w, http.StatusRequestEntityTooLarge, adminMediaResponse{Error: "upload too large (max 70MB total)"})
			return
		}
		writeJSON(w, http.StatusBadRequest, adminMediaResponse{Error: "invalid multipart form"})
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	// Validate concept_id
	conceptID := strings.TrimSpace(r.FormValue("concept_id"))
	if conceptID == "" {
		writeJSON(w, http.StatusBadRequest, adminMediaResponse{Error: "concept_id is required"})
		return
	}
	if !validConceptPattern.MatchString(conceptID) || len(conceptID) > maxConceptLength {
		writeJSON(w, http.StatusBadRequest, adminMediaResponse{Error: "invalid concept_id format"})
		return
	}

	// Verify concept exists
	name, err := h.DB.GetConcept(conceptID)
	if err != nil {
		slog.Error("admin media: failed to look up concept",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, adminMediaResponse{Error: "internal error"})
		return
	}
	if name == "" {
		writeJSON(w, http.StatusBadRequest, adminMediaResponse{Error: "concept not found"})
		return
	}

	// Photo is required
	photoFile, photoHeader, err := r.FormFile("photo")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, adminMediaResponse{Error: "photo file is required"})
		return
	}
	defer photoFile.Close()

	// Validate photo
	photoType := photoHeader.Header.Get("Content-Type")
	photoExt, ok := allowedImageTypes[photoType]
	if !ok {
		writeJSON(w, http.StatusBadRequest, adminMediaResponse{Error: "photo must be JPEG, PNG, WebP, or GIF"})
		return
	}
	if photoHeader.Size > maxUploadPhotoSize {
		writeJSON(w, http.StatusBadRequest, adminMediaResponse{Error: "photo too large (max 10MB)"})
		return
	}

	// Optional audio
	var audioExt string
	audioFile, audioHeader, err := r.FormFile("audio")
	if err == nil {
		defer audioFile.Close()
		audioType := audioHeader.Header.Get("Content-Type")
		ext, ok := allowedAudioTypes[audioType]
		if !ok {
			writeJSON(w, http.StatusBadRequest, adminMediaResponse{Error: "audio must be MP3, WAV, or OGG"})
			return
		}
		if audioHeader.Size > maxUploadAudioSize {
			writeJSON(w, http.StatusBadRequest, adminMediaResponse{Error: "audio too large (max 5MB)"})
			return
		}
		audioExt = ext
	}

	// Optional video
	var videoExt string
	videoFile, videoHeader, err := r.FormFile("video")
	if err == nil {
		defer videoFile.Close()
		videoType := videoHeader.Header.Get("Content-Type")
		ext, ok := allowedVideoTypes[videoType]
		if !ok {
			writeJSON(w, http.StatusBadRequest, adminMediaResponse{Error: "video must be MP4 or WebM"})
			return
		}
		if videoHeader.Size > maxUploadVideoSize {
			writeJSON(w, http.StatusBadRequest, adminMediaResponse{Error: "video too large (max 50MB)"})
			return
		}
		videoExt = ext
	}

	// Determine next set number
	setNum, err := h.DB.NextMediaSetNumber(conceptID)
	if err != nil {
		slog.Error("admin media: failed to get next set number",
			"error", err,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, adminMediaResponse{Error: "internal error"})
		return
	}
	setName := fmt.Sprintf("set%d", setNum)

	// Resolve media directory
	mediaDir := os.Getenv("MEDIA_DIR")
	if mediaDir == "" {
		mediaDir = "data/media"
	}

	// Create set directory
	setDir := filepath.Join(mediaDir, conceptID, setName)
	if err := os.MkdirAll(setDir, 0755); err != nil {
		slog.Error("admin media: failed to create set directory",
			"error", err,
			"path", setDir,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, adminMediaResponse{Error: "internal error"})
		return
	}

	// Save photo
	photoFileName := "photo" + photoExt
	photoPath := filepath.Join(setDir, photoFileName)
	if err := saveUploadedFile(photoFile, photoPath); err != nil {
		slog.Error("admin media: failed to save photo",
			"error", err,
			"path", photoPath,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, adminMediaResponse{Error: "failed to save photo"})
		return
	}

	// Database paths (relative, matching seedMediaFromDisk convention)
	dbPhotoPath := filepath.Join("data/media", conceptID, setName, photoFileName)
	dbAudioPath := ""
	dbVideoPath := ""

	// Save audio if provided
	if audioFile != nil {
		audioFileName := "audio" + audioExt
		audioPath := filepath.Join(setDir, audioFileName)
		if err := saveUploadedFile(audioFile, audioPath); err != nil {
			slog.Error("admin media: failed to save audio",
				"error", err,
				"path", audioPath,
				"request_id", logging.GetRequestID(r.Context()))
			writeJSON(w, http.StatusInternalServerError, adminMediaResponse{Error: "failed to save audio"})
			return
		}
		dbAudioPath = filepath.Join("data/media", conceptID, setName, audioFileName)
	}

	// Save video if provided
	if videoFile != nil {
		videoFileName := "video" + videoExt
		videoPath := filepath.Join(setDir, videoFileName)
		if err := saveUploadedFile(videoFile, videoPath); err != nil {
			slog.Error("admin media: failed to save video",
				"error", err,
				"path", videoPath,
				"request_id", logging.GetRequestID(r.Context()))
			writeJSON(w, http.StatusInternalServerError, adminMediaResponse{Error: "failed to save video"})
			return
		}
		dbVideoPath = filepath.Join("data/media", conceptID, setName, videoFileName)
	}

	// Insert into database
	if err := h.DB.SeedMediaSet(conceptID, dbPhotoPath, dbAudioPath, dbVideoPath); err != nil {
		slog.Error("admin media: failed to insert media set",
			"error", err,
			"concept_id", conceptID,
			"request_id", logging.GetRequestID(r.Context()))
		writeJSON(w, http.StatusInternalServerError, adminMediaResponse{Error: "failed to register media set"})
		return
	}

	slog.Info("admin uploaded media set",
		"user_id", userID,
		"concept_id", conceptID,
		"set", setName,
		"photo", dbPhotoPath,
		"audio", dbAudioPath,
		"video", dbVideoPath,
		"request_id", logging.GetRequestID(r.Context()))

	resp := adminMediaResponse{
		ConceptID: conceptID,
		Set:       setName,
		PhotoPath: dbPhotoPath,
	}
	if dbAudioPath != "" {
		resp.AudioPath = dbAudioPath
	}
	if dbVideoPath != "" {
		resp.VideoPath = dbVideoPath
	}

	writeJSON(w, http.StatusCreated, resp)
}

// saveUploadedFile writes the contents of an uploaded file to disk.
func saveUploadedFile(src io.Reader, destPath string) error {
	dst, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("create file %s: %w", destPath, err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("write file %s: %w", destPath, err)
	}

	return nil
}
