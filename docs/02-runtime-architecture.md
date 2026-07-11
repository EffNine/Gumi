# Novexa Runtime Architecture

Version: 1.0  
Status: Draft  
Scope: Local-first runtime architecture

---

# 1. Purpose

This document defines the high-level runtime architecture of Novexa.

Novexa is an intelligence runtime that sits between AI applications and local inference engines.

Its purpose is to make local AI models:

- more stable
- more reliable
- easier to integrate
- easier to observe
- safer to use in real applications
- more consistent across providers

Novexa does not replace local inference engines.

Novexa improves the layer around them.

---

# 2. Architecture Summary

Novexa is designed as a modular monolith with a plugin-based internal architecture.

The runtime receives OpenAI-compatible API requests, processes them through a deterministic intelligence pipeline, sends them to a local model provider, validates the response, and returns a clean response to the client application.

```text
Application
    ↓
Novexa Runtime
    ↓
Local Inference Engine
    ↓
Local Model
```

Supported local inference engines in V1:

- Ollama
- LM Studio
- OpenAI-compatible local servers

Future providers:

- llama.cpp server
- vLLM
- SGLang
- Text Generation Inference
- optional cloud providers

---

# 3. Core Architectural Decision

Novexa V1 must be a **modular monolith**, not microservices.

## Why

A modular monolith gives Novexa:

- simpler development
- easier debugging
- lower runtime overhead
- easier local installation
- simpler distribution
- better developer experience
- fewer moving parts

Microservices are unnecessary for V1.

Novexa should only consider service separation after there is real user scale, heavy enterprise demand, or distributed runtime requirements.

---

# 4. Runtime Layer Position

Novexa sits between applications and inference providers.

```text
Application Layer
────────────────────────────────────
Claude Code
Cline
Continue
Open WebUI
AnythingLLM
Custom Apps
SDK Users

Runtime Layer
────────────────────────────────────
Novexa Runtime

Provider Layer
────────────────────────────────────
Ollama
LM Studio
llama.cpp
vLLM
SGLang

Model Layer
────────────────────────────────────
Qwen
DeepSeek
Llama
Gemma
Mistral
GLM
Phi
```

Novexa does not compete with the provider layer.

Novexa improves how applications use the provider layer.

---

# 5. High-Level Runtime Flow

```text
Client Request
    ↓
Gateway Engine
    ↓
Authentication / Workspace Resolution
    ↓
Pipeline Engine
    ↓
Session Engine
    ↓
Context Engine
    ↓
Memory Engine
    ↓
Prompt Engine
    ↓
Guard Engine
    ↓
Provider Engine
    ↓
Local Provider
    ↓
Local Model
    ↓
Response Engine
    ↓
Validation Engine
    ↓
Repair Engine
    ↓
Telemetry Engine
    ↓
Client Response
```

The request must not be sent directly from Gateway Engine to Provider Engine.

Every request must pass through the Pipeline Engine.

---

# 6. Runtime Engines

Novexa consists of multiple internal engines.

Each engine has a clear responsibility.

```text
Novexa Runtime
├── Gateway Engine
├── Pipeline Engine
├── Workspace Engine
├── Session Engine
├── Context Engine
├── Memory Engine
├── Prompt Engine
├── Guard Engine
├── Provider Engine
├── Response Engine
├── Validation Engine
├── Repair Engine
├── Telemetry Engine
├── Config Engine
└── Plugin Engine
```

---

# 7. Engine Responsibilities

## 7.1 Gateway Engine

Responsible for receiving external API requests.

Responsibilities:

- expose OpenAI-compatible endpoints
- parse incoming requests
- validate request shape
- handle streaming and non-streaming responses
- normalize API errors
- forward valid requests to Pipeline Engine

The Gateway Engine should not contain intelligence logic.

It is an interface layer.

---

## 7.2 Pipeline Engine

Responsible for orchestrating the request lifecycle.

Responsibilities:

- execute engines in correct order
- trigger plugin hooks
- handle retries
- handle fallback decisions
- enforce timeout rules
- maintain pipeline state
- produce final response metadata

The Pipeline Engine is the heart of Novexa.

All request processing must flow through it.

---

## 7.3 Workspace Engine

Responsible for project-level isolation.

Responsibilities:

- resolve workspace from API key or local config
- load workspace-specific settings
- isolate sessions, memory, logs, and provider settings
- support future multi-project workflows

