// Package domain defines core domain types for the ModelGate LLM gateway.
package domain

import (
	"context"
	"time"
)

// =============================================================================
// Provider Types
// =============================================================================

// Provider represents an LLM provider
type Provider string

const (
	ProviderGemini      Provider = "gemini"
	ProviderAnthropic   Provider = "anthropic"
	ProviderOpenAI      Provider = "openai"
	ProviderBedrock     Provider = "bedrock"
	ProviderOllama      Provider = "ollama"
	ProviderAzureOpenAI Provider = "azure_openai"
	ProviderGroq        Provider = "groq"
	ProviderMistral     Provider = "mistral"
	ProviderTogether    Provider = "together"
	ProviderCohere      Provider = "cohere"
)

// AllProviders returns all supported providers
func AllProviders() []Provider {
	return []Provider{
		ProviderGemini,
		ProviderAnthropic,
		ProviderOpenAI,
		ProviderBedrock,
		ProviderOllama,
		ProviderAzureOpenAI,
		ProviderGroq,
		ProviderMistral,
		ProviderTogether,
		ProviderCohere,
	}
}

// ParseProvider parses a provider string
func ParseProvider(s string) (Provider, bool) {
	switch s {
	case "gemini", "google":
		return ProviderGemini, true
	case "anthropic", "claude":
		return ProviderAnthropic, true
	case "openai", "gpt":
		return ProviderOpenAI, true
	case "bedrock", "aws", "aws-bedrock", "aws_bedrock":
		return ProviderBedrock, true
	case "ollama", "local":
		return ProviderOllama, true
	case "azure_openai", "azure", "azureopenai":
		return ProviderAzureOpenAI, true
	case "groq":
		return ProviderGroq, true
	case "mistral", "mistralai":
		return ProviderMistral, true
	case "together", "togetherai":
		return ProviderTogether, true
	case "cohere":
		return ProviderCohere, true
	default:
		return "", false
	}
}

// =============================================================================
// Model Types
// =============================================================================

// ModelInfo contains metadata about an LLM model
type ModelInfo struct {
	ID                string   `json:"id" yaml:"id"`
	Name              string   `json:"name" yaml:"name"`
	Provider          Provider `json:"provider" yaml:"provider"`
	SupportsTools     bool     `json:"supports_tools" yaml:"supports_tools"`
	SupportsReasoning bool     `json:"supports_reasoning" yaml:"supports_reasoning"`
	ContextLimit      uint32   `json:"context_limit" yaml:"context_limit"`
	OutputLimit       uint32   `json:"output_limit" yaml:"output_limit"`
	InputCostPer1M    float64  `json:"input_cost_per_1m" yaml:"input_cost_per_1m"`
	OutputCostPer1M   float64  `json:"output_cost_per_1m" yaml:"output_cost_per_1m"`
	Enabled           bool     `json:"enabled" yaml:"enabled"`
	// NativeModelID is the full provider-specific model ID for API calls.
	// For Bedrock, this is the full inference profile ID (e.g., "us.anthropic.claude-3-5-sonnet-20241022-v2:0").
	// For other providers, this may be the same as ID or empty.
	NativeModelID string `json:"native_model_id,omitempty" yaml:"native_model_id,omitempty"`
}

// =============================================================================
// Chat Types
// =============================================================================

// ChatRequest represents a chat completion request
type ChatRequest struct {
	Model            string           `json:"model"`
	Prompt           string           `json:"prompt"`
	Messages         []Message        `json:"messages"`
	SystemPrompt     string           `json:"system_prompt,omitempty"`
	Temperature      *float32         `json:"temperature,omitempty"`
	MaxTokens        *int32           `json:"max_tokens,omitempty"`
	Tools            []Tool           `json:"tools,omitempty"`
	ToolChoice       *ToolChoice      `json:"tool_choice,omitempty"`
	ReasoningConfig  *ReasoningConfig `json:"reasoning_config,omitempty"`
	Documents        []Document       `json:"documents,omitempty"`
	AdditionalParams map[string]any   `json:"additional_params,omitempty"`
	Streaming        bool             `json:"stream,omitempty"` // Whether to stream the response

	// Request context
	RequestID string `json:"request_id,omitempty"`

	// API Key context (for RBAC)
	APIKeyID string `json:"api_key_id,omitempty"`
	RoleID   string `json:"role_id,omitempty"`  // Single role (if API key assigned to a role)
	GroupID  string `json:"group_id,omitempty"` // Group (if API key assigned to a group)
}

// Message represents a chat message
type Message struct {
	Role           string          `json:"role"`
	Content        []ContentBlock  `json:"content"`
	ToolCalls      []ToolCall      `json:"tool_calls,omitempty"`
	ToolCallID     string          `json:"tool_call_id,omitempty"`
	ReasoningSteps []ReasoningStep `json:"reasoning_steps,omitempty"`
}

// ContentBlock represents a content block in a message
type ContentBlock struct {
	Type       string      `json:"type"` // "text", "image", "tool_result"
	Text       string      `json:"text,omitempty"`
	ImageURL   string      `json:"image_url,omitempty"`
	ImageData  []byte      `json:"image_data,omitempty"`
	MediaType  string      `json:"media_type,omitempty"`
	ToolResult *ToolResult `json:"tool_result,omitempty"`
}

// Tool represents a tool/function definition
type Tool struct {
	Type     string             `json:"type"`
	Function FunctionDefinition `json:"function"`
}

