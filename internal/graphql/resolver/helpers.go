package resolver

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"modelgate/internal/audit"
	"modelgate/internal/domain"
	"modelgate/internal/graphql/model"
	"modelgate/internal/mcp"
	"modelgate/internal/policy"
	"modelgate/internal/storage/postgres"

	"github.com/google/uuid"
)

// =============================================================================
// HELPER FUNCTIONS FOR POLICY CONVERSION
// =============================================================================

// convertInputToDomainPolicy converts GraphQL RolePolicyInput to domain.RolePolicy
func convertInputToDomainPolicy(input *model.RolePolicyInput, roleID string) *domain.RolePolicy {
	policy := &domain.RolePolicy{
		RoleID: roleID,
	}

	// Prompt policies
	if input.PromptPolicies != nil {
		pp := input.PromptPolicies
		policy.PromptPolicies = domain.PromptPolicies{}

		// PII Policy
		if pp.PiiPolicy != nil {
			policy.PromptPolicies.PIIPolicy = domain.PIIPolicyConfig{
				Enabled:     pp.PiiPolicy.Enabled != nil && *pp.PiiPolicy.Enabled,
				ScanInputs:  pp.PiiPolicy.ScanInputs != nil && *pp.PiiPolicy.ScanInputs,
				ScanOutputs: pp.PiiPolicy.ScanOutputs != nil && *pp.PiiPolicy.ScanOutputs,
				Categories:  pp.PiiPolicy.Categories,
			}
			if pp.PiiPolicy.OnDetection != nil {
				policy.PromptPolicies.PIIPolicy.OnDetection = domain.PIIAction(strings.ToLower(string(*pp.PiiPolicy.OnDetection)))
			}
			if pp.PiiPolicy.Redaction != nil {
				policy.PromptPolicies.PIIPolicy.Redaction = domain.PIIRedactionConfig{
					PlaceholderFormat:      derefStr(pp.PiiPolicy.Redaction.PlaceholderFormat),
					StoreOriginals:         pp.PiiPolicy.Redaction.StoreOriginals != nil && *pp.PiiPolicy.Redaction.StoreOriginals,
					RestoreInResponse:      pp.PiiPolicy.Redaction.RestoreInResponse != nil && *pp.PiiPolicy.Redaction.RestoreInResponse,
					ConsistentPlaceholders: pp.PiiPolicy.Redaction.ConsistentPlaceholders != nil && *pp.PiiPolicy.Redaction.ConsistentPlaceholders,
				}
			}
		}

		// Direct Injection Detection
		if pp.DirectInjectionDetection != nil {
			policy.PromptPolicies.DirectInjectionDetection = convertInjectionDetection(pp.DirectInjectionDetection)
		}

		// Indirect Injection Detection
		if pp.IndirectInjectionDetection != nil {
			policy.PromptPolicies.IndirectInjectionDetection = convertInjectionDetection(pp.IndirectInjectionDetection)
		}

		// Content Filtering
		if pp.ContentFiltering != nil {
			policy.PromptPolicies.ContentFiltering = domain.ContentFilteringConfig{
				Enabled:               pp.ContentFiltering.Enabled != nil && *pp.ContentFiltering.Enabled,
				BlockedCategories:     pp.ContentFiltering.BlockedCategories,
				CustomBlockedPatterns: pp.ContentFiltering.CustomBlockedPatterns,
				CustomAllowedPatterns: pp.ContentFiltering.CustomAllowedPatterns,
			}
			if pp.ContentFiltering.OnDetection != nil {
				policy.PromptPolicies.ContentFiltering.OnDetection = domain.DetectionAction(strings.ToLower(string(*pp.ContentFiltering.OnDetection)))
			}
		}

		// Input Bounds
		if pp.InputBounds != nil {
			policy.PromptPolicies.InputBounds = domain.InputBoundsConfig{
				Enabled:         pp.InputBounds.Enabled != nil && *pp.InputBounds.Enabled,
				MaxPromptLength: derefInt(pp.InputBounds.MaxPromptLength),
				MaxMessageCount: derefInt(pp.InputBounds.MaxMessageCount),
			}
		}

		// System Prompt Protection
		if pp.SystemPromptProtection != nil {
			policy.PromptPolicies.SystemPromptProtection = domain.SystemPromptProtectionConfig{
				Enabled:                  pp.SystemPromptProtection.Enabled != nil && *pp.SystemPromptProtection.Enabled,
				DetectExtractionAttempts: pp.SystemPromptProtection.DetectExtractionAttempts != nil && *pp.SystemPromptProtection.DetectExtractionAttempts,
				AddAntiExtractionSuffix:  pp.SystemPromptProtection.AddAntiExtractionSuffix != nil && *pp.SystemPromptProtection.AddAntiExtractionSuffix,
			}
		}

		// Output Validation
		if pp.OutputValidation != nil {
			policy.PromptPolicies.OutputValidation = domain.OutputValidationConfig{
				Enabled:             pp.OutputValidation.Enabled != nil && *pp.OutputValidation.Enabled,
				EnforceSchema:       pp.OutputValidation.EnforceSchema != nil && *pp.OutputValidation.EnforceSchema,
				DetectCodeExecution: pp.OutputValidation.DetectCodeExecution != nil && *pp.OutputValidation.DetectCodeExecution,
				DetectSecretLeakage: pp.OutputValidation.DetectSecretLeakage != nil && *pp.OutputValidation.DetectSecretLeakage,
				DetectPIILeakage:    pp.OutputValidation.DetectPIILeakage != nil && *pp.OutputValidation.DetectPIILeakage,
			}
		}

		// Structural Separation
		if pp.StructuralSeparation != nil {
			policy.PromptPolicies.StructuralSeparation = domain.StructuralSeparationConfig{
				Enabled:                  pp.StructuralSeparation.Enabled != nil && *pp.StructuralSeparation.Enabled,
				ForbidInstructionsInData: pp.StructuralSeparation.ForbidInstructionsInData != nil && *pp.StructuralSeparation.ForbidInstructionsInData,
				MarkRetrievedAsUntrusted: pp.StructuralSeparation.MarkRetrievedAsUntrusted != nil && *pp.StructuralSeparation.MarkRetrievedAsUntrusted,
			}
		}

		// Normalization
		if pp.Normalization != nil {
			policy.PromptPolicies.Normalization = domain.NormalizationConfig{
				Enabled:                  pp.Normalization.Enabled != nil && *pp.Normalization.Enabled,
				StripNullBytes:           pp.Normalization.StripNullBytes != nil && *pp.Normalization.StripNullBytes,
				RemoveInvisibleChars:     pp.Normalization.RemoveInvisibleChars != nil && *pp.Normalization.RemoveInvisibleChars,
				DetectMixedEncodings:     pp.Normalization.DetectMixedEncodings != nil && *pp.Normalization.DetectMixedEncodings,
				RejectSuspiciousEncoding: pp.Normalization.RejectSuspiciousEncoding != nil && *pp.Normalization.RejectSuspiciousEncoding,
			}
		}
	}

	// Tool policies
	if input.ToolPolicies != nil {
		tp := input.ToolPolicies
		policy.ToolPolicies = domain.ToolPolicies{
			AllowToolCalling:       tp.AllowToolCalling != nil && *tp.AllowToolCalling,
			AllowedTools:           tp.AllowedTools,
			BlockedTools:           tp.BlockedTools,
			MaxToolCallsPerRequest: derefInt(tp.MaxToolCallsPerRequest),
			RequireToolApproval:    tp.RequireToolApproval != nil && *tp.RequireToolApproval,
		}
	}

	// Rate limit policy
	if input.RateLimitPolicy != nil {
		rp := input.RateLimitPolicy
		policy.RateLimitPolicy = domain.RateLimitPolicy{
			RequestsPerMinute: derefInt(rp.RequestsPerMinute),
			RequestsPerHour:   derefInt(rp.RequestsPerHour),
			RequestsPerDay:    derefInt(rp.RequestsPerDay),
			TokensPerMinute:   int64(derefInt(rp.TokensPerMinute)),
			TokensPerHour:     int64(derefInt(rp.TokensPerHour)),
			TokensPerDay:      int64(derefInt(rp.TokensPerDay)),
		}
	}

	// Model restrictions
	if input.ModelRestrictions != nil {
		mr := input.ModelRestrictions
		allowedProviders := []domain.Provider{}
		for _, p := range mr.AllowedProviders {
			allowedProviders = append(allowedProviders, domain.Provider(strings.ToLower(string(p))))
		}
		policy.ModelRestriction = domain.ModelRestrictions{
			AllowedModels:       mr.AllowedModels,
			AllowedProviders:    allowedProviders,
			DefaultModel:        derefStr(mr.DefaultModel),
			MaxTokensPerRequest: int32(derefInt(mr.MaxTokensPerRequest)),
		}
	}

	// MCP Policies
	if input.McpPolicies != nil {
		mcp := input.McpPolicies
		policy.MCPPolicies = domain.MCPPolicies{
			Enabled:            mcp.Enabled != nil && *mcp.Enabled,
			AllowToolSearch:    mcp.AllowToolSearch != nil && *mcp.AllowToolSearch,
			AuditToolExecution: mcp.AuditToolExecution != nil && *mcp.AuditToolExecution,
		}
	}

	// Extended Policies - Caching
	if input.CachingPolicy != nil {
		cp := input.CachingPolicy
		policy.CachingPolicy = domain.CachingPolicy{
			Enabled:             cp.Enabled != nil && *cp.Enabled,
			SimilarityThreshold: derefFloat64(cp.SimilarityThreshold),
			TTLSeconds:          derefInt(cp.TTLSeconds),
			MaxCacheSize:        derefInt(cp.MaxCacheSize),
			CacheStreaming:      cp.CacheStreaming != nil && *cp.CacheStreaming,
			CacheToolCalls:      cp.CacheToolCalls != nil && *cp.CacheToolCalls,
			ExcludedModels:      cp.ExcludedModels,
			ExcludedPatterns:    cp.ExcludedPatterns,
			TrackSavings:        cp.TrackSavings != nil && *cp.TrackSavings,
		}
	}

	// Extended Policies - Routing
	if input.RoutingPolicy != nil {
		rp := input.RoutingPolicy
		policy.RoutingPolicy = domain.RoutingPolicy{
			Enabled:            rp.Enabled != nil && *rp.Enabled,
			AllowModelOverride: rp.AllowModelOverride != nil && *rp.AllowModelOverride,
		}
		if rp.Strategy != nil {
			policy.RoutingPolicy.Strategy = domain.RoutingStrategy(strings.ToLower(string(*rp.Strategy)))
		}
		if rp.CostConfig != nil {
			cc := rp.CostConfig
			policy.RoutingPolicy.CostConfig = &domain.CostRoutingConfig{
				SimpleQueryThreshold:  derefFloat64(cc.SimpleQueryThreshold),
				ComplexQueryThreshold: derefFloat64(cc.ComplexQueryThreshold),
				SimpleModels:          cc.SimpleModels,
				MediumModels:          cc.MediumModels,
				ComplexModels:         cc.ComplexModels,
			}
		}
		if rp.LatencyConfig != nil {
			lc := rp.LatencyConfig
			policy.RoutingPolicy.LatencyConfig = &domain.LatencyRoutingConfig{
				MaxLatencyMs:    derefInt(lc.MaxLatencyMs),
				PreferredModels: lc.PreferredModels,
			}
		}
		if rp.WeightedConfig != nil {
			wc := rp.WeightedConfig
			weights := make(map[string]int)
			for _, w := range wc.Weights {
				weights[w.Provider] = w.Weight
			}
			policy.RoutingPolicy.WeightedConfig = &domain.WeightedRoutingConfig{
				Weights: weights,
			}
		}
		if rp.CapabilityConfig != nil {
			cc := rp.CapabilityConfig
			taskModels := make(map[string][]string)
			for _, tm := range cc.TaskModels {
				taskModels[tm.TaskType] = tm.Models
			}
			policy.RoutingPolicy.CapabilityConfig = &domain.CapabilityRoutingConfig{
				TaskModels: taskModels,
			}
		}
	}

	// Extended Policies - Resilience
	if input.ResiliencePolicy != nil {
		rp := input.ResiliencePolicy
		policy.ResiliencePolicy = domain.ResiliencePolicy{
			Enabled:                 rp.Enabled != nil && *rp.Enabled,
			RetryEnabled:            rp.RetryEnabled != nil && *rp.RetryEnabled,
			MaxRetries:              derefInt(rp.MaxRetries),
			RetryBackoffMs:          derefInt(rp.RetryBackoffMs),
			RetryBackoffMax:         derefInt(rp.RetryBackoffMax),
			RetryJitter:             rp.RetryJitter != nil && *rp.RetryJitter,
			RetryOnTimeout:          rp.RetryOnTimeout != nil && *rp.RetryOnTimeout,
			RetryOnRateLimit:        rp.RetryOnRateLimit != nil && *rp.RetryOnRateLimit,
			RetryOnServerError:      rp.RetryOnServerError != nil && *rp.RetryOnServerError,
			RetryableErrors:         rp.RetryableErrors,
			FallbackEnabled:         rp.FallbackEnabled != nil && *rp.FallbackEnabled,
			CircuitBreakerEnabled:   rp.CircuitBreakerEnabled != nil && *rp.CircuitBreakerEnabled,
			CircuitBreakerThreshold: derefInt(rp.CircuitBreakerThreshold),
			CircuitBreakerTimeout:   derefInt(rp.CircuitBreakerTimeout),
			RequestTimeoutMs:        derefInt(rp.RequestTimeoutMs),
		}
		if rp.FallbackChain != nil {
			fallbackChain := make([]domain.FallbackConfig, 0, len(rp.FallbackChain))
			for _, fc := range rp.FallbackChain {
				fallbackChain = append(fallbackChain, domain.FallbackConfig{
					Provider:  fc.Provider,
					Model:     fc.Model,
					Priority:  fc.Priority,
					TimeoutMs: derefInt(fc.TimeoutMs),
				})
			}
			policy.ResiliencePolicy.FallbackChain = fallbackChain
		}
	}

	// Extended Policies - Budget
	if input.BudgetPolicy != nil {
		bp := input.BudgetPolicy
		policy.BudgetPolicy = domain.BudgetPolicy{
			Enabled:           bp.Enabled != nil && *bp.Enabled,
			DailyLimitUSD:     derefFloat64(bp.DailyLimitUsd),
			WeeklyLimitUSD:    derefFloat64(bp.WeeklyLimitUsd),
			MonthlyLimitUSD:   derefFloat64(bp.MonthlyLimitUsd),
			MaxCostPerRequest: derefFloat64(bp.MaxCostPerRequest),
			AlertThreshold:    derefFloat64(bp.AlertThreshold),
			CriticalThreshold: derefFloat64(bp.CriticalThreshold),
			AlertWebhook:      derefStr(bp.AlertWebhook),
			AlertEmails:       bp.AlertEmails,
			AlertSlack:        derefStr(bp.AlertSlack),
			SoftLimitEnabled:  bp.SoftLimitEnabled != nil && *bp.SoftLimitEnabled,
			SoftLimitBuffer:   derefFloat64(bp.SoftLimitBuffer),
		}
		if bp.OnExceeded != nil {
			policy.BudgetPolicy.OnExceeded = domain.BudgetExceededAction(strings.ToLower(string(*bp.OnExceeded)))
		}
	}

	return policy
}

