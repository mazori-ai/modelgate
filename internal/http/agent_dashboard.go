package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"modelgate/internal/domain"
)

// handleAgentDashboardStats retrieves comprehensive dashboard statistics for an agent
// GET /v1/agents/dashboard/stats?api_key_id={id}&start_time={iso8601}&end_time={iso8601}
func (s *Server) handleAgentDashboardStats(w http.ResponseWriter, r *http.Request, auth *AuthContext) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	// Parse query parameters
	apiKeyID := r.URL.Query().Get("api_key_id")
	startTimeStr := r.URL.Query().Get("start_time")
	endTimeStr := r.URL.Query().Get("end_time")

	// Default to last 24 hours if not specified
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)

	if startTimeStr != "" {
		parsed, err := time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			s.writeError(w, http.StatusBadRequest, "invalid start_time format, use ISO8601/RFC3339", "INVALID_TIME_FORMAT")
			return
		}
		startTime = parsed
	}

	if endTimeStr != "" {
		parsed, err := time.Parse(time.RFC3339, endTimeStr)
		if err != nil {
			s.writeError(w, http.StatusBadRequest, "invalid end_time format, use ISO8601/RFC3339", "INVALID_TIME_FORMAT")
			return
		}
		endTime = parsed
	}

	// Validate time range (max 90 days)
	if endTime.Sub(startTime) > 90*24*time.Hour {
		s.writeError(w, http.StatusBadRequest, "time range cannot exceed 90 days", "TIME_RANGE_TOO_LARGE")
		return
	}

	// Get tenant store (single-tenant mode)
	tenantStore := s.store.TenantStore()

	// If api_key_id is provided, verify it belongs to this tenant
	if apiKeyID != "" {
		apiKey, err := tenantStore.GetAPIKey(r.Context(), apiKeyID)
		if err != nil || apiKey == nil {
			s.writeError(w, http.StatusNotFound, "API key not found", "API_KEY_NOT_FOUND")
			return
		}
		// Note: In multi-tenant setup, tenant_id is already validated through auth
	}

	// Get dashboard stats
	dashboardStore := tenantStore.AgentDashboardStore()
	stats, err := dashboardStore.GetStats(r.Context(), auth.Tenant.ID, apiKeyID, startTime, endTime)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get dashboard stats: %v", err), "INTERNAL_ERROR")
		return
	}

	// Return stats
	s.writeJSON(w, http.StatusOK, stats)
}

// handleAgentRiskAssessment retrieves risk assessment for an agent
// GET /v1/agents/dashboard/risk?api_key_id={id}&start_time={iso8601}&end_time={iso8601}
func (s *Server) handleAgentRiskAssessment(w http.ResponseWriter, r *http.Request, auth *AuthContext) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	// Parse query parameters (same as stats endpoint)
	apiKeyID := r.URL.Query().Get("api_key_id")
	startTimeStr := r.URL.Query().Get("start_time")
	endTimeStr := r.URL.Query().Get("end_time")

	// Default to last 24 hours if not specified
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)

	if startTimeStr != "" {
		parsed, err := time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			s.writeError(w, http.StatusBadRequest, "invalid start_time format, use ISO8601/RFC3339", "INVALID_TIME_FORMAT")
			return
		}
		startTime = parsed
	}

	if endTimeStr != "" {
		parsed, err := time.Parse(time.RFC3339, endTimeStr)
		if err != nil {
			s.writeError(w, http.StatusBadRequest, "invalid end_time format, use ISO8601/RFC3339", "INVALID_TIME_FORMAT")
			return
		}
		endTime = parsed
	}

	// Get tenant store (single-tenant mode)
	tenantStore := s.store.TenantStore()

	// Get dashboard store and fetch stats (includes risk score)
	dashboardStore := tenantStore.AgentDashboardStore()
	stats, err := dashboardStore.GetStats(r.Context(), "default", apiKeyID, startTime, endTime)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get risk assessment: %v", err), "INTERNAL_ERROR")
		return
	}

	// Return just the risk score
	s.writeJSON(w, http.StatusOK, stats.RiskScore)
}

// handleListAgents lists all API keys (agents) for the authenticated tenant
// GET /v1/agents/list
func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request, auth *AuthContext) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	// Get tenant store (single-tenant mode)
	tenantStore := s.store.TenantStore()

	// Get all API keys
	apiKeys, err := tenantStore.ListAPIKeys(r.Context())
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list API keys: %v", err), "INTERNAL_ERROR")
		return
	}

	// Filter out sensitive information and format response
	type AgentInfo struct {
		ID         string     `json:"id"`
		Name       string     `json:"name"`
		KeyPrefix  string     `json:"key_prefix"`
		RoleID     string     `json:"role_id,omitempty"`
		RoleName   string     `json:"role_name,omitempty"`
		GroupID    string     `json:"group_id,omitempty"`
		GroupName  string     `json:"group_name,omitempty"`
		CreatedAt  time.Time  `json:"created_at"`
		LastUsedAt *time.Time `json:"last_used_at,omitempty"`
		Revoked    bool       `json:"revoked"`
	}

	agents := make([]AgentInfo, 0, len(apiKeys))
	for _, key := range apiKeys {
		agents = append(agents, AgentInfo{
			ID:         key.ID,
			Name:       key.Name,
			KeyPrefix:  key.KeyPrefix,
			RoleID:     key.RoleID,
			RoleName:   key.RoleName,
			GroupID:    key.GroupID,
			GroupName:  key.GroupName,
			CreatedAt:  key.CreatedAt,
			LastUsedAt: key.LastUsedAt,
			Revoked:    key.Revoked,
		})
	}

	response := map[string]interface{}{
		"agents": agents,
		"total":  len(agents),
	}

	s.writeJSON(w, http.StatusOK, response)
}

// handleRecordPolicyViolation manually records a policy violation (for testing/debugging)
// POST /v1/agents/dashboard/violations
func (s *Server) handleRecordPolicyViolation(w http.ResponseWriter, r *http.Request, auth *AuthContext) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "METHOD_NOT_ALLOWED")
		return
	}

	var event domain.PolicyViolationRecord
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body", "INVALID_REQUEST")
		return
	}

	// Get tenant store (single-tenant mode)
	tenantStore, err := s.store.GetTenantStore("default")
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get tenant store: %v", err), "INTERNAL_ERROR")
		return
	}

	// Record violation
	dashboardStore := tenantStore.AgentDashboardStore()
	if err := dashboardStore.RecordPolicyViolation(r.Context(), &event); err != nil {
		s.writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to record violation: %v", err), "INTERNAL_ERROR")
		return
	}

	s.writeJSON(w, http.StatusCreated, map[string]string{
		"status": "recorded",
		"id":     event.ID,
	})
}
