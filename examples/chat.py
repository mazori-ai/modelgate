#!/usr/bin/env python3
"""
ModelGate Chat Client

A simple interactive chat client for the ModelGate HTTP API.
Supports streaming responses and conversation history.

Usage:
    python chat.py [--model MODEL] [--api-key KEY] [--base-url URL]

Examples:
    # Basic usage (requires MODELGATE_API_KEY env var or no auth)
    python chat.py

    # Specify model
    python chat.py --model openai/gpt-5.1

    # With API key
    python chat.py --api-key mg_abc123...
"""

import argparse
import json
import os
import sys
from typing import Generator

try:
    import requests
except ImportError:
    print("Error: 'requests' library is required. Install with: pip install requests")
    sys.exit(1)


class ModelGateClient:
    """Client for ModelGate HTTP API (OpenAI-compatible)."""

    def __init__(self, base_url: str = "http://localhost:8080", api_key: str | None = None):
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key or os.environ.get("MODELGATE_API_KEY", "")
        self.session = requests.Session()
        if self.api_key:
            self.session.headers["Authorization"] = f"Bearer {self.api_key}"

    def list_models(self) -> list[dict]:
        """List available models."""
        response = self.session.get(f"{self.base_url}/v1/models")
        response.raise_for_status()
        return response.json().get("data", [])

    def chat(
        self,
        messages: list[dict],
        #model: str = "gemini/gemini-2.5-flash-preview-05-20",
        model: str = "openai/gpt-5.1",
        stream: bool = True,
        temperature: float = 0.7,
        max_tokens: int | None = None,
    ) -> Generator[str, None, None] | str:
        """
        Send a chat completion request.

        Args:
            messages: List of message dicts with 'role' and 'content'
            model: Model identifier (e.g., 'gemini/gemini-2.5-pro-preview-06-05')
            stream: Whether to stream the response
            temperature: Sampling temperature (0.0-1.0)
            max_tokens: Maximum tokens in response

        Returns:
            If stream=True: Generator yielding text chunks
            If stream=False: Complete response text
        """
        payload = {
            "model": model,
            "messages": messages,
            "stream": stream,
            "temperature": temperature,
        }
        if max_tokens:
            payload["max_tokens"] = max_tokens

        if stream:
            return self._stream_chat(payload)
        else:
            return self._complete_chat(payload)

    def _stream_chat(self, payload: dict) -> Generator[str, None, None]:
        """Handle streaming chat response."""
        # Use a long timeout for streaming - (connect_timeout, read_timeout)
        # read_timeout=None means no timeout for reading chunks
        response = self.session.post(
            f"{self.base_url}/v1/chat/completions",
            json=payload,
            stream=True,
            headers={"Accept": "text/event-stream"},
            timeout=(10, None),  # 10s to connect, no read timeout
        )
        response.raise_for_status()

        for line in response.iter_lines():
            if not line:
                continue

            line = line.decode("utf-8")
            if not line.startswith("data: "):
                continue

            data = line[6:]  # Remove "data: " prefix
            if data == "[DONE]":
                break

            try:
                chunk = json.loads(data)
                if "choices" in chunk and chunk["choices"]:
                    delta = chunk["choices"][0].get("delta", {})
                    if "content" in delta and delta["content"]:
                        yield delta["content"]
            except json.JSONDecodeError:
                continue

    def _complete_chat(self, payload: dict) -> str:
        """Handle non-streaming chat response."""
        response = self.session.post(
            f"{self.base_url}/v1/chat/completions",
            json=payload,
        )
        response.raise_for_status()

        data = response.json()
        if "choices" in data and data["choices"]:
            return data["choices"][0].get("message", {}).get("content", "")
        return ""

    def health_check(self) -> bool:
        """Check if the server is healthy."""
        try:
            response = self.session.get(f"{self.base_url}/health", timeout=5)
            return response.status_code == 200
        except requests.RequestException:
            return False


class ChatSession:
    """Interactive chat session manager."""

    def __init__(self, client: ModelGateClient, model: str, system_prompt: str | None = None):
        self.client = client
        self.model = model
        self.messages: list[dict] = []
        if system_prompt:
            self.messages.append({"role": "system", "content": system_prompt})

    def send(self, user_input: str, stream: bool = True) -> Generator[str, None, None] | str:
        """Send a message and get a response."""
        self.messages.append({"role": "user", "content": user_input})

        if stream:
            full_response = []
            for chunk in self.client.chat(self.messages, model=self.model, stream=True):
                full_response.append(chunk)
                yield chunk
            # Save assistant response to history
            self.messages.append({"role": "assistant", "content": "".join(full_response)})
        else:
            response = self.client.chat(self.messages, model=self.model, stream=False)
            self.messages.append({"role": "assistant", "content": response})
            return response

    def clear(self):
        """Clear conversation history (keep system prompt if any)."""
        system = next((m for m in self.messages if m["role"] == "system"), None)
        self.messages = [system] if system else []


