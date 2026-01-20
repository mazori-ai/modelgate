package policy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"

	"modelgate/internal/domain"

	"github.com/google/uuid"
)

// ToolDiscoveryService handles tool discovery and permission checking
type ToolDiscoveryService struct {
	mu sync.RWMutex
	// Cache of tool hashes to IDs for quick lookup
	// Key: roleID:name:schemaHash -> toolID
	toolCache map[string]string
}

// NewToolDiscoveryService creates a new tool discovery service
func NewToolDiscoveryService() *ToolDiscoveryService {
	return &ToolDiscoveryService{
		toolCache: make(map[string]string),
	}
}

// ToolSpec represents a tool definition from a request
type ToolSpec struct {
	Type     string `json:"type"`
	Function struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Parameters  any    `json:"parameters"`
	} `json:"function"`
}

// ComputeSchemaHash computes a canonical hash of the tool parameters schema
func ComputeSchemaHash(parameters any) string {
	// Canonicalize the JSON by marshaling and unmarshaling
	canonical, err := canonicalizeJSON(parameters)
	if err != nil {
		// Fallback to direct marshal
		data, _ := json.Marshal(parameters)
		hash := sha256.Sum256(data)
		return hex.EncodeToString(hash[:])
	}

	hash := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(hash[:])
}

// canonicalizeJSON produces a canonical JSON string with sorted keys
func canonicalizeJSON(v any) (string, error) {
	// First marshal to JSON
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}

	// Unmarshal to interface
	var parsed any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", err
	}

	// Recursively sort and marshal
	sorted := sortJSON(parsed)
	canonical, err := json.Marshal(sorted)
	if err != nil {
		return "", err
	}

	return string(canonical), nil
}

// sortJSON recursively sorts JSON objects by key
func sortJSON(v any) any {
	switch val := v.(type) {
	case map[string]any:
		// Sort keys
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		// Create ordered map representation as slice of pairs
		result := make(map[string]any, len(val))
		for _, k := range keys {
			result[k] = sortJSON(val[k])
		}
		return result

	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = sortJSON(item)
		}
		return result

	default:
		return val
	}
}

// RoleToolStore interface for role-scoped tool operations
type RoleToolStore interface {
	CreateOrUpdateRoleTool(ctx context.Context, tool *domain.RoleTool) error
	GetRoleTool(ctx context.Context, id string) (*domain.RoleTool, error)
	GetRoleToolByIdentity(ctx context.Context, roleID, name, schemaHash string) (*domain.RoleTool, error)
	UpdateRoleToolSeen(ctx context.Context, id string) error
	LogToolExecution(ctx context.Context, log *domain.ToolExecutionLog) error
}

