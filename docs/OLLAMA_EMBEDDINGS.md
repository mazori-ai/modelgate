# Ollama Embeddings for Semantic Tool Search

ModelGate uses **Ollama** as the default embedding provider for semantic tool search in the MCP Gateway. This document explains the architecture, installation, and configuration.

## Overview

### What is Ollama?

[Ollama](https://ollama.com) is an open-source tool for running large language models locally. It provides:
- **Free, local execution** - No API costs or rate limits
- **Privacy** - Data never leaves your infrastructure
- **Fast inference** - Optimized for Apple Silicon and NVIDIA GPUs
- **Simple API** - Compatible REST interface

### Why Embeddings?

ModelGate's MCP Gateway aggregates tools from multiple MCP servers. When you have 100s or 1000s of tools, finding the right one becomes challenging. Traditional keyword search fails when:

- User says "do math" but tool is named "calculator"
- User says "weather forecast" but tool description says "climate data"
- Tools have similar names but different purposes

**Semantic search with embeddings** solves this by understanding meaning, not just keywords.

### Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        ModelGate                                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐       │
│  │ MCP Server 1 │    │ MCP Server 2 │    │ MCP Server N │       │
│  │  10 tools    │    │  25 tools    │    │  50 tools    │       │
│  └──────┬───────┘    └──────┬───────┘    └──────┬───────┘       │
│         │                   │                   │                │
│         └───────────────────┼───────────────────┘                │
│                             ▼                                    │
│                   ┌─────────────────┐                           │
│                   │  MCP Gateway    │                           │
│                   │  (Tool Index)   │                           │
│                   └────────┬────────┘                           │
│                            │                                     │
│         ┌──────────────────┼──────────────────┐                 │
│         ▼                  ▼                  ▼                 │
│  ┌─────────────┐   ┌─────────────┐   ┌─────────────┐           │
│  │ Tool Name   │   │ Description │   │ Combined    │           │
│  │ Embedding   │   │ Embedding   │   │ Embedding   │           │
│  └─────────────┘   └─────────────┘   └─────────────┘           │
│                            │                                     │
│                            ▼                                     │
│                   ┌─────────────────┐                           │
│                   │   PostgreSQL    │                           │
│                   │  (Embeddings)   │                           │
│                   └─────────────────┘                           │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
                             │
                             ▼
                   ┌─────────────────┐
                   │     Ollama      │
                   │ nomic-embed-text│
                   │   (768 dims)    │
                   └─────────────────┘
```

## Installation

### macOS

```bash
# Install via Homebrew
brew install ollama

# Start Ollama server
ollama serve

# Pull the embedding model
ollama pull nomic-embed-text
```

### Linux

```bash
# Install Ollama
curl -fsSL https://ollama.com/install.sh | sh

# Start Ollama server
ollama serve &

# Pull the embedding model
ollama pull nomic-embed-text
```

### Windows

1. Download from [ollama.com/download](https://ollama.com/download)
2. Run the installer
3. Open Command Prompt:
```cmd
ollama serve
ollama pull nomic-embed-text
```

### Docker

```bash
# Run Ollama container
docker run -d \
  --name ollama \
  -p 11434:11434 \
  -v ollama_data:/root/.ollama \
  ollama/ollama

# Pull embedding model
docker exec ollama ollama pull nomic-embed-text
```

### Verify Installation

```bash
# Check Ollama is running
curl http://localhost:11434/api/tags

# Test embedding
curl http://localhost:11434/api/embeddings -d '{
  "model": "nomic-embed-text",
  "prompt": "hello world"
}' | jq '.embedding | length'
# Should output: 768
```

## Configuration

### ModelGate Configuration (`config.toml`)

```toml
# =============================================================================
# Embedder Configuration for Semantic Tool Search
# =============================================================================

[embedder]
type = "ollama"                          # "openai" or "ollama"
base_url = "http://localhost:11434"      # Ollama server URL
model = "nomic-embed-text"               # Embedding model
```

### Environment Variables (Alternative)

```bash
export MODELGATE_EMBEDDER_TYPE=ollama
export MODELGATE_EMBEDDER_URL=http://localhost:11434
export MODELGATE_EMBEDDER_MODEL=nomic-embed-text
```

### Docker Compose

When running with Docker Compose, use the service name:

```toml
[embedder]
type = "ollama"
base_url = "http://ollama:11434"
model = "nomic-embed-text"
```

## Embedding Models

### Recommended Models

| Model | Dimensions | Size | Speed | Quality |
|-------|------------|------|-------|---------|
| `nomic-embed-text` | 768 | 274MB | ⚡⚡⚡ | ⭐⭐⭐⭐ |
| `mxbai-embed-large` | 1024 | 669MB | ⚡⚡ | ⭐⭐⭐⭐⭐ |
| `all-minilm` | 384 | 45MB | ⚡⚡⚡⚡ | ⭐⭐⭐ |

**We recommend `nomic-embed-text`** for the best balance of speed and quality.

### Pulling Different Models

```bash
# High quality (slower)
ollama pull mxbai-embed-large

# Fast (lower quality)
ollama pull all-minilm

