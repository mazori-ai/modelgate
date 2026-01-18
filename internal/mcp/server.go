package mcp

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"modelgate/internal/domain"
	"modelgate/internal/storage/postgres"

	"github.com/google/uuid"
)

// MCPServer exposes ModelGate as an MCP server to agents
// This allows agents to discover and use tools from all connected MCP servers
// via a single unified interface with the tool_search capability.
type MCPServer struct {
	gateway *Gateway
	stores  map[string]*postgres.TenantStore
	store   *postgres.Store
	mu      sync.RWMutex

	// Server configuration
	serverInfo   ServerInfo
	capabilities ServerCapabilities
}

// ServerInfo contains MCP server metadata
type ServerInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
}

// ServerCapabilities defines what the server supports
type ServerCapabilities struct {
	Tools     *ToolCapabilities     `json:"tools,omitempty"`
	Resources *ResourceCapabilities `json:"resources,omitempty"`
	Prompts   *PromptCapabilities   `json:"prompts,omitempty"`
}

type ToolCapabilities struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type ResourceCapabilities struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

type PromptCapabilities struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// JSON-RPC types
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Error implements the error interface
func (e *RPCError) Error() string {
	return fmt.Sprintf("RPC error %d: %s", e.Code, e.Message)
}

// MCP Protocol types
type InitializeParams struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ClientInfo      ClientInfo             `json:"clientInfo"`
}

