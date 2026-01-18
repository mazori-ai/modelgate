# Policy-Driven Feature Architecture

## Core Principle

> **Implementation lives in specialized layers, but control lives in Policy.**

All new features (caching, routing, resilience, etc.) should be **configurable per Role** through the existing policy framework. This provides:

1. **Single Source of Truth** - All configuration in one place
2. **Role-Based Control** - Different roles get different capabilities
3. **Tenant Isolation** - Each tenant configures their own policies
4. **Audit Trail** - Policy changes are tracked

---

## Extended Policy Schema

### Current Policy Structure (4 Types)

```go
type RolePolicy struct {
    RoleID           string
    PromptPolicies   PromptPolicies    // Prompt sanity, PII, injection
    ToolPolicies     ToolPolicies      // Tool allow/block
    RateLimitPolicy  RateLimitPolicy   // Rate limits
    ModelRestriction ModelRestrictions // Model whitelist/blacklist
}
```

### Proposed Policy Structure (8 Types)

```go
type RolePolicy struct {
    RoleID           string
    
    // Existing policies (unchanged)
    PromptPolicies   PromptPolicies    
    ToolPolicies     ToolPolicies      
    RateLimitPolicy  RateLimitPolicy   
    ModelRestriction ModelRestrictions 
    
    // NEW policy types
    CachingPolicy    CachingPolicy     // Semantic caching control
    RoutingPolicy    RoutingPolicy     // Intelligent routing control
    ResiliencePolicy ResiliencePolicy  // Failover, retry, circuit breaker
    BudgetPolicy     BudgetPolicy      // Budget limits and alerts
}
```

---

## New Policy Types

### 1. Caching Policy

Controls semantic caching behavior per role.

```go
// internal/domain/rbac.go

type CachingPolicy struct {
    // Master switch
    Enabled           bool    `json:"enabled"`
    
    // Cache behavior
    SimilarityThreshold float64 `json:"similarity_threshold"` // 0.0-1.0, default 0.95
    TTLSeconds         int     `json:"ttl_seconds"`          // Cache TTL, default 3600
    MaxCacheSize       int     `json:"max_cache_size"`       // Per-role cache limit
    
    // What to cache
    CacheStreaming     bool    `json:"cache_streaming"`      // Cache streaming responses?
    CacheToolCalls     bool    `json:"cache_tool_calls"`     // Cache tool call responses?
    
    // Exclusions
    ExcludedModels     []string `json:"excluded_models"`     // Don't cache these models
    ExcludedPatterns   []string `json:"excluded_patterns"`   // Don't cache prompts matching these
    
    // Cost tracking
    TrackSavings       bool    `json:"track_savings"`        // Track cost savings from cache
}
```

**GraphQL Schema:**
```graphql
input CachingPolicyInput {
    enabled: Boolean!
    similarityThreshold: Float
    ttlSeconds: Int
    maxCacheSize: Int
    cacheStreaming: Boolean
    cacheToolCalls: Boolean
    excludedModels: [String!]
    excludedPatterns: [String!]
    trackSavings: Boolean
}

type CachingPolicy {
    enabled: Boolean!
    similarityThreshold: Float!
    ttlSeconds: Int!
    maxCacheSize: Int!
    cacheStreaming: Boolean!
    cacheToolCalls: Boolean!
    excludedModels: [String!]!
    excludedPatterns: [String!]!
    trackSavings: Boolean!
}
```

**UI Configuration:**

