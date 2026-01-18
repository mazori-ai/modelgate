package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"modelgate/internal/analytics"
	"modelgate/internal/domain"
)

// AgentDashboardStore handles agent dashboard data storage and retrieval
type AgentDashboardStore struct {
	db *DB
}

// NewAgentDashboardStore creates a new agent dashboard store
func NewAgentDashboardStore(db *DB) *AgentDashboardStore {
	return &AgentDashboardStore{db: db}
}

// GetStats retrieves comprehensive dashboard statistics for an agent
func (s *AgentDashboardStore) GetStats(ctx context.Context, tenantID, apiKeyID string, startTime, endTime time.Time) (*domain.AgentDashboardStats, error) {
	stats := &domain.AgentDashboardStats{
		APIKeyID: apiKeyID,
		TimeRange: domain.TimeRange{
			StartTime: startTime,
			EndTime:   endTime,
		},
	}

	// Get API key name
	apiKeyName, err := s.getAPIKeyName(ctx, apiKeyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get API key name: %w", err)
	}
	stats.APIKeyName = apiKeyName

	// Get provider/model usage
	providerUsage, err := s.getProviderModelUsage(ctx, apiKeyID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider usage: %w", err)
	}
	stats.ProviderUsage = providerUsage

	// Get token metrics
	tokenMetrics, err := s.getTokenMetrics(ctx, apiKeyID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get token metrics: %w", err)
	}
	stats.TokenMetrics = tokenMetrics

	// Get cache statistics
	cacheStats, err := s.getCacheStatistics(ctx, apiKeyID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get cache statistics: %w", err)
	}
	stats.CacheStats = cacheStats

	// Get tool call statistics
	toolCallStats, err := s.getToolCallStatistics(ctx, apiKeyID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get tool call statistics: %w", err)
	}
	stats.ToolCallStats = toolCallStats

	// Get policy violations
	violations, err := s.getPolicyViolationStats(ctx, apiKeyID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get policy violations: %w", err)
	}
	stats.Violations = violations

	// Calculate risk score
	violationEvents, err := s.GetPolicyViolations(ctx, apiKeyID, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get violation events for risk calculation: %w", err)
	}
	stats.RiskScore = analytics.CalculateRiskScore(violationEvents)

	return stats, nil
}

// getAPIKeyName retrieves the name of an API key
func (s *AgentDashboardStore) getAPIKeyName(ctx context.Context, apiKeyID string) (string, error) {
	if apiKeyID == "" {
		return "All API Keys", nil
	}

	var name string
	query := `SELECT name FROM api_keys WHERE id = $1`
	err := s.db.QueryRowContext(ctx, query, apiKeyID).Scan(&name)
	if err == sql.ErrNoRows {
		return "Unknown", nil
	}
	if err != nil {
		return "", err
	}
	return name, nil
}

