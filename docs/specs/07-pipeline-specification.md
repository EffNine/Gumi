# Gumi Pipeline Specification

Version: 1.0  
Status: Draft  
Scope: Intelligence Pipeline for Gumi Runtime V1

---

# 1. Purpose

This document defines the Gumi Intelligence Pipeline.

The Pipeline is the core execution path for every request handled by Gumi Runtime.

Its responsibilities are:

- receive normalized requests
- create Pipeline Context
- execute runtime engines in correct order
- trigger plugin hooks
- handle retries
- recover from failures where safe
- record telemetry
- return a final response

No request should bypass the Pipeline unless explicitly running in a diagnostic or internal system mode.

---

# 2. Pipeline Philosophy

The Pipeline must be:

- deterministic
- inspectable
- modular
- recoverable
- testable
- local-first
- privacy-first

A developer should be able to inspect a request and understand:

- which provider was used
- which model was used
- which engines ran
- what was optimized
- what was compressed
- what failed
- whether repair happened
- whether retry happened

Gumi must avoid hidden magic.

Every major decision should produce a pipeline event.

---

# 3. Pipeline Position

```text
Gateway Engine
    ↓
Pipeline Engine
    ↓
Runtime Engines
    ↓
Provider Engine
    ↓
Local Provider
    ↓
Runtime Engines
    ↓
Gateway Engine
```

Gateway Engine receives requests.

Pipeline Engine executes the intelligence lifecycle.

Provider Engine communicates with local inference engines.

---

# 4. Core Rule

Every external inference request must pass through the Pipeline Engine.

Invalid:

```text
Gateway Engine → Provider Engine
```

Valid:

```text
Gateway Engine → Pipeline Engine → Provider Engine
```

This rule keeps Gumi observable, configurable, and extensible.

---

# 5. Pipeline Lifecycle Overview

```text
1. Receive Normalized Request
2. Create Pipeline Context
3. Resolve Workspace
4. Resolve Config
5. Resolve Session
6. Resolve Model Profile
7. Run Pre-Request Hooks
8. Prepare Context
9. Retrieve Memory
10. Build Prompt
11. Apply Guardrails
12. Select Provider
13. Generate Response
14. Normalize Response
15. Validate Response
16. Repair If Needed
17. Retry If Needed
18. Run Post-Response Hooks
19. Record Telemetry
20. Return Final Response
```

---

# 6. Pipeline Context

The Pipeline Context is the single source of truth for a request.

Every engine reads from and writes to Pipeline Context.

Engines should not pass hidden data directly to each other.

---

## 6.1 Pipeline Context Structure

```text
PipelineContext
├── identity
│   ├── request_id
│   ├── workspace_id
│   ├── session_id
│   └── trace_id
│
├── mode
│   ├── runtime_mode
│   ├── stream
│   └── compatibility_level
│
├── request
│   ├── incoming_request
│   ├── normalized_request
│   ├── request_overrides
│   └── headers
│
├── config
│   ├── runtime_defaults
│   ├── global_config
│   ├── workspace_config
│   ├── env_overrides
│   ├── request_overrides
│   └── config_snapshot
│
├── model
│   ├── requested_model
│   ├── selected_provider
│   ├── selected_model
│   ├── model_profile
│   └── capabilities
│
├── messages
│   ├── original_messages
│   ├── normalized_messages
│   ├── session_messages
│   ├── compressed_messages
│   └── final_messages
│
├── context
│   ├── context_strategy
│   ├── token_budget
│   ├── context_package
│   ├── context_report
│   └── omitted_content_summary
│
├── memory
│   ├── memory_enabled
│   ├── memory_results
│   └── memory_report
│
├── prompt
│   ├── system_prompt
│   ├── developer_instructions
│   ├── prompt_package
│   └── prompt_report
│
├── guard
│   ├── guard_report
│   ├── blocked
│   ├── warnings
│   └── constraints
│
├── provider
│   ├── provider_request
│   ├── provider_response_raw
│   ├── provider_error
│   └── provider_latency_ms
│
├── response
│   ├── response_normalized
│   ├── validation_report
│   ├── repair_report
│   └── final_response
│
├── retry
│   ├── attempt
│   ├── max_attempts
│   ├── retry_reason
│   └── retry_history
│
├── telemetry
│   ├── events
│   ├── warnings
│   ├── errors
│   ├── timings
│   └── metrics
│
└── plugins
    ├── plugin_events
    ├── hook_results
    └── plugin_errors
```

