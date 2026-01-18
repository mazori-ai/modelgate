// Package provider implements LLM provider clients.
//
// AWS BEDROCK IMPLEMENTATION NOTES:
//
// This file is the main entry point for AWS Bedrock support.
// Model-specific implementations are in separate files:
//   - bedrock_anthropic.go - Claude/Anthropic models
//   - bedrock_nova.go - Amazon Nova models (uses ConverseStream API)
//   - bedrock_meta.go - Meta/Llama models
//   - bedrock_mistral.go - Mistral models
//
// AUTHENTICATION OPTIONS:
//  1. IAM Credentials (Access Key + Secret Key) - RECOMMENDED for streaming
//  2. Bearer Token (Long-Term API Key) - For non-streaming or simulated streaming
//
// STREAMING STRATEGY:
//   - ConverseStream API: Used for Nova models (provides usage metrics)
//   - InvokeModelWithResponseStream: Used for Claude (native streaming)
//   - Simulated streaming: Fallback for Bearer token auth
package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/bedrock"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"

	"modelgate/internal/config"
	"modelgate/internal/domain"
)

// BedrockClient is a client for AWS Bedrock
// Supports both Long-Term API Keys (like LLM Gateway) and IAM credentials
type BedrockClient struct {
	// Authentication fields
	apiKey    string // Long-Term API Key (Bearer token)
	accessKey string // IAM Access Key ID
	secretKey string // IAM Secret Access Key

	// Configuration
	regionPrefix string // "us.", "eu.", "global."
	region       string // Full region for IAM auth
	modelsURL    string // Custom models API endpoint

	// Clients
	httpClient      *http.Client           // For Bearer token auth
	runtimeClient   *bedrockruntime.Client // For IAM auth with true streaming
	useSDKStreaming bool                   // True if using IAM auth with AWS SDK

	// Cache
	modelCache map[string]string // Cache of short names to full model IDs
}

// NewBedrockClient creates a new Bedrock client
func NewBedrockClient(cfg config.BedrockConfig, settings ...domain.ConnectionSettings) (*BedrockClient, error) {
	// Use provided connection settings or defaults
	connSettings := domain.DefaultConnectionSettings()
	if len(settings) > 0 {
		connSettings = settings[0]
	}

	client := &BedrockClient{
		httpClient: BuildHTTPClient(connSettings),
		modelCache: make(map[string]string),
		modelsURL:  cfg.ModelsURL,
	}

	// PREFER IAM CREDENTIALS for true streaming support
	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		client.accessKey = cfg.AccessKeyID
		client.secretKey = cfg.SecretAccessKey
		client.region = cfg.Region
		if client.region == "" {
			client.region = "us-east-1"
		}

		// Initialize AWS SDK runtime client for streaming
		ctx := context.Background()

		// Configure HTTP transport for streaming using connection settings
		httpClient := &http.Client{
			Timeout: time.Duration(connSettings.RequestTimeoutSec) * time.Second,
			Transport: &http.Transport{
				DisableKeepAlives:     !connSettings.EnableKeepAlive,
				MaxIdleConns:          connSettings.MaxIdleConnections,
				MaxIdleConnsPerHost:   connSettings.MaxIdleConnections,
				MaxConnsPerHost:       connSettings.MaxConnections,
				IdleConnTimeout:       time.Duration(connSettings.IdleTimeoutSec) * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				ForceAttemptHTTP2:     connSettings.EnableHTTP2,
			},
		}

		awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
			awsconfig.WithRegion(client.region),
			awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				client.accessKey,
				client.secretKey,
				"",
			)),
			awsconfig.WithHTTPClient(httpClient),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config: %w", err)
		}

		client.runtimeClient = bedrockruntime.NewFromConfig(awsCfg)
		client.useSDKStreaming = true
	} else if cfg.APIKey != "" {
		client.apiKey = cfg.APIKey
		client.regionPrefix = cfg.RegionPrefix
		if client.regionPrefix == "" {
			client.regionPrefix = "us."
		}
		client.useSDKStreaming = false
	} else {
		return nil, fmt.Errorf("no credentials provided: need either (AccessKeyID + SecretAccessKey) or APIKey")
	}

	return client, nil
}

