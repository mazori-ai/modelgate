package postgres

import (
	"context"
	"database/sql"
)

// =============================================================================
// ADVANCED METRICS STORAGE - Cache, Routing, Resilience, Health
// =============================================================================

// CacheMetrics represents aggregated cache statistics
type CacheMetrics struct {
	Hits        int
	Misses      int
	HitRate     float64
	TokensSaved int64
	CostSaved   float64
	AvgLatency  float64
	Entries     int
}

// StrategyCount represents count of routing decisions by strategy
type StrategyCount struct {
	Strategy string
	Count    int
}

// ModelSwitch represents a model switch record
type ModelSwitch struct {
	FromModel string
	ToModel   string
	Count     int
}

// RoutingMetrics represents aggregated routing metrics
type RoutingMetrics struct {
	Decisions            int
	StrategyDistribution []StrategyCount
	ModelSwitches        []ModelSwitch
	Failures             int
}

// CircuitBreakerInfo represents circuit breaker state for a provider
type CircuitBreakerInfo struct {
	Provider string
	State    string
	Failures int
}

// ResilienceMetrics represents aggregated resilience metrics
type ResilienceMetrics struct {
	CircuitBreakers     []CircuitBreakerInfo
	RetryAttempts       int
	FallbackInvocations int
	FallbackSuccessRate float64
}

// ProviderHealthInfo represents health information for a provider/model
type ProviderHealthInfo struct {
	Provider     string
	Model        string
	HealthScore  float64
	SuccessRate  float64
	P95LatencyMs float64
	Requests     int
}

// =============================================================================
// CACHE METRICS
// =============================================================================

// GetCacheMetrics returns aggregated cache metrics
func (s *TenantStore) GetCacheMetrics(ctx context.Context) (*CacheMetrics, error) {
	metrics := &CacheMetrics{}

	// Get cache entry count and tokens saved from semantic_cache table
	cacheQuery := `
		SELECT 
			COALESCE(COUNT(*), 0) as entries,
			COALESCE(SUM((input_tokens + output_tokens) * hit_count), 0) as tokens_saved,
			COALESCE(SUM(cost_usd * hit_count), 0) as cost_saved,
			COALESCE(AVG(NULLIF(latency_ms, 0)), 0) as avg_latency
		FROM semantic_cache
		WHERE expires_at > NOW()
	`
	err := s.db.QueryRowContext(ctx, cacheQuery).Scan(
		&metrics.Entries,
		&metrics.TokensSaved,
		&metrics.CostSaved,
		&metrics.AvgLatency,
	)
	if err != nil && err != sql.ErrNoRows {
		// If table doesn't exist or query fails, return zeros
		return &CacheMetrics{}, nil
	}

	// Get actual hits and misses from cache_events table for accurate hit rate
	eventsQuery := `
		SELECT 
			COALESCE(COUNT(*) FILTER (WHERE hit = true), 0) as hits,
			COALESCE(COUNT(*) FILTER (WHERE hit = false), 0) as misses
		FROM cache_events
	`
	err = s.db.QueryRowContext(ctx, eventsQuery).Scan(&metrics.Hits, &metrics.Misses)
	if err != nil && err != sql.ErrNoRows {
		// Fallback to semantic_cache hit_count if cache_events fails
		fallbackQuery := `SELECT COALESCE(SUM(hit_count), 0) FROM semantic_cache WHERE expires_at > NOW()`
		_ = s.db.QueryRowContext(ctx, fallbackQuery).Scan(&metrics.Hits)
	}

	// Calculate hit rate
	total := metrics.Hits + metrics.Misses
	if total > 0 {
		metrics.HitRate = float64(metrics.Hits) / float64(total)
	}

	return metrics, nil
}

// =============================================================================
// ROUTING METRICS
// =============================================================================

