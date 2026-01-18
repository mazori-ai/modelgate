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

// OllamaClient is a client for Ollama API
type OllamaClient struct {
	httpClient *http.Client
	baseURL    string
	modelCache map[string]string // Cache of model aliases to native model IDs
}

// NewOllamaClient creates a new Ollama client
func NewOllamaClient(baseURL string, settings ...domain.ConnectionSettings) (*OllamaClient, error) {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	// Use provided settings or defaults
	connSettings := domain.DefaultConnectionSettings()
	if len(settings) > 0 {
		connSettings = settings[0]
	}

	return &OllamaClient{
		httpClient: BuildHTTPClient(connSettings),
		baseURL:    baseURL,
		modelCache: make(map[string]string),
	}, nil
}

// SetModelCache sets the model cache (implements ModelCacheable)
func (c *OllamaClient) SetModelCache(cache map[string]string) {
	c.modelCache = cache
}

// GetModelCache returns the model cache (implements ModelCacheable)
func (c *OllamaClient) GetModelCache() map[string]string {
	return c.modelCache
}

// resolveModelID resolves a model ID using the cache if available
func (c *OllamaClient) resolveModelID(model string) string {
	if c.modelCache != nil {
		if nativeID, ok := c.modelCache[model]; ok {
			return ExtractModelID(nativeID)
		}
	}
	return ExtractModelID(model)
}

// Provider returns the provider type
func (c *OllamaClient) Provider() domain.Provider {
	return domain.ProviderOllama
}

// SupportsModel checks if a model is supported
func (c *OllamaClient) SupportsModel(model string) bool {
	// Ollama supports any model that's been pulled
	return true
}

// ChatStream starts a streaming chat completion
func (c *OllamaClient) ChatStream(ctx context.Context, req *domain.ChatRequest) (<-chan domain.StreamEvent, error) {
	eventChan := make(chan domain.StreamEvent, 100)

	go func() {
		defer close(eventChan)

		url := c.baseURL + "/api/chat"
		ollamaReq := c.buildRequest(req)
		ollamaReq["stream"] = true

		body, err := json.Marshal(ollamaReq)
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

		c.parseStream(resp.Body, eventChan)
	}()

	return eventChan, nil
}

