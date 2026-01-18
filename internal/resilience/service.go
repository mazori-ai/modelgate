package resilience

import (
	"context"
	"time"

	"modelgate/internal/domain"
)

// Default values for resilience configuration
const (
	DefaultBackoffMaxMs = 30000 // 30 seconds
)

// Service provides resilience functionality (retry, circuit breaker, fallback)
type Service struct {
	circuitBreaker *CircuitBreaker
}

// NewService creates a new resilience service
func NewService(cb *CircuitBreaker) *Service {
	return &Service{
		circuitBreaker: cb,
	}
}

// ExecuteWithResilience wraps a request with retry, circuit breaker, and fallback
func (s *Service) ExecuteWithResilience(
	ctx context.Context,
	tenantID string,
	policy domain.ResiliencePolicy,
	primaryFn func(ctx context.Context) (*domain.ChatResponse, error),
	fallbackFn func(ctx context.Context, provider, model string) (*domain.ChatResponse, error),
) (*domain.ChatResponse, error) {

	// Build retry config from policy - use policy values, not hardcoded defaults
	backoffMax := policy.RetryBackoffMax
	if backoffMax <= 0 {
		backoffMax = DefaultBackoffMaxMs
	}

	retryConfig := RetryConfig{
		MaxRetries:         policy.MaxRetries,
		BackoffBase:        time.Duration(policy.RetryBackoffMs) * time.Millisecond,
		BackoffMax:         time.Duration(backoffMax) * time.Millisecond,
		Jitter:             policy.RetryJitter,
		RetryOnTimeout:     policy.RetryOnTimeout,
		RetryOnRateLimit:   policy.RetryOnRateLimit,
		RetryOnServerError: policy.RetryOnServerError,
	}

	var response *domain.ChatResponse
	var err error

	// Try primary provider with retry
	if policy.RetryEnabled {
		err = Retry(ctx, retryConfig, func() error {
			response, err = primaryFn(ctx)
			return err
		})
	} else {
		response, err = primaryFn(ctx)
	}

	// If primary failed and fallback enabled, try fallback chain
	if err != nil && policy.FallbackEnabled && len(policy.FallbackChain) > 0 {
		fallbackProviders := make([]FallbackProvider, len(policy.FallbackChain))
		for i, fb := range policy.FallbackChain {
			fallbackProviders[i] = FallbackProvider{
				Provider: fb.Provider,
				Model:    fb.Model,
				Priority: fb.Priority,
				Timeout:  time.Duration(fb.TimeoutMs) * time.Millisecond,
			}
		}

		// Use policy values for circuit breaker configuration
		fallbackConfig := FallbackExecutionConfig{
			CircuitBreakerThreshold: policy.CircuitBreakerThreshold,
			CircuitBreakerTimeout:   policy.CircuitBreakerTimeout,
		}

		// Apply defaults if not set
		if fallbackConfig.CircuitBreakerThreshold <= 0 {
			fallbackConfig.CircuitBreakerThreshold = 5
		}
		if fallbackConfig.CircuitBreakerTimeout <= 0 {
			fallbackConfig.CircuitBreakerTimeout = 60
		}

		fallbackChain := NewFallbackChainWithConfig(fallbackProviders, s.circuitBreaker, fallbackConfig)
		response, err = fallbackChain.Execute(ctx, tenantID, fallbackFn)
	}

	return response, err
}
