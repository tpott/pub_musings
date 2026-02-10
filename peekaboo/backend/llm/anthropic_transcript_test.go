package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// sampleTranscriptRequest returns a TranscriptRequest for testing.
func sampleTranscriptRequest() TranscriptRequest {
	return TranscriptRequest{
		Text: "show me a cat",
		Words: []WordData{
			{Word: "show", Start: 0.0, End: 0.3, Probability: 0.95},
			{Word: "me", Start: 0.3, End: 0.5, Probability: 0.98},
			{Word: "a", Start: 0.5, End: 0.6, Probability: 0.99},
			{Word: "cat", Start: 0.6, End: 0.9, Probability: 0.97},
		},
		Concepts: []string{"cat", "dog", "duck"},
	}
}

// anthropicTranscriptResponse builds a mock Anthropic response with given content blocks.
func anthropicTranscriptResponse(blocks []anthropicContentBlock) anthropicResponse {
	return anthropicResponse{
		ID:      "msg_test",
		Type:    "message",
		Role:    "assistant",
		Content: blocks,
		Model:   "claude-3-haiku-20240307",
		Usage:   anthropicUsage{InputTokens: 100, OutputTokens: 50},
	}
}

func newAnthropicTestProvider(t *testing.T, url string) Provider {
	t.Helper()
	provider, err := NewProvider(Config{
		Provider: "anthropic",
		APIKey:   "test-api-key",
		BaseURL:  url,
	})
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	return provider
}

