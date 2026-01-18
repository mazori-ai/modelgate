# ModelGate Prometheus Metrics

This document describes all Prometheus metrics exposed by ModelGate, including the newly added metrics for advanced features.

## Metrics Endpoint

All metrics are exposed at `/metrics` on the HTTP server port.

## Existing Core Metrics

### Request Metrics
- `modelgate_requests_total` - Total number of requests (labels: method, model, status, tenant_id)
- `modelgate_request_duration_seconds` - Request duration histogram (labels: method, model, tenant_id)
- `modelgate_requests_in_flight` - Current requests being processed (gauge)

### Token & Cost Metrics
- `modelgate_tokens_input_total` - Total input tokens (labels: model, provider, tenant_id)
- `modelgate_tokens_output_total` - Total output tokens (labels: model, provider, tenant_id)
- `modelgate_tokens_thinking_total` - Total thinking tokens (labels: model, provider, tenant_id)
- `modelgate_cost_usd_total` - Total cost in USD (labels: model, provider, tenant_id)

### Provider Metrics
- `modelgate_provider_requests_total` - Requests per provider (labels: provider, model)
- `modelgate_provider_errors_total` - Errors per provider (labels: provider, error_type)
- `modelgate_provider_latency_seconds` - Provider latency histogram (labels: provider, model)

### Tenant Metrics
- `modelgate_active_tenants` - Number of active tenants (gauge)
- `modelgate_tenant_requests_total` - Requests per tenant (labels: tenant_id, tier)
- `modelgate_tenant_tokens_total` - Tokens per tenant (labels: tenant_id, tier, type)
- `modelgate_tenant_cost_usd_total` - Cost per tenant (labels: tenant_id, tier)

---

## NEW: Semantic Cache Metrics

### Cache Performance
- **`modelgate_cache_hits_total`** - Total cache hits
  - Labels: `model`, `tenant_id`, `role_id`
  - Type: Counter
  - When: Incremented on every cache hit

- **`modelgate_cache_misses_total`** - Total cache misses
  - Labels: `model`, `tenant_id`, `role_id`
  - Type: Counter
  - When: Incremented on every cache miss

- **`modelgate_cache_latency_seconds`** - Cache lookup latency
  - Labels: `tenant_id`, `hit` (true/false)
  - Type: Histogram
  - Buckets: 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5
  - When: Recorded on every cache lookup

### Cache Savings
- **`modelgate_cache_tokens_saved_total`** - Total tokens saved via cache
  - Labels: `model`, `tenant_id`
  - Type: Counter
  - When: Incremented on cache hit with token count

- **`modelgate_cache_cost_saved_usd_total`** - Total cost saved via cache in USD
  - Labels: `model`, `tenant_id`
  - Type: Counter
  - When: Incremented on cache hit with cost amount

### Cache State
- **`modelgate_cache_entries`** - Number of cache entries per tenant
  - Labels: `tenant_id`
  - Type: Gauge
  - When: Updated periodically or on cache operations

**Example Queries:**
```promql
# Cache hit rate by tenant
rate(modelgate_cache_hits_total[5m]) / (rate(modelgate_cache_hits_total[5m]) + rate(modelgate_cache_misses_total[5m]))

# Cost savings per hour
rate(modelgate_cache_cost_saved_usd_total[1h])

# P95 cache lookup latency
histogram_quantile(0.95, rate(modelgate_cache_latency_seconds_bucket[5m]))
```

---

## NEW: Intelligent Routing Metrics

### Routing Decisions
- **`modelgate_routing_decisions_total`** - Total routing decisions
  - Labels: `strategy` (cost, latency, weighted, round_robin, capability), `tenant_id`
  - Type: Counter
  - When: Incremented on every routing decision

- **`modelgate_routing_model_switch_total`** - Total model switches by routing
  - Labels: `from_model`, `to_model`, `strategy`, `tenant_id`
  - Type: Counter
  - When: Incremented when routing selects a different model

- **`modelgate_routing_failures_total`** - Total routing failures
  - Labels: `reason`, `tenant_id`
  - Type: Counter
  - When: Incremented on routing errors

**Example Queries:**
```promql
# Routing strategy distribution
sum by (strategy) (rate(modelgate_routing_decisions_total[5m]))

# Most common model switches
topk(5, sum by (from_model, to_model) (rate(modelgate_routing_model_switch_total[1h])))
```

---

## NEW: Resilience & HA Metrics

### Circuit Breaker
- **`modelgate_circuit_breaker_state`** - Circuit breaker state
  - Labels: `provider`, `tenant_id`
  - Type: Gauge
  - Values: 0=closed, 1=half-open, 2=open
  - When: Updated on state transitions

### Retry Behavior
- **`modelgate_retry_attempts_total`** - Total retry attempts
  - Labels: `provider`, `tenant_id`, `reason`
  - Type: Counter
  - When: Incremented on each retry attempt

### Fallback Execution
- **`modelgate_fallback_invocations_total`** - Total fallback chain invocations
  - Labels: `primary_provider`, `fallback_provider`, `tenant_id`
  - Type: Counter
  - When: Incremented when fallback chain is triggered

