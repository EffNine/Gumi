# Novexa Agent Build Prompt

Version: 1.0  
Status: Ready  
Scope: Master implementation prompt for AI coding agents

---

# 1. Purpose

This document is the master prompt for any AI coding agent that will implement Novexa Runtime.

The coding agent must treat the files in `docs/` as the source of truth.

The agent must not invent new architecture unless explicitly asked.

The agent must implement Novexa one sprint at a time.

---

# 2. Project Context

You are implementing **Novexa Runtime**.

Novexa is a local-first AI runtime platform that sits between AI applications and local inference engines.

It exposes an OpenAI-compatible API and improves local AI reliability through:

- provider abstraction
- pipeline orchestration
- context management
- prompt optimization
- validation
- repair
- anti-loop detection
- telemetry
- model profiles
- CLI diagnostics
- local dashboard

Novexa is not:

- a chatbot
- a new AI model
- a hosted cloud service
- a billing platform
- a ChatGPT competitor
- an inference engine
- a replacement for Ollama or LM Studio

Novexa improves the layer around local models.

---

# 3. Mandatory Documentation Reading

Before implementing anything, read these files:

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
```

You must follow these documents exactly unless the user explicitly instructs otherwise.

---

# 4. Hard Rules

You must follow these rules:

```text
1. Do not add cloud providers in V1.
2. Do not add billing.
3. Do not add hosted user accounts.
4. Do not add team accounts.
5. Do not build marketplace features in V1.
6. Do not bypass Pipeline Engine.
7. Do not let Gateway Engine call providers directly after Pipeline exists.
8. Do not put prompt optimization inside provider adapters.
9. Do not put business logic inside providers.
10. Do not store full prompts by default.
11. Do not store full responses by default.
12. Do not send external telemetry.
13. Do not bind dashboard publicly by default.
14. Do not make plugin system required for basic runtime.
15. Do not implement microservices.
16. Do not change architecture without asking.
17. Do not implement multiple sprints at once unless requested.
18. Do not skip tests.
19. Do not remove documentation.
20. Do not rename core architecture terms.
```

---

# 5. Core Architecture Terms

Use these terms consistently:

| Term | Meaning |
|---|---|
| Runtime | The whole Novexa system |
| Engine | A major runtime component |
| Provider | Connection to Ollama, LM Studio, vLLM, etc. |
| Adapter | Provider-specific translation layer |
| Pipeline | Request lifecycle flow |
| Pipeline Context | Shared request state |
| Profile | Model-specific runtime preset |
| Session | Conversation state |
| Workspace | Project/app boundary |
| Guardrail | Reliability/safety control |
| Hook | Plugin extension point |
| Plugin | Optional extension |

Do not introduce alternative names unless requested.

---

# 6. Technical Stack

Use the recommended stack unless the user asks otherwise:

```text
Runtime: Go
API: HTTP + Server-Sent Events
API compatibility: OpenAI-compatible /v1
Storage: SQLite
Config: YAML
Dashboard: Next.js / React
Architecture: Modular monolith
```

The runtime should be implemented as a Go monorepo module first.

---

# 7. Repository Structure

Use this target structure:

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

If the current repo already has a structure, adapt carefully without breaking the architecture.

---

# 8. Implementation Method

Work in small controlled increments.

For each sprint:

1. State what sprint you are implementing.
2. List files you will create or modify.
3. Implement only that sprint.
4. Add tests where appropriate.
5. Run formatting.
6. Run tests.
7. Summarize what changed.
8. Explain how to run it.
9. Identify next sprint.

Do not silently jump ahead.

---

# 9. Sprint Order

Follow this order:

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

# 10. Current Task Template

When the user says:

```text
Start Sprint N
```

You must:

```text
1. Read docs/14-implementation-roadmap.md.
2. Find Sprint N.
3. Implement only Sprint N.
4. Do not implement future sprint work unless needed for compilation as a stub.
5. Keep future components as interfaces or placeholders.
```

---

# 11. Sprint 1 Build Prompt

Use this when starting actual implementation.

```text
You are implementing Novexa Runtime.

Read all files in docs/ first.

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

# 12. Sprint 2 Build Prompt

