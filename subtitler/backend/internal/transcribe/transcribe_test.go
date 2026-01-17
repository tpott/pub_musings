package transcribe

import (
	"os"
	"strings"
	"testing"

	"github.com/trevor/subtitler/internal/config"
)

// TestTranscribeJFK tests transcription with the JFK sample audio
// This is an integration test that requires whisper.cpp to be installed
func TestTranscribeJFK(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Load config
	cfg := config.Load()

	// Verify prerequisites exist
	if _, err := os.Stat(cfg.WhisperServerPath); os.IsNotExist(err) {
		t.Skipf("whisper-server not found at %s, skipping integration test", cfg.WhisperServerPath)
	}
	if _, err := os.Stat(cfg.WhisperModelPath); os.IsNotExist(err) {
		t.Skipf("whisper model not found at %s, skipping integration test", cfg.WhisperModelPath)
	}

	// Path to JFK sample audio
	samplePath := os.ExpandEnv("$HOME/Github/whisper.cpp/samples/jfk.wav")
	if _, err := os.Stat(samplePath); os.IsNotExist(err) {
		t.Skipf("JFK sample audio not found at %s, skipping integration test", samplePath)
	}

	// Create service and start whisper-server
	service := NewService(cfg)
	if err := service.Start(); err != nil {
		t.Fatalf("Failed to start whisper-server: %v", err)
	}
	defer service.Stop()

	// Test transcription with text format
	t.Run("TextFormat", func(t *testing.T) {
		transcript, err := service.TranscribeFile(samplePath, TranscribeOptions{
			Format: FormatText,
		})
		if err != nil {
			t.Fatalf("Failed to transcribe: %v", err)
		}

		// Verify transcript contains expected text
		transcript = strings.ToLower(transcript)
		if !strings.Contains(transcript, "ask not") || !strings.Contains(transcript, "country") {
			t.Errorf("Expected transcript to contain JFK quote, got: %s", transcript)
		}

		t.Logf("Text transcript: %s", transcript)
	})

	// Test transcription with SRT format
	t.Run("SRTFormat", func(t *testing.T) {
		transcript, err := service.TranscribeFile(samplePath, TranscribeOptions{
			Format: FormatSRT,
		})
		if err != nil {
			t.Fatalf("Failed to transcribe: %v", err)
		}

		// Verify SRT format (should have timing markers)
		if !strings.Contains(transcript, "-->") {
			t.Errorf("Expected SRT format with timing markers, got: %s", transcript)
		}

		t.Logf("SRT transcript:\n%s", transcript)
	})

	// Test transcription with VTT format
	t.Run("VTTFormat", func(t *testing.T) {
		transcript, err := service.TranscribeFile(samplePath, TranscribeOptions{
			Format: FormatVTT,
		})
		if err != nil {
			t.Fatalf("Failed to transcribe: %v", err)
		}

		// Verify VTT format (should start with WEBVTT)
		if !strings.HasPrefix(transcript, "WEBVTT") {
			t.Errorf("Expected VTT format starting with WEBVTT, got: %s", transcript[:min(100, len(transcript))])
		}

		t.Logf("VTT transcript:\n%s", transcript)
	})
}

// TestTranscribeNonexistentFile tests error handling for missing files
func TestTranscribeNonexistentFile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := config.Load()

	// Verify prerequisites exist
	if _, err := os.Stat(cfg.WhisperServerPath); os.IsNotExist(err) {
		t.Skipf("whisper-server not found, skipping integration test")
	}
	if _, err := os.Stat(cfg.WhisperModelPath); os.IsNotExist(err) {
		t.Skipf("whisper model not found, skipping integration test")
	}

	service := NewService(cfg)
	if err := service.Start(); err != nil {
		t.Fatalf("Failed to start whisper-server: %v", err)
	}
	defer service.Stop()

	// Try to transcribe nonexistent file
	_, err := service.TranscribeFile("/nonexistent/file.wav", TranscribeOptions{
		Format: FormatText,
	})
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
}

// TestWhisperServerLifecycle tests starting and stopping the whisper-server
func TestWhisperServerLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	cfg := config.Load()

	// Verify prerequisites exist
	if _, err := os.Stat(cfg.WhisperServerPath); os.IsNotExist(err) {
		t.Skipf("whisper-server not found, skipping integration test")
	}
	if _, err := os.Stat(cfg.WhisperModelPath); os.IsNotExist(err) {
		t.Skipf("whisper model not found, skipping integration test")
	}

	service := NewService(cfg)

	// Start server
	if err := service.Start(); err != nil {
		t.Fatalf("Failed to start whisper-server: %v", err)
	}

	// Verify it's running
	if !service.whisperServer.IsRunning() {
		t.Error("Expected whisper-server to be running")
	}

	// Stop server
	if err := service.Stop(); err != nil {
		t.Fatalf("Failed to stop whisper-server: %v", err)
	}

	// Verify it's stopped
	if service.whisperServer.IsRunning() {
		t.Error("Expected whisper-server to be stopped")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
