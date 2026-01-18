package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"modelgate/internal/domain"
	"modelgate/internal/storage/postgres"

	"github.com/google/uuid"
)

// Gateway manages MCP server connections and tool discovery per tenant
type Gateway struct {
	mu          sync.RWMutex
	connections map[string]*Connection // serverID -> connection
	stores      map[string]*postgres.TenantStore
	embedder    Embedder
	searchCache *SearchCache
}

// Connection represents an active MCP server connection
type Connection struct {
	Server     *domain.MCPServer
	Process    *exec.Cmd
	Stdin      io.WriteCloser
	Stdout     io.ReadCloser
	httpClient *http.Client
	Status     domain.MCPServerStatus
	LastError  error
	RetryCount int
	LastRetry  time.Time
	mu         sync.Mutex
}

// NewGateway creates a new MCP Gateway
func NewGateway(embedder Embedder) *Gateway {
	return &Gateway{
		connections: make(map[string]*Connection),
		stores:      make(map[string]*postgres.TenantStore),
		embedder:    embedder,
		searchCache: NewSearchCache(5 * time.Minute),
	}
}

// RegisterTenantStore registers a tenant's store
func (g *Gateway) RegisterTenantStore(tenantSlug string, store *postgres.TenantStore) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.stores[tenantSlug] = store
}

// Connect establishes a connection to an MCP server
func (g *Gateway) Connect(ctx context.Context, server *domain.MCPServer) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Check if already connected
	if conn, exists := g.connections[server.ID]; exists {
		if conn.Status == domain.MCPStatusConnected {
			return nil
		}
	}

	var conn *Connection
	var err error

	switch server.ServerType {
	case domain.MCPServerTypeStdio:
		conn, err = g.connectStdio(ctx, server)
	case domain.MCPServerTypeSSE:
		conn, err = g.connectSSE(ctx, server)
	case domain.MCPServerTypeWebSocket:
		conn, err = g.connectWebSocket(ctx, server)
	default:
		return fmt.Errorf("unsupported server type: %s", server.ServerType)
	}

	if err != nil {
		slog.Error("Failed to connect to MCP server", "server", server.Name, "error", err)
		return err
	}

	conn.Server = server
	conn.Status = domain.MCPStatusConnected
	g.connections[server.ID] = conn

	slog.Info("Connected to MCP server", "server", server.Name, "type", server.ServerType)
	return nil
}

// connectStdio starts a stdio-based MCP server process
func (g *Gateway) connectStdio(ctx context.Context, server *domain.MCPServer) (*Connection, error) {
	args := server.Arguments
	if len(args) == 0 {
		args = []string{}
	}

	cmd := exec.CommandContext(ctx, server.Endpoint, args...)

	// Set environment
	cmd.Env = os.Environ()
	for k, v := range server.Environment {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start MCP server: %w", err)
	}

	return &Connection{
		Process: cmd,
		Stdin:   stdin,
		Stdout:  stdout,
	}, nil
}

// connectSSE connects to an SSE-based MCP server
func (g *Gateway) connectSSE(ctx context.Context, server *domain.MCPServer) (*Connection, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Test connection
	req, err := http.NewRequestWithContext(ctx, "GET", server.Endpoint, nil)
	if err != nil {
		return nil, err
	}

	// Add SSE headers
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	// Add authentication headers
	g.addAuthHeaders(req, server)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SSE endpoint: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SSE endpoint returned status %d", resp.StatusCode)
	}

	return &Connection{
		httpClient: client,
	}, nil
}

// connectWebSocket connects to a WebSocket-based MCP server
func (g *Gateway) connectWebSocket(ctx context.Context, server *domain.MCPServer) (*Connection, error) {
	// WebSocket implementation would go here
	// For now, return an error as it requires additional dependencies
	return nil, fmt.Errorf("WebSocket transport not yet implemented")
}