In local-only V1, a default workspace is enough.

---

## 7.4 Session Engine

Responsible for conversation/session management.

Responsibilities:

- create session IDs
- track conversation state
- manage message history
- store recent interactions
- pass session context to Context Engine and Memory Engine

Session Engine handles short-term conversation continuity.

Memory Engine handles long-term knowledge.

---

## 7.5 Context Engine

Responsible for preparing the context window.

Responsibilities:

- count estimated tokens
- remove duplicate content
- trim irrelevant messages
- compress long history
- preserve important facts
- enforce model context limits
- build final context package

Context Engine must prevent context overflow and context pollution.

---

## 7.6 Memory Engine

Responsible for long-term memory and retrieval.

Responsibilities:

- store durable facts
- retrieve relevant memories
- summarize long conversations
- maintain project knowledge
- support future RAG workflows

Memory Engine is optional in V1.

It should be designed now but implemented progressively.

---

## 7.7 Prompt Engine

Responsible for transforming raw user intent into model-ready prompts.

Responsibilities:

- build system prompts
- apply model profile instructions
- apply workspace rules
- apply response format instructions
- improve vague prompts when safe
- preserve user intent
- avoid oversteering the model

Prompt Engine must not change the user's meaning.

It improves clarity, not intent.

---

## 7.8 Guard Engine

Responsible for runtime safety and behaviour control.

Responsibilities:

- apply local guardrails
- detect risky prompt patterns
- enforce output constraints
- detect likely hallucination conditions
- detect loop risk before generation
- block unsafe provider calls if required

Guard Engine should be configurable.

---

## 7.9 Provider Engine

Responsible for communication with inference providers.

Responsibilities:

- select provider
- select model
- normalize provider request format
- call local inference engine
- support streaming
- normalize provider errors
- return raw model output to Response Engine

Provider Engine is an adapter layer.

It should not contain business logic or prompt logic.

---

## 7.10 Response Engine

Responsible for processing raw model output.

Responsibilities:

- normalize output text
- extract assistant message
- detect incomplete generations
- detect repeated text
- detect formatting issues
- prepare response for validation

Response Engine converts provider-specific output into Novexa output.

---

## 7.11 Validation Engine

Responsible for checking response correctness.

Responsibilities:

- validate JSON schema
- validate Markdown structure
- validate YAML/XML where applicable
- verify required fields
- detect repeated paragraphs
- detect malformed structured output

Validation Engine should return a validation report, not directly mutate output.

---

## 7.12 Repair Engine

Responsible for fixing invalid responses.

Responsibilities:

- repair invalid JSON
- remove repeated output
- retry with stricter prompt if needed
- regenerate only broken sections where possible
- preserve valid content
- avoid unnecessary full regeneration

Repair Engine should run only when validation fails or loop detection triggers.

---

## 7.13 Telemetry Engine

Responsible for observability.

Responsibilities:

- log request metadata
- track latency
- track provider errors
- track token estimates
- track retries
- track repairs
- track context compression
- expose dashboard metrics

Telemetry must be privacy-first.

Local telemetry is enabled.

External telemetry must be opt-in.

---

## 7.14 Config Engine

Responsible for configuration loading and resolution.

Responsibilities:

- load global config
- load workspace config
- load model profiles
- load provider settings
- load plugin settings
- merge config precedence
- validate config

Config precedence:

```text
Runtime Defaults
    ↓
Global Config
    ↓
Workspace Config
    ↓
Environment Variables
    ↓
Request-Level Overrides
```

Request-level overrides have the highest priority.

---

## 7.15 Plugin Engine

Responsible for plugin loading and lifecycle.

Responsibilities:

- discover plugins
- validate plugin manifests
- load plugins safely
- expose runtime hooks
- isolate plugin failures
- enforce plugin permissions

Plugin Engine enables Novexa to grow without bloating the core runtime.

---

# 8. Intelligence Pipeline

The Intelligence Pipeline is the main request-processing path.

```text
Incoming Request
    ↓
Normalize
    ↓
Authenticate
    ↓
Resolve Workspace
    ↓
Load Config
    ↓
Create Pipeline Context
    ↓
Run Pre-Request Hooks
    ↓
Resolve Session
    ↓
Prepare Context
    ↓
Retrieve Memory
    ↓
Build Prompt
    ↓
Apply Guardrails
    ↓
Select Provider
    ↓
Generate
    ↓
Normalize Response
    ↓
Validate Response
    ↓
Repair If Needed
    ↓
Run Post-Response Hooks
    ↓
Record Telemetry
    ↓
Return Response
```

