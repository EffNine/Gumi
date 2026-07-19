# Gumi Implementation Roadmap

Version: 1.1  
Status: Updated post-v0.2.0-alpha (tagged 2026-07-14)  
Scope: Development roadmap for Gumi Runtime V1

---

# 1. Purpose

This document defines the implementation roadmap for Gumi Runtime V1.

The goal is to guide development in a disciplined order so Gumi becomes:

- usable early
- testable early
- stable before complex
- extensible without overengineering
- useful as a local AI runtime before adding advanced features

This roadmap is designed for solo development or AI-agent-assisted development.

---

# 2. Development Philosophy

Gumi should be built in layers.

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

Gumi must first work as a basic local OpenAI-compatible gateway.

Then it becomes intelligent.

Then it becomes extensible.

---

# 3. V1 Product Goal

V1 should allow a developer to:

```bash
gumi start
```

Then connect an OpenAI-compatible app to:

```text
http://localhost:8787/v1
```

And use local models through:

- Ollama
- LM Studio
- OpenAI-compatible local server

With Gumi providing:

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
- distributed runtime
- vector database
- enterprise RBAC
- remote telemetry
- full agent runtime with arbitrary tool execution (a lightweight agent mode
  with step budgets, tool-call loop detection, and JSON validation is already
  shipped in v0.2.0-alpha — see §7a)
- general-purpose tool orchestration framework (tool-call loop detection and
  tool-call JSON validation are shipped; a richer orchestration layer with
  custom tool schemas and parallel execution is deferred to post-V1)

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
gumi/
├── docs/
├── runtime/
│   ├── cmd/
│   │   └── gumi/
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
│   ├── src/                  # Vite + React (App.tsx, main.tsx, styles.css)
│   ├── dist/                 # built dashboard (embedded into binary at release)
│   ├── index.html
│   ├── package.json
│   ├── vite.config.ts
│   └── tsconfig.json
│
├── profiles/                 # 15 starter profiles shipped in v0.2.0-alpha
│   ├── generic-local.yaml
│   ├── qwen3-8b.yaml
│   ├── qwen2.5-coder-7b.yaml
│   ├── qwen3.5-2b.yaml
│   ├── qwen3.5-9b.yaml
│   ├── deepseek-r1-8b.yaml
│   ├── llama3.1-8b.yaml
│   ├── llama3.2-3b.yaml
│   ├── gemma3-1b.yaml
│   ├── gemma3-4b.yaml
│   ├── gemma3-12b.yaml
│   ├── gemma-4-e4b.yaml
│   ├── mistral-small.yaml
│   ├── ornith-1.0-9b-q4-km.yaml
│   └── essentialai-rnj-1.yaml
│
├── benchmark/                # Go-based benchmark harness (runner, scorer, report)
├── benchmarks/               # benchmark scripts + reports archive
├── scripts/                  # release, install, benchmark, profile-doctor shell scripts
├── plugins/                  # reserved (not implemented in V1)
├── docs/
│   └── examples/             # client integration examples
│       ├── python-openai/
│       ├── curl/
│       └── README.md
│
├── Dockerfile
├── Makefile
├── go.work                   # Go workspace: runtime/ + benchmark/
├── gumi.example.yaml
├── CHANGELOG.md
├── CONTRIBUTING.md
├── LICENSE
└── README.md
```

---

# 7. Development Phases

V1 development is divided into 10 phases.

```text
Phase 0: Repository and Documentation          ✅ shipped
Phase 1: Runtime Skeleton                      ✅ shipped
Phase 2: OpenAI-Compatible Gateway             ✅ shipped
Phase 3: Provider Adapters                     ✅ shipped
Phase 4: Pipeline Engine                       ✅ shipped
Phase 5: Storage and Telemetry                 ✅ shipped
Phase 6: Context and Prompt Engines            ✅ shipped
Phase 7: Validation, Repair, Guard             ✅ shipped
Phase 8: Model Profiles                        ✅ shipped (15 starter profiles)
Phase 9: CLI and Dashboard                     ✅ shipped (11 dashboard pages)
Phase 10: Packaging and Release                ✅ shipped (v0.2.0-alpha, 2026-07-14)
```

All 10 original phases are complete as of `v0.2.0-alpha`. Three additional
phases (11–13) were added during development and are also shipped. See §7a
for the full shipped-feature inventory and §8b onwards for per-phase status.

---

# 7a. Shipped Features (v0.2.0-alpha)

Tagged `v1.0.0-rc1` on 2026-07-20 after 13 sprints (Sprint 0 → Sprint 13).
This section lists what is already complete, grouped by subsystem. Source of
truth: `CHANGELOG.md`.

## Core runtime

- **OpenAI-compatible gateway** — `GET /health`, `GET /v1/models`,
  `POST /v1/chat/completions` (streaming + non-streaming), auth + request-ID
  middleware, standard error format.
- **Pipeline engine** — `PipelineContext`, `PipelineEvent`, direct / stabilized /
  structured / agent modes, retry structure, telemetry event recording. All
  requests route through the pipeline; none call providers directly.
- **Provider adapters** — Ollama, LM Studio, OpenAI-compatible local server.
  Health check, model discovery, non-streaming + streaming generate, error
  normalization, timeout handling.
- **Context + prompt engines** — message normalization, token estimation,
  sliding-window trim, stabilized/structured system prompts, context + prompt
  reports.
- **SQLite storage + telemetry** — `runtime_info`, `requests`, `pipeline_events`,
  `errors`, `provider_health`, `validation_reports`, `repair_reports` tables.
  Recent telemetry API, redaction utility (prompts not stored by default).

## Reliability layer

- **Validation engine** — empty-response, JSON validity, JSON extraction from
  any language-tagged code fence (```json, ```python, bare ```), structured
  output guard.
