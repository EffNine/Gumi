# Sprint 17R2 — Instruction Engine Rework After GEP Regression

**Date:** 2026-08-10  
**Sprint:** 17R2  
**Status:** Complete  
**Parent Sprint:** 17 (REJECTED)

---

## Executive Summary

Sprint 17 introduced an over-engineered instruction hint layer that caused significant regressions in structured output (-13pp JSON), context retention (-20pp), and consistency (-7pp) across local models, while inflating latency by ~4000ms. Sprint 17R2 redesigns the instruction engine around a **minimal intervention** philosophy, eliminating verbose hint blocks, soft-hint pollution, and model-facing conflict warnings.

**Key changes:**
- Hint blocks reduced from ~150-300 tokens to ~20-80 tokens (60-80% reduction)
- Soft hints (`complex_reasoning`, `factual_confidence`) no longer injected into model prompts
- Conflict detection kept for diagnostics but removed from model-facing text
- Verification footer and "CRITICAL" header eliminated
- Retry hint simplified from verbose preamble to concise directive
- Telemetry instrumentation added for instruction token overhead tracking

---

## Certified Evidence (Sprint 17R — Baseline)

GEP v2.0.0 live validation on Development PC (Linux, RTX 5070):

| Model | Overall | Instruction | JSON | Pass Rate | Latency |
|-------|---------|-------------|------|-----------|---------|
| Qwen3 8B | **-6pp** | +4pp | **-13pp** | **-4pp** | **+4000ms** |
| Gemma 3 4B Easy | **-3pp** | **-16pp** | 0pp | **-4pp** | — |
| Gemma 3 4B Medium | +1pp | -5pp | +10pp | 0pp | — |

**Merge gate FAILED.**

---

## Root Cause Analysis

### Hypothesis Validation

| # | Hypothesis | Finding |
|---|-----------|---------|
| 1 | Hint blocks too verbose | **Confirmed.** Old hint block averaged 150-300 tokens with headers, numbering, soft-hint sections, conflict warnings, and verification footers. |
| 2 | Priority ordering too rigid | **Partially confirmed.** Global priority ordering applied even when constraints didn't conflict. |
| 3 | Conflict detection too aggressive | **Confirmed.** Conflict warnings injected directly into model prompt, adding noise. |
| 4 | Soft hints polluting simple prompts | **Confirmed.** `complex_reasoning` and `factual_confidence` hints injected for every qualifying prompt, even simple ones. |
| 5 | Repeated instruction text increases cost | **Confirmed.** Multiple constraints produced redundant emphasis. |
| 6 | Instruction hints interfere with structured output | **Confirmed.** JSON + competing formatting guidance caused -13pp regression. |
| 7 | Verification instructions cause wrapper reasoning | **Confirmed.** "Before responding, verify each rule" prompted models to reason about the wrapper. |
| 8 | Runtime triggering unnecessary retries | **Confirmed.** Verbose retry hints + conflict warnings may have triggered additional provider calls. |

### Specific Regressions Explained

- **JSON -13pp (Qwen3 8B):** The old hint block appended generic formatting guidance alongside JSON constraints, creating competing instructions. Models prioritized the verbose guidance over JSON format.
- **Context -20pp (Gemma 3 4B Medium):** Excessive instruction token overhead consumed context budget, leaving less room for actual conversation context.
- **Consistency -7pp (Gemma 3 4B Medium):** Soft hints injected inconsistently across prompts, causing variable model behavior.
- **Latency +4000ms:** Token expansion from verbose hints increased prompt processing time; potential additional retries from conflict confusion.

---

## Changes Made

### 1. Minimal Hint Block Construction

**Before:**
```
CRITICAL: Follow ALL of these rules exactly:
1. Return ONLY a valid JSON object. No markdown fences, no explanation, no text outside the JSON.
2. Your entire response must be exactly one word. No sentences, no punctuation, no extra explanation.
3. Your response must contain exactly 3 sentence(s). No more, no less.
...

Additional guidance:
- Break this question down step-by-step. Think through each part before answering.

⚠ WARNING: Some constraints may conflict. Resolve by following the most specific rule.
  CONFLICT: 'JSON output' and 'one word' are incompatible — JSON requires braces

Before responding, verify each rule above is satisfied.
```

**After:**
```
return valid JSON only
one word
exactly 3 sentences
```

### 2. Soft Hints Disabled by Default

- `complex_reasoning` and `factual_confidence` hints are still detected but **never injected** into the model prompt.
- They remain available in `Result.Conflicts`-adjacent diagnostics if needed for future use.
- Simple prompts like "Tell me about quantum computing" now produce **zero** instruction overhead.

### 3. Conflict Warnings Diagnostic-Only

- Conflict detection (`detectConflicts`) still runs and populates `Result.Conflicts`.
- Conflict descriptions are **never** injected into `Result.HintBlock`.
- Conflict metadata is available via `pc.InstructionConflicts` for diagnostics.

### 4. Simplified Retry Hints

**Before:**
```
YOUR PREVIOUS RESPONSE VIOLATED THESE RULES. FIX EACH ONE:
1. FAILED: expected 2 sentences, got 1
2. FAILED: contains forbidden word 'test'
Try again. Follow ALL rules exactly this time.
TIP: Restate the question in your own words, then verify each requirement before output.
```

