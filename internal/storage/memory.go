// Package storage provides data storage implementations.
package storage

import (
	"context"
	"fmt"
	"sync"
	"time"

	"modelgate/internal/domain"
)

// MemoryStore provides in-memory storage for development/testing
type MemoryStore struct {
	tenants         map[string]*domain.Tenant
	apiKeys         map[string]*domain.APIKey
	policies        map[string]*domain.Policy
	usage           []*domain.UsageRecord
	providerConfigs map[string]*domain.TenantProviderConfig
	mu              sync.RWMutex
}

// NewMemoryStore creates a new in-memory store
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		tenants:         make(map[string]*domain.Tenant),
		apiKeys:         make(map[string]*domain.APIKey),
		policies:        make(map[string]*domain.Policy),
		usage:           []*domain.UsageRecord{},
		providerConfigs: make(map[string]*domain.TenantProviderConfig),
	}
}

// =============================================================================
// TenantRepository Implementation
// =============================================================================

func (s *MemoryStore) Create(ctx context.Context, tenant *domain.Tenant) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tenants[tenant.ID]; exists {
		return fmt.Errorf("tenant %s already exists", tenant.ID)
	}

	s.tenants[tenant.ID] = tenant
	return nil
}

func (s *MemoryStore) Get(ctx context.Context, id string) (*domain.Tenant, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tenant, ok := s.tenants[id]
	if !ok {
		return nil, fmt.Errorf("tenant %s not found", id)
	}

	return tenant, nil
}

func (s *MemoryStore) Update(ctx context.Context, tenant *domain.Tenant) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tenants[tenant.ID]; !exists {
		return fmt.Errorf("tenant %s not found", tenant.ID)
	}

	s.tenants[tenant.ID] = tenant
	return nil
}

func (s *MemoryStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.tenants, id)
	return nil
}

func (s *MemoryStore) List(ctx context.Context, filter domain.TenantFilter) ([]*domain.Tenant, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*domain.Tenant
	for _, t := range s.tenants {
		if filter.Status != "" && t.Status != filter.Status {
			continue
		}
		if filter.Tier != "" && t.Tier != filter.Tier {
			continue
		}
		result = append(result, t)
	}

	return result, "", nil
}

func (s *MemoryStore) GetByAPIKey(ctx context.Context, keyHash string) (*domain.Tenant, *domain.APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, key := range s.apiKeys {
		if key.KeyHash == keyHash && !key.Revoked {
			// Single-tenant mode: return default tenant
			tenant, ok := s.tenants["default"]
			if ok {
				return tenant, key, nil
			}
		}
	}

	return nil, nil, fmt.Errorf("API key not found")
}

// =============================================================================
// APIKeyRepository Implementation
// =============================================================================

func (s *MemoryStore) CreateAPIKey(ctx context.Context, key *domain.APIKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.apiKeys[key.ID] = key
	return nil
}

func (s *MemoryStore) GetAPIKey(ctx context.Context, id string) (*domain.APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key, ok := s.apiKeys[id]
	if !ok {
		return nil, fmt.Errorf("API key %s not found", id)
	}

	return key, nil
}

func (s *MemoryStore) GetByHash(ctx context.Context, hash string) (*domain.APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, key := range s.apiKeys {
		if key.KeyHash == hash {
			return key, nil
		}
	}

	return nil, fmt.Errorf("API key not found")
}

func (s *MemoryStore) ListAPIKeys(ctx context.Context, tenantID string) ([]*domain.APIKey, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Single-tenant mode: return all API keys
	var result []*domain.APIKey
	for _, key := range s.apiKeys {
		result = append(result, key)
	}

	return result, nil
}

func (s *MemoryStore) Revoke(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key, ok := s.apiKeys[id]
	if !ok {
		return fmt.Errorf("API key %s not found", id)
	}

	key.Revoked = true
	return nil
}

