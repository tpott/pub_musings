package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathGenerator_UserUploadDir(t *testing.T) {
	pg := NewPathGenerator("/data")
	path := pg.UserUploadDir(123)
	expected := filepath.Join("/data", "files", "uploads", "123")
	if path != expected {
		t.Errorf("UserUploadDir(123) = %s, want %s", path, expected)
	}
}

func TestPathGenerator_JobUploadDir(t *testing.T) {
	pg := NewPathGenerator("/data")
	path := pg.JobUploadDir(123, 456)
	expected := filepath.Join("/data", "files", "uploads", "123", "456")
	if path != expected {
		t.Errorf("JobUploadDir(123, 456) = %s, want %s", path, expected)
	}
}

func TestPathGenerator_JobUploadPath(t *testing.T) {
	pg := NewPathGenerator("/data")
	path := pg.JobUploadPath(123, 456, "video.mp4")
	expected := filepath.Join("/data", "files", "uploads", "123", "456", "video.mp4.encrypted")
	if path != expected {
		t.Errorf("JobUploadPath(123, 456, 'video.mp4') = %s, want %s", path, expected)
	}
}

func TestPathGenerator_UserResultDir(t *testing.T) {
	pg := NewPathGenerator("/data")
	path := pg.UserResultDir(123)
	expected := filepath.Join("/data", "files", "results", "123")
	if path != expected {
		t.Errorf("UserResultDir(123) = %s, want %s", path, expected)
	}
}

func TestPathGenerator_JobResultDir(t *testing.T) {
	pg := NewPathGenerator("/data")
	path := pg.JobResultDir(123, 456)
	expected := filepath.Join("/data", "files", "results", "123", "456")
	if path != expected {
		t.Errorf("JobResultDir(123, 456) = %s, want %s", path, expected)
	}
}

func TestPathGenerator_JobResultPath(t *testing.T) {
	pg := NewPathGenerator("/data")
	path := pg.JobResultPath(123, 456, "video.mp4", "srt")
	expected := filepath.Join("/data", "files", "results", "123", "456", "video.mp4.srt.encrypted")
	if path != expected {
		t.Errorf("JobResultPath(123, 456, 'video.mp4', 'srt') = %s, want %s", path, expected)
	}
}

func TestPathGenerator_EnsureJobUploadDir(t *testing.T) {
	tmpDir := t.TempDir()
	pg := NewPathGenerator(tmpDir)

	err := pg.EnsureJobUploadDir(123, 456)
	if err != nil {
		t.Fatalf("EnsureJobUploadDir failed: %v", err)
	}

	expectedDir := pg.JobUploadDir(123, 456)
	if _, err := os.Stat(expectedDir); os.IsNotExist(err) {
		t.Errorf("Directory %s was not created", expectedDir)
	}
}

func TestPathGenerator_EnsureJobResultDir(t *testing.T) {
	tmpDir := t.TempDir()
	pg := NewPathGenerator(tmpDir)

	err := pg.EnsureJobResultDir(123, 456)
	if err != nil {
		t.Fatalf("EnsureJobResultDir failed: %v", err)
	}

	expectedDir := pg.JobResultDir(123, 456)
	if _, err := os.Stat(expectedDir); os.IsNotExist(err) {
		t.Errorf("Directory %s was not created", expectedDir)
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"video.mp4", "video.mp4"},
		{"../../../etc/passwd", ".._.._.._etc_passwd"},
		{"file/with/slashes", "file_with_slashes"},
		{"file\\with\\backslashes", "file_with_backslashes"},
		{"file\x00with\x00nulls", "filewithnulls"},
		{"  .file.  ", "file"},
		{"", "file"},
		{"normal-file_name.mp4", "normal-file_name.mp4"},
		{"file..name", "file__name"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeFilename(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestPathGenerator_WithRelativePath(t *testing.T) {
	pg := NewPathGenerator("./data")
	path := pg.JobUploadPath(1, 2, "test.mp4")
	expected := filepath.Join(".", "data", "files", "uploads", "1", "2", "test.mp4.encrypted")
	if path != expected {
		t.Errorf("JobUploadPath with relative path = %s, want %s", path, expected)
	}
}