// addAuthHeaders adds authentication headers to a request
func (g *Gateway) addAuthHeaders(req *http.Request, server *domain.MCPServer) {
	switch server.AuthType {
	case domain.MCPAuthAPIKey:
		header := server.AuthConfig.APIKeyHeader
		if header == "" {
			header = "Authorization"
		}
		if header == "Authorization" {
			req.Header.Set(header, "Bearer "+server.AuthConfig.APIKey)
		} else {
			req.Header.Set(header, server.AuthConfig.APIKey)
		}
	case domain.MCPAuthBearer:
		// Bearer Token authentication - always uses Authorization header
		req.Header.Set("Authorization", "Bearer "+server.AuthConfig.BearerToken)
	case domain.MCPAuthBasic:
		req.SetBasicAuth(server.AuthConfig.Username, server.AuthConfig.Password)
	case domain.MCPAuthOAuth2:
		// OAuth2 would need token refresh logic
		// For now, assume we have a valid token
		req.Header.Set("Authorization", "Bearer "+server.AuthConfig.ClientSecret)
	}
}

// Disconnect closes a connection to an MCP server
func (g *Gateway) Disconnect(serverID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	conn, exists := g.connections[serverID]
	if !exists {
		return nil
	}

	if conn.Process != nil {
		conn.Process.Process.Kill()
	}
	if conn.Stdin != nil {
		conn.Stdin.Close()
	}
	if conn.Stdout != nil {
		conn.Stdout.Close()
	}

	conn.Status = domain.MCPStatusDisconnected
	delete(g.connections, serverID)

	return nil
}

// ListTools lists all tools from an MCP server
func (g *Gateway) ListTools(ctx context.Context, server *domain.MCPServer) ([]*domain.MCPTool, error) {
	g.mu.RLock()
	conn, exists := g.connections[server.ID]
	g.mu.RUnlock()

	if !exists || conn.Status != domain.MCPStatusConnected {
		// Try to connect first
		if err := g.Connect(ctx, server); err != nil {
			return nil, err
		}
		g.mu.RLock()
		conn = g.connections[server.ID]
		g.mu.RUnlock()
	}

	switch server.ServerType {
	case domain.MCPServerTypeStdio:
		return g.listToolsStdio(ctx, conn, server)
	case domain.MCPServerTypeSSE:
		return g.listToolsSSE(ctx, conn, server)
	default:
		return nil, fmt.Errorf("unsupported server type: %s", server.ServerType)
	}
}

// GetServerInfo gets server information including version from MCP server
func (g *Gateway) GetServerInfo(ctx context.Context, server *domain.MCPServer) (string, error) {
	g.mu.RLock()
	conn, exists := g.connections[server.ID]
	g.mu.RUnlock()

	if !exists || conn.Status != domain.MCPStatusConnected {
		// Try to connect first
		if err := g.Connect(ctx, server); err != nil {
			return "", err
		}
		g.mu.RLock()
		conn = g.connections[server.ID]
		g.mu.RUnlock()
	}

	switch server.ServerType {
	case domain.MCPServerTypeSSE:
		return g.getServerInfoSSE(ctx, conn, server)
	case domain.MCPServerTypeStdio:
		return g.getServerInfoStdio(ctx, conn, server)
	default:
		return "", fmt.Errorf("unsupported server type: %s", server.ServerType)
	}
}

