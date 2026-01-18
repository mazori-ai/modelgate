package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"modelgate/internal/domain"
)

// TenantRepositoryAdapter adapts the PostgreSQL store to implement domain.TenantRepository
// Note: In single-tenant mode, this provides compatibility for existing code
type TenantRepositoryAdapter struct {
	store *Store
}

// NewTenantRepositoryAdapter creates a new tenant repository adapter
func NewTenantRepositoryAdapter(store *Store) domain.TenantRepository {
	slog.Info("[ADAPTER] Creating TenantRepositoryAdapter")
	return &TenantRepositoryAdapter{store: store}
}

func (a *TenantRepositoryAdapter) Create(ctx context.Context, tenant *domain.Tenant) error {
	// Single tenant mode - not implemented
	return fmt.Errorf("multi-tenancy not supported in open source edition")
}

func (a *TenantRepositoryAdapter) Get(ctx context.Context, id string) (*domain.Tenant, error) {
	// Return a default tenant for single-tenant mode
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

func (a *TenantRepositoryAdapter) Update(ctx context.Context, tenant *domain.Tenant) error {
	return fmt.Errorf("multi-tenancy not supported in open source edition")
}

func (a *TenantRepositoryAdapter) Delete(ctx context.Context, id string) error {
	return fmt.Errorf("multi-tenancy not supported in open source edition")
}

func (a *TenantRepositoryAdapter) List(ctx context.Context, filter domain.TenantFilter) ([]*domain.Tenant, string, error) {
	// Return a single default tenant
	return []*domain.Tenant{{
		ID:     "default",
		Name:   "Default",
		Status: domain.TenantStatusActive,
		Tier:   domain.TenantTierFree,
		Metadata: map[string]string{
			"slug": "default",
		},
	}}, "", nil
}

func (a *TenantRepositoryAdapter) GetByAPIKey(ctx context.Context, keyHash string) (*domain.Tenant, *domain.APIKey, error) {
	slog.Info("[DEBUG] GetByAPIKey called", "keyHash", keyHash[:16]+"...")

	// Get API key from the single tenant store
	apiKeyWithRole, err := a.store.tenantStore.GetAPIKeyByHash(ctx, keyHash)
	if err != nil {
		slog.Error("GetAPIKeyByHash failed", "error", err)
		return nil, nil, fmt.Errorf("API key not found: %w", err)
	}

	// Check if API key was found
	if apiKeyWithRole == nil {
		slog.Error("GetAPIKeyByHash returned nil without error")
		return nil, nil, fmt.Errorf("API key not found")
	}

	// Default tenant for single-tenant mode
	tenant := &domain.Tenant{
		ID:     "default",
		Name:   "Default",
		Status: domain.TenantStatusActive,
		Tier:   domain.TenantTierFree,
		Metadata: map[string]string{
			"slug": "default",
		},
	}

	// Convert APIKeyWithRole to APIKey
	apiKey := &apiKeyWithRole.APIKey
	apiKey.RoleID = apiKeyWithRole.RoleID
	apiKey.RoleName = apiKeyWithRole.RoleName
	apiKey.GroupID = apiKeyWithRole.GroupID
	apiKey.GroupName = apiKeyWithRole.GroupName

	return tenant, apiKey, nil
}

// APIKeyRepositoryAdapter adapts the PostgreSQL store to implement domain.APIKeyRepository
type APIKeyRepositoryAdapter struct {
	store *Store
}

// NewAPIKeyRepositoryAdapter creates a new API key repository adapter
func NewAPIKeyRepositoryAdapter(store *Store) domain.APIKeyRepository {
	return &APIKeyRepositoryAdapter{store: store}
}

func (a *APIKeyRepositoryAdapter) Create(ctx context.Context, key *domain.APIKey) error {
	return fmt.Errorf("not implemented: use tenant store CreateAPIKey")
}

func (a *APIKeyRepositoryAdapter) Get(ctx context.Context, id string) (*domain.APIKey, error) {
	apiKeyWithRole, err := a.store.tenantStore.GetAPIKey(ctx, id)
	if err != nil {
		return nil, err
	}
	return &apiKeyWithRole.APIKey, nil
}

func (a *APIKeyRepositoryAdapter) Update(ctx context.Context, key *domain.APIKey) error {
	return fmt.Errorf("not implemented: use tenant store UpdateAPIKey")
}

func (a *APIKeyRepositoryAdapter) Delete(ctx context.Context, id string) error {
	return a.store.tenantStore.RevokeAPIKey(ctx, id, "deleted")
}

func (a *APIKeyRepositoryAdapter) GetByHash(ctx context.Context, hash string) (*domain.APIKey, error) {
	apiKeyWithRole, err := a.store.tenantStore.GetAPIKeyByHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	return &apiKeyWithRole.APIKey, nil
}

func (a *APIKeyRepositoryAdapter) List(ctx context.Context, tenantID string) ([]*domain.APIKey, error) {
	keys, err := a.store.tenantStore.ListAPIKeys(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*domain.APIKey, len(keys))
	for i, k := range keys {
		result[i] = &k.APIKey
	}
	return result, nil
}

func (a *APIKeyRepositoryAdapter) Revoke(ctx context.Context, id string) error {
	return a.store.tenantStore.RevokeAPIKey(ctx, id, "revoked")
}

func (a *APIKeyRepositoryAdapter) UpdateLastUsed(ctx context.Context, id string) error {
	return nil // Silent success
}

// UsageRepositoryAdapter adapts the PostgreSQL store to implement domain.UsageRepository
type UsageRepositoryAdapter struct {
	store *Store
}

// NewUsageRepositoryAdapter creates a new usage repository adapter
func NewUsageRepositoryAdapter(store *Store) domain.UsageRepository {
	slog.Info("[ADAPTER] Creating UsageRepositoryAdapter")
	return &UsageRepositoryAdapter{store: store}
}

// Record records usage to the database
func (a *UsageRepositoryAdapter) Record(ctx context.Context, record *domain.UsageRecord) error {
	return a.store.tenantStore.RecordUsage(ctx, record)
}

// GetStats retrieves usage statistics
func (a *UsageRepositoryAdapter) GetStats(ctx context.Context, tenantID string, startTime, endTime time.Time, granularity string) (*domain.UsageStats, error) {
	return a.store.tenantStore.GetUsageStats(ctx, startTime, endTime)
}

// GetTenantQuotas retrieves tenant quotas (not implemented)
func (a *UsageRepositoryAdapter) GetTenantQuotas(ctx context.Context, tenantID string) (*domain.TenantQuotas, error) {
	return &domain.TenantQuotas{}, nil
}

// UpdateTenantQuotas updates tenant quotas (not implemented)
func (a *UsageRepositoryAdapter) UpdateTenantQuotas(ctx context.Context, tenantID string, quotas *domain.TenantQuotas) error {
	return nil
}
