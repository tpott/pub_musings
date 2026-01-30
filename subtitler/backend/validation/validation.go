// Package validation provides input validation utilities for user-provided data.
package validation

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Input length limits
const (
	// MaxSegmentTextLength is the maximum length for subtitle segment text (10KB)
	MaxSegmentTextLength = 10 * 1024

	// MaxEmailLength is the maximum length for email addresses (RFC 5321)
	MaxEmailLength = 254

	// MinPasswordLength is the minimum length for passwords
	MinPasswordLength = 8

	// MaxPasswordLength is the maximum length for passwords (bcrypt limit)
	MaxPasswordLength = 72

	// MaxTOTPCodeLength is the maximum length for TOTP codes
	MaxTOTPCodeLength = 10

	// MaxRecoveryCodeLength is the maximum length for recovery codes
	MaxRecoveryCodeLength = 32

	// MaxAlignTextLength is the maximum length for transcript alignment text (100KB)
	MaxAlignTextLength = 100 * 1024

	// MaxLanguageCodeLength is the maximum length for language codes
	MaxLanguageCodeLength = 10

	// MaxFilenameLength is the maximum length for filenames
	MaxFilenameLength = 255

	// MaxMIMETypeLength is the maximum length for MIME types
	MaxMIMETypeLength = 100
)

// ValidateSegmentText validates the text field of a subtitle segment.
func ValidateSegmentText(text string) error {
	if len(text) > MaxSegmentTextLength {
		return fmt.Errorf("segment text is too long (max %d bytes, got %d)", MaxSegmentTextLength, len(text))
	}
	return nil
}

// ValidateEmail validates email address format and length.
// Note: For full email validation including format, use auth.ValidateEmail()
func ValidateEmail(email string) error {
	email = strings.TrimSpace(email)
	if email == "" {
		return fmt.Errorf("email is required")
	}
	if len(email) > MaxEmailLength {
		return fmt.Errorf("email is too long (max %d characters)", MaxEmailLength)
	}
	return nil
}

// ValidatePassword validates password length (8-72 characters).
// Note: For full password validation including complexity requirements
// (uppercase, lowercase, number, special character), use auth.ValidatePassword()
func ValidatePassword(password string) error {
	if password == "" {
		return fmt.Errorf("password is required")
	}
	if len(password) < MinPasswordLength {
		return fmt.Errorf("password must be at least %d characters", MinPasswordLength)
	}
	if len(password) > MaxPasswordLength {
		return fmt.Errorf("password is too long (max %d characters)", MaxPasswordLength)
	}
	return nil
}

// ValidateTOTPCode validates TOTP code format and length.
func ValidateTOTPCode(code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return fmt.Errorf("TOTP code is required")
	}
	if len(code) > MaxTOTPCodeLength {
		return fmt.Errorf("TOTP code is too long (max %d characters)", MaxTOTPCodeLength)
	}
	return nil
}

// ValidateRecoveryCode validates recovery code format and length.
func ValidateRecoveryCode(code string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return fmt.Errorf("recovery code is required")
	}
	if len(code) > MaxRecoveryCodeLength {
		return fmt.Errorf("recovery code is too long (max %d characters)", MaxRecoveryCodeLength)
	}
	return nil
}

// ValidateAlignText validates transcript text for alignment operations.
func ValidateAlignText(text string) error {
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("text is required")
	}
	if len(text) > MaxAlignTextLength {
		return fmt.Errorf("text is too long (max %d bytes)", MaxAlignTextLength)
	}
	return nil
}

// ValidateLanguageCode validates language code length.
func ValidateLanguageCode(code string) error {
	if len(code) > MaxLanguageCodeLength {
		return fmt.Errorf("language code is too long (max %d characters)", MaxLanguageCodeLength)
	}
	return nil
}

// ValidateFilename validates filename length.
func ValidateFilename(filename string) error {
	if filename == "" {
		return fmt.Errorf("filename is required")
	}
	if len(filename) > MaxFilenameLength {
		return fmt.Errorf("filename is too long (max %d characters)", MaxFilenameLength)
	}
	return nil
}

