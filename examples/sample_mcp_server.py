#!/usr/bin/env python3
"""
Sample MCP Server for ModelGate Integration

This is a sample MCP server with various utility tools that can be registered
with ModelGate to demonstrate the MCP Gateway functionality.

Tools included:
- File operations (read, write, list)
- Data processing (JSON, CSV parsing)
- Utilities (calculator, datetime, file info)
- Search (file search, content search)

Usage:
    # Start this MCP server
    python sample_mcp_server.py
    
    # Or run with uvx:
    uvx mcp run sample_mcp_server.py

Registering with ModelGate:
    1. Go to Tenant Admin UI: http://localhost:3000/tenant/acme/mcp
    2. Add new MCP Server:
       - Name: sample-tools
       - Type: stdio
       - Command: python
       - Arguments: /path/to/sample_mcp_server.py
    3. Click "Connect" to establish connection
    4. The tools will appear in the MCP Gateway

Then use the modelgate_mcp_demo.py to access these tools:
    python modelgate_mcp_demo.py --tenant acme --api-key your-key
"""

import csv
import datetime
import hashlib
import io
import json
import math
import os
import re
import subprocess
import sys
from pathlib import Path
from typing import Any

try:
    from mcp.server import Server
    from mcp.server.stdio import stdio_server
    from mcp.types import (
        Tool,
        TextContent,
        INVALID_PARAMS,
        INTERNAL_ERROR,
    )
except ImportError:
    print("Error: MCP SDK not installed. Install with: pip install mcp", file=sys.stderr)
    sys.exit(1)


# Create MCP server
server = Server("sample-mcp-tools")

# Working directory (can be configured)
WORK_DIR = Path.cwd()


# =============================================================================
# Tool Definitions
# =============================================================================

