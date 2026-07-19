# Gumi API Specification

Version: 1.0  
Status: Draft  
Scope: Gumi Runtime V1 API contract

---

# 1. Purpose

This document defines the public API contract for Gumi Runtime V1.

Gumi exposes an OpenAI-compatible API so existing AI applications can connect to Gumi with minimal or no code changes.

The API must be:

- predictable
- stable
- local-first
- OpenAI-compatible where possible
- easy to debug
- easy to extend
- suitable for local development and future production usage

---

# 2. API Design Goals

Gumi API must support:

1. Existing OpenAI-compatible clients.
2. Local model routing.
3. Provider abstraction.
4. Streaming and non-streaming chat.
5. Structured output validation.
6. Runtime metadata.
7. Clear error reporting.
8. Local observability.

---

# 3. Base URL

Default local runtime URL:

```text
http://localhost:8787/v1
```

Example environment variables:

```bash
export OPENAI_BASE_URL=http://localhost:8787/v1
export OPENAI_API_KEY=gumi-local
```

For local V1, the API key can be a static local key unless authentication is disabled in config.

---

# 4. Authentication

## 4.1 Header

Gumi should support OpenAI-style bearer authentication:

```http
Authorization: Bearer gumi-local
```

## 4.2 Local V1 Behaviour

In V1, authentication modes:

```yaml
auth:
  mode: local
```

Supported modes:

```text
disabled
local
api_key
```

## 4.3 Rules

- `disabled` is allowed for personal local use.
- `local` uses a default local development key.
- `api_key` validates API keys from local config or database.
- Future cloud/enterprise auth is out of scope for V1.

---

# 5. Content Type

All JSON endpoints use:

```http
Content-Type: application/json
```

Streaming endpoints use Server-Sent Events:

```http
Content-Type: text/event-stream
```

---

# 6. V1 Public Endpoints

Required V1 endpoints:

```http
GET  /health
GET  /v1/models
POST /v1/chat/completions
```

Gumi runtime endpoints:

```http
GET  /v1/gumi/status
GET  /v1/gumi/providers
GET  /v1/gumi/config
GET  /v1/gumi/telemetry/recent
POST /v1/gumi/doctor
```

Memory engine endpoints (shipped, opt-in):

```http
GET  /v1/gumi/memory/facts       # list/search stored facts
POST /v1/gumi/memory/facts       # store a new fact
GET  /v1/gumi/memory/model-fit   # model performance data for dashboard
POST /v1/gumi/memory/clear       # clear all memory data
GET  /v1/gumi/memory/status      # memory engine status (enabled, db_path, facts_count, etc.)
```

LM Studio model management endpoints (shipped, opt-in via `providers.lmstudio.model_management.enabled`):

```http
GET  /v1/gumi/lmstudio/models         # list available models from LM Studio
POST /v1/gumi/lmstudio/models/load    # load a model with optional config
POST /v1/gumi/lmstudio/models/unload  # unload a loaded model instance
```

Log streaming endpoint (shipped):

```http
GET  /v1/gumi/logs/stream  # SSE stream of runtime log entries (level, engine, request_id, message)
```

Config management endpoints (shipped):

```http
POST /v1/gumi/config/save  # save current runtime config to disk
```

Profile management endpoints (shipped):

```http
GET  /v1/gumi/profiles     # list available model profiles
POST /v1/gumi/profiles/test # test a profile with a sample request
```

Dashboard serving endpoint (shipped, embedded in Go binary):

```http
GET /dashboard/  # SPA dashboard at http://127.0.0.1:8788 (served by separate HTTP server)
```

Future endpoints:

```http
GET  /v1/gumi/sessions
GET  /v1/gumi/sessions/{session_id}
```

---

# 7. Health Endpoint

## 7.1 Request

```http
GET /health
```

## 7.2 Response

```json
{
  "status": "ok",
  "runtime": "gumi",
  "version": "0.1.0",
  "mode": "local",
  "timestamp": "2026-07-10T00:00:00Z"
}
```

## 7.3 Status Values

```text
ok
degraded
error
```

---

# 8. Models Endpoint

## 8.1 Request

```http
GET /v1/models
```

## 8.2 OpenAI-Compatible Response

```json
{
  "object": "list",
  "data": [
    {
      "id": "local:auto",
      "object": "model",
      "created": 0,
      "owned_by": "gumi"
    },
    {
      "id": "ollama:qwen3:8b",
      "object": "model",
      "created": 0,
      "owned_by": "ollama"
    },
    {
      "id": "lmstudio:local-model",
      "object": "model",
      "created": 0,
      "owned_by": "lmstudio"
    }
  ]
}
```

