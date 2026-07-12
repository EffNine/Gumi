# Novexa

**Intelligence Runtime for Local AI**

Run local models with a cleaner OpenAI-compatible API, validated model profiles,
provider-specific fixes, telemetry, and reliability guardrails.

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

- OpenAI-compatible `/v1/chat/completions` (streaming and non-streaming) (streaming and non-streaming)
- local provider adapters
- model profiles
- runtime modes
- prompt and context handling
- JSON validation and repair
- anti-loop and safety guards
- instruction-following assist (auto-detects 14 constraint types)
- local telemetry
- agent mode (step budget enforcement, tool-call loop detection, tool-call JSON validation, context compaction hints)
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
docker build -t novexa:0.1.0-alpha .
docker run -d --name novexa \
  -p 127.0.0.1:8787:8787 \
  -p 127.0.0.1:8788:8788 \
  -v novexa-data:/data \
  novexa:0.1.0-alpha
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

For LM Studio:

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
- LM Studio
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
- [LM Studio setup](./docs/integrations/lmstudio.md)

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

Novexa `0.1.0-alpha` is usable, but not feature-complete:

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

## License

Novexa is licensed under the Apache License 2.0. See [LICENSE](./LICENSE).
