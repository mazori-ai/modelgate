#!/usr/bin/env python3
"""
MCP Client with ModelGate Integration

A Python client that connects to ModelGate and uses MCP tools to enhance
the LLM's capabilities with file operations, system commands, and more.

Usage:
    # Start the MCP server first (in another terminal):
    python mcp_server.py

    # Then run the client:
    python mcp_client.py

    # Or with custom settings:
    python mcp_client.py --model openai/gpt-4o --api-key your_key

Examples:
    User: "Read the contents of README.md"
    User: "List all Python files in the current directory"
    User: "Calculate 15 * 23 + 47"
    User: "What's the current time?"
"""

import argparse
import asyncio
import json
import os
import sys
from typing import Any

try:
    import requests
except ImportError:
    print("Error: 'requests' library required. Install with: pip install requests")
    sys.exit(1)

try:
    from mcp import ClientSession, StdioServerParameters
    from mcp.client.stdio import stdio_client
except ImportError:
    print("Error: MCP SDK not installed. Install with: pip install mcp", file=sys.stderr)
    sys.exit(1)


class ModelGateClient:
    """Client for ModelGate HTTP API with MCP tool support."""

    def __init__(
        self,
        base_url: str = "http://localhost:8080",
        api_key: str | None = None,
        model: str = "openai/gpt-4.1"
    ):
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key or os.environ.get("MODELGATE_API_KEY", "")
        self.model = model
        self.session = requests.Session()
        if self.api_key:
            self.session.headers["Authorization"] = f"Bearer {self.api_key}"

    def chat_with_tools(
        self,
        messages: list[dict],
        tools: list[dict],
        temperature: float = 0.7,
    ) -> dict:
        """
        Send a chat request with tool definitions.

        Returns the complete response including any tool calls.
        """
        payload = {
            "model": self.model,
            "messages": messages,
            "tools": tools,
            "temperature": temperature,
        }

        response = self.session.post(
            f"{self.base_url}/v1/chat/completions",
            json=payload,
        )
        
        # Handle errors with detailed message
        if not response.ok:
            try:
                error_data = response.json()
                if "error" in error_data:
                    error_msg = error_data["error"].get("message", str(error_data["error"]))
                    error_type = error_data["error"].get("type", "unknown_error")
                    raise requests.exceptions.HTTPError(
                        f"[{error_type}] {error_msg}",
                        response=response
                    )
            except (ValueError, KeyError):
                pass
            response.raise_for_status()
        
        return response.json()


class MCPToolExecutor:
    """Executes MCP tools via stdio connection."""

    def __init__(self, mcp_session: ClientSession):
        self.session = mcp_session

    async def list_tools(self) -> list[dict]:
        """Get list of available tools from MCP server."""
        response = await self.session.list_tools()
        tools = []

        for tool in response.tools:
            tools.append({
                "type": "function",
                "function": {
                    "name": tool.name,
                    "description": tool.description,
                    "parameters": tool.inputSchema
                }
            })

        return tools

    async def execute_tool(self, tool_name: str, arguments: dict) -> str:
        """Execute a tool and return the result."""
        response = await self.session.call_tool(tool_name, arguments)

        # Combine all text content from the response
        result_text = []
        for content in response.content:
            if hasattr(content, 'text'):
                result_text.append(content.text)

        return "\n".join(result_text)


