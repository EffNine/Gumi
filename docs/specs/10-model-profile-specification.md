# Gumi Model Profile Specification

Version: 1.0  
Status: Draft  
Scope: Model profile system for Gumi Runtime V1

---

# 1. Purpose

This document defines the Model Profile system in Gumi Runtime.

Model Profiles describe how specific local models should be used.

They help Gumi decide:

- default generation settings
- context strategy
- prompt style
- structured output behaviour
- anti-loop behaviour
- known model weaknesses
- provider compatibility
- task suitability

Model Profiles reduce manual tuning and improve local model reliability.

---

# 2. Core Philosophy

Different models behave differently.

A Qwen model may respond better to explicit structured instructions.

A DeepSeek coder model may perform better for code and debugging.

A Llama model may require different repetition controls.

A small model may need shorter context and stronger prompt formatting.

Gumi should not treat all models the same.

Model Profiles allow Gumi to adapt runtime behaviour to each model.

---

# 3. What Model Profiles Solve

Without Model Profiles, users must manually tune:

- temperature
- top_p
- repeat penalty
- max tokens
- prompt style
- JSON instruction format
- context size
- stop tokens
- tool-calling expectations
- structured output reliability

This creates a poor developer experience.

Model Profiles turn that messy tuning process into a reusable preset.

---

# 4. Model Profile Position

```text
Request
    ↓
Provider Engine
    ↓
Model Profile Resolution
    ↓
Context Engine
    ↓
Prompt Engine
    ↓
Guard Engine
    ↓
Provider Adapter
```

Model Profiles influence multiple engines.

They do not generate responses.

They guide runtime behaviour.

---

# 5. Model Profile Responsibilities

A Model Profile should define:

- model identity
- provider compatibility
- model capabilities
- recommended defaults
- context strategy
- prompt strategy
- guard strategy
- validation strategy
- known weaknesses
- recommended use cases
- unsupported use cases

---

# 6. Model Profile File Location

Default profile directory:

```text
profiles/
```

Example files:

```text
profiles/qwen3-8b.yaml
profiles/deepseek-r1-8b.yaml
profiles/llama3.1-8b.yaml
profiles/gemma3-12b.yaml
```

Global user profile directory:

```text
~/.gumi/profiles/
```

Project-level profile directory:

```text
./profiles/
```

---

# 7. Profile Loading Order

Profiles should be loaded in this order:

```text
1. Built-in Gumi profiles
2. User global profiles
3. Workspace/project profiles
4. Request-level overrides
```

Higher layers override lower layers.

---

# 8. Profile ID Convention

Profile IDs should use lowercase kebab-case.

Examples:

```text
qwen3-8b
qwen2.5-coder-7b
deepseek-r1-8b
llama3.1-8b
gemma3-12b
mistral-small
```

---

# 9. Model ID Mapping

A profile may map to different provider-native names.

Example:

```yaml
id: qwen3-8b

models:
  ollama:
    - qwen3:8b
    - qwen3:latest

  lmstudio:
    - qwen3-8b
    - qwen3-8b-instruct

  openai_compatible_local:
    - qwen3-8b
```

This allows the same logical profile to apply across providers.

---

# 10. Model Profile Schema

```yaml
id: qwen3-8b
name: Qwen3 8B
version: 1
family: qwen
size: 8b
type: instruct

models:
  ollama:
    - qwen3:8b

capabilities:
  chat: true
  streaming: true
  structured_output: medium
  json_mode: medium
  tool_calling: weak
  vision: false
  long_context: medium
  coding: strong
  reasoning: medium
  creative_writing: medium

defaults:
  temperature: 0.4
  top_p: 0.9
  repeat_penalty: 1.12
  max_tokens: 4096
  stop: []

context:
  strategy: hybrid
  max_input_tokens: 24000
  preserve_recent_messages: 8
  compression: true

prompt:
  style: technical
  instruction_strength: strong
  json_instruction_style: explicit
  system_prompt_style: direct

guard:
  anti_loop: aggressive
  context_overflow: true
  structured_output: true

validation:
  json_repair: true
  repetition_check: true
  markdown_check: true

routing:
  priority: 80
  preferred_tasks:
    - coding
    - technical_qa
    - structured_output

avoid_tasks:
  - high_accuracy_current_facts
  - legal_advice
  - medical_advice

known_weaknesses:
  - May repeat when context is too long.
  - Requires explicit JSON-only instructions for structured output.
  - May over-explain simple answers.

notes:
  - Good general-purpose technical local model.
```