- **Repair engine** — local JSON parse repair, trailing-prose cleanup,
  pipeline retry integration with `response_format`-aware strip logic.
- **Guard engine** — repetition detection (skips JSON/structured output to
  avoid false positives), regex cleanup for repeated output, anti-loop
  instruction guard, retry-budget guard.
- **Reliability fixes (P0–P3)** — exponential backoff for LM Studio
  model-loading 502s (~50% → 0% errors), validation telemetry records issue
  code + message + location, repetition false positives eliminated (113 → 0),
  JSON repair handles any language-tagged fence.

## Agent mode

- **Agent mode** — dedicated runtime mode for agentic coding loops with
  step-budget enforcement, tool-call loop detection, tool-call JSON validation,
  and sliding-window context compaction that trims old messages.
- **Agentic coding router** — automatic per-step model selection by difficulty
  (trivial → novel) and task type (fix, refactor, feature, test, review, docs,
  search, plan) using structural heuristics. Opt-in, agent-mode only.
  Spec: `docs/specs/19-agentic-coding-router-specification.md`.
- **Memory engine** — zero-VRAM persistent memory (SQLite). `FactStore`,
  `EpisodeStore`, `ModelFitStore`, `InjectionEngine`, `ExtractionEngine`.
  Shared across models, survives session boundaries. Feeds router feedback
  loop. API endpoints + CLI commands.
  Spec: `docs/specs/20-memory-engine-specification.md`.
- **LM Studio model management** — load, unload, list models via LM Studio v1
  REST API. Per-model config overrides (context length, flash attention,
  KV-cache offload, eval batch size, num experts). Auto-unload on model
  switch. CLI commands: `gumi lmstudio status|load|unload|models`.
  Spec: `docs/specs/06-provider-adapter-specification.md` §22.

## Intelligence layer

- **Instruction-following assist** — auto-detects 14 constraint types
  (sentence count, word limits, forbidden words, end-with, JSON, line count,
  bullets, capital-start, min chars, no commas, no markdown, sections, no
  rhyme, min words) from user prompts. Injects numbered reminders. Post-
  generation validation with automatic retry (max 2).
- **Managed thinking orchestration** — controlled reasoning for local
  reasoning models. Token budget split (output + reasoning), reasoning
  stripping, automatic disable for JSON / tool-calling workflows. Per-request
  override via `gumi.thinking.enabled`. Telemetry for thinking duration +
  token reservation.

## Model profiles

