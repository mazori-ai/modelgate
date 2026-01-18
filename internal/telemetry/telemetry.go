// Package telemetry provides observability with Prometheus metrics and structured logging.
package telemetry

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metrics for ModelGate
type Metrics struct {
	// Request metrics
	RequestsTotal    *prometheus.CounterVec
	RequestDuration  *prometheus.HistogramVec
	RequestsInFlight prometheus.Gauge

	// Token metrics
	TokensInput    *prometheus.CounterVec
	TokensOutput   *prometheus.CounterVec
	TokensThinking *prometheus.CounterVec

	// Cost metrics
	CostUSD *prometheus.CounterVec

	// Provider metrics
	ProviderRequests *prometheus.CounterVec
	ProviderErrors   *prometheus.CounterVec
	ProviderLatency  *prometheus.HistogramVec

	// Tool metrics
	ToolCalls  *prometheus.CounterVec
	ToolErrors *prometheus.CounterVec

	// Policy metrics
	PolicyEvaluations *prometheus.CounterVec
	PolicyViolations  *prometheus.CounterVec

	// Tenant metrics
	ActiveTenants  prometheus.Gauge
	TenantRequests *prometheus.CounterVec
	TenantTokens   *prometheus.CounterVec
	TenantCost     *prometheus.CounterVec

	// Prompt safety metrics
	PromptAnalysis    *prometheus.CounterVec
	OutlierDetections *prometheus.CounterVec

	// System metrics
	StreamConnections prometheus.Gauge

	// NEW: Semantic Cache Metrics
	CacheHits        *prometheus.CounterVec   // Cache hits by model, tenant
	CacheMisses      *prometheus.CounterVec   // Cache misses by model, tenant
	CacheTokensSaved *prometheus.CounterVec   // Tokens saved via cache
	CacheCostSaved   *prometheus.CounterVec   // Cost saved via cache (USD)
	CacheEntries     *prometheus.GaugeVec     // Number of cache entries per tenant
	CacheLatency     *prometheus.HistogramVec // Cache lookup latency

	// NEW: Routing Metrics
	RoutingDecisions   *prometheus.CounterVec // Routing decisions by strategy
	RoutingModelSwitch *prometheus.CounterVec // Model switches by routing
	RoutingFailures    *prometheus.CounterVec // Routing failures by reason

	// NEW: Resilience Metrics
	CircuitBreakerState *prometheus.GaugeVec   // Circuit breaker state (0=closed, 1=half-open, 2=open)
	RetryAttempts       *prometheus.CounterVec // Retry attempts by provider
	FallbackInvocations *prometheus.CounterVec // Fallback chain invocations
	FallbackSuccess     *prometheus.CounterVec // Successful fallback executions

	// NEW: Health Tracking Metrics
	ProviderHealth      *prometheus.GaugeVec // Provider health score (0-1)
	ProviderSuccessRate *prometheus.GaugeVec // Provider success rate
	ProviderP95Latency  *prometheus.GaugeVec // Provider P95 latency

	// NEW: Multi-Key Metrics
	APIKeyUsage      *prometheus.CounterVec // API key usage by provider
	APIKeyHealth     *prometheus.GaugeVec   // API key health score
	APIKeyRateLimits *prometheus.CounterVec // Rate limit hits by key
}

