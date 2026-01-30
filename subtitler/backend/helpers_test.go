package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGenerateID(t *testing.T) {
	t.Run("generates valid hex string", func(t *testing.T) {
		id, err := generateID()
		if err != nil {
			t.Fatalf("generateID() returned error: %v", err)
		}

		// ID should be 32 characters (16 bytes hex encoded)
		if len(id) != 32 {
			t.Errorf("generateID() returned ID of length %d, expected 32", len(id))
		}

		// ID should be valid hex
		for _, c := range id {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
				t.Errorf("generateID() returned invalid hex character: %c", c)
			}
		}
	})

	t.Run("generates unique IDs", func(t *testing.T) {
		ids := make(map[string]bool)
		for i := 0; i < 100; i++ {
			id, err := generateID()
			if err != nil {
				t.Fatalf("generateID() returned error: %v", err)
			}
			if ids[id] {
				t.Errorf("generateID() returned duplicate ID: %s", id)
			}
			ids[id] = true
		}
	})
}

func TestEscapeFFmpegFilterPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"plain path", "/tmp/video.srt", "/tmp/video.srt"},
		{"path with single quote", "/tmp/it's.srt", `/tmp/it\'s.srt`},
		{"path with colon", "/tmp/file:name.srt", `/tmp/file\:name.srt`},
		{"path with backslash", `C:\temp\file.srt`, `C\:\\temp\\file.srt`},
		{"path with brackets", "/tmp/file[1].srt", `/tmp/file\[1\].srt`},
		{"path with semicolon", "/tmp/file;rm.srt", `/tmp/file\;rm.srt`},
		{"path with multiple specials", "/tmp/it's [1]:test.srt", `/tmp/it\'s \[1\]\:test.srt`},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeFFmpegFilterPath(tt.input)
			if got != tt.expected {
				t.Errorf("escapeFFmpegFilterPath(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestGenerateETag(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "\"e3b0c44298fc1c14\"", // SHA256 of empty string, first 8 bytes
		},
		{
			name:     "hello world",
			input:    "hello world",
			expected: "\"b94d27b9934d3e08\"", // SHA256 of "hello world", first 8 bytes
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := generateETag(tc.input)
			if result != tc.expected {
				t.Errorf("generateETag(%q) = %q, expected %q", tc.input, result, tc.expected)
			}
		})
	}

	t.Run("same input produces same ETag", func(t *testing.T) {
		etag1 := generateETag("test data")
		etag2 := generateETag("test data")
		if etag1 != etag2 {
			t.Errorf("Same input should produce same ETag: %q != %q", etag1, etag2)
		}
	})

	t.Run("different input produces different ETag", func(t *testing.T) {
		etag1 := generateETag("data1")
		etag2 := generateETag("data2")
		if etag1 == etag2 {
			t.Errorf("Different input should produce different ETag: %q == %q", etag1, etag2)
		}
	})

	t.Run("ETag format is quoted", func(t *testing.T) {
		etag := generateETag("test")
		if !strings.HasPrefix(etag, "\"") || !strings.HasSuffix(etag, "\"") {
			t.Errorf("ETag should be quoted: %q", etag)
		}
	})
}

func TestHandleConditionalRequest(t *testing.T) {
	etag := generateETag("test content")

	t.Run("returns true when ETag matches", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("If-None-Match", etag)
		rec := httptest.NewRecorder()

		result := handleConditionalRequest(rec, req, etag)

		if !result {
			t.Error("handleConditionalRequest() should return true when ETag matches")
		}
		if rec.Code != http.StatusNotModified {
			t.Errorf("Expected status 304, got %d", rec.Code)
		}
		if rec.Header().Get("ETag") != etag {
			t.Errorf("ETag header = %q, expected %q", rec.Header().Get("ETag"), etag)
		}
	})

	t.Run("returns true when If-None-Match is *", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("If-None-Match", "*")
		rec := httptest.NewRecorder()

		result := handleConditionalRequest(rec, req, etag)

		if !result {
			t.Error("handleConditionalRequest() should return true for '*' match")
		}
		if rec.Code != http.StatusNotModified {
			t.Errorf("Expected status 304, got %d", rec.Code)
		}
	})

	t.Run("returns false when ETag doesn't match", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("If-None-Match", "\"different-etag\"")
		rec := httptest.NewRecorder()

		result := handleConditionalRequest(rec, req, etag)

		if result {
			t.Error("handleConditionalRequest() should return false when ETag doesn't match")
		}
	})

	t.Run("returns false when no If-None-Match header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		result := handleConditionalRequest(rec, req, etag)

		if result {
			t.Error("handleConditionalRequest() should return false without If-None-Match header")
		}
	})
}

func TestSetCacheHeaders(t *testing.T) {
	etag := generateETag("test content")

	t.Run("sets ETag and Cache-Control with maxAge", func(t *testing.T) {
		rec := httptest.NewRecorder()
		setCacheHeaders(rec, etag, 3600)

		if rec.Header().Get("ETag") != etag {
			t.Errorf("ETag header = %q, expected %q", rec.Header().Get("ETag"), etag)
		}
		cacheControl := rec.Header().Get("Cache-Control")
		if !strings.Contains(cacheControl, "private") {
			t.Error("Cache-Control should contain 'private'")
		}
		if !strings.Contains(cacheControl, "max-age=3600") {
			t.Errorf("Cache-Control should contain 'max-age=3600', got: %q", cacheControl)
		}
	})

	t.Run("sets must-revalidate when maxAge is 0", func(t *testing.T) {
		rec := httptest.NewRecorder()
		setCacheHeaders(rec, etag, 0)

		if rec.Header().Get("ETag") != etag {
			t.Errorf("ETag header = %q, expected %q", rec.Header().Get("ETag"), etag)
		}
		cacheControl := rec.Header().Get("Cache-Control")
		if !strings.Contains(cacheControl, "must-revalidate") {
			t.Errorf("Cache-Control should contain 'must-revalidate' when maxAge=0, got: %q", cacheControl)
		}
	})
}

func TestValidatePathID(t *testing.T) {
	t.Run("accepts valid 32-char hex ID", func(t *testing.T) {
		rec := httptest.NewRecorder()
		id := "0123456789abcdef0123456789abcdef"

		result, ok := validatePathID(rec, id, "video_id")

		if !ok {
			t.Error("validatePathID() should return true for valid ID")
		}
		if result != id {
			t.Errorf("validatePathID() = %q, expected %q", result, id)
		}
	})

	t.Run("rejects empty ID", func(t *testing.T) {
		rec := httptest.NewRecorder()

		_, ok := validatePathID(rec, "", "video_id")

		if ok {
			t.Error("validatePathID() should return false for empty ID")
		}
		if rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", rec.Code)
		}
	})

	t.Run("rejects short ID", func(t *testing.T) {
		rec := httptest.NewRecorder()

		_, ok := validatePathID(rec, "0123456789abcdef", "video_id")

		if ok {
			t.Error("validatePathID() should return false for short ID")
		}
		if rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", rec.Code)
		}
	})

	t.Run("rejects invalid hex characters", func(t *testing.T) {
		rec := httptest.NewRecorder()

		_, ok := validatePathID(rec, "zzzz456789abcdef0123456789abcdef", "video_id")

		if ok {
			t.Error("validatePathID() should return false for invalid hex")
		}
		if rec.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got %d", rec.Code)
		}
	})
}