```
┌─────────────────────────────────────────────────────────────────┐
│  🗄️ Caching Policy                                    [Enabled] │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Similarity Threshold    [========●==] 0.95                     │
│  Higher = more exact match required                              │
│                                                                  │
│  Cache TTL               [3600] seconds (1 hour)                │
│                                                                  │
│  ☑ Cache streaming responses                                    │
│  ☐ Cache tool call responses                                    │
│                                                                  │
│  Excluded Models:                                                │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │ gpt-4-turbo  ✕ │ claude-3-opus  ✕ │ + Add model            │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ☑ Track cost savings from cache hits                           │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

### 2. Routing Policy

Controls intelligent routing behavior per role.

```go
type RoutingPolicy struct {
    // Master switch
    Enabled          bool            `json:"enabled"`
    
    // Strategy selection
    Strategy         RoutingStrategy `json:"strategy"` // cost, latency, weighted, round_robin, capability
    
    // Cost-based routing config
    CostConfig       *CostRoutingConfig `json:"cost_config,omitempty"`
    
    // Latency-based routing config
    LatencyConfig    *LatencyRoutingConfig `json:"latency_config,omitempty"`
    
    // Weighted routing config
    WeightedConfig   *WeightedRoutingConfig `json:"weighted_config,omitempty"`
    
    // Capability-based routing config
    CapabilityConfig *CapabilityRoutingConfig `json:"capability_config,omitempty"`
    
    // Override: if model explicitly specified, skip routing
    AllowModelOverride bool `json:"allow_model_override"`
}

type RoutingStrategy string

const (
    RoutingStrategyCost       RoutingStrategy = "cost"
    RoutingStrategyLatency    RoutingStrategy = "latency"
    RoutingStrategyWeighted   RoutingStrategy = "weighted"
    RoutingStrategyRoundRobin RoutingStrategy = "round_robin"
    RoutingStrategyCapability RoutingStrategy = "capability"
)

type CostRoutingConfig struct {
    // Complexity thresholds
    SimpleQueryThreshold  float64  `json:"simple_query_threshold"`  // < this = simple
    ComplexQueryThreshold float64  `json:"complex_query_threshold"` // > this = complex
    
    // Model tiers
    SimpleModels  []string `json:"simple_models"`  // Cheap models
    MediumModels  []string `json:"medium_models"`  // Mid-tier
    ComplexModels []string `json:"complex_models"` // Premium
}

type LatencyRoutingConfig struct {
    // Use models with latency below threshold
    MaxLatencyMs     int      `json:"max_latency_ms"`
    PreferredModels  []string `json:"preferred_models"` // Try these first
}

type WeightedRoutingConfig struct {
    // Provider weights (must sum to 100)
    Weights map[string]int `json:"weights"` // provider -> weight %
}

type CapabilityRoutingConfig struct {
    // Task type -> preferred models
    TaskModels map[string][]string `json:"task_models"`
    // e.g., "code" -> ["gpt-4", "claude-3-opus"]
    //       "translation" -> ["gpt-4", "gemini-pro"]
}
```

**GraphQL Schema:**
```graphql
enum RoutingStrategy {
    COST
    LATENCY
    WEIGHTED
    ROUND_ROBIN
    CAPABILITY
}

input RoutingPolicyInput {
    enabled: Boolean!
    strategy: RoutingStrategy!
    costConfig: CostRoutingConfigInput
    latencyConfig: LatencyRoutingConfigInput
    weightedConfig: WeightedRoutingConfigInput
    allowModelOverride: Boolean
}

type RoutingPolicy {
    enabled: Boolean!
    strategy: RoutingStrategy!
    costConfig: CostRoutingConfig
    latencyConfig: LatencyRoutingConfig
    weightedConfig: WeightedRoutingConfig
    allowModelOverride: Boolean!
}
```

**UI Configuration:**

```
┌─────────────────────────────────────────────────────────────────┐
│  🚦 Routing Policy                                   [Enabled]  │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  Strategy: ● Cost-Optimized                                     │
│            ○ Latency-Optimized                                  │
│            ○ Weighted Distribution                              │
│            ○ Round Robin                                        │
│            ○ Capability-Based                                   │
│                                                                  │
│  ─── Cost-Optimized Settings ───────────────────────────────── │
│                                                                  │
│  Simple Query Threshold    [========●==] 0.30                   │
│  Complex Query Threshold   [=======●===] 0.70                   │
│                                                                  │
│  Simple Queries (cheap):                                        │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │ gpt-3.5-turbo │ claude-3-haiku │ + Add                     │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  Medium Queries:                                                 │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │ gpt-4o-mini │ claude-3-sonnet │ + Add                      │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  Complex Queries (premium):                                      │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │ gpt-4o │ claude-3-opus │ + Add                             │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ☑ Allow explicit model override (bypass routing)              │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

