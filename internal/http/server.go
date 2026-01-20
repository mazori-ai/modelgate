// Package http provides the OpenAI-compatible HTTP API server.
package http

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"modelgate/internal/config"
	"modelgate/internal/domain"
	"modelgate/internal/gateway"
	"modelgate/internal/graphql/generated"
	"modelgate/internal/graphql/resolver"
	"modelgate/internal/mcp"
	"modelgate/internal/policy"
	"modelgate/internal/responses"
	"modelgate/internal/storage/postgres"
	"modelgate/internal/telemetry"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/google/uuid"
)

// MCPServerInterface defines the interface for MCP server
type MCPServerInterface interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

// Server is the HTTP API server (serves both OpenAI API and GraphQL)
type Server struct {
	config               *config.Config
	gateway              *gateway.Service
	dispatcher           *gateway.Dispatcher
	pgStore              *postgres.Store
	store                *postgres.Store // Alias for pgStore for consistency
	metrics              *telemetry.Metrics
	mux                  *http.ServeMux
	toolDiscoveryService *policy.ToolDiscoveryService
	mcpServer            MCPServerInterface
	mcpGateway           *mcp.Gateway
	responsesService     *responses.Service
	graphqlHandler       *handler.Server
	graphqlResolver      *resolver.Resolver
}

// NewServer creates a new unified HTTP server (OpenAI API + GraphQL)
func NewServer(
	cfg *config.Config,
	gw *gateway.Service,
	dispatcher *gateway.Dispatcher,
	pgStore *postgres.Store,
	metrics *telemetry.Metrics,
	responsesService *responses.Service,
) *Server {
	s := &Server{
		config:               cfg,
		gateway:              gw,
		dispatcher:           dispatcher,
		pgStore:              pgStore,
		store:                pgStore,
		metrics:              metrics,
		mux:                  http.NewServeMux(),
		toolDiscoveryService: policy.NewToolDiscoveryService(),
		responsesService:     responsesService,
	}

	// Initialize GraphQL handler
	s.initGraphQL()
	s.setupRoutes()
	return s
}

// initGraphQL initializes the GraphQL handler
func (s *Server) initGraphQL() {
	// Create resolver with all dependencies (database-only auth)
	s.graphqlResolver = resolver.NewResolver(s.config, s.gateway, s.pgStore)

	// Create GraphQL handler
	srv := handler.NewDefaultServer(generated.NewExecutableSchema(generated.Config{
		Resolvers: s.graphqlResolver,
	}))

	// Add transports
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})
	srv.AddTransport(transport.MultipartForm{})

	// Add extensions
	srv.Use(extension.Introspection{})

	s.graphqlHandler = srv
}

// SetMCPGateway sets the MCP gateway for GraphQL resolver
func (s *Server) SetMCPGateway(gateway *mcp.Gateway) {
	s.mcpGateway = gateway
	if s.graphqlResolver != nil {
		s.graphqlResolver.SetMCPGateway(gateway)
	}
}

// setupRoutes configures all HTTP routes (OpenAI API + GraphQL)
func (s *Server) setupRoutes() {
	// =========================================================================
	// OpenAI-compatible API endpoints
	// =========================================================================
	s.mux.HandleFunc("POST /v1/chat/completions", s.withAuthContext(s.handleChatCompletions))
	s.mux.HandleFunc("POST /v1/embeddings", s.withAuth(s.handleEmbeddings))
	s.mux.HandleFunc("GET /v1/models", s.withAuthContext(s.handleListModelsFiltered))
	s.mux.HandleFunc("GET /v1/models/{model}", s.withAuthContext(s.handleGetModelFiltered))

	// Responses API endpoint (structured outputs)
	if s.responsesService != nil {
		s.mux.HandleFunc("POST /v1/responses", s.withAuthContext(s.handleResponses))
	}

	// MCP Gateway endpoint
	if s.mcpServer != nil {
		s.mux.HandleFunc("/mcp", s.handleMCP)
	}

	// Agent Dashboard endpoints
	s.mux.HandleFunc("GET /v1/agents/dashboard/stats", s.withAuthContext(s.handleAgentDashboardStats))
	s.mux.HandleFunc("GET /v1/agents/dashboard/risk", s.withAuthContext(s.handleAgentRiskAssessment))
	s.mux.HandleFunc("GET /v1/agents/list", s.withAuthContext(s.handleListAgents))
	s.mux.HandleFunc("POST /v1/agents/dashboard/violations", s.withAuthContext(s.handleRecordPolicyViolation))

	// =========================================================================
	// GraphQL API endpoints (for Web UI)
	// =========================================================================
	if s.graphqlHandler != nil {
		s.mux.Handle("/graphql", s.withGraphQLAuth(s.graphqlHandler))
		s.mux.Handle("/playground", playground.Handler("ModelGate GraphQL", "/graphql"))
	}

	// =========================================================================
	// Infrastructure endpoints
	// =========================================================================
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("GET /ready", s.handleReady)
	s.mux.HandleFunc("GET /dispatcher/stats", s.handleDispatcherStats)
	s.mux.Handle("GET /metrics", telemetry.Handler())

	// =========================================================================
	// Web UI (static files) - serves from /app/web/dist in Docker
	// =========================================================================
	s.setupStaticFileServer()
}

// setupStaticFileServer serves the web UI static files
func (s *Server) setupStaticFileServer() {
	// Try multiple paths for static files
	staticPaths := []string{
		"/app/web/dist", // Docker path
		"./web/dist",    // Local development path
		"../web/dist",   // Alternative local path
	}

	var staticDir string
	for _, path := range staticPaths {
		if _, err := os.Stat(path); err == nil {
			staticDir = path
			break
		}
	}

	if staticDir == "" {
		slog.Debug("No static files directory found, web UI will not be served")
		return
	}

	slog.Info("Serving web UI static files", "path", staticDir)

	// Create file server
	fs := http.FileServer(http.Dir(staticDir))

	// Serve static files, with SPA fallback to index.html
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Skip API paths
		path := r.URL.Path
		if strings.HasPrefix(path, "/v1/") ||
			strings.HasPrefix(path, "/graphql") ||
			strings.HasPrefix(path, "/playground") ||
			strings.HasPrefix(path, "/mcp") ||
			strings.HasPrefix(path, "/health") ||
			strings.HasPrefix(path, "/ready") ||
			strings.HasPrefix(path, "/metrics") ||
			strings.HasPrefix(path, "/dispatcher") {
			http.NotFound(w, r)
			return
		}

		// Check if file exists
		filePath := staticDir + path
		if _, err := os.Stat(filePath); err == nil {
			fs.ServeHTTP(w, r)
			return
		}

		// SPA fallback: serve index.html for all other routes
		http.ServeFile(w, r, staticDir+"/index.html")
	})
}

// handleMCP delegates to the MCP server
func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request) {
	// Add CORS headers for MCP
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key, X-Tenant")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	s.mcpServer.ServeHTTP(w, r)
}

// SetMCPServer sets the MCP server for handling /mcp requests
func (s *Server) SetMCPServer(mcpServer MCPServerInterface) {
	s.mcpServer = mcpServer
	// Re-setup routes to include MCP
	s.mux = http.NewServeMux()
	s.setupRoutes()
}

