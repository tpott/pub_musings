package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tpott/pub_musings/peekaboo/backend/llm"
)

// Intent handler tests for input validation and provider variants.

func TestIntentHandler_MissingText(t *testing.T) {
	// Create a dummy provider (won't be called due to validation)
	provider, _ := llm.NewProvider(llm.Config{
		Provider: "anthropic",
		APIKey:   "test-key",
	})
	handler := NewIntentHandlerWithProvider(provider)

	reqBody, _ := json.Marshal(IntentRequest{Text: ""})
	req := httptest.NewRequest(http.MethodPost, "/api/intent", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

func TestIntentHandler_TextTooLong(t *testing.T) {
	// Create a dummy provider (won't be called due to validation)
	provider, _ := llm.NewProvider(llm.Config{
		Provider: "anthropic",
		APIKey:   "test-key",
	})
	handler := NewIntentHandlerWithProvider(provider)

	// Create text longer than 500 characters
	longText := make([]byte, 501)
	for i := range longText {
		longText[i] = 'a'
	}

	reqBody, _ := json.Marshal(IntentRequest{Text: string(longText)})
	req := httptest.NewRequest(http.MethodPost, "/api/intent", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}

	var resp IntentResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Error != "text too long (max 500 characters)" {
		t.Errorf("Expected 'text too long' error, got %q", resp.Error)
	}
}

func TestIntentHandler_TextAtMaxLength(t *testing.T) {
	// Test that text at exactly 500 characters is accepted
	mockAnthropic := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := anthropicTestResponse{
			ID:   "msg_max",
			Type: "message",
			Role: "assistant",
			Content: []anthropicTestContentBlock{
				{
					Type:  "tool_use",
					ID:    "toolu_max",
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

	// Create text at exactly 500 characters
	maxText := make([]byte, 500)
	for i := range maxText {
		maxText[i] = 'a'
	}

	reqBody, _ := json.Marshal(IntentRequest{Text: string(maxText)})
	req := httptest.NewRequest(http.MethodPost, "/api/intent", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	// Should succeed, not be rejected
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200 for text at max length, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestIntentHandler_InvalidJSON(t *testing.T) {
	// Create a dummy provider (won't be called due to validation)
	provider, _ := llm.NewProvider(llm.Config{
		Provider: "anthropic",
		APIKey:   "test-key",
	})
	handler := NewIntentHandlerWithProvider(provider)

	req := httptest.NewRequest(http.MethodPost, "/api/intent", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

func TestIntentHandler_WrongMethod(t *testing.T) {
	// Create a dummy provider (won't be called)
	provider, _ := llm.NewProvider(llm.Config{
		Provider: "anthropic",
		APIKey:   "test-key",
	})
	handler := NewIntentHandlerWithProvider(provider)

	req := httptest.NewRequest(http.MethodGet, "/api/intent", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", rr.Code)
	}
}

func TestIntentHandler_BodyTooLarge(t *testing.T) {
	// Create a dummy provider (won't be called due to body size limit)
	provider, _ := llm.NewProvider(llm.Config{
		Provider: "anthropic",
		APIKey:   "test-key",
	})
	handler := NewIntentHandlerWithProvider(provider)

	// Create a body larger than 5KB
	largeBody := `{"text": "` + strings.Repeat("a", 6000) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/intent", strings.NewReader(largeBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected status 413, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp IntentResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Error != "request body too large" {
		t.Errorf("Expected 'request body too large' error, got %q", resp.Error)
	}
}

func TestIntentHandler_OpenAIProvider(t *testing.T) {
	// Test that OpenAI provider also works through the handler
	mockOpenAI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request path for OpenAI
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("Expected /v1/chat/completions, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-openai-key" {
			t.Errorf("Expected Authorization header, got %s", r.Header.Get("Authorization"))
		}

		// Return OpenAI-style response with tool call
		resp := map[string]interface{}{
			"id":     "chatcmpl-123",
			"object": "chat.completion",
			"model":  "gpt-4o-mini",
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "",
					},
					"tool_calls": []map[string]interface{}{
						{
							"id":   "call_123",
							"type": "function",
							"function": map[string]interface{}{
								"name":      "show_media",
								"arguments": `{"subject":"cat"}`,
							},
						},
					},
					"finish_reason": "tool_calls",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockOpenAI.Close()

	provider, err := llm.NewProvider(llm.Config{
		Provider: "openai",
		APIKey:   "test-openai-key",
		BaseURL:  mockOpenAI.URL,
	})
	if err != nil {
		t.Fatalf("Failed to create OpenAI provider: %v", err)
	}

	handler := NewIntentHandlerWithProvider(provider)

	reqBody, _ := json.Marshal(IntentRequest{Text: "show me a cat"})
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
