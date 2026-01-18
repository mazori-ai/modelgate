package resilience

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"
)

// RetryConfig configuration for retry logic
type RetryConfig struct {
	MaxRetries         int
	BackoffBase        time.Duration
	BackoffMax         time.Duration
	Jitter             bool
	RetryOnTimeout     bool
	RetryOnRateLimit   bool
	RetryOnServerError bool
}

// Retry executes a function with exponential backoff retry logic
func Retry(ctx context.Context, config RetryConfig, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Calculate backoff
			backoff := calculateBackoff(attempt, config.BackoffBase, config.BackoffMax, config.Jitter)

			select {
			case <-time.After(backoff):
				// Continue to retry
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		err := fn()
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Check if error is retryable
		if !isRetryableError(err, config) {
			return err // Non-retryable error
		}
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

// calculateBackoff calculates exponential backoff with optional jitter
func calculateBackoff(attempt int, base, max time.Duration, jitter bool) time.Duration {
	// Exponential backoff: base * 2^attempt
	backoff := base * time.Duration(math.Pow(2, float64(attempt)))

	if backoff > max {
		backoff = max
	}

	if jitter {
		// Add random jitter (±25%)
		jitterRange := float64(backoff) * 0.25
		jitterAmount := (rand.Float64() - 0.5) * 2 * jitterRange
		backoff = backoff + time.Duration(jitterAmount)
	}

	if backoff < 0 {
		backoff = base
	}

	return backoff
}

// isRetryableError checks if an error should be retried
func isRetryableError(err error, config RetryConfig) bool {
	if err == nil {
		return false
	}

	// Check error message for common retryable patterns
	errStr := strings.ToLower(err.Error())

	if config.RetryOnTimeout && (strings.Contains(errStr, "timeout") || strings.Contains(errStr, "deadline exceeded")) {
		return true
	}

	if config.RetryOnRateLimit && (strings.Contains(errStr, "rate limit") || strings.Contains(errStr, "429")) {
		return true
	}

	if config.RetryOnServerError && (strings.Contains(errStr, "500") || strings.Contains(errStr, "502") ||
		strings.Contains(errStr, "503") || strings.Contains(errStr, "504") ||
		strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "broken pipe")) {
		return true
	}

	return false
}
