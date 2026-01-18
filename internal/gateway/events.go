package gateway

import (
	"context"
	"log/slog"

	"modelgate/internal/domain"
)

// recordPolicyViolationEvent records a policy violation to both metrics and database
func (s *Service) recordPolicyViolationEvent(ctx context.Context, tenantID, apiKeyID, policyID, policyName, violationType string, severity int, message string) {
	// Record to Prometheus metrics
	if s.metrics != nil {
		severityStr := getSeverityString(severity)
		s.metrics.RecordPolicyViolation(policyID, violationType, severityStr)
	}

	// Persist to PostgreSQL
	if s.pgStore != nil {
		event := &domain.PolicyViolationRecord{
			APIKeyID:   apiKeyID,
			PolicyID:   policyID,
			PolicyName: policyName,
			ViolationType: violationType,
			Severity:      severity,
			Message:       message,
			Metadata:      make(map[string]interface{}), // Initialize empty metadata to avoid JSON parsing errors
		}

		// Get tenant slug from tenant ID
		tenantSlug := s.getTenantSlug(ctx, tenantID)
		if tenantSlug == "" {
			slog.Warn("Failed to get tenant slug for policy violation event", "tenant_id", tenantID)
			return
		}

		tenantStore, err := s.pgStore.GetTenantStore(tenantSlug)
		if err != nil {
			slog.Warn("Failed to get tenant store for policy violation event", "error", err)
			return
		}

		dashboardStore := tenantStore.AgentDashboardStore()
		if err := dashboardStore.RecordPolicyViolation(ctx, event); err != nil {
			slog.Warn("Failed to record policy violation to database", "error", err)
		}
	}
}

// recordToolCallEvent records a tool call event to both metrics and database
func (s *Service) recordToolCallEvent(ctx context.Context, tenantID, apiKeyID, toolName, model, provider string, success bool, errorMessage string) {
	// Record to Prometheus metrics
	if s.metrics != nil {
		s.metrics.RecordToolCall(toolName, model, tenantID)
	}

	// Persist to PostgreSQL
	if s.pgStore == nil {
		return
	}

	event := &domain.ToolCallRecord{
		APIKeyID: apiKeyID,
		ToolName: toolName,
		Model:    model,
		Provider:     provider,
		Success:      success,
		ErrorMessage: errorMessage,
	}

	// Get tenant slug from tenant ID
	tenantSlug := s.getTenantSlug(ctx, tenantID)
	if tenantSlug == "" {
		slog.Warn("Failed to get tenant slug for tool call event", "tenant_id", tenantID)
		return
	}

	tenantStore, err := s.pgStore.GetTenantStore(tenantSlug)
	if err != nil {
		slog.Warn("Failed to get tenant store for tool call event", "error", err, "slug", tenantSlug)
		return
	}

	dashboardStore := tenantStore.AgentDashboardStore()
	if err := dashboardStore.RecordToolCall(ctx, event); err != nil {
		slog.Warn("Failed to record tool call to database", "error", err)
	}
}

// recordCacheHitEvent records a cache hit event to both metrics and database
func (s *Service) recordCacheHitEvent(ctx context.Context, tenantID, apiKeyID, model string, tokensSaved int64, costSaved float64) {
	// Record to Prometheus metrics
	if s.metrics != nil {
		roleID := "" // Extract from request if available
		s.metrics.RecordCacheHit(model, tenantID, roleID, tokensSaved, costSaved)
	}

	// Persist to PostgreSQL
	if s.pgStore != nil {
		event := &domain.CacheEventRecord{
			APIKeyID: apiKeyID,
			Model:    model,
			Hit:      true,
			TokensSaved:  tokensSaved,
			CostSavedUSD: costSaved,
		}

		// Get tenant slug from tenant ID
		tenantSlug := s.getTenantSlug(ctx, tenantID)
		if tenantSlug == "" {
			slog.Warn("Failed to get tenant slug for cache hit event", "tenant_id", tenantID)
			return
		}

		tenantStore, err := s.pgStore.GetTenantStore(tenantSlug)
		if err != nil {
			slog.Warn("Failed to get tenant store for cache hit event", "error", err)
			return
		}

		dashboardStore := tenantStore.AgentDashboardStore()
		if err := dashboardStore.RecordCacheEvent(ctx, event); err != nil {
			slog.Warn("Failed to record cache hit to database", "error", err)
		}
	}
}

// recordCacheMissEvent records a cache miss event to both metrics and database
func (s *Service) recordCacheMissEvent(ctx context.Context, tenantID, apiKeyID, model string) {
	// Record to Prometheus metrics
	if s.metrics != nil {
		roleID := "" // Extract from request if available
		s.metrics.RecordCacheMiss(model, tenantID, roleID)
	}

	// Persist to PostgreSQL
	if s.pgStore != nil {
		event := &domain.CacheEventRecord{
			APIKeyID: apiKeyID,
			Model:    model,
			Hit:      false,
		}

		// Get tenant slug from tenant ID
		tenantSlug := s.getTenantSlug(ctx, tenantID)
		if tenantSlug == "" {
			slog.Warn("Failed to get tenant slug for cache miss event", "tenant_id", tenantID)
			return
		}

		tenantStore, err := s.pgStore.GetTenantStore(tenantSlug)
		if err != nil {
			slog.Warn("Failed to get tenant store for cache miss event", "error", err)
			return
		}

		dashboardStore := tenantStore.AgentDashboardStore()
		if err := dashboardStore.RecordCacheEvent(ctx, event); err != nil {
			slog.Warn("Failed to record cache miss to database", "error", err)
		}
	}
}

// getSeverityString converts severity integer to string for Prometheus
func getSeverityString(severity int) string {
	switch severity {
	case 1:
		return "low"
	case 2:
		return "medium"
	case 3:
		return "high"
	case 4:
		return "critical"
	case 5:
		return "severe"
	default:
		return "unknown"
	}
}

// getTenantSlug retrieves tenant slug from tenant ID
// In single-tenant mode, always returns "default"
func (s *Service) getTenantSlug(ctx context.Context, tenantID string) string {
	// Single-tenant mode - always return default
	return "default"
}
