// Package enforcement provides policy enforcement implementations
package enforcement

import (
	"context"
	"errors"
	"modelgate/internal/domain"
	"sync"
	"time"
)

// BudgetEnforcer enforces budget policies per role
type BudgetEnforcer struct {
	mu     sync.RWMutex
	usage  map[string]*BudgetUsage // tenantID:roleID -> usage
	alerts map[string]*AlertState  // tenantID:roleID -> alert state
}

// BudgetUsage tracks budget usage per role
type BudgetUsage struct {
	DailyCostUSD   float64
	WeeklyCostUSD  float64
	MonthlyCostUSD float64
	DayStart       time.Time
	WeekStart      time.Time
	MonthStart     time.Time
	RequestCount   int
}

// AlertState tracks alert state
type AlertState struct {
	WarningAlertSent  bool
	CriticalAlertSent bool
	ExceededAlertSent bool
	LastAlertAt       time.Time
}

// BudgetViolation represents a budget violation
type BudgetViolation struct {
	Type         string  // daily, weekly, monthly, per_request
	Limit        float64
	Current      float64
	Exceeded     bool
	AlertLevel   string // warning, critical, exceeded
}

// NewBudgetEnforcer creates a new budget enforcer
func NewBudgetEnforcer() *BudgetEnforcer {
	return &BudgetEnforcer{
		usage:  make(map[string]*BudgetUsage),
		alerts: make(map[string]*AlertState),
	}
}

// CheckBudget checks if the request is within budget
func (e *BudgetEnforcer) CheckBudget(ctx context.Context, policy domain.BudgetPolicy, tenantID, roleID string, estimatedCost float64) (*BudgetViolation, error) {
	if !policy.Enabled {
		return nil, nil
	}

	e.mu.RLock()
	key := tenantID + ":" + roleID
	usage := e.usage[key]
	e.mu.RUnlock()

	if usage == nil {
		usage = e.initUsage(key)
	}

	// Reset periods if needed
	e.resetPeriods(usage)

	// Check per-request limit
	if policy.MaxCostPerRequest > 0 && estimatedCost > policy.MaxCostPerRequest {
		return &BudgetViolation{
			Type:       "per_request",
			Limit:      policy.MaxCostPerRequest,
			Current:    estimatedCost,
			Exceeded:   true,
			AlertLevel: "exceeded",
		}, errors.New("request would exceed per-request cost limit")
	}

	// Check daily limit
	if policy.DailyLimitUSD > 0 {
		newDaily := usage.DailyCostUSD + estimatedCost
		if newDaily > policy.DailyLimitUSD {
			violation := &BudgetViolation{
				Type:       "daily",
				Limit:      policy.DailyLimitUSD,
				Current:    newDaily,
				Exceeded:   true,
				AlertLevel: "exceeded",
			}
			return violation, e.handleExceeded(policy, tenantID, roleID, violation)
		}
		
		// Check thresholds
		if ratio := newDaily / policy.DailyLimitUSD; ratio >= policy.CriticalThreshold {
			e.sendAlert(tenantID, roleID, "critical", "daily", policy)
		} else if ratio >= policy.AlertThreshold {
			e.sendAlert(tenantID, roleID, "warning", "daily", policy)
		}
	}

	// Check weekly limit
	if policy.WeeklyLimitUSD > 0 {
		newWeekly := usage.WeeklyCostUSD + estimatedCost
		if newWeekly > policy.WeeklyLimitUSD {
			violation := &BudgetViolation{
				Type:       "weekly",
				Limit:      policy.WeeklyLimitUSD,
				Current:    newWeekly,
				Exceeded:   true,
				AlertLevel: "exceeded",
			}
			return violation, e.handleExceeded(policy, tenantID, roleID, violation)
		}
	}

	// Check monthly limit
	if policy.MonthlyLimitUSD > 0 {
		newMonthly := usage.MonthlyCostUSD + estimatedCost
		if newMonthly > policy.MonthlyLimitUSD {
			violation := &BudgetViolation{
				Type:       "monthly",
				Limit:      policy.MonthlyLimitUSD,
				Current:    newMonthly,
				Exceeded:   true,
				AlertLevel: "exceeded",
			}
			return violation, e.handleExceeded(policy, tenantID, roleID, violation)
		}
		
		// Check thresholds
		if ratio := newMonthly / policy.MonthlyLimitUSD; ratio >= policy.CriticalThreshold {
			e.sendAlert(tenantID, roleID, "critical", "monthly", policy)
		} else if ratio >= policy.AlertThreshold {
			e.sendAlert(tenantID, roleID, "warning", "monthly", policy)
		}
	}

	return nil, nil
}

