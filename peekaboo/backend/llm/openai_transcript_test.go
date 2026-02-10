package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// openaiTranscriptResponse builds a mock OpenAI response with given tool calls.
func openaiTranscriptResponse(calls []toolCall) openaiResponse {
	return openaiResponse{
		ID:      "chatcmpl-test",
		Object:  "chat.completion",
		Created: 1699000000,
		Model:   "gpt-4o-mini",
		Choices: []openaiChoice{
			{
				Index:        0,
				ToolCalls:    calls,
				FinishReason: "tool_calls",
			},
		},
		Usage: openaiUsage{
			PromptTokens:     200,
			CompletionTokens: 80,
			TotalTokens:      280,
		},
	}
}

func newOpenAITestProvider(t *testing.T, url string) Provider {
	t.Helper()
	provider, err := NewProvider(Config{
		Provider: "openai",
		APIKey:   "test-api-key",
		BaseURL:  url,
	})
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	return provider
}

func TestOpenAIProcessTranscript_ShowMedia(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openaiRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("Failed to decode request: %v", err)
		}
		if len(req.Messages) != 2 {
			t.Errorf("Expected 2 messages (system+user), got %d", len(req.Messages))
		}
		if len(req.Tools) != 3 {
			t.Errorf("Expected 3 tools, got %d", len(req.Tools))
		}

		resp := openaiTranscriptResponse([]toolCall{
			{ID: "call_1", Type: "function", Function: functionCall{
				Name:      "show_media",
				Arguments: `{"subject":"cat","instruction_end_word_index":3}`,
			}},
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	provider := newOpenAITestProvider(t, mockServer.URL)
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
	if a.InstructionEndWordIdx != 3 {
		t.Errorf("Expected word idx 3, got %d", a.InstructionEndWordIdx)
	}

	// Verify metadata
	if result.Model != "gpt-4o-mini" {
		t.Errorf("Expected model gpt-4o-mini, got %s", result.Model)
	}
	if result.InputTokens != 200 {
		t.Errorf("Expected 200 input tokens, got %d", result.InputTokens)
	}
	if result.OutputTokens != 80 {
		t.Errorf("Expected 80 output tokens, got %d", result.OutputTokens)
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

func TestOpenAIProcessTranscript_TTSDroppedWithShowMedia(t *testing.T) {
	// When LLM returns both TTS and show_media, TTS should be dropped
	// because the media already has its own audio.
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openaiTranscriptResponse([]toolCall{
			{ID: "call_1", Type: "function", Function: functionCall{
				Name:      "text_to_speech",
				Arguments: `{"text":"Here comes a cat!"}`,
			}},
			{ID: "call_2", Type: "function", Function: functionCall{
				Name:      "show_media",
				Arguments: `{"subject":"cat","instruction_end_word_index":3}`,
			}},
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	provider := newOpenAITestProvider(t, mockServer.URL)
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

func TestOpenAIProcessTranscript_MultipleShowMediaEnforcement(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openaiTranscriptResponse([]toolCall{
			{ID: "call_1", Type: "function", Function: functionCall{
				Name:      "show_media",
				Arguments: `{"subject":"cat","instruction_end_word_index":3}`,
			}},
			{ID: "call_2", Type: "function", Function: functionCall{
				Name:      "show_media",
				Arguments: `{"subject":"dog","instruction_end_word_index":6}`,
			}},
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	provider := newOpenAITestProvider(t, mockServer.URL)
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

func TestOpenAIProcessTranscript_WaitForMore(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openaiTranscriptResponse([]toolCall{
			{ID: "call_1", Type: "function", Function: functionCall{
				Name:      "wait_for_more",
				Arguments: `{"reason":"sentence cut off mid-phrase"}`,
			}},
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	provider := newOpenAITestProvider(t, mockServer.URL)
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

func TestOpenAIProcessTranscript_TextToSpeechOnly(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openaiTranscriptResponse([]toolCall{
			{ID: "call_1", Type: "function", Function: functionCall{
				Name:      "text_to_speech",
				Arguments: `{"text":"I don't know that animal. Try cat or dog!"}`,
			}},
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	provider := newOpenAITestProvider(t, mockServer.URL)
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

func TestOpenAIProcessTranscript_APIErrors(t *testing.T) {
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

			provider := newOpenAITestProvider(t, mockServer.URL)
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

func TestOpenAIProcessTranscript_MalformedJSON(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{not valid json`))
	}))
	defer mockServer.Close()

	provider := newOpenAITestProvider(t, mockServer.URL)
	_, err := provider.ProcessTranscript(context.Background(), sampleTranscriptRequest())
	if err == nil {
		t.Fatal("Expected error for malformed JSON response")
	}
	if !strings.Contains(err.Error(), "decode response") {
		t.Errorf("Expected 'decode response' error, got: %v", err)
	}
}

func TestOpenAIProcessTranscript_EmptyChoices(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openaiResponse{
			ID:      "chatcmpl-test",
			Object:  "chat.completion",
			Created: 1699000000,
			Model:   "gpt-4o-mini",
			Choices: []openaiChoice{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	provider := newOpenAITestProvider(t, mockServer.URL)
	_, err := provider.ProcessTranscript(context.Background(), sampleTranscriptRequest())
	if err == nil {
		t.Fatal("Expected error for empty choices")
	}
	if !strings.Contains(err.Error(), "no choices") {
		t.Errorf("Expected 'no choices' error, got: %v", err)
	}
}

func TestOpenAIProcessTranscript_InvalidFunctionArgs(t *testing.T) {
	tests := []struct {
		name     string
		funcName string
		args     string
		wantErr  string
	}{
		{
			"invalid show_media args",
			"show_media",
			`{"subject": 123}`,
			"unmarshal show_media",
		},
		{
			"invalid text_to_speech args",
			"text_to_speech",
			`{"text": 123}`,
			"unmarshal text_to_speech",
		},
		{
			"invalid wait_for_more args",
			"wait_for_more",
			`{"reason": 123}`,
			"unmarshal wait_for_more",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				resp := openaiTranscriptResponse([]toolCall{
					{ID: "call_1", Type: "function", Function: functionCall{
						Name:      tc.funcName,
						Arguments: tc.args,
					}},
				})
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			}))
			defer mockServer.Close()

			provider := newOpenAITestProvider(t, mockServer.URL)
			_, err := provider.ProcessTranscript(context.Background(), sampleTranscriptRequest())
			if err == nil {
				t.Fatal("Expected error for invalid function args")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("Expected error containing %q, got: %v", tc.wantErr, err)
			}
		})
	}
}

func TestOpenAIProcessTranscript_ContextCancellation(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer mockServer.Close()

	provider := newOpenAITestProvider(t, mockServer.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := provider.ProcessTranscript(ctx, sampleTranscriptRequest())
	if err == nil {
		t.Fatal("Expected error when context is cancelled")
	}
}

func TestOpenAIProcessTranscript_NetworkError(t *testing.T) {
	provider := newOpenAITestProvider(t, "http://localhost:1")
	_, err := provider.ProcessTranscript(context.Background(), sampleTranscriptRequest())
	if err == nil {
		t.Fatal("Expected error for network failure")
	}
	if !strings.Contains(err.Error(), "send request") {
		t.Errorf("Expected 'send request' error, got: %v", err)
	}
}

func TestOpenAIProcessTranscript_NonFunctionToolCall(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openaiTranscriptResponse([]toolCall{
			{ID: "call_1", Type: "not_function", Function: functionCall{
				Name:      "show_media",
				Arguments: `{"subject":"cat","instruction_end_word_index":3}`,
			}},
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	provider := newOpenAITestProvider(t, mockServer.URL)
	result, err := provider.ProcessTranscript(context.Background(), sampleTranscriptRequest())
	if err != nil {
		t.Fatalf("ProcessTranscript failed: %v", err)
	}
	if len(result.Actions) != 0 {
		t.Errorf("Expected 0 actions for non-function tool call, got %d", len(result.Actions))
	}
}

func TestOpenAIProcessTranscript_RequestStructure(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("Missing Authorization header")
		}

		body, _ := io.ReadAll(r.Body)
		var req openaiRequest
		json.Unmarshal(body, &req)

		if req.MaxTokens != 512 {
			t.Errorf("Expected max_tokens 512, got %d", req.MaxTokens)
		}
		if req.ToolChoice != "required" {
			t.Errorf("Expected tool_choice required, got %s", req.ToolChoice)
		}
		if len(req.Messages) != 2 {
			t.Errorf("Expected 2 messages, got %d", len(req.Messages))
		}
		if req.Messages[0].Role != "system" {
			t.Errorf("Expected first message role system, got %s", req.Messages[0].Role)
		}
		if len(req.Tools) != 3 {
			t.Errorf("Expected 3 tools, got %d", len(req.Tools))
		}

		toolNames := map[string]bool{}
		for _, tool := range req.Tools {
			toolNames[tool.Function.Name] = true
		}
		for _, name := range []string{"show_media", "text_to_speech", "wait_for_more"} {
			if !toolNames[name] {
				t.Errorf("Missing tool: %s", name)
			}
		}

		resp := openaiTranscriptResponse([]toolCall{
			{ID: "call_1", Type: "function", Function: functionCall{
				Name:      "show_media",
				Arguments: `{"subject":"cat","instruction_end_word_index":3}`,
			}},
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	provider := newOpenAITestProvider(t, mockServer.URL)
	_, err := provider.ProcessTranscript(context.Background(), sampleTranscriptRequest())
	if err != nil {
		t.Fatalf("ProcessTranscript failed: %v", err)
	}
}

func TestOpenAIProcessTranscript_ErrorBodyTruncation(t *testing.T) {
	largeBody := strings.Repeat("x", 20*1024) // 20 KB, exceeds maxErrorBodyBytes
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(largeBody))
	}))
	defer mockServer.Close()

	provider := newOpenAITestProvider(t, mockServer.URL)
	_, err := provider.ProcessTranscript(context.Background(), sampleTranscriptRequest())
	if err == nil {
		t.Fatal("Expected error")
	}
	if len(err.Error()) > 15*1024 {
		t.Errorf("Error message too large (%d bytes), should be truncated", len(err.Error()))
	}
}
