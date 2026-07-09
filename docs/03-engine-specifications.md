# Novexa Engine Specifications

Version: 1.0  
Status: Draft  
Scope: Internal engine contracts for Novexa Runtime V1

---

# 1. Purpose

This document defines the responsibility, input, output, lifecycle, and boundaries of every core engine inside Novexa Runtime.

Each engine must have:

- a clear purpose
- explicit input
- explicit output
- no hidden side effects
- no direct dependency on unrelated engines
- clear failure behavior
- testable behaviour

The goal is to make Novexa easy to implement, test, extend, and maintain.

---

# 2. Engine Design Philosophy

Novexa engines must follow these rules:

1. Engines process data.
2. Engines do not own the entire request lifecycle.
3. Pipeline Engine orchestrates execution.
4. Engines communicate through Pipeline Context.
5. Engines should be replaceable.
6. Engines should be testable in isolation.
7. Engines must report what they changed.
8. Engines must fail gracefully.

---

# 3. Shared Engine Contract

Every engine should follow this conceptual contract:

```text
Engine
├── Name()
├── Init(config)
├── HealthCheck()
├── Process(pipeline_context)
├── Shutdown()
└── Metrics()
```

Not every engine needs complex initialization.

But every engine must expose enough information for:

- diagnostics
- telemetry
- debugging
- dashboard visibility
- CLI doctor checks

---

# 4. Pipeline Context

All engines receive and update a shared Pipeline Context.

The Pipeline Context is the single source of truth for the request lifecycle.

## 4.1 Pipeline Context Fields

```text
PipelineContext
├── request_id
├── workspace_id
├── session_id
├── request_mode
├── incoming_request
├── normalized_request
├── config_snapshot
├── model_profile
├── messages_original
├── messages_normalized
├── messages_compressed
├── memory_results
├── prompt_package
├── provider_request
├── provider_response_raw
├── response_normalized
├── validation_report
├── repair_report
├── telemetry_events
├── plugin_events
├── warnings
├── errors
└── final_response
```

## 4.2 Rule

Engines should not pass custom data directly to each other.

They must write to and read from Pipeline Context.

---

# 5. Engine Status Types

Each engine returns an Engine Result.

```text
EngineResult
├── status
├── changed
├── warnings
├── errors
├── events
└── metadata
```

Allowed status values:

```text
success
skipped
warning
retry_requested
failed_recoverable
failed_fatal
```

---

# 6. Gateway Engine

## 6.1 Purpose

Gateway Engine receives external API requests and exposes Novexa-compatible endpoints.

Its main role is protocol compatibility.

It must not contain intelligence logic.

---

## 6.2 Responsibilities

Gateway Engine is responsible for:

- exposing HTTP API
- supporting OpenAI-compatible routes
- parsing requests
- validating request shape
- handling streaming transport
- handling non-streaming transport
- normalizing API errors
- forwarding valid requests to Pipeline Engine

---

## 6.3 V1 Endpoints

```http
GET  /health
GET  /v1/models
POST /v1/chat/completions
```

Optional V1 endpoints:

```http
GET  /v1/novexa/status
GET  /v1/novexa/providers
GET  /v1/novexa/config
```

---

## 6.4 Input

Gateway Engine input:

```text
HTTP Request
```

Example OpenAI-compatible request:

```json
{
  "model": "local:auto",
  "messages": [
    {
      "role": "user",
      "content": "Explain Docker."
    }
  ],
  "temperature": 0.4,
  "stream": false
}
```

---

## 6.5 Output

Gateway Engine output:

```text
NormalizedRequest
```

The normalized request is passed to Pipeline Engine.

---

## 6.6 Boundaries

Gateway Engine must not:

- select models
- optimize prompts
- compress context
- validate model output
- repair responses
- call providers directly

---

## 6.7 Failure Behaviour

Invalid request shape should return:

```json
{
  "error": {
    "code": "INVALID_REQUEST",
    "message": "The request body is not valid.",
    "engine": "gateway",
    "retryable": false
  }
}
```

