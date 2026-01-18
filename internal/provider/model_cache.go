// Package provider implements LLM provider clients.
package provider

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"modelgate/internal/domain"
	"modelgate/internal/storage/postgres"
)

// ModelCacheable is an optional interface that providers can implement
// to receive model cache updates from the database
type ModelCacheable interface {
	// SetModelCache sets the model cache for this provider
	// The cache maps short model names to full native model IDs
	SetModelCache(cache map[string]string)

	// GetModelCache returns the current model cache
	GetModelCache() map[string]string
}

// ModelCacheService manages in-memory model caches for all providers across tenants
// It loads models from the database and populates provider caches to reduce latency
type ModelCacheService struct {
	mu sync.RWMutex

	// caches maps tenant_id -> provider -> model_id -> native_model_id
	caches map[string]map[domain.Provider]map[string]string
}

// NewModelCacheService creates a new model cache service
func NewModelCacheService() *ModelCacheService {
	return &ModelCacheService{
		caches: make(map[string]map[domain.Provider]map[string]string),
	}
}

// LoadFromDatabase loads model cache from database for a specific tenant and provider
func (s *ModelCacheService) LoadFromDatabase(ctx context.Context, tenantStore *postgres.TenantStore, tenantID string, provider domain.Provider) error {
	models, err := tenantStore.ListAvailableModels(ctx, string(provider))
	if err != nil {
		return fmt.Errorf("failed to list available models: %w", err)
	}

	cache := s.buildCacheFromModels(models, provider)
	s.setCache(tenantID, provider, cache)

	slog.Info("Model cache loaded from database",
		"tenant_id", tenantID,
		"provider", provider,
		"model_count", len(cache))

	return nil
}

// LoadAllProvidersFromDatabase loads model cache for all providers for a tenant
func (s *ModelCacheService) LoadAllProvidersFromDatabase(ctx context.Context, tenantStore *postgres.TenantStore, tenantID string) error {
	// Load all models (empty string means all providers)
	models, err := tenantStore.ListAvailableModels(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to list available models: %w", err)
	}

	// Group models by provider
	providerModels := make(map[domain.Provider][]*postgres.AvailableModel)
	for _, model := range models {
		provider, ok := domain.ParseProvider(model.Provider)
		if !ok {
			continue
		}
		providerModels[provider] = append(providerModels[provider], model)
	}

	// Build and set cache for each provider
	for provider, pModels := range providerModels {
		cache := s.buildCacheFromModels(pModels, provider)
		s.setCache(tenantID, provider, cache)

		slog.Debug("Model cache loaded",
			"tenant_id", tenantID,
			"provider", provider,
			"model_count", len(cache))
	}

	slog.Info("All provider model caches loaded from database",
		"tenant_id", tenantID,
		"provider_count", len(providerModels))

	return nil
}

// buildCacheFromModels builds a cache map from database models
func (s *ModelCacheService) buildCacheFromModels(models []*postgres.AvailableModel, provider domain.Provider) map[string]string {
	cache := make(map[string]string)

	for _, model := range models {
		// Use NativeModelID if available, otherwise use ModelID
		nativeID := model.NativeModelID
		if nativeID == "" {
			nativeID = model.ModelID
		}

		// Cache by full model_id (e.g., "bedrock/claude-3-5-sonnet")
		cache[model.ModelID] = nativeID

		// Cache by short name without provider prefix
		switch provider {
		case domain.ProviderBedrock:
			if strings.HasPrefix(model.ModelID, "bedrock/") {
				shortName := strings.TrimPrefix(model.ModelID, "bedrock/")
				cache[shortName] = nativeID
			}
		case domain.ProviderOpenAI:
			if strings.HasPrefix(model.ModelID, "openai/") {
				shortName := strings.TrimPrefix(model.ModelID, "openai/")
				cache[shortName] = nativeID
			}
		case domain.ProviderAnthropic:
			if strings.HasPrefix(model.ModelID, "anthropic/") {
				shortName := strings.TrimPrefix(model.ModelID, "anthropic/")
				cache[shortName] = nativeID
			}
		case domain.ProviderGemini:
			if strings.HasPrefix(model.ModelID, "gemini/") || strings.HasPrefix(model.ModelID, "google/") {
				shortName := strings.TrimPrefix(model.ModelID, "gemini/")
				shortName = strings.TrimPrefix(shortName, "google/")
				cache[shortName] = nativeID
			}
		case domain.ProviderAzureOpenAI:
			if strings.HasPrefix(model.ModelID, "azure/") {
				shortName := strings.TrimPrefix(model.ModelID, "azure/")
				cache[shortName] = nativeID
			}
		case domain.ProviderGroq:
			if strings.HasPrefix(model.ModelID, "groq/") {
				shortName := strings.TrimPrefix(model.ModelID, "groq/")
				cache[shortName] = nativeID
			}
		case domain.ProviderMistral:
			if strings.HasPrefix(model.ModelID, "mistral/") {
				shortName := strings.TrimPrefix(model.ModelID, "mistral/")
				cache[shortName] = nativeID
			}
		case domain.ProviderTogether:
			if strings.HasPrefix(model.ModelID, "together/") {
				shortName := strings.TrimPrefix(model.ModelID, "together/")
				cache[shortName] = nativeID
			}
		case domain.ProviderCohere:
			if strings.HasPrefix(model.ModelID, "cohere/") {
				shortName := strings.TrimPrefix(model.ModelID, "cohere/")
				cache[shortName] = nativeID
			}
		case domain.ProviderOllama:
			if strings.HasPrefix(model.ModelID, "ollama/") {
				shortName := strings.TrimPrefix(model.ModelID, "ollama/")
				cache[shortName] = nativeID
			}
		}

		// Also cache the native model ID itself for direct lookups
		cache[nativeID] = nativeID

		// Cache by model name as well
		if model.ModelName != "" && model.ModelName != model.ModelID {
			cache[model.ModelName] = nativeID
		}
	}

	return cache
}

