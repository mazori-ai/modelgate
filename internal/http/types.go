// Package http provides the OpenAI-compatible HTTP API server.
package http

// =============================================================================
// OpenAI-Compatible Request/Response Types
// =============================================================================

// ChatCompletionRequest is the OpenAI-compatible chat completion request
type ChatCompletionRequest struct {
	Model            string        `json:"model"`
	Messages         []ChatMessage `json:"messages"`
	Temperature      *float32      `json:"temperature,omitempty"`
	MaxTokens        *int32        `json:"max_tokens,omitempty"`
	Stream           bool          `json:"stream,omitempty"`
	Tools            []Tool        `json:"tools,omitempty"`
	ToolChoice       interface{}   `json:"tool_choice,omitempty"`
	ResponseFormat   interface{}   `json:"response_format,omitempty"`
	ReasoningEffort  *string       `json:"reasoning_effort,omitempty"`
	N                *int          `json:"n,omitempty"`
	Stop             interface{}   `json:"stop,omitempty"`
	PresencePenalty  *float32      `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float32      `json:"frequency_penalty,omitempty"`
	User             *string       `json:"user,omitempty"`
}

// ChatMessage represents a message in the conversation
type ChatMessage struct {
	Role             string      `json:"role"`
	Content          interface{} `json:"content"` // string or []ContentPart
	Name             *string     `json:"name,omitempty"`
	ToolCalls        []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID       string      `json:"tool_call_id,omitempty"`
	ReasoningContent *string     `json:"reasoning_content,omitempty"`
}

// Tool represents a tool definition
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction represents a function definition
type ToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ToolCall represents a tool call from the assistant
type ToolCall struct {
	ID       string        `json:"id"`
	Type     string        `json:"type"`
	Function *FunctionCall `json:"function,omitempty"`
}

// FunctionCall represents a function call
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatCompletionResponse is the OpenAI-compatible chat completion response
type ChatCompletionResponse struct {
	ID                string   `json:"id"`
	Object            string   `json:"object"`
	Created           int64    `json:"created"`
	Model             string   `json:"model"`
	Choices           []Choice `json:"choices"`
	Usage             *Usage   `json:"usage,omitempty"`
	SystemFingerprint *string  `json:"system_fingerprint,omitempty"`
}

// Choice represents a completion choice
type Choice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason,omitempty"`
	Logprobs     interface{} `json:"logprobs,omitempty"`
}

// Usage represents token usage
type Usage struct {
	PromptTokens     int32 `json:"prompt_tokens"`
	CompletionTokens int32 `json:"completion_tokens"`
	TotalTokens      int32 `json:"total_tokens"`
}

// ChatCompletionChunk is a streaming response chunk
type ChatCompletionChunk struct {
	ID                string        `json:"id"`
	Object            string        `json:"object"`
	Created           int64         `json:"created"`
	Model             string        `json:"model"`
	Choices           []ChunkChoice `json:"choices"`
	SystemFingerprint *string       `json:"system_fingerprint,omitempty"`
}

// ChunkChoice represents a streaming chunk choice
type ChunkChoice struct {
	Index        int     `json:"index"`
	Delta        Delta   `json:"delta"`
	FinishReason *string `json:"finish_reason,omitempty"`
}

// Delta represents the delta in a streaming chunk
type Delta struct {
	Role      *string    `json:"role,omitempty"`
	Content   *string    `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// =============================================================================
// Embeddings Types
// =============================================================================

// EmbeddingsRequest is the OpenAI-compatible embeddings request
type EmbeddingsRequest struct {
	Model          string      `json:"model"`
	Input          interface{} `json:"input"` // string or []string
	EncodingFormat *string     `json:"encoding_format,omitempty"`
	Dimensions     *int32      `json:"dimensions,omitempty"`
	User           *string     `json:"user,omitempty"`
}

// EmbeddingsResponse is the OpenAI-compatible embeddings response
type EmbeddingsResponse struct {
	Object string          `json:"object"`
	Data   []EmbeddingData `json:"data"`
	Model  string          `json:"model"`
	Usage  EmbeddingUsage  `json:"usage"`
}

// EmbeddingData represents a single embedding
type EmbeddingData struct {
	Object    string    `json:"object"`
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}

// EmbeddingUsage represents token usage for embeddings
type EmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// =============================================================================
// Models Types
// =============================================================================

// ModelsResponse is the OpenAI-compatible models response
type ModelsResponse struct {
	Object string      `json:"object"`
	Data   []ModelData `json:"data"`
}

// ModelData represents model information
type ModelData struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// =============================================================================
// Error Types
// =============================================================================

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail represents error details
type ErrorDetail struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
	Param   string `json:"param,omitempty"`
}