func (s *MemoryStore) UpdateLastUsed(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key, ok := s.apiKeys[id]
	if !ok {
		return fmt.Errorf("API key %s not found", id)
	}

	now := time.Now()
	key.LastUsedAt = &now
	return nil
}

func (s *MemoryStore) UpdateAPIKeyRoleGroup(ctx context.Context, id, name, roleID, groupID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key, ok := s.apiKeys[id]
	if !ok {
		return fmt.Errorf("API key %s not found", id)
	}

	if name != "" {
		key.Name = name
	}
	key.RoleID = roleID
	key.GroupID = groupID
	key.UpdatedAt = time.Now()
	return nil
}

// =============================================================================
// PolicyRepository Implementation
// =============================================================================

func (s *MemoryStore) CreatePolicy(ctx context.Context, policy *domain.Policy) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.policies[policy.ID] = policy
	return nil
}

func (s *MemoryStore) GetPolicy(ctx context.Context, id string) (*domain.Policy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	policy, ok := s.policies[id]
	if !ok {
		return nil, fmt.Errorf("policy %s not found", id)
	}

	return policy, nil
}

func (s *MemoryStore) UpdatePolicy(ctx context.Context, policy *domain.Policy) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.policies[policy.ID] = policy
	return nil
}

func (s *MemoryStore) DeletePolicy(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.policies, id)
	return nil
}

func (s *MemoryStore) ListPolicies(ctx context.Context, filter domain.PolicyFilter) ([]*domain.Policy, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*domain.Policy
	for _, p := range s.policies {
		if filter.Type != "" && p.Type != filter.Type {
			continue
		}
		result = append(result, p)
	}

	return result, "", nil
}

func (s *MemoryStore) GetByTenant(ctx context.Context, tenantID string) ([]*domain.Policy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tenant, ok := s.tenants[tenantID]
	if !ok {
		return nil, fmt.Errorf("tenant %s not found", tenantID)
	}

	var result []*domain.Policy
	for _, policyID := range tenant.PolicyIDs {
		if policy, ok := s.policies[policyID]; ok {
			result = append(result, policy)
		}
	}

	return result, nil
}

// =============================================================================
// UsageRepository Implementation
// =============================================================================

func (s *MemoryStore) Record(ctx context.Context, record *domain.UsageRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.usage = append(s.usage, record)

	// Update tenant quotas (single-tenant mode)
	if tenant, ok := s.tenants["default"]; ok {
		tenant.Quotas.RequestsUsed++
		tenant.Quotas.TokensUsed += record.TotalTokens
		tenant.Quotas.CostUsedUSD += record.CostUSD
	}

	return nil
}

func (s *MemoryStore) GetStats(ctx context.Context, tenantID string, startTime, endTime time.Time, granularity string) (*domain.UsageStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &domain.UsageStats{
		UsageByModel:    make(map[string]domain.ModelUsage),
		UsageByProvider: make(map[domain.Provider]domain.ProviderUsage),
	}

	// Single-tenant mode: process all records
	for _, record := range s.usage {
		if !startTime.IsZero() && record.Timestamp.Before(startTime) {
			continue
		}
		if !endTime.IsZero() && record.Timestamp.After(endTime) {
			continue
		}

		stats.TotalRequests++
		stats.TotalTokens += record.TotalTokens
		stats.TotalCostUSD += record.CostUSD

		// By model
		mu := stats.UsageByModel[record.Model]
		mu.ModelID = record.Model
		mu.Requests++
		mu.InputTokens += record.InputTokens
		mu.OutputTokens += record.OutputTokens
		mu.CostUSD += record.CostUSD
		stats.UsageByModel[record.Model] = mu

		// By provider
		pu := stats.UsageByProvider[record.Provider]
		pu.Provider = record.Provider
		pu.Requests++
		pu.Tokens += record.TotalTokens
		pu.CostUSD += record.CostUSD
		stats.UsageByProvider[record.Provider] = pu
	}

	return stats, nil
}