---

## 6.2 Pipeline Context Rules

1. Pipeline Context must be created once per request.
2. Every mutation must be recorded as an event.
3. Engines may only modify their assigned context sections.
4. Sensitive values must be redacted before telemetry export.
5. Final response is generated from Pipeline Context.
6. Pipeline Context should be serializable for debugging.
7. Full prompts and responses must not be stored unless enabled.

---

# 7. Pipeline Events

Pipeline events make Gumi explainable.

Every significant action should emit an event.

---

## 7.1 Event Format

```text
PipelineEvent
├── timestamp
├── request_id
├── engine
├── event
├── severity
├── message
└── metadata
```

Example:

```yaml
timestamp: 2026-07-10T00:00:00Z
request_id: req_abc123
engine: context
event: context_compressed
severity: info
message: Context was compressed successfully.
metadata:
  original_estimated_tokens: 18000
  compressed_estimated_tokens: 6200
  strategy: hybrid
```

---

## 7.2 Severity Levels

```text
debug
info
warning
error
fatal
```

---

## 7.3 Required Events

Every request should emit at least:

```text
request_received
pipeline_started
workspace_resolved
config_resolved
session_resolved
model_resolved
provider_selected
provider_request_started
provider_request_completed
response_normalized
validation_completed
telemetry_recorded
pipeline_completed
```

If applicable:

```text
context_compressed
prompt_optimized
guard_warning
guard_blocked
validation_failed
repair_started
repair_completed
retry_started
retry_completed
plugin_hook_executed
provider_error
pipeline_failed
```

---

# 8. Runtime Modes

Gumi supports multiple runtime modes.

Runtime mode controls how much processing the Pipeline performs.

---

## 8.1 Direct Mode

Direct Mode performs minimal processing.

```text
Request
    ↓
Config
    ↓
Provider
    ↓
Response
```

Use cases:

- maximum speed
- debugging provider behaviour
- user wants raw model output
- benchmarking

Direct Mode should skip:

- context compression
- prompt optimization
- validation
- repair
- memory

Direct Mode still includes:

- request normalization
- config resolution
- provider selection
- provider call
- telemetry

---

## 8.2 Lightweight Mode

Lightweight Mode is the recommended default for coding agents and other applications that already provide their own interaction scaffolding but still want Gumi as the central tuning layer.

```text
Request
    ↓
Config
    ↓
Resolve Model Profile
    ↓
Apply Profile Defaults
    ↓
Apply Thinking Policy
    ↓
Apply Minimal Prompt Policy
    ↓
Provider
    ↓
Response
    ↓
Telemetry
```

Position:

- Higher overhead than Direct Mode: it still resolves profiles, applies defaults, and enforces a minimal prompt policy.
- Lower overhead than Stabilized Mode: it skips context compression, long system wrappers, memory, and most validation/repair.

Use cases:

- OpenCode
- Continue
- Cline
- Open WebUI
- custom coding agents
- apps that should not receive a heavy Gumi prompt wrapper

Lightweight Mode should apply:

- request normalization
- workspace and config resolution
- model profile resolution
- provider selection
- model profile defaults (temperature, top_p, repeat_penalty, max_tokens, stop, thinking)
- thinking policy (disable reasoning when profile or request says false)
- minimal exact-format/plain-text instruction
- lightweight guard checks (empty prompt, context overflow estimate, unsupported feature)
- local telemetry

Lightweight Mode should skip by default:

- context compression, summarization, and memory retrieval
- the full Gumi base system prompt
- long prompt wrappers and workspace instruction blocks
- anti-loop, hallucination, and structured-output guards (unless explicitly requested)
- validation and repair, except when response_format is present
- retry loops beyond provider-level retries

Strategic purpose:

```text
Apps keep simple config.
Gumi owns shared model tuning.
```

---

## 8.2.1 Lightweight Mode Engines

