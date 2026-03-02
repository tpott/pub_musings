package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tpott/pub_musings/peekaboo/backend/llm"
)

// Intent handler tests for core request handling and LLM response processing.

// createTestProvider creates a provider pointing to a test server.
func createTestProvider(t *testing.T, serverURL string) llm.Provider {
	provider, err := llm.NewProvider(llm.Config{
		Provider: "anthropic",
		APIKey:   "test-api-key",
		BaseURL:  serverURL,
	})
	if err != nil {
		t.Fatalf("Failed to create test provider: %v", err)
	}
	return provider
}

// Anthropic API response types for test mocking
type anthropicTestResponse struct {
	ID      string                      `json:"id"`
	Type    string                      `json:"type"`
	Role    string                      `json:"role"`
	Content []anthropicTestContentBlock `json:"content"`
	Model   string                      `json:"model"`
}

type anthropicTestContentBlock struct {
	Type  string          `json:"type"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
	Text  string          `json:"text,omitempty"`
}

func TestIntentHandler_Success(t *testing.T) {
	// Create mock Anthropic API server
	mockAnthropic := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.URL.Path != "/v1/messages" {
			t.Errorf("Expected /v1/messages, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.Header.Get("x-api-key") != "test-api-key" {
			t.Errorf("Expected x-api-key header, got %s", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Errorf("Expected anthropic-version header, got %s", r.Header.Get("anthropic-version"))
		}

		// Return mock response with tool_use
		resp := anthropicTestResponse{
			ID:   "msg_123",
			Type: "message",
			Role: "assistant",
			Content: []anthropicTestContentBlock{
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
	defer mockAnthropic.Close()

	// Create handler with mock provider
	provider := createTestProvider(t, mockAnthropic.URL)
	handler := NewIntentHandlerWithProvider(provider)

	// Create test request
	reqBody, _ := json.Marshal(IntentRequest{Text: "show me a cat"})
	req := httptest.NewRequest(http.MethodPost, "/api/intent", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	// Make request
	handler.ServeHTTP(rr, req)

	// Check response
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp IntentResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.Subject != "cat" {
		t.Errorf("Expected subject 'cat', got %q", resp.Subject)
	}
	if resp.Error != "" {
		t.Errorf("Expected no error, got %q", resp.Error)
	}
}

func TestIntentHandler_FuzzyInput(t *testing.T) {
	// Test with fuzzy input like "I wanna see a kitty cat"
	mockAnthropic := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := anthropicTestResponse{
			ID:   "msg_456",
			Type: "message",
			Role: "assistant",
			Content: []anthropicTestContentBlock{
				{
					Type:  "tool_use",
					ID:    "toolu_456",
					Name:  "show_media",
					Input: json.RawMessage(`{"subject": "cat"}`),
				},
			},
			Model: "claude-3-haiku-20240307",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockAnthropic.Close()

	provider := createTestProvider(t, mockAnthropic.URL)
	handler := NewIntentHandlerWithProvider(provider)

	reqBody, _ := json.Marshal(IntentRequest{Text: "I wanna see a kitty cat"})
	req := httptest.NewRequest(http.MethodPost, "/api/intent", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp IntentResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Subject != "cat" {
		t.Errorf("Expected subject 'cat', got %q", resp.Subject)
	}
}

func TestIntentHandler_AnthropicError(t *testing.T) {
	mockAnthropic := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer mockAnthropic.Close()

	provider := createTestProvider(t, mockAnthropic.URL)
	handler := NewIntentHandlerWithProvider(provider)

	reqBody, _ := json.Marshal(IntentRequest{Text: "show me a dog"})
	req := httptest.NewRequest(http.MethodPost, "/api/intent", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d", rr.Code)
	}

	var resp IntentResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	// Error should be generic, not exposing internal API details
	if resp.Error != "intent extraction failed" {
		t.Errorf("Expected generic error 'intent extraction failed', got %q", resp.Error)
	}
}

func TestIntentHandler_NoToolUseInResponse(t *testing.T) {
	mockAnthropic := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return response without tool_use (just text)
		resp := anthropicTestResponse{
			ID:   "msg_789",
			Type: "message",
			Role: "assistant",
			Content: []anthropicTestContentBlock{
				{
					Type: "text",
					Text: "I'm sorry, I didn't understand what you want to see.",
				},
			},
			Model: "claude-3-haiku-20240307",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockAnthropic.Close()

	provider := createTestProvider(t, mockAnthropic.URL)
	handler := NewIntentHandlerWithProvider(provider)

	reqBody, _ := json.Marshal(IntentRequest{Text: "hello there"})
	req := httptest.NewRequest(http.MethodPost, "/api/intent", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// No tool call is not an error - it means no actionable intent was found
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp IntentResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Subject != "" {
		t.Errorf("Expected empty subject, got %q", resp.Subject)
	}
	if resp.Error != "" {
		t.Errorf("Expected no error, got %q", resp.Error)
	}
}

func TestIntentHandler_DifferentSubjects(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"dog", "show me a dog", "dog"},
		{"cow", "I want to see a cow", "cow"},
		{"duck", "can you show me a duck please", "duck"},
		{"pig", "piggy!", "pig"},
		{"chicken", "show a chicken", "chicken"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockAnthropic := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				resp := anthropicTestResponse{
					ID:   "msg_test",
					Type: "message",
					Role: "assistant",
					Content: []anthropicTestContentBlock{
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
			defer mockAnthropic.Close()

			provider := createTestProvider(t, mockAnthropic.URL)
			handler := NewIntentHandlerWithProvider(provider)

			reqBody, _ := json.Marshal(IntentRequest{Text: tc.input})
			req := httptest.NewRequest(http.MethodPost, "/api/intent", bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
			}

			var resp IntentResponse
			json.NewDecoder(rr.Body).Decode(&resp)
			if resp.Subject != tc.expected {
				t.Errorf("Expected subject %q, got %q", tc.expected, resp.Subject)
			}
		})
	}
}
