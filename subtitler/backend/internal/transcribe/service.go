package transcribe

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/trevor/subtitler/internal/config"
)

// OutputFormat represents the format of transcription output
type OutputFormat string

const (
	FormatText OutputFormat = "text"
	FormatJSON OutputFormat = "json"
	FormatSRT  OutputFormat = "srt"
	FormatVTT  OutputFormat = "vtt"
)

// TranscribeOptions holds options for transcription
type TranscribeOptions struct {
	Format   OutputFormat
	Language string // ISO 639-1 language code (e.g., "en", "es"), empty for auto-detect
}

// Service handles transcription operations
type Service struct {
	cfg           *config.Config
	whisperServer *WhisperServer
}

// NewService creates a new transcription service
func NewService(cfg *config.Config) *Service {
	return &Service{
		cfg:           cfg,
		whisperServer: NewWhisperServer(cfg),
	}
}

// Start starts the whisper-server
func (s *Service) Start() error {
	return s.whisperServer.Start()
}

// Stop stops the whisper-server
func (s *Service) Stop() error {
	return s.whisperServer.Stop()
}

// TranscribeFile transcribes an audio file and returns the transcript
func (s *Service) TranscribeFile(audioPath string, options TranscribeOptions) (string, error) {
	if !s.whisperServer.IsRunning() {
		return "", fmt.Errorf("whisper-server is not running")
	}

	// Validate file exists
	if _, err := os.Stat(audioPath); os.IsNotExist(err) {
		return "", fmt.Errorf("audio file does not exist: %s", audioPath)
	}

	// Open the audio file
	file, err := os.Open(audioPath)
	if err != nil {
		return "", fmt.Errorf("failed to open audio file: %w", err)
	}
	defer file.Close()

	// Create multipart form data
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add file field
	part, err := writer.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		return "", fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(part, file); err != nil {
		return "", fmt.Errorf("failed to copy file data: %w", err)
	}

	// Add response_format field
	if err := writer.WriteField("response_format", string(options.Format)); err != nil {
		return "", fmt.Errorf("failed to write response_format field: %w", err)
	}

	// Add language field if specified
	if options.Language != "" {
		if err := writer.WriteField("language", options.Language); err != nil {
			return "", fmt.Errorf("failed to write language field: %w", err)
		}
	}

	// Close the multipart writer
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Send request to whisper-server
	endpoint := fmt.Sprintf("%s/inference", s.whisperServer.GetEndpoint())
	req, err := http.NewRequest("POST", endpoint, body)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request to whisper-server: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("whisper-server returned error status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Read response body
	transcript, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	return string(transcript), nil
}
