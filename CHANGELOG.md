# Changelog

All notable changes to Gumi are documented in this file.

## 0.2.0-alpha — 2026-07-15

### Added

- **Agent mode** — Dedicated runtime mode for agentic coding loops with step-budget enforcement, tool-call loop detection, tool-call JSON validation, and sliding-window context compaction.
- **Agentic Coding Router** — Automatic per-step model selection by difficulty (trivial → novel) and task type (fix, refactor, feature, test, review, docs, search, plan). Opt-in, agent-mode only.
- **Memory Engine** — Zero-VRAM persistent memory for coding agents. Facts, episode summaries, and model-fit data stored in SQLite. Shared across all models, survives session boundaries.
- **LM Studio Model Management** — Load, unload, and list models via LM Studio's v1 REST API. Per-model config overrides for context length, flash attention, KV-cache offload, and more.
- **Instruction-Following Assist** — Auto-detects 14 constraint types from user prompts and injects explicit numbered reminders. Post-generation validation with automatic retry.
- **Managed Thinking** — Controlled reasoning for local reasoning models with token budget split, reasoning stripping, and automatic disable for JSON/tool-calling workflows.
- **Deep tool schema validation** — Recursive argument-type, enum, nested-object, and array-item checks for tool calls, plus schema violations injected into retry prompts.
- **Integration test harness** — 8 deterministic tests covering the full gateway → pipeline → provider → telemetry chain.
- **Documentation site** (`docs-site/`) — Static docs site served at `/docs-site/` with Quickstart, Integrations, Architecture, Benchmarks, and Changelog pages.
- **Anime poker-face logo** — SVG logo mark and favicon replacing the emoji across the docs site.

### Changed

- Dashboard dependency versions pinned from `latest` to exact versions.
- Project version aligned to `0.2.0-alpha` across Makefile, runtime, and dashboard package.json.

### Fixed

- **P0: Retry backoff** — Exponential backoff for LM Studio model-loading 502s (~50% → 0% HTTP errors).
- **P1: Validation telemetry** — Validation failures now record issue code, message, and location (was empty `{}`).
- **P2: JSON repetition** — Repetition detection skips JSON/structured output (113 false positives → 0).
- **P3: Non-JSON fence repair** — JSON repair handles any language-tagged code fence (```python, ```javascript, etc.).
- **`fmtInt` recursion bug** — Removed recursive call causing stack overflow on negative values.
- **Unused import** — Removed unused `strings` import from `router/engine.go`.

## 0.2.0-alpha — 2026-07-14

### Added

- **Agent mode** — Dedicated runtime mode for agentic coding loops with step-budget enforcement, tool-call loop detection, tool-call JSON validation, and sliding-window context compaction.
- **Agentic Coding Router** — Automatic per-step model selection by difficulty (trivial → novel) and task type (fix, refactor, feature, test, review, docs, search, plan). Opt-in, agent-mode only.
- **Memory Engine** — Zero-VRAM persistent memory for coding agents. Facts, episode summaries, and model-fit data stored in SQLite. Shared across all models, survives session boundaries.
- **LM Studio Model Management** — Load, unload, and list models via LM Studio's v1 REST API. Per-model config overrides for context length, flash attention, KV-cache offload, and more.
## 0.1.0-alpha — Spec Review Fix Sprint (WP7)

### Fixed

- **`docs/specs/04-api-specification.md`** §6 — Added memory engine routes
  (`GET/POST /v1/gumi/memory/facts`, `GET /v1/gumi/memory/model-fit`,
  `POST /v1/gumi/memory/clear`, `GET /v1/gumi/memory/status`) and LM Studio
  model management routes to the V1 endpoint listing. §11.1 — Removed "reserved
  for future use" for agent mode; documented agent mode as shipped with router
  + memory + LM Studio model management integration.
- **`docs/specs/05-configuration-specification.md`** §7.3 — Marked agent mode as
  shipped (was "reserved for future versions"). §6 + §12.6 — Replaced the old
  `engines.memory` two-field shape (`enabled` + `mode`) with the actual top-level
  `memory:` block (14 fields matching `MemoryConfig` in `config.go`). Added
  `routing:` block documentation (§12.7) with classifier escalation thresholds
  (`retries`, `steps`, `repetitions`) and coding rules.
