# Instruction Engine Rework — Sprint 17R2

**Date:** 2026-08-10  
**Component:** `runtime/internal/instruction/engine.go`  
**Sprint:** 17R2

---

## Problem Statement

Sprint 17's instruction engine optimization introduced regressions across multiple capabilities:

| Capability | Qwen3 8B | Gemma 3 4B Easy | Gemma 3 4B Medium |
|-----------|----------|-----------------|-------------------|
| Overall | -6pp | -3pp | +1pp |
| Instruction | +4pp | -16pp | -5pp |
| Structured Output / JSON | **-13pp** | 0pp | +10pp |
| Context Retention | — | — | **-20pp** |
| Consistency | — | — | **-7pp** |
| Pass Rate | **-4pp** | **-4pp** | 0pp |
| Latency | **+4000ms** | — | — |

The primary hypothesis: **the instruction hint layer was over-engineered**, producing verbose, competing guidance that confused local models and inflated prompt token counts.

---

## Redesign Principles

1. **No hard constraint → inject nothing.**
2. **Simple hard constraint → inject only the minimum necessary reminder.**
3. **Multiple hard constraints → combine into a concise instruction block.**
4. **Soft hints must NOT be injected by default.**
5. **Complex reasoning and factual confidence must not modify simple prompts.**
6. **Do not repeat the original user instruction unnecessarily.**
7. **Do not add generic "think step-by-step" guidance automatically.**
8. **Do not add generic confidence instructions automatically.**
9. **Preserve exact user intent.**
10. **Never introduce model-specific prompt wording.**

---

## Architecture Changes

### Before (Sprint 17)

```
User Prompt → Extract() → Result{Constraints, HintBlock, Conflicts}
                                      │
                                      ├─→ "CRITICAL: Follow ALL of these rules exactly:"
                                      ├─→ Numbered priority-ordered list
                                      ├─→ "Additional guidance:" soft hints section
                                      ├─→ "⚠ WARNING: Some constraints may conflict."
                                      └─→ "Before responding, verify each rule above is satisfied."
```

Typical hint block size: **150-300 tokens**

### After (Sprint 17R2)

```
User Prompt → Extract() → Result{Constraints, HintBlock, Conflicts}
                                      │
                                      ├─→ Minimal constraint hints (one line each)
                                      ├─→ Conflicts → diagnostics only (not injected)
                                      └─→ Soft hints → detected but not injected
```

Typical hint block size: **20-80 tokens** (60-80% reduction)

---

## Code Changes

### 1. `runtime/internal/instruction/engine.go`

**Rewritten `Extract()` method:**
- Separated constraint extraction into `extractConstraints()` returning `(hardConstraints, softHints)`
- Soft hints (`complex_reasoning`, `factual_confidence`) detected but never injected
- Replaced `buildPrioritizedHintBlock()` with `buildMinimalHintBlock()`

**New `buildMinimalHintBlock()`:**
- One line per constraint
- No headers, no footers, no numbering
- No soft hints
- No conflict warnings
- Maximum 80 chars per line, 300 chars total for multi-constraint prompts

**Simplified `BuildRetryHint()`:**
- Removed "YOUR PREVIOUS RESPONSE VIOLATED THESE RULES. FIX EACH ONE:" preamble
- Removed "TIP: Restate the question..." footer
- Kept concise "Fix these issues:" header with numbered violations

### 2. `runtime/internal/pipeline/context.go`

Added instruction telemetry fields to `Context`:
```go
InstructionHintTokens        int
InstructionHardConstraintCnt int
InstructionSoftHintCnt       int
```

### 3. `runtime/internal/telemetry/telemetry.go`

Added `InstructionHintTokens` to `RequestRecord` for persistence.

### 4. `runtime/internal/storage/schema.go`

Added `instruction_hint_tokens` column migration (auto-applied via `ensureColumn`).

### 5. `runtime/internal/gateway/handlers.go`

Populated `InstructionHintTokens` from pipeline context in both streaming and non-streaming telemetry paths.

---

## Hint Block Examples

### Single Constraint

