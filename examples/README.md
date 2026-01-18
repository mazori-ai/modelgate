# ModelGate Examples

Python examples for interacting with the ModelGate API.

## Prerequisites

- Python 3.9+
- ModelGate server running on `http://localhost:8080`

## Quick Start

```bash
cd examples
pip install -r requirements.txt
export MODELGATE_API_KEY=mg_your_api_key  # Optional
python chat.py
```

## Examples

| File | Description |
|------|-------------|
| `chat.py` | Interactive chat client with streaming |
| `responses_basic.py` | Structured outputs (JSON schema) |
| `mcp_client.py` | MCP tool integration with LLM |
| `mcp_server.py` | MCP server with utility tools |
| `local_mcp_server.py` | Local MCP test server |
| `modelgate_mcp_agent.py` | Full MCP agent with tool orchestration |
| `modelgate_mcp_toolsearch_demo.py` | Semantic tool search demo |

## Chat Client

```bash
python chat.py
python chat.py --model openai/gpt-4o
python chat.py --api-key mg_your_key
```

Commands: `/help`, `/clear`, `/model`, `/models`, `/quit`

## Structured Outputs

```bash
python responses_basic.py
```

## MCP Tools

```bash
# Terminal 1: Start MCP server
python mcp_server.py

# Terminal 2: Run client
python mcp_client.py
```

Available tools: `read_file`, `write_file`, `list_directory`, `execute_command`, `parse_json`, `parse_csv`, `calculate`, `get_datetime`, `search_files`, `file_info`

## MCP Agent Demo

```bash
# Run the full MCP agent with tool orchestration
python modelgate_mcp_agent.py --api-key mg_your_key
```

## Semantic Tool Search Demo

Demonstrates the `search_tools` capability for discovering tools by natural language description:

```bash
python modelgate_mcp_toolsearch_demo.py --api-key mg_your_key
```

This demo shows how AI agents can discover tools dynamically using semantic search.

## Using OpenAI SDK

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8080/v1",
    api_key="your-api-key"  # Or leave empty
)

response = client.chat.completions.create(
    model="openai/gpt-4o",
    messages=[{"role": "user", "content": "Hello!"}]
)
print(response.choices[0].message.content)
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `MODELGATE_API_KEY` | API key for authentication |