func convertInjectionDetection(input *model.InjectionDetectionInput) domain.InjectionDetectionConfig {
	cfg := domain.InjectionDetectionConfig{
		Enabled: input.Enabled != nil && *input.Enabled,
	}
	if input.DetectionMethod != nil {
		cfg.DetectionMethod = domain.DetectionMethod(strings.ToLower(string(*input.DetectionMethod)))
	}
	if input.Sensitivity != nil {
		cfg.Sensitivity = domain.DetectionSensitivity(strings.ToLower(string(*input.Sensitivity)))
	}
	if input.OnDetection != nil {
		cfg.OnDetection = domain.DetectionAction(strings.ToLower(string(*input.OnDetection)))
	}
	if input.BlockThreshold != nil {
		cfg.BlockThreshold = *input.BlockThreshold
	}
	return cfg
}

// convertDomainPolicyToModel converts domain.RolePolicy to GraphQL model.RolePolicy
func convertDomainPolicyToModel(dp *domain.RolePolicy) *model.RolePolicy {
	if dp == nil {
		return nil
	}

	result := &model.RolePolicy{
		ID:        dp.RoleID,
		RoleID:    dp.RoleID,
		CreatedAt: dp.CreatedAt,
		UpdatedAt: dp.UpdatedAt,
	}

	// Prompt Policies
	pp := dp.PromptPolicies
	result.PromptPolicies = &model.PromptPolicies{
		StructuralSeparation: &model.StructuralSeparationConfig{
			Enabled:                  pp.StructuralSeparation.Enabled,
			TemplateFormat:           model.TemplateFormat(strings.ToUpper(string(pp.StructuralSeparation.TemplateFormat))),
			ForbidInstructionsInData: pp.StructuralSeparation.ForbidInstructionsInData,
			MarkRetrievedAsUntrusted: pp.StructuralSeparation.MarkRetrievedAsUntrusted,
		},
		Normalization: &model.NormalizationConfig{
			Enabled:                  pp.Normalization.Enabled,
			UnicodeNormalization:     model.UnicodeNormForm(string(pp.Normalization.UnicodeNormalization)),
			StripNullBytes:           pp.Normalization.StripNullBytes,
			RemoveInvisibleChars:     pp.Normalization.RemoveInvisibleChars,
			DetectMixedEncodings:     pp.Normalization.DetectMixedEncodings,
			RejectSuspiciousEncoding: pp.Normalization.RejectSuspiciousEncoding,
		},
		InputBounds: &model.InputBoundsConfig{
			Enabled:          pp.InputBounds.Enabled,
			MaxPromptLength:  pp.InputBounds.MaxPromptLength,
			MaxPromptTokens:  pp.InputBounds.MaxPromptTokens,
			MaxMessageCount:  pp.InputBounds.MaxMessageCount,
			MaxMessageLength: pp.InputBounds.MaxMessageLength,
		},
		DirectInjectionDetection:   convertDomainInjectionDetection(pp.DirectInjectionDetection),
		IndirectInjectionDetection: convertDomainInjectionDetection(pp.IndirectInjectionDetection),
		PiiPolicy: &model.PIIPolicyConfig{
			Enabled:     pp.PIIPolicy.Enabled,
			ScanInputs:  pp.PIIPolicy.ScanInputs,
			ScanOutputs: pp.PIIPolicy.ScanOutputs,
			Categories:  pp.PIIPolicy.Categories,
			OnDetection: model.PIIAction(strings.ToUpper(string(pp.PIIPolicy.OnDetection))),
			Redaction: &model.PIIRedactionConfig{
				PlaceholderFormat: pp.PIIPolicy.Redaction.PlaceholderFormat,
				StoreOriginals:    pp.PIIPolicy.Redaction.StoreOriginals,
				RestoreInResponse: pp.PIIPolicy.Redaction.RestoreInResponse,
			},
		},
		ContentFiltering: &model.ContentFilteringConfig{
			Enabled:               pp.ContentFiltering.Enabled,
			BlockedCategories:     pp.ContentFiltering.BlockedCategories,
			CustomBlockedPatterns: pp.ContentFiltering.CustomBlockedPatterns,
			OnDetection:           model.DetectionAction(strings.ToUpper(string(pp.ContentFiltering.OnDetection))),
		},
		SystemPromptProtection: &model.SystemPromptProtectionConfig{
			Enabled:                  pp.SystemPromptProtection.Enabled,
			DetectExtractionAttempts: pp.SystemPromptProtection.DetectExtractionAttempts,
			AddAntiExtractionSuffix:  pp.SystemPromptProtection.AddAntiExtractionSuffix,
		},
		OutputValidation: &model.OutputValidationConfig{
			Enabled:             pp.OutputValidation.Enabled,
			EnforceSchema:       pp.OutputValidation.EnforceSchema,
			DetectCodeExecution: pp.OutputValidation.DetectCodeExecution,
			DetectSecretLeakage: pp.OutputValidation.DetectSecretLeakage,
			DetectPIILeakage:    pp.OutputValidation.DetectPIILeakage,
			OnViolation:         model.OutputViolationAction(strings.ToUpper(string(pp.OutputValidation.OnViolation))),
		},
	}

	// Tool Policies
	tp := dp.ToolPolicies
	result.ToolPolicies = &model.ToolPolicies{
		AllowToolCalling:       tp.AllowToolCalling,
		AllowedTools:           tp.AllowedTools,
		BlockedTools:           tp.BlockedTools,
		MaxToolCallsPerRequest: tp.MaxToolCallsPerRequest,
		RequireToolApproval:    tp.RequireToolApproval,
	}

	// Rate Limit Policy
	rp := dp.RateLimitPolicy
	result.RateLimitPolicy = &model.RateLimitPolicy{
		RequestsPerMinute: rp.RequestsPerMinute,
		RequestsPerHour:   rp.RequestsPerHour,
		RequestsPerDay:    rp.RequestsPerDay,
		TokensPerMinute:   int(rp.TokensPerMinute),
		TokensPerHour:     int(rp.TokensPerHour),
		TokensPerDay:      int(rp.TokensPerDay),
	}

	// Model Restrictions
	mr := dp.ModelRestriction
	allowedProviders := make([]model.Provider, 0, len(mr.AllowedProviders))
	for _, p := range mr.AllowedProviders {
		allowedProviders = append(allowedProviders, model.Provider(strings.ToUpper(string(p))))
	}
	result.ModelRestrictions = &model.ModelRestrictions{
		AllowedModels:       mr.AllowedModels,
		AllowedProviders:    allowedProviders,
		DefaultModel:        mr.DefaultModel,
		MaxTokensPerRequest: int(mr.MaxTokensPerRequest),
	}

	// Extended Policies - Caching
	cp := dp.CachingPolicy
	result.CachingPolicy = &model.CachingPolicy{
		Enabled:             cp.Enabled,
		SimilarityThreshold: cp.SimilarityThreshold,
		TTLSeconds:          cp.TTLSeconds,
		MaxCacheSize:        cp.MaxCacheSize,
		CacheStreaming:      cp.CacheStreaming,
		CacheToolCalls:      cp.CacheToolCalls,
		ExcludedModels:      cp.ExcludedModels,
		ExcludedPatterns:    cp.ExcludedPatterns,
		TrackSavings:        cp.TrackSavings,
	}

	// Extended Policies - Routing
	rtp := dp.RoutingPolicy
	result.RoutingPolicy = &model.RoutingPolicy{
		Enabled:            rtp.Enabled,
		Strategy:           model.RoutingStrategy(strings.ToUpper(string(rtp.Strategy))),
		AllowModelOverride: rtp.AllowModelOverride,
	}
	if rtp.CostConfig != nil {
		result.RoutingPolicy.CostConfig = &model.CostRoutingConfig{
			SimpleQueryThreshold:  rtp.CostConfig.SimpleQueryThreshold,
			ComplexQueryThreshold: rtp.CostConfig.ComplexQueryThreshold,
			SimpleModels:          rtp.CostConfig.SimpleModels,
			MediumModels:          rtp.CostConfig.MediumModels,
			ComplexModels:         rtp.CostConfig.ComplexModels,
		}
	}
	if rtp.LatencyConfig != nil {
		result.RoutingPolicy.LatencyConfig = &model.LatencyRoutingConfig{
			MaxLatencyMs:    rtp.LatencyConfig.MaxLatencyMs,
			PreferredModels: rtp.LatencyConfig.PreferredModels,
		}
	}
	if rtp.WeightedConfig != nil {
		weights := make([]model.ProviderWeight, 0, len(rtp.WeightedConfig.Weights))
		for provider, weight := range rtp.WeightedConfig.Weights {
			weights = append(weights, model.ProviderWeight{
				Provider: provider,
				Weight:   weight,
			})
		}
		result.RoutingPolicy.WeightedConfig = &model.WeightedRoutingConfig{
			Weights: weights,
		}
	}
	if rtp.CapabilityConfig != nil {
		taskModels := make([]model.TaskModelMapping, 0, len(rtp.CapabilityConfig.TaskModels))
		for taskType, models := range rtp.CapabilityConfig.TaskModels {
			taskModels = append(taskModels, model.TaskModelMapping{
				TaskType: taskType,
				Models:   models,
			})
		}
		result.RoutingPolicy.CapabilityConfig = &model.CapabilityRoutingConfig{
			TaskModels: taskModels,
		}
	}

	// Extended Policies - Resilience
	rsp := dp.ResiliencePolicy
	result.ResiliencePolicy = &model.ResiliencePolicy{
		Enabled:                 rsp.Enabled,
		RetryEnabled:            rsp.RetryEnabled,
		MaxRetries:              rsp.MaxRetries,
		RetryBackoffMs:          rsp.RetryBackoffMs,
		RetryBackoffMax:         rsp.RetryBackoffMax,
		RetryJitter:             rsp.RetryJitter,
		RetryOnTimeout:          rsp.RetryOnTimeout,
		RetryOnRateLimit:        rsp.RetryOnRateLimit,
		RetryOnServerError:      rsp.RetryOnServerError,
		RetryableErrors:         rsp.RetryableErrors,
		FallbackEnabled:         rsp.FallbackEnabled,
		CircuitBreakerEnabled:   rsp.CircuitBreakerEnabled,
		CircuitBreakerThreshold: rsp.CircuitBreakerThreshold,
		CircuitBreakerTimeout:   rsp.CircuitBreakerTimeout,
		RequestTimeoutMs:        rsp.RequestTimeoutMs,
	}
	if rsp.FallbackChain != nil {
		fallbackChain := make([]model.FallbackConfig, 0, len(rsp.FallbackChain))
		for _, fc := range rsp.FallbackChain {
			fallbackChain = append(fallbackChain, model.FallbackConfig{
				Provider:  fc.Provider,
				Model:     fc.Model,
				Priority:  fc.Priority,
				TimeoutMs: fc.TimeoutMs,
			})
		}
		result.ResiliencePolicy.FallbackChain = fallbackChain
	}

	// Extended Policies - Budget
	bp := dp.BudgetPolicy
	result.BudgetPolicy = &model.BudgetPolicy{
		Enabled:           bp.Enabled,
		DailyLimitUsd:     bp.DailyLimitUSD,
		WeeklyLimitUsd:    bp.WeeklyLimitUSD,
		MonthlyLimitUsd:   bp.MonthlyLimitUSD,
		MaxCostPerRequest: bp.MaxCostPerRequest,
		AlertThreshold:    bp.AlertThreshold,
		CriticalThreshold: bp.CriticalThreshold,
		AlertWebhook:      bp.AlertWebhook,
		AlertEmails:       bp.AlertEmails,
		AlertSlack:        bp.AlertSlack,
		OnExceeded:        model.BudgetExceededAction(strings.ToUpper(string(bp.OnExceeded))),
		SoftLimitEnabled:  bp.SoftLimitEnabled,
		SoftLimitBuffer:   bp.SoftLimitBuffer,
	}

	// MCP Policies
	mcp := dp.MCPPolicies
	result.McpPolicies = &model.MCPPolicies{
		Enabled:            mcp.Enabled,
		AllowToolSearch:    mcp.AllowToolSearch,
		AuditToolExecution: mcp.AuditToolExecution,
	}

	return result
}

