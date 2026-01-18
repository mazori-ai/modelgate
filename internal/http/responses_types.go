// Package http provides HTTP types for the responses API endpoint.
package http

// ResponsesRequest is the HTTP request for POST /v1/responses
type ResponsesRequest struct {
	Model          string               `json:"model"`
	Messages       []ChatMessage        `json:"messages"` // Reuse ChatMessage from types.go
	ResponseSchema ResponsesSchemaInput `json:"response_schema"`
	Temperature    *float32             `json:"temperature,omitempty"`
	MaxTokens      *int32               `json:"max_tokens,omitempty"`
	TopP           *float32             `json:"top_p,omitempty"`
	User           *string              `json:"user,omitempty"`
}

// ResponsesSchemaInput defines the JSON schema for the request
type ResponsesSchemaInput struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Schema      map[string]interface{} `json:"schema"`
	Strict      bool                   `json:"strict,omitempty"`
}

// ResponsesResponse is the HTTP response for /v1/responses
type ResponsesResponse struct {
	ID       string                 `json:"id"`
	Object   string                 `json:"object"`
	Created  int64                  `json:"created"`
	Model    string                 `json:"model"`
	Response map[string]interface{} `json:"response"`
	Usage    ResponsesUsageOutput   `json:"usage"`
}

// ResponsesUsageOutput represents token usage in the response
type ResponsesUsageOutput struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
