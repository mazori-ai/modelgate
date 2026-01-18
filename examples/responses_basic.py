#!/usr/bin/env python3
"""
ModelGate Responses API - Basic Example

Demonstrates the /v1/responses endpoint for structured outputs.
This endpoint guarantees JSON output that conforms to a provided schema.

Usage:
    python responses_basic.py [--api-key KEY] [--base-url URL]

Examples:
    # Basic usage
    python responses_basic.py

    # With custom API key
    python responses_basic.py --api-key mg_abc123...
"""

import argparse
import json
import os
import sys

try:
    import requests
except ImportError:
    print("Error: 'requests' library is required. Install with: pip install requests")
    sys.exit(1)


class ModelGateResponsesClient:
    """Client for ModelGate /v1/responses API (structured outputs)."""

    def __init__(self, base_url: str = "http://localhost:8080", api_key: str | None = None):
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key or os.environ.get("MODELGATE_API_KEY", "")
        self.session = requests.Session()
        if self.api_key:
            self.session.headers["Authorization"] = f"Bearer {self.api_key}"

    def generate_response(
        self,
        messages: list[dict],
        response_schema: dict,
        model: str = "openai/gpt-5.1",
        temperature: float = 0.7,
        max_tokens: int | None = None,
    ) -> dict:
        """
        Generate a structured response that conforms to a JSON schema.

        Args:
            messages: List of message dicts with 'role' and 'content'
            response_schema: JSON schema definition with 'name', 'schema', etc.
            model: Model identifier (e.g., 'openai/gpt-4o', 'claude-sonnet-4')
            temperature: Sampling temperature (0.0-1.0)
            max_tokens: Maximum tokens in response

        Returns:
            Dict containing 'id', 'response', 'usage', and metadata
        """
        payload = {
            "model": model,
            "messages": messages,
            "response_schema": response_schema,
            "temperature": temperature,
        }
        if max_tokens:
            payload["max_tokens"] = max_tokens

        response = self.session.post(
            f"{self.base_url}/v1/responses",
            json=payload,
            timeout=30,
        )
        response.raise_for_status()

        return response.json()

    def health_check(self) -> bool:
        """Check if the server is healthy."""
        try:
            response = self.session.get(f"{self.base_url}/health", timeout=5)
            return response.status_code == 200
        except requests.RequestException:
            return False


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


def example_1_simple_math():
    """Example 1: Simple mathematical calculation with structured output."""
    print_colored("\n" + "=" * 70, "cyan")
    print_colored("Example 1: Simple Math Calculation", "cyan")
    print_colored("=" * 70, "cyan")

    client = ModelGateResponsesClient()

    # Define the schema for a math result
    schema = {
        "name": "math_result",
        "description": "Result of a mathematical calculation",
        "schema": {
            "type": "object",
            "properties": {
                "calculation": {
                    "type": "string",
                    "description": "The mathematical expression"
                },
                "result": {
                    "type": "number",
                    "description": "The numerical result"
                },
                "explanation": {
                    "type": "string",
                    "description": "Step-by-step explanation"
                }
            },
            "required": ["calculation", "result", "explanation"],
            "additionalProperties": False
        }
    }

    messages = [
        {"role": "user", "content": "What is 2 + 2?"}
    ]

    print_colored("\nRequest:", "yellow")
    print_colored(f"  Prompt: {messages[0]['content']}", "gray")
    print_colored(f"  Schema: math_result (calculation, result, explanation)", "gray")

    try:
        response = client.generate_response(messages, schema)

        print_colored("\nResponse:", "green")
        print_colored(json.dumps(response["response"], indent=2), "white")

        print_colored("\nMetadata:", "blue")
        print_colored(f"  Model: {response['model']}", "gray")
        print_colored(f"  Tokens: {response['usage']['total_tokens']}", "gray")

        # Check for implementation mode header
        print_colored(f"  Response ID: {response['id']}", "gray")

    except requests.exceptions.HTTPError as e:
        print_colored(f"\n❌ Error: {e}", "red")
        if hasattr(e, "response") and e.response is not None:
            try:
                error_detail = e.response.json()
                print_colored(f"   {json.dumps(error_detail, indent=2)}", "gray")
            except:
                pass
    except Exception as e:
        print_colored(f"\n❌ Error: {e}", "red")


