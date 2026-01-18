// Package provider implements LLM provider clients.
package provider

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"modelgate/internal/config"
	"modelgate/internal/domain"
)

// BuildHTTPClient creates an HTTP client with the specified connection settings
func BuildHTTPClient(settings domain.ConnectionSettings) *http.Client {
	transport := &http.Transport{
		MaxIdleConns:        settings.MaxIdleConnections,
		MaxIdleConnsPerHost: settings.MaxIdleConnections,
		MaxConnsPerHost:     settings.MaxConnections,
		IdleConnTimeout:     time.Duration(settings.IdleTimeoutSec) * time.Second,
		DisableKeepAlives:   !settings.EnableKeepAlive,
		ForceAttemptHTTP2:   settings.EnableHTTP2,
	}

	return &http.Client{
		Timeout:   time.Duration(settings.RequestTimeoutSec) * time.Second,
		Transport: transport,
	}
}

// Provider type constants for external use
const (
	ProviderGemini      = domain.ProviderGemini
	ProviderAnthropic   = domain.ProviderAnthropic
	ProviderOpenAI      = domain.ProviderOpenAI
	ProviderBedrock     = domain.ProviderBedrock
	ProviderOllama      = domain.ProviderOllama
	ProviderAzureOpenAI = domain.ProviderAzureOpenAI
	ProviderGroq        = domain.ProviderGroq
	ProviderMistral     = domain.ProviderMistral
	ProviderTogether    = domain.ProviderTogether
	ProviderCohere      = domain.ProviderCohere
)

// Manager manages multiple LLM provider clients
type Manager struct {
	clients       map[domain.Provider]domain.LLMClient            // Global fallback clients
	tenantClients map[string]map[domain.Provider]domain.LLMClient // Tenant-specific clients
	config        *config.Config
	modelCache    *ModelCacheService // Centralized model cache for all providers
	mu            sync.RWMutex
}

// NewManager creates a new provider manager
func NewManager(cfg *config.Config) (*Manager, error) {
	m := &Manager{
		clients:       make(map[domain.Provider]domain.LLMClient),
		tenantClients: make(map[string]map[domain.Provider]domain.LLMClient),
		config:        cfg,
		modelCache:    NewModelCacheService(),
	}

	// NOTE: Provider clients are now loaded per-tenant from the database (provider_configs table)
	// No global fallback clients are initialized from environment variables
	// Each tenant configures their own provider API keys via the GraphQL API

	return m, nil
}

// GetModelCacheService returns the model cache service
func (m *Manager) GetModelCacheService() *ModelCacheService {
	return m.modelCache
}

