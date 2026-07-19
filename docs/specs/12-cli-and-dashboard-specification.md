# Gumi CLI & Dashboard Specification

Version: 1.0  
Status: Draft  
Scope: CLI commands, local dashboard, diagnostics, telemetry UX, and developer control surface for Gumi Runtime V1

---

# 1. Purpose

This document defines the CLI and Dashboard experience for Gumi Runtime.

The CLI and Dashboard are the main developer control surfaces.

They help users:

- start Gumi
- inspect runtime status
- diagnose provider problems
- view local telemetry
- inspect model profiles
- test providers
- debug failed requests
- understand what Gumi changed during a request

Gumi should feel simple from the outside and transparent inside.

---

# 2. Core Philosophy

Gumi should be easy to start and easy to understand.

The ideal first-run experience:

```bash
gumi start
```

Then open:

```text
http://localhost:8788
```

A developer should immediately see:

- runtime is running
- provider is connected or not
- default model is available or missing
- API endpoint is ready
- how to connect an OpenAI-compatible app

---

# 3. CLI Goals

The CLI must be:

- predictable
- fast
- helpful
- scriptable
- local-first
- readable
- friendly to beginners
- useful for advanced users

CLI output should prefer clear status and suggestions over cryptic errors.

---

# 4. Dashboard Goals

The Dashboard must be:

- local-only by default
- privacy-first
- useful without cloud login
- focused on observability
- fast to load
- clear for debugging
- useful while apps are actively using Gumi

Dashboard should not require account creation in V1.

---

# 5. CLI Command Overview

V1 CLI commands:

```bash
gumi start
gumi stop
gumi restart
gumi status
gumi doctor
gumi config show
gumi providers
gumi models
gumi benchmark
gumi logs
gumi version
gumi lmstudio status
gumi lmstudio load <model>
gumi lmstudio unload <instance-id>
gumi lmstudio models
gumi memory status
gumi memory facts [search]
gumi memory clear --force
```

The `lmstudio` subcommands manage LM Studio model lifecycle (load, unload,\nlist available models). See [06-provider-adapter-specification.md §22](./06-provider-adapter-specification.md).\n\nThe `memory` subcommands inspect and manage the memory engine database (facts,\nmodel fit data, reset). See [20-memory-engine-specification.md](./20-memory-engine-specification.md).\nBoth command groups support `--json` for machine-readable output.

Future commands:

```bash
gumi profile list
gumi profile show
gumi profile validate
gumi plugin list
gumi plugin enable
gumi plugin disable
gumi trace
gumi session list
gumi workspace create
```

---

# 6. CLI Global Flags

All commands should support:

```bash
--config <path>
--json
--verbose
--quiet
--no-color
```

## 6.1 Flag Behaviour

| Flag | Purpose |
|---|---|
| `--config` | Use custom config path |
| `--json` | Output machine-readable JSON |
| `--verbose` | Show detailed diagnostic output |
| `--quiet` | Show minimal output |
| `--no-color` | Disable terminal colors |

---

# 7. gumi start

## 7.1 Purpose

Starts the Gumi Runtime.

```bash
gumi start
```

## 7.2 Default Behaviour

Starts:

- API server on `127.0.0.1:8787`
- dashboard on `127.0.0.1:8788`
- provider health checks
- local telemetry storage
- configured engines

## 7.3 Example Output

```text
Gumi Runtime 0.1.0

API        http://127.0.0.1:8787/v1
Dashboard  http://127.0.0.1:8788
Mode       stabilized
Provider   ollama
Model      qwen3:8b

Status     ready

Use with OpenAI-compatible clients:

export OPENAI_BASE_URL=http://127.0.0.1:8787/v1
export OPENAI_API_KEY=gumi-local
```

## 7.4 Useful Flags

```bash
gumi start --port 8787
gumi start --dashboard-port 8788
gumi start --provider ollama
gumi start --model qwen3:8b
gumi start --mode direct
gumi start --config ./gumi.yaml
```

## 7.5 Failure Example

```text
Gumi could not start.

Reason:
- API port 8787 is already in use.

Suggestion:
- Stop the process using port 8787.
- Or run: gumi start --port 8790
```

---

# 8. gumi stop

## 8.1 Purpose

Stops a running Gumi Runtime.

```bash
gumi stop
```

## 8.2 Behaviour

