package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"modelgate/internal/domain"
)

// ============================================================================
// Role Tools (unified tool discovery + permissions)
// ============================================================================

// CreateOrUpdateRoleTool creates or updates a tool for a specific role
// Uses upsert on (role_id, name, schema_hash) to handle duplicates
func (s *TenantStore) CreateOrUpdateRoleTool(ctx context.Context, tool *domain.RoleTool) error {
	parametersJSON, err := json.Marshal(tool.Parameters)
	if err != nil {
		return err
	}

	// Use ON CONFLICT to handle duplicates - update if exists
	query := `
		INSERT INTO role_tools (
			id, role_id, name, description, schema_hash, parameters, category,
			first_seen_at, last_seen_at, first_seen_by, seen_count,
			status, decided_by, decided_by_email, decided_at, decision_reason,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18
		)
		ON CONFLICT (role_id, name, schema_hash) DO UPDATE SET
			description = EXCLUDED.description,
			parameters = EXCLUDED.parameters,
			category = COALESCE(EXCLUDED.category, role_tools.category),
			last_seen_at = EXCLUDED.last_seen_at,
			seen_count = role_tools.seen_count + 1,
			updated_at = EXCLUDED.updated_at
		RETURNING id
	`

	now := time.Now()
	if tool.FirstSeenAt.IsZero() {
		tool.FirstSeenAt = now
	}
	if tool.LastSeenAt.IsZero() {
		tool.LastSeenAt = now
	}
	if tool.SeenCount == 0 {
		tool.SeenCount = 1
	}
	if tool.Status == "" {
		tool.Status = domain.ToolStatusPending
	}

	var actualID string
	err = s.db.QueryRowContext(ctx, query,
		tool.ID, tool.RoleID, tool.Name, tool.Description, tool.SchemaHash, parametersJSON, nullString(tool.Category),
		tool.FirstSeenAt, tool.LastSeenAt, nullString(tool.FirstSeenBy), tool.SeenCount,
		tool.Status, nullString(tool.DecidedBy), nullString(tool.DecidedByEmail), tool.DecidedAt, nullString(tool.DecisionReason),
		now, now,
	).Scan(&actualID)

	if err != nil {
		return err
	}

	// Update the tool ID with actual ID (in case of conflict, existing ID is returned)
	tool.ID = actualID
	return nil
}

// GetRoleTool gets a role tool by ID
func (s *TenantStore) GetRoleTool(ctx context.Context, id string) (*domain.RoleTool, error) {
	query := `
		SELECT id, role_id, name, description, schema_hash, parameters, category,
			first_seen_at, last_seen_at, first_seen_by, seen_count,
			status, decided_by, decided_by_email, decided_at, decision_reason,
			created_at, updated_at
		FROM role_tools
		WHERE id = $1
	`
	return s.scanRoleTool(s.db.QueryRowContext(ctx, query, id))
}

// GetRoleToolByIdentity gets a tool by its unique identity within a role
func (s *TenantStore) GetRoleToolByIdentity(ctx context.Context, roleID, name, schemaHash string) (*domain.RoleTool, error) {
	query := `
		SELECT id, role_id, name, description, schema_hash, parameters, category,
			first_seen_at, last_seen_at, first_seen_by, seen_count,
			status, decided_by, decided_by_email, decided_at, decision_reason,
			created_at, updated_at
		FROM role_tools
		WHERE role_id = $1 AND name = $2 AND schema_hash = $3
	`
	return s.scanRoleTool(s.db.QueryRowContext(ctx, query, roleID, name, schemaHash))
}

// GetRoleToolByName gets a tool by name within a role (ignoring schema hash)
// Returns the most recently seen variant if multiple exist
func (s *TenantStore) GetRoleToolByName(ctx context.Context, roleID, name string) (*domain.RoleTool, error) {
	query := `
		SELECT id, role_id, name, description, schema_hash, parameters, category,
			first_seen_at, last_seen_at, first_seen_by, seen_count,
			status, decided_by, decided_by_email, decided_at, decision_reason,
			created_at, updated_at
		FROM role_tools
		WHERE role_id = $1 AND name = $2
		ORDER BY last_seen_at DESC
		LIMIT 1
	`
	return s.scanRoleTool(s.db.QueryRowContext(ctx, query, roleID, name))
}

