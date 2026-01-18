package domain

import (
	"encoding/json"
	"time"
)

// MCPServerType defines the transport type for MCP servers
type MCPServerType string

const (
	MCPServerTypeStdio     MCPServerType = "stdio"     // Local process
	MCPServerTypeSSE       MCPServerType = "sse"       // Server-Sent Events (HTTP)
	MCPServerTypeWebSocket MCPServerType = "websocket" // WebSocket
)

// MCPAuthType defines authentication methods for MCP servers
type MCPAuthType string

const (
	MCPAuthNone   MCPAuthType = "none"
	MCPAuthAPIKey MCPAuthType = "api_key"
	MCPAuthBearer MCPAuthType = "bearer"
	MCPAuthOAuth2 MCPAuthType = "oauth2"
	MCPAuthBasic  MCPAuthType = "basic"
	MCPAuthMTLS   MCPAuthType = "mtls"
	MCPAuthAWSIAM MCPAuthType = "aws_iam"
)

// MCPServerStatus represents the connection status of an MCP server
type MCPServerStatus string

const (
	MCPStatusPending      MCPServerStatus = "pending"
	MCPStatusConnected    MCPServerStatus = "connected"
	MCPStatusDisconnected MCPServerStatus = "disconnected"
	MCPStatusError        MCPServerStatus = "error"
	MCPStatusDisabled     MCPServerStatus = "disabled"
)

// MCPServer represents an MCP server configuration
type MCPServer struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description,omitempty"`

	// Connection settings
	ServerType  MCPServerType     `json:"server_type"`
	Endpoint    string            `json:"endpoint"`
	Arguments   []string          `json:"arguments,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`

	// Authentication
	AuthType   MCPAuthType   `json:"auth_type"`
	AuthConfig MCPAuthConfig `json:"auth_config,omitempty"`

	// Version control
	Version    string     `json:"version,omitempty"`
	CommitHash string     `json:"commit_hash,omitempty"`
	LastSyncAt *time.Time `json:"last_sync_at,omitempty"`

	// Status
	Status          MCPServerStatus `json:"status"`
	LastHealthCheck *time.Time      `json:"last_health_check,omitempty"`
	ErrorMessage    string          `json:"error_message,omitempty"`
	RetryCount      int             `json:"retry_count"`

	// Settings
	AutoSync                   bool `json:"auto_sync"`
	SyncIntervalMinutes        int  `json:"sync_interval_minutes"`
	HealthCheckIntervalSeconds int  `json:"health_check_interval_seconds"`

	// Metadata
	Tags      []string          `json:"tags,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	ToolCount int               `json:"tool_count"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	CreatedBy string            `json:"created_by,omitempty"`
}