---

# 11. Required Fields

Every profile must include:

```text
id
name
version
family
models
capabilities
defaults
context
prompt
guard
validation
```

Optional fields:

```text
routing
avoid_tasks
known_weaknesses
notes
benchmarks
hardware
```

---

# 12. Capability Values

Boolean capabilities:

```text
true
false
unknown
```

Quality capabilities:

```text
none
weak
medium
strong
unknown
```

Example:

```yaml
capabilities:
  chat: true
  streaming: true
  structured_output: medium
  tool_calling: weak
  reasoning: medium
```

---

# 13. Capability Definitions

## chat

Model supports normal chat completion.

## streaming

Provider/model combination supports streaming.

## structured_output

How reliable the model is when asked for structured output.

Values:

```text
none
weak
medium
strong
unknown
```

## json_mode

How reliable the model is when asked for valid JSON.

## tool_calling

How reliable the model is for tool-call style output.

V1 may not fully support tool calling, but the capability should exist for future Agent Mode.

## vision

Whether the model supports image input.

V1 does not require vision support.

## long_context

How well the model handles long context.

## coding

How suitable the model is for coding tasks.

## reasoning

How suitable the model is for reasoning-heavy tasks.

## creative_writing

How suitable the model is for writing tasks.

---

# 14. Default Generation Settings

Model Profiles can define generation defaults.

```yaml
defaults:
  temperature: 0.4
  top_p: 0.9
  repeat_penalty: 1.12
  max_tokens: 4096
  stop: []
```

## 14.3 Thinking Default

Model Profiles can define a default thinking preference for models that support reasoning output.

```yaml
defaults:
  temperature: 0.4
  top_p: 0.9
  repeat_penalty: 1.12
  max_tokens: 4096
  stop: []
  thinking: false
```

When `thinking` is set to `false`, the runtime will request the provider to disable reasoning/thinking output. This is useful for small models where reasoning can exhaust the token budget.

When `thinking` is absent (nil), the runtime does not send any thinking preference to the provider, preserving default provider behaviour.

Precedence:

```text
request-level gumi.thinking.enabled
  >
model profile defaults.thinking
  >
provider default / unspecified
```

## 14.4 Temperature Guidelines

Suggested defaults:

```text
coding: 0.2 - 0.5
structured output: 0.1 - 0.3
technical Q&A: 0.3 - 0.6
creative writing: 0.7 - 1.0
```

## 14.2 Repeat Penalty Guidelines

For models prone to repetition:

```yaml
repeat_penalty: 1.10
```

For severe repetition risk:

```yaml
repeat_penalty: 1.15
```

Gumi must only pass repeat penalty when provider supports it.

---

# 15. Context Configuration

```yaml
context:
  strategy: hybrid
  max_input_tokens: 24000
  preserve_recent_messages: 8
  compression: true
```

## 15.1 Strategy Values

```text
none
trim
summarize
compress
hybrid
```

## 15.2 Context Rule

Small models should use stricter context compression.

Large-context models may allow more raw history.

If model context limit is unknown, use conservative defaults.

---

# 16. Prompt Configuration

```yaml
prompt:
  style: technical
  instruction_strength: strong
  json_instruction_style: explicit
  system_prompt_style: direct
```

## 16.1 Prompt Styles

```text
general
technical
coding
reasoning
creative
structured
```

## 16.2 Instruction Strength

```text
light
standard
strong
strict
```

## 16.3 JSON Instruction Style

```text
none
simple
explicit
schema_first
```

## 16.4 System Prompt Style

```text
minimal
direct
verbose
structured
```

---

# 17. Guard Configuration

```yaml
guard:
  anti_loop: aggressive
  context_overflow: true
  structured_output: true
```

## 17.1 Anti-Loop Levels

```text
off
light
standard
aggressive
```

## 17.2 Behaviour

### off

No anti-loop prompt or validation changes.

### light

Add minor anti-repeat instruction.

### standard

Add anti-repeat instruction and response repetition validation.

### aggressive

Add anti-repeat instruction, lower temperature, apply repeat penalty, and enable stricter repetition detection.

---

# 18. Validation Configuration

```yaml
validation:
  json_repair: true
  repetition_check: true
  markdown_check: true
```

