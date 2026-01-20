// Package postgres provides PostgreSQL storage implementation for ModelGate.
package postgres

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"modelgate/internal/domain"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// TenantStore handles tenant database operations
type TenantStore struct {
	db         *DB
	tenantSlug string
}

// NewTenantStore creates a new tenant store
func NewTenantStore(db *DB, tenantSlug string) *TenantStore {
	return &TenantStore{db: db, tenantSlug: tenantSlug}
}

// DB returns the underlying database connection
func (s *TenantStore) DB() *DB {
	return s.db
}

// =============================================================================
// User Operations
// =============================================================================

// TenantUser represents a user within a tenant
type TenantUser struct {
	ID             string         `json:"id"`
	Email          string         `json:"email"`
	Name           string         `json:"name"`
	Role           string         `json:"role"`
	IsActive       bool           `json:"is_active"`
	LastLoginAt    *time.Time     `json:"last_login_at,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	CreatedBy      string         `json:"created_by,omitempty"`
	CreatedByEmail string         `json:"created_by_email,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// CreateUser creates a new tenant user
func (s *TenantStore) CreateUser(ctx context.Context, email, password, name, role, createdBy, createdByEmail string) (*TenantUser, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	id := uuid.New().String()
	now := time.Now()

	query := `
		INSERT INTO users (id, email, password_hash, name, role, is_active, created_by, created_by_email, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, true, $6, $7, $8, $9)
		RETURNING id, email, name, role, is_active, created_by, created_by_email, created_at, updated_at
	`

	var user TenantUser
	var createdByVal, createdByEmailVal sql.NullString
	err = s.db.QueryRowContext(ctx, query, id, email, string(hashedPassword), name, role,
		sql.NullString{String: createdBy, Valid: createdBy != ""},
		sql.NullString{String: createdByEmail, Valid: createdByEmail != ""},
		now, now).Scan(
		&user.ID, &user.Email, &user.Name, &user.Role, &user.IsActive,
		&createdByVal, &createdByEmailVal, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}

	user.CreatedBy = createdByVal.String
	user.CreatedByEmail = createdByEmailVal.String
	return &user, nil
}

// GetUser gets a user by ID
func (s *TenantStore) GetUser(ctx context.Context, id string) (*TenantUser, error) {
	query := `
		SELECT id, email, name, role, is_active, last_login_at, metadata, created_by, created_by_email, created_at, updated_at
		FROM users WHERE id = $1
	`

	var user TenantUser
	var metadataJSON []byte
	var createdBy, createdByEmail sql.NullString

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.Email, &user.Name, &user.Role, &user.IsActive,
		&user.LastLoginAt, &metadataJSON, &createdBy, &createdByEmail, &user.CreatedAt, &user.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal(metadataJSON, &user.Metadata)
	user.CreatedBy = createdBy.String
	user.CreatedByEmail = createdByEmail.String
	return &user, nil
}

// UpdateUser updates a user
func (s *TenantStore) UpdateUser(ctx context.Context, id string, name *string, role *string, isActive *bool) (*TenantUser, error) {
	// Build dynamic update query
	updates := []string{}
	args := []interface{}{}
	argIdx := 1

	if name != nil {
		updates = append(updates, fmt.Sprintf("name = $%d", argIdx))
		args = append(args, *name)
		argIdx++
	}
	if role != nil {
		updates = append(updates, fmt.Sprintf("role = $%d", argIdx))
		args = append(args, *role)
		argIdx++
	}
	if isActive != nil {
		updates = append(updates, fmt.Sprintf("is_active = $%d", argIdx))
		args = append(args, *isActive)
		argIdx++
	}

	if len(updates) == 0 {
		return s.GetUser(ctx, id)
	}

	updates = append(updates, fmt.Sprintf("updated_at = $%d", argIdx))
	args = append(args, time.Now())
	argIdx++

	args = append(args, id)
	query := fmt.Sprintf(`UPDATE users SET %s WHERE id = $%d`, strings.Join(updates, ", "), argIdx)

	_, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	return s.GetUser(ctx, id)
}

// DeleteUser deletes a user
func (s *TenantStore) DeleteUser(ctx context.Context, id string) error {
	query := `DELETE FROM users WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, id)
	return err
}

// GetUserByEmail gets a user by email
func (s *TenantStore) GetUserByEmail(ctx context.Context, email string) (*TenantUser, string, error) {
	query := `
		SELECT id, email, password_hash, name, role, is_active, last_login_at, metadata, created_by, created_by_email, created_at, updated_at
		FROM users WHERE email = $1
	`

	var user TenantUser
	var passwordHash string
	var metadataJSON []byte
	var createdBy, createdByEmail sql.NullString

	err := s.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID, &user.Email, &passwordHash, &user.Name, &user.Role, &user.IsActive,
		&user.LastLoginAt, &metadataJSON, &createdBy, &createdByEmail, &user.CreatedAt, &user.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}

	json.Unmarshal(metadataJSON, &user.Metadata)
	user.CreatedBy = createdBy.String
	user.CreatedByEmail = createdByEmail.String
	return &user, passwordHash, nil
}

// ValidateUserPassword validates a user's password
func (s *TenantStore) ValidateUserPassword(ctx context.Context, email, password string) (*TenantUser, error) {
	user, passwordHash, err := s.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, fmt.Errorf("user not found")
	}
	if !user.IsActive {
		return nil, fmt.Errorf("user is not active")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid password")
	}

	// Update last login
	s.db.ExecContext(ctx, "UPDATE users SET last_login_at = $1 WHERE id = $2", time.Now(), user.ID)

	return user, nil
}

// ListUsers lists all users
func (s *TenantStore) ListUsers(ctx context.Context) ([]*TenantUser, error) {
	query := `
		SELECT id, email, name, role, is_active, last_login_at, metadata, created_by, created_by_email, created_at, updated_at
		FROM users ORDER BY created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*TenantUser
	for rows.Next() {
		var user TenantUser
		var metadataJSON []byte
		var createdBy, createdByEmail sql.NullString

		err := rows.Scan(&user.ID, &user.Email, &user.Name, &user.Role, &user.IsActive,
			&user.LastLoginAt, &metadataJSON, &createdBy, &createdByEmail, &user.CreatedAt, &user.UpdatedAt)
		if err != nil {
			return nil, err
		}

		json.Unmarshal(metadataJSON, &user.Metadata)
		user.CreatedBy = createdBy.String
		user.CreatedByEmail = createdByEmail.String
		users = append(users, &user)
	}

	return users, nil
}

// =============================================================================
// Session Operations
// =============================================================================

// TenantSession represents a user session
type TenantSession struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateSession creates a new session
func (s *TenantStore) CreateSession(ctx context.Context, userID string, duration time.Duration) (*TenantSession, string, error) {
	token := uuid.New().String() + "-" + uuid.New().String()
	tokenHash := hashAPIKey(token)

	id := uuid.New().String()
	now := time.Now()
	expiresAt := now.Add(duration)

	query := `
		INSERT INTO sessions (id, user_id, token_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := s.db.ExecContext(ctx, query, id, userID, tokenHash, expiresAt, now)
	if err != nil {
		return nil, "", err
	}

	session := &TenantSession{
		ID:        id,
		UserID:    userID,
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}

	return session, token, nil
}

// GetSessionByToken gets a session by token
func (s *TenantStore) GetSessionByToken(ctx context.Context, token string) (*TenantSession, *TenantUser, error) {
	tokenHash := hashAPIKey(token)

	query := `
		SELECT s.id, s.user_id, s.expires_at, s.created_at,
		       u.id, u.email, u.name, u.role, u.is_active, u.last_login_at, u.metadata, u.created_at, u.updated_at
		FROM sessions s
		JOIN users u ON s.user_id = u.id
		WHERE s.token_hash = $1 AND s.expires_at > $2 AND u.is_active = true
	`

	var session TenantSession
	var user TenantUser
	var metadataJSON []byte

	err := s.db.QueryRowContext(ctx, query, tokenHash, time.Now()).Scan(
		&session.ID, &session.UserID, &session.ExpiresAt, &session.CreatedAt,
		&user.ID, &user.Email, &user.Name, &user.Role, &user.IsActive,
		&user.LastLoginAt, &metadataJSON, &user.CreatedAt, &user.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}

	json.Unmarshal(metadataJSON, &user.Metadata)
	return &session, &user, nil
}

// DeleteSession deletes a session
func (s *TenantStore) DeleteSession(ctx context.Context, token string) error {
	tokenHash := hashAPIKey(token)
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE token_hash = $1", tokenHash)
	return err
}

// =============================================================================
// Role Operations
// =============================================================================

// CreateRole creates a new role
func (s *TenantStore) CreateRole(ctx context.Context, role *domain.Role) error {
	if role.ID == "" {
		role.ID = uuid.New().String()
	}

	permissionsJSON, _ := json.Marshal(role.Permissions)
	now := time.Now()
	role.CreatedAt = now
	role.UpdatedAt = now

	query := `
		INSERT INTO roles (id, name, description, permissions, is_default, created_by, created_by_email, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := s.db.ExecContext(ctx, query, role.ID, role.Name, role.Description,
		permissionsJSON, role.IsDefault,
		sql.NullString{String: role.CreatedBy, Valid: role.CreatedBy != ""},
		sql.NullString{String: role.CreatedByEmail, Valid: role.CreatedByEmail != ""},
		now, now)
	return err
}

// GetRole gets a role by ID
func (s *TenantStore) GetRole(ctx context.Context, id string) (*domain.Role, error) {
	query := `
		SELECT id, name, description, permissions, is_default, created_by, created_by_email, created_at, updated_at
		FROM roles WHERE id = $1
	`

	var role domain.Role
	var permissionsJSON []byte
	var createdBy, createdByEmail sql.NullString

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&role.ID, &role.Name, &role.Description, &permissionsJSON,
		&role.IsDefault, &createdBy, &createdByEmail, &role.CreatedAt, &role.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal(permissionsJSON, &role.Permissions)
	role.CreatedBy = createdBy.String
	role.CreatedByEmail = createdByEmail.String

	// Load associated policy
	policy, err := s.GetRolePolicy(ctx, id)
	if err == nil && policy != nil {
		role.Policy = policy
	}

	return &role, nil
}

// GetRoleByName gets a role by name
func (s *TenantStore) GetRoleByName(ctx context.Context, name string) (*domain.Role, error) {
	query := `
		SELECT id, name, description, permissions, is_default, created_at, updated_at
		FROM roles WHERE name = $1
	`

	var role domain.Role
	var permissionsJSON []byte

	err := s.db.QueryRowContext(ctx, query, name).Scan(
		&role.ID, &role.Name, &role.Description, &permissionsJSON,
		&role.IsDefault, &role.CreatedAt, &role.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal(permissionsJSON, &role.Permissions)
	return &role, nil
}

// UpdateRole updates a role
func (s *TenantStore) UpdateRole(ctx context.Context, role *domain.Role) error {
	permissionsJSON, _ := json.Marshal(role.Permissions)
	role.UpdatedAt = time.Now()

	query := `
		UPDATE roles 
		SET name = $2, description = $3, permissions = $4, is_default = $5, updated_at = $6
		WHERE id = $1
	`

	_, err := s.db.ExecContext(ctx, query, role.ID, role.Name, role.Description,
		permissionsJSON, role.IsDefault, role.UpdatedAt)
	return err
}

// DeleteRole deletes a role
func (s *TenantStore) DeleteRole(ctx context.Context, id string) error {
	// Don't delete system roles
	var isSystem bool
	s.db.QueryRowContext(ctx, "SELECT is_system FROM roles WHERE id = $1", id).Scan(&isSystem)
	if isSystem {
		return fmt.Errorf("cannot delete system role")
	}

	_, err := s.db.ExecContext(ctx, "DELETE FROM roles WHERE id = $1 AND is_system = false", id)
	return err
}

// ListRoles lists all roles
func (s *TenantStore) ListRoles(ctx context.Context) ([]*domain.Role, error) {
	query := `
		SELECT id, name, description, permissions, is_default, created_by, created_by_email, created_at, updated_at
		FROM roles ORDER BY name
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []*domain.Role
	for rows.Next() {
		var role domain.Role
		var permissionsJSON []byte
		var createdBy, createdByEmail sql.NullString

		err := rows.Scan(&role.ID, &role.Name, &role.Description, &permissionsJSON,
			&role.IsDefault, &createdBy, &createdByEmail, &role.CreatedAt, &role.UpdatedAt)
		if err != nil {
			return nil, err
		}

		json.Unmarshal(permissionsJSON, &role.Permissions)
		role.CreatedBy = createdBy.String
		role.CreatedByEmail = createdByEmail.String
		roles = append(roles, &role)
	}

	return roles, nil
}

// GetDefaultRole gets the default role
func (s *TenantStore) GetDefaultRole(ctx context.Context) (*domain.Role, error) {
	query := `
		SELECT id, name, description, permissions, is_default, created_at, updated_at
		FROM roles WHERE is_default = true LIMIT 1
	`

	var role domain.Role
	var permissionsJSON []byte

	err := s.db.QueryRowContext(ctx, query).Scan(
		&role.ID, &role.Name, &role.Description, &permissionsJSON,
		&role.IsDefault, &role.CreatedAt, &role.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal(permissionsJSON, &role.Permissions)
	return &role, nil
}

// =============================================================================
// Role Policy Operations
// =============================================================================

// CreateRolePolicy creates a role policy
func (s *TenantStore) CreateRolePolicy(ctx context.Context, policy *domain.RolePolicy) error {
	if policy.ID == "" {
		policy.ID = uuid.New().String()
	}

	// Marshal all policy types
	promptJSON, _ := json.Marshal(policy.PromptPolicies)
	toolJSON, _ := json.Marshal(policy.ToolPolicies)
	rateLimitJSON, _ := json.Marshal(policy.RateLimitPolicy)
	modelJSON, _ := json.Marshal(policy.ModelRestriction)
	mcpJSON, _ := json.Marshal(policy.MCPPolicies)

	// Marshal extended policies
	cachingJSON, _ := json.Marshal(policy.CachingPolicy)
	routingJSON, _ := json.Marshal(policy.RoutingPolicy)
	resilienceJSON, _ := json.Marshal(policy.ResiliencePolicy)
	budgetJSON, _ := json.Marshal(policy.BudgetPolicy)

	now := time.Now()
	policy.CreatedAt = now
	policy.UpdatedAt = now

	query := `
		INSERT INTO role_policies (
			id, role_id, prompt_policies, tool_policies, rate_limit_policy,
			model_restrictions, mcp_policies, caching_policy, routing_policy,
			resilience_policy, budget_policy, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		ON CONFLICT (role_id) DO UPDATE SET
			prompt_policies = EXCLUDED.prompt_policies,
			tool_policies = EXCLUDED.tool_policies,
			rate_limit_policy = EXCLUDED.rate_limit_policy,
			model_restrictions = EXCLUDED.model_restrictions,
			mcp_policies = EXCLUDED.mcp_policies,
			caching_policy = EXCLUDED.caching_policy,
			routing_policy = EXCLUDED.routing_policy,
			resilience_policy = EXCLUDED.resilience_policy,
			budget_policy = EXCLUDED.budget_policy,
			updated_at = EXCLUDED.updated_at
	`

	_, err := s.db.ExecContext(ctx, query, policy.ID, policy.RoleID,
		promptJSON, toolJSON, rateLimitJSON, modelJSON, mcpJSON,
		cachingJSON, routingJSON, resilienceJSON, budgetJSON, now, now)
	return err
}

// GetRolePolicy gets a role's policy
func (s *TenantStore) GetRolePolicy(ctx context.Context, roleID string) (*domain.RolePolicy, error) {
	query := `
		SELECT id, role_id, prompt_policies, tool_policies, rate_limit_policy, model_restrictions,
		       COALESCE(mcp_policies, '{}'),
		       COALESCE(caching_policy, '{}'),
		       COALESCE(routing_policy, '{}'),
		       COALESCE(resilience_policy, '{}'),
		       COALESCE(budget_policy, '{}'),
		       created_at, updated_at
		FROM role_policies WHERE role_id = $1
	`

	var policy domain.RolePolicy
	var promptJSON, toolJSON, rateLimitJSON, modelJSON, mcpJSON []byte
	var cachingJSON, routingJSON, resilienceJSON, budgetJSON []byte

	err := s.db.QueryRowContext(ctx, query, roleID).Scan(
		&policy.ID, &policy.RoleID, &promptJSON, &toolJSON, &rateLimitJSON, &modelJSON, &mcpJSON,
		&cachingJSON, &routingJSON, &resilienceJSON, &budgetJSON,
		&policy.CreatedAt, &policy.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Unmarshal all policy types
	json.Unmarshal(promptJSON, &policy.PromptPolicies)
	json.Unmarshal(toolJSON, &policy.ToolPolicies)
	json.Unmarshal(rateLimitJSON, &policy.RateLimitPolicy)
	json.Unmarshal(modelJSON, &policy.ModelRestriction)
	json.Unmarshal(mcpJSON, &policy.MCPPolicies)

	// Unmarshal extended policies
	json.Unmarshal(cachingJSON, &policy.CachingPolicy)
	json.Unmarshal(routingJSON, &policy.RoutingPolicy)
	json.Unmarshal(resilienceJSON, &policy.ResiliencePolicy)
	json.Unmarshal(budgetJSON, &policy.BudgetPolicy)

	return &policy, nil
}

// UpdateRolePolicy updates a role's policy
func (s *TenantStore) UpdateRolePolicy(ctx context.Context, policy *domain.RolePolicy) error {
	return s.CreateRolePolicy(ctx, policy) // Upsert
}

// =============================================================================
// Group Operations
// =============================================================================

// CreateGroup creates a new group
func (s *TenantStore) CreateGroup(ctx context.Context, group *domain.Group) error {
	if group.ID == "" {
		group.ID = uuid.New().String()
	}

	now := time.Now()
	group.CreatedAt = now
	group.UpdatedAt = now

	query := `
		INSERT INTO groups (id, name, description, created_by, created_by_email, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := s.db.ExecContext(ctx, query, group.ID, group.Name, group.Description,
		sql.NullString{String: group.CreatedBy, Valid: group.CreatedBy != ""},
		sql.NullString{String: group.CreatedByEmail, Valid: group.CreatedByEmail != ""},
		now, now)
	if err != nil {
		return err
	}

	// Add role associations
	for _, roleID := range group.RoleIDs {
		_, err := s.db.ExecContext(ctx,
			"INSERT INTO group_roles (group_id, role_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
			group.ID, roleID)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetGroup gets a group by ID
func (s *TenantStore) GetGroup(ctx context.Context, id string) (*domain.Group, error) {
	query := `SELECT id, name, description, created_by, created_by_email, created_at, updated_at FROM groups WHERE id = $1`

	var group domain.Group
	var createdBy, createdByEmail sql.NullString
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&group.ID, &group.Name, &group.Description, &createdBy, &createdByEmail, &group.CreatedAt, &group.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	group.CreatedBy = createdBy.String
	group.CreatedByEmail = createdByEmail.String

	// Get role IDs
	rows, err := s.db.QueryContext(ctx, "SELECT role_id FROM group_roles WHERE group_id = $1", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var roleID string
		rows.Scan(&roleID)
		group.RoleIDs = append(group.RoleIDs, roleID)
	}

	return &group, nil
}

// ListGroups lists all groups
func (s *TenantStore) ListGroups(ctx context.Context) ([]*domain.Group, error) {
	query := `SELECT id, name, description, created_by, created_by_email, created_at, updated_at FROM groups ORDER BY name`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []*domain.Group
	for rows.Next() {
		var group domain.Group
		var createdBy, createdByEmail sql.NullString
		err := rows.Scan(&group.ID, &group.Name, &group.Description, &createdBy, &createdByEmail, &group.CreatedAt, &group.UpdatedAt)
		if err != nil {
			return nil, err
		}
		group.CreatedBy = createdBy.String
		group.CreatedByEmail = createdByEmail.String
		groups = append(groups, &group)
	}

	// Load role IDs for each group
	for _, group := range groups {
		roleRows, err := s.db.QueryContext(ctx, "SELECT role_id FROM group_roles WHERE group_id = $1", group.ID)
		if err != nil {
			continue
		}
		for roleRows.Next() {
			var roleID string
			roleRows.Scan(&roleID)
			group.RoleIDs = append(group.RoleIDs, roleID)
		}
		roleRows.Close()
	}

	return groups, nil
}

// UpdateGroup updates a group
func (s *TenantStore) UpdateGroup(ctx context.Context, group *domain.Group) error {
	group.UpdatedAt = time.Now()

	query := `UPDATE groups SET name = $2, description = $3, updated_at = $4 WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, group.ID, group.Name, group.Description, group.UpdatedAt)
	if err != nil {
		return err
	}

	// Update role associations
	s.db.ExecContext(ctx, "DELETE FROM group_roles WHERE group_id = $1", group.ID)
	for _, roleID := range group.RoleIDs {
		s.db.ExecContext(ctx, "INSERT INTO group_roles (group_id, role_id) VALUES ($1, $2)", group.ID, roleID)
	}

	return nil
}

// DeleteGroup deletes a group
func (s *TenantStore) DeleteGroup(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM groups WHERE id = $1", id)
	return err
}

// GetGroupRoles gets all roles assigned to a group with their policies loaded
func (s *TenantStore) GetGroupRoles(ctx context.Context, groupID string) ([]*domain.Role, error) {
	// Get all role IDs for this group
	rows, err := s.db.QueryContext(ctx, "SELECT role_id FROM group_roles WHERE group_id = $1", groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roleIDs []string
	for rows.Next() {
		var roleID string
		if err := rows.Scan(&roleID); err != nil {
			return nil, err
		}
		roleIDs = append(roleIDs, roleID)
	}

	// Load each role with its policy
	var roles []*domain.Role
	for _, roleID := range roleIDs {
		role, err := s.GetRole(ctx, roleID)
		if err == nil && role != nil {
			roles = append(roles, role)
		}
	}

	return roles, nil
}

// =============================================================================
// API Key Operations
// =============================================================================

// CreateAPIKey creates a new API key with role or group assignment
func (s *TenantStore) CreateAPIKey(ctx context.Context, name string, roleID string, groupID string, scopes []string, expiresAt *time.Time) (*domain.APIKey, string, error) {
	// Generate key
	keyBytes := make([]byte, 32)
	rand.Read(keyBytes)
	fullKey := "mg_" + hex.EncodeToString(keyBytes)
	keyPrefix := fullKey[:11]
	keyHash := hashAPIKey(fullKey)

	id := uuid.New().String()
	now := time.Now()
	scopesJSON, _ := json.Marshal(scopes)

	// API key can have either role_id OR group_id, not both
	var roleIDPtr, groupIDPtr interface{}
	if roleID != "" {
		roleIDPtr = roleID
		groupIDPtr = nil
	} else if groupID != "" {
		roleIDPtr = nil
		groupIDPtr = groupID
	}

	query := `
		INSERT INTO api_keys (id, name, key_prefix, key_hash, role_id, group_id, scopes, expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := s.db.ExecContext(ctx, query, id, name, keyPrefix, keyHash, roleIDPtr, groupIDPtr, scopesJSON, expiresAt, now, now)
	if err != nil {
		return nil, "", err
	}

	apiKey := &domain.APIKey{
		ID:        id,
		Name:      name,
		KeyPrefix: keyPrefix,
		KeyHash:   keyHash,
		RoleID:    roleID,
		GroupID:   groupID,
		Scopes:    scopes,
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}

	return apiKey, fullKey, nil
}

// GetAPIKey gets an API key by ID
func (s *TenantStore) GetAPIKey(ctx context.Context, id string) (*domain.APIKeyWithRole, error) {
	query := `
		SELECT k.id, k.name, k.key_prefix, k.key_hash, k.role_id, k.group_id, k.scopes, k.expires_at, k.last_used_at, k.is_revoked, k.created_at, k.updated_at,
		       r.name as role_name, g.name as group_name
		FROM api_keys k
		LEFT JOIN roles r ON k.role_id = r.id
		LEFT JOIN groups g ON k.group_id = g.id
		WHERE k.id = $1
	`

	var key domain.APIKeyWithRole
	var scopesJSON []byte
	var roleID, roleName, groupID, groupName sql.NullString
	var expiresAt, lastUsedAt sql.NullTime

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&key.ID, &key.Name, &key.KeyPrefix, &key.KeyHash, &roleID, &groupID, &scopesJSON,
		&expiresAt, &lastUsedAt, &key.Revoked, &key.CreatedAt, &key.UpdatedAt, &roleName, &groupName)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal(scopesJSON, &key.Scopes)
	if roleID.Valid {
		key.RoleID = roleID.String
	}
	if roleName.Valid {
		key.RoleName = roleName.String
	}
	if groupID.Valid {
		key.GroupID = groupID.String
	}
	if groupName.Valid {
		key.GroupName = groupName.String
	}
	if expiresAt.Valid {
		t := expiresAt.Time
		key.ExpiresAt = &t
	}
	if lastUsedAt.Valid {
		t := lastUsedAt.Time
		key.LastUsedAt = &t
	}

	return &key, nil
}

// GetAPIKeyByHash gets an API key by its hash
func (s *TenantStore) GetAPIKeyByHash(ctx context.Context, keyHash string) (*domain.APIKeyWithRole, error) {
	query := `
		SELECT k.id, k.name, k.key_prefix, k.key_hash, k.role_id, k.group_id, k.scopes, k.expires_at, k.last_used_at, k.is_revoked, k.created_at, k.updated_at,
		       r.name as role_name, g.name as group_name
		FROM api_keys k
		LEFT JOIN roles r ON k.role_id = r.id
		LEFT JOIN groups g ON k.group_id = g.id
		WHERE k.key_hash = $1 AND k.is_revoked = false
	`

	var key domain.APIKeyWithRole
	var scopesJSON []byte
	var roleID, roleName, groupID, groupName sql.NullString
	var expiresAt, lastUsedAt sql.NullTime

	err := s.db.QueryRowContext(ctx, query, keyHash).Scan(
		&key.ID, &key.Name, &key.KeyPrefix, &key.KeyHash, &roleID, &groupID, &scopesJSON,
		&expiresAt, &lastUsedAt, &key.Revoked, &key.CreatedAt, &key.UpdatedAt, &roleName, &groupName)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal(scopesJSON, &key.Scopes)
	if roleID.Valid {
		key.RoleID = roleID.String
	}
	if roleName.Valid {
		key.RoleName = roleName.String
	}
	if groupID.Valid {
		key.GroupID = groupID.String
	}
	if groupName.Valid {
		key.GroupName = groupName.String
	}
	if expiresAt.Valid {
		t := expiresAt.Time
		key.ExpiresAt = &t
	}
	if lastUsedAt.Valid {
		t := lastUsedAt.Time
		key.LastUsedAt = &t
	}

	return &key, nil
}

// UpdateAPIKeyCreator updates the creator info for an API key
func (s *TenantStore) UpdateAPIKeyCreator(ctx context.Context, keyID string, creatorID string, creatorEmail string) error {
	query := `UPDATE api_keys SET created_by = $1, created_by_email = $2 WHERE id = $3`
	_, err := s.db.ExecContext(ctx, query, creatorID, creatorEmail, keyID)
	return err
}

// ListAPIKeys lists all API keys
func (s *TenantStore) ListAPIKeys(ctx context.Context) ([]*domain.APIKeyWithRole, error) {
	query := `
		SELECT k.id, k.name, k.key_prefix, k.role_id, k.group_id, k.scopes, k.expires_at, k.last_used_at, k.is_revoked, k.created_at, k.updated_at,
		       k.created_by, k.created_by_email,
		       r.name as role_name, g.name as group_name
		FROM api_keys k
		LEFT JOIN roles r ON k.role_id = r.id
		LEFT JOIN groups g ON k.group_id = g.id
		ORDER BY k.created_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*domain.APIKeyWithRole
	for rows.Next() {
		var key domain.APIKeyWithRole
		var scopesJSON []byte
		var roleID, roleName, groupID, groupName, createdBy, createdByEmail sql.NullString
		var expiresAt, lastUsedAt sql.NullTime

		err := rows.Scan(&key.ID, &key.Name, &key.KeyPrefix, &roleID, &groupID, &scopesJSON,
			&expiresAt, &lastUsedAt, &key.Revoked, &key.CreatedAt, &key.UpdatedAt,
			&createdBy, &createdByEmail, &roleName, &groupName)
		if err != nil {
			return nil, err
		}

		json.Unmarshal(scopesJSON, &key.Scopes)
		if createdBy.Valid {
			key.APIKey.CreatedBy = createdBy.String
		}
		if createdByEmail.Valid {
			key.APIKey.CreatedByEmail = createdByEmail.String
		}
		if roleID.Valid {
			key.RoleID = roleID.String
		}
		if roleName.Valid {
			key.RoleName = roleName.String
		}
		if groupID.Valid {
			key.GroupID = groupID.String
		}
		if groupName.Valid {
			key.GroupName = groupName.String
		}
		if expiresAt.Valid {
			t := expiresAt.Time
			key.ExpiresAt = &t
		}
		if lastUsedAt.Valid {
			t := lastUsedAt.Time
			key.LastUsedAt = &t
		}

		keys = append(keys, &key)
	}

	return keys, nil
}

// UpdateAPIKey updates an API key's name, role, or group assignment
func (s *TenantStore) UpdateAPIKey(ctx context.Context, id string, name string, roleID string, groupID string) error {
	// API key can have either role_id OR group_id, not both
	var roleIDPtr, groupIDPtr interface{}
	if roleID != "" {
		roleIDPtr = roleID
		groupIDPtr = nil
	} else if groupID != "" {
		roleIDPtr = nil
		groupIDPtr = groupID
	} else {
		// Clear both if neither provided
		roleIDPtr = nil
		groupIDPtr = nil
	}

	query := `
		UPDATE api_keys 
		SET name = $2, role_id = $3, group_id = $4, updated_at = $5
		WHERE id = $1
	`

	_, err := s.db.ExecContext(ctx, query, id, name, roleIDPtr, groupIDPtr, time.Now())
	return err
}

// RevokeAPIKey revokes an API key
func (s *TenantStore) RevokeAPIKey(ctx context.Context, id, reason string) error {
	query := `UPDATE api_keys SET is_revoked = true, revoked_at = $2, revoked_reason = $3, updated_at = $4 WHERE id = $1`
	_, err := s.db.ExecContext(ctx, query, id, time.Now(), reason, time.Now())
	return err
}

// DeleteAPIKey permanently deletes an API key
func (s *TenantStore) DeleteAPIKey(ctx context.Context, id string) error {
	query := `DELETE FROM api_keys WHERE id = $1`
	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return fmt.Errorf("API key not found")
	}

	return nil
}

// UpdateAPIKeyLastUsed updates the last used timestamp
func (s *TenantStore) UpdateAPIKeyLastUsed(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE api_keys SET last_used_at = $2 WHERE id = $1", id, time.Now())
	return err
}

// =============================================================================
// Provider Config Operations
// =============================================================================

// SaveProviderConfig saves a provider configuration
// Note: API keys are stored separately in provider_api_keys table for multi-key support
func (s *TenantStore) SaveProviderConfig(ctx context.Context, config *domain.ProviderConfig) error {
	// Store additional fields in extra_settings
	extra := make(map[string]string)
	if config.ExtraSettings != nil {
		for k, v := range config.ExtraSettings {
			extra[k] = v
		}
	}
	// Store Azure and Bedrock specific fields
	if config.ResourceName != "" {
		extra["resource_name"] = config.ResourceName
	}
	if config.APIVersion != "" {
		extra["api_version"] = config.APIVersion
	}
	if config.RegionPrefix != "" {
		extra["region_prefix"] = config.RegionPrefix
	}
	// Store connection settings in extra_settings as JSON
	connJSON, _ := json.Marshal(config.ConnectionSettings)
	extra["connection_settings"] = string(connJSON)

	extraJSON, _ := json.Marshal(extra)
	now := time.Now()

	query := `
		INSERT INTO provider_configs (provider, is_enabled, base_url, region, models_url, extra_settings, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (provider) DO UPDATE SET
			is_enabled = EXCLUDED.is_enabled,
			base_url = EXCLUDED.base_url,
			region = EXCLUDED.region,
			models_url = EXCLUDED.models_url,
			extra_settings = EXCLUDED.extra_settings,
			updated_at = EXCLUDED.updated_at
	`

	_, err := s.db.ExecContext(ctx, query,
		config.Provider, config.Enabled, config.BaseURL, config.Region,
		config.ModelsURL, extraJSON, now, now)
	return err
}

// GetProviderConfig gets a provider configuration
// Note: API keys are stored separately in provider_api_keys table
func (s *TenantStore) GetProviderConfig(ctx context.Context, provider domain.Provider) (*domain.ProviderConfig, error) {
	query := `
		SELECT provider, is_enabled, base_url, region, models_url, extra_settings
		FROM provider_configs WHERE provider = $1
	`

	var config domain.ProviderConfig
	var extraJSON []byte
	var baseURL, region, modelsURL sql.NullString

	err := s.db.QueryRowContext(ctx, query, provider).Scan(
		&config.Provider, &config.Enabled, &baseURL, &region, &modelsURL, &extraJSON)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if region.Valid {
		config.Region = region.String
	}
	if modelsURL.Valid {
		config.ModelsURL = modelsURL.String
	}
	json.Unmarshal(extraJSON, &config.ExtraSettings)

	// Parse connection settings from extra_settings, use defaults if not set
	config.ConnectionSettings = domain.DefaultConnectionSettings()
	if config.ExtraSettings != nil {
		if connStr, ok := config.ExtraSettings["connection_settings"]; ok {
			json.Unmarshal([]byte(connStr), &config.ConnectionSettings)
		}
		// Extract Azure/Bedrock specific fields from extra_settings
		if v, ok := config.ExtraSettings["resource_name"]; ok {
			config.ResourceName = v
		}
		if v, ok := config.ExtraSettings["api_version"]; ok {
			config.APIVersion = v
		}
		if v, ok := config.ExtraSettings["region_prefix"]; ok {
			config.RegionPrefix = v
		}
	}

	return &config, nil
}

// ListProviderConfigs lists all provider configurations
// Note: API keys are stored separately in provider_api_keys table
func (s *TenantStore) ListProviderConfigs(ctx context.Context) ([]*domain.ProviderConfig, error) {
	query := `
		SELECT provider, is_enabled, base_url, region, models_url, extra_settings
		FROM provider_configs ORDER BY provider
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []*domain.ProviderConfig
	for rows.Next() {
		var config domain.ProviderConfig
		var extraJSON []byte
		var baseURL, region, modelsURL sql.NullString

		err := rows.Scan(&config.Provider, &config.Enabled, &baseURL, &region, &modelsURL, &extraJSON)
		if err != nil {
			return nil, err
		}

		if baseURL.Valid {
			config.BaseURL = baseURL.String
		}
		if region.Valid {
			config.Region = region.String
		}
		if modelsURL.Valid {
			config.ModelsURL = modelsURL.String
		}
		json.Unmarshal(extraJSON, &config.ExtraSettings)

		// Parse connection settings from extra_settings, use defaults if not set
		config.ConnectionSettings = domain.DefaultConnectionSettings()
		if config.ExtraSettings != nil {
			if connStr, ok := config.ExtraSettings["connection_settings"]; ok {
				json.Unmarshal([]byte(connStr), &config.ConnectionSettings)
			}
			// Extract Azure/Bedrock specific fields from extra_settings
			if v, ok := config.ExtraSettings["resource_name"]; ok {
				config.ResourceName = v
			}
			if v, ok := config.ExtraSettings["api_version"]; ok {
				config.APIVersion = v
			}
			if v, ok := config.ExtraSettings["region_prefix"]; ok {
				config.RegionPrefix = v
			}
		}

		configs = append(configs, &config)
	}

	return configs, nil
}

// =============================================================================
// Available Tools Operations
// =============================================================================

// CreateTool creates a new available tool
func (s *TenantStore) CreateTool(ctx context.Context, tool *domain.AvailableTool) error {
	if tool.ID == "" {
		tool.ID = uuid.New().String()
	}

	schemaJSON, _ := json.Marshal(tool.Schema)
	now := time.Now()
	tool.CreatedAt = now
	tool.UpdatedAt = now

	query := `
		INSERT INTO available_tools (id, name, description, category, schema, is_builtin, is_enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := s.db.ExecContext(ctx, query, tool.ID, tool.Name, tool.Description, tool.Category,
		schemaJSON, tool.IsBuiltIn, tool.Enabled, now, now)
	return err
}

// GetTool gets an available tool by ID
func (s *TenantStore) GetTool(ctx context.Context, id string) (*domain.AvailableTool, error) {
	query := `
		SELECT id, name, description, category, schema, is_builtin, is_enabled, created_at, updated_at
		FROM available_tools WHERE id = $1
	`

	var tool domain.AvailableTool
	var schemaJSON []byte

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&tool.ID, &tool.Name, &tool.Description, &tool.Category, &schemaJSON,
		&tool.IsBuiltIn, &tool.Enabled, &tool.CreatedAt, &tool.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal(schemaJSON, &tool.Schema)
	return &tool, nil
}

// ListTools lists all available tools
func (s *TenantStore) ListTools(ctx context.Context) ([]*domain.AvailableTool, error) {
	query := `
		SELECT id, name, description, category, schema, is_builtin, is_enabled, created_at, updated_at
		FROM available_tools ORDER BY category, name
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tools []*domain.AvailableTool
	for rows.Next() {
		var tool domain.AvailableTool
		var schemaJSON []byte

		err := rows.Scan(&tool.ID, &tool.Name, &tool.Description, &tool.Category, &schemaJSON,
			&tool.IsBuiltIn, &tool.Enabled, &tool.CreatedAt, &tool.UpdatedAt)
		if err != nil {
			return nil, err
		}

		json.Unmarshal(schemaJSON, &tool.Schema)
		tools = append(tools, &tool)
	}

	return tools, nil
}

// UpdateTool updates an available tool
func (s *TenantStore) UpdateTool(ctx context.Context, tool *domain.AvailableTool) error {
	schemaJSON, _ := json.Marshal(tool.Schema)
	tool.UpdatedAt = time.Now()

	query := `
		UPDATE available_tools 
		SET name = $2, description = $3, category = $4, schema = $5, is_enabled = $6, updated_at = $7
		WHERE id = $1
	`

	_, err := s.db.ExecContext(ctx, query, tool.ID, tool.Name, tool.Description, tool.Category,
		schemaJSON, tool.Enabled, tool.UpdatedAt)
	return err
}

// DeleteTool deletes an available tool
func (s *TenantStore) DeleteTool(ctx context.Context, id string) error {
	// Don't delete built-in tools
	_, err := s.db.ExecContext(ctx, "DELETE FROM available_tools WHERE id = $1 AND is_builtin = false", id)
	return err
}

// =============================================================================
// Usage Records Operations
// =============================================================================

// RecordUsage records API usage
func (s *TenantStore) RecordUsage(ctx context.Context, record *domain.UsageRecord) error {
	if record.ID == "" {
		record.ID = uuid.New().String()
	}

	// Marshal metadata or use empty JSON if nil
	var metadataJSON []byte
	var err error
	if record.Metadata != nil && len(record.Metadata) > 0 {
		metadataJSON, err = json.Marshal(record.Metadata)
		if err != nil {
			// Fall back to empty JSON on marshal error
			metadataJSON = []byte("{}")
		}
	} else {
		metadataJSON = []byte("{}")
	}

	query := `
		INSERT INTO usage_records (id, api_key_id, request_id, model, provider, input_tokens, output_tokens,
			total_tokens, cost_usd, latency_ms, is_success, error_code, error_message, tool_calls,
			thinking_tokens, metadata, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
	`

	// Convert APIKeyID to UUID or nil
	var apiKeyID interface{}
	if record.APIKeyID != "" {
		apiKeyID = record.APIKeyID
	}

	_, err = s.db.ExecContext(ctx, query, record.ID, apiKeyID, record.RequestID, record.Model,
		record.Provider, record.InputTokens, record.OutputTokens, record.TotalTokens,
		record.CostUSD, record.LatencyMs, record.Success, record.ErrorCode, record.ErrorMessage,
		record.ToolCalls, record.ThinkingTokens, metadataJSON, record.Timestamp)
	return err
}

// GetUsageStats gets usage statistics
func (s *TenantStore) GetUsageStats(ctx context.Context, startTime, endTime time.Time) (*domain.UsageStats, error) {
	query := `
		SELECT 
			COUNT(*) as total_requests,
			COALESCE(SUM(total_tokens), 0) as total_tokens,
			COALESCE(SUM(cost_usd), 0) as total_cost
		FROM usage_records 
		WHERE created_at >= $1 AND created_at <= $2
	`

	var stats domain.UsageStats
	err := s.db.QueryRowContext(ctx, query, startTime, endTime).Scan(
		&stats.TotalRequests, &stats.TotalTokens, &stats.TotalCostUSD)
	if err != nil {
		return nil, err
	}

	return &stats, nil
}

// ListUsageRecords lists usage records with optional filters
func (s *TenantStore) ListUsageRecords(ctx context.Context, startTime, endTime time.Time, model, status, apiKeyID string, limit int) ([]*domain.UsageRecord, error) {
	query := `
		SELECT ur.id, ur.api_key_id, ak.name as api_key_name, ur.request_id, ur.model, ur.provider,
			ur.input_tokens, ur.output_tokens, ur.total_tokens, ur.cost_usd, ur.latency_ms,
			ur.is_success, ur.error_code, ur.error_message, ur.tool_calls, ur.thinking_tokens,
			ur.created_at
		FROM usage_records ur
		LEFT JOIN api_keys ak ON ur.api_key_id = ak.id
		WHERE ur.created_at >= $1 AND ur.created_at <= $2
	`
	args := []interface{}{startTime, endTime}
	argIndex := 3

	if model != "" {
		query += fmt.Sprintf(" AND ur.model = $%d", argIndex)
		args = append(args, model)
		argIndex++
	}

	if status == "success" {
		query += fmt.Sprintf(" AND ur.is_success = $%d", argIndex)
		args = append(args, true)
		argIndex++
	} else if status == "error" {
		query += fmt.Sprintf(" AND ur.is_success = $%d", argIndex)
		args = append(args, false)
		argIndex++
	}

	if apiKeyID != "" {
		query += fmt.Sprintf(" AND ur.api_key_id = $%d", argIndex)
		args = append(args, apiKeyID)
		argIndex++
	}

	query += " ORDER BY ur.created_at DESC"

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []*domain.UsageRecord
	for rows.Next() {
		var record domain.UsageRecord
		var apiKeyID, apiKeyName sql.NullString
		var errorCode, errorMessage sql.NullString

		err := rows.Scan(&record.ID, &apiKeyID, &apiKeyName, &record.RequestID, &record.Model, &record.Provider,
			&record.InputTokens, &record.OutputTokens, &record.TotalTokens, &record.CostUSD,
			&record.LatencyMs, &record.Success, &errorCode, &errorMessage, &record.ToolCalls,
			&record.ThinkingTokens, &record.Timestamp)
		if err != nil {
			return nil, err
		}

		if apiKeyID.Valid {
			record.APIKeyID = apiKeyID.String
		}
		if apiKeyName.Valid {
			record.APIKeyName = apiKeyName.String
		}
		if errorCode.Valid {
			record.ErrorCode = errorCode.String
		}
		if errorMessage.Valid {
			record.ErrorMessage = errorMessage.String
		}
		records = append(records, &record)
	}

	return records, rows.Err()
}

// GetUsageRecord gets a single usage record by ID
func (s *TenantStore) GetUsageRecord(ctx context.Context, id string) (*domain.UsageRecord, error) {
	query := `
		SELECT ur.id, ur.api_key_id, ak.name as api_key_name, ur.request_id, ur.model, ur.provider,
			ur.input_tokens, ur.output_tokens, ur.total_tokens, ur.cost_usd, ur.latency_ms,
			ur.is_success, ur.error_code, ur.error_message, ur.tool_calls, ur.thinking_tokens,
			ur.metadata, ur.created_at
		FROM usage_records ur
		LEFT JOIN api_keys ak ON ur.api_key_id = ak.id
		WHERE ur.id = $1
	`

	var record domain.UsageRecord
	var apiKeyID, apiKeyName sql.NullString
	var errorCode, errorMessage sql.NullString
	var metadataJSON []byte

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&record.ID, &apiKeyID, &apiKeyName, &record.RequestID, &record.Model, &record.Provider,
		&record.InputTokens, &record.OutputTokens, &record.TotalTokens, &record.CostUSD,
		&record.LatencyMs, &record.Success, &errorCode, &errorMessage, &record.ToolCalls,
		&record.ThinkingTokens, &metadataJSON, &record.Timestamp)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if apiKeyID.Valid {
		record.APIKeyID = apiKeyID.String
	}
	if apiKeyName.Valid {
		record.APIKeyName = apiKeyName.String
	}
	if errorCode.Valid {
		record.ErrorCode = errorCode.String
	}
	if errorMessage.Valid {
		record.ErrorMessage = errorMessage.String
	}

	// Parse metadata JSON
	if len(metadataJSON) > 0 {
		var metadata map[string]any
		if err := json.Unmarshal(metadataJSON, &metadata); err == nil {
			record.Metadata = metadata
		}
	}

	return &record, nil
}

// GetUsageStatsByModel gets usage statistics grouped by model
func (s *TenantStore) GetUsageStatsByModel(ctx context.Context, startTime, endTime time.Time) (map[string]*domain.ModelUsageStats, error) {
	query := `
		SELECT
			model,
			COUNT(*) as requests,
			COALESCE(SUM(input_tokens), 0) as input_tokens,
			COALESCE(SUM(output_tokens), 0) as output_tokens,
			COALESCE(SUM(cost_usd), 0) as cost_usd
		FROM usage_records
		WHERE created_at >= $1 AND created_at <= $2
		GROUP BY model
		ORDER BY cost_usd DESC
	`

	rows, err := s.db.QueryContext(ctx, query, startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]*domain.ModelUsageStats)
	for rows.Next() {
		var model string
		var modelStats domain.ModelUsageStats

		err := rows.Scan(&model, &modelStats.Requests, &modelStats.InputTokens,
			&modelStats.OutputTokens, &modelStats.CostUSD)
		if err != nil {
			return nil, err
		}

		stats[model] = &modelStats
	}

	return stats, rows.Err()
}

// GetUsageStatsByProvider gets usage statistics grouped by provider
func (s *TenantStore) GetUsageStatsByProvider(ctx context.Context, startTime, endTime time.Time) (map[string]*domain.ProviderUsageStats, error) {
	query := `
		SELECT
			provider,
			COUNT(*) as requests,
			COALESCE(SUM(total_tokens), 0) as total_tokens,
			COALESCE(SUM(cost_usd), 0) as cost_usd,
			COALESCE(AVG(latency_ms), 0) as avg_latency_ms
		FROM usage_records
		WHERE created_at >= $1 AND created_at <= $2
		GROUP BY provider
		ORDER BY cost_usd DESC
	`

	rows, err := s.db.QueryContext(ctx, query, startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]*domain.ProviderUsageStats)
	for rows.Next() {
		var provider string
		var providerStats domain.ProviderUsageStats

		err := rows.Scan(&provider, &providerStats.Requests, &providerStats.TotalTokens,
			&providerStats.CostUSD, &providerStats.AvgLatencyMs)
		if err != nil {
			return nil, err
		}

		stats[provider] = &providerStats
	}

	return stats, rows.Err()
}

// GetUsageStatsByAPIKey gets usage statistics grouped by API key
func (s *TenantStore) GetUsageStatsByAPIKey(ctx context.Context, startTime, endTime time.Time) (map[string]*domain.APIKeyUsageStats, error) {
	query := `
		SELECT
			ur.api_key_id,
			ak.name as api_key_name,
			COUNT(*) as requests,
			COALESCE(SUM(ur.input_tokens + ur.output_tokens), 0) as total_tokens,
			COALESCE(SUM(ur.cost_usd), 0) as cost_usd
		FROM usage_records ur
		LEFT JOIN api_keys ak ON ur.api_key_id = ak.id
		WHERE ur.created_at >= $1 AND ur.created_at <= $2 AND ur.api_key_id IS NOT NULL
		GROUP BY ur.api_key_id, ak.name
		ORDER BY cost_usd DESC
	`

	rows, err := s.db.QueryContext(ctx, query, startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]*domain.APIKeyUsageStats)
	for rows.Next() {
		var apiKeyID, apiKeyName string
		var apiKeyStats domain.APIKeyUsageStats

		err := rows.Scan(&apiKeyID, &apiKeyName, &apiKeyStats.Requests, &apiKeyStats.TotalTokens, &apiKeyStats.CostUSD)
		if err != nil {
			return nil, err
		}

		apiKeyStats.APIKeyID = apiKeyID
		apiKeyStats.APIKeyName = apiKeyName
		stats[apiKeyID] = &apiKeyStats
	}

	return stats, rows.Err()
}

// GetUsageTimeSeries gets usage over time for charts
func (s *TenantStore) GetUsageTimeSeries(ctx context.Context, startTime, endTime time.Time, interval string) ([]*domain.UsageTimePoint, error) {
	// interval can be "hour", "day", "week", "month"
	var truncFunc string
	switch interval {
	case "hour":
		truncFunc = "date_trunc('hour', created_at)"
	case "day":
		truncFunc = "date_trunc('day', created_at)"
	case "week":
		truncFunc = "date_trunc('week', created_at)"
	case "month":
		truncFunc = "date_trunc('month', created_at)"
	default:
		truncFunc = "date_trunc('day', created_at)"
	}

	query := fmt.Sprintf(`
		SELECT
			%s as time_bucket,
			COUNT(*) as requests,
			COALESCE(SUM(total_tokens), 0) as total_tokens,
			COALESCE(SUM(cost_usd), 0) as cost_usd
		FROM usage_records
		WHERE created_at >= $1 AND created_at <= $2
		GROUP BY time_bucket
		ORDER BY time_bucket ASC
	`, truncFunc)

	rows, err := s.db.QueryContext(ctx, query, startTime, endTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []*domain.UsageTimePoint
	for rows.Next() {
		var point domain.UsageTimePoint

		err := rows.Scan(&point.Timestamp, &point.Requests, &point.Tokens, &point.CostUSD)
		if err != nil {
			return nil, err
		}

		points = append(points, &point)
	}

	return points, rows.Err()
}

// =============================================================================
// Model Configurations
// =============================================================================

// SaveModelConfig creates or updates a model configuration
func (s *TenantStore) SaveModelConfig(ctx context.Context, config *domain.ModelConfig) error {
	if config.ID == "" {
		config.ID = uuid.New().String()
	}

	metadataJSON, _ := json.Marshal(config.Metadata)

	query := `
		INSERT INTO model_configs (id, model_id, is_enabled, alias, max_tokens_override, cost_multiplier, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (model_id) DO UPDATE SET
			is_enabled = EXCLUDED.is_enabled,
			alias = EXCLUDED.alias,
			max_tokens_override = EXCLUDED.max_tokens_override,
			cost_multiplier = EXCLUDED.cost_multiplier,
			metadata = EXCLUDED.metadata,
			updated_at = EXCLUDED.updated_at
	`

	now := time.Now()
	_, err := s.db.ExecContext(ctx, query, config.ID, config.ModelID, config.IsEnabled,
		config.Alias, config.MaxTokensOverride, config.CostMultiplier, metadataJSON, now, now)
	return err
}

// GetModelConfig gets a model configuration by model ID
func (s *TenantStore) GetModelConfig(ctx context.Context, modelID string) (*domain.ModelConfig, error) {
	query := `
		SELECT id, model_id, is_enabled, alias, max_tokens_override, cost_multiplier, metadata, created_at, updated_at
		FROM model_configs
		WHERE model_id = $1
	`

	var config domain.ModelConfig
	var alias sql.NullString
	var maxTokensOverride sql.NullInt64
	var metadataJSON []byte

	err := s.db.QueryRowContext(ctx, query, modelID).Scan(
		&config.ID, &config.ModelID, &config.IsEnabled, &alias, &maxTokensOverride,
		&config.CostMultiplier, &metadataJSON, &config.CreatedAt, &config.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if alias.Valid {
		config.Alias = alias.String
	}
	if maxTokensOverride.Valid {
		config.MaxTokensOverride = int(maxTokensOverride.Int64)
	}
	if len(metadataJSON) > 0 {
		json.Unmarshal(metadataJSON, &config.Metadata)
	}

	return &config, nil
}

// ListModelConfigs lists all model configurations
func (s *TenantStore) ListModelConfigs(ctx context.Context) ([]*domain.ModelConfig, error) {
	query := `
		SELECT id, model_id, is_enabled, alias, max_tokens_override, cost_multiplier, metadata, created_at, updated_at
		FROM model_configs
		ORDER BY model_id
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []*domain.ModelConfig
	for rows.Next() {
		var config domain.ModelConfig
		var alias sql.NullString
		var maxTokensOverride sql.NullInt64
		var metadataJSON []byte

		err := rows.Scan(&config.ID, &config.ModelID, &config.IsEnabled, &alias,
			&maxTokensOverride, &config.CostMultiplier, &metadataJSON, &config.CreatedAt, &config.UpdatedAt)
		if err != nil {
			return nil, err
		}

		if alias.Valid {
			config.Alias = alias.String
		}
		if maxTokensOverride.Valid {
			config.MaxTokensOverride = int(maxTokensOverride.Int64)
		}
		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &config.Metadata)
		}

		configs = append(configs, &config)
	}

	return configs, rows.Err()
}

// DeleteModelConfig deletes a model configuration
func (s *TenantStore) DeleteModelConfig(ctx context.Context, modelID string) error {
	query := `DELETE FROM model_configs WHERE model_id = $1`
	_, err := s.db.ExecContext(ctx, query, modelID)
	return err
}

// =============================================================================
// Telemetry Configuration
// =============================================================================

// SaveTelemetryConfig creates or updates telemetry configuration
func (s *TenantStore) SaveTelemetryConfig(ctx context.Context, config *domain.TelemetryConfig) error {
	if config.ID == "" {
		config.ID = uuid.New().String()
	}

	metadataJSON, _ := json.Marshal(config.Metadata)

	// Delete all existing telemetry configs (there should only be one)
	_, err := s.db.ExecContext(ctx, `DELETE FROM telemetry_config`)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO telemetry_config (id, prometheus_enabled, prometheus_endpoint, otlp_enabled,
			otlp_endpoint, log_level, export_usage_data, webhook_url, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`

	now := time.Now()
	_, err = s.db.ExecContext(ctx, query, config.ID, config.PrometheusEnabled, config.PrometheusEndpoint,
		config.OTLPEnabled, config.OTLPEndpoint, config.LogLevel, config.ExportUsageData,
		config.WebhookURL, metadataJSON, now, now)
	return err
}

// GetTelemetryConfig gets the telemetry configuration
func (s *TenantStore) GetTelemetryConfig(ctx context.Context) (*domain.TelemetryConfig, error) {
	query := `
		SELECT id, prometheus_enabled, prometheus_endpoint, otlp_enabled, otlp_endpoint,
			log_level, export_usage_data, webhook_url, metadata, created_at, updated_at
		FROM telemetry_config
		LIMIT 1
	`

	var config domain.TelemetryConfig
	var prometheusEndpoint, otlpEndpoint, webhookURL sql.NullString
	var metadataJSON []byte

	err := s.db.QueryRowContext(ctx, query).Scan(
		&config.ID, &config.PrometheusEnabled, &prometheusEndpoint, &config.OTLPEnabled,
		&otlpEndpoint, &config.LogLevel, &config.ExportUsageData, &webhookURL,
		&metadataJSON, &config.CreatedAt, &config.UpdatedAt)

	if err == sql.ErrNoRows {
		// Return default config
		return &domain.TelemetryConfig{
			ID:                uuid.New().String(),
			PrometheusEnabled: false,
			OTLPEnabled:       false,
			LogLevel:          "info",
			ExportUsageData:   false,
			Metadata:          make(map[string]string),
		}, nil
	}
	if err != nil {
		return nil, err
	}

	if prometheusEndpoint.Valid {
		config.PrometheusEndpoint = prometheusEndpoint.String
	}
	if otlpEndpoint.Valid {
		config.OTLPEndpoint = otlpEndpoint.String
	}
	if webhookURL.Valid {
		config.WebhookURL = webhookURL.String
	}
	if len(metadataJSON) > 0 {
		json.Unmarshal(metadataJSON, &config.Metadata)
	}

	return &config, nil
}

// =============================================================================
// Available Models Operations
// =============================================================================

// AvailableModel represents a model fetched from a provider's API
type AvailableModel struct {
	ID                string         `json:"id"`
	Provider          string         `json:"provider"`
	ModelID           string         `json:"model_id"`
	ModelName         string         `json:"model_name"`
	NativeModelID     string         `json:"native_model_id,omitempty"` // Full provider-specific model ID (e.g., Bedrock inference profile ID)
	Description       string         `json:"description,omitempty"`
	SupportsTools     bool           `json:"supports_tools"`
	SupportsVision    bool           `json:"supports_vision"`
	SupportsReasoning bool           `json:"supports_reasoning"`
	SupportsStreaming bool           `json:"supports_streaming"`
	ContextWindow     int            `json:"context_window"`
	MaxOutputTokens   int            `json:"max_output_tokens"`
	InputCostPer1M    float64        `json:"input_cost_per_1m"`
	OutputCostPer1M   float64        `json:"output_cost_per_1m"`
	ProviderMetadata  map[string]any `json:"provider_metadata,omitempty"`
	IsAvailable       bool           `json:"is_available"`
	IsDeprecated      bool           `json:"is_deprecated"`
	FetchedAt         time.Time      `json:"fetched_at"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

// SaveAvailableModels saves models fetched from a provider API
func (s *TenantStore) SaveAvailableModels(ctx context.Context, provider string, models []domain.ModelInfo) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Upsert each model
	for _, model := range models {
		// Store empty metadata for now (can be extended later)
		metadataJSON := []byte("{}")

		// Use NativeModelID if provided, otherwise use ID
		nativeModelID := model.NativeModelID
		if nativeModelID == "" {
			nativeModelID = model.ID
		}

		_, err := tx.ExecContext(ctx, `
			INSERT INTO available_models (
				provider, model_id, model_name, native_model_id, description,
				supports_tools, supports_vision, supports_reasoning, supports_streaming,
				context_window, max_output_tokens,
				input_cost_per_1m, output_cost_per_1m,
				provider_metadata, is_available, is_deprecated, fetched_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, NOW())
			ON CONFLICT (provider, model_id)
			DO UPDATE SET
				model_name = EXCLUDED.model_name,
				native_model_id = EXCLUDED.native_model_id,
				description = EXCLUDED.description,
				supports_tools = EXCLUDED.supports_tools,
				supports_vision = EXCLUDED.supports_vision,
				supports_reasoning = EXCLUDED.supports_reasoning,
				supports_streaming = EXCLUDED.supports_streaming,
				context_window = EXCLUDED.context_window,
				max_output_tokens = EXCLUDED.max_output_tokens,
				input_cost_per_1m = EXCLUDED.input_cost_per_1m,
				output_cost_per_1m = EXCLUDED.output_cost_per_1m,
				provider_metadata = EXCLUDED.provider_metadata,
				is_available = EXCLUDED.is_available,
				is_deprecated = EXCLUDED.is_deprecated,
				fetched_at = NOW(),
				updated_at = NOW()
		`,
			string(model.Provider),
			model.ID,
			model.Name,
			nativeModelID,
			"", // description from API if available
			model.SupportsTools,
			false, // supports_vision (could be added to ModelInfo)
			model.SupportsReasoning,
			true, // supports_streaming (default true)
			model.ContextLimit,
			model.OutputLimit,
			model.InputCostPer1M,
			model.OutputCostPer1M,
			metadataJSON,
			model.Enabled,
			false, // is_deprecated
		)

		if err != nil {
			return fmt.Errorf("insert model %s: %w", model.ID, err)
		}
	}

	return tx.Commit()
}

// ListAvailableModels returns all available models
func (s *TenantStore) ListAvailableModels(ctx context.Context, provider string) ([]*AvailableModel, error) {
	query := `
		SELECT
			id, provider, model_id, model_name, COALESCE(native_model_id, model_id) as native_model_id, description,
			supports_tools, supports_vision, supports_reasoning, supports_streaming,
			context_window, max_output_tokens,
			input_cost_per_1m, output_cost_per_1m,
			provider_metadata, is_available, is_deprecated,
			fetched_at, created_at, updated_at
		FROM available_models
		WHERE ($1 = '' OR provider = $1) AND is_available = true
		ORDER BY provider, model_name
	`

	rows, err := s.db.QueryContext(ctx, query, provider)
	if err != nil {
		return nil, fmt.Errorf("query models: %w", err)
	}
	defer rows.Close()

	var models []*AvailableModel
	for rows.Next() {
		var model AvailableModel
		var metadataJSON []byte
		var description sql.NullString

		err := rows.Scan(
			&model.ID, &model.Provider, &model.ModelID, &model.ModelName, &model.NativeModelID, &description,
			&model.SupportsTools, &model.SupportsVision, &model.SupportsReasoning, &model.SupportsStreaming,
			&model.ContextWindow, &model.MaxOutputTokens,
			&model.InputCostPer1M, &model.OutputCostPer1M,
			&metadataJSON, &model.IsAvailable, &model.IsDeprecated,
			&model.FetchedAt, &model.CreatedAt, &model.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan model: %w", err)
		}

		if description.Valid {
			model.Description = description.String
		}

		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &model.ProviderMetadata)
		}

		models = append(models, &model)
	}

	return models, rows.Err()
}

// DeleteProviderModels deletes all models for a provider (used before refresh)
func (s *TenantStore) DeleteProviderModels(ctx context.Context, provider string) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM available_models WHERE provider = $1
	`, provider)
	return err
}

// GetProviderModelsURL gets the models URL for a provider
func (s *TenantStore) GetProviderModelsURL(ctx context.Context, provider string) (string, error) {
	var modelsURL sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT models_url FROM provider_configs WHERE provider = $1
	`, provider).Scan(&modelsURL)

	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get models URL: %w", err)
	}

	if modelsURL.Valid {
		return modelsURL.String, nil
	}
	return "", nil
}

// UpdateProviderModelsURL updates the models URL for a provider
func (s *TenantStore) UpdateProviderModelsURL(ctx context.Context, provider, modelsURL string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE provider_configs SET models_url = $1, updated_at = NOW()
		WHERE provider = $2
	`, modelsURL, provider)
	return err
}

// =============================================================================
// Available Models Operations
// =============================================================================

// ListAvailableModelsForAPI lists all available models in domain.ModelInfo format
func (s *TenantStore) ListAvailableModelsForAPI(ctx context.Context) ([]domain.ModelInfo, error) {
	// Use the existing ListAvailableModels method
	availableModels, err := s.ListAvailableModels(ctx, "")
	if err != nil {
		return nil, err
	}

	// Convert to domain.ModelInfo
	models := make([]domain.ModelInfo, 0, len(availableModels))
	for _, am := range availableModels {
		model := domain.ModelInfo{
			ID:                am.ModelID,
			Name:              am.ModelName,
			Provider:          domain.Provider(am.Provider),
			SupportsTools:     am.SupportsTools,
			SupportsReasoning: am.SupportsReasoning,
			ContextLimit:      uint32(am.ContextWindow),
			OutputLimit:       uint32(am.MaxOutputTokens),
			InputCostPer1M:    am.InputCostPer1M,
			OutputCostPer1M:   am.OutputCostPer1M,
			Enabled:           am.IsAvailable && !am.IsDeprecated,
		}
		models = append(models, model)
	}

	return models, nil
}

// =============================================================================
// Audit Log Operations
// =============================================================================

// CreateAuditLog creates a new audit log entry
func (s *TenantStore) CreateAuditLog(ctx context.Context, log *domain.AuditLog) error {
	if log.ID == "" {
		log.ID = uuid.New().String()
	}
	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now()
	}
	if log.Status == "" {
		log.Status = "success"
	}

	detailsJSON, _ := json.Marshal(log.Details)
	oldValueJSON, _ := json.Marshal(log.OldValue)
	newValueJSON, _ := json.Marshal(log.NewValue)

	query := `
		INSERT INTO audit_logs (
			id, timestamp, action, resource_type, resource_id, resource_name,
			actor_id, actor_email, actor_type, ip_address, user_agent,
			details, old_value, new_value, status, error_message
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`

	_, err := s.db.ExecContext(ctx, query,
		log.ID, log.Timestamp, log.Action, log.ResourceType, log.ResourceID, log.ResourceName,
		log.ActorID, log.ActorEmail, log.ActorType, log.IPAddress, log.UserAgent,
		detailsJSON, oldValueJSON, newValueJSON, log.Status, log.ErrorMessage,
	)
	return err
}

// ListAuditLogs retrieves audit logs with filtering
func (s *TenantStore) ListAuditLogs(ctx context.Context, filter domain.AuditLogFilter) ([]domain.AuditLog, error) {
	query := `
		SELECT id, timestamp, action, resource_type, resource_id, resource_name,
			   actor_id, actor_email, actor_type, ip_address, user_agent,
			   details, old_value, new_value, status, error_message
		FROM audit_logs
		WHERE 1=1
	`
	args := []interface{}{}
	argIdx := 1

	if filter.ResourceType != "" {
		query += fmt.Sprintf(" AND resource_type = $%d", argIdx)
		args = append(args, filter.ResourceType)
		argIdx++
	}
	if filter.ResourceID != "" {
		query += fmt.Sprintf(" AND resource_id = $%d", argIdx)
		args = append(args, filter.ResourceID)
		argIdx++
	}
	if filter.Action != "" {
		query += fmt.Sprintf(" AND action = $%d", argIdx)
		args = append(args, filter.Action)
		argIdx++
	}
	if filter.ActorID != "" {
		query += fmt.Sprintf(" AND actor_id = $%d", argIdx)
		args = append(args, filter.ActorID)
		argIdx++
	}
	if !filter.StartTime.IsZero() {
		query += fmt.Sprintf(" AND timestamp >= $%d", argIdx)
		args = append(args, filter.StartTime)
		argIdx++
	}
	if !filter.EndTime.IsZero() {
		query += fmt.Sprintf(" AND timestamp <= $%d", argIdx)
		args = append(args, filter.EndTime)
		argIdx++
	}

	query += " ORDER BY timestamp DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIdx)
		args = append(args, filter.Limit)
		argIdx++
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIdx)
		args = append(args, filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []domain.AuditLog
	for rows.Next() {
		var log domain.AuditLog
		var detailsJSON, oldValueJSON, newValueJSON []byte
		var actorEmail, ipAddress, userAgent, errorMessage sql.NullString

		err := rows.Scan(
			&log.ID, &log.Timestamp, &log.Action, &log.ResourceType, &log.ResourceID, &log.ResourceName,
			&log.ActorID, &actorEmail, &log.ActorType, &ipAddress, &userAgent,
			&detailsJSON, &oldValueJSON, &newValueJSON, &log.Status, &errorMessage,
		)
		if err != nil {
			return nil, err
		}

		log.ActorEmail = actorEmail.String
		log.IPAddress = ipAddress.String
		log.UserAgent = userAgent.String
		log.ErrorMessage = errorMessage.String

		json.Unmarshal(detailsJSON, &log.Details)
		json.Unmarshal(oldValueJSON, &log.OldValue)
		json.Unmarshal(newValueJSON, &log.NewValue)

		logs = append(logs, log)
	}

	return logs, rows.Err()
}

// GetAuditLog retrieves a single audit log by ID
func (s *TenantStore) GetAuditLog(ctx context.Context, id string) (*domain.AuditLog, error) {
	query := `
		SELECT id, timestamp, action, resource_type, resource_id, resource_name,
			   actor_id, actor_email, actor_type, ip_address, user_agent,
			   details, old_value, new_value, status, error_message
		FROM audit_logs WHERE id = $1
	`

	var log domain.AuditLog
	var detailsJSON, oldValueJSON, newValueJSON []byte
	var actorEmail, ipAddress, userAgent, errorMessage sql.NullString

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&log.ID, &log.Timestamp, &log.Action, &log.ResourceType, &log.ResourceID, &log.ResourceName,
		&log.ActorID, &actorEmail, &log.ActorType, &ipAddress, &userAgent,
		&detailsJSON, &oldValueJSON, &newValueJSON, &log.Status, &errorMessage,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	log.ActorEmail = actorEmail.String
	log.IPAddress = ipAddress.String
	log.UserAgent = userAgent.String
	log.ErrorMessage = errorMessage.String

	json.Unmarshal(detailsJSON, &log.Details)
	json.Unmarshal(oldValueJSON, &log.OldValue)
	json.Unmarshal(newValueJSON, &log.NewValue)

	return &log, nil
}

// CountAuditLogs returns the count of audit logs matching the filter
func (s *TenantStore) CountAuditLogs(ctx context.Context, filter domain.AuditLogFilter) (int, error) {
	query := `SELECT COUNT(*) FROM audit_logs WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if filter.ResourceType != "" {
		query += fmt.Sprintf(" AND resource_type = $%d", argIdx)
		args = append(args, filter.ResourceType)
		argIdx++
	}
	if filter.Action != "" {
		query += fmt.Sprintf(" AND action = $%d", argIdx)
		args = append(args, filter.Action)
		argIdx++
	}
	if filter.ActorID != "" {
		query += fmt.Sprintf(" AND actor_id = $%d", argIdx)
		args = append(args, filter.ActorID)
	}

	var count int
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

// =============================================================================
// Helper Functions
// =============================================================================

func hashAPIKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// =============================================================================
// Agent Dashboard
// =============================================================================

// AgentDashboardStore returns an agent dashboard store for this tenant
func (s *TenantStore) AgentDashboardStore() *AgentDashboardStore {
	return NewAgentDashboardStore(s.db)
}
