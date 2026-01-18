#!/usr/bin/env python3
"""
ModelGate MCP Gateway Chat Demo with Dynamic Tool Discovery

A full LLM chat application that demonstrates the Tool Search pattern:
1. Start with ONLY `tool_search` in context (minimal token usage)
2. LLM discovers tools on-demand using natural language search
3. Discovered tools are dynamically added to context
4. LLM then uses the discovered tools to complete tasks

This pattern is based on Anthropic's Tool Search Tool:
https://platform.claude.com/docs/en/agents-and-tools/tool-use/tool-search-tool

Key difference from Anthropic's server-side implementation:
- Anthropic returns `tool_reference` blocks that their API expands automatically
- ModelGate returns full tool specs that clients add to context manually
- ModelGate's approach works with ANY LLM, not just Anthropic

Architecture:
    ┌─────────────────┐              ┌─────────────────────────────────┐
    │  This Script    │              │          ModelGate :8080        │
    │  (Chat Client)  │              │                                 │
    └────────┬────────┘              │  /v1/chat/completions  (LLM)    │
             │                       │  /mcp                  (Tools)  │
             │  HTTP :8080           │                                 │
             │  Same API Key         │  Tenant auto-detected from key  │
             ├──────────────────────►│                                 │
             │                       └────────────────┬────────────────┘
             │                                        │
             │                           MCP (stdio/SSE) to external
             │                           MCP servers (file, git, etc)

Usage:
    # Start ModelGate first
    ./modelgate
    
    # Run the chat demo (tenant auto-detected from API key)
    python modelgate_mcp_demo.py --api-key your-key
    
    # With a specific model
    python modelgate_mcp_demo.py --api-key your-key --model openai/gpt-4o

Examples:
    User: "Calculate the square root of 144 plus pi"
    LLM: [searches for "calculator math"] -> [discovers calculator tool] -> [calls it]
    
    User: "Read the contents of README.md"
    LLM: [searches for "file read"] -> [discovers read_file tool] -> [calls it]
"""

import argparse
import json
import os
import sys
from typing import Any, Optional, Dict, List

try:
    import requests
except ImportError:
    print("Error: 'requests' library required. Install with: pip install requests")
    sys.exit(1)


class DynamicToolContext:
    """
    Manages dynamic tool context for LLM.
    
    Starts with only tool_search, then adds tools as they are discovered.
    This is ModelGate's client-side implementation of the Tool Search pattern.
    """
    
    def __init__(self):
        # tool_search is always available
        self.tool_search_spec = {
            "type": "function",
            "function": {
                "name": "tool_search",
                "description": "Search for available tools by natural language query. Returns tool specifications that will be added to your available tools. Use this FIRST when you need to perform an action you haven't used before.",
                "parameters": {
                    "type": "object",
                    "properties": {
                        "query": {
                            "type": "string",
                            "description": "Natural language description of the capability you're looking for (e.g., 'calculate math expressions', 'read files', 'get weather')"
                        },
                        "category": {
                            "type": "string",
                            "description": "Optional category filter: messaging, file-system, database, api, git, calendar, shell, search, other"
                        },
                        "max_results": {
                            "type": "integer",
                            "description": "Maximum number of tools to return (default: 5)",
                            "default": 5
                        }
                    },
                    "required": ["query"]
                }
            }
        }
        
        # Discovered tools (name -> spec)
        self.discovered_tools: Dict[str, dict] = {}
    
    def get_tools_for_llm(self) -> List[dict]:
        """Get all tools currently in context for LLM."""
        tools = [self.tool_search_spec]
        tools.extend(self.discovered_tools.values())
        return tools
    
    def add_tools_from_search(self, search_result_text: str) -> List[str]:
        """
        Parse tool_search result and add tools to context.
        
        Returns list of tool names that were added.
        """
        try:
            result = json.loads(search_result_text)
            tools = result.get("tools", [])
            added = []
            
            for tool in tools:
                name = tool.get("name")
                if name and name not in self.discovered_tools:
                    # Convert to LLM function format
                    self.discovered_tools[name] = {
                        "type": "function",
                        "function": {
                            "name": name,
                            "description": tool.get("description", ""),
                            "parameters": tool.get("inputSchema", {"type": "object", "properties": {}}),
                        }
                    }
                    # Include examples if available (per Anthropic's Tool Use Examples)
                    if "inputExamples" in tool:
                        self.discovered_tools[name]["function"]["input_examples"] = tool["inputExamples"]
                    added.append(name)
            
            return added
            
        except json.JSONDecodeError:
            return []
    
    def is_known_tool(self, name: str) -> bool:
        """Check if a tool is in current context."""
        return name == "tool_search" or name in self.discovered_tools
    
    def clear(self):
        """Clear discovered tools (keep only tool_search)."""
        self.discovered_tools.clear()
    
    def get_stats(self) -> str:
        """Get context stats string."""
        total = 1 + len(self.discovered_tools)
        return f"[{total} tools in context]"