// FunctionDefinition defines a function that can be called
type FunctionDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ToolCall represents a tool call from the model
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall represents a function call
type FunctionCall struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// ToolChoice controls how tools are selected
type ToolChoice struct {
	Mode string `json:"mode"` // "auto", "required", "none"
}

// ToolResult represents the result of a tool call
type ToolResult struct {
	ToolCallID string        `json:"tool_call_id"`
	ToolName   string        `json:"tool_name"`
	Result     []ResultBlock `json:"result"`
	IsError    bool          `json:"is_error"`
}

// ResultBlock represents a result block
type ResultBlock struct {
	Type      string         `json:"type"` // "text", "json", "image"
	Text      string         `json:"text,omitempty"`
	JSON      map[string]any `json:"json,omitempty"`
	ImageData string         `json:"image_data,omitempty"`
	MimeType  string         `json:"mime_type,omitempty"`
}

// ReasoningConfig configures extended reasoning mode
type ReasoningConfig struct {
	Enabled         bool  `json:"enabled"`
	BudgetTokens    int32 `json:"budget_tokens,omitempty"`
	IncludeThoughts bool  `json:"include_thoughts,omitempty"`
}

// ReasoningStep represents a step in reasoning
type ReasoningStep struct {
	Content   string `json:"content"`
	Signature string `json:"signature,omitempty"`
}

// Document represents a RAG document
type Document struct {
	ID              string            `json:"id"`
	Text            string            `json:"text"`
	AdditionalProps map[string]string `json:"additional_props,omitempty"`
}

// =============================================================================
// Response Types
// =============================================================================

// StreamEvent represents a streaming event
type StreamEvent interface {
	eventType() string
}

// TextChunk is a text content chunk
type TextChunk struct {
	Content string `json:"content"`
}

func (TextChunk) eventType() string { return "text" }

// ThinkingChunk is a thinking/reasoning chunk
type ThinkingChunk struct {
	Content string `json:"content"`
}

func (ThinkingChunk) eventType() string { return "thinking" }

// ThinkingSignatureChunk is a thinking signature
type ThinkingSignatureChunk struct {
	Signature string `json:"signature"`
}

func (ThinkingSignatureChunk) eventType() string { return "thinking_signature" }

// ToolCallEvent is a complete tool call event
type ToolCallEvent struct {
	ToolCall ToolCall `json:"tool_call"`
}

func (ToolCallEvent) eventType() string { return "tool_call" }

// ToolCallDelta is a partial tool call
type ToolCallDelta struct {
	ID    string `json:"id"`
	Delta string `json:"delta"`
}

func (ToolCallDelta) eventType() string { return "tool_call_delta" }

// UsageEvent contains token usage information
type UsageEvent struct {
	PromptTokens     int32   `json:"prompt_tokens"`
	CompletionTokens int32   `json:"completion_tokens"`
	TotalTokens      int32   `json:"total_tokens"`
	CostUSD          float64 `json:"cost_usd,omitempty"`
}

func (UsageEvent) eventType() string { return "usage" }

// FinishEvent indicates the stream has finished
type FinishEvent struct {
	Reason FinishReason `json:"reason"`
}

func (FinishEvent) eventType() string { return "finish" }

// FinishReason indicates why generation stopped
type FinishReason string

const (
	FinishReasonStop            FinishReason = "stop"
	FinishReasonToolCalls       FinishReason = "tool_calls"
	FinishReasonLength          FinishReason = "length"
	FinishReasonError           FinishReason = "error"
	FinishReasonPolicyViolation FinishReason = "policy_violation"
)

// PolicyViolationEvent indicates a policy violation
type PolicyViolationEvent struct {
	PolicyID      string `json:"policy_id"`
	PolicyName    string `json:"policy_name"`
	ViolationType string `json:"violation_type"`
	Message       string `json:"message"`
	Severity      string `json:"severity"`
}

func (PolicyViolationEvent) eventType() string { return "policy_violation" }

// ChatResponse is the full response for non-streaming
type ChatResponse struct {
	Content      string       `json:"content,omitempty"`
	ToolCalls    []ToolCall   `json:"tool_calls,omitempty"`
	Usage        *UsageEvent  `json:"usage,omitempty"`
	Model        string       `json:"model,omitempty"`
	FinishReason FinishReason `json:"finish_reason,omitempty"`
	Thinking     string       `json:"thinking,omitempty"`
	CostUSD      float64      `json:"cost_usd,omitempty"`
	Cached       bool         `json:"cached,omitempty"`       // True if response was served from cache
	LatencyMs    int64        `json:"latency_ms,omitempty"`   // Request latency in milliseconds
	Provider     Provider     `json:"provider,omitempty"`     // Provider that served the response
}

// =============================================================================
// Tenant Types
// =============================================================================

// TenantStatus represents the status of a tenant
type TenantStatus string

const (
	TenantStatusActive    TenantStatus = "active"
	TenantStatusSuspended TenantStatus = "suspended"
	TenantStatusPending   TenantStatus = "pending"
)

// TenantTier represents the tier of a tenant
type TenantTier string

const (
	TenantTierFree         TenantTier = "free"
	TenantTierStarter      TenantTier = "starter"
	TenantTierProfessional TenantTier = "professional"
	TenantTierEnterprise   TenantTier = "enterprise"
)

