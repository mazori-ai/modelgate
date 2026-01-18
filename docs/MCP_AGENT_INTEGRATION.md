# MCP Agent Integration Guide

This document explains how to expose ModelGate as an MCP server and how agents can connect and authenticate.

## Architecture Overview

```
┌─────────────────────┐          ┌─────────────────────┐          ┌─────────────────────┐
│   Agent/Claude      │ ──MCP──► │     ModelGate       │ ──MCP──► │ External MCP Servers│
│   (MCP Client)      │          │   (MCP Server)      │          │ (GitHub, Slack, etc)│
└─────────────────────┘          └─────────────────────┘          └─────────────────────┘
                                        │
                          Unified Endpoint: /mcp (Port 8080)
                          Same API key as LLM API
                          Tenant auto-detected from API key
                          
                          Exposes:
                          • tool_search - Find tools by description
                          • All discovered tools from connected MCP servers
```

## MCP Server Endpoint

ModelGate exposes a **unified MCP endpoint** on the same port as the LLM API:

```
http://localhost:8080/mcp
```

**Key Features:**
- Same port as chat completions API (8080)
- Same API key authentication
- **Tenant is automatically detected from the API key** - no need to specify it!

## Authentication

Agents authenticate using a **tenant API key** passed in the HTTP headers:

```
Authorization: Bearer <api-key>
```

**That's it!** The tenant is automatically detected from the API key.

### Authentication Flow

1. Agent sends request with API key
2. ModelGate validates the API key using `tenantService.ValidateAPIKey()`
3. Tenant is automatically detected from the `api_key_registry` table
4. Validates key is active and not expired
5. Returns authenticated client context with tenant/role/permissions

## MCP Protocol Support

ModelGate implements the MCP specification (version 2024-11-05):

### Supported Methods

| Method | Description |
|--------|-------------|
| `initialize` | Initialize connection with client info |
| `tools/list` | List all available tools for the authenticated role |
| `tools/call` | Execute a tool |
| `ping` | Health check |

### The `tool_search` Tool

ModelGate exposes a special `tool_search` tool that allows agents to discover tools from all connected MCP servers:

```json
{
  "name": "tool_search",
  "description": "Search for available tools across all connected MCP servers",
  "inputSchema": {
    "type": "object",
    "properties": {
      "query": {
        "type": "string",
        "description": "Natural language description of the capability"
      },
      "category": {
        "type": "string",
        "enum": ["messaging", "file-system", "database", "api", "git", "calendar", "shell", "search", "other"]
      },
      "max_results": {
        "type": "integer",
        "default": 5
      }
    },
    "required": ["query"]
  }
}
```

### Tool Naming Convention

Tools from connected MCP servers are exposed with a namespaced format:

```
{server-name}/{tool-name}
```

For example:
- `github/create_issue`
- `slack/send_message`
- `filesystem/read_file`

## Transport Options

### HTTP/SSE Transport (Primary)

For HTTP-based agents (tenant is auto-detected from API key):

**POST** `/mcp` - Send JSON-RPC requests
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/list",
  "params": {}
}
```

**GET** `/mcp` - SSE stream for server-to-client messages

### Stdio Transport (Local Agents)

For local agent integration via stdin/stdout, you can use the ModelGate MCP CLI wrapper:

```bash
# Set environment variables
export MODELGATE_URL=http://localhost:8080  # Same as LLM API
export MODELGATE_API_KEY=your-api-key       # Tenant auto-detected

# Run the wrapper
python claude_mcp_bridge.py
```

## Python Client Example

```python
#!/usr/bin/env python3
"""
ModelGate MCP Client - Connect agents to ModelGate's MCP Gateway

Note: Tenant is automatically detected from the API key!
"""

import json
import requests

class ModelGateMCPClient:
    def __init__(self, base_url: str, api_key: str):
        # Unified endpoint - tenant auto-detected from API key
        self.base_url = f"{base_url}/mcp"
        self.api_key = api_key
        self.headers = {
            "Authorization": f"Bearer {api_key}",
            "Content-Type": "application/json",
        }
        self.request_id = 0
    
    def _request(self, method: str, params: dict = None) -> dict:
        self.request_id += 1
        payload = {
            "jsonrpc": "2.0",
            "id": self.request_id,
            "method": method,
        }
        if params:
            payload["params"] = params
        
        response = requests.post(self.base_url, json=payload, headers=self.headers)
        response.raise_for_status()
        result = response.json()
        
        if "error" in result:
            raise Exception(f"MCP Error: {result['error']['message']}")
        
        return result.get("result", {})
    
    def initialize(self, client_name: str = "python-agent", client_version: str = "1.0.0"):
        """Initialize the MCP connection"""
        return self._request("initialize", {
            "protocolVersion": "2024-11-05",
            "capabilities": {},
            "clientInfo": {
                "name": client_name,
                "version": client_version
            }
        })
    
    def list_tools(self) -> list:
        """List all available tools"""
        result = self._request("tools/list")
        return result.get("tools", [])
    
    def call_tool(self, name: str, arguments: dict = None) -> dict:
        """Call a tool by name"""
        return self._request("tools/call", {
            "name": name,
            "arguments": arguments or {}
        })
    
    def search_tools(self, query: str, category: str = None, max_results: int = 5) -> dict:
        """Search for tools using natural language"""
        args = {"query": query, "max_results": max_results}
        if category:
            args["category"] = category
        return self.call_tool("tool_search", args)


