#!/usr/bin/env python3
"""
MCP Demo - Practical Examples

Demonstrates various use cases of MCP tools with ModelGate.
Shows how to use the MCP client programmatically without interactive mode.

Usage:
    # Make sure mcp_server.py is running or available
    python mcp_demo.py
"""

import asyncio
import json
import sys
from pathlib import Path

try:
    import requests
    from mcp import ClientSession, StdioServerParameters
    from mcp.client.stdio import stdio_client
except ImportError:
    print("Error: Required libraries not installed.")
    print("Install with: pip install requests mcp")
    sys.exit(1)


class SimpleModelGateClient:
    """Simplified ModelGate client for demos."""

    def __init__(self, base_url="http://localhost:8080", api_key=None, model="openai/gpt-4o"):
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key or os.environ.get("MODELGATE_API_KEY", "")
        self.model = model
        self.session = requests.Session()
        if self.api_key:
            self.session.headers["Authorization"] = f"Bearer {self.api_key}"

    def chat(self, messages, tools=None):
        """Send a chat request."""
        payload = {
            "model": self.model,
            "messages": messages,
            "temperature": 0.7,
        }
        if tools:
            payload["tools"] = tools

        response = self.session.post(
            f"{self.base_url}/v1/chat/completions",
            json=payload,
        )
        response.raise_for_status()
        return response.json()


async def get_mcp_tools(session):
    """Get tool definitions from MCP server."""
    response = await session.list_tools()
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


async def execute_tool(session, tool_name, arguments):
    """Execute a tool via MCP."""
    response = await session.call_tool(tool_name, arguments)
    result_text = []
    for content in response.content:
        if hasattr(content, 'text'):
            result_text.append(content.text)
    return "\n".join(result_text)


async def demo_file_operations(client, mcp_session):
    """Demo: File operations."""
    print("\n" + "="*60)
    print("📁 Demo 1: File Operations")
    print("="*60)

    tools = await get_mcp_tools(mcp_session)

    # Create a test file
    print("\n1. Creating a test file...")
    print("   Task: Write 'Hello from MCP!' to test.txt")

    messages = [
        {"role": "user", "content": "Write 'Hello from MCP!' to a file named test.txt"}
    ]

    response = client.chat(messages, tools)
    choice = response["choices"][0]
    message = choice["message"]

    if tool_calls := message.get("tool_calls"):
        for tool_call in tool_calls:
            tool_name = tool_call["function"]["name"]
            tool_args = json.loads(tool_call["function"]["arguments"])

            print(f"   🔧 Tool: {tool_name}")
            print(f"   📝 Args: {json.dumps(tool_args, indent=6)}")

            result = await execute_tool(mcp_session, tool_name, tool_args)
            print(f"   ✓ Result: {result}")

    # Read the file back
    print("\n2. Reading the file back...")
    messages = [
        {"role": "user", "content": "Read the contents of test.txt"}
    ]

    response = client.chat(messages, tools)
    choice = response["choices"][0]
    message = choice["message"]

    if tool_calls := message.get("tool_calls"):
        for tool_call in tool_calls:
            tool_name = tool_call["function"]["name"]
            tool_args = json.loads(tool_call["function"]["arguments"])

            print(f"   🔧 Tool: {tool_name}")
            result = await execute_tool(mcp_session, tool_name, tool_args)
            print(f"   ✓ Content: {result}")

    # Clean up
    Path("test.txt").unlink(missing_ok=True)


async def demo_system_commands(client, mcp_session):
    """Demo: System commands."""
    print("\n" + "="*60)
    print("💻 Demo 2: System Commands")
    print("="*60)

    tools = await get_mcp_tools(mcp_session)

    print("\n1. Listing current directory...")
    messages = [
        {"role": "user", "content": "Show me what's in the current directory"}
    ]

    response = client.chat(messages, tools)
    choice = response["choices"][0]
    message = choice["message"]

    if tool_calls := message.get("tool_calls"):
        for tool_call in tool_calls:
            tool_name = tool_call["function"]["name"]
            tool_args = json.loads(tool_call["function"]["arguments"])

            print(f"   🔧 Tool: {tool_name}")
            result = await execute_tool(mcp_session, tool_name, tool_args)
            print(f"   ✓ Result:\n{result[:300]}...")

    print("\n2. Getting system information...")
    messages = [
        {"role": "user", "content": "What's the current date and time?"}
    ]

    response = client.chat(messages, tools)
    choice = response["choices"][0]
    message = choice["message"]

    if tool_calls := message.get("tool_calls"):
        for tool_call in tool_calls:
            tool_name = tool_call["function"]["name"]
            tool_args = json.loads(tool_call["function"]["arguments"])

            print(f"   🔧 Tool: {tool_name}")
            result = await execute_tool(mcp_session, tool_name, tool_args)
            print(f"   ✓ Result: {result}")


