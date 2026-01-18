# ModelGate

<p align="center">
  <strong>🚀 Open Source LLM Gateway & MCP Server with Policy Enforcement, Semantic Tool Search & Intelligent Routing</strong>
</p>

<p align="center">
  <a href="https://github.com/mazori-ai/modelgate/blob/main/LICENSE"><img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" alt="License"></a>
  <a href="https://github.com/mazori-ai/modelgate/releases"><img src="https://img.shields.io/github/v/release/mazori-ai/modelgate" alt="Release"></a>
  <a href="https://github.com/mazori-ai/modelgate/actions"><img src="https://img.shields.io/github/actions/workflow/status/mazori-ai/modelgate/ci.yml?branch=main" alt="CI"></a>
  <a href="https://goreportcard.com/report/github.com/mazori-ai/modelgate"><img src="https://goreportcard.com/badge/github.com/mazori-ai/modelgate" alt="Go Report Card"></a>
</p>

<p align="center">
  <a href="#key-differentiators">Why ModelGate</a> •
  <a href="#features">Features</a> •
  <a href="#-mcp-gateway--semantic-tool-search">MCP Gateway</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#docker-deployment">Docker</a> •
  <a href="#api-usage">API</a> •
  <a href="CONTRIBUTING.md">Contributing</a>
</p>

---

> **Like Kong or Envoy, but purpose-built for LLMs** — Unified API for all providers, built-in security, MCP Gateway with semantic tool discovery, and intelligent routing.

---

## Key Differentiators

### 🔐 RBAC-First Architecture
ModelGate is built with **Role-Based Access Control at its core**, not bolted on as an afterthought. Every API request flows through a comprehensive policy engine that enforces 7 policy types out of the box: prompt security, tool access, rate limiting, model restrictions, MCP policies, semantic caching rules, and budget controls. Create roles, assign policies, and issue API keys with precise permissions—all without writing custom middleware.

### 🔌 True Multi-Provider with Local Model Support
Go beyond cloud providers. ModelGate natively supports **Ollama and other local model servers** alongside OpenAI, Anthropic, Google Gemini, AWS Bedrock, Azure OpenAI, Groq, Mistral, Together AI, and Cohere. Run cost-sensitive workloads on local models, sensitive data on air-gapped infrastructure, and complex reasoning on frontier models—all through a single, unified API with automatic provider key rotation and load balancing.

### 🔌 MCP Gateway with Tool Orchestration
ModelGate implements a full **Model Context Protocol (MCP) Gateway** that acts as a unified hub for AI agent tools. Unlike static tool definitions, MCP servers can register tools at runtime, enabling AI agents to discover new capabilities without code changes. The gateway handles:
- **Tool Registration** — Connect any MCP-compliant server (file systems, databases, APIs, custom tools)
- **Permission Enforcement** — Role-based tool access control (allow, deny, require approval)
- **Execution Logging** — Full audit trail of every tool invocation
- **Multi-Server Support** — Aggregate tools from multiple MCP servers into a single endpoint

### 🔎 Semantic Tool Search (`search_tools`)
Finding the right tool among hundreds shouldn't require exact name matching. ModelGate provides a built-in **`search_tools`** function that AI agents can invoke to discover relevant tools using natural language:

```json
{
  "name": "search_tools",
  "arguments": {
    "query": "something to read files from disk",
    "limit": 5
  }
}
```

The `search_tools` tool uses **vector embeddings** (via Ollama or OpenAI) to perform semantic similarity search across all registered tools. Ask for "something to send emails" and find `send_email`, `compose_message`, or `notify_user`—even if you don't know the exact tool name. This enables AI agents to:
- **Self-discover capabilities** at runtime without hardcoded tool lists
- **Find similar tools** when the primary tool is unavailable
- **Explore available tools** based on task descriptions

### 💾 Semantic Response Caching
Reduce costs and latency with **intelligent caching** that understands meaning, not just exact matches. Similar prompts hit the cache even when worded differently, dramatically reducing API costs for repetitive workloads. Configurable similarity thresholds let you balance cache hit rates against response accuracy.

### 🛡️ Built-in Prompt Security
Protect your LLM applications with **advanced prompt injection detection** using fuzzy pattern matching, homoglyph normalization, and synonym expansion. Detect and block PII, apply content filtering, and maintain comprehensive audit logs—all configurable per role.

---

## Features