// NewMetrics creates and registers all metrics
func NewMetrics(registry prometheus.Registerer) *Metrics {
	if registry == nil {
		registry = prometheus.DefaultRegisterer
	}

	factory := promauto.With(registry)

	return &Metrics{
		RequestsTotal: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "modelgate_requests_total",
				Help: "Total number of requests",
			},
			[]string{"method", "model", "status", "tenant_id"},
		),

		RequestDuration: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "modelgate_request_duration_seconds",
				Help:    "Request duration in seconds",
				Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 120},
			},
			[]string{"method", "model", "tenant_id"},
		),

		RequestsInFlight: factory.NewGauge(
			prometheus.GaugeOpts{
				Name: "modelgate_requests_in_flight",
				Help: "Number of requests currently being processed",
			},
		),

		TokensInput: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "modelgate_tokens_input_total",
				Help: "Total input tokens processed",
			},
			[]string{"model", "provider", "tenant_id"},
		),

		TokensOutput: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "modelgate_tokens_output_total",
				Help: "Total output tokens generated",
			},
			[]string{"model", "provider", "tenant_id"},
		),

		TokensThinking: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "modelgate_tokens_thinking_total",
				Help: "Total thinking/reasoning tokens used",
			},
			[]string{"model", "provider", "tenant_id"},
		),

		CostUSD: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "modelgate_cost_usd_total",
				Help: "Total cost in USD",
			},
			[]string{"model", "provider", "tenant_id"},
		),

		ProviderRequests: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "modelgate_provider_requests_total",
				Help: "Total requests per provider",
			},
			[]string{"provider", "model"},
		),

		ProviderErrors: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "modelgate_provider_errors_total",
				Help: "Total errors per provider",
			},
			[]string{"provider", "error_type"},
		),

		ProviderLatency: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "modelgate_provider_latency_seconds",
				Help:    "Provider API latency in seconds",
				Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
			},
			[]string{"provider", "model"},
		),

		ToolCalls: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "modelgate_tool_calls_total",
				Help: "Total tool calls",
			},
			[]string{"tool_name", "model", "tenant_id"},
		),

		ToolErrors: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "modelgate_tool_errors_total",
				Help: "Total tool call errors",
			},
			[]string{"tool_name", "error_type"},
		),

		PolicyEvaluations: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "modelgate_policy_evaluations_total",
				Help: "Total policy evaluations",
			},
			[]string{"policy_type", "result"},
		),

		PolicyViolations: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "modelgate_policy_violations_total",
				Help: "Total policy violations",
			},
			[]string{"policy_id", "violation_type", "severity"},
		),

		ActiveTenants: factory.NewGauge(
			prometheus.GaugeOpts{
				Name: "modelgate_active_tenants",
				Help: "Number of active tenants",
			},
		),

		TenantRequests: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "modelgate_tenant_requests_total",
				Help: "Total requests per tenant",
			},
			[]string{"tenant_id", "tier"},
		),

		TenantTokens: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "modelgate_tenant_tokens_total",
				Help: "Total tokens per tenant",
			},
			[]string{"tenant_id", "tier", "type"},
		),

		TenantCost: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "modelgate_tenant_cost_usd_total",
				Help: "Total cost per tenant in USD",
			},
			[]string{"tenant_id", "tier"},
		),

		PromptAnalysis: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "modelgate_prompt_analysis_total",
				Help: "Total prompt safety analyses",
			},
			[]string{"risk_level", "tenant_id"},
		),

		OutlierDetections: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "modelgate_outlier_detections_total",
				Help: "Total outlier detections",
			},
			[]string{"outlier_type", "tenant_id"},
		),

		StreamConnections: factory.NewGauge(
			prometheus.GaugeOpts{
				Name: "modelgate_stream_connections",
				Help: "Number of active streaming connections",
			},
		),

		// NEW: Semantic Cache Metrics
		CacheHits: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "modelgate_cache_hits_total",
				Help: "Total cache hits",
			},
			[]string{"model", "tenant_id", "role_id"},
		),

		CacheMisses: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "modelgate_cache_misses_total",
				Help: "Total cache misses",
			},
			[]string{"model", "tenant_id", "role_id"},
		),

		CacheTokensSaved: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "modelgate_cache_tokens_saved_total",
				Help: "Total tokens saved via semantic cache",
			},
			[]string{"model", "tenant_id"},
		),

		CacheCostSaved: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "modelgate_cache_cost_saved_usd_total",
				Help: "Total cost saved via semantic cache in USD",
			},
			[]string{"model", "tenant_id"},
		),

		CacheEntries: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "modelgate_cache_entries",
				Help: "Number of cache entries per tenant",
			},
			[]string{"tenant_id"},
		),

		CacheLatency: factory.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "modelgate_cache_lookup_seconds",
				Help:    "Cache lookup latency in seconds",
				Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5},
			},
			[]string{"tenant_id", "hit"},
		),

		// NEW: Routing Metrics
		RoutingDecisions: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "modelgate_routing_decisions_total",
				Help: "Total routing decisions by strategy",
			},
			[]string{"strategy", "tenant_id"},
		),

		RoutingModelSwitch: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "modelgate_routing_model_switch_total",
				Help: "Total model switches by routing",
			},
			[]string{"from_model", "to_model", "strategy", "tenant_id"},
		),

		RoutingFailures: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "modelgate_routing_failures_total",
				Help: "Total routing failures",
			},
			[]string{"reason", "tenant_id"},
		),

		// NEW: Resilience Metrics
		CircuitBreakerState: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "modelgate_circuit_breaker_state",
				Help: "Circuit breaker state (0=closed, 1=half-open, 2=open)",
			},
			[]string{"provider", "tenant_id"},
		),

		RetryAttempts: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "modelgate_retry_attempts_total",
				Help: "Total retry attempts",
			},
			[]string{"provider", "tenant_id", "reason"},
		),

		FallbackInvocations: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "modelgate_fallback_invocations_total",
				Help: "Total fallback chain invocations",
			},
			[]string{"primary_provider", "fallback_provider", "tenant_id"},
		),

		FallbackSuccess: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "modelgate_fallback_success_total",
				Help: "Total successful fallback executions",
			},
			[]string{"fallback_provider", "tenant_id"},
		),

		// NEW: Health Tracking Metrics
		ProviderHealth: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "modelgate_provider_health_score",
				Help: "Provider health score (0-1)",
			},
			[]string{"provider", "model", "tenant_id"},
		),

		ProviderSuccessRate: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "modelgate_provider_success_rate",
				Help: "Provider success rate (0-1)",
			},
			[]string{"provider", "model", "tenant_id"},
		),

		ProviderP95Latency: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "modelgate_provider_p95_latency_ms",
				Help: "Provider P95 latency in milliseconds",
			},
			[]string{"provider", "model", "tenant_id"},
		),

		// NEW: Multi-Key Metrics
		APIKeyUsage: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "modelgate_api_key_usage_total",
				Help: "Total API key usage by provider",
			},
			[]string{"provider", "key_name", "tenant_id"},
		),

		APIKeyHealth: factory.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "modelgate_api_key_health_score",
				Help: "API key health score (0-1)",
			},
			[]string{"provider", "key_name", "tenant_id"},
		),

		APIKeyRateLimits: factory.NewCounterVec(
			prometheus.CounterOpts{
				Name: "modelgate_api_key_rate_limits_total",
				Help: "Total rate limit hits by key",
			},
			[]string{"provider", "key_name", "tenant_id"},
		),
	}
}

