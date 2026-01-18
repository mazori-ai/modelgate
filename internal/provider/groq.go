// Package provider implements LLM provider clients.
package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"modelgate/internal/domain"
)

const groqAPIURL = "https://api.groq.com/openai/v1"

// GroqClient implements the LLMClient interface for Groq
type GroqClient struct {
	apiKey     string
	httpClient *http.Client
	modelCache map[string]string // Cache of model aliases to native model IDs
}

// NewGroqClient creates a new Groq client
func NewGroqClient(apiKey string, settings ...domain.ConnectionSettings) (*GroqClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("Groq API key is required")
	}

	// Use provided settings or defaults
	connSettings := domain.DefaultConnectionSettings()
	if len(settings) > 0 {
		connSettings = settings[0]
	}

	return &GroqClient{
		apiKey:     apiKey,
		httpClient: BuildHTTPClient(connSettings),
		modelCache: make(map[string]string),
	}, nil
}

// SetModelCache sets the model cache (implements ModelCacheable)
func (c *GroqClient) SetModelCache(cache map[string]string) {
	c.modelCache = cache
}

// GetModelCache returns the model cache (implements ModelCacheable)
func (c *GroqClient) GetModelCache() map[string]string {
	return c.modelCache
}

// resolveModelID resolves a model ID using the cache if available
func (c *GroqClient) resolveModelID(model string) string {
	if c.modelCache != nil {
		if nativeID, ok := c.modelCache[model]; ok {
			return ExtractModelID(nativeID)
		}
	}
	return ExtractModelID(model)
}

// Provider returns the provider type
func (c *GroqClient) Provider() domain.Provider {
	return domain.ProviderGroq
}

// SupportsModel checks if a model is supported
func (c *GroqClient) SupportsModel(model string) bool {
	groqModels := []string{
		"llama-3.3-70b-versatile",
		"llama-3.1-70b-versatile",
		"llama-3.1-8b-instant",
		"llama3-groq-70b-8192-tool-use-preview",
		"llama3-groq-8b-8192-tool-use-preview",
		"mixtral-8x7b-32768",
		"gemma2-9b-it",
		"gemma-7b-it",
	}
	for _, m := range groqModels {
		if strings.EqualFold(model, m) || strings.Contains(strings.ToLower(model), strings.ToLower(m)) {
			return true
		}
	}
	return false
}

// ChatStream performs streaming chat completion
func (c *GroqClient) ChatStream(ctx context.Context, req *domain.ChatRequest) (<-chan domain.StreamEvent, error) {
	events := make(chan domain.StreamEvent, 100)

	go func() {
		defer close(events)

		url := groqAPIURL + "/chat/completions"
		messages := c.buildMessages(req)

		body := map[string]any{
			"model":    req.Model,
			"messages": messages,
			"stream":   true,
		}

		if req.Temperature != nil {
			body["temperature"] = *req.Temperature
		}
		if req.MaxTokens != nil {
			body["max_tokens"] = *req.MaxTokens
		}
		if len(req.Tools) > 0 {
			body["tools"] = c.convertTools(req.Tools)
		}

		jsonBody, _ := json.Marshal(body)
		httpReq, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonBody)))
		if err != nil {
			events <- domain.FinishEvent{Reason: domain.FinishReasonError}
			return
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			events <- domain.FinishEvent{Reason: domain.FinishReasonError}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			events <- domain.TextChunk{Content: fmt.Sprintf("Groq error: %s", string(bodyBytes))}
			events <- domain.FinishEvent{Reason: domain.FinishReasonError}
			return
		}

		c.processSSEStream(resp.Body, events)
	}()

	return events, nil
}

