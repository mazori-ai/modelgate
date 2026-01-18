package health

import (
	"context"
	"database/sql"
	"sync"
	"time"
)

// ProviderHealth represents health metrics for a provider
type ProviderHealth struct {
	Provider      string
	Model         string
	SuccessCount  int
	TotalRequests int
	AvgLatencyMs  float64
	P95LatencyMs  float64
	ErrorCount    int
	HealthScore   float64 // 0.0-1.0
	LastSuccessAt time.Time
	LastFailureAt time.Time
}

// Tracker tracks provider health metrics for routing decisions
type Tracker struct {
	db    *sql.DB
	cache sync.Map // tenant:provider:model -> *ProviderHealth
}

// NewTracker creates a new health tracker
func NewTracker(db *sql.DB) *Tracker {
	return &Tracker{db: db}
}

// RecordSuccess updates health metrics after successful request
func (t *Tracker) RecordSuccess(ctx context.Context, tenantID, provider, model string, latencyMs int) {
	go t.updateHealth(context.Background(), tenantID, provider, model, true, latencyMs, "")
}

// RecordFailure updates health metrics after failed request
func (t *Tracker) RecordFailure(ctx context.Context, tenantID, provider, model, errorType string) {
	go t.updateHealth(context.Background(), tenantID, provider, model, false, 0, errorType)
}

// updateHealth updates health metrics in database
func (t *Tracker) updateHealth(ctx context.Context, tenantID, provider, model string, success bool, latencyMs int, errorType string) {
	query := `SELECT update_provider_health($1, $2, $3, $4, $5, $6)`

	_, err := t.db.ExecContext(ctx, query, tenantID, provider, model, success, latencyMs, errorType)
	if err != nil {
		// Log error but don't fail
		return
	}

	// Invalidate cache
	cacheKey := tenantID + ":" + provider + ":" + model
	t.cache.Delete(cacheKey)
}

// GetHealth retrieves health metrics for a provider
func (t *Tracker) GetHealth(ctx context.Context, tenantID, provider, model string) (*ProviderHealth, error) {
	// Check cache first
	cacheKey := tenantID + ":" + provider + ":" + model
	if cached, ok := t.cache.Load(cacheKey); ok {
		return cached.(*ProviderHealth), nil
	}

	query := `
		SELECT success_count, total_requests, avg_latency_ms, p95_latency_ms,
		       error_count, health_score, last_success_at, last_failure_at
		FROM provider_health
		WHERE tenant_id = $1 AND provider = $2 AND ($3 = '' OR model = $3)
	`

	var health ProviderHealth
	var lastSuccess, lastFailure sql.NullTime

	health.Provider = provider
	health.Model = model

	err := t.db.QueryRowContext(ctx, query, tenantID, provider, model).Scan(
		&health.SuccessCount, &health.TotalRequests, &health.AvgLatencyMs,
		&health.P95LatencyMs, &health.ErrorCount, &health.HealthScore,
		&lastSuccess, &lastFailure,
	)

	if err == sql.ErrNoRows {
		// No data yet, return perfect health
		health.HealthScore = 1.0
	} else if err != nil {
		return nil, err
	}

	if lastSuccess.Valid {
		health.LastSuccessAt = lastSuccess.Time
	}
	if lastFailure.Valid {
		health.LastFailureAt = lastFailure.Time
	}

	// Cache for 60 seconds
	t.cache.Store(cacheKey, &health)
	time.AfterFunc(60*time.Second, func() {
		t.cache.Delete(cacheKey)
	})

	return &health, nil
}

// GetAllHealth retrieves health for all providers for a tenant
func (t *Tracker) GetAllHealth(ctx context.Context, tenantID string) ([]*ProviderHealth, error) {
	query := `
		SELECT provider, model, success_count, total_requests, avg_latency_ms,
		       p95_latency_ms, error_count, health_score, last_success_at, last_failure_at
		FROM provider_health
		WHERE tenant_id = $1
		ORDER BY health_score DESC
	`

	rows, err := t.db.QueryContext(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var healths []*ProviderHealth
	for rows.Next() {
		var h ProviderHealth
		var lastSuccess, lastFailure sql.NullTime

		err := rows.Scan(
			&h.Provider, &h.Model, &h.SuccessCount, &h.TotalRequests,
			&h.AvgLatencyMs, &h.P95LatencyMs, &h.ErrorCount, &h.HealthScore,
			&lastSuccess, &lastFailure,
		)
		if err != nil {
			continue
		}

		if lastSuccess.Valid {
			h.LastSuccessAt = lastSuccess.Time
		}
		if lastFailure.Valid {
			h.LastFailureAt = lastFailure.Time
		}

		healths = append(healths, &h)
	}

	return healths, nil
}
