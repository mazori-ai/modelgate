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

// GeminiClient is a client for Google Gemini API
type GeminiClient struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
	modelCache map[string]string // Cache of model aliases to native model IDs
}

// NewGeminiClient creates a new Gemini client
func NewGeminiClient(apiKey string, settings ...domain.ConnectionSettings) (*GeminiClient, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	// Use provided settings or defaults
	connSettings := domain.DefaultConnectionSettings()
	if len(settings) > 0 {
		connSettings = settings[0]
	}

	return &GeminiClient{
		apiKey:     apiKey,
		httpClient: BuildHTTPClient(connSettings),
		baseURL:    "https://generativelanguage.googleapis.com/v1beta",
		modelCache: make(map[string]string),
	}, nil
}

// SetModelCache sets the model cache (implements ModelCacheable)
func (c *GeminiClient) SetModelCache(cache map[string]string) {
	c.modelCache = cache
}

// GetModelCache returns the model cache (implements ModelCacheable)
func (c *GeminiClient) GetModelCache() map[string]string {
	return c.modelCache
}

// resolveModelID resolves a model ID using the cache if available
func (c *GeminiClient) resolveModelID(model string) string {
	if c.modelCache != nil {
		if nativeID, ok := c.modelCache[model]; ok {
			return ExtractModelID(nativeID)
		}
	}
	return ExtractModelID(model)
}

// Provider returns the provider type
func (c *GeminiClient) Provider() domain.Provider {
	return domain.ProviderGemini
}

// SupportsModel checks if a model is supported
func (c *GeminiClient) SupportsModel(model string) bool {
	modelID := ExtractModelID(model)
	return strings.HasPrefix(strings.ToLower(modelID), "gemini")
}

// ChatStream starts a streaming chat completion
func (c *GeminiClient) ChatStream(ctx context.Context, req *domain.ChatRequest) (<-chan domain.StreamEvent, error) {
	eventChan := make(chan domain.StreamEvent, 100)

	go func() {
		defer close(eventChan)

		// Build the request body
		modelID := c.resolveModelID(req.Model)
		url := fmt.Sprintf("%s/models/%s:streamGenerateContent?key=%s&alt=sse", c.baseURL, modelID, c.apiKey)

		geminiReq := c.buildRequest(req)
		body, err := json.Marshal(geminiReq)
		if err != nil {
			eventChan <- domain.PolicyViolationEvent{
				Message: fmt.Sprintf("Failed to marshal request: %v", err),
			}
			return
		}

		// Debug logging
		slog.Debug("[GEMINI] Request",
			"model", modelID,
			"url", url,
			"messages_count", len(req.Messages),
			"has_system_prompt", req.SystemPrompt != "",
			"request_body", string(body),
		)

		httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
		if err != nil {
			slog.Error("[GEMINI] Failed to create request", "error", err)
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			slog.Error("[GEMINI] HTTP request failed", "error", err)
			eventChan <- domain.FinishEvent{Reason: domain.FinishReasonError}
			return
		}
		defer resp.Body.Close()

		slog.Debug("[GEMINI] Response status", "status", resp.Status, "status_code", resp.StatusCode)

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			slog.Error("[GEMINI] API error", "status", resp.Status, "body", string(bodyBytes))
			eventChan <- domain.PolicyViolationEvent{
				Message: fmt.Sprintf("API error: %s - %s", resp.Status, string(bodyBytes)),
			}
			return
		}

		// Parse SSE stream
		c.parseSSEStream(resp.Body, eventChan)
	}()

	return eventChan, nil
}

