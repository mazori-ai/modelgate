// Package domain defines core domain types for the ModelGate LLM gateway.
package domain

import (
	"time"
)

// =============================================================================
// Role Types
// =============================================================================

// Role represents a role that can be assigned to API keys
type Role struct {
	ID             string      `json:"id"`
	Name           string      `json:"name"`
	Description    string      `json:"description"`
	Permissions    []string    `json:"permissions"`      // List of permission strings
	IsDefault      bool        `json:"is_default"`       // Whether this is the default role for new API keys
	IsSystem       bool        `json:"is_system"`        // Whether this is a system role (cannot be deleted)
	Policy         *RolePolicy `json:"policy,omitempty"` // Associated policy (populated on read)
	CreatedBy      string      `json:"created_by,omitempty"`
	CreatedByEmail string      `json:"created_by_email,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

// MCPPolicies defines policies for MCP Gateway functionality
type MCPPolicies struct {
	Enabled            bool `json:"enabled"`
	AllowToolSearch    bool `json:"allow_tool_search"`
	AuditToolExecution bool `json:"audit_tool_execution"`
}

// RolePolicy associates a role with policy configurations
// Extended to support 10 policy types: Prompt, Tool, RateLimit, Model, Caching, Routing, Resilience, Budget, Concurrency, MCP
type RolePolicy struct {
	ID     string `json:"id"`
	RoleID string `json:"role_id"`

	// Core Policies (Original 4)
	PromptPolicies   PromptPolicies    `json:"prompt_policies"`
	ToolPolicies     ToolPolicies      `json:"tool_policies"`
	RateLimitPolicy  RateLimitPolicy   `json:"rate_limit_policy"`
	ModelRestriction ModelRestrictions `json:"model_restrictions"`

	// Extended Policies (5) - Policy-Driven Features
	CachingPolicy     CachingPolicy     `json:"caching_policy"`
	RoutingPolicy     RoutingPolicy     `json:"routing_policy"`
	ResiliencePolicy  ResiliencePolicy  `json:"resilience_policy"`
	BudgetPolicy      BudgetPolicy      `json:"budget_policy"`
	ConcurrencyPolicy ConcurrencyPolicy `json:"concurrency_policy"`

	// MCP Gateway Policy
	MCPPolicies MCPPolicies `json:"mcp_policies"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ConcurrencyPolicy controls request queuing and priority per role
type ConcurrencyPolicy struct {
	Enabled  bool `json:"enabled"`
	Priority int  `json:"priority"` // 0-10, higher = processed first
}

// =============================================================================
// Group Types
// =============================================================================

// Group represents a group of users/API keys
type Group struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	RoleIDs        []string  `json:"role_ids"` // Roles assigned to this group
	CreatedBy      string    `json:"created_by,omitempty"`
	CreatedByEmail string    `json:"created_by_email,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// GroupMember represents membership in a group
type GroupMember struct {
	GroupID    string    `json:"group_id"`
	MemberID   string    `json:"member_id"`   // Can be user ID or API key ID
	MemberType string    `json:"member_type"` // "user" or "api_key"
	AddedAt    time.Time `json:"added_at"`
}

// =============================================================================
// Prompt Policy Types (Extended for comprehensive security)
// =============================================================================

// PromptPolicies defines comprehensive prompt security policies (OWASP-aligned)
type PromptPolicies struct {
	// =========================================================================
	// 1. STRUCTURAL SEPARATION
	// =========================================================================
	StructuralSeparation StructuralSeparationConfig `json:"structural_separation"`

	// =========================================================================
	// 2. INPUT NORMALIZATION
	// =========================================================================
	Normalization NormalizationConfig `json:"normalization"`

	// =========================================================================
	// 3. INPUT BOUNDS
	// =========================================================================
	InputBounds InputBoundsConfig `json:"input_bounds"`

	// =========================================================================
	// 4. INJECTION DETECTION
	// =========================================================================
	DirectInjectionDetection   InjectionDetectionConfig `json:"direct_injection_detection"`
	IndirectInjectionDetection InjectionDetectionConfig `json:"indirect_injection_detection"`

	// =========================================================================
	// 5. PII POLICY
	// =========================================================================
	PIIPolicy PIIPolicyConfig `json:"pii_policy"`

	// =========================================================================
	// 6. CONTENT FILTERING
	// =========================================================================
	ContentFiltering ContentFilteringConfig `json:"content_filtering"`

	// =========================================================================
	// 7. SYSTEM PROMPT PROTECTION
	// =========================================================================
	SystemPromptProtection SystemPromptProtectionConfig `json:"system_prompt_protection"`

	// =========================================================================
	// 8. OUTPUT VALIDATION
	// =========================================================================
	OutputValidation OutputValidationConfig `json:"output_validation"`
}

// StructuralSeparationConfig separates instructions from data
type StructuralSeparationConfig struct {
	Enabled                  bool           `json:"enabled"`
	TemplateFormat           TemplateFormat `json:"template_format"` // xml, json, markdown
	SystemSection            string         `json:"system_section"`
	UserSection              string         `json:"user_section"`
	RetrievedSection         string         `json:"retrieved_section"`
	ForbidInstructionsInData bool           `json:"forbid_instructions_in_data"`
	QuoteUserContent         bool           `json:"quote_user_content"`
	MarkRetrievedAsUntrusted bool           `json:"mark_retrieved_as_untrusted"`
}

// TemplateFormat defines prompt template formats
type TemplateFormat string

const (
	TemplateFormatXML      TemplateFormat = "xml"
	TemplateFormatJSON     TemplateFormat = "json"
	TemplateFormatMarkdown TemplateFormat = "markdown"
)

// NormalizationConfig for input canonicalization
type NormalizationConfig struct {
	Enabled                  bool            `json:"enabled"`
	UnicodeNormalization     UnicodeNormForm `json:"unicode_normalization"`
	NormalizeNewlines        bool            `json:"normalize_newlines"`
	StripNullBytes           bool            `json:"strip_null_bytes"`
	RemoveInvisibleChars     bool            `json:"remove_invisible_chars"`
	DetectMixedEncodings     bool            `json:"detect_mixed_encodings"`
	DecodeBase64             bool            `json:"decode_base64"`
	DecodeURLEncoding        bool            `json:"decode_url_encoding"`
	RejectSuspiciousEncoding bool            `json:"reject_suspicious_encoding"`
	CollapseWhitespace       bool            `json:"collapse_whitespace"`
	TrimWhitespace           bool            `json:"trim_whitespace"`
}

// UnicodeNormForm defines Unicode normalization forms
type UnicodeNormForm string

const (
	UnicodeNFC  UnicodeNormForm = "NFC"
	UnicodeNFKC UnicodeNormForm = "NFKC"
	UnicodeNFD  UnicodeNormForm = "NFD"
	UnicodeNFKD UnicodeNormForm = "NFKD"
)

// InputBoundsConfig for input limits
type InputBoundsConfig struct {
	Enabled             bool    `json:"enabled"`
	MaxPromptLength     int     `json:"max_prompt_length"`
	MaxPromptTokens     int     `json:"max_prompt_tokens"`
	MaxMessageCount     int     `json:"max_message_count"`
	MaxMessageLength    int     `json:"max_message_length"`
	MaxJSONNestingDepth int     `json:"max_json_nesting_depth"`
	MaxURLCount         int     `json:"max_url_count"`
	MaxAttachmentCount  int     `json:"max_attachment_count"`
	MaxAttachmentSize   int     `json:"max_attachment_size"`
	MaxRepeatedPhrases  int     `json:"max_repeated_phrases"`
	AnomalyThreshold    float64 `json:"anomaly_threshold"`
}

// InjectionDetectionConfig for prompt injection detection
type InjectionDetectionConfig struct {
	Enabled         bool                 `json:"enabled"`
	DetectionMethod DetectionMethod      `json:"detection_method"`
	Sensitivity     DetectionSensitivity `json:"sensitivity"`
	OnDetection     DetectionAction      `json:"on_detection"`
	BlockThreshold  float64              `json:"block_threshold"`

	// Pattern-based detection
	PatternDetection PatternDetectionConfig `json:"pattern_detection"`

	// ML-based detection
	MLDetection MLDetectionConfig `json:"ml_detection"`
}

// DetectionMethod defines detection approaches
type DetectionMethod string

const (
	DetectionMethodRules  DetectionMethod = "rules"
	DetectionMethodML     DetectionMethod = "ml"
	DetectionMethodHybrid DetectionMethod = "hybrid"
)

// DetectionSensitivity defines sensitivity levels
type DetectionSensitivity string

const (
	SensitivityLow      DetectionSensitivity = "low"
	SensitivityMedium   DetectionSensitivity = "medium"
	SensitivityHigh     DetectionSensitivity = "high"
	SensitivityParanoid DetectionSensitivity = "paranoid"
)

// DetectionAction defines actions on detection
type DetectionAction string

const (
	DetectionActionBlock      DetectionAction = "block"
	DetectionActionWarn       DetectionAction = "warn"
	DetectionActionLog        DetectionAction = "log"
	DetectionActionQuarantine DetectionAction = "quarantine"
	DetectionActionTransform  DetectionAction = "transform"
)

// PatternDetectionConfig for pattern-based injection detection
type PatternDetectionConfig struct {
	Enabled                    bool     `json:"enabled"`
	DetectIgnoreInstructions   bool     `json:"detect_ignore_instructions"`
	DetectSystemPromptRequests bool     `json:"detect_system_prompt_requests"`
	DetectRoleConfusion        bool     `json:"detect_role_confusion"`
	DetectJailbreakPhrases     bool     `json:"detect_jailbreak_phrases"`
	DetectToolCoercion         bool     `json:"detect_tool_coercion"`
	DetectEncodingEvasion      bool     `json:"detect_encoding_evasion"`
	CustomBlockPatterns        []string `json:"custom_block_patterns"`
	CustomWarnPatterns         []string `json:"custom_warn_patterns"`

	// Fuzzy matching configuration (new)
	EnableFuzzyMatching  bool                 `json:"enable_fuzzy_matching"`  // Enable Levenshtein-based fuzzy matching
	EnableWordMatching   bool                 `json:"enable_word_matching"`   // Enable word-level Jaccard similarity
	EnableNormalization  bool                 `json:"enable_normalization"`   // Enable text normalization (homoglyphs, l33t)
	DisableFuzzyMatching bool                 `json:"disable_fuzzy_matching"` // Explicitly disable fuzzy matching
	DisableWordMatching  bool                 `json:"disable_word_matching"`  // Explicitly disable word matching
	DisableNormalization bool                 `json:"disable_normalization"`  // Explicitly disable normalization
	FuzzyThreshold       float64              `json:"fuzzy_threshold"`        // Similarity threshold (0.0-1.0, default 0.85)
	Sensitivity          DetectionSensitivity `json:"sensitivity"`            // low, medium, high, paranoid
	WhitelistedPhrases   []string             `json:"whitelisted_phrases"`    // Phrases to exclude from detection
}

// MLDetectionConfig for ML-based injection detection
type MLDetectionConfig struct {
	Enabled            bool    `json:"enabled"`
	Model              string  `json:"model"` // builtin, openai-moderation, azure-content-safety, custom
	CustomEndpoint     string  `json:"custom_endpoint"`
	CustomAPIKey       string  `json:"custom_api_key"`
	InjectionThreshold float64 `json:"injection_threshold"`
	JailbreakThreshold float64 `json:"jailbreak_threshold"`
}

// PIIPolicyConfig for PII handling
type PIIPolicyConfig struct {
	Enabled       bool               `json:"enabled"`
	ScanInputs    bool               `json:"scan_inputs"`
	ScanOutputs   bool               `json:"scan_outputs"`
	ScanRetrieved bool               `json:"scan_retrieved"`
	Categories    []string           `json:"categories"` // email, phone, ssn, credit_card, etc.
	OnDetection   PIIAction          `json:"on_detection"`
	Redaction     PIIRedactionConfig `json:"redaction"`
}

// PIIAction defines PII handling actions
type PIIAction string

const (
	PIIActionBlock   PIIAction = "block"   // Block the request entirely
	PIIActionRedact  PIIAction = "redact"  // Replace with placeholders like [EMAIL REDACTED]
	PIIActionRewrite PIIAction = "rewrite" // Transform using deterministic character rotation
	PIIActionWarn    PIIAction = "warn"    // Allow but log warning
	PIIActionLog     PIIAction = "log"     // Allow and log for audit
)

// PIIRedactionConfig for PII redaction settings
type PIIRedactionConfig struct {
	PlaceholderFormat      string `json:"placeholder_format"`
	StoreOriginals         bool   `json:"store_originals"`
	RestoreInResponse      bool   `json:"restore_in_response"`
	ConsistentPlaceholders bool   `json:"consistent_placeholders"`
}

// ContentFilteringConfig for content filtering
type ContentFilteringConfig struct {
	Enabled               bool            `json:"enabled"`
	BlockedCategories     []string        `json:"blocked_categories"`
	CustomBlockedPatterns []string        `json:"custom_blocked_patterns"`
	CustomAllowedPatterns []string        `json:"custom_allowed_patterns"`
	OnDetection           DetectionAction `json:"on_detection"`
}

// SystemPromptProtectionConfig for system prompt security
type SystemPromptProtectionConfig struct {
	Enabled                  bool   `json:"enabled"`
	DetectExtractionAttempts bool   `json:"detect_extraction_attempts"`
	AddAntiExtractionSuffix  bool   `json:"add_anti_extraction_suffix"`
	AntiExtractionSuffix     string `json:"anti_extraction_suffix"`

	// Canary tokens
	CanaryEnabled     bool   `json:"canary_enabled"`
	CanaryToken       string `json:"canary_token"`
	CanaryAlertOnLeak bool   `json:"canary_alert_on_leak"`
	CanaryWebhook     string `json:"canary_webhook"`
}

// OutputValidationConfig for output security
type OutputValidationConfig struct {
	Enabled bool `json:"enabled"`

	// Schema enforcement
	EnforceSchema       bool   `json:"enforce_schema"`
	OutputSchema        string `json:"output_schema"`
	RejectInvalidSchema bool   `json:"reject_invalid_schema"`

	// Dangerous content detection
	DetectCodeExecution bool `json:"detect_code_execution"`
	DetectSQLStatements bool `json:"detect_sql_statements"`
	DetectShellCommands bool `json:"detect_shell_commands"`
	DetectHTMLScripts   bool `json:"detect_html_scripts"`

	// Auto-escaping
	EscapeForHTML bool `json:"escape_for_html"`
	EscapeForSQL  bool `json:"escape_for_sql"`
	EscapeForCLI  bool `json:"escape_for_cli"`

	// Leakage detection
	DetectSecretLeakage       bool     `json:"detect_secret_leakage"`
	SecretPatterns            []string `json:"secret_patterns"`
	DetectPIILeakage          bool     `json:"detect_pii_leakage"`
	DetectSystemPromptLeakage bool     `json:"detect_system_prompt_leakage"`

	// Content policy
	ApplyContentFiltering bool `json:"apply_content_filtering"`

	// Action
	OnViolation OutputViolationAction `json:"on_violation"`
}

// OutputViolationAction defines output violation handling
type OutputViolationAction string

const (
	OutputActionBlock      OutputViolationAction = "block"
	OutputActionRedact     OutputViolationAction = "redact"
	OutputActionWarn       OutputViolationAction = "warn"
	OutputActionLog        OutputViolationAction = "log"
	OutputActionRegenerate OutputViolationAction = "regenerate"
)

// =============================================================================
// Tool Policy Types
// =============================================================================

// ToolPolicies defines policies for tool/function calling
type ToolPolicies struct {
	AllowToolCalling       bool                  `json:"allow_tool_calling"`
	AllowedTools           []string              `json:"allowed_tools"` // Whitelist of tool names (* for all)
	BlockedTools           []string              `json:"blocked_tools"` // Blacklist of tool names
	ToolConfigs            map[string]ToolConfig `json:"tool_configs"`  // Per-tool configuration
	MaxToolCallsPerRequest int                   `json:"max_tool_calls_per_request"`
	RequireToolApproval    bool                  `json:"require_tool_approval"` // Require human approval for tool calls
}

// ToolConfig contains configuration for a specific tool
type ToolConfig struct {
	ToolName           string   `json:"tool_name"`
	Enabled            bool     `json:"enabled"`
	AllowedParameters  []string `json:"allowed_parameters"` // Whitelist of allowed parameters
	BlockedParameters  []string `json:"blocked_parameters"` // Blacklist of parameters
	MaxCallsPerRequest int      `json:"max_calls_per_request"`
	RequireApproval    bool     `json:"require_approval"`
}

// =============================================================================
// Rate Limit Policy Types
// =============================================================================

// RateLimitPolicy defines rate limiting for a role
type RateLimitPolicy struct {
	// Request rate limits
	RequestsPerMinute int `json:"requests_per_minute"`
	RequestsPerHour   int `json:"requests_per_hour"`
	RequestsPerDay    int `json:"requests_per_day"`

	// Token rate limits
	TokensPerMinute int64 `json:"tokens_per_minute"`
	TokensPerHour   int64 `json:"tokens_per_hour"`
	TokensPerDay    int64 `json:"tokens_per_day"`

	// Cost limits
	CostPerMinuteUSD float64 `json:"cost_per_minute_usd"`
	CostPerHourUSD   float64 `json:"cost_per_hour_usd"`
	CostPerDayUSD    float64 `json:"cost_per_day_usd"`
	CostPerMonthUSD  float64 `json:"cost_per_month_usd"`

	// Burst settings
	BurstLimit  int   `json:"burst_limit"`
	BurstTokens int64 `json:"burst_tokens"`

	// Per-model limits
	PerModelLimits map[string]ModelRateLimit `json:"per_model_limits"`
}

// ModelRateLimit defines rate limits for a specific model
type ModelRateLimit struct {
	ModelID           string  `json:"model_id"`
	RequestsPerMinute int     `json:"requests_per_minute"`
	TokensPerMinute   int64   `json:"tokens_per_minute"`
	CostPerDayUSD     float64 `json:"cost_per_day_usd"`
}

// =============================================================================
// Model Restriction Types
// =============================================================================

// ModelRestrictions defines which models a role can access
type ModelRestrictions struct {
	AllowedModels       []string   `json:"allowed_models"` // Only these models are allowed
	AllowedProviders    []Provider `json:"allowed_providers"`
	DefaultModel        string     `json:"default_model"`                    // Default model if not specified
	MaxTokensPerRequest int32      `json:"max_tokens_per_request,omitempty"` // Maximum tokens per request
}

// =============================================================================
// Caching Policy Types (NEW)
// =============================================================================

// CachingPolicy controls semantic caching behavior per role
type CachingPolicy struct {
	// Master switch
	Enabled bool `json:"enabled"`

	// Cache behavior
	SimilarityThreshold float64 `json:"similarity_threshold"` // 0.0-1.0, default 0.95
	TTLSeconds          int     `json:"ttl_seconds"`          // Cache TTL, default 3600
	MaxCacheSize        int     `json:"max_cache_size"`       // Per-role cache limit (entries)

	// What to cache
	CacheStreaming bool `json:"cache_streaming"`  // Cache streaming responses?
	CacheToolCalls bool `json:"cache_tool_calls"` // Cache tool call responses?

	// Exclusions
	ExcludedModels   []string `json:"excluded_models"`   // Don't cache these models
	ExcludedPatterns []string `json:"excluded_patterns"` // Don't cache prompts matching these

	// Cost tracking
	TrackSavings bool `json:"track_savings"` // Track cost savings from cache
}

// =============================================================================
// Routing Policy Types (NEW)
// =============================================================================

// RoutingPolicy controls intelligent routing behavior per role
type RoutingPolicy struct {
	// Master switch
	Enabled bool `json:"enabled"`

	// Strategy selection
	Strategy RoutingStrategy `json:"strategy"` // cost, latency, weighted, round_robin, capability

	// Strategy-specific configurations
	CostConfig       *CostRoutingConfig       `json:"cost_config,omitempty"`
	LatencyConfig    *LatencyRoutingConfig    `json:"latency_config,omitempty"`
	WeightedConfig   *WeightedRoutingConfig   `json:"weighted_config,omitempty"`
	CapabilityConfig *CapabilityRoutingConfig `json:"capability_config,omitempty"`

	// Override: if model explicitly specified, skip routing
	AllowModelOverride bool `json:"allow_model_override"`
}

// RoutingStrategy defines available routing strategies
type RoutingStrategy string

const (
	RoutingStrategyCost       RoutingStrategy = "cost"
	RoutingStrategyLatency    RoutingStrategy = "latency"
	RoutingStrategyWeighted   RoutingStrategy = "weighted"
	RoutingStrategyRoundRobin RoutingStrategy = "round_robin"
	RoutingStrategyCapability RoutingStrategy = "capability"
)

// CostRoutingConfig for cost-optimized routing
type CostRoutingConfig struct {
	SimpleQueryThreshold  float64  `json:"simple_query_threshold"`  // < this = simple (0.0-1.0)
	ComplexQueryThreshold float64  `json:"complex_query_threshold"` // > this = complex (0.0-1.0)
	SimpleModels          []string `json:"simple_models"`           // Cheap models for simple queries
	MediumModels          []string `json:"medium_models"`           // Mid-tier models
	ComplexModels         []string `json:"complex_models"`          // Premium models for complex queries
}

// LatencyRoutingConfig for latency-optimized routing
type LatencyRoutingConfig struct {
	MaxLatencyMs    int      `json:"max_latency_ms"`   // Route to providers with latency below this
	PreferredModels []string `json:"preferred_models"` // Try these first
}

// WeightedRoutingConfig for weighted distribution
type WeightedRoutingConfig struct {
	Weights map[string]int `json:"weights"` // provider -> weight (must sum to 100)
}

// CapabilityRoutingConfig for capability-based routing
type CapabilityRoutingConfig struct {
	TaskModels map[string][]string `json:"task_models"` // task type -> preferred models
}

// =============================================================================
// Resilience Policy Types (NEW)
// =============================================================================

// ResiliencePolicy controls failover, retry, and circuit breaker behavior per role
type ResiliencePolicy struct {
	// Master switch
	Enabled bool `json:"enabled"`

	// Retry configuration
	RetryEnabled    bool `json:"retry_enabled"`
	MaxRetries      int  `json:"max_retries"`       // Default: 3
	RetryBackoffMs  int  `json:"retry_backoff_ms"`  // Base backoff, default: 1000
	RetryBackoffMax int  `json:"retry_backoff_max"` // Max backoff, default: 30000
	RetryJitter     bool `json:"retry_jitter"`      // Add randomness to backoff

	// Retryable errors
	RetryOnTimeout     bool     `json:"retry_on_timeout"`
	RetryOnRateLimit   bool     `json:"retry_on_rate_limit"`
	RetryOnServerError bool     `json:"retry_on_server_error"` // 5xx errors
	RetryableErrors    []string `json:"retryable_errors"`      // Custom error codes

	// Fallback chain
	FallbackEnabled bool             `json:"fallback_enabled"`
	FallbackChain   []FallbackConfig `json:"fallback_chain"`

	// Circuit breaker
	CircuitBreakerEnabled   bool `json:"circuit_breaker_enabled"`
	CircuitBreakerThreshold int  `json:"circuit_breaker_threshold"` // Failures before open
	CircuitBreakerTimeout   int  `json:"circuit_breaker_timeout"`   // Seconds before half-open

	// Timeout
	RequestTimeoutMs int `json:"request_timeout_ms"` // Per-request timeout
}

// FallbackConfig defines a fallback provider in the chain
type FallbackConfig struct {
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	Priority  int    `json:"priority"`   // Lower = higher priority
	TimeoutMs int    `json:"timeout_ms"` // Timeout for this provider
}

// =============================================================================
// Budget Policy Types (NEW)
// =============================================================================

// BudgetPolicy controls budget limits and alerts per role
type BudgetPolicy struct {
	// Master switch
	Enabled bool `json:"enabled"`

	// Budget limits (0 = unlimited)
	DailyLimitUSD   float64 `json:"daily_limit_usd"`
	WeeklyLimitUSD  float64 `json:"weekly_limit_usd"`
	MonthlyLimitUSD float64 `json:"monthly_limit_usd"`

	// Per-request limits
	MaxCostPerRequest float64 `json:"max_cost_per_request"`

	// Alert thresholds (0.0-1.0)
	AlertThreshold    float64 `json:"alert_threshold"`    // Alert at this % of budget
	CriticalThreshold float64 `json:"critical_threshold"` // Critical alert at this %

	// Alert channels
	AlertWebhook string   `json:"alert_webhook"`
	AlertEmails  []string `json:"alert_emails"`
	AlertSlack   string   `json:"alert_slack"` // Slack webhook

	// Behavior when budget exceeded
	OnExceeded BudgetExceededAction `json:"on_exceeded"`

	// Soft limits (warn but allow)
	SoftLimitEnabled bool    `json:"soft_limit_enabled"`
	SoftLimitBuffer  float64 `json:"soft_limit_buffer"` // Allow this % over budget
}

// BudgetExceededAction defines what happens when budget is exceeded
type BudgetExceededAction string

const (
	BudgetActionBlock    BudgetExceededAction = "block"    // Block all requests
	BudgetActionWarn     BudgetExceededAction = "warn"     // Allow but warn
	BudgetActionThrottle BudgetExceededAction = "throttle" // Reduce rate limit
)

// =============================================================================
// Available Tool Definition
// =============================================================================

// AvailableTool represents a tool that can be enabled/disabled per role
type AvailableTool struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Category    string         `json:"category"`
	Schema      map[string]any `json:"schema"` // JSON Schema for the tool
	IsBuiltIn   bool           `json:"is_built_in"`
	Enabled     bool           `json:"enabled"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// =============================================================================
// Extended API Key with Role
// =============================================================================

// APIKeyWithRole extends APIKey with role information
type APIKeyWithRole struct {
	APIKey
	RoleID   string   `json:"role_id"`
	RoleName string   `json:"role_name,omitempty"`
	GroupIDs []string `json:"group_ids,omitempty"`
}

// =============================================================================
// Repository Interfaces
// =============================================================================

// RoleRepository is the interface for role storage
type RoleRepository interface {
	Create(tenantID string, role *Role) error
	Get(tenantID, id string) (*Role, error)
	GetByName(tenantID, name string) (*Role, error)
	Update(tenantID string, role *Role) error
	Delete(tenantID, id string) error
	List(tenantID string) ([]*Role, error)
	GetDefault(tenantID string) (*Role, error)
}

// RolePolicyRepository is the interface for role policy storage
type RolePolicyRepository interface {
	Create(tenantID string, policy *RolePolicy) error
	Get(tenantID, roleID string) (*RolePolicy, error)
	Update(tenantID string, policy *RolePolicy) error
	Delete(tenantID, roleID string) error
}

// GroupRepository is the interface for group storage
type GroupRepository interface {
	Create(tenantID string, group *Group) error
	Get(tenantID, id string) (*Group, error)
	GetByName(tenantID, name string) (*Group, error)
	Update(tenantID string, group *Group) error
	Delete(tenantID, id string) error
	List(tenantID string) ([]*Group, error)
	AddMember(tenantID string, member *GroupMember) error
	RemoveMember(tenantID, groupID, memberID string) error
	GetMembers(tenantID, groupID string) ([]*GroupMember, error)
	GetGroupsForMember(tenantID, memberID string) ([]*Group, error)
}

// AvailableToolRepository is the interface for available tool storage
type AvailableToolRepository interface {
	Create(tenantID string, tool *AvailableTool) error
	Get(tenantID, id string) (*AvailableTool, error)
	GetByName(tenantID, name string) (*AvailableTool, error)
	Update(tenantID string, tool *AvailableTool) error
	Delete(tenantID, id string) error
	List(tenantID string) ([]*AvailableTool, error)
	ListByCategory(tenantID, category string) ([]*AvailableTool, error)
}

// =============================================================================
// Default Roles
// =============================================================================

// DefaultRoles returns the default roles to create for new tenants
func DefaultRoles() []*Role {
	return []*Role{
		{
			Name:        "admin",
			Description: "Full administrative access",
			Permissions: []string{"*"},
			IsDefault:   false,
		},
		{
			Name:        "developer",
			Description: "Standard developer access with all models and tools",
			Permissions: []string{"chat:*", "embed:*", "models:list", "tools:*"},
			IsDefault:   true,
		},
		{
			Name:        "readonly",
			Description: "Read-only access for monitoring",
			Permissions: []string{"models:list", "usage:view"},
			IsDefault:   false,
		},
	}
}

// DefaultRolePolicy returns the default policy for a role
func DefaultRolePolicy(roleID, roleName string) *RolePolicy {
	policy := &RolePolicy{
		RoleID: roleID,

		// Prompt Policies with sensible defaults
		PromptPolicies: PromptPolicies{
			Normalization: NormalizationConfig{
				Enabled:              true,
				UnicodeNormalization: UnicodeNFKC,
				NormalizeNewlines:    true,
				StripNullBytes:       true,
				RemoveInvisibleChars: true,
			},
			InputBounds: InputBoundsConfig{
				Enabled:         true,
				MaxPromptLength: 100000,
				MaxMessageCount: 100,
			},
			DirectInjectionDetection: InjectionDetectionConfig{
				Enabled:         true,
				DetectionMethod: DetectionMethodHybrid,
				Sensitivity:     SensitivityMedium,
				OnDetection:     DetectionActionBlock,
				BlockThreshold:  0.85,
				PatternDetection: PatternDetectionConfig{
					Enabled:                    true,
					DetectIgnoreInstructions:   true,
					DetectSystemPromptRequests: true,
					DetectRoleConfusion:        true,
					DetectJailbreakPhrases:     true,
					DetectToolCoercion:         true,
				},
			},
			PIIPolicy: PIIPolicyConfig{
				Enabled:     true,
				ScanInputs:  true,
				ScanOutputs: true,
				OnDetection: PIIActionRedact,
			},
			SystemPromptProtection: SystemPromptProtectionConfig{
				Enabled:                  true,
				DetectExtractionAttempts: true,
			},
			OutputValidation: OutputValidationConfig{
				Enabled:          true,
				DetectPIILeakage: true,
			},
		},

		// Tool Policies
		ToolPolicies: ToolPolicies{
			AllowToolCalling:       true,
			AllowedTools:           []string{"*"},
			MaxToolCallsPerRequest: 50,
		},

		// Rate Limit Policies
		RateLimitPolicy: RateLimitPolicy{
			RequestsPerMinute: 60,
			RequestsPerHour:   1000,
			RequestsPerDay:    10000,
			TokensPerMinute:   100000,
			TokensPerHour:     1000000,
			TokensPerDay:      10000000,
			CostPerDayUSD:     100.0,
			CostPerMonthUSD:   1000.0,
			BurstLimit:        10,
		},

		// Model Restrictions - empty means all models allowed
		ModelRestriction: ModelRestrictions{},

		// NEW: Caching Policy (disabled by default)
		CachingPolicy: CachingPolicy{
			Enabled:             false,
			SimilarityThreshold: 0.95,
			TTLSeconds:          3600,
			MaxCacheSize:        1000,
			TrackSavings:        true,
		},

		// NEW: Routing Policy (disabled by default)
		RoutingPolicy: RoutingPolicy{
			Enabled:            false,
			Strategy:           RoutingStrategyCost,
			AllowModelOverride: true,
			CostConfig: &CostRoutingConfig{
				SimpleQueryThreshold:  0.3,
				ComplexQueryThreshold: 0.7,
			},
		},

		// NEW: Resilience Policy (disabled by default)
		ResiliencePolicy: ResiliencePolicy{
			Enabled:                 false,
			RetryEnabled:            true,
			MaxRetries:              3,
			RetryBackoffMs:          1000,
			RetryBackoffMax:         30000,
			RetryJitter:             true,
			RetryOnTimeout:          true,
			RetryOnRateLimit:        true,
			RetryOnServerError:      true,
			FallbackEnabled:         false,
			CircuitBreakerEnabled:   false,
			CircuitBreakerThreshold: 5,
			CircuitBreakerTimeout:   60,
			RequestTimeoutMs:        30000,
		},

		// NEW: Budget Policy (disabled by default)
		BudgetPolicy: BudgetPolicy{
			Enabled:           false,
			AlertThreshold:    0.8,
			CriticalThreshold: 0.95,
			OnExceeded:        BudgetActionWarn,
		},
	}

	// Readonly role has more restrictive defaults
	if roleName == "readonly" {
		policy.ToolPolicies.AllowToolCalling = false
		policy.RateLimitPolicy.RequestsPerMinute = 10
		policy.RateLimitPolicy.TokensPerMinute = 10000
	}

	return policy
}
