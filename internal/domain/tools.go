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

// RoleTool represents a tool discovered for a specific role
// This is the unified model that combines tool identity, usage tracking, and permissions
// Each role has its own set of tools - the same tool can exist in multiple roles
type RoleTool struct {
	ID     string `json:"id"`
	RoleID string `json:"role_id"`

	// Tool identity
	Name        string `json:"name"`
	Description string `json:"description"` // One-line description from tool spec
	SchemaHash  string `json:"schema_hash"` // SHA256 of canonical JSON schema
	Parameters  any    `json:"parameters"`  // Full JSON schema for parameters
	Category    string `json:"category,omitempty"`

	// Usage tracking
	FirstSeenAt time.Time `json:"first_seen_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
	FirstSeenBy string    `json:"first_seen_by,omitempty"` // API Key ID that first used this tool
	SeenCount   int       `json:"seen_count"`

	// Permission (inline - no separate table needed)
	Status         ToolPermissionStatus `json:"status"`
	DecidedBy      string               `json:"decided_by,omitempty"`
	DecidedByEmail string               `json:"decided_by_email,omitempty"`
	DecidedAt      *time.Time           `json:"decided_at,omitempty"`
	DecisionReason string               `json:"decision_reason,omitempty"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DiscoveredTool is an alias for RoleTool for backward compatibility in some contexts
// Deprecated: Use RoleTool instead
type DiscoveredTool = RoleTool

// ToolRolePermission is kept for backward compatibility but now just wraps RoleTool
// Deprecated: Use RoleTool instead - permissions are now inline
type ToolRolePermission struct {
	ID             string               `json:"id"`
	ToolID         string               `json:"tool_id"` // Same as RoleTool.ID
	RoleID         string               `json:"role_id"`
	Status         ToolPermissionStatus `json:"status"`
	DecidedBy      string               `json:"decided_by,omitempty"`
	DecidedByEmail string               `json:"decided_by_email,omitempty"`
	DecidedAt      *time.Time           `json:"decided_at,omitempty"`
	DecisionReason string               `json:"decision_reason,omitempty"`
	CreatedAt      time.Time            `json:"created_at"`
	UpdatedAt      time.Time            `json:"updated_at"`

	// Joined fields (for backward compatibility)
	Tool *RoleTool `json:"tool,omitempty"`
	Role *Role     `json:"role,omitempty"`
}

// ToolExecutionLog represents an audit log entry for a tool call attempt
type ToolExecutionLog struct {
	ID            string    `json:"id"`
	RoleToolID    string    `json:"role_tool_id,omitempty"` // Reference to role_tools table
	ToolName      string    `json:"tool_name"`              // Stored for audit even if tool deleted
	RoleID        string    `json:"role_id"`
	APIKeyID      string    `json:"api_key_id,omitempty"`
	RequestID     string    `json:"request_id,omitempty"`
	ToolArguments any       `json:"tool_arguments,omitempty"`
	Status        string    `json:"status"` // ALLOWED, BLOCKED, REMOVED
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

// ToolFilter represents filters for querying role tools
type ToolFilter struct {
	Name     string               `json:"name,omitempty"`
	Category string               `json:"category,omitempty"`
	Status   ToolPermissionStatus `json:"status,omitempty"`
}

// ToolRepository defines the interface for role-scoped tool operations
type ToolRepository interface {
	// Role tool operations (combined discovery + permissions)
	CreateOrUpdateRoleTool(ctx context.Context, tool *RoleTool) error
	GetRoleTool(ctx context.Context, id string) (*RoleTool, error)
	GetRoleToolByIdentity(ctx context.Context, roleID, name, schemaHash string) (*RoleTool, error)
	UpdateRoleToolSeen(ctx context.Context, id string) error
	ListRoleTools(ctx context.Context, roleID string, filter ToolFilter, limit, offset int) ([]*RoleTool, int, error)
	DeleteRoleTool(ctx context.Context, id string) error

	// Permission updates (inline in role_tools table)
	SetRoleToolPermission(ctx context.Context, id string, status ToolPermissionStatus, decidedBy, decidedByEmail, reason string) error
	BulkSetRoleToolPermissions(ctx context.Context, roleID string, status ToolPermissionStatus, decidedBy, decidedByEmail string) (int, error)

	// Execution logging
	LogToolExecution(ctx context.Context, log *ToolExecutionLog) error
	ListToolExecutionLogs(ctx context.Context, filter ToolLogFilter, limit, offset int) ([]*ToolExecutionLog, int, error)
}

// ToolLogFilter represents filters for querying tool execution logs
type ToolLogFilter struct {
	ToolID    string    `json:"tool_id,omitempty"`
	RoleID    string    `json:"role_id,omitempty"`
	Status    string    `json:"status,omitempty"` // ALLOWED, BLOCKED
	StartTime time.Time `json:"start_time,omitempty"`
	EndTime   time.Time `json:"end_time,omitempty"`
}
