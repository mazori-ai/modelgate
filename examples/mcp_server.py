#!/usr/bin/env python3
"""
MCP Server with Utility Tools

A stdio-based MCP server that provides 10 useful tools for file operations,
system commands, data processing, and more.

Usage:
    python mcp_server.py

This server communicates via stdio and follows the MCP protocol.
"""

import asyncio
import json
import os
import sys
import subprocess
import csv
import io
from datetime import datetime
from typing import Any
from pathlib import Path

try:
    from mcp.server import Server
    from mcp.server.stdio import stdio_server
    from mcp.types import Tool, TextContent
except ImportError:
    print("Error: MCP SDK not installed. Install with: pip install mcp", file=sys.stderr)
    sys.exit(1)


# Initialize MCP server
app = Server("utility-tools")


@app.list_tools()
async def list_tools() -> list[Tool]:
    """List all available tools."""
    return [
        Tool(
            name="read_file",
            description="Read the contents of a file from the filesystem",
            inputSchema={
                "type": "object",
                "properties": {
                    "path": {
                        "type": "string",
                        "description": "Path to the file to read"
                    }
                },
                "required": ["path"]
            }
        ),
        Tool(
            name="write_file",
            description="Write content to a file, creating it if it doesn't exist",
            inputSchema={
                "type": "object",
                "properties": {
                    "path": {
                        "type": "string",
                        "description": "Path to the file to write"
                    },
                    "content": {
                        "type": "string",
                        "description": "Content to write to the file"
                    }
                },
                "required": ["path", "content"]
            }
        ),
        Tool(
            name="list_directory",
            description="List files and directories in a given path",
            inputSchema={
                "type": "object",
                "properties": {
                    "path": {
                        "type": "string",
                        "description": "Directory path to list (default: current directory)",
                        "default": "."
                    }
                }
            }
        ),
        Tool(
            name="execute_command",
            description="Execute a shell command and return its output",
            inputSchema={
                "type": "object",
                "properties": {
                    "command": {
                        "type": "string",
                        "description": "Shell command to execute"
                    },
                    "timeout": {
                        "type": "number",
                        "description": "Command timeout in seconds (default: 30)",
                        "default": 30
                    }
                },
                "required": ["command"]
            }
        ),
        Tool(
            name="parse_json",
            description="Parse JSON string and return formatted output",
            inputSchema={
                "type": "object",
                "properties": {
                    "json_string": {
                        "type": "string",
                        "description": "JSON string to parse"
                    },
                    "pretty": {
                        "type": "boolean",
                        "description": "Whether to pretty-print the output (default: true)",
                        "default": True
                    }
                },
                "required": ["json_string"]
            }
        ),
        Tool(
            name="parse_csv",
            description="Parse CSV string and return structured data",
            inputSchema={
                "type": "object",
                "properties": {
                    "csv_string": {
                        "type": "string",
                        "description": "CSV string to parse"
                    },
                    "has_header": {
                        "type": "boolean",
                        "description": "Whether the CSV has a header row (default: true)",
                        "default": True
                    }
                },
                "required": ["csv_string"]
            }
        ),
        Tool(
            name="calculate",
            description="Evaluate a mathematical expression safely",
            inputSchema={
                "type": "object",
                "properties": {
                    "expression": {
                        "type": "string",
                        "description": "Mathematical expression to evaluate (e.g., '2 + 2 * 3')"
                    }
                },
                "required": ["expression"]
            }
        ),
        Tool(
            name="get_datetime",
            description="Get current date and time in various formats",
            inputSchema={
                "type": "object",
                "properties": {
                    "format": {
                        "type": "string",
                        "description": "Format string (default: ISO 8601). Use Python strftime format codes.",
                        "default": "%Y-%m-%dT%H:%M:%S"
                    },
                    "timezone": {
                        "type": "string",
                        "description": "Timezone name (e.g., 'UTC', 'US/Pacific')",
                        "default": "UTC"
                    }
                }
            }
        ),
        Tool(
            name="search_files",
            description="Search for files matching a pattern in a directory",
            inputSchema={
                "type": "object",
                "properties": {
                    "directory": {
                        "type": "string",
                        "description": "Directory to search in (default: current directory)",
                        "default": "."
                    },
                    "pattern": {
                        "type": "string",
                        "description": "Glob pattern to match files (e.g., '*.py', '**/*.txt')"
                    }
                },
                "required": ["pattern"]
            }
        ),
        Tool(
            name="file_info",
            description="Get detailed information about a file or directory",
            inputSchema={
                "type": "object",
                "properties": {
                    "path": {
                        "type": "string",
                        "description": "Path to the file or directory"
                    }
                },
                "required": ["path"]
            }
        )
    ]