## 8.3 Model Naming Convention

Gumi model IDs should follow:

```text
provider:model
```

Examples:

```text
local:auto
ollama:qwen3:8b
ollama:deepseek-r1:8b
lmstudio:qwen2.5-coder-7b
openai-compatible:local-model
```

## 8.4 Special Model IDs

```text
local:auto
```

Means Gumi chooses the best configured local model.

```text
provider:auto
```

Means Gumi chooses the best model from that provider.

---

# 9. Chat Completions Endpoint

## 9.1 Request

```http
POST /v1/chat/completions
```

## 9.2 Basic Request Body

```json
{
  "model": "local:auto",
  "messages": [
    {
      "role": "user",
      "content": "Explain Docker in simple terms."
    }
  ],
  "temperature": 0.4,
  "stream": false
}
```

## 9.3 Supported OpenAI-Compatible Fields

V1 should accept:

```text
model
messages
temperature
top_p
max_tokens
stream
stop
presence_penalty
frequency_penalty
response_format
tools
tool_choice
metadata
```

V1 does not need to fully implement tools yet, but it should accept and preserve unsupported fields where possible.

---

# 10. Message Format

## 10.1 Supported Roles

```text
system
developer
user
assistant
tool
```

## 10.2 Basic Message

```json
{
  "role": "user",
  "content": "What is local AI?"
}
```

## 10.3 Text Content

```json
{
  "role": "user",
  "content": "Summarize this paragraph."
}
```

## 10.4 Multi-Part Content

Optional V1 support:

```json
{
  "role": "user",
  "content": [
    {
      "type": "text",
      "text": "Describe this image."
    }
  ]
}
```

Vision/image input is out of scope for V1 unless provider supports it and the adapter can pass through safely.

---

# 11. Gumi Request Extensions

Gumi supports optional extension fields.

These fields should not break OpenAI-compatible clients because they are optional.

## 11.1 Runtime Mode

```json
{
  "model": "local:auto",
  "messages": [
    {
      "role": "user",
      "content": "Give me JSON output."
    }
  ],
  "gumi": {
    "mode": "stabilized"
  }
}
```

Supported modes:

```text
direct
lightweight
stabilized
structured
agent
```

V1 supports:

```text
direct
lightweight
stabilized
structured
agent
```

Agent mode enables the agentic coding pipeline: the Agentic Coding Router
(see [19-agentic-coding-router-specification.md](./19-agentic-coding-router-specification.md))
classifies each step and routes to the best local model. The Memory Engine
(see [20-memory-engine-specification.md](./20-memory-engine-specification.md))
injects relevant facts and records outcomes. LM Studio model management
auto-loads the selected model before generation.

---

## 11.2 Guard Configuration

```json
{
  "gumi": {
    "guard": {
      "anti_loop": true,
      "structured_output": true,
      "context_overflow": true
    }
  }
}
```

---

## 11.3 Context Configuration

```json
{
  "gumi": {
    "context": {
      "strategy": "hybrid",
      "max_input_tokens": 16000,
      "preserve_recent_messages": 8
    }
  }
}
```

Supported context strategies:

```text
none
trim
summarize
compress
hybrid
```

---

## 11.4 Validation Configuration

```json
{
  "gumi": {
    "validation": {
      "enabled": true,
      "repair": true
    }
  }
}
```

---

## 11.5 Session Configuration

```json
{
  "gumi": {
    "session": {
      "id": "session_abc123",
      "persist": true
    }
  }
}
```

---

## 11.6 Telemetry Configuration

```json
{
  "gumi": {
    "telemetry": {
      "include_metadata": true,
      "include_pipeline_events": false
    }
  }
}
```

---

# 12. Response Format

## 12.1 Basic OpenAI-Compatible Response

```json
{
  "id": "chatcmpl_nvx_123",
  "object": "chat.completion",
  "created": 1783651200,
  "model": "ollama:qwen3:8b",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Docker is a tool that packages applications..."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 120,
    "completion_tokens": 80,
    "total_tokens": 200
  }
}
```

## 12.2 Gumi Metadata Extension

When requested, Gumi may include runtime metadata:

```json
{
  "id": "chatcmpl_nvx_123",
  "object": "chat.completion",
  "created": 1783651200,
  "model": "ollama:qwen3:8b",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Docker is a tool that packages applications..."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 120,
    "completion_tokens": 80,
    "total_tokens": 200
  },
  "gumi": {
    "request_id": "req_abc123",
    "provider": "ollama",
    "runtime_mode": "stabilized",
    "context_compressed": true,
    "validation_passed": true,
    "repair_applied": false,
    "retry_count": 0,
    "latency_ms": 842
  }
}
```

