# Gumi Context & Prompt Engine Specification

Version: 1.0  
Status: Draft  
Scope: Context Engine and Prompt Engine behaviour for Gumi Runtime V1

---

# 1. Purpose

This document defines how Gumi prepares context and prompts before sending a request to a local model.

The Context Engine and Prompt Engine are two of the most important parts of Gumi.

They are responsible for making local models behave more reliably without changing the model itself.

---

# 2. Core Philosophy

Local models often fail because of poor surrounding conditions, not because the model is useless.

Common causes:

- too much irrelevant context
- repeated conversation history
- weak system prompts
- vague user prompts
- missing output instructions
- bad model-specific settings
- context overflow
- conflicting instructions
- old failed attempts polluting the current request

Gumi improves the environment around the model.

---

# 3. Engine Relationship

```text
Normalized Request
    ↓
Session Engine
    ↓
Context Engine
    ↓
Memory Engine
    ↓
Prompt Engine
    ↓
Guard Engine
    ↓
Provider Engine
```

Context Engine decides **what information should be included**.

Prompt Engine decides **how that information should be presented to the model**.

---

# 4. Responsibility Split

## 4.1 Context Engine

Context Engine answers:

```text
What should the model know?
```

It handles:

- message trimming
- duplicate removal
- summarization
- compression
- token budgeting
- context ordering
- fact preservation

## 4.2 Prompt Engine

Prompt Engine answers:

```text
How should the model be instructed?
```

It handles:

- system prompt construction
- model profile instructions
- output format instructions
- guardrail instructions
- structured output instructions
- final message assembly

---

# 5. Context Engine Goals

Context Engine must:

1. Reduce noise.
2. Preserve intent.
3. Preserve useful facts.
4. Avoid context overflow.
5. Avoid repeated history.
6. Keep recent conversation useful.
7. Respect model context limits.
8. Produce inspectable context reports.

---

# 6. Prompt Engine Goals

Prompt Engine must:

1. Improve instruction clarity.
2. Preserve user meaning.
3. Respect developer/system instructions.
4. Apply model-specific prompt style.
5. Improve structured output reliability.
6. Avoid excessive prompt bloat.
7. Produce inspectable prompt reports.

---

# 7. Context Engine Inputs

Context Engine receives:

```text
ContextInput
├── request_id
├── workspace_id
├── session_id
├── runtime_mode
├── original_messages
├── session_messages
├── memory_results
├── model_profile
├── config_snapshot
├── response_format
└── token_budget
```

---

# 8. Context Engine Outputs

Context Engine writes:

```text
ContextOutput
├── normalized_messages
├── compressed_messages
├── context_package
├── context_report
├── omitted_content_summary
├── warnings
└── telemetry_events
```

---

# 9. Prompt Engine Inputs

Prompt Engine receives:

```text
PromptInput
├── context_package
├── memory_results
├── model_profile
├── config_snapshot
├── normalized_request
├── response_format
├── guard_constraints
└── runtime_mode
```

---

# 10. Prompt Engine Outputs

Prompt Engine writes:

```text
PromptOutput
├── prompt_package
├── final_messages
├── prompt_report
├── warnings
└── telemetry_events
```

---

# 11. Token Budgeting

Context Engine must estimate token budget before provider call.

## 11.1 Token Budget Fields

```text
TokenBudget
├── model_context_limit
├── reserved_output_tokens
├── reserved_system_tokens
├── reserved_memory_tokens
├── available_input_tokens
├── estimated_input_tokens
└── overflow_tokens
```

## 11.2 Default Budget Rule

```text
available_input_tokens =
model_context_limit
- reserved_output_tokens
- reserved_system_tokens
- reserved_memory_tokens
```

## 11.3 Default Reservations

```yaml
token_budget:
  reserved_output_tokens: 2048
  reserved_system_tokens: 1200
  reserved_memory_tokens: 1200
```

These values can be overridden by config or model profile.

---

# 12. Context Strategies

Supported strategies:

```text
none
trim
summarize
compress
hybrid
```

---

## 12.1 none

No context processing.

Used in Direct Mode.

```text
original_messages → final_messages
```

Use when:

- debugging provider behaviour
- benchmarking raw model output
- maximum speed required

---

## 12.2 trim

Remove content until request fits token budget.

Priority for removal:

```text
1. old assistant messages
2. old repeated user messages
3. verbose logs
4. old failed attempts
5. stale tool results
6. old low-priority context
```

