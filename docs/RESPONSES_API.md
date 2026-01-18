# ModelGate Responses API

The `/v1/responses` endpoint provides **guaranteed structured JSON output** that conforms to a provided JSON schema. This is a separate endpoint from `/v1/chat/completions`, designed specifically for structured data extraction.

## Overview

### What It Does

The Responses API ensures:
1. **Strict JSON Schema Adherence** - Output is guaranteed to match your schema
2. **No Invalid JSON** - Unlike `response_format: json_object`, this provides true guarantees
3. **Schema Validation** - The schema is validated and enforced at the API level
4. **Multi-Provider Support** - Works with any provider through intelligent fallbacks

### How It Differs from Chat Completions

| Feature | `/v1/chat/completions` | `/v1/responses` |
|---------|------------------------|-----------------|
| **Purpose** | General chat | Structured data extraction |
| **Output** | Freeform text/JSON | Guaranteed JSON matching schema |
| **Schema** | Optional `response_format` | Required `response_schema` |
| **Validation** | Best-effort | Strict validation |
| **Response Field** | `choices[0].message.content` | `response` (parsed JSON) |

## Quick Start

### Python Example

```python
import requests
import json

API_KEY = "your-modelgate-api-key"
BASE_URL = "http://localhost:8080"

response = requests.post(
    f"{BASE_URL}/v1/responses",
    headers={
        "Authorization": f"Bearer {API_KEY}",
        "Content-Type": "application/json"
    },
    json={
        "model": "openai/gpt-4o",
        "messages": [
            {"role": "user", "content": "What is 2 + 2?"}
        ],
        "response_schema": {
            "name": "math_result",
            "description": "Mathematical calculation result",
            "schema": {
                "type": "object",
                "properties": {
                    "result": {"type": "number"},
                    "explanation": {"type": "string"}
                },
                "required": ["result", "explanation"]
            }
        }
    }
)

data = response.json()
print(f"Result: {data['response']['result']}")
print(f"Explanation: {data['response']['explanation']}")
```

### curl Example

```bash
curl -X POST http://localhost:8080/v1/responses \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "openai/gpt-4o",
    "messages": [
      {"role": "user", "content": "Extract: John Doe, john@example.com, 555-1234"}
    ],
    "response_schema": {
      "name": "contact_info",
      "schema": {
        "type": "object",
        "properties": {
          "name": {"type": "string"},
          "email": {"type": "string", "format": "email"},
          "phone": {"type": "string"}
        },
        "required": ["name", "email", "phone"]
      }
    }
  }'
```

## API Reference

### Request Format

```json
{
  "model": "openai/gpt-4o",
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "Your prompt here"}
  ],
  "response_schema": {
    "name": "schema_name",
    "description": "Optional description",
    "schema": {
      "type": "object",
      "properties": {...},
      "required": [...]
    },
    "strict": true
  },
  "temperature": 0.7,
  "max_tokens": 500
}
```

### Response Format

```json
{
  "id": "resp_abc123...",
  "object": "response",
  "created": 1704000000,
  "model": "gpt-4o",
  "response": {
    "result": 4,
    "explanation": "2 plus 2 equals 4"
  },
  "usage": {
    "prompt_tokens": 50,
    "completion_tokens": 30,
    "total_tokens": 80
  }
}
```

### Response Headers

ModelGate adds metadata headers to indicate how the response was generated:

| Header | Description | Example |
|--------|-------------|---------|
| `X-ModelGate-Provider` | Provider used | `openai` |
| `X-ModelGate-Implementation-Mode` | Strategy used | `native`, `json_mode`, `prompt_based` |
| `X-ModelGate-Schema-Validated` | Schema validation passed | `true` |
| `X-ModelGate-Retry-Count` | Retries (prompt-based only) | `0` |

## Provider Support

### Implementation Strategies

ModelGate uses different strategies based on provider capabilities:

| Provider | Strategy | Description |
|----------|----------|-------------|
| **OpenAI** | `native` | Uses OpenAI's native `/v1/responses` endpoint |
| **Azure OpenAI** | `native` | Uses Azure's `/openai/responses` endpoint |
| **Groq** | `json_mode` | Uses chat completions with JSON mode + validation |
| **Gemini** | `json_mode` | Uses chat completions with JSON mode + validation |
| **Together AI** | `json_mode` | Uses chat completions with JSON mode + validation |
| **Cohere** | `json_mode` | Uses chat completions with JSON mode + validation |
| **Anthropic** | `prompt_based` | Schema injection in prompt + validation + retries |
| **Bedrock** | `prompt_based` | Schema injection in prompt + validation + retries |
| **Mistral** | `prompt_based` | Schema injection in prompt + validation + retries |
| **Ollama** | `prompt_based` | Schema injection in prompt + validation + retries |

### Strategy Details

**Native (OpenAI/Azure)**:
- Direct API call to provider's responses endpoint
- Highest accuracy and performance
- Schema validation handled by provider

**JSON Mode**:
- Uses `response_format: { type: "json_object" }` in chat completions
- Schema instructions added to system prompt
- Post-validation against JSON schema