async def run_interactive_session(
    modelgate_client: ModelGateClient,
    mcp_executor: MCPToolExecutor
):
    """Run an interactive chat session with tool calling support."""

    print("\n" + "="*60)
    print("🚀 ModelGate + MCP Interactive Chat")
    print("="*60)
    print(f"Model: {modelgate_client.model}")
    print("Type 'quit' or 'exit' to end the session")
    print("Type 'help' for available commands")
    print("="*60 + "\n")

    # Get available tools from MCP server
    print("📦 Loading MCP tools...", end=" ", flush=True)
    tools = await mcp_executor.list_tools()
    print(f"✓ Loaded {len(tools)} tools\n")

    # Show available tools
    print("Available tools:")
    for tool in tools:
        func = tool["function"]
        print(f"  • {func['name']}: {func['description']}")
    print()

    # Conversation history
    messages = []

    while True:
        try:
            # Get user input
            user_input = input("\n🧑 You: ").strip()

            if not user_input:
                continue

            # Handle commands
            if user_input.lower() in ("quit", "exit"):
                print("\n👋 Goodbye!")
                break

            if user_input.lower() == "help":
                print("\nCommands:")
                print("  quit/exit - End the session")
                print("  help      - Show this message")
                print("  clear     - Clear conversation history")
                continue

            if user_input.lower() == "clear":
                messages = []
                print("✓ Conversation history cleared")
                continue

            # Add user message
            messages.append({
                "role": "user",
                "content": user_input
            })

            # Tool call loop - keep going until model stops requesting tools
            while True:
                # Send request to ModelGate
                print("🤖 Assistant: ", end="", flush=True)
                response = modelgate_client.chat_with_tools(messages, tools)

                if not response.get("choices"):
                    print("\n❌ No response from model")
                    break

                choice = response["choices"][0]
                message = choice["message"]

                # Check if model wants to use tools
                tool_calls = message.get("tool_calls", [])

                if not tool_calls:
                    # No tool calls - model is done, show response
                    content = message.get("content", "")
                    print(content)

                    # Add assistant message to history
                    messages.append({
                        "role": "assistant",
                        "content": content
                    })
                    break

                # Model wants to call tools
                print(f"[calling {len(tool_calls)} tool(s)...]")

                # Add assistant message with tool calls to history
                messages.append({
                    "role": "assistant",
                    "content": message.get("content"),
                    "tool_calls": tool_calls
                })

                # Execute each tool call
                for tool_call in tool_calls:
                    tool_name = tool_call["function"]["name"]
                    tool_args = json.loads(tool_call["function"]["arguments"])

                    print(f"  🔧 Executing: {tool_name}({json.dumps(tool_args, separators=(',', ':'))})")

                    try:
                        # Execute tool via MCP
                        result = await mcp_executor.execute_tool(tool_name, tool_args)

                        # Add tool result to messages
                        messages.append({
                            "role": "tool",
                            "tool_call_id": tool_call["id"],
                            "name": tool_name,
                            "content": result
                        })

                        print(f"     ✓ Done")

                    except Exception as e:
                        error_msg = f"Error executing {tool_name}: {str(e)}"
                        print(f"     ❌ {error_msg}")

                        # Add error as tool result
                        messages.append({
                            "role": "tool",
                            "tool_call_id": tool_call["id"],
                            "name": tool_name,
                            "content": error_msg
                        })

                # Continue loop to get model's next response after tool execution
                print("🤖 Assistant: ", end="", flush=True)

        except KeyboardInterrupt:
            print("\n\n👋 Goodbye!")
            break
        except requests.exceptions.RequestException as e:
            print(f"\n❌ Request error: {e}")
        except Exception as e:
            print(f"\n❌ Error: {e}")
            import traceback
            traceback.print_exc()


async def main():
    """Main entry point."""
    parser = argparse.ArgumentParser(
        description="MCP Client with ModelGate Integration"
    )
    parser.add_argument(
        "--model",
        "-m",
        default="openai/gpt-4.1",
        help="Model to use (default: openai/gpt-4.1)"
    )
    parser.add_argument(
        "--api-key",
        "-k",
        default=None,
        help="API key for ModelGate authentication"
    )
    parser.add_argument(
        "--base-url",
        "-u",
        default="http://localhost:8080",
        help="ModelGate server URL (default: http://localhost:8080)"
    )
    parser.add_argument(
        "--mcp-server",
        default="python",
        help="Command to run MCP server (default: python)"
    )
    parser.add_argument(
        "--mcp-script",
        default="mcp_server.py",
        help="Path to MCP server script (default: mcp_server.py)"
    )

    args = parser.parse_args()

    # Initialize ModelGate client
    modelgate_client = ModelGateClient(
        base_url=args.base_url,
        api_key=args.api_key,
        model=args.model
    )

    # Connect to MCP server via stdio
    server_params = StdioServerParameters(
        command=args.mcp_server,
        args=[args.mcp_script],
        env=None
    )

    print("🔌 Connecting to MCP server...", end=" ", flush=True)

    try:
        async with stdio_client(server_params) as (read, write):
            async with ClientSession(read, write) as session:
                await session.initialize()

                print("✓ Connected\n")

                # Create tool executor
                mcp_executor = MCPToolExecutor(session)

                # Run interactive session
                await run_interactive_session(modelgate_client, mcp_executor)

    except FileNotFoundError:
        print(f"\n❌ MCP server script not found: {args.mcp_script}")
        print("Make sure mcp_server.py is in the current directory")
        sys.exit(1)
    except Exception as e:
        print(f"\n❌ Failed to connect to MCP server: {e}")
        print("\nMake sure:")
        print("  1. The MCP server script exists")
        print("  2. Python is installed and in PATH")
        print("  3. MCP dependencies are installed (pip install mcp)")
        sys.exit(1)


if __name__ == "__main__":
    asyncio.run(main())
