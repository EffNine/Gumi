# Novexa Implementation Roadmap

Version: 1.0  
Status: Draft  
Scope: Development roadmap for Novexa Runtime V1

---

# 1. Purpose

This document defines the implementation roadmap for Novexa Runtime V1.

The goal is to guide development in a disciplined order so Novexa becomes:

- usable early
- testable early
- stable before complex
- extensible without overengineering
- useful as a local AI runtime before adding advanced features

This roadmap is designed for solo development or AI-agent-assisted development.

---

# 2. Development Philosophy

Novexa should be built in layers.

Do not build everything at once.

The correct order is:

```text
Working Gateway
    ↓
Provider Connection
    ↓
Pipeline
    ↓
Telemetry
    ↓
Context + Prompt Intelligence
    ↓
Validation + Repair
    ↓
CLI + Dashboard
    ↓
Profiles
    ↓
Plugins
    ↓
Memory
```

Novexa must first work as a basic local OpenAI-compatible gateway.

Then it becomes intelligent.

Then it becomes extensible.

---

# 3. V1 Product Goal

V1 should allow a developer to:

```bash
novexa start
```

Then connect an OpenAI-compatible app to:

```text
http://localhost:8787/v1
```

And use local models through:

- Ollama
- LM Studio
- OpenAI-compatible local server

With Novexa providing:

- provider abstraction
- basic pipeline
- model profile support
- context preparation
- prompt optimization
- JSON validation
- repair
- anti-loop detection
- local telemetry
- CLI diagnostics
- local dashboard

---

# 4. V1 Non-Goals

Do not build in V1:

- hosted cloud service
- billing
- user accounts
- team accounts
- cloud fallback
- commercial marketplace
- full agent runtime
- tool orchestration
- distributed runtime
- vector database
- enterprise RBAC
- remote telemetry

These are future features.

---

# 5. Recommended Tech Stack

## 5.1 Runtime

Recommended:

```text
Go
```

Reasons:

- single binary
- strong concurrency
- easy HTTP server
- good cross-platform support
- simpler than Rust for early development
- faster and lighter than Node for runtime service

Alternative:

```text
Rust
```

Use Rust only if performance and safety are more important than development speed.

---

## 5.2 Dashboard

Recommended:

```text
Next.js / React
```

Alternative:

```text
SvelteKit
```

For V1, dashboard can be simple.

Do not overdesign UI before runtime works.

---

## 5.3 Local Storage

```text
SQLite
```

---

## 5.4 Config

```text
YAML
```

---

## 5.5 API

```text
HTTP + Server-Sent Events
OpenAI-compatible routes
```

---

# 6. Repository Structure

Recommended monorepo:

```text
novexa/
├── docs/
├── runtime/
│   ├── cmd/
│   │   └── novexa/
│   ├── internal/
│   │   ├── api/
│   │   ├── gateway/
│   │   ├── pipeline/
│   │   ├── config/
│   │   ├── providers/
│   │   ├── context/
│   │   ├── prompt/
│   │   ├── guard/
│   │   ├── validation/
│   │   ├── repair/
│   │   ├── telemetry/
│   │   ├── storage/
│   │   ├── profiles/
│   │   ├── plugins/
│   │   └── cli/
│   ├── pkg/
│   ├── go.mod
│   └── README.md
│
├── dashboard/
│   ├── app/
│   ├── components/
│   ├── lib/
│   └── package.json
│
├── profiles/
│   ├── generic-local.yaml
│   ├── qwen3-8b.yaml
│   ├── deepseek-r1-8b.yaml
│   └── llama3.1-8b.yaml
│
├── plugins/
├── examples/
│   ├── python-openai-client/
│   ├── node-openai-client/
│   └── curl/
│
└── README.md
```

---

# 7. Development Phases

V1 development is divided into 10 phases.

```text
Phase 0: Repository and Documentation
Phase 1: Runtime Skeleton
Phase 2: OpenAI-Compatible Gateway
Phase 3: Provider Adapters
Phase 4: Pipeline Engine
Phase 5: Storage and Telemetry
Phase 6: Context and Prompt Engines
Phase 7: Validation, Repair, Guard
Phase 8: Model Profiles
Phase 9: CLI and Dashboard
Phase 10: Packaging and Release
```

