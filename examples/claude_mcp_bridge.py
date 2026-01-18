#!/usr/bin/env python3
"""
ModelGate MCP Bridge for Claude Desktop

This script acts as a bridge between Claude Desktop (or any MCP-compatible agent)
and ModelGate's MCP Gateway. It translates stdio-based MCP messages to HTTP.

Installation:
1. Save this file to a permanent location
2. Install dependencies: pip install requests
3. Add to Claude Desktop config (see below)

Claude Desktop Configuration (~/.config/claude-desktop/config.json):
{
  "mcpServers": {
    "modelgate": {
      "command": "python3",
      "args": ["/path/to/claude_mcp_bridge.py"],
      "env": {
        "MODELGATE_URL": "http://localhost:8080",
        "MODELGATE_API_KEY": "your-tenant-api-key"
      }
    }
  }
}

Environment Variables:
- MODELGATE_URL: Base URL of ModelGate (default: http://localhost:8080)
- MODELGATE_API_KEY: Tenant API key (tenant is auto-detected from key!)
- MODELGATE_DEBUG: Set to "1" to enable debug logging to stderr

Note: MODELGATE_TENANT is no longer required - tenant is auto-detected from API key!

How it works:
1. Claude Desktop launches this script as a subprocess
2. Claude sends JSON-RPC messages via stdin
3. This bridge forwards them to ModelGate's HTTP MCP endpoint
4. Responses are sent back to Claude via stdout
"""

import json
import os
import sys
import logging
from typing import Optional

import requests


# Configure logging
DEBUG = os.environ.get("MODELGATE_DEBUG", "0") == "1"
if DEBUG:
    logging.basicConfig(
        level=logging.DEBUG,
        format="%(asctime)s [%(levelname)s] %(message)s",
        stream=sys.stderr,
    )
else:
    logging.basicConfig(
        level=logging.WARNING,
        format="%(asctime)s [%(levelname)s] %(message)s",
        stream=sys.stderr,
    )

logger = logging.getLogger(__name__)


class MCPBridge:
    """Bridge between stdio MCP and ModelGate HTTP MCP."""
    
    def __init__(self):
        self.base_url = os.environ.get("MODELGATE_URL", "http://localhost:8080")
        self.api_key = os.environ.get("MODELGATE_API_KEY", "")
        
        if not self.api_key:
            raise ValueError("MODELGATE_API_KEY environment variable is required")
        
        # Unified endpoint - tenant is auto-detected from API key
        self.endpoint = f"{self.base_url.rstrip('/')}/mcp"
        self.headers = {
            "Authorization": f"Bearer {self.api_key}",
            "Content-Type": "application/json",
        }
        self.session = requests.Session()
        self.session.headers.update(self.headers)
        
        logger.info(f"Initialized bridge to {self.endpoint}")
    
    def forward_request(self, request: dict) -> dict:
        """Forward a JSON-RPC request to ModelGate and return the response."""
        logger.debug(f"Forwarding request: {request.get('method')}")
        
        try:
            response = self.session.post(
                self.endpoint,
                json=request,
                timeout=60,  # Longer timeout for tool execution
            )
            
            logger.debug(f"Response status: {response.status_code}")
            
            if response.status_code == 401:
                return self._error_response(
                    request.get("id"),
                    -32001,
                    "Authentication failed. Check your API key.",
                )
            
            if response.status_code == 404:
                return self._error_response(
                    request.get("id"),
                    -32002,
                    "MCP endpoint not found. Ensure ModelGate is running.",
                )
            
            response.raise_for_status()
            return response.json()
            
        except requests.exceptions.ConnectionError:
            return self._error_response(
                request.get("id"),
                -32003,
                f"Failed to connect to ModelGate at {self.endpoint}",
            )
        except requests.exceptions.Timeout:
            return self._error_response(
                request.get("id"),
                -32004,
                "Request timed out",
            )
        except requests.exceptions.HTTPError as e:
            return self._error_response(
                request.get("id"),
                -32005,
                f"HTTP error: {str(e)}",
            )
        except json.JSONDecodeError:
            return self._error_response(
                request.get("id"),
                -32006,
                "Invalid JSON response from server",
            )
    
    def _error_response(self, request_id: Optional[int], code: int, message: str) -> dict:
        """Create a JSON-RPC error response."""
        return {
            "jsonrpc": "2.0",
            "id": request_id,
            "error": {
                "code": code,
                "message": message,
            },
        }
    
    def run(self):
        """Main loop - read from stdin, forward to ModelGate, write to stdout."""
        logger.info("Starting MCP bridge...")
        
        for line in sys.stdin:
            line = line.strip()
            if not line:
                continue
            
            logger.debug(f"Received: {line[:100]}...")
            
            try:
                request = json.loads(line)
            except json.JSONDecodeError as e:
                logger.error(f"Failed to parse request: {e}")
                response = self._error_response(None, -32700, "Parse error")
                print(json.dumps(response))
                sys.stdout.flush()
                continue
            
            # Forward to ModelGate
            response = self.forward_request(request)
            
            # Send response
            response_json = json.dumps(response)
            logger.debug(f"Sending: {response_json[:100]}...")
            print(response_json)
            sys.stdout.flush()
        
        logger.info("Bridge stopped")


def main():
    try:
        bridge = MCPBridge()
        bridge.run()
    except ValueError as e:
        # Configuration error
        logger.error(str(e))
        print(json.dumps({
            "jsonrpc": "2.0",
            "id": None,
            "error": {
                "code": -32000,
                "message": str(e),
            },
        }))
        sys.stdout.flush()
        sys.exit(1)
    except KeyboardInterrupt:
        logger.info("Interrupted")
        sys.exit(0)
    except Exception as e:
        logger.exception("Unexpected error")
        print(json.dumps({
            "jsonrpc": "2.0",
            "id": None,
            "error": {
                "code": -32603,
                "message": f"Internal error: {str(e)}",
            },
        }))
        sys.stdout.flush()
        sys.exit(1)


if __name__ == "__main__":
    main()