@server.list_tools()
async def list_tools() -> list[Tool]:
    """Return list of available tools."""
    return [
        # File Operations
        Tool(
            name="read_file",
            description="Read the contents of a file from the filesystem",
            inputSchema={
                "type": "object",
                "properties": {
                    "path": {
                        "type": "string",
                        "description": "Path to the file to read"
                    },
                    "encoding": {
                        "type": "string",
                        "description": "File encoding (default: utf-8)",
                        "default": "utf-8"
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
                    },
                    "append": {
                        "type": "boolean",
                        "description": "Append to file instead of overwriting",
                        "default": False
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
                        "description": "Path to the directory to list (default: current directory)",
                        "default": "."
                    },
                    "include_hidden": {
                        "type": "boolean",
                        "description": "Include hidden files (starting with .)",
                        "default": False
                    },
                    "recursive": {
                        "type": "boolean",
                        "description": "List recursively",
                        "default": False
                    }
                }
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
        ),
        
        # Search Operations
        Tool(
            name="search_files",
            description="Search for files matching a pattern in a directory",
            inputSchema={
                "type": "object",
                "properties": {
                    "pattern": {
                        "type": "string",
                        "description": "Glob pattern to match (e.g., '*.py', '**/*.md')"
                    },
                    "path": {
                        "type": "string",
                        "description": "Directory to search in (default: current)",
                        "default": "."
                    },
                    "max_results": {
                        "type": "integer",
                        "description": "Maximum number of results to return",
                        "default": 50
                    }
                },
                "required": ["pattern"]
            }
        ),
        Tool(
            name="search_content",
            description="Search for text content within files",
            inputSchema={
                "type": "object",
                "properties": {
                    "query": {
                        "type": "string",
                        "description": "Text or regex pattern to search for"
                    },
                    "path": {
                        "type": "string",
                        "description": "Directory to search in",
                        "default": "."
                    },
                    "file_pattern": {
                        "type": "string",
                        "description": "File pattern to search in (e.g., '*.py')",
                        "default": "*"
                    },
                    "case_sensitive": {
                        "type": "boolean",
                        "description": "Case-sensitive search",
                        "default": False
                    },
                    "max_results": {
                        "type": "integer",
                        "description": "Maximum number of matches",
                        "default": 20
                    }
                },
                "required": ["query"]
            }
        ),
        
        # Data Processing
        Tool(
            name="parse_json",
            description="Parse JSON string and return formatted output with optional path extraction",
            inputSchema={
                "type": "object",
                "properties": {
                    "json_string": {
                        "type": "string",
                        "description": "JSON string to parse"
                    },
                    "path": {
                        "type": "string",
                        "description": "JSON path to extract (e.g., 'data.items[0].name')"
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
                    "delimiter": {
                        "type": "string",
                        "description": "Field delimiter (default: comma)",
                        "default": ","
                    },
                    "has_header": {
                        "type": "boolean",
                        "description": "First row is header",
                        "default": True
                    }
                },
                "required": ["csv_string"]
            }
        ),
        
        # Utilities
        Tool(
            name="calculate",
            description="Evaluate a mathematical expression safely",
            inputSchema={
                "type": "object",
                "properties": {
                    "expression": {
                        "type": "string",
                        "description": "Mathematical expression to evaluate (e.g., '2 + 3 * 4', 'sqrt(16)', 'sin(pi/2)')"
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
                        "description": "Output format: 'iso', 'date', 'time', 'timestamp', or strftime format",
                        "default": "iso"
                    },
                    "timezone": {
                        "type": "string",
                        "description": "Timezone (default: local)",
                        "default": "local"
                    }
                }
            }
        ),
        Tool(
            name="hash_text",
            description="Generate hash of text using various algorithms",
            inputSchema={
                "type": "object",
                "properties": {
                    "text": {
                        "type": "string",
                        "description": "Text to hash"
                    },
                    "algorithm": {
                        "type": "string",
                        "description": "Hash algorithm: md5, sha1, sha256, sha512",
                        "default": "sha256",
                        "enum": ["md5", "sha1", "sha256", "sha512"]
                    }
                },
                "required": ["text"]
            }
        ),
        Tool(
            name="run_command",
            description="Execute a shell command and return output (with safety restrictions)",
            inputSchema={
                "type": "object",
                "properties": {
                    "command": {
                        "type": "string",
                        "description": "Command to execute"
                    },
                    "timeout": {
                        "type": "integer",
                        "description": "Timeout in seconds",
                        "default": 30
                    },
                    "cwd": {
                        "type": "string",
                        "description": "Working directory"
                    }
                },
                "required": ["command"]
            }
        ),
        Tool(
            name="environment_info",
            description="Get information about the current environment",
            inputSchema={
                "type": "object",
                "properties": {
                    "include_env_vars": {
                        "type": "boolean",
                        "description": "Include environment variables (filtered)",
                        "default": False
                    }
                }
            }
        ),
    ]


# =============================================================================
# Tool Implementations
# =============================================================================

@server.call_tool()
async def call_tool(name: str, arguments: dict[str, Any]) -> list[TextContent]:
    """Execute a tool and return results."""
    
    try:
        if name == "read_file":
            return await tool_read_file(arguments)
        elif name == "write_file":
            return await tool_write_file(arguments)
        elif name == "list_directory":
            return await tool_list_directory(arguments)
        elif name == "file_info":
            return await tool_file_info(arguments)
        elif name == "search_files":
            return await tool_search_files(arguments)
        elif name == "search_content":
            return await tool_search_content(arguments)
        elif name == "parse_json":
            return await tool_parse_json(arguments)
        elif name == "parse_csv":
            return await tool_parse_csv(arguments)
        elif name == "calculate":
            return await tool_calculate(arguments)
        elif name == "get_datetime":
            return await tool_get_datetime(arguments)
        elif name == "hash_text":
            return await tool_hash_text(arguments)
        elif name == "run_command":
            return await tool_run_command(arguments)
        elif name == "environment_info":
            return await tool_environment_info(arguments)
        else:
            return [TextContent(type="text", text=f"Unknown tool: {name}")]
            
    except Exception as e:
        return [TextContent(type="text", text=f"Error: {str(e)}")]


async def tool_read_file(args: dict) -> list[TextContent]:
    """Read file contents."""
    path = Path(args["path"]).expanduser()
    encoding = args.get("encoding", "utf-8")
    
    if not path.exists():
        return [TextContent(type="text", text=f"File not found: {path}")]
    
    if not path.is_file():
        return [TextContent(type="text", text=f"Not a file: {path}")]
    
    try:
        content = path.read_text(encoding=encoding)
        return [TextContent(type="text", text=content)]
    except UnicodeDecodeError:
        # Try reading as binary
        content = path.read_bytes()
        return [TextContent(type="text", text=f"[Binary file, {len(content)} bytes]")]


async def tool_write_file(args: dict) -> list[TextContent]:
    """Write content to file."""
    path = Path(args["path"]).expanduser()
    content = args["content"]
    append = args.get("append", False)
    
    # Create parent directories if needed
    path.parent.mkdir(parents=True, exist_ok=True)
    
    mode = "a" if append else "w"
    path.open(mode).write(content)
    
    action = "Appended to" if append else "Wrote"
    return [TextContent(type="text", text=f"{action} {path} ({len(content)} chars)")]


async def tool_list_directory(args: dict) -> list[TextContent]:
    """List directory contents."""
    path = Path(args.get("path", ".")).expanduser()
    include_hidden = args.get("include_hidden", False)
    recursive = args.get("recursive", False)
    
    if not path.exists():
        return [TextContent(type="text", text=f"Directory not found: {path}")]
    
    if not path.is_dir():
        return [TextContent(type="text", text=f"Not a directory: {path}")]
    
    entries = []
    
    if recursive:
        iterator = path.rglob("*")
    else:
        iterator = path.iterdir()
    
    for entry in sorted(iterator):
        if not include_hidden and entry.name.startswith("."):
            continue
        
        if entry.is_dir():
            entries.append(f"📁 {entry.name}/")
        else:
            size = entry.stat().st_size
            if size < 1024:
                size_str = f"{size}B"
            elif size < 1024 * 1024:
                size_str = f"{size/1024:.1f}KB"
            else:
                size_str = f"{size/1024/1024:.1f}MB"
            entries.append(f"📄 {entry.name} ({size_str})")
    
    if not entries:
        return [TextContent(type="text", text="Directory is empty")]
    
    return [TextContent(type="text", text="\n".join(entries))]


async def tool_file_info(args: dict) -> list[TextContent]:
    """Get file/directory information."""
    path = Path(args["path"]).expanduser()
    
    if not path.exists():
        return [TextContent(type="text", text=f"Path not found: {path}")]
    
    stat = path.stat()
    
    info = {
        "path": str(path.absolute()),
        "name": path.name,
        "type": "directory" if path.is_dir() else "file",
        "size": stat.st_size,
        "size_human": f"{stat.st_size / 1024:.2f} KB" if stat.st_size > 1024 else f"{stat.st_size} bytes",
        "created": datetime.datetime.fromtimestamp(stat.st_ctime).isoformat(),
        "modified": datetime.datetime.fromtimestamp(stat.st_mtime).isoformat(),
        "accessed": datetime.datetime.fromtimestamp(stat.st_atime).isoformat(),
    }
    
    if path.is_file():
        info["extension"] = path.suffix
        info["mime_type"] = _guess_mime_type(path)
    
    return [TextContent(type="text", text=json.dumps(info, indent=2))]


async def tool_search_files(args: dict) -> list[TextContent]:
    """Search for files matching pattern."""
    pattern = args["pattern"]
    path = Path(args.get("path", ".")).expanduser()
    max_results = args.get("max_results", 50)
    
    if not path.exists():
        return [TextContent(type="text", text=f"Directory not found: {path}")]
    
    matches = []
    for match in path.glob(pattern):
        if len(matches) >= max_results:
            break
        matches.append(str(match))
    
    if not matches:
        return [TextContent(type="text", text=f"No files matching '{pattern}' found")]
    
    result = f"Found {len(matches)} matches:\n" + "\n".join(matches)
    return [TextContent(type="text", text=result)]


async def tool_search_content(args: dict) -> list[TextContent]:
    """Search for content within files."""
    query = args["query"]
    path = Path(args.get("path", ".")).expanduser()
    file_pattern = args.get("file_pattern", "*")
    case_sensitive = args.get("case_sensitive", False)
    max_results = args.get("max_results", 20)
    
    if not path.exists():
        return [TextContent(type="text", text=f"Directory not found: {path}")]
    
    flags = 0 if case_sensitive else re.IGNORECASE
    try:
        pattern = re.compile(query, flags)
    except re.error as e:
        return [TextContent(type="text", text=f"Invalid regex pattern: {e}")]
    
    matches = []
    for file_path in path.rglob(file_pattern):
        if len(matches) >= max_results:
            break
        
        if not file_path.is_file():
            continue
        
        try:
            content = file_path.read_text(errors="ignore")
            for i, line in enumerate(content.splitlines(), 1):
                if pattern.search(line):
                    matches.append(f"{file_path}:{i}: {line[:100]}")
                    if len(matches) >= max_results:
                        break
        except Exception:
            continue
    
    if not matches:
        return [TextContent(type="text", text=f"No matches for '{query}' found")]
    
    return [TextContent(type="text", text="\n".join(matches))]


async def tool_parse_json(args: dict) -> list[TextContent]:
    """Parse and format JSON."""
    json_string = args["json_string"]
    path = args.get("path")
    
    try:
        data = json.loads(json_string)
    except json.JSONDecodeError as e:
        return [TextContent(type="text", text=f"Invalid JSON: {e}")]
    
    if path:
        # Extract value at path
        parts = re.split(r'\.|\[|\]', path)
        parts = [p for p in parts if p]
        
        current = data
        for part in parts:
            try:
                if part.isdigit():
                    current = current[int(part)]
                else:
                    current = current[part]
            except (KeyError, IndexError, TypeError) as e:
                return [TextContent(type="text", text=f"Path not found: {path}")]
        
        data = current
    
    return [TextContent(type="text", text=json.dumps(data, indent=2))]


async def tool_parse_csv(args: dict) -> list[TextContent]:
    """Parse CSV data."""
    csv_string = args["csv_string"]
    delimiter = args.get("delimiter", ",")
    has_header = args.get("has_header", True)
    
    reader = csv.reader(io.StringIO(csv_string), delimiter=delimiter)
    rows = list(reader)
    
    if not rows:
        return [TextContent(type="text", text="Empty CSV")]
    
    if has_header:
        headers = rows[0]
        data = [dict(zip(headers, row)) for row in rows[1:]]
    else:
        data = rows
    
    return [TextContent(type="text", text=json.dumps(data, indent=2))]


async def tool_calculate(args: dict) -> list[TextContent]:
    """Safely evaluate mathematical expression."""
    expression = args["expression"]
    
    # Safe math functions
    safe_dict = {
        "abs": abs,
        "round": round,
        "min": min,
        "max": max,
        "sum": sum,
        "pow": pow,
        "sqrt": math.sqrt,
        "sin": math.sin,
        "cos": math.cos,
        "tan": math.tan,
        "log": math.log,
        "log10": math.log10,
        "exp": math.exp,
        "pi": math.pi,
        "e": math.e,
        "floor": math.floor,
        "ceil": math.ceil,
    }
    
    try:
        # Remove potentially dangerous characters
        if any(c in expression for c in ['import', 'exec', 'eval', 'open', '__']):
            return [TextContent(type="text", text="Expression contains forbidden characters")]
        
        result = eval(expression, {"__builtins__": {}}, safe_dict)
        return [TextContent(type="text", text=f"{expression} = {result}")]
    except Exception as e:
        return [TextContent(type="text", text=f"Calculation error: {e}")]


async def tool_get_datetime(args: dict) -> list[TextContent]:
    """Get current date/time."""
    fmt = args.get("format", "iso")
    now = datetime.datetime.now()
    
    formats = {
        "iso": now.isoformat(),
        "date": now.strftime("%Y-%m-%d"),
        "time": now.strftime("%H:%M:%S"),
        "timestamp": str(int(now.timestamp())),
        "full": now.strftime("%A, %B %d, %Y at %I:%M %p"),
    }
    
    if fmt in formats:
        result = formats[fmt]
    else:
        try:
            result = now.strftime(fmt)
        except ValueError:
            result = formats["iso"]
    
    return [TextContent(type="text", text=result)]


async def tool_hash_text(args: dict) -> list[TextContent]:
    """Generate hash of text."""
    text = args["text"]
    algorithm = args.get("algorithm", "sha256")
    
    algos = {
        "md5": hashlib.md5,
        "sha1": hashlib.sha1,
        "sha256": hashlib.sha256,
        "sha512": hashlib.sha512,
    }
    
    if algorithm not in algos:
        return [TextContent(type="text", text=f"Unknown algorithm: {algorithm}")]
    
    hash_obj = algos[algorithm](text.encode())
    return [TextContent(type="text", text=f"{algorithm}: {hash_obj.hexdigest()}")]


async def tool_run_command(args: dict) -> list[TextContent]:
    """Execute shell command safely."""
    command = args["command"]
    timeout = args.get("timeout", 30)
    cwd = args.get("cwd")
    
    # Block dangerous commands
    dangerous = ["rm -rf", "mkfs", "dd if=", ":(){", "fork bomb"]
    if any(d in command.lower() for d in dangerous):
        return [TextContent(type="text", text="Command blocked for safety reasons")]
    
    try:
        result = subprocess.run(
            command,
            shell=True,
            capture_output=True,
            text=True,
            timeout=timeout,
            cwd=cwd,
        )
        
        output = result.stdout
        if result.stderr:
            output += f"\n[stderr]\n{result.stderr}"
        if result.returncode != 0:
            output += f"\n[exit code: {result.returncode}]"
        
        return [TextContent(type="text", text=output or "(no output)")]
        
    except subprocess.TimeoutExpired:
        return [TextContent(type="text", text=f"Command timed out after {timeout}s")]
    except Exception as e:
        return [TextContent(type="text", text=f"Command failed: {e}")]


async def tool_environment_info(args: dict) -> list[TextContent]:
    """Get environment information."""
    include_env = args.get("include_env_vars", False)
    
    info = {
        "python_version": sys.version,
        "platform": sys.platform,
        "cwd": str(Path.cwd()),
        "user": os.environ.get("USER", os.environ.get("USERNAME", "unknown")),
        "home": str(Path.home()),
        "pid": os.getpid(),
    }
    
    if include_env:
        # Filter sensitive env vars
        safe_prefixes = ["PATH", "HOME", "USER", "LANG", "TERM", "SHELL", "EDITOR"]
        info["env"] = {
            k: v for k, v in os.environ.items()
            if any(k.startswith(p) for p in safe_prefixes)
        }
    
    return [TextContent(type="text", text=json.dumps(info, indent=2))]


def _guess_mime_type(path: Path) -> str:
    """Guess MIME type from extension."""
    ext_map = {
        ".py": "text/x-python",
        ".js": "text/javascript",
        ".ts": "text/typescript",
        ".json": "application/json",
        ".md": "text/markdown",
        ".txt": "text/plain",
        ".html": "text/html",
        ".css": "text/css",
        ".xml": "application/xml",
        ".yaml": "application/x-yaml",
        ".yml": "application/x-yaml",
        ".toml": "application/toml",
        ".csv": "text/csv",
        ".pdf": "application/pdf",
        ".png": "image/png",
        ".jpg": "image/jpeg",
        ".jpeg": "image/jpeg",
        ".gif": "image/gif",
        ".svg": "image/svg+xml",
    }
    return ext_map.get(path.suffix.lower(), "application/octet-stream")


# =============================================================================
# Main
# =============================================================================

async def main():
    """Run the MCP server."""
    async with stdio_server() as (read_stream, write_stream):
        await server.run(
            read_stream,
            write_stream,
            server.create_initialization_options(),
        )


if __name__ == "__main__":
    import asyncio
    asyncio.run(main())

