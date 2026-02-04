package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

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

		// Parse and verify request body
		var req AnthropicRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}
		if req.Model != "claude-3-haiku-20240307" {
			t.Errorf("Expected claude-3-haiku-20240307 model, got %s", req.Model)
		}
		if len(req.Tools) != 1 || req.Tools[0].Name != "show_media" {
			t.Errorf("Expected show_media tool, got %+v", req.Tools)
		}

		// Return mock response with tool_use
		resp := AnthropicResponse{
			ID:   "msg_123",
			Type: "message",
			Role: "assistant",
			Content: []AnthropicContentBlock{
				{
					Type:  "tool_use",
					ID:    "toolu_123",
					Name:  "show_media",
					Input: json.RawMessage(`{"subject": "cat"}`),
				},
			},
			Model:      "claude-3-haiku-20240307",
			StopReason: "tool_use",
			Usage: AnthropicUsage{
				InputTokens:  50,
				OutputTokens: 20,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockAnthropic.Close()

	// Create handler with mock server
	handler := NewIntentHandler(mockAnthropic.URL, "test-api-key")

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
		resp := AnthropicResponse{
			ID:   "msg_456",
			Type: "message",
			Role: "assistant",
			Content: []AnthropicContentBlock{
				{
					Type:  "tool_use",
					ID:    "toolu_456",
					Name:  "show_media",
					Input: json.RawMessage(`{"subject": "cat"}`),
				},
			},
			Model:      "claude-3-haiku-20240307",
			StopReason: "tool_use",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockAnthropic.Close()

	handler := NewIntentHandler(mockAnthropic.URL, "test-api-key")

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

func TestIntentHandler_MissingText(t *testing.T) {
	handler := NewIntentHandler("http://localhost:9999", "test-key")

	reqBody, _ := json.Marshal(IntentRequest{Text: ""})
	req := httptest.NewRequest(http.MethodPost, "/api/intent", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

func TestIntentHandler_InvalidJSON(t *testing.T) {
	handler := NewIntentHandler("http://localhost:9999", "test-key")

	req := httptest.NewRequest(http.MethodPost, "/api/intent", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}

func TestIntentHandler_WrongMethod(t *testing.T) {
	handler := NewIntentHandler("http://localhost:9999", "test-key")

	req := httptest.NewRequest(http.MethodGet, "/api/intent", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", rr.Code)
	}
}

func TestIntentHandler_AnthropicError(t *testing.T) {
	mockAnthropic := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal server error"}`))
	}))
	defer mockAnthropic.Close()

	handler := NewIntentHandler(mockAnthropic.URL, "test-key")

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
	if resp.Error == "" {
		t.Error("Expected error message in response")
	}
}

func TestIntentHandler_NoToolUseInResponse(t *testing.T) {
	mockAnthropic := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return response without tool_use (just text)
		resp := AnthropicResponse{
			ID:   "msg_789",
			Type: "message",
			Role: "assistant",
			Content: []AnthropicContentBlock{
				{
					Type: "text",
					Text: "I'm sorry, I didn't understand what you want to see.",
				},
			},
			Model:      "claude-3-haiku-20240307",
			StopReason: "end_turn",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockAnthropic.Close()

	handler := NewIntentHandler(mockAnthropic.URL, "test-key")

	reqBody, _ := json.Marshal(IntentRequest{Text: "hello there"})
	req := httptest.NewRequest(http.MethodPost, "/api/intent", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("Expected status 500, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp IntentResponse
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp.Error == "" {
		t.Error("Expected error when no tool_use in response")
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
				resp := AnthropicResponse{
					ID:   "msg_test",
					Type: "message",
					Role: "assistant",
					Content: []AnthropicContentBlock{
						{
							Type:  "tool_use",
							ID:    "toolu_test",
							Name:  "show_media",
							Input: json.RawMessage(`{"subject": "` + tc.expected + `"}`),
						},
					},
					Model:      "claude-3-haiku-20240307",
					StopReason: "tool_use",
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			}))
			defer mockAnthropic.Close()

			handler := NewIntentHandler(mockAnthropic.URL, "test-key")

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