// ChatComplete performs a non-streaming chat completion
func (c *GeminiClient) ChatComplete(ctx context.Context, req *domain.ChatRequest) (*domain.ChatResponse, error) {
	// Collect from stream
	events, err := c.ChatStream(ctx, req)
	if err != nil {
		return nil, err
	}

	response := &domain.ChatResponse{
		Model: req.Model,
	}

	var contentBuilder strings.Builder
	var thinkingBuilder strings.Builder

	for event := range events {
		switch e := event.(type) {
		case domain.TextChunk:
			contentBuilder.WriteString(e.Content)
		case domain.ThinkingChunk:
			thinkingBuilder.WriteString(e.Content)
		case domain.ToolCallEvent:
			response.ToolCalls = append(response.ToolCalls, e.ToolCall)
		case domain.UsageEvent:
			response.Usage = &e
			response.CostUSD = e.CostUSD
		case domain.FinishEvent:
			response.FinishReason = e.Reason
		}
	}

	response.Content = contentBuilder.String()
	response.Thinking = thinkingBuilder.String()

	if response.FinishReason == "" {
		response.FinishReason = domain.FinishReasonStop
	}

	return response, nil
}

// Embed generates embeddings
func (c *GeminiClient) Embed(ctx context.Context, model string, texts []string, dimensions *int32) ([][]float32, int64, error) {
	// Gemini embedding API
	modelID := ExtractModelID(model)
	if modelID == "" {
		modelID = "embedding-001"
	}

	url := fmt.Sprintf("%s/models/%s:batchEmbedContents?key=%s", c.baseURL, modelID, c.apiKey)

	// Build request
	var requests []map[string]any
	for _, text := range texts {
		requests = append(requests, map[string]any{
			"model":   fmt.Sprintf("models/%s", modelID),
			"content": map[string]any{"parts": []map[string]string{{"text": text}}},
		})
	}

	body, _ := json.Marshal(map[string]any{"requests": requests})

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
		Embeddings []struct {
			Values []float32 `json:"values"`
		} `json:"embeddings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, 0, err
	}

	embeddings := make([][]float32, len(result.Embeddings))
	for i, e := range result.Embeddings {
		embeddings[i] = e.Values
	}

	// Estimate tokens
	var totalChars int
	for _, t := range texts {
		totalChars += len(t)
	}
	estimatedTokens := int64(totalChars / 4)

	return embeddings, estimatedTokens, nil
}

// CountTokens counts tokens in a request
func (c *GeminiClient) CountTokens(ctx context.Context, req *domain.ChatRequest) (int32, error) {
	modelID := ExtractModelID(req.Model)
	url := fmt.Sprintf("%s/models/%s:countTokens?key=%s", c.baseURL, modelID, c.apiKey)

	geminiReq := c.buildRequest(req)
	body, _ := json.Marshal(geminiReq)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result struct {
		TotalTokens int32 `json:"totalTokens"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	return result.TotalTokens, nil
}

// ListModels lists available models
func (c *GeminiClient) ListModels(ctx context.Context) ([]domain.ModelInfo, error) {
	url := fmt.Sprintf("%s/models?key=%s", c.baseURL, c.apiKey)

	httpReq, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read response body for debugging
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		slog.Error("[GEMINI] ListModels API error", "status", resp.Status, "body", string(bodyBytes))
		return nil, fmt.Errorf("API error: %s", resp.Status)
	}

	var result struct {
		Models []struct {
			Name                       string   `json:"name"`
			DisplayName                string   `json:"displayName"`
			SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
			InputTokenLimit            int      `json:"inputTokenLimit"`
			OutputTokenLimit           int      `json:"outputTokenLimit"`
		} `json:"models"`
	}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		slog.Error("[GEMINI] Failed to parse ListModels response", "error", err, "body", truncateStr(string(bodyBytes), 500))
		return nil, err
	}

	slog.Info("[GEMINI] ListModels raw response", "total_models", len(result.Models))

	var models []domain.ModelInfo
	for _, m := range result.Models {
		// Extract model ID from "models/gemini-1.5-pro" format
		modelID := strings.TrimPrefix(m.Name, "models/")

		slog.Debug("[GEMINI] Processing model",
			"raw_name", m.Name,
			"model_id", modelID,
			"display_name", m.DisplayName,
			"methods", m.SupportedGenerationMethods)

		// Only include models that support generateContent (chat)
		supportsChat := false
		supportsTools := false
		for _, method := range m.SupportedGenerationMethods {
			if method == "generateContent" {
				supportsChat = true
				supportsTools = true
			}
		}

		// Skip models that don't support chat
		if !supportsChat {
			slog.Debug("[GEMINI] Skipping non-chat model", "model", modelID, "methods", m.SupportedGenerationMethods)
			continue
		}

		// Skip embedding models
		if strings.Contains(modelID, "embedding") {
			slog.Debug("[GEMINI] Skipping embedding model", "model", modelID)
			continue
		}

		// Skip AQA models (question-answering only)
		if strings.Contains(modelID, "aqa") {
			slog.Debug("[GEMINI] Skipping AQA model", "model", modelID)
			continue
		}

		slog.Info("[GEMINI] Including model", "id", fmt.Sprintf("gemini/%s", modelID), "name", m.DisplayName)

		models = append(models, domain.ModelInfo{
			ID:            fmt.Sprintf("gemini/%s", modelID),
			Name:          m.DisplayName,
			Provider:      domain.ProviderGemini,
			SupportsTools: supportsTools,
			ContextLimit:  uint32(m.InputTokenLimit),
			OutputLimit:   uint32(m.OutputTokenLimit),
			Enabled:       true,
		})
	}

	slog.Info("[GEMINI] Final chat models list", "count", len(models))
	return models, nil
}

