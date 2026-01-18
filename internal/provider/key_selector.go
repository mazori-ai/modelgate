package provider

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"modelgate/internal/crypto"
	"modelgate/internal/domain"
)

// Health penalty constants
const (
	HealthPenaltyDefault   = 0.05
	HealthPenaltyRateLimit = 0.02
	HealthPenaltyAuthError = 0.5
	HealthRecoveryRate     = 0.01
)

// Error types for failure classification
const (
	ErrorTypeRateLimit = "rate_limit"
	ErrorTypeAuthError = "auth_error"
	ErrorTypeTimeout   = "timeout"
	ErrorTypeServer    = "server_error"
)

// ProviderAPIKey represents a single API key for a provider
type ProviderAPIKey struct {
	ID       string
	Provider domain.Provider

	// Authentication credentials
	APIKeyEncrypted          string // Encrypted API key (stored value)
	APIKeyDecrypted          string // Decrypted API key (runtime value, never persisted)
	AccessKeyIDEncrypted     string // Encrypted AWS Access Key ID (for Bedrock IAM auth)
	AccessKeyIDDecrypted     string // Decrypted Access Key ID (runtime value)
	SecretAccessKeyEncrypted string // Encrypted AWS Secret Access Key (for Bedrock IAM auth)
	SecretAccessKeyDecrypted string // Decrypted Secret Access Key (runtime value)
	CredentialType           string // 'api_key', 'iam_credentials', or 'both'

	KeyPrefix          string // First 12 characters for display (API key or Access Key)
	Name               string
	Priority           int
	Enabled            bool
	HealthScore        float64
	SuccessCount       int
	FailureCount       int
	RateLimitRemaining *int
	RateLimitResetAt   *time.Time
	RequestCount       int64
	LastUsedAt         *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// TenantDBProvider is a function that returns the database for a given tenant slug
type TenantDBProvider func(tenantSlug string) (*sql.DB, error)

// KeySelector selects the best API key for a provider
type KeySelector struct {
	getTenantDB   TenantDBProvider
	encryption    *crypto.EncryptionService
	roundRobinIdx map[string]int // tenant:provider -> index
	mu            sync.RWMutex
}

// NewKeySelector creates a new key selector without encryption (for backwards compatibility)
func NewKeySelector(getTenantDB TenantDBProvider) *KeySelector {
	return &KeySelector{
		getTenantDB:   getTenantDB,
		roundRobinIdx: make(map[string]int),
	}
}

// NewKeySelectorWithEncryption creates a new key selector with encryption support
func NewKeySelectorWithEncryption(getTenantDB TenantDBProvider, encryption *crypto.EncryptionService) *KeySelector {
	return &KeySelector{
		getTenantDB:   getTenantDB,
		encryption:    encryption,
		roundRobinIdx: make(map[string]int),
	}
}

// SelectKey chooses the best API key for a provider
// tenantSlug is used to get the database connection (single-tenant mode)
func (ks *KeySelector) SelectKey(ctx context.Context, tenantSlug string, provider domain.Provider) (*ProviderAPIKey, error) {
	db, err := ks.getTenantDB(tenantSlug)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant database: %w", err)
	}

	// For AWS Bedrock, prioritize IAM credentials (true streaming) over API keys
	// For other providers, use standard priority ordering
	var orderClause string
	if provider == domain.ProviderBedrock {
		// Bedrock: prefer iam_credentials, then sort by priority and health
		orderClause = `
			ORDER BY
				CASE
					WHEN credential_type = 'iam_credentials' THEN 0
					WHEN credential_type = 'both' THEN 1
					WHEN credential_type = 'api_key' THEN 2
				END,
				priority ASC,
				health_score DESC
		`
	} else {
		// Other providers: standard priority ordering
		orderClause = `ORDER BY priority ASC, health_score DESC`
	}

	query := `
		SELECT id, api_key_encrypted, access_key_id_encrypted, secret_access_key_encrypted,
		       credential_type, name, priority, health_score,
		       rate_limit_remaining, rate_limit_reset_at
		FROM provider_api_keys
		WHERE provider = $1
		  AND enabled = true
	` + orderClause

	rows, err := db.QueryContext(ctx, query, provider)
	if err != nil {
		return nil, fmt.Errorf("failed to query API keys: %w", err)
	}
	defer rows.Close()

	var keys []*ProviderAPIKey
	for rows.Next() {
		var key ProviderAPIKey
		var apiKeyEncrypted sql.NullString
		var accessKeyIDEncrypted sql.NullString
		var secretAccessKeyEncrypted sql.NullString
		var rateLimitRemaining sql.NullInt32
		var rateLimitResetAt sql.NullTime

		key.Provider = provider

		err := rows.Scan(
			&key.ID, &apiKeyEncrypted, &accessKeyIDEncrypted, &secretAccessKeyEncrypted,
			&key.CredentialType, &key.Name, &key.Priority,
			&key.HealthScore, &rateLimitRemaining, &rateLimitResetAt,
		)
		if err != nil {
			continue
		}

		// Decrypt API key if present
		if apiKeyEncrypted.Valid && apiKeyEncrypted.String != "" {
			key.APIKeyEncrypted = apiKeyEncrypted.String
			if ks.encryption != nil {
				decrypted, err := ks.encryption.Decrypt(key.APIKeyEncrypted)
				if err != nil {
					key.APIKeyDecrypted = key.APIKeyEncrypted
				} else {
					key.APIKeyDecrypted = decrypted
				}
			} else {
				key.APIKeyDecrypted = key.APIKeyEncrypted
			}
		}

		// Decrypt Access Key ID if present
		if accessKeyIDEncrypted.Valid && accessKeyIDEncrypted.String != "" {
			key.AccessKeyIDEncrypted = accessKeyIDEncrypted.String
			if ks.encryption != nil {
				decrypted, err := ks.encryption.Decrypt(key.AccessKeyIDEncrypted)
				if err != nil {
					key.AccessKeyIDDecrypted = key.AccessKeyIDEncrypted
				} else {
					key.AccessKeyIDDecrypted = decrypted
				}
			} else {
				key.AccessKeyIDDecrypted = key.AccessKeyIDEncrypted
			}
		}

		// Decrypt Secret Access Key if present
		if secretAccessKeyEncrypted.Valid && secretAccessKeyEncrypted.String != "" {
			key.SecretAccessKeyEncrypted = secretAccessKeyEncrypted.String
			if ks.encryption != nil {
				decrypted, err := ks.encryption.Decrypt(key.SecretAccessKeyEncrypted)
				if err != nil {
					key.SecretAccessKeyDecrypted = key.SecretAccessKeyEncrypted
				} else {
					key.SecretAccessKeyDecrypted = decrypted
				}
			} else {
				key.SecretAccessKeyDecrypted = key.SecretAccessKeyEncrypted
			}
		}

		if rateLimitRemaining.Valid {
			val := int(rateLimitRemaining.Int32)
			key.RateLimitRemaining = &val
		}
		if rateLimitResetAt.Valid {
			key.RateLimitResetAt = &rateLimitResetAt.Time
		}

		keys = append(keys, &key)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating API keys: %w", err)
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("no enabled API keys for provider %s", provider)
	}

	// Filter out rate-limited keys
	availableKeys := make([]*ProviderAPIKey, 0, len(keys))
	for _, key := range keys {
		if !ks.isRateLimited(key) {
			availableKeys = append(availableKeys, key)
		}
	}

	if len(availableKeys) == 0 {
		// All keys are rate-limited, use the one with earliest reset time
		return ks.selectByResetTime(keys), nil
	}

	// Round-robin selection among available keys of same priority
	selectedKey := ks.roundRobinSelect(provider, availableKeys)

	// Mark key as used (pass tenant slug for database access)
	go ks.recordKeyUsage(context.Background(), tenantSlug, selectedKey.ID)

	return selectedKey, nil
}