func (s *MemoryStore) GetTenantQuotas(ctx context.Context, tenantID string) (*domain.TenantQuotas, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tenant, ok := s.tenants[tenantID]
	if !ok {
		return nil, fmt.Errorf("tenant %s not found", tenantID)
	}

	return &tenant.Quotas, nil
}

func (s *MemoryStore) UpdateTenantQuotas(ctx context.Context, tenantID string, quotas *domain.TenantQuotas) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tenant, ok := s.tenants[tenantID]
	if !ok {
		return fmt.Errorf("tenant %s not found", tenantID)
	}

	tenant.Quotas = *quotas
	return nil
}

// =============================================================================
// Adapter implementations for interfaces
// =============================================================================

// TenantAdapter adapts MemoryStore to TenantRepository
type TenantAdapter struct {
	store *MemoryStore
}

func (s *MemoryStore) TenantRepository() domain.TenantRepository {
	return &TenantAdapter{store: s}
}

func (a *TenantAdapter) Create(ctx context.Context, tenant *domain.Tenant) error {
	return a.store.Create(ctx, tenant)
}

func (a *TenantAdapter) Get(ctx context.Context, id string) (*domain.Tenant, error) {
	return a.store.Get(ctx, id)
}

func (a *TenantAdapter) Update(ctx context.Context, tenant *domain.Tenant) error {
	return a.store.Update(ctx, tenant)
}

func (a *TenantAdapter) Delete(ctx context.Context, id string) error {
	return a.store.Delete(ctx, id)
}

func (a *TenantAdapter) List(ctx context.Context, filter domain.TenantFilter) ([]*domain.Tenant, string, error) {
	return a.store.List(ctx, filter)
}

func (a *TenantAdapter) GetByAPIKey(ctx context.Context, keyHash string) (*domain.Tenant, *domain.APIKey, error) {
	return a.store.GetByAPIKey(ctx, keyHash)
}

// APIKeyAdapter adapts MemoryStore to APIKeyRepository
type APIKeyAdapter struct {
	store *MemoryStore
}

func (s *MemoryStore) APIKeyRepository() domain.APIKeyRepository {
	return &APIKeyAdapter{store: s}
}

func (a *APIKeyAdapter) Create(ctx context.Context, key *domain.APIKey) error {
	return a.store.CreateAPIKey(ctx, key)
}

func (a *APIKeyAdapter) Get(ctx context.Context, id string) (*domain.APIKey, error) {
	return a.store.GetAPIKey(ctx, id)
}

func (a *APIKeyAdapter) GetByHash(ctx context.Context, hash string) (*domain.APIKey, error) {
	return a.store.GetByHash(ctx, hash)
}

func (a *APIKeyAdapter) List(ctx context.Context, tenantID string) ([]*domain.APIKey, error) {
	return a.store.ListAPIKeys(ctx, tenantID)
}

func (a *APIKeyAdapter) Update(ctx context.Context, key *domain.APIKey) error {
	return a.store.UpdateAPIKeyRoleGroup(ctx, key.ID, key.Name, key.RoleID, key.GroupID)
}

func (a *APIKeyAdapter) Revoke(ctx context.Context, id string) error {
	return a.store.Revoke(ctx, id)
}

func (a *APIKeyAdapter) UpdateLastUsed(ctx context.Context, id string) error {
	return a.store.UpdateLastUsed(ctx, id)
}

// PolicyAdapter adapts MemoryStore to PolicyRepository
type PolicyAdapter struct {
	store *MemoryStore
}

func (s *MemoryStore) PolicyRepository() domain.PolicyRepository {
	return &PolicyAdapter{store: s}
}

func (a *PolicyAdapter) Create(ctx context.Context, policy *domain.Policy) error {
	return a.store.CreatePolicy(ctx, policy)
}

func (a *PolicyAdapter) Get(ctx context.Context, id string) (*domain.Policy, error) {
	return a.store.GetPolicy(ctx, id)
}