- **15 starter profiles shipped** — `generic-local`, `qwen3-8b`,
  `qwen2.5-coder-7b`, `qwen3.5-2b`, `qwen3.5-9b`, `deepseek-r1-8b`,
  `llama3.1-8b`, `llama3.2-3b`, `gemma3-1b`, `gemma3-4b`, `gemma3-12b`,
  `gemma-4-e4b`, `mistral-small`, `ornith-1.0-9b-q4-km`, `essentialai-rnj-1`.
  Profile schema, loader, provider-alias matching, validation, per-profile
  guard + instruction-assist settings.

## CLI

- `gumi version` — build metadata (Version, Commit, BuildDate via ldflags).
- `gumi start` — runtime launch with graceful shutdown.
- `gumi status` — runtime + provider status.
- `gumi doctor` — diagnostic checks.
- `gumi config show` — effective config.
- `gumi providers` / `gumi models` — provider + model listing.
- `gumi logs` — recent log access.
- `gumi benchmark` — benchmark runner.
- `gumi lmstudio status|load|unload|models` — LM Studio model management.
- `gumi memory status|facts|clear` — memory engine inspection.
- All commands support `--json` for machine-readable output.

## Dashboard

- **11 pages** — overview, providers, recent requests, config, doctor,
  telemetry, memory, LM Studio model management, logs, model load/unload,
  config editor.
- **Real-time log streaming** via Server-Sent Events (SSE).
- **Model load/unload** controls (LM Studio management integration).
- **Config editor** — view + edit runtime config from the dashboard.
- **Dark mode**.
- Built with Vite + React (`dashboard/src/`), embedded into the Go binary at
  release time.

## Packaging + release

- Cross-platform release archives (macOS arm64/amd64, Linux amd64/arm64,
  Windows amd64) with SHA256 checksums.
- Multi-stage `Dockerfile` (Node build for dashboard, Go build for runtime,
  Alpine runtime image with non-root user + `/data` volume).
- `Makefile` targets: `test`, `vet`, `dashboard`, `build`, `run`, `release`,
  `clean`, `check-release`.
- GitHub Actions CI (gofmt, go test, go vet, npm ci/build) + release workflow
  (draft pre-release on `v*` tags).
- `gumi.example.yaml`, `docs/installation.md`, `docs/quickstart.md`,
  `docs/troubleshooting.md`, `docs/release-checklist.md`.

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

**Status: ✅ shipped (Sprint 0).**

---

# 9. Phase 1: Runtime Skeleton

## 9.1 Goal

Create runnable Gumi binary.

## 9.2 Tasks

```text
- initialize Go module
- create cmd/gumi entrypoint
- create basic CLI parser
- implement gumi version
- implement gumi start placeholder
- implement config loader stub
- implement logging stub
- implement graceful shutdown
```

## 9.3 Deliverables

```bash
gumi version
gumi start
```

## 9.4 Exit Criteria

Phase 1 is complete when:

```bash
go run ./cmd/gumi version
go run ./cmd/gumi start
```

both work.

Runtime does not need AI yet.

**Status: ✅ shipped (Sprint 1).**

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

**Status: ✅ shipped (Sprint 2).**

---

# 11. Phase 3: Provider Adapters

## 11.1 Goal

Connect Gumi to local providers.

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

- Gumi can list models from Ollama
- Gumi can send a chat request to Ollama
- Gumi can return an OpenAI-compatible response
- provider errors are normalized

**Status: ✅ shipped (Sprint 3).**

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

**Status: ✅ shipped (Sprint 4).**

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
GET /v1/gumi/telemetry/recent
GET /v1/gumi/status
```

## 13.4 Exit Criteria

Phase 5 is complete when dashboard/API can show recent request metadata without logging full prompts by default.

**Status: ✅ shipped (Sprint 5).**

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

**Status: ✅ shipped (Sprint 6).**

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

**Status: ✅ shipped (Sprint 7).** Repetition detection was later refined
(Sprint 12) to skip JSON/structured output, eliminating 113 false positives.

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

**Status: ✅ shipped (Sprint 8).** 15 starter profiles now exist (see §7a).

---

# 17. Phase 9: CLI and Dashboard

## 17.1 Goal

Create developer control surface.

## 17.2 CLI Tasks

```text
- implement gumi status
- implement gumi doctor
- implement gumi config show
- implement gumi providers
- implement gumi models
- implement gumi logs
- implement gumi benchmark basic
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