// getServerInfoSSE gets server info from SSE MCP server
func (g *Gateway) getServerInfoSSE(ctx context.Context, conn *Connection, server *domain.MCPServer) (string, error) {
	request := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo": map[string]any{
				"name":    "ModelGate",
				"version": "1.0.0",
			},
		},
	}

	reqBytes, _ := json.Marshal(request)
	req, err := http.NewRequestWithContext(ctx, "POST", server.Endpoint, bytes.NewReader(reqBytes))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	g.addAuthHeaders(req, server)

	resp, err := conn.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)

	var rpcResponse struct {
		Result struct {
			ServerInfo struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"serverInfo"`
		} `json:"result"`
	}

	// Parse SSE format
	if strings.HasPrefix(bodyStr, "event:") || strings.HasPrefix(bodyStr, "data:") {
		lines := strings.Split(bodyStr, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "data: ") {
				jsonData := strings.TrimPrefix(line, "data: ")
				if err := json.Unmarshal([]byte(jsonData), &rpcResponse); err == nil {
					break
				}
			}
		}
	} else {
		json.Unmarshal(bodyBytes, &rpcResponse)
	}

	if rpcResponse.Result.ServerInfo.Version != "" {
		return rpcResponse.Result.ServerInfo.Version, nil
	}

	return "unknown", nil
}

// getServerInfoStdio gets server info from stdio MCP server
func (g *Gateway) getServerInfoStdio(ctx context.Context, conn *Connection, server *domain.MCPServer) (string, error) {
	// Similar implementation for stdio
	return "unknown", nil
}

// listToolsStdio lists tools from a stdio MCP server
func (g *Gateway) listToolsStdio(ctx context.Context, conn *Connection, server *domain.MCPServer) ([]*domain.MCPTool, error) {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	// Send tools/list request
	request := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	}

	reqBytes, _ := json.Marshal(request)
	reqBytes = append(reqBytes, '\n')

	if _, err := conn.Stdin.Write(reqBytes); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Read response (simplified - real implementation needs proper JSON-RPC parsing)
	buf := make([]byte, 64*1024)
	n, err := conn.Stdout.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var response struct {
		Result struct {
			Tools []struct {
				Name        string         `json:"name"`
				Description string         `json:"description"`
				InputSchema map[string]any `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}

	if err := json.Unmarshal(buf[:n], &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	tools := make([]*domain.MCPTool, len(response.Result.Tools))
	for i, t := range response.Result.Tools {
		tools[i] = &domain.MCPTool{
			ID:           uuid.New().String(),
			ServerID:     server.ID,
			ServerName:   server.Name,
			Name:         t.Name,
			Description:  t.Description,
			InputSchema:  t.InputSchema,
			Category:     inferToolCategory(t.Name, t.Description),
			DeferLoading: true,
		}
	}

	return tools, nil
}

// listToolsSSE lists tools from an SSE MCP server using JSON-RPC
func (g *Gateway) listToolsSSE(ctx context.Context, conn *Connection, server *domain.MCPServer) ([]*domain.MCPTool, error) {
	// Prepare JSON-RPC request
	request := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	}

	reqBytes, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	// Send JSON-RPC request via POST to SSE endpoint
	req, err := http.NewRequestWithContext(ctx, "POST", server.Endpoint, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	g.addAuthHeaders(req, server)

	resp, err := conn.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send JSON-RPC request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("JSON-RPC request returned status %d: %s", resp.StatusCode, string(body))
	}

	// Read the response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse as SSE or JSON
	var rpcResponse struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  struct {
			Tools []struct {
				Name        string         `json:"name"`
				Description string         `json:"description"`
				InputSchema map[string]any `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	// Try parsing as SSE first (check if it starts with "event:" or "data:")
	bodyStr := string(bodyBytes)
	if strings.HasPrefix(bodyStr, "event:") || strings.HasPrefix(bodyStr, "data:") {
		// Parse SSE format
		lines := strings.Split(bodyStr, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "data: ") {
				jsonData := strings.TrimPrefix(line, "data: ")
				if err := json.Unmarshal([]byte(jsonData), &rpcResponse); err == nil {
					break
				}
			}
		}
	} else {
		// Parse as plain JSON
		if err := json.Unmarshal(bodyBytes, &rpcResponse); err != nil {
			return nil, fmt.Errorf("failed to decode JSON-RPC response: %w (body: %s)", err, bodyStr)
		}
	}

	if rpcResponse.Error != nil {
		return nil, fmt.Errorf("JSON-RPC error %d: %s", rpcResponse.Error.Code, rpcResponse.Error.Message)
	}

	tools := make([]*domain.MCPTool, len(rpcResponse.Result.Tools))
	for i, t := range rpcResponse.Result.Tools {
		tools[i] = &domain.MCPTool{
			ID:           uuid.New().String(),
			ServerID:     server.ID,
			ServerName:   server.Name,
			Name:         t.Name,
			Description:  t.Description,
			InputSchema:  t.InputSchema,
			Category:     inferToolCategory(t.Name, t.Description),
			DeferLoading: true,
		}
	}

	return tools, nil
}

// ExecuteTool executes a tool on an MCP server
func (g *Gateway) ExecuteTool(ctx context.Context, server *domain.MCPServer, toolName string, args map[string]any) (map[string]any, error) {
	g.mu.RLock()
	conn, exists := g.connections[server.ID]
	g.mu.RUnlock()

	// Auto-reconnect if not connected
	if !exists || conn.Status != domain.MCPStatusConnected {
		if err := g.Connect(ctx, server); err != nil {
			return nil, fmt.Errorf("failed to connect to server %s: %w", server.Name, err)
		}
		g.mu.RLock()
		conn = g.connections[server.ID]
		g.mu.RUnlock()
	}

	switch server.ServerType {
	case domain.MCPServerTypeStdio:
		return g.executeToolStdio(ctx, conn, toolName, args)
	case domain.MCPServerTypeSSE:
		return g.executeToolSSE(ctx, conn, server, toolName, args)
	default:
		return nil, fmt.Errorf("unsupported server type: %s", server.ServerType)
	}
}

func (g *Gateway) executeToolStdio(ctx context.Context, conn *Connection, toolName string, args map[string]any) (map[string]any, error) {
	conn.mu.Lock()
	defer conn.mu.Unlock()

	request := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      toolName,
			"arguments": args,
		},
	}

	reqBytes, _ := json.Marshal(request)
	reqBytes = append(reqBytes, '\n')

	if _, err := conn.Stdin.Write(reqBytes); err != nil {
		return nil, err
	}

	buf := make([]byte, 1024*1024) // 1MB buffer for tool results
	n, err := conn.Stdout.Read(buf)
	if err != nil {
		return nil, err
	}

	var response struct {
		Result map[string]any `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(buf[:n], &response); err != nil {
		return nil, err
	}

	if response.Error != nil {
		return nil, fmt.Errorf("tool error: %s", response.Error.Message)
	}

	return response.Result, nil
}

func (g *Gateway) executeToolSSE(ctx context.Context, conn *Connection, server *domain.MCPServer, toolName string, args map[string]any) (map[string]any, error) {
	// MCP protocol requires JSON-RPC 2.0 format
	toolURL := strings.TrimSuffix(server.Endpoint, "/")

	// Construct JSON-RPC 2.0 request for tools/call method
	rpcRequest := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      toolName,
			"arguments": args,
		},
	}

	body, _ := json.Marshal(rpcRequest)

	req, err := http.NewRequestWithContext(ctx, "POST", toolURL, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}

	g.addAuthHeaders(req, server)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := conn.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read response body for debugging
		bodyBytes, _ := io.ReadAll(resp.Body)
		bodyStr := string(bodyBytes)

		slog.Error("MCP tool call failed",
			"status", resp.StatusCode,
			"server", server.Name,
			"endpoint", toolURL,
			"tool", toolName,
			"response_body", bodyStr,
			"content_type", resp.Header.Get("Content-Type"),
			"accept", req.Header.Get("Accept"),
		)

		return nil, fmt.Errorf("tool call returned status %d: %s", resp.StatusCode, bodyStr)
	}

	// Check Content-Type to determine how to parse response
	contentType := resp.Header.Get("Content-Type")

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse response based on content type
	var rpcResponse map[string]any
	if strings.Contains(contentType, "text/event-stream") {
		// Parse SSE format: extract JSON from "data:" lines
		lines := strings.Split(string(bodyBytes), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "data:") {
				jsonData := strings.TrimPrefix(line, "data:")
				jsonData = strings.TrimSpace(jsonData)
				if err := json.Unmarshal([]byte(jsonData), &rpcResponse); err != nil {
					return nil, fmt.Errorf("failed to parse SSE JSON data: %w", err)
				}
				break
			}
		}
		if rpcResponse == nil {
			return nil, fmt.Errorf("no data field found in SSE response")
		}
	} else {
		// Parse as plain JSON
		if err := json.Unmarshal(bodyBytes, &rpcResponse); err != nil {
			return nil, fmt.Errorf("failed to parse JSON response: %w", err)
		}
	}

	// Check for JSON-RPC error
	if errObj, ok := rpcResponse["error"]; ok {
		return nil, fmt.Errorf("MCP error: %v", errObj)
	}

	// Extract result from JSON-RPC response
	result, ok := rpcResponse["result"].(map[string]any)
	if !ok {
		return rpcResponse, nil // Return full response if result is not a map
	}

	return result, nil
}

