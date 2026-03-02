// Package api provides HTTP handlers for the Peekaboo backend.
package api

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"filippo.io/age"
	"github.com/tpott/pub_musings/peekaboo/backend/crypto"
)

// EncryptedFileServer serves files that are stored encrypted with age.
// It decrypts files on-demand when serving.
type EncryptedFileServer struct {
	// BaseDir is the directory containing encrypted .age files
	BaseDir string
	// Identity is the age identity used for decryption
	Identity *age.X25519Identity
}

// NewEncryptedFileServer creates a new EncryptedFileServer.
func NewEncryptedFileServer(baseDir string, identity *age.X25519Identity) *EncryptedFileServer {
	return &EncryptedFileServer{
		BaseDir:  baseDir,
		Identity: identity,
	}
}

// ServeHTTP serves an encrypted file by decrypting it on-demand.
func (s *EncryptedFileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Clean the path to prevent directory traversal
	requestPath := filepath.Clean(r.URL.Path)

	// Defense-in-depth: verify resolved path stays within BaseDir
	resolvedPath := filepath.Join(s.BaseDir, requestPath)
	cleanBase := filepath.Clean(s.BaseDir) + string(filepath.Separator)
	if !strings.HasPrefix(resolvedPath, cleanBase) && resolvedPath != filepath.Clean(s.BaseDir) {
		http.NotFound(w, r)
		return
	}

	// Try encrypted file first (.age extension)
	encryptedPath := resolvedPath + ".age"
	plainPath := resolvedPath

	// Check for encrypted version
	if _, err := os.Stat(encryptedPath); err == nil {
		s.serveEncrypted(w, r, encryptedPath, requestPath)
		return
	}

	// Fall back to plain file (for LICENSE.txt, etc.)
	if _, err := os.Stat(plainPath); err == nil {
		http.ServeFile(w, r, plainPath)
		return
	}

	http.NotFound(w, r)
}

// serveEncrypted decrypts and serves an encrypted file.
func (s *EncryptedFileServer) serveEncrypted(w http.ResponseWriter, r *http.Request, encryptedPath, requestPath string) {
	// Open encrypted file
	f, err := os.Open(encryptedPath)
	if err != nil {
		slog.Error("failed to open encrypted file", "path", encryptedPath, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	// Create decrypted reader
	decrypted, err := crypto.DecryptReader(f, s.Identity)
	if err != nil {
		slog.Error("failed to decrypt file", "path", encryptedPath, "error", err)
		http.Error(w, "decryption failed", http.StatusInternalServerError)
		return
	}

	// Set content type based on file extension
	contentType := getContentType(requestPath)
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}

	// Copy decrypted content to response
	if _, err := io.Copy(w, decrypted); err != nil {
		// Log but don't send HTTP error - response headers may have already been sent
		slog.Error("failed to send decrypted file", "error", err, "path", requestPath)
		return
	}
}

// getContentType returns the MIME type for a file based on its extension.
func getContentType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".ogg":
		return "audio/ogg"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".txt":
		return "text/plain"
	default:
		return "application/octet-stream"
	}
}
