// Package gateway provides the main LLM gateway service.
package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"modelgate/internal/cache/semantic"
	"modelgate/internal/config"
	"modelgate/internal/domain"
	"modelgate/internal/policy"
	"modelgate/internal/provider"
	"modelgate/internal/resilience"
	"modelgate/internal/routing"
	"modelgate/internal/routing/health"
	"modelgate/internal/storage/postgres"
	"modelgate/internal/telemetry"

	"github.com/google/uuid"
)

// Service is the main gateway service
type Service struct {
	config            *config.Config
	providers         *provider.Manager
	policyEngine      domain.PolicyEngine
	policyEnforcement *policy.EnforcementService
	usageRepo         domain.UsageRepository
	pgStore           *postgres.Store
	metrics           *telemetry.Metrics

	// New advanced features
	semanticCache     semantic.CacheService
	router            *routing.Router
	healthTracker     *health.Tracker
	resilienceService *resilience.Service
	keySelector       *provider.KeySelector
}

// NewService creates a new gateway service (backward compatible)
func NewService(
	cfg *config.Config,
	providers *provider.Manager,
	policyEngine domain.PolicyEngine,
	usageRepo domain.UsageRepository,
	pgStore *postgres.Store,
	metrics *telemetry.Metrics,
) *Service {
	return &Service{
		config:            cfg,
		providers:         providers,
		policyEngine:      policyEngine,
		policyEnforcement: policy.NewEnforcementService(),
		usageRepo:         usageRepo,
		pgStore:           pgStore,
		metrics:           metrics,
	}
}

// NewServiceWithFeatures creates a new gateway service with advanced features
func NewServiceWithFeatures(
	cfg *config.Config,
	providers *provider.Manager,
	policyEngine domain.PolicyEngine,
	usageRepo domain.UsageRepository,
	pgStore *postgres.Store,
	metrics *telemetry.Metrics,
	semanticCache semantic.CacheService,
	router *routing.Router,
	healthTracker *health.Tracker,
	resilienceService *resilience.Service,
	keySelector *provider.KeySelector,
) *Service {
	return &Service{
		config:            cfg,
		providers:         providers,
		policyEngine:      policyEngine,
		policyEnforcement: policy.NewEnforcementService(),
		usageRepo:         usageRepo,
		pgStore:           pgStore,
		metrics:           metrics,
		semanticCache:     semanticCache,
		router:            router,
		healthTracker:     healthTracker,
		resilienceService: resilienceService,
		keySelector:       keySelector,
	}
}

// EnforcePolicy validates all policies before allowing an LLM operation
// This is the public method exposed for HTTP/gRPC servers to call
func (s *Service) EnforcePolicy(ctx context.Context, req *domain.ChatRequest, rolePolicy *domain.RolePolicy) error {
	if s.policyEnforcement == nil || rolePolicy == nil {
		return nil
	}

	enfCtx := &policy.EnforcementContext{
		TenantID: "", // Single-tenant mode
		APIKeyID: req.APIKeyID,
		ModelID:  req.Model,
		Messages: req.Messages,
		Tools:    req.Tools,
		RoleID:   req.RoleID,
		GroupID:  req.GroupID,
		Policy:   rolePolicy,
	}

	err := s.policyEnforcement.EnforcePolicy(ctx, enfCtx)

	// If there's a policy violation, record it to the database
	if err != nil {
		if policyViolation, ok := err.(*policy.PolicyViolation); ok {
			// Map violation code to severity (1-5)
			severity := s.getSeverityFromViolation(policyViolation)

			// Record the violation
			s.recordPolicyViolationEvent(
				ctx,
				"", // Single-tenant mode
				req.APIKeyID,
				"", // policyID - can be extracted from rolePolicy if needed
				"", // policyName - can be extracted from rolePolicy if needed
				policyViolation.Code,
				severity,
				policyViolation.Message,
			)
		}
	}

	return err
}

// getSeverityFromViolation maps violation codes to severity levels (1-5)
func (s *Service) getSeverityFromViolation(violation *policy.PolicyViolation) int {
	// Map violation codes to severity levels
	severityMap := map[string]int{
		// Critical violations (5)
		"injection_detected": 5,
		"pii_detected":       5,

		// High violations (4)
		"tool_blocked":      4,
		"model_not_allowed": 4,
		"tools_not_allowed": 4,

		// Medium violations (3)
		"blocked_content":           3,
		"rate_limit_exceeded":       3,
		"token_rate_limit_exceeded": 3,
		"tool_not_allowed":          3,

		// Low violations (2)
		"too_many_tools":    2,
		"too_many_messages": 2,
		"prompt_too_long":   2,
	}

	if severity, ok := severityMap[violation.Code]; ok {
		return severity
	}

	// Default to medium severity
	return 3
}

