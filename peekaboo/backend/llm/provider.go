// Package llm provides a provider abstraction for LLM intent recognition.
package llm

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"
)

// IntentResult contains the result of intent extraction.
type IntentResult struct {
	Subject string
}

// WordData contains per-word data from whisper's verbose_json response.
type WordData struct {
	Word        string  `json:"word"`
	Start       float64 `json:"start"`
	End         float64 `json:"end"`
	Probability float64 `json:"probability"`
}

// TranscriptRequest is the input for the rich transcript processing method.
type TranscriptRequest struct {
	Text     string     // Full transcript text
	Words    []WordData // Per-word timing and probability data
	Concepts []string   // Available concepts from the database
}

// ToolAction represents one action the LLM wants to take.
type ToolAction struct {
	Type                  string // "show_media", "text_to_speech", or "wait_for_more"
	Subject               string // For show_media: the concept to show
	InstructionEndWordIdx int    // For show_media: 0-based index of last word in this command
	Text                  string // For text_to_speech: the text to speak
	Reason                string // For wait_for_more: why the transcript seems incomplete
}

// TranscriptResult is the output of the rich transcript processing method.
type TranscriptResult struct {
	Actions []ToolAction // Ordered list of actions to execute
}

// Provider is the interface for LLM providers.
type Provider interface {
	// ExtractIntent extracts a subject from text using function/tool calling.
	ExtractIntent(ctx context.Context, text string) (*IntentResult, error)
	// ProcessTranscript processes a transcript with word-level data using
	// tool_choice:any with show_media, text_to_speech, and wait_for_more tools.
	// Returns ordered actions to execute. At most one show_media per response.
	ProcessTranscript(ctx context.Context, req TranscriptRequest) (*TranscriptResult, error)
	// HealthCheck verifies the LLM provider is reachable and API key is valid.
	HealthCheck(ctx context.Context) error
}

// Config holds configuration for LLM providers.
type Config struct {
	Provider string // "anthropic" or "openai"
	APIKey   string
	BaseURL  string // Optional, for testing or custom endpoints
	Model    string // Optional, provider-specific default used if empty
	Client   *http.Client
}

// NewProvider creates a new LLM provider based on config.
func NewProvider(cfg Config) (Provider, error) {
	if cfg.Client == nil {
		cfg.Client = &http.Client{Timeout: 30 * time.Second}
	}

	switch cfg.Provider {
	case "anthropic", "":
		return newAnthropicProvider(cfg)
	case "openai":
		return newOpenAIProvider(cfg)
	default:
		return nil, fmt.Errorf("unknown provider: %s", cfg.Provider)
	}
}

// NewProviderFromEnv creates a provider using environment variables.
// LLM_PROVIDER: "anthropic" (default) or "openai"
// ANTHROPIC_API_KEY or OPENAI_API_KEY depending on provider
func NewProviderFromEnv() (Provider, error) {
	provider := os.Getenv("LLM_PROVIDER")
	if provider == "" {
		provider = "anthropic"
	}

	var apiKey string
	switch provider {
	case "anthropic":
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	case "openai":
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	return NewProvider(Config{
		Provider: provider,
		APIKey:   apiKey,
	})
}
