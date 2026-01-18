package routing

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"

	"modelgate/internal/domain"
	"modelgate/internal/routing/health"
)

// ProviderConfigSource provides available providers and models for a tenant
type ProviderConfigSource interface {
	GetAvailableProviders(ctx context.Context, tenantID string) ([]string, error)
	GetProviderModels(ctx context.Context, tenantID, provider string) ([]string, error)
}

// DefaultProviderModels contains fallback model lists per provider
var DefaultProviderModels = map[string][]string{
	"openai":    {"gpt-4o", "gpt-4o-mini"},
	"anthropic": {"claude-sonnet-4-20250514", "claude-3-5-haiku-20241022"},
	"gemini":    {"gemini-2.0-flash-exp"},
	"ollama":    {"llama3.2"},
	"bedrock":   {"anthropic.claude-3-5-sonnet-20241022-v2:0"},
	"azure":     {"gpt-4"},
	"groq":      {"llama-3.1-70b-versatile"},
	"mistral":   {"mistral-large-latest"},
	"together":  {"meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo"},
	"cohere":    {"command-r-plus"},
}

// DefaultProviders is the fallback list of available providers
var DefaultProviders = []string{"openai", "anthropic", "gemini", "ollama"}

// Router handles intelligent routing decisions
type Router struct {
	healthTracker  *health.Tracker
	configSource   ProviderConfigSource
	providerCache  map[string][]string // provider -> available models
	mu             sync.RWMutex
	roundRobinIdx  map[string]int // For round-robin strategy
}

// NewRouter creates a new router with default configuration
func NewRouter(healthTracker *health.Tracker) *Router {
	return &Router{
		healthTracker: healthTracker,
		providerCache: make(map[string][]string),
		roundRobinIdx: make(map[string]int),
	}
}

// NewRouterWithConfig creates a new router with a provider configuration source
func NewRouterWithConfig(healthTracker *health.Tracker, configSource ProviderConfigSource) *Router {
	return &Router{
		healthTracker: healthTracker,
		configSource:  configSource,
		providerCache: make(map[string][]string),
		roundRobinIdx: make(map[string]int),
	}
}

// Route selects the best provider and model based on policy
func (r *Router) Route(ctx context.Context, req *domain.ChatRequest, policy domain.RoutingPolicy) (provider, model string, err error) {
	switch policy.Strategy {
	case domain.RoutingStrategyCost:
		return r.routeByCost(ctx, req, policy.CostConfig)
	case domain.RoutingStrategyLatency:
		return r.routeByLatency(ctx, req, policy.LatencyConfig)
	case domain.RoutingStrategyWeighted:
		return r.routeByWeighted(ctx, req, policy.WeightedConfig)
	case domain.RoutingStrategyRoundRobin:
		return r.routeRoundRobin(ctx, req)
	case domain.RoutingStrategyCapability:
		return r.routeByCapability(ctx, req, policy.CapabilityConfig)
	default:
		return "", "", fmt.Errorf("unknown routing strategy: %s", policy.Strategy)
	}
}

// routeByCost analyzes prompt complexity and routes to appropriate tier
func (r *Router) routeByCost(ctx context.Context, req *domain.ChatRequest, config *domain.CostRoutingConfig) (string, string, error) {
	if config == nil {
		return "", "", fmt.Errorf("cost routing config is required")
	}

	// Analyze prompt complexity
	complexity := r.analyzeComplexity(req.Messages, req.Tools)

	var candidates []string
	if complexity < config.SimpleQueryThreshold {
		// Simple query - use cheap models
		candidates = config.SimpleModels
	} else if complexity < config.ComplexQueryThreshold {
		// Medium query - use mid-tier models
		candidates = config.MediumModels
	} else {
		// Complex query - use premium models
		candidates = config.ComplexModels
	}

	if len(candidates) == 0 {
		return "", "", fmt.Errorf("no candidate models for complexity %.2f", complexity)
	}

	// Select best from candidates based on health
	return r.selectBestCandidate(ctx, "", candidates)
}

