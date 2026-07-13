<div align="center">

# Novexa

### Intelligence Runtime for Local AI

**Novexa is an intelligence runtime that makes local AI models more stable,
reliable, and production-ready.**

[![Go Version](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![CI](https://github.com/EffNine/Novexa/actions/workflows/ci.yml/badge.svg)](https://github.com/EffNine/Novexa/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/EffNine/Novexa?include_prereleases&label=release)](https://github.com/EffNine/Novexa/releases)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue)](./LICENSE)
[![GitHub stars](https://img.shields.io/github/stars/EffNine/Novexa?style=social)](https://github.com/EffNine/Novexa/stargazers)

[Quick start](#get-started) · [Benchmarks](#benchmarks) · [Docs](./docs/) · [Integrations](./docs/integrations/README.md) · [Changelog](./CHANGELOG.md)

</div>

---

Novexa sits between your app and your local inference server:

```text
OpenCode / Continue / Cline / Open WebUI / SDK
        ↓
Novexa Runtime
http://127.0.0.1:8787/v1
        ↓
LM Studio / Ollama / OpenAI-compatible local server
        ↓
Local model
```

Novexa is not a model, chatbot, or hosted cloud gateway. It is the runtime layer
around local AI.

---

## Quick demo

Start Novexa and point any OpenAI-compatible client at it — that's it.

```bash
# Build and start
make build
./novexa start

# Any OpenAI SDK / cURL works out of the box
curl http://127.0.0.1:8787/v1/chat/completions \
  -H "Authorization: Bearer novexa-local" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "lmstudio:qwen2.5-coder-7b-instruct",
    "messages": [{"role": "user", "content": "Write a Go function that adds two ints. Code only."}]
  }'
```

```python
from openai import OpenAI

client = OpenAI(base_url="http://127.0.0.1:8787/v1", api_key="novexa-local")
print(client.chat.completions.create(
    model="lmstudio:qwen2.5-coder-7b-instruct",
    messages=[{"role": "user", "content": "Write a tiny TypeScript add function."}],
).choices[0].message.content)
```

Dashboard: **http://127.0.0.1:8788**

<!-- A demo GIF will go here once recorded. -->

![Novexa quick demo](./docs/assets/novexa-demo.gif)

---

## Key metrics

Benchmarked on Ornith 9B and Qwen 3.5 9B via LM Studio.
Full report: [`benchmarks/reports/SUMMARY-20260712.md`](benchmarks/reports/SUMMARY-20260712.md).

| Metric | Direct (no Novexa) | Novexa | Change |
|---|---|---|---|
| JSON validity (agentic) | 0% | **100%** | +100% |
| JSON + required keys | 0% | **100%** | +100% |
| Tool-call accuracy | 100% | 100% | maintained |
| Latency p50 (JSON) | 2,949 ms | **352 ms** | **8.4× faster** |
| HTTP errors | ~50% | **0%** | eliminated |
| Repetition false positives | 113 | **0** | eliminated |
| Instruction following (structured) | 67% | **100%** | +33% |

These per-turn gains compound across multi-turn agent loops (30+ turns),
where a single broken JSON response can stall the entire run.

---

## Who is this for?

- **OpenCode / Continue / Cline users** — You run a coding agent on a local
  model and hit broken JSON, repeated output, or empty responses. Point your
  client at Novexa instead of the raw provider and those failure modes
  disappear.
- **Ollama / LM Studio users** — You like local inference but the model is
  rough in real apps. Novexa adds JSON repair, instruction-following assist,
  anti-loop guards, and telemetry without replacing your model.
- **Local AI app builders** — You're building on top of local models and need
  an OpenAI-compatible reliability layer with model routing, memory, and
  provider-specific fixes. Novexa is that layer.

---

## What makes Novexa different?

| | Novexa | Raw Ollama / LM Studio | Cloud gateways |
|---|---|---|---|
| Runs locally (no data leaves your machine) | ✅ | ✅ | ❌ |
| OpenAI-compatible drop-in | ✅ | partial | ✅ |
| JSON validation + repair | ✅ | ❌ | varies |
| Instruction-following assist | ✅ | ❌ | ❌ |
| Per-step model routing (agent) | ✅ | ❌ | ❌ |
| Persistent cross-model memory | ✅ | ❌ | ❌ |
| Provider-specific quirk fixes | ✅ | ❌ | ❌ |
| Local telemetry dashboard | ✅ | ❌ | ✅ |
| Local-first, no cloud dependency | ✅ | ✅ | ❌ |

Novexa improves the **layer around the model** instead of replacing the model.
It is not an agent framework, a model, or a hosted cloud gateway — it is the
runtime that makes whatever model you already run behave reliably.

---

## Screenshots

> The Novexa dashboard runs at `http://127.0.0.1:8788` and shows request
> metadata, provider status, telemetry, and diagnostics. Full prompts and
> responses are hidden by default.

<!-- Replace the placeholders below with real screenshots once captured. -->

| Dashboard overview | Provider status |
|---|---|
| ![Dashboard overview](./docs/assets/dashboard-overview.png) | ![Provider status](./docs/assets/dashboard-providers.png) |

| Request telemetry | Doctor diagnostics |
|---|---|
| ![Request telemetry](./docs/assets/dashboard-telemetry.png) | ![Doctor diagnostics](./docs/assets/dashboard-doctor.png) |

---

## Why Novexa?

Local AI is private and cheap, but it is often rough in real apps:

- broken JSON
- repeated output
- empty or reasoning-only responses
- weak instruction following
- provider-specific quirks
- model-specific tuning headaches
- poor debugging visibility

Novexa improves the layer around the model instead of replacing the model.

It provides:

- OpenAI-compatible `/v1/chat/completions` (streaming and non-streaming)
- local provider adapters
- model profiles
- runtime modes
- **agentic coding router** — automatic per-step model selection by task difficulty
- **memory engine** — zero-VRAM persistent memory (facts, episodes, model-fit tracking) shared across all models, survives session boundaries
- prompt and context handling
- JSON validation and repair
- anti-loop and safety guards
- instruction-following assist (auto-detects 14 constraint types)
- local telemetry
- agent mode (step budget enforcement, tool-call loop detection, tool-call JSON validation, context compaction)
- CLI diagnostics
- local dashboard

---

## Get Started

Build from source:

```bash
git clone https://github.com/EffNine/Novexa.git
cd Novexa
make build
./novexa start
```

Or download a pre-built archive from
[GitHub Releases](https://github.com/EffNine/Novexa/releases).

### Docker

```bash
docker build -t novexa:0.2.0-alpha .
docker run -d --name novexa \
  -p 127.0.0.1:8787:8787 \
  -p 127.0.0.1:8788:8788 \
  -v novexa-data:/data \
  novexa:0.2.0-alpha
```

The runtime stores telemetry at `/data/.novexa/novexa.db` on a persistent Docker
volume. See [Installation → Docker](./docs/installation.md#docker) for details.

Default endpoints:

```text
API:       http://127.0.0.1:8787/v1
Dashboard: http://127.0.0.1:8788
API key:   novexa-local
```

See:

- [Installation](./docs/installation.md)
- [Quickstart](./docs/quickstart.md)
- [Troubleshooting](./docs/troubleshooting.md)
- [Integration guides](./docs/integrations/README.md)

---

## Recommended Local Setup

For LM Studio, Novexa uses its OpenAI-compatible API for inference and can
optionally manage model lifecycle via LM Studio's v1 REST API:

| Capability | Novexa today | LM Studio v1 API available |
|---|---|---|
| Chat completion (temperature, top_p, tools, etc.) | ✅ | ✅ |
| Per-model default temperature via profiles | ✅ | ✅ |
| Model loading with custom config | ✅ | `POST /api/v1/models/load` |
| Model unloading | ✅ | `POST /api/v1/models/unload` |
| Context length per model | ✅ | `context_length` in load request |
| Flash attention / GPU offload | ✅ | `flash_attention`, `offload_kv_cache` in load request |
| Auto-unload previous model on switch | ✅ | `POST /api/v1/models/unload` |

Basic LM Studio setup:

```bash
NOVEXA_PROVIDER_DEFAULT=lmstudio \
NOVEXA_LMSTUDIO_URL=http://localhost:1234/v1 \
NOVEXA_DEFAULT_MODEL=qwen2.5-coder-7b-instruct \
NOVEXA_PROVIDER_TIMEOUT_SECONDS=120 \
./novexa start
```

For a LAN-hosted LM Studio server, replace the URL:

```bash
NOVEXA_LMSTUDIO_URL=http://192.168.0.164:1234/v1
```

Recommended model choices:

| Use case | Model ID | Profile |
|---|---|---|
| Coding | `lmstudio:qwen2.5-coder-7b-instruct` | `qwen2.5-coder-7b` |
| Agentic coding | `lmstudio:ornith-1.0-9b@q4_k_m` | `ornith-1.0-9b-q4-km` |
| Fast chat | `lmstudio:qwen/qwen3-1.7b` | `qwen3-1.7b` |
| Mid-size chat | `lmstudio:google/gemma-4-e4b` | `gemma-4-e4b` |
| Ollama fast chat | `ollama:llama3.2:3b` | `llama3.2-3b` |
| Ollama mid-size | `ollama:gemma3:4b` | `gemma3-4b` |

Apps should only need:

```text
base_url: http://127.0.0.1:8787/v1
api_key: novexa-local
model: lmstudio:qwen2.5-coder-7b-instruct
```

Novexa handles profile tuning, thinking policy, provider quirks, JSON handling,
and runtime behavior.

---

## Benchmarks

Novexa improves local model reliability across multiple dimensions. Full
report: [`benchmarks/reports/SUMMARY-20260712.md`](benchmarks/reports/SUMMARY-20260712.md).

### Ornith 9B — Agentic Coding (Tool calls + JSON + Multi-turn)

| Metric | Direct LM Studio | Novexa Stabilized | Improvement |
|---|---|---|---|
| JSON Validity | 0% | **100%** | +100% |
| JSON + Required Keys | 0% | **100%** | +100% |
| Tool Call Accuracy | 100% | 100% | maintained |
| Latency p50 (JSON) | 2,949ms | 352ms | **8.4× faster** |

### Instruction-Following Assist

Novexa automatically detects formatting constraints (sentence count, word
restrictions, bullet format, JSON, line count, etc.) and injects explicit
reminders into the system prompt:

```
Prompt: "2 sentences, end with 'learning', no word 'language'"
→ Novexa: "CRITICAL: 1. Exactly 2 sentences. 2. End with 'learning'.
           3. Do NOT use the word 'language'."
→ Valid response in 1 attempt ✅
```

### How Novexa Helps Agent Frameworks

Novexa is not an agent framework. It improves **every turn** inside any
agent loop (OpenCode, Continue, Claude Code, Terminus-2). When an agent
makes 30+ turns to solve a task, Novexa's per-turn reliability gains
compound:

| Per-turn improvement | After 30 turns | Compound effect |
|---|---|---|
| JSON: 0% → 100% | Zero parsing failures | Agent never gets stuck on bad JSON |
| Instruction: 78% → 100% | Fewer wrong file edits | Higher SWE-Bench success rate |
| Tool calls: 100% maintained | All tool invocations valid | No wasted episodes |

> **Ornith 9B scores 43.1% on Terminal-Bench 2.1 and 69.4% on SWE-Bench
> Verified when using agent frameworks.** Novexa helps local deployments
> close the gap with cloud-grade reliability per turn.

---

## Supported Providers

Implemented providers:

- Ollama
- LM Studio (OpenAI-compatible + planned v1 REST API model management)
- OpenAI-compatible local servers

Future candidates:

- llama.cpp server
- vLLM
- SGLang
- Text Generation Inference
- LocalAI
- KoboldCpp

---

## Runtime Modes

Novexa supports multiple runtime modes:

| Mode | Benchmark label | Best for |
|---|---|---|
| `direct` | `B-NovexaDirect` | Diagnostics and raw provider comparison |
| `lightweight` | `C-NovexaLightweight` | Coding agents and low-token calls |
| `stabilized` | `D-NovexaStabilized` | General chat quality and reliability |
| `structured` | `E-NovexaStructured` | JSON/schema-sensitive workflows |
| `agent` | — | Agentic coding loops (with optional router) |

Provider-direct benchmarks use:

```text
A-LMStudioDirect
A-OllamaDirect
```

`direct` is diagnostic only. `stabilized` and `structured` are the main quality
gates. `lightweight` is optimized for tools such as OpenCode, Continue, and
Cline.

---

## Integration Guides

Current guides:

- [OpenCode](./docs/integrations/opencode.md)
- [Continue](./docs/integrations/continue.md)
- [Cline](./docs/integrations/cline.md)
- [Open WebUI](./docs/integrations/open-webui.md)
- [OpenAI SDK clients](./docs/integrations/openai-sdk.md)
- [LM Studio setup](./docs/integrations/lmstudio.md) — including management API capabilities

All guides use the same basic pattern: point the client at Novexa's
OpenAI-compatible API, then let Novexa handle provider and model behavior.

---

## OpenAI-Compatible Usage

cURL:

```bash
curl http://127.0.0.1:8787/v1/chat/completions \
  -H "Authorization: Bearer novexa-local" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "lmstudio:qwen2.5-coder-7b-instruct",
    "messages": [
      {
        "role": "user",
        "content": "Write a Go function that adds two ints. Return code only."
      }
    ],
    "novexa": {
      "mode": "lightweight"
    }
  }'
```

Python:

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://127.0.0.1:8787/v1",
    api_key="novexa-local",
)

response = client.chat.completions.create(
    model="lmstudio:qwen2.5-coder-7b-instruct",
    messages=[
        {"role": "user", "content": "Write a tiny TypeScript add function."}
    ],
)

print(response.choices[0].message.content)
```

---

## Validated Profiles

Official LM Studio-validated profiles:

| Profile | LM Studio model | Role | Result |
|---|---|---|---|
| `qwen2.5-coder-7b` | `qwen2.5-coder-7b-instruct` | Coding baseline | Good baseline |
| `qwen3-1.7b` | `qwen/qwen3-1.7b` | Fast chat | Good baseline |
| `ornith-1.0-9b-q4-km` | `ornith-1.0-9b@q4_k_m` | Quality alternative | Good baseline |
| `qwen3.5-9b` | `qwen/qwen3.5-9b` | Larger Qwen option | Good baseline |
| `gemma-4-e4b` | `google/gemma-4-e4b` | Mid-size chat | Tuned profile |

Profiles apply defaults such as:

- `temperature`
- `top_p`
- `max_tokens`
- thinking/reasoning policy
- exact-format instructions
- JSON-only instructions
- guard settings

If no matching profile exists, Novexa falls back to `generic-local`.

---

## Agentic Coding Router

Novexa includes an **Agentic Coding Router** that automatically selects the
right model for each coding task based on difficulty. When enabled, the router
classifies every agent step using structural heuristics (message length, file
count, traceback presence, keywords, step count) and routes to the optimal
model:

| Difficulty | Example | Routes to |
|------------|---------|-----------|
| 1 — trivial | Typo fix, rename variable | Tiny/fast model (Gemma 3 1B, Qwen3 1.7B) |
| 2 — simple | Add parameter, fix import | Small model (Qwen 2.5 Coder 7B) |
| 3 — moderate | Implement function, error handling | Medium model (Ornith 9B) |
| 4 — complex | Multi-file refactor, feature | Strong model (DeepSeek R1 8B) |
| 5 — novel | New algorithm, architecture design | Strongest available + reasoning |

The router re-evaluates at every agent step — so a "fix typo" step uses a tiny
model while the next "implement payment handler" step escalates to a large one.
Routing is **opt-in** (disabled by default) and only activates in agent mode.

```yaml
# novexa.yaml
routing:
  enabled: true         # Enable per-step coding routing
```

Clients can also provide per-request hints:

```json
{
  "model": "lmstudio:qwen2.5-coder-7b-instruct",
  "novexa": {
    "routing": {
      "hint_difficulty": 4,
      "hint_task_type": "refactor",
      "preferred_provider": "lmstudio",
      "min_context": 32768
    }
  }
}
```

See the full specification at `docs/specs/19-agentic-coding-router-specification.md`.

---

## Agentic Coding

Novexa focuses on three hero models for agentic coding with local AI:

| Role | Model | Profile |
|---|---|---|
| Primary fast coder | `lmstudio:qwen2.5-coder-7b-instruct` | `qwen2.5-coder-7b` |
| Complex reasoning fallback | `lmstudio:qwen/qwen3.5-9b` | `qwen3.5-9b` |
| Quality alternative | `lmstudio:ornith-1.0-9b@q4_k_m` | `ornith-1.0-9b-q4-km` |

These models declare `tool_calling: weak` because they do not reliably emit native OpenAI-style `tool_calls`. Novexa adds a prompt-based tool-calling shim for them:

1. Converts `tools` into explicit prompt instructions and a JSON schema.
2. Asks the model to reply with a JSON tool call object.
3. Parses the response back into OpenAI-compatible `tool_calls`.
4. Validates tool names and required arguments.
5. Repairs or retries when the output is malformed.

Agentic modes also harden structured JSON output, summarize old tool results to save context budget, and detect repeated tool calls.

---

## Memory Engine (Experimental)

Novexa includes a **Memory Engine** that gives coding agents persistent,
cross-model memory using zero VRAM. Facts, episode summaries, and model-fit
data are stored in SQLite — shared across all models, surviving model swaps
and session boundaries.

### How It Works

```
Agent Step → Memory Engine retrieves relevant facts → injects into context
    ↓
Model processes step
    ↓
Memory Engine extracts new facts → updates model fit → stores episode
    ↓
(next step starts with updated memory)
```

### Memory Types

| Type | Storage | Persistence | Purpose |
|------|---------|-------------|---------|
| Facts | SQLite + Go map cache | Cross-session | Project knowledge, preferences |
| Episodes | SQLite | Session (auto-summarized) | What happened, what worked |
| Model Fit | SQLite | Cross-session | Router feedback, performance history |

### Injection

Memory is injected as a prepended system message within a configurable token
budget (default 1200 tokens). Facts are scored by `relevance × confidence ×
access_frequency` — the most relevant facts are selected first.

```yaml
# novexa.yaml
memory:
  enabled: true                     # Opt-in in V1
  injection_budget_tokens: 1200    # Tokens reserved for memory context
  max_injected_facts: 20           # Maximum facts per injection
  max_facts: 10000                 # Max stored facts before eviction
  track_model_fit: true            # Record model performance per task type
```

### CLI Commands

```bash
novexa memory status        # Show database path, fact count, model fit entries
novexa memory facts         # List stored facts
novexa memory facts search  # Search facts by key/value
novexa memory clear --force # Reset all memory
```

### API Endpoints

```text
GET  /v1/novexa/memory/facts       # List/search stored facts
GET  /v1/novexa/memory/model-fit   # Model performance data
GET  /v1/novexa/memory/status      # Memory engine status
POST /v1/novexa/memory/clear       # Clear all memory
```

Memory is **opt-in** (disabled by default, set `memory.enabled: true`).
No GPU memory is used at any tier.

See the full specification at `docs/specs/20-memory-engine-specification.md`.

---

## Managed Thinking (Experimental)

Local reasoning models can behave more like frontier models when they are allowed to think before answering. Novexa adds a **managed thinking** layer so reasoning is controlled, not chaotic:

- `thinking_policy` in model profiles decides when thinking is enabled.
- Token budget is split into output budget + reasoning budget.
- Reasoning returned in a separate field (`reasoning_content`) is stripped from the final response.
- Reasoning wrapped in explicit markers (`<thinking>`, `<reasoning>`, fenced blocks) is stripped.
- Thinking is automatically disabled for JSON/schema and tool-calling workflows.
- Telemetry records thinking mode and reasoning presence without storing reasoning text.

Enable per request:

```json
{
  "novexa": {
    "thinking": { "enabled": true }
  }
}
```

Current limitation: some local models emit reasoning as plain prose inside the main content. Novexa strips explicit markers and separate reasoning fields, but cannot yet remove free-form reasoning prose from every model.

Run the managed thinking benchmark:

```bash
LMSTUDIO_URL=http://localhost:1234/v1 \
NOVEXA_URL=http://127.0.0.1:8787/v1 \
ATTEMPTS=1 \
./scripts/benchmark-managed-thinking.sh qwen/qwen3.5-9b
```

---

## Benchmarking

Run a single model benchmark:

```bash
BENCHMARK_PROVIDER=lmstudio \
LMSTUDIO_URL=http://localhost:1234/v1 \
ATTEMPTS=3 \
./scripts/benchmark-local-model.sh qwen2.5-coder-7b-instruct
```

Run the LM Studio model matrix:

```bash
ATTEMPTS=1 \
LMSTUDIO_URL=http://localhost:1234/v1 \
./scripts/benchmark-lmstudio-matrix.sh
```

Each benchmark writes:

```text
benchmarks/<model>-<timestamp>.md
benchmarks/<model>-<timestamp>.json
```

Use Profile Doctor on a JSON report:

```bash
./scripts/profile-doctor.sh benchmarks/<report>.json
```

Profile Doctor is read-only. It recommends tuning changes but does not edit
profiles automatically.

### Standard before/after scorecard

For a reproducible comparison against an established benchmark, install the
EleutherAI LM Evaluation Harness and run IFEval through the direct provider and
Novexa with identical generation settings:

```bash
python3 -m venv .venv-bench
.venv-bench/bin/pip install "lm-eval[api]" langdetect immutabledict

DIRECT_BASE_URL=http://192.168.0.164:1234/v1 \
NOVEXA_BASE_URL=http://127.0.0.1:8787/v1 \
NOVEXA_MODEL=lmstudio:qwen/qwen3.5-9b \
LM_EVAL_BIN="$PWD/.venv-bench/bin/lm_eval" \
./scripts/benchmark-standard-scorecard.sh qwen/qwen3.5-9b
```

The default task is `ifeval`, a standard instruction-following benchmark. The
runner writes raw `lm-eval` output plus a Markdown and JSON scorecard under
`benchmarks/standard/`. Add supported generation tasks with
`STANDARD_TASKS=ifeval,<task>`; keep task version, model artifact, few-shot
count, and generation settings unchanged for every comparison. Start Novexa in
the mode being measured before running the scorecard; `stabilized` is its
default mode. For example: `go run ./cmd/novexa start --mode stabilized`.
Use `LIMIT=50` only for a fast validation run; omit it for the full score.

### Terminal-Bench agent scorecard

Terminal-Bench measures a complete coding agent (model, Novexa, agent loop, and
terminal tools), rather than the model alone. It requires Docker Desktop and a
Python 3.13 environment:

```bash
python3.13 -m venv .venv-terminal
.venv-terminal/bin/pip install terminal-bench

DIRECT_BASE_URL=http://192.168.0.164:1234/v1 \
NOVEXA_BASE_URL=http://127.0.0.1:8787/v1 \
./scripts/benchmark-terminal-bench.sh qwen/qwen3.5-9b
```

The first run uses five `terminal-bench-core==0.1.1` tasks and the same
`terminus-2` agent for both endpoints. Increase `TERMINAL_BENCH_TASKS` only
after the smoke run succeeds.

### Agentic coding benchmark

Run a focused comparison of direct LM Studio vs. Novexa on tool-calling,
structured JSON, and multi-turn prompts:

```bash
LMSTUDIO_URL=http://localhost:1234/v1 \
NOVEXA_URL=http://127.0.0.1:8787/v1 \
ATTEMPTS=3 \
./scripts/benchmark-agentic-coding.sh qwen2.5-coder-7b-instruct
```

Results are written to `benchmarks/agentic/<model>-<timestamp>.md` and
`benchmarks/agentic/<model>-<timestamp>.json`.

---

## CLI

Implemented commands:

```bash
novexa start
novexa status
novexa doctor
novexa config show
novexa providers
novexa models
novexa benchmark
novexa logs
novexa version
novexa stop
novexa restart
```

---

## Dashboard

Default dashboard:

```text
http://127.0.0.1:8788
```

The dashboard is local-only by default. Secrets are redacted. Prompts and
responses are hidden by default.

---

## Telemetry and Privacy

Novexa is local-first and privacy-first.

Default telemetry behavior:

```yaml
telemetry:
  local: true
  external: false
  log_prompts: false
  log_responses: false
```

By default, Novexa stores metadata only. It does not store full prompts or full
responses unless explicitly configured. It does not send external telemetry.

---

## Alpha Limitations

Novexa `0.2.0-alpha` is usable, but not feature-complete:

- Continue tab autocomplete should use LM Studio directly for now.
- Dockerfile exists, but Docker image verification may vary by host.
- Profile Doctor is read-only.

---

## Release Targets

Alpha release archives are built for:

- macOS arm64
- macOS amd64
- Linux amd64
- Linux arm64
- Windows amd64

---

## Development Rules

Novexa follows these rules:

1. Keep Novexa local-first.
2. Do not add cloud providers in V1.
3. Do not add billing in V1.
4. Do not bypass the Pipeline Engine.
5. Keep provider adapters thin.
6. Do not store prompts or responses by default.
7. Do not send external telemetry by default.
8. Benchmark before tuning model profiles.

---

## Contributing

Contributions are welcome! Novexa is local-first and runtime-only — please
read [CONTRIBUTING.md](./CONTRIBUTING.md) before opening a PR to understand the
core rules (no cloud providers in V1, no billing, keep provider adapters thin,
don't bypass the Pipeline Engine).

Quick start for contributors:

```bash
make test    # go test ./runtime/...
make vet     # go vet ./runtime/...
make dashboard  # build the React dashboard
make build   # build the runtime binary
```

Use the [bug report](.github/ISSUE_TEMPLATE/bug_report.md) and
[feature request](.github/ISSUE_TEMPLATE/feature_request.md) templates when
opening issues.

---

## Star history

[![Star History Chart](https://api.star-history.com/svg?repos=EffNine/Novexa&type=Date)](https://star-history.com/#EffNine/Novexa&Date)

---

## License

Novexa is licensed under the Apache License 2.0. See [LICENSE](./LICENSE).
