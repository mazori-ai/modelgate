// Package policy provides policy enforcement for LLM operations.
package policy

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"modelgate/internal/domain"
)

// =============================================================================
// Policy Enforcement Service
// =============================================================================

// EnforcementService enforces policies for all LLM operations
type EnforcementService struct {
	rateLimiter *RateLimiter
}

// NewEnforcementService creates a new policy enforcement service
func NewEnforcementService() *EnforcementService {
	return &EnforcementService{
		rateLimiter: NewRateLimiter(),
	}
}

// EnforcementContext contains all information needed for policy enforcement
type EnforcementContext struct {
	TenantID string
	APIKeyID string
	ModelID  string
	Messages []domain.Message
	Tools    []domain.Tool
	RoleID   string
	GroupID  string
	Policy   *domain.RolePolicy
}

// PolicyViolation represents a policy violation error
type PolicyViolation struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Type    string `json:"type"` // model, prompt, tool, rate_limit
}

func (e *PolicyViolation) Error() string {
	return fmt.Sprintf("[%s] %s: %s", e.Type, e.Code, e.Message)
}

// =============================================================================
// Main Enforcement Method
// =============================================================================

// EnforcePolicy validates all policies before allowing an LLM operation
func (s *EnforcementService) EnforcePolicy(ctx context.Context, enfCtx *EnforcementContext) error {
	if enfCtx.Policy == nil {
		// No policy defined, allow all operations
		return nil
	}

	// 1. Model Restriction Check
	if err := s.validateModelRestrictions(enfCtx); err != nil {
		return err
	}

	// 2. Prompt Policy Check
	if err := s.validatePromptPolicies(enfCtx); err != nil {
		return err
	}

	// 3. Tool Policy Check
	if err := s.validateToolPolicies(enfCtx); err != nil {
		return err
	}

	// 4. Rate Limit Check
	if err := s.validateRateLimits(ctx, enfCtx); err != nil {
		return err
	}

	return nil
}

// =============================================================================
// 1. Model Restriction Validation
// =============================================================================

func (s *EnforcementService) validateModelRestrictions(enfCtx *EnforcementContext) error {
	restrictions := &enfCtx.Policy.ModelRestriction

	// If allowed models are configured, the model must be in the allowed list
	if len(restrictions.AllowedModels) > 0 {
		allowed := false
		for _, modelID := range restrictions.AllowedModels {
			if modelID == enfCtx.ModelID {
				allowed = true
				break
			}
		}
		if !allowed {
			return &PolicyViolation{
				Code:    "model_not_allowed",
				Message: fmt.Sprintf("Model '%s' is not in the allowed list", enfCtx.ModelID),
				Type:    "model",
			}
		}
	}

	return nil
}

// =============================================================================
// 2. Prompt Policy Validation
// =============================================================================

