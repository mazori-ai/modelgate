package audit

import (
	"context"
	"log/slog"
	"net/http"

	"modelgate/internal/domain"
	"modelgate/internal/storage/postgres"
)

// Service handles audit logging
type Service struct {
	pgStore *postgres.Store
}

// NewService creates a new audit service
func NewService(pgStore *postgres.Store) *Service {
	return &Service{pgStore: pgStore}
}

// Actor represents the user performing an action
type Actor struct {
	ID    string
	Email string
	Type  string // "user", "admin", "system"
}

// LogEntry represents an audit log entry to be created
type LogEntry struct {
	TenantSlug   string
	Action       domain.AuditAction
	ResourceType domain.AuditResourceType
	ResourceID   string
	ResourceName string
	Actor        Actor
	IPAddress    string
	UserAgent    string
	Details      map[string]any
	OldValue     map[string]any
	NewValue     map[string]any
	Status       string
	ErrorMessage string
}

// Log creates an audit log entry
func (s *Service) Log(ctx context.Context, entry LogEntry) error {
	if entry.TenantSlug == "" {
		slog.Warn("Audit log skipped: no tenant slug")
		return nil
	}

	tenantStore, err := s.pgStore.GetTenantStore(entry.TenantSlug)
	if err != nil {
		slog.Error("Failed to get tenant store for audit", "error", err, "tenant", entry.TenantSlug)
		return err
	}

	log := &domain.AuditLog{
		Action:       entry.Action,
		ResourceType: entry.ResourceType,
		ResourceID:   entry.ResourceID,
		ResourceName: entry.ResourceName,
		ActorID:      entry.Actor.ID,
		ActorEmail:   entry.Actor.Email,
		ActorType:    entry.Actor.Type,
		IPAddress:    entry.IPAddress,
		UserAgent:    entry.UserAgent,
		Details:      entry.Details,
		OldValue:     entry.OldValue,
		NewValue:     entry.NewValue,
		Status:       entry.Status,
		ErrorMessage: entry.ErrorMessage,
	}

	if log.Status == "" {
		log.Status = "success"
	}
	if log.ActorType == "" {
		log.ActorType = "user"
	}

	err = tenantStore.CreateAuditLog(ctx, log)
	if err != nil {
		slog.Error("Failed to create audit log", "error", err)
	}
	return err
}

// LogSuccess creates a success audit log
func (s *Service) LogSuccess(ctx context.Context, entry LogEntry) error {
	entry.Status = "success"
	return s.Log(ctx, entry)
}

// LogFailure creates a failure audit log
func (s *Service) LogFailure(ctx context.Context, entry LogEntry, errMsg string) error {
	entry.Status = "failure"
	entry.ErrorMessage = errMsg
	return s.Log(ctx, entry)
}

// ExtractRequestInfo extracts IP and User-Agent from HTTP request
func ExtractRequestInfo(r *http.Request) (ipAddress, userAgent string) {
	if r == nil {
		return "", ""
	}

	// Try X-Forwarded-For first for proxied requests
	ipAddress = r.Header.Get("X-Forwarded-For")
	if ipAddress == "" {
		ipAddress = r.Header.Get("X-Real-IP")
	}
	if ipAddress == "" {
		ipAddress = r.RemoteAddr
	}

	userAgent = r.Header.Get("User-Agent")
	return
}

// Context keys for audit info
type contextKey string

const (
	ContextKeyActor     contextKey = "audit_actor"
	ContextKeyIPAddress contextKey = "audit_ip"
	ContextKeyUserAgent contextKey = "audit_ua"
)

// WithActor adds actor to context
func WithActor(ctx context.Context, actor Actor) context.Context {
	return context.WithValue(ctx, ContextKeyActor, actor)
}

// GetActorFromContext retrieves actor from context
func GetActorFromContext(ctx context.Context) Actor {
	if actor, ok := ctx.Value(ContextKeyActor).(Actor); ok {
		return actor
	}
	return Actor{ID: "unknown", Email: "unknown", Type: "system"}
}

// WithRequestInfo adds request info to context
func WithRequestInfo(ctx context.Context, r *http.Request) context.Context {
	ip, ua := ExtractRequestInfo(r)
	ctx = context.WithValue(ctx, ContextKeyIPAddress, ip)
	ctx = context.WithValue(ctx, ContextKeyUserAgent, ua)
	return ctx
}

// GetIPFromContext retrieves IP from context
func GetIPFromContext(ctx context.Context) string {
	if ip, ok := ctx.Value(ContextKeyIPAddress).(string); ok {
		return ip
	}
	return ""
}

// GetUserAgentFromContext retrieves User-Agent from context
func GetUserAgentFromContext(ctx context.Context) string {
	if ua, ok := ctx.Value(ContextKeyUserAgent).(string); ok {
		return ua
	}
	return ""
}