// StoreKey stores a new API key with encryption
// tenantSlug is used to get the database connection (single-tenant mode)
func (ks *KeySelector) StoreKey(ctx context.Context, tenantSlug string, provider domain.Provider, apiKey, name string, priority int, accessKeyID, secretAccessKey string) (string, error) {
	db, err := ks.getTenantDB(tenantSlug)
	if err != nil {
		return "", fmt.Errorf("failed to get tenant database: %w", err)
	}

	// Validate: must have either API key OR both IAM credentials
	if apiKey == "" && (accessKeyID == "" || secretAccessKey == "") {
		return "", fmt.Errorf("must provide either API key or both Access Key ID and Secret Access Key")
	}

	// Determine credential type
	credentialType := "api_key"
	if accessKeyID != "" && secretAccessKey != "" {
		if apiKey != "" {
			credentialType = "both"
		} else {
			credentialType = "iam_credentials"
		}
	}

	// Encrypt credentials if encryption is enabled
	var encryptedAPIKey, encryptedAccessKeyID, encryptedSecretKey string

	if apiKey != "" {
		if ks.encryption != nil {
			encryptedAPIKey, err = ks.encryption.Encrypt(apiKey)
			if err != nil {
				return "", fmt.Errorf("failed to encrypt API key: %w", err)
			}
		} else {
			encryptedAPIKey = apiKey
		}
	}

	if accessKeyID != "" {
		if ks.encryption != nil {
			encryptedAccessKeyID, err = ks.encryption.Encrypt(accessKeyID)
			if err != nil {
				return "", fmt.Errorf("failed to encrypt Access Key ID: %w", err)
			}
		} else {
			encryptedAccessKeyID = accessKeyID
		}
	}

	if secretAccessKey != "" {
		if ks.encryption != nil {
			encryptedSecretKey, err = ks.encryption.Encrypt(secretAccessKey)
			if err != nil {
				return "", fmt.Errorf("failed to encrypt Secret Access Key: %w", err)
			}
		} else {
			encryptedSecretKey = secretAccessKey
		}
	}

	// Store credentials with NULL for empty values
	query := `
		INSERT INTO provider_api_keys (
			provider,
			api_key_encrypted,
			access_key_id_encrypted, secret_access_key_encrypted,
			credential_type,
			name, priority
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`

	var id string
	err = db.QueryRowContext(
		ctx, query,
		provider,
		nullIfEmpty(encryptedAPIKey),
		nullIfEmpty(encryptedAccessKeyID),
		nullIfEmpty(encryptedSecretKey),
		credentialType,
		name, priority,
	).Scan(&id)

	if err != nil {
		return "", fmt.Errorf("failed to store API key: %w", err)
	}

	return id, nil
}