if __name__ == "__main__":
    # Example usage - tenant is auto-detected from API key!
    client = ModelGateMCPClient(
        base_url="http://localhost:8080",  # Same as LLM API
        api_key="your-api-key-here"
    )
    
    # Initialize connection
    init_result = client.initialize()
    print(f"Connected to: {init_result['serverInfo']['name']}")
    
    # List available tools
    tools = client.list_tools()
    print(f"\nAvailable tools ({len(tools)}):")
    for tool in tools:
        print(f"  - {tool['name']}: {tool.get('description', 'No description')}")
    
    # Search for file-related tools
    search_result = client.search_tools("read and write files")
    print(f"\nSearch results:\n{search_result['content'][0]['text']}")
    
    # Call a tool (example)
    # result = client.call_tool("filesystem/list_directory", {"path": "."})
```

## Claude Desktop Integration

To use ModelGate as an MCP server with Claude Desktop, add this to your Claude Desktop config:

```json
{
  "mcpServers": {
    "modelgate": {
      "command": "python",
      "args": ["/path/to/claude_mcp_bridge.py"],
      "env": {
        "MODELGATE_URL": "http://localhost:8080",
        "MODELGATE_API_KEY": "your-api-key"
      }
    }
  }
}
```

**Note:** `MODELGATE_TENANT` is no longer required - the tenant is automatically detected from your API key!

### Bridge Script (claude_mcp_bridge.py)

See `examples/claude_mcp_bridge.py` for a full implementation. Here's a minimal example:

```python
#!/usr/bin/env python3
"""
MCP Bridge for Claude Desktop - Translates stdio to HTTP
Tenant is automatically detected from API key!
"""
import json
import os
import sys
import requests

BASE_URL = os.environ.get("MODELGATE_URL", "http://localhost:8080")
API_KEY = os.environ.get("MODELGATE_API_KEY", "")

def main():
    # Unified endpoint - tenant auto-detected from API key
    url = f"{BASE_URL}/mcp"
    headers = {
        "Authorization": f"Bearer {API_KEY}",
        "Content-Type": "application/json",
    }
    
    for line in sys.stdin:
        if not line.strip():
            continue
        
        try:
            request = json.loads(line)
            response = requests.post(url, json=request, headers=headers)
            result = response.json()
            print(json.dumps(result))
            sys.stdout.flush()
        except Exception as e:
            error_response = {
                "jsonrpc": "2.0",
                "id": request.get("id") if 'request' in dir() else None,
                "error": {"code": -32603, "message": str(e)}
            }
            print(json.dumps(error_response))
            sys.stdout.flush()

if __name__ == "__main__":
    main()
```

## Policy-Based Access Control

Tool access is controlled by the authenticated API key's role:

1. **PENDING** - Tool awaiting admin decision (blocked by default)
2. **ALLOWED** - Tool is allowed to be used
3. **DENIED** - Tool is explicitly denied (request blocked with error)
4. **REMOVED** - Tool is stripped from request silently (LLM won't see it)

Configure tool permissions in the Tenant Admin UI under **Roles → Tools**.

## Health Check

```bash
curl http://localhost:8080/health
# {"status":"healthy"}
```

## Error Handling

The MCP server returns standard JSON-RPC errors:

| Code | Message | Description |
|------|---------|-------------|
| -32700 | Parse error | Invalid JSON |
| -32600 | Invalid Request | Missing required fields |
| -32601 | Method not found | Unknown method |
| -32602 | Invalid params | Invalid method parameters |
| -32603 | Internal error | Server error |

## Token Savings with Tool Search

The `tool_search` approach significantly reduces token usage compared to sending all tool definitions in every request:

| Approach | Tools | Tokens per Request | Monthly Cost (1M requests) |
|----------|-------|-------------------|---------------------------|
| All Tools | 50 | ~15,000 | ~$4,500 |
| Tool Search | 50 | ~500 | ~$150 |
| **Savings** | | **~97%** | **~$4,350/month** |

## Security Considerations

1. **API Key Rotation**: Regularly rotate tenant API keys
2. **Role-Based Access**: Configure appropriate tool permissions per role
3. **Audit Logging**: All tool executions are logged
4. **Rate Limiting**: Configure rate limits in role policies
5. **TLS**: Use HTTPS in production

## Troubleshooting

### Connection Refused
- Check if ModelGate is running on port 8080
- Verify firewall rules

### Authentication Failed
- Verify API key is correct
- Check key is active and not expired

### Tool Not Found
- Verify tool exists and is connected
- Check role has permission to access the tool
- Ensure MCP server is connected in Tenant Admin

### Tool Denied
- Check tool permissions in Roles → Tools
- Contact tenant admin to approve the tool