// scanRoleTool scans a single row into a RoleTool
func (s *TenantStore) scanRoleTool(row *sql.Row) (*domain.RoleTool, error) {
	var tool domain.RoleTool
	var parametersJSON []byte
	var category, firstSeenBy, decidedBy, decidedByEmail, decisionReason sql.NullString
	var decidedAt sql.NullTime

	err := row.Scan(
		&tool.ID, &tool.RoleID, &tool.Name, &tool.Description, &tool.SchemaHash, &parametersJSON, &category,
		&tool.FirstSeenAt, &tool.LastSeenAt, &firstSeenBy, &tool.SeenCount,
		&tool.Status, &decidedBy, &decidedByEmail, &decidedAt, &decisionReason,
		&tool.CreatedAt, &tool.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal(parametersJSON, &tool.Parameters)
	if category.Valid {
		tool.Category = category.String
	}
	if firstSeenBy.Valid {
		tool.FirstSeenBy = firstSeenBy.String
	}
	if decidedBy.Valid {
		tool.DecidedBy = decidedBy.String
	}
	if decidedByEmail.Valid {
		tool.DecidedByEmail = decidedByEmail.String
	}
	if decidedAt.Valid {
		tool.DecidedAt = &decidedAt.Time
	}
	if decisionReason.Valid {
		tool.DecisionReason = decisionReason.String
	}

	return &tool, nil
}

// UpdateRoleToolSeen updates the last_seen_at and increments seen_count
func (s *TenantStore) UpdateRoleToolSeen(ctx context.Context, id string) error {
	query := `
		UPDATE role_tools
		SET last_seen_at = $2, seen_count = seen_count + 1, updated_at = $2
		WHERE id = $1
	`
	_, err := s.db.ExecContext(ctx, query, id, time.Now())
	return err
}

// ListRoleTools lists tools for a specific role with filtering
func (s *TenantStore) ListRoleTools(ctx context.Context, roleID string, filter domain.ToolFilter, limit, offset int) ([]*domain.RoleTool, int, error) {
	whereClause := "WHERE role_id = $1"
	args := []interface{}{roleID}
	argIdx := 2

	if filter.Name != "" {
		whereClause += fmt.Sprintf(" AND name ILIKE $%d", argIdx)
		args = append(args, "%"+filter.Name+"%")
		argIdx++
	}

	if filter.Category != "" {
		whereClause += fmt.Sprintf(" AND category = $%d", argIdx)
		args = append(args, filter.Category)
		argIdx++
	}

	if filter.Status != "" {
		whereClause += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, string(filter.Status))
		argIdx++
	}

	// Count total
	countQuery := "SELECT COUNT(*) FROM role_tools " + whereClause
	var total int
	err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get paginated results
	query := `
		SELECT id, role_id, name, description, schema_hash, parameters, category,
			first_seen_at, last_seen_at, first_seen_by, seen_count,
			status, decided_by, decided_by_email, decided_at, decision_reason,
			created_at, updated_at
		FROM role_tools
	` + whereClause + fmt.Sprintf(" ORDER BY last_seen_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var tools []*domain.RoleTool
	for rows.Next() {
		var tool domain.RoleTool
		var parametersJSON []byte
		var category, firstSeenBy, decidedBy, decidedByEmail, decisionReason sql.NullString
		var decidedAt sql.NullTime

		err := rows.Scan(
			&tool.ID, &tool.RoleID, &tool.Name, &tool.Description, &tool.SchemaHash, &parametersJSON, &category,
			&tool.FirstSeenAt, &tool.LastSeenAt, &firstSeenBy, &tool.SeenCount,
			&tool.Status, &decidedBy, &decidedByEmail, &decidedAt, &decisionReason,
			&tool.CreatedAt, &tool.UpdatedAt,
		)
		if err != nil {
			return nil, 0, err
		}

		json.Unmarshal(parametersJSON, &tool.Parameters)
		if category.Valid {
			tool.Category = category.String
		}
		if firstSeenBy.Valid {
			tool.FirstSeenBy = firstSeenBy.String
		}
		if decidedBy.Valid {
			tool.DecidedBy = decidedBy.String
		}
		if decidedByEmail.Valid {
			tool.DecidedByEmail = decidedByEmail.String
		}
		if decidedAt.Valid {
			tool.DecidedAt = &decidedAt.Time
		}
		if decisionReason.Valid {
			tool.DecisionReason = decisionReason.String
		}

		tools = append(tools, &tool)
	}

	return tools, total, nil
}

// DeleteRoleTool deletes a role tool
func (s *TenantStore) DeleteRoleTool(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM role_tools WHERE id = $1", id)
	return err
}

// ============================================================================
// Role Tool Permissions (inline updates)
// ============================================================================

// SetRoleToolPermission updates the permission status for a role tool
func (s *TenantStore) SetRoleToolPermission(ctx context.Context, id string, status domain.ToolPermissionStatus, decidedBy, decidedByEmail, reason string) error {
	now := time.Now()
	query := `
		UPDATE role_tools
		SET status = $2, decided_by = $3, decided_by_email = $4, decided_at = $5, decision_reason = $6, updated_at = $5
		WHERE id = $1
	`
	_, err := s.db.ExecContext(ctx, query, id, status, nullString(decidedBy), nullString(decidedByEmail), now, nullString(reason))
	return err
}

// BulkSetRoleToolPermissions sets permissions for all pending tools for a role
func (s *TenantStore) BulkSetRoleToolPermissions(ctx context.Context, roleID string, status domain.ToolPermissionStatus, decidedBy, decidedByEmail string) (int, error) {
	now := time.Now()
	query := `
		UPDATE role_tools
		SET status = $2, decided_by = $3, decided_by_email = $4, decided_at = $5, updated_at = $5
		WHERE role_id = $1 AND status = 'PENDING'
	`
	result, err := s.db.ExecContext(ctx, query, roleID, status, nullString(decidedBy), nullString(decidedByEmail), now)
	if err != nil {
		return 0, err
	}
	count, _ := result.RowsAffected()
	return int(count), nil
}

// ============================================================================
// Tool Execution Logs
// ============================================================================

// LogToolExecution logs a tool call attempt
func (s *TenantStore) LogToolExecution(ctx context.Context, log *domain.ToolExecutionLog) error {
	inputParamsJSON, _ := json.Marshal(log.ToolArguments)

	query := `
		INSERT INTO tool_execution_logs (
			id, role_tool_id, tool_name, role_id, api_key_id,
			request_id, input_params, status, error_message,
			duration_ms, token_count, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		)
	`

	now := time.Now()
	if log.ExecutedAt.IsZero() {
		log.ExecutedAt = now
	}

	_, err := s.db.ExecContext(ctx, query,
		log.ID, nullString(log.RoleToolID), log.ToolName, log.RoleID, nullString(log.APIKeyID),
		nullString(log.RequestID), inputParamsJSON, log.Status, nullString(log.BlockReason),
		0, 0, log.ExecutedAt,
	)

	return err
}

// ListToolExecutionLogs lists tool execution logs with filtering
func (s *TenantStore) ListToolExecutionLogs(ctx context.Context, filter domain.ToolLogFilter, limit, offset int) ([]*domain.ToolExecutionLog, int, error) {
	whereClause := "WHERE 1=1"
	args := []interface{}{}
	argIdx := 1

	if filter.ToolID != "" {
		whereClause += fmt.Sprintf(" AND role_tool_id = $%d", argIdx)
		args = append(args, filter.ToolID)
		argIdx++
	}

	if filter.RoleID != "" {
		whereClause += fmt.Sprintf(" AND role_id = $%d", argIdx)
		args = append(args, filter.RoleID)
		argIdx++
	}

	if filter.Status != "" {
		whereClause += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, filter.Status)
		argIdx++
	}

	if !filter.StartTime.IsZero() {
		whereClause += fmt.Sprintf(" AND created_at >= $%d", argIdx)
		args = append(args, filter.StartTime)
		argIdx++
	}

	if !filter.EndTime.IsZero() {
		whereClause += fmt.Sprintf(" AND created_at <= $%d", argIdx)
		args = append(args, filter.EndTime)
		argIdx++
	}

	// Count total
	countQuery := "SELECT COUNT(*) FROM tool_execution_logs " + whereClause
	var total int
	err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get paginated results
	query := `
		SELECT id, role_tool_id, tool_name, role_id, api_key_id,
			request_id, input_params, status, error_message, created_at
		FROM tool_execution_logs
	` + whereClause + fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []*domain.ToolExecutionLog
	for rows.Next() {
		var log domain.ToolExecutionLog
		var roleToolID, apiKeyID, requestID, errorMessage sql.NullString
		var inputParamsJSON []byte

		err := rows.Scan(
			&log.ID, &roleToolID, &log.ToolName, &log.RoleID, &apiKeyID,
			&requestID, &inputParamsJSON, &log.Status, &errorMessage, &log.ExecutedAt,
		)
		if err != nil {
			return nil, 0, err
		}

		if roleToolID.Valid {
			log.RoleToolID = roleToolID.String
		}
		if apiKeyID.Valid {
			log.APIKeyID = apiKeyID.String
		}
		if requestID.Valid {
			log.RequestID = requestID.String
		}
		if errorMessage.Valid {
			log.BlockReason = errorMessage.String
		}
		json.Unmarshal(inputParamsJSON, &log.ToolArguments)

		logs = append(logs, &log)
	}

	return logs, total, nil
}

