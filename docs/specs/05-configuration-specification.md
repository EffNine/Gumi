# Gumi Configuration Specification

Version: 1.0  
Status: Draft  
Scope: Runtime configuration contract for Gumi V1

---

# 1. Purpose

This document defines the configuration system for Gumi Runtime.

The configuration system controls:

- runtime behaviour
- provider selection
- model defaults
- engine settings
- dashboard settings
- telemetry settings
- authentication
- plugin loading
- model profiles

Gumi must work with zero configuration, but advanced users must be able to control runtime behaviour explicitly.

---

# 2. Configuration Philosophy

Gumi configuration should be:

- readable
- explicit
- predictable
- local-first
- safe by default
- easy to override
- easy to inspect

A user should be able to run:

```bash
gumi start
```

without creating any config file.

Advanced users can create:

```text
gumi.yaml
```

---

# 3. Configuration File Location

Gumi should search for config in this order:

```text
1. --config CLI argument
2. NOVEXA_CONFIG environment variable
3. ./gumi.yaml
4. ~/.gumi/gumi.yaml
5. runtime defaults
```

Example:

```bash
gumi start --config ./gumi.yaml
```

Example:

```bash
export NOVEXA_CONFIG=/Users/afnan/.gumi/gumi.yaml
gumi start
```

---

# 4. Configuration Precedence

Final configuration is resolved using this order:

```text
Runtime Defaults
    ↓
Global Config
    ↓
Workspace Config
    ↓
Environment Variables
    ↓
CLI Flags
    ↓
Request-Level Overrides
```

Higher layers override lower layers.

Request-level overrides have the highest priority.

---

# 5. Default Behaviour

If no config exists, Gumi should start with safe local defaults.

Default assumptions:

```yaml
runtime:
  mode: stabilized
  host: 127.0.0.1
  port: 8787

dashboard:
  enabled: true
  host: 127.0.0.1
  port: 8788

auth:
  mode: local
  local_key: gumi-local

provider:
  default: ollama

providers:
  ollama:
    enabled: true
    url: http://localhost:11434
    default_model: local:auto

telemetry:
  local: true
  external: false
  log_prompts: false
  log_responses: false
```

---

# 6. Full Configuration Example

```yaml
runtime:
  name: gumi
  mode: stabilized
  host: 127.0.0.1
  port: 8787
  environment: local
  log_level: info

auth:
  mode: local
  local_key: gumi-local

dashboard:
  enabled: true
  host: 127.0.0.1
  port: 8788

provider:
  default: ollama
  model_selection: auto

providers:
  ollama:
    enabled: true
    url: http://localhost:11434
    default_model: qwen3:8b
    timeout_seconds: 90

  lmstudio:
    enabled: true
    url: http://localhost:1234/v1
    default_model: local-model
    timeout_seconds: 90

  openai_compatible_local:
    enabled: false
    url: http://localhost:8000/v1
    default_model: local-model
    timeout_seconds: 90

engines:
  context:
    enabled: true
    strategy: hybrid
    max_input_tokens: 16000
    preserve_recent_messages: 8
    compression_enabled: true

  prompt:
    enabled: true
    profile_mode: auto
    optimize_system_prompt: true
    preserve_user_intent: true

  guard:
    enabled: true
    anti_loop: true
    context_overflow: true
    structured_output: true

  validation:
    enabled: true
    json: true
    markdown: true
    repair: true

  repair:
    enabled: true
    max_attempts: 1
    allow_full_regeneration: true
    preserve_valid_content: true

  telemetry:
    enabled: true

memory:
  enabled: false
  engine: sqlite
  db_path: ~/.gumi/memory.db
  max_facts: 10000
  max_episodes_per_session: 500
  model_fit_retention_days: 90
  injection_budget_tokens: 1200
  min_confidence: 0.3
  max_injected_facts: 20
  extract_enabled: true
  min_observation_count: 2
  track_model_fit: true
  model_fit_decay: 0.95
  hot_cache_max_size: 500

routing:
  enabled: false
  mode: agentic_coding
  classifier:
    method: structural
    escalation_threshold:
      retries: 3
      steps: 6
      repetitions: 3

telemetry:
  local: true
  external: false
  storage: sqlite
  log_prompts: false
  log_responses: false
  retain_days: 14

plugins:
  enabled: true
  directory: ./plugins
  allow_unsigned: false

model_profiles:
  enabled: true
  directory: ./profiles

timeouts:
  request_seconds: 120
  provider_seconds: 90
  repair_seconds: 30

rate_limit:
  enabled: false
  requests_per_minute: 120
```