func convertDomainInjectionDetection(cfg domain.InjectionDetectionConfig) *model.InjectionDetectionConfig {
	return &model.InjectionDetectionConfig{
		Enabled:         cfg.Enabled,
		DetectionMethod: model.DetectionMethod(strings.ToUpper(string(cfg.DetectionMethod))),
		Sensitivity:     model.DetectionSensitivity(strings.ToUpper(string(cfg.Sensitivity))),
		OnDetection:     model.DetectionAction(strings.ToUpper(string(cfg.OnDetection))),
		BlockThreshold:  cfg.BlockThreshold,
	}
}

// Helper functions
func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}

func derefFloat64(f *float64) float64 {
	if f == nil {
		return 0.0
	}
	return *f
}

// GetAuditActor creates an audit.Actor from the context
func GetAuditActor(ctx context.Context) audit.Actor {
	userID := GetUserFromContext(ctx)
	email := GetUserEmailFromContext(ctx)
	actorType := "user"
	if IsAdminFromContext(ctx) {
		actorType = "admin"
	}
	if userID == "" {
		userID = "unknown"
		actorType = "system"
	}
	return audit.Actor{
		ID:    userID,
		Email: email,
		Type:  actorType,
	}
}

// convertTenantUserToModel converts postgres.TenantUser to GraphQL model
func convertTenantUserToModel(u *postgres.TenantUser) model.User {
	status := "active"
	if !u.IsActive {
		status = "inactive"
	}

	result := model.User{
		ID:        u.ID,
		Email:     u.Email,
		Name:      u.Name,
		Role:      u.Role,
		Status:    status,
		CreatedAt: u.CreatedAt,
	}

	if u.LastLoginAt != nil {
		result.LastLoginAt = u.LastLoginAt
	}
	if u.CreatedBy != "" {
		result.CreatedBy = &u.CreatedBy
	}
	if u.CreatedByEmail != "" {
		result.CreatedByEmail = &u.CreatedByEmail
	}

	return result
}