// ============================================================================
// Legacy Compatibility Functions (deprecated, for gradual migration)
// ============================================================================

// GetDiscoveredTool is deprecated - use GetRoleTool instead
func (s *TenantStore) GetDiscoveredTool(ctx context.Context, id string) (*domain.RoleTool, error) {
	return s.GetRoleTool(ctx, id)
}

// CreateDiscoveredTool is deprecated - use CreateOrUpdateRoleTool instead
func (s *TenantStore) CreateDiscoveredTool(ctx context.Context, tool *domain.RoleTool) error {
	return s.CreateOrUpdateRoleTool(ctx, tool)
}

// GetToolByIdentity is deprecated - use GetRoleToolByIdentity instead
// Note: This requires roleID now, so callers must be updated
func (s *TenantStore) GetToolByIdentity(ctx context.Context, name, description, schemaHash string) (*domain.RoleTool, error) {
	// This function can no longer work without roleID
	// Return nil to indicate "not found" - callers should use GetRoleToolByIdentity
	return nil, nil
}

// UpdateToolSeen is deprecated - use UpdateRoleToolSeen instead
func (s *TenantStore) UpdateToolSeen(ctx context.Context, id string) error {
	return s.UpdateRoleToolSeen(ctx, id)
}

// ListDiscoveredTools is deprecated - use ListRoleTools instead
func (s *TenantStore) ListDiscoveredTools(ctx context.Context, filter domain.ToolFilter, limit, offset int) ([]*domain.RoleTool, int, error) {
	// Without roleID, we can't list tools anymore
	// Return empty to avoid breaking callers, but they should migrate to ListRoleTools
	return nil, 0, nil
}

