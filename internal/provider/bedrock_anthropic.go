package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/document"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"

	"modelgate/internal/domain"
)

// Anthropic/Claude request format
type anthropicRequest struct {
	AnthropicVersion string          `json:"anthropic_version,omitempty"`
	MaxTokens        int             `json:"max_tokens"`
	Messages         []anthropicMsg  `json:"messages"`
	System           string          `json:"system,omitempty"`
	Temperature      *float32        `json:"temperature,omitempty"`
	TopP             *float32        `json:"top_p,omitempty"`
	Tools            []anthropicTool `json:"tools,omitempty"`
}

type anthropicMsg struct {
	Role    string                `json:"role"`
	Content []anthropicMsgContent `json:"content"`
}

// anthropicMsgContent represents content in an Anthropic message.
// Different content types use different fields:
// - text: uses Text field
// - tool_use: uses ID, Name, Input fields (Input must not be nil)
// - tool_result: uses ToolUseID, Content fields
type anthropicMsgContent struct {
	Type      string                 `json:"type"`
	Text      string                 `json:"text,omitempty"`
	ID        string                 `json:"id,omitempty"`
	Name      string                 `json:"name,omitempty"`
	Input     map[string]interface{} `json:"input,omitempty"`
	ToolUseID string                 `json:"tool_use_id,omitempty"`
	Content   string                 `json:"content,omitempty"`
}

// MarshalJSON customizes JSON marshaling to ensure tool_use always has input field
func (c anthropicMsgContent) MarshalJSON() ([]byte, error) {
	type Alias anthropicMsgContent
	
	// For tool_use type, ensure input is always present (even if empty)
	if c.Type == "tool_use" {
		// Create a map with all fields, including input
		m := map[string]interface{}{
			"type": c.Type,
			"id":   c.ID,
			"name": c.Name,
		}
		if c.Input != nil {
			m["input"] = c.Input
		} else {
			m["input"] = map[string]interface{}{}
		}
		return json.Marshal(m)
	}
	
	// For other types, use default marshaling
	return json.Marshal(Alias(c))
}

type anthropicTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"input_schema"`
}

type anthropicResponse struct {
	ID         string                `json:"id"`
	Type       string                `json:"type"`
	Role       string                `json:"role"`
	Content    []anthropicMsgContent `json:"content"`
	Model      string                `json:"model"`
	StopReason string                `json:"stop_reason"`
	Usage      anthropicUsage        `json:"usage"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Streaming event types for Anthropic/Claude
type anthropicStreamEvent struct {
	Type    string                  `json:"type"`
	Message *anthropicStreamMessage `json:"message,omitempty"`
	Delta   *anthropicStreamDelta   `json:"delta,omitempty"`
	Index   int                     `json:"index,omitempty"`
	Usage   *anthropicStreamUsage   `json:"usage,omitempty"`
}

type anthropicStreamMessage struct {
	ID           string                `json:"id"`
	Type         string                `json:"type"`
	Role         string                `json:"role"`
	Content      []anthropicMsgContent `json:"content"`
	Model        string                `json:"model"`
	StopReason   *string               `json:"stop_reason"`
	StopSequence *string               `json:"stop_sequence"`
	Usage        *anthropicStreamUsage `json:"usage"`
}

type anthropicStreamDelta struct {
	Type       string `json:"type"`
	Text       string `json:"text"`
	StopReason string `json:"stop_reason,omitempty"`
}

type anthropicStreamUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// anthropicStream implements streaming for Anthropic/Claude models
func (c *BedrockClient) anthropicStream(ctx context.Context, req *domain.ChatRequest, modelID string) (<-chan domain.StreamEvent, error) {
	if !c.useSDKStreaming {
		return c.anthropicSimulatedStream(ctx, req, modelID)
	}

	eventChan := make(chan domain.StreamEvent, 256)

	anthropicReq := c.buildAnthropicRequest(req)
	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	input := &bedrockruntime.InvokeModelWithResponseStreamInput{
		ModelId:     aws.String(modelID),
		ContentType: aws.String("application/json"),
		Body:        body,
	}

	go func() {
		defer close(eventChan)

		startTime := time.Now()

		output, err := c.runtimeClient.InvokeModelWithResponseStream(ctx, input)
		if err != nil {
			fmt.Printf("[ANTHROPIC ERROR] Failed to invoke model %s: %v\n", modelID, err)
			eventChan <- domain.PolicyViolationEvent{
				Message:  fmt.Sprintf("Failed to invoke model %s: %v", modelID, err),
				Severity: "critical",
			}
			eventChan <- domain.FinishEvent{Reason: domain.FinishReasonError}
			return
		}

		apiLatency := time.Since(startTime)
		if apiLatency > time.Second {
			fmt.Printf("[ANTHROPIC PERF] API call latency: %v (model: %s)\n", apiLatency, modelID)
		}

		stream := output.GetStream()
		defer stream.Close()

		var inputTokens, outputTokens int
		firstTokenReceived := false

		for event := range stream.Events() {
			switch v := event.(type) {
			case *types.ResponseStreamMemberChunk:
				var streamEvent anthropicStreamEvent
				if err := json.Unmarshal(v.Value.Bytes, &streamEvent); err != nil {
					continue
				}

				switch streamEvent.Type {
				case "content_block_delta":
					if streamEvent.Delta != nil && streamEvent.Delta.Text != "" {
						if !firstTokenReceived {
							ttft := time.Since(startTime)
							if ttft > time.Second {
								fmt.Printf("[ANTHROPIC PERF] Time to first token: %v\n", ttft)
							}
							firstTokenReceived = true
						}
						eventChan <- domain.TextChunk{Content: streamEvent.Delta.Text}
					}
				case "message_start":
					if streamEvent.Message != nil && streamEvent.Message.Usage != nil {
						inputTokens = streamEvent.Message.Usage.InputTokens
					}
				case "message_stop":
					goto streamDone
				case "message_delta":
					if streamEvent.Usage != nil {
						outputTokens = streamEvent.Usage.OutputTokens
					}
				}
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

		eventChan <- domain.UsageEvent{
			PromptTokens:     int32(inputTokens),
			CompletionTokens: int32(outputTokens),
			TotalTokens:      int32(inputTokens + outputTokens),
		}

		eventChan <- domain.FinishEvent{Reason: domain.FinishReasonStop}
	}()

	return eventChan, nil
}

// anthropicSimulatedStream uses non-streaming endpoint and simulates streaming
func (c *BedrockClient) anthropicSimulatedStream(ctx context.Context, req *domain.ChatRequest, modelID string) (<-chan domain.StreamEvent, error) {
	eventChan := make(chan domain.StreamEvent, 100)

	anthropicReq := c.buildAnthropicRequest(req)
	body, _ := json.Marshal(anthropicReq)

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
		var anthropicResp anthropicResponse
		if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
			eventChan <- domain.PolicyViolationEvent{
				Message:  fmt.Sprintf("Failed to decode response: %v", err),
				Severity: "critical",
			}
			eventChan <- domain.FinishEvent{Reason: domain.FinishReasonError}
			return
		}

		// Process response content - handle both text and tool_use
		var toolCalls []domain.ToolCall
		for _, content := range anthropicResp.Content {
			switch content.Type {
			case "text":
				if content.Text != "" {
					text := content.Text
					chunkSize := 20
					for i := 0; i < len(text); i += chunkSize {
						end := i + chunkSize
						if end > len(text) {
							end = len(text)
						}
						eventChan <- domain.TextChunk{Content: text[i:end]}
					}
				}
			case "tool_use":
				toolCalls = append(toolCalls, domain.ToolCall{
					ID:   content.ID,
					Type: "function",
					Function: domain.FunctionCall{
						Name:      content.Name,
						Arguments: content.Input,
					},
				})
			}
		}
		
		// Emit tool call events
		for _, tc := range toolCalls {
			eventChan <- domain.ToolCallEvent{ToolCall: tc}
		}

		eventChan <- domain.UsageEvent{
			PromptTokens:     int32(anthropicResp.Usage.InputTokens),
			CompletionTokens: int32(anthropicResp.Usage.OutputTokens),
			TotalTokens:      int32(anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens),
		}

		var reason domain.FinishReason
		switch anthropicResp.StopReason {
		case "end_turn":
			reason = domain.FinishReasonStop
		case "max_tokens":
			reason = domain.FinishReasonLength
		case "tool_use":
			reason = domain.FinishReasonToolCalls
		default:
			reason = domain.FinishReasonStop
		}
		eventChan <- domain.FinishEvent{Reason: reason}
	}()

	return eventChan, nil
}