// convertDomainAuditLogToModel converts domain AuditLog to GraphQL model
func convertDomainAuditLogToModel(log domain.AuditLog) model.AuditLog {
	result := model.AuditLog{
		ID:           log.ID,
		Timestamp:    log.Timestamp,
		Action:       model.AuditAction(strings.ToUpper(string(log.Action))),
		ResourceType: model.AuditResourceType(strings.ToUpper(string(log.ResourceType))),
		ResourceID:   log.ResourceID,
		ActorID:      log.ActorID,
		ActorType:    log.ActorType,
		Details:      log.Details,
		OldValue:     log.OldValue,
		NewValue:     log.NewValue,
		Status:       log.Status,
	}

	if log.ResourceName != "" {
		result.ResourceName = &log.ResourceName
	}
	if log.ActorEmail != "" {
		result.ActorEmail = &log.ActorEmail
	}
	if log.IPAddress != "" {
		result.IPAddress = &log.IPAddress
	}
	if log.UserAgent != "" {
		result.UserAgent = &log.UserAgent
	}
	if log.ErrorMessage != "" {
		result.ErrorMessage = &log.ErrorMessage
	}

	return result
}

// =============================================================================
// PLAN LIMITS AND CONNECTION SETTINGS HELPERS
// =============================================================================