Preserve:

```text
1. latest user request
2. system messages
3. developer messages
4. explicit constraints
5. recent assistant response
6. important decisions
```

---

## 12.3 summarize

Summarize older conversation into a compact block.

Example summary format:

```text
Previous conversation summary:
- Goal: Build Gumi Runtime.
- Decisions:
  - Local-first.
  - OpenAI-compatible.
  - Modular monolith.
- Current task: Design Context and Prompt Engine.
```

Use when:

- history is long
- conversation continuity matters
- trimming would remove too much useful context

---

## 12.4 compress

Rewrite context into structured compact form.

Example:

```text
Context Package:
Goal:
- Build Gumi Runtime.

Current Task:
- Write Context & Prompt Engine specification.

Important Decisions:
- Use Engine terminology.
- Use Pipeline Context.
- V1 is local-only.
- Cloud fallback is out of scope.

Constraints:
- OpenAI-compatible.
- No cloud billing.
- Must work offline.
```

Use when:

- local model has limited context
- repeated conversation exists
- prompt needs strong structure

---

## 12.5 hybrid

Default strategy.

Hybrid may apply:

```text
dedupe → trim → summarize → compress
```

Use when:

- normal stabilized mode
- structured mode
- context exceeds safe budget
- local model reliability matters

---

# 13. Context Priority System

Each context item should receive a priority score.

```text
ContextItem
├── content
├── source
├── role
├── age
├── estimated_tokens
├── priority
├── preserve
└── reason
```

Priority levels:

```text
critical
high
medium
low
discard
```

---

## 13.1 Critical Context

Never remove unless request is impossible.

Examples:

- system instruction
- developer instruction
- current user request
- response schema
- explicit user constraints
- safety-critical constraints

---

## 13.2 High Priority Context

Preserve if possible.

Examples:

- latest assistant answer
- recent user clarifications
- current project decisions
- active file references
- accepted architecture choices

---

## 13.3 Medium Priority Context

Keep when budget allows.

Examples:

- useful earlier examples
- previous implementation notes
- recent telemetry
- related user preferences

---

## 13.4 Low Priority Context

Remove first.

Examples:

- repeated explanations
- stale alternatives
- failed old attempts
- verbose logs
- unrelated chit-chat

---

## 13.5 Discard Context

Always remove.

Examples:

- exact duplicate messages
- repeated generated loops
- empty messages
- malformed irrelevant logs
- provider noise

---

# 14. Duplicate Detection

Context Engine should detect duplication.

Duplicate types:

```text
exact_duplicate
near_duplicate
repeated_instruction
repeated_output
repeated_code_block
repeated_error_log
```

## 14.1 Duplicate Handling

Default behaviour:

```text
remove duplicate
record telemetry event
preserve one canonical version
```

Example telemetry:

```yaml
event: duplicate_context_removed
engine: context
metadata:
  duplicate_type: near_duplicate
  removed_items: 3
```

---

# 15. Important Fact Extraction

Context Engine should extract useful durable facts from conversation context.

Example:

```text
Important Facts:
- Project name: Gumi.
- Product category: local AI runtime.
- V1 scope: local-only.
- Default API port: 8787.
- Dashboard port: 8788.
```

Fact categories:

```text
project_decision
user_constraint
technical_constraint
active_task
naming_decision
architecture_decision
configuration_decision
```

V1 can implement fact extraction using simple rules.

Advanced extraction can be added later.

---

# 16. Context Package Format

Context Engine produces a Context Package.

```text
ContextPackage
├── active_request
├── system_context
├── developer_context
├── recent_messages
├── preserved_facts
├── decisions
├── constraints
├── relevant_memory
├── omitted_content_summary
├── token_budget_report
└── warnings
```

---

## 16.1 Example Context Package

```yaml
active_request:
  role: user
  content: "Proceed with Context and Prompt Engine specification."

preserved_facts:
  - Gumi is a local-first AI runtime.
  - V1 must not require cloud providers.
  - Gumi exposes OpenAI-compatible APIs.

decisions:
  - Use modular monolith architecture.
  - Use Engine terminology.
  - All requests pass through Pipeline Engine.

constraints:
  - Must work offline.
  - Must support Ollama and LM Studio.
  - Must be provider agnostic.

omitted_content_summary:
  - Earlier naming discussion was omitted.
  - Repeated architecture explanations were compressed.

token_budget_report:
  model_context_limit: 32000
  estimated_before: 18000
  estimated_after: 6200
```