// PlanLimits defines resource ceilings per tenant tier
type PlanLimits struct {
	// Connection limits (per provider)
	MaxConnectionsPerProvider int `json:"max_connections_per_provider"`
	MaxIdleConnections        int `json:"max_idle_connections"`

	// Concurrency limits (across all roles combined)
	MaxConcurrentRequests int `json:"max_concurrent_requests"`
	MaxQueuedRequests     int `json:"max_queued_requests"`

	// Resource limits
	MaxRoles     int `json:"max_roles"`
	MaxAPIKeys   int `json:"max_api_keys"`
	MaxProviders int `json:"max_providers"`
}

// DefaultPlanLimits returns the default limits for each tenant tier
var DefaultPlanLimits = map[TenantTier]PlanLimits{
	TenantTierFree: {
		MaxConnectionsPerProvider: 5,
		MaxIdleConnections:        2,
		MaxConcurrentRequests:     2,
		MaxQueuedRequests:         5,
		MaxRoles:                  2,
		MaxAPIKeys:                5,
		MaxProviders:              2,
	},
	TenantTierStarter: {
		MaxConnectionsPerProvider: 20,
		MaxIdleConnections:        10,
		MaxConcurrentRequests:     20,
		MaxQueuedRequests:         50,
		MaxRoles:                  10,
		MaxAPIKeys:                50,
		MaxProviders:              5,
	},
	TenantTierProfessional: {
		MaxConnectionsPerProvider: 50,
		MaxIdleConnections:        25,
		MaxConcurrentRequests:     100,
		MaxQueuedRequests:         500,
		MaxRoles:                  50,
		MaxAPIKeys:                200,
		MaxProviders:              10,
	},
	TenantTierEnterprise: {
		MaxConnectionsPerProvider: 100,
		MaxIdleConnections:        50,
		MaxConcurrentRequests:     500,
		MaxQueuedRequests:         2000,
		MaxRoles:                  -1, // Unlimited
		MaxAPIKeys:                -1, // Unlimited
		MaxProviders:              -1, // Unlimited
	},
}

// GetPlanLimits returns the plan limits for a tenant, considering custom overrides
func GetPlanLimits(tier TenantTier, customLimits *PlanLimits) PlanLimits {
	defaults := DefaultPlanLimits[tier]
	if customLimits == nil {
		return defaults
	}

	// Apply custom overrides (only if positive)
	result := defaults
	if customLimits.MaxConnectionsPerProvider > 0 {
		result.MaxConnectionsPerProvider = customLimits.MaxConnectionsPerProvider
	}
	if customLimits.MaxIdleConnections > 0 {
		result.MaxIdleConnections = customLimits.MaxIdleConnections
	}
	if customLimits.MaxConcurrentRequests > 0 {
		result.MaxConcurrentRequests = customLimits.MaxConcurrentRequests
	}
	if customLimits.MaxQueuedRequests > 0 {
		result.MaxQueuedRequests = customLimits.MaxQueuedRequests
	}
	if customLimits.MaxRoles != 0 {
		result.MaxRoles = customLimits.MaxRoles
	}
	if customLimits.MaxAPIKeys != 0 {
		result.MaxAPIKeys = customLimits.MaxAPIKeys
	}
	if customLimits.MaxProviders != 0 {
		result.MaxProviders = customLimits.MaxProviders
	}
	return result
}

// Tenant represents a customer/organization
type Tenant struct {
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	Email              string            `json:"email"`
	Status             TenantStatus      `json:"status"`
	Tier               TenantTier        `json:"tier"`
	Settings           TenantSettings    `json:"settings"`
	Quotas             TenantQuotas      `json:"quotas"`
	PolicyIDs          []string          `json:"policy_ids"`
	Metadata           map[string]string `json:"metadata"`
	PlanLimitsOverride *PlanLimits       `json:"plan_limits_override,omitempty"` // Custom limits (enterprise only)
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
}

// GetEffectivePlanLimits returns the effective plan limits for this tenant
func (t *Tenant) GetEffectivePlanLimits() PlanLimits {
	return GetPlanLimits(t.Tier, t.PlanLimitsOverride)
}

// TenantSettings contains tenant-specific settings
type TenantSettings struct {
	AllowedProviders    []Provider `json:"allowed_providers"`
	AllowedModels       []string   `json:"allowed_models"`
	MaxCostPerRequest   float64    `json:"max_cost_per_request"`
	MaxTokensPerRequest int32      `json:"max_tokens_per_request"`
	AllowToolCalling    bool       `json:"allow_tool_calling"`
	AllowReasoningMode  bool       `json:"allow_reasoning_mode"`
	MonthlyBudgetUSD    float64    `json:"monthly_budget_usd"`
	RateLimitRPM        int32      `json:"rate_limit_rpm"`
	RateLimitTPM        int32      `json:"rate_limit_tpm"`
}