---

# 8. Phase 0: Repository and Documentation

## 8.1 Goal

Create project structure and lock architectural documents.

## 8.2 Tasks

```text
- create monorepo
- create docs folder
- add architecture docs
- add README
- add license
- add contribution notes
- add initial roadmap
- add ADR folder
```

## 8.3 Deliverables

```text
docs/
README.md
LICENSE
CONTRIBUTING.md
```

## 8.4 Exit Criteria

Phase 0 is complete when:

- docs are committed
- repository structure exists
- architecture direction is clear
- coding agent has source-of-truth documents

---

# 9. Phase 1: Runtime Skeleton

## 9.1 Goal

Create runnable Novexa binary.

## 9.2 Tasks

```text
- initialize Go module
- create cmd/novexa entrypoint
- create basic CLI parser
- implement novexa version
- implement novexa start placeholder
- implement config loader stub
- implement logging stub
- implement graceful shutdown
```

## 9.3 Deliverables

```bash
novexa version
novexa start
```

## 9.4 Exit Criteria

Phase 1 is complete when:

```bash
go run ./cmd/novexa version
go run ./cmd/novexa start
```

both work.

Runtime does not need AI yet.

---

# 10. Phase 2: OpenAI-Compatible Gateway

## 10.1 Goal

Expose basic HTTP API.

## 10.2 Tasks

```text
- implement HTTP server
- implement GET /health
- implement GET /v1/models placeholder
- implement POST /v1/chat/completions placeholder
- implement OpenAI-compatible request structs
- implement OpenAI-compatible response structs
- implement standard error format
- implement request ID middleware
- implement auth middleware with local key
```

## 10.3 Deliverables

```http
GET /health
GET /v1/models
POST /v1/chat/completions
```

## 10.4 Exit Criteria

Phase 2 is complete when cURL can call the gateway and receive valid JSON response.

Example:

```bash
curl http://localhost:8787/health
```

---

# 11. Phase 3: Provider Adapters

## 11.1 Goal

Connect Novexa to local providers.

## 11.2 Tasks

```text
- create ProviderAdapter interface
- implement OpenAI-compatible local adapter
- implement Ollama adapter
- implement LM Studio adapter
- implement provider health check
- implement model discovery
- implement non-streaming generate
- implement streaming generate
- implement provider error mapping
- implement provider timeout
```

## 11.3 Deliverables

Providers:

```text
openai_compatible_local
ollama
lmstudio
```

## 11.4 Exit Criteria

Phase 3 is complete when:

- Novexa can list models from Ollama
- Novexa can send a chat request to Ollama
- Novexa can return an OpenAI-compatible response
- provider errors are normalized

---

# 12. Phase 4: Pipeline Engine

## 12.1 Goal

Route all requests through Pipeline Engine.

## 12.2 Tasks

```text
- create PipelineContext struct
- create PipelineEvent struct
- create Pipeline Engine
- implement direct mode
- implement stabilized mode skeleton
- implement structured mode skeleton
- implement retry structure
- implement pipeline event recording
- connect Gateway → Pipeline → Provider
```

## 12.3 Deliverables

```text
PipelineContext
Pipeline Engine
Pipeline events
Direct Mode
Stabilized Mode skeleton
```

## 12.4 Exit Criteria

Phase 4 is complete when no request calls provider directly from gateway.

All chat requests must pass through Pipeline Engine.

---

# 13. Phase 5: Storage and Telemetry

## 13.1 Goal

Store local metadata for observability.

## 13.2 Tasks

```text
- implement SQLite storage
- create database schema
- create runtime_info table
- create requests table
- create pipeline_events table
- create errors table
- create provider_health table
- create validation_reports table
- create repair_reports table
- implement telemetry writer
- implement recent telemetry API
- implement redaction utility
```

## 13.3 Deliverables

