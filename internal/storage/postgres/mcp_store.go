package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"modelgate/internal/domain"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// ============================================
// MCP SERVER OPERATIONS
// ============================================

// marshalAuthConfigForStorage bypasses the MarshalJSON method to store unmasked values
func marshalAuthConfigForStorage(config domain.MCPAuthConfig) ([]byte, error) {
	// Create a map with all fields to bypass custom MarshalJSON
	data := map[string]interface{}{
		"api_key":        config.APIKey,
		"api_key_header": config.APIKeyHeader,
		"bearer_token":   config.BearerToken,
		"client_id":      config.ClientID,
		"client_secret":  config.ClientSecret,
		"token_url":      config.TokenURL,
		"scopes":         config.Scopes,
		"username":       config.Username,
		"password":       config.Password,
		"client_cert":    config.ClientCert,
		"client_key":     config.ClientKey,
		"ca_cert":        config.CACert,
		"aws_region":     config.AWSRegion,
		"aws_role_arn":   config.AWSRoleARN,
	}
	return json.Marshal(data)
}

// CreateMCPServer creates a new MCP server
func (s *TenantStore) CreateMCPServer(ctx context.Context, server *domain.MCPServer) error {
	if server.ID == "" {
		server.ID = uuid.New().String()
	}

	arguments, _ := json.Marshal(server.Arguments)
	environment, _ := json.Marshal(server.Environment)
	authConfig, _ := marshalAuthConfigForStorage(server.AuthConfig) // Don't use json.Marshal to avoid masking
	metadata, _ := json.Marshal(server.Metadata)

	query := `
		INSERT INTO mcp_servers (
			id, name, slug, description,
			server_type, endpoint, arguments, environment,
			auth_type, auth_config_encrypted,
			version, commit_hash,
			status, auto_sync, sync_interval_minutes, health_check_interval_seconds,
			tags, metadata, created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
	`

	// Convert empty string to nil for UUID fields
	var createdBy interface{}
	if server.CreatedBy != "" {
		createdBy = server.CreatedBy
	}

	_, err := s.db.ExecContext(ctx, query,
		server.ID, server.Name, server.Slug, server.Description,
		server.ServerType, server.Endpoint, arguments, environment,
		server.AuthType, authConfig, // TODO: Encrypt auth config
		server.Version, server.CommitHash,
		server.Status, server.AutoSync, server.SyncIntervalMinutes, server.HealthCheckIntervalSeconds,
		pq.Array(server.Tags), metadata, createdBy,
	)

	return err
}

// UpdateMCPServer updates an existing MCP server
func (s *TenantStore) UpdateMCPServer(ctx context.Context, server *domain.MCPServer) error {
	arguments, _ := json.Marshal(server.Arguments)
	environment, _ := json.Marshal(server.Environment)
	authConfig, _ := marshalAuthConfigForStorage(server.AuthConfig) // Don't use json.Marshal to avoid masking
	metadata, _ := json.Marshal(server.Metadata)

	query := `
		UPDATE mcp_servers SET
			name = $2, slug = $3, description = $4,
			server_type = $5, endpoint = $6, arguments = $7, environment = $8,
			auth_type = $9, auth_config_encrypted = $10,
			version = $11, commit_hash = $12, last_sync_at = $13,
			status = $14, last_health_check = $15, error_message = $16, retry_count = $17,
			auto_sync = $18, sync_interval_minutes = $19, health_check_interval_seconds = $20,
			tags = $21, metadata = $22, tool_count = $23
		WHERE id = $1
	`

	_, err := s.db.ExecContext(ctx, query,
		server.ID, server.Name, server.Slug, server.Description,
		server.ServerType, server.Endpoint, arguments, environment,
		server.AuthType, authConfig,
		server.Version, server.CommitHash, server.LastSyncAt,
		server.Status, server.LastHealthCheck, server.ErrorMessage, server.RetryCount,
		server.AutoSync, server.SyncIntervalMinutes, server.HealthCheckIntervalSeconds,
		pq.Array(server.Tags), metadata, server.ToolCount,
	)

	return err
}

// DeleteMCPServer deletes an MCP server and all its tools
func (s *TenantStore) DeleteMCPServer(ctx context.Context, serverID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM mcp_servers WHERE id = $1", serverID)
	return err
}

// GetMCPServer retrieves an MCP server by ID
func (s *TenantStore) GetMCPServer(ctx context.Context, serverID string) (*domain.MCPServer, error) {
	query := `
		SELECT id, name, slug, description,
			server_type, endpoint, arguments, environment,
			auth_type, auth_config_encrypted,
			version, commit_hash, last_sync_at,
			status, last_health_check, error_message, retry_count,
			auto_sync, sync_interval_minutes, health_check_interval_seconds,
			tags, metadata, created_at, updated_at, created_by,
			(SELECT COUNT(*) FROM mcp_tools WHERE server_id = mcp_servers.id) as tool_count
		FROM mcp_servers
		WHERE id = $1
	`

	return s.scanMCPServer(s.db.QueryRowContext(ctx, query, serverID))
}

// GetMCPServerByName retrieves an MCP server by name
func (s *TenantStore) GetMCPServerByName(ctx context.Context, name string) (*domain.MCPServer, error) {
	query := `
		SELECT id, name, slug, description,
			server_type, endpoint, arguments, environment,
			auth_type, auth_config_encrypted,
			version, commit_hash, last_sync_at,
			status, last_health_check, error_message, retry_count,
			auto_sync, sync_interval_minutes, health_check_interval_seconds,
			tags, metadata, created_at, updated_at, created_by,
			(SELECT COUNT(*) FROM mcp_tools WHERE server_id = mcp_servers.id) as tool_count
		FROM mcp_servers
		WHERE name = $1
	`

	return s.scanMCPServer(s.db.QueryRowContext(ctx, query, name))
}

