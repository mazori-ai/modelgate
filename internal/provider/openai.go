// Package provider implements LLM provider clients.
package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"modelgate/internal/domain"
)

// OpenAIClient is a client for OpenAI API
type OpenAIClient struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
	modelCache map[string]string // Cache of model aliases to native model IDs
}

// NewOpenAIClient creates a new OpenAI client
func NewOpenAIClient(apiKey, baseURL string, settings ...domain.ConnectionSettings) (*OpenAIClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	// Use provided settings or defaults
	connSettings := domain.DefaultConnectionSettings()
	if len(settings) > 0 {
		connSettings = settings[0]
	}

	return &OpenAIClient{
		apiKey:     apiKey,
		httpClient: BuildHTTPClient(connSettings),
		baseURL:    baseURL,
		modelCache: make(map[string]string),
	}, nil
}

// SetModelCache sets the model cache (implements ModelCacheable)
func (c *OpenAIClient) SetModelCache(cache map[string]string) {
	c.modelCache = cache
}

// GetModelCache returns the model cache (implements ModelCacheable)
func (c *OpenAIClient) GetModelCache() map[string]string {
	return c.modelCache
}

// resolveModelID resolves a model ID using the cache if available
func (c *OpenAIClient) resolveModelID(model string) string {
	// First try the cache
	if c.modelCache != nil {
		if nativeID, ok := c.modelCache[model]; ok {
			return ExtractModelID(nativeID)
		}
	}
	// Fall back to extracting from the model string
	return ExtractModelID(model)
}

// Provider returns the provider type
func (c *OpenAIClient) Provider() domain.Provider {
	return domain.ProviderOpenAI
}

// SupportsModel checks if a model is supported
func (c *OpenAIClient) SupportsModel(model string) bool {
	modelID := strings.ToLower(ExtractModelID(model))
	return strings.HasPrefix(modelID, "gpt") || strings.HasPrefix(modelID, "o1") || strings.HasPrefix(modelID, "text-embedding")
}

