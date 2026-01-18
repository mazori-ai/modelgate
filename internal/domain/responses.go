// Package domain defines response API domain types.
package domain

// ResponseRequest represents a structured output request for the /v1/responses endpoint
type ResponseRequest struct {
	Model          string         `json:"model"`
	Messages       []Message      `json:"messages"`
	ResponseSchema ResponseSchema `json:"response_schema"`
	Temperature    *float32       `json:"temperature,omitempty"`
	MaxTokens      *int32         `json:"max_tokens,omitempty"`
	TopP           *float32       `json:"top_p,omitempty"`

	// Request context (internal)
	RequestID string `json:"request_id,omitempty"`
	APIKeyID  string `json:"api_key_id,omitempty"`
	RoleID    string `json:"role_id,omitempty"`
	GroupID   string `json:"group_id,omitempty"`
}

// ResponseSchema defines the expected JSON schema for structured outputs
type ResponseSchema struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Schema      map[string]interface{} `json:"schema"`
	Strict      bool                   `json:"strict,omitempty"`
}

// StructuredResponse represents a structured output response from /v1/responses endpoint
type StructuredResponse struct {
	ID       string                 `json:"id"`
	Object   string                 `json:"object"` // "response"
	Created  int64                  `json:"created"`
	Model    string                 `json:"model"`
	Response map[string]interface{} `json:"response"` // Parsed JSON response
	Usage    ResponseUsage          `json:"usage"`
	Metadata *ResponseMetadata      `json:"metadata,omitempty"`
}

// ResponseMetadata contains additional info about how the response was generated
type ResponseMetadata struct {
	Provider           string `json:"provider"`
	ImplementationMode string `json:"implementation_mode"` // "native", "json_mode", "prompt_based"
	SchemaValidated    bool   `json:"schema_validated"`
	RetryCount         int    `json:"retry_count,omitempty"`
}

// ResponseUsage represents token usage for responses API
type ResponseUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