// GetOrCreateTenantClient returns a client for a tenant+provider, creating if needed
func (m *Manager) GetOrCreateTenantClient(tenantID string, provider domain.Provider, providerCfg *domain.ProviderConfig) (domain.LLMClient, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if we already have a cached tenant client
	if tenantClients, ok := m.tenantClients[tenantID]; ok {
		if client, ok := tenantClients[provider]; ok {
			return client, nil
		}
	}

	// Get connection settings from provider config
	connSettings := providerCfg.ConnectionSettings
	if connSettings.MaxConnections == 0 {
		connSettings = domain.DefaultConnectionSettings()
	}

	// Create new client based on provider config
	var client domain.LLMClient
	var err error

	switch provider {
	case domain.ProviderGemini:
		if providerCfg.APIKey == "" {
			return nil, fmt.Errorf("Gemini API key not configured for tenant")
		}
		client, err = NewGeminiClient(providerCfg.APIKey, connSettings)

	case domain.ProviderAnthropic:
		if providerCfg.APIKey == "" {
			return nil, fmt.Errorf("Anthropic API key not configured for tenant")
		}
		client, err = NewAnthropicClient(providerCfg.APIKey, connSettings)

	case domain.ProviderOpenAI:
		if providerCfg.APIKey == "" {
			return nil, fmt.Errorf("OpenAI API key not configured for tenant")
		}
		client, err = NewOpenAIClient(providerCfg.APIKey, providerCfg.OrgID, connSettings)

	case domain.ProviderBedrock:
		bedrockCfg := config.BedrockConfig{
			APIKey:          providerCfg.APIKey,
			RegionPrefix:    providerCfg.RegionPrefix,
			ModelsURL:       providerCfg.ModelsURL,
			Region:          providerCfg.Region,
			AccessKeyID:     providerCfg.AccessKeyID,
			SecretAccessKey: providerCfg.SecretAccessKey,
			Profile:         providerCfg.Profile,
		}
		if bedrockCfg.Region == "" {
			bedrockCfg.Region = "us-east-1"
		}
		client, err = NewBedrockClient(bedrockCfg, connSettings)

	case domain.ProviderOllama:
		baseURL := providerCfg.BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		client, err = NewOllamaClient(baseURL, connSettings)

	case domain.ProviderAzureOpenAI:
		if providerCfg.APIKey == "" {
			return nil, fmt.Errorf("Azure OpenAI API key not configured for tenant")
		}
		if providerCfg.BaseURL == "" {
			return nil, fmt.Errorf("Azure OpenAI endpoint not configured for tenant")
		}
		client, err = NewAzureOpenAIClient(AzureOpenAIConfig{
			APIKey:             providerCfg.APIKey,
			Endpoint:           providerCfg.BaseURL,
			APIVersion:         providerCfg.ExtraSettings["api_version"],
			Deployment:         providerCfg.ExtraSettings["deployment"],
			ConnectionSettings: connSettings,
		})

	case domain.ProviderGroq:
		if providerCfg.APIKey == "" {
			return nil, fmt.Errorf("Groq API key not configured for tenant")
		}
		client, err = NewGroqClient(providerCfg.APIKey, connSettings)

	case domain.ProviderMistral:
		if providerCfg.APIKey == "" {
			return nil, fmt.Errorf("Mistral API key not configured for tenant")
		}
		client, err = NewMistralClient(providerCfg.APIKey, connSettings)

	case domain.ProviderTogether:
		if providerCfg.APIKey == "" {
			return nil, fmt.Errorf("Together AI API key not configured for tenant")
		}
		client, err = NewTogetherClient(providerCfg.APIKey, connSettings)

	case domain.ProviderCohere:
		if providerCfg.APIKey == "" {
			return nil, fmt.Errorf("Cohere API key not configured for tenant")
		}
		client, err = NewCohereClient(providerCfg.APIKey, connSettings)

	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create %s client: %w", provider, err)
	}

	// Apply model cache if client supports it
	if m.modelCache != nil {
		m.modelCache.ApplyToClient(tenantID, provider, client)
	}

	// Cache the client
	if _, ok := m.tenantClients[tenantID]; !ok {
		m.tenantClients[tenantID] = make(map[domain.Provider]domain.LLMClient)
	}
	m.tenantClients[tenantID][provider] = client

	return client, nil
}

// InvalidateTenantClients removes all cached clients for a tenant (call when config changes)
func (m *Manager) InvalidateTenantClients(tenantID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tenantClients, tenantID)

	// Also invalidate model cache
	if m.modelCache != nil {
		m.modelCache.InvalidateTenantCache(tenantID)
	}
}

// SetProviderModelCache sets the model cache for a tenant's provider
// The cache maps model IDs to native model IDs (e.g., inference profile IDs for Bedrock)
// This should be called after loading models from the database
func (m *Manager) SetProviderModelCache(tenantID string, provider domain.Provider, cache map[string]string) {
	// Store in centralized cache
	if m.modelCache != nil {
		m.modelCache.setCache(tenantID, provider, cache)
	}

	// Also apply to existing client if it implements ModelCacheable
	m.mu.RLock()
	defer m.mu.RUnlock()

	if tenantClients, ok := m.tenantClients[tenantID]; ok {
		if client, ok := tenantClients[provider]; ok {
			if cacheable, ok := client.(ModelCacheable); ok {
				cacheable.SetModelCache(cache)
			}
		}
	}
}

// SetBedrockModelCache is a convenience method for Bedrock (backwards compatible)
func (m *Manager) SetBedrockModelCache(tenantID string, cache map[string]string) error {
	m.SetProviderModelCache(tenantID, domain.ProviderBedrock, cache)
	return nil
}

// GetClientForTenant returns a client for a specific tenant and provider
// Falls back to global client if tenant doesn't have the provider configured
func (m *Manager) GetClientForTenant(tenantID string, provider domain.Provider, providerCfg *domain.ProviderConfig) (domain.LLMClient, error) {
	// If provider config is provided and has credentials, use tenant-specific client
	if providerCfg != nil && providerCfg.Enabled {
		return m.GetOrCreateTenantClient(tenantID, provider, providerCfg)
	}

	// Fall back to global client
	return m.GetClient(provider)
}

// Register adds a provider client
func (m *Manager) Register(provider domain.Provider, client domain.LLMClient) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clients[provider] = client
	return nil
}

// NewRegistry creates a new empty manager (alias for testing/manual setup)
func NewRegistry() *Manager {
	return &Manager{
		clients: make(map[domain.Provider]domain.LLMClient),
	}
}