// MCPAuthConfig stores authentication credentials for MCP servers
type MCPAuthConfig struct {
	// API Key auth
	APIKey       string `json:"api_key,omitempty"`
	APIKeyHeader string `json:"api_key_header,omitempty"`

	// Bearer Token auth
	BearerToken string `json:"bearer_token,omitempty"`

	// OAuth2
	ClientID     string   `json:"client_id,omitempty"`
	ClientSecret string   `json:"client_secret,omitempty"`
	TokenURL     string   `json:"token_url,omitempty"`
	Scopes       []string `json:"scopes,omitempty"`

	// Basic auth
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`

	// mTLS
	ClientCert string `json:"client_cert,omitempty"`
	ClientKey  string `json:"client_key,omitempty"`
	CACert     string `json:"ca_cert,omitempty"`

	// AWS IAM
	AWSRegion  string `json:"aws_region,omitempty"`
	AWSRoleARN string `json:"aws_role_arn,omitempty"`
}

// MCPTool represents a tool discovered from an MCP server
type MCPTool struct {
	ID         string `json:"id"`
	ServerID   string `json:"server_id"`
	ServerName string `json:"server_name,omitempty"`

	// Identity
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`

	// Schema
	InputSchema   map[string]any   `json:"input_schema"`
	OutputSchema  map[string]any   `json:"output_schema,omitempty"`
	InputExamples []map[string]any `json:"input_examples,omitempty"`

	// Embeddings (stored as float32 slices)
	NameEmbedding        []float32 `json:"-"`
	DescriptionEmbedding []float32 `json:"-"`
	CombinedEmbedding    []float32 `json:"-"`

	// Deferred loading (per Anthropic's pattern)
	DeferLoading bool `json:"defer_loading"`

	// Status
	IsDeprecated       bool       `json:"is_deprecated"`
	DeprecationMessage string     `json:"deprecation_message,omitempty"`
	DeprecatedAt       *time.Time `json:"deprecated_at,omitempty"`

	// Metrics
	Version            string     `json:"version,omitempty"`
	ExecutionCount     int64      `json:"execution_count"`
	LastExecutedAt     *time.Time `json:"last_executed_at,omitempty"`
	AvgExecutionTimeMs int        `json:"avg_execution_time_ms"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MCPServerVersion represents a version snapshot of an MCP server
type MCPServerVersion struct {
	ID         string `json:"id"`
	ServerID   string `json:"server_id"`
	Version    string `json:"version"`
	CommitHash string `json:"commit_hash,omitempty"`

	// Snapshot
	ToolDefinitions []MCPTool `json:"tool_definitions"`
	ToolCount       int       `json:"tool_count"`

	// Changes
	Changes            []MCPSchemaChange `json:"changes,omitempty"`
	ChangesSummary     string            `json:"changes_summary,omitempty"`
	HasBreakingChanges bool              `json:"has_breaking_changes"`

	CreatedAt time.Time `json:"created_at"`
	CreatedBy string    `json:"created_by,omitempty"`
}

// MCPSchemaChange represents a change between versions
type MCPSchemaChange struct {
	Type     MCPChangeType `json:"type"`
	ToolName string        `json:"tool_name"`
	Field    string        `json:"field,omitempty"`
	OldValue any           `json:"old_value,omitempty"`
	NewValue any           `json:"new_value,omitempty"`
	Breaking bool          `json:"breaking"`
}

// MCPChangeType defines the type of schema change
type MCPChangeType string

const (
	MCPChangeAdded    MCPChangeType = "ADDED"
	MCPChangeRemoved  MCPChangeType = "REMOVED"
	MCPChangeModified MCPChangeType = "MODIFIED"
)

// MCPToolVisibility defines how a tool is exposed via MCP
type MCPToolVisibility string

const (
	// MCPVisibilityDeny - Tool is completely hidden and blocked (default)
	MCPVisibilityDeny MCPToolVisibility = "DENY"
	// MCPVisibilitySearch - Tool is only available via tool_search, not in tools/list
	MCPVisibilitySearch MCPToolVisibility = "SEARCH"
	// MCPVisibilityAllow - Tool is visible in tools/list AND searchable
	MCPVisibilityAllow MCPToolVisibility = "ALLOW"
)

// MCPToolPermission represents visibility permission for an MCP tool
type MCPToolPermission struct {
	ID     string `json:"id"`
	RoleID string `json:"role_id"`

	// Target - must be a specific tool (server-level removed)
	ServerID string `json:"server_id"`
	ToolID   string `json:"tool_id"`

	// Visibility state (DENY, SEARCH, ALLOW)
	Visibility MCPToolVisibility `json:"visibility"`

	// Audit
	DecidedBy      string     `json:"decided_by,omitempty"`
	DecidedByEmail string     `json:"decided_by_email,omitempty"`
	DecidedAt      *time.Time `json:"decided_at,omitempty"`
	DecisionReason string     `json:"decision_reason,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MCPToolExecution represents a tool execution log entry
type MCPToolExecution struct {
	ID       string `json:"id"`
	ServerID string `json:"server_id"`
	ToolID   string `json:"tool_id"`

	// Context
	RoleID    string `json:"role_id,omitempty"`
	APIKeyID  string `json:"api_key_id,omitempty"`
	RequestID string `json:"request_id,omitempty"`

	// Execution
	InputParams  map[string]any     `json:"input_params,omitempty"`
	OutputResult map[string]any     `json:"output_result,omitempty"`
	Status       MCPExecutionStatus `json:"status"`
	ErrorMessage string             `json:"error_message,omitempty"`

	// Timing
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	DurationMs  int        `json:"duration_ms"`

	CreatedAt time.Time `json:"created_at"`
}

// MCPExecutionStatus represents the status of a tool execution
type MCPExecutionStatus string

const (
	MCPExecSuccess MCPExecutionStatus = "SUCCESS"
	MCPExecError   MCPExecutionStatus = "ERROR"
	MCPExecBlocked MCPExecutionStatus = "BLOCKED"
	MCPExecTimeout MCPExecutionStatus = "TIMEOUT"
)

// SearchStrategy defines how to search for tools
type SearchStrategy string

const (
	SearchStrategyRegex    SearchStrategy = "regex"
	SearchStrategyBM25     SearchStrategy = "bm25"
	SearchStrategySemantic SearchStrategy = "semantic"
	SearchStrategyHybrid   SearchStrategy = "hybrid"
)

// ToolSearchRequest represents a search query
type ToolSearchRequest struct {
	Query         string         `json:"query"`
	Strategy      SearchStrategy `json:"strategy"`
	ServerIDs     []string       `json:"server_ids,omitempty"`
	Categories    []string       `json:"categories,omitempty"`
	MaxResults    int            `json:"max_results"`
	MinScore      float64        `json:"min_score"`
	IncludeSchema bool           `json:"include_schema"`
}

// ToolSearchResult represents a matched tool
type ToolSearchResult struct {
	Tool        *MCPTool `json:"tool"`
	ServerID    string   `json:"server_id"`
	ServerName  string   `json:"server_name"`
	Score       float64  `json:"score"`
	MatchReason string   `json:"match_reason,omitempty"`

	// Deferred loading support
	DeferLoading bool   `json:"defer_loading"`
	ToolRef      string `json:"tool_ref"`
}

// ToolSearchResponse represents the search response
type ToolSearchResponse struct {
	Tools          []*ToolSearchResult `json:"tools"`
	Query          string              `json:"query"`
	TotalAvailable int                 `json:"total_available"`
	TotalAllowed   int                 `json:"total_allowed"`
}

// EnhancedMCPPolicies defines MCP-specific access control in role policy
type EnhancedMCPPolicies struct {
	Enabled         bool   `json:"enabled"`
	AllowToolSearch bool   `json:"allow_tool_search"`
	DefaultAction   string `json:"default_action"` // ALLOW, DENY, REQUIRE_APPROVAL

	// Server-level permissions
	ServerPermissions []MCPServerPermission `json:"server_permissions,omitempty"`

	// Tool-level overrides
	ToolOverrides []MCPToolOverride `json:"tool_overrides,omitempty"`

	// Category restrictions
	AllowedCategories []string `json:"allowed_categories,omitempty"`
	DeniedCategories  []string `json:"denied_categories,omitempty"`

	// Audit settings
	LogAllToolCalls      bool `json:"log_all_tool_calls"`
	RequireJustification bool `json:"require_justification"`
}

// MCPServerPermission defines server-level access
type MCPServerPermission struct {
	ServerID     string   `json:"server_id"`
	ServerName   string   `json:"server_name"`
	Permission   string   `json:"permission"` // ALLOWED, DENIED
	AllowedTools []string `json:"allowed_tools,omitempty"`
	DeniedTools  []string `json:"denied_tools,omitempty"`
}

// MCPToolOverride defines tool-level access override
type MCPToolOverride struct {
	ServerID   string `json:"server_id"`
	ToolName   string `json:"tool_name"`
	Permission string `json:"permission"` // ALLOWED, DENIED, REMOVED
	Reason     string `json:"reason,omitempty"`
}

// DefaultMCPPolicies returns default MCP policy settings
func DefaultMCPPolicies() EnhancedMCPPolicies {
	return EnhancedMCPPolicies{
		Enabled:              false,
		AllowToolSearch:      true,
		DefaultAction:        "DENY",
		LogAllToolCalls:      true,
		RequireJustification: false,
	}
}

// MarshalJSON custom marshaler for MCPAuthConfig to handle sensitive fields
func (c MCPAuthConfig) MarshalJSON() ([]byte, error) {
	type Alias MCPAuthConfig
	masked := Alias(c)

	// Mask sensitive fields for JSON output
	if masked.APIKey != "" {
		masked.APIKey = "***"
	}
	if masked.BearerToken != "" {
		masked.BearerToken = "***"
	}
	if masked.ClientSecret != "" {
		masked.ClientSecret = "***"
	}
	if masked.Password != "" {
		masked.Password = "***"
	}
	if masked.ClientKey != "" {
		masked.ClientKey = "***"
	}

	return json.Marshal(masked)
}

// MCPRepository defines the interface for MCP data operations
type MCPRepository interface {
	// Servers
	CreateMCPServer(ctx any, server *MCPServer) error
	UpdateMCPServer(ctx any, server *MCPServer) error
	DeleteMCPServer(ctx any, serverID string) error
	GetMCPServer(ctx any, serverID string) (*MCPServer, error)
	GetMCPServerByName(ctx any, tenantID, name string) (*MCPServer, error)
	ListMCPServers(ctx any, tenantID string) ([]*MCPServer, error)

	// Tools
	UpsertMCPTool(ctx any, tool *MCPTool) error
	DeleteMCPTool(ctx any, toolID string) error
	DeleteMCPToolsByServer(ctx any, serverID string) error
	GetMCPTool(ctx any, toolID string) (*MCPTool, error)
	GetMCPToolByName(ctx any, serverID, name string) (*MCPTool, error)
	ListMCPTools(ctx any, serverID string) ([]*MCPTool, error)
	ListMCPToolsByTenant(ctx any, tenantID string) ([]*MCPTool, error)
	UpdateToolEmbeddings(ctx any, toolID string, nameEmb, descEmb, combinedEmb []float32) error

	// Versions
	CreateMCPServerVersion(ctx any, version *MCPServerVersion) error
	GetMCPServerVersion(ctx any, versionID string) (*MCPServerVersion, error)
	GetLatestMCPServerVersion(ctx any, serverID string) (*MCPServerVersion, error)
	ListMCPServerVersions(ctx any, serverID string) ([]*MCPServerVersion, error)

	// Permissions
	GetMCPToolPermission(ctx any, roleID, toolID string) (*MCPToolPermission, error)
	GetMCPServerPermission(ctx any, roleID, serverID string) (*MCPToolPermission, error)
	SetMCPToolPermission(ctx any, perm *MCPToolPermission) error
	ListMCPPermissions(ctx any, roleID string) ([]*MCPToolPermission, error)
	BulkSetMCPPermissions(ctx any, roleID string, serverIDs []string, status ToolPermissionStatus) (int, error)

	// Search
	SearchToolsByVector(ctx any, tenantID string, embedding []float32, limit int) ([]*MCPTool, error)
	SearchToolsByText(ctx any, tenantID, query string, limit int) ([]*MCPTool, error)

	// Executions
	LogMCPToolExecution(ctx any, exec *MCPToolExecution) error
	ListMCPToolExecutions(ctx any, tenantID string, limit, offset int) ([]*MCPToolExecution, int, error)
}
