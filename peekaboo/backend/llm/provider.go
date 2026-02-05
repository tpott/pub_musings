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

// Provider is the interface for LLM providers.
type Provider interface {
	// ExtractIntent extracts a subject from text using function/tool calling.
	ExtractIntent(ctx context.Context, text string) (*IntentResult, error)
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