// buildRequest builds a Gemini API request
func (c *GeminiClient) buildRequest(req *domain.ChatRequest) map[string]any {
	geminiReq := map[string]any{}

	// Build contents
	var contents []map[string]any

	// Add system prompt
	if req.SystemPrompt != "" {
		contents = append(contents, map[string]any{
			"role":  "user",
			"parts": []map[string]string{{"text": req.SystemPrompt}},
		})
		contents = append(contents, map[string]any{
			"role":  "model",
			"parts": []map[string]string{{"text": "I understand and will follow these instructions."}},
		})
	}

	// Add messages
	for _, msg := range req.Messages {
		role := "user"
		if msg.Role == "assistant" {
			role = "model"
		}

		var parts []map[string]any
		for _, content := range msg.Content {
			switch content.Type {
			case "text":
				parts = append(parts, map[string]any{"text": content.Text})
			case "image":
				if content.ImageURL != "" {
					parts = append(parts, map[string]any{
						"fileData": map[string]string{
							"fileUri":  content.ImageURL,
							"mimeType": content.MediaType,
						},
					})
				}
			}
		}

		// Handle tool calls from assistant
		if len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				parts = append(parts, map[string]any{
					"functionCall": map[string]any{
						"name": tc.Function.Name,
						"args": tc.Function.Arguments,
					},
				})
			}
		}

		// Handle tool result messages (role="tool" or messages with ToolCallID)
		if msg.ToolCallID != "" {
			// Find the tool name from content or use a placeholder
			toolName := "unknown"
			resultContent := ""
			if len(msg.Content) > 0 && msg.Content[0].Type == "text" {
				resultContent = msg.Content[0].Text
			}

			parts = append(parts, map[string]any{
				"functionResponse": map[string]any{
					"name": toolName,
					"response": map[string]any{
						"content": resultContent,
					},
				},
			})
		}

		if len(parts) > 0 {
			contents = append(contents, map[string]any{
				"role":  role,
				"parts": parts,
			})
		}
	}

	// Add current prompt
	if req.Prompt != "" {
		contents = append(contents, map[string]any{
			"role":  "user",
			"parts": []map[string]string{{"text": req.Prompt}},
		})
	}

	geminiReq["contents"] = contents

	// Add tools
	if len(req.Tools) > 0 {
		var functions []map[string]any
		for _, tool := range req.Tools {
			functions = append(functions, map[string]any{
				"name":        tool.Function.Name,
				"description": tool.Function.Description,
				"parameters":  tool.Function.Parameters,
			})
		}
		geminiReq["tools"] = []map[string]any{
			{"functionDeclarations": functions},
		}
	}

	// Generation config
	generationConfig := map[string]any{}
	if req.Temperature != nil {
		generationConfig["temperature"] = *req.Temperature
	}
	if req.MaxTokens != nil {
		generationConfig["maxOutputTokens"] = *req.MaxTokens
	}
	if len(generationConfig) > 0 {
		geminiReq["generationConfig"] = generationConfig
	}

	return geminiReq
}

