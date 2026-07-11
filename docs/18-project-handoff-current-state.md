# Novexa Project Handoff: Current State

Last updated: 2026-07-11  
Repo: `/Users/afnanrudy/Github-Projects/Novexa`  
Current branch: `main`  
Latest verified commit: `d9881c6 feat: add profile doctor`

---

## 1. What Novexa Is

Novexa is a **local-first AI runtime** that sits between AI applications and local inference engines.

It is not a model, chatbot, or cloud gateway. Its purpose is to make local models feel more stable and production-ready through:

- OpenAI-compatible API
- provider adapters
- request pipeline
- context and prompt processing
- validation, repair, and guardrails
- local telemetry
- model profiles
- benchmark tooling
- Profile Doctor suggestions

Core positioning:

```text
Application
    ↓
Novexa Runtime
    ↓
Ollama / LM Studio / OpenAI-compatible Local Server
    ↓
Local Model
```

Primary docs:

- Vision: `docs/00-vision-and-positioning.md`
- Architecture: `docs/02-runtime-architecture.md`
- Engine specs: `docs/03-engine-specifications.md`
- Implementation roadmap: `docs/14-implementation-roadmap.md`
- Long-term plan: `docs/17-long-term-plan.md`

---

## 2. Product Decisions Already Made

Locked decisions:

- Novexa is **local-first**.
- V1 does not use cloud providers, billing, accounts, team management, or marketplace.
- Runtime is a **Go modular monolith**.
- API is OpenAI-compatible under `/v1`.
- Storage is local SQLite.
- Dashboard is local-only by default.
- Telemetry is local-only by default.
- Prompt and response content are not stored by default.
- Providers are adapters, not business logic.
- Every request should go through the Pipeline Engine.
- Model behavior should be improved through profiles and runtime tuning, not model fine-tuning/training.

Important rule:

```text
Novexa improves local model providers. It should not compete with Ollama, LM Studio, vLLM, or llama.cpp.
```

---

## 3. Current Implementation State

Sprints 1-10 are complete and committed.

Latest important commits:

```text
d9881c6 feat: add profile doctor
fa74f31 feat: export benchmark reports
43b6df9 feat: tune qwen profile and benchmark quality
36f7a11 docs: clarify alpha limitations
91fab52 build: package alpha release
e5c40fa feat: add CLI and local dashboard
110f2cb feat: add model profiles
276b6e4 feat: add validation repair and guard engines
d5b1759 feat: add context and prompt engines
b78cdcb feat: Sprint 5 storage and telemetry
0fb8a21 feat: add pipeline engine skeleton
125aeab feat: Sprint 3 provider adapters
```

Current working tree should be clean after `d9881c6`.

Core runtime areas:

- API types: `runtime/internal/api/`
- Gateway: `runtime/internal/gateway/`
- Pipeline: `runtime/internal/pipeline/`
- Providers: `runtime/internal/provider/`
- Context Engine: `runtime/internal/context/`
- Prompt Engine: `runtime/internal/prompt/`
- Guard Engine: `runtime/internal/guard/`
- Validation Engine: `runtime/internal/validation/`
- Repair Engine: `runtime/internal/repair/`
- Storage/Telemetry: `runtime/internal/storage/`, `runtime/internal/telemetry/`
- Profile loader/matcher: `runtime/internal/profiles/`
- CLI: `runtime/internal/cli/`
- Dashboard server: `runtime/internal/dashboard/`

---

## 4. Runtime Commands

From repo root:

```bash
cd runtime
go test ./...
go run ./cmd/novexa version
go run ./cmd/novexa start
```

Default runtime URLs:

```text
API:       http://127.0.0.1:8787/v1
Dashboard: http://127.0.0.1:8788
API key:   novexa-local
```

If port is already in use:

```bash
lsof -nP -iTCP:8787 -sTCP:LISTEN
```

Do not start multiple runtimes on the same port.

---

## 5. Supported Providers

Implemented provider adapters:

- Ollama
- LM Studio
- OpenAI-compatible local server

Provider defaults:

```text
Ollama:                 http://localhost:11434
LM Studio:              http://localhost:1234/v1
OpenAI-compatible local: http://localhost:8000/v1
```