// SyncServer syncs tools from an MCP server
func (g *Gateway) SyncServer(ctx context.Context, tenantSlug string, server *domain.MCPServer) (*domain.MCPServerVersion, error) {
	g.mu.RLock()
	store, exists := g.stores[tenantSlug]
	g.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("tenant store not found: %s", tenantSlug)
	}

	// Get server version from MCP server
	serverVersion, err := g.GetServerInfo(ctx, server)
	if err != nil {
		slog.Warn("Failed to get server version", "error", err)
		serverVersion = "unknown"
	}

	// Get current tools from server
	tools, err := g.ListTools(ctx, server)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	// Get previous version for comparison
	prevVersion, err := store.GetLatestMCPServerVersion(ctx, server.ID)
	if err != nil {
		slog.Warn("Failed to get previous version", "error", err)
	}

	// Compute changes
	var changes []domain.MCPSchemaChange
	if prevVersion != nil {
		changes = g.computeChanges(prevVersion.ToolDefinitions, tools)
	} else {
		for _, tool := range tools {
			changes = append(changes, domain.MCPSchemaChange{
				Type:     domain.MCPChangeAdded,
				ToolName: tool.Name,
			})
		}
	}

	// Use server version as-is
	newVersion := serverVersion

	// Create version snapshot (UPSERT will handle insert or update)
	version := &domain.MCPServerVersion{
		ID:                 uuid.New().String(),
		ServerID:           server.ID,
		Version:            newVersion,
		CommitHash:         server.CommitHash,
		ToolDefinitions:    make([]domain.MCPTool, len(tools)),
		ToolCount:          len(tools),
		Changes:            changes,
		ChangesSummary:     g.summarizeChanges(changes),
		HasBreakingChanges: hasBreakingChanges(changes),
		CreatedAt:          time.Now(),
	}

	for i, t := range tools {
		version.ToolDefinitions[i] = *t
	}

	// Save version (uses UPSERT - inserts new or updates existing)
	if err := store.CreateMCPServerVersion(ctx, version); err != nil {
		return nil, fmt.Errorf("failed to save version: %w", err)
	}

	// Upsert tools and index embeddings
	for _, tool := range tools {
		if err := store.UpsertMCPTool(ctx, tool); err != nil {
			slog.Warn("Failed to upsert tool", "tool", tool.Name, "error", err)
			continue
		}

		// Index embeddings if embedder is available
		if g.embedder != nil {
			if err := g.indexToolEmbeddings(ctx, store, tool); err != nil {
				slog.Warn("Failed to index tool embeddings", "tool", tool.Name, "error", err)
			}
		}
	}

	// Mark removed tools as deprecated
	if prevVersion != nil {
		for _, change := range changes {
			if change.Type == domain.MCPChangeRemoved {
				store.DeprecateMCPTool(ctx, server.ID, change.ToolName, "Removed in version "+newVersion)
			}
		}
	}

	// Update server
	now := time.Now()
	server.LastSyncAt = &now
	server.Version = newVersion
	server.Status = domain.MCPStatusConnected
	server.ErrorMessage = "" // Clear any previous error
	server.RetryCount = 0    // Reset retry count
	server.ToolCount = len(tools)
	store.UpdateMCPServer(ctx, server)

	slog.Info("Synced MCP server",
		"server", server.Name,
		"version", newVersion,
		"tools", len(tools),
		"changes", len(changes),
	)

	return version, nil
}

