#!/usr/bin/env python3
"""
ModelGate MCP Agent with Dynamic Tool Discovery

This script demonstrates the Tool Search pattern based on:
https://platform.claude.com/docs/en/agents-and-tools/tool-use/tool-search-tool

The agent:
1. Starts with only the 'tool_search' tool in context (minimal footprint)
2. Dynamically discovers tools on-demand using natural language search
3. Adds discovered tools to context for subsequent LLM calls
4. Executes tools with full schema validation

This pattern enables working with 1000s of tools without context bloat.

Usage:
    python modelgate_mcp_agent.py --api-key your-key
    python modelgate_mcp_agent.py --api-key your-key --demo
"""

import argparse
import json
import os
import sys
from typing import Optional, Dict, List, Any

import requests


class ModelGateMCPAgent:
    """
    MCP Agent with dynamic tool discovery.
    
    Implements the Tool Search Tool pattern where:
    - Tools are discovered on-demand, not loaded upfront
    - Only relevant tools are added to LLM context
    - Reduces token usage by 85%+ compared to loading all tools
    
    Based on: https://platform.claude.com/docs/en/agents-and-tools/tool-use/tool-search-tool
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
        self.server_info = None
        self.capabilities = None
        
        # Dynamic tool context - starts empty, populated by tool_search
        self.discovered_tools: Dict[str, dict] = {}
        
        # The tool_search tool definition (always available)
        self.tool_search_definition = {
            "name": "tool_search",
            "description": "Search for tools by natural language query. Returns tool definitions that can be added to context.",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "query": {
                        "type": "string",
                        "description": "Natural language description of the capability you're looking for"
                    },
                    "category": {
                        "type": "string",
                        "description": "Optional category filter (messaging, file-system, database, api, git, calendar, shell, search, other)"
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
    
    def _next_id(self) -> int:
        self.request_id += 1
        return self.request_id
    
    def _send_request(self, method: str, params: Optional[dict] = None) -> dict:
        """Send a JSON-RPC request to the MCP server."""
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
                timeout=30,
            )
            response.raise_for_status()
            result = response.json()
            
            if "error" in result and result["error"]:
                error = result["error"]
                raise MCPError(error.get("code", -1), error.get("message", "Unknown error"))
            
            return result.get("result", {})
            
        except requests.exceptions.ConnectionError:
            raise MCPError(-1, f"Failed to connect to {self.endpoint}")
        except requests.exceptions.Timeout:
            raise MCPError(-1, "Request timed out")
        except requests.exceptions.HTTPError as e:
            raise MCPError(-1, f"HTTP error: {e}")
    
    def initialize(self, client_name: str = "ModelGate-MCP-Agent", client_version: str = "1.0.0") -> dict:
        """Initialize the MCP connection."""
        result = self._send_request("initialize", {
            "protocolVersion": "2024-11-05",
            "capabilities": {"tools": {}},
            "clientInfo": {"name": client_name, "version": client_version},
        })
        
        self.initialized = True
        self.server_info = result.get("serverInfo", {})
        self.capabilities = result.get("capabilities", {})
        return result
    
    def get_context_tools(self) -> List[dict]:
        """
        Get current tools available in context.
        
        Returns:
        - tool_search (always available)
        - Any tools discovered via tool_search
        
        This is what you would pass to an LLM as the 'tools' parameter.
        """
        tools = [self.tool_search_definition]
        tools.extend(self.discovered_tools.values())
        return tools
    
    def get_context_tools_for_llm(self) -> List[dict]:
        """
        Get tools formatted for LLM function calling (OpenAI/Anthropic format).
        
        Converts MCP tool format to LLM function calling format.
        """
        llm_tools = []
        for tool in self.get_context_tools():
            llm_tool = {
                "type": "function",
                "function": {
                    "name": tool["name"],
                    "description": tool.get("description", ""),
                    "parameters": tool.get("inputSchema", {"type": "object", "properties": {}})
                }
            }
            # Include examples if available (per Anthropic's Tool Use Examples)
            if "inputExamples" in tool:
                llm_tool["function"]["input_examples"] = tool["inputExamples"]
            llm_tools.append(llm_tool)
        return llm_tools
    
    def search_tools(
        self,
        query: str,
        category: Optional[str] = None,
        max_results: int = 5,
        add_to_context: bool = True,
    ) -> List[dict]:
        """
        Search for tools and optionally add them to context.
        
        This implements the core Tool Search pattern:
        1. Send query to tool_search
        2. Receive tool definitions
        3. Add to context for LLM use
        
        Args:
            query: Natural language description of needed capability
            category: Optional category filter
            max_results: Max tools to return
            add_to_context: If True, discovered tools are added to context
            
        Returns:
            List of discovered tool definitions
        """
        if not self.initialized:
            raise MCPError(-1, "Must call initialize() first")
        
        args = {"query": query, "max_results": max_results}
        if category:
            args["category"] = category
        
        result = self._send_request("tools/call", {
            "name": "tool_search",
            "arguments": args,
        })
        
        # Parse the JSON response
        content = result.get("content", [])
        tools = []
        
        for block in content:
            if block.get("type") == "text":
                try:
                    search_result = json.loads(block.get("text", "{}"))
                    tools = search_result.get("tools", [])
                except json.JSONDecodeError:
                    continue
        
        # Add discovered tools to context
        if add_to_context and tools:
            for tool in tools:
                tool_name = tool.get("name")
                if tool_name:
                    # Remove metadata before adding to context
                    clean_tool = {k: v for k, v in tool.items() if not k.startswith("_")}
                    self.discovered_tools[tool_name] = clean_tool
        
        return tools
    
    def call_tool(self, name: str, arguments: Optional[dict] = None) -> dict:
        """
        Call a tool by name.
        
        Tool must be either:
        - 'tool_search' (always available)
        - A tool discovered via tool_search and added to context
        """
        if not self.initialized:
            raise MCPError(-1, "Must call initialize() first")
        
        # Check if tool is in context (except tool_search which is always available)
        if name != "tool_search" and name not in self.discovered_tools:
            raise MCPError(
                -1, 
                f"Tool '{name}' not in context. Use search_tools() to discover it first."
            )
        
        result = self._send_request("tools/call", {
            "name": name,
            "arguments": arguments or {},
        })
        return result
    
    def clear_context(self):
        """Clear discovered tools from context (keep only tool_search)."""
        self.discovered_tools.clear()
    
    def list_all_tools(self) -> List[dict]:
        """
        List ALL available tools from the server.
        
        Note: This is for debugging/admin purposes. In production, prefer
        using search_tools() to discover tools on-demand.
        """
        if not self.initialized:
            raise MCPError(-1, "Must call initialize() first")
        
        result = self._send_request("tools/list")
        return result.get("tools", [])
    
    def ping(self) -> bool:
        """Check if the server is responsive."""
        try:
            self._send_request("ping")
            return True
        except MCPError:
            return False


class MCPError(Exception):
    """MCP protocol error."""
    def __init__(self, code: int, message: str):
        self.code = code
        self.message = message
        super().__init__(f"MCP Error {code}: {message}")


def demo_tool_search_pattern(agent: ModelGateMCPAgent):
    """
    Demonstrates the Tool Search pattern for dynamic tool discovery.
    
    This pattern is based on:
    https://platform.claude.com/docs/en/agents-and-tools/tool-use/tool-search-tool
    """
    print("\n" + "=" * 70)
    print("🔍 TOOL SEARCH PATTERN DEMO")
    print("=" * 70)
    
    # Step 1: Show initial context (only tool_search)
    print("\n📋 STEP 1: Initial Context (minimal footprint)")
    print("-" * 50)
    initial_tools = agent.get_context_tools()
    print(f"   Tools in context: {len(initial_tools)}")
    for tool in initial_tools:
        print(f"   • {tool['name']}")
    print("\n   💡 Only tool_search is loaded - all other tools are deferred!")
    
    # Step 2: Search for calculator tools
    print("\n📋 STEP 2: Search for 'calculator' tools")
    print("-" * 50)
    discovered = agent.search_tools("calculator", max_results=3)
    print(f"   Found {len(discovered)} tools:")
    for tool in discovered:
        print(f"   • {tool.get('name')}: {tool.get('description', '')[:50]}...")
    
    # Step 3: Show updated context
    print("\n📋 STEP 3: Context after discovery")
    print("-" * 50)
    current_tools = agent.get_context_tools()
    print(f"   Tools in context: {len(current_tools)}")
    for tool in current_tools:
        print(f"   • {tool['name']}")
    
    # Step 4: Show LLM-ready format
    print("\n📋 STEP 4: LLM Function Calling Format")
    print("-" * 50)
    llm_tools = agent.get_context_tools_for_llm()
    print("   Tools formatted for LLM:")
    print(json.dumps(llm_tools, indent=2)[:500] + "...")
    
    # Step 5: Use a discovered tool
    if discovered:
        print("\n📋 STEP 5: Use Discovered Tool")
        print("-" * 50)
        tool_name = discovered[0].get("name")
        print(f"   Calling: {tool_name}")
        try:
            result = agent.call_tool(tool_name, {"expression": "2 + 2"})
            content = result.get("content", [])
            for block in content:
                if block.get("type") == "text":
                    print(f"   Result: {block.get('text', '')[:100]}")
        except MCPError as e:
            print(f"   Error: {e.message}")
    
    # Step 6: Search for more tools
    print("\n📋 STEP 6: Search for 'file' tools (adds to existing context)")
    print("-" * 50)
    more_tools = agent.search_tools("file read write", max_results=3)
    print(f"   Found {len(more_tools)} more tools")
    
    # Final context
    print("\n📋 FINAL: Context now contains discovered tools")
    print("-" * 50)
    final_tools = agent.get_context_tools()
    print(f"   Total tools in context: {len(final_tools)}")
    for tool in final_tools:
        print(f"   • {tool['name']}")
    
    print("\n" + "=" * 70)
    print("✅ DEMO COMPLETE")
    print("=" * 70)
    print("""
