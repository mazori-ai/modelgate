package domain

import (
	"context"
	"time"
)

// ToolPermissionStatus represents the permission state for a tool
type ToolPermissionStatus string

const (
	ToolStatusPending ToolPermissionStatus = "PENDING"
	ToolStatusAllowed ToolPermissionStatus = "ALLOWED"
	ToolStatusDenied  ToolPermissionStatus = "DENIED"
	ToolStatusRemoved ToolPermissionStatus = "REMOVED" // Tool is stripped from request before sending to LLM
)

// DiscoveredTool represents a tool discovered from incoming requests
type DiscoveredTool struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"` // One-line description from tool spec
	SchemaHash  string    `json:"schema_hash"` // SHA256 of canonical JSON schema
	Parameters  any       `json:"parameters"`  // Full JSON schema for parameters
	FirstSeenAt time.Time `json:"first_seen_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
	FirstSeenBy string    `json:"first_seen_by,omitempty"` // API Key ID
	SeenCount   int       `json:"seen_count"`
	Category    string    `json:"category,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ToolRolePermission represents the permission for a tool-role combination
type ToolRolePermission struct {
	ID             string               `json:"id"`
	ToolID         string               `json:"tool_id"`
	RoleID         string               `json:"role_id"`
	Status         ToolPermissionStatus `json:"status"`
	DecidedBy      string               `json:"decided_by,omitempty"`
	DecidedByEmail string               `json:"decided_by_email,omitempty"`
	DecidedAt      *time.Time           `json:"decided_at,omitempty"`
	DecisionReason string               `json:"decision_reason,omitempty"`
	CreatedAt      time.Time            `json:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at"`

	// Joined fields (populated when fetching with tool data)
	Tool *DiscoveredTool `json:"tool,omitempty"`
	Role *Role           `json:"role,omitempty"`
}

// ToolExecutionLog represents an audit log entry for a tool call attempt
type ToolExecutionLog struct {
	ID            string    `json:"id"`
	ToolID        string    `json:"tool_id"`
	RoleID        string    `json:"role_id"`
	APIKeyID      string    `json:"api_key_id,omitempty"`
	RequestID     string    `json:"request_id,omitempty"`
	ToolName      string    `json:"tool_name"`
	ToolArguments any       `json:"tool_arguments,omitempty"`
	Status        string    `json:"status"` // ALLOWED, BLOCKED
	BlockReason   string    `json:"block_reason,omitempty"`
	ExecutedAt    time.Time `json:"executed_at"`
	Model         string    `json:"model,omitempty"`
	IPAddress     string    `json:"ip_address,omitempty"`
}

// EnhancedToolPolicies extends the existing ToolPolicies with new fields
type EnhancedToolPolicies struct {
	// Master switch - if false, all tool calls are allowed without restriction
	Enabled bool `json:"enabled"`

	// Default behavior for new/unknown tools: "BLOCK" or "ALLOW"
	DefaultAction string `json:"default_action"`

	// Notification settings
	NotifyOnNewTool     bool `json:"notify_on_new_tool"`
	NotifyOnBlockedCall bool `json:"notify_on_blocked_call"`

	// Execution constraints
	MaxToolsPerRequest  int `json:"max_tools_per_request"`
	MaxExecutionsPerMin int `json:"max_executions_per_min"`

	// Validation
	ValidateArguments bool `json:"validate_arguments"`
	SanitizeArguments bool `json:"sanitize_arguments"`
}

// DefaultEnhancedToolPolicies returns default tool policy settings
func DefaultEnhancedToolPolicies() EnhancedToolPolicies {
	return EnhancedToolPolicies{
		Enabled:             true,
		DefaultAction:       "BLOCK", // Default deny
		NotifyOnNewTool:     true,
		NotifyOnBlockedCall: false,
		MaxToolsPerRequest:  10,
		MaxExecutionsPerMin: 100,
		ValidateArguments:   false,
		SanitizeArguments:   true,
	}
}

// ToolFilter represents filters for querying tools
type ToolFilter struct {
	Name     string               `json:"name,omitempty"`
	Category string               `json:"category,omitempty"`
	Status   ToolPermissionStatus `json:"status,omitempty"`
	RoleID   string               `json:"role_id,omitempty"` // Required if filtering by status
}

// ToolRepository defines the interface for tool data operations
type ToolRepository interface {
	// Tool discovery
	CreateTool(ctx context.Context, tool *DiscoveredTool) error
	GetTool(ctx context.Context, id string) (*DiscoveredTool, error)
	GetToolByIdentity(ctx context.Context, tenantID, name, description, schemaHash string) (*DiscoveredTool, error)
	UpdateToolSeen(ctx context.Context, id string) error
	ListTools(ctx context.Context, tenantID string, filter ToolFilter, limit, offset int) ([]*DiscoveredTool, int, error)
	DeleteTool(ctx context.Context, id string) error

	// Tool permissions
	CreateToolPermission(ctx context.Context, perm *ToolRolePermission) error
	GetToolPermission(ctx context.Context, toolID, roleID string) (*ToolRolePermission, error)
	UpdateToolPermission(ctx context.Context, perm *ToolRolePermission) error
	ListToolPermissions(ctx context.Context, roleID string) ([]*ToolRolePermission, error)
	ListPendingTools(ctx context.Context, tenantID string) ([]*DiscoveredTool, error)
	BulkUpdatePermissions(ctx context.Context, roleID string, status ToolPermissionStatus, decidedBy, decidedByEmail string) (int, error)

	// Execution logging
	LogToolExecution(ctx context.Context, log *ToolExecutionLog) error
	ListToolExecutionLogs(ctx context.Context, tenantID string, filter ToolLogFilter, limit, offset int) ([]*ToolExecutionLog, int, error)
}

// ToolLogFilter represents filters for querying tool execution logs
type ToolLogFilter struct {
	ToolID    string    `json:"tool_id,omitempty"`
	RoleID    string    `json:"role_id,omitempty"`
	Status    string    `json:"status,omitempty"` // ALLOWED, BLOCKED
	StartTime time.Time `json:"start_time,omitempty"`
	EndTime   time.Time `json:"end_time,omitempty"`
}