- stop API server
- stop dashboard
- flush telemetry
- gracefully shutdown plugins
- release lock file

## 8.3 Example Output

```text
Gumi stopped successfully.
```

---

# 9. gumi restart

## 9.1 Purpose

Restarts the Gumi Runtime.

```bash
gumi restart
```

Equivalent to:

```bash
gumi stop
gumi start
```

Should preserve config and telemetry.

---

# 10. gumi status

## 10.1 Purpose

Displays current runtime status.

```bash
gumi status
```

## 10.2 Example Output

```text
Gumi Status

Runtime
  Status:   running
  Version:  0.1.0
  Mode:     stabilized
  Uptime:   1h 14m

API
  URL:      http://127.0.0.1:8787/v1
  Status:   ok

Dashboard
  URL:      http://127.0.0.1:8788
  Status:   ok

Provider
  Default:  ollama
  Status:   ok
  URL:      http://localhost:11434
  Model:    qwen3:8b
```

## 10.3 JSON Output

```bash
gumi status --json
```

```json
{
  "runtime": {
    "status": "running",
    "version": "0.1.0",
    "mode": "stabilized",
    "uptime_seconds": 4440
  },
  "api": {
    "url": "http://127.0.0.1:8787/v1",
    "status": "ok"
  },
  "dashboard": {
    "url": "http://127.0.0.1:8788",
    "status": "ok"
  },
  "provider": {
    "default": "ollama",
    "status": "ok",
    "url": "http://localhost:11434",
    "model": "qwen3:8b"
  }
}
```

---

# 11. gumi doctor

## 11.1 Purpose

Runs diagnostics and suggests fixes.

```bash
gumi doctor
```

Doctor should be one of the most useful CLI commands.

It must check:

- config validity
- API port availability
- dashboard port availability
- provider reachability
- default model availability
- model profile availability
- telemetry database writability
- plugin validity
- local-only safety settings

## 11.2 Example Output

```text
Gumi Doctor

Runtime Config        ok
API Port 8787         ok
Dashboard Port 8788   ok
Ollama Provider       ok
Default Model         warning
Model Profile         ok
Telemetry Storage     ok
Plugin Directory      ok

Warnings:
- Default model qwen3:8b is not installed in Ollama.

Suggestion:
- Run: ollama pull qwen3:8b
- Or update providers.ollama.default_model in gumi.yaml
```

## 11.3 JSON Output

```json
{
  "status": "warning",
  "checks": [
    {
      "name": "runtime_config",
      "status": "ok",
      "message": "Config is valid."
    },
    {
      "name": "default_model",
      "status": "warning",
      "message": "Default model qwen3:8b is not installed.",
      "suggestion": "Run: ollama pull qwen3:8b"
    }
  ]
}
```

---

# 12. gumi config show

## 12.1 Purpose

Shows resolved runtime configuration.

```bash
gumi config show
```

## 12.2 Behaviour

Displays final merged config after applying:

```text
defaults
global config
workspace config
environment variables
CLI flags
```

## 12.3 Security Rule

Sensitive values must be redacted.

Example:

```yaml
providers:
  openai_compatible_local:
    api_key: "***REDACTED***"
```

---

# 13. gumi providers

## 13.1 Purpose

Lists configured providers and health.

```bash
gumi providers
```

## 13.2 Example Output

```text
Providers

ollama
  Status:   ok
  URL:      http://localhost:11434
  Models:   4
  Default:  qwen3:8b

lmstudio
  Status:   offline
  URL:      http://localhost:1234/v1
  Models:   unknown
```

## 13.3 Future Subcommands

```bash
gumi providers test ollama
gumi providers refresh
```

---

# 14. gumi models

## 14.1 Purpose

Lists available models across providers.

```bash
gumi models
```

## 14.2 Example Output

```text
Models

local:auto
  Provider: gumi
  Status:   virtual

ollama:qwen3:8b
  Provider: ollama
  Profile:  qwen3-8b
  Status:   available

ollama:deepseek-r1:8b
  Provider: ollama
  Profile:  deepseek-r1-8b
  Status:   available
```

## 14.3 Useful Flags

```bash
gumi models --provider ollama
gumi models --profiles
gumi models --json
```

---

# 15. gumi benchmark

## 15.1 Purpose

Runs simple local benchmarks.