KEY TAKEAWAYS:
1. Start with only tool_search in context (saves tokens)
2. Search for tools when needed using natural language
3. Discovered tools are automatically added to context
4. LLM can then use the discovered tools
5. Clear context when switching tasks to manage token budget

This pattern enables working with 1000s of tools without context bloat!
""")


def interactive_session(agent: ModelGateMCPAgent):
    """Run an interactive session with dynamic tool discovery."""
    print("\n" + "=" * 60)
    print("🤖 ModelGate MCP Agent - Interactive Session")
    print("=" * 60)
    print(f"Server: {agent.server_info.get('name', 'Unknown')}")
    print(f"Version: {agent.server_info.get('version', 'Unknown')}")
    print("-" * 60)
    print("Commands:")
    print("  search <q>  - Search & add tools to context")
    print("  context     - Show tools in current context")
    print("  call <tool> - Call a tool from context")
    print("  clear       - Clear discovered tools from context")
    print("  list        - List ALL server tools (admin)")
    print("  quit/exit   - Exit the session")
    print("=" * 60)
    
    while True:
        try:
            # Show context size
            ctx_size = len(agent.get_context_tools())
            user_input = input(f"\n[{ctx_size} tools] 🧑 Command: ").strip()
            
            if not user_input:
                continue
            
            if user_input.lower() in ("quit", "exit", "q"):
                print("Goodbye!")
                break
            
            if user_input.lower() == "context":
                tools = agent.get_context_tools()
                print(f"\n📦 Context Tools ({len(tools)}):")
                for tool in tools:
                    desc = tool.get("description", "")[:50]
                    print(f"  • {tool['name']}")
                    if desc:
                        print(f"    {desc}...")
                continue
            
            if user_input.lower() == "clear":
                agent.clear_context()
                print("✅ Context cleared (only tool_search remains)")
                continue
            
            if user_input.lower() == "list":
                tools = agent.list_all_tools()
                print(f"\n📦 All Server Tools ({len(tools)}):")
                for tool in tools:
                    print(f"  • {tool.get('name')}")
                continue
            
            if user_input.lower().startswith("search "):
                query = user_input[7:].strip()
                print(f"\n🔍 Searching: {query}")
                tools = agent.search_tools(query)
                print(f"✅ Found and added {len(tools)} tools to context:")
                for tool in tools:
                    print(f"  • {tool.get('name')}")
                continue
            
            if user_input.lower().startswith("call "):
                tool_name = user_input[5:].strip()
                
                if tool_name not in agent.discovered_tools and tool_name != "tool_search":
                    print(f"⚠️  Tool '{tool_name}' not in context. Use 'search' first.")
                    continue
                
                print(f"\n🔧 Calling: {tool_name}")
                args_input = input("   Arguments (JSON or empty): ").strip()
                args = {}
                if args_input:
                    try:
                        args = json.loads(args_input)
                    except json.JSONDecodeError:
                        print("   ⚠️ Invalid JSON, using empty arguments")
                
                result = agent.call_tool(tool_name, args)
                content = result.get("content", [])
                
                if result.get("isError"):
                    print(f"\n❌ Error:")
                else:
                    print(f"\n✅ Result:")
                
                for block in content:
                    if block.get("type") == "text":
                        text = block.get("text", "")
                        # Pretty print JSON if possible
                        try:
                            parsed = json.loads(text)
                            print(json.dumps(parsed, indent=2))
                        except:
                            print(text)
                continue
            
            print(f"Unknown command: {user_input}")
            print("Use 'search <query>', 'context', 'call <tool>', 'clear', or 'quit'")
            
        except MCPError as e:
            print(f"\n❌ MCP Error: {e.message}")
        except KeyboardInterrupt:
            print("\n\nInterrupted. Goodbye!")
            break


def main():
    parser = argparse.ArgumentParser(
        description="ModelGate MCP Agent with Dynamic Tool Discovery"
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
        "--demo",
        action="store_true",
        help="Run the Tool Search pattern demo",
    )
    parser.add_argument(
        "--search",
        type=str,
        help="Search for tools and print results",
    )
    
    args = parser.parse_args()
    
    # Create agent
    agent = ModelGateMCPAgent(
        base_url=args.url,
        api_key=args.api_key,
    )
    
    try:
        # Initialize
        print("🔌 Connecting to ModelGate MCP Gateway...")
        agent.initialize()
        print(f"✅ Connected to {agent.server_info.get('name', 'Unknown')}")
        
        if args.demo:
            demo_tool_search_pattern(agent)
            return
        
        if args.search:
            print(f"\n🔍 Searching for: {args.search}")
            tools = agent.search_tools(args.search)
            print(f"\n📦 Found {len(tools)} tools:")
            print(json.dumps(tools, indent=2))
            return
        
        # Interactive mode
        interactive_session(agent)
        
    except MCPError as e:
        print(f"❌ Error: {e.message}")
        sys.exit(1)
    except KeyboardInterrupt:
        print("\n\nInterrupted. Goodbye!")
        sys.exit(0)


if __name__ == "__main__":
    main()
