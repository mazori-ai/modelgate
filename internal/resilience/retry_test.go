package resilience

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetry(t *testing.T) {
	t.Run("success on first try", func(t *testing.T) {
		attempts := 0
		config := RetryConfig{
			MaxRetries:  3,
			BackoffBase: 10 * time.Millisecond,
			BackoffMax:  100 * time.Millisecond,
		}

		err := Retry(context.Background(), config, func() error {
			attempts++
			return nil
		})

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if attempts != 1 {
			t.Errorf("Expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("success after retries", func(t *testing.T) {
		attempts := 0
		config := RetryConfig{
			MaxRetries:         3,
			BackoffBase:        10 * time.Millisecond,
			BackoffMax:         100 * time.Millisecond,
			RetryOnServerError: true,
		}

		err := Retry(context.Background(), config, func() error {
			attempts++
			if attempts < 3 {
				return errors.New("500 server error")
			}
			return nil
		})

		if err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
		if attempts != 3 {
			t.Errorf("Expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("max retries exceeded", func(t *testing.T) {
		attempts := 0
		config := RetryConfig{
			MaxRetries:         2,
			BackoffBase:        10 * time.Millisecond,
			BackoffMax:         100 * time.Millisecond,
			RetryOnServerError: true,
		}

		err := Retry(context.Background(), config, func() error {
			attempts++
			return errors.New("500 persistent error")
		})

		if err == nil {
			t.Error("Expected error after max retries")
		}
		if attempts != 3 { // initial + 2 retries
			t.Errorf("Expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("non-retryable error", func(t *testing.T) {
		attempts := 0
		config := RetryConfig{
			MaxRetries:         3,
			BackoffBase:        10 * time.Millisecond,
			BackoffMax:         100 * time.Millisecond,
			RetryOnServerError: true, // Only retry server errors
		}

		err := Retry(context.Background(), config, func() error {
			attempts++
			return errors.New("400 bad request") // Not a server error
		})

		if err == nil {
			t.Error("Expected error for non-retryable")
		}
		if attempts != 1 {
			t.Errorf("Expected 1 attempt for non-retryable, got %d", attempts)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		attempts := 0
		config := RetryConfig{
			MaxRetries:         10,
			BackoffBase:        100 * time.Millisecond,
			BackoffMax:         1 * time.Second,
			RetryOnServerError: true,
		}

		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()

		err := Retry(ctx, config, func() error {
			attempts++
			return errors.New("500 server error")
		})

		if !errors.Is(err, context.Canceled) {
			t.Errorf("Expected context.Canceled, got: %v", err)
		}
		if attempts > 2 {
			t.Errorf("Should have stopped early due to cancellation, got %d attempts", attempts)
		}
	})

	t.Run("retry on timeout", func(t *testing.T) {
		attempts := 0
		config := RetryConfig{
			MaxRetries:     2,
			BackoffBase:    10 * time.Millisecond,
			BackoffMax:     100 * time.Millisecond,
			RetryOnTimeout: true,
		}

		err := Retry(context.Background(), config, func() error {
			attempts++
			if attempts < 3 {
				return errors.New("timeout exceeded")
			}
			return nil
		})

		if err != nil {
			t.Errorf("Expected success after retry, got: %v", err)
		}
		if attempts != 3 {
			t.Errorf("Expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("retry on rate limit", func(t *testing.T) {
		attempts := 0
		config := RetryConfig{
			MaxRetries:       2,
			BackoffBase:      10 * time.Millisecond,
			BackoffMax:       100 * time.Millisecond,
			RetryOnRateLimit: true,
		}

		err := Retry(context.Background(), config, func() error {
			attempts++
			if attempts < 3 {
				return errors.New("429 rate limit exceeded")
			}
			return nil
		})

		if err != nil {
			t.Errorf("Expected success after retry, got: %v", err)
		}
		if attempts != 3 {
			t.Errorf("Expected 3 attempts, got %d", attempts)
		}
	})
}

func TestCalculateBackoff(t *testing.T) {
	t.Run("exponential growth", func(t *testing.T) {
		base := 100 * time.Millisecond
		max := 10 * time.Second

		b1 := calculateBackoff(1, base, max, false)
		b2 := calculateBackoff(2, base, max, false)
		b3 := calculateBackoff(3, base, max, false)

		if b1 >= b2 || b2 >= b3 {
			t.Error("Backoff should grow exponentially")
		}
	})

	t.Run("respects max", func(t *testing.T) {
		base := 100 * time.Millisecond
		max := 500 * time.Millisecond

		b := calculateBackoff(10, base, max, false)
		if b > max {
			t.Errorf("Backoff %v exceeds max %v", b, max)
		}
	})

	t.Run("jitter adds variation", func(t *testing.T) {
		base := 100 * time.Millisecond
		max := 10 * time.Second

		// Calculate multiple times with jitter
		results := make(map[time.Duration]bool)
		for i := 0; i < 100; i++ {
			b := calculateBackoff(2, base, max, true)
			results[b] = true
		}

		// With jitter, we should get multiple different values
		if len(results) < 5 {
			t.Error("Jitter should produce variation in backoff values")
		}
	})
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		config   RetryConfig
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			config:   RetryConfig{},
			expected: false,
		},
		{
			name:     "timeout error with retry enabled",
			err:      errors.New("context deadline exceeded"),
			config:   RetryConfig{RetryOnTimeout: true},
			expected: true,
		},
		{
			name:     "timeout error with retry disabled",
			err:      errors.New("context deadline exceeded"),
			config:   RetryConfig{RetryOnTimeout: false},
			expected: false,
		},
		{
			name:     "rate limit with retry enabled",
			err:      errors.New("status 429: rate limit"),
			config:   RetryConfig{RetryOnRateLimit: true},
			expected: true,
		},
		{
			name:     "server error 500",
			err:      errors.New("status 500: internal server error"),
			config:   RetryConfig{RetryOnServerError: true},
			expected: true,
		},
		{
			name:     "server error 502",
			err:      errors.New("502 bad gateway"),
			config:   RetryConfig{RetryOnServerError: true},
			expected: true,
		},
		{
			name:     "server error 503",
			err:      errors.New("503 service unavailable"),
			config:   RetryConfig{RetryOnServerError: true},
			expected: true,
		},
		{
			name:     "connection refused",
			err:      errors.New("connection refused"),
			config:   RetryConfig{RetryOnServerError: true},
			expected: true,
		},
		{
			name:     "connection reset",
			err:      errors.New("connection reset by peer"),
			config:   RetryConfig{RetryOnServerError: true},
			expected: true,
		},
		{
			name:     "client error not retried",
			err:      errors.New("400 bad request"),
			config:   RetryConfig{RetryOnServerError: true},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryableError(tt.err, tt.config)
			if result != tt.expected {
				t.Errorf("isRetryableError() = %v, want %v", result, tt.expected)
			}
		})
	}
}
