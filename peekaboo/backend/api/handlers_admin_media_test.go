package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"testing"

	"github.com/tpott/pub_musings/peekaboo/backend/crypto"
)

// createFormFileWithType creates a multipart form file part with a specific Content-Type.
func createFormFileWithType(writer *multipart.Writer, fieldName, fileName, contentType string) (io.Writer, error) {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldName, fileName))
	h.Set("Content-Type", contentType)
	return writer.CreatePart(h)
}

// createMediaUploadRequest creates a multipart form request for media upload.
func createMediaUploadRequest(t *testing.T, conceptID string, photo []byte, audio []byte, video []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add concept_id field
	if err := writer.WriteField("concept_id", conceptID); err != nil {
		t.Fatalf("WriteField concept_id failed: %v", err)
	}

	// Add photo file with correct MIME type
	if photo != nil {
		part, err := createFormFileWithType(writer, "photo", "photo.jpg", "image/jpeg")
		if err != nil {
			t.Fatalf("CreateFormFile photo failed: %v", err)
		}
		if _, err := part.Write(photo); err != nil {
			t.Fatalf("Write photo failed: %v", err)
		}
	}

	// Add audio file with correct MIME type
	if audio != nil {
		part, err := createFormFileWithType(writer, "audio", "audio.mp3", "audio/mpeg")
		if err != nil {
			t.Fatalf("CreateFormFile audio failed: %v", err)
		}
		if _, err := part.Write(audio); err != nil {
			t.Fatalf("Write audio failed: %v", err)
		}
	}

	// Add video file with correct MIME type
	if video != nil {
		part, err := createFormFileWithType(writer, "video", "video.mp4", "video/mp4")
		if err != nil {
			t.Fatalf("CreateFormFile video failed: %v", err)
		}
		if _, err := part.Write(video); err != nil {
			t.Fatalf("Write video failed: %v", err)
		}
	}

	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/admin/media", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestUploadMedia_PhotoOnly(t *testing.T) {
	database := setupAuthTestDB(t)
	user, token := createAdminSession(t, database, "admin@example.com")
	handler := NewAdminHandler(database, user.ID)

	// Set MEDIA_DIR to temp directory
	tmpDir := t.TempDir()
	t.Setenv("MEDIA_DIR", tmpDir)

	photo := bytes.Repeat([]byte{0xFF, 0xD8, 0xFF}, 100) // fake JPEG data
	req := createMediaUploadRequest(t, "cat", photo, nil, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.HandleUploadMedia(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp adminMediaResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if resp.ConceptID != "cat" {
		t.Errorf("concept_id = %q, want cat", resp.ConceptID)
	}
	if resp.Set != "set1" {
		t.Errorf("set = %q, want set1", resp.Set)
	}
	if resp.PhotoPath != "data/media/cat/set1/photo.jpg" {
		t.Errorf("photo_path = %q, want data/media/cat/set1/photo.jpg", resp.PhotoPath)
	}
	if resp.AudioPath != "" {
		t.Errorf("audio_path should be empty, got %q", resp.AudioPath)
	}

	// Verify file exists on disk
	photoPath := filepath.Join(tmpDir, "cat", "set1", "photo.jpg")
	if _, err := os.Stat(photoPath); os.IsNotExist(err) {
		t.Error("Photo file was not created on disk")
	}
}

func TestUploadMedia_PhotoAndAudio(t *testing.T) {
	database := setupAuthTestDB(t)
	user, token := createAdminSession(t, database, "admin@example.com")
	handler := NewAdminHandler(database, user.ID)

	tmpDir := t.TempDir()
	t.Setenv("MEDIA_DIR", tmpDir)

	photo := bytes.Repeat([]byte{0xFF, 0xD8, 0xFF}, 100)
	audio := bytes.Repeat([]byte{0x00}, 200)
	req := createMediaUploadRequest(t, "cat", photo, audio, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.HandleUploadMedia(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp adminMediaResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.AudioPath != "data/media/cat/set1/audio.mp3" {
		t.Errorf("audio_path = %q, want data/media/cat/set1/audio.mp3", resp.AudioPath)
	}

	// Verify audio file on disk
	audioPath := filepath.Join(tmpDir, "cat", "set1", "audio.mp3")
	if _, err := os.Stat(audioPath); os.IsNotExist(err) {
		t.Error("Audio file was not created on disk")
	}
}

func TestUploadMedia_IncrementingSetNumbers(t *testing.T) {
	database := setupAuthTestDB(t)
	user, token := createAdminSession(t, database, "admin@example.com")
	handler := NewAdminHandler(database, user.ID)

	tmpDir := t.TempDir()
	t.Setenv("MEDIA_DIR", tmpDir)

	photo := bytes.Repeat([]byte{0xFF}, 50)

	// Upload first set
	req1 := createMediaUploadRequest(t, "cat", photo, nil, nil)
	req1.Header.Set("Authorization", "Bearer "+token)
	w1 := httptest.NewRecorder()
	handler.HandleUploadMedia(w1, req1)

	if w1.Code != http.StatusCreated {
		t.Fatalf("First upload: expected 201, got %d: %s", w1.Code, w1.Body.String())
	}

	var resp1 adminMediaResponse
	json.Unmarshal(w1.Body.Bytes(), &resp1)
	if resp1.Set != "set1" {
		t.Errorf("First upload set = %q, want set1", resp1.Set)
	}

	// Upload second set
	req2 := createMediaUploadRequest(t, "cat", photo, nil, nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	w2 := httptest.NewRecorder()
	handler.HandleUploadMedia(w2, req2)

	if w2.Code != http.StatusCreated {
		t.Fatalf("Second upload: expected 201, got %d: %s", w2.Code, w2.Body.String())
	}

	var resp2 adminMediaResponse
	json.Unmarshal(w2.Body.Bytes(), &resp2)
	if resp2.Set != "set2" {
		t.Errorf("Second upload set = %q, want set2", resp2.Set)
	}
}

func TestUploadMedia_MissingConceptID(t *testing.T) {
	database := setupAuthTestDB(t)
	user, token := createAdminSession(t, database, "admin@example.com")
	handler := NewAdminHandler(database, user.ID)

	photo := bytes.Repeat([]byte{0xFF}, 50)
	req := createMediaUploadRequest(t, "", photo, nil, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.HandleUploadMedia(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUploadMedia_ConceptNotFound(t *testing.T) {
	database := setupAuthTestDB(t)
	user, token := createAdminSession(t, database, "admin@example.com")
	handler := NewAdminHandler(database, user.ID)

	photo := bytes.Repeat([]byte{0xFF}, 50)
	req := createMediaUploadRequest(t, "nonexistent", photo, nil, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.HandleUploadMedia(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d: %s", w.Code, w.Body.String())
	}

	var resp adminMediaResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Error != "concept not found" {
		t.Errorf("Expected error='concept not found', got %q", resp.Error)
	}
}

func TestUploadMedia_MissingPhoto(t *testing.T) {
	database := setupAuthTestDB(t)
	user, token := createAdminSession(t, database, "admin@example.com")
	handler := NewAdminHandler(database, user.ID)

	// Create request with only concept_id, no photo
	req := createMediaUploadRequest(t, "cat", nil, nil, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.HandleUploadMedia(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUploadMedia_NotAuthenticated(t *testing.T) {
	database := setupAuthTestDB(t)
	handler := NewAdminHandler(database, "some-user-id")

	photo := bytes.Repeat([]byte{0xFF}, 50)
	req := createMediaUploadRequest(t, "cat", photo, nil, nil)
	w := httptest.NewRecorder()
	handler.HandleUploadMedia(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401, got %d", w.Code)
	}
}

func TestUploadMedia_Forbidden(t *testing.T) {
	database := setupAuthTestDB(t)
	_, token := createAdminSession(t, database, "normie@example.com")
	handler := NewAdminHandler(database, "other-user-id")

	photo := bytes.Repeat([]byte{0xFF}, 50)
	req := createMediaUploadRequest(t, "cat", photo, nil, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.HandleUploadMedia(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUploadMedia_InvalidConceptFormat(t *testing.T) {
	database := setupAuthTestDB(t)
	user, token := createAdminSession(t, database, "admin@example.com")
	handler := NewAdminHandler(database, user.ID)

	photo := bytes.Repeat([]byte{0xFF}, 50)
	req := createMediaUploadRequest(t, "../etc", photo, nil, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.HandleUploadMedia(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUploadMedia_DatabaseRecordCreated(t *testing.T) {
	database := setupAuthTestDB(t)
	user, token := createAdminSession(t, database, "admin@example.com")
	handler := NewAdminHandler(database, user.ID)

	tmpDir := t.TempDir()
	t.Setenv("MEDIA_DIR", tmpDir)

	photo := bytes.Repeat([]byte{0xFF}, 50)
	req := createMediaUploadRequest(t, "cat", photo, nil, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.HandleUploadMedia(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// Verify media set exists in database
	ms, err := database.GetRandomMediaSet("cat")
	if err != nil {
		t.Fatalf("GetRandomMediaSet failed: %v", err)
	}
	if ms == nil {
		t.Fatal("Expected media set in database, got nil")
	}
	if ms.PhotoPath != "data/media/cat/set1/photo.jpg" {
		t.Errorf("DB photo_path = %q, want data/media/cat/set1/photo.jpg", ms.PhotoPath)
	}
}

func TestCleanupFiles_RemovesFilesAndEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	setDir := filepath.Join(tmpDir, "cat", "set1")
	if err := os.MkdirAll(setDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Create some files
	photoPath := filepath.Join(setDir, "photo.jpg")
	audioPath := filepath.Join(setDir, "audio.mp3")
	for _, p := range []string{photoPath, audioPath} {
		if err := os.WriteFile(p, []byte("test"), 0644); err != nil {
			t.Fatalf("WriteFile %s: %v", p, err)
		}
	}

	cleanupFiles([]string{photoPath, audioPath}, setDir)

	// Files should be removed
	for _, p := range []string{photoPath, audioPath} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("Expected %s to be removed", p)
		}
	}

	// Empty set directory should also be removed
	if _, err := os.Stat(setDir); !os.IsNotExist(err) {
		t.Errorf("Expected empty set directory %s to be removed", setDir)
	}
}

func TestCleanupFiles_LeavesNonEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	setDir := filepath.Join(tmpDir, "cat", "set1")
	if err := os.MkdirAll(setDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Create files — only clean up one, leave the other
	photoPath := filepath.Join(setDir, "photo.jpg")
	otherFile := filepath.Join(setDir, "other.txt")
	for _, p := range []string{photoPath, otherFile} {
		if err := os.WriteFile(p, []byte("test"), 0644); err != nil {
			t.Fatalf("WriteFile %s: %v", p, err)
		}
	}

	cleanupFiles([]string{photoPath}, setDir)

	// Cleaned file should be gone
	if _, err := os.Stat(photoPath); !os.IsNotExist(err) {
		t.Errorf("Expected %s to be removed", photoPath)
	}

	// Directory should remain (still has other.txt)
	if _, err := os.Stat(setDir); os.IsNotExist(err) {
		t.Error("Expected set directory to remain (not empty)")
	}
}

func TestCleanupFiles_NonexistentFilesNoError(t *testing.T) {
	tmpDir := t.TempDir()

	// Cleaning up nonexistent files should not panic or error
	cleanupFiles([]string{
		filepath.Join(tmpDir, "does-not-exist.jpg"),
		filepath.Join(tmpDir, "also-missing.mp3"),
	}, filepath.Join(tmpDir, "no-such-dir"))
}

func TestUploadMedia_EncryptedAtRest(t *testing.T) {
	database := setupAuthTestDB(t)
	user, token := createAdminSession(t, database, "admin@example.com")
	handler := NewAdminHandler(database, user.ID)

	// Set up age identity for encryption
	identity, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}
	handler.Identity = identity

	tmpDir := t.TempDir()
	t.Setenv("MEDIA_DIR", tmpDir)

	photo := bytes.Repeat([]byte{0xFF, 0xD8, 0xFF}, 100)
	audio := bytes.Repeat([]byte{0x00}, 200)
	req := createMediaUploadRequest(t, "cat", photo, audio, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.HandleUploadMedia(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// Encrypted files should exist on disk
	photoAgePath := filepath.Join(tmpDir, "cat", "set1", "photo.jpg.age")
	if _, err := os.Stat(photoAgePath); os.IsNotExist(err) {
		t.Error("Expected photo.jpg.age to exist on disk")
	}
	audioAgePath := filepath.Join(tmpDir, "cat", "set1", "audio.mp3.age")
	if _, err := os.Stat(audioAgePath); os.IsNotExist(err) {
		t.Error("Expected audio.mp3.age to exist on disk")
	}

	// Plaintext files should NOT exist
	photoPlainPath := filepath.Join(tmpDir, "cat", "set1", "photo.jpg")
	if _, err := os.Stat(photoPlainPath); !os.IsNotExist(err) {
		t.Error("Expected plaintext photo.jpg to NOT exist on disk")
	}
	audioPlainPath := filepath.Join(tmpDir, "cat", "set1", "audio.mp3")
	if _, err := os.Stat(audioPlainPath); !os.IsNotExist(err) {
		t.Error("Expected plaintext audio.mp3 to NOT exist on disk")
	}

	// Decrypt and verify content matches original
	photoCipher, err := os.ReadFile(photoAgePath)
	if err != nil {
		t.Fatalf("Read photo.jpg.age: %v", err)
	}
	photoDecrypted, err := crypto.DecryptBytes(photoCipher, identity)
	if err != nil {
		t.Fatalf("DecryptBytes photo: %v", err)
	}
	if !bytes.Equal(photoDecrypted, photo) {
		t.Error("Decrypted photo doesn't match original")
	}

	audioCipher, err := os.ReadFile(audioAgePath)
	if err != nil {
		t.Fatalf("Read audio.mp3.age: %v", err)
	}
	audioDecrypted, err := crypto.DecryptBytes(audioCipher, identity)
	if err != nil {
		t.Fatalf("DecryptBytes audio: %v", err)
	}
	if !bytes.Equal(audioDecrypted, audio) {
		t.Error("Decrypted audio doesn't match original")
	}

	// DB paths should NOT include .age
	var resp adminMediaResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}
	if resp.PhotoPath != "data/media/cat/set1/photo.jpg" {
		t.Errorf("photo_path = %q, want data/media/cat/set1/photo.jpg", resp.PhotoPath)
	}
	if resp.AudioPath != "data/media/cat/set1/audio.mp3" {
		t.Errorf("audio_path = %q, want data/media/cat/set1/audio.mp3", resp.AudioPath)
	}

	// Verify DB record also lacks .age
	ms, err := database.GetRandomMediaSet("cat")
	if err != nil {
		t.Fatalf("GetRandomMediaSet: %v", err)
	}
	if ms == nil {
		t.Fatal("Expected media set in database")
	}
	if ms.PhotoPath != "data/media/cat/set1/photo.jpg" {
		t.Errorf("DB photo_path = %q, want data/media/cat/set1/photo.jpg", ms.PhotoPath)
	}
}

func TestUploadMedia_PlaintextWhenNoIdentity(t *testing.T) {
	database := setupAuthTestDB(t)
	user, token := createAdminSession(t, database, "admin@example.com")
	handler := NewAdminHandler(database, user.ID)
	// handler.Identity is nil — no encryption

	tmpDir := t.TempDir()
	t.Setenv("MEDIA_DIR", tmpDir)

	photo := bytes.Repeat([]byte{0xFF, 0xD8, 0xFF}, 100)
	req := createMediaUploadRequest(t, "cat", photo, nil, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handler.HandleUploadMedia(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d: %s", w.Code, w.Body.String())
	}

	// Plaintext file should exist
	photoPlainPath := filepath.Join(tmpDir, "cat", "set1", "photo.jpg")
	if _, err := os.Stat(photoPlainPath); os.IsNotExist(err) {
		t.Error("Expected plaintext photo.jpg to exist on disk")
	}

	// .age file should NOT exist
	photoAgePath := filepath.Join(tmpDir, "cat", "set1", "photo.jpg.age")
	if _, err := os.Stat(photoAgePath); !os.IsNotExist(err) {
		t.Error("Expected photo.jpg.age to NOT exist when identity is nil")
	}
}

func TestUploadMedia_EncryptedCleanupOnFailure(t *testing.T) {
	// Verify that cleanupFiles works with .age file paths
	tmpDir := t.TempDir()
	setDir := filepath.Join(tmpDir, "cat", "set1")
	if err := os.MkdirAll(setDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Create .age files simulating encrypted uploads
	photoAgePath := filepath.Join(setDir, "photo.jpg.age")
	audioAgePath := filepath.Join(setDir, "audio.mp3.age")
	for _, p := range []string{photoAgePath, audioAgePath} {
		if err := os.WriteFile(p, []byte("encrypted-data"), 0644); err != nil {
			t.Fatalf("WriteFile %s: %v", p, err)
		}
	}

	cleanupFiles([]string{photoAgePath, audioAgePath}, setDir)

	// .age files should be removed
	for _, p := range []string{photoAgePath, audioAgePath} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("Expected %s to be removed", p)
		}
	}

	// Empty set directory should also be removed
	if _, err := os.Stat(setDir); !os.IsNotExist(err) {
		t.Errorf("Expected empty set directory to be removed")
	}
}