## 12.3 Metadata Rule

Gumi metadata should be disabled by default for strict compatibility.

Enable using:

```json
{
  "gumi": {
    "telemetry": {
      "include_metadata": true
    }
  }
}
```

---

# 13. Streaming Response

## 13.1 Request

```json
{
  "model": "local:auto",
  "messages": [
    {
      "role": "user",
      "content": "Write a short explanation of APIs."
    }
  ],
  "stream": true
}
```

## 13.2 Response Content Type

```http
Content-Type: text/event-stream
```

## 13.3 Streaming Chunk Format

```text
data: {"id":"chatcmpl_nvx_123","object":"chat.completion.chunk","created":1783651200,"model":"ollama:qwen3:8b","choices":[{"index":0,"delta":{"role":"assistant"},"finish_reason":null}]}

data: {"id":"chatcmpl_nvx_123","object":"chat.completion.chunk","created":1783651200,"model":"ollama:qwen3:8b","choices":[{"index":0,"delta":{"content":"An API"},"finish_reason":null}]}

data: {"id":"chatcmpl_nvx_123","object":"chat.completion.chunk","created":1783651200,"model":"ollama:qwen3:8b","choices":[{"index":0,"delta":{"content":" lets apps communicate."},"finish_reason":null}]}

data: {"id":"chatcmpl_nvx_123","object":"chat.completion.chunk","created":1783651200,"model":"ollama:qwen3:8b","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]
```

---

# 14. Structured Output

## 14.1 JSON Object Mode

```json
{
  "model": "local:auto",
  "messages": [
    {
      "role": "user",
      "content": "Return a profile for Gumi."
    }
  ],
  "response_format": {
    "type": "json_object"
  },
  "gumi": {
    "mode": "structured",
    "validation": {
      "enabled": true,
      "repair": true
    }
  }
}
```

## 14.2 JSON Schema Mode

```json
{
  "model": "local:auto",
  "messages": [
    {
      "role": "user",
      "content": "Score this answer."
    }
  ],
  "response_format": {
    "type": "json_schema",
    "json_schema": {
      "name": "score_result",
      "schema": {
        "type": "object",
        "properties": {
          "score": {
            "type": "number"
          },
          "reason": {
            "type": "string"
          }
        },
        "required": ["score", "reason"],
        "additionalProperties": false
      }
    }
  },
  "gumi": {
    "mode": "structured"
  }
}
```

## 14.3 Structured Output Behaviour

When structured output is requested:

1. Prompt Engine adds strict format instructions.
2. Provider Engine sends the request.
3. Validation Engine validates the output.
4. Repair Engine repairs if possible.
5. If repair fails, Pipeline Engine retries once.
6. If still invalid, return structured error.

---

# 15. Error Format

## 15.1 Standard Error Response

```json
{
  "error": {
    "code": "PROVIDER_UNAVAILABLE",
    "message": "Ollama is not reachable at http://localhost:11434.",
    "type": "provider_error",
    "engine": "provider",
    "retryable": true,
    "suggestion": "Start Ollama or update providers.ollama.url in gumi.yaml.",
    "request_id": "req_abc123"
  }
}
```

## 15.2 Error Fields

```text
code
message
type
engine
retryable
suggestion
request_id
details
```

`details` is optional.

---

# 16. Error Categories

```text
request_error
auth_error
config_error
workspace_error
session_error
context_error
prompt_error
guard_error
provider_error
response_error
validation_error
repair_error
plugin_error
runtime_error
```

---

# 17. Error Codes

## 17.1 Request Errors

```text
INVALID_REQUEST
MISSING_MESSAGES
INVALID_MESSAGES
UNSUPPORTED_ROLE
UNSUPPORTED_CONTENT_TYPE
```

## 17.2 Auth Errors

```text
MISSING_API_KEY
INVALID_API_KEY
AUTH_DISABLED
```

## 17.3 Config Errors

```text
INVALID_CONFIG
MISSING_PROVIDER
MISSING_MODEL
INVALID_RUNTIME_MODE
```

## 17.4 Provider Errors

```text
PROVIDER_UNAVAILABLE
PROVIDER_TIMEOUT
PROVIDER_BAD_RESPONSE
MODEL_NOT_FOUND
MODEL_UNSUPPORTED
STREAMING_UNSUPPORTED
```

## 17.5 Context Errors

```text
CONTEXT_LIMIT_EXCEEDED
CONTEXT_COMPRESSION_FAILED
CONTEXT_EMPTY_AFTER_PROCESSING
```