- **`modelgate_fallback_success_total`** - Successful fallback executions
  - Labels: `fallback_provider`, `tenant_id`
  - Type: Counter
  - When: Incremented when fallback succeeds

**Example Queries:**
```promql
# Providers with open circuit breakers
count by (provider) (modelgate_circuit_breaker_state == 2)

# Fallback success rate
rate(modelgate_fallback_success_total[5m]) / rate(modelgate_fallback_invocations_total[5m])

# Retry frequency by provider
sum by (provider) (rate(modelgate_retry_attempts_total[5m]))
```

---

## NEW: Health Tracking Metrics

### Provider Health
- **`modelgate_provider_health_score`** - Provider health score (0-1)
  - Labels: `provider`, `model`, `tenant_id`
  - Type: Gauge
  - Range: 0.0 (unhealthy) to 1.0 (healthy)
  - When: Updated on every request completion

- **`modelgate_provider_success_rate`** - Provider success rate (0-1)
  - Labels: `provider`, `model`, `tenant_id`
  - Type: Gauge
  - Range: 0.0 to 1.0
  - When: Updated based on recent requests

- **`modelgate_provider_p95_latency_ms`** - Provider P95 latency in milliseconds
  - Labels: `provider`, `model`, `tenant_id`
  - Type: Gauge
  - When: Updated based on latency tracking

**Example Queries:**
```promql
# Unhealthy providers (health < 0.8)
modelgate_provider_health_score < 0.8

# Slowest providers by P95 latency
topk(5, max by (provider) (modelgate_provider_p95_latency_ms))

# Success rate by provider
avg by (provider) (modelgate_provider_success_rate)
```

---

## NEW: Multi-Key Metrics

### API Key Usage
- **`modelgate_api_key_usage_total`** - Total API key usage
  - Labels: `provider`, `key_name`, `tenant_id`
  - Type: Counter
  - When: Incremented on each API key use

- **`modelgate_api_key_health_score`** - API key health score (0-1)
  - Labels: `provider`, `key_name`, `tenant_id`
  - Type: Gauge
  - Range: 0.0 (unhealthy) to 1.0 (healthy)
  - When: Updated on key usage

- **`modelgate_api_key_rate_limits_total`** - Total rate limit hits
  - Labels: `provider`, `key_name`, `tenant_id`
  - Type: Counter
  - When: Incremented on rate limit responses

**Example Queries:**
```promql
# API key usage distribution
sum by (key_name) (rate(modelgate_api_key_usage_total[1h]))

# Keys hitting rate limits frequently
rate(modelgate_api_key_rate_limits_total[5m]) > 0

# Unhealthy API keys
modelgate_api_key_health_score < 0.5
```

---

## Integration with Gateway

All metrics are automatically recorded in the gateway service during:
- **ChatComplete**: Non-streaming requests
- **ChatStream**: Streaming requests

### Recording Points

1. **Cache Metrics**: Recorded on cache lookups (hit/miss) in both ChatComplete and ChatStream
2. **Routing Metrics**: Recorded when routing makes model selection decisions
3. **Health Metrics**: Recorded by HealthTracker on every request completion
4. **Resilience Metrics**: Recorded by ResilienceService during retry/fallback operations
5. **Multi-Key Metrics**: Recorded by KeySelector when selecting API keys

---

## Grafana Dashboard Examples

### Cache Performance Dashboard
```
Row 1: Cache Hit Rate, Cost Savings, Token Savings
Row 2: Cache Latency P50/P95/P99
Row 3: Cache Entries by Tenant
```

### Routing Intelligence Dashboard
```
Row 1: Routing Strategy Distribution
Row 2: Model Switches by Strategy
Row 3: Routing Failures by Reason
```

### Resilience Dashboard
```
Row 1: Circuit Breaker States
Row 2: Retry Attempts by Provider
Row 3: Fallback Success Rate
```

### Provider Health Dashboard
```
Row 1: Provider Health Scores
Row 2: Provider Success Rates
Row 3: Provider P95 Latency Comparison
```

---

## Alerting Recommendations

### Critical Alerts
```yaml
# High cache miss rate
alert: HighCacheMissRate
expr: rate(modelgate_cache_misses_total[5m]) / (rate(modelgate_cache_hits_total[5m]) + rate(modelgate_cache_misses_total[5m])) > 0.9
for: 10m
severity: warning

# Circuit breaker open
alert: CircuitBreakerOpen
expr: modelgate_circuit_breaker_state == 2
for: 5m
severity: critical

# Low provider health
alert: LowProviderHealth
expr: modelgate_provider_health_score < 0.5
for: 10m
severity: warning

# High API key rate limits
alert: HighRateLimitHits
expr: rate(modelgate_api_key_rate_limits_total[5m]) > 0.1
for: 5m
severity: warning
```

---

## Accessing Metrics

### Prometheus Configuration
```yaml
scrape_configs:
  - job_name: 'modelgate'
    static_configs:
      - targets: ['localhost:8081']  # HTTP server port
    scrape_interval: 15s
    metrics_path: '/metrics'
```

### Example cURL
```bash
curl http://localhost:8081/metrics | grep modelgate_cache
```

---

**Last Updated**: January 4, 2026
**Status**: ✅ All metrics implemented and integrated