// ValidateMIMEType validates MIME type length.
func ValidateMIMEType(mimeType string) error {
	if len(mimeType) > MaxMIMETypeLength {
		return fmt.Errorf("MIME type is too long (max %d characters)", MaxMIMETypeLength)
	}
	return nil
}

// BurnMode represents valid burn mode values
type BurnMode string

const (
	BurnModeBurn  BurnMode = "burn"
	BurnModeEmbed BurnMode = "embed"
)

// ValidateBurnMode validates the burn mode parameter.
func ValidateBurnMode(mode string) (BurnMode, error) {
	switch mode {
	case "", "burn":
		return BurnModeBurn, nil
	case "embed":
		return BurnModeEmbed, nil
	default:
		return "", fmt.Errorf("invalid burn mode: must be 'burn' or 'embed'")
	}
}

// AlignMode represents valid alignment mode values
type AlignMode string

const (
	AlignModeDefault AlignMode = ""
	AlignModeLyrics  AlignMode = "lyrics"
)

// ValidateAlignMode validates the alignment mode parameter.
func ValidateAlignMode(mode string) (AlignMode, error) {
	switch mode {
	case "":
		return AlignModeDefault, nil
	case "lyrics":
		return AlignModeLyrics, nil
	default:
		return "", fmt.Errorf("invalid align mode: must be empty or 'lyrics'")
	}
}

// ValidateHexID validates that an ID is a 32-character hexadecimal string.
// This matches the format generated by generateID() (16 bytes = 32 hex chars).
func ValidateHexID(id string) error {
	if len(id) != 32 {
		return fmt.Errorf("invalid ID format: must be 32 characters")
	}
	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return fmt.Errorf("invalid ID format: must be hexadecimal")
		}
	}
	return nil
}

// SanitizeFileExtension validates and sanitizes a file extension extracted from a filename.
// It rejects extensions containing path separators (/, \) or null bytes which could be
// used for path traversal attacks. Returns the sanitized extension or an error.
//
// The extension should include the leading dot (e.g., ".mp4").
// If the extension is empty, it returns ".mp4" as a safe default.
func SanitizeFileExtension(ext string) (string, error) {
	// Default to .mp4 for empty extensions
	if ext == "" {
		return ".mp4", nil
	}

	// Check for null bytes (can truncate paths in some systems)
	if strings.ContainsAny(ext, "\x00") {
		return "", fmt.Errorf("invalid file extension: contains null byte")
	}

	// Check for path separators (path traversal attempt)
	if strings.ContainsAny(ext, "/\\") {
		return "", fmt.Errorf("invalid file extension: contains path separator")
	}

	// Check for parent directory traversal
	if strings.Contains(ext, "..") {
		return "", fmt.Errorf("invalid file extension: contains path traversal")
	}

	// Ensure extension starts with a dot
	if !strings.HasPrefix(ext, ".") {
		return "", fmt.Errorf("invalid file extension: must start with dot")
	}

	// Length check - extensions shouldn't be excessively long
	if len(ext) > 20 {
		return "", fmt.Errorf("invalid file extension: too long (max 20 characters)")
	}

	return ext, nil
}

// ValidateFilePath validates a file path for security issues.
// It checks for path traversal patterns (..) and null bytes that could be used
// for attacks when the path is used in file operations or exec.Command.
//
// This function does NOT verify the path exists or is within any specific directory.
// Use pathvalidator.Validator for constraining paths to specific directories.
func ValidateFilePath(path string) error {
	if path == "" {
		return fmt.Errorf("path is required")
	}

	// Check for null bytes (can truncate paths in some systems)
	if strings.Contains(path, "\x00") {
		return fmt.Errorf("invalid path: contains null byte")
	}

	// Check for .. path segments (path traversal)
	// This catches attempts like "../etc/passwd" or "/foo/../bar"
	parts := strings.Split(path, string(filepath.Separator))
	for _, part := range parts {
		if part == ".." {
			return fmt.Errorf("invalid path: contains path traversal")
		}
	}

	// Also check with forward slash for cross-platform (user might provide Unix paths on Windows)
	if filepath.Separator != '/' {
		parts = strings.Split(path, "/")
		for _, part := range parts {
			if part == ".." {
				return fmt.Errorf("invalid path: contains path traversal")
			}
		}
	}

	return nil
}