| Engine | Runs | Role |
|---|---|---|
| Gateway Engine | Yes | Receives and normalizes request, extracts `gumi.*` overrides. |
| Workspace Engine | Yes | Resolves default workspace. |
| Config Engine | Yes | Builds config snapshot with lightweight defaults. |
| Session Engine | No | Lightweight mode is stateless per request by default. |
| Context Engine | No | App owns context; token budget is only estimated for overflow guard. |
| Memory Engine | No | Disabled by default. |
| Prompt Engine | Partial | Applies minimal prompt policy only. |
| Guard Engine | Partial | Runs empty-prompt, context-overflow, and unsupported-feature checks. |
| Provider Engine | Yes | Selects provider/model, normalizes request, applies profile defaults, calls provider. |
| Response Engine | Yes | Normalizes provider output. |
| Validation Engine | Conditional | Only when `response_format` is present. |
| Repair Engine | Conditional | Only when `response_format` validation fails and repair is explicitly enabled. |
| Telemetry Engine | Yes | Records mode, profile, provider, latency, warnings. |

---

## 8.2.2 Lightweight Mode Prompt Policy

Prompt Engine optimization level forced to **light** in lightweight mode.

Applied:

- If the request contains no `system` or `developer` message, a minimal system prompt may be inserted from the model profile (`system_prompt_style: minimal`) or the generic fallback.
- If a `system` or `developer` message exists, Gumi does **not** replace it. It may append a single short anti-loop sentence only when `guard.anti_loop != off`.
- When `response_format.type = json_object`, append a minimal JSON-only instruction.
- When `response_format.type = json_schema`, append a minimal schema-conformance instruction.

Skipped:

- Full base Gumi system prompt.
- Runtime mode instruction block.
- Workspace instruction block.
- Memory block.
- Context compression summary.
- Vague-prompt rewriting.
- Additional task framing.

---

## 8.2.3 Lightweight Mode Defaults and Overrides

Default settings for lightweight mode:

```yaml
engines:
  context:
    enabled: false
  memory:
    enabled: false
  validation:
    enabled: false
  repair:
    enabled: false
  guard:
    anti_loop: light
    context_overflow: true
    structured_output: false
  prompt:
    optimization_level: light
    preserve_user_intent: true
```

Request-level overrides can opt back into individual features:

```json
{
  "gumi": {
    "mode": "lightweight",
    "validation": { "enabled": true, "repair": true },
    "guard": { "anti_loop": "aggressive" }
  }
}
```

---

## 8.2.4 Lightweight Mode Risks and Non-Goals

Risks:

| Risk | Mitigation |
|---|---|
| Apps assume full stabilization quality. | Telemetry and response metadata expose `runtime_mode: lightweight` and skipped engines. |
| Weak client prompts are not rescued. | Document that lightweight mode preserves app prompts. |
| Long sessions overflow context. | Guard emits `context_overflow_warning` but does not compress unless requested. |
| JSON output wrapped in markdown. | JSON instruction is added only when `response_format` is explicit. |

Non-goals:

- Not a separate pipeline implementation; it is a branch of the existing Pipeline Engine.
- Not a prompt optimization mode.
- Not a context management or memory mode.
- Not an agent orchestration mode (no step budgets or tool-call validation).
- Not a replacement for Direct Mode, which remains the raw passthrough.

---

## 8.3 Stabilized Mode

Stabilized Mode is the default.

```text
Request
    ↓
Context Engine
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

Use cases:

- normal app usage
- local chatbot
- coding assistant gateway
- OpenAI-compatible app integration

Stabilized Mode improves reliability without requiring strict schema.

---

## 8.4 Structured Mode

Structured Mode is for JSON/schema-heavy tasks.

It adds stronger validation and repair.

```text
Request
    ↓
Context Engine
    ↓
Prompt Engine with strict formatting
    ↓
Guard Engine
    ↓
Provider Engine
    ↓
Validation Engine
    ↓
Repair Engine
    ↓
Retry if malformed
    ↓