```text
SQLite database
Telemetry Engine
GET /v1/novexa/telemetry/recent
GET /v1/novexa/status
```

## 13.4 Exit Criteria

Phase 5 is complete when dashboard/API can show recent request metadata without logging full prompts by default.

---

# 14. Phase 6: Context and Prompt Engines

## 14.1 Goal

Improve model input quality.

## 14.2 Tasks

```text
- implement message normalization
- implement token estimation
- implement trim strategy
- implement basic context package
- implement context report
- implement prompt package builder
- implement base system prompt
- implement stabilized mode instructions
- implement structured mode instructions
- implement prompt report
- record context/prompt telemetry
```

## 14.3 Deliverables

```text
Context Engine
Prompt Engine
Context Report
Prompt Report
```

## 14.4 Exit Criteria

Phase 6 is complete when stabilized mode sends model-ready messages through Context + Prompt Engines and records what changed.

---

# 15. Phase 7: Validation, Repair, Guard

## 15.1 Goal

Add stability shield.

## 15.2 Tasks

```text
- implement empty response validation
- implement JSON validity validation
- implement JSON extraction from markdown fences
- implement local JSON parse repair
- implement basic repetition detection
- implement regex cleanup for repeated output
- implement structured output guard
- implement anti-loop instruction guard
- implement retry budget guard
- integrate repair with pipeline retry
```

## 15.3 Deliverables

```text
Guard Engine
Validation Engine
Repair Engine
Anti-loop detection
JSON repair
```

## 15.4 Exit Criteria

Phase 7 is complete when:

- invalid JSON can be repaired when safe
- repeated output can be detected
- structured mode returns valid JSON or clear error
- repair events appear in telemetry

---

# 16. Phase 8: Model Profiles

## 16.1 Goal

Apply model-specific settings.

## 16.2 Tasks

```text
- implement profile schema
- create built-in generic-local profile
- create starter profiles
- implement profile loader
- implement provider alias matching
- implement profile validation
- apply profile defaults to provider requests
- apply profile instructions to Prompt Engine
- apply profile guard settings to Guard Engine
```

## 16.3 Built-In Starter Profiles

```text
generic-local
qwen3-8b
qwen2.5-coder-7b
deepseek-r1-8b
llama3.1-8b
gemma3-12b
mistral-small
```

## 16.4 Exit Criteria

Phase 8 is complete when requesting `ollama:qwen3:8b` applies the `qwen3-8b` profile automatically.

---

# 17. Phase 9: CLI and Dashboard

## 17.1 Goal

Create developer control surface.

## 17.2 CLI Tasks

```text
- implement novexa status
- implement novexa doctor
- implement novexa config show
- implement novexa providers
- implement novexa models
- implement novexa logs
- implement novexa benchmark basic
```

## 17.3 Dashboard Tasks

```text
- create dashboard shell
- create overview page
- create providers page
- create recent requests page
- create config page
- create doctor page
- create telemetry page
```

## 17.4 Exit Criteria

Phase 9 is complete when a user can start Novexa, open dashboard, see provider status, recent requests, and diagnostics.

---

# 18. Phase 10: Packaging and Release

## 18.1 Goal

Make Novexa installable and usable.

## 18.2 Tasks

```text
- create release build scripts
- create Dockerfile
- create install instructions
- create example configs
- create quickstart guide
- create troubleshooting guide
- create changelog
- create GitHub release workflow
- test on Windows, macOS, Linux
```

## 18.3 Deliverables

```text
Novexa binary
Docker image
Quickstart docs
Release notes
```

## 18.4 Exit Criteria

Phase 10 is complete when a new user can install Novexa, start it, connect to Ollama, and use it through an OpenAI-compatible client.

---

# 19. Sprint Plan

Recommended sprint structure:

```text
Sprint 0: Setup and docs
Sprint 1: Runtime skeleton
Sprint 2: Gateway API
Sprint 3: Provider adapters
Sprint 4: Pipeline engine
Sprint 5: Telemetry storage
Sprint 6: Context + Prompt
Sprint 7: Validation + Repair
Sprint 8: Model profiles
Sprint 9: CLI + Dashboard
Sprint 10: Packaging + release
```