// RegistrationRequest represents a tenant registration request
type RegistrationRequest struct {
	ID                string    `json:"id"`
	OrganizationName  string    `json:"organization_name"`
	OrganizationEmail string    `json:"organization_email"`
	AdminName         string    `json:"admin_name"`
	AdminEmail        string    `json:"admin_email"`
	AdminPassword     string    `json:"admin_password"`
	Slug              string    `json:"slug"`
	Status            string    `json:"status"` // pending, approved, rejected
	RejectionReason   string    `json:"rejection_reason,omitempty"`
	RequestedAt       time.Time `json:"requested_at"`
	ReviewedAt        time.Time `json:"reviewed_at,omitempty"`
	ReviewedBy        string    `json:"reviewed_by,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// =============================================================================
// Tenant Provider Configuration Types
// =============================================================================

// TenantProviderConfig stores provider-specific configuration
type TenantProviderConfig struct {
	Providers map[Provider]ProviderConfig  `json:"providers"`
	Models    map[string]TenantModelConfig `json:"models"`
	CreatedAt time.Time                    `json:"created_at"`
	UpdatedAt time.Time                    `json:"updated_at"`
}

// ConnectionSettings defines HTTP connection pool settings for a provider
type ConnectionSettings struct {
	MaxConnections     int  `json:"max_connections"`      // Max TCP connections to provider
	MaxIdleConnections int  `json:"max_idle_connections"` // Connections kept warm for reuse
	IdleTimeoutSec     int  `json:"idle_timeout_sec"`     // Close idle connections after this time
	RequestTimeoutSec  int  `json:"request_timeout_sec"`  // Max time for a single request
	EnableHTTP2        bool `json:"enable_http2"`         // Use HTTP/2 for better performance
	EnableKeepAlive    bool `json:"enable_keep_alive"`    // Reuse connections
}

// DefaultConnectionSettings returns sensible defaults
func DefaultConnectionSettings() ConnectionSettings {
	return ConnectionSettings{
		MaxConnections:     10,
		MaxIdleConnections: 5,
		IdleTimeoutSec:     90,
		RequestTimeoutSec:  300,
		EnableHTTP2:        true,
		EnableKeepAlive:    true,
	}
}

// ProviderConfig contains credentials and settings for an LLM provider
type ProviderConfig struct {
	Provider Provider `json:"provider"`
	Enabled  bool     `json:"enabled"`
	APIKey   string   `json:"api_key,omitempty"`
	BaseURL  string   `json:"base_url,omitempty"`
	OrgID    string   `json:"org_id,omitempty"`

	// AWS Bedrock configuration (supports Long-Term API Keys)
	Region          string `json:"region,omitempty"`            // For AWS Bedrock
	RegionPrefix    string `json:"region_prefix,omitempty"`     // For AWS Bedrock (us., eu., global.)
	AccessKeyID     string `json:"access_key_id,omitempty"`     // For AWS Bedrock (legacy)
	SecretAccessKey string `json:"secret_access_key,omitempty"` // For AWS Bedrock (legacy)
	Profile         string `json:"profile,omitempty"`           // For AWS Bedrock
	ModelsURL       string `json:"models_url,omitempty"`        // For AWS Bedrock - custom models API endpoint

	// Azure OpenAI configuration
	// See: https://docs.llmgateway.io/integrations/azure
	ResourceName string `json:"resource_name,omitempty"` // Azure resource name (preferred)
	APIVersion   string `json:"api_version,omitempty"`   // Azure API version (e.g., 2024-08-01-preview)

	// Connection pool settings (validated against tenant plan limits)
	ConnectionSettings ConnectionSettings `json:"connection_settings"`

	ExtraSettings map[string]string `json:"extra_settings,omitempty"`
}

// TenantModelConfig contains tenant-specific model configuration
type TenantModelConfig struct {
	ModelID           string  `json:"model_id"`
	Enabled           bool    `json:"enabled"`
	Alias             string  `json:"alias,omitempty"`
	MaxTokensOverride int32   `json:"max_tokens_override,omitempty"`
	CostMultiplier    float64 `json:"cost_multiplier,omitempty"` // For internal cost tracking/markup
}

// TenantQuotas contains usage quotas
type TenantQuotas struct {
	RequestsUsed  int64     `json:"requests_used"`
	RequestsLimit int64     `json:"requests_limit"`
	TokensUsed    int64     `json:"tokens_used"`
	TokensLimit   int64     `json:"tokens_limit"`
	CostUsedUSD   float64   `json:"cost_used_usd"`
	CostLimitUSD  float64   `json:"cost_limit_usd"`
	PeriodStart   time.Time `json:"period_start"`
	PeriodEnd     time.Time `json:"period_end"`
}

// APIKey represents an API key
type APIKey struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	KeyPrefix string   `json:"key_prefix"`
	KeyHash   string   `json:"-"`
	Scopes    []string `json:"scopes"`
	// RBAC: API key can be assigned to either a Role OR a Group (not both)
	// If GroupID is set, the API key inherits permissions from all Roles in the Group
	RoleID         string     `json:"role_id,omitempty"`    // Associated role for RBAC
	RoleName       string     `json:"role_name,omitempty"`  // Role name for display
	GroupID        string     `json:"group_id,omitempty"`   // Associated group for RBAC (alternative to role)
	GroupName      string     `json:"group_name,omitempty"` // Group name for display
	CreatedAt      time.Time  `json:"created_at"`
	CreatedBy      string     `json:"created_by,omitempty"`       // User ID who created the key
	CreatedByEmail string     `json:"created_by_email,omitempty"` // Email of creator for display
	UpdatedAt      time.Time  `json:"updated_at,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
	Revoked        bool       `json:"revoked"`
}

// =============================================================================
// Policy Types
// =============================================================================

// PolicyType represents the type of policy
type PolicyType string

const (
	PolicyTypeToolAccess    PolicyType = "tool_access"
	PolicyTypeModelAccess   PolicyType = "model_access"
	PolicyTypePromptFilter  PolicyType = "prompt_filter"
	PolicyTypeRateLimit     PolicyType = "rate_limit"
	PolicyTypeCostLimit     PolicyType = "cost_limit"
	PolicyTypeContentFilter PolicyType = "content_filter"
)

// Effect represents the effect of a policy statement
type Effect string

const (
	EffectAllow Effect = "allow"
	EffectDeny  Effect = "deny"
)

