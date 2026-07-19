# Gumi Validation, Repair & Guard Specification

Version: 1.0  
Status: Draft  
Scope: Guard Engine, Validation Engine, Repair Engine, and anti-loop behaviour for Gumi Runtime V1

---

# 1. Purpose

This document defines how Gumi detects, prevents, validates, and repairs problematic model behaviour.

The main goal is to make local AI output more reliable before it reaches the application.

This includes:

- malformed JSON
- invalid structured output
- repeated output
- empty responses
- incomplete responses
- unsupported model capabilities
- context overflow risk
- prompt loop risk
- low-quality output signals

Gumi does not guarantee perfect correctness.

Gumi improves reliability by detecting and handling common failure modes.

---

# 2. Engine Roles

This specification covers three engines:

```text
Guard Engine
Validation Engine
Repair Engine
```

## 2.1 Guard Engine

Runs before provider generation.

It prevents known bad conditions before sending a prompt to the model.

## 2.2 Validation Engine

Runs after provider generation.

It checks whether the model output satisfies required constraints.

## 2.3 Repair Engine

Runs after validation failure.

It attempts to fix output safely.

---

# 3. Stability Flow

```text
Prompt Package
    ↓
Guard Engine
    ↓
Provider Engine
    ↓
Response Engine
    ↓
Validation Engine
    ↓
Repair Engine if needed
    ↓
Retry if needed
    ↓
Final Response
```

---

# 4. Core Philosophy

Gumi should prefer:

```text
prevent → detect → repair → retry → fail clearly
```

Instead of:

```text
generate blindly → return broken output
```

---

# 5. Guard Engine

## 5.1 Purpose

Guard Engine evaluates risk before generation.

It does not call the provider.

It checks whether the request is safe, valid, supported, and likely to produce useful output.

---

## 5.2 Guard Engine Inputs

```text
GuardInput
├── request_id
├── runtime_mode
├── normalized_request
├── prompt_package
├── context_package
├── model_profile
├── provider_capabilities
├── response_format
├── config_snapshot
└── retry_history
```

---

## 5.3 Guard Engine Outputs

```text
GuardOutput
├── decision
├── guard_report
├── constraints
├── warnings
├── blocked
└── telemetry_events
```

---

## 5.4 Guard Decisions

Allowed decisions:

```text
allow
warn
modify_constraints
request_retry
block
```

## allow

Continue pipeline without changes.

## warn

Continue pipeline but record warning.

## modify_constraints

Continue pipeline with adjusted constraints.

Example:

```text
lower temperature
reduce max_tokens
force structured mode
enable anti-loop instruction
```

## request_retry

Ask Pipeline Engine to retry using a new strategy.

## block

Stop pipeline and return clear error.

Blocking should be rare.

---

# 6. V1 Guard Types

V1 should support these guards:

```text
empty_prompt_guard
context_overflow_guard
structured_output_guard
provider_capability_guard
anti_loop_guard
retry_budget_guard
```

---

# 7. Empty Prompt Guard

## 7.1 Purpose

Prevents sending empty or useless prompts to provider.

## 7.2 Trigger Conditions

```text
messages empty
latest user message empty
content only whitespace
content removed by context processing
```

## 7.3 Action

Return:

```text
INVALID_REQUEST
```

Example error:

```json
{
  "error": {
    "code": "EMPTY_PROMPT",
    "message": "The prompt is empty after normalization.",
    "type": "request_error",
    "engine": "guard",
    "retryable": false,
    "suggestion": "Provide a non-empty user message."
  }
}
```

---

# 8. Context Overflow Guard

## 8.1 Purpose

Prevents sending prompts that exceed model context window.

## 8.2 Trigger Conditions

```text
estimated_input_tokens > available_input_tokens
context_engine_failed_to_reduce
model_context_limit_unknown_and_prompt_large
```

## 8.3 Action

Possible actions:

```text
warn
modify_constraints
block
```

Default:

```text
modify_constraints → request stricter context compression
```

If still too large:

```text
block with CONTEXT_LIMIT_EXCEEDED
```

---

# 9. Structured Output Guard

## 9.1 Purpose

Ensures structured output requests get strict instructions and validation.

## 9.2 Trigger Conditions

```text
response_format.type = json_object
response_format.type = json_schema
gumi.mode = structured
```

## 9.3 Action

