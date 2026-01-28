// Package httputil provides HTTP utility functions.
package httputil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"unicode"

	"github.com/trevor/subtitler/backend/logging"
)

// DefaultMaxJSONBodySize is the default maximum size for JSON request bodies (1MB).
const DefaultMaxJSONBodySize = 1 << 20 // 1MB

// DecodeJSONBody decodes JSON from the request body with a size limit.
// Returns an error if the body exceeds maxSize bytes.
// If maxSize is 0, DefaultMaxJSONBodySize (1MB) is used.
// This function wraps r.Body with http.MaxBytesReader to prevent memory
// exhaustion from oversized requests.
func DecodeJSONBody(r *http.Request, w http.ResponseWriter, v interface{}, maxSize int64) error {
	if maxSize == 0 {
		maxSize = DefaultMaxJSONBodySize
	}

	// Wrap the body with MaxBytesReader to enforce size limit
	r.Body = http.MaxBytesReader(w, r.Body, maxSize)

	return json.NewDecoder(r.Body).Decode(v)
}

// RespondJSON writes a JSON response with the given status code and data.
// Logs an error if JSON encoding fails but does not change the HTTP response
// since headers may have already been sent.
func RespondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logging.Error("failed to encode JSON response",
			"error", err.Error(),
			"status_code", statusCode,
		)
	}
}

// RespondError writes a JSON error response with the given status code and message.
// Sets Content-Type header to application/json before writing.
// Logs an error if JSON encoding fails.
func RespondError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	}); err != nil {
		logging.Error("failed to encode error response",
			"error", err.Error(),
			"status_code", statusCode,
			"message", message,
		)
	}
}

// RespondErrorf writes a JSON error response using a formatted message.
// Convenience wrapper around RespondError for sprintf-style formatting.
func RespondErrorf(w http.ResponseWriter, statusCode int, format string, args ...interface{}) {
	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}
	RespondError(w, statusCode, msg)
}

// ContentDisposition generates a Content-Disposition header value for file downloads.
// It follows RFC 5987 to properly encode non-ASCII filenames, ensuring compatibility
// with modern browsers while providing fallback for older clients.
//
// The header includes both:
// - filename="..." - ASCII-safe fallback (special chars replaced with underscores)
// - filename*=UTF-8”... - RFC 5987 encoded version for Unicode support
//
// Example output for "日本語ファイル.mp4":
//
//	attachment; filename="____________.mp4"; filename*=UTF-8''%E6%97%A5%E6%9C%AC%E8%AA%9E%E3%83%95%E3%82%A1%E3%82%A4%E3%83%AB.mp4
func ContentDisposition(filename string) string {
	// Generate ASCII-safe fallback filename
	asciiFallback := sanitizeToASCII(filename)

	// Check if filename is already pure ASCII and safe
	if isPureASCIISafe(filename) {
		// Simple case: ASCII-only filename, just quote it
		return `attachment; filename="` + escapeQuotes(filename) + `"`
	}

	// Need RFC 5987 encoding for non-ASCII characters
	encodedFilename := encodeRFC5987(filename)

	// Return both forms for maximum compatibility
	return `attachment; filename="` + escapeQuotes(asciiFallback) + `"; filename*=UTF-8''` + encodedFilename
}

// sanitizeToASCII creates an ASCII-safe version of a filename.
// Non-ASCII characters are replaced with underscores.
// Control characters and problematic punctuation are also replaced.
func sanitizeToASCII(filename string) string {
	var result strings.Builder
	result.Grow(len(filename))

	for _, r := range filename {
		if r >= 0x20 && r < 0x7F && r != '"' && r != '\\' && r != '/' && r != ':' && r != '*' && r != '?' && r != '<' && r != '>' && r != '|' {
			result.WriteRune(r)
		} else if r >= 0x80 {
			// Non-ASCII character, replace with underscore
			result.WriteRune('_')
		} else if r == '"' || r == '\\' {
			// Escape quotes and backslashes
			result.WriteRune('_')
		} else {
			// Control characters or filesystem-unsafe chars
			result.WriteRune('_')
		}
	}

	return result.String()
}

// isPureASCIISafe checks if a filename contains only safe ASCII characters.
func isPureASCIISafe(filename string) bool {
	for _, r := range filename {
		// Check for printable ASCII that doesn't need special handling
		if r < 0x20 || r >= 0x7F || r == '"' || r == '\\' || r == '/' || r == ':' || r == '*' || r == '?' || r == '<' || r == '>' || r == '|' {
			return false
		}
	}
	return true
}

// escapeQuotes escapes double quotes and backslashes in a filename for the quoted-string format.
func escapeQuotes(filename string) string {
	var result strings.Builder
	result.Grow(len(filename))

	for _, r := range filename {
		if r == '"' || r == '\\' {
			result.WriteRune('\\')
		}
		result.WriteRune(r)
	}

	return result.String()
}

// encodeRFC5987 encodes a filename according to RFC 5987 (ext-value format).
// Characters are percent-encoded except for attr-char:
// ALPHA / DIGIT / "!" / "#" / "$" / "&" / "+" / "-" / "." / "^" / "_" / "`" / "|" / "~"
func encodeRFC5987(filename string) string {
	// RFC 5987 attr-char safe characters (no need to encode these)
	isSafe := func(r rune) bool {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r < 128 // Only ASCII letters/digits are safe
		}
		switch r {
		case '!', '#', '$', '&', '+', '-', '.', '^', '_', '`', '|', '~':
			return true
		}
		return false
	}

	var result strings.Builder
	result.Grow(len(filename) * 3) // Worst case: every char becomes %XX

	for _, r := range filename {
		if isSafe(r) {
			result.WriteRune(r)
		} else {
			// Percent-encode the UTF-8 bytes
			encoded := url.PathEscape(string(r))
			// url.PathEscape encodes spaces as %20, which is what we want
			result.WriteString(encoded)
		}
	}

	return result.String()
}
