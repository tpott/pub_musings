package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PathGenerator provides utilities for generating file storage paths
type PathGenerator struct {
	dataDir string
}

// NewPathGenerator creates a new PathGenerator with the given data directory
func NewPathGenerator(dataDir string) *PathGenerator {
	return &PathGenerator{
		dataDir: dataDir,
	}
}

// UserUploadDir returns the directory path for a user's uploaded files
func (pg *PathGenerator) UserUploadDir(userID int64) string {
	return filepath.Join(pg.dataDir, "files", "uploads", fmt.Sprintf("%d", userID))
}

// JobUploadDir returns the directory path for a specific job's uploaded files
func (pg *PathGenerator) JobUploadDir(userID, jobID int64) string {
	return filepath.Join(pg.UserUploadDir(userID), fmt.Sprintf("%d", jobID))
}

// JobUploadPath returns the full path for an uploaded file
func (pg *PathGenerator) JobUploadPath(userID, jobID int64, filename string) string {
	sanitized := sanitizeFilename(filename)
	return filepath.Join(pg.JobUploadDir(userID, jobID), sanitized+".encrypted")
}

// UserResultDir returns the directory path for a user's result files
func (pg *PathGenerator) UserResultDir(userID int64) string {
	return filepath.Join(pg.dataDir, "files", "results", fmt.Sprintf("%d", userID))
}

// JobResultDir returns the directory path for a specific job's result files
func (pg *PathGenerator) JobResultDir(userID, jobID int64) string {
	return filepath.Join(pg.UserResultDir(userID), fmt.Sprintf("%d", jobID))
}

// JobResultPath returns the full path for a result file
func (pg *PathGenerator) JobResultPath(userID, jobID int64, filename, format string) string {
	sanitized := sanitizeFilename(filename)
	return filepath.Join(pg.JobResultDir(userID, jobID), fmt.Sprintf("%s.%s.encrypted", sanitized, format))
}

// EnsureJobUploadDir creates the upload directory for a job if it doesn't exist
func (pg *PathGenerator) EnsureJobUploadDir(userID, jobID int64) error {
	dir := pg.JobUploadDir(userID, jobID)
	return os.MkdirAll(dir, 0755)
}

// EnsureJobResultDir creates the result directory for a job if it doesn't exist
func (pg *PathGenerator) EnsureJobResultDir(userID, jobID int64) error {
	dir := pg.JobResultDir(userID, jobID)
	return os.MkdirAll(dir, 0755)
}

// sanitizeFilename removes or replaces characters that are problematic in filenames
func sanitizeFilename(filename string) string {
	// Remove path separators FIRST
	// This turns "../../../etc/passwd" into ".._.._.._etc_passwd"
	filename = strings.ReplaceAll(filename, "/", "_")
	filename = strings.ReplaceAll(filename, "\\", "_")

	// Remove null bytes
	filename = strings.ReplaceAll(filename, "\x00", "")

	// Replace ".." with "__", but NOT if it's adjacent to "_"
	// This handles "file..name" -> "file__name"
	// But preserves ".._" and "_.." patterns from path traversal sanitization
	var result strings.Builder
	i := 0
	for i < len(filename) {
		// Check if we have ".." at this position
		if i+1 < len(filename) && filename[i] == '.' && filename[i+1] == '.' {
			// Check what follows
			if i+2 < len(filename) && filename[i+2] == '_' {
				// Pattern: ".._" - keep it
				result.WriteString(".._")
				i += 3
			} else if i > 0 && filename[i-1] == '_' {
				// Pattern: "_.." - keep it
				result.WriteString("..")
				i += 2
			} else {
				// Pattern: "..X" where X is not "_" and not preceded by "_" - replace
				result.WriteString("__")
				i += 2
			}
		} else {
			result.WriteByte(filename[i])
			i++
		}
	}
	filename = result.String()

	// Trim spaces from edges
	filename = strings.Trim(filename, " ")

	// Trim isolated dots (single dots, not part of ".." sequences)
	// From the start
	for len(filename) > 0 && filename[0] == '.' {
		if len(filename) > 1 && filename[1] == '.' {
			// This is part of a ".." sequence, don't trim
			break
		}
		filename = filename[1:]
	}

	// From the end
	for len(filename) > 0 && filename[len(filename)-1] == '.' {
		if len(filename) > 1 && filename[len(filename)-2] == '.' {
			// This is part of a ".." sequence, don't trim
			break
		}
		filename = filename[:len(filename)-1]
	}

	// If filename is empty after sanitization, use a default
	if filename == "" {
		filename = "file"
	}

	return filename
}
