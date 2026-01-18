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

// Mistral request format
type mistralRequest struct {
	Prompt      string  `json:"prompt"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
	Temperature float32 `json:"temperature,omitempty"`
	TopP        float32 `json:"top_p,omitempty"`
	TopK        int     `json:"top_k,omitempty"`
}

// Mistral response format
type mistralResponse struct {
	Outputs []struct {
		Text       string `json:"text"`
		StopReason string `json:"stop_reason"`
	} `json:"outputs"`
}

// Mistral streaming event format
type mistralStreamEvent struct {
	Outputs []struct {
		Text       string `json:"text"`
		StopReason string `json:"stop_reason,omitempty"`
	} `json:"outputs,omitempty"`
}

// mistralStream implements streaming for Mistral models
func (c *BedrockClient) mistralStream(ctx context.Context, req *domain.ChatRequest, modelID string) (<-chan domain.StreamEvent, error) {
	if !c.useSDKStreaming {
		return c.mistralSimulatedStream(ctx, req, modelID)
	}

	eventChan := make(chan domain.StreamEvent, 256)

	mistralReq := c.buildMistralRequest(req)
	body, err := json.Marshal(mistralReq)
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
			fmt.Printf("[MISTRAL ERROR] Failed to invoke model %s: %v\n", modelID, err)
			eventChan <- domain.PolicyViolationEvent{
				Message:  fmt.Sprintf("Failed to invoke model %s: %v", modelID, err),
				Severity: "critical",
			}
			eventChan <- domain.FinishEvent{Reason: domain.FinishReasonError}
			return
		}

		apiLatency := time.Since(startTime)
		if apiLatency > time.Second {
			fmt.Printf("[MISTRAL PERF] API call latency: %v (model: %s)\n", apiLatency, modelID)
		}

		stream := output.GetStream()
		defer stream.Close()

		var outputTokens int
		firstTokenReceived := false

		for event := range stream.Events() {
			switch v := event.(type) {
			case *types.ResponseStreamMemberChunk:
				var mistralEvent mistralStreamEvent
				if err := json.Unmarshal(v.Value.Bytes, &mistralEvent); err != nil {
					continue
				}

				for _, output := range mistralEvent.Outputs {
					if output.Text != "" {
						if !firstTokenReceived {
							ttft := time.Since(startTime)
							if ttft > time.Second {
								fmt.Printf("[MISTRAL PERF] Time to first token: %v\n", ttft)
							}
							firstTokenReceived = true
						}
						eventChan <- domain.TextChunk{Content: output.Text}
						outputTokens++
					}

					if output.StopReason != "" {
						goto streamDone
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

		// Mistral doesn't provide token counts in streaming, estimate
		eventChan <- domain.UsageEvent{
			PromptTokens:     0, // Not available in streaming
			CompletionTokens: int32(outputTokens),
			TotalTokens:      int32(outputTokens),
		}

		eventChan <- domain.FinishEvent{Reason: domain.FinishReasonStop}
	}()

	return eventChan, nil
}

// mistralSimulatedStream uses non-streaming endpoint and simulates streaming
func (c *BedrockClient) mistralSimulatedStream(ctx context.Context, req *domain.ChatRequest, modelID string) (<-chan domain.StreamEvent, error) {
	eventChan := make(chan domain.StreamEvent, 100)

	mistralReq := c.buildMistralRequest(req)
	body, _ := json.Marshal(mistralReq)

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
		var mistralResp mistralResponse
		if err := json.Unmarshal(respBody, &mistralResp); err != nil {
			eventChan <- domain.PolicyViolationEvent{
				Message:  fmt.Sprintf("Failed to decode response: %v", err),
				Severity: "critical",
			}
			eventChan <- domain.FinishEvent{Reason: domain.FinishReasonError}
			return
		}

		// Simulate streaming
		if len(mistralResp.Outputs) > 0 {
			text := mistralResp.Outputs[0].Text
			chunkSize := 20
			for i := 0; i < len(text); i += chunkSize {
				end := i + chunkSize
				if end > len(text) {
					end = len(text)
				}
				eventChan <- domain.TextChunk{Content: text[i:end]}
			}
		}

		// Mistral doesn't provide token counts
		eventChan <- domain.UsageEvent{
			PromptTokens:     0,
			CompletionTokens: 0,
			TotalTokens:      0,
		}

		eventChan <- domain.FinishEvent{Reason: domain.FinishReasonStop}
	}()

	return eventChan, nil
}

// buildMistralRequest converts domain request to Mistral format
func (c *BedrockClient) buildMistralRequest(req *domain.ChatRequest) mistralRequest {
	mistralReq := mistralRequest{
		MaxTokens:   2048,
		Temperature: 0.7,
		TopP:        0.9,
	}

	if req.MaxTokens != nil {
		mistralReq.MaxTokens = int(*req.MaxTokens)
	}

	if req.Temperature != nil {
		mistralReq.Temperature = *req.Temperature
	}

	// Build prompt from messages (Mistral uses [INST] format)
	var prompt strings.Builder

	// Add system prompt if provided
	if req.SystemPrompt != "" {
		prompt.WriteString("<s>[INST] ")
		prompt.WriteString(req.SystemPrompt)
		prompt.WriteString("\n\n")
	} else {
		prompt.WriteString("<s>[INST] ")
	}

	// Convert messages to Mistral format
	for i, msg := range req.Messages {
		if msg.Role == "user" {
			if i > 0 {
				prompt.WriteString("[INST] ")
			}
			for _, content := range msg.Content {
				if content.Type == "text" || content.Type == "" {
					prompt.WriteString(content.Text)
				}
			}
			prompt.WriteString(" [/INST]")
		} else if msg.Role == "assistant" {
			for _, content := range msg.Content {
				if content.Type == "text" || content.Type == "" {
					prompt.WriteString(content.Text)
				}
			}
			prompt.WriteString("</s>")
		}
	}

	// Handle simple prompt
	if req.Prompt != "" && len(req.Messages) == 0 {
		prompt.WriteString(req.Prompt)
		prompt.WriteString(" [/INST]")
	}

	mistralReq.Prompt = prompt.String()
	return mistralReq
}

// mistralComplete performs a non-streaming chat completion for Mistral
func (c *BedrockClient) mistralComplete(ctx context.Context, req *domain.ChatRequest, modelID string) (*domain.ChatResponse, error) {
	mistralReq := c.buildMistralRequest(req)
	body, _ := json.Marshal(mistralReq)

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
	var mistralResp mistralResponse
	if err := json.Unmarshal(respBody, &mistralResp); err != nil {
		return nil, err
	}

	response := &domain.ChatResponse{
		Model: req.Model,
	}

	if len(mistralResp.Outputs) > 0 {
		response.Content = mistralResp.Outputs[0].Text
		switch mistralResp.Outputs[0].StopReason {
		case "stop", "end":
			response.FinishReason = domain.FinishReasonStop
		case "length":
			response.FinishReason = domain.FinishReasonLength
		default:
			response.FinishReason = domain.FinishReasonStop
		}
	}

	return response, nil
}
