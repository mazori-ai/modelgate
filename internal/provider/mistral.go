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

const mistralAPIURL = "https://api.mistral.ai/v1"

// MistralClient implements the LLMClient interface for Mistral AI
type MistralClient struct {
	apiKey     string
	httpClient *http.Client
	modelCache map[string]string // Cache of model aliases to native model IDs
}

// NewMistralClient creates a new Mistral client
func NewMistralClient(apiKey string, settings ...domain.ConnectionSettings) (*MistralClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("Mistral API key is required")
	}

	// Use provided settings or defaults
	connSettings := domain.DefaultConnectionSettings()
	if len(settings) > 0 {
		connSettings = settings[0]
	}

	return &MistralClient{
		apiKey:     apiKey,
		httpClient: BuildHTTPClient(connSettings),
		modelCache: make(map[string]string),
	}, nil
}

// SetModelCache sets the model cache (implements ModelCacheable)
func (c *MistralClient) SetModelCache(cache map[string]string) {
	c.modelCache = cache
}

// GetModelCache returns the model cache (implements ModelCacheable)
func (c *MistralClient) GetModelCache() map[string]string {
	return c.modelCache
}

// resolveModelID resolves a model ID using the cache if available
func (c *MistralClient) resolveModelID(model string) string {
	if c.modelCache != nil {
		if nativeID, ok := c.modelCache[model]; ok {
			return ExtractModelID(nativeID)
		}
	}
	return ExtractModelID(model)
}

// Provider returns the provider type
func (c *MistralClient) Provider() domain.Provider {
	return domain.ProviderMistral
}

// SupportsModel checks if a model is supported
func (c *MistralClient) SupportsModel(model string) bool {
	mistralModels := []string{
		"mistral-large-latest",
		"mistral-large-2411",
		"pixtral-large-latest",
		"ministral-3b-latest",
		"ministral-8b-latest",
		"mistral-small-latest",
		"codestral-latest",
		"mistral-embed",
		"mistral-moderation-latest",
	}
	for _, m := range mistralModels {
		if strings.EqualFold(model, m) || strings.Contains(strings.ToLower(model), strings.ToLower(m)) {
			return true
		}
	}
	return false
}

// ChatStream performs streaming chat completion
func (c *MistralClient) ChatStream(ctx context.Context, req *domain.ChatRequest) (<-chan domain.StreamEvent, error) {
	events := make(chan domain.StreamEvent, 100)

	go func() {
		defer close(events)

		url := mistralAPIURL + "/chat/completions"
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
			events <- domain.TextChunk{Content: fmt.Sprintf("Mistral error: %s", string(bodyBytes))}
			events <- domain.FinishEvent{Reason: domain.FinishReasonError}
			return
		}

		c.processSSEStream(resp.Body, events)
	}()

	return events, nil
}