func (s *EnforcementService) validatePromptPolicies(enfCtx *EnforcementContext) error {
	promptPolicy := enfCtx.Policy.PromptPolicies

	// Policy feature flags
	piiEnabled := promptPolicy.PIIPolicy.Enabled
	piiCategories := promptPolicy.PIIPolicy.Categories
	piiAction := promptPolicy.PIIPolicy.OnDetection
	injectionEnabled := promptPolicy.DirectInjectionDetection.Enabled
	contentFilterEnabled := promptPolicy.ContentFiltering.Enabled

	// Extract text content from all messages
	var messageTexts []string
	totalLength := 0
	for _, msg := range enfCtx.Messages {
		msgText := s.extractMessageText(msg)
		messageTexts = append(messageTexts, msgText)
		totalLength += len(msgText)
	}

	// Validate max prompt length using InputBounds
	maxPromptLen := promptPolicy.InputBounds.MaxPromptLength
	if maxPromptLen > 0 && totalLength > maxPromptLen {
		return &PolicyViolation{
			Code:    "prompt_too_long",
			Message: fmt.Sprintf("Prompt length %d exceeds maximum %d", totalLength, maxPromptLen),
			Type:    "prompt",
		}
	}

	// Validate max message count using InputBounds
	maxMsgCount := promptPolicy.InputBounds.MaxMessageCount
	if maxMsgCount > 0 && len(enfCtx.Messages) > maxMsgCount {
		return &PolicyViolation{
			Code:    "too_many_messages",
			Message: fmt.Sprintf("Message count %d exceeds maximum %d", len(enfCtx.Messages), maxMsgCount),
			Type:    "prompt",
		}
	}

	// Check for blocked patterns (from ContentFiltering)
	// Only check the latest user message, not the entire conversation history
	if contentFilterEnabled && len(promptPolicy.ContentFiltering.CustomBlockedPatterns) > 0 {
		// Find the last user message
		var lastUserMessage string
		for i := len(enfCtx.Messages) - 1; i >= 0; i-- {
			if enfCtx.Messages[i].Role == "user" {
				lastUserMessage = s.extractMessageText(enfCtx.Messages[i])
				break
			}
		}

		if lastUserMessage != "" {
			for _, pattern := range promptPolicy.ContentFiltering.CustomBlockedPatterns {
				matched, _ := regexp.MatchString(pattern, lastUserMessage)
				if matched {
					return &PolicyViolation{
						Code:    "blocked_content",
						Message: "Prompt contains blocked content pattern",
						Type:    "prompt",
					}
				}
			}
		}
	}

	// Block injection attempts using DirectInjectionDetection
	// Only check the latest user message, not the entire conversation history
	if injectionEnabled {
		patternConfig := promptPolicy.DirectInjectionDetection.PatternDetection
		// If pattern detection is not explicitly enabled, use defaults
		if !patternConfig.Enabled {
			patternConfig = domain.PatternDetectionConfig{
				Enabled:                    true,
				DetectIgnoreInstructions:   true,
				DetectSystemPromptRequests: true,
				DetectRoleConfusion:        true,
				DetectJailbreakPhrases:     true,
				DetectToolCoercion:         false,
				DetectEncodingEvasion:      false,
			}
		}

		// Find the last user message (not assistant/system messages)
		var lastUserMessage string
		for i := len(enfCtx.Messages) - 1; i >= 0; i-- {
			if enfCtx.Messages[i].Role == "user" {
				lastUserMessage = s.extractMessageText(enfCtx.Messages[i])
				break
			}
		}

		// Only check the latest user message for injection
		if lastUserMessage != "" && s.detectPromptInjection(lastUserMessage, patternConfig) {
			action := promptPolicy.DirectInjectionDetection.OnDetection
			if action == "" || action == "block" || action == "BLOCK" {
				preview := lastUserMessage
				if len(preview) > 100 {
					preview = preview[:100] + "..."
				}
				slog.Info("Blocking request due to injection detection in latest user message", "message_length", len(lastUserMessage), "message_preview", preview)
				return &PolicyViolation{
					Code:    "injection_detected",
					Message: "Potential prompt injection detected",
					Type:    "prompt",
				}
			}
			// Log if action is WARN or LOG
			slog.Warn("Prompt injection detected but not blocked", "action", action)
		}
	}

	// PII scanning using PIIPolicy
	// Only scan the latest user message for input PII, not the entire conversation history
	if piiEnabled {
		// Find the last user message
		var lastUserMsgIndex = -1
		for i := len(enfCtx.Messages) - 1; i >= 0; i-- {
			if enfCtx.Messages[i].Role == "user" {
				lastUserMsgIndex = i
				break
			}
		}

		// Only scan the latest user message
		if lastUserMsgIndex >= 0 {
			msg := &enfCtx.Messages[lastUserMsgIndex]
			for j := range msg.Content {
				if msg.Content[j].Type == "text" && msg.Content[j].Text != "" {
					originalText := msg.Content[j].Text

					if piiFound := s.detectPII(originalText, piiCategories); piiFound != "" {
						// Check the action to take
						switch {
						case piiAction == "" || piiAction == "block" || piiAction == "BLOCK":
							return &PolicyViolation{
								Code:    "pii_detected",
								Message: fmt.Sprintf("Personal Identifiable Information detected: %s", piiFound),
								Type:    "prompt",
							}
						case piiAction == "redact" || piiAction == "REDACT":
							// Redact PII from the message with placeholders
							redactedText := s.redactPII(originalText, piiCategories)
							msg.Content[j].Text = redactedText
							slog.Debug("PII redacted from message", "category", piiFound)
						case piiAction == "rewrite" || piiAction == "REWRITE":
							// Rewrite PII with deterministic transformation
							rewrittenText := s.rewritePII(originalText, piiCategories)
							msg.Content[j].Text = rewrittenText
							slog.Debug("PII rewritten in message", "category", piiFound)
						case piiAction == "warn" || piiAction == "WARN":
							slog.Warn("PII detected but allowed by policy", "category", piiFound)
						case piiAction == "log" || piiAction == "LOG":
							slog.Info("PII detected", "category", piiFound)
						}
					}
				}
			}
		}
	}

	return nil
}

