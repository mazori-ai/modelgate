// Package responses implements the /v1/responses endpoint logic.
package responses

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"modelgate/internal/config"
	"modelgate/internal/domain"
	"modelgate/internal/provider"
	"modelgate/internal/storage/postgres"

	"github.com/google/uuid"
)

// Service handles structured output requests for the /v1/responses endpoint
type Service struct {
	config          *config.Config
	providerManager *provider.Manager
	pgStore         *postgres.Store
	validator       *SchemaValidator
	retryConfig     RetryConfig
}

// RetryConfig defines retry behavior for non-native providers
type RetryConfig struct {
	MaxRetries         int
	RetryableProviders map[domain.Provider]bool
}

// NewService creates a new Responses service
func NewService(cfg *config.Config, providerManager *provider.Manager, pgStore *postgres.Store) *Service {
	return &Service{
		config:          cfg,
		providerManager: providerManager,
		pgStore:         pgStore,
		validator:       NewSchemaValidator(),
		retryConfig: RetryConfig{
			MaxRetries: 3,
			RetryableProviders: map[domain.Provider]bool{
				domain.ProviderAnthropic: true,
				domain.ProviderBedrock:   true,
				domain.ProviderGemini:    true,
				domain.ProviderMistral:   true,
				domain.ProviderOllama:    true,
			},
		},
	}
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

		slog.Debug("Loading provider client for responses",
			"tenant_id", tenantID,
			"provider", providerType,
			"model", model)

		// Create or get cached tenant-specific client
		return s.providerManager.GetOrCreateTenantClient(tenantID, providerType, providerCfg)
	}

	// Fallback to global provider if no tenant context
	return s.providerManager.GetClient(providerType)
}

// GenerateResponse generates a structured response based on provider capabilities
func (s *Service) GenerateResponse(ctx context.Context, req *domain.ResponseRequest) (*domain.StructuredResponse, error) {
	// 1. Get provider client
	providerClient, err := s.getClientForTenant(ctx, "", "", req.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider client: %w", err)
	}

	// 2. Determine strategy based on provider
	strategy := s.getProviderStrategy(providerClient)

	// 3. Generate response based on strategy
	var result *domain.StructuredResponse

	switch strategy {
	case StrategyNative:
		result, err = s.generateNative(ctx, req, providerClient)
	case StrategyJSONMode:
		result, err = s.generateWithJSONMode(ctx, req, providerClient)
	case StrategyPromptBased:
		result, err = s.generateWithPrompt(ctx, req, providerClient)
	default:
		return nil, fmt.Errorf("unknown strategy: %s", strategy)
	}

	if err != nil {
		return nil, err
	}

	// 4. Add metadata
	if result.Metadata == nil {
		result.Metadata = &domain.ResponseMetadata{}
	}
	result.Metadata.Provider = string(providerClient.Provider())
	result.Metadata.ImplementationMode = string(strategy)
	result.Metadata.SchemaValidated = true

	return result, nil
}

// ProviderStrategy represents the implementation approach
type ProviderStrategy string

const (
	StrategyNative      ProviderStrategy = "native"
	StrategyJSONMode    ProviderStrategy = "json_mode"
	StrategyPromptBased ProviderStrategy = "prompt_based"
)

// getProviderStrategy determines which strategy to use for a client
func (s *Service) getProviderStrategy(client domain.LLMClient) ProviderStrategy {
	providerType := client.Provider()

	// Determine strategy based on provider capabilities
	switch providerType {
	case domain.ProviderOpenAI, domain.ProviderAzureOpenAI:
		// Check if provider implements ResponsesCapable interface
		if _, ok := client.(domain.ResponsesCapable); ok {
			return StrategyNative
		}
		// Fallback to JSON mode if native not available
		return StrategyJSONMode

	case domain.ProviderGroq, domain.ProviderTogether, domain.ProviderCohere:
		return StrategyJSONMode

	case domain.ProviderGemini:
		return StrategyJSONMode

	default:
		// Anthropic, Bedrock, Mistral, Ollama use prompt-based
		return StrategyPromptBased
	}
}

// generateNative uses provider's native responses endpoint
func (s *Service) generateNative(ctx context.Context, req *domain.ResponseRequest, client domain.LLMClient) (*domain.StructuredResponse, error) {
	// Check if provider implements ResponsesCapable interface
	responder, ok := client.(domain.ResponsesCapable)
	if !ok {
		return nil, fmt.Errorf("provider does not support native responses API")
	}

	return responder.GenerateResponse(ctx, req)
}

// generateWithJSONMode uses JSON mode + validation
func (s *Service) generateWithJSONMode(ctx context.Context, req *domain.ResponseRequest, client domain.LLMClient) (*domain.StructuredResponse, error) {
	// Convert to chat request with JSON mode enabled
	chatReq := s.convertToJSONModeRequest(req)

	// Call chat completion (non-streaming)
	resp, err := client.ChatComplete(ctx, chatReq)
	if err != nil {
		return nil, fmt.Errorf("chat completion failed: %w", err)
	}

	// Extract and validate JSON
	return s.validateAndParse(resp, req)
}