// Handler returns the HTTP handler
func (s *Server) Handler() http.Handler {
	return s.corsMiddleware(s.mux)
}

// corsMiddleware adds CORS headers
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// AuthContext contains authentication context for a request
type AuthContext struct {
	Tenant *domain.Tenant
	APIKey *domain.APIKey
}

// withAuth wraps a handler with authentication
func (s *Server) withAuth(handler func(http.ResponseWriter, *http.Request, *domain.Tenant)) http.HandlerFunc {
	return s.withAuthContext(func(w http.ResponseWriter, r *http.Request, auth *AuthContext) {
		handler(w, r, auth.Tenant)
	})
}

// withAuthContext wraps a handler with full authentication context
func (s *Server) withAuthContext(handler func(http.ResponseWriter, *http.Request, *AuthContext)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check for API key or session token
		authHeader := r.Header.Get("Authorization")
		tokenStr := ""
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
		}
		if tokenStr == "" {
			tokenStr = r.Header.Get("X-API-Key")
		}

		auth := &AuthContext{}

		if tokenStr != "" {
			// First try to validate as a session token
			if s.store != nil {
				session, user, err := s.store.GetSessionByToken(r.Context(), tokenStr)
				if err == nil && session != nil && user != nil {
					// Valid session token - create default tenant
					auth.Tenant = &domain.Tenant{
						ID:     "default",
						Name:   "Default",
						Status: domain.TenantStatusActive,
						Tier:   domain.TenantTierFree,
						Metadata: map[string]string{
							"slug": "default",
						},
					}
					// Session token auth doesn't have an API key, but that's OK for dashboard endpoints
					handler(w, r, auth)
					return
				}
			}

			// If session validation failed or no tenant slug, try as API key
			if s.store != nil {
				keyHash := hashAPIKey(tokenStr)
				tenant, apiKey, err := s.store.TenantRepository().GetByAPIKey(r.Context(), keyHash)
				if err != nil {
					// Check if it's the admin token
					if s.config.Server.AuthToken != "" && tokenStr == s.config.Server.AuthToken {
						// Admin access - create a synthetic tenant
						auth.Tenant = &domain.Tenant{
							ID:     "default",
							Name:   "Default",
							Status: domain.TenantStatusActive,
							Tier:   domain.TenantTierFree,
						}
					} else {
						s.writeError(w, http.StatusUnauthorized, "unauthorized", "Invalid API key or session token")
						return
					}
				} else {
					auth.Tenant = tenant
					auth.APIKey = apiKey
				}
			}
		} else if s.config.Server.AuthToken != "" {
			// Auth is required but no token provided
			s.writeError(w, http.StatusUnauthorized, "unauthorized", "API key or session token required")
			return
		}

		handler(w, r, auth)
	}
}

// withGraphQLAuth wraps GraphQL handler with authentication context
func (s *Server) withGraphQLAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Extract request info for audit
		ipAddress := r.Header.Get("X-Forwarded-For")
		if ipAddress == "" {
			ipAddress = r.Header.Get("X-Real-IP")
		}
		if ipAddress == "" {
			ipAddress = r.RemoteAddr
		}
		ctx = context.WithValue(ctx, resolver.ContextKeyIPAddress, ipAddress)
		ctx = context.WithValue(ctx, resolver.ContextKeyUserAgent, r.Header.Get("User-Agent"))

		// Single-tenant mode - always use "default" tenant
		tenantSlug := "default"
		ctx = context.WithValue(ctx, resolver.ContextKeyTenant, tenantSlug)

		// Create default tenant object
		defaultTenant := &domain.Tenant{
			ID:     "default",
			Name:   "Default",
			Status: domain.TenantStatusActive,
			Tier:   domain.TenantTierFree,
			Metadata: map[string]string{
				"slug": "default",
			},
		}
		ctx = context.WithValue(ctx, "tenant", defaultTenant)

		// Extract auth token
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			ctx = context.WithValue(ctx, resolver.ContextKeyToken, token)

			// Validate session from database
			if s.pgStore != nil {
				session, user, err := s.pgStore.GetSessionByToken(ctx, token)
				if err == nil && session != nil && user != nil {
					domainUser := &domain.User{
						ID:    user.ID,
						Email: user.Email,
						Name:  user.Name,
						Role:  domain.UserRole(user.Role),
					}
					ctx = context.WithValue(ctx, resolver.ContextKeyUser, domainUser)
					ctx = context.WithValue(ctx, resolver.ContextKeyUserEmail, user.Email)
				}
			}
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// enforcePoliciesForRequest loads and enforces policies for a chat request
// SECURE BY DEFAULT: Blocks all requests unless policies are successfully loaded and validated
// Returns a ToolPolicyResult with any removed tools (for response headers)
func (s *Server) enforcePoliciesForRequest(ctx context.Context, req *domain.ChatRequest, auth *AuthContext) (*ToolPolicyResult, error) {
	// SECURITY: Require authentication
	if auth.Tenant == nil {
		return nil, &policy.PolicyViolation{
			Code:    "authentication_required",
			Message: "Tenant authentication required",
			Type:    "auth",
		}
	}

	if auth.APIKey == nil {
		return nil, &policy.PolicyViolation{
			Code:    "api_key_required",
			Message: "API key authentication required",
			Type:    "auth",
		}
	}

	// SECURITY: Require store access
	if s.pgStore == nil {
		return nil, &policy.PolicyViolation{
			Code:    "policy_store_unavailable",
			Message: "Policy enforcement unavailable",
			Type:    "system",
		}
	}

	// Get tenant store (single-tenant mode)
	tenantStore := s.pgStore.TenantStore()

	// Collect all role policies (direct role + group roles)
	var rolePolicies []*domain.RolePolicy
	var policyLoadErrors []string

	// Get direct role policy
	if auth.APIKey.RoleID != "" {
		role, err := tenantStore.GetRole(ctx, auth.APIKey.RoleID)
		if err != nil {
			policyLoadErrors = append(policyLoadErrors, fmt.Sprintf("failed to load role %s: %v", auth.APIKey.RoleID, err))
		} else if role == nil {
			policyLoadErrors = append(policyLoadErrors, fmt.Sprintf("role %s not found", auth.APIKey.RoleID))
		} else if role.Policy != nil {
			rolePolicies = append(rolePolicies, role.Policy)
		}
	}

	// Get group role policies
	if auth.APIKey.GroupID != "" {
		groupRoles, err := tenantStore.GetGroupRoles(ctx, auth.APIKey.GroupID)
		if err != nil {
			policyLoadErrors = append(policyLoadErrors, fmt.Sprintf("failed to load group roles for %s: %v", auth.APIKey.GroupID, err))
		} else {
			for _, role := range groupRoles {
				if role.Policy != nil {
					rolePolicies = append(rolePolicies, role.Policy)
				}
			}
		}
	}

	// SECURITY: API key must have at least a role OR group assigned
	if auth.APIKey.RoleID == "" && auth.APIKey.GroupID == "" {
		return nil, &policy.PolicyViolation{
			Code:    "no_role_assigned",
			Message: "API key must be assigned to a role or group",
			Type:    "auth",
		}
	}

	// SECURITY: Must successfully load at least one policy
	if len(rolePolicies) == 0 {
		if len(policyLoadErrors) > 0 {
			return nil, &policy.PolicyViolation{
				Code:    "policy_load_failed",
				Message: fmt.Sprintf("Failed to load policies: %s", strings.Join(policyLoadErrors, "; ")),
				Type:    "system",
			}
		}
		return nil, &policy.PolicyViolation{
			Code:    "no_policy_configured",
			Message: "No policy configured for this API key",
			Type:    "auth",
		}
	}

	// Enforce each policy (any violation blocks the request)
	for _, rolePolicy := range rolePolicies {
		if err := s.gateway.EnforcePolicy(ctx, req, rolePolicy); err != nil {
			return nil, err
		}
	}

	// SECURITY: Enforce tool policy if request contains tools
	var toolResult *ToolPolicyResult
	if len(req.Tools) > 0 && auth.APIKey.RoleID != "" {
		var err error
		toolResult, err = s.enforceToolPolicy(ctx, req, auth, tenantStore)
		if err != nil {
			return nil, err
		}
	}

	return toolResult, nil
}

