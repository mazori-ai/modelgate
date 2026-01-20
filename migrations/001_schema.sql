-- ModelGate Open Source Edition - Single Tenant Database Schema
-- This schema supports a single installation without multi-tenancy

-- =============================================================================
-- Enable pgvector extension for semantic search (requires pgvector to be installed)
-- =============================================================================
CREATE EXTENSION IF NOT EXISTS vector;

-- =============================================================================
-- Users Table
-- Users who can access the admin portal
-- =============================================================================
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    name VARCHAR(255),
    role VARCHAR(50) NOT NULL DEFAULT 'admin',  -- admin, member, viewer
    is_active BOOLEAN DEFAULT TRUE,
    last_login_at TIMESTAMP WITH TIME ZONE,
    metadata JSONB DEFAULT '{}',
    created_by UUID,
    created_by_email VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- =============================================================================
-- Sessions Table
-- =============================================================================
CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) UNIQUE NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_sessions_token ON sessions(token_hash);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);

-- =============================================================================
-- Roles Table
-- Roles that can be assigned to API keys
-- =============================================================================
CREATE TABLE IF NOT EXISTS roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) UNIQUE NOT NULL,
    description TEXT,
    permissions JSONB DEFAULT '[]',
    is_default BOOLEAN DEFAULT FALSE,
    is_system BOOLEAN DEFAULT FALSE,
    created_by UUID,
    created_by_email VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_roles_name ON roles(name);
CREATE INDEX IF NOT EXISTS idx_roles_default ON roles(is_default) WHERE is_default = TRUE;

-- =============================================================================
-- Role Policies Table
-- Policy configuration for each role
-- =============================================================================
CREATE TABLE IF NOT EXISTS role_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    prompt_policies JSONB DEFAULT '{}',
    tool_policies JSONB DEFAULT '{}',
    rate_limit_policy JSONB DEFAULT '{}',
    model_restrictions JSONB DEFAULT '{}',
    caching_policy JSONB DEFAULT '{}',
    routing_policy JSONB DEFAULT '{}',
    resilience_policy JSONB DEFAULT '{}',
    budget_policy JSONB DEFAULT '{}',
    mcp_policies JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(role_id)
);

CREATE INDEX IF NOT EXISTS idx_role_policies_role ON role_policies(role_id);

-- =============================================================================
-- Groups Table
-- Groups for organizing API keys
-- =============================================================================
CREATE TABLE IF NOT EXISTS groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) UNIQUE NOT NULL,
    description TEXT,
    metadata JSONB DEFAULT '{}',
    created_by UUID,
    created_by_email VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_groups_name ON groups(name);

-- =============================================================================
-- Group Roles Table
-- Many-to-many relationship between groups and roles
-- =============================================================================
CREATE TABLE IF NOT EXISTS group_roles (
    group_id UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    PRIMARY KEY (group_id, role_id)
);

-- =============================================================================
-- API Keys Table
-- =============================================================================
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    key_prefix VARCHAR(20) NOT NULL,
    key_hash VARCHAR(255) UNIQUE NOT NULL,
    role_id UUID REFERENCES roles(id) ON DELETE SET NULL,
    group_id UUID REFERENCES groups(id) ON DELETE SET NULL,
    scopes JSONB DEFAULT '[]',
    expires_at TIMESTAMP WITH TIME ZONE,
    last_used_at TIMESTAMP WITH TIME ZONE,
    is_revoked BOOLEAN DEFAULT FALSE,
    revoked_at TIMESTAMP WITH TIME ZONE,
    revoked_reason TEXT,
    metadata JSONB DEFAULT '{}',
    created_by UUID,
    created_by_email VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_api_keys_key_hash ON api_keys(key_hash);