// convertDomainPlanLimitsToModel converts domain.PlanLimits to model.PlanLimits
func convertDomainPlanLimitsToModel(pl domain.PlanLimits) *model.PlanLimits {
	return &model.PlanLimits{
		MaxConnectionsPerProvider: pl.MaxConnectionsPerProvider,
		MaxIdleConnections:        pl.MaxIdleConnections,
		MaxConcurrentRequests:     pl.MaxConcurrentRequests,
		MaxQueuedRequests:         pl.MaxQueuedRequests,
		MaxRoles:                  pl.MaxRoles,
		MaxAPIKeys:                pl.MaxAPIKeys,
		MaxProviders:              pl.MaxProviders,
	}
}

// convertInputToDomainPlanLimits converts model.PlanLimitsInput to domain.PlanLimits
func convertInputToDomainPlanLimits(input *model.PlanLimitsInput) *domain.PlanLimits {
	if input == nil {
		return nil
	}

	pl := &domain.PlanLimits{}
	if input.MaxConnectionsPerProvider != nil {
		pl.MaxConnectionsPerProvider = *input.MaxConnectionsPerProvider
	}
	if input.MaxIdleConnections != nil {
		pl.MaxIdleConnections = *input.MaxIdleConnections
	}
	if input.MaxConcurrentRequests != nil {
		pl.MaxConcurrentRequests = *input.MaxConcurrentRequests
	}
	if input.MaxQueuedRequests != nil {
		pl.MaxQueuedRequests = *input.MaxQueuedRequests
	}
	if input.MaxRoles != nil {
		pl.MaxRoles = *input.MaxRoles
	}
	if input.MaxAPIKeys != nil {
		pl.MaxAPIKeys = *input.MaxAPIKeys
	}
	if input.MaxProviders != nil {
		pl.MaxProviders = *input.MaxProviders
	}
	return pl
}