// ToolPolicyResult stores the result of tool policy enforcement for response headers
type ToolPolicyResult struct {
	RemovedTools []string // Names of tools that were stripped from request
}

// enforceToolPolicy discovers tools and checks if they are allowed for the role
// Returns a ToolPolicyResult with any removed tools (for response headers) and an error if blocked
func (s *Server) enforceToolPolicy(ctx context.Context, req *domain.ChatRequest, auth *AuthContext, tenantStore *postgres.TenantStore) (*ToolPolicyResult, error) {
	result := &ToolPolicyResult{
		RemovedTools: []string{},
	}

	// Get role policy to check tool settings
	role, err := tenantStore.GetRole(ctx, auth.APIKey.RoleID)
	if err != nil {
		slog.Warn("Failed to get role for tool policy", "role_id", auth.APIKey.RoleID, "error", err)
		return result, nil // Allow if we can't load role
	}

	// If tool calling is explicitly disabled, block
	if role != nil && role.Policy != nil && !role.Policy.ToolPolicies.AllowToolCalling {
		return nil, &policy.PolicyViolation{
			Code:    "tool_calling_disabled",
			Message: "Tool calling is disabled for this role",
			Type:    "tool",
		}
	}

	// Discover tools and register them for this role
	discoveredTools, err := s.toolDiscoveryService.DiscoverToolsForRole(
		ctx,
		auth.APIKey.RoleID,
		auth.APIKey.ID,
		req.Tools,
		tenantStore,
	)
	if err != nil {
		slog.Error("Failed to discover tools", "error", err)
		// Non-blocking: continue if discovery fails
	}

	// Check permissions for discovered tools
	if len(discoveredTools) > 0 {
		// Get the tool policy from role
		toolPolicies := domain.DefaultEnhancedToolPolicies()
		if role != nil && role.Policy != nil {
			// Check if requireToolApproval is set - if so, use BLOCK as default
			if role.Policy.ToolPolicies.RequireToolApproval {
				toolPolicies.DefaultAction = "BLOCK"
			}
		}

		permResult, err := s.toolDiscoveryService.CheckToolPermissions(
			ctx,
			auth.APIKey.RoleID,
			discoveredTools,
			toolPolicies,
			tenantStore,
		)
		if err != nil {
			slog.Error("Failed to check tool permissions", "error", err)
			// Secure by default: block on error
			return nil, &policy.PolicyViolation{
				Code:    "tool_permission_error",
				Message: "Failed to verify tool permissions",
				Type:    "tool",
			}
		}

		// Handle REMOVED tools - filter them from req.Tools
		removedTools := permResult.RemovedTools()
		if len(removedTools) > 0 {
			removedToolNames := make(map[string]bool)
			for _, t := range removedTools {
				removedToolNames[t.ToolName] = true
				result.RemovedTools = append(result.RemovedTools, t.ToolName)

				slog.Info("Tool removed from request",
					"tool_name", t.ToolName,
					"tool_id", t.ToolID,
					"reason", t.Reason,
					"role_id", auth.APIKey.RoleID,
					"tenant_id", auth.Tenant.ID,
				)

				// Log execution attempt as REMOVED
				tenantStore.LogToolExecution(ctx, &domain.ToolExecutionLog{
					ID:          uuid.New().String(),
					RoleToolID:  t.ToolID,
					ToolName:    t.ToolName,
					RoleID:      auth.APIKey.RoleID,
					APIKeyID:    auth.APIKey.ID,
					RequestID:   req.RequestID,
					Status:      "REMOVED",
					BlockReason: t.Reason,
					Model:       req.Model,
				})
			}

			// Filter removed tools from the request
			filteredTools := make([]domain.Tool, 0, len(req.Tools))
			for _, tool := range req.Tools {
				if tool.Function.Name != "" && !removedToolNames[tool.Function.Name] {
					filteredTools = append(filteredTools, tool)
				}
			}
			req.Tools = filteredTools

			slog.Info("Filtered tools from request",
				"original_count", len(req.Tools)+len(removedTools),
				"remaining_count", len(req.Tools),
				"removed", result.RemovedTools,
			)
		}

		// Check if request should be blocked (DENIED or PENDING with default deny)
		if !permResult.Allowed {
			// Build list of blocked tools for error message
			blocked := permResult.BlockedTools()
			toolNames := make([]string, len(blocked))
			for i, t := range blocked {
				toolNames[i] = t.ToolName
			}

			// Log the blocked tools
			for _, t := range blocked {
				slog.Info("Tool blocked by policy",
					"tool_name", t.ToolName,
					"tool_id", t.ToolID,
					"reason", t.Reason,
					"role_id", auth.APIKey.RoleID,
					"tenant_id", auth.Tenant.ID,
				)

				// Log execution attempt
				tenantStore.LogToolExecution(ctx, &domain.ToolExecutionLog{
					ID:          uuid.New().String(),
					RoleToolID:  t.ToolID,
					ToolName:    t.ToolName,
					RoleID:      auth.APIKey.RoleID,
					APIKeyID:    auth.APIKey.ID,
					RequestID:   req.RequestID,
					Status:      "BLOCKED",
					BlockReason: t.Reason,
					Model:       req.Model,
				})
			}

			return nil, &policy.PolicyViolation{
				Code:    "tool_not_allowed",
				Message: fmt.Sprintf("The following tools are not allowed: %s. Contact your administrator to approve them.", strings.Join(toolNames, ", ")),
				Type:    "tool",
			}
		}

		// Log allowed tool executions
		for _, t := range permResult.AllowedTools() {
			tenantStore.LogToolExecution(ctx, &domain.ToolExecutionLog{
				ID:         uuid.New().String(),
				RoleToolID: t.ToolID,
				ToolName:   t.ToolName,
				RoleID:     auth.APIKey.RoleID,
				APIKeyID:   auth.APIKey.ID,
				Status:     "ALLOWED",
				Model:      req.Model,
			})
		}
	}

	return result, nil
}

