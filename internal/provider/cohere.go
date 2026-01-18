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

const cohereAPIURL = "https://api.cohere.com/v2"

// CohereClient implements the LLMClient interface for Cohere
type CohereClient struct {
	apiKey     string
	httpClient *http.Client
	modelCache map[string]string // Cache of model aliases to native model IDs
}

// NewCohereClient creates a new Cohere client
func NewCohereClient(apiKey string, settings ...domain.ConnectionSettings) (*CohereClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("Cohere API key is required")
	}

	// Use provided settings or defaults
	connSettings := domain.DefaultConnectionSettings()
	if len(settings) > 0 {
		connSettings = settings[0]
	}

	return &CohereClient{
		apiKey:     apiKey,
		httpClient: BuildHTTPClient(connSettings),
		modelCache: make(map[string]string),
	}, nil
}

// SetModelCache sets the model cache (implements ModelCacheable)
func (c *CohereClient) SetModelCache(cache map[string]string) {
	c.modelCache = cache
}

// GetModelCache returns the model cache (implements ModelCacheable)
func (c *CohereClient) GetModelCache() map[string]string {
	return c.modelCache
}

// resolveModelID resolves a model ID using the cache if available
func (c *CohereClient) resolveModelID(model string) string {
	if c.modelCache != nil {
		if nativeID, ok := c.modelCache[model]; ok {
			return ExtractModelID(nativeID)
		}
	}
	return ExtractModelID(model)
}

// Provider returns the provider type
func (c *CohereClient) Provider() domain.Provider {
	return domain.ProviderCohere
}

// SupportsModel checks if a model is supported
func (c *CohereClient) SupportsModel(model string) bool {
	cohereModels := []string{
		"command-r-plus",
		"command-r",
		"command-light",
		"command",
		"embed-english-v3.0",
		"embed-multilingual-v3.0",
		"rerank-english-v3.0",
		"rerank-multilingual-v3.0",
	}
	for _, m := range cohereModels {
		if strings.EqualFold(model, m) || strings.Contains(strings.ToLower(model), strings.ToLower(m)) {
			return true
		}
	}
	return false
}

// ChatStream performs streaming chat completion
func (c *CohereClient) ChatStream(ctx context.Context, req *domain.ChatRequest) (<-chan domain.StreamEvent, error) {
	events := make(chan domain.StreamEvent, 100)

	go func() {
		defer close(events)

		url := cohereAPIURL + "/chat"
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
			events <- domain.TextChunk{Content: fmt.Sprintf("Cohere error: %s", string(bodyBytes))}
			events <- domain.FinishEvent{Reason: domain.FinishReasonError}
			return
		}

		c.processSSEStream(resp.Body, events)
	}()

	return events, nil
}

// ChatComplete performs non-streaming chat completion
func (c *CohereClient) ChatComplete(ctx context.Context, req *domain.ChatRequest) (*domain.ChatResponse, error) {
	url := cohereAPIURL + "/chat"
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
		return nil, fmt.Errorf("Cohere API error: %s", string(bodyBytes))
	}

	var result struct {
		Message struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string         `json:"name"`
					Arguments map[string]any `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
		Usage        struct {
			BilledUnits struct {
				InputTokens  int32 `json:"input_tokens"`
				OutputTokens int32 `json:"output_tokens"`
			} `json:"billed_units"`
			Tokens struct {
				InputTokens  int32 `json:"input_tokens"`
				OutputTokens int32 `json:"output_tokens"`
			} `json:"tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	response := &domain.ChatResponse{
		Model:        req.Model,
		FinishReason: domain.FinishReason(result.FinishReason),
		Usage: &domain.UsageEvent{
			PromptTokens:     result.Usage.Tokens.InputTokens,
			CompletionTokens: result.Usage.Tokens.OutputTokens,
			TotalTokens:      result.Usage.Tokens.InputTokens + result.Usage.Tokens.OutputTokens,
		},
	}

	// Extract text content
	for _, content := range result.Message.Content {
		if content.Type == "text" {
			response.Content += content.Text
		}
	}

	// Extract tool calls
	for _, tc := range result.Message.ToolCalls {
		response.ToolCalls = append(response.ToolCalls, domain.ToolCall{
			ID:   tc.ID,
			Type: tc.Type,
			Function: domain.FunctionCall{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		})
	}

	return response, nil
}