func TestAnthropicProcessTranscript_ShowMedia(t *testing.T) {
	idx := 3
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request structure
		var req anthropicRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}
		if req.System == "" {
			t.Error("Expected non-empty system prompt")
		}
		if len(req.Tools) != 3 {
			t.Errorf("Expected 3 tools, got %d", len(req.Tools))
		}

		resp := anthropicTranscriptResponse([]anthropicContentBlock{
			{
				Type:  "tool_use",
				ID:    "toolu_1",
				Name:  "show_media",
				Input: json.RawMessage(fmt.Sprintf(`{"subject":"cat","instruction_end_word_index":%d}`, idx)),
			},
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	provider := newAnthropicTestProvider(t, mockServer.URL)
	result, err := provider.ProcessTranscript(context.Background(), sampleTranscriptRequest())
	if err != nil {
		t.Fatalf("ProcessTranscript failed: %v", err)
	}

	if len(result.Actions) != 1 {
		t.Fatalf("Expected 1 action, got %d", len(result.Actions))
	}
	a := result.Actions[0]
	if a.Type != "show_media" {
		t.Errorf("Expected show_media, got %s", a.Type)
	}
	if a.Subject != "cat" {
		t.Errorf("Expected subject cat, got %s", a.Subject)
	}
	if a.InstructionEndWordIdx != idx {
		t.Errorf("Expected word idx %d, got %d", idx, a.InstructionEndWordIdx)
	}

	// Verify metadata fields populated
	if result.Model != "claude-3-haiku-20240307" {
		t.Errorf("Expected model claude-3-haiku-20240307, got %s", result.Model)
	}
	if result.InputTokens != 100 {
		t.Errorf("Expected 100 input tokens, got %d", result.InputTokens)
	}
	if result.OutputTokens != 50 {
		t.Errorf("Expected 50 output tokens, got %d", result.OutputTokens)
	}
	if result.SystemPrompt == "" {
		t.Error("Expected non-empty SystemPrompt")
	}
	if result.InputText == "" {
		t.Error("Expected non-empty InputText")
	}
	if result.RawResponse == nil {
		t.Error("Expected non-nil RawResponse")
	}
}

func TestAnthropicProcessTranscript_TTSDroppedWithShowMedia(t *testing.T) {
	// When LLM returns both TTS and show_media, TTS should be dropped
	// because the media already has its own audio.
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := anthropicTranscriptResponse([]anthropicContentBlock{
			{
				Type:  "tool_use",
				ID:    "toolu_1",
				Name:  "text_to_speech",
				Input: json.RawMessage(`{"text":"Here comes a cat!"}`),
			},
			{
				Type:  "tool_use",
				ID:    "toolu_2",
				Name:  "show_media",
				Input: json.RawMessage(`{"subject":"cat","instruction_end_word_index":3}`),
			},
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	provider := newAnthropicTestProvider(t, mockServer.URL)
	result, err := provider.ProcessTranscript(context.Background(), sampleTranscriptRequest())
	if err != nil {
		t.Fatalf("ProcessTranscript failed: %v", err)
	}

	if len(result.Actions) != 1 {
		t.Fatalf("Expected 1 action (TTS dropped), got %d", len(result.Actions))
	}
	if result.Actions[0].Type != "show_media" {
		t.Errorf("Expected show_media action, got %s", result.Actions[0].Type)
	}
	if result.Actions[0].Subject != "cat" {
		t.Errorf("Expected subject cat, got %s", result.Actions[0].Subject)
	}
}

func TestAnthropicProcessTranscript_MultipleShowMediaEnforcement(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := anthropicTranscriptResponse([]anthropicContentBlock{
			{
				Type:  "tool_use",
				ID:    "toolu_1",
				Name:  "show_media",
				Input: json.RawMessage(`{"subject":"cat","instruction_end_word_index":3}`),
			},
			{
				Type:  "tool_use",
				ID:    "toolu_2",
				Name:  "show_media",
				Input: json.RawMessage(`{"subject":"dog","instruction_end_word_index":6}`),
			},
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	provider := newAnthropicTestProvider(t, mockServer.URL)
	result, err := provider.ProcessTranscript(context.Background(), sampleTranscriptRequest())
	if err != nil {
		t.Fatalf("ProcessTranscript failed: %v", err)
	}

	showCount := 0
	for _, a := range result.Actions {
		if a.Type == "show_media" {
			showCount++
		}
	}
	if showCount != 1 {
		t.Errorf("Expected exactly 1 show_media, got %d", showCount)
	}
	if result.Actions[0].Subject != "cat" {
		t.Errorf("Expected first show_media subject cat, got %s", result.Actions[0].Subject)
	}
}

func TestAnthropicProcessTranscript_WaitForMore(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := anthropicTranscriptResponse([]anthropicContentBlock{
			{
				Type:  "tool_use",
				ID:    "toolu_1",
				Name:  "wait_for_more",
				Input: json.RawMessage(`{"reason":"sentence cut off mid-phrase"}`),
			},
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	provider := newAnthropicTestProvider(t, mockServer.URL)
	result, err := provider.ProcessTranscript(context.Background(), sampleTranscriptRequest())
	if err != nil {
		t.Fatalf("ProcessTranscript failed: %v", err)
	}

	if len(result.Actions) != 1 {
		t.Fatalf("Expected 1 action, got %d", len(result.Actions))
	}
	if result.Actions[0].Type != "wait_for_more" {
		t.Errorf("Expected wait_for_more, got %s", result.Actions[0].Type)
	}
	if result.Actions[0].Reason != "sentence cut off mid-phrase" {
		t.Errorf("Unexpected reason: %s", result.Actions[0].Reason)
	}
}

func TestAnthropicProcessTranscript_TextToSpeechOnly(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := anthropicTranscriptResponse([]anthropicContentBlock{
			{
				Type:  "tool_use",
				ID:    "toolu_1",
				Name:  "text_to_speech",
				Input: json.RawMessage(`{"text":"I don't know that animal. Try cat or dog!"}`),
			},
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	provider := newAnthropicTestProvider(t, mockServer.URL)
	result, err := provider.ProcessTranscript(context.Background(), sampleTranscriptRequest())
	if err != nil {
		t.Fatalf("ProcessTranscript failed: %v", err)
	}

	if len(result.Actions) != 1 {
		t.Fatalf("Expected 1 action, got %d", len(result.Actions))
	}
	if result.Actions[0].Type != "text_to_speech" {
		t.Errorf("Expected text_to_speech, got %s", result.Actions[0].Type)
	}
}

func TestAnthropicProcessTranscript_APIErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErr    string
	}{
		{"400 bad request", 400, `{"error":"bad request"}`, "400"},
		{"429 rate limit", 429, `{"error":"rate limited"}`, "429"},
		{"500 server error", 500, `{"error":"internal"}`, "500"},
		{"503 unavailable", 503, `{"error":"unavailable"}`, "503"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				w.Write([]byte(tc.body))
			}))
			defer mockServer.Close()

			provider := newAnthropicTestProvider(t, mockServer.URL)
			_, err := provider.ProcessTranscript(context.Background(), sampleTranscriptRequest())
			if err == nil {
				t.Fatal("Expected error for API failure")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("Expected error containing %q, got: %v", tc.wantErr, err)
			}
		})
	}
}

func TestAnthropicProcessTranscript_MalformedJSON(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{not valid json`))
	}))
	defer mockServer.Close()

	provider := newAnthropicTestProvider(t, mockServer.URL)
	_, err := provider.ProcessTranscript(context.Background(), sampleTranscriptRequest())
	if err == nil {
		t.Fatal("Expected error for malformed JSON response")
	}
	if !strings.Contains(err.Error(), "decode response") {
		t.Errorf("Expected 'decode response' error, got: %v", err)
	}
}

func TestAnthropicProcessTranscript_InvalidToolInputs(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		input    string
		wantErr  string
	}{
		{
			"invalid show_media input",
			"show_media",
			`{"subject": 123}`,
			"unmarshal show_media input",
		},
		{
			"invalid text_to_speech input",
			"text_to_speech",
			`{"text": 123}`,
			"unmarshal text_to_speech input",
		},
		{
			"invalid wait_for_more input",
			"wait_for_more",
			`{"reason": 123}`,
			"unmarshal wait_for_more input",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				resp := anthropicTranscriptResponse([]anthropicContentBlock{
					{
						Type:  "tool_use",
						ID:    "toolu_1",
						Name:  tc.toolName,
						Input: json.RawMessage(tc.input),
					},
				})
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			}))
			defer mockServer.Close()

			provider := newAnthropicTestProvider(t, mockServer.URL)
			_, err := provider.ProcessTranscript(context.Background(), sampleTranscriptRequest())
			if err == nil {
				t.Fatal("Expected error for invalid tool input")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("Expected error containing %q, got: %v", tc.wantErr, err)
			}
		})
	}
}

func TestAnthropicProcessTranscript_ContextCancellation(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block until context cancelled
		<-r.Context().Done()
	}))
	defer mockServer.Close()

	provider := newAnthropicTestProvider(t, mockServer.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := provider.ProcessTranscript(ctx, sampleTranscriptRequest())
	if err == nil {
		t.Fatal("Expected error when context is cancelled")
	}
}

func TestAnthropicProcessTranscript_NetworkError(t *testing.T) {
	provider := newAnthropicTestProvider(t, "http://localhost:1")
	_, err := provider.ProcessTranscript(context.Background(), sampleTranscriptRequest())
	if err == nil {
		t.Fatal("Expected error for network failure")
	}
	if !strings.Contains(err.Error(), "send request") {
		t.Errorf("Expected 'send request' error, got: %v", err)
	}
}

func TestAnthropicProcessTranscript_NoToolUseBlocks(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := anthropicTranscriptResponse([]anthropicContentBlock{
			{Type: "text", Text: "I'm not sure what to do."},
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	provider := newAnthropicTestProvider(t, mockServer.URL)
	result, err := provider.ProcessTranscript(context.Background(), sampleTranscriptRequest())
	if err != nil {
		t.Fatalf("ProcessTranscript failed: %v", err)
	}
	if len(result.Actions) != 0 {
		t.Errorf("Expected 0 actions for text-only response, got %d", len(result.Actions))
	}
}

func TestAnthropicProcessTranscript_RequestStructure(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify HTTP details
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.Header.Get("x-api-key") != "test-api-key" {
			t.Errorf("Missing x-api-key header")
		}
		if r.Header.Get("anthropic-version") != "2023-06-01" {
			t.Errorf("Missing anthropic-version header")
		}

		// Verify request body
		body, _ := io.ReadAll(r.Body)
		var req anthropicRequest
		json.Unmarshal(body, &req)

		if req.MaxTokens != 512 {
			t.Errorf("Expected max_tokens 512, got %d", req.MaxTokens)
		}
		if req.System == "" {
			t.Error("Expected system prompt")
		}
		if !strings.Contains(req.System, "cat") {
			t.Error("System prompt should contain concept 'cat'")
		}
		if len(req.Tools) != 3 {
			t.Errorf("Expected 3 tools, got %d", len(req.Tools))
		}

		toolNames := map[string]bool{}
		for _, tool := range req.Tools {
			toolNames[tool.Name] = true
		}
		for _, name := range []string{"show_media", "text_to_speech", "wait_for_more"} {
			if !toolNames[name] {
				t.Errorf("Missing tool: %s", name)
			}
		}

		resp := anthropicTranscriptResponse([]anthropicContentBlock{
			{Type: "tool_use", ID: "toolu_1", Name: "show_media",
				Input: json.RawMessage(`{"subject":"cat","instruction_end_word_index":3}`)},
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	provider := newAnthropicTestProvider(t, mockServer.URL)
	_, err := provider.ProcessTranscript(context.Background(), sampleTranscriptRequest())
	if err != nil {
		t.Fatalf("ProcessTranscript failed: %v", err)
	}
}

func TestAnthropicProcessTranscript_ErrorBodyTruncation(t *testing.T) {
	largeBody := strings.Repeat("x", 20*1024) // 20 KB, exceeds maxErrorBodyBytes
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(largeBody))
	}))
	defer mockServer.Close()

	provider := newAnthropicTestProvider(t, mockServer.URL)
	_, err := provider.ProcessTranscript(context.Background(), sampleTranscriptRequest())
	if err == nil {
		t.Fatal("Expected error")
	}
	if len(err.Error()) > 15*1024 {
		t.Errorf("Error message too large (%d bytes), should be truncated", len(err.Error()))
	}
}