// writePolicyViolationError writes a policy violation error in OpenAI error format
func (s *Server) writePolicyViolationError(w http.ResponseWriter, err error) {
	policyViolation, ok := err.(*policy.PolicyViolation)
	if !ok {
		s.writeError(w, http.StatusForbidden, "policy_violation", err.Error())
		return
	}

	// Map policy type to HTTP status code
	statusCode := http.StatusForbidden
	switch policyViolation.Type {
	case "rate_limit":
		statusCode = http.StatusTooManyRequests
	case "model":
		statusCode = http.StatusForbidden
	case "prompt", "tool":
		statusCode = http.StatusBadRequest
	case "auth":
		statusCode = http.StatusUnauthorized // 401 for authentication failures
	case "system":
		statusCode = http.StatusServiceUnavailable // 503 for system errors
	}

	s.writeJSON(w, statusCode, ErrorResponse{
		Error: ErrorDetail{
			Message: policyViolation.Message,
			Type:    policyViolation.Code,
			Code:    policyViolation.Code,
		},
	})
}

// recordPolicyViolation creates a usage record for policy-blocked requests
// This ensures that blocked requests appear in the request logs for visibility
func (s *Server) recordPolicyViolation(ctx context.Context, req *domain.ChatRequest, auth *AuthContext, err error, startTime time.Time) {
	slog.Debug("Recording policy violation", "model", req.Model, "error", err)

	// Extract policy violation details
	policyViolation, ok := err.(*policy.PolicyViolation)
	if !ok {
		slog.Debug("Not a policy violation error")
		return
	}

	// Require database access
	if s.pgStore == nil {
		slog.Warn("Cannot record policy violation: pgStore is nil")
		return
	}

	slog.Info("Recording policy violation",
		"model", req.Model,
		"api_key", req.APIKeyID,
		"code", policyViolation.Code,
		"message", policyViolation.Message)

	// Get tenant store (single-tenant mode)
	tenantStore := s.pgStore.TenantStore()

	// Determine provider from model
	provider := domain.Provider("unknown")
	if s.config != nil {
		if p, ok := s.config.GetProviderForModel(req.Model); ok {
			provider = p
		}
	}

	// Generate request ID if not set
	requestID := req.RequestID
	if requestID == "" {
		requestID = uuid.New().String()
	}

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

	// Create usage record for the blocked request
	record := &domain.UsageRecord{
		ID:           uuid.New().String(),
		APIKeyID:     req.APIKeyID,
		RequestID:    requestID,
		Model:        req.Model,
		Provider:     provider,
		InputTokens:  0, // Request never processed
		OutputTokens: 0,
		TotalTokens:  0,
		CostUSD:      0,
		LatencyMs:    time.Since(startTime).Milliseconds(),
		Success:      false,
		ErrorCode:    policyViolation.Code,
		ErrorMessage: policyViolation.Message,
		ToolCalls:    0,
		Metadata:     metadata,
		Timestamp:    time.Now(),
	}

	slog.Debug("Created usage record", "record_id", record.ID, "request_id", requestID)

	// Store the record
	if recordErr := tenantStore.RecordUsage(ctx, record); recordErr != nil {
		slog.Error("Failed to record usage", "error", recordErr)
		return
	}

	slog.Info("Successfully recorded policy violation", "record_id", record.ID)
}

// handleChatCompletions handles POST /v1/chat/completions
func (s *Server) handleChatCompletions(w http.ResponseWriter, r *http.Request, auth *AuthContext) {
	startTime := time.Now()

	var req ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	// Convert to domain request
	domainReq := s.convertChatRequest(&req)
	// Pass API key and role/group info for RBAC policy enforcement
	if auth.APIKey != nil {
		domainReq.APIKeyID = auth.APIKey.ID
		domainReq.RoleID = auth.APIKey.RoleID
		domainReq.GroupID = auth.APIKey.GroupID
	}

	// Enforce policies before processing request
	toolResult, err := s.enforcePoliciesForRequest(r.Context(), domainReq, auth)
	if err != nil {
		// Record policy violation in usage logs for visibility
		s.recordPolicyViolation(r.Context(), domainReq, auth, err, startTime)
		s.writePolicyViolationError(w, err)
		return
	}

	// Add headers for removed tools (if any)
	if toolResult != nil && len(toolResult.RemovedTools) > 0 {
		w.Header().Set("X-ModelGate-Removed-Tools", strings.Join(toolResult.RemovedTools, ","))
		w.Header().Set("X-ModelGate-Warning", fmt.Sprintf("%d tool(s) removed from request", len(toolResult.RemovedTools)))
	}

	// If dispatcher is available, use it for backpressure and queuing
	if s.dispatcher != nil {
		s.handleChatCompletionsWithDispatcher(w, r, domainReq, &req, auth)
		return
	}

	// Fallback to direct processing (no backpressure)
	if req.Stream {
		s.handleStreamingResponse(w, r, domainReq, &req)
	} else {
		s.handleNonStreamingResponse(w, r, domainReq, &req)
	}
}

// handleChatCompletionsWithDispatcher uses the dispatcher for backpressure
func (s *Server) handleChatCompletionsWithDispatcher(w http.ResponseWriter, r *http.Request, domainReq *domain.ChatRequest, req *ChatCompletionRequest, auth *AuthContext) {
	// Determine priority from role policy
	priority := s.getPriorityForRequest(r.Context(), auth)

	// Create dispatch request
	dispatchReq := &gateway.DispatchRequest{
		Ctx:        r.Context(),
		ChatReq:    domainReq,
		TenantID:   "", // Single-tenant mode
		TenantSlug: "default",
		APIKeyID:   domainReq.APIKeyID,
		RoleID:     domainReq.RoleID,
		GroupID:    domainReq.GroupID,
		Priority:   priority,
	}

	// Submit to dispatcher
	result, err := s.dispatcher.Submit(r.Context(), dispatchReq)
	if err != nil {
		if err == gateway.ErrQueueFull {
			// Backpressure: server is overloaded
			w.Header().Set("Retry-After", "5")
			s.writeError(w, http.StatusServiceUnavailable, "overloaded",
				"Server is overloaded, please retry after a few seconds")
			return
		}
		if err == gateway.ErrQueueTimeout {
			w.Header().Set("Retry-After", "10")
			s.writeError(w, http.StatusServiceUnavailable, "queue_timeout",
				"Request timed out waiting in queue")
			return
		}
		if err == gateway.ErrShuttingDown {
			s.writeError(w, http.StatusServiceUnavailable, "shutting_down",
				"Server is shutting down")
			return
		}
		s.writeError(w, http.StatusInternalServerError, "dispatch_error", err.Error())
		return
	}

	// Handle the result
	if req.Stream {
		if result.Error != nil {
			s.writeError(w, http.StatusInternalServerError, "stream_error", result.Error.Error())
			return
		}
		s.handleStreamingResponseFromEvents(w, r, result.EventsCh, req)
	} else {
		if result.Error != nil {
			s.writeError(w, http.StatusInternalServerError, "completion_error", result.Error.Error())
			return
		}
		s.handleNonStreamingResponseFromResult(w, result.Response, req)
	}
}