// Provider returns the provider type
func (c *BedrockClient) Provider() domain.Provider {
	return domain.ProviderBedrock
}

// SupportsModel checks if a model is supported
func (c *BedrockClient) SupportsModel(model string) bool {
	return strings.HasPrefix(model, "bedrock/") || strings.HasPrefix(model, "aws-bedrock/")
}

// SetModelCache sets the model cache from external source (e.g., database)
func (c *BedrockClient) SetModelCache(cache map[string]string) {
	c.modelCache = cache
}

// GetModelCache returns the current model cache
func (c *BedrockClient) GetModelCache() map[string]string {
	return c.modelCache
}

// mapModelToBedrockID maps LLM Gateway style model names to Bedrock model IDs
func (c *BedrockClient) mapModelToBedrockID(model string) string {
	modelID := model
	if strings.HasPrefix(model, "aws-bedrock/") {
		modelID = strings.TrimPrefix(model, "aws-bedrock/")
	} else if strings.HasPrefix(model, "bedrock/") {
		modelID = strings.TrimPrefix(model, "bedrock/")
	}

	if fullID, ok := c.modelCache[modelID]; ok {
		return fullID
	}

	if strings.Contains(modelID, ".anthropic.") || strings.Contains(modelID, ".meta.") ||
		strings.Contains(modelID, ".amazon.") || strings.Contains(modelID, ".mistral.") ||
		strings.Contains(modelID, ".ai21.") || strings.Contains(modelID, ".cohere.") {
		return modelID
	}

	return modelID
}

// getBedrockEndpoint returns the Bedrock endpoint URL
func (c *BedrockClient) getBedrockEndpoint() string {
	region := c.region
	if region == "" {
		switch c.regionPrefix {
		case "eu.":
			region = "eu-central-1"
		case "global.":
			region = "us-east-1"
		default:
			region = "us-east-1"
		}
	}
	return fmt.Sprintf("https://bedrock-runtime.%s.amazonaws.com", region)
}

// deriveRegion derives the region from region prefix or returns configured region
func (c *BedrockClient) deriveRegion() string {
	if c.region != "" {
		return c.region
	}
	switch c.regionPrefix {
	case "eu.":
		return "eu-central-1"
	case "global.":
		return "us-east-1"
	default:
		return "us-east-1"
	}
}

// ChatStream starts a streaming chat completion
// Routes to the appropriate implementation based on model family
func (c *BedrockClient) ChatStream(ctx context.Context, req *domain.ChatRequest) (<-chan domain.StreamEvent, error) {
	modelID := c.mapModelToBedrockID(req.Model)

	// Route to appropriate streaming implementation
	if isNovaModel(modelID) {
		// Nova uses ConverseStream API for proper usage metrics
		return c.novaConverseStream(ctx, req, modelID)
	} else if isMetaModel(modelID) {
		return c.metaStream(ctx, req, modelID)
	} else if isMistralModel(modelID) {
		return c.mistralStream(ctx, req, modelID)
	} else {
		// Default: Anthropic/Claude
		return c.anthropicStream(ctx, req, modelID)
	}
}

// ChatComplete performs a non-streaming chat completion
func (c *BedrockClient) ChatComplete(ctx context.Context, req *domain.ChatRequest) (*domain.ChatResponse, error) {
	modelID := c.mapModelToBedrockID(req.Model)

	// Route to appropriate completion implementation
	if isNovaModel(modelID) {
		return c.novaComplete(ctx, req, modelID)
	} else if isMetaModel(modelID) {
		return c.metaComplete(ctx, req, modelID)
	} else if isMistralModel(modelID) {
		return c.mistralComplete(ctx, req, modelID)
	} else {
		return c.anthropicComplete(ctx, req, modelID)
	}
}