// GetRoutingMetrics returns aggregated routing metrics for a tenant
func (s *TenantStore) GetRoutingMetrics(ctx context.Context) (*RoutingMetrics, error) {
	metrics := &RoutingMetrics{
		StrategyDistribution: []StrategyCount{},
		ModelSwitches:        []ModelSwitch{},
	}

	// Get total decisions and failure count
	totalQuery := `
		SELECT 
			COUNT(*) as total,
			COALESCE(SUM(CASE WHEN success = false THEN 1 ELSE 0 END), 0) as failures
		FROM routing_decisions
	`
	err := s.db.QueryRowContext(ctx, totalQuery).Scan(&metrics.Decisions, &metrics.Failures)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	// Get strategy distribution
	strategyQuery := `
		SELECT strategy, COUNT(*) as count
		FROM routing_decisions
		GROUP BY strategy
		ORDER BY count DESC
	`
	rows, err := s.db.QueryContext(ctx, strategyQuery)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var sc StrategyCount
			if err := rows.Scan(&sc.Strategy, &sc.Count); err != nil {
				continue
			}
			metrics.StrategyDistribution = append(metrics.StrategyDistribution, sc)
		}
	}

	// For model switches, we'd need additional tracking in routing_decisions
	// For now, return empty list

	return metrics, nil
}

// =============================================================================
// RESILIENCE METRICS
// =============================================================================

// GetResilienceMetrics returns aggregated resilience metrics for a tenant
func (s *TenantStore) GetResilienceMetrics(ctx context.Context) (*ResilienceMetrics, error) {
	metrics := &ResilienceMetrics{
		CircuitBreakers: []CircuitBreakerInfo{},
	}

	// Get circuit breaker states
	cbQuery := `
		SELECT provider, state, failure_count
		FROM circuit_breaker_state
		ORDER BY provider
	`
	rows, err := s.db.QueryContext(ctx, cbQuery)
	if err != nil && err != sql.ErrNoRows {
		// Table might not exist
		metrics.CircuitBreakers = []CircuitBreakerInfo{}
	}
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var cb CircuitBreakerInfo
			if err := rows.Scan(&cb.Provider, &cb.State, &cb.Failures); err != nil {
				continue
			}
			metrics.CircuitBreakers = append(metrics.CircuitBreakers, cb)
		}
	}

	// Get fallback stats
	fallbackQuery := `
		SELECT 
			COUNT(*) as total,
			COALESCE(SUM(CASE WHEN fallback_success THEN 1 ELSE 0 END), 0) as successes
		FROM fallback_events
	`
	var total, successes int
	err = s.db.QueryRowContext(ctx, fallbackQuery).Scan(&total, &successes)
	if err == nil {
		metrics.FallbackInvocations = total
		if total > 0 {
			metrics.FallbackSuccessRate = float64(successes) / float64(total)
		}
	}

	// Retry attempts would need to be tracked separately
	// For now, return 0
	metrics.RetryAttempts = 0

	return metrics, nil
}

// =============================================================================
// PROVIDER HEALTH METRICS
// =============================================================================

// GetProviderHealthMetrics returns provider health metrics for a tenant
func (s *TenantStore) GetProviderHealthMetrics(ctx context.Context) ([]ProviderHealthInfo, error) {
	metrics := []ProviderHealthInfo{}

	// Query from usage_records to get actual provider health data
	query := `
		SELECT 
			provider,
			model,
			COUNT(*) as requests,
			AVG(CASE WHEN is_success THEN 1.0 ELSE 0.0 END) as success_rate,
			COALESCE(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY latency_ms), 0) as p95_latency
		FROM usage_records
		WHERE created_at >= NOW() - INTERVAL '24 hours'
		GROUP BY provider, model
		ORDER BY requests DESC
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		// If query fails, return empty metrics
		return metrics, nil
	}
	defer rows.Close()

	for rows.Next() {
		var p ProviderHealthInfo
		if err := rows.Scan(&p.Provider, &p.Model, &p.Requests, &p.SuccessRate, &p.P95LatencyMs); err != nil {
			continue
		}
		// Calculate health score based on success rate
		p.HealthScore = p.SuccessRate
		metrics = append(metrics, p)
	}

	return metrics, nil
}