// getPriorityForRequest determines request priority from role policy
func (s *Server) getPriorityForRequest(ctx context.Context, auth *AuthContext) int {
	// Default priority
	priority := 5

	if auth.APIKey == nil || s.pgStore == nil {
		return priority
	}

	// Get role policy to check for priority settings (single-tenant mode)
	tenantStore := s.pgStore.TenantStore()

	// Get role policy
	rolePolicy, err := tenantStore.GetRolePolicy(ctx, auth.APIKey.RoleID)
	if err != nil || rolePolicy == nil {
		return priority
	}

	// Use concurrency policy priority if configured
	if rolePolicy.ConcurrencyPolicy.Enabled && rolePolicy.ConcurrencyPolicy.Priority > 0 {
		priority = rolePolicy.ConcurrencyPolicy.Priority
		if priority > 10 {
			priority = 10
		}
	}

	return priority
}

// handleStreamingResponseFromEvents handles streaming from dispatcher result
func (s *Server) handleStreamingResponseFromEvents(w http.ResponseWriter, r *http.Request, events <-chan domain.StreamEvent, req *ChatCompletionRequest) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeError(w, http.StatusInternalServerError, "server_error", "Streaming not supported")
		return
	}

	rc := http.NewResponseController(w)

	id := fmt.Sprintf("chatcmpl-%s", uuid.New().String())
	created := time.Now().Unix()
	chunkCount := 0

	// Set initial write deadline
	if err := rc.SetWriteDeadline(time.Now().Add(30 * time.Minute)); err != nil {
		slog.Warn("Failed to set write deadline", "error", err)
	}

	// Send initial chunk with role
	if err := s.writeSSEChunk(w, flusher, ChatCompletionChunk{
		ID:      id,
		Object:  "chat.completion.chunk",
		Created: created,
		Model:   req.Model,
		Choices: []ChunkChoice{{
			Index: 0,
			Delta: Delta{
				Role: stringPtr("assistant"),
			},
		}},
	}); err != nil {
		slog.Error("Failed to write initial SSE chunk", "error", err)
		return
	}

	for event := range events {
		chunkCount++

		if chunkCount%50 == 0 {
			if err := rc.SetWriteDeadline(time.Now().Add(30 * time.Minute)); err != nil {
				slog.Warn("Failed to extend write deadline", "error", err, "chunk", chunkCount)
			}
		}

		var writeErr error

		switch e := event.(type) {
		case domain.TextChunk:
			writeErr = s.writeSSEChunk(w, flusher, ChatCompletionChunk{
				ID:      id,
				Object:  "chat.completion.chunk",
				Created: created,
				Model:   req.Model,
				Choices: []ChunkChoice{{
					Index: 0,
					Delta: Delta{
						Content: stringPtr(e.Content),
					},
				}},
			})

		case domain.ToolCallEvent:
			argsJSON, _ := json.Marshal(e.ToolCall.Function.Arguments)
			writeErr = s.writeSSEChunk(w, flusher, ChatCompletionChunk{
				ID:      id,
				Object:  "chat.completion.chunk",
				Created: created,
				Model:   req.Model,
				Choices: []ChunkChoice{{
					Index: 0,
					Delta: Delta{
						ToolCalls: []ToolCall{{
							ID:   e.ToolCall.ID,
							Type: "function",
							Function: &FunctionCall{
								Name:      e.ToolCall.Function.Name,
								Arguments: string(argsJSON),
							},
						}},
					},
				}},
			})

		case domain.FinishEvent:
			reason := "stop"
			if e.Reason == domain.FinishReasonToolCalls {
				reason = "tool_calls"
			} else if e.Reason == domain.FinishReasonLength {
				reason = "length"
			} else if e.Reason == domain.FinishReasonError {
				reason = "error"
			}
			writeErr = s.writeSSEChunk(w, flusher, ChatCompletionChunk{
				ID:      id,
				Object:  "chat.completion.chunk",
				Created: created,
				Model:   req.Model,
				Choices: []ChunkChoice{{
					Index:        0,
					Delta:        Delta{},
					FinishReason: stringPtr(reason),
				}},
			})

		case domain.PolicyViolationEvent:
			writeErr = s.writeSSEChunk(w, flusher, ChatCompletionChunk{
				ID:      id,
				Object:  "chat.completion.chunk",
				Created: created,
				Model:   req.Model,
				Choices: []ChunkChoice{{
					Index: 0,
					Delta: Delta{
						Content: stringPtr(fmt.Sprintf("Policy Violation: %s", e.Message)),
					},
				}},
			})
		}

		if writeErr != nil {
			slog.Error("Failed to write SSE chunk", "error", writeErr, "chunk", chunkCount)
			for range events {
				// Drain remaining events
			}
			return
		}
	}

	// Send done marker
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

