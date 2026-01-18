#!/usr/bin/env python3
"""
Local MCP Server for Testing ModelGate MCP Gateway

This server implements the Model Context Protocol (MCP) and exposes several
test tools for validating the ModelGate MCP Gateway functionality.

Usage:
    # Run as stdio server (for local testing)
    python local_mcp_server.py --mode stdio
    
    # Run as HTTP/SSE server (for remote access)
    python local_mcp_server.py --mode http --port 8085

Tools provided:
    - calculator: Perform mathematical calculations
    - get_weather: Get mock weather data for a city
    - search_files: Search for files by pattern
    - send_notification: Send a mock notification
    - get_system_info: Get system information
    - database_query: Execute a mock database query
    - translate_text: Mock translation service
    - image_analyze: Mock image analysis
"""

import argparse
import json
import sys
import os
import math
import platform
import datetime
from http.server import HTTPServer, BaseHTTPRequestHandler
from typing import Any, Dict, List, Optional
import threading
import uuid


# MCP Protocol Version
MCP_VERSION = "2024-11-05"


class MCPToolRegistry:
    """Registry of available MCP tools"""
    
    def __init__(self):
        self.tools = {}
        self._register_tools()
    
    def _register_tools(self):
        """Register all available tools"""
        
        # Calculator tool
        self.tools["calculator"] = {
            "name": "calculator",
            "description": "Perform mathematical calculations. Supports basic arithmetic, trigonometry, and common functions.",
            "category": "utilities",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "expression": {
                        "type": "string",
                        "description": "Mathematical expression to evaluate (e.g., '2 + 2', 'sin(3.14)', 'sqrt(16)')"
                    }
                },
                "required": ["expression"]
            },
            "handler": self._handle_calculator
        }
        
        # Weather tool
        self.tools["get_weather"] = {
            "name": "get_weather",
            "description": "Get current weather information for a specified city. Returns temperature, conditions, and forecast.",
            "category": "api",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "city": {
                        "type": "string",
                        "description": "City name (e.g., 'San Francisco', 'London', 'Tokyo')"
                    },
                    "units": {
                        "type": "string",
                        "enum": ["celsius", "fahrenheit"],
                        "description": "Temperature units",
                        "default": "celsius"
                    }
                },
                "required": ["city"]
            },
            "handler": self._handle_weather
        }
        
        # File search tool
        self.tools["search_files"] = {
            "name": "search_files",
            "description": "Search for files matching a pattern in a directory. Returns list of matching file paths.",
            "category": "file-system",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "pattern": {
                        "type": "string",
                        "description": "File pattern to search for (e.g., '*.py', 'README*')"
                    },
                    "directory": {
                        "type": "string",
                        "description": "Directory to search in (defaults to current directory)",
                        "default": "."
                    },
                    "recursive": {
                        "type": "boolean",
                        "description": "Whether to search subdirectories",
                        "default": False
                    }
                },
                "required": ["pattern"]
            },
            "handler": self._handle_search_files
        }
        
        # Notification tool
        self.tools["send_notification"] = {
            "name": "send_notification",
            "description": "Send a notification message. Supports different priority levels and channels.",
            "category": "messaging",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "message": {
                        "type": "string",
                        "description": "Notification message content"
                    },
                    "title": {
                        "type": "string",
                        "description": "Notification title"
                    },
                    "priority": {
                        "type": "string",
                        "enum": ["low", "normal", "high", "urgent"],
                        "description": "Notification priority level",
                        "default": "normal"
                    },
                    "channel": {
                        "type": "string",
                        "enum": ["email", "slack", "sms", "push"],
                        "description": "Notification channel",
                        "default": "push"
                    }
                },
                "required": ["message"]
            },
            "handler": self._handle_notification
        }
        
        # System info tool
        self.tools["get_system_info"] = {
            "name": "get_system_info",
            "description": "Get information about the current system including OS, CPU, memory, and Python version.",
            "category": "utilities",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "include_env": {
                        "type": "boolean",
                        "description": "Include environment variables in response",
                        "default": False
                    }
                },
                "required": []
            },
            "handler": self._handle_system_info
        }
        
        # Database query tool
        self.tools["database_query"] = {
            "name": "database_query",
            "description": "Execute a database query. Returns mock data for testing purposes.",
            "category": "database",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "query": {
                        "type": "string",
                        "description": "SQL query to execute"
                    },
                    "database": {
                        "type": "string",
                        "description": "Database name",
                        "default": "test_db"
                    },
                    "limit": {
                        "type": "integer",
                        "description": "Maximum number of rows to return",
                        "default": 10
                    }
                },
                "required": ["query"]
            },
            "handler": self._handle_database_query
        }
        
        # Translation tool
        self.tools["translate_text"] = {
            "name": "translate_text",
            "description": "Translate text from one language to another. Supports major world languages.",
            "category": "api",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "text": {
                        "type": "string",
                        "description": "Text to translate"
                    },
                    "source_language": {
                        "type": "string",
                        "description": "Source language code (e.g., 'en', 'es', 'fr')",
                        "default": "auto"
                    },
                    "target_language": {
                        "type": "string",
                        "description": "Target language code (e.g., 'en', 'es', 'fr')"
                    }
                },
                "required": ["text", "target_language"]
            },
            "handler": self._handle_translate
        }
        
        # Image analysis tool
        self.tools["analyze_image"] = {
            "name": "analyze_image",
            "description": "Analyze an image and return detected objects, text, and scene description.",
            "category": "api",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "image_url": {
                        "type": "string",
                        "description": "URL of the image to analyze"
                    },
                    "analysis_types": {
                        "type": "array",
                        "items": {"type": "string"},
                        "description": "Types of analysis: objects, text, faces, scenes",
                        "default": ["objects", "scenes"]
                    }
                },
                "required": ["image_url"]
            },
            "handler": self._handle_image_analyze
        }
        
        # Echo tool (simple test)
        self.tools["echo"] = {
            "name": "echo",
            "description": "Simple echo tool that returns the input message. Useful for testing connectivity.",
            "category": "utilities",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "message": {
                        "type": "string",
                        "description": "Message to echo back"
                    }
                },
                "required": ["message"]
            },
            "handler": self._handle_echo
        }
        
        # Execute shell command (dangerous - for testing policy enforcement)
        self.tools["execute_shell"] = {
            "name": "execute_shell",
            "description": "Execute a shell command on the system. ⚠️ DANGEROUS - should be restricted by policy.",
            "category": "shell",
            "inputSchema": {
                "type": "object",
                "properties": {
                    "command": {
                        "type": "string",
                        "description": "Shell command to execute"
                    },
                    "timeout": {
                        "type": "integer",
                        "description": "Command timeout in seconds",
                        "default": 30
                    }
                },
                "required": ["command"]
            },
            "handler": self._handle_shell
        }
    
    # Tool handlers
    
    def _handle_calculator(self, args: Dict) -> Dict:
        """Handle calculator tool"""
        expression = args.get("expression", "")
        
        # Safe eval with limited scope
        allowed_names = {
            "sin": math.sin, "cos": math.cos, "tan": math.tan,
            "sqrt": math.sqrt, "log": math.log, "log10": math.log10,
            "exp": math.exp, "pow": pow, "abs": abs,
            "pi": math.pi, "e": math.e
        }
        
        try:
            # Very basic sanitization
            for char in expression:
                if char not in "0123456789+-*/().sincoqrtlgexpwab ":
                    raise ValueError(f"Invalid character in expression: {char}")
            
            result = eval(expression, {"__builtins__": {}}, allowed_names)
            return {"result": result, "expression": expression}
        except Exception as e:
            return {"error": str(e), "expression": expression}
    
    def _handle_weather(self, args: Dict) -> Dict:
        """Handle weather tool (mock data)"""
        city = args.get("city", "Unknown")
        units = args.get("units", "celsius")
        
        # Mock weather data
        import random
        temp = random.randint(15, 30) if units == "celsius" else random.randint(59, 86)
        conditions = random.choice(["Sunny", "Cloudy", "Partly Cloudy", "Rainy", "Clear"])
        
        return {
            "city": city,
            "temperature": temp,
            "units": units,
            "conditions": conditions,
            "humidity": random.randint(30, 80),
            "wind_speed": random.randint(5, 25),
            "forecast": [
                {"day": "Tomorrow", "high": temp + 2, "low": temp - 5, "conditions": "Sunny"},
                {"day": "Day After", "high": temp + 1, "low": temp - 3, "conditions": "Cloudy"}
            ],
            "timestamp": datetime.datetime.now().isoformat()
        }
    
    def _handle_search_files(self, args: Dict) -> Dict:
        """Handle file search tool"""
        pattern = args.get("pattern", "*")
        directory = args.get("directory", ".")
        recursive = args.get("recursive", False)
        
        import fnmatch
        import os
        
        matches = []
        try:
            if recursive:
                for root, dirs, files in os.walk(directory):
                    for filename in fnmatch.filter(files, pattern):
                        matches.append(os.path.join(root, filename))
            else:
                if os.path.isdir(directory):
                    for filename in os.listdir(directory):
                        if fnmatch.fnmatch(filename, pattern):
                            matches.append(os.path.join(directory, filename))
            
            return {
                "pattern": pattern,
                "directory": directory,
                "recursive": recursive,
                "matches": matches[:50],  # Limit results
                "total_found": len(matches)
            }
        except Exception as e:
            return {"error": str(e)}
    
    def _handle_notification(self, args: Dict) -> Dict:
        """Handle notification tool (mock)"""
        message = args.get("message", "")
        title = args.get("title", "Notification")
        priority = args.get("priority", "normal")
        channel = args.get("channel", "push")
        
        notification_id = str(uuid.uuid4())[:8]
        
        return {
            "status": "sent",
            "notification_id": notification_id,
            "channel": channel,
            "priority": priority,
            "title": title,
            "message_preview": message[:50] + "..." if len(message) > 50 else message,
            "timestamp": datetime.datetime.now().isoformat()
        }
    
    def _handle_system_info(self, args: Dict) -> Dict:
        """Handle system info tool"""
        include_env = args.get("include_env", False)
        
        info = {
            "os": platform.system(),
            "os_version": platform.version(),
            "architecture": platform.machine(),
            "python_version": platform.python_version(),
            "hostname": platform.node(),
            "processor": platform.processor(),
            "timestamp": datetime.datetime.now().isoformat()
        }
        
        if include_env:
            # Only include safe environment variables
            safe_vars = ["PATH", "HOME", "USER", "SHELL", "LANG"]
            info["environment"] = {k: os.environ.get(k, "") for k in safe_vars if k in os.environ}
        
        return info
    
    def _handle_database_query(self, args: Dict) -> Dict:
        """Handle database query tool (mock)"""
        query = args.get("query", "")
        database = args.get("database", "test_db")
        limit = args.get("limit", 10)
        
        # Mock response based on query type
        query_lower = query.lower()
        
        if "select" in query_lower:
            # Mock SELECT results
            return {
                "query": query,
                "database": database,
                "rows": [
                    {"id": 1, "name": "Test User 1", "email": "user1@test.com"},
                    {"id": 2, "name": "Test User 2", "email": "user2@test.com"},
                    {"id": 3, "name": "Test User 3", "email": "user3@test.com"},
                ][:limit],
                "row_count": min(3, limit),
                "execution_time_ms": 12
            }
        else:
            return {
                "query": query,
                "database": database,
                "affected_rows": 1,
                "execution_time_ms": 8
            }
    
    def _handle_translate(self, args: Dict) -> Dict:
        """Handle translation tool (mock)"""
        text = args.get("text", "")
        source = args.get("source_language", "auto")
        target = args.get("target_language", "en")
        
        # Mock translations
        translations = {
            "es": {"hello": "hola", "world": "mundo", "thank you": "gracias"},
            "fr": {"hello": "bonjour", "world": "monde", "thank you": "merci"},
            "de": {"hello": "hallo", "world": "welt", "thank you": "danke"},
            "ja": {"hello": "こんにちは", "world": "世界", "thank you": "ありがとう"},
        }
        
        translated = f"[{target}] {text}"  # Simple mock
        
        return {
            "original_text": text,
            "translated_text": translated,
            "source_language": source,
            "target_language": target,
            "confidence": 0.95
        }
    
    def _handle_image_analyze(self, args: Dict) -> Dict:
        """Handle image analysis tool (mock)"""
        image_url = args.get("image_url", "")
        analysis_types = args.get("analysis_types", ["objects", "scenes"])
        
        result = {
            "image_url": image_url,
            "analysis_types": analysis_types,
            "results": {}
        }
        
        if "objects" in analysis_types:
            result["results"]["objects"] = [
                {"label": "person", "confidence": 0.95, "bounding_box": [10, 20, 100, 200]},
                {"label": "laptop", "confidence": 0.88, "bounding_box": [150, 100, 250, 180]},
            ]
        
        if "scenes" in analysis_types:
            result["results"]["scenes"] = [
                {"label": "office", "confidence": 0.82},
                {"label": "indoor", "confidence": 0.96},
            ]
        
        if "text" in analysis_types:
            result["results"]["text"] = [
                {"text": "Hello World", "confidence": 0.91, "location": [50, 50, 150, 70]}
            ]
        
        if "faces" in analysis_types:
            result["results"]["faces"] = [
                {"confidence": 0.94, "emotion": "neutral", "age_range": [25, 35]}
            ]
        
        return result
    
    def _handle_echo(self, args: Dict) -> Dict:
        """Handle echo tool"""
        message = args.get("message", "")
        return {
            "echo": message,
            "timestamp": datetime.datetime.now().isoformat(),
            "server": "local-mcp-test-server"
        }
    
    def _handle_shell(self, args: Dict) -> Dict:
        """Handle shell command tool"""
        command = args.get("command", "")
        timeout = args.get("timeout", 30)
        
        # For safety, only allow certain commands in this test server
        allowed_prefixes = ["echo", "date", "whoami", "pwd", "ls", "dir"]
        
        cmd_lower = command.lower().strip()
        is_allowed = any(cmd_lower.startswith(prefix) for prefix in allowed_prefixes)
        
        if not is_allowed:
            return {
                "error": "Command not allowed in test server",
                "command": command,
                "allowed_commands": allowed_prefixes
            }
        
        import subprocess
        try:
            result = subprocess.run(
                command,
                shell=True,
                capture_output=True,
                text=True,
                timeout=timeout
            )
            return {
                "command": command,
                "stdout": result.stdout,
                "stderr": result.stderr,
                "return_code": result.returncode
            }
        except subprocess.TimeoutExpired:
            return {"error": "Command timed out", "command": command}
        except Exception as e:
            return {"error": str(e), "command": command}
    
    def get_tool_definitions(self) -> List[Dict]:
        """Get all tool definitions for MCP tools/list"""
        return [
            {
                "name": tool["name"],
                "description": tool["description"],
                "inputSchema": tool["inputSchema"]
            }
            for tool in self.tools.values()
        ]
    
    def call_tool(self, name: str, arguments: Dict) -> Dict:
        """Call a tool by name with arguments"""
        if name not in self.tools:
            return {"error": f"Unknown tool: {name}"}
        
        handler = self.tools[name]["handler"]
        return handler(arguments)


