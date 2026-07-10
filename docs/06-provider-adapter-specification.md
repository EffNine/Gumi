# Novexa Provider Adapter Specification

Version: 1.0  
Status: Draft  
Scope: Provider adapter contract for Novexa Runtime V1

---

# 1. Purpose

This document defines how Novexa connects to local inference providers.

Provider adapters allow Novexa to communicate with different local AI engines through one stable internal interface.

V1 providers:

- Ollama
- LM Studio
- OpenAI-compatible local servers

Future providers:

- llama.cpp server
- vLLM
- SGLang
- Text Generation Inference
- KoboldCpp
- LocalAI
- optional cloud providers

---

# 2. Core Principle

Providers are adapters.

Providers are not business logic.

The rest of Novexa must not care whether the backend is Ollama, LM Studio, vLLM, or another inference engine.

---

# 3. Provider Layer Position

```text
Application
    ↓
Novexa Gateway
    ↓
Pipeline Engine
    ↓
Provider Engine
    ↓
Provider Adapter
    ↓
Local Inference Provider
    ↓
Local Model
```

Provider adapters sit at the edge of the runtime.

They translate Novexa requests into provider-specific requests and normalize provider responses back into Novexa format.

---

# 4. Provider Engine vs Provider Adapter

## Provider Engine

The Provider Engine decides:

- which provider to use
- which model to use
- whether the provider is healthy
- how to map request metadata
- how to normalize the result

## Provider Adapter

The Provider Adapter knows:

- provider API routes
- provider request format
- provider response format
- provider streaming format
- provider error format

The adapter must not contain Novexa intelligence logic.

---

# 5. Provider Adapter Contract

Every provider adapter must implement this conceptual interface:

```text
ProviderAdapter
├── name()
├── type()
├── health_check()
├── list_models()
├── get_model(model_id)
├── generate(request)
├── stream(request)
├── capabilities()
├── normalize_error(error)
└── shutdown()
```

---

# 6. Provider Adapter Metadata

Each adapter should expose metadata:

```text
ProviderMetadata
├── name
├── display_name
├── type
├── version
├── base_url
├── enabled
├── status
├── supports_streaming
├── supports_chat
├── supports_embeddings
├── supports_tools
├── supports_vision
└── notes
```

Example:

```json
{
  "name": "ollama",
  "display_name": "Ollama",
  "type": "local",
  "version": "0.1.0",
  "base_url": "http://localhost:11434",
  "enabled": true,
  "status": "ok",
  "supports_streaming": true,
  "supports_chat": true,
  "supports_embeddings": false,
  "supports_tools": false,
  "supports_vision": false,
  "notes": []
}
```

---

# 7. Provider Status

Allowed provider status values:

```text
ok
offline
degraded
misconfigured
unknown
```

## ok

Provider is reachable and responding correctly.

## offline

Provider cannot be reached.

## degraded

Provider is reachable but one or more features are unavailable.

## misconfigured

Provider config is invalid.

## unknown

Status has not been checked yet.

---

# 8. Provider Health Check

Each adapter must implement a health check.

## 8.1 Health Check Output

```text
ProviderHealth
├── status
├── latency_ms
├── message
├── error
├── checked_at
└── metadata
```

Example:

```json
{
  "status": "ok",
  "latency_ms": 18,
  "message": "Ollama is reachable.",
  "error": null,
  "checked_at": "2026-07-10T00:00:00Z",
  "metadata": {
    "models_available": 4
  }
}
```

---

# 9. Model Discovery

Each provider adapter must support model discovery where possible.

## 9.1 Internal Model Format

```text
ProviderModel
├── id
├── provider
├── provider_model_id
├── display_name
├── family
├── size
├── quantization
├── context_length
├── capabilities
├── installed
├── metadata
└── discovered_at
```

Example:

```json
{
  "id": "ollama:qwen3:8b",
  "provider": "ollama",
  "provider_model_id": "qwen3:8b",
  "display_name": "Qwen3 8B",
  "family": "qwen",
  "size": "8b",
  "quantization": null,
  "context_length": null,
  "capabilities": {
    "chat": true,
    "streaming": true,
    "structured_output": "medium",
    "tool_calling": "unknown",
    "vision": false
  },
  "installed": true,
  "metadata": {},
  "discovered_at": "2026-07-10T00:00:00Z"
}
```