---

# 7. Runtime Configuration

## 7.1 Schema

```yaml
runtime:
  name: gumi
  mode: stabilized
  host: 127.0.0.1
  port: 8787
  environment: local
  log_level: info
```

## 7.2 Fields

| Field | Type | Default | Description |
|---|---:|---|---|
| `name` | string | `gumi` | Runtime name |
| `mode` | string | `stabilized` | Default runtime mode |
| `host` | string | `127.0.0.1` | API host |
| `port` | number | `8787` | API port |
| `environment` | string | `local` | Runtime environment |
| `log_level` | string | `info` | Logging verbosity |

## 7.3 Runtime Modes

Supported modes:

```text
direct
stabilized
structured
agent
```

V1 implements:

```text
direct
stabilized
structured
agent
```

Agent mode is now shipped. See [19-agentic-coding-router-specification.md](./19-agentic-coding-router-specification.md).

---

# 8. Authentication Configuration

## 8.1 Schema

```yaml
auth:
  mode: local
  local_key: gumi-local
```

## 8.2 Supported Modes

```text
disabled
local
api_key
```

## 8.3 Mode Behaviour

## disabled

No authentication required.

Use only for trusted local development.

## local

Uses a local development key.

Default:

```text
gumi-local
```

## api_key

Validates API keys from local config or database.

Example:

```yaml
auth:
  mode: api_key
  keys:
    - name: default
      key: nvx_local_123
      workspace: default
```

## 8.4 Security Rule

Authentication should default to `local`, not `disabled`.

---

# 9. Dashboard Configuration

## 9.1 Schema

```yaml
dashboard:
  enabled: true
  host: 127.0.0.1
  port: 8788
```

## 9.2 Fields

| Field | Type | Default | Description |
|---|---:|---|---|
| `enabled` | boolean | `true` | Enable local dashboard |
| `host` | string | `127.0.0.1` | Dashboard host |
| `port` | number | `8788` | Dashboard port |

## 9.3 Rule

Dashboard must be local-only by default.

Do not bind to `0.0.0.0` unless user explicitly configures it.

---

# 10. Provider Configuration

## 10.1 Global Provider Selection

```yaml
provider:
  default: ollama
  model_selection: auto
```

## 10.2 Supported Provider Selection Modes

```text
auto
explicit
profile_based
```

## auto

Gumi chooses from available providers.

## explicit

Use the provider specified by config or request.

## profile_based

Use model profile recommendation.

---

# 11. Provider Adapter Configuration

## 11.1 Ollama

```yaml
providers:
  ollama:
    enabled: true
    url: http://localhost:11434
    default_model: qwen3:8b
    timeout_seconds: 90
```

## 11.2 LM Studio

```yaml
providers:
  lmstudio:
    enabled: true
    url: http://localhost:1234/v1
    default_model: local-model
    timeout_seconds: 90
```

## 11.3 OpenAI-Compatible Local Server

```yaml
providers:
  openai_compatible_local:
    enabled: false
    url: http://localhost:8000/v1
    default_model: local-model
    timeout_seconds: 90
    api_key: local-key
```

## 11.4 Provider Fields

| Field | Type | Required | Description |
|---|---:|---:|---|
| `enabled` | boolean | yes | Enable provider |
| `url` | string | yes | Provider base URL |
| `default_model` | string | no | Default model for provider |
| `timeout_seconds` | number | no | Provider timeout |
| `api_key` | string | no | Provider key, if needed |

## 11.5 Provider Security Rule

Provider API keys must be redacted in:

- logs
- dashboard
- `/v1/gumi/config`

---

# 12. Engine Configuration

Engine configuration controls the intelligence pipeline.

---

## 12.1 Context Engine

```yaml
engines:
  context:
    enabled: true
    strategy: hybrid
    max_input_tokens: 16000
    preserve_recent_messages: 8
    compression_enabled: true
```

### Fields

| Field | Type | Default |
|---|---:|---|
| `enabled` | boolean | `true` |
| `strategy` | string | `hybrid` |
| `max_input_tokens` | number | `16000` |
| `preserve_recent_messages` | number | `8` |
| `compression_enabled` | boolean | `true` |

