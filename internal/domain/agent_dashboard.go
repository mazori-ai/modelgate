package domain

import (
	"context"
	"time"
)

// =============================================================================
// Agent Dashboard Types
// =============================================================================

// AgentDashboardStats contains comprehensive agent statistics
type AgentDashboardStats struct {
	APIKeyID   string    `json:"api_key_id"`
	APIKeyName string    `json:"api_key_name"`
	TimeRange  TimeRange `json:"time_range"`

	// Provider and Model Usage
	ProviderUsage []ProviderModelUsage `json:"provider_usage"`

	// Token Metrics
	TokenMetrics TokenMetrics `json:"token_metrics"`

	// Cache Usage
	CacheStats CacheStatistics `json:"cache_stats"`

	// Tool Calls
	ToolCallStats []ToolCallStatistic `json:"tool_call_stats"`

	// Policy Violations
	Violations []PolicyViolationStat `json:"violations"`

	// Risk Assessment
	RiskScore RiskAssessment `json:"risk_score"`
}

// TimeRange represents a time range for querying
type TimeRange struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

// ProviderModelUsage contains usage statistics for a provider/model combination
type ProviderModelUsage struct {
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	RequestCount int64   `json:"request_count"`
	TokenCount   int64   `json:"token_count"`
	CostUSD      float64 `json:"cost_usd"`
}

// TokenMetrics contains token usage metrics
type TokenMetrics struct {
	TotalInput    int64                     `json:"total_input"`
	TotalOutput   int64                     `json:"total_output"`
	TotalThinking int64                     `json:"total_thinking"`
	TotalCost     float64                   `json:"total_cost_usd"`
	ByModel       map[string]TokenBreakdown `json:"by_model"`
}

// TokenBreakdown contains token breakdown for a specific model
type TokenBreakdown struct {
	InputTokens    int64   `json:"input_tokens"`
	OutputTokens   int64   `json:"output_tokens"`
	ThinkingTokens int64   `json:"thinking_tokens"`
	CostUSD        float64 `json:"cost_usd"`
}

// CacheStatistics contains cache usage statistics
type CacheStatistics struct {
	TotalHits    int64   `json:"total_hits"`
	TotalMisses  int64   `json:"total_misses"`
	HitRate      float64 `json:"hit_rate"` // Percentage (0-100)
	TokensSaved  int64   `json:"tokens_saved"`
	CostSavedUSD float64 `json:"cost_saved_usd"`
}

// ToolCallStatistic contains statistics for a specific tool
type ToolCallStatistic struct {
	ToolName     string  `json:"tool_name"`
	SuccessCount int64   `json:"success_count"`
	FailureCount int64   `json:"failure_count"`
	TotalCount   int64   `json:"total_count"`
	SuccessRate  float64 `json:"success_rate"` // Percentage (0-100)
}

// PolicyViolationStat contains statistics for policy violations by type
type PolicyViolationStat struct {
	ViolationType string  `json:"violation_type"`
	Count         int64   `json:"count"`
	AvgSeverity   float64 `json:"avg_severity"` // Average severity (1-5)
}

// RiskAssessment contains risk assessment information
type RiskAssessment struct {
	Score      float64            `json:"score"` // 0-100
	Level      string             `json:"level"` // low, medium, high, critical
	Violations int64              `json:"total_violations"`
	Details    map[string]float64 `json:"details"` // breakdown by violation type
}

// =============================================================================
// Event Types for Recording (Dashboard-specific, distinct from streaming events)
// =============================================================================

// PolicyViolationRecord represents a policy violation event to be recorded in the database
type PolicyViolationRecord struct {
	ID            string         `json:"id"`
	APIKeyID      string         `json:"api_key_id,omitempty"`
	PolicyID      string         `json:"policy_id"`
	PolicyName    string         `json:"policy_name"`
	ViolationType string         `json:"violation_type"`
	Severity      int            `json:"severity"` // 1=low, 2=medium, 3=high, 4=critical, 5=severe
	Message       string         `json:"message,omitempty"`
	Timestamp     time.Time      `json:"timestamp"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

// ToolCallRecord represents a tool call event to be recorded in the database
type ToolCallRecord struct {
	ID           string    `json:"id"`
	APIKeyID     string    `json:"api_key_id,omitempty"`
	ToolName     string    `json:"tool_name"`
	Model        string    `json:"model"`
	Provider     string    `json:"provider"`
	Success      bool      `json:"success"`
	ErrorMessage string    `json:"error_message,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

// CacheEventRecord represents a cache hit/miss event to be recorded in the database
type CacheEventRecord struct {
	ID           string    `json:"id"`
	APIKeyID     string    `json:"api_key_id,omitempty"`
	Model        string    `json:"model"`
	Hit          bool      `json:"hit"`
	TokensSaved  int64     `json:"tokens_saved"`
	CostSavedUSD float64   `json:"cost_saved_usd"`
	Timestamp    time.Time `json:"timestamp"`
}

// =============================================================================
// Repository Interfaces
// =============================================================================

// AgentDashboardRepository handles agent dashboard data storage and retrieval
type AgentDashboardRepository interface {
	// GetStats retrieves comprehensive dashboard statistics for an agent
	GetStats(ctx context.Context, tenantID, apiKeyID string, startTime, endTime time.Time) (*AgentDashboardStats, error)

	// RecordPolicyViolation records a policy violation event
	RecordPolicyViolation(ctx context.Context, event *PolicyViolationRecord) error

	// RecordToolCall records a tool call event
	RecordToolCall(ctx context.Context, event *ToolCallRecord) error

	// RecordCacheEvent records a cache hit/miss event
	RecordCacheEvent(ctx context.Context, event *CacheEventRecord) error

	// GetPolicyViolations retrieves policy violations for a time range
	GetPolicyViolations(ctx context.Context, tenantID, apiKeyID string, startTime, endTime time.Time) ([]PolicyViolationRecord, error)

	// CleanupOldData removes data older than the retention period (30 days)
	CleanupOldData(ctx context.Context) error
}
