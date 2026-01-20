package resolver

import (
	"context"
	"errors"

	"modelgate/internal/domain"
	"modelgate/internal/graphql/model"
)

var errToolTenantRequired = errors.New("tenant context required")

// getActorInfoFromContext extracts user information from the context
// Returns actorID, actorEmail, actorType
func getActorInfoFromContext(ctx context.Context) (string, string, string) {
	// Try to get user from context (set by auth middleware)
	// Must use ContextKeyUser constant, not a raw string, because Go compares context keys by identity
	if user, ok := ctx.Value(ContextKeyUser).(*domain.User); ok && user != nil {
		return user.ID, user.Email, "user"
	}

	// Fallback: check if we have user ID and email set separately (from tenant user auth)
	userID := GetUserFromContext(ctx)
	userEmail := GetUserEmailFromContext(ctx)
	if userID != "" {
		actorType := "user"
		if IsAdminFromContext(ctx) {
			actorType = "admin"
		}
		return userID, userEmail, actorType
	}

	// Return empty if no user found
	return "", "", ""
}

// ============================================================================
// Query Resolvers
// ============================================================================

// DiscoveredToolsImpl implements the discoveredTools query
// Note: Now requires roleId in filter to get role-specific tools
func (r *queryResolver) DiscoveredToolsImpl(ctx context.Context, filter *model.DiscoveredToolFilter, limit *int, offset *int) (*model.DiscoveredToolConnection, error) {
	tenantSlug := GetTenantFromContext(ctx)
	if tenantSlug == "" {
		return nil, errToolTenantRequired
	}

	store, err := r.PGStore.GetTenantStore(tenantSlug)
	if err != nil {
		return nil, err
	}

	// RoleID is now required for listing tools
	if filter == nil || filter.RoleID == nil || *filter.RoleID == "" {
		return &model.DiscoveredToolConnection{
			Items:      []model.DiscoveredTool{},
			TotalCount: 0,
			HasMore:    false,
		}, nil
	}

	// Build filter
	domainFilter := domain.ToolFilter{}
	if filter.Name != nil {
		domainFilter.Name = *filter.Name
	}
	if filter.Category != nil {
		domainFilter.Category = *filter.Category
	}
	if filter.Status != nil {
		domainFilter.Status = domain.ToolPermissionStatus(*filter.Status)
	}

	limitVal := 50
	if limit != nil {
		limitVal = *limit
	}
	offsetVal := 0
	if offset != nil {
		offsetVal = *offset
	}

	tools, total, err := store.ListRoleTools(ctx, *filter.RoleID, domainFilter, limitVal, offsetVal)
	if err != nil {
		return nil, err
	}

	items := make([]model.DiscoveredTool, 0, len(tools))
	for _, t := range tools {
		items = append(items, convertRoleToolToModel(t))
	}

	return &model.DiscoveredToolConnection{
		Items:      items,
		TotalCount: total,
		HasMore:    offsetVal+len(items) < total,
	}, nil
}

// DiscoveredToolImpl implements the discoveredTool query
func (r *queryResolver) DiscoveredToolImpl(ctx context.Context, id string) (*model.DiscoveredTool, error) {
	tenantSlug := GetTenantFromContext(ctx)
	if tenantSlug == "" {
		return nil, errToolTenantRequired
	}

	store, err := r.PGStore.GetTenantStore(tenantSlug)
	if err != nil {
		return nil, err
	}

	tool, err := store.GetRoleTool(ctx, id)
	if err != nil {
		return nil, err
	}
	if tool == nil {
		return nil, nil
	}

	result := convertRoleToolToModel(tool)
	return &result, nil
}