func (a *PolicyAdapter) Update(ctx context.Context, policy *domain.Policy) error {
	return a.store.UpdatePolicy(ctx, policy)
}

func (a *PolicyAdapter) Delete(ctx context.Context, id string) error {
	return a.store.DeletePolicy(ctx, id)
}

func (a *PolicyAdapter) List(ctx context.Context, filter domain.PolicyFilter) ([]*domain.Policy, string, error) {
	return a.store.ListPolicies(ctx, filter)
}

func (a *PolicyAdapter) GetByTenant(ctx context.Context, tenantID string) ([]*domain.Policy, error) {
	return a.store.GetByTenant(ctx, tenantID)
}

// UsageAdapter adapts MemoryStore to UsageRepository
type UsageAdapter struct {
	store *MemoryStore
}

func (s *MemoryStore) UsageRepository() domain.UsageRepository {
	return &UsageAdapter{store: s}
}

func (a *UsageAdapter) Record(ctx context.Context, record *domain.UsageRecord) error {
	return a.store.Record(ctx, record)
}

func (a *UsageAdapter) GetStats(ctx context.Context, tenantID string, startTime, endTime time.Time, granularity string) (*domain.UsageStats, error) {
	return a.store.GetStats(ctx, tenantID, startTime, endTime, granularity)
}

func (a *UsageAdapter) GetTenantQuotas(ctx context.Context, tenantID string) (*domain.TenantQuotas, error) {
	return a.store.GetTenantQuotas(ctx, tenantID)
}

func (a *UsageAdapter) UpdateTenantQuotas(ctx context.Context, tenantID string, quotas *domain.TenantQuotas) error {
	return a.store.UpdateTenantQuotas(ctx, tenantID, quotas)
}

// =============================================================================
// TenantProviderConfigRepository Implementation
// =============================================================================

func (s *MemoryStore) GetProviderConfig(ctx context.Context, tenantID string) (*domain.TenantProviderConfig, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Single-tenant mode: use "default" key
	config, ok := s.providerConfigs["default"]
	if !ok {
		// Return empty config
		return &domain.TenantProviderConfig{
			Providers: make(map[domain.Provider]domain.ProviderConfig),
			Models:    make(map[string]domain.TenantModelConfig),
		}, nil
	}

	return config, nil
}

func (s *MemoryStore) SaveProviderConfig(ctx context.Context, config *domain.TenantProviderConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Single-tenant mode: use "default" key
	s.providerConfigs["default"] = config
	return nil
}

func (s *MemoryStore) DeleteProviderConfig(ctx context.Context, tenantID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.providerConfigs, tenantID)
	return nil
}

// ProviderConfigAdapter adapts MemoryStore to TenantProviderConfigRepository
type ProviderConfigAdapter struct {
	store *MemoryStore
}

func (s *MemoryStore) ProviderConfigRepository() domain.TenantProviderConfigRepository {
	return &ProviderConfigAdapter{store: s}
}

func (a *ProviderConfigAdapter) Get(ctx context.Context, tenantID string) (*domain.TenantProviderConfig, error) {
	return a.store.GetProviderConfig(ctx, tenantID)
}

func (a *ProviderConfigAdapter) Save(ctx context.Context, config *domain.TenantProviderConfig) error {
	return a.store.SaveProviderConfig(ctx, config)
}

func (a *ProviderConfigAdapter) Delete(ctx context.Context, tenantID string) error {
	return a.store.DeleteProviderConfig(ctx, tenantID)
}

// =============================================================================
// Direct Access Methods (for analytics)
// =============================================================================

// RLock acquires a read lock on the store
func (s *MemoryStore) RLock() {
	s.mu.RLock()
}

// RUnlock releases the read lock on the store
func (s *MemoryStore) RUnlock() {
	s.mu.RUnlock()
}

// Usage returns the usage records (must be called with RLock held)
func (s *MemoryStore) Usage() []*domain.UsageRecord {
	return s.usage
}

