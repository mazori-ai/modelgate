package semantic

import (
	"context"

	"modelgate/internal/domain"
)

// CacheService defines the interface for semantic caching operations
type CacheService interface {
	// Get attempts to retrieve a cached response
	// roleID: role for cache isolation (can be empty for global lookup)
	Get(
		ctx context.Context,
		roleID, model string,
		messages []domain.Message,
		config domain.CachingPolicy,
	) (*domain.ChatResponse, bool, error)

	// Set stores a response in the cache
	// roleID: role for cache isolation
	Set(
		ctx context.Context,
		roleID, model, provider string,
		messages []domain.Message,
		response *domain.ChatResponse,
		config domain.CachingPolicy,
	) error

	// SetWithLatency stores a response with latency tracking
	SetWithLatency(
		ctx context.Context,
		req SetRequest,
		config domain.CachingPolicy,
	) error
}