// ChatStream starts a streaming chat completion
func (c *OpenAIClient) ChatStream(ctx context.Context, req *domain.ChatRequest) (<-chan domain.StreamEvent, error) {
	eventChan := make(chan domain.StreamEvent, 100)

	go func() {
		defer close(eventChan)

		url := c.baseURL + "/chat/completions"
		openaiReq := c.buildRequest(req)
		openaiReq["stream"] = true
		// Request usage data in streaming responses
		openaiReq["stream_options"] = map[string]any{
			"include_usage": true,
		}

		body, err := json.Marshal(openaiReq)
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
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

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
func (c *OpenAIClient) ChatComplete(ctx context.Context, req *domain.ChatRequest) (*domain.ChatResponse, error) {
	url := c.baseURL + "/chat/completions"
	openaiReq := c.buildRequest(req)
	openaiReq["stream"] = false

	body, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
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
		return nil, fmt.Errorf("API error: %s - %s", resp.Status, string(bodyBytes))
	}

	var result struct {
		ID      string `json:"id"`
		Choices []struct {
			Message struct {
				Role      string `json:"role"`
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
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	response := &domain.ChatResponse{
		Model: req.Model,
		Usage: &domain.UsageEvent{
			PromptTokens:     result.Usage.PromptTokens,
			CompletionTokens: result.Usage.CompletionTokens,
			TotalTokens:      result.Usage.TotalTokens,
		},
	}

	if len(result.Choices) > 0 {
		choice := result.Choices[0]
		response.Content = choice.Message.Content

		for _, tc := range choice.Message.ToolCalls {
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

		switch choice.FinishReason {
		case "stop":
			response.FinishReason = domain.FinishReasonStop
		case "tool_calls":
			response.FinishReason = domain.FinishReasonToolCalls
		case "length":
			response.FinishReason = domain.FinishReasonLength
		default:
			response.FinishReason = domain.FinishReasonStop
		}
	}

	return response, nil
}

// Embed generates embeddings
func (c *OpenAIClient) Embed(ctx context.Context, model string, texts []string, dimensions *int32) ([][]float32, int64, error) {
	url := c.baseURL + "/embeddings"

	modelID := ExtractModelID(model)
	if modelID == "" {
		modelID = "text-embedding-3-small"
	}

	embedReq := map[string]any{
		"model": modelID,
		"input": texts,
	}
	if dimensions != nil {
		embedReq["dimensions"] = *dimensions
	}

	body, _ := json.Marshal(embedReq)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
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
		return nil, 0, fmt.Errorf("API error: %s - %s", resp.Status, string(bodyBytes))
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
		Usage struct {
			TotalTokens int64 `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, 0, err
	}

	embeddings := make([][]float32, len(result.Data))
	for _, d := range result.Data {
		embeddings[d.Index] = d.Embedding
	}

	return embeddings, result.Usage.TotalTokens, nil
}

// CountTokens counts tokens in a request
func (c *OpenAIClient) CountTokens(ctx context.Context, req *domain.ChatRequest) (int32, error) {
	// OpenAI doesn't have a token counting endpoint
	// Use tiktoken estimation
	var totalChars int
	for _, msg := range req.Messages {
		for _, content := range msg.Content {
			totalChars += len(content.Text)
		}
	}
	totalChars += len(req.Prompt)
	totalChars += len(req.SystemPrompt)

	// Rough estimate: 1 token ≈ 4 characters
	return int32(totalChars / 4), nil
}

// ListModels lists available models
func (c *OpenAIClient) ListModels(ctx context.Context) ([]domain.ModelInfo, error) {
	url := c.baseURL + "/models"

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var models []domain.ModelInfo
	for _, m := range result.Data {
		// Filter to chat models
		if strings.HasPrefix(m.ID, "gpt-") || strings.HasPrefix(m.ID, "o1") {
			models = append(models, domain.ModelInfo{
				ID:            fmt.Sprintf("openai/%s", m.ID),
				Name:          m.ID,
				Provider:      domain.ProviderOpenAI,
				SupportsTools: !strings.HasPrefix(m.ID, "o1"),
				Enabled:       true,
			})
		}
	}

	return models, nil
}

// buildRequest builds an OpenAI API request
func (c *OpenAIClient) buildRequest(req *domain.ChatRequest) map[string]any {
	openaiReq := map[string]any{
		"model": ExtractModelID(req.Model),
	}

	if req.MaxTokens != nil {
		openaiReq["max_tokens"] = *req.MaxTokens
	}

	if req.Temperature != nil {
		openaiReq["temperature"] = *req.Temperature
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
		openaiMsg := map[string]any{
			"role": msg.Role,
		}

		// Handle content
		if len(msg.Content) == 1 && msg.Content[0].Type == "text" {
			openaiMsg["content"] = msg.Content[0].Text
		} else {
			var content []map[string]any
			for _, c := range msg.Content {
				switch c.Type {
				case "text":
					content = append(content, map[string]any{
						"type": "text",
						"text": c.Text,
					})
				case "image":
					content = append(content, map[string]any{
						"type": "image_url",
						"image_url": map[string]string{
							"url": c.ImageURL,
						},
					})
				}
			}
			if len(content) > 0 {
				openaiMsg["content"] = content
			}
		}

		// Handle tool calls
		if len(msg.ToolCalls) > 0 {
			var toolCalls []map[string]any
			for _, tc := range msg.ToolCalls {
				argsJSON, _ := json.Marshal(tc.Function.Arguments)
				toolCalls = append(toolCalls, map[string]any{
					"id":   tc.ID,
					"type": "function",
					"function": map[string]any{
						"name":      tc.Function.Name,
						"arguments": string(argsJSON),
					},
				})
			}
			openaiMsg["tool_calls"] = toolCalls
		}

		// Handle tool result
		if msg.ToolCallID != "" {
			openaiMsg["role"] = "tool"
			openaiMsg["tool_call_id"] = msg.ToolCallID
			if len(msg.Content) > 0 {
				openaiMsg["content"] = msg.Content[0].Text
			}
		}

		messages = append(messages, openaiMsg)
	}

	// Add current prompt
	if req.Prompt != "" {
		messages = append(messages, map[string]any{
			"role":    "user",
			"content": req.Prompt,
		})
	}

	openaiReq["messages"] = messages

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
		openaiReq["tools"] = tools
	}

	return openaiReq
}

// parseSSEStream parses the SSE stream from OpenAI
func (c *OpenAIClient) parseSSEStream(body io.Reader, eventChan chan<- domain.StreamEvent) {
	buf := make([]byte, 4096)
	var lineBuffer strings.Builder
	finishSent := false            // Track if we've already sent FinishEvent
	var pendingFinishReason string // Buffer finish reason until usage arrives

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
					if data == "[DONE]" {
						// Send buffered finish event if not sent yet
						if !finishSent && pendingFinishReason != "" {
							var reason domain.FinishReason
							switch pendingFinishReason {
							case "stop":
								reason = domain.FinishReasonStop
							case "tool_calls":
								reason = domain.FinishReasonToolCalls
							case "length":
								reason = domain.FinishReasonLength
							default:
								reason = domain.FinishReasonStop
							}
							eventChan <- domain.FinishEvent{Reason: reason}
						}
						return
					}
					c.parseChunk(data, eventChan, &finishSent, &pendingFinishReason)
				}
			}
		}

		if err != nil {
			if err != io.EOF && !finishSent {
				eventChan <- domain.FinishEvent{Reason: domain.FinishReasonError}
			}
			return
		}
	}
}

// parseChunk parses a JSON chunk from the stream
func (c *OpenAIClient) parseChunk(data string, eventChan chan<- domain.StreamEvent, finishSent *bool, pendingFinishReason *string) {
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
			TotalTokens      int32 `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return
	}

	// Send usage first if present, then send any pending finish event
	if chunk.Usage.TotalTokens > 0 {
		eventChan <- domain.UsageEvent{
			PromptTokens:     chunk.Usage.PromptTokens,
			CompletionTokens: chunk.Usage.CompletionTokens,
			TotalTokens:      chunk.Usage.TotalTokens,
		}

		// Now send finish event if one was buffered
		if *pendingFinishReason != "" && !*finishSent {
			var reason domain.FinishReason
			switch *pendingFinishReason {
			case "stop":
				reason = domain.FinishReasonStop
			case "tool_calls":
				reason = domain.FinishReasonToolCalls
			case "length":
				reason = domain.FinishReasonLength
			default:
				reason = domain.FinishReasonStop
			}
			eventChan <- domain.FinishEvent{Reason: reason}
			*finishSent = true
			*pendingFinishReason = "" // Clear the buffer
		}
	}

	for _, choice := range chunk.Choices {
		if choice.Delta.Content != "" {
			eventChan <- domain.TextChunk{Content: choice.Delta.Content}
		}

		for _, tc := range choice.Delta.ToolCalls {
			if tc.Function.Arguments != "" {
				eventChan <- domain.ToolCallDelta{
					ID:    tc.ID,
					Delta: tc.Function.Arguments,
				}
			}
		}

		// Buffer finish reason instead of sending immediately
		if choice.FinishReason != "" && !*finishSent {
			*pendingFinishReason = choice.FinishReason
		}
	}
}

// GenerateResponse implements the ResponsesCapable interface for OpenAI's native /v1/responses endpoint
func (c *OpenAIClient) GenerateResponse(ctx context.Context, req *domain.ResponseRequest) (*domain.StructuredResponse, error) {
	url := c.baseURL + "/responses"

	// Build OpenAI request following their Responses API format
	// The text.format object requires: type, name, and schema fields at the top level
	formatObj := map[string]any{
		"type":   "json_schema",
		"name":   req.ResponseSchema.Name, // Required at format level
		"schema": req.ResponseSchema.Schema,
		"strict": req.ResponseSchema.Strict,
	}

	// Add description if provided
	if req.ResponseSchema.Description != "" {
		formatObj["description"] = req.ResponseSchema.Description
	}

	openaiReq := map[string]any{
		"model": c.resolveModelID(req.Model),
		"text": map[string]any{
			"format": formatObj,
		},
	}

	// Build messages
	var messages []map[string]any
	for _, msg := range req.Messages {
		openaiMsg := map[string]any{
			"role": msg.Role,
		}

		// Handle content
		if len(msg.Content) == 1 && msg.Content[0].Type == "text" {
			openaiMsg["content"] = msg.Content[0].Text
		} else {
			var content []map[string]any
			for _, c := range msg.Content {
				switch c.Type {
				case "text":
					content = append(content, map[string]any{
						"type": "text",
						"text": c.Text,
					})
				case "image":
					content = append(content, map[string]any{
						"type": "image_url",
						"image_url": map[string]string{
							"url": c.ImageURL,
						},
					})
				}
			}
			if len(content) > 0 {
				openaiMsg["content"] = content
			}
		}

		messages = append(messages, openaiMsg)
	}
	openaiReq["input"] = messages

	// Add optional parameters
	if req.Temperature != nil {
		openaiReq["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		openaiReq["max_tokens"] = *req.MaxTokens
	}
	if req.TopP != nil {
		openaiReq["top_p"] = *req.TopP
	}

	// Make request
	body, err := json.Marshal(openaiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body for parsing
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAI API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Log raw response for debugging
	slog.Debug("OpenAI Responses API raw response", "body", string(bodyBytes))

	// Parse OpenAI response - the response structure includes output array
	var openaiResp struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created_at"` // Note: created_at not created
		Model   string `json:"model"`
		Output  []struct {
			Type    string `json:"type"`
			ID      string `json:"id"`
			Status  string `json:"status"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
			TotalTokens  int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(bodyBytes, &openaiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Extract the JSON response from output
	var parsedResponse map[string]interface{}
	for _, output := range openaiResp.Output {
		if output.Type == "message" && len(output.Content) > 0 {
			for _, content := range output.Content {
				if content.Type == "output_text" || content.Type == "text" {
					// Parse the JSON text
					if err := json.Unmarshal([]byte(content.Text), &parsedResponse); err != nil {
						slog.Warn("Failed to parse JSON from output", "text", content.Text, "error", err)
						// Return raw text as response if JSON parsing fails
						parsedResponse = map[string]interface{}{"raw_output": content.Text}
					}
					break
				}
			}
		}
	}

	// Convert to domain response
	return &domain.StructuredResponse{
		ID:       openaiResp.ID,
		Object:   openaiResp.Object,
		Created:  openaiResp.Created,
		Model:    openaiResp.Model,
		Response: parsedResponse,
		Usage: domain.ResponseUsage{
			PromptTokens:     openaiResp.Usage.InputTokens,
			CompletionTokens: openaiResp.Usage.OutputTokens,
			TotalTokens:      openaiResp.Usage.TotalTokens,
		},
	}, nil
}