CREATE INDEX IF NOT EXISTS idx_api_keys_role ON api_keys(role_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_group ON api_keys(group_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_revoked ON api_keys(is_revoked);

-- =============================================================================
-- Provider Configurations Table
-- LLM provider credentials
-- =============================================================================
CREATE TABLE IF NOT EXISTS provider_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider VARCHAR(50) UNIQUE NOT NULL,
    is_enabled BOOLEAN DEFAULT FALSE,
    base_url VARCHAR(500),
    region VARCHAR(100),
    models_url VARCHAR(500),
    extra_settings JSONB DEFAULT '{}',
    connection_settings JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_provider_configs_provider ON provider_configs(provider);

-- =============================================================================
-- Provider API Keys Table (Multi-key support with health tracking)
-- Supports multiple API keys per provider for load balancing and failover
-- =============================================================================
CREATE TABLE IF NOT EXISTS provider_api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider VARCHAR(50) NOT NULL,
    name VARCHAR(100) NOT NULL,
    
    -- Credentials (API key and/or IAM credentials for AWS Bedrock)
    api_key_encrypted TEXT,                              -- Encrypted API key (NULL if using IAM only)
    access_key_id_encrypted VARCHAR(255),                -- Encrypted AWS Access Key ID (for Bedrock)
    secret_access_key_encrypted TEXT,                    -- Encrypted AWS Secret Access Key (for Bedrock)
    credential_type VARCHAR(20) DEFAULT 'api_key',       -- 'api_key', 'iam_credentials', or 'both'
    
    -- Selection and health
    priority INTEGER DEFAULT 1,                          -- Lower = higher priority (1 is highest)
    enabled BOOLEAN DEFAULT TRUE,                        -- Is this key active?
    health_score DECIMAL(3,2) DEFAULT 1.0,              -- 0.0 to 1.0 (1.0 = healthy)
    
    -- Usage statistics
    success_count INTEGER DEFAULT 0,
    failure_count INTEGER DEFAULT 0,
    request_count BIGINT DEFAULT 0,
    last_used_at TIMESTAMP WITH TIME ZONE,
    
    -- Rate limiting tracking
    rate_limit_remaining INTEGER,
    rate_limit_reset_at TIMESTAMP WITH TIME ZONE,
    
    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    UNIQUE(provider, name)
);

CREATE INDEX IF NOT EXISTS idx_provider_api_keys_provider ON provider_api_keys(provider);
CREATE INDEX IF NOT EXISTS idx_provider_api_keys_enabled ON provider_api_keys(enabled);
CREATE INDEX IF NOT EXISTS idx_provider_api_keys_priority ON provider_api_keys(priority);

-- =============================================================================
-- Available Models Table
-- Models fetched from provider APIs
-- =============================================================================
CREATE TABLE IF NOT EXISTS available_models (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider VARCHAR(50) NOT NULL,
    model_id VARCHAR(255) NOT NULL,
    model_name VARCHAR(255) NOT NULL,
    native_model_id VARCHAR(500),
    description TEXT,
    supports_tools BOOLEAN DEFAULT FALSE,
    supports_vision BOOLEAN DEFAULT FALSE,
    supports_reasoning BOOLEAN DEFAULT FALSE,
    supports_streaming BOOLEAN DEFAULT TRUE,
    context_window INTEGER DEFAULT 0,
    max_output_tokens INTEGER DEFAULT 0,
    input_cost_per_1m DECIMAL(10, 6) DEFAULT 0,
    output_cost_per_1m DECIMAL(10, 6) DEFAULT 0,
    provider_metadata JSONB DEFAULT '{}',
    is_available BOOLEAN DEFAULT TRUE,
    is_deprecated BOOLEAN DEFAULT FALSE,
    fetched_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(provider, model_id)
);

CREATE INDEX IF NOT EXISTS idx_available_models_provider ON available_models(provider);
CREATE INDEX IF NOT EXISTS idx_available_models_model_id ON available_models(model_id);

-- =============================================================================
-- Model Configurations Table
-- Model enablement and overrides
-- =============================================================================
CREATE TABLE IF NOT EXISTS model_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_id VARCHAR(255) NOT NULL,
    is_enabled BOOLEAN DEFAULT TRUE,
    alias VARCHAR(100),
    max_tokens_override INTEGER,
    cost_multiplier DECIMAL(5, 2) DEFAULT 1.0,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(model_id)
);

CREATE INDEX IF NOT EXISTS idx_model_configs_model ON model_configs(model_id);

-- =============================================================================
-- Available Tools Table
-- Tools that can be enabled/disabled per role
-- =============================================================================
CREATE TABLE IF NOT EXISTS available_tools (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) UNIQUE NOT NULL,
    description TEXT,
    category VARCHAR(100),
    schema JSONB NOT NULL,
    is_builtin BOOLEAN DEFAULT FALSE,
    is_enabled BOOLEAN DEFAULT TRUE,
    status VARCHAR(50) DEFAULT 'active',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_available_tools_name ON available_tools(name);