// parseSSEStream parses the SSE stream from Gemini
func (c *GeminiClient) parseSSEStream(body io.Reader, eventChan chan<- domain.StreamEvent) {
	// Simple SSE parser
	buf := make([]byte, 4096)
	var lineBuffer strings.Builder

	for {
		n, err := body.Read(buf)
		if n > 0 {
			chunk := string(buf[:n])
			lineBuffer.WriteString(chunk)

			// Process complete lines
			content := lineBuffer.String()
			lines := strings.Split(content, "\n")

			// Keep incomplete line in buffer
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
						eventChan <- domain.FinishEvent{Reason: domain.FinishReasonStop}
						return
					}

					c.parseChunk(data, eventChan)
				}
			}
		}

		if err != nil {
			if err != io.EOF {
				eventChan <- domain.FinishEvent{Reason: domain.FinishReasonError}
			} else {
				eventChan <- domain.FinishEvent{Reason: domain.FinishReasonStop}
			}
			return
		}
	}
}

// parseChunk parses a JSON chunk from the stream
func (c *GeminiClient) parseChunk(data string, eventChan chan<- domain.StreamEvent) {
	var chunk struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text         string `json:"text"`
					FunctionCall struct {
						Name string         `json:"name"`
						Args map[string]any `json:"args"`
					} `json:"functionCall"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
		UsageMetadata struct {
			PromptTokenCount     int32 `json:"promptTokenCount"`
			CandidatesTokenCount int32 `json:"candidatesTokenCount"`
			TotalTokenCount      int32 `json:"totalTokenCount"`
		} `json:"usageMetadata"`
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Status  string `json:"status"`
		} `json:"error"`
	}

	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		slog.Error("[GEMINI] Failed to parse chunk", "error", err, "data", data)
		return
	}

	// Check for errors in response
	if chunk.Error.Message != "" {
		slog.Error("[GEMINI] API error in chunk", "code", chunk.Error.Code, "message", chunk.Error.Message, "status", chunk.Error.Status)
		eventChan <- domain.PolicyViolationEvent{Message: chunk.Error.Message}
		return
	}

	slog.Debug("[GEMINI] Parsed chunk", "candidates", len(chunk.Candidates), "data_preview", truncateStr(data, 200))

	// Send content events
	for _, candidate := range chunk.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				eventChan <- domain.TextChunk{Content: part.Text}
			}
			if part.FunctionCall.Name != "" {
				eventChan <- domain.ToolCallEvent{
					ToolCall: domain.ToolCall{
						ID:   fmt.Sprintf("call_%d", len(part.FunctionCall.Name)), // Generate ID
						Type: "function",
						Function: domain.FunctionCall{
							Name:      part.FunctionCall.Name,
							Arguments: part.FunctionCall.Args,
						},
					},
				}
			}
		}

	}

	// Send usage metadata if present (comes with final chunks)
	if chunk.UsageMetadata.TotalTokenCount > 0 {
		eventChan <- domain.UsageEvent{
			PromptTokens:     chunk.UsageMetadata.PromptTokenCount,
			CompletionTokens: chunk.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      chunk.UsageMetadata.TotalTokenCount,
		}
	}

	// Send finish reason (comes after usage in Gemini streaming)
	for _, candidate := range chunk.Candidates {
		if candidate.FinishReason != "" {
			var reason domain.FinishReason
			switch candidate.FinishReason {
			case "STOP":
				reason = domain.FinishReasonStop
			case "MAX_TOKENS":
				reason = domain.FinishReasonLength
			default:
				reason = domain.FinishReasonStop
			}
			eventChan <- domain.FinishEvent{Reason: reason}
		}
	}
}

// truncateStr truncates a string to maxLen chars
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