// nullIfEmpty returns nil if string is empty, otherwise returns the string (for SQL NULL handling)
func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// isRateLimited checks if a key is currently rate limited
func (ks *KeySelector) isRateLimited(key *ProviderAPIKey) bool {
	if key.RateLimitRemaining == nil || key.RateLimitResetAt == nil {
		return false // No rate limit info
	}

	if *key.RateLimitRemaining <= 0 && time.Now().Before(*key.RateLimitResetAt) {
		return true // Rate limited
	}

	return false
}

// selectByResetTime selects key with earliest reset time
func (ks *KeySelector) selectByResetTime(keys []*ProviderAPIKey) *ProviderAPIKey {
	var earliest *ProviderAPIKey
	var earliestTime time.Time

	for _, key := range keys {
		if key.RateLimitResetAt != nil {
			if earliest == nil || key.RateLimitResetAt.Before(earliestTime) {
				earliest = key
				earliestTime = *key.RateLimitResetAt
			}
		}
	}

	if earliest != nil {
		return earliest
	}

	return keys[0] // Fallback to first key
}

// roundRobinSelect performs round-robin selection within priority groups
func (ks *KeySelector) roundRobinSelect(provider domain.Provider, keys []*ProviderAPIKey) *ProviderAPIKey {
	if len(keys) == 1 {
		return keys[0]
	}

	// Group by priority
	priorityGroups := make(map[int][]*ProviderAPIKey)
	for _, key := range keys {
		priorityGroups[key.Priority] = append(priorityGroups[key.Priority], key)
	}

	// Get highest priority group (lowest number)
	minPriority := int(^uint(0) >> 1) // Max int
	for priority := range priorityGroups {
		if priority < minPriority {
			minPriority = priority
		}
	}

	topPriorityKeys := priorityGroups[minPriority]

	// Round-robin within top priority group
	ks.mu.Lock()
	cacheKey := string(provider) // Single-tenant: just use provider name
	idx := ks.roundRobinIdx[cacheKey]
	selectedKey := topPriorityKeys[idx%len(topPriorityKeys)]
	ks.roundRobinIdx[cacheKey] = idx + 1
	ks.mu.Unlock()

	return selectedKey
}

// recordKeyUsage updates usage stats
func (ks *KeySelector) recordKeyUsage(ctx context.Context, tenantSlug, keyID string) {
	db, err := ks.getTenantDB(tenantSlug)
	if err != nil {
		return // Silently fail for background operation
	}
	query := `
		UPDATE provider_api_keys
		SET request_count = request_count + 1,
		    last_used_at = NOW()
		WHERE id = $1
	`
	_, _ = db.ExecContext(ctx, query, keyID)
}