// Policy represents an access control policy
type Policy struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Type        PolicyType        `json:"type"`
	Statements  []PolicyStatement `json:"statements"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Enabled     bool              `json:"enabled"`
	Priority    int32             `json:"priority"`
}

// PolicyStatement represents a single policy statement (ARN-style)
type PolicyStatement struct {
	Sid        string            `json:"sid"`
	Effect     Effect            `json:"effect"`
	Actions    []string          `json:"actions"`   // e.g., ["modelgate:InvokeModel", "modelgate:CallTool"]
	Resources  []string          `json:"resources"` // ARN patterns, e.g., ["arn:modelgate:model:gemini/*"]
	Conditions []PolicyCondition `json:"conditions"`
}

// PolicyCondition represents a condition in a policy statement
type PolicyCondition struct {
	Operator string   `json:"operator"` // "StringEquals", "StringLike", "NumericLessThan", etc.
	Key      string   `json:"key"`      // e.g., "request:TokenCount", "tenant:Tier"
	Values   []string `json:"values"`
}

// PolicyEvaluationResult is the result of policy evaluation
type PolicyEvaluationResult struct {
	Allowed         bool              `json:"allowed"`
	Violations      []PolicyViolation `json:"violations,omitempty"`
	MatchedPolicies []string          `json:"matched_policies,omitempty"`
}

// PolicyViolation represents a policy violation
type PolicyViolation struct {
	PolicyID      string `json:"policy_id"`
	PolicyName    string `json:"policy_name"`
	ViolationType string `json:"violation_type"`
	Message       string `json:"message"`
	Severity      string `json:"severity"`
}

// =============================================================================
// Prompt Safety Types
// =============================================================================

// PromptAnalysis contains the result of prompt analysis
type PromptAnalysis struct {
	RequestID       string            `json:"request_id"`
	SafetyScore     PromptSafetyScore `json:"safety_score"`
	OutlierAnalysis OutlierAnalysis   `json:"outlier_analysis"`
	ContentFlags    []ContentFlag     `json:"content_flags"`
}

// PromptSafetyScore contains safety scoring
type PromptSafetyScore struct {
	OverallScore   float64            `json:"overall_score"`
	CategoryScores map[string]float64 `json:"category_scores"`
	IsSafe         bool               `json:"is_safe"`
	RiskLevel      string             `json:"risk_level"` // "low", "medium", "high", "critical"
}

// OutlierType represents the type of outlier detected
type OutlierType string

const (
	OutlierTypeLength    OutlierType = "length"
	OutlierTypePattern   OutlierType = "pattern"
	OutlierTypeContent   OutlierType = "content"
	OutlierTypeFrequency OutlierType = "frequency"
	OutlierTypeInjection OutlierType = "injection"
)

// OutlierAnalysis contains outlier detection results
type OutlierAnalysis struct {
	IsOutlier      bool        `json:"is_outlier"`
	AnomalyScore   float64     `json:"anomaly_score"`
	OutlierReasons []string    `json:"outlier_reasons"`
	OutlierType    OutlierType `json:"outlier_type"`
}

// ContentFlag represents a content flag
type ContentFlag struct {
	Category    string  `json:"category"`
	Subcategory string  `json:"subcategory"`
	Confidence  float64 `json:"confidence"`
	Description string  `json:"description"`
	Blocking    bool    `json:"blocking"`
}

// =============================================================================
// Usage and Cost Types
// =============================================================================

// UsageRecord represents a usage record
type UsageRecord struct {
	ID             string         `json:"id"`
	APIKeyID       string         `json:"api_key_id,omitempty"`
	APIKeyName     string         `json:"api_key_name,omitempty"`
	RequestID      string         `json:"request_id"`
	Model          string         `json:"model"`
	Provider       Provider       `json:"provider"`
	InputTokens    int64          `json:"input_tokens"`
	OutputTokens   int64          `json:"output_tokens"`
	TotalTokens    int64          `json:"total_tokens"`
	CostUSD        float64        `json:"cost_usd"`
	LatencyMs      int64          `json:"latency_ms"`
	Success        bool           `json:"success"`
	ErrorCode      string         `json:"error_code,omitempty"`
	ErrorMessage   string         `json:"error_message,omitempty"`
	ToolCalls      int32          `json:"tool_calls"`
	ThinkingTokens int64          `json:"thinking_tokens,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	Timestamp      time.Time      `json:"timestamp"`
}

// UsageStats contains aggregated usage statistics
type UsageStats struct {
	TotalRequests   int64                      `json:"total_requests"`
	TotalTokens     int64                      `json:"total_tokens"`
	TotalCostUSD    float64                    `json:"total_cost_usd"`
	UsageByModel    map[string]ModelUsage      `json:"usage_by_model"`
	UsageByProvider map[Provider]ProviderUsage `json:"usage_by_provider"`
	DataPoints      []UsageDataPoint           `json:"data_points"`
}

// ModelUsage contains per-model usage
type ModelUsage struct {
	ModelID      string  `json:"model_id"`
	Requests     int64   `json:"requests"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
}

// ProviderUsage contains per-provider usage
type ProviderUsage struct {
	Provider Provider `json:"provider"`
	Requests int64    `json:"requests"`
	Tokens   int64    `json:"tokens"`
	CostUSD  float64  `json:"cost_usd"`
}

// UsageDataPoint is a time-series data point
type UsageDataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Requests  int64     `json:"requests"`
	Tokens    int64     `json:"tokens"`
	CostUSD   float64   `json:"cost_usd"`
}

// ModelUsageStats is an alias for ModelUsage (for database compatibility)
type ModelUsageStats = ModelUsage

// ProviderUsageStats contains per-provider usage statistics
type ProviderUsageStats struct {
	Provider     string  `json:"provider"`
	Requests     int64   `json:"requests"`
	TotalTokens  int64   `json:"total_tokens"`
	CostUSD      float64 `json:"cost_usd"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
}