```text
enable validation.json
enable repair
force prompt strictness
lower temperature if needed
disable streaming warning if structured + stream
```

## 9.4 Structured Streaming Warning

If structured mode uses streaming:

```text
warn: streaming structured output cannot be fully repaired after chunks are sent
```

---

# 10. Provider Capability Guard

## 10.1 Purpose

Prevents requests that the selected provider/model cannot support.

## 10.2 Trigger Conditions

Examples:

```text
tools requested but model/tooling unsupported
vision input provided but provider has no vision support
streaming requested but provider cannot stream
json_schema requested but model profile says structured_output = weak
```

## 10.3 Action

Possible actions:

```text
warn
modify_constraints
block
```

Examples:

```text
if streaming unsupported → return STREAMING_UNSUPPORTED
if tools unsupported in V1 → warn and ignore safely
if vision unsupported → block
```

---

# 11. Anti-Loop Guard

## 11.1 Purpose

Reduces repeated output and generation loops.

Anti-loop protection exists both before and after generation.

Before generation, Guard Engine adds constraints.

After generation, Validation Engine detects repetition.

---

## 11.2 Pre-Generation Signals

Anti-loop guard should activate when:

```text
model_profile.guard.anti_loop = aggressive
previous retry failed due to repetition
prompt contains repeated instructions
context contains repeated assistant output
high max_tokens with vague prompt
temperature too high for structured output
```

---

## 11.3 Pre-Generation Actions

Possible actions:

```text
add anti-repeat instruction
lower temperature
increase repeat penalty if supported
trim repeated context
reduce max_tokens
```

Example anti-loop instruction:

```text
Avoid repeating the same sentence, paragraph, list, or code block.
When the answer is complete, stop.
If you are uncertain, state uncertainty once and stop.
```

---

# 12. Retry Budget Guard

## 12.1 Purpose

Prevents infinite retry loops.

## 12.2 Trigger Conditions

```text
retry.attempt >= retry.max_attempts
repair attempts exceeded
same retry reason repeated
same provider failure repeated
```

## 12.3 Action

Block further retry and return the best available error or response.

---

# 13. Validation Engine

## 13.1 Purpose

Validation Engine checks model output after generation.

It does not mutate output directly.

It produces a validation report.

---

## 13.2 Validation Engine Inputs

```text
ValidationInput
├── request_id
├── response_normalized
├── response_format
├── runtime_mode
├── guard_constraints
├── model_profile
├── config_snapshot
└── retry_history
```

---

## 13.3 Validation Engine Outputs

```text
ValidationOutput
├── validation_report
├── telemetry_events
├── warnings
└── errors
```

---

# 14. Validation Report

```text
ValidationReport
├── passed
├── severity
├── issues
├── repairable
├── suggested_repair_strategy
├── confidence
└── metadata
```

Example:

```yaml
passed: false
severity: error
issues:
  - code: INVALID_JSON
    message: Output is not valid JSON.
    location: response.content
repairable: true
suggested_repair_strategy: local_parse_repair
confidence: 0.94
metadata:
  response_length: 820
```

---

## 14.1 Telemetry Storage (Sprint 12)

Validation and repair reports are persisted to the Gumi telemetry database
for post-hoc diagnosis:

**`validation_reports` table:**

| Column | Description |
|---|---|
| `request_id` | Associated request |
| `passed` | Whether validation passed |
| `severity` | `info`, `warning`, or `error` |
| `repairable` | Whether the issue can be repaired |
| `suggested_repair_strategy` | `none`, `retry_generation`, `local_parse_repair`, `regex_cleanup` |
| `issues_json` | Array of `{code, message, location}` |
| `metadata_json` | Additional metadata |

**`repair_reports` table:**

| Column | Description |
|---|---|
| `request_id` | Associated request |
| `attempted` | Whether repair was attempted |
| `strategy` | Repair strategy used |
| `success` | Whether repair succeeded |
| `changes_json` | Number of changes applied |
| `remaining_issues_json` | Number of remaining issues |
| `retry_requested` | Whether a retry was requested |

Reports are recorded at all validation outcomes: pass, pass-after-repair,
pass-after-retry, and final failure. The `errors` table's `details_json` for
`VALIDATION_FAILED` errors includes a human-readable issue summary in the
`cause` field.

Query examples:

