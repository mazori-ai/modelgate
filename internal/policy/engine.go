// Package policy implements the policy engine with ARN-style access control and prompt safety.
package policy

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"unicode/utf8"

	"modelgate/internal/domain"
)

// Engine implements the PolicyEngine interface
type Engine struct {
	policyRepo     domain.PolicyRepository
	tenantRepo     domain.TenantRepository
	rolePolicyRepo domain.RolePolicyRepository
	groupRepo      domain.GroupRepository
	
	// Cached compiled patterns
	patternCache map[string]*regexp.Regexp
	cacheMu      sync.RWMutex
	
	// Configuration
	config EngineConfig
}

// EngineConfig contains policy engine configuration
type EngineConfig struct {
	EnablePromptSafety     bool
	EnableOutlierDetection bool
	MaxPromptLength        int
	AnomalyThreshold       float64
	InjectionPatterns      []string
	BlockedPatterns        []string
}

// DefaultEngineConfig returns default configuration
func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		EnablePromptSafety:     true,
		EnableOutlierDetection: true,
		MaxPromptLength:        100000,
		AnomalyThreshold:       0.8,
		InjectionPatterns: []string{
			`(?i)ignore\s+(previous|all|above)\s+(instructions?|prompts?)`,
			`(?i)disregard\s+(previous|all|your)\s+(instructions?|prompts?)`,
			`(?i)you\s+are\s+now\s+(a|an|in)`,
			`(?i)pretend\s+(you|to\s+be)`,
			`(?i)jailbreak`,
			`(?i)bypass\s+(safety|filter|restriction)`,
		},
		BlockedPatterns: []string{
			// Add patterns for content you want to block
		},
	}
}

// NewEngine creates a new policy engine
func NewEngine(policyRepo domain.PolicyRepository, tenantRepo domain.TenantRepository, config EngineConfig) *Engine {
	return &Engine{
		policyRepo:   policyRepo,
		tenantRepo:   tenantRepo,
		patternCache: make(map[string]*regexp.Regexp),
		config:       config,
	}
}

// SetRolePolicyRepo sets the role policy repository
func (e *Engine) SetRolePolicyRepo(repo domain.RolePolicyRepository) {
	e.rolePolicyRepo = repo
}

// SetGroupRepo sets the group repository
func (e *Engine) SetGroupRepo(repo domain.GroupRepository) {
	e.groupRepo = repo
}

// Evaluate evaluates a request against policies
func (e *Engine) Evaluate(ctx context.Context, tenantID string, req *domain.ChatRequest) (*domain.PolicyEvaluationResult, error) {
	result := &domain.PolicyEvaluationResult{
		Allowed:         true,
		Violations:      []domain.PolicyViolation{},
		MatchedPolicies: []string{},
	}

	// Get tenant
	tenant, err := e.tenantRepo.Get(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("getting tenant: %w", err)
	}

	// Check tenant status
	if tenant.Status != domain.TenantStatusActive {
		result.Allowed = false
		result.Violations = append(result.Violations, domain.PolicyViolation{
			PolicyID:      "system",
			PolicyName:    "Tenant Status",
			ViolationType: "tenant_suspended",
			Message:       "Tenant account is not active",
			Severity:      "critical",
		})
		return result, nil
	}

	// Check tenant settings
	if err := e.checkTenantSettings(tenant, req, result); err != nil {
		return nil, err
	}

	// Get and evaluate policies
	policies, err := e.policyRepo.GetByTenant(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("getting policies: %w", err)
	}

	for _, policy := range policies {
		if !policy.Enabled {
			continue
		}

		if violated := e.evaluatePolicy(policy, tenant, req); violated != nil {
			result.Violations = append(result.Violations, *violated)
			result.MatchedPolicies = append(result.MatchedPolicies, policy.ID)
			
			if violated.Severity == "critical" {
				result.Allowed = false
			}
		}
	}

	// Check prompt safety
	if e.config.EnablePromptSafety {
		promptAnalysis, _ := e.AnalyzePrompt(ctx, tenantID, req)
		if promptAnalysis != nil && !promptAnalysis.SafetyScore.IsSafe {
			for _, flag := range promptAnalysis.ContentFlags {
				if flag.Blocking {
					result.Allowed = false
					result.Violations = append(result.Violations, domain.PolicyViolation{
						PolicyID:      "prompt_safety",
						PolicyName:    "Prompt Safety",
						ViolationType: flag.Category,
						Message:       flag.Description,
						Severity:      "high",
					})
				}
			}
		}
	}

	// Check role-based policies
	if e.rolePolicyRepo != nil {
		if req.RoleID != "" {
			// Single role assigned to API key
			rolePolicy, err := e.rolePolicyRepo.Get(tenantID, req.RoleID)
			if err == nil && rolePolicy != nil {
				e.checkRolePolicy(rolePolicy, tenant, req, result)
			}
		} else if req.GroupID != "" && e.groupRepo != nil {
			// Group assigned to API key - combine policies from all roles in the group
			// For groups, we use the MOST PERMISSIVE policy (union of allowed models/tools)
			group, err := e.groupRepo.Get(tenantID, req.GroupID)
			if err == nil && group != nil && len(group.RoleIDs) > 0 {
				e.checkGroupPolicies(group, tenantID, tenant, req, result)
			}
		}
	}

	return result, nil
}