// APIKeyUsageStats contains per-API-key usage statistics
type APIKeyUsageStats struct {
	APIKeyID    string  `json:"api_key_id"`
	APIKeyName  string  `json:"api_key_name"`
	Requests    int64   `json:"requests"`
	TotalTokens int64   `json:"total_tokens"`
	CostUSD     float64 `json:"cost_usd"`
}

// UsageTimePoint is a time-series data point (alias for compatibility)
type UsageTimePoint = UsageDataPoint

// ModelConfig represents tenant-specific model configuration
type ModelConfig struct {
	ID                string            `json:"id"`
	ModelID           string            `json:"model_id"`
	IsEnabled         bool              `json:"is_enabled"`
	Alias             string            `json:"alias,omitempty"`
	MaxTokensOverride int               `json:"max_tokens_override,omitempty"`
	CostMultiplier    float64           `json:"cost_multiplier"`
	Metadata          map[string]string `json:"metadata,omitempty"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
}

// =============================================================================
// Interfaces
// =============================================================================

// LLMClient is the interface for LLM provider clients
type LLMClient interface {
	// ChatStream starts a streaming chat completion
	ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error)

	// ChatComplete performs a non-streaming chat completion
	ChatComplete(ctx context.Context, req *ChatRequest) (*ChatResponse, error)

	// Embed generates embeddings
	Embed(ctx context.Context, model string, texts []string, dimensions *int32) ([][]float32, int64, error)

	// CountTokens counts tokens in a request
	CountTokens(ctx context.Context, req *ChatRequest) (int32, error)

	// ListModels lists available models
	ListModels(ctx context.Context) ([]ModelInfo, error)

	// Provider returns the provider type
	Provider() Provider

	// SupportsModel checks if a model is supported
	SupportsModel(model string) bool
}

// ResponsesCapable is an optional interface for providers that support native /v1/responses endpoint
// Providers that don't implement this will fall back to prompt-based or JSON mode strategies
type ResponsesCapable interface {
	// GenerateResponse generates a structured response using the provider's native responses API
	GenerateResponse(ctx context.Context, req *ResponseRequest) (*StructuredResponse, error)
}

// TenantRepository is the interface for tenant storage
type TenantRepository interface {
	Create(ctx context.Context, tenant *Tenant) error
	Get(ctx context.Context, id string) (*Tenant, error)
	Update(ctx context.Context, tenant *Tenant) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter TenantFilter) ([]*Tenant, string, error)
	GetByAPIKey(ctx context.Context, keyHash string) (*Tenant, *APIKey, error)
}

// TenantFilter for listing tenants
type TenantFilter struct {
	Status    TenantStatus
	Tier      TenantTier
	PageSize  int32
	PageToken string
}

// APIKeyRepository is the interface for API key storage
type APIKeyRepository interface {
	Create(ctx context.Context, key *APIKey) error
	Get(ctx context.Context, id string) (*APIKey, error)
	GetByHash(ctx context.Context, hash string) (*APIKey, error)
	List(ctx context.Context, tenantID string) ([]*APIKey, error)
	Update(ctx context.Context, key *APIKey) error // Update API key (role/group assignment, name, etc.)
	Revoke(ctx context.Context, id string) error
	UpdateLastUsed(ctx context.Context, id string) error
}

// PolicyRepository is the interface for policy storage
type PolicyRepository interface {
	Create(ctx context.Context, policy *Policy) error
	Get(ctx context.Context, id string) (*Policy, error)
	Update(ctx context.Context, policy *Policy) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter PolicyFilter) ([]*Policy, string, error)
	GetByTenant(ctx context.Context, tenantID string) ([]*Policy, error)
}

// PolicyFilter for listing policies
type PolicyFilter struct {
	Type      PolicyType
	PageSize  int32
	PageToken string
}

// UsageRepository is the interface for usage storage
type UsageRepository interface {
	Record(ctx context.Context, record *UsageRecord) error
	GetStats(ctx context.Context, tenantID string, startTime, endTime time.Time, granularity string) (*UsageStats, error)
	GetTenantQuotas(ctx context.Context, tenantID string) (*TenantQuotas, error)
	UpdateTenantQuotas(ctx context.Context, tenantID string, quotas *TenantQuotas) error
}

// TenantProviderConfigRepository is the interface for tenant provider config storage
type TenantProviderConfigRepository interface {
	Get(ctx context.Context, tenantID string) (*TenantProviderConfig, error)
	Save(ctx context.Context, config *TenantProviderConfig) error
	Delete(ctx context.Context, tenantID string) error
}

// PolicyEngine is the interface for policy evaluation
type PolicyEngine interface {
	// Evaluate evaluates a request against policies
	Evaluate(ctx context.Context, tenantID string, req *ChatRequest) (*PolicyEvaluationResult, error)

	// EvaluateToolCall evaluates a tool call against policies
	EvaluateToolCall(ctx context.Context, tenantID string, toolCall *ToolCall) (*PolicyEvaluationResult, error)

	// AnalyzePrompt performs prompt safety analysis
	AnalyzePrompt(ctx context.Context, tenantID string, req *ChatRequest) (*PromptAnalysis, error)
}

// RoleModelFilter is an interface for filtering models based on role policies
type RoleModelFilter interface {
	// GetAllowedModelsForRole returns models that are allowed for a specific role
	GetAllowedModelsForRole(ctx context.Context, tenantID, roleID string, availableModels []ModelInfo) ([]ModelInfo, error)
}

// =============================================================================
// Request Log Types (for Analytics Dashboard)
// =============================================================================

// RequestLog represents a detailed log of an API request
type RequestLog struct {
	ID             string   `json:"id"`
	APIKeyID       string   `json:"api_key_id,omitempty"`
	APIKeyName     string   `json:"api_key_name,omitempty"`
	RequestID      string   `json:"request_id"`
	Model          string   `json:"model"`
	Provider       Provider `json:"provider"`
	Method         string   `json:"method"` // "chat", "embed", "complete"
	InputTokens    int64    `json:"input_tokens"`
	OutputTokens   int64    `json:"output_tokens"`
	TotalTokens    int64    `json:"total_tokens"`
	CostUSD        float64  `json:"cost_usd"`
	LatencyMs      int64    `json:"latency_ms"`
	Success        bool     `json:"success"`
	ErrorCode      string   `json:"error_code,omitempty"`
	ErrorMessage   string   `json:"error_message,omitempty"`
	StatusCode     int      `json:"status_code"`
	UserAgent      string   `json:"user_agent,omitempty"`
	IPAddress      string   `json:"ip_address,omitempty"`
	ToolCalls      int32    `json:"tool_calls"`
	ThinkingTokens int64    `json:"thinking_tokens,omitempty"`
	Streaming      bool     `json:"streaming"`
	// Optional: request/response content for debugging (configurable)
	RequestContent  string         `json:"request_content,omitempty"`
	ResponseContent string         `json:"response_content,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
}

