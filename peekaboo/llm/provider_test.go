package llm

import (
	"testing"
)

func TestNewProvider_Anthropic(t *testing.T) {
	provider, err := NewProvider(Config{
		Provider: "anthropic",
		APIKey:   "test-key",
	})
	if err != nil {
		t.Fatalf("Failed to create anthropic provider: %v", err)
	}

	_, ok := provider.(*anthropicProvider)
	if !ok {
		t.Error("Expected anthropicProvider type")
	}
}

func TestNewProvider_OpenAI(t *testing.T) {
	provider, err := NewProvider(Config{
		Provider: "openai",
		APIKey:   "test-key",
	})
	if err != nil {
		t.Fatalf("Failed to create openai provider: %v", err)
	}

	_, ok := provider.(*openaiProvider)
	if !ok {
		t.Error("Expected openaiProvider type")
	}
}

func TestNewProvider_Default(t *testing.T) {
	// Empty provider string should default to anthropic
	provider, err := NewProvider(Config{
		Provider: "",
		APIKey:   "test-key",
	})
	if err != nil {
		t.Fatalf("Failed to create default provider: %v", err)
	}

	_, ok := provider.(*anthropicProvider)
	if !ok {
		t.Error("Expected anthropicProvider type for default")
	}
}

func TestNewProvider_Unknown(t *testing.T) {
	_, err := NewProvider(Config{
		Provider: "unknown",
		APIKey:   "test-key",
	})
	if err == nil {
		t.Error("Expected error for unknown provider")
	}
}

func TestNewProvider_CustomModel(t *testing.T) {
	provider, err := NewProvider(Config{
		Provider: "anthropic",
		APIKey:   "test-key",
		Model:    "claude-3-5-sonnet-20241022",
	})
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	ap, ok := provider.(*anthropicProvider)
	if !ok {
		t.Fatal("Expected anthropicProvider type")
	}

	if ap.model != "claude-3-5-sonnet-20241022" {
		t.Errorf("Expected custom model, got %s", ap.model)
	}
}

func TestNewProvider_CustomBaseURL(t *testing.T) {
	provider, err := NewProvider(Config{
		Provider: "openai",
		APIKey:   "test-key",
		BaseURL:  "https://custom.openai.com",
	})
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	op, ok := provider.(*openaiProvider)
	if !ok {
		t.Fatal("Expected openaiProvider type")
	}

	if op.baseURL != "https://custom.openai.com" {
		t.Errorf("Expected custom base URL, got %s", op.baseURL)
	}
}
