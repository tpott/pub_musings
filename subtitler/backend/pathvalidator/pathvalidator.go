// Package pathvalidator provides utilities for validating file paths to prevent
// path traversal attacks. It ensures paths stay within allowed base directories.
package pathvalidator

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Common errors returned by path validation functions
var (
	ErrPathTraversal    = errors.New("path traversal attempt detected")
	ErrOutsideBaseDir   = errors.New("path is outside allowed directory")
	ErrEmptyPath        = errors.New("path is empty")
	ErrAbsolutePath     = errors.New("absolute paths not allowed")
	ErrSymlinkTraversal = errors.New("symlink points outside allowed directory")
)

// Validator validates paths against configured base directories.
type Validator struct {
	// baseDirs are the allowed base directories (absolute paths)
	baseDirs []string
}

// New creates a new path validator with the specified base directories.
// All base directories are resolved to absolute paths.
func New(baseDirs ...string) (*Validator, error) {
	v := &Validator{
		baseDirs: make([]string, 0, len(baseDirs)),
	}

	for _, dir := range baseDirs {
		// Get absolute path
		absDir, err := filepath.Abs(dir)
		if err != nil {
			return nil, err
		}

		// Clean the path to normalize it
		absDir = filepath.Clean(absDir)
		v.baseDirs = append(v.baseDirs, absDir)
	}

	return v, nil
}

// ValidatePath validates that a path, when combined with the base directory,
// stays within one of the allowed base directories.
// It handles both relative and absolute paths.
func (v *Validator) ValidatePath(path string) error {
	if path == "" {
		return ErrEmptyPath
	}

	// Check for obvious path traversal patterns BEFORE cleaning
	// This catches attempts like "uploads/../etc/passwd"
	if containsTraversalPatterns(path) {
		return ErrPathTraversal
	}

	// Clean the path to normalize it
	cleanPath := filepath.Clean(path)

	// If path is already absolute, verify it's within allowed directories
	if filepath.IsAbs(cleanPath) {
		return v.validateAbsolutePath(cleanPath)
	}

	// For relative paths, also check the cleaned version for leading ..
	// This catches edge cases where cleaning produces a traversal
	if strings.HasPrefix(cleanPath, "..") {
		return ErrPathTraversal
	}

	return nil
}

// ValidateAbsolutePath validates that a path is within allowed directories.
// If the path is relative, it is converted to absolute using the current working directory.
func (v *Validator) ValidateAbsolutePath(path string) error {
	if path == "" {
		return ErrEmptyPath
	}

	// Check for traversal patterns before cleaning
	if containsTraversalPatterns(path) {
		return ErrPathTraversal
	}

	// Convert to absolute path if relative
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	cleanPath := filepath.Clean(absPath)
	return v.validateAbsolutePath(cleanPath)
}

// validateAbsolutePath is the internal implementation.
func (v *Validator) validateAbsolutePath(cleanPath string) error {
	for _, baseDir := range v.baseDirs {
		// Check if path is within this base directory
		if isWithinDir(cleanPath, baseDir) {
			return nil
		}
	}
	return ErrOutsideBaseDir
}

// SafeJoin safely joins a base directory with path components, ensuring the
// result stays within the base directory. Returns an error if path traversal
// is detected.
func (v *Validator) SafeJoin(baseDir string, components ...string) (string, error) {
	// Get absolute path of base directory
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", err
	}
	absBase = filepath.Clean(absBase)

	// Check that base directory is one of our allowed directories
	baseAllowed := false
	for _, allowedDir := range v.baseDirs {
		if absBase == allowedDir || isWithinDir(absBase, allowedDir) {
			baseAllowed = true
			break
		}
	}
	if !baseAllowed {
		return "", ErrOutsideBaseDir
	}

	// Join all components
	joined := absBase
	for _, comp := range components {
		// Check each component for traversal patterns
		if containsTraversalPatterns(comp) {
			return "", ErrPathTraversal
		}
		joined = filepath.Join(joined, comp)
	}

	// Clean and verify final path
	joined = filepath.Clean(joined)

	// Verify result is still within base directory
	if !isWithinDir(joined, absBase) {
		return "", ErrPathTraversal
	}

	return joined, nil
}

// ValidateAndResolve validates a path and resolves symlinks to ensure the
// final resolved path is also within the allowed directories.
// This is useful when you need to prevent symlink-based attacks.
func (v *Validator) ValidateAndResolve(path string) (string, error) {
	// First validate the path itself
	if err := v.ValidatePath(path); err != nil {
		return "", err
	}

	cleanPath := filepath.Clean(path)

	// Check if file exists before trying to resolve
	if _, err := os.Lstat(cleanPath); err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, just return the cleaned path
			// The caller will handle the non-existence
			return cleanPath, nil
		}
		return "", err
	}

	// Resolve symlinks and validate the real path
	realPath, err := filepath.EvalSymlinks(cleanPath)
	if err != nil {
		return "", err
	}

	// Validate the resolved path
	if err := v.ValidateAbsolutePath(realPath); err != nil {
		return "", ErrSymlinkTraversal
	}

	return realPath, nil
}

// isWithinDir checks if a path is within a directory.
// Both paths should be cleaned absolute paths.
func isWithinDir(path, dir string) bool {
	// Make sure both paths end consistently (no trailing slash)
	path = filepath.Clean(path)
	dir = filepath.Clean(dir)

	// Check if path starts with directory prefix
	// We add a separator to prevent /foo/bar matching /foo/baz
	if path == dir {
		return true
	}
	return strings.HasPrefix(path, dir+string(filepath.Separator))
}

// containsTraversalPatterns checks for common path traversal patterns.
func containsTraversalPatterns(path string) bool {
	// Check for null bytes (used in some attacks)
	if strings.Contains(path, "\x00") {
		return true
	}

	// Check for .. path segments (but not filenames like "..test" or "test..")
	// We need to check for ".." as a complete path segment
	parts := strings.Split(path, string(filepath.Separator))
	for _, part := range parts {
		if part == ".." {
			return true
		}
	}

	// Also check with forward slash for cross-platform
	if filepath.Separator != '/' {
		parts = strings.Split(path, "/")
		for _, part := range parts {
			if part == ".." {
				return true
			}
		}
	}

	return false
}

// MustNew creates a new validator, panicking on error.
// This is useful for initialization in var blocks.
func MustNew(baseDirs ...string) *Validator {
	v, err := New(baseDirs...)
	if err != nil {
		panic(err)
	}
	return v
}

// IsWithin checks if a path is within a base directory.
// This is a convenience function that creates a temporary validator.
func IsWithin(path, baseDir string) (bool, error) {
	v, err := New(baseDir)
	if err != nil {
		return false, err
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return false, err
	}

	err = v.ValidateAbsolutePath(absPath)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, ErrOutsideBaseDir) {
		return false, nil
	}
	return false, err
}
