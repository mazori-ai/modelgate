// Package config provides configuration management for ModelGate.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"modelgate/internal/domain"

	"github.com/BurntSushi/toml"
)

// Config is the root configuration structure
type Config struct {
	Server    ServerConfig           `toml:"server"`
	Telemetry TelemetryConfig        `toml:"telemetry"`
	Database  DatabaseConfig         `toml:"database"`
	Providers ProvidersConfig        `toml:"providers"`
	Models    map[string]ModelConfig `toml:"models"`
	Aliases   map[string]string      `toml:"aliases"`
	Policies  PolicyConfig           `toml:"policies"`
	Security  SecurityConfig         `toml:"security"`
	Embedder  EmbedderConfig         `toml:"embedder"`
}

// EmbedderConfig contains embedder settings for semantic search
type EmbedderConfig struct {
	Type    string `toml:"type"`     // "openai", "ollama", "local"
	APIKey  string `toml:"api_key"`  // For OpenAI
	BaseURL string `toml:"base_url"` // For Ollama or custom endpoint
	Model   string `toml:"model"`    // Model name (e.g., "text-embedding-3-small", "nomic-embed-text")
}

// ServerConfig contains server settings
type ServerConfig struct {
	HTTPPort       int           `toml:"http_port"`    // Unified API port (OpenAI + GraphQL + MCP)
	MetricsPort    int           `toml:"metrics_port"` // Prometheus metrics (served on HTTPPort /metrics)
	BindAddress    string        `toml:"bind_address"`
	AuthToken      string        `toml:"auth_token"`
	ReadTimeout    time.Duration `toml:"read_timeout"`
	WriteTimeout   time.Duration `toml:"write_timeout"`
	MaxRequestSize int64         `toml:"max_request_size"`

	// Adaptive dispatcher configuration (Bifrost-style)
	MinWorkers         int     `toml:"min_workers"`          // Minimum workers (always running)
	MaxWorkers         int     `toml:"max_workers"`          // Maximum workers (scale up limit)
	MaxQueuedRequests  int     `toml:"max_queued_requests"`  // Max requests waiting in queue
	ScaleUpThreshold   float64 `toml:"scale_up_threshold"`   // Queue utilization % to scale up
	ScaleDownThreshold float64 `toml:"scale_down_threshold"` // Queue utilization % to scale down
}

// TelemetryConfig contains telemetry settings
type TelemetryConfig struct {
	Enabled           bool   `toml:"enabled"`
	ServiceName       string `toml:"service_name"`
	OTLPEndpoint      string `toml:"otlp_endpoint"`
	PrometheusEnabled bool   `toml:"prometheus_enabled"`
	PrometheusPort    int    `toml:"prometheus_port"`
	Traces            bool   `toml:"traces"`
	Metrics           bool   `toml:"metrics"`
	Logs              bool   `toml:"logs"`
	LogFormat         string `toml:"log_format"`
	LogLevel          string `toml:"log_level"`
}

// DatabaseConfig contains database settings
type DatabaseConfig struct {
	Driver     string        `toml:"driver"` // "postgres", "sqlite", "memory"
	DSN        string        `toml:"dsn"`    // Full DSN (alternative to individual fields)
	Host       string        `toml:"host"`
	Port       int           `toml:"port"`
	User       string        `toml:"user"`
	Password   string        `toml:"password"`
	Database   string        `toml:"database"` // Database name
	SSLMode    string        `toml:"ssl_mode"`
	MaxConns   int           `toml:"max_conns"`
	MaxIdle    int           `toml:"max_idle"`
	ConnMaxAge time.Duration `toml:"conn_max_age"`
}

// GetDSN returns the DSN for the database
func (d *DatabaseConfig) GetDSN() string {
	if d.DSN != "" {
		return d.DSN
	}
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.Database, d.SSLMode)
}

// GetBaseDSN returns DSN without database name (for creating databases)
func (d *DatabaseConfig) GetBaseDSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.SSLMode)
}