---

# 20. Sprint 0: Setup and Docs

## Goal

Prepare repository and source-of-truth documents.

## Tasks

```text
- create repo
- add docs
- add README
- add LICENSE
- add initial issue templates
- add development checklist
```

## Output

A clean repo ready for implementation.

---

# 21. Sprint 1: Runtime Skeleton

## Goal

Create runnable runtime.

## Tasks

```text
- setup Go module
- add CLI entrypoint
- add version command
- add start command placeholder
- add config loader placeholder
- add logger
- add graceful shutdown
```

## Acceptance Criteria

```text
novexa version works
novexa start runs without crashing
```

---

# 22. Sprint 2: Gateway API

## Goal

Expose OpenAI-compatible local API.

## Tasks

```text
- add HTTP server
- add /health
- add /v1/models
- add /v1/chat/completions
- add request structs
- add response structs
- add error structs
- add auth middleware
- add request ID middleware
```

## Acceptance Criteria

```text
curl /health returns ok
curl /v1/models returns list
chat endpoint returns placeholder OpenAI-compatible response
```

---

# 23. Sprint 3: Provider Adapters

## Goal

Call local models.

## Tasks

```text
- create ProviderAdapter interface
- implement OpenAI-compatible adapter
- implement Ollama adapter
- implement LM Studio adapter
- add provider health check
- add model discovery
- add generate method
- add stream method
- add error normalization
```

## Acceptance Criteria

```text
Novexa can call Ollama and return actual model output
```

---

# 24. Sprint 4: Pipeline Engine

## Goal

Add disciplined request lifecycle.

## Tasks

```text
- create PipelineContext
- create PipelineEvent
- implement Pipeline Engine
- move provider call into pipeline
- implement direct mode
- implement stabilized mode skeleton
- record pipeline events
```

## Acceptance Criteria

```text
Every request creates pipeline context and events
```

---

# 25. Sprint 5: Telemetry Storage

## Goal

Persist local metadata.

## Tasks

```text
- add SQLite
- create schema
- record request metadata
- record pipeline events
- record provider health
- record errors
- add recent telemetry endpoint
```

## Acceptance Criteria

```text
Recent request metadata visible through API
No full prompt stored by default
```

---

# 26. Sprint 6: Context + Prompt

## Goal

Improve prompts before generation.

## Tasks

```text
- implement token estimation
- implement trim strategy
- implement context package
- implement prompt package
- add stabilized system prompt
- add structured prompt instructions
- add context report
- add prompt report
```

## Acceptance Criteria

```text
Stabilized mode applies context and prompt processing
Telemetry shows context and prompt events
```

---

# 27. Sprint 7: Validation + Repair

## Goal

Improve output quality.

## Tasks

```text
- validate empty response
- validate JSON
- repair JSON from markdown
- repair JSON with trailing prose
- detect repeated lines
- cleanup repeated output
- retry invalid structured output once
```

## Acceptance Criteria

```text
Structured mode returns valid JSON or clear error
Repeated output is detected
Repair metadata is recorded
```

---

# 28. Sprint 8: Model Profiles

## Goal

Apply model-specific behaviour.

## Tasks

```text
- define profile schema
- implement generic profile
- add qwen profile
- add deepseek profile
- add llama profile
- implement profile loader
- match provider model to profile
- apply profile defaults
```

## Acceptance Criteria

```text
Profile is applied automatically based on selected model
```

---

# 29. Sprint 9: CLI + Dashboard

## Goal

Make Novexa observable and usable.

## Tasks

```text
- implement status command
- implement doctor command
- implement providers command
- implement models command
- build dashboard overview
- build providers page
- build recent requests page
- build doctor page
```

## Acceptance Criteria

```text
User can diagnose provider/model problems without reading logs manually
```

---

# 30. Sprint 10: Packaging + Release

## Goal

Prepare first public release.

## Tasks