---

# 17. Context Report

Every Context Engine run should produce a report.

```text
ContextReport
├── strategy_used
├── estimated_tokens_before
├── estimated_tokens_after
├── compression_ratio
├── items_removed
├── items_summarized
├── facts_preserved
├── warnings
└── fallback_used
```

Example:

```yaml
strategy_used: hybrid
estimated_tokens_before: 18200
estimated_tokens_after: 6400
compression_ratio: 0.35
items_removed: 12
items_summarized: 8
facts_preserved: 14
warnings: []
fallback_used: false
```

---

# 18. Context Failure Behaviour

Context Engine must fail gracefully.

## 18.1 Compression Failure

If compression fails:

```text
fallback to trim
record warning
continue pipeline
```

## 18.2 Token Estimation Failure

If token estimation fails:

```text
use approximate character-based estimate
record warning
continue pipeline
```

## 18.3 Context Still Too Large

If context is still too large:

```text
return CONTEXT_LIMIT_EXCEEDED
suggest reducing input or using bigger context model
```

---

# 19. Prompt Package Format

Prompt Engine produces a Prompt Package.

```text
PromptPackage
├── system_prompt
├── developer_instructions
├── model_profile_instructions
├── context_block
├── memory_block
├── user_messages
├── response_format_instructions
├── guardrail_instructions
├── final_messages
└── prompt_report
```

---

# 20. Prompt Construction Order

Prompt Engine should build prompts in this order:

```text
1. Base system prompt
2. Runtime mode instructions
3. Model profile instructions
4. Workspace instructions
5. Guardrail instructions
6. Context block
7. Memory block
8. Response format instructions
9. User messages
```

---

# 21. Base System Prompt

Default base system prompt:

```text
You are an AI assistant running through Gumi Runtime.

Follow the user's request accurately.
Do not invent facts.
When information is missing, say what is missing.
Avoid repeating yourself.
Keep output aligned with the requested format.
```

This should be configurable.

---

# 22. Runtime Mode Instructions

## 22.1 Direct Mode

```text
Gumi Direct Mode:
Respond normally using the provided messages.
Do not apply extra formatting unless requested.
```

## 22.2 Stabilized Mode

```text
Gumi Stabilized Mode:
Prioritize clarity, correctness, and non-repetition.
Use the provided context carefully.
Avoid hallucinating unsupported claims.
If the prompt is ambiguous, answer with reasonable assumptions and state them briefly.
```

## 22.3 Structured Mode

```text
Gumi Structured Mode:
Return only the requested structured output.
Do not include prose outside the structure.
Ensure the output can be parsed by a machine.
```

---

# 23. Model Profile Instructions

Model profile may add instructions.

Example:

```yaml
prompt:
  style: technical
  instruction_strength: strong
  json_instruction_style: explicit
```

Prompt Engine converts this into provider-ready instructions.

Example:

```text
Use precise technical language.
Follow formatting instructions strictly.
For JSON output, return raw JSON only.
```

---

# 24. Response Format Instructions

If `response_format.type = json_object`:

```text
Return a valid JSON object only.
Do not wrap the JSON in markdown.
Do not include explanation outside the JSON.
```

If `response_format.type = json_schema`:

```text
Return JSON that conforms exactly to the provided schema.
Do not include additional fields.
Do not include markdown fences.
```

---

# 25. Guardrail Instructions

Guard Engine may provide constraints.

Examples:

```text
Avoid repeating the same sentence.
If output becomes uncertain, stop cleanly instead of looping.
Do not continue generating after the answer is complete.
```

For anti-loop:

```text
Do not repeat phrases or paragraphs.
If you notice you are restating the same content, stop and provide the final answer.
```

---

# 26. Prompt Optimization Levels

Prompt Engine should support levels:

```text
off
light
standard
strict
```

## off

No optimization.

## light

Small system prompt improvements only.

## standard

Default stabilized prompt.

## strict

Use strong formatting and reliability instructions.

Structured Mode uses strict by default.

---

# 27. User Intent Preservation

Prompt Engine must preserve user intent.

It must not:

- change the requested task
- add unrelated constraints
- overrule user style unless unsafe
- invent missing requirements
- turn a simple request into a complex workflow
- add cloud or external dependency instructions