No cloud providers are implemented in V1.

---

## 6. Current Model Profiles

Profile files live in `profiles/`.

Current profiles:

```text
generic-local.yaml
qwen3-8b.yaml
qwen3.5-2b.yaml
qwen2.5-coder-7b.yaml
deepseek-r1-8b.yaml
llama3.1-8b.yaml
gemma3-12b.yaml
mistral-small.yaml
qwen3-1.7b.yaml          ← new, LM Studio validated
ornith-1.0-9b-q4-km.yaml ← new, LM Studio validated
gemma-4-e4b.yaml         ← new, LM Studio validated
```

### LM Studio Validated Profiles

Benchmark matrix run on 2026-07-11 against `http://192.168.0.164:1234/v1` (3 attempts per model):

| Profile | LM Studio Model | Size | Role | Novexa Pass | Direct p50 | Doctor |
|---------|----------------|------|------|-------------|------------|--------|
| `qwen2.5-coder-7b` | `qwen2.5-coder-7b-instruct` | 7B | **Coding** | 21/21 | 114ms | Good baseline |
| `qwen3-1.7b` | `qwen/qwen3-1.7b` | 1.7B | **Fast chat** | 21/21 | 94ms | Good baseline |
| `ornith-1.0-9b-q4-km` | `ornith-1.0-9b@q4_k_m` | 9B | **Quality alt** | 21/21 | 182ms | Good baseline |
| `qwen3.5-9b` | `qwen/qwen3.5-9b` | 9B | **Technical** | 18/21 | 197ms | Good baseline |
| `gemma-4-e4b` | `google/gemma-4-e4b` | 4B | **Mid-size** | 15/21 | 175ms | Needs tuning |

**Recommended default model choices:**

| Use Case | LM Studio Model | Profile |
|----------|---------------|---------|
| Coding | `qwen2.5-coder-7b-instruct` | `qwen2.5-coder-7b` |
| Fast general chat | `qwen/qwen3-1.7b` | `qwen3-1.7b` |
| Mid-size general chat | `google/gemma-4-e4b` | `gemma-4-e4b` |
| Quality alternative | `ornith-1.0-9b@q4_k_m` | `ornith-1.0-9b-q4-km` |

**Benchmark mode notes:**
- **A-LMStudioDirect** — raw provider pass-through. Diagnostic only; not a quality gate.
- **B-NovexaDirect** — thin Novexa proxy. Diagnostic only; not a quality gate.
- **C-NovexaStabilized** — main quality gate. Includes context, prompt, validation, repair, and telemetry.
- **D-NovexaStructured** — strict JSON/schema output mode. Quality gate for structured output.

All validated profiles pass 100% through Novexa stabilized and structured modes.

### LM Studio Benchmark Matrix

Run benchmarks across all LM Studio models and produce a summary table:

```bash
ATTEMPTS=1 LMSTUDIO_URL=http://192.168.0.164:1234/v1 ./scripts/benchmark-lmstudio-matrix.sh
```

The matrix auto-detects models, runs each through `benchmark-local-model.sh`, runs Profile Doctor on each JSON report, and saves a summary to `benchmarks/lmstudio-matrix-<timestamp>.md`.

Use `ATTEMPTS=3` for more reliable pass/fail data.

### Models Skipped

| Model | Reason |
|-------|--------|
| `qwen2.5-0.5b-instruct` | Too small for useful work |
| `qwen2.5-coder-0.5b-instruct` | Too small, stabilized mode fails concise |
| `qwen/qwen2.5-coder-14b` | 7.6s structured latency — unusable on current hardware |
| `text-embedding-nomic-embed-text-v1.5` | Embedding model, not chat |

### Profile Tuning Pattern

All validated profiles follow the same tuning pattern:

1. `defaults.thinking: false` — prevents token exhaustion on reasoning
2. `defaults.max_tokens: 512` (small) or `1024` (9B) — conservative generation budget
3. `prompt.instruction_strength: strict` — forces exact-format compliance
4. Exact-format instruction: "For one-word or exact-format prompts, output only the requested final content"
5. JSON instruction: "When the user asks for JSON, return ONLY the raw JSON object. No markdown fences, no code blocks, no explanation before or after."
6. `guard.anti_loop: aggressive` — prevents repetition loops

