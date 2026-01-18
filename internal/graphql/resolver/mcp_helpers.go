package resolver

import (
	"strings"

	"modelgate/internal/domain"
	"modelgate/internal/graphql/model"
)

// graphqlToMCPServerType converts GraphQL enum to domain type
func graphqlToMCPServerType(t model.MCPServerType) domain.MCPServerType {
	return domain.MCPServerType(strings.ToLower(string(t)))
}

// graphqlToMCPAuthType converts GraphQL enum to domain type
func graphqlToMCPAuthType(t model.MCPAuthType) domain.MCPAuthType {
	return domain.MCPAuthType(strings.ToLower(string(t)))
}

// Helper functions for MCP resolvers

func ptrToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func ptrToBool(b *bool, def bool) bool {
	if b == nil {
		return def
	}
	return *b
}

func ptrToInt(i *int, def int) int {
	if i == nil {
		return def
	}
	return *i
}

func inputToMCPAuthConfig(input *model.MCPAuthConfigInput) domain.MCPAuthConfig {
	if input == nil {
		return domain.MCPAuthConfig{}
	}

	config := domain.MCPAuthConfig{}

	// Skip masked values (***) - these are placeholders from masked JSON output
	// Only set values that are not the mask placeholder
	if input.APIKey != nil && *input.APIKey != "***" {
		config.APIKey = *input.APIKey
	}
	if input.APIKeyHeader != nil {
		config.APIKeyHeader = *input.APIKeyHeader
	}
	if input.BearerToken != nil && *input.BearerToken != "***" {
		config.BearerToken = *input.BearerToken
	}
	if input.ClientID != nil {
		config.ClientID = *input.ClientID
	}
	if input.ClientSecret != nil && *input.ClientSecret != "***" {
		config.ClientSecret = *input.ClientSecret
	}
	if input.TokenURL != nil {
		config.TokenURL = *input.TokenURL
	}
	if input.Scopes != nil {
		config.Scopes = input.Scopes
	}
	if input.Username != nil {
		config.Username = *input.Username
	}
	if input.Password != nil && *input.Password != "***" {
		config.Password = *input.Password
	}
	if input.ClientCert != nil {
		config.ClientCert = *input.ClientCert
	}
	if input.ClientKey != nil && *input.ClientKey != "***" {
		config.ClientKey = *input.ClientKey
	}
	if input.CaCert != nil {
		config.CACert = *input.CaCert
	}
	if input.AWSRegion != nil {
		config.AWSRegion = *input.AWSRegion
	}
	if input.AWSRoleArn != nil {
		config.AWSRoleARN = *input.AWSRoleArn
	}

	return config
}

func domainToMCPServerModel(s *domain.MCPServer) *model.MCPServer {
	return &model.MCPServer{
		ID:          s.ID,
		Name:        s.Name,
		Description: &s.Description,
		ServerType:          model.MCPServerType(s.ServerType),
		Endpoint:            s.Endpoint,
		AuthType:            model.MCPAuthType(s.AuthType),
		Version:             &s.Version,
		Status:              model.MCPServerStatus(s.Status),
		LastHealthCheck:     s.LastHealthCheck,
		LastSyncAt:          s.LastSyncAt,
		ErrorMessage:        &s.ErrorMessage,
		AutoSync:            s.AutoSync,
		SyncIntervalMinutes: s.SyncIntervalMinutes,
		ToolCount:           s.ToolCount,
		CreatedAt:           s.CreatedAt,
		UpdatedAt:           s.UpdatedAt,
		CreatedBy:           &s.CreatedBy,
	}
}