---

# 7. Pipeline Engine

## 7.1 Purpose

Pipeline Engine orchestrates the entire request lifecycle.

It is the heart of Novexa.

---

## 7.2 Responsibilities

Pipeline Engine is responsible for:

- creating Pipeline Context
- calling engines in correct order
- triggering plugin hooks
- managing retries
- enforcing timeout budgets
- handling recoverable failures
- producing final response
- recording pipeline events

---

## 7.3 Default Engine Order

```text
Gateway Engine
↓
Workspace Engine
↓
Config Engine
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
Response Engine
↓
Validation Engine
↓
Repair Engine
↓
Telemetry Engine
```

Plugin hooks may run between these stages.

---

## 7.4 Input

Pipeline Engine input:

```text
NormalizedRequest
```

---

## 7.5 Output

Pipeline Engine output:

```text
FinalResponse
```

---

## 7.6 Retry Rules

Pipeline Engine may retry when:

- provider timeout
- provider unavailable
- validation failed
- loop detected
- structured output malformed
- incomplete response detected

Pipeline Engine must not retry endlessly.

Default limits:

```yaml
retry:
  max_attempts: 2
  max_repair_attempts: 1
  max_provider_attempts: 2
```

---

## 7.7 Failure Behaviour

If an engine returns `failed_recoverable`, Pipeline Engine may retry or skip depending on config.

If an engine returns `failed_fatal`, Pipeline Engine must stop and return a structured error.

---

# 8. Workspace Engine

## 8.1 Purpose

Workspace Engine resolves project-level configuration and isolation.

In V1, Novexa supports a default local workspace.

---

## 8.2 Responsibilities

Workspace Engine is responsible for:

- resolving workspace ID
- loading workspace metadata
- isolating sessions
- isolating memory
- isolating telemetry
- supporting future multi-project usage

---

## 8.3 Input

```text
PipelineContext with normalized request
```

---

## 8.4 Output

```text
PipelineContext with workspace_id
```

---

## 8.5 V1 Behaviour

If no workspace is provided:

```text
workspace_id = default
```

---

## 8.6 Boundaries

Workspace Engine must not:

- manage provider selection
- modify prompts
- validate responses
- perform memory retrieval directly

---

# 9. Config Engine

## 9.1 Purpose

Config Engine loads and resolves runtime configuration.

---

## 9.2 Responsibilities

Config Engine is responsible for:

- loading runtime defaults
- loading global config
- loading workspace config
- reading environment variables
- applying request-level overrides
- validating final config snapshot
- exposing config to Pipeline Context

---

## 9.3 Config Precedence

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

---

## 9.4 Input

```text
PipelineContext with workspace_id
```

---

## 9.5 Output

```text
PipelineContext with config_snapshot
```

---

## 9.6 Failure Behaviour

Invalid config should return:

```json
{
  "error": {
    "code": "INVALID_CONFIG",
    "message": "The provider default_model is missing.",
    "engine": "config",
    "retryable": false
  }
}
```

---

# 10. Session Engine

## 10.1 Purpose

Session Engine manages short-term conversation state.

---

## 10.2 Responsibilities

Session Engine is responsible for:

- creating session IDs
- resolving existing sessions
- storing recent message history
- tracking conversation metadata
- linking requests to sessions
- preparing conversation state for Context Engine

---

## 10.3 Input

```text
PipelineContext with normalized_request
```

---

## 10.4 Output

```text
PipelineContext with session_id and recent session messages
```

---

## 10.5 Session ID Rules

Session may be resolved from:

- explicit request metadata
- API key workspace default
- generated request session

If no session exists, create a new one.

---

## 10.6 V1 Storage

V1 may store sessions in:

```text
SQLite
```

or in-memory for early development.

---

# 11. Context Engine

## 11.1 Purpose

Context Engine prepares the best possible context window for the selected model.

It prevents context overflow, context pollution, and unnecessary token waste.

---

## 11.2 Responsibilities

Context Engine is responsible for:

- estimating token count
- trimming irrelevant messages
- removing duplicate content
- compressing long conversations
- preserving important facts
- respecting model context limit
- creating a compact context package

---

## 11.3 Input

```text
PipelineContext with:
- messages_original
- session messages
- model_profile
- config_snapshot
```

---

## 11.4 Output

```text
PipelineContext with:
- messages_normalized
- messages_compressed
- context_report
```

---

## 11.5 Context Strategies

Supported strategies:

```text
none
trim
summarize
compress
hybrid
```

## none

No context processing.

Used in Direct Mode.

## trim

Remove oldest or lowest-priority content.

## summarize

Summarize older conversation history.

## compress

Rewrite context into a compact structured form.

## hybrid

Trim, summarize, and compress based on token budget.

---

## 11.6 Context Package Format

```text
ContextPackage
├── active_user_request
├── recent_messages
├── preserved_facts
├── decisions
├── constraints
├── relevant_memory
├── omitted_content_summary
└── token_budget_report
```

---

## 11.7 Failure Behaviour

If compression fails, Context Engine should fallback to trim strategy.

It should not fail the entire request unless context cannot be safely prepared.

---

# 12. Memory Engine

## 12.1 Purpose

Memory Engine manages durable and retrievable long-term knowledge.

Memory Engine is optional in V1.

---

## 12.2 Responsibilities

Memory Engine is responsible for:

- storing durable user/project facts
- retrieving relevant memories
- summarizing long sessions
- linking memory to workspace
- supporting future RAG
- preventing memory pollution

---

## 12.3 Input

```text
PipelineContext with:
- workspace_id
- session_id
- active_user_request
- context_package
```

---

## 12.4 Output

```text
PipelineContext with:
- memory_results
- memory_report
```

---

## 12.5 V1 Memory Scope

V1 may implement:

- session summary memory
- workspace notes
- basic keyword retrieval

Full vector RAG is V2.

---

## 12.6 Boundaries

Memory Engine must not:

- silently store sensitive content
- overwrite user intent
- inject unrelated memory
- require cloud services

---

# 13. Prompt Engine

## 13.1 Purpose

Prompt Engine builds the model-ready prompt package.

It improves clarity while preserving user intent.

---

## 13.2 Responsibilities

Prompt Engine is responsible for:

- building system prompt
- applying model profile instructions
- applying workspace rules
- applying response format rules
- applying guardrail instructions
- preparing tool instructions in future versions
- creating final provider-ready messages

---

## 13.3 Input

```text
PipelineContext with:
- context_package
- memory_results
- model_profile
- config_snapshot
- normalized_request
```

---

## 13.4 Output

```text
PipelineContext with prompt_package
```

---

## 13.5 Prompt Package Format

```text
PromptPackage
├── system_prompt
├── developer_instructions
├── context_block
├── memory_block
├── user_messages
├── response_format_instructions
├── model_profile_instructions
└── final_messages
```

---

## 13.6 Rules

Prompt Engine must:

- preserve user intent
- avoid over-optimizing simple prompts
- never invent facts
- respect response format
- respect Direct Mode
- expose what was changed through telemetry

---

# 14. Guard Engine

## 14.1 Purpose

Guard Engine applies runtime behaviour controls.

It focuses on reliability, not policy enforcement alone.

---

## 14.2 Responsibilities

Guard Engine is responsible for:

- detecting high hallucination risk
- detecting loop risk
- enforcing response constraints
- checking prompt risk conditions
- applying structured output requirements
- blocking unsafe provider calls when configured
- generating guard warnings

---

## 14.3 Input

```text
PipelineContext with prompt_package
```

---

## 14.4 Output

```text
PipelineContext with guard_report
```

---

## 14.5 Guard Types

V1 guard types:

```text
anti_loop
structured_output
context_overflow
empty_prompt
provider_unavailable
unsupported_model_feature
```

Future guard types:

```text
tool_safety
agent_step_limit
hallucination_checker
source_grounding
```

---

## 14.6 Failure Behaviour

Guard Engine may:

- allow
- warn
- modify request constraints
- request retry
- block request