The pipeline should be deterministic.

A request should produce explainable pipeline events.

---

# 9. Pipeline Context

Every request creates a Pipeline Context object.

The Pipeline Context travels through all engines.

It contains:

- request ID
- workspace ID
- session ID
- selected model
- selected provider
- incoming messages
- normalized messages
- retrieved memories
- compressed context
- prompt package
- provider request
- raw provider response
- validation report
- repair report
- telemetry data
- plugin events
- errors
- final response

The Pipeline Context is the single source of truth during request processing.

---

# 10. Request Modes

Novexa should support different request modes.

## 10.1 Direct Mode

Minimal processing.

```text
Request → Provider → Response
```

Used for maximum speed.

---

## 10.2 Lightweight Mode

Low-overhead mode for apps that already manage their own workflow prompts, such as OpenCode, Continue, Cline, Open WebUI, or custom agents.

```text
Request
    ↓
Model Profile Defaults
    ↓
Thinking Policy
    ↓
Minimal Prompt Policy
    ↓
Provider
    ↓
Telemetry
    ↓
Response
```

Used when the app should keep its own behavior while Novexa centralizes model tuning.

Lightweight Mode sits between Direct Mode and Stabilized Mode:

- Direct Mode forwards requests almost unchanged.
- Lightweight Mode adds model profile defaults, thinking policy, minimal prompt policy, and telemetry.
- Stabilized Mode adds context compression, memory, full prompt wrapping, guardrails, validation, and repair.

Lightweight Mode should apply:

- provider and model profile defaults
- thinking on/off policy
- max token and sampling defaults
- minimal exact-format/plain-text guard
- local telemetry

Lightweight Mode should avoid:

- heavy context wrappers
- long system prompts
- unnecessary JSON instructions
- expensive validation/repair unless explicitly requested

---

## 10.3 Stabilized Mode

Default mode.

```text
Request
    ↓
Context
    ↓
Prompt
    ↓
Provider
    ↓
Validation
    ↓
Repair
    ↓
Response
```

Used for normal applications.

---

## 10.4 Structured Mode

Used when a schema is required.

Additional features:

- strict JSON schema validation
- automatic repair
- retry on malformed output
- response field enforcement

---

## 10.5 Agent Mode

Future mode.

Used for coding agents and tool-driven workflows.

Additional features:

- step budget
- tool call monitoring
- loop detection
- rollback hints
- progress scoring
- action validation

---

# 11. Provider Abstraction

Provider Engine must expose a stable provider interface.

Every provider adapter must implement the same contract.

Conceptual interface:

```text
Provider
├── Name()
├── HealthCheck()
├── ListModels()
├── Generate()
├── Stream()
└── EstimateCapabilities()
```

Provider-specific complexity must stay inside provider adapters.

The rest of Novexa should not care whether the backend is Ollama, LM Studio, vLLM, or another engine.

---

# 12. Model Profiles

Model Profiles define recommended behaviour per model.

A profile may include:

- context limit
- default temperature
- default top_p
- repeat penalty
- stop tokens
- prompt style
- reasoning behaviour
- structured output capability
- tool calling capability
- recommended context strategy
- known weaknesses

Example:

```yaml
model: qwen3-8b
provider: ollama

capabilities:
  chat: true
  structured_output: medium
  tool_calling: weak
  long_context: medium

defaults:
  temperature: 0.4
  top_p: 0.9
  repeat_penalty: 1.12

context:
  strategy: compress
  max_input_tokens: 24000

guard:
  anti_loop: aggressive
  json_repair: true

notes:
  - Better for coding and technical Q&A than creative writing.
  - Requires strong instruction formatting for JSON output.
```

Model Profiles are a key Novexa feature.

They reduce manual tuning.

---

# 13. Plugin Architecture Position

Plugins are not allowed to bypass the Pipeline Engine.

Plugins must attach to defined hooks.

Example hooks:

```text
before_request
after_request_normalized
before_context
after_context
before_prompt
after_prompt
before_provider
after_provider
before_validation
after_validation
before_response
after_response
on_error
```