// handleNonStreamingResponseFromResult handles non-streaming from dispatcher result
func (s *Server) handleNonStreamingResponseFromResult(w http.ResponseWriter, resp *domain.ChatResponse, req *ChatCompletionRequest) {
	if resp == nil {
		s.writeError(w, http.StatusInternalServerError, "no_response", "No response received")
		return
	}

	// Build message
	msg := ChatMessage{
		Role:    "assistant",
		Content: resp.Content,
	}

	// Handle tool calls
	if len(resp.ToolCalls) > 0 {
		for _, tc := range resp.ToolCalls {
			argsJSON, _ := json.Marshal(tc.Function.Arguments)
			msg.ToolCalls = append(msg.ToolCalls, ToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: &FunctionCall{
					Name:      tc.Function.Name,
					Arguments: string(argsJSON),
				},
			})
		}
	}

	// Determine finish reason
	reason := "stop"
	if resp.FinishReason == domain.FinishReasonToolCalls {
		reason = "tool_calls"
	} else if resp.FinishReason == domain.FinishReasonLength {
		reason = "length"
	}

	// Build response
	response := ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-%s", uuid.New().String()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []Choice{{
			Index:        0,
			Message:      msg,
			FinishReason: reason,
		}},
	}

	// Add usage if available
	if resp.Usage != nil {
		response.Usage = &Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleDispatcherStats returns dispatcher statistics
func (s *Server) handleDispatcherStats(w http.ResponseWriter, r *http.Request) {
	if s.dispatcher == nil {
		s.writeError(w, http.StatusNotFound, "not_configured", "Dispatcher not configured")
		return
	}

	stats := s.dispatcher.Stats()
	_, maxConcurrent, queued, maxQueued := s.dispatcher.Capacity()

	response := map[string]interface{}{
		"healthy": s.dispatcher.IsHealthy(),
		"workers": map[string]interface{}{
			"current":     stats.CurrentWorkers,
			"max":         maxConcurrent,
			"scaled_up":   stats.WorkersScaledUp,
			"scaled_down": stats.WorkersScaledDown,
		},
		"queues": map[string]interface{}{
			"total":           queued,
			"max":             maxQueued,
			"utilization_pct": float64(queued) / float64(maxQueued) * 100,
			"high_priority":   stats.HighPriorityQueueDepth,
			"normal_priority": stats.NormalPriorityQueueDepth,
			"low_priority":    stats.LowPriorityQueueDepth,
		},
		"requests": map[string]interface{}{
			"received":  stats.RequestsReceived,
			"queued":    stats.RequestsQueued,
			"processed": stats.RequestsProcessed,
			"rejected":  stats.RequestsRejected,
			"timed_out": stats.RequestsTimedOut,
		},
		"timing_ms": map[string]interface{}{
			"avg_queue_wait":  s.dispatcher.AvgQueueWaitMs(),
			"avg_processing":  s.dispatcher.AvgProcessingMs(),
			"max_queue_wait":  stats.MaxQueueWaitMs,
			"max_processing":  stats.MaxProcessingMs,
			"last_queue_wait": stats.LastQueueWaitMs,
			"last_processing": stats.LastProcessingMs,
		},
	}

	// Include tenant stats if requested
	tenantID := r.URL.Query().Get("tenant")
	if tenantID != "" {
		current, limit := s.dispatcher.TenantStats(tenantID)
		response["tenant"] = map[string]interface{}{
			"id":               tenantID,
			"current_requests": current,
			"limit":            limit,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleStreamingResponse handles SSE streaming
func (s *Server) handleStreamingResponse(w http.ResponseWriter, r *http.Request, domainReq *domain.ChatRequest, req *ChatCompletionRequest) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeError(w, http.StatusInternalServerError, "server_error", "Streaming not supported")
		return
	}

	// Use ResponseController to extend write deadlines for long-running SSE streams
	// This prevents "i/o timeout" errors when the WriteTimeout is exceeded
	rc := http.NewResponseController(w)

	events, err := s.gateway.ChatStream(r.Context(), domainReq)
	if err != nil {
		s.writeSSEError(w, flusher, err)
		return
	}

	id := fmt.Sprintf("chatcmpl-%s", uuid.New().String())
	created := time.Now().Unix()
	chunkCount := 0

	// Extend the write deadline for the entire streaming response
	// Set to 30 minutes to handle very long responses
	if err := rc.SetWriteDeadline(time.Now().Add(30 * time.Minute)); err != nil {
		slog.Warn("Failed to set write deadline", "error", err)
	}

	// Send initial chunk with role
	if err := s.writeSSEChunk(w, flusher, ChatCompletionChunk{
		ID:      id,
		Object:  "chat.completion.chunk",
		Created: created,
		Model:   req.Model,
		Choices: []ChunkChoice{{
			Index: 0,
			Delta: Delta{
				Role: stringPtr("assistant"),
			},
		}},
	}); err != nil {
		slog.Error("Failed to write initial SSE chunk", "error", err)
		return
	}

	for event := range events {
		chunkCount++

		// Extend write deadline every 50 chunks to prevent timeout during long streams
		if chunkCount%50 == 0 {
			if err := rc.SetWriteDeadline(time.Now().Add(30 * time.Minute)); err != nil {
				slog.Warn("Failed to extend write deadline", "error", err, "chunk", chunkCount)
			}
		}

		var writeErr error

		switch e := event.(type) {
		case domain.TextChunk:
			writeErr = s.writeSSEChunk(w, flusher, ChatCompletionChunk{
				ID:      id,
				Object:  "chat.completion.chunk",
				Created: created,
				Model:   req.Model,
				Choices: []ChunkChoice{{
					Index: 0,
					Delta: Delta{
						Content: stringPtr(e.Content),
					},
				}},
			})

		case domain.ToolCallEvent:
			argsJSON, _ := json.Marshal(e.ToolCall.Function.Arguments)
			writeErr = s.writeSSEChunk(w, flusher, ChatCompletionChunk{
				ID:      id,
				Object:  "chat.completion.chunk",
				Created: created,
				Model:   req.Model,
				Choices: []ChunkChoice{{
					Index: 0,
					Delta: Delta{
						ToolCalls: []ToolCall{{
							ID:   e.ToolCall.ID,
							Type: "function",
							Function: &FunctionCall{
								Name:      e.ToolCall.Function.Name,
								Arguments: string(argsJSON),
							},
						}},
					},
				}},
			})

		case domain.FinishEvent:
			reason := "stop"
			if e.Reason == domain.FinishReasonToolCalls {
				reason = "tool_calls"
			} else if e.Reason == domain.FinishReasonLength {
				reason = "length"
			} else if e.Reason == domain.FinishReasonError {
				reason = "error"
			}
			writeErr = s.writeSSEChunk(w, flusher, ChatCompletionChunk{
				ID:      id,
				Object:  "chat.completion.chunk",
				Created: created,
				Model:   req.Model,
				Choices: []ChunkChoice{{
					Index:        0,
					Delta:        Delta{},
					FinishReason: stringPtr(reason),
				}},
			})
			slog.Debug("SSE stream finished", "chunks", chunkCount, "reason", reason)

		case domain.PolicyViolationEvent:
			// Send error message to client as content and then finish with error
			slog.Error("Policy violation in stream", "message", e.Message)
			writeErr = s.writeSSEChunk(w, flusher, ChatCompletionChunk{
				ID:      id,
				Object:  "chat.completion.chunk",
				Created: created,
				Model:   req.Model,
				Choices: []ChunkChoice{{
					Index: 0,
					Delta: Delta{
						Content: stringPtr(fmt.Sprintf("Error: %s", e.Message)),
					},
				}},
			})
			// Also send finish event with error
			if writeErr == nil {
				writeErr = s.writeSSEChunk(w, flusher, ChatCompletionChunk{
					ID:      id,
					Object:  "chat.completion.chunk",
					Created: created,
					Model:   req.Model,
					Choices: []ChunkChoice{{
						Index:        0,
						Delta:        Delta{},
						FinishReason: stringPtr("error"),
					}},
				})
			}

		}

		if writeErr != nil {
			slog.Error("Failed to write SSE chunk", "error", writeErr, "chunk", chunkCount)
			// Don't return - let the channel drain to avoid blocking the provider
			continue
		}
	}

	// Send [DONE] marker
	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
	slog.Debug("SSE stream complete", "total_chunks", chunkCount)
}

// handleNonStreamingResponse handles non-streaming response
func (s *Server) handleNonStreamingResponse(w http.ResponseWriter, r *http.Request, domainReq *domain.ChatRequest, req *ChatCompletionRequest) {
	response, err := s.gateway.ChatComplete(r.Context(), domainReq)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}

	// Convert to OpenAI format
	msg := ChatMessage{
		Role:    "assistant",
		Content: response.Content,
	}

	if len(response.ToolCalls) > 0 {
		for _, tc := range response.ToolCalls {
			argsJSON, _ := json.Marshal(tc.Function.Arguments)
			msg.ToolCalls = append(msg.ToolCalls, ToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: &FunctionCall{
					Name:      tc.Function.Name,
					Arguments: string(argsJSON),
				},
			})
		}
	}

	reason := "stop"
	if response.FinishReason == domain.FinishReasonToolCalls {
		reason = "tool_calls"
	}

	resp := ChatCompletionResponse{
		ID:      fmt.Sprintf("chatcmpl-%s", uuid.New().String()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   req.Model,
		Choices: []Choice{{
			Index:        0,
			Message:      msg,
			FinishReason: reason,
		}},
	}

	if response.Usage != nil {
		resp.Usage = &Usage{
			PromptTokens:     response.Usage.PromptTokens,
			CompletionTokens: response.Usage.CompletionTokens,
			TotalTokens:      response.Usage.TotalTokens,
		}
	}

	s.writeJSON(w, http.StatusOK, resp)
}

// handleEmbeddings handles POST /v1/embeddings
func (s *Server) handleEmbeddings(w http.ResponseWriter, r *http.Request, tenantObj *domain.Tenant) {
	var req EmbeddingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	// Get input texts
	var texts []string
	switch v := req.Input.(type) {
	case string:
		texts = []string{v}
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				texts = append(texts, s)
			}
		}
	}

	tenantID := ""
	if tenantObj != nil {
		tenantID = tenantObj.ID
	}

	embeddings, tokens, err := s.gateway.Embed(r.Context(), req.Model, texts, req.Dimensions, tenantID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}

	// Convert to OpenAI format
	var data []EmbeddingData
	for i, emb := range embeddings {
		data = append(data, EmbeddingData{
			Object:    "embedding",
			Embedding: emb,
			Index:     i,
		})
	}

	s.writeJSON(w, http.StatusOK, EmbeddingsResponse{
		Object: "list",
		Data:   data,
		Model:  req.Model,
		Usage: EmbeddingUsage{
			PromptTokens: int(tokens),
			TotalTokens:  int(tokens),
		},
	})
}