Blocking should be rare and explicit.

---

# 15. Provider Engine

## 15.1 Purpose

Provider Engine communicates with inference providers.

It hides provider-specific APIs from the rest of Novexa.

---

## 15.2 Responsibilities

Provider Engine is responsible for:

- selecting provider
- selecting model
- checking provider health
- converting prompt package to provider request
- sending generate request
- handling streaming
- normalizing provider errors
- returning raw provider output

---

## 15.3 Input

```text
PipelineContext with:
- prompt_package
- model_profile
- provider config
```

---

## 15.4 Output

```text
PipelineContext with provider_response_raw
```

---

## 15.5 Provider Adapter Contract

Each provider adapter must implement:

```text
ProviderAdapter
├── name()
├── health_check()
├── list_models()
├── generate(request)
├── stream(request)
├── normalize_error(error)
└── capabilities()
```

---

## 15.6 V1 Providers

V1 must support:

```text
ollama
lmstudio
openai_compatible_local
```

---

## 15.7 Provider Selection

Provider selection may use:

- explicit request model
- config default provider
- model profile provider
- provider availability
- request mode

---

## 15.8 Boundary Rule

Provider Engine must not:

- optimize prompts
- validate content quality
- repair model output
- store memory
- perform business logic

---

# 16. Response Engine

## 16.1 Purpose

Response Engine normalizes provider-specific output into Novexa response format.

---

## 16.2 Responsibilities

Response Engine is responsible for:

- extracting assistant content
- handling provider metadata
- normalizing token usage
- detecting incomplete generations
- detecting repeated output
- preparing output for Validation Engine

---

## 16.3 Input

```text
PipelineContext with provider_response_raw
```

---

## 16.4 Output

```text
PipelineContext with response_normalized
```

---

## 16.5 Repeat Detection

Response Engine should detect:

- repeated lines
- repeated paragraphs
- repeated code blocks
- repeated tool calls in future Agent Mode

---

# 17. Validation Engine

## 17.1 Purpose

Validation Engine checks whether model output satisfies required constraints.

It reports problems but does not directly repair output.

---

## 17.2 Responsibilities

Validation Engine is responsible for:

- validating JSON schema
- validating required response format
- checking malformed Markdown
- checking YAML/XML validity where applicable
- detecting empty response
- detecting repetition
- producing validation report

---

## 17.3 Input

```text
PipelineContext with response_normalized
```

---

## 17.4 Output

```text
PipelineContext with validation_report
```

---

## 17.5 Validation Report Format

```text
ValidationReport
├── passed
├── severity
├── issues
├── repairable
├── suggested_repair_strategy
└── metadata
```

---

## 17.6 Severity Levels

```text
info
warning
error
fatal
```

---

# 18. Repair Engine

## 18.1 Purpose

Repair Engine fixes invalid or low-quality outputs when safe.

---

## 18.2 Responsibilities

Repair Engine is responsible for:

- repairing broken JSON
- removing repeated output
- requesting limited regeneration
- retrying with stricter prompt
- preserving valid content
- recording repair report

---

## 18.3 Input

```text
PipelineContext with:
- response_normalized
- validation_report
```

---

## 18.4 Output

```text
PipelineContext with:
- repaired response
- repair_report
```

---

## 18.5 Repair Strategies

```text
none
local_parse_repair
regex_cleanup
model_repair
retry_generation
fallback_provider
```

V1 should support:

```text
local_parse_repair
regex_cleanup
retry_generation
```

Cloud fallback is out of scope for V1.

---

## 18.6 Repair Limits

Default:

```yaml
repair:
  max_attempts: 1
  allow_full_regeneration: true
  preserve_valid_content: true
```

---

# 19. Telemetry Engine

## 19.1 Purpose

Telemetry Engine records what happened during request processing.

Telemetry makes Novexa explainable.

---

## 19.2 Responsibilities

Telemetry Engine is responsible for tracking:

- request ID
- workspace ID
- provider
- model
- latency
- estimated tokens
- context compression
- prompt optimization
- validation results
- repair attempts
- errors
- warnings
- plugin events

---

## 19.3 Input

```text
PipelineContext
```

---

## 19.4 Output

```text
TelemetryRecord
```

---

## 19.5 Privacy

Default telemetry must not store full prompt or full response.

Detailed prompt logging must be explicitly enabled.

---

# 20. Plugin Engine

## 20.1 Purpose

Plugin Engine allows Novexa to be extended safely.

Plugins should enhance runtime behaviour without bloating core.

---

## 20.2 Responsibilities

Plugin Engine is responsible for:

- discovering plugins
- validating plugin manifests
- loading plugins
- exposing hooks
- enforcing permissions
- isolating plugin failures
- recording plugin telemetry

---

## 20.3 Plugin Manifest

Example:

```yaml
name: novexa-plugin-better-json
version: 0.1.0
type: validation
entry: plugin.wasm

permissions:
  - read_prompt_metadata
  - modify_response

hooks:
  - before_validation
  - after_validation
```

---

## 20.4 Hook Categories

```text
request_hooks
context_hooks
prompt_hooks
provider_hooks
validation_hooks
response_hooks
error_hooks
telemetry_hooks
```

---

## 20.5 Plugin Failure Rule

Plugin failure must not crash the runtime unless plugin is marked as required.

---

# 21. Dashboard Engine

## 21.1 Purpose

Dashboard Engine exposes local observability UI.

---

## 21.2 Responsibilities

Dashboard Engine is responsible for displaying:

- runtime status
- provider status
- active model
- recent requests
- latency
- retries
- validation failures
- repair events
- context compression stats
- plugin status

---

## 21.3 V1 Dashboard Scope

V1 dashboard should be local-only.

Default:

```text
http://localhost:8788
```

---

# 22. CLI Engine

## 22.1 Purpose

CLI Engine provides developer control over Novexa.

---

## 22.2 V1 Commands

```bash
novexa start
novexa stop
novexa status
novexa doctor
novexa providers
novexa models
novexa config
novexa benchmark
```

---

## 22.3 Future Commands

```bash
novexa plugin install
novexa plugin list
novexa profile list
novexa profile test
novexa workspace create
novexa session inspect
```

---

# 23. Engine Interaction Rules

## 23.1 No Direct Cross-Engine Calls

Engines should not call each other directly except through Pipeline Engine.

Bad:

```text
Prompt Engine → Provider Engine
```

Good:

```text
Prompt Engine → Pipeline Context → Pipeline Engine → Provider Engine
```

---

## 23.2 No Business Logic in Providers

Provider adapters only translate and transport requests.

---

## 23.3 No Hidden Mutation

If an engine modifies Pipeline Context, it must record an event.

Example:

```text
event: context_compressed
metadata:
  original_estimated_tokens: 18000
  compressed_estimated_tokens: 6200
```

---

## 23.4 Everything Must Be Inspectable

Every major runtime decision must be visible in telemetry.

---

# 24. Testing Requirements

Each engine must have:

- unit tests
- failure tests
- config tests
- telemetry event tests

Pipeline Engine must have:

- integration tests
- retry tests
- provider failure tests
- validation repair tests
- streaming tests

---

# 25. V1 Implementation Priority

Implement engines in this order:

```text
1. Config Engine
2. Gateway Engine
3. Provider Engine
4. Pipeline Engine
5. Response Engine
6. Telemetry Engine
7. Context Engine
8. Prompt Engine
9. Validation Engine
10. Repair Engine
11. Session Engine
12. Dashboard Engine
13. CLI Engine
14. Plugin Engine
15. Memory Engine
```

Reason:

Novexa must first function as a gateway.

Then it becomes intelligent.

Then it becomes extensible.

---

# 26. Final Engine Statement

Novexa engines are small, explicit, observable components coordinated by the Pipeline Engine.

Each engine owns one responsibility.

The runtime becomes powerful not because any single engine is complex, but because the pipeline combines simple engines into a stable intelligence layer for local AI.