Response
```

Use cases:

- app backends
- extraction
- classification
- scoring
- structured API responses
- tool-like JSON generation

Structured Mode should activate automatically when:

```text
response_format.type = json_object
response_format.type = json_schema
```

Unless explicitly disabled.

---

## 8.4 Agent Mode

Agent Mode is a coding-agent governance layer implemented in Sprint 14 (v0.2.0-alpha).

It extends stabilized mode with four additional governance layers:

### 8.4.1 Step Budget Enforcement

Agent frameworks send multiple requests within a session. Agent mode enforces a step budget by counting `assistant` messages in the incoming request. When the count exceeds `max_steps` (default: 30), the pipeline returns `AGENT_STEP_LIMIT_EXCEEDED` (HTTP 429).

The calling agent framework should manage its own step counter and stop when Gumi returns this error.

### 8.4.2 Tool-Call Loop Detection

The guard engine's `hasToolCallLoop` check runs on every agent request. If the same tool call (same name + arguments) repeats:

- **2+ times** (standard detection): a loop-break hint is injected into the system prompt.
- **3+ times** (strict/aggressive detection): the request is blocked with `AGENT_TOOL_CALL_LOOP` (HTTP 409).

The loop detection level is configured via `runtime.agent.loop_detection` (default: `strict`).

### 8.4.3 Tool-Call JSON Validation + Repair

When the provider returns an assistant message with `tool_calls`:

1. Each `tool_call.arguments` is validated as valid JSON via `json.Unmarshal`.
2. If invalid, the repair engine attempts to extract and fix the JSON.
3. If repair succeeds, the tool call is patched. If repair fails, the response is flagged with `AGENT_INVALID_TOOL_CALL` (HTTP 422).

### 8.4.4 Context Compaction Hints

After every request, the estimated token count of the full message list is compared against `context_compaction_threshold` (default: 0.85) of the model's context limit. If exceeded, a system hint is injected: "The conversation is approaching the model's context limit. Summarize key findings and trim redundant information."

The message list is not modified — only warned/hinted. The agent framework decides when to compact.

### 8.4.5 Streaming Support

Unlike structured mode, agent mode supports streaming. Governance checks run pre-generation (step budget, loop detection). Tool-call validation runs post-hoc on the accumulated stream buffer.

### 8.4.6 V1 Scope

**Included:**
- Step budget enforcement (per-request)
- Tool-call loop detection (inline, per-turn)
- Tool-call JSON validation + repair
- Context compaction hints
- Streaming support
- Telemetry (agent_step_count, agent_loop_detected)

**Deferred (Year 2+):**
- Rollback hints
- Progress scoring
- Planning checks
- Session-level step tracking

---

# 9. Default Pipeline Orders

---

## 9.1 Direct Mode Order

```text
1. request_received
2. normalize_request
3. resolve_workspace
4. resolve_config
5. resolve_model
6. select_provider
7. call_provider
8. normalize_response
9. record_telemetry
10. return_response
```

---

## 9.2 Lightweight Mode Order

```text
1. request_received
2. normalize_request
3. create_pipeline_context
4. resolve_workspace
5. resolve_config
6. resolve_model_profile
7. apply_profile_defaults
8. apply_thinking_policy
9. run_before_request_hooks
10. build_minimal_prompt
11. apply_lightweight_guardrails
12. run_before_provider_hooks
13. select_provider
14. call_provider
15. run_after_provider_hooks
16. normalize_response
17. validate_response_if_format_requested
18. repair_if_explicitly_enabled
19. run_after_response_hooks
20. record_telemetry
21. return_response
```

Notes:

- `resolve_session` is skipped by default.
- `prepare_context` and `retrieve_memory` are skipped.
- Validation and repair run only when `response_format` is present or explicitly enabled.

---

## 9.3 Stabilized Mode Order

```text
1. request_received
2. normalize_request
3. create_pipeline_context
4. resolve_workspace
5. resolve_config
6. resolve_session
7. resolve_model_profile
8. enforce_structured_mode
9. run_before_request_hooks
10. prepare_context
11. retrieve_memory_if_enabled
12. build_strict_prompt
13. apply_guardrails
14. run_before_provider_hooks
15. select_provider
16. call_provider
17. run_after_provider_hooks
18. normalize_response
19. validate_schema
20. repair_if_needed
21. retry_if_still_invalid
22. run_after_response_hooks
23. record_telemetry
24. return_response
```

---

# 10. Pipeline Stage Specifications

---

## 10.1 Normalize Request

Purpose:

Convert API-specific request into internal normalized form.

Input:

```text
OpenAI-compatible request
```

Output:

```text
NormalizedRequest
```

Responsibilities:

- validate basic request shape
- normalize messages
- normalize model ID
- parse Gumi extension field
- resolve stream flag
- preserve unsupported fields where safe

Failure examples:

```text
INVALID_REQUEST
MISSING_MESSAGES
UNSUPPORTED_ROLE
```

---

## 10.2 Resolve Workspace

Purpose:

Determine workspace identity.

Input:

```text
NormalizedRequest
Headers
```

Output:

```text
workspace_id
```

Default V1:

```text
default
```

Sources:

```text
X-Gumi-Workspace header
API key mapping
request metadata
default workspace
```

---

## 10.3 Resolve Config

Purpose:

Create final config snapshot.

Input:

```text
workspace_id
environment
CLI flags
request overrides
```

Output:

```text
config_snapshot
```

Rules:

- request overrides apply only to this request
- sensitive values must be redacted in telemetry
- invalid config stops pipeline

---

## 10.4 Resolve Session

Purpose:

Resolve or create session.

Input:

```text
NormalizedRequest
X-Gumi-Session header
gumi.session.id
```

Output:

```text
session_id
session_messages
```

Rules:

- session is optional
- if no session exists, create ephemeral session
- persistent session requires config or request flag

---

## 10.5 Resolve Model Profile

Purpose:

Load model-specific behaviour recommendations.

Input:

```text
requested_model
provider config
profiles directory
```

Output:

```text
model_profile
capabilities
```

Rules:

- missing profile should not fail request
- use generic profile if specific profile missing
- emit warning when profile is missing

---

## 10.6 Prepare Context

Purpose:

Fit useful information into model context window.

Input:

```text
original_messages
session_messages
memory_results
model_profile
config_snapshot
```

Output:

```text
context_package
context_report
```

Strategies:

```text
none
trim
summarize
compress
hybrid
```

Fallback:

If compression fails, fallback to trim.

---

## 10.7 Retrieve Memory

Purpose:

Inject relevant durable memory.

Input:

```text
workspace_id
session_id
active_user_request
context_package
```

Output:

```text
memory_results
memory_report
```

V1:

- optional
- disabled by default
- may support session summary memory only

---

## 10.8 Build Prompt

Purpose:

Create final provider-ready messages.

Input:

```text
context_package
memory_results
model_profile
response_format
guard constraints
```

Output:

```text
prompt_package
final_messages
```

Rules:

- preserve user intent
- do not invent facts
- do not over-optimize direct/simple requests
- include structured output instructions when required

---

## 10.9 Apply Guardrails

Purpose:

Detect risk and enforce runtime constraints.

Input:

```text
prompt_package
normalized_request
config_snapshot
model_profile
```

Output:

```text
guard_report
constraints
```

Possible outcomes:

```text
allow
warn
modify_constraints
block
request_retry
```

V1 guards:

```text
anti_loop
context_overflow
structured_output
provider_capability_check
empty_prompt
```

---

## 10.10 Select Provider

Purpose:

Choose provider and model.

Input:

```text
requested_model
model_profile
config_snapshot
provider_health
```

Output:

```text
selected_provider
selected_model
provider_request
```

Selection priority:

```text
1. explicit request model
2. request override
3. workspace config
4. model profile
5. global default
6. auto selection
```

---

## 10.11 Call Provider

Purpose:

Send request to selected local provider.

Input:

```text
provider_request
```

Output:

```text
provider_response_raw
provider_error
```

Rules:

- enforce timeout
- support streaming
- normalize provider errors
- provider adapter does not decide retry

---

## 10.12 Normalize Response

Purpose:

Convert provider response into internal response shape.

Input:

```text
provider_response_raw
```

Output:

```text
response_normalized
```

Responsibilities:

- extract assistant content
- normalize usage
- normalize finish reason
- detect incomplete response
- detect obvious repetition

---

## 10.13 Validate Response

Purpose:

Check whether response satisfies constraints.

Input:

```text
response_normalized
response_format
guard constraints
```

Output:

```text
validation_report
```

Validation types:

```text
json
json_schema
markdown
empty_response
repetition
finish_reason
```

---

## 10.14 Repair If Needed

Purpose:

Fix broken or invalid output.

Input:

```text
response_normalized
validation_report
prompt_package
```

Output:

```text
repair_report
updated_response_normalized
```

Repair strategies:

```text
local_parse_repair
regex_cleanup
model_repair
retry_generation
```

V1 default:

```text
local_parse_repair
regex_cleanup
retry_generation
```

---

## 10.15 Retry If Needed

Purpose:

Safely retry failed generation.

Retry can happen after:

- provider error
- validation failure
- repair failure
- repeated output
- incomplete response
- timeout if retryable

Retry must never be infinite.

---

## 10.16 Record Telemetry

Purpose:

Store local observability data.

Input:

```text
PipelineContext
```

Output:

```text
TelemetryRecord
```

Rules:

- no full prompt logging by default
- no full response logging by default
- local telemetry enabled by default
- external telemetry disabled by default

---

# 11. Retry Specification

---

## 11.1 Retry Limits

Default:

```yaml
retry:
  max_attempts: 2
  max_provider_attempts: 2
  max_validation_attempts: 1
  max_repair_attempts: 1