type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    ServerCapabilities `json:"capabilities"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
}

type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

type ListToolsResult struct {
	Tools []ToolDefinition `json:"tools"`
}

type CallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type CallToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// AuthenticatedClient represents an authenticated MCP client connection
type AuthenticatedClient struct {
	TenantSlug  string
	RoleID      string
	APIKeyID    string
	ClientInfo  ClientInfo
	ConnectedAt time.Time
}

// NewMCPServer creates a new MCP server instance
func NewMCPServer(gateway *Gateway, store *postgres.Store) *MCPServer {
	return &MCPServer{
		gateway: gateway,
		store:   store,
		stores:  make(map[string]*postgres.TenantStore),
		serverInfo: ServerInfo{
			Name:        "ModelGate MCP Gateway",
			Version:     "1.0.0",
			Description: "Unified MCP gateway with Tool Search capability",
		},
		capabilities: ServerCapabilities{
			Tools: &ToolCapabilities{
				ListChanged: true,
			},
		},
	}
}

// RegisterTenantStore registers a tenant store
func (s *MCPServer) RegisterTenantStore(tenantSlug string, store *postgres.TenantStore) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stores[tenantSlug] = store
}

// ============================================
// HTTP/SSE TRANSPORT
// ============================================

// ServeHTTP handles MCP requests over HTTP/SSE
// Authentication is via Bearer token (tenant API key)
func (s *MCPServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Authenticate the request
	client, err := s.authenticateRequest(r)
	if err != nil {
		http.Error(w, `{"error": "unauthorized", "message": "`+err.Error()+`"}`, http.StatusUnauthorized)
		return
	}

	switch r.Method {
	case http.MethodGet:
		// SSE endpoint for server-to-client messages
		s.handleSSE(w, r, client)
	case http.MethodPost:
		// JSON-RPC endpoint for client-to-server messages
		s.handleJSONRPC(w, r, client)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// authenticateRequest validates the API key and returns client context
func (s *MCPServer) authenticateRequest(r *http.Request) (*AuthenticatedClient, error) {
	// Extract Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		authHeader = r.Header.Get("X-API-Key")
		if authHeader == "" {
			return nil, fmt.Errorf("missing authorization header")
		}
	}

	// Parse Bearer token
	apiKey := strings.TrimPrefix(authHeader, "Bearer ")

	// Validate API key using the store's tenant repository
	tenantRepo := s.store.TenantRepository()
	_, apiKeyObj, err := tenantRepo.GetByAPIKey(r.Context(), hashAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("invalid API key: %w", err)
	}

	// Single tenant mode - always use "default"
	tenantSlug := "default"

	// Ensure tenant store is registered for MCP operations
	s.mu.RLock()
	_, exists := s.stores[tenantSlug]
	s.mu.RUnlock()

	if !exists {
		if store, err := s.store.GetTenantStore(tenantSlug); err == nil {
			s.RegisterTenantStore(tenantSlug, store)
		}
	}

	return &AuthenticatedClient{
		TenantSlug:  tenantSlug,
		RoleID:      apiKeyObj.RoleID,
		APIKeyID:    apiKeyObj.ID,
		ConnectedAt: time.Now(),
	}, nil
}

// hashAPIKey creates a SHA-256 hash of the API key
func hashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

// handleSSE handles Server-Sent Events for server-to-client messages
func (s *MCPServer) handleSSE(w http.ResponseWriter, r *http.Request, client *AuthenticatedClient) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "SSE not supported", http.StatusInternalServerError)
		return
	}

	// Send initial connection message
	event := map[string]interface{}{
		"type":    "connection",
		"status":  "connected",
		"tenant":  client.TenantSlug,
		"message": "Connected to ModelGate MCP Gateway",
	}
	data, _ := json.Marshal(event)
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()

	// Keep connection alive
	ctx := r.Context()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

// handleJSONRPC handles JSON-RPC requests
func (s *MCPServer) handleJSONRPC(w http.ResponseWriter, r *http.Request, client *AuthenticatedClient) {
	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeJSONRPCError(w, nil, -32700, "Parse error", nil)
		return
	}

	slog.Debug("MCP request received",
		"method", req.Method,
		"tenant", client.TenantSlug,
		"role_id", client.RoleID,
	)

	ctx := r.Context()
	result, err := s.handleMethod(ctx, client, req.Method, req.Params)
	if err != nil {
		if rpcErr, ok := err.(*RPCError); ok {
			s.writeJSONRPCError(w, req.ID, rpcErr.Code, rpcErr.Message, rpcErr.Data)
		} else {
			s.writeJSONRPCError(w, req.ID, -32603, err.Error(), nil)
		}
		return
	}

	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (s *MCPServer) writeJSONRPCError(w http.ResponseWriter, id interface{}, code int, message string, data interface{}) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleMethod routes MCP method calls
func (s *MCPServer) handleMethod(ctx context.Context, client *AuthenticatedClient, method string, params json.RawMessage) (interface{}, error) {
	switch method {
	case "initialize":
		var p InitializeParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &RPCError{Code: -32602, Message: "Invalid params"}
		}
		client.ClientInfo = p.ClientInfo
		return s.handleInitialize(ctx, client, p)

	case "tools/list":
		return s.handleListTools(ctx, client)

	case "tools/call":
		var p CallToolParams
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &RPCError{Code: -32602, Message: "Invalid params"}
		}
		return s.handleCallTool(ctx, client, p)

	case "ping":
		return map[string]interface{}{}, nil

	default:
		return nil, &RPCError{Code: -32601, Message: "Method not found: " + method}
	}
}

// handleInitialize handles the MCP initialize request
func (s *MCPServer) handleInitialize(ctx context.Context, client *AuthenticatedClient, params InitializeParams) (*InitializeResult, error) {
	slog.Info("MCP client initialized",
		"client", params.ClientInfo.Name,
		"version", params.ClientInfo.Version,
		"tenant", client.TenantSlug,
	)

	return &InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities:    s.capabilities,
		ServerInfo:      s.serverInfo,
	}, nil
}

// handleListTools returns all available tools for the client
// Only tools with ALLOW visibility are returned in tools/list
// Tools with SEARCH visibility are hidden but can be discovered via tool_search
// Tools with DENY visibility are completely hidden
func (s *MCPServer) handleListTools(ctx context.Context, client *AuthenticatedClient) (*ListToolsResult, error) {
	tools := []ToolDefinition{}

	// 1. Add the special tool_search tool (always visible)
	tools = append(tools, ToolDefinition{
		Name:        "tool_search",
		Description: "Search for available tools across all connected MCP servers. Use this to discover capabilities before using specific tools.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Natural language description of the capability you're looking for",
				},
				"category": map[string]interface{}{
					"type":        "string",
					"description": "Optional category filter",
					"enum":        []string{"messaging", "file-system", "database", "api", "git", "calendar", "shell", "search", "other"},
				},
				"max_results": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of tools to return (default: 5)",
					"default":     5,
				},
			},
			"required": []string{"query"},
		},
	})

	// 2. Get MCP tools with ALLOW visibility only
	s.mu.RLock()
	store, exists := s.stores[client.TenantSlug]
	s.mu.RUnlock()

	if exists {
		// List tools from all connected MCP servers
		mcpTools, err := store.ListAllMCPTools(ctx)
		if err == nil {
			for _, tool := range mcpTools {
				if tool.IsDeprecated {
					continue
				}

				// Check visibility - only ALLOW tools appear in tools/list
				visibility := store.GetMCPToolVisibility(ctx, client.RoleID, tool.ID)
				if visibility != domain.MCPVisibilityAllow {
					continue // SEARCH and DENY tools are not listed
				}

				// Convert to MCP tool definition with sanitized name
				tools = append(tools, ToolDefinition{
					Name:        SanitizeToolName(tool.ServerName, tool.Name),
					Description: tool.Description,
					InputSchema: tool.InputSchema,
				})
			}
		}
	}

	return &ListToolsResult{Tools: tools}, nil
}

// handleCallTool executes a tool
func (s *MCPServer) handleCallTool(ctx context.Context, client *AuthenticatedClient, params CallToolParams) (*CallToolResult, error) {
	startTime := time.Now()

	slog.Info("MCP tool call",
		"tool", params.Name,
		"tenant", client.TenantSlug,
		"role_id", client.RoleID,
	)

	// Handle special tool_search tool
	if params.Name == "tool_search" {
		return s.handleToolSearch(ctx, client, params.Arguments)
	}

	// Parse tool name: server_slug__tool_name
	serverSlug, toolName, ok := ParseToolName(params.Name)
	if !ok {
		return &CallToolResult{
			Content: []ContentBlock{{Type: "text", Text: "Invalid tool name format. Expected: server_slug__tool_name"}},
			IsError: true,
		}, nil
	}

	// Get store
	s.mu.RLock()
	store, exists := s.stores[client.TenantSlug]
	s.mu.RUnlock()

	if !exists {
		return &CallToolResult{
			Content: []ContentBlock{{Type: "text", Text: "Tenant store not found"}},
			IsError: true,
		}, nil
	}

	// Find the server
	servers, err := store.ListMCPServers(ctx)
	if err != nil {
		return &CallToolResult{
			Content: []ContentBlock{{Type: "text", Text: "Failed to list servers: " + err.Error()}},
			IsError: true,
		}, nil
	}

	var targetServer *domain.MCPServer
	for _, srv := range servers {
		if srv.Slug == serverSlug {
			targetServer = srv
			break
		}
	}

	if targetServer == nil {
		return &CallToolResult{
			Content: []ContentBlock{{Type: "text", Text: "Server not found: " + serverSlug}},
			IsError: true,
		}, nil
	}

	// Find the tool
	tool, err := store.GetMCPToolByName(ctx, targetServer.ID, toolName)
	if err != nil || tool == nil {
		return &CallToolResult{
			Content: []ContentBlock{{Type: "text", Text: "Tool not found: " + toolName}},
			IsError: true,
		}, nil
	}

	// Check visibility - DENY tools cannot be called
	visibility := store.GetMCPToolVisibility(ctx, client.RoleID, tool.ID)
	if visibility == domain.MCPVisibilityDeny {
		return &CallToolResult{
			Content: []ContentBlock{{Type: "text", Text: "Tool access denied by policy"}},
			IsError: true,
		}, nil
	}
	// ALLOW and SEARCH visibility tools can be called

	// Execute via gateway
	result, err := s.gateway.ExecuteTool(ctx, targetServer, toolName, params.Arguments)

	// Log execution
	execStatus := domain.MCPExecSuccess
	errMsg := ""
	if err != nil {
		execStatus = domain.MCPExecError
		errMsg = err.Error()
	}

	store.LogMCPToolExecution(ctx, &domain.MCPToolExecution{
		ID:           uuid.New().String(),
		ServerID:     targetServer.ID,
		ToolID:       tool.ID,
		RoleID:       client.RoleID,
		APIKeyID:     client.APIKeyID,
		InputParams:  params.Arguments,
		OutputResult: result,
		Status:       execStatus,
		ErrorMessage: errMsg,
		StartedAt:    startTime,
		DurationMs:   int(time.Since(startTime).Milliseconds()),
	})

	if err != nil {
		return &CallToolResult{
			Content: []ContentBlock{{Type: "text", Text: "Tool execution failed: " + err.Error()}},
			IsError: true,
		}, nil
	}

	// Convert result to content blocks
	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	return &CallToolResult{
		Content: []ContentBlock{{Type: "text", Text: string(resultJSON)}},
	}, nil
}

// handleToolSearch handles the special tool_search tool
// Returns full tool specifications in MCP format that can be directly added to context
// Based on Anthropic's Tool Search Tool pattern: https://platform.claude.com/docs/en/agents-and-tools/tool-use/tool-search-tool
func (s *MCPServer) handleToolSearch(ctx context.Context, client *AuthenticatedClient, args map[string]interface{}) (*CallToolResult, error) {
	query, _ := args["query"].(string)
	if query == "" {
		return &CallToolResult{
			Content: []ContentBlock{{Type: "text", Text: "Missing required argument: query"}},
			IsError: true,
		}, nil
	}

	category, _ := args["category"].(string)
	maxResults := 5
	if mr, ok := args["max_results"].(float64); ok {
		maxResults = int(mr)
	}

	// Build search request
	req := &domain.ToolSearchRequest{
		Query:         query,
		Strategy:      domain.SearchStrategyHybrid,
		MaxResults:    maxResults,
		IncludeSchema: true, // Always include schema for tool specs
	}
	if category != "" {
		req.Categories = []string{category}
	}

	// Search - includes visibility filtering for ALLOW and SEARCH tools
	response, err := s.gateway.SearchTools(ctx, client.TenantSlug, client.RoleID, req)
	if err != nil {
		return &CallToolResult{
			Content: []ContentBlock{{Type: "text", Text: "Search failed: " + err.Error()}},
			IsError: true,
		}, nil
	}

	allowedTools := response.Tools

	if len(allowedTools) == 0 {
		return &CallToolResult{
			Content: []ContentBlock{{Type: "text", Text: `{"query":"` + query + `","total_found":0,"tools":[],"message":"No tools found. Try a different query or check your permissions."}`}},
		}, nil
	}

	// Build tool specifications in MCP format (same as tools/list response)
	// This allows direct addition to the tools context
	mcpTools := make([]map[string]interface{}, 0, len(allowedTools))
	for _, result := range allowedTools {
		// Build the sanitized tool name with server prefix for routing
		fullName := SanitizeToolName(result.ServerName, result.Tool.Name)

		// MCP tool format - matches tools/list response exactly
		mcpTool := map[string]interface{}{
			"name":        fullName,
			"description": result.Tool.Description,
		}

		// Add input schema
		if len(result.Tool.InputSchema) > 0 {
			mcpTool["inputSchema"] = result.Tool.InputSchema
		} else {
			mcpTool["inputSchema"] = map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}
		}

		// Add examples if available (per Anthropic's Tool Use Examples pattern)
		if len(result.Tool.InputExamples) > 0 {
			mcpTool["inputExamples"] = result.Tool.InputExamples
		}

		// Add metadata for context
		mcpTool["_metadata"] = map[string]interface{}{
			"server":   result.ServerName,
			"category": result.Tool.Category,
			"score":    result.Score,
		}

		mcpTools = append(mcpTools, mcpTool)
	}

	// Return response in a format that matches MCP tools/list
	// The "tools" array can be directly merged with existing tools
	searchResult := map[string]interface{}{
		"query":       query,
		"total_found": len(allowedTools),
		"tools":       mcpTools,
	}

	resultJSON, err := json.Marshal(searchResult)
	if err != nil {
		return &CallToolResult{
			Content: []ContentBlock{{Type: "text", Text: "Failed to format results: " + err.Error()}},
			IsError: true,
		}, nil
	}

	return &CallToolResult{
		Content: []ContentBlock{{Type: "text", Text: string(resultJSON)}},
	}, nil
}

// ============================================
// STDIO TRANSPORT (for local agents)
// ============================================

// ServeStdio handles MCP over stdio (for local agent integration)
func (s *MCPServer) ServeStdio(ctx context.Context, stdin io.Reader, stdout io.Writer, client *AuthenticatedClient) error {
	scanner := bufio.NewScanner(stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			s.writeStdioError(stdout, nil, -32700, "Parse error")
			continue
		}

		result, err := s.handleMethod(ctx, client, req.Method, req.Params)
		if err != nil {
			if rpcErr, ok := err.(*RPCError); ok {
				s.writeStdioError(stdout, req.ID, rpcErr.Code, rpcErr.Message)
			} else {
				s.writeStdioError(stdout, req.ID, -32603, err.Error())
			}
			continue
		}

		resp := JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  result,
		}
		respBytes, _ := json.Marshal(resp)
		stdout.Write(respBytes)
		stdout.Write([]byte("\n"))
	}

	return scanner.Err()
}

func (s *MCPServer) writeStdioError(stdout io.Writer, id interface{}, code int, message string) {
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &RPCError{Code: code, Message: message},
	}
	respBytes, _ := json.Marshal(resp)
	stdout.Write(respBytes)
	stdout.Write([]byte("\n"))
}

// ============================================
// API KEY VALIDATION HELPER
// ============================================
