package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	// MaxFileSize is approximately 200MB (enough for ~10 minutes of 1080p video)
	MaxFileSize = 200 * 1024 * 1024 // 200MB in bytes
)

var (
	// AllowedExtensions defines the file types we accept
	AllowedExtensions = map[string]bool{
		// Audio formats
		".mp3":  true,
		".wav":  true,
		".m4a":  true,
		".ogg":  true,
		".flac": true,
		// Video formats
		".mp4":  true,
		".webm": true,
		".mkv":  true,
		".avi":  true,
		".mov":  true,
	}
)

// ValidateFile checks if the file has a valid extension and size
func ValidateFile(filename string, size int64) error {
	// Check file size
	if size > MaxFileSize {
		return fmt.Errorf("file size %d bytes exceeds maximum allowed size of %d bytes (200MB)", size, MaxFileSize)
	}

	if size == 0 {
		return fmt.Errorf("file is empty")
	}

	// Check file extension
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		return fmt.Errorf("file has no extension")
	}

	if !AllowedExtensions[ext] {
		return fmt.Errorf("file type %s is not supported", ext)
	}

	return nil
}

// SaveUploadedFile saves the uploaded file to the specified destination path
func SaveUploadedFile(src io.Reader, destPath string) error {
	// Ensure the directory exists
	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create the destination file
	dst, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	// Copy the data
	_, err = io.Copy(dst, src)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
