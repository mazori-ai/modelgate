// Package responses implements the /v1/responses endpoint logic.
package responses

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

// SchemaValidator validates JSON against JSON schemas
type SchemaValidator struct{}

// NewSchemaValidator creates a new schema validator
func NewSchemaValidator() *SchemaValidator {
	return &SchemaValidator{}
}

// Validate validates JSON content against a schema
func (v *SchemaValidator) Validate(content string, schema map[string]interface{}) error {
	// 1. Try to parse as-is
	var parsed interface{}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		// Try extracting from code blocks or mixed text
		extracted := extractJSON(content)
		if err := json.Unmarshal([]byte(extracted), &parsed); err != nil {
			return fmt.Errorf("response is not valid JSON: %w", err)
		}
		content = extracted
	}

	// 2. Validate against JSON schema
	schemaLoader := gojsonschema.NewGoLoader(schema)
	documentLoader := gojsonschema.NewStringLoader(content)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return fmt.Errorf("schema validation failed: %w", err)
	}

	if !result.Valid() {
		var errs []string
		for _, err := range result.Errors() {
			errs = append(errs, err.String())
		}
		return fmt.Errorf("response does not match schema: %s", strings.Join(errs, "; "))
	}

	return nil
}

// ParseAndValidate parses and validates JSON, returning the parsed object
func (v *SchemaValidator) ParseAndValidate(content string, schema map[string]interface{}) (map[string]interface{}, error) {
	// First validate
	if err := v.Validate(content, schema); err != nil {
		return nil, err
	}

	// Extract clean JSON if needed
	var parsed interface{}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		content = extractJSON(content)
		if err := json.Unmarshal([]byte(content), &parsed); err != nil {
			return nil, fmt.Errorf("failed to parse validated JSON: %w", err)
		}
	}

	// Convert to map
	result, ok := parsed.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("response is not a JSON object")
	}

	return result, nil
}

// extractJSON extracts JSON from markdown code blocks or mixed text
func extractJSON(content string) string {
	// Try extracting from ```json ... ``` or ``` ... ```
	codeBlockRe := regexp.MustCompile("```(?:json)?\\s*\\n([\\s\\S]*?)\\n```")
	if matches := codeBlockRe.FindStringSubmatch(content); len(matches) > 1 {
		extracted := strings.TrimSpace(matches[1])
		// Verify it's valid JSON before returning
		var test interface{}
		if err := json.Unmarshal([]byte(extracted), &test); err == nil {
			return extracted
		}
	}

	// Try finding JSON object boundaries { ... }
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start >= 0 && end > start {
		extracted := content[start : end+1]
		// Verify it's valid JSON before returning
		var test interface{}
		if err := json.Unmarshal([]byte(extracted), &test); err == nil {
			return extracted
		}
	}

	// Try finding JSON array boundaries [ ... ]
	start = strings.Index(content, "[")
	end = strings.LastIndex(content, "]")
	if start >= 0 && end > start {
		extracted := content[start : end+1]
		// Verify it's valid JSON before returning
		var test interface{}
		if err := json.Unmarshal([]byte(extracted), &test); err == nil {
			return extracted
		}
	}

	// Return as-is if no extraction worked
	return content
}