// analyzeComplexity scores prompt complexity (0.0-1.0)
func (r *Router) analyzeComplexity(messages []domain.Message, tools []domain.Tool) float64 {
	complexity := 0.0

	// Factor 1: Message length and count (30% weight)
	totalChars := 0
	for _, msg := range messages {
		for _, block := range msg.Content {
			if block.Type == "text" {
				totalChars += len(block.Text)
			}
		}
	}

	// Simple heuristic: < 500 chars = simple, > 2000 = complex
	if totalChars < 500 {
		complexity += 0.1
	} else if totalChars < 2000 {
		complexity += 0.2
	} else {
		complexity += 0.3
	}

	// Factor 2: Number of tools (40% weight)
	if len(tools) == 0 {
		complexity += 0.0
	} else if len(tools) <= 5 {
		complexity += 0.2
	} else {
		complexity += 0.4
	}

	// Factor 3: Conversation depth (30% weight)
	if len(messages) <= 2 {
		complexity += 0.1
	} else if len(messages) <= 5 {
		complexity += 0.2
	} else {
		complexity += 0.3
	}

	return complexity
}

// routeByLatency selects provider with best latency
func (r *Router) routeByLatency(ctx context.Context, req *domain.ChatRequest, config *domain.LatencyRoutingConfig) (string, string, error) {
	if config == nil || len(config.PreferredModels) == 0 {
		return "", "", fmt.Errorf("latency routing config is required")
	}

	bestProvider := ""
	bestModel := ""
	bestLatency := float64(999999)

	for _, modelID := range config.PreferredModels {
		provider, model := r.parseModelID(modelID)

		health, err := r.healthTracker.GetHealth(ctx, "", provider, model)
		if err != nil {
			continue
		}

		// Handle new providers with no latency data (assume reasonable default)
		avgLatency := health.AvgLatencyMs
		if avgLatency == 0 && health.TotalRequests == 0 {
			avgLatency = 500 // Default 500ms for new providers
		}

		if avgLatency < bestLatency && avgLatency < float64(config.MaxLatencyMs) {
			bestProvider = provider
			bestModel = model
			bestLatency = avgLatency
		}
	}

	if bestProvider == "" {
		return "", "", fmt.Errorf("no provider meets latency requirement of %dms", config.MaxLatencyMs)
	}

	return bestProvider, bestModel, nil
}

// routeByWeighted distributes requests by configured weights
func (r *Router) routeByWeighted(ctx context.Context, req *domain.ChatRequest, config *domain.WeightedRoutingConfig) (string, string, error) {
	if config == nil || len(config.Weights) == 0 {
		return "", "", fmt.Errorf("weighted routing config is required")
	}

	// Validate weights: must be positive and sum to 100
	totalWeight := 0
	for provider, weight := range config.Weights {
		if weight < 0 {
			return "", "", fmt.Errorf("weight for provider %s cannot be negative: %d", provider, weight)
		}
		totalWeight += weight
	}
	if totalWeight != 100 {
		return "", "", fmt.Errorf("weights must sum to 100, got %d", totalWeight)
	}

	// Select provider by weight
	random := rand.Intn(100)
	cumulative := 0

	// Sort providers for deterministic iteration (maps are unordered)
	providers := make([]string, 0, len(config.Weights))
	for provider := range config.Weights {
		providers = append(providers, provider)
	}

	for _, provider := range providers {
		weight := config.Weights[provider]
		cumulative += weight
		if random < cumulative {
			// Select a model from this provider
			models := r.getProviderModels(ctx, "", provider)
			if len(models) == 0 {
				// This provider has no models, skip and try next
				continue
			}
			return provider, models[0], nil
		}
	}

	return "", "", fmt.Errorf("failed to select provider by weight")
}

// routeRoundRobin cycles through available providers
func (r *Router) routeRoundRobin(ctx context.Context, req *domain.ChatRequest) (string, string, error) {
	providers := r.getAvailableProviders(ctx, "")
	if len(providers) == 0 {
		return "", "", fmt.Errorf("no providers available")
	}

	r.mu.Lock()
	idx := r.roundRobinIdx["default"]
	provider := providers[idx%len(providers)]
	r.roundRobinIdx["default"] = idx + 1
	r.mu.Unlock()

	models := r.getProviderModels(ctx, "", provider)
	if len(models) == 0 {
		return "", "", fmt.Errorf("no models available for provider %s", provider)
	}

	return provider, models[0], nil
}

