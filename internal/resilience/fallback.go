package resilience

import (
	"context"
	"fmt"
	"sort"
	"time"

	"modelgate/internal/domain"
)

// FallbackProvider represents a fallback provider option
type FallbackProvider struct {
	Provider string
	Model    string
	Priority int // Lower = higher priority
	Timeout  time.Duration
}

// FallbackConfig contains configuration for fallback chain execution
type FallbackExecutionConfig struct {
	CircuitBreakerThreshold int // Number of failures before opening circuit
	CircuitBreakerTimeout   int // Seconds before transitioning to half-open
}

// DefaultFallbackConfig returns sensible defaults for fallback execution
func DefaultFallbackConfig() FallbackExecutionConfig {
	return FallbackExecutionConfig{
		CircuitBreakerThreshold: 5,
		CircuitBreakerTimeout:   60,
	}
}

// FallbackChain manages fallback chain execution
type FallbackChain struct {
	providers      []FallbackProvider
	circuitBreaker *CircuitBreaker
	config         FallbackExecutionConfig
}

// NewFallbackChain creates a new fallback chain with default config
func NewFallbackChain(providers []FallbackProvider, cb *CircuitBreaker) *FallbackChain {
	return NewFallbackChainWithConfig(providers, cb, DefaultFallbackConfig())
}

// NewFallbackChainWithConfig creates a new fallback chain with custom config
func NewFallbackChainWithConfig(providers []FallbackProvider, cb *CircuitBreaker, config FallbackExecutionConfig) *FallbackChain {
	// Sort by priority
	sorted := make([]FallbackProvider, len(providers))
	copy(sorted, providers)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority < sorted[j].Priority
	})

	return &FallbackChain{
		providers:      sorted,
		circuitBreaker: cb,
		config:         config,
	}
}

// Execute tries each provider in priority order until success
func (fc *FallbackChain) Execute(
	ctx context.Context,
	tenantID string,
	executeFn func(ctx context.Context, provider, model string) (*domain.ChatResponse, error),
) (*domain.ChatResponse, error) {
	var lastErr error

	for _, fallback := range fc.providers {
		// Check circuit breaker
		allowed, err := fc.circuitBreaker.AllowRequest(
			ctx,
			tenantID,
			fallback.Provider,
			fc.config.CircuitBreakerThreshold,
			fc.config.CircuitBreakerTimeout,
		)
		if err != nil || !allowed {
			continue // Skip this provider
		}

		// Try this provider with timeout
		response, err := fc.executeWithTimeout(ctx, tenantID, fallback, executeFn)
		if err == nil {
			return response, nil
		}

		lastErr = err
		// Continue to next provider
	}

	if lastErr == nil {
		return nil, fmt.Errorf("all fallback providers unavailable (circuit breakers open)")
	}
	return nil, fmt.Errorf("all fallback providers failed: %w", lastErr)
}

// executeWithTimeout executes a single provider with proper timeout handling
// This fixes the defer-in-loop issue by properly scoping the cancel function
func (fc *FallbackChain) executeWithTimeout(
	ctx context.Context,
	tenantID string,
	fallback FallbackProvider,
	executeFn func(ctx context.Context, provider, model string) (*domain.ChatResponse, error),
) (*domain.ChatResponse, error) {
	// Create context with timeout - cancel will be called when this function returns
	timeoutCtx, cancel := context.WithTimeout(ctx, fallback.Timeout)
	defer cancel()

	// Try this provider
	response, err := executeFn(timeoutCtx, fallback.Provider, fallback.Model)

	if err == nil {
		// Success
		fc.circuitBreaker.RecordSuccess(ctx, tenantID, fallback.Provider)
		return response, nil
	}

	// Record failure
	fc.circuitBreaker.RecordFailure(ctx, tenantID, fallback.Provider, fc.config.CircuitBreakerThreshold)
	return nil, err
}
