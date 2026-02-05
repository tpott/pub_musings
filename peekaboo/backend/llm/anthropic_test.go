package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAnthropicProvider_ExtractIntent_Success(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.URL.Path != "/v1/messages" {
			t.Errorf("Expected /v1/messages, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.Header.Get("x-api-key") != "test-api-key" {
			t.Errorf("Expected x-api-key header")
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Errorf("Expected anthropic-version header")
		}

		// Parse and verify request body
		var req anthropicRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}
		if req.Model != "claude-3-haiku-20240307" {
			t.Errorf("Expected claude-3-haiku-20240307 model, got %s", req.Model)
		}
		if len(req.Tools) != 1 || req.Tools[0].Name != "show_media" {
			t.Errorf("Expected show_media tool")
		}

		// Return mock response with tool_use
		resp := anthropicResponse{
			ID:   "msg_123",
			Type: "message",
			Role: "assistant",
			Content: []anthropicContentBlock{
				{
					Type:  "tool_use",
					ID:    "toolu_123",
					Name:  "show_media",
					Input: json.RawMessage(`{"subject": "cat"}`),
				},
			},
			Model: "claude-3-haiku-20240307",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	provider, err := NewProvider(Config{
		Provider: "anthropic",
		APIKey:   "test-api-key",
		BaseURL:  mockServer.URL,
	})
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	result, err := provider.ExtractIntent(context.Background(), "show me a cat")
	if err != nil {
		t.Fatalf("ExtractIntent failed: %v", err)
	}

	if result.Subject != "cat" {
		t.Errorf("Expected subject 'cat', got %q", result.Subject)
	}
}

func TestAnthropicProvider_ExtractIntent_APIError(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer mockServer.Close()

	provider, _ := NewProvider(Config{
		Provider: "anthropic",
		APIKey:   "test-api-key",
		BaseURL:  mockServer.URL,
	})

	_, err := provider.ExtractIntent(context.Background(), "show me a dog")
	if err == nil {
		t.Error("Expected error for API failure")
	}
}

func TestAnthropicProvider_ExtractIntent_NoToolUse(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := anthropicResponse{
			ID:   "msg_789",
			Type: "message",
			Role: "assistant",
			Content: []anthropicContentBlock{
				{
					Type: "text",
					Text: "I didn't understand.",
				},
			},
			Model: "claude-3-haiku-20240307",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	provider, _ := NewProvider(Config{
		Provider: "anthropic",
		APIKey:   "test-key",
		BaseURL:  mockServer.URL,
	})

	result, err := provider.ExtractIntent(context.Background(), "hello there")
	if err != nil {
		t.Errorf("Expected no error when no tool_use in response, got: %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil result when no tool_use in response, got: %+v", result)
	}
}

func TestAnthropicProvider_DifferentSubjects(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"dog", "show me a dog", "dog"},
		{"cow", "I want to see a cow", "cow"},
		{"duck", "can you show me a duck please", "duck"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				resp := anthropicResponse{
					ID:   "msg_test",
					Type: "message",
					Role: "assistant",
					Content: []anthropicContentBlock{
						{
							Type:  "tool_use",
							ID:    "toolu_test",
							Name:  "show_media",
							Input: json.RawMessage(`{"subject": "` + tc.expected + `"}`),
						},
					},
					Model: "claude-3-haiku-20240307",
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			}))
			defer mockServer.Close()

			provider, _ := NewProvider(Config{
				Provider: "anthropic",
				APIKey:   "test-key",
				BaseURL:  mockServer.URL,
			})

			result, err := provider.ExtractIntent(context.Background(), tc.input)
			if err != nil {
				t.Fatalf("ExtractIntent failed: %v", err)
			}

			if result.Subject != tc.expected {
				t.Errorf("Expected subject %q, got %q", tc.expected, result.Subject)
			}
		})
	}
}

func TestAnthropicProvider_HealthCheck_Success(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.URL.Path != "/v1/messages" {
			t.Errorf("Expected /v1/messages, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.Header.Get("x-api-key") != "test-api-key" {
			t.Errorf("Expected x-api-key header")
		}

		// Parse request body to verify it's a minimal request
		var req anthropicRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}
		if req.MaxTokens != 1 {
			t.Errorf("Expected max_tokens=1 for health check, got %d", req.MaxTokens)
		}

		// Return minimal successful response
		resp := anthropicResponse{
			ID:      "msg_health",
			Type:    "message",
			Role:    "assistant",
			Content: []anthropicContentBlock{{Type: "text", Text: "h"}},
			Model:   "claude-3-haiku-20240307",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	provider, err := NewProvider(Config{
		Provider: "anthropic",
		APIKey:   "test-api-key",
		BaseURL:  mockServer.URL,
	})
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	err = provider.HealthCheck(context.Background())
	if err != nil {
		t.Errorf("HealthCheck failed: %v", err)
	}
}

func TestAnthropicProvider_HealthCheck_APIError(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": {"type": "authentication_error", "message": "invalid api key"}}`))
	}))
	defer mockServer.Close()

	provider, _ := NewProvider(Config{
		Provider: "anthropic",
		APIKey:   "invalid-key",
		BaseURL:  mockServer.URL,
	})

	err := provider.HealthCheck(context.Background())
	if err == nil {
		t.Error("Expected error for API failure")
	}
	if !contains(err.Error(), "401") {
		t.Errorf("Expected 401 error, got: %v", err)
	}
}

func TestAnthropicProvider_HealthCheck_NetworkError(t *testing.T) {
	// Use an invalid URL to simulate network error
	provider, _ := NewProvider(Config{
		Provider: "anthropic",
		APIKey:   "test-key",
		BaseURL:  "http://localhost:1", // Port 1 should fail to connect
	})

	err := provider.HealthCheck(context.Background())
	if err == nil {
		t.Error("Expected error for network failure")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