## 17.6 Validation Errors

```text
VALIDATION_FAILED
INVALID_JSON
INVALID_SCHEMA
EMPTY_RESPONSE
REPEATED_OUTPUT
```

## 17.7 Repair Errors

```text
REPAIR_FAILED
RETRY_LIMIT_EXCEEDED
```

## 17.8 Plugin Errors

```text
PLUGIN_LOAD_FAILED
PLUGIN_HOOK_FAILED
PLUGIN_PERMISSION_DENIED
```

---

# 18. Runtime Status Endpoint

## 18.1 Request

```http
GET /v1/gumi/status
```

## 18.2 Response

```json
{
  "runtime": {
    "name": "gumi",
    "version": "0.1.0",
    "mode": "local",
    "uptime_seconds": 3600
  },
  "gateway": {
    "status": "ok",
    "port": 8787
  },
  "dashboard": {
    "status": "ok",
    "url": "http://localhost:8788"
  },
  "providers": [
    {
      "name": "ollama",
      "status": "ok",
      "url": "http://localhost:11434",
      "models": 4
    }
  ],
  "engines": {
    "pipeline": "ok",
    "context": "ok",
    "prompt": "ok",
    "validation": "ok",
    "repair": "ok",
    "telemetry": "ok"
  }
}
```

---

# 19. Providers Endpoint

## 19.1 Request

```http
GET /v1/gumi/providers
```

## 19.2 Response

```json
{
  "providers": [
    {
      "name": "ollama",
      "status": "ok",
      "url": "http://localhost:11434",
      "default_model": "qwen3:8b",
      "capabilities": {
        "chat": true,
        "streaming": true,
        "embeddings": false,
        "tools": false,
        "vision": false
      }
    },
    {
      "name": "lmstudio",
      "status": "offline",
      "url": "http://localhost:1234",
      "default_model": null,
      "capabilities": {
        "chat": true,
        "streaming": true
      }
    }
  ]
}
```

---

# 20. Config Endpoint

## 20.1 Request

```http
GET /v1/gumi/config
```

## 20.2 Response

Sensitive values must be redacted.

```json
{
  "runtime": {
    "mode": "stabilized",
    "port": 8787
  },
  "provider": {
    "default": "ollama"
  },
  "providers": {
    "ollama": {
      "url": "http://localhost:11434",
      "default_model": "qwen3:8b"
    }
  },
  "telemetry": {
    "local": true,
    "external": false,
    "log_prompts": false
  }
}
```

---

# 21. Recent Telemetry Endpoint

## 21.1 Request

```http
GET /v1/gumi/telemetry/recent
```

## 21.2 Response

```json
{
  "items": [
    {
      "request_id": "req_abc123",
      "timestamp": "2026-07-10T00:00:00Z",
      "model": "ollama:qwen3:8b",
      "provider": "ollama",
      "runtime_mode": "stabilized",
      "latency_ms": 842,
      "context_compressed": true,
      "validation_passed": true,
      "repair_applied": false,
      "retry_count": 0,
      "status": "success"
    }
  ]
}
```

Default telemetry must not expose full prompt or response.

---

# 22. Doctor Endpoint

## 22.1 Request

```http
POST /v1/gumi/doctor
```

## 22.2 Response

```json
{
  "status": "warning",
  "checks": [
    {
      "name": "runtime",
      "status": "ok",
      "message": "Gumi runtime is running."
    },
    {
      "name": "ollama_provider",
      "status": "ok",
      "message": "Ollama is reachable."
    },
    {
      "name": "default_model",
      "status": "warning",
      "message": "Default model qwen3:8b is not installed.",
      "suggestion": "Run: ollama pull qwen3:8b"
    }
  ]
}
```

---

# 23. OpenAI Compatibility Level

Gumi V1 targets partial OpenAI compatibility.

## 23.1 Must Support

```text
/v1/models
/v1/chat/completions
non-streaming chat
streaming chat
basic usage fields
basic error format
```

## 23.2 Should Accept

```text
temperature
top_p
max_tokens
stop
response_format
metadata
```

## 23.3 May Ignore Safely in V1

```text
tools
tool_choice
parallel_tool_calls
logprobs
top_logprobs
seed
user
```

If ignored, Gumi should log a warning in telemetry.

---

# 24. Compatibility Rule

Gumi should not reject unknown OpenAI fields unless they create unsafe or impossible behaviour.

Unknown fields should be preserved where possible and passed to provider adapters only if supported.

---

# 25. Request ID

Every request must have a request ID.

If client provides:

```http
X-Request-ID: custom-id
```