// checkGroupPolicies checks request against combined policies of all roles in the group
// Uses the MOST PERMISSIVE approach: if ANY role allows, it's allowed
func (e *Engine) checkGroupPolicies(group *domain.Group, tenantID string, tenant *domain.Tenant, req *domain.ChatRequest, result *domain.PolicyEvaluationResult) {
	// Collect all role policies
	var rolePolicies []*domain.RolePolicy
	for _, roleID := range group.RoleIDs {
		rp, err := e.rolePolicyRepo.Get(tenantID, roleID)
		if err == nil && rp != nil {
			rolePolicies = append(rolePolicies, rp)
		}
	}
	
	if len(rolePolicies) == 0 {
		return // No policies to check
	}
	
	// Combine model restrictions - use MOST PERMISSIVE approach
	modelAllowed := e.checkGroupModelRestrictions(rolePolicies, req, result)
	if !modelAllowed {
		result.Allowed = false
	}
	
	// Combine provider restrictions - if ANY role allows the provider, it's allowed
	e.checkGroupProviderRestrictions(rolePolicies, req, result)
	
	// Combine token limits - use the HIGHEST limit
	e.checkGroupTokenLimits(rolePolicies, req, result)
	
	// Combine tool restrictions - if ANY role allows tools, they're allowed
	e.checkGroupToolRestrictions(rolePolicies, req, result)
}

// checkGroupModelRestrictions checks model access across all roles in a group
// Returns true if the model is allowed by ANY role
func (e *Engine) checkGroupModelRestrictions(rolePolicies []*domain.RolePolicy, req *domain.ChatRequest, result *domain.PolicyEvaluationResult) bool {
	modelAllowed := false
	
	for _, rolePolicy := range rolePolicies {
		restrictions := rolePolicy.ModelRestriction
		
		// If no allowed models are configured, allow all
		if len(restrictions.AllowedModels) == 0 {
			modelAllowed = true
			break
		}
		
		// Check if model is in allowed list
		for _, m := range restrictions.AllowedModels {
			if matchesPattern(req.Model, m) {
				modelAllowed = true
				break
			}
		}
		
		if modelAllowed {
			break // At least one role allows, so we're good
		}
	}
	
	if !modelAllowed {
		result.Violations = append(result.Violations, domain.PolicyViolation{
			PolicyID:      "group_policy",
			PolicyName:    "Group Model Restriction",
			ViolationType: "model_not_allowed",
			Message:       fmt.Sprintf("Model %s is not allowed for any role in the group", req.Model),
			Severity:      "high",
		})
	}
	
	return modelAllowed
}

// checkGroupProviderRestrictions checks provider access across all roles
func (e *Engine) checkGroupProviderRestrictions(rolePolicies []*domain.RolePolicy, req *domain.ChatRequest, result *domain.PolicyEvaluationResult) {
	provider := e.extractProviderFromModel(req.Model)
	providerAllowed := false
	
	for _, rolePolicy := range rolePolicies {
		// If any role has no provider restrictions, allow all
		if len(rolePolicy.ModelRestriction.AllowedProviders) == 0 {
			providerAllowed = true
			break
		}
		
		// Check if provider is allowed
		for _, p := range rolePolicy.ModelRestriction.AllowedProviders {
			if p == provider {
				providerAllowed = true
				break
			}
		}
		
		if providerAllowed {
			break
		}
	}
	
	if !providerAllowed {
		result.Allowed = false
		result.Violations = append(result.Violations, domain.PolicyViolation{
			PolicyID:      "group_policy",
			PolicyName:    "Group Provider Restriction",
			ViolationType: "provider_not_allowed",
			Message:       fmt.Sprintf("Provider %s is not allowed for any role in the group", provider),
			Severity:      "high",
		})
	}
}