// convertDomainConnectionSettingsToModel converts domain.ConnectionSettings to model.ConnectionSettings
func convertDomainConnectionSettingsToModel(cs domain.ConnectionSettings) *model.ConnectionSettings {
	return &model.ConnectionSettings{
		MaxConnections:     cs.MaxConnections,
		MaxIdleConnections: cs.MaxIdleConnections,
		IdleTimeoutSec:     cs.IdleTimeoutSec,
		RequestTimeoutSec:  cs.RequestTimeoutSec,
		EnableHTTP2:        cs.EnableHTTP2,
		EnableKeepAlive:    cs.EnableKeepAlive,
	}
}

// convertInputToDomainConnectionSettings converts model.ConnectionSettingsInput to domain.ConnectionSettings
func convertInputToDomainConnectionSettings(input *model.ConnectionSettingsInput) domain.ConnectionSettings {
	cs := domain.DefaultConnectionSettings()
	if input == nil {
		return cs
	}

	if input.MaxConnections != nil {
		cs.MaxConnections = *input.MaxConnections
	}
	if input.MaxIdleConnections != nil {
		cs.MaxIdleConnections = *input.MaxIdleConnections
	}
	if input.IdleTimeoutSec != nil {
		cs.IdleTimeoutSec = *input.IdleTimeoutSec
	}
	if input.RequestTimeoutSec != nil {
		cs.RequestTimeoutSec = *input.RequestTimeoutSec
	}
	if input.EnableHTTP2 != nil {
		cs.EnableHTTP2 = *input.EnableHTTP2
	}
	if input.EnableKeepAlive != nil {
		cs.EnableKeepAlive = *input.EnableKeepAlive
	}
	return cs
}