CREATE INDEX IF NOT EXISTS idx_available_tools_category ON available_tools(category);

-- =============================================================================
-- MCP Servers Table
-- MCP server configurations (single-tenant)
-- =============================================================================
CREATE TABLE IF NOT EXISTS mcp_servers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) UNIQUE NOT NULL,
    slug VARCHAR(255) UNIQUE NOT NULL,
    description TEXT,
    server_type VARCHAR(50) NOT NULL DEFAULT 'SSE',  -- SSE, STDIO, WebSocket
    endpoint VARCHAR(500) NOT NULL,
    arguments JSONB DEFAULT '[]',
    environment JSONB DEFAULT '{}',
    auth_type VARCHAR(50) DEFAULT 'NONE',
    auth_config_encrypted JSONB DEFAULT '{}',
    version VARCHAR(50),
    commit_hash VARCHAR(64),
    last_sync_at TIMESTAMP WITH TIME ZONE,
    status VARCHAR(50) DEFAULT 'PENDING',
    last_health_check TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    retry_count INTEGER DEFAULT 0,
    auto_sync BOOLEAN DEFAULT FALSE,
    sync_interval_minutes INTEGER DEFAULT 60,
    health_check_interval_seconds INTEGER DEFAULT 300,
    tags TEXT[] DEFAULT '{}',
    metadata JSONB DEFAULT '{}',
    tool_count INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_by UUID
);

CREATE INDEX IF NOT EXISTS idx_mcp_servers_name ON mcp_servers(name);
CREATE INDEX IF NOT EXISTS idx_mcp_servers_slug ON mcp_servers(slug);
CREATE INDEX IF NOT EXISTS idx_mcp_servers_status ON mcp_servers(status);

-- =============================================================================
-- MCP Tools Table
-- Tools discovered from MCP servers (single-tenant)
-- =============================================================================
CREATE TABLE IF NOT EXISTS mcp_tools (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id UUID NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    category VARCHAR(100),
    input_schema JSONB DEFAULT '{}',
    output_schema JSONB DEFAULT '{}',
    input_examples JSONB DEFAULT '[]',
    defer_loading BOOLEAN DEFAULT FALSE,
    is_deprecated BOOLEAN DEFAULT FALSE,
    deprecation_message TEXT,
    deprecated_at TIMESTAMP WITH TIME ZONE,
    version VARCHAR(50),
    execution_count BIGINT DEFAULT 0,
    last_executed_at TIMESTAMP WITH TIME ZONE,
    avg_execution_time_ms INTEGER DEFAULT 0,
    name_embedding TEXT,
    description_embedding TEXT,
    combined_embedding TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(server_id, name)
);

CREATE INDEX IF NOT EXISTS idx_mcp_tools_server ON mcp_tools(server_id);
CREATE INDEX IF NOT EXISTS idx_mcp_tools_name ON mcp_tools(name);
CREATE INDEX IF NOT EXISTS idx_mcp_tools_category ON mcp_tools(category);

-- =============================================================================
-- MCP Server Versions Table
-- Version history for MCP servers
-- =============================================================================
CREATE TABLE IF NOT EXISTS mcp_server_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id UUID NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    version VARCHAR(50) NOT NULL,
    commit_hash VARCHAR(64),
    tool_definitions JSONB DEFAULT '[]',
    tool_count INTEGER DEFAULT 0,
    changes JSONB DEFAULT '[]',
    changes_summary TEXT,
    has_breaking_changes BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_by UUID,
    UNIQUE(server_id, version)
);

CREATE INDEX IF NOT EXISTS idx_mcp_server_versions_server ON mcp_server_versions(server_id);

-- =============================================================================
-- MCP Tool Permissions Table
-- Role-based visibility for MCP tools
-- =============================================================================
CREATE TABLE IF NOT EXISTS mcp_tool_permissions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    server_id UUID NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    tool_id UUID NOT NULL REFERENCES mcp_tools(id) ON DELETE CASCADE,
    visibility VARCHAR(20) NOT NULL DEFAULT 'DENY',  -- ALLOW, DENY, SEARCH
    decided_by UUID,
    decided_by_email VARCHAR(255),
    decided_at TIMESTAMP WITH TIME ZONE,
    decision_reason TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(role_id, tool_id)
);