// ChatComplete performs non-streaming chat completion
func (c *GroqClient) ChatComplete(ctx context.Context, req *domain.ChatRequest) (*domain.ChatResponse, error) {
	url := groqAPIURL + "/chat/completions"
	messages := c.buildMessages(req)

	body := map[string]any{
		"model":    req.Model,
		"messages": messages,
	}

	if req.Temperature != nil {
		body["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		body["max_tokens"] = *req.MaxTokens
	}
	if len(req.Tools) > 0 {
		body["tools"] = c.convertTools(req.Tools)
	}

	jsonBody, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Groq API error: %s", string(bodyBytes))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				ToolCalls []struct {
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int32   `json:"prompt_tokens"`
			CompletionTokens int32   `json:"completion_tokens"`
			TotalTokens      int32   `json:"total_tokens"`
			PromptTime       float64 `json:"prompt_time"`
			CompletionTime   float64 `json:"completion_time"`
		} `json:"usage"`
		Model string `json:"model"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	response := &domain.ChatResponse{
		Model: result.Model,
		Usage: &domain.UsageEvent{
			PromptTokens:     result.Usage.PromptTokens,
			CompletionTokens: result.Usage.CompletionTokens,
			TotalTokens:      result.Usage.TotalTokens,
		},
	}

	if len(result.Choices) > 0 {
		response.Content = result.Choices[0].Message.Content
		response.FinishReason = domain.FinishReason(result.Choices[0].FinishReason)

		for _, tc := range result.Choices[0].Message.ToolCalls {
			var args map[string]any
			json.Unmarshal([]byte(tc.Function.Arguments), &args)
			response.ToolCalls = append(response.ToolCalls, domain.ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: domain.FunctionCall{
					Name:      tc.Function.Name,
					Arguments: args,
				},
			})
		}
	}

	return response, nil
}

// Embed generates embeddings (Groq doesn't support embeddings yet)
func (c *GroqClient) Embed(ctx context.Context, model string, texts []string, dimensions *int32) ([][]float32, int64, error) {
	return nil, 0, fmt.Errorf("Groq does not support embeddings")
}

// CountTokens counts tokens in a request
func (c *GroqClient) CountTokens(ctx context.Context, req *domain.ChatRequest) (int32, error) {
	var total int32
	for _, msg := range req.Messages {
		for _, block := range msg.Content {
			if block.Type == "text" {
				total += int32(len(block.Text) / 4)
			}
		}
	}
	return total, nil
}

// ListModels lists available models
func (c *GroqClient) ListModels(ctx context.Context) ([]domain.ModelInfo, error) {
	return []domain.ModelInfo{
		{
			ID:              "llama-3.3-70b-versatile",
			Name:            "Llama 3.3 70B Versatile",
			Provider:        domain.ProviderGroq,
			SupportsTools:   true,
			ContextLimit:    128000,
			OutputLimit:     32768,
			InputCostPer1M:  0.59,
			OutputCostPer1M: 0.79,
			Enabled:         true,
		},
		{
			ID:              "llama-3.1-70b-versatile",
			Name:            "Llama 3.1 70B Versatile",
			Provider:        domain.ProviderGroq,
			SupportsTools:   true,
			ContextLimit:    131072,
			OutputLimit:     8000,
			InputCostPer1M:  0.59,
			OutputCostPer1M: 0.79,
			Enabled:         true,
		},
		{
			ID:              "llama-3.1-8b-instant",
			Name:            "Llama 3.1 8B Instant",
			Provider:        domain.ProviderGroq,
			SupportsTools:   true,
			ContextLimit:    131072,
			OutputLimit:     8000,
			InputCostPer1M:  0.05,
			OutputCostPer1M: 0.08,
			Enabled:         true,
		},
		{
			ID:              "llama3-groq-70b-8192-tool-use-preview",
			Name:            "Llama 3 Groq 70B Tool Use",
			Provider:        domain.ProviderGroq,
			SupportsTools:   true,
			ContextLimit:    8192,
			InputCostPer1M:  0.89,
			OutputCostPer1M: 0.89,
			Enabled:         true,
		},
		{
			ID:              "mixtral-8x7b-32768",
			Name:            "Mixtral 8x7B",
			Provider:        domain.ProviderGroq,
			SupportsTools:   true,
			ContextLimit:    32768,
			InputCostPer1M:  0.24,
			OutputCostPer1M: 0.24,
			Enabled:         true,
		},
		{
			ID:              "gemma2-9b-it",
			Name:            "Gemma 2 9B IT",
			Provider:        domain.ProviderGroq,
			SupportsTools:   false,
			ContextLimit:    8192,
			InputCostPer1M:  0.20,
			OutputCostPer1M: 0.20,
			Enabled:         true,
		},
	}, nil
}

// Helper methods

func (c *GroqClient) buildMessages(req *domain.ChatRequest) []map[string]any {
	messages := make([]map[string]any, 0)

	if req.SystemPrompt != "" {
		messages = append(messages, map[string]any{
			"role":    "system",
			"content": req.SystemPrompt,
		})
	}

	for _, msg := range req.Messages {
		m := map[string]any{"role": msg.Role}

		if len(msg.Content) == 1 && msg.Content[0].Type == "text" {
			m["content"] = msg.Content[0].Text
		} else {
			var textContent strings.Builder
			for _, block := range msg.Content {
				if block.Type == "text" {
					textContent.WriteString(block.Text)
				}
			}
			m["content"] = textContent.String()
		}

		if len(msg.ToolCalls) > 0 {
			toolCalls := make([]map[string]any, len(msg.ToolCalls))
			for i, tc := range msg.ToolCalls {
				args, _ := json.Marshal(tc.Function.Arguments)
				toolCalls[i] = map[string]any{
					"id":   tc.ID,
					"type": "function",
					"function": map[string]any{
						"name":      tc.Function.Name,
						"arguments": string(args),
					},
				}
			}
			m["tool_calls"] = toolCalls
		}

		if msg.ToolCallID != "" {
			m["tool_call_id"] = msg.ToolCallID
		}

		messages = append(messages, m)
	}

	return messages
}

func (c *GroqClient) convertTools(tools []domain.Tool) []map[string]any {
	result := make([]map[string]any, len(tools))
	for i, tool := range tools {
		result[i] = map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        tool.Function.Name,
				"description": tool.Function.Description,
				"parameters":  tool.Function.Parameters,
			},
		}
	}
	return result
}

func (c *GroqClient) processSSEStream(body io.Reader, events chan<- domain.StreamEvent) {
	scanner := bufio.NewScanner(body)
	var inputTokens, outputTokens int32

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			events <- domain.UsageEvent{
				PromptTokens:     inputTokens,
				CompletionTokens: outputTokens,
				TotalTokens:      inputTokens + outputTokens,
			}
			events <- domain.FinishEvent{Reason: domain.FinishReasonStop}
			return
		}

		var chunk struct {
			Choices []struct {
				Delta struct {
					Content   string `json:"content"`
					ToolCalls []struct {
						Index    int    `json:"index"`
						ID       string `json:"id"`
						Type     string `json:"type"`
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			XGroq struct {
				Usage struct {
					PromptTokens     int32 `json:"prompt_tokens"`
					CompletionTokens int32 `json:"completion_tokens"`
				} `json:"usage"`
			} `json:"x_groq"`
		}

		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if chunk.XGroq.Usage.PromptTokens > 0 {
			inputTokens = chunk.XGroq.Usage.PromptTokens
			outputTokens = chunk.XGroq.Usage.CompletionTokens
		}

		if len(chunk.Choices) > 0 {
			delta := chunk.Choices[0].Delta
			if delta.Content != "" {
				events <- domain.TextChunk{Content: delta.Content}
			}

			for _, tc := range delta.ToolCalls {
				if tc.ID != "" {
					var args map[string]any
					json.Unmarshal([]byte(tc.Function.Arguments), &args)
					events <- domain.ToolCallEvent{
						ToolCall: domain.ToolCall{
							ID:   tc.ID,
							Type: "function",
							Function: domain.FunctionCall{
								Name:      tc.Function.Name,
								Arguments: args,
							},
						},
					}
				}
			}

			if chunk.Choices[0].FinishReason != "" {
				if chunk.Choices[0].FinishReason == "tool_calls" {
					events <- domain.FinishEvent{Reason: domain.FinishReasonToolCalls}
				} else {
					events <- domain.FinishEvent{Reason: domain.FinishReasonStop}
				}
				return
			}
		}
	}
}