// ChatComplete performs a non-streaming chat completion
func (c *OllamaClient) ChatComplete(ctx context.Context, req *domain.ChatRequest) (*domain.ChatResponse, error) {
	url := c.baseURL + "/api/chat"
	ollamaReq := c.buildRequest(req)
	ollamaReq["stream"] = false

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")

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
		Message struct {
			Role      string `json:"role"`
			Content   string `json:"content"`
			ToolCalls []struct {
				Function struct {
					Name      string         `json:"name"`
					Arguments map[string]any `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
		Done            bool  `json:"done"`
		PromptEvalCount int32 `json:"prompt_eval_count"`
		EvalCount       int32 `json:"eval_count"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	response := &domain.ChatResponse{
		Content: result.Message.Content,
		Model:   req.Model,
		Usage: &domain.UsageEvent{
			PromptTokens:     result.PromptEvalCount,
			CompletionTokens: result.EvalCount,
			TotalTokens:      result.PromptEvalCount + result.EvalCount,
		},
		FinishReason: domain.FinishReasonStop,
	}

	// Handle tool calls
	for i, tc := range result.Message.ToolCalls {
		response.ToolCalls = append(response.ToolCalls, domain.ToolCall{
			ID:   fmt.Sprintf("call_%d", i),
			Type: "function",
			Function: domain.FunctionCall{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		})
	}

	if len(response.ToolCalls) > 0 {
		response.FinishReason = domain.FinishReasonToolCalls
	}

	return response, nil
}

// Embed generates embeddings
func (c *OllamaClient) Embed(ctx context.Context, model string, texts []string, dimensions *int32) ([][]float32, int64, error) {
	url := c.baseURL + "/api/embeddings"

	modelID := ExtractModelID(model)
	if modelID == "" {
		modelID = "nomic-embed-text"
	}

	var embeddings [][]float32
	var totalTokens int64

	for _, text := range texts {
		embedReq := map[string]any{
			"model":  modelID,
			"prompt": text,
		}

		body, _ := json.Marshal(embedReq)

		httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
		if err != nil {
			return nil, 0, err
		}

		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			return nil, 0, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return nil, 0, fmt.Errorf("API error: %s - %s", resp.Status, string(bodyBytes))
		}

		var result struct {
			Embedding []float32 `json:"embedding"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, 0, err
		}

		embeddings = append(embeddings, result.Embedding)
		totalTokens += int64(len(text) / 4) // Rough estimate
	}

	return embeddings, totalTokens, nil
}

// CountTokens counts tokens in a request
func (c *OllamaClient) CountTokens(ctx context.Context, req *domain.ChatRequest) (int32, error) {
	// Ollama doesn't have a token counting endpoint
	var totalChars int
	for _, msg := range req.Messages {
		for _, content := range msg.Content {
			totalChars += len(content.Text)
		}
	}
	totalChars += len(req.Prompt)
	totalChars += len(req.SystemPrompt)

	return int32(totalChars / 4), nil
}

// ListModels lists available models
func (c *OllamaClient) ListModels(ctx context.Context) ([]domain.ModelInfo, error) {
	url := c.baseURL + "/api/tags"

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Models []struct {
			Name       string `json:"name"`
			ModifiedAt string `json:"modified_at"`
			Size       int64  `json:"size"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var models []domain.ModelInfo
	for _, m := range result.Models {
		models = append(models, domain.ModelInfo{
			ID:            fmt.Sprintf("ollama/%s", m.Name),
			Name:          m.Name,
			Provider:      domain.ProviderOllama,
			SupportsTools: true, // Modern Ollama models support tools
			Enabled:       true,
		})
	}

	return models, nil
}

// buildRequest builds an Ollama API request
func (c *OllamaClient) buildRequest(req *domain.ChatRequest) map[string]any {
	ollamaReq := map[string]any{
		"model": ExtractModelID(req.Model),
	}

	// Build messages
	var messages []map[string]any

	if req.SystemPrompt != "" {
		messages = append(messages, map[string]any{
			"role":    "system",
			"content": req.SystemPrompt,
		})
	}

	for _, msg := range req.Messages {
		ollamaMsg := map[string]any{
			"role": msg.Role,
		}

		// Handle content
		if len(msg.Content) == 1 && msg.Content[0].Type == "text" {
			ollamaMsg["content"] = msg.Content[0].Text
		} else {
			var content strings.Builder
			for _, c := range msg.Content {
				if c.Type == "text" {
					content.WriteString(c.Text)
				}
			}
			ollamaMsg["content"] = content.String()
		}

		// Handle tool calls from assistant
		if len(msg.ToolCalls) > 0 {
			var toolCalls []map[string]any
			for _, tc := range msg.ToolCalls {
				toolCalls = append(toolCalls, map[string]any{
					"function": map[string]any{
						"name":      tc.Function.Name,
						"arguments": tc.Function.Arguments,
					},
				})
			}
			ollamaMsg["tool_calls"] = toolCalls
		}

		// Handle tool result messages (role="tool")
		if msg.ToolCallID != "" {
			ollamaMsg["tool_call_id"] = msg.ToolCallID
		}

		messages = append(messages, ollamaMsg)
	}

	// Add current prompt
	if req.Prompt != "" {
		messages = append(messages, map[string]any{
			"role":    "user",
			"content": req.Prompt,
		})
	}

	ollamaReq["messages"] = messages

	// Add tools
	if len(req.Tools) > 0 {
		var tools []map[string]any
		for _, tool := range req.Tools {
			tools = append(tools, map[string]any{
				"type": "function",
				"function": map[string]any{
					"name":        tool.Function.Name,
					"description": tool.Function.Description,
					"parameters":  tool.Function.Parameters,
				},
			})
		}
		ollamaReq["tools"] = tools
	}

	// Options
	options := map[string]any{}
	if req.Temperature != nil {
		options["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		options["num_predict"] = *req.MaxTokens
	}
	if len(options) > 0 {
		ollamaReq["options"] = options
	}

	return ollamaReq
}

// parseStream parses the NDJSON stream from Ollama
func (c *OllamaClient) parseStream(body io.Reader, eventChan chan<- domain.StreamEvent) {
	decoder := json.NewDecoder(body)

	for {
		var chunk struct {
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			Done            bool  `json:"done"`
			PromptEvalCount int32 `json:"prompt_eval_count"`
			EvalCount       int32 `json:"eval_count"`
		}

		if err := decoder.Decode(&chunk); err != nil {
			if err != io.EOF {
				eventChan <- domain.FinishEvent{Reason: domain.FinishReasonError}
			}
			return
		}

		if chunk.Message.Content != "" {
			eventChan <- domain.TextChunk{Content: chunk.Message.Content}
		}

		if chunk.Done {
			// Send usage
			if chunk.PromptEvalCount > 0 || chunk.EvalCount > 0 {
				eventChan <- domain.UsageEvent{
					PromptTokens:     chunk.PromptEvalCount,
					CompletionTokens: chunk.EvalCount,
					TotalTokens:      chunk.PromptEvalCount + chunk.EvalCount,
				}
			}
			eventChan <- domain.FinishEvent{Reason: domain.FinishReasonStop}
			return
		}
	}
}