// RoleToolPermissionsImpl implements the roleToolPermissions query
// Now simplified - tools and permissions are in the same table
func (r *queryResolver) RoleToolPermissionsImpl(ctx context.Context, roleID string) ([]model.ToolWithPermission, error) {
	tenantSlug := GetTenantFromContext(ctx)
	if tenantSlug == "" {
		return nil, errToolTenantRequired
	}

	store, err := r.PGStore.GetTenantStore(tenantSlug)
	if err != nil {
		return nil, err
	}

	// Get all tools for this role (they now include permissions inline)
	tools, _, err := store.ListRoleTools(ctx, roleID, domain.ToolFilter{}, 1000, 0)
	if err != nil {
		return nil, err
	}

	// Build result - permissions are now inline in the tool
	results := make([]model.ToolWithPermission, 0, len(tools))
	for _, tool := range tools {
		toolModel := convertRoleToolToModel(tool)

		var decidedBy, decidedByEmail, decisionReason *string
		if tool.DecidedBy != "" {
			decidedBy = &tool.DecidedBy
		}
		if tool.DecidedByEmail != "" {
			decidedByEmail = &tool.DecidedByEmail
		}
		if tool.DecisionReason != "" {
			decisionReason = &tool.DecisionReason
		}

		results = append(results, model.ToolWithPermission{
			Tool:           &toolModel,
			Status:         model.ToolPermissionStatus(tool.Status),
			DecidedBy:      decidedBy,
			DecidedByEmail: decidedByEmail,
			DecidedAt:      tool.DecidedAt,
			DecisionReason: decisionReason,
		})
	}

	return results, nil
}

// PendingToolsImpl implements the pendingTools query
func (r *queryResolver) PendingToolsImpl(ctx context.Context) ([]model.DiscoveredTool, error) {
	tenantSlug := GetTenantFromContext(ctx)
	if tenantSlug == "" {
		return nil, errToolTenantRequired
	}

	store, err := r.PGStore.GetTenantStore(tenantSlug)
	if err != nil {
		return nil, err
	}

	tools, err := store.ListPendingTools(ctx)
	if err != nil {
		return nil, err
	}

	results := make([]model.DiscoveredTool, 0, len(tools))
	for _, t := range tools {
		results = append(results, convertRoleToolToModel(t))
	}

	return results, nil
}

// ToolExecutionLogsImpl implements the toolExecutionLogs query
func (r *queryResolver) ToolExecutionLogsImpl(ctx context.Context, filter *model.ToolExecutionLogFilter, limit *int, offset *int) (*model.ToolExecutionLogConnection, error) {
	tenantSlug := GetTenantFromContext(ctx)
	if tenantSlug == "" {
		return nil, errToolTenantRequired
	}

	store, err := r.PGStore.GetTenantStore(tenantSlug)
	if err != nil {
		return nil, err
	}

	// Build filter
	domainFilter := domain.ToolLogFilter{}
	if filter != nil {
		if filter.ToolID != nil {
			domainFilter.ToolID = *filter.ToolID
		}
		if filter.RoleID != nil {
			domainFilter.RoleID = *filter.RoleID
		}
		if filter.Status != nil {
			domainFilter.Status = *filter.Status
		}
		if filter.StartDate != nil {
			domainFilter.StartTime = *filter.StartDate
		}
		if filter.EndDate != nil {
			domainFilter.EndTime = *filter.EndDate
		}
	}

	limitVal := 50
	if limit != nil {
		limitVal = *limit
	}
	offsetVal := 0
	if offset != nil {
		offsetVal = *offset
	}

	logs, total, err := store.ListToolExecutionLogs(ctx, domainFilter, limitVal, offsetVal)
	if err != nil {
		return nil, err
	}

	items := make([]model.ToolExecutionLog, 0, len(logs))
	for _, l := range logs {
		items = append(items, convertDomainLogToModel(l))
	}

	return &model.ToolExecutionLogConnection{
		Items:      items,
		TotalCount: total,
		HasMore:    offsetVal+len(items) < total,
	}, nil
}

// ============================================================================
// Mutation Resolvers
// ============================================================================

// SetToolPermissionImpl implements the setToolPermission mutation
// Now updates the role_tools table directly (permission is inline)
func (r *mutationResolver) SetToolPermissionImpl(ctx context.Context, input model.SetToolPermissionInput) (*model.ToolRolePermission, error) {
	tenantSlug := GetTenantFromContext(ctx)
	if tenantSlug == "" {
		return nil, errToolTenantRequired
	}

	store, err := r.PGStore.GetTenantStore(tenantSlug)
	if err != nil {
		return nil, err
	}

	// Get actor info from context
	actorID, actorEmail, _ := getActorInfoFromContext(ctx)

	reason := ""
	if input.Reason != nil {
		reason = *input.Reason
	}

	// Update the tool's permission directly (it's now inline in role_tools)
	if err := store.SetRoleToolPermission(ctx, input.ToolID, domain.ToolPermissionStatus(input.Status), actorID, actorEmail, reason); err != nil {
		return nil, err
	}

	// Fetch the updated tool for the response
	tool, _ := store.GetRoleTool(ctx, input.ToolID)
	role, _ := store.GetRole(ctx, input.RoleID)

	return convertRoleToolToPermModel(tool, role), nil
}