// checkGroupTokenLimits checks token limits - uses the HIGHEST limit
func (e *Engine) checkGroupTokenLimits(rolePolicies []*domain.RolePolicy, req *domain.ChatRequest, result *domain.PolicyEvaluationResult) {
	if req.MaxTokens == nil {
		return
	}
	
	var maxAllowed int32 = 0
	for _, rolePolicy := range rolePolicies {
		if rolePolicy.ModelRestriction.MaxTokensPerRequest > maxAllowed {
			maxAllowed = rolePolicy.ModelRestriction.MaxTokensPerRequest
		}
	}
	
	// If any role has no limit (0), allow unlimited
	if maxAllowed == 0 {
		return
	}
	
	if *req.MaxTokens > maxAllowed {
		result.Allowed = false
		result.Violations = append(result.Violations, domain.PolicyViolation{
			PolicyID:      "group_policy",
			PolicyName:    "Group Token Limit",
			ViolationType: "token_limit_exceeded",
			Message:       fmt.Sprintf("Requested %d tokens exceeds group limit of %d", *req.MaxTokens, maxAllowed),
			Severity:      "high",
		})
	}
}

// checkGroupToolRestrictions checks tool access across all roles
func (e *Engine) checkGroupToolRestrictions(rolePolicies []*domain.RolePolicy, req *domain.ChatRequest, result *domain.PolicyEvaluationResult) {
	if len(req.Tools) == 0 {
		return
	}
	
	// Check if any role allows tool calling
	toolCallingAllowed := false
	for _, rolePolicy := range rolePolicies {
		if rolePolicy.ToolPolicies.AllowToolCalling {
			toolCallingAllowed = true
			break
		}
	}
	
	if !toolCallingAllowed {
		result.Allowed = false
		result.Violations = append(result.Violations, domain.PolicyViolation{
			PolicyID:      "group_policy",
			PolicyName:    "Group Tool Access",
			ViolationType: "tool_calling_blocked",
			Message:       "Tool calling is not allowed for any role in the group",
			Severity:      "high",
		})
		return
	}
	
	// Check individual tools - allowed if ANY role allows it
	for _, tool := range req.Tools {
		toolAllowed := false
		
		for _, rolePolicy := range rolePolicies {
			if !rolePolicy.ToolPolicies.AllowToolCalling {
				continue
			}
			
			// Check if tool is in allowed list
			for _, allowedTool := range rolePolicy.ToolPolicies.AllowedTools {
				if matchesPattern(tool.Function.Name, allowedTool) {
					toolAllowed = true
					break
				}
			}
			
			if toolAllowed {
				break
			}
		}
		
		if !toolAllowed {
			result.Violations = append(result.Violations, domain.PolicyViolation{
				PolicyID:      "group_policy",
				PolicyName:    "Group Tool Access",
				ViolationType: "tool_not_allowed",
				Message:       fmt.Sprintf("Tool %s is not allowed for any role in the group", tool.Function.Name),
				Severity:      "medium",
			})
		}
	}
}