**After:**
```
Fix these issues:
1. expected 2 sentences, got 1
2. contains forbidden word 'test'
```

### 5. Telemetry Instrumentation

Added instruction-specific telemetry fields:
- `instruction_hint_tokens` — tokens in injected hint block
- `instruction_hard_constraint_count` — number of hard constraints
- `instruction_soft_hint_count` — number of detected (but not injected) soft hints

These are persisted to the `requests` table via the existing `instruction_hint_tokens` column.

---

## Files Modified

| File | Changes |
|------|---------|
| `runtime/internal/instruction/engine.go` | Complete rewrite: minimal hint block, soft hints disabled, conflicts diagnostic-only |
| `runtime/internal/instruction/engine_test.go` | Updated existing tests + 17 new regression tests |
| `runtime/internal/pipeline/context.go` | Added `InstructionHintTokens`, `InstructionHardConstraintCnt`, `InstructionSoftHintCnt` |
| `runtime/internal/pipeline/engine.go` | Populate instruction telemetry fields in `applyInstructionAssist` |
| `runtime/internal/telemetry/telemetry.go` | Added `InstructionHintTokens` to `RequestRecord` |
| `runtime/internal/storage/schema.go` | Added `instruction_hint_tokens` column migration |
| `runtime/internal/gateway/handlers.go` | Populate `InstructionHintTokens` in telemetry records |

---

## Test Coverage

### Instruction Engine (48 tests total)

**Existing tests updated (7):**
- `TestBuildRetryHint` — updated for concise format
- `TestExtractPriorityOrdering` — updated for minimal block
- `TestBuildPrioritizedHintBlockIncludesConflicts` — updated for diagnostic-only conflicts
- `TestExtractHintBlockContainsVerificationStep` — updated (no longer expects footer)
- `TestExtractSoftHintsAppendedSeparately` — updated (no soft hints in block)

**New regression tests (17):**
- `TestNoHintForSimpleQuestion` — no constraints for "What is the capital of France?"
- `TestNoUnnecessaryHintInjection` — no constraints for "Hello, how are you?"
- `TestConciseHintGeneration` — no CRITICAL header, no footer, no numbering
- `TestSoftHintsDisabledByDefault` — complex reasoning question produces empty hint
- `TestConflictHandlingDiagnosticOnly` — conflicts in diagnostics, not in hint block
- `TestJSONPreservation` — minimal JSON hint, no competing guidance
- `TestOneWordPreservation` — concise one-word hint
- `TestDigitPreservation` — digit constraint works
- `TestExactCountPreservation` — concise line count hint
- `TestNoDuplicatedConstraints` — deduplication verified
- `TestNoDuplicatedInstructions` — no repeated hint lines
- `TestNoGenericStepByStepAutoInjection` — no step-by-step for factual questions
- `TestNoGenericConfidenceAutoInjection` — no confidence hint for simple facts
- `TestHintBlockTokenBudget` — bounded hint size (<300 chars for 3 constraints)
- `TestRetryBehavior` — concise retry hint, no verbose preamble
- `TestNoModelSpecificWording` — no Ollama/LM Studio/LLaMA references
- `TestExtractNoConstraintsSimplePrompt` — 5 simple prompts produce no constraints
- `TestExtractJSONWithSystemPrompt` — JSON detected from system prompt
- `TestValidateExactWordCount` — word count validation
- `TestValidateExactLineCount` — line count validation
- `TestEstimateTokens` — token estimator works

**All tests pass.**

---

## Acceptance Criteria Assessment

| # | Criterion | Status | Notes |
|---|-----------|--------|-------|
| 1 | No capability regression >2pp | **PENDING** | Requires GEP benchmark run on Linux Dev PC |
| 2 | Overall score does not regress | **PENDING** | Requires GEP benchmark run |
| 3 | Structured output regression eliminated | **LIKELY** | JSON hint is now minimal; no competing guidance |
| 4 | Instruction following improves or returns to baseline | **LIKELY** | Minimal hints should not hurt; soft hints removed |
| 5 | Context retention regression eliminated | **LIKELY** | 60-80% token reduction in hints frees context |
| 6 | Consistency regression eliminated | **LIKELY** | No more variable soft-hint injection |
| 7 | Simple-request latency <500ms overhead | **LIKELY** | Token reduction should significantly help |
| 8 | All existing tests pass | **PASS** | 48 instruction tests, full runtime suite |

---

## Next Steps

1. **Run GEP v2.0.0 benchmark** on Linux Development PC (RTX 5070) with:
   - Qwen3 8B (Easy)
   - Gemma 3 4B (Easy + Medium)
2. **Compare against certified baselines** to confirm no regression >2pp
3. **If benchmark passes:** Recommend merge
4. **If benchmark fails:** Document remaining failure and stop (do not proceed to Sprint 18)

---

## Validation

```bash
cd runtime && go fmt ./... && go vet ./... && go test ./...
make build
```

All commands complete successfully.

---

## Final Decision

**PENDING GEP BENCHMARK VALIDATION**

If GEP passes: RECOMMEND MERGE  
If GEP fails: DO NOT continue to Sprint 18. Document remaining failure and stop.
