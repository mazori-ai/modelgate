package semantic

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/pgvector/pgvector-go"
	"modelgate/internal/domain"
)

// CacheEntry represents a cached response with embedding
type CacheEntry struct {
	ID              string
	RoleID          string          // Role-based cache isolation
	Model           string
	RequestHash     string
	RequestContent  []byte          // JSON serialized request/messages
	ResponseContent []byte          // JSON serialized ChatResponse
	Embedding       pgvector.Vector // For semantic similarity search
	InputTokens     int
	OutputTokens    int
	CostUSD         float64
	LatencyMs       int    // Original request latency
	Provider        string // Provider that served the request
	HitCount        int
	CreatedAt       time.Time
	ExpiresAt       time.Time
	LastHitAt       time.Time
}

// Repository handles semantic cache database operations
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new semantic cache repository
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// GetByHash attempts exact match by hash (fast path)
// roleID can be empty string to match any role, or specific role for isolation
func (r *Repository) GetByHash(ctx context.Context, roleID, model, requestHash string) (*CacheEntry, error) {
	var query string
	var args []interface{}

	if roleID != "" {
		// Role-specific cache lookup
		query = `
			SELECT id, role_id, response_content, input_tokens, output_tokens, cost_usd,
			       latency_ms, provider, hit_count, created_at, expires_at
			FROM semantic_cache
			WHERE model = $1
			  AND request_hash = $2
			  AND role_id = $3
			  AND expires_at > NOW()
			LIMIT 1
		`
		args = []interface{}{model, requestHash, roleID}
	} else {
		// Global cache lookup (any role)
		query = `
			SELECT id, role_id, response_content, input_tokens, output_tokens, cost_usd,
			       latency_ms, provider, hit_count, created_at, expires_at
			FROM semantic_cache
			WHERE model = $1
			  AND request_hash = $2
			  AND expires_at > NOW()
			LIMIT 1
		`
		args = []interface{}{model, requestHash}
	}

	var entry CacheEntry
	entry.Model = model
	entry.RequestHash = requestHash

	var roleIDNull sql.NullString
	var latencyMsNull sql.NullInt64
	var providerNull sql.NullString

	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&entry.ID, &roleIDNull, &entry.ResponseContent, &entry.InputTokens, &entry.OutputTokens,
		&entry.CostUSD, &latencyMsNull, &providerNull, &entry.HitCount, &entry.CreatedAt, &entry.ExpiresAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Cache miss
	}
	if err != nil {
		return nil, err
	}

	if roleIDNull.Valid {
		entry.RoleID = roleIDNull.String
	}
	if latencyMsNull.Valid {
		entry.LatencyMs = int(latencyMsNull.Int64)
	}
	if providerNull.Valid {
		entry.Provider = providerNull.String
	}

	// Update hit count asynchronously
	go r.incrementHitCount(context.Background(), entry.ID)

	return &entry, nil
}