// getProviderModelUsage retrieves provider and model usage statistics
func (s *AgentDashboardStore) getProviderModelUsage(ctx context.Context, apiKeyID string, startTime, endTime time.Time) ([]domain.ProviderModelUsage, error) {
	query := `
		SELECT
			provider,
			model,
			COUNT(*) as request_count,
			COALESCE(SUM(input_tokens + output_tokens + COALESCE(thinking_tokens, 0)), 0) as token_count,
			COALESCE(SUM(cost_usd), 0) as cost_usd
		FROM usage_records
		WHERE created_at >= $1
			AND created_at <= $2
			AND ($3::uuid IS NULL OR api_key_id = $3::uuid)
		GROUP BY provider, model
		ORDER BY request_count DESC
	`

	var apiKeyUUID *uuid.UUID
	if apiKeyID != "" {
		parsed, err := uuid.Parse(apiKeyID)
		if err == nil {
			apiKeyUUID = &parsed
		}
	}

	rows, err := s.db.QueryContext(ctx, query, startTime, endTime, apiKeyUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var usage []domain.ProviderModelUsage
	for rows.Next() {
		var u domain.ProviderModelUsage
		if err := rows.Scan(&u.Provider, &u.Model, &u.RequestCount, &u.TokenCount, &u.CostUSD); err != nil {
			return nil, err
		}
		usage = append(usage, u)
	}

	return usage, rows.Err()
}

// getTokenMetrics retrieves token usage metrics
func (s *AgentDashboardStore) getTokenMetrics(ctx context.Context, apiKeyID string, startTime, endTime time.Time) (domain.TokenMetrics, error) {
	query := `
		SELECT
			COALESCE(SUM(input_tokens), 0) as total_input,
			COALESCE(SUM(output_tokens), 0) as total_output,
			COALESCE(SUM(thinking_tokens), 0) as total_thinking,
			COALESCE(SUM(cost_usd), 0) as total_cost
		FROM usage_records
		WHERE created_at >= $1
			AND created_at <= $2
			AND ($3::uuid IS NULL OR api_key_id = $3::uuid)
	`

	var apiKeyUUID *uuid.UUID
	if apiKeyID != "" {
		parsed, err := uuid.Parse(apiKeyID)
		if err == nil {
			apiKeyUUID = &parsed
		}
	}

	var metrics domain.TokenMetrics
	err := s.db.QueryRowContext(ctx, query, startTime, endTime, apiKeyUUID).Scan(
		&metrics.TotalInput,
		&metrics.TotalOutput,
		&metrics.TotalThinking,
		&metrics.TotalCost,
	)
	if err != nil {
		return metrics, err
	}

	// Get per-model breakdown
	byModelQuery := `
		SELECT
			model,
			COALESCE(SUM(input_tokens), 0) as input_tokens,
			COALESCE(SUM(output_tokens), 0) as output_tokens,
			COALESCE(SUM(thinking_tokens), 0) as thinking_tokens,
			COALESCE(SUM(cost_usd), 0) as cost_usd
		FROM usage_records
		WHERE created_at >= $1
			AND created_at <= $2
			AND ($3::uuid IS NULL OR api_key_id = $3::uuid)
		GROUP BY model
	`

	rows, err := s.db.QueryContext(ctx, byModelQuery, startTime, endTime, apiKeyUUID)
	if err != nil {
		return metrics, err
	}
	defer rows.Close()

	metrics.ByModel = make(map[string]domain.TokenBreakdown)
	for rows.Next() {
		var model string
		var breakdown domain.TokenBreakdown
		if err := rows.Scan(&model, &breakdown.InputTokens, &breakdown.OutputTokens, &breakdown.ThinkingTokens, &breakdown.CostUSD); err != nil {
			return metrics, err
		}
		metrics.ByModel[model] = breakdown
	}

	return metrics, rows.Err()
}

// getCacheStatistics retrieves cache usage statistics
func (s *AgentDashboardStore) getCacheStatistics(ctx context.Context, apiKeyID string, startTime, endTime time.Time) (domain.CacheStatistics, error) {
	query := `
		SELECT
			COUNT(*) FILTER (WHERE hit = true) as total_hits,
			COUNT(*) FILTER (WHERE hit = false) as total_misses,
			COALESCE(SUM(tokens_saved), 0) as tokens_saved,
			COALESCE(SUM(cost_saved_usd), 0) as cost_saved
		FROM cache_events
		WHERE timestamp >= $1
			AND timestamp <= $2
			AND ($3::uuid IS NULL OR api_key_id = $3::uuid)
	`

	var apiKeyUUID *uuid.UUID
	if apiKeyID != "" {
		parsed, err := uuid.Parse(apiKeyID)
		if err == nil {
			apiKeyUUID = &parsed
		}
	}

	var stats domain.CacheStatistics
	err := s.db.QueryRowContext(ctx, query, startTime, endTime, apiKeyUUID).Scan(
		&stats.TotalHits,
		&stats.TotalMisses,
		&stats.TokensSaved,
		&stats.CostSavedUSD,
	)
	if err != nil {
		return stats, err
	}

	// Calculate hit rate
	totalRequests := stats.TotalHits + stats.TotalMisses
	if totalRequests > 0 {
		stats.HitRate = float64(stats.TotalHits) / float64(totalRequests) * 100
	}

	return stats, nil
}

// getToolCallStatistics retrieves tool call statistics
func (s *AgentDashboardStore) getToolCallStatistics(ctx context.Context, apiKeyID string, startTime, endTime time.Time) ([]domain.ToolCallStatistic, error) {
	query := `
		SELECT
			tool_name,
			COUNT(*) FILTER (WHERE success = true) as success_count,
			COUNT(*) FILTER (WHERE success = false) as failure_count,
			COUNT(*) as total_count
		FROM tool_call_events
		WHERE timestamp >= $1
			AND timestamp <= $2
			AND ($3::uuid IS NULL OR api_key_id = $3::uuid)
		GROUP BY tool_name
		ORDER BY total_count DESC
	`

	var apiKeyUUID *uuid.UUID
	if apiKeyID != "" {
		parsed, err := uuid.Parse(apiKeyID)
		if err == nil {
			apiKeyUUID = &parsed
		}
	}

	rows, err := s.db.QueryContext(ctx, query, startTime, endTime, apiKeyUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []domain.ToolCallStatistic
	for rows.Next() {
		var s domain.ToolCallStatistic
		if err := rows.Scan(&s.ToolName, &s.SuccessCount, &s.FailureCount, &s.TotalCount); err != nil {
			return nil, err
		}
		// Calculate success rate
		if s.TotalCount > 0 {
			s.SuccessRate = float64(s.SuccessCount) / float64(s.TotalCount) * 100
		}
		stats = append(stats, s)
	}

	return stats, rows.Err()
}

// getPolicyViolationStats retrieves policy violation statistics
func (s *AgentDashboardStore) getPolicyViolationStats(ctx context.Context, apiKeyID string, startTime, endTime time.Time) ([]domain.PolicyViolationStat, error) {
	query := `
		SELECT
			violation_type,
			COUNT(*) as count,
			AVG(severity) as avg_severity
		FROM policy_violation_events
		WHERE timestamp >= $1
			AND timestamp <= $2
			AND ($3::uuid IS NULL OR api_key_id = $3::uuid)
		GROUP BY violation_type
		ORDER BY count DESC
	`

	var apiKeyUUID *uuid.UUID
	if apiKeyID != "" {
		parsed, err := uuid.Parse(apiKeyID)
		if err == nil {
			apiKeyUUID = &parsed
		}
	}

	rows, err := s.db.QueryContext(ctx, query, startTime, endTime, apiKeyUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []domain.PolicyViolationStat
	for rows.Next() {
		var s domain.PolicyViolationStat
		if err := rows.Scan(&s.ViolationType, &s.Count, &s.AvgSeverity); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}

	return stats, rows.Err()
}

// RecordPolicyViolation records a policy violation event
func (s *AgentDashboardStore) RecordPolicyViolation(ctx context.Context, event *domain.PolicyViolationRecord) error {
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	var metadataJSON []byte
	var err error
	if event.Metadata != nil {
		metadataJSON, err = json.Marshal(event.Metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
	}

	var apiKeyUUID *uuid.UUID
	if event.APIKeyID != "" {
		parsed, err := uuid.Parse(event.APIKeyID)
		if err == nil {
			apiKeyUUID = &parsed
		}
	}

	query := `
		INSERT INTO policy_violation_events (
			id, api_key_id, policy_id, policy_name,
			violation_type, severity, message, timestamp, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err = s.db.ExecContext(ctx, query,
		event.ID,
		apiKeyUUID,
		event.PolicyID,
		event.PolicyName,
		event.ViolationType,
		event.Severity,
		event.Message,
		event.Timestamp,
		metadataJSON,
	)

	return err
}

// RecordToolCall records a tool call event
func (s *AgentDashboardStore) RecordToolCall(ctx context.Context, event *domain.ToolCallRecord) error {
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	var apiKeyUUID *uuid.UUID
	if event.APIKeyID != "" {
		parsed, err := uuid.Parse(event.APIKeyID)
		if err == nil {
			apiKeyUUID = &parsed
		}
	}

	query := `
		INSERT INTO tool_call_events (
			id, api_key_id, tool_name, model, provider,
			success, error_message, timestamp
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := s.db.ExecContext(ctx, query,
		event.ID,
		apiKeyUUID,
		event.ToolName,
		event.Model,
		event.Provider,
		event.Success,
		event.ErrorMessage,
		event.Timestamp,
	)

	return err
}

// RecordCacheEvent records a cache hit/miss event
func (s *AgentDashboardStore) RecordCacheEvent(ctx context.Context, event *domain.CacheEventRecord) error {
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	var apiKeyUUID *uuid.UUID
	if event.APIKeyID != "" {
		parsed, err := uuid.Parse(event.APIKeyID)
		if err == nil {
			apiKeyUUID = &parsed
		}
	}

	query := `
		INSERT INTO cache_events (
			id, api_key_id, model, hit,
			tokens_saved, cost_saved_usd, timestamp
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := s.db.ExecContext(ctx, query,
		event.ID,
		apiKeyUUID,
		event.Model,
		event.Hit,
		event.TokensSaved,
		event.CostSavedUSD,
		event.Timestamp,
	)

	return err
}

// GetPolicyViolations retrieves policy violations for a time range
func (s *AgentDashboardStore) GetPolicyViolations(ctx context.Context, apiKeyID string, startTime, endTime time.Time) ([]domain.PolicyViolationRecord, error) {
	query := `
		SELECT
			id, COALESCE(api_key_id::text, ''), policy_id, policy_name,
			violation_type, severity, COALESCE(message, ''), timestamp, COALESCE(metadata, '{}')
		FROM policy_violation_events
		WHERE timestamp >= $1
			AND timestamp <= $2
			AND ($3::uuid IS NULL OR api_key_id = $3::uuid)
		ORDER BY timestamp DESC
	`

	var apiKeyUUID *uuid.UUID
	if apiKeyID != "" {
		parsed, err := uuid.Parse(apiKeyID)
		if err == nil {
			apiKeyUUID = &parsed
		}
	}

	rows, err := s.db.QueryContext(ctx, query, startTime, endTime, apiKeyUUID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []domain.PolicyViolationRecord
	for rows.Next() {
		var event domain.PolicyViolationRecord
		var metadataJSON []byte

		if err := rows.Scan(
			&event.ID,
			&event.APIKeyID,
			&event.PolicyID,
			&event.PolicyName,
			&event.ViolationType,
			&event.Severity,
			&event.Message,
			&event.Timestamp,
			&metadataJSON,
		); err != nil {
			return nil, err
		}

		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &event.Metadata); err != nil {
				// Log error but continue
				continue
			}
		}

		events = append(events, event)
	}

	return events, rows.Err()
}

// CleanupOldData removes data older than the retention period (30 days)
func (s *AgentDashboardStore) CleanupOldData(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "SELECT cleanup_old_dashboard_data()")
	return err
}