Gumi should use it.

Otherwise Gumi generates:

```text
req_<random>
```

The request ID must appear in:

- telemetry
- logs
- errors
- optional response metadata

---

# 26. Headers

Supported request headers:

```http
Authorization
Content-Type
X-Request-ID
X-Gumi-Workspace
X-Gumi-Session
```

Response headers:

```http
X-Request-ID
X-Gumi-Provider
X-Gumi-Model
X-Gumi-Runtime-Mode
```

---

# 27. Rate Limiting

V1 local runtime does not need strict rate limiting.

Optional config:

```yaml
rate_limit:
  enabled: false
  requests_per_minute: 120
```

If enabled and exceeded:

```json
{
  "error": {
    "code": "RATE_LIMIT_EXCEEDED",
    "message": "Local rate limit exceeded.",
    "type": "runtime_error",
    "engine": "gateway",
    "retryable": true,
    "suggestion": "Wait before retrying or increase rate_limit.requests_per_minute."
  }
}
```

---

# 28. Timeout Rules

Default timeout config:

```yaml
timeout:
  request_seconds: 120
  provider_seconds: 90
  repair_seconds: 30
```

Timeout error:

```json
{
  "error": {
    "code": "PROVIDER_TIMEOUT",
    "message": "The provider did not respond within 90 seconds.",
    "type": "provider_error",
    "engine": "provider",
    "retryable": true,
    "suggestion": "Use a smaller model, reduce context size, or increase timeout.provider_seconds."
  }
}
```

---

# 29. API Versioning

V1 routes are under:

```text
/v1
```

Breaking changes require a new version:

```text
/v2
```

Gumi-specific extension fields should remain backwards compatible.

---

# 30. Lightweight Mode Example

## 30.1 Request with `gumi.mode = lightweight`

```bash
curl http://localhost:8787/v1/chat/completions \
  -H "Authorization: Bearer gumi-local" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "lmstudio:qwen2.5-coder-7b-instruct",
    "messages": [
      {
        "role": "user",
        "content": "Refactor this function to use early returns."
      }
    ],
    "gumi": {
      "mode": "lightweight"
    }
  }'
```

## 30.2 OpenAI-Compatible Python Client

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8787/v1",
    api_key="gumi-local",
)

response = client.chat.completions.create(
    model="lmstudio:qwen2.5-coder-7b-instruct",
    messages=[
        {"role": "user", "content": "Refactor this function to use early returns."}
    ],
    extra_body={"gumi": {"mode": "lightweight"}},
)

print(response.choices[0].message.content)
```

## 30.3 OpenAI-Compatible JavaScript Client

```js
import OpenAI from "openai";

const client = new OpenAI({
  baseURL: "http://localhost:8787/v1",
  apiKey: "gumi-local",
});

const response = await client.chat.completions.create({
  model: "lmstudio:qwen2.5-coder-7b-instruct",
  messages: [
    { role: "user", content: "Refactor this function to use early returns." }
  ],
  // Use the library's extension mechanism for extra body fields if available.
});

console.log(response.choices[0].message.content);
```

In lightweight mode Gumi resolves the `qwen2.5-coder-7b` profile, applies its defaults, preserves the app-provided messages, and forwards the request to the LM Studio adapter.

---

# 31. General Example Usage

## 31.1 Python OpenAI Client

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8787/v1",
    api_key="gumi-local",
)

response = client.chat.completions.create(
    model="local:auto",
    messages=[
        {"role": "user", "content": "Explain local AI runtime."}
    ],
)

print(response.choices[0].message.content)
```

## 31.2 JavaScript OpenAI Client

```js
import OpenAI from "openai";

const client = new OpenAI({
  baseURL: "http://localhost:8787/v1",
  apiKey: "gumi-local",
});

const response = await client.chat.completions.create({
  model: "local:auto",
  messages: [
    {
      role: "user",
      content: "Explain local AI runtime."
    }
  ]
});

console.log(response.choices[0].message.content);
```

## 31.3 cURL

```bash
curl http://localhost:8787/v1/chat/completions \
  -H "Authorization: Bearer gumi-local" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "local:auto",
    "messages": [
      {
        "role": "user",
        "content": "Explain Docker."
      }
    ]
  }'
```

---

# 32. Final API Statement

Gumi V1 API is an OpenAI-compatible local runtime API.

Its purpose is to allow existing AI applications to use local models through Gumi while gaining stability features such as context management, prompt optimization, validation, repair, telemetry, and provider abstraction.

The API should feel familiar to developers while quietly exposing Gumi's runtime intelligence through optional extensions.