// ProvidersConfig contains provider-specific settings
type ProvidersConfig struct {
	Gemini    GeminiConfig    `toml:"gemini"`
	Anthropic AnthropicConfig `toml:"anthropic"`
	OpenAI    OpenAIConfig    `toml:"openai"`
	Bedrock   BedrockConfig   `toml:"bedrock"`
	Ollama    OllamaConfig    `toml:"ollama"`
}

// GeminiConfig contains Gemini-specific settings
type GeminiConfig struct {
	APIKey  string `toml:"api_key"`
	Enabled bool   `toml:"enabled"`
}

// AnthropicConfig contains Anthropic-specific settings
type AnthropicConfig struct {
	APIKey  string `toml:"api_key"`
	Enabled bool   `toml:"enabled"`
}

// OpenAIConfig contains OpenAI-specific settings
type OpenAIConfig struct {
	APIKey  string `toml:"api_key"`
	BaseURL string `toml:"base_url"`
	OrgID   string `toml:"org_id"`
	Enabled bool   `toml:"enabled"`
}

// BedrockConfig contains AWS Bedrock-specific settings
type BedrockConfig struct {
	// Long-Term API Key authentication (preferred, like LLM Gateway)
	APIKey       string `toml:"api_key"`
	RegionPrefix string `toml:"region_prefix"` // "us.", "eu.", "global."
	ModelsURL    string `toml:"models_url"`    // Custom models API endpoint

	// Legacy IAM-style authentication (for backward compatibility)
	Region          string `toml:"region"`
	AccessKeyID     string `toml:"access_key_id"`
	SecretAccessKey string `toml:"secret_access_key"`
	Profile         string `toml:"profile"`

	Enabled bool `toml:"enabled"`
}

// OllamaConfig contains Ollama-specific settings
type OllamaConfig struct {
	BaseURL string `toml:"base_url"`
	Enabled bool   `toml:"enabled"`
}

// ModelConfig contains model metadata
type ModelConfig struct {
	Name              string  `toml:"name"`
	Provider          string  `toml:"provider"`
	SupportsTools     bool    `toml:"supports_tools"`
	SupportsReasoning bool    `toml:"supports_reasoning"`
	ContextLimit      uint32  `toml:"context_limit"`
	OutputLimit       uint32  `toml:"output_limit"`
	InputCostPer1M    float64 `toml:"input_cost_per_1m"`
	OutputCostPer1M   float64 `toml:"output_cost_per_1m"`
	Enabled           bool    `toml:"enabled"`
}

// PolicyConfig contains default policy settings
type PolicyConfig struct {
	DefaultPromptFilters []PromptFilterConfig `toml:"default_prompt_filters"`
	OutlierDetection     OutlierConfig        `toml:"outlier_detection"`
	EnableSafetyChecks   bool                 `toml:"enable_safety_checks"`
}

// PromptFilterConfig configures prompt filtering
type PromptFilterConfig struct {
	Category string   `toml:"category"`
	Patterns []string `toml:"patterns"`
	Action   string   `toml:"action"` // "block", "warn", "log"
	Severity string   `toml:"severity"`
}

// OutlierConfig configures outlier detection
type OutlierConfig struct {
	Enabled            bool    `toml:"enabled"`
	MaxPromptLength    int     `toml:"max_prompt_length"`
	AnomalyThreshold   float64 `toml:"anomaly_threshold"`
	InjectionDetection bool    `toml:"injection_detection"`
}

// SecurityConfig contains security settings
type SecurityConfig struct {
	EnableRateLimiting  bool   `toml:"enable_rate_limiting"`
	DefaultRPM          int    `toml:"default_rpm"`
	DefaultTPM          int    `toml:"default_tpm"`
	APIKeyHashAlgorithm string `toml:"api_key_hash_algorithm"`
	JWTSecret           string `toml:"jwt_secret"`
	AdminAPIKey         string `toml:"admin_api_key"`
}