Most important tuned profile right now:

```text
profiles/qwen3.5-2b.yaml
```

Why it matters:

- It disables thinking by default for `qwen3.5:2b`.
- It reduces token exhaustion / empty-content behavior.
- It adds exact-format instructions.
- It prevents plain answers from being wrapped in JSON unless JSON is requested.
- It has been benchmarked against Ollama direct and Novexa modes.

Important Qwen behavior discovered:

- Qwen/Ollama can emit `message.thinking`.
- If thinking consumes the generation budget, `message.content` can be empty.
- Novexa now supports `novexa.thinking.enabled` and profile-level `defaults.thinking`.
- Request-level thinking override has priority over profile default.

---

## 7. Benchmark Tooling

Benchmark script:

```bash
./scripts/benchmark-local-model.sh qwen3.5:2b
```

Optional attempts:

```bash
ATTEMPTS=3 ./scripts/benchmark-local-model.sh qwen3.5:2b
```

The script tests:

- A: Ollama direct with `think:false`
- B: Novexa direct mode
- C: Novexa stabilized mode
- D: Novexa structured JSON mode

It scores:

- exact one-word answers
- JSON validity
- required JSON keys
- markdown fence avoidance
- latency p50/p95

Generated reports:

```text
benchmarks/<model>-<timestamp>.md
benchmarks/<model>-<timestamp>.json
```

`benchmarks/` is ignored by git.

Known verified benchmark for `qwen3.5:2b`:

```text
Novexa exact instruction following: passed
Novexa JSON output: passed
Profile Doctor result: Good baseline
```

---

## 8. Profile Doctor

Profile Doctor script:

```bash
./scripts/profile-doctor.sh benchmarks/<report>.json profiles/<profile>.yaml
```

It reads benchmark JSON and prints:

- model
- report path
- result
- findings
- recommended profile changes
- suggested YAML patch snippet

It does **not** edit YAML files.

Detection rules implemented:

- exact prompt failures
- JSON failures
- empty responses
- Novexa worse than Ollama
- high latency
- stable success / good baseline

Fixture reports live in:

```text
scripts/fixtures/
```

Fixture coverage:

- all pass
- JSON failure
- exact answer failure
- empty response
- high latency

Verification command:

```bash
for f in scripts/fixtures/profile-doctor-*.json; do
  ./scripts/profile-doctor.sh "$f"
done
```

---

## 9. Release / Packaging State

Alpha packaging exists.

Important files:

- `Makefile`
- `Dockerfile`
- `.dockerignore`
- `scripts/build-release.sh`
- `scripts/check-release.sh`
- `scripts/install.sh`
- `.github/workflows/ci.yml`
- `.github/workflows/release.yml`
- `docs/installation.md`
- `docs/quickstart.md`
- `docs/troubleshooting.md`
- `docs/release-checklist.md`

Verified previously:

```text
go test ./...
go vet ./...
npm run build
make test
make dashboard
make build
make release
make check-release
```

Known limitations documented:

- YAML config parsing is not implemented.
- `novexa stop` and `novexa restart` are not implemented.
- Streaming chat completions are not implemented.
- Dockerfile exists, but Docker image was not manually built/tested in the environment where packaging was done.
- Cross-platform artifacts are cross-compiled, not manually run on every target.

---

## 10. New Direction: Central Tuning Layer

Decision added on 2026-07-11:

Novexa should become the shared tuning layer for apps like:

- OpenCode
- Continue
- Cline
- Open WebUI
- custom coding agents
- custom local AI apps

The goal is for each app to keep config minimal:

```text
base_url = http://127.0.0.1:8787/v1
api_key  = novexa-local
model    = ollama:<model-name>
```

Novexa should own reusable model settings:

- temperature
- top_p
- max_tokens
- thinking on/off
- exact-format instructions
- JSON behavior
- anti-loop behavior
- provider quirks
- telemetry

This avoids duplicating model tuning across every app.

