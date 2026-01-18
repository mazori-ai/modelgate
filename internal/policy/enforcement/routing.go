// Package enforcement provides policy enforcement implementations
package enforcement

import (
	"context"
	"math/rand"
	"modelgate/internal/domain"
	"sync"
	"time"
)

// RoutingEnforcer enforces routing policies per role
type RoutingEnforcer struct {
	mu              sync.RWMutex
	latencyMetrics  map[string]*ProviderLatency // provider:model -> latency
	roundRobinIndex map[string]int              // tenantID:roleID -> index
}

// ProviderLatency tracks latency metrics for a provider/model
type ProviderLatency struct {
	AvgLatencyMs float64
	P50LatencyMs float64
	P95LatencyMs float64
	P99LatencyMs float64
	RequestCount int
	LastUpdated  time.Time
}

// NewRoutingEnforcer creates a new routing enforcer
func NewRoutingEnforcer() *RoutingEnforcer {
	return &RoutingEnforcer{
		latencyMetrics:  make(map[string]*ProviderLatency),
		roundRobinIndex: make(map[string]int),
	}
}

// Route determines the best provider/model based on policy
func (e *RoutingEnforcer) Route(ctx context.Context, policy domain.RoutingPolicy, req *domain.ChatRequest, availableModels []string) (provider string, model string) {
	if !policy.Enabled {
		return "", req.Model
	}

	// If model is explicitly specified and override is allowed, use it
	if req.Model != "" && policy.AllowModelOverride {
		return "", req.Model
	}

	switch policy.Strategy {
	case domain.RoutingStrategyCost:
		return e.routeByCost(ctx, policy, req, availableModels)
	case domain.RoutingStrategyLatency:
		return e.routeByLatency(ctx, policy, req, availableModels)
	case domain.RoutingStrategyWeighted:
		return e.routeByWeight(ctx, policy, req, availableModels)
	case domain.RoutingStrategyRoundRobin:
		return e.routeByRoundRobin(ctx, policy, req, availableModels)
	case domain.RoutingStrategyCapability:
		return e.routeByCapability(ctx, policy, req, availableModels)
	default:
		return "", req.Model
	}
}

// routeByCost routes based on query complexity and model cost
func (e *RoutingEnforcer) routeByCost(ctx context.Context, policy domain.RoutingPolicy, req *domain.ChatRequest, availableModels []string) (string, string) {
	if policy.CostConfig == nil {
		return "", req.Model
	}

	// Estimate query complexity (simplified - could use ML model)
	complexity := e.estimateComplexity(req)

	config := policy.CostConfig

	if complexity < config.SimpleQueryThreshold {
		// Use simple/cheap model
		if len(config.SimpleModels) > 0 {
			return "", config.SimpleModels[rand.Intn(len(config.SimpleModels))]
		}
	} else if complexity > config.ComplexQueryThreshold {
		// Use complex/premium model
		if len(config.ComplexModels) > 0 {
			return "", config.ComplexModels[rand.Intn(len(config.ComplexModels))]
		}
	} else {
		// Use medium model
		if len(config.MediumModels) > 0 {
			return "", config.MediumModels[rand.Intn(len(config.MediumModels))]
		}
	}

	return "", req.Model
}