```bash
# View recent validation failures with issue details
sqlite3 ~/.gumi/gumi.db \
  "SELECT request_id, severity, issues_json FROM validation_reports WHERE passed=0 ORDER BY id DESC LIMIT 10;"

# View repair outcomes
sqlite3 ~/.gumi/gumi.db \
  "SELECT request_id, strategy, success FROM repair_reports ORDER BY id DESC LIMIT 10;"

# View validation error details
sqlite3 ~/.gumi/gumi.db \
  "SELECT request_id, details_json FROM errors WHERE code='VALIDATION_FAILED' ORDER BY created_at DESC LIMIT 5;"
```

---

# 15. Validation Types

V1 validation types:

```text
empty_response
incomplete_response
repetition
json_validity
json_schema
markdown_basic
finish_reason
```

Future validation types:

```text
citation_grounding
claim_verification
tool_call_validity
agent_progress
source_only_answer
```

---

# 16. Empty Response Validation

## 16.1 Trigger Conditions

```text
content empty
content whitespace only
provider finish_reason = stop but no content
provider response missing assistant content
```

## 16.2 Result

```yaml
passed: false
severity: error
repairable: true
suggested_repair_strategy: retry_generation
```

---

# 17. Incomplete Response Validation

## 17.1 Trigger Conditions

```text
finish_reason = length
content ends abruptly
unclosed code fence
unclosed JSON object
unclosed list structure
```

## 17.2 Result

```yaml
passed: false
severity: warning
repairable: true
suggested_repair_strategy: retry_generation
```

For JSON, prefer local repair first if possible.

---

# 18. Repetition Validation

## 18.1 Purpose

Detects repeated text and loops.

## 18.2 Repetition Types

```text
repeated_sentence
repeated_paragraph
repeated_line
repeated_code_block
repeated_json_field
repeated_list_item
degenerate_token_loop
```

---

## 18.3 Detection Methods

V1 can use simple deterministic checks:

```text
exact repeated line count
repeated n-gram ratio
duplicate paragraph count
same sentence repeated more than N times
low unique-token ratio
repeated suffix pattern
```

---

## 18.4 JSON-Aware Exclusion (Sprint 12)

Repetition detection is **skipped** for JSON output because JSON structural
elements (`}`, `"type":`, repeated keys across array objects) legitimately
repeat and should not be flagged as loops.

The `hasRepetition` function skips detection when:

- `response_format.type` is `json_object` or `json_schema`
- `runtime_mode` is `structured`
- The content parses as valid JSON (starts with `{` or `[` and passes
  `json.Valid`)

Plain-text repetition detection is unchanged — actual loops in prose or code
are still caught.

---

## 18.5 Suggested Default Thresholds

```yaml
repetition:
  max_same_line: 2
  max_same_sentence: 2
  max_same_paragraph: 1
  min_unique_token_ratio: 0.25
  repeated_suffix_chars: 80
```

---

## 18.6 Result

```yaml
passed: false
severity: error
repairable: true
suggested_repair_strategy: regex_cleanup
```

If repetition is severe:

```yaml
suggested_repair_strategy: retry_generation
```

---

# 19. JSON Validity Validation

## 19.1 Trigger Conditions

Validation applies when:

```text
response_format.type = json_object
response_format.type = json_schema
gumi.mode = structured
```

or when output appears intended to be JSON.

---

## 19.2 Checks

```text
parse as JSON
ensure root object when json_object requested
ensure no markdown fences
ensure no prose outside JSON
ensure valid encoding
```

---

## 19.3 Result for Invalid JSON

```yaml
passed: false
severity: error
repairable: true
suggested_repair_strategy: local_parse_repair
```

---

# 20. JSON Schema Validation

## 20.1 Trigger Conditions

```text
response_format.type = json_schema
```

## 20.2 Checks

```text
required fields
field types
additionalProperties
enum constraints
array item types
nested object structure
```

## 20.3 Result

If schema invalid but JSON parse works:

```yaml
passed: false
severity: error
repairable: true
suggested_repair_strategy: model_repair
```

If JSON parse fails:

```yaml
suggested_repair_strategy: local_parse_repair
```

---

# 21. Markdown Basic Validation

## 21.1 Trigger Conditions

Run when output includes Markdown.

## 21.2 Checks

```text
unclosed code fences
broken tables
excessive heading repetition
empty bullet lists
repeated bullet loops
```

## 21.3 Result

Markdown validation should usually produce warning, not fatal error.

