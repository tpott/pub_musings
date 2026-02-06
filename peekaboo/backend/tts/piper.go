// Package tts provides text-to-speech integration for peekaboo.
package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Provider is the interface for TTS providers.
type Provider interface {
	// Synthesize converts text to speech and returns WAV audio data.
	Synthesize(ctx context.Context, text string) ([]byte, error)
}

// PiperConfig holds configuration for the Piper TTS provider.
type PiperConfig struct {
	ServerURL   string       // Base URL of the Piper server (e.g., "http://localhost:5000")
	LengthScale float64      // Speaking speed (default 1.0, lower = faster)
	Client      *http.Client // Optional HTTP client
}

// piperProvider implements Provider for Piper TTS.
type piperProvider struct {
	serverURL   string
	lengthScale float64
	client      *http.Client
}

// piperRequest is the JSON payload for Piper API.
type piperRequest struct {
	Text        string  `json:"text"`
	LengthScale float64 `json:"length_scale,omitempty"`
}

// NewPiperProvider creates a new Piper TTS provider.
func NewPiperProvider(cfg PiperConfig) (Provider, error) {
	if cfg.ServerURL == "" {
		return nil, fmt.Errorf("piper server URL is required")
	}

	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	lengthScale := cfg.LengthScale
	if lengthScale == 0 {
		lengthScale = 1.0
	}

	return &piperProvider{
		serverURL:   cfg.ServerURL,
		lengthScale: lengthScale,
		client:      client,
	}, nil
}

// Synthesize converts text to speech using the Piper server.
// Returns WAV audio data (16-bit PCM).
func (p *piperProvider) Synthesize(ctx context.Context, text string) ([]byte, error) {
	if text == "" {
		return nil, fmt.Errorf("text is required")
	}

	// Build request payload
	reqBody := piperRequest{
		Text: text,
	}
	if p.lengthScale != 1.0 {
		reqBody.LengthScale = p.lengthScale
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.serverURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Make request
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("piper request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("piper returned status %d: %s", resp.StatusCode, string(body))
	}

	// Read audio data
	audio, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio: %w", err)
	}

	return audio, nil
}

// Config holds general TTS configuration.
type Config struct {
	Provider    string       // Currently only "piper" is supported
	ServerURL   string       // Base URL of the TTS server
	LengthScale float64      // Speaking speed (default 1.0)
	Client      *http.Client // Optional HTTP client
}

// NewProvider creates a TTS provider based on config.
func NewProvider(cfg Config) (Provider, error) {
	switch cfg.Provider {
	case "piper", "":
		return NewPiperProvider(PiperConfig{
			ServerURL:   cfg.ServerURL,
			LengthScale: cfg.LengthScale,
			Client:      cfg.Client,
		})
	default:
		return nil, fmt.Errorf("unknown TTS provider: %s", cfg.Provider)
	}
}
