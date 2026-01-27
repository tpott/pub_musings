// Package errmsg provides user-friendly error messages for production.
// Internal error details are logged but not exposed to users unless LOG_VERBOSE is enabled.
package errmsg

import (
	"os"
	"strings"
)

// verbose controls whether detailed error messages are returned to clients.
// Set via LOG_VERBOSE environment variable (default: false).
var verbose bool

func init() {
	v := strings.ToLower(os.Getenv("LOG_VERBOSE"))
	verbose = v == "true" || v == "1"
}

// IsVerbose returns whether verbose error messages are enabled.
func IsVerbose() bool {
	return verbose
}

// SetVerbose allows tests to override the verbose setting.
func SetVerbose(v bool) {
	verbose = v
}

// UserMessage returns a user-friendly error message.
// If verbose mode is enabled, returns the detailed error.
// Otherwise, returns the user-friendly message.
func UserMessage(userMsg string, detailedErr error) string {
	if verbose && detailedErr != nil {
		return detailedErr.Error()
	}
	return userMsg
}

// Common user-friendly error messages for various failure scenarios.
// These messages are designed to be helpful without leaking implementation details.
const (
	// Generic errors
	ErrInternalServer = "An unexpected error occurred. Please try again."
	ErrBadRequest     = "Invalid request. Please check your input and try again."
	ErrNotFound       = "The requested resource was not found."
	ErrUnauthorized   = "Authentication required. Please log in."
	ErrForbidden      = "You don't have permission to access this resource."

	// Authentication errors
	ErrLoginFailed        = "Invalid email or password."
	ErrRegistrationFailed = "Registration failed. Please try again."
	ErrSessionExpired     = "Your session has expired. Please log in again."
	ErrAccountLocked      = "Your account is temporarily locked. Please try again later."

	// Email errors
	ErrEmailFailed      = "Failed to send email. Please try again."
	ErrEmailNotVerified = "Please verify your email address first."
	ErrInvalidToken     = "Invalid or expired verification link."

	// Upload/transcription errors
	ErrUploadFailed     = "Failed to upload file. Please try again."
	ErrFileTooLarge     = "File exceeds maximum allowed size."
	ErrInvalidFileType  = "Invalid file type. Please upload a video file."
	ErrTranscribeFailed = "Transcription failed. Please try again."
	ErrVideoNotFound    = "Video not found."
	ErrVideoProcessing  = "Video is still processing. Please wait."

	// Subtitle errors
	ErrSubtitlesNotReady = "Subtitles are not ready yet. Please wait for transcription to complete."
	ErrBurnFailed        = "Failed to embed subtitles. Please try again."
	ErrAlignFailed       = "Failed to align transcript. Please try again."

	// Database errors
	ErrDatabaseError = "A database error occurred. Please try again."

	// External service errors
	ErrServiceUnavailable = "A required service is temporarily unavailable. Please try again."
)

// ForInternalError returns a user-friendly message for internal server errors.
// The detailed error is only included if verbose mode is enabled.
func ForInternalError(detailedErr error) string {
	return UserMessage(ErrInternalServer, detailedErr)
}

// ForDatabaseError returns a user-friendly message for database errors.
func ForDatabaseError(detailedErr error) string {
	return UserMessage(ErrDatabaseError, detailedErr)
}

// ForVideoNotFound returns a user-friendly message when a video is not found.
func ForVideoNotFound(detailedErr error) string {
	return UserMessage(ErrVideoNotFound, detailedErr)
}

// ForTranscriptionError returns a user-friendly message for transcription failures.
func ForTranscriptionError(detailedErr error) string {
	return UserMessage(ErrTranscribeFailed, detailedErr)
}

// ForUploadError returns a user-friendly message for upload failures.
func ForUploadError(detailedErr error) string {
	return UserMessage(ErrUploadFailed, detailedErr)
}

// ForServiceError returns a user-friendly message for external service failures.
func ForServiceError(detailedErr error) string {
	return UserMessage(ErrServiceUnavailable, detailedErr)
}

// ForBurnError returns a user-friendly message for subtitle burning failures.
func ForBurnError(detailedErr error) string {
	return UserMessage(ErrBurnFailed, detailedErr)
}

// ForAlignError returns a user-friendly message for alignment failures.
func ForAlignError(detailedErr error) string {
	return UserMessage(ErrAlignFailed, detailedErr)
}