---

# 22. Finish Reason Validation

## 22.1 Checks

```text
finish_reason = stop
finish_reason = length
finish_reason = error
finish_reason missing
```

## 22.2 Result

If `length`:

```text
incomplete_response warning or error depending on mode
```

If `error`:

```text
provider_error
```

---

# 23. Repair Engine

## 23.1 Purpose

Repair Engine fixes invalid output when safe and practical.

It should preserve valid content where possible.

It must not invent missing facts.

---

## 23.2 Repair Engine Inputs

```text
RepairInput
├── response_normalized
├── validation_report
├── prompt_package
├── response_format
├── model_profile
├── config_snapshot
└── retry_history
```

---

## 23.3 Repair Engine Outputs

```text
RepairOutput
├── repaired_response
├── repair_report
├── retry_requested
├── warnings
└── telemetry_events
```

---

# 24. Repair Report

```text
RepairReport
├── attempted
├── strategy
├── success
├── changes
├── remaining_issues
├── retry_requested
└── metadata
```

Example:

```yaml
attempted: true
strategy: regex_cleanup
success: true
changes:
  - removed repeated final paragraph
remaining_issues: []
retry_requested: false
```

---

# 25. Repair Strategies

Supported repair strategies:

```text
none
local_parse_repair
regex_cleanup
model_repair
retry_generation
fallback_provider
```

V1 supports:

```text
local_parse_repair
regex_cleanup
retry_generation
```

Out of scope for V1:

```text
fallback_provider
cloud_repair
```

---

# 26. Local Parse Repair

## 26.1 Purpose

Fix common JSON formatting issues without calling the model.

## 26.2 Fixes

```text
remove markdown fences
trim prose before first JSON object
trim prose after last JSON object
fix trailing commas where safe
close missing braces where obvious
normalize smart quotes where safe
extract JSON object from text
```

## 26.3 Rule

Local parse repair must not invent missing required values.

It may only clean syntax and extract valid structures.

---

# 27. Regex Cleanup

## 27.1 Purpose

Remove obvious repeated content or formatting noise.

## 27.2 Fixes

```text
remove repeated final paragraphs
remove duplicated lines
remove repeated code fence duplicates
remove repeated list items
remove obvious generation loops
```

## 27.3 Rule

Regex cleanup must preserve first valid occurrence.

It should not remove unique meaningful content.

---

# 28. Retry Generation

## 28.1 Purpose

Ask the same provider/model to regenerate with improved constraints.

## 28.2 Retry Adjustments

Depending on failure reason:

For invalid JSON:

```text
lower temperature
add JSON-only instruction
reduce max_tokens
remove markdown permission
```

For repetition:

```text
lower temperature
increase repeat penalty if supported
trim repeated context
add anti-repeat instruction
reduce max_tokens
```

For timeout:

```text
reduce context
reduce max_tokens
retry once
```

For incomplete response:

```text
increase max_tokens if possible
or ask continuation if safe
```

---

# 29. Model Repair

Model repair is future or optional V1.

It sends the broken output back to the model with a repair-only instruction.

Example:

```text
Fix the following JSON so it conforms to the schema.
Return only the repaired JSON.
Do not add new information.
```

This can be useful but must be used carefully because it consumes another generation.

---

# 30. Hallucination Risk Detection

V1 does not perform full factual verification.

However, Guard and Validation can detect hallucination risk signals.

## 30.1 Risk Signals

```text
question asks for current/latest information
model lacks source context
user asks for exact numbers/dates
answer contains many unsupported claims
context package has insufficient facts
model profile known weakness
```

## 30.2 V1 Behaviour

V1 should not claim to verify truth.

It may emit warning:

```text
HALLUCINATION_RISK_HIGH
```

Example metadata:

```yaml
event: hallucination_risk_detected
severity: warning
metadata:
  reason: "User asked for current information but no source context is available."
```

## 30.3 Future Behaviour

Future versions may support:

```text
source grounding
RAG verification
citation checking
external verifier model
hybrid cloud verifier
```

---

# 31. Anti-Loop System

Anti-loop is shared across Guard, Validation, Repair, and Pipeline.

---

## 31.1 Anti-Loop Layers

```text
1. Context deduplication
2. Prompt anti-repeat instruction
3. Provider parameter adjustment
4. Response repetition detection
5. Cleanup repair
6. Retry with stricter constraints
7. Stop retry budget
```

