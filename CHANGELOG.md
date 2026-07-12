# Changelog

All notable changes to Novexa are documented in this file.

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
  Tests 3 configurations (direct with thinking on, Novexa with thinking on,
  Novexa with thinking off) across debugging, planning, and JSON prompts.
  Detects reasoning leaks and measures latency impact of thinking on/off.
- **Terminal-Bench comparison**: Full agent harness evaluation using
  terminus-2 agent on 5 pinned terminal-bench-core tasks. Compares direct
  LM Studio vs Novexa stabilized mode. Measures task resolution, episode
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
- **Benchmark suite restructure**: raw data goes to `~/.novexa/benchmarks/`,
  summary reports stay in `benchmarks/reports/`.
- **Comprehensive benchmark report** (`benchmarks/reports/SUMMARY-20260712.md`):
  Agentic Coding + Local Model benchmarks for Ornith 9B and Qwen 3.5 9B.
- **`benchmarks/README.md`** documenting methodology and quick start.

### Benchmark Results (Ornith 9B)

| Metric | Direct | Novexa | Improvement |
|---|---|---|---|
| JSON Validity (Agentic) | 0% | **100%** | +100% |
| JSON Validity (Local) | 0% | **100%** | +100% |
| Instruction Following | 78% | **100%** | +22% |
| Tool Call Accuracy | 100% | 100% | maintained |
| Latency p50 (JSON) | 2,949ms | 352ms | 8.4× faster |

### Changed

- All benchmark scripts now output raw data to `~/.novexa/benchmarks/` (outside
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
  `novexa.example.yaml`, `README.md`, `LICENSE`, and `CHANGELOG.md`.
- `novexa.example.yaml` documenting the planned local-first configuration
  format for the alpha. YAML config parsing is not implemented yet.
- Installation guide (`docs/installation.md`) covering source builds, release
  archives, Docker, macOS, Linux, Windows, starting, dashboard, client setup,
  and uninstalling.
- Quickstart guide (`docs/quickstart.md`) with Ollama setup, model pull,
  startup, dashboard, chat completions, and `novexa doctor`.
- Troubleshooting guide (`docs/troubleshooting.md`) for common issues such as
  port conflicts, Ollama availability, missing models, dashboard build errors,
  SQLite permissions, invalid API keys, provider timeouts, missing profiles,
  streaming, and macOS quarantine.
- Release checklist (`docs/release-checklist.md`) for verifying builds,
  archives, Docker, and security before publishing.
- Build-time version metadata package (`runtime/internal/version`) with
  `Version`, `Commit`, and `BuildDate` injected via ldflags. `novexa version`
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
