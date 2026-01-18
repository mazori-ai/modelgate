package mcp

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// Regex to match invalid characters for tool names (OpenAI requires ^[a-zA-Z0-9_-]+$)
	invalidCharsRegex = regexp.MustCompile(`[^a-z0-9_-]+`)
)

// SanitizeServerName converts a server name to a slug-safe format
// Example: "Local MCP" → "local_mcp"
// Example: "GitHub Server v2.0" → "github_server_v2_0"
func SanitizeServerName(serverName string) string {
	// Convert to lowercase
	slug := strings.ToLower(serverName)

	// Replace invalid characters with underscore
	slug = invalidCharsRegex.ReplaceAllString(slug, "_")

	// Remove leading/trailing underscores
	slug = strings.Trim(slug, "_")

	// Collapse multiple underscores into one
	for strings.Contains(slug, "__") {
		slug = strings.ReplaceAll(slug, "__", "_")
	}

	return slug
}

// SanitizeToolName creates an OpenAI-compliant tool name with routing information
// Format: {server_slug}__{tool_name}
// Example: ("Local MCP", "calculator") → "local_mcp__calculator"
// Example: ("Tavily MCP", "search") → "tavily_mcp__search"
func SanitizeToolName(serverName, toolName string) string {
	serverSlug := SanitizeServerName(serverName)

	// Tool name should already be compliant, but sanitize just in case
	toolNameClean := strings.ToLower(toolName)
	toolNameClean = strings.ReplaceAll(toolNameClean, " ", "_")
	toolNameClean = invalidCharsRegex.ReplaceAllString(toolNameClean, "_")
	toolNameClean = strings.Trim(toolNameClean, "_")

	return fmt.Sprintf("%s__%s", serverSlug, toolNameClean)
}

// ParseToolName extracts server slug and tool name from a sanitized tool name
// Format: {server_slug}__{tool_name}
// Returns: (serverSlug, toolName, ok)
// Example: "local_mcp__calculator" → ("local_mcp", "calculator", true)
func ParseToolName(fullName string) (serverSlug, toolName string, ok bool) {
	parts := strings.Split(fullName, "__")
	if len(parts) != 2 {
		return "", "", false
	}

	return parts[0], parts[1], true
}

// IsValidToolName checks if a tool name follows the sanitized format
func IsValidToolName(name string) bool {
	_, _, ok := ParseToolName(name)
	return ok
}