// RequestLogFilter for querying request logs
type RequestLogFilter struct {
	APIKeyID   string
	Model      string
	Provider   Provider
	Success    *bool
	StartTime  time.Time
	EndTime    time.Time
	SearchText string
	PageSize   int32
	PageToken  string
	SortBy     string // "created_at", "latency_ms", "cost_usd", "total_tokens"
	SortOrder  string // "asc", "desc"
}

// RequestLogRepository is the interface for request log storage
type RequestLogRepository interface {
	Create(ctx context.Context, log *RequestLog) error
	Get(ctx context.Context, id string) (*RequestLog, error)
	List(ctx context.Context, filter RequestLogFilter) ([]*RequestLog, string, error)
	Delete(ctx context.Context, id string) error
	DeleteOlderThan(ctx context.Context, tenantID string, before time.Time) (int64, error)
}

// =============================================================================
// Dashboard Analytics Types
// =============================================================================

// DashboardStats contains aggregated dashboard statistics
type DashboardStats struct {
	Period            string          `json:"period"` // "day", "week", "month"
	TotalRequests     int64           `json:"total_requests"`
	TotalTokens       int64           `json:"total_tokens"`
	TotalCostUSD      float64         `json:"total_cost_usd"`
	ActiveAPIKeys     int             `json:"active_api_keys"`
	AvgLatencyMs      float64         `json:"avg_latency_ms"`
	SuccessRate       float64         `json:"success_rate"`
	QuotaUsage        QuotaUsageStats `json:"quota_usage"`
	TopModels         []ModelStats    `json:"top_models"`
	RequestsByHour    []HourlyStats   `json:"requests_by_hour,omitempty"`
	CostTrend         []DailyStats    `json:"cost_trend,omitempty"`
	ProviderBreakdown []ProviderStats `json:"provider_breakdown,omitempty"`
}

// QuotaUsageStats contains quota usage information
type QuotaUsageStats struct {
	RequestsUsed    int64   `json:"requests_used"`
	RequestsLimit   int64   `json:"requests_limit"`
	RequestsPercent float64 `json:"requests_percent"`
	TokensUsed      int64   `json:"tokens_used"`
	TokensLimit     int64   `json:"tokens_limit"`
	TokensPercent   float64 `json:"tokens_percent"`
	CostUsedUSD     float64 `json:"cost_used_usd"`
	CostLimitUSD    float64 `json:"cost_limit_usd"`
	CostPercent     float64 `json:"cost_percent"`
}