// SetToolPermissionsBulkImpl implements the setToolPermissionsBulk mutation
func (r *mutationResolver) SetToolPermissionsBulkImpl(ctx context.Context, input model.SetToolPermissionsBulkInput) ([]model.ToolRolePermission, error) {
	tenantSlug := GetTenantFromContext(ctx)
	if tenantSlug == "" {
		return nil, errToolTenantRequired
	}

	store, err := r.PGStore.GetTenantStore(tenantSlug)
	if err != nil {
		return nil, err
	}

	// Get actor info
	actorID, actorEmail, _ := getActorInfoFromContext(ctx)

	results := make([]model.ToolRolePermission, 0, len(input.Permissions))

	for _, p := range input.Permissions {
		reason := ""
		if p.Reason != nil {
			reason = *p.Reason
		}

		if err := store.SetRoleToolPermission(ctx, p.ToolID, domain.ToolPermissionStatus(p.Status), actorID, actorEmail, reason); err != nil {
			continue
		}

		tool, _ := store.GetRoleTool(ctx, p.ToolID)
		role, _ := store.GetRole(ctx, input.RoleID)
		if tool != nil {
			results = append(results, *convertRoleToolToPermModel(tool, role))
		}
	}

	return results, nil
}

// ApproveAllPendingToolsImpl implements the approveAllPendingTools mutation
func (r *mutationResolver) ApproveAllPendingToolsImpl(ctx context.Context, roleID string) (int, error) {
	tenantSlug := GetTenantFromContext(ctx)
	if tenantSlug == "" {
		return 0, errToolTenantRequired
	}

	store, err := r.PGStore.GetTenantStore(tenantSlug)
	if err != nil {
		return 0, err
	}

	actorID, actorEmail, _ := getActorInfoFromContext(ctx)

	count, err := store.BulkSetRoleToolPermissions(ctx, roleID, domain.ToolStatusAllowed, actorID, actorEmail)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// DenyAllPendingToolsImpl implements the denyAllPendingTools mutation
func (r *mutationResolver) DenyAllPendingToolsImpl(ctx context.Context, roleID string) (int, error) {
	tenantSlug := GetTenantFromContext(ctx)
	if tenantSlug == "" {
		return 0, errToolTenantRequired
	}

	store, err := r.PGStore.GetTenantStore(tenantSlug)
	if err != nil {
		return 0, err
	}

	actorID, actorEmail, _ := getActorInfoFromContext(ctx)

	count, err := store.BulkSetRoleToolPermissions(ctx, roleID, domain.ToolStatusDenied, actorID, actorEmail)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// RemoveAllPendingToolsImpl implements the removeAllPendingTools mutation
func (r *mutationResolver) RemoveAllPendingToolsImpl(ctx context.Context, roleID string) (int, error) {
	tenantSlug := GetTenantFromContext(ctx)
	if tenantSlug == "" {
		return 0, errToolTenantRequired
	}

	store, err := r.PGStore.GetTenantStore(tenantSlug)
	if err != nil {
		return 0, err
	}

	actorID, actorEmail, _ := getActorInfoFromContext(ctx)

	count, err := store.BulkSetRoleToolPermissions(ctx, roleID, domain.ToolStatusRemoved, actorID, actorEmail)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// DeleteDiscoveredToolImpl implements the deleteDiscoveredTool mutation
func (r *mutationResolver) DeleteDiscoveredToolImpl(ctx context.Context, id string) (bool, error) {
	tenantSlug := GetTenantFromContext(ctx)
	if tenantSlug == "" {
		return false, errToolTenantRequired
	}

	store, err := r.PGStore.GetTenantStore(tenantSlug)
	if err != nil {
		return false, err
	}

	if err := store.DeleteRoleTool(ctx, id); err != nil {
		return false, err
	}

	return true, nil
}

// ============================================================================
// Helper Functions
// ============================================================================

// convertRoleToolToModel converts a domain.RoleTool to a GraphQL model.DiscoveredTool
func convertRoleToolToModel(t *domain.RoleTool) model.DiscoveredTool {
	var firstSeenBy, category *string
	if t.FirstSeenBy != "" {
		firstSeenBy = &t.FirstSeenBy
	}
	if t.Category != "" {
		category = &t.Category
	}

	// Convert Parameters to map[string]any
	var params map[string]any
	if t.Parameters != nil {
		if m, ok := t.Parameters.(map[string]any); ok {
			params = m
		} else if m, ok := t.Parameters.(map[string]interface{}); ok {
			params = m
		}
	}
	if params == nil {
		params = make(map[string]any)
	}

	return model.DiscoveredTool{
		ID:          t.ID,
		Name:        t.Name,
		Description: t.Description,
		SchemaHash:  t.SchemaHash,
		Parameters:  params,
		Category:    category,
		FirstSeenAt: t.FirstSeenAt,
		LastSeenAt:  t.LastSeenAt,
		FirstSeenBy: firstSeenBy,
		SeenCount:   t.SeenCount,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}

// convertDomainToolToModel is an alias for backward compatibility
func convertDomainToolToModel(t *domain.RoleTool) model.DiscoveredTool {
	return convertRoleToolToModel(t)
}

func convertDomainLogToModel(l *domain.ToolExecutionLog) model.ToolExecutionLog {
	var apiKeyID, requestID, blockReason, logModel *string
	if l.APIKeyID != "" {
		apiKeyID = &l.APIKeyID
	}
	if l.RequestID != "" {
		requestID = &l.RequestID
	}
	if l.BlockReason != "" {
		blockReason = &l.BlockReason
	}
	if l.Model != "" {
		logModel = &l.Model
	}

	return model.ToolExecutionLog{
		ID:          l.ID,
		ToolID:      l.RoleToolID, // Map RoleToolID to ToolID for GraphQL
		ToolName:    l.ToolName,
		RoleID:      l.RoleID,
		APIKeyID:    apiKeyID,
		RequestID:   requestID,
		Status:      l.Status,
		BlockReason: blockReason,
		ExecutedAt:  l.ExecutedAt,
		Model:       logModel,
	}
}

// convertRoleToolToPermModel converts a RoleTool to ToolRolePermission model
func convertRoleToolToPermModel(tool *domain.RoleTool, role *domain.Role) *model.ToolRolePermission {
	if tool == nil {
		return nil
	}

	var decidedBy, decidedByEmail, decisionReason *string
	if tool.DecidedBy != "" {
		decidedBy = &tool.DecidedBy
	}
	if tool.DecidedByEmail != "" {
		decidedByEmail = &tool.DecidedByEmail
	}
	if tool.DecisionReason != "" {
		decisionReason = &tool.DecisionReason
	}

	toolModel := convertRoleToolToModel(tool)

	result := &model.ToolRolePermission{
		ID:             tool.ID,
		Status:         model.ToolPermissionStatus(tool.Status),
		DecidedBy:      decidedBy,
		DecidedByEmail: decidedByEmail,
		DecidedAt:      tool.DecidedAt,
		DecisionReason: decisionReason,
		CreatedAt:      tool.CreatedAt,
		UpdatedAt:      tool.UpdatedAt,
		Tool:           &toolModel,
	}

	if role != nil {
		result.Role = &model.Role{
			ID:   role.ID,
			Name: role.Name,
		}
	}

	return result
}

// convertDomainPermToModel is kept for backward compatibility
func convertDomainPermToModel(p *domain.ToolRolePermission, tool *domain.RoleTool, role *domain.Role) *model.ToolRolePermission {
	if tool != nil {
		return convertRoleToolToPermModel(tool, role)
	}

	var decidedBy, decidedByEmail, decisionReason *string
	if p.DecidedBy != "" {
		decidedBy = &p.DecidedBy
	}
	if p.DecidedByEmail != "" {
		decidedByEmail = &p.DecidedByEmail
	}
	if p.DecisionReason != "" {
		decisionReason = &p.DecisionReason
	}

	result := &model.ToolRolePermission{
		ID:             p.ID,
		Status:         model.ToolPermissionStatus(p.Status),
		DecidedBy:      decidedBy,
		DecidedByEmail: decidedByEmail,
		DecidedAt:      p.DecidedAt,
		DecisionReason: decisionReason,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}

	if role != nil {
		result.Role = &model.Role{
			ID:   role.ID,
			Name: role.Name,
		}
	}

	return result
}