```bash
gumi benchmark
```

Benchmark should help users understand:

- provider latency
- model response speed
- runtime overhead
- context engine overhead
- validation overhead
- streaming behaviour

## 15.2 Example Output

```text
Gumi Benchmark

Provider: ollama
Model:    qwen3:8b
Mode:     stabilized

Runtime Overhead
  Gateway:      2ms
  Context:      8ms
  Prompt:       3ms
  Validation:   2ms
  Total:        15ms

Provider
  First Token:  480ms
  Total:        1840ms
  Tokens/sec:   42.1
```

## 15.3 Subcommands

```bash
gumi benchmark provider
gumi benchmark context
gumi benchmark prompt
gumi benchmark validation
gumi benchmark full
```

---

# 16. gumi logs

## 16.1 Purpose

Shows local runtime logs.

```bash
gumi logs
```

Useful flags:

```bash
gumi logs --tail
gumi logs --follow
gumi logs --level warning
gumi logs --request req_abc123
```

## 16.2 Privacy Rule

Logs must not include full prompts or responses unless enabled.

---

# 17. gumi version

## 17.1 Purpose

Shows version information.

```bash
gumi version
```

Example:

```text
Gumi Runtime: 0.1.0
Config Schema: 1
Plugin Schema: 1
API Version: v1
```

---

# 18. Future CLI: gumi trace

## 18.1 Purpose

Inspects one request lifecycle.

```bash
gumi trace req_abc123
```

Example:

```text
Trace req_abc123

Mode:      stabilized
Provider:  ollama
Model:     qwen3:8b
Status:    success

Timeline:
0ms     request_received
2ms     config_resolved
5ms     context_started
13ms    context_completed
16ms    prompt_completed
18ms    provider_request_started
842ms   provider_request_completed
846ms   validation_completed
849ms   telemetry_recorded

Events:
- context_compressed
- model_profile_applied
- validation_passed
```

This is future but should influence telemetry design now.

---

# 19. Dashboard Overview

Default dashboard URL:

```text
http://127.0.0.1:8788
```

Dashboard should be enabled by default.

It should bind only to localhost unless explicitly configured otherwise.

---

# 20. Dashboard Navigation

V1 dashboard sections (11 pages, all implemented):

```text
Overview      — Runtime status, pipeline visualization, provider health, recent activity
Playground    — Interactive chat with provider/model/mode selection
Requests      — Request history table with filtering and status indicators
Analytics     — Latency distribution, provider breakdown, success rate, trends
Providers     — Provider status cards with health indicators
Models        — Model listing with load/unload and configuration
Memory        — Facts CRUD, model-fit leaderboard, memory engine status
Profiles      — Model profile listing and details
Logs          — Real-time log streaming via SSE with level filtering
Config        — Resolved config viewer with redacted secrets
Doctor        — Visual diagnostic checks with suggestions
```

---

# 21. Dashboard: Overview Page

## 21.1 Purpose

Shows system status at a glance.

Should display:

- runtime status
- API URL
- dashboard URL
- runtime mode
- default provider
- default model
- provider health
- recent request count
- error count
- average latency

## 21.2 Example Cards

```text
Runtime
  Running
  Mode: stabilized
  Uptime: 1h 14m

Provider
  Ollama
  Status: ok
  Model: qwen3:8b

Requests
  Last 15 min: 42
  Errors: 1
  Avg Latency: 1.2s

Stability
  Repairs: 3
  Retries: 1
  Validation failures: 3
```

## 21.3 Pipeline visualization

The Overview page includes a visual pipeline diagram showing the active
processing stages:

```text
Gateway → Context → Prompt → Provider → Validate → Repair
```

Each stage lights up green when active and shows mode badges (compressed,
repaired, retried, validated).

---

# 22. Dashboard: Playground Page

## 22.1 Purpose

Interactive chat interface for testing providers and models.

Layout:

```text
+------------------------------------------+
|  [Provider ▼] [Model ▼] [Mode ▼] [Send]  |
+------------------------------------------+
|                                          |
|  Chat messages (user + assistant)        |
|                                          |
|  [text input]                    [send]  |
+------------------------------------------+
```

Features:

- **Provider selector**: Dropdown of configured providers (ollama, lmstudio, etc.)
- **Model selector**: Models available from the selected provider
- **Mode selector**: Pipeline mode (direct, lightweight, stabilized, structured, agent)
- **Real-time streaming**: Responses appear token-by-token via SSE
- **Message history**: Conversation persisted in session state
- Example: Provider dropdown selects "lmstudio" → model list filters to
  LM Studio models only → response shows provider prefix resolution

## 22.2 Provider-Model Synchronization

When the user changes the provider, the model list updates to show only models
available from that provider. The resolved model ID follows the
`provider:model_name` convention. If the selected model doesn't match the
current provider, a fallback model is auto-selected.

---

# 23. Dashboard: Requests Page

## 23.1 Purpose

Shows recent requests.

Fields:

```text
timestamp
request_id
mode
provider
model
latency
status
validation
repair
retry_count
```

## 23.2 Privacy Rule

Do not show full prompt/response by default.

If detailed logging is disabled, show metadata only.

---

# 24. Dashboard: Request Detail Page

## 24.1 Purpose

Shows explainable lifecycle for one request.

Sections:

```text
Summary
Timeline
Pipeline Events
Provider Info
Context Report
Prompt Report
Validation Report
Repair Report
Errors/Warnings
```

## 24.2 Sensitive Content

Full prompt and response should only appear if:

```yaml
telemetry:
  log_prompts: true
  log_responses: true
```

Otherwise show:

```text
Prompt hidden by privacy settings.
Response hidden by privacy settings.
```

---

# 25. Dashboard: Analytics Page

## 25.1 Purpose

Client-side computed analytics from telemetry data.

Metrics displayed:

- Total request count
- Success rate (percentage)
- Average latency (ms)
- Repair rate (percentage)

Visualizations:

- **Latency distribution**: Bar chart with bins (0–100ms, 100–500ms, 500–1000ms, 1000–2000ms, 2000–5000ms, 5000ms+)
- **Provider breakdown**: Per-provider request count, average latency, and percentage share
- **Recent trends**: Last 10 requests with model, provider, latency, and status indicators

Data source: `GET /v1/gumi/telemetry/recent` — all analytics computed client-side.

---

# 26. Dashboard: Providers Page

## 26.1 Purpose

Shows provider health and models.

Fields:

```text
provider name
status
url
latency
models count
default model
streaming support
last checked
```

Actions:

```text
refresh health
test provider
copy config example
```

---

# 27. Dashboard: Models Page

## 27.1 Purpose

Shows available models with load/unload management.

Fields:

```text
model ID
provider
profile
status
capabilities
context limit
recommended use
known weaknesses
```

Useful labels:

```text
Good for coding
Good for structured output
High repetition risk
No profile found
```

Actions:

```text
Load model (with custom config: context length, flash attention, GPU offload)
Unload model instance
Refresh model list
View model details
```

For LM Studio, model loading supports:

```text
context_length
flash_attention
offload_kv_cache
eval_batch_size
num_experts
```

---

# 28. Dashboard: Memory Page

## 28.1 Purpose

Inspect and manage the memory engine database.

Tabs:

- **Facts**: Browse existing facts (key, value, confidence, access count). Add new facts via inline form.
- **Model Fit**: Table of per-model performance (model_id, task_type, difficulty, attempts, successes, avg latency).
- **Status**: Memory engine status (enabled/disabled, facts count, model-fit records, injection budget).

Actions:

```text
Refresh data
Clear all memory (with confirmation dialog)
Create a new fact
```

API endpoints:

```http
GET  /v1/gumi/memory/facts       # list stored facts
POST /v1/gumi/memory/facts       # create a fact (body: {key, value, source?, confidence?})
GET  /v1/gumi/memory/model-fit   # model performance data
POST /v1/gumi/memory/clear       # clear all memory
GET  /v1/gumi/memory/status      # memory engine status
```

---

# 29. Dashboard: Profiles Page

## 29.1 Purpose

Shows model profile details.

Fields:

```text
profile ID
family
size
capabilities
defaults
context strategy
prompt style
guard settings
known weaknesses
```

Actions:

```text
validate profile
copy profile
open profile file
```

---

# 30. Dashboard: Logs Page

## 30.1 Purpose

Real-time log viewer with SSE streaming.

Features:

- Live streaming from `GET /v1/gumi/logs/stream` (Server-Sent Events)
- Level filter toggle (all, info, warn, error)
- Auto-scroll to latest entries
- Color-coded log levels