CREATE INDEX IF NOT EXISTS idx_mcp_tool_permissions_role ON mcp_tool_permissions(role_id);
CREATE INDEX IF NOT EXISTS idx_mcp_tool_permissions_tool ON mcp_tool_permissions(tool_id);

-- =============================================================================
-- MCP Tool Executions Table
-- Execution logs for MCP tools
-- =============================================================================
CREATE TABLE IF NOT EXISTS mcp_tool_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id UUID NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    tool_id UUID NOT NULL REFERENCES mcp_tools(id) ON DELETE CASCADE,
    role_id UUID,
    api_key_id UUID,
    request_id VARCHAR(100),
    input_params JSONB DEFAULT '{}',
    output_result JSONB DEFAULT '{}',
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    error_message TEXT,
    started_at TIMESTAMP WITH TIME ZONE NOT NULL,
    completed_at TIMESTAMP WITH TIME ZONE,
    duration_ms INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_mcp_tool_executions_server ON mcp_tool_executions(server_id);
CREATE INDEX IF NOT EXISTS idx_mcp_tool_executions_tool ON mcp_tool_executions(tool_id);
CREATE INDEX IF NOT EXISTS idx_mcp_tool_executions_created ON mcp_tool_executions(created_at);

-- =============================================================================
-- Usage Records Table
-- Detailed usage tracking
-- =============================================================================
CREATE TABLE IF NOT EXISTS usage_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    api_key_id UUID REFERENCES api_keys(id) ON DELETE SET NULL,
    request_id VARCHAR(100),
    model VARCHAR(255) NOT NULL,
    provider VARCHAR(50) NOT NULL,
    input_tokens BIGINT DEFAULT 0,
    output_tokens BIGINT DEFAULT 0,
    total_tokens BIGINT DEFAULT 0,
    cost_usd DECIMAL(10, 6) DEFAULT 0,
    latency_ms INTEGER DEFAULT 0,
    is_success BOOLEAN DEFAULT TRUE,
    error_code VARCHAR(100),
    error_message TEXT,
    tool_calls INTEGER DEFAULT 0,
    thinking_tokens BIGINT DEFAULT 0,
    is_cached BOOLEAN DEFAULT FALSE,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_usage_records_api_key ON usage_records(api_key_id);
CREATE INDEX IF NOT EXISTS idx_usage_records_model ON usage_records(model);
CREATE INDEX IF NOT EXISTS idx_usage_records_created ON usage_records(created_at);

-- =============================================================================
-- Semantic Cache Table
-- =============================================================================
CREATE TABLE IF NOT EXISTS semantic_cache (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    role_id UUID REFERENCES roles(id) ON DELETE CASCADE,
    model VARCHAR(255) NOT NULL,
    request_hash VARCHAR(64) NOT NULL,
    request_content JSONB NOT NULL,
    response_content JSONB NOT NULL,
    embedding vector(768),  -- For semantic similarity search (nomic-embed-text)
    input_tokens INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    cost_usd DECIMAL(10, 6) DEFAULT 0,
    latency_ms INTEGER DEFAULT 0,
    provider VARCHAR(50),
    hit_count INTEGER DEFAULT 0,
    last_hit_at TIMESTAMP WITH TIME ZONE,
    expires_at TIMESTAMP WITH TIME ZONE,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(request_hash, model)
);

CREATE INDEX IF NOT EXISTS idx_semantic_cache_hash ON semantic_cache(request_hash);
CREATE INDEX IF NOT EXISTS idx_semantic_cache_model ON semantic_cache(model);
CREATE INDEX IF NOT EXISTS idx_semantic_cache_expires ON semantic_cache(expires_at);
CREATE INDEX IF NOT EXISTS idx_semantic_cache_role ON semantic_cache(role_id);

-- Create HNSW index for vector similarity search (if pgvector available)
CREATE INDEX IF NOT EXISTS idx_semantic_cache_embedding ON semantic_cache USING hnsw (embedding vector_cosine_ops);