```

Attempt count includes the original attempt.

---

## 11.2 Retryable Conditions

```text
PROVIDER_TIMEOUT
PROVIDER_UNAVAILABLE
PROVIDER_BAD_RESPONSE
EMPTY_RESPONSE
REPEATED_OUTPUT
INVALID_JSON
INVALID_SCHEMA
INCOMPLETE_RESPONSE
```

---

## 11.3 Non-Retryable Conditions

```text
INVALID_REQUEST
INVALID_API_KEY
INVALID_CONFIG
MODEL_NOT_FOUND
MODEL_UNSUPPORTED
UNSUPPORTED_CONTENT_TYPE
PLUGIN_PERMISSION_DENIED
```

---

## 11.4 Retry Strategy

Retry should adjust strategy.

Examples:

For repetition:

```text
increase repeat penalty
lower temperature
shorten context
add anti-repeat instruction
```

For invalid JSON:

```text
switch to stricter prompt
request JSON only
run schema repair
```

For timeout:

```text
reduce max_tokens
trim context
retry once
```

For provider unavailable:

```text
re-check health
retry once
return clear suggestion
```

---

## 11.5 Retry History

Each retry must be recorded.

```text
RetryRecord
├── attempt
├── reason
├── strategy
├── changes_applied
├── result
└── timestamp
```

Example:

```yaml
attempt: 2
reason: INVALID_JSON
strategy: stricter_structured_prompt
changes_applied:
  - added_json_only_instruction
  - reduced_temperature_to_0.2
