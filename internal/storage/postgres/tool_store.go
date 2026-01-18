package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"time"

	"modelgate/internal/domain"
)

// ============================================================================
// Discovered Tools
// ============================================================================

// CreateDiscoveredTool creates a new discovered tool
func (s *TenantStore) CreateDiscoveredTool(ctx context.Context, tool *domain.DiscoveredTool) error {
	parametersJSON, err := json.Marshal(tool.Parameters)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO discovered_tools (
			id, name, description, schema_hash, parameters,
			first_seen_at, last_seen_at, first_seen_by, seen_count, category,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		)
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

	_, err = s.db.ExecContext(ctx, query,
		tool.ID, tool.Name, tool.Description, tool.SchemaHash, parametersJSON,
		tool.FirstSeenAt, tool.LastSeenAt, nullString(tool.FirstSeenBy), tool.SeenCount, nullString(tool.Category),
		now, now)

	return err
}

// GetDiscoveredTool gets a discovered tool by ID
func (s *TenantStore) GetDiscoveredTool(ctx context.Context, id string) (*domain.DiscoveredTool, error) {
	query := `
		SELECT id, name, description, schema_hash, parameters,
			first_seen_at, last_seen_at, first_seen_by, seen_count, category,
			created_at, updated_at
		FROM discovered_tools
		WHERE id = $1
	`

	var tool domain.DiscoveredTool
	var parametersJSON []byte
	var firstSeenBy, category sql.NullString

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&tool.ID, &tool.Name, &tool.Description, &tool.SchemaHash, &parametersJSON,
		&tool.FirstSeenAt, &tool.LastSeenAt, &firstSeenBy, &tool.SeenCount, &category,
		&tool.CreatedAt, &tool.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal(parametersJSON, &tool.Parameters)
	if firstSeenBy.Valid {
		tool.FirstSeenBy = firstSeenBy.String
	}
	if category.Valid {
		tool.Category = category.String
	}

	return &tool, nil
}