// DiscoverToolsForRole processes tools from a request and stores them for a specific role
// This is the new role-scoped version - each role has its own set of discovered tools
func (s *ToolDiscoveryService) DiscoverToolsForRole(
	ctx context.Context,
	roleID string,
	apiKeyID string,
	tools []domain.Tool,
	store RoleToolStore,
) ([]*domain.RoleTool, error) {
	discovered := make([]*domain.RoleTool, 0, len(tools))

	for _, tool := range tools {
		if tool.Type != "function" {
			continue
		}

		name := tool.Function.Name
		description := tool.Function.Description
		if description == "" {
			description = "(no description)"
		}
		parameters := tool.Function.Parameters
		if parameters == nil {
			parameters = map[string]any{}
		}

		schemaHash := ComputeSchemaHash(parameters)

		// Check cache first (role-scoped cache key)
		cacheKey := fmt.Sprintf("%s:%s:%s", roleID, name, schemaHash)
		s.mu.RLock()
		toolID, cached := s.toolCache[cacheKey]
		s.mu.RUnlock()

		if cached {
			// Tool exists in cache, verify it still exists in database
			existing, err := store.GetRoleTool(ctx, toolID)
			if err == nil && existing != nil {
				// Tool still exists, update last seen
				if err := store.UpdateRoleToolSeen(ctx, toolID); err != nil {
					slog.Warn("Failed to update tool seen", "tool_id", toolID, "error", err)
				}
				discovered = append(discovered, existing)
				continue
			}
			// Tool was deleted from database - invalidate cache and recreate
			slog.Info("Tool in cache but not in database, invalidating cache", "tool_id", toolID, "name", name)
			s.mu.Lock()
			delete(s.toolCache, cacheKey)
			s.mu.Unlock()
		}

		// Check database for existing tool (by role + name + schema_hash)
		existing, err := store.GetRoleToolByIdentity(ctx, roleID, name, schemaHash)
		if err != nil {
			slog.Warn("Failed to check tool existence", "name", name, "role_id", roleID, "error", err)
			continue
		}

		if existing != nil {
			// Tool exists, update cache and last seen
			slog.Debug("Found existing role tool", "name", name, "role_id", roleID, "tool_id", existing.ID)
			s.mu.Lock()
			s.toolCache[cacheKey] = existing.ID
			s.mu.Unlock()

			if err := store.UpdateRoleToolSeen(ctx, existing.ID); err != nil {
				slog.Warn("Failed to update tool seen", "tool_id", existing.ID, "error", err)
			}

			discovered = append(discovered, existing)
			continue
		}

		// New tool for this role - create it
		newTool := &domain.RoleTool{
			ID:          uuid.New().String(),
			RoleID:      roleID,
			Name:        name,
			Description: description,
			SchemaHash:  schemaHash,
			Parameters:  parameters,
			FirstSeenBy: apiKeyID,
			SeenCount:   1,
			Category:    inferCategory(name, description),
			Status:      domain.ToolStatusPending, // New tools start as pending
		}

		if err := store.CreateOrUpdateRoleTool(ctx, newTool); err != nil {
			slog.Warn("Failed to create/update role tool", "name", name, "role_id", roleID, "error", err)
			continue
		}

		slog.Info("Discovered new tool for role",
			"tool_id", newTool.ID,
			"name", name,
			"role_id", roleID)

		// Update cache with actual ID (might be different if upsert returned existing)
		s.mu.Lock()
		s.toolCache[cacheKey] = newTool.ID
		s.mu.Unlock()

		discovered = append(discovered, newTool)
	}

	return discovered, nil
}

// inferCategory tries to infer a tool category from its name and description
func inferCategory(name, description string) string {
	lower := strings.ToLower(name + " " + description)

	switch {
	case strings.Contains(lower, "file") || strings.Contains(lower, "read") ||
		strings.Contains(lower, "write") || strings.Contains(lower, "directory"):
		return "file_operations"

	case strings.Contains(lower, "http") || strings.Contains(lower, "request") ||
		strings.Contains(lower, "api") || strings.Contains(lower, "fetch"):
		return "network"

	case strings.Contains(lower, "execute") || strings.Contains(lower, "command") ||
		strings.Contains(lower, "shell") || strings.Contains(lower, "system"):
		return "system"

	case strings.Contains(lower, "database") || strings.Contains(lower, "sql") ||
		strings.Contains(lower, "query"):
		return "database"

	case strings.Contains(lower, "search") || strings.Contains(lower, "find"):
		return "search"

	case strings.Contains(lower, "calculate") || strings.Contains(lower, "math"):
		return "math"

	case strings.Contains(lower, "time") || strings.Contains(lower, "date"):
		return "datetime"

	case strings.Contains(lower, "parse") || strings.Contains(lower, "json") ||
		strings.Contains(lower, "csv") || strings.Contains(lower, "xml"):
		return "parsing"

	default:
		return "general"
	}
}