// handleListModelsFiltered handles GET /v1/models with role-based filtering
func (s *Server) handleListModelsFiltered(w http.ResponseWriter, r *http.Request, auth *AuthContext) {
	// Load models from tenant database (single-tenant mode)
	var models []domain.ModelInfo
	if s.pgStore != nil {
		tenantStore := s.pgStore.TenantStore()
		var err error
		models, err = tenantStore.ListAvailableModelsForAPI(r.Context())
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, "server_error", "Failed to list models")
			return
		}
	}

	// If no models from database, fall back to gateway
	if len(models) == 0 {
		var err error
		models, _, err = s.gateway.ListModels(r.Context(), "")
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, "server_error", err.Error())
			return
		}
	}

	// Filter models by role/group if API key has one
	filteredModels := models
	if auth.APIKey != nil && s.pgStore != nil {
		tenantStore := s.pgStore.TenantStore()
		// Collect all applicable model restrictions
		var restrictions []*domain.ModelRestrictions

		// Case 1: API key has a direct role assignment
		if auth.APIKey.RoleID != "" {
			role, err := tenantStore.GetRole(r.Context(), auth.APIKey.RoleID)
			if err == nil && role != nil && role.Policy != nil {
				restrictions = append(restrictions, &role.Policy.ModelRestriction)
			}
		}

		// Case 2: API key has a group assignment (inherits from ALL roles in the group)
		if auth.APIKey.GroupID != "" {
			groupRoles, err := tenantStore.GetGroupRoles(r.Context(), auth.APIKey.GroupID)
			if err == nil {
				for _, role := range groupRoles {
					if role.Policy != nil {
						restrictions = append(restrictions, &role.Policy.ModelRestriction)
					}
				}
			}
		}

		// Apply combined filtering from all roles
		if len(restrictions) > 0 {
			filteredModels = filterModelsByPolicies(models, restrictions)
		}
	}

	var data []ModelData
	for _, m := range filteredModels {
		data = append(data, ModelData{
			ID:      m.ID,
			Object:  "model",
			Created: 1234567890,
			OwnedBy: string(m.Provider),
		})
	}

	s.writeJSON(w, http.StatusOK, ModelsResponse{
		Object: "list",
		Data:   data,
	})
}

// filterModelsByPolicies filters models based on multiple role policies (for group memberships)
func filterModelsByPolicies(models []domain.ModelInfo, restrictions []*domain.ModelRestrictions) []domain.ModelInfo {
	if len(restrictions) == 0 {
		return models
	}

	// If only one restriction, use simple logic
	if len(restrictions) == 1 {
		return filterModelsByPolicy(models, restrictions[0])
	}

	// Multiple restrictions: collect all allowed models from all restrictions
	allowedModels := make(map[string]bool)
	hasAllowedModels := false

	for _, restriction := range restrictions {
		if restriction == nil {
			continue
		}

		if len(restriction.AllowedModels) > 0 {
			hasAllowedModels = true
			for _, modelID := range restriction.AllowedModels {
				allowedModels[modelID] = true
			}
		}
	}

	// If no allowed models are configured, return all models
	if !hasAllowedModels {
		return models
	}

	// Apply filtering - model must be in at least one allowed list
	filtered := []domain.ModelInfo{}
	for _, m := range models {
		if allowedModels[m.ID] {
			filtered = append(filtered, m)
		}
	}

	return filtered
}

// filterModelsByPolicy filters models based on a single role policy
func filterModelsByPolicy(models []domain.ModelInfo, restrictions *domain.ModelRestrictions) []domain.ModelInfo {
	if restrictions == nil {
		return models
	}

	// If no allowed models configured, return all
	if len(restrictions.AllowedModels) == 0 {
		return models
	}

	// Only return models in the allowed list
	allowedMap := make(map[string]bool)
	for _, modelID := range restrictions.AllowedModels {
		allowedMap[modelID] = true
	}

	filtered := []domain.ModelInfo{}
	for _, m := range models {
		if allowedMap[m.ID] {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

// handleGetModelFiltered handles GET /v1/models/{model} with role-based access check
func (s *Server) handleGetModelFiltered(w http.ResponseWriter, r *http.Request, auth *AuthContext) {
	modelID := r.PathValue("model")
	tenantID := ""
	roleID := ""
	if auth.Tenant != nil {
		tenantID = auth.Tenant.ID
	}
	if auth.APIKey != nil {
		roleID = auth.APIKey.RoleID
	}

	// Get all models
	models, _, err := s.gateway.ListModels(r.Context(), tenantID)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "server_error", err.Error())
		return
	}

	// Filter models by role
	filteredModels := models
	if roleID != "" && tenantID != "" {
		filteredModels, err = s.gateway.GetAllowedModelsForRole(r.Context(), tenantID, roleID, models)
		if err != nil {
			filteredModels = models
		}
	}

	for _, m := range filteredModels {
		if m.ID == modelID {
			s.writeJSON(w, http.StatusOK, ModelData{
				ID:      m.ID,
				Object:  "model",
				Created: 1234567890,
				OwnedBy: string(m.Provider),
			})
			return
		}
	}

	// Check if model exists but is blocked for this role
	for _, m := range models {
		if m.ID == modelID {
			s.writeError(w, http.StatusForbidden, "model_not_allowed", fmt.Sprintf("Model %s is not allowed for your API key's role", modelID))
			return
		}
	}

	s.writeError(w, http.StatusNotFound, "model_not_found", fmt.Sprintf("Model %s not found", modelID))
}

// handleHealth handles health check
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleReady handles readiness check
func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// Helper methods

func (s *Server) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *Server) writeError(w http.ResponseWriter, status int, errType, message string) {
	s.writeJSON(w, status, ErrorResponse{
		Error: ErrorDetail{
			Type:    errType,
			Message: message,
		},
	})
}