// Embed generates embeddings
func (c *BedrockClient) Embed(ctx context.Context, model string, texts []string, dimensions *int32) ([][]float32, int64, error) {
	modelID := c.mapModelToBedrockID(model)
	endpoint := c.getBedrockEndpoint()

	var embeddings [][]float32
	var totalTokens int64

	for _, text := range texts {
		reqBody := map[string]interface{}{
			"inputText": text,
		}
		if dimensions != nil {
			reqBody["dimensions"] = *dimensions
		}

		body, _ := json.Marshal(reqBody)
		url := fmt.Sprintf("%s/model/%s/invoke", endpoint, modelID)

		httpReq, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(string(body)))
		if err != nil {
			return nil, 0, err
		}

		httpReq.Header.Set("Content-Type", "application/json")
		if c.apiKey != "" {
			httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
		}

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			return nil, 0, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			return nil, 0, fmt.Errorf("bedrock embed error %d: %s", resp.StatusCode, string(respBody))
		}

		var result struct {
			Embedding   []float32 `json:"embedding"`
			InputTokens int64     `json:"inputTextTokenCount"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, 0, err
		}

		embeddings = append(embeddings, result.Embedding)
		totalTokens += result.InputTokens
	}

	return embeddings, totalTokens, nil
}

// CountTokens counts tokens in a request
func (c *BedrockClient) CountTokens(ctx context.Context, req *domain.ChatRequest) (int32, error) {
	var totalChars int
	for _, msg := range req.Messages {
		for _, content := range msg.Content {
			totalChars += len(content.Text)
		}
	}
	totalChars += len(req.Prompt)
	totalChars += len(req.SystemPrompt)
	return int32(totalChars / 4), nil
}

// ListModels lists available models dynamically from AWS Bedrock API
func (c *BedrockClient) ListModels(ctx context.Context) ([]domain.ModelInfo, error) {
	if c.apiKey != "" {
		if strings.HasPrefix(c.apiKey, "ABSK") {
			return c.listModelsViaREST(ctx)
		}
		parts := strings.Split(c.apiKey, ":")
		if len(parts) == 2 {
			return c.listModelsViaSDK(ctx, parts[0], parts[1])
		}
	}

	if c.accessKey != "" && c.secretKey != "" {
		return c.listModelsViaSDK(ctx, c.accessKey, c.secretKey)
	}

	return nil, fmt.Errorf("no valid credentials configured for Bedrock")
}

// listModelsViaREST uses AWS Bedrock REST API with Bearer token
func (c *BedrockClient) listModelsViaREST(ctx context.Context) ([]domain.ModelInfo, error) {
	url := c.modelsURL
	if url == "" {
		region := c.deriveRegion()
		url = fmt.Sprintf("https://bedrock.%s.amazonaws.com/inference-profiles", region)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		InferenceProfileSummaries []struct {
			InferenceProfileID   string `json:"inferenceProfileId"`
			InferenceProfileName string `json:"inferenceProfileName"`
			Status               string `json:"status"`
		} `json:"inferenceProfileSummaries"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	var models []domain.ModelInfo
	for _, profile := range result.InferenceProfileSummaries {
		if profile.Status != "ACTIVE" {
			continue
		}

		shortName := c.extractShortModelName(profile.InferenceProfileID)
		c.cacheModelVariants(profile.InferenceProfileID)
		capabilities := c.inferModelCapabilities(profile.InferenceProfileID)

		models = append(models, domain.ModelInfo{
			ID:                fmt.Sprintf("bedrock/%s", shortName),
			Name:              profile.InferenceProfileName,
			Provider:          domain.ProviderBedrock,
			NativeModelID:     profile.InferenceProfileID,
			SupportsTools:     capabilities.SupportsTools,
			SupportsReasoning: capabilities.SupportsReasoning,
			ContextLimit:      uint32(capabilities.ContextLimit),
			OutputLimit:       uint32(capabilities.OutputLimit),
			InputCostPer1M:    capabilities.InputCostPer1M,
			OutputCostPer1M:   capabilities.OutputCostPer1M,
			Enabled:           true,
		})
	}

	return models, nil
}

