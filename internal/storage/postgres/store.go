// Package postgres provides PostgreSQL storage implementation for ModelGate.
// This is the single-tenant open source edition.
package postgres

import (
	"context"
	"log"
	"time"

	"modelgate/internal/config"
	"modelgate/internal/domain"
)

// Store is the main PostgreSQL store that manages all storage operations
type Store struct {
	config      *config.DatabaseConfig
	db          *DB
	tenantStore *TenantStore
}

// NewStore creates a new PostgreSQL store
func NewStore(cfg *config.DatabaseConfig) (*Store, error) {
	store := &Store{
		config: cfg,
	}

	// Initialize database
	db, err := InitDB(cfg)
	if err != nil {
		return nil, err
	}
	store.db = db

	// Create store for all operations
	store.tenantStore = NewTenantStore(db, "default")

	log.Println("PostgreSQL store initialized successfully")
	return store, nil
}

// Close closes all database connections
func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// DB returns the database connection for direct access
func (s *Store) DB() *DB {
	return s.db
}

// Config returns the database configuration
func (s *Store) Config() *config.DatabaseConfig {
	return s.config
}

// TenantStore returns the underlying tenant store for direct access
// This is the preferred method for single-tenant mode
func (s *Store) TenantStore() *TenantStore {
	return s.tenantStore
}

// GetTenantStore returns the tenant store (deprecated - use TenantStore() instead)
// Kept for compatibility during migration
func (s *Store) GetTenantStore(tenantSlug string) (*TenantStore, error) {
	return s.tenantStore, nil
}

// TenantRepository returns a repository adapter for tenant operations
func (s *Store) TenantRepository() domain.TenantRepository {
	return NewTenantRepositoryAdapter(s)
}

// APIKeyRepository returns a repository adapter for API key operations
func (s *Store) APIKeyRepository() domain.APIKeyRepository {
	return NewAPIKeyRepositoryAdapter(s)
}

// UsageRepository returns a repository adapter for usage tracking operations
func (s *Store) UsageRepository() domain.UsageRepository {
	return NewUsageRepositoryAdapter(s)
}

// =============================================================================
// User Operations
// =============================================================================

// CreateUser creates a user in the database
func (s *Store) CreateUser(ctx context.Context, email, password, name, role, createdBy, createdByEmail string) (*TenantUser, error) {
	return s.tenantStore.CreateUser(ctx, email, password, name, role, createdBy, createdByEmail)
}

// ValidateUserPassword validates a user's password
func (s *Store) ValidateUserPassword(ctx context.Context, email, password string) (*TenantUser, error) {
	return s.tenantStore.ValidateUserPassword(ctx, email, password)
}

// CreateSession creates a session for a user
func (s *Store) CreateSession(ctx context.Context, userID string, duration time.Duration) (*TenantSession, string, error) {
	return s.tenantStore.CreateSession(ctx, userID, duration)
}

// GetSessionByToken gets a session by token
func (s *Store) GetSessionByToken(ctx context.Context, token string) (*TenantSession, *TenantUser, error) {
	return s.tenantStore.GetSessionByToken(ctx, token)
}

// =============================================================================
// Role Operations
// =============================================================================

// ListRoles lists all roles
func (s *Store) ListRoles(ctx context.Context) ([]*domain.Role, error) {
	return s.tenantStore.ListRoles(ctx)
}

// CreateRole creates a new role
func (s *Store) CreateRole(ctx context.Context, role *domain.Role) error {
	return s.tenantStore.CreateRole(ctx, role)
}

// GetRole gets a role by ID
func (s *Store) GetRole(ctx context.Context, roleID string) (*domain.Role, error) {
	return s.tenantStore.GetRole(ctx, roleID)
}

// UpdateRole updates a role
func (s *Store) UpdateRole(ctx context.Context, role *domain.Role) error {
	return s.tenantStore.UpdateRole(ctx, role)
}

// DeleteRole deletes a role
func (s *Store) DeleteRole(ctx context.Context, roleID string) error {
	return s.tenantStore.DeleteRole(ctx, roleID)
}

// GetRolePolicy gets a role's policy
func (s *Store) GetRolePolicy(ctx context.Context, roleID string) (*domain.RolePolicy, error) {
	return s.tenantStore.GetRolePolicy(ctx, roleID)
}

// CreateRolePolicy creates or updates a role's policy
func (s *Store) CreateRolePolicy(ctx context.Context, policy *domain.RolePolicy) error {
	return s.tenantStore.CreateRolePolicy(ctx, policy)
}

// UpdateRolePolicy updates a role's policy
func (s *Store) UpdateRolePolicy(ctx context.Context, policy *domain.RolePolicy) error {
	return s.tenantStore.UpdateRolePolicy(ctx, policy)
}

// GetDefaultRole gets the default role
func (s *Store) GetDefaultRole(ctx context.Context) (*domain.Role, error) {
	return s.tenantStore.GetDefaultRole(ctx)
}

