package audio

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestCheckFFmpegAvailable(t *testing.T) {
	// This test checks the actual system - will pass if ffmpeg is installed
	err := CheckFFmpegAvailable()
	if err != nil {
		t.Logf("ffmpeg not available (expected in some test environments): %v", err)
		// Don't fail - just log. This test documents the behavior.
	}
}

func TestCheckFFmpegAvailable_NotInPath(t *testing.T) {
	// Save original PATH
	origPath := os.Getenv("PATH")
	defer os.Setenv("PATH", origPath)

	// Set PATH to empty to simulate missing ffmpeg
	os.Setenv("PATH", "/nonexistent-path-for-testing")

	err := CheckFFmpegAvailable()
	if err == nil {
		t.Error("Expected error when ffmpeg is not in PATH")
	}
	if !strings.Contains(err.Error(), "ffmpeg not found") {
		t.Errorf("Expected 'ffmpeg not found' error, got: %v", err)
	}
}

func TestFFmpegExtractor_ExtractAudio_NotFound(t *testing.T) {
	// Save original PATH
	origPath := os.Getenv("PATH")
	defer os.Setenv("PATH", origPath)

	// Set PATH to empty to simulate missing ffmpeg
	os.Setenv("PATH", "/nonexistent-path-for-testing")

	extractor := NewFFmpegExtractor()
	err := extractor.ExtractAudio("/tmp/test.mp4", "/tmp/test.wav")

	if err == nil {
		t.Error("Expected error when ffmpeg is not in PATH")
	}
	// The error should indicate ffmpeg couldn't be found/executed
	if !strings.Contains(err.Error(), "ffmpeg error") {
		t.Errorf("Expected 'ffmpeg error', got: %v", err)
	}
}

func TestFFmpegExtractor_ExtractAudio_InvalidInput(t *testing.T) {
	// Skip if ffmpeg is not available
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available, skipping integration test")
	}

	extractor := NewFFmpegExtractor()
	err := extractor.ExtractAudio("/nonexistent/video.mp4", "/tmp/output.wav")

	if err == nil {
		t.Error("Expected error for nonexistent input file")
	}
	if !strings.Contains(err.Error(), "ffmpeg error") {
		t.Errorf("Expected 'ffmpeg error', got: %v", err)
	}
}

func TestMockExtractor(t *testing.T) {
	mock := NewMockExtractor()

	// Test successful extraction
	err := mock.ExtractAudio("/video.mp4", "/audio.wav")
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}
	if mock.CallCount != 1 {
		t.Errorf("Expected CallCount=1, got %d", mock.CallCount)
	}
	if mock.LastVideoPath != "/video.mp4" {
		t.Errorf("Expected LastVideoPath='/video.mp4', got '%s'", mock.LastVideoPath)
	}
	if mock.LastAudioPath != "/audio.wav" {
		t.Errorf("Expected LastAudioPath='/audio.wav', got '%s'", mock.LastAudioPath)
	}

	// Test failed extraction with default error
	mock.ShouldFail = true
	err = mock.ExtractAudio("/video2.mp4", "/audio2.wav")
	if err == nil {
		t.Error("Expected error when ShouldFail is true")
	}
	if !strings.Contains(err.Error(), "mock extraction failed") {
		t.Errorf("Expected 'mock extraction failed' error, got: %v", err)
	}
	if mock.CallCount != 2 {
		t.Errorf("Expected CallCount=2, got %d", mock.CallCount)
	}

	// Test failed extraction with custom error
	customErr := errors.New("custom ffmpeg error: codec not supported")
	mock.FailError = customErr
	err = mock.ExtractAudio("/video3.mp4", "/audio3.wav")
	if err != customErr {
		t.Errorf("Expected custom error, got: %v", err)
	}
}

func TestExtractorInterface(t *testing.T) {
	// Verify both implementations satisfy the Extractor interface
	var _ Extractor = (*FFmpegExtractor)(nil)
	var _ Extractor = (*MockExtractor)(nil)
}