// ChatComplete performs non-streaming chat completion
func (c *MistralClient) ChatComplete(ctx context.Context, req *domain.ChatRequest) (*domain.ChatResponse, error) {
	url := mistralAPIURL + "/chat/completions"
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
		return nil, fmt.Errorf("Mistral API error: %s", string(bodyBytes))
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
			PromptTokens     int32 `json:"prompt_tokens"`
			CompletionTokens int32 `json:"completion_tokens"`
			TotalTokens      int32 `json:"total_tokens"`
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

// Embed generates embeddings
func (c *MistralClient) Embed(ctx context.Context, model string, texts []string, dimensions *int32) ([][]float32, int64, error) {
	url := mistralAPIURL + "/embeddings"

	if model == "" {
		model = "mistral-embed"
	}

	body := map[string]any{
		"model": model,
		"input": texts,
	}

	jsonBody, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, 0, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, 0, fmt.Errorf("Mistral API error: %s", string(bodyBytes))
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
		Usage struct {
			TotalTokens int64 `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, 0, err
	}

	embeddings := make([][]float32, len(result.Data))
	for i, d := range result.Data {
		embeddings[i] = d.Embedding
	}

	return embeddings, result.Usage.TotalTokens, nil
}

// CountTokens counts tokens in a request
func (c *MistralClient) CountTokens(ctx context.Context, req *domain.ChatRequest) (int32, error) {
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
func (c *MistralClient) ListModels(ctx context.Context) ([]domain.ModelInfo, error) {
	return []domain.ModelInfo{
		{
			ID:              "mistral-large-latest",
			Name:            "Mistral Large",
			Provider:        domain.ProviderMistral,
			SupportsTools:   true,
			ContextLimit:    128000,
			OutputLimit:     8192,
			InputCostPer1M:  2.0,
			OutputCostPer1M: 6.0,
			Enabled:         true,
		},
		{
			ID:              "pixtral-large-latest",
			Name:            "Pixtral Large (Vision)",
			Provider:        domain.ProviderMistral,
			SupportsTools:   true,
			ContextLimit:    128000,
			OutputLimit:     8192,
			InputCostPer1M:  2.0,
			OutputCostPer1M: 6.0,
			Enabled:         true,
		},
		{
			ID:              "mistral-small-latest",
			Name:            "Mistral Small",
			Provider:        domain.ProviderMistral,
			SupportsTools:   true,
			ContextLimit:    32000,
			OutputLimit:     8192,
			InputCostPer1M:  0.1,
			OutputCostPer1M: 0.3,
			Enabled:         true,
		},
		{
			ID:              "ministral-8b-latest",
			Name:            "Ministral 8B",
			Provider:        domain.ProviderMistral,
			SupportsTools:   true,
			ContextLimit:    128000,
			OutputLimit:     8192,
			InputCostPer1M:  0.1,
			OutputCostPer1M: 0.1,
			Enabled:         true,
		},
		{
			ID:              "ministral-3b-latest",
			Name:            "Ministral 3B",
			Provider:        domain.ProviderMistral,
			SupportsTools:   true,
			ContextLimit:    128000,
			OutputLimit:     8192,
			InputCostPer1M:  0.04,
			OutputCostPer1M: 0.04,
			Enabled:         true,
		},
		{
			ID:              "codestral-latest",
			Name:            "Codestral",
			Provider:        domain.ProviderMistral,
			SupportsTools:   false,
			ContextLimit:    32000,
			OutputLimit:     8192,
			InputCostPer1M:  0.3,
			OutputCostPer1M: 0.9,
			Enabled:         true,
		},
		{
			ID:             "mistral-embed",
			Name:           "Mistral Embed",
			Provider:       domain.ProviderMistral,
			SupportsTools:  false,
			ContextLimit:   8192,
			InputCostPer1M: 0.1,
			Enabled:        true,
		},
	}, nil
}

// Helper methods

func (c *MistralClient) buildMessages(req *domain.ChatRequest) []map[string]any {
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
			content := make([]map[string]any, 0)
			for _, block := range msg.Content {
				switch block.Type {
				case "text":
					content = append(content, map[string]any{
						"type": "text",
						"text": block.Text,
					})
				case "image":
					content = append(content, map[string]any{
						"type": "image_url",
						"image_url": map[string]any{
							"url": block.ImageURL,
						},
					})
				}
			}
			m["content"] = content
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

func (c *MistralClient) convertTools(tools []domain.Tool) []map[string]any {
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

func (c *MistralClient) processSSEStream(body io.Reader, events chan<- domain.StreamEvent) {
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
						Function struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Usage struct {
				PromptTokens     int32 `json:"prompt_tokens"`
				CompletionTokens int32 `json:"completion_tokens"`
			} `json:"usage"`
		}

		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		if chunk.Usage.PromptTokens > 0 {
			inputTokens = chunk.Usage.PromptTokens
			outputTokens = chunk.Usage.CompletionTokens
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