// checkRolePolicy checks request against role-specific policies
func (e *Engine) checkRolePolicy(rolePolicy *domain.RolePolicy, tenant *domain.Tenant, req *domain.ChatRequest, result *domain.PolicyEvaluationResult) {
	// Check model restrictions based on mode (whitelist or blacklist)
	e.checkModelRestrictions(rolePolicy, req, result)
	
	// Check allowed providers
	if len(rolePolicy.ModelRestriction.AllowedProviders) > 0 {
		provider := e.extractProviderFromModel(req.Model)
		allowed := false
		for _, p := range rolePolicy.ModelRestriction.AllowedProviders {
			if p == provider {
				allowed = true
				break
			}
		}
		if !allowed {
			result.Allowed = false
			result.Violations = append(result.Violations, domain.PolicyViolation{
				PolicyID:      "role_policy",
				PolicyName:    "Role Provider Restriction",
				ViolationType: "provider_not_allowed",
				Message:       fmt.Sprintf("Provider %s is not allowed for this role", provider),
				Severity:      "high",
			})
		}
	}

	// Check max tokens restriction
	if rolePolicy.ModelRestriction.MaxTokensPerRequest > 0 && req.MaxTokens != nil {
		if *req.MaxTokens > rolePolicy.ModelRestriction.MaxTokensPerRequest {
			result.Allowed = false
			result.Violations = append(result.Violations, domain.PolicyViolation{
				PolicyID:      "role_policy",
				PolicyName:    "Role Token Limit",
				ViolationType: "token_limit_exceeded",
				Message:       fmt.Sprintf("Requested %d tokens exceeds role limit of %d", *req.MaxTokens, rolePolicy.ModelRestriction.MaxTokensPerRequest),
				Severity:      "high",
			})
		}
	}

	// Check tool restrictions
	if len(req.Tools) > 0 {
		// Check if tool calling is blocked for this role
		if !rolePolicy.ToolPolicies.AllowToolCalling {
			result.Allowed = false
			result.Violations = append(result.Violations, domain.PolicyViolation{
				PolicyID:      "role_policy",
				PolicyName:    "Role Tool Access",
				ViolationType: "tool_calling_blocked",
				Message:       "Tool calling is not allowed for this role",
				Severity:      "high",
			})
		} else if len(rolePolicy.ToolPolicies.AllowedTools) > 0 {
			// Check each tool against allowed tools
			for _, tool := range req.Tools {
				allowed := false
				for _, allowedTool := range rolePolicy.ToolPolicies.AllowedTools {
					if matchesPattern(tool.Function.Name, allowedTool) {
						allowed = true
						break
					}
				}
				if !allowed {
					result.Violations = append(result.Violations, domain.PolicyViolation{
						PolicyID:      "role_policy",
						PolicyName:    "Role Tool Access",
						ViolationType: "tool_not_allowed",
						Message:       fmt.Sprintf("Tool %s is not allowed for this role", tool.Function.Name),
						Severity:      "medium",
					})
				}
			}
		}

		// Check blocked tools
		for _, tool := range req.Tools {
			for _, blockedTool := range rolePolicy.ToolPolicies.BlockedTools {
				if matchesPattern(tool.Function.Name, blockedTool) {
					result.Allowed = false
					result.Violations = append(result.Violations, domain.PolicyViolation{
						PolicyID:      "role_policy",
						PolicyName:    "Role Tool Access",
						ViolationType: "tool_blocked",
						Message:       fmt.Sprintf("Tool %s is blocked for this role", tool.Function.Name),
						Severity:      "high",
					})
					break
				}
			}
		}
	}

	// Check prompt policies using InputBounds
	maxPromptLen := rolePolicy.PromptPolicies.InputBounds.MaxPromptLength
	if maxPromptLen > 0 {
		promptLen := len(req.Prompt)
		for _, msg := range req.Messages {
			for _, content := range msg.Content {
				if content.Type == "text" {
					promptLen += len(content.Text)
				}
			}
		}
		if promptLen > maxPromptLen {
			result.Allowed = false
			result.Violations = append(result.Violations, domain.PolicyViolation{
				PolicyID:      "role_policy",
				PolicyName:    "Role Prompt Limit",
				ViolationType: "prompt_too_long",
				Message:       fmt.Sprintf("Prompt length %d exceeds role limit of %d", promptLen, maxPromptLen),
				Severity:      "high",
			})
		}
	}
}

// checkTenantSettings checks request against tenant settings
func (e *Engine) checkTenantSettings(tenant *domain.Tenant, req *domain.ChatRequest, result *domain.PolicyEvaluationResult) error {
	settings := tenant.Settings

	// Check allowed models
	if len(settings.AllowedModels) > 0 {
		allowed := false
		for _, m := range settings.AllowedModels {
			if matchesPattern(req.Model, m) {
				allowed = true
				break
			}
		}
		if !allowed {
			result.Allowed = false
			result.Violations = append(result.Violations, domain.PolicyViolation{
				PolicyID:      "tenant_settings",
				PolicyName:    "Model Access",
				ViolationType: "model_not_allowed",
				Message:       fmt.Sprintf("Model %s is not allowed for this tenant", req.Model),
				Severity:      "high",
			})
		}
	}

	// Check tool calling
	if len(req.Tools) > 0 && !settings.AllowToolCalling {
		result.Allowed = false
		result.Violations = append(result.Violations, domain.PolicyViolation{
			PolicyID:      "tenant_settings",
			PolicyName:    "Tool Calling",
			ViolationType: "tool_calling_not_allowed",
			Message:       "Tool calling is not allowed for this tenant",
			Severity:      "high",
		})
	}

	// Check reasoning mode
	if req.ReasoningConfig != nil && req.ReasoningConfig.Enabled && !settings.AllowReasoningMode {
		result.Allowed = false
		result.Violations = append(result.Violations, domain.PolicyViolation{
			PolicyID:      "tenant_settings",
			PolicyName:    "Reasoning Mode",
			ViolationType: "reasoning_not_allowed",
			Message:       "Reasoning mode is not allowed for this tenant",
			Severity:      "high",
		})
	}

	// Check max tokens
	if settings.MaxTokensPerRequest > 0 && req.MaxTokens != nil {
		if *req.MaxTokens > settings.MaxTokensPerRequest {
			result.Allowed = false
			result.Violations = append(result.Violations, domain.PolicyViolation{
				PolicyID:      "tenant_settings",
				PolicyName:    "Token Limit",
				ViolationType: "token_limit_exceeded",
				Message:       fmt.Sprintf("Requested %d tokens exceeds limit of %d", *req.MaxTokens, settings.MaxTokensPerRequest),
				Severity:      "high",
			})
		}
	}

	return nil
}

