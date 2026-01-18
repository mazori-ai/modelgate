package semantic

import (
	"context"
	"database/sql"

	"modelgate/internal/cache/embedding"
	"modelgate/internal/domain"
)

// TenantAwareService wraps the semantic cache service
// In single-tenant mode, this uses a single database connection
type TenantAwareService struct {
	db               *sql.DB
	embeddingService *embedding.EmbeddingService
	service          *Service
}

// NewTenantAwareService creates a semantic cache service
func NewTenantAwareService(
	db *sql.DB,
	embeddingSvc *embedding.EmbeddingService,
) *TenantAwareService {
	repo := NewRepository(db)
	svc := NewService(repo, embeddingSvc)

	return &TenantAwareService{
		db:               db,
		embeddingService: embeddingSvc,
		service:          svc,
	}
}

// Get attempts to retrieve a cached response
// roleID: role for cache isolation
func (s *TenantAwareService) Get(
	ctx context.Context,
	roleID, model string,
	messages []domain.Message,
	config domain.CachingPolicy,
) (*domain.ChatResponse, bool, error) {
	return s.service.Get(ctx, roleID, model, messages, config)
}

// Set stores a response in the cache
func (s *TenantAwareService) Set(
	ctx context.Context,
	roleID, model, provider string,
	messages []domain.Message,
	response *domain.ChatResponse,
	config domain.CachingPolicy,
) error {
	return s.service.Set(ctx, roleID, model, provider, messages, response, config)
}

// SetWithLatency stores a response in the cache with latency tracking
func (s *TenantAwareService) SetWithLatency(
	ctx context.Context,
	req SetRequest,
	config domain.CachingPolicy,
) error {
	return s.service.SetWithLatency(ctx, req, config)
}

// GetStats retrieves cache statistics
func (s *TenantAwareService) GetStats(ctx context.Context) (*CacheStats, error) {
	return s.service.GetStats(ctx)
}

// Cleanup removes expired cache entries
func (s *TenantAwareService) Cleanup(ctx context.Context) error {
	return s.service.Cleanup(ctx)
}

// Invalidate removes a specific cache entry
func (s *TenantAwareService) Invalidate(ctx context.Context, model string, messages []domain.Message) error {
	return s.service.Invalidate(ctx, model, messages)
}

// InvalidateAll removes all cache entries
func (s *TenantAwareService) InvalidateAll(ctx context.Context) error {
	return s.service.InvalidateAll(ctx)
}

// InvalidateByRole removes all cache entries for a role
func (s *TenantAwareService) InvalidateByRole(ctx context.Context, roleID string) error {
	return s.service.InvalidateByRole(ctx, roleID)
}

// Count returns the number of active cache entries
func (s *TenantAwareService) Count(ctx context.Context) (int64, error) {
	return s.service.Count(ctx)
}
