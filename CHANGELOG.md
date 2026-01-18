# Changelog

All notable changes to ModelGate will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial open source release
- OpenAI-compatible API (`/v1/chat/completions`, `/v1/embeddings`)
- Multi-provider support (OpenAI, Anthropic, Google, AWS Bedrock, Azure, Ollama)
- MCP (Model Context Protocol) Gateway with tool_search capability
- Policy enforcement pipeline (model, prompt, tool, rate limit)
- Prompt injection detection with fuzzy matching
- PII detection and redaction
- Role-based access control (RBAC)
- Tool permission management (ALLOW, DENY, SEARCH visibility)
- GraphQL management API
- React-based admin dashboard
- Docker support with single-image deployment
- PostgreSQL with pgvector for semantic caching

### Security
- Prompt injection detection with pattern matching
- PII scanning with configurable actions
- API key authentication
- Role-based policy enforcement

## [1.0.0] - 2026-01-17

### Added
- 🎉 Initial public release

### Features
- **Unified LLM Gateway**: Single API for multiple LLM providers
- **MCP Gateway**: First-class MCP protocol support
- **Policy Engine**: Modular, sequential policy enforcement
- **Security**: Built-in prompt injection and PII detection
- **RBAC**: Role and group-based access control
- **Tool Management**: Fine-grained tool permissions

### Supported Providers
- OpenAI (GPT-4, GPT-4o, o1, o3)
- Anthropic (Claude 3.5, Claude 4)
- Google (Gemini 2.0)
- AWS Bedrock
- Azure OpenAI
- Ollama (local models)

---

## Release Notes Format

### Added
New features and capabilities

### Changed
Changes to existing functionality

### Deprecated
Features that will be removed in future versions

### Removed
Features that have been removed

### Fixed
Bug fixes

### Security
Security-related changes and fixes