// getClientForTenant returns a client for the given tenant and model
// This loads provider configuration on-demand from the database per session
// For single-tenant mode, use tenantSlug="default"
func (s *Service) getClientForTenant(ctx context.Context, tenantID string, tenantSlug string, model string) (domain.LLMClient, error) {
	providerType, ok := s.config.GetProviderForModel(model)
	if !ok {
		return nil, fmt.Errorf("unknown provider for model: %s", model)
	}

	// For single-tenant mode, use defaults
	if tenantID == "" {
		tenantID = "default"
	}
	if tenantSlug == "" {
		tenantSlug = "default"
	}

	// Load provider configuration from database on-demand
	if s.pgStore != nil {
		tenantStore, err := s.pgStore.GetTenantStore(tenantSlug)
		if err != nil {
			slog.Error("Failed to get tenant store", "tenant_id", tenantID, "slug", tenantSlug, "error", err)
			return nil, fmt.Errorf("failed to access tenant configuration")
		}

		// Load provider config from database
		providerCfg, err := tenantStore.GetProviderConfig(ctx, providerType)
		if err != nil {
			slog.Error("Failed to load provider config", "tenant_id", tenantID, "provider", providerType, "error", err)
			return nil, fmt.Errorf("provider %s not configured for this tenant", providerType)
		}

		if providerCfg == nil || !providerCfg.Enabled {
			return nil, fmt.Errorf("provider %s is not enabled for this tenant", providerType)
		}

		// Fetch API key from provider_api_keys table (multi-key support)
		if s.keySelector != nil {
			apiKey, err := s.keySelector.SelectKey(ctx, tenantSlug, providerType)
			if err != nil {
				slog.Debug("No API key found for provider", "provider", providerType, "error", err)
				// For Ollama, API key is not required
				if providerType != domain.ProviderOllama {
					return nil, fmt.Errorf("no API key configured for provider %s", providerType)
				}
			} else if apiKey != nil {
				// Populate credentials from the selected key
				providerCfg.APIKey = apiKey.APIKeyDecrypted
				// For Bedrock, also populate IAM credentials if available
				if providerType == domain.ProviderBedrock {
					if apiKey.AccessKeyIDDecrypted != "" {
						providerCfg.AccessKeyID = apiKey.AccessKeyIDDecrypted
					}
					if apiKey.SecretAccessKeyDecrypted != "" {
						providerCfg.SecretAccessKey = apiKey.SecretAccessKeyDecrypted
					}
				}
				slog.Debug("Selected API key for provider",
					"provider", providerType,
					"key_name", apiKey.Name,
					"key_prefix", apiKey.KeyPrefix)
			}
		}

		// Check if model cache is populated, if not load from database
		cacheService := s.providers.GetModelCacheService()
		if cacheService != nil && cacheService.GetCache(tenantID, providerType) == nil {
			// Load model cache from database on first access
			if err := cacheService.LoadFromDatabase(ctx, tenantStore, tenantID, providerType); err != nil {
				slog.Warn("Failed to load model cache from database",
					"tenant_id", tenantID,
					"provider", providerType,
					"error", err)
			}
		}

		// Load available models from database (for model validation)
		availableModels, err := tenantStore.ListAvailableModels(ctx, string(providerType))
		if err == nil && len(availableModels) > 0 {
			modelEnabled := false
			// Strip provider prefix for comparison
			modelToCheck := model
			if strings.HasPrefix(model, "bedrock/") {
				modelToCheck = strings.TrimPrefix(model, "bedrock/")
			} else if strings.HasPrefix(model, "aws-bedrock/") {
				modelToCheck = strings.TrimPrefix(model, "aws-bedrock/")
			}

			for _, am := range availableModels {
				// Check if the model matches (with or without prefix)
				if am.ModelID == model || am.ModelID == modelToCheck ||
					strings.HasSuffix(am.ModelID, "/"+modelToCheck) ||
					strings.Contains(am.ModelID, modelToCheck) {
					if am.IsAvailable && !am.IsDeprecated {
						modelEnabled = true
						break
					}
				}
			}
			if !modelEnabled {
				slog.Warn("Model not found in available models",
					"requested_model", model,
					"model_to_check", modelToCheck,
					"available_count", len(availableModels))
				return nil, fmt.Errorf("model %s is not enabled for this tenant", model)
			}
		}

		slog.Debug("Loading provider client for session",
			"tenant_id", tenantID,
			"provider", providerType,
			"model", model)

		// Create or get cached tenant-specific client
		// The client will automatically receive the model cache from the cache service
		return s.providers.GetOrCreateTenantClient(tenantID, providerType, providerCfg)
	}

	return nil, fmt.Errorf("tenant configuration not available")
}