Future mode to implement:

```text
lightweight
```

Purpose:

- lower token overhead than stabilized mode
- useful for OpenCode and coding agents
- apply profile defaults, thinking policy, and minimal prompt policy
- avoid heavy context/prompt wrappers unless needed

Suggested mode positioning:

```text
direct       = raw-ish provider path for debugging/benchmarking
lightweight = shared tuning with minimal wrapper
stabilized  = reliability-focused normal app mode
structured  = JSON/schema mode with validation and repair
agent       = future coding-agent governance mode
```

Current implementation does not yet have `lightweight` mode. It is now documented in:

- `docs/02-runtime-architecture.md`
- `docs/07-pipeline-specification.md`
- `docs/17-long-term-plan.md`

---

## 11. Current Best Next Step

Recommended next project step:

```text
Model Profile Expansion + Doctor Loop for OpenCode stack
```

Why:

Novexa now has the loop:

```text
benchmark → JSON report → Profile Doctor → profile tuning suggestion
```

The next value comes from using that loop on real local models.

Suggested workflow:

```bash
ollama list
ATTEMPTS=3 ./scripts/benchmark-local-model.sh <model>
./scripts/profile-doctor.sh benchmarks/<generated-report>.json profiles/<matching-profile>.yaml
```

Target models to tune next:

- GLM 5.2
- Kimi Code
- DeepSeek Flash
- `qwen2.5-coder:7b`
- `llama3.2:3b`
- `gemma3:1b` or `gemma3:4b`
- `deepseek-r1:8b`
- `qwen3:8b`

Only edit profile YAML when benchmark evidence clearly supports the change.

Do not change runtime code unless a real bug is found.

---

## 12. Suggested Agent Prompt For Next Session

```text
You are continuing Novexa.

Read:
- docs/18-project-handoff-current-state.md
- README.md
- docs/10-model-profile-specification.md
- scripts/benchmark-local-model.sh
- scripts/profile-doctor.sh

Goal:
Run the benchmark/profile-doctor loop for available local Ollama models and tune profiles based on evidence.

Priority models if available:
- GLM 5.2
- Kimi Code
- DeepSeek Flash
- qwen2.5-coder
- qwen3.5:2b

Product direction:
Novexa should be the central tuning layer for OpenCode/Continue/Cline/Open WebUI so those apps only need base_url, api_key, and model.

Rules:
- Do not add cloud providers.
- Do not edit generated benchmark reports.
- Do not change runtime code unless a benchmark exposes a real bug.
- Only edit profile YAML when benchmark evidence clearly supports it.
- Preserve privacy defaults.
- Run tests after profile changes.

Workflow:
1. Check git status.
2. List available Ollama models.
3. Match models to profiles.
4. For each model with a profile, run:
   ATTEMPTS=3 ./scripts/benchmark-local-model.sh <model>
5. Run:
   ./scripts/profile-doctor.sh <generated-json-report> <matching-profile-yaml>
6. Summarize pass/fail quality, latency p50/p95, doctor result, and suggested changes.
7. Apply conservative profile updates only when evidence is clear.
8. Re-run benchmark after any profile change.
9. Run go test ./... from runtime.
10. Do not commit unless asked.
```

---

## 13. Suggested Skills For Future Agents

Recommended Codex skills:

- `diagnose`: use when benchmark results reveal confusing runtime behavior or regression.
- `review`: use before release or before merging a large agent-generated diff.
- `tdd`: use if adding runtime features such as CLI `novexa profile doctor`.
- `improve-codebase-architecture`: use before larger refactors.
- `handoff`: use before ending a long session or switching models.

---

## 14. Important Caveats

- `novexa-local` is the documented default local API key. Treat it as non-secret but do not introduce real secrets.
- Do not store raw prompts/responses by default.
- Do not store raw model thinking / chain-of-thought.
- Thinking telemetry should remain safe metadata only, such as whether reasoning content was present.
- Generated reports under `benchmarks/` are local artifacts and ignored by git.
- If testing benchmark scripts from a sandbox that cannot access localhost, use an environment/session that can reach `127.0.0.1:8787` and `localhost:11434`.
