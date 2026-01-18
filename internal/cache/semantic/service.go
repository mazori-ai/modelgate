package semantic

import (
	"context"
	"time"

	"modelgate/internal/cache/embedding"
	"modelgate/internal/domain"
)

// Service provides semantic caching functionality
type Service struct {
	repo      *Repository
	embedding *embedding.EmbeddingService
}

// NewService creates a new semantic cache service
func NewService(repo *Repository, embeddingSvc *embedding.EmbeddingService) *Service {
	return &Service{
		repo:      repo,
		embedding: embeddingSvc,
	}
}

// CacheResult contains the result of a cache lookup
type CacheResult struct {
	Response   *domain.ChatResponse
	Hit        bool
	Similarity float64 // Similarity score if semantic match (1.0 for exact match)
	LatencyMs  int     // Original request latency (for stats)
}

// Get attempts to retrieve a cached response
// roleID: role for cache isolation (can be empty for global lookup)
func (s *Service) Get(
	ctx context.Context,
	roleID, model string,
	messages []domain.Message,
	config domain.CachingPolicy,
) (*domain.ChatResponse, bool, error) {
	result, err := s.GetWithDetails(ctx, roleID, model, messages, config)
	if err != nil {
		return nil, false, err
	}
	return result.Response, result.Hit, nil
}

// GetWithDetails attempts to retrieve a cached response with additional metadata
func (s *Service) GetWithDetails(
	ctx context.Context,
	roleID, model string,
	messages []domain.Message,
	config domain.CachingPolicy,
) (*CacheResult, error) {
	result := &CacheResult{Hit: false}

	if !config.Enabled {
		return result, nil
	}

	// Normalize prompt for consistent hashing
	normalizedPrompt := embedding.NormalizePrompt(messages)
	if normalizedPrompt == "" {
		return result, nil // Nothing to cache
	}
	requestHash := embedding.HashPrompt(normalizedPrompt)

	// Fast path: exact match by hash
	entry, err := s.repo.GetByHash(ctx, roleID, model, requestHash)
	if err != nil {
		return nil, err
	}
	if entry != nil {
		// Exact match found
		response, err := ParseResponse(entry.ResponseContent)
		if err != nil {
			return nil, err
		}

		// Mark as cached response
		response.Cached = true

		result.Response = response
		result.Hit = true
		result.Similarity = 1.0 // Exact match
		result.LatencyMs = entry.LatencyMs
		return result, nil
	}

	// Slow path: semantic similarity search (if embedding service available and threshold > 0)
	if config.SimilarityThreshold > 0 && s.embedding != nil {
		embeddingVec, err := s.embedding.GenerateEmbedding(ctx, normalizedPrompt)
		if err != nil {
			// Log error but don't fail the request - proceed without semantic cache
			return result, nil
		}

		entry, similarity, err := s.repo.SearchBySimilarity(
			ctx, roleID, model, embeddingVec, config.SimilarityThreshold,
		)
		if err != nil {
			return nil, err
		}

		if entry != nil && similarity >= config.SimilarityThreshold {
			response, err := ParseResponse(entry.ResponseContent)
			if err != nil {
				return nil, err
			}

			// Mark as cached response (semantic match)
			response.Cached = true

			result.Response = response
			result.Hit = true
			result.Similarity = similarity
			result.LatencyMs = entry.LatencyMs
			return result, nil
		}
	}

	// Cache miss
	return result, nil
}

// SetRequest contains all parameters needed to cache a response
type SetRequest struct {
	RoleID    string // Role for cache isolation
	Model     string
	Provider  string
	Messages  []domain.Message
	Response  *domain.ChatResponse
	LatencyMs int // Latency of the original request in milliseconds
}

// Set stores a response in the cache
func (s *Service) Set(
	ctx context.Context,
	roleID, model, provider string,
	messages []domain.Message,
	response *domain.ChatResponse,
	config domain.CachingPolicy,
) error {
	return s.SetWithLatency(ctx, SetRequest{
		RoleID:   roleID,
		Model:    model,
		Provider: provider,
		Messages: messages,
		Response: response,
	}, config)
}