result: success
```

---

# 12. Repair Specification

---

## 12.1 Repair Decision

Repair Engine runs when:

```text
validation_report.passed = false
validation_report.repairable = true
repair.enabled = true
```

---

## 12.2 Repair Priority

```text
1. local_parse_repair
2. regex_cleanup
3. model_repair
4. retry_generation
```

V1 may skip `model_repair` if not implemented yet.

---

## 12.3 Repair Report

```text
RepairReport
├── attempted
├── strategy
├── success
├── changes
├── remaining_issues
└── metadata
```

Example:

```yaml
attempted: true
strategy: local_parse_repair
success: true
changes:
  - removed trailing markdown fence
  - parsed JSON object successfully
remaining_issues: []
```

---

# 13. Hook Specification

Plugin hooks allow safe extension of the pipeline.

---

## 13.1 Hook Order

```text
before_request
after_request_normalized
before_context
after_context
before_memory
after_memory
before_prompt
after_prompt
before_guard
after_guard
before_provider
after_provider
before_validation
after_validation
before_repair
after_repair
before_response
after_response
on_error
```

---

## 13.2 Hook Input

Plugins receive a limited view of Pipeline Context based on permissions.

```text
PluginHookInput
├── hook_name
├── request_id
├── workspace_id
├── allowed_context
└── metadata
```

---

## 13.3 Hook Output

```text
PluginHookResult
├── status
├── changes
├── warnings
├── errors
└── metadata
```

Allowed statuses:

```text
success
skipped
warning
failed_recoverable
failed_fatal
```

---

## 13.4 Hook Failure Rule

Plugin failure must not crash runtime unless plugin is marked as required.

Default behaviour:

```text
plugin failure → warning → continue pipeline
```

---

## 13.5 Hook Mutation Rule

Plugins can only modify allowed sections.

Example:

A prompt plugin may modify:

```text
prompt_package
prompt_report
```

It may not modify:

```text
provider_response_raw
telemetry database
auth config
```

---

# 14. Streaming Pipeline

Streaming is more complex because response validation happens after or during generation.

---

## 14.1 Streaming Flow

```text
Request
    ↓
Pre-provider pipeline stages
    ↓
Provider streaming starts
    ↓
Chunks normalized
    ↓
Chunks forwarded to client
    ↓
Final accumulated output validated
    ↓
