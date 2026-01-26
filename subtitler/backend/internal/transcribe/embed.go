package transcribe

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	FormatEmbedded OutputFormat = "embedded"
)

// EmbedSubtitles takes a video/audio file and an SRT subtitle file,
// and creates a new video file with the subtitles embedded (burned in).
// Returns the path to the output video file.
func EmbedSubtitles(videoPath, srtPath, outputPath string) error {
	// Verify ffmpeg is installed
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg not found in PATH: %w", err)
	}

	// Verify input files exist
	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		return fmt.Errorf("video file does not exist: %s", videoPath)
	}
	if _, err := os.Stat(srtPath); os.IsNotExist(err) {
		return fmt.Errorf("subtitle file does not exist: %s", srtPath)
	}

	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Build ffmpeg command
	// -i: input file
	// -vf subtitles=: video filter to burn subtitles
	// -c:a copy: copy audio codec (no re-encoding)
	// -c:v libx264: encode video with h264
	// -preset fast: encoding speed/quality tradeoff
	// -y: overwrite output file if exists
	args := []string{
		"-i", videoPath,
		"-vf", fmt.Sprintf("subtitles=%s", srtPath),
		"-c:a", "copy",
		"-c:v", "libx264",
		"-preset", "fast",
		"-y",
		outputPath,
	}

	// Execute ffmpeg
	cmd := exec.Command("ffmpeg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}
