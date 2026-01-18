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
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/document"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"

	"modelgate/internal/domain"
)

// Nova request format (uses Bedrock Converse API style)
type novaRequest struct {
	Messages        []novaMessage        `json:"messages"`
	System          []novaSystemContent  `json:"system,omitempty"`
	InferenceConfig *novaInferenceConfig `json:"inferenceConfig,omitempty"`
	ToolConfig      *novaToolConfig      `json:"toolConfig,omitempty"`
}

type novaMessage struct {
	Role    string             `json:"role"`
	Content []novaContentBlock `json:"content"`
}

type novaContentBlock struct {
	Text       string          `json:"text,omitempty"`
	ToolUse    *novaToolUse    `json:"toolUse,omitempty"`
	ToolResult *novaToolResult `json:"toolResult,omitempty"`
}

type novaToolUse struct {
	ToolUseId string                 `json:"toolUseId"`
	Name      string                 `json:"name"`
	Input     map[string]interface{} `json:"input"`
}

type novaToolResult struct {
	ToolUseId string                  `json:"toolUseId"`
	Content   []novaToolResultContent `json:"content"`
}

type novaToolResultContent struct {
	Text string `json:"text,omitempty"`
}

type novaSystemContent struct {
	Text string `json:"text"`
}

type novaInferenceConfig struct {
	MaxTokens     int      `json:"maxTokens,omitempty"`
	Temperature   *float32 `json:"temperature,omitempty"`
	TopP          *float32 `json:"topP,omitempty"`
	StopSequences []string `json:"stopSequences,omitempty"`
}

// Tool configuration for Nova Converse API
type novaToolConfig struct {
	Tools []novaTool `json:"tools"`
}

// novaTool represents a tool definition for the Converse API
// The format is: { "toolSpec": { "name": ..., "description": ..., "inputSchema": ... } }
type novaTool struct {
	ToolSpec *novaToolSpec `json:"toolSpec,omitempty"`
}

type novaToolSpec struct {
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	InputSchema *novaInputSchema `json:"inputSchema"`
}

type novaInputSchema struct {
	JSON map[string]interface{} `json:"json"`
}

// Nova response format (for non-streaming)
type novaResponse struct {
	Output struct {
		Message struct {
			Role    string                `json:"role"`
			Content []novaResponseContent `json:"content"`
		} `json:"message"`
	} `json:"output"`
	StopReason string `json:"stopReason"`
	Usage      struct {
		InputTokens  int `json:"inputTokens"`
		OutputTokens int `json:"outputTokens"`
	} `json:"usage"`
}

type novaResponseContent struct {
	Text    string `json:"text,omitempty"`
	ToolUse *struct {
		ToolUseId string                 `json:"toolUseId"`
		Name      string                 `json:"name"`
		Input     map[string]interface{} `json:"input"`
	} `json:"toolUse,omitempty"`
}