// DeleteDiscoveredTool is deprecated - use DeleteRoleTool instead
func (s *TenantStore) DeleteDiscoveredTool(ctx context.Context, id string) error {
	return s.DeleteRoleTool(ctx, id)
}

// CreateToolPermission is deprecated - permissions are now inline in role_tools
func (s *TenantStore) CreateToolPermission(ctx context.Context, perm *domain.ToolRolePermission) error {
	return s.SetRoleToolPermission(ctx, perm.ToolID, perm.Status, perm.DecidedBy, perm.DecidedByEmail, perm.DecisionReason)
}

// GetToolPermission is deprecated - use GetRoleTool and check .Status
func (s *TenantStore) GetToolPermission(ctx context.Context, toolID, roleID string) (*domain.ToolRolePermission, error) {
	tool, err := s.GetRoleTool(ctx, toolID)
	if err != nil || tool == nil {
		return nil, err
	}
	return &domain.ToolRolePermission{
		ID:             tool.ID,
		ToolID:         tool.ID,
		RoleID:         tool.RoleID,
		Status:         tool.Status,
		DecidedBy:      tool.DecidedBy,
		DecidedByEmail: tool.DecidedByEmail,
		DecidedAt:      tool.DecidedAt,
		DecisionReason: tool.DecisionReason,
		CreatedAt:      tool.CreatedAt,
		UpdatedAt:      tool.UpdatedAt,
		Tool:           tool,
	}, nil
}