// evaluatePolicy evaluates a single policy against the request
func (e *Engine) evaluatePolicy(policy *domain.Policy, tenant *domain.Tenant, req *domain.ChatRequest) *domain.PolicyViolation {
	for _, statement := range policy.Statements {
		// Check if action matches
		actionMatches := false
		requestAction := e.getRequestAction(req)
		for _, action := range statement.Actions {
			if matchesPattern(requestAction, action) {
				actionMatches = true
				break
			}
		}
		if !actionMatches {
			continue
		}

		// Check if resource matches
		resourceMatches := false
		requestResource := e.getRequestResource(req)
		for _, resource := range statement.Resources {
			if matchesARN(requestResource, resource) {
				resourceMatches = true
				break
			}
		}
		if !resourceMatches {
			continue
		}

		// Check conditions
		conditionsMet := true
		for _, condition := range statement.Conditions {
			if !e.evaluateCondition(condition, tenant, req) {
				conditionsMet = false
				break
			}
		}
		if !conditionsMet {
			continue
		}

		// Statement matches
		if statement.Effect == domain.EffectDeny {
			return &domain.PolicyViolation{
				PolicyID:      policy.ID,
				PolicyName:    policy.Name,
				ViolationType: "policy_deny",
				Message:       fmt.Sprintf("Request denied by policy statement %s", statement.Sid),
				Severity:      "high",
			}
		}
	}

	return nil
}

// getRequestAction determines the action for a request
func (e *Engine) getRequestAction(req *domain.ChatRequest) string {
	if len(req.Tools) > 0 {
		return "modelgate:InvokeModelWithTools"
	}
	if req.ReasoningConfig != nil && req.ReasoningConfig.Enabled {
		return "modelgate:InvokeModelWithReasoning"
	}
	return "modelgate:InvokeModel"
}

// getRequestResource builds an ARN for the request
func (e *Engine) getRequestResource(req *domain.ChatRequest) string {
	// Format: arn:modelgate:model:provider/model-id
	return fmt.Sprintf("arn:modelgate:model:%s", req.Model)
}

// evaluateCondition evaluates a policy condition
func (e *Engine) evaluateCondition(condition domain.PolicyCondition, tenant *domain.Tenant, req *domain.ChatRequest) bool {
	value := e.getConditionValue(condition.Key, tenant, req)
	
	switch condition.Operator {
	case "StringEquals":
		for _, v := range condition.Values {
			if value == v {
				return true
			}
		}
		return false
		
	case "StringNotEquals":
		for _, v := range condition.Values {
			if value == v {
				return false
			}
		}
		return true
		
	case "StringLike":
		for _, v := range condition.Values {
			if matchesPattern(value, v) {
				return true
			}
		}
		return false
		
	case "NumericLessThan":
		// Implement numeric comparison
		return true
		
	case "NumericGreaterThan":
		return true
		
	default:
		return true
	}
}

