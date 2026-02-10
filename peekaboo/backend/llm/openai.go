package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	defaultOpenAIURL   = "https://api.openai.com"
	defaultOpenAIModel = "gpt-4o-mini"
)

type openaiProvider struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

func newOpenAIProvider(cfg Config) (*openaiProvider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY is required for openai provider")
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultOpenAIURL
	}

	model := cfg.Model
	if model == "" {
		model = defaultOpenAIModel
	}

	return &openaiProvider{
		baseURL: baseURL,
		apiKey:  cfg.APIKey,
		model:   model,
		client:  cfg.Client,
	}, nil
}

// OpenAI API types

type openaiRequest struct {
	Model      string          `json:"model"`
	Messages   []openaiMessage `json:"messages"`
	Tools      []openaiTool    `json:"tools,omitempty"`
	ToolChoice string          `json:"tool_choice,omitempty"`
	MaxTokens  int             `json:"max_tokens,omitempty"`
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiTool struct {
	Type     string         `json:"type"`
	Function openaiFunction `json:"function"`
}

type openaiFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type openaiResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []openaiChoice `json:"choices"`
	Usage   openaiUsage    `json:"usage"`
}

type openaiChoice struct {
	Index        int           `json:"index"`
	Message      openaiMessage `json:"message"`
	ToolCalls    []toolCall    `json:"tool_calls,omitempty"`
	FinishReason string        `json:"finish_reason"`
}

type toolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function functionCall `json:"function"`
}

type functionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func (p *openaiProvider) ExtractIntent(ctx context.Context, text string) (*IntentResult, error) {
	tool := openaiTool{
		Type: "function",
		Function: openaiFunction{
			Name:        "show_media",
			Description: "Show a photo or video of something",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"subject": map[string]interface{}{
						"type":        "string",
						"description": "The thing to show (e.g., cat, dog, cow)",
					},
				},
				"required": []string{"subject"},
			},
		},
	}

	apiReq := openaiRequest{
		Model: p.model,
		Messages: []openaiMessage{
			{
				Role:    "user",
				Content: fmt.Sprintf("Extract the subject from this voice command. The user said: %q", text),
			},
		},
		Tools:      []openaiTool{tool},
		ToolChoice: "auto",
		MaxTokens:  256,
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		return nil, fmt.Errorf("openai API error: %d: %s", resp.StatusCode, string(body))
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxSuccessBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var apiResp openaiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Find the function call in the first choice
	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := apiResp.Choices[0]
	for _, tc := range choice.ToolCalls {
		if tc.Type == "function" && tc.Function.Name == "show_media" {
			var input showMediaInput
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
				return nil, fmt.Errorf("unmarshal function arguments: %w", err)
			}
			return &IntentResult{Subject: input.Subject}, nil
		}
	}

	// No function call means no actionable intent found (e.g., silence, unclear speech)
	return nil, nil
}

// ProcessTranscript processes a transcript with word-level data using tool_choice:required.
func (p *openaiProvider) ProcessTranscript(ctx context.Context, req TranscriptRequest) (*TranscriptResult, error) {
	systemPrompt := buildSystemPrompt(req.Concepts)
	userMessage := buildUserMessage(req.Text, req.Words)

	tools := []openaiTool{
		{
			Type: "function",
			Function: openaiFunction{
				Name:        "show_media",
				Description: "Display a photo or video of the requested subject with its sound. At most one show_media call per request.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"subject": map[string]interface{}{
							"type":        "string",
							"description": "The media subject — must be one of the available concepts",
						},
						"instruction_end_word_index": map[string]interface{}{
							"type":        "integer",
							"description": "0-based index of the last transcript word belonging to this command",
						},
					},
					"required": []string{"subject", "instruction_end_word_index"},
				},
			},
		},
		{
			Type: "function",
			Function: openaiFunction{
				Name:        "text_to_speech",
				Description: "Speak a short message to the user. Only use when you cannot show media (unknown concept) or need to explain a limitation (multiple subjects). Never narrate what you are about to show.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"text": map[string]interface{}{
							"type":        "string",
							"description": "The message to speak aloud. Keep under 30 words.",
						},
					},
					"required": []string{"text"},
				},
			},
		},
		{
			Type: "function",
			Function: openaiFunction{
				Name:        "wait_for_more",
				Description: "The transcript appears incomplete — wait for more audio.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"reason": map[string]interface{}{
							"type":        "string",
							"description": "Why the transcript seems incomplete",
						},
					},
					"required": []string{"reason"},
				},
			},
		},
	}

	apiReq := openaiRequest{
		Model: p.model,
		Messages: []openaiMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMessage},
		},
		Tools:      tools,
		ToolChoice: "required",
		MaxTokens:  512,
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		return nil, fmt.Errorf("openai API error: %d: %s", resp.StatusCode, string(body))
	}

	// Read full response body for raw logging and decoding
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB max
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var apiResp openaiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	result, err := parseOpenAIToolActions(apiResp.Choices[0].ToolCalls)
	if err != nil {
		return nil, err
	}

	result.RawResponse = json.RawMessage(respBody)
	result.Model = apiResp.Model
	result.InputTokens = apiResp.Usage.PromptTokens
	result.OutputTokens = apiResp.Usage.CompletionTokens
	result.SystemPrompt = systemPrompt
	result.InputText = userMessage

	return result, nil
}

// parseOpenAIToolActions extracts ToolActions from OpenAI tool calls.
func parseOpenAIToolActions(calls []toolCall) (*TranscriptResult, error) {
	var actions []ToolAction
	showMediaSeen := false

	for _, tc := range calls {
		if tc.Type != "function" {
			continue
		}
		switch tc.Function.Name {
		case "show_media":
			if showMediaSeen {
				continue
			}
			showMediaSeen = true
			var input showMediaInput
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
				return nil, fmt.Errorf("unmarshal show_media: %w", err)
			}
			wordIdx := -1
			if input.InstructionEndWordIdx != nil {
				wordIdx = *input.InstructionEndWordIdx
			}
			actions = append(actions, ToolAction{
				Type:                  "show_media",
				Subject:               input.Subject,
				InstructionEndWordIdx: wordIdx,
			})
		case "text_to_speech":
			var input textToSpeechInput
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
				return nil, fmt.Errorf("unmarshal text_to_speech: %w", err)
			}
			actions = append(actions, ToolAction{
				Type: "text_to_speech",
				Text: input.Text,
			})
		case "wait_for_more":
			var input waitForMoreInput
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
				return nil, fmt.Errorf("unmarshal wait_for_more: %w", err)
			}
			actions = append(actions, ToolAction{
				Type:   "wait_for_more",
				Reason: input.Reason,
			})
		}
	}

	return &TranscriptResult{Actions: actions}, nil
}

// HealthCheck verifies the OpenAI API is reachable and API key is valid.
// Uses a minimal completion request with max_tokens=1 to minimize cost.
func (p *openaiProvider) HealthCheck(ctx context.Context) error {
	apiReq := openaiRequest{
		Model: p.model,
		Messages: []openaiMessage{
			{
				Role:    "user",
				Content: "hi",
			},
		},
		MaxTokens: 1,
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		return fmt.Errorf("openai API error: %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