// GetClient returns the client for a provider
func (m *Manager) GetClient(provider domain.Provider) (domain.LLMClient, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, ok := m.clients[provider]
	if !ok {
		return nil, fmt.Errorf("provider %s not configured", provider)
	}

	return client, nil
}

// GetClientForModel returns the appropriate client for a model
func (m *Manager) GetClientForModel(model string) (domain.LLMClient, error) {
	provider, ok := m.config.GetProviderForModel(model)
	if !ok {
		// Try to infer from model name
		provider = inferProviderFromModel(model)
		if provider == "" {
			return nil, fmt.Errorf("cannot determine provider for model: %s", model)
		}
	}

	return m.GetClient(provider)
}

// AvailableProviders returns the list of available providers
func (m *Manager) AvailableProviders() []domain.Provider {
	m.mu.RLock()
	defer m.mu.RUnlock()

	providers := make([]domain.Provider, 0, len(m.clients))
	for p := range m.clients {
		providers = append(providers, p)
	}
	return providers
}

// ListAllModels returns all available models from all providers
func (m *Manager) ListAllModels(ctx context.Context) ([]domain.ModelInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var allModels []domain.ModelInfo

	// First, add models from config
	for id, modelCfg := range m.config.Models {
		if modelCfg.Enabled {
			allModels = append(allModels, modelCfg.ToModelInfo(id))
		}
	}

	// Optionally, query providers for their models
	// (This could be expensive, so we might want to cache)
	for _, client := range m.clients {
		models, err := client.ListModels(ctx)
		if err != nil {
			// Log but don't fail
			continue
		}

		// Only add models not already in config
		for _, model := range models {
			found := false
			for _, existing := range allModels {
				if existing.ID == model.ID {
					found = true
					break
				}
			}
			if !found {
				allModels = append(allModels, model)
			}
		}
	}

	return allModels, nil
}

// inferProviderFromModel attempts to infer the provider from model name
func inferProviderFromModel(model string) domain.Provider {
	// Check for provider/model format
	parts := strings.SplitN(model, "/", 2)
	if len(parts) == 2 {
		provider, ok := domain.ParseProvider(parts[0])
		if ok {
			return provider
		}
		// Check for known org prefixes from providers like Together AI
		switch parts[0] {
		case "meta-llama", "mistralai", "Qwen", "deepseek-ai", "togethercomputer":
			return domain.ProviderTogether
		}
	}

	// Try to infer from model name patterns
	modelLower := strings.ToLower(model)

	switch {
	case strings.HasPrefix(modelLower, "gemini"):
		return domain.ProviderGemini
	case strings.HasPrefix(modelLower, "claude"):
		return domain.ProviderAnthropic
	case strings.HasPrefix(modelLower, "gpt") || strings.HasPrefix(modelLower, "o1"):
		return domain.ProviderOpenAI
	case strings.Contains(modelLower, "anthropic.claude"):
		return domain.ProviderBedrock
	// Groq models
	case strings.HasPrefix(modelLower, "llama-3.") && strings.Contains(modelLower, "groq"):
		return domain.ProviderGroq
	case strings.HasPrefix(modelLower, "llama3-groq"):
		return domain.ProviderGroq
	case strings.HasPrefix(modelLower, "mixtral-8x7b"):
		return domain.ProviderGroq
	case strings.HasPrefix(modelLower, "gemma2-") || strings.HasPrefix(modelLower, "gemma-7b"):
		return domain.ProviderGroq
	// Mistral models
	case strings.HasPrefix(modelLower, "mistral-") || strings.HasPrefix(modelLower, "pixtral") || strings.HasPrefix(modelLower, "ministral") || strings.HasPrefix(modelLower, "codestral"):
		return domain.ProviderMistral
	// Cohere models
	case strings.HasPrefix(modelLower, "command-r") || strings.HasPrefix(modelLower, "command"):
		return domain.ProviderCohere
	case strings.HasPrefix(modelLower, "embed-english") || strings.HasPrefix(modelLower, "embed-multilingual"):
		return domain.ProviderCohere
	// Together AI (open source models)
	case strings.Contains(modelLower, "llama") && strings.Contains(modelLower, "instruct"):
		return domain.ProviderTogether
	// Ollama fallback for local models
	case strings.HasPrefix(modelLower, "llama") || strings.HasPrefix(modelLower, "qwen"):
		return domain.ProviderOllama
	}

	return ""
}

// ExtractModelID extracts the model ID from "provider/model" format
func ExtractModelID(fullModel string) string {
	parts := strings.SplitN(fullModel, "/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return fullModel
}