// listModelsViaSDK uses AWS SDK with IAM credentials
func (c *BedrockClient) listModelsViaSDK(ctx context.Context, accessKey, secretKey string) ([]domain.ModelInfo, error) {
	region := c.region
	if region == "" {
		region = c.deriveRegion()
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			accessKey, secretKey, "",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client := bedrock.NewFromConfig(cfg)
	profilesResult, err := client.ListInferenceProfiles(ctx, &bedrock.ListInferenceProfilesInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list inference profiles: %w", err)
	}

	var models []domain.ModelInfo
	for _, profile := range profilesResult.InferenceProfileSummaries {
		if profile.InferenceProfileId == nil {
			continue
		}

		profileID := *profile.InferenceProfileId
		displayName := profileID
		if profile.InferenceProfileName != nil {
			displayName = *profile.InferenceProfileName
		}

		shortName := c.extractShortModelName(profileID)
		c.cacheModelVariants(profileID)
		capabilities := c.inferModelCapabilities(profileID)

		models = append(models, domain.ModelInfo{
			ID:                fmt.Sprintf("bedrock/%s", shortName),
			Name:              displayName,
			Provider:          domain.ProviderBedrock,
			NativeModelID:     profileID,
			SupportsTools:     capabilities.SupportsTools,
			SupportsReasoning: capabilities.SupportsReasoning,
			ContextLimit:      uint32(capabilities.ContextLimit),
			OutputLimit:       uint32(capabilities.OutputLimit),
			InputCostPer1M:    capabilities.InputCostPer1M,
			OutputCostPer1M:   capabilities.OutputCostPer1M,
			Enabled:           true,
		})
	}

	return models, nil
}

// Model family detection helpers
func isAnthropicModel(modelID string) bool {
	return strings.Contains(modelID, "anthropic") || strings.Contains(modelID, "claude")
}

func isNovaModel(modelID string) bool {
	return strings.Contains(modelID, "nova") || strings.Contains(modelID, "amazon.nova")
}

func isMetaModel(modelID string) bool {
	return strings.Contains(modelID, "meta") || strings.Contains(modelID, "llama")
}

func isMistralModel(modelID string) bool {
	return strings.Contains(modelID, "mistral")
}

// Helper functions for model name processing
func (c *BedrockClient) extractShortModelName(fullID string) string {
	parts := strings.Split(fullID, ":")
	base := parts[0]

	if idx := strings.Index(base, "."); idx != -1 {
		base = base[idx+1:]
	}

	base = strings.TrimSuffix(base, "-instruct-v1")
	if idx := strings.LastIndex(base, "-202"); idx != -1 {
		remaining := base[idx+1:]
		if len(remaining) >= 8 && isNumeric(remaining[:8]) {
			base = base[:idx]
		}
	}

	return base
}

func (c *BedrockClient) cacheModelVariants(profileID string) {
	parts := strings.Split(profileID, ":")
	baseWithVersion := parts[0]

	base := baseWithVersion
	if idx := strings.Index(base, "."); idx != -1 {
		base = base[idx+1:]
	}

	var vendor string
	if idx := strings.Index(base, "."); idx != -1 {
		vendor = base[:idx]
	}

	modelWithoutVendor := base
	if idx := strings.Index(base, "."); idx != -1 {
		modelWithoutVendor = base[idx+1:]
	}

	shortName := modelWithoutVendor
	shortName = strings.TrimSuffix(shortName, "-instruct-v1")
	if idx := strings.LastIndex(shortName, "-202"); idx != -1 {
		remaining := shortName[idx+1:]
		if len(remaining) >= 8 && isNumeric(remaining[:8]) {
			shortName = shortName[:idx]
		}
	}

	c.modelCache[shortName] = profileID
	if vendor != "" {
		c.modelCache[vendor+"."+shortName] = profileID
	}
	c.modelCache[base] = profileID
	c.modelCache[baseWithVersion] = profileID
	c.modelCache[profileID] = profileID
}

func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

type modelCapabilities struct {
	SupportsTools     bool
	SupportsReasoning bool
	ContextLimit      int
	OutputLimit       int
	InputCostPer1M    float64
	OutputCostPer1M   float64
}

func (c *BedrockClient) inferModelCapabilities(modelID string) modelCapabilities {
	caps := modelCapabilities{
		ContextLimit:    8192,
		OutputLimit:     4096,
		InputCostPer1M:  1.0,
		OutputCostPer1M: 3.0,
	}

	// Claude models
	if strings.Contains(modelID, "claude-3-5-sonnet") {
		caps.SupportsTools = true
		caps.ContextLimit = 200000
		caps.OutputLimit = 8192
		caps.InputCostPer1M = 3.0
		caps.OutputCostPer1M = 15.0
	} else if strings.Contains(modelID, "claude-3-5-haiku") {
		caps.SupportsTools = true
		caps.ContextLimit = 200000
		caps.OutputLimit = 8192
		caps.InputCostPer1M = 0.25
		caps.OutputCostPer1M = 1.25
	} else if strings.Contains(modelID, "claude-3-7-sonnet") {
		caps.SupportsTools = true
		caps.SupportsReasoning = true
		caps.ContextLimit = 200000
		caps.OutputLimit = 8192
		caps.InputCostPer1M = 3.0
		caps.OutputCostPer1M = 15.0
	} else if strings.Contains(modelID, "claude") {
		caps.SupportsTools = true
		caps.ContextLimit = 100000
		caps.OutputLimit = 4096
		caps.InputCostPer1M = 3.0
		caps.OutputCostPer1M = 15.0
	}

	// Nova models
	if strings.Contains(modelID, "nova-pro") {
		caps.SupportsTools = true
		caps.ContextLimit = 300000
		caps.OutputLimit = 5000
		caps.InputCostPer1M = 0.8
		caps.OutputCostPer1M = 3.2
	} else if strings.Contains(modelID, "nova-lite") {
		caps.SupportsTools = true
		caps.ContextLimit = 300000
		caps.OutputLimit = 5000
		caps.InputCostPer1M = 0.06
		caps.OutputCostPer1M = 0.24
	} else if strings.Contains(modelID, "nova-micro") {
		caps.ContextLimit = 128000
		caps.OutputLimit = 5000
		caps.InputCostPer1M = 0.035
		caps.OutputCostPer1M = 0.14
	}

	// Llama models
	if strings.Contains(modelID, "llama3-2-90b") || strings.Contains(modelID, "llama-3-2-90b") {
		caps.SupportsTools = true
		caps.ContextLimit = 128000
		caps.InputCostPer1M = 2.0
		caps.OutputCostPer1M = 2.0
	} else if strings.Contains(modelID, "llama") {
		caps.ContextLimit = 8192
		caps.InputCostPer1M = 0.5
		caps.OutputCostPer1M = 0.5
	}

	// Mistral models
	if strings.Contains(modelID, "mistral-large") {
		caps.SupportsTools = true
		caps.ContextLimit = 32768
		caps.OutputLimit = 8192
		caps.InputCostPer1M = 4.0
		caps.OutputCostPer1M = 12.0
	} else if strings.Contains(modelID, "mixtral") {
		caps.ContextLimit = 32768
		caps.InputCostPer1M = 0.45
		caps.OutputCostPer1M = 0.7
	}

	return caps
}
