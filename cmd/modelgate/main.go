// Package main is the entry point for the ModelGate server.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"modelgate/internal/cache/embedding"
	"modelgate/internal/cache/semantic"
	"modelgate/internal/config"
	"modelgate/internal/crypto"
	"modelgate/internal/gateway"
	httpserver "modelgate/internal/http"
	"modelgate/internal/mcp"
	"modelgate/internal/policy"
	"modelgate/internal/provider"
	"modelgate/internal/resilience"
	"modelgate/internal/responses"
	"modelgate/internal/routing"
	"modelgate/internal/routing/health"
	"modelgate/internal/storage"
	"modelgate/internal/storage/postgres"
	"modelgate/internal/telemetry"
)

// openAIEmbeddingAdapter adapts OpenAI embedder to embedding.EmbeddingClient interface
type openAIEmbeddingAdapter struct {
	embedder *mcp.OpenAIEmbedder
}

func newOpenAIEmbeddingAdapter(apiKey, model string) *openAIEmbeddingAdapter {
	embedder := mcp.NewOpenAIEmbedder(apiKey)
	if model != "" {
		embedder = mcp.NewOpenAIEmbedderWithBaseURL(apiKey, "", model)
	}
	return &openAIEmbeddingAdapter{embedder: embedder}
}

func (a *openAIEmbeddingAdapter) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	// Use batch embedding for efficiency (single API call)
	return a.embedder.EmbedBatch(ctx, texts)
}

// ollamaEmbeddingAdapter adapts Ollama embedder to embedding.EmbeddingClient interface
type ollamaEmbeddingAdapter struct {
	embedder *mcp.OllamaEmbedder
}

func newOllamaEmbeddingAdapter(baseURL, model string) *ollamaEmbeddingAdapter {
	return &ollamaEmbeddingAdapter{
		embedder: mcp.NewOllamaEmbedder(baseURL, model),
	}
}

func (a *ollamaEmbeddingAdapter) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	// Use batch embedding for efficiency
	return a.embedder.EmbedBatch(ctx, texts)
}