// LoadModelCacheForTenant loads the model cache for all providers for a tenant
// This should be called when a tenant is accessed for the first time or when models are refreshed
func (s *Service) LoadModelCacheForTenant(ctx context.Context, tenantSlug string) error {
	if s.pgStore == nil {
		return nil
	}

	tenantStore, err := s.pgStore.GetTenantStore(tenantSlug)
	if err != nil {
		return fmt.Errorf("failed to get tenant store: %w", err)
	}

	cacheService := s.providers.GetModelCacheService()
	if cacheService == nil {
		return nil
	}

	return cacheService.LoadAllProvidersFromDatabase(ctx, tenantStore, tenantSlug)
}

// RefreshProviderModels refreshes the model cache for a specific provider
// This should be called when models are fetched from a provider
func (s *Service) RefreshProviderModels(ctx context.Context, tenantID string, provider domain.Provider, models []domain.ModelInfo) {
	cacheService := s.providers.GetModelCacheService()
	if cacheService != nil {
		cacheService.RefreshFromModels(tenantID, provider, models)
	}
}

// ChatStream handles streaming chat completion
// Integrates: semantic caching, intelligent routing, resilience, and health tracking
func (s *Service) ChatStream(ctx context.Context, req *domain.ChatRequest) (<-chan domain.StreamEvent, error) {
	startTime := time.Now()

	// Generate request ID if not set
	if req.RequestID == "" {
		req.RequestID = uuid.New().String()
	}

	// Resolve model alias
	req.Model = s.config.ResolveModel(req.Model)
	originalModel := req.Model

	// Get provider
	providerType, ok := s.config.GetProviderForModel(req.Model)
	if !ok {
		return nil, fmt.Errorf("unknown provider for model: %s", req.Model)
	}

	// Start metrics recording
	var recorder *telemetry.RequestRecorder
	if s.metrics != nil {
		recorder = s.metrics.NewRequestRecorder("ChatStream", req.Model, "", string(providerType))
	}

	// Get role policy for advanced features
	rolePolicy := s.getRolePolicy(ctx, req.RoleID)

	// =========================================================================
	// 1. SEMANTIC CACHE - Check for cached response
	// =========================================================================
	if s.isCacheEnabled(rolePolicy) && rolePolicy.CachingPolicy.CacheStreaming {
		cachedResponse, hit, err := s.semanticCache.Get(
			ctx, req.RoleID, req.Model, req.Messages, rolePolicy.CachingPolicy,
		)
		if err != nil {
			slog.Warn("Semantic cache lookup failed (streaming)", "error", err, "request_id", req.RequestID)
		} else if hit {
			slog.Info("Semantic cache hit (streaming)",
				"request_id", req.RequestID,
				"model", req.Model)

			// Record cache hit metrics and persist to database
			if cachedResponse.Usage != nil {
				tokensSaved := int64(cachedResponse.Usage.PromptTokens + cachedResponse.Usage.CompletionTokens)
				s.recordCacheHitEvent(ctx, "", req.APIKeyID, req.Model, tokensSaved, cachedResponse.CostUSD)
			}

			// Convert cached response to stream events
			return s.convertResponseToStream(cachedResponse, recorder, startTime), nil
		}

		// Record cache miss and persist to database
		s.recordCacheMissEvent(ctx, "", req.APIKeyID, req.Model)
	}

	// =========================================================================
	// 2. INTELLIGENT ROUTING - Select optimal provider/model
	// =========================================================================
	if s.isRoutingEnabled(rolePolicy) {
		routedProvider, routedModel, err := s.router.Route(ctx, req, rolePolicy.RoutingPolicy)
		if err != nil {
			slog.Warn("Routing failed (streaming), using original model",
				"error", err,
				"original_model", req.Model,
				"request_id", req.RequestID)
			// Record routing failure
			if s.metrics != nil {
				s.metrics.RecordRoutingFailure(err.Error(), "")
			}
		} else if routedProvider != "" && routedModel != "" {
			newModel := routedProvider + "/" + routedModel
			// Record routing decision
			if s.metrics != nil {
				s.metrics.RecordRoutingDecision(string(rolePolicy.RoutingPolicy.Strategy), "")
			}
			if newModel != req.Model {
				slog.Info("Routing selected different model (streaming)",
					"original", req.Model,
					"selected", newModel,
					"strategy", rolePolicy.RoutingPolicy.Strategy,
					"request_id", req.RequestID)
				// Record model switch
				if s.metrics != nil {
					s.metrics.RecordModelSwitch(originalModel, newModel, string(rolePolicy.RoutingPolicy.Strategy), "")
				}
				req.Model = newModel
				// Update provider type for the new model
				if newProviderType, ok := s.config.GetProviderForModel(req.Model); ok {
					providerType = newProviderType
				}
			}
		}
	}

	// NOTE: For streaming, we don't do explicit circuit breaker checks or fallbacks
	// Health tracking will inform routing decisions for subsequent requests
	// Policy enforcement is now done at the HTTP/gRPC layer BEFORE reaching gateway
	// The new policy enforcement module (internal/policy/enforcement.go) handles all validation
	// Old ARN-based policy engine is skipped for HTTP/gRPC requests

	// =========================================================================
	// 4. GET CLIENT - Load provider client
	// =========================================================================
	client, err := s.getClientForTenant(ctx, "", "default", req.Model)
	if err != nil {
		if recorder != nil {
			recorder.RecordError("provider_error")
		}
		// Record failure in health tracker
		if s.healthTracker != nil {
			s.healthTracker.RecordFailure(ctx, "", string(providerType), req.Model, "provider_error")
		}
		return nil, fmt.Errorf("getting provider client: %w", err)
	}

	// =========================================================================
	// 5. CALL PROVIDER - Start streaming
	// =========================================================================
	slog.Info("Gateway: Calling provider ChatStream",
		"model", req.Model,
		"tool_count", len(req.Tools),
		"request_id", req.RequestID,
	)
	events, err := client.ChatStream(ctx, req)
	if err != nil {
		if recorder != nil {
			recorder.RecordError("stream_error")
		}
		// Record failure in health tracker
		if s.healthTracker != nil {
			s.healthTracker.RecordFailure(ctx, "", string(providerType), req.Model, "stream_error")
		}
		return nil, err
	}

	// =========================================================================
	// 6. WRAP EVENTS - Buffer response, track metrics, cache on completion
	// =========================================================================
	wrappedEvents := make(chan domain.StreamEvent, 100)
	go func() {
		defer close(wrappedEvents)

		var inputTokens, outputTokens int64
		var costUSD float64

		// Buffer response for caching (if enabled)
		var bufferedContent strings.Builder
		var toolCalls []domain.ToolCall
		shouldCache := s.isCacheEnabled(rolePolicy) && rolePolicy.CachingPolicy.CacheStreaming

		for event := range events {
			// Buffer text chunks for caching
			if textChunk, ok := event.(domain.TextChunk); ok && shouldCache {
				bufferedContent.WriteString(textChunk.Content)
			}

			// Buffer tool call events
			if toolCallEvent, ok := event.(domain.ToolCallEvent); ok {
				toolCalls = append(toolCalls, toolCallEvent.ToolCall)
				// Record tool call and persist to database
				s.recordToolCallEvent(ctx, "", req.APIKeyID, toolCallEvent.ToolCall.Function.Name, req.Model, string(providerType), true, "")
			}

			// Track metrics from usage events
			if usage, ok := event.(domain.UsageEvent); ok {
				inputTokens = int64(usage.PromptTokens)
				outputTokens = int64(usage.CompletionTokens)

				slog.Info("Received UsageEvent (streaming)",
					"model", req.Model,
					"input_tokens", inputTokens,
					"output_tokens", outputTokens,
					"request_id", req.RequestID)

				// Calculate cost
				if modelCfg, ok := s.config.GetModel(req.Model); ok {
					costUSD = modelCfg.CalculateCost(inputTokens, outputTokens)
					usage.CostUSD = costUSD
					event = usage
				}
			}

			// Send event to consumer
			wrappedEvents <- event

			// Handle finish event - cache, track health, record usage
			if finish, ok := event.(domain.FinishEvent); ok {
				latencyMs := time.Since(startTime).Milliseconds()

				slog.Info("Received FinishEvent (streaming)",
					"model", req.Model,
					"input_tokens", inputTokens,
					"output_tokens", outputTokens,
					"cost_usd", costUSD,
					"latency_ms", latencyMs,
					"request_id", req.RequestID,
					"reason", finish.Reason)

				success := finish.Reason == domain.FinishReasonStop || finish.Reason == domain.FinishReasonToolCalls

				if success {
					if recorder != nil {
						recorder.RecordSuccess(inputTokens, outputTokens, costUSD)
					}

					// Record all accumulated tool calls to database
					for _, toolCall := range toolCalls {
						s.recordToolCallEvent(ctx, "", req.APIKeyID, toolCall.Function.Name, req.Model, string(providerType), true, "")
					}

					// =========================================================================
					// 7. SEMANTIC CACHE - Store buffered response
					// =========================================================================
					// Don't cache responses with tool_calls or responses from conversations with tool results
					// Tool results are time-dependent (e.g., get_datetime, read_file, search_web)
					hasToolMessages := false
					for _, msg := range req.Messages {
						if msg.Role == "tool" {
							hasToolMessages = true
							break
						}
					}
					if shouldCache && bufferedContent.Len() > 0 && finish.Reason != domain.FinishReasonToolCalls && !hasToolMessages {
						go func() {
							// Construct response from buffered data
							bufferedResponse := &domain.ChatResponse{
								Content:      bufferedContent.String(),
								ToolCalls:    toolCalls,
								Model:        originalModel,
								FinishReason: finish.Reason,
								Usage: &domain.UsageEvent{
									PromptTokens:     int32(inputTokens),
									CompletionTokens: int32(outputTokens),
									TotalTokens:      int32(inputTokens + outputTokens),
									CostUSD:          costUSD,
								},
								CostUSD:   costUSD,
								LatencyMs: latencyMs,
								Provider:  providerType,
								Cached:    false,
							}

							cacheErr := s.semanticCache.Set(
								context.Background(),
								req.RoleID, originalModel, string(providerType),
								req.Messages, bufferedResponse,
								rolePolicy.CachingPolicy,
							)
							if cacheErr != nil {
								slog.Warn("Failed to cache streaming response", "error", cacheErr, "request_id", req.RequestID)
							} else {
								slog.Debug("Cached streaming response", "request_id", req.RequestID, "content_length", bufferedContent.Len())
							}
						}()
					}

					// =========================================================================
					// 8. HEALTH TRACKING - Record success
					// =========================================================================
					if s.healthTracker != nil {
						s.healthTracker.RecordSuccess(ctx, "", string(providerType), req.Model, int(latencyMs))
					}

					// =========================================================================
					// 9. USAGE TRACKING - Record API usage
					// =========================================================================
					if s.usageRepo != nil {
						s.recordUsage(ctx, req, inputTokens, outputTokens, costUSD, time.Since(startTime), true, "")
					}
				} else if finish.Reason == domain.FinishReasonError {
					if recorder != nil {
						recorder.RecordError("stream_error")
					}

					// Record failure in health tracker
					if s.healthTracker != nil {
						s.healthTracker.RecordFailure(ctx, "", string(providerType), req.Model, "stream_error")
					}

					if s.usageRepo != nil {
						s.recordUsage(ctx, req, inputTokens, outputTokens, costUSD, time.Since(startTime), false, "stream_error")
					}
				}
			}
		}
	}()

	return wrappedEvents, nil
}