class ModelGateLLMClient:
    """Client for ModelGate's LLM HTTP API (OpenAI-compatible)."""
    
    def __init__(
        self,
        base_url: str = "http://localhost:8080",
        api_key: str = "",
        model: str = "openai/gpt-4o",
    ):
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key
        self.model = model
        self.session = requests.Session()
        self.session.headers["Authorization"] = f"Bearer {api_key}"
        self.session.headers["Content-Type"] = "application/json"
    
    def chat(
        self,
        messages: list[dict],
        tools: Optional[list[dict]] = None,
        temperature: float = 0.7,
    ) -> dict:
        """Send a chat completion request with optional tools."""
        payload = {
            "model": self.model,
            "messages": messages,
            "temperature": temperature,
        }
        
        if tools:
            payload["tools"] = tools
        
        response = self.session.post(
            f"{self.base_url}/v1/chat/completions",
            json=payload,
            timeout=120,
        )
        
        if not response.ok:
            try:
                error_data = response.json()
                if "error" in error_data:
                    error_msg = error_data["error"].get("message", str(error_data["error"]))
                    error_type = error_data["error"].get("type", "unknown_error")
                    raise ChatError(f"[{error_type}] {error_msg}")
            except (ValueError, KeyError):
                pass
            response.raise_for_status()
        
        return response.json()


class ModelGateMCPClient:
    """
    Client for ModelGate's MCP Gateway (tool discovery and execution).
    
    Uses the unified /mcp endpoint on port 8080 (same as chat API).
    Tenant is automatically detected from the API key.
    """
    
    def __init__(
        self,
        base_url: str = "http://localhost:8080",
        api_key: str = "",
    ):
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key
        self.endpoint = f"{self.base_url}/mcp"
        self.headers = {
            "Authorization": f"Bearer {api_key}",
            "Content-Type": "application/json",
        }
        self.request_id = 0
        self.initialized = False
        self.server_info = {}
    
    def _next_id(self) -> int:
        self.request_id += 1
        return self.request_id
    
    def _request(self, method: str, params: Optional[dict] = None) -> dict:
        """Send a JSON-RPC request to the MCP Gateway."""
        payload = {
            "jsonrpc": "2.0",
            "id": self._next_id(),
            "method": method,
        }
        if params:
            payload["params"] = params
        
        try:
            response = requests.post(
                self.endpoint,
                json=payload,
                headers=self.headers,
                timeout=60,
            )
            
            if response.status_code == 401:
                raise MCPError(-32001, "MCP Gateway authentication failed - check your API key")
            if response.status_code == 404:
                raise MCPError(-32002, "MCP Gateway not available - ensure ModelGate is running")
            
            response.raise_for_status()
            result = response.json()
            
            if "error" in result and result["error"]:
                error = result["error"]
                raise MCPError(error.get("code", -1), error.get("message", "Unknown error"))
            
            return result.get("result", {})
            
        except requests.exceptions.ConnectionError:
            raise MCPError(-1, f"Cannot connect to MCP Gateway at {self.endpoint}")
        except requests.exceptions.Timeout:
            raise MCPError(-1, "MCP Gateway request timed out")
    
    def initialize(self) -> dict:
        """Initialize the MCP connection."""
        result = self._request("initialize", {
            "protocolVersion": "2024-11-05",
            "capabilities": {"tools": {}},
            "clientInfo": {"name": "ModelGate-MCP-Chat", "version": "1.0.0"},
        })
        
        self.initialized = True
        self.server_info = result.get("serverInfo", {})
        return result
    
    def call_tool(self, name: str, arguments: Optional[dict] = None) -> dict:
        """Execute a tool via the MCP Gateway."""
        if not self.initialized:
            self.initialize()
        
        return self._request("tools/call", {
            "name": name,
            "arguments": arguments or {},
        })
    
    def list_tools(self) -> list[dict]:
        """List all available tools (for admin/debug purposes)."""
        if not self.initialized:
            self.initialize()
        
        result = self._request("tools/list")
        return result.get("tools", [])


class ChatError(Exception):
    """Chat API error."""
    pass


class MCPError(Exception):
    """MCP protocol error."""
    def __init__(self, code: int, message: str):
        self.code = code
        self.message = message
        super().__init__(f"MCP Error {code}: {message}")


def extract_tool_result(result: dict) -> str:
    """Extract text content from MCP tool result."""
    content = result.get("content", [])
    texts = []
    for block in content:
        if block.get("type") == "text":
            texts.append(block.get("text", ""))
    return "\n".join(texts) if texts else str(result)