Plugins should be powerful but controlled.

---

# 14. Error Handling Philosophy

Novexa must fail gracefully.

Errors should be classified.

Error categories:

- request_error
- config_error
- provider_error
- context_error
- validation_error
- repair_error
- plugin_error
- runtime_error

Every error should include:

- error code
- human-readable message
- engine origin
- retryable flag
- suggested fix where possible

Example:

```json
{
  "error": {
    "code": "PROVIDER_UNAVAILABLE",
    "message": "Ollama is not reachable at http://localhost:11434.",
    "engine": "provider",
    "retryable": true,
    "suggestion": "Start Ollama or update the provider URL in novexa.yaml."
  }
}
```

---

# 15. Telemetry Philosophy

Telemetry must help users understand what Novexa did.

Telemetry should answer:

- Which provider was used?
- Which model was used?
- How long did it take?
- Was context compressed?
- Was output repaired?
- Did retry happen?
- Did loop detection trigger?
- Did validation pass?

Telemetry must not expose private prompt content unless the user explicitly enables detailed local logs.

External telemetry must be opt-in.

---

# 16. Configuration Philosophy

Novexa should be usable with zero config.

But advanced users should be able to control everything.

Default startup should work like:

```bash
novexa start
```

Advanced config should live in:

```text
novexa.yaml
```

Example:

```yaml
runtime:
  mode: stabilized
  port: 8787

provider:
  default: ollama

providers:
  ollama:
    url: http://localhost:11434
    default_model: qwen3:8b

engines:
  context:
    enabled: true
    strategy: compress

  prompt:
    enabled: true
    profile_mode: auto

  validation:
    enabled: true
    repair: true

  guard:
    anti_loop: true

telemetry:
  local: true
  external: false
```

---

# 17. Security Boundaries

Novexa must define clear security boundaries.

## Runtime Boundary

The runtime controls request processing.

## Provider Boundary

Providers are external systems.

Provider output is untrusted until validated.

## Plugin Boundary

Plugins are untrusted unless explicitly trusted.

## Workspace Boundary

Workspace data must be isolated.

## Telemetry Boundary

Telemetry must not leak user content by default.

---

# 18. V1 Scope

V1 must include:

- OpenAI-compatible chat completions endpoint
- local provider support
- Ollama adapter
- LM Studio adapter
- basic Pipeline Engine
- basic Context Engine
- basic Prompt Engine
- basic Validation Engine
- basic Repair Engine
- basic Telemetry Engine
- model profiles
- local dashboard
- CLI start/status/doctor commands

---

# 19. Out of Scope for V1

V1 must not include:

- cloud billing
- hosted inference
- team accounts
- enterprise RBAC
- marketplace
- paid plugin system
- hosted memory
- distributed runtime
- multi-node orchestration
- commercial cloud fallback

These are future features.

---

# 20. Architecture Principles

Novexa architecture must follow these rules:

1. Every request goes through Pipeline Engine.
2. Providers are adapters, not business logic.
3. Engines must be replaceable.
4. Plugins attach through hooks only.
5. Runtime must work offline.
6. Telemetry must be local by default.
7. Configuration must be explicit and inspectable.
8. Validation must not be hidden.
9. Repair must be explainable.
10. Model profiles must be versioned.

---

# 21. Target Developer Experience

A developer should be able to run:

```bash
novexa start
```

Then point an app to:

```text
http://localhost:8787/v1
```

And use an OpenAI-compatible client.

Example:

```bash
export OPENAI_BASE_URL=http://localhost:8787/v1
export OPENAI_API_KEY=novexa-local
```

No app rewrite should be required for basic use.

---

# 22. Architectural North Star

Novexa should make local AI feel less fragile.

The runtime must behave like an invisible stabilizer:

- quietly improving prompts
- managing context
- detecting failures
- repairing bad output
- exposing useful telemetry
- preserving developer control

The best version of Novexa feels simple from the outside and disciplined inside.

---

# 23. Final Architecture Statement

Novexa is a local-first AI runtime built as a modular monolith.

It exposes an OpenAI-compatible gateway, processes requests through an intelligence pipeline, connects to local inference providers through adapters, validates and repairs model output, and exposes transparent telemetry through a local dashboard.

Its architecture is designed to make local AI applications more stable, reliable, observable, and production-ready without depending on cloud providers.