// =============================================================================
// Group Operations
// =============================================================================

// ListGroups lists all groups
func (s *Store) ListGroups(ctx context.Context) ([]*domain.Group, error) {
	return s.tenantStore.ListGroups(ctx)
}

// CreateGroup creates a new group
func (s *Store) CreateGroup(ctx context.Context, group *domain.Group) error {
	return s.tenantStore.CreateGroup(ctx, group)
}

// GetGroup gets a group by ID
func (s *Store) GetGroup(ctx context.Context, groupID string) (*domain.Group, error) {
	return s.tenantStore.GetGroup(ctx, groupID)
}

// UpdateGroup updates a group
func (s *Store) UpdateGroup(ctx context.Context, group *domain.Group) error {
	return s.tenantStore.UpdateGroup(ctx, group)
}

// DeleteGroup deletes a group
func (s *Store) DeleteGroup(ctx context.Context, groupID string) error {
	return s.tenantStore.DeleteGroup(ctx, groupID)
}

// =============================================================================
// API Key Operations
// =============================================================================

// CreateAPIKey creates a new API key with role or group assignment
func (s *Store) CreateAPIKey(ctx context.Context, name, roleID, groupID string, scopes []string) (*domain.APIKey, string, error) {
	return s.tenantStore.CreateAPIKey(ctx, name, roleID, groupID, scopes, nil)
}

// GetAPIKey gets an API key by ID
func (s *Store) GetAPIKey(ctx context.Context, keyID string) (*domain.APIKeyWithRole, error) {
	return s.tenantStore.GetAPIKey(ctx, keyID)
}

// ListAPIKeys lists all API keys
func (s *Store) ListAPIKeys(ctx context.Context) ([]*domain.APIKeyWithRole, error) {
	return s.tenantStore.ListAPIKeys(ctx)
}

// GetAPIKeyByHash gets an API key by its hash
func (s *Store) GetAPIKeyByHash(ctx context.Context, keyHash string) (*domain.APIKeyWithRole, error) {
	return s.tenantStore.GetAPIKeyByHash(ctx, keyHash)
}

// UpdateAPIKey updates an API key's name, role, or group assignment
func (s *Store) UpdateAPIKey(ctx context.Context, keyID, name, roleID, groupID string) error {
	return s.tenantStore.UpdateAPIKey(ctx, keyID, name, roleID, groupID)
}

// RevokeAPIKey revokes an API key
func (s *Store) RevokeAPIKey(ctx context.Context, keyID, reason string) error {
	return s.tenantStore.RevokeAPIKey(ctx, keyID, reason)
}

// =============================================================================
// Provider Operations
// =============================================================================

// ListProviderConfigs lists all provider configurations
func (s *Store) ListProviderConfigs(ctx context.Context) ([]*domain.ProviderConfig, error) {
	return s.tenantStore.ListProviderConfigs(ctx)
}

// SaveProviderConfig saves a provider configuration
func (s *Store) SaveProviderConfig(ctx context.Context, config *domain.ProviderConfig) error {
	return s.tenantStore.SaveProviderConfig(ctx, config)
}

// GetProviderConfig gets a provider configuration
func (s *Store) GetProviderConfig(ctx context.Context, provider domain.Provider) (*domain.ProviderConfig, error) {
	return s.tenantStore.GetProviderConfig(ctx, provider)
}

// =============================================================================
// Tool Operations
// =============================================================================

// ListTools lists all available tools
func (s *Store) ListTools(ctx context.Context) ([]*domain.AvailableTool, error) {
	return s.tenantStore.ListTools(ctx)
}

// CreateTool creates a new available tool
func (s *Store) CreateTool(ctx context.Context, tool *domain.AvailableTool) error {
	return s.tenantStore.CreateTool(ctx, tool)
}

// UpdateTool updates an available tool
func (s *Store) UpdateTool(ctx context.Context, tool *domain.AvailableTool) error {
	return s.tenantStore.UpdateTool(ctx, tool)
}

// DeleteTool deletes an available tool
func (s *Store) DeleteTool(ctx context.Context, toolID string) error {
	return s.tenantStore.DeleteTool(ctx, toolID)
}

// =============================================================================
// Usage Operations
// =============================================================================

// RecordUsage records API usage
func (s *Store) RecordUsage(ctx context.Context, record *domain.UsageRecord) error {
	return s.tenantStore.RecordUsage(ctx, record)
}

// GetUsageStats gets usage statistics
func (s *Store) GetUsageStats(ctx context.Context, startTime, endTime time.Time) (*domain.UsageStats, error) {
	return s.tenantStore.GetUsageStats(ctx, startTime, endTime)
}

// ListUsageRecords lists usage records with filters
func (s *Store) ListUsageRecords(ctx context.Context, startTime, endTime time.Time, model, status, apiKeyID string, limit int) ([]*domain.UsageRecord, error) {
	return s.tenantStore.ListUsageRecords(ctx, startTime, endTime, model, status, apiKeyID, limit)
}