### 3. Resilience Policy

Controls failover, retry, and circuit breaker behavior per role.

```go
type ResiliencePolicy struct {
    // Master switch
    Enabled          bool `json:"enabled"`
    
    // Retry configuration
    RetryEnabled     bool `json:"retry_enabled"`
    MaxRetries       int  `json:"max_retries"`        // Default: 3
    RetryBackoffMs   int  `json:"retry_backoff_ms"`   // Base backoff, default: 1000
    RetryBackoffMax  int  `json:"retry_backoff_max"`  // Max backoff, default: 30000
    RetryJitter      bool `json:"retry_jitter"`       // Add randomness to backoff
    
    // Retryable errors
    RetryOnTimeout   bool     `json:"retry_on_timeout"`
    RetryOnRateLimit bool     `json:"retry_on_rate_limit"`
    RetryOnServerError bool   `json:"retry_on_server_error"` // 5xx errors
    RetryableErrors  []string `json:"retryable_errors"`      // Custom error codes
    
    // Fallback chain
    FallbackEnabled  bool              `json:"fallback_enabled"`
    FallbackChain    []FallbackConfig  `json:"fallback_chain"`
    
    // Circuit breaker
    CircuitBreakerEnabled  bool `json:"circuit_breaker_enabled"`
    CircuitBreakerThreshold int  `json:"circuit_breaker_threshold"` // Failures before open
    CircuitBreakerTimeout   int  `json:"circuit_breaker_timeout"`   // Seconds before half-open
    
    // Timeout
    RequestTimeoutMs int `json:"request_timeout_ms"` // Per-request timeout
}

type FallbackConfig struct {
    Provider  string `json:"provider"`
    Model     string `json:"model"`
    Priority  int    `json:"priority"`  // Lower = higher priority
    TimeoutMs int    `json:"timeout_ms"`
}
```

**GraphQL Schema:**
```graphql
input ResiliencePolicyInput {
    enabled: Boolean!
    
    # Retry
    retryEnabled: Boolean
    maxRetries: Int
    retryBackoffMs: Int
    retryBackoffMax: Int
    retryJitter: Boolean
    retryOnTimeout: Boolean
    retryOnRateLimit: Boolean
    retryOnServerError: Boolean
    
    # Fallback
    fallbackEnabled: Boolean
    fallbackChain: [FallbackConfigInput!]
    
    # Circuit Breaker
    circuitBreakerEnabled: Boolean
    circuitBreakerThreshold: Int
    circuitBreakerTimeout: Int
    
    # Timeout
    requestTimeoutMs: Int
}

input FallbackConfigInput {
    provider: String!
    model: String!
    priority: Int!
    timeoutMs: Int
}
```

**UI Configuration:**