**Input:** "Answer in exactly 2 sentences."  
**Before:**
```
CRITICAL: Follow ALL of these rules exactly:
1. Your response must contain exactly 2 sentence(s). No more, no less.
Before responding, verify each rule above is satisfied.
```
**After:**
```
exactly 2 sentences
```

### Multiple Constraints

**Input:** "Return valid JSON. Do not use the word 'test'. Answer in exactly 3 sentences."  
**Before:**
```
CRITICAL: Follow ALL of these rules exactly:
1. Return ONLY a valid JSON object. No markdown fences, no explanation, no text outside the JSON.
2. Your response must contain exactly 3 sentence(s). No more, no less.
3. Do NOT use the word 'test' anywhere in your response.
Before responding, verify each rule above is satisfied.
```
**After:**
```
return valid JSON only
no 'test'
exactly 3 sentences
```

### No Hard Constraints

**Input:** "Tell me about quantum computing."  
**Before:**
```
CRITICAL: Follow ALL of these rules exactly:
...
Additional guidance:
- If you are uncertain about any factual claim, state your confidence level.
```
**After:**
```
(empty — no injection)
```

### JSON-Only Prompt

**Input:** "Return a JSON object with name and age."  
**Before:**
```
CRITICAL: Follow ALL of these rules exactly:
1. Return ONLY a valid JSON object. No markdown fences, no explanation, no text outside the JSON.
...
Before responding, verify each rule above is satisfied.
```
**After:**
```
return valid JSON only
```

---

## Soft Hint Policy

| Scenario | Old Behavior | New Behavior |
|----------|-------------|--------------|
| Factual question ("What is X?") | Injected confidence hint | No injection |
| Complex reasoning question | Injected step-by-step hint | No injection |
| Multi-step task | Injected step-by-step hint | No injection |
| Constraint + factual question | Injected both hard + soft hints | Only hard constraint injected |

Soft hints are still **detected** and available in `Result.Constraints` with `Check: "none"`, but they are excluded from `HintBlock` and do not trigger `HasConstraints = true`.

---

## Conflict Handling

Conflicts are still detected but moved to **diagnostics-only**:

| Conflict Type | Diagnostic Message | Model-Facing |
|--------------|-------------------|--------------|
| JSON + one_word | "CONFLICT: 'JSON output' and 'one word' are incompatible" | Never injected |
| one_word + word_count(n≠1) | "CONFLICT: 'one word' but also 'exactly N words'" | Never injected |
| one_word + sentence_count(1) | "NOTE: compatible only if single word" | Never injected |
| digit_answer + one_word | "NOTE: compatible" | Never injected |

Conflict metadata remains accessible via `pc.InstructionConflicts` for observability.

---

## Telemetry

New fields tracked per request:

| Field | Description |
|-------|-------------|
| `instruction_hint_tokens` | Approximate token count of injected hint block |
| `instruction_hard_constraint_count` | Number of hard constraints extracted |
| `instruction_soft_hint_count` | Number of soft hints detected (not injected) |
| `instruction_retry_count` | Number of instruction-constraint retries |
| `instruction_hint_injected` | Whether any hint was injected |

These fields are persisted to the `requests` table and available via the dashboard telemetry API.

---

## Backward Compatibility

- `Constraint` struct: **unchanged**
- `Result` struct: `PriorityOrder` field removed (was unused)
- `Extract()` signature: **unchanged**
- `Validate()` signature: **unchanged**
- `IsFormatRestrictive()` signature: **unchanged**
- `BuildRetryHint()` signature: **unchanged**
- Database schema: **backward compatible** (new column has DEFAULT 0)

---

## Test Results

```
ok  github.com/EffNine/gumi/runtime/internal/instruction  0.317s  (48 tests)
ok  github.com/EffNine/gumi/runtime/internal/pipeline      6.482s  (all pass)
ok  github.com/EffNine/gumi/runtime/internal/gateway       2.054s  (all pass)
ok  github.com/EffNine/gumi/runtime/internal/telemetry     (cached)
ok  github.com/EffNine/gumi/runtime/internal/storage       (cached)
```