// convertResponseToStream converts a cached response into stream events
func (s *Service) convertResponseToStream(response *domain.ChatResponse, recorder *telemetry.RequestRecorder, startTime time.Time) <-chan domain.StreamEvent {
	events := make(chan domain.StreamEvent, 10)

	go func() {
		defer close(events)

		// Send text chunk event
		if response.Content != "" {
			events <- domain.TextChunk{Content: response.Content}
		}

		// Send tool call events
		if len(response.ToolCalls) > 0 {
			for _, toolCall := range response.ToolCalls {
				events <- domain.ToolCallEvent{ToolCall: toolCall}
			}
		}

		// Send usage event
		if response.Usage != nil {
			events <- domain.UsageEvent{
				PromptTokens:     response.Usage.PromptTokens,
				CompletionTokens: response.Usage.CompletionTokens,
				TotalTokens:      response.Usage.TotalTokens,
				CostUSD:          response.CostUSD,
			}
		}

		// Send finish event
		events <- domain.FinishEvent{Reason: response.FinishReason}

		// Record metrics for cached response
		if recorder != nil {
			recorder.RecordSuccess(0, 0, 0) // Cache hit - no new tokens consumed
		}
	}()

	return events
}

// ChatComplete handles non-streaming chat completion
// Integrates: semantic caching, intelligent routing, resilience, and health tracking
func (s *Service) ChatComplete(ctx context.Context, req *domain.ChatRequest) (*domain.ChatResponse, error) {
	startTime := time.Now()

	if req.RequestID == "" {
		req.RequestID = uuid.New().String()
	}

	req.Model = s.config.ResolveModel(req.Model)
	originalModel := req.Model

	providerType, ok := s.config.GetProviderForModel(req.Model)
	if !ok {
		return nil, fmt.Errorf("unknown provider for model: %s", req.Model)
	}

	var recorder *telemetry.RequestRecorder
	if s.metrics != nil {
		recorder = s.metrics.NewRequestRecorder("ChatComplete", req.Model, "", string(providerType))
	}

	// Get role policy for advanced features
	rolePolicy := s.getRolePolicy(ctx, req.RoleID)

	// =========================================================================
	// 1. SEMANTIC CACHE - Check for cached response
	// =========================================================================
	if s.isCacheEnabled(rolePolicy) {
		cachedResponse, hit, err := s.semanticCache.Get(
			ctx, req.RoleID, req.Model, req.Messages, rolePolicy.CachingPolicy,
		)
		if err != nil {
			slog.Warn("Semantic cache lookup failed", "error", err, "request_id", req.RequestID)
		} else if hit {
			slog.Info("Semantic cache hit",
				"request_id", req.RequestID,
				"model", req.Model,
				"tenant_id", "")

			// Mark response as cached
			cachedResponse.Cached = true
			cachedResponse.LatencyMs = time.Since(startTime).Milliseconds()

			// Record cache hit metrics and persist to database
			if cachedResponse.Usage != nil {
				tokensSaved := int64(cachedResponse.Usage.PromptTokens + cachedResponse.Usage.CompletionTokens)
				s.recordCacheHitEvent(ctx, "", req.APIKeyID, req.Model, tokensSaved, cachedResponse.CostUSD)
			}

			if recorder != nil {
				recorder.RecordSuccess(0, 0, 0) // Cache hit - no tokens consumed
			}
			return cachedResponse, nil
		}

		// Record cache miss and persist to database
		s.recordCacheMissEvent(ctx, "", req.APIKeyID, req.Model)
	}

	// =========================================================================
	// 2. INTELLIGENT ROUTING - Select optimal provider/model
	// =========================================================================
	if s.isRoutingEnabled(rolePolicy) {
		routedProvider, routedModel, err := s.router.Route(ctx, req, rolePolicy.RoutingPolicy)
		if err != nil {
			slog.Warn("Routing failed, using original model",
				"error", err,
				"original_model", req.Model,
				"request_id", req.RequestID)
			// Record routing failure
			if s.metrics != nil {
				s.metrics.RecordRoutingFailure(err.Error(), "")
			}
		} else if routedProvider != "" && routedModel != "" {
			newModel := routedProvider + "/" + routedModel
			// Record routing decision
			if s.metrics != nil {
				s.metrics.RecordRoutingDecision(string(rolePolicy.RoutingPolicy.Strategy), "")
			}
			if newModel != req.Model {
				slog.Info("Routing selected different model",
					"original", req.Model,
					"selected", newModel,
					"strategy", rolePolicy.RoutingPolicy.Strategy,
					"request_id", req.RequestID)
				// Record model switch
				if s.metrics != nil {
					s.metrics.RecordModelSwitch(originalModel, newModel, string(rolePolicy.RoutingPolicy.Strategy), "")
				}
				req.Model = newModel
				// Update provider type for the new model
				if newProviderType, ok := s.config.GetProviderForModel(req.Model); ok {
					providerType = newProviderType
				}
			}
		}
	}

	// =========================================================================
	// 3. GET CLIENT - Load provider client
	// =========================================================================
	client, err := s.getClientForTenant(ctx, "", "default", req.Model)
	if err != nil {
		if recorder != nil {
			recorder.RecordError("provider_error")
		}
		return nil, fmt.Errorf("getting provider client: %w", err)
	}

	// =========================================================================
	// 4. EXECUTE WITH RESILIENCE - Retry, circuit breaker, fallback
	// =========================================================================
	slog.Info("Gateway: Calling provider ChatComplete",
		"model", req.Model,
		"tool_count", len(req.Tools),
		"request_id", req.RequestID,
	)
	var response *domain.ChatResponse
	if s.isResilienceEnabled(rolePolicy) {
		// Execute with resilience service
		response, err = s.resilienceService.ExecuteWithResilience(
			ctx,
			"",
			rolePolicy.ResiliencePolicy,
			// Primary execution function
			func(ctx context.Context) (*domain.ChatResponse, error) {
				return client.ChatComplete(ctx, req)
			},
			// Fallback function (called when primary fails and fallback is configured)
			func(ctx context.Context, fallbackProvider, fallbackModel string) (*domain.ChatResponse, error) {
				fallbackClient, err := s.getClientForTenant(ctx, "", "default", fallbackProvider+"/"+fallbackModel)
				if err != nil {
					return nil, err
				}
				// Create a copy of request with fallback model
				fallbackReq := *req
				fallbackReq.Model = fallbackProvider + "/" + fallbackModel
				return fallbackClient.ChatComplete(ctx, &fallbackReq)
			},
		)
	} else {
		// Direct execution without resilience
		response, err = client.ChatComplete(ctx, req)
	}

	// Calculate latency
	latencyMs := time.Since(startTime).Milliseconds()

	// =========================================================================
	// 5. HANDLE ERRORS - Record health metrics on failure
	// =========================================================================
	if err != nil {
		if recorder != nil {
			recorder.RecordError("completion_error")
		}

		// Record failure in health tracker
		if s.healthTracker != nil {
			s.healthTracker.RecordFailure(ctx, "", string(providerType), req.Model, "request_error")
		}

		return nil, err
	}

	// =========================================================================
	// 6. CALCULATE COST
	// =========================================================================
	if response.Usage != nil {
		if modelCfg, ok := s.config.GetModel(req.Model); ok {
			response.CostUSD = modelCfg.CalculateCost(
				int64(response.Usage.PromptTokens),
				int64(response.Usage.CompletionTokens),
			)
		}

		if recorder != nil {
			recorder.RecordSuccess(
				int64(response.Usage.PromptTokens),
				int64(response.Usage.CompletionTokens),
				response.CostUSD,
			)
		}
	}

	// Set response metadata
	response.LatencyMs = latencyMs
	response.Provider = providerType

	// =========================================================================
	// 7. SEMANTIC CACHE - Store response for future use
	// =========================================================================
	// Don't cache responses with tool_calls or responses from conversations with tool results
	// Tool results are time-dependent (e.g., get_datetime, read_file, search_web)
	hasToolMessages := false
	for _, msg := range req.Messages {
		if msg.Role == "tool" {
			hasToolMessages = true
			break
		}
	}
	if s.isCacheEnabled(rolePolicy) && response.FinishReason != domain.FinishReasonToolCalls && !hasToolMessages {
		go func() {
			cacheErr := s.semanticCache.Set(
				context.Background(),
				req.RoleID, originalModel, string(providerType),
				req.Messages, response,
				rolePolicy.CachingPolicy,
			)
			if cacheErr != nil {
				slog.Warn("Failed to cache response", "error", cacheErr, "request_id", req.RequestID)
			}
		}()
	}

	// =========================================================================
	// 8. HEALTH TRACKING - Record success
	// =========================================================================
	if s.healthTracker != nil {
		s.healthTracker.RecordSuccess(ctx, "", string(providerType), req.Model, int(latencyMs))
	}

	// =========================================================================
	// 9. USAGE TRACKING - Record API usage
	// =========================================================================
	if response.Usage != nil && s.usageRepo != nil {
		s.recordUsage(ctx, req,
			int64(response.Usage.PromptTokens),
			int64(response.Usage.CompletionTokens),
			response.CostUSD,
			time.Since(startTime),
			true, "",
		)
	}

	// =========================================================================
	// 10. TOOL CALL TRACKING - Record tool calls to database
	// =========================================================================
	for _, toolCall := range response.ToolCalls {
		s.recordToolCallEvent(ctx, "", req.APIKeyID, toolCall.Function.Name, req.Model, string(providerType), true, "")
	}

	return response, nil
}