// Default returns a default configuration
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			HTTPPort:       8080,
			MetricsPort:    9090,
			BindAddress:    "0.0.0.0",
			ReadTimeout:    5 * time.Minute,  // Increased for long streaming requests
			WriteTimeout:   10 * time.Minute, // Increased for long streaming responses
			MaxRequestSize: 10 * 1024 * 1024, // 10MB
		},
		Telemetry: TelemetryConfig{
			Enabled:     true,
			ServiceName: "modelgate",
			Traces:      true,
			Metrics:     true,
			Logs:        true,
			LogFormat:   "pretty",
			LogLevel:    "info",
		},
		Database: DatabaseConfig{
			Driver:     "postgres",
			Host:       "localhost",
			Port:       5432,
			User:       "postgres",
			Password:   "postgres",
			Database:   "modelgate",
			SSLMode:    "disable",
			MaxConns:   20,
			MaxIdle:    5,
			ConnMaxAge: 30 * time.Minute,
		},
		Providers: ProvidersConfig{
			Ollama: OllamaConfig{
				BaseURL: "http://localhost:11434",
				Enabled: true,
			},
		},
		Models:  make(map[string]ModelConfig),
		Aliases: make(map[string]string),
		Policies: PolicyConfig{
			EnableSafetyChecks: true,
			OutlierDetection: OutlierConfig{
				Enabled:            true,
				MaxPromptLength:    100000,
				AnomalyThreshold:   0.8,
				InjectionDetection: true,
			},
		},
		Security: SecurityConfig{
			EnableRateLimiting:  true,
			DefaultRPM:          60,
			DefaultTPM:          100000,
			APIKeyHashAlgorithm: "sha256",
		},
	}
}

// Load loads configuration from a file
func Load(path string) (*Config, error) {
	// Start with defaults
	cfg := Default()

	// Parse TOML
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		// If file doesn't exist, return defaults
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	// Substitute environment variables
	cfg.substituteEnvVars()

	return cfg, nil
}

// LoadOrDefault loads config from file or returns defaults
func LoadOrDefault(path string) *Config {
	if path == "" {
		return Default()
	}

	cfg, err := Load(path)
	if err != nil {
		fmt.Printf("Warning: Failed to load config from %s: %v\n", path, err)
		return Default()
	}

	return cfg
}

// substituteEnvVars substitutes ${VAR} patterns with environment variables
// and applies direct MODELGATE_* environment variable overrides
func (c *Config) substituteEnvVars() {
	// Expand ${VAR} patterns in config values
	c.Server.AuthToken = expandEnv(c.Server.AuthToken)

	c.Providers.Gemini.APIKey = expandEnv(c.Providers.Gemini.APIKey)
	c.Providers.Anthropic.APIKey = expandEnv(c.Providers.Anthropic.APIKey)
	c.Providers.OpenAI.APIKey = expandEnv(c.Providers.OpenAI.APIKey)
	c.Providers.Bedrock.AccessKeyID = expandEnv(c.Providers.Bedrock.AccessKeyID)
	c.Providers.Bedrock.SecretAccessKey = expandEnv(c.Providers.Bedrock.SecretAccessKey)

	c.Database.DSN = expandEnv(c.Database.DSN)
	c.Database.Host = expandEnv(c.Database.Host)
	c.Database.User = expandEnv(c.Database.User)
	c.Database.Password = expandEnv(c.Database.Password)
	c.Security.JWTSecret = expandEnv(c.Security.JWTSecret)
	c.Security.AdminAPIKey = expandEnv(c.Security.AdminAPIKey)

	// Direct environment variable overrides for Docker deployment
	// Database configuration
	if v := os.Getenv("MODELGATE_DB_DRIVER"); v != "" {
		c.Database.Driver = v
	}
	if v := os.Getenv("MODELGATE_DB_HOST"); v != "" {
		c.Database.Host = v
	}
	if v := os.Getenv("MODELGATE_DB_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.Database.Port = port
		}
	}
	if v := os.Getenv("MODELGATE_DB_USER"); v != "" {
		c.Database.User = v
	}
	if v := os.Getenv("MODELGATE_DB_PASSWORD"); v != "" {
		c.Database.Password = v
	}
	if v := os.Getenv("MODELGATE_DB_NAME"); v != "" {
		c.Database.Database = v
	}
	if v := os.Getenv("MODELGATE_DB_SSL_MODE"); v != "" {
		c.Database.SSLMode = v
	}

	// Server configuration
	if v := os.Getenv("MODELGATE_HTTP_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.Server.HTTPPort = port
		}
	}
	if v := os.Getenv("MODELGATE_METRICS_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.Telemetry.PrometheusPort = port
		}
	}

	// Embedder configuration
	if v := os.Getenv("MODELGATE_EMBEDDER_TYPE"); v != "" {
		c.Embedder.Type = v
	}
	if v := os.Getenv("MODELGATE_EMBEDDER_URL"); v != "" {
		c.Embedder.BaseURL = v
	}
	if v := os.Getenv("MODELGATE_EMBEDDER_MODEL"); v != "" {
		c.Embedder.Model = v
	}
	if v := os.Getenv("MODELGATE_OPENAI_API_KEY"); v != "" {
		c.Embedder.APIKey = v
	}
}

