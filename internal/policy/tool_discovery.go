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
	toolCache map[string]string // tenantID:name:description:hash -> toolID
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

// DiscoverToolsFromRequest processes tools from a request and stores new ones
func (s *ToolDiscoveryService) DiscoverToolsFromRequest(
	ctx context.Context,
	tenantID string,
	apiKeyID string,
	tools []domain.Tool,
	store ToolStore,
) ([]*domain.DiscoveredTool, error) {
	discovered := make([]*domain.DiscoveredTool, 0, len(tools))

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

		// Check cache first
		cacheKey := fmt.Sprintf("%s:%s:%s:%s", tenantID, name, description, schemaHash)
		s.mu.RLock()
		toolID, cached := s.toolCache[cacheKey]
		s.mu.RUnlock()

		if cached {
			// Tool exists in cache, verify it still exists in database
			existing, err := store.GetDiscoveredTool(ctx, toolID)
			if err == nil && existing != nil {
				// Tool still exists, update last seen
				if err := store.UpdateToolSeen(ctx, toolID); err != nil {
					slog.Warn("Failed to update tool seen", "tool_id", toolID, "error", err)
				}
				discovered = append(discovered, existing)
				continue
			}
			// Tool was deleted from database - invalidate cache and continue to recreate
			slog.Info("Tool in cache but not in database, invalidating cache", "tool_id", toolID, "name", name)
			s.mu.Lock()
			delete(s.toolCache, cacheKey)
			s.mu.Unlock()
			// Fall through to check database / create new tool
		}

		// Check database
		existing, err := store.GetToolByIdentity(ctx, name, description, schemaHash)
		if err != nil {
			slog.Warn("Failed to check tool existence", "name", name, "error", err)
			continue
		}

		if existing != nil {
			// Tool exists, update cache and last seen
			slog.Info("Found existing tool by identity", "name", name, "tool_id", existing.ID, "schema_hash", schemaHash)
			s.mu.Lock()
			s.toolCache[cacheKey] = existing.ID
			s.mu.Unlock()

			if err := store.UpdateToolSeen(ctx, existing.ID); err != nil {
				slog.Warn("Failed to update tool seen", "tool_id", existing.ID, "error", err)
			}

			discovered = append(discovered, existing)
			continue
		}

		// New tool - create it
		newTool := &domain.DiscoveredTool{
			ID:          uuid.New().String(),
			Name:        name,
			Description: description,
			SchemaHash:  schemaHash,
			Parameters:  parameters,
			FirstSeenBy: apiKeyID,
			SeenCount:   1,
			Category:    inferCategory(name, description),
		}

		if err := store.CreateDiscoveredTool(ctx, newTool); err != nil {
			// Might be a race condition - try to fetch again
			if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
				existing, _ := store.GetToolByIdentity(ctx, name, description, schemaHash)
				if existing != nil {
					discovered = append(discovered, existing)
					s.mu.Lock()
					s.toolCache[cacheKey] = existing.ID
					s.mu.Unlock()
					continue
				}
			}
			slog.Warn("Failed to create tool", "name", name, "error", err)
			continue
		}

		slog.Info("Discovered new tool",
			"tool_id", newTool.ID,
			"name", name,
			"description", description,
			"tenant_id", tenantID)

		// Update cache
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
// Returns nil if all tools are allowed, otherwise returns an error with details
func (s *ToolDiscoveryService) CheckToolPermissions(
	ctx context.Context,
	roleID string,
	tools []*domain.DiscoveredTool,
	policy domain.EnhancedToolPolicies,
	store ToolStore,
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

		// Get permission for this tool-role combination
		perm, err := store.GetToolPermission(ctx, tool.ID, roleID)
		if err != nil {
			checkResult.Status = "ERROR"
			checkResult.Reason = fmt.Sprintf("failed to check permission: %v", err)
			result.Tools = append(result.Tools, checkResult)
			result.Allowed = false
			continue
		}

		if perm == nil {
			// No explicit permission - use default action
			slog.Info("No permission found for tool", "tool_id", tool.ID, "tool_name", tool.Name, "role_id", roleID, "default_action", policy.DefaultAction)
			if policy.DefaultAction == "ALLOW" {
				checkResult.Status = "ALLOWED"
				checkResult.Reason = "default policy: allow unknown tools"
			} else {
				checkResult.Status = "BLOCKED"
				checkResult.Reason = "pending review (default deny)"
				result.Allowed = false
			}
		} else {
			slog.Info("Permission found for tool", "tool_id", tool.ID, "tool_name", tool.Name, "role_id", roleID, "perm_status", perm.Status, "perm_tool_id", perm.ToolID)
			switch perm.Status {
			case domain.ToolStatusAllowed:
				checkResult.Status = "ALLOWED"
				checkResult.Reason = "explicitly allowed"

			case domain.ToolStatusDenied:
				checkResult.Status = "BLOCKED"
				if perm.DecisionReason != "" {
					checkResult.Reason = perm.DecisionReason
				} else {
					checkResult.Reason = "explicitly denied"
				}
				result.Allowed = false

			case domain.ToolStatusRemoved:
				checkResult.Status = "REMOVED"
				if perm.DecisionReason != "" {
					checkResult.Reason = perm.DecisionReason
				} else {
					checkResult.Reason = "tool removed from request"
				}
				// Note: REMOVED does NOT set result.Allowed = false
				// The request continues without this tool

			case domain.ToolStatusPending:
				if policy.DefaultAction == "ALLOW" {
					checkResult.Status = "ALLOWED"
					checkResult.Reason = "pending review (default allow)"
				} else {
					checkResult.Status = "BLOCKED"
					checkResult.Reason = "pending review (default deny)"
					result.Allowed = false
				}

			default:
				// Unknown status - log and treat as pending
				slog.Warn("Unknown tool permission status", "tool_id", tool.ID, "tool_name", tool.Name, "status", perm.Status)
				if policy.DefaultAction == "ALLOW" {
					checkResult.Status = "ALLOWED"
					checkResult.Reason = "unknown status, default allow"
				} else {
					checkResult.Status = "BLOCKED"
					checkResult.Reason = "unknown status, default deny"
					result.Allowed = false
				}
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

// ToolStore interface for tool repository operations
type ToolStore interface {
	CreateDiscoveredTool(ctx context.Context, tool *domain.DiscoveredTool) error
	GetDiscoveredTool(ctx context.Context, id string) (*domain.DiscoveredTool, error)
	GetToolByIdentity(ctx context.Context, name, description, schemaHash string) (*domain.DiscoveredTool, error)
	UpdateToolSeen(ctx context.Context, id string) error
	GetToolPermission(ctx context.Context, toolID, roleID string) (*domain.ToolRolePermission, error)
	LogToolExecution(ctx context.Context, log *domain.ToolExecutionLog) error
}