### Strategies

```text
none
trim
summarize
compress
hybrid
```

---

## 12.2 Prompt Engine

```yaml
engines:
  prompt:
    enabled: true
    profile_mode: auto
    optimize_system_prompt: true
    preserve_user_intent: true
```

### Fields

| Field | Type | Default |
|---|---:|---|
| `enabled` | boolean | `true` |
| `profile_mode` | string | `auto` |
| `optimize_system_prompt` | boolean | `true` |
| `preserve_user_intent` | boolean | `true` |

### Profile Modes

```text
off
auto
strict
```

---

## 12.3 Guard Engine

```yaml
engines:
  guard:
    enabled: true
    anti_loop: true
    context_overflow: true
    structured_output: true
```

### Fields

| Field | Type | Default |
|---|---:|---|
| `enabled` | boolean | `true` |
| `anti_loop` | boolean | `true` |
| `context_overflow` | boolean | `true` |
| `structured_output` | boolean | `true` |

---

## 12.4 Validation Engine

```yaml
engines:
  validation:
    enabled: true
    json: true
    markdown: true
    repair: true
```

### Fields

| Field | Type | Default |
|---|---:|---|
| `enabled` | boolean | `true` |
| `json` | boolean | `true` |
| `markdown` | boolean | `true` |
| `repair` | boolean | `true` |

---

## 12.5 Repair Engine

```yaml
engines:
  repair:
    enabled: true
    max_attempts: 1
    allow_full_regeneration: true
    preserve_valid_content: true
```

### Fields

| Field | Type | Default |
|---|---:|---|
| `enabled` | boolean | `true` |
| `max_attempts` | number | `1` |
| `allow_full_regeneration` | boolean | `true` |
| `preserve_valid_content` | boolean | `true` |

---

## 12.6 Memory Engine

The memory engine uses a **top-level** `memory:` block (not under `engines:`).

```yaml
memory:
  enabled: false           # Opt-in in V1
  engine: sqlite           # sqlite | in_memory (for testing)
  db_path: ~/.gumi/memory.db
  max_facts: 10000
  max_episodes_per_session: 500
  model_fit_retention_days: 90
  injection_budget_tokens: 1200
  min_confidence: 0.3
  max_injected_facts: 20
  extract_enabled: true
  min_observation_count: 2
  track_model_fit: true
  model_fit_decay: 0.95
  hot_cache_max_size: 500
```

### Fields

| Field | Type | Default | Description |
|---|---:|---|---|
| `enabled` | boolean | `false` | Enable the memory engine |
| `engine` | string | `sqlite` | Storage backend (`sqlite` or `in_memory`) |
| `db_path` | string | `~/.gumi/memory.db` | SQLite database path |
| `max_facts` | number | `10000` | Max stored facts before eviction |
| `max_episodes_per_session` | number | `500` | Max episodes stored per session |
| `model_fit_retention_days` | number | `90` | Model fit data retention |
| `injection_budget_tokens` | number | `1200` | Token budget for memory injection |
| `min_confidence` | number | `0.3` | Minimum confidence to inject a fact |
| `max_injected_facts` | number | `20` | Maximum facts per injection |
| `extract_enabled` | boolean | `true` | Extract facts from responses |
| `min_observation_count` | number | `2` | Observations required to store a fact |
| `track_model_fit` | boolean | `true` | Record per-model performance |
| `model_fit_decay` | number | `0.95` | Exponential decay for older observations |
| `hot_cache_max_size` | number | `500` | Max entries in the L1 hot cache |

See [20-memory-engine-specification.md](./20-memory-engine-specification.md) for the full memory engine contract.

---

## 12.7 Routing Engine

The Agentic Coding Router uses a **top-level** `routing:` block.

```yaml
routing:
  enabled: false
  mode: agentic_coding
  classifier:
    method: structural
    escalation_threshold:
      retries: 3
      steps: 6
      repetitions: 3
  coding_rules:
    - name: custom-trivial
      prefer: fastest
      min_coding: weak
      max_size: small
```

### Fields

