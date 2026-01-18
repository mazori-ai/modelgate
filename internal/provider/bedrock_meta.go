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
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"

	"modelgate/internal/domain"
)

// Meta/Llama request format
type metaRequest struct {
	Prompt      string  `json:"prompt"`
	MaxGenLen   int     `json:"max_gen_len,omitempty"`
	Temperature float32 `json:"temperature,omitempty"`
	TopP        float32 `json:"top_p,omitempty"`
}

// Meta/Llama response format
type metaResponse struct {
	Generation           string `json:"generation"`
	PromptTokenCount     int    `json:"prompt_token_count"`
	GenerationTokenCount int    `json:"generation_token_count"`
	StopReason           string `json:"stop_reason"`
}

// Meta/Llama streaming event format
type metaStreamEvent struct {
	Generation           string `json:"generation,omitempty"`
	PromptTokenCount     int    `json:"prompt_token_count,omitempty"`
	GenerationTokenCount int    `json:"generation_token_count,omitempty"`
	StopReason           string `json:"stop_reason,omitempty"`
}

// metaStream implements streaming for Meta/Llama models
func (c *BedrockClient) metaStream(ctx context.Context, req *domain.ChatRequest, modelID string) (<-chan domain.StreamEvent, error) {
	if !c.useSDKStreaming {
		return c.metaSimulatedStream(ctx, req, modelID)
	}

	eventChan := make(chan domain.StreamEvent, 256)

	metaReq := c.buildMetaRequest(req)
	body, err := json.Marshal(metaReq)
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
			fmt.Printf("[META ERROR] Failed to invoke model %s: %v\n", modelID, err)
			eventChan <- domain.PolicyViolationEvent{
				Message:  fmt.Sprintf("Failed to invoke model %s: %v", modelID, err),
				Severity: "critical",
			}
			eventChan <- domain.FinishEvent{Reason: domain.FinishReasonError}
			return
		}

		apiLatency := time.Since(startTime)
		if apiLatency > time.Second {
			fmt.Printf("[META PERF] API call latency: %v (model: %s)\n", apiLatency, modelID)
		}

		stream := output.GetStream()
		defer stream.Close()

		var inputTokens, outputTokens int
		firstTokenReceived := false

		for event := range stream.Events() {
			switch v := event.(type) {
			case *types.ResponseStreamMemberChunk:
				var metaEvent metaStreamEvent
				if err := json.Unmarshal(v.Value.Bytes, &metaEvent); err != nil {
					continue
				}

				if metaEvent.Generation != "" {
					if !firstTokenReceived {
						ttft := time.Since(startTime)
						if ttft > time.Second {
							fmt.Printf("[META PERF] Time to first token: %v\n", ttft)
						}
						firstTokenReceived = true
					}
					eventChan <- domain.TextChunk{Content: metaEvent.Generation}
				}

				if metaEvent.PromptTokenCount > 0 {
					inputTokens = metaEvent.PromptTokenCount
				}
				if metaEvent.GenerationTokenCount > 0 {
					outputTokens = metaEvent.GenerationTokenCount
				}

				if metaEvent.StopReason != "" {
					goto streamDone
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

// metaSimulatedStream uses non-streaming endpoint and simulates streaming
func (c *BedrockClient) metaSimulatedStream(ctx context.Context, req *domain.ChatRequest, modelID string) (<-chan domain.StreamEvent, error) {
	eventChan := make(chan domain.StreamEvent, 100)

	metaReq := c.buildMetaRequest(req)
	body, _ := json.Marshal(metaReq)

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
		var metaResp metaResponse
		if err := json.Unmarshal(respBody, &metaResp); err != nil {
			eventChan <- domain.PolicyViolationEvent{
				Message:  fmt.Sprintf("Failed to decode response: %v", err),
				Severity: "critical",
			}
			eventChan <- domain.FinishEvent{Reason: domain.FinishReasonError}
			return
		}

		// Simulate streaming
		text := metaResp.Generation
		chunkSize := 20
		for i := 0; i < len(text); i += chunkSize {
			end := i + chunkSize
			if end > len(text) {
				end = len(text)
			}
			eventChan <- domain.TextChunk{Content: text[i:end]}
		}

		eventChan <- domain.UsageEvent{
			PromptTokens:     int32(metaResp.PromptTokenCount),
			CompletionTokens: int32(metaResp.GenerationTokenCount),
			TotalTokens:      int32(metaResp.PromptTokenCount + metaResp.GenerationTokenCount),
		}

		eventChan <- domain.FinishEvent{Reason: domain.FinishReasonStop}
	}()

	return eventChan, nil
}

// buildMetaRequest converts domain request to Meta/Llama format
func (c *BedrockClient) buildMetaRequest(req *domain.ChatRequest) metaRequest {
	metaReq := metaRequest{
		MaxGenLen:   2048,
		Temperature: 0.7,
		TopP:        0.9,
	}

	if req.MaxTokens != nil {
		metaReq.MaxGenLen = int(*req.MaxTokens)
	}

	if req.Temperature != nil {
		metaReq.Temperature = *req.Temperature
	}

	// Build prompt from messages (Meta uses Llama 3 format)
	var prompt strings.Builder

	// Add system prompt if provided
	if req.SystemPrompt != "" {
		prompt.WriteString("<|begin_of_text|><|start_header_id|>system<|end_header_id|>\n\n")
		prompt.WriteString(req.SystemPrompt)
		prompt.WriteString("<|eot_id|>")
	} else {
		prompt.WriteString("<|begin_of_text|>")
	}

	// Convert messages to Llama format
	for _, msg := range req.Messages {
		prompt.WriteString("<|start_header_id|>")
		prompt.WriteString(msg.Role)
		prompt.WriteString("<|end_header_id|>\n\n")
		for _, content := range msg.Content {
			if content.Type == "text" || content.Type == "" {
				prompt.WriteString(content.Text)
			}
		}
		prompt.WriteString("<|eot_id|>")
	}

	// Handle simple prompt
	if req.Prompt != "" && len(req.Messages) == 0 {
		prompt.WriteString("<|start_header_id|>user<|end_header_id|>\n\n")
		prompt.WriteString(req.Prompt)
		prompt.WriteString("<|eot_id|>")
	}

	// Add assistant header for response
	prompt.WriteString("<|start_header_id|>assistant<|end_header_id|>\n\n")

	metaReq.Prompt = prompt.String()
	return metaReq
}

// metaComplete performs a non-streaming chat completion for Llama
func (c *BedrockClient) metaComplete(ctx context.Context, req *domain.ChatRequest, modelID string) (*domain.ChatResponse, error) {
	metaReq := c.buildMetaRequest(req)
	body, _ := json.Marshal(metaReq)

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
	var metaResp metaResponse
	if err := json.Unmarshal(respBody, &metaResp); err != nil {
		return nil, err
	}

	response := &domain.ChatResponse{
		Model:   req.Model,
		Content: metaResp.Generation,
		Usage: &domain.UsageEvent{
			PromptTokens:     int32(metaResp.PromptTokenCount),
			CompletionTokens: int32(metaResp.GenerationTokenCount),
			TotalTokens:      int32(metaResp.PromptTokenCount + metaResp.GenerationTokenCount),
		},
	}

	switch metaResp.StopReason {
	case "stop", "end":
		response.FinishReason = domain.FinishReasonStop
	case "length":
		response.FinishReason = domain.FinishReasonLength
	default:
		response.FinishReason = domain.FinishReasonStop
	}

	return response, nil
}