// ModelStats contains per-model statistics
type ModelStats struct {
	Model        string  `json:"model"`
	Provider     string  `json:"provider"`
	Requests     int64   `json:"requests"`
	Tokens       int64   `json:"tokens"`
	CostUSD      float64 `json:"cost_usd"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	SuccessRate  float64 `json:"success_rate"`
	Percentage   float64 `json:"percentage"` // Percentage of total requests
}

// ProviderStats contains per-provider statistics
type ProviderStats struct {
	Provider     Provider `json:"provider"`
	Requests     int64    `json:"requests"`
	Tokens       int64    `json:"tokens"`
	CostUSD      float64  `json:"cost_usd"`
	AvgLatencyMs float64  `json:"avg_latency_ms"`
	SuccessRate  float64  `json:"success_rate"`
	Percentage   float64  `json:"percentage"`
}

// HourlyStats contains hourly statistics
type HourlyStats struct {
	Hour     int     `json:"hour"` // 0-23
	Requests int64   `json:"requests"`
	Tokens   int64   `json:"tokens"`
	CostUSD  float64 `json:"cost_usd"`
}

// DailyStats contains daily statistics
type DailyStats struct {
	Date     string  `json:"date"` // YYYY-MM-DD
	Requests int64   `json:"requests"`
	Tokens   int64   `json:"tokens"`
	CostUSD  float64 `json:"cost_usd"`
}

// =============================================================================
// Budget Alert Types
// =============================================================================

// BudgetAlert represents a budget alert configuration
type BudgetAlert struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Type          string     `json:"type"`           // "cost", "tokens", "requests"
	ThresholdType string     `json:"threshold_type"` // "absolute", "percentage"
	Threshold     float64    `json:"threshold"`
	Period        string     `json:"period"` // "daily", "weekly", "monthly"
	Enabled       bool       `json:"enabled"`
	NotifyEmail   string     `json:"notify_email,omitempty"`
	NotifyWebhook string     `json:"notify_webhook,omitempty"`
	LastTriggered *time.Time `json:"last_triggered,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// BudgetAlertRepository is the interface for budget alert storage
type BudgetAlertRepository interface {
	Create(ctx context.Context, alert *BudgetAlert) error
	Get(ctx context.Context, id string) (*BudgetAlert, error)
	List(ctx context.Context, tenantID string) ([]*BudgetAlert, error)
	Update(ctx context.Context, alert *BudgetAlert) error
	Delete(ctx context.Context, id string) error
}

// =============================================================================
// Model Performance Types
// =============================================================================

// ModelPerformance contains performance metrics for a model
type ModelPerformance struct {
	Model           string    `json:"model"`
	Provider        Provider  `json:"provider"`
	TotalRequests   int64     `json:"total_requests"`
	SuccessfulReqs  int64     `json:"successful_requests"`
	FailedReqs      int64     `json:"failed_requests"`
	SuccessRate     float64   `json:"success_rate"`
	AvgLatencyMs    float64   `json:"avg_latency_ms"`
	P50LatencyMs    float64   `json:"p50_latency_ms"`
	P95LatencyMs    float64   `json:"p95_latency_ms"`
	P99LatencyMs    float64   `json:"p99_latency_ms"`
	AvgInputTokens  float64   `json:"avg_input_tokens"`
	AvgOutputTokens float64   `json:"avg_output_tokens"`
	TotalCostUSD    float64   `json:"total_cost_usd"`
	CostPerRequest  float64   `json:"cost_per_request"`
	TokensPerSecond float64   `json:"tokens_per_second"`
	Period          string    `json:"period"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ModelComparison contains a comparison between models
type ModelComparison struct {
	Models             []string                    `json:"models"`
	Metrics            []string                    `json:"metrics"` // metrics being compared
	PerformanceData    map[string]ModelPerformance `json:"performance_data"`
	BestForSpeed       string                      `json:"best_for_speed"`
	BestForCost        string                      `json:"best_for_cost"`
	BestForReliability string                      `json:"best_for_reliability"`
	Period             string                      `json:"period"`
}

// =============================================================================
// Audit Log Types
// =============================================================================

// AuditAction represents the type of audited action
type AuditAction string

const (
	AuditActionCreate AuditAction = "create"
	AuditActionUpdate AuditAction = "update"
	AuditActionDelete AuditAction = "delete"
	AuditActionRevoke AuditAction = "revoke"
	AuditActionLogin  AuditAction = "login"
	AuditActionLogout AuditAction = "logout"
)

// AuditResourceType represents the type of resource being audited
type AuditResourceType string

const (
	AuditResourceRole     AuditResourceType = "role"
	AuditResourcePolicy   AuditResourceType = "policy"
	AuditResourceGroup    AuditResourceType = "group"
	AuditResourceAPIKey   AuditResourceType = "api_key"
	AuditResourceUser     AuditResourceType = "user"
	AuditResourceProvider AuditResourceType = "provider"
	AuditResourceTenant   AuditResourceType = "tenant"
	AuditResourceSession  AuditResourceType = "session"
)

// AuditLog represents an audit log entry
type AuditLog struct {
	ID           string            `json:"id"`
	Timestamp    time.Time         `json:"timestamp"`
	Action       AuditAction       `json:"action"`
	ResourceType AuditResourceType `json:"resource_type"`
	ResourceID   string            `json:"resource_id"`
	ResourceName string            `json:"resource_name"`
	ActorID      string            `json:"actor_id"`    // User ID who performed the action
	ActorEmail   string            `json:"actor_email"` // Email of the actor
	ActorType    string            `json:"actor_type"`  // "user", "admin", "system"
	IPAddress    string            `json:"ip_address"`
	UserAgent    string            `json:"user_agent"`
	Details      map[string]any    `json:"details"`       // Additional details about the action
	OldValue     map[string]any    `json:"old_value"`     // Previous state (for updates)
	NewValue     map[string]any    `json:"new_value"`     // New state (for creates/updates)
	Status       string            `json:"status"`        // "success", "failure"
	ErrorMessage string            `json:"error_message"` // If status is failure
}

// AuditLogRepository defines audit log storage operations
type AuditLogRepository interface {
	Create(ctx context.Context, log *AuditLog) error
	List(ctx context.Context, filter AuditLogFilter) ([]AuditLog, error)
	GetByID(ctx context.Context, id string) (*AuditLog, error)
	Count(ctx context.Context, filter AuditLogFilter) (int, error)
}

// AuditLogFilter for querying audit logs
type AuditLogFilter struct {
	ResourceType AuditResourceType
	ResourceID   string
	Action       AuditAction
	ActorID      string
	StartTime    time.Time
	EndTime      time.Time
	Limit        int
	Offset       int
}