// setCache sets the cache for a tenant and provider
func (s *ModelCacheService) setCache(tenantID string, provider domain.Provider, cache map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.caches[tenantID]; !ok {
		s.caches[tenantID] = make(map[domain.Provider]map[string]string)
	}
	s.caches[tenantID][provider] = cache
}

// GetCache returns the cache for a tenant and provider
func (s *ModelCacheService) GetCache(tenantID string, provider domain.Provider) map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if tenantCaches, ok := s.caches[tenantID]; ok {
		if cache, ok := tenantCaches[provider]; ok {
			return cache
		}
	}
	return nil
}

// LookupNativeModelID looks up the native model ID for a given model
func (s *ModelCacheService) LookupNativeModelID(tenantID string, provider domain.Provider, modelID string) (string, bool) {
	cache := s.GetCache(tenantID, provider)
	if cache == nil {
		return "", false
	}

	// Try direct lookup
	if nativeID, ok := cache[modelID]; ok {
		return nativeID, true
	}

	// Try without provider prefix
	shortName := ExtractModelID(modelID)
	if nativeID, ok := cache[shortName]; ok {
		return nativeID, true
	}

	return "", false
}

// InvalidateTenantCache removes all caches for a tenant
func (s *ModelCacheService) InvalidateTenantCache(tenantID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.caches, tenantID)
}

// InvalidateProviderCache removes cache for a specific tenant and provider
func (s *ModelCacheService) InvalidateProviderCache(tenantID string, provider domain.Provider) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if tenantCaches, ok := s.caches[tenantID]; ok {
		delete(tenantCaches, provider)
	}
}

// ApplyToClient applies the model cache to a client if it implements ModelCacheable
func (s *ModelCacheService) ApplyToClient(tenantID string, provider domain.Provider, client domain.LLMClient) {
	if cacheable, ok := client.(ModelCacheable); ok {
		cache := s.GetCache(tenantID, provider)
		if cache != nil {
			cacheable.SetModelCache(cache)
		}
	}
}

// RefreshFromModels updates the cache when models are fetched from a provider
func (s *ModelCacheService) RefreshFromModels(tenantID string, provider domain.Provider, models []domain.ModelInfo) {
	cache := make(map[string]string)

	for _, model := range models {
		// Use NativeModelID if available, otherwise use ID
		nativeID := model.NativeModelID
		if nativeID == "" {
			nativeID = model.ID
		}

		// Cache by model ID
		cache[model.ID] = nativeID

		// Cache by short name (without provider prefix)
		shortName := ExtractModelID(model.ID)
		if shortName != model.ID {
			cache[shortName] = nativeID
		}

		// Cache by name
		if model.Name != "" && model.Name != model.ID {
			cache[model.Name] = nativeID
		}

		// Cache native ID itself
		cache[nativeID] = nativeID
	}

	s.setCache(tenantID, provider, cache)

	slog.Info("Model cache refreshed from provider",
		"tenant_id", tenantID,
		"provider", provider,
		"model_count", len(cache))
}