| Field | Type | Default | Description |
|---|---:|---|---|
| `enabled` | boolean | `false` | Enable the agentic coding router |
| `mode` | string | `agentic_coding` | Router mode (only `agentic_coding` in V1) |
| `classifier.method` | string | `structural` | Classification method |
| `classifier.escalation_threshold.retries` | number | `3` | Retry count to escalate difficulty |
| `classifier.escalation_threshold.steps` | number | `6` | Step count to escalate difficulty |
| `classifier.escalation_threshold.repetitions` | number | `3` | Repeated tool-call count to escalate |
| `coding_rules` | list | — | Override default routing rules |

See [19-agentic-coding-router-specification.md](./19-agentic-coding-router-specification.md) for the full router contract.

---

## 12.8 Telemetry Engine

```yaml
engines:
  telemetry:
    enabled: true
```

Telemetry Engine should usually stay enabled.

---

# 13. Telemetry Configuration

```yaml
telemetry:
  local: true
  external: false
  storage: sqlite
  log_prompts: false
  log_responses: false
  retain_days: 14
```

## Fields

| Field | Type | Default | Description |
|---|---:|---|---|
| `local` | boolean | `true` | Store local telemetry |
| `external` | boolean | `false` | Send external telemetry |
| `storage` | string | `sqlite` | Local telemetry storage |
| `log_prompts` | boolean | `false` | Store full prompts |
| `log_responses` | boolean | `false` | Store full responses |
| `retain_days` | number | `14` | Local telemetry retention |

## Privacy Rule

Full prompt and response logging must be disabled by default.

---

# 14. Plugin Configuration

```yaml
plugins:
  enabled: true
  directory: ./plugins
  allow_unsigned: false
```

## Fields

| Field | Type | Default |
|---|---:|---|
| `enabled` | boolean | `true` |
| `directory` | string | `./plugins` |
| `allow_unsigned` | boolean | `false` |

## V1 Rule

Plugin Engine can be designed in V1 but fully implemented later.

The config should reserve space for plugins from the beginning.

---

# 15. Model Profile Configuration

```yaml
model_profiles:
  enabled: true
  directory: ./profiles
```

## Fields

| Field | Type | Default |
|---|---:|---|
| `enabled` | boolean | `true` |
| `directory` | string | `./profiles` |

---

# 16. Model Profile File Format

Model profiles should live in:

```text
profiles/
```

Example:

```text
profiles/qwen3-8b.yaml
```

Example profile:

```yaml
id: qwen3-8b
provider: ollama
model: qwen3:8b
version: 1

capabilities:
  chat: true
  streaming: true
  structured_output: medium
  tool_calling: weak
  long_context: medium

defaults:
  temperature: 0.4
  top_p: 0.9
  repeat_penalty: 1.12
  max_tokens: 4096

context:
  strategy: hybrid
  max_input_tokens: 24000
  preserve_recent_messages: 8

prompt:
  style: technical
  instruction_strength: strong
  json_instruction_style: explicit

guard:
  anti_loop: aggressive
  json_repair: true

known_weaknesses:
  - May over-explain simple answers.
  - Requires explicit JSON instructions.
  - Can repeat when context is too long.

notes:
  - Good for technical Q&A and coding.
```

---

# 17. Timeouts Configuration

```yaml
timeouts:
  request_seconds: 120
  provider_seconds: 90
  repair_seconds: 30
```

## Timeout Rules

| Timeout | Description |
|---|---|
| `request_seconds` | Total request timeout |
| `provider_seconds` | Provider call timeout |
| `repair_seconds` | Repair attempt timeout |

Provider timeout must be shorter than total request timeout.

---

# 18. Rate Limit Configuration

```yaml
rate_limit:
  enabled: false
  requests_per_minute: 120
```

V1 local runtime does not require rate limiting by default.

Useful for:

- shared local servers
- team machines
- LAN usage
- testing apps

---

# 19. Workspace Configuration

V1 supports a default workspace.

Future workspace config:

```yaml
workspaces:
  default:
    name: Default Workspace
    memory_enabled: false
    telemetry_enabled: true
    provider: ollama
```

V1 should be designed so workspace support can expand later without breaking architecture.

---

# 20. Environment Variables

Gumi should support common environment overrides.

