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

- OpenAI-compatible `/v1/chat/completions`
- local provider adapters
- model profiles
- runtime modes
- prompt and context handling
- JSON validation and repair
- anti-loop and safety guards
- local telemetry
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
| Fast chat | `lmstudio:qwen/qwen3-1.7b` | `qwen3-1.7b` |
| Mid-size chat | `lmstudio:google/gemma-4-e4b` | `gemma-4-e4b` |
| Quality alternative | `lmstudio:ornith-1.0-9b@q4_k_m` | `ornith-1.0-9b-q4-km` |

Apps should only need:

```text
base_url: http://127.0.0.1:8787/v1
api_key: novexa-local
model: lmstudio:qwen2.5-coder-7b-instruct
```

Novexa handles profile tuning, thinking policy, provider quirks, JSON handling,
and runtime behavior.

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
```

Not implemented yet:

```bash
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

- YAML configuration loading is not implemented yet; the runtime uses safe
  defaults and environment overrides.
- `novexa stop` and `novexa restart` are not implemented yet.
- Streaming responses are not implemented yet.
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