// Handler returns an HTTP handler for Prometheus metrics
func Handler() http.Handler {
	return promhttp.Handler()
}

// RequestRecorder helps record metrics for a request
type RequestRecorder struct {
	metrics   *Metrics
	method    string
	model     string
	tenantID  string
	provider  string
	startTime time.Time
}

// NewRequestRecorder creates a new request recorder
func (m *Metrics) NewRequestRecorder(method, model, tenantID, provider string) *RequestRecorder {
	m.RequestsInFlight.Inc()
	return &RequestRecorder{
		metrics:   m,
		method:    method,
		model:     model,
		tenantID:  tenantID,
		provider:  provider,
		startTime: time.Now(),
	}
}

// RecordSuccess records a successful request
func (r *RequestRecorder) RecordSuccess(inputTokens, outputTokens int64, costUSD float64) {
	duration := time.Since(r.startTime).Seconds()

	r.metrics.RequestsInFlight.Dec()
	r.metrics.RequestsTotal.WithLabelValues(r.method, r.model, "success", r.tenantID).Inc()
	r.metrics.RequestDuration.WithLabelValues(r.method, r.model, r.tenantID).Observe(duration)

	r.metrics.TokensInput.WithLabelValues(r.model, r.provider, r.tenantID).Add(float64(inputTokens))
	r.metrics.TokensOutput.WithLabelValues(r.model, r.provider, r.tenantID).Add(float64(outputTokens))
	r.metrics.CostUSD.WithLabelValues(r.model, r.provider, r.tenantID).Add(costUSD)

	r.metrics.ProviderRequests.WithLabelValues(r.provider, r.model).Inc()
	r.metrics.ProviderLatency.WithLabelValues(r.provider, r.model).Observe(duration)
}