```text
- build binaries
- create Docker image
- write quickstart
- write troubleshooting
- create release notes
- test on Windows
- test on macOS
- test on Linux
```

## Acceptance Criteria

```text
New user can install and use Novexa with Ollama in under 10 minutes
```

---

# 31. Sprint 11: Managed Thinking Experiment (Post-V1)

## Goal

Turn local model thinking/reasoning into a reliable, observable feature instead of a source of broken output and runaway latency.

## Background

Local models with reasoning support (Qwen3, DeepSeek-R1, Gemma with thinking, etc.) can behave more like frontier models when they are allowed to perform internal "research" before answering. However, naive thinking causes:

- reasoning traces that exhaust `max_tokens`
- reasoning leaking into JSON / tool calls / chat output
- unpredictable latency spikes
- corrupted structured output

Novexa should manage thinking the same way it manages context, prompts, and validation: as part of the intelligence layer around the model.

## Hypothesis

If Novexa can decide **when** thinking is useful, **reserve tokens** for it, and **strip reasoning blocks** from the final response, then a 7B–9B local model with thinking enabled can match frontier-model behaviour on complex tasks without breaking tool-calling or JSON workflows.

## Experiment Tasks

```text
[x] Add thinking_policy section to model profiles
[x] Support reasoning_budget and output_budget split
[x] Detect reasoning content in provider responses
[x] Strip reasoning blocks before validation/repair
[x] Keep thinking disabled by default in lightweight and structured modes
[x] Allow per-request override via novexa.thinking.enabled
[x] Add telemetry event for thinking duration and token reservation
[x] Benchmark direct vs. managed-thinking on complex coding/debugging tasks
[ ] Strip free-form reasoning prose from models that do not use explicit markers
```

## Proposed Profile Schema

```yaml
defaults:
  thinking: false

thinking_policy:
  allowed: true
  default_mode: disabled            # disabled | light | full
  strip_reasoning: true
  reasoning_token_budget: 2048
  enable_when:
    - context_too_large
    - multi_step_task
    - debugging
    - unknown_domain
  disable_when:
    - response_format_json
    - tool_calling
    - one_word_answer
```

## Acceptance Criteria

```text
Managed thinking does not break existing tool-calling or JSON benchmarks
Reasoning traces are never returned to the client by default
Latency overhead from thinking is reported in telemetry
User can enable/disable thinking per request
Benchmark shows improvement on complex tasks when thinking is enabled
```

## Why It Fits Novexa

This aligns with:

- Principle 4: Intelligence Before Models
- Vision: "make local AI more stable, smarter, easier to integrate"
- Tagline: "Run any local model like it's a premium AI"

Thinking is not a model change. It is a usage pattern that Novexa can orchestrate.

---

# 32. Minimum Viable Release

The minimum public V1 release should include:

```text
- novexa start
- OpenAI-compatible /v1/chat/completions
- Ollama provider
- LM Studio provider
- direct mode
- stabilized mode
- basic context processing
- basic prompt optimizer
- JSON validation
- JSON repair
- repetition detection
- SQLite telemetry
- novexa doctor
- dashboard overview
```

---

# 32. MVP Cutline

If development takes too long, cut these from first release:

```text
- plugin execution
- advanced dashboard pages
- full model profile pack
- benchmark command
- session persistence
- memory engine
- markdown validation
- streaming repair
```

Do not cut:

```text
- OpenAI compatibility
- Ollama support
- Pipeline Engine
- telemetry metadata
- JSON validation/repair
- doctor command
```

---

# 33. Release Versioning

Suggested versions:

```text
0.1.0 - first working gateway
0.2.0 - provider adapters stable
0.3.0 - pipeline + telemetry
0.4.0 - context + prompt engines
0.5.0 - validation + repair
0.6.0 - model profiles
0.7.0 - CLI + dashboard
0.8.0 - packaged alpha
1.0.0 - stable V1
```

---

# 34. Definition of Done

A feature is done only when:

```text
- implementation completed
- unit tests added
- integration tests added where applicable
- telemetry events added
- error handling added
- config documented
- docs updated
- dashboard/CLI visibility considered
```

