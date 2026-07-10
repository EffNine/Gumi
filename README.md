
# Novexa

**Intelligence Runtime for Local AI**

Run any local model like it's a premium AI.

---

## What is Novexa?

Novexa is a local-first AI runtime that sits between AI applications and local inference engines.

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

Novexa is not a model.

Novexa is not a chatbot.

Novexa is not a cloud gateway in V1.

Novexa is the runtime layer around local AI.

---

## Why Novexa?

Local AI is powerful, private, and cheap to run.

But local models often feel fragile:

- hallucinated answers
- repeated output
- broken JSON
- weak instruction following
- context overflow
- poor debugging visibility
- model-specific tuning headaches
- inconsistent provider behaviour

Novexa improves the experience around local models instead of replacing them.

```text
Application
    ↓
Novexa Runtime
    ↓
Ollama / LM Studio / OpenAI-compatible Local Server
    ↓
Local Model
```

A proxy forwards requests.

Novexa manages the lifecycle around the request.

---

## Core Promise

A developer should be able to run:

```bash
novexa start
```

Then point an OpenAI-compatible app to:

```text
http://localhost:8787/v1
```

And use local models with better stability, cleaner output, telemetry, and diagnostics.

---

## Get Started (0.1.0 Alpha)

```bash
# Build from source
git clone https://github.com/novexa/novexa.git
cd novexa
make build
./novexa start
```

Or download a pre-built release archive from the
[GitHub releases](https://github.com/novexa/novexa/releases) page.

Then open the dashboard at http://127.0.0.1:8788 and point an OpenAI-compatible
client at http://127.0.0.1:8787/v1 with API key `novexa-local`.

See [docs/installation.md](./docs/installation.md) and
[docs/quickstart.md](./docs/quickstart.md) for platform-specific instructions.

### Alpha limitations

- YAML configuration loading is not implemented yet; the runtime uses safe
  hard-coded defaults.
- `novexa stop` and `novexa restart` are not implemented yet.
- Streaming responses are not implemented yet.

---

## V1 Focus

Novexa V1 is local-only.

No cloud billing.

No hosted inference.

No user accounts.

No team accounts.

No marketplace.

No external AI dependency.

V1 focuses on making local models easier and safer to use in real applications.

---

## Supported Providers

Planned V1 providers:

- Ollama
- LM Studio
- OpenAI-compatible local servers

Future providers:

- llama.cpp server
- vLLM
- SGLang
- Text Generation Inference
- LocalAI
- KoboldCpp

---

## OpenAI-Compatible API

Default local API:

```text
http://localhost:8787/v1
```

Default local dashboard:

```text
http://localhost:8788
```

Example environment variables:

```bash
export OPENAI_BASE_URL=http://localhost:8787/v1
export OPENAI_API_KEY=novexa-local
```

Example Python usage:

```python
from openai import OpenAI

client = OpenAI(
    base_url="http://localhost:8787/v1",
    api_key="novexa-local",
)

response = client.chat.completions.create(
    model="local:auto",
    messages=[
        {"role": "user", "content": "Explain local AI runtime."}
    ],
)

print(response.choices[0].message.content)
```

---

## Runtime Architecture

Novexa is designed as a modular monolith.

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

Every request must pass through the Pipeline Engine.

```text
Gateway Engine
    ↓
Pipeline Engine
    ↓
Provider Engine
```

This rule keeps Novexa observable, configurable, and extensible.

---

## Intelligence Pipeline

Default stabilized flow:

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

## Runtime Modes

Novexa supports multiple runtime modes.

| Mode | Purpose |
|---|---|
| `direct` | Minimal processing for raw provider behaviour |
| `stabilized` | Default mode with context, prompt, validation, repair, and telemetry |
| `structured` | Strict JSON/schema output mode |
| `agent` | Future mode for coding agents and tool workflows |

V1 implements:

- direct
- stabilized
- structured

---

## Model Profiles

Model Profiles are one of Novexa's key features.

They define model-specific runtime behaviour:

- temperature
- top_p
- repeat penalty
- context strategy
- prompt style
- structured output reliability
- anti-loop behaviour
- known weaknesses
- preferred tasks

Starter profiles planned for V1:

- generic-local
- qwen3-8b
- qwen2.5-coder-7b
- deepseek-r1-8b
- llama3.1-8b
- gemma3-12b
- mistral-small

If no matching profile exists, Novexa uses `generic-local`.

---

## Stability Features

Novexa aims to improve reliability through:

- empty prompt guard
- context overflow guard
- structured output guard
- anti-loop guard
- retry budget guard
- empty response validation
- JSON validation
- JSON schema validation
- repetition detection
- local JSON repair
- regex cleanup
- retry generation

Novexa does not claim to eliminate hallucination completely.

It can detect risk signals and make failures easier to see and handle.

---

## Telemetry and Privacy

Novexa is local-first and privacy-first.

Default telemetry behaviour:

```yaml
telemetry:
  local: true
  external: false
  log_prompts: false
  log_responses: false
```

By default, Novexa stores metadata only.

It does not store full prompts or full responses unless explicitly configured.

It does not send external telemetry in V1.

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

Not yet implemented:

```bash
novexa stop
novexa restart
```

Most important first-run commands:

```bash
novexa start
novexa doctor
novexa status
```

---

## Dashboard

Default dashboard:

```text
http://127.0.0.1:8788
```

Planned dashboard sections:

- Overview
- Requests
- Providers
- Models
- Profiles
- Telemetry
- Config
- Doctor
- Logs

Dashboard is local-only by default.

Secrets are redacted.

Prompts and responses are hidden by default.

---

## Repository Structure

Target structure:

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

## Implementation Roadmap

Sprint order:

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

## MVP Cutline

If development needs to be reduced, cut:

- plugin execution
- advanced dashboard pages
- full model profile pack
- benchmark command
- session persistence
- memory engine
- markdown validation
- streaming repair

Do not cut:

- OpenAI compatibility
- Ollama support
- Pipeline Engine
- telemetry metadata
- JSON validation and repair
- doctor command

---

## Documentation

Planning documents:

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

## Development Rules

Core rules:

1. Keep Novexa local-first.
2. Do not add cloud providers in V1.
3. Do not add billing in V1.
4. Do not implement microservices in V1.
5. Do not bypass the Pipeline Engine.
6. Keep provider adapters thin.
7. Do not store prompts by default.
8. Do not store responses by default.
9. Do not send external telemetry by default.
10. Keep the runtime modular and observable.

---

## Current Status

Sprints 1 through 10 are complete. Novexa is packaged as a 0.1.0 alpha release
with cross-platform binaries, a Docker image, and installation documentation.

The runtime exposes an OpenAI-compatible local HTTP gateway at:

```text
http://127.0.0.1:8787/v1
```

The local dashboard is available at:

```text
http://127.0.0.1:8788
```

Supported release targets:

- macOS arm64
- macOS amd64
- Linux amd64
- Linux arm64
- Windows amd64

The next step is to test the alpha and prepare for the first stable V1 release.

---

## First Implementation Prompt

Use this with an AI coding agent:

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

## License

License not selected yet.

Recommended options:

- MIT for maximum adoption
- Apache-2.0 for stronger patent language

Decision pending.

---

## Final Statement

Novexa should be built like infrastructure, not like a weekend chatbot.

The first goal is not to build every possible AI feature.

The first goal is to build a clean, local-first, OpenAI-compatible runtime that makes local AI easier to integrate, observe, and stabilize.

The lifecycle around the request is the product.