---

## 31.2 Anti-Loop Events

```text
loop_risk_detected
anti_loop_instruction_added
repetition_detected
repetition_repaired
retry_due_to_repetition
retry_budget_exceeded
```

---

## 31.3 Anti-Loop Example Flow

```text
Model returns repeated paragraph
    ↓
Response Engine detects repeated suffix
    ↓
Validation Engine fails with REPEATED_OUTPUT
    ↓
Repair Engine removes repeated section
    ↓
Validation passes
    ↓
Final response returned with repair metadata
```

If severe:

```text
Repair fails
    ↓
Pipeline retries with stricter prompt and lower temperature
    ↓
If still repeats, return error or best safe response
```

---

# 32. Structured Output Flow

```text
Request with response_format
    ↓
Structured Output Guard activates
    ↓
Prompt Engine adds strict format instructions
    ↓
Provider generates
    ↓
Validation Engine parses JSON
    ↓
Schema validation if provided
    ↓
Repair Engine fixes if possible
    ↓
Retry once if still invalid
    ↓
Return valid JSON or clear error
```

---

# 33. Error Codes

## 33.1 Guard Errors

```text
EMPTY_PROMPT
CONTEXT_LIMIT_EXCEEDED
UNSUPPORTED_MODEL_FEATURE
STREAMING_UNSUPPORTED
RETRY_BUDGET_EXCEEDED
```

## 33.2 Validation Errors

```text
EMPTY_RESPONSE
INCOMPLETE_RESPONSE
REPEATED_OUTPUT
INVALID_JSON
INVALID_SCHEMA
INVALID_MARKDOWN
FINISH_REASON_ERROR
```

## 33.3 Repair Errors

```text
REPAIR_FAILED
REPAIR_UNSAFE
RETRY_GENERATION_FAILED
```

---

# 34. Metadata in Response

If metadata enabled:

```json
{
  "gumi": {
    "validation_passed": true,
    "repair_applied": true,
    "repair_strategy": "regex_cleanup",
    "warnings": [],
    "retry_count": 0
  }
}
```

Default strict OpenAI compatibility should hide Gumi metadata unless requested.

---

# 35. Telemetry Events

Required events:

```text
guard_started
guard_completed
guard_warning
guard_blocked
validation_started
validation_completed
validation_failed
repair_started
repair_completed
repair_failed
repetition_detected
structured_output_validated
json_repaired
retry_requested
```

---

# 36. Testing Requirements

Guard Engine tests:

- empty prompt
- context overflow
- structured mode activation
- unsupported streaming
- unsupported tools
- retry budget exceeded
- anti-loop pre-generation signal

Validation Engine tests:

- empty response
- valid JSON
- invalid JSON
- JSON with markdown fence
- schema valid
- schema invalid
- repeated sentence
- repeated paragraph
- incomplete response
- unclosed code fence

Repair Engine tests:

- extract JSON from prose
- remove markdown fence
- remove trailing text after JSON
- remove repeated paragraph
- fail safely on unrecoverable JSON
- retry request generation
- preserve valid content

Pipeline integration tests:

- structured output repaired successfully
- repeated output repaired successfully
- repair fails then retry succeeds
- retry budget exceeded
- streaming structured warning
- provider timeout retry

---

# 37. V1 Implementation Priority

Implement in this order:

```text
1. Empty response validation
2. JSON validity validation
3. Local JSON parse repair
4. Basic repetition detection
5. Regex cleanup repair
6. Structured output guard
7. Context overflow guard
8. Retry budget guard
9. Incomplete response detection
10. Markdown basic validation
11. Hallucination risk warning
```

Reason:

Structured output and repetition are the highest-value reliability wins for V1.

---

# 38. Anti-Patterns

Avoid:

```text
Repair Engine inventing missing facts
Validation Engine mutating output directly
Guard Engine blocking too aggressively
Retrying indefinitely
Returning broken JSON in structured mode
Hiding repair events from telemetry
Using cloud verification in V1
Calling provider from Repair Engine without Pipeline
Treating hallucination risk warning as proof
```

---

# 39. Final Statement

Guard Engine prevents predictable failure conditions.

Validation Engine detects broken or risky output.

Repair Engine fixes what can be safely fixed.

Together, these engines form Gumi's stability shield.

They turn local model output from fragile raw generation into a more reliable runtime response.