Validation settings guide Validation Engine and Repair Engine.

---

# 19. Routing Configuration

```yaml
routing:
  priority: 80
  preferred_tasks:
    - coding
    - technical_qa
    - structured_output
```

## 19.1 Priority

Priority is a number from:

```text
0 - 100
```

Higher priority means Gumi prefers this model during auto-selection when task matches.

## 19.2 Task Types

Suggested task types:

```text
general_chat
technical_qa
coding
debugging
summarization
translation
creative_writing
structured_output
classification
extraction
reasoning
```

---

# 20. Avoid Tasks

```yaml
avoid_tasks:
  - high_accuracy_current_facts
  - legal_advice
  - medical_advice
```

Avoid tasks are not hard blocks.

They are warnings unless configured otherwise.

---

# 21. Known Weaknesses

Profiles should document model weaknesses.

Example:

```yaml
known_weaknesses:
  - May repeat when context is too long.
  - May produce invalid JSON unless instructions are strict.
  - May hallucinate current information without sources.
```

Prompt Engine and Guard Engine may use these weaknesses to adjust runtime behaviour.

---

# 22. Hardware Recommendations

Optional section:

```yaml
hardware:
  min_vram_gb: 8
  recommended_vram_gb: 12
  min_ram_gb: 16
  recommended_ram_gb: 32
```

This can help Dashboard and CLI Doctor provide recommendations.

---

# 23. Benchmark Metadata

Optional section:

```yaml
benchmarks:
  coding: medium
  reasoning: medium
  json_reliability: medium
  repetition_risk: medium
  latency: fast
```

Benchmark values:

```text
poor
weak
medium
good
excellent
unknown
```

---

# 24. Generic Fallback Profile

If no matching profile exists, Gumi should use a generic local model profile.

```yaml
id: generic-local
name: Generic Local Model
version: 1
family: unknown
size: unknown
type: instruct

capabilities:
  chat: true
  streaming: unknown
  structured_output: weak
  json_mode: weak
  tool_calling: unknown
  vision: false
  long_context: unknown
  coding: unknown
  reasoning: unknown
  creative_writing: unknown

defaults:
  temperature: 0.5
  top_p: 0.9
  repeat_penalty: 1.1
  max_tokens: 2048
  stop: []

context:
  strategy: hybrid
  max_input_tokens: 8000
  preserve_recent_messages: 6
  compression: true

prompt:
  style: general
  instruction_strength: standard
  json_instruction_style: explicit
  system_prompt_style: direct

guard:
  anti_loop: standard
  context_overflow: true
  structured_output: true

validation:
  json_repair: true
  repetition_check: true
  markdown_check: true

known_weaknesses:
  - Unknown model profile. Conservative settings applied.
```

---

# 25. Profile Resolution

Profile Engine should resolve profile using this order:

```text
1. Exact provider:model match
2. Provider-native model alias
3. Logical model ID
4. Family match
5. Generic fallback profile
```

Example:

Request model:

```text
ollama:qwen3:8b
```

Resolution:

```text
provider = ollama
provider model = qwen3:8b
profile = qwen3-8b
```

---

# 26. Profile Matching Rules

A model profile can match:

```yaml
models:
  ollama:
    - qwen3:8b
    - qwen3:latest
```

If discovered provider model matches any alias, use the profile.

Matching should be case-insensitive where safe.

---

# 27. Profile Overrides

Users may override profile fields in config.

Example:

```yaml
profile_overrides:
  qwen3-8b:
    defaults:
      temperature: 0.3
    guard:
      anti_loop: aggressive
```

Request-level overrides may also apply.

Example:

```json
{
  "gumi": {
    "profile": {
      "defaults": {
        "temperature": 0.2
      }
    }
  }
}
```

Request-level overrides affect only that request.

---

# 28. Profile Validation

Every profile must be validated at load time.

Validation checks:

- valid YAML
- required fields exist
- known capability values
- valid generation settings
- valid context strategy
- valid anti-loop level
- valid model mappings
- version field exists

Invalid profile should not crash runtime.

It should be skipped with warning.

---

# 29. Profile Versioning

Profiles must include:

```yaml
version: 1
```

Future breaking changes require version increment.

Example future version:

```yaml
schema_version: 2
```

V1 should support schema version 1.

---

# 30. Built-In Profiles for V1

