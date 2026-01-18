// Package enforcement provides policy enforcement implementations
package enforcement

import (
	"context"
	"errors"
	"math/rand"
	"modelgate/internal/domain"
	"sync"
	"time"
)

// ResilienceEnforcer enforces resilience policies (retry, fallback, circuit breaker)
type ResilienceEnforcer struct {
	mu              sync.RWMutex
	circuitBreakers map[string]*CircuitBreaker // tenantID:provider:model -> breaker
}

// CircuitBreaker tracks circuit breaker state
type CircuitBreaker struct {
	State         CircuitState
	FailureCount  int
	SuccessCount  int
	LastFailureAt time.Time
	LastSuccessAt time.Time
	OpenedAt      time.Time
	HalfOpenAt    time.Time
}

// CircuitState represents circuit breaker state
type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"
	CircuitOpen     CircuitState = "open"
	CircuitHalfOpen CircuitState = "half_open"
)

// NewResilienceEnforcer creates a new resilience enforcer
func NewResilienceEnforcer() *ResilienceEnforcer {
	return &ResilienceEnforcer{
		circuitBreakers: make(map[string]*CircuitBreaker),
	}
}

// ExecuteWithResilience executes a request with retry and fallback
func (e *ResilienceEnforcer) ExecuteWithResilience(
	ctx context.Context,
	policy domain.ResiliencePolicy,
	tenantID string,
	provider string,
	model string,
	execute func(ctx context.Context, provider, model string) (*domain.ChatResponse, error),
) (*domain.ChatResponse, error) {
	if !policy.Enabled {
		return execute(ctx, provider, model)
	}

	// Check circuit breaker first
	if policy.CircuitBreakerEnabled {
		if !e.canExecute(tenantID, provider, model, policy) {
			// Try fallback chain
			if policy.FallbackEnabled && len(policy.FallbackChain) > 0 {
				return e.executeFallbackChain(ctx, policy, tenantID, execute)
			}
			return nil, errors.New("circuit breaker is open")
		}
	}

	// Execute with retry
	var lastErr error
	var resp *domain.ChatResponse

	maxAttempts := 1
	if policy.RetryEnabled {
		maxAttempts = policy.MaxRetries + 1
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Apply timeout
		execCtx := ctx
		if policy.RequestTimeoutMs > 0 {
			var cancel context.CancelFunc
			execCtx, cancel = context.WithTimeout(ctx, time.Duration(policy.RequestTimeoutMs)*time.Millisecond)
			defer cancel()
		}

		resp, lastErr = execute(execCtx, provider, model)

		if lastErr == nil {
			e.recordSuccess(tenantID, provider, model, policy)
			return resp, nil
		}

		// Check if error is retryable
		if !e.isRetryable(lastErr, policy) {
			break
		}

		// Record failure
		e.recordFailure(tenantID, provider, model, policy)

		// Don't wait after last attempt
		if attempt < maxAttempts-1 {
			backoff := e.calculateBackoff(attempt, policy)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	// All retries failed - try fallback chain
	if policy.FallbackEnabled && len(policy.FallbackChain) > 0 {
		return e.executeFallbackChain(ctx, policy, tenantID, execute)
	}

	return nil, lastErr
}

// executeFallbackChain tries each fallback in order
func (e *ResilienceEnforcer) executeFallbackChain(
	ctx context.Context,
	policy domain.ResiliencePolicy,
	tenantID string,
	execute func(ctx context.Context, provider, model string) (*domain.ChatResponse, error),
) (*domain.ChatResponse, error) {
	var lastErr error

	for _, fb := range policy.FallbackChain {
		// Check circuit breaker for fallback provider
		if policy.CircuitBreakerEnabled && !e.canExecute(tenantID, fb.Provider, fb.Model, policy) {
			continue
		}

		// Apply fallback-specific timeout
		execCtx := ctx
		if fb.TimeoutMs > 0 {
			var cancel context.CancelFunc
			execCtx, cancel = context.WithTimeout(ctx, time.Duration(fb.TimeoutMs)*time.Millisecond)
			defer cancel()
		}

		resp, err := execute(execCtx, fb.Provider, fb.Model)
		if err == nil {
			e.recordSuccess(tenantID, fb.Provider, fb.Model, policy)
			return resp, nil
		}

		e.recordFailure(tenantID, fb.Provider, fb.Model, policy)
		lastErr = err
	}

	return nil, lastErr
}

// isRetryable checks if an error should be retried
func (e *ResilienceEnforcer) isRetryable(err error, policy domain.ResiliencePolicy) bool {
	errStr := err.Error()

	// Check for timeout
	if policy.RetryOnTimeout && (errors.Is(err, context.DeadlineExceeded) || containsAny(errStr, []string{"timeout", "timed out"})) {
		return true
	}

	// Check for rate limit
	if policy.RetryOnRateLimit && containsAny(errStr, []string{"rate limit", "rate_limit", "429", "too many requests"}) {
		return true
	}

	// Check for server error
	if policy.RetryOnServerError && containsAny(errStr, []string{"500", "502", "503", "504", "internal server error"}) {
		return true
	}

	// Check custom retryable errors
	for _, pattern := range policy.RetryableErrors {
		if containsAny(errStr, []string{pattern}) {
			return true
		}
	}

	return false
}

// calculateBackoff calculates backoff duration for retry
func (e *ResilienceEnforcer) calculateBackoff(attempt int, policy domain.ResiliencePolicy) time.Duration {
	// Exponential backoff: base * 2^attempt
	backoff := time.Duration(policy.RetryBackoffMs) * time.Millisecond
	for i := 0; i < attempt; i++ {
		backoff *= 2
	}

	// Cap at max
	maxBackoff := time.Duration(policy.RetryBackoffMax) * time.Millisecond
	if maxBackoff > 0 && backoff > maxBackoff {
		backoff = maxBackoff
	}

	// Add jitter if enabled
	if policy.RetryJitter {
		jitter := time.Duration(rand.Int63n(int64(backoff) / 4))
		backoff += jitter
	}

	return backoff
}

// canExecute checks if circuit breaker allows execution
func (e *ResilienceEnforcer) canExecute(tenantID, provider, model string, policy domain.ResiliencePolicy) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	key := tenantID + ":" + provider + ":" + model
	cb, exists := e.circuitBreakers[key]
	if !exists {
		return true // No breaker = closed
	}

	switch cb.State {
	case CircuitClosed:
		return true
	case CircuitOpen:
		// Check if timeout has passed for half-open transition
		if time.Since(cb.OpenedAt) > time.Duration(policy.CircuitBreakerTimeout)*time.Second {
			// Transition to half-open (would need write lock in production)
			return true
		}
		return false
	case CircuitHalfOpen:
		return true // Allow one request to test
	}

	return true
}

// recordSuccess records a successful execution
func (e *ResilienceEnforcer) recordSuccess(tenantID, provider, model string, policy domain.ResiliencePolicy) {
	if !policy.CircuitBreakerEnabled {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	key := tenantID + ":" + provider + ":" + model
	cb, exists := e.circuitBreakers[key]
	if !exists {
		cb = &CircuitBreaker{State: CircuitClosed}
		e.circuitBreakers[key] = cb
	}

	cb.SuccessCount++
	cb.LastSuccessAt = time.Now()

	// If half-open and successful, close the circuit
	if cb.State == CircuitHalfOpen {
		cb.State = CircuitClosed
		cb.FailureCount = 0
	}
}

// recordFailure records a failed execution
func (e *ResilienceEnforcer) recordFailure(tenantID, provider, model string, policy domain.ResiliencePolicy) {
	if !policy.CircuitBreakerEnabled {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	key := tenantID + ":" + provider + ":" + model
	cb, exists := e.circuitBreakers[key]
	if !exists {
		cb = &CircuitBreaker{State: CircuitClosed}
		e.circuitBreakers[key] = cb
	}

	cb.FailureCount++
	cb.LastFailureAt = time.Now()

	// Check if threshold reached
	if cb.FailureCount >= policy.CircuitBreakerThreshold {
		cb.State = CircuitOpen
		cb.OpenedAt = time.Now()
	}
}

// GetCircuitState returns the circuit breaker state for a provider/model
func (e *ResilienceEnforcer) GetCircuitState(tenantID, provider, model string) CircuitState {
	e.mu.RLock()
	defer e.mu.RUnlock()

	key := tenantID + ":" + provider + ":" + model
	cb, exists := e.circuitBreakers[key]
	if !exists {
		return CircuitClosed
	}
	return cb.State
}

// containsAny checks if s contains any of the substrings
func containsAny(s string, substrs []string) bool {
	for _, sub := range substrs {
		if len(s) >= len(sub) {
			// Simple check - in production use proper string matching
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