Phase 9 is complete when a user can start Gumi, open dashboard, see provider status, recent requests, and diagnostics.

**Status: ✅ shipped (Sprint 9).** Dashboard now has 11 pages including
real-time log SSE, model load/unload, config editor, and dark mode (see §7a).

---

# 18. Phase 10: Packaging and Release

## 18.1 Goal

Make Gumi installable and usable.

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
Gumi binary
Docker image
Quickstart docs
Release notes
```

## 18.4 Exit Criteria

Phase 10 is complete when a new user can install Gumi, start it, connect to Ollama, and use it through an OpenAI-compatible client.

**Status: ✅ shipped (Sprint 10).** Tagged `v0.2.0-alpha` on 2026-07-14.
Cross-platform archives, Docker image, Makefile, GitHub Actions CI + release
workflows, quickstart/installation/troubleshooting docs all delivered.

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
Sprint 11: Agentic Coding Router ✅ (structural V1 complete)
Sprint 12: Engine fine-tuning + Router integration ✅ (5 engine fixes, telemetry)
Sprint 13: Agentic Coding Memory Engine (zero-VRAM persistence) ✅
Sprint 14: LM Studio Model Management (load/unload/configure models via API) ✅
```

---

# 19b. Phase 11: Agentic Coding Router ✅

Status: **Implemented** (Sprint 11-12, 2026-07-12 — 2026-07-13)

## Goal

Let coding agents automatically use the right model for each step — tiny/fast
for trivial typo fixes, large/capable for complex multi-file refactors.

## Tasks

- ✅ Add `routing` section to config schema (agentic_coding mode only, opt-in)
- ✅ Implement `CodingTaskClassifier` (structural heuristics: text length, file
  count, traceback presence, keywords, step count, retry count)
- ✅ Implement `CodingModelRegistry` (from profiles + coding_strength rating)
- ✅ Implement `CodingRuleEngine` (first-match, difficulty-based rules)
- ✅ Implement 5 difficulty levels (trivial, simple, moderate, complex, novel)
- ✅ Integrate into agent mode at resolveProviderAndProfile
- ✅ Add CodingRoute + CodingTaskProfile to Pipeline Context
- ✅ Implement agent-state-aware escalation skeleton (step count, retry)
- ✅ Record routing telemetry
- ✅ `buildAvailableModelSet()` helper on Engine

## Deliverables

```text
runtime/internal/router/              # New package (classifier, registry, engine, telemetry)
CodingTaskClassifier                  # Structural-only classification
CodingModelRegistry                   # Indexes profiles by coding capability
CodingRuleEngine                      # 11 default rules, first-match
RoutingConfig                         # Opt-in, disabled by default
RoutingExtensions                     # Per-request hints (difficulty, task type, preferred model)
Pipeline integration                  # resolveProviderAndProfile() routing path
```

## Exit Criteria

Phase 11 is complete when an agent session routes a "fix typo" step to a
tiny/fast model and a "implement multi-file feature" step to a large/capable
model, with routing decisions visible in telemetry and the router re-evaluating
each step of the agent loop.

See the full specification at:
[docs/specs/19-agentic-coding-router-specification.md](./19-agentic-coding-router-specification.md)

---

# 19c. Phase 12: Agentic Coding Memory Engine

Status: **Implemented** ✅ (2026-07-13)

## Goal

Give coding agents persistent, cross-model memory that survives model swaps and
session boundaries — using zero VRAM.

## Tasks