def run_chat_session(
    llm_client: ModelGateLLMClient,
    mcp_client: ModelGateMCPClient,
):
    """Run an interactive chat session with dynamic tool discovery."""
    
    # Dynamic tool context - starts with only tool_search
    tool_context = DynamicToolContext()
    
    print("\n" + "=" * 70)
    print("🚀 ModelGate MCP Gateway Chat with Dynamic Tool Discovery")
    print("=" * 70)
    print(f"LLM Model: {llm_client.model}")
    print(f"MCP Gateway: {mcp_client.server_info.get('name', 'ModelGate')}")
    print("-" * 70)
    print("🔑 KEY FEATURE: Dynamic Tool Discovery")
    print("   • Only 'tool_search' is loaded initially (saves tokens)")
    print("   • LLM discovers tools on-demand using natural language")
    print("   • Discovered tools are added to context automatically")
    print("-" * 70)
    print("Commands: 'quit', 'clear', 'context', 'all-tools', 'help'")
    print("=" * 70)
    
    # Show initial context
    print(f"\n📦 Initial context: {tool_context.get_stats()}")
    print("   • tool_search (always available)")
    print("\n💡 Try: 'Calculate the square root of 144' - the LLM will discover calculator tools!")
    
    # System message that explains the dynamic discovery pattern
    system_message = {
        "role": "system",
        "content": """You are a helpful assistant with access to tools through ModelGate's MCP Gateway.

CRITICAL: You start with ONLY the `tool_search` tool. You MUST use it to discover other tools!

When a user asks you to do something:
1. FIRST use `tool_search` with a natural language query to find relevant tools
   Example: tool_search(query="calculate math expressions")
   Example: tool_search(query="read file contents")
2. The search will return tool specifications that will be added to your available tools
3. THEN call the discovered tools to complete the task

Tool naming: Tools from external servers use format `server_name/tool_name`
Example: `Local MCP/calculator`, `sample-tools/read_file`

IMPORTANT: 
- If you don't know what tools are available, use tool_search first!
- After tool_search, you'll have access to the discovered tools
- You cannot use a tool unless you've discovered it via tool_search first"""
    }
    
    messages = [system_message]
    
    while True:
        try:
            # Show context size in prompt
            user_input = input(f"\n{tool_context.get_stats()} 🧑 You: ").strip()
            
            if not user_input:
                continue
            
            # Handle commands
            if user_input.lower() in ("quit", "exit", "q"):
                print("\n👋 Goodbye!")
                break
            
            if user_input.lower() == "clear":
                messages = [system_message]
                tool_context.clear()
                print("✓ Conversation and tool context cleared")
                print(f"📦 Context reset: {tool_context.get_stats()}")
                continue
            
            if user_input.lower() == "context":
                tools = tool_context.get_tools_for_llm()
                print(f"\n📦 Current Tool Context ({len(tools)} tools):")
                for tool in tools:
                    name = tool["function"]["name"]
                    desc = tool["function"].get("description", "")[:50]
                    marker = "🔍" if name == "tool_search" else "🔧"
                    print(f"  {marker} {name}")
                    if desc:
                        print(f"     {desc}...")
                continue
            
            if user_input.lower() == "all-tools":
                print("\n📦 Fetching all server tools (admin view)...")
                all_tools = mcp_client.list_tools()
                print(f"Total available: {len(all_tools)}")
                for tool in all_tools:
                    name = tool.get("name", "unknown")
                    desc = tool.get("description", "")[:50]
                    in_ctx = "✓" if tool_context.is_known_tool(name) else " "
                    print(f"  [{in_ctx}] {name}")
                    if desc:
                        print(f"       {desc}...")
                continue
            
            if user_input.lower() == "help":
                print("\nCommands:")
                print("  quit     - Exit the chat")
                print("  clear    - Clear conversation and tool context")
                print("  context  - Show tools currently in context")
                print("  all-tools- Show all available server tools (admin)")
                print("\nHow it works:")
                print("  1. LLM starts with only 'tool_search' available")
                print("  2. When you ask for something, LLM searches for relevant tools")
                print("  3. Discovered tools are added to context automatically")
                print("  4. LLM then uses the discovered tools")
                print("\nExample prompts:")
                print("  • 'Calculate 2^10 + sqrt(144)'")
                print("  • 'What's the current date and time?'")
                print("  • 'Search for file tools'")
                continue
            
            # Add user message and track conversation state
            messages_before_turn = len(messages)
            messages.append({"role": "user", "content": user_input})

            # Tool call loop - uses dynamic context!
            max_iterations = 10
            iteration = 0

            while iteration < max_iterations:
                iteration += 1
                print("🤖 Assistant: ", end="", flush=True)

                # Get current tools from dynamic context
                current_tools = tool_context.get_tools_for_llm()

                try:
                    response = llm_client.chat(messages, tools=current_tools)
                except ChatError as e:
                    print(f"\n❌ Chat error: {e}")
                    # Revert all messages added during this turn to maintain valid conversation state
                    del messages[messages_before_turn:]
                    break
                
                if not response.get("choices"):
                    print("\n❌ No response from model")
                    break
                
                choice = response["choices"][0]
                message = choice["message"]
                tool_calls = message.get("tool_calls", [])
                
                # No tool calls - show response and break
                if not tool_calls:
                    content = message.get("content", "")
                    print(content)
                    messages.append({"role": "assistant", "content": content})
                    break
                
                # Model wants to call tools
                print(f"[calling {len(tool_calls)} tool(s)...]")
                
                # Add assistant message with tool calls
                messages.append({
                    "role": "assistant",
                    "content": message.get("content"),
                    "tool_calls": tool_calls,
                })
                
                # Execute each tool
                for tool_call in tool_calls:
                    tool_name = tool_call["function"]["name"]
                    try:
                        tool_args = json.loads(tool_call["function"]["arguments"])
                    except json.JSONDecodeError:
                        tool_args = {}
                    
                    print(f"  🔧 {tool_name}({json.dumps(tool_args, separators=(',', ':'))[:60]})")
                    
                    try:
                        # Execute via MCP Gateway
                        result = mcp_client.call_tool(tool_name, tool_args)
                        result_text = extract_tool_result(result)
                        
                        # Special handling for tool_search - add discovered tools to context
                        if tool_name == "tool_search":
                            added_tools = tool_context.add_tools_from_search(result_text)
                            if added_tools:
                                print(f"     ✓ Discovered {len(added_tools)} tools: {', '.join(added_tools)}")
                                print(f"     📦 {tool_context.get_stats()}")
                            else:
                                print(f"     ✓ No new tools found")
                        elif result.get("isError"):
                            print(f"     ❌ Error")
                        else:
                            # Truncate long results for display
                            display_result = result_text[:100] + "..." if len(result_text) > 100 else result_text
                            print(f"     ✓ {display_result}")
                        
                        # Add tool result to messages
                        messages.append({
                            "role": "tool",
                            "tool_call_id": tool_call["id"],
                            "name": tool_name,
                            "content": result_text,
                        })
                        
                    except MCPError as e:
                        error_msg = f"Tool error: {e.message}"
                        print(f"     ❌ {error_msg}")
                        messages.append({
                            "role": "tool",
                            "tool_call_id": tool_call["id"],
                            "name": tool_name,
                            "content": error_msg,
                        })
                
                # Continue to get model's response after tool execution
                print("🤖 Assistant: ", end="", flush=True)
            
        except KeyboardInterrupt:
            print("\n\n👋 Goodbye!")
            break
        except Exception as e:
            print(f"\n❌ Error: {e}")
            import traceback
            traceback.print_exc()