class MCPServer:
    """MCP Server implementation"""
    
    def __init__(self):
        self.registry = MCPToolRegistry()
        self.server_info = {
            "name": "local-test-server",
            "version": "1.0.0",
            "description": "Local MCP server for testing ModelGate MCP Gateway"
        }
        self.capabilities = {
            "tools": {}
        }
    
    def handle_request(self, request: Dict) -> Dict:
        """Handle an MCP JSON-RPC request"""
        method = request.get("method", "")
        params = request.get("params", {})
        request_id = request.get("id")
        
        try:
            if method == "initialize":
                result = self._handle_initialize(params)
            elif method == "tools/list":
                result = self._handle_list_tools()
            elif method == "tools/call":
                result = self._handle_call_tool(params)
            elif method == "ping":
                result = {}
            else:
                return {
                    "jsonrpc": "2.0",
                    "id": request_id,
                    "error": {"code": -32601, "message": f"Method not found: {method}"}
                }
            
            return {
                "jsonrpc": "2.0",
                "id": request_id,
                "result": result
            }
        
        except Exception as e:
            return {
                "jsonrpc": "2.0",
                "id": request_id,
                "error": {"code": -32603, "message": str(e)}
            }
    
    def _handle_initialize(self, params: Dict) -> Dict:
        """Handle initialize request"""
        client_info = params.get("clientInfo", {})
        print(f"[MCP] Client connected: {client_info.get('name', 'unknown')}", file=sys.stderr)
        
        return {
            "protocolVersion": MCP_VERSION,
            "capabilities": self.capabilities,
            "serverInfo": self.server_info
        }
    
    def _handle_list_tools(self) -> Dict:
        """Handle tools/list request"""
        return {"tools": self.registry.get_tool_definitions()}
    
    def _handle_call_tool(self, params: Dict) -> Dict:
        """Handle tools/call request"""
        name = params.get("name", "")
        arguments = params.get("arguments", {})
        
        print(f"[MCP] Tool call: {name}", file=sys.stderr)
        
        result = self.registry.call_tool(name, arguments)
        
        return {
            "content": [
                {
                    "type": "text",
                    "text": json.dumps(result, indent=2)
                }
            ]
        }