// indexToolEmbeddings creates embeddings for a tool
func (g *Gateway) indexToolEmbeddings(ctx context.Context, store *postgres.TenantStore, tool *domain.MCPTool) error {
	// Create combined text for embedding
	combinedText := fmt.Sprintf("%s: %s", tool.Name, tool.Description)
	if len(tool.InputSchema) > 0 {
		schemaText := formatSchemaForEmbedding(tool.InputSchema)
		combinedText += "\nParameters: " + schemaText
	}

	// Generate embeddings
	nameEmb, err := g.embedder.Embed(ctx, tool.Name)
	if err != nil {
		return err
	}

	descEmb, err := g.embedder.Embed(ctx, tool.Description)
	if err != nil {
		return err
	}

	combinedEmb, err := g.embedder.Embed(ctx, combinedText)
	if err != nil {
		return err
	}

	return store.UpdateToolEmbeddings(ctx, tool.ID, nameEmb, descEmb, combinedEmb)
}

func (g *Gateway) computeChanges(oldTools []domain.MCPTool, newTools []*domain.MCPTool) []domain.MCPSchemaChange {
	var changes []domain.MCPSchemaChange

	oldMap := make(map[string]domain.MCPTool)
	for _, t := range oldTools {
		oldMap[t.Name] = t
	}

	newMap := make(map[string]*domain.MCPTool)
	for _, t := range newTools {
		newMap[t.Name] = t
	}

	// Check for added and modified tools
	for name, newTool := range newMap {
		if oldTool, exists := oldMap[name]; exists {
			// Check if schema changed
			oldSchema, _ := json.Marshal(oldTool.InputSchema)
			newSchema, _ := json.Marshal(newTool.InputSchema)
			if string(oldSchema) != string(newSchema) {
				changes = append(changes, domain.MCPSchemaChange{
					Type:     domain.MCPChangeModified,
					ToolName: name,
					Field:    "input_schema",
					Breaking: true,
				})
			}
		} else {
			changes = append(changes, domain.MCPSchemaChange{
				Type:     domain.MCPChangeAdded,
				ToolName: name,
			})
		}
	}

	// Check for removed tools
	for name := range oldMap {
		if _, exists := newMap[name]; !exists {
			changes = append(changes, domain.MCPSchemaChange{
				Type:     domain.MCPChangeRemoved,
				ToolName: name,
				Breaking: true,
			})
		}
	}

	return changes
}