### 🔌 Multi-Provider Support
- **OpenAI** - GPT-5.2, GPT-5.2-mini, GPT-5.2-nano, O3, O4-mini, embeddings
- **Anthropic** - Claude 4.5 Opus, Claude 4.5 Sonnet, Claude 4.5 Haiku
- **Google Gemini** - Gemini 2.5 Pro, Gemini 2.5 Flash, Gemini 2.0 Flash
- **AWS Bedrock** - Claude (Sonnet, Haiku), Nova Pro/Lite/Micro, Llama 3, Mistral
- **Azure OpenAI** - All Azure-hosted OpenAI models
- **Ollama** - Local models (Llama 3.2, Qwen 3, Mistral, etc.)
- **Groq** - Llama, Mixtral with ultra-low latency
- **Mistral AI** - Mistral Large, Medium, Small
- **Together AI, Cohere** - Various open-source models

### 🚀 OpenAI-Compatible API
Drop-in replacement for OpenAI API with full streaming support. Use with any OpenAI SDK.

### 🔧 MCP Gateway & Semantic Tool Search
- Full MCP server implementation for AI agent tool orchestration
- Built-in `search_tools` function for semantic tool discovery
- Connect multiple MCP servers (file systems, databases, APIs)
- Role-based tool permissions (allow, deny, require approval)
- Automatic tool indexing with vector embeddings

### 💾 Semantic Caching
Reduce costs and latency with intelligent response caching based on semantic similarity.

### 🔐 Granular Access Control
- Role-based access control (RBAC)
- API key management
- 7 policy types (security, rate limiting, model access, budget, etc.)

### 📊 Comprehensive Observability
- Request logs with full details
- Cost tracking by provider/model
- Agent dashboard for AI agent activities
- Prometheus metrics
- Audit trails

### Enterprise Features (Available in Enterprise Edition)
- Intelligent routing (cost/latency optimized)
- Resilience patterns (retries, circuit breakers, fallbacks)
- Multi-tenant isolation
- Advanced budget controls

---

## Quick Start

### Prerequisites

| Dependency | Version | Required | Purpose |
|------------|---------|----------|---------|
| **PostgreSQL** | 15+ | ✅ Yes | Database storage |
| **pgvector** | 0.5+ | ✅ Yes | Vector similarity search for semantic caching |
| **Ollama** | Latest | ✅ Yes | Embeddings for semantic search & tool discovery |
| **nomic-embed-text** | - | ✅ Yes | Default embedding model (pulled via Ollama) |
| **Go** | 1.21+ | For building | Or use Docker |
| **Node.js** | 18+ | For Web UI | Or use Docker |