// getPlanCeiling returns connection settings ceiling based on tenant tier
func getPlanCeiling(tier domain.TenantTier) *model.ConnectionSettings {
	limits := domain.DefaultPlanLimits[tier]
	return &model.ConnectionSettings{
		MaxConnections:     limits.MaxConnectionsPerProvider,
		MaxIdleConnections: limits.MaxIdleConnections,
		IdleTimeoutSec:     90,  // Standard
		RequestTimeoutSec:  300, // Standard
		EnableHTTP2:        true,
		EnableKeepAlive:    true,
	}
}

// =============================================================================
// MCP TOOL SYNC HELPER
// =============================================================================

// syncMCPToolToRoleTools syncs an MCP tool to the role_tools table
// when it's approved for a role. Creates a role-specific tool entry with ALLOWED status.
func (r *mutationResolver) syncMCPToolToRoleTools(
	ctx context.Context,
	store *postgres.TenantStore,
	roleID string,
	mcpToolID string,
	actorID string,
) error {
	// 1. Get MCP tool details
	mcpTool, err := store.GetMCPTool(ctx, mcpToolID)
	if err != nil || mcpTool == nil {
		return fmt.Errorf("MCP tool not found: %w", err)
	}

	// Validate server name is available for sanitization
	if mcpTool.ServerName == "" {
		return fmt.Errorf("MCP tool has no server name, cannot sanitize tool name")
	}

	// Sanitize tool name with server prefix (e.g., "local_mcp__calculator")
	sanitizedName := mcp.SanitizeToolName(mcpTool.ServerName, mcpTool.Name)

	// 2. Compute schema hash
	schemaHash := policy.ComputeSchemaHash(mcpTool.InputSchema)

	// 3. Check if tool already exists for this role
	existingTool, _ := store.GetRoleToolByIdentity(ctx, roleID, sanitizedName, schemaHash)

	now := time.Now()

	if existingTool != nil {
		// Tool already exists for this role - update permission to ALLOWED
		if existingTool.Status != domain.ToolStatusAllowed {
			if err := store.SetRoleToolPermission(ctx, existingTool.ID, domain.ToolStatusAllowed, actorID, "", "Auto-approved from MCP tool permission"); err != nil {
				return fmt.Errorf("failed to update tool permission: %w", err)
			}
		}
		// Update last seen
		if err := store.UpdateRoleToolSeen(ctx, existingTool.ID); err != nil {
			slog.Warn("Failed to update tool seen", "tool_id", existingTool.ID, "error", err)
		}
	} else {
		// 4. Create new role tool with ALLOWED status
		roleTool := &domain.RoleTool{
			ID:             uuid.New().String(),
			RoleID:         roleID,
			Name:           sanitizedName,
			Description:    mcpTool.Description,
			SchemaHash:     schemaHash,
			Parameters:     mcpTool.InputSchema,
			FirstSeenBy:    actorID,
			SeenCount:      1,
			Category:       mcpTool.Category,
			Status:         domain.ToolStatusAllowed,
			DecidedBy:      actorID,
			DecidedAt:      &now,
			DecisionReason: "Auto-approved from MCP tool permission",
		}

		if err := store.CreateOrUpdateRoleTool(ctx, roleTool); err != nil {
			return fmt.Errorf("failed to create role tool: %w", err)
		}

		slog.Info("Synced MCP tool to role tools",
			"mcp_tool_id", mcpToolID,
			"mcp_tool_name", sanitizedName,
			"role_tool_id", roleTool.ID,
			"role_id", roleID,
		)
	}

	// 5. Also ensure tool_search is allowed for this role
	if err := r.ensureToolSearchAllowed(ctx, store, roleID, actorID); err != nil {
		slog.Warn("Failed to ensure tool_search is allowed",
			"role_id", roleID,
			"error", err,
		)
		// Don't fail the main operation
	}

	return nil
}

