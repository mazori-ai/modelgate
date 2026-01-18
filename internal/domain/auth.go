// Package domain provides authentication and user types.
package domain

import (
	"context"
	"time"
)

// =============================================================================
// User Types
// =============================================================================

// UserRole represents a user's role
type UserRole string

const (
	UserRoleSuperAdmin  UserRole = "super_admin"  // Platform admin - manages all tenants
	UserRoleTenantAdmin UserRole = "tenant_admin" // Tenant admin - manages their tenant
	UserRoleTenantUser  UserRole = "tenant_user"  // Regular tenant user
)

// User represents a user in the system
type User struct {
	ID           string            `json:"id"`
	Email        string            `json:"email"`
	Name         string            `json:"name"`
	PasswordHash string            `json:"-"`
	Role         UserRole          `json:"role"`
	Status       UserStatus        `json:"status"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	LastLoginAt  time.Time         `json:"last_login_at,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// UserStatus represents user account status
type UserStatus string

const (
	UserStatusActive    UserStatus = "active"
	UserStatusSuspended UserStatus = "suspended"
	UserStatusPending   UserStatus = "pending"
)

// =============================================================================
// Session Types
// =============================================================================

// Session represents an authenticated session
type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Token     string    `json:"-"`
	UserAgent string    `json:"user_agent,omitempty"`
	IPAddress string    `json:"ip_address,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// =============================================================================
// Auth Configuration Types
// =============================================================================

// AuthConfig represents authentication configuration
type AuthConfig struct {
	ID               string           `json:"id"`
	Type             AuthType         `json:"type"`
	Enabled          bool             `json:"enabled"`
	OIDCConfig       *OIDCConfig      `json:"oidc_config,omitempty"`
	SAMLConfig       *SAMLConfig      `json:"saml_config,omitempty"`
	LocalAuthEnabled bool             `json:"local_auth_enabled"`
	MFARequired      bool             `json:"mfa_required"`
	SessionDuration  time.Duration    `json:"session_duration"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
}

// AuthType represents the authentication method type
type AuthType string

const (
	AuthTypeLocal    AuthType = "local"
	AuthTypeOIDC     AuthType = "oidc"
	AuthTypeSAML     AuthType = "saml"
	AuthTypeKeycloak AuthType = "keycloak"
)

// OIDCConfig contains OIDC/OAuth2 configuration
type OIDCConfig struct {
	ProviderURL  string   `json:"provider_url"`
	ClientID     string   `json:"client_id"`
	ClientSecret string   `json:"-"`
	RedirectURL  string   `json:"redirect_url"`
	Scopes       []string `json:"scopes"`
}

// SAMLConfig contains SAML configuration
type SAMLConfig struct {
	EntityID         string `json:"entity_id"`
	SSOURL           string `json:"sso_url"`
	Certificate      string `json:"certificate"`
	AllowIDPInitiated bool  `json:"allow_idp_initiated"`
}

// =============================================================================
// Telemetry Configuration Types
// =============================================================================

// TelemetryConfig represents telemetry configuration
type TelemetryConfig struct {
	ID                  string            `json:"id"`
	PrometheusEnabled   bool              `json:"prometheus_enabled"`
	PrometheusEndpoint  string            `json:"prometheus_endpoint,omitempty"`
	OTLPEnabled         bool              `json:"otlp_enabled"`
	OTLPEndpoint        string            `json:"otlp_endpoint,omitempty"`
	LogLevel            string            `json:"log_level"`
	ExportUsageData     bool              `json:"export_usage_data"`
	WebhookURL          string            `json:"webhook_url,omitempty"`
	Metadata            map[string]string `json:"metadata,omitempty"`
	CreatedAt           time.Time         `json:"created_at"`
	UpdatedAt           time.Time         `json:"updated_at"`
}

// =============================================================================
// Repository Interfaces
// =============================================================================

// UserRepository is the interface for user storage
type UserRepository interface {
	Create(ctx context.Context, user *User) error
	Get(ctx context.Context, id string) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByTenantAndEmail(ctx context.Context, tenantID, email string) (*User, error)
	Update(ctx context.Context, user *User) error
	Delete(ctx context.Context, id string) error
	ListByTenant(ctx context.Context, tenantID string) ([]*User, error)
}

// SessionRepository is the interface for session storage
type SessionRepository interface {
	Create(ctx context.Context, session *Session) error
	Get(ctx context.Context, id string) (*Session, error)
	GetByToken(ctx context.Context, token string) (*Session, error)
	Delete(ctx context.Context, id string) error
	DeleteByUser(ctx context.Context, userID string) error
	DeleteExpired(ctx context.Context) error
}

// AuthConfigRepository is the interface for auth config storage
type AuthConfigRepository interface {
	Create(ctx context.Context, config *AuthConfig) error
	Get(ctx context.Context, tenantID string) (*AuthConfig, error)
	Update(ctx context.Context, config *AuthConfig) error
	Delete(ctx context.Context, tenantID string) error
}

// TelemetryConfigRepository is the interface for telemetry config storage
type TelemetryConfigRepository interface {
	Create(ctx context.Context, config *TelemetryConfig) error
	Get(ctx context.Context, tenantID string) (*TelemetryConfig, error)
	Update(ctx context.Context, config *TelemetryConfig) error
	Delete(ctx context.Context, tenantID string) error
}

