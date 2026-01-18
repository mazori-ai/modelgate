package resilience

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
)

// CircuitState represents the circuit breaker state
type CircuitState string

const (
	StateClosed   CircuitState = "closed"    // Normal operation
	StateOpen     CircuitState = "open"      // Failures exceeded threshold
	StateHalfOpen CircuitState = "half_open" // Testing if recovered
)

// CircuitBreaker implements circuit breaker pattern for provider failures
type CircuitBreaker struct {
	db    *sql.DB
	cache sync.Map // tenant:provider -> *CircuitStatus
}

// CircuitStatus represents the current status of a circuit
type CircuitStatus struct {
	State                CircuitState
	FailureCount         int
	ConsecutiveSuccesses int
	LastFailureAt        time.Time
	OpenedAt             time.Time
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(db *sql.DB) *CircuitBreaker {
	return &CircuitBreaker{db: db}
}

// AllowRequest checks if request is allowed based on circuit state
func (cb *CircuitBreaker) AllowRequest(ctx context.Context, tenantID, provider string, threshold, timeoutSec int) (bool, error) {
	status, err := cb.getStatus(ctx, tenantID, provider)
	if err != nil {
		return true, err // Fail open on error
	}

	switch status.State {
	case StateClosed:
		return true, nil

	case StateOpen:
		// Check if timeout has elapsed
		if time.Since(status.OpenedAt) > time.Duration(timeoutSec)*time.Second {
			// Transition to half-open
			cb.transitionToHalfOpen(ctx, tenantID, provider)
			return true, nil
		}
		return false, fmt.Errorf("circuit breaker open for provider %s", provider)

	case StateHalfOpen:
		// Allow one test request
		return true, nil

	default:
		return true, nil
	}
}

// RecordSuccess records a successful request
func (cb *CircuitBreaker) RecordSuccess(ctx context.Context, tenantID, provider string) {
	go func() {
		status, _ := cb.getStatus(context.Background(), tenantID, provider)

		if status.State == StateHalfOpen {
			// Test request succeeded, close circuit
			cb.transitionToClosed(context.Background(), tenantID, provider)
		}
	}()
}

// RecordFailure records a failed request
func (cb *CircuitBreaker) RecordFailure(ctx context.Context, tenantID, provider string, threshold int) {
	go func() {
		ctx := context.Background()

		query := `
			INSERT INTO circuit_breaker_state (tenant_id, provider, state, failure_count, last_failure_at)
			VALUES ($1, $2, $3, 1, NOW())
			ON CONFLICT (tenant_id, provider) DO UPDATE SET
				failure_count = circuit_breaker_state.failure_count + 1,
				last_failure_at = NOW()
			RETURNING failure_count
		`

		var failureCount int
		err := cb.db.QueryRowContext(ctx, query, tenantID, provider, StateClosed).Scan(&failureCount)
		if err != nil {
			return
		}

		// Check if threshold exceeded
		if failureCount >= threshold {
			cb.transitionToOpen(ctx, tenantID, provider)
		}

		// Invalidate cache
		cb.cache.Delete(tenantID + ":" + provider)
	}()
}

// getStatus retrieves the current circuit status
func (cb *CircuitBreaker) getStatus(ctx context.Context, tenantID, provider string) (*CircuitStatus, error) {
	// Check cache
	cacheKey := tenantID + ":" + provider
	if cached, ok := cb.cache.Load(cacheKey); ok {
		return cached.(*CircuitStatus), nil
	}

	query := `
		SELECT state, failure_count, consecutive_successes, last_failure_at, opened_at
		FROM circuit_breaker_state
		WHERE tenant_id = $1 AND provider = $2
	`

	var status CircuitStatus
	var state string
	var lastFailure, opened sql.NullTime

	err := cb.db.QueryRowContext(ctx, query, tenantID, provider).Scan(
		&state, &status.FailureCount, &status.ConsecutiveSuccesses, &lastFailure, &opened,
	)

	if err == sql.ErrNoRows {
		// No record, circuit is closed
		status = CircuitStatus{State: StateClosed}
	} else if err != nil {
		return nil, err
	} else {
		status.State = CircuitState(state)
		if lastFailure.Valid {
			status.LastFailureAt = lastFailure.Time
		}
		if opened.Valid {
			status.OpenedAt = opened.Time
		}
	}

	// Cache for 10 seconds
	cb.cache.Store(cacheKey, &status)
	time.AfterFunc(10*time.Second, func() {
		cb.cache.Delete(cacheKey)
	})

	return &status, nil
}

// transitionToOpen transitions circuit to open state
func (cb *CircuitBreaker) transitionToOpen(ctx context.Context, tenantID, provider string) {
	query := `
		UPDATE circuit_breaker_state
		SET state = $1, opened_at = NOW(), last_state_change_at = NOW()
		WHERE tenant_id = $2 AND provider = $3
	`

	_, _ = cb.db.ExecContext(ctx, query, StateOpen, tenantID, provider)
	cb.cache.Delete(tenantID + ":" + provider)
}

// transitionToHalfOpen transitions circuit to half-open state
func (cb *CircuitBreaker) transitionToHalfOpen(ctx context.Context, tenantID, provider string) {
	query := `
		UPDATE circuit_breaker_state
		SET state = $1, last_state_change_at = NOW()
		WHERE tenant_id = $2 AND provider = $3
	`

	_, _ = cb.db.ExecContext(ctx, query, StateHalfOpen, tenantID, provider)
	cb.cache.Delete(tenantID + ":" + provider)
}

// transitionToClosed transitions circuit to closed state
func (cb *CircuitBreaker) transitionToClosed(ctx context.Context, tenantID, provider string) {
	query := `
		UPDATE circuit_breaker_state
		SET state = $1, failure_count = 0, consecutive_successes = 0, last_state_change_at = NOW()
		WHERE tenant_id = $2 AND provider = $3
	`

	_, _ = cb.db.ExecContext(ctx, query, StateClosed, tenantID, provider)
	cb.cache.Delete(tenantID + ":" + provider)
}