// ensureToolSearchAllowed ensures the special tool_search tool is added as an allowed
// role tool for the given role. This is called when any MCP tool is given
// ALLOW or SEARCH visibility so agents can use the search capability.
func (r *mutationResolver) ensureToolSearchAllowed(
	ctx context.Context,
	store *postgres.TenantStore,
	roleID string,
	actorID string,
) error {
	// tool_search has a fixed schema
	toolSearchSchema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Natural language description of the capability you're looking for",
			},
			"category": map[string]interface{}{
				"type":        "string",
				"description": "Optional category filter",
			},
			"max_results": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of tools to return (default: 5)",
			},
		},
		"required": []string{"query"},
	}

	description := "Search for available tools across all connected MCP servers. Use this to discover capabilities before using specific tools."
	schemaHash := policy.ComputeSchemaHash(toolSearchSchema)

	// Check if tool_search already exists for this role
	existingTool, _ := store.GetRoleToolByIdentity(ctx, roleID, "tool_search", schemaHash)

	now := time.Now()

	if existingTool != nil {
		// Already exists - ensure it's ALLOWED
		if existingTool.Status != domain.ToolStatusAllowed {
			if err := store.SetRoleToolPermission(ctx, existingTool.ID, domain.ToolStatusAllowed, actorID, "", "Auto-approved: enables MCP tool discovery via tool_search"); err != nil {
				return fmt.Errorf("failed to update tool_search permission: %w", err)
			}
		}
		return nil
	}

	// Create tool_search as a role tool with ALLOWED status
	toolSearch := &domain.RoleTool{
		ID:             uuid.New().String(),
		RoleID:         roleID,
		Name:           "tool_search",
		Description:    description,
		SchemaHash:     schemaHash,
		Parameters:     toolSearchSchema,
		FirstSeenBy:    actorID,
		SeenCount:      1,
		Category:       "search",
		Status:         domain.ToolStatusAllowed,
		DecidedBy:      actorID,
		DecidedAt:      &now,
		DecisionReason: "Auto-approved: enables MCP tool discovery via tool_search",
	}

	if err := store.CreateOrUpdateRoleTool(ctx, toolSearch); err != nil {
		return fmt.Errorf("failed to create tool_search: %w", err)
	}

	slog.Info("Ensured tool_search is allowed for role",
		"tool_id", toolSearch.ID,
		"role_id", roleID,
	)

	return nil
}