> **Using Docker?** All dependencies (PostgreSQL, pgvector, Ollama, nomic-embed-text) are automatically set up with `make compose-up`. Skip to [Docker Deployment](#docker-deployment).

### Installing Ollama

Ollama is required for semantic features (tool search, semantic caching). It runs locally and provides embeddings via the `nomic-embed-text` model.

<details>
<summary><strong>macOS</strong></summary>

```bash
# Install Ollama
brew install ollama

# Start Ollama service
ollama serve &

# Pull the embedding model
ollama pull nomic-embed-text
```
</details>

<details>
<summary><strong>Linux</strong></summary>

```bash
# Install Ollama
curl -fsSL https://ollama.com/install.sh | sh

# Start Ollama service
systemctl start ollama
# Or run directly: ollama serve &

# Pull the embedding model
ollama pull nomic-embed-text
```
</details>

<details>
<summary><strong>Docker (Automatic)</strong></summary>

When using `make compose-up`, Ollama is automatically started and the embedding model is pulled:

```yaml
# From docker-compose.yml
ollama:
  image: ollama/ollama:latest
  
ollama-init:
  command: ollama pull nomic-embed-text  # Auto-pulls model
```

No manual setup required when using Docker Compose.
</details>

<details>
<summary><strong>Windows</strong></summary>

Download and install from [ollama.com/download](https://ollama.com/download)

Then in PowerShell:
```powershell
ollama pull nomic-embed-text
```
</details>

### Installing pgvector

pgvector is a PostgreSQL extension that enables vector similarity search, used by ModelGate for semantic caching. Without pgvector, semantic caching will fall back to exact-match caching.

<details>
<summary><strong>macOS (Homebrew)</strong></summary>

```bash
# Install PostgreSQL with pgvector
brew install postgresql@16 pgvector

# Start PostgreSQL
brew services start postgresql@16

# Enable extension in your database
psql -d modelgate -c "CREATE EXTENSION IF NOT EXISTS vector;"
```
</details>

<details>
<summary><strong>Ubuntu/Debian</strong></summary>

```bash
# Add PostgreSQL APT repository (if not already added)
sudo sh -c 'echo "deb http://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/pgdg.list'
wget --quiet -O - https://www.postgresql.org/media/keys/ACCC4CF8.asc | sudo apt-key add -
sudo apt update

# Install PostgreSQL and pgvector
sudo apt install postgresql-16 postgresql-16-pgvector

# Enable extension
sudo -u postgres psql -d modelgate -c "CREATE EXTENSION IF NOT EXISTS vector;"
```
</details>

<details>
<summary><strong>Docker (Automatic)</strong></summary>

The Docker Compose setup uses `ankane/pgvector` image which has pgvector pre-installed:

```yaml
# From docker-compose.yml
postgres:
  image: ankane/pgvector:latest  # pgvector included
```

No manual setup required when using Docker.
</details>

<details>
<summary><strong>From Source</strong></summary>

```bash
# Clone pgvector
git clone --branch v0.7.0 https://github.com/pgvector/pgvector.git
cd pgvector

# Build and install (requires PostgreSQL dev headers)
make
sudo make install

# Enable extension
psql -d modelgate -c "CREATE EXTENSION IF NOT EXISTS vector;"
```
</details>

<details>
<summary><strong>Verify Installation</strong></summary>

```bash
# Check if pgvector is available
psql -d modelgate -c "SELECT * FROM pg_available_extensions WHERE name = 'vector';"

# Check if extension is enabled
psql -d modelgate -c "SELECT extname, extversion FROM pg_extension WHERE extname = 'vector';"
```

Expected output:
```
 extname | extversion 
---------+------------
 vector  | 0.7.0
```
</details>

---

### Option 1: Docker (Recommended)

```bash
# Clone repository
git clone https://github.com/mazori-ai/modelgate.git
cd modelgate

# Copy environment config
cp .env.example .env

# Start services (with local Ollama for embeddings)
docker-compose --profile with-ollama up -d

# Or without Ollama (use OpenAI embeddings)
docker-compose up -d
```

Access the dashboard at **http://localhost:8080**

Default login: `admin@modelgate.local` / `admin123`

### Option 2: Manual Installation

```bash
# 1. Install PostgreSQL with pgvector
# macOS
brew install postgresql@16 pgvector

# Ubuntu/Debian
sudo apt install postgresql-16 postgresql-16-pgvector

# 2. Create database
createdb modelgate
psql modelgate -c "CREATE EXTENSION IF NOT EXISTS vector;"

# 3. Clone and build
git clone https://github.com/mazori-ai/modelgate.git
cd modelgate
make build

# 4. Start the server (auto-applies schema)
./bin/modelgate

# Web UI is automatically served at http://localhost:8080
# For frontend development with hot-reload:
cd web && pnpm install && pnpm run dev
```

---

## Docker Deployment

### Production Deployment

   ```bash
# 1. Configure environment
cp .env.example .env
vim .env  # Edit with your settings

# 2. Build and start
docker-compose build
docker-compose up -d

# 3. View logs
docker-compose logs -f modelgate
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTGRES_USER` | postgres | Database user |
| `POSTGRES_PASSWORD` | postgres | Database password |
| `POSTGRES_DB` | modelgate | Database name |
| `HTTP_PORT` | 8080 | Unified port (API + Web UI + GraphQL + MCP) |
| `EMBEDDER_TYPE` | ollama | `ollama` or `openai` |
| `EMBEDDER_URL` | http://ollama:11434 | Ollama server URL |
| `OPENAI_API_KEY` | - | Required for OpenAI embeddings |

### Docker Compose Profiles

```bash
# Standard (no local embeddings)
docker-compose up -d

# With Ollama for local embeddings
docker-compose --profile with-ollama up -d
```

---

## API Usage

ModelGate provides an **OpenAI-compatible API** on port 8080.

### Chat Completions

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer mg-your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "anthropic/claude-sonnet-4-20250514",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

### Streaming

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer mg-your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "Write a poem"}],
    "stream": true
  }'
```

### Using Model Aliases

```bash
# Use aliases defined in config.toml
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer mg-your-api-key" \
  -d '{"model": "claude", "messages": [...]}'  # → claude-sonnet-4
```

### Embeddings

```bash
curl http://localhost:8080/v1/embeddings \
  -H "Authorization: Bearer mg-your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "text-embedding-3-small",
    "input": "Hello world"
  }'
```

### List Models

```bash
curl http://localhost:8080/v1/models \
  -H "Authorization: Bearer mg-your-api-key"
```

### Tool Calling with MCP

```bash
# AI agent can discover and use tools dynamically
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer mg-your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude",
    "messages": [{"role": "user", "content": "Find tools that can help me read files"}],
    "tools": [
      {
        "type": "function",
        "function": {
          "name": "search_tools",
          "description": "Search for available tools by description",
          "parameters": {
            "type": "object",
            "properties": {
              "query": {"type": "string", "description": "Natural language description of what you need"},
              "limit": {"type": "integer", "description": "Max results to return"}
            },
            "required": ["query"]
          }
        }
      }
    ]
  }'