func (g *Gateway) computeNextVersion(prev *domain.MCPServerVersion, changes []domain.MCPSchemaChange) string {
	if prev == nil {
		return "1.0.0"
	}

	parts := strings.Split(prev.Version, ".")
	if len(parts) != 3 {
		return "1.0.0"
	}

	major := parseInt(parts[0])
	minor := parseInt(parts[1])
	patch := parseInt(parts[2])

	hasBreaking := hasBreakingChanges(changes)
	hasNew := false
	for _, c := range changes {
		if c.Type == domain.MCPChangeAdded {
			hasNew = true
			break
		}
	}

	if hasBreaking {
		return fmt.Sprintf("%d.0.0", major+1)
	} else if hasNew {
		return fmt.Sprintf("%d.%d.0", major, minor+1)
	} else {
		return fmt.Sprintf("%d.%d.%d", major, minor, patch+1)
	}
}

func (g *Gateway) summarizeChanges(changes []domain.MCPSchemaChange) string {
	added := 0
	removed := 0
	modified := 0

	for _, c := range changes {
		switch c.Type {
		case domain.MCPChangeAdded:
			added++
		case domain.MCPChangeRemoved:
			removed++
		case domain.MCPChangeModified:
			modified++
		}
	}

	return fmt.Sprintf("%d added, %d removed, %d modified", added, removed, modified)
}

func hasBreakingChanges(changes []domain.MCPSchemaChange) bool {
	for _, c := range changes {
		if c.Breaking {
			return true
		}
	}
	return false
}

func parseInt(s string) int {
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}

func formatSchemaForEmbedding(schema map[string]any) string {
	var sb strings.Builder
	if props, ok := schema["properties"].(map[string]any); ok {
		for name, prop := range props {
			if propMap, ok := prop.(map[string]any); ok {
				sb.WriteString(fmt.Sprintf("- %s (%v): %v\n",
					name,
					propMap["type"],
					propMap["description"],
				))
			}
		}
	}
	return sb.String()
}

func inferToolCategory(name, description string) string {
	name = strings.ToLower(name)
	description = strings.ToLower(description)
	combined := name + " " + description

	categories := map[string][]string{
		"file-system": {"file", "read", "write", "directory", "folder", "path"},
		"git":         {"git", "commit", "branch", "repository", "pull", "push"},
		"database":    {"database", "sql", "query", "table", "record"},
		"api":         {"api", "http", "request", "endpoint", "rest"},
		"messaging":   {"message", "send", "slack", "email", "notification", "chat"},
		"calendar":    {"calendar", "event", "schedule", "meeting", "appointment"},
		"search":      {"search", "find", "query", "lookup"},
		"shell":       {"shell", "command", "execute", "run", "terminal"},
	}

	for category, keywords := range categories {
		for _, keyword := range keywords {
			if strings.Contains(combined, keyword) {
				return category
			}
		}
	}

	return "other"
}

// ============================================
// TOOL SEARCH
// ============================================

