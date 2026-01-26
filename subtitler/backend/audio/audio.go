// Package audio provides audio extraction capabilities for video processing.
package audio

import (
	"fmt"
	"os/exec"
)

// Extractor defines the interface for extracting audio from video files.
type Extractor interface {
	// ExtractAudio extracts audio from a video file and saves it as WAV.
	// videoPath: path to the input video file
	// audioPath: path where the output WAV file will be saved
	ExtractAudio(videoPath, audioPath string) error
}

// FFmpegExtractor implements Extractor using ffmpeg.
type FFmpegExtractor struct{}

// NewFFmpegExtractor creates a new FFmpegExtractor.
func NewFFmpegExtractor() *FFmpegExtractor {
	return &FFmpegExtractor{}
}

// ExtractAudio uses ffmpeg to extract audio from video as WAV.
// The output is 16kHz mono PCM, which is the format expected by whisper.
func (f *FFmpegExtractor) ExtractAudio(videoPath, audioPath string) error {
	cmd := exec.Command("ffmpeg",
		"-i", videoPath,
		"-vn",                  // no video
		"-acodec", "pcm_s16le", // WAV format
		"-ar", "16000", // 16kHz sample rate (whisper expects this)
		"-ac", "1", // mono
		"-y", // overwrite output
		audioPath,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg error: %v, output: %s", err, string(output))
	}
	return nil
}

// CheckFFmpegAvailable checks if ffmpeg is available in the system PATH.
// Returns nil if ffmpeg is found, or an error describing the problem.
func CheckFFmpegAvailable() error {
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg not found in PATH: %w. Install ffmpeg to enable video processing", err)
	}
	// Verify it's executable by getting version
	cmd := exec.Command(path, "-version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("ffmpeg found at %s but failed to execute: %w", path, err)
	}
	// Log version info (first line contains version)
	if len(output) > 0 {
		// Version check succeeded
		return nil
	}
	return nil
}

// ValidateVideoFile uses ffprobe to verify that a file is a valid video.
// This provides defense-in-depth beyond MIME type checking, as MIME types
// can be spoofed by malicious clients.
// Returns nil if the file is a valid video, or an error describing the problem.
func ValidateVideoFile(filePath string) error {
	// First check if ffprobe is available
	probePath, err := exec.LookPath("ffprobe")
	if err != nil {
		// ffprobe not available - fall back to allowing the file
		// (MIME check already passed at this point)
		return nil
	}

	// Use ffprobe to check if file contains a video stream
	// -v error: only show errors
	// -select_streams v:0: select first video stream
	// -show_entries stream=codec_type: only show codec type
	// -of csv=p=0: output as plain CSV without headers
	cmd := exec.Command(probePath,
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=codec_type",
		"-of", "csv=p=0",
		filePath,
	)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("invalid video file: ffprobe failed to read media streams")
	}

	// Check if output contains "video"
	outputStr := string(output)
	if outputStr == "" || outputStr == "\n" {
		return fmt.Errorf("invalid video file: no video stream found")
	}

	return nil
}

// MockExtractor is a test implementation of Extractor.
type MockExtractor struct {
	// ShouldFail controls whether ExtractAudio returns an error.
	ShouldFail bool
	// FailError is the error to return when ShouldFail is true.
	FailError error
	// CallCount tracks how many times ExtractAudio was called.
	CallCount int
	// LastVideoPath stores the last videoPath argument.
	LastVideoPath string
	// LastAudioPath stores the last audioPath argument.
	LastAudioPath string
}

// NewMockExtractor creates a new MockExtractor for testing.
func NewMockExtractor() *MockExtractor {
	return &MockExtractor{}
}

// ExtractAudio implements the Extractor interface for testing.
func (m *MockExtractor) ExtractAudio(videoPath, audioPath string) error {
	m.CallCount++
	m.LastVideoPath = videoPath
	m.LastAudioPath = audioPath
	if m.ShouldFail {
		if m.FailError != nil {
			return m.FailError
		}
		return fmt.Errorf("mock extraction failed")
	}
	return nil
}