// getConditionValue gets the value for a condition key
func (e *Engine) getConditionValue(key string, tenant *domain.Tenant, req *domain.ChatRequest) string {
	switch key {
	case "tenant:Tier":
		return string(tenant.Tier)
	case "tenant:Status":
		return string(tenant.Status)
	case "request:Model":
		return req.Model
	case "request:ToolCount":
		return fmt.Sprintf("%d", len(req.Tools))
	case "request:HasReasoning":
		if req.ReasoningConfig != nil && req.ReasoningConfig.Enabled {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

// EvaluateToolCall evaluates a tool call against policies
func (e *Engine) EvaluateToolCall(ctx context.Context, tenantID string, toolCall *domain.ToolCall) (*domain.PolicyEvaluationResult, error) {
	result := &domain.PolicyEvaluationResult{
		Allowed:         true,
		Violations:      []domain.PolicyViolation{},
		MatchedPolicies: []string{},
	}

	// Get policies
	policies, err := e.policyRepo.GetByTenant(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("getting policies: %w", err)
	}

	// Build tool ARN
	toolARN := fmt.Sprintf("arn:modelgate:tool:%s", toolCall.Function.Name)

	for _, policy := range policies {
		if !policy.Enabled || policy.Type != domain.PolicyTypeToolAccess {
			continue
		}

		for _, statement := range policy.Statements {
			// Check if action matches
			actionMatches := false
			for _, action := range statement.Actions {
				if matchesPattern("modelgate:CallTool", action) {
					actionMatches = true
					break
				}
			}
			if !actionMatches {
				continue
			}

			// Check if resource matches
			resourceMatches := false
			for _, resource := range statement.Resources {
				if matchesARN(toolARN, resource) {
					resourceMatches = true
					break
				}
			}
			if !resourceMatches {
				continue
			}

			result.MatchedPolicies = append(result.MatchedPolicies, policy.ID)

			if statement.Effect == domain.EffectDeny {
				result.Allowed = false
				result.Violations = append(result.Violations, domain.PolicyViolation{
					PolicyID:      policy.ID,
					PolicyName:    policy.Name,
					ViolationType: "tool_access_denied",
					Message:       fmt.Sprintf("Tool %s is not allowed", toolCall.Function.Name),
					Severity:      "high",
				})
			}
		}
	}

	return result, nil
}

// AnalyzePrompt performs prompt safety analysis
func (e *Engine) AnalyzePrompt(ctx context.Context, tenantID string, req *domain.ChatRequest) (*domain.PromptAnalysis, error) {
	analysis := &domain.PromptAnalysis{
		RequestID: req.RequestID,
		SafetyScore: domain.PromptSafetyScore{
			OverallScore:   1.0,
			CategoryScores: make(map[string]float64),
			IsSafe:         true,
			RiskLevel:      "low",
		},
		ContentFlags: []domain.ContentFlag{},
	}

	// Combine all text content
	var allText strings.Builder
	allText.WriteString(req.Prompt)
	allText.WriteString(" ")
	allText.WriteString(req.SystemPrompt)
	for _, msg := range req.Messages {
		for _, content := range msg.Content {
			if content.Type == "text" {
				allText.WriteString(" ")
				allText.WriteString(content.Text)
			}
		}
	}
	fullText := allText.String()

	// Check prompt length
	if e.config.EnableOutlierDetection {
		analysis.OutlierAnalysis = e.detectOutliers(fullText)
		if analysis.OutlierAnalysis.IsOutlier {
			analysis.SafetyScore.OverallScore -= 0.3
		}
	}

	// Check for injection patterns
	for _, pattern := range e.config.InjectionPatterns {
		re := e.getCompiledPattern(pattern)
		if re != nil && re.MatchString(fullText) {
			analysis.ContentFlags = append(analysis.ContentFlags, domain.ContentFlag{
				Category:    "injection",
				Subcategory: "prompt_injection",
				Confidence:  0.9,
				Description: "Potential prompt injection detected",
				Blocking:    true,
			})
			analysis.SafetyScore.OverallScore -= 0.5
			analysis.SafetyScore.CategoryScores["injection"] = 0.9
		}
	}

	// Check for blocked patterns
	for _, pattern := range e.config.BlockedPatterns {
		re := e.getCompiledPattern(pattern)
		if re != nil && re.MatchString(fullText) {
			analysis.ContentFlags = append(analysis.ContentFlags, domain.ContentFlag{
				Category:    "blocked_content",
				Subcategory: "pattern_match",
				Confidence:  1.0,
				Description: "Content matches blocked pattern",
				Blocking:    true,
			})
			analysis.SafetyScore.OverallScore -= 0.5
		}
	}

	// Determine if safe
	if analysis.SafetyScore.OverallScore < 0.5 {
		analysis.SafetyScore.IsSafe = false
		analysis.SafetyScore.RiskLevel = "high"
	} else if analysis.SafetyScore.OverallScore < 0.7 {
		analysis.SafetyScore.RiskLevel = "medium"
	}

	return analysis, nil
}

// detectOutliers performs outlier detection on the prompt
func (e *Engine) detectOutliers(text string) domain.OutlierAnalysis {
	analysis := domain.OutlierAnalysis{
		IsOutlier:      false,
		AnomalyScore:   0,
		OutlierReasons: []string{},
	}

	// Check length
	charCount := utf8.RuneCountInString(text)
	if charCount > e.config.MaxPromptLength {
		analysis.IsOutlier = true
		analysis.AnomalyScore = 0.9
		analysis.OutlierReasons = append(analysis.OutlierReasons, 
			fmt.Sprintf("Prompt length (%d) exceeds maximum (%d)", charCount, e.config.MaxPromptLength))
		analysis.OutlierType = domain.OutlierTypeLength
	}

	// Check for unusual patterns
	// High ratio of special characters
	specialCount := 0
	for _, r := range text {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == ' ') {
			specialCount++
		}
	}
	if charCount > 0 && float64(specialCount)/float64(charCount) > 0.3 {
		analysis.AnomalyScore = max(analysis.AnomalyScore, 0.6)
		analysis.OutlierReasons = append(analysis.OutlierReasons, "High ratio of special characters")
		analysis.OutlierType = domain.OutlierTypePattern
	}

	// Check for repeated patterns
	if hasRepeatedPatterns(text) {
		analysis.AnomalyScore = max(analysis.AnomalyScore, 0.5)
		analysis.OutlierReasons = append(analysis.OutlierReasons, "Repeated patterns detected")
	}

	if analysis.AnomalyScore >= e.config.AnomalyThreshold {
		analysis.IsOutlier = true
	}

	return analysis
}

// getCompiledPattern returns a compiled regex pattern, caching for reuse
func (e *Engine) getCompiledPattern(pattern string) *regexp.Regexp {
	e.cacheMu.RLock()
	if re, ok := e.patternCache[pattern]; ok {
		e.cacheMu.RUnlock()
		return re
	}
	e.cacheMu.RUnlock()

	e.cacheMu.Lock()
	defer e.cacheMu.Unlock()

	// Double-check after acquiring write lock
	if re, ok := e.patternCache[pattern]; ok {
		return re
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil
	}

	e.patternCache[pattern] = re
	return re
}

// Helper functions

// matchesPattern checks if a value matches a pattern with wildcards
func matchesPattern(value, pattern string) bool {
	if pattern == "*" {
		return true
	}

	// Convert wildcard pattern to regex
	regexPattern := "^" + regexp.QuoteMeta(pattern) + "$"
	regexPattern = strings.ReplaceAll(regexPattern, `\*`, ".*")
	regexPattern = strings.ReplaceAll(regexPattern, `\?`, ".")

	re, err := regexp.Compile(regexPattern)
	if err != nil {
		return false
	}

	return re.MatchString(value)
}

// matchesARN checks if a resource ARN matches a pattern ARN
func matchesARN(resource, pattern string) bool {
	if pattern == "*" {
		return true
	}

	resourceParts := strings.Split(resource, ":")
	patternParts := strings.Split(pattern, ":")

	if len(resourceParts) != len(patternParts) {
		// Allow pattern to have fewer parts with wildcard at end
		if len(patternParts) < len(resourceParts) && patternParts[len(patternParts)-1] == "*" {
			patternParts = append(patternParts, strings.Repeat("*:", len(resourceParts)-len(patternParts))+"*")
		} else {
			return false
		}
	}

	for i, pp := range patternParts {
		if pp == "*" {
			continue
		}
		if i >= len(resourceParts) {
			return false
		}
		if !matchesPattern(resourceParts[i], pp) {
			return false
		}
	}

	return true
}

// hasRepeatedPatterns checks for unusual repeated patterns
func hasRepeatedPatterns(text string) bool {
	if len(text) < 100 {
		return false
	}

	// Check for repeated substrings
	for windowSize := 5; windowSize <= 50; windowSize++ {
		counts := make(map[string]int)
		for i := 0; i <= len(text)-windowSize; i++ {
			substr := text[i : i+windowSize]
			counts[substr]++
			if counts[substr] > 10 {
				return true
			}
		}
	}

	return false
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// checkModelRestrictions checks if the requested model is allowed based on role policy
func (e *Engine) checkModelRestrictions(rolePolicy *domain.RolePolicy, req *domain.ChatRequest, result *domain.PolicyEvaluationResult) {
	restrictions := rolePolicy.ModelRestriction
	
	// If no allowed models are configured, allow all models
	if len(restrictions.AllowedModels) == 0 {
		return
	}
	
	// Check if model is in the allowed list
	allowed := false
	for _, m := range restrictions.AllowedModels {
		if matchesPattern(req.Model, m) {
			allowed = true
			break
		}
	}
	
	if !allowed {
		result.Allowed = false
		result.Violations = append(result.Violations, domain.PolicyViolation{
			PolicyID:      "role_policy",
			PolicyName:    "Role Model Restriction",
			ViolationType: "model_not_allowed",
			Message:       fmt.Sprintf("Model %s is not in the allowed list for this role", req.Model),
			Severity:      "high",
		})
	}
}

// extractProviderFromModel extracts the provider from a model ID
// e.g., "azure/gpt-4o" -> ProviderAzureOpenAI, "openai/gpt-4" -> ProviderOpenAI
func (e *Engine) extractProviderFromModel(model string) domain.Provider {
	modelLower := strings.ToLower(model)
	
	// Check for provider prefixes first
	if strings.HasPrefix(modelLower, "azure/") {
		return domain.ProviderAzureOpenAI
	}
	if strings.HasPrefix(modelLower, "aws-bedrock/") || strings.HasPrefix(modelLower, "bedrock/") {
		return domain.ProviderBedrock
	}
	if strings.HasPrefix(modelLower, "groq/") {
		return domain.ProviderGroq
	}
	if strings.HasPrefix(modelLower, "mistral/") {
		return domain.ProviderMistral
	}
	if strings.HasPrefix(modelLower, "together/") {
		return domain.ProviderTogether
	}
	if strings.HasPrefix(modelLower, "cohere/") {
		return domain.ProviderCohere
	}
	if strings.HasPrefix(modelLower, "ollama/") {
		return domain.ProviderOllama
	}
	
	// Infer from model name patterns
	if strings.HasPrefix(modelLower, "gpt-") || strings.HasPrefix(modelLower, "o1") || strings.HasPrefix(modelLower, "text-embedding") {
		return domain.ProviderOpenAI
	}
	if strings.HasPrefix(modelLower, "claude") {
		return domain.ProviderAnthropic
	}
	if strings.HasPrefix(modelLower, "gemini") {
		return domain.ProviderGemini
	}
	if strings.Contains(modelLower, "llama") || strings.Contains(modelLower, "mixtral") || strings.Contains(modelLower, "mistral") {
		// Could be multiple providers - check more specific patterns
		if strings.Contains(modelLower, "groq") {
			return domain.ProviderGroq
		}
		return domain.ProviderTogether // Default for open models
	}
	
	return domain.ProviderOpenAI // Default fallback
}

// GetAllowedModelsForRole returns the list of models allowed for a specific role
func (e *Engine) GetAllowedModelsForRole(ctx context.Context, tenantID, roleID string, availableModels []domain.ModelInfo) ([]domain.ModelInfo, error) {
	if e.rolePolicyRepo == nil {
		return availableModels, nil
	}
	
	rolePolicy, err := e.rolePolicyRepo.Get(tenantID, roleID)
	if err != nil {
		return availableModels, nil // Return all if policy not found
	}
	
	restrictions := rolePolicy.ModelRestriction
	
	// If no allowed models are configured, return all available models
	if len(restrictions.AllowedModels) == 0 && len(restrictions.AllowedProviders) == 0 {
		return availableModels, nil
	}
	
	var filteredModels []domain.ModelInfo
	
	for _, model := range availableModels {
		// Check provider restrictions first
		if len(restrictions.AllowedProviders) > 0 {
			providerAllowed := false
			for _, p := range restrictions.AllowedProviders {
				if p == model.Provider {
					providerAllowed = true
					break
				}
			}
			if !providerAllowed {
				continue
			}
		}
		
		// Check allowed models list
		if len(restrictions.AllowedModels) > 0 {
			allowed := false
			for _, m := range restrictions.AllowedModels {
				if matchesPattern(model.ID, m) || matchesPattern(model.Name, m) {
					allowed = true
					break
				}
			}
			if !allowed {
				continue
			}
		}
		
		filteredModels = append(filteredModels, model)
	}
	
	return filteredModels, nil
}