// Embed generates embeddings
func (c *CohereClient) Embed(ctx context.Context, model string, texts []string, dimensions *int32) ([][]float32, int64, error) {
	url := cohereAPIURL + "/embed"

	if model == "" {
		model = "embed-english-v3.0"
	}

	body := map[string]any{
		"model":      model,
		"texts":      texts,
		"input_type": "search_document",
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
		return nil, 0, fmt.Errorf("Cohere API error: %s", string(bodyBytes))
	}

	var result struct {
		Embeddings [][]float32 `json:"embeddings"`
		Meta       struct {
			BilledUnits struct {
				InputTokens int64 `json:"input_tokens"`
			} `json:"billed_units"`
		} `json:"meta"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, 0, err
	}

	return result.Embeddings, result.Meta.BilledUnits.InputTokens, nil
}

// CountTokens counts tokens in a request
func (c *CohereClient) CountTokens(ctx context.Context, req *domain.ChatRequest) (int32, error) {
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
func (c *CohereClient) ListModels(ctx context.Context) ([]domain.ModelInfo, error) {
	return []domain.ModelInfo{
		{
			ID:              "command-r-plus",
			Name:            "Command R+",
			Provider:        domain.ProviderCohere,
			SupportsTools:   true,
			ContextLimit:    128000,
			OutputLimit:     4096,
			InputCostPer1M:  2.5,
			OutputCostPer1M: 10.0,
			Enabled:         true,
		},
		{
			ID:              "command-r",
			Name:            "Command R",
			Provider:        domain.ProviderCohere,
			SupportsTools:   true,
			ContextLimit:    128000,
			OutputLimit:     4096,
			InputCostPer1M:  0.15,
			OutputCostPer1M: 0.6,
			Enabled:         true,
		},
		{
			ID:              "command",
			Name:            "Command",
			Provider:        domain.ProviderCohere,
			SupportsTools:   false,
			ContextLimit:    4096,
			OutputLimit:     4096,
			InputCostPer1M:  1.0,
			OutputCostPer1M: 2.0,
			Enabled:         true,
		},
		{
			ID:              "command-light",
			Name:            "Command Light",
			Provider:        domain.ProviderCohere,
			SupportsTools:   false,
			ContextLimit:    4096,
			OutputLimit:     4096,
			InputCostPer1M:  0.3,
			OutputCostPer1M: 0.6,
			Enabled:         true,
		},
		{
			ID:             "embed-english-v3.0",
			Name:           "Embed English v3.0",
			Provider:       domain.ProviderCohere,
			SupportsTools:  false,
			ContextLimit:   512,
			InputCostPer1M: 0.1,
			Enabled:        true,
		},
		{
			ID:             "embed-multilingual-v3.0",
			Name:           "Embed Multilingual v3.0",
			Provider:       domain.ProviderCohere,
			SupportsTools:  false,
			ContextLimit:   512,
			InputCostPer1M: 0.1,
			Enabled:        true,
		},
	}, nil
}

// Helper methods

func (c *CohereClient) buildMessages(req *domain.ChatRequest) []map[string]any {
	messages := make([]map[string]any, 0)

	if req.SystemPrompt != "" {
		messages = append(messages, map[string]any{
			"role":    "system",
			"content": req.SystemPrompt,
		})
	}

	for _, msg := range req.Messages {
		m := map[string]any{"role": msg.Role}

		// Cohere expects content as string or array
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
				toolCalls[i] = map[string]any{
					"id":   tc.ID,
					"type": "function",
					"function": map[string]any{
						"name":      tc.Function.Name,
						"arguments": tc.Function.Arguments,
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

func (c *CohereClient) convertTools(tools []domain.Tool) []map[string]any {
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

func (c *CohereClient) processSSEStream(body io.Reader, events chan<- domain.StreamEvent) {
	scanner := bufio.NewScanner(body)
	var inputTokens, outputTokens int32

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		var event struct {
			Type  string `json:"type"`
			Delta struct {
				Message struct {
					Content struct {
						Text string `json:"text"`
					} `json:"content"`
				} `json:"message"`
			} `json:"delta"`
			Usage struct {
				Tokens struct {
					InputTokens  int32 `json:"input_tokens"`
					OutputTokens int32 `json:"output_tokens"`
				} `json:"tokens"`
			} `json:"usage"`
		}

		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "content-delta":
			if event.Delta.Message.Content.Text != "" {
				events <- domain.TextChunk{Content: event.Delta.Message.Content.Text}
			}
		case "message-end":
			inputTokens = event.Usage.Tokens.InputTokens
			outputTokens = event.Usage.Tokens.OutputTokens
			events <- domain.UsageEvent{
				PromptTokens:     inputTokens,
				CompletionTokens: outputTokens,
				TotalTokens:      inputTokens + outputTokens,
			}
			events <- domain.FinishEvent{Reason: domain.FinishReasonStop}
			return
		}
	}
}
