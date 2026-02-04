// Package api provides HTTP handlers for the Peekaboo backend.
package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// IntentRequest is the incoming request to /api/intent.
type IntentRequest struct {
	Text string `json:"text"`
}

// IntentResponse is the response from /api/intent.
type IntentResponse struct {
	Subject string `json:"subject,omitempty"`
	Error   string `json:"error,omitempty"`
}

// AnthropicRequest is the request to the Anthropic API.
type AnthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []AnthropicMessage `json:"messages"`
	Tools     []AnthropicTool    `json:"tools"`
}

// AnthropicMessage is a message in the Anthropic API.
type AnthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AnthropicTool defines a tool for the Anthropic API.
type AnthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

// AnthropicResponse is the response from the Anthropic API.
type AnthropicResponse struct {
	ID           string                   `json:"id"`
	Type         string                   `json:"type"`
	Role         string                   `json:"role"`
	Content      []AnthropicContentBlock  `json:"content"`
	Model        string                   `json:"model"`
	StopReason   string                   `json:"stop_reason"`
	StopSequence *string                  `json:"stop_sequence"`
	Usage        AnthropicUsage           `json:"usage"`
}

// AnthropicContentBlock is a content block in the Anthropic response.
type AnthropicContentBlock struct {
	Type  string          `json:"type"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
	Text  string          `json:"text,omitempty"`
}

// AnthropicUsage tracks API usage.
type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ShowMediaInput is the expected input from the show_media tool call.
type ShowMediaInput struct {
	Subject string `json:"subject"`
}

// IntentHandler handles POST /api/intent requests.
type IntentHandler struct {
	AnthropicURL    string
	AnthropicAPIKey string
	Client          *http.Client
}

// NewIntentHandler creates a new IntentHandler.
// If anthropicURL is empty, it defaults to the Anthropic API endpoint.
// If apiKey is empty, it reads from ANTHROPIC_API_KEY env var.
func NewIntentHandler(anthropicURL, apiKey string) *IntentHandler {
	if anthropicURL == "" {
		anthropicURL = "https://api.anthropic.com"
	}
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	return &IntentHandler{
		AnthropicURL:    anthropicURL,
		AnthropicAPIKey: apiKey,
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ServeHTTP handles the intent recognition request.
func (h *IntentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse JSON body
	var req IntentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, IntentResponse{Error: "invalid JSON body"})
		return
	}

	if req.Text == "" {
		writeJSON(w, http.StatusBadRequest, IntentResponse{Error: "missing text field"})
		return
	}

	// Extract intent using Anthropic API
	subject, err := h.extractIntent(req.Text)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, IntentResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, IntentResponse{Subject: subject})
}

// extractIntent calls the Anthropic API to extract intent from text.
func (h *IntentHandler) extractIntent(text string) (string, error) {
	// Build the tool definition
	tool := AnthropicTool{
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

	// Build the request
	apiReq := AnthropicRequest{
		Model:     "claude-3-haiku-20240307",
		MaxTokens: 256,
		Messages: []AnthropicMessage{
			{
				Role:    "user",
				Content: fmt.Sprintf("Extract the subject from this voice command. The user said: %q", text),
			},
		},
		Tools: []AnthropicTool{tool},
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, h.AnthropicURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", h.AnthropicAPIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := h.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("anthropic API error: %d: %s", resp.StatusCode, string(body))
	}

	var apiResp AnthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	// Find the tool_use content block
	for _, block := range apiResp.Content {
		if block.Type == "tool_use" && block.Name == "show_media" {
			var input ShowMediaInput
			if err := json.Unmarshal(block.Input, &input); err != nil {
				return "", fmt.Errorf("unmarshal tool input: %w", err)
			}
			return input.Subject, nil
		}
	}

	return "", fmt.Errorf("no show_media tool call in response")
}