// generateWithPrompt uses prompt engineering + validation + retry
func (s *Service) generateWithPrompt(ctx context.Context, req *domain.ResponseRequest, client domain.LLMClient) (*domain.StructuredResponse, error) {
	maxRetries := s.retryConfig.MaxRetries
	if !s.retryConfig.RetryableProviders[client.Provider()] {
		maxRetries = 1
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Inject schema into system prompt
		chatReq := s.convertToPromptBasedRequest(req)

		// Call chat completion (non-streaming)
		resp, err := client.ChatComplete(ctx, chatReq)
		if err != nil {
			lastErr = fmt.Errorf("chat completion failed: %w", err)
			continue
		}

		// Try to validate
		result, err := s.validateAndParse(resp, req)
		if err == nil {
			if result.Metadata == nil {
				result.Metadata = &domain.ResponseMetadata{}
			}
			result.Metadata.RetryCount = attempt
			return result, nil
		}

		lastErr = err
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}

// convertToJSONModeRequest creates a chat request with JSON mode
func (s *Service) convertToJSONModeRequest(req *domain.ResponseRequest) *domain.ChatRequest {
	// Inject schema guidance in system prompt
	systemPrompt := s.buildSchemaPrompt(req.ResponseSchema)

	// Check if there's already a system message
	messages := req.Messages
	if len(messages) > 0 && messages[0].Role == "system" {
		// Append to existing system message
		if len(messages[0].Content) > 0 {
			systemPrompt = messages[0].Content[0].Text + "\n\n" + systemPrompt
		}
		messages = messages[1:] // Remove system message, we'll use SystemPrompt field
	}

	return &domain.ChatRequest{
		Model:        req.Model,
		Messages:     messages,
		SystemPrompt: systemPrompt,
		Temperature:  req.Temperature,
		MaxTokens:    req.MaxTokens,
		RequestID:    req.RequestID,
		APIKeyID:     req.APIKeyID,
		RoleID:       req.RoleID,
		GroupID:      req.GroupID,
		AdditionalParams: map[string]any{
			"response_format": map[string]string{"type": "json_object"},
		},
	}
}

// convertToPromptBasedRequest creates a chat request with schema in prompt
func (s *Service) convertToPromptBasedRequest(req *domain.ResponseRequest) *domain.ChatRequest {
	systemPrompt := s.buildSchemaPrompt(req.ResponseSchema)

	// Check if there's already a system message
	messages := req.Messages
	if len(messages) > 0 && messages[0].Role == "system" {
		// Append to existing system message
		if len(messages[0].Content) > 0 {
			systemPrompt = messages[0].Content[0].Text + "\n\n" + systemPrompt
		}
		messages = messages[1:] // Remove system message
	}

	return &domain.ChatRequest{
		Model:        req.Model,
		Messages:     messages,
		SystemPrompt: systemPrompt,
		Temperature:  req.Temperature,
		MaxTokens:    req.MaxTokens,
		RequestID:    req.RequestID,
		APIKeyID:     req.APIKeyID,
		RoleID:       req.RoleID,
		GroupID:      req.GroupID,
	}
}

// buildSchemaPrompt creates a prompt with schema instructions
func (s *Service) buildSchemaPrompt(schema domain.ResponseSchema) string {
	schemaJSON, _ := json.MarshalIndent(schema.Schema, "", "  ")

	prompt := fmt.Sprintf(`You must respond with ONLY valid JSON that strictly conforms to this schema:

Schema Name: %s`, schema.Name)

	if schema.Description != "" {
		prompt += fmt.Sprintf("\nDescription: %s", schema.Description)
	}

	prompt += fmt.Sprintf(`

JSON Schema:
%s

IMPORTANT:
- Output ONLY the JSON object, no additional text
- Follow the schema exactly
- Include all required fields
- Use correct data types
- Do not add fields not in the schema`, string(schemaJSON))

	return prompt
}

// validateAndParse validates the response and parses it into a StructuredResponse
func (s *Service) validateAndParse(chatResp *domain.ChatResponse, req *domain.ResponseRequest) (*domain.StructuredResponse, error) {
	// Get the content from the response
	content := chatResp.Content

	// Parse and validate
	parsedResponse, err := s.validator.ParseAndValidate(content, req.ResponseSchema.Schema)
	if err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Create structured response
	return &domain.StructuredResponse{
		ID:       uuid.New().String(),
		Object:   "response",
		Created:  time.Now().Unix(),
		Model:    chatResp.Model,
		Response: parsedResponse,
		Usage: domain.ResponseUsage{
			PromptTokens:     int(chatResp.Usage.PromptTokens),
			CompletionTokens: int(chatResp.Usage.CompletionTokens),
			TotalTokens:      int(chatResp.Usage.TotalTokens),
		},
	}, nil
}