Log entries display:

```text
timestamp
level (colored badge)
engine source
request_id
message
```

---

# 31. Dashboard: Config Page

## 31.1 Purpose

Displays resolved config.

Rules:

- redact secrets
- show config source
- show overrides
- show warnings

Example:

```text
runtime.mode = stabilized
source: ./gumi.yaml

providers.ollama.url = http://localhost:11434
source: default config
```

Action:

```text
Save config to disk (POST /v1/gumi/config/save)
```

---

# 32. Dashboard: Doctor Page

## 32.1 Purpose

Visual version of `gumi doctor`.

Checks:

```text
runtime
ports
config
providers
models
profiles
telemetry
plugins
local safety
```

Each check should include:

- status
- message
- suggestion

---

# 33. Dashboard API Dependencies

Dashboard uses the following local Gumi APIs:

```http
GET  /v1/gumi/status                # overview, runtime status
GET  /v1/gumi/providers             # overview, providers
GET  /v1/gumi/config                # config page
POST /v1/gumi/config/save           # config page (save action)
GET  /v1/gumi/telemetry/recent      # requests, analytics
POST /v1/gumi/doctor                # doctor page
GET  /v1/gumi/profiles              # profiles page
POST /v1/gumi/profiles/test         # profiles page (test action)
GET  /v1/gumi/memory/facts          # memory page
POST /v1/gumi/memory/facts          # memory page (create fact)
GET  /v1/gumi/memory/model-fit      # memory page
POST /v1/gumi/memory/clear          # memory page (clear action)
GET  /v1/gumi/memory/status         # memory page
GET  /v1/gumi/lmstudio/models       # models page
POST /v1/gumi/lmstudio/models/load  # models page (load action)
POST /v1/gumi/lmstudio/models/unload# models page (unload action)
GET  /v1/gumi/logs/stream           # logs page (SSE stream)
```

Future:

```http
GET /v1/gumi/trace/{request_id}
```

---

# 34. Dashboard Security

Default:

```yaml
dashboard:
  host: 127.0.0.1
```

Dashboard should not expose publicly unless user configures:

```yaml
dashboard:
  host: 0.0.0.0
```

If user binds dashboard publicly, Gumi should warn.

---

# 35. Dashboard Privacy

Dashboard must not display:

- full prompts by default
- full responses by default
- API keys
- provider tokens
- secrets from config
- raw memory content by default

---

# 36. First-Run UX

When user runs:

```bash
gumi start
```

If Ollama is unavailable:

```text
Gumi started, but Ollama is not reachable.

API:        http://127.0.0.1:8787/v1
Dashboard: http://127.0.0.1:8788

Provider issue:
- Ollama is not reachable at http://localhost:11434.

Fix:
- Start Ollama.
- Or configure another provider in gumi.yaml.
```

Gumi should start in degraded mode if possible so Dashboard can still show the problem.

---

# 37. Local App Integration Helper

CLI should show integration instructions.

Example:

```bash
gumi status
```

Output includes:

```text
OpenAI-compatible setup:

OPENAI_BASE_URL=http://127.0.0.1:8787/v1
OPENAI_API_KEY=gumi-local
```

Dashboard should include copy buttons for:

- Python client
- JavaScript client
- cURL
- Continue config
- Cline config
- Open WebUI config

---

# 38. Runtime Status Values

Runtime status values:

```text
starting
running
degraded
stopping
stopped
error
```

Provider status values:

```text
ok
offline
degraded
misconfigured
unknown
```

Request status values:

```text
success
failed
repaired
retried
streaming
cancelled
```

---

# 39. Diagnostics Quality Bar

Every diagnostic message should include:

```text
what happened
why it matters
how to fix it
```

Bad:

```text
Provider failed.
```

Good:

```text
Ollama is not reachable at http://localhost:11434.
Gumi cannot generate responses through Ollama until it is running.
Start Ollama or change providers.ollama.url in gumi.yaml.
```

---

# 40. Telemetry Storage for UI

V1 can store telemetry in SQLite.

Suggested tables are defined elsewhere in storage specification.

Dashboard should query recent telemetry through API, not read database directly.

---

# 41. CLI and Dashboard Testing Requirements

CLI tests:

- start command config resolution
- status command when running
- status command when stopped
- doctor warnings
- provider list output
- models output
- JSON output mode
- redaction of secrets
- port conflict error
- missing provider suggestion

Dashboard tests:

- overview loads
- providers page shows provider state
- requests page hides prompt by default
- config page redacts secrets
- doctor page shows suggestions
- telemetry page handles empty state
- degraded provider state displayed clearly

---

# 42. V1 Implementation Priority

Implement in this order:

```text
1. CLI version
2. CLI start
3. CLI status
4. CLI doctor
5. CLI config show
6. CLI providers
7. CLI models
8. Basic dashboard shell
9. Dashboard overview
10. Dashboard providers
11. Dashboard recent requests
12. Dashboard config
13. Dashboard doctor
14. Dashboard logs
15. Dashboard playground
16. Dashboard analytics
17. Dashboard memory
18. Benchmark command
19. Logs command
```

CLI must be useful before dashboard becomes polished.

---

# 43. Anti-Patterns

Avoid:

```text
Dashboard requiring cloud login
Dashboard exposing prompts by default
```

---

# 21. Dashboard: Overview Page

## 21.1 Purpose

Shows system status at a glance.

Should display:

- runtime status
- API URL
- dashboard URL
- runtime mode
- default provider
- default model
- provider health
- recent request count
- error count
- average latency

## 21.2 Example Cards

```text
Runtime
  Running
  Mode: stabilized
  Uptime: 1h 14m

Provider
  Ollama
  Status: ok
  Model: qwen3:8b

Requests
  Last 15 min: 42
  Errors: 1
  Avg Latency: 1.2s

Stability
  Repairs: 3
  Retries: 1
  Validation failures: 3
```

---

# 22. Dashboard: Requests Page

## 22.1 Purpose

Shows recent requests.

Fields:

```text
timestamp
request_id
mode
provider
model
latency
status
validation
repair
retry_count
```

## 22.2 Privacy Rule

Do not show full prompt/response by default.

If detailed logging is disabled, show metadata only.

---

# 23. Dashboard: Request Detail Page

## 23.1 Purpose

Shows explainable lifecycle for one request.

Sections:

```text
Summary
Timeline
Pipeline Events
Provider Info
Context Report
Prompt Report
Validation Report
Repair Report
Errors/Warnings
```

## 23.2 Sensitive Content

Full prompt and response should only appear if:

```yaml
telemetry:
  log_prompts: true
  log_responses: true
```

Otherwise show:

```text
Prompt hidden by privacy settings.
Response hidden by privacy settings.
```

---

# 24. Dashboard: Providers Page

## 24.1 Purpose

Shows provider health and models.

Fields:

```text
provider name
status
url
latency
models count
default model
streaming support
last checked
```

Actions:

```text
refresh health
test provider
copy config example
```

---

# 25. Dashboard: Models Page

## 25.1 Purpose

Shows available models.

Fields:

```text
model ID
provider
profile
status
capabilities
context limit
recommended use
known weaknesses
```

Useful labels:

```text
Good for coding
Good for structured output
High repetition risk
No profile found
```

---

# 26. Dashboard: Profiles Page

## 26.1 Purpose

Shows model profile details.

Fields:

```text
profile ID
family
size
capabilities
defaults
context strategy
prompt style
guard settings
known weaknesses
```

Actions:

```text
validate profile
copy profile
open profile file
```

---

# 27. Dashboard: Telemetry Page

## 27.1 Purpose

Shows local observability metrics.

Metrics:

```text
request_count
success_rate
error_rate
average_latency
provider_latency
runtime_overhead
validation_failures
repair_count
retry_count
context_compressions
loop_detections
```

Charts can be simple in V1.

No cloud analytics required.

---

# 28. Dashboard: Config Page

## 28.1 Purpose

Displays resolved config.

Rules:

- redact secrets
- show config source
- show overrides
- show warnings

Example:

```text
runtime.mode = stabilized
source: ./gumi.yaml

providers.ollama.url = http://localhost:11434
source: default config
```

---

# 29. Dashboard: Doctor Page

## 29.1 Purpose

Visual version of `gumi doctor`.

Checks:

```text
runtime
ports
config
providers
models
profiles
telemetry
plugins
local safety
```

Each check should include:

- status
- message
- suggestion

---

# 30. Dashboard: Logs Page