// routeByCapability routes based on task type
func (r *Router) routeByCapability(ctx context.Context, req *domain.ChatRequest, config *domain.CapabilityRoutingConfig) (string, string, error) {
	if config == nil || len(config.TaskModels) == 0 {
		return "", "", fmt.Errorf("capability routing config is required")
	}

	// Detect task type from prompt
	taskType := r.detectTaskType(req.Messages)

	candidates, ok := config.TaskModels[taskType]
	if !ok || len(candidates) == 0 {
		// Fallback to default task
		candidates, ok = config.TaskModels["default"]
		if !ok {
			return "", "", fmt.Errorf("no models configured for task type: %s", taskType)
		}
	}

	return r.selectBestCandidate(ctx, "", candidates)
}

// detectTaskType detects task type from messages
func (r *Router) detectTaskType(messages []domain.Message) string {
	// Simple keyword-based detection
	text := ""
	for _, msg := range messages {
		for _, block := range msg.Content {
			if block.Type == "text" {
				text += strings.ToLower(block.Text) + " "
			}
		}
	}

	// Keyword matching with priority (more specific first)
	keywords := map[string][]string{
		"code":        {"function", "class", "code", "programming", "debug", "implement", "compile", "syntax"},
		"translation": {"translate", "language", "french", "spanish", "german", "mandarin", "japanese"},
		"creative":    {"write", "story", "poem", "creative", "imagine", "fiction", "narrative"},
		"analysis":    {"analyze", "explain", "summarize", "review", "evaluate", "assess"},
		"math":        {"calculate", "equation", "formula", "mathematical", "compute", "solve"},
	}

	for taskType, words := range keywords {
		for _, word := range words {
			if strings.Contains(text, word) {
				return taskType
			}
		}
	}

	return "default"
}

// selectBestCandidate chooses the healthiest provider from candidates
func (r *Router) selectBestCandidate(ctx context.Context, tenantID string, candidates []string) (string, string, error) {
	if len(candidates) == 0 {
		return "", "", fmt.Errorf("no candidates provided")
	}

	bestProvider := ""
	bestModel := ""
	bestScore := -1.0 // Allow 0 scores to be selected

	for _, modelID := range candidates {
		provider, model := r.parseModelID(modelID)

		health, err := r.healthTracker.GetHealth(ctx, tenantID, provider, model)
		if err != nil {
			continue
		}

		if health.HealthScore > bestScore {
			bestProvider = provider
			bestModel = model
			bestScore = health.HealthScore
		}
	}

	if bestProvider == "" {
		// Fallback to first candidate (no health data available)
		bestProvider, bestModel = r.parseModelID(candidates[0])
	}

	return bestProvider, bestModel, nil
}

// parseModelID parses "provider/model" format
func (r *Router) parseModelID(modelID string) (provider, model string) {
	parts := strings.SplitN(modelID, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", modelID
}

// getAvailableProviders returns list of available providers for tenant
func (r *Router) getAvailableProviders(ctx context.Context, tenantID string) []string {
	// Try to get from config source first
	if r.configSource != nil {
		providers, err := r.configSource.GetAvailableProviders(ctx, tenantID)
		if err == nil && len(providers) > 0 {
			return providers
		}
	}

	// Fallback to defaults
	return DefaultProviders
}

// getProviderModels returns available models for a provider
func (r *Router) getProviderModels(ctx context.Context, tenantID, provider string) []string {
	// Try to get from config source first
	if r.configSource != nil {
		models, err := r.configSource.GetProviderModels(ctx, tenantID, provider)
		if err == nil && len(models) > 0 {
			return models
		}
	}

	// Check local cache
	r.mu.RLock()
	if models, ok := r.providerCache[provider]; ok && len(models) > 0 {
		r.mu.RUnlock()
		return models
	}
	r.mu.RUnlock()

	// Fallback to default models per provider
	if models, ok := DefaultProviderModels[provider]; ok {
		return models
	}

	// Return empty slice instead of nil to prevent nil pointer issues
	return []string{}
}

// SetProviderModels sets available models for a provider (for initialization/caching)
func (r *Router) SetProviderModels(provider string, models []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providerCache[provider] = models
}

// SetConfigSource sets the provider configuration source
func (r *Router) SetConfigSource(source ProviderConfigSource) {
	r.configSource = source
}

// ClearProviderCache clears the cached provider models
func (r *Router) ClearProviderCache() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providerCache = make(map[string][]string)
}
