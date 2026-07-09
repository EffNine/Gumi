
# Novexa Runtime - Agent Handoff Summary

Version: 1.0  
Status: Planning Pack Summary  
Purpose: Quick handoff document for AI coding agents before implementation

---

# 1. Project Identity

## Product Name

Novexa

## Category

Local-first AI Runtime Platform

## One-Line Description

Novexa is an intelligence runtime that sits between AI applications and local inference engines to make local AI models more stable, reliable, observable, and production-ready.

## Tagline

Run any local model like it's a premium AI.

---

# 2. What Novexa Is

Novexa is:

- a local AI runtime
- an OpenAI-compatible gateway
- a reliability layer for local models
- a modular monolith
- a developer infrastructure product
- a context, prompt, validation, repair, telemetry, and provider abstraction layer

Novexa improves the experience of using models through:

- Ollama
- LM Studio
- OpenAI-compatible local servers
- future providers such as vLLM, SGLang, llama.cpp, and TGI

---

# 3. What Novexa Is Not

Novexa is not:

- a chatbot
- a new AI model
- an inference engine
- a cloud AI gateway in V1
- a billing platform
- a ChatGPT competitor
- a replacement for Ollama or LM Studio
- a hosted SaaS product in V1

Novexa does not compete with local model providers.

Novexa improves them.

---

# 4. Core V1 Strategy

V1 must focus on local models only.

No cloud billing.

No hosted inference.

No external AI API dependency.

The first product should be:

```text
Novexa Runtime
```

A user should be able to run:

```bash
novexa start
```

Then connect any OpenAI-compatible app to:

```text
http://localhost:8787/v1
```

And use local models with better stability, better diagnostics, cleaner output, and runtime protection.

---

# 5. Core Architecture

Novexa sits between applications and local inference providers.

```text
Application
    ↓
Novexa Runtime
    ↓
Ollama / LM Studio / OpenAI-compatible Local Server
    ↓
Local Model
```

Internal runtime:

```text
Novexa Runtime
├── Gateway Engine
├── Pipeline Engine
├── Workspace Engine
├── Config Engine
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
├── Plugin Engine
├── CLI
└── Dashboard
```

---

# 6. Most Important Rule

Every request must pass through the Pipeline Engine.

Invalid:

```text
Gateway Engine → Provider Engine
```

Valid:

```text
Gateway Engine → Pipeline Engine → Provider Engine
```

This is what makes Novexa more than a proxy.

---

# 7. Pipeline Summary

Default stabilized pipeline:

```text
Request Received
    ↓
Normalize Request
    ↓
Resolve Workspace
    ↓
Resolve Config
    ↓
Resolve Session
    ↓
Resolve Model Profile
    ↓
Prepare Context
    ↓
Retrieve Memory if Enabled
    ↓
Build Prompt
    ↓
Apply Guardrails
    ↓
Select Provider
    ↓
Call Provider
    ↓
Normalize Response
    ↓
Validate Response
    ↓
Repair if Needed
    ↓
Record Telemetry
    ↓
Return Response
```

---

# 8. Runtime Modes

Novexa supports these modes:

```text
direct
stabilized
structured
agent
```

V1 implements:

```text
direct
stabilized
structured
```

Agent mode is reserved for future versions.

## Direct Mode

Minimal processing.

Used for benchmarking and raw provider testing.

## Stabilized Mode

Default mode.

Uses context preparation, prompt optimization, validation, repair, and telemetry.

## Structured Mode

Used for JSON/schema-heavy tasks.

Applies strict prompt rules, JSON validation, repair, and retry.

---

# 9. API Contract

Default API URL:

```text
http://localhost:8787/v1
```

Default dashboard URL:

```text
http://localhost:8788
```

Required endpoints:

```http
GET  /health
GET  /v1/models
POST /v1/chat/completions
```

Novexa diagnostic endpoints:

```http
GET  /v1/novexa/status
GET  /v1/novexa/providers
GET  /v1/novexa/config
GET  /v1/novexa/telemetry/recent
POST /v1/novexa/doctor
```

The API must be OpenAI-compatible wherever possible.

Users should be able to set:

```bash
export OPENAI_BASE_URL=http://localhost:8787/v1
export OPENAI_API_KEY=novexa-local
```

---

# 10. Provider Support

V1 providers:

```text
ollama
lmstudio
openai_compatible_local
```

Future providers:

```text
llama.cpp
vLLM
SGLang
TGI
KoboldCpp
LocalAI
```

Provider adapters must be thin translation layers.

They must not contain:

- prompt optimization
- business logic
- memory logic
- validation logic
- repair logic

---

# 11. Configuration Summary

Novexa should work with zero config.

Default:

```bash
novexa start
```

Advanced users can create:

```text
novexa.yaml
```

Default local config principles:

```yaml
runtime:
  mode: stabilized
  host: 127.0.0.1
  port: 8787

dashboard:
  enabled: true
  host: 127.0.0.1
  port: 8788

auth:
  mode: local
  local_key: novexa-local

provider:
  default: ollama

telemetry:
  local: true
  external: false
  log_prompts: false
  log_responses: false
```

Safe defaults:

- bind to localhost only
- no external telemetry
- no full prompt logging
- no full response logging
- no cloud provider by default
- no public dashboard by default

---

# 12. Model Profiles

Model Profiles are a key Novexa feature.

They define model-specific behaviour:

- defaults
- temperature
- top_p
- repeat penalty
- context strategy
- prompt style
- structured output reliability
- anti-loop settings
- known weaknesses
- preferred tasks

Starter built-in profiles:

```text
generic-local
qwen3-8b
qwen2.5-coder-7b
deepseek-r1-8b
llama3.1-8b
gemma3-12b
mistral-small
```

If profile is missing, use:

```text
generic-local
```

Missing profile must not crash runtime.

---

# 13. Stability Shield

Novexa improves reliability through:

```text
Guard Engine
Validation Engine
Repair Engine
```

V1 should support:

- empty prompt guard
- context overflow guard
- structured output guard
- anti-loop guard
- retry budget guard
- empty response validation
- JSON validation
- JSON schema validation
- basic Markdown validation
- repetition detection
- local JSON repair
- regex cleanup
- retry generation

V1 must not claim perfect hallucination elimination.

It may detect hallucination risk signals and warn.

---

# 14. Telemetry and Storage

V1 uses:

```text
SQLite
```

Default storage path:

```text
~/.novexa/novexa.db
```

Telemetry stores metadata only by default.

Required tables:

```text
runtime_info
requests
pipeline_events
provider_events
provider_health
validation_reports
repair_reports
errors
logs
sessions
```

Privacy rule:

```text
Do not store full prompt or full response by default.
```

External telemetry:

```text
disabled by default
```

---

# 15. CLI Summary

V1 CLI commands:

```bash
novexa start
novexa stop
novexa restart
novexa status
novexa doctor
novexa config show
novexa providers
novexa models
novexa benchmark
novexa logs
novexa version
```

Most important commands:

```bash
novexa start
novexa doctor
novexa status
```

Doctor must explain:

- what happened
- why it matters
- how to fix it

---

# 16. Dashboard Summary

Default:

```text
http://127.0.0.1:8788
```

Dashboard pages:

```text
Overview
Requests
Providers
Models
Profiles
Telemetry
Config
Doctor
Logs
```

Dashboard must not show full prompts or full responses unless explicitly enabled.

Dashboard must redact secrets.

Dashboard must bind to localhost by default.

---

# 17. Plugin System Summary

Plugin system is important long term, but should not block V1.

V1 should design:

- manifest schema
- hook names
- permissions model
- built-in plugin registry
- hook placeholders

Full third-party plugin execution can come later.

Core plugin idea:

```text
Small stable core.
Optional extensions around it.
```

Hooks include:

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
before_repair
after_repair
before_response
after_response
on_error
```

---

# 18. Recommended Tech Stack

Runtime:

```text
Go
```

API:

```text
HTTP + Server-Sent Events
OpenAI-compatible /v1
```

Storage:

```text
SQLite
```

Config:

```text
YAML
```

Dashboard:

```text
Next.js / React
```

Architecture:

```text
Modular monolith
```

Avoid microservices in V1.

---

# 19. Target Repository Structure

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
├── profiles/
├── plugins/
├── examples/
└── README.md
```

---

# 20. Sprint Order

Implementation must follow this order:

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

Do not implement multiple sprints at once unless explicitly requested.

---

# 21. MVP Cutline

If development takes too long, cut:

```text
plugin execution
advanced dashboard pages
full model profile pack
benchmark command
session persistence
memory engine
markdown validation
streaming repair
```

Do not cut:

```text
OpenAI compatibility
Ollama support
Pipeline Engine
telemetry metadata
JSON validation/repair
doctor command
```

---

# 22. Hard Rules for Coding Agents

Coding agents must obey:

```text
1. Do not add cloud providers in V1.
2. Do not add billing.
3. Do not add user accounts.
4. Do not build marketplace features.
5. Do not implement microservices.
6. Do not bypass Pipeline Engine.
7. Do not put business logic inside providers.
8. Do not store full prompts by default.
9. Do not store full responses by default.
10. Do not send external telemetry.
11. Do not bind dashboard publicly by default.
12. Do not skip tests.
13. Do not rename architecture terms.
14. Do not change architecture without approval.
```

---

# 23. First Agent Prompt

Use this prompt to start implementation:

```text
You are implementing Novexa Runtime.

Read all files in docs/ first.

Follow the architecture exactly.

Start with Sprint 1 only.

Sprint 1 goal:
Create runnable runtime skeleton.

Tasks:
- initialize Go module under runtime/
- create cmd/novexa entrypoint
- implement novexa version
- implement novexa start placeholder
- implement config loader placeholder
- implement logger
- implement graceful shutdown

Do not implement providers yet.
Do not implement dashboard yet.
Do not implement Pipeline Engine yet.
Do not implement cloud providers.
Do not implement storage yet.
Do not change architecture documents.

Required output:
- files created
- files modified
- commands to run
- tests added
- what works now
- next recommended sprint
```

---

# 24. Full Document Pack

The complete planning pack should contain:

```text
docs/00-vision-and-positioning.md
docs/01-core-principles.md
docs/02-runtime-architecture.md
docs/03-engine-specifications.md
docs/04-api-specification.md
docs/05-configuration-specification.md
docs/06-provider-adapter-specification.md
docs/07-pipeline-specification.md
docs/08-context-and-prompt-engine-specification.md
docs/09-validation-repair-guard-specification.md
docs/10-model-profile-specification.md
docs/11-plugin-system-specification.md
docs/12-cli-and-dashboard-specification.md
docs/13-storage-and-telemetry-specification.md
docs/14-implementation-roadmap.md
docs/15-agent-build-prompt.md
docs/16-agent-handoff-summary.md
```

---

# 25. Final Handoff Statement

Novexa should be built like infrastructure, not like a weekend chatbot.

The first milestone is not to build every feature.

The first milestone is to create a clean, local-first, OpenAI-compatible runtime that can talk to local models and expose what happened during each request.

A proxy forwards requests.

Novexa manages the lifecycle around the request.

That lifecycle is the product.
