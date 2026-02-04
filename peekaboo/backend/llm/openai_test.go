package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIProvider_ExtractIntent_Success(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("Expected /v1/chat/completions, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("Expected Authorization header with Bearer token")
		}

		// Parse and verify request body
		var req openaiRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}
		if req.Model != "gpt-4o-mini" {
			t.Errorf("Expected gpt-4o-mini model, got %s", req.Model)
		}
		if len(req.Tools) != 1 || req.Tools[0].Function.Name != "show_media" {
			t.Errorf("Expected show_media function")
		}

		// Return mock response with function call
		resp := openaiResponse{
			ID:      "chatcmpl-123",
			Object:  "chat.completion",
			Created: 1699000000,
			Model:   "gpt-4o-mini",
			Choices: []openaiChoice{
				{
					Index: 0,
					Message: openaiMessage{
						Role: "assistant",
					},
					ToolCalls: []toolCall{
						{
							ID:   "call_123",
							Type: "function",
							Function: functionCall{
								Name:      "show_media",
								Arguments: `{"subject": "cat"}`,
							},
						},
					},
					FinishReason: "tool_calls",
				},
			},
			Usage: openaiUsage{
				PromptTokens:     50,
				CompletionTokens: 20,
				TotalTokens:      70,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	provider, err := NewProvider(Config{
		Provider: "openai",
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

func TestOpenAIProvider_ExtractIntent_APIError(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": {"message": "internal server error"}}`))
	}))
	defer mockServer.Close()

	provider, _ := NewProvider(Config{
		Provider: "openai",
		APIKey:   "test-api-key",
		BaseURL:  mockServer.URL,
	})

	_, err := provider.ExtractIntent(context.Background(), "show me a dog")
	if err == nil {
		t.Error("Expected error for API failure")
	}
}

func TestOpenAIProvider_ExtractIntent_NoFunctionCall(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openaiResponse{
			ID:      "chatcmpl-789",
			Object:  "chat.completion",
			Created: 1699000000,
			Model:   "gpt-4o-mini",
			Choices: []openaiChoice{
				{
					Index: 0,
					Message: openaiMessage{
						Role:    "assistant",
						Content: "I didn't understand.",
					},
					FinishReason: "stop",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	provider, _ := NewProvider(Config{
		Provider: "openai",
		APIKey:   "test-key",
		BaseURL:  mockServer.URL,
	})

	_, err := provider.ExtractIntent(context.Background(), "hello there")
	if err == nil {
		t.Error("Expected error when no function call in response")
	}
}

func TestOpenAIProvider_ExtractIntent_NoChoices(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openaiResponse{
			ID:      "chatcmpl-empty",
			Object:  "chat.completion",
			Created: 1699000000,
			Model:   "gpt-4o-mini",
			Choices: []openaiChoice{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	provider, _ := NewProvider(Config{
		Provider: "openai",
		APIKey:   "test-key",
		BaseURL:  mockServer.URL,
	})

	_, err := provider.ExtractIntent(context.Background(), "test")
	if err == nil {
		t.Error("Expected error when no choices in response")
	}
}

func TestOpenAIProvider_DifferentSubjects(t *testing.T) {
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
				resp := openaiResponse{
					ID:      "chatcmpl-test",
					Object:  "chat.completion",
					Created: 1699000000,
					Model:   "gpt-4o-mini",
					Choices: []openaiChoice{
						{
							Index: 0,
							ToolCalls: []toolCall{
								{
									ID:   "call_test",
									Type: "function",
									Function: functionCall{
										Name:      "show_media",
										Arguments: `{"subject": "` + tc.expected + `"}`,
									},
								},
							},
							FinishReason: "tool_calls",
						},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			}))
			defer mockServer.Close()

			provider, _ := NewProvider(Config{
				Provider: "openai",
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