// RecordError records a failed request
func (r *RequestRecorder) RecordError(errorType string) {
	duration := time.Since(r.startTime).Seconds()

	r.metrics.RequestsInFlight.Dec()
	r.metrics.RequestsTotal.WithLabelValues(r.method, r.model, "error", r.tenantID).Inc()
	r.metrics.RequestDuration.WithLabelValues(r.method, r.model, r.tenantID).Observe(duration)

	r.metrics.ProviderErrors.WithLabelValues(r.provider, errorType).Inc()
}

// RecordToolCall records a tool call
func (m *Metrics) RecordToolCall(toolName, model, tenantID string) {
	m.ToolCalls.WithLabelValues(toolName, model, tenantID).Inc()
}

// RecordPolicyViolation records a policy violation
func (m *Metrics) RecordPolicyViolation(policyID, violationType, severity string) {
	m.PolicyViolations.WithLabelValues(policyID, violationType, severity).Inc()
}

// RecordPromptAnalysis records prompt analysis results
func (m *Metrics) RecordPromptAnalysis(riskLevel, tenantID string) {
	m.PromptAnalysis.WithLabelValues(riskLevel, tenantID).Inc()
}

// RecordOutlierDetection records an outlier detection
func (m *Metrics) RecordOutlierDetection(outlierType, tenantID string) {
	m.OutlierDetections.WithLabelValues(outlierType, tenantID).Inc()
}

// Logger provides structured logging
type Logger interface {
	Debug(msg string, fields ...any)
	Info(msg string, fields ...any)
	Warn(msg string, fields ...any)
	Error(msg string, fields ...any)
	With(fields ...any) Logger
}

// Context key for logger
type loggerContextKey struct{}

// LoggerFromContext retrieves logger from context
func LoggerFromContext(ctx context.Context) Logger {
	if l, ok := ctx.Value(loggerContextKey{}).(Logger); ok {
		return l
	}
	return &noopLogger{}
}

// ContextWithLogger adds logger to context
func ContextWithLogger(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey{}, logger)
}

// noopLogger is a no-op logger
type noopLogger struct{}

func (noopLogger) Debug(msg string, fields ...any) {}
func (noopLogger) Info(msg string, fields ...any)  {}
func (noopLogger) Warn(msg string, fields ...any)  {}
func (noopLogger) Error(msg string, fields ...any) {}
func (l noopLogger) With(fields ...any) Logger     { return l }

// RecordRequest records a simple request metric
func (m *Metrics) RecordRequest(method, status string, duration time.Duration) {
	m.RequestsTotal.WithLabelValues(method, "", status, "").Inc()
	m.RequestDuration.WithLabelValues(method, "", "").Observe(duration.Seconds())
}

// Init initializes the telemetry system
func Init(cfg interface{}) (*Metrics, func(), error) {
	metrics := NewMetrics(nil)
	return metrics, func() {}, nil
}

// ============================================================================
// NEW: Advanced Feature Metrics Recording Methods
// ============================================================================

// RecordCacheHit records a semantic cache hit
func (m *Metrics) RecordCacheHit(model, tenantID, roleID string, tokensSaved int64, costSaved float64) {
	m.CacheHits.WithLabelValues(model, tenantID, roleID).Inc()
	if tokensSaved > 0 {
		m.CacheTokensSaved.WithLabelValues(model, tenantID).Add(float64(tokensSaved))
	}
	if costSaved > 0 {
		m.CacheCostSaved.WithLabelValues(model, tenantID).Add(costSaved)
	}
}

// RecordCacheMiss records a semantic cache miss
func (m *Metrics) RecordCacheMiss(model, tenantID, roleID string) {
	m.CacheMisses.WithLabelValues(model, tenantID, roleID).Inc()
}

