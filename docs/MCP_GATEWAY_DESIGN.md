# MCP Gateway Design Document

## Executive Summary

This document outlines the design for implementing a **per-tenant MCP (Model Context Protocol) Gateway** in ModelGate. The gateway will provide centralized management of MCP servers, intelligent tool discovery using RAG-based search, policy-driven tool access control, and version management.

The design is inspired by [Anthropic's Advanced Tool Use features](https://www.anthropic.com/engineering/advanced-tool-use), specifically the **Tool Search Tool** pattern which enables dynamic discovery of tools without consuming the context window.

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [MCP Server Management](#mcp-server-management)
3. [Tool Search Tool Implementation](#tool-search-tool-implementation)
4. [RAG-Based Tool Discovery](#rag-based-tool-discovery)
5. [Policy Integration](#policy-integration)
6. [Version Control](#version-control)
7. [Database Schema](#database-schema)
8. [API Design](#api-design)
9. [Implementation Plan](#implementation-plan)
10. [Security Considerations](#security-considerations)

---

## 1. Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────────┐
│                              Tenant Boundary                                     │
│  ┌─────────────────────────────────────────────────────────────────────────────┐│
│  │                          MCP Gateway (Per Tenant)                            ││
│  │                                                                              ││
│  │  ┌────────────────────────────────────────────────────────────────────────┐ ││
│  │  │                     Tool Search Tool Interface                          │ ││
│  │  │  ┌──────────────┐  ┌──────────────┐  ┌────────────────────────────────┐│ ││
│  │  │  │ Regex Search │  │ BM25 Search  │  │ Semantic Search (RAG + LLM)    ││ ││
│  │  │  │   (Fast)     │  │ (Standard)   │  │       (Most Accurate)          ││ ││
│  │  │  └──────────────┘  └──────────────┘  └────────────────────────────────┘│ ││
│  │  └────────────────────────────────────────────────────────────────────────┘ ││
│  │                                      │                                       ││
│  │                                      ▼                                       ││
│  │  ┌────────────────────────────────────────────────────────────────────────┐ ││
│  │  │                      Tool Registry & Cache                              │ ││
│  │  │  ┌──────────────────────────────────────────────────────────────────┐  │ ││
│  │  │  │ Vector Store (pgvector / Qdrant)                                  │  │ ││
│  │  │  │ - Tool name embeddings                                            │  │ ││
│  │  │  │ - Tool description embeddings                                     │  │ ││
│  │  │  │ - Parameter schema embeddings                                     │  │ ││
│  │  │  │ - Usage example embeddings                                        │  │ ││
│  │  │  └──────────────────────────────────────────────────────────────────┘  │ ││
│  │  └────────────────────────────────────────────────────────────────────────┘ ││
│  │                                      │                                       ││
│  │                                      ▼                                       ││
│  │  ┌────────────────────────────────────────────────────────────────────────┐ ││
│  │  │                       MCP Server Connections                            │ ││
│  │  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌────────────┐  │ ││
│  │  │  │ GitHub MCP   │  │ Slack MCP    │  │ Jira MCP     │  │ Custom MCP │  │ ││
│  │  │  │ v2.1.0       │  │ v1.5.0       │  │ v3.0.0       │  │ v1.0.0     │  │ ││
│  │  │  │ 35 tools     │  │ 11 tools     │  │ 25 tools     │  │ 5 tools    │  │ ││
│  │  │  └──────────────┘  └──────────────┘  └──────────────┘  └────────────┘  │ ││
│  │  └────────────────────────────────────────────────────────────────────────┘ ││
│  │                                                                              ││
│  │  ┌────────────────────────────────────────────────────────────────────────┐ ││
│  │  │                         Policy Layer                                    │ ││
│  │  │  ┌──────────────────────────────────────────────────────────────────┐  │ ││
│  │  │  │ Per-Role Tool Access Control                                      │  │ ││
│  │  │  │ - Allow/Deny/Remove per tool                                      │  │ ││
│  │  │  │ - Allow/Deny per MCP server                                       │  │ ││
│  │  │  │ - Tool categories (file-system, network, database, etc.)          │  │ ││
│  │  │  │ - Audit logging of all tool executions                            │  │ ││
│  │  │  └──────────────────────────────────────────────────────────────────┘  │ ││
│  │  └────────────────────────────────────────────────────────────────────────┘ ││
│  └─────────────────────────────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────────────────────────────┘
```

### Key Components

| Component | Purpose |
|-----------|---------|
| **Tool Search Tool** | Single interface for discovering tools across all MCP servers |
| **Tool Registry** | Stores tool definitions, embeddings, and metadata |
| **MCP Connection Manager** | Manages connections to multiple MCP servers |
| **Policy Layer** | Controls which tools are accessible per role |
| **Version Controller** | Tracks MCP server versions and tool schema changes |

---

## 2. MCP Server Management

### 2.1 MCP Server Configuration

Each tenant can register multiple MCP servers with various authentication methods:

```go
// internal/domain/mcp.go

// MCPServerType defines the transport type
type MCPServerType string

const (
    MCPServerTypeStdio  MCPServerType = "stdio"   // Local process
    MCPServerTypeSSE    MCPServerType = "sse"     // Server-Sent Events (HTTP)
    MCPServerTypeWebSocket MCPServerType = "websocket" // WebSocket
)

// MCPAuthType defines authentication methods
type MCPAuthType string

const (
    MCPAuthNone        MCPAuthType = "none"
    MCPAuthAPIKey      MCPAuthType = "api_key"
    MCPAuthOAuth2      MCPAuthType = "oauth2"
    MCPAuthBasic       MCPAuthType = "basic"
    MCPAuthMTLS        MCPAuthType = "mtls"
    MCPAuthAWSIAM      MCPAuthType = "aws_iam"
)

// MCPServer represents an MCP server configuration
type MCPServer struct {
    ID          string            `json:"id"`
    TenantID    string            `json:"tenant_id"`
    Name        string            `json:"name"`
    Description string            `json:"description"`
    
    // Connection settings
    ServerType  MCPServerType     `json:"server_type"`
    Endpoint    string            `json:"endpoint"`      // URL for SSE/WebSocket, command for stdio
    Arguments   []string          `json:"arguments"`     // For stdio: command arguments
    Environment map[string]string `json:"environment"`   // Environment variables
    
    // Authentication
    AuthType    MCPAuthType       `json:"auth_type"`
    AuthConfig  MCPAuthConfig     `json:"auth_config"`
    
    // Version control
    Version     string            `json:"version"`       // Semantic version
    CommitHash  string            `json:"commit_hash"`   // Git commit if applicable
    
    // Status
    Status      MCPServerStatus   `json:"status"`
    LastHealthCheck time.Time     `json:"last_health_check"`
    ToolCount   int               `json:"tool_count"`
    
    // Metadata
    Tags        []string          `json:"tags"`
    CreatedAt   time.Time         `json:"created_at"`
    UpdatedAt   time.Time         `json:"updated_at"`
    CreatedBy   string            `json:"created_by"`
}

// MCPAuthConfig stores authentication credentials
type MCPAuthConfig struct {
    // API Key auth
    APIKey       string `json:"api_key,omitempty"`
    APIKeyHeader string `json:"api_key_header,omitempty"` // e.g., "Authorization" or "X-API-Key"
    
    // OAuth2
    ClientID     string `json:"client_id,omitempty"`
    ClientSecret string `json:"client_secret,omitempty"`
    TokenURL     string `json:"token_url,omitempty"`
    Scopes       []string `json:"scopes,omitempty"`
    
    // Basic auth
    Username     string `json:"username,omitempty"`
    Password     string `json:"password,omitempty"`
    
    // mTLS
    ClientCert   string `json:"client_cert,omitempty"`   // PEM encoded
    ClientKey    string `json:"client_key,omitempty"`    // PEM encoded
    CACert       string `json:"ca_cert,omitempty"`       // PEM encoded
    
    // AWS IAM
    AWSRegion    string `json:"aws_region,omitempty"`
    AWSRoleARN   string `json:"aws_role_arn,omitempty"`
}

// MCPServerStatus represents connection status
type MCPServerStatus string

const (
    MCPStatusConnected    MCPServerStatus = "connected"
    MCPStatusDisconnected MCPServerStatus = "disconnected"
    MCPStatusError        MCPServerStatus = "error"
    MCPStatusPending      MCPServerStatus = "pending"
)
```

### 2.2 MCP Connection Manager

```go
// internal/mcp/connection_manager.go

type ConnectionManager struct {
    mu          sync.RWMutex
    connections map[string]*MCPConnection  // serverID -> connection
    toolCache   map[string][]*MCPTool      // serverID -> tools
    embedder    Embedder
    vectorStore VectorStore
}

type MCPConnection struct {
    Server      *domain.MCPServer
    Client      mcp.Client
    Status      domain.MCPServerStatus
    LastError   error
    RetryCount  int
    LastRetry   time.Time
}

// Connect establishes connection to an MCP server
func (m *ConnectionManager) Connect(ctx context.Context, server *domain.MCPServer) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    var client mcp.Client
    var err error
    
    switch server.ServerType {
    case domain.MCPServerTypeStdio:
        client, err = m.connectStdio(ctx, server)
    case domain.MCPServerTypeSSE:
        client, err = m.connectSSE(ctx, server)
    case domain.MCPServerTypeWebSocket:
        client, err = m.connectWebSocket(ctx, server)
    default:
        return fmt.Errorf("unsupported server type: %s", server.ServerType)
    }
    
    if err != nil {
        return err
    }
    
    // Initialize and list tools
    if err := client.Initialize(ctx); err != nil {
        return fmt.Errorf("failed to initialize MCP client: %w", err)
    }
    
    tools, err := client.ListTools(ctx)
    if err != nil {
        return fmt.Errorf("failed to list tools: %w", err)
    }
    
    // Store connection and tools
    m.connections[server.ID] = &MCPConnection{
        Server: server,
        Client: client,
        Status: domain.MCPStatusConnected,
    }
    m.toolCache[server.ID] = tools
    
    // Index tools in vector store
    if err := m.indexTools(ctx, server.ID, tools); err != nil {
        slog.Warn("Failed to index tools in vector store", "server_id", server.ID, "error", err)
    }
    
    return nil
}

// connectSSE connects to an SSE-based MCP server with authentication
func (m *ConnectionManager) connectSSE(ctx context.Context, server *domain.MCPServer) (mcp.Client, error) {
    transport := &http.Transport{
        TLSClientConfig: &tls.Config{},
    }
    
    // Configure mTLS if needed
    if server.AuthType == domain.MCPAuthMTLS {
        cert, err := tls.X509KeyPair(
            []byte(server.AuthConfig.ClientCert),
            []byte(server.AuthConfig.ClientKey),
        )
        if err != nil {
            return nil, fmt.Errorf("failed to load client certificate: %w", err)
        }
        
        caCertPool := x509.NewCertPool()
        caCertPool.AppendCertsFromPEM([]byte(server.AuthConfig.CACert))
        
        transport.TLSClientConfig = &tls.Config{
            Certificates: []tls.Certificate{cert},
            RootCAs:      caCertPool,
        }
    }
    
    httpClient := &http.Client{Transport: transport}
    
    // Create SSE client with auth
    sseClient := mcp.NewSSEClient(server.Endpoint, httpClient)
    
    // Add authentication
    switch server.AuthType {
    case domain.MCPAuthAPIKey:
        sseClient.SetHeader(server.AuthConfig.APIKeyHeader, server.AuthConfig.APIKey)
    case domain.MCPAuthBasic:
        auth := base64.StdEncoding.EncodeToString(
            []byte(server.AuthConfig.Username + ":" + server.AuthConfig.Password),
        )
        sseClient.SetHeader("Authorization", "Basic "+auth)
    case domain.MCPAuthOAuth2:
        token, err := m.getOAuth2Token(ctx, server.AuthConfig)
        if err != nil {
            return nil, fmt.Errorf("failed to get OAuth2 token: %w", err)
        }
        sseClient.SetHeader("Authorization", "Bearer "+token)
    }
    
    return sseClient, nil
}
```

---

## 3. Tool Search Tool Implementation

Based on [Anthropic's Tool Search Tool](https://www.anthropic.com/engineering/advanced-tool-use#tool-search-tool), we implement three search strategies:

### 3.1 Search Strategies

```go
// internal/mcp/tool_search.go

// ToolSearcher provides unified tool search across all MCP servers
type ToolSearcher struct {
    vectorStore   VectorStore
    bm25Index     *BM25Index
    regexCache    map[string]*regexp.Regexp
    toolRegistry  *ToolRegistry
    llmClient     LLMClient  // For semantic re-ranking
}

// SearchStrategy defines how to search for tools
type SearchStrategy string

const (
    SearchStrategyRegex    SearchStrategy = "regex"     // Fast, pattern matching
    SearchStrategyBM25     SearchStrategy = "bm25"      // Standard text search
    SearchStrategySemantic SearchStrategy = "semantic"  // RAG + LLM (most accurate)
    SearchStrategyHybrid   SearchStrategy = "hybrid"    // Combines all strategies
)

// ToolSearchRequest represents a search query
type ToolSearchRequest struct {
    Query       string          `json:"query"`
    Strategy    SearchStrategy  `json:"strategy"`
    ServerIDs   []string        `json:"server_ids,omitempty"`   // Filter by server
    Categories  []string        `json:"categories,omitempty"`   // Filter by category
    MaxResults  int             `json:"max_results"`
    MinScore    float64         `json:"min_score"`
    IncludeSchema bool          `json:"include_schema"`         // Include full schema in response
}

// ToolSearchResult represents a matched tool
type ToolSearchResult struct {
    Tool        *MCPTool  `json:"tool"`
    ServerID    string    `json:"server_id"`
    ServerName  string    `json:"server_name"`
    Score       float64   `json:"score"`
    MatchReason string    `json:"match_reason"`
    
    // Deferred loading support (per Anthropic's pattern)
    DeferLoading bool     `json:"defer_loading"`
    ToolRef      string   `json:"tool_ref"`  // Reference for deferred expansion
}

// Search performs tool search using the specified strategy
func (s *ToolSearcher) Search(ctx context.Context, tenantID string, req *ToolSearchRequest) ([]*ToolSearchResult, error) {
    switch req.Strategy {
    case SearchStrategyRegex:
        return s.searchRegex(ctx, tenantID, req)
    case SearchStrategyBM25:
        return s.searchBM25(ctx, tenantID, req)
    case SearchStrategySemantic:
        return s.searchSemantic(ctx, tenantID, req)
    case SearchStrategyHybrid:
        return s.searchHybrid(ctx, tenantID, req)
    default:
        return s.searchHybrid(ctx, tenantID, req)
    }
}

// searchSemantic uses embeddings and LLM for most accurate search
func (s *ToolSearcher) searchSemantic(ctx context.Context, tenantID string, req *ToolSearchRequest) ([]*ToolSearchResult, error) {
    // 1. Generate embedding for the query
    queryEmbedding, err := s.embedder.Embed(ctx, req.Query)
    if err != nil {
        return nil, fmt.Errorf("failed to embed query: %w", err)
    }
    
    // 2. Search vector store for similar tools
    vectorResults, err := s.vectorStore.Search(ctx, VectorSearchRequest{
        TenantID:  tenantID,
        Embedding: queryEmbedding,
        TopK:      req.MaxResults * 3,  // Get more for re-ranking
        Filter: VectorFilter{
            ServerIDs:  req.ServerIDs,
            Categories: req.Categories,
        },
    })
    if err != nil {
        return nil, fmt.Errorf("vector search failed: %w", err)
    }
    
    // 3. LLM re-ranking for precision
    rerankedResults, err := s.rerankWithLLM(ctx, req.Query, vectorResults)
    if err != nil {
        slog.Warn("LLM re-ranking failed, using vector results", "error", err)
        rerankedResults = vectorResults
    }
    
    // 4. Filter by score and limit
    results := make([]*ToolSearchResult, 0, req.MaxResults)
    for _, vr := range rerankedResults {
        if vr.Score < req.MinScore {
            continue
        }
        if len(results) >= req.MaxResults {
            break
        }
        
        tool, err := s.toolRegistry.GetTool(ctx, vr.ToolID)
        if err != nil {
            continue
        }
        
        results = append(results, &ToolSearchResult{
            Tool:         tool,
            ServerID:     vr.ServerID,
            ServerName:   vr.ServerName,
            Score:        vr.Score,
            MatchReason:  vr.MatchReason,
            DeferLoading: !req.IncludeSchema,
            ToolRef:      fmt.Sprintf("%s/%s", vr.ServerID, tool.Name),
        })
    }
    
    return results, nil
}

// rerankWithLLM uses an LLM to re-rank search results for precision
func (s *ToolSearcher) rerankWithLLM(ctx context.Context, query string, results []VectorSearchResult) ([]VectorSearchResult, error) {
    if len(results) == 0 {
        return results, nil
    }
    
    // Build prompt for re-ranking
    prompt := fmt.Sprintf(`Given the user query: "%s"

Rank the following tools by relevance (most relevant first). Return only the tool IDs in order.

Tools:
`, query)
    
    for i, r := range results {
        prompt += fmt.Sprintf("%d. %s: %s\n", i+1, r.ToolName, r.ToolDescription)
    }
    
    // Call LLM for re-ranking
    response, err := s.llmClient.Complete(ctx, &domain.ChatRequest{
        Model: "openai/gpt-4o-mini",  // Use fast model for re-ranking
        Messages: []domain.Message{
            {Role: "system", Content: []domain.ContentBlock{{Type: "text", Text: "You are a tool ranking assistant. Return only the tool numbers in order of relevance, comma-separated."}}},
            {Role: "user", Content: []domain.ContentBlock{{Type: "text", Text: prompt}}},
        },
        MaxTokens: 100,
    })
    if err != nil {
        return nil, err
    }
    
    // Parse ranking and reorder results
    // ... parsing logic
    
    return results, nil
}
```

### 3.2 Tool Search Tool API

The Tool Search Tool itself is exposed as a special tool that Claude can call:

```go
// internal/mcp/tool_search_tool.go

// ToolSearchTool is a special tool that enables dynamic tool discovery
// This follows Anthropic's pattern: https://www.anthropic.com/engineering/advanced-tool-use
var ToolSearchTool = &domain.Tool{
    Type: "function",
    Function: domain.Function{
        Name:        "tool_search",
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
                    "description": "Optional category filter: messaging, file-system, database, api, etc.",
                    "enum":        []string{"messaging", "file-system", "database", "api", "git", "calendar", "other"},
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

// HandleToolSearch executes the tool search
func (g *MCPGateway) HandleToolSearch(ctx context.Context, tenantID, roleID string, args map[string]any) (*ToolSearchResponse, error) {
    query, _ := args["query"].(string)
    category, _ := args["category"].(string)
    maxResults := 5
    if mr, ok := args["max_results"].(float64); ok {
        maxResults = int(mr)
    }
    
    // Get allowed tools for this role from policy
    policy, err := g.policyService.GetRolePolicy(ctx, tenantID, roleID)
    if err != nil {
        return nil, err
    }
    
    // Build search request
    req := &ToolSearchRequest{
        Query:      query,
        Strategy:   SearchStrategyHybrid,
        MaxResults: maxResults,
    }
    if category != "" {
        req.Categories = []string{category}
    }
    
    // Search for tools
    results, err := g.toolSearcher.Search(ctx, tenantID, req)
    if err != nil {
        return nil, err
    }
    
    // Filter by policy
    allowedResults := make([]*ToolSearchResult, 0, len(results))
    for _, r := range results {
        // Check if tool is allowed by policy
        permission := g.getToolPermission(policy, r.ServerID, r.Tool.Name)
        if permission == domain.ToolStatusAllowed || permission == domain.ToolStatusPending {
            allowedResults = append(allowedResults, r)
        }
    }
    
    return &ToolSearchResponse{
        Tools: allowedResults,
        Query: query,
        TotalAvailable: len(results),
        TotalAllowed:   len(allowedResults),
    }, nil
}
```

---

## 4. RAG-Based Tool Discovery

### 4.1 Vector Store Schema

Using PostgreSQL with pgvector extension for embeddings:

```sql
-- migrations/tenant/008_mcp_gateway.sql

-- Enable pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- MCP Servers table
CREATE TABLE mcp_servers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    
    -- Connection
    server_type VARCHAR(50) NOT NULL,  -- stdio, sse, websocket
    endpoint TEXT NOT NULL,
    arguments JSONB DEFAULT '[]',
    environment JSONB DEFAULT '{}',
    
    -- Authentication
    auth_type VARCHAR(50) NOT NULL DEFAULT 'none',
    auth_config JSONB DEFAULT '{}',  -- Encrypted
    
    -- Version control
    version VARCHAR(50),
    commit_hash VARCHAR(64),
    last_sync_at TIMESTAMPTZ,
    
    -- Status
    status VARCHAR(50) DEFAULT 'pending',
    last_health_check TIMESTAMPTZ,
    error_message TEXT,
    
    -- Metadata
    tags TEXT[] DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    created_by UUID,
    
    CONSTRAINT fk_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id),
    CONSTRAINT unique_server_name UNIQUE (tenant_id, name)
);

-- MCP Server versions for history
CREATE TABLE mcp_server_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id UUID NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    version VARCHAR(50) NOT NULL,
    commit_hash VARCHAR(64),
    tool_definitions JSONB NOT NULL,  -- Snapshot of all tool definitions
    changes_summary TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    created_by UUID,
    
    CONSTRAINT unique_version UNIQUE (server_id, version)
);

-- MCP Tools table (discovered from servers)
CREATE TABLE mcp_tools (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    server_id UUID NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    
    -- Tool identity
    name VARCHAR(255) NOT NULL,
    description TEXT,
    category VARCHAR(100),
    
    -- Schema
    input_schema JSONB NOT NULL,
    input_examples JSONB DEFAULT '[]',  -- Per Anthropic's Tool Use Examples
    
    -- Embeddings for semantic search
    name_embedding vector(1536),
    description_embedding vector(1536),
    combined_embedding vector(1536),  -- Name + description + schema
    
    -- Metadata
    version VARCHAR(50),
    is_deprecated BOOLEAN DEFAULT FALSE,
    deprecation_message TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    
    CONSTRAINT unique_tool UNIQUE (server_id, name)
);

-- Create vector indexes for fast similarity search
CREATE INDEX idx_mcp_tools_name_embedding ON mcp_tools 
    USING ivfflat (name_embedding vector_cosine_ops) WITH (lists = 100);
    
CREATE INDEX idx_mcp_tools_description_embedding ON mcp_tools 
    USING ivfflat (description_embedding vector_cosine_ops) WITH (lists = 100);
    
CREATE INDEX idx_mcp_tools_combined_embedding ON mcp_tools 
    USING ivfflat (combined_embedding vector_cosine_ops) WITH (lists = 100);

-- MCP Tool permissions (extends existing tool_role_permissions)
CREATE TABLE mcp_tool_permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    role_id UUID NOT NULL,
    
    -- Can be tool-level or server-level
    server_id UUID REFERENCES mcp_servers(id) ON DELETE CASCADE,
    tool_id UUID REFERENCES mcp_tools(id) ON DELETE CASCADE,
    
    -- Permission
    status VARCHAR(20) NOT NULL CHECK (status IN ('PENDING', 'ALLOWED', 'DENIED', 'REMOVED')),
    
    -- Audit
    decided_by UUID,
    decided_by_email VARCHAR(255),
    decided_at TIMESTAMPTZ,
    decision_reason TEXT,
    
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    
    -- Either tool_id or server_id must be set, not both
    CONSTRAINT check_permission_target CHECK (
        (tool_id IS NOT NULL AND server_id IS NULL) OR
        (tool_id IS NULL AND server_id IS NOT NULL)
    )
);

-- Full-text search index for BM25-style search
CREATE INDEX idx_mcp_tools_fts ON mcp_tools 
    USING gin(to_tsvector('english', name || ' ' || COALESCE(description, '')));
```

### 4.2 Embedding Service

```go
// internal/mcp/embeddings.go

type Embedder interface {
    Embed(ctx context.Context, text string) ([]float32, error)
    EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

// OpenAIEmbedder uses OpenAI's text-embedding-3-small model
type OpenAIEmbedder struct {
    client *openai.Client
    model  string
}

func NewOpenAIEmbedder(apiKey string) *OpenAIEmbedder {
    return &OpenAIEmbedder{
        client: openai.NewClient(apiKey),
        model:  "text-embedding-3-small",  // 1536 dimensions, cost-effective
    }
}

func (e *OpenAIEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
    resp, err := e.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
        Model: e.model,
        Input: []string{text},
    })
    if err != nil {
        return nil, err
    }
    
    // Convert float64 to float32 for pgvector
    embedding := make([]float32, len(resp.Data[0].Embedding))
    for i, v := range resp.Data[0].Embedding {
        embedding[i] = float32(v)
    }
    return embedding, nil
}

// ToolIndexer creates embeddings for tool discovery
type ToolIndexer struct {
    embedder Embedder
    store    *TenantStore
}

// IndexTool creates embeddings for a tool
func (i *ToolIndexer) IndexTool(ctx context.Context, tool *MCPTool) error {
    // Create combined text for embedding
    combinedText := fmt.Sprintf("%s: %s\nParameters: %s",
        tool.Name,
        tool.Description,
        formatSchemaForEmbedding(tool.InputSchema),
    )
    
    // Generate embeddings
    nameEmb, err := i.embedder.Embed(ctx, tool.Name)
    if err != nil {
        return err
    }
    
    descEmb, err := i.embedder.Embed(ctx, tool.Description)
    if err != nil {
        return err
    }
    
    combinedEmb, err := i.embedder.Embed(ctx, combinedText)
    if err != nil {
        return err
    }
    
    // Store in database
    return i.store.UpdateToolEmbeddings(ctx, tool.ID, nameEmb, descEmb, combinedEmb)
}

func formatSchemaForEmbedding(schema map[string]any) string {
    // Convert schema to human-readable format for better embeddings
    var sb strings.Builder
    
    if props, ok := schema["properties"].(map[string]any); ok {
        for name, prop := range props {
            propMap := prop.(map[string]any)
            sb.WriteString(fmt.Sprintf("- %s (%s): %s\n",
                name,
                propMap["type"],
                propMap["description"],
            ))
        }
    }
    
    return sb.String()
}
```

---

## 5. Policy Integration

### 5.1 MCP Policy in Role Configuration

```go
// internal/domain/rbac.go - Extended

// EnhancedMCPPolicies defines MCP-specific access control
type EnhancedMCPPolicies struct {
    // Global settings
    Enabled           bool     `json:"enabled"`
    AllowToolSearch   bool     `json:"allow_tool_search"`   // Allow use of tool_search tool
    DefaultAction     string   `json:"default_action"`      // ALLOW, DENY, or REQUIRE_APPROVAL
    
    // Server-level permissions
    ServerPermissions []MCPServerPermission `json:"server_permissions"`
    
    // Tool-level overrides
    ToolOverrides     []MCPToolOverride     `json:"tool_overrides"`
    
    // Category restrictions
    AllowedCategories []string `json:"allowed_categories"`
    DeniedCategories  []string `json:"denied_categories"`
    
    // Audit settings
    LogAllToolCalls   bool     `json:"log_all_tool_calls"`
    RequireJustification bool  `json:"require_justification"`  // User must explain why
}

type MCPServerPermission struct {
    ServerID    string `json:"server_id"`
    ServerName  string `json:"server_name"`  // For display
    Permission  string `json:"permission"`   // ALLOWED, DENIED
    AllowedTools []string `json:"allowed_tools,omitempty"`  // If set, only these tools are allowed
    DeniedTools  []string `json:"denied_tools,omitempty"`   // If set, these tools are denied
}

type MCPToolOverride struct {
    ServerID   string `json:"server_id"`
    ToolName   string `json:"tool_name"`
    Permission string `json:"permission"`  // ALLOWED, DENIED, REMOVED
    Reason     string `json:"reason,omitempty"`
}
```

### 5.2 Policy UI Component

```typescript
// web/src/components/policies/MCPPolicyEditor.tsx

interface MCPServer {
  id: string
  name: string
  description: string
  status: 'connected' | 'disconnected' | 'error'
  toolCount: number
  version: string
}

interface MCPTool {
  id: string
  serverId: string
  serverName: string
  name: string
  description: string
  category: string
  permission: 'PENDING' | 'ALLOWED' | 'DENIED' | 'REMOVED'
}

export function MCPPolicyEditor({ roleId, tenantSlug }: Props) {
  const [servers, setServers] = useState<MCPServer[]>([])
  const [tools, setTools] = useState<MCPTool[]>([])
  const [selectedServer, setSelectedServer] = useState<string | null>(null)
  const [searchQuery, setSearchQuery] = useState('')
  
  // Filter tools by server and search query
  const filteredTools = useMemo(() => {
    return tools.filter(tool => {
      const matchesServer = !selectedServer || tool.serverId === selectedServer
      const matchesSearch = !searchQuery || 
        tool.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
        tool.description.toLowerCase().includes(searchQuery.toLowerCase())
      return matchesServer && matchesSearch
    })
  }, [tools, selectedServer, searchQuery])
  
  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex justify-between items-center">
        <div>
          <h3 className="text-lg font-semibold">MCP Tool Access</h3>
          <p className="text-sm text-muted-foreground">
            Configure which MCP tools this role can access
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" size="sm">
            <RefreshCw className="h-4 w-4 mr-1" />
            Sync Tools
          </Button>
          <Button variant="outline" size="sm">
            Allow All Pending
          </Button>
          <Button variant="outline" size="sm">
            Deny All Pending
          </Button>
        </div>
      </div>
      
      {/* Server Filter */}
      <div className="flex gap-4">
        <div className="flex-1">
          <Input
            placeholder="Search tools..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="max-w-md"
          />
        </div>
        <Select value={selectedServer || 'all'} onValueChange={(v) => setSelectedServer(v === 'all' ? null : v)}>
          <SelectTrigger className="w-48">
            <SelectValue placeholder="All Servers" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All Servers</SelectItem>
            {servers.map(server => (
              <SelectItem key={server.id} value={server.id}>
                <div className="flex items-center gap-2">
                  <span className={cn(
                    "h-2 w-2 rounded-full",
                    server.status === 'connected' ? "bg-green-500" :
                    server.status === 'error' ? "bg-red-500" : "bg-gray-400"
                  )} />
                  {server.name}
                  <Badge variant="outline" className="ml-1">{server.toolCount}</Badge>
                </div>
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      
      {/* Server-level permissions */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Server Permissions</CardTitle>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Server</TableHead>
                <TableHead>Version</TableHead>
                <TableHead>Tools</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Permission</TableHead>
                <TableHead>Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {servers.map(server => (
                <TableRow key={server.id}>
                  <TableCell>
                    <div>
                      <div className="font-medium">{server.name}</div>
                      <div className="text-sm text-muted-foreground">{server.description}</div>
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge variant="outline">{server.version}</Badge>
                  </TableCell>
                  <TableCell>{server.toolCount}</TableCell>
                  <TableCell>
                    <Badge variant={server.status === 'connected' ? 'default' : 'destructive'}>
                      {server.status}
                    </Badge>
                  </TableCell>
                  <TableCell>
                    <Select defaultValue="ALLOWED">
                      <SelectTrigger className="w-32">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="ALLOWED">Allow All</SelectItem>
                        <SelectItem value="DENIED">Deny All</SelectItem>
                        <SelectItem value="CUSTOM">Custom</SelectItem>
                      </SelectContent>
                    </Select>
                  </TableCell>
                  <TableCell>
                    <Button variant="ghost" size="sm">
                      Configure Tools
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
      
      {/* Tool-level permissions */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Tool Permissions</CardTitle>
          <CardDescription>
            {filteredTools.length} tools • 
            {filteredTools.filter(t => t.permission === 'ALLOWED').length} allowed • 
            {filteredTools.filter(t => t.permission === 'PENDING').length} pending
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Tool</TableHead>
                <TableHead>Server</TableHead>
                <TableHead>Category</TableHead>
                <TableHead>Permission</TableHead>
                <TableHead>Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filteredTools.map(tool => (
                <TableRow key={tool.id}>
                  <TableCell>
                    <div>
                      <code className="text-sm font-mono">{tool.name}</code>
                      <div className="text-sm text-muted-foreground line-clamp-1">
                        {tool.description}
                      </div>
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge variant="outline">{tool.serverName}</Badge>
                  </TableCell>
                  <TableCell>
                    <Badge variant="secondary">{tool.category}</Badge>
                  </TableCell>
                  <TableCell>
                    <ToolPermissionBadge permission={tool.permission} />
                  </TableCell>
                  <TableCell>
                    <div className="flex gap-1">
                      <Button variant="ghost" size="sm" onClick={() => setPermission(tool.id, 'ALLOWED')}>
                        <Check className="h-4 w-4" />
                      </Button>
                      <Button variant="ghost" size="sm" onClick={() => setPermission(tool.id, 'DENIED')}>
                        <X className="h-4 w-4" />
                      </Button>
                      <Button variant="ghost" size="sm" onClick={() => setPermission(tool.id, 'REMOVED')}>
                        <EyeOff className="h-4 w-4" />
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  )
}
```

---

## 6. Version Control

### 6.1 MCP Server Versioning

```go
// internal/mcp/version_control.go

type VersionController struct {
    store *TenantStore
    differ *SchemaDiffer
}

// MCPServerVersion represents a version snapshot
type MCPServerVersion struct {
    ID              string              `json:"id"`
    ServerID        string              `json:"server_id"`
    Version         string              `json:"version"`
    CommitHash      string              `json:"commit_hash,omitempty"`
    ToolDefinitions []MCPTool           `json:"tool_definitions"`
    ChangesSummary  string              `json:"changes_summary"`
    Changes         []SchemaChange      `json:"changes"`
    CreatedAt       time.Time           `json:"created_at"`
    CreatedBy       string              `json:"created_by"`
}

// SchemaChange represents a change between versions
type SchemaChange struct {
    Type       ChangeType `json:"type"`       // ADDED, REMOVED, MODIFIED
    ToolName   string     `json:"tool_name"`
    Field      string     `json:"field,omitempty"`
    OldValue   any        `json:"old_value,omitempty"`
    NewValue   any        `json:"new_value,omitempty"`
    Breaking   bool       `json:"breaking"`   // Is this a breaking change?
}

type ChangeType string

const (
    ChangeTypeAdded    ChangeType = "ADDED"
    ChangeTypeRemoved  ChangeType = "REMOVED"
    ChangeTypeModified ChangeType = "MODIFIED"
)

// SyncServer syncs tools from an MCP server and creates a version snapshot
func (v *VersionController) SyncServer(ctx context.Context, serverID string) (*MCPServerVersion, error) {
    // Get current tools from server
    server, err := v.store.GetMCPServer(ctx, serverID)
    if err != nil {
        return nil, err
    }
    
    conn, err := v.connectionManager.GetConnection(serverID)
    if err != nil {
        return nil, err
    }
    
    currentTools, err := conn.Client.ListTools(ctx)
    if err != nil {
        return nil, err
    }
    
    // Get previous version
    prevVersion, err := v.store.GetLatestVersion(ctx, serverID)
    if err != nil && !errors.Is(err, sql.ErrNoRows) {
        return nil, err
    }
    
    // Compute changes
    var changes []SchemaChange
    if prevVersion != nil {
        changes = v.differ.ComputeChanges(prevVersion.ToolDefinitions, currentTools)
    } else {
        // First version - all tools are new
        for _, tool := range currentTools {
            changes = append(changes, SchemaChange{
                Type:     ChangeTypeAdded,
                ToolName: tool.Name,
            })
        }
    }
    
    // Determine new version number
    newVersion := v.computeNextVersion(prevVersion, changes)
    
    // Create version snapshot
    version := &MCPServerVersion{
        ID:              uuid.New().String(),
        ServerID:        serverID,
        Version:         newVersion,
        CommitHash:      server.CommitHash,
        ToolDefinitions: currentTools,
        Changes:         changes,
        ChangesSummary:  v.summarizeChanges(changes),
        CreatedAt:       time.Now(),
    }
    
    // Store version
    if err := v.store.CreateMCPServerVersion(ctx, version); err != nil {
        return nil, err
    }
    
    // Update tools in database and re-index embeddings
    for _, tool := range currentTools {
        if err := v.store.UpsertMCPTool(ctx, serverID, &tool); err != nil {
            slog.Warn("Failed to upsert tool", "tool", tool.Name, "error", err)
        }
        
        // Re-index for search
        if err := v.indexer.IndexTool(ctx, &tool); err != nil {
            slog.Warn("Failed to index tool", "tool", tool.Name, "error", err)
        }
    }
    
    // Mark removed tools as deprecated
    if prevVersion != nil {
        for _, change := range changes {
            if change.Type == ChangeTypeRemoved {
                if err := v.store.DeprecateTool(ctx, serverID, change.ToolName, "Tool removed in version "+newVersion); err != nil {
                    slog.Warn("Failed to deprecate tool", "tool", change.ToolName, "error", err)
                }
            }
        }
    }
    
    return version, nil
}

// computeNextVersion determines the next semantic version
func (v *VersionController) computeNextVersion(prev *MCPServerVersion, changes []SchemaChange) string {
    if prev == nil {
        return "1.0.0"
    }
    
    // Parse current version
    parts := strings.Split(prev.Version, ".")
    if len(parts) != 3 {
        return "1.0.0"
    }
    
    major, _ := strconv.Atoi(parts[0])
    minor, _ := strconv.Atoi(parts[1])
    patch, _ := strconv.Atoi(parts[2])
    
    // Determine version bump based on changes
    hasBreaking := false
    hasNew := false
    
    for _, change := range changes {
        if change.Breaking {
            hasBreaking = true
        }
        if change.Type == ChangeTypeAdded {
            hasNew = true
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

// RollbackTo rolls back a server to a previous version
func (v *VersionController) RollbackTo(ctx context.Context, serverID, versionID string) error {
    version, err := v.store.GetMCPServerVersion(ctx, versionID)
    if err != nil {
        return err
    }
    
    // Clear current tools
    if err := v.store.DeleteMCPTools(ctx, serverID); err != nil {
        return err
    }
    
    // Restore tools from version snapshot
    for _, tool := range version.ToolDefinitions {
        if err := v.store.UpsertMCPTool(ctx, serverID, &tool); err != nil {
            return err
        }
        
        // Re-index
        if err := v.indexer.IndexTool(ctx, &tool); err != nil {
            slog.Warn("Failed to index tool", "tool", tool.Name, "error", err)
        }
    }
    
    return nil
}
```

### 6.2 Version History UI

```typescript
// web/src/components/mcp/VersionHistory.tsx

export function VersionHistory({ serverId }: { serverId: string }) {
  const { data: versions } = useQuery(GET_MCP_SERVER_VERSIONS, {
    variables: { serverId }
  })
  
  return (
    <Card>
      <CardHeader>
        <CardTitle>Version History</CardTitle>
        <CardDescription>
          Track changes to tool definitions across versions
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className="space-y-4">
          {versions?.map((version, index) => (
            <div key={version.id} className="relative pl-6 pb-6 border-l-2 border-border last:pb-0">
              <div className="absolute -left-2 top-0 h-4 w-4 rounded-full bg-primary" />
              
              <div className="flex justify-between items-start">
                <div>
                  <div className="flex items-center gap-2">
                    <Badge variant="outline" className="font-mono">
                      v{version.version}
                    </Badge>
                    {index === 0 && (
                      <Badge variant="default">Current</Badge>
                    )}
                  </div>
                  <p className="text-sm text-muted-foreground mt-1">
                    {format(new Date(version.createdAt), 'PPp')}
                  </p>
                </div>
                
                <div className="flex gap-2">
                  <Button variant="outline" size="sm">
                    View Changes
                  </Button>
                  {index > 0 && (
                    <Button variant="outline" size="sm">
                      Rollback
                    </Button>
                  )}
                </div>
              </div>
              
              <div className="mt-3">
                <p className="text-sm">{version.changesSummary}</p>
                
                <div className="flex gap-4 mt-2 text-sm">
                  <span className="text-green-600">
                    +{version.changes.filter(c => c.type === 'ADDED').length} added
                  </span>
                  <span className="text-red-600">
                    -{version.changes.filter(c => c.type === 'REMOVED').length} removed
                  </span>
                  <span className="text-yellow-600">
                    ~{version.changes.filter(c => c.type === 'MODIFIED').length} modified
                  </span>
                </div>
                
                {version.changes.some(c => c.breaking) && (
                  <Badge variant="destructive" className="mt-2">
                    Contains Breaking Changes
                  </Badge>
                )}
              </div>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  )
}
```

---

## 7. Complete Database Schema

```sql
-- migrations/tenant/008_mcp_gateway.sql (complete)

-- Enable pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- ============================================
-- MCP SERVERS
-- ============================================

CREATE TABLE mcp_servers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    
    -- Connection
    server_type VARCHAR(50) NOT NULL CHECK (server_type IN ('stdio', 'sse', 'websocket')),
    endpoint TEXT NOT NULL,
    arguments JSONB DEFAULT '[]',
    environment JSONB DEFAULT '{}',
    
    -- Authentication (encrypted at application level)
    auth_type VARCHAR(50) NOT NULL DEFAULT 'none' 
        CHECK (auth_type IN ('none', 'api_key', 'oauth2', 'basic', 'mtls', 'aws_iam')),
    auth_config_encrypted BYTEA,  -- Encrypted JSON
    
    -- Version control
    version VARCHAR(50) DEFAULT '0.0.0',
    commit_hash VARCHAR(64),
    last_sync_at TIMESTAMPTZ,
    
    -- Status
    status VARCHAR(50) DEFAULT 'pending' 
        CHECK (status IN ('pending', 'connected', 'disconnected', 'error', 'disabled')),
    last_health_check TIMESTAMPTZ,
    error_message TEXT,
    retry_count INTEGER DEFAULT 0,
    
    -- Settings
    auto_sync BOOLEAN DEFAULT TRUE,
    sync_interval_minutes INTEGER DEFAULT 60,
    health_check_interval_seconds INTEGER DEFAULT 30,
    
    -- Metadata
    tags TEXT[] DEFAULT '{}',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    created_by UUID,
    
    CONSTRAINT unique_server_name UNIQUE (tenant_id, name)
);

CREATE INDEX idx_mcp_servers_tenant ON mcp_servers(tenant_id);
CREATE INDEX idx_mcp_servers_status ON mcp_servers(status);

-- ============================================
-- MCP SERVER VERSIONS
-- ============================================

CREATE TABLE mcp_server_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id UUID NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    version VARCHAR(50) NOT NULL,
    commit_hash VARCHAR(64),
    
    -- Snapshot
    tool_definitions JSONB NOT NULL,
    tool_count INTEGER NOT NULL,
    
    -- Changes from previous version
    changes JSONB DEFAULT '[]',
    changes_summary TEXT,
    has_breaking_changes BOOLEAN DEFAULT FALSE,
    
    -- Audit
    created_at TIMESTAMPTZ DEFAULT NOW(),
    created_by UUID,
    
    CONSTRAINT unique_version UNIQUE (server_id, version)
);

CREATE INDEX idx_mcp_versions_server ON mcp_server_versions(server_id);
CREATE INDEX idx_mcp_versions_created ON mcp_server_versions(created_at DESC);

-- ============================================
-- MCP TOOLS
-- ============================================

CREATE TABLE mcp_tools (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    server_id UUID NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    
    -- Identity
    name VARCHAR(255) NOT NULL,
    description TEXT,
    category VARCHAR(100),
    
    -- Schema
    input_schema JSONB NOT NULL,
    output_schema JSONB,
    
    -- Examples (per Anthropic's Tool Use Examples pattern)
    input_examples JSONB DEFAULT '[]',
    
    -- Embeddings for semantic search (1536 dimensions for OpenAI)
    name_embedding vector(1536),
    description_embedding vector(1536),
    combined_embedding vector(1536),
    
    -- Deferred loading flag (per Anthropic's pattern)
    defer_loading BOOLEAN DEFAULT TRUE,
    
    -- Status
    is_deprecated BOOLEAN DEFAULT FALSE,
    deprecation_message TEXT,
    deprecated_at TIMESTAMPTZ,
    
    -- Metadata
    version VARCHAR(50),
    execution_count BIGINT DEFAULT 0,
    last_executed_at TIMESTAMPTZ,
    avg_execution_time_ms INTEGER,
    
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    
    CONSTRAINT unique_tool UNIQUE (server_id, name)
);

-- Vector indexes for similarity search
CREATE INDEX idx_mcp_tools_name_emb ON mcp_tools 
    USING ivfflat (name_embedding vector_cosine_ops) WITH (lists = 100);
CREATE INDEX idx_mcp_tools_desc_emb ON mcp_tools 
    USING ivfflat (description_embedding vector_cosine_ops) WITH (lists = 100);
CREATE INDEX idx_mcp_tools_combined_emb ON mcp_tools 
    USING ivfflat (combined_embedding vector_cosine_ops) WITH (lists = 100);

-- Full-text search index
CREATE INDEX idx_mcp_tools_fts ON mcp_tools 
    USING gin(to_tsvector('english', name || ' ' || COALESCE(description, '')));

-- Other indexes
CREATE INDEX idx_mcp_tools_tenant ON mcp_tools(tenant_id);
CREATE INDEX idx_mcp_tools_server ON mcp_tools(server_id);
CREATE INDEX idx_mcp_tools_category ON mcp_tools(category);

-- ============================================
-- MCP TOOL PERMISSIONS
-- ============================================

CREATE TABLE mcp_tool_permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    role_id UUID NOT NULL,
    
    -- Permission target (either server-level or tool-level)
    server_id UUID REFERENCES mcp_servers(id) ON DELETE CASCADE,
    tool_id UUID REFERENCES mcp_tools(id) ON DELETE CASCADE,
    
    -- Permission
    status VARCHAR(20) NOT NULL 
        CHECK (status IN ('PENDING', 'ALLOWED', 'DENIED', 'REMOVED')),
    
    -- Audit
    decided_by UUID,
    decided_by_email VARCHAR(255),
    decided_at TIMESTAMPTZ,
    decision_reason TEXT,
    
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    
    -- Either tool_id or server_id must be set
    CONSTRAINT check_permission_target CHECK (
        (tool_id IS NOT NULL AND server_id IS NULL) OR
        (tool_id IS NULL AND server_id IS NOT NULL)
    ),
    
    -- Unique constraint per role
    CONSTRAINT unique_tool_permission UNIQUE (role_id, tool_id),
    CONSTRAINT unique_server_permission UNIQUE (role_id, server_id)
);

CREATE INDEX idx_mcp_perms_role ON mcp_tool_permissions(role_id);
CREATE INDEX idx_mcp_perms_tool ON mcp_tool_permissions(tool_id);
CREATE INDEX idx_mcp_perms_server ON mcp_tool_permissions(server_id);

-- ============================================
-- MCP TOOL EXECUTION LOGS
-- ============================================

CREATE TABLE mcp_tool_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    server_id UUID NOT NULL,
    tool_id UUID NOT NULL,
    
    -- Request context
    role_id UUID,
    api_key_id UUID,
    request_id VARCHAR(255),
    
    -- Execution details
    input_params JSONB,
    output_result JSONB,
    status VARCHAR(50) NOT NULL CHECK (status IN ('SUCCESS', 'ERROR', 'BLOCKED', 'TIMEOUT')),
    error_message TEXT,
    
    -- Timing
    started_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    duration_ms INTEGER,
    
    -- Audit
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_mcp_exec_tenant ON mcp_tool_executions(tenant_id);
CREATE INDEX idx_mcp_exec_tool ON mcp_tool_executions(tool_id);
CREATE INDEX idx_mcp_exec_created ON mcp_tool_executions(created_at DESC);

-- ============================================
-- MCP SEARCH CACHE
-- ============================================

CREATE TABLE mcp_search_cache (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    query_hash VARCHAR(64) NOT NULL,  -- SHA256 of query
    query_text TEXT NOT NULL,
    
    -- Results
    results JSONB NOT NULL,
    result_count INTEGER NOT NULL,
    
    -- Cache control
    expires_at TIMESTAMPTZ NOT NULL,
    hit_count INTEGER DEFAULT 0,
    
    created_at TIMESTAMPTZ DEFAULT NOW(),
    
    CONSTRAINT unique_query UNIQUE (tenant_id, query_hash)
);

CREATE INDEX idx_mcp_search_expires ON mcp_search_cache(expires_at);
```

---

## 8. GraphQL API Design

```graphql
# schema/mcp.graphql

# ============================================
# TYPES
# ============================================

type MCPServer {
  id: ID!
  name: String!
  description: String
  serverType: MCPServerType!
  endpoint: String!
  authType: MCPAuthType!
  version: String
  status: MCPServerStatus!
  lastHealthCheck: DateTime
  toolCount: Int!
  tools: [MCPTool!]!
  versions: [MCPServerVersion!]!
  createdAt: DateTime!
  updatedAt: DateTime!
}

type MCPTool {
  id: ID!
  serverId: ID!
  serverName: String!
  name: String!
  description: String
  category: String
  inputSchema: JSON!
  inputExamples: [JSON!]
  isDeprecated: Boolean!
  deprecationMessage: String
  permission(roleId: ID!): MCPToolPermission
  executionCount: Int!
  avgExecutionTimeMs: Int
}

type MCPToolPermission {
  status: ToolPermissionStatus!
  decidedBy: String
  decidedAt: DateTime
  reason: String
}

type MCPServerVersion {
  id: ID!
  version: String!
  commitHash: String
  toolCount: Int!
  changes: [MCPSchemaChange!]!
  changesSummary: String
  hasBreakingChanges: Boolean!
  createdAt: DateTime!
}

type MCPSchemaChange {
  type: ChangeType!
  toolName: String!
  field: String
  oldValue: JSON
  newValue: JSON
  breaking: Boolean!
}

type ToolSearchResult {
  tool: MCPTool!
  serverName: String!
  score: Float!
  matchReason: String
  deferLoading: Boolean!
  toolRef: String!
}

type ToolSearchResponse {
  tools: [ToolSearchResult!]!
  query: String!
  totalAvailable: Int!
  totalAllowed: Int!
}

# ============================================
# ENUMS
# ============================================

enum MCPServerType {
  STDIO
  SSE
  WEBSOCKET
}

enum MCPAuthType {
  NONE
  API_KEY
  OAUTH2
  BASIC
  MTLS
  AWS_IAM
}

enum MCPServerStatus {
  PENDING
  CONNECTED
  DISCONNECTED
  ERROR
  DISABLED
}

enum ChangeType {
  ADDED
  REMOVED
  MODIFIED
}

enum SearchStrategy {
  REGEX
  BM25
  SEMANTIC
  HYBRID
}

# ============================================
# INPUTS
# ============================================

input CreateMCPServerInput {
  name: String!
  description: String
  serverType: MCPServerType!
  endpoint: String!
  arguments: [String!]
  environment: JSON
  authType: MCPAuthType!
  authConfig: MCPAuthConfigInput
  tags: [String!]
}

input MCPAuthConfigInput {
  apiKey: String
  apiKeyHeader: String
  clientId: String
  clientSecret: String
  tokenUrl: String
  scopes: [String!]
  username: String
  password: String
  clientCert: String
  clientKey: String
  caCert: String
  awsRegion: String
  awsRoleArn: String
}

input UpdateMCPServerInput {
  name: String
  description: String
  endpoint: String
  authType: MCPAuthType
  authConfig: MCPAuthConfigInput
  tags: [String!]
}

input ToolSearchInput {
  query: String!
  strategy: SearchStrategy
  serverIds: [ID!]
  categories: [String!]
  maxResults: Int
  minScore: Float
  includeSchema: Boolean
}

input SetMCPPermissionInput {
  roleId: ID!
  serverId: ID
  toolId: ID
  status: ToolPermissionStatus!
  reason: String
}

# ============================================
# QUERIES
# ============================================

extend type Query {
  # MCP Servers
  mcpServers: [MCPServer!]!
  mcpServer(id: ID!): MCPServer
  
  # MCP Tools
  mcpTools(serverId: ID, category: String): [MCPTool!]!
  mcpTool(id: ID!): MCPTool
  
  # Tool Search (main interface)
  searchTools(input: ToolSearchInput!): ToolSearchResponse!
  
  # Version History
  mcpServerVersions(serverId: ID!): [MCPServerVersion!]!
  
  # Permissions
  mcpPermissions(roleId: ID!): [MCPToolPermission!]!
}

# ============================================
# MUTATIONS
# ============================================

extend type Mutation {
  # MCP Server Management
  createMCPServer(input: CreateMCPServerInput!): MCPServer!
  updateMCPServer(id: ID!, input: UpdateMCPServerInput!): MCPServer!
  deleteMCPServer(id: ID!): Boolean!
  
  # Server Actions
  connectMCPServer(id: ID!): MCPServer!
  disconnectMCPServer(id: ID!): MCPServer!
  syncMCPServer(id: ID!): MCPServerVersion!
  
  # Version Control
  rollbackMCPServer(serverId: ID!, versionId: ID!): MCPServer!
  
  # Permissions
  setMCPPermission(input: SetMCPPermissionInput!): MCPToolPermission!
  bulkSetMCPPermissions(roleId: ID!, serverIds: [ID!], status: ToolPermissionStatus!): Int!
  
  # Tool Examples (per Anthropic's pattern)
  addToolExample(toolId: ID!, example: JSON!): MCPTool!
  removeToolExample(toolId: ID!, exampleIndex: Int!): MCPTool!
}

# ============================================
# SUBSCRIPTIONS
# ============================================

extend type Subscription {
  mcpServerStatus(id: ID!): MCPServer!
  mcpToolExecution(serverId: ID): MCPToolExecution!
}
```

---

## 9. Implementation Plan

### Phase 1: Foundation (Week 1-2)

| Task | Priority | Effort |
|------|----------|--------|
| Database schema migration | High | 1 day |
| MCP server domain types | High | 1 day |
| Basic CRUD for MCP servers | High | 2 days |
| Connection manager (stdio, SSE) | High | 3 days |
| GraphQL resolvers | High | 2 days |
| Basic UI for server management | Medium | 2 days |

### Phase 2: Tool Discovery (Week 3-4)

| Task | Priority | Effort |
|------|----------|--------|
| pgvector setup and testing | High | 1 day |
| Embedding service integration | High | 2 days |
| Tool indexing pipeline | High | 2 days |
| Regex search implementation | Medium | 1 day |
| BM25 search implementation | Medium | 1 day |
| Semantic search + LLM reranking | High | 3 days |
| Tool Search Tool API | High | 1 day |

### Phase 3: Policy Integration (Week 5)

| Task | Priority | Effort |
|------|----------|--------|
| MCP policy schema extension | High | 1 day |
| Permission enforcement | High | 2 days |
| Policy UI (MCP tab) | High | 2 days |

### Phase 4: Version Control (Week 6)

| Task | Priority | Effort |
|------|----------|--------|
| Version snapshot creation | Medium | 2 days |
| Schema diffing | Medium | 2 days |
| Rollback functionality | Medium | 1 day |
| Version history UI | Medium | 2 days |

### Phase 5: Production Hardening (Week 7-8)

| Task | Priority | Effort |
|------|----------|--------|
| Health checks and monitoring | High | 2 days |
| Connection pooling | High | 1 day |
| Rate limiting per server | Medium | 1 day |
| Audit logging | High | 1 day |
| Error handling and retries | High | 2 days |
| Documentation | Medium | 2 days |
| Integration tests | High | 3 days |

---

## 10. Security Considerations

### 10.1 Authentication Credential Storage

```go
// Encrypt auth configs before storage
func (s *TenantStore) CreateMCPServer(ctx context.Context, server *domain.MCPServer) error {
    // Encrypt auth config
    encryptedConfig, err := s.encryptor.Encrypt(server.AuthConfig)
    if err != nil {
        return err
    }
    
    // Store with encrypted config
    query := `
        INSERT INTO mcp_servers (id, tenant_id, name, auth_config_encrypted, ...)
        VALUES ($1, $2, $3, $4, ...)
    `
    _, err = s.db.ExecContext(ctx, query, server.ID, server.TenantID, server.Name, encryptedConfig)
    return err
}
```

### 10.2 Network Security

- All MCP connections use TLS 1.3
- mTLS support for high-security environments
- Rate limiting per MCP server to prevent abuse
- Timeout enforcement on all MCP calls

### 10.3 Audit Trail

All MCP operations are logged:
- Server connections/disconnections
- Tool discoveries
- Permission changes
- Tool executions (with optional input/output logging)

---

## References

1. [Anthropic Advanced Tool Use](https://www.anthropic.com/engineering/advanced-tool-use) - Tool Search Tool, Programmatic Tool Calling, Tool Use Examples
2. [Model Context Protocol Specification](https://spec.modelcontextprotocol.io/)
3. [pgvector Documentation](https://github.com/pgvector/pgvector)
4. [OpenAI Embeddings API](https://platform.openai.com/docs/guides/embeddings)

---

## Appendix: Token Savings Analysis

Based on Anthropic's findings, here's the expected token savings with Tool Search Tool:

| Scenario | Without Tool Search | With Tool Search | Savings |
|----------|-------------------|------------------|---------|
| 5 MCP servers, 58 tools | ~55K tokens | ~4K tokens | **93%** |
| 10 MCP servers, 100+ tools | ~100K tokens | ~5K tokens | **95%** |
| Enterprise setup, 200+ tools | ~200K tokens | ~8K tokens | **96%** |

This enables:
- Faster response times (less context to process)
- Lower costs (fewer tokens = lower API costs)
- Better accuracy (less confusion from irrelevant tools)
- Unlimited tool scaling (add more servers without context bloat)

