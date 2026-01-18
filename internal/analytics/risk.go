// Package analytics provides analytics and risk assessment functionality.
package analytics

import (
	"math"
	"modelgate/internal/domain"
)

// RiskWeights defines the weight multipliers for different violation types
var RiskWeights = map[string]float64{
	"authentication": 5.0,  // Critical - auth failures are high risk
	"tool_access":    3.0,  // High - unauthorized tool access
	"rate_limit":     2.0,  // Medium - excessive requests
	"cost_limit":     1.5,  // Low-Medium - budget concerns
	"generic":        1.0,  // Baseline
}

// CalculateRiskScore calculates a risk score based on policy violations
// Score is normalized to 0-100 scale with weighted severity
func CalculateRiskScore(violations []domain.PolicyViolationRecord) domain.RiskAssessment {
	if len(violations) == 0 {
		return domain.RiskAssessment{
			Score:      0,
			Level:      "low",
			Violations: 0,
			Details:    make(map[string]float64),
		}
	}

	score := 0.0
	details := make(map[string]float64)

	// Calculate weighted score
	for _, v := range violations {
		weight := RiskWeights[v.ViolationType]
		if weight == 0 {
			weight = RiskWeights["generic"]
		}

		// Contribution = severity (1-5) * weight
		contribution := float64(v.Severity) * weight
		score += contribution
		details[v.ViolationType] += contribution
	}

	// Normalize to 0-100 scale
	// Assume a violation count of 50+ critical violations = 100 score
	// Using logarithmic scale for better distribution
	normalizedScore := math.Min((score / 10.0), 100.0)

	// Determine risk level based on score
	level := determineRiskLevel(normalizedScore)

	return domain.RiskAssessment{
		Score:      roundToTwoDecimals(normalizedScore),
		Level:      level,
		Violations: int64(len(violations)),
		Details:    roundDetails(details),
	}
}

// determineRiskLevel categorizes the risk score into levels
func determineRiskLevel(score float64) string {
	switch {
	case score >= 70:
		return "critical"
	case score >= 40:
		return "high"
	case score >= 20:
		return "medium"
	default:
		return "low"
	}
}

// roundToTwoDecimals rounds a float64 to 2 decimal places
func roundToTwoDecimals(value float64) float64 {
	return math.Round(value*100) / 100
}

// roundDetails rounds all values in the details map to 2 decimal places
func roundDetails(details map[string]float64) map[string]float64 {
	rounded := make(map[string]float64)
	for k, v := range details {
		rounded[k] = roundToTwoDecimals(v)
	}
	return rounded
}

// GetRiskLevel returns a human-readable risk level for a given score
func GetRiskLevel(score float64) string {
	return determineRiskLevel(score)
}

// GetRiskColor returns a color code for the risk level (useful for UI)
func GetRiskColor(level string) string {
	switch level {
	case "critical":
		return "#dc2626" // red-600
	case "high":
		return "#ea580c" // orange-600
	case "medium":
		return "#ca8a04" // yellow-600
	case "low":
		return "#16a34a" // green-600
	default:
		return "#6b7280" // gray-500
	}
}

// RiskThresholds defines score thresholds for each risk level
type RiskThresholds struct {
	Low      float64
	Medium   float64
	High     float64
	Critical float64
}

// DefaultRiskThresholds returns the default risk score thresholds
func DefaultRiskThresholds() RiskThresholds {
	return RiskThresholds{
		Low:      0,
		Medium:   20,
		High:     40,
		Critical: 70,
	}
}