// RecordSuccess updates health metrics after successful request
func (ks *KeySelector) RecordSuccess(ctx context.Context, tenantSlug, keyID string, rateLimitRemaining int, rateLimitResetAt time.Time) {
	db, err := ks.getTenantDB(tenantSlug)
	if err != nil {
		return // Silently fail
	}
	query := `
		UPDATE provider_api_keys
		SET success_count = success_count + 1,
		    health_score = LEAST(1.0, health_score + $2),
		    rate_limit_remaining = $3,
		    rate_limit_reset_at = $4,
		    updated_at = NOW()
		WHERE id = $1
	`
	_, _ = db.ExecContext(ctx, query, keyID, HealthRecoveryRate, rateLimitRemaining, rateLimitResetAt)
}

// RecordFailure updates health metrics after failed request
func (ks *KeySelector) RecordFailure(ctx context.Context, tenantSlug, keyID string, errorType string) {
	db, err := ks.getTenantDB(tenantSlug)
	if err != nil {
		return // Silently fail
	}
	// Decrease health score based on error type
	healthPenalty := getHealthPenalty(errorType)

	query := `
		UPDATE provider_api_keys
		SET failure_count = failure_count + 1,
		    health_score = GREATEST(0.0, health_score - $2),
		    updated_at = NOW()
		WHERE id = $1
	`
	_, _ = db.ExecContext(ctx, query, keyID, healthPenalty)
}

// getHealthPenalty returns the health penalty for a given error type
func getHealthPenalty(errorType string) float64 {
	switch errorType {
	case ErrorTypeRateLimit:
		return HealthPenaltyRateLimit // Smaller penalty for rate limits
	case ErrorTypeAuthError:
		return HealthPenaltyAuthError // Large penalty for auth errors
	default:
		return HealthPenaltyDefault
	}
}

// UpdateRateLimit updates rate limit info from response headers
func (ks *KeySelector) UpdateRateLimit(ctx context.Context, tenantSlug, keyID string, remaining int, resetAt time.Time) {
	db, err := ks.getTenantDB(tenantSlug)
	if err != nil {
		return // Silently fail
	}
	query := `
		UPDATE provider_api_keys
		SET rate_limit_remaining = $2,
		    rate_limit_reset_at = $3
		WHERE id = $1
	`
	_, _ = db.ExecContext(ctx, query, keyID, remaining, resetAt)
}

// DisableKey disables a key (e.g., after authentication failure)
func (ks *KeySelector) DisableKey(ctx context.Context, tenantSlug, keyID string, reason string) error {
	db, err := ks.getTenantDB(tenantSlug)
	if err != nil {
		return err
	}
	query := `
		UPDATE provider_api_keys
		SET enabled = false,
		    updated_at = NOW()
		WHERE id = $1
	`
	_, err = db.ExecContext(ctx, query, keyID)
	return err
}

// GetKeyHealth returns the current health score for a key
func (ks *KeySelector) GetKeyHealth(ctx context.Context, tenantSlug, keyID string) (float64, error) {
	db, err := ks.getTenantDB(tenantSlug)
	if err != nil {
		return 0, err
	}
	query := `SELECT health_score FROM provider_api_keys WHERE id = $1`
	var healthScore float64
	err = db.QueryRowContext(ctx, query, keyID).Scan(&healthScore)
	return healthScore, err
}