```
┌─────────────────────────────────────────────────────────────────┐
│  🛡️ Resilience Policy                                [Enabled] │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ─── Retry Settings ──────────────────────────────── [Enabled] │
│                                                                  │
│  Max Retries:        [3]                                        │
│  Backoff Base:       [1000] ms                                  │
│  Backoff Max:        [30000] ms                                 │
│  ☑ Add jitter to backoff                                       │
│                                                                  │
│  Retry on:                                                       │
│  ☑ Timeout    ☑ Rate Limit    ☑ Server Errors (5xx)            │
│                                                                  │
│  ─── Fallback Chain ─────────────────────────────── [Enabled]  │
│                                                                  │
│  Priority │ Provider  │ Model              │ Timeout            │
│  ─────────┼───────────┼────────────────────┼──────────          │
│     1     │ OpenAI    │ gpt-4o             │ 30s     [↑] [↓] [✕]│
│     2     │ Anthropic │ claude-3-5-sonnet  │ 30s     [↑] [↓] [✕]│
│     3     │ Gemini    │ gemini-pro         │ 30s     [↑] [↓] [✕]│
│  ─────────┴───────────┴────────────────────┴──────────          │
│                                    [+ Add Fallback Provider]    │
│                                                                  │
│  ─── Circuit Breaker ────────────────────────────── [Enabled]  │
│                                                                  │
│  Failure Threshold:  [5] failures before opening                │
│  Reset Timeout:      [60] seconds before half-open              │
│                                                                  │
│  ─── Timeout ───────────────────────────────────────────────── │
│                                                                  │
│  Request Timeout:    [30000] ms                                 │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

### 4. Budget Policy

Controls budget limits and alerts per role.

```go
type BudgetPolicy struct {
    // Master switch
    Enabled           bool    `json:"enabled"`
    
    // Budget limits (0 = unlimited)
    DailyLimitUSD     float64 `json:"daily_limit_usd"`
    WeeklyLimitUSD    float64 `json:"weekly_limit_usd"`
    MonthlyLimitUSD   float64 `json:"monthly_limit_usd"`
    
    // Per-request limits
    MaxCostPerRequest float64 `json:"max_cost_per_request"`
    
    // Alert thresholds (0.0-1.0)
    AlertThreshold    float64 `json:"alert_threshold"`    // Alert at this % of budget
    CriticalThreshold float64 `json:"critical_threshold"` // Critical alert at this %
    
    // Alert channels
    AlertWebhook      string   `json:"alert_webhook"`
    AlertEmails       []string `json:"alert_emails"`
    AlertSlack        string   `json:"alert_slack"` // Slack webhook
    
    // Behavior when budget exceeded
    OnExceeded        BudgetExceededAction `json:"on_exceeded"`
    
    // Soft limits (warn but allow)
    SoftLimitEnabled  bool    `json:"soft_limit_enabled"`
    SoftLimitBuffer   float64 `json:"soft_limit_buffer"` // Allow this % over budget
}

type BudgetExceededAction string

const (
    BudgetActionBlock   BudgetExceededAction = "block"   // Block all requests
    BudgetActionWarn    BudgetExceededAction = "warn"    // Allow but warn
    BudgetActionThrottle BudgetExceededAction = "throttle" // Reduce rate limit
)
```

**GraphQL Schema:**
```graphql
enum BudgetExceededAction {
    BLOCK
    WARN
    THROTTLE
}