---

# 10. Model ID Naming

Novexa model IDs should follow:

```text
provider:model
```

Examples:

```text
ollama:qwen3:8b
ollama:deepseek-r1:8b
lmstudio:local-model
openai-compatible:local-model
```

Reserved IDs:

```text
local:auto
provider:auto
```

## 10.1 Rule

Provider adapters should use provider-native model names internally, but expose Novexa model IDs externally.

---

# 11. Provider Request Format

Provider Engine sends a normalized request to the adapter.

```text
ProviderRequest
├── request_id
├── model
├── messages
├── system_prompt
├── temperature
├── top_p
├── max_tokens
├── stop
├── stream
├── response_format
├── tools
├── metadata
└── timeout_seconds
```

Example:

```json
{
  "request_id": "req_abc123",
  "model": "qwen3:8b",
  "messages": [
    {
      "role": "system",
      "content": "You are a helpful local AI assistant."
    },
    {
      "role": "user",
      "content": "Explain Docker."
    }
  ],
  "temperature": 0.4,
  "top_p": 0.9,
  "max_tokens": 1024,
  "stop": [],
  "stream": false,
  "response_format": null,
  "tools": [],
  "metadata": {},
  "timeout_seconds": 90
}
```

---

# 12. Provider Response Format

Adapters must return normalized responses.

```text
ProviderResponse
├── request_id
├── provider
├── model
├── content
├── role
├── finish_reason
├── usage
├── raw
├── latency_ms
├── error
└── metadata
```

Example:

```json
{
  "request_id": "req_abc123",
  "provider": "ollama",
  "model": "qwen3:8b",
  "content": "Docker is a tool that packages applications...",
  "role": "assistant",
  "finish_reason": "stop",
  "usage": {
    "prompt_tokens": 120,
    "completion_tokens": 80,
    "total_tokens": 200
  },
  "raw": {},
  "latency_ms": 842,
  "error": null,
  "metadata": {}
}
```

---

# 13. Streaming Response Format

Adapters must stream normalized chunks.

```text
ProviderStreamChunk
├── request_id
├── provider
├── model
├── delta
├── role
├── finish_reason
├── raw
├── error
└── metadata
```

Example:

```json
{
  "request_id": "req_abc123",
  "provider": "ollama",
  "model": "qwen3:8b",
  "delta": "Docker",
  "role": "assistant",
  "finish_reason": null,
  "raw": {},
  "error": null,
  "metadata": {}
}
```

Final chunk:

```json
{
  "request_id": "req_abc123",
  "provider": "ollama",
  "model": "qwen3:8b",
  "delta": "",
  "role": "assistant",
  "finish_reason": "stop",
  "raw": {},
  "error": null,
  "metadata": {}
}
```

---

# 14. Capability Model

Provider capabilities should be normalized.

```text
ProviderCapabilities
├── chat
├── streaming
├── embeddings
├── tools
├── vision
├── structured_output
├── json_mode
├── model_discovery
└── health_check
```

Capability values:

```text
true
false
unknown
```

For quality-based capabilities:

```text
none
weak
medium
strong
unknown
```

Example:

```json
{
  "chat": true,
  "streaming": true,
  "embeddings": false,
  "tools": false,
  "vision": false,
  "structured_output": "medium",
  "json_mode": "weak",
  "model_discovery": true,
  "health_check": true
}
```

---

# 15. Provider Selection

Provider Engine selects provider using this priority:

```text
1. Explicit request model
2. Request-level Novexa override
3. Workspace config
4. Model profile
5. Global default provider
6. Auto selection
```

## 15.1 Explicit Model

If request uses:

```text
ollama:qwen3:8b
```

Then Provider Engine selects:

```text
provider = ollama
model = qwen3:8b
```

## 15.2 Local Auto

If request uses:

```text
local:auto
```

Provider Engine selects the best available local model based on config and model profiles.

## 15.3 Provider Auto

If request uses:

```text
ollama:auto
```

Provider Engine selects the default or best model within Ollama.

---

# 16. Auto Selection Rules

V1 simple auto-selection:

```text
1. Use configured default provider.
2. Use configured default model.
3. If default model is missing, use first available model.
4. If provider is offline, return provider error.
```

Future auto-selection can consider:

- task type
- latency
- context size
- model profile
- previous failure
- structured output requirement
- tool support
- available VRAM
- user preference