```

### Python SDK Example

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8080/v1",
    api_key="mg-your-api-key"
)

response = client.chat.completions.create(
    model="claude",  # Uses alias
    messages=[{"role": "user", "content": "Hello!"}]
)
print(response.choices[0].message.content)
```

---

## Configuration

### config.toml

The config file contains **server settings only**. Provider API keys and models are configured via the Dashboard UI.

```toml
[server]
http_port = 8080          # Unified port (API + Web UI + GraphQL + MCP)
bind_address = "0.0.0.0"

[database]
driver = "postgres"
host = "localhost"
port = 5432
user = "postgres"
password = "postgres"
database = "modelgate"
ssl_mode = "disable"

[embedder]
type = "ollama"                      # or "openai" for cloud embeddings
base_url = "http://localhost:11434"
model = "nomic-embed-text"

[telemetry]
log_level = "info"                   # debug, info, warn, error

# Model aliases (optional) - map friendly names to full model IDs
[aliases]
claude = "anthropic/claude-sonnet-4-20250514"
gpt4 = "openai/gpt-4o"
```

### Provider & Model Setup

Configure LLM providers in the **Dashboard UI**:

1. **Providers** → Add API keys for OpenAI, Anthropic, Bedrock, etc.
2. **Models** → Refresh models from providers, enable/disable as needed
3. **Roles** → Create roles with model access policies

---

## Policy Types

ModelGate supports 7 policy types for granular control:

| Policy | Description |
|--------|-------------|
| **Prompt Security** | Injection detection, PII protection, content filtering |
| **Tool Access** | Control MCP tool availability per role |
| **MCP Policies** | Manage MCP server access and discovery |
| **Rate Limiting** | Request/token limits per minute/hour/day |
| **Model Restrictions** | Control model access per role |
| **Semantic Caching** | Configure caching behavior |
| **Budget Controls** | Cost limits and alerts |

---

## Documentation

| Document | Description |
|----------|-------------|
| [MCP Gateway Design](docs/MCP_GATEWAY_DESIGN.md) | MCP integration architecture |
| [MCP Agent Integration](docs/MCP_AGENT_INTEGRATION.md) | Using MCP with AI agents |
| [Policy Enforcement](docs/POLICY_ENFORCEMENT.md) | Policy system details |
| [Prompt Security](docs/PROMPT_SECURITY_FRAMEWORK.md) | Security framework |
| [Metrics](docs/METRICS.md) | Prometheus metrics reference |

---

## Development

### Local Development

```bash
# Full setup (installs tools + dependencies)
make setup

# Run backend in dev mode
make dev

# Run frontend dev server (separate terminal)
make web-dev
```

### Build Targets

```bash
# Build everything (backend + frontend)
make all

# Build only the Go binary
make modelgate

# Build only the frontend
make web

# Build Docker images
make docker-build
```

### GraphQL Code Generation

When modifying the GraphQL schema (`internal/graphql/schema/*.graphql`), regenerate the Go code:

```bash
# Generate GraphQL resolvers and types
make graphql

# This generates code in:
#   - internal/graphql/generated/generated.go
#   - internal/graphql/model/models_gen.go
```

To install the gqlgen tool manually:

```bash
go install github.com/99designs/gqlgen@latest
```

### Testing

```bash
# Run all tests
make test

# Run tests with coverage report
make test-coverage

# Run linter
make lint
```

### All Make Targets

```bash
make help    # Show all available targets
```

---

## Community

- **GitHub Issues** — Bug reports and feature requests
- **GitHub Discussions** — Questions and ideas
- **Discord** — Real-time chat and community support
- **Twitter** — Updates and announcements

## Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

- Read the [Code of Conduct](CODE_OF_CONDUCT.md)
- Check [good first issues](https://github.com/mazori-ai/modelgate/labels/good%20first%20issue) for newcomers
- See [SECURITY.md](SECURITY.md) for reporting vulnerabilities

---

## License

Apache License 2.0 — see [LICENSE](LICENSE) for details.

---

## Enterprise Edition

Need multi-tenancy, intelligent routing, or advanced resilience features?

Contact us at **info@mazori.ai** for the Enterprise Edition:

- Multi-tenant isolation with separate databases
- Intelligent request routing (cost/latency optimized)
- Resilience patterns (retries, circuit breakers, fallbacks)
- Advanced budget controls with department-level limits
- Priority support and SLAs

---

<p align="center">
  <sub>Built with ❤️ by the ModelGate community</sub>
</p>
