package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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

	result, err := provider.ExtractIntent(context.Background(), "hello there")
	if err != nil {
		t.Errorf("Expected no error when no function call in response, got: %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil result when no function call in response, got: %+v", result)
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

func TestOpenAIProvider_HealthCheck_Success(t *testing.T) {
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

		// Parse request body to verify it's a minimal request
		var req openaiRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}
		if req.MaxTokens != 1 {
			t.Errorf("Expected max_tokens=1 for health check, got %d", req.MaxTokens)
		}

		// Return minimal successful response
		resp := openaiResponse{
			ID:      "chatcmpl-health",
			Object:  "chat.completion",
			Created: 1699000000,
			Model:   "gpt-4o-mini",
			Choices: []openaiChoice{
				{
					Index: 0,
					Message: openaiMessage{
						Role:    "assistant",
						Content: "h",
					},
					FinishReason: "length",
				},
			},
			Usage: openaiUsage{
				PromptTokens:     5,
				CompletionTokens: 1,
				TotalTokens:      6,
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

	err = provider.HealthCheck(context.Background())
	if err != nil {
		t.Errorf("HealthCheck failed: %v", err)
	}
}

func TestOpenAIProvider_HealthCheck_APIError(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": {"message": "Invalid API key", "type": "invalid_request_error"}}`))
	}))
	defer mockServer.Close()

	provider, _ := NewProvider(Config{
		Provider: "openai",
		APIKey:   "invalid-key",
		BaseURL:  mockServer.URL,
	})

	err := provider.HealthCheck(context.Background())
	if err == nil {
		t.Error("Expected error for API failure")
	}
	if !containsSubstring(err.Error(), "401") {
		t.Errorf("Expected 401 error, got: %v", err)
	}
}

func TestOpenAIProvider_HealthCheck_NetworkError(t *testing.T) {
	// Use an invalid URL to simulate network error
	provider, _ := NewProvider(Config{
		Provider: "openai",
		APIKey:   "test-key",
		BaseURL:  "http://localhost:1", // Port 1 should fail to connect
	})

	err := provider.HealthCheck(context.Background())
	if err == nil {
		t.Error("Expected error for network failure")
	}
}

func TestOpenAIProvider_ExtractIntent_ContextCancellation(t *testing.T) {
	// Channel to coordinate test timing
	reqReceived := make(chan struct{})
	cancelDone := make(chan struct{})

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Signal that request was received
		close(reqReceived)

		// Wait for the test to signal it has cancelled the context
		<-cancelDone

		// Add a small delay to ensure client has time to process cancellation
		time.Sleep(50 * time.Millisecond)

		// Return a response (client should have already cancelled)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(openaiResponse{
			ID:      "chatcmpl-test",
			Object:  "chat.completion",
			Created: 1699000000,
			Model:   "gpt-4o-mini",
			Choices: []openaiChoice{
				{
					Index: 0,
					Message: openaiMessage{
						Role:    "assistant",
						Content: "test",
					},
					FinishReason: "stop",
				},
			},
		})
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

	// Create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Start the request in a goroutine
	errChan := make(chan error, 1)
	go func() {
		_, err := provider.ExtractIntent(ctx, "show me a cat")
		errChan <- err
	}()

	// Wait for request to be received, then cancel
	<-reqReceived
	cancel()
	close(cancelDone)

	// Wait for the error
	err = <-errChan
	if err == nil {
		t.Fatal("Expected error when context is cancelled")
	}

	// The error should be context.Canceled (wrapped in "send request" error)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}
}

func TestParseOpenAIToolActions_MissingWordIdx(t *testing.T) {
	calls := []toolCall{
		{
			ID:   "call_1",
			Type: "function",
			Function: functionCall{
				Name:      "show_media",
				Arguments: `{"subject": "cat"}`,
			},
		},
	}

	result, err := parseOpenAIToolActions(calls)
	if err != nil {
		t.Fatalf("parseOpenAIToolActions failed: %v", err)
	}
	if len(result.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(result.Actions))
	}
	if result.Actions[0].InstructionEndWordIdx != -1 {
		t.Errorf("expected InstructionEndWordIdx=-1 for missing field, got %d",
			result.Actions[0].InstructionEndWordIdx)
	}
}

func TestParseOpenAIToolActions_ZeroWordIdx(t *testing.T) {
	calls := []toolCall{
		{
			ID:   "call_1",
			Type: "function",
			Function: functionCall{
				Name:      "show_media",
				Arguments: `{"subject": "cat", "instruction_end_word_index": 0}`,
			},
		},
	}

	result, err := parseOpenAIToolActions(calls)
	if err != nil {
		t.Fatalf("parseOpenAIToolActions failed: %v", err)
	}
	if result.Actions[0].InstructionEndWordIdx != 0 {
		t.Errorf("expected InstructionEndWordIdx=0, got %d",
			result.Actions[0].InstructionEndWordIdx)
	}
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