// CountTokens counts tokens in a request
func (s *Service) CountTokens(ctx context.Context, req *domain.ChatRequest) (int32, float64, error) {
	req.Model = s.config.ResolveModel(req.Model)

	client, err := s.providers.GetClientForModel(req.Model)
	if err != nil {
		return 0, 0, err
	}

	tokens, err := client.CountTokens(ctx, req)
	if err != nil {
		return 0, 0, err
	}

	// Calculate estimated cost
	var cost float64
	if modelCfg, ok := s.config.GetModel(req.Model); ok {
		cost = (float64(tokens) / 1_000_000.0) * modelCfg.InputCostPer1M
	}

	return tokens, cost, nil
}

// ListModels lists available models
func (s *Service) ListModels(ctx context.Context, provider string) ([]domain.ModelInfo, map[string]string, error) {
	models, err := s.providers.ListAllModels(ctx)
	if err != nil {
		return nil, nil, err
	}

	// Filter by provider if specified
	if provider != "" {
		filtered := []domain.ModelInfo{}
		for _, m := range models {
			if string(m.Provider) == provider {
				filtered = append(filtered, m)
			}
		}
		models = filtered
	}

	return models, s.config.Aliases, nil
}

// ListProviderModels fetches models from a specific provider using tenant configuration
// tenantSlug is used for database lookup, tenantID (optional) is the UUID for filtering
func (s *Service) ListProviderModels(ctx context.Context, tenantSlug string, provider domain.Provider, providerCfg *domain.ProviderConfig) ([]domain.ModelInfo, error) {
	// Get the tenant ID (UUID) from context or database if needed
	var tenantID string
	if tenant, ok := ctx.Value("tenant").(*domain.Tenant); ok && tenant != nil {
		tenantID = tenant.ID
	}

	// Fetch API key from provider_api_keys table (multi-key support)
	if s.keySelector != nil && providerCfg.APIKey == "" {
		apiKey, err := s.keySelector.SelectKey(ctx, tenantSlug, provider)
		if err != nil {
			slog.Debug("No API key found for provider", "provider", provider, "tenant_slug", tenantSlug, "tenant_id", tenantID, "error", err)
			// For Ollama, API key is not required
			if provider != domain.ProviderOllama {
				return nil, fmt.Errorf("no API key configured for provider %s", provider)
			}
		} else if apiKey != nil {
			// Populate credentials from the selected key
			providerCfg.APIKey = apiKey.APIKeyDecrypted
			// For Bedrock, also populate IAM credentials if available
			if provider == domain.ProviderBedrock {
				if apiKey.AccessKeyIDDecrypted != "" {
					providerCfg.AccessKeyID = apiKey.AccessKeyIDDecrypted
				}
				if apiKey.SecretAccessKeyDecrypted != "" {
					providerCfg.SecretAccessKey = apiKey.SecretAccessKeyDecrypted
				}
			}
			slog.Debug("Selected API key for ListProviderModels",
				"provider", provider,
				"key_name", apiKey.Name,
				"key_prefix", apiKey.KeyPrefix)
		}
	}

	// Get or create client for this provider with the tenant's config
	client, err := s.providers.GetOrCreateTenantClient(tenantSlug, provider, providerCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create provider client: %w", err)
	}

	// Fetch models from the provider
	models, err := client.ListModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list models from provider: %w", err)
	}

	return models, nil
}

