// Package provider implements LLM provider clients.
package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"modelgate/internal/domain"
)

const anthropicAPIVersion = "2023-06-01"

// AnthropicClient is a client for Anthropic Claude API
type AnthropicClient struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
	modelCache map[string]string // Cache of model aliases to native model IDs
}

// NewAnthropicClient creates a new Anthropic client
func NewAnthropicClient(apiKey string, settings ...domain.ConnectionSettings) (*AnthropicClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	// Use provided settings or defaults
	connSettings := domain.DefaultConnectionSettings()
	if len(settings) > 0 {
		connSettings = settings[0]
	}

	return &AnthropicClient{
		apiKey:     apiKey,
		httpClient: BuildHTTPClient(connSettings),
		baseURL:    "https://api.anthropic.com/v1",
		modelCache: make(map[string]string),
	}, nil
}

// SetModelCache sets the model cache (implements ModelCacheable)
func (c *AnthropicClient) SetModelCache(cache map[string]string) {
	c.modelCache = cache
}

// GetModelCache returns the model cache (implements ModelCacheable)
func (c *AnthropicClient) GetModelCache() map[string]string {
	return c.modelCache
}

// resolveModelID resolves a model ID using the cache if available
func (c *AnthropicClient) resolveModelID(model string) string {
	if c.modelCache != nil {
		if nativeID, ok := c.modelCache[model]; ok {
			return ExtractModelID(nativeID)
		}
	}
	return ExtractModelID(model)
}

// Provider returns the provider type
func (c *AnthropicClient) Provider() domain.Provider {
	return domain.ProviderAnthropic
}

// SupportsModel checks if a model is supported
func (c *AnthropicClient) SupportsModel(model string) bool {
	modelID := ExtractModelID(model)
	return strings.HasPrefix(strings.ToLower(modelID), "claude")
}

// ChatStream starts a streaming chat completion
func (c *AnthropicClient) ChatStream(ctx context.Context, req *domain.ChatRequest) (<-chan domain.StreamEvent, error) {
	eventChan := make(chan domain.StreamEvent, 100)

	go func() {
		defer close(eventChan)

		url := c.baseURL + "/messages"
		anthropicReq := c.buildRequest(req)
		anthropicReq["stream"] = true

		body, err := json.Marshal(anthropicReq)
		if err != nil {
			eventChan <- domain.PolicyViolationEvent{
				Message: fmt.Sprintf("Failed to marshal request: %v", err),
			}
			return
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
		if err != nil {
			return
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("x-api-key", c.apiKey)
		httpReq.Header.Set("anthropic-version", anthropicAPIVersion)

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			eventChan <- domain.FinishEvent{Reason: domain.FinishReasonError}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			eventChan <- domain.PolicyViolationEvent{
				Message: fmt.Sprintf("API error: %s - %s", resp.Status, string(bodyBytes)),
			}
			return
		}

		c.parseSSEStream(resp.Body, eventChan)
	}()

	return eventChan, nil
}

// ChatComplete performs a non-streaming chat completion
func (c *AnthropicClient) ChatComplete(ctx context.Context, req *domain.ChatRequest) (*domain.ChatResponse, error) {
	url := c.baseURL + "/messages"
	anthropicReq := c.buildRequest(req)
	anthropicReq["stream"] = false

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicAPIVersion)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(bodyBytes))
	}

	var result struct {
		ID      string `json:"id"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
		Usage      struct {
			InputTokens  int32 `json:"input_tokens"`
			OutputTokens int32 `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var content strings.Builder
	for _, c := range result.Content {
		if c.Type == "text" {
			content.WriteString(c.Text)
		}
	}

	var reason domain.FinishReason
	switch result.StopReason {
	case "end_turn":
		reason = domain.FinishReasonStop
	case "tool_use":
		reason = domain.FinishReasonToolCalls
	case "max_tokens":
		reason = domain.FinishReasonLength
	default:
		reason = domain.FinishReasonStop
	}

	return &domain.ChatResponse{
		Content: content.String(),
		Model:   req.Model,
		Usage: &domain.UsageEvent{
			PromptTokens:     result.Usage.InputTokens,
			CompletionTokens: result.Usage.OutputTokens,
			TotalTokens:      result.Usage.InputTokens + result.Usage.OutputTokens,
		},
		FinishReason: reason,
	}, nil
}

