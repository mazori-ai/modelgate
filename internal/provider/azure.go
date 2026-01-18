// Package provider implements LLM provider clients.
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"modelgate/internal/domain"
)

// AzureOpenAIClient implements the LLMClient interface for Azure OpenAI
// Compatible with LLM Gateway Azure integration
// See: https://docs.llmgateway.io/integrations/azure
type AzureOpenAIClient struct {
	apiKey       string
	resourceName string // Resource name (e.g., "my-openai-resource")
	endpoint     string // Full endpoint URL
	apiVersion   string
	deployment   string
	httpClient   *http.Client
	modelCache   map[string]string // Cache of model aliases to native model IDs
}

// AzureOpenAIConfig holds Azure OpenAI configuration
// Supports both resource name and full endpoint URL configuration
type AzureOpenAIConfig struct {
	APIKey             string
	ResourceName       string // Resource name (like LLM Gateway) - preferred
	Endpoint           string // Full endpoint URL (e.g., https://your-resource.openai.azure.com)
	APIVersion         string // e.g., 2024-02-15-preview
	Deployment         string // Default deployment name
	ConnectionSettings domain.ConnectionSettings
}

// NewAzureOpenAIClient creates a new Azure OpenAI client
func NewAzureOpenAIClient(cfg AzureOpenAIConfig) (*AzureOpenAIClient, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("Azure OpenAI API key is required")
	}

	// Determine endpoint and resource name
	var endpoint, resourceName string

	if cfg.ResourceName != "" {
		// LLM Gateway style: resource name provided
		resourceName = cfg.ResourceName
		endpoint = fmt.Sprintf("https://%s.openai.azure.com", resourceName)
	} else if cfg.Endpoint != "" {
		// Legacy: full endpoint URL provided
		endpoint = strings.TrimSuffix(cfg.Endpoint, "/")
		// Extract resource name from endpoint
		// e.g., https://my-openai-resource.openai.azure.com -> my-openai-resource
		if strings.Contains(endpoint, ".openai.azure.com") {
			parts := strings.Split(strings.TrimPrefix(endpoint, "https://"), ".")
			if len(parts) > 0 {
				resourceName = parts[0]
			}
		}
	} else {
		return nil, fmt.Errorf("Azure OpenAI resource name or endpoint is required")
	}

	apiVersion := cfg.APIVersion
	if apiVersion == "" {
		apiVersion = "2024-08-01-preview" // Updated to latest version
	}

	// Use provided connection settings or defaults
	connSettings := cfg.ConnectionSettings
	if connSettings.MaxConnections == 0 {
		connSettings = domain.DefaultConnectionSettings()
	}

	return &AzureOpenAIClient{
		apiKey:       cfg.APIKey,
		resourceName: resourceName,
		endpoint:     endpoint,
		apiVersion:   apiVersion,
		deployment:   cfg.Deployment,
		httpClient:   BuildHTTPClient(connSettings),
		modelCache:   make(map[string]string),
	}, nil
}

// SetModelCache sets the model cache (implements ModelCacheable)
func (c *AzureOpenAIClient) SetModelCache(cache map[string]string) {
	c.modelCache = cache
}

// GetModelCache returns the model cache (implements ModelCacheable)
func (c *AzureOpenAIClient) GetModelCache() map[string]string {
	return c.modelCache
}

// Provider returns the provider type
func (c *AzureOpenAIClient) Provider() domain.Provider {
	return domain.ProviderAzureOpenAI
}

// SupportsModel checks if a model is supported
func (c *AzureOpenAIClient) SupportsModel(model string) bool {
	// Support both naming conventions:
	// - azure/gpt-4o (LLM Gateway style)
	// - gpt-4o (direct deployment name)
	return strings.HasPrefix(model, "azure/") || !strings.Contains(model, "/")
}

// mapModelToDeployment converts LLM Gateway style model names to Azure deployment names
func (c *AzureOpenAIClient) mapModelToDeployment(model string) string {
	// Strip azure/ prefix if present
	deployment := model
	if strings.HasPrefix(model, "azure/") {
		deployment = strings.TrimPrefix(model, "azure/")
	}

	// Map common model names to Azure deployment names
	// Note: gpt-3.5-turbo in LLM Gateway maps to gpt-35-turbo in Azure
	modelMappings := map[string]string{
		"gpt-3.5-turbo": "gpt-35-turbo",
	}

	if mapped, ok := modelMappings[deployment]; ok {
		return mapped
	}

	return deployment
}

// ChatStream performs streaming chat completion
func (c *AzureOpenAIClient) ChatStream(ctx context.Context, req *domain.ChatRequest) (<-chan domain.StreamEvent, error) {
	events := make(chan domain.StreamEvent, 100)

	go func() {
		defer close(events)

		deployment := c.deployment
		if deployment == "" {
			deployment = c.mapModelToDeployment(req.Model)
		}

		url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s",
			c.endpoint, deployment, c.apiVersion)

		// Build messages
		messages := c.buildMessages(req)

		body := map[string]any{
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
		httpReq.Header.Set("api-key", c.apiKey)

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			events <- domain.FinishEvent{Reason: domain.FinishReasonError}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			events <- domain.TextChunk{Content: fmt.Sprintf("Azure OpenAI error: %s", string(bodyBytes))}
			events <- domain.FinishEvent{Reason: domain.FinishReasonError}
			return
		}

		c.processSSEStream(resp.Body, events)
	}()

	return events, nil
}