---

# 17. Error Normalization

Provider adapters must normalize provider-specific errors into Novexa errors.

## 17.1 Provider Error Format

```text
ProviderError
├── code
├── message
├── provider
├── retryable
├── status_code
├── raw_error
└── suggestion
```

---

# 18. Standard Provider Error Codes

```text
PROVIDER_UNAVAILABLE
PROVIDER_TIMEOUT
PROVIDER_BAD_RESPONSE
PROVIDER_UNAUTHORIZED
PROVIDER_RATE_LIMITED
MODEL_NOT_FOUND
MODEL_UNSUPPORTED
STREAMING_UNSUPPORTED
INVALID_PROVIDER_CONFIG
PROVIDER_INTERNAL_ERROR
```

---

# 19. Error Mapping Examples

## 19.1 Connection Refused

Provider-native error:

```text
ECONNREFUSED
```

Novexa error:

```json
{
  "code": "PROVIDER_UNAVAILABLE",
  "message": "Ollama is not reachable.",
  "provider": "ollama",
  "retryable": true,
  "suggestion": "Start Ollama or update providers.ollama.url in novexa.yaml."
}
```

## 19.2 Model Missing

Provider-native error:

```text
model not found
```

Novexa error:

```json
{
  "code": "MODEL_NOT_FOUND",
  "message": "Model qwen3:8b was not found in Ollama.",
  "provider": "ollama",
  "retryable": false,
  "suggestion": "Run: ollama pull qwen3:8b"
}
```

## 19.3 Timeout

Novexa error:

```json
{
  "code": "PROVIDER_TIMEOUT",
  "message": "The provider did not respond within the configured timeout.",
  "provider": "ollama",
  "retryable": true,
  "suggestion": "Use a smaller model, reduce context, or increase provider timeout."
}
```

---

# 20. Ollama Adapter

## 20.1 Provider Name

```text
ollama
```

## 20.2 Default URL

```text
http://localhost:11434
```

## 20.3 Health Check

Recommended endpoint:

```http
GET /api/tags
```

If reachable, provider status is `ok`.

---

## 20.4 List Models

Endpoint:

```http
GET /api/tags
```

Adapter should convert Ollama model list into `ProviderModel`.

---

## 20.5 Generate Chat

Recommended endpoint:

```http
POST /api/chat
```

Request mapping:

| Novexa Field | Ollama Field |
|---|---|
| `model` | `model` |
| `messages` | `messages` |
| `stream` | `stream` |
| `temperature` | `options.temperature` |
| `top_p` | `options.top_p` |
| `stop` | `options.stop` |
| `max_tokens` | `options.num_predict` |
| `novexa.thinking.enabled` | `think` |

The `think` field is a top-level boolean in the Ollama request body. It is only sent when explicitly set. When absent, Ollama uses its default thinking behaviour.

---

## 20.6 Ollama Non-Streaming Response Mapping

Ollama response field:

```text
message.content
```

maps to:

```text
ProviderResponse.content
```

Ollama also supports:

```text
message.thinking
```

This field contains model reasoning text. Novexa must never append thinking/reasoning text into the assistant final content. If a model finishes with empty `message.content` but non-empty `message.thinking`, Novexa returns a clear normalized error explaining that the model exhausted output in reasoning, with a suggestion to increase `max_tokens` or disable thinking.

Ollama done reason maps to:

```text
finish_reason
```

---

## 20.7 Ollama Streaming Response Mapping

Ollama streaming chunks should be converted into `ProviderStreamChunk`.

Each chunk content:

```text
message.content
```

maps to:

```text
delta
```

When `done = true`, emit final chunk with finish reason.

---

## 20.8 Ollama Notes

Ollama may return token timing and evaluation metadata.

Novexa should preserve this inside:

```text
ProviderResponse.metadata
```

---

# 21. LM Studio Adapter

## 21.1 Provider Name

```text
lmstudio
```

## 21.2 Default URL

```text
http://localhost:1234/v1
```

## 21.3 Health Check

Recommended endpoint:

```http
GET /v1/models
```

If reachable, provider status is `ok`.

---

## 21.4 List Models

Endpoint:

```http
GET /v1/models
```

LM Studio already uses OpenAI-compatible model format.

Adapter should normalize model IDs into:

```text
lmstudio:<model_id>
```

---

## 21.5 Generate Chat

Endpoint:

```http
POST /v1/chat/completions
```

LM Studio is mostly OpenAI-compatible.

Adapter should pass through:

- model
- messages
- temperature
- top_p
- max_tokens
- stream
- stop
- response_format

---

## 21.6 Response Mapping

LM Studio response:

```text
choices[0].message.content
```

maps to:

```text
ProviderResponse.content
```

Streaming response:

```text
choices[0].delta.content
```

maps to:

```text
ProviderStreamChunk.delta
```

---

# 22. OpenAI-Compatible Local Adapter

## 22.1 Provider Name

```text
openai_compatible_local
```

## 22.2 Default URL

```text
http://localhost:8000/v1
```

## 22.3 Purpose

This adapter supports any local server that implements OpenAI-compatible endpoints.

Examples:

- vLLM OpenAI server
- SGLang OpenAI server
- LocalAI
- llama.cpp OpenAI-compatible server
- custom local servers

---

## 22.4 Required Endpoints

```http
GET  /v1/models
POST /v1/chat/completions
```

---

## 22.5 Response Mapping

Same as OpenAI chat completions format.

---

# 23. Streaming Rules

Streaming should preserve provider streaming where possible.

If provider supports streaming:

```text
Provider streaming → Novexa normalized stream → OpenAI-compatible SSE
```

If provider does not support streaming:

V1 may either:

- return `STREAMING_UNSUPPORTED`
- or simulate streaming by chunking final response

Default should be:

```text
return STREAMING_UNSUPPORTED
```

Simulated streaming can be added later.

---

# 24. Timeout Rules

Provider adapters must respect configured provider timeout.

Default:

```yaml
providers:
  ollama:
    timeout_seconds: 90
```

Timeouts must be reported as:

```text
PROVIDER_TIMEOUT
```

---

# 25. Retry Responsibility

Provider adapters do not decide retry strategy.

They return normalized errors.

Pipeline Engine decides whether to retry.

---

# 26. Provider Logging

Adapters should log:

- provider name
- request ID
- model
- latency
- error code if any

Adapters must not log full prompt or full response unless detailed local logging is enabled.

---

# 27. Provider Telemetry Events

Provider Engine should emit events:

```text
provider_selected
provider_health_checked
provider_request_started
provider_request_completed
provider_request_failed
provider_model_missing
provider_stream_started
provider_stream_completed
```

Example event:

```yaml
event: provider_request_completed
metadata:
  provider: ollama
  model: qwen3:8b
  latency_ms: 842
```

---

# 28. Provider Security Rules

Provider adapters must:

1. Redact API keys.
2. Validate provider URLs.
3. Avoid sending data to non-local providers unless explicitly configured.
4. Respect local-first mode.
5. Treat provider output as untrusted.
6. Never execute provider output.

---

# 29. Local-First Provider Rule

In V1, providers should default to local URLs only.

Allowed by default:

```text
localhost
127.0.0.1
::1
```

LAN/private IPs may be allowed by explicit config.

Public external URLs should require explicit confirmation or config.

---

# 30. Provider Doctor Checks

`novexa doctor` should check each provider:

- provider reachable
- default model available
- response latency
- streaming support
- model list available
- URL appears safe/local
- provider timeout valid

Example:

```json
{
  "name": "ollama_provider",
  "status": "ok",
  "message": "Ollama is reachable.",
  "metadata": {
    "url": "http://localhost:11434",
    "models": 4,
    "latency_ms": 18
  }
}
```

---

# 31. Testing Requirements

Each provider adapter must have:

- unit tests for request mapping
- unit tests for response mapping
- unit tests for error mapping
- health check tests
- model discovery tests
- streaming tests
- timeout tests

Integration tests should be optional because they require local providers installed.

---

# 32. V1 Implementation Priority

Implement in this order:

```text
1. OpenAI-compatible local adapter
2. Ollama adapter
3. LM Studio adapter
```

Reason:

The OpenAI-compatible adapter can also help support LM Studio-like servers and provides a useful baseline.

---

# 33. Final Provider Statement

Novexa provider adapters are thin translation layers.

They connect the Novexa intelligence pipeline to local inference engines without leaking provider-specific complexity into the rest of the runtime.

A good provider adapter should be boring, predictable, testable, and replaceable.