input BudgetPolicyInput {
    enabled: Boolean!
    dailyLimitUSD: Float
    weeklyLimitUSD: Float
    monthlyLimitUSD: Float
    maxCostPerRequest: Float
    alertThreshold: Float
    criticalThreshold: Float
    alertWebhook: String
    alertEmails: [String!]
    alertSlack: String
    onExceeded: BudgetExceededAction
    softLimitEnabled: Boolean
    softLimitBuffer: Float
}
```

**UI Configuration:**

```
┌─────────────────────────────────────────────────────────────────┐
│  💰 Budget Policy                                    [Enabled]  │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ─── Spending Limits ─────────────────────────────────────────  │
│                                                                  │
│  Daily Limit:     $ [50.00]                                     │
│  Weekly Limit:    $ [250.00]                                    │
│  Monthly Limit:   $ [1000.00]                                   │
│  Per-Request Max: $ [5.00]                                      │
│                                                                  │
│  ─── Alert Thresholds ────────────────────────────────────────  │
│                                                                  │
│  Warning Alert:   [====●=====] 80%                              │
│  Critical Alert:  [======●===] 95%                              │
│                                                                  │
│  ─── Alert Channels ──────────────────────────────────────────  │
│                                                                  │
│  Webhook URL:  [https://hooks.company.com/budget-alerts      ]  │
│                                                                  │
│  Email Recipients:                                               │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │ admin@company.com ✕ │ finance@company.com ✕ │ + Add        │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  Slack Webhook: [https://hooks.slack.com/services/...        ]  │
│                                                                  │
│  ─── When Budget Exceeded ────────────────────────────────────  │
│                                                                  │
│  Action: ● Block all requests                                   │
│          ○ Warn but allow                                       │
│          ○ Throttle (reduce rate limit)                         │
│                                                                  │
│  ☐ Enable soft limit (allow small buffer over budget)          │
│     Buffer: [10]%                                               │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## Updated Database Schema

```sql
-- migrations/tenant/003_extended_policies.sql

-- Add new policy columns to roles table
ALTER TABLE roles 
ADD COLUMN IF NOT EXISTS caching_policy JSONB DEFAULT '{"enabled": false}',
ADD COLUMN IF NOT EXISTS routing_policy JSONB DEFAULT '{"enabled": false}',
ADD COLUMN IF NOT EXISTS resilience_policy JSONB DEFAULT '{"enabled": false}',
ADD COLUMN IF NOT EXISTS budget_policy JSONB DEFAULT '{"enabled": false}';

-- Create index for policy lookups
CREATE INDEX IF NOT EXISTS idx_roles_policies ON roles USING GIN (
    prompt_policies, 
    tool_policies, 
    rate_limit_policy, 
    model_restrictions,
    caching_policy,
    routing_policy,
    resilience_policy,
    budget_policy
);

-- Budget usage tracking per role
CREATE TABLE IF NOT EXISTS role_budget_usage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    daily_cost_usd DECIMAL(10, 4) NOT NULL DEFAULT 0,
    weekly_cost_usd DECIMAL(10, 4) NOT NULL DEFAULT 0,
    monthly_cost_usd DECIMAL(10, 4) NOT NULL DEFAULT 0,
    request_count INT NOT NULL DEFAULT 0,
    token_count BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(role_id, date)
);

-- Cache statistics per role
CREATE TABLE IF NOT EXISTS role_cache_stats (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    cache_hits INT NOT NULL DEFAULT 0,
    cache_misses INT NOT NULL DEFAULT 0,
    cost_saved_usd DECIMAL(10, 4) NOT NULL DEFAULT 0,
    latency_saved_ms BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(role_id, date)
);

-- Circuit breaker state per provider per tenant
CREATE TABLE IF NOT EXISTS circuit_breaker_state (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    provider VARCHAR(50) NOT NULL,
    state VARCHAR(20) NOT NULL DEFAULT 'closed', -- closed, open, half_open
    failure_count INT NOT NULL DEFAULT 0,
    last_failure_at TIMESTAMP,
    opened_at TIMESTAMP,
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(tenant_id, provider)
);
```

---

## Policy Enforcement Flow

```go
// internal/policy/enforcement.go

func (s *EnforcementService) EnforcePolicy(ctx context.Context, enfCtx *EnforcementContext) error {
    if enfCtx.Policy == nil {
        return nil
    }

    // 1. Model Restriction Check (existing)
    if err := s.validateModelRestrictions(enfCtx); err != nil {
        return err
    }

    // 2. Prompt Policy Check (existing + PII redaction)
    if err := s.validatePromptPolicies(enfCtx); err != nil {
        return err
    }

    // 3. Tool Policy Check (existing)
    if err := s.validateToolPolicies(enfCtx); err != nil {
        return err
    }

    // 4. Rate Limit Check (existing)
    if err := s.validateRateLimits(ctx, enfCtx); err != nil {
        return err
    }

    // 5. Budget Policy Check (NEW)
    if err := s.validateBudget(ctx, enfCtx); err != nil {
        return err
    }

    // Store policy decisions in context for gateway to use
    enfCtx.PolicyDecisions = &PolicyDecisions{
        CachingEnabled:    enfCtx.Policy.CachingPolicy.Enabled,
        CachingConfig:     enfCtx.Policy.CachingPolicy,
        RoutingEnabled:    enfCtx.Policy.RoutingPolicy.Enabled,
        RoutingConfig:     enfCtx.Policy.RoutingPolicy,
        ResilienceEnabled: enfCtx.Policy.ResiliencePolicy.Enabled,
        ResilienceConfig:  enfCtx.Policy.ResiliencePolicy,
    }

    return nil
}

// PolicyDecisions carries policy configuration to the gateway
type PolicyDecisions struct {
    CachingEnabled    bool
    CachingConfig     CachingPolicy
    RoutingEnabled    bool
    RoutingConfig     RoutingPolicy
    ResilienceEnabled bool
    ResilienceConfig  ResiliencePolicy
}
```

---

## Gateway Using Policy Decisions

```go
// internal/gateway/gateway.go

func (s *Service) ChatComplete(ctx context.Context, req *domain.ChatRequest) (*domain.ChatResponse, error) {
    // Get policy decisions from context (set by enforcement layer)
    decisions := GetPolicyDecisions(ctx)
    
    // 1. Check semantic cache IF enabled by policy
    if decisions != nil && decisions.CachingEnabled {
        cacheConfig := decisions.CachingConfig
        if s.cache != nil && !s.isExcludedFromCache(req, cacheConfig) {
            if cached, found := s.cache.Get(ctx, req.TenantID, req.Model, 
                cacheConfig.SimilarityThreshold); found {
                return cached, nil
            }
        }
    }
    
    // 2. Apply intelligent routing IF enabled by policy
    if decisions != nil && decisions.RoutingEnabled && req.Model == "" {
        routingConfig := decisions.RoutingConfig
        if !routingConfig.AllowModelOverride || req.Model == "" {
            provider, model := s.route(ctx, req, routingConfig)
            req.Model = model
            ctx = context.WithValue(ctx, "preferred_provider", provider)
        }
    }
    
    // 3. Execute with resilience IF enabled by policy
    var response *domain.ChatResponse
    var err error
    
    if decisions != nil && decisions.ResilienceEnabled {
        resConfig := decisions.ResilienceConfig
        response, err = s.executeWithResilience(ctx, req, resConfig)
    } else {
        response, err = s.executeRequest(ctx, req)
    }
    
    if err != nil {
        return nil, err
    }
    
    // 4. Store in cache IF enabled by policy
    if decisions != nil && decisions.CachingEnabled && response.Error == nil {
        go s.cache.Set(ctx, req.TenantID, req.Model, response, 
            time.Duration(decisions.CachingConfig.TTLSeconds)*time.Second)
    }
    
    return response, nil
}

func (s *Service) executeWithResilience(ctx context.Context, req *domain.ChatRequest, config ResiliencePolicy) (*domain.ChatResponse, error) {
    // Build retry policy from config
    retryPolicy := &resilience.RetryPolicy{
        Enabled:      config.RetryEnabled,
        MaxRetries:   config.MaxRetries,
        BackoffBase:  time.Duration(config.RetryBackoffMs) * time.Millisecond,
        BackoffMax:   time.Duration(config.RetryBackoffMax) * time.Millisecond,
        Jitter:       config.RetryJitter,
    }
    
    // Build fallback chain from config
    var fallbackChain []resilience.FallbackProvider
    if config.FallbackEnabled {
        for _, fb := range config.FallbackChain {
            fallbackChain = append(fallbackChain, resilience.FallbackProvider{
                Provider: domain.Provider(fb.Provider),
                Model:    fb.Model,
                Priority: fb.Priority,
                Timeout:  time.Duration(fb.TimeoutMs) * time.Millisecond,
            })
        }
    }
    
    // Execute with resilience
    return s.resilience.Execute(ctx, req, retryPolicy, fallbackChain, 
        config.CircuitBreakerEnabled, s.executeRequest)
}
```

---

## UI: Policy Editor with All 8 Types

The Role Policy Editor should show all 8 policy types in a tabbed interface:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  Edit Role: production-api-user                                    [Save]   │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │ 📝 Prompts │ 🔧 Tools │ ⏱️ Rate Limit │ 🤖 Models │                   │   │
│  │ 🗄️ Caching │ 🚦 Routing │ 🛡️ Resilience │ 💰 Budget │                 │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
│                                                                              │
│  [Selected Tab Content Here]                                                 │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

## Default Policies by Tier

When a tenant is created, default role policies are created based on tier:

```go
func DefaultPoliciesForTier(tier TenantTier) RolePolicy {
    switch tier {
    case TierFree:
        return RolePolicy{
            CachingPolicy: CachingPolicy{
                Enabled:            true,  // Free tier gets caching to reduce costs
                SimilarityThreshold: 0.98,
                TTLSeconds:         3600,
            },
            RoutingPolicy: RoutingPolicy{
                Enabled:  true,
                Strategy: RoutingStrategyCost, // Always cost-optimized
            },
            ResiliencePolicy: ResiliencePolicy{
                Enabled:      false, // No failover for free tier
            },
            BudgetPolicy: BudgetPolicy{
                Enabled:       true,
                DailyLimitUSD: 1.0,
                MonthlyLimitUSD: 10.0,
                OnExceeded:    BudgetActionBlock,
            },
        }
        
    case TierProfessional:
        return RolePolicy{
            CachingPolicy: CachingPolicy{
                Enabled:            true,
                SimilarityThreshold: 0.95,
                TTLSeconds:         7200,
            },
            RoutingPolicy: RoutingPolicy{
                Enabled:  true,
                Strategy: RoutingStrategyCost,
            },
            ResiliencePolicy: ResiliencePolicy{
                Enabled:         true,
                RetryEnabled:    true,
                MaxRetries:      2,
                FallbackEnabled: true,
                CircuitBreakerEnabled: true,
            },
            BudgetPolicy: BudgetPolicy{
                Enabled:        true,
                MonthlyLimitUSD: 500.0,
                AlertThreshold:  0.8,
                OnExceeded:     BudgetActionWarn,
            },
        }
        
    case TierEnterprise:
        return RolePolicy{
            CachingPolicy: CachingPolicy{
                Enabled:            true,
                SimilarityThreshold: 0.90, // More aggressive caching
                TTLSeconds:         14400,
            },
            RoutingPolicy: RoutingPolicy{
                Enabled:  true,
                Strategy: RoutingStrategyLatency, // Latency-optimized
            },
            ResiliencePolicy: ResiliencePolicy{
                Enabled:         true,
                RetryEnabled:    true,
                MaxRetries:      3,
                FallbackEnabled: true,
                FallbackChain: []FallbackConfig{
                    {Provider: "openai", Model: "gpt-4o", Priority: 1},
                    {Provider: "anthropic", Model: "claude-3-5-sonnet", Priority: 2},
                    {Provider: "gemini", Model: "gemini-pro", Priority: 3},
                },
                CircuitBreakerEnabled: true,
            },
            BudgetPolicy: BudgetPolicy{
                Enabled:        true,
                MonthlyLimitUSD: 10000.0,
                AlertThreshold:  0.9,
                OnExceeded:     BudgetActionWarn, // Never block enterprise
            },
        }
    }
}
```

---

## Summary

| Policy Type | Controls | Implementation Layer |
|-------------|----------|---------------------|
| **Prompt** (existing) | PII, injection, length | `internal/policy/enforcement.go` |
| **Tool** (existing) | Tool allow/block | `internal/policy/enforcement.go` |
| **Rate Limit** (existing) | RPM, TPM limits | `internal/policy/enforcement.go` |
| **Model** (existing) | Model whitelist/blacklist | `internal/policy/enforcement.go` |
| **Caching** (NEW) | Semantic cache on/off, TTL, threshold | `internal/cache/semantic/` |
| **Routing** (NEW) | Cost/latency routing strategy | `internal/gateway/router.go` |
| **Resilience** (NEW) | Retry, failover, circuit breaker | `internal/resilience/` |
| **Budget** (NEW) | Spending limits, alerts | `internal/policy/budget.go` |

**Key Principle:** The policy framework is the **control plane**. The actual implementations live in specialized packages, but they all check policy configuration before acting.