-- =============================================================================
-- Cache Statistics Table (aggregated cache metrics)
-- =============================================================================
CREATE TABLE IF NOT EXISTS cache_stats (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    date DATE NOT NULL DEFAULT CURRENT_DATE,
    model VARCHAR(255),
    cache_hits INTEGER DEFAULT 0,
    cache_misses INTEGER DEFAULT 0,
    tokens_saved BIGINT DEFAULT 0,
    cost_saved_usd DECIMAL(10, 6) DEFAULT 0,
    latency_saved_ms BIGINT DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(date, model)
);

CREATE INDEX IF NOT EXISTS idx_cache_stats_date ON cache_stats(date);
CREATE INDEX IF NOT EXISTS idx_cache_stats_model ON cache_stats(model);

-- =============================================================================
-- Provider Health Table
-- =============================================================================
CREATE TABLE IF NOT EXISTS provider_health (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider VARCHAR(50) NOT NULL,
    model VARCHAR(255) DEFAULT '',
    status VARCHAR(50) DEFAULT 'unknown',
    latency_p50_ms INTEGER,
    latency_p95_ms INTEGER,
    latency_p99_ms INTEGER,
    success_rate DECIMAL(5, 4),
    error_rate DECIMAL(5, 4),
    last_error TEXT,
    last_check_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(provider, model)
);

CREATE INDEX IF NOT EXISTS idx_provider_health_provider ON provider_health(provider);

-- =============================================================================
-- Circuit Breaker State Table
-- =============================================================================
CREATE TABLE IF NOT EXISTS circuit_breaker_state (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider VARCHAR(50) NOT NULL,
    model VARCHAR(255) DEFAULT '',
    state VARCHAR(50) DEFAULT 'closed',
    failure_count INTEGER DEFAULT 0,
    success_count INTEGER DEFAULT 0,
    last_failure_at TIMESTAMP WITH TIME ZONE,
    last_success_at TIMESTAMP WITH TIME ZONE,
    opened_at TIMESTAMP WITH TIME ZONE,
    half_open_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(provider, model)
);

CREATE INDEX IF NOT EXISTS idx_circuit_breaker_provider ON circuit_breaker_state(provider);