// extractMessageText extracts text content from a message's content blocks
func (s *EnforcementService) extractMessageText(msg domain.Message) string {
	var text strings.Builder
	for _, block := range msg.Content {
		if block.Type == "text" && block.Text != "" {
			text.WriteString(block.Text)
		}
	}
	return text.String()
}

// truncateForLog truncates a string for logging purposes
func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// detectPromptInjection detects prompt injection patterns based on policy configuration
// Uses the new fuzzy detection module for comprehensive pattern matching including:
// - Exact string matching (fast path)
// - Text normalization (homoglyphs, l33t speak)
// - Levenshtein-based fuzzy matching (catches typos)
// - Word-level Jaccard similarity (catches word reordering)
func (s *EnforcementService) detectPromptInjection(content string, patternConfig domain.PatternDetectionConfig) bool {
	lower := strings.ToLower(content)

	// Built-in whitelist of common false positive phrases
	// These are legitimate phrases that contain words like "ignore" but are not attacks
	builtInWhitelist := []string{
		"ignore case",
		"case insensitive",
		"ignore errors",
		"ignore warnings",
		"ignore whitespace",
		"ignore blank",
		"ignore empty",
		"ignore null",
		"ignore missing",
		"ignore duplicates",
		"ignore spaces",
		"ignore punctuation",
		"ignore formatting",
		"ignore comments",
		"ignore hidden",
		"ignore file",
		"ignore directory",
		"ignore pattern",
		"ignore line",
		"ignore row",
		"ignore column",
		"ignore field",
		"skip empty",
		"skip blank",
		"skip null",
		"discard empty",
		"discard null",
		"forget password",
		"forgot password",
		"forgot my password",
		"reset password",
	}

	// Check built-in whitelist first
	for _, phrase := range builtInWhitelist {
		if strings.Contains(lower, phrase) {
			slog.Debug("Content matches built-in whitelisted phrase, skipping injection detection", "phrase", phrase)
			return false
		}
	}

	// Check user-configured whitelisted phrases
	if len(patternConfig.WhitelistedPhrases) > 0 {
		for _, phrase := range patternConfig.WhitelistedPhrases {
			if strings.Contains(lower, strings.ToLower(phrase)) {
				slog.Debug("Content matches whitelisted phrase, skipping injection detection", "phrase", phrase)
				return false
			}
		}
	}

	// Use the fuzzy detection module for comprehensive pattern matching
	result := DetectPromptInjectionFuzzy(content, patternConfig)
	if result.Detected {
		slog.Info("Prompt injection detected",
			"pattern_type", result.PatternType,
			"matched_pattern", result.MatchedText,
			"method", result.Method,
			"confidence", result.Confidence,
			"content_length", len(content),
		)
		return true
	}

	// Also check for special tokens used by various models (these are exact matches)
	specialTokens := []string{
		"<|system|>",
		"<|im_start|>",
		"<|im_end|>",
		"</system>",
		"<|endoftext|>",
		"[INST]",
		"[/INST]",
		"<<SYS>>",
		"<</SYS>>",
	}
	for _, token := range specialTokens {
		if strings.Contains(content, token) {
			slog.Info("Special token detected in content", "token", token, "content_length", len(content))
			return true
		}
	}

	return false
}

