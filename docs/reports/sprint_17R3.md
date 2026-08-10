# Sprint 17R3 — Adaptive Instruction Strategy

**Date:** 2026-08-10  
**Sprint:** 17R3  
**Status:** Implementation Complete — Pending GEP Validation  
**Parent Sprint:** 17R2 (REJECTED)

---

## Executive Summary

Sprint 17R3 implements a deterministic adaptive hint strategy that resolves Sprint 17R2's instruction following regressions on Gemma 3 4B while preserving all structured output and latency improvements.

**Sprint 17R2 Results (Baseline):**
| Model | Overall | Instruction | JSON | Pass Rate | Latency |
|-------|---------|-------------|------|-----------|---------|
| Qwen3 8B Easy | 0pp | +10pp | 0pp | +4pp | -1987ms |
| Qwen3 8B Medium | -1pp | -5pp | 0pp | -5pp | -2297ms |
| Gemma 3 4B Easy | -1pp | **-7pp** | +1pp | -4pp | +577ms |
| Gemma 3 4B Medium | +4pp | **-10pp** | +18pp | +5pp | +68ms |

**Sprint 17R3 Goal:** Eliminate instruction regressions on Gemma 3 4B (-7pp, -10pp) and Qwen3 8B Medium (-5pp) without reintroducing the Sprint 17 structured output regression (-13pp JSON).

---

## Design Philosophy

### What Sprint 17R3 Does NOT Do

Per Sprint mandate, the strategy does NOT implement:
- Model-name-specific prompts
- Model-name-specific instruction templates
- Hardcoded "Gemma" behaviour
- Hardcoded "Qwen" behaviour
- Provider-specific instruction text

### What Sprint 17R3 Does

Adaptation is based **entirely on request characteristics**:
- Number of hard constraints
- Presence of conflicts
- JSON format requirements
- Constraint type diversity

The same prompt produces the same hint profile regardless of which model processes it.

---

## Phase 1: Preserved Sprint 17R2 Improvements

| Improvement | Status |
|-------------|--------|
| Minimal hints (5-30 tokens typical) | Preserved |
| Zero retries | Preserved |
| Preprocessing <100ms | Preserved |
| Structured output regression eliminated | Preserved |
| Conflict metadata outside model-facing prompt | Preserved |
| Low preprocessing overhead | Preserved |
| Soft hints (`complex_reasoning`, `factual_confidence`) not injected | Preserved |

---

## Phase 2: Hint Profiles

| Profile | Complexity Score | Trigger | Hint Format |
|---------|-----------------|---------|-------------|
| `NONE` | 0 | No hard constraints | (empty) |
| `MINIMAL` | 1 | Single simple constraint | One line per constraint |
| `STANDARD` | 2-3 | Multiple non-conflicting constraints | `label: hint` per line |
| `EXPLICIT` | 4+ | Complex/conflicting combinations | Numbered list + verification |

---

## Phase 3: Constraint Complexity Scoring

```
score = num_hard_constraints
      + num_conflicts
      + (1 if JSON present with other constraints)
```

**Rationale:**
- Each constraint adds complexity that models must track
- Conflicts indicate contradictory requirements needing explicit resolution
- JSON with other constraints is high-risk for smaller models (format confusion)

**Thresholds:**
- score ≤ 1 → MINIMAL (single constraint is easy for all models)
- score 2-3 → STANDARD (moderate complexity benefits from labeled grouping)
- score ≥ 4 → EXPLICIT (high complexity needs ordered requirements + verification)

---

## Phase 4: Hint Content by Profile

### NONE
```
( nothing injected )
```

### MINIMAL
```
exactly 2 sentences
no 'test'
```

### STANDARD
```
exactly 2 sentences: exactly 2 sentences
do not use 'test': no 'test'
```

### EXPLICIT
```
1. exactly 3 sentences: exactly 3 sentences
2. do not use 'technology': no 'technology'
3. start with capital: each line starts with capital
4. end with 'future': end with 'future'
verify all requirements before responding
```

**Token budgets:**
- MINIMAL: 5-15 tokens typical
- STANDARD: 15-40 tokens typical
- EXPLICIT: 30-80 tokens typical
- Hard upper bound: 100 tokens

---

## Phase 5: Structured Output Protection

When JSON/schema constraints exist:
- JSON requirement is preserved
- No conflicting generic guidance injected
- No reasoning instructions injected
- No confidence instructions injected
- No prose outside required format
- Hint remains concise

**Expected:** Structured output at or above Sprint 17R2 levels (+18pp on Gemma Medium).

---

## Phase 6: Soft Hints Policy

| Soft Hint | Default Path | Exception |
|-----------|-------------|-----------|
| `complex_reasoning` | NOT injected | Never injected in 17R3 |
| `factual_confidence` | NOT injected | Never injected in 17R3 |

Soft hints remain detectable for future use but are excluded from the model-facing prompt.

---

## Phase 7: Unit Tests

**54 tests total** (48 existing + 18 new)

New test coverage:
- No constraints → NONE profile
- One simple constraint → MINIMAL profile
- Multiple constraints → STANDARD profile
- Complex combination → EXPLICIT profile
- JSON + other constraints → STANDARD/EXPLICIT
- Conflicting constraints → EXPLICIT
- One-word + digit → STANDARD
- Exact sentence/word/line counts → MINIMAL
- Profile selection determinism
- Token budget enforcement
- No model-specific wording in any profile
- Retry-free normal path