Telemetry recorded
```

---

## 14.2 Streaming Limitation

For V1, repair cannot fully work after content has already been streamed to the client.

Therefore:

- structured mode should prefer non-streaming
- streaming structured output may be marked experimental
- validation for streaming may be telemetry-only
- severe repetition detection may stop stream early

---

## 14.3 Streaming Events

Streaming should emit:

```text
stream_started
stream_chunk_received
stream_completed
stream_stopped_by_guard
stream_failed
```

---

# 15. Failure Handling

---

## 15.1 Failure Categories

```text
request_failure
config_failure
provider_failure
context_failure
validation_failure
repair_failure
plugin_failure
runtime_failure
```

---

## 15.2 Failure Response Requirements

Every failure response must include:

- code
- message
- type
- engine
- retryable
- suggestion
- request_id

---

## 15.3 Fatal Failure

Fatal failure stops pipeline.

Examples:

```text
INVALID_REQUEST
INVALID_CONFIG
INVALID_API_KEY
```

---

## 15.4 Recoverable Failure

Recoverable failure may continue, skip, repair, or retry.

Examples:

```text
CONTEXT_COMPRESSION_FAILED
VALIDATION_FAILED
PLUGIN_HOOK_FAILED
PROVIDER_TIMEOUT
```

---

# 16. Timeout Budget

Pipeline should enforce total timeout budget.

Default:

```yaml
timeouts:
  request_seconds: 120
  provider_seconds: 90
  repair_seconds: 30
```

Rules:

- provider timeout must be less than request timeout
- repair must fit inside remaining request time
- retries must respect total request timeout
- timeout events must be recorded

---

# 17. Pipeline Metrics

Pipeline should expose:

```text
request_count
success_count
failure_count
avg_latency_ms
provider_latency_ms
context_latency_ms
prompt_latency_ms
validation_latency_ms
repair_latency_ms
retry_count
repair_count
validation_failure_count
provider_error_count
```

Dashboard should use these metrics.

---

# 18. Pipeline Testing Requirements

Pipeline must have tests for:

- direct mode success
- lightweight mode success
- stabilized mode success
- structured mode success
- provider unavailable
- provider timeout
- invalid request
- invalid config
- context compression fallback
- validation failure
- repair success
- repair failure
- retry success
- retry limit exceeded
- plugin hook failure
- streaming success
- streaming provider error
- lightweight mode preserves app system prompt
- lightweight mode applies profile defaults
- lightweight mode skips context compression

---

# 19. Pipeline Debugging

Gumi should support debug inspection.

CLI future command:

```bash
gumi trace req_abc123
```

Possible output:

```text
Request: req_abc123
Mode: stabilized
Provider: ollama
Model: qwen3:8b

Timeline:
- request_received: 0ms
- config_resolved: 3ms
- context_compressed: 19ms
- prompt_built: 22ms
- provider_request_started: 24ms
- provider_request_completed: 842ms
- validation_completed: 849ms
- telemetry_recorded: 855ms

Warnings:
- model_profile_missing
```

---

# 20. Pipeline Anti-Patterns

Avoid:

```text
Gateway directly calling Provider
Provider doing prompt optimization
Prompt Engine doing provider selection
Validation Engine mutating output directly
Repair Engine retrying without Pipeline
Plugins mutating unrestricted context
Telemetry storing full prompts by default
Retry loops without limits
```

---

# 21. V1 Implementation Priority

Build pipeline in this order:

```text
1. Pipeline Context
2. Pipeline Event system
3. Direct Mode pipeline
4. Provider call integration
5. Lightweight Mode pipeline
6. Stabilized Mode pipeline
7. Context Engine integration
8. Prompt Engine integration
9. Validation Engine integration
10. Repair Engine integration
11. Streaming pipeline
12. Plugin hook placeholders
13. Structured Mode pipeline
```

Reason:

Lightweight Mode is a thin branch of the pipeline and should be added before Stabilized Mode so that coding-agent integrations can be tested early.

Reason:

Start with a working gateway, then progressively add intelligence.

---

# 22. Final Pipeline Statement

The Gumi Pipeline is the disciplined execution layer that turns a simple local model call into a stable, observable, recoverable AI runtime interaction.

It is the main reason Gumi is more than a proxy.

A proxy forwards requests.

Gumi understands the lifecycle around the request.