// GetToolByIdentity gets a tool by its unique identity (name + description + schema_hash)
func (s *TenantStore) GetToolByIdentity(ctx context.Context, name, description, schemaHash string) (*domain.DiscoveredTool, error) {
	query := `
		SELECT id, name, description, schema_hash, parameters,
			first_seen_at, last_seen_at, first_seen_by, seen_count, category,
			created_at, updated_at
		FROM discovered_tools
		WHERE name = $1 AND description = $2 AND schema_hash = $3
	`

	var tool domain.DiscoveredTool
	var parametersJSON []byte
	var firstSeenBy, category sql.NullString

	err := s.db.QueryRowContext(ctx, query, name, description, schemaHash).Scan(
		&tool.ID, &tool.Name, &tool.Description, &tool.SchemaHash, &parametersJSON,
		&tool.FirstSeenAt, &tool.LastSeenAt, &firstSeenBy, &tool.SeenCount, &category,
		&tool.CreatedAt, &tool.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal(parametersJSON, &tool.Parameters)
	if firstSeenBy.Valid {
		tool.FirstSeenBy = firstSeenBy.String
	}
	if category.Valid {
		tool.Category = category.String
	}

	return &tool, nil
}

// UpdateToolSeen updates the last_seen_at and increments seen_count
func (s *TenantStore) UpdateToolSeen(ctx context.Context, id string) error {
	query := `
		UPDATE discovered_tools
		SET last_seen_at = $2, seen_count = seen_count + 1, updated_at = $2
		WHERE id = $1
	`
	_, err := s.db.ExecContext(ctx, query, id, time.Now())
	return err
}

// ListDiscoveredTools lists discovered tools with filtering
func (s *TenantStore) ListDiscoveredTools(ctx context.Context, filter domain.ToolFilter, limit, offset int) ([]*domain.DiscoveredTool, int, error) {
	// Build query with filters
	query := `
		SELECT dt.id, dt.name, dt.description, dt.schema_hash, dt.parameters,
			dt.first_seen_at, dt.last_seen_at, dt.first_seen_by, dt.seen_count, dt.category,
			dt.created_at, dt.updated_at
		FROM discovered_tools dt
	`

	countQuery := `SELECT COUNT(*) FROM discovered_tools dt`

	whereClause := ` WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if filter.Name != "" {
		whereClause += ` AND dt.name ILIKE $` + string(rune('0'+argIdx))
		args = append(args, "%"+filter.Name+"%")
		argIdx++
	}

	if filter.Category != "" {
		whereClause += ` AND dt.category = $` + string(rune('0'+argIdx))
		args = append(args, filter.Category)
		argIdx++
	}

	if filter.RoleID != "" && filter.Status != "" {
		// Join with permissions table to filter by status
		query = `
			SELECT dt.id, dt.name, dt.description, dt.schema_hash, dt.parameters,
				dt.first_seen_at, dt.last_seen_at, dt.first_seen_by, dt.seen_count, dt.category,
				dt.created_at, dt.updated_at
			FROM discovered_tools dt
			LEFT JOIN tool_role_permissions trp ON dt.id = trp.tool_id AND trp.role_id = $` + string(rune('0'+argIdx))

		countQuery = `SELECT COUNT(*) FROM discovered_tools dt
			LEFT JOIN tool_role_permissions trp ON dt.id = trp.tool_id AND trp.role_id = $` + string(rune('0'+argIdx))

		args = append(args, filter.RoleID)
		argIdx++

		if filter.Status == domain.ToolStatusPending {
			whereClause += ` AND (trp.status IS NULL OR trp.status = 'PENDING')`
		} else {
			whereClause += ` AND trp.status = $` + string(rune('0'+argIdx))
			args = append(args, string(filter.Status))
			argIdx++
		}
	}

	// Count total
	var total int
	err := s.db.QueryRowContext(ctx, countQuery+whereClause, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get paginated results
	query += whereClause + ` ORDER BY dt.last_seen_at DESC LIMIT $` + string(rune('0'+argIdx)) + ` OFFSET $` + string(rune('0'+argIdx+1))
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var tools []*domain.DiscoveredTool
	for rows.Next() {
		var tool domain.DiscoveredTool
		var parametersJSON []byte
		var firstSeenBy, category sql.NullString

		err := rows.Scan(
			&tool.ID, &tool.Name, &tool.Description, &tool.SchemaHash, &parametersJSON,
			&tool.FirstSeenAt, &tool.LastSeenAt, &firstSeenBy, &tool.SeenCount, &category,
			&tool.CreatedAt, &tool.UpdatedAt)
		if err != nil {
			return nil, 0, err
		}

		json.Unmarshal(parametersJSON, &tool.Parameters)
		if firstSeenBy.Valid {
			tool.FirstSeenBy = firstSeenBy.String
		}
		if category.Valid {
			tool.Category = category.String
		}

		tools = append(tools, &tool)
	}

	return tools, total, nil
}

// DeleteDiscoveredTool deletes a discovered tool
func (s *TenantStore) DeleteDiscoveredTool(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM discovered_tools WHERE id = $1", id)
	return err
}

// ============================================================================
// Tool Role Permissions
// ============================================================================

// CreateToolPermission creates a new tool-role permission
func (s *TenantStore) CreateToolPermission(ctx context.Context, perm *domain.ToolRolePermission) error {
	query := `
		INSERT INTO tool_role_permissions (
			id, tool_id, role_id, status,
			decided_by, decided_by_email, decided_at, decision_reason,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		)
		ON CONFLICT (tool_id, role_id) DO UPDATE SET
			status = EXCLUDED.status,
			decided_by = EXCLUDED.decided_by,
			decided_by_email = EXCLUDED.decided_by_email,
			decided_at = EXCLUDED.decided_at,
			decision_reason = EXCLUDED.decision_reason,
			updated_at = EXCLUDED.updated_at
	`

	now := time.Now()
	_, err := s.db.ExecContext(ctx, query,
		perm.ID, perm.ToolID, perm.RoleID, perm.Status,
		nullString(perm.DecidedBy), nullString(perm.DecidedByEmail), perm.DecidedAt, nullString(perm.DecisionReason),
		now, now)

	return err
}

// GetToolPermission gets the permission for a tool-role combination
// First tries to find an exact tool_id match, then falls back to name-based matching
// This allows a single permission (e.g., "REMOVED") to apply to all variants of a tool
func (s *TenantStore) GetToolPermission(ctx context.Context, toolID, roleID string) (*domain.ToolRolePermission, error) {
	slog.Info("GetToolPermission called", "tool_id", toolID, "role_id", roleID)

	// First try exact tool_id match
	query := `
		SELECT id, tool_id, role_id, status,
			decided_by, decided_by_email, decided_at, decision_reason,
			created_at, updated_at
		FROM tool_role_permissions
		WHERE tool_id = $1 AND role_id = $2
	`

	var perm domain.ToolRolePermission
	var decidedBy, decidedByEmail, decisionReason sql.NullString
	var decidedAt sql.NullTime

	err := s.db.QueryRowContext(ctx, query, toolID, roleID).Scan(
		&perm.ID, &perm.ToolID, &perm.RoleID, &perm.Status,
		&decidedBy, &decidedByEmail, &decidedAt, &decisionReason,
		&perm.CreatedAt, &perm.UpdatedAt)

	if err == sql.ErrNoRows {
		slog.Info("No exact permission match, falling back to name-based lookup", "tool_id", toolID, "role_id", roleID)
		// No exact match - try to find a permission for ANY tool with the same name
		// This allows setting a permission once (e.g., REMOVED) that applies to all variants
		return s.getToolPermissionByName(ctx, toolID, roleID)
	}
	if err != nil {
		slog.Error("Error getting tool permission", "tool_id", toolID, "role_id", roleID, "error", err)
		return nil, err
	}

	slog.Info("Found exact permission match", "tool_id", toolID, "role_id", roleID, "status", perm.Status)

	if decidedBy.Valid {
		perm.DecidedBy = decidedBy.String
	}
	if decidedByEmail.Valid {
		perm.DecidedByEmail = decidedByEmail.String
	}
	if decidedAt.Valid {
		perm.DecidedAt = &decidedAt.Time
	}
	if decisionReason.Valid {
		perm.DecisionReason = decisionReason.String
	}

	return &perm, nil
}

// getToolPermissionByName looks up a permission by tool name for a role
// This allows a permission set on one variant of a tool to apply to all variants with the same name
func (s *TenantStore) getToolPermissionByName(ctx context.Context, toolID, roleID string) (*domain.ToolRolePermission, error) {
	// First get the tool name for this tool ID
	var toolName string
	err := s.db.QueryRowContext(ctx,
		"SELECT name FROM discovered_tools WHERE id = $1", toolID).Scan(&toolName)
	if err != nil {
		// Tool not found, return nil
		return nil, nil
	}

	// Look for any permission for a tool with this name and role
	// Prefer REMOVED or DENIED status (more restrictive) if multiple exist
	query := `
		SELECT trp.id, trp.tool_id, trp.role_id, trp.status,
			trp.decided_by, trp.decided_by_email, trp.decided_at, trp.decision_reason,
			trp.created_at, trp.updated_at
		FROM tool_role_permissions trp
		JOIN discovered_tools dt ON trp.tool_id = dt.id
		WHERE dt.name = $1 AND trp.role_id = $2
		ORDER BY 
			CASE trp.status 
				WHEN 'REMOVED' THEN 1
				WHEN 'DENIED' THEN 2
				WHEN 'ALLOWED' THEN 3
				ELSE 4 
			END
		LIMIT 1
	`

	var perm domain.ToolRolePermission
	var decidedBy, decidedByEmail, decisionReason sql.NullString
	var decidedAt sql.NullTime

	err = s.db.QueryRowContext(ctx, query, toolName, roleID).Scan(
		&perm.ID, &perm.ToolID, &perm.RoleID, &perm.Status,
		&decidedBy, &decidedByEmail, &decidedAt, &decisionReason,
		&perm.CreatedAt, &perm.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if decidedBy.Valid {
		perm.DecidedBy = decidedBy.String
	}
	if decidedByEmail.Valid {
		perm.DecidedByEmail = decidedByEmail.String
	}
	if decidedAt.Valid {
		perm.DecidedAt = &decidedAt.Time
	}
	if decisionReason.Valid {
		perm.DecisionReason = decisionReason.String
	}

	return &perm, nil
}

// UpdateToolPermission updates a tool-role permission
func (s *TenantStore) UpdateToolPermission(ctx context.Context, perm *domain.ToolRolePermission) error {
	query := `
		UPDATE tool_role_permissions
		SET status = $3, decided_by = $4, decided_by_email = $5, decided_at = $6, decision_reason = $7, updated_at = $8
		WHERE tool_id = $1 AND role_id = $2
	`

	_, err := s.db.ExecContext(ctx, query,
		perm.ToolID, perm.RoleID, perm.Status,
		nullString(perm.DecidedBy), nullString(perm.DecidedByEmail), perm.DecidedAt, nullString(perm.DecisionReason),
		time.Now())

	return err
}

// ListToolPermissions lists all permissions for a role
func (s *TenantStore) ListToolPermissions(ctx context.Context, roleID string) ([]*domain.ToolRolePermission, error) {
	query := `
		SELECT trp.id, trp.tool_id, trp.role_id, trp.status,
			trp.decided_by, trp.decided_by_email, trp.decided_at, trp.decision_reason,
			trp.created_at, trp.updated_at,
			dt.id, dt.name, dt.description, dt.schema_hash, dt.parameters,
			dt.first_seen_at, dt.last_seen_at, dt.first_seen_by, dt.seen_count, dt.category
		FROM tool_role_permissions trp
		JOIN discovered_tools dt ON trp.tool_id = dt.id
		WHERE trp.role_id = $1
		ORDER BY dt.name
	`

	rows, err := s.db.QueryContext(ctx, query, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []*domain.ToolRolePermission
	for rows.Next() {
		var perm domain.ToolRolePermission
		var tool domain.DiscoveredTool
		var decidedBy, decidedByEmail, decisionReason sql.NullString
		var decidedAt sql.NullTime
		var parametersJSON []byte
		var firstSeenBy, category sql.NullString

		err := rows.Scan(
			&perm.ID, &perm.ToolID, &perm.RoleID, &perm.Status,
			&decidedBy, &decidedByEmail, &decidedAt, &decisionReason,
			&perm.CreatedAt, &perm.UpdatedAt,
			&tool.ID, &tool.Name, &tool.Description, &tool.SchemaHash, &parametersJSON,
			&tool.FirstSeenAt, &tool.LastSeenAt, &firstSeenBy, &tool.SeenCount, &category)
		if err != nil {
			return nil, err
		}

		if decidedBy.Valid {
			perm.DecidedBy = decidedBy.String
		}
		if decidedByEmail.Valid {
			perm.DecidedByEmail = decidedByEmail.String
		}
		if decidedAt.Valid {
			perm.DecidedAt = &decidedAt.Time
		}
		if decisionReason.Valid {
			perm.DecisionReason = decisionReason.String
		}

		json.Unmarshal(parametersJSON, &tool.Parameters)
		if firstSeenBy.Valid {
			tool.FirstSeenBy = firstSeenBy.String
		}
		if category.Valid {
			tool.Category = category.String
		}

		perm.Tool = &tool
		perms = append(perms, &perm)
	}

	return perms, nil
}

// ListPendingTools lists all tools that don't have permissions set for any role (pending review)
func (s *TenantStore) ListPendingTools(ctx context.Context) ([]*domain.DiscoveredTool, error) {
	query := `
		SELECT dt.id, dt.name, dt.description, dt.schema_hash, dt.parameters,
			dt.first_seen_at, dt.last_seen_at, dt.first_seen_by, dt.seen_count, dt.category,
			dt.created_at, dt.updated_at
		FROM discovered_tools dt
		WHERE NOT EXISTS (
			SELECT 1 FROM tool_role_permissions trp
			WHERE trp.tool_id = dt.id AND trp.status != 'PENDING'
		)
		ORDER BY dt.last_seen_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tools []*domain.DiscoveredTool
	for rows.Next() {
		var tool domain.DiscoveredTool
		var parametersJSON []byte
		var firstSeenBy, category sql.NullString

		err := rows.Scan(
			&tool.ID, &tool.Name, &tool.Description, &tool.SchemaHash, &parametersJSON,
			&tool.FirstSeenAt, &tool.LastSeenAt, &firstSeenBy, &tool.SeenCount, &category,
			&tool.CreatedAt, &tool.UpdatedAt)
		if err != nil {
			return nil, err
		}

		json.Unmarshal(parametersJSON, &tool.Parameters)
		if firstSeenBy.Valid {
			tool.FirstSeenBy = firstSeenBy.String
		}
		if category.Valid {
			tool.Category = category.String
		}

		tools = append(tools, &tool)
	}

	return tools, nil
}

// BulkUpdatePermissions sets permissions for all tools for a role
func (s *TenantStore) BulkUpdatePermissions(ctx context.Context, roleID string, status domain.ToolPermissionStatus, decidedBy, decidedByEmail string) (int, error) {
	now := time.Now()

	// First, insert permissions for tools that don't have any
	insertQuery := `
		INSERT INTO tool_role_permissions (id, tool_id, role_id, status, decided_by, decided_by_email, decided_at, created_at, updated_at)
		SELECT gen_random_uuid(), dt.id, $1, $2, $3, $4, $5, $5, $5
		FROM discovered_tools dt
		WHERE NOT EXISTS (
			SELECT 1 FROM tool_role_permissions trp
			WHERE trp.tool_id = dt.id AND trp.role_id = $1
		)
	`

	_, err := s.db.ExecContext(ctx, insertQuery, roleID, status, nullString(decidedBy), nullString(decidedByEmail), now)
	if err != nil {
		return 0, err
	}

	// Then update existing pending permissions
	updateQuery := `
		UPDATE tool_role_permissions
		SET status = $2, decided_by = $3, decided_by_email = $4, decided_at = $5, updated_at = $5
		WHERE role_id = $1 AND status = 'PENDING'
	`

	result, err := s.db.ExecContext(ctx, updateQuery, roleID, status, nullString(decidedBy), nullString(decidedByEmail), now)
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
	outputResultJSON := []byte("{}")

	query := `
		INSERT INTO tool_execution_logs (
			id, tool_id, role_id, api_key_id,
			request_id, input_params, output_result, status, error_message,
			duration_ms, token_count, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		)
	`

	now := time.Now()
	if log.ExecutedAt.IsZero() {
		log.ExecutedAt = now
	}

	// Map status - "blocked" becomes error_message
	status := log.Status
	errorMessage := log.BlockReason

	_, err := s.db.ExecContext(ctx, query,
		log.ID, log.ToolID, log.RoleID, nullString(log.APIKeyID),
		nullString(log.RequestID), inputParamsJSON, outputResultJSON, status, nullString(errorMessage),
		0, 0, log.ExecutedAt)

	return err
}

// ListToolExecutionLogs lists tool execution logs with filtering
func (s *TenantStore) ListToolExecutionLogs(ctx context.Context, filter domain.ToolLogFilter, limit, offset int) ([]*domain.ToolExecutionLog, int, error) {
	query := `
		SELECT tel.id, tel.tool_id, tel.role_id, tel.api_key_id,
			tel.request_id, tel.input_params, tel.status, tel.error_message,
			tel.duration_ms, tel.created_at, dt.name as tool_name
		FROM tool_execution_logs tel
		LEFT JOIN discovered_tools dt ON tel.tool_id = dt.id
		WHERE 1=1
	`

	countQuery := `SELECT COUNT(*) FROM tool_execution_logs tel WHERE 1=1`

	args := []interface{}{}
	argIdx := 1

	if filter.ToolID != "" {
		query += ` AND tel.tool_id = $` + string(rune('0'+argIdx))
		countQuery += ` AND tel.tool_id = $` + string(rune('0'+argIdx))
		args = append(args, filter.ToolID)
		argIdx++
	}

	if filter.RoleID != "" {
		query += ` AND tel.role_id = $` + string(rune('0'+argIdx))
		countQuery += ` AND tel.role_id = $` + string(rune('0'+argIdx))
		args = append(args, filter.RoleID)
		argIdx++
	}

	if filter.Status != "" {
		query += ` AND tel.status = $` + string(rune('0'+argIdx))
		countQuery += ` AND tel.status = $` + string(rune('0'+argIdx))
		args = append(args, filter.Status)
		argIdx++
	}

	if !filter.StartTime.IsZero() {
		query += ` AND tel.created_at >= $` + string(rune('0'+argIdx))
		countQuery += ` AND tel.created_at >= $` + string(rune('0'+argIdx))
		args = append(args, filter.StartTime)
		argIdx++
	}

	if !filter.EndTime.IsZero() {
		query += ` AND tel.created_at <= $` + string(rune('0'+argIdx))
		countQuery += ` AND tel.created_at <= $` + string(rune('0'+argIdx))
		args = append(args, filter.EndTime)
		argIdx++
	}

	// Count total
	var total int
	err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// Get paginated results
	query += ` ORDER BY tel.created_at DESC LIMIT $` + string(rune('0'+argIdx)) + ` OFFSET $` + string(rune('0'+argIdx+1))
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []*domain.ToolExecutionLog
	for rows.Next() {
		var log domain.ToolExecutionLog
		var apiKeyID, requestID, errorMessage, toolName sql.NullString
		var inputParamsJSON []byte
		var durationMs sql.NullInt64

		err := rows.Scan(
			&log.ID, &log.ToolID, &log.RoleID, &apiKeyID,
			&requestID, &inputParamsJSON, &log.Status, &errorMessage,
			&durationMs, &log.ExecutedAt, &toolName)
		if err != nil {
			return nil, 0, err
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
		if toolName.Valid {
			log.ToolName = toolName.String
		}
		json.Unmarshal(inputParamsJSON, &log.ToolArguments)

		logs = append(logs, &log)
	}

	return logs, total, nil
}

// Helper function for nullable strings
func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