// Embed generates embeddings
func (s *Service) Embed(ctx context.Context, model string, texts []string, dimensions *int32, tenantID string) ([][]float32, int64, error) {
	model = s.config.ResolveModel(model)

	client, err := s.providers.GetClientForModel(model)
	if err != nil {
		return nil, 0, err
	}

	embeddings, tokens, err := client.Embed(ctx, model, texts, dimensions)
	if err != nil {
		return nil, 0, err
	}

	// Record metrics
	if s.metrics != nil && tenantID != "" {
		providerType, _ := s.config.GetProviderForModel(model)
		s.metrics.TokensInput.WithLabelValues(model, string(providerType), tenantID).Add(float64(tokens))
	}

	return embeddings, tokens, nil
}

// recordUsage records usage to the repository
func (s *Service) recordUsage(
	ctx context.Context,
	req *domain.ChatRequest,
	inputTokens, outputTokens int64,
	costUSD float64,
	latency time.Duration,
	success bool,
	errorCode string,
) {
	providerType, _ := s.config.GetProviderForModel(req.Model)

	// Extract last user message as prompt
	var lastUserMessage string
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			// Concatenate all text content blocks
			for _, block := range req.Messages[i].Content {
				if block.Type == "text" && block.Text != "" {
					lastUserMessage = block.Text
					break
				}
			}
			if lastUserMessage != "" {
				break
			}
		}
	}

	// Create metadata with prompt
	metadata := map[string]any{}
	if lastUserMessage != "" {
		metadata["prompt"] = lastUserMessage
	}

	record := &domain.UsageRecord{
		ID:           uuid.New().String(),
		APIKeyID:     req.APIKeyID,
		RequestID:    req.RequestID,
		Model:        req.Model,
		Provider:     providerType,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  inputTokens + outputTokens,
		CostUSD:      costUSD,
		LatencyMs:    latency.Milliseconds(),
		Success:      success,
		ErrorCode:    errorCode,
		ToolCalls:    int32(len(req.Tools)),
		Metadata:     metadata,
		Timestamp:    time.Now(),
	}

	// Record in background
	go func() {
		_ = s.usageRepo.Record(context.Background(), record)
	}()
}