// CheckToolPermissions checks if all tools are allowed for a role
// Now simplified since permissions are inline in the RoleTool
func (s *ToolDiscoveryService) CheckToolPermissions(
	ctx context.Context,
	roleID string,
	tools []*domain.RoleTool,
	policy domain.EnhancedToolPolicies,
	store RoleToolStore,
) (*ToolPolicyResult, error) {
	result := &ToolPolicyResult{
		Allowed: true,
		Tools:   make([]ToolCheckResult, 0, len(tools)),
	}

	for _, tool := range tools {
		checkResult := ToolCheckResult{
			ToolID:   tool.ID,
			ToolName: tool.Name,
		}

		// Permission is now inline in the tool
		switch tool.Status {
		case domain.ToolStatusAllowed:
			checkResult.Status = "ALLOWED"
			checkResult.Reason = "explicitly allowed"

		case domain.ToolStatusDenied:
			checkResult.Status = "BLOCKED"
			if tool.DecisionReason != "" {
				checkResult.Reason = tool.DecisionReason
			} else {
				checkResult.Reason = "explicitly denied"
			}
			result.Allowed = false

		case domain.ToolStatusRemoved:
			checkResult.Status = "REMOVED"
			if tool.DecisionReason != "" {
				checkResult.Reason = tool.DecisionReason
			} else {
				checkResult.Reason = "tool removed from request"
			}
			// REMOVED does NOT set result.Allowed = false - request continues without this tool

		case domain.ToolStatusPending:
			// Use default action for pending tools
			if policy.DefaultAction == "ALLOW" {
				checkResult.Status = "ALLOWED"
				checkResult.Reason = "pending review (default allow)"
			} else {
				checkResult.Status = "BLOCKED"
				checkResult.Reason = "pending review (default deny)"
				result.Allowed = false
			}

		default:
			// Unknown status - use default action
			slog.Warn("Unknown tool permission status", "tool_id", tool.ID, "tool_name", tool.Name, "status", tool.Status)
			if policy.DefaultAction == "ALLOW" {
				checkResult.Status = "ALLOWED"
				checkResult.Reason = "unknown status, default allow"
			} else {
				checkResult.Status = "BLOCKED"
				checkResult.Reason = "unknown status, default deny"
				result.Allowed = false
			}
		}

		result.Tools = append(result.Tools, checkResult)
	}

	return result, nil
}

// ToolPolicyResult represents the result of checking tool permissions
type ToolPolicyResult struct {
	Allowed bool              `json:"allowed"`
	Tools   []ToolCheckResult `json:"tools"`
}

// ToolCheckResult represents the permission check result for a single tool
type ToolCheckResult struct {
	ToolID   string `json:"tool_id"`
	ToolName string `json:"tool_name"`
	Status   string `json:"status"` // ALLOWED, BLOCKED, REMOVED, ERROR
	Reason   string `json:"reason"`
}

// BlockedTools returns the list of blocked tools (DENIED or PENDING with default deny)
func (r *ToolPolicyResult) BlockedTools() []ToolCheckResult {
	blocked := make([]ToolCheckResult, 0)
	for _, t := range r.Tools {
		if t.Status == "BLOCKED" {
			blocked = append(blocked, t)
		}
	}
	return blocked
}

// RemovedTools returns the list of tools that will be stripped from the request
func (r *ToolPolicyResult) RemovedTools() []ToolCheckResult {
	removed := make([]ToolCheckResult, 0)
	for _, t := range r.Tools {
		if t.Status == "REMOVED" {
			removed = append(removed, t)
		}
	}
	return removed
}

// AllowedTools returns the list of explicitly allowed tools
func (r *ToolPolicyResult) AllowedTools() []ToolCheckResult {
	allowed := make([]ToolCheckResult, 0)
	for _, t := range r.Tools {
		if t.Status == "ALLOWED" {
			allowed = append(allowed, t)
		}
	}
	return allowed
}

// ============================================================================
// Deprecated: Legacy compatibility - will be removed
// ============================================================================

// ToolStore is deprecated - use RoleToolStore instead
type ToolStore interface {
	RoleToolStore
	// Legacy methods for backward compatibility
	CreateDiscoveredTool(ctx context.Context, tool *domain.RoleTool) error
	GetDiscoveredTool(ctx context.Context, id string) (*domain.RoleTool, error)
	GetToolByIdentity(ctx context.Context, name, description, schemaHash string) (*domain.RoleTool, error)
	UpdateToolSeen(ctx context.Context, id string) error
	GetToolPermission(ctx context.Context, toolID, roleID string) (*domain.ToolRolePermission, error)
}

// DiscoverToolsFromRequest is deprecated - use DiscoverToolsForRole instead
// This is kept for backward compatibility but now requires roleID through the apiKeyID lookup
func (s *ToolDiscoveryService) DiscoverToolsFromRequest(
	ctx context.Context,
	tenantID string,
	apiKeyID string,
	tools []domain.Tool,
	store ToolStore,
) ([]*domain.RoleTool, error) {
	// This function is deprecated - callers should use DiscoverToolsForRole
	// For now, return empty to avoid breaking changes
	slog.Warn("DiscoverToolsFromRequest is deprecated, use DiscoverToolsForRole with roleID")
	return nil, nil
}