-- =============================================================================
-- Telemetry Configuration Table
-- =============================================================================
CREATE TABLE IF NOT EXISTS telemetry_config (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    prometheus_enabled BOOLEAN DEFAULT FALSE,
    prometheus_endpoint VARCHAR(500),
    otlp_enabled BOOLEAN DEFAULT FALSE,
    otlp_endpoint VARCHAR(500),
    log_level VARCHAR(20) DEFAULT 'info',
    export_usage_data BOOLEAN DEFAULT FALSE,
    webhook_url VARCHAR(500),
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- =============================================================================
-- Audit Logs Table
-- =============================================================================
CREATE TABLE IF NOT EXISTS audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    action VARCHAR(50) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id VARCHAR(255) NOT NULL,
    resource_name VARCHAR(255),
    actor_id VARCHAR(255),
    actor_email VARCHAR(255),
    actor_type VARCHAR(50) DEFAULT 'user',
    ip_address VARCHAR(45),
    user_agent TEXT,
    details JSONB DEFAULT '{}',
    old_value JSONB,
    new_value JSONB,
    status VARCHAR(20) DEFAULT 'success',
    error_message TEXT
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_timestamp ON audit_logs(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_resource ON audit_logs(resource_type, resource_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_actor ON audit_logs(actor_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action);

-- =============================================================================
-- Agent Sessions Table (for Agent Dashboard)
-- =============================================================================
CREATE TABLE IF NOT EXISTS agent_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    api_key_id UUID REFERENCES api_keys(id) ON DELETE SET NULL,
    agent_name VARCHAR(255),
    started_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    ended_at TIMESTAMP WITH TIME ZONE,
    status VARCHAR(50) DEFAULT 'active',
    total_requests INTEGER DEFAULT 0,
    total_tokens BIGINT DEFAULT 0,
    total_cost_usd DECIMAL(10, 6) DEFAULT 0,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_sessions_api_key ON agent_sessions(api_key_id);
CREATE INDEX IF NOT EXISTS idx_agent_sessions_status ON agent_sessions(status);

-- =============================================================================
-- Policy Violations Table (for Agent Dashboard)
-- =============================================================================
CREATE TABLE IF NOT EXISTS policy_violations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    api_key_id UUID REFERENCES api_keys(id) ON DELETE SET NULL,
    session_id UUID REFERENCES agent_sessions(id) ON DELETE SET NULL,
    violation_type VARCHAR(100) NOT NULL,
    severity VARCHAR(50) NOT NULL,
    policy_id VARCHAR(255),
    policy_name VARCHAR(255),
    message TEXT,
    request_content TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_policy_violations_api_key ON policy_violations(api_key_id);
CREATE INDEX IF NOT EXISTS idx_policy_violations_session ON policy_violations(session_id);
CREATE INDEX IF NOT EXISTS idx_policy_violations_type ON policy_violations(violation_type);
CREATE INDEX IF NOT EXISTS idx_policy_violations_created ON policy_violations(created_at);

-- =============================================================================
-- Policy Violation Events Table (for Agent Dashboard analytics)
-- =============================================================================
CREATE TABLE IF NOT EXISTS policy_violation_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    api_key_id UUID REFERENCES api_keys(id) ON DELETE SET NULL,
    policy_id VARCHAR(255),
    policy_name VARCHAR(255),
    violation_type VARCHAR(100) NOT NULL,
    severity INTEGER DEFAULT 1,
    message TEXT,
    metadata JSONB DEFAULT '{}',
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_policy_violation_events_api_key ON policy_violation_events(api_key_id);
CREATE INDEX IF NOT EXISTS idx_policy_violation_events_type ON policy_violation_events(violation_type);
CREATE INDEX IF NOT EXISTS idx_policy_violation_events_timestamp ON policy_violation_events(timestamp);

-- =============================================================================
-- Tool Call Events Table (for Agent Dashboard analytics)
-- =============================================================================
CREATE TABLE IF NOT EXISTS tool_call_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    api_key_id UUID REFERENCES api_keys(id) ON DELETE SET NULL,
    tool_name VARCHAR(255) NOT NULL,
    model VARCHAR(255),
    provider VARCHAR(50),
    success BOOLEAN DEFAULT TRUE,
    error_message TEXT,
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tool_call_events_api_key ON tool_call_events(api_key_id);
CREATE INDEX IF NOT EXISTS idx_tool_call_events_tool ON tool_call_events(tool_name);
CREATE INDEX IF NOT EXISTS idx_tool_call_events_timestamp ON tool_call_events(timestamp);

-- =============================================================================
-- Cache Events Table (for Agent Dashboard analytics)
-- =============================================================================
CREATE TABLE IF NOT EXISTS cache_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    api_key_id UUID REFERENCES api_keys(id) ON DELETE SET NULL,
    model VARCHAR(255),
    hit BOOLEAN NOT NULL,
    tokens_saved BIGINT DEFAULT 0,
    cost_saved_usd DECIMAL(10, 6) DEFAULT 0,
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cache_events_api_key ON cache_events(api_key_id);
CREATE INDEX IF NOT EXISTS idx_cache_events_timestamp ON cache_events(timestamp);

-- =============================================================================
-- Role Tools Table
-- Tools discovered from AI model requests, scoped to roles (single-tenant)
-- Each role has its own set of discovered tools with inline permissions
-- =============================================================================
CREATE TABLE IF NOT EXISTS role_tools (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    
    -- Tool identity
    name VARCHAR(255) NOT NULL,
    description TEXT,
    schema_hash VARCHAR(64) NOT NULL,
    parameters JSONB DEFAULT '{}',
    category VARCHAR(100),
    
    -- Usage tracking
    first_seen_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    last_seen_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    first_seen_by UUID,  -- API Key ID that first used this tool
    seen_count INTEGER DEFAULT 1,
    
    -- Permission (inline, no separate table needed)
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',  -- ALLOWED, DENIED, PENDING, REMOVED
    decided_by UUID,
    decided_by_email VARCHAR(255),
    decided_at TIMESTAMP WITH TIME ZONE,
    decision_reason TEXT,
    
    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Each role has its own unique set of tools (same tool can exist in multiple roles)
    UNIQUE(role_id, name, schema_hash)
);

CREATE INDEX IF NOT EXISTS idx_role_tools_role ON role_tools(role_id);
CREATE INDEX IF NOT EXISTS idx_role_tools_name ON role_tools(name);
CREATE INDEX IF NOT EXISTS idx_role_tools_schema_hash ON role_tools(schema_hash);
CREATE INDEX IF NOT EXISTS idx_role_tools_category ON role_tools(category);
CREATE INDEX IF NOT EXISTS idx_role_tools_status ON role_tools(status);

-- =============================================================================
-- Tool Execution Logs Table
-- Logs of tool executions (single-tenant)
-- =============================================================================
CREATE TABLE IF NOT EXISTS tool_execution_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    role_tool_id UUID REFERENCES role_tools(id) ON DELETE SET NULL,  -- Can be null if tool was deleted
    tool_name VARCHAR(255) NOT NULL,  -- Store name for audit even if tool deleted
    role_id UUID,
    api_key_id UUID,
    request_id VARCHAR(100),
    input_params JSONB DEFAULT '{}',
    output_result JSONB DEFAULT '{}',
    status VARCHAR(50) NOT NULL DEFAULT 'success',
    error_message TEXT,
    duration_ms INTEGER DEFAULT 0,
    token_count INTEGER DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tool_execution_logs_role_tool ON tool_execution_logs(role_tool_id);
CREATE INDEX IF NOT EXISTS idx_tool_execution_logs_tool_name ON tool_execution_logs(tool_name);
CREATE INDEX IF NOT EXISTS idx_tool_execution_logs_api_key ON tool_execution_logs(api_key_id);
CREATE INDEX IF NOT EXISTS idx_tool_execution_logs_created ON tool_execution_logs(created_at);

-- =============================================================================
-- Functions
-- =============================================================================

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Triggers for updated_at
DROP TRIGGER IF EXISTS update_users_updated_at ON users;
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_roles_updated_at ON roles;
CREATE TRIGGER update_roles_updated_at BEFORE UPDATE ON roles FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_role_policies_updated_at ON role_policies;
CREATE TRIGGER update_role_policies_updated_at BEFORE UPDATE ON role_policies FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_groups_updated_at ON groups;
CREATE TRIGGER update_groups_updated_at BEFORE UPDATE ON groups FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_api_keys_updated_at ON api_keys;
CREATE TRIGGER update_api_keys_updated_at BEFORE UPDATE ON api_keys FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_provider_configs_updated_at ON provider_configs;
CREATE TRIGGER update_provider_configs_updated_at BEFORE UPDATE ON provider_configs FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_model_configs_updated_at ON model_configs;
CREATE TRIGGER update_model_configs_updated_at BEFORE UPDATE ON model_configs FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_available_tools_updated_at ON available_tools;
CREATE TRIGGER update_available_tools_updated_at BEFORE UPDATE ON available_tools FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_mcp_servers_updated_at ON mcp_servers;
CREATE TRIGGER update_mcp_servers_updated_at BEFORE UPDATE ON mcp_servers FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_mcp_tools_updated_at ON mcp_tools;
CREATE TRIGGER update_mcp_tools_updated_at BEFORE UPDATE ON mcp_tools FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_telemetry_config_updated_at ON telemetry_config;
CREATE TRIGGER update_telemetry_config_updated_at BEFORE UPDATE ON telemetry_config FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_provider_health_updated_at ON provider_health;
CREATE TRIGGER update_provider_health_updated_at BEFORE UPDATE ON provider_health FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_circuit_breaker_state_updated_at ON circuit_breaker_state;
CREATE TRIGGER update_circuit_breaker_state_updated_at BEFORE UPDATE ON circuit_breaker_state FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_agent_sessions_updated_at ON agent_sessions;
CREATE TRIGGER update_agent_sessions_updated_at BEFORE UPDATE ON agent_sessions FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- =============================================================================
-- Default Admin User
-- Password: admin123 (bcrypt hashed)
-- =============================================================================
INSERT INTO users (email, password_hash, name, role) 
VALUES (
    'admin@modelgate.local', 
    '$2a$10$ugQErLcXwctEDJm8fIMCzepikBc.3nWQYoU7FnP20.pVBVXl.1FT2',
    'Admin',
    'admin'
) ON CONFLICT (email) DO NOTHING;

-- =============================================================================
-- Enable pgvector extension (if available, for semantic search)
-- =============================================================================
-- Note: Run this manually if pgvector is installed:
-- CREATE EXTENSION IF NOT EXISTS vector;