// SearchBySimilarity uses pgvector similarity search
func (r *Repository) SearchBySimilarity(
	ctx context.Context,
	roleID, model string,
	embedding pgvector.Vector,
	similarityThreshold float64,
) (*CacheEntry, float64, error) {
	var query string
	var args []interface{}

	if roleID != "" {
		query = `
			SELECT
				id, role_id, response_content, input_tokens, output_tokens, cost_usd,
				latency_ms, provider, hit_count, created_at, expires_at,
				1 - (embedding <=> $1::vector) as similarity
			FROM semantic_cache
			WHERE model = $2
			  AND role_id = $3
			  AND expires_at > NOW()
			  AND embedding IS NOT NULL
			  AND 1 - (embedding <=> $1::vector) >= $4
			ORDER BY similarity DESC
			LIMIT 1
		`
		args = []interface{}{embedding, model, roleID, similarityThreshold}
	} else {
		query = `
			SELECT
				id, role_id, response_content, input_tokens, output_tokens, cost_usd,
				latency_ms, provider, hit_count, created_at, expires_at,
				1 - (embedding <=> $1::vector) as similarity
			FROM semantic_cache
			WHERE model = $2
			  AND expires_at > NOW()
			  AND embedding IS NOT NULL
			  AND 1 - (embedding <=> $1::vector) >= $3
			ORDER BY similarity DESC
			LIMIT 1
		`
		args = []interface{}{embedding, model, similarityThreshold}
	}

	var entry CacheEntry
	var similarity float64
	entry.Model = model

	var roleIDNull sql.NullString
	var latencyMsNull sql.NullInt64
	var providerNull sql.NullString

	err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&entry.ID, &roleIDNull, &entry.ResponseContent, &entry.InputTokens, &entry.OutputTokens,
		&entry.CostUSD, &latencyMsNull, &providerNull, &entry.HitCount, &entry.CreatedAt, &entry.ExpiresAt,
		&similarity,
	)

	if err == sql.ErrNoRows {
		return nil, 0, nil // Cache miss
	}
	if err != nil {
		// If vector operations not supported, return miss silently
		return nil, 0, nil
	}

	if roleIDNull.Valid {
		entry.RoleID = roleIDNull.String
	}
	if latencyMsNull.Valid {
		entry.LatencyMs = int(latencyMsNull.Int64)
	}
	if providerNull.Valid {
		entry.Provider = providerNull.String
	}

	// Update hit count asynchronously
	go r.incrementHitCount(context.Background(), entry.ID)

	return &entry, similarity, nil
}

// Set stores a new cache entry with optional embedding
func (r *Repository) Set(ctx context.Context, entry *CacheEntry) error {
	// First try with embedding if available
	if len(entry.Embedding.Slice()) > 0 {
		query := `
			INSERT INTO semantic_cache (
				role_id, model, request_hash, request_content, response_content,
				embedding, input_tokens, output_tokens, cost_usd, latency_ms, provider, expires_at
			) VALUES ($1, $2, $3, $4, $5, $6::vector, $7, $8, $9, $10, $11, $12)
			ON CONFLICT (request_hash, model) DO UPDATE SET
				response_content = EXCLUDED.response_content,
				embedding = EXCLUDED.embedding,
				input_tokens = EXCLUDED.input_tokens,
				output_tokens = EXCLUDED.output_tokens,
				cost_usd = EXCLUDED.cost_usd,
				latency_ms = EXCLUDED.latency_ms,
				provider = EXCLUDED.provider,
				expires_at = EXCLUDED.expires_at,
				last_hit_at = NOW()
		`

		var roleID interface{}
		if entry.RoleID != "" {
			roleID = entry.RoleID
		}

		_, err := r.db.ExecContext(ctx, query,
			roleID, entry.Model, entry.RequestHash, entry.RequestContent,
			entry.ResponseContent, entry.Embedding, entry.InputTokens, entry.OutputTokens,
			entry.CostUSD, entry.LatencyMs, entry.Provider, entry.ExpiresAt,
		)

		if err == nil {
			return nil
		}
		// Fall through to try without embedding if there's an error
	}

	// Insert without embedding
	query := `
		INSERT INTO semantic_cache (
			role_id, model, request_hash, request_content, response_content,
			input_tokens, output_tokens, cost_usd, latency_ms, provider, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (request_hash, model) DO UPDATE SET
			response_content = EXCLUDED.response_content,
			input_tokens = EXCLUDED.input_tokens,
			output_tokens = EXCLUDED.output_tokens,
			cost_usd = EXCLUDED.cost_usd,
			latency_ms = EXCLUDED.latency_ms,
			provider = EXCLUDED.provider,
			expires_at = EXCLUDED.expires_at,
			last_hit_at = NOW()
	`

	var roleID interface{}
	if entry.RoleID != "" {
		roleID = entry.RoleID
	}

	_, err := r.db.ExecContext(ctx, query,
		roleID, entry.Model, entry.RequestHash, entry.RequestContent,
		entry.ResponseContent, entry.InputTokens, entry.OutputTokens,
		entry.CostUSD, entry.LatencyMs, entry.Provider, entry.ExpiresAt,
	)

	return err
}