// SearchTools searches for tools using the specified strategy
// roleID is used to filter tools by visibility (ALLOW and SEARCH)
func (g *Gateway) SearchTools(ctx context.Context, tenantSlug string, roleID string, req *domain.ToolSearchRequest) (*domain.ToolSearchResponse, error) {
	g.mu.RLock()
	store, exists := g.stores[tenantSlug]
	g.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("tenant store not found: %s", tenantSlug)
	}

	// Check cache first (include roleID in cache key)
	cacheKey := fmt.Sprintf("%s:%s", roleID, req.Query)
	if result := g.searchCache.Get(tenantSlug, cacheKey); result != nil {
		return result, nil
	}

	// Use semantic search with embeddings for better natural language matching
	tools, scores, err := g.searchSemantic(ctx, store, req)
	if err != nil {
		return nil, err
	}

	// Filter by visibility - only ALLOW and SEARCH tools are searchable
	// If roleID is empty (admin view), skip visibility filtering
	var allowedTools []*domain.MCPTool
	if roleID == "" {
		// Admin view - return all tools without visibility filtering
		allowedTools = tools
	} else {
		// Filter by role visibility
		for _, tool := range tools {
			visibility := store.GetMCPToolVisibility(ctx, roleID, tool.ID)
			if visibility == domain.MCPVisibilityAllow || visibility == domain.MCPVisibilitySearch {
				allowedTools = append(allowedTools, tool)
			}
		}
	}

	// Build results with similarity scores from semantic search
	// Minimum similarity threshold - tools below this are considered irrelevant
	minScore := 0.5 // Cosine similarity threshold (0.5 = moderately similar)
	if req.MinScore > 0 {
		minScore = req.MinScore
	}

	var results []*domain.ToolSearchResult
	for _, tool := range allowedTools {
		// Get similarity score from semantic search
		score := 0.0
		for j, t := range tools {
			if t.ID == tool.ID && j < len(scores) {
				score = scores[j]
				break
			}
		}

		// Only include tools with sufficient similarity
		if score >= minScore {
			results = append(results, &domain.ToolSearchResult{
				Tool:         tool,
				ServerID:     tool.ServerID,
				ServerName:   tool.ServerName,
				Score:        score,
				DeferLoading: !req.IncludeSchema,
				ToolRef:      fmt.Sprintf("%s/%s", tool.ServerID, tool.Name),
			})
		}
	}

	// Sort by score descending (already sorted by semantic search, but re-sort after filtering)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Apply max results limit
	if req.MaxResults > 0 && len(results) > req.MaxResults {
		results = results[:req.MaxResults]
	}

	// TODO: [Future Enhancement] LLM-based reranking
	// Placeholder for future implementation:
	// results = g.rerankWithLLM(ctx, results, req.Query)
	// This would use an LLM to re-score results based on semantic understanding
	// of the query intent and tool capabilities.

	response := &domain.ToolSearchResponse{
		Tools:          results,
		Query:          req.Query,
		TotalAvailable: len(tools),
		TotalAllowed:   len(allowedTools),
	}

	// Cache result
	g.searchCache.Set(tenantSlug, cacheKey, response)

	return response, nil
}

// searchSemantic performs vector similarity search using embeddings
// Returns tools and their similarity scores
func (g *Gateway) searchSemantic(ctx context.Context, store *postgres.TenantStore, req *domain.ToolSearchRequest) ([]*domain.MCPTool, []float64, error) {
	if g.embedder == nil {
		return nil, nil, fmt.Errorf("embedder not configured - semantic search requires an embedding service")
	}

	// Embed the query
	embedding, err := g.embedder.Embed(ctx, req.Query)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to embed query: %w", err)
	}

	maxResults := req.MaxResults
	if maxResults == 0 {
		maxResults = 10
	}

	// Search by vector similarity - returns tools sorted by similarity
	return store.SearchToolsByVectorWithScores(ctx, embedding, maxResults)
}

// GetToolSearchTool returns the special tool_search tool definition
func (g *Gateway) GetToolSearchTool() *domain.Tool {
	return &domain.Tool{
		Type: "function",
		Function: domain.FunctionDefinition{
			Name: "tool_search",
			Description: `Search for available tools across all connected MCP servers.
Use this tool to discover capabilities before attempting to use specific tools.
The search understands natural language queries like "send a message to slack" or "create github issue".

Returns tool names, descriptions, and references that can be used to invoke the tools.`,
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "Natural language description of the capability you're looking for",
					},
					"category": map[string]any{
						"type":        "string",
						"description": "Optional category filter: messaging, file-system, database, api, git, calendar, other",
						"enum":        []string{"messaging", "file-system", "database", "api", "git", "calendar", "shell", "search", "other"},
					},
					"max_results": map[string]any{
						"type":        "integer",
						"description": "Maximum number of tools to return (default: 5)",
						"default":     5,
					},
				},
				"required": []string{"query"},
			},
		},
	}
}

