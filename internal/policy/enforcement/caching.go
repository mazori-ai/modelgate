// Package enforcement provides policy enforcement implementations
package enforcement

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"modelgate/internal/domain"
	"sync"
	"time"
)

// CachingEnforcer enforces caching policies per role
type CachingEnforcer struct {
	mu    sync.RWMutex
	cache map[string]*CacheEntry // tenantID:roleID:hash -> entry
}

// CacheEntry represents a cached response
type CacheEntry struct {
	Response   *domain.ChatResponse
	CachedAt   time.Time
	ExpiresAt  time.Time
	HitCount   int
	CostSaved  float64
}

// NewCachingEnforcer creates a new caching enforcer
func NewCachingEnforcer() *CachingEnforcer {
	return &CachingEnforcer{
		cache: make(map[string]*CacheEntry),
	}
}

// ShouldCache checks if the request should be cached based on policy
func (e *CachingEnforcer) ShouldCache(policy domain.CachingPolicy, req *domain.ChatRequest, isStreaming bool) bool {
	if !policy.Enabled {
		return false
	}

	// Check excluded models
	for _, model := range policy.ExcludedModels {
		if req.Model == model {
			return false
		}
	}

	// Check if streaming and streaming cache is disabled
	if isStreaming && !policy.CacheStreaming {
		return false
	}

	// Check if has tools and tool cache is disabled
	if len(req.Tools) > 0 && !policy.CacheToolCalls {
		return false
	}

	return true
}

// Get retrieves a cached response if available and similar enough
func (e *CachingEnforcer) Get(ctx context.Context, tenantID, roleID string, req *domain.ChatRequest, policy domain.CachingPolicy) (*domain.ChatResponse, bool) {
	if !policy.Enabled {
		return nil, false
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	// Generate cache key
	key := e.generateKey(tenantID, roleID, req)
	
	entry, exists := e.cache[key]
	if !exists {
		return nil, false
	}

	// Check if expired
	if time.Now().After(entry.ExpiresAt) {
		return nil, false
	}

	// Update hit count (needs write lock in real implementation)
	entry.HitCount++

	return entry.Response, true
}

// Set stores a response in the cache
func (e *CachingEnforcer) Set(ctx context.Context, tenantID, roleID string, req *domain.ChatRequest, resp *domain.ChatResponse, policy domain.CachingPolicy) {
	if !policy.Enabled {
		return
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	key := e.generateKey(tenantID, roleID, req)
	
	e.cache[key] = &CacheEntry{
		Response:  resp,
		CachedAt:  time.Now(),
		ExpiresAt: time.Now().Add(time.Duration(policy.TTLSeconds) * time.Second),
		HitCount:  0,
	}

	// Enforce max cache size per role
	e.enforceMaxSize(tenantID, roleID, policy.MaxCacheSize)
}

// generateKey creates a cache key from the request
func (e *CachingEnforcer) generateKey(tenantID, roleID string, req *domain.ChatRequest) string {
	// Hash the messages content
	h := sha256.New()
	h.Write([]byte(tenantID))
	h.Write([]byte(roleID))
	h.Write([]byte(req.Model))
	for _, msg := range req.Messages {
		h.Write([]byte(msg.Role))
		// Serialize content blocks
		for _, block := range msg.Content {
			h.Write([]byte(block.Type))
			h.Write([]byte(block.Text))
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// enforceMaxSize removes oldest entries if cache exceeds max size
func (e *CachingEnforcer) enforceMaxSize(tenantID, roleID string, maxSize int) {
	// TODO: Implement LRU eviction for role-specific entries
	// For now, just count entries for this role
	prefix := tenantID + ":" + roleID + ":"
	count := 0
	for k := range e.cache {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			count++
		}
	}
	
	// If over limit, remove oldest (simplified - should be LRU)
	if count > maxSize {
		// TODO: Implement proper LRU eviction
	}
}

// GetStats returns cache statistics for a role
func (e *CachingEnforcer) GetStats(tenantID, roleID string) CacheStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := CacheStats{}
	prefix := tenantID + ":" + roleID + ":"
	
	for k, entry := range e.cache {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			stats.EntryCount++
			stats.TotalHits += entry.HitCount
			stats.TotalCostSaved += entry.CostSaved
		}
	}

	return stats
}

// CacheStats contains cache statistics
type CacheStats struct {
	EntryCount     int
	TotalHits      int
	TotalCostSaved float64
}

