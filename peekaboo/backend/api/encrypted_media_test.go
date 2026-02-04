package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/tpott/pub_musings/peekaboo/backend/crypto"
)

func TestEncryptedFileServer_ServesDecryptedFile(t *testing.T) {
	dir := t.TempDir()

	// Create test data
	plaintext := []byte("Hello, this is test image data!")
	plainFile := filepath.Join(dir, "test.jpg")
	encFile := filepath.Join(dir, "test.jpg.age")

	if err := os.WriteFile(plainFile, plaintext, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Generate key and encrypt file
	identity, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	if err := crypto.EncryptFile(plainFile, encFile, identity); err != nil {
		t.Fatalf("EncryptFile failed: %v", err)
	}

	// Remove plain file to ensure we're serving the encrypted one
	os.Remove(plainFile)

	// Create server
	server := NewEncryptedFileServer(dir, identity)

	// Make request
	req := httptest.NewRequest(http.MethodGet, "/test.jpg", nil)
	rr := httptest.NewRecorder()

	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	// Check content type
	contentType := rr.Header().Get("Content-Type")
	if contentType != "image/jpeg" {
		t.Errorf("Expected Content-Type 'image/jpeg', got %q", contentType)
	}

	// Check decrypted content
	body, _ := io.ReadAll(rr.Body)
	if string(body) != string(plaintext) {
		t.Errorf("Decrypted content doesn't match.\nGot: %s\nWant: %s", body, plaintext)
	}
}

func TestEncryptedFileServer_ServesPlainFile(t *testing.T) {
	dir := t.TempDir()

	// Create plain file (no encrypted version)
	plaintext := []byte("This is a plain text file")
	plainFile := filepath.Join(dir, "LICENSE.txt")

	if err := os.WriteFile(plainFile, plaintext, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Generate key (not used for this file)
	identity, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey failed: %v", err)
	}

	// Create server
	server := NewEncryptedFileServer(dir, identity)

	// Make request
	req := httptest.NewRequest(http.MethodGet, "/LICENSE.txt", nil)
	rr := httptest.NewRecorder()

	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Check content
	body, _ := io.ReadAll(rr.Body)
	if string(body) != string(plaintext) {
		t.Errorf("Content doesn't match.\nGot: %s\nWant: %s", body, plaintext)
	}
}

func TestEncryptedFileServer_NotFound(t *testing.T) {
	dir := t.TempDir()

	identity, _ := crypto.GenerateKey()
	server := NewEncryptedFileServer(dir, identity)

	req := httptest.NewRequest(http.MethodGet, "/nonexistent.jpg", nil)
	rr := httptest.NewRecorder()

	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", rr.Code)
	}
}

func TestEncryptedFileServer_WrongMethod(t *testing.T) {
	dir := t.TempDir()

	identity, _ := crypto.GenerateKey()
	server := NewEncryptedFileServer(dir, identity)

	req := httptest.NewRequest(http.MethodPost, "/test.jpg", nil)
	rr := httptest.NewRecorder()

	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", rr.Code)
	}
}

func TestEncryptedFileServer_NestedPath(t *testing.T) {
	dir := t.TempDir()

	// Create nested directory structure
	nestedDir := filepath.Join(dir, "cat", "set1")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	plaintext := []byte("Cat photo data")
	plainFile := filepath.Join(nestedDir, "photo.jpg")
	encFile := filepath.Join(nestedDir, "photo.jpg.age")

	if err := os.WriteFile(plainFile, plaintext, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	identity, _ := crypto.GenerateKey()
	if err := crypto.EncryptFile(plainFile, encFile, identity); err != nil {
		t.Fatalf("EncryptFile failed: %v", err)
	}
	os.Remove(plainFile)

	server := NewEncryptedFileServer(dir, identity)

	req := httptest.NewRequest(http.MethodGet, "/cat/set1/photo.jpg", nil)
	rr := httptest.NewRecorder()

	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body, _ := io.ReadAll(rr.Body)
	if string(body) != string(plaintext) {
		t.Errorf("Decrypted content doesn't match")
	}
}

func TestEncryptedFileServer_ContentTypes(t *testing.T) {
	tests := []struct {
		filename    string
		contentType string
	}{
		{"test.jpg", "image/jpeg"},
		{"test.jpeg", "image/jpeg"},
		{"test.png", "image/png"},
		{"test.gif", "image/gif"},
		{"test.webp", "image/webp"},
		{"test.mp3", "audio/mpeg"},
		{"test.wav", "audio/wav"},
		{"test.ogg", "audio/ogg"},
		{"test.mp4", "video/mp4"},
		{"test.webm", "video/webm"},
		{"test.txt", "text/plain"},
		{"test.bin", "application/octet-stream"},
	}

	for _, tc := range tests {
		t.Run(tc.filename, func(t *testing.T) {
			dir := t.TempDir()

			plaintext := []byte("test data")
			plainFile := filepath.Join(dir, tc.filename)
			encFile := plainFile + ".age"

			if err := os.WriteFile(plainFile, plaintext, 0644); err != nil {
				t.Fatalf("WriteFile failed: %v", err)
			}

			identity, _ := crypto.GenerateKey()
			if err := crypto.EncryptFile(plainFile, encFile, identity); err != nil {
				t.Fatalf("EncryptFile failed: %v", err)
			}
			os.Remove(plainFile)

			server := NewEncryptedFileServer(dir, identity)

			req := httptest.NewRequest(http.MethodGet, "/"+tc.filename, nil)
			rr := httptest.NewRecorder()

			server.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", rr.Code)
			}

			contentType := rr.Header().Get("Content-Type")
			if contentType != tc.contentType {
				t.Errorf("Expected Content-Type %q, got %q", tc.contentType, contentType)
			}
		})
	}
}

func TestEncryptedFileServer_PrefersEncrypted(t *testing.T) {
	dir := t.TempDir()

	// Create both plain and encrypted versions
	plainData := []byte("plain version")
	encryptedData := []byte("encrypted version (decrypted)")

	plainFile := filepath.Join(dir, "test.jpg")
	encFile := filepath.Join(dir, "test.jpg.age")

	// Write plain file
	if err := os.WriteFile(plainFile, plainData, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Create encrypted file with different content
	identity, _ := crypto.GenerateKey()

	// First create a temp file with encrypted content, then encrypt it
	tempFile := filepath.Join(dir, "temp.txt")
	if err := os.WriteFile(tempFile, encryptedData, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := crypto.EncryptFile(tempFile, encFile, identity); err != nil {
		t.Fatalf("EncryptFile failed: %v", err)
	}
	os.Remove(tempFile)

	server := NewEncryptedFileServer(dir, identity)

	req := httptest.NewRequest(http.MethodGet, "/test.jpg", nil)
	rr := httptest.NewRecorder()

	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	// Should return the encrypted version (decrypted)
	body, _ := io.ReadAll(rr.Body)
	if string(body) != string(encryptedData) {
		t.Errorf("Expected encrypted version content.\nGot: %s\nWant: %s", body, encryptedData)
	}
}