func (s *Server) writeSSEChunk(w io.Writer, flusher http.Flusher, chunk any) error {
	data, _ := json.Marshal(chunk)
	_, err := fmt.Fprintf(w, "data: %s\n\n", data)
	if err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

func (s *Server) writeSSEError(w io.Writer, flusher http.Flusher, err error) {
	fmt.Fprintf(w, "data: {\"error\": \"%s\"}\n\n", err.Error())
	flusher.Flush()
}

func (s *Server) convertChatRequest(req *ChatCompletionRequest) *domain.ChatRequest {
	domainReq := &domain.ChatRequest{
		Model:       req.Model,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Streaming:   req.Stream,
		RequestID:   uuid.New().String(),
	}

	// Convert messages
	for _, msg := range req.Messages {
		domainMsg := domain.Message{
			Role: msg.Role,
		}

		// Handle content
		switch content := msg.Content.(type) {
		case string:
			if msg.Role == "system" {
				domainReq.SystemPrompt = content
				continue
			}
			domainMsg.Content = []domain.ContentBlock{{
				Type: "text",
				Text: content,
			}}
		case []interface{}:
			for _, c := range content {
				if cm, ok := c.(map[string]interface{}); ok {
					if t, ok := cm["type"].(string); ok {
						switch t {
						case "text":
							domainMsg.Content = append(domainMsg.Content, domain.ContentBlock{
								Type: "text",
								Text: cm["text"].(string),
							})
						case "image_url":
							if imgURL, ok := cm["image_url"].(map[string]interface{}); ok {
								domainMsg.Content = append(domainMsg.Content, domain.ContentBlock{
									Type:     "image",
									ImageURL: imgURL["url"].(string),
								})
							}
						}
					}
				}
			}
		}

		// Handle tool calls
		if msg.ToolCalls != nil {
			for _, tc := range msg.ToolCalls {
				var args map[string]any
				if tc.Function != nil {
					json.Unmarshal([]byte(tc.Function.Arguments), &args)
				}
				domainMsg.ToolCalls = append(domainMsg.ToolCalls, domain.ToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: domain.FunctionCall{
						Name:      tc.Function.Name,
						Arguments: args,
					},
				})
			}
		}

		// Handle tool result
		if msg.ToolCallID != "" {
			domainMsg.ToolCallID = msg.ToolCallID
		}

		domainReq.Messages = append(domainReq.Messages, domainMsg)
	}

	// Convert tools
	for _, tool := range req.Tools {
		domainReq.Tools = append(domainReq.Tools, domain.Tool{
			Type: tool.Type,
			Function: domain.FunctionDefinition{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				Parameters:  tool.Function.Parameters,
			},
		})
	}

	return domainReq
}

// handleResponses handles POST /v1/responses - structured outputs API
func (s *Server) handleResponses(w http.ResponseWriter, r *http.Request, auth *AuthContext) {
	startTime := time.Now()

	var req ResponsesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body")
		return
	}

	// Convert to domain request
	domainReq := s.convertResponsesRequest(&req, auth)

	// Enforce policies (reuse existing policy engine)
	toolResult, err := s.enforcePoliciesForRequest(r.Context(), &domain.ChatRequest{
		Model:    domainReq.Model,
		Messages: domainReq.Messages,
		APIKeyID: domainReq.APIKeyID,
		RoleID:   domainReq.RoleID,
		GroupID:  domainReq.GroupID,
	}, auth)
	if err != nil {
		s.writePolicyViolationError(w, err)
		return
	}

	// Add headers for removed tools (if any)
	if toolResult != nil && len(toolResult.RemovedTools) > 0 {
		w.Header().Set("X-ModelGate-Removed-Tools", strings.Join(toolResult.RemovedTools, ","))
		w.Header().Set("X-ModelGate-Warning", fmt.Sprintf("%d tool(s) removed from request", len(toolResult.RemovedTools)))
	}

	// Call responses service
	resp, err := s.responsesService.GenerateResponse(r.Context(), domainReq)
	if err != nil {
		slog.Error("responses generation failed", "error", err, "model", domainReq.Model)
		s.writeError(w, http.StatusInternalServerError, "generation_error", err.Error())
		return
	}

	// Convert to HTTP response
	httpResp := &ResponsesResponse{
		ID:       resp.ID,
		Object:   resp.Object,
		Created:  resp.Created,
		Model:    resp.Model,
		Response: resp.Response,
		Usage: ResponsesUsageOutput{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}

	// Add metadata headers
	if resp.Metadata != nil {
		w.Header().Set("X-ModelGate-Provider", resp.Metadata.Provider)
		w.Header().Set("X-ModelGate-Implementation-Mode", resp.Metadata.ImplementationMode)
		w.Header().Set("X-ModelGate-Schema-Validated", fmt.Sprintf("%t", resp.Metadata.SchemaValidated))
		if resp.Metadata.RetryCount > 0 {
			w.Header().Set("X-ModelGate-Retry-Count", fmt.Sprintf("%d", resp.Metadata.RetryCount))
		}
	}

	// Record metrics (reuse existing telemetry)
	duration := time.Since(startTime)
	s.metrics.RecordRequest("responses", domainReq.Model, duration)
	// Note: Token metrics can be added when we extend the telemetry package

	// Write response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(httpResp)
}

// convertResponsesRequest converts HTTP to domain request
func (s *Server) convertResponsesRequest(req *ResponsesRequest, auth *AuthContext) *domain.ResponseRequest {
	domainReq := &domain.ResponseRequest{
		Model:       req.Model,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		TopP:        req.TopP,
		RequestID:   uuid.New().String(),
		ResponseSchema: domain.ResponseSchema{
			Name:        req.ResponseSchema.Name,
			Description: req.ResponseSchema.Description,
			Schema:      req.ResponseSchema.Schema,
			Strict:      req.ResponseSchema.Strict,
		},
	}

	// Convert messages (reuse existing ChatMessage conversion logic)
	for _, msg := range req.Messages {
		domainMsg := domain.Message{
			Role: msg.Role,
		}

		// Handle content
		switch content := msg.Content.(type) {
		case string:
			if msg.Role == "system" {
				// System messages are handled separately in the service
			}
			domainMsg.Content = []domain.ContentBlock{{
				Type: "text",
				Text: content,
			}}
		case []interface{}:
			for _, c := range content {
				if cm, ok := c.(map[string]interface{}); ok {
					if t, ok := cm["type"].(string); ok {
						switch t {
						case "text":
							if text, ok := cm["text"].(string); ok {
								domainMsg.Content = append(domainMsg.Content, domain.ContentBlock{
									Type: "text",
									Text: text,
								})
							}
						case "image_url":
							if imgURL, ok := cm["image_url"].(map[string]interface{}); ok {
								if url, ok := imgURL["url"].(string); ok {
									domainMsg.Content = append(domainMsg.Content, domain.ContentBlock{
										Type:     "image",
										ImageURL: url,
									})
								}
							}
						}
					}
				}
			}
		}

		domainReq.Messages = append(domainReq.Messages, domainMsg)
	}

	// Add API key context
	if auth.APIKey != nil {
		domainReq.APIKeyID = auth.APIKey.ID
		domainReq.RoleID = auth.APIKey.RoleID
		domainReq.GroupID = auth.APIKey.GroupID
	}

	return domainReq
}

func stringPtr(s string) *string {
	return &s
}

// Start starts the HTTP server
func (s *Server) Start(ctx context.Context, addr string) error {
	server := &http.Server{
		Addr:         addr,
		Handler:      s.Handler(),
		ReadTimeout:  s.config.Server.ReadTimeout,
		WriteTimeout: s.config.Server.WriteTimeout,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	return server.ListenAndServe()
}

// hashAPIKey creates a SHA-256 hash of the API key
func hashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}