// SetWithLatency stores a response in the cache with latency tracking
func (s *Service) SetWithLatency(
	ctx context.Context,
	req SetRequest,
	config domain.CachingPolicy,
) error {
	if !config.Enabled {
		return nil
	}

	// Check model exclusions
	for _, excluded := range config.ExcludedModels {
		if excluded == req.Model {
			return nil // Don't cache this model
		}
	}

	// Normalize prompt
	normalizedPrompt := embedding.NormalizePrompt(req.Messages)
	if normalizedPrompt == "" {
		return nil // Nothing to cache
	}

	// Check excluded patterns
	for _, pattern := range config.ExcludedPatterns {
		if matchesPattern(normalizedPrompt, pattern) {
			return nil // Don't cache prompts matching this pattern
		}
	}

	requestHash := embedding.HashPrompt(normalizedPrompt)

	// Serialize request and response
	requestBytes, err := SerializeRequest(req.Messages)
	if err != nil {
		return err
	}

	responseBytes, err := SerializeResponse(req.Response)
	if err != nil {
		return err
	}

	// Calculate expiration
	expiresAt := time.Now().Add(time.Duration(config.TTLSeconds) * time.Second)

	// Calculate tokens and cost
	inputTokens := 0
	outputTokens := 0
	if req.Response.Usage != nil {
		inputTokens = int(req.Response.Usage.PromptTokens)
		outputTokens = int(req.Response.Usage.CompletionTokens)
	}

	// Create cache entry
	entry := &CacheEntry{
		RoleID:          req.RoleID,
		Model:           req.Model,
		RequestHash:     requestHash,
		RequestContent:  requestBytes,
		ResponseContent: responseBytes,
		InputTokens:     inputTokens,
		OutputTokens:    outputTokens,
		CostUSD:         req.Response.CostUSD,
		LatencyMs:       req.LatencyMs,
		Provider:        req.Provider,
		ExpiresAt:       expiresAt,
	}

	// Generate and store embedding for semantic search (if enabled)
	if config.SimilarityThreshold > 0 && s.embedding != nil {
		embeddingVec, err := s.embedding.GenerateEmbedding(ctx, normalizedPrompt)
		if err == nil {
			entry.Embedding = embeddingVec
		}
		// If embedding fails, we still store without it (exact match still works)
	}

	return s.repo.Set(ctx, entry)
}

// matchesPattern checks if a prompt matches a pattern
func matchesPattern(prompt, pattern string) bool {
	return len(pattern) > 0 && len(prompt) > 0 &&
		(prompt == pattern || containsIgnoreCase(prompt, pattern))
}

// containsIgnoreCase checks if s contains substr (case-insensitive)
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || indexOf(toLower(s), toLower(substr)) >= 0)
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// GetStats retrieves cache statistics
func (s *Service) GetStats(ctx context.Context) (*CacheStats, error) {
	return s.repo.GetStats(ctx)
}

// Cleanup removes expired cache entries
func (s *Service) Cleanup(ctx context.Context) error {
	return s.repo.Cleanup(ctx)
}

// Invalidate removes a specific cache entry
func (s *Service) Invalidate(ctx context.Context, model string, messages []domain.Message) error {
	normalizedPrompt := embedding.NormalizePrompt(messages)
	requestHash := embedding.HashPrompt(normalizedPrompt)
	return s.repo.Delete(ctx, model, requestHash)
}

// InvalidateByHash removes a cache entry by its hash
func (s *Service) InvalidateByHash(ctx context.Context, model, requestHash string) error {
	return s.repo.Delete(ctx, model, requestHash)
}

// InvalidateAll removes all cache entries
func (s *Service) InvalidateAll(ctx context.Context) error {
	return s.repo.DeleteAll(ctx)
}

// InvalidateByRole removes all cache entries for a role
func (s *Service) InvalidateByRole(ctx context.Context, roleID string) error {
	return s.repo.DeleteByRole(ctx, roleID)
}

// Count returns the number of active cache entries
func (s *Service) Count(ctx context.Context) (int64, error) {
	return s.repo.Count(ctx)
}