// GetConnectionStatus returns the connection status of all servers
func (g *Gateway) GetConnectionStatus() map[string]domain.MCPServerStatus {
	g.mu.RLock()
	defer g.mu.RUnlock()

	status := make(map[string]domain.MCPServerStatus)
	for id, conn := range g.connections {
		status[id] = conn.Status
	}
	return status
}

// ============================================
// SEARCH CACHE
// ============================================

// SearchCache caches search results
type SearchCache struct {
	mu    sync.RWMutex
	cache map[string]*cacheEntry
	ttl   time.Duration
}

type cacheEntry struct {
	result    *domain.ToolSearchResponse
	expiresAt time.Time
}

func NewSearchCache(ttl time.Duration) *SearchCache {
	c := &SearchCache{
		cache: make(map[string]*cacheEntry),
		ttl:   ttl,
	}
	go c.cleanup()
	return c
}

func (c *SearchCache) Get(tenantID, query string) *domain.ToolSearchResponse {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := tenantID + ":" + query
	entry, exists := c.cache[key]
	if !exists || time.Now().After(entry.expiresAt) {
		return nil
	}
	return entry.result
}

func (c *SearchCache) Set(tenantID, query string, result *domain.ToolSearchResponse) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := tenantID + ":" + query
	c.cache[key] = &cacheEntry{
		result:    result,
		expiresAt: time.Now().Add(c.ttl),
	}
}

func (c *SearchCache) cleanup() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, entry := range c.cache {
			if now.After(entry.expiresAt) {
				delete(c.cache, key)
			}
		}
		c.mu.Unlock()
	}
}

// ============================================
// HELPER FUNCTIONS
// ============================================

// GetAllowedTools filters tools by visibility (ALLOW and SEARCH are searchable)
func (g *Gateway) GetAllowedTools(ctx context.Context, tenantSlug, roleID string, tools []*domain.ToolSearchResult) []*domain.ToolSearchResult {
	g.mu.RLock()
	store, exists := g.stores[tenantSlug]
	g.mu.RUnlock()

	if !exists {
		return nil
	}

	allowed := make([]*domain.ToolSearchResult, 0)
	for _, result := range tools {
		// Check visibility - ALLOW and SEARCH are both searchable
		visibility := store.GetMCPToolVisibility(ctx, roleID, result.Tool.ID)
		if visibility == domain.MCPVisibilityAllow || visibility == domain.MCPVisibilitySearch {
			allowed = append(allowed, result)
		}
		// DENY tools are not returned in search results
	}

	return allowed
}

// GetToolStats returns statistics about MCP tools
func (g *Gateway) GetToolStats(ctx context.Context, tenantSlug string) (map[string]int, error) {
	g.mu.RLock()
	store, exists := g.stores[tenantSlug]
	g.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("tenant store not found: %s", tenantSlug)
	}

	tools, err := store.ListAllMCPTools(ctx)
	if err != nil {
		return nil, err
	}

	stats := map[string]int{
		"total":       len(tools),
		"by_category": 0,
	}

	categories := make(map[string]int)
	for _, tool := range tools {
		categories[tool.Category]++
	}

	for _, count := range categories {
		stats["by_category"] += count
	}

	return stats, nil
}

// ListServers returns all MCP servers for a tenant
func (g *Gateway) ListServers(ctx context.Context, tenantSlug string) ([]*domain.MCPServer, error) {
	g.mu.RLock()
	store, exists := g.stores[tenantSlug]
	g.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("tenant store not found: %s", tenantSlug)
	}

	// List all MCP servers
	servers, err := store.ListMCPServers(ctx)
	if err != nil {
		return nil, err
	}

	// Update connection status
	for _, server := range servers {
		if conn, ok := g.connections[server.ID]; ok {
			server.Status = conn.Status
		}
	}

	// Sort by name
	sort.Slice(servers, func(i, j int) bool {
		return servers[i].Name < servers[j].Name
	})

	return servers, nil
}