def main():
    parser = argparse.ArgumentParser(
        description="ModelGate MCP Gateway Chat with Dynamic Tool Discovery"
    )
    parser.add_argument(
        "--url",
        default="http://localhost:8080",
        help="ModelGate URL (default: http://localhost:8080)",
    )
    parser.add_argument(
        "--api-key",
        default=os.environ.get("MODELGATE_API_KEY", ""),
        help="API key (or set MODELGATE_API_KEY env var)",
    )
    parser.add_argument(
        "--model",
        "-m",
        default="openai/gpt-5.1",
        help="LLM model to use (default: openai/gpt-4o)",
    )
    
    args = parser.parse_args()
    
    if not args.api_key:
        print("❌ Error: API key required")
        print("Use --api-key or set MODELGATE_API_KEY environment variable")
        sys.exit(1)
    
    # Create clients
    llm_client = ModelGateLLMClient(
        base_url=args.url,
        api_key=args.api_key,
        model=args.model,
    )
    
    mcp_client = ModelGateMCPClient(
        base_url=args.url,
        api_key=args.api_key,
    )
    
    try:
        # Initialize MCP Gateway connection
        print("🔌 Connecting to ModelGate MCP Gateway...")
        mcp_client.initialize()
        print(f"✅ Connected to {mcp_client.server_info.get('name', 'ModelGate MCP Gateway')}")
        
        # Run chat session with dynamic tool discovery
        run_chat_session(llm_client, mcp_client)
        
    except MCPError as e:
        print(f"❌ MCP Error: {e.message}")
        print("\nMake sure ModelGate is running (./modelgate)")
        sys.exit(1)
    except KeyboardInterrupt:
        print("\n\n👋 Goodbye!")


if __name__ == "__main__":
    main()