// routeByLatency routes to lowest latency provider
func (e *RoutingEnforcer) routeByLatency(ctx context.Context, policy domain.RoutingPolicy, req *domain.ChatRequest, availableModels []string) (string, string) {
	if policy.LatencyConfig == nil {
		return "", req.Model
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	// Find model with lowest latency
	var bestModel string
	var bestLatency float64 = -1

	for _, model := range availableModels {
		if metrics, ok := e.latencyMetrics[model]; ok {
			if bestLatency < 0 || metrics.AvgLatencyMs < bestLatency {
				bestLatency = metrics.AvgLatencyMs
				bestModel = model
			}
		}
	}

	if bestModel != "" && (policy.LatencyConfig.MaxLatencyMs == 0 || bestLatency < float64(policy.LatencyConfig.MaxLatencyMs)) {
		return "", bestModel
	}

	// Fall back to preferred models
	if len(policy.LatencyConfig.PreferredModels) > 0 {
		return "", policy.LatencyConfig.PreferredModels[0]
	}

	return "", req.Model
}

// routeByWeight routes based on provider weights
func (e *RoutingEnforcer) routeByWeight(ctx context.Context, policy domain.RoutingPolicy, req *domain.ChatRequest, availableModels []string) (string, string) {
	if policy.WeightedConfig == nil || len(policy.WeightedConfig.Weights) == 0 {
		return "", req.Model
	}

	// Calculate total weight
	totalWeight := 0
	for _, weight := range policy.WeightedConfig.Weights {
		totalWeight += weight
	}

	if totalWeight == 0 {
		return "", req.Model
	}

	// Random selection based on weight
	r := rand.Intn(totalWeight)
	cumulative := 0
	
	for provider, weight := range policy.WeightedConfig.Weights {
		cumulative += weight
		if r < cumulative {
			return provider, ""
		}
	}

	return "", req.Model
}

// routeByRoundRobin routes in round-robin fashion
func (e *RoutingEnforcer) routeByRoundRobin(ctx context.Context, policy domain.RoutingPolicy, req *domain.ChatRequest, availableModels []string) (string, string) {
	if len(availableModels) == 0 {
		return "", req.Model
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// Single-tenant mode: use model as key
	key := req.Model
	idx := e.roundRobinIndex[key]
	e.roundRobinIndex[key] = (idx + 1) % len(availableModels)

	return "", availableModels[idx]
}

// routeByCapability routes based on task type
func (e *RoutingEnforcer) routeByCapability(ctx context.Context, policy domain.RoutingPolicy, req *domain.ChatRequest, availableModels []string) (string, string) {
	if policy.CapabilityConfig == nil || len(policy.CapabilityConfig.TaskModels) == 0 {
		return "", req.Model
	}

	// Detect task type from prompt (simplified)
	taskType := e.detectTaskType(req)

	if models, ok := policy.CapabilityConfig.TaskModels[taskType]; ok && len(models) > 0 {
		return "", models[0]
	}

	return "", req.Model
}

// estimateComplexity estimates query complexity (0.0 - 1.0)
func (e *RoutingEnforcer) estimateComplexity(req *domain.ChatRequest) float64 {
	// Simple heuristics:
	// - Longer prompts = more complex
	// - More messages = more complex
	// - Tool usage = more complex
	
	totalLen := 0
	for _, msg := range req.Messages {
		totalLen += len(msg.Content)
	}

	complexity := 0.0
	
	// Length factor (0-0.4)
	if totalLen > 5000 {
		complexity += 0.4
	} else {
		complexity += float64(totalLen) / 5000 * 0.4
	}

	// Message count factor (0-0.3)
	msgCount := len(req.Messages)
	if msgCount > 20 {
		complexity += 0.3
	} else {
		complexity += float64(msgCount) / 20 * 0.3
	}

	// Tool usage factor (0-0.3)
	if len(req.Tools) > 0 {
		complexity += 0.3
	}

	return complexity
}

// detectTaskType detects the task type from the request
func (e *RoutingEnforcer) detectTaskType(req *domain.ChatRequest) string {
	// Simple keyword-based detection
	// In production, use ML classifier
	
	if len(req.Messages) == 0 {
		return "general"
	}

	// Extract text from last message's content blocks
	lastMsgBlocks := req.Messages[len(req.Messages)-1].Content
	var lastMsg string
	for _, block := range lastMsgBlocks {
		if block.Type == "text" {
			lastMsg += block.Text
		}
	}

	// Check for code-related keywords
	codeKeywords := []string{"code", "function", "class", "debug", "implement", "algorithm"}
	for _, kw := range codeKeywords {
		if containsIgnoreCase(lastMsg, kw) {
			return "code"
		}
	}

	// Check for translation keywords
	translationKeywords := []string{"translate", "translation", "language"}
	for _, kw := range translationKeywords {
		if containsIgnoreCase(lastMsg, kw) {
			return "translation"
		}
	}

	// Check for analysis keywords
	analysisKeywords := []string{"analyze", "analysis", "compare", "evaluate"}
	for _, kw := range analysisKeywords {
		if containsIgnoreCase(lastMsg, kw) {
			return "analysis"
		}
	}

	return "general"
}

// containsIgnoreCase checks if s contains substr (case-insensitive)
func containsIgnoreCase(s, substr string) bool {
	// Simple implementation - in production use strings.Contains with ToLower
	return len(s) >= len(substr) // Placeholder
}

// RecordLatency records latency for a provider/model
func (e *RoutingEnforcer) RecordLatency(provider, model string, latencyMs float64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	key := provider + ":" + model
	
	if metrics, ok := e.latencyMetrics[key]; ok {
		// Running average
		metrics.AvgLatencyMs = (metrics.AvgLatencyMs*float64(metrics.RequestCount) + latencyMs) / float64(metrics.RequestCount+1)
		metrics.RequestCount++
		metrics.LastUpdated = time.Now()
	} else {
		e.latencyMetrics[key] = &ProviderLatency{
			AvgLatencyMs: latencyMs,
			RequestCount: 1,
			LastUpdated:  time.Now(),
		}
	}
}

