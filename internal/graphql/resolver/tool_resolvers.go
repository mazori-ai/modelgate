package resolver

import (
	"context"
	"errors"
	"time"

	"modelgate/internal/domain"
	"modelgate/internal/graphql/model"

	"github.com/google/uuid"
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
func (r *queryResolver) DiscoveredToolsImpl(ctx context.Context, filter *model.DiscoveredToolFilter, limit *int, offset *int) (*model.DiscoveredToolConnection, error) {
	tenantSlug := GetTenantFromContext(ctx)
	if tenantSlug == "" {
		return nil, errToolTenantRequired
	}

	store, err := r.PGStore.GetTenantStore(tenantSlug)
	if err != nil {
		return nil, err
	}

	// Build filter
	domainFilter := domain.ToolFilter{}
	if filter != nil {
		if filter.Name != nil {
			domainFilter.Name = *filter.Name
		}
		if filter.Category != nil {
			domainFilter.Category = *filter.Category
		}
		if filter.RoleID != nil {
			domainFilter.RoleID = *filter.RoleID
		}
		if filter.Status != nil {
			domainFilter.Status = domain.ToolPermissionStatus(*filter.Status)
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

	tools, total, err := store.ListDiscoveredTools(ctx, domainFilter, limitVal, offsetVal)
	if err != nil {
		return nil, err
	}

	items := make([]model.DiscoveredTool, 0, len(tools))
	for _, t := range tools {
		items = append(items, convertDomainToolToModel(t))
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

	tool, err := store.GetDiscoveredTool(ctx, id)
	if err != nil {
		return nil, err
	}
	if tool == nil {
		return nil, nil
	}

	result := convertDomainToolToModel(tool)
	return &result, nil
}

// RoleToolPermissionsImpl implements the roleToolPermissions query
func (r *queryResolver) RoleToolPermissionsImpl(ctx context.Context, roleID string) ([]model.ToolWithPermission, error) {
	tenantSlug := GetTenantFromContext(ctx)
	if tenantSlug == "" {
		return nil, errToolTenantRequired
	}

	store, err := r.PGStore.GetTenantStore(tenantSlug)
	if err != nil {
		return nil, err
	}

	// Get all tools
	tools, _, err := store.ListDiscoveredTools(ctx, domain.ToolFilter{}, 1000, 0)
	if err != nil {
		return nil, err
	}

	// Get permissions for this role
	permissions, err := store.ListToolPermissions(ctx, roleID)
	if err != nil {
		return nil, err
	}

	// Create a map for quick lookup
	permMap := make(map[string]*domain.ToolRolePermission)
	for _, p := range permissions {
		permMap[p.ToolID] = p
	}

	// Build result
	results := make([]model.ToolWithPermission, 0, len(tools))
	for _, tool := range tools {
		toolModel := convertDomainToolToModel(tool)
		status := model.ToolPermissionStatusPending

		var decidedBy, decidedByEmail, decisionReason *string
		var decidedAt *time.Time

		if perm, ok := permMap[tool.ID]; ok {
			status = model.ToolPermissionStatus(perm.Status)
			if perm.DecidedBy != "" {
				decidedBy = &perm.DecidedBy
			}
			if perm.DecidedByEmail != "" {
				decidedByEmail = &perm.DecidedByEmail
			}
			if perm.DecisionReason != "" {
				decisionReason = &perm.DecisionReason
			}
			decidedAt = perm.DecidedAt
		}

		results = append(results, model.ToolWithPermission{
			Tool:           &toolModel,
			Status:         status,
			DecidedBy:      decidedBy,
			DecidedByEmail: decidedByEmail,
			DecidedAt:      decidedAt,
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
		results = append(results, convertDomainToolToModel(t))
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

	now := time.Now()
	perm := &domain.ToolRolePermission{
		ID:             uuid.New().String(),
		ToolID:         input.ToolID,
		RoleID:         input.RoleID,
		Status:         domain.ToolPermissionStatus(input.Status),
		DecidedBy:      actorID,
		DecidedByEmail: actorEmail,
		DecidedAt:      &now,
	}

	if input.Reason != nil {
		perm.DecisionReason = *input.Reason
	}

	// Create or update permission
	if err := store.CreateToolPermission(ctx, perm); err != nil {
		return nil, err
	}

	// Fetch the tool and role for the response
	tool, _ := store.GetDiscoveredTool(ctx, input.ToolID)
	role, _ := store.GetRole(ctx, input.RoleID)

	return convertDomainPermToModel(perm, tool, role), nil
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
	now := time.Now()

	for _, p := range input.Permissions {
		perm := &domain.ToolRolePermission{
			ID:             uuid.New().String(),
			ToolID:         p.ToolID,
			RoleID:         input.RoleID,
			Status:         domain.ToolPermissionStatus(p.Status),
			DecidedBy:      actorID,
			DecidedByEmail: actorEmail,
			DecidedAt:      &now,
		}

		if p.Reason != nil {
			perm.DecisionReason = *p.Reason
		}

		if err := store.CreateToolPermission(ctx, perm); err != nil {
			continue
		}

		tool, _ := store.GetDiscoveredTool(ctx, p.ToolID)
		role, _ := store.GetRole(ctx, input.RoleID)
		results = append(results, *convertDomainPermToModel(perm, tool, role))
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

	count, err := store.BulkUpdatePermissions(ctx, roleID, domain.ToolStatusAllowed, actorID, actorEmail)
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

	count, err := store.BulkUpdatePermissions(ctx, roleID, domain.ToolStatusDenied, actorID, actorEmail)
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

	count, err := store.BulkUpdatePermissions(ctx, roleID, domain.ToolStatusRemoved, actorID, actorEmail)
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

	if err := store.DeleteDiscoveredTool(ctx, id); err != nil {
		return false, err
	}

	return true, nil
}

// ============================================================================
// Helper Functions
// ============================================================================

func convertDomainToolToModel(t *domain.DiscoveredTool) model.DiscoveredTool {
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
		ToolID:      l.ToolID,
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

func convertDomainPermToModel(p *domain.ToolRolePermission, tool *domain.DiscoveredTool, role *domain.Role) *model.ToolRolePermission {
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

	if tool != nil {
		t := convertDomainToolToModel(tool)
		result.Tool = &t
	}

	if role != nil {
		// Convert role to model - minimal fields for now
		result.Role = &model.Role{
			ID:   role.ID,
			Name: role.Name,
		}
	}

	return result
}