// GetUsageRecord gets a single usage record
func (s *Store) GetUsageRecord(ctx context.Context, id string) (*domain.UsageRecord, error) {
	return s.tenantStore.GetUsageRecord(ctx, id)
}

// GetUsageStatsByModel gets usage statistics grouped by model
func (s *Store) GetUsageStatsByModel(ctx context.Context, startTime, endTime time.Time) (map[string]*domain.ModelUsageStats, error) {
	return s.tenantStore.GetUsageStatsByModel(ctx, startTime, endTime)
}

// GetUsageStatsByProvider gets usage statistics grouped by provider
func (s *Store) GetUsageStatsByProvider(ctx context.Context, startTime, endTime time.Time) (map[string]*domain.ProviderUsageStats, error) {
	return s.tenantStore.GetUsageStatsByProvider(ctx, startTime, endTime)
}

// GetUsageStatsByAPIKey gets usage statistics grouped by API key
func (s *Store) GetUsageStatsByAPIKey(ctx context.Context, startTime, endTime time.Time) (map[string]*domain.APIKeyUsageStats, error) {
	return s.tenantStore.GetUsageStatsByAPIKey(ctx, startTime, endTime)
}

// GetUsageTimeSeries gets usage over time for charts
func (s *Store) GetUsageTimeSeries(ctx context.Context, startTime, endTime time.Time, interval string) ([]*domain.UsageTimePoint, error) {
	return s.tenantStore.GetUsageTimeSeries(ctx, startTime, endTime, interval)
}

// =============================================================================
// Model Operations
// =============================================================================

// SaveModelConfig creates or updates a model configuration
func (s *Store) SaveModelConfig(ctx context.Context, config *domain.ModelConfig) error {
	return s.tenantStore.SaveModelConfig(ctx, config)
}

// GetModelConfig gets a model configuration
func (s *Store) GetModelConfig(ctx context.Context, modelID string) (*domain.ModelConfig, error) {
	return s.tenantStore.GetModelConfig(ctx, modelID)
}

// ListModelConfigs lists all model configurations
func (s *Store) ListModelConfigs(ctx context.Context) ([]*domain.ModelConfig, error) {
	return s.tenantStore.ListModelConfigs(ctx)
}

// DeleteModelConfig deletes a model configuration
func (s *Store) DeleteModelConfig(ctx context.Context, modelID string) error {
	return s.tenantStore.DeleteModelConfig(ctx, modelID)
}

// =============================================================================
// Telemetry Operations
// =============================================================================

// SaveTelemetryConfig creates or updates telemetry configuration
func (s *Store) SaveTelemetryConfig(ctx context.Context, config *domain.TelemetryConfig) error {
	return s.tenantStore.SaveTelemetryConfig(ctx, config)
}

// GetTelemetryConfig gets the telemetry configuration
func (s *Store) GetTelemetryConfig(ctx context.Context) (*domain.TelemetryConfig, error) {
	return s.tenantStore.GetTelemetryConfig(ctx)
}

// =============================================================================
// Available Models Operations
// =============================================================================

// SaveAvailableModels saves models fetched from a provider API
func (s *Store) SaveAvailableModels(ctx context.Context, provider string, models []domain.ModelInfo) error {
	return s.tenantStore.SaveAvailableModels(ctx, provider, models)
}

// ListAvailableModels returns all available models (optionally filtered by provider)
func (s *Store) ListAvailableModels(ctx context.Context, provider string) ([]*AvailableModel, error) {
	return s.tenantStore.ListAvailableModels(ctx, provider)
}

// DeleteProviderModels deletes all models for a provider
func (s *Store) DeleteProviderModels(ctx context.Context, provider string) error {
	return s.tenantStore.DeleteProviderModels(ctx, provider)
}

// GetProviderModelsURL gets the custom models URL for a provider
func (s *Store) GetProviderModelsURL(ctx context.Context, provider string) (string, error) {
	return s.tenantStore.GetProviderModelsURL(ctx, provider)
}

// UpdateProviderModelsURL updates the custom models URL for a provider
func (s *Store) UpdateProviderModelsURL(ctx context.Context, provider, modelsURL string) error {
	return s.tenantStore.UpdateProviderModelsURL(ctx, provider, modelsURL)
}

// =============================================================================
// Default Tenant (for compatibility with code expecting tenant operations)
// =============================================================================

// GetTenant returns the default tenant configuration
func (s *Store) GetTenant(ctx context.Context, id string) (*domain.Tenant, error) {
	return &domain.Tenant{
		ID:     "default",
		Name:   "Default",
		Status: domain.TenantStatusActive,
		Tier:   domain.TenantTierFree,
		Metadata: map[string]string{
			"slug": "default",
		},
	}, nil
}

// GetTenantBySlug returns the default tenant configuration
func (s *Store) GetTenantBySlug(ctx context.Context, slug string) (*domain.Tenant, error) {
	return s.GetTenant(ctx, "default")
}