// novaConverseStream uses the Bedrock Converse API for Nova models
// This provides proper usage metrics unlike InvokeModelWithResponseStream
func (c *BedrockClient) novaConverseStream(ctx context.Context, req *domain.ChatRequest, modelID string) (<-chan domain.StreamEvent, error) {
	if !c.useSDKStreaming {
		// Fall back to simulated streaming if no SDK available
		return c.novaSimulatedStream(ctx, req, modelID)
	}

	eventChan := make(chan domain.StreamEvent, 256)

	// Build Converse API messages
	var messages []types.Message
	for _, msg := range req.Messages {
		var contentBlocks []types.ContentBlock
		for _, content := range msg.Content {
			if content.Text != "" {
				contentBlocks = append(contentBlocks, &types.ContentBlockMemberText{
					Value: content.Text,
				})
			}
		}
		if len(contentBlocks) > 0 {
			messages = append(messages, types.Message{
				Role:    types.ConversationRole(msg.Role),
				Content: contentBlocks,
			})
		}
	}

	// Add simple prompt as user message if provided
	if req.Prompt != "" && len(messages) == 0 {
		messages = append(messages, types.Message{
			Role: types.ConversationRoleUser,
			Content: []types.ContentBlock{
				&types.ContentBlockMemberText{Value: req.Prompt},
			},
		})
	}

	// Build inference config
	maxTokens := int32(4096)
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}

	inferenceConfig := &types.InferenceConfiguration{
		MaxTokens: aws.Int32(maxTokens),
	}
	if req.Temperature != nil {
		inferenceConfig.Temperature = req.Temperature
	}

	// Build system prompt
	var system []types.SystemContentBlock
	if req.SystemPrompt != "" {
		system = append(system, &types.SystemContentBlockMemberText{
			Value: req.SystemPrompt,
		})
	}

	// Build tool configuration if tools are provided
	var toolConfig *types.ToolConfiguration
	if len(req.Tools) > 0 {
		var tools []types.Tool
		for _, tool := range req.Tools {
			if tool.Function.Name != "" {
				// Build tool input schema from parameters
				inputSchema := make(map[string]interface{})
				inputSchema["type"] = "object"
				if tool.Function.Parameters != nil {
					if props, ok := tool.Function.Parameters["properties"]; ok {
						inputSchema["properties"] = props
					}
					if required, ok := tool.Function.Parameters["required"]; ok {
						inputSchema["required"] = required
					}
				}

				tools = append(tools, &types.ToolMemberToolSpec{
					Value: types.ToolSpecification{
						Name:        aws.String(tool.Function.Name),
						Description: aws.String(tool.Function.Description),
						InputSchema: &types.ToolInputSchemaMemberJson{
							Value: document.NewLazyDocument(inputSchema),
						},
					},
				})
			}
		}
		if len(tools) > 0 {
			toolConfig = &types.ToolConfiguration{
				Tools: tools,
			}
			slog.Info("Bedrock Nova (streaming): Sending tools to model",
				"tool_count", len(tools),
				"model", modelID,
			)
		}
	}

	input := &bedrockruntime.ConverseStreamInput{
		ModelId:         aws.String(modelID),
		Messages:        messages,
		InferenceConfig: inferenceConfig,
		System:          system,
		ToolConfig:      toolConfig,
	}

	go func() {
		defer close(eventChan)

		startTime := time.Now()

		output, err := c.runtimeClient.ConverseStream(ctx, input)
		if err != nil {
			fmt.Printf("[NOVA ERROR] ConverseStream failed: %v\n", err)
			eventChan <- domain.PolicyViolationEvent{
				Message:  fmt.Sprintf("ConverseStream failed: %v", err),
				Severity: "critical",
			}
			eventChan <- domain.FinishEvent{Reason: domain.FinishReasonError}
			return
		}

		stream := output.GetStream()
		defer stream.Close()

		var inputTokens, outputTokens int
		firstTokenReceived := false

		// Tool call tracking for streaming
		var currentToolID string
		var currentToolName string
		var currentToolInput strings.Builder

		for event := range stream.Events() {
			switch v := event.(type) {
			case *types.ConverseStreamOutputMemberContentBlockStart:
				// Handle tool use start
				if v.Value.Start != nil {
					if toolUse, ok := v.Value.Start.(*types.ContentBlockStartMemberToolUse); ok {
						currentToolID = aws.ToString(toolUse.Value.ToolUseId)
						currentToolName = aws.ToString(toolUse.Value.Name)
						currentToolInput.Reset()
						slog.Info("Bedrock Nova (streaming): Tool call started",
							"tool_id", currentToolID,
							"tool_name", currentToolName,
						)
					}
				}

			case *types.ConverseStreamOutputMemberContentBlockDelta:
				// Handle text delta
				if delta, ok := v.Value.Delta.(*types.ContentBlockDeltaMemberText); ok {
					if delta.Value != "" {
						if !firstTokenReceived {
							ttft := time.Since(startTime)
							if ttft > time.Second {
								fmt.Printf("[NOVA PERF] Time to first token: %v\n", ttft)
							}
							firstTokenReceived = true
						}
						eventChan <- domain.TextChunk{Content: delta.Value}
					}
				}
				// Handle tool input delta
				if toolDelta, ok := v.Value.Delta.(*types.ContentBlockDeltaMemberToolUse); ok {
					if toolDelta.Value.Input != nil {
						currentToolInput.WriteString(*toolDelta.Value.Input)
					}
				}

			case *types.ConverseStreamOutputMemberContentBlockStop:
				// If we were building a tool call, emit it now
				if currentToolID != "" && currentToolName != "" {
					// Parse the accumulated JSON input
					var args map[string]interface{}
					if inputStr := currentToolInput.String(); inputStr != "" {
						if err := json.Unmarshal([]byte(inputStr), &args); err != nil {
							slog.Warn("Failed to parse tool input JSON",
								"tool_name", currentToolName,
								"error", err,
								"input", inputStr,
							)
							args = make(map[string]interface{})
						}
					} else {
						args = make(map[string]interface{})
					}

					eventChan <- domain.ToolCallEvent{
						ToolCall: domain.ToolCall{
							ID:   currentToolID,
							Type: "function",
							Function: domain.FunctionCall{
								Name:      currentToolName,
								Arguments: args,
							},
						},
					}
					slog.Info("Bedrock Nova (streaming): Tool call completed",
						"tool_id", currentToolID,
						"tool_name", currentToolName,
					)

					// Reset for next tool call
					currentToolID = ""
					currentToolName = ""
					currentToolInput.Reset()
				}

			case *types.ConverseStreamOutputMemberMetadata:
				// This event contains usage metrics!
				if v.Value.Usage != nil {
					if v.Value.Usage.InputTokens != nil {
						inputTokens = int(*v.Value.Usage.InputTokens)
					}
					if v.Value.Usage.OutputTokens != nil {
						outputTokens = int(*v.Value.Usage.OutputTokens)
					}
					fmt.Printf("[NOVA DEBUG] Usage from metadata: input=%d, output=%d\n", inputTokens, outputTokens)
				}

			case *types.ConverseStreamOutputMemberMessageStop:
				// Message complete, usage should have been received
				goto streamDone
			}
		}

	streamDone:
		if err := stream.Err(); err != nil {
			eventChan <- domain.PolicyViolationEvent{
				Message:  fmt.Sprintf("Stream error: %v", err),
				Severity: "critical",
			}
			eventChan <- domain.FinishEvent{Reason: domain.FinishReasonError}
			return
		}

		// Send usage event
		eventChan <- domain.UsageEvent{
			PromptTokens:     int32(inputTokens),
			CompletionTokens: int32(outputTokens),
			TotalTokens:      int32(inputTokens + outputTokens),
		}

		eventChan <- domain.FinishEvent{Reason: domain.FinishReasonStop}
	}()

	return eventChan, nil
}