```text
You are implementing Novexa Runtime.

Read docs/04-api-specification.md and docs/14-implementation-roadmap.md.

Start Sprint 2 only.

Sprint 2 goal:
Expose basic OpenAI-compatible local API.

Tasks:
- implement HTTP server
- implement GET /health
- implement GET /v1/models placeholder
- implement POST /v1/chat/completions placeholder
- implement OpenAI-compatible request structs
- implement OpenAI-compatible response structs
- implement standard error format
- implement request ID middleware
- implement auth middleware with local key

Do not implement real provider calls yet.
Do not bypass future Pipeline design.
For chat completions, return a placeholder OpenAI-compatible response for now.
Do not implement cloud providers.

Required output:
- files created
- files modified
- commands to run
- tests added
- curl examples
- next recommended sprint
```

---

# 13. Sprint 3 Build Prompt

```text
You are implementing Novexa Runtime.

Read docs/06-provider-adapter-specification.md.

Start Sprint 3 only.

Sprint 3 goal:
Connect Novexa to local providers.

Tasks:
- create ProviderAdapter interface
- implement OpenAI-compatible local adapter
- implement Ollama adapter
- implement LM Studio adapter
- implement provider health check
- implement model discovery
- implement non-streaming generate
- implement streaming generate if practical
- implement provider error mapping
- implement provider timeout

Do not implement cloud providers.
Do not put prompt optimization in providers.
Do not put business logic in providers.
Provider adapters must stay thin.

Required output:
- provider files created
- health check behaviour
- model list behaviour
- cURL examples
- tests added
- known limitations
- next recommended sprint
```

---

# 14. Sprint 4 Build Prompt

```text
You are implementing Novexa Runtime.

Read docs/07-pipeline-specification.md and docs/03-engine-specifications.md.

Start Sprint 4 only.

Sprint 4 goal:
Route all requests through Pipeline Engine.

Tasks:
- create PipelineContext struct
- create PipelineEvent struct
- create Pipeline Engine
- implement Direct Mode
- implement Stabilized Mode skeleton
- implement Structured Mode skeleton
- implement retry structure
- implement pipeline event recording
- connect Gateway → Pipeline → Provider

Hard rule:
After this sprint, Gateway Engine must not call Provider Engine directly.

Do not implement Context Engine fully yet.
Do not implement Validation/Repair fully yet.
Use placeholders where needed.

Required output:
- pipeline files created
- updated request flow
- tests added
- evidence that gateway uses pipeline
- next recommended sprint
```

---

# 15. Sprint 5 Build Prompt

```text
You are implementing Novexa Runtime.

Read docs/13-storage-and-telemetry-specification.md.

Start Sprint 5 only.

Sprint 5 goal:
Persist local metadata for observability.

Tasks:
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

Privacy rules:
- Do not store full prompts by default.
- Do not store full responses by default.
- Do not send external telemetry.
- Redact secrets.

Required output:
- storage files created
- schema created
- telemetry API examples
- tests added
- privacy behaviour confirmed
- next recommended sprint
```

---

# 16. Sprint 6 Build Prompt

```text
You are implementing Novexa Runtime.

Read docs/08-context-and-prompt-engine-specification.md.

Start Sprint 6 only.

Sprint 6 goal:
Improve model input quality.

Tasks:
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

Do not implement advanced memory.
Do not implement cloud verification.
Prompt Engine must preserve user intent.

Required output:
- context engine files
- prompt engine files
- tests added
- examples of transformed prompt package
- telemetry events
- next recommended sprint
```

---

# 17. Sprint 7 Build Prompt

```text
You are implementing Novexa Runtime.

Read docs/09-validation-repair-guard-specification.md.

Start Sprint 7 only.

Sprint 7 goal:
Add stability shield.

Tasks:
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

Rules:
- Repair Engine must not invent missing facts.
- Validation Engine reports issues, it does not mutate output directly.
- Do not add cloud repair.
- Do not retry infinitely.

Required output:
- guard/validation/repair files
- tests added
- invalid JSON repair example
- repeated output detection example
- next recommended sprint
```

---

# 18. Sprint 8 Build Prompt

