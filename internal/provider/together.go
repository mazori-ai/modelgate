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

const togetherAPIURL = "https://api.together.xyz/v1"

// TogetherClient implements the LLMClient interface for Together AI
type TogetherClient struct {
	apiKey     string
	httpClient *http.Client
	modelCache map[string]string // Cache of model aliases to native model IDs
}

// NewTogetherClient creates a new Together AI client
func NewTogetherClient(apiKey string, settings ...domain.ConnectionSettings) (*TogetherClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("Together AI API key is required")
	}

	// Use provided settings or defaults
	connSettings := domain.DefaultConnectionSettings()
	if len(settings) > 0 {
		connSettings = settings[0]
	}

	return &TogetherClient{
		apiKey:     apiKey,
		httpClient: BuildHTTPClient(connSettings),
		modelCache: make(map[string]string),
	}, nil
}

// SetModelCache sets the model cache (implements ModelCacheable)
func (c *TogetherClient) SetModelCache(cache map[string]string) {
	c.modelCache = cache
}

// GetModelCache returns the model cache (implements ModelCacheable)
func (c *TogetherClient) GetModelCache() map[string]string {
	return c.modelCache
}

// resolveModelID resolves a model ID using the cache if available
func (c *TogetherClient) resolveModelID(model string) string {
	if c.modelCache != nil {
		if nativeID, ok := c.modelCache[model]; ok {
			return ExtractModelID(nativeID)
		}
	}
	return ExtractModelID(model)
}

// Provider returns the provider type
func (c *TogetherClient) Provider() domain.Provider {
	return domain.ProviderTogether
}

// SupportsModel checks if a model is supported
func (c *TogetherClient) SupportsModel(model string) bool {
	// Together AI supports many open-source models
	return strings.Contains(model, "/") || strings.HasPrefix(model, "meta-llama") ||
		strings.HasPrefix(model, "mistralai") || strings.HasPrefix(model, "Qwen")
}

// ChatStream performs streaming chat completion
func (c *TogetherClient) ChatStream(ctx context.Context, req *domain.ChatRequest) (<-chan domain.StreamEvent, error) {
	events := make(chan domain.StreamEvent, 100)

	go func() {
		defer close(events)

		url := togetherAPIURL + "/chat/completions"
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
			events <- domain.TextChunk{Content: fmt.Sprintf("Together AI error: %s", string(bodyBytes))}
			events <- domain.FinishEvent{Reason: domain.FinishReasonError}
			return
		}

		c.processSSEStream(resp.Body, events)
	}()

	return events, nil
}

// ChatComplete performs non-streaming chat completion
func (c *TogetherClient) ChatComplete(ctx context.Context, req *domain.ChatRequest) (*domain.ChatResponse, error) {
	url := togetherAPIURL + "/chat/completions"
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
		return nil, fmt.Errorf("Together AI API error: %s", string(bodyBytes))
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
func (c *TogetherClient) Embed(ctx context.Context, model string, texts []string, dimensions *int32) ([][]float32, int64, error) {
	url := togetherAPIURL + "/embeddings"

	if model == "" {
		model = "togethercomputer/m2-bert-80M-8k-retrieval"
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
		return nil, 0, fmt.Errorf("Together AI API error: %s", string(bodyBytes))
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
func (c *TogetherClient) CountTokens(ctx context.Context, req *domain.ChatRequest) (int32, error) {
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
func (c *TogetherClient) ListModels(ctx context.Context) ([]domain.ModelInfo, error) {
	return []domain.ModelInfo{
		{
			ID:              "meta-llama/Llama-3.3-70B-Instruct-Turbo",
			Name:            "Llama 3.3 70B Instruct Turbo",
			Provider:        domain.ProviderTogether,
			SupportsTools:   true,
			ContextLimit:    128000,
			InputCostPer1M:  0.88,
			OutputCostPer1M: 0.88,
			Enabled:         true,
		},
		{
			ID:              "meta-llama/Meta-Llama-3.1-405B-Instruct-Turbo",
			Name:            "Llama 3.1 405B Instruct Turbo",
			Provider:        domain.ProviderTogether,
			SupportsTools:   true,
			ContextLimit:    128000,
			InputCostPer1M:  3.5,
			OutputCostPer1M: 3.5,
			Enabled:         true,
		},
		{
			ID:              "meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo",
			Name:            "Llama 3.1 70B Instruct Turbo",
			Provider:        domain.ProviderTogether,
			SupportsTools:   true,
			ContextLimit:    128000,
			InputCostPer1M:  0.88,
			OutputCostPer1M: 0.88,
			Enabled:         true,
		},
		{
			ID:              "meta-llama/Meta-Llama-3.1-8B-Instruct-Turbo",
			Name:            "Llama 3.1 8B Instruct Turbo",
			Provider:        domain.ProviderTogether,
			SupportsTools:   true,
			ContextLimit:    128000,
			InputCostPer1M:  0.18,
			OutputCostPer1M: 0.18,
			Enabled:         true,
		},
		{
			ID:              "mistralai/Mixtral-8x22B-Instruct-v0.1",
			Name:            "Mixtral 8x22B Instruct",
			Provider:        domain.ProviderTogether,
			SupportsTools:   true,
			ContextLimit:    65536,
			InputCostPer1M:  1.2,
			OutputCostPer1M: 1.2,
			Enabled:         true,
		},
		{
			ID:              "Qwen/Qwen2.5-72B-Instruct-Turbo",
			Name:            "Qwen 2.5 72B Instruct Turbo",
			Provider:        domain.ProviderTogether,
			SupportsTools:   true,
			ContextLimit:    128000,
			InputCostPer1M:  1.2,
			OutputCostPer1M: 1.2,
			Enabled:         true,
		},
		{
			ID:              "deepseek-ai/DeepSeek-V3",
			Name:            "DeepSeek V3",
			Provider:        domain.ProviderTogether,
			SupportsTools:   true,
			ContextLimit:    128000,
			InputCostPer1M:  0.9,
			OutputCostPer1M: 0.9,
			Enabled:         true,
		},
	}, nil
}

// Helper methods

func (c *TogetherClient) buildMessages(req *domain.ChatRequest) []map[string]any {
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

func (c *TogetherClient) convertTools(tools []domain.Tool) []map[string]any {
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

func (c *TogetherClient) processSSEStream(body io.Reader, events chan<- domain.StreamEvent) {
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
					Content string `json:"content"`
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