// ChatComplete performs non-streaming chat completion
func (c *AzureOpenAIClient) ChatComplete(ctx context.Context, req *domain.ChatRequest) (*domain.ChatResponse, error) {
	deployment := c.deployment
	if deployment == "" {
		deployment = c.mapModelToDeployment(req.Model)
	}

	url := fmt.Sprintf("%s/openai/deployments/%s/chat/completions?api-version=%s",
		c.endpoint, deployment, c.apiVersion)

	messages := c.buildMessages(req)

	body := map[string]any{
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
	httpReq.Header.Set("api-key", c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Azure OpenAI API error: %s", string(bodyBytes))
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
func (c *AzureOpenAIClient) Embed(ctx context.Context, model string, texts []string, dimensions *int32) ([][]float32, int64, error) {
	deployment := c.deployment
	if deployment == "" {
		deployment = c.mapModelToDeployment(model)
	}

	url := fmt.Sprintf("%s/openai/deployments/%s/embeddings?api-version=%s",
		c.endpoint, deployment, c.apiVersion)

	body := map[string]any{
		"input": texts,
	}
	if dimensions != nil {
		body["dimensions"] = *dimensions
	}

	jsonBody, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(jsonBody)))
	if err != nil {
		return nil, 0, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("api-key", c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, 0, fmt.Errorf("Azure OpenAI API error: %s", string(bodyBytes))
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
func (c *AzureOpenAIClient) CountTokens(ctx context.Context, req *domain.ChatRequest) (int32, error) {
	// Approximate token count
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
// Uses LLM Gateway style naming with azure/ prefix
// See: https://docs.llmgateway.io/integrations/azure
func (c *AzureOpenAIClient) ListModels(ctx context.Context) ([]domain.ModelInfo, error) {
	// Azure OpenAI deployments - using LLM Gateway naming convention
	return []domain.ModelInfo{
		{
			ID:              "azure/gpt-4o",
			Name:            "GPT-4o",
			Provider:        domain.ProviderAzureOpenAI,
			SupportsTools:   true,
			ContextLimit:    128000,
			OutputLimit:     16384,
			InputCostPer1M:  2.5,
			OutputCostPer1M: 10.0,
			Enabled:         true,
		},
		{
			ID:              "azure/gpt-4o-mini",
			Name:            "GPT-4o Mini",
			Provider:        domain.ProviderAzureOpenAI,
			SupportsTools:   true,
			ContextLimit:    128000,
			OutputLimit:     16384,
			InputCostPer1M:  0.15,
			OutputCostPer1M: 0.6,
			Enabled:         true,
		},
		{
			ID:              "azure/gpt-4-turbo",
			Name:            "GPT-4 Turbo",
			Provider:        domain.ProviderAzureOpenAI,
			SupportsTools:   true,
			ContextLimit:    128000,
			OutputLimit:     4096,
			InputCostPer1M:  10.0,
			OutputCostPer1M: 30.0,
			Enabled:         true,
		},
		{
			ID:              "azure/gpt-4",
			Name:            "GPT-4",
			Provider:        domain.ProviderAzureOpenAI,
			SupportsTools:   true,
			ContextLimit:    8192,
			OutputLimit:     4096,
			InputCostPer1M:  30.0,
			OutputCostPer1M: 60.0,
			Enabled:         true,
		},
		{
			ID:              "azure/gpt-3.5-turbo",
			Name:            "GPT-3.5 Turbo",
			Provider:        domain.ProviderAzureOpenAI,
			SupportsTools:   true,
			ContextLimit:    16384,
			OutputLimit:     4096,
			InputCostPer1M:  0.5,
			OutputCostPer1M: 1.5,
			Enabled:         true,
		},
		{
			ID:             "azure/text-embedding-3-large",
			Name:           "Text Embedding 3 Large",
			Provider:       domain.ProviderAzureOpenAI,
			SupportsTools:  false,
			ContextLimit:   8191,
			InputCostPer1M: 0.13,
			Enabled:        true,
		},
		{
			ID:             "azure/text-embedding-3-small",
			Name:           "Text Embedding 3 Small",
			Provider:       domain.ProviderAzureOpenAI,
			SupportsTools:  false,
			ContextLimit:   8191,
			InputCostPer1M: 0.02,
			Enabled:        true,
		},
	}, nil
}

// Helper methods

func (c *AzureOpenAIClient) buildMessages(req *domain.ChatRequest) []map[string]any {
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

func (c *AzureOpenAIClient) convertTools(tools []domain.Tool) []map[string]any {
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

func (c *AzureOpenAIClient) processSSEStream(body io.Reader, events chan<- domain.StreamEvent) {
	reader := NewSSEReader(body)
	var inputTokens, outputTokens int32

	for {
		event, err := reader.ReadEvent()
		if err == io.EOF {
			break
		}
		if err != nil {
			events <- domain.FinishEvent{Reason: domain.FinishReasonError}
			return
		}

		if event.Data == "[DONE]" {
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
			Usage struct {
				PromptTokens     int32 `json:"prompt_tokens"`
				CompletionTokens int32 `json:"completion_tokens"`
			} `json:"usage"`
		}

		if err := json.Unmarshal([]byte(event.Data), &chunk); err != nil {
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
				} else if tc.Function.Arguments != "" {
					events <- domain.ToolCallDelta{Delta: tc.Function.Arguments}
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