# Update config.toml
[embedder]
model = "mxbai-embed-large"
```

## How It Works

### 1. Tool Indexing (On MCP Server Sync)

When you connect and sync an MCP server:

```
MCP Server → Tools List → For each tool:
                          │
                          ├─► Embed tool name
                          ├─► Embed tool description
                          └─► Embed combined (name + description)
                                    │
                                    ▼
                              Store in PostgreSQL
```

### 2. Search Query (On tool_search call)

```
User Query: "do math calculations"
                │
                ▼
         Embed query with Ollama
                │
                ▼
         Cosine similarity with all tool embeddings
                │
                ▼
         Filter by visibility (ALLOW/SEARCH)
                │
                ▼
         Return tools with score >= 0.5
```

### 3. Similarity Scoring

Cosine similarity ranges from -1 to 1:
- **1.0** = Identical meaning
- **0.7+** = Strong match
- **0.5-0.7** = Moderate match
- **< 0.5** = Filtered out (irrelevant)

Example scores:
```
Query: "calculator"
├── calculator (0.73) ✓
├── get_weather (0.42) ✗
└── search_files (0.38) ✗

Query: "weather forecast"
├── get_weather (0.76) ✓
├── get_system_info (0.54) ✓
└── calculator (0.41) ✗
```

## Usage

### Tool Search via MCP

```bash
curl -X POST http://localhost:8080/mcp \
  -H "Authorization: Bearer your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "tools/call",
    "params": {
      "name": "tool_search",
      "arguments": {
        "query": "calculate math expressions",
        "max_results": 5
      }
    }
  }'
```

### Response Format

```json
{
  "query": "calculate math expressions",
  "total_found": 1,
  "tools": [
    {
      "name": "Local MCP/calculator",
      "description": "Perform mathematical calculations...",
      "inputSchema": {
        "type": "object",
        "properties": {
          "expression": {"type": "string"}
        }
      },
      "_metadata": {
        "server": "Local MCP",
        "category": "other",
        "score": 0.73
      }
    }
  ]
}
```

### Python Example

```python
import requests

def search_tools(query: str, api_key: str, max_results: int = 5):
    response = requests.post(
        "http://localhost:8080/mcp",
        headers={
            "Authorization": f"Bearer {api_key}",
            "Content-Type": "application/json"
        },
        json={
            "jsonrpc": "2.0",
            "id": 1,
            "method": "tools/call",
            "params": {
                "name": "tool_search",
                "arguments": {
                    "query": query,
                    "max_results": max_results
                }
            }
        }
    )
    
    result = response.json()
    text = result["result"]["content"][0]["text"]
    return json.loads(text)["tools"]

# Usage
tools = search_tools("file operations", "your-api-key")
for tool in tools:
    print(f"{tool['name']}: {tool['_metadata']['score']:.2f}")
```

## Troubleshooting

### Ollama Not Running

```bash
# Check if Ollama is running
curl http://localhost:11434/api/tags

# Start Ollama
ollama serve &
```

### Model Not Found

```bash
# List installed models
ollama list

# Pull missing model
ollama pull nomic-embed-text
```

### Empty Search Results

1. **Check embeddings are indexed:**
```sql
-- Connect to tenant database
SELECT name, LENGTH(combined_embedding) as embed_len 
FROM mcp_tools 
WHERE combined_embedding IS NOT NULL;
```

2. **Check tool visibility:**
```sql
SELECT t.name, p.visibility 
FROM mcp_tools t 
LEFT JOIN mcp_tool_permissions p ON t.id = p.tool_id;
```

3. **Re-sync MCP server** to regenerate embeddings:
   - Go to MCP Servers in UI
   - Click Sync on the server

### Slow Embedding Performance

```bash
# Check GPU usage (macOS)
sudo powermetrics --samplers gpu_power

# For NVIDIA GPUs, ensure CUDA is available
nvidia-smi
```

## Alternative: OpenAI Embeddings

If you prefer using OpenAI's embedding API:

```toml
[embedder]
type = "openai"
api_key = "sk-..."  # Or use OPENAI_API_KEY env var
model = "text-embedding-3-small"
```

**Comparison:**

| Feature | Ollama | OpenAI |
|---------|--------|--------|
| Cost | Free | $0.02/1M tokens |
| Privacy | Local | Cloud |
| Speed | Fast | Network latency |
| Quality | Good | Excellent |
| Offline | Yes | No |

## Future: LLM Reranking

For even better search results, we plan to add LLM-based reranking:

```
Query → Semantic Search (top 20) → LLM Rerank → Final Results (top 5)
```

This will use a small, fast LLM (like `qwen3:0.6b`) to understand query intent and reorder results. This is currently a placeholder in the code.

## References

- [Ollama Documentation](https://github.com/ollama/ollama)
- [nomic-embed-text Model](https://ollama.com/library/nomic-embed-text)
- [Anthropic Tool Search](https://platform.claude.com/docs/en/agents-and-tools/tool-use/tool-search-tool)
- [Cosine Similarity](https://en.wikipedia.org/wiki/Cosine_similarity)