```text
[x] create runtime/internal/memory/ package
[x] implement FactStore (SQLite KV, ephemeral + file tiers)
  [x] SetFact(key, value, ttl)
  [x] GetFact(key) → value
  [x] SearchFacts(query) → ranked facts
  [x] DeleteFact(key)
  [x] GarbageCollectExpired()
  [x] Hot cache (Go map for frequently accessed facts)
  [x] Confidence scoring + deduplication
[x] implement ModelFitStore (per-model performance tracking)
  [x] RecordOutcome(modelID, difficulty, taskType, success, latency)
  [x] GetBestModel(difficulty, taskType) → modelID
  [x] GetModelProfile(modelID) → stats
  [x] Exponential weighted moving average for latency/retries
  [x] integrate with CodingRuleEngine for preference strategy selection
  [x] implement Phase 3 self-tuning (rule overrides, model boost/demote, exploration)
[x] implement EpisodeStore (compressed step history)
  [x] StoreEpisode(episode)
  [x] GetRecentEpisodes(sessionID, n) → episodes
  [x] SummarizeEpisodes(sessionID) → compressed string
[x] implement InjectionEngine (budget-aware memory injection)
  [x] SelectRelevantFacts(requestText) → ranked facts
  [x] FormatInjection(facts, episodes, fitData, budget) → formatted block
  [x] Stay within configurable token budget (default 1200)
  [x] Prioritize: model fit → recent episodes → relevant facts
[x] implement ExtractionEngine (post-generation memory harvesting)
  [x] ExtractFactsFromResponse(response) → facts (structural patterns)
  [x] UpdateModelFit (via RecordOutcome)
  [x] StoreEpisode (via StoreEpisode)
[x] add GET /v1/gumi/memory/facts API
[x] add GET /v1/gumi/memory/model-fit API
[x] add POST /v1/gumi/memory/clear API
[x] add GET /v1/gumi/memory/status API
[x] add memory section to config schema (enabled, db_path, max_facts, etc.)
[x] add CLI commands: gumi memory status, gumi memory facts, gumi memory clear
[x] wire into pipeline: prepareMemory() pre-generation, extractMemory() post-generation
[x] add memory events to telemetry (memory_injected, facts_extracted, model_fit_updated)
[x] add memory dashboard page (facts count, model fit table, self-tuning snapshot)
```

## Deliverables

```text
[x] runtime/internal/memory/ package
[x] FactStore, ModelFitStore, EpisodeStore
[x] InjectionEngine, ExtractionEngine
[x] Memory API endpoints (facts, model-fit, clear, status, self-tuning)
[x] CLI commands (status, facts, clear)
[x] Config section (MemoryConfig)
[x] Dashboard page (facts count, model fit table, self-tuning snapshot via GET /v1/gumi/self-tuning)
[x] Zero VRAM (SQLite + Go map, no GPU)
```

## Exit Criteria

Status: **10/10 complete** ✅

- [x] After an agent session, facts are persisted and retrievable via API
- [x] A follow-up session can retrieve relevant facts from previous sessions
- [x] Model fit data is recorded and queryable via API/CLI (router integration + self-tuning complete)
- [x] Memory injection stays within configurable token budget (default 1200)
- [x] Dashboard shows memory statistics (facts count, model fit table, self-tuning tab)
- [x] `gumi memory status` works
- [x] `gumi memory facts` works
- [x] `gumi memory clear` works
- [x] No GPU memory is used (SQLite + Go map, verified by design)
- [x] Pipeline hooks in stabilized, agent, and streaming agent modes

See the full specification at:
[docs/specs/20-memory-engine-specification.md](./20-memory-engine-specification.md)

---

# 19d. Phase 13: LM Studio Model Management

Status: **Implemented** ✅

## Goal

Let Gumi dynamically load, unload, and configure models on a remote LM Studio
instance via its v1 REST API (`/api/v1/models/load`, `/api/v1/models/unload`).
Combined with the Agentic Coding Router, Gumi can load the right model with
the right config for each task and unload it when done — keeping only the
needed model in VRAM.

## Background

LM Studio exposes a management API beyond the OpenAI-compatible endpoints:

| Endpoint | Purpose |
|----------|---------|
| `POST /api/v1/models/load` | Load a model with `context_length`, `flash_attention`, `offload_kv_cache_to_gpu`, `eval_batch_size`, `num_experts` |
| `POST /api/v1/models/unload` | Unload a model by `instance_id` |
| `GET /api/v1/models` | List available models on disk |
| `POST /api/v1/models/download` | Download models on-demand |

## Current State

Gumi's `LMStudioAdapter` only uses the OpenAI-compatible `/v1/chat/completions`
endpoint. It controls inference parameters (temperature, top_p, max_tokens,
tools, etc.) but cannot manage model lifecycle — loading, unloading, or
configuring GPU/context settings.

