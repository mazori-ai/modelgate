//go:generate go run github.com/99designs/gqlgen generate

package resolver

import (
	"context"

	"modelgate/internal/audit"
	"modelgate/internal/config"
	"modelgate/internal/domain"
	"modelgate/internal/gateway"
	"modelgate/internal/mcp"
	"modelgate/internal/storage/postgres"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	Config       *config.Config
	Gateway      *gateway.Service
	PGStore      *postgres.Store
	AuditService *audit.Service
	mcpGateway   *mcp.Gateway
}

// NewResolver creates a new resolver with all dependencies
func NewResolver(
	cfg *config.Config,
	gateway *gateway.Service,
	pgStore *postgres.Store,
) *Resolver {
	return &Resolver{
		Config:       cfg,
		Gateway:      gateway,
		PGStore:      pgStore,
		AuditService: audit.NewService(pgStore),
	}
}

// Context keys for authentication
type contextKey string

const (
	ContextKeyUser      contextKey = "user"
	ContextKeyUserEmail contextKey = "userEmail"
	ContextKeyTenant    contextKey = "tenant"
	ContextKeyToken     contextKey = "token"
	ContextKeyIsAdmin   contextKey = "isAdmin"
	ContextKeyIPAddress contextKey = "ipAddress"
	ContextKeyUserAgent contextKey = "userAgent"
)

// GetTenantFromContext retrieves the tenant slug from context
// Returns "default" for single-tenant mode
func GetTenantFromContext(ctx context.Context) string {
	if tenant, ok := ctx.Value(ContextKeyTenant).(string); ok && tenant != "" {
		return tenant
	}
	return "default" // Single-tenant mode
}

// GetUserFromContext retrieves the user ID from context
func GetUserFromContext(ctx context.Context) string {
	// Try *domain.User first (set by auth middleware)
	if user, ok := ctx.Value(ContextKeyUser).(*domain.User); ok && user != nil {
		return user.ID
	}
	// Fallback to string
	if user, ok := ctx.Value(ContextKeyUser).(string); ok {
		return user
	}
	return ""
}

// GetUserEmailFromContext retrieves the user email from context
func GetUserEmailFromContext(ctx context.Context) string {
	// Try to get from ContextKeyUserEmail first
	if email, ok := ctx.Value(ContextKeyUserEmail).(string); ok && email != "" {
		return email
	}
	// Fallback to *domain.User
	if user, ok := ctx.Value(ContextKeyUser).(*domain.User); ok && user != nil {
		return user.Email
	}
	return ""
}

// GetIPFromContext retrieves the IP address from context
func GetIPFromContext(ctx context.Context) string {
	if ip, ok := ctx.Value(ContextKeyIPAddress).(string); ok {
		return ip
	}
	return ""
}

// GetUserAgentFromContext retrieves the User-Agent from context
func GetUserAgentFromContext(ctx context.Context) string {
	if ua, ok := ctx.Value(ContextKeyUserAgent).(string); ok {
		return ua
	}
	return ""
}

// IsAdminFromContext checks if the current user is admin
func IsAdminFromContext(ctx context.Context) bool {
	if isAdmin, ok := ctx.Value(ContextKeyIsAdmin).(bool); ok {
		return isAdmin
	}
	return false
}

// SetMCPGateway sets the MCP gateway for the resolver
func (r *Resolver) SetMCPGateway(gw *mcp.Gateway) {
	r.mcpGateway = gw
}