// RecordCost records the actual cost of a request
func (e *BudgetEnforcer) RecordCost(tenantID, roleID string, cost float64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	key := tenantID + ":" + roleID
	usage := e.usage[key]
	if usage == nil {
		usage = &BudgetUsage{
			DayStart:   startOfDay(time.Now()),
			WeekStart:  startOfWeek(time.Now()),
			MonthStart: startOfMonth(time.Now()),
		}
		e.usage[key] = usage
	}

	usage.DailyCostUSD += cost
	usage.WeeklyCostUSD += cost
	usage.MonthlyCostUSD += cost
	usage.RequestCount++
}

// GetUsage returns current budget usage for a role
func (e *BudgetEnforcer) GetUsage(tenantID, roleID string) *BudgetUsage {
	e.mu.RLock()
	defer e.mu.RUnlock()

	key := tenantID + ":" + roleID
	if usage, ok := e.usage[key]; ok {
		return usage
	}
	return nil
}

// initUsage initializes usage tracking for a role
func (e *BudgetEnforcer) initUsage(key string) *BudgetUsage {
	e.mu.Lock()
	defer e.mu.Unlock()

	if usage, ok := e.usage[key]; ok {
		return usage
	}

	now := time.Now()
	usage := &BudgetUsage{
		DayStart:   startOfDay(now),
		WeekStart:  startOfWeek(now),
		MonthStart: startOfMonth(now),
	}
	e.usage[key] = usage
	return usage
}

// resetPeriods resets usage counters for new periods
func (e *BudgetEnforcer) resetPeriods(usage *BudgetUsage) {
	now := time.Now()

	// Reset daily
	if startOfDay(now) != usage.DayStart {
		usage.DailyCostUSD = 0
		usage.DayStart = startOfDay(now)
	}

	// Reset weekly
	if startOfWeek(now) != usage.WeekStart {
		usage.WeeklyCostUSD = 0
		usage.WeekStart = startOfWeek(now)
	}

	// Reset monthly
	if startOfMonth(now) != usage.MonthStart {
		usage.MonthlyCostUSD = 0
		usage.MonthStart = startOfMonth(now)
	}
}

// handleExceeded handles budget exceeded based on policy
func (e *BudgetEnforcer) handleExceeded(policy domain.BudgetPolicy, tenantID, roleID string, violation *BudgetViolation) error {
	// Send alert
	e.sendAlert(tenantID, roleID, "exceeded", violation.Type, policy)

	switch policy.OnExceeded {
	case domain.BudgetActionBlock:
		return errors.New("budget exceeded: " + violation.Type + " limit")
	case domain.BudgetActionWarn:
		// Allow but with warning (violation is returned to caller)
		return nil
	case domain.BudgetActionThrottle:
		// TODO: Reduce rate limit
		return nil
	default:
		return nil
	}
}

// sendAlert sends a budget alert
func (e *BudgetEnforcer) sendAlert(tenantID, roleID, level, period string, policy domain.BudgetPolicy) {
	e.mu.Lock()
	defer e.mu.Unlock()

	key := tenantID + ":" + roleID
	state := e.alerts[key]
	if state == nil {
		state = &AlertState{}
		e.alerts[key] = state
	}

	// Check if already sent
	switch level {
	case "warning":
		if state.WarningAlertSent {
			return
		}
		state.WarningAlertSent = true
	case "critical":
		if state.CriticalAlertSent {
			return
		}
		state.CriticalAlertSent = true
	case "exceeded":
		if state.ExceededAlertSent {
			return
		}
		state.ExceededAlertSent = true
	}

	state.LastAlertAt = time.Now()

	// Send to webhook
	if policy.AlertWebhook != "" {
		go e.sendWebhook(policy.AlertWebhook, tenantID, roleID, level, period)
	}

	// Send emails
	for _, email := range policy.AlertEmails {
		go e.sendEmail(email, tenantID, roleID, level, period)
	}

	// Send Slack
	if policy.AlertSlack != "" {
		go e.sendSlack(policy.AlertSlack, tenantID, roleID, level, period)
	}
}

// sendWebhook sends an alert to a webhook (placeholder)
func (e *BudgetEnforcer) sendWebhook(url, tenantID, roleID, level, period string) {
	// TODO: Implement HTTP POST to webhook
	// payload := map[string]string{
	//     "tenant_id": tenantID,
	//     "role_id": roleID,
	//     "level": level,
	//     "period": period,
	// }
}

// sendEmail sends an alert email (placeholder)
func (e *BudgetEnforcer) sendEmail(email, tenantID, roleID, level, period string) {
	// TODO: Implement email sending
}

// sendSlack sends an alert to Slack (placeholder)
func (e *BudgetEnforcer) sendSlack(webhook, tenantID, roleID, level, period string) {
	// TODO: Implement Slack webhook
}

// Helper functions for time periods
func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func startOfWeek(t time.Time) time.Time {
	weekday := int(t.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	return startOfDay(t.AddDate(0, 0, -weekday+1))
}

func startOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