**Prompt-Based**:
- Schema injected into system prompt with detailed instructions
- Automatic retries (up to 3) on validation failure
- JSON extraction from response (handles code blocks, mixed text)

## JSON Schema Support

### Supported Schema Features

```json
{
  "type": "object",
  "properties": {
    "name": {"type": "string", "description": "Person's name"},
    "age": {"type": "integer", "minimum": 0},
    "email": {"type": "string", "format": "email"},
    "status": {"type": "string", "enum": ["active", "inactive"]},
    "tags": {
      "type": "array",
      "items": {"type": "string"}
    },
    "address": {
      "type": "object",
      "properties": {
        "city": {"type": "string"},
        "country": {"type": "string"}
      }
    }
  },
  "required": ["name", "email"],
  "additionalProperties": false
}
```

### Best Practices

1. **Use descriptive names**: Schema names should be meaningful (e.g., `contact_info`, `order_summary`)
2. **Add descriptions**: Property descriptions help the model understand intent
3. **Specify required fields**: Always list required properties explicitly
4. **Use enums for choices**: When values are limited, use `enum` for accuracy
5. **Set `additionalProperties: false`**: Prevents extra fields in output

## Examples

### Contact Extraction

```python
schema = {
    "name": "contact_info",
    "schema": {
        "type": "object",
        "properties": {
            "name": {"type": "string"},
            "email": {"type": "string", "format": "email"},
            "phone": {"type": "string"},
            "company": {"type": "string"},
            "role": {"type": "string"}
        },
        "required": ["name", "email"]
    }
}

messages = [
    {"role": "user", "content": "Extract: John Doe, Senior Engineer at Acme Corp. john@acme.com, 555-0123"}
]
```

### Data Classification

```python
schema = {
    "name": "sentiment_analysis",
    "schema": {
        "type": "object",
        "properties": {
            "sentiment": {
                "type": "string",
                "enum": ["positive", "negative", "neutral"]
            },
            "confidence": {
                "type": "number",
                "minimum": 0,
                "maximum": 1
            },
            "key_phrases": {
                "type": "array",
                "items": {"type": "string"}
            }
        },
        "required": ["sentiment", "confidence"]
    }
}
```

### Task Breakdown

```python
schema = {
    "name": "project_tasks",
    "schema": {
        "type": "object",
        "properties": {
            "project_name": {"type": "string"},
            "tasks": {
                "type": "array",
                "items": {
                    "type": "object",
                    "properties": {
                        "id": {"type": "integer"},
                        "title": {"type": "string"},
                        "priority": {"type": "string", "enum": ["low", "medium", "high", "critical"]},
                        "estimated_hours": {"type": "number"}
                    },
                    "required": ["id", "title", "priority"]
                }
            }
        },
        "required": ["project_name", "tasks"]
    }
}
```

## Error Handling

### Common Errors

```json
{
  "error": {
    "type": "invalid_request",
    "message": "Invalid JSON body"
  }
}
```

```json
{
  "error": {
    "type": "generation_error",
    "message": "failed after 3 attempts: validation failed: missing required field 'name'"
  }
}
```

### Error Types

| Error Type | Description |
|------------|-------------|
| `invalid_request` | Malformed request (bad JSON, missing fields) |
| `generation_error` | Model failed to generate valid output |
| `policy_violation` | Request blocked by policy engine |
| `provider_error` | Upstream provider error |

## Running the Examples

```bash
cd examples

# Install dependencies
pip install requests

# Run basic examples
python responses_basic.py

# With custom API key
python responses_basic.py --api-key mg_your_key

# With custom server
python responses_basic.py --base-url http://your-server:8080
```

## Architecture

```
Client Request (POST /v1/responses)
         │
         ▼
┌─────────────────┐
│   HTTP Server   │
│ (handleResponses)│
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Policy Engine   │
│ (auth, limits)  │
└────────┬────────┘
         │
         ▼
┌─────────────────────────────────────┐
│         Responses Service           │
│  ┌─────────────────────────────┐    │
│  │   Strategy Selection        │    │
│  │   OpenAI → Native           │    │
│  │   Groq → JSON Mode          │    │
│  │   Claude → Prompt-Based     │    │
│  └─────────────┬───────────────┘    │
│                │                    │
│  ┌─────────────▼───────────────┐    │
│  │   Provider Client           │    │
│  │   (OpenAI, Anthropic, etc)  │    │
│  └─────────────┬───────────────┘    │
│                │                    │
│  ┌─────────────▼───────────────┐    │
│  │   Schema Validator          │    │
│  │   (gojsonschema)            │    │
│  └─────────────────────────────┘    │
└─────────────────────────────────────┘
         │
         ▼
    JSON Response
```

## Related Documentation

- [OpenAI Responses API Reference](https://platform.openai.com/docs/api-reference/responses)
- [JSON Schema Specification](https://json-schema.org/)
- [ModelGate Policy Framework](./POLICY_DRIVEN_FEATURES.md)