// buildAnthropicRequest converts domain request to Anthropic/Claude format
func (c *BedrockClient) buildAnthropicRequest(req *domain.ChatRequest) anthropicRequest {
	anthropicReq := anthropicRequest{
		AnthropicVersion: "bedrock-2023-05-31",
		MaxTokens:        4096,
	}

	if req.MaxTokens != nil {
		anthropicReq.MaxTokens = int(*req.MaxTokens)
	}

	if req.Temperature != nil {
		anthropicReq.Temperature = req.Temperature
	}

	if req.SystemPrompt != "" {
		anthropicReq.System = req.SystemPrompt
	}

	for _, msg := range req.Messages {
		aMsg := anthropicMsg{
			Role:    msg.Role,
			Content: []anthropicMsgContent{},
		}
		
		// Handle text content (only if non-empty)
		for _, content := range msg.Content {
			if (content.Type == "text" || content.Type == "") && content.Text != "" {
				aMsg.Content = append(aMsg.Content, anthropicMsgContent{
					Type: "text",
					Text: content.Text,
				})
			}
		}
		
		// Handle tool calls from assistant messages
		if msg.Role == "assistant" && len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				// Ensure input is never nil (Anthropic requires the field)
				input := tc.Function.Arguments
				if input == nil {
					input = make(map[string]interface{})
				}
				aMsg.Content = append(aMsg.Content, anthropicMsgContent{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: input,
				})
			}
		}
		
		// Handle tool results
		if msg.Role == "tool" && msg.ToolCallID != "" {
			// Get result text
			var resultText string
			for _, content := range msg.Content {
				if content.Text != "" {
					resultText = content.Text
					break
				}
			}
			// Tool results go in a user message for Anthropic
			aMsg = anthropicMsg{
				Role: "user",
				Content: []anthropicMsgContent{
					{
						Type:      "tool_result",
						ToolUseID: msg.ToolCallID,
						Content:   resultText,
					},
				},
			}
		}
		
		if len(aMsg.Content) > 0 {
			anthropicReq.Messages = append(anthropicReq.Messages, aMsg)
		}
	}

	if req.Prompt != "" && len(anthropicReq.Messages) == 0 {
		anthropicReq.Messages = append(anthropicReq.Messages, anthropicMsg{
			Role: "user",
			Content: []anthropicMsgContent{{
				Type: "text",
				Text: req.Prompt,
			}},
		})
	}

	for _, tool := range req.Tools {
		anthropicReq.Tools = append(anthropicReq.Tools, anthropicTool{
			Name:        tool.Function.Name,
			Description: tool.Function.Description,
			InputSchema: tool.Function.Parameters,
		})
	}

	return anthropicReq
}

// anthropicComplete performs a non-streaming chat completion for Claude
func (c *BedrockClient) anthropicComplete(ctx context.Context, req *domain.ChatRequest, modelID string) (*domain.ChatResponse, error) {
	// Use SDK Converse API if available (IAM credentials)
	if c.useSDKStreaming && c.runtimeClient != nil {
		return c.anthropicConverseComplete(ctx, req, modelID)
	}

	// Fallback to HTTP with Bearer token
	anthropicReq := c.buildAnthropicRequest(req)
	body, _ := json.Marshal(anthropicReq)

	endpoint := c.getBedrockEndpoint()
	url := fmt.Sprintf("%s/model/%s/invoke", endpoint, modelID)

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
	var anthropicResp anthropicResponse
	if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
		return nil, err
	}

	response := &domain.ChatResponse{
		Model: req.Model,
		Usage: &domain.UsageEvent{
			PromptTokens:     int32(anthropicResp.Usage.InputTokens),
			CompletionTokens: int32(anthropicResp.Usage.OutputTokens),
			TotalTokens:      int32(anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens),
		},
	}

	// Process response content - handle both text and tool_use
	var textContent strings.Builder
	var toolCalls []domain.ToolCall
	
	for _, content := range anthropicResp.Content {
		switch content.Type {
		case "text":
			textContent.WriteString(content.Text)
		case "tool_use":
			toolCalls = append(toolCalls, domain.ToolCall{
				ID:   content.ID,
				Type: "function",
				Function: domain.FunctionCall{
					Name:      content.Name,
					Arguments: content.Input,
				},
			})
		}
	}
	
	response.Content = textContent.String()
	response.ToolCalls = toolCalls

	switch anthropicResp.StopReason {
	case "end_turn":
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

// anthropicConverseComplete uses the SDK's Converse API for non-streaming requests
// This properly handles IAM authentication and supports tool calling
func (c *BedrockClient) anthropicConverseComplete(ctx context.Context, req *domain.ChatRequest, modelID string) (*domain.ChatResponse, error) {
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