## Tasks

```text
[x] add LoadModel(ctx, modelID, config) method to LMStudioAdapter
[x] add UnloadModel(ctx, instanceID) method to LMStudioAdapter
[x] add GetLoadedModel(ctx) method to LMStudioAdapter
[x] add ModelManagementConfig to LM Studio provider settings
[x] integrate into resolveProviderAndProfile
[x] add model lifecycle events to telemetry
[x] add model management dashboard view
[x] add CLI commands: gumi lmstudio status, load, unload, models
[x] extend with ModelManager interface (optional, LMStudio-only)
```

## Deliverables

```text
[x] LMStudioAdapter extended with LoadModel/UnloadModel (ModelManager interface)
[x] ModelManagementConfig in provider settings (LMStudioMgmtConfig)
[x] Pipeline integration for auto-load/unload on route change (applyModelManagement)
[x] Telemetry events for model lifecycle (model_load_started/succeeded/failed)
[x] Dashboard model management view
[x] CLI commands for manual model control (status, load, unload, models)
```

## Exit Criteria

Status: **7/7 complete** ✅

- [x] Gumi can load a model on a remote LM Studio with custom config
- [x] Router selects a different model → Gumi unloads current, loads target
- [x] Per-model config is applied (per-model overrides → mgmt defaults → zero values)
- [x] Model lifecycle events appear in telemetry and pipeline events
- [x] `gumi lmstudio status` shows loaded model and available models
- [x] Manual load/unload via CLI works
- [x] Dashboard model management view (shipped with dashboard overhaul; see Memory/Models pages)

See the LM Studio REST API docs at:
[https://lmstudio.ai/docs/developer/rest](https://lmstudio.ai/docs/developer/rest)

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
gumi version works
gumi start runs without crashing
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
Gumi can call Ollama and return actual model output
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

Make Gumi observable and usable.

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
New user can install and use Gumi with Ollama in under 10 minutes
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

Gumi should manage thinking the same way it manages context, prompts, and validation: as part of the intelligence layer around the model.

## Hypothesis

If Gumi can decide **when** thinking is useful, **reserve tokens** for it, and **strip reasoning blocks** from the final response, then a 7B–9B local model with thinking enabled can match frontier-model behaviour on complex tasks without breaking tool-calling or JSON workflows.

## Experiment Tasks

```text
[x] Add thinking_policy section to model profiles
[x] Support reasoning_budget and output_budget split
[x] Detect reasoning content in provider responses
[x] Strip reasoning blocks before validation/repair
[x] Keep thinking disabled by default in lightweight and structured modes
[x] Allow per-request override via gumi.thinking.enabled
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

## Why It Fits Gumi

This aligns with:

- Principle 4: Intelligence Before Models
- Vision: "make local AI more stable, smarter, easier to integrate"
- Tagline: "Run any local model like it's a premium AI"

Thinking is not a model change. It is a usage pattern that Gumi can orchestrate.

---

# 32. Minimum Viable Release

The minimum public V1 release should include:

```text
- gumi start
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
- gumi doctor
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

Gumi should improve:

```text
- structured output validity
- repeated output detection
- provider error clarity
- context fit reliability
- local model integration simplicity
```

Gumi should not claim:

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

Examples directory created under `docs/examples/` with starter snippets:

```text
docs/examples/curl/
docs/examples/python-openai/
```

Each example includes setup, request, expected output, and troubleshooting notes. Additional client configs (Continue, Cline, Open WebUI) can be added incrementally.

**Status: partially shipped.** Core cURL and Python examples delivered.

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
You are implementing Gumi Runtime.

Read all files in docs/ first.

Follow the architecture exactly.

Start with Sprint 1 only:
- initialize Go module
- create cmd/gumi entrypoint
- implement gumi version
- implement gumi start placeholder
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

Gumi must be built like infrastructure, not like a weekend chatbot.

The roadmap prioritizes a working local runtime first, then intelligence, then observability, then extensibility.

A disciplined roadmap is what prevents Gumi from becoming a beautiful architecture document with no usable product.