// novaSimulatedStream uses the non-streaming Converse API and simulates streaming
func (c *BedrockClient) novaSimulatedStream(ctx context.Context, req *domain.ChatRequest, modelID string) (<-chan domain.StreamEvent, error) {
	eventChan := make(chan domain.StreamEvent, 100)

	novaReq := c.buildNovaRequest(req)
	body, _ := json.Marshal(novaReq)

	go func() {
		defer close(eventChan)

		endpoint := c.getBedrockEndpoint()
		url := fmt.Sprintf("%s/model/%s/invoke", endpoint, modelID)

		httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
		if err != nil {
			eventChan <- domain.PolicyViolationEvent{
				Message:  fmt.Sprintf("Failed to create request: %v", err),
				Severity: "critical",
			}
			eventChan <- domain.FinishEvent{Reason: domain.FinishReasonError}
			return
		}

		httpReq.Header.Set("Content-Type", "application/json")
		if c.apiKey != "" {
			httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
		}

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			eventChan <- domain.PolicyViolationEvent{
				Message:  fmt.Sprintf("HTTP request failed: %v", err),
				Severity: "critical",
			}
			eventChan <- domain.FinishEvent{Reason: domain.FinishReasonError}
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			eventChan <- domain.PolicyViolationEvent{
				Message:  fmt.Sprintf("API error %d: %s", resp.StatusCode, string(respBody)),
				Severity: "critical",
			}
			eventChan <- domain.FinishEvent{Reason: domain.FinishReasonError}
			return
		}

		respBody, _ := io.ReadAll(resp.Body)
		var novaResp novaResponse
		if err := json.Unmarshal(respBody, &novaResp); err != nil {
			eventChan <- domain.PolicyViolationEvent{
				Message:  fmt.Sprintf("Failed to decode response: %v", err),
				Severity: "critical",
			}
			eventChan <- domain.FinishEvent{Reason: domain.FinishReasonError}
			return
		}

		// Process response content - handle both text and tool use
		var textContent strings.Builder
		var toolCalls []domain.ToolCall

		for _, content := range novaResp.Output.Message.Content {
			if content.Text != "" {
				textContent.WriteString(content.Text)
			}
			if content.ToolUse != nil {
				toolCalls = append(toolCalls, domain.ToolCall{
					ID:   content.ToolUse.ToolUseId,
					Type: "function",
					Function: domain.FunctionCall{
						Name:      content.ToolUse.Name,
						Arguments: content.ToolUse.Input,
					},
				})
			}
		}

		// Simulate streaming for text content
		if text := textContent.String(); text != "" {
			chunkSize := 20
			for i := 0; i < len(text); i += chunkSize {
				end := i + chunkSize
				if end > len(text) {
					end = len(text)
				}
				eventChan <- domain.TextChunk{Content: text[i:end]}
			}
		}

		// Emit tool call events
		for _, tc := range toolCalls {
			eventChan <- domain.ToolCallEvent{ToolCall: tc}
		}

		eventChan <- domain.UsageEvent{
			PromptTokens:     int32(novaResp.Usage.InputTokens),
			CompletionTokens: int32(novaResp.Usage.OutputTokens),
			TotalTokens:      int32(novaResp.Usage.InputTokens + novaResp.Usage.OutputTokens),
		}

		// Determine finish reason
		finishReason := domain.FinishReasonStop
		if novaResp.StopReason == "tool_use" || len(toolCalls) > 0 {
			finishReason = domain.FinishReasonToolCalls
		}
		eventChan <- domain.FinishEvent{Reason: finishReason}
	}()

	return eventChan, nil
}

