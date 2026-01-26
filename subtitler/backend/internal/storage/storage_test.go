package storage

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		size     int64
		wantErr  bool
	}{
		{
			name:     "valid mp3 file",
			filename: "audio.mp3",
			size:     1024 * 1024, // 1MB
			wantErr:  false,
		},
		{
			name:     "valid mp4 file",
			filename: "video.mp4",
			size:     50 * 1024 * 1024, // 50MB
			wantErr:  false,
		},
		{
			name:     "invalid extension",
			filename: "document.pdf",
			size:     1024 * 1024,
			wantErr:  true,
		},
		{
			name:     "no extension",
			filename: "noextension",
			size:     1024 * 1024,
			wantErr:  true,
		},
		{
			name:     "file too large",
			filename: "huge.mp4",
			size:     MaxFileSize + 1,
			wantErr:  true,
		},
		{
			name:     "empty file",
			filename: "empty.mp3",
			size:     0,
			wantErr:  true,
		},
		{
			name:     "valid wav file",
			filename: "audio.wav",
			size:     1024 * 1024,
			wantErr:  false,
		},
		{
			name:     "valid m4a file",
			filename: "audio.m4a",
			size:     1024 * 1024,
			wantErr:  false,
		},
		{
			name:     "valid webm file",
			filename: "video.webm",
			size:     1024 * 1024,
			wantErr:  false,
		},
		{
			name:     "case insensitive extension",
			filename: "audio.MP3",
			size:     1024 * 1024,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFile(tt.filename, tt.size)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSaveUploadedFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	tests := []struct {
		name     string
		content  []byte
		destPath string
		wantErr  bool
	}{
		{
			name:     "save file successfully",
			content:  []byte("test content"),
			destPath: filepath.Join(tempDir, "test1.txt"),
			wantErr:  false,
		},
		{
			name:     "save file in nested directory",
			content:  []byte("nested content"),
			destPath: filepath.Join(tempDir, "subdir", "test2.txt"),
			wantErr:  false,
		},
		{
			name:     "save empty file",
			content:  []byte{},
			destPath: filepath.Join(tempDir, "empty.txt"),
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := bytes.NewReader(tt.content)
			err := SaveUploadedFile(src, tt.destPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("SaveUploadedFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify the file was created and contains the correct content
				savedContent, err := os.ReadFile(tt.destPath)
				if err != nil {
					t.Errorf("Failed to read saved file: %v", err)
					return
				}
				if !bytes.Equal(savedContent, tt.content) {
					t.Errorf("Saved content = %v, want %v", savedContent, tt.content)
				}
			}
		})
	}
}