class MCPStdioServer:
    """MCP server over stdio (for local testing)"""
    
    def __init__(self):
        self.mcp = MCPServer()
    
    def run(self):
        """Run the stdio server"""
        print("[MCP Server] Starting stdio server...", file=sys.stderr)
        print(f"[MCP Server] Providing {len(self.mcp.registry.tools)} tools", file=sys.stderr)
        
        for line in sys.stdin:
            line = line.strip()
            if not line:
                continue
            
            try:
                request = json.loads(line)
                response = self.mcp.handle_request(request)
                print(json.dumps(response), flush=True)
            except json.JSONDecodeError as e:
                print(f"[MCP] JSON parse error: {e}", file=sys.stderr)


class MCPHTTPHandler(BaseHTTPRequestHandler):
    """HTTP handler for MCP over SSE"""
    
    mcp_server = None
    
    def log_message(self, format, *args):
        """Override to log to stderr"""
        print(f"[HTTP] {args[0]}", file=sys.stderr)
    
    def do_OPTIONS(self):
        """Handle CORS preflight"""
        self.send_response(200)
        self.send_header("Access-Control-Allow-Origin", "*")
        self.send_header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
        self.send_header("Access-Control-Allow-Headers", "Content-Type, Authorization")
        self.end_headers()
    
    def do_GET(self):
        """Handle GET requests"""
        if self.path == "/" or self.path == "":
            # Root path - return server info for SSE connection test
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Access-Control-Allow-Origin", "*")
            self.end_headers()
            self.wfile.write(json.dumps({
                "name": self.mcp_server.server_info["name"],
                "version": self.mcp_server.server_info["version"],
                "status": "connected",
                "tools_count": len(self.mcp_server.registry.tools)
            }).encode())
        elif self.path == "/health":
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(json.dumps({"status": "ok"}).encode())
        elif self.path == "/tools":
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Access-Control-Allow-Origin", "*")
            self.end_headers()
            tools = self.mcp_server.registry.get_tool_definitions()
            self.wfile.write(json.dumps({"tools": tools}, indent=2).encode())
        else:
            self.send_response(404)
            self.end_headers()
    
    def do_POST(self):
        """Handle POST requests (MCP JSON-RPC)"""
        content_length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(content_length).decode()
        
        try:
            request = json.loads(body)
            response = self.mcp_server.handle_request(request)
            
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Access-Control-Allow-Origin", "*")
            self.end_headers()
            self.wfile.write(json.dumps(response).encode())
        
        except json.JSONDecodeError:
            self.send_response(400)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(json.dumps({"error": "Invalid JSON"}).encode())