// buildNovaRequest converts domain request to Amazon Nova format
func (c *BedrockClient) buildNovaRequest(req *domain.ChatRequest) novaRequest {
	novaReq := novaRequest{
		InferenceConfig: &novaInferenceConfig{
			MaxTokens: 4096,
		},
	}

	if req.MaxTokens != nil {
		novaReq.InferenceConfig.MaxTokens = int(*req.MaxTokens)
	}

	if req.Temperature != nil {
		novaReq.InferenceConfig.Temperature = req.Temperature
	}

	if req.SystemPrompt != "" {
		novaReq.System = []novaSystemContent{{Text: req.SystemPrompt}}
	}

	for _, msg := range req.Messages {
		novaMsg := novaMessage{
			Role:    msg.Role,
			Content: []novaContentBlock{},
		}

		// Handle text content
		for _, content := range msg.Content {
			if (content.Type == "text" || content.Type == "") && content.Text != "" {
				novaMsg.Content = append(novaMsg.Content, novaContentBlock{
					Text: content.Text,
				})
			}
		}

		// Handle tool calls from assistant messages
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				novaMsg.Content = append(novaMsg.Content, novaContentBlock{
					ToolUse: &novaToolUse{
						ToolUseId: tc.ID,
						Name:      tc.Function.Name,
						Input:     tc.Function.Arguments,
					},
				})
			}
		}

		// Handle tool role messages (tool results)
		if msg.Role == "tool" && msg.ToolCallID != "" {
			// Tool results should be in a user message for Nova
			var resultText string
			for _, content := range msg.Content {
				if content.Text != "" {
					resultText = content.Text
					break
				}
			}
			novaMsg = novaMessage{
				Role: "user",
				Content: []novaContentBlock{
					{
						ToolResult: &novaToolResult{
							ToolUseId: msg.ToolCallID,
							Content:   []novaToolResultContent{{Text: resultText}},
						},
					},
				},
			}
		}

		if len(novaMsg.Content) > 0 {
			novaReq.Messages = append(novaReq.Messages, novaMsg)
		}
	}

	if req.Prompt != "" && len(novaReq.Messages) == 0 {
		novaReq.Messages = append(novaReq.Messages, novaMessage{
			Role:    "user",
			Content: []novaContentBlock{{Text: req.Prompt}},
		})
	}

	// Add tool configuration if tools are provided
	if len(req.Tools) > 0 {
		var tools []novaTool
		for _, tool := range req.Tools {
			if tool.Function.Name != "" {
				// Build input schema - Bedrock requires a proper JSON schema
				inputSchema := map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				}
				if tool.Function.Parameters != nil {
					// Copy properties if present
					if props, ok := tool.Function.Parameters["properties"]; ok {
						inputSchema["properties"] = props
					}
					// Copy required if present
					if required, ok := tool.Function.Parameters["required"]; ok {
						inputSchema["required"] = required
					}
					// Copy type if present (override default)
					if typeVal, ok := tool.Function.Parameters["type"]; ok {
						inputSchema["type"] = typeVal
					}
				}

				tools = append(tools, novaTool{
					ToolSpec: &novaToolSpec{
						Name:        tool.Function.Name,
						Description: tool.Function.Description,
						InputSchema: &novaInputSchema{
							JSON: inputSchema,
						},
					},
				})
			}
		}
		if len(tools) > 0 {
			novaReq.ToolConfig = &novaToolConfig{
				Tools: tools,
			}
			slog.Info("Bedrock Nova (HTTP): Built request with tools",
				"tool_count", len(tools),
			)
		}
	}

	return novaReq
}