// ListToolPermissions is deprecated - use ListRoleTools instead
func (s *TenantStore) ListToolPermissions(ctx context.Context, roleID string) ([]*domain.ToolRolePermission, error) {
	tools, _, err := s.ListRoleTools(ctx, roleID, domain.ToolFilter{}, 1000, 0)
	if err != nil {
		return nil, err
	}

	perms := make([]*domain.ToolRolePermission, len(tools))
	for i, tool := range tools {
		perms[i] = &domain.ToolRolePermission{
			ID:             tool.ID,
			ToolID:         tool.ID,
			RoleID:         tool.RoleID,
			Status:         tool.Status,
			DecidedBy:      tool.DecidedBy,
			DecidedByEmail: tool.DecidedByEmail,
			DecidedAt:      tool.DecidedAt,
			DecisionReason: tool.DecisionReason,
			CreatedAt:      tool.CreatedAt,
			UpdatedAt:      tool.UpdatedAt,
			Tool:           tool,
		}
	}
	return perms, nil
}

// ListPendingTools is deprecated - use ListRoleTools with status filter instead
func (s *TenantStore) ListPendingTools(ctx context.Context) ([]*domain.RoleTool, error) {
	// List all pending tools across all roles (for backward compat)
	query := `
		SELECT id, role_id, name, description, schema_hash, parameters, category,
			first_seen_at, last_seen_at, first_seen_by, seen_count,
			status, decided_by, decided_by_email, decided_at, decision_reason,
			created_at, updated_at
		FROM role_tools
		WHERE status = 'PENDING'
		ORDER BY last_seen_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tools []*domain.RoleTool
	for rows.Next() {
		var tool domain.RoleTool
		var parametersJSON []byte
		var category, firstSeenBy, decidedBy, decidedByEmail, decisionReason sql.NullString
		var decidedAt sql.NullTime

		err := rows.Scan(
			&tool.ID, &tool.RoleID, &tool.Name, &tool.Description, &tool.SchemaHash, &parametersJSON, &category,
			&tool.FirstSeenAt, &tool.LastSeenAt, &firstSeenBy, &tool.SeenCount,
			&tool.Status, &decidedBy, &decidedByEmail, &decidedAt, &decisionReason,
			&tool.CreatedAt, &tool.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		json.Unmarshal(parametersJSON, &tool.Parameters)
		if category.Valid {
			tool.Category = category.String
		}
		if firstSeenBy.Valid {
			tool.FirstSeenBy = firstSeenBy.String
		}
		if decidedBy.Valid {
			tool.DecidedBy = decidedBy.String
		}
		if decidedByEmail.Valid {
			tool.DecidedByEmail = decidedByEmail.String
		}
		if decidedAt.Valid {
			tool.DecidedAt = &decidedAt.Time
		}
		if decisionReason.Valid {
			tool.DecisionReason = decisionReason.String
		}

		tools = append(tools, &tool)
	}

	return tools, nil
}

// BulkUpdatePermissions is deprecated - use BulkSetRoleToolPermissions instead
func (s *TenantStore) BulkUpdatePermissions(ctx context.Context, roleID string, status domain.ToolPermissionStatus, decidedBy, decidedByEmail string) (int, error) {
	return s.BulkSetRoleToolPermissions(ctx, roleID, status, decidedBy, decidedByEmail)
}

// Helper function for nullable strings
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