// ListMCPServers lists all MCP servers
func (s *TenantStore) ListMCPServers(ctx context.Context) ([]*domain.MCPServer, error) {
	query := `
		SELECT id, name, slug, description,
			server_type, endpoint, arguments, environment,
			auth_type, auth_config_encrypted,
			version, commit_hash, last_sync_at,
			status, last_health_check, error_message, retry_count,
			auto_sync, sync_interval_minutes, health_check_interval_seconds,
			tags, metadata, created_at, updated_at, created_by,
			(SELECT COUNT(*) FROM mcp_tools WHERE server_id = mcp_servers.id) as tool_count
		FROM mcp_servers
		ORDER BY name
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var servers []*domain.MCPServer
	for rows.Next() {
		server, err := s.scanMCPServerFromRows(rows)
		if err != nil {
			return nil, err
		}
		servers = append(servers, server)
	}

	return servers, rows.Err()
}

func (s *TenantStore) scanMCPServer(row *sql.Row) (*domain.MCPServer, error) {
	var server domain.MCPServer
	var arguments, environment, authConfig, metadata []byte
	var lastSyncAt, lastHealthCheck sql.NullTime
	var errorMessage, commitHash, createdBy sql.NullString
	var tags pq.StringArray

	err := row.Scan(
		&server.ID, &server.Name, &server.Slug, &server.Description,
		&server.ServerType, &server.Endpoint, &arguments, &environment,
		&server.AuthType, &authConfig,
		&server.Version, &commitHash, &lastSyncAt,
		&server.Status, &lastHealthCheck, &errorMessage, &server.RetryCount,
		&server.AutoSync, &server.SyncIntervalMinutes, &server.HealthCheckIntervalSeconds,
		&tags, &metadata, &server.CreatedAt, &server.UpdatedAt, &createdBy,
		&server.ToolCount,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal(arguments, &server.Arguments)
	_ = json.Unmarshal(environment, &server.Environment)
	_ = json.Unmarshal(authConfig, &server.AuthConfig)
	_ = json.Unmarshal(metadata, &server.Metadata)

	server.Tags = tags
	if lastSyncAt.Valid {
		server.LastSyncAt = &lastSyncAt.Time
	}
	if lastHealthCheck.Valid {
		server.LastHealthCheck = &lastHealthCheck.Time
	}
	if errorMessage.Valid {
		server.ErrorMessage = errorMessage.String
	}
	if commitHash.Valid {
		server.CommitHash = commitHash.String
	}
	if createdBy.Valid {
		server.CreatedBy = createdBy.String
	}

	return &server, nil
}

func (s *TenantStore) scanMCPServerFromRows(rows *sql.Rows) (*domain.MCPServer, error) {
	var server domain.MCPServer
	var arguments, environment, authConfig, metadata []byte
	var lastSyncAt, lastHealthCheck sql.NullTime
	var errorMessage, commitHash, createdBy sql.NullString
	var tags pq.StringArray

	err := rows.Scan(
		&server.ID, &server.Name, &server.Slug, &server.Description,
		&server.ServerType, &server.Endpoint, &arguments, &environment,
		&server.AuthType, &authConfig,
		&server.Version, &commitHash, &lastSyncAt,
		&server.Status, &lastHealthCheck, &errorMessage, &server.RetryCount,
		&server.AutoSync, &server.SyncIntervalMinutes, &server.HealthCheckIntervalSeconds,
		&tags, &metadata, &server.CreatedAt, &server.UpdatedAt, &createdBy,
		&server.ToolCount,
	)
	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal(arguments, &server.Arguments)
	_ = json.Unmarshal(environment, &server.Environment)
	_ = json.Unmarshal(authConfig, &server.AuthConfig)
	_ = json.Unmarshal(metadata, &server.Metadata)

	server.Tags = tags
	if lastSyncAt.Valid {
		server.LastSyncAt = &lastSyncAt.Time
	}
	if lastHealthCheck.Valid {
		server.LastHealthCheck = &lastHealthCheck.Time
	}
	if errorMessage.Valid {
		server.ErrorMessage = errorMessage.String
	}
	if commitHash.Valid {
		server.CommitHash = commitHash.String
	}
	if createdBy.Valid {
		server.CreatedBy = createdBy.String
	}

	return &server, nil
}

// ============================================
// MCP TOOL OPERATIONS
// ============================================

// UpsertMCPTool creates or updates an MCP tool
func (s *TenantStore) UpsertMCPTool(ctx context.Context, tool *domain.MCPTool) error {
	if tool.ID == "" {
		tool.ID = uuid.New().String()
	}

	inputSchema, _ := json.Marshal(tool.InputSchema)
	outputSchema, _ := json.Marshal(tool.OutputSchema)
	inputExamples, _ := json.Marshal(tool.InputExamples)

	query := `
		INSERT INTO mcp_tools (
			id, server_id, name, description, category,
			input_schema, output_schema, input_examples,
			defer_loading, version
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (server_id, name) DO UPDATE SET
			description = EXCLUDED.description,
			category = EXCLUDED.category,
			input_schema = EXCLUDED.input_schema,
			output_schema = EXCLUDED.output_schema,
			input_examples = EXCLUDED.input_examples,
			defer_loading = EXCLUDED.defer_loading,
			version = EXCLUDED.version,
			is_deprecated = FALSE,
			updated_at = NOW()
	`

	_, err := s.db.ExecContext(ctx, query,
		tool.ID, tool.ServerID, tool.Name, tool.Description, tool.Category,
		inputSchema, outputSchema, inputExamples,
		tool.DeferLoading, tool.Version,
	)

	return err
}

// DeleteMCPTool deletes an MCP tool
func (s *TenantStore) DeleteMCPTool(ctx context.Context, toolID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM mcp_tools WHERE id = $1", toolID)
	return err
}

// DeleteMCPToolsByServer deletes all tools for a server
func (s *TenantStore) DeleteMCPToolsByServer(ctx context.Context, serverID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM mcp_tools WHERE server_id = $1", serverID)
	return err
}

// GetMCPTool retrieves an MCP tool by ID
func (s *TenantStore) GetMCPTool(ctx context.Context, toolID string) (*domain.MCPTool, error) {
	query := `
		SELECT t.id, t.server_id, s.name as server_name,
			t.name, t.description, t.category,
			t.input_schema, t.output_schema, t.input_examples,
			t.defer_loading, t.is_deprecated, t.deprecation_message, t.deprecated_at,
			t.version, t.execution_count, t.last_executed_at, t.avg_execution_time_ms,
			t.created_at, t.updated_at
		FROM mcp_tools t
		JOIN mcp_servers s ON t.server_id = s.id
		WHERE t.id = $1
	`

	return s.scanMCPTool(s.db.QueryRowContext(ctx, query, toolID))
}

// GetMCPToolByName retrieves an MCP tool by server and name
func (s *TenantStore) GetMCPToolByName(ctx context.Context, serverID, name string) (*domain.MCPTool, error) {
	query := `
		SELECT t.id, t.server_id, s.name as server_name,
			t.name, t.description, t.category,
			t.input_schema, t.output_schema, t.input_examples,
			t.defer_loading, t.is_deprecated, t.deprecation_message, t.deprecated_at,
			t.version, t.execution_count, t.last_executed_at, t.avg_execution_time_ms,
			t.created_at, t.updated_at
		FROM mcp_tools t
		JOIN mcp_servers s ON t.server_id = s.id
		WHERE t.server_id = $1 AND t.name = $2
	`

	return s.scanMCPTool(s.db.QueryRowContext(ctx, query, serverID, name))
}

// ListMCPTools lists all tools for a server
func (s *TenantStore) ListMCPTools(ctx context.Context, serverID string) ([]*domain.MCPTool, error) {
	query := `
		SELECT t.id, t.server_id, s.name as server_name,
			t.name, t.description, t.category,
			t.input_schema, t.output_schema, t.input_examples,
			t.defer_loading, t.is_deprecated, t.deprecation_message, t.deprecated_at,
			t.version, t.execution_count, t.last_executed_at, t.avg_execution_time_ms,
			t.created_at, t.updated_at
		FROM mcp_tools t
		JOIN mcp_servers s ON t.server_id = s.id
		WHERE t.server_id = $1
		ORDER BY t.name
	`

	return s.queryMCPTools(ctx, query, serverID)
}

// ListAllMCPTools lists all tools in the database
func (s *TenantStore) ListAllMCPTools(ctx context.Context) ([]*domain.MCPTool, error) {
	query := `
		SELECT t.id, t.server_id, s.name as server_name,
			t.name, t.description, t.category,
			t.input_schema, t.output_schema, t.input_examples,
			t.defer_loading, t.is_deprecated, t.deprecation_message, t.deprecated_at,
			t.version, t.execution_count, t.last_executed_at, t.avg_execution_time_ms,
			t.created_at, t.updated_at
		FROM mcp_tools t
		JOIN mcp_servers s ON t.server_id = s.id
		ORDER BY s.name, t.name
	`

	return s.queryMCPTools(ctx, query)
}

// UpdateToolEmbeddings updates the embeddings for a tool
func (s *TenantStore) UpdateToolEmbeddings(ctx context.Context, toolID string, nameEmb, descEmb, combinedEmb []float32) error {
	query := `
		UPDATE mcp_tools SET
			name_embedding = $2,
			description_embedding = $3,
			combined_embedding = $4,
			updated_at = NOW()
		WHERE id = $1
	`

	_, err := s.db.ExecContext(ctx, query,
		toolID,
		vectorToString(nameEmb),
		vectorToString(descEmb),
		vectorToString(combinedEmb),
	)

	return err
}

// DeprecateMCPTool marks a tool as deprecated
func (s *TenantStore) DeprecateMCPTool(ctx context.Context, serverID, toolName, message string) error {
	query := `
		UPDATE mcp_tools SET
			is_deprecated = TRUE,
			deprecation_message = $3,
			deprecated_at = NOW(),
			updated_at = NOW()
		WHERE server_id = $1 AND name = $2
	`

	_, err := s.db.ExecContext(ctx, query, serverID, toolName, message)
	return err
}

func (s *TenantStore) queryMCPTools(ctx context.Context, query string, args ...any) ([]*domain.MCPTool, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tools []*domain.MCPTool
	for rows.Next() {
		tool, err := s.scanMCPToolFromRows(rows)
		if err != nil {
			return nil, err
		}
		tools = append(tools, tool)
	}

	return tools, rows.Err()
}

func (s *TenantStore) scanMCPTool(row *sql.Row) (*domain.MCPTool, error) {
	var tool domain.MCPTool
	var inputSchema, outputSchema, inputExamples []byte
	var deprecationMessage, version sql.NullString
	var deprecatedAt, lastExecutedAt sql.NullTime
	var avgExecTime sql.NullInt64

	err := row.Scan(
		&tool.ID, &tool.ServerID, &tool.ServerName,
		&tool.Name, &tool.Description, &tool.Category,
		&inputSchema, &outputSchema, &inputExamples,
		&tool.DeferLoading, &tool.IsDeprecated, &deprecationMessage, &deprecatedAt,
		&version, &tool.ExecutionCount, &lastExecutedAt, &avgExecTime,
		&tool.CreatedAt, &tool.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal(inputSchema, &tool.InputSchema)
	_ = json.Unmarshal(outputSchema, &tool.OutputSchema)
	_ = json.Unmarshal(inputExamples, &tool.InputExamples)

	if deprecationMessage.Valid {
		tool.DeprecationMessage = deprecationMessage.String
	}
	if deprecatedAt.Valid {
		tool.DeprecatedAt = &deprecatedAt.Time
	}
	if version.Valid {
		tool.Version = version.String
	}
	if lastExecutedAt.Valid {
		tool.LastExecutedAt = &lastExecutedAt.Time
	}
	if avgExecTime.Valid {
		tool.AvgExecutionTimeMs = int(avgExecTime.Int64)
	}

	return &tool, nil
}

func (s *TenantStore) scanMCPToolFromRows(rows *sql.Rows) (*domain.MCPTool, error) {
	var tool domain.MCPTool
	var inputSchema, outputSchema, inputExamples []byte
	var deprecationMessage, version sql.NullString
	var deprecatedAt, lastExecutedAt sql.NullTime
	var avgExecTime sql.NullInt64

	err := rows.Scan(
		&tool.ID, &tool.ServerID, &tool.ServerName,
		&tool.Name, &tool.Description, &tool.Category,
		&inputSchema, &outputSchema, &inputExamples,
		&tool.DeferLoading, &tool.IsDeprecated, &deprecationMessage, &deprecatedAt,
		&version, &tool.ExecutionCount, &lastExecutedAt, &avgExecTime,
		&tool.CreatedAt, &tool.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal(inputSchema, &tool.InputSchema)
	_ = json.Unmarshal(outputSchema, &tool.OutputSchema)
	_ = json.Unmarshal(inputExamples, &tool.InputExamples)

	if deprecationMessage.Valid {
		tool.DeprecationMessage = deprecationMessage.String
	}
	if deprecatedAt.Valid {
		tool.DeprecatedAt = &deprecatedAt.Time
	}
	if version.Valid {
		tool.Version = version.String
	}
	if lastExecutedAt.Valid {
		tool.LastExecutedAt = &lastExecutedAt.Time
	}
	if avgExecTime.Valid {
		tool.AvgExecutionTimeMs = int(avgExecTime.Int64)
	}

	return &tool, nil
}

// vectorToString converts a float32 slice to CSV format for storage
func vectorToString(v []float32) string {
	if len(v) == 0 {
		return ""
	}
	parts := make([]string, len(v))
	for i, f := range v {
		parts[i] = fmt.Sprintf("%f", f)
	}
	return strings.Join(parts, ",")
}

// parseVector converts a CSV string back to float32 slice
func parseVector(s string) []float32 {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]float32, 0, len(parts))
	for _, p := range parts {
		if f, err := strconv.ParseFloat(strings.TrimSpace(p), 32); err == nil {
			result = append(result, float32(f))
		}
	}
	return result
}

// cosineSimilarity computes cosine similarity between two vectors
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// ============================================
// MCP VERSION OPERATIONS
// ============================================

// CreateMCPServerVersion creates a new version snapshot or updates if exists
func (s *TenantStore) CreateMCPServerVersion(ctx context.Context, version *domain.MCPServerVersion) error {
	if version.ID == "" {
		version.ID = uuid.New().String()
	}

	toolDefs, _ := json.Marshal(version.ToolDefinitions)
	changes, _ := json.Marshal(version.Changes)

	// Convert empty string to nil for UUID fields
	var createdBy interface{}
	if version.CreatedBy != "" {
		createdBy = version.CreatedBy
	}

	query := `
		INSERT INTO mcp_server_versions (
			id, server_id, version, commit_hash,
			tool_definitions, tool_count,
			changes, changes_summary, has_breaking_changes,
			created_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (server_id, version)
		DO UPDATE SET
			tool_definitions = EXCLUDED.tool_definitions,
			tool_count = EXCLUDED.tool_count,
			changes = EXCLUDED.changes,
			changes_summary = EXCLUDED.changes_summary,
			has_breaking_changes = EXCLUDED.has_breaking_changes,
			created_at = NOW()
	`

	_, err := s.db.ExecContext(ctx, query,
		version.ID, version.ServerID, version.Version, version.CommitHash,
		toolDefs, version.ToolCount,
		changes, version.ChangesSummary, version.HasBreakingChanges,
		createdBy,
	)

	return err
}

// UpdateMCPServerVersion updates an existing MCP server version
func (s *TenantStore) UpdateMCPServerVersion(ctx context.Context, version *domain.MCPServerVersion) error {
	toolDefs, _ := json.Marshal(version.ToolDefinitions)
	changes, _ := json.Marshal(version.Changes)

	query := `
		UPDATE mcp_server_versions SET
			tool_definitions = $1,
			tool_count = $2,
			changes = $3,
			changes_summary = $4,
			has_breaking_changes = $5,
			created_at = $6
		WHERE id = $7
	`

	_, err := s.db.ExecContext(ctx, query,
		toolDefs, version.ToolCount,
		changes, version.ChangesSummary, version.HasBreakingChanges,
		version.CreatedAt,
		version.ID,
	)

	return err
}

// GetMCPServerVersion retrieves a version by ID
func (s *TenantStore) GetMCPServerVersion(ctx context.Context, versionID string) (*domain.MCPServerVersion, error) {
	query := `
		SELECT id, server_id, version, commit_hash,
			tool_definitions, tool_count,
			changes, changes_summary, has_breaking_changes,
			created_at, created_by
		FROM mcp_server_versions
		WHERE id = $1
	`

	return s.scanMCPServerVersion(s.db.QueryRowContext(ctx, query, versionID))
}

// GetLatestMCPServerVersion retrieves the latest version for a server
func (s *TenantStore) GetLatestMCPServerVersion(ctx context.Context, serverID string) (*domain.MCPServerVersion, error) {
	query := `
		SELECT id, server_id, version, commit_hash,
			tool_definitions, tool_count,
			changes, changes_summary, has_breaking_changes,
			created_at, created_by
		FROM mcp_server_versions
		WHERE server_id = $1
		ORDER BY created_at DESC
		LIMIT 1
	`

	return s.scanMCPServerVersion(s.db.QueryRowContext(ctx, query, serverID))
}

// ListMCPServerVersions lists all versions for a server
func (s *TenantStore) ListMCPServerVersions(ctx context.Context, serverID string) ([]*domain.MCPServerVersion, error) {
	query := `
		SELECT id, server_id, version, commit_hash,
			tool_definitions, tool_count,
			changes, changes_summary, has_breaking_changes,
			created_at, created_by
		FROM mcp_server_versions
		WHERE server_id = $1
		ORDER BY created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []*domain.MCPServerVersion
	for rows.Next() {
		v, err := s.scanMCPServerVersionFromRows(rows)
		if err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}

	return versions, rows.Err()
}

func (s *TenantStore) scanMCPServerVersion(row *sql.Row) (*domain.MCPServerVersion, error) {
	var v domain.MCPServerVersion
	var toolDefs, changes []byte
	var commitHash, changesSummary, createdBy sql.NullString

	err := row.Scan(
		&v.ID, &v.ServerID, &v.Version, &commitHash,
		&toolDefs, &v.ToolCount,
		&changes, &changesSummary, &v.HasBreakingChanges,
		&v.CreatedAt, &createdBy,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal(toolDefs, &v.ToolDefinitions)
	_ = json.Unmarshal(changes, &v.Changes)

	if commitHash.Valid {
		v.CommitHash = commitHash.String
	}
	if changesSummary.Valid {
		v.ChangesSummary = changesSummary.String
	}
	if createdBy.Valid {
		v.CreatedBy = createdBy.String
	}

	return &v, nil
}

func (s *TenantStore) scanMCPServerVersionFromRows(rows *sql.Rows) (*domain.MCPServerVersion, error) {
	var v domain.MCPServerVersion
	var toolDefs, changes []byte
	var commitHash, changesSummary, createdBy sql.NullString

	err := rows.Scan(
		&v.ID, &v.ServerID, &v.Version, &commitHash,
		&toolDefs, &v.ToolCount,
		&changes, &changesSummary, &v.HasBreakingChanges,
		&v.CreatedAt, &createdBy,
	)
	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal(toolDefs, &v.ToolDefinitions)
	_ = json.Unmarshal(changes, &v.Changes)

	if commitHash.Valid {
		v.CommitHash = commitHash.String
	}
	if changesSummary.Valid {
		v.ChangesSummary = changesSummary.String
	}
	if createdBy.Valid {
		v.CreatedBy = createdBy.String
	}

	return &v, nil
}

// ============================================
// MCP PERMISSION OPERATIONS
// ============================================

// GetMCPToolPermission gets visibility permission for a tool
func (s *TenantStore) GetMCPToolPermission(ctx context.Context, roleID, toolID string) (*domain.MCPToolPermission, error) {
	query := `
		SELECT id, role_id, server_id, tool_id,
			visibility, decided_by, decided_by_email, decided_at, decision_reason,
			created_at, updated_at
		FROM mcp_tool_permissions
		WHERE role_id = $1 AND tool_id = $2
	`

	return s.scanMCPToolPermission(s.db.QueryRowContext(ctx, query, roleID, toolID))
}

// GetMCPToolVisibility returns the visibility state for a tool (defaults to DENY)
func (s *TenantStore) GetMCPToolVisibility(ctx context.Context, roleID, toolID string) domain.MCPToolVisibility {
	perm, err := s.GetMCPToolPermission(ctx, roleID, toolID)
	if err != nil || perm == nil {
		return domain.MCPVisibilityDeny // Default is DENY
	}
	return perm.Visibility
}

// SetMCPToolPermission sets or updates a tool visibility permission
func (s *TenantStore) SetMCPToolPermission(ctx context.Context, perm *domain.MCPToolPermission) error {
	if perm.ID == "" {
		perm.ID = uuid.New().String()
	}

	// Both server_id and tool_id are required
	if perm.ServerID == "" || perm.ToolID == "" {
		return fmt.Errorf("both server_id and tool_id are required")
	}

	// Require authenticated user for audit trail
	if perm.DecidedBy == "" {
		return fmt.Errorf("decided_by (user ID) is required for audit trail")
	}

	query := `
		INSERT INTO mcp_tool_permissions (
			id, role_id, server_id, tool_id, visibility,
			decided_by, decided_by_email, decided_at, decision_reason
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (role_id, tool_id) DO UPDATE SET
			visibility = EXCLUDED.visibility,
			decided_by = EXCLUDED.decided_by,
			decided_by_email = EXCLUDED.decided_by_email,
			decided_at = EXCLUDED.decided_at,
			decision_reason = EXCLUDED.decision_reason,
			updated_at = NOW()
	`

	_, err := s.db.ExecContext(ctx, query,
		perm.ID, perm.RoleID, perm.ServerID, perm.ToolID, perm.Visibility,
		perm.DecidedBy, perm.DecidedByEmail, perm.DecidedAt, perm.DecisionReason,
	)
	return err
}

// ListMCPPermissions lists all MCP permissions for a role
func (s *TenantStore) ListMCPPermissions(ctx context.Context, roleID string) ([]*domain.MCPToolPermission, error) {
	query := `
		SELECT id, role_id, server_id, tool_id,
			visibility, decided_by, decided_by_email, decided_at, decision_reason,
			created_at, updated_at
		FROM mcp_tool_permissions
		WHERE role_id = $1
		ORDER BY created_at
	`

	rows, err := s.db.QueryContext(ctx, query, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []*domain.MCPToolPermission
	for rows.Next() {
		p, err := s.scanMCPToolPermissionFromRows(rows)
		if err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}

	return perms, rows.Err()
}

// BulkSetMCPVisibility sets visibility for all tools in a server
func (s *TenantStore) BulkSetMCPVisibility(ctx context.Context, roleID, serverID string, visibility domain.MCPToolVisibility, actorID, actorEmail string) (int, error) {
	now := time.Now()

	// Get all tools for this server
	tools, err := s.ListMCPTools(ctx, serverID)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, tool := range tools {
		perm := &domain.MCPToolPermission{
			RoleID:         roleID,
			ServerID:       serverID,
			ToolID:         tool.ID,
			Visibility:     visibility,
			DecidedBy:      actorID,
			DecidedByEmail: actorEmail,
			DecidedAt:      &now,
		}
		if err := s.SetMCPToolPermission(ctx, perm); err == nil {
			count++
		}
	}

	return count, nil
}

func (s *TenantStore) scanMCPToolPermission(row *sql.Row) (*domain.MCPToolPermission, error) {
	var p domain.MCPToolPermission
	var decidedBy, decidedByEmail, decisionReason sql.NullString
	var decidedAt sql.NullTime

	err := row.Scan(
		&p.ID, &p.RoleID, &p.ServerID, &p.ToolID,
		&p.Visibility, &decidedBy, &decidedByEmail, &decidedAt, &decisionReason,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if decidedBy.Valid {
		p.DecidedBy = decidedBy.String
	}
	if decidedByEmail.Valid {
		p.DecidedByEmail = decidedByEmail.String
	}
	if decidedAt.Valid {
		p.DecidedAt = &decidedAt.Time
	}
	if decisionReason.Valid {
		p.DecisionReason = decisionReason.String
	}

	return &p, nil
}

func (s *TenantStore) scanMCPToolPermissionFromRows(rows *sql.Rows) (*domain.MCPToolPermission, error) {
	var p domain.MCPToolPermission
	var decidedBy, decidedByEmail, decisionReason sql.NullString
	var decidedAt sql.NullTime

	err := rows.Scan(
		&p.ID, &p.RoleID, &p.ServerID, &p.ToolID,
		&p.Visibility, &decidedBy, &decidedByEmail, &decidedAt, &decisionReason,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if decidedBy.Valid {
		p.DecidedBy = decidedBy.String
	}
	if decidedByEmail.Valid {
		p.DecidedByEmail = decidedByEmail.String
	}
	if decidedAt.Valid {
		p.DecidedAt = &decidedAt.Time
	}
	if decisionReason.Valid {
		p.DecisionReason = decisionReason.String
	}

	return &p, nil
}

// GetMCPToolsWithVisibility returns all tools for a server with their visibility for a role
func (s *TenantStore) GetMCPToolsWithVisibility(ctx context.Context, serverID, roleID string) ([]struct {
	Tool       *domain.MCPTool
	Visibility domain.MCPToolVisibility
	DecidedBy  string
	DecidedAt  *time.Time
}, error) {
	// Get all tools for the server
	tools, err := s.ListMCPTools(ctx, serverID)
	if err != nil {
		return nil, err
	}

	result := make([]struct {
		Tool       *domain.MCPTool
		Visibility domain.MCPToolVisibility
		DecidedBy  string
		DecidedAt  *time.Time
	}, len(tools))

	for i, tool := range tools {
		result[i].Tool = tool
		result[i].Visibility = domain.MCPVisibilityDeny // Default

		// Get permission if exists
		perm, err := s.GetMCPToolPermission(ctx, roleID, tool.ID)
		if err == nil && perm != nil {
			result[i].Visibility = perm.Visibility
			result[i].DecidedBy = perm.DecidedBy
			result[i].DecidedAt = perm.DecidedAt
		}
	}

	return result, nil
}

// ============================================
// MCP SEARCH OPERATIONS
// ============================================

// SearchToolsByVector performs semantic search using embeddings
// Uses in-memory cosine similarity computation (works without pgvector)
func (s *TenantStore) SearchToolsByVector(ctx context.Context, queryEmbedding []float32, limit int) ([]*domain.MCPTool, error) {
	if len(queryEmbedding) == 0 {
		return nil, fmt.Errorf("embedding is empty")
	}

	// Fetch all tools with embeddings
	query := `
		SELECT t.id, t.server_id, s.name as server_name,
			t.name, t.description, t.category,
			t.input_schema, t.output_schema, t.input_examples,
			t.defer_loading, t.is_deprecated, t.deprecation_message, t.deprecated_at,
			t.version, t.execution_count, t.last_executed_at, t.avg_execution_time_ms,
			t.created_at, t.updated_at, t.combined_embedding
		FROM mcp_tools t
		JOIN mcp_servers s ON t.server_id = s.id
		WHERE t.combined_embedding IS NOT NULL
			AND t.combined_embedding != ''
			AND t.is_deprecated = FALSE
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query tools: %w", err)
	}
	defer rows.Close()

	type toolWithScore struct {
		tool  *domain.MCPTool
		score float64
	}

	var toolsWithScores []toolWithScore

	for rows.Next() {
		var tool domain.MCPTool
		var serverName sql.NullString
		var category sql.NullString
		var inputSchemaBytes []byte
		var outputSchemaBytes []byte
		var inputExamplesBytes []byte
		var deprecationMessage sql.NullString
		var deprecatedAt sql.NullTime
		var version sql.NullString
		var executionCount sql.NullInt64
		var lastExecutedAt sql.NullTime
		var avgExecutionTimeMs sql.NullInt64
		var combinedEmbeddingStr sql.NullString

		err := rows.Scan(
			&tool.ID, &tool.ServerID, &serverName,
			&tool.Name, &tool.Description, &category,
			&inputSchemaBytes, &outputSchemaBytes, &inputExamplesBytes,
			&tool.DeferLoading, &tool.IsDeprecated, &deprecationMessage, &deprecatedAt,
			&version, &executionCount, &lastExecutedAt, &avgExecutionTimeMs,
			&tool.CreatedAt, &tool.UpdatedAt, &combinedEmbeddingStr,
		)
		if err != nil {
			continue
		}

		if serverName.Valid {
			tool.ServerName = serverName.String
		}
		if category.Valid {
			tool.Category = category.String
		}
		if len(inputSchemaBytes) > 0 {
			json.Unmarshal(inputSchemaBytes, &tool.InputSchema)
		}
		if len(outputSchemaBytes) > 0 {
			json.Unmarshal(outputSchemaBytes, &tool.OutputSchema)
		}
		if len(inputExamplesBytes) > 0 {
			json.Unmarshal(inputExamplesBytes, &tool.InputExamples)
		}
		if deprecationMessage.Valid {
			tool.DeprecationMessage = deprecationMessage.String
		}
		if deprecatedAt.Valid {
			tool.DeprecatedAt = &deprecatedAt.Time
		}
		if version.Valid {
			tool.Version = version.String
		}
		if executionCount.Valid {
			tool.ExecutionCount = executionCount.Int64
		}
		if lastExecutedAt.Valid {
			tool.LastExecutedAt = &lastExecutedAt.Time
		}
		if avgExecutionTimeMs.Valid {
			tool.AvgExecutionTimeMs = int(avgExecutionTimeMs.Int64)
		}

		// Compute similarity if embedding exists
		if combinedEmbeddingStr.Valid && combinedEmbeddingStr.String != "" {
			toolEmbedding := parseVector(combinedEmbeddingStr.String)
			if len(toolEmbedding) > 0 {
				score := cosineSimilarity(queryEmbedding, toolEmbedding)
				toolsWithScores = append(toolsWithScores, toolWithScore{
					tool:  &tool,
					score: score,
				})
			}
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tools: %w", err)
	}

	// Sort by similarity score descending
	sort.Slice(toolsWithScores, func(i, j int) bool {
		return toolsWithScores[i].score > toolsWithScores[j].score
	})

	// Return top results
	if limit > len(toolsWithScores) {
		limit = len(toolsWithScores)
	}

	result := make([]*domain.MCPTool, limit)
	for i := 0; i < limit; i++ {
		result[i] = toolsWithScores[i].tool
	}

	return result, nil
}

// SearchToolsByVectorWithScores performs semantic search and returns tools with their similarity scores
func (s *TenantStore) SearchToolsByVectorWithScores(ctx context.Context, queryEmbedding []float32, limit int) ([]*domain.MCPTool, []float64, error) {
	if len(queryEmbedding) == 0 {
		return nil, nil, fmt.Errorf("embedding is empty")
	}

	// Fetch all tools with embeddings
	query := `
		SELECT t.id, t.server_id, s.name as server_name,
			t.name, t.description, t.category,
			t.input_schema, t.output_schema, t.input_examples,
			t.defer_loading, t.is_deprecated, t.deprecation_message, t.deprecated_at,
			t.version, t.execution_count, t.last_executed_at, t.avg_execution_time_ms,
			t.created_at, t.updated_at, t.combined_embedding
		FROM mcp_tools t
		JOIN mcp_servers s ON t.server_id = s.id
		WHERE t.combined_embedding IS NOT NULL
			AND t.combined_embedding != ''
			AND t.is_deprecated = FALSE
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query tools: %w", err)
	}
	defer rows.Close()

	type toolWithScore struct {
		tool  *domain.MCPTool
		score float64
	}

	var toolsWithScores []toolWithScore

	for rows.Next() {
		var tool domain.MCPTool
		var serverName sql.NullString
		var category sql.NullString
		var inputSchemaBytes []byte
		var outputSchemaBytes []byte
		var inputExamplesBytes []byte
		var deprecationMessage sql.NullString
		var deprecatedAt sql.NullTime
		var version sql.NullString
		var executionCount sql.NullInt64
		var lastExecutedAt sql.NullTime
		var avgExecutionTimeMs sql.NullInt64
		var combinedEmbeddingStr sql.NullString

		err := rows.Scan(
			&tool.ID, &tool.ServerID, &serverName,
			&tool.Name, &tool.Description, &category,
			&inputSchemaBytes, &outputSchemaBytes, &inputExamplesBytes,
			&tool.DeferLoading, &tool.IsDeprecated, &deprecationMessage, &deprecatedAt,
			&version, &executionCount, &lastExecutedAt, &avgExecutionTimeMs,
			&tool.CreatedAt, &tool.UpdatedAt, &combinedEmbeddingStr,
		)
		if err != nil {
			continue
		}

		if serverName.Valid {
			tool.ServerName = serverName.String
		}
		if category.Valid {
			tool.Category = category.String
		}
		if len(inputSchemaBytes) > 0 {
			json.Unmarshal(inputSchemaBytes, &tool.InputSchema)
		}
		if len(outputSchemaBytes) > 0 {
			json.Unmarshal(outputSchemaBytes, &tool.OutputSchema)
		}
		if len(inputExamplesBytes) > 0 {
			json.Unmarshal(inputExamplesBytes, &tool.InputExamples)
		}
		if deprecationMessage.Valid {
			tool.DeprecationMessage = deprecationMessage.String
		}
		if deprecatedAt.Valid {
			tool.DeprecatedAt = &deprecatedAt.Time
		}
		if version.Valid {
			tool.Version = version.String
		}
		if executionCount.Valid {
			tool.ExecutionCount = executionCount.Int64
		}
		if lastExecutedAt.Valid {
			tool.LastExecutedAt = &lastExecutedAt.Time
		}
		if avgExecutionTimeMs.Valid {
			tool.AvgExecutionTimeMs = int(avgExecutionTimeMs.Int64)
		}

		// Compute similarity if embedding exists
		if combinedEmbeddingStr.Valid && combinedEmbeddingStr.String != "" {
			toolEmbedding := parseVector(combinedEmbeddingStr.String)
			if len(toolEmbedding) > 0 {
				score := cosineSimilarity(queryEmbedding, toolEmbedding)
				toolsWithScores = append(toolsWithScores, toolWithScore{
					tool:  &tool,
					score: score,
				})
			}
		}
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("error iterating tools: %w", err)
	}

	// Sort by similarity score descending
	sort.Slice(toolsWithScores, func(i, j int) bool {
		return toolsWithScores[i].score > toolsWithScores[j].score
	})

	// Return top results with scores
	if limit > len(toolsWithScores) {
		limit = len(toolsWithScores)
	}

	tools := make([]*domain.MCPTool, limit)
	scores := make([]float64, limit)
	for i := 0; i < limit; i++ {
		tools[i] = toolsWithScores[i].tool
		scores[i] = toolsWithScores[i].score
	}

	return tools, scores, nil
}

// SearchToolsByText performs full-text search
func (s *TenantStore) SearchToolsByText(ctx context.Context, query string, limit int) ([]*domain.MCPTool, error) {
	sqlQuery := `
		SELECT t.id, t.server_id, s.name as server_name,
			t.name, t.description, t.category,
			t.input_schema, t.output_schema, t.input_examples,
			t.defer_loading, t.is_deprecated, t.deprecation_message, t.deprecated_at,
			t.version, t.execution_count, t.last_executed_at, t.avg_execution_time_ms,
			t.created_at, t.updated_at
		FROM mcp_tools t
		JOIN mcp_servers s ON t.server_id = s.id
		WHERE t.is_deprecated = FALSE
			AND to_tsvector('english', t.name || ' ' || COALESCE(t.description, '')) @@ plainto_tsquery('english', $1)
		ORDER BY ts_rank(to_tsvector('english', t.name || ' ' || COALESCE(t.description, '')), plainto_tsquery('english', $1)) DESC
		LIMIT $2
	`

	return s.queryMCPTools(ctx, sqlQuery, query, limit)
}

// ============================================
// MCP EXECUTION LOGGING
// ============================================

// LogMCPToolExecution logs a tool execution
func (s *TenantStore) LogMCPToolExecution(ctx context.Context, exec *domain.MCPToolExecution) error {
	if exec.ID == "" {
		exec.ID = uuid.New().String()
	}

	inputParams, _ := json.Marshal(exec.InputParams)
	outputResult, _ := json.Marshal(exec.OutputResult)

	query := `
		INSERT INTO mcp_tool_executions (
			id, server_id, tool_id,
			role_id, api_key_id, request_id,
			input_params, output_result, status, error_message,
			started_at, completed_at, duration_ms
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`

	_, err := s.db.ExecContext(ctx, query,
		exec.ID, exec.ServerID, exec.ToolID,
		exec.RoleID, exec.APIKeyID, exec.RequestID,
		inputParams, outputResult, exec.Status, exec.ErrorMessage,
		exec.StartedAt, exec.CompletedAt, exec.DurationMs,
	)

	// Update tool execution stats
	if err == nil {
		s.updateToolExecutionStats(ctx, exec.ToolID, exec.DurationMs)
	}

	return err
}

func (s *TenantStore) updateToolExecutionStats(ctx context.Context, toolID string, durationMs int) {
	query := `
		UPDATE mcp_tools SET
			execution_count = execution_count + 1,
			last_executed_at = NOW(),
			avg_execution_time_ms = (
				COALESCE(avg_execution_time_ms, 0) * execution_count + $2
			) / (execution_count + 1)
		WHERE id = $1
	`
	s.db.ExecContext(ctx, query, toolID, durationMs)
}

// ListMCPToolExecutions lists tool executions
func (s *TenantStore) ListMCPToolExecutions(ctx context.Context, limit, offset int) ([]*domain.MCPToolExecution, int, error) {
	countQuery := "SELECT COUNT(*) FROM mcp_tool_executions"
	var total int
	s.db.QueryRowContext(ctx, countQuery).Scan(&total)

	query := `
		SELECT id, server_id, tool_id,
			role_id, api_key_id, request_id,
			input_params, output_result, status, error_message,
			started_at, completed_at, duration_ms, created_at
		FROM mcp_tool_executions
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := s.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var execs []*domain.MCPToolExecution
	for rows.Next() {
		var e domain.MCPToolExecution
		var inputParams, outputResult []byte
		var roleID, apiKeyID, requestID, errorMessage sql.NullString
		var completedAt sql.NullTime

		err := rows.Scan(
			&e.ID, &e.ServerID, &e.ToolID,
			&roleID, &apiKeyID, &requestID,
			&inputParams, &outputResult, &e.Status, &errorMessage,
			&e.StartedAt, &completedAt, &e.DurationMs, &e.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}

		_ = json.Unmarshal(inputParams, &e.InputParams)
		_ = json.Unmarshal(outputResult, &e.OutputResult)

		if roleID.Valid {
			e.RoleID = roleID.String
		}
		if apiKeyID.Valid {
			e.APIKeyID = apiKeyID.String
		}
		if requestID.Valid {
			e.RequestID = requestID.String
		}
		if errorMessage.Valid {
			e.ErrorMessage = errorMessage.String
		}
		if completedAt.Valid {
			e.CompletedAt = &completedAt.Time
		}

		execs = append(execs, &e)
	}

	return execs, total, rows.Err()
}
