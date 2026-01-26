package main

import (
	"os"
	"strings"
	"testing"
)

func TestFormatSRTTimestamp(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{0.0, "00:00:00,000"},
		{1.5, "00:00:01,500"},
		{61.123, "00:01:01,123"},
		{3661.999, "01:01:01,999"},
		{3723.456, "01:02:03,456"},
	}

	for _, tc := range tests {
		result := formatSRTTimestamp(tc.input)
		if result != tc.expected {
			t.Errorf("formatSRTTimestamp(%f) = %s, expected %s", tc.input, result, tc.expected)
		}
	}
}

func TestGenerateSRT(t *testing.T) {
	result := &WhisperResult{
		Language: "en",
		Duration: 10.0,
		Text:     "Hello world. This is a test.",
		Segments: []WhisperSegment{
			{ID: 0, Start: 0.0, End: 2.5, Text: " Hello world."},
			{ID: 1, Start: 3.0, End: 5.5, Text: " This is a test."},
		},
	}

	srt := generateSRT(result)

	// Check that we have proper SRT format
	if !strings.Contains(srt, "1\n") {
		t.Error("SRT should contain sequence number 1")
	}
	if !strings.Contains(srt, "2\n") {
		t.Error("SRT should contain sequence number 2")
	}
	if !strings.Contains(srt, "00:00:00,000 --> 00:00:02,500") {
		t.Error("SRT should contain first timestamp")
	}
	if !strings.Contains(srt, "00:00:03,000 --> 00:00:05,500") {
		t.Error("SRT should contain second timestamp")
	}
	if !strings.Contains(srt, "Hello world.") {
		t.Error("SRT should contain first subtitle text")
	}
	if !strings.Contains(srt, "This is a test.") {
		t.Error("SRT should contain second subtitle text")
	}

	// Check proper spacing between entries
	lines := strings.Split(srt, "\n")
	if len(lines) < 8 {
		t.Errorf("SRT should have at least 8 lines, got %d", len(lines))
	}
}

func TestGenerateSRTEmpty(t *testing.T) {
	result := &WhisperResult{
		Language: "en",
		Duration: 0.0,
		Text:     "",
		Segments: []WhisperSegment{},
	}

	srt := generateSRT(result)

	if srt != "" {
		t.Errorf("Empty segments should produce empty SRT, got: %s", srt)
	}
}

func TestFormatVTTTimestamp(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{0.0, "00:00:00.000"},
		{1.5, "00:00:01.500"},
		{61.123, "00:01:01.123"},
		{3661.999, "01:01:01.999"},
		{3723.456, "01:02:03.456"},
	}

	for _, tc := range tests {
		result := formatVTTTimestamp(tc.input)
		if result != tc.expected {
			t.Errorf("formatVTTTimestamp(%f) = %s, expected %s", tc.input, result, tc.expected)
		}
	}
}

func TestGenerateVTT(t *testing.T) {
	result := &WhisperResult{
		Language: "en",
		Duration: 10.0,
		Text:     "Hello world. This is a test.",
		Segments: []WhisperSegment{
			{ID: 0, Start: 0.0, End: 2.5, Text: " Hello world."},
			{ID: 1, Start: 3.0, End: 5.5, Text: " This is a test."},
		},
	}

	vtt := generateVTT(result)

	// Check WEBVTT header
	if !strings.HasPrefix(vtt, "WEBVTT\n\n") {
		t.Error("VTT should start with 'WEBVTT' header followed by blank line")
	}

	// Check that we have proper VTT format
	if !strings.Contains(vtt, "1\n") {
		t.Error("VTT should contain cue identifier 1")
	}
	if !strings.Contains(vtt, "2\n") {
		t.Error("VTT should contain cue identifier 2")
	}
	// VTT uses period instead of comma for milliseconds
	if !strings.Contains(vtt, "00:00:00.000 --> 00:00:02.500") {
		t.Error("VTT should contain first timestamp with period separator")
	}
	if !strings.Contains(vtt, "00:00:03.000 --> 00:00:05.500") {
		t.Error("VTT should contain second timestamp with period separator")
	}
	if !strings.Contains(vtt, "Hello world.") {
		t.Error("VTT should contain first subtitle text")
	}
	if !strings.Contains(vtt, "This is a test.") {
		t.Error("VTT should contain second subtitle text")
	}
}

func TestGenerateVTTEmpty(t *testing.T) {
	result := &WhisperResult{
		Language: "en",
		Duration: 0.0,
		Text:     "",
		Segments: []WhisperSegment{},
	}

	vtt := generateVTT(result)

	// Even with no segments, VTT should have header
	if vtt != "WEBVTT\n\n" {
		t.Errorf("Empty segments should produce just WEBVTT header, got: %s", vtt)
	}
}

func TestGetEnvOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		expected     string
	}{
		{
			name:         "uses default when env not set",
			key:          "TEST_CONFIG_NOT_SET",
			defaultValue: "default-value",
			envValue:     "",
			expected:     "default-value",
		},
		{
			name:         "uses env value when set",
			key:          "TEST_CONFIG_SET",
			defaultValue: "default-value",
			envValue:     "env-value",
			expected:     "env-value",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Clear the env var first
			os.Unsetenv(tc.key)

			// Set env if test case specifies a value
			if tc.envValue != "" {
				os.Setenv(tc.key, tc.envValue)
				defer os.Unsetenv(tc.key)
			}

			result := getEnvOrDefault(tc.key, tc.defaultValue)
			if result != tc.expected {
				t.Errorf("getEnvOrDefault(%s, %s) = %s, expected %s",
					tc.key, tc.defaultValue, result, tc.expected)
			}
		})
	}
}

func TestGetEnvSizeOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue int64
		envValue     string
		expected     int64
	}{
		{
			name:         "uses default when env not set",
			key:          "TEST_SIZE_NOT_SET",
			defaultValue: 100,
			envValue:     "",
			expected:     100,
		},
		{
			name:         "parses plain number",
			key:          "TEST_SIZE_PLAIN",
			defaultValue: 100,
			envValue:     "500",
			expected:     500,
		},
		{
			name:         "parses KB suffix",
			key:          "TEST_SIZE_KB",
			defaultValue: 100,
			envValue:     "10K",
			expected:     10 * 1024,
		},
		{
			name:         "parses MB suffix",
			key:          "TEST_SIZE_MB",
			defaultValue: 100,
			envValue:     "500M",
			expected:     500 * 1024 * 1024,
		},
		{
			name:         "parses GB suffix",
			key:          "TEST_SIZE_GB",
			defaultValue: 100,
			envValue:     "1G",
			expected:     1 * 1024 * 1024 * 1024,
		},
		{
			name:         "handles lowercase suffix",
			key:          "TEST_SIZE_LOWER",
			defaultValue: 100,
			envValue:     "500m",
			expected:     500 * 1024 * 1024,
		},
		{
			name:         "handles whitespace",
			key:          "TEST_SIZE_WHITESPACE",
			defaultValue: 100,
			envValue:     " 500M ",
			expected:     500 * 1024 * 1024,
		},
		{
			name:         "uses default for invalid value",
			key:          "TEST_SIZE_INVALID",
			defaultValue: 100,
			envValue:     "not-a-number",
			expected:     100,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Clear the env var first
			os.Unsetenv(tc.key)

			// Set env if test case specifies a value
			if tc.envValue != "" {
				os.Setenv(tc.key, tc.envValue)
				defer os.Unsetenv(tc.key)
			}

			result := getEnvSizeOrDefault(tc.key, tc.defaultValue)
			if result != tc.expected {
				t.Errorf("getEnvSizeOrDefault(%s, %d) = %d, expected %d",
					tc.key, tc.defaultValue, result, tc.expected)
			}
		})
	}
}

func TestInitConfig(t *testing.T) {
	// Save original values
	origMaxUploadSize := maxUploadSize
	origUploadDir := uploadDir
	origDBPath := dbPath
	origKeyPath := keyPath

	// Restore after test
	defer func() {
		maxUploadSize = origMaxUploadSize
		uploadDir = origUploadDir
		dbPath = origDBPath
		keyPath = origKeyPath
	}()

	// Set environment variables
	os.Setenv("MAX_UPLOAD_SIZE", "100M")
	os.Setenv("UPLOAD_DIR", "/custom/uploads")
	os.Setenv("DB_PATH", "/custom/db.sqlite")
	os.Setenv("KEY_PATH", "/custom/key.age")
	defer func() {
		os.Unsetenv("MAX_UPLOAD_SIZE")
		os.Unsetenv("UPLOAD_DIR")
		os.Unsetenv("DB_PATH")
		os.Unsetenv("KEY_PATH")
	}()

	// Call initConfig
	initConfig()

	// Verify values
	if maxUploadSize != 100*1024*1024 {
		t.Errorf("maxUploadSize = %d, expected %d", maxUploadSize, 100*1024*1024)
	}
	if uploadDir != "/custom/uploads" {
		t.Errorf("uploadDir = %s, expected /custom/uploads", uploadDir)
	}
	if dbPath != "/custom/db.sqlite" {
		t.Errorf("dbPath = %s, expected /custom/db.sqlite", dbPath)
	}
	if keyPath != "/custom/key.age" {
		t.Errorf("keyPath = %s, expected /custom/key.age", keyPath)
	}
}

func TestInitConfigDefaults(t *testing.T) {
	// Save original values
	origMaxUploadSize := maxUploadSize
	origUploadDir := uploadDir
	origDBPath := dbPath
	origKeyPath := keyPath

	// Restore after test
	defer func() {
		maxUploadSize = origMaxUploadSize
		uploadDir = origUploadDir
		dbPath = origDBPath
		keyPath = origKeyPath
	}()

	// Clear environment variables
	os.Unsetenv("MAX_UPLOAD_SIZE")
	os.Unsetenv("UPLOAD_DIR")
	os.Unsetenv("DB_PATH")
	os.Unsetenv("KEY_PATH")

	// Call initConfig
	initConfig()

	// Verify defaults are used
	if maxUploadSize != defaultMaxUploadSize {
		t.Errorf("maxUploadSize = %d, expected default %d", maxUploadSize, defaultMaxUploadSize)
	}
	if uploadDir != defaultUploadDir {
		t.Errorf("uploadDir = %s, expected default %s", uploadDir, defaultUploadDir)
	}
	if dbPath != defaultDBPath {
		t.Errorf("dbPath = %s, expected default %s", dbPath, defaultDBPath)
	}
	if keyPath != defaultKeyPath {
		t.Errorf("keyPath = %s, expected default %s", keyPath, defaultKeyPath)
	}
}