// Embed generates embeddings (not supported by Anthropic)
func (c *AnthropicClient) Embed(ctx context.Context, model string, texts []string, dimensions *int32) ([][]float32, int64, error) {
	return nil, 0, fmt.Errorf("Anthropic does not support embeddings")
}

// CountTokens counts tokens in a request
func (c *AnthropicClient) CountTokens(ctx context.Context, req *domain.ChatRequest) (int32, error) {
	url := c.baseURL + "/messages/count_tokens"

	anthropicReq := c.buildRequest(req)
	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return 0, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", anthropicAPIVersion)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result struct {
		InputTokens int32 `json:"input_tokens"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	return result.InputTokens, nil
}

// ListModels lists available models
func (c *AnthropicClient) ListModels(ctx context.Context) ([]domain.ModelInfo, error) {
	// Anthropic doesn't have a models endpoint, return known models
	return []domain.ModelInfo{
		{
			ID:                "anthropic/claude-3-5-sonnet-20241022",
			Name:              "Claude 3.5 Sonnet",
			Provider:          domain.ProviderAnthropic,
			SupportsTools:     true,
			SupportsReasoning: false,
			ContextLimit:      200000,
			OutputLimit:       8192,
			InputCostPer1M:    3.0,
			OutputCostPer1M:   15.0,
			Enabled:           true,
		},
		{
			ID:                "anthropic/claude-3-7-sonnet-20250219",
			Name:              "Claude 3.7 Sonnet",
			Provider:          domain.ProviderAnthropic,
			SupportsTools:     true,
			SupportsReasoning: true,
			ContextLimit:      200000,
			OutputLimit:       8192,
			InputCostPer1M:    3.0,
			OutputCostPer1M:   15.0,
			Enabled:           true,
		},
		{
			ID:                "anthropic/claude-3-5-haiku-20241022",
			Name:              "Claude 3.5 Haiku",
			Provider:          domain.ProviderAnthropic,
			SupportsTools:     true,
			SupportsReasoning: false,
			ContextLimit:      200000,
			OutputLimit:       8192,
			InputCostPer1M:    0.8,
			OutputCostPer1M:   4.0,
			Enabled:           true,
		},
	}, nil
}

// buildRequest builds an Anthropic API request
func (c *AnthropicClient) buildRequest(req *domain.ChatRequest) map[string]any {
	anthropicReq := map[string]any{
		"model":      ExtractModelID(req.Model),
		"max_tokens": 8192,
	}

	if req.MaxTokens != nil {
		anthropicReq["max_tokens"] = *req.MaxTokens
	}

	if req.Temperature != nil {
		anthropicReq["temperature"] = *req.Temperature
	}

	if req.SystemPrompt != "" {
		anthropicReq["system"] = req.SystemPrompt
	}

	// Build messages
	var messages []map[string]any
	for _, msg := range req.Messages {
		role := msg.Role
		if role == "user" || role == "assistant" {
			var content []map[string]any
			for _, c := range msg.Content {
				switch c.Type {
				case "text":
					content = append(content, map[string]any{
						"type": "text",
						"text": c.Text,
					})
				case "image":
					if c.ImageURL != "" {
						content = append(content, map[string]any{
							"type": "image",
							"source": map[string]any{
								"type": "url",
								"url":  c.ImageURL,
							},
						})
					}
				case "tool_result":
					if c.ToolResult != nil {
						var resultContent []map[string]any
						for _, r := range c.ToolResult.Result {
							switch r.Type {
							case "text":
								resultContent = append(resultContent, map[string]any{
									"type": "text",
									"text": r.Text,
								})
							case "json":
								resultContent = append(resultContent, map[string]any{
									"type": "text",
									"text": mustMarshal(r.JSON),
								})
							}
						}
						content = append(content, map[string]any{
							"type":        "tool_result",
							"tool_use_id": c.ToolResult.ToolCallID,
							"content":     resultContent,
						})
					}
				}
			}

			// Handle tool calls in assistant messages
			for _, tc := range msg.ToolCalls {
				content = append(content, map[string]any{
					"type":  "tool_use",
					"id":    tc.ID,
					"name":  tc.Function.Name,
					"input": tc.Function.Arguments,
				})
			}

			if len(content) > 0 {
				messages = append(messages, map[string]any{
					"role":    role,
					"content": content,
				})
			}
		}
	}

	// Add current prompt
	if req.Prompt != "" {
		messages = append(messages, map[string]any{
			"role":    "user",
			"content": req.Prompt,
		})
	}

	anthropicReq["messages"] = messages

	// Add tools
	if len(req.Tools) > 0 {
		var tools []map[string]any
		for _, tool := range req.Tools {
			tools = append(tools, map[string]any{
				"name":         tool.Function.Name,
				"description":  tool.Function.Description,
				"input_schema": tool.Function.Parameters,
			})
		}
		anthropicReq["tools"] = tools
	}

	// Extended thinking
	if req.ReasoningConfig != nil && req.ReasoningConfig.Enabled {
		budgetTokens := int32(10000)
		if req.ReasoningConfig.BudgetTokens > 0 {
			budgetTokens = req.ReasoningConfig.BudgetTokens
		}
		anthropicReq["thinking"] = map[string]any{
			"type":          "enabled",
			"budget_tokens": budgetTokens,
		}
	}

	return anthropicReq
}

// parseSSEStream parses the SSE stream from Anthropic
func (c *AnthropicClient) parseSSEStream(body io.Reader, eventChan chan<- domain.StreamEvent) {
	buf := make([]byte, 4096)
	var lineBuffer strings.Builder

	for {
		n, err := body.Read(buf)
		if n > 0 {
			chunk := string(buf[:n])
			lineBuffer.WriteString(chunk)

			content := lineBuffer.String()
			lines := strings.Split(content, "\n")

			lineBuffer.Reset()
			if !strings.HasSuffix(content, "\n") {
				lineBuffer.WriteString(lines[len(lines)-1])
				lines = lines[:len(lines)-1]
			}

			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "data: ") {
					data := strings.TrimPrefix(line, "data: ")
					c.parseChunk(data, eventChan)
				}
			}
		}

		if err != nil {
			if err != io.EOF {
				eventChan <- domain.FinishEvent{Reason: domain.FinishReasonError}
			}
			return
		}
	}
}

// parseChunk parses a JSON chunk from the stream
func (c *AnthropicClient) parseChunk(data string, eventChan chan<- domain.StreamEvent) {
	var event struct {
		Type  string `json:"type"`
		Index int    `json:"index"`
		Delta struct {
			Type       string `json:"type"`
			Text       string `json:"text"`
			StopReason string `json:"stop_reason"`
		} `json:"delta"`
		ContentBlock struct {
			Type  string `json:"type"`
			ID    string `json:"id"`
			Name  string `json:"name"`
			Input any    `json:"input"`
			Text  string `json:"text"`
		} `json:"content_block"`
		Message struct {
			Usage struct {
				InputTokens  int32 `json:"input_tokens"`
				OutputTokens int32 `json:"output_tokens"`
			} `json:"usage"`
		} `json:"message"`
		Usage struct {
			InputTokens  int32 `json:"input_tokens"`
			OutputTokens int32 `json:"output_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return
	}

	switch event.Type {
	case "content_block_delta":
		if event.Delta.Type == "text_delta" && event.Delta.Text != "" {
			eventChan <- domain.TextChunk{Content: event.Delta.Text}
		} else if event.Delta.Type == "thinking_delta" && event.Delta.Text != "" {
			eventChan <- domain.ThinkingChunk{Content: event.Delta.Text}
		}

	case "content_block_start":
		if event.ContentBlock.Type == "tool_use" {
			// Tool use block started - we'll accumulate input
		}

	case "content_block_stop":
		// Content block finished

	case "message_delta":
		// Send UsageEvent BEFORE FinishEvent so tokens are recorded correctly
		if event.Usage.OutputTokens > 0 {
			eventChan <- domain.UsageEvent{
				PromptTokens:     event.Usage.InputTokens,
				CompletionTokens: event.Usage.OutputTokens,
				TotalTokens:      event.Usage.InputTokens + event.Usage.OutputTokens,
			}
		}
		if event.Delta.StopReason != "" {
			var reason domain.FinishReason
			switch event.Delta.StopReason {
			case "end_turn":
				reason = domain.FinishReasonStop
			case "tool_use":
				reason = domain.FinishReasonToolCalls
			case "max_tokens":
				reason = domain.FinishReasonLength
			default:
				reason = domain.FinishReasonStop
			}
			eventChan <- domain.FinishEvent{Reason: reason}
		}

	case "message_start":
		if event.Message.Usage.InputTokens > 0 {
			eventChan <- domain.UsageEvent{
				PromptTokens: event.Message.Usage.InputTokens,
			}
		}

	case "message_stop":
		// Message complete
	}
}

func mustMarshal(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