// ListKeys returns all API keys for a provider
func (ks *KeySelector) ListKeys(ctx context.Context, tenantSlug string, provider domain.Provider) ([]*ProviderAPIKey, error) {
	db, err := ks.getTenantDB(tenantSlug)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant database: %w", err)
	}

	query := `
		SELECT id, provider,
		       api_key_encrypted, access_key_id_encrypted, secret_access_key_encrypted,
		       credential_type,
		       name, priority, enabled,
		       health_score, success_count, failure_count, rate_limit_remaining,
		       rate_limit_reset_at, request_count, last_used_at, created_at, updated_at
		FROM provider_api_keys
		WHERE provider = $1
		ORDER BY priority ASC, health_score DESC
	`

	rows, err := db.QueryContext(ctx, query, string(provider))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*ProviderAPIKey
	for rows.Next() {
		key := &ProviderAPIKey{}
		var apiKeyEncrypted sql.NullString
		var accessKeyIDEncrypted sql.NullString
		var secretAccessKeyEncrypted sql.NullString
		var rateLimitRemaining sql.NullInt64
		var rateLimitResetAt sql.NullTime
		var lastUsedAt sql.NullTime
		var name sql.NullString

		err := rows.Scan(
			&key.ID, &key.Provider,
			&apiKeyEncrypted, &accessKeyIDEncrypted, &secretAccessKeyEncrypted,
			&key.CredentialType,
			&name,
			&key.Priority, &key.Enabled, &key.HealthScore, &key.SuccessCount,
			&key.FailureCount, &rateLimitRemaining, &rateLimitResetAt,
			&key.RequestCount, &lastUsedAt, &key.CreatedAt, &key.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Handle nullable fields
		if name.Valid {
			key.Name = name.String
		}
		if rateLimitRemaining.Valid {
			remaining := int(rateLimitRemaining.Int64)
			key.RateLimitRemaining = &remaining
		}
		if rateLimitResetAt.Valid {
			key.RateLimitResetAt = &rateLimitResetAt.Time
		}
		if lastUsedAt.Valid {
			key.LastUsedAt = &lastUsedAt.Time
		}

		// Decrypt API key if present
		if apiKeyEncrypted.Valid && apiKeyEncrypted.String != "" {
			key.APIKeyEncrypted = apiKeyEncrypted.String
			if ks.encryption != nil {
				decrypted, err := ks.encryption.Decrypt(key.APIKeyEncrypted)
				if err == nil {
					key.APIKeyDecrypted = decrypted
				} else {
					key.APIKeyDecrypted = key.APIKeyEncrypted // Fallback to encrypted value
				}
			} else {
				key.APIKeyDecrypted = key.APIKeyEncrypted
			}

			// Generate key prefix from API key (first 12 characters)
			if len(key.APIKeyDecrypted) > 12 {
				key.KeyPrefix = key.APIKeyDecrypted[:12]
			} else {
				key.KeyPrefix = key.APIKeyDecrypted
			}
		}

		// Decrypt Access Key ID if present
		if accessKeyIDEncrypted.Valid && accessKeyIDEncrypted.String != "" {
			key.AccessKeyIDEncrypted = accessKeyIDEncrypted.String
			if ks.encryption != nil {
				decrypted, err := ks.encryption.Decrypt(key.AccessKeyIDEncrypted)
				if err == nil {
					key.AccessKeyIDDecrypted = decrypted
				} else {
					key.AccessKeyIDDecrypted = key.AccessKeyIDEncrypted
				}
			} else {
				key.AccessKeyIDDecrypted = key.AccessKeyIDEncrypted
			}

			// If no API key prefix, use Access Key ID prefix
			if key.KeyPrefix == "" {
				if len(key.AccessKeyIDDecrypted) > 12 {
					key.KeyPrefix = key.AccessKeyIDDecrypted[:12]
				} else {
					key.KeyPrefix = key.AccessKeyIDDecrypted
				}
			}
		}

		// Decrypt Secret Access Key if present
		if secretAccessKeyEncrypted.Valid && secretAccessKeyEncrypted.String != "" {
			key.SecretAccessKeyEncrypted = secretAccessKeyEncrypted.String
			if ks.encryption != nil {
				decrypted, err := ks.encryption.Decrypt(key.SecretAccessKeyEncrypted)
				if err == nil {
					key.SecretAccessKeyDecrypted = decrypted
				} else {
					key.SecretAccessKeyDecrypted = key.SecretAccessKeyEncrypted
				}
			} else {
				key.SecretAccessKeyDecrypted = key.SecretAccessKeyEncrypted
			}
		}

		keys = append(keys, key)
	}

	return keys, rows.Err()
}

// UpdateKey updates the metadata of an API key (name, priority, enabled)
func (ks *KeySelector) UpdateKey(ctx context.Context, tenantSlug, keyID, name string, priority int, enabled bool) error {
	db, err := ks.getTenantDB(tenantSlug)
	if err != nil {
		return err
	}
	query := `
		UPDATE provider_api_keys
		SET name = $1,
		    priority = $2,
		    enabled = $3,
		    updated_at = NOW()
		WHERE id = $4
	`
	_, err = db.ExecContext(ctx, query, name, priority, enabled, keyID)
	return err
}

// DeleteKey deletes an API key
func (ks *KeySelector) DeleteKey(ctx context.Context, tenantSlug, keyID string) error {
	db, err := ks.getTenantDB(tenantSlug)
	if err != nil {
		return err
	}
	query := `DELETE FROM provider_api_keys WHERE id = $1`
	_, err = db.ExecContext(ctx, query, keyID)
	return err
}