```text
You are implementing Novexa Runtime.

Read docs/10-model-profile-specification.md.

Start Sprint 8 only.

Sprint 8 goal:
Apply model-specific behaviour.

Tasks:
- define profile schema
- implement generic profile
- add starter profiles
- implement profile loader
- implement provider alias matching
- implement profile validation
- apply profile defaults to provider requests
- apply profile instructions to Prompt Engine
- apply profile guard settings to Guard Engine

Starter profiles:
- generic-local
- qwen3-8b
- qwen2.5-coder-7b
- deepseek-r1-8b
- llama3.1-8b
- gemma3-12b
- mistral-small

Rules:
- Missing profile must not crash runtime.
- Use generic fallback profile.
- Do not exaggerate model capabilities.

Required output:
- profiles created
- loader implemented
- tests added
- profile matching example
- next recommended sprint
```

---

# 19. Sprint 9 Build Prompt

```text
You are implementing Novexa Runtime.

Read docs/12-cli-and-dashboard-specification.md.

Start Sprint 9 only.

Sprint 9 goal:
Make Novexa observable and usable.

CLI tasks:
- implement novexa status
- implement novexa doctor
- implement novexa config show
- implement novexa providers
- implement novexa models
- implement novexa logs
- implement novexa benchmark basic

Dashboard tasks:
- create dashboard shell
- create overview page
- create providers page
- create recent requests page
- create config page
- create doctor page
- create telemetry page

Rules:
- Dashboard must be local-only by default.
- Do not expose prompts by default.
- Redact secrets.
- Keep UI functional before beautiful.

Required output:
- CLI commands implemented
- dashboard pages created
- screenshots or page descriptions
- tests added where practical
- next recommended sprint
```

---

# 20. Sprint 10 Build Prompt

```text
You are implementing Novexa Runtime.

Read docs/14-implementation-roadmap.md.

Start Sprint 10 only.

Sprint 10 goal:
Prepare first public release.

Tasks:
- create release build scripts
- create Dockerfile
- create install instructions
- create example configs
- create quickstart guide
- create troubleshooting guide
- create changelog
- create GitHub release workflow
- test on Windows, macOS, Linux where possible

Rules:
- Do not add new features in release sprint.
- Fix packaging and docs only unless critical bug.
- Ensure first-run experience works.

Required output:
- release files created
- installation instructions
- quickstart
- build commands
- known limitations
- release checklist
```

---

# 21. Code Quality Rules

The coding agent must:

```text
- prefer clear code over clever code
- keep packages small
- keep interfaces explicit
- add errors with suggestions
- write tests for core logic
- format code
- avoid global hidden state
- avoid hardcoded user paths
- avoid leaking secrets
- keep provider adapters thin
- keep pipeline observable
```

---

# 22. Error Quality Rules

Every user-facing error should include:

```text
code
message
engine
retryable
suggestion
request_id where applicable
```

Bad:

```text
provider failed
```

Good:

```text
Ollama is not reachable at http://localhost:11434.
Start Ollama or update providers.ollama.url in novexa.yaml.
```

---

# 23. Privacy Rules

The coding agent must enforce:

```text
telemetry.external = false by default
telemetry.log_prompts = false by default
telemetry.log_responses = false by default
dashboard.host = 127.0.0.1 by default
runtime.host = 127.0.0.1 by default
secrets redacted everywhere
```

---

# 24. Testing Rules

Every sprint should include tests where possible.

Minimum test focus:

```text
config loading
request parsing
provider mapping
pipeline context
error formatting
redaction
JSON validation
JSON repair
profile loading
telemetry writes
```

Do not claim tests pass unless they were run.

If tests cannot be run, say why.

---

# 25. Completion Response Format

After each sprint, respond with:

```text
Implemented:
- ...

Files created:
- ...

Files modified:
- ...

How to run:
- ...

Tests:
- ...

Known limitations:
- ...

Next recommended sprint:
- ...
```

---

# 26. Architecture Change Protocol

If implementation reveals a needed architecture change:

1. Stop.
2. Explain the issue.
3. Propose the smallest change.
4. Identify docs affected.
5. Ask for approval.
6. Only then modify architecture.

Do not silently change architecture.

---

# 27. Final Agent Instruction

Build Novexa like infrastructure.

Start small.

Keep it local.

Keep it observable.

Keep it modular.

Do not chase extra features.

The goal is not to build the biggest AI system.

The goal is to make local AI feel stable enough for real applications.