// novaComplete performs a non-streaming chat completion for Nova
func (c *BedrockClient) novaComplete(ctx context.Context, req *domain.ChatRequest, modelID string) (*domain.ChatResponse, error) {
	// Use SDK Converse API if available (IAM credentials)
	if c.useSDKStreaming && c.runtimeClient != nil {
		return c.novaConverseComplete(ctx, req, modelID)
	}

	// Fallback to HTTP with Converse API (supports tools)
	// Fallback to HTTP with Bearer token (now with tool support)
	novaReq := c.buildNovaRequest(req)
	body, _ := json.Marshal(novaReq)

	endpoint := c.getBedrockEndpoint()
	// Use /converse endpoint when tools are present (supports tool calling), otherwise /invoke
	var url string
	if len(req.Tools) > 0 {
		url = fmt.Sprintf("%s/model/%s/converse", endpoint, modelID)
	} else {
		url = fmt.Sprintf("%s/model/%s/invoke", endpoint, modelID)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bedrock API error %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, _ := io.ReadAll(resp.Body)
	var novaResp novaResponse
	if err := json.Unmarshal(respBody, &novaResp); err != nil {
		return nil, err
	}

	response := &domain.ChatResponse{
		Model: req.Model,
		Usage: &domain.UsageEvent{
			PromptTokens:     int32(novaResp.Usage.InputTokens),
			CompletionTokens: int32(novaResp.Usage.OutputTokens),
			TotalTokens:      int32(novaResp.Usage.InputTokens + novaResp.Usage.OutputTokens),
		},
	}

	// Process response content - handle both text and tool use
	var textContent strings.Builder
	var toolCalls []domain.ToolCall

	for _, content := range novaResp.Output.Message.Content {
		if content.Text != "" {
			textContent.WriteString(content.Text)
		}
		if content.ToolUse != nil {
			toolCalls = append(toolCalls, domain.ToolCall{
				ID:   content.ToolUse.ToolUseId,
				Type: "function",
				Function: domain.FunctionCall{
					Name:      content.ToolUse.Name,
					Arguments: content.ToolUse.Input,
				},
			})
		}
	}

	response.Content = textContent.String()
	response.ToolCalls = toolCalls

	if len(toolCalls) > 0 {
		slog.Info("Bedrock Nova (HTTP): Received tool calls",
			"tool_count", len(toolCalls),
		)
	}

	// Map stop reason
	switch novaResp.StopReason {
	case "end_turn", "stop":
		response.FinishReason = domain.FinishReasonStop
	case "max_tokens":
		response.FinishReason = domain.FinishReasonLength
	case "tool_use":
		response.FinishReason = domain.FinishReasonToolCalls
	default:
		response.FinishReason = domain.FinishReasonStop
	}

	return response, nil
}

// novaConverseComplete uses the SDK's Converse API for non-streaming requests
// This properly handles IAM authentication and supports tool calling
func (c *BedrockClient) novaConverseComplete(ctx context.Context, req *domain.ChatRequest, modelID string) (*domain.ChatResponse, error) {
	// Build Converse API messages
	// NOTE: Bedrock Converse API only supports "user" and "assistant" roles
	// Tool results must be sent as ToolResultBlock content within a user message
	// IMMEDIATELY after the assistant's tool_use turn
	var messages []types.Message

	// First pass: collect consecutive tool results and convert to user messages
	i := 0
	for i < len(req.Messages) {
		msg := req.Messages[i]

		switch msg.Role {
		case "tool":
			// Collect ALL consecutive tool results into a single user message
			// This handles cases where multiple tools are called in one turn
			var toolResults []types.ContentBlock
			for i < len(req.Messages) && req.Messages[i].Role == "tool" {
				toolMsg := req.Messages[i]
				resultText := ""
				for _, content := range toolMsg.Content {
					if content.Text != "" {
						resultText += content.Text
					}
				}

				toolResults = append(toolResults, &types.ContentBlockMemberToolResult{
					Value: types.ToolResultBlock{
						ToolUseId: aws.String(toolMsg.ToolCallID),
						Content: []types.ToolResultContentBlock{
							&types.ToolResultContentBlockMemberText{
								Value: resultText,
							},
						},
					},
				})
				i++
			}

			// Add as user message (tool results must be in a user message)
			if len(toolResults) > 0 {
				messages = append(messages, types.Message{
					Role:    types.ConversationRoleUser,
					Content: toolResults,
				})
			}
			continue // Don't increment i again, already done in the inner loop

		case "assistant":
			var contentBlocks []types.ContentBlock

			// Add text content if present
			for _, content := range msg.Content {
				if content.Text != "" {
					contentBlocks = append(contentBlocks, &types.ContentBlockMemberText{
						Value: content.Text,
					})
				}
			}

			// Add tool use blocks if the message has tool calls
			if len(msg.ToolCalls) > 0 {
				for _, tc := range msg.ToolCalls {
					var argsDoc document.Interface
					if tc.Function.Arguments != nil {
						argsDoc = document.NewLazyDocument(tc.Function.Arguments)
					} else {
						argsDoc = document.NewLazyDocument(map[string]interface{}{})
					}

					contentBlocks = append(contentBlocks, &types.ContentBlockMemberToolUse{
						Value: types.ToolUseBlock{
							ToolUseId: aws.String(tc.ID),
							Name:      aws.String(tc.Function.Name),
							Input:     argsDoc,
						},
					})
				}
			}

			if len(contentBlocks) > 0 {
				messages = append(messages, types.Message{
					Role:    types.ConversationRoleAssistant,
					Content: contentBlocks,
				})
			}

		case "user":
			var contentBlocks []types.ContentBlock

			for _, content := range msg.Content {
				if content.Text != "" {
					contentBlocks = append(contentBlocks, &types.ContentBlockMemberText{
						Value: content.Text,
					})
				}
			}

			if len(contentBlocks) > 0 {
				messages = append(messages, types.Message{
					Role:    types.ConversationRoleUser,
					Content: contentBlocks,
				})
			}

		default:
			// Skip system (handled separately) and unknown roles
		}
		i++
	}

	// Add simple prompt as user message if provided
	if req.Prompt != "" && len(messages) == 0 {
		messages = append(messages, types.Message{
			Role: types.ConversationRoleUser,
			Content: []types.ContentBlock{
				&types.ContentBlockMemberText{Value: req.Prompt},
			},
		})
	}

	// Build inference config
	maxTokens := int32(4096)
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}

	inferenceConfig := &types.InferenceConfiguration{
		MaxTokens: aws.Int32(maxTokens),
	}
	if req.Temperature != nil {
		inferenceConfig.Temperature = req.Temperature
	}

	// Build system prompt
	var system []types.SystemContentBlock
	if req.SystemPrompt != "" {
		system = append(system, &types.SystemContentBlockMemberText{
			Value: req.SystemPrompt,
		})
	}

	// Build tool configuration if tools are provided
	var toolConfig *types.ToolConfiguration
	if len(req.Tools) > 0 {
		var tools []types.Tool
		for _, tool := range req.Tools {
			if tool.Function.Name != "" {
				// Build tool input schema from parameters
				inputSchema := make(map[string]interface{})
				inputSchema["type"] = "object"
				if tool.Function.Parameters != nil {
					if props, ok := tool.Function.Parameters["properties"]; ok {
						inputSchema["properties"] = props
					}
					if required, ok := tool.Function.Parameters["required"]; ok {
						inputSchema["required"] = required
					}
				}

				tools = append(tools, &types.ToolMemberToolSpec{
					Value: types.ToolSpecification{
						Name:        aws.String(tool.Function.Name),
						Description: aws.String(tool.Function.Description),
						InputSchema: &types.ToolInputSchemaMemberJson{
							Value: document.NewLazyDocument(inputSchema),
						},
					},
				})
			}
		}
		if len(tools) > 0 {
			toolConfig = &types.ToolConfiguration{
				Tools: tools,
			}
			slog.Info("Bedrock Nova: Sending tools to model",
				"tool_count", len(tools),
				"model", modelID,
			)
		}
	}

	input := &bedrockruntime.ConverseInput{
		ModelId:         aws.String(modelID),
		Messages:        messages,
		InferenceConfig: inferenceConfig,
		System:          system,
		ToolConfig:      toolConfig,
	}

	output, err := c.runtimeClient.Converse(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("bedrock Converse API error: %w", err)
	}

	// Build response
	response := &domain.ChatResponse{
		Model: req.Model,
	}

	// Extract content and tool calls
	if output.Output != nil {
		if msgOutput, ok := output.Output.(*types.ConverseOutputMemberMessage); ok {
			var textContent strings.Builder
			var toolCalls []domain.ToolCall

			for _, block := range msgOutput.Value.Content {
				switch v := block.(type) {
				case *types.ContentBlockMemberText:
					textContent.WriteString(v.Value)
				case *types.ContentBlockMemberToolUse:
					// Handle tool call - Input is a document.Interface
					args := make(map[string]interface{})
					if v.Value.Input != nil {
						// Use UnmarshalSmithyDocument to extract the arguments
						v.Value.Input.UnmarshalSmithyDocument(&args)
					}

					toolCalls = append(toolCalls, domain.ToolCall{
						ID:   aws.ToString(v.Value.ToolUseId),
						Type: "function",
						Function: domain.FunctionCall{
							Name:      aws.ToString(v.Value.Name),
							Arguments: args,
						},
					})
				}
			}

			response.Content = textContent.String()
			response.ToolCalls = toolCalls

			slog.Info("Bedrock Nova: Response received",
				"has_content", len(textContent.String()) > 0,
				"tool_calls_count", len(toolCalls),
				"stop_reason", string(output.StopReason),
			)
		}
	}

	// Extract usage
	if output.Usage != nil {
		response.Usage = &domain.UsageEvent{
			PromptTokens:     int32(aws.ToInt32(output.Usage.InputTokens)),
			CompletionTokens: int32(aws.ToInt32(output.Usage.OutputTokens)),
			TotalTokens:      int32(aws.ToInt32(output.Usage.InputTokens) + aws.ToInt32(output.Usage.OutputTokens)),
		}
	}

	// Map stop reason
	switch output.StopReason {
	case types.StopReasonEndTurn:
		response.FinishReason = domain.FinishReasonStop
	case types.StopReasonMaxTokens:
		response.FinishReason = domain.FinishReasonLength
	case types.StopReasonToolUse:
		response.FinishReason = domain.FinishReasonToolCalls
	default:
		response.FinishReason = domain.FinishReasonStop
	}

	return response, nil
}

// truncateForDebug truncates a string for debug logging
func truncateForDebug(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
