# Instruction Engine Review

**Date:** 2026-08-09  
**Sprint:** 17  
**Component:** `runtime/internal/instruction/engine.go`

---

## Current Architecture

The Instruction Engine extracts formatting and content constraints from user prompts, injects reminder hints into the system prompt, and validates responses post-generation with automatic retry on failure.

### Pipeline Integration

```
User Prompt ──► Extract() ──► Result{Constraints, HintBlock}
                            │
                            ├─► Injected into system prompt (applyInstructionAssist)
                            └─► Stored in pc.InstructionConstraints
                                         │
                                         ▼
                              Post-generation Validate()
                                         │
                                         ├─► All pass ──► Return response
                                         └─► Fail ──► Retry with BuildRetryHint()
```

### Constraint Types Supported

| Type | Check Function | Description |
|------|---------------|-------------|
| `sentences` | `sentence_count` | Exact sentence count |
| `words` | `word_count` | Exact word count |
| `lines` | `line_count` | Exact line count |
| `bullets` | `dash_bullets` | Dash-format bullet points |
| `no_word` | `forbidden_word` | Forbidden word exclusion |
| `end_with` | `end_with` | Response must end with specific word |
| `capital_start` | `capital_start` | Each line starts with capital |
| `json` | `json` | Valid JSON output |
| `min_chars` | `min_chars` | Minimum character count |
| `min_words` | `min_words` | Minimum word count |
| `no_commas` | `no_commas` | No comma characters |
| `no_markdown` | `no_markdown` | No markdown formatting |
| `sections` | `sections` | Highlighted sections |
| `no_rhyme` | `no_rhyme` | No rhyming lines |
| `digit_answer` | `digit_answer` | Numeric digit only |
| `one_word` | `one_word` | Exactly one word |

### Soft Hints (Check = "none")

| Type | Description |
|------|-------------|
| `complex_reasoning` | Step-by-step reasoning guidance |
| `factual_confidence` | Confidence indication for factual claims |

These are appended after hard constraints but do not trigger `HasConstraints=true`.

---

## Identified Issues

### 1. No Constraint Priority Ordering
**Problem:** Constraints were appended in detection order, not importance order. JSON and one-word constraints (most commonly violated by local models) appeared after less critical constraints.

**Impact:** Local models process the first hints most carefully. Critical format constraints buried deep in the hint block are more likely to be missed.

### 2. No Deduplication
**Problem:** If the same constraint type appeared multiple times (e.g., two `no_word` constraints for the same word from different regex matches), both would be injected into the hint block.

**Impact:** Redundant hints waste token budget and can confuse models.

### 3. No Conflict Detection
**Problem:** Contradictory constraints (e.g., "one word" + "exactly 5 words", or "JSON" + "one word") were silently accepted.

**Impact:** Models given contradictory instructions produce inconsistent or hallucinated outputs.

### 4. Conflicting System Prompt Guidance
**Problem:** The default system prompt includes "Quality guidelines: think step-by-step for complex tasks..." which directly conflicts with format-restrictive constraints like `one_word`, `digit_answer`, and exact count constraints.

**Impact:** Models receiving both verbose guidance and strict format constraints show reduced compliance on constrained tasks.

### 5. Verbose Hint Block Construction
**Problem:** The original hint block used a simple numbered list without grouping or priority separation.

**Impact:** Harder for local models to parse and prioritize constraints.

---

## Optimization Summary

| # | Optimization | File | Lines Changed |
|---|-------------|------|---------------|
| 1 | Priority-ordered hint block | `instruction/engine.go` | +50 |
| 2 | Constraint deduplication | `instruction/engine.go` | +15 |
| 3 | Conflict detection | `instruction/engine.go` | +55 |
| 4 | Remove conflicting guidance | `pipeline/engine.go` | +25 |
| 5 | Context tracking for optimizations | `pipeline/context.go` | +4 |
| 6 | New unit tests | `instruction/engine_test.go` | +80 |

**Total new code:** ~200 lines  
**Total tests added:** 7