async def demo_data_processing(client, mcp_session):
    """Demo: Data processing."""
    print("\n" + "="*60)
    print("📊 Demo 3: Data Processing")
    print("="*60)

    tools = await get_mcp_tools(mcp_session)

    print("\n1. Parsing JSON data...")
    messages = [
        {"role": "user", "content": 'Parse this JSON and format it nicely: {"name":"Alice","age":30,"city":"NYC"}'}
    ]

    response = client.chat(messages, tools)
    choice = response["choices"][0]
    message = choice["message"]

    if tool_calls := message.get("tool_calls"):
        for tool_call in tool_calls:
            tool_name = tool_call["function"]["name"]
            tool_args = json.loads(tool_call["function"]["arguments"])

            print(f"   🔧 Tool: {tool_name}")
            result = await execute_tool(mcp_session, tool_name, tool_args)
            print(f"   ✓ Result:\n{result}")

    print("\n2. Mathematical calculation...")
    messages = [
        {"role": "user", "content": "Calculate: (15 * 23) + 47 - (100 / 4)"}
    ]

    response = client.chat(messages, tools)
    choice = response["choices"][0]
    message = choice["message"]

    if tool_calls := message.get("tool_calls"):
        for tool_call in tool_calls:
            tool_name = tool_call["function"]["name"]
            tool_args = json.loads(tool_call["function"]["arguments"])

            print(f"   🔧 Tool: {tool_name}")
            print(f"   📝 Expression: {tool_args.get('expression')}")
            result = await execute_tool(mcp_session, tool_name, tool_args)
            print(f"   ✓ Result: {result}")


async def demo_multi_tool_workflow(client, mcp_session):
    """Demo: Multi-tool workflow."""
    print("\n" + "="*60)
    print("🔄 Demo 4: Multi-Tool Workflow")
    print("="*60)

    tools = await get_mcp_tools(mcp_session)

    print("\nTask: Create a JSON file with system information")
    print("This will demonstrate the model using multiple tools in sequence\n")

    messages = [
        {
            "role": "user",
            "content": (
                "Create a JSON file called 'system_info.json' that contains "
                "the current date/time and a list of files in the current directory. "
                "Format it nicely with proper JSON structure."
            )
        }
    ]

    max_iterations = 5
    iteration = 0

    while iteration < max_iterations:
        iteration += 1
        print(f"\n--- Iteration {iteration} ---")

        response = client.chat(messages, tools)
        choice = response["choices"][0]
        message = choice["message"]

        tool_calls = message.get("tool_calls", [])

        if not tool_calls:
            # Model is done - show final response
            content = message.get("content", "")
            print(f"\n🤖 Final response: {content}")
            break

        # Add assistant message to history
        messages.append({
            "role": "assistant",
            "content": message.get("content"),
            "tool_calls": tool_calls
        })

        # Execute tools
        for tool_call in tool_calls:
            tool_name = tool_call["function"]["name"]
            tool_args = json.loads(tool_call["function"]["arguments"])

            print(f"🔧 Calling: {tool_name}")
            print(f"   Args: {json.dumps(tool_args, indent=9)}")

            try:
                result = await execute_tool(mcp_session, tool_name, tool_args)
                print(f"   ✓ Success")

                # Add tool result to messages
                messages.append({
                    "role": "tool",
                    "tool_call_id": tool_call["id"],
                    "name": tool_name,
                    "content": result
                })
            except Exception as e:
                print(f"   ❌ Error: {e}")
                messages.append({
                    "role": "tool",
                    "tool_call_id": tool_call["id"],
                    "name": tool_name,
                    "content": f"Error: {str(e)}"
                })

    # Show the created file
    if Path("system_info.json").exists():
        print("\n✓ File created successfully!")
        print("Content:")
        print(Path("system_info.json").read_text())
        # Clean up
        Path("system_info.json").unlink()


async def main():
    """Run all demos."""
    print("\n" + "="*60)
    print("🚀 ModelGate + MCP Demos")
    print("="*60)
    print("\nThis demo shows various use cases of MCP tools with ModelGate.")
    print("Make sure ModelGate is running on http://localhost:8080\n")

    # Initialize client
    client = SimpleModelGateClient()

    # Connect to MCP server
    server_params = StdioServerParameters(
        command="python3",
        args=["mcp_server.py"],
        env=None
    )

    print("🔌 Connecting to MCP server...")

    try:
        async with stdio_client(server_params) as (read, write):
            async with ClientSession(read, write) as session:
                await session.initialize()
                print("✓ Connected to MCP server\n")

                # Run demos
                await demo_file_operations(client, session)
                await demo_system_commands(client, session)
                await demo_data_processing(client, session)
                await demo_multi_tool_workflow(client, session)

                print("\n" + "="*60)
                print("✓ All demos completed!")
                print("="*60 + "\n")

    except Exception as e:
        print(f"\n❌ Error: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)


if __name__ == "__main__":
    asyncio.run(main())