V1 should ship with starter profiles for:

```text
generic-local
qwen3-8b
qwen2.5-coder-7b
deepseek-r1-8b
llama3.1-8b
gemma3-12b
mistral-small
qwen3.5-2b
```

These profiles can be approximate and improved over time.

### LM Studio Validated Profiles

The following profiles have been benchmarked against LM Studio and validated with Profile Doctor:

| Profile | LM Studio Model | Size | Role | Gumi Pass | Direct p50 | Doctor |
|---------|----------------|------|------|-------------|------------|--------|
| `qwen2.5-coder-7b` | `qwen2.5-coder-7b-instruct` | 7B | Coding | 21/21 | 114ms | Good baseline |
| `qwen3-1.7b` | `qwen/qwen3-1.7b` | 1.7B | Fast chat | 21/21 | 94ms | Good baseline |
| `ornith-1.0-9b-q4-km` | `ornith-1.0-9b@q4_k_m` | 9B | Quality alt | 21/21 | 182ms | Good baseline |
| `qwen3.5-9b` | `qwen/qwen3.5-9b` | 9B | Technical | 18/21 | 197ms | Good baseline |
| `gemma-4-e4b` | `google/gemma-4-e4b` | 4B | Mid-size | 15/21 | 175ms | Needs tuning |

**Benchmark mode notes:**
- **A-LMStudioDirect** — raw provider pass-through. Diagnostic only.
- **B-GumiDirect** — thin Gumi proxy. Diagnostic only.
- **C-NovexaStabilized** — main quality gate. All validated profiles pass 100%.
- **D-NovexaStructured** — strict JSON/schema output mode. Quality gate for structured output.

**Recommended default model choices:**

| Use Case | LM Studio Model | Profile |
|----------|---------------|---------|
| Coding | `qwen2.5-coder-7b-instruct` | `qwen2.5-coder-7b` |
| Fast general chat | `qwen/qwen3-1.7b` | `qwen3-1.7b` |
| Mid-size general chat | `google/gemma-4-e4b` | `gemma-4-e4b` |
| Quality alternative | `ornith-1.0-9b@q4_k_m` | `ornith-1.0-9b-q4-km` |

Run the benchmark matrix to validate profiles against your LM Studio server:

```bash
ATTEMPTS=1 LMSTUDIO_URL=http://192.168.0.164:1234/v1 ./scripts/benchmark-lmstudio-matrix.sh
```

---

# 31. Example Profile: Qwen3 8B

```yaml
id: qwen3-8b
name: Qwen3 8B
version: 1
family: qwen
size: 8b
type: instruct

models:
  ollama:
    - qwen3:8b
    - qwen3:latest

capabilities:
  chat: true
  streaming: true
  structured_output: medium
  json_mode: medium
  tool_calling: weak
  vision: false
  long_context: medium
  coding: strong
  reasoning: medium
  creative_writing: medium

defaults:
  temperature: 0.4
  top_p: 0.9
  repeat_penalty: 1.12
  max_tokens: 4096
  stop: []

context:
  strategy: hybrid
  max_input_tokens: 24000
  preserve_recent_messages: 8
  compression: true

prompt:
  style: technical
  instruction_strength: strong
  json_instruction_style: explicit
  system_prompt_style: direct

guard:
  anti_loop: aggressive
  context_overflow: true
  structured_output: true

validation:
  json_repair: true
  repetition_check: true
  markdown_check: true

routing:
  priority: 85
  preferred_tasks:
    - coding
    - technical_qa
    - structured_output

avoid_tasks:
  - high_accuracy_current_facts

known_weaknesses:
  - May repeat when context is too long.
  - Requires explicit JSON-only instructions for structured output.
```

---

# 32. Example Profile: DeepSeek R1 8B