func main() {
	// Parse command line flags
	configPath := flag.String("config", "config.toml", "Path to configuration file")
	flag.Parse()

	// Setup structured logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	slog.Info("Starting ModelGate",
		"version", "0.1.0",
		"http_port", cfg.Server.HTTPPort,
	)

	// Initialize telemetry
	metrics, shutdown, err := telemetry.Init(cfg)
	if err != nil {
		slog.Error("Failed to initialize telemetry", "error", err)
		os.Exit(1)
	}
	defer shutdown()

	// Initialize PostgreSQL storage
	var pgStore *postgres.Store
	var memStore *storage.MemoryStore

	if cfg.Database.Driver != "postgres" {
		slog.Error("Only PostgreSQL storage is supported")
		os.Exit(1)
	}

	slog.Info("Initializing PostgreSQL storage",
		"host", cfg.Database.Host,
		"port", cfg.Database.Port,
		"database", cfg.Database.Database,
	)
	pgStore, err = postgres.NewStore(&cfg.Database)
	if err != nil {
		slog.Error("Failed to initialize PostgreSQL storage", "error", err)
		os.Exit(1)
	}
	defer pgStore.Close()

	// Initialize memory store for policy repository (pending migration to PostgreSQL)
	memStore = storage.NewMemoryStore()
	slog.Info("PostgreSQL storage initialized successfully")

	// Initialize provider manager (auto-registers from env vars)
	providerManager, err := provider.NewManager(cfg)
	if err != nil {
		slog.Warn("Provider manager warning", "error", err)
	}

	// Log registered providers
	for _, p := range providerManager.AvailableProviders() {
		slog.Info("Registered provider", "provider", p)
	}

	// Initialize policy engine
	policyEngine := policy.NewEngine(
		memStore.PolicyRepository(),
		pgStore.TenantRepository(),
		policy.DefaultEngineConfig(),
	)

	// Initialize encryption service for API key encryption
	var encryptionService *crypto.EncryptionService
	encryptionKey := os.Getenv("MODELGATE_ENCRYPTION_KEY")
	if encryptionKey != "" {
		var err error
		encryptionService, err = crypto.NewEncryptionServiceFromString(encryptionKey)
		if err != nil {
			slog.Warn("Failed to initialize encryption service, API keys will be stored in plain text", "error", err)
		} else {
			slog.Info("Encryption service initialized", "key_id", encryptionService.KeyID())
		}
	} else {
		slog.Warn("No encryption key configured (MODELGATE_ENCRYPTION_KEY), API keys will be stored in plain text")
	}

	// Initialize semantic caching services
	// 1. Embedding service for semantic similarity
	// Supports both Ollama (default) and OpenAI embedders
	var embeddingClient embedding.EmbeddingClient
	switch cfg.Embedder.Type {
	case "openai":
		if cfg.Embedder.APIKey != "" {
			embeddingClient = newOpenAIEmbeddingAdapter(cfg.Embedder.APIKey, cfg.Embedder.Model)
			slog.Info("Semantic cache: using OpenAI embeddings", "model", cfg.Embedder.Model)
		} else {
			slog.Warn("Semantic cache: OpenAI embedder configured but no API key provided")
		}
	case "ollama":
		baseURL := cfg.Embedder.BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		model := cfg.Embedder.Model
		if model == "" {
			model = "nomic-embed-text"
		}
		embeddingClient = newOllamaEmbeddingAdapter(baseURL, model)
		slog.Info("Semantic cache: using Ollama embeddings", "url", baseURL, "model", model)
	default:
		// Default to Ollama with nomic-embed-text
		embeddingClient = newOllamaEmbeddingAdapter("http://localhost:11434", "nomic-embed-text")
		slog.Info("Semantic cache: using default Ollama embeddings", "model", "nomic-embed-text")
	}
	embeddingService := embedding.NewEmbeddingService(embeddingClient, cfg.Embedder.Model)

	// 2. Semantic cache service (single-tenant mode)
	semanticCacheService := semantic.NewTenantAwareService(pgStore.DB().GetDB(), embeddingService)
	slog.Info("Semantic cache service initialized")

	// Initialize intelligent routing services
	// 1. Health tracker for provider health monitoring
	healthTracker := health.NewTracker(pgStore.DB().GetDB())

	// 2. Router with health tracking
	router := routing.NewRouter(healthTracker)
	slog.Info("Intelligent routing service initialized")

	// Initialize resilience services
	// 1. Circuit breaker
	circuitBreaker := resilience.NewCircuitBreaker(pgStore.DB().GetDB())

	// 2. Resilience service
	resilienceService := resilience.NewService(circuitBreaker)
	slog.Info("Resilience service initialized")

	// Initialize multi-key selector with tenant database provider
	// This function returns the database for a given tenant slug
	getTenantDB := func(tenantSlug string) (*sql.DB, error) {
		tenantStore, err := pgStore.GetTenantStore(tenantSlug)
		if err != nil {
			return nil, err
		}
		return tenantStore.DB().GetDB(), nil
	}

	var keySelector *provider.KeySelector
	if encryptionService != nil {
		keySelector = provider.NewKeySelectorWithEncryption(getTenantDB, encryptionService)
	} else {
		keySelector = provider.NewKeySelector(getTenantDB)
	}
	slog.Info("Multi-key selector initialized", "encryption_enabled", encryptionService != nil)

	// Initialize gateway service with all new services
	gatewayService := gateway.NewServiceWithFeatures(
		cfg,
		providerManager,
		policyEngine,
		pgStore.UsageRepository(),
		pgStore,
		metrics,
		semanticCacheService,
		router,
		healthTracker,
		resilienceService,
		keySelector,
	)

	// Initialize adaptive dispatcher with channel-based queuing
	dispatcherConfig := gateway.DefaultDispatcherConfig()
	// Override from config if needed
	if cfg.Server.MinWorkers > 0 {
		dispatcherConfig.MinWorkers = cfg.Server.MinWorkers
	}
	if cfg.Server.MaxWorkers > 0 {
		dispatcherConfig.MaxWorkers = cfg.Server.MaxWorkers
	}
	if cfg.Server.MaxQueuedRequests > 0 {
		dispatcherConfig.MaxQueuedRequests = cfg.Server.MaxQueuedRequests
	}
	if cfg.Server.ScaleUpThreshold > 0 {
		dispatcherConfig.ScaleUpThreshold = cfg.Server.ScaleUpThreshold
	}
	if cfg.Server.ScaleDownThreshold > 0 {
		dispatcherConfig.ScaleDownThreshold = cfg.Server.ScaleDownThreshold
	}

	dispatcher := gateway.NewDispatcher(dispatcherConfig, gatewayService)
	dispatcher.Start()

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		slog.Info("Received shutdown signal", "signal", sig)
		dispatcher.Stop() // Stop dispatcher first
		cancel()
	}()

	// Initialize MCP Gateway and Server BEFORE starting HTTP server
	// Create embedder based on config for semantic tool search
	var embedder mcp.Embedder
	switch cfg.Embedder.Type {
	case "openai":
		if cfg.Embedder.BaseURL != "" {
			embedder = mcp.NewOpenAIEmbedderWithBaseURL(cfg.Embedder.APIKey, cfg.Embedder.BaseURL, cfg.Embedder.Model)
		} else {
			embedder = mcp.NewOpenAIEmbedder(cfg.Embedder.APIKey)
		}
		slog.Info("Using OpenAI embedder", "model", cfg.Embedder.Model)
	case "ollama":
		embedder = mcp.NewOllamaEmbedder(cfg.Embedder.BaseURL, cfg.Embedder.Model)
		slog.Info("Using Ollama embedder", "url", cfg.Embedder.BaseURL, "model", cfg.Embedder.Model)
	default:
		// Default to Ollama with nomic-embed-text
		embedder = mcp.NewOllamaEmbedder("http://localhost:11434", "nomic-embed-text")
		slog.Info("Using default Ollama embedder", "model", "nomic-embed-text")
	}
	mcpGateway := mcp.NewGateway(embedder)
	mcpServer := mcp.NewMCPServer(mcpGateway, pgStore)

	// Initialize responses service for /v1/responses endpoint (structured outputs)
	// Uses provider manager to dynamically resolve providers based on tenant configuration
	responsesService := responses.NewService(cfg, providerManager, pgStore)

	// Start unified HTTP server (OpenAI API + GraphQL)
	httpAddr := fmt.Sprintf(":%d", cfg.Server.HTTPPort)
	httpServer := httpserver.NewServer(cfg, gatewayService, dispatcher, pgStore, metrics, responsesService)
	// Set MCP Server and Gateway
	httpServer.SetMCPServer(mcpServer)
	httpServer.SetMCPGateway(mcpGateway)
	go func() {
		slog.Info("Starting unified HTTP server",
			"addr", httpAddr,
			"endpoints", []string{"/v1/*", "/graphql", "/mcp", "/metrics"},
		)
		if err := httpServer.Start(ctx, httpAddr); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
			cancel()
		}
	}()

	// Register default tenant store with MCP server
	go func() {
		// Give the system a moment to initialize
		time.Sleep(2 * time.Second)

		// Register default tenant store (single-tenant mode)
		if store, err := pgStore.GetTenantStore("default"); err == nil {
			mcpServer.RegisterTenantStore("default", store)
			mcpGateway.RegisterTenantStore("default", store)
			slog.Debug("Registered default tenant for MCP")
		}
	}()

	slog.Info("ModelGate ready",
		"api_endpoint", fmt.Sprintf("http://localhost:%d/v1", cfg.Server.HTTPPort),
		"graphql_endpoint", fmt.Sprintf("http://localhost:%d/graphql", cfg.Server.HTTPPort),
		"playground", fmt.Sprintf("http://localhost:%d/playground", cfg.Server.HTTPPort),
		"mcp_endpoint", fmt.Sprintf("http://localhost:%d/mcp", cfg.Server.HTTPPort),
	)

	// Wait for shutdown
	<-ctx.Done()
	slog.Info("Shutting down...")

	// Give servers time to complete graceful shutdown
	time.Sleep(2 * time.Second)
	slog.Info("ModelGate stopped")
}