If optimization changes meaning, it is invalid.

---

# 28. Conflict Resolution

Instruction priority:

```text
1. System-level safety and runtime rules
2. Developer instructions
3. User instructions
4. Workspace instructions
5. Model profile instructions
6. Prompt optimization hints
```

Model profile instructions must never override user intent.

---

# 29. Prompt Report

Prompt Engine should produce a report.

```text
PromptReport
├── optimization_level
├── model_profile_applied
├── response_format_applied
├── guardrails_applied
├── system_prompt_tokens_estimated
├── final_prompt_tokens_estimated
├── warnings
└── changes
```

Example:

```yaml
optimization_level: standard
model_profile_applied: qwen3-8b
response_format_applied: false
guardrails_applied:
  - anti_loop
changes:
  - added anti-repetition instruction
  - added missing uncertainty instruction
warnings: []
```

---

# 30. Prompt Failure Behaviour

Prompt Engine should fail rarely.

## 30.1 Missing Model Profile

If profile missing:

```text
use generic local model profile
emit warning
continue pipeline
```

## 30.2 Prompt Too Large

If prompt package exceeds token budget:

```text
send back to Context Engine for stricter compression
or fail with CONTEXT_LIMIT_EXCEEDED
```

## 30.3 Invalid Response Format

If response format config is invalid:

```text
return INVALID_REQUEST or INVALID_CONFIG
```

---

# 31. Context and Prompt Telemetry Events

Required events:

```text
context_started
context_completed
context_compressed
context_trimmed
context_fallback_used
prompt_started
prompt_completed
prompt_optimized
model_profile_applied
response_format_instruction_applied
```

Optional events:

```text
duplicate_context_removed
important_facts_extracted
token_budget_estimated
prompt_warning
```

---

# 32. Example Full Flow

User request:

```json
{
  "model": "local:auto",
  "messages": [
    {
      "role": "user",
      "content": "Return JSON with project name and purpose."
    }
  ],
  "response_format": {
    "type": "json_object"
  },
  "gumi": {
    "mode": "structured"
  }
}
```

Context Engine output:

```yaml
context_package:
  active_request: "Return JSON with project name and purpose."
  preserved_facts:
    - "Project name is Gumi."
    - "Gumi is a local-first AI runtime."
  constraints:
    - "Return JSON only."
```

Prompt Engine final messages:

```json
[
  {
    "role": "system",
    "content": "You are an AI assistant running through Gumi Runtime. Return valid JSON only. Do not include markdown fences."
  },
  {
    "role": "user",
    "content": "Context:\\nProject name: Gumi\\nPurpose: local-first AI runtime.\\n\\nTask:\\nReturn JSON with project name and purpose."
  }
]
```

Expected model output:

```json
{
  "project_name": "Gumi",
  "purpose": "A local-first AI runtime that makes local models more stable and production-ready."
}
```

---

# 33. Testing Requirements

Context Engine tests:

- trim strategy
- summarize strategy
- compress strategy
- hybrid strategy
- duplicate removal
- token budget overflow
- compression fallback
- important fact preservation
- omitted content summary
- context report generation

Prompt Engine tests:

- direct mode prompt
- stabilized mode prompt
- structured mode prompt
- model profile application
- response format instructions
- anti-loop instructions
- prompt report generation
- user intent preservation
- missing profile fallback
- invalid response format handling

---

# 34. V1 Implementation Priority

Implement in this order:

```text
1. Message normalization
2. Token estimation
3. Trim strategy
4. Basic context package
5. Prompt package builder
6. Stabilized system prompt
7. Structured output instructions
8. Context report
9. Prompt report
10. Duplicate detection
11. Hybrid strategy
12. Basic compression
```

Reason:

Gumi must first become useful and predictable before adding advanced context intelligence.

---

# 35. Anti-Patterns

Avoid:

```text
Prompt Engine changing user intent
Context Engine removing current request
Context Engine keeping all history blindly
Prompt Engine adding too many instructions
Model profiles overriding user instructions
Hidden prompt changes with no telemetry
Compression without omitted content summary
Structured mode with vague output instructions
```

---

# 36. Final Statement

Context Engine decides what the model should know.

Prompt Engine decides how the model should be instructed.

Together, they form the first intelligence layer of Gumi.

They make local models less fragile by reducing context noise, preserving important information, applying model-specific guidance, and producing clearer provider-ready prompts.