def run_http_server(port: int):
    """Run the HTTP server"""
    MCPHTTPHandler.mcp_server = MCPServer()
    
    server = HTTPServer(("", port), MCPHTTPHandler)
    print(f"[MCP Server] Starting HTTP server on port {port}...", file=sys.stderr)
    print(f"[MCP Server] Health check: http://localhost:{port}/health", file=sys.stderr)
    print(f"[MCP Server] Tools list: http://localhost:{port}/tools", file=sys.stderr)
    print(f"[MCP Server] MCP endpoint: POST http://localhost:{port}/", file=sys.stderr)
    print(f"[MCP Server] Providing {len(MCPHTTPHandler.mcp_server.registry.tools)} tools", file=sys.stderr)
    
    server.serve_forever()


def main():
    parser = argparse.ArgumentParser(
        description="Local MCP Server for Testing ModelGate",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Available Tools:
  - calculator       Perform mathematical calculations
  - get_weather      Get mock weather data
  - search_files     Search for files by pattern
  - send_notification Send a mock notification
  - get_system_info  Get system information
  - database_query   Execute a mock database query
  - translate_text   Mock translation service
  - analyze_image    Mock image analysis
  - echo             Simple echo for testing
  - execute_shell    Execute shell commands (restricted)

Examples:
  # Run as stdio server
  python local_mcp_server.py --mode stdio
  
  # Run as HTTP server on port 8085
  python local_mcp_server.py --mode http --port 8085
  
  # Test with curl
  curl http://localhost:8085/tools
  curl -X POST http://localhost:8085/ -H "Content-Type: application/json" \\
       -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
"""
    )
    
    parser.add_argument(
        "--mode",
        choices=["stdio", "http"],
        default="http",
        help="Server mode: stdio (for local process) or http (for network access)"
    )
    
    parser.add_argument(
        "--port",
        type=int,
        default=8085,
        help="Port for HTTP server (default: 8085)"
    )
    
    args = parser.parse_args()
    
    if args.mode == "stdio":
        server = MCPStdioServer()
        server.run()
    else:
        run_http_server(args.port)


if __name__ == "__main__":
    main()