// expandEnv expands ${VAR} or $VAR patterns
func expandEnv(s string) string {
	if s == "" {
		return s
	}
	return os.ExpandEnv(s)
}

// GetModel returns model configuration by ID
func (c *Config) GetModel(modelID string) (*ModelConfig, bool) {
	m, ok := c.Models[modelID]
	return &m, ok
}

// ResolveModel resolves a model alias to actual model ID
func (c *Config) ResolveModel(modelID string) string {
	if resolved, ok := c.Aliases[modelID]; ok {
		return resolved
	}
	return modelID
}

// IsModelAvailable checks if a model is available and enabled
func (c *Config) IsModelAvailable(modelID string) bool {
	m, ok := c.Models[modelID]
	return ok && m.Enabled
}

// GetProviderForModel determines the provider for a model
func (c *Config) GetProviderForModel(modelID string) (domain.Provider, bool) {
	// First check if it's in the model config
	if m, ok := c.Models[modelID]; ok {
		return domain.ParseProvider(m.Provider)
	}

	// Otherwise parse from the model ID format "provider/model"
	parts := strings.SplitN(modelID, "/", 2)
	if len(parts) != 2 {
		return "", false
	}

	return domain.ParseProvider(parts[0])
}

// ExtractModelID extracts the model ID from "provider/model" format
func ExtractModelID(fullModel string) string {
	parts := strings.SplitN(fullModel, "/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return fullModel
}

// ToModelInfo converts ModelConfig to domain.ModelInfo
func (m *ModelConfig) ToModelInfo(id string) domain.ModelInfo {
	provider, _ := domain.ParseProvider(m.Provider)
	return domain.ModelInfo{
		ID:                id,
		Name:              m.Name,
		Provider:          provider,
		SupportsTools:     m.SupportsTools,
		SupportsReasoning: m.SupportsReasoning,
		ContextLimit:      m.ContextLimit,
		OutputLimit:       m.OutputLimit,
		InputCostPer1M:    m.InputCostPer1M,
		OutputCostPer1M:   m.OutputCostPer1M,
		Enabled:           m.Enabled,
	}
}

// CalculateCost calculates cost for token usage
func (m *ModelConfig) CalculateCost(inputTokens, outputTokens int64) float64 {
	inputCost := (float64(inputTokens) / 1_000_000.0) * m.InputCostPer1M
	outputCost := (float64(outputTokens) / 1_000_000.0) * m.OutputCostPer1M
	return inputCost + outputCost
}