// GetAllowedModelsForRole filters models based on the role's policy
// This is used to show only models that the API key's role can access
func (s *Service) GetAllowedModelsForRole(ctx context.Context, tenantID, roleID string, availableModels []domain.ModelInfo) ([]domain.ModelInfo, error) {
	if s.policyEngine == nil {
		return availableModels, nil
	}

	// Use the policy engine's GetAllowedModelsForRole method
	if roleFilter, ok := s.policyEngine.(domain.RoleModelFilter); ok {
		return roleFilter.GetAllowedModelsForRole(ctx, tenantID, roleID, availableModels)
	}

	// Fallback: return all models
	return availableModels, nil
}

// InvalidateTenantProviderClients removes all cached provider clients for a tenant
// This should be called when provider configurations are updated
func (s *Service) InvalidateTenantProviderClients(tenantID string) {
	if s.providers != nil {
		s.providers.InvalidateTenantClients(tenantID)
	}
}

// getRolePolicy retrieves the role policy for advanced feature configuration
// Returns nil if policy cannot be loaded (features will be disabled)
func (s *Service) getRolePolicy(ctx context.Context, roleID string) *domain.RolePolicy {
	if s.pgStore == nil || roleID == "" {
		return nil
	}

	policy, err := s.pgStore.GetRolePolicy(ctx, roleID)
	if err != nil {
		slog.Debug("Failed to load role policy, advanced features disabled",
			"role_id", roleID,
			"error", err)
		return nil
	}

	return policy
}

// isCacheEnabled checks if semantic caching is enabled for this request
func (s *Service) isCacheEnabled(policy *domain.RolePolicy) bool {
	return s.semanticCache != nil && policy != nil && policy.CachingPolicy.Enabled
}

// isRoutingEnabled checks if intelligent routing is enabled for this request
func (s *Service) isRoutingEnabled(policy *domain.RolePolicy) bool {
	return s.router != nil && policy != nil && policy.RoutingPolicy.Enabled
}

// isResilienceEnabled checks if resilience features are enabled for this request
func (s *Service) isResilienceEnabled(policy *domain.RolePolicy) bool {
	return s.resilienceService != nil && policy != nil && policy.ResiliencePolicy.Enabled
}

// GetKeySelector returns the key selector service for multi-key management
func (s *Service) GetKeySelector() interface{} {
	return s.keySelector
}