- **`docs/specs/06-provider-adapter-specification.md`** — Added §22 LM Studio
  Model Management documenting the `ModelManager` interface, `/api/v1/models/load`,
  `/api/v1/models/unload`, `/api/v1/models` endpoints, per-model config
  resolution, and pipeline integration. Renumbered subsequent sections (§23-27).
- **`docs/specs/12-cli-and-dashboard-specification.md`** §5 — Added `gumi
  lmstudio` (status, load, unload, models) and `gumi memory` (status, facts,
  clear) to the V1 command set.
- **`docs/specs/19-agentic-coding-router-specification.md`** — Marked Phase 2
  (agent-state awareness) as shipped (Sprint 13). Documented `repetitions`
  escalation — detects repeating tool-call patterns (same function name +
  arguments) via `applyEscalation` in `classifier.go`.
- **`docs/specs/20-memory-engine-specification.md`** — Marked Phase 1 + CLI as
  shipped (Sprint 12). Added `POST /v1/gumi/memory/facts` to §6.3 (agent-driven
  fact storage, Open Question #2 resolved). Updated implementation plan file
  paths to match shipped code (`memory.go`, `schema.go`, `gateway/memory.go`,
  `cli/memory.go`).

## 0.1.0-alpha — Sprint 12: Agentic Coding Memory Engine

### Added

- **Memory Engine** (`runtime/internal/memory/`): Zero-VRAM persistent memory
  for agentic coding agents. Shared across all models, survives session boundaries.
  - `FactStore` — SQLite-backed key-value store with TTL, LRU eviction,
    hot cache (Go map), confidence scoring, deduplication
  - `EpisodeStore` — Compressed step histories with outcomes, model tracking,
    session-scoped with automatic summarization (~30× compression)
  - `ModelFitStore` — Per-model performance tracking per difficulty/task type.
    Records success rate, latency, retries. Feeds router feedback loop.
    Exponential weighted moving average for latency/retries.
  - `InjectionEngine` — `SelectRelevantFacts()` with relevance scoring
    (key/value match, confidence, access frequency). `FormatInjection()` formats
    facts + episode summaries + model fit data within token budget (default 1200).
  - `ExtractionEngine` — `ExtractFactsFromResponse()` uses structural patterns
    (file paths, error messages, import statements) — no model inference needed.
    Confidence scoring per extraction pattern.
- **MemoryConfig** (`config.go`): Full config section with `enabled: false` (opt-in),
  `engine`, `db_path`, `max_facts`, `max_episodes_per_session`, `injection_budget_tokens`,
  `min_confidence`, `max_injected_facts`, `extract_enabled`, `track_model_fit`,
  `model_fit_decay`, etc. Safe defaults for all settings.
- **Pipeline integration** (`engine.go`): `prepareMemory()` called after
  `resolveProviderAndProfile`, injects memory as prepended system message.
  `extractMemory()` called after provider generate, extracts facts + updates
  model fit + stores episode. Wired into `runStabilized`, `runAgent`,
  `runStreamAgent`.
- **API endpoints** (`gateway/memory.go`):
  - `GET /v1/gumi/memory/facts` — list/search stored facts
  - `GET /v1/gumi/memory/model-fit` — model performance data
  - `POST /v1/gumi/memory/clear` — clear all memory
  - `GET /v1/gumi/memory/status` — memory engine status
- **CLI commands** (`cli/memory.go`):
  - `gumi memory status` — show database path, fact count, model fit entries
  - `gumi memory facts [search]` — list or search facts
  - `gumi memory clear --force` — reset all memory
  - All commands support `--json` for machine-readable output
- **Per-request override** (`api/chat.go`): `MemoryExtension` with
  `enable_injection`, `max_injected_facts`, `reset_session`
- **Pipeline context** (`context.go`): `InjectedMemory` string and `MemoryFacts`
  refs for telemetry tracking

### Changed

- **Pipeline Engine** (`engine.go`): `Engine` struct now has `memoryEngine` field.
  `New()` initializes memory if `cfg.Memory.Enabled` is true.
  `MemoryEngine()` accessor added for gateway API.

## 0.1.0-alpha — Sprint 13: LM Studio Model Management

### Added

- **LM Studio v1 REST API model management** (`runtime/internal/provider/lmstudio_mgmt.go`):
  - `LoadModel(ctx, modelID, config)` — `POST /api/v1/models/load` with
    configurable `context_length`, `flash_attention`, `offload_kv_cache_to_gpu`,
    `eval_batch_size`, `num_experts`. Returns `instance_id` and applied config.
  - `UnloadModel(ctx, instanceID)` — `POST /api/v1/models/unload`
  - `ListAvailableModels(ctx)` — `GET /api/v1/models` lists all models on disk
  - `BuildPerModelConfig(modelID)` — returns per-model config overrides
  - `ModelManager` interface — optional interface for provider adapters
  - Per-model config resolution: CLI flags → management defaults → per-model
    overrides from config file → final merged config sent to LM Studio
- **Model management config** (`config.go`): `LMStudioMgmtConfig` with
  `enabled`, `default_context_length`, `default_flash_attention`,
  `default_offload_kv_cache`, `default_eval_batch_size`, `auto_unload`,
  and `model_config` map for per-model overrides.
- **Pipeline integration** (`engine.go` `applyModelManagement`): After
  `resolveProviderAndProfile` selects a provider+model, if the provider is
  LM Studio with management enabled, loads the model before generation.
  Telemetry events: `model_load_started`, `model_load_succeeded`,
  `model_load_failed`. Falls through silently if management is not configured.
- **CLI commands** (`cli/lmstudio.go`):
  - `gumi lmstudio status [--url <base>]` — shows available models on disk
  - `gumi lmstudio load <model> [--context-length N] [--flash-attention] [--offload-kv-cache]` — loads a model
  - `gumi lmstudio unload <instance-id>` — unloads a model
  - `gumi lmstudio models [--url <base>]` — lists all models on disk
  - All commands support `--json` for machine-readable output
  - URL resolution: flag → config file → `http://localhost:1234/v1`

### Changed

- **LM Studio adapter** (`lmstudio.go`): Added `mgmtConfig *config.LMStudioMgmtConfig`
  and `loadedInstanceID string` fields to track loaded model state. Added
  `LoadedModelID()` accessor.
- **Pipeline engine** (`engine.go`): `resolveProviderAndProfile` calls
  `applyModelManagement` after both routing path and default resolution path.

## 0.1.0-alpha — Sprint 12b: Agentic Coding Router + Engine Fine-Tuning

### Added

- **Agentic Coding Router** (`runtime/internal/router/`):
  - `CodingTaskClassifier` — structural heuristics classify coding tasks into 5
    difficulty levels (trivial, simple, moderate, complex, novel) and 8 task
    types (fix, refactor, feature, test, review, docs, search, plan). No AI
    inference — purely message length, file count, traceback presence,
    keywords, code block size, step count, retry count.
  - `CodingModelRegistry` — indexes available models by coding capability from
    YAML profiles. Supports preference strategies: fastest, best_coding,
    best_combo, largest_context, explicit.
  - `CodingRuleEngine` — first-match rule engine with 11 built-in default rules
    covering all difficulty + task_type combinations.
  - `RoutingTelemetry` — records every routing decision as pipeline events.
  - Full specification at `docs/specs/19-agentic-coding-router-specification.md`.
- **Routing config** (`config.go`): `RoutingConfig` section — `enabled: bool`
  (opt-in, disabled by default), `mode: string`.
- **Routing API extensions** (`api/chat.go`): `RoutingExtensions` with
  `hint_difficulty`, `hint_task_type`, `preferred_provider`, `preferred_model`,
  `min_context` for per-request overrides.
- **Router integration into pipeline** (`engine.go`):
  - `resolveProviderAndProfile()` now checks if routing is enabled + agent mode
    before default resolution.
  - `buildAvailableModelSet()` helper builds `"provider:model"` key map from
    all registered providers.
  - Router fields (`codingRouter`, `codingRegistry`, `codingClassifier`) added
    to Engine struct and initialized in `New()`.

### Changed

- **Tool shim refined** (`engine.go` `isWeakToolCalling`): Removed "medium"
  from weak-ToolCalling check. Only "weak", "none", "unknown" trigger the
  tool-calling shim — saves tokens on mid-tier models that handle tools fine.
- **Agent thinking policy** (`engine.go` `applyThinkingPolicy`): Agent mode no
  longer unconditionally disables thinking. Now calls `applyThinkingPolicy`
  with `AgentMode=true` — profiles with reasoning models opt-in via
  `thinking_policy` rules.
- **Context compaction** (`engine.go` `checkAgentContextCompaction`): Upgraded
  from a hint-only "please summarize" injection to a sliding-window trim that
  actually removes old messages when estimated tokens exceed threshold.
- **Lightweight mode anti-loop** (`engine.go` `applyLightweightGuard`): New
  guard detects repeated tool calls (3×+) and long conversations (20+ turns),
  injects loop-break hints into the system prompt.

### Fixed

- **`fmtInt` recursion bug**: Removed recursive call in telemetry formatter
  that caused stack overflow on negative values.
- **Unused import**: Removed unused `strings` import from `router/engine.go`.

## 0.1.0-alpha — Sprint 12: Reliability Fixes, Telemetry & JSON Repair

### Fixed

- **P0: Intermittent 502 errors from LM Studio model loading** (404 errors in
  DB). Root cause: LM Studio returns `"Failed to load model ... Engine protocol
  startup was aborted"` when swapping models in/out of memory. The old retry
  logic used a flat 500ms backoff (too short for model loading, which takes
  3-10 seconds) and unconditionally stripped `response_format` on every retry
  (degrading JSON quality even when the format wasn't the problem).
  - `generateOnce` now uses exponential backoff: 3s → 6s for model-loading
    errors, 2s → 4s for other retryable errors.
  - `response_format` is only stripped when the error message specifically
    mentions `response_format`, `json_schema`, or `json_object`.
  - Two new helper functions: `isModelLoadingError` and `isResponseFormatError`
    classify the error to choose the right retry strategy.
- **P1: Validation failure details were stored as `{}`** in the errors table.
  The `validation_reports` and `repair_reports` tables existed in the schema but
  were never written to.
  - Added `telemetry.RecordValidationReport` — writes validation outcome
    (passed, severity, issues with code/message/location) to
    `validation_reports` table.
  - Added `telemetry.RecordRepairReport` — writes repair outcome (attempted,
    strategy, success, changes, retry requested) to `repair_reports` table.
  - `VALIDATION_FAILED` errors now include a human-readable issue summary in
    `details_json` (e.g., `"REPETITION at choices[0].message.content: assistant
    response contains repeated lines or sentences"`).
  - Validation reports are recorded at all outcomes: pass, pass-after-repair,
    pass-after-retry, and final failure.
- **P2: Repetition false positives on JSON output** (113 validation failures in
  stabilized mode). The `hasRepetition` function flagged any line appearing 3+
  times, but JSON structural elements (`}`, `"type":`, repeated keys across
  array objects) legitimately repeat.
  - `hasRepetition` now accepts the response format and runtime mode, and
    skips repetition detection for `json_object`, `json_schema`, `structured`
    mode, and any content that parses as valid JSON.
  - Plain-text repetition detection is unchanged (still catches actual loops).
  - 2 new tests: `TestValidateRepetitionSkipsJSON`,
    `TestValidateRepetitionSkipsStructuredMode`.
- **P3: JSON repair only handled ```json fences, not other language tags**.
  Models like Essential AI RNJ-1 wrap JSON in ```python code blocks. The
  `ExtractJSONCandidate` and `requiresJSON` functions in
  `runtime/internal/validation/engine.go` only detected ```json fences, so
  JSON validation/repair never triggered for python-fenced (or other
  language-tagged) JSON.
  - `ExtractJSONCandidate` now strips any language-tagged code fence
    (```python, ```javascript, bare ```, etc.), not just ```json.
  - `requiresJSON` now detects any ```-fenced content as a potential JSON
    candidate, triggering validation and repair.
  - `checkJSON` in `runtime/internal/instruction/engine.go` similarly handles
    any language-tagged fence.
  - 4 new tests: `TestExtractJSONCandidatePythonFence`,
    `TestExtractJSONCandidateBareFence`, `TestRequiresJSONDetectsPythonFence`,
    `TestRepairJSONExtractsFromPythonFence`.
  - Result: RNJ-1 stabilized JSON went from 0/3 → **3/3 (100%)**; Ornith 9B
    direct JSON went from 0/3 → **3/3 (100%)**.

### Added

- **New model profile: `essentialai-rnj-1`** (`profiles/essentialai-rnj-1.yaml`).
  Essential AI RNJ-1 reasoning model with `reasoning_content` field support.
  Conservative settings until capabilities are validated.
- **Managed thinking benchmark** (`scripts/benchmark-managed-thinking.sh`):
  Tests 3 configurations (direct with thinking on, Gumi with thinking on,
  Gumi with thinking off) across debugging, planning, and JSON prompts.
  Detects reasoning leaks and measures latency impact of thinking on/off.
- **Terminal-Bench comparison**: Full agent harness evaluation using
  terminus-2 agent on 5 pinned terminal-bench-core tasks. Compares direct
  LM Studio vs Gumi stabilized mode. Measures task resolution, episode
  counts, parser warnings, and per-task timing.

### Benchmark Results (Post-Sprint-12, all fixes applied)

| Metric | Pre-Sprint-12 | Post-Sprint-12 |
|---|---|---|
| HTTP/curl errors | ~50% failure rate | **0** |
| Repetition false positives | 113 validation failures | **0** |
| Validation reports stored | 0 | **15+ per benchmark run** |
| Repair reports stored | 0 | **7+ per benchmark run** |
| Error details_json for VALIDATION_FAILED | `{}` | Issue code + message + location |
| RNJ-1 stabilized JSON valid | 0/3 (0%) | **3/3 (100%)** |
| Ornith 9B direct JSON valid | 0/3 (0%) | **3/3 (100%)** |
| Terminal-Bench JSON parser warnings | 11 | **1** (−91%) |
| Terminal-Bench agent timeouts | 2/5 | **1/5** (−50%) |

### Models Benchmarked (Sprint 12)

| Model | Local Model | Agentic | Managed Thinking | Terminal-Bench | Profile Doctor |
|---|---|---|---|---|---|
| Ornith 9B (q4_k_m) | ✅ post-fix | ✅ post-fix | ✅ (archive) | ✅ | Good baseline (caveat) |
| Qwen 3.5 9B | ✅ post-fix | ✅ post-fix | ✅ (archive) | — | Good baseline (caveat) |
| Essential AI RNJ-1 | ✅ post-fix | ✅ | ✅ | — | Good baseline (caveat) |
| Qwen3 1.7B | — | — | ✅ | — | — |
| Qwen3.5 2B | — | — | ✅ | — | — |

## 0.1.0-alpha — Sprint 11: Instruction Assist & Benchmarks

### Added

- **Instruction-Following Assist Engine** (`runtime/internal/instruction/`):
  - Auto-detects 14 constraint types from user prompts (sentences, words, lines,
    bullets, forbidden words, end-with, capital-start, JSON, min chars, min
    words, no commas, no markdown, sections, no rhyme).
  - Injects explicit numbered reminders into the system prompt.
  - Post-generation validation with automatic retry (max 2) on constraint
    violations.
  - 26 unit tests covering extraction, validation, and retry hints.
- `instruction_assist` profile field (`prompt.instruction_assist`) — enabled for
  Ornith 9B and Qwen 3.5 9B profiles.
- **Benchmark suite restructure**: raw data goes to `~/.gumi/benchmarks/`,
  summary reports stay in `benchmarks/reports/`.
- **Comprehensive benchmark report** (`benchmarks/reports/SUMMARY-20260712.md`):
  Agentic Coding + Local Model benchmarks for Ornith 9B and Qwen 3.5 9B.
- **`benchmarks/README.md`** documenting methodology and quick start.

### Benchmark Results (Ornith 9B)

| Metric | Direct | Gumi | Improvement |
|---|---|---|---|
| JSON Validity (Agentic) | 0% | **100%** | +100% |
| JSON Validity (Local) | 0% | **100%** | +100% |
| Instruction Following | 78% | **100%** | +22% |
| Tool Call Accuracy | 100% | 100% | maintained |
| Latency p50 (JSON) | 2,949ms | 352ms | 8.4× faster |

### Changed

- All benchmark scripts now output raw data to `~/.gumi/benchmarks/` (outside
  repo) and summary reports to `benchmarks/reports/`.
- Archived 60+ historical benchmark files to `benchmarks/archive/`.

## 0.1.0-alpha — Sprint 10: Packaging and Release

### Added

- Cross-platform release build scripts (`scripts/build-release.sh`,
  `scripts/check-release.sh`, `scripts/install.sh`).
- Top-level `Makefile` with targets: `test`, `vet`, `dashboard`, `build`, `run`,
  `release`, `clean`, `check-release`.
- Multi-stage `Dockerfile` that builds the dashboard in Node, the runtime in
  Go, and ships a small Alpine image with a non-root user and a `/data` volume.
- `.dockerignore` to keep the image small and avoid shipping secrets or build
  artifacts.
- GitHub Actions CI workflow (`.github/workflows/ci.yml`) for gofmt, go test,
  go vet, npm ci/build, and `git diff --check`.
- GitHub Actions release workflow (`.github/workflows/release.yml`) triggered
  on `v*` tags, building all supported targets, packaging archives, generating
  SHA256 checksums, and uploading draft pre-release artifacts.
- Release archive layout including binary, `dashboard/dist`, starter profiles,
  `gumi.example.yaml`, `README.md`, `LICENSE`, and `CHANGELOG.md`.
- `gumi.example.yaml` documenting the planned local-first configuration
  format for the alpha. YAML config parsing is not implemented yet.
- Installation guide (`docs/installation.md`) covering source builds, release
  archives, Docker, macOS, Linux, Windows, starting, dashboard, client setup,
  and uninstalling.
- Quickstart guide (`docs/quickstart.md`) with Ollama setup, model pull,
  startup, dashboard, chat completions, and `gumi doctor`.
- Troubleshooting guide (`docs/troubleshooting.md`) for common issues such as
  port conflicts, Ollama availability, missing models, dashboard build errors,
  SQLite permissions, invalid API keys, provider timeouts, missing profiles,
  streaming, and macOS quarantine.
- Release checklist (`docs/release-checklist.md`) for verifying builds,
  archives, Docker, and security before publishing.
- Build-time version metadata package (`runtime/internal/version`) with
  `Version`, `Commit`, and `BuildDate` injected via ldflags. `gumi version`
  shows metadata in release builds while preserving the one-line default output
  for development builds.

### Release targets

- macOS arm64
- macOS amd64
- Linux amd64
- Linux arm64
- Windows amd64

### Known limitations

- YAML configuration parsing is not implemented in the alpha.
- CLI `stop` and `restart` commands are not implemented.
- Streaming chat completions are not supported.
- Cross-platform release artifacts are cross-compiled but not necessarily
  manually run on every target.
- A Dockerfile is included for convenience but the Docker image has not been
  manually built or tested in this environment.

## Previous sprints

- Sprint 9: CLI and dashboard (status, doctor, config show, providers, models,
  benchmark, logs; local dashboard with overview, providers, requests, config,
  doctor, telemetry).
- Sprint 8: Model profiles with starter presets.
- Sprint 7: Validation, repair, and guard engines.
- Sprint 6: Context and prompt engines.
- Sprint 5: SQLite storage and local telemetry.
- Sprint 4: Pipeline engine.
- Sprint 3: Provider adapters for Ollama, LM Studio, and OpenAI-compatible
  local servers.
- Sprint 2: OpenAI-compatible gateway API.
- Sprint 1: Runtime skeleton.
- Sprint 0: Repository and documentation setup.