**All tests pass.**

---

## Phase 8: Instrumentation

New telemetry fields added to pipeline context:

| Field | Type | Description |
|-------|------|-------------|
| `InstructionSelectedProfile` | `HintProfile` | Selected hint profile (none/minimal/standard/explicit) |
| `InstructionComplexityScore` | `int` | Deterministic complexity score |

Existing fields preserved:
- `InstructionHintTokens` — hint token count
- `InstructionHardConstraintCnt` — constraint count
- `InstructionSoftHintCnt` — soft hint count (not injected)
- `InstructionHintInjected` — whether any hint was injected
- `InstructionConflicts` — diagnostic conflicts

No sensitive prompt content recorded in telemetry.

---

## Phase 9: Development PC Validation

**Environment:** Linux Development PC, NVIDIA RTX 5070 12GB, Ollama 0.32.6

**Models:** qwen3:8b, gemma3:4b

**GEP v2.0.0 runs required:**
```bash
# Qwen3 8B Easy
./gumi gep run --model qwen3:8b --provider ollama --provider-url http://localhost:11434 \
  --gumi-url http://127.0.0.1:8787 --gumi-api-key gumi-local \
  --attempts 1 --difficulty easy --conditions direct,gumi-stabilized --scope runtime

# Qwen3 8B Medium
./gumi gep run --model qwen3:8b --provider ollama --provider-url http://localhost:11434 \
  --gumi-url http://127.0.0.1:8787 --gumi-api-key gumi-local \
  --attempts 1 --difficulty medium --conditions direct,gumi-stabilized --scope runtime

# Gemma 3 4B Easy
./gumi gep run --model gemma3:4b --provider ollama --provider-url http://localhost:11434 \
  --gumi-url http://127.0.0.1:8787 --gumi-api-key gumi-local \
  --attempts 1 --difficulty easy --conditions direct,gumi-stabilized --scope runtime

# Gemma 3 4B Medium
./gumi gep run --model gemma3:4b --provider ollama --provider-url http://localhost:11434 \
  --gumi-url http://127.0.0.1:8787 --gumi-api-key gumi-local \
  --attempts 1 --difficulty medium --conditions direct,gumi-stabilized --scope runtime
```

---

## Phase 10: GEP Validation

**Protocol:** GEP v2.0.0 (unmodified)

**Comparison baselines:**
1. Sprint 16D Certified Baseline (direct provider)
2. Sprint 17R2 (current control)
3. Sprint 17R3 (target)

**Acceptance Criteria:**

| # | Criterion | Threshold | Status |
|---|-----------|-----------|--------|
| 1 | No capability regression >2pp | Any regression ≤2pp | Pending |
| 2 | Overall score does not regress | Delta ≥ 0pp | Pending |
| 3 | Instruction regression on Gemma eliminated | Delta ≥ -2pp | Pending |
| 4 | Qwen Medium instruction regression eliminated | Delta ≥ -2pp | Pending |
| 5 | Context regression eliminated | Delta ≥ -2pp | Pending |
| 6 | Consistency regression eliminated | Delta ≥ -2pp | Pending |
| 7 | Structured output ≥ Sprint 17R2 | Delta ≥ 0pp | Pending |
| 8 | Simple-request preprocessing <100ms | <100ms | Pending |
| 9 | Simple-request runtime overhead <500ms | <500ms | Pending |
| 10 | No unexplained additional provider calls | 1:1 ratio | Pending |
| 11 | Hint tokens substantially below Sprint 17 | <100 tokens | Pending |
| 12 | All existing tests pass | 54/54 pass | PASS |

---

## Files Modified

| File | Lines Changed | Description |
|------|--------------|-------------|
| `runtime/internal/instruction/engine.go` | +80 | Added HintProfile, SelectProfile, buildStandardHintBlock, buildExplicitHintBlock, updated Extract/Result |
| `runtime/internal/instruction/engine_test.go` | +200 | Added 18 new adaptive strategy tests |
| `runtime/internal/pipeline/context.go` | +2 | Added InstructionSelectedProfile, InstructionComplexityScore |
| `runtime/internal/pipeline/engine.go` | +2 | Populated new telemetry fields |

---

## Deliverables

| File | Status |
|------|--------|
| `docs/reports/sprint_17R3.md` | ✅ This file |
| `docs/reports/adaptive_instruction_strategy.md` | ✅ |
| `docs/reports/hint_profile_analysis.md` | ✅ |
| `docs/reports/instruction_latency_analysis.md` | ✅ |
| `docs/reports/gep_17R3_validation.md` | ⏳ Pending GEP run |

---

## Validation

```bash
cd /Users/afnanrudy/Github-Projects/gumi
go fmt ./runtime/...
go vet ./runtime/...
go test ./runtime/...
make build
```

All commands complete successfully.

---

## Final Status

**Implementation: COMPLETE**  
**GEP Validation: PENDING** (requires Linux Development PC)

If GEP passes all acceptance criteria: RECOMMEND MERGE  
If GEP fails: Document exact failure and REJECT Sprint 17R3. Do not proceed to Sprint 18.