func domainToMCPToolModel(t *domain.MCPTool) *model.MCPTool {
	return &model.MCPTool{
		ID:                 t.ID,
		ServerID:           t.ServerID,
		ServerName:         t.ServerName,
		Name:               t.Name,
		Description:        &t.Description,
		Category:           &t.Category,
		InputSchema:        t.InputSchema,
		DeferLoading:       t.DeferLoading,
		IsDeprecated:       t.IsDeprecated,
		DeprecationMessage: &t.DeprecationMessage,
		ExecutionCount:     int(t.ExecutionCount),
		CreatedAt:          t.CreatedAt,
		UpdatedAt:          t.UpdatedAt,
	}
}

func domainToMCPServerVersionModel(v *domain.MCPServerVersion) *model.MCPServerVersion {
	return &model.MCPServerVersion{
		ID:                 v.ID,
		ServerID:           v.ServerID,
		Version:            v.Version,
		CommitHash:         &v.CommitHash,
		ToolCount:          v.ToolCount,
		ChangesSummary:     &v.ChangesSummary,
		HasBreakingChanges: v.HasBreakingChanges,
		CreatedAt:          v.CreatedAt,
		CreatedBy:          &v.CreatedBy,
	}
}

func domainToMCPToolPermissionModel(p *domain.MCPToolPermission) *model.MCPToolPermission {
	m := &model.MCPToolPermission{
		ID:         p.ID,
		RoleID:     p.RoleID,
		ServerID:   p.ServerID,
		ToolID:     p.ToolID,
		Visibility: model.MCPToolVisibility(p.Visibility),
	}

	if p.DecidedBy != "" {
		m.DecidedBy = &p.DecidedBy
	}
	if p.DecidedByEmail != "" {
		m.DecidedByEmail = &p.DecidedByEmail
	}
	if p.DecidedAt != nil {
		m.DecidedAt = p.DecidedAt
	}
	if p.DecisionReason != "" {
		m.DecisionReason = &p.DecisionReason
	}

	return m
}

func domainToMCPToolExecutionModel(e *domain.MCPToolExecution) model.MCPToolExecution {
	m := model.MCPToolExecution{
		ID:         e.ID,
		ServerID:   e.ServerID,
		ToolID:     e.ToolID,
		Status:     string(e.Status),
		StartedAt:  e.StartedAt,
		DurationMs: &e.DurationMs,
	}

	if e.RoleID != "" {
		m.RoleID = &e.RoleID
	}
	if e.RequestID != "" {
		m.RequestID = &e.RequestID
	}
	if e.InputParams != nil {
		m.InputParams = e.InputParams
	}
	if e.OutputResult != nil {
		m.OutputResult = e.OutputResult
	}
	if e.ErrorMessage != "" {
		m.ErrorMessage = &e.ErrorMessage
	}
	if e.CompletedAt != nil {
		m.CompletedAt = e.CompletedAt
	}

	return m
}

func domainToToolSearchResponse(r *domain.ToolSearchResponse) *model.ToolSearchResponse {
	tools := make([]model.ToolSearchResult, len(r.Tools))
	for i, t := range r.Tools {
		tools[i] = model.ToolSearchResult{
			Tool:         domainToMCPToolModel(t.Tool),
			ServerID:     t.ServerID,
			ServerName:   t.ServerName,
			Score:        t.Score,
			DeferLoading: t.DeferLoading,
			ToolRef:      t.ToolRef,
		}
		if t.MatchReason != "" {
			tools[i].MatchReason = &t.MatchReason
		}
	}

	return &model.ToolSearchResponse{
		Tools:          tools,
		Query:          r.Query,
		TotalAvailable: r.TotalAvailable,
		TotalAllowed:   r.TotalAllowed,
	}
}

// Model types for MCP
type MCPServerModel = model.MCPServer
type MCPToolModel = model.MCPTool
type MCPServerVersionModel = model.MCPServerVersion
type MCPToolPermissionModel = model.MCPToolPermission
type MCPToolExecutionModel = model.MCPToolExecution
type MCPServerWithToolsModel = model.MCPServerWithTools
type MCPToolWithVisibilityModel = model.MCPToolWithVisibility
type MCPServerToolStatsModel = model.MCPServerToolStats