## 30.1 Purpose

Shows recent local logs.

Filters:

```text
level
engine
request_id
provider
time range
```

Privacy rule applies.

---

# 31. Dashboard API Dependencies

Dashboard should use local Gumi APIs:

```http
GET /v1/gumi/status
GET /v1/gumi/providers
GET /v1/gumi/config
GET /v1/gumi/telemetry/recent
POST /v1/gumi/doctor
```

Future:

```http
GET /v1/gumi/trace/{request_id}
GET /v1/gumi/profiles
GET /v1/gumi/plugins
```

---

# 32. Dashboard Security

Default:

```yaml
dashboard:
  host: 127.0.0.1
```

Dashboard should not expose publicly unless user configures:

```yaml
dashboard:
  host: 0.0.0.0
```

If user binds dashboard publicly, Gumi should warn.

---

# 33. Dashboard Privacy

Dashboard must not display:

- full prompts by default
- full responses by default
- API keys
- provider tokens
- secrets from config
- raw memory content by default

---

# 34. First-Run UX

When user runs:

```bash
gumi start
```

If Ollama is unavailable:

```text
Gumi started, but Ollama is not reachable.

API:        http://127.0.0.1:8787/v1
Dashboard: http://127.0.0.1:8788

Provider issue:
- Ollama is not reachable at http://localhost:11434.

Fix:
- Start Ollama.
- Or configure another provider in gumi.yaml.
```

Gumi should start in degraded mode if possible so Dashboard can still show the problem.

---

# 35. Local App Integration Helper

CLI should show integration instructions.

Example:

```bash
gumi status
```

Output includes:

```text
OpenAI-compatible setup:

OPENAI_BASE_URL=http://127.0.0.1:8787/v1
OPENAI_API_KEY=gumi-local
```

Dashboard should include copy buttons for:

- Python client
- JavaScript client
- cURL
- Continue config
- Cline config
- Open WebUI config

---

# 36. Runtime Status Values

Runtime status values:

```text
starting
running
degraded
stopping
stopped
error
```

Provider status values:

```text
ok
offline
degraded
misconfigured
unknown
```

Request status values:

```text
success
failed
repaired
retried
streaming
cancelled
```

---

# 37. Diagnostics Quality Bar

Every diagnostic message should include:

```text
what happened
why it matters
how to fix it
```

Bad:

```text
Provider failed.
```

Good:

```text
Ollama is not reachable at http://localhost:11434.
Gumi cannot generate responses through Ollama until it is running.
Start Ollama or change providers.ollama.url in gumi.yaml.
```

---

# 38. Telemetry Storage for UI

V1 can store telemetry in SQLite.

Suggested tables are defined elsewhere in storage specification.

Dashboard should query recent telemetry through API, not read database directly.

---

# 39. CLI and Dashboard Testing Requirements

CLI tests:

- start command config resolution
- status command when running
- status command when stopped
- doctor warnings
- provider list output
- models output
- JSON output mode
- redaction of secrets
- port conflict error
- missing provider suggestion

Dashboard tests:

- overview loads
- providers page shows provider state
- requests page hides prompt by default
- config page redacts secrets
- doctor page shows suggestions
- telemetry page handles empty state
- degraded provider state displayed clearly

---

# 40. V1 Implementation Priority

Implement in this order:

```text
1. CLI version
2. CLI start
3. CLI status
4. CLI doctor
5. CLI config show
6. CLI providers
7. CLI models
8. Basic dashboard shell
9. Dashboard overview
10. Dashboard providers
11. Dashboard recent requests
12. Dashboard config
13. Dashboard doctor
14. Benchmark command
15. Logs command
```

Reason:

CLI must be useful before dashboard becomes polished.

---

# 41. Anti-Patterns

Avoid:

```text
Dashboard requiring cloud login
Dashboard exposing prompts by default
CLI showing cryptic provider errors
CLI requiring config for first run
Binding dashboard publicly by default
Hiding degraded provider status
Showing secrets in config output
Making benchmark too complex in V1
Dashboard reading database directly
```

---

# 42. Final CLI & Dashboard Statement

The CLI starts and controls Gumi.

The Dashboard explains Gumi.

Together, they make the runtime feel trustworthy.

A good local AI runtime should not behave like a black box.

Gumi should show what happened, why it happened, and how to fix it when something breaks.