```yaml
id: deepseek-r1-8b
name: DeepSeek R1 8B
version: 1
family: deepseek
size: 8b
type: reasoning

models:
  ollama:
    - deepseek-r1:8b

capabilities:
  chat: true
  streaming: true
  structured_output: weak
  json_mode: weak
  tool_calling: weak
  vision: false
  long_context: medium
  coding: medium
  reasoning: strong
  creative_writing: medium

defaults:
  temperature: 0.5
  top_p: 0.9
  repeat_penalty: 1.12
  max_tokens: 4096
  stop: []

context:
  strategy: hybrid
  max_input_tokens: 16000
  preserve_recent_messages: 6
  compression: true

prompt:
  style: reasoning
  instruction_strength: standard
  json_instruction_style: explicit
  system_prompt_style: direct

guard:
  anti_loop: aggressive
  context_overflow: true
  structured_output: true

validation:
  json_repair: true
  repetition_check: true
  markdown_check: true

routing:
  priority: 75
  preferred_tasks:
    - reasoning
    - technical_qa

avoid_tasks:
  - strict_json_schema
  - high_accuracy_current_facts

known_weaknesses:
  - May include reasoning-like text when structured output is requested.
  - May require strict instruction to avoid verbose output.
```

---

# 33. Example Profile: Llama 3.1 8B

```yaml
id: llama3.1-8b
name: Llama 3.1 8B
version: 1
family: llama
size: 8b
type: instruct

models:
  ollama:
    - llama3.1:8b

capabilities:
  chat: true
  streaming: true
  structured_output: medium
  json_mode: medium
  tool_calling: weak
  vision: false
  long_context: medium
  coding: medium
  reasoning: medium
  creative_writing: strong

defaults:
  temperature: 0.5
  top_p: 0.9
  repeat_penalty: 1.1
  max_tokens: 4096
  stop: []

context:
  strategy: hybrid
  max_input_tokens: 16000
  preserve_recent_messages: 8
  compression: true

prompt:
  style: general
  instruction_strength: standard
  json_instruction_style: explicit
  system_prompt_style: direct

guard:
  anti_loop: standard
  context_overflow: true
  structured_output: true

validation:
  json_repair: true
  repetition_check: true
  markdown_check: true

routing:
  priority: 70
  preferred_tasks:
    - general_chat
    - creative_writing
    - summarization

known_weaknesses:
  - May need stricter prompting for technical precision.
```

---

# 34. Profile Contribution Rules

Community profiles should follow these rules:

1. Include model aliases.
2. Include tested provider names.
3. Include conservative defaults.
4. Document known weaknesses.
5. Avoid exaggerated claims.
6. Include benchmark notes if available.
7. Prefer safe defaults over maximum creativity.
8. Do not include provider API keys or private paths.

---

# 35. CLI Commands for Profiles

V1 or future CLI:

```bash
gumi profile list
gumi profile show qwen3-8b
gumi profile validate ./profiles/qwen3-8b.yaml
gumi profile test qwen3-8b
gumi profile doctor
```

Possible output:

```text
Profile: qwen3-8b
Status: valid
Provider aliases:
- ollama:qwen3:8b

Defaults:
- temperature: 0.4
- repeat_penalty: 1.12

Warnings:
- structured_output is medium, repair recommended
```

---

# 36. Dashboard Profile Features

Dashboard should display:

- selected model profile
- active defaults
- detected capabilities
- known weaknesses
- profile warnings
- applied overrides
- recommendation notes

---

# 37. Telemetry Events

Profile-related events:

```text
model_profile_resolved
model_profile_missing
generic_profile_applied
model_profile_override_applied
model_profile_invalid
model_profile_warning
```

Example:

```yaml
event: model_profile_resolved
engine: config
metadata:
  requested_model: ollama:qwen3:8b
  profile_id: qwen3-8b
```

---

# 38. Testing Requirements

Tests should cover:

- exact profile match
- alias profile match
- missing profile fallback
- invalid profile skip
- profile override
- request-level override
- capability lookup
- provider alias mapping
- prompt instruction generation
- guard setting generation

---

# 39. V1 Implementation Priority

Implement in this order:

```text
1. Profile schema
2. Built-in generic profile
3. Profile loader
4. Provider alias matching
5. Profile validation
6. Config overrides
7. Prompt Engine integration
8. Guard Engine integration
9. Dashboard display
10. CLI profile commands
```

---

# 40. Anti-Patterns

Avoid:

```text
Hardcoding model behaviour inside Prompt Engine
Using one profile for all models
Claiming unsupported capabilities
Failing runtime because profile is missing
Letting profile override explicit user request
Making profiles too complex for users to edit
Using model profiles as marketing claims
```

---

# 41. Final Statement

Model Profiles are Gumi's model intelligence layer.

They let Gumi adapt to each local model without changing the model itself.

A good Model Profile makes a local model easier to use, less fragile, and more predictable.

This is one of Gumi's most important advantages over a simple proxy.