// RecordCacheLookup records cache lookup latency
func (m *Metrics) RecordCacheLookup(tenantID string, hit bool, duration time.Duration) {
	hitStr := "false"
	if hit {
		hitStr = "true"
	}
	m.CacheLatency.WithLabelValues(tenantID, hitStr).Observe(duration.Seconds())
}

// UpdateCacheEntries updates the cache entries gauge
func (m *Metrics) UpdateCacheEntries(tenantID string, count int) {
	m.CacheEntries.WithLabelValues(tenantID).Set(float64(count))
}

// RecordRoutingDecision records a routing decision
func (m *Metrics) RecordRoutingDecision(strategy, tenantID string) {
	m.RoutingDecisions.WithLabelValues(strategy, tenantID).Inc()
}

// RecordModelSwitch records when routing switches models
func (m *Metrics) RecordModelSwitch(fromModel, toModel, strategy, tenantID string) {
	m.RoutingModelSwitch.WithLabelValues(fromModel, toModel, strategy, tenantID).Inc()
}

// RecordRoutingFailure records a routing failure
func (m *Metrics) RecordRoutingFailure(reason, tenantID string) {
	m.RoutingFailures.WithLabelValues(reason, tenantID).Inc()
}

// UpdateCircuitBreakerState updates circuit breaker state gauge
// state: 0=closed, 1=half-open, 2=open
func (m *Metrics) UpdateCircuitBreakerState(provider, tenantID, state string) {
	var stateValue float64
	switch state {
	case "closed":
		stateValue = 0
	case "half_open":
		stateValue = 1
	case "open":
		stateValue = 2
	}
	m.CircuitBreakerState.WithLabelValues(provider, tenantID).Set(stateValue)
}

// RecordRetryAttempt records a retry attempt
func (m *Metrics) RecordRetryAttempt(provider, tenantID, reason string) {
	m.RetryAttempts.WithLabelValues(provider, tenantID, reason).Inc()
}

// RecordFallbackInvocation records a fallback chain invocation
func (m *Metrics) RecordFallbackInvocation(primaryProvider, fallbackProvider, tenantID string) {
	m.FallbackInvocations.WithLabelValues(primaryProvider, fallbackProvider, tenantID).Inc()
}

// RecordFallbackSuccess records a successful fallback execution
func (m *Metrics) RecordFallbackSuccess(fallbackProvider, tenantID string) {
	m.FallbackSuccess.WithLabelValues(fallbackProvider, tenantID).Inc()
}

// UpdateProviderHealth updates provider health score
func (m *Metrics) UpdateProviderHealth(provider, model, tenantID string, healthScore float64) {
	m.ProviderHealth.WithLabelValues(provider, model, tenantID).Set(healthScore)
}

// UpdateProviderSuccessRate updates provider success rate
func (m *Metrics) UpdateProviderSuccessRate(provider, model, tenantID string, successRate float64) {
	m.ProviderSuccessRate.WithLabelValues(provider, model, tenantID).Set(successRate)
}

// UpdateProviderP95Latency updates provider P95 latency
func (m *Metrics) UpdateProviderP95Latency(provider, model, tenantID string, latencyMs float64) {
	m.ProviderP95Latency.WithLabelValues(provider, model, tenantID).Set(latencyMs)
}

// RecordAPIKeyUsage records API key usage
func (m *Metrics) RecordAPIKeyUsage(provider, keyName, tenantID string) {
	m.APIKeyUsage.WithLabelValues(provider, keyName, tenantID).Inc()
}

// UpdateAPIKeyHealth updates API key health score
func (m *Metrics) UpdateAPIKeyHealth(provider, keyName, tenantID string, healthScore float64) {
	m.APIKeyHealth.WithLabelValues(provider, keyName, tenantID).Set(healthScore)
}

// RecordAPIKeyRateLimit records a rate limit hit
func (m *Metrics) RecordAPIKeyRateLimit(provider, keyName, tenantID string) {
	m.APIKeyRateLimits.WithLabelValues(provider, keyName, tenantID).Inc()
}
