package httputil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestContentDisposition(t *testing.T) {
	tests := []struct {
		name            string
		filename        string
		wantContains    []string
		wantNotContains []string
	}{
		{
			name:         "simple ASCII filename",
			filename:     "video.mp4",
			wantContains: []string{`attachment; filename="video.mp4"`},
			// Should NOT have filename* for pure ASCII
			wantNotContains: []string{"filename*="},
		},
		{
			name:         "ASCII with spaces",
			filename:     "my video file.mp4",
			wantContains: []string{`attachment; filename="my video file.mp4"`},
			// Should NOT have filename* for pure ASCII with spaces
			wantNotContains: []string{"filename*="},
		},
		{
			name:     "Japanese filename",
			filename: "日本語.mp4",
			wantContains: []string{
				`attachment; filename="`,
				`filename*=UTF-8''`,
				".mp4", // Extension should appear
			},
		},
		{
			name:     "Chinese filename",
			filename: "中文视频.mp4",
			wantContains: []string{
				`attachment; filename="`,
				`filename*=UTF-8''`,
			},
		},
		{
			name:     "Korean filename",
			filename: "한국어.mp4",
			wantContains: []string{
				`attachment; filename="`,
				`filename*=UTF-8''`,
			},
		},
		{
			name:     "Hindi/Devanagari filename",
			filename: "हिंदी.mp4",
			wantContains: []string{
				`attachment; filename="`,
				`filename*=UTF-8''`,
			},
		},
		{
			name:     "filename with quotes",
			filename: `video "final".mp4`,
			wantContains: []string{
				`attachment; filename="`,
				`filename*=UTF-8''`,
			},
			// Quotes should be escaped or replaced in ASCII fallback
			wantNotContains: []string{`filename="video "final".mp4"`},
		},
		{
			name:     "filename with backslash",
			filename: `video\test.mp4`,
			wantContains: []string{
				`attachment; filename="`,
				`filename*=UTF-8''`,
			},
		},
		{
			name:     "mixed ASCII and Unicode",
			filename: "video_日本語_test.mp4",
			wantContains: []string{
				`attachment; filename="`,
				`filename*=UTF-8''`,
				"video_", // ASCII parts should be preserved
				"_test.mp4",
			},
		},
		{
			name:         "filename with safe punctuation",
			filename:     "video-file_v1.0.mp4",
			wantContains: []string{`attachment; filename="video-file_v1.0.mp4"`},
			// Safe punctuation should NOT trigger filename*
			wantNotContains: []string{"filename*="},
		},
		{
			name:     "filename with colon",
			filename: "video:test.mp4",
			wantContains: []string{
				`attachment; filename="`,
				`filename*=UTF-8''`,
			},
		},
		{
			name:     "emoji filename",
			filename: "🎬video🎥.mp4",
			wantContains: []string{
				`attachment; filename="`,
				`filename*=UTF-8''`,
			},
		},
		{
			name:         "already percent encoded looking name",
			filename:     "video%20file.mp4",
			wantContains: []string{`attachment; filename="video%20file.mp4"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContentDisposition(tt.filename)

			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("ContentDisposition(%q) = %q, want to contain %q", tt.filename, result, want)
				}
			}

			for _, notWant := range tt.wantNotContains {
				if strings.Contains(result, notWant) {
					t.Errorf("ContentDisposition(%q) = %q, should not contain %q", tt.filename, result, notWant)
				}
			}
		})
	}
}

func TestSanitizeToASCII(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{
			name:     "simple ASCII",
			filename: "video.mp4",
			want:     "video.mp4",
		},
		{
			name:     "Japanese becomes underscores",
			filename: "日本語.mp4",
			want:     "___.mp4",
		},
		{
			name:     "mixed preserves ASCII",
			filename: "video_日本語_test.mp4",
			want:     "video_____test.mp4", // 3 Japanese chars = 3 underscores, plus surrounding underscores
		},
		{
			name:     "quotes replaced",
			filename: `video "test".mp4`,
			want:     `video _test_.mp4`,
		},
		{
			name:     "backslash replaced",
			filename: `video\test.mp4`,
			want:     `video_test.mp4`,
		},
		{
			name:     "filesystem chars replaced",
			filename: "video:*?<>|.mp4",
			want:     "video______.mp4",
		},
		{
			name:     "control chars replaced",
			filename: "video\x00\x1f.mp4",
			want:     "video__.mp4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeToASCII(tt.filename)
			if got != tt.want {
				t.Errorf("sanitizeToASCII(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}

func TestIsPureASCIISafe(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{"simple ASCII", "video.mp4", true},
		{"ASCII with spaces", "my video.mp4", true},
		{"ASCII with safe punctuation", "video-file_v1.0.mp4", true},
		{"has quote", `video"test.mp4`, false},
		{"has backslash", `video\test.mp4`, false},
		{"has colon", "video:test.mp4", false},
		{"has asterisk", "video*test.mp4", false},
		{"has question mark", "video?test.mp4", false},
		{"has angle brackets", "video<test>.mp4", false},
		{"has pipe", "video|test.mp4", false},
		{"has unicode", "日本語.mp4", false},
		{"has emoji", "🎬.mp4", false},
		{"has control char", "video\x00.mp4", false},
		{"has DEL char", "video\x7F.mp4", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isPureASCIISafe(tt.filename)
			if got != tt.want {
				t.Errorf("isPureASCIISafe(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestEncodeRFC5987(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{
			name:     "simple ASCII",
			filename: "video.mp4",
			want:     "video.mp4",
		},
		{
			name:     "space becomes percent encoded",
			filename: "my video.mp4",
			want:     "my%20video.mp4",
		},
		{
			name:     "safe punctuation preserved",
			filename: "video-file_v1.0.mp4",
			want:     "video-file_v1.0.mp4",
		},
		{
			name:     "Japanese encoded",
			filename: "日本語.mp4",
			// 日 = E6 97 A5, 本 = E6 9C AC, 語 = E8 AA 9E
			want: "%E6%97%A5%E6%9C%AC%E8%AA%9E.mp4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := encodeRFC5987(tt.filename)
			if got != tt.want {
				t.Errorf("encodeRFC5987(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}

func TestEscapeQuotes(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{"no special chars", "video.mp4", "video.mp4"},
		{"has quote", `video"test.mp4`, `video\"test.mp4`},
		{"has backslash", `video\test.mp4`, `video\\test.mp4`},
		{"has both", `video"te\st".mp4`, `video\"te\\st\".mp4`},
		{"multiple quotes", `a"b"c`, `a\"b\"c`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeQuotes(tt.filename)
			if got != tt.want {
				t.Errorf("escapeQuotes(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}

// TestContentDispositionEdgeCases tests edge cases and security considerations
func TestContentDispositionEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		check    func(t *testing.T, result string)
	}{
		{
			name:     "empty filename",
			filename: "",
			check: func(t *testing.T, result string) {
				if !strings.Contains(result, "attachment") {
					t.Error("should still contain attachment")
				}
			},
		},
		{
			name:     "only extension",
			filename: ".mp4",
			check: func(t *testing.T, result string) {
				if !strings.Contains(result, ".mp4") {
					t.Error("should contain extension")
				}
			},
		},
		{
			name:     "very long filename",
			filename: strings.Repeat("a", 255) + ".mp4",
			check: func(t *testing.T, result string) {
				if !strings.Contains(result, "attachment") {
					t.Error("should handle long filenames")
				}
			},
		},
		{
			name:     "path injection attempt",
			filename: "../../../etc/passwd",
			check: func(t *testing.T, result string) {
				// Slashes should be sanitized
				if strings.Contains(result, "/") && strings.Contains(result, `filename="/`) {
					t.Error("should not allow path separators in ASCII fallback")
				}
			},
		},
		{
			name:     "null byte injection",
			filename: "video\x00.mp4",
			check: func(t *testing.T, result string) {
				// Null bytes should be handled
				if strings.Contains(result, "\x00") {
					t.Error("should not allow null bytes")
				}
			},
		},
		{
			name:     "newline injection",
			filename: "video\r\n.mp4",
			check: func(t *testing.T, result string) {
				// Newlines should be encoded or removed
				if strings.Contains(result, "\r") || strings.Contains(result, "\n") {
					t.Error("should not allow raw newlines")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContentDisposition(tt.filename)
			tt.check(t, result)
		})
	}
}

// ========== RespondError Tests ==========

func TestRespondError(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		message     string
		wantCode    int
		wantMessage string
	}{
		{
			name:        "bad request",
			statusCode:  http.StatusBadRequest,
			message:     "Invalid input",
			wantCode:    http.StatusBadRequest,
			wantMessage: "Invalid input",
		},
		{
			name:        "not found",
			statusCode:  http.StatusNotFound,
			message:     "Resource not found",
			wantCode:    http.StatusNotFound,
			wantMessage: "Resource not found",
		},
		{
			name:        "internal server error",
			statusCode:  http.StatusInternalServerError,
			message:     "Something went wrong",
			wantCode:    http.StatusInternalServerError,
			wantMessage: "Something went wrong",
		},
		{
			name:        "forbidden",
			statusCode:  http.StatusForbidden,
			message:     "Access denied",
			wantCode:    http.StatusForbidden,
			wantMessage: "Access denied",
		},
		{
			name:        "unauthorized",
			statusCode:  http.StatusUnauthorized,
			message:     "Authentication required",
			wantCode:    http.StatusUnauthorized,
			wantMessage: "Authentication required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			RespondError(w, tt.statusCode, tt.message)

			// Check status code
			if w.Code != tt.wantCode {
				t.Errorf("RespondError() status = %d, want %d", w.Code, tt.wantCode)
			}

			// Check Content-Type header
			contentType := w.Header().Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("RespondError() Content-Type = %q, want %q", contentType, "application/json")
			}

			// Parse and check response body
			var resp map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("Failed to parse response JSON: %v", err)
			}

			if resp["error"] != tt.wantMessage {
				t.Errorf("RespondError() error = %q, want %q", resp["error"], tt.wantMessage)
			}
		})
	}
}

func TestRespondErrorf(t *testing.T) {
	tests := []struct {
		name        string
		statusCode  int
		format      string
		args        []interface{}
		wantMessage string
	}{
		{
			name:        "with format args",
			statusCode:  http.StatusBadRequest,
			format:      "Not all chunks received. Expected %d, got %d",
			args:        []interface{}{10, 5},
			wantMessage: "Not all chunks received. Expected 10, got 5",
		},
		{
			name:        "with string arg",
			statusCode:  http.StatusNotFound,
			format:      "Video %s not found",
			args:        []interface{}{"abc123"},
			wantMessage: "Video abc123 not found",
		},
		{
			name:        "no args",
			statusCode:  http.StatusInternalServerError,
			format:      "Plain error message",
			args:        nil,
			wantMessage: "Plain error message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			RespondErrorf(w, tt.statusCode, tt.format, tt.args...)

			// Check status code
			if w.Code != tt.statusCode {
				t.Errorf("RespondErrorf() status = %d, want %d", w.Code, tt.statusCode)
			}

			// Parse and check response body
			var resp map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("Failed to parse response JSON: %v", err)
			}

			if resp["error"] != tt.wantMessage {
				t.Errorf("RespondErrorf() error = %q, want %q", resp["error"], tt.wantMessage)
			}
		})
	}
}