---

# 35. Testing Strategy

## Unit Tests

Required for:

```text
config loader
provider adapters
pipeline context
context strategies
prompt builder
validation
repair
profile loader
redaction
```

## Integration Tests

Required for:

```text
gateway to pipeline
pipeline to provider
structured output repair
provider error mapping
telemetry recording
```

## Manual Tests

Required for:

```text
Ollama local
LM Studio local
OpenAI-compatible local server
streaming output
dashboard
CLI doctor
```

---

# 36. Performance Targets

V1 target overhead excluding provider generation:

```text
Gateway overhead: <5ms
Pipeline overhead: <10ms
Context basic processing: <20ms
Prompt processing: <10ms
Validation: <10ms
Telemetry write: <10ms
```

Total runtime overhead target:

```text
<50ms for normal non-heavy requests
```

Heavy context compression may exceed this and should be reported separately.

---

# 37. Quality Targets

Novexa should improve:

```text
- structured output validity
- repeated output detection
- provider error clarity
- context fit reliability
- local model integration simplicity
```

Novexa should not claim:

```text
- perfect factual correctness
- hallucination elimination
- cloud-level reasoning
- guaranteed output correctness
```

---

# 38. Documentation Deliverables

Before first public release, create:

```text
README.md
docs/quickstart.md
docs/installation.md
docs/configuration.md
docs/providers/ollama.md
docs/providers/lmstudio.md
docs/openai-compatible-clients.md
docs/troubleshooting.md
docs/model-profiles.md
docs/telemetry-and-privacy.md
```

---

# 39. Examples

Create examples:

```text
examples/curl
examples/python-openai
examples/node-openai
examples/continue-config
examples/cline-config
examples/open-webui-config
```

Each example should include:

- setup
- request
- expected output
- troubleshooting notes

---

# 40. Risks

## 40.1 Scope Creep

Risk:

```text
Building agents, memory, marketplace, and cloud before gateway is stable.
```

Mitigation:

```text
Follow MVP cutline.
```

## 40.2 Too Much Plugin Work Early

Risk:

```text
Overbuilding plugin system before runtime has users.
```

Mitigation:

```text
Implement manifest and hook placeholders first.
```

## 40.3 Dashboard Overdesign

Risk:

```text
Spending too much time on UI polish before runtime works.
```

Mitigation:

```text
Build simple functional dashboard first.
```

## 40.4 Provider API Changes

Risk:

```text
Ollama/LM Studio API behaviour changes.
```

Mitigation:

```text
Keep provider adapters isolated and well-tested.
```

## 40.5 Privacy Mistakes

Risk:

```text
Accidentally logging prompts or secrets.
```

Mitigation:

```text
Redaction tests and prompt logging disabled by default.
```

---

# 41. Coding Agent Instructions

When using an AI coding agent, instruct it:

```text
Follow the docs as source of truth.

Do not invent new architecture.

Implement one sprint at a time.

Do not skip tests.

Do not add cloud providers in V1.

Do not bypass Pipeline Engine.

Do not store prompts by default.

Keep provider adapters thin.

Keep runtime modular monolith.

Ask for confirmation before changing architecture documents.
```

---

# 42. Suggested First Agent Prompt

```text
You are implementing Novexa Runtime.

Read all files in docs/ first.

Follow the architecture exactly.

Start with Sprint 1 only:
- initialize Go module
- create cmd/novexa entrypoint
- implement novexa version
- implement novexa start placeholder
- implement config loader placeholder
- implement logger
- implement graceful shutdown

Do not implement providers yet.
Do not implement dashboard yet.
Do not add cloud providers.
Do not change architecture without asking.

After implementation, provide:
- files created
- commands to run
- tests added
- next recommended sprint
```

---

# 43. Final Roadmap Statement

Novexa must be built like infrastructure, not like a weekend chatbot.

The roadmap prioritizes a working local runtime first, then intelligence, then observability, then extensibility.

A disciplined roadmap is what prevents Novexa from becoming a beautiful architecture document with no usable product.