```bash
NOVEXA_CONFIG=./gumi.yaml
NOVEXA_HOST=127.0.0.1
NOVEXA_PORT=8787
NOVEXA_DASHBOARD_PORT=8788
NOVEXA_AUTH_MODE=local
NOVEXA_LOCAL_KEY=gumi-local
NOVEXA_PROVIDER=ollama
NOVEXA_OLLAMA_URL=http://localhost:11434
GUMI_LMSTUDIO_URL=http://localhost:1234/v1
NOVEXA_LOG_LEVEL=info
```

Environment variables override config file values.

---

# 21. CLI Flags

CLI flags override config file and environment variables.

Examples:

```bash
gumi start --port 8787
gumi start --provider ollama
gumi start --model qwen3:8b
gumi start --config ./gumi.yaml
gumi start --mode direct
```

---

# 22. Request-Level Overrides

Request-level overrides are allowed through the `gumi` extension field.

Example:

```json
{
  "model": "local:auto",
  "messages": [
    {
      "role": "user",
      "content": "Return valid JSON."
    }
  ],
  "gumi": {
    "mode": "structured",
    "validation": {
      "enabled": true,
      "repair": true
    }
  }
}
```

Request-level override should only affect the current request.

It must not mutate global config.

---

# 23. Config Validation

Config Engine must validate config at startup.

Validation should check:

- valid runtime mode
- valid port numbers
- provider URL format
- enabled provider exists
- default model exists if required
- engine flags are valid
- telemetry settings are safe
- plugin directory exists or can be created
- model profile directory exists or can be created

---

# 24. Config Error Format

Example:

```json
{
  "error": {
    "code": "INVALID_CONFIG",
    "message": "providers.ollama.url must be a valid URL.",
    "engine": "config",
    "retryable": false,
    "suggestion": "Set providers.ollama.url to http://localhost:11434."
  }
}
```

---

# 25. Config Inspection

Users should be able to inspect resolved config:

```bash
gumi config show
```

API:

```http
GET /v1/gumi/config
```

Sensitive values must be redacted.

Example:

```yaml
providers:
  openai_compatible_local:
    api_key: "***REDACTED***"
```

---

# 26. Config Doctor

The doctor command should check configuration and environment.

```bash
gumi doctor
```

Checks:

- config file parse
- provider reachable
- default model available
- port available
- dashboard port available
- model profiles loaded
- plugin directory valid
- telemetry database writable

---

# 27. Safe Defaults

Gumi V1 defaults must follow these rules:

1. Bind only to localhost.
2. Do not send external telemetry.
3. Do not log full prompts.
4. Do not log full responses.
5. Do not require cloud API keys.
6. Do not enable unsafe plugins.
7. Do not expose dashboard publicly.
8. Do not mutate global config from requests.

---

# 28. Example Minimal Config

```yaml
provider:
  default: ollama

providers:
  ollama:
    url: http://localhost:11434
    default_model: qwen3:8b
```

---

# 29. Example Direct Mode Config

```yaml
runtime:
  mode: direct

engines:
  context:
    enabled: false
  prompt:
    enabled: false
  validation:
    enabled: false
  repair:
    enabled: false
```

Use when the user wants maximum speed and minimal Gumi processing.

---

# 30. Example Stabilized Mode Config

```yaml
runtime:
  mode: stabilized

engines:
  context:
    enabled: true
    strategy: hybrid

  prompt:
    enabled: true
    profile_mode: auto

  guard:
    enabled: true
    anti_loop: true

  validation:
    enabled: true
    repair: true

  repair:
    enabled: true
```

This is the recommended default mode.

---

# 31. Example Structured Mode Config

```yaml
runtime:
  mode: structured

engines:
  context:
    enabled: true
    strategy: hybrid

  prompt:
    enabled: true
    profile_mode: strict

  guard:
    enabled: true
    structured_output: true
    anti_loop: true

  validation:
    enabled: true
    json: true
    repair: true

  repair:
    enabled: true
    max_attempts: 1
```

Use for JSON-heavy applications.

---

# 32. Configuration Versioning

Config files should include optional version:

```yaml
version: 1
```

If omitted, assume latest V1-compatible schema.

Breaking config changes require migration tools.

Future:

```bash
gumi config migrate
```

---

# 33. Final Configuration Statement

Gumi configuration is designed to be invisible for beginners and precise for advanced users.

The default experience should be:

```bash
gumi start
```

The advanced experience should allow full control through `gumi.yaml`, environment variables, CLI flags, and request-level overrides.

Gumi must remain local-first, safe by default, and fully inspectable.