// detectPII detects personally identifiable information
func (s *EnforcementService) detectPII(content string, categories []string) string {
	// Comprehensive PII patterns
	patterns := map[string]*regexp.Regexp{
		// Email: standard email format
		"email": regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`),
		// Phone: 408-325-6890, 408.325.6890, 408 325 6890, 4083256890, +1-408-325-6890
		"phone": regexp.MustCompile(`\b(?:\+?1[-.\s]?)?\d{3}[-.\s]?\d{3}[-.\s]?\d{4}\b`),
		// SSN: 123-45-6789
		"ssn": regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
		// Credit card: 4111-1111-1111-1111 or 4111111111111111
		"credit_card": regexp.MustCompile(`\b\d{4}[-\s]?\d{4}[-\s]?\d{4}[-\s]?\d{4}\b`),
		// IP address: IPv4 format like 192.168.1.1
		"ip_address": regexp.MustCompile(`\b(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\b`),
		// Date of birth: MM/DD/YYYY, MM-DD-YYYY, YYYY-MM-DD, DD/MM/YYYY
		"dob": regexp.MustCompile(`\b(?:\d{1,2}[-/]\d{1,2}[-/]\d{2,4}|\d{4}[-/]\d{1,2}[-/]\d{1,2})\b`),
		// Address: Street addresses like "123 Main St", "456 Oak Avenue, Apt 7"
		"address": regexp.MustCompile(`(?i)\b\d{1,5}\s+(?:[A-Za-z]+\s+){1,4}(?:street|st|avenue|ave|road|rd|boulevard|blvd|drive|dr|lane|ln|way|court|ct|circle|cir|place|pl|terrace|ter)\b(?:\s*,?\s*(?:apt|apartment|suite|ste|unit|#)\s*\d+[A-Za-z]?)?`),
		// Name: Common patterns like "Name: John Smith", "Full Name: Jane Doe"
		// Also matches "Mr./Mrs./Ms./Dr. Firstname Lastname"
		"name": regexp.MustCompile(`(?i)(?:(?:name|full\s*name|customer|patient|user)\s*:\s*([A-Z][a-z]+(?:\s+[A-Z][a-z]+)+))|(?:(?:Mr\.?|Mrs\.?|Ms\.?|Dr\.?|Miss)\s+[A-Z][a-z]+(?:\s+[A-Z][a-z]+)+)`),
	}

	// If no specific categories, check all
	if len(categories) == 0 {
		for category, pattern := range patterns {
			if pattern.MatchString(content) {
				return category
			}
		}
	} else {
		// Check only specified categories
		for _, category := range categories {
			if pattern, exists := patterns[category]; exists {
				if pattern.MatchString(content) {
					return category
				}
			}
		}
	}

	return ""
}

// redactPII replaces PII in content with redaction placeholders
func (s *EnforcementService) redactPII(content string, categories []string) string {
	// PII patterns with their replacement placeholders
	patterns := map[string]struct {
		regex       *regexp.Regexp
		placeholder string
	}{
		"email": {
			regex:       regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`),
			placeholder: "[EMAIL REDACTED]",
		},
		"phone": {
			regex:       regexp.MustCompile(`\b(?:\+?1[-.\s]?)?\d{3}[-.\s]?\d{3}[-.\s]?\d{4}\b`),
			placeholder: "[PHONE REDACTED]",
		},
		"ssn": {
			regex:       regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
			placeholder: "[SSN REDACTED]",
		},
		"credit_card": {
			regex:       regexp.MustCompile(`\b\d{4}[-\s]?\d{4}[-\s]?\d{4}[-\s]?\d{4}\b`),
			placeholder: "[CREDIT CARD REDACTED]",
		},
		"ip_address": {
			regex:       regexp.MustCompile(`\b(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\b`),
			placeholder: "[IP ADDRESS REDACTED]",
		},
		"dob": {
			regex:       regexp.MustCompile(`\b(?:\d{1,2}[-/]\d{1,2}[-/]\d{2,4}|\d{4}[-/]\d{1,2}[-/]\d{1,2})\b`),
			placeholder: "[DOB REDACTED]",
		},
		"address": {
			regex:       regexp.MustCompile(`(?i)\b\d{1,5}\s+(?:[A-Za-z]+\s+){1,4}(?:street|st|avenue|ave|road|rd|boulevard|blvd|drive|dr|lane|ln|way|court|ct|circle|cir|place|pl|terrace|ter)\b(?:\s*,?\s*(?:apt|apartment|suite|ste|unit|#)\s*\d+[A-Za-z]?)?`),
			placeholder: "[ADDRESS REDACTED]",
		},
		"name": {
			regex:       regexp.MustCompile(`(?i)(?:(?:name|full\s*name|customer|patient|user)\s*:\s*([A-Z][a-z]+(?:\s+[A-Z][a-z]+)+))|(?:(?:Mr\.?|Mrs\.?|Ms\.?|Dr\.?|Miss)\s+[A-Z][a-z]+(?:\s+[A-Z][a-z]+)+)`),
			placeholder: "[NAME REDACTED]",
		},
	}

	result := content

	// If no specific categories, redact all
	if len(categories) == 0 {
		for _, p := range patterns {
			result = p.regex.ReplaceAllString(result, p.placeholder)
		}
	} else {
		// Redact only specified categories
		for _, category := range categories {
			if p, exists := patterns[category]; exists {
				result = p.regex.ReplaceAllString(result, p.placeholder)
			}
		}
	}

	return result
}

// rewritePII transforms PII using deterministic character rotation
// Same input always produces the same output (unlike redaction which loses information)
// Useful for maintaining data utility while protecting privacy
func (s *EnforcementService) rewritePII(content string, categories []string) string {
	// PII patterns with their rewrite functions
	patterns := map[string]struct {
		regex       *regexp.Regexp
		rewriteFunc func(string) string
	}{
		"email": {
			regex:       regexp.MustCompile(`\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`),
			rewriteFunc: rewriteEmail,
		},
		"phone": {
			regex:       regexp.MustCompile(`\b(?:\+?1[-.\s]?)?\d{3}[-.\s]?\d{3}[-.\s]?\d{4}\b`),
			rewriteFunc: rewritePhone,
		},
		"ssn": {
			regex:       regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
			rewriteFunc: rewriteSSN,
		},
		"credit_card": {
			regex:       regexp.MustCompile(`\b\d{4}[-\s]?\d{4}[-\s]?\d{4}[-\s]?\d{4}\b`),
			rewriteFunc: rewriteCreditCard,
		},
		"ip_address": {
			regex:       regexp.MustCompile(`\b(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\b`),
			rewriteFunc: rewriteIPAddress,
		},
		"dob": {
			regex:       regexp.MustCompile(`\b(?:\d{1,2}[-/]\d{1,2}[-/]\d{2,4}|\d{4}[-/]\d{1,2}[-/]\d{1,2})\b`),
			rewriteFunc: rewriteDOB,
		},
		"address": {
			regex:       regexp.MustCompile(`(?i)\b\d{1,5}\s+(?:[A-Za-z]+\s+){1,4}(?:street|st|avenue|ave|road|rd|boulevard|blvd|drive|dr|lane|ln|way|court|ct|circle|cir|place|pl|terrace|ter)\b(?:\s*,?\s*(?:apt|apartment|suite|ste|unit|#)\s*\d+[A-Za-z]?)?`),
			rewriteFunc: rewriteAddress,
		},
		"name": {
			regex:       regexp.MustCompile(`(?i)(?:(?:name|full\s*name|customer|patient|user)\s*:\s*([A-Z][a-z]+(?:\s+[A-Z][a-z]+)+))|(?:(?:Mr\.?|Mrs\.?|Ms\.?|Dr\.?|Miss)\s+[A-Z][a-z]+(?:\s+[A-Z][a-z]+)+)`),
			rewriteFunc: rewriteName,
		},
	}

	result := content

	// If no specific categories, rewrite all
	if len(categories) == 0 {
		for _, p := range patterns {
			result = p.regex.ReplaceAllStringFunc(result, p.rewriteFunc)
		}
	} else {
		// Rewrite only specified categories
		for _, category := range categories {
			if p, exists := patterns[category]; exists {
				result = p.regex.ReplaceAllStringFunc(result, p.rewriteFunc)
			}
		}
	}

	return result
}

// rotateChar performs a deterministic character rotation using non-standard offsets
// Uses ROT7 for letters and ROT3 for digits (intentionally NOT ROT13 to avoid easy recognition)
// Same character always produces the same output
func rotateChar(c rune) rune {
	switch {
	case c >= 'a' && c <= 'z':
		return 'a' + (c-'a'+7)%26 // ROT7 - less recognizable than ROT13
	case c >= 'A' && c <= 'Z':
		return 'A' + (c-'A'+7)%26
	case c >= '0' && c <= '9':
		return '0' + (c-'0'+3)%10 // ROT3 for digits
	default:
		return c
	}
}

// rotateString applies character rotation to a string
func rotateString(s string) string {
	var result strings.Builder
	for _, c := range s {
		result.WriteRune(rotateChar(c))
	}
	return result.String()
}

// rewriteEmail transforms an email address using character rotation
// e.g., "john@example.com" -> "qvou@lehtwsl.jvt"
// e.g., "foo@bar.com" -> "mvv@ihy.jvt"
func rewriteEmail(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return rotateString(email)
	}

	// Rotate local part
	localPart := rotateString(parts[0])

	// Rotate domain but keep the TLD structure
	domainParts := strings.Split(parts[1], ".")
	for i := range domainParts {
		domainParts[i] = rotateString(domainParts[i])
	}

	return localPart + "@" + strings.Join(domainParts, ".")
}

// rewritePhone transforms a phone number to repeating digits
// The repeating digit is determined by the first digit of the original
// e.g., "408-325-6890" -> "888-888-8888" (based on 4+4=8)
func rewritePhone(phone string) string {
	// Extract just the digits
	var digits []rune
	for _, c := range phone {
		if c >= '0' && c <= '9' {
			digits = append(digits, c)
		}
	}

	if len(digits) == 0 {
		return phone
	}

	// Use first digit + 4 (mod 10) as the repeating digit
	// This ensures deterministic output: same input -> same output
	firstDigit := digits[0] - '0'
	repeatDigit := '0' + (firstDigit+4)%10

	// Reconstruct the phone with same format but repeating digits
	var result strings.Builder
	for _, c := range phone {
		if c >= '0' && c <= '9' {
			result.WriteRune(repeatDigit)
		} else {
			result.WriteRune(c) // Keep separators like -, ., space
		}
	}

	return result.String()
}

// rewriteSSN transforms an SSN to a deterministic fake SSN
// Uses repeating digits based on first digit
// e.g., "123-45-6789" -> "666-66-6666" (based on 1+5=6)
func rewriteSSN(ssn string) string {
	// Extract just the digits
	var digits []rune
	for _, c := range ssn {
		if c >= '0' && c <= '9' {
			digits = append(digits, c)
		}
	}

	if len(digits) == 0 {
		return ssn
	}

	// Use first digit + 5 (mod 10) as the repeating digit
	firstDigit := digits[0] - '0'
	repeatDigit := '0' + (firstDigit+5)%10

	// Reconstruct with same format
	var result strings.Builder
	for _, c := range ssn {
		if c >= '0' && c <= '9' {
			result.WriteRune(repeatDigit)
		} else {
			result.WriteRune(c)
		}
	}

	return result.String()
}

// rewriteCreditCard transforms a credit card to a deterministic fake number
// Uses repeating digits based on first digit
// e.g., "4111-1111-1111-1111" -> "9999-9999-9999-9999" (based on 4+5=9)
func rewriteCreditCard(cc string) string {
	// Extract just the digits
	var digits []rune
	for _, c := range cc {
		if c >= '0' && c <= '9' {
			digits = append(digits, c)
		}
	}

	if len(digits) == 0 {
		return cc
	}

	// Use first digit + 5 (mod 10) as the repeating digit
	firstDigit := digits[0] - '0'
	repeatDigit := '0' + (firstDigit+5)%10

	// Reconstruct with same format
	var result strings.Builder
	for _, c := range cc {
		if c >= '0' && c <= '9' {
			result.WriteRune(repeatDigit)
		} else {
			result.WriteRune(c)
		}
	}

	return result.String()
}

// rewriteIPAddress transforms an IP address to a deterministic fake IP
// e.g., "192.168.1.100" -> "10.10.10.10" (uses first octet mod to generate repeating pattern)
func rewriteIPAddress(ip string) string {
	// Parse the IP and generate a deterministic fake based on first octet
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return ip
	}

	// Use first octet to determine the fake IP pattern
	// This ensures same IP always maps to same fake IP
	firstOctet := 0
	for _, c := range parts[0] {
		if c >= '0' && c <= '9' {
			firstOctet = firstOctet*10 + int(c-'0')
		}
	}

	// Generate a repeating pattern based on first octet
	fakeOctet := (firstOctet + 7) % 200 // Keep it in valid range, add 7 for obfuscation
	if fakeOctet < 10 {
		fakeOctet = 10 // Ensure at least 10
	}

	return fmt.Sprintf("%d.%d.%d.%d", fakeOctet, fakeOctet, fakeOctet, fakeOctet)
}

// rewriteDOB transforms a date of birth to a deterministic fake date
// Preserves format but changes digits using rotation
// e.g., "12/25/1990" -> "45/58/4223"
func rewriteDOB(dob string) string {
	var result strings.Builder
	for _, c := range dob {
		if c >= '0' && c <= '9' {
			result.WriteRune('0' + (c-'0'+3)%10) // ROT3 for digits
		} else {
			result.WriteRune(c) // Keep separators
		}
	}
	return result.String()
}

// rewriteAddress transforms a street address using character rotation
// e.g., "123 Main Street" -> "456 Thpu Zayllz"
func rewriteAddress(address string) string {
	return rotateString(address)
}

// rewriteName transforms a name using character rotation
// Preserves structure like "Name: " prefix but rotates the actual name
// e.g., "Name: John Smith" -> "Name: Qvou Ztp ao"
// e.g., "Mr. John Smith" -> "Ty. Qvou Ztp ao"
func rewriteName(name string) string {
	// For names with prefix like "Name: John Smith", rotate only the name part
	if idx := strings.Index(strings.ToLower(name), ":"); idx != -1 {
		prefix := name[:idx+1]
		namePart := name[idx+1:]
		return prefix + rotateString(namePart)
	}
	// For names like "Mr. John Smith", rotate everything
	return rotateString(name)
}

// =============================================================================
// 3. Tool Policy Validation
// =============================================================================

func (s *EnforcementService) validateToolPolicies(enfCtx *EnforcementContext) error {
	toolPolicy := enfCtx.Policy.ToolPolicies

	// Check if tool calling is allowed at all
	if !toolPolicy.AllowToolCalling && len(enfCtx.Tools) > 0 {
		return &PolicyViolation{
			Code:    "tools_not_allowed",
			Message: "Tool calling is not allowed by policy",
			Type:    "tool",
		}
	}

	// Check max tool calls per request
	if toolPolicy.MaxToolCallsPerRequest > 0 && len(enfCtx.Tools) > toolPolicy.MaxToolCallsPerRequest {
		return &PolicyViolation{
			Code:    "too_many_tools",
			Message: fmt.Sprintf("Number of tools %d exceeds maximum %d", len(enfCtx.Tools), toolPolicy.MaxToolCallsPerRequest),
			Type:    "tool",
		}
	}

	// Validate each tool
	for _, tool := range enfCtx.Tools {
		toolName := tool.Function.Name

		// Check if tool is in allowed list
		if len(toolPolicy.AllowedTools) > 0 {
			allowed := false
			for _, allowedTool := range toolPolicy.AllowedTools {
				if allowedTool == toolName {
					allowed = true
					break
				}
			}
			if !allowed {
				return &PolicyViolation{
					Code:    "tool_not_allowed",
					Message: fmt.Sprintf("Tool '%s' is not in the allowed list", toolName),
					Type:    "tool",
				}
			}
		}

		// Check if tool is in blocked list
		if len(toolPolicy.BlockedTools) > 0 {
			for _, blockedTool := range toolPolicy.BlockedTools {
				if blockedTool == toolName {
					return &PolicyViolation{
						Code:    "tool_blocked",
						Message: fmt.Sprintf("Tool '%s' is blocked by policy", toolName),
						Type:    "tool",
					}
				}
			}
		}
	}

	return nil
}

// =============================================================================
// 4. Rate Limit Validation
// =============================================================================

func (s *EnforcementService) validateRateLimits(ctx context.Context, enfCtx *EnforcementContext) error {
	ratePolicy := enfCtx.Policy.RateLimitPolicy

	// Skip if no rate limits configured
	if ratePolicy.RequestsPerMinute == 0 && ratePolicy.TokensPerMinute == 0 {
		return nil
	}

	identifier := fmt.Sprintf("%s:%s", enfCtx.TenantID, enfCtx.APIKeyID)

	// Check requests per minute
	if ratePolicy.RequestsPerMinute > 0 {
		if !s.rateLimiter.AllowRequest(identifier, ratePolicy.RequestsPerMinute) {
			return &PolicyViolation{
				Code:    "rate_limit_exceeded",
				Message: fmt.Sprintf("Rate limit exceeded: %d requests per minute", ratePolicy.RequestsPerMinute),
				Type:    "rate_limit",
			}
		}
	}

	// Check tokens per minute (estimated based on message length)
	if ratePolicy.TokensPerMinute > 0 {
		estimatedTokens := s.estimateTokens(enfCtx.Messages)
		if !s.rateLimiter.AllowTokens(identifier, estimatedTokens, int(ratePolicy.TokensPerMinute)) {
			return &PolicyViolation{
				Code:    "token_rate_limit_exceeded",
				Message: fmt.Sprintf("Token rate limit exceeded: %d tokens per minute", ratePolicy.TokensPerMinute),
				Type:    "rate_limit",
			}
		}
	}

	return nil
}

// estimateTokens provides a rough estimate of token count
func (s *EnforcementService) estimateTokens(messages []domain.Message) int {
	totalChars := 0
	for _, msg := range messages {
		for _, block := range msg.Content {
			if block.Type == "text" {
				totalChars += len(block.Text)
			}
		}
	}
	// Rough estimation: 1 token ≈ 4 characters
	return totalChars / 4
}

// =============================================================================
// Rate Limiter Implementation
// =============================================================================

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	requestBuckets map[string]*tokenBucket
	tokenBuckets   map[string]*tokenBucket
	mu             sync.RWMutex
}

type tokenBucket struct {
	tokens     int
	lastRefill time.Time
	capacity   int
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		requestBuckets: make(map[string]*tokenBucket),
		tokenBuckets:   make(map[string]*tokenBucket),
	}

	// Background cleanup of old buckets
	go rl.cleanup()

	return rl
}

// AllowRequest checks if a request is allowed based on rate limit
func (rl *RateLimiter) AllowRequest(identifier string, ratePerMinute int) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, exists := rl.requestBuckets[identifier]
	if !exists {
		bucket = &tokenBucket{
			tokens:     ratePerMinute - 1,
			lastRefill: time.Now(),
			capacity:   ratePerMinute,
		}
		rl.requestBuckets[identifier] = bucket
		return true
	}

	// Update capacity if rate limit changed
	if bucket.capacity != ratePerMinute {
		bucket.capacity = ratePerMinute
		if bucket.tokens > ratePerMinute {
			bucket.tokens = ratePerMinute
		}
	}

	// Refill tokens based on time elapsed
	now := time.Now()
	if now.Sub(bucket.lastRefill) >= time.Minute {
		bucket.tokens = bucket.capacity
		bucket.lastRefill = now
	}

	// Check if tokens available
	if bucket.tokens > 0 {
		bucket.tokens--
		return true
	}

	return false
}

// AllowTokens checks if token consumption is allowed
func (rl *RateLimiter) AllowTokens(identifier string, tokensNeeded, ratePerMinute int) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	bucket, exists := rl.tokenBuckets[identifier]
	if !exists {
		bucket = &tokenBucket{
			tokens:     ratePerMinute - tokensNeeded,
			lastRefill: time.Now(),
			capacity:   ratePerMinute,
		}
		rl.tokenBuckets[identifier] = bucket
		return true
	}

	// Refill tokens based on time elapsed
	now := time.Now()
	elapsed := now.Sub(bucket.lastRefill)
	if elapsed >= time.Minute {
		bucket.tokens = bucket.capacity
		bucket.lastRefill = now
	}

	// Check if enough tokens available
	if bucket.tokens >= tokensNeeded {
		bucket.tokens -= tokensNeeded
		return true
	}

	return false
}

// cleanup removes old buckets periodically
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()

		// Clean request buckets
		for id, bucket := range rl.requestBuckets {
			if now.Sub(bucket.lastRefill) > 10*time.Minute {
				delete(rl.requestBuckets, id)
			}
		}

		// Clean token buckets
		for id, bucket := range rl.tokenBuckets {
			if now.Sub(bucket.lastRefill) > 10*time.Minute {
				delete(rl.tokenBuckets, id)
			}
		}

		rl.mu.Unlock()
	}
}