def print_colored(text: str, color: str = "default", end: str = "\n"):
    """Print colored text to terminal."""
    colors = {
        "red": "\033[91m",
        "green": "\033[92m",
        "yellow": "\033[93m",
        "blue": "\033[94m",
        "magenta": "\033[95m",
        "cyan": "\033[96m",
        "white": "\033[97m",
        "gray": "\033[90m",
        "default": "\033[0m",
    }
    print(f"{colors.get(color, '')}{text}\033[0m", end=end, flush=True)


def print_banner():
    """Print welcome banner."""
    print()
    print_colored("╔════════════════════════════════════════════════════════════╗", "cyan")
    print_colored("║            🚀 ModelGate Chat Client                       ║", "cyan")
    print_colored("╚════════════════════════════════════════════════════════════╝", "cyan")
    print()


def print_help():
    """Print help information."""
    print_colored("\nCommands:", "yellow")
    print_colored("  /help    - Show this help message", "gray")
    print_colored("  /clear   - Clear conversation history", "gray")
    print_colored("  /model   - Show current model", "gray")
    print_colored("  /models  - List available models", "gray")
    print_colored("  /history - Show conversation history", "gray")
    print_colored("  /quit    - Exit the chat", "gray")
    print()


def main():
    parser = argparse.ArgumentParser(description="ModelGate Chat Client")
    parser.add_argument(
        "--model",
        "-m",
        default="openai/gpt-5.1",
        help="Model to use (default: openai/gpt-5.1)",
    )
    parser.add_argument(
        "--api-key",
        "-k",
        default=None,
        help="API key for authentication (or set MODELGATE_API_KEY env var)",
    )
    parser.add_argument(
        "--base-url",
        "-u",
        default="http://localhost:8080",
        help="ModelGate server URL (default: http://localhost:8080)",
    )
    parser.add_argument(
        "--system",
        "-s",
        default=None,
        help="System prompt for the conversation",
    )
    parser.add_argument(
        "--no-stream",
        action="store_true",
        help="Disable streaming (wait for complete response)",
    )

    args = parser.parse_args()

    # Initialize client
    client = ModelGateClient(base_url=args.base_url, api_key=args.api_key)

    # Check server health
    print_banner()
    print_colored("Connecting to ModelGate...", "gray", end=" ")

    if not client.health_check():
        print_colored("❌ Failed", "red")
        print_colored(f"\nCannot connect to {args.base_url}", "red")
        print_colored("Make sure the ModelGate server is running.", "gray")
        sys.exit(1)

    print_colored("✓ Connected", "green")
    print_colored(f"Model: {args.model}", "gray")
    print_colored("Type /help for commands, /quit to exit.\n", "gray")

    # Start chat session
    session = ChatSession(client, model=args.model, system_prompt=args.system)
    stream = not args.no_stream

    while True:
        try:
            # Get user input
            print_colored("You: ", "green", end="")
            user_input = input().strip()

            if not user_input:
                continue

            # Handle commands
            if user_input.startswith("/"):
                cmd = user_input.lower()

                if cmd in ("/quit", "/exit", "/q"):
                    print_colored("\nGoodbye! 👋", "cyan")
                    break

                elif cmd == "/help":
                    print_help()
                    continue

                elif cmd == "/clear":
                    session.clear()
                    print_colored("✓ Conversation cleared\n", "green")
                    continue

                elif cmd == "/model":
                    print_colored(f"Current model: {session.model}\n", "cyan")
                    continue

                elif cmd == "/models":
                    print_colored("Available models:", "cyan")
                    try:
                        models = client.list_models()
                        for m in models:
                            print_colored(f"  • {m['id']} ({m.get('owned_by', 'unknown')})", "gray")
                    except Exception as e:
                        print_colored(f"  Error: {e}", "red")
                    print()
                    continue

                elif cmd == "/history":
                    print_colored("Conversation history:", "cyan")
                    for msg in session.messages:
                        role = msg["role"].capitalize()
                        content = msg["content"][:100] + "..." if len(msg["content"]) > 100 else msg["content"]
                        print_colored(f"  [{role}] {content}", "gray")
                    print()
                    continue

                else:
                    print_colored(f"Unknown command: {user_input}. Type /help for commands.\n", "yellow")
                    continue

            # Send message and display response
            print_colored("Assistant: ", "blue", end="")

            try:
                if stream:
                    for chunk in session.send(user_input, stream=True):
                        print(chunk, end="", flush=True)
                    print()  # Newline after streaming
                else:
                    response = session.send(user_input, stream=False)
                    print(response)
            except requests.exceptions.HTTPError as e:
                print_colored(f"\n❌ Error: {e}", "red")
                if hasattr(e, "response") and e.response is not None:
                    try:
                        error_detail = e.response.json()
                        print_colored(f"   {error_detail}", "gray")
                    except:
                        pass
            except requests.exceptions.ConnectionError:
                print_colored("\n❌ Connection lost. Is the server still running?", "red")
            except Exception as e:
                print_colored(f"\n❌ Error: {e}", "red")

            print()

        except KeyboardInterrupt:
            print_colored("\n\nGoodbye! 👋", "cyan")
            break

        except EOFError:
            print_colored("\nGoodbye! 👋", "cyan")
            break


if __name__ == "__main__":
    main()

