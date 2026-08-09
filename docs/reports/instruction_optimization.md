# Instruction Engine Optimization Report

**Date:** 2026-08-09  
**Sprint:** 17  
**Status:** Complete

---

## Optimizations Implemented

### 1. Priority-Ordered Hint Block

**Before:**
```
CRITICAL: Follow these rules exactly:
1. Your response must contain exactly 3 sentence(s). No more, no less.
2. Do NOT use the word 'technology' anywhere.
3. Each line of your response MUST start with a capital letter.
4. Your response must END with the word 'future'.
Before responding, verify each rule above is satisfied.
```

**After:**
```
CRITICAL: Follow ALL of these rules exactly:
1. Return ONLY a valid JSON object. No markdown fences, no explanation, no text outside the JSON.
2. Your entire response must be exactly one word. No sentences, no punctuation, no extra explanation.
3. Your response must contain exactly 3 sentence(s). No more, no less.
4. Do NOT use the word 'technology' anywhere.
...
Before responding, verify each rule above is satisfied.
```

**Rationale:** Local models (especially <8B) are most sensitive to the first few instructions in a prompt. By promoting JSON, one_word, and digit_answer constraints to the top, compliance improves on the most commonly failed constraints.

**Priority Order:**
| Priority | Constraint Type | Rationale |
|----------|----------------|-----------|
| 1 | `json` | Most commonly failed; JSON validation is strict |
| 2 | `one_word` | Simple format, easy to miss |
| 3 | `digit_answer` | Math models often spell numbers |
| 4 | `sentences` | Exact count is hard for small models |
| 5 | `words` | Exact word count is challenging |
| 6 | `lines` | Line-based constraints |
| 7+ | Other constraints | Lower priority, less commonly violated |

### 2. Constraint Deduplication

**Change:** Added `deduplicateConstraints()` function that removes duplicate constraint types, keeping the first occurrence.

**Example:**
```go
// Input: "Do not use the word 'test'. Avoid the word 'test'."
// Before: 2 no_word constraints for "test"
// After: 1 no_word constraint for "test", DeduplicatedCount=1
```

**Rationale:** Reduces hint block token usage and prevents model confusion from repeated instructions.

### 3. Conflict Detection

**Change:** Added `detectConflicts()` function that identifies contradictory constraint combinations.

**Detected Conflicts:**
| Conflict | Detection |
|----------|-----------|
| `one_word` + `word_count` (n≠1) | `CONFLICT: 'one word' but also 'exactly N words'` |
| `json` + `one_word` | `CONFLICT: 'JSON output' and 'one word' are incompatible` |
| `one_word` + `sentence_count` (1) | `NOTE: compatible only if sentence is a single word` |
| `digit_answer` + `one_word` | `NOTE: compatible — a digit is a single word` |

**Rationale:** When conflicts exist, the model needs explicit warning to prioritize the most specific rule. The hint block now appends conflict warnings at the end.

### 4. Removal of Conflicting System Prompt Guidance

**Change:** In `applyInstructionAssist()`, when format-restrictive constraints are detected (`one_word`, `digit_answer`, exact counts), the "Quality guidelines: think step-by-step..." line is removed from the system prompt.

**Rationale:** The step-by-step guidance encourages verbose, multi-sentence responses. When the user explicitly requests "one word" or "exactly 3 sentences", this guidance actively works against the constraint. Removing it eliminates a source of contradiction.

### 5. Improved Hint Block Structure

**Before:**
```
CRITICAL: Follow these rules exactly:
1. <hint>
2. <hint>
3. <hint>
Before responding, verify each rule above is satisfied.
```

**After:**
```
CRITICAL: Follow ALL of these rules exactly:
1. <priority-1 hint>
2. <priority-2 hint>
...

Additional guidance:
- <soft hint 1>
- <soft hint 2>

⚠ WARNING: Some constraints may conflict. Resolve by following the most specific rule.
  CONFLICT: 'JSON output' and 'one word' are incompatible — JSON requires braces

Before responding, verify each rule above is satisfied.
```

**Rationale:** Clearer separation between hard constraints (must follow) and soft hints (recommended). Conflict warnings are prominently displayed.

---

## Files Modified

| File | Changes |
|------|---------|
| `runtime/internal/instruction/engine.go` | +120 lines (priority ordering, dedup, conflicts, new Result fields) |
| `runtime/internal/pipeline/engine.go` | +30 lines (conflicting guidance removal, metadata tracking) |
| `runtime/internal/pipeline/context.go` | +4 lines (new context fields) |
| `runtime/internal/instruction/engine_test.go` | +80 lines (7 new tests) |

---

## Backward Compatibility

All changes are backward compatible:
- `Constraint` struct is unchanged
- `Result` struct gains new optional fields (`Conflicts`, `DeduplicatedCount`)
- `Extract()` signature unchanged
- `Validate()` signature unchanged
- Existing tests all pass

---

## Test Coverage

| Package | Before | After |
|---------|--------|-------|
| `instruction` | 34 tests | 41 tests |
| `pipeline` | 42 tests | 42 tests (all pass) |

New tests:
- `TestExtractPriorityOrdering` — verifies JSON before one_word before sentences
- `TestExtractDeduplication` — verifies duplicate constraints are merged
- `TestExtractConflictDetection` — verifies JSON+one_word conflict detected
- `TestExtractConflictOneWordVsWordCount` — verifies one_word+5_words conflict
- `TestIsFormatRestrictive` — verifies format-restrictive detection logic
- `TestBuildPrioritizedHintBlockIncludesConflicts` — verifies warnings in hint
- `TestExtractHintBlockContainsVerificationStep` — verifies footer present
- `TestExtractSoftHintsAppendedSeparately` — verifies soft hints grouping