@app.call_tool()
async def call_tool(name: str, arguments: Any) -> list[TextContent]:
    """Handle tool calls."""

    try:
        if name == "read_file":
            path = arguments["path"]
            with open(path, "r") as f:
                content = f.read()
            return [TextContent(type="text", text=content)]

        elif name == "write_file":
            path = arguments["path"]
            content = arguments["content"]
            # Create parent directories if they don't exist
            os.makedirs(os.path.dirname(path) or ".", exist_ok=True)
            with open(path, "w") as f:
                f.write(content)
            return [TextContent(type="text", text=f"Successfully wrote {len(content)} characters to {path}")]

        elif name == "list_directory":
            path = arguments.get("path", ".")
            entries = []
            for entry in sorted(os.listdir(path)):
                full_path = os.path.join(path, entry)
                entry_type = "dir" if os.path.isdir(full_path) else "file"
                size = os.path.getsize(full_path) if os.path.isfile(full_path) else 0
                entries.append(f"{entry_type:5} {size:>10} {entry}")

            result = f"Contents of {path}:\n" + "\n".join(entries)
            return [TextContent(type="text", text=result)]

        elif name == "execute_command":
            command = arguments["command"]
            timeout = arguments.get("timeout", 30)

            try:
                result = subprocess.run(
                    command,
                    shell=True,
                    capture_output=True,
                    text=True,
                    timeout=timeout
                )
                output = f"Exit Code: {result.returncode}\n\n"
                if result.stdout:
                    output += f"STDOUT:\n{result.stdout}\n"
                if result.stderr:
                    output += f"STDERR:\n{result.stderr}\n"
                return [TextContent(type="text", text=output)]
            except subprocess.TimeoutExpired:
                return [TextContent(type="text", text=f"Command timed out after {timeout} seconds")]

        elif name == "parse_json":
            json_string = arguments["json_string"]
            pretty = arguments.get("pretty", True)

            parsed = json.loads(json_string)
            if pretty:
                result = json.dumps(parsed, indent=2)
            else:
                result = json.dumps(parsed)
            return [TextContent(type="text", text=result)]

        elif name == "parse_csv":
            csv_string = arguments["csv_string"]
            has_header = arguments.get("has_header", True)

            reader = csv.reader(io.StringIO(csv_string))
            rows = list(reader)

            if has_header and rows:
                headers = rows[0]
                data = []
                for row in rows[1:]:
                    data.append(dict(zip(headers, row)))
                result = json.dumps(data, indent=2)
            else:
                result = json.dumps(rows, indent=2)

            return [TextContent(type="text", text=result)]

        elif name == "calculate":
            expression = arguments["expression"]

            # Safe evaluation - only allow basic math operations
            allowed_names = {
                "abs": abs, "round": round, "min": min, "max": max,
                "sum": sum, "pow": pow, "divmod": divmod
            }

            try:
                # Compile and evaluate the expression safely
                code = compile(expression, "<string>", "eval")
                # Check for any dangerous operations
                for name in code.co_names:
                    if name not in allowed_names:
                        raise ValueError(f"Unsafe operation: {name}")

                result = eval(expression, {"__builtins__": {}}, allowed_names)
                return [TextContent(type="text", text=str(result))]
            except Exception as e:
                return [TextContent(type="text", text=f"Calculation error: {str(e)}")]

        elif name == "get_datetime":
            format_str = arguments.get("format", "%Y-%m-%dT%H:%M:%S")
            timezone = arguments.get("timezone", "UTC")

            now = datetime.now()
            formatted = now.strftime(format_str)

            result = f"Current time ({timezone}): {formatted}"
            return [TextContent(type="text", text=result)]

        elif name == "search_files":
            directory = arguments.get("directory", ".")
            pattern = arguments["pattern"]

            path = Path(directory)
            matches = list(path.glob(pattern))

            if matches:
                result = f"Found {len(matches)} file(s) matching '{pattern}':\n"
                result += "\n".join(str(m) for m in sorted(matches))
            else:
                result = f"No files found matching '{pattern}' in {directory}"

            return [TextContent(type="text", text=result)]

        elif name == "file_info":
            path = arguments["path"]

            if not os.path.exists(path):
                return [TextContent(type="text", text=f"Path does not exist: {path}")]

            stat = os.stat(path)
            is_dir = os.path.isdir(path)

            info = {
                "path": os.path.abspath(path),
                "type": "directory" if is_dir else "file",
                "size": stat.st_size if not is_dir else None,
                "created": datetime.fromtimestamp(stat.st_ctime).isoformat(),
                "modified": datetime.fromtimestamp(stat.st_mtime).isoformat(),
                "permissions": oct(stat.st_mode)[-3:],
            }

            if is_dir:
                info["items"] = len(os.listdir(path))

            result = json.dumps(info, indent=2)
            return [TextContent(type="text", text=result)]

        else:
            return [TextContent(type="text", text=f"Unknown tool: {name}")]

    except Exception as e:
        return [TextContent(type="text", text=f"Error executing {name}: {str(e)}")]


async def main():
    """Run the MCP server via stdio."""
    async with stdio_server() as (read_stream, write_stream):
        await app.run(
            read_stream,
            write_stream,
            app.create_initialization_options()
        )


if __name__ == "__main__":
    asyncio.run(main())