// SetWithEmbedding stores a cache entry and its embedding
func (r *Repository) SetWithEmbedding(ctx context.Context, entry *CacheEntry, embedding pgvector.Vector) error {
	entry.Embedding = embedding
	return r.Set(ctx, entry)
}

// incrementHitCount updates hit count (async, fire-and-forget)
func (r *Repository) incrementHitCount(ctx context.Context, id string) {
	query := `
		UPDATE semantic_cache
		SET hit_count = hit_count + 1, last_hit_at = NOW()
		WHERE id = $1
	`
	_, _ = r.db.ExecContext(ctx, query, id)
}

// Cleanup removes expired entries
func (r *Repository) Cleanup(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM semantic_cache WHERE expires_at < NOW()")
	return err
}

// CacheStats represents cache performance metrics
type CacheStats struct {
	TotalHits         int64
	TotalMisses       int64
	TotalTokensSaved  int64
	TotalCostSaved    float64
	TotalLatencySaved int64
	HitRate           float64
	EntryCount        int64
}

// GetStats returns cache statistics
func (r *Repository) GetStats(ctx context.Context) (*CacheStats, error) {
	query := `
		SELECT
			COUNT(*) as entry_count,
			COALESCE(SUM(hit_count), 0) as total_hits,
			COALESCE(SUM((input_tokens + output_tokens) * hit_count), 0) as total_tokens_saved,
			COALESCE(SUM(latency_ms * hit_count), 0) as total_latency_saved
		FROM semantic_cache
		WHERE expires_at > NOW()
	`

	var stats CacheStats
	err := r.db.QueryRowContext(ctx, query).Scan(
		&stats.EntryCount, &stats.TotalHits, &stats.TotalTokensSaved, &stats.TotalLatencySaved,
	)

	if err != nil {
		return nil, err
	}

	return &stats, nil
}

// ParseResponse deserializes a cached response
func ParseResponse(data []byte) (*domain.ChatResponse, error) {
	var response domain.ChatResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// SerializeResponse serializes a response for caching
func SerializeResponse(response *domain.ChatResponse) ([]byte, error) {
	return json.Marshal(response)
}

// SerializeRequest serializes messages for caching
func SerializeRequest(messages []domain.Message) ([]byte, error) {
	return json.Marshal(messages)
}

// Delete removes a specific cache entry by hash
func (r *Repository) Delete(ctx context.Context, model, requestHash string) error {
	query := `DELETE FROM semantic_cache WHERE model = $1 AND request_hash = $2`
	_, err := r.db.ExecContext(ctx, query, model, requestHash)
	return err
}

// DeleteAll removes all cache entries
func (r *Repository) DeleteAll(ctx context.Context) error {
	query := `DELETE FROM semantic_cache`
	_, err := r.db.ExecContext(ctx, query)
	return err
}

// DeleteByModel removes all cache entries for a specific model
func (r *Repository) DeleteByModel(ctx context.Context, model string) error {
	query := `DELETE FROM semantic_cache WHERE model = $1`
	_, err := r.db.ExecContext(ctx, query, model)
	return err
}

// DeleteByRole removes all cache entries for a specific role
func (r *Repository) DeleteByRole(ctx context.Context, roleID string) error {
	query := `DELETE FROM semantic_cache WHERE role_id = $1`
	_, err := r.db.ExecContext(ctx, query, roleID)
	return err
}

// Count returns the number of active cache entries
func (r *Repository) Count(ctx context.Context) (int64, error) {
	query := `SELECT COUNT(*) FROM semantic_cache WHERE expires_at > NOW()`
	var count int64
	err := r.db.QueryRowContext(ctx, query).Scan(&count)
	return count, err
}
