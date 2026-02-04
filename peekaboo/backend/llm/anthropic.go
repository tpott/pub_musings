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
	defaultAnthropicURL   = "https://api.anthropic.com"
	defaultAnthropicModel = "claude-3-haiku-20240307"
)

type anthropicProvider struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

func newAnthropicProvider(cfg Config) (*anthropicProvider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY is required for anthropic provider")
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultAnthropicURL
	}

	model := cfg.Model
	if model == "" {
		model = defaultAnthropicModel
	}

	return &anthropicProvider{
		baseURL: baseURL,
		apiKey:  cfg.APIKey,
		model:   model,
		client:  cfg.Client,
	}, nil
}

// Anthropic API types

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []anthropicMessage `json:"messages"`
	Tools     []anthropicTool    `json:"tools"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

type anthropicResponse struct {
	ID      string                  `json:"id"`
	Type    string                  `json:"type"`
	Role    string                  `json:"role"`
	Content []anthropicContentBlock `json:"content"`
	Model   string                  `json:"model"`
}

type anthropicContentBlock struct {
	Type  string          `json:"type"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
	Text  string          `json:"text,omitempty"`
}

type showMediaInput struct {
	Subject string `json:"subject"`
}

func (p *anthropicProvider) ExtractIntent(ctx context.Context, text string) (*IntentResult, error) {
	tool := anthropicTool{
		Name:        "show_media",
		Description: "Show a photo or video of something",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"subject": map[string]interface{}{
					"type":        "string",
					"description": "The thing to show (e.g., cat, dog, cow)",
				},
			},
			"required": []string{"subject"},
		},
	}

	apiReq := anthropicRequest{
		Model:     p.model,
		MaxTokens: 256,
		Messages: []anthropicMessage{
			{
				Role:    "user",
				Content: fmt.Sprintf("Extract the subject from this voice command. The user said: %q", text),
			},
		},
		Tools: []anthropicTool{tool},
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anthropic API error: %d: %s", resp.StatusCode, string(body))
	}

	var apiResp anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	// Find the tool_use content block
	for _, block := range apiResp.Content {
		if block.Type == "tool_use" && block.Name == "show_media" {
			var input showMediaInput
			if err := json.Unmarshal(block.Input, &input); err != nil {
				return nil, fmt.Errorf("unmarshal tool input: %w", err)
			}
			return &IntentResult{Subject: input.Subject}, nil
		}
	}

	return nil, fmt.Errorf("no show_media tool call in response")
}