def example_2_contact_extraction():
    """Example 2: Extract structured contact information from text."""
    print_colored("\n" + "=" * 70, "cyan")
    print_colored("Example 2: Contact Information Extraction", "cyan")
    print_colored("=" * 70, "cyan")

    client = ModelGateResponsesClient()

    # Define the schema for contact info
    schema = {
        "name": "contact_info",
        "description": "Structured contact information",
        "schema": {
            "type": "object",
            "properties": {
                "name": {
                    "type": "string",
                    "description": "Full name of the person"
                },
                "email": {
                    "type": "string",
                    "format": "email",
                    "description": "Email address"
                },
                "phone": {
                    "type": "string",
                    "description": "Phone number"
                },
                "company": {
                    "type": "string",
                    "description": "Company name"
                },
                "role": {
                    "type": "string",
                    "description": "Job title or role"
                }
            },
            "required": ["name", "email"],
            "additionalProperties": False
        }
    }

    messages = [
        {
            "role": "user",
            "content": "Extract the contact information: John Doe works as a Senior Engineer at Tech Corp. You can reach him at john.doe@techcorp.com or call 555-0123."
        }
    ]

    print_colored("\nRequest:", "yellow")
    print_colored(f"  Prompt: {messages[0]['content'][:80]}...", "gray")
    print_colored(f"  Schema: contact_info (name, email, phone, company, role)", "gray")

    try:
        response = client.generate_response(messages, schema)

        print_colored("\nExtracted Contact:", "green")
        contact = response["response"]
        for key, value in contact.items():
            print_colored(f"  {key.capitalize()}: {value}", "white")

        print_colored(f"\nTokens used: {response['usage']['total_tokens']}", "gray")

    except Exception as e:
        print_colored(f"\n❌ Error: {e}", "red")


def example_3_task_breakdown():
    """Example 3: Break down a project into structured tasks."""
    print_colored("\n" + "=" * 70, "cyan")
    print_colored("Example 3: Project Task Breakdown", "cyan")
    print_colored("=" * 70, "cyan")

    client = ModelGateResponsesClient()

    # Define the schema for project tasks
    schema = {
        "name": "project_plan",
        "description": "Structured project plan with tasks",
        "schema": {
            "type": "object",
            "properties": {
                "project_name": {
                    "type": "string",
                    "description": "Name of the project"
                },
                "tasks": {
                    "type": "array",
                    "description": "List of tasks",
                    "items": {
                        "type": "object",
                        "properties": {
                            "id": {
                                "type": "integer",
                                "description": "Task number"
                            },
                            "title": {
                                "type": "string",
                                "description": "Task title"
                            },
                            "description": {
                                "type": "string",
                                "description": "Task description"
                            },
                            "priority": {
                                "type": "string",
                                "enum": ["low", "medium", "high", "critical"],
                                "description": "Task priority"
                            },
                            "estimated_hours": {
                                "type": "number",
                                "description": "Estimated hours to complete"
                            }
                        },
                        "required": ["id", "title", "priority"]
                    }
                }
            },
            "required": ["project_name", "tasks"]
        }
    }

    messages = [
        {
            "role": "user",
            "content": "Create a project plan for building a simple todo list web application. Break it down into 5 main tasks with priorities."
        }
    ]

    print_colored("\nRequest:", "yellow")
    print_colored(f"  Prompt: {messages[0]['content']}", "gray")
    print_colored(f"  Schema: project_plan (project_name, tasks[])", "gray")

    try:
        response = client.generate_response(messages, schema)

        print_colored("\nProject Plan:", "green")
        plan = response["response"]
        print_colored(f"  Project: {plan['project_name']}\n", "white")

        for task in plan["tasks"]:
            priority_color = {
                "low": "gray",
                "medium": "blue",
                "high": "yellow",
                "critical": "red"
            }.get(task.get("priority", "medium"), "gray")

            print_colored(f"  [{task['id']}] {task['title']}", "white")
            if "description" in task:
                print_colored(f"      {task['description']}", "gray")
            print_colored(f"      Priority: {task.get('priority', 'N/A')}", priority_color)
            if "estimated_hours" in task:
                print_colored(f"      Estimate: {task['estimated_hours']}h", "gray")
            print()

        print_colored(f"Total tokens used: {response['usage']['total_tokens']}", "gray")

    except Exception as e:
        print_colored(f"\n❌ Error: {e}", "red")


def main():
    parser = argparse.ArgumentParser(description="ModelGate Responses API Basic Examples")
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

    args = parser.parse_args()

    # Initialize client and check health
    client = ModelGateResponsesClient(base_url=args.base_url, api_key=args.api_key)

    print_colored("\n╔════════════════════════════════════════════════════════════╗", "cyan")
    print_colored("║     🚀 ModelGate Responses API - Basic Examples           ║", "cyan")
    print_colored("╚════════════════════════════════════════════════════════════╝", "cyan")

    print_colored("\nConnecting to ModelGate...", "gray", end=" ")

    if not client.health_check():
        print_colored("❌ Failed", "red")
        print_colored(f"\nCannot connect to {args.base_url}", "red")
        print_colored("Make sure the ModelGate server is running.", "gray")
        sys.exit(1)

    print_colored("✓ Connected", "green")
    print_colored(f"Server: {args.base_url}", "gray")

    # Run examples
    try:
        example_1_simple_math()
        example_2_contact_extraction()
        example_3_task_breakdown()

        print_colored("\n" + "=" * 70, "cyan")
        print_colored("✓ All examples completed successfully!", "green")
        print_colored("=" * 70, "cyan")
        print()

    except KeyboardInterrupt:
        print_colored("\n\nExecution interrupted by user", "yellow")
    except Exception as e:
        print_colored(f"\n❌ Unexpected error: {e}", "red")
        import traceback
        traceback.print_exc()


if __name__ == "__main__":
    main()
