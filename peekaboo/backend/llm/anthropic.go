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
	Model      string                 `json:"model"`
	MaxTokens  int                    `json:"max_tokens"`
	System     string                 `json:"system,omitempty"`
	Messages   []anthropicMessage     `json:"messages"`
	Tools      []anthropicTool        `json:"tools,omitempty"`
	ToolChoice map[string]interface{} `json:"tool_choice,omitempty"`
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
	Usage   anthropicUsage          `json:"usage"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicContentBlock struct {
	Type  string          `json:"type"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
	Text  string          `json:"text,omitempty"`
}

type showMediaInput struct {
	Subject               string `json:"subject"`
	InstructionEndWordIdx *int   `json:"instruction_end_word_index"`
}

type textToSpeechInput struct {
	Text string `json:"text"`
}

type waitForMoreInput struct {
	Reason string `json:"reason"`
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
		Tools:      []anthropicTool{tool},
		ToolChoice: map[string]interface{}{"type": "auto"},
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
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
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

	// No tool call means no actionable intent found (e.g., silence, unclear speech)
	return nil, nil
}

// buildSystemPrompt creates the system prompt with available concept tags.
func buildSystemPrompt(concepts []string) string {
	var sb fmt.Stringer = &conceptBuilder{concepts: concepts}
	return sb.String()
}

type conceptBuilder struct {
	concepts []string
}

func (cb *conceptBuilder) String() string {
	result := `You are the intent engine for a voice-controlled media application for children. You receive transcribed speech and decide what action to take.

Transcription may be noisy — background sounds, microphone artifacts, or unclear pronunciation can cause errors. Low-probability words (shown in the word list) are especially suspect. Be generous in interpretation.

<available-concepts>
`
	for _, c := range cb.concepts {
		result += "<concept>" + c + "</concept>\n"
	}
	result += `</available-concepts>

Rules:
- Call show_media at most once per request.
- If the user names multiple subjects, call text_to_speech to explain you will show one, then call show_media for the first one mentioned.
- If the transcript is clearly incomplete (cut off mid-phrase), call wait_for_more.
- If you cannot match any known concept, call text_to_speech to ask the user to try again.
- Keep all spoken text short and friendly.`
	return result
}

// buildUserMessage creates the user message with per-word data.
func buildUserMessage(text string, words []WordData) string {
	msg := fmt.Sprintf("Transcript: %q\nWords:\n", text)
	for i, w := range words {
		msg += fmt.Sprintf("[%d] %q  start=%.2f end=%.2f prob=%.2f\n",
			i, w.Word, w.Start, w.End, w.Probability)
	}
	return msg
}

// transcriptTools returns the three tool definitions for ProcessTranscript.
func transcriptTools() []anthropicTool {
	return []anthropicTool{
		{
			Name:        "show_media",
			Description: "Display a photo or video of the requested subject with its sound. At most one show_media call per request.",
			InputSchema: map[string]interface{}{
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
		{
			Name:        "text_to_speech",
			Description: "Speak a short message to the user. Use for feedback, clarification, or explaining partial fulfillment.",
			InputSchema: map[string]interface{}{
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
		{
			Name:        "wait_for_more",
			Description: "The transcript appears incomplete — a sentence was cut off mid-phrase. Wait for more audio before acting.",
			InputSchema: map[string]interface{}{
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
	}
}

// ProcessTranscript processes a transcript with word-level data using tool_choice:any.
func (p *anthropicProvider) ProcessTranscript(ctx context.Context, req TranscriptRequest) (*TranscriptResult, error) {
	systemPrompt := buildSystemPrompt(req.Concepts)
	userMessage := buildUserMessage(req.Text, req.Words)

	apiReq := anthropicRequest{
		Model:     p.model,
		MaxTokens: 512,
		System:    systemPrompt,
		Messages: []anthropicMessage{
			{Role: "user", Content: userMessage},
		},
		Tools:      transcriptTools(),
		ToolChoice: map[string]interface{}{"type": "any"},
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		return nil, fmt.Errorf("anthropic API error: %d: %s", resp.StatusCode, string(body))
	}

	// Read full response body for raw logging and decoding
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB max
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var apiResp anthropicResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	result, err := parseToolActions(apiResp.Content)
	if err != nil {
		return nil, err
	}

	result.RawResponse = json.RawMessage(respBody)
	result.Model = apiResp.Model
	result.InputTokens = apiResp.Usage.InputTokens
	result.OutputTokens = apiResp.Usage.OutputTokens
	result.SystemPrompt = systemPrompt
	result.InputText = userMessage

	return result, nil
}

// parseToolActions extracts ToolActions from Anthropic content blocks.
// Enforces at most one show_media action.
func parseToolActions(blocks []anthropicContentBlock) (*TranscriptResult, error) {
	var actions []ToolAction
	showMediaSeen := false

	for _, block := range blocks {
		if block.Type != "tool_use" {
			continue
		}
		switch block.Name {
		case "show_media":
			if showMediaSeen {
				continue // At most one show_media per response
			}
			showMediaSeen = true
			var input showMediaInput
			if err := json.Unmarshal(block.Input, &input); err != nil {
				return nil, fmt.Errorf("unmarshal show_media input: %w", err)
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
			if err := json.Unmarshal(block.Input, &input); err != nil {
				return nil, fmt.Errorf("unmarshal text_to_speech input: %w", err)
			}
			actions = append(actions, ToolAction{
				Type: "text_to_speech",
				Text: input.Text,
			})
		case "wait_for_more":
			var input waitForMoreInput
			if err := json.Unmarshal(block.Input, &input); err != nil {
				return nil, fmt.Errorf("unmarshal wait_for_more input: %w", err)
			}
			actions = append(actions, ToolAction{
				Type:   "wait_for_more",
				Reason: input.Reason,
			})
		}
	}

	return &TranscriptResult{Actions: actions}, nil
}

// HealthCheck verifies the Anthropic API is reachable and API key is valid.
// Uses a minimal completion request with max_tokens=1 to minimize cost.
func (p *anthropicProvider) HealthCheck(ctx context.Context) error {
	apiReq := anthropicRequest{
		Model:     p.model,
		MaxTokens: 1,
		Messages: []anthropicMessage{
			{
				Role:    "user",
				Content: "hi",
			},
		},
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		return fmt.Errorf("anthropic API error: %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
